package storage

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"huozhi/internal/config"
)

func initBackupTest(t *testing.T, cfg *config.Config) {
	t.Helper()
	mu.Lock()
	oldConfig := config.AppConfig
	oldInited, oldS3, oldLocal, oldBackup := inited, useS3, localDir, backupDir
	oldClient, oldBucket, oldPrefix := s3Client, s3Bucket, s3Prefix
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		defer mu.Unlock()
		config.AppConfig = oldConfig
		inited, useS3, localDir, backupDir = oldInited, oldS3, oldLocal, oldBackup
		s3Client, s3Bucket, s3Prefix = oldClient, oldBucket, oldPrefix
	})
	config.AppConfig = cfg
	if err := Init(); err != nil {
		t.Fatal(err)
	}
}

func TestBackupLocalLifecycle(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		Upload: config.UploadConfig{Path: filepath.Join(dir, "uploads")},
		Backup: config.BackupConfig{Path: filepath.Join(dir, "uploads", "private")},
	}
	initBackupTest(t, cfg)
	ctx := context.Background()
	name := "auto-7-20260917-120000.zip"
	data := []byte("private backup")
	path, err := SaveBackup(ctx, 7, name, data)
	if err != nil || path != filepath.Join(cfg.Backup.Path, name) {
		t.Fatalf("保存失败: %q %v", path, err)
	}
	for path, mode := range map[string]os.FileMode{cfg.Backup.Path: 0o700, path: 0o600} {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != mode {
			t.Fatalf("权限错误: %s %v", path, err)
		}
	}
	objects, err := ListBackups(ctx, 7)
	if err != nil || len(objects) != 1 || objects[0].Name != name || objects[0].Size != int64(len(data)) || objects[0].Time.IsZero() {
		t.Fatalf("列举失败: %+v %v", objects, err)
	}
	rc, err := OpenBackup(ctx, 7, name)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(rc)
	rc.Close()
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("读取失败: %q %v", got, err)
	}
	other, err := ListBackups(ctx, 70)
	if err != nil || other == nil || len(other) != 0 {
		t.Fatalf("用户隔离失败: %+v %v", other, err)
	}
	for _, key := range []string{"backups/7/" + name, "private/" + name, "/backups/7/" + name, "./backups/7/" + name, "private//" + name, "private/../private/" + name} {
		if rc, _, err := Open(key); err == nil {
			rc.Close()
			t.Fatalf("公共接口泄露备份: %s", key)
		}
		if err := Delete(key); err == nil {
			t.Fatalf("公共接口可删除备份: %s", key)
		}
	}
	if n, err := CleanupOrphans(nil, 0); err != nil || n != 0 {
		t.Fatalf("清理误删备份: %d %v", n, err)
	}
	if err := DeleteBackup(ctx, 7, name); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenBackup(ctx, 7, name); !os.IsNotExist(err) {
		t.Fatalf("删除未生效: %v", err)
	}
	if err := DeleteBackup(ctx, 7, name); err != nil {
		t.Fatal(err)
	}
}

func TestBackupNamesAndCancellation(t *testing.T) {
	dir := t.TempDir()
	initBackupTest(t, &config.Config{Upload: config.UploadConfig{Path: filepath.Join(dir, "uploads")}, Backup: config.BackupConfig{Path: filepath.Join(dir, "backups")}})
	for _, name := range []string{"", "auto-70-a.zip", "auto-7-.zip", "../auto-7-a.zip", "auto-7-a/other.zip", `auto-7-a\other.zip`, "auto-7-%2e.zip", "auto-7-a.ZIP", "auto-7-a.zip\x00"} {
		if _, err := SaveBackup(context.Background(), 7, name, []byte("x")); err == nil {
			t.Errorf("保存未拒绝非法名称: %q", name)
		}
		if rc, err := OpenBackup(context.Background(), 7, name); err == nil {
			rc.Close()
			t.Errorf("读取未拒绝非法名称: %q", name)
		}
		if err := DeleteBackup(context.Background(), 7, name); err == nil {
			t.Errorf("删除未拒绝非法名称: %q", name)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	name := "auto-7-a.zip"
	if _, err := SaveBackup(ctx, 7, name, nil); err != context.Canceled {
		t.Errorf("保存未传播取消: %v", err)
	}
	if _, err := ListBackups(ctx, 7); err != context.Canceled {
		t.Errorf("列举未传播取消: %v", err)
	}
	if _, err := OpenBackup(ctx, 7, name); err != context.Canceled {
		t.Errorf("读取未传播取消: %v", err)
	}
	if err := DeleteBackup(ctx, 7, name); err != context.Canceled {
		t.Errorf("删除未传播取消: %v", err)
	}
}
