package handlers

import (
	"context"
	"huozhi/internal/config"
	"huozhi/internal/database"
	"huozhi/internal/models"
	"huozhi/internal/storage"
	"path/filepath"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAutoBackupDue(t *testing.T) {
	parse := func(value string) time.Time {
		t.Helper()
		if value == "" {
			return time.Time{}
		}
		v, err := time.Parse(time.RFC3339, value)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	for _, tt := range []struct {
		name      string
		frequency string
		zone      string
		clock     string
		now       string
		last      string
		disabled  bool
		want      bool
		wantError bool
	}{
		{name: "到时前不执行", frequency: "daily", zone: "Asia/Shanghai", clock: "03:00", now: "2026-09-17T18:59:59Z"},
		{name: "按用户时区到点执行", frequency: "daily", zone: "Asia/Shanghai", clock: "03:00", now: "2026-09-17T19:00:00Z", want: true},
		{name: "错过分钟仍补跑", frequency: "daily", zone: "Asia/Shanghai", clock: "03:00", now: "2026-09-17T20:05:00Z", want: true},
		{name: "同周期成功后不重复", frequency: "daily", zone: "Asia/Shanghai", clock: "03:00", now: "2026-09-17T20:05:00Z", last: "2026-09-17T19:00:00Z"},
		{name: "上一天成功不阻止本次", frequency: "daily", zone: "Asia/Shanghai", clock: "03:00", now: "2026-09-17T20:05:00Z", last: "2026-09-16T20:05:00Z", want: true},
		{name: "禁用不执行", frequency: "daily", zone: "Asia/Shanghai", clock: "03:00", now: "2026-09-17T20:05:00Z", disabled: true},
		{name: "空时区使用上海", frequency: "daily", clock: "03:00", now: "2026-09-17T19:00:00Z", want: true},
		{name: "空频率兼容每日", zone: "UTC", clock: "03:00", now: "2026-09-17T03:00:00Z", want: true},
		{name: "用户当地周日", frequency: "weekly", zone: "Asia/Shanghai", clock: "03:00", now: "2026-09-19T19:00:00Z", want: true},
		{name: "周日到时前", frequency: "weekly", zone: "Asia/Shanghai", clock: "03:00", now: "2026-09-19T18:59:59Z"},
		{name: "周备份周一补跑", frequency: "weekly", zone: "UTC", clock: "03:00", now: "2026-09-21T01:00:00Z", want: true},
		{name: "周备份成功不重复", frequency: "weekly", zone: "UTC", clock: "03:00", now: "2026-09-21T04:00:00Z", last: "2026-09-20T03:01:00Z"},
		{name: "跨年周备份", frequency: "weekly", zone: "UTC", clock: "03:00", now: "2027-01-01T04:00:00Z", last: "2026-12-27T03:01:00Z"},
		{name: "用户当地月初", frequency: "monthly", zone: "Asia/Shanghai", clock: "03:00", now: "2026-09-30T19:00:00Z", want: true},
		{name: "月初到时前", frequency: "monthly", zone: "UTC", clock: "03:00", now: "2026-10-01T02:59:59Z"},
		{name: "月备份错过月初补跑", frequency: "monthly", zone: "UTC", clock: "03:00", now: "2026-10-17T04:00:00Z", last: "2026-09-01T03:00:00Z", want: true},
		{name: "月备份成功不重复", frequency: "monthly", zone: "UTC", clock: "03:00", now: "2026-10-17T04:00:00Z", last: "2026-10-01T03:00:00Z"},
		{name: "负时区仍为前一天", frequency: "daily", zone: "America/New_York", clock: "23:00", now: "2026-09-18T03:05:00Z", want: true},
		{name: "未来执行记录不重复", frequency: "daily", zone: "UTC", clock: "03:00", now: "2026-09-17T04:00:00Z", last: "2026-09-18T04:00:00Z"},
		{name: "非法时间", frequency: "daily", zone: "UTC", clock: "25:00", now: "2026-09-17T04:00:00Z", wantError: true},
		{name: "非标准时间", frequency: "daily", zone: "UTC", clock: "3:00", now: "2026-09-17T04:00:00Z", wantError: true},
		{name: "非法时区", frequency: "daily", zone: "Invalid/Zone", clock: "03:00", now: "2026-09-17T04:00:00Z", wantError: true},
		{name: "非法频率", frequency: "yearly", zone: "UTC", clock: "03:00", now: "2026-09-17T04:00:00Z", wantError: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			u := models.User{
				AutoBackupEnabled:   !tt.disabled,
				AutoBackupFrequency: tt.frequency,
				AutoBackupTime:      tt.clock,
				AutoBackupLastRun:   parse(tt.last),
				Timezone:            tt.zone,
			}
			got, err := autoBackupDue(u, parse(tt.now))
			if got != tt.want || (err != nil) != tt.wantError {
				t.Fatalf("是否到期=%v，错误=%v；期望=%v，期望错误=%v", got, err, tt.want, tt.wantError)
			}
		})
	}
}

func TestAutoBackupSchedulerRetryAndDeduplication(t *testing.T) {
	originalDB, originalConfig := database.DB, config.AppConfig
	dir := t.TempDir()
	db, err := gorm.Open(sqlite.Open(filepath.Join(dir, "schedule.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	database.DB = db
	cfg := *originalConfig
	cfg.Upload.Path = filepath.Join(dir, "uploads")
	cfg.Backup.Path = filepath.Join(dir, "backups")
	cfg.S3 = config.S3Config{}
	config.AppConfig = &cfg
	t.Cleanup(func() {
		database.DB, config.AppConfig = originalDB, originalConfig
		sqlDB.Close()
	})
	if err := storage.Init(); err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.User{}); err != nil {
		t.Fatal(err)
	}
	u := models.User{Username: "备份调度测试", PasswordHash: "x", Timezone: "Asia/Shanghai", AutoBackupEnabled: true, AutoBackupTime: "03:00", AutoBackupFrequency: "daily"}
	if err := db.Create(&u).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 17, 19, 5, 0, 0, time.UTC)
	runAutoBackupsAt(now)
	if err := db.First(&u, u.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !u.AutoBackupLastRun.IsZero() {
		t.Fatal("备份失败不应更新最后执行时间")
	}
	if err := db.AutoMigrate(&models.Book{}, &models.Account{}, &models.Category{}, &models.Tag{}, &models.Budget{}, &models.SavingPlan{}, &models.SavingRecord{}, &models.Recurring{}, &models.Installment{}, &models.Reimbursement{}, &models.Loan{}, &models.LoanRepayment{}, &models.Transaction{}, &models.TransactionTag{}); err != nil {
		t.Fatal(err)
	}
	for _, at := range []time.Time{now, now.Add(time.Minute)} {
		runAutoBackupsAt(at)
	}
	if err := db.First(&u, u.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !u.AutoBackupLastRun.Equal(now) {
		t.Fatalf("最后执行时间=%v，期望=%v", u.AutoBackupLastRun, now)
	}
	backups, err := storage.ListBackups(context.Background(), u.ID)
	if err != nil || len(backups) != 1 {
		t.Fatalf("备份数量=%d，错误=%v，期望只生成一份", len(backups), err)
	}
	runAutoBackupsAt(now.AddDate(0, 0, 1))
	backups, err = storage.ListBackups(context.Background(), u.ID)
	if err != nil || len(backups) != 2 {
		t.Fatalf("备份数量=%d，错误=%v，期望次日生成第二份", len(backups), err)
	}
}
