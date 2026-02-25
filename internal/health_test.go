package internal

import (
	"strings"
	"testing"
	"time"
)

// ============================================================================
// NewHealthChecker TESTS
// ============================================================================

func TestNewHealthChecker_DefaultConfig(t *testing.T) {
	metrics := NewMetricsCollector()
	hc := NewHealthChecker(metrics, nil)

	if hc == nil {
		t.Fatal("NewHealthChecker returned nil")
	}
	if hc.metrics != metrics {
		t.Error("metrics not set correctly")
	}
	if hc.maxMemoryBytes != defaultMaxMemoryBytes {
		t.Errorf("expected default maxMemoryBytes %d, got %d", defaultMaxMemoryBytes, hc.maxMemoryBytes)
	}
	if hc.maxErrorRatePercent != defaultMaxErrorRatePercent {
		t.Errorf("expected default maxErrorRatePercent %.1f, got %.1f", defaultMaxErrorRatePercent, hc.maxErrorRatePercent)
	}
}

func TestNewHealthChecker_CustomConfig(t *testing.T) {
	metrics := NewMetricsCollector()
	config := &HealthCheckerConfig{
		MaxMemoryBytes:      512 * 1024 * 1024, // 512MB
		MaxErrorRatePercent: 5.0,
	}
	hc := NewHealthChecker(metrics, config)

	if hc.maxMemoryBytes != 512*1024*1024 {
		t.Errorf("expected maxMemoryBytes 512MB, got %d", hc.maxMemoryBytes)
	}
	if hc.maxErrorRatePercent != 5.0 {
		t.Errorf("expected maxErrorRatePercent 5.0, got %.1f", hc.maxErrorRatePercent)
	}
}

func TestNewHealthChecker_NilMetrics(t *testing.T) {
	hc := NewHealthChecker(nil, nil)

	if hc == nil {
		t.Fatal("NewHealthChecker returned nil")
	}
	if hc.metrics != nil {
		t.Error("expected nil metrics")
	}
}

func TestNewHealthChecker_PartialConfig(t *testing.T) {
	metrics := NewMetricsCollector()

	tests := []struct {
		name              string
		config            *HealthCheckerConfig
		expectedMemory    uint64
		expectedErrorRate float64
	}{
		{
			name:              "only MaxMemoryBytes",
			config:            &HealthCheckerConfig{MaxMemoryBytes: 256 * 1024 * 1024},
			expectedMemory:    256 * 1024 * 1024,
			expectedErrorRate: defaultMaxErrorRatePercent,
		},
		{
			name:              "only MaxErrorRatePercent",
			config:            &HealthCheckerConfig{MaxErrorRatePercent: 20.0},
			expectedMemory:    defaultMaxMemoryBytes,
			expectedErrorRate: 20.0,
		},
		{
			name:              "zero MaxMemoryBytes (should use default)",
			config:            &HealthCheckerConfig{MaxMemoryBytes: 0, MaxErrorRatePercent: 15.0},
			expectedMemory:    defaultMaxMemoryBytes,
			expectedErrorRate: 15.0,
		},
		{
			name:              "zero MaxErrorRatePercent (should use default)",
			config:            &HealthCheckerConfig{MaxMemoryBytes: 100 * 1024 * 1024, MaxErrorRatePercent: 0},
			expectedMemory:    100 * 1024 * 1024,
			expectedErrorRate: defaultMaxErrorRatePercent,
		},
		{
			name:              "MaxErrorRatePercent over 100 (should use default)",
			config:            &HealthCheckerConfig{MaxErrorRatePercent: 150.0},
			expectedMemory:    defaultMaxMemoryBytes,
			expectedErrorRate: defaultMaxErrorRatePercent,
		},
		{
			name:              "negative MaxErrorRatePercent (should use default)",
			config:            &HealthCheckerConfig{MaxErrorRatePercent: -5.0},
			expectedMemory:    defaultMaxMemoryBytes,
			expectedErrorRate: defaultMaxErrorRatePercent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hc := NewHealthChecker(metrics, tt.config)
			if hc.maxMemoryBytes != tt.expectedMemory {
				t.Errorf("expected maxMemoryBytes %d, got %d", tt.expectedMemory, hc.maxMemoryBytes)
			}
			if hc.maxErrorRatePercent != tt.expectedErrorRate {
				t.Errorf("expected maxErrorRatePercent %.1f, got %.1f", tt.expectedErrorRate, hc.maxErrorRatePercent)
			}
		})
	}
}

