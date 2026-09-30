package database

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type pureGoGuardModel struct {
	ID   uint `gorm:"primarykey"`
	Name string
}

// TestPureGoSqliteDriver 守卫测试：确认 glebarez/sqlite（纯 Go，无 CGO）在
// CGO_ENABLED=0 下可正常注册、建表、读写。等价于 bili-history 的 driver_purego_test.go，
// 防止回归到需要 CGO 的 mattn 驱动。
func TestPureGoSqliteDriver(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open pure-go sqlite: %v", err)
	}
	if err := db.AutoMigrate(&pureGoGuardModel{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	if err := db.Create(&pureGoGuardModel{Name: "ping"}).Error; err != nil {
		t.Fatalf("create: %v", err)
	}
	var m pureGoGuardModel
	if err := db.First(&m, "name = ?", "ping").Error; err != nil {
		t.Fatalf("read: %v", err)
	}
	if m.Name != "ping" {
		t.Fatalf("unexpected value: %q", m.Name)
	}
}
