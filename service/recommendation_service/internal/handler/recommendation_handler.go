package handler

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"recommendation_service/internal/config"
	"recommendation_service/internal/converter"
	"recommendation_service/internal/repository"
	"recommendation_service/internal/service"
	"recommendation_service/pkg/logger"
	"recommendation_service/proto/proto_gen"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

// RecommendationServiceHandler 推荐服务处理器
type RecommendationServiceHandler struct {
	proto_gen.UnimplementedRecommendationServiceServer
	config                     *config.Config
	logger                     logger.Logger
	videoRecommendationService service.VideoRecommendationService
	converter                  *converter.RecommendationConverter
}

// NewRecommendationServiceHandler 创建推荐服务处理器
func NewRecommendationServiceHandler(cfg *config.Config, log logger.Logger, db *gorm.DB, redis *redis.Client) *RecommendationServiceHandler {
	// 创建推荐仓库
	recommendationRepo := repository.NewRecommendationRepository(db, redis)

	// 创建视频推荐服务
	videoRecommendationService := service.NewVideoRecommendationService(cfg, log, recommendationRepo)

	return &RecommendationServiceHandler{
		config:                     cfg,
		logger:                     log,
		videoRecommendationService: videoRecommendationService,
		converter:                  converter.NewRecommendationConverter(),
	}
}

// GetPersonalizedRecommendations 获取个性化推荐
func (h *RecommendationServiceHandler) GetPersonalizedRecommendations(ctx context.Context, req *proto_gen.GetPersonalizedRecommendationsRequest) (*proto_gen.GetPersonalizedRecommendationsResponse, error) {
	h.logger.Info("GetPersonalizedRecommendations called", "user_id", req.UserId, "page", req.Page, "page_size", req.PageSize)

	// 参数校验
	if req.UserId == "" {
		return &proto_gen.GetPersonalizedRecommendationsResponse{
			StatusCode: 400,
			StatusMsg:  "用户ID不能为空",
			Videos:     nil,
			HasMore:    false,
		}, nil
	}

	if req.Page <= 0 {
		req.Page = 1
	}

	if req.PageSize <= 0 || req.PageSize > 100 {
		req.PageSize = 20
	}

	// 调用推荐服务获取个性化推荐
	videos, hasMore, err := h.videoRecommendationService.GetPersonalizedRecommendations(ctx, req.UserId, req.Page, req.PageSize, req.Category, req.UserTags)
	if err != nil {
		h.logger.Error("GetPersonalizedRecommendations failed", "error", err, "user_id", req.UserId)
		return &proto_gen.GetPersonalizedRecommendationsResponse{
			StatusCode: 500,
			StatusMsg:  err.Error(),
			Videos:     nil,
			HasMore:    false,
		}, nil
	}

	// 转换为proto格式
	protoVideos := make([]*proto_gen.Video, 0, len(videos))
	for _, video := range videos {
		protoVideos = append(protoVideos, h.converter.ModelToProto(video))
	}

	return &proto_gen.GetPersonalizedRecommendationsResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		Videos:     protoVideos,
		HasMore:    hasMore,
	}, nil
}

// GetGeneralRecommendations 获取通用推荐
func (h *RecommendationServiceHandler) GetGeneralRecommendations(ctx context.Context, req *proto_gen.GetGeneralRecommendationsRequest) (*proto_gen.GetGeneralRecommendationsResponse, error) {
	h.logger.Info("GetGeneralRecommendations called", "page", req.Page, "page_size", req.PageSize, "category", req.Category)

	// 参数校验
	if req.Page <= 0 {
		req.Page = 1
	}

	if req.PageSize <= 0 || req.PageSize > 100 {
		req.PageSize = 20
	}

	// 调用推荐服务获取通用推荐
	videos, hasMore, err := h.videoRecommendationService.GetGeneralRecommendations(ctx, req.Page, req.PageSize, req.Category)
	if err != nil {
		h.logger.Error("GetGeneralRecommendations failed", "error", err, "category", req.Category)
		return &proto_gen.GetGeneralRecommendationsResponse{
			StatusCode: 500,
			StatusMsg:  err.Error(),
			Videos:     nil,
			HasMore:    false,
		}, nil
	}

	// 转换为proto格式
	protoVideos := make([]*proto_gen.Video, 0, len(videos))
	for _, video := range videos {
		protoVideos = append(protoVideos, h.converter.ModelToProto(video))
	}

	return &proto_gen.GetGeneralRecommendationsResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		Videos:     protoVideos,
		HasMore:    hasMore,
	}, nil
}

// UpdateUserPreferences 更新用户偏好
func (h *RecommendationServiceHandler) UpdateUserPreferences(ctx context.Context, req *proto_gen.UpdateUserPreferencesRequest) (*proto_gen.UpdateUserPreferencesResponse, error) {
	h.logger.Info("UpdateUserPreferences called", "user_id", req.UserId)

	// 参数校验
	if req.UserId == "" {
		return &proto_gen.UpdateUserPreferencesResponse{
			StatusCode: 400,
			StatusMsg:  "用户ID不能为空",
		}, nil
	}

	// 调用推荐服务更新用户偏好
	err := h.videoRecommendationService.UpdateUserPreferences(ctx, req.UserId, req.Categories, req.Tags, req.CategoryWeights, req.TagWeights)
	if err != nil {
		h.logger.Error("UpdateUserPreferences failed", "error", err, "user_id", req.UserId)
		return &proto_gen.UpdateUserPreferencesResponse{
			StatusCode: 500,
			StatusMsg:  err.Error(),
		}, nil
	}

	return &proto_gen.UpdateUserPreferencesResponse{
		StatusCode: 0,
		StatusMsg:  "用户偏好更新成功",
	}, nil
}

// RecordUserAction 记录用户行为
func (h *RecommendationServiceHandler) RecordUserAction(ctx context.Context, req *proto_gen.RecordUserActionRequest) (*proto_gen.RecordUserActionResponse, error) {
	h.logger.Info("RecordUserAction called", "user_id", req.UserId, "video_id", req.VideoId, "action_type", req.ActionType)

	// 参数校验
	if req.UserId == "" || req.VideoId == "" || req.ActionType == "" {
		return &proto_gen.RecordUserActionResponse{
			StatusCode: 400,
			StatusMsg:  "参数不能为空",
		}, nil
	}

	// 验证行为类型
	validActionTypes := []string{"view", "like", "share", "comment", "complete"}
	isValidActionType := false
	for _, validType := range validActionTypes {
		if req.ActionType == validType {
			isValidActionType = true
			break
		}
	}
	if !isValidActionType {
		return &proto_gen.RecordUserActionResponse{
			StatusCode: 400,
			StatusMsg:  "无效的行为类型",
		}, nil
	}

	// 如果时间戳为空，使用当前时间
	if req.Timestamp == 0 {
		req.Timestamp = time.Now().Unix()
	}

	// 调用推荐服务记录用户行为
	err := h.videoRecommendationService.RecordUserAction(ctx, req.UserId, req.VideoId, req.ActionType, req.Duration, req.TotalDuration, req.Timestamp)
	if err != nil {
		h.logger.Error("RecordUserAction failed", "error", err, "user_id", req.UserId, "video_id", req.VideoId)
		return &proto_gen.RecordUserActionResponse{
			StatusCode: 500,
			StatusMsg:  err.Error(),
		}, nil
	}

	return &proto_gen.RecordUserActionResponse{
		StatusCode: 0,
		StatusMsg:  "用户行为记录成功",
	}, nil
}
