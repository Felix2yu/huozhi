package main

import (
	"os"
	"testing"
	"time"

	"huozhi/internal/database"
	"huozhi/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupCmdDB(t *testing.T) uint {
	t.Helper()
	f, err := os.CreateTemp("", "huozhi-cmd-*.db")
	if err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(sqlite.Open(f.Name()), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	database.DB = db
	if err := db.AutoMigrate(
		&models.User{}, &models.Book{}, &models.BookMember{}, &models.AccountGroup{},
		&models.Account{}, &models.Category{}, &models.Tag{}, &models.Transaction{},
		&models.TransactionTag{}, &models.Budget{}, &models.SavingPlan{}, &models.SavingRecord{},
		&models.Recurring{}, &models.Installment{}, &models.Reimbursement{},
		&models.AssetSnapshot{}, &models.SyncLog{},
	); err != nil {
		t.Fatal(err)
	}
	user := models.User{Username: "cmduser", PasswordHash: "x", Status: 1, Currency: "CNY"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	book := models.Book{UserID: user.ID, Name: "B", Currency: "CNY", IsDefault: true}
	db.Create(&book)
	acc := models.Account{UserID: user.ID, BookID: book.ID, Name: "现金", Type: models.AccCash, Currency: "CNY", Balance: 0}
	db.Create(&acc)
	cat := models.Category{UserID: user.ID, BookID: book.ID, Name: "餐饮", Kind: models.KindExpense}
	db.Create(&cat)
	t.Cleanup(func() { os.Remove(f.Name()) })
	return user.ID
}

func seedRecurring(t *testing.T, uid, bookID, accID, catID uint, mut func(*models.Recurring)) uint {
	t.Helper()
	r := models.Recurring{
		UserID: uid, BookID: bookID, Name: "房租", Type: models.TxExpense,
		Amount: 3000, CategoryID: catID, AccountID: accID, RecurringType: models.RecMonthly,
		NextRunAt: time.Now().Add(-time.Hour), Status: "active",
	}
	if mut != nil {
		mut(&r)
	}
	if err := database.DB.Create(&r).Error; err != nil {
		t.Fatal(err)
	}
	return r.ID
}

func TestRunDueRecurrings(t *testing.T) {
	uid := setupCmdDB(t)
	// fetch ids
	var b models.Book
	database.DB.Where("user_id = ?", uid).First(&b)
	var a models.Account
	database.DB.Where("user_id = ?", uid).First(&a)
	var c models.Category
	database.DB.Where("user_id = ?", uid).First(&c)

	recID := seedRecurring(t, uid, b.ID, a.ID, c.ID, nil)

	runDueRecurrings()

	var txs []models.Transaction
	database.DB.Where("recurring_id = ?", recID).Find(&txs)
	if len(txs) != 1 {
		t.Fatalf("expected 1 tx, got %d", len(txs))
	}
	var r models.Recurring
	database.DB.First(&r, recID)
	if r.RunCount != 1 {
		t.Fatalf("expected run_count 1, got %d", r.RunCount)
	}
	if !r.NextRunAt.After(time.Now()) {
		t.Fatalf("expected next_run_at advanced to future, got %v", r.NextRunAt)
	}
}

func TestProcessRecurringSkipsWhenFuture(t *testing.T) {
	uid := setupCmdDB(t)
	var b models.Book
	database.DB.Where("user_id = ?", uid).First(&b)
	var a models.Account
	database.DB.Where("user_id = ?", uid).First(&a)
	var c models.Category
	database.DB.Where("user_id = ?", uid).First(&c)

	recID := seedRecurring(t, uid, b.ID, a.ID, c.ID, func(r *models.Recurring) {
		r.NextRunAt = time.Now().Add(time.Hour) // future
	})
	processRecurring(&models.Recurring{ID: recID})
	var txs []models.Transaction
	database.DB.Where("recurring_id = ?", recID).Find(&txs)
	if len(txs) != 0 {
		t.Fatalf("future recurring should not create tx, got %d", len(txs))
	}
}

func TestProcessRecurringPausedByMaxTimes(t *testing.T) {
	uid := setupCmdDB(t)
	var b models.Book
	database.DB.Where("user_id = ?", uid).First(&b)
	var a models.Account
	database.DB.Where("user_id = ?", uid).First(&a)
	var c models.Category
	database.DB.Where("user_id = ?", uid).First(&c)

	recID := seedRecurring(t, uid, b.ID, a.ID, c.ID, func(r *models.Recurring) {
		r.MaxTimes = 1
		r.RunCount = 1
	})
	processRecurring(&models.Recurring{ID: recID})
	var r models.Recurring
	database.DB.First(&r, recID)
	if r.Status != "paused" {
		t.Fatalf("expected paused, got %s", r.Status)
	}
}

func TestProcessRecurringPausedByEndDate(t *testing.T) {
	uid := setupCmdDB(t)
	var b models.Book
	database.DB.Where("user_id = ?", uid).First(&b)
	var a models.Account
	database.DB.Where("user_id = ?", uid).First(&a)
	var c models.Category
	database.DB.Where("user_id = ?", uid).First(&c)

	recID := seedRecurring(t, uid, b.ID, a.ID, c.ID, func(r *models.Recurring) {
		r.EndDate = time.Now().Add(-time.Hour)
	})
	processRecurring(&models.Recurring{ID: recID})
	var r models.Recurring
	database.DB.First(&r, recID)
	if r.Status != "paused" {
		t.Fatalf("expected paused, got %s", r.Status)
	}
}

func TestSaveDailyAssetSnapshot(t *testing.T) {
	uid := setupCmdDB(t)
	saveDailyAssetSnapshot()
	var snaps []models.AssetSnapshot
	database.DB.Where("user_id = ?", uid).Find(&snaps)
	if len(snaps) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snaps))
	}
}

