package handlers

import (
	"encoding/csv"
	"fmt"
	"huozhi/internal/database"
	"huozhi/internal/dto"
	"huozhi/internal/middleware"
	"huozhi/internal/models"
	"io"
	"mime/multipart"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// ========== 导入导出 ==========

// ExportTransactions 导出CSV
func ExportTransactions(c *gin.Context) {
	uid := middleware.GetUID(c)
	bookID := c.Query("book_id")
	start := c.Query("start_date")
	end := c.Query("end_date")

	q := database.DB.Preload("Tags").Where("user_id = ?", uid)
	if bookID != "" {
		q = q.Where("book_id = ?", bookID)
	}
	if start != "" {
		q = q.Where("tx_date >= ?", start)
	}
	if end != "" {
		q = q.Where("tx_date < ?", end)
	}
	var txs []models.Transaction
	q.Order("tx_date ASC, id ASC").Find(&txs)

	// 写入CSV缓存
	c.Writer.Header().Set("Content-Type", "text/csv; charset=utf-8")
	c.Writer.Header().Set("Content-Disposition", "attachment; filename=huozhi_transactions_"+time.Now().Format("20060102")+".csv")
	c.Writer.Write([]byte{0xEF, 0xBB, 0xBF}) // UTF-8 BOM

	w := csv.NewWriter(c.Writer)
	w.Write([]string{"交易日期", "类型", "分类", "金额", "货币", "账户", "目标账户", "描述", "商家", "标签", "备注", "报销状态"})

	// 映射ID→名称缓存
	catCache := accountNameCache()
	accCache := accountNameCache()

	for _, t := range txs {
		tagNames := make([]string, 0, len(t.Tags))
		for _, tg := range t.Tags {
			tagNames = append(tagNames, tg.Name)
		}
		toAccStr := ""
		if t.ToAccountID > 0 {
			toAccStr = accCache(t.ToAccountID)
		}
		w.Write([]string{
			t.TxDate.Format("2006-01-02 15:04:05"),
			string(t.Type),
			catCache(t.CategoryID),
			strconv.FormatFloat(t.Amount, 'f', 2, 64),
			t.Currency,
			accCache(t.AccountID),
			toAccStr,
			t.Description,
			t.Merchant,
			strings.Join(tagNames, ";"),
			t.Remark,
			t.ReimburseStatus,
		})
	}
	w.Flush()
}

// 简单缓存
func accountNameCache() func(uint) string {
	type m struct {
		ID   uint
		Name string
	}
	var accounts []m
	database.DB.Model(&models.Account{}).Select("id, name").Scan(&accounts)
	var cats []m
	database.DB.Model(&models.Category{}).Select("id, name").Scan(&cats)
	cache := make(map[uint]string)
	for _, a := range accounts {
		cache[a.ID] = a.Name
	}
	for _, cc := range cats {
		cache[cc.ID] = cc.Name
	}
	return func(id uint) string {
		if id == 0 {
			return ""
		}
		return cache[id]
	}
}

// ImportTransactions 导入（模板CSV/微信/支付宝 通用简化版）
func ImportTransactions(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.ImportRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		Bad(c, err.Error())
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		Bad(c, "缺少文件: "+err.Error())
		return
	}
	defer file.Close()

	parser := pickParser(req.Source, header)
	records, err := parser(file, uid, req.BookID)
	if err != nil {
		InternalErr(c, "解析失败: "+err.Error())
		return
	}

	// 入库
	created, skipped := 0, 0
	db := database.DB.Begin()
	for _, rec := range records {
		rec.UserID = uid
		rec.BookID = req.BookID
		if rec.AccountID == 0 || rec.CategoryID == 0 {
			skipped++
			continue
		}
		if err := db.Create(&rec).Error; err == nil {
			created++
			// 更新余额
			var from, to models.Account
			db.First(&from, rec.AccountID)
			if rec.ToAccountID > 0 {
				db.First(&to, rec.ToAccountID)
			}
			updateAccountBalances(db, &rec, &from, &to, true)
		} else {
			skipped++
		}
	}
	db.Commit()

	OK(c, gin.H{
		"created": created,
		"skipped": skipped,
		"total":   len(records),
		"source":  req.Source,
	})
}

// pickParser 选择解析器
func pickParser(source string, header *multipart.FileHeader) func(io.Reader, uint, uint) ([]models.Transaction, error) {
	switch source {
	case "wechat":
		return parseWeChat
	case "alipay":
		return parseAlipay
	case "template":
		fallthrough
	default:
		return parseTemplateCSV
	}
}

