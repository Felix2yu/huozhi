package handlers

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"huozhi/internal/database"
	"huozhi/internal/models"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ========== 钱迹 (QianJi) 账单导入 ==========
// 钱迹导出的 xlsx 为单工作表，表头（示例）：
// ID / 时间 / 账本 / 分类 / 二级分类 / 类型 / 金额 / 币种 / 账户1 / 账户2 /
// 备注 / 已报销 / 手续费 / 优惠券 / 记账者 / 账单标记 / 标签 / 账单图片 / 关联账单
// 类型取值：支出 / 收入 / 转账 / 退款 等。

// parseQianJi 解析钱迹 xlsx 账单
func parseQianJi(r io.Reader, uid, bookID uint) ([]models.Transaction, error) {
	rows, err := readXLSX(r)
	if err != nil {
		return nil, err
	}
	return parseQianJiRows(rows, uid, bookID)
}

// parseQianJiRows 核心解析逻辑（按表头名映射，便于单测）
func parseQianJiRows(rows [][]string, uid, bookID uint) ([]models.Transaction, error) {
	if len(rows) < 2 {
		return nil, fmt.Errorf("空文件或缺少表头")
	}
	col := map[string]int{}
	for i, h := range rows[0] {
		col[strings.TrimSpace(h)] = i
	}
	get := func(row []string, name string) string {
		i, ok := col[name]
		if !ok || i >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[i])
	}

	// 账本名 -> bookID 缓存：每行的「账本」列决定归属账本，缺省回退到导入目标账本
	bookCache := map[string]uint{}
	resolveBook := func(name string) uint {
		if name == "" {
			return bookID
		}
		if id, ok := bookCache[name]; ok {
			return id
		}
		id := findBookByNameOrCreate(uid, name).ID
		bookCache[name] = id
		return id
	}
	// 账本 -> 兜底分类(其他支出/其他收入/转账) 缓存
	fallbackCache := map[uint][3]models.Category{}
	getFallbacks := func(bid uint) (exp, inc, tr models.Category) {
		if c, ok := fallbackCache[bid]; ok {
			return c[0], c[1], c[2]
		}
		e := findCategoryOrCreate(uid, bid, "其他支出", models.KindExpense, "📦")
		i := findCategoryOrCreate(uid, bid, "其他收入", models.KindIncome, "💰")
		t := findCategoryOrCreate(uid, bid, "转账", models.KindExpense, "🔁")
		fallbackCache[bid] = [3]models.Category{e, i, t}
		return e, i, t
	}

	var out []models.Transaction
	for _, row := range rows[1:] {
		if len(row) == 0 {
			continue
		}
		typeStr := get(row, "类型")
		amountStr := get(row, "金额")
		if amountStr == "" {
			continue
		}
		amt := parseFloat(amountStr)
		if amt <= 0 {
			continue
		}

		// 归属账本：优先取「账本」列，否则回退到导入目标账本
		rowBookID := resolveBook(get(row, "账本"))
		expenseCat, incomeCat, transferCat := getFallbacks(rowBookID)

		// 日期：钱迹为 "2006-01-02 15:04:05"，兼容多种格式
		d := parseQianJiDate(get(row, "时间"))

		tx := models.Transaction{
			BookID:      rowBookID,
			Amount:      amt,
			Currency:    firstNonEmpty(get(row, "币种"), "CNY"),
			TxDate:      d,
			Description: get(row, "备注"),
			Remark:      get(row, "备注"),
			// 钱迹原始交易 ID：用于导入幂等去重与「关联账单」解析
			ExternalID: get(row, "ID"),
			// 记账者（协作账本中记录是谁记的账）
			RecordedBy: get(row, "记账者"),
			// 账单标记（信用卡账单归属标记）
			BillMarker: get(row, "账单标记"),
		}

		// 账户（先解析，便于按「账户1+账户2 同时存在」兜底识别转账）
		a1 := get(row, "账户1")
		a2 := get(row, "账户2")

		subName := get(row, "二级分类")
		switch typeStr {
		case "收入":
			tx.Type = models.TxIncome
			parentID := matchCategory(uid, rowBookID, get(row, "分类"), models.KindIncome, incomeCat)
			tx.CategoryID = resolveSubCategory(uid, parentID, subName)
		case "退款":
			tx.Type = models.TxRefund
			parentID := matchCategoryAny(uid, bookID, get(row, "分类"), incomeCat)
			tx.CategoryID = resolveSubCategory(uid, parentID, subName)
		case "转账":
			tx.Type = models.TxTransfer
			tx.CategoryID = resolveSubCategory(uid, transferCat.ID, subName)
			if fee := parseFloat(get(row, "手续费")); fee > 0 {
				tx.TransferFee = fee
			}
			if disc := parseFloat(get(row, "优惠券")); disc > 0 {
				tx.TransferDiscount = disc
			}
		case "支出":
			tx.Type = models.TxExpense
			parentID := matchCategory(uid, rowBookID, get(row, "分类"), models.KindExpense, expenseCat)
			tx.CategoryID = resolveSubCategory(uid, parentID, subName)
		default:
			// 其他 / 未知类型：钱迹中「账户1、账户2 同时存在」即视为转账（含信用卡还款等）；
			// 否则保守视为支出。
			if a1 != "" && a2 != "" {
				tx.Type = models.TxTransfer
				tx.CategoryID = resolveSubCategory(uid, transferCat.ID, subName)
				if fee := parseFloat(get(row, "手续费")); fee > 0 {
					tx.TransferFee = fee
				}
				if disc := parseFloat(get(row, "优惠券")); disc > 0 {
					tx.TransferDiscount = disc
				}
			} else {
				tx.Type = models.TxExpense
				parentID := matchCategory(uid, rowBookID, get(row, "分类"), models.KindExpense, expenseCat)
				tx.CategoryID = resolveSubCategory(uid, parentID, subName)
			}
		}

		// 账户
		if a1 != "" {
			tx.AccountID = findAccountByNameOrCreate(uid, bookID, a1).ID
		}
		if a2 != "" {
			tx.ToAccountID = findAccountByNameOrCreate(uid, bookID, a2).ID
		}

		// 报销状态：钱迹「已报销」列一般为「是/否」、空、日期或报销金额。
		// 若是非空数字则视为报销金额（同时置 done）；否则按否定词判定；其余非空视为已报销。
		if rb := get(row, "已报销"); rb != "" {
			if num := parseFloat(rb); num > 0 {
				tx.ReimburseStatus = "done"
				tx.ReimburseAmount = num
			} else if isReimbursed(rb) {
				tx.ReimburseStatus = "done"
			} else {
				tx.ReimburseStatus = "none"
			}
		}

		// 标签
		if tags := get(row, "标签"); tags != "" {
			for _, name := range splitTags(tags) {
				t := findTagOrCreate(uid, rowBookID, name)
				tx.Tags = append(tx.Tags, &t)
			}
		}

		// 账单图片：钱迹导出为图片 URL 列表（分号/换行/竖线分隔），直接存入 Images
		if imgs := get(row, "账单图片"); imgs != "" {
			tx.Images = splitImages(imgs)
		}

		// 关联账单：记录原始交易外部 ID，导入入库后再解析为 RefundOfID
		if rel := get(row, "关联账单"); rel != "" {
			tx.RefundOfExternalID = rel
		}

		tx.IncludeInBalance = true
		tx.IncludeInBudget = true
		out = append(out, tx)
	}
	return out, nil
}

