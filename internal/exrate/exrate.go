// Package exrate 提供汇率拉取、缓存与折算。
//
// 设计要点：
//  1. 只使用**无需 API Key** 的公开汇率接口，并配置多源回退：任一源不可用
//     （网络故障、墙、上游限流）时自动切下一个，全部失败则回退到数据库里
//     上一次成功拉取的快照，保证「离线/上游挂了」不会让外币记账变成 0 折算。
//  2. 汇率落库（models.ExchangeRate）而非只放内存：进程重启后仍有可用数据，
//     且前端可以直接读到「上次更新时间 / 数据来源」，便于判断新鲜度。
//  3. Rate 的口径固定为「1 单位外币 = ? 单位基准币」，与 Transaction.ExchangeRate
//     完全一致，调用方拿到即可直接乘。
package exrate

import (
	"context"
	"encoding/json"
	"fmt"
	"huozhi/internal/config"
	"huozhi/internal/database"
	"huozhi/internal/models"
	"log"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm/clause"
)

// DefaultSymbols 默认拉取的币种。覆盖记账场景最常见的结算货币；
// 上游返回的其它币种会被忽略，避免把上百种冷门币种写进库里。
var DefaultSymbols = []string{
	"CNY", "USD", "EUR", "HKD", "JPY", "GBP", "SGD", "AUD",
	"CAD", "KRW", "TWD", "MOP", "THB", "MYR", "NZD", "CHF",
	"RUB", "INR", "VND", "PHP", "IDR", "AED",
}

// Snapshot 一个基准货币下的汇率快照。
type Snapshot struct {
	Base      string             `json:"base"`
	Rates     map[string]float64 `json:"rates"` // 1 单位币种 = ? 单位 Base
	Source    string             `json:"source"`
	FetchedAt time.Time          `json:"fetched_at"`
	// Stale 表示数据已超过刷新间隔（上游可能还是同一份，但不再保证新鲜）。
	// 注意：拉取失败回退旧数据时它为 true，此时 Rates 仍是可用的历史值。
	Stale bool `json:"stale"`
	// Error 最近一次刷新失败的原因。为空表示本次数据来自上游且成功。
	Error string `json:"error,omitempty"`
}

func (s Snapshot) Rate(currency string) (float64, bool) {
	cur := strings.ToUpper(strings.TrimSpace(currency))
	if cur == "" || cur == s.Base {
		return 1, true
	}
	if s.Rates == nil {
		return 1, false
	}
	r, ok := s.Rates[cur]
	if !ok || !isFinite(r) || r <= 0 {
		return 1, false
	}
	return r, true
}

// ==================== 进程内缓存 ====================

var (
	mu       sync.RWMutex
	cache    = map[string]Snapshot{}
	lastFail = map[string]string{}
)

// Cached 取内存缓存中的快照（可能为零值）。
func Cached(base string) (Snapshot, bool) {
	mu.RLock()
	defer mu.RUnlock()
	s, ok := cache[normalize(base)]
	return s, ok
}

func remember(s Snapshot) Snapshot {
	mu.Lock()
	cache[s.Base] = s
	if s.Error == "" {
		delete(lastFail, s.Base)
	} else {
		lastFail[s.Base] = s.Error
	}
	mu.Unlock()
	return s
}

// LastError 返回该基准货币最近一次刷新失败的原因（成功则为空）。
func LastError(base string) string {
	mu.RLock()
	defer mu.RUnlock()
	return lastFail[normalize(base)]
}

// ==================== 对外主入口 ====================

// Get 取「1 单位 currency = ? 单位 base」的汇率。
// 命中不了的兜底返回 1 —— 与 models.BaseRate 的口径一致：
// 折算失败时保持原币金额，而不是把金额放大成天文数字或清零。
func Get(base, currency string) (float64, bool) {
	b := normalize(base)
	cur := strings.ToUpper(strings.TrimSpace(currency))
	if cur == "" || cur == b {
		return 1, true
	}
	if s, ok := Cached(b); ok {
		if r, ok := s.Rate(cur); ok {
			return r, true
		}
	}
	// 缓存未命中（进程刚启动 / 该基准币从未刷新过）：读库兜底
	var row models.ExchangeRate
	if database.DB != nil {
		err := database.DB.Where("base = ? AND currency = ?", b, cur).Take(&row).Error
		if err == nil && row.Rate > 0 && isFinite(row.Rate) {
			return row.Rate, true
		}
	}
	return 1, false
}