// parseTemplateCSV 解析模板CSV
func parseTemplateCSV(f io.Reader, uid, bookID uint) ([]models.Transaction, error) {
	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("空文件")
	}

	// 默认分类/账户映射
	expenseCat := findCategoryOrCreate(uid, bookID, "其他支出", models.KindExpense, "📦")
	incomeCat := findCategoryOrCreate(uid, bookID, "其他收入", models.KindIncome, "💰")
	acc := findAccountOrFirst(uid, bookID)

	var out []models.Transaction
	for i := 1; i < len(records); i++ {
		row := records[i]
		if len(row) < 4 {
			continue
		}
		dateStr := row[0]
		ttype := strings.ToLower(strings.TrimSpace(row[1]))
		amountStr := strings.TrimSpace(row[2])
		catName := strings.TrimSpace(row[3])
		desc := ""
		if len(row) > 4 {
			desc = row[4]
		}
		amt, _ := strconv.ParseFloat(amountStr, 64)
		if amt <= 0 {
			continue
		}
		d, e := time.Parse("2006-01-02", dateStr)
		if e != nil {
			d, _ = time.Parse("2006/1/2", dateStr)
		}
		if d.IsZero() {
			d = time.Now()
		}

		tx := models.Transaction{
			Amount:      amt,
			Currency:    "CNY",
			AccountID:   acc.ID,
			TxDate:      d,
			Description: desc,
		}
		switch ttype {
		case "expense", "支出", "outcome":
			tx.Type = models.TxExpense
			tx.CategoryID = matchCategory(uid, bookID, catName, models.KindExpense, expenseCat)
		case "income", "收入":
			tx.Type = models.TxIncome
			tx.CategoryID = matchCategory(uid, bookID, catName, models.KindIncome, incomeCat)
		default:
			if amt > 0 {
				tx.Type = models.TxExpense
				tx.CategoryID = expenseCat.ID
			}
		}
		tx.IncludeInBalance = true
		tx.IncludeInBudget = true
		out = append(out, tx)
	}
	return out, nil
}

// parseWeChat 简化版微信账单解析（正式版需处理微信开头20多行说明）
func parseWeChat(f io.Reader, uid, bookID uint) ([]models.Transaction, error) {
	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1
	all, _ := reader.ReadAll()
	expenseCat := findCategoryOrCreate(uid, bookID, "其他支出", models.KindExpense, "📦")
	incomeCat := findCategoryOrCreate(uid, bookID, "其他收入", models.KindIncome, "💰")
	acc := findAccountOrFirst(uid, bookID)

	var out []models.Transaction
	started := false
	for _, row := range all {
		if len(row) == 0 {
			continue
		}
		first := strings.TrimSpace(row[0])
		if first == "交易时间" || strings.HasPrefix(first, "交易时间") {
			started = true
			continue
		}
		if !started || len(row) < 5 {
			continue
		}
		// 微信列：交易时间,交易类型,交易对方,商品,收/支,金额(元),支付方式,当前状态,交易单号,商户单号,备注
		dateStr := row[0]
		ioType := strings.TrimSpace(row[4])
		amountStr := strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(row[5]), "¥"), "￥")
		desc := strings.TrimSpace(row[3])
		merchant := strings.TrimSpace(row[2])
		amt, _ := strconv.ParseFloat(amountStr, 64)
		if amt <= 0 {
			continue
		}
		d, _ := time.Parse("2006-01-02 15:04:05", dateStr)
		if d.IsZero() {
			continue
		}
		tx := models.Transaction{
			Amount:      amt,
			Currency:    "CNY",
			AccountID:   acc.ID,
			TxDate:      d,
			Description: desc,
			Merchant:    merchant,
		}
		switch ioType {
		case "支出":
			tx.Type = models.TxExpense
			tx.CategoryID = expenseCat.ID
		case "收入":
			tx.Type = models.TxIncome
			tx.CategoryID = incomeCat.ID
		default:
			continue
		}
		tx.IncludeInBalance = true
		tx.IncludeInBudget = true
		out = append(out, tx)
	}
	return out, nil
}

