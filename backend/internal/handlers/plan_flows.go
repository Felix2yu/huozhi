package handlers

import (
	"encoding/json"
	"fmt"
	"huozhi/internal/database"
	"huozhi/internal/dto"
	"huozhi/internal/middleware"
	"huozhi/internal/models"
	"huozhi/pkg/auth"
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ========== B4 / B5 / B6：让「分期 / 报销 / 存钱计划」真正产生资金流 ==========
//
// 这三个模块此前只写记录：不生成交易、不改账户余额、不推进状态，
// 与 README 承诺的「自动生成还款日历」「收款入账」「计划进度联动资产」不符。

// ---------- B4 分期：自动生成每期还款 ----------

// RunInstallmentRepayments 扫描到期的分期，生成还款交易并推进状态。
// 由 main.go 调度器每日调用，返回本轮生成的还款笔数。
func RunInstallmentRepayments(now time.Time) int {
	if database.DB == nil {
		return 0
	}
	// 只处理到「今天」为止的期次（含今天）
	today := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())
	var list []models.Installment
	database.DB.Where("status = ? AND next_repay_date <= ?", "active", today).Find(&list)

	generated := 0
	for _, ins := range list {
		// 幂等：该期次已有还款交易则跳过
		var exist int64
		database.DB.Model(&models.Transaction{}).
			Where("installment_id = ? AND installment_index = ?", ins.ID, ins.PaidMonths+1).
			Count(&exist)
		if exist > 0 {
			continue
		}
		db := database.DB.Begin()
		var acc models.Account
		if err := db.Where("id = ?", ins.AccountID).First(&acc).Error; err != nil {
			db.Rollback()
			continue
		}
		repayDate := ins.NextRepayDate
		if repayDate.IsZero() {
			repayDate = ins.FirstRepayDate
		}
		idx := ins.PaidMonths + 1
		tx := models.Transaction{
			UserID:           ins.UserID,
			BookID:           ins.BookID,
			Type:             models.TxExpense,
			Amount:           ins.MonthlyAmount,
			Currency:         acc.Currency,
			CategoryID:       ins.CategoryID,
			AccountID:        ins.AccountID,
			TxDate:           repayDate,
			Description:      "[分期] " + ins.Name,
			InstallmentID:    ins.ID,
			InstallmentIndex: idx,
			InstallmentTotal: ins.TotalMonths,
			RelatedType:      models.RelatedInstallmentRepay,
			IncludeInBalance: true,
			IncludeInBudget:  true,
		}
		if err := db.Create(&tx).Error; err != nil {
			db.Rollback()
			continue
		}
		updateAccountBalances(db, &tx, &acc, nil, true)
		applyBudgetUsed(db, ins.UserID, ins.BookID, ins.CategoryID, tx.TxDate, tx.AmountInBase(), tx.Type, true, 1)

		paidMonths := idx
		nextRepay := repayDate.AddDate(0, 1, 0)
		status := "active"
		if paidMonths >= ins.TotalMonths {
			status = "done"
			nextRepay = repayDate
		}
		db.Model(&models.Installment{}).Where("id = ?", ins.ID).Updates(map[string]interface{}{
			"paid_months":     paidMonths,
			"next_repay_date": nextRepay,
			"status":          status,
		})
		db.Commit()
		generated++
		log.Printf("[Cron] 分期还款已生成 installment_id=%d 第 %d/%d 期", ins.ID, idx, ins.TotalMonths)
	}
	return generated
}

// InstallmentRunner 分期还款调度器（每日一次）
func InstallmentRunner() {
	tick := time.NewTicker(6 * time.Hour)
	defer tick.Stop()
	for range tick.C {
		if n := RunInstallmentRepayments(time.Now()); n > 0 {
			log.Printf("[Cron] 分期还款生成 %d 笔", n)
		}
	}
}

// ---------- B5 报销：收款时生成入账交易 ----------