// Convert 把 amount（以 currency 计价）折算为 base 金额。
func Convert(base, currency string, amount float64) (float64, float64, bool) {
	rate, ok := Get(base, currency)
	return amount * rate, rate, ok
}

// SnapshotOf 取指定基准币的快照。缓存/库中都没有时返回空快照（Rates 为空）。
func SnapshotOf(base string) Snapshot {
	b := normalize(base)
	if s, ok := Cached(b); ok {
		return s
	}
	return loadFromDB(b)
}

// Refresh 拉取并刷新汇率。
//
// force=false 且缓存仍在刷新间隔内时直接返回缓存，避免每次记账都打上游。
// 上游不可用时回退到数据库快照并返回错误，由调用方决定如何提示用户
// （HTTP 层照样返回 200 + stale=true，绝不让记账流程中断）。
func Refresh(base string, force bool) (Snapshot, error) {
	b := normalize(base)
	if !Enabled() {
		return SnapshotOf(b), fmt.Errorf("汇率功能已在服务端配置中关闭（fx.disabled）")
	}
	if !force {
		if s, ok := Cached(b); ok && !isStale(s) && len(s.Rates) > 0 {
			return s, nil
		}
	}

	cfg := fxConfig()
	symbols := cfg.symbols()
	snap, err := fetchWithFallback(b, symbols)
	if err != nil {
		old := SnapshotOf(b)
		if len(old.Rates) > 0 {
			old.Stale = true
			old.Error = err.Error()
			log.Printf("[FX] 刷新失败，沿用 %s 的历史汇率（fetched_at=%v）: %v", b, old.FetchedAt, err)
			return remember(old), err
		}
		remember(Snapshot{Base: b, Rates: map[string]float64{}, Error: err.Error()})
		return Snapshot{Base: b, Rates: map[string]float64{}, Error: err.Error()}, err
	}
	if err := persist(b, snap); err != nil {
		log.Printf("[FX] 汇率落库失败（本次结果仅存内存）: %v", err)
	}
	return remember(snap), nil
}

// RefreshIfStale 仅当快照过期时才刷新（供后台调度与前端懒加载使用）。
func RefreshIfStale(base string) (Snapshot, error) {
	s := SnapshotOf(base)
	if len(s.Rates) > 0 && !isStale(s) {
		return s, nil
	}
	return Refresh(base, true)
}

// ==================== 上游数据源 ====================

type provider struct {
	name     string
	endpoint string // 含 %s 占位符时填入小写基准币
	fetch    func(ctx context.Context, endpoint, base string, symbols []string) (map[string]float64, time.Time, error)
}

// providers 按优先级排列。前两个都是免密钥公开接口：
//   - erapi：https://open.er-api.com/v6/latest/{BASE}，返回 1 基准币 = N 外币
//   - currencyapi：@fawazahmed0/currency-api 的公共镜像，同样免密钥
var providers = []provider{
	{
		name:     "er-api",
		endpoint: "https://open.er-api.com/v6/latest",
		fetch:    fetchERAPI,
	},
	{
		name:     "currency-api",
		endpoint: "https://cdn.jsdelivr.net/npm/@fawazahmed0/currency-api@latest/v1/currencies",
		fetch:    fetchCurrencyAPI,
	},
}

func fetchWithFallback(base string, symbols []string) (Snapshot, error) {
	cfg := fxConfig()
	timeout := time.Duration(cfg.timeoutSeconds()) * time.Second
	var errs []string
	order := providerOrder(cfg.Provider)
	for _, p := range order {
		endpoint := p.endpoint
		if cfg.Endpoint != "" {
			endpoint = strings.TrimRight(cfg.Endpoint, "/")
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		rates, at, err := p.fetch(ctx, endpoint, base, symbols)
		cancel()
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", p.name, err))
			continue
		}
		// 换算口径：上游给的是「1 基准币 = N 外币」，这里统一取倒数存成
		// 「1 外币 = N 基准币」，与 Transaction.ExchangeRate 对齐。
		out := make(map[string]float64, len(symbols)+1)
		for _, sym := range symbols {
			cur := strings.ToUpper(sym)
			if cur == base {
				out[cur] = 1
				continue
			}
			v, ok := rates[cur]
			if !ok || !isFinite(v) || v <= 0 {
				continue
			}
			out[cur] = 1 / v
		}
		out[base] = 1
		if len(out) <= 1 {
			errs = append(errs, fmt.Sprintf("%s: 返回数据中没有可用币种", p.name))
			continue
		}
		if at.IsZero() {
			at = time.Now().UTC()
		}
		return Snapshot{Base: base, Rates: out, Source: p.name, FetchedAt: at.UTC()}, nil
	}
	return Snapshot{}, fmt.Errorf("全部汇率源均不可用（%s）", strings.Join(errs, "；"))
}