func parseQianJiDate(s string) time.Time {
	for _, layout := range []string{
		"2006-01-02 15:04:05",
		"2006-01-02",
		"2006/01/02 15:04:05",
		"2006/01/02",
	} {
		if d, err := time.Parse(layout, strings.TrimSpace(s)); err == nil {
			return d
		}
	}
	return time.Now()
}

// matchCategoryAny 按名称匹配分类（不限 kind），用于退款等需保留原分类名的场景
func matchCategoryAny(uid, bookID uint, name string, fallback models.Category) uint {
	if name == "" {
		return fallback.ID
	}
	var c models.Category
	if err := database.DB.Where("user_id = ? AND (book_id = 0 OR book_id = ?) AND name = ?", uid, bookID, name).First(&c).Error; err == nil {
		return c.ID
	}
	if err := database.DB.Where("user_id = ? AND (book_id = 0 OR book_id = ?) AND name LIKE ?", uid, bookID, "%"+name+"%").First(&c).Error; err == nil {
		return c.ID
	}
	return fallback.ID
}

// resolveSubCategory 在一级分类 parentID 下查找/创建名为 subName 的二级分类，
// 返回该二级分类的 ID；若 subName 为空则返回 parentID 本身（仅挂一级分类）。
// 二级分类继承父级的 book_id / kind / 图标 / 颜色，与 app 既有的「叶子分类」约定一致。
func resolveSubCategory(uid, parentID uint, subName string) uint {
	subName = strings.TrimSpace(subName)
	if subName == "" {
		return parentID
	}
	var parent models.Category
	if err := database.DB.First(&parent, parentID).Error; err != nil {
		return parentID
	}
	kind := parent.Kind
	var sub models.Category
	if err := database.DB.Where("user_id = ? AND parent_id = ? AND name = ? AND kind = ?",
		uid, parentID, subName, kind).First(&sub).Error; err == nil {
		return sub.ID
	}
	sub = models.Category{
		UserID:   uid,
		BookID:   parent.BookID,
		ParentID: parentID,
		Name:     subName,
		Kind:     kind,
		Icon:     parent.Icon,
		Color:    parent.Color,
	}
	database.DB.Create(&sub)
	return sub.ID
}

