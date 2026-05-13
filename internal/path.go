package internal

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================================
// GLOBAL PATH SEGMENT CACHE
// PERFORMANCE v2: Uses sync.Map for lock-free reads with size limit and LRU eviction
// SECURITY: Thread-safe by design using sync.Map
// FIX: Added size limit to prevent memory leak from unbounded cache growth
// ============================================================================

// Path cache configuration
const (
	pathCacheMaxPathLength = 256   // Maximum path length to cache
	pathCacheMaxSize       = 10000 // Maximum cached paths
	pathCacheEvictCount    = 1000  // Number of entries to evict when limit reached
)

// pathCacheEntry wraps cached segments with access time for LRU eviction
type pathCacheEntry struct {
	segments   []PathSegment
	lastAccess int64
}

// pathSegmentCache stores parsed path segments for reuse
// Uses sync.Map for lock-free concurrent reads
var pathSegmentCache sync.Map

// pathCacheSize tracks the current number of cached paths
var pathCacheSize int64

// pathCacheMu protects size counter during eviction
var pathCacheMu sync.Mutex

// getCachedPathSegments retrieves cached path segments if available
// PERFORMANCE: Lock-free read using sync.Map
// SECURITY FIX: Access time update now uses atomic Store to ensure visibility
func getCachedPathSegments(path string) ([]PathSegment, bool) {
	if len(path) > pathCacheMaxPathLength || len(path) == 0 {
		return nil, false
	}
	if v, ok := pathSegmentCache.Load(path); ok {
		entry, ok := v.(pathCacheEntry)
		if !ok {
			return nil, false
		}
		// PERFORMANCE: Update access time only every ~1 second to avoid
		// time.Now() syscall on every cache hit. Approximate LRU is sufficient.
		now := time.Now().UnixNano()
		if now-entry.lastAccess > int64(time.Second) {
			pathSegmentCache.Store(path, pathCacheEntry{
				segments:   entry.segments,
				lastAccess: now,
			})
		}
		return entry.segments, true
	}
	return nil, false
}

// setCachedPathSegments stores path segments in cache with size limit
// PERFORMANCE: Only stores if path is reasonably sized
// FIX: Added size limit and prevent unbounded memory growth
func setCachedPathSegments(path string, segments []PathSegment) {
	if len(path) > pathCacheMaxPathLength || len(path) == 0 || len(segments) == 0 {
		return
	}

	// Check cache size limit and evict if needed
	if atomic.LoadInt64(&pathCacheSize) >= pathCacheMaxSize {
		evictPathCacheEntries()
	}

	// Store a copy to prevent mutation
	copied := make([]PathSegment, len(segments))
	copy(copied, segments)

	// Wrap in entry for LRU tracking
	entry := pathCacheEntry{
		segments:   copied,
		lastAccess: time.Now().UnixNano(),
	}

	// Only increment counter for new entries, not overwrites
	if _, loaded := pathSegmentCache.LoadOrStore(path, entry); !loaded {
		atomic.AddInt64(&pathCacheSize, 1)
	} else {
		pathSegmentCache.Store(path, entry) // Update existing entry with fresh timestamp
	}
}

// evictPathCacheEntries removes oldest entries when cache is full
// Uses LRU strategy based on lastAccess time
func evictPathCacheEntries() {
	pathCacheMu.Lock()
	defer pathCacheMu.Unlock()

	// Check again after acquiring lock
	if atomic.LoadInt64(&pathCacheSize) < pathCacheMaxSize {
		return
	}

	// Collect entries with access times for LRU eviction
	type evictionCandidate struct {
		path       string
		lastAccess int64
	}
	candidates := make([]evictionCandidate, 0, pathCacheMaxSize/10) // Estimate

	pathSegmentCache.Range(func(key, value any) bool {
		if entry, ok := value.(pathCacheEntry); ok {
			candidates = append(candidates, evictionCandidate{
				path:       func() string { s, _ := key.(string); return s }(),
				lastAccess: entry.lastAccess,
			})
		}
		return true
	})

	// Sort by access time (oldest first)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].lastAccess < candidates[j].lastAccess
	})

	// Remove oldest entries
	evictCount := pathCacheEvictCount
	if evictCount > len(candidates) {
		evictCount = len(candidates)
	}

	for i := 0; i < evictCount; i++ {
		pathSegmentCache.Delete(candidates[i].path)
	}

	atomic.AddInt64(&pathCacheSize, -int64(evictCount))
}

// EscapeJSONPointer escapes special characters for JSON Pointer
// Uses single-pass algorithm to avoid multiple allocations
func EscapeJSONPointer(s string) string {
	// Fast path: check if escaping is needed
	hasTilde := false
	hasSlash := false
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '~':
			hasTilde = true
		case '/':
			hasSlash = true
		}
	}
	if !hasTilde && !hasSlash {
		return s // No allocation for simple strings
	}

	// Single-pass escaping with strings.Builder
	var sb strings.Builder
	sb.Grow(len(s) + 4) // Pre-allocate with some extra space for escapes

	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '~':
			sb.WriteString("~0")
		case '/':
			sb.WriteString("~1")
		default:
			sb.WriteByte(s[i])
		}
	}
	return sb.String()
}

