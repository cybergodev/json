package json

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================================
// DOS Protection
// ============================================================================

// DOSProtection provides protection against Denial of Service attacks
type DOSProtection struct {
	// Rate limiting
	requestsPerSecond int64
	currentSecond     int64
	requestCount      int64

	// Request size limiting
	maxRequestSize int64

	// Depth limiting
	maxDepth int

	// Concurrent request limiting
	maxConcurrent int64
	currentActive int64

	// IP-based rate limiting (simplified)
	ipLimits sync.Map // map[string]*ipLimit

	// Circuit breaker
	failureCount     int64
	failureThreshold int64
	circuitOpen      int32
	lastFailure      int64
	resetTimeout     time.Duration
}

type ipLimit struct {
	count      int64
	lastReset  int64
	blocked    bool
	blockUntil int64
}

// NewDOSProtection creates a new DOS protection instance
func NewDOSProtection(config *DOSConfig) *DOSProtection {
	if config == nil {
		config = DefaultDOSConfig()
	}

	return &DOSProtection{
		requestsPerSecond: config.RequestsPerSecond,
		maxRequestSize:    config.MaxRequestSize,
		maxDepth:          config.MaxDepth,
		maxConcurrent:     config.MaxConcurrent,
		failureThreshold:  config.FailureThreshold,
		resetTimeout:      config.ResetTimeout,
	}
}

// DOSConfig holds DOS protection configuration
type DOSConfig struct {
	RequestsPerSecond int64
	MaxRequestSize    int64
	MaxDepth          int
	MaxConcurrent     int64
	FailureThreshold  int64
	ResetTimeout      time.Duration
}

// DefaultDOSConfig returns default DOS protection configuration
func DefaultDOSConfig() *DOSConfig {
	return &DOSConfig{
		RequestsPerSecond: 1000,
		MaxRequestSize:    10 * 1024 * 1024, // 10MB
		MaxDepth:          32,
		MaxConcurrent:     100,
		FailureThreshold:  10,
		ResetTimeout:      60 * time.Second,
	}
}

// CheckRequest checks if a request should be allowed
func (dp *DOSProtection) CheckRequest(size int64, clientID string) error {
	// Check circuit breaker
	if atomic.LoadInt32(&dp.circuitOpen) == 1 {
		lastFailure := atomic.LoadInt64(&dp.lastFailure)
		if time.Now().Unix()-lastFailure < int64(dp.resetTimeout.Seconds()) {
			return &JsonsError{
				Op:      "dos_protection",
				Message: "circuit breaker is open",
				Err:     ErrCircuitOpen,
			}
		}
		// Reset circuit breaker
		atomic.StoreInt32(&dp.circuitOpen, 0)
		atomic.StoreInt64(&dp.failureCount, 0)
	}

	// Check request size
	if size > dp.maxRequestSize {
		dp.recordFailure()
		return &JsonsError{
			Op:      "dos_protection",
			Message: fmt.Sprintf("request size %d exceeds limit %d", size, dp.maxRequestSize),
			Err:     ErrSizeLimit,
		}
	}

	// Check rate limit
	if err := dp.checkRateLimit(); err != nil {
		dp.recordFailure()
		return err
	}

	// Check concurrent requests
	if err := dp.checkConcurrentLimit(); err != nil {
		dp.recordFailure()
		return err
	}

	// Check IP-based rate limit
	if clientID != "" {
		if err := dp.checkIPLimit(clientID); err != nil {
			return err
		}
	}

	return nil
}

// checkRateLimit checks global rate limit
func (dp *DOSProtection) checkRateLimit() error {
	now := time.Now().Unix()
	currentSecond := atomic.LoadInt64(&dp.currentSecond)

	if now != currentSecond {
		// New second, reset counter
		if atomic.CompareAndSwapInt64(&dp.currentSecond, currentSecond, now) {
			atomic.StoreInt64(&dp.requestCount, 0)
		}
	}

	count := atomic.AddInt64(&dp.requestCount, 1)
	if count > dp.requestsPerSecond {
		atomic.AddInt64(&dp.requestCount, -1) // Rollback
		return &JsonsError{
			Op:      "dos_protection",
			Message: "rate limit exceeded",
			Err:     ErrRateLimitExceeded,
		}
	}

	return nil
}

