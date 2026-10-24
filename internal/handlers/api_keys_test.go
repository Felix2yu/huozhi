package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"huozhi/internal/database"
	"huozhi/internal/middleware"
	"huozhi/internal/models"
)

// ===== 密钥管理（多把 / 带 scope）=====
//
// 这套接口是「MCP 能写、牵丝/幕间只读」得以成立的前提：
// 只有一把密钥时权限只能取并集，只读消费方必然连带拿到写权限。

func TestAPIKeysCRUD(t *testing.T) {
	uid, tok, _ := registerRealUser(t)

	// 新建时缺省为只读：接只取数的应用是新建密钥最常见的目的
	var readID uint
	t.Run("新建缺省只读", func(t *testing.T) {
		w := do(authReq("POST", "/api/api-keys", tok, map[string]string{"name": "牵丝"}))
		if w.Code != 200 {
			t.Fatalf("期望 200，实际 %d %s", w.Code, w.Body.String())
		}
		bag := decode(t, w)["data"].(map[string]interface{})
		if bag["scope"] != models.ScopeRead {
			t.Fatalf("缺省 scope 应为 read，实际 %v", bag["scope"])
		}
		if bag["enabled"] != true {
			t.Fatalf("新建密钥应默认启用，实际 %v", bag["enabled"])
		}
		key, _ := bag["key"].(string)
		if !middleware.ValidateAPIKey(key) {
			t.Fatalf("生成的密钥格式不对: %q", key)
		}
		readID = uint(bag["id"].(float64))

		// 只读密钥能读公开账单接口
		if w := pubReq("GET", "/api/public/bills", key); w.Code != 200 {
			t.Fatalf("只读密钥读账单期望 200，实际 %d", w.Code)
		}
	})

	var writeKey string
	t.Run("新建读写", func(t *testing.T) {
		w := do(authReq("POST", "/api/api-keys", tok, map[string]string{"name": "Claude", "scope": "write"}))
		bag := decode(t, w)["data"].(map[string]interface{})
		if bag["scope"] != models.ScopeWrite {
			t.Fatalf("scope 应为 write，实际 %v", bag["scope"])
		}
		writeKey = bag["key"].(string)
	})

	t.Run("非法 scope 拒绝", func(t *testing.T) {
		w := do(authReq("POST", "/api/api-keys", tok, map[string]string{"name": "x", "scope": "admin"}))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("期望 400，实际 %d", w.Code)
		}
	})

	t.Run("列表按创建顺序", func(t *testing.T) {
		w := do(authReq("GET", "/api/api-keys", tok, nil))
		rows := decode(t, w)["data"].([]interface{})
		if len(rows) != 2 {
			t.Fatalf("期望 2 把密钥，实际 %d", len(rows))
		}
		first := rows[0].(map[string]interface{})
		if first["name"] != "牵丝" || first["scope"] != models.ScopeRead {
			t.Fatalf("首条不对: %v", first)
		}
	})

	t.Run("改名与改权限", func(t *testing.T) {
		w := do(authReq("PATCH", "/api/api-keys/"+pubID(readID), tok,
			map[string]interface{}{"name": "牵丝（正式）", "scope": "write"}))
		if w.Code != 200 {
			t.Fatalf("期望 200，实际 %d %s", w.Code, w.Body.String())
		}
		bag := decode(t, w)["data"].(map[string]interface{})
		if bag["name"] != "牵丝（正式）" || bag["scope"] != models.ScopeWrite {
			t.Fatalf("更新未生效: %v", bag)
		}
		// 改回只读，后续用例依赖
		do(authReq("PATCH", "/api/api-keys/"+pubID(readID), tok, map[string]interface{}{"scope": "read"}))
	})

	t.Run("停用即失效", func(t *testing.T) {
		do(authReq("PATCH", "/api/api-keys/"+pubID(readID), tok, map[string]interface{}{"enabled": false}))
		if w := pubReq("GET", "/api/public/bills", writeKey); w.Code != 200 {
			t.Fatalf("另一把密钥不应受影响，实际 %d", w.Code)
		}
		var row models.APIKey
		database.DB.First(&row, readID)
		if w := pubReq("GET", "/api/public/bills", row.Key); w.Code != 401 {
			t.Fatalf("停用后期望 401，实际 %d", w.Code)
		}
		do(authReq("PATCH", "/api/api-keys/"+pubID(readID), tok, map[string]interface{}{"enabled": true}))
		if w := pubReq("GET", "/api/public/bills", row.Key); w.Code != 200 {
			t.Fatalf("恢复后期望 200，实际 %d", w.Code)
		}
	})

	t.Run("吊销后列表与鉴权同时失效", func(t *testing.T) {
		var row models.APIKey
		database.DB.First(&row, readID)
		w := do(authReq("DELETE", "/api/api-keys/"+pubID(readID), tok, nil))
		if w.Code != 200 {
			t.Fatalf("期望 200，实际 %d", w.Code)
		}
		if w := pubReq("GET", "/api/public/bills", row.Key); w.Code != 401 {
			t.Fatalf("吊销后期望 401，实际 %d", w.Code)
		}
		list := decode(t, do(authReq("GET", "/api/api-keys", tok, nil)))["data"].([]interface{})
		if len(list) != 1 {
			t.Fatalf("吊销后应只剩 1 把，实际 %d", len(list))
		}
		// 再删同一把：已不存在
		if w := do(authReq("DELETE", "/api/api-keys/"+pubID(readID), tok, nil)); w.Code != 404 {
			t.Fatalf("重复吊销期望 404，实际 %d", w.Code)
		}
	})

	t.Run("不能操作他人密钥", func(t *testing.T) {
		_, otherTok, _ := registerRealUser(t)
		w := do(authReq("POST", "/api/api-keys", otherTok, map[string]string{"name": "别人的"}))
		otherID := uint(decode(t, w)["data"].(map[string]interface{})["id"].(float64))

		if w := do(authReq("PATCH", "/api/api-keys/"+pubID(otherID), tok,
			map[string]interface{}{"name": "篡改"})); w.Code != 404 {
			t.Fatalf("跨账号改名期望 404，实际 %d", w.Code)
		}
		if w := do(authReq("DELETE", "/api/api-keys/"+pubID(otherID), tok, nil)); w.Code != 404 {
			t.Fatalf("跨账号吊销期望 404，实际 %d", w.Code)
		}
		var row models.APIKey
		if err := database.DB.First(&row, otherID).Error; err != nil {
			t.Fatalf("他人密钥不应被删: %v", err)
		}
	})

	_ = uid
}

