package handlers

import (
	"encoding/json"
	"huozhi/internal/database"
	"huozhi/internal/dto"
	"huozhi/internal/middleware"
	"huozhi/internal/models"
	"huozhi/internal/ws"
	"math"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ========== 交易 Transaction ==========

// CreateTransaction 创建交易（核心：同步更新账户余额）
func CreateTransaction(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.CreateTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Bad(c, "参数错误: "+err.Error())
		return
	}

	// 转账类型必须有目标账户
	if req.Type == "transfer" && req.ToAccountID == 0 {
		Bad(c, "转账需要指定目标账户")
		return
	}

	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 读取账户（加乐观锁，余额操作要谨慎）
	var fromAcc, toAcc models.Account
	if err := tx.Where("id = ? AND user_id = ?", req.AccountID, uid).First(&fromAcc).Error; err != nil {
		tx.Rollback()
		Bad(c, "来源账户不存在")
		return
	}
	if req.ToAccountID > 0 {
		if err := tx.Where("id = ? AND user_id = ?", req.ToAccountID, uid).First(&toAcc).Error; err != nil {
			tx.Rollback()
			Bad(c, "目标账户不存在")
			return
		}
	}

	// 构建交易
	newTx := models.Transaction{
		UserID:           uid,
		BookID:           req.BookID,
		Type:             models.TransactionType(req.Type),
		Amount:           req.Amount,
		Currency:         firstNotEmpty(req.Currency, "CNY"),
		ExchangeRate:     req.ExchangeRate,
		CategoryID:       req.CategoryID,
		AccountID:        req.AccountID,
		ToAccountID:      req.ToAccountID,
		TransferFee:      req.TransferFee,
		TransferDiscount: req.TransferDiscount,
		RefundOfID:       req.RefundOfID,
		TxDate:           req.TxDate.T(),
		Description:      req.Description,
		Images:           req.Images,
		Merchant:         req.Merchant,
		Location:         req.Location,
		IncludeInBalance: req.IncludeInBalance,
		IncludeInBudget:  req.IncludeInBudget,
		RecurringID:      req.RecurringID,
		InstallmentID:    req.InstallmentID,
		Remark:           req.Remark,
		ReimburseStatus:  req.ReimburseStatus,
	}
	if !newTx.IncludeInBalance {
		newTx.IncludeInBalance = true
	}

	if err := tx.Create(&newTx).Error; err != nil {
		tx.Rollback()
		InternalErr(c, "创建交易失败: "+err.Error())
		return
	}

	// 标签关联
	if len(req.TagIDs) > 0 {
		for _, tid := range req.TagIDs {
			tx.Create(&models.TransactionTag{TransactionID: newTx.ID, TagID: tid})
			tx.Model(&models.Tag{}).Where("id = ?", tid).UpdateColumn("count", gorm.Expr("count + 1"))
		}
		// 预加载标签
		var tags []models.Tag
		tx.Where("id IN ?", req.TagIDs).Find(&tags)
		tagPtrs := make([]*models.Tag, len(tags))
		for i := range tags {
			tagPtrs[i] = &tags[i]
		}
		newTx.Tags = tagPtrs
	}

	// 更新账户余额
	updateAccountBalances(tx, &newTx, &fromAcc, &toAcc, true)

	if req.BookID > 0 {
		applyBudgetUsed(tx, uid, req.BookID, newTx.CategoryID, newTx.TxDate, newTx.Amount, newTx.Type, newTx.IncludeInBudget, 1)
	}

	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		InternalErr(c, "提交失败")
		return
	}

	// 检查预算超限 → 推送 alert（仅支出且计入预算）
	if newTx.Type == models.TxExpense && newTx.IncludeInBudget && req.BookID > 0 {
		var overBudgets []models.Budget
		database.DB.Where(
			"user_id = ? AND book_id = ? AND start_date <= ? AND end_date >= ? AND used_amount > amount",
			uid, req.BookID, newTx.TxDate, newTx.TxDate,
		).Find(&overBudgets)
		if len(overBudgets) > 0 {
			data, _ := json.Marshal(map[string]interface{}{
				"count":     len(overBudgets),
				"tx_amount": newTx.Amount,
			})
			ws.DefaultHub.BroadcastWithData(uid, ws.Message{
				Type:   "alert",
				Table:  "budgets",
				Action: "over_budget",
				Data:   data,
			})
		}
	}

	// 重新加载完整信息
	database.DB.Preload("Tags").First(&newTx, newTx.ID)
	Broadcast(c, "transactions", "create", newTx.ID)
	Created(c, newTx)
}

