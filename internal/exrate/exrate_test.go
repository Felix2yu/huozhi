package exrate

import (
	"huozhi/internal/config"
	"net/http"
	"net/http/httptest"
	"testing"
)

// withFxConfig 在测试期间替换全局配置，返回还原函数
func withFxConfig(cfg config.FxConfig) func() {
	old := config.AppConfig
	config.AppConfig = &config.Config{Fx: cfg}
	return func() { config.AppConfig = old }
}

func TestFetchERAPIDirection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/CNY" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"result": "success",
			"base_code": "CNY",
			"time_last_update_utc": "Wed, 16 Sep 2026 00:02:31 +0000",
			"rates": {"CNY": 1, "USD": 0.14, "JPY": 21.5, "EUR": 0.12}
		}`))
	}))
	defer srv.Close()

	restore := withFxConfig(config.FxConfig{Provider: "er-api", Endpoint: srv.URL, TimeoutSeconds: 5})
	defer restore()

	snap, err := fetchWithFallback("CNY", []string{"CNY", "USD", "JPY", "EUR"})
	if err != nil {
		t.Fatalf("fetchWithFallback: %v", err)
	}
	if snap.Base != "CNY" || snap.Source != "er-api" {
		t.Fatalf("unexpected snapshot: %+v", snap)
	}
	// 口径：1 单位外币 = ? 单位基准币，即上游值的倒数
	if got := snap.Rates["USD"]; got < 7.14 || got > 7.15 {
		t.Errorf("USD rate = %v, want ~7.1429 (1/0.14)", got)
	}
	if got := snap.Rates["CNY"]; got != 1 {
		t.Errorf("base self rate = %v, want 1", got)
	}
	if _, ok := snap.Rates["EUR"]; !ok {
		t.Errorf("EUR missing from snapshot: %+v", snap.Rates)
	}
	if snap.FetchedAt.IsZero() {
		t.Errorf("FetchedAt not parsed from time_last_update_utc")
	}
}

func TestFetchCurrencyAPIDirection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cny.json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"date":"2026-09-16","cny":{"usd":0.14,"hkd":1.09,"jpy":21.5}}`))
	}))
	defer srv.Close()

	restore := withFxConfig(config.FxConfig{Provider: "currency-api", Endpoint: srv.URL, TimeoutSeconds: 5})
	defer restore()

	snap, err := fetchWithFallback("CNY", []string{"USD", "HKD"})
	if err != nil {
		t.Fatalf("fetchWithFallback: %v", err)
	}
	if got := snap.Rates["HKD"]; got < 0.91 || got > 0.92 {
		t.Errorf("HKD rate = %v, want ~0.9174 (1/1.09)", got)
	}
	if _, ok := snap.Rates["JPY"]; ok {
		t.Errorf("JPY should be filtered out (not in symbols)")
	}
}

func TestFetchFallbackToNextProvider(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		// 第一个源的形态（/{BASE}）返回 500，第二个源（/{base}.json）返回正常数据
		if r.URL.Path == "/cny.json" {
			_, _ = w.Write([]byte(`{"date":"2026-09-16","cny":{"usd":0.14}}`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	restore := withFxConfig(config.FxConfig{Provider: "auto", Endpoint: srv.URL, TimeoutSeconds: 5})
	defer restore()

	snap, err := fetchWithFallback("CNY", []string{"USD"})
	if err != nil {
		t.Fatalf("fallback failed: %v", err)
	}
	if snap.Source != "currency-api" {
		t.Errorf("source = %s, want currency-api", snap.Source)
	}
	if hits < 2 {
		t.Errorf("expected both providers to be tried, hits=%d", hits)
	}
}

func TestAllProvidersFail(t *testing.T) {
	restore := withFxConfig(config.FxConfig{Provider: "auto", Endpoint: "http://127.0.0.1:1", TimeoutSeconds: 1})
	defer restore()

	if _, err := fetchWithFallback("CNY", []string{"USD"}); err == nil {
		t.Fatal("expected error when every provider fails")
	}
}

func TestSnapshotRate(t *testing.T) {
	s := Snapshot{Base: "CNY", Rates: map[string]float64{"USD": 7.2}}
	if r, ok := s.Rate("usd"); !ok || r != 7.2 {
		t.Errorf("Rate(usd) = %v,%v want 7.2,true", r, ok)
	}
	if r, ok := s.Rate("CNY"); !ok || r != 1 {
		t.Errorf("Rate(base) = %v,%v want 1,true", r, ok)
	}
	if r, ok := s.Rate("XYZ"); ok || r != 1 {
		t.Errorf("Rate(unknown) = %v,%v want 1,false", r, ok)
	}
	// 脏数据不能污染折算
	dirty := Snapshot{Base: "CNY", Rates: map[string]float64{"USD": 0, "JPY": -1}}
	if _, ok := dirty.Rate("USD"); ok {
		t.Errorf("zero rate should be rejected")
	}
	if _, ok := dirty.Rate("JPY"); ok {
		t.Errorf("negative rate should be rejected")
	}
}

func TestGetUnknownFallsBackToOne(t *testing.T) {
	// 数据库未初始化 / 无缓存时，折算必须保持原币金额而不是放大或清零
	r, ok := Get("CNY", "USD")
	if r != 1 {
		t.Errorf("Get = %v, want 1", r)
	}
	if ok {
		t.Errorf("Get should report not-resolved when no data available")
	}
	if r, ok := Get("CNY", "CNY"); r != 1 || !ok {
		t.Errorf("base→base = %v,%v want 1,true", r, ok)
	}
}

func TestNormalizeAndSymbols(t *testing.T) {
	if got := normalize(""); got != "CNY" {
		t.Errorf("normalize(\"\") = %s, want CNY", got)
	}
	if got := normalize(" usd "); got != "USD" {
		t.Errorf("normalize = %s, want USD", got)
	}
	restore := withFxConfig(config.FxConfig{Symbols: []string{" usd ", "USD", "", "jpy"}})
	defer restore()
	got := Symbols()
	if len(got) != 2 || got[0] != "USD" || got[1] != "JPY" {
		t.Errorf("Symbols() = %v, want [USD JPY]", got)
	}
}

func TestProviderOrder(t *testing.T) {
	if len(providerOrder("auto")) != len(providers) {
		t.Errorf("auto should try every provider")
	}
	if got := providerOrder("er-api"); len(got) != 1 || got[0].name != "er-api" {
		t.Errorf("providerOrder(er-api) = %+v", got)
	}
	if got := providerOrder("nonexistent"); len(got) != len(providers) {
		t.Errorf("unknown provider should fall back to all")
	}
}
