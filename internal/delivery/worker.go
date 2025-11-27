package delivery

import (
	"context"
	"database/sql"
	"fmt"
	"miemie/internal/logger"
	"miemie/internal/storage"
	"sync/atomic"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// NewDeliveryWorker 创建新的邮递员
func NewDeliveryWorker(id int, system *DeliverySystem) *DeliveryWorker {
	return &DeliveryWorker{
		ID:       id,
		taskChan: make(chan DeliveryTask, 100),
		system:   system,
		stopChan: make(chan bool),
	}
}

// Start 启动邮递员
func (dw *DeliveryWorker) Start(ctx context.Context) {
	logger.Infof("Delivery worker %d started", dw.ID)
	defer logger.Infof("Delivery worker %d stopped", dw.ID)

	for {
		select {
		case <-ctx.Done():
			return
		case task := <-dw.taskChan:
			dw.processTask(ctx, task)
		case <-dw.stopChan:
			return
		}
	}
}

// Stop 停止邮递员
func (dw *DeliveryWorker) Stop() {
	close(dw.stopChan)
}

// processTask 处理投递任务
func (dw *DeliveryWorker) processTask(ctx context.Context, task DeliveryTask) {
	startTime := time.Now()

	// 记录活跃任务
	dw.activeJobs.Store(task.ID, startTime)
	defer dw.activeJobs.Delete(task.ID)

	// 检查任务是否过期
	if task.IsExpired() {
		logger.Infof("Task %s expired, skipping", task.ID)
		dw.scheduleRetry(task, "task_expired")
		return
	}

	// 处理目标用户列表
	var targetUsers []string
	if len(task.TargetUsers) > 0 {
		targetUsers = task.TargetUsers
	} else if task.Message != nil {
		// 如果没有指定目标用户，使用消息的接收者
		targetUsers = []string{task.Message.UserID}
	} else {
		logger.Infof("Task %s has no target users", task.ID)
		return
	}

	// 为每个用户投递消息
	successCount := 0
	for _, userID := range targetUsers {
		if err := dw.deliverToUser(ctx, userID, task); err != nil {
			logger.Infof("Failed to deliver task %s to user %s: %v", task.ID, userID, err)
			// 单个用户失败不整体重试，继续处理其他用户
			continue
		}
		successCount++
	}

	// 更新统计
	if successCount > 0 {
		atomic.AddInt64(&dw.system.stats.TotalDelivered, 1)
		deliveryTime := time.Since(startTime)
		dw.updateAvgDeliveryTime(deliveryTime)
	}

	if successCount == 0 && len(targetUsers) > 0 {
		// 所有用户都失败，安排重试
		dw.scheduleRetry(task, "all_users_failed")
	}
}

// deliverToUser 投递消息到指定用户
func (dw *DeliveryWorker) deliverToUser(ctx context.Context, userID string, task DeliveryTask) error {
	if task.Message == nil {
		return fmt.Errorf("message is nil")
	}

	// 获取用户工作空间
	ws, err := dw.system.workspaceManager.GetUserWorkspace(userID)
	if err != nil {
		return fmt.Errorf("failed to get user workspace: %w", err)
	}

	// 🔧 修复：直接使用工作空间的数据库连接，不需要重新打开
	// 使用竞争让路机制检查数据库连接是否可用
	if !dw.isDatabaseAvailable(ctx, ws.MessagesDB) {
		return fmt.Errorf("database not available for user %s", userID)
	}

	// 存储消息到用户数据库
	userStorage := storage.NewUserMessageStorage(ws)
	if err := userStorage.CreateMessage(task.Message); err != nil {
		return fmt.Errorf("failed to create message: %w", err)
	}

	// 通过WebSocket广播给用户
	if dw.system.wsManager != nil {
		dw.system.wsManager.BroadcastMessage(task.Message)
	}

	logger.Infof("Message %s delivered to user %s by worker %d",
		task.Message.ID, userID, dw.ID)

	return nil
}

// isDatabaseAvailable 检查数据库是否可用（简化的竞争让路机制）
func (dw *DeliveryWorker) isDatabaseAvailable(ctx context.Context, db *sql.DB) bool {
	const maxRetries = 3
	const retryDelay = 10 * time.Millisecond

	for attempt := 0; attempt < maxRetries; attempt++ {
		// 快速检测数据库连接是否可用
		conn, err := db.Conn(ctx)
		if err != nil {
			continue
		}

		// 尝试执行一个简单查询
		var result int
		err = conn.QueryRowContext(ctx, "SELECT 1").Scan(&result)
		conn.Close()

		if err == nil && result == 1 {
			return true // 数据库可用
		}

		// 检测到竞争，让路重试
		if attempt < maxRetries-1 {
			select {
			case <-time.After(retryDelay * time.Duration(attempt+1)):
				continue
			case <-ctx.Done():
				return false
			}
		}
	}

	return false // 数据库不可用
}

// scheduleRetry 安排重试
func (dw *DeliveryWorker) scheduleRetry(task DeliveryTask, reason string) {
	if dw.system.retryManager != nil {
		if dw.system.retryManager.ScheduleRetry(task, reason) {
			atomic.AddInt64(&dw.system.stats.TotalRetried, 1)
			logger.Infof("Task %s scheduled for retry: %s", task.ID, reason)
		} else {
			logger.Infof("Task %s abandoned: max retries exceeded", task.ID)
			atomic.AddInt64(&dw.system.stats.TotalFailed, 1)
		}
	}
}

// updateAvgDeliveryTime 更新平均投递时间
func (dw *DeliveryWorker) updateAvgDeliveryTime(deliveryTime time.Duration) {
	dw.system.statsMutex.Lock()
	defer dw.system.statsMutex.Unlock()

	// 简单的移动平均
	if dw.system.stats.AvgDeliveryTime == 0 {
		dw.system.stats.AvgDeliveryTime = deliveryTime
	} else {
		// 使用0.9的权重给历史值，0.1给新值
		dw.system.stats.AvgDeliveryTime =
			time.Duration(float64(dw.system.stats.AvgDeliveryTime)*0.9 + float64(deliveryTime)*0.1)
	}
}

// GetActiveTaskCount 获取当前活跃任务数量
func (dw *DeliveryWorker) GetActiveTaskCount() int {
	count := 0
	dw.activeJobs.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}