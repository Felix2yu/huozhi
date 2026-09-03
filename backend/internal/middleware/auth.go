package middleware

import (
	"huozhi/pkg/jwt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

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