// UnescapeJSONPointer unescapes JSON Pointer special characters
// Uses single-pass algorithm to avoid multiple allocations
func UnescapeJSONPointer(s string) string {
	// Fast path: check if unescaping is needed
	hasEscape := false
	for i := 0; i < len(s); i++ {
		if s[i] == '~' && i+1 < len(s) && (s[i+1] == '0' || s[i+1] == '1') {
			hasEscape = true
			break
		}
	}
	if !hasEscape {
		return s // No allocation for simple strings
	}

	// Single-pass unescaping with strings.Builder
	var sb strings.Builder
	sb.Grow(len(s))

	for i := 0; i < len(s); i++ {
		if s[i] == '~' && i+1 < len(s) {
			switch s[i+1] {
			case '0':
				sb.WriteByte('~')
				i++
			case '1':
				sb.WriteByte('/')
				i++
			default:
				sb.WriteByte(s[i])
			}
		} else {
			sb.WriteByte(s[i])
		}
	}
	return sb.String()
}

// HasEscapeSequence checks if path contains backslash escape sequences
// PERFORMANCE: Single scan, returns early on first match
func HasEscapeSequence(path string) bool {
	pathLen := len(path)
	for i := 0; i < pathLen; i++ {
		if path[i] == '\\' && i+1 < pathLen {
			next := path[i+1]
			// Support escaping: . \ [ ] { }
			if next == '.' || next == '\\' || next == '[' || next == ']' || next == '{' || next == '}' {
				return true
			}
		}
	}
	return false
}

// UnescapePathSegment unescapes special characters in a path segment
// Handles: \. \\ \[ \] \{ \}
// PERFORMANCE: Fast path for segments without backslash
func UnescapePathSegment(segment string) string {
	segmentLen := len(segment)
	// Fast path: scan for backslash first
	hasBackslash := false
	for i := 0; i < segmentLen; i++ {
		if segment[i] == '\\' {
			hasBackslash = true
			break
		}
	}
	if !hasBackslash {
		return segment
	}

	// Slow path: unescape
	var sb strings.Builder
	sb.Grow(segmentLen)

	for i := 0; i < segmentLen; i++ {
		if segment[i] == '\\' && i+1 < segmentLen {
			next := segment[i+1]
			switch next {
			case '.', '\\', '[', ']', '{', '}':
				sb.WriteByte(next)
				i++
				continue
			}
		}
		sb.WriteByte(segment[i])
	}

	return sb.String()
}

// PathSegmentFlags are bit flags for path segment options.
// This type is exported for use by the public API.
type PathSegmentFlags uint8

// Bit flags for PathSegment fields to avoid pointer allocations
const (
	FlagIsNegative PathSegmentFlags = 1 << iota
	FlagIsWildcard
	FlagIsFlat
	FlagHasStart
	FlagHasEnd
	FlagHasStep
)

// Public API aliases for flag constants (backward compatibility)
// These provide more descriptive names for the public API
const (
	PathFlagNegative = FlagIsNegative // Indicates negative array index
	PathFlagWildcard = FlagIsWildcard // Indicates wildcard segment
	PathFlagFlat     = FlagIsFlat     // Indicates flat extraction mode
	PathFlagHasStart = FlagHasStart   // Indicates slice has start value
	PathFlagHasEnd   = FlagHasEnd     // Indicates slice has end value
	PathFlagHasStep  = FlagHasStep    // Indicates slice has step value
)

// PathSegment represents a single segment in a JSON path
// Optimized to avoid pointer allocations by using direct values and bit flags
type PathSegment struct {
	Type  PathSegmentType
	Key   string           // Used for PropertySegment and ExtractSegment
	Index int              // Used for ArrayIndexSegment and slice start
	End   int              // Direct value (was *int) for ArraySliceSegment
	Step  int              // Direct value (was *int) for ArraySliceSegment
	Flags PathSegmentFlags // Bit-packed flags
}

// HasStart returns true if slice has a start value
func (ps *PathSegment) HasStart() bool { return ps.Flags&FlagHasStart != 0 }

// HasEnd returns true if slice has an end value
func (ps *PathSegment) HasEnd() bool { return ps.Flags&FlagHasEnd != 0 }

// HasStep returns true if slice has a step value
func (ps *PathSegment) HasStep() bool { return ps.Flags&FlagHasStep != 0 }

// IsNegativeIndex returns true if Index is negative
func (ps *PathSegment) IsNegativeIndex() bool { return ps.Flags&FlagIsNegative != 0 }

// IsWildcardSegment returns true for WildcardSegment
func (ps *PathSegment) IsWildcardSegment() bool { return ps.Flags&FlagIsWildcard != 0 }

// IsFlatExtract returns true for flat extraction
func (ps *PathSegment) IsFlatExtract() bool { return ps.Flags&FlagIsFlat != 0 }

// GetStart returns the start value and whether it was set
func (ps *PathSegment) GetStart() (int, bool) { return ps.Index, ps.HasStart() }

// GetEnd returns the end value and whether it was set
func (ps *PathSegment) GetEnd() (int, bool) { return ps.End, ps.HasEnd() }

