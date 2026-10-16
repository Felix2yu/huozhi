package handlers_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"huozhi/internal/config"
	"huozhi/internal/database"
	"huozhi/internal/handlers"
	"huozhi/internal/models"
	"huozhi/internal/storage"
)

func setupAutoBackupStorage(t *testing.T) {
	t.Helper()
	original := config.AppConfig
	cfg := *original
	dir := t.TempDir()
	cfg.Upload.Path = filepath.Join(dir, "uploads")
	cfg.Backup.Path = filepath.Join(dir, "backups")
	cfg.S3 = config.S3Config{}
	config.AppConfig = &cfg
	t.Cleanup(func() { config.AppConfig = original })
	if err := storage.Init(); err != nil {
		t.Fatal(err)
	}
}

func createAutoBackup(t *testing.T, uid uint, token string) string {
	t.Helper()
	w := do(authReq(http.MethodPost, "/api/io/auto-backups", token, nil))
	if w.Code != http.StatusCreated {
		t.Fatalf("立即备份状态码=%d，响应=%s", w.Code, w.Body.String())
	}
	var response struct {
		Data struct {
			Name    string `json:"name"`
			Storage string `json:"storage"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	name := response.Data.Name
	if !strings.HasPrefix(name, fmt.Sprintf("auto-%d-", uid)) || !strings.HasSuffix(name, ".zip") || filepath.Base(name) != name {
		t.Fatalf("备份名称错误：%q", name)
	}
	if response.Data.Storage != "local" {
		t.Fatalf("存储类型=%q，期望 local", response.Data.Storage)
	}
	return name
}

func listAutoBackups(t *testing.T, token string) []storage.BackupObject {
	t.Helper()
	w := do(authReq(http.MethodGet, "/api/io/auto-backups", token, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("备份列表状态码=%d，响应=%s", w.Code, w.Body.String())
	}
	var response struct {
		Data []storage.BackupObject `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	return response.Data
}

func TestAutoBackupCreateDownloadRestoreImages(t *testing.T) {
	setupAutoBackupStorage(t)
	uid, token := newUser(t)
	bookID := seedBook(t, uid)
	accountID := seedAccount(t, uid, bookID)
	categoryID := seedCategory(t, uid, bookID, models.KindExpense, "备份分类")
	var imageData bytes.Buffer
	if err := png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	imageURL, err := storage.SaveBytes(imageData.Bytes(), "receipt.png", uid)
	if err != nil {
		t.Fatal(err)
	}
	key, ok := storage.KeyFromURL(imageURL)
	if !ok || !strings.HasPrefix(key, itoa(uid)+"/") {
		t.Fatalf("图片路径错误：%q", imageURL)
	}
	tx := models.Transaction{
		UserID: uid, BookID: bookID, AccountID: accountID, CategoryID: categoryID,
		Type: models.TxExpense, Amount: 1234, Currency: "CNY", TxDate: time.Now().UTC(),
		Description: "自动备份图片", Images: []string{imageURL},
	}
	if err := database.DB.Create(&tx).Error; err != nil {
		t.Fatal(err)
	}
	otherUID, _ := newUser(t)
	seedBook(t, otherUID)

	name := createAutoBackup(t, uid, token)
	w := do(authReq(http.MethodGet, "/api/io/auto-backups/"+name, token, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("下载状态码=%d，响应=%s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); got != "application/zip" {
		t.Fatalf("下载 Content-Type=%q", got)
	}
	if got := w.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Fatalf("下载 Cache-Control=%q", got)
	}
	disposition, params, err := mime.ParseMediaType(w.Header().Get("Content-Disposition"))
	if err != nil || disposition != "attachment" || params["filename"] != name {
		t.Fatalf("下载附件头错误：%q，错误=%v", w.Header().Get("Content-Disposition"), err)
	}
	archive := append([]byte(nil), w.Body.Bytes()...)
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatal(err)
	}
	entries := make(map[string][]byte)
	for _, file := range zr.File {
		if _, exists := entries[file.Name]; exists {
			t.Fatalf("ZIP 条目重复：%q", file.Name)
		}
		rc, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, readErr := io.ReadAll(rc)
		closeErr := rc.Close()
		if readErr != nil || closeErr != nil {
			t.Fatalf("读取 ZIP 条目失败：%v，%v", readErr, closeErr)
		}
		entries[file.Name] = data
	}
	if len(entries) != 2 || !bytes.Equal(entries["images/"+key], imageData.Bytes()) {
		t.Fatalf("ZIP 图片内容或条目数量错误：条目数=%d，图片路径=%q", len(entries), "images/"+key)
	}
	var snapshot struct {
		Version      string               `json:"version"`
		Books        []models.Book        `json:"books"`
		Transactions []models.Transaction `json:"transactions"`
	}
	if err := json.Unmarshal(entries["backup.json"], &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Version != "2.0" || len(snapshot.Books) != 1 || snapshot.Books[0].ID != bookID || snapshot.Books[0].UserID != uid {
		t.Fatalf("快照版本或账本隔离错误：%+v", snapshot)
	}
	if len(snapshot.Transactions) != 1 {
		t.Fatalf("快照交易数量=%d", len(snapshot.Transactions))
	}
	gotTx := snapshot.Transactions[0]
	if gotTx.ID != tx.ID || gotTx.UserID != uid || gotTx.Amount != tx.Amount || !reflect.DeepEqual(gotTx.Images, tx.Images) {
		t.Fatalf("快照交易错误：%+v", gotTx)
	}
	listed := listAutoBackups(t, token)
	if len(listed) != 1 || listed[0].Name != name || listed[0].Size != int64(len(archive)) || listed[0].Time.IsZero() {
		t.Fatalf("立即备份未正确列出：%+v", listed)
	}

	if err := storage.Delete(key); err != nil {
		t.Fatal(err)
	}
	if rc, _, err := storage.Open(key); err == nil {
		rc.Close()
		t.Fatal("恢复前原图片仍然存在")
	}
	if err := database.DB.Model(&tx).Update("description", "备份之后修改").Error; err != nil {
		t.Fatal(err)
	}
	req := authReq(http.MethodPost, "/api/io/restore?mode=replace", token, nil)
	req.Body = io.NopCloser(bytes.NewReader(archive))
	req.ContentLength = int64(len(archive))
	req.Header.Set("Content-Type", "application/zip")
	w = do(req)
	if w.Code != http.StatusOK {
		t.Fatalf("恢复状态码=%d，响应=%s", w.Code, w.Body.String())
	}
	data := decode(t, w)["data"].(map[string]interface{})
	if data["mode"] != "replace" || data["images_restored"] != float64(1) {
		t.Fatalf("恢复结果错误：%v", data)
	}
	var restored []models.Transaction
	if err := database.DB.Where("user_id = ?", uid).Find(&restored).Error; err != nil {
		t.Fatal(err)
	}
	if len(restored) != 1 || restored[0].ID != tx.ID || restored[0].Description != "自动备份图片" || restored[0].Amount != tx.Amount || len(restored[0].Images) != 1 {
		t.Fatalf("恢复交易错误：%+v", restored)
	}
	restoredURL := restored[0].Images[0]
	if restoredURL == imageURL || !strings.HasPrefix(restoredURL, "/api/uploads/"+itoa(uid)+"/") {
		t.Fatalf("恢复图片未重建当前用户路径：%q", restoredURL)
	}
	w = do(authReq(http.MethodGet, restoredURL, token, nil))
	if w.Code != http.StatusOK || !bytes.Equal(w.Body.Bytes(), imageData.Bytes()) {
		t.Fatalf("恢复图片下载失败或内容不一致：状态码=%d", w.Code)
	}
}

func TestAutoBackupDownloadAuthAndListIsolation(t *testing.T) {
	setupAutoBackupStorage(t)
	uid, token := newUser(t)
	otherUID, otherToken := newUser(t)
	_, emptyToken := newUser(t)
	name := createAutoBackup(t, uid, token)
	otherName := createAutoBackup(t, otherUID, otherToken)
	path := "/api/io/auto-backups/" + name
	for _, tc := range []struct {
		name   string
		token  string
		status int
	}{
		{name: "无凭据", status: http.StatusUnauthorized},
		{name: "无效凭据", token: "invalid-token", status: http.StatusUnauthorized},
		{name: "其他用户", token: otherToken, status: http.StatusNotFound},
		{name: "备份所有者", token: token, status: http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := do(authReq(http.MethodGet, path, tc.token, nil))
			if w.Code != tc.status {
				t.Fatalf("下载状态码=%d，期望=%d", w.Code, tc.status)
			}
		})
	}
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		w := do(authReq(method, "/api/io/auto-backups", "", nil))
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("%s 未认证状态码=%d", method, w.Code)
		}
	}
	for _, tc := range []struct {
		token string
		name  string
	}{{token: token, name: name}, {token: otherToken, name: otherName}} {
		listed := listAutoBackups(t, tc.token)
		if len(listed) != 1 || listed[0].Name != tc.name {
			t.Fatalf("备份列表未隔离：%+v，期望=%q", listed, tc.name)
		}
	}
	if listed := listAutoBackups(t, emptyToken); len(listed) != 0 {
		t.Fatalf("无备份用户列表不为空：%+v", listed)
	}
	w := do(authReq(http.MethodGet, "/api/io/auto-backups/"+otherName, token, nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("反向跨用户下载状态码=%d", w.Code)
	}
}

func TestAutoBackupCleanupOldBackups(t *testing.T) {
	setupAutoBackupStorage(t)
	for _, tc := range []struct {
		name string
		keep int
		want int
	}{
		{name: "保留最新两份", keep: 2, want: 2},
		{name: "零值默认保留七份", keep: 0, want: 7},
		{name: "负值默认保留七份", keep: -1, want: 7},
		{name: "保留数大于总数", keep: 20, want: 9},
	} {
		t.Run(tc.name, func(t *testing.T) {
			uid, token := newUser(t)
			otherUID, otherToken := newUser(t)
			ctx := context.Background()
			for _, day := range []int{5, 1, 9, 2, 8, 3, 7, 4, 6} {
				for _, owner := range []uint{uid, otherUID} {
					name := fmt.Sprintf("auto-%d-202601%02d-120000.zip", owner, day)
					if _, err := storage.SaveBackup(ctx, owner, name, []byte(name)); err != nil {
						t.Fatal(err)
					}
				}
			}
			othersBefore := listAutoBackups(t, otherToken)
			if len(othersBefore) != 9 {
				t.Fatalf("其他用户初始备份数=%d", len(othersBefore))
			}
			handlers.CleanupOldBackups(uid, tc.keep)
			listed := listAutoBackups(t, token)
			if len(listed) != tc.want {
				t.Fatalf("保留数量=%d，期望=%d", len(listed), tc.want)
			}
			for i, backup := range listed {
				want := fmt.Sprintf("auto-%d-202601%02d-120000.zip", uid, 9-i)
				if backup.Name != want {
					t.Fatalf("保留备份=%q，期望=%q", backup.Name, want)
				}
			}
			for day := 1; day <= 9-tc.want; day++ {
				name := fmt.Sprintf("auto-%d-202601%02d-120000.zip", uid, day)
				if rc, err := storage.OpenBackup(ctx, uid, name); err == nil {
					rc.Close()
					t.Fatalf("旧备份未删除：%q", name)
				}
			}
			if othersAfter := listAutoBackups(t, otherToken); !reflect.DeepEqual(othersBefore, othersAfter) {
				t.Fatalf("清理修改了其他用户备份：之前=%+v，之后=%+v", othersBefore, othersAfter)
			}
			for _, backup := range othersBefore {
				rc, err := storage.OpenBackup(ctx, otherUID, backup.Name)
				if err != nil {
					t.Fatal(err)
				}
				data, readErr := io.ReadAll(rc)
				closeErr := rc.Close()
				if readErr != nil || closeErr != nil || string(data) != backup.Name {
					t.Fatalf("其他用户备份内容被修改：%q，错误=%v，%v", backup.Name, readErr, closeErr)
				}
			}
			handlers.CleanupOldBackups(uid, tc.keep)
			if again := listAutoBackups(t, token); !reflect.DeepEqual(listed, again) {
				t.Fatalf("重复清理结果不一致：%+v", again)
			}
		})
	}
}