// providerOrder 根据配置名决定尝试顺序；auto / 未知则用全部源。
func providerOrder(name string) []provider {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" || name == "auto" {
		return providers
	}
	for _, p := range providers {
		if p.name == name {
			return []provider{p}
		}
	}
	return providers
}

func httpGet(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "huozhi/0.1 (+self-hosted)")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d", res.StatusCode)
	}
	buf := make([]byte, 0, 1<<20)
	chunk := make([]byte, 32*1024)
	for {
		n, rerr := res.Body.Read(chunk)
		if n > 0 {
			if len(buf)+n > 8<<20 {
				return nil, fmt.Errorf("响应体过大")
			}
			buf = append(buf, chunk[:n]...)
		}
		if rerr != nil {
			break
		}
	}
	return buf, nil
}

// fetchERAPI 解析 https://open.er-api.com/v6/latest/{BASE}
func fetchERAPI(ctx context.Context, endpoint, base string, symbols []string) (map[string]float64, time.Time, error) {
	body, err := httpGet(ctx, fmt.Sprintf("%s/%s", strings.TrimRight(endpoint, "/"), strings.ToUpper(base)))
	if err != nil {
		return nil, time.Time{}, err
	}
	var payload struct {
		Result string             `json:"result"`
		Base   string             `json:"base_code"`
		Rates  map[string]float64 `json:"rates"`
		Update string             `json:"time_last_update_utc"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, time.Time{}, fmt.Errorf("解析失败: %w", err)
	}
	if strings.ToLower(payload.Result) != "success" || len(payload.Rates) == 0 {
		return nil, time.Time{}, fmt.Errorf("上游返回异常: result=%s", payload.Result)
	}
	// 上游给的是「1 基准币 = N 外币」，直接返回，由调用方统一取倒数
	out := pickSymbols(payload.Rates, symbols)
	return out, parseUpdateTime(payload.Update), nil
}

// fetchCurrencyAPI 解析 .../v1/currencies/{base}.json
func fetchCurrencyAPI(ctx context.Context, endpoint, base string, symbols []string) (map[string]float64, time.Time, error) {
	url := fmt.Sprintf("%s/%s.json", strings.TrimRight(endpoint, "/"), strings.ToLower(base))
	body, err := httpGet(ctx, url)
	if err != nil {
		return nil, time.Time{}, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, time.Time{}, fmt.Errorf("解析失败: %w", err)
	}
	key := strings.ToLower(base)
	inner, ok := raw[key]
	if !ok {
		// 有些镜像把结构包在 {date, base: {...}} 里，这里只取 {date,...} 形态
		return nil, time.Time{}, fmt.Errorf("响应中缺少 %s 节点", key)
	}
	var rates map[string]float64
	if err := json.Unmarshal(inner, &rates); err != nil {
		return nil, time.Time{}, fmt.Errorf("解析失败: %w", err)
	}
	out := pickSymbols(rates, symbols)
	if len(out) == 0 {
		return nil, time.Time{}, fmt.Errorf("没有可用币种")
	}
	at := time.Time{}
	if d := raw["date"]; len(d) > 0 {
		var ds string
		if err := json.Unmarshal(d, &ds); err == nil && ds != "" {
			if t, err := time.Parse("2006-01-02", ds); err == nil {
				at = t.UTC()
			}
		}
	}
	return out, at, nil
}

// pickSymbols 只保留关心的币种并统一成大写键。上游返回的键大小写不定
// （er-api 全大写、currency-api 全小写），这里统一归一化。
func pickSymbols(src map[string]float64, symbols []string) map[string]float64 {
	normal := make(map[string]float64, len(src))
	for k, v := range src {
		normal[strings.ToUpper(k)] = v
	}
	out := make(map[string]float64, len(symbols))
	for _, s := range symbols {
		cur := strings.ToUpper(s)
		if v, ok := normal[cur]; ok && isFinite(v) && v > 0 {
			out[cur] = v
		}
	}
	return out
}

func parseUpdateTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	layouts := []string{
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"Mon, 02 Jan 2006 15:04:05 MST",
		time.RFC1123,
		time.RFC3339,
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

// ==================== 持久化 ====================

func persist(base string, snap Snapshot) error {
	if database.DB == nil {
		return fmt.Errorf("数据库未初始化")
	}
	rows := make([]models.ExchangeRate, 0, len(snap.Rates))
	curs := make([]string, 0, len(snap.Rates))
	for cur, rate := range snap.Rates {
		if !isFinite(rate) || rate <= 0 {
			continue
		}
		curs = append(curs, cur)
		rows = append(rows, models.ExchangeRate{
			Base:      base,
			Currency:  cur,
			Rate:      rate,
			Source:    snap.Source,
			FetchedAt: snap.FetchedAt,
		})
	}
	if len(rows) == 0 {
		return nil
	}
	sort.Strings(curs)
	err := database.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "base"}, {Name: "currency"}},
		DoUpdates: clause.AssignmentColumns([]string{"rate", "source", "fetched_at", "updated_at"}),
	}).Create(&rows).Error
	if err != nil {
		return err
	}
	// 清理该基准币下已不在拉取名单里的历史币种（如用户改了 symbols 配置）
	if len(curs) > 0 {
		database.DB.Where("base = ? AND currency NOT IN ?", base, curs).
			Delete(&models.ExchangeRate{})
	}
	return nil
}

func loadFromDB(base string) Snapshot {
	snap := Snapshot{Base: base, Rates: map[string]float64{}}
	if database.DB == nil {
		return snap
	}
	var rows []models.ExchangeRate
	if err := database.DB.Where("base = ?", base).Find(&rows).Error; err != nil || len(rows) == 0 {
		return snap
	}
	latest := time.Time{}
	for _, r := range rows {
		if r.Rate > 0 && isFinite(r.Rate) {
			snap.Rates[r.Currency] = r.Rate
		}
		if r.FetchedAt.After(latest) {
			latest = r.FetchedAt
		}
		if r.Source != "" {
			snap.Source = r.Source
		}
	}
	snap.FetchedAt = latest
	snap.Stale = isStale(snap)
	if e := LastError(base); e != "" {
		snap.Error = e
	}
	return snap
}

// ==================== 配置与工具 ====================

// Enabled 汇率功能是否在服务端开启
func Enabled() bool {
	cfg := fxConfig()
	return cfg.IsEnabled()
}

// RefreshInterval 刷新间隔（小时）
func RefreshInterval() time.Duration {
	h := fxConfig().refreshHours()
	if h <= 0 {
		h = 12
	}
	return time.Duration(h) * time.Hour
}

// DefaultBase 服务端配置的默认基准货币
func DefaultBase() string {
	return normalize(fxConfig().DefaultBase)
}

// Symbols 当前配置的币种列表
func Symbols() []string {
	return fxConfig().symbols()
}

type fxConf struct {
	config.FxConfig
}

func fxConfig() fxConf {
	if config.AppConfig == nil {
		return fxConf{config.FxConfig{}}
	}
	return fxConf{config.AppConfig.Fx}
}

func (c fxConf) timeoutSeconds() int {
	if c.TimeoutSeconds > 0 {
		return c.TimeoutSeconds
	}
	return 10
}

func (c fxConf) refreshHours() int {
	if c.RefreshIntervalHours > 0 {
		return c.RefreshIntervalHours
	}
	return 12
}

func (c fxConf) symbols() []string {
	if len(c.Symbols) == 0 {
		return DefaultSymbols
	}
	out := make([]string, 0, len(c.Symbols))
	seen := map[string]struct{}{}
	for _, s := range c.Symbols {
		cur := strings.ToUpper(strings.TrimSpace(s))
		if cur == "" {
			continue
		}
		if _, dup := seen[cur]; dup {
			continue
		}
		seen[cur] = struct{}{}
		out = append(out, cur)
	}
	if len(out) == 0 {
		return DefaultSymbols
	}
	return out
}

// isStale 距上次成功拉取已超过刷新间隔
func isStale(s Snapshot) bool {
	if s.FetchedAt.IsZero() || len(s.Rates) == 0 {
		return true
	}
	return time.Since(s.FetchedAt) > RefreshInterval()
}

func normalize(base string) string {
	b := strings.ToUpper(strings.TrimSpace(base))
	if b == "" {
		return "CNY"
	}
	return b
}

func isFinite(f float64) bool { return !math.IsNaN(f) && !math.IsInf(f, 0) }

// CurrenciesFor 返回快照中已排序的币种列表（供设置页展示）
func (s Snapshot) Currencies() []string {
	out := make([]string, 0, len(s.Rates))
	for c := range s.Rates {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}
