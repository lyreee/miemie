package api

import (
	"fmt"
	"miemie/internal/config"
	"miemie/internal/delivery"
	"miemie/internal/logger"
	"miemie/internal/middleware"
	"miemie/internal/models"
	"miemie/internal/storage"
	"miemie/internal/websocket"
	"miemie/internal/workspace"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type SimpleAPIHandler struct {
	workspaceManager *workspace.Manager
	wsManager       *websocket.Manager
	config          *config.Config
	deliverySystem  *delivery.DeliverySystem // 新增投递系统
}

func SetupSimpleRoutes(r *gin.Engine, cfg *config.Config, wsManager *websocket.Manager) {
	workspaceManager := workspace.NewManagerWithConfig(cfg.Server.UserStorage, cfg)

	// 创建投递系统
	deliverySystem := delivery.NewDeliverySystemWithConfig(workspaceManager, wsManager, cfg)
	if err := deliverySystem.Start(); err != nil {
		panic(fmt.Sprintf("Failed to start delivery system: %v", err))
	}

	handler := &SimpleAPIHandler{
		workspaceManager: workspaceManager,
		wsManager:       wsManager,
		config:          cfg,
		deliverySystem:  deliverySystem,
	}

	// 添加用户ID中间件
	r.Use(middleware.UserIDMiddleware())

	api := r.Group("/api/v3")
	{
		// 消息相关API
		api.POST("/messages", handler.CreateMessage)
		api.GET("/messages", handler.GetMessages)
		api.GET("/messages/:id", handler.GetMessage)

		// 频道相关API
		api.GET("/channels", handler.GetChannels)
		api.GET("/channels/:id", handler.GetChannel)
		api.POST("/channels", handler.CreateChannel)

		// 批量操作API
		api.POST("/messages/batch", handler.CreateMessagesBatch)

		// 用户相关API
		api.GET("/user/stats", handler.GetUserStats)
		api.POST("/messages/:id/read", handler.MarkAsRead)
		api.GET("/messages/unread-count", handler.GetUnreadCount)

		// 投递系统API
		api.GET("/delivery/stats", handler.GetDeliveryStats)

		// 缓存管理API
		api.GET("/workspace/cache/stats", handler.GetWorkspaceCacheStats)
	}
}

// CreateMessage 创建单条消息（使用投递系统）
func (h *SimpleAPIHandler) CreateMessage(c *gin.Context) {
	var req models.CreateMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request parameters",
			"error":   err.Error(),
		})
		return
	}

	// 如果没有指定频道，使用默认频道
	if req.ChannelID == "" {
		req.ChannelID = "default"
	}

	// 设置默认值
	if req.MessageType == "" {
		req.MessageType = "text"
	}
	if req.Priority == 0 {
		req.Priority = 5
	}

	// 从上下文获取用户ID
	userID := middleware.GetUserID(c)

	// 创建消息
	message := models.NewMessage(req, userID)

	// 🚀 记录API请求日志
	logger.WithFields(logrus.Fields{
		"user_id":     userID,
		"channel_id":  req.ChannelID,
		"message_id":  message.ID,
		"priority":    req.Priority,
		"title":       req.Title,
		"api":         "POST /api/v3/messages",
	}).Info("API: Message creation request")

	// 通过投递系统异步处理消息
	if h.deliverySystem != nil {
		err := h.deliverySystem.SubmitMessage(message, []string{userID})
		if err != nil {
			logger.WithFields(logrus.Fields{
				"user_id":    userID,
				"message_id": message.ID,
				"error":      err.Error(),
				"api":        "POST /api/v3/messages",
			}).Error("API: Failed to submit message to delivery system")
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    500,
				"message": "Failed to submit message to delivery system",
				"error":   err.Error(),
			})
			return
		}
	} else {
		logger.WithFields(logrus.Fields{
			"user_id": userID,
			"api":     "POST /api/v3/messages",
		}).Warn("API: Delivery system not available")
		// 降级处理：如果投递系统不可用，直接存储（兼容性）
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "Delivery system not available",
		})
		return
	}

	// 🎯 记录成功响应
	logger.WithFields(logrus.Fields{
		"user_id":    userID,
		"message_id": message.ID,
		"channel_id": req.ChannelID,
		"api":        "POST /api/v3/messages",
	}).Info("API: Message submitted successfully")

	// 立即返回响应（异步投递）
	c.JSON(http.StatusAccepted, gin.H{
		"code":    202,
		"message": "Message submitted for delivery",
		"data": gin.H{
			"message_id":   message.ID,
			"user_id":      userID,
			"channel_id":   message.ChannelID,
			"priority":     message.Priority,
			"submitted_at": time.Now(),
		},
	})
}

