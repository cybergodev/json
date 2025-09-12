package json

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cybergodev/json/internal"
)

// IteratorControl represents control flags for iteration
type IteratorControl int

const (
	IteratorNormal   IteratorControl = iota // normal execution
	IteratorContinue                        // Continue current item and continue
	IteratorBreak                           // Break entire iteration
)

// Nested call tracking to prevent state conflicts with enhanced memory leak prevention
var (
	nestedCallTracker = make(map[uint64]int) // goroutine ID -> nesting level
	nestedCallMutex   sync.RWMutex
	lastCleanupTime   int64        // Unix timestamp of last cleanup
	maxTrackerSize    = 5000       // Reduced maximum to prevent memory bloat
	cleanupInterval   = int64(180) // Cleanup every 3 minutes (reduced from 5)
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

// cleanupDeadGoroutines removes entries for goroutines that no longer exist with enhanced logic
func cleanupDeadGoroutines() {
	// Enhanced cleanup strategy to prevent memory leaks
	trackerSize := len(nestedCallTracker)

	if trackerSize > maxTrackerSize {
		// Aggressive cleanup: clear entire tracker
		nestedCallTracker = make(map[uint64]int, maxTrackerSize/2)
	} else if trackerSize > maxTrackerSize/2 {
		// Moderate cleanup: remove entries with zero nesting level
		for gid, level := range nestedCallTracker {
			if level <= 0 {
				delete(nestedCallTracker, gid)
			}
		}

		// If still too large, clear everything
		if len(nestedCallTracker) > maxTrackerSize*3/4 {
			nestedCallTracker = make(map[uint64]int, maxTrackerSize/2)
		}
	}
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

// Continue skips the current iteration elegantly without requiring return
func (it *Iterator) Continue() {
	panic(IteratorControlSignal{Type: IteratorContinue})
}

// Break stops the entire iteration elegantly without requiring return
func (it *Iterator) Break() {
	panic(IteratorControlSignal{Type: IteratorBreak})
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

	// Use compatibility layer - new architecture handles all paths uniformly
	if iv.processor.needsLegacyComplexHandling(adjustedPath) {
		result, err = iv.processor.navigateToPath(searchContext, adjustedPath)
	} else {
		unifiedProcessor := NewRecursiveProcessor(iv.processor)
		result, err = unifiedProcessor.ProcessRecursively(searchContext, adjustedPath, OpGet, nil)
	}

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
		arrayUtils := internal.NewArrayUtils()
		start, end, step := arrayUtils.ParseSliceFromSegment(segment.Value)
		if start == -999999 { // Invalid slice
			return nil, nil
		}

		// Use unified array slicing
		startPtr := &start
		endPtr := &end
		stepPtr := &step
		if end == -1 {
			endPtr = nil // Use default end
		}

		result := arrayUtils.PerformArraySlice(arr, startPtr, endPtr, stepPtr)
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

// =============================================================================
// GET WITH DEFAULT VALUE METHODS FOR ITERABLEVALUE
// =============================================================================

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

	// If the result is empty string and the original value was nil, return default
	if res == "" && rawValue == nil {
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

// =============================================================================
// GLOBAL FUNCTIONS FOR TYPED DEFAULT VALUES WITH ITERABLEVALUE
// =============================================================================

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

	// Use compatibility layer - new architecture handles all paths uniformly
	if iv.processor.needsLegacyComplexHandling(adjustedPath) {
		result, err = iv.processor.navigateToPath(searchContext, adjustedPath)
	} else {
		unifiedProcessor := NewRecursiveProcessor(iv.processor)
		result, err = unifiedProcessor.ProcessRecursively(searchContext, adjustedPath, OpGet, nil)
	}

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

// Helper functions for type conversion
func convertToInt(value any) (int, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	case json.Number:
		// Handle json.Number to preserve original format
		if i, err := v.Int64(); err == nil {
			return int(i), nil
		}
		// Try as float if integer conversion fails
		if f, err := v.Float64(); err == nil {
			return int(f), nil
		}
		return 0, fmt.Errorf("cannot convert json.Number '%s' to int", string(v))
	case string:
		if i, err := strconv.Atoi(v); err == nil {
			return i, nil
		}
		return 0, fmt.Errorf("cannot convert string '%s' to int", v)
	default:
		return 0, fmt.Errorf("cannot convert %T to int", v)
	}
}

func convertToInt64(value any) (int64, error) {
	switch v := value.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case float64:
		return int64(v), nil
	case json.Number:
		// Handle json.Number to preserve original format
		if i, err := v.Int64(); err == nil {
			return i, nil
		}
		// Try as float if integer conversion fails
		if f, err := v.Float64(); err == nil {
			return int64(f), nil
		}
		return 0, fmt.Errorf("cannot convert json.Number '%s' to int64", string(v))
	case string:
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return i, nil
		}
		return 0, fmt.Errorf("cannot convert string '%s' to int64", v)
	default:
		return 0, fmt.Errorf("cannot convert %T to int64", v)
	}
}

func convertToFloat64(value any) (float64, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case json.Number:
		// Handle json.Number to preserve original format
		if f, err := v.Float64(); err == nil {
			return f, nil
		}
		return 0, fmt.Errorf("cannot convert json.Number '%s' to float64", string(v))
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f, nil
		}
		return 0, fmt.Errorf("cannot convert string '%s' to float64", v)
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", v)
	}
}