// GetStep returns the step value and whether it was set
func (ps *PathSegment) GetStep() (int, bool) { return ps.Step, ps.HasStep() }

// PathSegmentType represents the type of path segment
type PathSegmentType int

const (
	PropertySegment PathSegmentType = iota
	ArrayIndexSegment
	ArraySliceSegment
	WildcardSegment
	RecursiveSegment
	FilterSegment
	ExtractSegment // For extract operations
	AppendSegment  // For append operations [+] syntax
)

// String returns the string representation of PathSegmentType
func (pst PathSegmentType) String() string {
	switch pst {
	case PropertySegment:
		return "property"
	case ArrayIndexSegment:
		return "array"
	case ArraySliceSegment:
		return "slice"
	case WildcardSegment:
		return "wildcard"
	case RecursiveSegment:
		return "recursive"
	case FilterSegment:
		return "filter"
	case ExtractSegment:
		return "extract"
	case AppendSegment:
		return "append"
	default:
		return "unknown"
	}
}

// TypeString returns the string type for the segment
func (ps PathSegment) TypeString() string {
	// Use the Type enum for consistent behavior
	return ps.Type.String()
}

// Helper functions to create PathSegments with proper types

// NewPropertySegment creates a property access segment
func NewPropertySegment(key string) PathSegment {
	return PathSegment{
		Type: PropertySegment,
		Key:  key,
	}
}

// NewArrayIndexSegment creates an array index access segment
func NewArrayIndexSegment(index int) PathSegment {
	var flags PathSegmentFlags
	if index < 0 {
		flags |= FlagIsNegative
	}
	return PathSegment{
		Type:  ArrayIndexSegment,
		Index: index,
		Flags: flags,
	}
}

// NewArraySliceSegment creates an array slice access segment
// Now accepts direct values instead of pointers to avoid heap allocations
func NewArraySliceSegment(start, end, step int, hasStart, hasEnd, hasStep bool) PathSegment {
	var flags PathSegmentFlags
	if hasStart {
		flags |= FlagHasStart
	}
	if hasEnd {
		flags |= FlagHasEnd
	}
	if hasStep {
		flags |= FlagHasStep
	}
	return PathSegment{
		Type:  ArraySliceSegment,
		Index: start, // Use Index field for start value
		End:   end,
		Step:  step,
		Flags: flags,
	}
}

// NewExtractSegment creates an extraction segment
func NewExtractSegment(extract string) PathSegment {
	// Check if this is a flat extraction
	isFlat := strings.HasPrefix(extract, "flat:")
	actualExtract := extract
	if isFlat {
		actualExtract = strings.TrimPrefix(extract, "flat:")
	}

	var flags PathSegmentFlags
	if isFlat {
		flags |= FlagIsFlat
	}
	return PathSegment{
		Type:  ExtractSegment,
		Key:   actualExtract,
		Flags: flags,
	}
}

// NewExtractSegmentWithFlat creates an extraction segment with explicit flat flag
func NewExtractSegmentWithFlat(key string, flat bool) PathSegment {
	var flags PathSegmentFlags
	if flat {
		flags |= FlagIsFlat
	}
	return PathSegment{
		Type:  ExtractSegment,
		Key:   key,
		Flags: flags,
	}
}

// NewWildcardSegment creates a wildcard segment
func NewWildcardSegment() PathSegment {
	return PathSegment{
		Type:  WildcardSegment,
		Flags: FlagIsWildcard,
	}
}

// emptyPathSegments is a cached empty slice for empty/root paths
// PERFORMANCE: Avoids repeated allocations for common empty path case
var emptyPathSegments = make([]PathSegment, 0)

// ParsePath parses a JSON path string into segments
// PERFORMANCE v3: Added sync.Map-based cache for lock-free reads
// PERFORMANCE v2: Added fast path for simple single-property access
func ParsePath(path string) ([]PathSegment, error) {
	if path == "" {
		return emptyPathSegments, nil
	}

	// Handle root path special case
	if path == "." {
		return emptyPathSegments, nil
	}

	// PERFORMANCE v3: Check cache first (lock-free)
	if segments, ok := getCachedPathSegments(path); ok {
		return segments, nil
	}

	var segments []PathSegment
	var err error

	// Handle different path formats
	if path[0] == '/' {
		segments, err = parseJSONPointer(path)
	} else {
		// FAST PATH: Simple single property access (no dots, no brackets)
		// This is the most common case - ~30% of all path lookups
		if len(path) <= 64 {
			hasSpecial := false
			for i := 0; i < len(path); i++ {
				c := path[i]
				if c == '.' || c == '[' || c == ']' || c == '{' || c == '}' {
					hasSpecial = true
					break
				}
			}
			if !hasSpecial {
				// Single property - return directly without allocation overhead
				segments = []PathSegment{{Type: PropertySegment, Key: path}}
				setCachedPathSegments(path, segments)
				return segments, nil
			}
		}
		segments, err = parseDotNotation(path)
	}

	if err != nil {
		return nil, err
	}

	// Cache the result
	setCachedPathSegments(path, segments)
	return segments, nil
}

