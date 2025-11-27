# 用户服务技术架构文档

## 1. 整体架构设计

### 1.1 服务端口分离
- **Admin服务**：`8081`端口 - 管理后台
- **User服务**：`8080`端口 - 客户端服务

### 1.2 架构原则
- 与admin服务共享相同的用户存储层
- 独立的API网关和认证体系
- 实时通信与REST API并存
- 多端在线状态同步

### 1.3 系统架构图

```
┌─────────────────────────────────────────────────────────┐
│                    客户端层                              │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │
│  │   移动端App │  │   PC客户端   │  │   H5 Web    │     │
│  └─────────────┘  └─────────────┘  └─────────────┘     │
└─────────────────────────────────────────────────────────┘
                                │
                                │ HTTPS + WebSocket
                                ▼
┌─────────────────────────────────────────────────────────┐
│                 用户服务 (8080)                          │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │
│  │  OAuth2认证  │  │  WebSocket  │  │   HTTP API  │     │
│  │             │  │   即时通知   │  │   消息读取   │     │
│  └─────────────┘  └─────────────┘  └─────────────┘     │
└─────────────────────────────────────────────────────────┘
                                │
                                │ 共享存储访问
                                ▼
┌─────────────────────────────────────────────────────────┐
│                  共享存储层                              │
│  /user_storage/{userId}/                                  │
│  ├─ messages.db     (消息存储)                           │
│  ├─ read_status.db  (状态存储)                           │
│  └─ .sync/          (同步信息)                           │
└─────────────────────────────────────────────────────────┘
```

## 2. WebSocket即时通知系统

### 2.1 连接管理架构

```go
type ConnectionManager struct {
    connections sync.Map             // userId -> []*ClientConnection
    hubs        map[string]*Hub      // channelId -> Hub
    limiter     *rate.Limiter        // 连接限制
}

type ClientConnection struct {
    ID          string              // 连接唯一标识
    UserID      string              // 用户ID
    DeviceType  string              // 设备类型 (mobile, desktop, web)
    Socket      *websocket.Conn     // WebSocket连接
    LastSeen    time.Time          // 最后活跃时间
    SendChan    chan []byte        // 发送队列
    CloseChan   chan bool          // 关闭信号
}

type Hub struct {
    ChannelID   string              // 频道ID
    Clients     map[string]bool     // 在线用户ID集合
    Broadcast   chan []byte         // 广播消息
    Register    chan *Client        // 用户注册
    Unregister  chan *Client        // 用户注销
}
```

### 2.2 多端同步策略

- **状态广播**：消息推送到用户的所有在线设备
- **状态同步**：跨设备的已读状态实时同步
- **在线管理**：心跳检测 + 设备去重

### 2.3 消息路由流程

```
消息投递系统 → WebSocket Hub → 用户所有连接 → 客户端接收
```

### 2.4 WebSocket API设计

```javascript
// 客户端连接
ws://localhost:8080/ws?token={jwt_token}

// 消息格式
{
    "type": "message",
    "data": {
        "id": "msg-123",
        "channel_id": "channel-456",
        "title": "新消息",
        "content": "消息内容",
        "created_at": "2025-01-15T10:30:00Z"
    }
}

// 状态更新消息
{
    "type": "status_update",
    "data": {
        "message_id": "msg-123",
        "status": "read",
        "device_id": "device-789"
    }
}

// 心跳消息
{
    "type": "ping"
}
```

## 3. HTTP消息读取和已读确认机制

### 3.1 API设计

```go
// 消息发送API (核心工作接口)
POST   /api/v3/messages/send              // 发送单条消息
POST   /api/v3/messages/send-batch        // 批量发送消息
POST   /api/v3/messages/send-template     // 使用模板发送消息
GET    /api/v3/messages/templates         // 获取消息模板列表
POST   /api/v3/messages/templates         // 创建消息模板
PUT    /api/v3/messages/templates/{id}    // 更新消息模板
DELETE /api/v3/messages/templates/{id}    // 删除消息模板

// 消息读取API
GET    /api/v3/messages                    // 获取消息列表
GET    /api/v3/messages/{id}              // 获取单条消息
POST   /api/v3/messages/{id}/read         // 标记已读
POST   /api/v3/messages/read-batch        // 批量标记已读
GET    /api/v3/messages/unread-count      // 获取未读数量

// 频道相关API
GET    /api/v3/channels                   // 获取用户频道列表
GET    /api/v3/channels/{id}/messages     // 获取频道消息

// 数据库备份
GET    /api/v3/backup/database            // 下载SQLite数据库
GET    /api/v3/backup/messages            // 仅备份消息数据
```