// parseAlipay 简化版支付宝账单解析
func parseAlipay(f io.Reader, uid, bookID uint) ([]models.Transaction, error) {
	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1
	all, _ := reader.ReadAll()
	expenseCat := findCategoryOrCreate(uid, bookID, "其他支出", models.KindExpense, "📦")
	incomeCat := findCategoryOrCreate(uid, bookID, "其他收入", models.KindIncome, "💰")
	acc := findAccountOrFirst(uid, bookID)

	var out []models.Transaction
	started := false
	for _, row := range all {
		if len(row) == 0 {
			continue
		}
		first := strings.TrimSpace(row[0])
		if first == "交易时间" || first == "交易创建时间" {
			started = true
			continue
		}
		if !started || len(row) < 6 {
			continue
		}
		// 支付宝列大致：交易创建时间,付款时间,最近修改时间,交易来源地,类型,交易对方,商品名称,金额（元）,收/支,交易状态,服务费,成功退款,备注,资金状态
		dateStr := strings.TrimSpace(row[0])
		if strings.TrimSpace(row[1]) != "" {
			dateStr = strings.TrimSpace(row[1])
		}
		ioType := strings.TrimSpace(row[8])
		amountStr := strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(row[7]), "¥"), "￥")
		amountStr = strings.ReplaceAll(amountStr, ",", "")
		desc := strings.TrimSpace(row[6])
		merchant := strings.TrimSpace(row[5])
		status := strings.TrimSpace(row[9])
		if status != "交易成功" && status != "支付成功" {
			continue
		}
		amt, _ := strconv.ParseFloat(amountStr, 64)
		if amt <= 0 {
			continue
		}
		d, _ := time.Parse("2006-01-02 15:04:05", dateStr)
		if d.IsZero() {
			continue
		}
		tx := models.Transaction{
			Amount:      amt,
			Currency:    "CNY",
			AccountID:   acc.ID,
			TxDate:      d,
			Description: desc,
			Merchant:    merchant,
		}
		switch ioType {
		case "支出":
			tx.Type = models.TxExpense
			tx.CategoryID = expenseCat.ID
		case "收入":
			tx.Type = models.TxIncome
			tx.CategoryID = incomeCat.ID
		default:
			continue
		}
		tx.IncludeInBalance = true
		tx.IncludeInBudget = true
		out = append(out, tx)
	}
	return out, nil
}

// ========== 辅助工具 ==========
func findCategoryOrCreate(uid, bookID uint, name string, kind models.CategoryKind, icon string) models.Category {
	var c models.Category
	err := database.DB.Where("user_id = ? AND (book_id = 0 OR book_id = ?) AND name = ? AND kind = ?", uid, bookID, name, kind).First(&c).Error
	if err == nil {
		return c
	}
	c = models.Category{
		UserID: uid, BookID: bookID, Name: name, Kind: kind, Icon: icon,
	}
	database.DB.Create(&c)
	return c
}

func matchCategory(uid, bookID uint, name string, kind models.CategoryKind, fallback models.Category) uint {
	if name == "" {
		return fallback.ID
	}
	var c models.Category
	err := database.DB.Where("user_id = ? AND (book_id = 0 OR book_id = ?) AND name = ? AND kind = ?", uid, bookID, name, kind).First(&c).Error
	if err == nil {
		return c.ID
	}
	// 模糊
	err = database.DB.Where("user_id = ? AND (book_id = 0 OR book_id = ?) AND kind = ? AND name LIKE ?",
		uid, bookID, kind, "%"+name+"%").First(&c).Error
	if err == nil {
		return c.ID
	}
	return fallback.ID
}

func findAccountOrFirst(uid, bookID uint) models.Account {
	var c models.Account
	err := database.DB.Where("user_id = ? AND (book_id = 0 OR book_id = ?)", uid, bookID).
		Order("id ASC").First(&c).Error
	if err != nil {
		// 创建一个默认账户
		c = models.Account{
			UserID: uid, BookID: bookID, Name: "导入账户",
			Type: models.AccCash, Currency: "CNY", Icon: "📦",
		}
		database.DB.Create(&c)
	}
	return c
}

// DownloadImportTemplate 下载CSV模板
func DownloadImportTemplate(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/csv; charset=utf-8")
	c.Writer.Header().Set("Content-Disposition", "attachment; filename=huozhi_template.csv")
	c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})
	w := csv.NewWriter(c.Writer)
	w.Write([]string{"交易日期", "类型(expense/income)", "金额", "分类名称", "描述/备注"})
	now := time.Now()
	w.Write([]string{now.Format("2006-01-02"), "expense", "35.50", "餐饮", "午餐：黄焖鸡米饭"})
	w.Write([]string{now.Format("2006-01-02"), "income", "8000.00", "工资", "8月工资"})
	w.Flush()
}