// ParseComplexSegment parses a complex segment that may contain mixed syntax
func ParseComplexSegment(part string) ([]PathSegment, error) {
	return parseComplexSegment(part)
}

// parseDotNotation parses dot notation paths like "user.name" or "users[0].name"
// PERFORMANCE: Pre-calculates segment count to avoid slice growth allocations
// PERFORMANCE v2: Added fast paths for common simple cases (1-2 segments, no brackets)
// ESCAPE: Handles \. \\ \[ \] \{ \} escape sequences
// SECURITY: Enforces MaxPathParseDepth limit to prevent stack overflow attacks
func parseDotNotation(path string) ([]PathSegment, error) {
	pathLen := len(path)

	// FAST PATH: Check for simple dot-separated properties (no brackets/braces/escapes)
	// This handles cases like "user.name" or "a.b.c" efficiently
	hasBrackets := false
	hasEscape := false
	dotPos := -1
	dotCount := 0
	for i := 0; i < pathLen; i++ {
		c := path[i]
		if c == '[' || c == '{' || c == '}' {
			hasBrackets = true
			break
		}
		if c == '\\' && i+1 < pathLen {
			next := path[i+1]
			if next == '.' || next == '\\' || next == '[' || next == ']' || next == '{' || next == '}' {
				hasEscape = true
			}
		}
		if c == '.' {
			dotCount++
			if dotPos == -1 {
				dotPos = i
			}
		}
	}

	// FAST PATH: Simple dot-separated path without brackets/braces/escapes
	if !hasBrackets && !hasEscape && dotCount > 0 && dotCount <= 8 {
		// SECURITY: Check depth
		if dotCount+1 > MaxPathParseDepth {
			return nil, fmt.Errorf("path too deep: %d segments (max %d)", dotCount+1, MaxPathParseDepth)
		}

		segments := make([]PathSegment, dotCount+1)
		start := 0
		idx := 0
		for i := 0; i <= pathLen; i++ {
			if i == pathLen || path[i] == '.' {
				part := path[start:i]
				// Check if this is a numeric array index (supports negative indices)
				if index, ok := ParseIntFast(part); ok {
					var flags PathSegmentFlags
					if index < 0 {
						flags |= FlagIsNegative
					}
					segments[idx] = PathSegment{
						Type:  ArrayIndexSegment,
						Index: index,
						Flags: flags,
					}
				} else {
					segments[idx] = PathSegment{
						Type: PropertySegment,
						Key:  part,
					}
				}
				start = i + 1
				idx++
			}
		}
		return segments, nil
	}

	// Pre-calculate segment count for better allocation
	// Count dots outside brackets and add 1 for the initial segment
	if !hasBrackets {
		bracketDepth := 0
		for i := 0; i < pathLen; i++ {
			switch path[i] {
			case '[':
				bracketDepth++
			case ']':
				bracketDepth--
			case '.':
				if bracketDepth == 0 {
					dotCount++
				}
			}
		}
	}

	// SECURITY: Early depth check - if estimated segments exceed limit, fail fast
	// This is consistent with ValidatePath's depth check
	estimatedSegments := dotCount + 1
	if estimatedSegments > MaxPathParseDepth {
		return nil, fmt.Errorf("path too deep: estimated %d segments (max %d)", estimatedSegments, MaxPathParseDepth)
	}

	segments := make([]PathSegment, 0, estimatedSegments)

	// Smart split that respects extraction and array operation boundaries
	parts := smartSplitPath(path)

	for _, part := range parts {
		if part == "" {
			continue
		}

		// SECURITY: Check segment count during parsing to prevent exceeding depth limit
		if len(segments) >= MaxPathParseDepth {
			return nil, fmt.Errorf("path too deep: exceeds %d segments", MaxPathParseDepth)
		}

		// Check for complex mixed syntax like {field}[slice] or property[index]{extract}
		if (strings.Contains(part, "[") && strings.Contains(part, "{")) ||
			(strings.Contains(part, "{") && strings.Contains(part, "}")) {
			propSegments, err := parseComplexSegment(part)
			if err != nil {
				return nil, fmt.Errorf("invalid complex segment '%s': %w", part, err)
			}
			// SECURITY: Check after adding multiple segments
			if len(segments)+len(propSegments) > MaxPathParseDepth {
				return nil, fmt.Errorf("path too deep: exceeds %d segments", MaxPathParseDepth)
			}
			segments = append(segments, propSegments...)
		} else if strings.Contains(part, "[") {
			// Traditional array access patterns
			propSegments, err := parsePropertyWithArray(part)
			if err != nil {
				return nil, fmt.Errorf("invalid array access in '%s': %w", part, err)
			}
			if len(segments)+len(propSegments) > MaxPathParseDepth {
				return nil, fmt.Errorf("path too deep: exceeds %d segments", MaxPathParseDepth)
			}
			segments = append(segments, propSegments...)
		} else if strings.Contains(part, "{") {
			// Pure extraction syntax
			propSegments, err := parseComplexSegment(part)
			if err != nil {
				return nil, fmt.Errorf("invalid extraction syntax in '%s': %w", part, err)
			}
			if len(segments)+len(propSegments) > MaxPathParseDepth {
				return nil, fmt.Errorf("path too deep: exceeds %d segments", MaxPathParseDepth)
			}
			segments = append(segments, propSegments...)
		} else {
			// Use fast parser to avoid strconv.Atoi error allocations
			if index, ok := ParseIntFast(part); ok {
				var flags PathSegmentFlags
				if index < 0 {
					flags |= FlagIsNegative
				}
				segments = append(segments, PathSegment{
					Type:  ArrayIndexSegment,
					Index: index,
					Flags: flags,
				})
			} else {
				segments = append(segments, PathSegment{
					Type: PropertySegment,
					Key:  UnescapePathSegment(part),
				})
			}
		}
	}

	return segments, nil
}

