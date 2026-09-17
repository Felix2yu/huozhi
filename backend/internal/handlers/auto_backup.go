package handlers

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"huozhi/internal/database"
	"huozhi/internal/middleware"
	"huozhi/internal/models"
	"huozhi/internal/storage"
	"io"
	"log"
	"mime"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
)

// ========== 自动备份 ==========

func GenerateAutoBackup(userID uint) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	return generateAutoBackup(ctx, userID)
}

func generateAutoBackup(ctx context.Context, userID uint) (string, error) {
	snap := backupSnapshot{Version: "2.0", ExportedAt: time.Now()}
	for _, dest := range []any{&snap.Books, &snap.Accounts, &snap.Categories, &snap.Tags, &snap.Budgets, &snap.SavingPlans, &snap.SavingRecords, &snap.Recurrings, &snap.Installments, &snap.Reimbursements} {
		if err := database.DB.WithContext(ctx).Where("user_id = ?", userID).Find(dest).Error; err != nil {
			return "", fmt.Errorf("读取备份数据失败: %w", err)
		}
	}
	if err := database.DB.WithContext(ctx).Preload("Tags").Where("user_id = ?", userID).Find(&snap.Transactions).Error; err != nil {
		return "", fmt.Errorf("读取交易失败: %w", err)
	}

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
		if err := ctx.Err(); err != nil {
			return "", err
		}
		rc, _, err := storage.Open(img.key)
		if err != nil {
			return "", fmt.Errorf("读取备份图片失败: %w", err)
		}
		f, err := zipWriter.Create(img.zipPath)
		if err != nil {
			rc.Close()
			return "", err
		}
		_, err = io.Copy(f, rc)
		rc.Close()
		if err != nil {
			return "", err
		}
	}
	if err := zipWriter.Close(); err != nil {
		return "", err
	}
	filename := fmt.Sprintf("auto-%d-%s.zip", userID, time.Now().Format("20060102-150405.000000000"))
	return storage.SaveBackup(ctx, userID, filename, buf.Bytes())
}

// CleanupOldBackups 删除超过保留份数的旧自动备份
func CleanupOldBackups(userID uint, keepCount int) {
	if keepCount <= 0 {
		keepCount = 7
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	backups, err := storage.ListBackups(ctx, userID)
	if err != nil {
		log.Printf("[AutoBackup] 用户 %d 列举失败: %v", userID, err)
		return
	}
	for i := keepCount; i < len(backups); i++ {
		if err := storage.DeleteBackup(ctx, userID, backups[i].Name); err != nil {
			log.Printf("[AutoBackup] 用户 %d 清理失败: %v", userID, err)
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

func ListAutoBackup(c *gin.Context) {
	backups, err := storage.ListBackups(c.Request.Context(), middleware.GetUID(c))
	if err != nil {
		InternalErr(c, "读取备份列表失败")
		return
	}
	OK(c, backups)
}

func CreateAutoBackup(c *gin.Context) {
	uid := middleware.GetUID(c)
	location, err := generateAutoBackup(c.Request.Context(), uid)
	if err != nil {
		log.Printf("[AutoBackup] 用户 %d 立即备份失败: %v", uid, err)
		InternalErr(c, "备份失败，请检查存储配置和图片是否可读取")
		return
	}
	backend := "local"
	if storage.UsingS3() {
		backend = "s3"
	}
	Created(c, gin.H{"name": filepath.Base(location), "storage": backend})
}

func DownloadAutoBackup(c *gin.Context) {
	name := c.Param("name")
	rc, err := storage.OpenBackup(c.Request.Context(), middleware.GetUID(c), name)
	if err != nil {
		NotFound(c, "备份不存在或暂时无法读取")
		return
	}
	defer rc.Close()
	c.Header("Cache-Control", "private, no-store")
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	_, _ = io.Copy(c.Writer, rc)
}
