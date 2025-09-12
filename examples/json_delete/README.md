# JSON Delete Functionality Examples

This example demonstrates comprehensive JSON deletion functionality provided by the `json` package.

## Features Demonstrated

### 1. Basic Deletion Operations
- **Standard Delete**: `json.Delete()` - Removes target values but retains null placeholders
- **Clean Delete**: `json.DeleteWithCleanNull()` - Removes target values and cleans up null values
- **Custom Options**: Using `ProcessorOptions` for fine-grained control

### 2. Array Operations
- **Element Deletion**: Delete specific array elements by index
- **Negative Indexing**: Use negative indices to delete from the end
- **Range Deletion**: Delete ranges of elements using `[start:end]` syntax
- **Open-ended Ranges**: Delete from index to end using `[start:]`

### 3. Conditional Deletion
- **ForeachReturn Integration**: Delete elements based on conditions
- **Value-based Filtering**: Remove elements matching specific criteria
- **Complex Conditions**: Support for multiple condition checks

### 4. Advanced Path Expressions
- **Wildcard Deletion**: Use `{}` to target all elements in arrays/objects
- **Nested Path Deletion**: Delete deeply nested values with dot notation
- **Bulk Operations**: Delete multiple similar paths in one operation

### 5. Error Handling
- **Invalid Path Handling**: Proper error messages for malformed paths
- **Non-existent Path Handling**: Graceful handling of missing paths (no-op)
- **Validation**: Input validation with helpful error messages

## Usage Examples

### Basic Usage
```go
// Standard deletion
result, err := json.Delete(jsonStr, "field.subfield")

// Clean deletion with null cleanup
result, err := json.DeleteWithCleanNull(jsonStr, "array{field}")

// Custom options
opts := &json.ProcessorOptions{
    CleanupNulls:  true,
    CompactArrays: false,
}
result, err := json.Delete(jsonStr, "path", opts)
```

### Array Operations
```go
// Delete by index
result, err := json.Delete(jsonStr, "users[1]")

// Delete by negative index
result, err := json.Delete(jsonStr, "users[-1]")

// Delete range
result, err := json.Delete(jsonStr, "users[1:4]")

// Delete from index to end
result, err := json.Delete(jsonStr, "users[2:]")
```

### Conditional Deletion
```go
result, err := json.ForeachReturn(jsonStr, func(key any, item *json.IterableValue) {
    if item.GetBool("active") == false {
        err := item.Delete("")
        if err != nil {
            log.Printf("Failed to delete: %v", err)
        }
    }
})
```

## Key Benefits

1. **Flexible**: Multiple deletion methods for different use cases
2. **Safe**: Proper error handling and validation
3. **Efficient**: Optimized for performance with large JSON structures
4. **Intuitive**: Easy-to-understand path expressions
5. **Comprehensive**: Covers simple to complex deletion scenarios

## ProcessorOptions

The `ProcessorOptions` struct provides fine-grained control:

- `CleanupNulls`: Remove null values after deletion
- `CompactArrays`: Remove null values from arrays and compact them
- `StrictMode`: Enable strict validation
- `CacheResults`: Cache results for better performance

## Path Expression Syntax

- `field` - Delete a simple field
- `field.subfield` - Delete nested field
- `array[0]` - Delete array element by index
- `array[-1]` - Delete last array element
- `array[1:3]` - Delete array range
- `array[2:]` - Delete from index to end
- `array{field}` - Delete field from all array elements
- `object{field}` - Delete field from all object values

## 🚀 Running the Example

To run this example:

```bash
cd examples/json_delete
go run example.go
```

### Expected Output
The example will demonstrate:
- ✅ Basic field and array element deletion
- ✅ Custom deletion options (cleanup, compacting)
- ✅ Bulk deletion using path expressions
- ✅ Complex nested deletion scenarios
- ✅ Error handling and edge cases

## 🔗 Related Examples

- [**basic**](../basic/) - Learn basic JSON operations first
- [**json_set**](../json_set/) - Learn about setting values
- [**json_get**](../json_get/) - Learn about retrieving values