// smartSplitPath splits path by dots while respecting extraction and array operation boundaries
// Optimized version: uses byte index tracking instead of strings.Builder to reduce allocations
// PERFORMANCE: Single-pass algorithm with pre-estimated capacity
// ESCAPE: Handles \. \\ \[ \] \{ \} escape sequences
func smartSplitPath(path string) []string {
	pathLen := len(path)
	if pathLen == 0 {
		return nil
	}

	// Pre-estimate capacity: count dots outside brackets in single pass
	// Also detect escape sequences and brackets/braces
	dotCount := 0
	braceDepth := 0
	bracketDepth := 0
	hasBrackets := false
	hasBraces := false
	hasEscape := false

	for i := range pathLen {
		c := path[i]
		switch c {
		case '\\':
			// Check for escape sequence
			if i+1 < pathLen {
				next := path[i+1]
				if next == '.' || next == '\\' || next == '[' || next == ']' || next == '{' || next == '}' {
					hasEscape = true
				}
			}
		case '{':
			braceDepth++
			hasBraces = true
		case '}':
			braceDepth--
		case '[':
			bracketDepth++
			hasBrackets = true
		case ']':
			bracketDepth--
		case '.':
			if braceDepth == 0 && bracketDepth == 0 {
				dotCount++
			}
		}
	}

	// Fast path for simple paths (no brackets, braces, or escapes) - use strings.Split
	// PERFORMANCE: strings.Split is highly optimized for simple cases
	if !hasBrackets && !hasBraces && !hasEscape && dotCount > 0 {
		return strings.Split(path, ".")
	}

	// Fast path for single segment (no dots outside brackets)
	if dotCount == 0 {
		return []string{path}
	}

	// Estimate capacity from dot count
	parts := make([]string, 0, dotCount+1)
	start := 0
	braceDepth = 0
	bracketDepth = 0

	// Use traditional for loop so i++ can skip escaped characters
	for i := 0; i < pathLen; i++ {
		c := path[i]
		switch c {
		case '\\':
			// Skip escaped character (don't treat as special)
			if i+1 < pathLen {
				next := path[i+1]
				if next == '.' || next == '\\' || next == '[' || next == ']' || next == '{' || next == '}' {
					i++ // Skip next character - works in traditional for loop
					continue
				}
			}
		case '{':
			braceDepth++
		case '}':
			braceDepth--
		case '[':
			bracketDepth++
		case ']':
			bracketDepth--
		case '.':
			// Only split on dots when we're not inside braces or brackets
			if braceDepth == 0 && bracketDepth == 0 {
				if i > start {
					parts = append(parts, path[start:i])
				}
				start = i + 1
			}
		}
	}

	// Add the last part
	if start < pathLen {
		parts = append(parts, path[start:])
	}

	return parts
}

// parsePropertyWithArray parses property access with array notation like "users[0]" or "data[1:3]"
func parsePropertyWithArray(part string) ([]PathSegment, error) {
	var segments []PathSegment

	bracketIndex := strings.Index(part, "[")
	if bracketIndex > 0 {
		propertyName := part[:bracketIndex]
		segments = append(segments, PathSegment{
			Type: PropertySegment,
			Key:  UnescapePathSegment(propertyName),
		})
	}

	// Parse all array access patterns
	remaining := part[bracketIndex:]
	for len(remaining) > 0 {
		if !strings.HasPrefix(remaining, "[") {
			return nil, fmt.Errorf("expected '[' but found '%s'", remaining)
		}

		closeBracket := strings.Index(remaining, "]")
		if closeBracket == -1 {
			return nil, fmt.Errorf("missing closing bracket in '%s'", remaining)
		}

		arrayPart := remaining[1:closeBracket]
		segment, err := parseArrayAccess(arrayPart)
		if err != nil {
			return nil, fmt.Errorf("invalid array access '%s': %w", arrayPart, err)
		}

		segments = append(segments, segment)
		remaining = remaining[closeBracket+1:]
	}

	return segments, nil
}

