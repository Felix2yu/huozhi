package handlers

import (
	"fmt"
	"huozhi/internal/database"
	"huozhi/internal/dto"
	"huozhi/internal/models"
	"huozhi/internal/ws"
	"math"
	"sort"
	"strconv"
	"time"

	"gorm.io/gorm"
)

// 本文件把交易（账单）的全部业务逻辑从 gin 层剥离出来，形成与传输协议无关的核心层。
//
// 背景：MCP / 定时任务 / 未来的 CLI 都需要在没有 *gin.Context 的环境下完成
// 「创建一笔账 → 更新账户余额 → 占用预算 → 生成转账手续费派生流水」这一整套动作。
// 若各调用方各自抄一遍，余额、预算、派生流水的口径迟早分叉（本项目历史上已多次
// 因此出现「明细加总 ≠ 统计页」「周期记账与手工记账口径不一致」）。
// 因此这里给出唯一实现，HTTP handler 与 MCP tool 都只是它的薄封装。

// ==================== 错误类型 ====================

// CoreError 与传输协议无关的业务错误。Status 是建议的 HTTP 状态码，
// HTTP handler 直接返回它，MCP 侧按其映射 JSON-RPC 错误码。
type CoreError struct {
	Status  int
	Message string
}

func (e *CoreError) Error() string { return e.Message }

func coreBad(msg string) *CoreError       { return &CoreError{Status: 400, Message: msg} }
func coreForbidden(msg string) *CoreError { return &CoreError{Status: 403, Message: msg} }
func coreNotFound(msg string) *CoreError  { return &CoreError{Status: 404, Message: msg} }
func coreInternal(msg string) *CoreError  { return &CoreError{Status: 500, Message: msg} }

// ==================== 权限 / 范围（供 MCP 等非 HTTP 调用方复用） ====================

// VisibleBookIDs 当前用户可见的账本 ID：自己创建的 + 被邀请加入的共享账本。
func VisibleBookIDs(uid uint) []uint { return accessibleBookIDs(uid) }

// ScopeByBooks 把查询范围限定为「本人 或 本人可见的共享账本」。
// 括号不可省：后续 Where 以 AND 连接，OR 不加括号会被优先级吞掉。
func ScopeByBooks(q *gorm.DB, uid uint, ids []uint) *gorm.DB {
	if len(ids) == 0 {
		return q.Where("user_id = ?", uid)
	}
	return q.Where("(user_id = ? OR book_id IN ?)", uid, ids)
}

// CanWriteBookFor 是否对某账本有写权限（owner 或 editor 成员）。
func CanWriteBookFor(uid uint, ids []uint, bookID uint) bool {
	if bookID == 0 {
		return false
	}
	var book models.Book
	if err := database.DB.Where("id = ? AND user_id = ?", bookID, uid).First(&book).Error; err == nil {
		return true
	}
	if len(ids) == 0 {
		return false
	}
	var mem models.BookMember
	if err := database.DB.Where("book_id = ? AND user_id = ? AND role IN ?",
		bookID, uid, []string{"owner", "editor"}).First(&mem).Error; err == nil {
		return true
	}
	return false
}

// CanWriteTxFor 是否可写某条已存在的交易：自己记的账永远可改，
// 共享账本里他人所记的账需要对该账本有写权限。
func CanWriteTxFor(uid uint, ids []uint, tx *models.Transaction) bool {
	if tx == nil {
		return false
	}
	if tx.UserID == uid {
		return true
	}
	return CanWriteBookFor(uid, ids, tx.BookID)
}

// OwnsOrSharesAccountFor 校验账户归属：本人所有，或属于本人可见的共享账本。
func OwnsOrSharesAccountFor(uid uint, ids []uint, accountID uint) bool {
	if accountID == 0 {
		return false
	}
	var acc models.Account
	if err := database.DB.Where("id = ? AND user_id = ?", accountID, uid).First(&acc).Error; err == nil {
		return true
	}
	if len(ids) == 0 {
		return false
	}
	return database.DB.Where("id = ? AND book_id IN ?", accountID, ids).First(&acc).Error == nil
}

