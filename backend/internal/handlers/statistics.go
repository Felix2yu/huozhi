package handlers

import (
	"fmt"
	"sort"
	"huozhi/internal/database"
	"huozhi/internal/dto"
	"huozhi/internal/middleware"
	"huozhi/internal/models"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ========== 统计分析 Statistics ==========

// GetStatistics 多维度统计
func GetStatistics(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.StatisticsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		Bad(c, "参数错误: "+err.Error())
		return
	}

	// 基础查询构造器：每次返回全新查询。GORM 链式 Where 会在同一语句上累积条件，
	// 复用同一个 *gorm.DB 追加不同 type 条件会导致后续查询自相矛盾而查空。
	base := func() *gorm.DB {
		q := database.DB.Model(&models.Transaction{}).Where("user_id = ?", uid)
		if req.BookID > 0 {
			q = q.Where("book_id = ?", req.BookID)
		}
		return q.Where("tx_date >= ? AND tx_date < ? AND include_in_balance = ?",
			req.StartDate, req.EndDate.AddDate(0, 0, 1), true)
	}

	// 基础汇总
	var totalIncome, totalExpense models.Money
	var incomeCount, expenseCount int64
	var incSum, expSum float64
	base().Where("type IN ?", []string{string(models.TxIncome), string(models.TxRefund)}).
		Select("COALESCE(SUM(amount), 0), COUNT(*)").Row().Scan(&incSum, &incomeCount)
	base().Where("type = ?", string(models.TxExpense)).
		Select("COALESCE(SUM(amount), 0), COUNT(*)").Row().Scan(&expSum, &expenseCount)
	totalIncome = models.FromCents(incSum)
	totalExpense = models.FromCents(expSum)

	result := gin.H{
		"range": gin.H{
			"start":         req.StartDate.Format("2006-01-02"),
			"end":           req.EndDate.Format("2006-01-02"),
			"days":          int(req.EndDate.T().Sub(req.StartDate.T()).Hours()/24) + 1,
		},
		"summary": gin.H{
			"total_income":  totalIncome,
			"total_expense": totalExpense,
			"net":           totalIncome - totalExpense,
			"income_count":  incomeCount,
			"expense_count": expenseCount,
			"transaction_count": incomeCount + expenseCount,
			"avg_daily_expense": 0.0,
		},
	}
	days := int(req.EndDate.T().Sub(req.StartDate.T()).Hours()/24) + 1
	if days > 0 {
		result["summary"].(gin.H)["avg_daily_expense"] = round2(totalExpense.Yuan() / float64(days))
		result["summary"].(gin.H)["avg_daily_income"] = round2(totalIncome.Yuan() / float64(days))
	}

	// ========== 按维度分类 ==========
	dimension := req.Dimension
	if dimension == "" {
		dimension = "category"
	}

	// 1) 按分类
	if dimension == "category" || dimension == "all" {
		var catRows []struct {
			CategoryID uint
			Type       string
			SumAmount  float64
			Count      int64
		}
		base().Select("category_id, type, SUM(amount) sum_amount, COUNT(*) count").
			Where("type IN ?", []string{string(models.TxExpense), string(models.TxIncome), string(models.TxRefund)}).
			Group("category_id, type").Scan(&catRows)

		var catMap = make(map[uint]map[string]interface{})
		for _, r := range catRows {
			k := r.CategoryID
			if _, ok := catMap[k]; !ok {
				catMap[k] = map[string]interface{}{
					"category_id": r.CategoryID,
					"income":      models.Money(0),
					"expense":     models.Money(0),
					"count":       int64(0),
				}
			}
			switch r.Type {
			case string(models.TxIncome), string(models.TxRefund):
				catMap[k]["income"] = catMap[k]["income"].(models.Money) + models.FromCents(r.SumAmount)
			case string(models.TxExpense):
				catMap[k]["expense"] = catMap[k]["expense"].(models.Money) + models.FromCents(r.SumAmount)
			}
			catMap[k]["count"] = catMap[k]["count"].(int64) + r.Count
		}

		// 取分类信息
		catIDs := make([]uint, 0, len(catMap))
		for id := range catMap {
			catIDs = append(catIDs, id)
		}
		var cats []models.Category
		database.DB.Where("id IN ?", catIDs).Find(&cats)
		catInfo := make(map[uint]models.Category)
		for _, c := range cats {
			catInfo[c.ID] = c
		}

		// 用 make([]T, 0) 初始化，保证空数据时序列化为 [] 而非 null，
		// 否则前端 data.by_category_expense.length 会因 null 抛 TypeError 而崩溃渲染。
		expenseRank := make([]categoryRankItem, 0)
		incomeRank := make([]categoryRankItem, 0)

		for id, m := range catMap {
			info := catInfo[id]
			expAmt := m["expense"].(models.Money)
			incAmt := m["income"].(models.Money)
			if expAmt > 0 {
				pct := 0.0
				if totalExpense > 0 {
					pct = round2(expAmt.Yuan() / totalExpense.Yuan() * 100)
				}
				expenseRank = append(expenseRank, categoryRankItem{
					ID: id, Name: info.Name, Icon: info.Icon, Color: info.Color,
					Kind: string(info.Kind), Amount: expAmt, Count: m["count"].(int64),
					Percent: pct, ParentID: info.ParentID,
				})
			}
			if incAmt > 0 {
				pct := 0.0
				if totalIncome > 0 {
					pct = round2(incAmt.Yuan() / totalIncome.Yuan() * 100)
				}
				incomeRank = append(incomeRank, categoryRankItem{
					ID: id, Name: info.Name, Icon: info.Icon, Color: info.Color,
					Kind: string(info.Kind), Amount: incAmt, Count: m["count"].(int64),
					Percent: pct, ParentID: info.ParentID,
				})
			}
		}
		// 排序
		sortByAmountDesc(expenseRank)
		sortByAmountDesc(incomeRank)
		result["by_category_expense"] = expenseRank
		result["by_category_income"] = incomeRank
	}

	// 2) 按账本（全部账本聚合视图下可见各账本开销）
	if dimension == "book" || dimension == "all" {
		var bookRows []struct {
			BookID    uint
			Type      string
			SumAmount float64
		}
		base().Select("book_id, type, SUM(amount) sum_amount").
			Where("type IN ?", []string{string(models.TxExpense), string(models.TxIncome), string(models.TxRefund)}).
			Group("book_id, type").Scan(&bookRows)
		out := map[uint]gin.H{}
		for _, r := range bookRows {
			if _, ok := out[r.BookID]; !ok {
				out[r.BookID] = gin.H{"book_id": r.BookID, "income": models.Money(0), "expense": models.Money(0)}
			}
			switch r.Type {
			case string(models.TxIncome), string(models.TxRefund):
				out[r.BookID]["income"] = out[r.BookID]["income"].(models.Money) + models.FromCents(r.SumAmount)
			case string(models.TxExpense):
				out[r.BookID]["expense"] = out[r.BookID]["expense"].(models.Money) + models.FromCents(r.SumAmount)
			}
		}
		// 补充账本名称
		bookIDs := make([]uint, 0, len(out))
		for id := range out {
			bookIDs = append(bookIDs, id)
		}
		if len(bookIDs) > 0 {
			var books []models.Book
			database.DB.Where("id IN ?", bookIDs).Find(&books)
			for _, b := range books {
				if _, ok := out[b.ID]; ok {
					out[b.ID]["book_name"] = b.Name
					out[b.ID]["icon"] = b.Icon
				}
			}
		}
		result["by_book"] = out
	}

	// 2b) 按账户
	if dimension == "account" || dimension == "all" {
		var accRows []struct {
			AccountID uint
			Type      string
			SumAmount float64
		}
		base().Select("account_id, type, SUM(amount) sum_amount").
			Where("type IN ?", []string{string(models.TxExpense), string(models.TxIncome), string(models.TxRefund)}).
			Group("account_id, type").Scan(&accRows)
		out := map[uint]gin.H{}
		for _, r := range accRows {
			if _, ok := out[r.AccountID]; !ok {
				out[r.AccountID] = gin.H{"account_id": r.AccountID, "income": models.Money(0), "expense": models.Money(0)}
			}
			switch r.Type {
			case string(models.TxIncome), string(models.TxRefund):
				out[r.AccountID]["income"] = out[r.AccountID]["income"].(models.Money) + models.FromCents(r.SumAmount)
			case string(models.TxExpense):
				out[r.AccountID]["expense"] = out[r.AccountID]["expense"].(models.Money) + models.FromCents(r.SumAmount)
			}
		}
		result["by_account"] = out
	}

	// 3) 按时间趋势（每日/每月）
	var trendRows []struct {
		Day       string
		Type      string
		SumAmount float64
	}
	// 按方言生成日期分组表达式：SQLite 用 strftime，PostgreSQL 用 to_char。
	dateGroup := database.DateGroupExpr("tx_date", dateGrainOf(dimension))
	base().Select(fmt.Sprintf("%s day, type, SUM(amount) sum_amount", dateGroup)).
		Where("type IN ?", []string{string(models.TxExpense), string(models.TxIncome), string(models.TxRefund)}).
		Group("day, type").Order("day ASC").Scan(&trendRows)

	type trendPoint struct {
		Date    string       `json:"date"`
		Income  models.Money `json:"income"`
		Expense models.Money `json:"expense"`
		Net     models.Money `json:"net"`
	}
	trendMap := make(map[string]*trendPoint)
	var trendOrder []string
	for _, r := range trendRows {
		if _, ok := trendMap[r.Day]; !ok {
			trendMap[r.Day] = &trendPoint{Date: r.Day}
			trendOrder = append(trendOrder, r.Day)
		}
		switch r.Type {
		case string(models.TxIncome), string(models.TxRefund):
			trendMap[r.Day].Income += models.FromCents(r.SumAmount)
		case string(models.TxExpense):
			trendMap[r.Day].Expense += models.FromCents(r.SumAmount)
		}
	}
	trendList := make([]trendPoint, 0, len(trendOrder))
	for _, d := range trendOrder {
		t := trendMap[d]
		t.Net = t.Income - t.Expense
		trendList = append(trendList, *t)
	}
	result["trend"] = trendList

	// 4) Top 支出排行榜
	// 用 make([]T, 0) 初始化，保证空数据时序列化为 [] 而非 null。
	topExp := make([]struct {
		ID          uint         `json:"id"`
		Amount      models.Money `json:"amount"`
		Description string       `json:"description"`
		TxDate      time.Time    `json:"tx_date"`
		CategoryID  uint         `json:"category_id"`
		Merchant    string       `json:"merchant"`
	}, 0)
	base().Where("type = ?", string(models.TxExpense)).Order("amount DESC").Limit(10).
		Select("id, amount, description, tx_date, category_id, merchant").Scan(&topExp)
	result["top_expense"] = topExp

	// 5) 资产曲线（月度资产快照）
	snapshots := make([]models.AssetSnapshot, 0)
	database.DB.Where("user_id = ? AND snap_date >= ? AND snap_date <= ?",
		uid, req.StartDate, req.EndDate).Order("snap_date ASC").Find(&snapshots)
	result["asset_snapshots"] = snapshots

	OK(c, result)
}

