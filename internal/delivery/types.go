package delivery

import (
	"miemie/internal/models"
	"sync"
	"time"
)

// DeliveryTask 投递任务
type DeliveryTask struct {
	ID          string              // 任务ID
	ChannelID   string              // 频道ID
	Message     *models.Message     // 消息内容
	TargetUsers []string            // 目标用户列表
	Priority    int                 // 优先级 (1-10)
	RetryCount  int                 // 重试次数
	CreatedAt   time.Time           // 创建时间
	Timeout     time.Duration       // 超时时间
}

// IsExpired 检查任务是否过期
func (dt *DeliveryTask) IsExpired() bool {
	return time.Since(dt.CreatedAt) > dt.Timeout
}

// RetryTask 重试任务
type RetryTask struct {
	OriginalTask DeliveryTask       // 原始任务
	RetryCount   int                // 重试次数
	NextRetry    time.Time          // 下次重试时间
	Reason       string             // 重试原因
}

// DeliveryWorker 邮递员协程
type DeliveryWorker struct {
	ID         int
	taskChan   chan DeliveryTask    // 个人任务队列
	system     *DeliverySystem
	stopChan   chan bool
	activeJobs sync.Map             // 正在执行的任务
}

// QueueManager 队列管理器
type QueueManager struct {
	entryQueue    chan DeliveryTask    // 入口队列
	highQueue     chan DeliveryTask    // 高优先级队列
	normalQueue   chan DeliveryTask    // 普通优先级队列
	lowQueue      chan DeliveryTask    // 低优先级队列
	workerQueues  []chan DeliveryTask  // 工作队列
	workerCount   int
	config        QueueConfig
}

// QueueConfig 队列配置
type QueueConfig struct {
	EntryQueueSize    int           // 入口队列大小
	PriorityQueueSize int           // 优先级队列大小
	WorkerQueueSize   int           // 工作队列大小
	MaxWorkers        int           // 最大邮递员数量
	MinWorkers        int           // 最小邮递员数量
	QueueTimeout      time.Duration // 队列超时时间
}

// BackpressureCtrl 背压控制器
type BackpressureCtrl struct {
	rejectionCount  int64
	acceptanceCount int64
	windowSize      time.Duration
	lastWindow      time.Time
	mu              sync.Mutex
}

// RetryManager 重试管理器
type RetryManager struct {
	retryQueue  chan RetryTask
	maxRetries  int
	backoffBase time.Duration
	backoffMax  time.Duration
}

// DeliveryStats 投递统计
type DeliveryStats struct {
	TotalReceived     int64     // 总接收数
	TotalDelivered    int64     // 总投递数
	TotalFailed       int64     // 总失败数
	TotalRetried      int64     // 总重试数
	AvgDeliveryTime   time.Duration // 平均投递时间
	QueueDepth        int       // 当前队列深度
	ActiveWorkers     int       // 活跃邮递员数
	LastUpdate        time.Time
}

// MemoryPressure 内存压力级别
type MemoryPressure int

const (
	MemoryPressureLow MemoryPressure = iota
	MemoryPressureMedium
	MemoryPressureHigh
	MemoryPressureCritical
)