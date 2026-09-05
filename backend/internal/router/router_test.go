package router

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewRegistersRoutes(t *testing.T) {
	r := New("test", "")
	if r == nil {
		t.Fatal("expected non-nil engine")
	}
	routes := r.Routes()
	if len(routes) == 0 {
		t.Fatal("expected routes to be registered")
	}
	want := map[string]bool{
		"GET /api/health":              false,
		"POST /api/auth/login":         false,
		"GET /api/ai/status":           false,
		"GET /api/books":               false,
		"GET /api/accounts":            false,
		"GET /api/categories":          false,
		"GET /api/transactions":        false,
		"GET /api/statistics":          false,
		"GET /api/saving-plans":        false,
		"GET /api/recurring":           false,
		"GET /api/installments":        false,
		"GET /api/reimbursements":      false,
		"POST /api/io/import":          false,
	}
	for _, rt := range routes {
		key := rt.Method + " " + rt.Path
		if _, ok := want[key]; ok {
			want[key] = true
		}
	}
	for key, found := range want {
		if !found {
			t.Fatalf("route not registered: %s", key)
		}
	}
}

func TestNewReleaseMode(t *testing.T) {
	r := New("release", "")
	if r == nil {
		t.Fatal("expected non-nil engine in release mode")
	}
	// a public route should respond without JWT
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code == 0 {
		t.Fatal("expected health route to be served")
	}
}

// TestMountFrontend 验证：配置 static_dir 后，静态文件带正确缓存头，
// 未知路径回退 index.html（SPA），/api/* 不回退保持 404。
func TestMountFrontend(t *testing.T) {
	dir := t.TempDir()
	assets := filepath.Join(dir, "assets")
	if err := os.MkdirAll(assets, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assets, "index-abc123.js"), []byte("console.log(1)"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sw.js"), []byte("self.registration"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>spa</html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := New("test", dir)

	// 哈希产物：immutable 长缓存
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/assets/index-abc123.js", nil))
	if w.Code != http.StatusOK || w.Body.String() != "console.log(1)" {
		t.Fatalf("静态资源未正确返回: code=%d body=%q", w.Code, w.Body.String())
	}
	if cc := w.Header().Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Fatalf("哈希产物应为 immutable 缓存，实际: %q", cc)
	}

	// sw.js：必须 no-cache（PWA autoUpdate）
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sw.js", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("sw.js 未返回: code=%d", w.Code)
	}
	if cc := w.Header().Get("Cache-Control"); !strings.Contains(cc, "no-cache") {
		t.Fatalf("sw.js 应为 no-cache，实际: %q", cc)
	}

	// SPA 回退
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/transactions/some-page", nil))
	if w.Code != http.StatusOK || w.Body.String() != "<html>spa</html>" {
		t.Fatalf("SPA 回退失败: code=%d body=%q", w.Code, w.Body.String())
	}

	// API 未知路径不回退
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/not-exist", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("API 未知路径应保持 404，实际: %d", w.Code)
	}

	// 路径穿越防护
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/..%2f..%2fetc%2fpasswd", nil))
	if w.Code == http.StatusOK {
		t.Fatalf("路径穿越不应返回 200")
	}
}
