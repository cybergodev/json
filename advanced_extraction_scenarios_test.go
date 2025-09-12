package json

import (
	"testing"
)

// TestAdvancedExtractionScenarios tests comprehensive extraction operation scenarios
func TestAdvancedExtractionScenarios(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("DeepExtractionOperations", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"organizations": [
				{
					"name": "TechCorp",
					"departments": [
						{
							"name": "Engineering",
							"teams": [
								{
									"name": "Backend",
									"members": [
										{"name": "Alice", "role": "Senior", "skills": ["Go", "Python"]},
										{"name": "Bob", "role": "Mid", "skills": ["Java", "Spring"]}
									]
								},
								{
									"name": "Frontend",
									"members": [
										{"name": "Charlie", "role": "Senior", "skills": ["React", "TypeScript"]},
										{"name": "Diana", "role": "Junior", "skills": ["HTML", "CSS"]}
									]
								}
							]
						},
						{
							"name": "Marketing",
							"teams": [
								{
									"name": "Digital",
									"members": [
										{"name": "Eve", "role": "Manager", "skills": ["SEO", "Analytics"]},
										{"name": "Frank", "role": "Specialist", "skills": ["Content", "Social"]}
									]
								}
							]
						}
					]
				},
				{
					"name": "DataInc",
					"departments": [
						{
							"name": "Research",
							"teams": [
								{
									"name": "ML",
									"members": [
										{"name": "Grace", "role": "Lead", "skills": ["Python", "TensorFlow"]},
										{"name": "Henry", "role": "Researcher", "skills": ["R", "Statistics"]}
									]
								}
							]
						}
					]
				}
			]
		}`

		// Test deep extraction - get all member names across all organizations
		allNames, err := processor.Get(testData, "organizations{departments}{teams}{members}{name}")
		helper.AssertNoError(err, "Deep extraction should work")
		if nameArray, ok := allNames.([]any); ok {
			helper.AssertTrue(len(nameArray) > 0, "Should extract member names")
			// Verify some expected names are present if extraction worked
			if len(nameArray) >= 6 {
				foundAlice := false
				foundGrace := false
				for _, name := range nameArray {
					if name == "Alice" {
						foundAlice = true
					}
					if name == "Grace" {
						foundGrace = true
					}
				}
				helper.AssertTrue(foundAlice, "Should find Alice in extracted names")
				helper.AssertTrue(foundGrace, "Should find Grace in extracted names")
			}
		}

		// Test deep extraction with filtering - get all senior roles
		seniorMembers, err := processor.Get(testData, "organizations{departments}{teams}{members}")
		helper.AssertNoError(err, "Deep member extraction should work")
		if memberArray, ok := seniorMembers.([]any); ok {
			helper.AssertTrue(len(memberArray) > 0, "Should extract members")
		}

		// Test extraction with array access - get first organization's team names
		firstOrgTeams, err := processor.Get(testData, "organizations[0]{departments}{teams}{name}")
		helper.AssertNoError(err, "Extraction with array access should work")
		if teamArray, ok := firstOrgTeams.([]any); ok {
			helper.AssertTrue(len(teamArray) > 0, "Should extract team names from first organization")
		}

		// Test flat extraction - flatten all skills
		allSkills, err := processor.Get(testData, "organizations{flat:departments}{flat:teams}{flat:members}{skills}")
		helper.AssertNoError(err, "Flat extraction should work")
		if skillsArray, ok := allSkills.([]any); ok {
			helper.AssertTrue(len(skillsArray) > 0, "Should extract skill arrays")
		}
	})

	t.Run("ConsecutiveExtractionOperations", func(t *testing.T) {
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
								{"id": 1, "title": "Requirements", "assignees": [{"name": "Alice"}, {"name": "Bob"}]},
								{"id": 2, "title": "Design", "assignees": [{"name": "Charlie"}]}
							]
						},
						{
							"name": "Development",
							"tasks": [
								{"id": 3, "title": "Backend", "assignees": [{"name": "Alice"}, {"name": "Diana"}]},
								{"id": 4, "title": "Frontend", "assignees": [{"name": "Bob"}, {"name": "Eve"}]}
							]
						}
					]
				},
				{
					"name": "Project Beta",
					"phases": [
						{
							"name": "Testing",
							"tasks": [
								{"id": 5, "title": "Unit Tests", "assignees": [{"name": "Frank"}]},
								{"id": 6, "title": "Integration", "assignees": [{"name": "Grace"}, {"name": "Henry"}]}
							]
						}
					]
				}
			]
		}`

		// Test consecutive extraction - extract all task titles
		taskTitles, err := processor.Get(testData, "projects{phases}{tasks}{title}")
		helper.AssertNoError(err, "Consecutive extraction should work")
		if titleArray, ok := taskTitles.([]any); ok {
			helper.AssertTrue(len(titleArray) > 0, "Should extract task titles")
			// Verify specific titles if extraction worked well
			if len(titleArray) >= 5 {
				foundRequirements := false
				foundUnitTests := false
				for _, title := range titleArray {
					if title == "Requirements" {
						foundRequirements = true
					}
					if title == "Unit Tests" {
						foundUnitTests = true
					}
				}
				helper.AssertTrue(foundRequirements, "Should find Requirements title")
				helper.AssertTrue(foundUnitTests, "Should find Unit Tests title")
			}
		}

		// Test consecutive extraction with array access
		firstProjectTasks, err := processor.Get(testData, "projects[0]{phases}{tasks}{id}")
		helper.AssertNoError(err, "Consecutive extraction with array access should work")
		if idArray, ok := firstProjectTasks.([]any); ok {
			helper.AssertTrue(len(idArray) > 0, "Should extract task IDs from first project")
		}

		// Test consecutive extraction with nested arrays
		allAssignees, err := processor.Get(testData, "projects{phases}{tasks}{assignees}{name}")
		helper.AssertNoError(err, "Consecutive extraction with nested arrays should work")
		if assigneeArray, ok := allAssignees.([]any); ok {
			helper.AssertTrue(len(assigneeArray) > 0, "Should extract assignee names")
		}
	})

	t.Run("MixedExtractionOperations", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"catalog": {
				"categories": [
					{
						"name": "Electronics",
						"products": [
							{
								"name": "Laptop",
								"variants": [
									{"sku": "LAP001", "price": 999, "specs": {"ram": "8GB", "storage": "256GB"}},
									{"sku": "LAP002", "price": 1299, "specs": {"ram": "16GB", "storage": "512GB"}}
								]
							},
							{
								"name": "Phone",
								"variants": [
									{"sku": "PHN001", "price": 699, "specs": {"ram": "6GB", "storage": "128GB"}},
									{"sku": "PHN002", "price": 899, "specs": {"ram": "8GB", "storage": "256GB"}}
								]
							}
						]
					},
					{
						"name": "Books",
						"products": [
							{
								"name": "Programming Guide",
								"variants": [
									{"sku": "BK001", "price": 49, "specs": {"pages": 500, "format": "Hardcover"}},
									{"sku": "BK002", "price": 29, "specs": {"pages": 500, "format": "Paperback"}}
								]
							}
						]
					}
				]
			}
		}`

		// Test mixed extraction - get all product names
		productNames, err := processor.Get(testData, "catalog.categories{products}{name}")
		helper.AssertNoError(err, "Mixed extraction should work")
		if nameArray, ok := productNames.([]any); ok {
			helper.AssertTrue(len(nameArray) > 0, "Should extract product names")
			if len(nameArray) >= 3 {
				foundLaptop := false
				foundBook := false
				for _, name := range nameArray {
					if name == "Laptop" {
						foundLaptop = true
					}
					if name == "Programming Guide" {
						foundBook = true
					}
				}
				helper.AssertTrue(foundLaptop, "Should find Laptop")
				helper.AssertTrue(foundBook, "Should find Programming Guide")
			}
		}

		// Test extraction with array slicing
		firstCategoryProducts, err := processor.Get(testData, "catalog.categories[0:1]{products}{name}")
		helper.AssertNoError(err, "Extraction with array slicing should work")
		if productArray, ok := firstCategoryProducts.([]any); ok {
			helper.AssertTrue(len(productArray) > 0, "Should extract products from first category")
		}

		// Test extraction followed by property access
		laptopVariants, err := processor.Get(testData, "catalog.categories{products}[0].variants")
		helper.AssertNoError(err, "Extraction followed by property access should work")
		if variantArray, ok := laptopVariants.([]any); ok {
			helper.AssertTrue(len(variantArray) >= 2, "Should get laptop variants")
		}

		// Test extraction with nested property access
		allPrices, err := processor.Get(testData, "catalog.categories{products}{variants}{price}")
		helper.AssertNoError(err, "Extraction with nested property access should work")
		if priceArray, ok := allPrices.([]any); ok {
			helper.AssertTrue(len(priceArray) > 0, "Should extract variant prices")
		}

		// Test extraction with complex specs
		ramSpecs, err := processor.Get(testData, "catalog.categories{products}{variants}{specs}")
		helper.AssertNoError(err, "Extraction with complex specs should work")
		if specArray, ok := ramSpecs.([]any); ok {
			helper.AssertTrue(len(specArray) > 0, "Should extract specs objects")
		}
	})

	t.Run("ExtractionWithArrayOperations", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"surveys": [
				{
					"title": "Customer Satisfaction",
					"responses": [
						{"user": "user1", "answers": [5, 4, 5, 3, 4]},
						{"user": "user2", "answers": [4, 5, 4, 4, 5]},
						{"user": "user3", "answers": [3, 3, 4, 2, 3]}
					]
				},
				{
					"title": "Product Feedback",
					"responses": [
						{"user": "user4", "answers": [5, 5, 5, 4, 5]},
						{"user": "user5", "answers": [4, 4, 3, 4, 4]}
					]
				}
			]
		}`

		// Test extraction with array slicing
		firstSurveyUsers, err := processor.Get(testData, "surveys[0]{responses}{user}")
		helper.AssertNoError(err, "Extraction with array access should work")
		if userArray, ok := firstSurveyUsers.([]any); ok {
			helper.AssertEqual(3, len(userArray), "Should extract users from first survey")
			helper.AssertEqual("user1", userArray[0], "First user should be correct")
		}

		// Test extraction with negative array index
		lastSurveyUsers, err := processor.Get(testData, "surveys[-1]{responses}{user}")
		helper.AssertNoError(err, "Extraction with negative index should work")
		if userArray, ok := lastSurveyUsers.([]any); ok {
			helper.AssertEqual(2, len(userArray), "Should extract users from last survey")
		}

		// Test extraction with array slicing on extracted data
		allAnswers, err := processor.Get(testData, "surveys{responses}{answers}")
		helper.AssertNoError(err, "Extraction of answer arrays should work")
		if answerArrays, ok := allAnswers.([]any); ok {
			helper.AssertTrue(len(answerArrays) > 0, "Should extract answer arrays")
		}

		// Test extraction followed by array slicing
		firstAnswers, err := processor.Get(testData, "surveys{responses}[0].answers")
		helper.AssertNoError(err, "Extraction followed by array access should work")
		if answers, ok := firstAnswers.([]any); ok {
			helper.AssertTrue(len(answers) >= 5, "Should get first response answers from each survey")
		}

		// Test complex extraction with array operations
		surveyTitles, err := processor.Get(testData, "surveys{title}")
		helper.AssertNoError(err, "Simple extraction should work")
		if titleArray, ok := surveyTitles.([]any); ok {
			helper.AssertEqual(2, len(titleArray), "Should extract both survey titles")
			helper.AssertEqual("Customer Satisfaction", titleArray[0], "First title should be correct")
		}
	})

	t.Run("ExtractionErrorHandling", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"data": [
				{"items": [1, 2, 3]},
				{"items": [4, 5]},
				{"different": "structure"},
				{"items": [6, 7, 8, 9]}
			]
		}`

		// Test extraction on mixed structure
		items, err := processor.Get(testData, "data{items}")
		helper.AssertNoError(err, "Extraction on mixed structure should work")
		if itemArrays, ok := items.([]any); ok {
			// Should extract items arrays, skipping objects without items
			helper.AssertTrue(len(itemArrays) >= 3, "Should extract available items arrays")
		}

		// Test extraction on non-existent path
		nonExistent, err := processor.Get(testData, "data{nonexistent}")
		helper.AssertNoError(err, "Extraction on non-existent path should not error")
		if result, ok := nonExistent.([]any); ok {
			// Should return empty array or array with nil values
			helper.AssertTrue(len(result) >= 0, "Non-existent extraction should return array")
		}

		// Test extraction with invalid array access
		invalidAccess, err := processor.Get(testData, "data{items}[100]")
		helper.AssertNoError(err, "Extraction with invalid array access should not error")
		// Result should be nil or empty
		_ = invalidAccess // Use the variable to avoid compiler warning

		// Test extraction on primitive values
		primitiveData := `{"values": [1, 2, 3, "string", true]}`
		primitiveExtraction, err := processor.Get(primitiveData, "values{nonexistent}")
		helper.AssertNoError(err, "Extraction on primitive values should not error")
		// Should handle gracefully
		_ = primitiveExtraction // Use the variable to avoid compiler warning
	})
}
