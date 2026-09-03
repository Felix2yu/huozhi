package main

import (
	"flag"
	"fmt"
	"huozhi/internal/config"
	"huozhi/internal/database"
	"huozhi/internal/models"
	"huozhi/internal/router"
	"huozhi/internal/ws"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func main() {
	var cfgPath string
	var showVersion bool
	flag.StringVar(&cfgPath, "c", "./config.yaml", "配置文件路径")
	flag.BoolVar(&showVersion, "v", false, "查看版本")
	flag.Parse()

	if showVersion {
		fmt.Println("Huozhi 记账系统 v0.1.0")
		return
	}

	// 加载配置
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Printf("加载配置文件失败 (%v), 使用默认配置", err)
		cfg = config.Default()
		_ = os.MkdirAll("./uploads", 0755)
	}
	if _, err := os.Stat(cfg.Upload.Path); os.IsNotExist(err) {
		os.MkdirAll(cfg.Upload.Path, 0755)
	}

	// 连接数据库
	db, err := database.Init(&cfg.Database)
	if err != nil {
		log.Fatalf("数据库连接失败: %v", err)
	}
	log.Printf("数据库连接成功: driver=%s", cfg.Database.Driver)

	// 自动迁移
	err = database.AutoMigrate(db,
		&models.User{},
		&models.Book{},
		&models.BookMember{},
		&models.AccountGroup{},
		&models.Account{},
		&models.Category{},
		&models.Tag{},
		&models.Transaction{},
		&models.TransactionTag{},
		&models.Budget{},
		&models.SavingPlan{},
		&models.SavingRecord{},
		&models.Recurring{},
		&models.Installment{},
		&models.Reimbursement{},
		&models.AssetSnapshot{},
		&models.SyncLog{},
	)
	if err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}
	log.Println("数据库迁移完成")

	// 启动HTTP服务
	r := router.New(cfg.Server.Mode)

	srvAddr := ":" + cfg.Server.Port
	log.Printf("Huozhi 服务启动于 http://0.0.0.0%s (%s mode)", srvAddr, cfg.Server.Mode)

	go func() {
		if err := r.Run(srvAddr); err != nil {
			log.Fatalf("HTTP 服务失败: %v", err)
		}
	}()

	// 周期记账调度器
	go recurringRunner()

	// 资产快照（每日凌晨）
	go dailySnapshot()

	// WebSocket Hub
	go ws.DefaultHub.Run()
	log.Println("[WS] WebSocket Hub 已启动，监听 /api/ws")

	// 优雅退出
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Huozhi 服务正在关闭...")
}

// recurringRunner 每分钟扫描需执行的周期记账任务
func recurringRunner() {
	log.Println("[Cron] 周期记账调度器已启动")
	tick := time.NewTicker(30 * time.Second)
	defer tick.Stop()
	for range tick.C {
		runDueRecurrings()
	}
}

func runDueRecurrings() {
	if database.DB == nil {
		return
	}
	now := time.Now()
	var list []models.Recurring
	// 1. 执行到期的 active 任务
	database.DB.Where("status = 'active' AND next_run_at <= ?", now).Find(&list)
	for _, r := range list {
		processRecurring(&r)
	}
	// 2. 自动暂停已结束 / 已达上限的任务
	// 注意：SQLite / MySQL 将零值 time.Time 存为 '0000-00-00 00:00:00'（而非 Go 的
	// '0001-01-01'），不能简单用 != '0001-01-01' 判断。用 end_date > '0001-02-01'
	// 排除所有零值表示，避免把"未设结束时间"的周期性任务误判为已到期而自动暂停。
	database.DB.Exec(`
		UPDATE recurrings SET status = 'paused', next_run_at = NULL
		WHERE status = 'active' AND (
			(end_date IS NOT NULL AND end_date > '0001-02-01' AND end_date <= ?)
			OR (max_times > 0 AND run_count >= max_times)
		)`, now)
}