// payReimbursement 在报销状态变为 received/partial 时生成收款交易并增加账户余额。
// 此前只把关联交易标成 done，钱在资产里凭空消失，净资产失真。
func payReimbursement(db *gorm.DB, rm *models.Reimbursement, accountID uint, amount models.Money, when time.Time) (uint, bool) {
	if amount <= 0 || accountID == 0 {
		return 0, false
	}
	var acc models.Account
	if err := db.Where("id = ?", accountID).First(&acc).Error; err != nil {
		return 0, false
	}
	// 幂等：该报销单已生成过等额收款交易则跳过
	var exist int64
	db.Model(&models.Transaction{}).
		Where("related_type = ? AND book_id = ? AND account_id = ? AND amount = ?",
			models.RelatedReimburseReceived, rm.BookID, accountID, amount).Count(&exist)
	if exist > 0 {
		return 0, false
	}
	catID := ensureSystemCategory(db, rm.UserID, rm.BookID, "报销回款", models.KindIncome, "🧾")
	tx := models.Transaction{
		UserID:           rm.UserID,
		BookID:           rm.BookID,
		Type:             models.TxIncome,
		Amount:           amount,
		Currency:         acc.Currency,
		CategoryID:       catID,
		AccountID:        accountID,
		TxDate:           when,
		Description:      "[报销] " + rm.Name,
		RelatedType:      models.RelatedReimburseReceived,
		IncludeInBalance: true,
		IncludeInBudget:  false,
	}
	if err := db.Create(&tx).Error; err != nil {
		return 0, false
	}
	updateAccountBalances(db, &tx, &acc, nil, true)
	return tx.ID, true
}

// ---------- B6 存钱计划：存入时生成资金流 ----------

// depositSaving 存钱时生成一条「来源账户 → 计划账户」的转账交易（无计划账户时记支出）。
// 此前只累加 current_amount，不产生交易也不扣减账户，计划进度与真实资产脱节。
func depositSaving(db *gorm.DB, plan *models.SavingPlan, amount models.Money,
	fromAccountID uint, when time.Time) uint {
	if amount <= 0 {
		return 0
	}
	var from models.Account
	if fromAccountID > 0 {
		db.Where("id = ?", fromAccountID).First(&from)
	}
	if from.ID == 0 {
		// 未指定来源账户时取该用户第一个非负债账户
		db.Where("user_id = ? AND type NOT IN ?", plan.UserID,
			[]models.AccountType{models.AccCredit, models.AccLiability}).First(&from)
	}
	if from.ID == 0 {
		return 0
	}

	if plan.AccountID > 0 && plan.AccountID != from.ID {
		var to models.Account
		if err := db.Where("id = ?", plan.AccountID).First(&to).Error; err == nil {
			tx := models.Transaction{
				UserID: plan.UserID, BookID: plan.BookID, Type: models.TxTransfer,
				Amount: amount, Currency: from.Currency, AccountID: from.ID, ToAccountID: to.ID,
				CategoryID: ensureSystemCategory(db, plan.UserID, plan.BookID, "转账", models.KindSystem, "🔄"),
				TxDate: when, Description: "[存钱] " + plan.Name,
				RelatedType: models.RelatedSaving, IncludeInBalance: true, IncludeInBudget: false,
			}
			if err := db.Create(&tx).Error; err == nil {
				updateAccountBalances(db, &tx, &from, &to, true)
				return tx.ID
			}
		}
	}
	// 降级：无目标账户时记作支出，资金流出同样体现在余额上
	tx := models.Transaction{
		UserID: plan.UserID, BookID: plan.BookID, Type: models.TxExpense,
		Amount: amount, Currency: from.Currency, AccountID: from.ID,
		CategoryID: ensureSystemCategory(db, plan.UserID, plan.BookID, "存钱", models.KindExpense, "🏦"),
		TxDate: when, Description: "[存钱] " + plan.Name,
		RelatedType: models.RelatedSaving, IncludeInBalance: true, IncludeInBudget: false,
	}
	if err := db.Create(&tx).Error; err != nil {
		return 0
	}
	updateAccountBalances(db, &tx, &from, nil, true)
	return tx.ID
}

// ========== B7：账户余额对账与修复 ==========

// recomputeAccountBalance 按流水重算账户余额（不写库，仅计算）
func recomputeAccountBalance(db *gorm.DB, acc *models.Account) (models.Money, int) {
	var txs []models.Transaction
	db.Where("(account_id = ? OR to_account_id = ?) AND include_in_balance = ?",
		acc.ID, acc.ID, true).Order("tx_date ASC, id ASC").Find(&txs)

	sign := int64(1)
	if acc.Type == models.AccCredit || acc.Type == models.AccLiability {
		sign = -1
	}
	delta := int64(0)
	count := 0
	for _, t := range txs {
		if !t.IncludeInBalance {
			continue
		}
		count++
		// 折算到基准币种后再计入（原 B-02）：余额引擎亦按 AmountInBase() 增减，
		// 对账口径必须一致，否则每笔外币流水都会被误报为「账实不符」。
		amt := int64(t.AmountInBase())
		discount := int64(t.ToBaseMoney(t.TransferDiscount))
		switch models.TxBalanceDirection(t.Type) {
		case -1:
			if t.AccountID == acc.ID {
				delta -= amt * sign
			}
		case 1:
			if t.AccountID == acc.ID {
				delta += amt * sign
			}
		default:
			switch t.Type {
			case models.TxTransfer:
				if t.AccountID == acc.ID {
					delta += (-amt + discount) * sign
				}
				if t.ToAccountID == acc.ID {
					delta += amt * sign
				}
			case models.TxAdjust:
				// 新格式：金额恒正，方向由 Remark 标记；旧格式：amount 直接是差额（可负）
				if t.AccountID != acc.ID {
					continue
				}
				if t.Amount < 0 {
					delta += amt * sign
				} else if t.Remark == "调减" {
					delta -= amt * sign
				} else {
					delta += amt * sign
				}
			}
		}
	}
	return acc.InitialAmount + models.Money(delta), count
}

