package ws

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // 同源部署，信任所有
	},
}

// ServeWS Gin handler，升级并注册连接
// 认证方式：通过 JWTAuth 中间件（支持 Authorization header 或 ?token=）
func ServeWS(hub *Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		uidRaw, exists := c.Get("uid")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "未认证"})
			return
		}
		userID, ok := uidRaw.(uint)
		if !ok || userID == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "未认证"})
			return
		}

		// 升级
		wsConn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}

		conn := NewConn(hub, userID, wsConn)
		hub.register <- conn

		go conn.WritePump()
		go conn.ReadPump()
	}
}

// TokenFromWS 从 WebSocket 握手的 query/header 提取 token（供 auth middleware 使用）
func TokenFromWS(r *http.Request) string {
	if q := r.URL.Query().Get("token"); q != "" {
		return q
	}
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return ""
}
