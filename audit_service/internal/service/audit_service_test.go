package service

import (
	"context"
	"github.com/vision_world/audit_service/internal/config"
	"github.com/vision_world/audit_service/internal/model"
	"github.com/vision_world/audit_service/internal/repository"
	"github.com/vision_world/audit_service/pkg/logger"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAuditRepository 模拟审核仓库
type MockAuditRepository struct {
	mock.Mock
}

func (m *MockAuditRepository) CreateAuditRecord(ctx context.Context, record *model.AuditRecord) (uint64, error) {
	args := m.Called(ctx, record)
	return args.Get(0).(uint64), args.Error(1)
}

func (m *MockAuditRepository) GetAuditRecord(ctx context.Context, id uint64) (*model.AuditRecord, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.AuditRecord), args.Error(1)
}

func (m *MockAuditRepository) UpdateAuditRecord(ctx context.Context, record *model.AuditRecord) error {
	args := m.Called(ctx, record)
	return args.Error(0)
}

func (m *MockAuditRepository) GetAuditRecordByContentID(ctx context.Context, contentID string) (*model.AuditRecord, error) {
	args := m.Called(ctx, contentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.AuditRecord), args.Error(1)
}

func (m *MockAuditRepository) ListAuditRecords(ctx context.Context, req *ListAuditRecordsRequest) (*ListAuditRecordsResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ListAuditRecordsResponse), args.Error(1)
}

func (m *MockAuditRepository) CreateTemplate(ctx context.Context, template *model.AuditTemplate) (uint64, error) {
	args := m.Called(ctx, template)
	return args.Get(0).(uint64), args.Error(1)
}

func (m *MockAuditRepository) UpdateTemplate(ctx context.Context, template *model.AuditTemplate) error {
	args := m.Called(ctx, template)
	return args.Error(0)
}

func (m *MockAuditRepository) DeleteTemplate(ctx context.Context, id uint64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockAuditRepository) GetTemplate(ctx context.Context, id uint64) (*model.AuditTemplate, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.AuditTemplate), args.Error(1)
}

func (m *MockAuditRepository) ListTemplates(ctx context.Context, req *ListTemplatesRequest) (*ListTemplatesResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ListTemplatesResponse), args.Error(1)
}

func (m *MockAuditRepository) AddToWhitelist(ctx context.Context, item *model.AuditWhitelist) error {
	args := m.Called(ctx, item)
	return args.Error(0)
}

func (m *MockAuditRepository) RemoveFromWhitelist(ctx context.Context, contentID string) error {
	args := m.Called(ctx, contentID)
	return args.Error(0)
}

func (m *MockAuditRepository) IsWhitelisted(ctx context.Context, contentID string, contentType string) (bool, error) {
	args := m.Called(ctx, contentID, contentType)
	return args.Bool(0), args.Error(1)
}

func (m *MockAuditRepository) AddToBlacklist(ctx context.Context, item *model.AuditBlacklist) error {
	args := m.Called(ctx, item)
	return args.Error(0)
}

func (m *MockAuditRepository) RemoveFromBlacklist(ctx context.Context, contentID string) error {
	args := m.Called(ctx, contentID)
	return args.Error(0)
}

func (m *MockAuditRepository) IsBlacklisted(ctx context.Context, contentID string, contentType string) (bool, error) {
	args := m.Called(ctx, contentID, contentType)
	return args.Bool(0), args.Error(1)
}

func (m *MockAuditRepository) AddToManualReviewQueue(ctx context.Context, auditID uint64) error {
	args := m.Called(ctx, auditID)
	return args.Error(0)
}

func (m *MockAuditRepository) GetManualReviewQueue(ctx context.Context, req *GetManualReviewQueueRequest) (*GetManualReviewQueueResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*GetManualReviewQueueResponse), args.Error(1)
}

func (m *MockAuditRepository) GetAuditStatistics(ctx context.Context, req *GetAuditStatisticsRequest) (*GetAuditStatisticsResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*GetAuditStatisticsResponse), args.Error(1)
}

