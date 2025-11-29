package json

import (
	"strings"
	"sync"
)

// StringBuilderPool provides optimized string builder pooling
type StringBuilderPool struct {
	pool sync.Pool
}

// NewStringBuilderPool creates a new string builder pool
func NewStringBuilderPool(initialCapacity int) *StringBuilderPool {
	return &StringBuilderPool{
		pool: sync.Pool{
			New: func() any {
				sb := &strings.Builder{}
				sb.Grow(initialCapacity)
				return sb
			},
		},
	}
}

// Get retrieves a string builder from the pool
func (p *StringBuilderPool) Get() *strings.Builder {
	sb := p.pool.Get().(*strings.Builder)
	sb.Reset()
	return sb
}

// Put returns a string builder to the pool
func (p *StringBuilderPool) Put(sb *strings.Builder) {
	if sb == nil {
		return
	}

	// Only pool builders with reasonable capacity
	if sb.Cap() <= 16384 && sb.Cap() >= 256 {
		sb.Reset()
		p.pool.Put(sb)
	}
}

// OptimizedStringConcat efficiently concatenates strings
func OptimizedStringConcat(parts ...string) string {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}

	// Calculate total length
	totalLen := 0
	for _, part := range parts {
		totalLen += len(part)
	}

	// Pre-allocate builder
	var sb strings.Builder
	sb.Grow(totalLen)

	for _, part := range parts {
		sb.WriteString(part)
	}

	return sb.String()
}

// JoinPath efficiently joins path segments
func JoinPath(base, segment string) string {
	if base == "" {
		return segment
	}
	if segment == "" {
		return base
	}

	var sb strings.Builder
	sb.Grow(len(base) + len(segment) + 1)
	sb.WriteString(base)
	sb.WriteByte('.')
	sb.WriteString(segment)
	return sb.String()
}

// TruncateString efficiently truncates a string with ellipsis
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}

	if maxLen <= 3 {
		return s[:maxLen]
	}

	// For long strings, show beginning and end
	if maxLen > 50 {
		prefixLen := maxLen/2 - 7
		suffixLen := maxLen/2 - 7

		var sb strings.Builder
		sb.Grow(maxLen)
		sb.WriteString(s[:prefixLen])
		sb.WriteString("...[truncated]...")
		sb.WriteString(s[len(s)-suffixLen:])
		return sb.String()
	}

	// For shorter strings, just truncate with ...
	var sb strings.Builder
	sb.Grow(maxLen)
	sb.WriteString(s[:maxLen-3])
	sb.WriteString("...")
	return sb.String()
}

// BuildPathWithPrefix efficiently builds a path with prefix
func BuildPathWithPrefix(prefix, path string) string {
	if prefix == "" {
		return path
	}

	// Check if path already starts with prefix
	if strings.HasPrefix(path, prefix+".") {
		return path
	}

	return JoinPath(prefix, path)
}

// RemovePathPrefix efficiently removes a path prefix
func RemovePathPrefix(path, prefix string) string {
	if prefix == "" {
		return path
	}

	prefixWithDot := prefix + "."
	if strings.HasPrefix(path, prefixWithDot) {
		return path[len(prefixWithDot):]
	}

	if path == prefix {
		return ""
	}

	return path
}