// updateAccountBalances 更新相关账户余额
// isDebtAccount 信用卡 / 负债类账户：balance 正数表示「欠款」，与 statistics 资产概览口径一致。
// 这类账户的余额方向与普通资产账户相反，记账时需要对消增减方向。
func isDebtAccount(a *models.Account) bool {
	return a != nil && (a.Type == models.AccCredit || a.Type == models.AccLiability)
}

// debtSign 返回账户余额方向系数：普通资产账户为 +1（正数=我拥有的钱）；
// 信用卡/负债账户为 -1（正数=欠款，增减方向与资产相反）。
func debtSign(a *models.Account) float64 {
	if isDebtAccount(a) {
		return -1
	}
	return 1
}

func updateAccountBalances(db *gorm.DB, tx *models.Transaction, from, to *models.Account, isAdd bool) {
	factor := 1.0
	if !isAdd {
		factor = -1
	}
	if !tx.IncludeInBalance {
		return
	}
	// ds = 账户方向系数，债务类账户取反
	dsFrom := debtSign(from)
	dsTo := debtSign(to)
	switch tx.Type {
	case models.TxExpense, models.TxReimburse:
		// 支出：from账户余额减少（信用卡则欠款增加）
		db.Model(from).Update("balance", gorm.Expr("balance - ?", tx.Amount*factor*dsFrom))
	case models.TxIncome, models.TxRefund:
		// 收入/退款：from账户余额增加（信用卡则欠款减少）
		db.Model(from).Update("balance", gorm.Expr("balance + ?", tx.Amount*factor*dsFrom))
	case models.TxTransfer:
		// 转账：from 减少 amount、增加 discount（信用卡侧方向取反）；to 增加 amount
		db.Model(from).Update("balance", gorm.Expr("balance - ? + ?", tx.Amount*factor*dsFrom, tx.TransferDiscount*factor*dsFrom))
		if tx.TransferFee > 0 {
			db.Model(from).Update("balance", gorm.Expr("balance - ?", tx.TransferFee*factor*dsFrom))
		}
		if to != nil && to.ID > 0 {
			db.Model(to).Update("balance", gorm.Expr("balance + ?", tx.Amount*factor*dsTo))
		}
	case models.TxAdjust:
		// 调整：直接设置（由Adjust处理，这里不做）
	}
}

// applyBudgetUsed 应用预算 used_amount 变更（支持所有 period_type，通过 start/end_date 范围匹配）
// factor=+1 增加（创建/修改为新值），factor=-1 减少（删除/撤销旧值）
func applyBudgetUsed(db *gorm.DB, uid, bookID, catID uint, date time.Time, amount float64, txType models.TransactionType, includeInBudget bool, factor float64) {
	if txType != models.TxExpense || !includeInBudget || amount <= 0 {
		return
	}
	delta := amount * factor
	// 总预算（category_id=0）
	db.Model(&models.Budget{}).
		Where("user_id = ? AND book_id = ? AND category_id = 0 AND start_date <= ? AND end_date >= ?",
			uid, bookID, date, date).
		Update("used_amount", gorm.Expr("used_amount + ?", delta))
	// 分类预算
	if catID > 0 {
		db.Model(&models.Budget{}).
			Where("user_id = ? AND book_id = ? AND category_id = ? AND start_date <= ? AND end_date >= ?",
				uid, bookID, catID, date, date).
			Update("used_amount", gorm.Expr("used_amount + ?", delta))
	}
}

// GetTransaction 获取单条
func GetTransaction(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.IDRequest
	c.ShouldBindUri(&req)
	var tx models.Transaction
	if err := database.DB.Preload("Tags").
		Where("id = ? AND user_id = ?", req.ID, uid).First(&tx).Error; err != nil {
		NotFound(c, "交易不存在")
		return
	}
	OK(c, tx)
}

