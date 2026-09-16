package handlers_test

import (
	"archive/zip"
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"
)

// 钱迹导出「账本」列恒有值，这里模拟真实场景
var repqBook = "日常账本"

// 复现：导入钱迹账单后，/categories 是否返回二级分类
func TestReproQianJiSubCategoryVisible(t *testing.T) {
	uid, tok, bookID := registerRealUser(t)
	t.Logf("uid=%d bookID=%d", uid, bookID)

	data := reproBuildXLSX([][]string{
		{"ID", "时间", "账本", "分类", "二级分类", "类型", "金额", "币种", "账户1", "账户2", "备注", "已报销", "手续费", "优惠券", "记账者", "账单标记", "标签", "账单图片", "关联账单"},
		{"r1", "2026-09-03 12:00:00", repqBook, "餐饮", "火锅", "支出", "120", "CNY", "招商银行储蓄卡", "", "聚餐", "", "", "", "", "", "", "", ""},
		{"r2", "2026-09-04 12:00:00", repqBook, "餐饮", "早餐", "支出", "15", "CNY", "招商银行储蓄卡", "", "", "", "", "", "", "", "", "", ""},
		{"r3", "2026-09-05 12:00:00", repqBook, "交通", "地铁", "支出", "6", "CNY", "支付宝", "", "", "", "", "", "", "", "", "", ""},
	})

	// 1) 导入
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("file", "qianji.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	fw.Write(data)
	mw.Close()

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/io/import?source=qianji&book_id=%d", bookID), &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+tok)
	w := do(req)
	t.Logf("IMPORT status=%d body=%s", w.Code, w.Body.String())

	// 2) 分类树
	w2 := do(authReq("GET", fmt.Sprintf("/api/categories?book_id=%d&kind=all", bookID), tok, nil))
	t.Logf("CATEGORIES status=%d body=%s", w2.Code, w2.Body.String())

	// 3) 流水
	w3 := do(authReq("GET", fmt.Sprintf("/api/transactions?book_id=%d&limit=10", bookID), tok, nil))
	s := w3.Body.String()
	if len(s) > 3000 {
		s = s[:3000]
	}
	t.Logf("TRANSACTIONS status=%d body=%s", w3.Code, s)
}

func reproBuildXLSX(rows [][]string) []byte {
	esc := func(s string) string {
		r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
		return r.Replace(s)
	}
	colLetter := func(i int) string {
		s := ""
		i++
		for i > 0 {
			i--
			s = string(rune('A'+i%26)) + s
			i /= 26
		}
		return s
	}
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	sb.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	for ri, row := range rows {
		sb.WriteString(fmt.Sprintf(`<row r="%d">`, ri+1))
		for ci, cell := range row {
			sb.WriteString(fmt.Sprintf(`<c r="%s%d" t="inlineStr"><is><t>%s</t></is></c>`, colLetter(ci), ri+1, esc(cell)))
		}
		sb.WriteString(`</row>`)
	}
	sb.WriteString(`</sheetData></worksheet>`)

	ct := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
		`<Default Extension="xml" ContentType="application/xml"/>` +
		`<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>` +
		`<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>` +
		`</Types>`
	rels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>` +
		`</Relationships>`
	wb := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" ` +
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
		`<sheets><sheet name="账单" sheetId="1" r:id="rId1"/></sheets></workbook>`
	wbRels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>` +
		`</Relationships>`

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	write := func(name, content string) {
		w, _ := zw.Create(name)
		w.Write([]byte(content))
	}
	write("[Content_Types].xml", ct)
	write("_rels/.rels", rels)
	write("xl/workbook.xml", wb)
	write("xl/_rels/workbook.xml.rels", wbRels)
	write("xl/worksheets/sheet1.xml", sb.String())
	zw.Close()
	return buf.Bytes()
}
