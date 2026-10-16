package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"huozhi/internal/database"
	"huozhi/internal/models"
	"net/http"

	"github.com/gin-gonic/gin"
)

// APIKeyAuth API密钥鉴权中间件（支持 X-API-Key header 或 ?api_key= query 参数）
func APIKeyAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		var apiKey string
		if key := c.GetHeader("X-API-Key"); key != "" {
			apiKey = key
		} else if key := c.Query("api_key"); key != "" {
			apiKey = key
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "API key required"})
			c.Abort()
			return
		}

		// 验证API key
		var user models.User
		if err := database.DB.Where("api_key = ? AND api_key_enabled = ? AND status = ?", apiKey, true, 1).First(&user).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or disabled API key"})
			c.Abort()
			return
		}

		// 设置用户ID到上下文
		c.Set("uid", user.ID)
		c.Set("username", user.Username)
		c.Next()
	}
}

// ValidateAPIKey 验证API key格式（64位十六进制字符串）
func ValidateAPIKey(key string) bool {
	if len(key) != 64 {
		return false
	}
	// 检查是否为有效的十六进制字符串
	for _, c := range key {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// GenerateAPIKey 生成随机API key（64位十六进制字符串）
func GenerateAPIKey() string {
	b := make([]byte, 32) // 32字节 = 64位十六进制
	if _, err := rand.Read(b); err != nil {
		// 如果加密随机数失败，使用备用方案
		const chars = "0123456789abcdef"
		result := make([]byte, 64)
		for i := range result {
			result[i] = chars[i%len(chars)]
		}
		return string(result)
	}
	return hex.EncodeToString(b)
}