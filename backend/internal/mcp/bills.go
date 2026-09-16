package mcp

import (
	"context"
	"errors"
	"fmt"
	"huozhi/internal/database"
	"huozhi/internal/dto"
	"huozhi/internal/handlers"
	"huozhi/internal/models"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// 本文件是 MCP 工具的具体实现。所有写操作都直接调用 handlers 包的核心层
// （CreateTx / UpdateTx / DeleteTx / RecoverTx / BatchDeleteTx），
// 与 Web 界面、定时任务共用同一套余额 / 预算 / 派生流水逻辑，
// 不在这里另起炉灶，避免「AI 记的账」和「手工记的账」口径分叉。

var toolHandlers = map[string]ToolHandler{
	"search_transactions":       handleSearchTransactions,
	"get_transaction":           handleGetTransaction,
	"analyze_spending":          handleAnalyzeSpending,
	"get_budget_status":         handleBudgetStatus,
	"list_books":                handleListBooks,
	"list_categories":           handleListCategories,
	"list_accounts":             handleListAccounts,
	"list_tags":                 handleListTags,
	"create_transaction":        handleCreateTransaction,
	"update_transaction":        handleUpdateTransaction,
	"delete_transaction":        handleDeleteTransaction,
	"recover_transaction":       handleRecoverTransaction,
	"batch_delete_transactions": handleBatchDeleteTransactions,
}

// ==================== 输出视图 ====================

// txView 面向模型的精简账单视图：金额统一为「元」，外键换成可读名称。
// 直接把 models.Transaction 整个丢给模型会带上一堆无意义的内部字段
// （exchange_rate、related_type、installment_* 等），白白消耗上下文。
type txView struct {
	ID          uint     `json:"id"`
	Date        string   `json:"date"`
	Type        string   `json:"type"`
	TypeName    string   `json:"type_name"`
	Amount      float64  `json:"amount"`
	Currency    string   `json:"currency,omitempty"`
	Category    string   `json:"category,omitempty"`
	Account     string   `json:"account,omitempty"`
	ToAccount   string   `json:"to_account,omitempty"`
	Merchant    string   `json:"merchant,omitempty"`
	Description string   `json:"description,omitempty"`
	Remark      string   `json:"remark,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	BookID      uint     `json:"book_id,omitempty"`
	BookName    string   `json:"book_name,omitempty"`
}

var typeNames = map[models.TransactionType]string{
	models.TxExpense:   "支出",
	models.TxIncome:    "收入",
	models.TxTransfer:  "转账",
	models.TxRefund:    "退款",
	models.TxReimburse: "报销",
	models.TxAdjust:    "余额调整",
}

// bookNames 批量取账本名，避免逐条查询
func bookNames(uid uint, ids []uint) map[uint]string {
	out := map[uint]string{}
	if len(ids) == 0 {
		return out
	}
	var books []models.Book
	handlers.ScopeByBooks(database.DB.Model(&models.Book{}), uid, ids).
		Select("id", "name").Where("id IN ?", uniqueIDs(ids)).Find(&books)
	for _, b := range books {
		out[b.ID] = b.Name
	}
	return out
}

func uniqueIDs(in []uint) []uint {
	seen := map[uint]struct{}{}
	out := make([]uint, 0, len(in))
	for _, id := range in {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func toView(t models.Transaction, books map[uint]string) txView {
	v := txView{
		ID:          t.ID,
		Date:        t.TxDate.Format("2006-01-02"),
		Type:        string(t.Type),
		TypeName:    typeNames[t.Type],
		Amount:      t.Amount.Yuan(),
		Currency:    t.Currency,
		Category:    t.CategoryName,
		Account:     t.AccountName,
		ToAccount:   t.ToAccountName,
		Merchant:    t.Merchant,
		Description: t.Description,
		Remark:      t.Remark,
		BookID:      t.BookID,
		BookName:    books[t.BookID],
	}
	if len(t.Tags) > 0 {
		names := make([]string, 0, len(t.Tags))
		for _, tag := range t.Tags {
			if tag != nil {
				names = append(names, tag.Name)
			}
		}
		v.Tags = names
	}
	return v
}

func viewsOf(list []models.Transaction) []txView {
	ids := make([]uint, 0, len(list))
	for _, t := range list {
		ids = append(ids, t.BookID)
	}
	var uid uint
	if len(list) > 0 {
		uid = list[0].UserID
	}
	books := bookNames(uid, ids)
	out := make([]txView, 0, len(list))
	for _, t := range list {
		out = append(out, toView(t, books))
	}
	return out
}

// ==================== 时间范围 ====================

var periodWords = map[string]string{
	"today":         "今天",
	"yesterday":     "昨天",
	"this_week":     "本周",
	"last_week":     "上周",
	"this_month":    "本月",
	"last_month":    "上月",
	"this_quarter":  "本季度",
	"last_quarter":  "上季度",
	"this_year":     "今年",
	"last_year":     "去年",
	"recent_7d":     "最近7天",
	"recent_30d":    "最近30天",
	"recent_90d":    "最近90天",
	"recent_7days":  "最近7天",
	"recent_30days": "最近30天",
	"recent_90days": "最近90天",
}

func periodToRange(p string, now time.Time) (start, end time.Time, ok bool) {
	p = strings.ToLower(strings.TrimSpace(p))
	if p == "" {
		return time.Time{}, time.Time{}, false
	}
	if word, exists := periodWords[p]; exists {
		return ParseRange(word, now)
	}
	return ParseRange(p, now)
}

// resolveTimeRange 解析工具入参里的时间条件。
// 优先级：start_date/end_date > period > 无（返回 ok=false，表示不限时间）。
func resolveTimeRange(args map[string]any, now time.Time) (start, end *time.Time, err error) {
	startStr := argString(args, "start_date")
	endStr := argString(args, "end_date")
	period := argString(args, "period")

	if startStr != "" {
		if s, e, ok := ParseRange(startStr, now); ok {
			start = &s
			// 区间词（"上月"）直接给出完整区间；单词（"昨天"）只作为起点
			if !s.Equal(e) {
				end = &e
			}
		} else {
			return nil, nil, toolErrorf("无法识别的起始日期「%s」。可用写法：今天 / 昨天 / 上周 / 上月 / 最近30天 / 2026-01-01。", startStr)
		}
	}
	if endStr != "" {
		if s, e, ok := ParseRange(endStr, now); ok {
			if e.After(s) {
				end = &e
			} else {
				end = &e
			}
		} else {
			return nil, nil, toolErrorf("无法识别的结束日期「%s」。可用写法：今天 / 昨天 / 上周 / 上月 / 最近30天 / 2026-01-31。", endStr)
		}
	}
	if start == nil && end == nil && period != "" {
		if s, e, ok := periodToRange(period, now); ok {
			start, end = &s, &e
		} else {
			return nil, nil, toolErrorf("无法识别的时间范围「%s」。可用值：%s。", period, "today / yesterday / this_week / last_week / this_month / last_month / this_year / last_year / recent_7d / recent_30d / recent_90d")
		}
	}
	if start != nil && end != nil && end.Before(*start) {
		return nil, nil, toolErrorf("结束日期（%s）早于起始日期（%s）", end.Format("2006-01-02"), start.Format("2006-01-02"))
	}
	return start, end, nil
}

// resolveAnalyzeRange 分析专用：默认本月 1 号 ~ 今天
func resolveAnalyzeRange(args map[string]any, now time.Time) (time.Time, time.Time, error) {
	start, end, err := resolveTimeRange(args, now)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if start == nil && end == nil {
		s, e, _ := ParseRange("本月", now)
		return s, e, nil
	}
	s, e := time.Time{}, time.Now()
	if start != nil {
		s = *start
	} else {
		s = time.Date(1970, 1, 1, 0, 0, 0, 0, now.Location())
	}
	if end != nil {
		e = *end
	}
	return s, e, nil
}

// ==================== 查询 ====================

func handleSearchTransactions(ctx context.Context, uid uint, args map[string]any) (any, error) {
	now := time.Now()
	start, end, err := resolveTimeRange(args, now)
	if err != nil {
		return nil, err
	}
	r := newResolver(uid)

	req := dto.QueryTransactionRequest{Type: strings.TrimSpace(argString(args, "type"))}
	if start != nil {
		req.StartDate = dto.FlexDate{Time: *start}
	}
	if end != nil {
		req.EndDate = dto.FlexDate{Time: *end}
	}

	if v := argString(args, "category"); v != "" {
		cands, err2 := r.Categories(v, "")
		if err2 != nil {
			return nil, err2
		}
		if len(cands) == 0 {
			return nil, toolErrorf("未找到分类「%s」。可用分类：%s", v, nameList(mustCandidates(r.Categories("", "")), 30))
		}
		req.CategoryIDs = IDsOf(cands)
	}
	if v := argString(args, "account"); v != "" {
		cands, err2 := r.Accounts(v)
		if err2 != nil {
			return nil, err2
		}
		if len(cands) == 0 {
			return nil, toolErrorf("未找到账户「%s」。可用账户：%s", v, nameList(mustCandidates(r.Accounts("")), 30))
		}
		req.AccountIDs = IDsOf(cands)
	}
	if v := argString(args, "book"); v != "" {
		cands, err2 := r.Books(v)
		if err2 != nil {
			return nil, err2
		}
		if len(cands) == 0 {
			return nil, toolErrorf("未找到账本「%s」。可用账本：%s", v, nameList(mustCandidates(r.Books("")), 20))
		}
		req.BookIDs = IDsOf(cands)
	}
	if v := argString(args, "tag"); v != "" {
		cands, err2 := r.Tags(v)
		if err2 != nil {
			return nil, err2
		}
		if len(cands) == 0 {
			return nil, toolErrorf("未找到标签「%s」", v)
		}
		req.TagIDs = IDsOf(cands)
	}
	req.Keyword = argString(args, "keyword")
	req.ReimburseStatus = argString(args, "reimburse_status")
	if v, ok := argFloat(args, "min_amount"); ok {
		req.MinAmount = v
	}
	if v, ok := argFloat(args, "max_amount"); ok {
		req.MaxAmount = v
	}
	if v, ok := argBool(args, "include_deleted"); ok {
		req.IncludeDeleted = v
	}

	page := 1
	if v, ok := argInt(args, "page"); ok && v > 0 {
		page = v
	}
	limit := 20
	if v, ok := argInt(args, "limit"); ok && v > 0 {
		limit = v
	}
	if limit > 100 {
		limit = 100
	}
	req.Page, req.PageSize = page, limit

	res, cerr := handlers.ListTx(uid, req)
	if cerr != nil {
		return nil, coreErr(cerr)
	}

	flat := make([]models.Transaction, 0, limit)
	for _, g := range res.Grouped {
		flat = append(flat, g.Transactions...)
	}

	out := map[string]any{
		"total":        res.Total,
		"page":         res.Page,
		"page_size":    res.PageSize,
		"has_more":     int64(res.Page*res.PageSize) < res.Total,
		"transactions": viewsOf(flat),
		"summary":      res.Summary,
	}
	rangeInfo := map[string]any{}
	if start != nil {
		rangeInfo["start"] = start.Format("2006-01-02")
	}
	if end != nil {
		rangeInfo["end"] = end.Format("2006-01-02")
	}
	if len(rangeInfo) > 0 {
		out["range"] = rangeInfo
	}
	if len(flat) == 0 {
		out["hint"] = "没有匹配的账单。可放宽时间范围、去掉分类/账户条件，或先调用 list_categories / list_accounts 确认名称。"
	}
	return out, nil
}

func handleGetTransaction(ctx context.Context, uid uint, args map[string]any) (any, error) {
	id, ok := argUint(args, "id")
	if !ok {
		return nil, errors.New("缺少参数 id")
	}
	tx, cerr := handlers.GetTx(uid, id)
	if cerr != nil {
		return nil, coreErr(cerr)
	}
	return toView(tx, bookNames(uid, []uint{tx.BookID})), nil
}

// ==================== 分析 ====================

type breakdownItem struct {
	ID      uint    `json:"id,omitempty"`
	Name    string  `json:"name"`
	Amount  float64 `json:"amount"`
	Percent float64 `json:"percent"`
	Count   int     `json:"count"`
	Average float64 `json:"average"`
}

type trendItem struct {
	Label   string  `json:"label"`
	Income  float64 `json:"income"`
	Expense float64 `json:"expense"`
	Net     float64 `json:"net"`
	Count   int     `json:"count"`
}

// aggRow 分析用的最小行：只取聚合真正需要的列，避免把整行 JSON 字段拉出来
type aggRow struct {
	ID           uint         `gorm:"column:id"`
	Type         string       `gorm:"column:type"`
	Amount       models.Money `gorm:"column:amount"`
	ExchangeRate float64      `gorm:"column:exchange_rate"`
	TxDate       time.Time    `gorm:"column:tx_date"`
	CategoryID   uint         `gorm:"column:category_id"`
	AccountID    uint         `gorm:"column:account_id"`
	Merchant     string       `gorm:"column:merchant"`
	BookID       uint         `gorm:"column:book_id"`
	Description  string       `gorm:"column:description"`
}

func baseAmountOf(amount models.Money, rate float64) models.Money {
	if rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
		return amount
	}
	if rate == 1 {
		return amount
	}
	return models.Money(math.Round(float64(amount) * rate))
}

func handleAnalyzeSpending(ctx context.Context, uid uint, args map[string]any) (any, error) {
	now := time.Now()
	start, end, err := resolveAnalyzeRange(args, now)
	if err != nil {
		return nil, err
	}
	if end.Before(start) {
		start, end = end, start
	}
	r := newResolver(uid)

	kind := strings.TrimSpace(strings.ToLower(argString(args, "kind")))
	if kind == "" {
		kind = "expense"
	}
	if kind != "expense" && kind != "income" && kind != "all" {
		return nil, toolErrorf("kind 只支持 expense / income / all，收到「%s」", kind)
	}
	dimension := strings.TrimSpace(strings.ToLower(argString(args, "dimension")))
	if dimension == "" {
		dimension = "category"
	}
	topN := 10
	if v, ok := argInt(args, "top_n"); ok && v > 0 {
		topN = v
	}
	if topN > 50 {
		topN = 50
	}
	compare := true
	if v, ok := argBool(args, "compare_previous"); ok {
		compare = v
	}

	ids := r.BookIDs
	q := handlers.ScopeByBooks(database.DB.Model(&models.Transaction{}), uid, ids).
		Where("tx_date >= ? AND tx_date < ?", start, end.AddDate(0, 0, 1)).
		Where("include_in_balance = ?", true)
	if v := argString(args, "book"); v != "" {
		cands, err2 := r.Books(v)
		if err2 != nil {
			return nil, err2
		}
		if len(cands) == 0 {
			return nil, toolErrorf("未找到账本「%s」", v)
		}
		q = q.Where("book_id IN ?", IDsOf(cands))
	}
	if v := argString(args, "category"); v != "" {
		cands, err2 := r.Categories(v, "")
		if err2 != nil {
			return nil, err2
		}
		if len(cands) == 0 {
			return nil, toolErrorf("未找到分类「%s」", v)
		}
		q = q.Where("category_id IN ?", IDsOf(cands))
	}
	if v := argString(args, "account"); v != "" {
		cands, err2 := r.Accounts(v)
		if err2 != nil {
			return nil, err2
		}
		if len(cands) == 0 {
			return nil, toolErrorf("未找到账户「%s」", v)
		}
		accIDs := IDsOf(cands)
		q = q.Where("account_id IN ? OR to_account_id IN ?", accIDs, accIDs)
	}

	var rows []aggRow
	if err := q.Select("id, type, amount, exchange_rate, tx_date, category_id, account_id, merchant, book_id, description").
		Order("tx_date ASC").Find(&rows).Error; err != nil {
		return nil, toolErrorf("查询失败: %v", err)
	}

	// ---- 汇总 ----
	var income, expense models.Money
	var incomeCount, expenseCount int
	for i := range rows {
		t := models.TransactionType(rows[i].Type)
		amt := baseAmountOf(rows[i].Amount, rows[i].ExchangeRate)
		b := models.TxStatsBucket(t)
		if b == models.StatsBucketIncome {
			income += amt
			incomeCount++
		} else if b == models.StatsBucketExpense {
			expense += amt
			expenseCount++
		} else {
			continue
		}
	}

	// ---- 维度拆分 ----
	agg := map[string]*bucket{}
	for i := range rows {
		t := models.TransactionType(rows[i].Type)
		b := models.TxStatsBucket(t)
		if b == "" {
			continue
		}
		if kind == "expense" && b != models.StatsBucketExpense {
			continue
		}
		if kind == "income" && b != models.StatsBucketIncome {
			continue
		}
		k := groupKey(dimension, rows[i], now)
		if agg[k] == nil {
			agg[k] = &bucket{}
		}
		agg[k].amount += baseAmountOf(rows[i].Amount, rows[i].ExchangeRate)
		agg[k].count++
	}
	// 标签维度需要额外取关联
	if dimension == "tag" {
		txIDs := make([]uint, 0, len(rows))
		for i := range rows {
			txIDs = append(txIDs, rows[i].ID)
		}
		agg = map[string]*bucket{}
		links := tagLinksOf(txIDs)
		tagNames := tagNamesOf(uid, ids)
		for i := range rows {
			t := models.TransactionType(rows[i].Type)
			b := models.TxStatsBucket(t)
			if b == "" {
				continue
			}
			if kind == "expense" && b != models.StatsBucketExpense {
				continue
			}
			if kind == "income" && b != models.StatsBucketIncome {
				continue
			}
			amtt := baseAmountOf(rows[i].Amount, rows[i].ExchangeRate)
			tagIDs := links[rows[i].ID]
			if len(tagIDs) == 0 {
				k := "未打标签"
				if agg[k] == nil {
					agg[k] = &bucket{}
				}
				agg[k].amount += amtt
				agg[k].count++
				continue
			}
			for _, tid := range tagIDs {
				name := tagNames[tid]
				if name == "" {
					name = fmt.Sprintf("标签#%d", tid)
				}
				if agg[name] == nil {
					agg[name] = &bucket{}
				}
				agg[name].amount += amtt
				agg[name].count++
			}
		}
	}

	total := income
	if kind == "expense" {
		total = expense
	} else if kind == "all" {
		total = income + expense
	}
	keyNames := resolveBreakdownNames(uid, r.BookIDs, dimension, agg)
	breakdown := make([]breakdownItem, 0, len(agg))
	for key, b := range agg {
		name := keyNames[key]
		if name == "" {
			name = key
		}
		pct := 0.0
		if total > 0 {
			pct = math.Round(b.amount.Yuan()/total.Yuan()*1000) / 10
		}
		avg := 0.0
		if b.count > 0 {
			avg = math.Round(b.amount.Yuan()/float64(b.count)*100) / 100
		}
		breakdown = append(breakdown, breakdownItem{
			Name: name, Amount: b.amount.Yuan(), Percent: pct, Count: b.count, Average: avg,
		})
	}
	sort.Slice(breakdown, func(i, j int) bool { return breakdown[i].Amount > breakdown[j].Amount })
	if len(breakdown) > topN {
		breakdown = breakdown[:topN]
	}

	// ---- 趋势 ----
	grain := "month"
	days := int(end.Sub(start).Hours()/24) + 1
	switch {
	case dimension == "day":
		grain = "day"
	case dimension == "week":
		grain = "week"
	case dimension == "month":
		grain = "month"
	case days <= 31:
		grain = "day"
	case days <= 120:
		grain = "week"
	}
	trendMap := map[string]*trendItem{}
	var trendOrder []string
	for i := range rows {
		label := trendLabel(grain, rows[i].TxDate)
		if trendMap[label] == nil {
			trendMap[label] = &trendItem{Label: label}
			trendOrder = append(trendOrder, label)
		}
		item := trendMap[label]
		amt := baseAmountOf(rows[i].Amount, rows[i].ExchangeRate)
		switch models.TxStatsBucket(models.TransactionType(rows[i].Type)) {
		case models.StatsBucketIncome:
			item.Income += amt.Yuan()
		case models.StatsBucketExpense:
			item.Expense += amt.Yuan()
		default:
			continue
		}
		item.Count++
	}
	sort.Strings(trendOrder)
	trend := make([]trendItem, 0, len(trendOrder))
	for _, label := range trendOrder {
		it := trendMap[label]
		it.Income = math.Round(it.Income*100) / 100
		it.Expense = math.Round(it.Expense*100) / 100
		it.Net = math.Round((it.Income-it.Expense)*100) / 100
		trend = append(trend, *it)
	}

	// ---- Top 大额 ----
	sort.Slice(rows, func(i, j int) bool {
		return baseAmountOf(rows[i].Amount, rows[i].ExchangeRate) > baseAmountOf(rows[j].Amount, rows[j].ExchangeRate)
	})
	top := make([]txView, 0, topN)
	books := bookNames(uid, func() []uint {
		out := make([]uint, 0, len(rows))
		for i := range rows {
			out = append(out, rows[i].BookID)
		}
		return out
	}())
	for i := range rows {
		if len(top) >= topN {
			break
		}
		b := models.TxStatsBucket(models.TransactionType(rows[i].Type))
		if b == "" {
			continue
		}
		if kind == "expense" && b != models.StatsBucketExpense {
			continue
		}
		if kind == "income" && b != models.StatsBucketIncome {
			continue
		}
		top = append(top, txView{
			ID: rows[i].ID, Date: rows[i].TxDate.Format("2006-01-02"),
			Type: rows[i].Type, TypeName: typeNames[models.TransactionType(rows[i].Type)],
			Amount:      baseAmountOf(rows[i].Amount, rows[i].ExchangeRate).Yuan(),
			Description: rows[i].Description, Merchant: rows[i].Merchant,
			BookID: rows[i].BookID, BookName: books[rows[i].BookID],
		})
	}

	out := map[string]any{
		"range": map[string]any{
			"start": start.Format("2006-01-02"),
			"end":   end.Format("2006-01-02"),
			"days":  days,
		},
		"kind":      kind,
		"dimension": dimension,
		"summary": map[string]any{
			"income":            income.Yuan(),
			"expense":           expense.Yuan(),
			"net":               (income - expense).Yuan(),
			"income_count":      incomeCount,
			"expense_count":     expenseCount,
			"transaction_count": incomeCount + expenseCount,
			"avg_daily_expense": round2(expense.Yuan() / float64(days)),
			"avg_daily_income":  round2(income.Yuan() / float64(days)),
		},
		"breakdown":        breakdown,
		"trend":            trend,
		"trend_grain":      grain,
		"top_transactions": top,
	}
	if dimension == "tag" {
		out["note"] = "标签维度下，一笔账单若打了多个标签会分别计入各标签，因此各项之和可能大于总额。"
	}

	// ---- 环比 ----
	if compare && days > 0 {
		prevStart := start.AddDate(0, 0, -days)
		prevEnd := start.AddDate(0, 0, -1)
		var prevRows []aggRow
		pq := handlers.ScopeByBooks(database.DB.Model(&models.Transaction{}), uid, ids).
			Where("tx_date >= ? AND tx_date < ?", prevStart, prevEnd.AddDate(0, 0, 1)).
			Where("include_in_balance = ?", true)
		if err := pq.Select("type, amount, exchange_rate").Find(&prevRows).Error; err == nil {
			var pIn, pEx models.Money
			for i := range prevRows {
				amt := baseAmountOf(prevRows[i].Amount, prevRows[i].ExchangeRate)
				switch models.TxStatsBucket(models.TransactionType(prevRows[i].Type)) {
				case models.StatsBucketIncome:
					pIn += amt
				case models.StatsBucketExpense:
					pEx += amt
				}
			}
			cur := expense
			prev := pEx
			if kind == "income" {
				cur, prev = income, pIn
			} else if kind == "all" {
				cur, prev = income+expense, pIn+pEx
			}
			delta := cur - prev
			pct := 0.0
			if prev > 0 {
				pct = math.Round(delta.Yuan()/prev.Yuan()*1000) / 10
			}
			out["comparison"] = map[string]any{
				"previous_range":  map[string]any{"start": prevStart.Format("2006-01-02"), "end": prevEnd.Format("2006-01-02")},
				"previous_amount": prev.Yuan(),
				"current_amount":  cur.Yuan(),
				"delta":           delta.Yuan(),
				"delta_percent":   pct,
				"direction":       directionOf(delta),
			}
		}
	}

	if len(rows) == 0 {
		out["hint"] = "该时间范围内没有符合条件的账单。"
	}
	return out, nil
}

func directionOf(d models.Money) string {
	switch {
	case d > 0:
		return "up"
	case d < 0:
		return "down"
	}
	return "flat"
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

func groupKey(dimension string, row aggRow, now time.Time) string {
	switch dimension {
	case "account":
		return fmt.Sprintf("acct:%d", row.AccountID)
	case "merchant":
		if strings.TrimSpace(row.Merchant) == "" {
			return "未填写商家"
		}
		return row.Merchant
	case "day":
		return row.TxDate.Format("2006-01-02")
	case "week":
		return weekStartOf(row.TxDate).Format("2006-01-02")
	case "month":
		return row.TxDate.Format("2006-01")
	default:
		return fmt.Sprintf("cat:%d", row.CategoryID)
	}
}

// bucket 某个维度下的聚合结果
type bucket struct {
	amount models.Money
	count  int
}

// resolveBreakdownNames 把聚合用的内部 key（cat:12 / acct:7）换成人能读懂的名字。
// 直接把 "cat:12" 丢给模型，它既无法朗读也无法据此继续追问。
func resolveBreakdownNames(uid uint, bookIDs []uint, dimension string, agg map[string]*bucket) map[string]string {
	out := make(map[string]string, len(agg))
	prefix := ""
	var numericIDs []uint
	switch dimension {
	case "category":
		prefix = "cat:"
	case "account":
		prefix = "acct:"
	default:
		for k := range agg {
			out[k] = k
		}
		return out
	}
	for k := range agg {
		id, err := strconv.ParseUint(strings.TrimPrefix(k, prefix), 10, 64)
		if err != nil {
			continue
		}
		numericIDs = append(numericIDs, uint(id))
	}
	names := map[uint]string{}
	if dimension == "category" {
		names = categoryNamesOf(uid, bookIDs, numericIDs)
	} else {
		var list []models.Account
		if len(numericIDs) > 0 {
			database.DB.Model(&models.Account{}).Select("id", "name").
				Where("id IN ?", uniqueIDs(numericIDs)).Find(&list)
			for _, a := range list {
				names[a.ID] = a.Name
			}
		}
	}
	for k := range agg {
		id, err := strconv.ParseUint(strings.TrimPrefix(k, prefix), 10, 64)
		if err != nil {
			out[k] = k
			continue
		}
		if n, ok := names[uint(id)]; ok && n != "" {
			out[k] = n
			continue
		}
		out[k] = "未分类"
		if dimension == "account" {
			out[k] = "未知账户"
		}
	}
	return out
}

func weekStartOf(t time.Time) time.Time {
	d := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	offset := int(d.Weekday()+6) % 7
	return d.AddDate(0, 0, -offset)
}

func trendLabel(grain string, t time.Time) string {
	switch grain {
	case "week":
		return weekStartOf(t).Format("2006-01-02")
	case "month":
		return t.Format("2006-01")
	}
	return t.Format("2006-01-02")
}

func tagLinksOf(txIDs []uint) map[uint][]uint {
	out := map[uint][]uint{}
	if len(txIDs) == 0 {
		return out
	}
	var links []models.TransactionTag
	database.DB.Where("transaction_id IN ?", uniqueIDs(txIDs)).Find(&links)
	for _, l := range links {
		out[l.TransactionID] = append(out[l.TransactionID], l.TagID)
	}
	return out
}

func tagNamesOf(uid uint, bookIDs []uint) map[uint]string {
	out := map[uint]string{}
	var tags []models.Tag
	handlers.ScopeByBooks(database.DB.Model(&models.Tag{}), uid, bookIDs).Select("id", "name").Find(&tags)
	for _, t := range tags {
		out[t.ID] = t.Name
	}
	return out
}

// ==================== 预算 ====================

func handleBudgetStatus(ctx context.Context, uid uint, args map[string]any) (any, error) {
	r := newResolver(uid)
	q := handlers.ScopeByBooks(database.DB.Model(&models.Budget{}), uid, r.BookIDs)
	if v := argString(args, "book"); v != "" {
		cands, err := r.Books(v)
		if err != nil {
			return nil, err
		}
		if len(cands) == 0 {
			return nil, toolErrorf("未找到账本「%s」", v)
		}
		q = q.Where("book_id IN ?", IDsOf(cands))
	}
	var budgets []models.Budget
	q.Order("start_date DESC").Find(&budgets)

	onlyAlert := false
	if v, ok := argBool(args, "only_over_alert"); ok {
		onlyAlert = v
	}
	books := bookNames(uid, func() []uint {
		out := make([]uint, 0, len(budgets))
		for _, b := range budgets {
			out = append(out, b.BookID)
		}
		return out
	}())
	catNames := categoryNamesOf(uid, r.BookIDs, func() []uint {
		out := make([]uint, 0, len(budgets))
		for _, b := range budgets {
			if b.CategoryID > 0 {
				out = append(out, b.CategoryID)
			}
		}
		return out
	}())

	type budgetView struct {
		ID         uint    `json:"id"`
		Name       string  `json:"name"`
		BookName   string  `json:"book_name,omitempty"`
		PeriodType string  `json:"period_type"`
		StartDate  string  `json:"start_date"`
		EndDate    string  `json:"end_date"`
		Amount     float64 `json:"amount"`
		Used       float64 `json:"used"`
		Remaining  float64 `json:"remaining"`
		UsageRate  float64 `json:"usage_rate"`
		AlertRate  float64 `json:"alert_rate"`
		OverBudget bool    `json:"over_budget"`
	}
	out := make([]budgetView, 0, len(budgets))
	for _, b := range budgets {
		rate := 0.0
		if b.Amount > 0 {
			rate = math.Round(b.UsedAmount.Yuan()/b.Amount.Yuan()*1000) / 10
		}
		alert := b.AlertRate
		if alert <= 0 {
			alert = 0.8
		}
		if onlyAlert && rate < alert*100 {
			continue
		}
		name := "总预算"
		if b.CategoryID > 0 {
			name = catNames[b.CategoryID]
			if name == "" {
				name = fmt.Sprintf("分类#%d", b.CategoryID)
			}
		}
		out = append(out, budgetView{
			ID: b.ID, Name: name, BookName: books[b.BookID],
			PeriodType: b.PeriodType,
			StartDate:  b.StartDate.Format("2006-01-02"),
			EndDate:    b.EndDate.Format("2006-01-02"),
			Amount:     b.Amount.Yuan(),
			Used:       b.UsedAmount.Yuan(),
			Remaining:  (b.Amount - b.UsedAmount).Yuan(),
			UsageRate:  rate,
			AlertRate:  round2(alert * 100),
			OverBudget: b.UsedAmount > b.Amount,
		})
	}
	if len(out) == 0 {
		return map[string]any{"budgets": out, "hint": "当前没有预算记录。"}, nil
	}
	return map[string]any{"budgets": out}, nil
}

func categoryNamesOf(uid uint, bookIDs []uint, ids []uint) map[uint]string {
	out := map[uint]string{}
	if len(ids) == 0 {
		return out
	}
	var cats []models.Category
	database.DB.Model(&models.Category{}).Select("id", "name").
		Where("id IN ?", uniqueIDs(ids)).Find(&cats)
	for _, c := range cats {
		out[c.ID] = c.Name
	}
	return out
}

// ==================== 字典 ====================

func handleListBooks(ctx context.Context, uid uint, args map[string]any) (any, error) {
	r := newResolver(uid)
	var list []models.Book
	handlers.ScopeByBooks(database.DB.Model(&models.Book{}), uid, r.BookIDs).
		Where("is_archived = ?", false).
		Order("is_default DESC, sort ASC, id ASC").Find(&list)
	type bookView struct {
		ID        uint   `json:"id"`
		Name      string `json:"name"`
		Currency  string `json:"currency"`
		IsDefault bool   `json:"is_default"`
		IsShared  bool   `json:"is_shared"`
	}
	out := make([]bookView, 0, len(list))
	for _, b := range list {
		shared := false
		for _, id := range r.BookIDs {
			if id == b.ID {
				shared = true
				break
			}
		}
		out = append(out, bookView{ID: b.ID, Name: b.Name, Currency: b.Currency, IsDefault: b.IsDefault, IsShared: shared})
	}
	return map[string]any{"books": out}, nil
}

func handleListCategories(ctx context.Context, uid uint, args map[string]any) (any, error) {
	r := newResolver(uid)
	kind := argString(args, "kind")
	keyword := argString(args, "keyword")
	cands, err := r.Categories(keyword, kind)
	if err != nil {
		return nil, err
	}
	// 重新取完整对象（candidate 只有 id/name）
	q := database.DB.Model(&models.Category{}).
		Where("(user_id = ? OR book_id IN ?)", uid, append(append([]uint{}, r.BookIDs...), 0)).
		Where("is_hidden = ?", false)
	if kind != "" {
		q = q.Where("kind = ?", kind)
	}
	var list []models.Category
	q.Order("kind ASC, sort ASC, id ASC").Find(&list)

	type catView struct {
		ID       uint   `json:"id"`
		Name     string `json:"name"`
		Kind     string `json:"kind"`
		ParentID uint   `json:"parent_id,omitempty"`
		BookID   uint   `json:"book_id,omitempty"`
	}
	matched := map[uint]struct{}{}
	if keyword != "" {
		for _, c := range cands {
			matched[c.ID] = struct{}{}
		}
	}
	flat := true
	if v, ok := argBool(args, "flatten"); ok {
		flat = v
	}
	out := make([]catView, 0, len(list))
	for _, c := range list {
		if keyword != "" {
			if _, ok := matched[c.ID]; !ok {
				continue
			}
		}
		out = append(out, catView{ID: c.ID, Name: c.Name, Kind: string(c.Kind), ParentID: c.ParentID, BookID: c.BookID})
	}
	if !flat {
		// 两级树
		type treeItem struct {
			catView
			Children []catView `json:"children,omitempty"`
		}
		roots := make([]treeItem, 0)
		children := map[uint][]catView{}
		for _, c := range out {
			if c.ParentID == 0 {
				roots = append(roots, treeItem{catView: c})
				continue
			}
			children[c.ParentID] = append(children[c.ParentID], c)
		}
		for i := range roots {
			roots[i].Children = children[roots[i].ID]
		}
		return map[string]any{"categories": roots}, nil
	}
	return map[string]any{"categories": out}, nil
}

func handleListAccounts(ctx context.Context, uid uint, args map[string]any) (any, error) {
	r := newResolver(uid)
	q := handlers.ScopeByBooks(database.DB.Model(&models.Account{}), uid, r.BookIDs)
	if v := argString(args, "type"); v != "" {
		q = q.Where("type = ?", v)
	}
	if v, ok := argBool(args, "include_archived"); !ok || !v {
		q = q.Where("is_archived = ?", false)
	}
	if v := argString(args, "book"); v != "" {
		cands, err := r.Books(v)
		if err != nil {
			return nil, err
		}
		if len(cands) == 0 {
			return nil, toolErrorf("未找到账本「%s」", v)
		}
		ids := IDsOf(cands)
		q = q.Where("book_id IN ? OR book_id = 0", ids)
	}
	var list []models.Account
	q.Order("sort ASC, id DESC").Find(&list)
	type accView struct {
		ID       uint    `json:"id"`
		Name     string  `json:"name"`
		Type     string  `json:"type"`
		Balance  float64 `json:"balance"`
		Currency string  `json:"currency"`
		BookID   uint    `json:"book_id,omitempty"`
	}
	out := make([]accView, 0, len(list))
	for _, a := range list {
		out = append(out, accView{ID: a.ID, Name: a.Name, Type: string(a.Type),
			Balance: a.Balance.Yuan(), Currency: a.Currency, BookID: a.BookID})
	}
	return map[string]any{"accounts": out}, nil
}

func handleListTags(ctx context.Context, uid uint, args map[string]any) (any, error) {
	r := newResolver(uid)
	q := handlers.ScopeByBooks(database.DB.Model(&models.Tag{}), uid, r.BookIDs)
	if v := argString(args, "book"); v != "" {
		cands, err := r.Books(v)
		if err != nil {
			return nil, err
		}
		if len(cands) == 0 {
			return nil, toolErrorf("未找到账本「%s」", v)
		}
		ids := IDsOf(cands)
		q = q.Where("book_id IN ? OR book_id = 0", ids)
	}
	var list []models.Tag
	q.Order("count DESC, id ASC").Find(&list)
	type tagView struct {
		ID    uint   `json:"id"`
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	out := make([]tagView, 0, len(list))
	for _, t := range list {
		out = append(out, tagView{ID: t.ID, Name: t.Name, Count: t.Count})
	}
	return map[string]any{"tags": out}, nil
}

// ==================== 写入 ====================

func handleCreateTransaction(ctx context.Context, uid uint, args map[string]any) (any, error) {
	amount, ok := argFloat(args, "amount")
	if !ok || amount <= 0 {
		return nil, errors.New("缺少参数 amount（金额，单位元，必须大于 0）")
	}
	txType := strings.TrimSpace(strings.ToLower(argString(args, "type")))
	if txType == "" {
		txType = "expense"
	}
	switch models.TransactionType(txType) {
	case models.TxExpense, models.TxIncome, models.TxTransfer, models.TxRefund, models.TxReimburse, models.TxAdjust:
	default:
		return nil, toolErrorf("非法的交易类型「%s」，可选：expense / income / transfer / refund / reimburse / adjust", txType)
	}

	r := newResolver(uid)
	book, err := r.DefaultBook(argString(args, "book"))
	if err != nil {
		return nil, err
	}

	// 账户：显式指定 > 唯一账户 > 报错并列出候选
	accountID := uint(0)
	if v := argString(args, "account"); v != "" {
		accountID, err = r.RequireAccount(v)
		if err != nil {
			return nil, err
		}
	} else {
		cands, err2 := r.Accounts("")
		if err2 != nil {
			return nil, err2
		}
		if len(cands) == 1 {
			accountID = cands[0].ID
		} else if len(cands) == 0 {
			return nil, errors.New("当前账本还没有任何账户，请先在应用里创建一个账户")
		} else {
			return nil, toolErrorf("请选择账户（account）。可用账户：%s", nameList(cands, 20))
		}
	}

	// 分类：显式指定 > 从备注里猜 > 「其他」> 报错
	kind := kindOfType(txType)
	categoryID := uint(0)
	if v := argString(args, "category"); v != "" {
		categoryID, err = r.RequireCategory(v, kind)
		if err != nil {
			return nil, err
		}
	} else {
		desc := argString(args, "description")
		categoryID = guessCategory(r, kind, desc)
		if categoryID == 0 && (txType == "transfer" || txType == "adjust") {
			// 转账 / 余额调整的分类是后端预置的隐藏分类（如「转账」），
			// 用户一般不传，这里直接取（没有就自动建），避免每次都报「请指定分类」。
			if txType == "transfer" {
				categoryID = handlers.EnsureSystemCategory(uid, book.ID, "转账", models.KindSystem, "🔄")
			} else {
				categoryID = handlers.EnsureSystemCategory(uid, book.ID, "余额调整", models.KindSystem, "⚙️")
			}
		}
		if categoryID == 0 {
			cands, err2 := r.Categories("", kind)
			if err2 != nil {
				return nil, err2
			}
			return nil, toolErrorf("请指定分类（category）。可用分类：%s", nameList(cands, 30))
		}
	}

	toAccountID := uint(0)
	if v := argString(args, "to_account"); v != "" {
		toAccountID, err = r.RequireAccount(v)
		if err != nil {
			return nil, err
		}
	}
	if txType == "transfer" && toAccountID == 0 {
		return nil, errors.New("转账（type=transfer）必须指定目标账户 to_account")
	}

	txDate := time.Now()
	if v := argString(args, "date"); v != "" {
		d, ok2 := ParseDate(v, time.Now())
		if !ok2 {
			return nil, toolErrorf("无法识别的日期「%s」。可用写法：今天 / 昨天 / 前天 / 上月5号 / 2026-01-31。", v)
		}
		// 记账日期保留当前时刻的时分秒，避免与「刚刚记的账」排序错乱
		txDate = d
	}

	req := dto.CreateTransactionRequest{
		BookID:      book.ID,
		Type:        txType,
		Amount:      amount,
		CategoryID:  categoryID,
		AccountID:   accountID,
		ToAccountID: toAccountID,
		TxDate:      dto.FlexDate{Time: txDate},
		Description: argString(args, "description"),
		Merchant:    argString(args, "merchant"),
		Location:    argString(args, "location"),
		Remark:      argString(args, "remark"),
		Currency:    argString(args, "currency"),
	}
	if v, ok := argFloat(args, "exchange_rate"); ok {
		req.ExchangeRate = v
	}
	if v, ok := argFloat(args, "transfer_fee"); ok {
		req.TransferFee = v
	}
	if names := argStringSlice(args, "tags"); len(names) > 0 {
		req.TagIDs = resolveTagIDs(r, names)
	}

	dry := false
	if v, ok := argBool(args, "dry_run"); ok {
		dry = v
	}
	if dry {
		preview := models.Transaction{
			BookID: req.BookID, Type: models.TransactionType(txType), Amount: models.FromYuan(amount),
			CategoryID: categoryID, AccountID: accountID, ToAccountID: toAccountID,
			TxDate: txDate, Description: req.Description, Merchant: req.Merchant,
			Location: req.Location, Remark: req.Remark, Currency: req.Currency,
		}
		list := []models.Transaction{preview}
		handlers.FillTxViewFields(list)
		return map[string]any{
			"dry_run": true,
			"applied": false,
			"book":    book.Name,
			"tag_ids": req.TagIDs,
			"preview": toView(list[0], bookNames(uid, []uint{book.ID})),
			"hint":    "这是一次预演，数据未写入。确认无误后去掉 dry_run 再调用一次即可真正记账。",
		}, nil
	}

	tx, cerr := handlers.CreateTx(uid, req)
	if cerr != nil {
		return nil, coreErr(cerr)
	}
	return map[string]any{
		"applied":     true,
		"message":     fmt.Sprintf("已记账：%s %.2f 元（%s）", typeNames[models.TransactionType(txType)], amount, tx.TxDate.Format("2006-01-02")),
		"transaction": toView(tx, bookNames(uid, []uint{tx.BookID})),
		"recoverable": true,
	}, nil
}

func handleUpdateTransaction(ctx context.Context, uid uint, args map[string]any) (any, error) {
	id, ok := argUint(args, "id")
	if !ok {
		return nil, errors.New("缺少参数 id")
	}
	old, cerr := handlers.GetTx(uid, id)
	if cerr != nil {
		return nil, coreErr(cerr)
	}

	r := newResolver(uid)
	req := dto.UpdateTransactionRequest{}
	changed := map[string]any{}

	if v, ok := argFloat(args, "amount"); ok {
		if v <= 0 {
			return nil, errors.New("amount 必须大于 0")
		}
		req.Amount = &v
		changed["amount"] = v
	}
	if v := argString(args, "type"); v != "" {
		switch models.TransactionType(v) {
		case models.TxExpense, models.TxIncome, models.TxTransfer, models.TxRefund, models.TxReimburse, models.TxAdjust:
			req.Type = &v
			changed["type"] = v
		default:
			return nil, toolErrorf("非法的交易类型「%s」", v)
		}
	}
	newType := string(old.Type)
	if req.Type != nil {
		newType = *req.Type
	}
	if v := argString(args, "category"); v != "" {
		cid, err := r.RequireCategory(v, "")
		if err != nil {
			return nil, err
		}
		req.CategoryID = &cid
		changed["category_id"] = cid
	}
	if v := argString(args, "account"); v != "" {
		aid, err := r.RequireAccount(v)
		if err != nil {
			return nil, err
		}
		req.AccountID = &aid
		changed["account_id"] = aid
	}
	if v := argString(args, "to_account"); v != "" {
		aid, err := r.RequireAccount(v)
		if err != nil {
			return nil, err
		}
		req.ToAccountID = &aid
		changed["to_account_id"] = aid
	}
	if v := argString(args, "book"); v != "" {
		book, err := r.DefaultBook(v)
		if err != nil {
			return nil, err
		}
		bid := book.ID
		req.BookID = &bid
		changed["book_id"] = bid
	}
	if v := argString(args, "date"); v != "" {
		d, ok2 := ParseDate(v, time.Now())
		if !ok2 {
			return nil, toolErrorf("无法识别的日期「%s」", v)
		}
		fd := dto.FlexDate{Time: d}
		req.TxDate = &fd
		changed["tx_date"] = d.Format("2006-01-02")
	}
	if v := argString(args, "description"); v != "" {
		req.Description = &v
		changed["description"] = v
	}
	if v := argString(args, "merchant"); v != "" {
		req.Merchant = &v
		changed["merchant"] = v
	}
	if v := argString(args, "location"); v != "" {
		req.Location = &v
		changed["location"] = v
	}
	if v := argString(args, "remark"); v != "" {
		req.Remark = &v
		changed["remark"] = v
	}
	if v, ok := argFloat(args, "transfer_fee"); ok {
		req.TransferFee = &v
		changed["transfer_fee"] = v
	}
	if names := argStringSlice(args, "tags"); len(names) > 0 {
		tagIDs := resolveTagIDs(r, names)
		req.TagIDs = &tagIDs
		changed["tags"] = names
	}
	if len(changed) == 0 {
		return nil, errors.New("没有提供任何要修改的字段。请至少传入 amount / category / account / date / description 等之一。")
	}
	if newType == "transfer" && old.ToAccountID == 0 && req.ToAccountID == nil {
		return nil, errors.New("把账单改成转账类型时必须指定 to_account")
	}

	dry := false
	if v, ok := argBool(args, "dry_run"); ok {
		dry = v
	}
	if dry {
		preview := old
		if req.Amount != nil {
			preview.Amount = models.FromYuan(*req.Amount)
		}
		if req.Type != nil {
			preview.Type = models.TransactionType(*req.Type)
		}
		if req.CategoryID != nil {
			preview.CategoryID = *req.CategoryID
		}
		if req.AccountID != nil {
			preview.AccountID = *req.AccountID
		}
		if req.TxDate != nil {
			preview.TxDate = req.TxDate.T()
		}
		if req.Description != nil {
			preview.Description = *req.Description
		}
		list := []models.Transaction{preview}
		handlers.FillTxViewFields(list)
		before := []models.Transaction{old}
		handlers.FillTxViewFields(before)
		books := bookNames(uid, []uint{old.BookID, preview.BookID})
		return map[string]any{
			"dry_run": true,
			"applied": false,
			"before":  toView(before[0], books),
			"after":   toView(list[0], books),
			"changed": changed,
			"hint":    "这是一次预演，数据未写入。确认无误后去掉 dry_run 再调用一次即可真正修改。",
		}, nil
	}

	tx, cerr := handlers.UpdateTx(uid, id, req)
	if cerr != nil {
		return nil, coreErr(cerr)
	}
	before := []models.Transaction{old}
	handlers.FillTxViewFields(before)
	books := bookNames(uid, []uint{old.BookID, tx.BookID})
	return map[string]any{
		"applied":     true,
		"message":     fmt.Sprintf("已更新账单 #%d", id),
		"changed":     changed,
		"before":      toView(before[0], books),
		"transaction": toView(tx, books),
	}, nil
}

func handleDeleteTransaction(ctx context.Context, uid uint, args map[string]any) (any, error) {
	id, ok := argUint(args, "id")
	if !ok {
		return nil, errors.New("缺少参数 id")
	}
	tx, cerr := handlers.GetTx(uid, id)
	if cerr != nil {
		return nil, coreErr(cerr)
	}
	dry := false
	if v, ok := argBool(args, "dry_run"); ok {
		dry = v
	}
	books := bookNames(uid, []uint{tx.BookID})
	if dry {
		return map[string]any{
			"dry_run": true,
			"applied": false,
			"preview": toView(tx, books),
			"impact": map[string]any{
				"account_balance_change": balanceImpactOf(tx),
				"budget_used_change":     budgetImpactOf(tx),
			},
			"hint": "这是一次预演，未删除。确认后去掉 dry_run 再调用一次；删除是软删除，可用 recover_transaction 恢复。",
		}, nil
	}
	if cerr := handlers.DeleteTx(uid, id); cerr != nil {
		return nil, coreErr(cerr)
	}
	return map[string]any{
		"applied": true,
		"message": fmt.Sprintf("已删除账单 #%d（软删除，可用 recover_transaction 恢复）", id),
		"deleted": toView(tx, books),
		"recover": map[string]any{"tool": "recover_transaction", "arguments": map[string]any{"id": id}},
	}, nil
}

func handleRecoverTransaction(ctx context.Context, uid uint, args map[string]any) (any, error) {
	id, ok := argUint(args, "id")
	if !ok {
		return nil, errors.New("缺少参数 id")
	}
	tx, cerr := handlers.RecoverTx(uid, id)
	if cerr != nil {
		return nil, coreErr(cerr)
	}
	return map[string]any{
		"applied":     true,
		"message":     fmt.Sprintf("已恢复账单 #%d", id),
		"transaction": toView(tx, bookNames(uid, []uint{tx.BookID})),
	}, nil
}

func handleBatchDeleteTransactions(ctx context.Context, uid uint, args map[string]any) (any, error) {
	rawIDs, ok := args["ids"]
	if !ok {
		return nil, errors.New("缺少参数 ids（账单 id 数组）")
	}
	list, ok := rawIDs.([]any)
	if !ok {
		return nil, errors.New("ids 必须是数组，例如 [1, 2, 3]")
	}
	ids := make([]uint, 0, len(list))
	for _, item := range list {
		switch v := item.(type) {
		case float64:
			if v > 0 {
				ids = append(ids, uint(v))
			}
		case string:
			if n, err := parseFloatID(v); err == nil && n > 0 {
				ids = append(ids, uint(n))
			}
		}
	}
	if len(ids) == 0 {
		return nil, errors.New("ids 为空或格式不正确")
	}

	// 先取出受影响账单，供预演与结果回执使用
	found := make([]models.Transaction, 0, len(ids))
	for _, id := range ids {
		if tx, cerr := handlers.GetTx(uid, id); cerr == nil {
			found = append(found, tx)
		}
	}
	totalAmount := models.Money(0)
	for _, tx := range found {
		totalAmount += tx.AmountInBase()
	}
	handlers.FillTxViewFields(found)

	dry := false
	if v, ok := argBool(args, "dry_run"); ok {
		dry = v
	}
	if dry {
		return map[string]any{
			"dry_run":      true,
			"applied":      false,
			"count":        len(found),
			"total_amount": totalAmount.Yuan(),
			"transactions": viewsOf(found),
			"hint":         "这是一次预演，未删除。确认后请取得用户明确同意，再带 confirm=true 调用。",
		}, nil
	}
	confirmed := false
	if v, ok := argBool(args, "confirm"); ok {
		confirmed = v
	}
	if !confirmed {
		return nil, toolErrorf("批量删除影响 %d 笔账单（合计 %.2f 元）。请先用 dry_run=true 预演，向用户展示清单并取得明确同意后，再带 confirm=true 调用。",
			len(found), totalAmount.Yuan())
	}

	n, cerr := handlers.BatchDeleteTx(uid, ids)
	if cerr != nil {
		return nil, coreErr(cerr)
	}
	return map[string]any{
		"applied":       true,
		"deleted_count": n,
		"not_found":     len(ids) - len(found),
		"message":       fmt.Sprintf("已删除 %d 笔账单（软删除，可用 recover_transaction 按 id 逐个恢复）", n),
	}, nil
}

// ==================== 辅助 ====================

func parseFloatID(s string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(s), 64)
}

func coreErr(e *handlers.CoreError) error {
	if e == nil {
		return nil
	}
	return errors.New(e.Message)
}

func kindOfType(t string) string {
	switch models.TransactionType(t) {
	case models.TxIncome, models.TxRefund:
		return string(models.KindIncome)
	case models.TxTransfer, models.TxAdjust:
		return string(models.KindSystem)
	}
	return string(models.KindExpense)
}

// guessCategory 未显式指定分类时的兜底：优先用备注里出现的分类名（"打车" 命中"交通" 不行，
// 但 "买了水果" 命中"水果" 可以），其次退回同名「其他」分类。
func guessCategory(r *Resolver, kind, description string) uint {
	desc := strings.ToLower(strings.TrimSpace(description))
	if desc != "" {
		cands, err := r.Categories("", kind)
		if err == nil {
			for _, c := range cands {
				name := strings.TrimSpace(c.Name)
				if len(name) >= 2 && strings.Contains(desc, strings.ToLower(name)) {
					return c.ID
				}
			}
		}
	}
	cands, err := r.Categories("其他", kind)
	if err == nil && len(cands) > 0 {
		return cands[0].ID
	}
	return 0
}

func resolveTagIDs(r *Resolver, names []string) []uint {
	out := make([]uint, 0, len(names))
	for _, n := range names {
		cands, err := r.Tags(n)
		if err != nil || len(cands) == 0 {
			continue
		}
		// 精确优先
		picked := cands[0].ID
		for _, c := range cands {
			if strings.EqualFold(strings.TrimSpace(c.Name), strings.TrimSpace(n)) {
				picked = c.ID
				break
			}
		}
		out = append(out, picked)
	}
	return out
}

// balanceImpactOf 删除/恢复某笔账单对账户余额的影响（元，含符号）
func balanceImpactOf(t models.Transaction) float64 {
	dir := models.TxBalanceDirection(t.Type)
	if dir == 0 {
		return 0
	}
	return -float64(dir) * t.AmountInBase().Yuan()
}

// budgetImpactOf 删除/恢复某笔账单对预算占用的影响（元，含符号）
func budgetImpactOf(t models.Transaction) float64 {
	if models.TxStatsBucket(t.Type) != models.StatsBucketExpense || !t.IncludeInBudget {
		return 0
	}
	return -t.AmountInBase().Yuan()
}

func mustCandidates(cs []candidate, err error) []candidate {
	if err != nil {
		return nil
	}
	return cs
}