### 3.2 已读状态同步流程

```
客户端读取消息 → HTTP已读确认 → 更新SQLite → WebSocket广播 → 其他设备同步
```

### 3.3 API响应格式

```javascript
// 获取消息列表响应
{
    "code": 200,
    "message": "success",
    "data": {
        "messages": [
            {
                "id": "msg-123",
                "channel_id": "channel-456",
                "title": "消息标题",
                "content": "消息内容",
                "message_type": "text",
                "priority": 5,
                "sender": "user-789",
                "created_at": "2025-01-15T10:30:00Z",
                "is_read": false,
                "metadata": {}
            }
        ],
        "unread_count": 15,
        "total": 100
    },
    "meta": {
        "page": 1,
        "size": 20,
        "total_pages": 5
    }
}
```

### 3.4 数据一致性保证

- **事务性操作**：确保已读状态准确性
- **乐观锁处理**：并发已读确认冲突解决
- **冲突解决**：以服务端状态为准
- **版本控制**：基于时间戳的冲突检测

## 4. 消息发送系统 (核心工作接口)

### 4.1 消息发送场景分析

**4.1.1 系统自动发送**
- 企业网盘文件分享通知
- 文档协作邀请通知
- 系统告警和状态通知
- 工作流状态变更通知
- 定时任务和提醒通知

**4.1.2 服务程序发送**
- 微服务间消息通知
- 第三方系统集成通知
- API调用触发的消息
- 批量消息推送

**4.1.3 人工发送**
- 用户直接发送消息
- 管理员发布公告
- 用户分享文件/文档
- 客服回复消息

### 4.2 消息发送API设计

```go
// 消息发送请求格式
type SendMessageRequest struct {
    // 基本信息
    ChannelID     string            `json:"channel_id"`      // 频道ID
    Title         string            `json:"title"`           // 消息标题
    Content       string            `json:"content"`         // 消息内容
    MessageType   string            `json:"message_type"`    // 消息类型

    // 接收者
    Recipients    []string          `json:"recipients"`      // 接收用户ID列表
    RecipientType string            `json:"recipient_type"`  // user|channel|role|all

    // 发送者信息
    Sender        *SenderInfo       `json:"sender,omitempty"`      // 发送者信息
    OnBehalfOf    string            `json:"on_behalf_of,omitempty"` // 代理发送

    // 优先级和调度
    Priority      int               `json:"priority"`        // 优先级 1-10
    ScheduledAt   *time.Time        `json:"scheduled_at,omitempty"` // 定时发送
    ExpireAt      *time.Time        `json:"expire_at,omitempty"`    // 过期时间

    // 消息元数据
    Metadata      map[string]interface{} `json:"metadata,omitempty"`

    // 附件和操作
    Attachments   []AttachmentInfo  `json:"attachments,omitempty"`   // 附件信息
    Actions       []ActionInfo      `json:"actions,omitempty"`       // 操作按钮
}

type SenderInfo struct {
    ID        string `json:"id"`         // 发送者ID
    Type      string `json:"type"`       // user|service|system
    Name      string `json:"name"`       // 发送者名称
    Avatar    string `json:"avatar"`     // 头像URL
    ServiceID string `json:"service_id,omitempty"` // 服务ID (服务发送时)
}

type AttachmentInfo struct {
    Type        string `json:"type"`         // file|image|link|document
    Name        string `json:"name"`         // 附件名称
    URL         string `json:"url"`          // 下载链接
    Size        int64  `json:"size"`         // 文件大小
    MimeType    string `json:"mime_type"`    // MIME类型
    Thumbnail   string `json:"thumbnail"`    // 缩略图
}

type ActionInfo struct {
    Type    string `json:"type"`        // button|link|confirm
    Text    string `json:"text"`        // 按钮文本
    URL     string `json:"url"`         // 链接地址
    Action  string `json:"action"`      // 回调动作
    Style   string `json:"style"`       // primary|secondary|danger
}
```

### 4.3 服务认证和权限机制