// ListTransactions 交易列表（分页+按日分组）
func ListTransactions(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.QueryTransactionRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		Bad(c, err.Error())
		return
	}
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 || req.PageSize > 200 {
		req.PageSize = 20
	}

	q := database.DB.Where("user_id = ?", uid)
	if req.BookID > 0 {
		q = q.Where("book_id = ?", req.BookID)
	}
	if req.Type != "" {
		q = q.Where("type = ?", req.Type)
	}
	if req.CategoryID > 0 {
		q = q.Where("category_id = ?", req.CategoryID)
	}
	if req.AccountID > 0 {
		q = q.Where("account_id = ? OR to_account_id = ?", req.AccountID, req.AccountID)
	}
	if !req.StartDate.IsZero() {
		q = q.Where("tx_date >= ?", req.StartDate)
	}
	if !req.EndDate.IsZero() {
		q = q.Where("tx_date <= ?", req.EndDate.AddDate(0, 0, 1))
	}
	if req.Keyword != "" {
		k := "%" + req.Keyword + "%"
		q = q.Where("description LIKE ? OR merchant LIKE ? OR remark LIKE ?", k, k, k)
	}
	if req.MinAmount > 0 {
		q = q.Where("amount >= ?", req.MinAmount)
	}
	if req.MaxAmount > 0 {
		q = q.Where("amount <= ?", req.MaxAmount)
	}
	if req.TagID > 0 {
		q = q.Joins("JOIN transaction_tags ON transactions.id = transaction_tags.transaction_id").
			Where("transaction_tags.tag_id = ?", req.TagID)
	}
	if req.ReimburseStatus != "" {
		q = q.Where("reimburse_status = ?", req.ReimburseStatus)
	}

	var total int64
	q.Model(&models.Transaction{}).Count(&total)

	var list []models.Transaction
	offset := (req.Page - 1) * req.PageSize
	q.Preload("Tags").Order("tx_date DESC, id DESC").
		Offset(offset).Limit(req.PageSize).Find(&list)

	// 按日分组
	dayMap := make(map[string][]models.Transaction)
	var dayOrder []string
	for _, t := range list {
		day := t.TxDate.Format("2006-01-02")
		if _, ok := dayMap[day]; !ok {
			dayOrder = append(dayOrder, day)
		}
		dayMap[day] = append(dayMap[day], t)
	}
	type dayGroup struct {
		Date        string               `json:"date"`
		DayIncome   float64              `json:"day_income"`
		DayExpense  float64              `json:"day_expense"`
		DayBalance  float64              `json:"day_balance"`
		Transactions []models.Transaction `json:"transactions"`
	}
	var grouped []dayGroup
	for _, d := range dayOrder {
		g := dayGroup{Date: d, Transactions: dayMap[d]}
		for _, t := range dayMap[d] {
			switch t.Type {
			case models.TxIncome, models.TxRefund:
				g.DayIncome += t.Amount
			case models.TxExpense:
				g.DayExpense += t.Amount
			}
		}
		g.DayBalance = g.DayIncome - g.DayExpense
		grouped = append(grouped, g)
	}

	// 汇总 — 必须用新 query，不能复用已执行过 Find 的 q
	var sumIn, sumOut float64
	sq := database.DB.Model(&models.Transaction{}).Where("user_id = ?", uid)
	if req.BookID > 0 {
		sq = sq.Where("book_id = ?", req.BookID)
	}
	if !req.StartDate.IsZero() {
		sq = sq.Where("tx_date >= ?", req.StartDate)
	}
	if !req.EndDate.IsZero() {
		sq = sq.Where("tx_date <= ?", req.EndDate.AddDate(0, 0, 1))
	}
	type sumRow struct {
		Type string  `gorm:"column:type"`
		Amt  float64 `gorm:"column:amt"`
	}
	var sums []sumRow
	if err := sq.Select("type, SUM(amount) as amt").Group("type").Scan(&sums).Error; err == nil {
		for _, s := range sums {
			switch s.Type {
			case string(models.TxIncome), string(models.TxRefund):
				sumIn += s.Amt
			case string(models.TxExpense):
				sumOut += s.Amt
			}
		}
	}

	PagedOK(c, gin.H{
		"grouped":     grouped,
		"summary":     gin.H{"total_income": sumIn, "total_expense": sumOut, "net": sumIn - sumOut},
		"flat_list":   list,
	}, req.Page, req.PageSize, total)
}

