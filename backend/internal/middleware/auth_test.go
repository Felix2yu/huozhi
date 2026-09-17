package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"huozhi/internal/config"
	"huozhi/internal/database"
	"huozhi/internal/models"
	"huozhi/pkg/jwt"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func init() {
	config.AppConfig = &config.Config{
		JWT: config.JWTConfig{Secret: "mw-secret", ExpireHours: 168, Issuer: "huozhi"},
	}
	gin.SetMode(gin.TestMode)
}

func TestJWTAuthBearer(t *testing.T) {
	tok, _ := jwt.GenerateToken(7, "bob")
	r := gin.New()
	r.GET("/x", JWTAuth(), func(c *gin.Context) { c.JSON(200, gin.H{"uid": GetUID(c)}) })
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("code %d", w.Code)
	}
}

func TestJWTAuthQuery(t *testing.T) {
	tok, _ := jwt.GenerateToken(9, "carol")
	r := gin.New()
	r.GET("/x", JWTAuth(), func(c *gin.Context) { c.JSON(200, gin.H{"ok": 1}) })
	req := httptest.NewRequest("GET", "/x?token="+tok, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("code %d", w.Code)
	}
}

func TestJWTAuthMissing(t *testing.T) {
	r := gin.New()
	r.GET("/x", JWTAuth())
	req := httptest.NewRequest("GET", "/x", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestJWTAuthBadFormat(t *testing.T) {
	r := gin.New()
	r.GET("/x", JWTAuth())
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Authorization", "Token abc")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestJWTAuthInvalid(t *testing.T) {
	r := gin.New()
	r.GET("/x", JWTAuth())
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func setupAuthDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "auth.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	originalDB := database.DB
	database.DB = db
	t.Cleanup(func() {
		database.DB = originalDB
		if err := sqlDB.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := db.AutoMigrate(&models.User{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestAPIKeyAuthDisabledUser(t *testing.T) {
	db := setupAuthDB(t)
	key := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	user := models.User{Username: "禁用用户", PasswordHash: "测试占位", APIKey: &key, APIKeyEnabled: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&user).Update("status", 0).Error; err != nil {
		t.Fatal(err)
	}
	called := false
	r := gin.New()
	r.GET("/x", APIKeyAuth(), func(c *gin.Context) {
		called = true
		c.Status(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("X-API-Key", key)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized || called {
		t.Fatalf("禁用用户应被拒绝: 状态码 %d，下游执行 %v", w.Code, called)
	}
}

func TestValidateAPIKey(t *testing.T) {
	for _, tt := range []struct {
		name string
		key  string
		want bool
	}{
		{"小写十六进制", strings.Repeat("0123456789abcdef", 4), true},
		{"大写十六进制", strings.Repeat("0123456789ABCDEF", 4), true},
		{"空值", "", false},
		{"过短", strings.Repeat("a", 63), false},
		{"过长", strings.Repeat("a", 65), false},
		{"非法字符", strings.Repeat("a", 63) + "g", false},
		{"空格", strings.Repeat("a", 63) + " ", false},
		{"非ASCII字符", strings.Repeat("a", 61) + "中", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateAPIKey(tt.key); got != tt.want {
				t.Fatalf("密钥格式校验 = %v，期望 %v", got, tt.want)
			}
		})
	}
}

func TestGenerateAPIKey(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 32; i++ {
		key := GenerateAPIKey()
		if !ValidateAPIKey(key) || key != strings.ToLower(key) {
			t.Fatal("生成的密钥应为64位小写十六进制字符串")
		}
		if seen[key] {
			t.Fatal("生成了重复密钥")
		}
		seen[key] = true
	}
}

func TestAPIKeyAndMCPAuth(t *testing.T) {
	db := setupAuthDB(t)
	activeKey := strings.Repeat("a", 64)
	otherKey := strings.Repeat("b", 64)
	disabledKey := strings.Repeat("c", 64)
	blockedKey := strings.Repeat("d", 64)
	deletedKey := strings.Repeat("e", 64)
	unknownKey := strings.Repeat("f", 64)
	users := []models.User{
		{Username: "正常用户", PasswordHash: "测试占位", APIKey: &activeKey, APIKeyEnabled: true},
		{Username: "另一用户", PasswordHash: "测试占位", APIKey: &otherKey, APIKeyEnabled: true},
		{Username: "密钥禁用", PasswordHash: "测试占位", APIKey: &disabledKey, APIKeyEnabled: false},
		{Username: "账户禁用", PasswordHash: "测试占位", APIKey: &blockedKey, APIKeyEnabled: true},
		{Username: "已删除", PasswordHash: "测试占位", APIKey: &deletedKey, APIKeyEnabled: true},
	}
	for i := range users {
		if err := db.Create(&users[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Model(&users[3]).Update("status", 0).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Delete(&users[4]).Error; err != nil {
		t.Fatal(err)
	}
	token, err := jwt.GenerateToken(users[1].ID, users[1].Username)
	if err != nil {
		t.Fatal(err)
	}
	zeroToken, err := jwt.GenerateToken(0, "无用户")
	if err != nil {
		t.Fatal(err)
	}

	for _, mode := range []string{"api_key", "mcp"} {
		t.Run(mode, func(t *testing.T) {
			type authCase struct {
				name          string
				headerKey     string
				authorization string
				query         url.Values
				wantUID       uint
				wantAuthType  string
			}
			cases := []authCase{
				{name: "缺少凭据"},
				{name: "请求头密钥", headerKey: activeKey, wantUID: users[0].ID, wantAuthType: "api_key"},
				{name: "查询参数密钥", query: url.Values{"api_key": {activeKey}}, wantUID: users[0].ID, wantAuthType: "api_key"},
				{name: "未知密钥", headerKey: unknownKey},
				{name: "无效格式", headerKey: "无效"},
				{name: "禁用密钥", headerKey: disabledKey},
				{name: "禁用用户", headerKey: blockedKey},
				{name: "已删除用户", headerKey: deletedKey},
				{name: "请求头优先于查询参数", headerKey: activeKey, query: url.Values{"api_key": {otherKey}}, wantUID: users[0].ID, wantAuthType: "api_key"},
				{name: "无效请求头不回退查询参数", headerKey: unknownKey, query: url.Values{"api_key": {activeKey}}},
			}
			if mode == "mcp" {
				cases = append(cases,
					authCase{name: "JWT请求头", authorization: "Bearer " + token, wantUID: users[1].ID, wantAuthType: "jwt"},
					authCase{name: "JWT查询参数", query: url.Values{"token": {token}}, wantUID: users[1].ID, wantAuthType: "jwt"},
					authCase{name: "JWT忽略大小写及空格", authorization: "bEaReR   " + token + "  ", wantUID: users[1].ID, wantAuthType: "jwt"},
					authCase{name: "JWT优先于密钥", authorization: "Bearer " + token, headerKey: activeKey, wantUID: users[1].ID, wantAuthType: "jwt"},
					authCase{name: "无效JWT回退独立密钥", authorization: "Bearer invalid.token.here", headerKey: activeKey, wantUID: users[0].ID, wantAuthType: "api_key"},
					authCase{name: "Bearer密钥", authorization: "Bearer " + activeKey, wantUID: users[0].ID, wantAuthType: "api_key"},
					authCase{name: "Bearer密钥去除空格", authorization: "bearer   " + activeKey + "  ", wantUID: users[0].ID, wantAuthType: "api_key"},
					authCase{name: "请求头密钥去除空格", headerKey: "  " + activeKey + "  ", wantUID: users[0].ID, wantAuthType: "api_key"},
					authCase{name: "查询密钥去除空格", query: url.Values{"api_key": {"  " + activeKey + "  "}}, wantUID: users[0].ID, wantAuthType: "api_key"},
					authCase{name: "请求头密钥优先于Bearer密钥", headerKey: activeKey, authorization: "Bearer " + otherKey, wantUID: users[0].ID, wantAuthType: "api_key"},
					authCase{name: "Bearer密钥优先于查询密钥", authorization: "Bearer " + activeKey, query: url.Values{"api_key": {otherKey}}, wantUID: users[0].ID, wantAuthType: "api_key"},
					authCase{name: "非Bearer头回退查询JWT", authorization: "Basic ignored", query: url.Values{"token": {token}}, wantUID: users[1].ID, wantAuthType: "jwt"},
					authCase{name: "非Bearer头回退查询密钥", authorization: "Basic ignored", query: url.Values{"api_key": {activeKey}}, wantUID: users[0].ID, wantAuthType: "api_key"},
					authCase{name: "空Bearer回退查询JWT", authorization: "Bearer ", query: url.Values{"token": {token}}, wantUID: users[1].ID, wantAuthType: "jwt"},
					authCase{name: "无效JWT", authorization: "Bearer invalid.token.here"},
					authCase{name: "零用户JWT", authorization: "Bearer " + zeroToken},
					authCase{name: "不完整Authorization", authorization: "Bearer"},
					authCase{name: "非Bearer认证", authorization: "Basic ignored"},
					authCase{name: "空白凭据", headerKey: "  ", authorization: "Bearer  ", query: url.Values{"api_key": {"  "}}},
				)
			}
			for _, tt := range cases {
				t.Run(tt.name, func(t *testing.T) {
					middleware := APIKeyAuth()
					if mode == "mcp" {
						middleware = MCPAuth()
					}
					called := 0
					r := gin.New()
					r.GET("/x", middleware, func(c *gin.Context) {
						called++
						if got := GetUID(c); got != tt.wantUID {
							t.Errorf("用户ID = %d，期望 %d", got, tt.wantUID)
						}
						if mode == "mcp" {
							if got := c.GetString("auth_type"); got != tt.wantAuthType {
								t.Errorf("鉴权类型 = %q，期望 %q", got, tt.wantAuthType)
							}
						} else if got := c.GetString("username"); got != users[0].Username {
							t.Errorf("用户名 = %q，期望 %q", got, users[0].Username)
						}
						c.Status(http.StatusNoContent)
					})
					req := httptest.NewRequest(http.MethodGet, "/x?"+tt.query.Encode(), nil)
					req.Header.Set("X-API-Key", tt.headerKey)
					req.Header.Set("Authorization", tt.authorization)
					w := httptest.NewRecorder()
					r.ServeHTTP(w, req)
					if tt.wantUID > 0 {
						if w.Code != http.StatusNoContent || called != 1 {
							t.Fatalf("鉴权成功应执行一次下游: 状态码 %d，执行次数 %d", w.Code, called)
						}
						if w.Header().Get("WWW-Authenticate") != "" {
							t.Fatal("成功响应不应包含鉴权挑战")
						}
						return
					}
					if w.Code != http.StatusUnauthorized || called != 0 {
						t.Fatalf("鉴权失败应中止请求: 状态码 %d，执行次数 %d", w.Code, called)
					}
					var body struct {
						Error string `json:"error"`
					}
					if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body.Error == "" {
						t.Fatalf("应返回JSON错误信息，解析错误 %v", err)
					}
					if mode == "mcp" && w.Header().Get("WWW-Authenticate") != `Bearer realm="huozhi-mcp"` {
						t.Fatal("MCP拒绝响应缺少正确的鉴权挑战")
					}
				})
			}
		})
	}
}

func TestGetUIDMissing(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	if GetUID(c) != 0 {
		t.Fatal("expected 0 when uid absent")
	}
}
