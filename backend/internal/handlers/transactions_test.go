package handlers_test

import (
	"testing"

	"huozhi/internal/database"
	"huozhi/internal/models"
)

func firstExpenseCat(t *testing.T, uid, bookID uint) uint {
	t.Helper()
	var c models.Category
	if err := database.DB.Where("user_id = ? AND book_id = ? AND kind = ?", uid, bookID, models.KindExpense).First(&c).Error; err != nil {
		t.Fatalf("firstExpenseCat: %v", err)
	}
	return c.ID
}

func twoAccounts(t *testing.T, uid, bookID uint) (uint, uint) {
	t.Helper()
	var as []models.Account
	database.DB.Where("user_id = ? AND book_id = ?", uid, bookID).Order("id ASC").Find(&as)
	if len(as) < 2 {
		t.Fatalf("need 2 accounts, got %d", len(as))
	}
	return as[0].ID, as[1].ID
}

func postTx(t *testing.T, tok string, body map[string]interface{}) uint {
	t.Helper()
	w := do(authReq("POST", "/api/transactions", tok, body))
	if w.Code != 201 {
		t.Fatalf("create tx %d %s", w.Code, w.Body.String())
	}
	m := decode(t, w)
	return uint(m["data"].(map[string]interface{})["id"].(float64))
}

func TestCreateTransactions(t *testing.T) {
	uid, tok, bookID := registerRealUser(t)
	cat := firstExpenseCat(t, uid, bookID)
	acc1, acc2 := twoAccounts(t, uid, bookID)

	// expense
	postTx(t, tok, map[string]interface{}{
		"book_id": bookID, "type": "expense", "amount": 35.5,
		"category_id": cat, "account_id": acc1, "tx_date": "2026-01-15",
		"description": "午餐",
	})
	// income
	postTx(t, tok, map[string]interface{}{
		"book_id": bookID, "type": "income", "amount": 8000,
		"category_id": 0, "account_id": acc1, "tx_date": "2026-01-10",
	})
	// transfer (requires to_account_id)
	postTx(t, tok, map[string]interface{}{
		"book_id": bookID, "type": "transfer", "amount": 100,
		"account_id": acc1, "to_account_id": acc2, "tx_date": "2026-01-12",
	})
	// transfer without to_account_id -> 400
	w := do(authReq("POST", "/api/transactions", tok, map[string]interface{}{
		"book_id": bookID, "type": "transfer", "amount": 100, "account_id": acc1, "tx_date": "2026-01-12",
	}))
	if w.Code != 400 {
		t.Fatalf("expected 400 got %d", w.Code)
	}
	// bad body (missing amount) -> 400
	w = do(authReq("POST", "/api/transactions", tok, map[string]interface{}{
		"book_id": bookID, "type": "expense", "account_id": acc1, "tx_date": "2026-01-15",
	}))
	if w.Code != 400 {
		t.Fatalf("expected 400 got %d %s", w.Code, w.Body.String())
	}
	// with tags
	w = do(authReq("POST", "/api/tags", tok, map[string]interface{}{"book_id": bookID, "name": "商务"}))
	tagID := uint(decode(t, w)["data"].(map[string]interface{})["id"].(float64))
	postTx(t, tok, map[string]interface{}{
		"book_id": bookID, "type": "expense", "amount": 20, "category_id": cat,
		"account_id": acc1, "tx_date": "2026-01-16", "tag_ids": []uint{tagID},
	})
}

func TestGetTransaction(t *testing.T) {
	uid, tok, bookID := registerRealUser(t)
	cat := firstExpenseCat(t, uid, bookID)
	acc1, _ := twoAccounts(t, uid, bookID)
	id := postTx(t, tok, map[string]interface{}{
		"book_id": bookID, "type": "expense", "amount": 10, "category_id": cat,
		"account_id": acc1, "tx_date": "2026-01-15",
	})
	w := do(authReq("GET", "/api/transactions/"+itoa(id), tok, nil))
	if w.Code != 200 {
		t.Fatalf("get %d %s", w.Code, w.Body.String())
	}
	w = do(authReq("GET", "/api/transactions/999999", tok, nil))
	if w.Code != 404 {
		t.Fatalf("expected 404 got %d", w.Code)
	}
}

