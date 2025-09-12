# JSON Library Basic Usage Examples

This example demonstrates the basic functionality of the `github.com/cybergodev/json` library and is the best starting point for learning the library.

## 🎯 Demonstrated Features

### 1. Basic Get Operations
- **Simple field access**: `"company"` - Get top-level field
- **Nested field access**: `"config.version"` - Use dot notation to access nested fields
- **Array element access**: `"users[0]"` - Access specific elements in arrays

### 2. Type-Safe Operations
- **`GetString()`** - Safely get string values
- **`GetInt()`** - Safely get integer values with automatic type conversion
- **`GetBool()`** - Safely get boolean values
- **`GetArray()`** - Safely get array values

### 3. Array Operations
- **Positive indexing**: `"users[0]"` - First element
- **Negative indexing**: `"users[-1]"` - Last element
- **Array slicing**: `"users[0:2]"` - First two elements
- **Tail slicing**: `"users[-2:]"` - Last two elements

### 4. Extraction Operations
- **Field extraction**: `"users{name}"` - Extract name field from all users
- **Multi-field extraction**: `"users{age}"` - Extract age field from all users
- **Flat extraction**: `"users{flat:skills}"` - Flatten all skills into a single array

### 5. Basic Set Operations
- **Simple field setting**: Modify top-level field values
- **Nested field setting**: Modify fields in nested objects
- **Array element setting**: Modify specific element values in arrays

### 6. Basic Delete Operations
- **Field deletion**: Delete specific fields from objects
- **Array element deletion**: Delete specific elements from arrays

### 7. Batch Operations
- **`GetMultiple()`** - Get values from multiple paths at once
- **`SetMultiple()`** - Set values for multiple paths at once

## 🚀 Running the Example

```bash
cd examples/basic
go run example.go
```

## 💡 Learning Path

After completing this basic example, we recommend learning other examples in the following order:

1. **[compatibility](../compatibility/)** - Learn about compatibility with the standard library
2. **[json_get](../json_get/)** - Deep dive into get operations and path expressions
3. **[json_set](../json_set/)** - Learn complex set and modification operations
4. **[flat_extraction](../flat_extraction/)** - Master flat extraction functionality
5. **[json_delete](../json_delete/)** - Learn delete operations and data cleanup
6. **[json_iteration](../json_iteration/)** - Master iteration and traversal functionality
7. **[json_encode](../json_encode/)** - Learn encoding and formatting functionality

## 🎯 Core Concepts

### Path Expressions
- **Dot notation access**: `"user.profile.name"` - Access nested objects
- **Array indexing**: `"users[0]"` - Access array elements
- **Extraction syntax**: `"users{name}"` - Batch extract fields

### Type Safety
- Use type-safe get methods to avoid type assertions
- Automatic handling of numeric type conversions (float64 ↔ int)
- Return zero values instead of panic when types don't match

### Performance Benefits
- Access specific fields without full JSON parsing
- Batch operations reduce repeated parsing overhead
- Smart caching improves repeated access performance

## 🔗 Related Documentation

- [Main Documentation](../../README.md) - Complete API documentation
- [Chinese Documentation](../../docs/doc_zh_CN.md) - Detailed Chinese documentation
- [Compatibility Guide](../../docs/COMPATIBILITY.md) - Migration guide
