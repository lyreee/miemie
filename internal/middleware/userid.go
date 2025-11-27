package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// UserIDMiddleware 从请求头中提取用户ID并设置到上下文
func UserIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从Header中获取User-ID
		userID := c.GetHeader("User-ID")

		// 如果没有User-ID，使用默认值
		if userID == "" {
			userID = "default"
		}

		// 清理和验证用户ID
		userID = strings.TrimSpace(userID)
		if userID == "" {
			userID = "default"
		}

		// 设置到上下文
		c.Set("user_id", userID)

		// 在响应头中返回用户ID
		c.Header("X-User-ID", userID)

		c.Next()
	}
}

// RequireUserID 确保请求包含有效的用户ID
func RequireUserID() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		if userID == "" || userID == "default" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "User-ID header is required",
				"error":   "Missing or invalid User-ID",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// GetUserID 从上下文中获取用户ID
func GetUserID(c *gin.Context) string {
	return c.GetString("user_id")
}