// findAccountByNameOrCreate 按名称匹配账户，不存在则按名称猜测类型后创建
func findAccountByNameOrCreate(uid, bookID uint, name string) models.Account {
	var a models.Account
	if err := database.DB.Where("user_id = ? AND (book_id = 0 OR book_id = ?) AND name = ?", uid, bookID, name).First(&a).Error; err == nil {
		return a
	}
	t := guessAccountType(name)
	a = models.Account{UserID: uid, BookID: bookID, Name: name, Type: t, Currency: "CNY"}
	switch t {
	case models.AccCredit:
		a.Icon = "💳"
	case models.AccBank:
		a.Icon = "🏦"
	case models.AccPrepaid:
		a.Icon = "🍱"
	case models.AccLiability:
		a.Icon = "💸"
	case models.AccVirtual:
		a.Icon = "📱"
	default:
		a.Icon = "💰"
	}
	database.DB.Create(&a)
	return a
}

// guessAccountType 依据账户名猜测账户类型（尽力而为）
func guessAccountType(name string) models.AccountType {
	n := strings.ToLower(name)
	switch {
	case strings.Contains(name, "信用卡") || strings.Contains(name, "贷记") || strings.Contains(n, "credit"):
		return models.AccCredit
	case strings.Contains(name, "花呗") || strings.Contains(name, "借呗") || strings.Contains(name, "白条") ||
		strings.Contains(name, "负债") || strings.Contains(n, "loan"):
		return models.AccLiability
	case strings.Contains(name, "银行") || strings.Contains(name, "储蓄") || strings.Contains(name, "借记"):
		return models.AccBank
	case strings.Contains(name, "饭卡") || strings.Contains(name, "公交") || strings.Contains(name, "储值") ||
		strings.Contains(name, "加油") || strings.Contains(name, "预付"):
		return models.AccPrepaid
	case strings.Contains(name, "支付宝") || strings.Contains(name, "微信") || strings.Contains(name, "零钱") ||
		strings.Contains(name, "余额"):
		return models.AccVirtual
	default:
		return models.AccCash
	}
}

// findTagOrCreate 按名称匹配或创建标签
func findTagOrCreate(uid, bookID uint, name string) models.Tag {
	var t models.Tag
	if err := database.DB.Where("user_id = ? AND (book_id = 0 OR book_id = ?) AND name = ?", uid, bookID, name).First(&t).Error; err == nil {
		return t
	}
	t = models.Tag{UserID: uid, BookID: bookID, Name: name}
	database.DB.Create(&t)
	return t
}

