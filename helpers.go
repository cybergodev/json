package json

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cybergodev/json/internal"
)

// Type constraints for Go 1.24+ generics
type (
	// Numeric represents all numeric types
	Numeric interface {
		~int | ~int8 | ~int16 | ~int32 | ~int64 |
			~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
			~float32 | ~float64
	}

	// Ordered represents types that can be ordered
	Ordered interface {
		Numeric | ~string
	}

	// JSONValue represents valid JSON value types
	JSONValue interface {
		~bool | ~string | Numeric | ~[]any | ~map[string]any | any
	}

	// Signed represents signed integer types
	Signed interface {
		~int | ~int8 | ~int16 | ~int32 | ~int64
	}

	// Unsigned represents unsigned integer types
	Unsigned interface {
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64
	}

	// Float represents floating-point types
	Float interface {
		~float32 | ~float64
	}
)

// GetTypedWithProcessor retrieves a typed value from JSON using a specific processor with optimized conversion
func GetTypedWithProcessor[T any](processor *Processor, jsonStr, path string, opts ...*ProcessorOptions) (T, error) {
	var zero T

	// Get the raw value
	value, err := processor.Get(jsonStr, path, opts...)
	if err != nil {
		return zero, err
	}

	// Handle null values specially
	if value == nil {
		return handleNullValue[T](path)
	}

	// Use unified type conversion for better performance
	if converted, ok := UnifiedTypeConversion[T](value); ok {
		return converted, nil
	}

	// Fallback: JSON marshaling/unmarshaling for complex types
	jsonBytes, err := json.Marshal(value)
	if err != nil {
		return zero, &JsonsError{
			Op:      "get_typed",
			Path:    path,
			Message: fmt.Sprintf("failed to marshal value for type conversion: %v", err),
			Err:     ErrTypeMismatch,
		}
	}

	var finalResult T
	if err := json.Unmarshal(jsonBytes, &finalResult); err != nil {
		return zero, &JsonsError{
			Op:      "get_typed",
			Path:    path,
			Message: fmt.Sprintf("failed to convert value to type %T: %v", finalResult, err),
			Err:     ErrTypeMismatch,
		}
	}

	return finalResult, nil
}

// tryDirectConversion is now replaced by UnifiedTypeConversion in type_conversion_unified.go

// handleNullValue handles null values for different target types
func handleNullValue[T any](path string) (T, error) {
	var zero T
	targetType := fmt.Sprintf("%T", zero)

	switch targetType {
	case "string":
		// For string type, return "null" as string representation
		if result, ok := any("null").(T); ok {
			return result, nil
		}
	case "*string":
		// For pointer to string, return nil pointer
		if result, ok := any((*string)(nil)).(T); ok {
			return result, nil
		}
	case "int", "int8", "int16", "int32", "int64":
		// For integer types, return zero value
		return zero, nil
	case "uint", "uint8", "uint16", "uint32", "uint64":
		// For unsigned integer types, return zero value
		return zero, nil
	case "float32", "float64":
		// For float types, return zero value
		return zero, nil
	case "bool":
		// For bool type, return false
		return zero, nil
	default:
		// For other types (slices, maps, structs, pointers), return zero value
		return zero, nil
	}

	return zero, &JsonsError{
		Op:      "get_typed",
		Path:    path,
		Message: fmt.Sprintf("cannot convert null to type %T", zero),
		Err:     ErrTypeMismatch,
	}
}

// conversionResult holds the result of a type conversion attempt
type conversionResult[T any] struct {
	value T
	err   error
}

// handleLargeNumberConversion handles conversion of large numbers to specific types
func handleLargeNumberConversion[T any](value any, path string) (conversionResult[T], bool) {
	var zero T

	// Get the target type information
	targetType := fmt.Sprintf("%T", zero)

	switch targetType {
	case "int64":
		if converted, err := SafeConvertToInt64(value); err == nil {
			// Use type assertion to convert to T (which we know is int64)
			if typedResult, ok := any(converted).(T); ok {
				return conversionResult[T]{value: typedResult, err: nil}, true
			}
		} else {
			// Return error with helpful message
			return conversionResult[T]{
				value: zero,
				err: &JsonsError{
					Op:      "get_typed",
					Path:    path,
					Message: fmt.Sprintf("large number conversion failed: %v", err),
					Err:     ErrTypeMismatch,
				},
			}, true
		}

	case "uint64":
		if converted, err := SafeConvertToUint64(value); err == nil {
			if typedResult, ok := any(converted).(T); ok {
				return conversionResult[T]{value: typedResult, err: nil}, true
			}
		} else {
			return conversionResult[T]{
				value: zero,
				err: &JsonsError{
					Op:      "get_typed",
					Path:    path,
					Message: fmt.Sprintf("large number conversion failed: %v", err),
					Err:     ErrTypeMismatch,
				},
			}, true
		}

	case "string":
		// For string type, convert any numeric value to string representation
		if strResult, ok := any(FormatNumber(value)).(T); ok {
			return conversionResult[T]{value: strResult, err: nil}, true
		}
	}

	// Not handled by this function
	return conversionResult[T]{value: zero, err: nil}, false
}

// IsValidJson quickly checks if a string is valid JSON
func IsValidJson(jsonStr string) bool {
	decoder := NewNumberPreservingDecoder(false) // Use fast validation
	_, err := decoder.DecodeToAny(jsonStr)
	return err == nil
}

// IsValidPath checks if a path expression is valid with comprehensive validation
func IsValidPath(path string) bool {
	// Empty path is invalid
	if path == "" {
		return false
	}

	// Root path "." is valid
	if path == "." {
		return true
	}

	// Use default processor for validation
	processor := getDefaultProcessor()
	err := processor.validatePath(path)
	return err == nil
}

// ValidatePath validates a path expression and returns detailed error information
func ValidatePath(path string) error {
	if path == "" {
		return &JsonsError{
			Op:      "validate_path",
			Path:    path,
			Message: "path cannot be empty",
			Err:     ErrInvalidPath,
		}
	}

	if path == "." {
		return nil
	}

	processor := getDefaultProcessor()
	return processor.validatePath(path)
}

// Internal path type checking functions (used by tests)
func isJSONPointerPath(path string) bool {
	return path != "" && path[0] == '/'
}

func isDotNotationPath(path string) bool {
	return path != "" && path != "." && path[0] != '/'
}

func isArrayPath(path string) bool {
	return strings.Contains(path, "[") && strings.Contains(path, "]")
}

func isSlicePath(path string) bool {
	return strings.Contains(path, "[") && strings.Contains(path, ":") && strings.Contains(path, "]")
}

func isExtractionPath(path string) bool {
	return strings.Contains(path, "{") && strings.Contains(path, "}")
}

func isJsonObject(data any) bool {
	_, ok := data.(map[string]any)
	return ok
}

func isJsonArray(data any) bool {
	_, ok := data.([]any)
	return ok
}

func isJsonPrimitive(data any) bool {
	switch data.(type) {
	case string, int, int32, int64, float32, float64, bool, nil:
		return true
	default:
		return false
	}
}

// DeepCopy creates a deep copy of JSON-compatible data with improved efficiency
func DeepCopy(data any) (any, error) {
	// Fast path for simple types that don't need deep copying
	switch v := data.(type) {
	case nil, bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, string:
		return v, nil
	}

	// Use JSON marshaling/unmarshaling for deep copy with number preservation
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data for deep copy: %v", err)
	}

	decoder := NewNumberPreservingDecoder(true)
	result, err := decoder.DecodeToAny(string(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal data for deep copy: %v", err)
	}

	return result, nil
}

// CompareJson compares two JSON strings for equality
func CompareJson(json1, json2 string) (bool, error) {
	decoder := NewNumberPreservingDecoder(true)

	data1, err := decoder.DecodeToAny(json1)
	if err != nil {
		return false, fmt.Errorf("invalid JSON in first argument: %v", err)
	}

	data2, err := decoder.DecodeToAny(json2)
	if err != nil {
		return false, fmt.Errorf("invalid JSON in second argument: %v", err)
	}

	// Convert both to JSON strings for comparison
	bytes1, err := json.Marshal(data1)
	if err != nil {
		return false, err
	}

	bytes2, err := json.Marshal(data2)
	if err != nil {
		return false, err
	}

	return string(bytes1) == string(bytes2), nil
}

// MergeJson merges two JSON objects
func MergeJson(json1, json2 string) (string, error) {
	decoder := NewNumberPreservingDecoder(true)

	data1, err := decoder.DecodeToAny(json1)
	if err != nil {
		return "", fmt.Errorf("invalid JSON in first argument: %v", err)
	}

	data2, err := decoder.DecodeToAny(json2)
	if err != nil {
		return "", fmt.Errorf("invalid JSON in second argument: %v", err)
	}

	// Ensure both are objects
	obj1, ok1 := data1.(map[string]any)
	obj2, ok2 := data2.(map[string]any)

	if !ok1 {
		return "", fmt.Errorf("first JSON is not an object")
	}
	if !ok2 {
		return "", fmt.Errorf("second JSON is not an object")
	}

	// Merge obj2 into obj1
	for key, value := range obj2 {
		obj1[key] = value
	}

	result, err := json.Marshal(obj1)
	if err != nil {
		return "", fmt.Errorf("failed to marshal merged result: %v", err)
	}

	return string(result), nil
}

// SafeTypeAssert performs a safe type assertion with error handling
func SafeTypeAssert[T any](value any) (T, bool) {
	if result, ok := value.(T); ok {
		return result, true
	}
	var zero T
	return zero, false
}

// TypeSafeConvert attempts to convert a value to the target type safely
func TypeSafeConvert[T any](value any) (T, error) {
	var zero T

	// Direct type assertion first
	if result, ok := value.(T); ok {
		return result, nil
	}

	// Handle common conversions
	targetType := fmt.Sprintf("%T", zero)
	return convertWithTypeInfo[T](value, targetType)
}

// convertWithTypeInfo handles type conversion with type information
func convertWithTypeInfo[T any](value any, targetType string) (T, error) {
	var zero T

	// Use the existing conversion logic but with better type safety
	convResult, handled := handleLargeNumberConversion[T](value, "type_conversion")
	if handled {
		return convResult.value, convResult.err
	}

	// Handle string conversions
	if str, ok := value.(string); ok {
		return convertStringToType[T](str, targetType)
	}

	return zero, fmt.Errorf("cannot convert %T to %s", value, targetType)
}

// convertStringToType converts string values to target types safely
func convertStringToType[T any](str, targetType string) (T, error) {
	var zero T

	switch targetType {
	case "int", "int64":
		if val, err := strconv.ParseInt(str, 10, 64); err == nil {
			if result, ok := any(val).(T); ok {
				return result, nil
			}
		}
	case "float64":
		if val, err := strconv.ParseFloat(str, 64); err == nil {
			if result, ok := any(val).(T); ok {
				return result, nil
			}
		}
	case "bool":
		if val, err := strconv.ParseBool(str); err == nil {
			if result, ok := any(val).(T); ok {
				return result, nil
			}
		}
	case "string":
		if result, ok := any(str).(T); ok {
			return result, nil
		}
	}

	return zero, fmt.Errorf("cannot convert string %q to %s", str, targetType)
}

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
	lastCleanupTime   int64 // Last cleanup timestamp for timeout map

	// Performance tracking
	totalOperations int64
	totalWaitTime   int64
	averageWaitTime int64
}

