package main

import (
	"fmt"
	"search_service/internal/handler"
	"search_service/internal/model"
)

// SimpleLogger 简单的日志记录器实现
type SimpleLogger struct{}

func (l *SimpleLogger) Debug(msg string, fields ...interface{}) {
	fmt.Printf("[DEBUG] %s %v\n", msg, fields)
}

func (l *SimpleLogger) Info(msg string, fields ...interface{}) {
	fmt.Printf("[INFO] %s %v\n", msg, fields)
}

func (l *SimpleLogger) Warn(msg string, fields ...interface{}) {
	fmt.Printf("[WARN] %s %v\n", msg, fields)
}

func (l *SimpleLogger) Error(msg string, fields ...interface{}) {
	fmt.Printf("[ERROR] %s %v\n", msg, fields)
}

func (l *SimpleLogger) Fatal(msg string, fields ...interface{}) {
	fmt.Printf("[FATAL] %s %v\n", msg, fields)
}

func (l *SimpleLogger) Sync() error {
	return nil
}

func main() {
	fmt.Println("=== Search Service Test ===")

	// 创建简单的logger
	logger := &SimpleLogger{}

	// 创建handler
	searchHandler := handler.NewSearchServiceHandler(nil, logger, nil, nil)

	// 测试搜索功能
	req := &model.SearchRequest{
		Query:       "测试",
		Page:        1,
		Size:        10,
		SearchType:  "video",
		Filter:      make(map[string]string),
		SortBy:      "relevance",
		SortOrder:   "desc",
		FuzzySearch: true,
	}

	resp, err := searchHandler.Search(nil, req)
	if err != nil {
		fmt.Printf("Search failed: %v\n", err)
		return
	}

	fmt.Printf("\nSearch Results:\n")
	fmt.Printf("Total: %d\n", resp.Total)
	fmt.Printf("Page: %d\n", resp.Page)
	fmt.Printf("Size: %d\n", resp.Size)
	fmt.Printf("Elapsed Time: %d ms\n", resp.ElapsedTime)
	fmt.Printf("Results:\n")
	for i, result := range resp.Results {
		fmt.Printf("  %d. ID: %s, Score: %.2f, Type: %s\n", i+1, result.ID, result.Score, result.Type)
		fmt.Printf("     Source: %v\n", result.Source)
	}

	// 测试搜索建议功能
	suggestions, err := searchHandler.GetSearchSuggestions(nil, "测试", 5)
	if err != nil {
		fmt.Printf("GetSearchSuggestions failed: %v\n", err)
		return
	}

	fmt.Printf("\nSearch Suggestions:\n")
	for i, suggestion := range suggestions {
		fmt.Printf("  %d. %s\n", i+1, suggestion)
	}

	fmt.Println("\n=== Test Completed ===")
}
