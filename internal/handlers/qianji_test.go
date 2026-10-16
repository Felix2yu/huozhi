package handlers

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"huozhi/internal/database"
	"huozhi/internal/models"
)

var qjSeq uint64

func qjSetup(t *testing.T) (uid, bookID uint) {
	t.Helper()
	n := atomic.AddUint64(&qjSeq, 1)
	u := models.User{
		Username:     fmt.Sprintf("qj_%d", n),
		Email:        strPtrOrNil(fmt.Sprintf("qj_%d@example.com", n)),
		Phone:        strPtrOrNil(fmt.Sprintf("qj_%d_phone", n)),
		PasswordHash: "x",
		Nickname:     "QJ",
		Currency:     "CNY",
		Status:       1,
	}
	if err := database.DB.Create(&u).Error; err != nil {
		t.Fatal(err)
	}
	b := models.Book{UserID: u.ID, Name: "默认账本", Currency: "CNY", IsDefault: true}
	if err := database.DB.Create(&b).Error; err != nil {
		t.Fatal(err)
	}
	// 预置分类，验证按名称匹配
	database.DB.Create(&models.Category{UserID: u.ID, BookID: b.ID, Name: "医疗", Kind: models.KindExpense, Icon: "x"})
	database.DB.Create(&models.Category{UserID: u.ID, BookID: b.ID, Name: "交通", Kind: models.KindExpense, Icon: "x"})
	return u.ID, b.ID
}

// qjBook 获取或创建指定名称的账本（配合钱迹样本里「账本=日常账本」使用）
func qjBook(t *testing.T, uid uint, name string) uint {
	t.Helper()
	var b models.Book
	if database.DB.Where("user_id = ? AND name = ?", uid, name).First(&b).Error == nil {
		return b.ID
	}
	b = models.Book{UserID: uid, Name: name, Currency: "CNY"}
	if err := database.DB.Create(&b).Error; err != nil {
		t.Fatal(err)
	}
	return b.ID
}

// 钱迹样本（与真实导出列一致）
func qianjiSampleRows() [][]string {
	return [][]string{
		{"ID", "时间", "账本", "分类", "二级分类", "类型", "金额", "币种", "账户1", "账户2", "备注", "已报销", "手续费", "优惠券", "记账者", "账单标记", "标签", "账单图片", "关联账单"},
		{"qj1", "2026-09-03 20:40:00", "日常账本", "医疗", "", "支出", "26.9", "CNY", "花呗", "", "家庭意外险", "", "", "", "子翼", "", "", "", ""},
		{"qj2", "2026-08-18 09:34:13", "日常账本", "交通", "火车", "退款", "14", "CNY", "平安银行信用卡", "", "", "", "", "", "子翼", "", "", "", ""},
		{"qj3", "2026-08-17 11:54:35", "日常账本", "其它", "", "转账", "8", "CNY", "交通银行万事达信用卡", "市政府饭卡", "", "", "2.0", "", "子翼", "", "出差;报销", "", ""},
	}
}

