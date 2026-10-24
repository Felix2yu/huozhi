package handlers_test

import (
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"testing"

	"huozhi/internal/database"
	"huozhi/internal/middleware"
	"huozhi/internal/models"
)

// ===== 对外只读账单接口（API Key）=====
//
// 这一组同时服务幕间（单张）与牵丝（搜索 + 批量），所以既有的历史字段
// （amount 元、tx_date "YYYY-MM-DD HH:MM:SS"）必须保持不变，
// 新增字段（amount_fen / category / account / book）只能追加。

func pubSetAPIKey(t *testing.T, uid uint) string {
	t.Helper()
	key := middleware.GenerateAPIKey()
	if err := database.DB.Model(&models.User{}).Where("id = ?", uid).
		Updates(map[string]interface{}{"api_key": key, "api_key_enabled": true}).Error; err != nil {
		t.Fatalf("set api key: %v", err)
	}
	return key
}

func pubReq(method, path, key string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if key != "" {
		req.Header.Set("X-API-Key", key)
	}
	return do(req)
}

func pubItems(t *testing.T, w *httptest.ResponseRecorder) []map[string]interface{} {
	t.Helper()
	m := decode(t, w)
	data, ok := m["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("响应缺少 data: %s", w.Body.String())
	}
	// 分页响应把列表放在 list 里
	raw, _ := data["list"].([]interface{})
	out := make([]map[string]interface{}, 0, len(raw))
	for _, it := range raw {
		out = append(out, it.(map[string]interface{}))
	}
	return out
}

func pubID(n uint) string { return strconv.FormatUint(uint64(n), 10) }

func TestPublicBills(t *testing.T) {
	uid, tok, bookID := registerRealUser(t)
	cat := firstExpenseCat(t, uid, bookID)
	acc, _ := twoAccounts(t, uid, bookID)

	id1 := postTx(t, tok, map[string]interface{}{
		"book_id": bookID, "type": "expense", "amount": 88,
		"category_id": cat, "account_id": acc, "tx_date": "2026-10-20",
		"description": "茶馆",
	})
	id2 := postTx(t, tok, map[string]interface{}{
		"book_id": bookID, "type": "expense", "amount": 12.5,
		"category_id": cat, "account_id": acc, "tx_date": "2026-10-20",
		"description": "打车",
	})
	postTx(t, tok, map[string]interface{}{
		"book_id": bookID, "type": "income", "amount": 100,
		"category_id": 0, "account_id": acc, "tx_date": "2026-10-21",
		"description": "报销",
	})

	key := pubSetAPIKey(t, uid)

	t.Run("无密钥 401", func(t *testing.T) {
		if w := pubReq("GET", "/api/public/bills", ""); w.Code != 401 {
			t.Fatalf("期望 401，实际 %d", w.Code)
		}
	})

	t.Run("错误密钥 401", func(t *testing.T) {
		if w := pubReq("GET", "/api/public/bills", middleware.GenerateAPIKey()); w.Code != 401 {
			t.Fatalf("期望 401，实际 %d", w.Code)
		}
	})

	t.Run("单张兼容幕间字段", func(t *testing.T) {
		w := pubReq("GET", "/api/public/bills/"+pubID(id1), key)
		if w.Code != 200 {
			t.Fatalf("期望 200，实际 %d %s", w.Code, w.Body.String())
		}
		var m map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &m)
		b := m["data"].(map[string]interface{})
		// 历史字段：amount 是「元」、tx_date 带时分秒 —— 幕间按这两个口径解析
		if b["amount"].(float64) != 88 {
			t.Fatalf("amount 期望 88（元），实际 %v", b["amount"])
		}
		if b["tx_date"].(string) != "2026-10-20 00:00:00" {
			t.Fatalf("tx_date 格式变了: %v", b["tx_date"])
		}
		// 新增字段：分与字典名称
		if b["amount_fen"].(float64) != 8800 {
			t.Fatalf("amount_fen 期望 8800，实际 %v", b["amount_fen"])
		}
		for _, f := range []string{"category", "account", "book"} {
			if s, _ := b[f].(string); s == "" {
				t.Fatalf("%s 未解析出名称: %v", f, b)
			}
		}
	})

	t.Run("列表与类型筛选", func(t *testing.T) {
		items := pubItems(t, pubReq("GET", "/api/public/bills?page_size=10", key))
		if len(items) != 3 {
			t.Fatalf("期望 3 条，实际 %d", len(items))
		}
		onlyExpense := pubItems(t, pubReq("GET", "/api/public/bills?type=expense", key))
		if len(onlyExpense) != 2 {
			t.Fatalf("type=expense 期望 2 条，实际 %d", len(onlyExpense))
		}
	})

	t.Run("关键词筛选", func(t *testing.T) {
		items := pubItems(t, pubReq("GET", "/api/public/bills?keyword=打车", key))
		if len(items) != 1 || items[0]["description"] != "打车" {
			t.Fatalf("关键词筛选结果不对: %+v", items)
		}
	})

	t.Run("按 id 批量取", func(t *testing.T) {
		items := pubItems(t, pubReq("GET",
			"/api/public/bills?ids="+pubID(id1)+","+pubID(id2)+",999999", key))
		if len(items) != 2 {
			t.Fatalf("期望 2 条（不存在的忽略），实际 %d", len(items))
		}
	})

	t.Run("越权访问他人账单 404", func(t *testing.T) {
		registerRealUser(t)
		var other models.User
		database.DB.Where("username LIKE 'ru_%'").Order("id DESC").First(&other)
		otherKey := pubSetAPIKey(t, other.ID)
		if w := pubReq("GET", "/api/public/bills/"+pubID(id1), otherKey); w.Code != 404 {
			t.Fatalf("期望 404，实际 %d", w.Code)
		}
	})

	t.Run("禁用密钥后立即失效", func(t *testing.T) {
		database.DB.Model(&models.User{}).Where("id = ?", uid).Update("api_key_enabled", false)
		if w := pubReq("GET", "/api/public/bills", key); w.Code != 401 {
			t.Fatalf("期望 401，实际 %d", w.Code)
		}
	})
}
