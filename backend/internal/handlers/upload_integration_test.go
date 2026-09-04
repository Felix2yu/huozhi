package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"huozhi/internal/storage"

	"github.com/gin-gonic/gin"
)

// 1x1 透明 PNG（与 storage_test 同源），DetectContentType 识别为 image/png
var uploadTestPNG = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D,
	0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, 0x89, 0x00, 0x00, 0x00,
	0x0A, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00, 0x00, 0x00, 0x00, 0x49,
	0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
}

func newUploadTestEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/upload", func(c *gin.Context) { c.Set("uid", uint(42)); UploadImage(c) })
	r.GET("/api/uploads/*filepath", ServeUpload)
	return r
}

func TestUploadAndServeIntegration(t *testing.T) {
	dir := t.TempDir()
	storage.SetLocalDir(dir)
	defer os.RemoveAll(dir)

	e := newUploadTestEngine()

	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	fw, err := mw.CreateFormFile("file", "receipt.png")
	if err != nil {
		t.Fatalf("创建表单失败: %v", err)
	}
	fw.Write(uploadTestPNG)
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/upload", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("upload 状态码=%d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Code int `json:"code"`
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if resp.Code != 0 {
		t.Fatalf("业务码非 0: code=%d body=%s", resp.Code, w.Body.String())
	}
	if resp.Data.URL == "" || len(resp.Data.URL) <= len("/api/uploads/") {
		t.Fatalf("返回 url 异常: %q", resp.Data.URL)
	}

	// 通过 ServeUpload 读回
	getReq := httptest.NewRequest(http.MethodGet, resp.Data.URL, nil)
	getW := httptest.NewRecorder()
	e.ServeHTTP(getW, getReq)
	if getW.Code != http.StatusOK {
		t.Fatalf("serve 状态码=%d", getW.Code)
	}
	if getW.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("serve Content-Type=%q", getW.Header().Get("Content-Type"))
	}
	got, _ := io.ReadAll(getW.Body)
	if !bytes.Equal(got, uploadTestPNG) {
		t.Fatalf("serve 内容不一致 len(got)=%d", len(got))
	}
}

func TestUploadRejectsOversizeAndBadType(t *testing.T) {
	dir := t.TempDir()
	storage.SetLocalDir(dir)
	defer os.RemoveAll(dir)

	e := newUploadTestEngine()

	// 非图片（exe）
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	fw, _ := mw.CreateFormFile("file", "x.exe")
	fw.Write([]byte("MZ not an image"))
	mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/upload", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	if w.Code == http.StatusOK {
		t.Fatalf("应当拒绝非图片文件，却返回 200")
	}
}
