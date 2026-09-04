package handlers

import (
	"huozhi/internal/database"
	"huozhi/internal/dto"
	"huozhi/internal/middleware"
	"huozhi/internal/models"
	"huozhi/pkg/auth"
	"huozhi/pkg/jwt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Register 用户注册
func Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Bad(c, "参数错误: "+err.Error())
		return
	}

	// 检查用户名是否重复
	var count int64
	database.DB.Model(&models.User{}).Where("username = ?", req.Username).Count(&count)
	if count > 0 {
		Fail(c, 1001, "用户名已存在")
		return
	}
	if req.Email != "" {
		database.DB.Model(&models.User{}).Where("email = ?", req.Email).Count(&count)
		if count > 0 {
			Fail(c, 1002, "邮箱已注册")
			return
		}
	}

	// 哈希密码
	passHash, err := auth.HashPassword(req.Password)
	if err != nil {
		InternalErr(c, "密码加密失败")
		return
	}

	user := models.User{
		Username:     req.Username,
		PasswordHash: passHash,
		Nickname:     req.Nickname,
		Locale:       "zh-CN",
		Timezone:     "Asia/Shanghai",
		Currency:     "CNY",
		MonthStart:   1,
		LastLoginAt:  time.Now(),
		Status:       1,
	}
	// 空 email/phone 存 NULL：SQLite 唯一索引对 '' 视为重复，NULL 则不冲突
	if req.Email != "" {
		user.Email = &req.Email
	}
	if req.Phone != "" {
		user.Phone = &req.Phone
	}

	if user.Nickname == "" {
		user.Nickname = user.Username
	}

	if err := database.DB.Create(&user).Error; err != nil {
		InternalErr(c, "创建用户失败: "+err.Error())
		return
	}

	// 初始化默认账本和内置分类
	initUserDefaults(&user)

	token, err := jwt.GenerateToken(user.ID, user.Username)
	if err != nil {
		InternalErr(c, "生成token失败")
		return
	}

	Created(c, dto.LoginResponse{
		Token:    token,
		ExpireIn: 24 * 7 * 3600,
		User:     user,
	})
}

// initUserDefaults 初始化用户默认账本、分类
func initUserDefaults(user *models.User) {
	db := database.DB

	// 默认账本
	defaultBook := models.Book{
		UserID:    user.ID,
		Name:      "日常账本",
		Icon:      "📘",
		Color:     "#3B82F6",
		Currency:  user.Currency,
		IsDefault: true,
	}
	db.Create(&defaultBook)

	// 默认现金账户
	db.Create(&models.Account{
		UserID:         user.ID,
		BookID:         defaultBook.ID,
		Name:           "现金",
		Type:           models.AccCash,
		Currency:       user.Currency,
		Balance:        0,
		InitialAmount:  0,
		Icon:           "💵",
		Color:          "#10B981",
		IncludeInTotal: true,
		Sort:           1,
	})
	db.Create(&models.Account{
		UserID:         user.ID,
		BookID:         defaultBook.ID,
		Name:           "储蓄卡",
		Type:           models.AccBank,
		Currency:       user.Currency,
		Icon:           "💳",
		Color:          "#6366F1",
		IncludeInTotal: true,
		Sort:           2,
	})

	// 默认支出分类
	expenseCats := []struct {
		name string
		icon string
	}{
		{"餐饮", "🍚"}, {"交通", "🚌"}, {"购物", "🛒"}, {"娱乐", "🎮"},
		{"居家", "🏠"}, {"医疗", "💊"}, {"教育", "📚"}, {"通讯", "📞"},
		{"旅行", "✈️"}, {"人情", "🎁"}, {"运动", "⚽"}, {"宠物", "🐶"},
		{"其他支出", "📦"},
	}
	for i, cat := range expenseCats {
		db.Create(&models.Category{
			UserID:   user.ID,
			BookID:   defaultBook.ID,
			Name:     cat.name,
			Kind:     models.KindExpense,
			Icon:     cat.icon,
			IsSystem: false,
			Sort:     i + 1,
		})
	}

	// 默认收入分类
	incomeCats := []struct {
		name string
		icon string
	}{
		{"工资", "💼"}, {"奖金", "🎊"}, {"红包", "🧧"}, {"投资", "📈"},
		{"兼职", "💻"}, {"理财", "🏦"}, {"退款", "↩️"}, {"其他收入", "💰"},
	}
	for i, cat := range incomeCats {
		db.Create(&models.Category{
			UserID:   user.ID,
			BookID:   defaultBook.ID,
			Name:     cat.name,
			Kind:     models.KindIncome,
			Icon:     cat.icon,
			IsSystem: false,
			Sort:     i + 1,
		})
	}

	// 系统分类（转账）
	db.Create(&models.Category{
		UserID:   user.ID,
		BookID:   defaultBook.ID,
		Name:     "转账",
		Kind:     models.KindSystem,
		Icon:     "🔄",
		IsSystem: true,
		Sort:     1,
	})
	db.Create(&models.Category{
		UserID:   user.ID,
		BookID:   defaultBook.ID,
		Name:     "余额调整",
		Kind:     models.KindSystem,
		Icon:     "⚙️",
		IsSystem: true,
		Sort:     2,
	})
}