// checkConcurrentLimit checks concurrent request limit
func (dp *DOSProtection) checkConcurrentLimit() error {
	current := atomic.LoadInt64(&dp.currentActive)
	if current >= dp.maxConcurrent {
		return &JsonsError{
			Op:      "dos_protection",
			Message: fmt.Sprintf("concurrent request limit %d exceeded", dp.maxConcurrent),
			Err:     ErrConcurrencyLimit,
		}
	}

	atomic.AddInt64(&dp.currentActive, 1)
	return nil
}

// checkIPLimit checks per-IP rate limit
func (dp *DOSProtection) checkIPLimit(clientID string) error {
	now := time.Now().Unix()

	limitVal, _ := dp.ipLimits.LoadOrStore(clientID, &ipLimit{
		lastReset: now,
	})

	limit := limitVal.(*ipLimit)

	// Check if blocked
	if limit.blocked && now < limit.blockUntil {
		return &JsonsError{
			Op:      "dos_protection",
			Message: "client temporarily blocked",
			Err:     ErrRateLimitExceeded,
		}
	}

	// Reset if new second
	if now != limit.lastReset {
		atomic.StoreInt64(&limit.count, 0)
		atomic.StoreInt64(&limit.lastReset, now)
		limit.blocked = false
	}

	// Increment and check
	count := atomic.AddInt64(&limit.count, 1)
	if count > dp.requestsPerSecond/10 { // Per-IP limit is 10% of global
		limit.blocked = true
		limit.blockUntil = now + 60 // Block for 60 seconds
		return &JsonsError{
			Op:      "dos_protection",
			Message: "per-client rate limit exceeded",
			Err:     ErrRateLimitExceeded,
		}
	}

	return nil
}

// ReleaseRequest releases a concurrent request slot
func (dp *DOSProtection) ReleaseRequest() {
	atomic.AddInt64(&dp.currentActive, -1)
}

// recordFailure records a failure for circuit breaker
func (dp *DOSProtection) recordFailure() {
	failures := atomic.AddInt64(&dp.failureCount, 1)
	atomic.StoreInt64(&dp.lastFailure, time.Now().Unix())

	if failures >= dp.failureThreshold {
		atomic.StoreInt32(&dp.circuitOpen, 1)
	}
}

// ValidateDepth validates JSON nesting depth
func (dp *DOSProtection) ValidateDepth(jsonStr string) error {
	depth := 0
	maxDepth := 0

	for _, char := range jsonStr {
		switch char {
		case '{', '[':
			depth++
			if depth > maxDepth {
				maxDepth = depth
			}
			if depth > dp.maxDepth {
				return &JsonsError{
					Op:      "dos_protection",
					Message: fmt.Sprintf("JSON depth %d exceeds limit %d", depth, dp.maxDepth),
					Err:     ErrDepthLimit,
				}
			}
		case '}', ']':
			depth--
		}
	}

	return nil
}

// GetStats returns DOS protection statistics
func (dp *DOSProtection) GetStats() DOSStats {
	return DOSStats{
		RequestsPerSecond: dp.requestsPerSecond,
		CurrentActive:     atomic.LoadInt64(&dp.currentActive),
		MaxConcurrent:     dp.maxConcurrent,
		FailureCount:      atomic.LoadInt64(&dp.failureCount),
		CircuitOpen:       atomic.LoadInt32(&dp.circuitOpen) == 1,
	}
}

// DOSStats represents DOS protection statistics
type DOSStats struct {
	RequestsPerSecond int64
	CurrentActive     int64
	MaxConcurrent     int64
	FailureCount      int64
	CircuitOpen       bool
}

// ============================================================================
// Goroutine Safety
// ============================================================================

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
