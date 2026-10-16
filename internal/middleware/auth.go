package middleware

import (
	"huozhi/internal/database"
	"huozhi/pkg/jwt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// tokenVersionMatches 校验 token 里的 tv 是否仍是该用户的当前版本。
//
// 老 token 没有 tv 字段（解析为 0），存量用户 token_version 也是 0，
// 所以升级后已有会话不会被误踢，只有改过密码才失效。
//
// 查不到用户（已注销）或版本对不上都返回 false —— 失败即拒绝。
func tokenVersionMatches(userID uint, tv int) bool {
	if database.DB == nil {
		return false
	}
	var ver int
	err := database.DB.Raw("SELECT token_version FROM users WHERE id = ?", userID).Row().Scan(&ver)
	return err == nil && ver == tv
}

// JWTAuth JWT鉴权中间件（支持 Authorization header 或 ?token= query 参数）
func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		var tokenStr string
		if authHeader := c.GetHeader("Authorization"); authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if !(len(parts) == 2 && parts[0] == "Bearer") {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization header format must be Bearer {token}"})
				c.Abort()
				return
			}
			tokenStr = parts[1]
		} else if qt := c.Query("token"); qt != "" {
			tokenStr = qt
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
			c.Abort()
			return
		}

		claims, err := jwt.ParseToken(tokenStr)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			c.Abort()
			return
		}

		// 改密码后 User.TokenVersion+1，旧 token 在此被拒。
		// 走主键索引单行查询，开销可忽略；换来的是「改密码即踢掉其它端」。
		if !tokenVersionMatches(claims.UserID, claims.TokenVersion) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "session expired, please login again"})
			c.Abort()
			return
		}

		c.Set("uid", claims.UserID)
		c.Set("username", claims.Username)
		c.Next()
	}
}

// GetUID 从context获取用户ID
func GetUID(c *gin.Context) uint {
	v, ok := c.Get("uid")
	if !ok {
		return 0
	}
	return v.(uint)
}
