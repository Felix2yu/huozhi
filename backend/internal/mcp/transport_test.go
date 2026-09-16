package mcp

import (
	"bytes"
	"huozhi/internal/database"
	"huozhi/internal/middleware"
	"huozhi/internal/models"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// newMCPEngine 搭一个只挂 MCP 端点的最小 gin 引擎（含鉴权中间件）
func newMCPEngine(t *testing.T) (*gin.Engine, *Server) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	s := newTestServer(t)
	r := gin.New()
	h := func(c *gin.Context) { s.HandleHTTP(c) }
	r.GET("/mcp", middleware.MCPAuth(), h)
	r.POST("/mcp", middleware.MCPAuth(), h)
	r.DELETE("/mcp", middleware.MCPAuth(), h)
	return r, s
}

func apiKeyOf(t *testing.T, uid uint) string {
	t.Helper()
	var u models.User
	if err := database.DB.First(&u, uid).Error; err != nil {
		t.Fatal(err)
	}
	if u.APIKey == nil {
		t.Fatal("user has no api key")
	}
	return *u.APIKey
}

func TestMCPHTTPUnauthorized(t *testing.T) {
	r, _ := newMCPEngine(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/mcp", bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("未带凭据应 401，got %d", w.Code)
	}
	if w.Header().Get("WWW-Authenticate") == "" {
		t.Error("401 应带 WWW-Authenticate 头")
	}
}

func TestMCPHTTPInitializeAndSession(t *testing.T) {
	r, _ := newMCPEngine(t)
	uid, _ := newTestUser(t, "u_http")
	key := apiKeyOf(t, uid)

	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","clientInfo":{"name":"t","version":"1"}}}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/mcp", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", key)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	if w.Header().Get(SessionHeader) == "" {
		t.Error("initialize 应返回 Mcp-Session-Id")
	}
	if !strings.Contains(w.Body.String(), `"protocolVersion":"2025-06-18"`) {
		t.Errorf("响应缺少协议版本: %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "search_transactions") || !strings.Contains(w.Body.String(), "instructions") {
		t.Errorf("initialize 应包含工具清单说明: %s", w.Body.String())
	}
}

func TestMCPHTTPSSEStreamResponse(t *testing.T) {
	r, _ := newMCPEngine(t)
	uid, _ := newTestUser(t, "u_sse")
	key := apiKeyOf(t, uid)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/mcp", bytes.NewBufferString(`{"jsonrpc":"2.0","id":7,"method":"tools/list"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", key)
	req.Header.Set("Accept", "application/json, text/event-stream")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("code=%d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Accept 含 text/event-stream 时应回 SSE，got %s", ct)
	}
	if !strings.Contains(w.Body.String(), "event: message") {
		t.Errorf("SSE 缺少 event 帧: %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "data: ") {
		t.Errorf("SSE 缺少 data 帧: %s", w.Body.String())
	}
}

func TestMCPHTTPNotificationReturns202(t *testing.T) {
	r, _ := newMCPEngine(t)
	uid, _ := newTestUser(t, "u_notif")
	key := apiKeyOf(t, uid)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/mcp",
		bytes.NewBufferString(`{"jsonrpc":"2.0","method":"notifications/initialized"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", key)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("通知应回 202，got %d", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Errorf("通知不应有响应体，got %q", w.Body.String())
	}
}

func TestMCPHTTPBatchRequest(t *testing.T) {
	r, _ := newMCPEngine(t)
	uid, _ := newTestUser(t, "u_batch_http")
	key := apiKeyOf(t, uid)

	body := `[{"jsonrpc":"2.0","id":1,"method":"ping"},{"jsonrpc":"2.0","id":2,"method":"tools/list"}]`
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/mcp", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", key)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("code=%d", w.Code)
	}
	if !strings.HasPrefix(strings.TrimSpace(w.Body.String()), "[") {
		t.Errorf("批量请求应回数组，got %s", w.Body.String())
	}
}

func TestMCPHTTPDeleteSession(t *testing.T) {
	r, s := newMCPEngine(t)
	uid, _ := newTestUser(t, "u_del")
	key := apiKeyOf(t, uid)

	// 先建会话
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/mcp", bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`))
	req.Header.Set("X-API-Key", key)
	r.ServeHTTP(w, req)
	sid := w.Header().Get(SessionHeader)
	if sid == "" {
		t.Fatal("未拿到会话 ID")
	}
	if s.GetSession(sid) == nil {
		t.Fatal("会话未登记")
	}

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("DELETE", "/mcp", nil)
	req2.Header.Set("X-API-Key", key)
	req2.Header.Set(SessionHeader, sid)
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusNoContent {
		t.Fatalf("DELETE 应回 204，got %d", w2.Code)
	}
	if s.GetSession(sid) != nil {
		t.Error("DELETE 后会话应被移除")
	}
}

func TestMCPHTTPGetWithoutSSEAccept(t *testing.T) {
	r, _ := newMCPEngine(t)
	uid, _ := newTestUser(t, "u_get")
	key := apiKeyOf(t, uid)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/mcp", nil)
	req.Header.Set("X-API-Key", key)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET 且未声明 SSE 时应 405，got %d", w.Code)
	}
}

func TestMCPHTTPBadJSON(t *testing.T) {
	r, _ := newMCPEngine(t)
	uid, _ := newTestUser(t, "u_badjson")
	key := apiKeyOf(t, uid)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/mcp", bytes.NewBufferString(`{not json`))
	req.Header.Set("X-API-Key", key)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("非法 JSON 应 400，got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "-32700") {
		t.Errorf("应返回解析错误码: %s", w.Body.String())
	}
}
