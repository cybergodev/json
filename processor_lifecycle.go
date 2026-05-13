package json

import (
	"context"
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
		// Mark as closing to prevent new operations
		atomic.StoreInt32(&p.state, processorStateClosing)

		// Drain the concurrency semaphore to release any waiting goroutines
		// Use context cancellation for clean goroutine termination
		if p.metrics != nil && p.metrics.concurrencySemaphore != nil {
			drainCtx, drainCancel := context.WithTimeout(context.Background(), semaphoreDrainTimeout)
			drainDone := make(chan struct{})
			go func() {
				defer close(drainDone)
				for {
					select {
					case <-drainCtx.Done():
						// Context cancelled - exit cleanly
						return
					case <-p.metrics.concurrencySemaphore:
						// Drained one slot
					default:
						// Semaphore is empty
						return
					}
				}
			}()

			select {
			case <-drainDone:
				// Drain completed - goroutine exited cleanly
				drainCancel()
			case <-drainCtx.Done():
				// Timeout on drain - cancel context to signal goroutine to exit,
				// then wait briefly for it to acknowledge
				drainCancel()
				select {
				case <-drainDone:
					// Goroutine exited after context cancellation
				case <-time.After(100 * time.Millisecond):
					// Goroutine still running - mark as timed-out so IsClosed() returns true.
					// Operations will fail fast instead of retrying a permanently closing processor.
					atomic.StoreInt32(&p.state, processorStateCloseTimedOut)
					return
				}
			}
		}

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

		// Mark as fully closed
		atomic.StoreInt32(&p.state, processorStateClosed)
	})

	return nil
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