// ============================================================================
// CheckHealth TESTS
// ============================================================================

func TestCheckHealth_AllHealthy(t *testing.T) {
	metrics := NewMetricsCollector()
	// Record some successful operations
	metrics.RecordOperation(100*time.Millisecond, true, 1024)
	metrics.RecordOperation(50*time.Millisecond, true, 512)

	hc := NewHealthChecker(metrics, nil)
	status := hc.CheckHealth()

	if !status.Healthy {
		t.Error("expected overall health to be true")
	}
	if status.Timestamp.IsZero() {
		t.Error("timestamp should not be zero")
	}
	if len(status.Checks) != 3 {
		t.Errorf("expected 3 checks, got %d", len(status.Checks))
	}

	for name, check := range status.Checks {
		if !check.Healthy {
			t.Errorf("check %q should be healthy: %s", name, check.Message)
		}
	}
}

func TestCheckHealth_NilMetrics(t *testing.T) {
	hc := NewHealthChecker(nil, nil)
	status := hc.CheckHealth()

	// Should be unhealthy because metrics is nil
	if status.Healthy {
		t.Error("expected overall health to be false with nil metrics")
	}

	// metrics and error_rate checks should fail
	if status.Checks["metrics"].Healthy {
		t.Error("metrics check should fail with nil metrics")
	}
	if status.Checks["error_rate"].Healthy {
		t.Error("error_rate check should fail with nil metrics")
	}
}

func TestCheckHealth_HighErrorRate(t *testing.T) {
	metrics := NewMetricsCollector()
	// Record operations with high failure rate (>10%)
	for i := 0; i < 20; i++ {
		metrics.RecordOperation(10*time.Millisecond, false, 0) // failures
	}
	for i := 0; i < 10; i++ {
		metrics.RecordOperation(10*time.Millisecond, true, 0) // successes
	}

	config := &HealthCheckerConfig{MaxErrorRatePercent: 10.0}
	hc := NewHealthChecker(metrics, config)
	status := hc.CheckHealth()

	// Should be unhealthy due to high error rate (66.67%)
	if status.Healthy {
		t.Error("expected overall health to be false with high error rate")
	}
	if status.Checks["error_rate"].Healthy {
		t.Error("error_rate check should fail with high error rate")
	}
}

// ============================================================================
// checkMetrics TESTS
// ============================================================================

func TestCheckMetrics_NilMetrics(t *testing.T) {
	hc := NewHealthChecker(nil, nil)
	result := hc.checkMetrics()

	if result.Healthy {
		t.Error("expected unhealthy with nil metrics")
	}
	if !strings.Contains(result.Message, "not initialized") {
		t.Errorf("unexpected message: %s", result.Message)
	}
}

func TestCheckMetrics_ValidMetrics(t *testing.T) {
	metrics := NewMetricsCollector()
	metrics.RecordOperation(100*time.Millisecond, true, 1024)

	hc := NewHealthChecker(metrics, nil)
	result := hc.checkMetrics()

	if !result.Healthy {
		t.Errorf("expected healthy with valid metrics: %s", result.Message)
	}
	if !strings.Contains(result.Message, "operations processed") {
		t.Errorf("unexpected message: %s", result.Message)
	}
}

// ============================================================================
// checkMemoryUsage TESTS
// ============================================================================

