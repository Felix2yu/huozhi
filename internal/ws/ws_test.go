package ws

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func TestHubBasics(t *testing.T) {
	h := NewHub()
	if h.Count() != 0 {
		t.Fatal("new hub should have 0 clients")
	}
	go h.Run()
	h.Broadcast(1, "transactions", "create", 5)
	h.BroadcastWithData(1, Message{Type: "alert", Table: "budgets"})
	time.Sleep(50 * time.Millisecond)
}

func TestTokenFromWS(t *testing.T) {
	req := httptest.NewRequest("GET", "/ws?token=abc", nil)
	if TokenFromWS(req) != "abc" {
		t.Fatal("query token extraction failed")
	}
	req2 := httptest.NewRequest("GET", "/ws", nil)
	req2.Header.Set("Authorization", "Bearer xyz")
	if TokenFromWS(req2) != "xyz" {
		t.Fatal("header token extraction failed")
	}
	if TokenFromWS(httptest.NewRequest("GET", "/ws", nil)) != "" {
		t.Fatal("empty token should be empty")
	}
}

func TestSafeLog(t *testing.T) {
	SafeLog("hello %s", "ws")
}

func TestServeWSNoUID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHub()
	r := gin.New()
	r.GET("/ws", ServeWS(h))
	srv := httptest.NewServer(r)
	defer srv.Close()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	_, resp, err := websocket.DefaultDialer.Dial(url, nil)
	if err == nil {
		t.Fatal("expected dial error (401)")
	}
	if resp != nil && resp.StatusCode != 401 {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestServeWSUIDZero(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHub()
	r := gin.New()
	r.GET("/ws", func(c *gin.Context) { c.Set("uid", 0) }, ServeWS(h))
	srv := httptest.NewServer(r)
	defer srv.Close()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	_, resp, err := websocket.DefaultDialer.Dial(url, nil)
	if err == nil {
		t.Fatal("expected dial error")
	}
	if resp != nil && resp.StatusCode != 401 {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestServeWSAndHub(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHub()
	go h.Run()
	r := gin.New()
	r.GET("/ws", func(c *gin.Context) { c.Set("uid", uint(1)) }, ServeWS(h))
	srv := httptest.NewServer(r)
	defer srv.Close()

	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	time.Sleep(100 * time.Millisecond)
	if h.Count() != 1 {
		t.Fatalf("expected 1 client, got %d", h.Count())
	}

	// broadcast a sync message and verify the client receives it
	h.Broadcast(1, "transactions", "create", 9)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read broadcast: %v", err)
	}
	if !strings.Contains(string(msg), "transactions") {
		t.Fatalf("unexpected message %s", msg)
	}

	conn.Close()
	time.Sleep(100 * time.Millisecond)
	if h.Count() != 0 {
		t.Fatalf("expected 0 after close, got %d", h.Count())
	}
}
