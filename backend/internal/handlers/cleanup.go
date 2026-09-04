package handlers

import (
	"encoding/json"
	"time"

	"huozhi/internal/config"
	"huozhi/internal/database"
	"huozhi/internal/models"
	"huozhi/internal/storage"

	"github.com/gin-gonic/gin"
)

// RunOrphanCleanup 收集所有仍被引用的附件 key，并清理存储中未被引用的孤儿文件（宽限期外）。
// 供后台定时任务与手动触发接口共用。
func RunOrphanCleanup() (int, error) {
	if database.DB == nil {
		return 0, nil
	}
	referenced := referencedImageKeys()
	return storage.CleanupOrphans(referenced, orphanGrace())
}

// CleanupOrphanUploads 手动触发孤儿附件清理（需登录）。
func CleanupOrphanUploads(c *gin.Context) {
	n, err := RunOrphanCleanup()
	if err != nil {
		InternalErr(c, "清理失败: "+err.Error())
		return
	}
	OK(c, gin.H{"deleted": n})
}

// referencedImageKeys 收集所有仍被引用的附件内部 key。
// 来源：交易图片（transactions.images，JSON 数组）+ 用户头像（users.avatar）。
// 若后续有新的附件消费方（如账本封面），需在此追加，否则其文件会被当作孤儿清理。
func referencedImageKeys() map[string]bool {
	set := map[string]bool{}

	// 1) 交易图片
	var imgCols []string
	database.DB.Model(&models.Transaction{}).Select("images").Scan(&imgCols)
	for _, col := range imgCols {
		if col == "" || col == "null" || col == "[]" {
			continue
		}
		var urls []string
		if err := json.Unmarshal([]byte(col), &urls); err != nil {
			continue
		}
		for _, u := range urls {
			if k, ok := storage.KeyFromURL(u); ok {
				set[k] = true
			}
		}
	}

	// 2) 用户头像（通用上传接口也可能用于头像）
	var avCols []string
	database.DB.Model(&models.User{}).Select("avatar").Where("avatar LIKE ?", "/api/uploads/%").Scan(&avCols)
	for _, u := range avCols {
		if k, ok := storage.KeyFromURL(u); ok {
			set[k] = true
		}
	}

	return set
}

// orphanGrace 返回孤儿文件宽限期（默认 60 分钟）。
func orphanGrace() time.Duration {
	m := 60
	if config.AppConfig != nil && config.AppConfig.Upload.OrphanGraceMinutes > 0 {
		m = config.AppConfig.Upload.OrphanGraceMinutes
	}
	return time.Duration(m) * time.Minute
}
