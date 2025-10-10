package main

import (
	"context"
	"fmt"
	"search_service/internal/config"
	"search_service/internal/handler"
	"search_service/internal/model"
	"search_service/pkg/logger"
)

func main() {
	// 加载配置
	cfg, err := config.LoadConfig("../../config/search-service.yaml")
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		return
	}

	// 初始化日志
	logger, err := logger.NewLogger(logger.Config{
		Level:      cfg.Logger.Level,
		Format:     cfg.Logger.Format,
		OutputPath: cfg.Logger.OutputPath,
	})
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		return
	}

	// 创建handler
	searchHandler := handler.NewSearchServiceHandler(cfg, logger, nil, nil)

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

	resp, err := searchHandler.Search(context.Background(), req)
	if err != nil {
		fmt.Printf("Search failed: %v\n", err)
		return
	}

	fmt.Printf("Search Results:\n")
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
	suggestions, err := searchHandler.GetSearchSuggestions(context.Background(), "测试", 5)
	if err != nil {
		fmt.Printf("GetSearchSuggestions failed: %v\n", err)
		return
	}

	fmt.Printf("\nSearch Suggestions:\n")
	for i, suggestion := range suggestions {
		fmt.Printf("  %d. %s\n", i+1, suggestion)
	}
}
