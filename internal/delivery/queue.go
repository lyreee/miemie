package delivery

import (
	"time"
)

// NewQueueManager 创建队列管理器
func NewQueueManager(config QueueConfig) *QueueManager {
	qm := &QueueManager{
		entryQueue:   make(chan DeliveryTask, config.EntryQueueSize),
		highQueue:    make(chan DeliveryTask, config.PriorityQueueSize),
		normalQueue:  make(chan DeliveryTask, config.PriorityQueueSize),
		lowQueue:     make(chan DeliveryTask, config.PriorityQueueSize),
		workerQueues: make([]chan DeliveryTask, 0),
		workerCount:  0,
		config:       config,
	}

	return qm
}

// AddWorker 添加工作队列
func (qm *QueueManager) AddWorker(taskChan chan DeliveryTask) {
	qm.workerQueues = append(qm.workerQueues, taskChan)
	qm.workerCount++
}

// RemoveWorker 移除工作队列
func (qm *QueueManager) RemoveWorker(taskChan chan DeliveryTask) {
	for i, wq := range qm.workerQueues {
		if wq == taskChan {
			qm.workerQueues = append(qm.workerQueues[:i], qm.workerQueues[i+1:]...)
			qm.workerCount--
			break
		}
	}
}

// DispatchTask 分发任务到优先级队列
func (qm *QueueManager) DispatchTask(task DeliveryTask) bool {
	var targetQueue chan DeliveryTask

	// 根据优先级选择队列
	switch {
	case task.Priority <= 3:
		targetQueue = qm.highQueue
	case task.Priority <= 7:
		targetQueue = qm.normalQueue
	default:
		targetQueue = qm.lowQueue
	}

	select {
	case targetQueue <- task:
		return true
	case <-time.After(100 * time.Millisecond):
		return false
	}
}

// GetNextTask 获取下一个任务（优先级调度）
func (qm *QueueManager) GetNextTask() (DeliveryTask, bool) {
	// 高优先级优先检查
	select {
	case task := <-qm.highQueue:
		return task, true
	default:
	}

	// 普通优先级
	select {
	case task := <-qm.normalQueue:
		return task, true
	default:
	}

	// 低优先级
	select {
	case task := <-qm.lowQueue:
		return task, true
	default:
	}

	var emptyTask DeliveryTask
	return emptyTask, false
}

// DistributeToWorkers 分发任务到工作队列
func (qm *QueueManager) DistributeToWorkers(task DeliveryTask) bool {
	if qm.workerCount == 0 {
		return false
	}

	// 简单的轮询分发
	workerIndex := int(time.Now().UnixNano()) % qm.workerCount
	targetQueue := qm.workerQueues[workerIndex]

	select {
	case targetQueue <- task:
		return true
	case <-time.After(50 * time.Millisecond):
		// 如果当前队列满了，尝试其他队列
		for i := 0; i < qm.workerCount; i++ {
			if i == workerIndex {
				continue
			}
			select {
			case qm.workerQueues[i] <- task:
				return true
			default:
				continue
			}
		}
		return false
	}
}

// GetQueueStats 获取队列统计
func (qm *QueueManager) GetQueueStats() map[string]int {
	return map[string]int{
		"entry_queue":   len(qm.entryQueue),
		"high_queue":    len(qm.highQueue),
		"normal_queue":  len(qm.normalQueue),
		"low_queue":     len(qm.lowQueue),
		"worker_count":  qm.workerCount,
	}
}

// AdjustWorkerCount 动态调整邮递员数量
func (qm *QueueManager) AdjustWorkerCount(queueDepth int, workers []*DeliveryWorker) {
	currentWorkers := len(workers)

	// 队列积压严重，增加邮递员
	if queueDepth > qm.config.EntryQueueSize/2 &&
		currentWorkers < qm.config.MaxWorkers {
		qm.addWorker(workers)
	}

	// 队列空闲，减少邮递员
	if queueDepth < qm.config.EntryQueueSize/10 &&
		currentWorkers > qm.config.MinWorkers {
		qm.removeWorker(workers)
	}
}

// addWorker 增加邮递员
func (qm *QueueManager) addWorker(workers []*DeliveryWorker) {
	if len(workers) >= qm.config.MaxWorkers {
		return
	}

	// 这里应该通过某种机制创建新的邮递员
	// 由于workers数组是外部的，我们只记录调整建议
	// logger.Infof("Recommendation: Add worker (current: %d, max: %d)",
	//	len(workers), qm.config.MaxWorkers)
}

// removeWorker 减少邮递员
func (qm *QueueManager) removeWorker(workers []*DeliveryWorker) {
	if len(workers) <= qm.config.MinWorkers {
		return
	}

	// logger.Infof("Recommendation: Remove worker (current: %d, min: %d)",
	//	len(workers), qm.config.MinWorkers)
}