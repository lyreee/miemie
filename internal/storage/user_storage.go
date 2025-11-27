package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"miemie/internal/models"
	"miemie/internal/workspace"
	"time"
)

type UserMessageStorage struct {
	workspace *workspace.Workspace
}

func NewUserMessageStorage(ws *workspace.Workspace) *UserMessageStorage {
	return &UserMessageStorage{
		workspace: ws,
	}
}

func (ums *UserMessageStorage) CreateMessage(message *models.Message) error {
	metadataJSON, _ := json.Marshal(message.Metadata)

	query := `
	INSERT INTO messages (id, channel_id, title, content, message_type, priority, sender, created_at, updated_at, metadata)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := ums.workspace.MessagesDB.Exec(query,
		message.ID,
		message.ChannelID,
		message.Title,
		message.Content,
		message.MessageType,
		message.Priority,
		message.Sender,
		message.CreatedAt,
		message.UpdatedAt,
		string(metadataJSON),
	)

	if err != nil {
		return fmt.Errorf("failed to create message: %w", err)
	}

	// 更新频道的最后消息时间
	return ums.updateChannelLastMessage(message.ChannelID, message.CreatedAt)
}

func (ums *UserMessageStorage) GetMessages(channelID string, limit, offset int) ([]*models.Message, error) {
	query := `
	SELECT id, channel_id, title, content, message_type, priority, sender, created_at, updated_at, metadata
	FROM messages
	WHERE channel_id = ?
	ORDER BY created_at DESC
	LIMIT ? OFFSET ?
	`

	rows, err := ums.workspace.MessagesDB.Query(query, channelID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query messages: %w", err)
	}
	defer rows.Close()

	var messages []*models.Message
	for rows.Next() {
		message := &models.Message{}
		var metadataJSON sql.NullString

		err := rows.Scan(
			&message.ID,
			&message.ChannelID,
			&message.Title,
			&message.Content,
			&message.MessageType,
			&message.Priority,
			&message.Sender,
			&message.CreatedAt,
			&message.UpdatedAt,
			&metadataJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}

		if metadataJSON.Valid {
			json.Unmarshal([]byte(metadataJSON.String), &message.Metadata)
		}

		messages = append(messages, message)
	}

	return messages, nil
}

func (ums *UserMessageStorage) GetMessage(id string) (*models.Message, error) {
	query := `
	SELECT id, channel_id, title, content, message_type, priority, sender, created_at, updated_at, metadata
	FROM messages
	WHERE id = ?
	`

	message := &models.Message{}
	var metadataJSON sql.NullString

	err := ums.workspace.MessagesDB.QueryRow(query, id).Scan(
		&message.ID,
		&message.ChannelID,
		&message.Title,
		&message.Content,
		&message.MessageType,
		&message.Priority,
		&message.Sender,
		&message.CreatedAt,
		&message.UpdatedAt,
		&metadataJSON,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("message not found")
		}
		return nil, fmt.Errorf("failed to get message: %w", err)
	}

	if metadataJSON.Valid {
		json.Unmarshal([]byte(metadataJSON.String), &message.Metadata)
	}

	return message, nil
}

func (ums *UserMessageStorage) CreateChannel(channel *models.Channel) error {
	query := `
	INSERT INTO channels (id, name, description, created_by, created_at)
	VALUES (?, ?, ?, ?, ?)
	`

	_, err := ums.workspace.MessagesDB.Exec(query,
		channel.ID,
		channel.Name,
		channel.Description,
		channel.CreatedBy,
		channel.CreatedAt,
	)

	return err
}

func (ums *UserMessageStorage) GetChannel(id string) (*models.Channel, error) {
	query := `
	SELECT id, name, description, created_by, created_at, last_message_at
	FROM channels
	WHERE id = ?
	`

	channel := &models.Channel{}
	err := ums.workspace.MessagesDB.QueryRow(query, id).Scan(
		&channel.ID,
		&channel.Name,
		&channel.Description,
		&channel.CreatedBy,
		&channel.CreatedAt,
		&channel.LastMessageAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("channel not found")
		}
		return nil, fmt.Errorf("failed to get channel: %w", err)
	}

	return channel, nil
}

func (ums *UserMessageStorage) GetAllChannels() ([]*models.Channel, error) {
	query := `
	SELECT id, name, description, created_by, created_at, last_message_at
	FROM channels
	ORDER BY last_message_at DESC, created_at DESC
	`

	rows, err := ums.workspace.MessagesDB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query channels: %w", err)
	}
	defer rows.Close()

	var channels []*models.Channel
	for rows.Next() {
		channel := &models.Channel{}
		err := rows.Scan(
			&channel.ID,
			&channel.Name,
			&channel.Description,
			&channel.CreatedBy,
			&channel.CreatedAt,
			&channel.LastMessageAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan channel: %w", err)
		}

		channels = append(channels, channel)
	}

	return channels, nil
}

func (ums *UserMessageStorage) updateChannelLastMessage(channelID string, messageTime time.Time) error {
	query := `UPDATE channels SET last_message_at = ? WHERE id = ?`
	_, err := ums.workspace.MessagesDB.Exec(query, messageTime, channelID)
	return err
}

// 添加已读状态
func (ums *UserMessageStorage) MarkAsRead(messageID, deviceID string) error {
	now := time.Now()
	query := `
	INSERT OR REPLACE INTO read_status (message_id, read_at, read_device)
	VALUES (?, ?, ?)
	`
	_, err := ums.workspace.ReadDB.Exec(query, messageID, now, deviceID)
	return err
}

// 批量标记已读
func (ums *UserMessageStorage) MarkMultipleAsRead(messageIDs []string, deviceID string) error {
	if len(messageIDs) == 0 {
		return nil
	}

	now := time.Now()
	tx, err := ums.workspace.ReadDB.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO read_status (message_id, read_at, read_device)
		VALUES (?, ?, ?)
	`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, messageID := range messageIDs {
		_, err = stmt.Exec(messageID, now, deviceID)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}

// 获取未读消息数量
func (ums *UserMessageStorage) GetUnreadCount(channelID string) (int, error) {
	// 获取所有消息ID，然后检查哪些未读
	var totalMessages []string
	var query string
	var args []interface{}

	if channelID == "" {
		query = "SELECT id FROM messages"
		args = []interface{}{}
	} else {
		query = "SELECT id FROM messages WHERE channel_id = ?"
		args = []interface{}{channelID}
	}

	rows, err := ums.workspace.MessagesDB.Query(query, args...)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var messageID string
		if err := rows.Scan(&messageID); err != nil {
			continue
		}
		totalMessages = append(totalMessages, messageID)
	}

	if len(totalMessages) == 0 {
		return 0, nil
	}

	// 检查哪些消息已读
	placeholders := ""
	for i := range totalMessages {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
	}

	readQuery := fmt.Sprintf("SELECT COUNT(*) FROM read_status WHERE message_id IN (%s)", placeholders)
	readArgs := make([]interface{}, len(totalMessages))
	for i, msgID := range totalMessages {
		readArgs[i] = msgID
	}

	var readCount int
	err = ums.workspace.ReadDB.QueryRow(readQuery, readArgs...).Scan(&readCount)
	if err != nil {
		return 0, err
	}

	return len(totalMessages) - readCount, nil
}