func TestListTransactions(t *testing.T) {
	uid, tok, bookID := registerRealUser(t)
	cat := firstExpenseCat(t, uid, bookID)
	acc1, acc2 := twoAccounts(t, uid, bookID)
	postTx(t, tok, map[string]interface{}{"book_id": bookID, "type": "expense", "amount": 35.5, "category_id": cat, "account_id": acc1, "tx_date": "2026-01-15", "description": "午餐 黄焖鸡", "merchant": "小店"})
	postTx(t, tok, map[string]interface{}{"book_id": bookID, "type": "income", "amount": 8000, "category_id": 0, "account_id": acc1, "tx_date": "2026-01-10"})
	postTx(t, tok, map[string]interface{}{"book_id": bookID, "type": "transfer", "amount": 100, "account_id": acc1, "to_account_id": acc2, "tx_date": "2026-01-12"})

	cases := []string{
		"/api/transactions?book_id=" + itoa(bookID),
		"/api/transactions?book_id=" + itoa(bookID) + "&type=expense",
		"/api/transactions?book_id=" + itoa(bookID) + "&start_date=2026-01-01&end_date=2026-01-31",
		"/api/transactions?book_id=" + itoa(bookID) + "&keyword=午餐",
		"/api/transactions?book_id=" + itoa(bookID) + "&min_amount=100&max_amount=9000",
		"/api/transactions?book_id=" + itoa(bookID) + "&account_id=" + itoa(acc1),
		"/api/transactions?page=1&page_size=2",
	}
	for _, p := range cases {
		w := do(authReq("GET", p, tok, nil))
		if w.Code != 200 {
			t.Fatalf("GET %s -> %d %s", p, w.Code, w.Body.String())
		}
	}
}

func TestUpdateTransaction(t *testing.T) {
	uid, tok, bookID := registerRealUser(t)
	cat := firstExpenseCat(t, uid, bookID)
	acc1, acc2 := twoAccounts(t, uid, bookID)
	id := postTx(t, tok, map[string]interface{}{"book_id": bookID, "type": "expense", "amount": 35.5, "category_id": cat, "account_id": acc1, "tx_date": "2026-01-15"})
	w := do(authReq("PUT", "/api/transactions/"+itoa(id), tok, map[string]interface{}{
		"book_id": bookID, "type": "expense", "amount": 50, "category_id": cat,
		"account_id": acc2, "tx_date": "2026-01-16",
	}))
	if w.Code != 200 {
		t.Fatalf("update %d %s", w.Code, w.Body.String())
	}
	// bad body
	w = do(authReq("PUT", "/api/transactions/"+itoa(id), tok, map[string]interface{}{"type": "expense"}))
	if w.Code != 400 {
		t.Fatalf("expected 400 got %d", w.Code)
	}
}

func TestDeleteTransaction(t *testing.T) {
	uid, tok, bookID := registerRealUser(t)
	cat := firstExpenseCat(t, uid, bookID)
	acc1, _ := twoAccounts(t, uid, bookID)
	id := postTx(t, tok, map[string]interface{}{"book_id": bookID, "type": "expense", "amount": 10, "category_id": cat, "account_id": acc1, "tx_date": "2026-01-15"})
	w := do(authReq("DELETE", "/api/transactions/"+itoa(id), tok, nil))
	if w.Code != 200 {
		t.Fatalf("delete %d %s", w.Code, w.Body.String())
	}
	w = do(authReq("DELETE", "/api/transactions/999999", tok, nil))
	if w.Code != 404 {
		t.Fatalf("expected 404 got %d", w.Code)
	}
}

