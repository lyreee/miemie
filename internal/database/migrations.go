package database

import (
	"fmt"
)

// MigrateToV1_1 添加用户ID支持
func (d *Database) MigrateToV1_1() error {
	// 检查是否已经存在user_id列
	var columnExists bool
	err := d.db.QueryRow(`
		SELECT COUNT(*) > 0 FROM pragma_table_info('messages') WHERE name = 'user_id'
	`).Scan(&columnExists)

	if err != nil {
		return fmt.Errorf("failed to check user_id column: %w", err)
	}

	if !columnExists {
		// 添加user_id列
		_, err := d.db.Exec(`
			ALTER TABLE messages ADD COLUMN user_id TEXT NOT NULL DEFAULT 'default'
		`)
		if err != nil {
			return fmt.Errorf("failed to add user_id column: %w", err)
		}

		// 为现有数据设置默认用户ID
		_, err = d.db.Exec(`
			UPDATE messages SET user_id = 'default' WHERE user_id = 'default'
		`)
		if err != nil {
			return fmt.Errorf("failed to update existing messages: %w", err)
		}

		// 创建用户ID索引
		_, err = d.db.Exec(`
			CREATE INDEX IF NOT EXISTS idx_user_id ON messages(user_id)
		`)
		if err != nil {
			return fmt.Errorf("failed to create user_id index: %w", err)
		}

		// 创建组合索引
		_, err = d.db.Exec(`
			CREATE INDEX IF NOT EXISTS idx_user_channel_created ON messages(user_id, channel_id, created_at)
		`)
		if err != nil {
			return fmt.Errorf("failed to create composite index: %w", err)
		}
	}

	return nil
}

// RunMigrations 运行所有数据库迁移
func (d *Database) RunMigrations() error {
	if err := d.MigrateToV1_1(); err != nil {
		return fmt.Errorf("migration to v1.1 failed: %w", err)
	}
	return nil
}