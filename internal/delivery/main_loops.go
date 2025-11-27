package delivery

import (
	"miemie/internal/logger"
	"sync/atomic"
	"time"
)

// runMainLoop 运行主处理循环（投递Channel入口）
func (ds *DeliverySystem) runMainLoop() {
	logger.Info("Delivery system main loop started")
	defer logger.Info("Delivery system main loop stopped")

	for {
		select {
		case <-ds.ctx.Done():
			return
		case task := <-ds.inputChan:
			// 将任务分发到优先级队列
			if !ds.queueManager.DispatchTask(task) {
				logger.Infof("Failed to dispatch task %s to priority queue", task.ID)
				atomic.AddInt64(&ds.stats.TotalFailed, 1)
				continue
			}
		}
	}
}

// runQueueManager 运行队列管理器（多层队列处理）
func (ds *DeliverySystem) runQueueManager() {
	logger.Info("Queue manager started")
	defer logger.Info("Queue manager stopped")

	ticker := time.NewTicker(1 * time.Second) // 1秒检查一次，减少日志频率
	defer ticker.Stop()

	for {
		select {
		case <-ds.ctx.Done():
			return
		case <-ticker.C:
			ds.processQueueBacklog()
			ds.adjustWorkerCount()
		}
	}
}

// processQueueBacklog 处理队列积压
func (ds *DeliverySystem) processQueueBacklog() {
	// 从优先级队列获取任务
	for {
		task, hasTask := ds.queueManager.GetNextTask()
		if !hasTask {
			break
		}

		// 分发到工作队列
		if !ds.queueManager.DistributeToWorkers(task) {
			// 工作队列满了，重新排队
			go func(t DeliveryTask) {
				time.Sleep(50 * time.Millisecond)
				ds.queueManager.DispatchTask(t)
			}(task)
			break
		}
	}
}

// adjustWorkerCount 动态调整邮递员数量
func (ds *DeliverySystem) adjustWorkerCount() {
	queueDepth := len(ds.inputChan)
	currentWorkers := len(ds.workers)

	// 动态调整逻辑
	ds.queueManager.AdjustWorkerCount(queueDepth, ds.workers)

	// 实际调整邮递员数量（简化版本）
	if queueDepth > ds.config.QueueLimit/2 && currentWorkers < ds.config.WorkerCount*2 {
		// logger.Infof("Queue depth high (%d), consider adding workers", queueDepth)
	}

	if queueDepth < ds.config.QueueLimit/10 && currentWorkers > ds.config.WorkerCount/2 {
		// logger.Infof("Queue depth low (%d), consider removing workers", queueDepth)
	}
}

// runRetryManager 运行重试管理器
func (ds *DeliverySystem) runRetryManager() {
	logger.Info("Retry manager started")
	defer logger.Info("Retry manager stopped")

	retryWorker := NewRetryWorker(ds.retryManager, ds)

	ds.wg.Add(1)
	go func() {
		defer ds.wg.Done()
		retryWorker.Start(ds.ctx)
	}()

	// 重试工作器的工作任务分发
	for {
		select {
		case <-ds.ctx.Done():
			return
		case task := <-retryWorker.workerChan:
			// 将重试任务直接发送给邮递员
			ds.dispatchToWorker(task)
		}
	}
}

// dispatchToWorker 分发任务给邮递员
func (ds *DeliverySystem) dispatchToWorker(task DeliveryTask) {
	// 简单的轮询分发
	workerIndex := int(time.Now().UnixNano()) % len(ds.workers)
	worker := ds.workers[workerIndex]

	if worker != nil {
		select {
		case worker.taskChan <- task:
			// 任务分发成功
		case <-time.After(50 * time.Millisecond):
			// 邮递员忙，尝试其他邮递员
			for i, w := range ds.workers {
				if i == workerIndex || w == nil {
					continue
				}
				select {
				case w.taskChan <- task:
					return
				default:
					continue
				}
			}
			// 所有邮递员都忙，任务重新排队
			go func(t DeliveryTask) {
				time.Sleep(100 * time.Millisecond)
				ds.inputChan <- t
			}(task)
		}
	}
}

// runStatsCollector 运行统计收集器
func (ds *DeliverySystem) runStatsCollector() {
	logger.Info("Stats collector started")
	defer logger.Info("Stats collector stopped")

	ticker := time.NewTicker(10 * time.Second) // 每10秒收集一次统计
	defer ticker.Stop()

	for {
		select {
		case <-ds.ctx.Done():
			return
		case <-ticker.C:
			ds.collectAndLogStats()
		}
	}
}

// collectAndLogStats 收集并记录统计信息
func (ds *DeliverySystem) collectAndLogStats() {
	stats := ds.GetStats()
	queueStats := ds.queueManager.GetQueueStats()
	retryStats := ds.retryManager.GetRetryStats()
	memoryStats := GetDetailedMemoryStats()
	rejectionCount, acceptanceCount, rejectionRate := ds.backpressure.GetStats()

	logger.Infof("=== Delivery System Stats ===")
	logger.Infof("Received: %d, Delivered: %d, Failed: %d, Retried: %d",
		stats.TotalReceived, stats.TotalDelivered, stats.TotalFailed, stats.TotalRetried)
	logger.Infof("Queue Depth: %d, Active Workers: %d, Avg Delivery Time: %v",
		stats.QueueDepth, stats.ActiveWorkers, stats.AvgDeliveryTime)
	logger.Infof("Queue Stats: %+v", queueStats)
	logger.Infof("Retry Stats: %+v", retryStats)
	logger.Infof("Memory: Alloc=%dMB, Goroutines=%d, Pressure=%s",
		memoryStats["alloc_mb"], memoryStats["goroutines"], GetCurrentMemoryPressure())
	logger.Infof("Backpressure: Reject=%d, Accept=%d, Rate=%.2f%%",
		rejectionCount, acceptanceCount, rejectionRate*100)
	logger.Infof("===============================")
}