package json

import (
	"fmt"
	"testing"
)

// TestForeachWithPathDataModification tests whether ForeachWithPath supports data modification
func TestForeachWithPathDataModification(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("ForeachWithPathModificationTest", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"company": {
				"departments": [
					{"name": "Engineering", "budget": 100000, "active": false},
					{"name": "Marketing", "budget": 50000, "active": false},
					{"name": "Sales", "budget": 75000, "active": true}
				]
			},
			"users": [
				{"id": 1, "name": "Alice", "status": "inactive", "age": 25},
				{"id": 2, "name": "Bob", "status": "active", "age": 30},
				{"id": 3, "name": "Charlie", "status": "inactive", "age": 17}
			]
		}`

		fmt.Println("=== 测试 ForeachWithPath 数据修改能力 ===")
		fmt.Println("原始数据:")
		fmt.Println(testData)

		// 测试1: 尝试在 ForeachWithPath 中修改数据
		fmt.Println("\n测试1: 在 ForeachWithPath 中修改部门数据")

		// 记录原始数据以便对比
		originalData := testData

		err := ForeachWithPath(testData, "company.departments", func(key any, dept *IterableValue) {
			deptName := dept.GetString("name")
			fmt.Printf("处理部门: %s\n", deptName)

			// 尝试修改数据
			dept.Set("active", true)
			dept.Set("updated", true)
			dept.Set("processed_at", "2024-01-01")

			// 尝试修改预算
			currentBudget := dept.GetInt("budget")
			dept.Set("budget", currentBudget+10000)
		})

		helper.AssertNoError(err, "ForeachWithPath should execute without error")

		// 检查原始数据是否被修改
		fmt.Println("\n检查原始数据是否被修改...")

		// 获取修改后的部门数据
		departments, err := processor.Get(testData, "company.departments")
		helper.AssertNoError(err, "Should be able to get departments")

		if deptArray, ok := departments.([]any); ok {
			fmt.Printf("部门数组长度: %d\n", len(deptArray))

			// 检查第一个部门是否被修改
			if len(deptArray) > 0 {
				if firstDept, ok := deptArray[0].(map[string]any); ok {
					fmt.Printf("第一个部门 active 状态: %v\n", firstDept["active"])
					fmt.Printf("第一个部门是否有 updated 字段: %v\n", firstDept["updated"] != nil)
					fmt.Printf("第一个部门预算: %v\n", firstDept["budget"])
				}
			}
		}

		// 测试2: 尝试在 ForeachWithPath 中修改用户数据
		fmt.Println("\n测试2: 在 ForeachWithPath 中修改用户数据")

		err = ForeachWithPath(testData, "users", func(key any, user *IterableValue) {
			userName := user.GetString("name")
			userAge := user.GetInt("age")
			fmt.Printf("处理用户: %s (年龄: %d)\n", userName, userAge)

			// 尝试修改用户状态
			if user.GetString("status") == "inactive" {
				user.Set("status", "active")
				user.Set("activated_by", "system")
			}

			// 为未成年人添加标记
			if userAge < 18 {
				user.Set("category", "minor")
				user.Set("requires_guardian", true)
			}
		})

		helper.AssertNoError(err, "ForeachWithPath should execute without error")

		// 检查用户数据是否被修改
		fmt.Println("\n检查用户数据是否被修改...")

		users, err := processor.Get(testData, "users")
		helper.AssertNoError(err, "Should be able to get users")

		if userArray, ok := users.([]any); ok {
			fmt.Printf("用户数组长度: %d\n", len(userArray))

			// 检查每个用户的修改情况
			for i, userData := range userArray {
				if userMap, ok := userData.(map[string]any); ok {
					fmt.Printf("用户 %d - 姓名: %v, 状态: %v, 分类: %v\n",
						i, userMap["name"], userMap["status"], userMap["category"])
				}
			}
		}

		// 测试3: 对比原始数据
		fmt.Println("\n测试3: 对比原始数据是否发生变化")
		fmt.Printf("原始数据长度: %d\n", len(originalData))
		fmt.Printf("当前数据长度: %d\n", len(testData))

		// 检查数据是否真的被修改了
		isDataModified := originalData != testData
		fmt.Printf("数据是否被修改: %v\n", isDataModified)

		// 结论
		fmt.Println("\n=== 测试结论 ===")
		if isDataModified {
			fmt.Println("✅ ForeachWithPath 支持数据修改")
		} else {
			fmt.Println("❌ ForeachWithPath 不支持数据修改 (只读遍历)")
		}
	})

	t.Run("CompareWithForeachReturn", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"items": [
				{"value": 1, "processed": false},
				{"value": 2, "processed": false},
				{"value": 3, "processed": false}
			]
		}`

		fmt.Println("\n=== 对比 ForeachWithPath 和 ForeachReturn ===")

		// 使用 ForeachReturn 修改数据
		fmt.Println("使用 ForeachReturn 修改数据:")
		modifiedJson, err := ForeachReturn(testData, func(key any, item *IterableValue) {
			if key == "items" {
				items := item.GetArray("")
				for i := range items {
					item.Set(fmt.Sprintf("[%d].processed", i), true)
					item.Set(fmt.Sprintf("[%d].modified_by", i), "ForeachReturn")
				}
			}
		})

		helper.AssertNoError(err, "ForeachReturn should work")
		fmt.Println("ForeachReturn 结果:")
		fmt.Println(modifiedJson)

		// 使用 ForeachWithPath 尝试修改数据
		fmt.Println("\n使用 ForeachWithPath 尝试修改数据:")
		originalTestData := testData

		err = ForeachWithPath(testData, "items", func(key any, item *IterableValue) {
			item.Set("processed", true)
			item.Set("modified_by", "ForeachWithPath")
		})

		helper.AssertNoError(err, "ForeachWithPath should execute")

		fmt.Printf("原始数据是否改变: %v\n", originalTestData != testData)
		fmt.Println("ForeachWithPath 后的数据:")
		fmt.Println(testData)

		// 结论
		fmt.Println("\n=== 对比结论 ===")
		fmt.Println("ForeachReturn: 返回修改后的JSON字符串 ✅")
		fmt.Println("ForeachWithPath: 只读遍历，不修改原始数据 ❌")
	})
}
