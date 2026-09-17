package storage

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"huozhi/internal/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type mockS3 struct {
	mu         sync.Mutex
	objects    map[string][]byte
	prefixes   []string
	deletes    []string
	requests   int
	failMethod string
	failToken  string
	badToken   bool
}

type mockContent struct {
	Key          string
	Size         int64
	LastModified string
}

type mockList struct {
	XMLName               xml.Name `xml:"ListBucketResult"`
	IsTruncated           bool
	NextContinuationToken string `xml:",omitempty"`
	Contents              []mockContent
}

func (m *mockS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests++
	if r.Method == m.failMethod || m.failToken != "" && r.URL.Query().Get("continuation-token") == m.failToken {
		w.WriteHeader(http.StatusForbidden)
		io.WriteString(w, `<Error><Code>AccessDenied</Code><Message>denied</Message></Error>`)
		return
	}
	if r.URL.Path != "/test-bucket" && !strings.HasPrefix(r.URL.Path, "/test-bucket/") {
		http.Error(w, "bucket", http.StatusBadRequest)
		return
	}
	key := strings.TrimPrefix(r.URL.Path, "/test-bucket/")
	if r.URL.Query().Get("list-type") == "2" {
		prefix := r.URL.Query().Get("prefix")
		m.prefixes = append(m.prefixes, prefix)
		keys := make([]string, 0, len(m.objects))
		for key := range m.objects {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		index, _ := strconv.Atoi(r.URL.Query().Get("continuation-token"))
		page := mockList{}
		if index < len(keys) {
			key := keys[index]
			page.Contents = []mockContent{{Key: key, Size: int64(len(m.objects[key])), LastModified: "2020-01-01T00:00:00Z"}}
			page.IsTruncated = index+1 < len(keys)
			if page.IsTruncated && !m.badToken {
				page.NextContinuationToken = strconv.Itoa(index + 1)
			}
		}
		w.Header().Set("Content-Type", "application/xml")
		xml.NewEncoder(w).Encode(page)
		return
	}
	switch r.Method {
	case http.MethodPut:
		if r.Header.Get("X-Amz-Acl") != "" && r.Header.Get("X-Amz-Acl") != "private" {
			http.Error(w, "acl", http.StatusBadRequest)
			return
		}
		data, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		m.objects[key] = data
	case http.MethodGet:
		data, ok := m.objects[key]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			io.WriteString(w, `<Error><Code>NoSuchKey</Code></Error>`)
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Write(data)
	case http.MethodDelete:
		m.deletes = append(m.deletes, key)
		delete(m.objects, key)
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method", http.StatusMethodNotAllowed)
	}
}

func initMockS3(t *testing.T, prefix string) *mockS3 {
	t.Helper()
	m := &mockS3{objects: make(map[string][]byte)}
	server := httptest.NewServer(m)
	t.Cleanup(server.Close)
	initBackupTest(t, &config.Config{S3: config.S3Config{Enabled: true, Endpoint: server.URL, Bucket: "test-bucket", Prefix: prefix, AccessKey: "test-key", SecretKey: "test-secret"}})
	return m
}

func TestBackupS3LifecycleAndIsolation(t *testing.T) {
	for _, prefix := range []string{"", "huozhi", "/nested/huozhi/"} {
		t.Run(prefix, func(t *testing.T) {
			m := initMockS3(t, prefix)
			prefix = strings.Trim(prefix, "/")
			ctx := context.Background()
			name := "auto-7-20260917.zip"
			data := []byte("private zip")
			key, err := SaveBackup(ctx, 7, name, data)
			if err != nil || key != joinKey(prefix, "backups/7/"+name) {
				t.Fatalf("上传失败: %s %v", key, err)
			}
			if _, err := SaveBackup(ctx, 7, "auto-7-20260918.zip", []byte("next")); err != nil {
				t.Fatal(err)
			}
			if _, err := SaveBackup(ctx, 70, "auto-70-20260917.zip", []byte("other")); err != nil {
				t.Fatal(err)
			}
			m.mu.Lock()
			m.objects[joinKey(prefix, "backups/7/auto-70-wrong.zip")] = []byte("wrong owner")
			m.objects[joinKey(prefix, "backups/7/auto-7-nested/file.zip")] = []byte("nested")
			m.objects[joinKey(prefix, "backups/7/auto-7-bad.ZIP")] = []byte("extension")
			m.mu.Unlock()
			objects, err := ListBackups(ctx, 7)
			if err != nil || len(objects) != 2 || objects[0].Name != "auto-7-20260918.zip" || objects[1].Size != int64(len(data)) || objects[1].Time.IsZero() {
				t.Fatalf("分页或隔离失败: %+v %v", objects, err)
			}
			m.mu.Lock()
			for _, got := range m.prefixes {
				if got != joinKey(prefix, "backups/7/") {
					t.Errorf("列举边界错误: %s", got)
				}
			}
			m.mu.Unlock()
			rc, err := OpenBackup(ctx, 7, name)
			if err != nil {
				t.Fatal(err)
			}
			got, err := io.ReadAll(rc)
			rc.Close()
			if err != nil || !bytes.Equal(got, data) {
				t.Fatalf("读取失败: %q %v", got, err)
			}
			m.mu.Lock()
			before := m.requests
			m.mu.Unlock()
			for _, bad := range []string{"backups/7/" + name, "/backups/7/" + name, "./backups/7/" + name, "//backups/7/" + name, "x/../backups/7/" + name, "backups\\7\\" + name, "%62ackups/7/" + name} {
				if rc, _, err := Open(bad); err == nil {
					rc.Close()
					t.Errorf("公共读取未隔离: %s", bad)
				}
				if err := Delete(bad); err == nil {
					t.Errorf("公共删除未隔离: %s", bad)
				}
			}
			if _, err := SaveBackup(ctx, 70, name, nil); err == nil {
				t.Error("跨用户保存未拒绝")
			}
			if rc, err := OpenBackup(ctx, 70, name); err == nil {
				rc.Close()
				t.Error("跨用户读取未拒绝")
			}
			if err := DeleteBackup(ctx, 70, name); err == nil {
				t.Error("跨用户删除未拒绝")
			}
			m.mu.Lock()
			if before != m.requests {
				t.Error("非法请求到达 S3")
			}
			m.mu.Unlock()
			if err := DeleteBackup(ctx, 7, name); err != nil {
				t.Fatal(err)
			}
			if _, err := OpenBackup(ctx, 7, name); err == nil {
				t.Error("删除未生效")
			}
		})
	}
}

func TestS3CleanupBoundaryAndErrors(t *testing.T) {
	m := initMockS3(t, "huozhi")
	m.mu.Lock()
	for _, key := range []string{"huozhi/7/a.png", "huozhi/7/b.png", "huozhi/7/keep.png", "huozhi/backups/7/auto-7-a.zip", "huozhi-other/7/a.png", "huozhi", "huozhi2/7/b.png"} {
		m.objects[key] = []byte("data")
	}
	m.mu.Unlock()
	n, err := CleanupOrphans(map[string]bool{"7/keep.png": true}, time.Hour)
	if err != nil || n != 2 {
		t.Fatalf("清理失败: %d %v", n, err)
	}
	m.mu.Lock()
	for _, prefix := range m.prefixes {
		if prefix != "huozhi/" {
			t.Errorf("图片列举缺少边界: %q", prefix)
		}
	}
	for _, key := range m.deletes {
		if key != "huozhi/7/a.png" && key != "huozhi/7/b.png" {
			t.Errorf("误删: %s", key)
		}
	}
	m.failMethod = http.MethodDelete
	m.mu.Unlock()
	if _, err := CleanupOrphans(nil, 0); err == nil {
		t.Error("清理未传播删除错误")
	}
	m.mu.Lock()
	m.failMethod = ""
	m.failToken = "1"
	m.mu.Unlock()
	if _, err := ListBackups(context.Background(), 7); err == nil {
		t.Error("备份未传播后续页错误")
	}
	if _, err := CleanupOrphans(nil, 0); err == nil {
		t.Error("清理未传播后续页错误")
	}
	m.mu.Lock()
	m.failToken = ""
	m.badToken = true
	m.mu.Unlock()
	if _, err := ListBackups(context.Background(), 7); err == nil {
		t.Error("备份未拒绝无效分页")
	}
	if _, err := CleanupOrphans(nil, 0); err == nil {
		t.Error("清理未拒绝无效分页")
	}
}

func TestBackupS3Errors(t *testing.T) {
	m := initMockS3(t, "")
	ctx := context.Background()
	name := "auto-7-a.zip"
	for _, method := range []string{http.MethodPut, http.MethodGet, http.MethodDelete} {
		m.mu.Lock()
		m.failMethod = method
		m.mu.Unlock()
		switch method {
		case http.MethodPut:
			if key, err := SaveBackup(ctx, 7, name, []byte("x")); err == nil || key != "" {
				t.Errorf("上传未传播错误: %q %v", key, err)
			}
		case http.MethodGet:
			if _, err := ListBackups(ctx, 7); err == nil {
				t.Error("列举未传播错误")
			}
			if _, err := OpenBackup(ctx, 7, name); err == nil {
				t.Error("读取未传播错误")
			}
		case http.MethodDelete:
			if err := DeleteBackup(ctx, 7, name); err == nil {
				t.Error("删除未传播错误")
			}
		}
	}
}

func TestS3EndpointResolution(t *testing.T) {
	for _, endpoint := range []string{"", "https://objects.example.test"} {
		for _, force := range []*bool{nil, aws.Bool(false), aws.Bool(true)} {
			t.Run(fmt.Sprintf("%s/%v", endpoint, force), func(t *testing.T) {
				client, err := newS3Client(&config.S3Config{Enabled: true, Bucket: "test-bucket", Region: "us-east-1", Endpoint: endpoint, ForcePathStyle: force, AccessKey: "test-key", SecretKey: "test-secret"})
				if err != nil {
					t.Fatal(err)
				}
				out, err := s3.NewPresignClient(client).PresignGetObject(context.Background(), &s3.GetObjectInput{Bucket: aws.String("test-bucket"), Key: aws.String("7/a.png")})
				if err != nil {
					t.Fatal(err)
				}
				pathStyle := endpoint != ""
				if force != nil {
					pathStyle = *force
				}
				host := "s3.us-east-1.amazonaws.com"
				if endpoint != "" {
					host = "objects.example.test"
				}
				want := "https://test-bucket." + host + "/7/a.png?"
				if pathStyle {
					want = "https://" + host + "/test-bucket/7/a.png?"
				}
				if !strings.HasPrefix(out.URL, want) {
					t.Fatalf("解析错误: %s", out.URL)
				}
			})
		}
	}
}

func TestInitResetsState(t *testing.T) {
	initMockS3(t, "old")
	dir := t.TempDir()
	config.AppConfig = &config.Config{Upload: config.UploadConfig{Path: filepath.Join(dir, "uploads")}, Backup: config.BackupConfig{Path: filepath.Join(dir, "backups")}}
	if err := Init(); err != nil || UsingS3() || s3Client != nil || s3Bucket != "" || s3Prefix != "" || !inited {
		t.Fatalf("切换本地未重置: %v", err)
	}
	config.AppConfig.S3.Enabled = true
	if err := Init(); err == nil || inited || UsingS3() || s3Client != nil {
		t.Fatalf("初始化失败未重置: %v", err)
	}
	if _, err := SaveBackup(context.Background(), 7, "auto-7-a.zip", nil); err == nil {
		t.Error("初始化失败仍可备份")
	}
	config.AppConfig = nil
	if err := Init(); err == nil || inited || UsingS3() {
		t.Fatalf("空配置未重置: %v", err)
	}
	blocker := filepath.Join(dir, "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	config.AppConfig = &config.Config{Upload: config.UploadConfig{Path: blocker}, Backup: config.BackupConfig{Path: filepath.Join(dir, "backups")}}
	if err := Init(); err == nil {
		t.Error("未传播目录错误")
	}
}
