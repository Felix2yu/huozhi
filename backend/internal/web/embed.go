// Package web 承载前端构建产物的内嵌（go:embed）与访问入口。
//
// 产物来源：SvelteKit adapter-static 输出到 frontend/build，由构建脚本/CI 在
// 编译前复制到本包下的 dist：
//
//	mkdir -p backend/internal/web/dist && cp -R frontend/build/. backend/internal/web/dist/
//
// dist 里常驻一个 .gitkeep 占位文件：未构建前端时 go build / go test 仍能编译通过，
// 此时 HasBuild 返回 false，后端只提供 API（不挂前端路由）。
package web

import (
	"embed"
	"io/fs"
)

// distFS 为内嵌的前端构建产物。用 all: 前缀以包含 .nojekyll、点开头文件等。
//
//go:embed all:dist
var distFS embed.FS

// FS 返回以站点根为起点的产物文件系统（已剥掉 dist 前缀），
// 可直接喂给 os.DirFS 的同套消费逻辑。
func FS() fs.FS {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		// "dist" 是编译期固定且合法的路径，fs.Sub 不可能在此报错；
		// 兜底返回未剥前缀的 FS 也好过返回 nil 让调用方 panic。
		return distFS
	}
	return sub
}

// HasBuild 报告是否已内嵌可用的前端产物（存在 index.html）。
// 未构建前端时 dist 下只有 .gitkeep，此时不应托管前端，
// 交给 vite dev server（本地开发）或前置反代（生产）。
func HasBuild() bool {
	info, err := fs.Stat(distFS, "dist/index.html")
	return err == nil && !info.IsDir()
}
