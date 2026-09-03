package middleware

import (
	"net/http/httptest"
	"testing"

	"huozhi/internal/config"
	"huozhi/pkg/jwt"

	"github.com/gin-gonic/gin"
)

func init() {
	config.AppConfig = &config.Config{
		JWT: config.JWTConfig{Secret: "mw-secret", ExpireHours: 168, Issuer: "huozhi"},
	}
	gin.SetMode(gin.TestMode)
}

func TestJWTAuthBearer(t *testing.T) {
	tok, _ := jwt.GenerateToken(7, "bob")
	r := gin.New()
	r.GET("/x", JWTAuth(), func(c *gin.Context) { c.JSON(200, gin.H{"uid": GetUID(c)}) })
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("code %d", w.Code)
	}
}

func TestJWTAuthQuery(t *testing.T) {
	tok, _ := jwt.GenerateToken(9, "carol")
	r := gin.New()
	r.GET("/x", JWTAuth(), func(c *gin.Context) { c.JSON(200, gin.H{"ok": 1}) })
	req := httptest.NewRequest("GET", "/x?token="+tok, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("code %d", w.Code)
	}
}

func TestJWTAuthMissing(t *testing.T) {
	r := gin.New()
	r.GET("/x", JWTAuth())
	req := httptest.NewRequest("GET", "/x", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestJWTAuthBadFormat(t *testing.T) {
	r := gin.New()
	r.GET("/x", JWTAuth())
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Authorization", "Token abc")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestJWTAuthInvalid(t *testing.T) {
	r := gin.New()
	r.GET("/x", JWTAuth())
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestGetUIDMissing(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	if GetUID(c) != 0 {
		t.Fatal("expected 0 when uid absent")
	}
}
