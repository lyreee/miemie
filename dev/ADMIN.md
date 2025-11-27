# 消息通知系统管理后台设计文档

## 1. 概述

### 1.1 设计目标
- 提供完整的管理后台系统，用于管理消息通知服务的各个方面
- 基于WebComponent技术栈，提供可复用的UI组件
- 支持多页面应用架构，提供统一的用户体验
- 面向管理员和技术运维人员，普通用户不访问此系统

### 1.2 技术栈
- **前端**: 原生JavaScript + WebComponent + Tailwind CSS
- **后端**: Go语言 + Gin框架
- **样式**: Tailwind CSS + 自定义样式
- **组件**: WebComponent自定义元素
- **构建**: 无需构建工具，直接运行

### 1.3 核心特性
- 🎨 统一的UI组件系统
- 📱 响应式设计，支持移动端
- 🔐 完善的权限管理
- 🌐 WebComponent技术栈
- 📊 实时数据展示
- ⚡ 高性能页面加载

## 2. 系统架构

### 2.1 整体架构

```
┌─────────────────────────────────────────────────────────┐
│                    浏览器端                              │
├─────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │
│  │ admin-topbar│  │ admin-toast │  │admin-dialog │     │
│  │  (顶部导航)  │  │ (消息通知)  │  │ (对话框)    │     │
│  └─────────────┘  └─────────────┘  └─────────────┘     │
│                                                           │
│  ┌─────────────────────────────────────────────────────┐ │
│  │                  页面容器                             │ │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐ │ │
│  │  │  Dashboard  │  │  Users      │  │ Channels    │ │ │
│  │  │  (仪表盘)   │  │ (用户管理)  │  │ (频道管理)  │ │ │
│  │  └─────────────┘  └─────────────┘  └─────────────┘ │ │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐ │ │
│  │  │  Senders    │  │ Permissions│  │ Messages    │ │ │
│  │  │ (发送者管理) │  │ (权限管理)  │  │ (消息管理)  │ │ │
│  │  └─────────────┘  └─────────────┘  └─────────────┘ │ │
│  └─────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────┘
                                │
                                │ HTTP/REST API
                                ▼
┌─────────────────────────────────────────────────────────┐
│                      Go后端服务                           │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │
│  │   认证服务   │  │  管理API    │  │  数据库      │     │
│  └─────────────┘  └─────────────┘  └─────────────┘     │
└─────────────────────────────────────────────────────────┘
```

### 2.2 目录结构

```
admin-frontend/
├── index.html                    # 主入口页面
├── login.html                    # 登录页面
├── css/
│   ├── tailwind.min.css         # Tailwind CSS框架
│   └── admin.css                # 自定义样式
├── js/
│   ├── config.js                # 配置文件
│   ├── app.js                   # 应用入口
│   ├── router.js                # 路由管理
│   ├── api.js                   # API请求封装
│   ├── auth.js                  # 认证管理
│   ├── utils.js                 # 工具函数
│   ├── security.js              # 安全相关
│   ├── components/              # 传统组件
│   │   ├── table.js             # 数据表格
│   │   ├── chart.js             # 图表组件
│   │   └── form.js              # 表单组件
│   ├── components/              # WebComponent组件
│   │   ├── base-component.js    # 基础组件类
│   │   ├── admin-topbar.js      # 顶部导航栏
│   │   ├── admin-toast.js       # 消息通知
│   │   └── admin-dialog.js      # 对话框
│   ├── pages/                   # 页面模块
│   │   ├── base-page.js         # 基础页面类
│   │   ├── dashboard.js         # 仪表盘
│   │   ├── users.js             # 用户管理
│   │   ├── channels.js          # 频道管理
│   │   ├── senders.js           # 发送者管理
│   │   ├── permissions.js       # 权限管理
│   │   ├── messages.js          # 消息管理
│   │   └── settings.js          # 系统设置
│   └── component-manager.js     # 组件管理器
└── assets/
    ├── icons/                   # 图标资源
    └── images/                  # 图片资源
```

## 3. WebComponent组件系统

### 3.1 基础组件类

```javascript
// js/components/webcomponents/base-component.js
class BaseComponent extends HTMLElement {
    constructor() {
        super()
        this.attachShadow({ mode: 'open' })
    }

    // 加载样式
    loadStyles() {
        const style = document.createElement('style')
        style.textContent = this.getStyles()
        this.shadowRoot.appendChild(style)
    }

    // 事件代理
    delegateEvent(selector, event, handler) {
        this.shadowRoot.addEventListener(event, (e) => {
            if (e.target.matches(selector)) {
                handler(e)
            }
        })
    }

    // 触发自定义事件
    emitEvent(eventName, detail = {}) {
        this.dispatchEvent(new CustomEvent(eventName, {
            bubbles: true,
            detail
        }))
    }
}
```

### 3.2 admin-topbar 顶部导航栏

**功能特性:**
- 显示系统logo和标题
- 提供主导航菜单
- 用户信息显示和菜单
- 通知铃铛和消息提醒
- 响应式设计

**使用方法:**
```html
<admin-topbar id="topbar"></admin-topbar>
```