func splitTags(s string) []string {
	parts := regexp.MustCompile(`[、,，;；\s/]+`).Split(s, -1)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// splitImages 按钱迹「账单图片」列的分隔符拆分图片 URL/路径（分号/换行/竖线）
func splitImages(s string) []string {
	parts := regexp.MustCompile(`[;\n|]+`).Split(s, -1)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// isReimbursed 判断钱迹「已报销」列是否表示已报销。
// 明确否定词（否/无/未/不/没/no/false/none 等）视为未报销，其余非空值视为已报销。
func isReimbursed(s string) bool {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return false
	}
	negatives := []string{"否", "无", "未", "不", "没", "no", "false", "none", "n/a", "null"}
	for _, n := range negatives {
		if strings.Contains(s, n) {
			return false
		}
	}
	return true
}

func parseFloat(s string) float64 {
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", "")
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// ========== 极简 xlsx 读取（无需第三方库，覆盖钱迹导出的简单结构） ==========

func readXLSX(r io.Reader) ([][]string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	// 共享字符串表
	var shared []string
	if f := zipFile(zr, "xl/sharedStrings.xml"); f != nil {
		rc, _ := f.Open()
		shared = readSharedStrings(rc)
		rc.Close()
	}
	// 定位工作表（优先 sheet1.xml，否则取第一个 worksheets/sheetN.xml）
	sheetFile := zipFile(zr, "xl/worksheets/sheet1.xml")
	if sheetFile == nil {
		re := regexp.MustCompile(`xl/worksheets/sheet\d+\.xml$`)
		for _, zf := range zr.File {
			if re.MatchString(zf.Name) {
				sheetFile = zf
				break
			}
		}
	}
	if sheetFile == nil {
		return nil, fmt.Errorf("未找到工作表")
	}
	rc, err := sheetFile.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return readSheet(rc, shared)
}

func readSharedStrings(r io.Reader) []string {
	dec := xml.NewDecoder(r)
	var strs []string
	var cur strings.Builder
	inT := false
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch e := tok.(type) {
		case xml.StartElement:
			if e.Name.Local == "t" {
				inT = true
				cur.Reset()
			}
		case xml.CharData:
			if inT {
				cur.Write(e)
			}
		case xml.EndElement:
			if e.Name.Local == "t" {
				inT = false
			}
			if e.Name.Local == "si" {
				strs = append(strs, cur.String())
				cur.Reset()
			}
		}
	}
	return strs
}

func readSheet(r io.Reader, shared []string) ([][]string, error) {
	dec := xml.NewDecoder(r)
	var rows [][]string
	var curRow []string
	maxCol := -1
	curCol := 0
	cellType := ""
	inV := false
	var vbuf strings.Builder

	flushCell := func() {
		if !inV {
			return
		}
		val := vbuf.String()
		if cellType == "s" {
			if idx, err := strconv.Atoi(val); err == nil && idx >= 0 && idx < len(shared) {
				val = shared[idx]
			}
		}
		for len(curRow) <= curCol {
			curRow = append(curRow, "")
		}
		curRow[curCol] = val
		inV = false
		vbuf.Reset()
	}

	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch e := tok.(type) {
		case xml.StartElement:
			switch e.Name.Local {
			case "row":
				curRow = []string{}
				maxCol = -1
			case "c":
				cellType = ""
				ref := ""
				for _, attr := range e.Attr {
					switch attr.Name.Local {
					case "r":
						ref = attr.Value
					case "t":
						cellType = attr.Value
					}
				}
				curCol = colIndex(ref)
				if curCol > maxCol {
					maxCol = curCol
				}
				inV = false
				vbuf.Reset()
			case "v", "t":
				if cellType == "inlineStr" || e.Name.Local == "v" {
					inV = true
					vbuf.Reset()
				}
			}
		case xml.CharData:
			if inV {
				vbuf.Write(e)
			}
		case xml.EndElement:
			switch e.Name.Local {
			case "v", "t":
				flushCell()
			case "row":
				if maxCol >= 0 {
					for len(curRow) <= maxCol {
						curRow = append(curRow, "")
					}
				}
				rows = append(rows, curRow)
			}
		}
	}
	return rows, nil
}

// colIndex 将单元格列引用（如 "B"）转为 0 基索引
func colIndex(ref string) int {
	idx := 0
	for _, r := range ref {
		if r >= 'A' && r <= 'Z' {
			idx = idx*26 + int(r-'A'+1)
		} else {
			break
		}
	}
	return idx - 1
}

// isXLSX 依据文件名后缀判断是否为 Excel 文件
func isXLSX(name string) bool {
	n := strings.ToLower(name)
	return strings.HasSuffix(n, ".xlsx") || strings.HasSuffix(n, ".xls")
}

// zipFile 在 zip 读取器中按名称查找文件
func zipFile(zr *zip.Reader, name string) *zip.File {
	for _, f := range zr.File {
		if f.Name == name {
			return f
		}
	}
	return nil
}
