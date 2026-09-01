package main

import (
	"flag"
	"fmt"
	"huozhi/internal/config"
	"huozhi/internal/database"
	"huozhi/internal/models"
	"huozhi/internal/router"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gorm.io/gorm"
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

	// 优雅退出
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Huozhi 服务正在关闭...")
}

// recurringRunner 每分钟扫描需执行的周期记账任务
func recurringRunner() {
	log.Println("[Cron] 周期记账调度器已启动")
	tick := time.NewTicker(60 * time.Second)
	defer tick.Stop()
	for range tick.C {
		runDueRecurrings()
	}
}

func runDueRecurrings() {
	if database.DB == nil {
		return
	}
	var list []models.Recurring
	database.DB.Where("status = 'active' AND next_run_at <= ?", time.Now()).Find(&list)
	for _, r := range list {
		processRecurring(&r)
	}
}

func processRecurring(r *models.Recurring) {
	db := database.DB.Begin()
	tx := models.Transaction{
		UserID:        r.UserID,
		BookID:        r.BookID,
		Type:          r.Type,
		Amount:        r.Amount,
		Currency:      "CNY",
		CategoryID:    r.CategoryID,
		AccountID:     r.AccountID,
		ToAccountID:   r.ToAccountID,
		TxDate:        time.Now(),
		Description:   "[周期]" + r.Description,
		RecurringID:   r.ID,
		IncludeInBalance: true,
		IncludeInBudget:  true,
	}
	if err := db.Create(&tx).Error; err == nil {
		// 更新余额
		var from, to models.Account
		db.First(&from, tx.AccountID)
		if tx.ToAccountID > 0 {
			db.First(&to, tx.ToAccountID)
		}
		// 使用闭包反射式调用：简化直接手动处理
		updateRecurBalances(db, &tx, &from, &to)
	}
	// 计算下一次
	next := computeNextRun(r)
	db.Model(r).Updates(map[string]interface{}{
		"last_run_at": time.Now(),
		"next_run_at": next,
	})
	db.Commit()
}

func updateRecurBalances(db *gorm.DB, tx *models.Transaction, from, to *models.Account) {
	if !tx.IncludeInBalance {
		return
	}
	switch tx.Type {
	case models.TxExpense, models.TxReimburse:
		db.Model(from).Update("balance", gorm.Expr("balance - ?", tx.Amount))
	case models.TxIncome:
		db.Model(from).Update("balance", gorm.Expr("balance + ?", tx.Amount))
	case models.TxTransfer:
		db.Model(from).Update("balance", gorm.Expr("balance - ?", tx.Amount))
		if to != nil && to.ID > 0 {
			db.Model(to).Update("balance", gorm.Expr("balance + ?", tx.Amount))
		}
	}
}

func computeNextRun(r *models.Recurring) time.Time {
	now := r.NextRunAt
	if now.IsZero() {
		now = time.Now()
	}
	switch r.RecurringType {
	case models.RecDaily:
		return now.AddDate(0, 0, r.Interval)
	case models.RecWeekly:
		return now.AddDate(0, 0, 7)
	case models.RecBiWeek:
		return now.AddDate(0, 0, 14)
	case models.RecMonthly:
		return now.AddDate(0, 1, 0)
	case models.RecYearly:
		return now.AddDate(1, 0, 0)
	case models.RecCustom:
		return now.AddDate(0, 0, r.Interval)
	}
	return now.AddDate(0, 0, 1)
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
