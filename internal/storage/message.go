package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"miemie/internal/models"
	"time"
)

type MessageStorage struct {
	db *sql.DB
}

func NewMessageStorage(db *sql.DB) *MessageStorage {
	return &MessageStorage{db: db}
}

func (ms *MessageStorage) CreateMessage(message *models.Message) error {
	metadataJSON, _ := json.Marshal(message.Metadata)

	query := `
	INSERT INTO messages (id, user_id, channel_id, title, content, message_type, priority, sender, created_at, updated_at, metadata)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := ms.db.Exec(query,
		message.ID,
		message.UserID,
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
	return ms.updateChannelLastMessage(message.ChannelID, message.CreatedAt)
}

func (ms *MessageStorage) GetMessages(userID, channelID string, limit, offset int) ([]*models.Message, error) {
	query := `
	SELECT id, user_id, channel_id, title, content, message_type, priority, sender, created_at, updated_at, metadata
	FROM messages
	WHERE user_id = ? AND channel_id = ?
	ORDER BY created_at DESC
	LIMIT ? OFFSET ?
	`

	rows, err := ms.db.Query(query, userID, channelID, limit, offset)
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
			&message.UserID,
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

func (ms *MessageStorage) GetMessage(userID, id string) (*models.Message, error) {
	query := `
	SELECT id, user_id, channel_id, title, content, message_type, priority, sender, created_at, updated_at, metadata
	FROM messages
	WHERE user_id = ? AND id = ?
	`

	message := &models.Message{}
	var metadataJSON sql.NullString

	err := ms.db.QueryRow(query, userID, id).Scan(
		&message.ID,
		&message.UserID,
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

func (ms *MessageStorage) CreateChannel(channel *models.Channel) error {
	query := `
	INSERT INTO channels (id, name, description, created_by, created_at)
	VALUES (?, ?, ?, ?, ?)
	`

	_, err := ms.db.Exec(query,
		channel.ID,
		channel.Name,
		channel.Description,
		channel.CreatedBy,
		channel.CreatedAt,
	)

	return err
}

func (ms *MessageStorage) GetChannel(id string) (*models.Channel, error) {
	query := `
	SELECT id, name, description, created_by, created_at, last_message_at
	FROM channels
	WHERE id = ?
	`

	channel := &models.Channel{}
	err := ms.db.QueryRow(query, id).Scan(
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

func (ms *MessageStorage) GetAllChannels() ([]*models.Channel, error) {
	query := `
	SELECT id, name, description, created_by, created_at, last_message_at
	FROM channels
	ORDER BY last_message_at DESC, created_at DESC
	`

	rows, err := ms.db.Query(query)
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

func (ms *MessageStorage) updateChannelLastMessage(channelID string, messageTime time.Time) error {
	query := `UPDATE channels SET last_message_at = ? WHERE id = ?`
	_, err := ms.db.Exec(query, messageTime, channelID)
	return err
}

// 创建默认频道
func (ms *MessageStorage) EnsureDefaultChannel() (*models.Channel, error) {
	channelID := "default"

	// 先尝试获取默认频道
	channel, err := ms.GetChannel(channelID)
	if err == nil {
		return channel, nil
	}

	// 如果不存在，创建默认频道
	defaultChannel := &models.Channel{
		ID:          channelID,
		Name:        "默认频道",
		Description: "系统默认消息频道",
		CreatedBy:   "system",
		CreatedAt:   time.Now(),
	}

	if err := ms.CreateChannel(defaultChannel); err != nil {
		return nil, fmt.Errorf("failed to create default channel: %w", err)
	}

	return defaultChannel, nil
}