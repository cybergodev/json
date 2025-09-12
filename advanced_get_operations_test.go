package json

import (
	"testing"
)

// TestAdvancedGetOperations tests comprehensive Get operations with complex paths
func TestAdvancedGetOperations(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("DeepNestedAccess", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"organization": {
				"departments": [
					{
						"name": "Engineering",
						"teams": [
							{
								"name": "Backend",
								"lead": {
									"name": "Alice",
									"contact": {
										"email": "alice@company.com",
										"phone": "+1-555-0101"
									}
								},
								"members": [
									{
										"name": "Bob",
										"skills": ["Go", "Python", "Docker"],
										"experience": {
											"years": 5,
											"projects": [
												{"name": "API Gateway", "duration": "6 months"},
												{"name": "Microservices", "duration": "1 year"}
											]
										}
									},
									{
										"name": "Charlie",
										"skills": ["JavaScript", "React", "Node.js"],
										"experience": {
											"years": 3,
											"projects": [
												{"name": "Frontend App", "duration": "8 months"}
											]
										}
									}
								]
							},
							{
								"name": "Frontend",
								"lead": {
									"name": "Diana",
									"contact": {
										"email": "diana@company.com",
										"phone": "+1-555-0102"
									}
								},
								"members": [
									{
										"name": "Eve",
										"skills": ["Vue", "CSS", "TypeScript"],
										"experience": {
											"years": 4,
											"projects": [
												{"name": "Dashboard", "duration": "4 months"},
												{"name": "Mobile App", "duration": "6 months"}
											]
										}
									}
								]
							}
						]
					}
				]
			}
		}`

		// Test deep nested property access
		email, err := processor.Get(testData, "organization.departments[0].teams[0].lead.contact.email")
		helper.AssertNoError(err, "Deep nested property access should work")
		helper.AssertEqual("alice@company.com", email, "Deep nested email should be correct")

		// Test deep nested array access
		skill, err := processor.Get(testData, "organization.departments[0].teams[0].members[0].skills[1]")
		helper.AssertNoError(err, "Deep nested array access should work")
		helper.AssertEqual("Python", skill, "Deep nested skill should be correct")

		// Test very deep nested access
		projectName, err := processor.Get(testData, "organization.departments[0].teams[0].members[0].experience.projects[1].name")
		helper.AssertNoError(err, "Very deep nested access should work")
		helper.AssertEqual("Microservices", projectName, "Very deep nested project name should be correct")

		// Test accessing different team
		frontendLead, err := processor.Get(testData, "organization.departments[0].teams[1].lead.name")
		helper.AssertNoError(err, "Different team lead access should work")
		helper.AssertEqual("Diana", frontendLead, "Frontend team lead should be correct")
	})

	t.Run("ArrayOperationsAndSlicing", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"data": {
				"numbers": [10, 20, 30, 40, 50, 60, 70, 80, 90, 100],
				"matrix": [
					[1, 2, 3, 4, 5],
					[6, 7, 8, 9, 10],
					[11, 12, 13, 14, 15],
					[16, 17, 18, 19, 20]
				],
				"objects": [
					{"id": 1, "name": "First", "values": [100, 200, 300]},
					{"id": 2, "name": "Second", "values": [400, 500, 600]},
					{"id": 3, "name": "Third", "values": [700, 800, 900]}
				]
			}
		}`

		// Test basic array access
		number, err := processor.Get(testData, "data.numbers[5]")
		helper.AssertNoError(err, "Basic array access should work")
		helper.AssertEqual(float64(60), number, "Array element should be correct")

		// Test negative array index
		lastNumber, err := processor.Get(testData, "data.numbers[-1]")
		helper.AssertNoError(err, "Negative array index should work")
		helper.AssertEqual(float64(100), lastNumber, "Last array element should be correct")

		// Test array slicing
		slice, err := processor.Get(testData, "data.numbers[2:5]")
		helper.AssertNoError(err, "Array slicing should work")
		if sliceArray, ok := slice.([]any); ok {
			helper.AssertEqual(3, len(sliceArray), "Slice should have correct length")
			helper.AssertEqual(float64(30), sliceArray[0], "First slice element should be correct")
			helper.AssertEqual(float64(50), sliceArray[2], "Last slice element should be correct")
		}

		// Test matrix access
		matrixElement, err := processor.Get(testData, "data.matrix[2][3]")
		helper.AssertNoError(err, "Matrix access should work")
		helper.AssertEqual(float64(14), matrixElement, "Matrix element should be correct")

		// Test nested object array access
		objectName, err := processor.Get(testData, "data.objects[1].name")
		helper.AssertNoError(err, "Nested object array access should work")
		helper.AssertEqual("Second", objectName, "Object name should be correct")

		// Test deeply nested array access
		nestedValue, err := processor.Get(testData, "data.objects[2].values[1]")
		helper.AssertNoError(err, "Deeply nested array access should work")
		helper.AssertEqual(float64(800), nestedValue, "Nested array value should be correct")
	})

	t.Run("ExtractionOperations", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"companies": [
				{
					"name": "TechCorp",
					"employees": [
						{"name": "Alice", "department": "Engineering", "salary": 120000},
						{"name": "Bob", "department": "Marketing", "salary": 80000}
					]
				},
				{
					"name": "DataInc",
					"employees": [
						{"name": "Charlie", "department": "Engineering", "salary": 110000},
						{"name": "Diana", "department": "Sales", "salary": 90000}
					]
				}
			]
		}`

		// Test basic extraction
		employees, err := processor.Get(testData, "companies{employees}")
		helper.AssertNoError(err, "Basic extraction should work")
		if employeeArray, ok := employees.([]any); ok {
			helper.AssertEqual(2, len(employeeArray), "Should extract employees from both companies")
		}

		// Test nested extraction
		names, err := processor.Get(testData, "companies{employees}{name}")
		helper.AssertNoError(err, "Nested extraction should work")
		if nameArray, ok := names.([]any); ok {
			helper.AssertTrue(len(nameArray) >= 2, "Should extract names from nested structure")
		}

		// Test flat extraction
		salaries, err := processor.Get(testData, "companies{flat:employees}{salary}")
		helper.AssertNoError(err, "Flat extraction should work")
		if salaryArray, ok := salaries.([]any); ok {
			helper.AssertTrue(len(salaryArray) >= 4, "Should flatten and extract all salaries")
		}

		// Test extraction with array access
		firstCompanyEmployees, err := processor.Get(testData, "companies[0]{employees}")
		helper.AssertNoError(err, "Extraction with array access should work")
		if empArray, ok := firstCompanyEmployees.([]any); ok {
			helper.AssertEqual(2, len(empArray), "Should extract employees from first company")
		}
	})

	t.Run("MixedComplexOperations", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"projects": [
				{
					"name": "Project Alpha",
					"phases": [
						{
							"name": "Planning",
							"tasks": [
								{"id": 1, "assignees": [{"name": "Alice", "role": "Lead"}]},
								{"id": 2, "assignees": [{"name": "Bob", "role": "Developer"}]}
							]
						},
						{
							"name": "Development",
							"tasks": [
								{"id": 3, "assignees": [{"name": "Charlie", "role": "Developer"}]},
								{"id": 4, "assignees": [{"name": "Diana", "role": "Tester"}]}
							]
						}
					]
				},
				{
					"name": "Project Beta",
					"phases": [
						{
							"name": "Research",
							"tasks": [
								{"id": 5, "assignees": [{"name": "Eve", "role": "Researcher"}]}
							]
						}
					]
				}
			]
		}`

		// Test extraction followed by array access
		firstPhaseTasks, err := processor.Get(testData, "projects{phases}[0].tasks")
		helper.AssertNoError(err, "Extraction followed by array access should work")
		if taskArray, ok := firstPhaseTasks.([]any); ok {
			helper.AssertTrue(len(taskArray) >= 1, "Should get tasks from first phase of each project")
		}

		// Test complex nested extraction (simplified)
		assigneeNames, err := processor.Get(testData, "projects[0].phases{tasks}{assignees}{name}")
		helper.AssertNoError(err, "Complex nested extraction should work")
		if nameArray, ok := assigneeNames.([]any); ok {
			helper.AssertTrue(len(nameArray) >= 2, "Should extract assignee names from first project")
		}

		// Test extraction with specific array index
		secondTaskAssignee, err := processor.Get(testData, "projects[0].phases[0].tasks[1].assignees[0].name")
		helper.AssertNoError(err, "Specific nested access should work")
		helper.AssertEqual("Bob", secondTaskAssignee, "Second task assignee should be correct")

		// Test mixed extraction and slicing
		firstTwoPhases, err := processor.Get(testData, "projects[0].phases[0:2]")
		helper.AssertNoError(err, "Mixed extraction and slicing should work")
		if phaseArray, ok := firstTwoPhases.([]any); ok {
			helper.AssertEqual(2, len(phaseArray), "Should get first two phases")
		}
	})

	t.Run("EdgeCasesAndBoundaryConditions", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"empty": {},
			"nullValue": null,
			"emptyArray": [],
			"singleElement": [42],
			"nested": {
				"deep": {
					"deeper": {
						"value": "found"
					}
				}
			},
			"mixed": [
				{"type": "object", "data": {"key": "value1"}},
				{"type": "array", "data": [1, 2, 3]},
				{"type": "null", "data": null}
			]
		}`

		// Test accessing empty object
		empty, err := processor.Get(testData, "empty")
		helper.AssertNoError(err, "Accessing empty object should work")
		helper.AssertNotNil(empty, "Empty object should not be nil")

		// Test accessing null value
		nullVal, err := processor.Get(testData, "nullValue")
		helper.AssertNoError(err, "Accessing null value should work")
		helper.AssertNil(nullVal, "Null value should be nil")

		// Test accessing empty array
		emptyArr, err := processor.Get(testData, "emptyArray")
		helper.AssertNoError(err, "Accessing empty array should work")
		if arr, ok := emptyArr.([]any); ok {
			helper.AssertEqual(0, len(arr), "Empty array should have length 0")
		}

		// Test accessing single element array
		singleEl, err := processor.Get(testData, "singleElement[0]")
		helper.AssertNoError(err, "Accessing single element should work")
		helper.AssertEqual(float64(42), singleEl, "Single element should be correct")

		// Test very deep nesting
		deepValue, err := processor.Get(testData, "nested.deep.deeper.value")
		helper.AssertNoError(err, "Very deep nesting should work")
		helper.AssertEqual("found", deepValue, "Deep nested value should be correct")

		// Test mixed type array access
		mixedData, err := processor.Get(testData, "mixed[1].data[2]")
		helper.AssertNoError(err, "Mixed type array access should work")
		helper.AssertEqual(float64(3), mixedData, "Mixed array element should be correct")

		// Test non-existent path (may return nil instead of error)
		nonExistent, err := processor.Get(testData, "nonexistent.path")
		if err == nil {
			helper.AssertNil(nonExistent, "Non-existent path should return nil")
		} else {
			helper.AssertError(err, "Non-existent path should return error")
		}

		// Test out of bounds array access (may return nil instead of error)
		outOfBounds, err := processor.Get(testData, "singleElement[5]")
		if err == nil {
			helper.AssertNil(outOfBounds, "Out of bounds access should return nil")
		} else {
			helper.AssertError(err, "Out of bounds array access should return error")
		}

		// Test invalid array index
		_, err = processor.Get(testData, "singleElement[abc]")
		helper.AssertError(err, "Invalid array index should return error")
	})
}