// NewConcurrencyManager creates a new concurrency manager
func NewConcurrencyManager(maxConcurrency int, operationsPerSecond int) *ConcurrencyManager {
	return &ConcurrencyManager{
		maxConcurrency:      int32(maxConcurrency),
		operationsPerSecond: int64(operationsPerSecond),
		lastResetTime:       time.Now().Unix(),
		operationTimeouts:   make(map[uint64]int64),
		lastCleanupTime:     time.Now().Unix(),
	}
}

// ExecuteWithConcurrencyControl executes a function with concurrency control
func (cm *ConcurrencyManager) ExecuteWithConcurrencyControl(
	ctx context.Context,
	operation func() error,
	timeout time.Duration,
) error {
	if atomic.LoadInt32(&cm.circuitOpen) == 1 {
		lastFailure := atomic.LoadInt64(&cm.lastFailureTime)
		if time.Now().Unix()-lastFailure > 60 {
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

	waitTime := time.Since(start).Nanoseconds()
	atomic.AddInt64(&cm.totalWaitTime, waitTime)
	atomic.AddInt64(&cm.totalOperations, 1)

	for {
		oldAvg := atomic.LoadInt64(&cm.averageWaitTime)
		newAvg := oldAvg + (waitTime-oldAvg)/10
		if atomic.CompareAndSwapInt64(&cm.averageWaitTime, oldAvg, newAvg) {
			break
		}
	}

	// Register operation for deadlock detection
	gid := getGoroutineIDForConcurrency()
	if gid != 0 {
		cm.timeoutMutex.Lock()
		cm.operationTimeouts[gid] = time.Now().UnixNano()

		now := time.Now().Unix()
		lastCleanup := atomic.LoadInt64(&cm.lastCleanupTime)
		if now-lastCleanup > 30 {
			if atomic.CompareAndSwapInt64(&cm.lastCleanupTime, lastCleanup, now) {
				go cm.cleanupStaleTimeouts()
			}
		}
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
			Err:     ErrOperationTimeout,
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

// cleanupStaleTimeouts removes stale entries from operationTimeouts map
func (cm *ConcurrencyManager) cleanupStaleTimeouts() {
	cm.timeoutMutex.Lock()
	defer cm.timeoutMutex.Unlock()

	const maxMapSize = 100
	const targetSize = 50

	currentSize := len(cm.operationTimeouts)

	if cm.operationTimeouts == nil || currentSize == 0 {
		return
	}

	// Recreate map if it exceeds limit
	if currentSize > maxMapSize {
		cm.operationTimeouts = make(map[uint64]int64, targetSize)
		return
	}

	// Remove stale entries
	now := time.Now().UnixNano()
	threshold := int64(60 * time.Second)

	for gid, startTime := range cm.operationTimeouts {
		if now-startTime > threshold {
			delete(cm.operationTimeouts, gid)
		}
	}
}

// DeadlockInfo represents information about a potential deadlock
type DeadlockInfo struct {
	GoroutineID uint64
	StartTime   time.Time
	Duration    time.Duration
}

// getGoroutineIDForConcurrency returns the current goroutine ID for concurrency tracking
// This is an alias to getGoroutineID to avoid code duplication
func getGoroutineIDForConcurrency() uint64 {
	return getGoroutineID()
}

// IteratorControl represents control flags for iteration
type IteratorControl int

const (
	IteratorNormal   IteratorControl = iota // normal execution
	IteratorContinue                        // Continue current item and continue
	IteratorBreak                           // Break entire iteration
)

// Nested call tracking to prevent state conflicts with optimized memory management
var (
	nestedCallTracker = make(map[uint64]int, 10) // Minimal size for better memory efficiency
	nestedCallMutex   sync.RWMutex
	lastCleanupTime   int64      // Unix timestamp of last cleanup
	maxTrackerSize    = 10       // Strict limit to prevent memory bloat
	cleanupInterval   = int64(5) // Cleanup every 5 seconds for balance
)

// getGoroutineID returns the current goroutine ID (optimized version)
func getGoroutineID() uint64 {
	var buf [32]byte // Reduced buffer size for efficiency
	n := runtime.Stack(buf[:], false)

	// Parse goroutine ID from stack trace
	// Format: "goroutine 123 [running]:"
	// Optimized parsing without string conversion
	if n < 10 {
		return 0
	}

	// Look for "goroutine " (10 bytes)
	const prefix = "goroutine "
	if n < len(prefix) {
		return 0
	}

	// Check if it starts with "goroutine "
	for i := 0; i < len(prefix); i++ {
		if buf[i] != prefix[i] {
			return 0
		}
	}

	// Parse the number after "goroutine "
	var id uint64
	for i := len(prefix); i < n && buf[i] != ' '; i++ {
		if buf[i] >= '0' && buf[i] <= '9' {
			id = id*10 + uint64(buf[i]-'0')
		} else {
			break
		}
	}

	return id
}

// isNestedCall checks if we're in a nested Foreach/ForeachReturn call
func isNestedCall() bool {
	gid := getGoroutineID()
	if gid == 0 {
		return false
	}

	nestedCallMutex.RLock()
	level := nestedCallTracker[gid]
	nestedCallMutex.RUnlock()

	return level > 0
}

// enterNestedCall increments the nesting level for current goroutine
func enterNestedCall() {
	gid := getGoroutineID()
	if gid == 0 {
		return
	}

	nestedCallMutex.Lock()
	defer nestedCallMutex.Unlock()

	// Prevent tracker from growing too large
	if len(nestedCallTracker) >= maxTrackerSize {
		cleanupDeadGoroutines()
	}

	nestedCallTracker[gid]++
}

// exitNestedCall decrements the nesting level for current goroutine
func exitNestedCall() {
	gid := getGoroutineID()
	if gid == 0 {
		return
	}

	nestedCallMutex.Lock()
	defer nestedCallMutex.Unlock()

	if nestedCallTracker[gid] > 0 {
		nestedCallTracker[gid]--
		if nestedCallTracker[gid] == 0 {
			delete(nestedCallTracker, gid)
		}
	}

	// Enhanced periodic cleanup to prevent memory leaks from dead goroutines
	now := time.Now().Unix()
	if now-lastCleanupTime > cleanupInterval {
		cleanupDeadGoroutines()
		lastCleanupTime = now
	}
}

// cleanupDeadGoroutines removes entries for goroutines that no longer exist
func cleanupDeadGoroutines() {
	if len(nestedCallTracker) <= maxTrackerSize {
		for gid, level := range nestedCallTracker {
			if level <= 0 {
				delete(nestedCallTracker, gid)
			}
		}
		return
	}

	// Clear everything if tracker exceeded limit
	clear(nestedCallTracker)
}

// IteratorControlSignal represents control flow signals for iteration
type IteratorControlSignal struct {
	Type IteratorControl
}

// Error implements error interface for IteratorControlSignal
func (ics IteratorControlSignal) Error() string {
	switch ics.Type {
	case IteratorContinue:
		return "iterator_continue"
	case IteratorBreak:
		return "iterator_break"
	default:
		return "iterator_normal"
	}
}

// Use the DeletedMarker from interfaces.go
var deletedMarker = DeletedMarker

// Iterator represents an iterator for JSON data with support for read/write operations
type Iterator struct {
	processor    *Processor        // JSON processor instance
	data         any               // Original parsed JSON data
	currentData  any               // Current data being iterated
	currentKey   any               // Current key (index for arrays, property name for objects)
	currentValue any               // Current value
	currentPath  string            // Current path in dot notation
	control      IteratorControl   // Control flag for iteration
	opts         *ProcessorOptions // Processing options
}

// NewIterator creates a new iterator for the given JSON data
func NewIterator(processor *Processor, data any, opts *ProcessorOptions) *Iterator {
	return &Iterator{
		processor:   processor,
		data:        data,
		currentData: data,
		control:     IteratorNormal,
		opts:        opts,
	}
}

// Get retrieves a value from the current item using a relative path
func (it *Iterator) Get(path string) (any, error) {
	if it.currentValue == nil {
		return nil, fmt.Errorf("no current value available")
	}

	// If path is empty, return current value
	if path == "" {
		return it.currentValue, nil
	}

	// Determine the correct context to search from (same logic as IterableValue.Get)
	var searchContext any

	switch it.currentValue.(type) {
	case map[string]any, map[any]any:
		// Current value is an object, search within it
		searchContext = it.currentValue
	default:
		// Current value is a primitive, but we might be iterating over object properties
		// In this case, we need to search from the parent object
		if it.currentData != nil {
			switch it.currentData.(type) {
			case map[string]any, map[any]any:
				searchContext = it.currentData
			default:
				searchContext = it.currentValue
			}
		} else {
			searchContext = it.currentValue
		}
	}

	if searchContext == nil {
		return nil, fmt.Errorf("no search context available")
	}

	// Navigate from the determined search context
	// Use compatibility layer for navigation
	return it.processor.navigateToPath(searchContext, path)
}

// Set sets a value in the current item using a relative path
func (it *Iterator) Set(path string, value any) error {
	if it.currentValue == nil {
		return fmt.Errorf("no current value available")
	}

	// If path is empty, we cannot set the root value directly
	if path == "" {
		return fmt.Errorf("cannot set root value of current item")
	}

	// Determine the correct target object for setting values (same logic as IterableValue.Set)
	var targetObject any

	switch it.currentValue.(type) {
	case map[string]any, map[any]any:
		// Current value is an object, set within it
		targetObject = it.currentValue
	default:
		// Current value is a primitive, but we might be iterating over object properties
		// In this case, we need to set in the parent object
		if it.currentData != nil {
			switch it.currentData.(type) {
			case map[string]any, map[any]any:
				targetObject = it.currentData
			default:
				targetObject = it.currentValue
			}
		} else {
			targetObject = it.currentValue
		}
	}

	if targetObject == nil {
		return fmt.Errorf("no target object available")
	}

	// Use unified recursive processor for consistent complex path handling
	unifiedProcessor := NewRecursiveProcessor(it.processor)
	_, err := unifiedProcessor.ProcessRecursivelyWithOptions(targetObject, path, OpSet, value, false)
	return err
}

// Delete deletes a value from the current item using a relative path
func (it *Iterator) Delete(path string) error {
	if it.currentValue == nil {
		return fmt.Errorf("no current value available")
	}

	// If path is empty, we cannot delete the root value
	if path == "" {
		return fmt.Errorf("cannot delete root value of current item")
	}

	// Determine the correct target object for deletion (same logic as IterableValue.Delete)
	var targetObject any

	switch it.currentValue.(type) {
	case map[string]any, map[any]any:
		// Current value is an object, delete within it
		targetObject = it.currentValue
	default:
		// Current value is a primitive, but we might be iterating over object properties
		// In this case, we need to delete from the parent object
		if it.currentData != nil {
			switch it.currentData.(type) {
			case map[string]any, map[any]any:
				targetObject = it.currentData
			default:
				targetObject = it.currentValue
			}
		} else {
			targetObject = it.currentValue
		}
	}

	if targetObject == nil {
		return fmt.Errorf("no target object available")
	}

	// Use unified recursive processor for consistent complex path handling
	unifiedProcessor := NewRecursiveProcessor(it.processor)
	_, err := unifiedProcessor.ProcessRecursively(targetObject, path, OpDelete, nil)
	return err
}

// Continue skips the current iteration
// Returns an error that should be returned from the callback
func (it *Iterator) Continue() error {
	it.control = IteratorContinue
	return ErrIteratorControl
}

// Break stops the entire iteration
// Returns an error that should be returned from the callback
func (it *Iterator) Break() error {
	it.control = IteratorBreak
	return ErrIteratorControl
}

// GetCurrentKey returns the current key (index for arrays, property name for objects)
func (it *Iterator) GetCurrentKey() any {
	return it.currentKey
}

// GetCurrentValue returns the current value
func (it *Iterator) GetCurrentValue() any {
	return it.currentValue
}

// GetCurrentPath returns the current path in dot notation
func (it *Iterator) GetCurrentPath() string {
	return it.currentPath
}

// ForeachCallback represents the callback function for iteration
// The value parameter is always *IterableValue, so no type assertion is needed
type ForeachCallback func(key any, value *IterableValue)

// ForeachCallbackWithIterator represents the callback function with iterator access
type ForeachCallbackWithIterator func(key, value any, it *Iterator)

// IterableValue represents a value that supports Get operations like the user expects
type IterableValue struct {
	data      any
	processor *Processor
	iterator  *Iterator // Reference to iterator for control operations
}

// Get retrieves a value using path notation (like user expects: item.Get("name"))
func (iv *IterableValue) Get(path string) any {
	if iv.processor == nil {
		return nil
	}

	// If path is empty, return current value
	if path == "" {
		return iv.data
	}

	// Determine the correct context to search from
	var searchContext any
	var adjustedPath = path

	if iv.iterator != nil {
		// We have iterator context, use the appropriate data source
		switch iv.iterator.currentValue.(type) {
		case map[string]any, map[any]any:
			// Current value is an object, search within it
			searchContext = iv.iterator.currentValue
			adjustedPath = path

			// If we're iterating over object properties and the path starts with the current key,
			// we need to adjust the path to be relative to the current object
			if iv.iterator.currentPath != "" && strings.HasPrefix(path, iv.iterator.currentPath+".") {
				// Remove the current path prefix from the search path
				adjustedPath = strings.TrimPrefix(path, iv.iterator.currentPath+".")
			} else if path == iv.iterator.currentPath {
				// The path exactly matches current path, return current value
				return iv.iterator.currentValue
			}
		case []any:
			// Current value is an array
			// Check if the path is meant for the current array or for root data
			if strings.HasPrefix(path, "[") || strings.HasPrefix(path, "{") {
				// Path starts with array/object syntax, search within current array
				searchContext = iv.iterator.currentValue
				adjustedPath = path
			} else {
				// Path is an absolute property name, search from root
				if iv.iterator.data != nil {
					searchContext = iv.iterator.data
				} else {
					searchContext = iv.data
				}
				adjustedPath = path
			}
		default:
			// Current value is a primitive, but we might be iterating over object properties
			// In this case, we need to search from the parent object
			if iv.iterator.currentData != nil {
				switch iv.iterator.currentData.(type) {
				case map[string]any, map[any]any:
					searchContext = iv.iterator.currentData
				default:
					// Use root data from iterator
					if iv.iterator.data != nil {
						searchContext = iv.iterator.data
					} else {
						searchContext = iv.data
					}
				}
			} else {
				// Use root data from iterator
				if iv.iterator.data != nil {
					searchContext = iv.iterator.data
				} else {
					searchContext = iv.data
				}
			}
			adjustedPath = path
		}
	} else {
		// No iterator context, use the data directly
		searchContext = iv.data
	}

	if searchContext == nil {
		return nil
	}

	// Navigation with better complex path support - use same logic as Get method
	var result any
	var err error

	// Use unified recursive processor for all paths
	unifiedProcessor := NewRecursiveProcessor(iv.processor)
	result, err = unifiedProcessor.ProcessRecursively(searchContext, adjustedPath, OpGet, nil)

	if err != nil {
		// If navigation failed, try alternative approaches for complex paths
		if iv.processor.isComplexPath(adjustedPath) {
			return iv.getComplexPathFallback(searchContext, adjustedPath)
		}
		return nil
	}
	return result
}

// getComplexPathFallback provides unified recursive handling for complex paths in IterableValue
func (iv *IterableValue) getComplexPathFallback(searchContext any, path string) any {
	if iv.processor == nil {
		return nil
	}

	// Use unified recursive processor for consistent path handling
	unifiedProcessor := NewRecursiveProcessor(iv.processor)
	result, err := unifiedProcessor.ProcessRecursively(searchContext, path, OpGet, nil)
	if err != nil {
		return nil
	}

	return result
}

// processSegmentSafely processes a path segment with error handling
func (iv *IterableValue) processSegmentSafely(current any, segment internal.PathSegment, allSegments []internal.PathSegment, segmentIndex int) (any, error) {
	switch segment.Type {
	case internal.PropertySegment:
		return iv.getPropertySafely(current, segment.Key)
	case internal.ArrayIndexSegment:
		return iv.getArrayIndexSafely(current, segment.Index)
	case internal.ArraySliceSegment:
		return iv.getArraySliceSafely(current, segment)
	case internal.ExtractSegment:
		return iv.getExtractSafely(current, segment)
	case internal.WildcardSegment:
		return iv.getWildcardSafely(current)
	default:
		return nil, fmt.Errorf("unsupported segment type: %v", segment.Type)
	}
}

// getPropertySafely safely gets a property value
func (iv *IterableValue) getPropertySafely(current any, key string) (any, error) {
	switch v := current.(type) {
	case map[string]any:
		if value, exists := v[key]; exists {
			return value, nil
		}
	case map[any]any:
		if value, exists := v[key]; exists {
			return value, nil
		}
	}
	return nil, nil // Return nil instead of error for missing properties
}

// getArrayIndexSafely safely gets an array element by index
func (iv *IterableValue) getArrayIndexSafely(current any, index int) (any, error) {
	if arr, ok := current.([]any); ok {
		// Handle negative indices
		if index < 0 {
			index = len(arr) + index
		}
		if index >= 0 && index < len(arr) {
			return arr[index], nil
		}
	}
	return nil, nil // Return nil instead of error for out of bounds
}

// getArraySliceSafely safely gets an array slice
func (iv *IterableValue) getArraySliceSafely(current any, segment internal.PathSegment) (any, error) {
	if arr, ok := current.([]any); ok {
		result := internal.PerformArraySlice(arr, segment.Start, segment.End, segment.Step)
		return result, nil
	}
	return nil, nil
}

// getExtractSafely safely performs extraction operations
func (iv *IterableValue) getExtractSafely(current any, segment internal.PathSegment) (any, error) {
	switch v := current.(type) {
	case []any:
		return iv.extractFromArraySafely(v, segment)
	case map[string]any:
		if value, exists := v[segment.Key]; exists {
			return value, nil
		}
	case map[any]any:
		if value, exists := v[segment.Key]; exists {
			return value, nil
		}
	}
	return nil, nil
}

// extractFromArraySafely safely extracts values from array elements
func (iv *IterableValue) extractFromArraySafely(arr []any, segment internal.PathSegment) (any, error) {
	var results []any

	for _, item := range arr {
		switch obj := item.(type) {
		case map[string]any:
			if value, exists := obj[segment.Key]; exists {
				if segment.IsFlat {
					// Flatten the value if it's an array
					if subArr, ok := value.([]any); ok {
						results = append(results, subArr...)
					} else {
						results = append(results, value)
					}
				} else {
					results = append(results, value)
				}
			}
		case map[any]any:
			if value, exists := obj[segment.Key]; exists {
				if segment.IsFlat {
					// Flatten the value if it's an array
					if subArr, ok := value.([]any); ok {
						results = append(results, subArr...)
					} else {
						results = append(results, value)
					}
				} else {
					results = append(results, value)
				}
			}
		}
	}

	return results, nil
}

// getWildcardSafely safely handles wildcard operations
func (iv *IterableValue) getWildcardSafely(current any) (any, error) {
	switch v := current.(type) {
	case []any:
		return v, nil
	case map[string]any:
		values := make([]any, 0, len(v))
		for _, value := range v {
			values = append(values, value)
		}
		return values, nil
	case map[any]any:
		values := make([]any, 0, len(v))
		for _, value := range v {
			values = append(values, value)
		}
		return values, nil
	}
	return nil, nil
}

// GetString retrieves a string value using path notation
func (iv *IterableValue) GetString(path string) string {
	value := iv.Get(path)
	if value == nil {
		return ""
	}
	return convertToString(value)
}

// GetInt retrieves an int value using path notation
func (iv *IterableValue) GetInt(path string) int {
	value := iv.Get(path)
	if value == nil {
		return 0
	}
	result, err := convertToInt(value)
	if err != nil {
		return 0
	}
	return result
}

// GetInt64 retrieves an int64 value using path notation
func (iv *IterableValue) GetInt64(path string) int64 {
	value := iv.Get(path)
	if value == nil {
		return 0
	}
	result, err := convertToInt64(value)
	if err != nil {
		return 0
	}
	return result
}

// GetFloat64 retrieves a float64 value using path notation
func (iv *IterableValue) GetFloat64(path string) float64 {
	value := iv.Get(path)
	if value == nil {
		return 0.0
	}
	result, err := convertToFloat64(value)
	if err != nil {
		return 0.0
	}
	return result
}

// GetBool retrieves a bool value using path notation
func (iv *IterableValue) GetBool(path string) bool {
	value := iv.Get(path)
	if value == nil {
		return false
	}
	result, err := convertToBool(value)
	if err != nil {
		return false
	}
	return result
}

// GetArray retrieves an array value using path notation
func (iv *IterableValue) GetArray(path string) []any {
	value := iv.Get(path)
	if value == nil {
		return nil
	}
	result, err := convertToArray(value)
	if err != nil {
		return nil
	}
	return result
}

// GetObject retrieves an object value using path notation
func (iv *IterableValue) GetObject(path string) map[string]any {
	value := iv.Get(path)
	if value == nil {
		return nil
	}
	result, err := convertToObject(value)
	if err != nil {
		return nil
	}
	return result
}

// GetWithDefault retrieves a value with a default fallback
func (iv *IterableValue) GetWithDefault(path string, defaultValue any) any {
	if iv.processor == nil {
		return defaultValue
	}

	// If path is empty, return current value or default
	if path == "" {
		if iv.data == nil {
			return defaultValue
		}
		return iv.data
	}

	// Try to get the value
	result, err := GetIterableValue[any](iv, path)
	if err != nil {
		return defaultValue
	}
	return result
}

// GetStringWithDefault retrieves a string value with a default fallback
func (iv *IterableValue) GetStringWithDefault(path, defaultValue string) string {
	// First check if the path exists
	rawValue := iv.Get(path)
	if rawValue == nil {
		return defaultValue
	}

	// Try to get typed value
	res, err := GetIterableValue[string](iv, path)
	if err != nil {
		return defaultValue
	}

	// If the result is empty string, return default
	if res == "" {
		return defaultValue
	}

	return res
}

// GetIntWithDefault retrieves an int value with a default fallback
func (iv *IterableValue) GetIntWithDefault(path string, defaultValue int) int {
	// First check if the path exists
	rawValue := iv.Get(path)
	if rawValue == nil {
		return defaultValue
	}

	// Try to get typed value
	res, err := GetIterableValue[int](iv, path)
	if err != nil {
		return defaultValue
	}
	return res
}

// GetInt64WithDefault retrieves an int64 value with a default fallback
func (iv *IterableValue) GetInt64WithDefault(path string, defaultValue int64) int64 {
	// First check if the path exists
	rawValue := iv.Get(path)
	if rawValue == nil {
		return defaultValue
	}

	// Try to get typed value
	res, err := GetIterableValue[int64](iv, path)
	if err != nil {
		return defaultValue
	}
	return res
}

// GetFloat64WithDefault retrieves a float64 value with a default fallback
func (iv *IterableValue) GetFloat64WithDefault(path string, defaultValue float64) float64 {
	// First check if the path exists
	rawValue := iv.Get(path)
	if rawValue == nil {
		return defaultValue
	}

	// Try to get typed value
	res, err := GetIterableValue[float64](iv, path)
	if err != nil {
		return defaultValue
	}
	return res
}

// GetBoolWithDefault retrieves a bool value with a default fallback
func (iv *IterableValue) GetBoolWithDefault(path string, defaultValue bool) bool {
	// First check if the path exists
	rawValue := iv.Get(path)
	if rawValue == nil {
		return defaultValue
	}

	// Try to get typed value
	res, err := GetIterableValue[bool](iv, path)
	if err != nil {
		return defaultValue
	}
	return res
}

// GetArrayWithDefault retrieves an array value with a default fallback
func (iv *IterableValue) GetArrayWithDefault(path string, defaultValue []any) []any {
	// First check if the path exists
	rawValue := iv.Get(path)
	if rawValue == nil {
		return defaultValue
	}

	// Try to get typed value
	res, err := GetIterableValue[[]any](iv, path)
	if err != nil {
		return defaultValue
	}
	return res
}

// GetObjectWithDefault retrieves an object value with a default fallback
func (iv *IterableValue) GetObjectWithDefault(path string, defaultValue map[string]any) map[string]any {
	// First check if the path exists
	rawValue := iv.Get(path)
	if rawValue == nil {
		return defaultValue
	}

	// Try to get typed value
	res, err := GetIterableValue[map[string]any](iv, path)
	if err != nil {
		return defaultValue
	}
	return res
}

// Note: GetTypedWithDefault cannot be implemented as a method because Go methods
// cannot have type parameters. Use the global GetIterableValueWithDefault function instead.
//
// Example usage:
//   result := GetIterableValueWithDefault[string](item, "path", "default")

// GetIterableValueWithDefault retrieves a typed value from IterableValue with a default fallback
func GetIterableValueWithDefault[T any](iv *IterableValue, path string, defaultValue T) T {
	// First check if the path exists
	rawValue := iv.Get(path)
	if rawValue == nil {
		return defaultValue
	}

	// Try to get typed value
	res, err := GetIterableValue[T](iv, path)
	if err != nil {
		return defaultValue
	}
	return res
}

// GetIterableValue is a generic function that provides GetTyped functionality for IterableValue
// Usage: name, err := GetIterableValue[string](item, "name")
func GetIterableValue[T any](iv *IterableValue, path string) (T, error) {
	var zero T

	if iv.processor == nil {
		return zero, fmt.Errorf("no processor available")
	}

	// If path is empty, return current value
	if path == "" {
		if iv.data == nil {
			return zero, fmt.Errorf("no data available")
		}
		return convertToTypeForIterator[T](iv.data, path)
	}

	// Determine the correct context to search from (same logic as Get method)
	var searchContext any
	var adjustedPath = path

	if iv.iterator != nil {
		// We have iterator context, use the appropriate data source
		switch iv.iterator.currentValue.(type) {
		case map[string]any, map[any]any:
			// Current value is an object, search within it
			searchContext = iv.iterator.currentValue

			// If we're iterating over object properties and the path starts with the current key,
			// we need to adjust the path to be relative to the current object
			if iv.iterator.currentPath != "" && strings.HasPrefix(path, iv.iterator.currentPath+".") {
				// Remove the current path prefix from the search path
				adjustedPath = strings.TrimPrefix(path, iv.iterator.currentPath+".")
			} else if path == iv.iterator.currentPath {
				// The path exactly matches current path, return current value
				return convertToTypeForIterator[T](iv.iterator.currentValue, path)
			}
		case []any:
			// Current value is an array, search within it directly
			searchContext = iv.iterator.currentValue
		default:
			// Current value is a primitive, but we might be iterating over object properties
			// In this case, we need to search from the parent object
			if iv.iterator.currentData != nil {
				switch iv.iterator.currentData.(type) {
				case map[string]any, map[any]any:
					searchContext = iv.iterator.currentData
				default:
					searchContext = iv.data
				}
			} else {
				searchContext = iv.data
			}
		}
	} else {
		// No iterator context, use the data directly
		searchContext = iv.data
	}

	if searchContext == nil {
		return zero, fmt.Errorf("no search context available")
	}

	// Use the same navigation logic as Get method for consistency
	var result any
	var err error

	// Use unified recursive processor for all paths
	unifiedProcessor := NewRecursiveProcessor(iv.processor)
	result, err = unifiedProcessor.ProcessRecursively(searchContext, adjustedPath, OpGet, nil)

	if err != nil {
		// If navigation failed, try alternative approaches for complex paths
		if iv.processor.isComplexPath(adjustedPath) {
			unifiedProcessor := NewRecursiveProcessor(iv.processor)
			result, err = unifiedProcessor.ProcessRecursively(searchContext, adjustedPath, OpGet, nil)
			if err != nil {
				return zero, err
			}
		} else {
			return zero, err
		}
	}

	// Use the same type conversion logic as GetTypedWithProcessor
	return convertToTypeForIterator[T](result, adjustedPath)
}

// convertToTypeForIterator converts a value to the specified type using improved logic
func convertToTypeForIterator[T any](value any, path string) (T, error) {
	var zero T

	// Handle null values specially
	if value == nil {
		return handleNullValue[T](path)
	}

	// Try direct type assertion first
	if typedValue, ok := value.(T); ok {
		return typedValue, nil
	}

	// Special handling for numeric types with large numbers
	convResult, handled := handleLargeNumberConversion[T](value, path)
	if handled {
		return convResult.value, convResult.err
	}

	// Improved type conversion for common cases
	targetType := fmt.Sprintf("%T", zero)

	switch targetType {
	case "int":
		if converted, err := convertToInt(value); err == nil {
			if result, ok := any(converted).(T); ok {
				return result, nil
			}
		}
	case "float64":
		if converted, err := convertToFloat64(value); err == nil {
			if result, ok := any(converted).(T); ok {
				return result, nil
			}
		}
	case "string":
		if converted := convertToString(value); converted != "" {
			if result, ok := any(converted).(T); ok {
				return result, nil
			}
		}
	case "bool":
		if converted, err := convertToBool(value); err == nil {
			if result, ok := any(converted).(T); ok {
				return result, nil
			}
		}
	case "[]interface {}":
		if converted, err := convertToArray(value); err == nil {
			if result, ok := any(converted).(T); ok {
				return result, nil
			}
		}
	case "map[string]interface {}":
		if converted, err := convertToObject(value); err == nil {
			if result, ok := any(converted).(T); ok {
				return result, nil
			}
		}
	}

	// Fallback to JSON marshaling/unmarshaling for type conversion with number preservation
	// Use custom encoder to preserve number formats
	config := NewPrettyConfig()
	config.PreserveNumbers = true

	encoder := NewCustomEncoder(config)
	defer encoder.Close()

	encodedJson, err := encoder.Encode(value)
	if err != nil {
		return zero, &JsonsError{
			Op:      "get_typed",
			Path:    path,
			Message: fmt.Sprintf("failed to marshal value for type conversion: %v", err),
			Err:     ErrTypeMismatch,
		}
	}

	var finalResult T
	// Use number-preserving unmarshal for better type conversion
	if err := PreservingUnmarshal([]byte(encodedJson), &finalResult, true); err != nil {
		return zero, &JsonsError{
			Op:      "get_typed",
			Path:    path,
			Message: fmt.Sprintf("failed to convert value to type %T: %v", finalResult, err),
			Err:     ErrTypeMismatch,
		}
	}

	return finalResult, nil
}

// convertToInt delegates to centralized type conversion
func convertToInt(value any) (int, error) {
	if result, ok := ConvertToInt(value); ok {
		return result, nil
	}
	return 0, fmt.Errorf("cannot convert %T to int", value)
}

func convertToInt64(value any) (int64, error) {
	return SafeConvertToInt64(value)
}

func convertToFloat64(value any) (float64, error) {
	if result, ok := ConvertToFloat64(value); ok {
		return result, nil
	}
	return 0, fmt.Errorf("cannot convert %T to float64", value)
}

func convertToString(value any) string {
	if value == nil {
		return ""
	}
	return ConvertToString(value)
}

func convertToBool(value any) (bool, error) {
	if result, ok := ConvertToBool(value); ok {
		return result, nil
	}
	return false, fmt.Errorf("cannot convert %T to bool", value)
}

func convertToArray(value any) ([]any, error) {
	if arr, ok := value.([]any); ok {
		return arr, nil
	}
	return nil, fmt.Errorf("cannot convert %T to []any", value)
}

func convertToObject(value any) (map[string]any, error) {
	if obj, ok := value.(map[string]any); ok {
		return obj, nil
	}
	return nil, fmt.Errorf("cannot convert %T to map[string]any", value)
}

// Set sets a value using path notation
func (iv *IterableValue) Set(path string, value any) error {
	if iv.processor == nil {
		return fmt.Errorf("no processor available")
	}

	// Determine the correct target object for setting values
	var targetObject any
	var adjustedPath = path

	if iv.iterator != nil {
		// We have iterator context, use the appropriate data source
		switch iv.iterator.currentValue.(type) {
		case map[string]any, map[any]any:
			// Current value is an object, set within it
			targetObject = iv.iterator.currentValue
		case []any:
			// Current value is an array, set within it directly
			targetObject = iv.iterator.currentValue
		default:
			// Current value is a primitive, but we might be iterating over object properties
			// In this case, we need to set in the parent object
			if iv.iterator.currentData != nil {
				switch iv.iterator.currentData.(type) {
				case map[string]any, map[any]any:
					targetObject = iv.iterator.currentData
				default:
					targetObject = iv.data
				}
			} else {
				targetObject = iv.data
			}
		}

		// Handle empty path (set current value)
		if path == "" {
			// For empty path, we need to set the current field in the parent object
			if iv.iterator.currentKey != nil && iv.iterator.currentData != nil {
				switch parent := iv.iterator.currentData.(type) {
				case map[string]any:
					if key, ok := iv.iterator.currentKey.(string); ok {
						parent[key] = value
						return nil
					}
				case map[any]any:
					parent[iv.iterator.currentKey] = value
					return nil
				}
			}
			return fmt.Errorf("cannot set current value: no parent context available")
		}
	} else {
		// No iterator context, use the data directly
		targetObject = iv.data
	}

	if targetObject == nil {
		return fmt.Errorf("no target object available")
	}

	// Use unified recursive processor for consistent complex path handling
	unifiedProcessor := NewRecursiveProcessor(iv.processor)
	_, err := unifiedProcessor.ProcessRecursivelyWithOptions(targetObject, adjustedPath, OpSet, value, false)
	return err
}

// SetWithAdd sets a value using path notation with automatic path creation
// This is equivalent to json.SetWithAdd() but works on the current iteration item
func (iv *IterableValue) SetWithAdd(path string, value any) error {
	if iv.processor == nil {
		return fmt.Errorf("no processor available")
	}

	// Determine the correct target object for setting values (same logic as Set method)
	var targetObject any
	var adjustedPath = path

	if iv.iterator != nil {
		// We have iterator context, use the appropriate data source
		switch iv.iterator.currentValue.(type) {
		case map[string]any, map[any]any:
			// Current value is an object, set within it
			targetObject = iv.iterator.currentValue
		case []any:
			// Current value is an array, set within it directly
			targetObject = iv.iterator.currentValue
		default:
			// Current value is a primitive, but we might be iterating over object properties
			// In this case, we need to set in the parent object
			if iv.iterator.currentData != nil {
				switch iv.iterator.currentData.(type) {
				case map[string]any, map[any]any:
					targetObject = iv.iterator.currentData
				default:
					targetObject = iv.data
				}
			} else {
				targetObject = iv.data
			}
		}

		// Handle empty path (set current value)
		if path == "" {
			// For empty path, we need to set the current field in the parent object
			if iv.iterator.currentKey != nil && iv.iterator.currentData != nil {
				switch parent := iv.iterator.currentData.(type) {
				case map[string]any:
					if key, ok := iv.iterator.currentKey.(string); ok {
						parent[key] = value
						return nil
					}
				case map[any]any:
					parent[iv.iterator.currentKey] = value
					return nil
				}
			}
			return fmt.Errorf("cannot set current value: no parent context available")
		}
	} else {
		// No iterator context, use the data directly
		targetObject = iv.data
	}

	if targetObject == nil {
		return fmt.Errorf("no target object available")
	}

	// Use unified recursive processor for consistent complex path handling
	unifiedProcessor := NewRecursiveProcessor(iv.processor)
	_, err := unifiedProcessor.ProcessRecursivelyWithOptions(targetObject, adjustedPath, OpSet, value, true)
	return err
}

// Delete deletes a value using path notation with unified behavior
func (iv *IterableValue) Delete(path string) error {
	if iv.processor == nil {
		return fmt.Errorf("no processor available")
	}

	if iv.iterator != nil {
		// We have iterator context - handle unified deletion logic
		return iv.deleteWithIteratorContext(path)
	} else {
		// No iterator context, use the data directly
		if path == "" {
			return fmt.Errorf("cannot delete root value")
		}
		// Use unified recursive processor for consistent complex path handling
		unifiedProcessor := NewRecursiveProcessor(iv.processor)
		_, err := unifiedProcessor.ProcessRecursively(iv.data, path, OpDelete, nil)
		return err
	}
}

// DeleteWithValidation deletes a value with path validation and suggestions
func (iv *IterableValue) DeleteWithValidation(path string) error {
	if iv.processor == nil {
		return fmt.Errorf("no processor available")
	}

	// Validate the path and provide suggestions if needed
	if warning := iv.validateDeletePath(path); warning != "" {
		// For now, we'll just log the warning. In a production environment,
		// you might want to use a proper logging framework or return the warning
		fmt.Printf("Warning: %s\n", warning)
	}

	// Perform the deletion
	return iv.Delete(path)
}

// validateDeletePath validates a delete path and returns warnings/suggestions
func (iv *IterableValue) validateDeletePath(path string) string {
	if path == "" {
		return ""
	}

	// Check for common mistakes when using ForeachReturn
	if iv.iterator != nil && iv.iterator.currentKey != nil {
		currentKey, ok := iv.iterator.currentKey.(string)
		if ok {
			// Check if the path starts with the current key (common mistake)
			if strings.HasPrefix(path, currentKey+".") {
				return fmt.Sprintf("Path '%s' starts with current key '%s'. "+
					"When iterating over '%s', you should use path '%s' instead of '%s'",
					path, currentKey, currentKey,
					strings.TrimPrefix(path, currentKey+"."), path)
			}

			// Check if the path equals the current key (might want to delete current item)
			if path == currentKey {
				return fmt.Sprintf("Path '%s' equals current key. "+
					"To delete the current item, use an empty path: item.Delete(\"\")", path)
			}
		}
	}

	// Check for obviously invalid paths
	if strings.Contains(path, "..") {
		return fmt.Sprintf("Path '%s' contains '..' which is not supported", path)
	}

	if strings.HasSuffix(path, ".") {
		return fmt.Sprintf("Path '%s' ends with '.' which is likely incorrect", path)
	}

	if strings.HasPrefix(path, ".") {
		return fmt.Sprintf("Path '%s' starts with '.' which is likely incorrect", path)
	}

	return ""
}

// deleteWithIteratorContext handles deletion with iterator context using unified logic
func (iv *IterableValue) deleteWithIteratorContext(path string) error {
	// Handle empty path (delete current field)
	if path == "" {
		return iv.deleteCurrentField()
	}

	// Semantic restriction: prevent using current key name to delete current node for simple values
	// This ensures consistent semantics and avoids confusion
	if iv.iterator.currentKey != nil {
		if keyStr, ok := iv.iterator.currentKey.(string); ok && keyStr == path {
			// Check if current value is a simple value (not an object)
			switch iv.data.(type) {
			case map[string]any, map[any]any:
				// Current value is an object, allow deleting same-named property
				// This is a legitimate operation: deleting a property within the object
			default:
				// Current value is a simple value (string, number, boolean, etc.)
				// Prohibit using current key name to avoid semantic confusion
				return fmt.Errorf("cannot use current key name '%s' as path to delete current node (simple value); use empty path item.Delete(\"\") instead", keyStr)
			}
		}
	}

	// For non-empty paths, always treat them as relative paths within the current value
	// The user should use empty path "" to delete the current item

	// For other paths, determine the correct target object
	// The key insight is that we should use the current value as the target
	// when it's an object, not the parent object
	var targetObject any

	// First, try to use the current value if it's an object or array
	switch iv.iterator.currentValue.(type) {
	case map[string]any, map[any]any:
		targetObject = iv.iterator.currentValue
	case []any:
		// Current value is an array, delete within it directly
		targetObject = iv.iterator.currentValue
	default:
		// If current value is not an object or array, check if we have a parent context
		if iv.iterator.currentData != nil {
			switch iv.iterator.currentData.(type) {
			case map[string]any, map[any]any:
				targetObject = iv.iterator.currentData
			default:
				// Fall back to root data
				targetObject = iv.data
			}
		} else {
			targetObject = iv.data
		}
	}

	if targetObject == nil {
		return fmt.Errorf("no target object available")
	}

	// Use unified recursive processor for consistent complex path handling
	unifiedProcessor := NewRecursiveProcessor(iv.processor)
	_, err := unifiedProcessor.ProcessRecursively(targetObject, path, OpDelete, nil)
	return err
}

// deleteCurrentField deletes the current field from its parent object or array
func (iv *IterableValue) deleteCurrentField() error {
	if iv.iterator.currentKey == nil || iv.iterator.currentData == nil {
		return fmt.Errorf("cannot delete current field: no parent context available")
	}

	switch parent := iv.iterator.currentData.(type) {
	case map[string]any:
		if key, ok := iv.iterator.currentKey.(string); ok {
			delete(parent, key)
			return nil
		}
		return fmt.Errorf("cannot delete current field: invalid key type")
	case map[any]any:
		delete(parent, iv.iterator.currentKey)
		return nil
	case []any:
		// Handle array deletion by index
		if index, ok := iv.iterator.currentKey.(int); ok {
			if index >= 0 && index < len(parent) {
				// Mark the element for deletion using a special marker
				// This maintains array structure and indices during iteration
				// The marked elements will be filtered out in the final result
				parent[index] = deletedMarker
				return nil
			}
			return fmt.Errorf("cannot delete current field: array index out of bounds")
		}
		return fmt.Errorf("cannot delete current field: invalid array index type")
	default:
		return fmt.Errorf("cannot delete current field: parent is not an object or array")
	}
}

// Continue skips the current iteration elegantly without requiring return
func (iv *IterableValue) Continue() {
	if iv.iterator != nil {
		iv.iterator.Continue()
	}
}

// Break stops the entire iteration elegantly without requiring return
func (iv *IterableValue) Break() {
	if iv.iterator != nil {
		iv.iterator.Break()
	}
}

// Exists checks if a path exists in the current item
func (iv *IterableValue) Exists(path string) bool {
	value := iv.Get(path)
	return value != nil
}

// ForeachNested performs nested iteration on a sub-path with isolated processor
// This prevents state conflicts when performing nested iterations
func (iv *IterableValue) ForeachNested(path string, callback ForeachCallback, opts ...*ProcessorOptions) error {
	value := iv.Get(path)
	if value == nil {
		return fmt.Errorf("path '%s' not found", path)
	}

	// Convert value to JSON string
	jsonStr, err := Encode(value)
	if err != nil {
		return fmt.Errorf("failed to encode value at path '%s': %v", path, err)
	}

	// Use nested iteration to avoid state conflicts
	return ForeachNested(jsonStr, callback, opts...)
}

// ForeachReturnNested performs nested iteration with modifications on a sub-path
// Returns the modified sub-structure and applies it back to the current item
func (iv *IterableValue) ForeachReturnNested(path string, callback ForeachCallback, opts ...*ProcessorOptions) error {
	value := iv.Get(path)
	if value == nil {
		return fmt.Errorf("path '%s' not found", path)
	}

	// Convert value to JSON string
	jsonStr, err := Encode(value)
	if err != nil {
		return fmt.Errorf("failed to encode value at path '%s': %v", path, err)
	}

	// Use nested iteration with modifications
	modifiedJson, err := ForeachReturnNested(jsonStr, callback, opts...)
	if err != nil {
		return fmt.Errorf("nested iteration failed for path '%s': %v", path, err)
	}

	// Parse the modified JSON back to data
	var modifiedData any
	err = Unmarshal([]byte(modifiedJson), &modifiedData)
	if err != nil {
		return fmt.Errorf("failed to unmarshal modified data for path '%s': %v", path, err)
	}

	// Set the modified data back to the original path
	return iv.Set(path, modifiedData)
}

// IsNull checks if a path contains a null value
func (iv *IterableValue) IsNull(path string) bool {
	value := iv.Get(path)
	return value == nil
}

// IsEmpty checks if a path contains an empty value (empty string, empty array, empty object)
func (iv *IterableValue) IsEmpty(path string) bool {
	value := iv.Get(path)
	if value == nil {
		return true
	}

	switch v := value.(type) {
	case string:
		return v == ""
	case []any:
		return len(v) == 0
	case map[string]any:
		return len(v) == 0
	case map[any]any:
		return len(v) == 0
	default:
		return false
	}
}

// Length returns the length of an array or object at the given path
func (iv *IterableValue) Length(path string) int {
	value := iv.Get(path)
	if value == nil {
		return 0
	}

	switch v := value.(type) {
	case []any:
		return len(v)
	case map[string]any:
		return len(v)
	case map[any]any:
		return len(v)
	case string:
		return len(v)
	default:
		return 0
	}
}

// Keys returns the keys of an object at the given path
func (iv *IterableValue) Keys(path string) []string {
	value := iv.Get(path)
	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		return keys
	default:
		return nil
	}
}

// Values returns the values of an object or array at the given path
func (iv *IterableValue) Values(path string) []any {
	value := iv.Get(path)
	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case []any:
		return v
	case map[string]any:
		values := make([]any, 0, len(v))
		for _, val := range v {
			values = append(values, val)
		}
		return values
	default:
		return nil
	}
}

// NewIterableValue creates a new IterableValue
func NewIterableValue(data any, processor *Processor) *IterableValue {
	return &IterableValue{
		data:      data,
		processor: processor,
	}
}

// NewIterableValueWithIterator creates a new IterableValue with iterator reference
func NewIterableValueWithIterator(data any, processor *Processor, iterator *Iterator) *IterableValue {
	return &IterableValue{
		data:      data,
		processor: processor,
		iterator:  iterator,
	}
}

// String returns the JSON string representation of the data
func (iv *IterableValue) String() string {
	if iv.data == nil {
		return "null"
	}

	if iv.processor != nil {
		// Use the processor's ToJSON method for consistent formatting
		jsonStr, err := iv.processor.ToJsonString(iv.data)
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return jsonStr
	}

	// Fallback to fmt.Sprintf
	return fmt.Sprintf("%v", iv.data)
}

// Foreach iterates over JSON data and calls the callback for each item
// This matches the user's expected usage: json.Foreach(jsonStr, func(key, item any) { ... })
// Automatically detects nested calls and uses isolated processor to prevent state conflicts
func Foreach(jsonStr string, callback ForeachCallback, opts ...*ProcessorOptions) error {
	// Check if this is a nested call
	if isNestedCall() {
		// Use isolated processor for nested calls
		return ForeachNested(jsonStr, callback, opts...)
	}

	// Track nesting level
	enterNestedCall()
	defer exitNestedCall()

	processor := getDefaultProcessor()
	return processor.Foreach(jsonStr, callback, opts...)
}

// ForeachReturn iterates over JSON data and returns the modified JSON string
// Automatically detects nested calls and uses isolated processor to prevent state conflicts
func ForeachReturn(jsonStr string, callback ForeachCallback, opts ...*ProcessorOptions) (string, error) {
	// Check if this is a nested call
	if isNestedCall() {
		// Use isolated processor for nested calls
		return ForeachReturnNested(jsonStr, callback, opts...)
	}

	// Track nesting level
	enterNestedCall()
	defer exitNestedCall()

	processor := getDefaultProcessor()
	return processor.ForeachReturn(jsonStr, callback, opts...)
}

// ForeachNested iterates over JSON data with isolated processor instance for nested calls
// This prevents state conflicts when nesting Foreach calls
func ForeachNested(jsonStr string, callback ForeachCallback, opts ...*ProcessorOptions) error {
	// Create a new processor instance to avoid state conflicts
	processor := New()
	defer processor.Close()
	return processor.Foreach(jsonStr, callback, opts...)
}

// ForeachReturnNested iterates over JSON data and returns the modified JSON string
// Uses isolated processor instance to prevent state conflicts in nested calls
func ForeachReturnNested(jsonStr string, callback ForeachCallback, opts ...*ProcessorOptions) (string, error) {
	// Create a new processor instance to avoid state conflicts
	processor := New()
	defer processor.Close()
	return processor.ForeachReturn(jsonStr, callback, opts...)
}

// ForeachWithIterator iterates over JSON data with iterator access in callback
// Automatically detects nested calls and uses isolated processor to prevent state conflicts
func ForeachWithIterator(jsonStr string, callback ForeachCallbackWithIterator, opts ...*ProcessorOptions) error {
	// Check if this is a nested call
	if isNestedCall() {
		// Use isolated processor for nested calls
		return ForeachWithIteratorNested(jsonStr, callback, opts...)
	}

	// Track nesting level
	enterNestedCall()
	defer exitNestedCall()

	processor := getDefaultProcessor()
	return processor.ForeachWithIterator(jsonStr, callback, opts...)
}

// ForeachWithIteratorNested iterates over JSON data with iterator access using isolated processor
func ForeachWithIteratorNested(jsonStr string, callback ForeachCallbackWithIterator, opts ...*ProcessorOptions) error {
	// Create a new processor instance to avoid state conflicts
	processor := New()
	defer processor.Close()
	return processor.ForeachWithIterator(jsonStr, callback, opts...)
}

// ForeachWithPath iterates over JSON data at a specific path
// This allows iteration over a subset of the JSON structure
func ForeachWithPath(jsonStr string, path string, callback ForeachCallback, opts ...*ProcessorOptions) error {
	// Check if this is a nested call
	if isNestedCall() {
		// Use isolated processor for nested calls
		return ForeachWithPathNested(jsonStr, path, callback, opts...)
	}

	// Track nesting level
	enterNestedCall()
	defer exitNestedCall()

	processor := getDefaultProcessor()
	return processor.ForeachWithPath(jsonStr, path, callback, opts...)
}

// ForeachWithPathNested iterates over JSON data at a specific path using isolated processor
func ForeachWithPathNested(jsonStr string, path string, callback ForeachCallback, opts ...*ProcessorOptions) error {
	// Create a new processor instance to avoid state conflicts
	processor := New()
	defer processor.Close()
	return processor.ForeachWithPath(jsonStr, path, callback, opts...)
}

// Foreach iterates over JSON data using the processor
// The callback receives key and an IterableValue that supports Get() method
func (p *Processor) Foreach(jsonStr string, callback ForeachCallback, opts ...*ProcessorOptions) error {
	return p.ForeachWithIterator(jsonStr, func(key, value any, it *Iterator) {
		// Create an IterableValue that supports Get() method and iterator control
		iterableValue := NewIterableValueWithIterator(value, p, it)
		callback(key, iterableValue)
	}, opts...)
}

// ForeachReturn iterates over JSON data and returns the modified JSON string
func (p *Processor) ForeachReturn(jsonStr string, callback ForeachCallback, opts ...*ProcessorOptions) (string, error) {
	if err := p.checkClosed(); err != nil {
		return "", err
	}

	options, err := p.prepareOptions(opts...)
	if err != nil {
		return "", err
	}

	if err := p.validateInput(jsonStr); err != nil {
		return "", err
	}

	// Parse JSON
	var data any
	err = p.Parse(jsonStr, &data, opts...)
	if err != nil {
		return "", err
	}

	// Create iterator
	iterator := NewIterator(p, data, options)

	// Perform iteration with modifications
	err = p.iterateData(data, "", iterator, func(key, value any, it *Iterator) {
		// Create an IterableValue that supports Get() method and iterator control
		iterableValue := NewIterableValueWithIterator(value, p, it)
		callback(key, iterableValue)
	})

	if err != nil {
		return "", err
	}

	// Clean up deleted markers from the data
	cleanedData := p.cleanupDeletedMarkers(data)

	// Convert modified data back to JSON string
	return p.ToJsonString(cleanedData, opts...)
}

// ForeachWithIterator iterates over JSON data with iterator access
func (p *Processor) ForeachWithIterator(jsonStr string, callback ForeachCallbackWithIterator, opts ...*ProcessorOptions) error {
	if err := p.checkClosed(); err != nil {
		return err
	}

	options, err := p.prepareOptions(opts...)
	if err != nil {
		return err
	}

	if err := p.validateInput(jsonStr); err != nil {
		return err
	}

	// Parse JSON
	var data any
	err = p.Parse(jsonStr, &data, opts...)
	if err != nil {
		return err
	}

	// Create iterator
	iterator := NewIterator(p, data, options)

	// Start iteration
	return p.iterateData(data, "", iterator, callback)
}

// ForeachWithPath iterates over JSON data at a specific path
func (p *Processor) ForeachWithPath(jsonStr string, path string, callback ForeachCallback, opts ...*ProcessorOptions) error {
	if err := p.checkClosed(); err != nil {
		return err
	}

	options, err := p.prepareOptions(opts...)
	if err != nil {
		return err
	}

	if err := p.validateInput(jsonStr); err != nil {
		return err
	}

	// Get the data at the specified path
	targetData, err := p.Get(jsonStr, path)
	if err != nil {
		return fmt.Errorf("failed to get data at path '%s': %v", path, err)
	}

	if targetData == nil {
		return fmt.Errorf("no data found at path '%s'", path)
	}

	// Create iterator for the target data
	iterator := NewIterator(p, targetData, options)

	// Start iteration on the target data
	return p.iterateData(targetData, path, iterator, func(key, value any, it *Iterator) {
		// Create an IterableValue that supports Get() method and iterator control
		iterableValue := NewIterableValueWithIterator(value, p, it)
		callback(key, iterableValue)
	})
}

// iterateData performs the actual iteration over the data structure
func (p *Processor) iterateData(data any, basePath string, iterator *Iterator, callback ForeachCallbackWithIterator) error {
	switch v := data.(type) {
	case []any:
		// Iterate over array
		for i, item := range v {
			// Update iterator state
			iterator.currentKey = i
			iterator.currentValue = item
			iterator.currentData = v // Set currentData to the array for proper parent context
			iterator.currentPath = p.buildPath(basePath, strconv.Itoa(i))
			iterator.control = IteratorNormal

			// Call callback and check for control signals
			callback(i, item, iterator)

			// Check control flags
			switch iterator.control {
			case IteratorBreak:
				return nil
			case IteratorContinue:
				continue
			case IteratorNormal:
				// normal execution
			}
		}

	case map[string]any:
		// Create a snapshot of keys to avoid dynamic iteration issues
		// This prevents newly added keys during iteration from being processed
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}

		// Iterate over the fixed key snapshot
		for _, key := range keys {
			// Get current value (may have been modified during iteration)
			value, exists := v[key]
			if !exists {
				// Key was deleted during iteration, skip it
				continue
			}

			// Update iterator state
			iterator.currentKey = key
			iterator.currentValue = value
			iterator.currentData = v // Set currentData to the object for proper parent context
			iterator.currentPath = p.buildPath(basePath, key)
			iterator.control = IteratorNormal

			// Call callback and check for control signals
			callback(key, value, iterator)

			// Check control flags
			switch iterator.control {
			case IteratorBreak:
				return nil
			case IteratorContinue:
				continue
			case IteratorNormal:
				// normal execution
			}
		}

	case map[any]any:
		// Create a snapshot of keys to avoid dynamic iteration issues
		// This prevents newly added keys during iteration from being processed
		keys := make([]any, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}

		// Iterate over the fixed key snapshot
		for _, key := range keys {
			// Get current value (may have been modified during iteration)
			value, exists := v[key]
			if !exists {
				// Key was deleted during iteration, skip it
				continue
			}

			// Update iterator state
			iterator.currentKey = key
			iterator.currentValue = value
			iterator.currentData = v // Set currentData to the object for proper parent context
			iterator.currentPath = p.buildPath(basePath, fmt.Sprintf("%v", key))
			iterator.control = IteratorNormal

			// Call callback and check for control signals
			callback(key, value, iterator)

			// Check control flags
			switch iterator.control {
			case IteratorBreak:
				return nil
			case IteratorContinue:
				continue
			case IteratorNormal:
				// normal execution
			}
		}

	default:
		// For non-iterable types, treat as single item
		iterator.currentKey = ""
		iterator.currentValue = data
		iterator.currentPath = basePath
		iterator.control = IteratorNormal

		callback("", data, iterator)
	}

	return nil
}

// SetMultiple sets multiple values using path notation with a map of path-value pairs
func (iv *IterableValue) SetMultiple(updates map[string]any) error {
	if iv.processor == nil {
		return fmt.Errorf("no processor available")
	}

	if len(updates) == 0 {
		return nil // No updates to apply
	}

	// Determine the correct target object for setting values
	var targetObject any
	var adjustedUpdates = make(map[string]any)

	if iv.iterator != nil {
		// We have iterator context, use the appropriate data source
		switch iv.iterator.currentValue.(type) {
		case map[string]any, map[any]any:
			// Current value is an object, search within it
			targetObject = iv.iterator.currentValue

			// Adjust paths to be relative to the current object
			for path, value := range updates {
				if iv.iterator.currentPath != "" && strings.HasPrefix(path, iv.iterator.currentPath+".") {
					// Remove the current path prefix from the search path
					adjustedPath := strings.TrimPrefix(path, iv.iterator.currentPath+".")
					adjustedUpdates[adjustedPath] = value
				} else if path == iv.iterator.currentPath {
					// Cannot set the root value of current item
					return fmt.Errorf("cannot set root value of current item with path '%s'", path)
				} else {
					adjustedUpdates[path] = value
				}
			}
		case []any:
			// Current value is an array, search within it directly
			targetObject = iv.iterator.currentValue
			adjustedUpdates = updates
		default:
			// Current value is a primitive, but we might be iterating over object properties
			// In this case, we need to search from the parent object
			if iv.iterator.currentData != nil {
				switch iv.iterator.currentData.(type) {
				case map[string]any, map[any]any:
					targetObject = iv.iterator.currentData
				default:
					targetObject = iv.data
				}
			} else {
				targetObject = iv.data
			}
			adjustedUpdates = updates
		}
	} else {
		// No iterator context, use the data directly
		targetObject = iv.data
		adjustedUpdates = updates
	}

	// Apply all updates
	var lastError error
	successCount := 0

	for path, value := range adjustedUpdates {
		err := iv.processor.setValueAtPathWithOptions(targetObject, path, value, true) // Always create paths in iterator context
		if err != nil {
			lastError = fmt.Errorf("failed to set path '%s': %v", path, err)
			// Continue with other updates even if one fails
		} else {
			successCount++
		}
	}

	// If no updates were successful and we have errors, return the last error
	if successCount == 0 && lastError != nil {
		return lastError
	}

	return nil
}

// buildPath builds a path string from base path and segment
func (p *Processor) buildPath(basePath, segment string) string {
	if basePath == "" {
		return segment
	}
	return basePath + "." + segment
}

// cleanupDeletedMarkers removes deleted markers from the data structure

// DeepExtractionResult represents the result of a deep extraction operation
type DeepExtractionResult struct {
	Values []any
	Paths  []string // Optional: track the paths where values were found
}

// handleDeepExtraction handles multiple consecutive extraction operations and mixed operations
// This solves the problem where a{g}{name} doesn't work as expected
func (p *Processor) handleDeepExtraction(data any, segments []PathSegment, startIndex int) (any, error) {
	if startIndex >= len(segments) {
		return data, nil
	}

	// Check if this is a single extraction followed by other operations
	if startIndex < len(segments) && segments[startIndex].TypeString() == "extract" {
		// Check if the next segment is not an extraction (mixed operation case)
		if startIndex+1 < len(segments) && segments[startIndex+1].TypeString() != "extract" {
			return p.handleMixedExtractionOperations(data, segments, startIndex)
		}
	}

	// Find all consecutive extract operations
	var extractSegments []PathSegment
	currentIndex := startIndex

	for currentIndex < len(segments) && segments[currentIndex].TypeString() == "extract" {
		extractSegments = append(extractSegments, segments[currentIndex])
		currentIndex++
	}

	if len(extractSegments) == 0 {
		return data, fmt.Errorf("no extract segments found")
	}

	// Apply consecutive extractions with proper flattening
	result, err := p.performConsecutiveExtractions(data, extractSegments)
	if err != nil {
		return nil, err
	}

	// If there are remaining segments after extractions, handle them properly
	if currentIndex < len(segments) {
		remainingSegments := segments[currentIndex:]

		// Check if the remaining segments are array operations (slice/index)
		// These need special handling for post-extraction operations
		if len(remainingSegments) > 0 {
			firstRemaining := remainingSegments[0]
			switch firstRemaining.TypeString() {
			case "slice":
				// For post-extraction slicing, we need to handle the structure correctly
				// If the extraction result is a flattened array, apply slice directly
				// If it's a structured array (array of arrays), we need different handling
				slicedResult := p.handlePostExtractionSliceWithStructureAwareness(result, firstRemaining, extractSegments)
				if slicedResult == nil {
					return nil, fmt.Errorf("failed to apply slice after extraction")
				}

				// If there are more segments after the slice, continue navigation
				if len(remainingSegments) > 1 {
					return p.navigateToPathWithSegments(slicedResult, remainingSegments[1:])
				}
				return slicedResult, nil

			case "array":
				// Apply array index operation to the extraction result
				indexedResult := p.handlePostExtractionArrayAccess(result, firstRemaining)
				if indexedResult == nil {
					return nil, fmt.Errorf("failed to apply array index after extraction")
				}

				// If there are more segments after the index, continue navigation
				if len(remainingSegments) > 1 {
					return p.navigateToPathWithSegments(indexedResult, remainingSegments[1:])
				}
				return indexedResult, nil

			default:
				// For other segment types, use regular navigation
				return p.navigateToPathWithSegments(result, remainingSegments)
			}
		}
	}

	return result, nil
}

// handlePostExtractionSliceWithStructureAwareness handles slice operations after extraction with structure awareness
func (p *Processor) handlePostExtractionSliceWithStructureAwareness(data any, sliceSegment PathSegment, extractSegments []PathSegment) any {
	arr, ok := data.([]any)
	if !ok {
		return nil
	}

	// Get slice parameters from segment
	start := 0
	end := len(arr)
	step := 1

	if sliceSegment.Start != nil {
		start = *sliceSegment.Start
	}
	if sliceSegment.End != nil {
		end = *sliceSegment.End
	}
	if sliceSegment.Step != nil {
		step = *sliceSegment.Step
	}

	// Check if any extraction segment is flat
	hasFlat := false
	for _, seg := range extractSegments {
		if seg.IsFlat {
			hasFlat = true
			break
		}
	}

	if hasFlat {
		return p.applySliceToArrayWithContext(arr, start, end, step, sliceSegment.String())
	} else {
		slicedResults := p.applySliceToArrayWithContext(arr, start, end, step, sliceSegment.String())
		return slicedResults
	}
}

// applySliceToArrayWithContext applies slice operation with segment context to distinguish open vs explicit negative slices
func (p *Processor) applySliceToArrayWithContext(arr []any, start, end, step int, segmentValue string) []any {
	// Handle negative indices
	if start < 0 {
		start = len(arr) + start
	}

	// Handle negative end index - distinguish between open slice and explicit negative
	if end < 0 {
		// Check if this is an open slice by examining the segment value
		isOpenSlice := strings.HasSuffix(segmentValue, ":") || strings.HasSuffix(segmentValue, ":]")

		if isOpenSlice {
			// Open slice like "[-1:]" - end should be array length
			end = len(arr)
		} else {
			// Explicit negative end index like "[-3:-1]" - convert to positive
			end = len(arr) + end
		}
	}

	// Normalize bounds
	if start < 0 {
		start = 0
	}
	if end > len(arr) {
		end = len(arr)
	}
	if start > end {
		return []any{}
	}

	// Apply step if specified
	if step <= 1 {
		return arr[start:end]
	}

	// Handle step > 1
	var result []any
	for i := start; i < end; i += step {
		if i < len(arr) {
			result = append(result, arr[i])
		}
	}
	return result
}

// handleMixedExtractionOperations handles extraction operations mixed with other operations
// This handles cases like {field}[0:2]{another} where extraction is followed by slice then extraction
func (p *Processor) handleMixedExtractionOperations(data any, segments []PathSegment, startIndex int) (any, error) {
	current := data

	for i := startIndex; i < len(segments); i++ {
		segment := segments[i]

		switch segment.TypeString() {
		case "extract":
			// Apply extraction - for mixed operations, we never preserve structure
			// to avoid extra wrapping that causes issues with subsequent slice operations
			result, err := p.performSingleExtraction(current, segment)
			if err != nil {
				return nil, fmt.Errorf("extraction failed at segment %d: %w", i, err)
			}
			current = result

		case "slice":
			if arr, ok := current.([]any); ok {
				start := 0
				end := len(arr)
				step := 1

				if segment.Start != nil {
					start = *segment.Start
				}
				if segment.End != nil {
					end = *segment.End
				}
				if segment.Step != nil {
					step = *segment.Step
				}

				slicedResult := p.applySliceToArrayWithContext(arr, start, end, step, segment.String())
				current = slicedResult
			} else {
				return nil, fmt.Errorf("cannot apply slice to non-array type %T", current)
			}

		case "array":
			// Apply array index operation
			current = p.handlePostExtractionArrayAccess(current, segment)
			if current == nil {
				return nil, fmt.Errorf("array index operation failed")
			}

		default:
			result := p.handlePropertyAccess(current, segment.Key)
			if !result.Exists {
				return nil, ErrPathNotFound
			}
			current = result.Value
		}
	}

	return current, nil
}

// handleMidChainSliceAfterExtraction handles slice operations in the middle of extraction chains
// This is needed for paths like {field}[0:2]{another} where we need to slice each extracted array
func (p *Processor) handleMidChainSliceAfterExtraction(data any, sliceSegment PathSegment, segmentIndex int, allSegments []PathSegment) any {
	if _, ok := data.([]any); !ok {
		return nil
	}

	// We need to reconstruct the original array structure to apply the slice correctly
	// The problem is that extraction flattens arrays, but for mid-chain slicing,
	// we need to know which items came from which original array

	// For now, let's implement a simpler approach: apply the slice to the flattened array
	// and then continue with the remaining operations
	sliced := p.handlePostExtractionArraySlice(data, sliceSegment)
	return sliced
}

// performConsecutiveExtractions performs multiple consecutive extractions with proper flattening
func (p *Processor) performConsecutiveExtractions(data any, extractSegments []PathSegment) (any, error) {
	current := data

	for i, segment := range extractSegments {
		// Determine structure preservation based on context
		// - If this is the last extraction and no flat: prefix, preserve structure for potential post-operations
		// - If flat: prefix is used, always flatten
		// - For intermediate extractions, preserve structure to maintain nesting
		isLastExtraction := i == len(extractSegments)-1
		preserveStructure := !isLastExtraction && !segment.IsFlat

		// Special handling for flat extractions
		if segment.IsFlat {
			result, err := p.performFlatExtraction(current, segment)
			if err != nil {
				return nil, fmt.Errorf("flat extraction failed at segment %d (%s): %w", i, segment.Key, err)
			}
			current = result
		} else {
			result, err := p.performSingleExtractionWithStructure(current, segment, preserveStructure)
			if err != nil {
				return nil, fmt.Errorf("extraction failed at segment %d (%s): %w", i, segment.Key, err)
			}
			current = result
		}
	}

	return current, nil
}

// performFlatExtraction performs flat extraction with proper flattening
func (p *Processor) performFlatExtraction(data any, segment PathSegment) (any, error) {
	switch data := data.(type) {
	case []any:
		return p.extractFromArrayWithFlat(data, segment.Key, true)
	case map[string]any:
		// For objects, flat extraction is the same as regular extraction
		return p.extractFromObject(data, segment.Key)
	default:
		return nil, nil
	}
}

// flattenExtractionResult flattens nested arrays from extraction results
func (p *Processor) flattenExtractionResult(data any) any {
	arr, ok := data.([]any)
	if !ok {
		return data
	}

	var flattened []any
	for _, item := range arr {
		if subArr, ok := item.([]any); ok {
			// If item is an array, flatten it
			flattened = append(flattened, subArr...)
		} else {
			// If item is not an array, add it directly
			flattened = append(flattened, item)
		}
	}

	return flattened
}

// performSingleExtraction performs a single extraction operation
func (p *Processor) performSingleExtraction(data any, segment PathSegment) (any, error) {
	switch data := data.(type) {
	case []any:
		return p.extractFromArrayWithFlat(data, segment.Key, segment.IsFlat)
	case map[string]any:
		return p.extractFromObject(data, segment.Key)
	default:
		// Return nil for boundary case - trying to extract from non-extractable type
		return nil, nil
	}
}

// performSingleExtractionWithStructure performs extraction with optional structure preservation
func (p *Processor) performSingleExtractionWithStructure(data any, segment PathSegment, preserveStructure bool) (any, error) {
	switch data := data.(type) {
	case []any:
		if preserveStructure {
			return p.extractFromArrayPreservingStructureWithFlat(data, segment.Key, segment.IsFlat)
		}
		return p.extractFromArrayWithFlat(data, segment.Key, segment.IsFlat)
	case map[string]any:
		return p.extractFromObject(data, segment.Key)
	default:
		// Return nil for boundary case - trying to extract from non-extractable type
		return nil, nil
	}
}

// extractFromArray extracts values from an array with nested support
func (p *Processor) extractFromArray(data any, fieldName string) (any, error) {
	return p.extractFromArrayWithFlat(data, fieldName, false)
}

// extractFromArrayWithFlat extracts values from an array with optional flattening
func (p *Processor) extractFromArrayWithFlat(data any, fieldName string, isFlat bool) (any, error) {
	arr, ok := data.([]any)
	if !ok {
		return nil, fmt.Errorf("expected array, got %T", data)
	}

	var results []any
	for _, item := range arr {
		switch v := item.(type) {
		case map[string]any:
			// Extract from object
			if value, exists := v[fieldName]; exists {
				if isFlat {
					// For flat extraction, recursively flatten arrays
					p.flattenValue(value, &results)
				} else {
					// For regular extraction, preserve array structure (don't flatten)
					results = append(results, value)
				}
			} else if isFlat {
				// For flat extraction, when field doesn't exist, add empty array to preserve structure
				results = append(results, []any{})
			}
		case []any:
			// If item is an array, recursively extract from it
			subResult, err := p.extractFromArrayWithFlat(item, fieldName, isFlat)
			if err == nil {
				if subArr, ok := subResult.([]any); ok {
					if isFlat {
						// For flat extraction, flatten recursive results
						// But preserve empty arrays to maintain structure
						if len(subArr) == 0 {
							results = append(results, []any{})
						} else {
							results = append(results, subArr...)
						}
					} else {
						// For non-flat extraction, preserve structure - don't flatten
						results = append(results, subArr)
					}
				} else {
					results = append(results, subResult)
				}
			} else if isFlat {
				// For flat extraction on empty arrays, add empty array to preserve structure
				results = append(results, []any{})
			}
		}
	}

	return results, nil
}

// extractFromArrayPreservingStructure extracts values from an array while preserving array structure
// This is used when the extraction is followed by slice operations that need to operate on individual arrays
func (p *Processor) extractFromArrayPreservingStructure(data any, fieldName string) (any, error) {
	arr, ok := data.([]any)
	if !ok {
		return nil, fmt.Errorf("expected array, got %T", data)
	}

	var results []any
	for _, item := range arr {
		switch v := item.(type) {
		case map[string]any:
			// Extract from object - preserve as individual array if the value is an array
			if value, exists := v[fieldName]; exists {
				if valueArr, ok := value.([]any); ok {
					// Keep as separate array instead of flattening
					results = append(results, valueArr)
				} else {
					// Single value - wrap in array to maintain structure
					results = append(results, []any{value})
				}
			}
		case []any:
			// If item is an array, recursively extract from it
			subResult, err := p.extractFromArrayPreservingStructure(item, fieldName)
			if err == nil {
				if subArr, ok := subResult.([]any); ok {
					results = append(results, subArr...)
				} else {
					results = append(results, subResult)
				}
			}
		}
	}

	return results, nil
}

// extractFromArrayPreservingStructureWithFlat extracts values from an array while preserving array structure with flat support
func (p *Processor) extractFromArrayPreservingStructureWithFlat(data any, fieldName string, isFlat bool) (any, error) {
	arr, ok := data.([]any)
	if !ok {
		return nil, fmt.Errorf("expected array, got %T", data)
	}

	var results []any
	for _, item := range arr {
		switch v := item.(type) {
		case map[string]any:
			// Extract from object - preserve as individual array if the value is an array
			if value, exists := v[fieldName]; exists {
				if isFlat {
					// For flat extraction with structure preservation, still flatten but wrap in array
					var flatResults []any
					p.flattenValue(value, &flatResults)
					results = append(results, flatResults)
				} else {
					// Original behavior
					if valueArr, ok := value.([]any); ok {
						// Keep as separate array instead of flattening
						results = append(results, valueArr)
					} else {
						// Single value - wrap in array to maintain structure
						results = append(results, []any{value})
					}
				}
			}
		case []any:
			// If item is an array, recursively extract from it with improved depth handling
			subResult, err := p.extractFromArrayPreservingStructureWithFlat(item, fieldName, isFlat)
			if err == nil {
				if subArr, ok := subResult.([]any); ok {
					// For deep nesting (3+ levels), maintain proper structure
					if isFlat {
						// Flatten all levels for flat extraction
						for _, subItem := range subArr {
							p.flattenValue(subItem, &results)
						}
					} else {
						// Preserve structure for non-flat extraction
						results = append(results, subArr...)
					}
				} else {
					results = append(results, subResult)
				}
			}
		}
	}

	return results, nil
}

// extractFromObject extracts values from an object
func (p *Processor) extractFromObject(data any, fieldName string) (any, error) {
	obj, ok := data.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected object, got %T", data)
	}

	if value, exists := obj[fieldName]; exists {
		return value, nil
	}

	return nil, fmt.Errorf("field '%s' not found", fieldName)
}

