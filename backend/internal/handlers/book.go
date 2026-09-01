package handlers

import (
	"huozhi/internal/database"
	"huozhi/internal/dto"
	"huozhi/internal/middleware"
	"huozhi/internal/models"

	"github.com/gin-gonic/gin"
)

// ========== 账本 Book ==========

// ListBooks 账本列表
func ListBooks(c *gin.Context) {
	uid := middleware.GetUID(c)
	var books []models.Book
	database.DB.Where("user_id = ?", uid).Order("is_default DESC, sort ASC, id DESC").Find(&books)
	OK(c, books)
}

// GetBook 获取单个账本
func GetBook(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.IDRequest
	if err := c.ShouldBindUri(&req); err != nil {
		Bad(c, err.Error())
		return
	}
	var book models.Book
	if err := database.DB.Where("id = ? AND user_id = ?", req.ID, uid).First(&book).Error; err != nil {
		NotFound(c, "账本不存在")
		return
	}
	OK(c, book)
}

// CreateBook 创建账本
func CreateBook(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.CreateBookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Bad(c, "参数错误: "+err.Error())
		return
	}

	if req.IsDefault {
		database.DB.Model(&models.Book{}).Where("user_id = ? AND is_default = ?", uid, true).Update("is_default", false)
	}

	book := models.Book{
		UserID:      uid,
		Name:        req.Name,
		Icon:        req.Icon,
		Color:       req.Color,
		Description: req.Description,
		Currency:    firstNotEmpty(req.Currency, "CNY"),
		IsDefault:   req.IsDefault,
		Sort:        req.Sort,
	}

	if err := database.DB.Create(&book).Error; err != nil {
		InternalErr(c, "创建失败: "+err.Error())
		return
	}

	// 新账本也初始化系统分类
	cats := []models.Category{
		{UserID: uid, BookID: book.ID, Name: "转账", Kind: models.KindSystem, Icon: "🔄", IsSystem: true, Sort: 1},
		{UserID: uid, BookID: book.ID, Name: "余额调整", Kind: models.KindSystem, Icon: "⚙️", IsSystem: true, Sort: 2},
	}
	database.DB.Create(&cats)

	Created(c, book)
}

// UpdateBook 更新账本
func UpdateBook(c *gin.Context) {
	uid := middleware.GetUID(c)
	var reqUri dto.IDRequest
	if err := c.ShouldBindUri(&reqUri); err != nil {
		Bad(c, err.Error())
		return
	}
	var req dto.UpdateBookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Bad(c, "参数错误: "+err.Error())
		return
	}

	if req.IsDefault {
		database.DB.Model(&models.Book{}).Where("user_id = ? AND is_default = ?", uid, true).Update("is_default", false)
	}

	updates := map[string]interface{}{
		"name":        req.Name,
		"icon":        req.Icon,
		"color":       req.Color,
		"description": req.Description,
		"currency":    req.Currency,
		"is_default":  req.IsDefault,
		"sort":        req.Sort,
	}
	if err := database.DB.Model(&models.Book{}).Where("id = ? AND user_id = ?", reqUri.ID, uid).Updates(updates).Error; err != nil {
		InternalErr(c, "更新失败: "+err.Error())
		return
	}

	var book models.Book
	database.DB.First(&book, reqUri.ID)
	OK(c, book)
}

// DeleteBook 删除账本
func DeleteBook(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.IDRequest
	if err := c.ShouldBindUri(&req); err != nil {
		Bad(c, err.Error())
		return
	}
	var book models.Book
	database.DB.Where("id = ? AND user_id = ?", req.ID, uid).First(&book)
	if book.IsDefault {
		Fail(c, 2001, "不能删除默认账本")
		return
	}
	database.DB.Delete(&book)
	OK(c, nil)
}

// ========== 账本成员 ==========

// ListBookMembers 成员列表
func ListBookMembers(c *gin.Context) {
	uid := middleware.GetUID(c)
	var reqUri dto.IDRequest
	c.ShouldBindUri(&reqUri)

	// 检查成员资格
	var mem models.BookMember
	if database.DB.Where("book_id = ? AND user_id = ?", reqUri.ID, uid).Take(&mem).Error != nil {
		// 若是账本拥有者也可以
		var book models.Book
		if database.DB.Where("id = ? AND user_id = ?", reqUri.ID, uid).Take(&book).Error != nil {
			Forbidden(c, "无权限")
			return
		}
	}

	var members []models.BookMember
	database.DB.Where("book_id = ?", reqUri.ID).Find(&members)
	OK(c, members)
}

// InviteBookMember 邀请成员
func InviteBookMember(c *gin.Context) {
	uid := middleware.GetUID(c)
	var reqUri dto.IDRequest
	c.ShouldBindUri(&reqUri)

	var book models.Book
	if err := database.DB.Where("id = ? AND user_id = ?", reqUri.ID, uid).First(&book).Error; err != nil {
		Forbidden(c, "只有账本拥有者可邀请成员")
		return
	}

	var req dto.InviteMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Bad(c, "参数错误: "+err.Error())
		return
	}

	var targetUser models.User
	if err := database.DB.Where("username = ? OR email = ?", req.Username, req.Username).First(&targetUser).Error; err != nil {
		Fail(c, 2002, "用户不存在")
		return
	}
	if targetUser.ID == uid {
		Fail(c, 2003, "不能邀请自己")
		return
	}

	mem := models.BookMember{
		BookID: reqUri.ID,
		UserID: targetUser.ID,
		Role:   req.Role,
	}
	if err := database.DB.Create(&mem).Error; err != nil {
		Fail(c, 2004, "该用户已是成员")
		return
	}
	Created(c, mem)
}

func firstNotEmpty(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