func (m *MockAuditRepository) GetViolationTrends(ctx context.Context, req *GetViolationTrendsRequest) (*GetViolationTrendsResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*GetViolationTrendsResponse), args.Error(1)
}

// setupTestService 设置测试服务
func setupTestService() (AuditService, *MockAuditRepository, *config.Config, logger.Logger) {
	// 创建配置
	cfg := &config.Config{
		Service: config.ServiceConfig{
			Name:    "audit-service",
			Version: "1.0.0",
		},
		Audit: config.AuditConfig{
			Strategies: config.AuditStrategies{
				Content: config.ContentStrategy{
					AutoBlockThreshold:    0.8,
					ManualReviewThreshold: 0.5,
				},
			},
		},
	}

	// 创建日志器
	log, _ := logger.NewLogger("debug", "json")

	// 创建模拟仓库
	mockRepo := new(MockAuditRepository)

	// 创建服务
	service := NewAuditService(cfg, log, mockRepo)

	return service, mockRepo, cfg, log
}

func TestSubmitContent_Success(t *testing.T) {
	service, mockRepo, _, _ := setupTestService()
	ctx := context.Background()

	// 设置期望
	mockRepo.On("IsWhitelisted", ctx, "test-content-id", "text").Return(false, nil)
	mockRepo.On("IsBlacklisted", ctx, "test-content-id", "text").Return(false, nil)
	mockRepo.On("CreateAuditRecord", ctx, mock.AnythingOfType("*model.AuditRecord")).Return(uint64(1), nil)
	mockRepo.On("AddToManualReviewQueue", ctx, uint64(1)).Return(nil)

	// 调用服务
	req := &SubmitContentRequest{
		ContentID:   "test-content-id",
		ContentType: "text",
		UploaderID:  "user123",
	}
	result, err := service.SubmitContent(ctx, req)

	// 验证结果
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, uint64(1), result.AuditID)

	mockRepo.AssertExpectations(t)
}

func TestSubmitContent_Whitelist(t *testing.T) {
	service, mockRepo, _, _ := setupTestService()
	ctx := context.Background()

	// 设置期望 - 内容在白名单中
	mockRepo.On("IsWhitelisted", ctx, "safe-content-id", "text").Return(true, nil)

	// 调用服务
	req := &SubmitContentRequest{
		ContentID:   "safe-content-id",
		ContentType: "text",
		UploaderID:  "user123",
	}
	result, err := service.SubmitContent(ctx, req)

	// 验证结果
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, string(model.AuditStatusAutoPassed), result.Status)

	mockRepo.AssertExpectations(t)
}

func TestSubmitContent_Blacklist(t *testing.T) {
	service, mockRepo, _, _ := setupTestService()
	ctx := context.Background()

	// 设置期望 - 内容在黑名单中
	mockRepo.On("IsWhitelisted", ctx, "bad-content-id", "text").Return(false, nil)
	mockRepo.On("IsBlacklisted", ctx, "bad-content-id", "text").Return(true, nil)

	// 调用服务
	req := &SubmitContentRequest{
		ContentID:   "bad-content-id",
		ContentType: "text",
		UploaderID:  "user123",
	}
	result, err := service.SubmitContent(ctx, req)

	// 验证结果
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, string(model.AuditStatusAutoBlocked), result.Status)

	mockRepo.AssertExpectations(t)
}

func TestGetAuditResult_Success(t *testing.T) {
	service, mockRepo, _, _ := setupTestService()
	ctx := context.Background()

	// 创建测试记录
	expectedRecord := &model.AuditRecord{
		ID:          1,
		ContentID:   "test-content-id",
		Status:      model.AuditStatusAutoPassed,
		ContentType: model.ContentTypeText,
		UploaderID:  "user123",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// 设置期望
	mockRepo.On("GetAuditRecordByContentID", ctx, "test-content-id").Return(expectedRecord, nil)

	// 调用服务
	result, err := service.GetAuditResult(ctx, "test-content-id")

	// 验证结果
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, expectedRecord.ID, result.AuditID)
	assert.Equal(t, string(expectedRecord.Status), result.Status)

	mockRepo.AssertExpectations(t)
}