// navigateToPathWithSegments navigates through remaining path segments
func (p *Processor) navigateToPathWithSegments(data any, segments []PathSegment) (any, error) {
	current := data

	for i, segment := range segments {
		switch segment.TypeString() {
		case "property":
			result := p.handlePropertyAccess(current, segment.Key)
			if !result.Exists {
				return nil, ErrPathNotFound
			}
			current = result.Value

		case "array":
			result := p.handleArrayAccess(current, segment)
			if !result.Exists {
				return nil, ErrPathNotFound
			}
			current = result.Value

		case "slice":
			result := p.handleArraySlice(current, segment)
			if !result.Exists {
				return nil, ErrPathNotFound
			}
			current = result.Value

		case "extract":
			// If we encounter another extraction, handle it with deep extraction
			return p.handleDeepExtraction(current, segments, i)

		default:
			return nil, fmt.Errorf("unsupported segment type: %s", segment.TypeString())
		}
	}

	return current, nil
}

// extractFieldName extracts field name from extraction syntax
func (p *Processor) extractFieldName(segment string) string {
	// Remove braces from {fieldName}
	if strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
		return segment[1 : len(segment)-1]
	}
	return segment
}

// ConsecutiveExtractionGroup represents a group of consecutive extraction operations
type ConsecutiveExtractionGroup struct {
	StartIndex int
	EndIndex   int
	Segments   []PathSegment
}

