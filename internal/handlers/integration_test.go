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

// setAPIKey 给用户生成并启用 API Key，返回明文。
// 走 middleware.GenerateAPIKey 而不是写死字符串，保证与生产同一格式（64 位十六进制）。
func setAPIKey(t *testing.T, uid uint) string {
	t.Helper()
	key := middleware.GenerateAPIKey()
	if err := database.DB.Model(&models.User{}).Where("id = ?", uid).
		Updates(map[string]interface{}{"api_key": key, "api_key_enabled": true}).Error; err != nil {
		t.Fatalf("set api key: %v", err)
	}
	return key
}

func apiKeyReq(method, path, key string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if key != "" {
		req.Header.Set("X-API-Key", key)
	}
	return do(req)
}

func integrationItems(t *testing.T, w *httptest.ResponseRecorder) []map[string]interface{} {
	t.Helper()
	m := decode(t, w)
	data, ok := m["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("响应缺少 data: %s", w.Body.String())
	}
	raw, _ := data["items"].([]interface{})
	out := make([]map[string]interface{}, 0, len(raw))
	for _, it := range raw {
		out = append(out, it.(map[string]interface{}))
	}
	return out
}

func TestIntegrationBillsReadOnly(t *testing.T) {
	uid, tok, bookID := registerRealUser(t)
	cat := firstExpenseCat(t, uid, bookID)
	acc1, _ := twoAccounts(t, uid, bookID)

	id1 := postTx(t, tok, map[string]interface{}{
		"book_id": bookID, "type": "expense", "amount": 88,
		"category_id": cat, "account_id": acc1, "tx_date": "2026-10-20",
		"description": "茶馆",
	})
	id2 := postTx(t, tok, map[string]interface{}{
		"book_id": bookID, "type": "expense", "amount": 12.5,
		"category_id": cat, "account_id": acc1, "tx_date": "2026-10-20",
		"description": "打车",
	})

	key := setAPIKey(t, uid)

	t.Run("无密钥 401", func(t *testing.T) {
		w := apiKeyReq("GET", "/api/integration/bills", "")
		if w.Code != 401 {
			t.Fatalf("期望 401，实际 %d %s", w.Code, w.Body.String())
		}
	})

	t.Run("错误密钥 401", func(t *testing.T) {
		w := apiKeyReq("GET", "/api/integration/bills", middleware.GenerateAPIKey())
		if w.Code != 401 {
			t.Fatalf("期望 401，实际 %d", w.Code)
		}
	})

	t.Run("列表带名称且金额为分", func(t *testing.T) {
		w := apiKeyReq("GET", "/api/integration/bills?page_size=10", key)
		if w.Code != 200 {
			t.Fatalf("期望 200，实际 %d %s", w.Code, w.Body.String())
		}
		items := integrationItems(t, w)
		if len(items) != 2 {
			t.Fatalf("期望 2 条，实际 %d", len(items))
		}
		var cafe map[string]interface{}
		for _, it := range items {
			if it["description"] == "茶馆" {
				cafe = it
			}
		}
		if cafe == nil {
			t.Fatal("没找到「茶馆」这条")
		}
		// 88 元必须序列化成 8800 分
		if got := cafe["amount_fen"].(float64); got != 8800 {
			t.Fatalf("amount_fen 期望 8800，实际 %v", got)
		}
		if cafe["tx_date"] != "2026-10-20" {
			t.Fatalf("tx_date 期望 2026-10-20，实际 %v", cafe["tx_date"])
		}
		for _, f := range []string{"category_name", "account_name", "book_name"} {
			if s, _ := cafe[f].(string); s == "" {
				t.Fatalf("%s 未解析出名称: %v", f, cafe)
			}
		}
	})

	t.Run("关键词筛选", func(t *testing.T) {
		w := apiKeyReq("GET", "/api/integration/bills?keyword=打车", key)
		items := integrationItems(t, w)
		if len(items) != 1 || items[0]["description"] != "打车" {
			t.Fatalf("关键词筛选结果不对: %+v", items)
		}
	})

	t.Run("按 id 批量取", func(t *testing.T) {
		w := apiKeyReq("GET", "/api/integration/bills?ids="+
			idParam(id1)+","+idParam(id2)+",999999", key)
		items := integrationItems(t, w)
		if len(items) != 2 {
			t.Fatalf("期望 2 条（不存在的 999999 忽略），实际 %d", len(items))
		}
	})

	t.Run("单条详情", func(t *testing.T) {
		w := apiKeyReq("GET", "/api/integration/bills/"+idParam(id1), key)
		if w.Code != 200 {
			t.Fatalf("期望 200，实际 %d %s", w.Code, w.Body.String())
		}
		var m map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &m)
		data := m["data"].(map[string]interface{})
		if uint(data["id"].(float64)) != id1 {
			t.Fatalf("返回了别的账单: %v", data)
		}
	})

	t.Run("越权访问他人账单 404", func(t *testing.T) {
		_, _, otherBook := registerRealUser(t)
		_ = otherBook
		// 用另一个用户的 key 访问第一个用户的账单
		var other models.User
		database.DB.Where("username LIKE 'ru_%'").Order("id DESC").First(&other)
		otherKey := setAPIKey(t, other.ID)
		w := apiKeyReq("GET", "/api/integration/bills/"+idParam(id1), otherKey)
		if w.Code != 404 {
			t.Fatalf("期望 404，实际 %d %s", w.Code, w.Body.String())
		}
	})

	t.Run("禁用密钥后立即失效", func(t *testing.T) {
		database.DB.Model(&models.User{}).Where("id = ?", uid).
			Update("api_key_enabled", false)
		w := apiKeyReq("GET", "/api/integration/bills", key)
		if w.Code != 401 {
			t.Fatalf("期望 401，实际 %d", w.Code)
		}
	})
}

func idParam(n uint) string {
	return strconv.FormatUint(uint64(n), 10)
}
