package handlers_test

import (
	"testing"
)

func TestHealthCheck(t *testing.T) {
	w := do(authReq("GET", "/api/health", "", nil))
	if w.Code != 200 {
		t.Fatalf("code %d", w.Code)
	}
}

func TestRegisterAndLogin(t *testing.T) {
	username := "reg_" + randomSuffix()
	w := do(authReq("POST", "/api/auth/register", "", map[string]string{
		"username": username, "password": "secret123", "nickname": "Reg",
		"email": username + "@example.com", "phone": username + "_phone",
	}))
	if w.Code != 201 {
		t.Fatalf("register code %d body=%s", w.Code, w.Body.String())
	}
	m := decode(t, w)
	data := m["data"].(map[string]interface{})
	if data["token"] == nil {
		t.Fatal("no token in register response")
	}

	w2 := do(authReq("POST", "/api/auth/login", "", map[string]string{
		"username": username, "password": "secret123",
	}))
	if w2.Code != 200 {
		t.Fatalf("login code %d", w2.Code)
	}

	w3 := do(authReq("POST", "/api/auth/login", "", map[string]string{
		"username": username, "password": "wrong",
	}))
	m3 := decode(t, w3)
	if int(m3["code"].(float64)) != 1004 {
		t.Fatalf("expected 1004, got %v", m3["code"])
	}
}

func TestRegisterDuplicate(t *testing.T) {
	username := "dup_" + randomSuffix()
	email := username + "@example.com"
	phone := username + "_phone"
	do(authReq("POST", "/api/auth/register", "", map[string]string{
		"username": username, "password": "secret123", "email": email, "phone": phone,
	}))
	w := do(authReq("POST", "/api/auth/register", "", map[string]string{
		"username": username, "password": "secret123", "email": email, "phone": phone,
	}))
	m := decode(t, w)
	if int(m["code"].(float64)) != 1001 {
		t.Fatalf("expected 1001, got %v", m["code"])
	}
}

