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
