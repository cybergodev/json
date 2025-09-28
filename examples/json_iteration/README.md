# JSON Iteration Example

This example demonstrates the comprehensive JSON iteration capabilities of the `github.com/cybergodev/json` library, showcasing advanced features for traversing, accessing, and modifying JSON data structures.

## 🚀 Features Demonstrated

### Core Iteration Functions
- **`json.Foreach()`** - Iterate over JSON objects and arrays with read-only access
- **`json.ForeachReturn()`** - Iterate with modification capabilities and return modified JSON
- **`ForeachReturnNested()`** - Perform nested iterations with deep structure modifications

### Advanced Path Operations
- **Dot notation access**: `"address.city"`, `"user.profile.name"`
- **Array indexing**: `"users[0]"`, `"items[-1]"` (negative indexing supported)
- **Array slicing**: `"numbers[1:3]"`, `"strings[::2]"`, `"contacts[-3:]"`
- **Batch operations**: `"contacts[-2:]{status}"` (apply to multiple elements)

### Type-Safe Value Retrieval
- `GetString()`, `GetInt()`, `GetBool()`, `GetFloat64()`
- `GetObject()`, `GetArray()`
- Generic type retrieval: `GetIterableValue[T]()`
- Automatic type conversion from strings to numbers

### Flow Control
- `Continue()` - Skip current iteration
- `Break()` - Terminate iteration early

### Data Modification Operations
- `Set()` - Modify existing values or create new ones
- `SetWithAdd()` - Add new nested structures
- `Delete()` - Remove elements (with array compaction)

## 📋 Example Scenarios

### 1. Basic Object Iteration (`iterableJsonObject`)
Demonstrates reading from a complex JSON object with nested structures:

```go
json.Foreach(jsonStr, func(key any, item *json.IterableValue) {
    if "name" == key.(string) {
        // Type-safe access
        fmt.Println(item.GetString("name"))
        fmt.Println(item.GetInt("age"))
        
        // Deep path access with slicing
        fmt.Println(item.Get("strings[::2]"))           // Every 2nd element
        fmt.Println(item.Get("address.contacts[1:3]"))  // Elements 1-2
        fmt.Println(item.Get("address.contacts[-1].name")) // Last element's name
        
        // Batch operations
        fmt.Println(item.Get("address.contacts[-2:]{status}")) // Status of last 2 contacts
    }
})
```

### 2. Array Iteration with Flow Control (`iterableJsonArray`)
Shows iteration over JSON arrays with conditional logic:

```go
json.Foreach(jsonStr, func(key any, item *json.IterableValue) {
    keyInt := key.(int)
    id := item.GetInt("id")
    
    if keyInt == 1 {
        item.Continue() // Skip this iteration
    }
    
    if item.GetString("status") == "banned" {
        item.Break() // End iteration
    }
})
```

### 3. Object Modification (`iterableObjectAndReturn`)
Demonstrates modifying JSON structure during iteration:

```go
result, _ := json.ForeachReturn(jsonStr, func(key any, item *json.IterableValue) {
    keyStr := key.(string)
    
    if keyStr == "name" {
        // Add new nested structures
        item.SetWithAdd("backup.temps[2:5]", []string{"reading", "coding", "music"})
        item.SetWithAdd("hobbies", []string{"reading", "coding", "music"})
        
        // Modify existing values
        item.Set("address.zipcode", 10001)
        item.Set("address.contacts[-2:]{status}", "active") // Batch update
        
        // Delete array elements (with compaction)
        item.Delete("address.contacts[0:3]") // Removes elements 0,1,2
    }
    
    if keyStr == "deprecated" {
        item.Delete("deprecated") // Delete current key
    }
})
```

### 4. Array Element Manipulation (`iterableArrayAndReturn`)
Shows how to modify individual array elements:

```go
result, _ := json.ForeachReturn(jsonStr, func(key any, item *json.IterableValue) {
    id := item.GetInt("id")
    name := item.GetString("name")
    
    if id == 1 {
        item.Delete("age") // Remove age field
    }
    
    if name == "Bob" {
        item.Set("level", "vip")
        item.Set("birthday", time.Now().Format("2006-01-02"))
    }
    
    if name == "Carol" {
        // Add nested structures
        item.SetWithAdd("other.score", []int{11, 22, 33})
        item.SetWithAdd("other.hobby", []string{"reading", "coding", "music"})
    }
    
    if item.GetString("status") == "banned" {
        item.Delete("") // Delete entire current element
    }
})
```

### 5. Nested Iteration (`iterationNested`)
Demonstrates complex nested operations on hierarchical data:

```go
json.ForeachReturn(complexData, func(key any, item *json.IterableValue) {
    if key == "company" {
        // First level: departments
        item.ForeachReturnNested("departments", func(deptKey any, deptItem *json.IterableValue) {
            deptName := deptItem.GetString("name")
            
            // Add department metadata
            deptItem.Set("status", "active")
            deptItem.Set("id", fmt.Sprintf("dept-%v", deptKey))
            
            // Second level: employees
            deptItem.ForeachReturnNested("employees", func(empKey any, empItem *json.IterableValue) {
                empName := empItem.GetString("name")
                empSalary := empItem.GetInt("salary")
                
                // Apply salary adjustments
                if empItem.GetString("level") == "senior" {
                    newSalary := empSalary + 5000
                    empItem.Set("salary", newSalary)
                    empItem.Set("bonus", 5000)
                }
            })
        })
    }
})
```

### 6. Deep Nested Processing (`deepNestedExample`)
Shows three-level nested iteration (company → departments → teams → members):

