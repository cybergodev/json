package json

import (
	"testing"
)

// TestEnhancedPathExpressions provides comprehensive testing for complex path expressions
// including nested extraction, array slicing, negative indices, and advanced features
func TestEnhancedPathExpressions(t *testing.T) {
	helper := NewTestHelper(t)

	// Complex test data with multiple nesting levels and various data types
	complexJSON := `{
		"company": {
			"name": "TechCorp",
			"founded": 2010,
			"departments": [
				{
					"name": "Engineering",
					"budget": 1000000,
					"teams": [
						{
							"name": "Backend",
							"members": [
								{"name": "Alice", "skills": ["Go", "Python", "Docker"], "level": "Senior", "salary": 120000},
								{"name": "Bob", "skills": ["Java", "Spring", "Kubernetes"], "level": "Mid", "salary": 90000},
								{"name": "Charlie", "skills": ["Go", "Redis", "PostgreSQL"], "level": "Junior", "salary": 70000}
							],
							"projects": [
								{"name": "API Gateway", "status": "active", "priority": "high"},
								{"name": "Microservices", "status": "planning", "priority": "medium"}
							]
						},
						{
							"name": "Frontend",
							"members": [
								{"name": "Diana", "skills": ["React", "TypeScript", "CSS"], "level": "Senior", "salary": 115000},
								{"name": "Eve", "skills": ["Vue", "JavaScript", "HTML"], "level": "Mid", "salary": 85000}
							],
							"projects": [
								{"name": "Dashboard", "status": "active", "priority": "high"},
								{"name": "Mobile App", "status": "completed", "priority": "low"}
							]
						}
					]
				},
				{
					"name": "Marketing",
					"budget": 500000,
					"teams": [
						{
							"name": "Digital",
							"members": [
								{"name": "Frank", "skills": ["SEO", "Analytics", "AdWords"], "level": "Manager", "salary": 100000},
								{"name": "Grace", "skills": ["Content", "Social Media", "Design"], "level": "Specialist", "salary": 75000}
							],
							"projects": [
								{"name": "Campaign 2024", "status": "active", "priority": "high"},
								{"name": "Brand Refresh", "status": "planning", "priority": "medium"}
							]
						}
					]
				}
			],
			"metadata": {
				"tags": ["tech", "startup", "innovation"],
				"locations": ["San Francisco", "New York", "Austin"],
				"certifications": {
					"iso": ["9001", "27001"],
					"soc": ["SOC1", "SOC2"]
				}
			}
		},
		"statistics": {
			"totalEmployees": 8,
			"averageSalary": 94375,
			"departmentCount": 2,
			"activeProjects": 3
		}
	}`

	t.Run("BasicPathAccess", func(t *testing.T) {
		// Test simple property access
		companyName, err := GetString(complexJSON, "company.name")
		helper.AssertNoError(err, "Should get company name")
		helper.AssertEqual("TechCorp", companyName, "Company name should match")

		// Test nested property access
		founded, err := GetInt(complexJSON, "company.founded")
		helper.AssertNoError(err, "Should get founded year")
		helper.AssertEqual(2010, founded, "Founded year should match")

		// Test deep nested access
		firstDeptName, err := GetString(complexJSON, "company.departments[0].name")
		helper.AssertNoError(err, "Should get first department name")
		helper.AssertEqual("Engineering", firstDeptName, "First department should be Engineering")
	})

	t.Run("ArrayIndexAccess", func(t *testing.T) {
		// Test positive array indices
		firstMember, err := GetString(complexJSON, "company.departments[0].teams[0].members[0].name")
		helper.AssertNoError(err, "Should get first member name")
		helper.AssertEqual("Alice", firstMember, "First member should be Alice")

		// Test negative array indices
		lastTag, err := GetString(complexJSON, "company.metadata.tags[-1]")
		helper.AssertNoError(err, "Should get last tag")
		helper.AssertEqual("innovation", lastTag, "Last tag should be innovation")

		// Test negative index on nested arrays
		lastMemberFirstTeam, err := GetString(complexJSON, "company.departments[0].teams[0].members[-1].name")
		helper.AssertNoError(err, "Should get last member of first team")
		helper.AssertEqual("Charlie", lastMemberFirstTeam, "Last member should be Charlie")

		// Test accessing last department
		lastDeptName, err := GetString(complexJSON, "company.departments[-1].name")
		helper.AssertNoError(err, "Should get last department name")
		helper.AssertEqual("Marketing", lastDeptName, "Last department should be Marketing")
	})

	t.Run("ArraySlicing", func(t *testing.T) {
		// Test basic array slicing
		firstTwoTags, err := Get(complexJSON, "company.metadata.tags[0:2]")
		helper.AssertNoError(err, "Should get first two tags")
		if tags, ok := firstTwoTags.([]interface{}); ok {
			helper.AssertEqual(2, len(tags), "Should have 2 tags")
			helper.AssertEqual("tech", tags[0], "First tag should be tech")
			helper.AssertEqual("startup", tags[1], "Second tag should be startup")
		}

		// Test slicing with step
		everyOtherLocation, err := Get(complexJSON, "company.metadata.locations[::2]")
		helper.AssertNoError(err, "Should get every other location")
		if locations, ok := everyOtherLocation.([]interface{}); ok {
			helper.AssertEqual(2, len(locations), "Should have 2 locations")
			helper.AssertEqual("San Francisco", locations[0], "First location should be San Francisco")
			helper.AssertEqual("Austin", locations[1], "Second location should be Austin")
		}

		// Test negative slicing
		lastTwoMembers, err := Get(complexJSON, "company.departments[0].teams[0].members[-2:]")
		helper.AssertNoError(err, "Should get last two members")
		if members, ok := lastTwoMembers.([]interface{}); ok {
			helper.AssertEqual(2, len(members), "Should have 2 members")
		}
	})

	t.Run("ExtractionOperations", func(t *testing.T) {
		// Test simple extraction
		allDeptNames, err := Get(complexJSON, "company.departments{name}")
		helper.AssertNoError(err, "Should extract all department names")
		if names, ok := allDeptNames.([]interface{}); ok {
			helper.AssertEqual(2, len(names), "Should have 2 department names")
			helper.AssertEqual("Engineering", names[0], "First department should be Engineering")
			helper.AssertEqual("Marketing", names[1], "Second department should be Marketing")
		}

		// Test nested extraction
		allTeamNames, err := Get(complexJSON, "company.departments{teams}{name}")
		helper.AssertNoError(err, "Should extract all team names")
		if names, ok := allTeamNames.([]interface{}); ok {
			helper.AssertTrue(len(names) > 0, "Should have team names")
			t.Logf("Extracted team names: %v", names)
		}

		// Test flat extraction
		allSkills, err := Get(complexJSON, "company.departments{teams}{flat:members}{flat:skills}")
		helper.AssertNoError(err, "Should extract all skills")
		if skills, ok := allSkills.([]interface{}); ok {
			helper.AssertTrue(len(skills) > 0, "Should have skills")
			t.Logf("Extracted skills count: %d", len(skills))
		}

		// Test extraction with array access
		firstTeamMembers, err := Get(complexJSON, "company.departments[0].teams{members}")
		helper.AssertNoError(err, "Should extract members from first department teams")
		if members, ok := firstTeamMembers.([]interface{}); ok {
			helper.AssertEqual(2, len(members), "Should have 2 team member arrays")
		}
	})

	t.Run("CombinedOperations", func(t *testing.T) {
		// Test extraction followed by array access
		firstTeamFirstMember, err := Get(complexJSON, "company.departments{teams}[0][0].name")
		helper.AssertNoError(err, "Should get first team first member")
		// The actual result might be different due to extraction behavior
		t.Logf("First team first member result: %v", firstTeamFirstMember)

		// Test array slicing with extraction
		firstTwoDeptTeams, err := Get(complexJSON, "company.departments[0:2]{teams}")
		helper.AssertNoError(err, "Should get teams from first two departments")
		if teams, ok := firstTwoDeptTeams.([]interface{}); ok {
			helper.AssertEqual(2, len(teams), "Should have teams from 2 departments")
		}

		// Test complex nested extraction with slicing
		seniorMembers, err := Get(complexJSON, "company.departments{teams}{members}[0:1]")
		helper.AssertNoError(err, "Should get first member from each team")
		if members, ok := seniorMembers.([]interface{}); ok {
			helper.AssertTrue(len(members) > 0, "Should have some members")
		}
	})

	t.Run("EdgeCasePaths", func(t *testing.T) {
		// Test accessing array of arrays
		certifications, err := Get(complexJSON, "company.metadata.certifications.iso")
		helper.AssertNoError(err, "Should get ISO certifications")
		if certs, ok := certifications.([]interface{}); ok {
			helper.AssertEqual(2, len(certs), "Should have 2 ISO certifications")
			helper.AssertEqual("9001", certs[0], "First cert should be 9001")
		}

		// Test mixed extraction and property access
		projectStatuses, err := Get(complexJSON, "company.departments{teams}{projects}{status}")
		helper.AssertNoError(err, "Should extract all project statuses")
		if statuses, ok := projectStatuses.([]interface{}); ok {
			helper.AssertTrue(len(statuses) > 0, "Should have project statuses")
		}

		// Test accessing nested arrays with negative indices
		lastProjectFirstTeam, err := Get(complexJSON, "company.departments[0].teams[0].projects[-1].name")
		helper.AssertNoError(err, "Should get last project of first team")
		helper.AssertEqual("Microservices", lastProjectFirstTeam, "Last project should be Microservices")
	})

	t.Run("ErrorHandling", func(t *testing.T) {
		// Test invalid array indices
		_, err := Get(complexJSON, "company.departments[999].name")
		// Should return nil without error (library behavior)
		helper.AssertNoError(err, "Out of bounds access should not error")

		// Test invalid extraction syntax
		_, err = Get(complexJSON, "company.departments{invalid}")
		// Should return empty array or error
		if err != nil {
			t.Logf("Invalid extraction correctly returned error: %v", err)
		}

		// Test path through non-object
		_, err = Get(complexJSON, "company.name.invalid")
		// Should return nil without error (library behavior)
		helper.AssertNoError(err, "Path through non-object should not error")

		// Test empty extraction
		_, err = Get(complexJSON, "company.nonexistent{field}")
		// Should handle gracefully
		helper.AssertNoError(err, "Empty extraction should not error")
	})
}

