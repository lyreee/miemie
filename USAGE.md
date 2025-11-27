# 消息接收系统使用指南

## 系统概述

这是一个基于Go语言开发的简单消息接收和实时通知系统，具有以下特点：

- ✅ **无认证设计** - 开箱即用，无需复杂配置
- ✅ **HTTP API** - 支持RESTful接口发送消息
- ✅ **WebSocket实时推送** - 消息实时推送给所有连接的客户端
- ✅ **SQLite存储** - 轻量级数据库，无需额外配置
- ✅ **支持频道和优先级** - 灵活的消息分类和管理
- ✅ **批量操作** - 支持批量发送消息

## 快速开始

### 1. 环境要求

- Go 1.21+
- 无需额外数据库（使用SQLite）

### 2. 启动服务

```bash
# 克隆或下载项目
# 进入项目目录
cd miemie

# 下载依赖
go mod tidy

# 启动服务
go run main.go
```

服务启动后会在8080端口监听。

### 3. 测试系统

打开浏览器访问：`http://localhost:8080/test.html`

## API使用方法

### 发送单条消息

```bash
curl -X POST http://localhost:8080/api/v3/messages \
  -H "Content-Type: application/json" \
  -d '{
    "title": "测试消息",
    "content": "这是一条测试消息",
    "sender": "test_user"
  }'
```

### 批量发送消息

```bash
curl -X POST http://localhost:8080/api/v3/messages/batch \
  -H "Content-Type: application/json" \
  -d '{
    "messages": [
      {
        "title": "消息1",
        "content": "内容1",
        "priority": 5
      },
      {
        "title": "消息2",
        "content": "内容2",
        "priority": 2
      }
    ]
  }'
```

### 获取消息列表

```bash
curl "http://localhost:8080/api/v3/messages?limit=10"
```

### 创建新频道

```bash
curl -X POST http://localhost:8080/api/v3/channels \
  -H "Content-Type: application/json" \
  -d '{
    "name": "项目通知",
    "description": "项目相关通知频道"
  }'
```

### 获取所有频道

```bash
curl "http://localhost:8080/api/v3/channels"
```

## WebSocket连接

### JavaScript客户端示例

```javascript
// 连接WebSocket
const ws = new WebSocket('ws://localhost:8080/ws');

ws.onopen = function() {
    console.log('WebSocket连接已建立');
};

ws.onmessage = function(event) {
    const data = JSON.parse(event.data);
    if (data.type === 'message') {
        console.log('收到新消息:', data.data);
        // 处理消息显示
        displayMessage(data.data);
    }
};

ws.onclose = function() {
    console.log('WebSocket连接已断开');
    // 可以在这里实现重连逻辑
};

function displayMessage(message) {
    console.log(`[${message.channel_id}] ${message.title}: ${message.content}`);
}
```

## 消息格式

### 发送消息参数

| 参数 | 类型 | 必需 | 默认值 | 说明 |
|------|------|------|--------|------|
| channel_id | string | 否 | "default" | 频道ID |
| title | string | 是 | - | 消息标题 |
| content | string | 是 | - | 消息内容 |
| message_type | string | 否 | "text" | 消息类型 |
| priority | int | 否 | 5 | 优先级(1-10，1最高) |
| sender | string | 否 | "anonymous" | 发送者 |

### 接收到的消息格式

```json
{
  "type": "message",
  "data": {
    "id": "uuid",
    "channel_id": "default",
    "title": "消息标题",
    "content": "消息内容",
    "message_type": "text",
    "priority": 5,
    "sender": "sender_name",
    "created_at": "2025-01-15T10:30:00Z",
    "updated_at": "2025-01-15T10:30:00Z",
    "metadata": {}
  }
}
```

## 常见使用场景

### 1. 系统监控告警

```bash
curl -X POST http://localhost:8080/api/v3/messages \
  -H "Content-Type: application/json" \
  -d '{
    "channel_id": "alerts",
    "title": "CPU使用率告警",
    "content": "服务器CPU使用率超过80%",
    "message_type": "alert",
    "priority": 2,
    "sender": "monitoring_system"
  }'
```

### 2. 应用通知

```bash
curl -X POST http://localhost:8080/api/v3/messages \
  -H "Content-Type: application/json" \
  -d '{
    "channel_id": "notifications",
    "title": "新订单通知",
    "content": "您有一个新的订单待处理",
    "message_type": "notification",
    "priority": 5,
    "sender": "order_system"
  }'
```

### 3. 日志信息推送

```bash
curl -X POST http://localhost:8080/api/v3/messages/batch \
  -H "Content-Type: application/json" \
  -d '{
    "messages": [
      {
        "channel_id": "logs",
        "title": "用户登录",
        "content": "用户user123从IP 192.168.1.100登录",
        "message_type": "system",
        "priority": 8,
        "sender": "auth_system"
      }
    ]
  }'
```

## 配置选项

### 环境变量配置

```bash
# 服务端口（默认8080）
export PORT=8080

# 数据库文件路径（默认./data/messages.db）
export DATABASE_PATH=/var/lib/miemie/messages.db

# 启动服务
go run main.go
```

### 数据库管理

数据库文件会自动创建，包含以下表：

- `messages` - 消息存储
- `channels` - 频道信息
- `read_status` - 已读状态（预留）

## 性能特性

- **并发支持** - 支持多个WebSocket客户端同时连接
- **消息广播** - 新消息会推送给所有连接的客户端
- **自动重连** - WebSocket连接断开时会自动重连
- **内存优化** - 限制内存中存储的消息数量
- **无阻塞** - 使用goroutine处理并发请求

## 故障排除

### 1. 端口被占用

```bash
# 检查端口占用
lsof -i :8080

# 使用其他端口
PORT=8081 go run main.go
```

### 2. 数据库权限问题

```bash
# 确保数据目录有写权限
mkdir -p ./data
chmod 755 ./data
```

### 3. WebSocket连接失败

- 检查防火墙设置
- 确认服务已正常启动
- 检查浏览器控制台错误信息

## 扩展建议

这个系统可以轻松扩展以支持：

1. **用户认证** - 添加JWT或OAuth2验证
2. **消息过滤** - 基于用户权限的消息路由
3. **持久连接** - Redis Pub/Sub支持
4. **消息持久化** - 支持MySQL、PostgreSQL
5. **API限流** - 防止消息发送过于频繁
6. **消息模板** - 支持模板变量替换
7. **文件附件** - 支持消息附件上传

## 许可证

MIT License