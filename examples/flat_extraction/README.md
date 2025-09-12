# Flat Extraction and Array Operations Example

This example demonstrates the powerful flat extraction feature and its interaction with array operations in the JSON library.

## Overview

The JSON library supports two types of extraction operations:

1. **Regular Extraction** (`{field}`) - Preserves nested array structure
2. **Flat Extraction** (`{flat:field}`) - Flattens all arrays into a single array

When combined with array operations (`[index]`, `[start:end]`), these extractions behave very differently and serve different use cases.

## Key Concepts

### Regular Extraction vs Flat Extraction

```go
// Regular extraction - preserves structure
regularSkills := json.Get(data, "company.departments{teams}{members}{skills}")
// Result: [[[Go Python] [Java Spring]] [[React Vue]]]

// Flat extraction - flattens everything
flatSkills := json.Get(data, "company.departments{teams}{members}{flat:skills}")
// Result: [Go Python Java Spring React Vue]
```

### Array Operations on Different Extraction Types

#### Flat Extraction + Array Operations
```go
// Get first element from flattened array
firstSkill := json.Get(data, "company.departments{teams}{members}{flat:skills}[0]")
// Result: "Go" (single element)

// Get slice from flattened array
skillSlice := json.Get(data, "company.departments{teams}{members}{flat:skills}[1:4]")
// Result: [Python Java Spring] (slice of flattened array)
```

#### Distributed Array Operations
```go
// Get first element from each person's skills array
firstFromEach := json.Get(data, "company.departments{teams}{members}{skills}[0]")
// Result: [Go Java React] (first from each individual array)

// Get slices from each person's skills array
slicesFromEach := json.Get(data, "company.departments{teams}{members}{skills}[0:2]")
// Result: [[Go Python] [Java Spring] [React Vue]] (slice from each array)
```

## Use Cases

### When to Use Flat Extraction

✅ **Perfect for:**
- Counting total elements across all arrays
- Finding unique values across all arrays
- Simple iteration over all values
- Aggregation operations
- Search operations across all data

```go
// Count total skills
allSkills := json.Get(data, "company.departments{teams}{members}{flat:skills}")
totalCount := len(allSkills.([]any))

// Get all unique skills
uniqueSkills := make(map[string]bool)
for _, skill := range allSkills.([]any) {
    uniqueSkills[skill.(string)] = true
}
```

### When to Use Distributed Operations

✅ **Perfect for:**
- Applying the same operation to each individual array
- Preserving relationships between arrays and their sources
- Getting corresponding elements from multiple arrays
- Element-wise modifications

```go
// Get primary skill for each person
primarySkills := json.Get(data, "company.departments{teams}{members}{skills}[0]")

// Set primary skill for each person
json.Set(data, "company.departments{teams}{members}{skills}[0]", "NewPrimarySkill")
```

## Running the Example

```bash
cd examples/flat_extraction
go run example.go
```

## Expected Output

The example will demonstrate:

1. **Basic flat extraction** - showing how flat extraction differs from regular extraction
2. **Array operations on flat extractions** - accessing elements in flattened arrays
3. **Distributed array operations** - applying operations to each individual array
4. **Set operations** - modifying data using both approaches
5. **Performance guidance** - when to use each approach

## Key Takeaways

- **Flat extraction** creates a single flattened array from all nested arrays
- **Distributed operations** apply array operations to each individual array separately
- **Array operations** work differently depending on the extraction type
- **Set operations** support both flat and distributed modification patterns
- Choose the approach based on whether you need **aggregated data** (flat) or **element-wise operations** (distributed)