```go
// 服务认证管理器
type ServiceAuthManager struct {
    services    map[string]*ServiceInfo    // 注册的服务列表
    apiKeys     map[string]string         // API Key映射
    permissions map[string][]string       // 服务权限
}

type ServiceInfo struct {
    ID          string            `json:"id"`
    Name        string            `json:"name"`
    Type        string            `json:"type"`        // internal|external|system
    ApiKey      string            `json:"api_key"`
    Permissions []string          `json:"permissions"` // 发送权限范围
    RateLimit   *RateLimit        `json:"rate_limit"`  // 频率限制
    Owner       string            `json:"owner"`       // 服务所有者
    CreatedAt   time.Time         `json:"created_at"`
    Status      string            `json:"status"`      // active|disabled|suspended
}

// 认证中间件
func ServiceAuthMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        authHeader := c.GetHeader("Authorization")
        if authHeader == "" {
            c.JSON(401, gin.H{"error": "Authorization header required"})
            c.Abort()
            return
        }

        // 支持多种认证方式
        if strings.HasPrefix(authHeader, "Bearer ") {
            // JWT Token认证 (用户发送)
            token := strings.TrimPrefix(authHeader, "Bearer ")
            if !validateUserJWT(token) {
                c.JSON(401, gin.H{"error": "Invalid user token"})
                c.Abort()
                return
            }
        } else if strings.HasPrefix(authHeader, "Service ") {
            // Service API Key认证 (服务发送)
            apiKey := strings.TrimPrefix(authHeader, "Service ")
            service := validateServiceAPIKey(apiKey)
            if service == nil {
                c.JSON(401, gin.H{"error": "Invalid service API key"})
                c.Abort()
                return
            }
            c.Set("service", service)
        } else {
            c.JSON(401, gin.H{"error": "Unsupported auth type"})
            c.Abort()
            return
        }

        c.Next()
    }
}
```

### 4.4 消息模板系统

```go
// 消息模板系统
type MessageTemplate struct {
    ID          string                 `json:"id"`
    Name        string                 `json:"name"`
    Category    string                 `json:"category"`     // system|business|notification
    Title       string                 `json:"title"`        // 模板标题 (支持变量)
    Content     string                 `json:"content"`      // 模板内容 (支持变量)
    Variables   []TemplateVariable     `json:"variables"`    // 模板变量定义
    MessageType string                 `json:"message_type"` // 消息类型
    Priority    int                    `json:"priority"`     // 默认优先级
    Actions     []ActionInfo           `json:"actions"`      // 默认操作按钮
    CreatedBy   string                 `json:"created_by"`
    CreatedAt   time.Time              `json:"created_at"`
    UpdatedAt   time.Time              `json:"updated_at"`
}

type TemplateVariable struct {
    Name        string `json:"name"`         // 变量名
    Type        string `json:"type"`         // string|number|date|url
    Required    bool   `json:"required"`     // 是否必填
    Default     string `json:"default"`      // 默认值
    Description string `json:"description"`  // 变量描述
}
```

### 4.5 常用消息模板

```go
// 文件分享通知模板
{
    "id": "file_share",
    "name": "文件分享通知",
    "category": "business",
    "title": "{{sender_name}} 分享了文件给你",
    "content": "{{sender_name}} 分享了文件 \"{{file_name}}\" 给你\n\n文件大小：{{file_size}}\n分享时间：{{share_time}}",
    "variables": [
        {"name": "sender_name", "type": "string", "required": true, "description": "发送者姓名"},
        {"name": "file_name", "type": "string", "required": true, "description": "文件名称"},
        {"name": "file_size", "type": "string", "required": true, "description": "文件大小"},
        {"name": "share_time", "type": "date", "required": true, "description": "分享时间"}
    ],
    "message_type": "file_share",
    "actions": [
        {"type": "link", "text": "查看文件", "url": "{{file_url}}", "style": "primary"}
    ]
}

// 文档协作邀请模板
{
    "id": "doc_collaboration",
    "name": "文档协作邀请",
    "category": "business",
    "title": "{{sender_name}} 邀请你协作编辑文档",
    "content": "{{sender_name}} 邀请你协作编辑文档 \"{{doc_title}}\"\n\n协作权限：{{permission}}\n截止时间：{{due_time}}",
    "variables": [
        {"name": "sender_name", "type": "string", "required": true},
        {"name": "doc_title", "type": "string", "required": true},
        {"name": "permission", "type": "string", "required": true},
        {"name": "due_time", "type": "date", "required": false}
    ],
    "message_type": "collaboration",
    "actions": [
        {"type": "link", "text": "打开文档", "url": "{{doc_url}}", "style": "primary"},
        {"type": "confirm", "text": "拒绝邀请", "action": "decline", "style": "secondary"}
    ]
}

// 系统告警模板
{
    "id": "system_alert",
    "name": "系统告警",
    "category": "system",
    "title": "系统告警：{{alert_type}}",
    "content": "告警详情：{{alert_message}}\n影响范围：{{affected_scope}}\n处理建议：{{suggestion}}",
    "variables": [
        {"name": "alert_type", "type": "string", "required": true},
        {"name": "alert_message", "type": "string", "required": true},
        {"name": "affected_scope", "type": "string", "required": true},
        {"name": "suggestion", "type": "string", "required": false}
    ],
    "message_type": "alert",
    "priority": 8
}
```