// GetAssetOverview 资产总览
func GetAssetOverview(c *gin.Context) {
	uid := middleware.GetUID(c)
	var accounts []models.Account
	database.DB.Where("user_id = ? AND is_archived = ?", uid, false).Find(&accounts)

	var totalAsset, totalDebt, cashOnHand models.Money
	assetByType := map[string]models.Money{}
	for _, a := range accounts {
		if !a.IncludeInTotal {
			continue
		}
		switch a.Type {
		case models.AccLiability:
			totalDebt += a.Balance
		case models.AccCredit:
			// 信用卡：已出账单=负债；这里简化：余额视为应还款
			if a.Balance > 0 {
				totalDebt += a.Balance
			}
		default:
			totalAsset += a.Balance
			assetByType[string(a.Type)] += a.Balance
			if a.Type == models.AccCash || a.Type == models.AccVirtual {
				cashOnHand += a.Balance
			}
		}
	}

	// 本月收支
	now := time.Now()
	first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	last := first.AddDate(0, 1, 0)
	var monthIncome, monthExpense models.Money
	var mi, me float64
	database.DB.Model(&models.Transaction{}).
		Where("user_id = ? AND tx_date >= ? AND tx_date < ? AND type IN ? AND include_in_balance = ?",
			uid, first, last, []string{string(models.TxIncome), string(models.TxRefund)}, true).
		Select("COALESCE(SUM(amount), 0)").Row().Scan(&mi)
	database.DB.Model(&models.Transaction{}).
		Where("user_id = ? AND tx_date >= ? AND tx_date < ? AND type = ? AND include_in_balance = ?",
			uid, first, last, string(models.TxExpense), true).
		Select("COALESCE(SUM(amount), 0)").Row().Scan(&me)
	// SUM(amount) 返回的是「分」，必须用 FromCents 还原成 Money。
	// 早期误用 FromYuan（元→分，×100），导致首页本月收支恒为真实值的 100 倍。
	monthIncome = models.FromCents(mi)
	monthExpense = models.FromCents(me)

	OK(c, gin.H{
		"total_asset":   totalAsset,
		"total_debt":    totalDebt,
		"net_asset":     totalAsset - totalDebt,
		"cash_on_hand":  cashOnHand,
		"by_type":       assetByType,
		"month_income":  monthIncome,
		"month_expense": monthExpense,
		"month_net":     monthIncome - monthExpense,
		"account_count": len(accounts),
	})
}