```go
// Three levels of nesting: company → departments → teams → members
item.ForeachReturnNested("departments", func(deptKey any, deptItem *json.IterableValue) {
    deptItem.ForeachReturnNested("teams", func(teamKey any, teamItem *json.IterableValue) {
        teamItem.ForeachReturnNested("members", func(memberKey any, memberItem *json.IterableValue) {
            // Process individual members at the deepest level
            level := memberItem.GetString("level")
            if level == "Senior" {
                salary := memberItem.GetInt("salary")
                memberItem.Set("salary", salary + 5000)
                memberItem.Set("bonus", 5000)
            }
        })
    })
})
```

### 7. Utility Methods (`utilityMethodsExample`)
Demonstrates comprehensive usage of IterableValue utility methods for data validation and analysis:

```go
json.Foreach(jsonStr, func(key any, item *json.IterableValue) {
    if keyStr == "user" {
        // 1. Existence checking
        fmt.Printf("user.email exists: %t\n", item.Exists("email"))
        fmt.Printf("user.phone exists: %t\n", item.Exists("phone"))
        fmt.Printf("user.projects[0].name exists: %t\n", item.Exists("projects[0].name"))

        // 2. Null value detection
        fmt.Printf("user.temp_data is null: %t\n", item.IsNull("temp_data"))
        fmt.Printf("user.profile.metadata is null: %t\n", item.IsNull("profile.metadata"))

        // 3. Empty value detection
        fmt.Printf("user.profile.bio is empty: %t\n", item.IsEmpty("profile.bio"))
        fmt.Printf("user.profile.skills is empty: %t\n", item.IsEmpty("profile.skills"))
        fmt.Printf("user.settings is empty: %t\n", item.IsEmpty("settings"))

        // 4. Length measurement
        fmt.Printf("user.name length: %d\n", item.Length("name"))
        fmt.Printf("user.projects length: %d\n", item.Length("projects"))
        fmt.Printf("user.profile.preferences length: %d\n", item.Length("profile.preferences"))

        // 5. Object structure analysis
        userKeys := item.Keys("")
        profileKeys := item.Keys("profile")
        fmt.Printf("User keys: %v\n", userKeys)
        fmt.Printf("Profile keys: %v\n", profileKeys)

        // 6. Value extraction
        tagsValues := item.Values("profile.tags")
        preferencesValues := item.Values("profile.preferences")
        fmt.Printf("Tags: %v\n", tagsValues)
        fmt.Printf("Preferences: %v\n", preferencesValues)

        // 7. Practical validation scenarios
        if item.Exists("email") && !item.IsEmpty("email") {
            fmt.Printf("✓ User has valid email: %s\n", item.GetString("email"))
        }

        if item.IsEmpty("profile.bio") {
            fmt.Println("⚠ User bio is empty - might need completion")
        }

        skillsLength := item.Length("profile.skills")
        if skillsLength == 0 {
            fmt.Println("📝 User has no skills listed - recommend adding some")
        } else {
            fmt.Printf("👍 User has %d skills listed\n", skillsLength)
        }

        // 8. Dynamic processing based on structure
        projectsLength := item.Length("projects")
        if projectsLength > 0 {
            fmt.Printf("🚀 User is working on %d projects\n", projectsLength)
            for i := 0; i < projectsLength; i++ {
                projectPath := fmt.Sprintf("projects[%d]", i)
                if item.Exists(projectPath + ".status") {
                    status := item.GetString(projectPath + ".status")
                    name := item.GetString(projectPath + ".name")
                    fmt.Printf("   - %s: %s\n", name, status)
                }
            }
        }
    }
})
```

#### Available Utility Methods:

- **`Exists(path string) bool`** - Check if a path exists in the data
- **`IsNull(path string) bool`** - Check if a path contains a null value
- **`IsEmpty(path string) bool`** - Check if a path contains an empty value (empty string, array, or object)
- **`Length(path string) int`** - Get the length of arrays, objects, or strings
- **`Keys(path string) []string`** - Get the keys of an object at the given path
- **`Values(path string) []any`** - Get the values of an object or array at the given path

These utility methods are essential for:
- **Data validation** - Checking existence, null values, and empty states
- **Conditional processing** - Making decisions based on data structure
- **Dynamic analysis** - Understanding object structure at runtime
- **Robust iteration** - Safely processing variable data structures

## 📊 Key Benefits

1. **Powerful Path Syntax**: Access deeply nested data with simple string paths
2. **Array Operations**: Python-style slicing and negative indexing
3. **Type Safety**: Automatic type conversion and generic support
4. **Flow Control**: Skip or break iterations based on conditions
5. **In-Place Modification**: Modify JSON structure during iteration
6. **Nested Processing**: Handle complex hierarchical data structures
7. **Batch Operations**: Apply changes to multiple elements at once

## 🔧 Use Cases

- **Data Transformation**: Convert JSON structures between different formats
- **Conditional Processing**: Apply custom logic during JSON traversal
- **Data Enrichment**: Add computed fields or metadata to existing JSON
- **Data Cleaning**: Remove unwanted fields or normalize data
- **Complex Queries**: Extract specific data from nested structures
- **Batch Updates**: Apply changes to multiple similar elements

## 🚀 Running the Example

To run this example:

```bash
cd examples/json_iteration
go run example.go
```

### Expected Output
The example will demonstrate:
- ✅ Object and array iteration with various callback patterns
- ✅ Nested iteration for complex data structures
- ✅ Data modification during iteration
- ✅ Utility methods for data analysis (Exists, IsNull, IsEmpty, etc.)
- ✅ Performance comparisons and best practices

## 🔗 Related Examples

- [**basic**](../basic/) - Learn basic operations first
- [**json_get**](../json_get/) - Understand path expressions
- [**json_set**](../json_set/) - Learn about data modification

