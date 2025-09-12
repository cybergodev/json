# JSON Set Operations Example

This example demonstrates comprehensive usage of the `json` package's Set functionality, including basic Set operations, path creation, array manipulation, and error handling.

## Overview

The Set functionality in the `json` package allows you to modify JSON data by setting values at specific paths. This example covers:

- **Basic Set Operations**: Setting values in existing JSON structures
- **SetWithAdd Operations**: Automatically creating paths when setting values
- **Custom Options**: Fine-grained control using ProcessorOptions
- **Array Operations**: Setting individual elements, ranges, and using wildcards
- **Path Creation**: Building complex nested structures from scratch
- **Type Handling**: Working with different JSON data types
- **Error Handling**: Proper error management and edge cases

## Key Functions Demonstrated

### Core Set Functions

```go
// Basic Set - requires existing paths
func Set(jsonStr, path string, value any, opts ...*ProcessorOptions) (string, error)

// SetWithAdd - automatically creates missing paths
func SetWithAdd(jsonStr, path string, value any) (string, error)
```

### ProcessorOptions

```go
type ProcessorOptions struct {
    CreatePaths   bool // Automatically create missing paths
    CleanupNulls  bool // Clean up null values after operations
    CompactArrays bool // Compact arrays by removing nulls
    StrictMode    bool // Enable strict validation
}
```

## Example Categories

### 1. Basic Set Operations (`basicSetOperations`)
- Setting existing fields
- Handling non-existent paths
- Nested field updates

### 2. SetWithAdd Operations (`setWithAddOperations`)
- Automatic path creation
- Deep nesting creation
- Complex object addition

### 3. Custom Options (`setWithCustomOptions`)
- Using ProcessorOptions for control
- StrictMode behavior
- Multiple option combinations

### 4. Array Operations (`setArrayElements`, `setArrayRanges`)
- Setting by positive/negative index
- Range operations (`[start:end]`)
- Wildcard operations (`{}`)
- Adding new array elements

### 5. Path Creation (`createNestedPaths`)
- Building structures from empty JSON
- Step-by-step vs. single-operation creation
- Complex nested structures

### 6. Complex Nested Operations (`setNestedElements`)
- Deep nested array updates
- Wildcard bulk operations
- Adding new complex objects

### 7. Batch Operations (`batchSetOperations`)
- Chaining multiple Set operations
- Efficient bulk updates
- Metadata management

### 8. Type-Specific Operations (`setDifferentTypes`)
- String, number, boolean values
- Arrays and objects
- Null values and type conversions

### 9. Error Handling (`errorHandlingExamples`)
- Invalid JSON handling
- Path syntax validation
- Graceful operation handling

## Path Syntax Examples

```go
// Simple paths
"user.name"                    // Object property
"users[0]"                     // Array element by index
"users[-1]"                    // Array element by negative index

// Range operations
"users[1:3]"                   // Array slice (elements 1 and 2)
"users[2:]"                    // From index 2 to end
"users[:2]"                    // From start to index 2

// Wildcard operations
"users{name}"                  // All 'name' fields in users array
"company.departments{budget}"  // All 'budget' fields in departments

// Complex nested paths
"company.departments[0].employees[1].salary"  // Deep nesting
"data.items{metadata}{tags}"                 // Multiple wildcards
```

## Usage Patterns

### Setting Simple Values
```go
// Update existing field
result, err := json.Set(jsonStr, "user.age", 31)

// Create new field
result, err := json.SetWithAdd(jsonStr, "user.city", "New York")
```

### Working with Arrays
```go
// Update array element
result, err := json.Set(jsonStr, "users[0].active", false)

// Add new array element
newUser := map[string]any{"id": 4, "name": "David"}
result, err := json.SetWithAdd(jsonStr, "users[3]", newUser)

// Bulk update all array elements
result, err := json.Set(jsonStr, "users{status}", "active")
```

### Creating Complex Structures
```go
// Step-by-step creation
workingJson := "{}"
workingJson, _ = json.SetWithAdd(workingJson, "company.name", "TechCorp")
workingJson, _ = json.SetWithAdd(workingJson, "company.employees[0].name", "John")

// Single operation with complex object
complexObj := map[string]any{
    "name": "Engineering",
    "employees": []map[string]any{
        {"name": "Alice", "role": "Developer"},
    },
}
result, err := json.SetWithAdd("{}", "department", complexObj)
```

## Running the Example

```bash
cd examples/json_set
go run example.go
```

The example will output detailed demonstrations of each operation type with before/after JSON structures and explanatory notes.

## Key Takeaways

1. **Set() vs SetWithAdd()**: Use Set() for existing paths, SetWithAdd() for automatic path creation
2. **ProcessorOptions**: Provide fine-grained control over behavior
3. **Path Syntax**: Supports arrays, objects, ranges, and wildcards
4. **Type Flexibility**: Can set any JSON-compatible data type
5. **Error Handling**: Always check errors for robust applications
6. **Batch Operations**: Chain operations for complex transformations
7. **Performance**: Wildcard operations are efficient for bulk updates

## Related Examples

- `json_delete/`: Demonstrates deletion operations
- `iteration/`: Shows iteration and modification patterns
- `compatibility/`: Covers compatibility features

For more detailed documentation, see the main package documentation and DOCUMENTATION.md.