// GetAssetTimeline 资产负债曲线
func GetAssetTimeline(c *gin.Context) {
	uid := middleware.GetUID(c)
	months := 6
	n, _ := fmt.Sscanf(c.DefaultQuery("months", "6"), "%d", &months)
	if n != 1 || months < 2 {
		months = 6
	}

	now := time.Now()
	type point struct {
		Month      string       `json:"month"`
		TotalAsset models.Money `json:"total_asset"`
		TotalDebt  models.Money `json:"total_debt"`
		NetAsset   models.Money `json:"net_asset"`
	}
	var points []point
	for i := months - 1; i >= 0; i-- {
		d := now.AddDate(0, -i, 0)
		// 先用快照。DATE(snap_date)=? 是 SQLite 专有写法，PostgreSQL 不可用，
		// 统一改为「当月区间内取最新一条」的范围查询，语义更健壮（月初无快照时取当月任意一天）。
		var snap models.AssetSnapshot
		monthStart := time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, d.Location())
		monthEnd := monthStart.AddDate(0, 1, 0)
		database.DB.Where("user_id = ? AND snap_date >= ? AND snap_date < ?", uid, monthStart, monthEnd).
			Order("snap_date DESC").First(&snap)
		if snap.ID > 0 {
			points = append(points, point{
				Month: d.Format("2006-01"),
				TotalAsset: snap.TotalAsset,
				TotalDebt: snap.TotalDebt,
				NetAsset: snap.NetAsset,
			})
			continue
		}
		// 否则用当前数据估算
		var accounts []models.Account
		database.DB.Where("user_id = ? AND is_archived = ?", uid, false).Find(&accounts)
		var a, de models.Money
		for _, ac := range accounts {
			if !ac.IncludeInTotal {
				continue
			}
			switch ac.Type {
			case models.AccLiability, models.AccCredit:
				de += ac.Balance
			default:
				a += ac.Balance
			}
		}
		points = append(points, point{
			Month:      d.Format("2006-01"),
			TotalAsset: a,
			TotalDebt:  de,
			NetAsset:   a - de,
		})
	}

	OK(c, points)
}