func processRecurring(r *models.Recurring) {
	now := time.Now()
	// 事务 + 行锁，防并发重复执行
	db := database.DB.Begin()
	var lock models.Recurring
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&lock, r.ID).Error; err != nil {
		db.Rollback()
		return
	}
	// 二次检查（可能已被其他 worker 处理 / 暂停 / 达上限）
	if lock.Status != "active" {
		db.Rollback()
		return
	}
	if !lock.EndDate.IsZero() && lock.EndDate.Before(now) {
		db.Model(&lock).Update("status", "paused")
		db.Model(&lock).Update("next_run_at", nil)
		db.Commit()
		return
	}
	if lock.MaxTimes > 0 && lock.RunCount >= lock.MaxTimes {
		db.Model(&lock).Update("status", "paused")
		db.Model(&lock).Update("next_run_at", nil)
		db.Commit()
		return
	}
	if lock.NextRunAt.After(now) {
		db.Rollback()
		return
	}

	// 防止同一分钟内被执行多次（兜底）
	windowStart := now.Add(-2 * time.Minute)
	var recentTx models.Transaction
	if err := db.Where("recurring_id = ? AND tx_date >= ?", lock.ID, windowStart).
		First(&recentTx).Error; err == nil {
		// 已执行过，跳过
		db.Model(&lock).Update("last_run_at", now)
		db.Model(&lock).Update("next_run_at", lock.ComputeNextRun(now))
		db.Commit()
		return
	}

	// 创建交易
	tx := models.Transaction{
		UserID:        lock.UserID,
		BookID:        lock.BookID,
		Type:          lock.Type,
		Amount:        lock.Amount,
		Currency:      "CNY",
		CategoryID:    lock.CategoryID,
		AccountID:     lock.AccountID,
		ToAccountID:   lock.ToAccountID,
		TxDate:        now,
		Description:   "[周期]" + lock.Description,
		RecurringID:   lock.ID,
		IncludeInBalance: true,
		IncludeInBudget:  true,
	}
	if err := db.Create(&tx).Error; err != nil {
		log.Printf("[Cron] 周期记账创建交易失败 recurring_id=%d err=%v", lock.ID, err)
		db.Rollback()
		return
	}
	// 复制标签关联
	for _, tagID := range lock.TagIDs {
		db.Create(&models.TransactionTag{TransactionID: tx.ID, TagID: tagID})
	}
	// 更新余额
	updateRecurBalances(db, &tx)

	// 更新任务状态
	// 注意：gorm.Expr 与时间字段不能放在同一个 map 中 Update，否则时间字段会被丢弃，
	// 导致 next_run_at / last_run_at 不落库。且 sqlite 驱动下纯 map 的 time.Time 字段
	// 也存在偶发丢弃，统一改用单列 Update 以保证 time.Time 字段可靠落库。
	newRunCount := lock.RunCount + 1
	next := lock.ComputeNextRun(now)
	db.Model(&lock).Update("run_count", gorm.Expr("run_count + 1"))
	db.Model(&lock).Update("last_run_at", now)
	if lock.MaxTimes > 0 && newRunCount >= lock.MaxTimes {
		db.Model(&lock).Update("status", "paused")
		db.Model(&lock).Update("next_run_at", nil)
	} else if !lock.EndDate.IsZero() && !next.Before(lock.EndDate) {
		db.Model(&lock).Update("status", "paused")
		db.Model(&lock).Update("next_run_at", nil)
	} else {
		db.Model(&lock).Update("next_run_at", next)
	}
	db.Commit()
	log.Printf("[Cron] 周期记账执行 ok recurring_id=%d tx_id=%d amount=%.2f type=%s next=%v",
		lock.ID, tx.ID, lock.Amount, lock.Type, next)
}

func updateRecurBalances(db *gorm.DB, tx *models.Transaction) {
	if !tx.IncludeInBalance {
		return
	}
	switch tx.Type {
	case models.TxExpense, models.TxReimburse:
		db.Model(&models.Account{}).Where("id = ?", tx.AccountID).
			Update("balance", gorm.Expr("balance - ?", tx.Amount))
	case models.TxIncome:
		db.Model(&models.Account{}).Where("id = ?", tx.AccountID).
			Update("balance", gorm.Expr("balance + ?", tx.Amount))
	case models.TxTransfer:
		db.Model(&models.Account{}).Where("id = ?", tx.AccountID).
			Update("balance", gorm.Expr("balance - ?", tx.Amount))
		if tx.ToAccountID > 0 {
			db.Model(&models.Account{}).Where("id = ?", tx.ToAccountID).
				Update("balance", gorm.Expr("balance + ?", tx.Amount))
		}
	}
}

func dailySnapshot() {
	log.Println("[Cron] 每日资产快照调度器已启动")
	for {
		now := time.Now()
		// 下一个 00:05
		next := time.Date(now.Year(), now.Month(), now.Day(), 0, 5, 0, 0, now.Location())
		if next.Before(now) {
			next = next.AddDate(0, 0, 1)
		}
		time.Sleep(time.Until(next))
		saveDailyAssetSnapshot()
	}
}

func saveDailyAssetSnapshot() {
	// 每个用户一份快照
	if database.DB == nil {
		return
	}
	var users []models.User
	database.DB.Select("id").Find(&users)
	today := time.Now()
	for _, u := range users {
		var accounts []models.Account
		database.DB.Where("user_id = ? AND is_archived = ?", u.ID, false).Find(&accounts)
		var asset, debt float64
		for _, a := range accounts {
			if !a.IncludeInTotal {
				continue
			}
			switch a.Type {
			case models.AccLiability, models.AccCredit:
				debt += a.Balance
			default:
				asset += a.Balance
			}
		}
		snap := models.AssetSnapshot{
			UserID: u.ID, SnapDate: today,
			TotalAsset: asset, TotalDebt: debt, NetAsset: asset - debt,
		}
		database.DB.Where("user_id = ? AND DATE(snap_date) = DATE(?)", u.ID, today).Delete(&models.AssetSnapshot{})
		database.DB.Create(&snap)
	}
	log.Printf("[Cron] 资产快照生成完成 user_count=%d", len(users))
}
