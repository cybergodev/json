package json

import (
	"testing"
	"sync"
)

// TestAdvancedForeachOperations tests comprehensive Foreach operations with complex scenarios
func TestAdvancedForeachOperations(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("DeepNestedIteration", func(t *testing.T) {
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
								"members": [
									{"name": "Alice", "level": "Senior", "salary": 120000},
									{"name": "Bob", "level": "Mid", "salary": 90000}
								]
							},
							{
								"name": "Frontend",
								"members": [
									{"name": "Charlie", "level": "Senior", "salary": 115000},
									{"name": "Diana", "level": "Junior", "salary": 70000}
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
									{"name": "Eve", "level": "Mid", "salary": 85000}
								]
							}
						]
					}
				]
			}
		}`

		// Test iterating through nested structure
		var totalSalary float64
		var memberCount int
		var seniorCount int

		err := processor.Foreach(testData, func(key any, value *IterableValue) {
			if key == "organization" {
				// Iterate through departments
				departments := value.Get("departments")
				if deptArray, ok := departments.([]any); ok {
					for _, dept := range deptArray {
						if deptMap, ok := dept.(map[string]any); ok {
							if teams, ok := deptMap["teams"].([]any); ok {
								for _, team := range teams {
									if teamMap, ok := team.(map[string]any); ok {
										if members, ok := teamMap["members"].([]any); ok {
											for _, member := range members {
												if memberMap, ok := member.(map[string]any); ok {
													memberCount++
													if salary, ok := memberMap["salary"].(float64); ok {
														totalSalary += salary
													}
													if level, ok := memberMap["level"].(string); ok && level == "Senior" {
														seniorCount++
													}
												}
											}
										}
									}
								}
							}
						}
					}
				}
			}
		})

		helper.AssertNoError(err, "Deep nested iteration should work")
		helper.AssertEqual(5, memberCount, "Should count all members")
		helper.AssertEqual(float64(480000), totalSalary, "Should sum all salaries")
		helper.AssertEqual(2, seniorCount, "Should count senior members")
	})

	t.Run("ArrayIterationWithComplexPaths", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"projects": [
				{
					"name": "Project Alpha",
					"tasks": [
						{"id": 1, "status": "completed", "priority": "high"},
						{"id": 2, "status": "in-progress", "priority": "medium"},
						{"id": 3, "status": "pending", "priority": "low"}
					]
				},
				{
					"name": "Project Beta",
					"tasks": [
						{"id": 4, "status": "completed", "priority": "high"},
						{"id": 5, "status": "completed", "priority": "medium"}
					]
				}
			]
		}`

		// Test iterating through array with complex processing
		var completedTasks int
		var highPriorityTasks int
		var tasksByProject = make(map[string]int)

		err := processor.Foreach(testData, func(key any, value *IterableValue) {
			if key == "projects" {
				projects := value.Get("")
				if projectArray, ok := projects.([]any); ok {
					for _, project := range projectArray {
						if projectMap, ok := project.(map[string]any); ok {
							projectName := projectMap["name"].(string)
							if tasks, ok := projectMap["tasks"].([]any); ok {
								tasksByProject[projectName] = len(tasks)
								for _, task := range tasks {
									if taskMap, ok := task.(map[string]any); ok {
										if status, ok := taskMap["status"].(string); ok && status == "completed" {
											completedTasks++
										}
										if priority, ok := taskMap["priority"].(string); ok && priority == "high" {
											highPriorityTasks++
										}
									}
								}
							}
						}
					}
				}
			}
		})

		helper.AssertNoError(err, "Array iteration should work")
		helper.AssertEqual(3, completedTasks, "Should count completed tasks")
		helper.AssertEqual(2, highPriorityTasks, "Should count high priority tasks")
		helper.AssertEqual(3, tasksByProject["Project Alpha"], "Project Alpha should have 3 tasks")
		helper.AssertEqual(2, tasksByProject["Project Beta"], "Project Beta should have 2 tasks")
	})

	t.Run("ConditionalIterationAndFiltering", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"inventory": {
				"electronics": [
					{"name": "Laptop", "price": 1200, "stock": 15, "category": "computers"},
					{"name": "Mouse", "price": 25, "stock": 100, "category": "accessories"},
					{"name": "Monitor", "price": 300, "stock": 8, "category": "displays"}
				],
				"books": [
					{"name": "Go Programming", "price": 45, "stock": 20, "category": "technical"},
					{"name": "Design Patterns", "price": 55, "stock": 12, "category": "technical"},
					{"name": "Fiction Novel", "price": 15, "stock": 50, "category": "fiction"}
				]
			}
		}`

		// Test conditional iteration with filtering
		var expensiveItems []string
		var lowStockItems []string
		var totalValue float64

		err := processor.Foreach(testData, func(key any, value *IterableValue) {
			if key == "inventory" {
				inventory := value.Get("")
				if inventoryMap, ok := inventory.(map[string]any); ok {
					for _, items := range inventoryMap {
						if itemArray, ok := items.([]any); ok {
							for _, item := range itemArray {
								if itemMap, ok := item.(map[string]any); ok {
									name := itemMap["name"].(string)
									price := itemMap["price"].(float64)
									stock := itemMap["stock"].(float64)

									// Filter expensive items (price > 50)
									if price > 50 {
										expensiveItems = append(expensiveItems, name)
									}

									// Filter low stock items (stock < 15)
									if stock < 15 {
										lowStockItems = append(lowStockItems, name)
									}

									// Calculate total inventory value
									totalValue += price * stock
								}
							}
						}
					}
				}
			}
		})

		helper.AssertNoError(err, "Conditional iteration should work")
		helper.AssertEqual(3, len(expensiveItems), "Should find 3 expensive items")
		helper.AssertEqual(2, len(lowStockItems), "Should find 2 low stock items")
		helper.AssertTrue(totalValue > 20000, "Total value should be substantial")
	})

	t.Run("NestedObjectIteration", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"users": {
				"user1": {
					"name": "Alice",
					"preferences": {
						"theme": "dark",
						"language": "en",
						"notifications": {
							"email": true,
							"push": false,
							"sms": true
						}
					},
					"activity": {
						"lastLogin": "2023-12-01",
						"loginCount": 45
					}
				},
				"user2": {
					"name": "Bob",
					"preferences": {
						"theme": "light",
						"language": "es",
						"notifications": {
							"email": false,
							"push": true,
							"sms": false
						}
					},
					"activity": {
						"lastLogin": "2023-11-28",
						"loginCount": 23
					}
				}
			}
		}`

		// Test nested object iteration
		var userCount int
		var totalLogins int
		var darkThemeUsers int
		var emailNotificationUsers int

		err := processor.Foreach(testData, func(key any, value *IterableValue) {
			if key == "users" {
				users := value.Get("")
				if usersMap, ok := users.(map[string]any); ok {
					for _, userData := range usersMap {
						if user, ok := userData.(map[string]any); ok {
							userCount++

							// Check activity
							if activity, ok := user["activity"].(map[string]any); ok {
								if loginCount, ok := activity["loginCount"].(float64); ok {
									totalLogins += int(loginCount)
								}
							}

							// Check preferences
							if prefs, ok := user["preferences"].(map[string]any); ok {
								if theme, ok := prefs["theme"].(string); ok && theme == "dark" {
									darkThemeUsers++
								}

								if notifications, ok := prefs["notifications"].(map[string]any); ok {
									if email, ok := notifications["email"].(bool); ok && email {
										emailNotificationUsers++
									}
								}
							}
						}
					}
				}
			}
		})

		helper.AssertNoError(err, "Nested object iteration should work")
		helper.AssertEqual(2, userCount, "Should count all users")
		helper.AssertEqual(68, totalLogins, "Should sum all login counts")
		helper.AssertEqual(1, darkThemeUsers, "Should count dark theme users")
		helper.AssertEqual(1, emailNotificationUsers, "Should count email notification users")
	})

	t.Run("ErrorHandlingInIteration", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"data": [
				{"id": 1, "value": "valid"},
				{"id": 2, "value": "error"},
				{"id": 3, "value": "valid"}
			]
		}`

		// Test error handling during iteration
		var processedCount int
		err := processor.Foreach(testData, func(key any, value *IterableValue) {
			if key == "data" {
				data := value.Get("")
				if dataArray, ok := data.([]any); ok {
					for _, item := range dataArray {
						if itemMap, ok := item.(map[string]any); ok {
							if val, ok := itemMap["value"].(string); ok && val == "error" {
								// We can't return error from this callback, so we'll just skip
								continue
							}
							processedCount++
						}
					}
				}
			}
		})

		helper.AssertNoError(err, "Should complete iteration without errors")
		helper.AssertEqual(2, processedCount, "Should process items except error item")

		// Test continuing iteration after handling errors
		processedCount = 0
		err = processor.Foreach(testData, func(key any, value *IterableValue) {
			if key == "data" {
				data := value.Get("")
				if dataArray, ok := data.([]any); ok {
					processedCount = len(dataArray)
				}
			}
		})

		helper.AssertNoError(err, "Should complete iteration without errors")
		helper.AssertEqual(3, processedCount, "Should count all items")
	})

	t.Run("ConcurrentIterationSafety", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"numbers": [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]
		}`

		// Test concurrent access during iteration
		var wg sync.WaitGroup
		var mu sync.Mutex
		var results []int

		wg.Add(2)

		// First goroutine: iterate and collect
		go func() {
			defer wg.Done()
			processor.Foreach(testData, func(key any, value *IterableValue) {
				if key == "numbers" {
					numbers := value.Get("")
					if numArray, ok := numbers.([]any); ok {
						for _, num := range numArray {
							if numVal, ok := num.(float64); ok {
								mu.Lock()
								results = append(results, int(numVal))
								mu.Unlock()
							}
						}
					}
				}
			})
		}()

		// Second goroutine: also iterate
		go func() {
			defer wg.Done()
			processor.Foreach(testData, func(key any, value *IterableValue) {
				// Just iterate, don't modify shared state
			})
		}()

		wg.Wait()

		mu.Lock()
		resultCount := len(results)
		mu.Unlock()

		helper.AssertEqual(10, resultCount, "Should collect all numbers safely")
	})
}
