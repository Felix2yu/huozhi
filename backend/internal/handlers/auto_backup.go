package handlers

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"huozhi/internal/database"
	"huozhi/internal/middleware"
	"huozhi/internal/models"
	"huozhi/internal/storage"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// ========== 自动备份 ==========

const autoBackupDir = "backups"

// ensureBackupDir 确保备份目录存在
func ensureBackupDir() string {
	dir := filepath.Join(".", autoBackupDir)
	os.MkdirAll(dir, 0755)
	return dir
}

// GenerateAutoBackup 为指定用户生成全量备份并保存到磁盘
// 返回备份文件路径和错误
func GenerateAutoBackup(userID uint) (string, error) {
	snap := backupSnapshot{Version: "2.0", ExportedAt: time.Now()}
	database.DB.Where("user_id = ?", userID).Find(&snap.Books)
	database.DB.Where("user_id = ?", userID).Find(&snap.Accounts)
	database.DB.Where("user_id = ?", userID).Find(&snap.Categories)
	database.DB.Where("user_id = ?", userID).Find(&snap.Tags)
	database.DB.Where("user_id = ?", userID).Find(&snap.Budgets)
	database.DB.Preload("Tags").Where("user_id = ?", userID).Find(&snap.Transactions)
	database.DB.Where("user_id = ?", userID).Find(&snap.SavingPlans)
	database.DB.Where("user_id = ?", userID).Find(&snap.SavingRecords)
	database.DB.Where("user_id = ?", userID).Find(&snap.Recurrings)
	database.DB.Where("user_id = ?", userID).Find(&snap.Installments)
	database.DB.Where("user_id = ?", userID).Find(&snap.Reimbursements)

	// 收集图片
	type imageEntry struct {
		zipPath string
		key     string
	}
	var images []imageEntry
	seen := map[string]bool{}
	for _, tx := range snap.Transactions {
		for _, imgPath := range tx.Images {
			key, ok := storage.KeyFromURL(imgPath)
			if !ok || seen[key] {
				continue
			}
			seen[key] = true
			images = append(images, imageEntry{zipPath: "images/" + key, key: key})
		}
	}

	// 创建 ZIP
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	jsonData, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return "", fmt.Errorf("序列化备份失败: %w", err)
	}
	f, err := zipWriter.Create("backup.json")
	if err != nil {
		return "", fmt.Errorf("创建backup.json失败: %w", err)
	}
	if _, err := f.Write(jsonData); err != nil {
		return "", fmt.Errorf("写入备份失败: %w", err)
	}

	for _, img := range images {
		rc, _, err := storage.Open(img.key)
		if err != nil {
			continue
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			continue
		}
		f, err := zipWriter.Create(img.zipPath)
		if err != nil {
			continue
		}
		f.Write(data)
	}
	zipWriter.Close()

	// 保存到磁盘
	dir := ensureBackupDir()
	filename := fmt.Sprintf("auto-%d-%s.zip", userID, time.Now().Format("20060102-150405"))
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		return "", fmt.Errorf("写入备份文件失败: %w", err)
	}

	return path, nil
}

// CleanupOldBackups 删除超过保留份数的旧自动备份
func CleanupOldBackups(userID uint, keepCount int) {
	dir := ensureBackupDir()
	prefix := fmt.Sprintf("auto-%d-", userID)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	var matches []os.DirEntry
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), prefix) && strings.HasSuffix(e.Name(), ".zip") {
			matches = append(matches, e)
		}
	}

	// 按文件名排序（包含时间戳，天然时间序）
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Name() > matches[j].Name()
	})

	// 删除超出保留份数的旧文件
	if len(matches) > keepCount {
		for _, e := range matches[keepCount:] {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

// AutoBackupRunner 后台定时检查并执行自动备份
// 每分钟检查一次，匹配用户的 backup_time 和 frequency
func AutoBackupRunner() {
	log.Println("[Cron] 自动备份调度器已启动")
	tick := time.NewTicker(60 * time.Second)
	defer tick.Stop()
	for range tick.C {
		runAutoBackups()
	}
}

func runAutoBackups() {
	if database.DB == nil {
		return
	}

	now := time.Now()
	currentTime := now.Format("15:04") // 当前时间 HH:MM

	var users []models.User
	database.DB.Where("auto_backup_enabled = ? AND auto_backup_time = ?", true, currentTime).Find(&users)

	for _, u := range users {
		// 检查频率：是否该今天执行
		if !shouldBackupToday(u, now) {
			continue
		}

		log.Printf("[AutoBackup] 用户 %d 开始自动备份", u.ID)
		path, err := GenerateAutoBackup(u.ID)
		if err != nil {
			log.Printf("[AutoBackup] 用户 %d 备份失败: %v", u.ID, err)
			continue
		}
		log.Printf("[AutoBackup] 用户 %d 备份完成: %s", u.ID, path)

		// 更新最后执行时间
		database.DB.Model(&models.User{}).Where("id = ?", u.ID).Update("auto_backup_last_run", now)

		// 清理旧备份
		keepCount := u.AutoBackupKeepCount
		if keepCount <= 0 {
			keepCount = 7
		}
		CleanupOldBackups(u.ID, keepCount)
	}
}

// shouldBackupToday 根据频率判断今天是否应执行备份
func shouldBackupToday(u models.User, now time.Time) bool {
	switch u.AutoBackupFrequency {
	case "daily":
		return true
	case "weekly":
		// 每周日执行
		return now.Weekday() == time.Sunday
	case "monthly":
		// 每月1号执行
		return now.Day() == 1
	default:
		return true
	}
}

// ListAutoBackups 列出用户的自动备份文件
func ListAutoBackups(userID uint) []map[string]interface{} {
	dir := ensureBackupDir()
	prefix := fmt.Sprintf("auto-%d-", userID)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var result []map[string]interface{}
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), prefix) && strings.HasSuffix(e.Name(), ".zip") {
			info, _ := e.Info()
			result = append(result, map[string]interface{}{
				"name": e.Name(),
				"size": info.Size(),
				"time": info.ModTime(),
			})
		}
	}

	// 按时间倒序
	sort.Slice(result, func(i, j int) bool {
		return result[i]["name"].(string) > result[j]["name"].(string)
	})

	return result
}

// ListAutoBackup GET /io/auto-backups —— 获取自动备份列表
func ListAutoBackup(c *gin.Context) {
	uid := middleware.GetUID(c)
	backups := ListAutoBackups(uid)
	if backups == nil {
		backups = []map[string]interface{}{}
	}
	OK(c, backups)
}
