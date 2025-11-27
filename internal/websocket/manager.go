package websocket

import (
	"encoding/json"
	"miemie/internal/logger"
	"miemie/internal/models"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源，生产环境需要更严格的检查
	},
}

type Client struct {
	ID     string
	UserID string
	Conn   *websocket.Conn
	Send   chan []byte
	Hub    *Manager
	Active bool
_mu    sync.RWMutex
}

func (c *Client) IsActive() bool {
	c._mu.RLock()
	defer c._mu.RUnlock()
	return c.Active
}

func (c *Client) SetActive(active bool) {
	c._mu.Lock()
	defer c._mu.Unlock()
	c.Active = active
}

type Manager struct {
	clients    map[*Client]bool
	userClients map[string]map[*Client]bool // 用户ID到客户端的映射
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

func NewManager() *Manager {
	return &Manager{
		clients:    make(map[*Client]bool),
		userClients: make(map[string]map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (m *Manager) Run() {
	for {
		select {
		case client := <-m.register:
			m.mu.Lock()
			m.clients[client] = true

			// 添加到用户映射
			if _, exists := m.userClients[client.UserID]; !exists {
				m.userClients[client.UserID] = make(map[*Client]bool)
			}
			m.userClients[client.UserID][client] = true

			m.mu.Unlock()
			logger.Infof("Client connected: %s (user: %s), total clients: %d", client.ID, client.UserID, len(m.clients))

		case client := <-m.unregister:
			m.mu.Lock()
			if _, ok := m.clients[client]; ok {
				delete(m.clients, client)

				// 从用户映射中移除
				if userMap, exists := m.userClients[client.UserID]; exists {
					delete(userMap, client)
					if len(userMap) == 0 {
						delete(m.userClients, client.UserID)
					}
				}

				close(client.Send)
				client.SetActive(false)
				logger.Infof("Client disconnected: %s (user: %s), total clients: %d", client.ID, client.UserID, len(m.clients))
			}
			m.mu.Unlock()

		case message := <-m.broadcast:
			m.mu.RLock()
			for client := range m.clients {
				if client.IsActive() {
					select {
					case client.Send <- message:
					default:
						// 发送失败，移除客户端
						close(client.Send)
						delete(m.clients, client)
						client.SetActive(false)
					}
				}
			}
			m.mu.RUnlock()
		}
	}
}

func (m *Manager) HandleWebSocket(c *gin.Context) {
	w := c.Writer
	r := c.Request

	// 从上下文获取用户ID
	userID := c.GetString("user_id")
	if userID == "" {
		// 如果上下文没有用户ID，尝试从URL参数获取
		userID = c.Query("user_id")
		if userID == "" {
			userID = "default"
		}
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Infof("WebSocket upgrade failed: %v", err)
		return
	}

	client := &Client{
		ID:     generateClientID(),
		UserID: userID,
		Conn:   conn,
		Send:   make(chan []byte, 256),
		Hub:    m,
		Active: true,
	}

	m.register <- client

	// 启动读写协程
	go client.writePump()
	go client.readPump()
}

func (m *Manager) BroadcastMessage(message *models.Message) {
	data, err := json.Marshal(map[string]interface{}{
		"type": "message",
		"data": message,
	})
	if err != nil {
		logger.Infof("Failed to marshal message: %v", err)
		return
	}

	// 只发送给对应用户的客户端
	m.mu.RLock()
	defer m.mu.RUnlock()

	if userClients, exists := m.userClients[message.UserID]; exists {
		for client := range userClients {
			if client.IsActive() {
				select {
				case client.Send <- data:
				default:
					// 发送失败，移除客户端
					client.SetActive(false)
				}
			}
		}
	} else {
		logger.Infof("No active clients for user: %s", message.UserID)
	}
}

func (m *Manager) GetClientCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.clients)
}

func (m *Manager) GetUserClientCount(userID string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if userClients, exists := m.userClients[userID]; exists {
		return len(userClients)
	}
	return 0
}

func (m *Manager) GetOnlineUsers() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	users := make([]string, 0, len(m.userClients))
	for userID := range m.userClients {
		users = append(users, userID)
	}
	return users
}

func (c *Client) readPump() {
	defer func() {
		c.Hub.unregister <- c
		c.Conn.Close()
	}()

	// 设置读取超时
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, _, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Infof("WebSocket error: %v", err)
			}
			break
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				// Hub关闭了连接
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// 发送队列中的其他消息
			n := len(c.Send)
			for i := 0; i < n; i++ {
				w.Write(<-c.Send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			// 发送心跳
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func generateClientID() string {
	return "client_" + time.Now().Format("20060102150405") + "_" + randomString(8)
}

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}