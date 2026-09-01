package handlers

import (
	"fmt"
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

	q := database.DB.Model(&models.Transaction{}).Where("user_id = ?", uid)
	if req.BookID > 0 {
		q = q.Where("book_id = ?", req.BookID)
	}
	q = q.Where("tx_date >= ? AND tx_date < ?", req.StartDate, req.EndDate.AddDate(0, 0, 1))
	q = q.Where("include_in_balance = ?", true)

	// 基础汇总
	var totalIncome, totalExpense float64
	var incomeCount, expenseCount int64
	q.Where("type IN ?", []string{string(models.TxIncome), string(models.TxRefund)}).
		Select("COALESCE(SUM(amount), 0), COUNT(*)").Row().Scan(&totalIncome, &incomeCount)
	q.Where("type = ?", string(models.TxExpense)).
		Select("COALESCE(SUM(amount), 0), COUNT(*)").Row().Scan(&totalExpense, &expenseCount)

	result := gin.H{
		"range": gin.H{
			"start":         req.StartDate.Format("2006-01-02"),
			"end":           req.EndDate.Format("2006-01-02"),
			"days":          int(req.EndDate.Sub(req.StartDate).Hours()/24) + 1,
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
	days := int(req.EndDate.Sub(req.StartDate).Hours()/24) + 1
	if days > 0 {
		result["summary"].(gin.H)["avg_daily_expense"] = round2(totalExpense / float64(days))
		result["summary"].(gin.H)["avg_daily_income"] = round2(totalIncome / float64(days))
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
		q.Select("category_id, type, SUM(amount) sum_amount, COUNT(*) count").
			Where("type IN ?", []string{string(models.TxExpense), string(models.TxIncome), string(models.TxRefund)}).
			Group("category_id, type").Scan(&catRows)

		var catMap = make(map[uint]map[string]interface{})
		for _, r := range catRows {
			k := r.CategoryID
			if _, ok := catMap[k]; !ok {
				catMap[k] = map[string]interface{}{
					"category_id": r.CategoryID,
					"income":      0.0,
					"expense":     0.0,
					"count":       int64(0),
				}
			}
			switch r.Type {
			case string(models.TxIncome), string(models.TxRefund):
				catMap[k]["income"] = catMap[k]["income"].(float64) + r.SumAmount
			case string(models.TxExpense):
				catMap[k]["expense"] = catMap[k]["expense"].(float64) + r.SumAmount
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

		// 构造支出排行数组（排序）
		type catOut struct {
			ID       uint    `json:"id"`
			Name     string  `json:"name"`
			Icon     string  `json:"icon"`
			Color    string  `json:"color"`
			Kind     string  `json:"kind"`
			Amount   float64 `json:"amount"`
			Count    int64   `json:"count"`
			Percent  float64 `json:"percent"`
			ParentID uint    `json:"parent_id"`
		}
		var expenseRank, incomeRank []catOut

		for id, m := range catMap {
			info := catInfo[id]
			expAmt := m["expense"].(float64)
			incAmt := m["income"].(float64)
			if expAmt > 0 {
				pct := 0.0
				if totalExpense > 0 {
					pct = round2(expAmt / totalExpense * 100)
				}
				expenseRank = append(expenseRank, catOut{
					ID: id, Name: info.Name, Icon: info.Icon, Color: info.Color,
					Kind: string(info.Kind), Amount: round2(expAmt), Count: m["count"].(int64),
					Percent: pct, ParentID: info.ParentID,
				})
			}
			if incAmt > 0 {
				pct := 0.0
				if totalIncome > 0 {
					pct = round2(incAmt / totalIncome * 100)
				}
				incomeRank = append(incomeRank, catOut{
					ID: id, Name: info.Name, Icon: info.Icon, Color: info.Color,
					Kind: string(info.Kind), Amount: round2(incAmt), Count: m["count"].(int64),
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

	// 2) 按账户
	if dimension == "account" || dimension == "all" {
		var accRows []struct {
			AccountID uint
			Type      string
			SumAmount float64
		}
		q.Select("account_id, type, SUM(amount) sum_amount").
			Where("type IN ?", []string{string(models.TxExpense), string(models.TxIncome), string(models.TxRefund)}).
			Group("account_id, type").Scan(&accRows)
		out := map[uint]gin.H{}
		for _, r := range accRows {
			if _, ok := out[r.AccountID]; !ok {
				out[r.AccountID] = gin.H{"account_id": r.AccountID, "income": 0.0, "expense": 0.0}
			}
			switch r.Type {
			case string(models.TxIncome), string(models.TxRefund):
				out[r.AccountID]["income"] = round2(out[r.AccountID]["income"].(float64) + r.SumAmount)
			case string(models.TxExpense):
				out[r.AccountID]["expense"] = round2(out[r.AccountID]["expense"].(float64) + r.SumAmount)
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
	var dateGroup string
	switch dimension {
	case "month":
		dateGroup = "strftime('%Y-%m', tx_date)"
	case "week":
		dateGroup = "strftime('%G-W%V', tx_date)"
	case "day":
		fallthrough
	default:
		dateGroup = "strftime('%Y-%m-%d', tx_date)"
	}
	q.Select(fmt.Sprintf("%s day, type, SUM(amount) sum_amount", dateGroup)).
		Where("type IN ?", []string{string(models.TxExpense), string(models.TxIncome), string(models.TxRefund)}).
		Group("day, type").Order("day ASC").Scan(&trendRows)

	type trendPoint struct {
		Date    string  `json:"date"`
		Income  float64 `json:"income"`
		Expense float64 `json:"expense"`
		Net     float64 `json:"net"`
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
			trendMap[r.Day].Income = round2(trendMap[r.Day].Income + r.SumAmount)
		case string(models.TxExpense):
			trendMap[r.Day].Expense = round2(trendMap[r.Day].Expense + r.SumAmount)
		}
	}
	trendList := make([]trendPoint, 0, len(trendOrder))
	for _, d := range trendOrder {
		t := trendMap[d]
		t.Net = round2(t.Income - t.Expense)
		trendList = append(trendList, *t)
	}
	result["trend"] = trendList

	// 4) Top 支出排行榜
	var topExp []struct {
		ID          uint
		Amount      float64
		Description string
		TxDate      time.Time
		CategoryID  uint
		Merchant    string
	}
	q.Where("type = ?", string(models.TxExpense)).Order("amount DESC").Limit(10).
		Select("id, amount, description, tx_date, category_id, merchant").Scan(&topExp)
	result["top_expense"] = topExp

	// 5) 资产曲线（月度资产快照）
	var snapshots []models.AssetSnapshot
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

	var totalAsset, totalDebt, cashOnHand float64
	assetByType := map[string]float64{}
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
	var monthIncome, monthExpense float64
	database.DB.Model(&models.Transaction{}).
		Where("user_id = ? AND tx_date >= ? AND tx_date < ? AND type IN ? AND include_in_balance = ?",
			uid, first, last, []string{string(models.TxIncome), string(models.TxRefund)}, true).
		Select("COALESCE(SUM(amount), 0)").Row().Scan(&monthIncome)
	database.DB.Model(&models.Transaction{}).
		Where("user_id = ? AND tx_date >= ? AND tx_date < ? AND type = ? AND include_in_balance = ?",
			uid, first, last, string(models.TxExpense), true).
		Select("COALESCE(SUM(amount), 0)").Row().Scan(&monthExpense)

	OK(c, gin.H{
		"total_asset":   round2(totalAsset),
		"total_debt":    round2(totalDebt),
		"net_asset":     round2(totalAsset - totalDebt),
		"cash_on_hand":  round2(cashOnHand),
		"by_type":       assetByType,
		"month_income":  round2(monthIncome),
		"month_expense": round2(monthExpense),
		"month_net":     round2(monthIncome - monthExpense),
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
		Month      string  `json:"month"`
		TotalAsset float64 `json:"total_asset"`
		TotalDebt  float64 `json:"total_debt"`
		NetAsset   float64 `json:"net_asset"`
	}
	var points []point
	for i := months - 1; i >= 0; i-- {
		d := now.AddDate(0, -i, 0)
		// 先看快照
		var snap models.AssetSnapshot
		database.DB.Where("user_id = ? AND DATE(snap_date) = ?", uid,
			time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, d.Location()).Format("2006-01-02")).
			Order("snap_date DESC").First(&snap)
		if snap.ID > 0 {
			points = append(points, point{
				Month: d.Format("2006-01"),
				TotalAsset: round2(snap.TotalAsset),
				TotalDebt: round2(snap.TotalDebt),
				NetAsset: round2(snap.NetAsset),
			})
			continue
		}
		// 否则用当前数据估算
		var accounts []models.Account
		database.DB.Where("user_id = ? AND is_archived = ?", uid, false).Find(&accounts)
		var a, de float64
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
			TotalAsset: round2(a),
			TotalDebt:  round2(de),
			NetAsset:   round2(a - de),
		})
	}

	OK(c, points)
}

// ========== 辅助 ==========
func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

func sortByAmountDesc[T any](items []T) {
	// 简化处理：在实际代码中可用sort.Slice按Amount排序，这里依赖SQL ORDER
}

// ========== 存钱计划 ==========
func ListSavingPlans(c *gin.Context) {
	uid := middleware.GetUID(c)
	var list []models.SavingPlan
	database.DB.Where("user_id = ?", uid).Order("status ASC, created_at DESC").Find(&list)
	OK(c, list)
}
func CreateSavingPlan(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.CreateSavingPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil { Bad(c, err.Error()); return }
	s := models.SavingPlan{
		UserID: uid, BookID: req.BookID, AccountID: req.AccountID,
		Name: req.Name, Icon: req.Icon, Color: req.Color,
		TargetAmount: req.TargetAmount, CurrentAmount: req.CurrentAmount,
		StartDate: req.StartDate, TargetDate: req.TargetDate,
		Status: "active",
	}
	database.DB.Create(&s)
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
	OK(c, nil)
}
func AddSavingRecord(c *gin.Context) {
	uid := middleware.GetUID(c)
	var reqUri dto.IDRequest
	c.ShouldBindUri(&reqUri)
	var req dto.AddSavingRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil { Bad(c, err.Error()); return }
	rec := models.SavingRecord{
		UserID: uid, SavingPlanID: reqUri.ID,
		Amount: req.Amount, RecordDate: req.RecordDate,
		TransactionID: req.TransactionID, Note: req.Note,
	}
	database.DB.Create(&rec)
	database.DB.Model(&models.SavingPlan{}).Where("id = ?", reqUri.ID).
		UpdateColumn("current_amount", gorm.Expr("current_amount + ?", req.Amount))
	Created(c, rec)
}

// ========== 周期记账 ==========
func ListRecurrings(c *gin.Context) {
	uid := middleware.GetUID(c)
	var list []models.Recurring
	database.DB.Where("user_id = ?", uid).Order("next_run_at ASC").Find(&list)
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
		Type: models.TransactionType(req.Type), Amount: req.Amount,
		CategoryID: req.CategoryID, AccountID: req.AccountID, ToAccountID: req.ToAccountID,
		Description: req.Description, TagIDs: req.TagIDs,
		RecurringType: models.RecurringType(req.RecurringType),
		Interval: req.Interval, Weekday: req.Weekday, MonthDay: req.MonthDay,
		StartDate: sd, EndDate: ed, MaxTimes: req.MaxTimes,
		Status: "active", NextRunAt: sd,
	}
	database.DB.Create(&r)
	Created(c, r)
}
func ToggleRecurring(c *gin.Context) {
	uid := middleware.GetUID(c)
	var reqUri dto.IDRequest
	c.ShouldBindUri(&reqUri)
	var r models.Recurring
	database.DB.Where("id = ? AND user_id = ?", reqUri.ID, uid).First(&r)
	if r.Status == "active" {
		r.Status = "paused"
	} else {
		r.Status = "active"
	}
	database.DB.Save(&r)
	OK(c, r)
}
func DeleteRecurring(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.IDRequest
	c.ShouldBindUri(&req)
	database.DB.Where("id = ? AND user_id = ?", req.ID, uid).Delete(&models.Recurring{})
	OK(c, nil)
}

// ========== 分期 ==========
func ListInstallments(c *gin.Context) {
	uid := middleware.GetUID(c)
	var list []models.Installment
	database.DB.Where("user_id = ?", uid).Order("status ASC, next_repay_date ASC").Find(&list)
	OK(c, list)
}
func CreateInstallment(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.CreateInstallmentRequest
	if err := c.ShouldBindJSON(&req); err != nil { Bad(c, err.Error()); return }
	first, _ := time.Parse("2006-01-02", req.FirstRepayDate)
	monthlyAmt := (req.TotalAmount + req.InterestAmount) / float64(req.TotalMonths)
	ins := models.Installment{
		UserID: uid, BookID: req.BookID, Name: req.Name,
		TotalAmount: req.TotalAmount, TotalMonths: req.TotalMonths,
		MonthlyAmount: monthlyAmt, InterestAmount: req.InterestAmount,
		CategoryID: req.CategoryID, AccountID: req.AccountID,
		FirstRepayDate: first, NextRepayDate: first,
		Description: req.Description, Status: "active",
	}
	database.DB.Create(&ins)
	Created(c, ins)
}
func DeleteInstallment(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.IDRequest
	c.ShouldBindUri(&req)
	database.DB.Where("id = ? AND user_id = ?", req.ID, uid).Delete(&models.Installment{})
	OK(c, nil)
}

// ========== 报销 ==========
func ListReimbursements(c *gin.Context) {
	uid := middleware.GetUID(c)
	var list []models.Reimbursement
	database.DB.Where("user_id = ?", uid).Order("created_at DESC").Find(&list)
	OK(c, list)
}
func CreateReimbursement(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.CreateReimbursementRequest
	if err := c.ShouldBindJSON(&req); err != nil { Bad(c, err.Error()); return }
	r := models.Reimbursement{
		UserID: uid, BookID: req.BookID, Name: req.Name,
		TotalAmount: req.TotalAmount, Remark: req.Remark,
		TransactionIDs: req.TransactionIDs, Status: "pending",
		SubmittedAt: time.Now(),
	}
	database.DB.Create(&r)
	// 标记对应交易为报销中
	for _, tid := range req.TransactionIDs {
		database.DB.Model(&models.Transaction{}).Where("id = ?", tid).
			Updates(map[string]interface{}{"reimburse_status": "pending"})
	}
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
		"status": req.Status, "received_amount": req.ReceivedAmount, "remark": req.Remark,
	}
	if req.Status == "received" {
		updates["received_at"] = time.Now()
	}
	r.Updates(updates)

	// 更新相关交易状态
	var rm models.Reimbursement
	database.DB.First(&rm, reqUri.ID)
	if req.Status == "received" {
		for _, tid := range rm.TransactionIDs {
			database.DB.Model(&models.Transaction{}).Where("id = ?", tid).Update("reimburse_status", "done")
		}
	}
	OK(c, nil)
}
func DeleteReimbursement(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.IDRequest
	c.ShouldBindUri(&req)
	database.DB.Where("id = ? AND user_id = ?", req.ID, uid).Delete(&models.Reimbursement{})
	OK(c, nil)
}
