# JSON File Marshal/Unmarshal Example

This example demonstrates the new `MarshalToFile` and `UnmarshalFromFile` functions that provide convenient file I/O operations for JSON data.

## Features Demonstrated

1. **MarshalToFile** - Convert Go data structures to JSON and save to files
2. **UnmarshalFromFile** - Read JSON files and convert to Go data structures
3. **Pretty formatting** - Save JSON with indentation for readability
4. **Compact formatting** - Save JSON without extra whitespace
5. **Processor methods** - Use advanced processor for file operations
6. **Error handling** - Proper error handling for common failure scenarios

## Key Functions

### Package-level Functions

```go
// Marshal data to file (compact by default)
err := json.MarshalToFile("data.json", data)

// Marshal data to file with pretty formatting
err := json.MarshalToFile("data.json", data, true)

// Unmarshal data from file
var result MyStruct
err := json.UnmarshalFromFile("data.json", &result)
```

### Processor Methods

```go
processor := json.New()
defer processor.Close()

// Marshal with processor-specific configuration
err := processor.MarshalToFile("data.json", data, true)

// Unmarshal with processor options
err := processor.UnmarshalFromFile("data.json", &result, opts...)
```

## Running the Example

```bash
# Run the example
go run -tags examples examples/marshal_file/example.go

# Or from the project root
go run -tags examples ./examples/marshal_file/
```

## Expected Output

The example will:

1. Create an `output/` directory
2. Generate several JSON files with different formatting
3. Demonstrate loading data back from files
4. Show error handling for common failure cases
5. Display the contents and operations performed

## Generated Files

- `user_compact.json` - User data in compact format
- `user_pretty.json` - User data with pretty formatting
- `config.json` - Configuration data as a map
- `advanced.json` - Data processed with advanced processor

## Security Features

The file operations include built-in security features:

- **Path validation** - Prevents path traversal attacks
- **Size limits** - Enforces maximum file size limits
- **Directory creation** - Automatically creates parent directories
- **Safe file permissions** - Uses secure file permissions (0644)

## Error Handling

The example demonstrates proper error handling for:

- Non-existent files
- Invalid file paths
- Nil unmarshal targets
- JSON parsing errors
- File system errors

## Use Cases

These functions are ideal for:

- **Configuration files** - Loading and saving application settings
- **Data persistence** - Storing application state
- **API responses** - Caching JSON responses to files
- **Data exchange** - Sharing structured data between applications
- **Backup and restore** - Creating JSON backups of data structures

## Performance Notes

- Files are read/written in a single operation for efficiency
- Directory creation is automatic and safe
- Memory usage is optimized for large files
- Thread-safe operations when using processor methods