// EnsureSystemCategory 取（或按需创建）系统分类，供 MCP 等非 HTTP 调用方使用。
// 转账 / 余额调整这类后端自动生成的交易必须挂在合法分类上，
// 用户手工删掉后也要能自动补回，避免产生 category_id=0 的脏数据。
func EnsureSystemCategory(uid, bookID uint, name string, kind models.CategoryKind, icon string) uint {
	return ensureSystemCategory(database.DB, uid, bookID, name, kind, icon)
}

// FillTxViewFields 补齐列表/详情返回体里的只读派生字段（折算金额、分类名、账户名）。
func FillTxViewFields(list []models.Transaction) { fillTxViewFields(list) }

// BaseAmountSQL 把原币金额按汇率折算到基准币种后再聚合的 SQL 片段。
// 与 models.Transaction.AmountInBase() 保持同一口径，避免各处手写 CASE WHEN 时漂移。
const BaseAmountSQL = baseAmountExpr

// ==================== 查询 ====================

// GetTx 读取单条交易（读范围与列表一致：本人或可见共享账本）。
func GetTx(uid uint, id uint) (models.Transaction, *CoreError) {
	var tx models.Transaction
	ids := VisibleBookIDs(uid)
	if err := ScopeByBooks(database.DB.Model(&models.Transaction{}), uid, ids).
		Preload("Tags").Where("id = ?", id).First(&tx).Error; err != nil {
		return tx, coreNotFound("交易不存在")
	}
	// 必须放进切片再填充：直接传 []T{tx} 是传值副本，填充结果拿不到。
	list := []models.Transaction{tx}
	fillTxViewFields(list)
	return list[0], nil
}

// TxDayGroup 按日分组的流水（与列表接口返回结构一致）
type TxDayGroup struct {
	Date         string               `json:"date"`
	DayIncome    models.Money         `json:"day_income"`
	DayExpense   models.Money         `json:"day_expense"`
	DayBalance   models.Money         `json:"day_balance"`
	Transactions []models.Transaction `json:"transactions"`
}

