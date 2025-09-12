# JSON Encode Example

This example demonstrates comprehensive usage of the `json.Encode` method and related encoding functionality from the JSON library.

## Overview

The example covers various aspects of JSON encoding in Go:

### 1. Basic Encoding (`json.Encode`)
- Basic data types (string, int, float, bool, nil)
- Arrays and slices
- Maps
- Simple structs

### 2. Pretty Printing (`json.EncodePretty`)
- Default pretty printing with indentation
- Custom indentation and formatting
- Key sorting and prefix options

### 3. Compact Encoding (`json.EncodeCompact`)
- Minimal JSON without whitespace
- Size comparison with pretty printing
- Performance benefits for network transmission

### 4. Custom Configuration
- HTML escaping control
- Unicode character handling
- Slash escaping options
- Empty field omission
- Number precision control

### 5. Predefined Configurations
- `json.NewPrettyConfig()` - Pretty formatted JSON
- `json.NewCompactConfig()` - Compact JSON
- `json.NewWebSafeConfig()` - Web-safe JSON with proper escaping
- `json.NewReadableConfig()` - Human-readable JSON with minimal escaping

### 6. Advanced Encoding Options
- Custom escape characters
- Newline and tab handling
- Maximum depth control
- Number preservation and precision

### 7. Struct Encoding with JSON Tags
- JSON tag usage (`json:"field_name"`)
- `omitempty` tag behavior
- Nested struct encoding
- Array of structs

### 8. Complex Data Structures
- Nested objects and arrays
- Company/department/employee hierarchy
- Selective field encoding
- Large data structure handling

### 9. Custom Type Encoding
- Custom `MarshalJSON()` implementation
- Mixed type interfaces
- Pointer handling
- Unsupported type errors

### 10. Performance Optimization
- Processor reuse vs new instances
- Configuration object reuse
- Compact vs pretty encoding performance
- Memory usage best practices

### 11. Error Handling
- Unsupported types (channels, functions, complex numbers)
- Circular reference detection
- Maximum depth exceeded
- Invalid UTF-8 handling
- Custom marshaler errors

## Running the Example

```bash
go run examples/json_encode/example.go
```

## Key Features Demonstrated

### Configuration Options
```go
config := &json.EncodeConfig{
    Pretty:          true,
    Indent:          "  ",
    EscapeHTML:      false,
    SortKeys:        true,
    OmitEmpty:       true,
    ValidateUTF8:    true,
    MaxDepth:        100,
    PreserveNumbers: true,
    FloatPrecision:  2,
    DisableEscaping: false,
    EscapeUnicode:   false,
    EscapeSlash:     false,
    CustomEscapes:   map[rune]string{'🚀': "\\u{rocket}"},
}
```

### Usage Patterns
```go
// Basic encoding
jsonStr, err := json.Encode(data)

// Pretty printing
prettyJSON, err := json.EncodePretty(data)

// Custom configuration
customJSON, err := json.Encode(data, config)

// Predefined configurations
webSafeJSON, err := json.Encode(data, json.NewWebSafeConfig())
```

### Custom Types
```go
type CustomID struct {
    Prefix string
    Number int
}

func (c CustomID) MarshalJSON() ([]byte, error) {
    return []byte(fmt.Sprintf(`"%s-%04d"`, c.Prefix, c.Number)), nil
}
```

## Performance Tips

1. **Reuse processors** instead of creating new ones for each operation
2. **Reuse configuration objects** to avoid repeated allocations
3. **Use compact encoding** for network transmission to reduce bandwidth
4. **Use pretty encoding** only for debugging and display purposes
5. **Consider streaming** for very large datasets

## Error Handling Best Practices

1. Always check for errors when encoding
2. Be aware of unsupported types (channels, functions, complex numbers)
3. Avoid circular references in data structures
4. Set appropriate MaxDepth limits for deeply nested data
5. Validate UTF-8 when dealing with external data
6. Handle custom marshaler errors gracefully

## Output

The example produces detailed output showing:
- JSON encoding results for various data types
- Performance comparisons between different encoding methods
- Error handling demonstrations
- Size comparisons between compact and pretty formats
- Configuration effects on output format
