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

// baseAmountExpr 把原币金额按汇率折算到基准币种后再聚合的 SQL 片段。
//
// 修复背景（原 B-02）：此前统计页各处一律用 SUM(amount)，汇率从未参与运算，
// 一笔 100 USD / 汇率 7.18 的支出在统计里只算 1.00 元。
// 这里与 models.Transaction.AmountInBase() 保持同一口径：汇率 <= 0 视为 1。
const baseAmountExpr = "CASE WHEN exchange_rate > 0 THEN amount * exchange_rate ELSE amount END"

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
		// 读范围与流水列表一致（本人 或 可见共享账本），消除两处口径差异（原 B-05）
		q := applyBookScope(c, database.DB.Model(&models.Transaction{}), uid)
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
	base().Where("type IN ?", models.TypesInBucket(models.StatsBucketIncome)).
		Select(fmt.Sprintf("COALESCE(SUM(%s), 0), COUNT(*)", baseAmountExpr)).
		Row().Scan(&incSum, &incomeCount)
	// 支出口径必须包含 reimburse：它真实扣减了账户余额，
	// 此前被排除在统计外，导致「流水明细加总 ≠ 统计页支出」（原 B-04）。
	base().Where("type IN ?", models.TypesInBucket(models.StatsBucketExpense)).
		Select(fmt.Sprintf("COALESCE(SUM(%s), 0), COUNT(*)", baseAmountExpr)).
		Row().Scan(&expSum, &expenseCount)
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
		base().Select(fmt.Sprintf("category_id, type, SUM(%s) sum_amount, COUNT(*) count", baseAmountExpr)).
			Where("type IN ?", []string{string(models.TxExpense), string(models.TxReimburse), string(models.TxIncome), string(models.TxRefund)}).
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
			case string(models.TxExpense), string(models.TxReimburse):
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
		base().Select(fmt.Sprintf("book_id, type, SUM(%s) sum_amount", baseAmountExpr)).
			Where("type IN ?", []string{string(models.TxExpense), string(models.TxReimburse), string(models.TxIncome), string(models.TxRefund)}).
			Group("book_id, type").Scan(&bookRows)
		out := map[uint]gin.H{}
		for _, r := range bookRows {
			if _, ok := out[r.BookID]; !ok {
				out[r.BookID] = gin.H{"book_id": r.BookID, "income": models.Money(0), "expense": models.Money(0)}
			}
			switch r.Type {
			case string(models.TxIncome), string(models.TxRefund):
				out[r.BookID]["income"] = out[r.BookID]["income"].(models.Money) + models.FromCents(r.SumAmount)
			case string(models.TxExpense), string(models.TxReimburse):
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
		base().Select(fmt.Sprintf("account_id, type, SUM(%s) sum_amount", baseAmountExpr)).
			Where("type IN ?", []string{string(models.TxExpense), string(models.TxReimburse), string(models.TxIncome), string(models.TxRefund)}).
			Group("account_id, type").Scan(&accRows)
		out := map[uint]gin.H{}
		for _, r := range accRows {
			if _, ok := out[r.AccountID]; !ok {
				out[r.AccountID] = gin.H{"account_id": r.AccountID, "income": models.Money(0), "expense": models.Money(0)}
			}
			switch r.Type {
			case string(models.TxIncome), string(models.TxRefund):
				out[r.AccountID]["income"] = out[r.AccountID]["income"].(models.Money) + models.FromCents(r.SumAmount)
			case string(models.TxExpense), string(models.TxReimburse):
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
	base().Select(fmt.Sprintf("%s day, type, SUM(%s) sum_amount", dateGroup, baseAmountExpr)).
		Where("type IN ?", []string{string(models.TxExpense), string(models.TxReimburse), string(models.TxIncome), string(models.TxRefund)}).
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
		case string(models.TxExpense), string(models.TxReimburse):
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
			uid, first, last, models.TypesInBucket(models.StatsBucketIncome), true).
		Select(fmt.Sprintf("COALESCE(SUM(%s), 0)", baseAmountExpr)).Row().Scan(&mi)
	// 支出口径含 reimburse（原 B-04）
	database.DB.Model(&models.Transaction{}).
		Where("user_id = ? AND tx_date >= ? AND tx_date < ? AND type IN ? AND include_in_balance = ?",
			uid, first, last, models.TypesInBucket(models.StatsBucketExpense), true).
		Select(fmt.Sprintf("COALESCE(SUM(%s), 0)", baseAmountExpr)).Row().Scan(&me)
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
	applyBookScope(c, database.DB.Model(&models.SavingPlan{}), uid).Order("status ASC, created_at DESC").Find(&list)
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
	applyBookScope(c, database.DB.Model(&models.Recurring{}), uid).Order("next_run_at ASC").Find(&list)
	OK(c, list)
}
func CreateRecurring(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.CreateRecurringRequest
	if err := c.ShouldBindJSON(&req); err != nil { Bad(c, "参数错误: "+err.Error()); return }
	// 用本地时区解析：time.Parse 会把「2026-09-25」解析成 UTC 零时，
	// 后续 09:00 的执行锚点也就变成了 UTC 09:00（东八区实际是下午 5 点），
	// 且西半球用户会整体偏移到前一天。
	sd, err := time.ParseInLocation("2006-01-02", req.StartDate, time.Local)
	if err != nil {
		Bad(c, "开始日期格式不正确，应为 YYYY-MM-DD")
		return
	}
	var ed time.Time
	if req.EndDate != "" {
		ed, _ = time.ParseInLocation("2006-01-02", req.EndDate, time.Local)
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
	// 首次执行时间：不得早于 start_date。
	// 此前用 ComputeNextRun(start_date - 1天) 兜底，导致 monthly 整月跳过起始月、
	// yearly 被推到次年且偏一天、带间隔的 daily/biweekly 偏 N-1 天。
	r.NextRunAt = r.ComputeFirstRun()
	// 设置了结束时间且首期已晚于结束时间 → 直接置为暂停，避免产生越界流水
	if !ed.IsZero() && r.NextRunAt.After(ed) {
		r.Status = "paused"
	}
	if r.MaxTimes > 0 && r.RunCount >= r.MaxTimes {
		r.Status = "paused"
	}
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
	applyBookScope(c, database.DB.Model(&models.Installment{}), uid).Order("status ASC, next_repay_date ASC").Find(&list)
	OK(c, list)
}
func CreateInstallment(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.CreateInstallmentRequest
	if err := c.ShouldBindJSON(&req); err != nil { Bad(c, err.Error()); return }
	// 首次还款日必填且必须是合法日期：解析失败会拿到零值时间，
	// 让 RunInstallmentRepayments 立刻判定「已到期」并生成一条日期为 0001-01-01 的还款流水。
	first, err := time.ParseInLocation("2006-01-02", req.FirstRepayDate, time.Local)
	if err != nil {
		Bad(c, "首次还款日格式不正确，应为 YYYY-MM-DD")
		return
	}
	total := models.FromYuan(req.TotalAmount) + models.FromYuan(req.InterestAmount)
	// 月供：用户显式指定时以其为准；否则按 (总额+利息)/期数 均摊，整数分四舍五入。
	monthlyAmt := models.FromYuan(req.MonthlyAmount)
	if monthlyAmt <= 0 || req.TotalMonths <= 0 {
		if req.TotalMonths > 0 {
			monthlyAmt = models.Money((int64(total) + int64(req.TotalMonths)/2) / int64(req.TotalMonths))
		}
	}
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

// normalizeIDs 把 nil 的 ID 切片归一化为空数组。
//
// 背景：TransactionIDs 用 serializer:json 存库，nil 切片会写成 SQL NULL，
// 读回来仍是 nil，JSON 序列化成 `null` 而不是 `[]`。前端直接取 `.length`
// 就是 TypeError，报销列表整页白屏。存量数据同样需要兜底，因此在列表出口再归一一次。
func normalizeIDs(ids []uint) []uint {
	if ids == nil {
		return []uint{}
	}
	return ids
}

func ListReimbursements(c *gin.Context) {
	uid := middleware.GetUID(c)
	var list []models.Reimbursement
	applyBookScope(c, database.DB.Model(&models.Reimbursement{}), uid).Order("created_at DESC").Find(&list)
	// 存量数据的 transaction_ids 可能是 NULL，出口统一兜底为空数组
	for i := range list {
		list[i].TransactionIDs = normalizeIDs(list[i].TransactionIDs)
	}
	OK(c, list)
}
func CreateReimbursement(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.CreateReimbursementRequest
	if err := c.ShouldBindJSON(&req); err != nil { Bad(c, err.Error()); return }
	r := models.Reimbursement{
		UserID: uid, BookID: req.BookID, Name: req.Name,
		TotalAmount: models.FromYuan(req.TotalAmount), Remark: req.Remark,
		// 必须初始化为空数组而不是 nil：nil 切片经 serializer:json 落库为 NULL，
		// 序列化回前端是 `transaction_ids: null`，列表页 `item.transaction_ids.length`
		// 直接抛 TypeError，整页白屏。
		TransactionIDs: normalizeIDs(req.TransactionIDs), Status: "pending",
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

	// 先读原单：更新前的总额是「已收齐」金额兜底与幂等入账的依据，
	// 也避免把不存在的 id 当成成功处理。
	var old models.Reimbursement
	if err := database.DB.Where("id = ? AND user_id = ?", reqUri.ID, uid).
		First(&old).Error; err != nil {
		NotFound(c, "报销单不存在")
		return
	}

	received := models.FromYuan(req.ReceivedAmount)
	// 「已收齐」但没填已收金额 → 按总额兜底。否则列表会显示「¥0 / ¥1000 · 已收齐」，
	// 与真实入账金额自相矛盾。
	if req.Status == "received" && received <= 0 {
		received = old.TotalAmount
	}
	updates := map[string]interface{}{
		"status": req.Status, "received_amount": received, "remark": req.Remark,
	}
	now := time.Now()
	if !req.ReceivedDate.IsZero() {
		now = req.ReceivedDate.T()
	}
	if req.Status == "received" {
		updates["received_at"] = now
	}
	if err := database.DB.Model(&models.Reimbursement{}).
		Where("id = ? AND user_id = ?", reqUri.ID, uid).Updates(updates).Error; err != nil {
		InternalErr(c, "更新失败: "+err.Error())
		return
	}

	// 更新相关交易状态 + 生成收款交易（B5）
	var rm models.Reimbursement
	database.DB.First(&rm, reqUri.ID)
	rm.TransactionIDs = normalizeIDs(rm.TransactionIDs)
	receivedTxID := uint(0)
	if req.Status == "received" || req.Status == "partial" {
		for _, tid := range rm.TransactionIDs {
			database.DB.Model(&models.Transaction{}).Where("id = ?", tid).Update("reimburse_status", "done")
		}
		amount := received
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
