package models

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Message struct {
	ID          string                 `json:"id"`
	UserID      string                 `json:"user_id"`
	ChannelID   string                 `json:"channel_id"`
	Title       string                 `json:"title"`
	Content     string                 `json:"content"`
	MessageType string                 `json:"message_type"`
	Priority    int                    `json:"priority"`
	Sender      string                 `json:"sender"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type CreateMessageRequest struct {
	UserID      string                 `json:"user_id,omitempty"`      // 可选，如果不提供则从Header获取
	ChannelID   string                 `json:"channel_id" binding:"required"`
	Title       string                 `json:"title" binding:"required"`
	Content     string                 `json:"content" binding:"required"`
	MessageType string                 `json:"message_type"`
	Priority    int                    `json:"priority"`
	Sender      string                 `json:"sender"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// NewMessage 创建新消息，需要传入用户ID
func NewMessage(req CreateMessageRequest, userID string) *Message {
	now := time.Now()
	_ , _ = json.Marshal(req.Metadata) // 预留，可能用于将来扩展

	// 使用请求中的UserID，如果没有则使用传入的userID
	finalUserID := req.UserID
	if finalUserID == "" {
		finalUserID = userID
	}

	return &Message{
		ID:          uuid.New().String(),
		UserID:      finalUserID,
		ChannelID:   req.ChannelID,
		Title:       req.Title,
		Content:     req.Content,
		MessageType: req.MessageType,
		Priority:    req.Priority,
		Sender:      req.Sender,
		CreatedAt:   now,
		UpdatedAt:   now,
		Metadata:    req.Metadata,
	}
}

// GenerateUUID 生成UUID
func GenerateUUID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

type Channel struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	LastMessageAt *time.Time `json:"last_message_at,omitempty"`
}

type ReadStatus struct {
	MessageID  string    `json:"message_id"`
	ReadAt     time.Time `json:"read_at"`
	ReadDevice string    `json:"read_device"`
	ArchivedAt *time.Time `json:"archived_at,omitempty"`
	StarredAt  *time.Time `json:"starred_at,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}