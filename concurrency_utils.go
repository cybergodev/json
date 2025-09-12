package json

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// ConcurrencyManager manages concurrent operations with enhanced safety
type ConcurrencyManager struct {
	// Concurrency limits
	maxConcurrency    int32
	currentOperations int64
	
	// Rate limiting
	operationsPerSecond int64
	lastResetTime       int64
	currentSecondOps    int64
	
	// Circuit breaker
	failureCount    int64
	lastFailureTime int64
	circuitOpen     int32 // 0=closed, 1=open
	
	// Deadlock detection
	operationTimeouts map[uint64]int64 // goroutine ID -> start time
	timeoutMutex      sync.RWMutex
	
	// Performance tracking
	totalOperations   int64
	totalWaitTime     int64
	averageWaitTime   int64
}

// NewConcurrencyManager creates a new concurrency manager
func NewConcurrencyManager(maxConcurrency int, operationsPerSecond int) *ConcurrencyManager {
	return &ConcurrencyManager{
		maxConcurrency:      int32(maxConcurrency),
		operationsPerSecond: int64(operationsPerSecond),
		lastResetTime:       time.Now().Unix(),
		operationTimeouts:   make(map[uint64]int64),
	}
}

// ExecuteWithConcurrencyControl executes a function with concurrency control
func (cm *ConcurrencyManager) ExecuteWithConcurrencyControl(
	ctx context.Context,
	operation func() error,
	timeout time.Duration,
) error {
	// Check circuit breaker
	if atomic.LoadInt32(&cm.circuitOpen) == 1 {
		// Check if we should try to close the circuit
		lastFailure := atomic.LoadInt64(&cm.lastFailureTime)
		if time.Now().Unix()-lastFailure > 60 { // 1 minute cooldown
			atomic.StoreInt32(&cm.circuitOpen, 0)
			atomic.StoreInt64(&cm.failureCount, 0)
		} else {
			return &JsonsError{
				Op:      "concurrency_control",
				Message: "circuit breaker is open",
				Err:     ErrOperationFailed,
			}
		}
	}
	
	// Rate limiting check
	if err := cm.checkRateLimit(); err != nil {
		return err
	}
	
	// Acquire concurrency slot
	start := time.Now()
	if err := cm.acquireSlot(ctx, timeout); err != nil {
		return err
	}
	defer cm.releaseSlot()
	
	// Record wait time
	waitTime := time.Since(start).Nanoseconds()
	atomic.AddInt64(&cm.totalWaitTime, waitTime)
	atomic.AddInt64(&cm.totalOperations, 1)
	
	// Update average wait time using exponential moving average
	for {
		oldAvg := atomic.LoadInt64(&cm.averageWaitTime)
		newAvg := oldAvg + (waitTime-oldAvg)/10 // Alpha = 0.1
		if atomic.CompareAndSwapInt64(&cm.averageWaitTime, oldAvg, newAvg) {
			break
		}
	}
	
	// Register operation for deadlock detection
	gid := getGoroutineIDForConcurrency()
	if gid != 0 {
		cm.timeoutMutex.Lock()
		cm.operationTimeouts[gid] = time.Now().UnixNano()
		cm.timeoutMutex.Unlock()
		
		defer func() {
			cm.timeoutMutex.Lock()
			delete(cm.operationTimeouts, gid)
			cm.timeoutMutex.Unlock()
		}()
	}
	
	// Execute operation with timeout
	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- &JsonsError{
					Op:      "concurrency_control",
					Message: "operation panicked",
					Err:     ErrOperationFailed,
				}
			}
		}()
		done <- operation()
	}()
	
	select {
	case err := <-done:
		if err != nil {
			cm.recordFailure()
		}
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(timeout):
		cm.recordFailure()
		return &JsonsError{
			Op:      "concurrency_control",
			Message: "operation timeout",
			Err:     ErrOperationTimeout,
		}
	}
}