// GetBill 获取月度账单完整数据（前端可渲染 + window.print() 导出 PDF）
// Query: month=2026-09, book_id=2
func GetBill(c *gin.Context) {
	uid := middleware.GetUID(c)
	monthStr := c.Query("month")
	if monthStr == "" {
		monthStr = time.Now().Format("2006-01")
	}
	bookIDStr := c.Query("book_id")

	// 解析月份
	loc := time.Now().Location()
	parsed, err := time.Parse("2006-01", monthStr)
	if err != nil {
		Bad(c, "月份格式错误，应为 YYYY-MM")
		return
	}
	first := time.Date(parsed.Year(), parsed.Month(), 1, 0, 0, 0, 0, loc)
	last := first.AddDate(0, 1, -1)

	q := database.DB.Where("user_id = ?", uid)
	if bookIDStr != "" && bookIDStr != "0" {
		var bid uint
		fmt.Sscanf(bookIDStr, "%d", &bid)
		if bid > 0 {
			q = q.Where("book_id = ?", bid)
		}
	}

	// === 收支汇总 ===
	var income, expense float64
	var incomeCnt, expenseCnt int64
	q.Model(&models.Transaction{}).
		Where("tx_date >= ? AND tx_date < ? AND type IN ? AND include_in_balance = ?",
			first, last.AddDate(0, 0, 1), []string{string(models.TxIncome), string(models.TxRefund)}, true).
		Select("COALESCE(SUM(amount), 0), COUNT(*)").Row().Scan(&income, &incomeCnt)
	q.Model(&models.Transaction{}).
		Where("tx_date >= ? AND tx_date < ? AND type = ? AND include_in_balance = ?",
			first, last.AddDate(0, 0, 1), string(models.TxExpense), true).
		Select("COALESCE(SUM(amount), 0), COUNT(*)").Row().Scan(&expense, &expenseCnt)

	// === 分类汇总 ===
	var catRows []struct {
		CategoryID uint
		Kind       string
		SumAmount  float64
		Count      int64
	}
	q.Model(&models.Transaction{}).
		Where("tx_date >= ? AND tx_date < ? AND type IN ?",
			first, last.AddDate(0, 0, 1), []string{string(models.TxExpense), string(models.TxIncome), string(models.TxRefund)}).
		Select("category_id, type, SUM(amount) sum_amount, COUNT(*) count").
		Group("category_id, type").Scan(&catRows)

	catMap := make(map[uint]map[string]float64)
	catCntMap := make(map[uint]int64)
	for _, r := range catRows {
		if _, ok := catMap[r.CategoryID]; !ok {
			catMap[r.CategoryID] = map[string]float64{"expense": 0, "income": 0}
		}
		switch r.Kind {
		case string(models.TxExpense):
			catMap[r.CategoryID]["expense"] += r.SumAmount
		default:
			catMap[r.CategoryID]["income"] += r.SumAmount
		}
		catCntMap[r.CategoryID] += r.Count
	}

	var cats []models.Category
	database.DB.Where("id IN ?", mapKeys(catMap)).Find(&cats)
	catInfo := make(map[uint]models.Category)
	for _, c := range cats {
		catInfo[c.ID] = c
	}

	type catOut struct {
		ID      uint    `json:"id"`
		Name    string  `json:"name"`
		Icon    string  `json:"icon"`
		Color   string  `json:"color"`
		Amount  float64 `json:"amount"`
		Count   int64   `json:"count"`
		Percent float64 `json:"percent"`
		Kind    string  `json:"kind"`
	}
	var expRank, incRank []catOut
	for id, m := range catMap {
		info := catInfo[id]
		if m["expense"] > 0 {
			pct := 0.0
			if expense > 0 {
				pct = round2(m["expense"] / expense * 100)
			}
			expRank = append(expRank, catOut{
				ID: id, Name: info.Name, Icon: info.Icon, Color: info.Color,
				Amount: round2(m["expense"]), Count: catCntMap[id], Percent: pct, Kind: "expense",
			})
		}
		if m["income"] > 0 {
			pct := 0.0
			if income > 0 {
				pct = round2(m["income"] / income * 100)
			}
			incRank = append(incRank, catOut{
				ID: id, Name: info.Name, Icon: info.Icon, Color: info.Color,
				Amount: round2(m["income"]), Count: catCntMap[id], Percent: pct, Kind: "income",
			})
		}
	}
	sortByAmountDesc(expRank)
	sortByAmountDesc(incRank)

	// === 每日趋势 ===
	type trendPoint struct {
		Date    string  `json:"date"`
		Income  float64 `json:"income"`
		Expense float64 `json:"expense"`
		Net     float64 `json:"net"`
	}
	var days []trendPoint
	for d := first; !d.After(last); d = d.AddDate(0, 0, 1) {
		var inc, exp float64
		q.Model(&models.Transaction{}).
			Where("tx_date >= ? AND tx_date < ? AND type IN ? AND include_in_balance = ?",
				d, d.AddDate(0, 0, 1), []string{string(models.TxIncome), string(models.TxRefund)}, true).
			Select("COALESCE(SUM(amount), 0)").Row().Scan(&inc)
		q.Model(&models.Transaction{}).
			Where("tx_date >= ? AND tx_date < ? AND type = ? AND include_in_balance = ?",
				d, d.AddDate(0, 0, 1), string(models.TxExpense), true).
			Select("COALESCE(SUM(amount), 0)").Row().Scan(&exp)
		days = append(days, trendPoint{
			Date:    d.Format("01-02"),
			Income:  round2(inc),
			Expense: round2(exp),
			Net:     round2(inc - exp),
		})
	}

	// === 预算执行 ===
	var budgets []models.Budget
	database.DB.Where("user_id = ? AND start_date <= ? AND end_date >= ?", uid, last, first).Find(&budgets)
	type budgOut struct {
		ID          uint    `json:"id"`
		Name        string  `json:"name"`
		Amount      float64 `json:"amount"`
		UsedAmount  float64 `json:"used_amount"`
		Remaining   float64 `json:"remaining"`
		UsageRate   float64 `json:"usage_rate"`
		IsOverBudget bool    `json:"is_over_budget"`
		CategoryID  uint    `json:"category_id"`
		CategoryName string `json:"category_name"`
	}
	var budgOuts []budgOut
	for _, b := range budgets {
		name := "总预算"
		if b.CategoryID > 0 {
			if c, ok := catInfo[b.CategoryID]; ok {
				name = c.Name
			}
		}
		rate := 0.0
		if b.Amount > 0 {
			rate = round2(b.UsedAmount / b.Amount * 100)
		}
		budgOuts = append(budgOuts, budgOut{
			ID: b.ID, Amount: b.Amount, UsedAmount: round2(b.UsedAmount),
			Remaining: round2(b.Amount - b.UsedAmount), UsageRate: rate,
			IsOverBudget: b.UsedAmount > b.Amount, CategoryID: b.CategoryID,
			Name: name, CategoryName: name,
		})
	}

	// === 账户资产快照 ===
	var accounts []models.Account
	database.DB.Where("user_id = ? AND is_archived = ?", uid, false).Find(&accounts)
	var totalAsset, totalDebt float64
	for _, a := range accounts {
		if !a.IncludeInTotal {
			continue
		}
		switch a.Type {
		case models.AccLiability, models.AccCredit:
			totalDebt += a.Balance
		default:
			totalAsset += a.Balance
		}
	}

	// === 书籍信息 ===
	var bookName string
	if bookIDStr != "" && bookIDStr != "0" {
		var b models.Book
		database.DB.Where("id = ? AND user_id = ?", mustUint(bookIDStr), uid).First(&b)
		bookName = b.Name
	}

	// === 用户信息 ===
	var user models.User
	database.DB.First(&user, uid)

	OK(c, gin.H{
		"meta": gin.H{
			"month":       monthStr,
			"range_start": first.Format("2006-01-02"),
			"range_end":   last.Format("2006-01-02"),
			"book_name":   bookName,
			"user_name":   user.Nickname,
			"generated_at": time.Now().Format("2006-01-02 15:04:05"),
		},
		"summary": gin.H{
			"total_income":      round2(income),
			"total_expense":     round2(expense),
			"net":               round2(income - expense),
			"income_count":      incomeCnt,
			"expense_count":     expenseCnt,
			"transaction_count": incomeCnt + expenseCnt,
			"avg_daily_expense": round2(expense / float64(last.Day())),
		},
		"category_expense": expRank,
		"category_income":  incRank,
		"daily_trend":      days,
		"budgets":          budgOuts,
		"assets": gin.H{
			"total_asset": round2(totalAsset),
			"total_debt":  round2(totalDebt),
			"net_asset":   round2(totalAsset - totalDebt),
		},
	})
}

func mapKeys(m map[uint]map[string]float64) []uint {
	out := make([]uint, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func mustUint(s string) uint {
	var n uint
	fmt.Sscanf(s, "%d", &n)
	return n
}