### 4.6 API使用示例

**4.6.1 企业网盘文件分享**
```bash
# 企业网盘服务调用消息发送API
POST /api/v3/messages/send
Authorization: Service netdrive_service_api_key
Content-Type: application/json

{
    "template_id": "file_share",
    "recipients": ["user123"],
    "on_behalf_of": "user456",  // 实际分享者
    "variables": {
        "sender_name": "张三",
        "file_name": "项目计划.pdf",
        "file_size": "2.5MB",
        "share_time": "2025-01-15 14:30:00",
        "file_url": "https://drive.company.com/files/abc123"
    }
}
```

**4.6.2 文档协作邀请**
```bash
# 文档系统调用消息发送API
POST /api/v3/messages/send
Authorization: Service docs_system_api_key
Content-Type: application/json

{
    "template_id": "doc_collaboration",
    "recipients": ["user789"],
    "on_behalf_of": "user456",
    "variables": {
        "sender_name": "李四",
        "doc_title": "Q1销售报告",
        "permission": "编辑",
        "due_time": "2025-01-20 18:00:00",
        "doc_url": "https://docs.company.com/doc/def456"
    }
}
```

**4.6.3 系统告警通知**
```bash
# 监控系统调用消息发送API
POST /api/v3/messages/send
Authorization: Service monitoring_api_key
Content-Type: application/json

{
    "template_id": "system_alert",
    "recipients": ["admin_team"],
    "variables": {
        "alert_type": "数据库连接异常",
        "alert_message": "主数据库连接池耗尽，当前活跃连接数：100/100",
        "affected_scope": "用户认证、订单查询",
        "suggestion": "立即检查数据库连接配置，考虑重启数据库服务"
    }
}
```

### 4.7 消息发送流程

```
外部系统/用户 → 发送API → 认证验证 → 权限检查 → 消息验证 → 投递队列 → 写入SQLite → WebSocket推送
```

### 4.8 发送权限控制

**4.8.1 服务权限分级**
- **系统级服务**: 可发送任何类型的消息，包括系统告警
- **业务级服务**: 只能发送业务相关消息，如文件分享、协作邀请
- **第三方服务**: 需要审核授权，限制发送频率和范围

**4.8.2 用户权限控制**
- **普通用户**: 只能发送个人消息
- **部门管理员**: 可向部门成员发送通知
- **系统管理员**: 可发送系统公告和告警

## 5. SQLite数据库备份功能

### 5.1 备份策略设计

```go
type BackupManager struct {
    storagePath  string             // 用户存储根路径
    compression  bool               // 是否压缩
    encryption   bool               // 是否加密
    retention    int                // 备份保留天数
}

// 备份类型
type BackupType int
const (
    FullBackup BackupType = iota    // 完整备份 (messages.db + read_status.db)
    MessagesOnly                   // 仅消息备份
    Incremental                    // 增量备份 (基于WAL文件)
)

type BackupRequest struct {
    UserID      string      `json:"user_id"`
    BackupType  BackupType  `json:"backup_type"`
    Compress    bool        `json:"compress"`
    Encrypt     bool        `json:"encrypt"`
    Since       time.Time   `json:"since,omitempty"`    // 增量备份的起始时间
}
```

### 5.2 备份实现流程

```
用户请求 → 权限验证 → 数据库锁定 → 文件复制 → 压缩加密 → 流式下载 → 解除锁定
```

