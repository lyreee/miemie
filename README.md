# miemie
message mie mie 咩~~咩~~ notification space server
一个简单的消息接收和实时通知系统，支持HTTP API发送消息和WebSocket实时接收。

![](miemie.png)


## 功能特性

- ✅ HTTP API 发送消息
- ✅ WebSocket 实时消息推送
- ✅ SQLite 数据库存储
- ✅ 支持频道和消息类型
- ✅ 消息优先级
- ✅ 批量消息发送
- ✅ 无需认证，开箱即用

## 快速开始

### 方法一：直接运行（开发模式）

```bash
# 1. 安装依赖
go mod tidy

# 2. 启动服务
go run main.go
```

### 方法二：编译运行（生产模式）

```bash
# 1. 使用编译脚本（推荐）
./build.sh

# 2. 启动服务
./start.sh
```

或手动编译：

```bash
# 1. 设置环境并编译
export GOROOT=/usr/local/go
export GOMODCACHE=/tmp/go-mod-cache
export GOCACHE=/tmp/go-build-cache
go mod tidy
go build -o miemie main.go

# 2. 启动服务
./miemie
```

服务将在 `http://localhost:8080` 启动。

### 3. 测试系统

打开浏览器访问 `http://localhost:8080/test.html` 进行测试。

## API 接口

### 发送单条消息

```bash
POST /api/v3/messages
Content-Type: application/json

{
  "channel_id": "default",
  "title": "消息标题",
  "content": "消息内容",
  "message_type": "text",
  "priority": 5,
  "sender": "sender_name"
}
```

### 批量发送消息

```bash
POST /api/v3/messages/batch
Content-Type: application/json

{
  "messages": [
    {
      "title": "消息1",
      "content": "内容1"
    },
    {
      "title": "消息2",
      "content": "内容2"
    }
  ]
}
```

### 获取消息列表

```bash
GET /api/v3/messages?channel_id=default&limit=20&offset=0
```

### 获取单条消息

```bash
GET /api/v3/messages/{message_id}
```

### 频道管理

```bash
# 获取所有频道
GET /api/v3/channels

# 创建频道
POST /api/v3/channels
Content-Type: application/json

{
  "name": "频道名称",
  "description": "频道描述"
}

# 获取单个频道
GET /api/v3/channels/{channel_id}
```

## WebSocket 连接

连接到 `ws://localhost:8080/ws` 接收实时消息推送。

### 消息格式

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
    "updated_at": "2025-01-15T10:30:00Z"
  }
}
```

## 数据库

系统使用 SQLite 数据库，数据库文件默认位置：`./data/messages.db`

### 表结构

#### messages 表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | TEXT | 消息唯一ID |
| channel_id | TEXT | 频道ID |
| title | TEXT | 消息标题 |
| content | TEXT | 消息内容 |
| message_type | TEXT | 消息类型 |
| priority | INTEGER | 优先级(1-10) |
| sender | TEXT | 发送者 |
| created_at | DATETIME | 创建时间 |
| updated_at | DATETIME | 更新时间 |
| metadata | TEXT | 元数据(JSON) |

#### channels 表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | TEXT | 频道唯一ID |
| name | TEXT | 频道名称 |
| description | TEXT | 频道描述 |
| created_by | TEXT | 创建者 |
| created_at | DATETIME | 创建时间 |
| last_message_at | DATETIME | 最后消息时间 |

#### read_status 表

| 字段 | 类型 | 说明 |
|------|------|------|
| message_id | TEXT | 消息ID |
| read_at | DATETIME | 已读时间 |
| read_device | TEXT | 阅读设备 |
| archived_at | DATETIME | 归档时间 |
| starred_at | DATETIME | 标记时间 |
| metadata | TEXT | 元数据(JSON) |

## 配置

### 环境变量

| 变量名 | 默认值 | 说明 |
|--------|--------|------|
| PORT | 8080 | 服务端口 |
| DATABASE_PATH | ./data/messages.db | 数据库文件路径 |

## 使用示例

### curl 发送消息

```bash
curl -X POST http://localhost:8080/api/v3/messages \
  -H "Content-Type: application/json" \
  -d '{
    "title": "测试消息",
    "content": "这是一条测试消息",
    "sender": "curl_tester"
  }'
```

### JavaScript 发送消息

```javascript
fetch('http://localhost:8080/api/v3/messages', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
  },
  body: JSON.stringify({
    title: 'Hello World',
    content: '来自JavaScript的消息',
    sender: 'js_client'
  })
})
.then(response => response.json())
.then(data => console.log('消息发送成功:', data));
```

### WebSocket 客户端

```javascript
const ws = new WebSocket('ws://localhost:8080/ws');

ws.onmessage = function(event) {
  const message = JSON.parse(event.data);
  if (message.type === 'message') {
    console.log('收到新消息:', message.data);
  }
};
```

## 项目结构

```
miemie/
├── main.go                     # 主程序入口
├── go.mod                      # Go模块文件
├── test.html                   # 测试页面
├── data/                       # 数据目录
│   └── messages.db             # SQLite数据库
├── internal/                   # 内部包
│   ├── api/                    # HTTP API处理
│   │   └── api.go
│   ├── config/                 # 配置管理
│   │   └── config.go
│   ├── database/               # 数据库连接
│   │   └── database.go
│   ├── models/                 # 数据模型
│   │   └── message.go
│   ├── storage/                # 数据存储
│   │   └── message.go
│   └── websocket/              # WebSocket处理
│       └── manager.go
└── dev/                        # 开发文档
    ├── ADMIN.md
    ├── DESIGN.md
    ├── PLAN.md
    ├── SERVER.md
    └── TEST.md
```

## 开发说明

### 扩展功能

系统设计为可扩展的，可以轻松添加以下功能：

1. **用户认证** - 在API中间件中添加JWT或OAuth2验证
2. **权限管理** - 基于角色的访问控制
3. **消息模板** - 支持模板消息和变量替换
4. **消息路由** - 基于规则的消息分发
5. **持久化** - 支持MySQL、PostgreSQL等其他数据库
6. **集群支持** - 多实例部署和负载均衡

### 性能优化建议

1. **数据库索引** - 根据查询模式添加合适的索引
2. **连接池** - 使用数据库连接池管理连接
3. **缓存** - 添加Redis缓存热门数据
4. **消息队列** - 使用消息队列处理高并发写入

## 许可证

MIT License

## 联系方式

如有问题或建议，请提交Issue或Pull Request。