// isDeepExtractionPath checks if a path contains consecutive extraction operations
func (p *Processor) isDeepExtractionPath(segments []PathSegment) bool {
	consecutiveCount := 0
	for _, segment := range segments {
		if segment.TypeString() == "extract" {
			consecutiveCount++
			if consecutiveCount >= 2 {
				return true
			}
		} else {
			consecutiveCount = 0
		}
	}
	return false
}

// preprocessPathForDeepExtraction preprocesses path segments for deep extraction
func (p *Processor) preprocessPathForDeepExtraction(segments []PathSegment) []PathSegment {
	// For now, just return the segments as-is
	// This can be extended for more complex preprocessing
	return segments
}

// detectConsecutiveExtractions detects groups of consecutive extraction operations
func (p *Processor) detectConsecutiveExtractions(segments []PathSegment) []ConsecutiveExtractionGroup {
	var groups []ConsecutiveExtractionGroup
	var currentGroup *ConsecutiveExtractionGroup

	for i, segment := range segments {
		if segment.TypeString() == "extract" {
			if currentGroup == nil {
				currentGroup = &ConsecutiveExtractionGroup{
					StartIndex: i,
					Segments:   []PathSegment{segment},
				}
			} else {
				currentGroup.Segments = append(currentGroup.Segments, segment)
			}
		} else {
			if currentGroup != nil && len(currentGroup.Segments) >= 1 { // Changed from >= 2 to >= 1
				currentGroup.EndIndex = i - 1
				groups = append(groups, *currentGroup)
			}
			currentGroup = nil
		}
	}

	// Handle case where consecutive extractions are at the end
	if currentGroup != nil && len(currentGroup.Segments) >= 1 { // Changed from >= 2 to >= 1
		currentGroup.EndIndex = len(segments) - 1
		groups = append(groups, *currentGroup)
	}

	return groups
}

// navigateWithDeepExtraction handles navigation with deep extraction groups
func (p *Processor) navigateWithDeepExtraction(data any, segments []PathSegment, groups []ConsecutiveExtractionGroup) (any, error) {
	if len(groups) == 0 {
		return p.navigateToPathWithSegments(data, segments)
	}

	// Handle the first group and any segments after it
	firstGroup := groups[0]

	// Navigate to the start of the first extraction group
	current := data
	for i := 0; i < firstGroup.StartIndex; i++ {
		result, err := p.navigateToPathWithSegments(current, segments[i:i+1])
		if err != nil {
			return nil, err
		}
		current = result
	}

	// Handle the deep extraction with all remaining segments
	// This includes both the extraction segments and any post-extraction segments
	remainingSegments := segments[firstGroup.StartIndex:]
	return p.handleDeepExtraction(current, remainingSegments, 0)
}