func TestCheckMemoryUsage_WithinLimit(t *testing.T) {
	metrics := NewMetricsCollector()
	config := &HealthCheckerConfig{
		MaxMemoryBytes: 10 * 1024 * 1024 * 1024, // 10GB - very high limit
	}
	hc := NewHealthChecker(metrics, config)
	result := hc.checkMemoryUsage()

	if !result.Healthy {
		t.Errorf("expected healthy memory check: %s", result.Message)
	}
	if !strings.Contains(result.Message, "Memory:") {
		t.Errorf("unexpected message format: %s", result.Message)
	}
}

func TestCheckMemoryUsage_MessageFormat(t *testing.T) {
	metrics := NewMetricsCollector()
	hc := NewHealthChecker(metrics, nil)
	result := hc.checkMemoryUsage()

	// Should contain memory info
	if !strings.Contains(result.Message, "MB") {
		t.Errorf("message should contain MB: %s", result.Message)
	}
}

// ============================================================================
// checkErrorRates TESTS
// ============================================================================

func TestCheckErrorRates_NilMetrics(t *testing.T) {
	hc := NewHealthChecker(nil, nil)
	result := hc.checkErrorRates()

	if result.Healthy {
		t.Error("expected unhealthy with nil metrics")
	}
	if !strings.Contains(result.Message, "metrics not available") {
		t.Errorf("unexpected message: %s", result.Message)
	}
}

func TestCheckErrorRates_NoOperations(t *testing.T) {
	metrics := NewMetricsCollector()
	hc := NewHealthChecker(metrics, nil)
	result := hc.checkErrorRates()

	if !result.Healthy {
		t.Errorf("expected healthy with no operations: %s", result.Message)
	}
	if !strings.Contains(result.Message, "No operations") {
		t.Errorf("unexpected message: %s", result.Message)
	}
}

func TestCheckErrorRates_LowErrorRate(t *testing.T) {
	metrics := NewMetricsCollector()
	// 5% error rate
	for i := 0; i < 95; i++ {
		metrics.RecordOperation(10*time.Millisecond, true, 0)
	}
	for i := 0; i < 5; i++ {
		metrics.RecordOperation(10*time.Millisecond, false, 0)
	}

	config := &HealthCheckerConfig{MaxErrorRatePercent: 10.0}
	hc := NewHealthChecker(metrics, config)
	result := hc.checkErrorRates()

	if !result.Healthy {
		t.Errorf("expected healthy with low error rate: %s", result.Message)
	}
	if !strings.Contains(result.Message, "healthy") {
		t.Errorf("unexpected message: %s", result.Message)
	}
}

func TestCheckErrorRates_HighErrorRate(t *testing.T) {
	metrics := NewMetricsCollector()
	// 50% error rate
	for i := 0; i < 50; i++ {
		metrics.RecordOperation(10*time.Millisecond, true, 0)
	}
	for i := 0; i < 50; i++ {
		metrics.RecordOperation(10*time.Millisecond, false, 0)
	}

	config := &HealthCheckerConfig{MaxErrorRatePercent: 10.0}
	hc := NewHealthChecker(metrics, config)
	result := hc.checkErrorRates()

	if result.Healthy {
		t.Error("expected unhealthy with high error rate")
	}
	if !strings.Contains(result.Message, "High error rate") {
		t.Errorf("unexpected message: %s", result.Message)
	}
}

func TestCheckErrorRates_ExactThreshold(t *testing.T) {
	metrics := NewMetricsCollector()
	// Exactly 10% error rate
	for i := 0; i < 90; i++ {
		metrics.RecordOperation(10*time.Millisecond, true, 0)
	}
	for i := 0; i < 10; i++ {
		metrics.RecordOperation(10*time.Millisecond, false, 0)
	}

	config := &HealthCheckerConfig{MaxErrorRatePercent: 10.0}
	hc := NewHealthChecker(metrics, config)
	result := hc.checkErrorRates()

	// At exactly 10%, it should still be healthy (not > threshold)
	if !result.Healthy {
		t.Errorf("expected healthy at exact threshold: %s", result.Message)
	}
}