// TxListResult 列表查询结果
type TxListResult struct {
	Grouped  []TxDayGroup   `json:"grouped"`
	Summary  map[string]any `json:"summary"`
	Total    int64          `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
}

// ListTx 交易列表（筛选 + 分页 + 按日分组 + 首页汇总）。
func ListTx(uid uint, req dto.QueryTransactionRequest) (TxListResult, *CoreError) {
	res := TxListResult{Grouped: make([]TxDayGroup, 0)}
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 || req.PageSize > 200 {
		req.PageSize = 20
	}
	ids := VisibleBookIDs(uid)

	// buildBase 是列表 / 计数 / 汇总**共用的唯一**筛选构造器，
	// 保证顶部「收支结余」与筛选后的列表口径一致（原 B-03）。
	base := database.DB.Model(&models.Transaction{})
	if req.IncludeDeleted {
		// Unscoped 去掉 GORM 自动追加的 deleted_at IS NULL，只看回收站
		base = database.DB.Unscoped().Model(&models.Transaction{})
	}
	buildBase := func(withBalanceOnly bool) *gorm.DB {
		q := ScopeByBooks(base, uid, ids)
		if req.IncludeDeleted {
			q = q.Where("deleted_at IS NOT NULL")
		}
		if req.BookID > 0 {
			q = q.Where("book_id = ?", req.BookID)
		} else if len(req.BookIDs) > 0 {
			q = q.Where("book_id IN ?", req.BookIDs)
		}
		if req.Type != "" {
			q = q.Where("type = ?", req.Type)
		}
		if req.CategoryID > 0 {
			q = q.Where("category_id = ?", req.CategoryID)
		} else if len(req.CategoryIDs) > 0 {
			q = q.Where("category_id IN ?", req.CategoryIDs)
		}
		if req.AccountID > 0 {
			q = q.Where("account_id = ? OR to_account_id = ?", req.AccountID, req.AccountID)
		} else if len(req.AccountIDs) > 0 {
			q = q.Where("account_id IN ? OR to_account_id IN ?", req.AccountIDs, req.AccountIDs)
		}
		if !req.StartDate.IsZero() {
			q = q.Where("tx_date >= ?", req.StartDate.T())
		}
		// 半开区间 [start, end+1d)，与统计页口径一致（原 B-09）
		if !req.EndDate.IsZero() {
			q = q.Where("tx_date < ?", req.EndDate.T().AddDate(0, 0, 1))
		}
		if req.Keyword != "" {
			k := "%" + req.Keyword + "%"
			like := database.LikeExpr()
			args := []interface{}{k, k, k, k, k}
			amountCond := ""
			// 数字关键词按 ±10% 区间近似匹配：精确等值会让「302」把所有 302.00 元的
			// 流水翻出来（原 B-10）。ParseFloat 会接受 NaN / ±Inf，必须先校验有限性。
			if n, err := strconv.ParseFloat(req.Keyword, 64); err == nil && isFiniteFloat(n) && n > 0 {
				amountCond = " OR (amount >= ? AND amount <= ?)"
				args = append(args, models.FromYuan(n*0.9), models.FromYuan(n*1.1))
			}
			q = q.Where(
				"(description "+like+" ? OR merchant "+like+" ? OR remark "+like+
					" ? OR location "+like+" ? OR category_id IN (SELECT id FROM categories WHERE name "+
					like+" ?)"+amountCond+")",
				args...,
			)
		}
		if req.MinAmount > 0 {
			q = q.Where("amount >= ?", models.FromYuan(req.MinAmount))
		}
		if req.MaxAmount > 0 {
			q = q.Where("amount <= ?", models.FromYuan(req.MaxAmount))
		}
		if req.TagID > 0 {
			q = q.Joins("JOIN transaction_tags tt ON transactions.id = tt.transaction_id").
				Where("tt.tag_id = ?", req.TagID)
		} else if len(req.TagIDs) > 0 {
			q = q.Joins("JOIN transaction_tags tt ON transactions.id = tt.transaction_id").
				Where("tt.tag_id IN ?", req.TagIDs)
		}
		if req.ReimburseStatus != "" {
			q = q.Where("reimburse_status = ?", req.ReimburseStatus)
		}
		if withBalanceOnly {
			q = q.Where("include_in_balance = ?", true)
		}
		return q
	}

	var total int64
	buildBase(false).Count(&total)

	var list []models.Transaction
	q := buildBase(false).Preload("Tags")
	if req.UseCursor() {
		// 游标分页：从上一页最后一条的 (tx_date, id) 继续，天然免疫插入位移（原 F-04）
		cd := req.CursorDate.T()
		q = q.Where("(tx_date < ? OR (tx_date = ? AND id < ?))", cd, cd, req.CursorID)
	} else {
		q = q.Offset((req.Page - 1) * req.PageSize)
	}
	q.Order("tx_date DESC, id DESC").Limit(req.PageSize).Find(&list)
	fillTxViewFields(list)

	dayMap := make(map[string][]models.Transaction)
	var dayOrder []string
	for _, t := range list {
		day := t.TxDate.Format("2006-01-02")
		if _, ok := dayMap[day]; !ok {
			dayOrder = append(dayOrder, day)
		}
		dayMap[day] = append(dayMap[day], t)
	}
	for _, d := range dayOrder {
		g := TxDayGroup{Date: d, Transactions: dayMap[d]}
		for i := range dayMap[d] {
			t := dayMap[d][i]
			if !t.IncludeInBalance {
				continue
			}
			// 收支口径与资金方向同源：models.TxStatsBucket（reimburse 必须计入支出）
			switch models.TxStatsBucket(t.Type) {
			case models.StatsBucketIncome:
				g.DayIncome += t.AmountInBase()
			case models.StatsBucketExpense:
				g.DayExpense += t.AmountInBase()
			}
		}
		g.DayBalance = g.DayIncome - g.DayExpense
		res.Grouped = append(res.Grouped, g)
	}

	// 汇总只在首页计算：「加载更多」每翻一页都重复全表聚合、结果完全相同（原 P-01）
	var sumIn, sumOut models.Money
	res.Summary = map[string]any{"total_income": sumIn, "total_expense": sumOut, "net": sumIn - sumOut}
	if !req.UseCursor() && req.Page == 1 {
		type sumRow struct {
			Type string  `gorm:"column:type"`
			Amt  float64 `gorm:"column:amt"`
		}
		var sums []sumRow
		// 外币笔设在 SQL 层折算，避免先累加再乘导致精度失真
		err := buildBase(true).
			Select("type, SUM(" + BaseAmountSQL + ") as amt").
			Group("type").Scan(&sums).Error
		if err == nil {
			for _, s := range sums {
				switch models.TxStatsBucket(models.TransactionType(s.Type)) {
				case models.StatsBucketIncome:
					sumIn += models.FromCents(s.Amt)
				case models.StatsBucketExpense:
					sumOut += models.FromCents(s.Amt)
				}
			}
		}
		res.Summary = map[string]any{"total_income": sumIn, "total_expense": sumOut, "net": sumIn - sumOut}
	}

	res.Total = total
	res.Page = req.Page
	res.PageSize = req.PageSize
	return res, nil
}

// ListDeletedTx 回收站：列出软删除的交易
func ListDeletedTx(uid uint, page, pageSize int) ([]models.Transaction, *CoreError) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}
	ids := VisibleBookIDs(uid)
	var list []models.Transaction
	ScopeByBooks(database.DB.Unscoped().Model(&models.Transaction{}), uid, ids).
		Preload("Tags").
		Where("deleted_at IS NOT NULL").
		Order("deleted_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&list)
	fillTxViewFields(list)
	return list, nil
}

// ==================== 写入 ====================

// CreateTx 创建交易（同步更新账户余额、占用预算、生成转账手续费派生流水）。
func CreateTx(uid uint, req dto.CreateTransactionRequest) (models.Transaction, *CoreError) {
	var out models.Transaction
	if req.Type == "transfer" && req.ToAccountID == 0 {
		return out, coreBad("转账需要指定目标账户")
	}
	// 汇率兜底：0 / 负数 / NaN / ±Inf 一律按 1，放任 0 入库会让后续折算全变 0。
	if req.ExchangeRate <= 0 || math.IsNaN(req.ExchangeRate) || math.IsInf(req.ExchangeRate, 0) {
		req.ExchangeRate = 1
	}
	if req.ReimburseStatus == "" {
		req.ReimburseStatus = "none"
	}

	ids := VisibleBookIDs(uid)
	dbtx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			dbtx.Rollback()
		}
	}()

	// 共享账本里的账户可能由他人创建，不能只按 user_id 校验（C5）
	var fromAcc, toAcc models.Account
	accQuery := func(id uint, out *models.Account) error {
		if len(ids) > 0 {
			return dbtx.Where("id = ? AND (user_id = ? OR book_id IN ?)", id, uid, ids).First(out).Error
		}
		return dbtx.Where("id = ? AND user_id = ?", id, uid).First(out).Error
	}
	if err := accQuery(req.AccountID, &fromAcc); err != nil {
		dbtx.Rollback()
		return out, coreBad("来源账户不存在")
	}
	if req.ToAccountID > 0 {
		if err := accQuery(req.ToAccountID, &toAcc); err != nil {
			dbtx.Rollback()
			return out, coreBad("目标账户不存在")
		}
	}
	if !CanWriteBookFor(uid, ids, req.BookID) {
		dbtx.Rollback()
		return out, coreForbidden("对该账本无写入权限")
	}

	newTx := models.Transaction{
		UserID:           uid,
		BookID:           req.BookID,
		Type:             models.TransactionType(req.Type),
		Amount:           models.FromYuan(req.Amount),
		Currency:         firstNotEmpty(req.Currency, "CNY"),
		ExchangeRate:     req.ExchangeRate,
		CategoryID:       req.CategoryID,
		AccountID:        req.AccountID,
		ToAccountID:      req.ToAccountID,
		TransferFee:      models.FromYuan(req.TransferFee),
		TransferDiscount: models.FromYuan(req.TransferDiscount),
		RefundOfID:       req.RefundOfID,
		TxDate:           req.TxDate.T(),
		Description:      req.Description,
		Images:           req.Images,
		Merchant:         req.Merchant,
		Location:         req.Location,
		IncludeInBalance: req.BalanceFlag(),
		IncludeInBudget:  req.BudgetFlag(),
		RecurringID:      req.RecurringID,
		InstallmentID:    req.InstallmentID,
		Remark:           req.Remark,
		ReimburseStatus:  req.ReimburseStatus,
	}
	if err := dbtx.Create(&newTx).Error; err != nil {
		dbtx.Rollback()
		return out, coreInternal("创建交易失败: " + err.Error())
	}

	if len(req.TagIDs) > 0 {
		for _, tid := range req.TagIDs {
			dbtx.Create(&models.TransactionTag{TransactionID: newTx.ID, TagID: tid})
		}
		adjustTagCounts(dbtx, req.TagIDs, 1)
		var tags []models.Tag
		dbtx.Where("id IN ?", req.TagIDs).Find(&tags)
		tagPtrs := make([]*models.Tag, len(tags))
		for i := range tags {
			tagPtrs[i] = &tags[i]
		}
		newTx.Tags = tagPtrs
	}

	updateAccountBalances(dbtx, &newTx, &fromAcc, &toAcc, true)
	syncTransferFeeDerived(dbtx, &newTx, &fromAcc)
	applyBudgetUsed(dbtx, uid, newTx.BookID, newTx.CategoryID, newTx.TxDate,
		newTx.AmountInBase(), newTx.Type, newTx.IncludeInBudget, 1)

	if err := dbtx.Commit().Error; err != nil {
		dbtx.Rollback()
		return out, coreInternal("提交失败")
	}

	// 预算提醒只针对本次支出真正相关的预算，且只在跨过阈值那一刻推送（原 B-11）
	broadcastBudgetAlerts(uid, newTx)

	database.DB.Preload("Tags").First(&newTx, newTx.ID)
	enriched := []models.Transaction{newTx}
	fillTxViewFields(enriched)
	ws.DefaultHub.Broadcast(uid, "transactions", "create", newTx.ID)
	return enriched[0], nil
}

// UpdateTx 更新交易（补丁语义：只有显式传入的字段才写库）。
func UpdateTx(uid uint, id uint, req dto.UpdateTransactionRequest) (models.Transaction, *CoreError) {
	var out models.Transaction
	ids := VisibleBookIDs(uid)
	db := database.DB.Begin()

	var old models.Transaction
	if err := ScopeByBooks(db.Model(&models.Transaction{}), uid, ids).
		Where("id = ?", id).First(&old).Error; err != nil {
		db.Rollback()
		return out, coreNotFound("交易不存在")
	}
	if !CanWriteTxFor(uid, ids, &old) {
		db.Rollback()
		return out, coreForbidden("对该账本无写入权限")
	}

	// 目标状态 = 旧值 + 补丁。未出现在请求体里的字段保持原样。
	target := old
	if req.BookID != nil && *req.BookID > 0 {
		target.BookID = *req.BookID
	}
	if req.Type != nil {
		target.Type = models.TransactionType(*req.Type)
	}
	if req.Amount != nil {
		target.Amount = models.FromYuan(*req.Amount)
	}
	if req.Currency != nil && *req.Currency != "" {
		target.Currency = *req.Currency
	}
	if req.ExchangeRate != nil {
		target.ExchangeRate = *req.ExchangeRate
	}
	if target.ExchangeRate <= 0 || math.IsNaN(target.ExchangeRate) || math.IsInf(target.ExchangeRate, 0) {
		target.ExchangeRate = 1
	}
	if req.CategoryID != nil {
		target.CategoryID = *req.CategoryID
	}
	if req.AccountID != nil {
		target.AccountID = *req.AccountID
	}
	if req.ToAccountID != nil {
		target.ToAccountID = *req.ToAccountID
	}
	if req.TransferFee != nil {
		target.TransferFee = models.FromYuan(*req.TransferFee)
	}
	if req.TransferDiscount != nil {
		target.TransferDiscount = models.FromYuan(*req.TransferDiscount)
	}
	if req.RefundOfID != nil {
		target.RefundOfID = *req.RefundOfID
	}
	if req.TxDate != nil {
		target.TxDate = req.TxDate.T()
	}
	if req.Description != nil {
		target.Description = *req.Description
	}
	if req.Images != nil {
		target.Images = *req.Images
	}
	if req.Merchant != nil {
		target.Merchant = *req.Merchant
	}
	if req.Location != nil {
		target.Location = *req.Location
	}
	if req.IncludeInBalance != nil {
		target.IncludeInBalance = *req.IncludeInBalance
	}
	if req.IncludeInBudget != nil {
		target.IncludeInBudget = *req.IncludeInBudget
	}
	if req.Remark != nil {
		target.Remark = *req.Remark
	}
	target.ReimburseStatus = req.NormalizedReimburseStatus(old.ReimburseStatus)

	// C14：类型白名单
	switch target.Type {
	case models.TxExpense, models.TxIncome, models.TxTransfer,
		models.TxRefund, models.TxReimburse, models.TxAdjust:
	default:
		db.Rollback()
		return out, coreBad("非法交易类型: " + string(target.Type))
	}
	// C14：归属校验 —— 账户/目标账户/分类必须属于当前用户（或共享账本）
	if !OwnsOrSharesAccountFor(uid, ids, target.AccountID) {
		db.Rollback()
		return out, coreBad("来源账户不存在或无权限")
	}
	if target.ToAccountID > 0 && !OwnsOrSharesAccountFor(uid, ids, target.ToAccountID) {
		db.Rollback()
		return out, coreBad("目标账户不存在或无权限")
	}
	if target.CategoryID > 0 {
		var cnt int64
		db.Model(&models.Category{}).
			Where("id = ? AND (user_id = ? OR book_id IN ?)",
				target.CategoryID, uid, append(ids, 0)).Count(&cnt)
		if cnt == 0 {
			db.Rollback()
			return out, coreBad("分类不存在或无权限")
		}
	}
	if target.Type == models.TxTransfer && target.ToAccountID == 0 {
		db.Rollback()
		return out, coreBad("转账需要指定目标账户")
	}
	if !CanWriteBookFor(uid, ids, target.BookID) && old.UserID != uid {
		db.Rollback()
		return out, coreForbidden("对该账本无写入权限")
	}

	// 撤销该交易派生出的子交易（手续费）
	revertDerivedTransactions(db, old.ID)

	var from, to models.Account
	db.First(&from, old.AccountID)
	if old.ToAccountID > 0 {
		db.First(&to, old.ToAccountID)
	}
	updateAccountBalances(db, &old, &from, &to, false)
	applyBudgetUsed(db, uid, old.BookID, old.CategoryID, old.TxDate,
		old.AmountInBase(), old.Type, old.IncludeInBudget, -1)

	// 只写本次真正提交的字段（补丁语义），其余保持库里的原值（原 B-01）
	updates := map[string]interface{}{}
	if req.BookID != nil {
		updates["book_id"] = target.BookID
	}
	if req.Type != nil {
		updates["type"] = target.Type
	}
	if req.Amount != nil {
		updates["amount"] = target.Amount
	}
	if req.Currency != nil {
		updates["currency"] = target.Currency
	}
	if req.ExchangeRate != nil || old.ExchangeRate != target.ExchangeRate {
		updates["exchange_rate"] = target.ExchangeRate
	}
	if req.CategoryID != nil {
		updates["category_id"] = target.CategoryID
	}
	if req.AccountID != nil {
		updates["account_id"] = target.AccountID
	}
	if req.ToAccountID != nil {
		updates["to_account_id"] = target.ToAccountID
	}
	if req.TransferFee != nil {
		updates["transfer_fee"] = target.TransferFee
	}
	if req.TransferDiscount != nil {
		updates["transfer_discount"] = target.TransferDiscount
	}
	if req.RefundOfID != nil {
		updates["refund_of_id"] = target.RefundOfID
	}
	if req.TxDate != nil {
		updates["tx_date"] = target.TxDate
	}
	if req.Description != nil {
		updates["description"] = target.Description
	}
	if req.Images != nil {
		updates["images"] = target.Images
	}
	if req.Merchant != nil {
		updates["merchant"] = target.Merchant
	}
	if req.Location != nil {
		updates["location"] = target.Location
	}
	if req.IncludeInBalance != nil {
		updates["include_in_balance"] = target.IncludeInBalance
	}
	if req.IncludeInBudget != nil {
		updates["include_in_budget"] = target.IncludeInBudget
	}
	if req.Remark != nil {
		updates["remark"] = target.Remark
	}
	if req.ReimburseStatus != nil || old.ReimburseStatus != target.ReimburseStatus {
		updates["reimburse_status"] = target.ReimburseStatus
	}
	if len(updates) > 0 {
		if err := db.Model(&models.Transaction{}).Where("id = ?", old.ID).Updates(updates).Error; err != nil {
			db.Rollback()
			return out, coreInternal("更新失败: " + err.Error())
		}
	}

	if req.HasTagIDs() {
		syncTxTags(db, old.ID, req.TagIDsOrNil())
	}

	var newTx models.Transaction
	db.Preload("Tags").First(&newTx, old.ID)
	var from2, to2 models.Account
	db.First(&from2, newTx.AccountID)
	if newTx.ToAccountID > 0 {
		db.First(&to2, newTx.ToAccountID)
	}
	updateAccountBalances(db, &newTx, &from2, &to2, true)
	applyBudgetUsed(db, uid, newTx.BookID, newTx.CategoryID, newTx.TxDate,
		newTx.AmountInBase(), newTx.Type, newTx.IncludeInBudget, 1)
	// 转账手续费派生交易：旧的已在前面回滚，这里按最新状态重建
	syncTransferFeeDerived(db, &newTx, &from2)

	if err := db.Commit().Error; err != nil {
		return out, coreInternal("提交失败: " + err.Error())
	}

	database.DB.Preload("Tags").First(&newTx, old.ID)
	updated := []models.Transaction{newTx}
	fillTxViewFields(updated)
	ws.DefaultHub.Broadcast(uid, "transactions", "update", old.ID)
	return updated[0], nil
}

// DeleteTx 删除交易（软删除 + 回滚余额与预算）。
func DeleteTx(uid uint, id uint) *CoreError {
	ids := VisibleBookIDs(uid)
	db := database.DB.Begin()
	var tx models.Transaction
	if err := ScopeByBooks(db.Model(&models.Transaction{}), uid, ids).
		Where("id = ?", id).First(&tx).Error; err != nil {
		db.Rollback()
		return coreNotFound("交易不存在")
	}
	if !CanWriteTxFor(uid, ids, &tx) {
		db.Rollback()
		return coreForbidden("对该账本无写入权限")
	}

	revertDerivedTransactions(db, tx.ID)

	var from, to models.Account
	db.First(&from, tx.AccountID)
	if tx.ToAccountID > 0 {
		db.First(&to, tx.ToAccountID)
	}
	updateAccountBalances(db, &tx, &from, &to, false)
	applyBudgetUsed(db, uid, tx.BookID, tx.CategoryID, tx.TxDate,
		tx.AmountInBase(), tx.Type, tx.IncludeInBudget, -1)
	adjustTagCounts(db, txTagIDs(db, tx.ID), -1)

	// 不删除 transaction_tags 关联行：RecoverTransaction 依赖它读回标签，
	// 硬删会让回收站恢复后标签绑定永久丢失（原 B-06）。
	db.Delete(&tx)
	if err := db.Commit().Error; err != nil {
		return coreInternal("删除失败: " + err.Error())
	}
	ws.DefaultHub.Broadcast(uid, "transactions", "delete", id)
	return nil
}

// BatchDeleteTx 批量删除，返回实际删除条数。
func BatchDeleteTx(uid uint, rawIDs []uint) (int, *CoreError) {
	if len(rawIDs) == 0 {
		return 0, coreBad("请选择要删除的交易")
	}
	// 去重 + 限流：id IN (?) 参数个数不受控会触发 SQLite 默认 999 参数上限（原 B-08）
	seen := make(map[uint]struct{}, len(rawIDs))
	ids := make([]uint, 0, len(rawIDs))
	for _, id := range rawIDs {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) > maxBatchDelete {
		return 0, coreBad(fmt.Sprintf("单次最多删除 %d 笔，请分批操作", maxBatchDelete))
	}

	bookIDs := VisibleBookIDs(uid)
	db := database.DB.Begin()

	var txs []models.Transaction
	ScopeByBooks(db.Model(&models.Transaction{}), uid, bookIDs).Where("id IN ?", ids).Find(&txs)
	// 写权限过滤：共享账本里他人所记、且自己只有 viewer 角色的流水跳过（原 B-05）
	writable := make([]models.Transaction, 0, len(txs))
	writableIDs := make([]uint, 0, len(txs))
	for i := range txs {
		if !CanWriteTxFor(uid, bookIDs, &txs[i]) {
			continue
		}
		writable = append(writable, txs[i])
		writableIDs = append(writableIDs, txs[i].ID)
	}
	if len(writable) == 0 {
		db.Rollback()
		return 0, coreForbidden("没有可删除的交易")
	}

	tagMap := txTagIDsBatch(db, writableIDs)

	accSet := make(map[uint]struct{}, len(writable)*2)
	for i := range writable {
		accSet[writable[i].AccountID] = struct{}{}
		if writable[i].ToAccountID > 0 {
			accSet[writable[i].ToAccountID] = struct{}{}
		}
	}
	accMap := make(map[uint]models.Account, len(accSet))
	if len(accSet) > 0 {
		var accs []models.Account
		db.Where("id IN ?", idSetKeys(accSet)).Find(&accs)
		for i := range accs {
			accMap[accs[i].ID] = accs[i]
		}
	}

	for i := range writable {
		revertDerivedTransactions(db, writable[i].ID)
	}

	type budgetKey struct {
		bookID uint
		catID  uint
		date   string
	}
	balanceDeltas := make(map[uint]int64, len(accSet))
	budgetDeltas := make(map[budgetKey]models.Money, len(writable))
	tagCount := make(map[uint]int, len(writable))
	for i := range writable {
		t := writable[i]
		for _, tid := range tagMap[t.ID] {
			tagCount[tid]++
		}
		from := accMap[t.AccountID]
		var to *models.Account
		if t.ToAccountID > 0 {
			if a, ok := accMap[t.ToAccountID]; ok {
				to = &a
			}
		}
		accumulateBalanceDelta(&t, &from, to, balanceDeltas, -1)
		if models.TxStatsBucket(t.Type) == models.StatsBucketExpense && t.IncludeInBudget {
			k := budgetKey{bookID: t.BookID, catID: t.CategoryID, date: t.TxDate.Format("2006-01-02")}
			budgetDeltas[k] += t.AmountInBase()
		}
	}
	flushBalanceDeltas(db, balanceDeltas)
	if len(budgetDeltas) > 0 {
		keys := make([]budgetKey, 0, len(budgetDeltas))
		for k := range budgetDeltas {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return keys[i].date < keys[j].date })
		for _, k := range keys {
			d, err := time.Parse("2006-01-02", k.date)
			if err != nil {
				continue
			}
			applyBudgetUsed(db, uid, k.bookID, k.catID, d, budgetDeltas[k], models.TxExpense, true, -1)
		}
	}
	adjustTagCountsBatch(db, tagCount, -1)

	db.Where("id IN ?", writableIDs).Delete(&models.Transaction{})
	if err := db.Commit().Error; err != nil {
		return 0, coreInternal("批量删除失败: " + err.Error())
	}
	ws.DefaultHub.Broadcast(uid, "transactions", "delete", 0)
	return len(writable), nil
}

// RecoverTx 撤销软删除（回收站恢复），并补回余额与预算。
func RecoverTx(uid uint, id uint) (models.Transaction, *CoreError) {
	var out models.Transaction
	ids := VisibleBookIDs(uid)
	db := database.DB.Begin()
	var tx models.Transaction
	if err := ScopeByBooks(db.Unscoped().Model(&models.Transaction{}), uid, ids).
		Where("id = ? AND deleted_at IS NOT NULL", id).First(&tx).Error; err != nil {
		db.Rollback()
		return out, coreNotFound("未找到已删除的交易")
	}
	if !CanWriteTxFor(uid, ids, &tx) {
		db.Rollback()
		return out, coreForbidden("对该账本无写入权限")
	}
	if err := db.Unscoped().Model(&models.Transaction{}).Where("id = ?", id).
		Update("deleted_at", nil).Error; err != nil {
		db.Rollback()
		return out, coreInternal("恢复失败: " + err.Error())
	}
	var from, to models.Account
	db.First(&from, tx.AccountID)
	if tx.ToAccountID > 0 {
		db.First(&to, tx.ToAccountID)
	}
	updateAccountBalances(db, &tx, &from, &to, true)
	applyBudgetUsed(db, uid, tx.BookID, tx.CategoryID, tx.TxDate,
		tx.AmountInBase(), tx.Type, tx.IncludeInBudget, 1)
	// 标签关联在删除时被刻意保留，这里一定能读回来（原 B-06）
	adjustTagCounts(db, txTagIDs(db, tx.ID), 1)
	// 重建派生交易（转账手续费）：删除时被硬删以保证回收站不留系统垃圾（原 B-07）
	syncTransferFeeDerived(db, &tx, &from)
	if err := db.Commit().Error; err != nil {
		return out, coreInternal("恢复失败: " + err.Error())
	}
	ws.DefaultHub.Broadcast(uid, "transactions", "recover", id)
	database.DB.Preload("Tags").First(&tx, id)
	list := []models.Transaction{tx}
	fillTxViewFields(list)
	return list[0], nil
}