func TestParseQianJiRows(t *testing.T) {
	uid, bookID := qjSetup(t)
	// 样本行「账本=日常账本」，分类需建在与该账本一致的账本下才能被按名匹配
	daily := qjBook(t, uid, "日常账本")
	database.DB.Create(&models.Category{UserID: uid, BookID: daily, Name: "医疗", Kind: models.KindExpense, Icon: "x"})
	database.DB.Create(&models.Category{UserID: uid, BookID: daily, Name: "交通", Kind: models.KindExpense, Icon: "x"})
	rows := qianjiSampleRows()
	txs, err := parseQianJiRows(rows, nil,uid, bookID)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(txs) != 3 {
		t.Fatalf("expected 3 txs, got %d", len(txs))
	}

	// 1) 支出
	tx0 := txs[0]
	if tx0.Type != models.TxExpense {
		t.Errorf("row0 type = %s, want expense", tx0.Type)
	}
	if tx0.Amount != models.Money(2690) {
		t.Errorf("row0 amount = %v, want 26.9", tx0.Amount.String())
	}
	if tx0.Currency != "CNY" {
		t.Errorf("row0 currency = %s, want CNY", tx0.Currency)
	}
	if tx0.TxDate.Year() != 2026 || tx0.TxDate.Month() != 9 || tx0.TxDate.Day() != 3 {
		t.Errorf("row0 date = %v, want 2026-09-03", tx0.TxDate)
	}
	if tx0.AccountID == 0 {
		t.Errorf("row0 account not resolved")
	}
	// 分类应匹配到已预置的「医疗」（分类建在与样本「账本=日常账本」一致的账本下）
	var c0 models.Category
	database.DB.First(&c0, tx0.CategoryID)
	if c0.Name != "医疗" {
		t.Errorf("row0 category = %s, want 医疗", c0.Name)
	}
	// 账户名「花呗」应被识别为负债
	var a0 models.Account
	database.DB.First(&a0, tx0.AccountID)
	if a0.Name != "花呗" || a0.Type != models.AccLiability {
		t.Errorf("row0 account = %s/%s, want 花呗/liability", a0.Name, a0.Type)
	}

	// 2) 退款（含二级分类「火车」）
	tx1 := txs[1]
	if tx1.Type != models.TxRefund {
		t.Errorf("row1 type = %s, want refund", tx1.Type)
	}
	if tx1.Amount != models.FromYuan(14) {
		t.Errorf("row1 amount = %v, want 14", tx1.Amount)
	}
	var c1 models.Category
	database.DB.First(&c1, tx1.CategoryID)
	if c1.Name != "火车" {
		t.Errorf("row1 category = %s, want 火车 (二级分类下挂到交通下)", c1.Name)
	}
	if c1.ParentID == 0 {
		t.Errorf("row1 二级分类未挂到一级分类「交通」下")
	}

	// 3) 转账
	tx2 := txs[2]
	if tx2.Type != models.TxTransfer {
		t.Errorf("row2 type = %s, want transfer", tx2.Type)
	}
	if tx2.Amount != models.FromYuan(8) {
		t.Errorf("row2 amount = %v, want 8", tx2.Amount)
	}
	if tx2.ToAccountID == 0 {
		t.Errorf("row2 ToAccountID not resolved")
	}
	if tx2.TransferFee != models.FromYuan(2) {
		t.Errorf("row2 transfer fee = %v, want 2.0", tx2.TransferFee)
	}
	var a2 models.Account
	database.DB.First(&a2, tx2.AccountID)
	if a2.Name != "交通银行万事达信用卡" || a2.Type != models.AccCredit {
		t.Errorf("row2 from account = %s/%s, want 交通银行万事达信用卡/credit", a2.Name, a2.Type)
	}
	var a2to models.Account
	database.DB.First(&a2to, tx2.ToAccountID)
	if a2to.Name != "市政府饭卡" || a2to.Type != models.AccPrepaid {
		t.Errorf("row2 to account = %s/%s, want 市政府饭卡/prepaid", a2to.Name, a2to.Type)
	}
	// 标签
	if len(tx2.Tags) != 2 {
		t.Errorf("row2 tags = %d, want 2 (出差, 报销)", len(tx2.Tags))
	}
}

// 钱迹导出含「二级分类」列时，应在一級分类下创建子分类并把交易挂到该二级分类
func TestParseQianJiRowsSubCategory(t *testing.T) {
	uid, bookID := qjSetup(t)
	daily := qjBook(t, uid, "日常账本")
	database.DB.Create(&models.Category{UserID: uid, BookID: daily, Name: "餐饮", Kind: models.KindExpense, Icon: "🍜"})

	rows := [][]string{
		{"ID", "时间", "账本", "分类", "二级分类", "类型", "金额", "币种", "账户1", "账户2", "备注", "已报销", "手续费", "优惠券", "记账者", "账单标记", "标签", "账单图片", "关联账单"},
		{"sc1", "2026-09-03 12:00:00", "日常账本", "餐饮", "火锅", "支出", "120", "CNY", "现金", "", "聚餐", "", "", "", "子翼", "", "", "", ""},
	}
	txs, err := parseQianJiRows(rows, nil,uid, bookID)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(txs) != 1 {
		t.Fatalf("want 1 tx, got %d", len(txs))
	}

	var sub models.Category
	database.DB.First(&sub, txs[0].CategoryID)
	if sub.Name != "火锅" {
		t.Fatalf("sub category name = %q, want 火锅", sub.Name)
	}
	if sub.ParentID == 0 {
		t.Fatalf("sub category has no parent_id (二级分类未挂到一级分类下)")
	}
	if sub.Kind != models.KindExpense {
		t.Fatalf("sub category kind = %s, want expense", sub.Kind)
	}
	var parent models.Category
	database.DB.First(&parent, sub.ParentID)
	if parent.Name != "餐饮" {
		t.Fatalf("parent name = %q, want 餐饮", parent.Name)
	}

	// 再次导入同名二级分类，应复用而非新建
	txs2, _ := parseQianJiRows(rows, nil,uid, bookID)
	var sub2 models.Category
	database.DB.First(&sub2, txs2[0].CategoryID)
	if sub2.ID != sub.ID {
		t.Fatalf("expected reuse of existing sub category, got new id %d vs %d", sub2.ID, sub.ID)
	}
}

