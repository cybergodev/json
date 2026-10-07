package json

import (
	"log/slog"
	"sync/atomic"
	"time"
)

// Close closes the processor and cleans up resources.
// This method is idempotent and thread-safe.
// After Close is called, all operations on the processor will return ErrProcessorClosed.
//
// IMPORTANT: Always call Close() to release resources:
//
//	processor, err := json.New()
//	if err != nil {
//	    return err
//	}
//	defer processor.Close()
func (p *Processor) Close() error {
	if p == nil {
		return nil
	}
	p.cleanupOnce.Do(func() {
		// Mark as closing so new operations fail fast via checkClosed().
		atomic.StoreInt32(&p.state, processorStateClosing)

		// Wait for in-flight operations to finish (bounded by closeOperationTimeout).
		// Previously this drained the concurrency semaphore by RECEIVING tokens,
		// which raced with releaseSemaphore() (also a receive) and could strand
		// in-flight operations blocking forever on their release. The atomic
		// activeOps counter (incremented by beginGovernedOp) has no such contention.
		// If the drain timed out, in-flight operations are STILL running. Do NOT
		// tear down the cache or security validator: those ops may concurrently
		// read/write them (getCachedResult/setCachedResult), and tearing them down
		// mid-flight races. Leave resources intact and mark CloseTimedOut so new
		// ops are rejected (IsClosed() returns true) while the in-flight ones
		// finish against undisturbed state.
		if !p.waitForActiveOps(closeOperationTimeout) {
			atomic.StoreInt32(&p.state, processorStateCloseTimedOut)
			// D-002: if the last in-flight op raced this store and finished
			// between the timeout and the state write, no endGovernedOp will
			// fire the deferred teardown below — finish it here. (Mirrors
			// beginGovernedOp's double-check pattern.)
			if atomic.LoadInt64(&p.activeOps) == 0 {
				p.finishDeferredTeardown()
			}
			return
		}

		// All in-flight operations have drained — safe to release resources.
		p.releaseProcessorResources()
	})

	return nil
}

// finishDeferredTeardown runs the resource release exactly once after a
// CloseTimedOut drain, from whichever side observes idle first: the last
// in-flight operation's endGovernedOp, or Close's post-timeout recheck.
func (p *Processor) finishDeferredTeardown() {
	p.deferredTeardownOnce.Do(func() {
		p.releaseProcessorResources()
	})
}

// releaseProcessorResources tears down the processor's own resources. It may
// only run once every in-flight operation has completed (see Close).
func (p *Processor) releaseProcessorResources() {
	// Safely close cache: cancels cleanup goroutines and clears data
	if p.cache != nil {
		p.cache.Close()
	}

	// Close security validator to release its cache
	if p.securityValidator != nil {
		p.securityValidator.Close()
	}

	// Reset resource tracking
	if p.resources != nil {
		atomic.StoreInt32(&p.resources.memoryPressure, 0)
		atomic.StoreInt64(&p.resources.lastMemoryCheck, 0)
		atomic.StoreInt64(&p.resources.lastPoolReset, 0)
	}

	// Release hook references to allow GC of captured closures
	p.hooksMu.Lock()
	p.hooks = nil
	p.hasHooks.Store(false)
	p.hooksMu.Unlock()

	// NOTE: Global caches (pathTypeCache, structEncoderCache) are NOT cleared
	// here because they are shared across ALL processor instances. Clearing them
	// in individual Close() would invalidate caches for other active processors.
	// Use ShutdownGlobalProcessor() for complete cleanup at application shutdown.

	// Resources fully released.
	atomic.StoreInt32(&p.state, processorStateClosed)
}