// ============================================================================
// HealthStatus.GetSummary TESTS
// ============================================================================

func TestHealthStatus_GetSummary_Healthy(t *testing.T) {
	status := HealthStatus{
		Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Healthy:   true,
		Checks: map[string]CheckResult{
			"metrics":    {Healthy: true, Message: "All good"},
			"memory":     {Healthy: true, Message: "50 MB used"},
			"error_rate": {Healthy: true, Message: "0.5%"},
		},
	}

	summary := status.GetSummary()

	if !strings.Contains(summary, "HEALTHY") {
		t.Error("summary should contain HEALTHY")
	}
	if !strings.Contains(summary, "2024-01-15T10:30:00Z") {
		t.Error("summary should contain timestamp")
	}
	if !strings.Contains(summary, "✓") {
		t.Error("summary should contain checkmark for healthy checks")
	}
	if !strings.Contains(summary, "metrics") {
		t.Error("summary should contain check name 'metrics'")
	}
	if !strings.Contains(summary, "All good") {
		t.Error("summary should contain check message")
	}
}

func TestHealthStatus_GetSummary_Unhealthy(t *testing.T) {
	status := HealthStatus{
		Timestamp: time.Now(),
		Healthy:   false,
		Checks: map[string]CheckResult{
			"metrics":    {Healthy: true, Message: "OK"},
			"memory":     {Healthy: false, Message: "High memory"},
			"error_rate": {Healthy: true, Message: "Low"},
		},
	}

	summary := status.GetSummary()

	if !strings.Contains(summary, "UNHEALTHY") {
		t.Error("summary should contain UNHEALTHY")
	}
	if !strings.Contains(summary, "✗") {
		t.Error("summary should contain X mark for unhealthy checks")
	}
	if !strings.Contains(summary, "High memory") {
		t.Error("summary should contain failure message")
	}
}

func TestHealthStatus_GetSummary_EmptyChecks(t *testing.T) {
	status := HealthStatus{
		Timestamp: time.Now(),
		Healthy:   true,
		Checks:    map[string]CheckResult{},
	}

	summary := status.GetSummary()

	if !strings.Contains(summary, "HEALTHY") {
		t.Error("summary should contain HEALTHY even with no checks")
	}
}

// ============================================================================
// HealthStatus.GetFailedChecks TESTS
// ============================================================================

func TestHealthStatus_GetFailedChecks_AllHealthy(t *testing.T) {
	status := HealthStatus{
		Healthy: true,
		Checks: map[string]CheckResult{
			"metrics":    {Healthy: true, Message: "OK"},
			"memory":     {Healthy: true, Message: "OK"},
			"error_rate": {Healthy: true, Message: "OK"},
		},
	}

	failed := status.GetFailedChecks()

	if len(failed) != 0 {
		t.Errorf("expected no failed checks, got %v", failed)
	}
}

func TestHealthStatus_GetFailedChecks_SomeFailed(t *testing.T) {
	status := HealthStatus{
		Healthy: false,
		Checks: map[string]CheckResult{
			"metrics":    {Healthy: true, Message: "OK"},
			"memory":     {Healthy: false, Message: "High"},
			"error_rate": {Healthy: false, Message: "High"},
		},
	}

	failed := status.GetFailedChecks()

	if len(failed) != 2 {
		t.Errorf("expected 2 failed checks, got %d: %v", len(failed), failed)
	}

	// Check that the expected names are in the list
	failedMap := make(map[string]bool)
	for _, name := range failed {
		failedMap[name] = true
	}
	if !failedMap["memory"] {
		t.Error("expected 'memory' in failed checks")
	}
	if !failedMap["error_rate"] {
		t.Error("expected 'error_rate' in failed checks")
	}
}