func TestBatchDeleteTransactions(t *testing.T) {
	uid, tok, bookID := registerRealUser(t)
	cat := firstExpenseCat(t, uid, bookID)
	acc1, _ := twoAccounts(t, uid, bookID)
	id1 := postTx(t, tok, map[string]interface{}{"book_id": bookID, "type": "expense", "amount": 10, "category_id": cat, "account_id": acc1, "tx_date": "2026-01-15"})
	id2 := postTx(t, tok, map[string]interface{}{"book_id": bookID, "type": "expense", "amount": 20, "category_id": cat, "account_id": acc1, "tx_date": "2026-01-16"})
	w := do(authReq("POST", "/api/transactions/batch-delete", tok, map[string]interface{}{
		"ids": []uint{id1, id2},
	}))
	if w.Code != 200 {
		t.Fatalf("batch delete %d %s", w.Code, w.Body.String())
	}
	m := decode(t, w)
	if int(m["data"].(map[string]interface{})["deleted_count"].(float64)) != 2 {
		t.Fatalf("expected 2 deleted, got %v", m["data"].(map[string]interface{})["deleted_count"])
	}
}

func TestBudgets(t *testing.T) {
	_, tok, bookID := registerRealUser(t)
	w := do(authReq("GET", "/api/budgets?book_id="+itoa(bookID), tok, nil))
	if w.Code != 200 {
		t.Fatalf("list budgets %d", w.Code)
	}
	w = do(authReq("POST", "/api/budgets", tok, map[string]interface{}{
		"book_id": bookID, "period_type": "monthly", "amount": 3000,
		"start_date": "2026-01-01", "end_date": "2026-01-31", "alert_rate": 0.9,
	}))
	if w.Code != 201 {
		t.Fatalf("create budget %d %s", w.Code, w.Body.String())
	}
	id := uint(decode(t, w)["data"].(map[string]interface{})["id"].(float64))
	w = do(authReq("PUT", "/api/budgets/"+itoa(id), tok, map[string]interface{}{
		"period_type": "monthly", "amount": 4000, "start_date": "2026-01-01T00:00:00Z", "end_date": "2026-01-31T00:00:00Z",
	}))
	if w.Code != 200 {
		t.Fatalf("update budget %d %s", w.Code, w.Body.String())
	}
	w = do(authReq("DELETE", "/api/budgets/"+itoa(id), tok, nil))
	if w.Code != 200 {
		t.Fatalf("delete budget %d", w.Code)
	}
}

func TestSavingPlans(t *testing.T) {
	_, tok, bookID := registerRealUser(t)
	w := do(authReq("GET", "/api/saving-plans", tok, nil))
	if w.Code != 200 {
		t.Fatalf("list saving plans %d", w.Code)
	}
	w = do(authReq("POST", "/api/saving-plans", tok, map[string]interface{}{
		"book_id": bookID, "name": "买房", "target_amount": 500000,
		"start_date": "2026-01-01", "target_date": "2030-01-01",
	}))
	if w.Code != 201 {
		t.Fatalf("create saving plan %d %s", w.Code, w.Body.String())
	}
	id := uint(decode(t, w)["data"].(map[string]interface{})["id"].(float64))
	w = do(authReq("PUT", "/api/saving-plans/"+itoa(id), tok, map[string]interface{}{
		"name": "换房", "target_amount": 600000,
	}))
	if w.Code != 200 {
		t.Fatalf("update saving plan %d %s", w.Code, w.Body.String())
	}
	w = do(authReq("POST", "/api/saving-plans/"+itoa(id)+"/records", tok, map[string]interface{}{
		"amount": 1000, "record_date": "2026-01-15",
	}))
	if w.Code != 201 {
		t.Fatalf("add saving record %d %s", w.Code, w.Body.String())
	}
	w = do(authReq("DELETE", "/api/saving-plans/"+itoa(id), tok, nil))
	if w.Code != 200 {
		t.Fatalf("delete saving plan %d", w.Code)
	}
}