// waitForActiveOps blocks until all in-flight operations (registered via
// beginGovernedOp) have completed, or the timeout elapses. Returns true if all
// operations finished before the deadline. Close() uses this to drain work
// without contending with the concurrency semaphore.
func (p *Processor) waitForActiveOps(timeout time.Duration) bool {
	if atomic.LoadInt64(&p.activeOps) <= 0 {
		return true
	}
	ticker := time.NewTicker(2 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for {
		if atomic.LoadInt64(&p.activeOps) <= 0 {
			return true
		}
		select {
		case <-ticker.C:
		case <-deadline.C:
			return atomic.LoadInt64(&p.activeOps) <= 0
		}
	}
}

// IsClosed returns true if the processor has been closed or close timed out.
// In both states the processor should not accept new operations.
//
// The closing (drain) state also reports true: checkClosed rejects new
// operations during that window, so reporting false here would let the
// config-processor registry hand out a processor whose every call fails.
func (p *Processor) IsClosed() bool {
	if p == nil {
		return true
	}
	return atomic.LoadInt32(&p.state) != processorStateActive
}

// AddHook adds an operation hook to the processor.
// Hooks are called before and after each operation.
// Multiple hooks can be added and are executed in order (Before) and reverse order (After).
//
// Example:
//
//	type LoggingHook struct{}
//	func (h *LoggingHook) Before(ctx json.HookContext) error {
//	    log.Printf("before %s", ctx.Operation)
//	    return nil
//	}
//	func (h *LoggingHook) After(ctx json.HookContext, result any, err error) (any, error) {
//	    log.Printf("after %s", ctx.Operation)
//	    return result, err
//	}
//
//	processor, err := json.New()
//	if err != nil {
//	    return err
//	}
//	defer processor.Close()
//	processor.AddHook(&LoggingHook{})
func (p *Processor) AddHook(hook Hook) {
	if p == nil {
		return
	}
	p.hooksMu.Lock()
	newHooks := make([]Hook, len(p.hooks)+1)
	copy(newHooks, p.hooks)
	newHooks[len(p.hooks)] = hook
	p.hooks = newHooks
	// Set the gate under the lock so any reader that observes hasHooks==true
	// also observes the populated p.hooks slice (happens-before via the mutex).
	p.hasHooks.Store(true)
	p.hooksMu.Unlock()
}

// snapshotHooks returns the processor's current hooks as a hookChain that is
// safe to iterate after the lock is released. AddHook and Close both replace
// p.hooks with a freshly allocated slice (never appending into the existing
// backing array), so the slice header captured here keeps pointing at an array
// that will never be mutated for the rest of the operation's lifetime.
//
// This is the read-side counterpart to AddHook: operations call it once near
// their entry to decide whether hooks are present, then run executeBefore /
// executeAfter against the stable snapshot.
//
// PERFORMANCE: The common case is a processor with no hooks registered. The
// hasHooks atomic gate lets that case return nil with a single atomic load —
// no mutex — so concurrent operations never contend on hooksMu. The lock is
// only taken when hooks are actually present. See hasHooks field comment.
func (p *Processor) snapshotHooks() hookChain {
	if p == nil || !p.hasHooks.Load() {
		return nil
	}
	p.hooksMu.Lock()
	hc := hookChain(p.hooks)
	p.hooksMu.Unlock()
	return hc
}

// delegateForPerCallParser builds a one-shot processor for a per-call
// Config.CustomPathParser (D-002/M33). The path-parsing funnels read the
// PROCESSOR-level parser only, so a parser arriving solely through a call's
// cfg would be ignored; delegating the whole operation to a processor built
// from that cfg honors it with the same semantics the package-level API
// already guarantees via its cache bypass. Returns (nil, nil) when no
// delegation is needed. Callers own the returned processor's Close (safe and
// immediate: it never has in-flight ops of its own).
func (p *Processor) delegateForPerCallParser(options *Config) (*Processor, error) {
	if options == nil || options.CustomPathParser == nil || p.config.CustomPathParser != nil {
		return nil, nil
	}
	return New(*options.Clone())
}

// hooksForOptions returns the hook chain for an operation: the processor's
// installed hooks plus any hooks supplied through the per-call Config.
//
// D-002: cfg.Hooks always take effect via New (they are installed as
// processor hooks), so package-level calls built from cfg run them — but the
// per-call form p.Get(s, path, cfgWithHooks) silently ignored the same field
// while it still participated in config hashing and cache keys. Honoring
// per-call hooks here makes both layers behave identically.
func (p *Processor) hooksForOptions(options *Config) hookChain {
	hc := p.snapshotHooks()
	if options == nil || len(options.Hooks) == 0 {
		return hc
	}
	merged := make(hookChain, 0, len(hc)+len(options.Hooks))
	merged = append(merged, hc...)
	merged = append(merged, options.Hooks...)
	return merged
}

// GetConfig returns a copy of the processor configuration
func (p *Processor) GetConfig() Config {
	if p == nil {
		return Config{}
	}
	return *p.config.Clone()
}

// SetLogger sets a custom structured logger for the processor
func (p *Processor) SetLogger(logger *slog.Logger) {
	if p == nil {
		return
	}
	if logger != nil {
		p.logger.Store(logger.With("component", "json-processor"))
	} else {
		p.logger.Store(slog.Default().With("component", "json-processor"))
	}
}

// getLogger safely retrieves the current logger (thread-safe).
// Returns slog.Default() when called on a nil Processor.
func (p *Processor) getLogger() *slog.Logger {
	if p == nil {
		return slog.Default().With("component", "json-processor")
	}
	if l, ok := p.logger.Load().(*slog.Logger); ok {
		return l
	}
	return slog.Default().With("component", "json-processor")
}
