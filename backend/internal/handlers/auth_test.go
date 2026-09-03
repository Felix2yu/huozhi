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
