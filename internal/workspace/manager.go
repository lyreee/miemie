package workspace

import (
	"database/sql"
	"fmt"
	"miemie/internal/config"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Workspace struct {
	UserID     string
	BasePath   string
	Database   *sql.DB
	MessagesDB *sql.DB
	ReadDB     *sql.DB
	mu         sync.RWMutex
}

type Manager struct {
	basePath string
	cache     *WorkspaceCache
	mu        sync.RWMutex
}

func NewManager(basePath string) *Manager {
	return NewManagerWithConfig(basePath, nil)
}

func NewManagerWithConfig(basePath string, cfg *config.Config) *Manager {
	// 配置缓存参数 - 使用配置文件中的值，如果配置为空则使用默认值
	var maxSize int
	var ttl time.Duration

	if cfg != nil {
		maxSize = cfg.Cache.Workspace.MaxSize
		ttl = cfg.Cache.GetTTL()
	} else {
		maxSize = 1000                 // 默认最大缓存1000个工作空间
		ttl = 30 * time.Minute        // 默认30分钟过期时间
	}

	return &Manager{
		basePath: basePath,
		cache:    NewWorkspaceCache(maxSize, ttl),
	}
}

// GetUserWorkspace 获取或创建用户工作空间
func (m *Manager) GetUserWorkspace(userID string) (*Workspace, error) {
	// 🔧 首先尝试从缓存获取
	if ws, found := m.cache.Get(userID); found {
		return ws, nil
	}

	// 缓存中没有，需要创建新的工作空间
	m.mu.Lock()
	defer m.mu.Unlock()

	// 双重检查，防止并发创建
	if ws, found := m.cache.Get(userID); found {
		return ws, nil
	}

	// 创建新工作空间
	ws, err := m.createUserWorkspace(userID)
	if err != nil {
		return nil, err
	}

	// 🔧 存入缓存
	m.cache.Put(userID, ws)

	return ws, nil
}

func (m *Manager) createUserWorkspace(userID string) (*Workspace, error) {
	// 创建用户目录结构
	userPath := filepath.Join(m.basePath, userID)
	if err := os.MkdirAll(userPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create user directory: %w", err)
	}

	// 创建同步信息目录
	syncPath := filepath.Join(userPath, ".sync")
	if err := os.MkdirAll(syncPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create sync directory: %w", err)
	}

	// 创建备份目录
	backupPath := filepath.Join(userPath, "backups")
	if err := os.MkdirAll(backupPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	// 初始化数据库文件
	messagesDBPath := filepath.Join(userPath, "messages.db")
	readDBPath := filepath.Join(userPath, "read_status.db")

	// 打开消息数据库
	messagesDB, err := sql.Open("sqlite3", messagesDBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open messages database: %w", err)
	}

	// 打开已读状态数据库
	readDB, err := sql.Open("sqlite3", readDBPath)
	if err != nil {
		messagesDB.Close()
		return nil, fmt.Errorf("failed to open read status database: %w", err)
	}

	// 创建工作空间
	ws := &Workspace{
		UserID:     userID,
		BasePath:   userPath,
		Database:   nil, // 保留兼容性
		MessagesDB: messagesDB,
		ReadDB:     readDB,
	}

	// 初始化数据库表结构
	if err := ws.initDatabase(); err != nil {
		messagesDB.Close()
		readDB.Close()
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	return ws, nil
}

func (ws *Workspace) initDatabase() error {
	// 🔧 启用WAL模式以提高并发性能
	if err := ws.enableWALMode(); err != nil {
		return fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// 初始化消息数据库
	if err := ws.initMessagesTable(); err != nil {
		return fmt.Errorf("failed to init messages table: %w", err)
	}

	// 初始化已读状态数据库
	if err := ws.initReadStatusTable(); err != nil {
		return fmt.Errorf("failed to init read status table: %w", err)
	}

	return nil
}

// enableWALMode 启用WAL模式以提高并发性能
func (ws *Workspace) enableWALMode() error {
	// 为消息数据库启用WAL
	if ws.MessagesDB != nil {
		if _, err := ws.MessagesDB.Exec("PRAGMA journal_mode=WAL"); err != nil {
			return fmt.Errorf("failed to enable WAL for messages DB: %w", err)
		}
		// 优化WAL性能
		if _, err := ws.MessagesDB.Exec("PRAGMA synchronous=NORMAL"); err != nil {
			return fmt.Errorf("failed to set synchronous mode for messages DB: %w", err)
		}
		if _, err := ws.MessagesDB.Exec("PRAGMA cache_size=10000"); err != nil {
			return fmt.Errorf("failed to set cache size for messages DB: %w", err)
		}
	}

	// 为已读状态数据库启用WAL
	if ws.ReadDB != nil {
		if _, err := ws.ReadDB.Exec("PRAGMA journal_mode=WAL"); err != nil {
			return fmt.Errorf("failed to enable WAL for read status DB: %w", err)
		}
		// 优化WAL性能
		if _, err := ws.ReadDB.Exec("PRAGMA synchronous=NORMAL"); err != nil {
			return fmt.Errorf("failed to set synchronous mode for read status DB: %w", err)
		}
		if _, err := ws.ReadDB.Exec("PRAGMA cache_size=5000"); err != nil {
			return fmt.Errorf("failed to set cache size for read status DB: %w", err)
		}
	}

	return nil
}

func (ws *Workspace) initMessagesTable() error {
	// 创建消息表
	createMessagesTable := `
	CREATE TABLE IF NOT EXISTS messages (
		id TEXT PRIMARY KEY,
		channel_id TEXT NOT NULL,
		title TEXT NOT NULL,
		content TEXT NOT NULL,
		message_type TEXT DEFAULT 'text',
		priority INTEGER DEFAULT 5,
		sender TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		metadata TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_channel_created ON messages(channel_id, created_at);
	CREATE INDEX IF NOT EXISTS idx_created ON messages(created_at);
	CREATE INDEX IF NOT EXISTS idx_priority ON messages(priority);
	`

	// 创建频道表
	createChannelsTable := `
	CREATE TABLE IF NOT EXISTS channels (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		description TEXT,
		created_by TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_message_at DATETIME
	);
	`

	// 创建用户频道关联表
	createUserChannelsTable := `
	CREATE TABLE IF NOT EXISTS user_channels (
		channel_id TEXT,
		user_id TEXT,
		joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		is_muted BOOLEAN DEFAULT FALSE,
		PRIMARY KEY (channel_id, user_id)
	);
	`

	tables := []string{createMessagesTable, createChannelsTable, createUserChannelsTable}
	for _, tableSQL := range tables {
		if _, err := ws.MessagesDB.Exec(tableSQL); err != nil {
			return err
		}
	}

	// 确保默认频道存在
	return ws.ensureDefaultChannel()
}

func (ws *Workspace) initReadStatusTable() error {
	// 创建已读状态表
	createReadStatusTable := `
	CREATE TABLE IF NOT EXISTS read_status (
		message_id TEXT PRIMARY KEY,
		read_at DATETIME NOT NULL,
		read_device TEXT,
		archived_at DATETIME,
		starred_at DATETIME,
		metadata TEXT
	);
	`

	// 创建阅读统计表
	createReadStatsTable := `
	CREATE TABLE IF NOT EXISTS read_stats (
		date TEXT PRIMARY KEY,
		total_read INTEGER DEFAULT 0,
		channel_stats TEXT,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`

	// 创建阅读位置表
	createReadingPositionTable := `
	CREATE TABLE IF NOT EXISTS reading_position (
		channel_id TEXT PRIMARY KEY,
		last_read_message_id TEXT,
		last_read_at DATETIME,
		position INTEGER DEFAULT 0
	);
	`

	tables := []string{createReadStatusTable, createReadStatsTable, createReadingPositionTable}
	for _, tableSQL := range tables {
		if _, err := ws.ReadDB.Exec(tableSQL); err != nil {
			return err
		}
	}

	return nil
}

func (ws *Workspace) ensureDefaultChannel() error {
	// 检查默认频道是否存在
	var count int
	err := ws.MessagesDB.QueryRow("SELECT COUNT(*) FROM channels WHERE id = ?", "default").Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		// 创建默认频道
		_, err = ws.MessagesDB.Exec(`
			INSERT INTO channels (id, name, description, created_by)
			VALUES (?, ?, ?, ?)
		`, "default", "默认频道", "系统默认消息频道", "system")
		return err
	}

	return nil
}

// Close 关闭工作空间的数据库连接
func (ws *Workspace) Close() error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	var errors []error

	if ws.MessagesDB != nil {
		if err := ws.MessagesDB.Close(); err != nil {
			errors = append(errors, err)
		}
	}

	if ws.ReadDB != nil {
		if err := ws.ReadDB.Close(); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("multiple errors occurred: %v", errors)
	}

	return nil
}

// Close 关闭缓存和所有工作空间
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 关闭缓存，会自动清理所有数据库连接
	m.cache.Close()

	return nil
}

// ListWorkspaces 列出活跃工作空间
func (m *Manager) ListWorkspaces() []string {
	return m.cache.ListActiveUsers()
}

// RemoveWorkspace 移除工作空间
func (m *Manager) RemoveWorkspace(userID string) error {
	m.cache.Remove(userID)
	return nil
}

// GetCacheStats 获取缓存统计信息
func (m *Manager) GetCacheStats() map[string]interface{} {
	return m.cache.Stats()
}

// GetCacheSize 获取缓存大小
func (m *Manager) GetCacheSize() int {
	return m.cache.Size()
}