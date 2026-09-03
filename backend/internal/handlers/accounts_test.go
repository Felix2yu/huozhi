package handlers_test

import (
	"testing"

	"huozhi/internal/database"
	"huozhi/internal/models"
)

// getFirstAccount returns the first account id owned by uid in the book.
func getFirstAccount(t *testing.T, uid, bookID uint) uint {
	t.Helper()
	var a models.Account
	if err := database.DB.Where("user_id = ? AND book_id = ?", uid, bookID).First(&a).Error; err != nil {
		t.Fatalf("getFirstAccount: %v", err)
	}
	return a.ID
}

func TestListAccounts(t *testing.T) {
	_, tok, bookID := registerRealUser(t)
	cases := []string{
		"/api/accounts?book_id=" + itoa(bookID),
		"/api/accounts?book_id=" + itoa(bookID) + "&type=cash",
		"/api/accounts?book_id=" + itoa(bookID) + "&type=bank&include_archived=1",
		"/api/accounts?book_id=" + itoa(bookID) + "&show_hidden=1",
		"/api/accounts",
	}
	for _, path := range cases {
		w := do(authReq("GET", path, tok, nil))
		if w.Code != 200 {
			t.Fatalf("GET %s -> %d %s", path, w.Code, w.Body.String())
		}
	}
}

func TestGetAccount(t *testing.T) {
	_, tok, bookID := registerRealUser(t)
	// fetch the real first account id
	var a models.Account
	database.DB.Where("book_id = ?", bookID).First(&a)
	w := do(authReq("GET", "/api/accounts/"+itoa(a.ID), tok, nil))
	if w.Code != 200 {
		t.Fatalf("get account %d %s", w.Code, w.Body.String())
	}
	// not found
	w = do(authReq("GET", "/api/accounts/999999", tok, nil))
	if w.Code != 404 {
		t.Fatalf("expected 404 got %d", w.Code)
	}
}

func TestCreateAccount(t *testing.T) {
	uid, tok, bookID := registerRealUser(t)
	w := do(authReq("POST", "/api/accounts", tok, map[string]interface{}{
		"book_id":       bookID,
		"name":          "招商银行",
		"type":          "bank",
		"initial_amount": 1000,
		"full_card_no":  "6225880123456789",
		"card_no4":      "6789",
		"cvv":           "123",
		"bill_day":      5,
		"repay_day":     25,
		"currency":      "CNY",
	}))
	if w.Code != 201 {
		t.Fatalf("create %d %s", w.Code, w.Body.String())
	}
	m := decode(t, w)
	data := m["data"].(map[string]interface{})
	accID := uint(data["id"].(float64))

	// zero initial amount (no adjust tx path)
	w = do(authReq("POST", "/api/accounts", tok, map[string]interface{}{
		"book_id": bookID, "name": "现金2", "type": "cash", "initial_amount": 0,
	}))
	if w.Code != 201 {
		t.Fatalf("create2 %d %s", w.Code, w.Body.String())
	}

	// bad request (missing name / invalid type)
	w = do(authReq("POST", "/api/accounts", tok, map[string]interface{}{
		"book_id": bookID, "type": "bogus",
	}))
	if w.Code != 400 {
		t.Fatalf("expected 400 got %d %s", w.Code, w.Body.String())
	}

	// full card returns decrypted
	w = do(authReq("GET", "/api/accounts/"+itoa(accID)+"/full-card", tok, nil))
	if w.Code != 200 {
		t.Fatalf("full-card %d %s", w.Code, w.Body.String())
	}
	fc := decode(t, w)
	data = fc["data"].(map[string]interface{})
	if data["full_card_no"] != "6225880123456789" {
		t.Fatalf("card mismatch %v", data["full_card_no"])
	}
	if data["cvv"] != "123" {
		t.Fatalf("cvv mismatch %v", data["cvv"])
	}

	_ = uid
}

func TestUpdateAccount(t *testing.T) {
	_, tok, bookID := registerRealUser(t)
	var a models.Account
	database.DB.Where("book_id = ?", bookID).First(&a)
	w := do(authReq("PUT", "/api/accounts/"+itoa(a.ID), tok, map[string]interface{}{
		"name": "改名", "type": "cash", "full_card_no": "4111111111111111", "cvv": "999",
	}))
	if w.Code != 200 {
		t.Fatalf("update %d %s", w.Code, w.Body.String())
	}
	// bad body
	w = do(authReq("PUT", "/api/accounts/"+itoa(a.ID), tok, map[string]interface{}{
		"type": "weird",
	}))
	if w.Code != 400 {
		t.Fatalf("expected 400 got %d", w.Code)
	}
}

func TestDeleteAccount(t *testing.T) {
	_, tok, bookID := registerRealUser(t)
	var a models.Account
	database.DB.Where("book_id = ?", bookID).First(&a)
	w := do(authReq("DELETE", "/api/accounts/"+itoa(a.ID), tok, nil))
	if w.Code != 200 {
		t.Fatalf("delete %d %s", w.Code, w.Body.String())
	}
}

func TestAdjustAccountBalance(t *testing.T) {
	uid, tok, bookID := registerRealUser(t)
	var a models.Account
	database.DB.Where("book_id = ?", bookID).First(&a)

	// system 余额调整 category must exist (created at register)
	adj := seedAdjustCategory(t, uid, bookID)
	_ = adj

	w := do(authReq("POST", "/api/accounts/"+itoa(a.ID)+"/adjust", tok, map[string]interface{}{
		"amount": 500, "description": "调整", "date": "2026-01-15",
	}))
	if w.Code != 200 {
		t.Fatalf("adjust %d %s", w.Code, w.Body.String())
	}
	// bad body
	w = do(authReq("POST", "/api/accounts/"+itoa(a.ID)+"/adjust", tok, map[string]interface{}{}))
	if w.Code != 400 {
		t.Fatalf("expected 400 got %d", w.Code)
	}
}

func TestAccountGroups(t *testing.T) {
	_, tok, _ := registerRealUser(t)
	w := do(authReq("GET", "/api/accounts/groups", tok, nil))
	if w.Code != 200 {
		t.Fatalf("list groups %d", w.Code)
	}
	w = do(authReq("POST", "/api/accounts/groups", tok, map[string]interface{}{
		"name": "投资组", "sort": 1,
	}))
	if w.Code != 201 {
		t.Fatalf("create group %d %s", w.Code, w.Body.String())
	}
	m := decode(t, w)
	gid := uint(m["data"].(map[string]interface{})["id"].(float64))
	w = do(authReq("DELETE", "/api/accounts/groups/"+itoa(gid), tok, nil))
	if w.Code != 200 {
		t.Fatalf("delete group %d", w.Code)
	}
}

func TestGetCreditSummary(t *testing.T) {
	_, tok, bookID := registerRealUser(t)
	// create a credit card with repay day
	w := do(authReq("POST", "/api/accounts", tok, map[string]interface{}{
		"book_id": bookID, "name": "信用卡", "type": "credit",
		"credit_limit": 20000, "bill_day": 5, "repay_day": 25,
	}))
	if w.Code != 201 {
		t.Fatalf("create credit %d %s", w.Code, w.Body.String())
	}
	w = do(authReq("GET", "/api/accounts/credit-summary", tok, nil))
	if w.Code != 200 {
		t.Fatalf("credit summary %d %s", w.Code, w.Body.String())
	}
}
