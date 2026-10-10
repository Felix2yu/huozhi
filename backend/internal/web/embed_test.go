package web

import (
	"io/fs"
	"testing"
)

// TestFSReadsPlaceholder 保证 embed 子树可被正常读取。
// .gitkeep 常驻仓库，无论是否构建过前端都必须存在——它是「未构建前端也能编译」的支点。
func TestFSReadsPlaceholder(t *testing.T) {
	if _, err := fs.ReadFile(FS(), ".gitkeep"); err != nil {
		t.Fatalf("FS() 应能读到 dist/.gitkeep: %v", err)
	}
}

// TestHasBuildMatchesIndexHTML 校验「是否内嵌产物」的判定与 index.html 的实际可读性一致。
func TestHasBuildMatchesIndexHTML(t *testing.T) {
	_, err := fs.ReadFile(FS(), "index.html")
	if got, want := HasBuild(), err == nil; got != want {
		t.Fatalf("HasBuild()=%v，而 index.html 可读性=%v，两者应一致", got, want)
	}
}