func TestRegisterInvalid(t *testing.T) {
	w := do(authReq("POST", "/api/auth/register", "", map[string]string{
		"username": "a", "password": "123",
	}))
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestProtectedRequiresAuth(t *testing.T) {
	w := do(authReq("GET", "/api/auth/me", "", nil))
	if w.Code != 401 {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestGetMeUpdateMeChangePwd(t *testing.T) {
	_, tok := newUser(t)

	w := do(authReq("GET", "/api/auth/me", tok, nil))
	if w.Code != 200 {
		t.Fatalf("getme %d", w.Code)
	}

	w2 := do(authReq("PUT", "/api/auth/me", tok, map[string]string{
		"nickname": "New", "email": "a@b.com", "locale": "en", "currency": "USD",
	}))
	if w2.Code != 200 {
		t.Fatalf("updateme %d", w2.Code)
	}

	w3 := do(authReq("POST", "/api/auth/password", tok, map[string]string{
		"old_password": "x", "new_password": "newpass123",
	}))
	if w3.Code != 200 {
		t.Fatalf("changepwd %d", w3.Code)
	}

	w4 := do(authReq("POST", "/api/auth/password", tok, map[string]string{
		"old_password": "wrong", "new_password": "newpass123",
	}))
	m4 := decode(t, w4)
	if int(m4["code"].(float64)) != 1006 {
		t.Fatalf("expected 1006 got %v", m4["code"])
	}

	w5 := do(authReq("POST", "/api/auth/logout", tok, nil))
	if w5.Code != 200 {
		t.Fatalf("logout %d", w5.Code)
	}
}

func TestGetMeNotFound(t *testing.T) {
	tok, _ := generateTokenFor(999999)
	w := do(authReq("GET", "/api/auth/me", tok, nil))
	if w.Code != 404 {
		t.Fatalf("expected 404 got %d", w.Code)
	}
}

// ============ Bug #1 回归: 注册不再因 APIKey UNIQUE 约束失败 ============
func TestRegisterMultipleUsersWithEmptyAPIKey(t *testing.T) {
	// 历史 bug: APIKey 字段是 string + uniqueIndex，多个新用户都存 "" 会违反唯一约束
	// 修复: APIKey 改为 *string，空值存 NULL，SQLite 多个 NULL 不冲突
	for i := 0; i < 5; i++ {
		username := "apikey_safe_" + randomSuffix()
		w := do(authReq("POST", "/api/auth/register", "", map[string]string{
			"username": username, "password": "secret123", "email": username + "@example.com",
		}))
		if w.Code != 201 {
			t.Fatalf("iteration %d: register failed code=%d body=%s", i, w.Code, w.Body.String())
		}
	}
}

// ============ Bug #2 回归: APIKey 生成/查询/切换完整流程 ============
func TestGenerateAndToggleAPIKey(t *testing.T) {
	_, tok := newUser(t)

	// 初始状态: 无 API Key
	w0 := do(authReq("GET", "/api/api-key", tok, nil))
	if w0.Code != 200 {
		t.Fatalf("get apikey info code=%d body=%s", w0.Code, w0.Body.String())
	}
	data0 := decode(t, w0)["data"].(map[string]interface{})
	if data0["has_api_key"].(bool) != false {
		t.Fatalf("expected has_api_key=false initially, got %v", data0["has_api_key"])
	}
	if data0["api_key"].(string) != "" {
		t.Fatalf("expected empty api_key, got %v", data0["api_key"])
	}

	// 生成 API Key
	w1 := do(authReq("POST", "/api/api-key/generate", tok, nil))
	if w1.Code != 200 {
		t.Fatalf("generate apikey code=%d body=%s", w1.Code, w1.Body.String())
	}
	data1 := decode(t, w1)["data"].(map[string]interface{})
	apikey := data1["api_key"].(string)
	if len(apikey) != 64 {
		t.Fatalf("expected 64-char hex apikey, got len=%d", len(apikey))
	}

	// 生成后 has_api_key = true, api_key_enabled = true
	w2 := do(authReq("GET", "/api/api-key", tok, nil))
	data2 := decode(t, w2)["data"].(map[string]interface{})
	if data2["has_api_key"].(bool) != true {
		t.Fatal("expected has_api_key=true after generation")
	}
	if data2["api_key_enabled"].(bool) != true {
		t.Fatal("expected api_key_enabled=true after generation")
	}

	// 切换禁用
	w3 := do(authReq("POST", "/api/api-key/toggle", tok, nil))
	data3 := decode(t, w3)["data"].(map[string]interface{})
	if data3["api_key_enabled"].(bool) != false {
		t.Fatal("expected api_key_enabled=false after toggle")
	}

	// 再切换回来
	w4 := do(authReq("POST", "/api/api-key/toggle", tok, nil))
	data4 := decode(t, w4)["data"].(map[string]interface{})
	if data4["api_key_enabled"].(bool) != true {
		t.Fatal("expected api_key_enabled=true after second toggle")
	}

	// 覆盖场景: ToggleAPIKey 在无 APIKey 时会先生成一个
	_, tok2 := newUser(t)
	w5 := do(authReq("POST", "/api/api-key/toggle", tok2, nil))
	if w5.Code != 200 {
		t.Fatalf("toggle apikey when none exists code=%d body=%s", w5.Code, w5.Body.String())
	}
}

// ============ Bug #1 更严苛: 不同注册方式空字段组合都能过 ============
func TestRegisterWithNullAPIKeyCombinations(t *testing.T) {
	// 分别测试: 只填 username, username+email, username+phone, 全部填
	cases := []map[string]string{
		{"username": "c1_" + randomSuffix(), "password": "secret123"},
		{"username": "c2_" + randomSuffix(), "password": "secret123", "email": "c2_" + randomSuffix() + "@x.com"},
		{"username": "c3_" + randomSuffix(), "password": "secret123", "phone": "13800" + randomSuffix()},
		{"username": "c4_" + randomSuffix(), "password": "secret123", "email": "c4_" + randomSuffix() + "@x.com", "phone": "13900" + randomSuffix()},
	}
	for _, body := range cases {
		w := do(authReq("POST", "/api/auth/register", "", body))
		if w.Code != 201 {
			t.Fatalf("register %v failed code=%d body=%s", body, w.Code, w.Body.String())
		}
	}
}