// ========== 辅助 ==========
func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

// categoryRankItem 分类排行条目。提升为包级类型，便于排序函数访问 Amount 字段
// （此前 sortByAmountDesc 用泛型却拿不到字段，函数体为空 → 排行顺序随机）。
type categoryRankItem struct {
	ID       uint         `json:"id"`
	Name     string       `json:"name"`
	Icon     string       `json:"icon"`
	Color    string       `json:"color"`
	Kind     string       `json:"kind"`
	Amount   models.Money `json:"amount"`
	Count    int64        `json:"count"`
	Percent  float64      `json:"percent"`
	ParentID uint         `json:"parent_id"`
}

// sortByAmountDesc 按金额降序稳定排序（金额相同时按 ID 升序，保证刷新结果稳定）
func sortByAmountDesc(items []categoryRankItem) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Amount != items[j].Amount {
			return items[i].Amount > items[j].Amount
		}
		return items[i].ID < items[j].ID
	})
}

// dateGrainOf 把统计维度映射到日期分组粒度
func dateGrainOf(dimension string) string {
	switch dimension {
	case "month":
		return "month"
	case "week":
		return "week"
	default:
		return "day"
	}
}

// ========== 存钱计划 ==========
func ListSavingPlans(c *gin.Context) {
	uid := middleware.GetUID(c)
	var list []models.SavingPlan
	applyBookScope(database.DB.Model(&models.SavingPlan{}), uid).Order("status ASC, created_at DESC").Find(&list)
	OK(c, list)
}
func CreateSavingPlan(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.CreateSavingPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil { Bad(c, err.Error()); return }
	s := models.SavingPlan{
		UserID: uid, BookID: req.BookID, AccountID: req.AccountID,
		Name: req.Name, Icon: req.Icon, Color: req.Color,
		TargetAmount: models.FromYuan(req.TargetAmount), CurrentAmount: models.FromYuan(req.CurrentAmount),
		StartDate: req.StartDate.T(), TargetDate: req.TargetDate.T(),
		Status: "active",
	}
	database.DB.Create(&s)
	Broadcast(c, "saving_plans", "create", s.ID)
	Created(c, s)
}
func UpdateSavingPlan(c *gin.Context) {
	uid := middleware.GetUID(c)
	var reqUri dto.IDRequest
	c.ShouldBindUri(&reqUri)
	var s models.SavingPlan
	c.ShouldBindJSON(&s)
	database.DB.Model(&models.SavingPlan{}).Where("id = ? AND user_id = ?", reqUri.ID, uid).Updates(map[string]interface{}{
		"name": s.Name, "icon": s.Icon, "color": s.Color, "target_amount": s.TargetAmount,
		"target_date": s.TargetDate, "status": s.Status,
	})
	var ns models.SavingPlan
	database.DB.First(&ns, reqUri.ID)
	OK(c, ns)
}
func DeleteSavingPlan(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.IDRequest
	c.ShouldBindUri(&req)
	database.DB.Where("id = ? AND user_id = ?", req.ID, uid).Delete(&models.SavingPlan{})
	Broadcast(c, "saving_plans", "delete", req.ID)
	OK(c, nil)
}
func AddSavingRecord(c *gin.Context) {
	uid := middleware.GetUID(c)
	var reqUri dto.IDRequest
	c.ShouldBindUri(&reqUri)
	var req dto.AddSavingRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil { Bad(c, err.Error()); return }
	// B6：存钱真实产生资金流 —— 生成交易、扣减来源账户余额、写回 transaction_id。
	// 旧实现只累加 current_amount，计划进度与真实资产完全脱节。
	db := database.DB.Begin()
	var plan models.SavingPlan
	if err := db.Where("id = ? AND user_id = ?", reqUri.ID, uid).First(&plan).Error; err != nil {
		db.Rollback()
		NotFound(c, "存钱计划不存在")
		return
	}
	amt := models.FromYuan(req.Amount)
	rec := models.SavingRecord{
		UserID: uid, SavingPlanID: reqUri.ID,
		Amount: amt, RecordDate: req.RecordDate.T(),
		TransactionID: req.TransactionID, Note: req.Note,
	}
	if err := db.Create(&rec).Error; err != nil {
		db.Rollback()
		InternalErr(c, "记录失败: "+err.Error())
		return
	}
	db.Model(&models.SavingPlan{}).Where("id = ?", reqUri.ID).
		UpdateColumn("current_amount", gorm.Expr("current_amount + ?", int64(amt)))

	txID := uint(0)
	if req.TransactionID == 0 {
		txID = depositSaving(db, &plan, amt, req.AccountID, req.RecordDate.T())
		if txID > 0 {
			db.Model(&models.SavingRecord{}).Where("id = ?", rec.ID).
				Update("transaction_id", txID)
			rec.TransactionID = txID
		}
	}
	if err := db.Commit().Error; err != nil {
		InternalErr(c, "提交失败: "+err.Error())
		return
	}

	// 达标自动完成
	var updated models.SavingPlan
	database.DB.First(&updated, reqUri.ID)
	if updated.TargetAmount > 0 && updated.CurrentAmount >= updated.TargetAmount && updated.Status == "active" {
		database.DB.Model(&models.SavingPlan{}).Where("id = ?", updated.ID).Update("status", "done")
		updated.Status = "done"
	}
	Created(c, gin.H{"record": rec, "transaction_id": txID, "plan": updated})
}

