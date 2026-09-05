package router

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// mountFrontend 将前端构建产物目录（Vite dist）挂到 gin 上，由后端直接托管 SPA，
// 免去容器内再架一层 nginx：
//   - /assets/* 为带内容哈希的产物，immutable 长缓存；
//   - sw.js / manifest 等 PWA 文件必须 no-cache（registerType: autoUpdate 需每次拉新）；
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
			case strings.HasPrefix(clean, "/assets/"):
				c.Header("Cache-Control", "public, max-age=31536000, immutable")
			case clean == "/sw.js" || strings.HasPrefix(clean, "/manifest") || strings.HasPrefix(clean, "/registerSW") || strings.HasPrefix(clean, "/workbox"):
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