// MCP 分权的链路验证：中间件解析 scope → 传输层注入 ctx → 工具分发拦截。
// 单测 mcp 包时是直接注入 scope 的，这一条走完整 HTTP 路径，
// 专门防止「忘了把 scope 从 gin 上下文带进 ctx」这种接线漏掉。
func TestMCPWriteScopeByAPIKey(t *testing.T) {
	_, tok, _ := registerRealUser(t)

	readKey := createKey(t, tok, "只读客户端", models.ScopeRead)
	writeKey := createKey(t, tok, "AI 客户端", models.ScopeWrite)

	mcpTools := func(key string) map[string]bool {
		body := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
		req := httptest.NewRequest("POST", "/api/mcp", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", key)
		w := do(req)
		if w.Code != 200 {
			t.Fatalf("tools/list 期望 200，实际 %d %s", w.Code, w.Body.String())
		}
		var resp struct {
			Result struct {
				Tools []struct {
					Name string `json:"name"`
				} `json:"tools"`
			} `json:"result"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("解析 MCP 响应失败: %v body=%s", err, w.Body.String())
		}
		out := map[string]bool{}
		for _, tl := range resp.Result.Tools {
			out[tl.Name] = true
		}
		return out
	}

	readTools := mcpTools(readKey)
	writeTools := mcpTools(writeKey)

	if readTools["delete_transaction"] || readTools["create_transaction"] {
		t.Fatal("只读密钥的 tools/list 不应出现写工具")
	}
	if !readTools["search_transactions"] {
		t.Fatal("只读密钥应能用读工具")
	}
	if !writeTools["delete_transaction"] || !writeTools["create_transaction"] {
		t.Fatal("读写密钥的 tools/list 应包含写工具（MCP 的写能力不能被砍掉）")
	}

	// 只读密钥硬调写工具：返回工具级错误，而不是执行
	body := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"delete_transaction","arguments":{"id":1}}}`
	req := httptest.NewRequest("POST", "/api/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", readKey)
	w := do(req)
	if !strings.Contains(w.Body.String(), "只读") {
		t.Fatalf("只读密钥调写工具应被拦下: %s", w.Body.String())
	}
}

// createKey 建一把指定 scope 的密钥并返回明文。
func createKey(t *testing.T, tok, name, scope string) string {
	t.Helper()
	w := do(authReq("POST", "/api/api-keys", tok, map[string]string{"name": name, "scope": scope}))
	if w.Code != 200 {
		t.Fatalf("创建密钥失败 %d %s", w.Code, w.Body.String())
	}
	bag := decode(t, w)["data"].(map[string]interface{})
	key, _ := bag["key"].(string)
	if key == "" {
		t.Fatalf("创建密钥未返回明文: %v", bag)
	}
	return key
}
// 迁移：users.api_key 搬进 api_keys，权限按读写；重复执行不产生第二把。
func TestMigrateAPIKeysLegacy(t *testing.T) {
	uid, _, _ := registerRealUser(t)
	key := middleware.GenerateAPIKey()
	if err := database.DB.Model(&models.User{}).Where("id = ?", uid).
		Updates(map[string]interface{}{"api_key": key, "api_key_enabled": true}).Error; err != nil {
		t.Fatalf("set legacy key: %v", err)
	}

	database.MigrateAPIKeys(database.DB)
	database.MigrateAPIKeys(database.DB) // 幂等

	var rows []models.APIKey
	if err := database.DB.Where("user_id = ? AND key = ?", uid, key).Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("历史密钥应迁移出 1 把，实际 %d", len(rows))
	}
	if rows[0].Scope != models.ScopeWrite || !rows[0].Legacy {
		t.Fatalf("历史密钥应为读写且标记 legacy: %+v", rows[0])
	}
	if w := pubReq("GET", "/api/public/bills", key); w.Code != 200 {
		t.Fatalf("迁移后历史密钥应仍可用，实际 %d", w.Code)
	}

	// 吊销后重启不能复活
	if err := database.DB.Delete(&models.APIKey{}, rows[0].ID).Error; err != nil {
		t.Fatal(err)
	}
	database.MigrateAPIKeys(database.DB)
	var alive int64
	database.DB.Model(&models.APIKey{}).Where("key = ?", key).Count(&alive)
	if alive != 0 {
		t.Fatal("已吊销的历史密钥不应被迁移逻辑复活")
	}
	if w := pubReq("GET", "/api/public/bills", key); w.Code != 401 {
		t.Fatalf("吊销后期望 401，实际 %d", w.Code)
	}
}