**API接口:**
```javascript
// 设置通知徽章
topbar.setNotificationBadge(count)

// 显示通知消息
topbar.showNotification(message, type)

// 监听导航事件
topbar.addEventListener('navigate', (e) => {
    console.log('导航到:', e.detail.page)
})
```

**事件:**
- `navigate` - 导航事件
- `logout` - 退出登录
- `show-notifications` - 显示通知列表

### 3.3 admin-toast 消息通知系统

**功能特性:**
- 四种消息类型: success, error, warning, info
- 自动消失和手动关闭
- 消息队列管理
- 动画效果
- 响应式布局

**使用方法:**
```javascript
// 全局事件触发
window.dispatchEvent(new CustomEvent('admin-toast', {
    detail: {
        message: '操作成功',
        type: 'success',
        duration: 3000
    }
}))

// 便捷函数
showSuccess('操作成功')
showError('操作失败')
showWarning('注意')
showInfo('提示信息')
```

**配置选项:**
```javascript
{
    message: string,     // 消息内容
    type: 'success'|'error'|'warning'|'info', // 消息类型
    duration: number,    // 显示时长(ms)，0表示不自动消失
    title: string        // 自定义标题
}
```

### 3.4 admin-dialog 对话框组件

**功能特性:**
- 多种尺寸: small, medium, large, fullscreen
- 自定义内容和按钮
- 背景点击关闭
- ESC键关闭
- 静态便捷方法

**使用方法:**
```html
<admin-dialog id="dialog" title="确认操作" size="small">
    <p>确定要执行此操作吗？</p>
    <button slot="footer" onclick="dialog.close('confirm')">确定</button>
</admin-dialog>

<script>
// 编程方式使用
const dialog = document.getElementById('dialog')
await dialog.open()
</script>
```

**静态便捷方法:**
```javascript
// 警告框
await showAlert('操作完成')

// 确认框
const result = await showConfirm('确定删除吗？')
if (result === 'confirm') {
    // 用户点击确认
}

// 输入框
const input = await showPrompt('请输入名称', '默认值')
if (input) {
    console.log('用户输入:', input)
}
```

**属性配置:**
```javascript
{
    title: string,        // 对话框标题
    size: 'small'|'medium'|'large'|'fullscreen', // 尺寸
    showClose: boolean,   // 是否显示关闭按钮
    backdrop: boolean,    // 是否显示背景遮罩
    content: string|HTMLElement, // 内容
    footer: string|HTMLElement  // 底部按钮
}
```

## 4. 页面系统

### 4.1 基础页面类

```javascript
// js/pages/base-page.js
class BasePage {
    constructor() {
        this.container = null
        this.topBar = null
        this.toast = null
    }

    async init() {
        await this.setupLayout()
        await this.setupComponents()
        await this.render()
        await this.bindEvents()
        this.onPageLoad()
    }

    // 抽象方法，子类必须实现
    async render() {
        throw new Error('render method must be implemented')
    }

    // 便捷方法
    showSuccess(message) { /* ... */ }
    showError(message) { /* ... */ }
    showLoading() { /* ... */ }
    // ... 其他便捷方法
}
```

### 4.2 页面实现示例

**仪表盘页面 (dashboard.js):**
```javascript
class DashboardPage extends BasePage {
    async render() {
        this.setPageTitle('仪表盘')
        this.showLoading()

        try {
            await this.loadDashboardData()
            const content = document.getElementById('page-content')
            content.innerHTML = this.getDashboardHTML()
            await this.renderCharts()
        } catch (error) {
            this.showErrorState('加载数据失败')
        }
    }

    getDashboardHTML() {
        return `
            <div class="page-header mb-6">
                <h1 class="text-2xl font-bold text-gray-900">仪表盘</h1>
            </div>

            <!-- 统计卡片 -->
            <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
                ${this.renderStatCards()}
            </div>

            <!-- 图表区域 -->
            <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
                <div class="bg-white p-6 rounded-lg shadow">
                    <h3 class="text-lg font-semibold mb-4">消息投递趋势</h3>
                    <div id="delivery-chart" class="h-64"></div>
                </div>
            </div>
        `
    }
}
```

### 4.3 组件管理器

```javascript
// js/component-manager.js
class ComponentManager {
    constructor() {
        this.components = new Map()
    }

    async init() {
        await this.registerComponents()
        this.setupGlobalUtils()
    }

    createPage(pageClass) {
        const page = new pageClass()
        this.components.set(pageClass.name, page)
        return page
    }

    setupGlobalUtils() {
        // 全局便捷函数
        window.showToast = (message, type = 'info') => {
            window.dispatchEvent(new CustomEvent('admin-toast', {
                detail: { message, type }
            }))
        }

        window.showAlert = (message, options) =>
            window.adminDialog.alert(message, options)
    }
}
```

## 5. 权限管理

### 5.1 权限模型