func TestHealthStatus_GetFailedChecks_AllFailed(t *testing.T) {
	status := HealthStatus{
		Healthy: false,
		Checks: map[string]CheckResult{
			"metrics":    {Healthy: false, Message: "Bad"},
			"memory":     {Healthy: false, Message: "Bad"},
			"error_rate": {Healthy: false, Message: "Bad"},
		},
	}

	failed := status.GetFailedChecks()

	if len(failed) != 3 {
		t.Errorf("expected 3 failed checks, got %d", len(failed))
	}
}

// ============================================================================
// Integration TESTS
// ============================================================================

func TestHealthCheck_Integration(t *testing.T) {
	// Create metrics collector
	metrics := NewMetricsCollector()

	// Simulate some workload
	for i := 0; i < 100; i++ {
		metrics.RecordOperation(time.Duration(i+1)*time.Millisecond, true, 1024)
	}
	for i := 0; i < 5; i++ {
		metrics.RecordOperation(5*time.Millisecond, false, 0)
	}
	metrics.RecordCacheHit()
	metrics.RecordCacheHit()
	metrics.RecordCacheMiss()

	// Create health checker with custom config
	config := &HealthCheckerConfig{
		MaxMemoryBytes:      2 * 1024 * 1024 * 1024, // 2GB
		MaxErrorRatePercent: 10.0,
	}
	hc := NewHealthChecker(metrics, config)

	// Perform health check
	status := hc.CheckHealth()

	// Verify results
	if !status.Healthy {
		t.Error("expected system to be healthy")
	}

	summary := status.GetSummary()
	t.Logf("Health Summary:\n%s", summary)

	failed := status.GetFailedChecks()
	if len(failed) > 0 {
		t.Errorf("unexpected failed checks: %v", failed)
	}

	// Verify all checks are present
	expectedChecks := []string{"metrics", "memory", "error_rate"}
	for _, check := range expectedChecks {
		if _, ok := status.Checks[check]; !ok {
			t.Errorf("missing check: %s", check)
		}
	}
}

func TestHealthCheck_Integration_FailingSystem(t *testing.T) {
	// Create metrics collector
	metrics := NewMetricsCollector()

	// Simulate failing workload (80% error rate)
	for i := 0; i < 20; i++ {
		metrics.RecordOperation(10*time.Millisecond, true, 0)
	}
	for i := 0; i < 80; i++ {
		metrics.RecordOperation(10*time.Millisecond, false, 0)
	}

	// Create health checker with strict error rate
	config := &HealthCheckerConfig{
		MaxErrorRatePercent: 5.0, // Only 5% allowed
	}
	hc := NewHealthChecker(metrics, config)

	// Perform health check
	status := hc.CheckHealth()

	// Should be unhealthy
	if status.Healthy {
		t.Error("expected system to be unhealthy with high error rate")
	}

	failed := status.GetFailedChecks()
	if len(failed) == 0 {
		t.Error("expected some failed checks")
	}

	// error_rate should be in failed list
	found := false
	for _, name := range failed {
		if name == "error_rate" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'error_rate' in failed checks")
	}

	summary := status.GetSummary()
	t.Logf("Health Summary (Unhealthy):\n%s", summary)
}

// ============================================================================
// Edge Cases TESTS
// ============================================================================

func TestCheckErrorRates_AllFailures(t *testing.T) {
	metrics := NewMetricsCollector()
	// 100% error rate
	for i := 0; i < 100; i++ {
		metrics.RecordOperation(10*time.Millisecond, false, 0)
	}

	hc := NewHealthChecker(metrics, nil)
	result := hc.checkErrorRates()

	if result.Healthy {
		t.Error("expected unhealthy with 100% error rate")
	}
}

func TestCheckErrorRates_AllSuccesses(t *testing.T) {
	metrics := NewMetricsCollector()
	// 0% error rate
	for i := 0; i < 100; i++ {
		metrics.RecordOperation(10*time.Millisecond, true, 0)
	}

	hc := NewHealthChecker(metrics, nil)
	result := hc.checkErrorRates()

	if !result.Healthy {
		t.Errorf("expected healthy with 0%% error rate: %s", result.Message)
	}
}

