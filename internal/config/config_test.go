package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	c := Default()
	if c.JWT.Secret != "huozhi-dev-secret-change-me" {
		t.Fatal("default jwt secret mismatch")
	}
	if c.Database.Driver != "sqlite" {
		t.Fatal("default driver mismatch")
	}
	if c.Upload.MaxSizeMB != 10 {
		t.Fatal("default upload size mismatch")
	}
	if c.Server.Port != "8080" {
		t.Fatal("default port mismatch")
	}
}

func writeCfg(t *testing.T) string {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	content := `
server:
  port: "9090"
  mode: debug
database:
  driver: sqlite
  file: "./test.db"
jwt:
  secret: mysecret
  expire_hours: 12
  issuer: testapp
upload:
  path: ./uploads
  max_size_mb: 5
  allowed: jpg,png
`
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoad(t *testing.T) {
	p := writeCfg(t)
	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Port != "9090" {
		t.Fatal("port not loaded")
	}
	if cfg.JWT.Secret != "mysecret" {
		t.Fatal("jwt secret not loaded")
	}
	if cfg.JWT.ExpireHours != 12 {
		t.Fatal("expire hours not loaded")
	}
	if cfg.Upload.MaxSizeMB != 5 {
		t.Fatal("upload size not loaded")
	}
	if AppConfig != cfg {
		t.Fatal("AppConfig global not set")
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load("/no/such/file.yaml"); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(p, []byte(":::not valid yaml :::\n\t-"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(p); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestLoadEnvOverrides(t *testing.T) {
	t.Setenv("HZ_DB_DRIVER", "postgres")
	t.Setenv("HZ_DB_DSN", "foo")
	t.Setenv("HZ_JWT_SECRET", "envsecret")
	t.Setenv("HZ_PORT", "7777")
	t.Setenv("HZ_UPLOAD_PATH", "/app/data/uploads")

	p := writeCfg(t)
	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Database.Driver != "postgres" {
		t.Fatal("driver override failed")
	}
	if cfg.Database.Name != "foo" {
		t.Fatal("dsn override failed")
	}
	if cfg.JWT.Secret != "envsecret" {
		t.Fatal("jwt override failed")
	}
	if cfg.Upload.Path != "/app/data/uploads" {
		t.Fatal("upload path override failed")
	}
	if cfg.Server.Port != "7777" {
		t.Fatal("port override failed")
	}
}
