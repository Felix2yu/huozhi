package storage

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"huozhi/internal/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// 存储层：统一封装「本地文件系统」与「S3 兼容对象存储」两种后端。
// 对外返回统一的公开路径 /api/uploads/<key>，由 Serve() 按后端取回数据，
// 这样前端无需关心后端差异，也免去了 S3 存储桶必须公开可读的限制。

const (
	// 上传路径前缀（与 router 中的 /api/uploads/* 静态路由一致）
	publicPrefix = "/api/uploads/"
	// 默认本地目录（未显式配置时使用）
	defaultLocalDir = "./uploads"
)

var (
	mu       sync.RWMutex
	inited   bool
	useS3    bool
	localDir string

	s3Client *s3.Client
	s3Bucket string
	s3Prefix string
)

// Init 依据全局配置初始化存储层。在 main 中配置加载后调用一次。
func Init() error {
	cfg := config.AppConfig
	if cfg == nil {
		return fmt.Errorf("config 未加载")
	}
	mu.Lock()
	defer mu.Unlock()

	localDir = cfg.Upload.Path
	if localDir == "" {
		localDir = defaultLocalDir
	}
	_ = os.MkdirAll(localDir, 0o755)

	if cfg.S3.Enabled && cfg.S3.Bucket != "" {
		client, err := newS3Client(&cfg.S3)
		if err != nil {
			return fmt.Errorf("初始化 S3 客户端失败: %w", err)
		}
		s3Client = client
		s3Bucket = cfg.S3.Bucket
		s3Prefix = strings.Trim(cfg.S3.Prefix, "/")
		useS3 = true
	}
	inited = true
	return nil
}

// SetLocalDir 仅用于测试：覆盖本地存储目录。
func SetLocalDir(dir string) {
	mu.Lock()
	defer mu.Unlock()
	localDir = dir
	_ = os.MkdirAll(localDir, 0o755)
}

// UsingS3 报告当前是否使用 S3 后端（便于运维/调试）。
func UsingS3() bool {
	mu.RLock()
	defer mu.RUnlock()
	return useS3
}

func newS3Client(s *config.S3Config) (*s3.Client, error) {
	ctx := context.Background()
	region := s.Region
	if region == "" {
		region = "us-east-1"
	}
	endpoint := s.Endpoint
	if endpoint != "" && !strings.Contains(endpoint, "://") {
		scheme := "https"
		if !s.UseSSL {
			scheme = "http"
		}
		endpoint = scheme + "://" + endpoint
	}

	resolver := aws.EndpointResolverWithOptionsFunc(
		func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			if endpoint != "" {
				return aws.Endpoint{URL: endpoint, HostnameImmutable: true}, nil
			}
			return aws.Endpoint{}, fmt.Errorf("未配置 S3 endpoint")
		})

	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(region),
		awsconfig.WithEndpointResolverWithOptions(resolver),
	}
	if s.AccessKey != "" || s.SecretKey != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(s.AccessKey, s.SecretKey, "")))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, err
	}
	// 自定义端点（MinIO / 阿里云 OSS 等）通常使用 path-style 寻址
	return s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = endpoint != ""
	}), nil
}

// Save 读取上传的文件头，校验类型/大小后落盘，返回公开路径。
func Save(file *multipart.FileHeader, uid uint) (string, error) {
	src, err := file.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	limit := maxBytes()
	data, err := io.ReadAll(io.LimitReader(src, limit+1))
	if err != nil {
		return "", err
	}
	if int64(len(data)) > limit {
		return "", fmt.Errorf("文件过大（上限 %dMB）", limit/1024/1024)
	}
	return SaveBytes(data, file.Filename, uid)
}

