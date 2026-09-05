package config

import (
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	JWT      JWTConfig      `yaml:"jwt"`
	Upload   UploadConfig   `yaml:"upload"`
	S3       S3Config       `yaml:"s3"`
}

// S3Config 对象存储配置（S3 兼容：AWS S3 / MinIO / 阿里云 OSS 等）。
// 当 Enabled=true 时，账单图片等附件上传至 S3；否则存储到本地 Upload.Path。
type S3Config struct {
	Enabled   bool   `yaml:"enabled"`
	Endpoint  string `yaml:"endpoint"`  // 自定义端点，如 http://localhost:9000 或 https://oss-cn-hangzhou.aliyuncs.com（AWS S3 留空）
	Region    string `yaml:"region"`    // 如 us-east-1 / cn-hangzhou（MinIO 可填 auto）
	Bucket    string `yaml:"bucket"`    // 存储桶名（需预先创建）
	Prefix    string `yaml:"prefix"`    // 对象键前缀，如 huozhi（可选）
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
	UseSSL    bool   `yaml:"use_ssl"`
}

type ServerConfig struct {
	Port         string `yaml:"port"`
	Mode         string `yaml:"mode"` // debug, release
	ReadTimeout  int    `yaml:"read_timeout"`
	WriteTimeout int    `yaml:"write_timeout"`
	// StaticDir 前端构建产物目录（SPA）。设置后由后端直接托管静态文件与路由回退，
	// 无需前置 nginx；留空则仅提供 API（本地开发走 vite dev server）。
	StaticDir string `yaml:"static_dir"`
}

type DatabaseConfig struct {
	Driver   string `yaml:"driver"`   // sqlite, postgres
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Name     string `yaml:"name"`
	File     string `yaml:"file"` // sqlite file path
	SSLMode  string `yaml:"ssl_mode"`
}

type JWTConfig struct {
	Secret     string `yaml:"secret"`
	ExpireHours int   `yaml:"expire_hours"`
	Issuer     string `yaml:"issuer"`
}

type UploadConfig struct {
	Path                   string `yaml:"path"`
	MaxSizeMB              int    `yaml:"max_size_mb"`
	Allowed                string `yaml:"allowed"`
	OrphanGraceMinutes     int    `yaml:"orphan_grace_minutes"`      // 孤儿文件宽限期（分钟），默认 60：上传后未关联交易的文件到期后自动清理
	CleanupIntervalMinutes int    `yaml:"cleanup_interval_minutes"`  // 孤儿自动清理周期（分钟），默认 360（6 小时），0=关闭自动清理
}

var AppConfig *Config

func Load(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// 环境变量覆盖
	if v := os.Getenv("HZ_DB_DRIVER"); v != "" {
		cfg.Database.Driver = v
	}
	if v := os.Getenv("HZ_DB_DSN"); v != "" {
		if cfg.Database.Driver == "sqlite" {
			cfg.Database.File = v
		} else {
			cfg.Database.Name = v
		}
	}
	if v := os.Getenv("HZ_JWT_SECRET"); v != "" {
		cfg.JWT.Secret = v
	}
	if v := os.Getenv("HZ_PORT"); v != "" {
		cfg.Server.Port = v
	}
	if v := os.Getenv("HZ_STATIC_DIR"); v != "" {
		cfg.Server.StaticDir = v
	}

	// S3 配置环境变量覆盖（便于容器/生产部署，无需修改配置文件）
	if v := os.Getenv("HZ_S3_ENABLED"); v == "true" || v == "1" {
		cfg.S3.Enabled = true
	}
	if v := os.Getenv("HZ_S3_ENDPOINT"); v != "" {
		cfg.S3.Endpoint = v
	}
	if v := os.Getenv("HZ_S3_REGION"); v != "" {
		cfg.S3.Region = v
	}
	if v := os.Getenv("HZ_S3_BUCKET"); v != "" {
		cfg.S3.Bucket = v
	}
	if v := os.Getenv("HZ_S3_PREFIX"); v != "" {
		cfg.S3.Prefix = v
	}
	if v := os.Getenv("HZ_S3_ACCESS_KEY"); v != "" {
		cfg.S3.AccessKey = v
	}
	if v := os.Getenv("HZ_S3_SECRET_KEY"); v != "" {
		cfg.S3.SecretKey = v
	}
	if v := os.Getenv("HZ_S3_USE_SSL"); v == "true" || v == "1" {
		cfg.S3.UseSSL = true
	}

	// 上传/孤儿清理配置环境变量覆盖
	if v := os.Getenv("HZ_UPLOAD_PATH"); v != "" {
		cfg.Upload.Path = v
	}
	if v := os.Getenv("HZ_UPLOAD_ORPHAN_GRACE_MINUTES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Upload.OrphanGraceMinutes = n
		}
	}
	if v := os.Getenv("HZ_UPLOAD_CLEANUP_INTERVAL_MINUTES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Upload.CleanupIntervalMinutes = n
		}
	}

	AppConfig = cfg
	return cfg, nil
}

func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Port:         "8080",
			Mode:         "debug",
			ReadTimeout:  30,
			WriteTimeout: 30,
		},
		Database: DatabaseConfig{
			Driver:  "sqlite",
			File:    "./huozhi.db",
			SSLMode: "disable",
		},
		JWT: JWTConfig{
			Secret:      "huozhi-dev-secret-change-me",
			ExpireHours: 24 * 7,
			Issuer:      "huozhi",
		},
		Upload: UploadConfig{
			Path:                   "./data/uploads",
			MaxSizeMB:              10,
			Allowed:                "jpg,jpeg,png,gif,webp,pdf,csv,xlsx",
			OrphanGraceMinutes:     60,
			CleanupIntervalMinutes: 360,
		},
		S3: S3Config{
			Enabled:   false,
			Endpoint:  "",
			Region:    "us-east-1",
			Bucket:    "",
			Prefix:    "huozhi",
			AccessKey: "",
			SecretKey: "",
			UseSSL:    true,
		},
	}
}