func TestHealthStatus_Timestamp(t *testing.T) {
	before := time.Now()
	metrics := NewMetricsCollector()
	hc := NewHealthChecker(metrics, nil)
	status := hc.CheckHealth()
	after := time.Now()

	if status.Timestamp.Before(before) || status.Timestamp.After(after) {
		t.Error("timestamp should be within test execution time")
	}
}

func TestNewHealthChecker_BoundaryValues(t *testing.T) {
	metrics := NewMetricsCollector()

	tests := []struct {
		name              string
		config            *HealthCheckerConfig
		expectedMemory    uint64
		expectedErrorRate float64
	}{
		{
			name:              "MaxErrorRatePercent exactly 100",
			config:            &HealthCheckerConfig{MaxErrorRatePercent: 100.0},
			expectedMemory:    defaultMaxMemoryBytes,
			expectedErrorRate: 100.0,
		},
		{
			name:              "MaxErrorRatePercent slightly above 100",
			config:            &HealthCheckerConfig{MaxErrorRatePercent: 100.1},
			expectedMemory:    defaultMaxMemoryBytes,
			expectedErrorRate: defaultMaxErrorRatePercent, // should use default
		},
		{
			name:              "very large MaxMemoryBytes",
			config:            &HealthCheckerConfig{MaxMemoryBytes: 1 << 62},
			expectedMemory:    1 << 62,
			expectedErrorRate: defaultMaxErrorRatePercent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hc := NewHealthChecker(metrics, tt.config)
			if hc.maxMemoryBytes != tt.expectedMemory {
				t.Errorf("expected maxMemoryBytes %d, got %d", tt.expectedMemory, hc.maxMemoryBytes)
			}
			if hc.maxErrorRatePercent != tt.expectedErrorRate {
				t.Errorf("expected maxErrorRatePercent %.1f, got %.1f", tt.expectedErrorRate, hc.maxErrorRatePercent)
			}
		})
	}
}

// ============================================================================
// Concurrent TESTS
// ============================================================================

func TestHealthCheck_ConcurrentChecks(t *testing.T) {
	metrics := NewMetricsCollector()
	hc := NewHealthChecker(metrics, nil)

	// Start some operations
	for i := 0; i < 50; i++ {
		metrics.RecordOperation(10*time.Millisecond, true, 0)
	}

	// Run multiple health checks concurrently
	done := make(chan HealthStatus, 10)
	for i := 0; i < 10; i++ {
		go func() {
			done <- hc.CheckHealth()
		}()
	}

	// Collect results
	for i := 0; i < 10; i++ {
		status := <-done
		if !status.Healthy {
			t.Error("concurrent health check returned unhealthy")
		}
	}
}

// ============================================================================
// Benchmark TESTS
// ============================================================================

func BenchmarkHealthCheck(b *testing.B) {
	metrics := NewMetricsCollector()
	for i := 0; i < 1000; i++ {
		metrics.RecordOperation(time.Millisecond, true, 1024)
	}
	hc := NewHealthChecker(metrics, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = hc.CheckHealth()
	}
}

func BenchmarkGetSummary(b *testing.B) {
	status := HealthStatus{
		Timestamp: time.Now(),
		Healthy:   true,
		Checks: map[string]CheckResult{
			"metrics":    {Healthy: true, Message: "All good"},
			"memory":     {Healthy: true, Message: "50 MB used"},
			"error_rate": {Healthy: true, Message: "0.5%"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = status.GetSummary()
	}
}

func BenchmarkGetFailedChecks(b *testing.B) {
	status := HealthStatus{
		Healthy: false,
		Checks: map[string]CheckResult{
			"metrics":    {Healthy: true, Message: "OK"},
			"memory":     {Healthy: false, Message: "High"},
			"error_rate": {Healthy: false, Message: "High"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = status.GetFailedChecks()
	}
}
