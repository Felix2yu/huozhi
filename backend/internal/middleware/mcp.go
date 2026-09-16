package middleware

import (
	"huozhi/internal/database"
	"huozhi/internal/models"
	"huozhi/pkg/jwt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// MCPAuth MCP 端点专用鉴权。
//
// 为什么不用现成的 JWTAuth / APIKeyAuth：MCP 客户端形态差异很大——
// Claude Desktop、Cursor、Cherry Studio 之类的桌面客户端通常只允许配一个
// Authorization: Bearer <token>，没法自定义 X-API-Key 头。这里一次兼容三种：
//
//  1. Authorization: Bearer <JWT 或 API Key>
//  2. X-API-Key: <API Key>
//  3. ?api_key=<API Key>（便于浏览器 / 网关调试）
//
// 判定顺序是先 JWT 后 API Key：JWT 是本地可验签的，不需要查库，命中就返回。
func MCPAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if uid := uidFromJWT(c); uid > 0 {
			c.Set("uid", uid)
			c.Set("auth_type", "jwt")
			c.Next()
			return
		}
		if uid := uidFromAPIKey(c); uid > 0 {
			c.Set("uid", uid)
			c.Set("auth_type", "api_key")
			c.Next()
			return
		}
		// 401 时带上 WWW-Authenticate，符合 MCP 规范对鉴权失败的要求
		c.Header("WWW-Authenticate", `Bearer realm="huozhi-mcp"`)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "MCP 端点需要鉴权：请在请求头带 Authorization: Bearer <API密钥或JWT>，或 X-API-Key: <API密钥>",
		})
		c.Abort()
	}
}

// uidFromJWT 从 Authorization: Bearer 或 ?token= 里解析 JWT
func uidFromJWT(c *gin.Context) uint {
	var tokenStr string
	if h := c.GetHeader("Authorization"); h != "" {
		parts := strings.SplitN(h, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			tokenStr = strings.TrimSpace(parts[1])
		}
	}
	if tokenStr == "" {
		tokenStr = c.Query("token")
	}
	if tokenStr == "" {
		return 0
	}
	claims, err := jwt.ParseToken(tokenStr)
	if err != nil {
		return 0
	}
	return claims.UserID
}

// uidFromAPIKey 从 X-API-Key / Authorization: Bearer / ?api_key= 里解析 API Key
func uidFromAPIKey(c *gin.Context) uint {
	key := strings.TrimSpace(c.GetHeader("X-API-Key"))
	if key == "" {
		if h := c.GetHeader("Authorization"); h != "" {
			parts := strings.SplitN(h, " ", 2)
			if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
				key = strings.TrimSpace(parts[1])
			}
		}
	}
	if key == "" {
		key = strings.TrimSpace(c.Query("api_key"))
	}
	if key == "" || !ValidateAPIKey(key) {
		return 0
	}
	var user models.User
	if err := database.DB.Where("api_key = ? AND api_key_enabled = ? AND status = ?", key, true, 1).
		First(&user).Error; err != nil {
		return 0
	}
	return user.ID
}