// 钱迹导出含「账本」「已报销」列时，应按账本列归属不同账本，并正确解析报销状态
func TestParseQianJiRowsBookAndReimburse(t *testing.T) {
	uid, _ := qjSetup(t)
	rows := [][]string{
		{"ID", "时间", "账本", "分类", "二级分类", "类型", "金额", "币种", "账户1", "账户2", "备注", "已报销", "手续费", "优惠券", "记账者", "账单标记", "标签", "账单图片", "关联账单"},
		{"b1", "2026-09-03 12:00:00", "旅行账本", "餐饮", "", "支出", "120", "CNY", "现金", "", "午餐", "是", "", "", "子翼", "", "", "", ""},
		{"b2", "2026-09-04 12:00:00", "旅行账本", "餐饮", "", "支出", "80", "CNY", "现金", "", "晚餐", "否", "", "", "子翼", "", "", "", ""},
		{"b3", "2026-09-05 12:00:00", "家庭账本", "餐饮", "", "支出", "50", "CNY", "现金", "", "买菜", "", "", "", "子翼", "", "", "", ""},
	}
	txs, err := parseQianJiRows(rows, nil,uid, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(txs) != 3 {
		t.Fatalf("want 3 txs, got %d", len(txs))
	}

	bookOf := func(tx models.Transaction) string {
		var b models.Book
		database.DB.First(&b, tx.BookID)
		return b.Name
	}
	if bookOf(txs[0]) != "旅行账本" || bookOf(txs[1]) != "旅行账本" {
		t.Errorf("row0/row1 book = %s/%s, want 旅行账本", bookOf(txs[0]), bookOf(txs[1]))
	}
	if bookOf(txs[2]) != "家庭账本" {
		t.Errorf("row2 book = %s, want 家庭账本", bookOf(txs[2]))
	}

	// 报销状态：是->done，否->none，空->未设置
	if txs[0].ReimburseStatus != "done" {
		t.Errorf("row0 reimburse = %q, want done", txs[0].ReimburseStatus)
	}
	if txs[1].ReimburseStatus != "none" {
		t.Errorf("row1 reimburse = %q, want none", txs[1].ReimburseStatus)
	}
	if txs[2].ReimburseStatus != "" {
		t.Errorf("row2 reimburse = %q, want empty", txs[2].ReimburseStatus)
	}
}

func TestParseQianJi_EndToEndXLSX(t *testing.T) {	uid, bookID := qjSetup(t)
	data := buildXLSX(t, qianjiSampleRows())
	txs, err := parseQianJi(bytes.NewReader(data), uid, bookID)
	if err != nil {
		t.Fatalf("parseQianJi error: %v", err)
	}
	if len(txs) != 3 {
		t.Fatalf("expected 3 txs from xlsx, got %d", len(txs))
	}
	if txs[0].Type != models.TxExpense || txs[2].Type != models.TxTransfer {
		t.Errorf("xlsx mapping mismatch: %s / %s", txs[0].Type, txs[2].Type)
	}
}

// 钱迹规则：账户1、账户2 同时存在即视为转账（含信用卡还款），即使 类型 缺失也应识别
func TestParseQianJiRowsTransferByAccounts(t *testing.T) {
	uid, bookID := qjSetup(t)
	rows := [][]string{
		{"ID", "时间", "账本", "分类", "二级分类", "类型", "金额", "币种", "账户1", "账户2", "备注", "已报销", "手续费", "优惠券", "记账者", "账单标记", "标签", "账单图片", "关联账单"},
		{"qjx", "2026-09-03 10:00:00", "日常账本", "", "", "", "500", "CNY", "招商银行储蓄卡", "平安银行信用卡", "信用卡还款", "", "", "", "子翼", "", "", "", ""},
	}
	txs, err := parseQianJiRows(rows, nil,uid, bookID)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(txs) != 1 {
		t.Fatalf("want 1 tx, got %d", len(txs))
	}
	tx := txs[0]
	if tx.Type != models.TxTransfer {
		t.Errorf("type = %s, want transfer (fallback by accounts)", tx.Type)
	}
	if tx.AccountID == 0 || tx.ToAccountID == 0 {
		t.Errorf("accounts not resolved: from=%d to=%d", tx.AccountID, tx.ToAccountID)
	}
	if tx.Amount != models.FromYuan(500) {
		t.Errorf("amount = %v, want 500", tx.Amount)
	}
}

// 信用卡余额方向：正数=欠款。还款（储蓄卡→信用卡）欠款应减少；反向（信用卡→饭卡）欠款应增加。
func TestCreditRepaymentBalanceDirection(t *testing.T) {
	uid, bookID := qjSetup(t)
	bank := models.Account{UserID: uid, BookID: bookID, Name: "招商银行储蓄卡", Type: models.AccBank, Balance: 1000, Currency: "CNY"}
	cc := models.Account{UserID: uid, BookID: bookID, Name: "平安银行信用卡", Type: models.AccCredit, Balance: 500, Currency: "CNY"} // 欠款 500
	prepaid := models.Account{UserID: uid, BookID: bookID, Name: "市政府饭卡", Type: models.AccPrepaid, Balance: 0, Currency: "CNY"}
	database.DB.Create(&bank)
	database.DB.Create(&cc)
	database.DB.Create(&prepaid)

	// 1) 信用卡还款：储蓄卡 -> 信用卡，欠款应减少
	repay := &models.Transaction{Type: models.TxTransfer, AccountID: bank.ID, ToAccountID: cc.ID, Amount: 200, IncludeInBalance: true}
	updateAccountBalances(database.DB, repay, &bank, &cc, true)
	var bank2, cc2 models.Account
	database.DB.First(&bank2, bank.ID)
	database.DB.First(&cc2, cc.ID)
	if bank2.Balance != 800 {
		t.Errorf("bank balance = %v, want 800 (cash reduced)", bank2.Balance)
	}
	if cc2.Balance != 300 {
		t.Errorf("credit debt = %v, want 300 (debt reduced by repayment)", cc2.Balance)
	}

	// 2) 反向：信用卡 -> 饭卡（含手续费 2），欠款应增加
	spend := &models.Transaction{Type: models.TxTransfer, AccountID: cc.ID, ToAccountID: prepaid.ID, Amount: 8, TransferFee: 2, IncludeInBalance: true}
	updateAccountBalances(database.DB, spend, &cc, &prepaid, true)
	var cc3, prepaid2 models.Account
	database.DB.First(&cc3, cc.ID)
	database.DB.First(&prepaid2, prepaid.ID)
	if cc3.Balance <= 300 {
		t.Errorf("credit debt = %v, want > 300 (debt increased when card used)", cc3.Balance)
	}
	if prepaid2.Balance != 8 {
		t.Errorf("prepaid balance = %v, want 8", prepaid2.Balance)
	}
}

// 钱迹导出中此前完全未映射的字段（除 ID 外）：记账者、账单标记、账单图片、关联账单、已报销金额
func TestParseQianJiRowsPreviouslyMissingFields(t *testing.T) {
	uid, _ := qjSetup(t)
	daily := qjBook(t, uid, "日常账本")
	database.DB.Create(&models.Category{UserID: uid, BookID: daily, Name: "餐饮", Kind: models.KindExpense, Icon: "🍜"})
	database.DB.Create(&models.Category{UserID: uid, BookID: daily, Name: "医疗", Kind: models.KindExpense, Icon: "x"})

	rows := [][]string{
		{"ID", "时间", "账本", "分类", "二级分类", "类型", "金额", "币种", "账户1", "账户2", "备注", "已报销", "手续费", "优惠券", "记账者", "账单标记", "标签", "账单图片", "关联账单"},
		// 支出：携带记账者、账单标记（账单图片列在真实钱迹导出中为空——图片内嵌于 xlsx，不占单元格文本）
		{"ext-001", "2026-09-03 12:00:00", "日常账本", "餐饮", "", "支出", "120", "CNY", "现金", "", "午餐", "", "", "", "子翼", "2026-08账单", "", "", ""},
		// 退款：关联回 ext-001（钱迹里「关联账单」通常指向原始支出），已报销列给金额
		{"ext-002", "2026-09-04 12:00:00", "日常账本", "医疗", "", "退款", "50", "CNY", "现金", "", "退药", "88.5", "", "", "子翼", "", "", "", "ext-001"},
	}
	txs, err := parseQianJiRows(rows, nil,uid, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(txs) != 2 {
		t.Fatalf("want 2 txs, got %d", len(txs))
	}

	// 1) 支出行：此前完全缺失的字段应被正确读取
	exp := txs[0]
	if exp.ExternalID != "ext-001" {
		t.Errorf("ExternalID = %q, want ext-001", exp.ExternalID)
	}
	if exp.RecordedBy != "子翼" {
		t.Errorf("RecordedBy = %q, want 子翼", exp.RecordedBy)
	}
	if exp.BillMarker != "2026-08账单" {
		t.Errorf("BillMarker = %q, want 2026-08账单", exp.BillMarker)
	}
	if len(exp.Images) != 0 {
		t.Errorf("Images = %v, want 空（账单图片列文本不再解析，图片走内嵌提取）", exp.Images)
	}

	// 2) 退款行：已报销列是数字 -> 报销状态 done + 报销金额；关联账单 -> RefundOfExternalID（原始引用留存）
	ref := txs[1]
	if ref.ExternalID != "ext-002" {
		t.Errorf("refund ExternalID = %q, want ext-002", ref.ExternalID)
	}
	if ref.ReimburseStatus != "done" {
		t.Errorf("refund ReimburseStatus = %q, want done", ref.ReimburseStatus)
	}
	if ref.ReimburseAmount != models.Money(8850) {
		t.Errorf("refund ReimburseAmount = %v, want 88.5", ref.ReimburseAmount.String())
	}
	if ref.RefundOfExternalID != "ext-001" {
		t.Errorf("RefundOfExternalID = %q, want ext-001", ref.RefundOfExternalID)
	}
}

// ============ 测试辅助：构造最小合法 xlsx（inline string） ============

const (
	ctTypes = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
		`<Default Extension="xml" ContentType="application/xml"/>` +
		`<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>` +
		`<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>` +
		`</Types>`
	relsRoot = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>` +
		`</Relationships>`
	wbXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" ` +
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
		`<sheets><sheet name="账单" sheetId="1" r:id="rId1"/></sheets></workbook>`
	wbRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>` +
		`</Relationships>`
)

func buildXLSX(t *testing.T, rows [][]string) []byte {
	t.Helper()
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	sb.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	for ri, row := range rows {
		sb.WriteString(fmt.Sprintf(`<row r="%d">`, ri+1))
		for ci, cell := range row {
			cl := colLetter(ci)
			sb.WriteString(fmt.Sprintf(`<c r="%s%d" t="inlineStr"><is><t>%s</t></is></c>`, cl, ri+1, xmlEscape(cell)))
		}
		sb.WriteString(`</row>`)
	}
	sb.WriteString(`</sheetData></worksheet>`)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	write := func(name, content string) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(content))
	}
	write("[Content_Types].xml", ctTypes)
	write("_rels/.rels", relsRoot)
	write("xl/workbook.xml", wbXML)
	write("xl/_rels/workbook.xml.rels", wbRels)
	write("xl/worksheets/sheet1.xml", sb.String())
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func xmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}

func colLetter(i int) string {
	s := ""
	i++
	for i > 0 {
		i--
		s = string(rune('A'+i%26)) + s
		i /= 26
	}
	return s
}

// 钱迹自定义一级分类（系统里不存在）不应被兜底吞掉：应按名称创建一级分类，
// 二级分类挂在其下；「分类=二级分类同名」时直接挂一级，不建同名子分类。
func TestParseQianJiRowsCustomTopCategory(t *testing.T) {
	uid, bookID := qjSetup(t)
	rows := [][]string{
		{"ID", "时间", "账本", "分类", "二级分类", "类型", "金额", "币种", "账户1", "账户2", "备注", "已报销", "手续费", "优惠券", "记账者", "账单标记", "标签", "账单图片", "关联账单"},
		{"qj-c1", "2026-09-04 12:00:00", "日常账本", "电器数码", "软件服务", "支出", "100", "CNY", "花呗", "", "Adobe", "", "", "", "子翼", "", "", "", ""},
		{"qj-c2", "2026-09-04 12:30:00", "日常账本", "植物花卉", "", "支出", "50", "CNY", "花呗", "", "多肉", "", "", "", "子翼", "", "", "", ""},
		{"qj-c3", "2026-09-04 13:00:00", "日常账本", "食物", "食物", "支出", "20", "CNY", "花呗", "", "夜宵", "", "", "", "子翼", "", "", "", ""},
	}
	txs, err := parseQianJiRows(rows, nil, uid, bookID)
	if err != nil {
		t.Fatal(err)
	}
	if len(txs) != 3 {
		t.Fatalf("parsed %d txs, want 3", len(txs))
	}

	assertTop := func(name string) models.Category {
		t.Helper()
		var c models.Category
		if err := database.DB.Where("user_id = ? AND name = ? AND parent_id = 0", uid, name).First(&c).Error; err != nil {
			t.Fatalf("自定义一级分类「%s」未创建: %v", name, err)
		}
		return c
	}

	// 1) 电器数码 → 新建一级分类；软件服务挂其下
	top := assertTop("电器数码")
	var sub models.Category
	if err := database.DB.First(&sub, txs[0].CategoryID).Error; err != nil {
		t.Fatal(err)
	}
	if sub.Name != "软件服务" || sub.ParentID != top.ID {
		t.Errorf("交易应挂在「电器数码」下的二级「软件服务」(parent=%d), got %s (parent=%d)", top.ID, sub.Name, sub.ParentID)
	}

	// 2) 植物花卉（无二级）→ 新建一级并直接挂
	top = assertTop("植物花卉")
	if txs[1].CategoryID != top.ID {
		t.Errorf("无二级分类的交易应直接挂一级「植物花卉」id=%d, got %d", top.ID, txs[1].CategoryID)
	}

	// 3) 分类=食物、二级分类=食物 → 只建一级「食物」，不建同名子分类
	top = assertTop("食物")
	var sameNameSubs int64
	database.DB.Model(&models.Category{}).Where("user_id = ? AND name = ? AND parent_id > 0", uid, "食物").Count(&sameNameSubs)
	if sameNameSubs != 0 {
		t.Errorf("不应创建与一级同名的二级分类")
	}
	if txs[2].CategoryID != top.ID {
		t.Errorf("同名二级应直接挂一级「食物」id=%d, got %d", top.ID, txs[2].CategoryID)
	}
}

// 钱迹「还款」本质是转账：从储蓄卡等资金账户转到信用卡 / 花呗 / 房贷等账户。
// 真实导出中「还款」是仅次于「支出」的高频类型，必须显式识别为转账，
// 不能依赖 default 分支的「账户1、账户2 同时存在」猜测 —— 一旦目标账户缺失，
// 旧实现会把它误判成支出。本用例覆盖两种账户结构以确保回归可控。
func TestParseQianJiRowsRepayment(t *testing.T) {
	uid, bookID := qjSetup(t)
	rows := [][]string{
		{"ID", "时间", "账本", "分类", "二级分类", "类型", "金额", "币种", "账户1", "账户2", "备注", "已报销", "手续费", "优惠券", "记账者", "账单标记", "标签", "账单图片", "关联账单"},
		// 标准还款：储蓄卡 -> 信用卡
		{"rp1", "2026-09-15 10:15:57", "日常账本", "其它", "", "还款", "180.59", "CNY", "交通银行储蓄卡", "交通银行万事达信用卡", "", "", "", "", "子翼", "", "", ""},
		// 还款到花呗（互联网金融账户，负债类），并带手续费
		{"rp2", "2026-09-15 10:14:44", "日常账本", "其它", "", "还款", "470.57", "CNY", "民生银行储蓄卡", "花呗", "", "", "2.0", "", "子翼", "", "", ""},
		// 目标账户缺失：仍应为转账（旧实现落到 default 后会判为支出）
		{"rp3", "2026-09-15 10:13:13", "日常账本", "其它", "", "还款", "121.63", "CNY", "民生银行储蓄卡", "", "", "", "", "", "子翼", "", "", ""},
	}
	txs, err := parseQianJiRows(rows, nil, uid, bookID)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(txs) != 3 {
		t.Fatalf("want 3 txs, got %d", len(txs))
	}

	// 1) 全部「还款」都必须落成转账（而非支出）
	for i, tx := range txs {
		if tx.Type != models.TxTransfer {
			t.Errorf("row%d type = %s, want transfer", i, tx.Type)
		}
		if tx.Amount != models.FromYuan([]float64{180.59, 470.57, 121.63}[i]) {
			t.Errorf("row%d amount = %v, want %v", i, tx.Amount.String(), tx.Amount)
		}
	}

	// 2) 储蓄卡 -> 信用卡：目标账户应被建为信用卡类型
	var cc models.Account
	if err := database.DB.First(&cc, txs[0].ToAccountID).Error; err != nil {
		t.Fatalf("row0 to account not resolved: %v", err)
	}
	if cc.Name != "交通银行万事达信用卡" || cc.Type != models.AccCredit {
		t.Errorf("row0 to account = %s/%s, want 交通银行万事达信用卡/credit", cc.Name, cc.Type)
	}

	// 3) 储蓄卡 -> 花呗：目标账户应为负债类，且手续费落入 TransferFee
	var hb models.Account
	if err := database.DB.First(&hb, txs[1].ToAccountID).Error; err != nil {
		t.Fatalf("row1 to account not resolved: %v", err)
	}
	if hb.Name != "花呗" || hb.Type != models.AccLiability {
		t.Errorf("row1 to account = %s/%s, want 花呗/liability", hb.Name, hb.Type)
	}
	if txs[1].TransferFee != models.FromYuan(2) {
		t.Errorf("row1 transfer fee = %v, want 2", txs[1].TransferFee)
	}

	// 4) 目标账户缺失的行：账户1 仍解析、转出方向保留，ToAccountID 为 0 但不影响类型判定
	if txs[2].AccountID == 0 {
		t.Errorf("row2 from account not resolved")
	}
	if txs[2].ToAccountID != 0 {
		t.Errorf("row2 to account = %d, want 0 (目标账户缺失)", txs[2].ToAccountID)
	}
}

// ============ 真实钱迹导出格式（sharedStrings）构造器 ============
// 真实钱迹导出使用 xl/sharedStrings.xml + t="s" 单元格，而非 inlineStr；
// 现有 buildXLSX / buildQianJiXLSXFull 走 inlineStr，未覆盖共享字符串读取路径。
// 这里按真实文件结构构造，用于端到端校验 readXLSXBytes 的 sharedStrings 分支。
func buildQianJiSharedXLSX(t *testing.T, rows [][]string) []byte {
	t.Helper()
	idx := map[string]int{}
	var shared []string
	for _, row := range rows {
		for _, cell := range row {
			if _, ok := idx[cell]; !ok {
				idx[cell] = len(shared)
				shared = append(shared, cell)
			}
		}
	}
	var ssb strings.Builder
	ssb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	ssb.WriteString(`<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="` +
		strconv.Itoa(len(shared)) + `" uniqueCount="` + strconv.Itoa(len(shared)) + `">`)
	for _, s := range shared {
		ssb.WriteString(`<si><t xml:space="preserve">` + xmlEscape(s) + `</t></si>`)
	}
	ssb.WriteString(`</sst>`)

	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	sb.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	for ri, row := range rows {
		sb.WriteString(fmt.Sprintf(`<row r="%d">`, ri+1))
		for ci, cell := range row {
			sb.WriteString(fmt.Sprintf(`<c r="%s%d" t="s"><v>%d</v></c>`, colLetter(ci), ri+1, idx[cell]))
		}
		sb.WriteString(`</row>`)
	}
	sb.WriteString(`</sheetData></worksheet>`)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	write := func(name, content string) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(content))
	}
	write("[Content_Types].xml", ctTypes)
	write("_rels/.rels", relsRoot)
	write("xl/workbook.xml", wbXML)
	write("xl/_rels/workbook.xml.rels", wbRels)
	write("xl/sharedStrings.xml", ssb.String())
	write("xl/worksheets/sheet1.xml", sb.String())
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestParseQianJi_RealExportShape 以真实钱迹导出格式（sharedStrings + 19 列表头）
// 端到端校验解析，覆盖此前测试缺失的取值：
//   - 类型「还款」→ 转账（真实导出高频类型）
//   - 手续费 / 优惠券 → TransferFee / TransferDiscount（此前优惠券无任何断言）
//   - 大整数金额（12345）与小数金额（13805.7）
//   - 无备注行、单标签行（真实导出常见）
//   - readXLSXBytes 的共享字符串读取分支（真实文件即此格式）
func TestParseQianJi_RealExportShape(t *testing.T) {
	uid, bookID := qjSetup(t)
	rows := [][]string{
		{"ID", "时间", "账本", "分类", "二级分类", "类型", "金额", "币种", "账户1", "账户2", "备注", "已报销", "手续费", "优惠券", "记账者", "账单标记", "标签", "账单图片", "关联账单"},
		// 还款：储蓄卡 -> 信用卡
		{"qj-r1", "2026-09-15 10:15:57", "日常账本", "其它", "", "还款", "180.59", "CNY", "交通银行储蓄卡", "交通银行万事达信用卡", "", "", "", "", "子翼", "", "", ""},
		// 还款：储蓄卡 -> 花呗，带优惠券 0.15
		{"qj-r2", "2026-09-15 10:14:44", "日常账本", "其它", "", "还款", "470.57", "CNY", "民生银行储蓄卡", "花呗", "", "", "", "0.15", "子翼", "", "", ""},
		// 转账：同时带手续费 1.7 与优惠券 0.15
		{"qj-t1", "2026-09-16 11:22:07", "日常账本", "其它", "", "转账", "8", "CNY", "招商银行信用卡", "市政府饭卡", "", "", "1.7", "0.15", "子翼", "", "", ""},
		// 支出：大整数金额 + 无备注 + 二级分类
		{"qj-e1", "2026-09-15 11:00:00", "日常账本", "住房", "房贷", "支出", "12345", "CNY", "房贷", "", "111", "", "", "", "子翼", "", "", ""},
		// 收入：小数金额 + 单标签
		{"qj-i1", "2026-09-15 15:11:29", "日常账本", "工资", "", "收入", "13805.7", "CNY", "民生银行储蓄卡", "", "", "", "", "", "子翼", "", "京东", ""},
	}
	data := buildQianJiSharedXLSX(t, rows)
	txs, err := parseQianJi(bytes.NewReader(data), uid, bookID)
	if err != nil {
		t.Fatalf("parseQianJi error: %v", err)
	}
	if len(txs) != 5 {
		t.Fatalf("want 5 txs, got %d", len(txs))
	}

	// 1) 还款 → 转账；优惠券写入 TransferDiscount
	if txs[0].Type != models.TxTransfer {
		t.Errorf("row0 type = %s, want transfer (还款)", txs[0].Type)
	}
	if txs[1].Type != models.TxTransfer {
		t.Errorf("row1 type = %s, want transfer (还款->花呗)", txs[1].Type)
	}
	if txs[1].TransferDiscount != models.FromYuan(0.15) {
		t.Errorf("row1 transfer discount = %v, want 0.15", txs[1].TransferDiscount)
	}
	var hb models.Account
	if err := database.DB.First(&hb, txs[1].ToAccountID).Error; err != nil {
		t.Fatalf("row1 to account not resolved: %v", err)
	}
	if hb.Type != models.AccLiability {
		t.Errorf("row1 to account type = %s, want liability (花呗)", hb.Type)
	}

	// 2) 转账：手续费 + 优惠券 同时落库
	if txs[2].TransferFee != models.FromYuan(1.7) {
		t.Errorf("row2 transfer fee = %v, want 1.7", txs[2].TransferFee)
	}
	if txs[2].TransferDiscount != models.FromYuan(0.15) {
		t.Errorf("row2 transfer discount = %v, want 0.15", txs[2].TransferDiscount)
	}

	// 3) 支出：大整数金额守恒，二级分类挂到一级「住房」下
	if txs[3].Type != models.TxExpense {
		t.Errorf("row3 type = %s, want expense", txs[3].Type)
	}
	if txs[3].Amount != models.FromYuan(12345) {
		t.Errorf("row3 amount = %v, want 12345", txs[3].Amount)
	}
	if txs[3].Description != "111" {
		t.Errorf("row3 description = %q, want 111", txs[3].Description)
	}
	var sub3 models.Category
	if err := database.DB.First(&sub3, txs[3].CategoryID).Error; err != nil {
		t.Fatalf("row3 category: %v", err)
	}
	if sub3.Name != "房贷" || sub3.ParentID == 0 {
		t.Errorf("row3 category = %s (parent=%d), want 房贷 under 住房", sub3.Name, sub3.ParentID)
	}
	// 真实导出里「房贷」账户既作支出账户、又作还款目标账户，必须识别为负债
	var mort models.Account
	if err := database.DB.First(&mort, txs[3].AccountID).Error; err != nil {
		t.Fatalf("row3 account: %v", err)
	}
	if mort.Name != "房贷" || mort.Type != models.AccLiability {
		t.Errorf("row3 account = %s/%s, want 房贷/liability", mort.Name, mort.Type)
	}

	// 4) 收入：小数金额守恒 + 单标签
	if txs[4].Type != models.TxIncome {
		t.Errorf("row4 type = %s, want income", txs[4].Type)
	}
	if txs[4].Amount != models.FromYuan(13805.7) {
		t.Errorf("row4 amount = %v, want 13805.7", txs[4].Amount)
	}
	if len(txs[4].Tags) != 1 || txs[4].Tags[0].Name != "京东" {
		t.Errorf("row4 tags = %v, want [京东]", txs[4].Tags)
	}
}