// ========== 周期记账 ==========
func ListRecurrings(c *gin.Context) {
	uid := middleware.GetUID(c)
	var list []models.Recurring
	applyBookScope(database.DB.Model(&models.Recurring{}), uid).Order("next_run_at ASC").Find(&list)
	OK(c, list)
}
func CreateRecurring(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.CreateRecurringRequest
	if err := c.ShouldBindJSON(&req); err != nil { Bad(c, "参数错误: "+err.Error()); return }
	sd, _ := time.Parse("2006-01-02", req.StartDate)
	var ed time.Time
	if req.EndDate != "" {
		ed, _ = time.Parse("2006-01-02", req.EndDate)
	}
	r := models.Recurring{
		UserID: uid, BookID: req.BookID, Name: req.Name,
		Type: models.TransactionType(req.Type), Amount: models.FromYuan(req.Amount),
		CategoryID: req.CategoryID, AccountID: req.AccountID, ToAccountID: req.ToAccountID,
		Description: req.Description, TagIDs: req.TagIDs,
		RecurringType: models.RecurringType(req.RecurringType),
		Interval: req.Interval, Weekday: req.Weekday, MonthDay: req.MonthDay,
		StartDate: sd, EndDate: ed, MaxTimes: req.MaxTimes,
		Status: "active",
	}
	// 正确计算首次执行时间
	r.NextRunAt = r.ComputeNextRun(sd.AddDate(0, 0, -1))
	database.DB.Create(&r)
	Broadcast(c, "recurring", "create", r.ID)
	Created(c, r)
}

