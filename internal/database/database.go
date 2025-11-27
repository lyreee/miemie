package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

type Database struct {
	db *sql.DB
}

func Initialize(databasePath string) (*Database, error) {
	// 确保数据库目录存在
	dir := filepath.Dir(databasePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// 打开数据库连接
	db, err := sql.Open("sqlite3", databasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 测试连接
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	database := &Database{db: db}

	// 初始化数据库表
	if err := database.initTables(); err != nil {
		return nil, fmt.Errorf("failed to initialize tables: %w", err)
	}

	// 运行数据库迁移
	if err := database.RunMigrations(); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return database, nil
}

func (d *Database) initTables() error {
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

	// 执行建表语句
	tables := []string{createMessagesTable, createChannelsTable, createReadStatusTable}
	for _, tableSQL := range tables {
		if _, err := d.db.Exec(tableSQL); err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}
	}

	return nil
}

func (d *Database) Close() error {
	return d.db.Close()
}

func (d *Database) GetDB() *sql.DB {
	return d.db
}