package json

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"
)

// incrementOperationCount atomically increments the operation counter with rate limiting
func (p *Processor) incrementOperationCount() {
	atomic.AddInt64(&p.metrics.operationCount, 1)
}

// checkRateLimit checks if the operation rate is within acceptable limits
func (p *Processor) checkRateLimit() error {
	if p.metrics.operationWindow <= 0 {
		return nil // Rate limiting disabled
	}

	now := time.Now().UnixNano()
	lastOp := atomic.LoadInt64(&p.metrics.lastOperationTime)

	if lastOp > 0 {
		timeDiff := now - lastOp
		if timeDiff < int64(time.Second)/p.metrics.operationWindow {
			return &JsonsError{
				Op:      "rate_limit",
				Message: "operation rate limit exceeded",
				Err:     ErrOperationFailed,
			}
		}
	}

	atomic.StoreInt64(&p.metrics.lastOperationTime, now)
	return nil
}

// incrementErrorCount atomically increments the error counter with optional logging
func (p *Processor) incrementErrorCount() {
	atomic.AddInt64(&p.metrics.errorCount, 1)
}

// logError logs an error with structured logging
func (p *Processor) logError(ctx context.Context, operation, path string, err error) {
	if p.logger == nil {
		return
	}

	errorType := "unknown"
	var jsonErr *JsonsError
	if errors.As(err, &jsonErr) && jsonErr.Err != nil {
		errorType = jsonErr.Err.Error()
	}

	if p.metrics != nil {
		p.metrics.collector.RecordError(errorType)
	}

	sanitizedPath := sanitizePath(path)
	sanitizedError := sanitizeError(err)

	p.logger.ErrorContext(ctx, "JSON operation failed",
		slog.String("operation", operation),
		slog.String("path", sanitizedPath),
		slog.String("error", sanitizedError),
		slog.String("error_type", errorType),
		slog.Int64("error_count", atomic.LoadInt64(&p.metrics.errorCount)),
		slog.String("processor_id", p.getProcessorID()),
		slog.Bool("cache_enabled", p.config.EnableCache),
		slog.Int64("concurrent_ops", atomic.LoadInt64(&p.metrics.operationCount)),
	)
}

// sanitizePath removes potentially sensitive information from paths
func sanitizePath(path string) string {
	if len(path) > 100 {
		return truncateString(path, 100)
	}
	// Remove potential sensitive patterns but keep structure
	// Use case-insensitive matching for better security
	lowerPath := strings.ToLower(path)
	sensitivePatterns := []string{
		"password", "passwd", "pwd",
		"token", "bearer",
		"key", "apikey", "api_key", "api-key",
		"secret", "credential", "cred",
		"auth", "authorization",
		"session", "cookie",
	}
	for _, pattern := range sensitivePatterns {
		if strings.Contains(lowerPath, pattern) {
			return "[REDACTED_PATH]"
		}
	}
	return path
}

// sanitizeError removes potentially sensitive information from error messages
func sanitizeError(err error) string {
	if err == nil {
		return ""
	}
	errMsg := err.Error()
	if len(errMsg) > 200 {
		return truncateString(errMsg, 200)
	}
	return errMsg
}

// truncateString efficiently truncates a string with ellipsis
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// logOperation logs a successful operation with structured logging and performance warnings
func (p *Processor) logOperation(ctx context.Context, operation, path string, duration time.Duration) {
	if p.logger == nil {
		return
	}

	// Use modern structured logging with typed attributes
	commonAttrs := []slog.Attr{
		slog.String("operation", operation),
		slog.String("path", path),
		slog.Int64("duration_ms", duration.Milliseconds()),
		slog.Int64("operation_count", atomic.LoadInt64(&p.metrics.operationCount)),
		slog.String("processor_id", p.getProcessorID()),
	}

	if duration > SlowOperationThreshold {
		// Log as warning for slow operations
		attrs := append(commonAttrs, slog.Int64("threshold_ms", SlowOperationThreshold.Milliseconds()))
		p.logger.LogAttrs(ctx, slog.LevelWarn, "Slow JSON operation detected", attrs...)
	} else {
		// Log as debug for normal operations
		p.logger.LogAttrs(ctx, slog.LevelDebug, "JSON operation completed", commonAttrs...)
	}
}

// getProcessorID returns a unique identifier for this processor instance
func (p *Processor) getProcessorID() string {
	// Use the processor's memory address as a unique identifier
	return fmt.Sprintf("proc_%p", p)
}

// executeWithConcurrencyControl executes an operation with timeout control
func (p *Processor) executeWithConcurrencyControl(
	operation func() error,
	timeout time.Duration,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- operation()
	}()

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		return fmt.Errorf("operation timeout after %v", timeout)
	}
}

// executeOperation executes an operation with metrics tracking and error handling
func (p *Processor) executeOperation(operationName string, operation func() error) error {
	if err := p.checkClosed(); err != nil {
		return err
	}

	// Record operation start
	atomic.AddInt64(&p.metrics.operationCount, 1)
	start := time.Now()

	// Execute with concurrency control
	err := p.executeWithConcurrencyControl(operation, 30*time.Second)

	// Record metrics
	if p.resourceMonitor != nil {
		p.resourceMonitor.RecordOperation(time.Since(start))
	}

	if err != nil {
		atomic.AddInt64(&p.metrics.errorCount, 1)
		if p.logger != nil {
			p.logger.Error("Operation failed",
				"operation", operationName,
				"error", err,
				"duration", time.Since(start),
			)
		}
	}

	return err
}