func TestRecurrings(t *testing.T) {
	uid, tok, bookID := registerRealUser(t)
	cat := firstExpenseCat(t, uid, bookID)
	acc1, _ := twoAccounts(t, uid, bookID)
	w := do(authReq("GET", "/api/recurring", tok, nil))
	if w.Code != 200 {
		t.Fatalf("list recurring %d", w.Code)
	}
	w = do(authReq("POST", "/api/recurring", tok, map[string]interface{}{
		"book_id": bookID, "name": "房租", "type": "expense", "amount": 3000,
		"category_id": cat, "account_id": acc1, "recurring_type": "monthly",
		"month_day": 1, "start_date": "2026-01-01",
	}))
	if w.Code != 201 {
		t.Fatalf("create recurring %d %s", w.Code, w.Body.String())
	}
	id := uint(decode(t, w)["data"].(map[string]interface{})["id"].(float64))
	w = do(authReq("POST", "/api/recurring/"+itoa(id)+"/toggle", tok, nil))
	if w.Code != 200 {
		t.Fatalf("toggle recurring %d %s", w.Code, w.Body.String())
	}
	w = do(authReq("DELETE", "/api/recurring/"+itoa(id), tok, nil))
	if w.Code != 200 {
		t.Fatalf("delete recurring %d", w.Code)
	}
}

func TestInstallments(t *testing.T) {
	uid, tok, bookID := registerRealUser(t)
	cat := firstExpenseCat(t, uid, bookID)
	acc1, _ := twoAccounts(t, uid, bookID)
	w := do(authReq("GET", "/api/installments", tok, nil))
	if w.Code != 200 {
		t.Fatalf("list installments %d", w.Code)
	}
	w = do(authReq("POST", "/api/installments", tok, map[string]interface{}{
		"book_id": bookID, "name": "手机分期", "total_amount": 6000, "total_months": 12,
		"category_id": cat, "account_id": acc1, "first_repay_date": "2026-01-05",
	}))
	if w.Code != 201 {
		t.Fatalf("create installment %d %s", w.Code, w.Body.String())
	}
	id := uint(decode(t, w)["data"].(map[string]interface{})["id"].(float64))
	w = do(authReq("DELETE", "/api/installments/"+itoa(id), tok, nil))
	if w.Code != 200 {
		t.Fatalf("delete installment %d", w.Code)
	}
}

func TestReimbursements(t *testing.T) {
	uid, tok, bookID := registerRealUser(t)
	cat := firstExpenseCat(t, uid, bookID)
	acc1, _ := twoAccounts(t, uid, bookID)
	txID := postTx(t, tok, map[string]interface{}{"book_id": bookID, "type": "expense", "amount": 200, "category_id": cat, "account_id": acc1, "tx_date": "2026-01-15"})

	w := do(authReq("GET", "/api/reimbursements", tok, nil))
	if w.Code != 200 {
		t.Fatalf("list reimbursements %d", w.Code)
	}
	w = do(authReq("POST", "/api/reimbursements", tok, map[string]interface{}{
		"book_id": bookID, "name": "出差报销", "total_amount": 200, "transaction_ids": []uint{txID},
	}))
	if w.Code != 201 {
		t.Fatalf("create reimbursement %d %s", w.Code, w.Body.String())
	}
	id := uint(decode(t, w)["data"].(map[string]interface{})["id"].(float64))
	w = do(authReq("PUT", "/api/reimbursements/"+itoa(id), tok, map[string]interface{}{
		"status": "received", "received_amount": 200,
	}))
	if w.Code != 200 {
		t.Fatalf("update reimbursement %d %s", w.Code, w.Body.String())
	}
	w = do(authReq("DELETE", "/api/reimbursements/"+itoa(id), tok, nil))
	if w.Code != 200 {
		t.Fatalf("delete reimbursement %d", w.Code)
	}
}
