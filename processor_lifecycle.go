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
		// finish against undisturbed state. cleanupOnce prevents a later Close()
		// from re-entering, so full teardown waits until the processor is idle or
		// process exit / ShutdownGlobalProcessor. Correctness over the previous
		// unconditional teardown, which raced in-flight ops against cache teardown.
		if !p.waitForActiveOps(closeOperationTimeout) {
			atomic.StoreInt32(&p.state, processorStateCloseTimedOut)
			return
		}

		// All in-flight operations have drained — safe to release resources.

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
		p.hooksMu.Unlock()

		// NOTE: Global caches (pathTypeCache, structEncoderCache) are NOT cleared
		// here because they are shared across ALL processor instances. Clearing them
		// in individual Close() would invalidate caches for other active processors.
		// Use ShutdownGlobalProcessor() for complete cleanup at application shutdown.

		// Resources fully released.
		atomic.StoreInt32(&p.state, processorStateClosed)
	})

	return nil
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
func (p *Processor) IsClosed() bool {
	if p == nil {
		return true
	}
	state := atomic.LoadInt32(&p.state)
	return state == processorStateClosed || state == processorStateCloseTimedOut
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
	p.hooksMu.Unlock()
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