```javascript
// 权限格式: resource:action
const permissions = [
    'dashboard:view',      // 查看仪表盘
    'users:view',          // 查看用户列表
    'users:create',        // 创建用户
    'users:edit',          // 编辑用户
    'users:delete',        // 删除用户
    'channels:view',       // 查看频道
    'channels:create',     // 创建频道
    'channels:edit',       // 编辑频道
    'channels:delete',     // 删除频道
    'messages:view',       // 查看消息
    'messages:send',       // 发送消息
    'messages:delete',     // 删除消息
    'settings:view',       // 查看设置
    'settings:edit'        // 编辑设置
]
```

### 5.2 权限控制实现

```javascript
// js/auth.js
class AuthManager {
    hasPermission(permission) {
        return this.permissions.includes(permission) ||
               this.permissions.includes('*')
    }

    canAccess(resource, action) {
        const permission = `${resource}:${action}`
        return this.hasPermission(permission) ||
               this.hasPermission(`${resource}:*`)
    }

    // 路由守卫
    requireAuth(permission = null) {
        if (!this.user) {
            window.location.href = '/login.html'
            return false
        }

        if (permission && !this.hasPermission(permission)) {
            this.showAccessDenied()
            return false
        }

        return true
    }
}
```

### 5.3 前端权限指令

```html
<!-- 基于权限控制元素显示 -->
<button data-permission="users:create">创建用户</button>
<div data-role="admin">管理员专用内容</div>

<script>
// 权限检查
document.querySelectorAll('[data-permission]').forEach(element => {
    const permission = element.dataset.permission
    if (!auth.hasPermission(permission)) {
        element.style.display = 'none'
    }
})
</script>
```

## 6. 发送者管理系统

### 6.1 发送者账户类型

**6.1.1 账户类型定义**
- **普通用户账户** (User): 通过OAuth2登录的个人用户，主要用于接收消息
- **服务程序账户** (Service): 企业内部服务或第三方应用的系统账户
- **管理员账户** (Admin): 具有管理权限的系统管理员

**6.1.2 权限矩阵**

| 账户类型 | 接收消息 | 发送消息 | 批量发送 | 使用模板 | 代理发送 | 创建频道 | 系统告警 |
|----------|----------|----------|----------|----------|----------|----------|----------|
| 普通用户 | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| 服务程序 | ❌ | ✅ | ✅ | ✅ | ✅ | ⚠️ | ⚠️ |
| 管理员 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |

*⚠️ 表示需要特殊权限授权*

### 6.2 发送者管理界面

**6.2.1 发送者列表页面**

```javascript
// js/pages/senders.js
class SendersManagementPage extends BasePage {
    async render() {
        this.setPageTitle('发送者管理')

        const content = document.getElementById('page-content')
        content.innerHTML = `
            <div class="page-header mb-6">
                <h1 class="text-2xl font-bold text-gray-900">发送者管理</h1>
                <p class="text-gray-600 mt-1">管理用户账户、服务程序账户和发送权限</p>
            </div>

            <!-- 操作栏 -->
            <div class="flex justify-between items-center mb-6">
                <div class="flex space-x-4">
                    <select id="senderTypeFilter" class="form-select">
                        <option value="all">全部类型</option>
                        <option value="user">普通用户</option>
                        <option value="service">服务程序</option>
                        <option value="admin">管理员</option>
                    </select>
                    <input type="text" id="searchInput" placeholder="搜索发送者..."
                           class="form-input w-64">
                </div>
                <button class="btn btn-primary" onclick="showCreateServiceSender()">
                    <i class="fas fa-plus mr-2"></i>创建服务账户
                </button>
            </div>

            <!-- 发送者列表 -->
            <div class="bg-white rounded-lg shadow overflow-hidden">
                <table class="min-w-full divide-y divide-gray-200">
                    <thead class="bg-gray-50">
                        <tr>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                                发送者信息
                            </th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                                类型
                            </th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                                权限范围
                            </th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                                状态
                            </th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                                发送统计
                            </th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                                操作
                            </th>
                        </tr>
                    </thead>
                    <tbody class="bg-white divide-y divide-gray-200" id="sendersTableBody">
                        <!-- 数据行动态生成 -->
                    </tbody>
                </table>
            </div>
        `

        await this.loadSendersData()
    }
}
```

**6.2.2 创建服务账户**

```javascript
function showCreateServiceSender() {
    const dialog = document.getElementById('dialog')
    dialog.innerHTML = `
        <div class="p-6">
            <h3 class="text-lg font-medium text-gray-900 mb-4">创建服务账户</h3>
            <form id="createServiceForm">
                <div class="grid grid-cols-2 gap-4 mb-4">
                    <div>
                        <label class="block text-sm font-medium text-gray-700 mb-1">服务名称</label>
                        <input type="text" name="name" required class="form-input w-full">
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 mb-1">服务类型</label>
                        <select name="service_type" required class="form-select w-full">
                            <option value="internal">内部服务</option>
                            <option value="external">第三方服务</option>
                            <option value="system">系统服务</option>
                        </select>
                    </div>
                </div>

                <div class="mb-4">
                    <label class="block text-sm font-medium text-gray-700 mb-1">发送权限</label>
                    <div class="space-y-2">
                        ${this.renderPermissionCheckboxes()}
                    </div>
                </div>

                <div class="grid grid-cols-2 gap-4 mb-4">
                    <div>
                        <label class="block text-sm font-medium text-gray-700 mb-1">频率限制</label>
                        <input type="number" name="rate_limit" value="100"
                               class="form-input w-full" placeholder="每小时最大发送数">
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 mb-1">负责人</label>
                        <input type="text" name="owner" required class="form-input w-full">
                    </div>
                </div>
            </form>
        </div>
    `

    dialog.title = '创建服务账户'
    dialog.size = 'large'
    dialog.open()
}
```

