package transcode

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/vision_world/video_service/pkg/logger"
)

// Config 转码服务配置
type Config struct {
	FFmpegPath      string        `mapstructure:"ffmpeg_path"`      // FFmpeg可执行文件路径
	WorkDir         string        `mapstructure:"work_dir"`         // 工作目录，用于临时文件
	OutputDir       string        `mapstructure:"output_dir"`       // 输出目录
	Preset          string        `mapstructure:"preset"`           // 转码预设（ultrafast, fast, medium, slow, veryslow）
	SegmentDuration int           `mapstructure:"segment_duration"` // 分片时长（秒），默认为0表示自适应
	Timeout         time.Duration `mapstructure:"timeout"`          // 转码超时时间
	LogLevel        string        `mapstructure:"log_level"`        // 日志级别
}

// Quality 视频质量配置
type Quality struct {
	Name      string // 质量名称（1080p, 720p, 480p）
	Width     int    // 宽度
	Height    int    // 高度
	Bitrate   string // 视频码率（如"5000k"）
	AudioRate string // 音频码率（如"128k"）
}

// Task 转码任务
type Task struct {
	ID           string    // 任务ID
	VideoID      string    // 视频ID
	InputPath    string    // 输入文件路径
	OutputPath   string    // 输出目录路径
	Qualities    []Quality // 转码质量列表
	Status       string    // 任务状态（pending, processing, completed, failed）
	ErrorMessage string    // 错误信息
	CreatedAt    time.Time // 创建时间
	UpdatedAt    time.Time // 更新时间
}

// Service 转码服务接口
type Service interface {
	// SubmitTask 提交转码任务
	SubmitTask(ctx context.Context, task *Task) error

	// GetTaskStatus 获取任务状态
	GetTaskStatus(ctx context.Context, taskID string) (*Task, error)

	// TranscodeVideo 执行视频转码（同步）
	TranscodeVideo(ctx context.Context, task *Task) error
}

// service 转码服务实现
type service struct {
	config Config
	logger logger.Logger
}

// NewService 创建转码服务实例
func NewService(cfg Config, log logger.Logger) Service {
	// 确保工作目录存在
	os.MkdirAll(cfg.WorkDir, 0755)
	os.MkdirAll(cfg.OutputDir, 0755)

	return &service{
		config: cfg,
		logger: log,
	}
}

// SubmitTask 提交转码任务
func (s *service) SubmitTask(ctx context.Context, task *Task) error {
	// 目前简单实现，直接执行转码
	return s.TranscodeVideo(ctx, task)
}

// GetTaskStatus 获取任务状态
func (s *service) GetTaskStatus(ctx context.Context, taskID string) (*Task, error) {
	// 目前简单实现，返回nil
	return nil, fmt.Errorf("not implemented")
}

