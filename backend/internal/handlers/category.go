package handlers

import (
	"huozhi/internal/database"
	"huozhi/internal/dto"
	"huozhi/internal/middleware"
	"huozhi/internal/models"

	"github.com/gin-gonic/gin"
)

// ========== 分类 Category ==========

// ListCategories 分类列表（按kind分组返回树状结构）
func ListCategories(c *gin.Context) {
	uid := middleware.GetUID(c)
	bookID := c.Query("book_id")
	kind := c.Query("kind") // expense/income/system/all
	includeArchived := c.Query("include_archived") == "1"

	q := database.DB.Where("user_id = ?", uid)
	if bookID != "" && bookID != "0" {
		q = q.Where("(book_id = ? OR book_id = 0)", bookID)
	}
	if kind != "" && kind != "all" {
		q = q.Where("kind = ?", kind)
	}
	if !includeArchived {
		q = q.Where("is_archived = ?", false)
	}

	var all []models.Category
	q.Order("sort ASC, id ASC").Find(&all)

	// 树状
	parentMap := make(map[uint][]models.Category)
	var roots []models.Category
	for _, cat := range all {
		if cat.ParentID == 0 {
			roots = append(roots, cat)
		} else {
			parentMap[cat.ParentID] = append(parentMap[cat.ParentID], cat)
		}
	}
	type treeCat struct {
		models.Category
		Children []models.Category `json:"children"`
	}
	result := map[string][]treeCat{
		"expense": {}, "income": {}, "system": {},
	}
	for _, r := range roots {
		tc := treeCat{Category: r, Children: parentMap[r.ID]}
		result[string(r.Kind)] = append(result[string(r.Kind)], tc)
	}
	OK(c, result)
}

// CreateCategory 创建分类
func CreateCategory(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Bad(c, "参数错误: "+err.Error())
		return
	}
	cat := models.Category{
		UserID:   uid,
		BookID:   req.BookID,
		ParentID: req.ParentID,
		Name:     req.Name,
		Kind:     models.CategoryKind(req.Kind),
		Icon:     req.Icon,
		Color:    req.Color,
		Sort:     req.Sort,
		NeedTag:  req.NeedTag,
	}
	if err := database.DB.Create(&cat).Error; err != nil {
		InternalErr(c, "创建失败")
		return
	}
	Created(c, cat)
}

// UpdateCategory 更新分类
func UpdateCategory(c *gin.Context) {
	uid := middleware.GetUID(c)
	var reqUri dto.IDRequest
	c.ShouldBindUri(&reqUri)
	var req dto.UpdateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Bad(c, err.Error())
		return
	}

	var old models.Category
	database.DB.Where("id = ? AND user_id = ?", reqUri.ID, uid).First(&old)
	if old.IsSystem {
		Fail(c, 3001, "系统分类不可修改")
		return
	}
	updates := map[string]interface{}{
		"name":      req.Name,
		"parent_id": req.ParentID,
		"icon":      req.Icon,
		"color":     req.Color,
		"sort":      req.Sort,
		"need_tag":  req.NeedTag,
	}
	database.DB.Model(&old).Updates(updates)
	var cat models.Category
	database.DB.First(&cat, reqUri.ID)
	OK(c, cat)
}

// DeleteCategory 删除分类
func DeleteCategory(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.IDRequest
	c.ShouldBindUri(&req)
	var cat models.Category
	database.DB.Where("id = ? AND user_id = ?", req.ID, uid).First(&cat)
	if cat.IsSystem {
		Fail(c, 3001, "系统分类不可删除")
		return
	}
	// 归档而非硬删除
	database.DB.Model(&cat).Update("is_archived", true)
	OK(c, nil)
}

// ========== 标签 Tag ==========

func ListTags(c *gin.Context) {
	uid := middleware.GetUID(c)
	var tags []models.Tag
	database.DB.Where("user_id = ?", uid).Order("sort ASC, count DESC, id DESC").Find(&tags)
	OK(c, tags)
}

func CreateTag(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.CreateTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Bad(c, err.Error())
		return
	}
	tag := models.Tag{
		UserID: uid,
		BookID: req.BookID,
		Name:   req.Name,
		Color:  req.Color,
		Sort:   req.Sort,
	}
	if err := database.DB.Create(&tag).Error; err != nil {
		Fail(c, 4001, "标签已存在")
		return
	}
	Created(c, tag)
}

func UpdateTag(c *gin.Context) {
	uid := middleware.GetUID(c)
	var reqUri dto.IDRequest
	c.ShouldBindUri(&reqUri)
	var t models.Tag
	if err := c.ShouldBindJSON(&t); err != nil {
		Bad(c, err.Error())
		return
	}
	database.DB.Model(&models.Tag{}).Where("id = ? AND user_id = ?", reqUri.ID, uid).
		Updates(map[string]interface{}{"name": t.Name, "color": t.Color, "sort": t.Sort})
	var nt models.Tag
	database.DB.First(&nt, reqUri.ID)
	OK(c, nt)
}

func DeleteTag(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.IDRequest
	c.ShouldBindUri(&req)
	database.DB.Where("id = ? AND user_id = ?", req.ID, uid).Delete(&models.Tag{})
	OK(c, nil)
}
