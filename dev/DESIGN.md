# 消息通知系统架构设计文档

## 1. 设计理念

### 1.1 核心理念
本系统的核心设计理念是：**消息投递到用户的个人云端存储空间，而非直接投递到设备**。

- **投递目标**：用户的个人云端存储空间
- **访问方式**：多端通过同步机制访问个人空间
- **数据归属**：以个人为单位，频道只是虚拟分组

### 1.2 架构原则
1. **读写分离**：投递后台化，读取优先化
2. **优雅降级**：极端情况下允许投递失败，但保证系统稳定
3. **暴力简单**：同步机制采用Git式的简单策略
4. **水平扩展**：用户间存储天然隔离，支持无限扩展

## 2. 整体架构

### 2.1 系统架构图

```
┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
│   移动端App     │  │   PC客户端       │  │   H5 Web页面    │
└─────────────────┘  └─────────────────┘  └─────────────────┘
         │                    │                    │
         └────────────────────┼────────────────────┘
                              │
              ┌─────────────────┴─────────────────┐
              │         API Gateway              │
              └─────────────────┬─────────────────┘
                              │
              ┌─────────────────▼─────────────────┐
              │        消息投递系统              │
              │                                  │
              │  ┌─────────────────────────────┐ │
              │  │    投递Channel             │ │
              │  └─────────────┬───────────────┘ │
              │                ▼                │
              │  ┌─────────────────────────────┐ │
              │  │    多邮递员协程池          │ │
              │  └─────────────┬───────────────┘ │
              │                ▼                │
              └─────────────────┬─────────────────┘
                              │
              ┌─────────────────▼─────────────────┐
              │        用户个人存储              │
              │  /user_storage/{userId}/          │
              │  ├─ messages.db     (消息存储)   │
              │  ├─ read_status.db  (状态存储)   │
              │  └─ .sync/          (同步信息)   │
              └──────────────────────────────────┘
```

### 2.2 数据流向

**投递流程：**
```
业务服务 → 投递API → 投递Channel → 邮递员协程 → 用户SQLite → 推送通知
```

**同步流程：**
```
客户端 → 同步检查 → 文件下载 → 本地替换 → 数据加载
```

## 3. 存储架构设计

### 3.1 个人存储目录结构

```
/user_storage/{userId}/
├── messages.db              # 消息主数据库
├── messages.db-wal          # WAL日志文件
├── messages.db-shm          # 共享内存文件
├── read_status.db           # 已读状态数据库
├── .sync/
│   ├── last_sync.json       # 最后同步时间
│   ├── messages_sync.json   # 消息同步状态
│   └── read_sync.json       # 状态同步状态
└── backups/
    ├── messages_20250101.db # 定期备份
    └── messages_20250102.db
```

### 3.2 消息数据库结构 (messages.db)

```sql
-- 消息表
CREATE TABLE messages (
    id TEXT PRIMARY KEY,
    channel_id TEXT NOT NULL,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    message_type TEXT DEFAULT 'text',
    priority INTEGER DEFAULT 5,
    sender TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    metadata JSON,
    INDEX idx_channel_created (channel_id, created_at),
    INDEX idx_created (created_at),
    INDEX idx_priority (priority)
);

-- 频道表
CREATE TABLE channels (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    created_by TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_message_at DATETIME
);

-- 用户频道关联表
CREATE TABLE user_channels (
    channel_id TEXT,
    user_id TEXT,
    joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    is_muted BOOLEAN DEFAULT FALSE,
    PRIMARY KEY (channel_id, user_id)
);
```

### 3.3 已读状态数据库结构 (read_status.db)