func ToggleRecurring(c *gin.Context) {
	uid := middleware.GetUID(c)
	var reqUri dto.IDRequest
	c.ShouldBindUri(&reqUri)
	var r models.Recurring
	if err := database.DB.Where("id = ? AND user_id = ?", reqUri.ID, uid).First(&r).Error; err != nil {
		Bad(c, "未找到该周期任务");
		return
	}
	if r.Status == "active" {
		database.DB.Model(&r).Update("status", "paused")
	} else {
		// 恢复时重算 next_run_at（避免 next_run_at 已过期导致立即触发）
		now := time.Now()
		nextRun := r.NextRunAt
		if nextRun.IsZero() || nextRun.Before(now) {
			nextRun = r.ComputeNextRun(now)
		}
		database.DB.Model(&r).Updates(map[string]interface{}{
			"status":     "active",
			"next_run_at": nextRun,
		})
	}
	// 重新查询返回最新值
	database.DB.First(&r, reqUri.ID)
	Broadcast(c, "recurring", "update", r.ID)
	OK(c, r)
}
func DeleteRecurring(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.IDRequest
	c.ShouldBindUri(&req)
	database.DB.Where("id = ? AND user_id = ?", req.ID, uid).Delete(&models.Recurring{})
	Broadcast(c, "recurring", "delete", req.ID)
	OK(c, nil)
}

