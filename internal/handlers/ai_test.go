package handlers_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func mockLLM(t *testing.T, content string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprintf(w, `{"choices":[{"message":{"content":%q}}]}`, content)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestAIStatus(t *testing.T) {
	_, tok, _ := registerRealUser(t)
	// auto-detect: no key -> disabled
	t.Setenv("AI_API_KEY", "")
	t.Setenv("AI_ENABLED", "")
	w := do(authReq("GET", "/api/ai/status", tok, nil))
	if w.Code != 200 {
		t.Fatalf("status %d", w.Code)
	}
	// explicit enabled
	t.Setenv("AI_ENABLED", "true")
	t.Setenv("AI_API_KEY", "k")
	t.Setenv("AI_MODEL", "gpt-4o")
	w = do(authReq("GET", "/api/ai/status", tok, nil))
	if w.Code != 200 {
		t.Fatalf("status2 %d", w.Code)
	}
	// explicit disabled
	t.Setenv("AI_ENABLED", "false")
	w = do(authReq("GET", "/api/ai/status", tok, nil))
	if w.Code != 200 {
		t.Fatalf("status3 %d", w.Code)
	}
}

func TestAIClassifyDisabled(t *testing.T) {
	t.Setenv("AI_API_KEY", "")
	t.Setenv("AI_ENABLED", "false")
	_, tok, _ := registerRealUser(t)
	w := do(authReq("POST", "/api/ai/classify", tok, map[string]interface{}{
		"description": "午饭", "amount": 35,
	}))
	if int(decode(t, w)["code"].(float64)) != 503 {
		t.Fatalf("expected 503 got %v", decode(t, w)["code"])
	}
}

func TestAISmartRecordDisabled(t *testing.T) {
	t.Setenv("AI_API_KEY", "")
	t.Setenv("AI_ENABLED", "false")
	_, tok, _ := registerRealUser(t)
	w := do(authReq("POST", "/api/ai/smart-record", tok, map[string]interface{}{
		"text": "午饭35",
	}))
	if int(decode(t, w)["code"].(float64)) != 503 {
		t.Fatalf("expected 503 got %v", decode(t, w)["code"])
	}
}

func TestAIClassifySuccess(t *testing.T) {
	t.Setenv("AI_API_KEY", "k")
	t.Setenv("AI_ENABLED", "")
	srv := mockLLM(t, `{"category_index":1,"type":"expense","confidence":0.9,"explanation":"ok"}`)
	t.Setenv("AI_ENDPOINT", srv.URL)

	_, tok, bookID := registerRealUser(t)
	w := do(authReq("POST", "/api/ai/classify", tok, map[string]interface{}{
		"description": "午饭", "amount": 35, "book_id": bookID,
	}))
	if w.Code != 200 {
		t.Fatalf("classify %d %s", w.Code, w.Body.String())
	}
}

func TestAIClassifyInvalidIndex(t *testing.T) {
	t.Setenv("AI_API_KEY", "k")
	t.Setenv("AI_ENABLED", "")
	srv := mockLLM(t, `{"category_index":99,"type":"expense","confidence":0.9}`)
	t.Setenv("AI_ENDPOINT", srv.URL)

	_, tok, bookID := registerRealUser(t)
	w := do(authReq("POST", "/api/ai/classify", tok, map[string]interface{}{
		"description": "午饭", "amount": 35, "book_id": bookID,
	}))
	if int(decode(t, w)["code"].(float64)) != 500 {
		t.Fatalf("expected 500 got %v %s", decode(t, w)["code"], w.Body.String())
	}
}

func TestAISmartRecordSuccess(t *testing.T) {
	t.Setenv("AI_API_KEY", "k")
	t.Setenv("AI_ENABLED", "")
	srv := mockLLM(t, `{"description":"午餐","amount":35,"type":"expense","category_index":1,"account_index":1,"tx_date":"2026-01-15","tags":[]}`)
	t.Setenv("AI_ENDPOINT", srv.URL)

	_, tok, bookID := registerRealUser(t)
	w := do(authReq("POST", "/api/ai/smart-record", tok, map[string]interface{}{
		"text": "午饭35", "book_id": bookID,
	}))
	if w.Code != 200 {
		t.Fatalf("smart record %d %s", w.Code, w.Body.String())
	}
}

func TestAISmartRecordBadAmount(t *testing.T) {
	t.Setenv("AI_API_KEY", "k")
	t.Setenv("AI_ENABLED", "")
	srv := mockLLM(t, `{"description":"x","amount":-1,"type":"expense","category_index":1,"account_index":1,"tx_date":"2026-01-15","tags":[]}`)
	t.Setenv("AI_ENDPOINT", srv.URL)

	_, tok, bookID := registerRealUser(t)
	w := do(authReq("POST", "/api/ai/smart-record", tok, map[string]interface{}{
		"text": "x", "book_id": bookID,
	}))
	if int(decode(t, w)["code"].(float64)) != 500 {
		t.Fatalf("expected 500 got %v %s", decode(t, w)["code"], w.Body.String())
	}
}
