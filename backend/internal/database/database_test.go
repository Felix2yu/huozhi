package database

import (
	"os"
	"path/filepath"
	"testing"

	"huozhi/internal/config"
	"huozhi/internal/models"
)

func TestInitSQLite(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.db")
	db, err := Init(&config.DatabaseConfig{Driver: "sqlite", File: file})
	if err != nil {
		t.Fatalf("Init sqlite failed: %v", err)
	}
	if db == nil {
		t.Fatal("expected non-nil db")
	}
	if DB != db {
		t.Fatal("package DB var was not set")
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("sqlite file not created: %v", err)
	}
}

func TestInitDefaultFallsBackToSQLite(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "default.db")
	// empty driver should fall through to sqlite
	db, err := Init(&config.DatabaseConfig{Driver: "", File: file})
	if err != nil {
		t.Fatalf("Init default driver failed: %v", err)
	}
	if db == nil {
		t.Fatal("expected non-nil db")
	}
}

func TestInitPostgresInvalidDSNReturnsError(t *testing.T) {
	// an unreachable host must surface an error from gorm.Open
	_, err := Init(&config.DatabaseConfig{
		Driver:   "postgres",
		Host:     "127.0.0.1",
		Port:     "1",
		User:     "nouser",
		Password: "nopass",
		Name:     "nodb",
		SSLMode:  "disable",
	})
	if err == nil {
		t.Fatal("expected error for invalid postgres DSN, got nil")
	}
}

func TestAutoMigrate(t *testing.T) {
	dir := t.TempDir()
	db, err := Init(&config.DatabaseConfig{Driver: "sqlite", File: filepath.Join(dir, "mig.db")})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if err := AutoMigrate(db, &models.User{}, &models.Recurring{}); err != nil {
		t.Fatalf("AutoMigrate failed: %v", err)
	}
}
