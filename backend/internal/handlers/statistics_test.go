package handlers_test

import (
	"testing"
	"time"

	"huozhi/internal/database"
	"huozhi/internal/models"
)

func TestGetStatistics(t *testing.T) {
	uid, tok, bookID := registerRealUser(t)
	cat := firstExpenseCat(t, uid, bookID)
	acc1, _ := twoAccounts(t, uid, bookID)

	postTx(t, tok, map[string]interface{}{"book_id": bookID, "type": "expense", "amount": 35.5, "category_id": cat, "account_id": acc1, "tx_date": "2026-01-15", "description": "午餐"})
	postTx(t, tok, map[string]interface{}{"book_id": bookID, "type": "income", "amount": 8000, "category_id": 0, "account_id": acc1, "tx_date": "2026-01-10"})

	// seed an asset snapshot to exercise that branch
	database.DB.Create(&models.AssetSnapshot{
		UserID: uid, SnapDate: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC), TotalAsset: 100, TotalDebt: 10, NetAsset: 90,
	})

	dims := []string{"category", "account", "month", "week", "day", "all", ""}
	for _, dim := range dims {
		path := "/api/statistics?book_id=" + itoa(bookID) + "&start_date=2026-01-01&end_date=2026-01-31"
		if dim != "" {
			path += "&dimension=" + dim
		}
		w := do(authReq("GET", path, tok, nil))
		if w.Code != 200 {
			t.Fatalf("GET %s -> %d %s", path, w.Code, w.Body.String())
		}
	}
	// kind filter
	w := do(authReq("GET", "/api/statistics?book_id="+itoa(bookID)+"&start_date=2026-01-01&end_date=2026-01-31&kind=expense", tok, nil))
	if w.Code != 200 {
		t.Fatalf("kind filter %d %s", w.Code, w.Body.String())
	}
}

func TestGetAssetOverview(t *testing.T) {
	_, tok, _ := registerRealUser(t)
	w := do(authReq("GET", "/api/statistics/assets", tok, nil))
	if w.Code != 200 {
		t.Fatalf("asset overview %d %s", w.Code, w.Body.String())
	}
}

func TestGetAssetTimeline(t *testing.T) {
	uid, tok, _ := registerRealUser(t)
	// snapshot covering a month
	database.DB.Create(&models.AssetSnapshot{
		UserID: uid, SnapDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), TotalAsset: 200, TotalDebt: 20, NetAsset: 180,
	})
	for _, q := range []string{"", "?months=3", "?months=abc"} {
		w := do(authReq("GET", "/api/statistics/assets/timeline"+q, tok, nil))
		if w.Code != 200 {
			t.Fatalf("timeline %q -> %d %s", q, w.Code, w.Body.String())
		}
	}
}