// GetMessages 获取消息列表
func (h *SimpleAPIHandler) GetMessages(c *gin.Context) {
	userID := middleware.GetUserID(c)
	channelID := c.DefaultQuery("channel_id", "default")
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 100 {
		limit = 20
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	// 获取用户工作空间
	ws, err := h.workspaceManager.GetUserWorkspace(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to get user workspace",
			"error":   err.Error(),
		})
		return
	}

	// 从用户工作空间获取消息
	userStorage := storage.NewUserMessageStorage(ws)
	messages, err := userStorage.GetMessages(channelID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to get messages",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"messages": messages,
			"limit":    limit,
			"offset":   offset,
		},
	})
}

// GetMessage 获取单条消息
func (h *SimpleAPIHandler) GetMessage(c *gin.Context) {
	userID := middleware.GetUserID(c)
	id := c.Param("id")

	// 获取用户工作空间
	ws, err := h.workspaceManager.GetUserWorkspace(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to get user workspace",
			"error":   err.Error(),
		})
		return
	}

	// 从用户工作空间获取消息
	userStorage := storage.NewUserMessageStorage(ws)
	message, err := userStorage.GetMessage(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "Message not found",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    message,
	})
}

// CreateChannel 创建频道
func (h *SimpleAPIHandler) CreateChannel(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request parameters",
			"error":   err.Error(),
		})
		return
	}

	// 获取用户工作空间
	ws, err := h.workspaceManager.GetUserWorkspace(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to get user workspace",
			"error":   err.Error(),
		})
		return
	}

	// 创建频道
	userStorage := storage.NewUserMessageStorage(ws)
	channel := &models.Channel{
		ID:          models.GenerateUUID(),
		Name:        req.Name,
		Description: req.Description,
		CreatedBy:   userID,
		CreatedAt:   time.Now(),
	}

	if err := userStorage.CreateChannel(channel); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to create channel",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Channel created successfully",
		"data":    channel,
	})
}

// GetChannel 获取频道
func (h *SimpleAPIHandler) GetChannel(c *gin.Context) {
	userID := middleware.GetUserID(c)
	id := c.Param("id")

	// 获取用户工作空间
	ws, err := h.workspaceManager.GetUserWorkspace(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to get user workspace",
			"error":   err.Error(),
		})
		return
	}

	// 从用户工作空间获取频道
	userStorage := storage.NewUserMessageStorage(ws)
	channel, err := userStorage.GetChannel(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "Channel not found",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    channel,
	})
}

// GetChannels 获取所有频道
func (h *SimpleAPIHandler) GetChannels(c *gin.Context) {
	userID := middleware.GetUserID(c)

	// 获取用户工作空间
	ws, err := h.workspaceManager.GetUserWorkspace(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to get user workspace",
			"error":   err.Error(),
		})
		return
	}

	// 从用户工作空间获取频道
	userStorage := storage.NewUserMessageStorage(ws)
	channels, err := userStorage.GetAllChannels()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to get channels",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    channels,
	})
}

