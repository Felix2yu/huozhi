package config

import (
	"os"
	"path/filepath"
	"testing"
)

func loadStorageConfig(t *testing.T, content string) (*Config, error) {
	t.Helper()
	old := AppConfig
	t.Cleanup(func() { AppConfig = old })
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return Load(path)
}

func TestStorageConfigDefaults(t *testing.T) {
	for _, content := range []string{"{}", "s3: {}", "s3:\n  use_ssl: true", "s3:\n  use_ssl: false"} {
		t.Run(content, func(t *testing.T) {
			cfg, err := loadStorageConfig(t, content)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.S3.UseSSL != (content != "s3:\n  use_ssl: false") {
				t.Fatal("SSL 默认值或显式 false 未生效")
			}
			if cfg.Backup.Path != "./backups" || cfg.S3.Enabled || cfg.S3.ForcePathStyle != nil {
				t.Fatalf("存储默认值错误: %+v", cfg)
			}
			if cfg.Server.Port != "" || cfg.Upload.CleanupIntervalMinutes != 0 || cfg.Upload.MaxSizeMB != 0 || cfg.JWT.ExpireHours != 0 || cfg.S3.Region != "" || cfg.MCP.Disabled || cfg.Fx.Disabled {
				t.Fatal("改变了其他缺失配置的零值语义")
			}
		})
	}
	if cfg := Default(); !cfg.S3.UseSSL || cfg.Backup.Path != "./backups" || cfg.S3.ForcePathStyle != nil {
		t.Fatal("Default 存储配置错误")
	}
}

func TestStorageConfigEnvOverrides(t *testing.T) {
	for _, value := range []string{"false", "0", "true", "1"} {
		t.Run(value, func(t *testing.T) {
			for _, key := range []string{"HZ_S3_ENABLED", "HZ_S3_USE_SSL", "HZ_S3_FORCE_PATH_STYLE"} {
				t.Setenv(key, value)
			}
			t.Setenv("HZ_BACKUP_PATH", "./data/backups")
			t.Setenv("HZ_S3_ENDPOINT", "localhost:9000")
			t.Setenv("HZ_S3_REGION", "test-region")
			t.Setenv("HZ_S3_BUCKET", "test-bucket")
			t.Setenv("HZ_S3_PREFIX", "private")
			t.Setenv("HZ_S3_ACCESS_KEY", "test-key")
			t.Setenv("HZ_S3_SECRET_KEY", "test-secret")
			cfg, err := loadStorageConfig(t, "backup:\n  path: ./old\ns3:\n  enabled: true\n  use_ssl: true\n  force_path_style: true\n  bucket: old-bucket\n")
			if err != nil {
				t.Fatal(err)
			}
			want := value == "true" || value == "1"
			if cfg.S3.Enabled != want || cfg.S3.UseSSL != want || cfg.S3.ForcePathStyle == nil || *cfg.S3.ForcePathStyle != want {
				t.Fatal("布尔环境变量未覆盖")
			}
			if cfg.Backup.Path != "./data/backups" || cfg.S3.Endpoint != "localhost:9000" || cfg.S3.Region != "test-region" || cfg.S3.Bucket != "test-bucket" || cfg.S3.Prefix != "private" || cfg.S3.AccessKey != "test-key" || cfg.S3.SecretKey != "test-secret" {
				t.Fatal("字符串环境变量未覆盖")
			}
		})
	}
}

func TestStorageConfigEmptyEnvOverrides(t *testing.T) {
	for _, key := range []string{"HZ_S3_ENDPOINT", "HZ_S3_PREFIX", "HZ_S3_ACCESS_KEY", "HZ_S3_SECRET_KEY"} {
		t.Setenv(key, "")
	}
	cfg, err := loadStorageConfig(t, "s3:\n  endpoint: localhost:9000\n  prefix: old\n  access_key: old-key\n  secret_key: old-secret\n")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.S3.Endpoint != "" || cfg.S3.Prefix != "" || cfg.S3.AccessKey != "" || cfg.S3.SecretKey != "" {
		t.Fatal("不能通过环境变量恢复 AWS 默认端点或凭证链")
	}
}

func TestStorageConfigInvalidBooleans(t *testing.T) {
	for _, key := range []string{"HZ_S3_ENABLED", "HZ_S3_USE_SSL", "HZ_S3_FORCE_PATH_STYLE"} {
		t.Run(key, func(t *testing.T) {
			t.Setenv(key, "invalid")
			if _, err := loadStorageConfig(t, "{}"); err == nil {
				t.Fatal("未拒绝非法布尔环境变量")
			}
		})
	}
}

func TestStorageConfigValidation(t *testing.T) {
	for _, content := range []string{
		"s3:\n  enabled: true",
		"s3:\n  enabled: true\n  bucket: bad/bucket",
		"s3:\n  enabled: true\n  bucket: 127.0.0.1",
		"s3:\n  enabled: true\n  bucket: bad..bucket",
		"s3:\n  access_key: lone-key",
		"s3:\n  secret_key: lone-secret",
		"s3:\n  endpoint: ftp://localhost",
		"s3:\n  endpoint: https://",
		"s3:\n  endpoint: https://user:password@localhost",
		"s3:\n  endpoint: https://localhost?token=secret",
	} {
		t.Run(content, func(t *testing.T) {
			previous := AppConfig
			if _, err := loadStorageConfig(t, content); err == nil {
				t.Fatal("未拒绝非法 S3 配置")
			}
			if AppConfig != previous {
				t.Fatal("配置失败覆盖了已有配置")
			}
		})
	}
	for _, useSSL := range []bool{true, false} {
		s := S3Config{Enabled: true, Bucket: "test-bucket", Endpoint: "localhost:9000", UseSSL: useSSL}
		if err := s.Validate(); err != nil {
			t.Fatal(err)
		}
		got, err := s.EndpointURL()
		want := "http://localhost:9000"
		if useSSL {
			want = "https://localhost:9000"
		}
		if err != nil || got != want {
			t.Fatalf("端点协议错误: %s %v", got, err)
		}
	}
}
