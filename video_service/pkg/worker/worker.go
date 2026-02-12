package worker

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Job 弹幕处理任务
type Job struct {
	ID      uint64
	Type    JobType // send:发送, audit:审核
	Payload interface{}
	Result  chan<- JobResult
}

type JobType string

const (
	JobTypeSend  JobType = "send"
	JobTypeAudit JobType = "audit"
)

type JobResult struct {
	Success bool
	Data    interface{}
	Error   error
}

// WorkerPool 协程池
type WorkerPool struct {
	workers  int
	jobQueue chan Job
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
	handler  func(Job) JobResult
}

// NewWorkerPool 创建协程池
func NewWorkerPool(workers int, queueSize int, handler func(Job) JobResult) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	return &WorkerPool{
		workers:  workers,
		jobQueue: make(chan Job, queueSize),
		ctx:      ctx,
		cancel:   cancel,
		handler:  handler,
	}
}

// Start 启动协程池
func (wp *WorkerPool) Start() {
	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}
}

// worker 工作协程
func (wp *WorkerPool) worker(id int) {
	defer wp.wg.Done()

	for {
		select {
		case job := <-wp.jobQueue:
			// 处理任务
			result := wp.handler(job)
			if job.Result != nil {
				select {
				case job.Result <- result:
				case <-time.After(time.Second):
					// 结果通道超时，避免阻塞
				}
			}

		case <-wp.ctx.Done():
			return
		}
	}
}

// Submit 提交任务（同步等待结果）
func (wp *WorkerPool) Submit(job Job) (JobResult, error) {
	resultChan := make(chan JobResult, 1)
	job.Result = resultChan

	select {
	case wp.jobQueue <- job:
		// 等待结果
		select {
		case result := <-resultChan:
			return result, nil
		case <-time.After(5 * time.Second):
			return JobResult{}, fmt.Errorf("任务处理超时")
		}
	case <-time.After(time.Second):
		return JobResult{}, fmt.Errorf("任务队列已满")
	}
}

// SubmitAsync 异步提交任务
func (wp *WorkerPool) SubmitAsync(job Job) bool {
	select {
	case wp.jobQueue <- job:
		return true
	default:
		return false
	}
}

// Stop 停止协程池
func (wp *WorkerPool) Stop() {
	wp.cancel()
	close(wp.jobQueue)
	wp.wg.Wait()
}