### 6.3 频道管理扩展

**6.3.1 频道类型定义**

```go
type Channel struct {
    ID              string                 `json:"id"`
    Name            string                 `json:"name"`
    Type            string                 `json:"type"` // public|private|system|department|project
    Description     string                 `json:"description"`
    OwnerID         string                 `json:"owner_id"`
    PublishPolicy   string                 `json:"publish_policy"` // owner|members|public
    JoinPolicy      string                 `json:"join_policy"` // open|approval|invite
    MemberCount     int                    `json:"member_count"`
    MessageCount    int                    `json:"message_count"`
    CreatedAt       time.Time              `json:"created_at"`
    UpdatedAt       time.Time              `json:"updated_at"`
    Metadata        map[string]interface{} `json:"metadata,omitempty"`
}
```

**6.3.2 频道管理界面**

```javascript
class ChannelsManagementPage extends BasePage {
    async render() {
        this.setPageTitle('频道管理')

        const content = document.getElementById('page-content')
        content.innerHTML = `
            <div class="page-header mb-6">
                <h1 class="text-2xl font-bold text-gray-900">频道管理</h1>
                <p class="text-gray-600 mt-1">管理消息频道、订阅关系和发布权限</p>
            </div>

            <!-- 频道网格 -->
            <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6" id="channelsGrid">
                <!-- 频道卡片动态生成 -->
            </div>
        `

        await this.loadChannelsData()
    }
}
```

## 7. 权限管理和审核系统

### 7.1 权限模型设计

**7.1.1 角色定义**
- **super_admin**: 超级管理员，拥有所有权限
- **admin**: 普通管理员，拥有管理权限
- **service_manager**: 服务管理员，管理服务账户
- **channel_manager**: 频道管理员，管理频道
- **user_manager**: 用户管理员，管理普通用户

**7.1.2 权限定义**

```go
const permissions = [
    // 用户管理
    "users:view", "users:create", "users:edit", "users:delete",

    // 服务账户管理
    "services:view", "services:create", "services:edit", "services:delete",
    "services:approve", "services:revoke_key",

    // 频道管理
    "channels:view", "channels:create", "channels:edit", "channels:delete",
    "channels:manage_members", "channels:set_permissions",

    // 消息管理
    "messages:view", "messages:send", "messages:delete", "messages:moderate",

    // 权限管理
    "permissions:view", "permissions:edit", "permissions:assign",

    // 系统管理
    "system:settings", "system:stats", "system:logs", "system:alerts"
]
```

### 7.2 服务账户审核流程

**7.2.1 申请流程**

```javascript
// 申请服务账户
function showServiceApplicationDialog() {
    const dialog = document.getElementById('dialog')
    dialog.innerHTML = `
        <div class="p-6">
            <h3 class="text-lg font-medium text-gray-900 mb-4">申请服务账户</h3>
            <form id="serviceApplicationForm">
                <div class="grid grid-cols-2 gap-4 mb-4">
                    <div>
                        <label class="block text-sm font-medium text-gray-700 mb-1">服务名称 *</label>
                        <input type="text" name="service_name" required class="form-input w-full">
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 mb-1">申请部门 *</label>
                        <input type="text" name="department" required class="form-input w-full">
                    </div>
                </div>

                <div class="mb-4">
                    <label class="block text-sm font-medium text-gray-700 mb-1">申请理由 *</label>
                    <textarea name="reason" rows="4" required class="form-input w-full"
                              placeholder="请详细说明申请服务账户的用途和业务场景"></textarea>
                </div>

                <div class="mb-4">
                    <label class="block text-sm font-medium text-gray-700 mb-1">所需权限 *</label>
                    <div class="space-y-2">
                        ${this.renderApplicationPermissionCheckboxes()}
                    </div>
                </div>
            </form>
        </div>
    `

    dialog.title = '申请服务账户'
    dialog.size = 'large'
    dialog.open()
}
```

**7.2.2 审核界面**