// SaveBytes 将字节直接存储，返回公开路径 /api/uploads/<key>。
func SaveBytes(data []byte, name string, uid uint) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("空文件")
	}
	if err := checkAllowed(name, data); err != nil {
		return "", err
	}
	ext := extFromNameAndData(name, data)
	key := fmt.Sprintf("%d/%s%s", uid, randHex(), ext)

	mu.RLock()
	s3Mode := useS3
	bucket := s3Bucket
	prefix := s3Prefix
	client := s3Client
	dir := localDir
	mu.RUnlock()

	if s3Mode {
		fullKey := joinKey(prefix, key)
		_, err := client.PutObject(context.Background(), &s3.PutObjectInput{
			Bucket:      aws.String(bucket),
			Key:         aws.String(fullKey),
			Body:        bytes.NewReader(data),
			ContentType: aws.String(http.DetectContentType(data)),
		})
		if err != nil {
			return "", fmt.Errorf("S3 上传失败: %w", err)
		}
	} else {
		full := filepath.Join(dir, key)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(full, data, 0o644); err != nil {
			return "", err
		}
	}
	return publicPrefix + key, nil
}

// Open 按 key（不含前缀）取回存储对象，返回可读流与内容类型。
func Open(key string) (io.ReadCloser, string, error) {
	key = strings.TrimPrefix(key, "/")
	if key == "" || strings.Contains(key, "..") {
		return nil, "", fmt.Errorf("非法 key")
	}

	mu.RLock()
	s3Mode := useS3
	bucket := s3Bucket
	prefix := s3Prefix
	client := s3Client
	dir := localDir
	mu.RUnlock()

	if s3Mode {
		out, err := client.GetObject(context.Background(), &s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(joinKey(prefix, key)),
		})
		if err != nil {
			return nil, "", err
		}
		ct := ""
		if out.ContentType != nil {
			ct = *out.ContentType
		}
		return out.Body, ct, nil
	}

	full := filepath.Join(dir, key)
	f, err := os.Open(full)
	if err != nil {
		return nil, "", err
	}
	return f, mimeType(full, f), nil
}

// Delete 删除指定 key 的存储对象（忽略不存在的错误）。
func Delete(key string) error {
	key = strings.TrimPrefix(key, "/")
	if key == "" || strings.Contains(key, "..") {
		return fmt.Errorf("非法 key")
	}
	mu.RLock()
	s3Mode := useS3
	bucket := s3Bucket
	prefix := s3Prefix
	client := s3Client
	dir := localDir
	mu.RUnlock()

	if s3Mode {
		_, err := client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(joinKey(prefix, key)),
		})
		return err
	}
	err := os.Remove(filepath.Join(dir, key))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// ============ 孤儿文件清理 ============

// StoredObject 描述存储中的一个对象（用于孤儿清理）。
type StoredObject struct {
	Key     string    // 内部 key（不含 S3 prefix，不含 /api/uploads/ 前缀）
	ModTime time.Time // 修改时间（S3 为 LastModified）
}

// KeyFromURL 将公开路径（/api/uploads/<key> 或 /uploads/<key>）还原为内部 key。
func KeyFromURL(u string) (string, bool) {
	if strings.HasPrefix(u, publicPrefix) {
		return strings.TrimPrefix(u, publicPrefix), true
	}
	if strings.HasPrefix(u, "/uploads/") {
		return strings.TrimPrefix(u, "/uploads/"), true
	}
	return "", false
}

// CleanupOrphans 删除未被引用的孤儿文件。referenced 为仍被引用的内部 key 集合；
// 不在集合中且修改时间早于 now-grace 的对象将被删除。
// grace 用于避免误删「刚上传、交易尚未保存」的临时文件。
func CleanupOrphans(referenced map[string]bool, grace time.Duration) (int, error) {
	mu.RLock()
	s3Mode := useS3
	bucket := s3Bucket
	prefix := s3Prefix
	client := s3Client
	dir := localDir
	mu.RUnlock()

	objs, err := listStoredObjects(s3Mode, bucket, prefix, client, dir)
	if err != nil {
		return 0, err
	}
	cutoff := time.Now().Add(-grace)
	deleted := 0
	for _, o := range objs {
		if referenced[o.Key] {
			continue
		}
		if !o.ModTime.IsZero() && o.ModTime.After(cutoff) {
			continue // 宽限期内的新文件保留
		}
		if err := Delete(o.Key); err == nil {
			deleted++
		}
	}
	return deleted, nil
}

