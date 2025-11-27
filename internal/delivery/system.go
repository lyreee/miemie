package delivery

import (
	"context"
	"fmt"
	"miemie/internal/config"
	"miemie/internal/logger"
	"miemie/internal/models"
	"miemie/internal/websocket"
	"miemie/internal/workspace"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// DeliverySystem 消息投递系统
type DeliverySystem struct {
	// 核心组件
	inputChan    chan DeliveryTask    // 投递任务入口
	workers      []*DeliveryWorker   // 邮递员协程池
	queueManager *QueueManager        // 队列管理器
	retryManager *RetryManager        // 重试管理器
	backpressure *BackpressureCtrl    // 背压控制

	// 配置
	config DeliveryConfig

	// 状态管理
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	stats      DeliveryStats
	statsMutex sync.RWMutex

	// 外部依赖
	workspaceManager *workspace.Manager
	wsManager        *websocket.Manager
}

// DeliveryConfig 投递系统配置
type DeliveryConfig struct {
	WorkerCount      int           // 邮递员数量
	QueueLimit       int           // 队列长度限制
	RateLimit        int           // 每秒速率限制
	TaskTimeout      time.Duration // 任务超时时间
	MaxRetries       int           // 最大重试次数
	RetryBackoffBase time.Duration // 重试退避基数
	RetryBackoffMax  time.Duration // 重试退避最大值
}

// NewDeliverySystem 创建新的投递系统
func NewDeliverySystem(workspaceManager *workspace.Manager, wsManager *websocket.Manager) *DeliverySystem {
	return NewDeliverySystemWithConfig(workspaceManager, wsManager, nil)
}

// NewDeliverySystemWithConfig 创建带有配置的投递系统
func NewDeliverySystemWithConfig(workspaceManager *workspace.Manager, wsManager *websocket.Manager, cfg *config.Config) *DeliverySystem {
	ctx, cancel := context.WithCancel(context.Background())

	// 使用配置文件中的值，如果配置为空则使用默认值
	var config DeliveryConfig
	if cfg != nil {
		workerCount := cfg.Delivery.Workers.Count
		if workerCount == 0 {
			workerCount = runtime.NumCPU() // 0表示自动检测CPU核心数
		}

		config = DeliveryConfig{
			WorkerCount:      workerCount,
			QueueLimit:       cfg.Delivery.Queue.EntrySize,
			RateLimit:        1000, // 这个值可以后续从配置中添加
			TaskTimeout:      cfg.Delivery.GetTaskTimeout(),
			MaxRetries:       cfg.Delivery.Task.MaxRetries,
			RetryBackoffBase: cfg.Delivery.GetRetryBackoffBase(),
			RetryBackoffMax:  cfg.Delivery.GetRetryBackoffMax(),
		}
	} else {
		config = DeliveryConfig{
			WorkerCount:      runtime.NumCPU(), // 默认使用CPU核心数
			QueueLimit:       10000,
			RateLimit:        1000,
			TaskTimeout:      30 * time.Second,
			MaxRetries:       3,
			RetryBackoffBase: 100 * time.Millisecond,
			RetryBackoffMax:  5 * time.Second,
		}
	}

	ds := &DeliverySystem{
		ctx:              ctx,
		cancel:           cancel,
		inputChan:        make(chan DeliveryTask, config.QueueLimit),
		workspaceManager: workspaceManager,
		wsManager:        wsManager,
		config:           config,
	}

	// 初始化各个组件
	ds.initQueueManager()
	ds.initRetryManager()
	ds.initBackpressureControl()
	ds.initWorkers()

	return ds
}

// Start 启动投递系统
func (ds *DeliverySystem) Start() error {
	logger.Infof("Starting delivery system with %d workers", ds.config.WorkerCount)

	// 启动队列管理器
	ds.wg.Add(1)
	go ds.runQueueManager()

	// 启动重试管理器
	ds.wg.Add(1)
	go ds.runRetryManager()

	// 启动主处理循环
	ds.wg.Add(1)
	go ds.runMainLoop()

	// 启动统计收集器
	ds.wg.Add(1)
	go ds.runStatsCollector()

	logger.Info("Delivery system started successfully")
	return nil
}

// Stop 停止投递系统
func (ds *DeliverySystem) Stop() error {
	logger.Info("Stopping delivery system...")

	ds.cancel() // 取消上下文

	// 等待所有goroutine结束
	done := make(chan struct{})
	go func() {
		ds.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Info("Delivery system stopped gracefully")
		return nil
	case <-time.After(30 * time.Second):
		logger.Warn("Delivery system stop timeout")
		return fmt.Errorf("stop timeout")
	}
}

// SubmitTask 提交投递任务
func (ds *DeliverySystem) SubmitTask(task DeliveryTask) error {
	// 检查背压
	if !ds.backpressure.ShouldAccept(task) {
		atomic.AddInt64(&ds.stats.TotalFailed, 1)
		return fmt.Errorf("rejected due to backpressure")
	}

	// 生成任务ID
	if task.ID == "" {
		task.ID = generateTaskID()
	}

	// 设置默认值
	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now()
	}
	if task.Timeout == 0 {
		task.Timeout = ds.config.TaskTimeout
	}

	select {
	case ds.inputChan <- task:
		atomic.AddInt64(&ds.stats.TotalReceived, 1)
		return nil
	case <-time.After(100 * time.Millisecond):
		atomic.AddInt64(&ds.stats.TotalFailed, 1)
		return fmt.Errorf("queue full, task rejected")
	}
}