```javascript
function reviewServiceApplication(applicationId, action) {
    const dialog = document.getElementById('dialog')
    dialog.innerHTML = `
        <div class="p-6">
            <h3 class="text-lg font-medium text-gray-900 mb-4">审核服务账户申请</h3>

            <div id="applicationDetails">
                <!-- 申请详情动态加载 -->
            </div>

            ${action === 'approve' ? `
                <div class="mt-6 p-4 bg-gray-50 rounded-lg">
                    <h4 class="text-sm font-medium text-gray-900 mb-3">配置服务账户</h4>
                    <div class="grid grid-cols-2 gap-4">
                        <div>
                            <label class="block text-sm font-medium text-gray-700 mb-1">频率限制</label>
                            <input type="number" name="rate_limit" class="form-input w-full" value="100">
                        </div>
                        <div>
                            <label class="block text-sm font-medium text-gray-700 mb-1">有效期</label>
                            <select name="expiry" class="form-select w-full">
                                <option value="30">30天</option>
                                <option value="90">90天</option>
                                <option value="365">1年</option>
                                <option value="0">永久</option>
                            </select>
                        </div>
                    </div>
                </div>
            ` : `
                <div class="mt-6">
                    <label class="block text-sm font-medium text-gray-700 mb-1">拒绝理由 *</label>
                    <textarea name="rejection_reason" rows="3" required class="form-input w-full"></textarea>
                </div>
            `}

            <div class="mt-6 flex justify-end space-x-3">
                <button type="button" class="btn btn-secondary" onclick="dialog.close()">取消</button>
                <button type="button" class="btn ${action === 'approve' ? 'btn-primary' : 'btn-danger'}"
                        onclick="submitApplicationReview('${applicationId}', '${action}')">
                    ${action === 'approve' ? '批准申请' : '拒绝申请'}
                </button>
            </div>
        </div>
    `

    dialog.title = '审核申请'
    dialog.size = 'large'
    dialog.open()
}
```

### 7.3 权限管理界面

```javascript
class PermissionsManagementPage extends BasePage {
    async render() {
        this.setPageTitle('权限管理')

        const content = document.getElementById('page-content')
        content.innerHTML = `
            <!-- 权限概览卡片 -->
            <div class="grid grid-cols-1 md:grid-cols-4 gap-6 mb-8">
                <div class="bg-white rounded-lg shadow p-6">
                    <div class="flex items-center">
                        <div class="p-3 bg-blue-100 rounded-full">
                            <i class="fas fa-users text-blue-600 text-xl"></i>
                        </div>
                        <div class="ml-4">
                            <p class="text-sm text-gray-500">总用户数</p>
                            <p class="text-2xl font-bold text-gray-900" id="totalUsers">-</p>
                        </div>
                    </div>
                </div>
                <div class="bg-white rounded-lg shadow p-6">
                    <div class="flex items-center">
                        <div class="p-3 bg-green-100 rounded-full">
                            <i class="fas fa-server text-green-600 text-xl"></i>
                        </div>
                        <div class="ml-4">
                            <p class="text-sm text-gray-500">服务账户</p>
                            <p class="text-2xl font-bold text-gray-900" id="totalServices">-</p>
                        </div>
                    </div>
                </div>
                <div class="bg-white rounded-lg shadow p-6">
                    <div class="flex items-center">
                        <div class="p-3 bg-purple-100 rounded-full">
                            <i class="fas fa-hashtag text-purple-600 text-xl"></i>
                        </div>
                        <div class="ml-4">
                            <p class="text-sm text-gray-500">频道总数</p>
                            <p class="text-2xl font-bold text-gray-900" id="totalChannels">-</p>
                        </div>
                    </div>
                </div>
                <div class="bg-white rounded-lg shadow p-6">
                    <div class="flex items-center">
                        <div class="p-3 bg-orange-100 rounded-full">
                            <i class="fas fa-exclamation-triangle text-orange-600 text-xl"></i>
                        </div>
                        <div class="ml-4">
                            <p class="text-sm text-gray-500">待审核</p>
                            <p class="text-2xl font-bold text-gray-900" id="pendingReviews">-</p>
                        </div>
                    </div>
                </div>
            </div>

            <!-- 标签页导航 -->
            <div class="bg-white rounded-lg shadow">
                <div class="border-b border-gray-200">
                    <nav class="flex space-x-8 px-6">
                        <button class="tab-btn py-4 px-1 border-b-2 font-medium text-sm border-indigo-500 text-indigo-600"
                                data-tab="roles" onclick="switchTab('roles')">
                            角色管理
                        </button>
                        <button class="tab-btn py-4 px-1 border-b-2 font-medium text-sm border-transparent text-gray-500"
                                data-tab="permissions" onclick="switchTab('permissions')">
                            权限配置
                        </button>
                        <button class="tab-btn py-4 px-1 border-b-2 font-medium text-sm border-transparent text-gray-500"
                                data-tab="audit" onclick="switchTab('audit')">
                            审核日志
                        </button>
                    </nav>
                </div>
            </div>
        `
    }
}
```

## 8. API接口设计

### 8.1 RESTful API规范

