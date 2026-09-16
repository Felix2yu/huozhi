package mcp

import (
	"bufio"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// 本文件实现 MCP 的 Streamable HTTP 传输（2025-06-18 起取代早期的 HTTP+SSE 双端点方案）：
//
//   - POST   /mcp  客户端 → 服务端。Accept 含 text/event-stream 时以 SSE 回送响应，
//     否则直接返回 JSON。通知（无 id）回 202 且无响应体。
//   - GET    /mcp  服务端 → 客户端的 SSE 长连接，用于服务端主动推送（当前仅心跳）。
//   - DELETE /mcp  显式结束会话。
//
// 鉴权交给上游中间件完成：本文件只从 gin 上下文读取 uid，不关心它是来自
// API Key 还是 JWT——两种接入方式共用同一套工具实现。

// SessionHeader 会话 ID 的 HTTP 头名（规范要求）
const SessionHeader = "Mcp-Session-Id"

// sseKeepAlive 心跳间隔：小于常见代理的 60s 空闲超时，避免连接被中间层静默断开
const sseKeepAlive = 25 * time.Second

// HandleHTTP MCP 传输入口，挂在 /mcp 与 /api/mcp 上。
func (s *Server) HandleHTTP(c *gin.Context) {
	switch c.Request.Method {
	case http.MethodPost:
		s.handlePost(c)
	case http.MethodGet:
		s.handleGet(c)
	case http.MethodDelete:
		s.handleDelete(c)
	default:
		c.Header("Allow", "GET, POST, DELETE")
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "仅支持 GET / POST / DELETE"})
	}
}

func (s *Server) handlePost(c *gin.Context) {
	uid := uidOf(c)
	if uid == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	raw, err := c.GetRawData()
	if err != nil {
		writeJSON(c, http.StatusBadRequest, Response{
			JSONRPC: jsonrpcVersion,
			Error:   &Error{Code: CodeParseError, Message: "读取请求体失败"},
		})
		return
	}

	// 会话解析：initialize 一定建新会话；其余请求沿用客户端带来的 ID，
	// 缺失或已过期（进程重启等）时重建，避免客户端卡在 404 上不可恢复。
	sessID := c.GetHeader(SessionHeader)
	if sessID == "" {
		sessID = c.Query("session_id")
	}
	sess := s.TouchSession(sessID, uid)
	c.Header(SessionHeader, sess.ID)

	payload, hasResponse, err := s.HandleBytes(c.Request.Context(), uid, raw)
	if err != nil {
		// 只有 JSON 解析失败才会走到这里，无法回带 id
		writeJSON(c, http.StatusBadRequest, Response{
			JSONRPC: jsonrpcVersion,
			Error:   &Error{Code: CodeParseError, Message: err.Error()},
		})
		return
	}
	if !hasResponse {
		c.Status(http.StatusAccepted)
		return
	}

	if wantsSSE(c) {
		writeSSEPayload(c, payload)
		return
	}
	writeJSON(c, http.StatusOK, json.RawMessage(payload))
}

func (s *Server) handleGet(c *gin.Context) {
	uid := uidOf(c)
	if uid == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}
	if !wantsSSE(c) {
		// 本服务端没有需要轮询的内容，按规范回 405
		c.Header("Allow", "POST, DELETE")
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "本端点仅用于 SSE 流，请携带 Accept: text/event-stream"})
		return
	}

	sessID := c.GetHeader(SessionHeader)
	if sessID == "" {
		sessID = c.Query("session_id")
	}
	sess := s.TouchSession(sessID, uid)
	c.Header(SessionHeader, sess.ID)

	ch, unregister := sess.addStream()
	if ch == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "会话已关闭"})
		return
	}
	defer unregister()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache, no-transform")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no") // 关闭 nginx 缓冲，否则事件会被攒着不下发
	c.Status(http.StatusOK)

	w := c.Writer
	bw := bufio.NewWriter(w)
	flush := func() {
		_ = bw.Flush()
		w.Flush()
	}
	// 先发一个注释帧，让客户端立刻确认连接已建立
	_, _ = bw.WriteString(": stream opened\n\n")
	flush()

	ticker := time.NewTicker(sseKeepAlive)
	defer ticker.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			writeSSEFrame(bw, "message", msg)
			flush()
		case <-ticker.C:
			_, _ = bw.WriteString(": keepalive\n\n")
			flush()
		}
	}
}

func (s *Server) handleDelete(c *gin.Context) {
	sessID := c.GetHeader(SessionHeader)
	if sessID == "" {
		sessID = c.Query("session_id")
	}
	if sessID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 " + SessionHeader})
		return
	}
	s.DropSession(sessID)
	c.Status(http.StatusNoContent)
}

// ==================== 辅助 ====================

func wantsSSE(c *gin.Context) bool {
	accept := c.GetHeader("Accept")
	if accept == "" {
		return false
	}
	return containsToken(accept, "text/event-stream")
}

func containsToken(header, token string) bool {
	// 简单扫描：Accept 形如 "application/json, text/event-stream"
	n, m := len(header), len(token)
	for i := 0; i+m <= n; i++ {
		if header[i:i+m] == token {
			// 确认落在分隔符边界上，避免 "text/event-stream-x" 被误判
			end := i + m
			if (i == 0 || header[i-1] == ',' || header[i-1] == ' ') &&
				(end == n || header[end] == ',' || header[end] == ';' || header[end] == ' ') {
				return true
			}
		}
	}
	return false
}

func writeSSEPayload(c *gin.Context, payload []byte) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache, no-transform")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	w := c.Writer
	bw := bufio.NewWriter(w)
	writeSSEFrame(bw, "message", payload)
	_ = bw.Flush()
	w.Flush()
}

func writeSSEFrame(bw *bufio.Writer, event string, data []byte) {
	_, _ = bw.WriteString("event: ")
	_, _ = bw.WriteString(event)
	_, _ = bw.WriteString("\n")
	for _, line := range splitLines(data) {
		_, _ = bw.WriteString("data: ")
		_, _ = bw.Write(line)
		_, _ = bw.WriteString("\n")
	}
	_, _ = bw.WriteString("\n")
}

func splitLines(b []byte) [][]byte {
	out := make([][]byte, 0, 1)
	start := 0
	for i := 0; i < len(b); i++ {
		if b[i] == '\n' {
			line := b[start:i]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			out = append(out, line)
			start = i + 1
		}
	}
	if start < len(b) {
		out = append(out, b[start:])
	}
	return out
}

func writeJSON(c *gin.Context, status int, v any) {
	c.JSON(status, v)
}

func uidOf(c *gin.Context) uint {
	v, ok := c.Get("uid")
	if !ok {
		return 0
	}
	switch x := v.(type) {
	case uint:
		return x
	case int:
		if x > 0 {
			return uint(x)
		}
	case int64:
		if x > 0 {
			return uint(x)
		}
	case float64:
		if x > 0 {
			return uint(x)
		}
	}
	return 0
}