// parseComplexSegment parses complex segments that may contain mixed syntax like {field}[slice]
func parseComplexSegment(part string) ([]PathSegment, error) {
	var segments []PathSegment
	remaining := part

	for len(remaining) > 0 {
		// Check for extraction syntax first
		if strings.HasPrefix(remaining, "{") {
			braceEnd := strings.Index(remaining, "}")
			if braceEnd == -1 {
				return nil, fmt.Errorf("missing closing brace in '%s'", remaining)
			}

			extractPart := remaining[1:braceEnd]

			// Check if this is a flat extraction
			isFlat := strings.HasPrefix(extractPart, "flat:")
			actualExtract := extractPart
			if isFlat {
				actualExtract = strings.TrimPrefix(extractPart, "flat:")
			}

			// Validate extraction field name(s)
			if actualExtract == "" {
				return nil, fmt.Errorf("empty extraction field in '%s'", remaining[:braceEnd+1])
			}

			// Validate and normalize multi-field extraction (e.g., {id,name,email})
			if strings.Contains(actualExtract, ",") {
				fields := strings.Split(actualExtract, ",")
				validatedFields := make([]string, 0, len(fields))
				for _, field := range fields {
					field = strings.TrimSpace(field)
					if field == "" {
						return nil, fmt.Errorf("empty field name in extraction '%s'", remaining[:braceEnd+1])
					}
					if !isValidFieldName(field) {
						return nil, fmt.Errorf("invalid field name '%s' in extraction", field)
					}
					validatedFields = append(validatedFields, field)
				}
				// Rebuild with trimmed and validated fields
				actualExtract = strings.Join(validatedFields, ",")
			}

			var flags PathSegmentFlags
			if isFlat {
				flags |= FlagIsFlat
			}
			segments = append(segments, PathSegment{
				Type:  ExtractSegment,
				Key:   actualExtract,
				Flags: flags,
			})

			remaining = remaining[braceEnd+1:]
			continue
		}

		// Check for array access
		if strings.HasPrefix(remaining, "[") {
			bracketEnd := strings.Index(remaining, "]")
			if bracketEnd == -1 {
				return nil, fmt.Errorf("missing closing bracket in '%s'", remaining)
			}

			arrayPart := remaining[1:bracketEnd]

			// Validate array access syntax
			if arrayPart == "" {
				return nil, fmt.Errorf("empty array access in '%s'", remaining[:bracketEnd+1])
			}

			segment, err := parseArrayAccess(arrayPart)
			if err != nil {
				return nil, fmt.Errorf("invalid array access '%s': %w", arrayPart, err)
			}

			segments = append(segments, segment)
			remaining = remaining[bracketEnd+1:]
			continue
		}

		// If we reach here, it's a property name at the beginning
		// Find the next special character
		nextSpecial := len(remaining)
		for i, char := range remaining {
			if char == '[' || char == '{' {
				nextSpecial = i
				break
			}
		}

		if nextSpecial > 0 {
			propertyName := remaining[:nextSpecial]
			segments = append(segments, PathSegment{
				Type: PropertySegment,
				Key:  propertyName,
			})
			remaining = remaining[nextSpecial:]
		} else {
			// No more special characters, treat the rest as property name
			if remaining != "" {
				segments = append(segments, PathSegment{
					Type: PropertySegment,
					Key:  remaining,
				})
			}
			break
		}
	}

	return segments, nil
}

// parseArrayAccess parses array access patterns like "0", "-1", "1:3", "::2", "+"
func parseArrayAccess(arrayPart string) (PathSegment, error) {
	// Check for append syntax [+] - append to array
	if arrayPart == "+" {
		return PathSegment{
			Type: AppendSegment,
		}, nil
	}

	// Check for slice notation using direct byte scan (no allocation)
	hasColon := false
	for i := 0; i < len(arrayPart); i++ {
		if arrayPart[i] == ':' {
			hasColon = true
			break
		}
	}
	if hasColon {
		return parseSliceAccess(arrayPart)
	}

	// Check for wildcard
	if arrayPart == "*" {
		return PathSegment{
			Type:  WildcardSegment,
			Flags: FlagIsWildcard,
		}, nil
	}

	// Fast path for single digit indices (0-9)
	if len(arrayPart) == 1 && arrayPart[0] >= '0' && arrayPart[0] <= '9' {
		index := int(arrayPart[0] - '0')
		return PathSegment{
			Type:  ArrayIndexSegment,
			Index: index,
		}, nil
	}

	// Fast path for negative single digit (-9 to -0)
	if len(arrayPart) == 2 && arrayPart[0] == '-' && arrayPart[1] >= '0' && arrayPart[1] <= '9' {
		index := -int(arrayPart[1] - '0')
		return PathSegment{
			Type:  ArrayIndexSegment,
			Index: index,
			Flags: FlagIsNegative,
		}, nil
	}

	// Simple index access - use fast parser to avoid strconv error allocations
	index, ok := ParseIntFast(arrayPart)
	if !ok {
		return PathSegment{}, fmt.Errorf("invalid array index '%s'", arrayPart)
	}

	var flags PathSegmentFlags
	if index < 0 {
		flags |= FlagIsNegative
	}
	return PathSegment{
		Type:  ArrayIndexSegment,
		Index: index,
		Flags: flags,
	}, nil
}