func TestUpdateAuditStatus_Success(t *testing.T) {
	service, mockRepo, _, _ := setupTestService()
	ctx := context.Background()

	// 创建测试记录
	auditRecord := &model.AuditRecord{
		ID:          1,
		ContentID:   "test-content-id",
		Status:      model.AuditStatusPending,
		ContentType: model.ContentTypeText,
		UploaderID:  "user123",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// 设置期望
	mockRepo.On("GetAuditRecord", ctx, uint64(1)).Return(auditRecord, nil)
	mockRepo.On("UpdateAuditRecord", ctx, mock.AnythingOfType("*model.AuditRecord")).Return(nil)

	// 调用服务
	req := &UpdateAuditStatusRequest{
		AuditID:    1,
		Status:     string(model.AuditStatusAutoPassed),
		ReviewerID: 123,
		Reason:     "manual review",
	}
	result, err := service.UpdateAuditStatus(ctx, req)

	// 验证结果
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)

	mockRepo.AssertExpectations(t)
}

func TestBatchSubmitContent_Success(t *testing.T) {
	service, mockRepo, _, _ := setupTestService()
	ctx := context.Background()

	// 设置期望
	mockRepo.On("IsWhitelisted", ctx, "content-1", "text").Return(false, nil)
	mockRepo.On("IsBlacklisted", ctx, "content-1", "text").Return(false, nil)
	mockRepo.On("CreateAuditRecord", ctx, mock.AnythingOfType("*model.AuditRecord")).Return(uint64(1), nil)
	mockRepo.On("AddToManualReviewQueue", ctx, uint64(1)).Return(nil)

	mockRepo.On("IsWhitelisted", ctx, "content-2", "text").Return(false, nil)
	mockRepo.On("IsBlacklisted", ctx, "content-2", "text").Return(false, nil)
	mockRepo.On("CreateAuditRecord", ctx, mock.AnythingOfType("*model.AuditRecord")).Return(uint64(2), nil)
	mockRepo.On("AddToManualReviewQueue", ctx, uint64(2)).Return(nil)

	// 调用服务
	req := &BatchSubmitContentRequest{
		ContentIDs:  []string{"content-1", "content-2"},
		ContentType: "text",
		UploaderID:  "user123",
	}
	result, err := service.BatchSubmitContent(ctx, req)

	// 验证结果
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Results, 2)

	mockRepo.AssertExpectations(t)
}

func TestAssignManualReview_Success(t *testing.T) {
	service, mockRepo, _, _ := setupTestService()
	ctx := context.Background()

	// 创建测试记录
	auditRecord := &model.AuditRecord{
		ID:          1,
		ContentID:   "test-content-id",
		Status:      model.AuditStatusPending,
		ContentType: model.ContentTypeText,
		UploaderID:  "user123",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// 设置期望
	mockRepo.On("GetAuditRecord", ctx, uint64(1)).Return(auditRecord, nil)
	mockRepo.On("UpdateAuditRecord", ctx, mock.AnythingOfType("*model.AuditRecord")).Return(nil)

	// 调用服务
	req := &AssignManualReviewRequest{
		AuditID:    1,
		ReviewerID: 123,
	}
	result, err := service.AssignManualReview(ctx, req)

	// 验证结果
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)

	mockRepo.AssertExpectations(t)
}

