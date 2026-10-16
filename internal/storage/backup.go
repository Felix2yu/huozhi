package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type BackupObject struct {
	Name string    `json:"name"`
	Size int64     `json:"size"`
	Time time.Time `json:"time"`
}

type backupStore struct {
	client *s3.Client
	bucket string
	prefix string
	dir    string
}

func backupState(ctx context.Context) (backupStore, error) {
	if err := ctx.Err(); err != nil {
		return backupStore{}, err
	}
	mu.RLock()
	defer mu.RUnlock()
	if !inited {
		return backupStore{}, fmt.Errorf("存储未初始化")
	}
	return backupStore{client: s3Client, bucket: s3Bucket, prefix: s3Prefix, dir: backupDir}, nil
}

func validBackupName(uid uint, name string) bool {
	prefix := "auto-" + strconv.FormatUint(uint64(uid), 10) + "-"
	if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".zip") || len(name) > 255 {
		return false
	}
	suffix := strings.TrimSuffix(strings.TrimPrefix(name, prefix), ".zip")
	if suffix == "" || strings.Contains(suffix, "..") {
		return false
	}
	for _, c := range suffix {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.') {
			return false
		}
	}
	return true
}

func (s backupStore) userPrefix(uid uint) string {
	return joinKey(s.prefix, "backups/"+strconv.FormatUint(uint64(uid), 10)+"/")
}

func SaveBackup(ctx context.Context, uid uint, name string, data []byte) (string, error) {
	s, err := backupState(ctx)
	if err != nil {
		return "", err
	}
	if !validBackupName(uid, name) {
		return "", fmt.Errorf("非法备份名称")
	}
	if s.client != nil {
		key := s.userPrefix(uid) + name
		_, err := s.client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key), Body: bytes.NewReader(data), ContentType: aws.String("application/zip")})
		if err != nil {
			return "", err
		}
		return key, nil
	}
	if err := privateBackupDir(s.dir); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(s.dir, ".backup-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	path := filepath.Join(s.dir, name)
	if err := os.Rename(f.Name(), path); err != nil {
		return "", err
	}
	return path, nil
}

func ListBackups(ctx context.Context, uid uint) ([]BackupObject, error) {
	s, err := backupState(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]BackupObject, 0)
	if s.client != nil {
		prefix := s.userPrefix(uid)
		var token *string
		seen := map[string]bool{}
		for {
			page, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: aws.String(s.bucket), Prefix: aws.String(prefix), ContinuationToken: token})
			if err != nil {
				return nil, err
			}
			for _, item := range page.Contents {
				key := aws.ToString(item.Key)
				name := strings.TrimPrefix(key, prefix)
				if !strings.HasPrefix(key, prefix) || !validBackupName(uid, name) {
					continue
				}
				out = append(out, BackupObject{Name: name, Size: aws.ToInt64(item.Size), Time: aws.ToTime(item.LastModified)})
			}
			if !aws.ToBool(page.IsTruncated) {
				break
			}
			next := aws.ToString(page.NextContinuationToken)
			if next == "" || seen[next] {
				return nil, fmt.Errorf("S3 分页 token 无效")
			}
			seen[next] = true
			token = page.NextContinuationToken
		}
	} else {
		if err := noSymlinks(s.dir); err != nil {
			if os.IsNotExist(err) {
				return out, nil
			}
			return nil, err
		}
		entries, err := os.ReadDir(s.dir)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if !validBackupName(uid, entry.Name()) {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				return nil, err
			}
			if info.Mode().IsRegular() {
				out = append(out, BackupObject{Name: entry.Name(), Size: info.Size(), Time: info.ModTime()})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name > out[j].Name })
	return out, nil
}

func OpenBackup(ctx context.Context, uid uint, name string) (io.ReadCloser, error) {
	s, err := backupState(ctx)
	if err != nil {
		return nil, err
	}
	if !validBackupName(uid, name) {
		return nil, fmt.Errorf("非法备份名称")
	}
	if s.client != nil {
		out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(s.userPrefix(uid) + name)})
		if err != nil {
			return nil, err
		}
		return out.Body, nil
	}
	path := filepath.Join(s.dir, name)
	if err := noSymlinks(path); err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("备份不是普通文件")
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return nil, err
	}
	return os.Open(path)
}

func DeleteBackup(ctx context.Context, uid uint, name string) error {
	s, err := backupState(ctx)
	if err != nil {
		return err
	}
	if !validBackupName(uid, name) {
		return fmt.Errorf("非法备份名称")
	}
	if s.client != nil {
		_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(s.userPrefix(uid) + name)})
		return err
	}
	path := filepath.Join(s.dir, name)
	if err := noSymlinks(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("备份不是普通文件")
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func privateBackupDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := noSymlinks(dir); err != nil {
		return err
	}
	return os.Chmod(dir, 0o700)
}

func noSymlinks(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("不允许符号链接: %s", path)
	}
	return nil
}

func withinPath(parent, child string) bool {
	parent, err := filepath.Abs(parent)
	if err != nil {
		return true
	}
	child, err = filepath.Abs(child)
	if err != nil {
		return true
	}
	if resolved, err := filepath.EvalSymlinks(parent); err == nil {
		parent = resolved
	}
	if resolved, err := filepath.EvalSymlinks(child); err == nil {
		child = resolved
	}
	rel, err := filepath.Rel(parent, child)
	return err != nil || rel == "." || rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func validPublicKey(key string) bool {
	if key == "" || strings.ContainsAny(key, "\\\x00%") || strings.Contains(key, "..") {
		return false
	}
	for _, part := range strings.Split(key, "/") {
		if part == "" || part == "." || strings.EqualFold(part, "backups") || strings.HasPrefix(part, "auto-") && strings.HasSuffix(part, ".zip") {
			return false
		}
	}
	return true
}

func publicLocalPath(dir, key string) (string, error) {
	if dir == "" || !validPublicKey(key) {
		return "", fmt.Errorf("非法图片路径")
	}
	resolvedDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", err
	}
	path := resolvedDir
	for _, part := range strings.Split(key, "/") {
		path = filepath.Join(path, part)
		if err := noSymlinks(path); err != nil {
			return "", err
		}
	}
	mu.RLock()
	backups := backupDir
	mu.RUnlock()
	if backups != "" && withinPath(backups, path) {
		return "", fmt.Errorf("不能通过图片接口访问备份")
	}
	return path, nil
}