// guessAccountType 依据账户名猜测类型。各类贷款（房贷/车贷/××贷）必须归为负债
// —— 与 AccLiability 注释「花呗/借呗/贷款」一致；否则会被当成现金资产，
// 而负债账户在余额引擎里方向系数取反（debtSign = -1），误判会让还款/支出的
// 欠款增减方向算反。
func TestGuessAccountType(t *testing.T) {
	cases := []struct {
		name string
		want models.AccountType
	}{
		// 负债：互联网金融 + 各类贷款
		{"花呗", models.AccLiability},
		{"借呗", models.AccLiability},
		{"京东白条", models.AccLiability},
		{"房贷", models.AccLiability},
		{"车贷", models.AccLiability},
		{"消费贷款", models.AccLiability},
		{"助学贷", models.AccLiability},
		// 信用卡：必须先于负债的「贷」字命中
		{"信用卡", models.AccCredit},
		{"招行贷记卡", models.AccCredit},
		{"交通银行万事达信用卡", models.AccCredit},
		// 储蓄卡 / 银行
		{"招商银行储蓄卡", models.AccBank},
		{"交通银行储蓄卡", models.AccBank},
		{"民生银行储蓄卡", models.AccBank},
		// 储值卡
		{"市政府饭卡", models.AccPrepaid},
		// 虚拟账户
		{"支付宝", models.AccVirtual},
		{"微信零钱", models.AccVirtual},
		// 现金
		{"现金", models.AccCash},
	}
	for _, c := range cases {
		if got := guessAccountType(c.name); got != c.want {
			t.Errorf("guessAccountType(%q) = %s, want %s", c.name, got, c.want)
		}
	}

	// 房贷须为债务账户：余额方向取反，保证还款/支出时欠款正确增减
	mortgage := &models.Account{Type: guessAccountType("房贷")}
	if !isDebtAccount(mortgage) {
		t.Errorf("房贷应识别为债务账户")
	}
	if debtSign(mortgage) != -1 {
		t.Errorf("房贷 debtSign = %d, want -1", debtSign(mortgage))
	}
}