// 检查消息是否已读
func (ums *UserMessageStorage) IsMessageRead(messageID string) (bool, error) {
	var count int
	err := ums.workspace.ReadDB.QueryRow("SELECT COUNT(*) FROM read_status WHERE message_id = ?", messageID).Scan(&count)
	return count > 0, err
}

// 获取消息的已读状态
func (ums *UserMessageStorage) GetReadStatus(messageID string) (*models.ReadStatus, error) {
	var readStatus models.ReadStatus
	var archivedAt, starredAt sql.NullTime
	var metadataJSON sql.NullString

	query := `
	SELECT message_id, read_at, read_device, archived_at, starred_at, metadata
	FROM read_status
	WHERE message_id = ?
	`

	err := ums.workspace.ReadDB.QueryRow(query, messageID).Scan(
		&readStatus.MessageID,
		&readStatus.ReadAt,
		&readStatus.ReadDevice,
		&archivedAt,
		&starredAt,
		&metadataJSON,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if archivedAt.Valid {
		readStatus.ArchivedAt = &archivedAt.Time
	}
	if starredAt.Valid {
		readStatus.StarredAt = &starredAt.Time
	}
	if metadataJSON.Valid {
		json.Unmarshal([]byte(metadataJSON.String), &readStatus.Metadata)
	}

	return &readStatus, nil
}

// 获取用户统计信息
func (ums *UserMessageStorage) GetUserStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 总消息数
	var totalMessages int
	err := ums.workspace.MessagesDB.QueryRow("SELECT COUNT(*) FROM messages").Scan(&totalMessages)
	if err != nil {
		return nil, err
	}
	stats["total_messages"] = totalMessages

	// 总频道数
	var totalChannels int
	err = ums.workspace.MessagesDB.QueryRow("SELECT COUNT(*) FROM channels").Scan(&totalChannels)
	if err != nil {
		return nil, err
	}
	stats["total_channels"] = totalChannels

	// 未读消息数
	unreadCount, err := ums.GetUnreadCount("")
	if err != nil {
		return nil, err
	}
	stats["unread_messages"] = unreadCount

	// 各频道消息数
	channelStats := make(map[string]int)
	rows, err := ums.workspace.MessagesDB.Query("SELECT channel_id, COUNT(*) FROM messages GROUP BY channel_id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var channelID string
		var count int
		if err := rows.Scan(&channelID, &count); err != nil {
			continue
		}
		channelStats[channelID] = count
	}
	stats["channel_stats"] = channelStats

	return stats, nil
}