package handlers

import (
	"fmt"
	"strings"
	"time"

	"huozhi/internal/database"
	"huozhi/internal/dto"
	"huozhi/internal/middleware"
	"huozhi/internal/models"

	"github.com/gin-gonic/gin"
)

// ========== 账本 Book ==========

// ListBooks 账本列表（含被邀请加入的共享账本）
func ListBooks(c *gin.Context) {
	uid := middleware.GetUID(c)
	var books []models.Book
	q := database.DB.Where("is_archived = ?", false)
	ids := accessibleBookIDs(uid)
	if len(ids) > 0 {
		// C5：共享账本此前只按 user_id 过滤，被邀请成员永远看不到该账本。
		// 括号不可省：后续 Where 用 AND 连接，OR 不加括号会被优先级吞掉。
		q = q.Where("(user_id = ? OR id IN ?)", uid, ids)
	} else {
		q = q.Where("user_id = ?", uid)
	}
	q.Order("is_default DESC, sort ASC, id DESC").Find(&books)

	// 标注共享来源与当前用户在其中的角色，供前端区分「我的 / 共享」
	type bookView struct {
		models.Book
		IsShared bool   `json:"is_shared"`
		Role     string `json:"role"`
	}
	roleMap := map[uint]string{}
	if len(ids) > 0 {
		var mems []models.BookMember
		database.DB.Where("user_id = ?", uid).Find(&mems)
		for _, m := range mems {
			roleMap[m.BookID] = m.Role
		}
	}
	out := make([]bookView, 0, len(books))
	for _, b := range books {
		v := bookView{Book: b}
		if r, ok := roleMap[b.ID]; ok {
			v.IsShared = true
			v.Role = r
		} else {
			v.Role = "owner"
		}
		out = append(out, v)
	}
	OK(c, out)
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

	Broadcast(c, "books", "create", book.ID)
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
		"is_archived": req.IsArchived, // C11：此前 updates 里没有该字段，账本无法归档
		"sort":        req.Sort,
	}
	if err := database.DB.Model(&models.Book{}).Where("id = ? AND user_id = ?", reqUri.ID, uid).Updates(updates).Error; err != nil {
		InternalErr(c, "更新失败: "+err.Error())
		return
	}

	var book models.Book
	database.DB.First(&book, reqUri.ID)
	Broadcast(c, "books", "update", book.ID)
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
	if book.ID == 0 {
		NotFound(c, "账本不存在")
		return
	}
	if book.IsDefault {
		Fail(c, 2001, "不能删除默认账本")
		return
	}

	// C11：直接软删会让其下交易/账户/分类变成 book_id 悬空的孤儿记录，
	// 在「全部账本」视图里依然出现却无法归类。这里先统计并给出处置方案：
	//   ?migrate_to=<bookID>  把子数据迁移到指定账本后再删除
	//   ?force=1              连同子数据一并软删（需客户端二次确认）
	txs, accounts, categories := countBookChildren(database.DB, uid, req.ID)
	if txs+accounts+categories > 0 {
		if mig := c.Query("migrate_to"); mig != "" {
			var mid uint
			if _, err := fmt.Sscanf(mig, "%d", &mid); err != nil || mid == 0 {
				Bad(c, "migrate_to 参数非法")
				return
			}
			var target models.Book
			if err := database.DB.Where("id = ? AND user_id = ?", mid, uid).First(&target).Error; err != nil {
				Bad(c, "目标账本不存在")
				return
			}
			database.DB.Model(&models.Transaction{}).
				Where("user_id = ? AND book_id = ?", uid, req.ID).Update("book_id", mid)
			database.DB.Model(&models.Account{}).
				Where("user_id = ? AND book_id = ?", uid, req.ID).Update("book_id", mid)
			database.DB.Model(&models.Category{}).
				Where("user_id = ? AND book_id = ?", uid, req.ID).Update("book_id", mid)
		} else if c.Query("force") != "1" {
			Fail(c, 2005, fmt.Sprintf(
				"该账本下还有 %d 笔交易、%d 个账户、%d 个分类。请先迁移（migrate_to）或确认一并删除（force=1）。",
				txs, accounts, categories))
			return
		} else {
			database.DB.Where("user_id = ? AND book_id = ?", uid, req.ID).Delete(&models.Transaction{})
			database.DB.Where("user_id = ? AND book_id = ?", uid, req.ID).Delete(&models.Account{})
			database.DB.Where("user_id = ? AND book_id = ?", uid, req.ID).Delete(&models.Category{})
			database.DB.Where("book_id = ?", req.ID).Delete(&models.Budget{})
		}
	}

	database.DB.Delete(&book)
	Broadcast(c, "books", "delete", req.ID)
	OK(c, nil)
}

