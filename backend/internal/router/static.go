package router

import (
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

// mountFrontend 将前端构建产物挂到 gin 的 NoRoute 上，由后端直接托管 SPA，
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
//
// fsys 同时覆盖两种产物来源，共用同一套缓存策略：
//   - os.DirFS(static_dir)：磁盘目录，可不重新编译就替换前端；
//   - web.FS()：go:embed 内嵌产物，用于单二进制分发。
func mountFrontend(r *gin.Engine, fsys fs.FS) {
	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		if strings.HasPrefix(p, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "接口不存在"})
			return
		}
		// 与 http.ServeFile 的行为保持一致：带 ".." 的请求直接 400，
		// 而不是 Clean 掉之后当成普通路径放行——后者会把穿越尝试
		// 变成一次 SPA 回退（200），掩盖真正的攻击信号。
		if strings.Contains(p, "..") {
			c.Status(http.StatusBadRequest)
			return
		}
		clean := path.Clean("/" + p)
		name := strings.TrimPrefix(clean, "/")
		if info, err := fs.Stat(fsys, name); err == nil && !info.IsDir() {
			setCacheHeader(c, clean)
			serveFileFS(c, fsys, name, info)
			return
		}
		// SPA 回退
		c.Header("Cache-Control", "no-cache")
		serveFileFS(c, fsys, "index.html", nil)
	})
}

// setCacheHeader 按产物类型设置缓存头，规则见 mountFrontend 注释。
func setCacheHeader(c *gin.Context, clean string) {
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
}

// serveFileFS 从 fs.FS 中取出文件并写入响应。info 为 nil 时自行 Stat。
func serveFileFS(c *gin.Context, fsys fs.FS, name string, info fs.FileInfo) {
	f, err := fsys.Open(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "页面不存在"})
		return
	}
	defer f.Close()

	if info == nil {
		info, err = f.Stat()
		if err != nil || info.IsDir() {
			c.JSON(http.StatusNotFound, gin.H{"error": "页面不存在"})
			return
		}
	}
	rs, ok := f.(io.ReadSeeker)
	if !ok {
		// embed.FS 与 os.DirFS 打开的文件都实现 io.Seeker，此处仅为防御
		c.Status(http.StatusInternalServerError)
		return
	}
	// ServeContent 负责 Content-Type 推断与 Range 请求；内嵌文件的 ModTime 为零值，
	// 此时它不会输出 Last-Modified，也不做 304——对带哈希的 immutable 资源无影响。
	http.ServeContent(c.Writer, c.Request, info.Name(), info.ModTime(), rs)
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
