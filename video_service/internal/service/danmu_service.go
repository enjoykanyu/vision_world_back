package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/sony/sonyflake"
	"github.com/vision_world/video_service/internal/model"
	"github.com/vision_world/video_service/internal/queue"
	"github.com/vision_world/video_service/internal/repository"
	"github.com/vision_world/video_service/pkg/worker"
	"gorm.io/gorm"
)

type DanmuService struct {
	repo        repository.DanmakuRepository
	mq          *queue.RabbitMQClient
	workerPool  *worker.WorkerPool
	idGenerator *sonyflake.Sonyflake
	db          *gorm.DB
	redis       *redis.Client
}

// 创建弹幕服务
func NewDanmuService(db *gorm.DB, mq *queue.RabbitMQClient, redis *redis.Client) (*DanmuService, error) {

	repo := repository.NewDanmakuRepository(db, redis)

	// 1,初始化ID生成器（雪花算法）
	settings := sonyflake.Settings{
		StartTime: time.Now(),
	}
	s := &DanmuService{
		redis:       redis,
		db:          db,
		mq:          mq,
		idGenerator: sonyflake.NewSonyflake(settings),
		repo:        repo,
	}

	//2,初始化协程池
	s.workerPool = worker.NewWorkerPool(
		10,          // 10个工作协程
		1000,        // 1000个任务队列
		s.handleJob, // 处理函数
	)
	s.workerPool.Start()
	return s, nil
}

// SubmitDanmaku 提交弹幕任务到协程池（异步）
func (s *DanmuService) SubmitDanmaku(req *SendDanmakuRequest) bool {
	log.Printf("SubmitDanmaku: %v", req)
	job := worker.Job{
		Type:    worker.JobTypeSend,
		Payload: req,
	}
	return s.workerPool.SubmitAsync(job)
}

// SendDanmakuRequest 发送弹幕请求
type SendDanmakuRequest struct {
	VideoID   uint64  `json:"video_id" binding:"required"`
	UserID    uint64  `json:"user_id" binding:"required"`
	Content   string  `json:"content" binding:"required,max=100"`
	VideoTime float64 `json:"video_time" binding:"required,min=0"`
	Color     string  `json:"color" default:"#FFFFFF"`
	FontSize  int     `json:"font_size" default:"25"`
	Position  int     `json:"position" default:"0"` // 0滚动 1顶部 2底部
	Speed     int     `json:"speed" default:"0"`    // 0正常 1慢 2快
}

// handleJob 处理任务
func (s *DanmuService) handleJob(job worker.Job) worker.JobResult {
	switch job.Type {
	case worker.JobTypeSend:
		return s.handleSendJob(job)
	case worker.JobTypeAudit:
		return s.handleAuditJob(job)
	default:
		return worker.JobResult{Success: false, Error: fmt.Errorf("未知任务类型")}
	}
}

// handleSendJob 处理发送弹幕任务
func (s *DanmuService) handleSendJob(job worker.Job) worker.JobResult {

	req := job.Payload.(*SendDanmakuRequest)
	ctx := context.Background()

	// 1. 生成唯一ID（雪花算法）
	id, err := s.idGenerator.NextID()
	if err != nil {
		return worker.JobResult{Success: false, Error: err}
	}

	// 3. 构建弹幕对象
	danmaku := &model.Danmuku{
		ID:        uint32(id),
		VideoID:   uint32(req.VideoID),
		UserID:    uint32(req.UserID),
		Content:   req.Content,
		VideoTime: float32(req.VideoTime),
		Color:     req.Color,
		FontSize:  uint32(req.FontSize),
		Position:  uint32(req.Position),
		Speed:     uint32(req.Speed),
	}

	// 5. 写入数据库
	if err := s.repo.CreateDanmaku(ctx, danmaku); err != nil {
		// 记录失败日志，后续可以重试
		log.Printf("Failed to create danmaku: %v", err)
		return worker.JobResult{Success: false, Error: err}
	}

	return worker.JobResult{
		Success: true,
		Data:    danmaku,
	}
}

// handleAuditJob 处理审核弹幕任务
func (s *DanmuService) handleAuditJob(job worker.Job) worker.JobResult {
	return worker.JobResult{
		Success: true,
		Data:    nil,
	}
}

// GetDanmakuByVideoID 获取视频弹幕列表
func (s *DanmuService) GetDanmakuByVideoID(ctx context.Context, videoID uint32) ([]*model.Danmuku, error) {
	return s.repo.GetDanmakuByVideoID(ctx, videoID)
}
