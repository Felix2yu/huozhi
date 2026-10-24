package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"huozhi/internal/database"
	"huozhi/internal/models"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// 上下文键：鉴权通过后由中间件注入，供下游判断权限。
const (
	// CtxScope 当前请求的权限范围（models.ScopeRead / models.ScopeWrite）。
	CtxScope = "scope"
	// CtxAPIKeyID 当前请求使用的密钥 id（JWT 请求为空），便于日志与「最后使用时间」。
	CtxAPIKeyID = "api_key_id"
)

// APIKeyAuth 对外只读接口的鉴权（X-API-Key 或 ?api_key=）。
//
// 只读与读写密钥都放行：这组接口本身只提供读能力，真正的分权点在 MCP
// （写工具只对读写密钥开放）。
func APIKeyAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := apiKeyFromRequest(c)
		if raw == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "API key required"})
			c.Abort()
			return
		}
		k, username := ResolveAPIKey(raw)
		if k == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or disabled API key"})
			c.Abort()
			return
		}
		c.Set("uid", k.UserID)
		c.Set("username", username)
		c.Set(CtxScope, k.Scope)
		c.Set(CtxAPIKeyID, k.ID)
		c.Next()
	}
}

// apiKeyFromRequest 取请求里的明文密钥。
// 请求头优先且不回退：带了无效请求头就不该再拿查询参数兜底，
// 否则「客户端以为自己在用 A 密钥」的错觉会一直存在。
func apiKeyFromRequest(c *gin.Context) string {
	if key := strings.TrimSpace(c.GetHeader("X-API-Key")); key != "" {
		return key
	}
	return strings.TrimSpace(c.Query("api_key"))
}

// ResolveAPIKey 按明文密钥查一把可用密钥，同时返回所属账号的用户名（供日志/上下文）。
// 查不到（未生成 / 已停用 / 已吊销 / 用户被禁用）返回 nil。
//
// 以 api_keys 表为准；只有「这把 key 从未进过 api_keys 表」时才回落到历史的
// users.api_key 字段（迁移前建的库、或迁移尚未执行）。一旦表里出现过这把 key
// （包括已吊销的软删除行），就不再回落——否则吊销只是把列表里的行藏起来，
// 鉴权仍会从 users 表把它读回来。
func ResolveAPIKey(raw string) (*models.APIKey, string) {
	raw = strings.TrimSpace(raw)
	if !ValidateAPIKey(raw) {
		return nil, ""
	}
	// 用户名用子查询一次带出来：鉴权是热路径，不值得为了一行 username 再查一次库。
	var row struct {
		models.APIKey
		Username string
	}
	err := database.DB.Model(&models.APIKey{}).
		Select("api_keys.*, (SELECT username FROM users WHERE users.id = api_keys.user_id) AS username").
		Where("api_keys.key = ? AND api_keys.enabled = ?", raw, true).
		Where("EXISTS (SELECT 1 FROM users WHERE users.id = api_keys.user_id AND users.status = 1 AND users.deleted_at IS NULL)").
		First(&row).Error
	if err == nil {
		touchAPIKey(row.ID, row.LastUsedAt)
		return &row.APIKey, row.Username
	}
	if err != gorm.ErrRecordNotFound {
		return nil, ""
	}
	return legacyAPIKey(raw)
}

// legacyAPIKey 从 users.api_key 兜底。仅用于尚未迁移的库。
func legacyAPIKey(raw string) (*models.APIKey, string) {
	var seen int64
	if err := database.DB.Unscoped().Model(&models.APIKey{}).
		Where("key = ?", raw).Count(&seen).Error; err != nil || seen > 0 {
		return nil, ""
	}
	var u models.User
	if err := database.DB.
		Where("api_key = ? AND api_key_enabled = ? AND status = 1", raw, true).
		First(&u).Error; err != nil {
		return nil, ""
	}
	return &models.APIKey{
		UserID: u.ID, Key: raw, Scope: models.ScopeWrite, Enabled: true, Legacy: true,
	}, u.Username
}

// touchAPIKey 记录密钥最后使用时间，供设置页判断「这把还在用」。
// 节流到 1 分钟：鉴权是每个请求都要走的热路径，没必要每次都写库。
func touchAPIKey(id uint, last time.Time) {
	if id == 0 {
		return
	}
	now := time.Now()
	if !last.IsZero() && now.Sub(last) < time.Minute {
		return
	}
	_ = database.DB.Model(&models.APIKey{}).
		Where("id = ? AND (last_used_at IS NULL OR last_used_at < ?)", id, now.Add(-time.Minute)).
		Update("last_used_at", now).Error
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