```sql
-- 已读状态表
CREATE TABLE read_status (
    message_id TEXT PRIMARY KEY,
    read_at DATETIME NOT NULL,
    read_device TEXT,
    archived_at DATETIME,
    starred_at DATETIME,
    metadata JSON
);

-- 阅读统计表
CREATE TABLE read_stats (
    date TEXT PRIMARY KEY,
    total_read INTEGER DEFAULT 0,
    channel_stats JSON,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 阅读位置表
CREATE TABLE reading_position (
    channel_id TEXT PRIMARY KEY,
    last_read_message_id TEXT,
    last_read_at DATETIME,
    position INTEGER DEFAULT 0
);
```

## 4. 消息投递系统设计

### 4.1 邮递员协程架构

```go
type DeliverySystem struct {
    inputChan    chan DeliveryTask    // 投递任务入口
    workers      []*DeliveryWorker   // 邮递员协程池
    workerCount  int                 // 邮递员数量
    queueLimit   int                 // 队列长度限制
    limiter      *rate.Limiter       // 流量控制
    retryManager *RetryManager       // 重试管理器
}

type DeliveryTask struct {
    ID          string              // 任务ID
    ChannelID   string              // 频道ID
    Message     *Message            // 消息内容
    TargetUsers []string            // 目标用户列表
    Priority    int                 // 优先级 (1-10)
    RetryCount  int                 // 重试次数
    CreatedAt   time.Time           // 创建时间
    Timeout     time.Duration       // 超时时间
}

type DeliveryWorker struct {
    ID         int
    taskChan   chan DeliveryTask    // 个人任务队列
    system     *DeliverySystem
    stopChan   chan bool
    activeJobs sync.Map             // 正在执行的任务
}
```

### 4.2 竞争让路机制

**核心策略：读取优先，写入让路**

```go
// 竞争检测式数据库打开
func (dw *DeliveryWorker) openUserDBWithRetry(ctx context.Context, dbPath string) (*sql.DB, error) {
    const maxRetries = 3
    const retryDelay = 50 * time.Millisecond

    for attempt := 0; attempt < maxRetries; attempt++ {
        // 快速检测数据库是否被锁定
        testDB, err := sql.Open("sqlite3", dbPath+"?mode=ro&timeout=1")
        if err != nil {
            return nil, err
        }

        _, err = testDB.Exec("SELECT 1")
        testDB.Close()

        if err == nil {
            // 数据库可访问，正式打开
            db, openErr := sql.Open("sqlite3", dbPath+"?mode=rwc&_busy_timeout=1000")
            if openErr == nil {
                return db, nil
            }
        }

        // 检测到竞争，让路重试
        if attempt < maxRetries-1 {
            select {
            case <-time.After(retryDelay * time.Duration(attempt+1)):
                continue
            case <-ctx.Done():
                return nil, ctx.Err()
            }
        }
    }

    return nil, context.DeadlineExceeded
}
```

### 4.3 智能重试策略

```go
type RetryManager struct {
    retryQueue  chan RetryTask
    maxRetries  int
    backoffBase time.Duration
    backoffMax  time.Duration
}

func (rm *RetryManager) ScheduleRetry(task DeliveryTask, reason string) bool {
    if task.RetryCount >= rm.maxRetries {
        return false // 超过最大重试次数，放弃
    }

    // 指数退避算法
    delay := time.Duration(math.Pow(2, float64(task.RetryCount))) * rm.backoffBase
    if delay > rm.backoffMax {
        delay = rm.backoffMax
    }

    retryTask := RetryTask{
        OriginalTask: task,
        RetryCount:   task.RetryCount + 1,
        NextRetry:    time.Now().Add(delay),
        Reason:       reason,
    }

    select {
    case rm.retryQueue <- retryTask:
        return true
    default:
        return false // 重试队列满了，丢弃任务
    }
}
```

## 5. 队列管理和流控机制

### 5.1 多层队列架构