// CreateMessagesBatch 批量创建消息（使用投递系统）
func (h *SimpleAPIHandler) CreateMessagesBatch(c *gin.Context) {
	var req struct {
		Messages []models.CreateMessageRequest `json:"messages" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request parameters",
			"error":   err.Error(),
		})
		return
	}

	// 从上下文获取用户ID
	userID := middleware.GetUserID(c)

	var submittedMessages []map[string]interface{}
	var errors []string

	// 🚀 通过投递系统批量处理消息
	for _, msgReq := range req.Messages {
		// 如果没有指定频道，使用默认频道
		if msgReq.ChannelID == "" {
			msgReq.ChannelID = "default"
		}

		// 设置默认值
		if msgReq.MessageType == "" {
			msgReq.MessageType = "text"
		}
		if msgReq.Priority == 0 {
			msgReq.Priority = 5
		}

		// 创建消息
		message := models.NewMessage(msgReq, userID)

		// 🎯 通过投递系统异步投递
		if h.deliverySystem != nil {
			err := h.deliverySystem.SubmitMessage(message, []string{userID})
			if err != nil {
				errors = append(errors, fmt.Sprintf("Failed to submit message %s: %v", message.ID, err))
				continue
			}

			// 记录提交成功的消息信息
			submittedMessages = append(submittedMessages, map[string]interface{}{
				"message_id":   message.ID,
				"user_id":      userID,
				"channel_id":   message.ChannelID,
				"priority":     message.Priority,
				"submitted_at": time.Now(),
			})
		} else {
			// 降级处理：如果投递系统不可用
			errors = append(errors, "Delivery system not available for message "+message.ID)
		}
	}

	response := gin.H{
		"code":    200,
		"message": "Batch submission completed",
		"data": gin.H{
			"submitted": submittedMessages,
			"count":     len(submittedMessages),
		},
	}

	if len(errors) > 0 {
		response["errors"] = errors
	}

	c.JSON(http.StatusAccepted, response)
}

// GetUserStats 获取用户统计信息
func (h *SimpleAPIHandler) GetUserStats(c *gin.Context) {
	userID := middleware.GetUserID(c)

	// 获取用户工作空间
	ws, err := h.workspaceManager.GetUserWorkspace(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to get user workspace",
			"error":   err.Error(),
		})
		return
	}

	// 从用户工作空间获取统计信息
	userStorage := storage.NewUserMessageStorage(ws)
	stats, err := userStorage.GetUserStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to get user stats",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    stats,
	})
}

// MarkAsRead 标记消息已读
func (h *SimpleAPIHandler) MarkAsRead(c *gin.Context) {
	userID := middleware.GetUserID(c)
	messageID := c.Param("id")

	// 获取用户工作空间
	ws, err := h.workspaceManager.GetUserWorkspace(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to get user workspace",
			"error":   err.Error(),
		})
		return
	}

	// 从用户工作空间标记已读
	userStorage := storage.NewUserMessageStorage(ws)
	err = userStorage.MarkAsRead(messageID, "api_client")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to mark as read",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Message marked as read",
	})
}

// GetUnreadCount 获取未读消息数量
func (h *SimpleAPIHandler) GetUnreadCount(c *gin.Context) {
	userID := middleware.GetUserID(c)
	channelID := c.Query("channel_id")

	// 获取用户工作空间
	ws, err := h.workspaceManager.GetUserWorkspace(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to get user workspace",
			"error":   err.Error(),
		})
		return
	}

	// 从用户工作空间获取未读数量
	userStorage := storage.NewUserMessageStorage(ws)
	count, err := userStorage.GetUnreadCount(channelID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to get unread count",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"unread_count": count,
		},
	})
}

// GetDeliveryStats 获取投递系统统计
func (h *SimpleAPIHandler) GetDeliveryStats(c *gin.Context) {
	if h.deliverySystem == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "Delivery system not available",
		})
		return
	}

	stats := h.deliverySystem.GetStats()
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"total_received":      stats.TotalReceived,
			"total_delivered":     stats.TotalDelivered,
			"total_failed":        stats.TotalFailed,
			"total_retried":       stats.TotalRetried,
			"avg_delivery_time":   stats.AvgDeliveryTime.String(),
			"queue_depth":         stats.QueueDepth,
			"active_workers":      stats.ActiveWorkers,
			"success_rate":        float64(stats.TotalDelivered) / float64(stats.TotalReceived) * 100,
			"failure_rate":        float64(stats.TotalFailed) / float64(stats.TotalReceived) * 100,
			"last_update":         stats.LastUpdate,
		},
	})
}

// GetWorkspaceCacheStats 获取工作空间缓存统计
func (h *SimpleAPIHandler) GetWorkspaceCacheStats(c *gin.Context) {
	if h.workspaceManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "Workspace manager not available",
		})
		return
	}

	stats := h.workspaceManager.GetCacheStats()
	size := h.workspaceManager.GetCacheSize()

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"cache_stats": stats,
			"cache_size": size,
			"active_workspaces": h.workspaceManager.ListWorkspaces(),
		},
	})
}