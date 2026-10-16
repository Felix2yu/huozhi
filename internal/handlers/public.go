package handlers

import (
	"huozhi/internal/database"
	"huozhi/internal/middleware"
	"huozhi/internal/models"
	"strconv"

	"github.com/gin-gonic/gin"
)

// GetPublicBill 获取公开账单信息（供外部系统查询）
// 需要有效的API key认证
func GetPublicBill(c *gin.Context) {
	uid := middleware.GetUID(c)
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		Bad(c, "无效的账单ID")
		return
	}

	var tx models.Transaction
	if err := database.DB.Where("id = ? AND user_id = ?", uint(id), uid).First(&tx).Error; err != nil {
		NotFound(c, "账单不存在")
		return
	}

	// 返回精简信息，只包含外部系统需要的字段
	type PublicBillResponse struct {
		ID          uint          `json:"id"`
		Description string        `json:"description"`
		Amount      models.Money  `json:"amount"`
		Currency    string        `json:"currency"`
		Type        string        `json:"type"`
		TxDate      string        `json:"tx_date"`
		Merchant    string        `json:"merchant"`
		Remark      string        `json:"remark"`
	}

	OK(c, PublicBillResponse{
		ID:          tx.ID,
		Description: tx.Description,
		Amount:      tx.Amount,
		Currency:    tx.Currency,
		Type:        string(tx.Type),
		TxDate:      tx.TxDate.Format("2006-01-02 15:04:05"),
		Merchant:    tx.Merchant,
		Remark:      tx.Remark,
	})
}

// ListPublicBills 获取公开账单列表（供外部系统查询）
// 支持分页和时间范围筛选
func ListPublicBills(c *gin.Context) {
	uid := middleware.GetUID(c)

	// 解析查询参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	q := database.DB.Where("user_id = ?", uid)

	// 时间范围筛选
	if startDate != "" {
		q = q.Where("tx_date >= ?", startDate)
	}
	if endDate != "" {
		q = q.Where("tx_date <= ?", endDate+" 23:59:59")
	}

	var total int64
	q.Model(&models.Transaction{}).Count(&total)

	var list []models.Transaction
	offset := (page - 1) * pageSize
	q.Order("tx_date DESC, id DESC").Offset(offset).Limit(pageSize).Find(&list)

	// 返回精简信息
	type PublicBillResponse struct {
		ID          uint          `json:"id"`
		Description string        `json:"description"`
		Amount      models.Money  `json:"amount"`
		Currency    string        `json:"currency"`
		Type        string        `json:"type"`
		TxDate      string        `json:"tx_date"`
		Merchant    string        `json:"merchant"`
	}

	var responseList []PublicBillResponse
	for _, tx := range list {
		responseList = append(responseList, PublicBillResponse{
			ID:          tx.ID,
			Description: tx.Description,
			Amount:      tx.Amount,
			Currency:    tx.Currency,
			Type:        string(tx.Type),
			TxDate:      tx.TxDate.Format("2006-01-02 15:04:05"),
			Merchant:    tx.Merchant,
		})
	}

	PagedOK(c, responseList, page, pageSize, total)
}