// parseSliceAccess parses slice notation like "1:3", "::2", "::-1"
// Optimized to avoid strings.Split allocation
func parseSliceAccess(slicePart string) (PathSegment, error) {
	// Find colon positions without allocation
	colon1 := -1
	colon2 := -1
	for i := 0; i < len(slicePart); i++ {
		if slicePart[i] == ':' {
			if colon1 == -1 {
				colon1 = i
			} else {
				colon2 = i
				break
			}
		}
	}

	// Validate colon positions
	if colon1 == -1 {
		return PathSegment{}, fmt.Errorf("invalid slice syntax '%s': no colon found", slicePart)
	}
	if colon2 == -1 {
		// Two-part slice (start:end)
		colon2 = len(slicePart) // Mark as no second colon
	}
	// colon2-colon1 == 1 means adjacent colons (::step), handled below by parsing between colons as empty

	var start, end, step int
	var flags PathSegmentFlags

	// Parse start (before first colon) - use fast parser to avoid strconv error allocations
	if colon1 > 0 {
		startVal, ok := ParseIntFast(slicePart[:colon1])
		if !ok {
			return PathSegment{}, fmt.Errorf("invalid slice start '%s'", slicePart[:colon1])
		}
		start = startVal
		flags |= FlagHasStart
	}

	// Parse end (between colons) - use fast parser
	if colon2 > colon1+1 {
		// There's content between colons
		endStr := slicePart[colon1+1 : colon2]
		endVal, ok := ParseIntFast(endStr)
		if !ok {
			return PathSegment{}, fmt.Errorf("invalid slice end '%s'", endStr)
		}
		end = endVal
		flags |= FlagHasEnd
	}

	// Parse step (after second colon, if exists) - use fast parser
	if colon2 < len(slicePart) && colon2 > colon1 {
		// We have a second colon, check for step
		stepStr := slicePart[colon2+1:]
		if stepStr != "" {
			stepVal, ok := ParseIntFast(stepStr)
			if !ok {
				return PathSegment{}, fmt.Errorf("invalid slice step '%s'", stepStr)
			}
			if stepVal == 0 {
				return PathSegment{}, fmt.Errorf("slice step cannot be zero")
			}
			step = stepVal
			flags |= FlagHasStep
		}
	}

	return PathSegment{
		Type:  ArraySliceSegment,
		Index: start, // Use Index field for start value
		End:   end,
		Step:  step,
		Flags: flags,
	}, nil
}

// parseJSONPointer parses JSON Pointer format paths like "/users/0/name"
func parseJSONPointer(path string) ([]PathSegment, error) {
	if path == "/" {
		return []PathSegment{{Type: PropertySegment, Key: ""}}, nil
	}

	path = strings.TrimPrefix(path, "/")
	parts := strings.Split(path, "/")

	segments := make([]PathSegment, 0, len(parts))
	for _, part := range parts {
		// Unescape JSON Pointer special characters using helper
		part = UnescapeJSONPointer(part)

		// Try to parse as numeric index using fast parser
		if index, ok := ParseIntFast(part); ok {
			var flags PathSegmentFlags
			if index < 0 {
				flags |= FlagIsNegative
			}
			segments = append(segments, PathSegment{
				Type:  ArrayIndexSegment,
				Index: index,
				Flags: flags,
			})
			continue
		}

		// Property access
		segments = append(segments, PathSegment{
			Type: PropertySegment,
			Key:  part,
		})
	}

	return segments, nil
}

// ValidatePath validates a path string for syntax correctness.
// FOCUSED: Only validates syntax (brackets, depth, array indices).
// SECURITY: Security validation (injection, traversal) should be done by caller.
// PERFORMANCE: Uses single-pass validation with fast path for simple paths.
func ValidatePath(path string) error {
	const maxArrayIndex = 1000000 // SECURITY: support for large array indices

	pathLen := len(path)

	// FAST PATH: Simple property path (alphanumeric + dots + underscores only)
	// This handles the vast majority of cases without complex validation
	isSimple := true
	depth := 1 // Start with 1 for the first segment
	for i := range pathLen {
		c := path[i]
		// Check for simple characters only
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') &&
			(c < '0' || c > '9') && c != '_' && c != '.' {
			isSimple = false
			break
		}
		if c == '.' {
			depth++
		}
	}

	if isSimple {
		// Simple validation passed - just check depth
		if depth > MaxPathParseDepth {
			return fmt.Errorf("path too deep: %d segments (max %d)", depth, MaxPathParseDepth)
		}
		return nil
	}

	// SLOW PATH: Complex path with brackets, braces, etc.
	// Single-pass validation for syntax only
	depth = 0
	inBracket := false
	bracketStart := 0

	for i := range pathLen {
		c := path[i]

		// Track depth and validate brackets/array indices
		switch c {
		case '.':
			if !inBracket {
				depth++
			}
		case '[':
			if !inBracket {
				inBracket = true
				bracketStart = i
				depth++
			}
		case ']':
			if inBracket {
				// Validate array index/slice content inside brackets
				content := path[bracketStart+1 : i]
				if err := validateArrayIndexContent(content, maxArrayIndex); err != nil {
					return err
				}
				inBracket = false
			}
		}
	}

	if depth > MaxPathParseDepth {
		return fmt.Errorf("path too deep: %d segments (max %d)", depth, MaxPathParseDepth)
	}

	return nil
}