// ArchiveBook 归档 / 取消归档账本（C11：停用账本不必删除）
func ArchiveBook(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.IDRequest
	if err := c.ShouldBindUri(&req); err != nil {
		Bad(c, "参数错误")
		return
	}
	archived := c.DefaultQuery("archived", "1") == "1"
	var book models.Book
	if err := database.DB.Where("id = ? AND user_id = ?", req.ID, uid).First(&book).Error; err != nil {
		NotFound(c, "账本不存在")
		return
	}
	if archived && book.IsDefault {
		Fail(c, 2001, "不能归档默认账本")
		return
	}
	database.DB.Model(&models.Book{}).Where("id = ? AND user_id = ?", req.ID, uid).
		Update("is_archived", archived)
	Broadcast(c, "books", "update", req.ID)
	OK(c, gin.H{"id": req.ID, "is_archived": archived})
}

// ListArchivedBooks 已归档账本（设置页管理入口）
func ListArchivedBooks(c *gin.Context) {
	uid := middleware.GetUID(c)
	var books []models.Book
	database.DB.Where("user_id = ? AND is_archived = ?", uid, true).
		Order("updated_at DESC").Find(&books)
	OK(c, books)
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

	who := strings.TrimSpace(req.Who())
	if who == "" {
		Bad(c, "请提供用户名或邮箱")
		return
	}
	// C5：旧实现 DTO 只有 username 字段，前端提交 { email } → 后端收到空串 →
	// `WHERE username = '' OR email = ''` 匹配不到，邀请必然失败。
	// 统一为「用户名或邮箱」匹配，并显式排除空值。
	var targetUser models.User
	if err := database.DB.Where("username = ? OR email = ?", who, who).First(&targetUser).Error; err != nil {
		Fail(c, 2002, "用户不存在")
		return
	}
	if targetUser.ID == uid {
		Fail(c, 2003, "不能邀请自己")
		return
	}
	role := req.Role
	if role == "" {
		role = "viewer"
	}

	// 已是成员：返回明确的业务错误码（前端据此提示「该用户已在账本中」）
	var exist models.BookMember
	if err := database.DB.Where("book_id = ? AND user_id = ?", reqUri.ID, targetUser.ID).
		First(&exist).Error; err == nil {
		Fail(c, 2004, "该用户已是成员")
		return
	}

	mem := models.BookMember{
		BookID:   reqUri.ID,
		UserID:   targetUser.ID,
		Role:     role,
		JoinedAt: time.Now(),
	}
	if err := database.DB.Create(&mem).Error; err != nil {
		Fail(c, 2004, "邀请失败: "+err.Error())
		return
	}
	// 确保账本所有者也有一条 owner 成员记录，供成员列表/权限判定统一使用
	var ownerMem models.BookMember
	if err := database.DB.Where("book_id = ? AND user_id = ?", reqUri.ID, uid).First(&ownerMem).Error; err != nil {
		database.DB.Create(&models.BookMember{BookID: reqUri.ID, UserID: uid, Role: "owner", JoinedAt: time.Now()})
	}
	Created(c, mem)
}

// RemoveBookMember 移除成员 / 退出共享账本
func RemoveBookMember(c *gin.Context) {
	uid := middleware.GetUID(c)
	var reqUri dto.IDRequest
	c.ShouldBindUri(&reqUri)
	memberID := c.Param("memberId")

	var book models.Book
	if err := database.DB.Where("id = ? AND user_id = ?", reqUri.ID, uid).First(&book).Error; err != nil {
		Forbidden(c, "只有账本拥有者可移除成员")
		return
	}
	database.DB.Where("id = ? AND book_id = ? AND role != ?", memberID, reqUri.ID, "owner").
		Delete(&models.BookMember{})
	OK(c, nil)
}

func firstNotEmpty(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// findBookByNameOrCreate 按名称匹配当前用户的账本，不存在则创建一个（用于导入时按「账本」列归属交易）。
// 仅做最小字段写入，不触发系统分类初始化/事件广播，避免导入副作用。
func findBookByNameOrCreate(uid uint, name string) models.Book {
	var b models.Book
	if err := database.DB.Where("user_id = ? AND name = ?", uid, name).First(&b).Error; err == nil {
		return b
	}
	b = models.Book{UserID: uid, Name: name, Currency: "CNY"}
	database.DB.Create(&b)
	return b
}
