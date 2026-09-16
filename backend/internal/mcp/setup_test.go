package mcp

import (
	"huozhi/internal/config"
	"huozhi/internal/database"
	"huozhi/internal/models"
	"huozhi/internal/ws"
	"os"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestMain 起一个临时 SQLite 库，覆盖 database.DB 全局句柄。
// 与 internal/handlers 的测试同构，避免为 MCP 单独造一套数据访问方式。
func TestMain(m *testing.M) {
	config.AppConfig = &config.Config{
		JWT:    config.JWTConfig{Secret: "test-secret", ExpireHours: 168, Issuer: "huozhi"},
		Upload: config.UploadConfig{Path: os.TempDir(), MaxSizeMB: 10, Allowed: "jpg,png"},
	}
	f, err := os.CreateTemp("", "huozhi-mcp-test-*.db")
	if err != nil {
		panic(err)
	}
	dbPath := f.Name()
	f.Close()

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	database.DB = db
	_ = db.AutoMigrate(
		&models.User{}, &models.Book{}, &models.BookMember{}, &models.AccountGroup{},
		&models.Account{}, &models.Category{}, &models.Tag{}, &models.Transaction{},
		&models.TransactionTag{}, &models.Budget{}, &models.SavingPlan{}, &models.SavingRecord{},
		&models.Recurring{}, &models.Installment{}, &models.Reimbursement{},
		&models.AssetSnapshot{}, &models.SyncLog{},
	)
	go ws.DefaultHub.Run()

	code := m.Run()
	os.Remove(dbPath)
	os.Exit(code)
}