// UpdateTransaction 更新交易
func UpdateTransaction(c *gin.Context) {
	uid := middleware.GetUID(c)
	var reqUri dto.IDRequest
	c.ShouldBindUri(&reqUri)
	var req dto.UpdateTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Bad(c, "参数错误: "+err.Error())
		return
	}

	db := database.DB.Begin()

	var old models.Transaction
	if err := db.Where("id = ? AND user_id = ?", reqUri.ID, uid).First(&old).Error; err != nil {
		db.Rollback()
		NotFound(c, "交易不存在")
		return
	}

	// 先撤销原交易对余额的影响
	var from, to models.Account
	db.First(&from, old.AccountID)
	if old.ToAccountID > 0 {
		db.First(&to, old.ToAccountID)
	}
	updateAccountBalances(db, &old, &from, &to, false)

	// 撤销旧预算 used_amount
	applyBudgetUsed(db, uid, old.BookID, old.CategoryID, old.TxDate, old.Amount, old.Type, old.IncludeInBudget, -1)

	// 更新字段
	updates := map[string]interface{}{
		"book_id":            req.BookID,
		"type":               req.Type,
		"amount":             req.Amount,
		"currency":           req.Currency,
		"exchange_rate":      req.ExchangeRate,
		"category_id":        req.CategoryID,
		"account_id":         req.AccountID,
		"to_account_id":      req.ToAccountID,
		"transfer_fee":       req.TransferFee,
		"transfer_discount":  req.TransferDiscount,
		"refund_of_id":       req.RefundOfID,
		"tx_date":            req.TxDate.T(),
		"description":        req.Description,
		"images":             req.Images,
		"merchant":           req.Merchant,
		"location":           req.Location,
		"include_in_balance": req.IncludeInBalance,
		"include_in_budget":  req.IncludeInBudget,
		"remark":             req.Remark,
		"reimburse_status":   req.ReimburseStatus,
	}
	db.Model(&old).Updates(updates)

	// 标签重新关联
	db.Where("transaction_id = ?", old.ID).Delete(&models.TransactionTag{})
	for _, tid := range req.TagIDs {
		db.Create(&models.TransactionTag{TransactionID: old.ID, TagID: tid})
	}

	// 新余额影响
	var newTx models.Transaction
	db.First(&newTx, old.ID)
	var from2, to2 models.Account
	db.First(&from2, newTx.AccountID)
	if newTx.ToAccountID > 0 {
		db.First(&to2, newTx.ToAccountID)
	}
	updateAccountBalances(db, &newTx, &from2, &to2, true)

	// 应用新预算 used_amount
	applyBudgetUsed(db, uid, newTx.BookID, newTx.CategoryID, newTx.TxDate, newTx.Amount, newTx.Type, newTx.IncludeInBudget, 1)

	db.Commit()

	database.DB.Preload("Tags").First(&newTx, old.ID)
	Broadcast(c, "transactions", "update", old.ID)
	OK(c, newTx)
}

// DeleteTransaction 删除交易（软删除+回滚余额）
func DeleteTransaction(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.IDRequest
	c.ShouldBindUri(&req)

	db := database.DB.Begin()
	var tx models.Transaction
	if err := db.Where("id = ? AND user_id = ?", req.ID, uid).First(&tx).Error; err != nil {
		db.Rollback()
		NotFound(c, "交易不存在")
		return
	}

	var from, to models.Account
	db.First(&from, tx.AccountID)
	if tx.ToAccountID > 0 {
		db.First(&to, tx.ToAccountID)
	}
	updateAccountBalances(db, &tx, &from, &to, false)

	// 撤销预算 used_amount（删除交易）
	applyBudgetUsed(db, uid, tx.BookID, tx.CategoryID, tx.TxDate, tx.Amount, tx.Type, tx.IncludeInBudget, -1)

	db.Delete(&tx)
	db.Commit()
	Broadcast(c, "transactions", "delete", req.ID)
	OK(c, nil)
}

