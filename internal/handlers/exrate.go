package handlers

import (
	"huozhi/internal/database"
	"huozhi/internal/exrate"
	"huozhi/internal/middleware"
	"huozhi/internal/models"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// ========== 汇率 ==========

// BaseCurrencyOf 取用户的基准货币。
//
// 基准货币 = User.Currency：外币流水按 exchange_rate 折算到它，
// 前端也用同一个值判断「这笔账是不是外币」。三处口径必须一致，
// 否则会出现「列表按美元显示、统计按人民币汇总」的自相矛盾。
// 用户缺失 / 未设置时回落到服务端配置的 default_base。
func BaseCurrencyOf(uid uint) string {
	if uid == 0 || database.DB == nil {
		return exrate.DefaultBase()
	}
	var cur string
	if err := database.DB.Model(&models.User{}).Where("id = ?", uid).
		Pluck("currency", &cur).Error; err != nil {
		return exrate.DefaultBase()
	}
	cur = strings.ToUpper(strings.TrimSpace(cur))
	if cur == "" {
		return exrate.DefaultBase()
	}
	return cur
}

// fxView 汇率接口统一返回体。
// 同时带回用户的偏好设置，避免前端为了渲染设置页再打一次 /auth/me。
type fxView struct {
	Base         string             `json:"base"`
	Rates        map[string]float64 `json:"rates"` // 1 单位币种 = ? 单位基准币
	Currencies   []string           `json:"currencies"`
	Source       string             `json:"source"`
	FetchedAt    *time.Time         `json:"fetched_at"`
	Stale        bool               `json:"stale"`
	Error        string             `json:"error,omitempty"`
	Enabled      bool               `json:"enabled"`
	AutoRefresh  bool               `json:"auto_refresh"`
	RefreshHours int                `json:"refresh_hours"`
}

func toFxView(s exrate.Snapshot, pref models.User) fxView {
	v := fxView{
		Base:         s.Base,
		Rates:        s.Rates,
		Currencies:   s.Currencies(),
		Source:       s.Source,
		Stale:        s.Stale,
		Error:        s.Error,
		Enabled:      exrate.Enabled(),
		AutoRefresh:  pref.FxAutoRefresh,
		RefreshHours: pref.FxRefreshHours,
	}
	if v.Rates == nil {
		v.Rates = map[string]float64{}
	}
	if !s.FetchedAt.IsZero() {
		t := s.FetchedAt
		v.FetchedAt = &t
	}
	return v
}

func loadFxPref(uid uint) models.User {
	var u models.User
	if uid > 0 && database.DB != nil {
		database.DB.Select("id", "currency", "fx_auto_refresh", "fx_refresh_hours").First(&u, uid)
	}
	return u
}

// ListExchangeRates 查询当前汇率快照。
//
// 缓存里没有任何数据时（首次启动 / 首次切到该基准币）才同步拉取一次，
// 之后一律走缓存或库里的快照 —— 记账表单每次输入都打上游既慢又会被限流。
func ListExchangeRates(c *gin.Context) {
	uid := middleware.GetUID(c)
	pref := loadFxPref(uid)
	base := strings.ToUpper(strings.TrimSpace(c.Query("base")))
	if base == "" {
		base = BaseCurrencyOf(uid)
	}
	snap := exrate.SnapshotOf(base)
	if len(snap.Rates) == 0 && exrate.Enabled() {
		// 首次使用：拉一次，失败也不阻塞（返回空表 + error，前端提示手动重试）
		snap, _ = exrate.Refresh(base, true)
	}
	OK(c, toFxView(snap, pref))
}

// refreshThrottle 防止用户狂点「立即刷新」把上游打爆 / 被限流。
var (
	refreshMu    sync.Mutex
	lastManualAt = map[string]time.Time{}
	manualMinGap = 10 * time.Second
)

// RefreshExchangeRates 强制刷新汇率。
//
// 上游全部不可用时：库里还有历史数据就照样返回（stale=true），
// 一次拉取失败不该让「记一笔外币账」整个功能不可用。
func RefreshExchangeRates(c *gin.Context) {
	uid := middleware.GetUID(c)
	pref := loadFxPref(uid)

	var body struct {
		Base string `json:"base"`
	}
	_ = c.ShouldBindJSON(&body)
	base := strings.ToUpper(strings.TrimSpace(body.Base))
	if base == "" {
		base = strings.ToUpper(strings.TrimSpace(c.Query("base")))
	}
	if base == "" {
		base = BaseCurrencyOf(uid)
	}

	refreshMu.Lock()
	last := lastManualAt[base]
	now := time.Now()
	if now.Sub(last) < manualMinGap {
		refreshMu.Unlock()
		Fail(c, 1008, "刷新过于频繁，请稍后再试")
		return
	}
	lastManualAt[base] = now
	refreshMu.Unlock()

	snap, err := exrate.Refresh(base, true)
	if err != nil && len(snap.Rates) == 0 {
		Fail(c, 1009, "汇率获取失败："+err.Error())
		return
	}
	OK(c, toFxView(snap, pref))
}

// ConvertAmount 金额换算（供记账表单实时预览，避免前端各处重复实现折算）。
func ConvertAmount(c *gin.Context) {
	uid := middleware.GetUID(c)
	base := strings.ToUpper(strings.TrimSpace(c.DefaultQuery("base", "")))
	if base == "" {
		base = BaseCurrencyOf(uid)
	}
	from := strings.ToUpper(strings.TrimSpace(c.DefaultQuery("from", c.DefaultQuery("currency", ""))))
	amount := queryFloat(c, "amount")
	converted, rate, ok := exrate.Convert(base, from, amount)
	OK(c, gin.H{
		"base":      base,
		"from":      from,
		"amount":    amount,
		"rate":      rate,
		"converted": converted,
		"resolved":  ok,
	})
}

func queryFloat(c *gin.Context, key string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(c.Query(key)), 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return v
}
