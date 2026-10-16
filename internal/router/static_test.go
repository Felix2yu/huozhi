package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"
)

// TestMountFrontendFS 覆盖 fs.FS 形态的产物来源（go:embed 走的就是这条路）：
// 缓存头按类型分流、SPA 回退、路径穿越 400、Content-Type 推断。
func TestMountFrontendFS(t *testing.T) {
	fsys := fstest.MapFS{
		"index.html":             &fstest.MapFile{Data: []byte("<html>spa</html>")},
		"assets/index-abc123.js": &fstest.MapFile{Data: []byte("console.log(1)")},
		"sw.js":                  &fstest.MapFile{Data: []byte("self.registration")},
		"_app/version.json":      &fstest.MapFile{Data: []byte(`{"v":1}`)},
	}
	r := gin.New()
	mountFrontend(r, fsys)

	// 哈希产物：immutable 长缓存 + 正确 Content-Type
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/assets/index-abc123.js", nil))
	if w.Code != http.StatusOK || w.Body.String() != "console.log(1)" {
		t.Fatalf("静态资源未正确返回: code=%d body=%q", w.Code, w.Body.String())
	}
	if cc := w.Header().Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Fatalf("哈希产物应为 immutable 缓存，实际: %q", cc)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "javascript") {
		t.Fatalf("js 资源 Content-Type 异常: %q", ct)
	}

	// sw.js：必须 no-store（PWA autoUpdate）
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sw.js", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("sw.js 未返回: code=%d", w.Code)
	}
	if cc := w.Header().Get("Cache-Control"); !strings.Contains(cc, "no-store") {
		t.Fatalf("sw.js 应为 no-store，实际: %q", cc)
	}

	// _app/version.json：短缓存
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/_app/version.json", nil))
	if cc := w.Header().Get("Cache-Control"); !strings.Contains(cc, "max-age=300") {
		t.Fatalf("_app/version.json 应短缓存，实际: %q", cc)
	}

	// SPA 回退
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/transactions/some-page", nil))
	if w.Code != http.StatusOK || w.Body.String() != "<html>spa</html>" {
		t.Fatalf("SPA 回退失败: code=%d body=%q", w.Code, w.Body.String())
	}

	// /api/* 不回退
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/not-exist", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("API 未知路径应保持 404，实际: %d", w.Code)
	}

	// 路径穿越：400，而不是被当成 SPA 回退返回 200
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/..%2f..%2fetc%2fpasswd", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("路径穿越应返回 400，实际: %d", w.Code)
	}
}

// TestMountFrontendMissingIndex 覆盖产物缺失（dist 下只有占位文件）时的兜底：
// 不能 500，也不能把 index.html 当空文件返回 200。
func TestMountFrontendMissingIndex(t *testing.T) {
	r := gin.New()
	mountFrontend(r, fstest.MapFS{".gitkeep": &fstest.MapFile{}})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code == http.StatusOK {
		t.Fatalf("index.html 缺失时不应返回 200，实际: %d", w.Code)
	}
}
