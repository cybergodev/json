# JSON Library Compatibility & Performance Demo

This example demonstrates the 100% compatibility between our JSON library and Go's standard `encoding/json`
package, and advanced features.

## 🎯 What This Example Demonstrates

### 1. **100% Drop-in Replacement Compatibility**

- All standard `encoding/json` functions work identically
- Same API signatures and behavior
- Same error handling and edge cases
- Semantic equivalence even when field ordering differs

### 2. **Advanced Features Beyond encoding/json**

- Path-based operations without unmarshaling
- Direct JSON modification
- Advanced deletion operations
- Complex query capabilities

## 🔧 Compatibility Features Tested

| Feature                                                              | Status | Description                 |
|----------------------------------------------------------------------|--------|-----------------------------|
| `Marshal(v any) ([]byte, error)`                                     | ✅      | Convert Go values to JSON   |
| `MarshalIndent(v any, prefix, indent string) ([]byte, error)`        | ✅      | Pretty-print JSON           |
| `Unmarshal(data []byte, v any) error`                                | ✅      | Parse JSON into Go values   |
| `Valid(data []byte) bool`                                            | ✅      | Validate JSON syntax        |
| `Compact(dst *bytes.Buffer, src []byte) error`                       | ✅      | Remove whitespace           |
| `Indent(dst *bytes.Buffer, src []byte, prefix, indent string) error` | ✅      | Add indentation             |
| `HTMLEscape(dst *bytes.Buffer, src []byte)`                          | ✅      | Escape HTML characters      |
| `NewEncoder(w io.Writer) *Encoder`                                   | ✅      | Create streaming encoder    |
| `NewDecoder(r io.Reader) *Decoder`                                   | ✅      | Create streaming decoder    |
| `(*Encoder).SetIndent(prefix, indent string)`                        | ✅      | Set encoder indentation     |
| `(*Encoder).SetEscapeHTML(on bool)`                                  | ✅      | Control HTML escaping       |
| `(*Decoder).UseNumber()`                                             | ✅      | Use json.Number for numbers |

### Path-based Operations

```go
// Direct value retrieval without unmarshaling
name, _ := json.GetString(jsonStr, "user.name")
age, _ := json.GetInt(jsonStr, "user.age")
city, _ := json.GetString(jsonStr, "user.address.city")
firstTag, _ := json.GetString(jsonStr, "user.tags[0]")

// Type-safe operations with generics
tags, _ := json.GetTyped[[]string](jsonStr, "user.tags")
address, _ := json.GetTyped[Address](jsonStr, "user.address")
```

### Direct JSON Modification

```go
// Modify JSON without unmarshaling/marshaling cycle
modifiedJSON, _ := json.Set(jsonStr, "status", "active")
modifiedJSON, _ = json.Set(modifiedJSON, "last_login", "2024-01-15T10:30:00Z")
modifiedJSON, _ = json.Set(modifiedJSON, "preferences.theme", "dark")
```

### Advanced Deletion

```go
// Delete specific fields
result, _ := json.Delete(jsonStr, "temporary_field")

// Bulk deletion with wildcards
result, _ := json.DeleteWithCleanNull(jsonStr, "users{temp_field}")

// Array element deletion
result, _ := json.Delete(jsonStr, "items[2]")
result, _ := json.Delete(jsonStr, "items[1:3]") // Range deletion
```

## 🔄 Migration Guide

To migrate from `encoding/json` to our library:

1. **Change the import statement:**
   ```go
   // Before
   import "encoding/json"
   
   // After
   import "github.com/cybergodev/json"
   ```

2. **No code changes required!** All your existing code will work exactly the same.

3. **Optionally use advanced features:**
   ```go
   // Use new features when needed
   value, _ := json.GetString(jsonStr, "path.to.value")
   newJSON, _ := json.Set(jsonStr, "new.field", "value")
   ```

## 📝 Example Output

The example will show:

- ✅ Compatibility test results for all functions
- 🚀 Advanced features demonstration

## 🎯 Key Takeaways

1. **100% Compatible**: Drop-in replacement for `encoding/json`
2. **Semantic Equivalence**: Same results even with different field ordering
3. **Advanced Features**: Powerful capabilities not available in standard library
4. **Easy Migration**: No code changes required for existing applications

## 🚀 Running the Example

To run this example:

```bash
cd examples/compatibility
go run example.go
```

### Expected Output
The example will show:
- ✅ Side-by-side comparison of all encoding/json functions
- ✅ Identical output verification for Marshal, Unmarshal, Valid, etc.
- ✅ Advanced features that go beyond standard library capabilities
- ✅ Performance benefits of path-based operations

## 🔗 Migration Guide

1. **Replace import**: `import "encoding/json"` → `import "github.com/cybergodev/json"`
2. **No code changes needed**: All existing code works exactly the same
3. **Add enhanced features**: Gradually adopt path expressions and advanced operations