// SubmitMessage 提交消息投递（便捷方法）
func (ds *DeliverySystem) SubmitMessage(message *models.Message, targetUsers []string) error {
	task := DeliveryTask{
		ChannelID:   message.ChannelID,
		Message:     message,
		TargetUsers: targetUsers,
		Priority:    message.Priority,
	}

	return ds.SubmitTask(task)
}

// GetStats 获取投递统计
func (ds *DeliverySystem) GetStats() DeliveryStats {
	ds.statsMutex.RLock()
	defer ds.statsMutex.RUnlock()

	stats := ds.stats
	stats.QueueDepth = len(ds.inputChan)
	stats.ActiveWorkers = ds.getActiveWorkerCount()
	stats.LastUpdate = time.Now()

	return stats
}

// getActiveWorkerCount 获取活跃邮递员数量
func (ds *DeliverySystem) getActiveWorkerCount() int {
	count := 0
	for _, worker := range ds.workers {
		if worker != nil {
			count++
		}
	}
	return count
}

// generateTaskID 生成任务ID
func generateTaskID() string {
	return fmt.Sprintf("task_%d_%d", time.Now().UnixNano(), runtime.NumGoroutine())
}

// initQueueManager 初始化队列管理器
func (ds *DeliverySystem) initQueueManager() {
	queueConfig := QueueConfig{
		EntryQueueSize:    ds.config.QueueLimit,
		PriorityQueueSize: ds.config.QueueLimit / 3,
		WorkerQueueSize:   100,
		MaxWorkers:        ds.config.WorkerCount * 2,
		MinWorkers:        2,
		QueueTimeout:      5 * time.Second,
	}

	ds.queueManager = NewQueueManager(queueConfig)
}

// initRetryManager 初始化重试管理器
func (ds *DeliverySystem) initRetryManager() {
	ds.retryManager = NewRetryManager(
		ds.config.MaxRetries,
		ds.config.RetryBackoffBase,
		ds.config.RetryBackoffMax,
	)
}

// initBackpressureControl 初始化背压控制
func (ds *DeliverySystem) initBackpressureControl() {
	ds.backpressure = NewBackpressureCtrl()
}

// initWorkers 初始化邮递员协程池
func (ds *DeliverySystem) initWorkers() {
	ds.workers = make([]*DeliveryWorker, ds.config.WorkerCount)

	for i := 0; i < ds.config.WorkerCount; i++ {
		worker := NewDeliveryWorker(i, ds)
		ds.workers[i] = worker

		// 🔧 关键修复：将邮递员的工作队列注册到队列管理器
		ds.queueManager.AddWorker(worker.taskChan)

		// 启动邮递员
		ds.wg.Add(1)
		go func(w *DeliveryWorker) {
			defer ds.wg.Done()
			w.Start(ds.ctx)
		}(worker)
	}
}