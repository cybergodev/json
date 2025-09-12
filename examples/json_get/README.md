# JSON Get Example

This is a comprehensive example of JSON data retrieval functionality, demonstrating various uses of the `Get()` method and related functions in the `json` package.

## Feature Demonstrations

### 1. Basic Retrieval
- Use `json.Get()` to retrieve values of any type
- Demonstrate retrieval of different data types: strings, numbers, booleans, null values
- Show access to nested objects and array elements

### 2. Type-Safe Retrieval
- Use type-safe functions like `json.GetString()`, `json.GetInt()`, `json.GetFloat64()`
- Use `json.GetTyped[T]()` for generic type-safe retrieval
- Demonstrate type-safe retrieval of arrays and objects

### 3. Array Access
- Access array elements using indices: `users[0]`, `users[1]`
- Support negative indices: `users[-1]` to access the last element
- Nested array access: `metadata.tags[0]`

### 4. Array Slice Access
- Range access: `users[0:3]` to get the first 3 users
- Open-ended ranges: `users[2:]` from index 2 to the end
- Negative index ranges: `users[-2:]` to get the last 2 elements
- Exclusive ranges: `users[1:-1]` to get middle elements

### 5. Nested Object Access
- Use dot notation: `company.name`, `company.config.debug`
- Deep nested access: `company.config.features.auth`
- Object access within arrays: `company.departments[0].employees[0].name`

### 6. Path Expressions
- Use wildcard `{}` for bulk data extraction
- Flatten nested arrays: `a{g}{name}` extracts all name fields
- Complex nested extraction: `company.departments{employees}{name}` gets all employee names
- Automatic array flattening: `company.departments{employees}{skills}` gets all skills

### 7. Multiple Path Retrieval
- Use `json.GetMultiple()` to batch retrieve multiple values
- More efficient than individual `Get()` calls
- Support batch operations on complex paths

### 8. Advanced Processor Usage
- Custom processor configuration
- Enable caching for improved performance
- Processor statistics and health checks
- Custom processing options

### 9. Error Handling
- Invalid JSON handling
- Non-existent path handling
- Invalid array index handling
- Type conversion error handling
- Null value handling

### 10. Performance Optimization
- Performance comparison: batch operations vs individual operations
- Performance comparison: path expressions vs multiple individual gets
- Performance comparison: processor reuse vs creating new processors
- Performance optimization recommendations

## Sample Data

The example uses three different JSON data structures:

1. **Basic JSON** (`jsonStr`): Complex structure with nested arrays and objects
2. **User Array JSON** (`arrayJsonStr`): User list and metadata
3. **Complex Enterprise JSON** (`complexJsonStr`): Multi-level nested enterprise organizational structure

## Key Features

- **Path Syntax Support**: Dot notation, array indices, slicing, JSON Pointer
- **Type Safety**: Automatic type conversion and type-safe getter functions
- **Batch Operations**: Efficient multi-path retrieval
- **Wildcard Support**: Use `{}` for bulk data extraction and automatic flattening
- **Error Handling**: Comprehensive error handling and edge case management
- **Performance Optimization**: Caching, processor reuse, and other performance optimization techniques

## Performance Tips

1. Use `GetMultiple()` for batch operations
2. Use path expressions with `{}` for bulk data extraction
3. Reuse processors instead of creating new ones
4. Enable caching for repeated operations
5. Use type-safe getter functions when possible

## 🚀 Running the Example

To run this example:

```bash
cd examples/json_get
go run example.go
```

### Expected Output
The example will demonstrate:
- ✅ Basic value retrieval with different data types
- ✅ Type-safe operations with automatic conversion
- ✅ Array access with positive and negative indices
- ✅ Complex path expressions and bulk extraction
- ✅ Performance comparisons and error handling

## 🔗 Related Examples

- [**basic**](../basic/) - Start here for basic JSON operations
- [**json_set**](../json_set/) - Learn about setting and modifying JSON data
- [**flat_extraction**](../flat_extraction/) - Advanced extraction techniques