### 5.3 备份API实现

```go
func (h *BackupHandler) DownloadDatabase(c *gin.Context) {
    userID := c.GetString("user_id")
    backupType := c.DefaultQuery("type", "full")
    compress := c.DefaultQuery("compress", "true") == "true"

    // 设置响应头
    c.Header("Content-Type", "application/octet-stream")
    c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=backup_%s.db", userID))

    if compress {
        c.Header("Content-Encoding", "gzip")
    }

    // 流式传输备份文件
    h.backupManager.StreamBackup(c.Writer, userID, backupType, compress)
}
```

### 5.4 性能优化措施

- **读取优化**：使用共享锁，允许并发读
- **流式传输**：避免大文件内存占用
- **断点续传**：支持大文件下载恢复
- **CDN缓存**：备份文件CDN分发
- **异步处理**：大文件备份异步生成

## 6. OAuth2认证集成方案

### 6.1 认证架构设计

```go
type AuthManager struct {
    providers    map[string]OAuthProvider  // 支持的OAuth2提供商
    jwtSecret    string                    // JWT密钥
    tokenStore   TokenStore               // Token存储
    sessionMgr   SessionManager           // 会话管理
}

// OAuth2提供商接口
type OAuthProvider interface {
    GetAuthURL(state string) string
    ExchangeCode(code string) (*TokenInfo, error)
    GetUserInfo(token string) (*UserInfo, error)
    ValidateToken(token string) (*TokenInfo, error)
}

type TokenInfo struct {
    AccessToken  string    `json:"access_token"`
    RefreshToken string    `json:"refresh_token,omitempty"`
    TokenType    string    `json:"token_type"`
    ExpiresIn    int       `json:"expires_in"`
    Scope        string    `json:"scope,omitempty"`
}

type UserInfo struct {
    ID          string            `json:"id"`
    Username    string            `json:"username"`
    Email       string            `json:"email"`
    Name        string            `json:"name"`
    Avatar      string            `json:"avatar,omitempty"`
    Provider    string            `json:"provider"`
    Attributes  map[string]string `json:"attributes,omitempty"`
}
```

### 6.2 支持的认证方式

- **GitHub OAuth** - 开发者用户
- **Google OAuth** - 企业用户
- **微信开放平台** - 个人用户
- **企业微信/钉钉** - 企业集成
- **自定义OAuth2** - 企业内部系统

### 6.3 认证流程

```
用户登录 → OAuth提供商授权 → 回调获取code → 换取access_token → 获取用户信息 → JWT生成 → 客户端存储
```

### 6.4 OAuth2 API设计

```go
// 认证相关API
GET    /api/v3/auth/providers           // 获取支持的认证提供商列表
GET    /api/v3/auth/{provider}/login    // 获取OAuth2登录URL
GET    /api/v3/auth/{provider}/callback // OAuth2回调处理
POST   /api/v3/auth/refresh            // 刷新JWT Token
POST   /api/v3/auth/logout             // 退出登录

// 用户信息API
GET    /api/v3/auth/profile            // 获取当前用户信息
PUT    /api/v3/auth/profile            // 更新用户信息
```

### 6.5 JWT Token设计

```json
{
    "iss": "message-service",
    "sub": "user-uuid",
    "aud": "client-app",
    "exp": 1640995200,
    "iat": 1640908800,
    "jti": "token-uuid",
    "user_id": "12345",
    "username": "john.doe",
    "email": "john@example.com",
    "provider": "github",
    "scope": ["read", "write"],
    "device_id": "device-uuid",
    "session_id": "session-uuid"
}
```

### 6.6 Token管理策略

- **访问令牌**：有效期2小时，用于API调用
- **刷新令牌**：有效期30天，用于更新访问令牌
- **设备令牌**：每个设备独立，支持多端登录
- **会话管理**：记录所有活跃会话，支持强制下线

## 7. 完整的技术服务流程

### 7.1 用户服务启动流程

1. **服务初始化**
   - 加载配置文件和环境变量
   - 初始化数据库连接池
   - 启动连接管理器

2. **认证模块启动**
   - 注册所有支持的OAuth2提供商
   - 初始化JWT密钥和存储
   - 启动会话管理器

3. **WebSocket服务启动**
   - 开启实时通信端口
   - 初始化连接Hub
   - 启动心跳检测协程

