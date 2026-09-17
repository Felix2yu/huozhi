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
	_ "time/tzdata"

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

func AutoBackupRunner() {
	log.Println("[Cron] 自动备份调度器已启动")
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	runAutoBackups()
	for range tick.C {
		runAutoBackups()
	}
}

func runAutoBackups() {
	runAutoBackupsAt(time.Now())
}

func runAutoBackupsAt(now time.Time) {
	if database.DB == nil {
		return
	}

	var users []models.User
	if err := database.DB.Where("auto_backup_enabled = ?", true).Find(&users).Error; err != nil {
		log.Printf("[AutoBackup] 读取自动备份设置失败: %v", err)
		return
	}

	for _, u := range users {
		due, err := autoBackupDue(u, now)
		if err != nil {
			log.Printf("[AutoBackup] 用户 %d 调度设置无效: %v", u.ID, err)
			continue
		}
		if !due {
			continue
		}

		log.Printf("[AutoBackup] 用户 %d 开始自动备份", u.ID)
		path, err := GenerateAutoBackup(u.ID)
		if err != nil {
			log.Printf("[AutoBackup] 用户 %d 备份失败: %v", u.ID, err)
			continue
		}
		log.Printf("[AutoBackup] 用户 %d 备份完成: %s", u.ID, path)

		if err := database.DB.Model(&models.User{}).Where("id = ?", u.ID).Update("auto_backup_last_run", now).Error; err != nil {
			log.Printf("[AutoBackup] 用户 %d 保存最后执行时间失败: %v", u.ID, err)
		}

		// 清理旧备份
		keepCount := u.AutoBackupKeepCount
		if keepCount <= 0 {
			keepCount = 7
		}
		CleanupOldBackups(u.ID, keepCount)
	}
}

func autoBackupDue(u models.User, now time.Time) (bool, error) {
	if !u.AutoBackupEnabled {
		return false, nil
	}
	clock, err := time.Parse("15:04", u.AutoBackupTime)
	if err != nil || clock.Format("15:04") != u.AutoBackupTime {
		return false, fmt.Errorf("备份时间必须为 HH:MM")
	}
	zone := u.Timezone
	if zone == "" {
		zone = "Asia/Shanghai"
	}
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return false, fmt.Errorf("无效时区 %q: %w", zone, err)
	}
	now = now.In(loc)
	year, month, day := now.Date()
	switch u.AutoBackupFrequency {
	case "", "daily":
	case "weekly":
		day -= int(now.Weekday())
	case "monthly":
		day = 1
	default:
		return false, fmt.Errorf("无效备份频率 %q", u.AutoBackupFrequency)
	}
	scheduled := time.Date(year, month, day, clock.Hour(), clock.Minute(), 0, 0, loc)
	return !now.Before(scheduled) && u.AutoBackupLastRun.Before(scheduled), nil
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