```javascript
// 认证相关
POST /api/admin/auth/login          // 登录
POST /api/admin/auth/logout         // 退出
GET  /api/admin/auth/profile        // 获取用户信息
POST /api/admin/auth/refresh        // 刷新token

// 仪表盘
GET /api/admin/dashboard/stats      // 获取统计数据
GET /api/admin/dashboard/metrics    // 获取性能指标

// 用户管理
GET    /api/admin/users             // 获取用户列表
POST   /api/admin/users             // 创建用户
GET    /api/admin/users/{id}        // 获取用户详情
PUT    /api/admin/users/{id}        // 更新用户
DELETE /api/admin/users/{id}        // 删除用户

// 发送者管理 (新增)
GET    /api/admin/senders           // 获取发送者列表
POST   /api/admin/senders           // 创建发送者
GET    /api/admin/senders/{id}      // 获取发送者详情
PUT    /api/admin/senders/{id}      // 更新发送者
DELETE /api/admin/senders/{id}      // 删除发送者
POST   /api/admin/senders/{id}/regenerate-key // 重新生成API Key
POST   /api/admin/senders/{id}/toggle-status   // 切换发送者状态

// 服务账户申请 (新增)
GET    /api/admin/service-applications           // 获取申请列表
POST   /api/admin/service-applications           // 提交申请
GET    /api/admin/service-applications/{id}      // 获取申请详情
POST   /api/admin/service-applications/{id}/approve  // 批准申请
POST   /api/admin/service-applications/{id}/reject   // 拒绝申请

// 频道管理 (扩展)
GET    /api/admin/channels          // 获取频道列表
POST   /api/admin/channels          // 创建频道
GET    /api/admin/channels/{id}     // 获取频道详情
PUT    /api/admin/channels/{id}     // 更新频道
DELETE /api/admin/channels/{id}     // 删除频道
GET    /api/admin/channels/{id}/members    // 获取频道成员
POST   /api/admin/channels/{id}/members    // 添加频道成员
DELETE /api/admin/channels/{id}/members/{userId} // 移除频道成员

// 消息管理
GET    /api/admin/messages          // 获取消息列表
POST   /api/admin/messages/send     // 发送消息
DELETE /api/admin/messages/{id}     // 删除消息

// 权限管理 (新增)
GET    /api/admin/roles             // 获取角色列表
POST   /api/admin/roles             // 创建角色
PUT    /api/admin/roles/{id}        // 更新角色
DELETE /api/admin/roles/{id}        // 删除角色
GET    /api/admin/permissions       // 获取权限列表
POST   /api/admin/permissions/assign // 分配权限

// 审核日志 (新增)
GET    /api/admin/audit-logs        // 获取审核日志
POST   /api/admin/audit-logs        // 创建审核记录
```

### 8.2 API响应格式

```javascript
// 成功响应
{
    "code": 200,
    "message": "success",
    "data": {
        // 实际数据
    },
    "meta": {
        "total": 100,
        "page": 1,
        "size": 20,
        "total_pages": 5
    }
}

// 错误响应
{
    "code": 400,
    "message": "参数错误",
    "data": null
}
```

### 8.3 API请求封装

```javascript
// js/api.js
class API {
    constructor() {
        this.baseURL = config.api.baseURL
        this.token = localStorage.getItem('admin_token')
    }

    async request(endpoint, options = {}) {
        const url = `${this.baseURL}${endpoint}`
        const defaultOptions = {
            headers: {
                'Content-Type': 'application/json',
                'Authorization': this.token
            }
        }

        const response = await fetch(url, { ...defaultOptions, ...options })
        const data = await response.json()

        if (!response.ok) {
            throw new Error(data.message || '请求失败')
        }

        return data
    }

    // 便捷方法
    get(endpoint, params = {}) {
        const queryString = new URLSearchParams(params).toString()
        const url = queryString ? `${endpoint}?${queryString}` : endpoint
        return this.request(url, { method: 'GET' })
    }

    post(endpoint, data = {}) {
        return this.request(endpoint, {
            method: 'POST',
            body: JSON.stringify(data)
        })
    }

    put(endpoint, data = {}) {
        return this.request(endpoint, {
            method: 'PUT',
            body: JSON.stringify(data)
        })
    }

    delete(endpoint) {
        return this.request(endpoint, { method: 'DELETE' })
    }
}
```

## 7. 样式系统

### 7.1 Tailwind CSS配置

```css
/* 基础样式重置 */
@layer base {
    html {
        font-family: 'Inter', system-ui, sans-serif;
    }
}

/* 自定义工具类 */
@layer utilities {
    .card {
        @apply bg-white rounded-lg shadow p-6 border border-gray-200;
    }

    .btn {
        @apply px-4 py-2 rounded-md font-medium transition-colors duration-200;
    }

    .btn-primary {
        @apply bg-blue-600 text-white hover:bg-blue-700;
    }

    .btn-secondary {
        @apply bg-gray-200 text-gray-900 hover:bg-gray-300;
    }

    .btn-danger {
        @apply bg-red-600 text-white hover:bg-red-700;
    }
}

/* 组件特定样式 */
@layer components {
    .page-header {
        @apply mb-6 pb-4 border-b border-gray-200;
    }

    .page-header h1 {
        @apply text-2xl font-bold text-gray-900;
    }

    .page-header p {
        @apply text-gray-600 mt-1;
    }
}
```

### 7.2 响应式设计

```css
/* 移动端适配 */
@media (max-width: 768px) {
    .admin-topbar .user-name {
        display: none;
    }

    .nav-item span {
        display: none;
    }

    .dashboard-grid {
        grid-template-columns: 1fr;
    }
}

/* 深色模式支持 */
@media (prefers-color-scheme: dark) {
    :host {
        /* 深色模式样式 */
    }
}
```