func convertToString(value any) string {
	// Handle nil values specially - return empty string instead of "<nil>"
	if value == nil {
		return ""
	}

	switch v := value.(type) {
	case string:
		return v
	case json.Number:
		// Handle json.Number to preserve original format
		return string(v)
	case int, int64, float64, bool:
		return fmt.Sprintf("%v", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func convertToBool(value any) (bool, error) {
	switch v := value.(type) {
	case bool:
		return v, nil
	case json.Number:
		// Handle json.Number to preserve original format
		if f, err := v.Float64(); err == nil {
			return f != 0, nil
		}
		return false, fmt.Errorf("cannot convert json.Number '%s' to bool", string(v))
	case string:
		if b, err := strconv.ParseBool(v); err == nil {
			return b, nil
		}
		return false, fmt.Errorf("cannot convert string '%s' to bool", v)
	case int, int64:
		return fmt.Sprintf("%v", v) != "0", nil
	case float64:
		return v != 0, nil
	default:
		return false, fmt.Errorf("cannot convert %T to bool", v)
	}
}

func convertToArray(value any) ([]any, error) {
	switch v := value.(type) {
	case []any:
		return v, nil
	default:
		return nil, fmt.Errorf("cannot convert %T to []any", v)
	}
}

func convertToObject(value any) (map[string]any, error) {
	switch v := value.(type) {
	case map[string]any:
		return v, nil
	default:
		return nil, fmt.Errorf("cannot convert %T to map[string]any", v)
	}
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

			// Call callback with panic/recover for elegant control
			func() {
				defer func() {
					if r := recover(); r != nil {
						if signal, ok := r.(IteratorControlSignal); ok {
							iterator.control = signal.Type
						} else {
							// Re-panic if it's not our control signal
							panic(r)
						}
					}
				}()
				callback(i, item, iterator)
			}()

			// Check control flags
			switch iterator.control {
			case IteratorBreak:
				return nil
			case IteratorContinue:
				continue
			case IteratorNormal:
				//  normal execution
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

			// Call callback with panic/recover for elegant control
			func() {
				defer func() {
					if r := recover(); r != nil {
						if signal, ok := r.(IteratorControlSignal); ok {
							iterator.control = signal.Type
						} else {
							// Re-panic if it's not our control signal
							panic(r)
						}
					}
				}()
				callback(key, value, iterator)
			}()

			// Check control flags
			switch iterator.control {
			case IteratorBreak:
				return nil
			case IteratorContinue:
				continue
			case IteratorNormal:
				//  normal execution
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

			// Call callback with panic/recover for elegant control
			func() {
				defer func() {
					if r := recover(); r != nil {
						if signal, ok := r.(IteratorControlSignal); ok {
							iterator.control = signal.Type
						} else {
							// Re-panic if it's not our control signal
							panic(r)
						}
					}
				}()
				callback(key, value, iterator)
			}()

			// Check control flags
			switch iterator.control {
			case IteratorBreak:
				return nil
			case IteratorContinue:
				continue
			case IteratorNormal:
				//  normal execution
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

	if updates == nil || len(updates) == 0 {
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