// AuditAccounts GET /accounts/audit —— 数据体检：账户当前余额 vs 流水重算余额
func AuditAccounts(c *gin.Context) {
	uid := middleware.GetUID(c)
	var accounts []models.Account
	database.DB.Where("user_id = ? AND is_archived = ?", uid, false).Find(&accounts)

	type row struct {
		AccountID   uint         `json:"account_id"`
		Name        string       `json:"name"`
		Type        string       `json:"type"`
		Balance     models.Money `json:"balance"`
		Computed    models.Money `json:"computed"`
		Diff        models.Money `json:"diff"`
		TxCount     int          `json:"tx_count"`
		NeedFix     bool         `json:"need_fix"`
	}
	rows := make([]row, 0, len(accounts))
	for i := range accounts {
		a := accounts[i]
		computed, n := recomputeAccountBalance(database.DB, &a)
		diff := computed - a.Balance
		rows = append(rows, row{
			AccountID: a.ID, Name: a.Name, Type: string(a.Type),
			Balance: a.Balance, Computed: computed, Diff: diff,
			TxCount: n, NeedFix: diff != 0,
		})
	}
	OK(c, gin.H{"accounts": rows, "checked": len(rows)})
}

// RecalcAccountBalance POST /accounts/:id/recalc —— 按流水重算并写回余额
func RecalcAccountBalance(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.IDRequest
	if err := c.ShouldBindUri(&req); err != nil {
		Bad(c, "参数错误")
		return
	}
	var acc models.Account
	if err := database.DB.Where("id = ? AND user_id = ?", req.ID, uid).First(&acc).Error; err != nil {
		NotFound(c, "账户不存在")
		return
	}
	computed, n := recomputeAccountBalance(database.DB, &acc)
	diff := computed - acc.Balance
	if diff == 0 {
		OK(c, gin.H{"account_id": acc.ID, "balance": acc.Balance, "changed": false, "tx_count": n})
		return
	}
	if err := database.DB.Model(&models.Account{}).Where("id = ?", acc.ID).
		Update("balance", computed).Error; err != nil {
		InternalErr(c, "修复失败: "+err.Error())
		return
	}
	OK(c, gin.H{"account_id": acc.ID, "old_balance": acc.Balance, "balance": computed,
		"diff": diff, "changed": true, "tx_count": n})
}

// ========== B8：全量备份 / 恢复 ==========

type backupSnapshot struct {
	Version      string                  `json:"version"`
	ExportedAt   time.Time               `json:"exported_at"`
	Books        []models.Book           `json:"books"`
	Accounts     []models.Account        `json:"accounts"`
	Categories   []models.Category       `json:"categories"`
	Tags         []models.Tag            `json:"tags"`
	Budgets      []models.Budget         `json:"budgets"`
	Transactions []models.Transaction    `json:"transactions"`
	SavingPlans  []models.SavingPlan     `json:"saving_plans"`
	SavingRecords []models.SavingRecord  `json:"saving_records"`
	Recurrings   []models.Recurring      `json:"recurrings"`
	Installments []models.Installment    `json:"installments"`
	Reimbursements []models.Reimbursement `json:"reimbursements"`
}

// ExportBackup GET /io/backup —— 导出当前用户全量数据（JSON 快照）
func ExportBackup(c *gin.Context) {
	uid := middleware.GetUID(c)
	snap := backupSnapshot{Version: "1.0", ExportedAt: time.Now()}
	database.DB.Where("user_id = ?", uid).Find(&snap.Books)
	database.DB.Where("user_id = ?", uid).Find(&snap.Accounts)
	database.DB.Where("user_id = ?", uid).Find(&snap.Categories)
	database.DB.Where("user_id = ?", uid).Find(&snap.Tags)
	database.DB.Where("user_id = ?", uid).Find(&snap.Budgets)
	database.DB.Preload("Tags").Where("user_id = ?", uid).Find(&snap.Transactions)
	database.DB.Where("user_id = ?", uid).Find(&snap.SavingPlans)
	database.DB.Where("user_id = ?", uid).Find(&snap.SavingRecords)
	database.DB.Where("user_id = ?", uid).Find(&snap.Recurrings)
	database.DB.Where("user_id = ?", uid).Find(&snap.Installments)
	database.DB.Where("user_id = ?", uid).Find(&snap.Reimbursements)

	c.Header("Content-Disposition", "attachment; filename=huozhi-backup-"+time.Now().Format("20060102")+".json")
	c.JSON(200, snap)
}

