package handlers

import (
	"io"
	"log"
	"strconv"
	"strings"

	"huozhi/internal/database"
	"huozhi/internal/middleware"
	"huozhi/internal/models"

	"github.com/gin-gonic/gin"
)

// 密钥管理（需要 JWT 登录）。
//
// 一个账号可以持有多把密钥，每把带 scope：
//   - read：只能取数。牵丝、幕间这类「只读消费方」用这把；
//   - write：可取数，也能通过 MCP 建/改/删账单。给 AI 客户端用。
//
// 密钥明文返回（用户确认沿用明文存储），因此列表接口本身就等于「可复制」；
// 设置页默认打码，点眼睛才显示。

const (
	// maxAPIKeysPerUser 单账号密钥上限。纯防手滑刷屏，不是安全边界。
	maxAPIKeysPerUser = 20
	apiKeyNameMaxLen  = 50
)

// ListAPIKeys 当前账号的密钥列表，按创建顺序。
func ListAPIKeys(c *gin.Context) {
	uid := middleware.GetUID(c)
	var rows []models.APIKey
	if err := database.DB.Where("user_id = ?", uid).Order("id ASC").Find(&rows).Error; err != nil {
		InternalErr(c, "读取密钥列表失败")
		return
	}
	OK(c, rows)
}

type createAPIKeyReq struct {
	Name  string `json:"name"`
	Scope string `json:"scope"`
}

// CreateAPIKey 新建一把密钥。scope 缺省为只读——新建密钥最常见的目的
// 是接一个只取数的应用，默认给写权限属于「默认值选错方向」。
func CreateAPIKey(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req createAPIKeyReq
	if err := c.ShouldBindJSON(&req); err != nil && err != io.EOF {
		Bad(c, "请求格式错误")
		return
	}

	scope := strings.TrimSpace(req.Scope)
	switch scope {
	case "":
		scope = models.ScopeRead
	case models.ScopeRead, models.ScopeWrite:
	default:
		Bad(c, "scope 只能是 read 或 write")
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "未命名密钥"
	}
	if len([]rune(name)) > apiKeyNameMaxLen {
		Bad(c, "密钥名称过长")
		return
	}

	var count int64
	if err := database.DB.Model(&models.APIKey{}).Where("user_id = ?", uid).Count(&count).Error; err != nil {
		InternalErr(c, "读取密钥数量失败")
		return
	}
	if count >= maxAPIKeysPerUser {
		Bad(c, "密钥数量已达上限，请先吊销不用的密钥")
		return
	}

	row := models.APIKey{
		UserID:  uid,
		Name:    name,
		Key:     middleware.GenerateAPIKey(),
		Scope:   scope,
		Enabled: true,
	}
	if err := database.DB.Create(&row).Error; err != nil {
		InternalErr(c, "生成密钥失败")
		return
	}
	log.Printf("[APIKey] 用户 %d 新建密钥 %d（%s / %s）", uid, row.ID, row.Name, row.Scope)
	OK(c, row)
}

type updateAPIKeyReq struct {
	Name    *string `json:"name"`
	Enabled *bool   `json:"enabled"`
	// Scope 允许改：先建成只读、后来发现要接 MCP，改 scope 比换密钥省事。
	Scope *string `json:"scope"`
}

// UpdateAPIKey 改名 / 停用 / 调整权限。停用立即失效，但记录保留可恢复。
func UpdateAPIKey(c *gin.Context) {
	uid := middleware.GetUID(c)
	row, ok := findAPIKey(c, uid)
	if !ok {
		return
	}

	var req updateAPIKeyReq
	if err := c.ShouldBindJSON(&req); err != nil && err != io.EOF {
		Bad(c, "请求格式错误")
		return
	}

	updates := map[string]any{}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			Bad(c, "密钥名称不能为空")
			return
		}
		if len([]rune(name)) > apiKeyNameMaxLen {
			Bad(c, "密钥名称过长")
			return
		}
		updates["name"] = name
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if req.Scope != nil {
		scope := strings.TrimSpace(*req.Scope)
		if scope != models.ScopeRead && scope != models.ScopeWrite {
			Bad(c, "scope 只能是 read 或 write")
			return
		}
		updates["scope"] = scope
	}
	if len(updates) == 0 {
		OK(c, row)
		return
	}
	if err := database.DB.Model(&models.APIKey{}).Where("id = ?", row.ID).Updates(updates).Error; err != nil {
		InternalErr(c, "更新密钥失败")
		return
	}
	log.Printf("[APIKey] 用户 %d 更新密钥 %d：%v", uid, row.ID, updates)
	database.DB.First(&row, row.ID)
	OK(c, row)
}

// DeleteAPIKey 吊销密钥（软删除）。软删除而非物理删除有两个作用：
// 记录可追溯；且迁移逻辑据此判断「这把旧密钥已经处理过」，不会在重启时复活。
func DeleteAPIKey(c *gin.Context) {
	uid := middleware.GetUID(c)
	row, ok := findAPIKey(c, uid)
	if !ok {
		return
	}
	if err := database.DB.Delete(&models.APIKey{}, row.ID).Error; err != nil {
		InternalErr(c, "吊销密钥失败")
		return
	}
	log.Printf("[APIKey] 用户 %d 吊销密钥 %d（%s）", uid, row.ID, row.Name)
	OK(c, gin.H{"deleted": true})
}

// findAPIKey 按路径参数取密钥，并校验归属——越权一律按不存在处理，
// 避免用 403/404 的差异探测别人有没有某个 id。
func findAPIKey(c *gin.Context, uid uint) (models.APIKey, bool) {
	var row models.APIKey
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		NotFound(c, "密钥不存在")
		return row, false
	}
	if err := database.DB.Where("id = ? AND user_id = ?", uint(id), uid).First(&row).Error; err != nil {
		NotFound(c, "密钥不存在")
		return row, false
	}
	return row, true
}