// Login 用户登录
func Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Bad(c, "参数错误: "+err.Error())
		return
	}

	var user models.User
	err := database.DB.Where("username = ? OR email = ? OR phone = ?", req.Username, req.Username, req.Username).First(&user).Error
	if err != nil {
		Fail(c, 1003, "用户不存在")
		return
	}

	if !auth.CheckPassword(req.Password, user.PasswordHash) {
		Fail(c, 1004, "密码错误")
		return
	}

	if user.Status != 1 {
		Fail(c, 1005, "账户已禁用")
		return
	}

	// 更新登录时间
	database.DB.Model(&user).Update("last_login_at", time.Now())

	token, err := jwt.GenerateToken(user.ID, user.Username)
	if err != nil {
		InternalErr(c, "生成token失败")
		return
	}

	OK(c, dto.LoginResponse{
		Token:    token,
		ExpireIn: 24 * 7 * 3600,
		User:     user,
	})
}

// Logout 登出（客户端清除token即可）
func Logout(c *gin.Context) {
	OK(c, nil)
}

// GetMe 获取当前用户信息
func GetMe(c *gin.Context) {
	uid := middleware.GetUID(c)
	var user models.User
	if err := database.DB.First(&user, uid).Error; err != nil {
		NotFound(c, "用户不存在")
		return
	}
	OK(c, user)
}

// strPtrOrNil 空字符串转 nil（存 NULL），非空返回指针
func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// UpdateMe 更新用户信息
func UpdateMe(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Bad(c, "参数错误: "+err.Error())
		return
	}

	if err := database.DB.Model(&models.User{}).Where("id = ?", uid).Updates(map[string]interface{}{
		"nickname": req.Nickname,
		"avatar":   req.Avatar,
		"email":    strPtrOrNil(req.Email),
		"phone":    strPtrOrNil(req.Phone),
		"locale":   req.Locale,
		"timezone": req.Timezone,
		"month_start": req.MonthStart,
		"currency":    req.Currency,
	}).Error; err != nil {
		InternalErr(c, "更新失败: "+err.Error())
		return
	}

	var user models.User
	database.DB.First(&user, uid)
	OK(c, user)
}

// ChangePassword 修改密码
func ChangePassword(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Bad(c, "参数错误: "+err.Error())
		return
	}

	var user models.User
	database.DB.First(&user, uid)
	if !auth.CheckPassword(req.OldPassword, user.PasswordHash) {
		Fail(c, 1006, "原密码错误")
		return
	}

	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		InternalErr(c, "密码加密失败")
		return
	}
	database.DB.Model(&user).Update("password_hash", hash)
	OK(c, nil)
}

// HealthCheck 健康检查
func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "time": time.Now().Format(time.RFC3339)})
}