// ========== 批量操作 ==========

type BatchDeleteRequest struct {
	IDs []uint `json:"ids"`
}

func BatchDeleteTransactions(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req BatchDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Bad(c, err.Error())
		return
	}
	// 逐条处理余额回滚
	db := database.DB.Begin()
	var txs []models.Transaction
	db.Where("id IN ? AND user_id = ?", req.IDs, uid).Find(&txs)
	for _, tx := range txs {
		var from, to models.Account
		db.First(&from, tx.AccountID)
		if tx.ToAccountID > 0 {
			db.First(&to, tx.ToAccountID)
		}
		updateAccountBalances(db, &tx, &from, &to, false)
		applyBudgetUsed(db, uid, tx.BookID, tx.CategoryID, tx.TxDate, tx.Amount, tx.Type, tx.IncludeInBudget, -1)
		db.Delete(&tx)
	}
	db.Commit()
	OK(c, gin.H{"deleted_count": len(txs)})
}

// ========== 预算 Budget ==========

func ListBudgets(c *gin.Context) {
	uid := middleware.GetUID(c)
	bookID := c.Query("book_id")
	q := database.DB.Where("user_id = ?", uid)
	if bookID != "" {
		q = q.Where("book_id = ?", bookID)
	}
	var list []models.Budget
	q.Order("start_date DESC, id DESC").Find(&list)
	// 计算剩余
	type budgetView struct {
		models.Budget
		Remaining   float64 `json:"remaining"`
		UsageRate   float64 `json:"usage_rate"`
		IsOverBudget bool    `json:"is_over_budget"`
		DailyBudget float64 `json:"daily_budget"`
	}
	out := make([]budgetView, 0, len(list))
	for _, b := range list {
		v := budgetView{Budget: b, Remaining: b.Amount - b.UsedAmount, UsageRate: 0}
		if b.Amount > 0 {
			v.UsageRate = math.Round(b.UsedAmount/b.Amount*1000) / 1000
		}
		v.IsOverBudget = b.UsedAmount > b.Amount
		days := b.EndDate.Sub(b.StartDate).Hours()/24 + 1
		if days > 0 {
			v.DailyBudget = math.Round((b.Amount-b.UsedAmount)/days*100) / 100
		}
		out = append(out, v)
	}
	OK(c, out)
}

func CreateBudget(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.CreateBudgetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Bad(c, "参数错误: "+err.Error())
		return
	}
	b := models.Budget{
		UserID:     uid,
		BookID:     req.BookID,
		PeriodType: req.PeriodType,
		CategoryID: req.CategoryID,
		Amount:     req.Amount,
		StartDate:  req.StartDate.T(),
		EndDate:    req.EndDate.T(),
		AlertRate:  req.AlertRate,
		RollOver:   req.RollOver,
	}
	if b.AlertRate <= 0 {
		b.AlertRate = 0.8
	}
	database.DB.Create(&b)
	Created(c, b)
}

func UpdateBudget(c *gin.Context) {
	uid := middleware.GetUID(c)
	var reqUri dto.IDRequest
	c.ShouldBindUri(&reqUri)
	var b models.Budget
	if err := c.ShouldBindJSON(&b); err != nil {
		Bad(c, err.Error())
		return
	}
	database.DB.Model(&models.Budget{}).Where("id = ? AND user_id = ?", reqUri.ID, uid).Updates(map[string]interface{}{
		"period_type": b.PeriodType,
		"category_id": b.CategoryID,
		"amount":      b.Amount,
		"start_date":  b.StartDate,
		"end_date":    b.EndDate,
		"alert_rate":  b.AlertRate,
		"roll_over":   b.RollOver,
	})
	var nb models.Budget
	database.DB.First(&nb, reqUri.ID)
	OK(c, nb)
}

func DeleteBudget(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.IDRequest
	c.ShouldBindUri(&req)
	database.DB.Where("id = ? AND user_id = ?", req.ID, uid).Delete(&models.Budget{})
	OK(c, nil)
}
