package handlers

import (
	"archive/zip"
	"bytes"
	"fmt"
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
		Email:        fmt.Sprintf("qj_%d@example.com", n),
		Phone:        fmt.Sprintf("qj_%d_phone", n),
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
	rows := qianjiSampleRows()
	txs, err := parseQianJiRows(rows, uid, bookID)
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
	if tx0.Amount != 26.9 {
		t.Errorf("row0 amount = %v, want 26.9", tx0.Amount)
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
	// 分类应匹配到已预置的「医疗」
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

	// 2) 退款
	tx1 := txs[1]
	if tx1.Type != models.TxRefund {
		t.Errorf("row1 type = %s, want refund", tx1.Type)
	}
	if tx1.Amount != 14 {
		t.Errorf("row1 amount = %v, want 14", tx1.Amount)
	}
	var c1 models.Category
	database.DB.First(&c1, tx1.CategoryID)
	if c1.Name != "交通" {
		t.Errorf("row1 category = %s, want 交通 (matchCategoryAny)", c1.Name)
	}

	// 3) 转账
	tx2 := txs[2]
	if tx2.Type != models.TxTransfer {
		t.Errorf("row2 type = %s, want transfer", tx2.Type)
	}
	if tx2.Amount != 8 {
		t.Errorf("row2 amount = %v, want 8", tx2.Amount)
	}
	if tx2.ToAccountID == 0 {
		t.Errorf("row2 ToAccountID not resolved")
	}
	if tx2.TransferFee != 2.0 {
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

func TestParseQianJi_EndToEndXLSX(t *testing.T) {
	uid, bookID := qjSetup(t)
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
	txs, err := parseQianJiRows(rows, uid, bookID)
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
	if tx.Amount != 500 {
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
