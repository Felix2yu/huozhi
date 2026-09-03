package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"sync/atomic"
	"testing"

	"huozhi/internal/config"
	"huozhi/internal/database"
	"huozhi/internal/models"
	"huozhi/internal/router"
	"huozhi/internal/ws"
	"huozhi/pkg/jwt"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var eng *gin.Engine
var userSeq uint64

func TestMain(m *testing.M) {
	config.AppConfig = &config.Config{
		JWT: config.JWTConfig{Secret: "test-secret", ExpireHours: 168, Issuer: "huozhi"},
		Upload: config.UploadConfig{Path: os.TempDir(), MaxSizeMB: 10, Allowed: "jpg,png"},
	}
	gin.SetMode(gin.TestMode)

	f, err := os.CreateTemp("", "huozhi-test-*.db")
	if err != nil {
		panic(err)
	}
	dbPath := f.Name()
	f.Close()

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	database.DB = db
	_ = db.AutoMigrate(
		&models.User{}, &models.Book{}, &models.BookMember{}, &models.AccountGroup{},
		&models.Account{}, &models.Category{}, &models.Tag{}, &models.Transaction{},
		&models.TransactionTag{}, &models.Budget{}, &models.SavingPlan{}, &models.SavingRecord{},
		&models.Recurring{}, &models.Installment{}, &models.Reimbursement{},
		&models.AssetSnapshot{}, &models.SyncLog{},
	)

	go ws.DefaultHub.Run()
	eng = router.New("test")

	code := m.Run()
	os.Remove(dbPath)
	os.Exit(code)
}

// newUser creates a user directly in the DB and returns its id + JWT.
func newUser(t *testing.T) (uint, string) {
	t.Helper()
	n := atomic.AddUint64(&userSeq, 1)
	suffix := strconv.FormatUint(n, 10)
	user := models.User{
		Username:     "user" + suffix,
		Email:        "user" + suffix + "@example.com",
		Phone:        "phone-" + suffix,
		PasswordHash: "x",
		Nickname:     "Test" + suffix,
		Currency:     "CNY",
		Status:       1,
	}
	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	tok, err := jwt.GenerateToken(user.ID, user.Username)
	if err != nil {
		t.Fatal(err)
	}
	return user.ID, tok
}

func seedBook(t *testing.T, uid uint) uint {
	t.Helper()
	b := models.Book{UserID: uid, Name: "Book", Currency: "CNY", IsDefault: true}
	database.DB.Create(&b)
	return b.ID
}

func seedAccount(t *testing.T, uid, bookID uint) uint {
	t.Helper()
	a := models.Account{UserID: uid, BookID: bookID, Name: "Cash", Type: models.AccCash, Currency: "CNY", Balance: 0}
	database.DB.Create(&a)
	return a.ID
}

func seedCategory(t *testing.T, uid, bookID uint, kind models.CategoryKind, name string) uint {
	t.Helper()
	c := models.Category{UserID: uid, BookID: bookID, Name: name, Kind: kind, Icon: "x"}
	database.DB.Create(&c)
	return c.ID
}

func seedAdjustCategory(t *testing.T, uid, bookID uint) uint {
	return seedCategory(t, uid, bookID, models.KindSystem, "余额调整")
}

func authReq(method, path, token string, body interface{}) *http.Request {
	var r *http.Request
	if body != nil {
		b, _ := json.Marshal(body)
		r = httptest.NewRequest(method, path, bytes.NewReader(b))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	return r
}

func do(req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	eng.ServeHTTP(w, req)
	return w
}

func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode response: %v body=%s", err, w.Body.String())
	}
	return m
}

func randomSuffix() string {
	return strconv.FormatUint(atomic.AddUint64(&userSeq, 1), 10)
}

func generateTokenFor(uid uint) (string, error) {
	return jwt.GenerateToken(uid, "x")
}

func itoa(u uint) string {
	return strconv.Itoa(int(u))
}

// registerRealUser registers via the HTTP endpoint so the default book and
// built-in categories are created by initUserDefaults, then returns credentials.
func registerRealUser(t *testing.T) (uid uint, tok string, bookID uint) {
	t.Helper()
	username := "ru_" + randomSuffix()
	w := do(authReq("POST", "/api/auth/register", "", map[string]string{
		"username": username, "password": "secret123", "nickname": "RU",
		"email": username + "@example.com", "phone": username + "_phone",
	}))
	if w.Code != 201 {
		t.Fatalf("register failed %d %s", w.Code, w.Body.String())
	}
	m := decode(t, w)
	data := m["data"].(map[string]interface{})
	tok = data["token"].(string)
	user := data["user"].(map[string]interface{})
	uid = uint(user["id"].(float64))
	var book models.Book
	database.DB.Where("user_id = ? AND is_default = ?", uid, true).First(&book)
	bookID = book.ID
	return
}
