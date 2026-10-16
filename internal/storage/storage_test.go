package storage

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// 1x1 透明 PNG，便于 DetectContentType 识别为 image/png
var tinyPNG = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D,
	0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, 0x89, 0x00, 0x00, 0x00,
	0x0A, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00, 0x00, 0x00, 0x00, 0x49,
	0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
}

func TestSaveAndOpen_Local(t *testing.T) {
	dir := t.TempDir()
	SetLocalDir(dir)

	url, err := SaveBytes(tinyPNG, "receipt.png", 42)
	if err != nil {
		t.Fatalf("SaveBytes 失败: %v", err)
	}
	if url == "" || len(url) < len(publicPrefix) {
		t.Fatalf("返回路径异常: %q", url)
	}
	key := url[len(publicPrefix):]
	if key == "" {
		t.Fatalf("key 为空")
	}

	// 校验文件确实落盘
	onDisk := filepath.Join(dir, key)
	if _, err := os.Stat(onDisk); err != nil {
		t.Fatalf("文件未落盘: %v", err)
	}

	// 读取回来比对内容
	rc, ct, err := Open(key)
	if err != nil {
		t.Fatalf("Open 失败: %v", err)
	}
	defer rc.Close()
	got, _ := io.ReadAll(rc)
	if !bytes.Equal(got, tinyPNG) {
		t.Fatalf("内容不一致: len(got)=%d len(want)=%d", len(got), len(tinyPNG))
	}
	if ct != "image/png" {
		t.Fatalf("Content-Type 错误: %q", ct)
	}

	// 路径穿越防护
	if _, _, err := Open("../etc/passwd"); err == nil {
		t.Fatalf("应当拒绝路径穿越")
	}
}

func TestCheckAllowed_RejectsExe(t *testing.T) {
	dir := t.TempDir()
	SetLocalDir(dir)
	if _, err := SaveBytes([]byte("MZ not an image"), "evil.exe", 1); err == nil {
		t.Fatalf("应当拒绝非图片文件")
	}
}

func TestCleanupOrphans_GraceAndReference(t *testing.T) {
	dir := t.TempDir()
	SetLocalDir(dir)

	refURL, err := SaveBytes(tinyPNG, "referenced.png", 1)
	if err != nil {
		t.Fatalf("SaveBytes 失败: %v", err)
	}
	orphanURL, err := SaveBytes(tinyPNG, "orphan.png", 1)
	if err != nil {
		t.Fatalf("SaveBytes 失败: %v", err)
	}
	refKey := refURL[len(publicPrefix):]
	orphanKey := orphanURL[len(publicPrefix):]

	// 1) 宽限期内：刚上传的文件即使未引用也不删除
	deleted, err := CleanupOrphans(map[string]bool{refKey: true}, time.Hour)
	if err != nil {
		t.Fatalf("CleanupOrphans 失败: %v", err)
	}
	if deleted != 0 {
		t.Fatalf("宽限期内不应删除任何文件，实际删除 %d", deleted)
	}

	// 2) 宽限期 0：未引用的孤儿被删除，被引用的保留
	deleted, err = CleanupOrphans(map[string]bool{refKey: true}, 0)
	if err != nil {
		t.Fatalf("CleanupOrphans 失败: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("期望删除 1 个孤儿，实际 %d", deleted)
	}
	if _, err := os.Stat(filepath.Join(dir, orphanKey)); !os.IsNotExist(err) {
		t.Fatalf("孤儿文件未被删除: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, refKey)); err != nil {
		t.Fatalf("被引用文件被误删: %v", err)
	}
}