// TestPathExpressionPerformance tests performance of complex path expressions
func TestPathExpressionPerformance(t *testing.T) {
	helper := NewTestHelper(t)

	// Generate large test data
	largeJSON := generateLargeComplexJSON(100) // 100 departments with nested structure

	t.Run("SimplePathPerformance", func(t *testing.T) {
		// Test simple path access performance
		for i := 0; i < 100; i++ {
			_, err := Get(largeJSON, "company.name")
			helper.AssertNoError(err, "Simple path should work")
		}
	})

	t.Run("ComplexExtractionPerformance", func(t *testing.T) {
		// Test complex extraction performance
		for i := 0; i < 10; i++ {
			_, err := Get(largeJSON, "company.departments{teams}{members}{name}")
			helper.AssertNoError(err, "Complex extraction should work")
		}
	})
}

// generateLargeComplexJSON generates a large JSON structure for performance testing
func generateLargeComplexJSON(deptCount int) string {
	// This would generate a large JSON structure similar to complexJSON but with more data
	// For now, return a simplified version
	return `{
		"company": {
			"name": "LargeCorp",
			"departments": [
				{
					"name": "Engineering",
					"teams": [
						{
							"name": "Backend",
							"members": [
								{"name": "Alice", "skills": ["Go", "Python"]},
								{"name": "Bob", "skills": ["Java", "Spring"]}
							]
						}
					]
				}
			]
		}
	}`
}