4. **HTTP API服务启动**
   - 开启RESTful接口
   - 注册路由中间件
   - 启动监控指标收集

5. **连接Admin服务**
   - 建立服务间通信通道
   - 同步用户数据变化
   - 启动健康检查

### 7.2 客户端连接流程

1. **OAuth认证**
   - 用户选择认证提供商
   - 跳转到OAuth2授权页面
   - 授权成功后回调获取code

2. **JWT获取**
   - 使用code换取access_token
   - 获取用户信息并创建账户
   - 生成JWT访问令牌

3. **WebSocket连接**
   - 使用JWT建立WebSocket连接
   - 发送设备信息和能力
   - 加入相关频道的Hub

4. **初始同步**
   - 下载未读消息列表
   - 同步已读状态
   - 获取用户频道信息

5. **保持在线**
   - 定期发送心跳包
   - 处理服务器推送消息
   - 上报设备状态变化

### 7.3 消息投递通知流程

1. **Admin/业务系统** → 调用投递API
2. **投递系统** → 写入用户SQLite
3. **状态检查** → 查询用户在线连接
4. **WebSocket推送** → 实时通知所有在线设备
5. **客户端处理** → 显示通知并可选已读确认
6. **状态同步** → 广播状态变更到其他设备

### 7.4 多端同步机制

- **即时同步**：WebSocket推送状态变更
- **定期同步**：客户端轮询检查数据一致性
- **冲突解决**：以服务端时间为准，最后操作生效
- **离线处理**：设备上线后同步离线期间的变化

## 8. 关键技术考虑

### 8.1 性能考虑

- **连接池管理**：WebSocket连接复用，避免内存泄漏
- **数据库优化**：SQLite读写分离，使用WAL模式提高并发
- **流式处理**：备份文件流式传输，控制内存使用
- **无状态认证**：JWT无状态设计，减少数据库查询
- **缓存策略**：用户信息和会话数据缓存

### 8.2 安全考虑

- **认证安全**：OAuth2标准化认证流程
- **令牌管理**：JWT token定期轮换和安全存储
- **连接安全**：WebSocket连接鉴权和数据加密
- **访问控制**：备份文件访问权限验证
- **网络安全**：CORS策略配置和安全头设置

### 8.3 扩展性考虑

- **模块化设计**：可插拔的OAuth2提供商架构
- **水平扩展**：支持分布式部署的连接管理
- **灵活配置**：可配置的备份策略和数据保留
- **权限系统**：细粒度的权限控制和角色管理

### 8.4 可靠性保证

- **故障转移**：多实例部署和负载均衡
- **数据一致性**：事务操作和冲突检测
- **监控告警**：关键指标监控和异常告警
- **恢复机制**：自动重连和数据同步恢复

## 9. 部署和运维

### 9.1 环境配置

```bash
# 服务端口
USER_SERVER_PORT=8080
ADMIN_SERVER_PORT=8081

# 数据库配置
USER_STORAGE_PATH=/data/user_storage
DATABASE_BACKUP_PATH=/data/backups

# 认证配置
JWT_SECRET=your-secret-key
JWT_EXPIRE_TIME=2h
REFRESH_TOKEN_EXPIRE_TIME=720h

# OAuth2提供商配置
GITHUB_CLIENT_ID=your-github-client-id
GITHUB_CLIENT_SECRET=your-github-client-secret
GOOGLE_CLIENT_ID=your-google-client-id
GOOGLE_CLIENT_SECRET=your-google-client-secret

# WebSocket配置
WS_MAX_CONNECTIONS=10000
WS_PING_INTERVAL=30s
WS_READ_BUFFER_SIZE=4096
WS_WRITE_BUFFER_SIZE=4096
```

### 9.2 监控指标

- **连接指标**：在线用户数、WebSocket连接数
- **消息指标**：消息投递量、已读率、响应时间
- **性能指标**：内存使用、CPU使用、数据库性能
- **错误指标**：认证失败、连接错误、API错误率

### 9.3 日志记录

- **访问日志**：记录所有API请求和响应
- **业务日志**：记录用户操作和业务事件
- **错误日志**：记录异常和错误信息
- **安全日志**：记录认证和安全相关事件

这个用户服务架构与现有的admin服务形成了完整的消息通知系统，通过分离的端口和功能模块，为用户提供了高性能、高可用的实时消息服务。