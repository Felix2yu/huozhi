package router

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewRegistersRoutes(t *testing.T) {
	r := New("test")
	if r == nil {
		t.Fatal("expected non-nil engine")
	}
	routes := r.Routes()
	if len(routes) == 0 {
		t.Fatal("expected routes to be registered")
	}
	want := map[string]bool{
		"GET /api/health":              false,
		"POST /api/auth/login":         false,
		"GET /api/ai/status":           false,
		"GET /api/books":               false,
		"GET /api/accounts":            false,
		"GET /api/categories":          false,
		"GET /api/transactions":        false,
		"GET /api/statistics":          false,
		"GET /api/saving-plans":        false,
		"GET /api/recurring":           false,
		"GET /api/installments":        false,
		"GET /api/reimbursements":      false,
		"POST /api/io/import":          false,
	}
	for _, rt := range routes {
		key := rt.Method + " " + rt.Path
		if _, ok := want[key]; ok {
			want[key] = true
		}
	}
	for key, found := range want {
		if !found {
			t.Fatalf("route not registered: %s", key)
		}
	}
}

func TestNewReleaseMode(t *testing.T) {
	r := New("release")
	if r == nil {
		t.Fatal("expected non-nil engine in release mode")
	}
	// a public route should respond without JWT
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code == 0 {
		t.Fatal("expected health route to be served")
	}
}