// checkRateLimit checks if the operation is within rate limits
func (cm *ConcurrencyManager) checkRateLimit() error {
	if cm.operationsPerSecond <= 0 {
		return nil // No rate limiting
	}
	
	now := time.Now().Unix()
	lastReset := atomic.LoadInt64(&cm.lastResetTime)
	
	// Reset counter if we're in a new second
	if now > lastReset {
		if atomic.CompareAndSwapInt64(&cm.lastResetTime, lastReset, now) {
			atomic.StoreInt64(&cm.currentSecondOps, 0)
		}
	}
	
	// Check if we're within limits
	current := atomic.AddInt64(&cm.currentSecondOps, 1)
	if current > cm.operationsPerSecond {
		atomic.AddInt64(&cm.currentSecondOps, -1) // Rollback
		return &JsonsError{
			Op:      "rate_limit",
			Message: "rate limit exceeded",
			Err:     ErrRateLimitExceeded,
		}
	}
	
	return nil
}

// acquireSlot acquires a concurrency slot
func (cm *ConcurrencyManager) acquireSlot(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	
	for {
		current := atomic.LoadInt64(&cm.currentOperations)
		if current >= int64(atomic.LoadInt32(&cm.maxConcurrency)) {
			// Wait a bit and retry
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Millisecond):
				if time.Now().After(deadline) {
					return &JsonsError{
						Op:      "acquire_slot",
						Message: "timeout acquiring concurrency slot",
						Err:     ErrOperationTimeout,
					}
				}
				continue
			}
		}
		
		// Try to increment
		if atomic.CompareAndSwapInt64(&cm.currentOperations, current, current+1) {
			return nil
		}
	}
}

// releaseSlot releases a concurrency slot
func (cm *ConcurrencyManager) releaseSlot() {
	atomic.AddInt64(&cm.currentOperations, -1)
}

// recordFailure records a failure for circuit breaker
func (cm *ConcurrencyManager) recordFailure() {
	failures := atomic.AddInt64(&cm.failureCount, 1)
	atomic.StoreInt64(&cm.lastFailureTime, time.Now().Unix())
	
	// Open circuit if too many failures
	if failures >= 10 { // Threshold: 10 failures
		atomic.StoreInt32(&cm.circuitOpen, 1)
	}
}

// GetStats returns concurrency statistics
func (cm *ConcurrencyManager) GetStats() ConcurrencyStats {
	return ConcurrencyStats{
		MaxConcurrency:      int(atomic.LoadInt32(&cm.maxConcurrency)),
		CurrentOperations:   atomic.LoadInt64(&cm.currentOperations),
		TotalOperations:     atomic.LoadInt64(&cm.totalOperations),
		AverageWaitTime:     time.Duration(atomic.LoadInt64(&cm.averageWaitTime)),
		CircuitOpen:         atomic.LoadInt32(&cm.circuitOpen) == 1,
		FailureCount:        atomic.LoadInt64(&cm.failureCount),
		OperationsPerSecond: atomic.LoadInt64(&cm.operationsPerSecond),
	}
}

// ConcurrencyStats represents concurrency statistics
type ConcurrencyStats struct {
	MaxConcurrency      int
	CurrentOperations   int64
	TotalOperations     int64
	AverageWaitTime     time.Duration
	CircuitOpen         bool
	FailureCount        int64
	OperationsPerSecond int64
}

// DetectDeadlocks detects potential deadlocks
func (cm *ConcurrencyManager) DetectDeadlocks() []DeadlockInfo {
	cm.timeoutMutex.RLock()
	defer cm.timeoutMutex.RUnlock()
	
	var deadlocks []DeadlockInfo
	now := time.Now().UnixNano()
	threshold := int64(30 * time.Second) // 30 second threshold
	
	for gid, startTime := range cm.operationTimeouts {
		if now-startTime > threshold {
			deadlocks = append(deadlocks, DeadlockInfo{
				GoroutineID: gid,
				StartTime:   time.Unix(0, startTime),
				Duration:    time.Duration(now - startTime),
			})
		}
	}
	
	return deadlocks
}

// DeadlockInfo represents information about a potential deadlock
type DeadlockInfo struct {
	GoroutineID uint64
	StartTime   time.Time
	Duration    time.Duration
}

// getGoroutineIDForConcurrency returns the current goroutine ID for concurrency tracking
func getGoroutineIDForConcurrency() uint64 {
	// This is a simplified implementation
	// In production, you might want to use a more robust method
	return uint64(runtime.NumGoroutine())
}