// listStoredObjects 列出存储中的全部对象（含修改时间）。
func listStoredObjects(s3Mode bool, bucket, prefix string, client *s3.Client, dir string) ([]StoredObject, error) {
	if s3Mode {
		var out []StoredObject
		ctx := context.Background()
		var token *string
		for {
			page, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
				Bucket:            aws.String(bucket),
				Prefix:            aws.String(prefix),
				ContinuationToken: token,
			})
			if err != nil {
				return nil, err
			}
			for _, item := range page.Contents {
				key := aws.ToString(item.Key)
				rel := strings.TrimPrefix(key, prefix)
				rel = strings.TrimPrefix(rel, "/")
				mt := time.Time{}
				if item.LastModified != nil {
					mt = *item.LastModified
				}
				out = append(out, StoredObject{Key: rel, ModTime: mt})
			}
			if page.IsTruncated != nil && *page.IsTruncated {
				token = page.NextContinuationToken
			} else {
				break
			}
		}
		return out, nil
	}

	var out []StoredObject
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过不可访问的条目
		}
		if info.IsDir() {
			return nil
		}
		rel, e := filepath.Rel(dir, p)
		if e != nil {
			return nil
		}
		out = append(out, StoredObject{Key: filepath.ToSlash(rel), ModTime: info.ModTime()})
		return nil
	})
	return out, err
}

// ============ 内部工具 ============

func maxBytes() int64 {
	if config.AppConfig != nil && config.AppConfig.Upload.MaxSizeMB > 0 {
		return int64(config.AppConfig.Upload.MaxSizeMB) * 1024 * 1024
	}
	return 10 * 1024 * 1024
}

// allowedSet 返回允许上传的扩展名集合。
func allowedSet() map[string]bool {
	m := map[string]bool{}
	src := ""
	if config.AppConfig != nil {
		src = config.AppConfig.Upload.Allowed
	}
	if src == "" {
		src = "jpg,jpeg,png,gif,webp,pdf,csv,xlsx"
	}
	for _, e := range strings.Split(src, ",") {
		e = strings.TrimSpace(strings.ToLower(e))
		if e != "" {
			m[e] = true
		}
	}
	return m
}

func checkAllowed(name string, data []byte) error {
	ct := http.DetectContentType(data)
	// 先按真实内容类型放行图片/PDF
	switch {
	case strings.HasPrefix(ct, "image/"), ct == "application/pdf":
		return nil
	}
	// 再按扩展名兜底
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
	if allowedSet()[ext] {
		return nil
	}
	return fmt.Errorf("不支持的文件类型: %s", ct)
}

func extFromNameAndData(name string, data []byte) string {
	if ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(name), ".")); ext != "" && allowedSet()[ext] {
		return "." + ext
	}
	ct := http.DetectContentType(data)
	switch {
	case strings.HasPrefix(ct, "image/jpeg"):
		return ".jpg"
	case strings.HasPrefix(ct, "image/png"):
		return ".png"
	case strings.HasPrefix(ct, "image/gif"):
		return ".gif"
	case strings.HasPrefix(ct, "image/webp"):
		return ".webp"
	case ct == "application/pdf":
		return ".pdf"
	}
	return ".bin"
}

func mimeType(path string, f *os.File) string {
	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	if n > 0 {
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			// 读失败则回退到扩展名
			return detectByExt(path)
		}
		return http.DetectContentType(buf[:n])
	}
	return detectByExt(path)
}

func detectByExt(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".pdf":
		return "application/pdf"
	}
	return "application/octet-stream"
}

func joinKey(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "/" + key
}

func randHex() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