// TranscodeVideo 执行视频转码
func (s *service) TranscodeVideo(ctx context.Context, task *Task) error {
	s.logger.Info("Starting video transcoding process", "task_id", task.ID, "video_id", task.VideoID, "input_path", task.InputPath, "output_path", task.OutputPath)
	task.Status = "processing"
	task.UpdatedAt = time.Now()

	// 确保输出目录存在
	if err := os.MkdirAll(task.OutputPath, 0755); err != nil {
		s.logger.Error("Failed to create output directory", "task_id", task.ID, "output_path", task.OutputPath, "error", err)
		task.Status = "failed"
		task.ErrorMessage = fmt.Sprintf("failed to create output directory: %v", err)
		task.UpdatedAt = time.Now()
		return err
	}

	// 检查FFmpeg路径是否有效
	s.logger.Info("Checking FFmpeg path", "task_id", task.ID, "ffmpeg_path", s.config.FFmpegPath)
	if err := s.checkFFmpegPath(); err != nil {
		s.logger.Error("FFmpeg path check failed", "task_id", task.ID, "error", err)
		task.Status = "failed"
		task.ErrorMessage = err.Error()
		task.UpdatedAt = time.Now()
		return err
	}

	s.logger.Info("FFmpeg path check passed", "task_id", task.ID, "ffmpeg_path", s.config.FFmpegPath)

	// 获取视频时长，用于自适应分片
	videoDuration, err := s.getVideoDuration(task.InputPath)
	if err != nil {
		s.logger.Error("Failed to get video duration", "task_id", task.ID, "error", err)
		task.Status = "failed"
		task.ErrorMessage = fmt.Sprintf("failed to get video duration: %v", err)
		task.UpdatedAt = time.Now()
		return err
	}

	// 根据视频时长自适应调整分片时长
	segmentDuration := s.calculateAdaptiveSegmentDuration(videoDuration)
	s.logger.Info("Adaptive segment duration calculated", "task_id", task.ID, "video_duration", videoDuration, "segment_duration", segmentDuration)

	// 生成HLS主播放列表
	s.logger.Info("Generating master playlist", "task_id", task.ID, "video_id", task.VideoID)
	masterPlaylistPath := filepath.Join(task.OutputPath, "index.m3u8")
	masterPlaylist := s.generateMasterPlaylist(task.Qualities, task.VideoID, videoDuration, segmentDuration)
	if err := os.WriteFile(masterPlaylistPath, []byte(masterPlaylist), 0644); err != nil {
		s.logger.Error("Failed to write master playlist", "task_id", task.ID, "master_playlist_path", masterPlaylistPath, "error", err)
		task.Status = "failed"
		task.ErrorMessage = fmt.Sprintf("failed to write master playlist: %v", err)
		task.UpdatedAt = time.Now()
		return fmt.Errorf("failed to write master playlist: %w", err)
	}

	s.logger.Info("Master playlist generated successfully", "task_id", task.ID, "master_playlist_path", masterPlaylistPath, "content", masterPlaylist)

	// 为每种质量执行转码
	for i, quality := range task.Qualities {
		s.logger.Info("Starting transcoding for quality",
			"task_id", task.ID,
			"video_id", task.VideoID,
			"quality_index", i,
			"quality_name", quality.Name,
			"width", quality.Width,
			"height", quality.Height,
			"bitrate", quality.Bitrate)

		if err := s.transcodeQuality(ctx, task, quality, segmentDuration); err != nil {
			s.logger.Error("Failed to transcode quality", "task_id", task.ID, "quality_name", quality.Name, "error", err)
			task.Status = "failed"
			task.ErrorMessage = fmt.Sprintf("failed to transcode quality %s: %v", quality.Name, err)
			task.UpdatedAt = time.Now()
			return fmt.Errorf("failed to transcode quality %s: %w", quality.Name, err)
		}

		s.logger.Info("Quality transcoding completed successfully", "task_id", task.ID, "quality_name", quality.Name)
	}

	// 生成分片元数据文件
	if err := s.generateSegmentMetadata(task, videoDuration, segmentDuration); err != nil {
		s.logger.Warn("Failed to generate segment metadata", "task_id", task.ID, "error", err)
		// 不影响主流程，继续执行
	}

	task.Status = "completed"
	task.UpdatedAt = time.Now()

	s.logger.Info("Video transcoding completed successfully",
		"task_id", task.ID,
		"video_id", task.VideoID,
		"output_path", task.OutputPath)

	return nil
}

// checkFFmpegPath 检查FFmpeg路径是否有效
func (s *service) checkFFmpegPath() error {
	// 检查当前配置的FFmpeg路径
	if _, err := os.Stat(s.config.FFmpegPath); err == nil {
		return nil
	}

	// 如果配置的路径无效，尝试在PATH中查找
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		s.logger.Error("FFmpeg not found in PATH", "error", err)
		return fmt.Errorf("ffmpeg not found: %w", err)
	}

	// 更新配置中的FFmpeg路径
	s.config.FFmpegPath = ffmpegPath
	s.logger.Info("Found FFmpeg in PATH, updated configuration", "path", ffmpegPath)

	return nil
}

// generateMasterPlaylist 生成HLS主播放列表
func (s *service) generateMasterPlaylist(qualities []Quality, videoID string, videoDuration float64, segmentDuration int) string {
	var playlist strings.Builder

	// HLS主播放列表头部
	playlist.WriteString("#EXTM3U\n")
	playlist.WriteString("#EXT-X-VERSION:3\n\n")

	// 为每种质量添加播放列表条目
	for _, quality := range qualities {
		playlist.WriteString(fmt.Sprintf("#EXT-X-STREAM-INF:BANDWIDTH=%s,RESOLUTION=%dx%d\n",
			quality.Bitrate, quality.Width, quality.Height))
		playlist.WriteString(fmt.Sprintf("%s/index.m3u8\n\n", quality.Name))
	}

	return playlist.String()
}

// getVideoDuration 获取视频时长
func (s *service) getVideoDuration(inputPath string) (float64, error) {
	cmd := exec.Command(s.config.FFmpegPath,
		"-i", inputPath,
		"-f", "null",
		"-")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("failed to get video duration: %w, output: %s", err, string(output))
	}

	// 解析输出获取时长
	outputStr := string(output)
	durationStr := ""
	for _, line := range strings.Split(outputStr, "\n") {
		if strings.Contains(line, "Duration:") {
			parts := strings.Split(line, "Duration:")
			if len(parts) > 1 {
				durationStr = strings.TrimSpace(strings.Split(parts[1], ",")[0])
			}
			break
		}
	}

	if durationStr == "" {
		return 0, fmt.Errorf("duration not found in ffmpeg output")
	}

	// 解析时长字符串（格式：HH:MM:SS.mmm）
	var hours, minutes, seconds, milliseconds float64
	fmt.Sscanf(durationStr, "%f:%f:%f.%f", &hours, &minutes, &seconds, &milliseconds)

	totalSeconds := hours*3600 + minutes*60 + seconds + milliseconds/1000
	return totalSeconds, nil
}

