package handler

import (
	"context"
	"github.com/vision_world/audit_service/internal/config"
	"github.com/vision_world/audit_service/internal/model"
	"github.com/vision_world/audit_service/internal/repository"
	"github.com/vision_world/audit_service/pkg/logger"
	pb "github.com/vision_world/audit_service/proto/audit/v1"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// MockAuditService 模拟审核服务
type MockAuditService struct{}

func (m *MockAuditService) SubmitContent(ctx context.Context, content string, contentType model.ContentType, userID string) (*model.AuditRecord, error) {
	return &model.AuditRecord{
		ID:          1,
		Content:     content,
		ContentType: contentType,
		UserID:      userID,
		Status:      model.AuditStatusPending,
	}, nil
}

func (m *MockAuditService) GetAuditResult(ctx context.Context, id uint) (*model.AuditRecord, error) {
	if id == 999 {
		return nil, status.Error(codes.NotFound, "audit record not found")
	}
	return &model.AuditRecord{
		ID:          id,
		Content:     "test content",
		ContentType: model.ContentTypeText,
		UserID:      "user123",
		Status:      model.AuditStatusApproved,
	}, nil
}

func (m *MockAuditService) UpdateAuditStatus(ctx context.Context, id uint, status model.AuditStatus, reason string) error {
	if id == 999 {
		return status.Error(codes.NotFound, "audit record not found")
	}
	return nil
}

func (m *MockAuditService) ListAuditRecords(ctx context.Context, req *repository.ListAuditRecordsRequest) (*repository.ListAuditRecordsResponse, error) {
	return &repository.ListAuditRecordsResponse{
		Records: []*model.AuditRecord{
			{
				ID:          1,
				Content:     "test content 1",
				ContentType: model.ContentTypeText,
				UserID:      "user123",
				Status:      model.AuditStatusApproved,
			},
			{
				ID:          2,
				Content:     "test content 2",
				ContentType: model.ContentTypeImage,
				UserID:      "user456",
				Status:      model.AuditStatusRejected,
			},
		},
		Total: 2,
	}, nil
}

func (m *MockAuditService) GetAuditStatistics(ctx context.Context, startTime, endTime int64) (*model.AuditStatistics, error) {
	return &model.AuditStatistics{
		TotalAudits:    100,
		ApprovedCount:  80,
		RejectedCount:  15,
		PendingCount:   5,
		ViolationCount: 15,
	}, nil
}

func (m *MockAuditService) GetViolationTrends(ctx context.Context, startTime, endTime int64, interval string) ([]*model.ViolationTrend, error) {
	return []*model.ViolationTrend{
		{
			Time:           "2023-01-01",
			ViolationCount: 10,
			TotalCount:     100,
		},
		{
			Time:           "2023-01-02",
			ViolationCount: 15,
			TotalCount:     120,
		},
	}, nil
}

func (m *MockAuditService) IsContentSafe(record *model.AuditRecord) bool {
	return record.Status == model.AuditStatusApproved
}

// MockAuditRepository 模拟审核仓库
type MockAuditRepository struct{}

func (m *MockAuditRepository) CreateAuditRecord(record *model.AuditRecord) error {
	return nil
}

func (m *MockAuditRepository) GetAuditRecord(id uint) (*model.AuditRecord, error) {
	return &model.AuditRecord{
		ID:          id,
		Content:     "test content",
		ContentType: model.ContentTypeText,
		UserID:      "user123",
		Status:      model.AuditStatusApproved,
	}, nil
}

func (m *MockAuditRepository) UpdateAuditStatus(id uint, status model.AuditStatus, reason string) error {
	return nil
}

func (m *MockAuditRepository) ListAuditRecords(req *repository.ListAuditRecordsRequest) (*repository.ListAuditRecordsResponse, error) {
	return &repository.ListAuditRecordsResponse{
		Records: []*model.AuditRecord{
			{
				ID:          1,
				Content:     "test content",
				ContentType: model.ContentTypeText,
				UserID:      "user123",
				Status:      model.AuditStatusApproved,
			},
		},
		Total: 1,
	}, nil
}

func setupTestHandler() *AuditServiceHandler {
	cfg := &config.Config{}
	log, _ := logger.NewLogger("debug", "json")
	mockService := &MockAuditService{}
	mockRepo := &MockAuditRepository{}

	return NewAuditServiceHandler(cfg, log, mockService, mockRepo)
}

func TestSubmitContent_Success(t *testing.T) {
	handler := setupTestHandler()
	ctx := context.Background()

	req := &pb.SubmitContentRequest{
		Content:     "test content",
		ContentType: pb.ContentType_CONTENT_TYPE_TEXT,
		UserId:      "user123",
	}

	resp, err := handler.SubmitContent(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.Success)
	assert.NotEmpty(t, resp.AuditId)
}

func TestSubmitContent_InvalidRequest(t *testing.T) {
	handler := setupTestHandler()
	ctx := context.Background()

	// 测试空内容
	req := &pb.SubmitContentRequest{
		Content:     "",
		ContentType: pb.ContentType_CONTENT_TYPE_TEXT,
		UserId:      "user123",
	}

	resp, err := handler.SubmitContent(ctx, req)

	require.Error(t, err)
	assert.Nil(t, resp)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestGetAuditResult_Success(t *testing.T) {
	handler := setupTestHandler()
	ctx := context.Background()

	req := &pb.GetAuditResultRequest{
		AuditId: "1",
	}

	resp, err := handler.GetAuditResult(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "1", resp.AuditId)
	assert.Equal(t, pb.AuditStatus_AUDIT_STATUS_APPROVED, resp.Status)
}

func TestGetAuditResult_NotFound(t *testing.T) {
	handler := setupTestHandler()
	ctx := context.Background()

	req := &pb.GetAuditResultRequest{
		AuditId: "999",
	}

	resp, err := handler.GetAuditResult(ctx, req)

	require.Error(t, err)
	assert.Nil(t, resp)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
}

func TestUpdateAuditStatus_Success(t *testing.T) {
	handler := setupTestHandler()
	ctx := context.Background()

	req := &pb.UpdateAuditStatusRequest{
		AuditId: "1",
		Status:  pb.AuditStatus_AUDIT_STATUS_APPROVED,
		Reason:  "manual review",
	}

	resp, err := handler.UpdateAuditStatus(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.Success)
}

func TestListAuditRecords_Success(t *testing.T) {
	handler := setupTestHandler()
	ctx := context.Background()

	req := &pb.ListAuditRecordsRequest{
		Page:     1,
		PageSize: 10,
	}

	resp, err := handler.ListAuditRecords(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Records, 2)
	assert.Equal(t, int32(2), resp.Total)
}

func TestGetAuditStatistics_Success(t *testing.T) {
	handler := setupTestHandler()
	ctx := context.Background()

	req := &pb.GetAuditStatisticsRequest{
		StartTime: 1672531200, // 2023-01-01
		EndTime:   1675209600, // 2023-02-01
	}

	resp, err := handler.GetAuditStatistics(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, int32(100), resp.TotalAudits)
	assert.Equal(t, int32(80), resp.ApprovedCount)
	assert.Equal(t, int32(15), resp.RejectedCount)
	assert.Equal(t, int32(5), resp.PendingCount)
}

func TestGetViolationTrends_Success(t *testing.T) {
	handler := setupTestHandler()
	ctx := context.Background()

	req := &pb.GetViolationTrendsRequest{
		StartTime: 1672531200, // 2023-01-01
		EndTime:   1675209600, // 2023-02-01
		Interval:  "daily",
	}

	resp, err := handler.GetViolationTrends(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Trends, 2)
	assert.Equal(t, "2023-01-01", resp.Trends[0].Time)
	assert.Equal(t, int32(10), resp.Trends[0].ViolationCount)
}
