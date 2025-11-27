package delivery

import (
	"context"
	"miemie/internal/logger"
	"math"
	"time"
)

// NewRetryManager 创建重试管理器
func NewRetryManager(maxRetries int, backoffBase, backoffMax time.Duration) *RetryManager {
	return &RetryManager{
		retryQueue:  make(chan RetryTask, maxRetries*100), // 重试队列大小
		maxRetries:  maxRetries,
		backoffBase: backoffBase,
		backoffMax:  backoffMax,
	}
}

// ScheduleRetry 安排重试（核心重试策略）
func (rm *RetryManager) ScheduleRetry(task DeliveryTask, reason string) bool {
	if task.RetryCount >= rm.maxRetries {
		return false // 超过最大重试次数，放弃
	}

	// 指数退避算法
	delay := time.Duration(math.Pow(2, float64(task.RetryCount))) * rm.backoffBase
	if delay > rm.backoffMax {
		delay = rm.backoffMax
	}

	// 添加随机抖动，避免雷群效应
	jitter := time.Duration(float64(delay) * 0.1 * (2.0*float64(task.RetryCount%10)/10.0 - 1.0))
	delay = delay + jitter

	retryTask := RetryTask{
		OriginalTask: task,
		RetryCount:   task.RetryCount + 1,
		NextRetry:    time.Now().Add(delay),
		Reason:       reason,
	}

	select {
	case rm.retryQueue <- retryTask:
		logger.Infof("Task %s scheduled for retry #%d in %v (reason: %s)",
			task.ID, retryTask.RetryCount, delay, reason)
		return true
	default:
		logger.Infof("Retry queue full, task %s abandoned", task.ID)
		return false // 重试队列满了，丢弃任务
	}
}

// GetNextRetry 获取下一个重试任务
func (rm *RetryManager) GetNextRetry() (RetryTask, bool) {
	var emptyTask RetryTask

	// 检查是否有到期的重试任务
	for {
		select {
		case task := <-rm.retryQueue:
			if time.Now().After(task.NextRetry) {
				return task, true
			}
			// 任务还未到期，重新放回队列
			select {
			case rm.retryQueue <- task:
			default:
				// 队列满了，丢弃
				logger.Infof("Retry queue full, task %s lost", task.OriginalTask.ID)
			}
		default:
			return emptyTask, false
		}
	}
}

// GetRetryStats 获取重试统计
func (rm *RetryManager) GetRetryStats() map[string]interface{} {
	return map[string]interface{}{
		"queue_length":   len(rm.retryQueue),
		"max_retries":    rm.maxRetries,
		"backoff_base":   rm.backoffBase.String(),
		"backoff_max":    rm.backoffMax.String(),
	}
}

// RetryWorker 重试工作器
type RetryWorker struct {
	retryManager *RetryManager
	workerChan   chan DeliveryTask
	system       *DeliverySystem
	stopChan     chan bool
}

// NewRetryWorker 创建重试工作器
func NewRetryWorker(retryManager *RetryManager, system *DeliverySystem) *RetryWorker {
	return &RetryWorker{
		retryManager: retryManager,
		workerChan:   make(chan DeliveryTask, 100),
		system:       system,
		stopChan:     make(chan bool),
	}
}

// Start 启动重试工作器
func (rw *RetryWorker) Start(ctx context.Context) {
	logger.Info("Retry worker started")
	defer logger.Info("Retry worker stopped")

	ticker := time.NewTicker(1 * time.Second) // 每秒检查一次重试队列
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-rw.stopChan:
			return
		case <-ticker.C:
			rw.processRetries(ctx)
		}
	}
}

// Stop 停止重试工作器
func (rw *RetryWorker) Stop() {
	close(rw.stopChan)
}

// processRetries 处理重试任务
func (rw *RetryWorker) processRetries(ctx context.Context) {
	// 获取所有到期的重试任务
	for {
		retryTask, hasMore := rw.retryManager.GetNextRetry()
		if !hasMore {
			break
		}

		// 更新重试次数
		retryTask.OriginalTask.RetryCount = retryTask.RetryCount

		select {
		case rw.workerChan <- retryTask.OriginalTask:
			logger.Infof("Retrying task %s (attempt %d)", retryTask.OriginalTask.ID, retryTask.RetryCount)
		case <-time.After(100 * time.Millisecond):
			// 工作队列满了，任务重新排队
			go func(task RetryTask) {
				time.Sleep(100 * time.Millisecond)
				rw.retryManager.ScheduleRetry(task.OriginalTask, "worker_queue_full")
			}(retryTask)
		}
	}
}

// BatchRetryProcessor 批量重试处理器
type BatchRetryProcessor struct {
	retryManager *RetryManager
	batchSize    int
	batchTimeout time.Duration
}

// NewBatchRetryProcessor 创建批量重试处理器
func NewBatchRetryProcessor(retryManager *RetryManager) *BatchRetryProcessor {
	return &BatchRetryProcessor{
		retryManager: retryManager,
		batchSize:    10,
		batchTimeout: 5 * time.Second,
	}
}

// ProcessBatch 处理批量重试
func (brp *BatchRetryProcessor) ProcessBatch(ctx context.Context) []DeliveryTask {
	var tasks []DeliveryTask
	deadline := time.Now().Add(brp.batchTimeout)

	for len(tasks) < brp.batchSize && time.Now().Before(deadline) {
		retryTask, hasMore := brp.retryManager.GetNextRetry()
		if !hasMore {
			break
		}

		retryTask.OriginalTask.RetryCount = retryTask.RetryCount
		tasks = append(tasks, retryTask.OriginalTask)
	}

	return tasks
}