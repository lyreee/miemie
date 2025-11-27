package main

import (
	"miemie/internal/api"
	"miemie/internal/config"
	"miemie/internal/logger"
	"miemie/internal/websocket"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

func main() {
	// 加载配置
	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("Failed to load config: %v", err)
	}

	// 初始化日志系统
	if err := logger.InitLogger(&cfg.Logging); err != nil {
		logger.Fatalf("Failed to initialize logger: %v", err)
	}

	// 确保用户存储目录存在
	if err := os.MkdirAll(cfg.Server.UserStorage, 0755); err != nil {
		logger.Fatalf("Failed to create user storage directory: %v", err)
	}

	logger.Info("User storage directory created successfully")

	// 初始化WebSocket管理器
	wsManager := websocket.NewManager()

	// 创建Gin路由
	r := gin.Default()

	// 启用CORS
	if cfg.API.CORS.Enabled {
		r.Use(func(c *gin.Context) {
			origin := c.Request.Header.Get("Origin")
			if len(cfg.API.CORS.AllowedOrigins) == 0 || cfg.API.CORS.AllowedOrigins[0] == "*" {
				c.Header("Access-Control-Allow-Origin", "*")
			} else {
				allowed := false
				for _, allowedOrigin := range cfg.API.CORS.AllowedOrigins {
					if allowedOrigin == origin {
						allowed = true
						break
					}
				}
				if allowed {
					c.Header("Access-Control-Allow-Origin", origin)
				}
			}

			methods := ""
			if len(cfg.API.CORS.AllowedMethods) > 0 {
				methods = strings.Join(cfg.API.CORS.AllowedMethods, ", ")
			} else {
				methods = "GET, POST, PUT, DELETE, OPTIONS"
			}
			c.Header("Access-Control-Allow-Methods", methods)

			headers := ""
			if len(cfg.API.CORS.AllowedHeaders) > 0 {
				headers = strings.Join(cfg.API.CORS.AllowedHeaders, ", ")
			} else {
				headers = "Content-Type, Authorization"
			}
			c.Header("Access-Control-Allow-Headers", headers)

			if c.Request.Method == "OPTIONS" {
				c.AbortWithStatus(204)
				return
			}

			c.Next()
		})
	}

	// 设置路由
	api.SetupSimpleRoutes(r, cfg, wsManager)

	// WebSocket路由
	r.GET("/ws", wsManager.HandleWebSocket)

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
			"message": "Message service is running",
		})
	})

	// 启动服务器
	logger.Infof("Starting server on port %s", cfg.Server.Port)
	if err := r.Run(":" + cfg.Server.Port); err != nil {
		logger.Fatalf("Failed to start server: %v", err)
	}
}