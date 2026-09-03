package handlers_test

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
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
