package json

import (
	"context"
	"sync"
	"time"
)

// GoroutineManager manages goroutines to prevent leaks
type GoroutineManager struct {
	wg            sync.WaitGroup
	ctx           context.Context
	cancel        context.CancelFunc
	active        int64
	maxGoroutines int64
}

// NewGoroutineManager creates a new goroutine manager
func NewGoroutineManager(maxGoroutines int) *GoroutineManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &GoroutineManager{
		ctx:           ctx,
		cancel:        cancel,
		maxGoroutines: int64(maxGoroutines),
	}
}

// Go safely starts a goroutine with tracking
func (gm *GoroutineManager) Go(f func(ctx context.Context)) error {
	// Check if we're at capacity
	if gm.active >= gm.maxGoroutines {
		return &JsonsError{
			Op:      "goroutine_manager",
			Message: "maximum goroutines reached",
			Err:     ErrResourceExhausted,
		}
	}

	gm.wg.Add(1)
	gm.active++

	go func() {
		defer func() {
			gm.wg.Done()
			gm.active--

			// Recover from panics
			if r := recover(); r != nil {
				// Log panic (in production, use proper logging)
				_ = r
			}
		}()

		f(gm.ctx)
	}()

	return nil
}

// Wait waits for all goroutines to complete
func (gm *GoroutineManager) Wait() {
	gm.wg.Wait()
}

// WaitWithTimeout waits for all goroutines with a timeout
func (gm *GoroutineManager) WaitWithTimeout(timeout time.Duration) bool {
	done := make(chan struct{})
	go func() {
		gm.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}

// Shutdown gracefully shuts down all goroutines
func (gm *GoroutineManager) Shutdown(timeout time.Duration) error {
	// Cancel context to signal all goroutines
	gm.cancel()

	// Wait with timeout
	if !gm.WaitWithTimeout(timeout) {
		return &JsonsError{
			Op:      "goroutine_manager",
			Message: "timeout waiting for goroutines to finish",
			Err:     ErrOperationTimeout,
		}
	}

	return nil
}

// GetActiveCount returns the number of active goroutines
func (gm *GoroutineManager) GetActiveCount() int64 {
	return gm.active
}

// SafeGo is a helper function to safely start a goroutine with panic recovery
func SafeGo(f func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// Log panic (in production, use proper logging)
				_ = r
			}
		}()
		f()
	}()
}

// SafeGoWithContext is a helper function to safely start a goroutine with context
func SafeGoWithContext(ctx context.Context, f func(context.Context)) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// Log panic (in production, use proper logging)
				_ = r
			}
		}()
		f(ctx)
	}()
}
