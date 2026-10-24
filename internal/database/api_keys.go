package database

import (
	"log"

	"huozhi/internal/models"

	"gorm.io/gorm"
)

// MigrateAPIKeys 把历史上的单把 users.api_key 迁成 api_keys 表里的一把读写密钥。
//
// 背景：密钥原本是 users 表上的一个字段，一个账号只有一把。要区分「MCP 要能写、
// 牵丝/幕间只能读」就必须支持多把带 scope 的密钥，否则权限只能取并集。
//
// 迁移策略（用户确认）：
//   - 旧密钥一律按「读写」迁移，现有的 MCP 客户端配置无需改动即可继续工作；
//   - 名称固定为「默认密钥（读写）」，便于在设置页识别；
//   - users.api_key 原列**保留不清空**，旧版本二进制回滚后仍能读到，
//     但新代码只认 api_keys 表。
//
// 幂等与防复活：按 key 值查（含软删除），已存在就跳过。这样用户在设置页吊销过
// 的旧密钥不会因为下次启动又被 users.api_key 重新灌回来——吊销用的是软删除，
// 记录还在，只是不再出现在列表里。
func MigrateAPIKeys(db *gorm.DB) {
	if db == nil {
		return
	}
	var users []models.User
	if err := db.Where("api_key IS NOT NULL AND api_key <> ''").Find(&users).Error; err != nil {
		log.Printf("[DB] 读取历史 API 密钥失败: %v", err)
		return
	}
	migrated := 0
	for _, u := range users {
		if u.APIKey == nil || *u.APIKey == "" {
			continue
		}
		var exists int64
		// Unscoped：软删除过的也算「已处理」，否则吊销后的密钥会在重启时复活
		if err := db.Unscoped().Model(&models.APIKey{}).Where("key = ?", *u.APIKey).Count(&exists).Error; err != nil {
			log.Printf("[DB] 检查 API 密钥是否存在失败: %v", err)
			continue
		}
		if exists > 0 {
			continue
		}
		row := models.APIKey{
			UserID:  u.ID,
			Name:    "默认密钥（读写）",
			Key:     *u.APIKey,
			Scope:   models.ScopeWrite,
			Enabled: u.APIKeyEnabled,
			Legacy:  true,
		}
		if err := db.Create(&row).Error; err != nil {
			log.Printf("[DB] 迁移历史 API 密钥失败(user=%d): %v", u.ID, err)
			continue
		}
		migrated++
	}
	if migrated > 0 {
		log.Printf("[DB] 已把 %d 把历史 API 密钥迁移到 api_keys（读写权限）", migrated)
	}
}