// calculateAdaptiveSegmentDuration 根据视频时长自适应计算分片时长
func (s *service) calculateAdaptiveSegmentDuration(videoDuration float64) int {
	// 如果配置了固定分片时长，使用配置值
	if s.config.SegmentDuration > 0 {
		return s.config.SegmentDuration
	}

	// 根据视频时长自适应调整分片大小
	// 短视频（<5分钟）：5秒/片
	// 中等视频（5-30分钟）：10秒/片
	// 长视频（>30分钟）：15秒/片
	durationMinutes := videoDuration / 60

	if durationMinutes < 5 {
		return 5
	} else if durationMinutes < 30 {
		return 10
	} else {
		return 15
	}
}

// generateSegmentMetadata 生成分片元数据文件
func (s *service) generateSegmentMetadata(task *Task, videoDuration float64, segmentDuration int) error {
	metadataPath := filepath.Join(task.OutputPath, "segments.json")

	// 估算分片数量
	segmentCount := int(math.Ceil(videoDuration / float64(segmentDuration)))

	// 构建元数据
	metadata := map[string]interface{}{
		"video_id":         task.VideoID,
		"video_duration":   videoDuration,
		"segment_duration": segmentDuration,
		"segment_count":    segmentCount,
		"segments":         []interface{}{},
		"generated_at":     time.Now().Unix(),
	}

	// 添加分片信息
	segments := metadata["segments"].([]interface{})
	for i := 0; i < segmentCount; i++ {
		segmentStart := float64(i) * float64(segmentDuration)
		segmentEnd := math.Min(segmentStart+float64(segmentDuration), videoDuration)

		segments = append(segments, map[string]interface{}{
			"index":    i,
			"start":    segmentStart,
			"end":      segmentEnd,
			"duration": segmentEnd - segmentStart,
			"filename": fmt.Sprintf("segment_%03d.ts", i),
		})
	}
	metadata["segments"] = segments

	// 转换为JSON
	jsonData, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal segment metadata: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(metadataPath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write segment metadata: %w", err)
	}

	s.logger.Info("Segment metadata generated successfully", "task_id", task.ID, "metadata_path", metadataPath, "segment_count", segmentCount)
	return nil
}

// transcodeQuality 转码单个质量
func (s *service) transcodeQuality(ctx context.Context, task *Task, quality Quality, segmentDuration int) error {
	// 创建质量输出目录
	qualityDir := filepath.Join(task.OutputPath, quality.Name)
	os.MkdirAll(qualityDir, 0755)

	// 输出文件路径
	outputPath := filepath.Join(qualityDir, "index.m3u8")
	segmentPath := filepath.Join(qualityDir, "segment_%03d.ts")

	// 构建FFmpeg命令
	cmd := exec.CommandContext(ctx, s.config.FFmpegPath,
		"-i", task.InputPath,
		"-preset", s.config.Preset,
		"-c:v", "h264",
		"-b:v", quality.Bitrate,
		"-s", fmt.Sprintf("%dx%d", quality.Width, quality.Height),
		"-c:a", "aac",
		"-b:a", quality.AudioRate,
		"-f", "hls",
		"-hls_time", fmt.Sprintf("%d", segmentDuration),
		"-hls_list_size", "0", // 保留所有分片
		"-hls_segment_filename", segmentPath,
		outputPath)

	// 捕获输出
	output, err := cmd.CombinedOutput()
	if err != nil {
		s.logger.Error("FFmpeg transcode failed",
			"video_id", task.VideoID,
			"quality", quality.Name,
			"error", err,
			"output", string(output))
		return fmt.Errorf("ffmpeg transcode failed: %w, output: %s", err, string(output))
	}

	s.logger.Info("FFmpeg transcode completed",
		"video_id", task.VideoID,
		"quality", quality.Name,
		"output_path", outputPath)

	return nil
}

// DefaultQualities 默认转码质量配置
func DefaultQualities() []Quality {
	return []Quality{
		{
			Name:      "1080p",
			Width:     1920,
			Height:    1080,
			Bitrate:   "5000k",
			AudioRate: "128k",
		},
		{
			Name:      "720p",
			Width:     1280,
			Height:    720,
			Bitrate:   "3000k",
			AudioRate: "128k",
		},
		{
			Name:      "480p",
			Width:     854,
			Height:    480,
			Bitrate:   "1500k",
			AudioRate: "96k",
		},
	}
}