```
┌─────────────────┐
│   入口层        │ ← 接收所有投递请求
│  entryQueue     │
└─────────┬───────┘
          │
┌─────────▼───────┐
│   缓冲层        │ ← 按优先级分组
│ priorityQueues  │
│  ├─ High        │
│  ├─ Normal      │
│  └─ Low         │
└─────────┬───────┘
          │
┌─────────▼───────┐
│   工作层        │ ← 邮递员个人队列
│ workerQueues    │
│  ├─ Worker1     │
│  ├─ Worker2     │
│  └─ WorkerN     │
└─────────────────┘
```

### 5.2 背压控制机制

```go
type BackpressureCtrl struct {
    rejectionCount  int64
    acceptanceCount int64
    windowSize      time.Duration
    lastWindow      time.Time
    mu              sync.Mutex
}

func (bp *BackpressureCtrl) ShouldAccept(task DeliveryTask) bool {
    bp.mu.Lock()
    defer bp.mu.Unlock()

    // 计算当前拒绝率
    total := bp.rejectionCount + bp.acceptanceCount
    if total == 0 {
        bp.acceptanceCount++
        return true
    }

    rejectionRate := float64(bp.rejectionCount) / float64(total)

    // 根据内存压力和拒绝率决定
    pressure := GetCurrentMemoryPressure()

    switch pressure {
    case MemoryPressureCritical:
        // 只接受高优先级任务
        if task.Priority < 8 {
            bp.rejectionCount++
            return false
        }
    case MemoryPressureHigh:
        // 拒绝率超过30%时开始限流
        if rejectionRate > 0.3 && task.Priority < 6 {
            bp.rejectionCount++
            return false
        }
    case MemoryPressureMedium:
        // 拒绝率超过50%时限制低优先级
        if rejectionRate > 0.5 && task.Priority < 4 {
            bp.rejectionCount++
            return false
        }
    }

    bp.acceptanceCount++
    return true
}
```

### 5.3 动态资源调优

```go
func (qm *QueueManager) adjustWorkerCount() {
    queueDepth := len(qm.entryQueue)
    currentWorkers := len(qm.workerQueues)

    // 队列积压严重，增加邮递员
    if queueDepth > qm.config.EntryQueueSize/2 &&
       currentWorkers < qm.config.MaxWorkers {
        qm.addWorker()
    }

    // 队列空闲，减少邮递员
    if queueDepth < qm.config.EntryQueueSize/10 &&
       currentWorkers > 2 {
        qm.removeWorker()
    }
}
```

## 6. 同步机制设计

### 6.1 简化同步策略

**核心原则：暴力简单，可靠优先**

```json
// 同步检查请求
GET /api/v3/sync/{userId}/check
Response:
{
  "serverVersion": "2025-01-15T10:30:00Z",
  "serverSize": 52428800,
  "serverChecksum": "sha256:abc123...",
  "localVersion": "2025-01-15T09:00:00Z",
  "needSync": true
}

// 完整数据库下载
GET /api/v3/sync/{userId}/download
Response: SQLite文件流
```

### 6.2 分离同步策略

**消息同步：完整数据库替换**
- 检查版本差异
- 下载完整messages.db
- 原子性替换本地文件

**状态同步：增量更新**
- 基于时间戳的增量查询
- 合并状态变更
- 冲突时以服务端为准

### 6.3 客户端同步实现

```javascript
class SQLiteSync {
    async sync() {
        // 检查消息更新
        const msgCheck = await this.checkMessagesSync();
        if (msgCheck.needSync) {
            await this.syncMessages(msgCheck);
        }

        // 同步已读状态
        await this.syncReadStatus();
    }

    async syncMessages(checkResult) {
        // 下载完整数据库
        const response = await fetch(`/api/v3/sync/${this.userId}/download`);
        const blob = await response.blob();

        // 验证文件完整性
        const checksum = await this.calculateChecksum(blob);
        if (checksum !== checkResult.serverChecksum) {
            throw new Error('Checksum mismatch');
        }

        // 原子性替换文件
        await this.replaceLocalDB(blob);

        // 通知应用刷新数据
        this.notifyDataRefreshed();
    }
}
```

## 7. 性能和扩展性分析