// ImportBackup POST /io/restore?mode=replace|merge —— 从全量快照恢复
// replace（默认）：先清空当前用户业务数据再导入；merge：按 ID 冲突则跳过。
func ImportBackup(c *gin.Context) {
	uid := middleware.GetUID(c)
	mode := c.DefaultQuery("mode", "replace")

	body, err := c.GetRawData()
	if err != nil || len(body) == 0 {
		Bad(c, "未读到备份内容")
		return
	}
	var snap backupSnapshot
	if err := json.Unmarshal(body, &snap); err != nil {
		Bad(c, "备份文件解析失败: "+err.Error())
		return
	}

	db := database.DB.Begin()
	if mode == "replace" {
		// 硬删当前用户业务数据（账号本身保留）
		purgeUserBusinessData(db, uid)
	}

	counts := map[string]int{}

	for i := range snap.Books {
		snap.Books[i].UserID = uid
		if err := db.Create(&snap.Books[i]).Error; err == nil {
			counts["books"]++
		}
	}
	for i := range snap.Accounts {
		snap.Accounts[i].UserID = uid
		if err := db.Create(&snap.Accounts[i]).Error; err == nil {
			counts["accounts"]++
		}
	}
	for i := range snap.Categories {
		snap.Categories[i].UserID = uid
		if err := db.Create(&snap.Categories[i]).Error; err == nil {
			counts["categories"]++
		}
	}
	for i := range snap.Tags {
		snap.Tags[i].UserID = uid
		if err := db.Create(&snap.Tags[i]).Error; err == nil {
			counts["tags"]++
		}
	}
	for i := range snap.Budgets {
		snap.Budgets[i].UserID = uid
		if err := db.Create(&snap.Budgets[i]).Error; err == nil {
			counts["budgets"]++
		}
	}
	for i := range snap.Transactions {
		t := snap.Transactions[i]
		t.UserID = uid
		if err := db.Create(&t).Error; err == nil {
			counts["transactions"]++
			for _, tg := range t.Tags {
				db.Create(&models.TransactionTag{TransactionID: t.ID, TagID: tg.ID})
			}
		}
	}
	for i := range snap.SavingPlans {
		snap.SavingPlans[i].UserID = uid
		if err := db.Create(&snap.SavingPlans[i]).Error; err == nil {
			counts["saving_plans"]++
		}
	}
	for i := range snap.SavingRecords {
		snap.SavingRecords[i].UserID = uid
		if err := db.Create(&snap.SavingRecords[i]).Error; err == nil {
			counts["saving_records"]++
		}
	}
	for i := range snap.Recurrings {
		snap.Recurrings[i].UserID = uid
		if err := db.Create(&snap.Recurrings[i]).Error; err == nil {
			counts["recurrings"]++
		}
	}
	for i := range snap.Installments {
		snap.Installments[i].UserID = uid
		if err := db.Create(&snap.Installments[i]).Error; err == nil {
			counts["installments"]++
		}
	}
	for i := range snap.Reimbursements {
		snap.Reimbursements[i].UserID = uid
		if err := db.Create(&snap.Reimbursements[i]).Error; err == nil {
			counts["reimbursements"]++
		}
	}

	if err := db.Commit().Error; err != nil {
		InternalErr(c, "恢复失败: "+err.Error())
		return
	}
	OK(c, gin.H{"mode": mode, "imported": counts})
}

// ---------- B9：清空数据 ----------

