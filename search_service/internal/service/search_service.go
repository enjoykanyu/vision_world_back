package service

import (
	"context"
	"search_service/internal/model"
	"search_service/internal/repository"
	"search_service/pkg/logger"
)

// SearchService 搜索服务接口
type SearchService interface {
	// Search 执行搜索
	Search(ctx context.Context, req model.SearchRequest) (*model.SearchResponse, error)

	// IndexDocument 索引文档
	IndexDocument(ctx context.Context, doc model.SearchModel) error

	// DeleteDocument 删除文档
	DeleteDocument(ctx context.Context, id string, docType string) error

	// GetSearchSuggestions 获取搜索建议
	GetSearchSuggestions(ctx context.Context, prefix string, limit int) ([]string, error)
}

// searchService 搜索服务实现
type searchService struct {
	repo   repository.SearchRepository
	logger logger.Logger
}

// NewSearchService 创建搜索服务实例
func NewSearchService(repo repository.SearchRepository, logger logger.Logger) SearchService {
	return &searchService{
		repo:   repo,
		logger: logger,
	}
}

// Search 执行搜索
func (s *searchService) Search(ctx context.Context, req model.SearchRequest) (*model.SearchResponse, error) {
	// 记录搜索日志
	s.logger.Info("Executing search", "query", req.Query, "page", req.Page, "size", req.Size)

	// 执行搜索
	result, err := s.repo.SearchDocuments(ctx, req)
	if err != nil {
		s.logger.Error("Failed to search documents", "error", err)
		return nil, err
	}

	s.logger.Info("Search completed", "total_results", result.Total)
	return result, nil
}

// IndexDocument 索引文档
func (s *searchService) IndexDocument(ctx context.Context, doc model.SearchModel) error {
	s.logger.Info("Indexing document")

	err := s.repo.IndexDocument(ctx, doc)
	if err != nil {
		s.logger.Error("Failed to index document", "error", err)
		return err
	}

	s.logger.Info("Document indexed successfully")
	return nil
}

// DeleteDocument 删除文档
func (s *searchService) DeleteDocument(ctx context.Context, id string, docType string) error {
	s.logger.Info("Deleting document", "id", id, "type", docType)

	err := s.repo.DeleteDocument(ctx, id, docType)
	if err != nil {
		s.logger.Error("Failed to delete document", "error", err)
		return err
	}

	s.logger.Info("Document deleted successfully")
	return nil
}

// GetSearchSuggestions 获取搜索建议
func (s *searchService) GetSearchSuggestions(ctx context.Context, prefix string, limit int) ([]string, error) {
	s.logger.Info("Getting search suggestions", "prefix", prefix, "limit", limit)

	suggestions, err := s.repo.GetSearchSuggestions(ctx, prefix, limit)
	if err != nil {
		s.logger.Error("Failed to get search suggestions", "error", err)
		return nil, err
	}

	s.logger.Info("Search suggestions retrieved", "count", len(suggestions))
	return suggestions, nil
}