// ========== 分期 ==========
func ListInstallments(c *gin.Context) {
	uid := middleware.GetUID(c)
	var list []models.Installment
	applyBookScope(database.DB.Model(&models.Installment{}), uid).Order("status ASC, next_repay_date ASC").Find(&list)
	OK(c, list)
}
func CreateInstallment(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.CreateInstallmentRequest
	if err := c.ShouldBindJSON(&req); err != nil { Bad(c, err.Error()); return }
	first, _ := time.Parse("2006-01-02", req.FirstRepayDate)
	total := models.FromYuan(req.TotalAmount) + models.FromYuan(req.InterestAmount)
	// 月供 = (总额+利息)/期数，整数分四舍五入
	monthlyAmt := models.Money((int64(total) + int64(req.TotalMonths)/2) / int64(req.TotalMonths))
	ins := models.Installment{
		UserID: uid, BookID: req.BookID, Name: req.Name,
		TotalAmount: models.FromYuan(req.TotalAmount), TotalMonths: req.TotalMonths,
		MonthlyAmount: monthlyAmt, InterestAmount: models.FromYuan(req.InterestAmount),
		CategoryID: req.CategoryID, AccountID: req.AccountID,
		FirstRepayDate: first, NextRepayDate: first,
		Description: req.Description, Status: "active",
	}
	database.DB.Create(&ins)
	Broadcast(c, "installments", "create", ins.ID)
	Created(c, ins)
}
func DeleteInstallment(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.IDRequest
	c.ShouldBindUri(&req)
	database.DB.Where("id = ? AND user_id = ?", req.ID, uid).Delete(&models.Installment{})
	Broadcast(c, "installments", "delete", req.ID)
	OK(c, nil)
}

// ========== 报销 ==========
func ListReimbursements(c *gin.Context) {
	uid := middleware.GetUID(c)
	var list []models.Reimbursement
	applyBookScope(database.DB.Model(&models.Reimbursement{}), uid).Order("created_at DESC").Find(&list)
	OK(c, list)
}
func CreateReimbursement(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.CreateReimbursementRequest
	if err := c.ShouldBindJSON(&req); err != nil { Bad(c, err.Error()); return }
	r := models.Reimbursement{
		UserID: uid, BookID: req.BookID, Name: req.Name,
		TotalAmount: models.FromYuan(req.TotalAmount), Remark: req.Remark,
		TransactionIDs: req.TransactionIDs, Status: "pending",
		SubmittedAt: time.Now(),
	}
	database.DB.Create(&r)
	// 标记对应交易为报销中
	for _, tid := range req.TransactionIDs {
		database.DB.Model(&models.Transaction{}).Where("id = ?", tid).
			Updates(map[string]interface{}{"reimburse_status": "pending"})
	}
	Broadcast(c, "reimbursements", "create", r.ID)
	Created(c, r)
}
func UpdateReimbursement(c *gin.Context) {
	uid := middleware.GetUID(c)
	var reqUri dto.IDRequest
	c.ShouldBindUri(&reqUri)
	var req dto.UpdateReimbursementRequest
	if err := c.ShouldBindJSON(&req); err != nil { Bad(c, err.Error()); return }
	r := database.DB.Model(&models.Reimbursement{}).Where("id = ? AND user_id = ?", reqUri.ID, uid)
	updates := map[string]interface{}{
		"status": req.Status, "received_amount": models.FromYuan(req.ReceivedAmount), "remark": req.Remark,
	}
	now := time.Now()
	if !req.ReceivedDate.IsZero() {
		now = req.ReceivedDate.T()
	}
	if req.Status == "received" {
		updates["received_at"] = now
	}
	r.Updates(updates)

	// 更新相关交易状态 + 生成收款交易（B5）
	var rm models.Reimbursement
	database.DB.First(&rm, reqUri.ID)
	receivedTxID := uint(0)
	if req.Status == "received" || req.Status == "partial" {
		for _, tid := range rm.TransactionIDs {
			database.DB.Model(&models.Transaction{}).Where("id = ?", tid).Update("reimburse_status", "done")
		}
		amount := models.FromYuan(req.ReceivedAmount)
		if amount <= 0 {
			amount = rm.TotalAmount
		}
		db := database.DB.Begin()
		if id, ok := payReimbursement(db, &rm, req.AccountID, amount, now); ok {
			receivedTxID = id
			db.Commit()
		} else {
			db.Rollback()
		}
	}
	Broadcast(c, "reimbursements", "update", reqUri.ID)
	OK(c, gin.H{"id": rm.ID, "status": rm.Status, "received_amount": rm.ReceivedAmount,
		"transaction_id": receivedTxID})
}
func DeleteReimbursement(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.IDRequest
	c.ShouldBindUri(&req)
	database.DB.Where("id = ? AND user_id = ?", req.ID, uid).Delete(&models.Reimbursement{})
	Broadcast(c, "reimbursements", "delete", req.ID)
	OK(c, nil)
}