## 8. 部署配置

### 8.1 环境配置

```javascript
// js/config.js
const config = {
    api: {
        baseURL: window.location.hostname === 'localhost'
            ? 'http://localhost:8081/api/admin'
            : '/api/admin',
        timeout: 30000
    },
    auth: {
        tokenKey: 'admin_token',
        refreshTokenKey: 'admin_refresh_token'
    },
    features: {
        enableCharts: true,
        enableRealTime: true,
        enableExport: true
    }
}
```

### 8.2 入口文件

```html
<!-- index.html -->
<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>消息通知系统管理后台</title>
    <script src="https://cdn.tailwindcss.com"></script>
    <link href="css/admin.css" rel="stylesheet">
</head>
<body class="bg-gray-100">
    <div id="app">
        <!-- 应用内容将在这里动态加载 -->
    </div>

    <!-- 核心JavaScript文件 -->
    <script src="js/config.js"></script>
    <script src="js/security.js"></script>
    <script src="js/utils.js"></script>
    <script src="js/api.js"></script>
    <script src="js/auth.js"></script>
    <script src="js/router.js"></script>

    <!-- WebComponent组件 -->
    <script src="js/components/webcomponents/admin-topbar.js"></script>
    <script src="js/components/webcomponents/admin-toast.js"></script>
    <script src="js/components/webcomponents/admin-dialog.js"></script>

    <!-- 传统组件 -->
    <script src="js/components/table.js"></script>
    <script src="js/components/chart.js"></script>

    <!-- 页面模块 -->
    <script src="js/pages/base-page.js"></script>
    <script src="js/pages/dashboard.js"></script>
    <script src="js/pages/users.js"></script>
    <script src="js/pages/channels.js"></script>
    <script src="js/pages/messages.js"></script>
    <script src="js/pages/settings.js"></script>

    <!-- 组件管理器 -->
    <script src="js/component-manager.js"></script>

    <!-- 应用启动 -->
    <script src="js/app.js"></script>
</body>
</html>
```

### 8.3 应用启动

```javascript
// js/app.js
class AdminApp {
    constructor() {
        this.router = new Router()
        this.currentPage = null
    }

    async init() {
        try {
            // 初始化组件管理器
            await window.componentManager.init()

            // 验证认证状态
            await auth.validateToken()

            // 启动路由
            this.router.handleRoute()

            console.log('管理后台启动成功')
        } catch (error) {
            console.error('应用启动失败:', error)
            this.handleStartupError(error)
        }
    }

    handleStartupError(error) {
        document.getElementById('app').innerHTML = `
            <div class="min-h-screen flex items-center justify-center bg-gray-50">
                <div class="text-center">
                    <div class="text-red-500 text-6xl mb-4">⚠️</div>
                    <h2 class="text-2xl font-bold mb-2">系统启动失败</h2>
                    <p class="text-gray-600 mb-4">${error.message}</p>
                    <button onclick="location.reload()" class="btn btn-primary">
                        重新加载
                    </button>
                </div>
            </div>
        `
    }
}

// 启动应用
document.addEventListener('DOMContentLoaded', () => {
    window.adminApp = new AdminApp()
    window.adminApp.init()
})
```

## 9. 安全考虑

### 9.1 前端安全措施

```javascript
// XSS防护
function escapeHtml(unsafe) {
    return unsafe
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;")
        .replace(/'/g, "&#039;")
}

// CSRF防护
function addCSRFToken(formData) {
    formData.append('csrf_token', security.csrfToken)
    return formData
}

// 输入验证
function validateInput(input, rules) {
    const errors = []

    if (rules.required && !input.trim()) {
        errors.push('此字段为必填项')
    }

    if (rules.maxLength && input.length > rules.maxLength) {
        errors.push(`长度不能超过${rules.maxLength}个字符`)
    }

    if (rules.email && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(input)) {
        errors.push('请输入有效的邮箱地址')
    }

    return { valid: errors.length === 0, errors }
}
```

### 9.2 安全配置

```javascript
// 安全头设置
const securityHeaders = {
    'Content-Security-Policy': "default-src 'self'",
    'X-Frame-Options': 'DENY',
    'X-Content-Type-Options': 'nosniff',
    'Referrer-Policy': 'strict-origin-when-cross-origin'
}

// 敏感操作确认
async function confirmSensitiveOperation(action) {
    const confirmed = await showConfirm(
        `确定要执行${action}操作吗？此操作不可撤销。`,
        { title: '安全确认' }
    )
    return confirmed === 'confirm'
}
```

## 10. 性能优化

### 10.1 加载优化