// purgeUserBusinessData 硬删该用户的全部业务数据（保留 User 本身与登录凭据）。
// 覆盖范围比 ImportBackup 的 replace 更完整，含账本成员、账户分组、
// 关联表、资产快照与同步日志，避免残留孤儿行。
func purgeUserBusinessData(db *gorm.DB, uid uint) {
	// 关联表先删（无 user_id，按父表归属）
	db.Unscoped().Exec(
		"DELETE FROM transaction_tags WHERE transaction_id IN (SELECT id FROM transactions WHERE user_id = ?)",
		uid)
	db.Unscoped().Where("user_id = ?", uid).Delete(&models.Transaction{})
	db.Unscoped().Where("user_id = ?", uid).Delete(&models.Budget{})
	db.Unscoped().Where("user_id = ?", uid).Delete(&models.SavingRecord{})
	db.Unscoped().Where("user_id = ?", uid).Delete(&models.SavingPlan{})
	db.Unscoped().Where("user_id = ?", uid).Delete(&models.Recurring{})
	db.Unscoped().Where("user_id = ?", uid).Delete(&models.Installment{})
	db.Unscoped().Where("user_id = ?", uid).Delete(&models.Reimbursement{})
	db.Unscoped().Where("user_id = ?", uid).Delete(&models.Tag{})
	db.Unscoped().Where("user_id = ?", uid).Delete(&models.Account{})
	db.Unscoped().Where("user_id = ?", uid).Delete(&models.AccountGroup{})
	db.Unscoped().Where("user_id = ?", uid).Delete(&models.AssetSnapshot{})
	db.Unscoped().Where("user_id = ?", uid).Delete(&models.SyncLog{})
	db.Unscoped().Where("user_id = ?", uid).Delete(&models.Category{})
	// 账本成员（按本人 id 与自有账本两条路径清）
	db.Unscoped().Where("user_id = ?", uid).Delete(&models.BookMember{})
	db.Unscoped().Exec(
		"DELETE FROM book_members WHERE book_id IN (SELECT id FROM books WHERE user_id = ?)", uid)
	db.Unscoped().Where("user_id = ?", uid).Delete(&models.Book{})
}

// ClearUserData POST /io/reset —— 清空当前用户的全部业务数据并重建默认账本与内置分类。
//
// 危险操作，三重保护：
//  1. 必须二次验证登录密码；
//  2. 必须显式传 confirm=CLEAR_ALL_DATA；
//  3. 服务端先落一份全量快照到日志目录，万一误操作仍可人工恢复。
func ClearUserData(c *gin.Context) {
	uid := middleware.GetUID(c)

	var req struct {
		Password string `json:"password" binding:"required"`
		Confirm  string `json:"confirm" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Bad(c, "请提供登录密码与确认字串")
		return
	}
	if req.Confirm != "CLEAR_ALL_DATA" {
		Bad(c, "确认字串不正确，未执行任何操作")
		return
	}

	var user models.User
	if err := database.DB.Where("id = ?", uid).First(&user).Error; err != nil {
		NotFound(c, "用户不存在")
		return
	}
	if !auth.CheckPassword(req.Password, user.PasswordHash) {
		Forbidden(c, "密码错误")
		return
	}

	// 落一份安全快照，便于人工回滚
	snap := backupSnapshot{Version: "1.0", ExportedAt: time.Now()}
	database.DB.Where("user_id = ?", uid).Find(&snap.Books)
	database.DB.Where("user_id = ?", uid).Find(&snap.Accounts)
	database.DB.Where("user_id = ?", uid).Find(&snap.Categories)
	database.DB.Where("user_id = ?", uid).Find(&snap.Tags)
	database.DB.Where("user_id = ?", uid).Find(&snap.Budgets)
	database.DB.Preload("Tags").Where("user_id = ?", uid).Find(&snap.Transactions)
	database.DB.Where("user_id = ?", uid).Find(&snap.SavingPlans)
	database.DB.Where("user_id = ?", uid).Find(&snap.SavingRecords)
	database.DB.Where("user_id = ?", uid).Find(&snap.Recurrings)
	database.DB.Where("user_id = ?", uid).Find(&snap.Installments)
	database.DB.Where("user_id = ?", uid).Find(&snap.Reimbursements)
	if buf, err := json.Marshal(snap); err == nil {
		_ = os.MkdirAll("backups", 0o755)
		name := fmt.Sprintf("backups/pre-clear-%d-%s.json", uid, time.Now().Format("20060102-150405"))
		if err := os.WriteFile(name, buf, 0o600); err == nil {
			log.Printf("[Audit] 清空数据前快照已保存: %s", name)
		}
	}

	db := database.DB.Begin()
	purgeUserBusinessData(db, uid)
	if err := db.Commit().Error; err != nil {
		InternalErr(c, "清空失败: "+err.Error())
		return
	}
	// 重建默认账本与内置分类，保证清空后应用仍可用
	initUserDefaults(&user)

	log.Printf("[Audit] 用户 %d 已清空全部业务数据", uid)
	Broadcast(c, "all", "reset", 0)
	OK(c, gin.H{"cleared": true})
}