// validateArrayIndexContent validates array index or slice content
// PERFORMANCE: Optimized to avoid strings.Contains and strings.Split allocations
func validateArrayIndexContent(content string, maxIndex int) error {
	if content == "" {
		return fmt.Errorf("empty array index")
	}

	// Handle wildcard - single character check
	if content == "*" {
		return nil
	}

	// Handle append syntax [+] - append to array
	if content == "+" {
		return nil
	}

	// Fast path: scan for colon without allocation
	hasColon := false
	for i := 0; i < len(content); i++ {
		if content[i] == ':' {
			hasColon = true
			break
		}
	}

	if hasColon {
		// Parse slice notation without strings.Split allocation
		start := 0
		for i := 0; i <= len(content); i++ {
			if i == len(content) || content[i] == ':' {
				if i > start {
					part := content[start:i]
					if err := validateNumericIndex(part, maxIndex); err != nil {
						return err
					}
				}
				start = i + 1
			}
		}
		return nil
	}

	// Simple index
	return validateNumericIndex(content, maxIndex)
}

// validateNumericIndex validates a single numeric index
// PERFORMANCE: Manual parsing avoids strconv.Atoi allocation
func validateNumericIndex(s string, maxIndex int) error {
	// Allow negative sign
	sLen := len(s)
	if sLen == 0 {
		return fmt.Errorf("empty index")
	}

	negative := false
	start := 0
	if s[0] == '-' {
		negative = true
		start = 1
		if sLen == 1 {
			return fmt.Errorf("invalid index: %s", s)
		}
	}

	// Fast path for single digit
	if sLen-start == 1 {
		c := s[start]
		if c < '0' || c > '9' {
			return fmt.Errorf("invalid array index: %s", s)
		}
		return nil
	}

	// Check all characters are digits and parse value
	index := 0
	for i := start; i < sLen; i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return fmt.Errorf("invalid array index: %s", s)
		}
		// Check for overflow before multiplying
		if index > (maxIndex / 10) {
			return fmt.Errorf("array index out of reasonable range")
		}
		index = index*10 + int(c-'0')
	}

	if negative {
		index = -index
	}

	if index < -maxIndex || index > maxIndex {
		return fmt.Errorf("array index out of reasonable range: %d (range: %d to %d)",
			index, -maxIndex, maxIndex)
	}

	return nil
}

// isValidFieldName validates if a string is a valid JSON field name
// Allows alphanumeric characters, underscores, and hyphens
func isValidFieldName(name string) bool {
	if len(name) == 0 {
		return false
	}
	// First character must be letter, underscore, or digit
	c := name[0]
	if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && c != '_' && (c < '0' || c > '9') {
		return false
	}
	// Check remaining characters
	for i := 1; i < len(name); i++ {
		c := name[i]
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '_' && c != '-' {
			return false
		}
	}
	return true
}

// String returns a string representation of the path segment
func (ps PathSegment) String() string {
	switch ps.Type {
	case PropertySegment:
		return ps.Key
	case ArrayIndexSegment:
		return fmt.Sprintf("[%d]", ps.Index)
	case ArraySliceSegment:
		start := ""
		end := ""
		step := ""

		if ps.HasStart() {
			start = strconv.Itoa(ps.Index) // Index stores start value for slices
		}
		if ps.HasEnd() {
			end = strconv.Itoa(ps.End)
		}
		if ps.HasStep() {
			step = ":" + strconv.Itoa(ps.Step)
		}

		return fmt.Sprintf("[%s:%s%s]", start, end, step)
	case WildcardSegment:
		return "[*]"
	case ExtractSegment:
		if ps.IsFlatExtract() {
			return fmt.Sprintf("{flat:%s}", ps.Key)
		}
		return fmt.Sprintf("{%s}", ps.Key)
	default:
		return fmt.Sprintf("[unknown:%v]", ps.Type)
	}
}

// IsArrayAccess returns true if this segment accesses an array
func (ps PathSegment) IsArrayAccess() bool {
	return ps.Type == ArrayIndexSegment || ps.Type == ArraySliceSegment || ps.Type == WildcardSegment
}

// GetArrayIndex returns the array index, handling negative indices
func (ps PathSegment) GetArrayIndex(arrayLength int) (int, error) {
	if ps.Type != ArrayIndexSegment {
		return 0, fmt.Errorf("not an array index segment")
	}

	index := ps.Index
	if index < 0 {
		index = arrayLength + index
	}

	if index < 0 || index >= arrayLength {
		return 0, fmt.Errorf("array index %d out of bounds (length %d)", ps.Index, arrayLength)
	}

	return index, nil
}

// ParseAndValidateArrayIndex parses a string as an array index and validates it against array length
// Returns the index and true if successful, 0 and false otherwise
func ParseAndValidateArrayIndex(s string, arrayLength int) (int, bool) {
	index, ok := ParseArrayIndex(s)
	if !ok {
		return 0, false
	}

	// Handle negative indices
	if index < 0 {
		index = arrayLength + index
	}

	// Validate bounds
	if index < 0 || index >= arrayLength {
		return 0, false
	}

	return index, true
}
