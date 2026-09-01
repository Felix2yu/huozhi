package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	JWT      JWTConfig      `yaml:"jwt"`
	Upload   UploadConfig   `yaml:"upload"`
}

type ServerConfig struct {
	Port         string `yaml:"port"`
	Mode         string `yaml:"mode"` // debug, release
	ReadTimeout  int    `yaml:"read_timeout"`
	WriteTimeout int    `yaml:"write_timeout"`
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
	Path      string `yaml:"path"`
	MaxSizeMB int    `yaml:"max_size_mb"`
	Allowed   string `yaml:"allowed"`
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
			Path:      "./uploads",
			MaxSizeMB: 10,
			Allowed:   "jpg,jpeg,png,gif,webp,pdf,csv,xlsx",
		},
	}
}