### 7.1 性能预估

**SQLite性能指标：**
- 小消息写入(2KB): ~1000 inserts/second
- 批量事务写入: ~5000 inserts/second
- 并发读取: 几乎无限制
- 单用户数据量: 100万消息 ≈ 500MB

**系统吞吐量：**
- 单邮递员: ~100消息/秒
- 10个邮递员: ~1000消息/秒
- 峰值处理: ~5000消息/秒（短期）

### 7.2 存储成本分析

**单用户存储需求：**
- 每日消息：50条 × 2KB = 100KB
- 每月存储：3MB
- 每年存储：36MB
- 索引开销：+20% ≈ 43MB/年

**1万用户规模：**
- 年存储量：430GB
- 对象存储成本：~$60/年
- 备份存储：~$120/年

### 7.3 扩展性设计

**水平扩展能力：**
- ✅ 用户间存储完全隔离，天然支持无限扩展
- ✅ 邮递员协程池可根据负载动态调整
- ✅ 队列系统支持多实例部署

**性能优化措施：**
- SQLite WAL模式提高并发性能
- 按用户分片避免热点
- 冷热数据分离存储
- 智能缓存策略

## 8. 容错和监控

### 8.1 故障处理机制

**数据库锁竞争：**
- 检测到锁定时自动让路
- 多级重试策略
- 超时保护机制

**系统过载保护：**
- 内存监控和背压控制
- 优先级队列处理
- 熔断机制保护核心功能

**数据一致性保证：**
- SQLite ACID特性
- 定期数据库检查点
- 备份和恢复机制

### 8.2 监控指标

**核心监控指标：**
- 队列长度和等待时间
- 邮递员利用率
- 内存使用情况
- 数据库锁竞争频率
- 投递成功率和延迟
- 用户同步状态

**告警阈值：**
- 队列积压 > 80%
- 内存使用 > 90%
- 投递失败率 > 5%
- 数据库锁等待 > 1秒

## 9. 部署建议

### 9.1 系统配置

**硬件配置建议：**
- CPU: 8核心+（支持邮递员协程）
- 内存: 16GB+（队列+缓存）
- 存储: SSD 1TB+（IOPS > 5000）
- 网络: 千兆网络

**软件配置：**
- Go 1.19+
- SQLite 3.39+
- PostgreSQL 14+（元数据存储）
- Redis 6+（缓存）

### 9.2 容量规划

**用户规模对应配置：**

| 用户数 | 邮递员数 | 队列大小 | 内存需求 | 存储需求/年 |
|--------|----------|----------|----------|-------------|
| 1千    | 4        | 1000     | 4GB      | 43GB        |
| 1万    | 20       | 10000    | 8GB      | 430GB       |
| 10万   | 100      | 50000    | 16GB     | 4.3TB       |

### 9.3 运维建议

**备份策略：**
- 每日自动备份用户数据库
- 保留最近30天的备份
- 异地备份关键数据

**监控运维：**
- 实时监控关键指标
- 定期性能测试和调优
- 容量规划和扩容预警

**安全措施：**
- 数据传输加密
- 存储数据加密
- 访问权限控制
- 操作审计日志

## 10. 实施路线图

### 10.1 MVP版本 (1-2个月)
- 基础用户和频道管理
- 简单消息投递功能
- 基本的Web界面
- SQLite存储实现

### 10.2 第二阶段 (2-3个月)
- 多邮递员协程系统
- 完整的同步机制
- 移动端应用
- 推送通知功能

### 10.3 第三阶段 (3-4个月)
- 性能优化和监控
- PC客户端应用
- 高级功能实现
- 安全加固

### 10.4 第四阶段 (持续)
- 容量扩展和优化
- 用户反馈收集和改进
- 新功能开发
- 系统稳定性提升

---

**文档版本：** v1.0
**创建时间：** 2025-01-15
**最后更新：** 2025-01-15
**负责人：** 开发团队