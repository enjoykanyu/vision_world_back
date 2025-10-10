package model

// SearchModel 搜索模型基础接口
type SearchModel interface {
	// Index 索引文档
	Index() error

	// Search 搜索文档
	Search(query string) ([]interface{}, error)

	// Delete 删除索引
	Delete() error
}

// SearchResult 搜索结果
type SearchResult struct {
	ID     string                 `json:"id"`
	Score  float64                `json:"score"`
	Source map[string]interface{} `json:"source"`
	Type   string                 `json:"type"`
}

// SearchRequest 搜索请求
type SearchRequest struct {
	Query       string            `json:"query"`
	Page        int               `json:"page"`
	Size        int               `json:"size"`
	SearchType  string            `json:"search_type"`
	Filter      map[string]string `json:"filter"`
	SortBy      string            `json:"sort_by"`
	SortOrder   string            `json:"sort_order"`
	FuzzySearch bool              `json:"fuzzy_search"`
}

// SearchResponse 搜索响应
type SearchResponse struct {
	Results     []SearchResult `json:"results"`
	Total       int64          `json:"total"`
	Page        int            `json:"page"`
	Size        int            `json:"size"`
	ElapsedTime int64          `json:"elapsed_time"` // 毫秒
}

// SuggestionRequest 搜索建议请求
type SuggestionRequest struct {
	Prefix string `json:"prefix"`
	Limit  int    `json:"limit"`
}

// SuggestionResponse 搜索建议响应
type SuggestionResponse struct {
	Suggestions []string `json:"suggestions"`
}
