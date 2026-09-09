package router

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// mountFrontend 将前端构建产物目录挂到 gin 上，由后端直接托管 SPA，
// 免去容器内再架一层 nginx。同时支持 Vite (React) 与 SvelteKit adapter-static：
//
// SvelteKit adapter-static 产物结构：
//
//	/_app/immutable/**  带内容哈希（JS/CSS/nodes/chunks），长缓存 + immutable
//	/_app/version.json   版本信息，短缓存
//	/_app/**             非 immutable 的运行时文件，不缓存
//
// Vite (React) 产物结构：
//
//	/assets/**           带内容哈希，长缓存 + immutable
//
// 通用规则：
//   - sw.js / manifest / workbox 等 PWA 文件必须 no-cache（autoUpdate 需每次拉新）；
//   - 其余未匹配路径回退 index.html（前端路由）；/api/* 不回退，保持 404 语义。
func mountFrontend(r *gin.Engine, dir string) {
	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		if strings.HasPrefix(p, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "接口不存在"})
			return
		}
		// Clean 掉 ".." 等路径穿越片段后再拼到静态目录下
		clean := path.Clean("/" + p)
		full := filepath.Join(dir, filepath.FromSlash(clean))
		if info, err := os.Stat(full); err == nil && !info.IsDir() {
			switch {
			// SvelteKit: /_app/immutable/** → 带内容哈希，永不改变
			case strings.HasPrefix(clean, "/_app/immutable/"):
				c.Header("Cache-Control", "public, max-age=31536000, immutable")
			// Vite (React): /assets/** → 带内容哈希
			case strings.HasPrefix(clean, "/assets/"):
				c.Header("Cache-Control", "public, max-age=31536000, immutable")
			// SvelteKit: /_app/version.json → 版本信息，适度缓存
			case clean == "/_app/version.json":
				c.Header("Cache-Control", "public, max-age=300")
			// SvelteKit: /_app/** (non-immutable) → 运行时文件，不缓存
			case strings.HasPrefix(clean, "/_app/"):
				c.Header("Cache-Control", "no-cache")
			// PWA 文件：必须每次拉新
			case isPWAFile(clean):
				c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
			default:
				c.Header("Cache-Control", "no-cache")
			}
			c.File(full)
			return
		}
		// SPA 回退
		c.Header("Cache-Control", "no-cache")
		c.File(filepath.Join(dir, "index.html"))
	})
}

// isPWAFile 判断是否为 PWA 相关文件（需要禁用缓存以确保 Service Worker 更新）。
func isPWAFile(p string) bool {
	switch {
	case p == "/sw.js",
		strings.HasPrefix(p, "/manifest"),
		strings.HasPrefix(p, "/registerSW"),
		strings.HasPrefix(p, "/workbox-"):
		return true
	}
	return false
}