func TestProcessRecurringIncomeUpdatesBalance(t *testing.T) {
	uid := setupCmdDB(t)
	var b models.Book
	database.DB.Where("user_id = ?", uid).First(&b)
	var a models.Account
	database.DB.Where("user_id = ?", uid).First(&a)
	var c models.Category
	database.DB.Where("user_id = ?", uid).First(&c)

	recID := seedRecurring(t, uid, b.ID, a.ID, c.ID, func(r *models.Recurring) {
		r.Type = models.TxIncome
	})
	processRecurring(&models.Recurring{ID: recID})

	var acc models.Account
	database.DB.First(&acc, a.ID)
	if acc.Balance != 3000 {
		t.Fatalf("expected income to raise balance to 3000, got %v", acc.Balance)
	}
}

func TestProcessRecurringTransferUpdatesBalance(t *testing.T) {
	uid := setupCmdDB(t)
	var b models.Book
	database.DB.Where("user_id = ?", uid).First(&b)
	var a models.Account
	database.DB.Where("user_id = ?", uid).First(&a)
	var c models.Category
	database.DB.Where("user_id = ?", uid).First(&c)
	to := models.Account{UserID: uid, BookID: b.ID, Name: "储蓄卡", Type: models.AccBank, Currency: "CNY", Balance: 0}
	database.DB.Create(&to)

	recID := seedRecurring(t, uid, b.ID, a.ID, c.ID, func(r *models.Recurring) {
		r.Type = models.TxTransfer
		r.ToAccountID = to.ID
	})
	processRecurring(&models.Recurring{ID: recID})

	var from, dst models.Account
	database.DB.First(&from, a.ID)
	database.DB.First(&dst, to.ID)
	if from.Balance != -3000 {
		t.Fatalf("expected from-account balance -3000, got %v", from.Balance)
	}
	if dst.Balance != 3000 {
		t.Fatalf("expected to-account balance 3000, got %v", dst.Balance)
	}
}