func TestCompleteManualReview_Success(t *testing.T) {
	service, mockRepo, _, _ := setupTestService()
	ctx := context.Background()

	// 创建测试记录
	auditRecord := &model.AuditRecord{
		ID:          1,
		ContentID:   "test-content-id",
		Status:      model.AuditStatusPending,
		ContentType: model.ContentTypeText,
		UploaderID:  "user123",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// 设置期望
	mockRepo.On("GetAuditRecord", ctx, uint64(1)).Return(auditRecord, nil)
	mockRepo.On("UpdateAuditRecord", ctx, mock.AnythingOfType("*model.AuditRecord")).Return(nil)

	// 调用服务
	req := &CompleteManualReviewRequest{
		AuditID:    1,
		Status:     string(model.AuditStatusAutoPassed),
		ReviewerID: 123,
		Reason:     "manual review",
	}
	result, err := service.CompleteManualReview(ctx, req)

	// 验证结果
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)

	mockRepo.AssertExpectations(t)
}

func TestCreateTemplate_Success(t *testing.T) {
	service, mockRepo, _, _ := setupTestService()
	ctx := context.Background()

	// 设置期望
	mockRepo.On("CreateTemplate", ctx, mock.AnythingOfType("*model.AuditTemplate")).Return(uint64(1), nil)

	// 调用服务
	req := &CreateTemplateRequest{
		Name:        "Test Template",
		Description: "Test Description",
		ContentType: "text",
		Level:       "low",
		CreatedBy:   123,
	}
	result, err := service.CreateTemplate(ctx, req)

	// 验证结果
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, uint64(1), result.TemplateID)

	mockRepo.AssertExpectations(t)
}

func TestAddToWhitelist_Success(t *testing.T) {
	service, mockRepo, _, _ := setupTestService()
	ctx := context.Background()

	// 设置期望
	mockRepo.On("AddToWhitelist", ctx, mock.AnythingOfType("*model.AuditWhitelist")).Return(nil)

	// 调用服务
	req := &AddToWhitelistRequest{
		ContentID:   "test-content-id",
		ContentType: "text",
		CreatedBy:   123,
	}
	result, err := service.AddToWhitelist(ctx, req)

	// 验证结果
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)

	mockRepo.AssertExpectations(t)
}

func TestAddToBlacklist_Success(t *testing.T) {
	service, mockRepo, _, _ := setupTestService()
	ctx := context.Background()

	// 设置期望
	mockRepo.On("AddToBlacklist", ctx, mock.AnythingOfType("*model.AuditBlacklist")).Return(nil)

	// 调用服务
	req := &AddToBlacklistRequest{
		ContentID:   "test-content-id",
		ContentType: "text",
		CreatedBy:   123,
	}
	result, err := service.AddToBlacklist(ctx, req)

	// 验证结果
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)

	mockRepo.AssertExpectations(t)
}

func TestGetAuditStatistics_Success(t *testing.T) {
	service, mockRepo, _, _ := setupTestService()
	ctx := context.Background()

	// 设置期望
	mockRepo.On("GetAuditStatistics", ctx, mock.AnythingOfType("*GetAuditStatisticsRequest")).Return(&GetAuditStatisticsResponse{}, nil)

	// 调用服务
	req := &GetAuditStatisticsRequest{
		StartDate: "2023-01-01",
		EndDate:   "2023-12-31",
	}
	result, err := service.GetAuditStatistics(ctx, req)

	// 验证结果
	assert.NoError(t, err)
	assert.NotNil(t, result)

	mockRepo.AssertExpectations(t)
}

func TestGetViolationTrends_Success(t *testing.T) {
	service, mockRepo, _, _ := setupTestService()
	ctx := context.Background()

	// 设置期望
	mockRepo.On("GetViolationTrends", ctx, mock.AnythingOfType("*GetViolationTrendsRequest")).Return(&GetViolationTrendsResponse{}, nil)

	// 调用服务
	req := &GetViolationTrendsRequest{
		StartDate: "2023-01-01",
		EndDate:   "2023-12-31",
	}
	result, err := service.GetViolationTrends(ctx, req)

	// 验证结果
	assert.NoError(t, err)
	assert.NotNil(t, result)

	mockRepo.AssertExpectations(t)
}