```javascript
// 懒加载页面模块
class LazyPageLoader {
    constructor() {
        this.loadedPages = new Set()
    }

    async loadPage(pageName) {
        if (this.loadedPages.has(pageName)) {
            return
        }

        try {
            await import(`./pages/${pageName}.js`)
            this.loadedPages.add(pageName)
        } catch (error) {
            console.error(`加载页面失败: ${pageName}`, error)
        }
    }
}

// 图片懒加载
function setupLazyLoading() {
    const images = document.querySelectorAll('img[data-src]')
    const imageObserver = new IntersectionObserver((entries) => {
        entries.forEach(entry => {
            if (entry.isIntersecting) {
                const img = entry.target
                img.src = img.dataset.src
                img.removeAttribute('data-src')
                imageObserver.unobserve(img)
            }
        })
    })

    images.forEach(img => imageObserver.observe(img))
}
```

### 10.2 缓存策略

```javascript
// API请求缓存
class APICache {
    constructor(ttl = 5 * 60 * 1000) { // 5分钟
        this.cache = new Map()
        this.ttl = ttl
    }

    get(key) {
        const item = this.cache.get(key)
        if (item && Date.now() - item.timestamp < this.ttl) {
            return item.data
        }
        return null
    }

    set(key, data) {
        this.cache.set(key, {
            data,
            timestamp: Date.now()
        })
    }
}

const apiCache = new APICache()
```

## 11. 监控和日志

### 11.1 错误监控

```javascript
// 全局错误处理
window.addEventListener('error', (event) => {
    console.error('全局错误:', event.error)

    // 发送错误报告到监控系统
    if (config.monitoring.enabled) {
        this.sendErrorReport({
            message: event.error.message,
            stack: event.error.stack,
            url: window.location.href,
            timestamp: Date.now()
        })
    }
})

// 性能监控
function measurePageLoad() {
    window.addEventListener('load', () => {
        const perfData = performance.getEntriesByType('navigation')[0]
        console.log('页面加载时间:', perfData.loadEventEnd - perfData.fetchStart, 'ms')
    })
}
```

### 11.2 用户行为跟踪

```javascript
// 用户操作日志
class UserActionLogger {
    constructor() {
        this.actions = []
    }

    log(action, details = {}) {
        const logEntry = {
            action,
            details,
            timestamp: Date.now(),
            url: window.location.href,
            userAgent: navigator.userAgent
        }

        this.actions.push(logEntry)
        console.log('用户操作:', logEntry)

        // 发送到后端日志系统
        this.sendLog(logEntry)
    }

    sendLog(logEntry) {
        if (config.logging.enabled) {
            api.post('/logs/user-actions', logEntry).catch(console.error)
        }
    }
}

const userLogger = new UserActionLogger()
```

## 12. 开发指南

### 12.1 开发环境设置

1. **安装依赖**: 无需npm安装，直接使用CDN引入Tailwind CSS
2. **启动后端**: `go run main.go` 启动Go后端服务
3. **启动前端**: 直接用浏览器打开`index.html`或使用Live Server

### 12.2 开发规范

**命名规范:**
- 文件名: kebab-case (例: user-management.js)
- 类名: PascalCase (例: UserManagementPage)
- 函数名: camelCase (例: loadUserData)
- 常量: UPPER_SNAKE_CASE (例: API_BASE_URL)

**代码规范:**
- 使用ES6+语法
- 采用模块化设计
- 完善的错误处理
- 详细的注释文档

### 12.3 添加新页面

```javascript
// 1. 创建页面类
class NewPage extends BasePage {
    async render() {
        this.setPageTitle('新页面')

        const content = document.getElementById('page-content')
        content.innerHTML = `
            <h1>新页面内容</h1>
        `
    }

    async bindEvents() {
        // 绑定页面事件
    }
}

// 2. 注册路由
router.register('/new-page', 'new-page')

// 3. 在router.js中添加页面加载逻辑
if (page === 'new-page') {
    window.componentManager.createPage(NewPage)
}
```

### 12.4 添加新组件

```javascript
// 1. 创建WebComponent
class NewComponent extends BaseComponent {
    getStyles() {
        return `
            :host {
                display: block;
            }
        `
    }

    render() {
        this.shadowRoot.innerHTML = `
            <div class="new-component">
                <!-- 组件内容 -->
            </div>
        `
    }
}

// 2. 注册组件
customElements.define('new-component', NewComponent)

// 3. 使用组件
<new-component></new-component>
```

## 13. 部署和维护

### 13.1 部署清单

**前端部署:**
- [ ] 配置生产环境API地址
- [ ] 启用Gzip压缩
- [ ] 设置缓存策略
- [ ] 配置安全头
- [ ] 测试所有页面功能

**后端部署:**
- [ ] 配置数据库连接
- [ ] 设置环境变量
- [ ] 启用HTTPS
- [ ] 配置CORS
- [ ] 设置监控告警

### 13.2 维护指南

**日常维护:**
- 定期检查系统日志
- 监控API响应时间
- 备份重要数据
- 更新安全补丁

**性能优化:**
- 监控页面加载速度
- 优化数据库查询
- 压缩静态资源
- 使用CDN加速

---

**文档版本:** v1.0
**创建时间:** 2025-01-15
**最后更新:** 2025-01-15
**维护人员:** 开发团队

**联系方式:**
- 技术支持: dev-team@company.com
- 问题反馈: issues@company.com