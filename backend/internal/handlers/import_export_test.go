package handlers_test

import (
	"archive/zip"
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"huozhi/internal/database"
	"huozhi/internal/models"
)

func authMultipartReq(t *testing.T, method, path, token, fieldName, fileName, content string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile(fieldName, fileName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(method, path, &buf)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	return r
}

func TestExportTransactions(t *testing.T) {
	uid, tok, bookID := registerRealUser(t)
	cat := firstExpenseCat(t, uid, bookID)
	acc1, _ := twoAccounts(t, uid, bookID)
	postTx(t, tok, map[string]interface{}{"book_id": bookID, "type": "expense", "amount": 35.5, "category_id": cat, "account_id": acc1, "tx_date": "2026-01-15", "description": "午餐"})

	w := do(authReq("GET", "/api/io/export?book_id="+itoa(bookID), tok, nil))
	if w.Code != 200 {
		t.Fatalf("export %d %s", w.Code, w.Body.String())
	}
	if len(w.Body.Bytes()) == 0 {
		t.Fatalf("export body empty")
	}
	// without book_id
	w = do(authReq("GET", "/api/io/export", tok, nil))
	if w.Code != 200 {
		t.Fatalf("export all %d", w.Code)
	}
}

func TestDownloadImportTemplate(t *testing.T) {
	_, tok, _ := registerRealUser(t)
	w := do(authReq("GET", "/api/io/template", tok, nil))
	if w.Code != 200 {
		t.Fatalf("template %d %s", w.Code, w.Body.String())
	}
	if len(w.Body.Bytes()) == 0 {
		t.Fatalf("template empty")
	}
}

func TestImportTransactions(t *testing.T) {
	uid, tok, bookID := registerRealUser(t)
	csv := "交易日期,类型(expense/income),金额,分类名称,描述/备注\n" +
		"2026-01-15,expense,35.5,餐饮,午餐\n" +
		"2026-01-10,income,8000,工资,工资\n" +
		"bad-row\n"

	w := do(authMultipartReq(t, "POST",
		"/api/io/import?source=template&book_id="+itoa(bookID), tok,
		"file", "huozhi.csv", csv))
	if w.Code != 200 {
		t.Fatalf("import %d %s", w.Code, w.Body.String())
	}
	m := decode(t, w)
	data := m["data"].(map[string]interface{})
	created := int(data["created"].(float64))
	if created < 2 {
		t.Fatalf("expected >=2 created, got %v", data)
	}

	// missing file -> 400
	w = do(authReq("POST", "/api/io/import?source=template&book_id="+itoa(bookID), tok, nil))
	if w.Code != 400 {
		t.Fatalf("expected 400 no file got %d", w.Code)
	}

	// invalid source -> 400
	w = do(authMultipartReq(t, "POST",
		"/api/io/import?source=bogus&book_id="+itoa(bookID), tok,
		"file", "x.csv", csv))
	if w.Code != 400 {
		t.Fatalf("expected 400 bad source got %d %s", w.Code, w.Body.String())
	}

	_ = uid
}

// makeQianJiXLSX 构造一个极简钱迹格式 xlsx（仅含 sharedStrings + sheet1，解析器只读取这两部分）。
// 单行数据：2026-01-15 支出 35.5 餐饮 现金，标签「午饭；晚饭」（splitTags 拆成两个标签）。
func makeQianJiXLSX(t *testing.T) []byte {
	t.Helper()
	shared := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
<si><t>时间</t></si>
<si><t>类型</t></si>
<si><t>金额</t></si>
<si><t>分类</t></si>
<si><t>账户1</t></si>
<si><t>标签</t></si>
<si><t>账本</t></si>
<si><t>2026-01-15 12:00:00</t></si>
<si><t>支出</t></si>
<si><t>餐饮</t></si>
<si><t>现金</t></si>
<si><t>35.5</t></si>
<si><t>午饭；晚饭</t></si>
<si><t>日常账本</t></si>
</sst>`
	sheet := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
<sheetData>
<row>
<c r="A1" t="s"><v>0</v></c>
<c r="B1" t="s"><v>1</v></c>
<c r="C1" t="s"><v>2</v></c>
<c r="D1" t="s"><v>3</v></c>
<c r="E1" t="s"><v>4</v></c>
<c r="F1" t="s"><v>5</v></c>
<c r="G1" t="s"><v>6</v></c>
</row>
<row>
<c r="A2" t="s"><v>7</v></c>
<c r="B2" t="s"><v>8</v></c>
<c r="C2" t="s"><v>11</v></c>
<c r="D2" t="s"><v>9</v></c>
<c r="E2" t="s"><v>10</v></c>
<c r="F2" t="s"><v>12</v></c>
<c r="G2" t="s"><v>13</v></c>
</row>
</sheetData>
</worksheet>`

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w1, _ := zw.Create("xl/sharedStrings.xml")
	w1.Write([]byte(shared))
	w2, _ := zw.Create("xl/worksheets/sheet1.xml")
	w2.Write([]byte(sheet))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestImportQianJiTagsLinked 验证钱迹导入后：标签与交易建立关联，且 Tag.count 正确累加。
func TestImportQianJiTagsLinked(t *testing.T) {
	uid, tok, bookID := registerRealUser(t)
	xlsx := makeQianJiXLSX(t)
	w := do(authMultipartReq(t, "POST",
		"/api/io/import?source=qianji&book_id="+itoa(bookID), tok,
		"file", "qianji.xlsx", string(xlsx)))
	if w.Code != 200 {
		t.Fatalf("import %d %s", w.Code, w.Body.String())
	}
	m := decode(t, w)
	if int(m["data"].(map[string]interface{})["created"].(float64)) < 1 {
		t.Fatalf("expected >=1 created, got %v", m["data"])
	}

	// 标签应被创建
	for _, name := range []string{"午饭", "晚饭"} {
		var tag models.Tag
		if err := database.DB.Where("user_id = ? AND name = ?", uid, name).First(&tag).Error; err != nil {
			t.Fatalf("tag %s not found: %v", name, err)
		}
		if tag.Count != 1 {
			t.Errorf("tag %s count = %d, want 1", name, tag.Count)
		}
		// 关联应存在
		var joinCount int64
		database.DB.Table("transaction_tags").Where("tag_id = ?", tag.ID).Count(&joinCount)
		if joinCount != 1 {
			t.Errorf("transaction_tags for %s = %d, want 1", name, joinCount)
		}
	}
}

func TestGetBill(t *testing.T) {
	uid, tok, bookID := registerRealUser(t)
	cat := firstExpenseCat(t, uid, bookID)
	acc1, _ := twoAccounts(t, uid, bookID)
	postTx(t, tok, map[string]interface{}{"book_id": bookID, "type": "expense", "amount": 35.5, "category_id": cat, "account_id": acc1, "tx_date": "2026-01-15", "description": "午餐"})
	// ensure an account exists for asset snapshot section
	var cnt int64
	database.DB.Model(&models.Account{}).Where("user_id = ?", uid).Count(&cnt)
	if cnt == 0 {
		t.Fatalf("no accounts")
	}

	w := do(authReq("GET", "/api/io/bill?month=2026-01&book_id="+itoa(bookID), tok, nil))
	if w.Code != 200 {
		t.Fatalf("bill %d %s", w.Code, w.Body.String())
	}
	// default month (no param)
	w = do(authReq("GET", "/api/io/bill", tok, nil))
	if w.Code != 200 {
		t.Fatalf("bill default %d %s", w.Code, w.Body.String())
	}
	// bad month format -> 400
	w = do(authReq("GET", "/api/io/bill?month=not-a-month", tok, nil))
	if w.Code != 400 {
		t.Fatalf("expected 400 bad month got %d", w.Code)
	}
}

// ============ 完整 19 列钱迹 xlsx 构造（inline string，无需 sharedStrings） ============

const (
	iqCtTypes = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
		`<Default Extension="xml" ContentType="application/xml"/>` +
		`<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>` +
		`<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>` +
		`</Types>`
	iqRelsRoot = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>` +
		`</Relationships>`
	iqWbXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" ` +
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
		`<sheets><sheet name="账单" sheetId="1" r:id="rId1"/></sheets></workbook>`
	iqWbRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>` +
		`</Relationships>`
)

func iqColLetter(i int) string {
	s := ""
	i++
	for i > 0 {
		i--
		s = string(rune('A'+i%26)) + s
		i /= 26
	}
	return s
}

func iqXmlEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(s)
}

// buildQianJiXLSXFull 构造完整钱迹格式 xlsx（inline string），供导入级测试使用
func buildQianJiXLSXFull(t *testing.T, rows [][]string) []byte {
	t.Helper()
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	sb.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	for ri, row := range rows {
		sb.WriteString(fmt.Sprintf(`<row r="%d">`, ri+1))
		for ci, cell := range row {
			sb.WriteString(fmt.Sprintf(`<c r="%s%d" t="inlineStr"><is><t>%s</t></is></c>`, iqColLetter(ci), ri+1, iqXmlEscape(cell)))
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
	write("[Content_Types].xml", iqCtTypes)
	write("_rels/.rels", iqRelsRoot)
	write("xl/workbook.xml", iqWbXML)
	write("xl/_rels/workbook.xml.rels", iqWbRels)
	write("xl/worksheets/sheet1.xml", sb.String())
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestImportQianJiIdempotentAndRefundLink 验证：
//  1. 钱迹导入写入 ExternalID，且「关联账单」被解析为真实 RefundOfID 外键；
//  2. 再次导入同一文件时基于 ExternalID 幂等去重（created=0，不产生重复账单）。
func TestImportQianJiIdempotentAndRefundLink(t *testing.T) {
	uid, tok, bookID := registerRealUser(t)
	rows := [][]string{
		{"ID", "时间", "账本", "分类", "二级分类", "类型", "金额", "币种", "账户1", "账户2", "备注", "已报销", "手续费", "优惠券", "记账者", "账单标记", "标签", "账单图片", "关联账单"},
		{"ext-001", "2026-09-03 12:00:00", "日常账本", "餐饮", "", "支出", "120", "CNY", "现金", "", "午餐", "", "", "", "子翼", "", "", "", ""},
		{"ext-002", "2026-09-04 12:00:00", "日常账本", "餐饮", "", "退款", "50", "CNY", "现金", "", "退餐", "", "", "", "子翼", "", "", "", "ext-001"},
	}
	xlsx := buildQianJiXLSXFull(t, rows)
	w := do(authMultipartReq(t, "POST",
		"/api/io/import?source=qianji&book_id="+itoa(bookID), tok,
		"file", "qianji.xlsx", string(xlsx)))
	if w.Code != 200 {
		t.Fatalf("import %d %s", w.Code, w.Body.String())
	}
	m := decode(t, w)
	data := m["data"].(map[string]interface{})
	if int(data["created"].(float64)) != 2 {
		t.Fatalf("expected 2 created, got %v", data)
	}

	// ExternalID 应已写入，且退款的 RefundOfID 指向原始支出（按 ExternalID 反查内部 id）
	var exp, ref models.Transaction
	if err := database.DB.Where("user_id = ? AND external_id = ?", uid, "ext-001").First(&exp).Error; err != nil {
		t.Fatalf("expense by external_id: %v", err)
	}
	if err := database.DB.Where("user_id = ? AND external_id = ?", uid, "ext-002").First(&ref).Error; err != nil {
		t.Fatalf("refund by external_id: %v", err)
	}
	if ref.RefundOfID != exp.ID {
		t.Fatalf("refund.refund_of_id = %d, want %d (应指向原始支出)", ref.RefundOfID, exp.ID)
	}

	// 再次导入同一文件：应基于 ExternalID 幂等，created=0
	w2 := do(authMultipartReq(t, "POST",
		"/api/io/import?source=qianji&book_id="+itoa(bookID), tok,
		"file", "qianji.xlsx", string(xlsx)))
	if w2.Code != 200 {
		t.Fatalf("reimport %d %s", w2.Code, w2.Body.String())
	}
	m2 := decode(t, w2)
	data2 := m2["data"].(map[string]interface{})
	if int(data2["created"].(float64)) != 0 {
		t.Fatalf("expected 0 created on reimport (idempotent by external_id), got %v", data2)
	}
}
