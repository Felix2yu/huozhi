package handlers

import (
	"huozhi/internal/database"
	"huozhi/internal/dto"
	"huozhi/internal/middleware"
	"huozhi/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
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

// ReorderCategories 批量更新分类排序（拖拽排序用）。
// 单独的排序接口不受 is_system 限制：系统预置分类也应支持调整显示顺序。
func ReorderCategories(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req struct {
		Items []struct {
			ID   uint `json:"id"`
			Sort int  `json:"sort"`
		} `json:"items" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Bad(c, "参数错误: "+err.Error())
		return
	}
	for _, it := range req.Items {
		database.DB.Model(&models.Category{}).
			Where("id = ? AND user_id = ?", it.ID, uid).
			Update("sort", it.Sort)
	}
	Broadcast(c, "categories", "update", 0)
	OK(c, nil)
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
	Broadcast(c, "categories", "create", cat.ID)
	Created(c, cat)
}

// UpdateCategory 更新分类
func UpdateCategory(c *gin.Context) {
	uid := middleware.GetUID(c)
	var reqUri dto.IDRequest
	c.ShouldBindUri(&reqUri)
	// 先解析原始 JSON，用于区分「未传入字段」与「显式传了零值」，
	// 作为 update mask，避免部分更新时把未传字段清零。
	raw := map[string]interface{}{}
	if err := c.ShouldBindBodyWith(&raw, binding.JSON); err != nil {
		Bad(c, "参数错误")
		return
	}
	var req dto.UpdateCategoryRequest
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		Bad(c, err.Error())
		return
	}

	var old models.Category
	database.DB.Where("id = ? AND user_id = ?", reqUri.ID, uid).First(&old)
	if old.IsSystem {
		Fail(c, 3001, "系统分类不可修改")
		return
	}
	if req.ParentID != 0 && req.ParentID == old.ID {
		Bad(c, "参数错误: 父分类不能是自身")
		return
	}
	// 依据原始 JSON 中出现的键，仅更新显式传入的字段
	updates := map[string]interface{}{}
	if _, ok := raw["name"]; ok {
		updates["name"] = req.Name
	}
	if _, ok := raw["parent_id"]; ok {
		updates["parent_id"] = req.ParentID
	}
	if _, ok := raw["icon"]; ok {
		updates["icon"] = req.Icon
	}
	if _, ok := raw["color"]; ok {
		updates["color"] = req.Color
	}
	if _, ok := raw["sort"]; ok {
		updates["sort"] = req.Sort
	}
	if _, ok := raw["kind"]; ok {
		updates["kind"] = models.CategoryKind(req.Kind)
	}
	if _, ok := raw["need_tag"]; ok && req.NeedTag != nil {
		updates["need_tag"] = *req.NeedTag
	}
	if len(updates) == 0 {
		Bad(c, "参数错误：未提供任何更新字段")
		return
	}
	database.DB.Model(&old).Updates(updates)
	var cat models.Category
	database.DB.First(&cat, reqUri.ID)
	Broadcast(c, "categories", "update", cat.ID)
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
	Broadcast(c, "categories", "delete", req.ID)
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
	Broadcast(c, "tags", "create", tag.ID)
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
	Broadcast(c, "tags", "update", nt.ID)
	OK(c, nt)
}

func DeleteTag(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.IDRequest
	c.ShouldBindUri(&req)
	database.DB.Where("id = ? AND user_id = ?", req.ID, uid).Delete(&models.Tag{})
	Broadcast(c, "tags", "delete", req.ID)
	OK(c, nil)
}
