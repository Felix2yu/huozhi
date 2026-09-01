package handlers

import (
	"huozhi/internal/database"
	"huozhi/internal/dto"
	"huozhi/internal/middleware"
	"huozhi/internal/models"
	"time"

	"github.com/gin-gonic/gin"
)

// ========== 账户 Account ==========

// ListAccounts 账户列表
func ListAccounts(c *gin.Context) {
	uid := middleware.GetUID(c)
	bookID := c.Query("book_id")
	typeFilter := c.Query("type")
	includeArchived := c.Query("include_archived") == "1"

	q := database.DB.Where("user_id = ?", uid)
	if bookID != "" && bookID != "0" {
		q = q.Where("(book_id = ? OR book_id = 0)", bookID)
	}
	if typeFilter != "" {
		q = q.Where("type = ?", typeFilter)
	}
	if !includeArchived {
		q = q.Where("is_archived = ?", false)
	}
	if hideHidden := c.Query("show_hidden") != "1"; hideHidden {
		q = q.Where("is_hidden = ?", false)
	}

	var accounts []models.Account
	q.Order("is_archived ASC, sort ASC, id DESC").Find(&accounts)

	// 汇总
	type summary struct {
		TotalAsset  float64 `json:"total_asset"`
		TotalDebt   float64 `json:"total_debt"`
		NetAsset    float64 `json:"net_asset"`
		CashFlow    float64 `json:"cash_flow"`
	}
	s := summary{}
	for _, a := range accounts {
		if !a.IncludeInTotal || a.IsArchived {
			continue
		}
		switch a.Type {
		case models.AccLiability, models.AccCredit:
			s.TotalDebt += a.Balance
		default:
			s.TotalAsset += a.Balance
		}
	}
	s.NetAsset = s.TotalAsset - s.TotalDebt

	OK(c, gin.H{"accounts": accounts, "summary": s})
}

// GetAccount 单个账户
func GetAccount(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.IDRequest
	c.ShouldBindUri(&req)
	var acc models.Account
	if err := database.DB.Where("id = ? AND user_id = ?", req.ID, uid).First(&acc).Error; err != nil {
		NotFound(c, "账户不存在")
		return
	}
	OK(c, acc)
}

// CreateAccount 创建账户
func CreateAccount(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.CreateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Bad(c, "参数错误: "+err.Error())
		return
	}

	acc := models.Account{
		UserID:         uid,
		BookID:         req.BookID,
		Name:           req.Name,
		Type:           models.AccountType(req.Type),
		Currency:       firstNotEmpty(req.Currency, "CNY"),
		Balance:        req.InitialAmount,
		InitialAmount:  req.InitialAmount,
		Icon:           req.Icon,
		Color:          req.Color,
		BankName:       req.BankName,
		CardNo4:        req.CardNo4,
		CreditLimit:    req.CreditLimit,
		BillDay:        req.BillDay,
		RepayDay:       req.RepayDay,
		APR:            req.APR,
		IncludeInTotal: req.IncludeInTotal,
		IncludeInBudget: req.IncludeInBudget,
		IsHidden:       req.IsHidden,
		GroupID:        req.GroupID,
		Sort:           req.Sort,
		Remark:         req.Remark,
	}

	if err := database.DB.Create(&acc).Error; err != nil {
		InternalErr(c, "创建失败: "+err.Error())
		return
	}

	// 记录初始金额对应的调整交易（可选，让历史有迹可循）
	if req.InitialAmount != 0 {
		var adjCat models.Category
		database.DB.Where("user_id = ? AND kind = ? AND name = ?", uid, models.KindSystem, "余额调整").First(&adjCat)
		if adjCat.ID > 0 {
			adjTx := models.Transaction{
				UserID:            uid,
				BookID:            req.BookID,
				Type:              models.TxAdjust,
				Amount:            req.InitialAmount,
				Currency:          acc.Currency,
				CategoryID:        adjCat.ID,
				AccountID:         acc.ID,
				TxDate:            time.Now(),
				Description:       "初始金额",
				IncludeInBalance:  true,
				IncludeInBudget:   false,
			}
			database.DB.Create(&adjTx)
		}
	}

	Created(c, acc)
}

// UpdateAccount 更新账户
func UpdateAccount(c *gin.Context) {
	uid := middleware.GetUID(c)
	var reqUri dto.IDRequest
	c.ShouldBindUri(&reqUri)
	var req dto.UpdateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Bad(c, "参数错误: "+err.Error())
		return
	}
	updates := map[string]interface{}{
		"name":             req.Name,
		"type":             req.Type,
		"currency":         req.Currency,
		"icon":             req.Icon,
		"color":            req.Color,
		"bank_name":        req.BankName,
		"card_no4":         req.CardNo4,
		"credit_limit":     req.CreditLimit,
		"bill_day":         req.BillDay,
		"repay_day":        req.RepayDay,
		"apr":              req.APR,
		"include_in_total": req.IncludeInTotal,
		"include_in_budget": req.IncludeInBudget,
		"is_hidden":        req.IsHidden,
		"group_id":         req.GroupID,
		"sort":             req.Sort,
		"remark":           req.Remark,
	}
	database.DB.Model(&models.Account{}).Where("id = ? AND user_id = ?", reqUri.ID, uid).Updates(updates)
	var acc models.Account
	database.DB.First(&acc, reqUri.ID)
	OK(c, acc)
}

// DeleteAccount 删除/归档账户
func DeleteAccount(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.IDRequest
	c.ShouldBindUri(&req)
	// 安全起见，不硬删除，改为归档
	if err := database.DB.Model(&models.Account{}).Where("id = ? AND user_id = ?", req.ID, uid).
		Update("is_archived", true).Error; err != nil {
		InternalErr(c, "归档失败")
		return
	}
	OK(c, nil)
}

// AdjustAccountBalance 调整账户余额
func AdjustAccountBalance(c *gin.Context) {
	uid := middleware.GetUID(c)
	var reqUri dto.IDRequest
	c.ShouldBindUri(&reqUri)
	var req dto.AdjustAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Bad(c, "参数错误: "+err.Error())
		return
	}

	var acc models.Account
	if err := database.DB.Where("id = ? AND user_id = ?", reqUri.ID, uid).First(&acc).Error; err != nil {
		NotFound(c, "账户不存在")
		return
	}

	// 计算差额
	diff := req.Amount - acc.Balance

	// 更新余额
	database.DB.Model(&acc).Update("balance", req.Amount)

	// 插入调整记录
	var adjCat models.Category
	database.DB.Where("user_id = ? AND kind = ? AND name = ?", uid, models.KindSystem, "余额调整").First(&adjCat)
	tx := models.Transaction{
		UserID:           uid,
		BookID:           acc.BookID,
		Type:             models.TxAdjust,
		Amount:           diff,
		Currency:         acc.Currency,
		CategoryID:       adjCat.ID,
		AccountID:        acc.ID,
		TxDate:           req.Date,
		Description:      firstNotEmpty(req.Description, "余额调整"),
		IncludeInBalance: true,
		IncludeInBudget:  false,
	}
	database.DB.Create(&tx)

	OK(c, gin.H{"new_balance": req.Amount, "diff": diff, "transaction": tx})
}

// ========== 资产分组 ==========

func ListAccountGroups(c *gin.Context) {
	uid := middleware.GetUID(c)
	var groups []models.AccountGroup
	database.DB.Where("user_id = ?", uid).Order("sort ASC, id DESC").Find(&groups)
	OK(c, groups)
}

func CreateAccountGroup(c *gin.Context) {
	uid := middleware.GetUID(c)
	var g models.AccountGroup
	if err := c.ShouldBindJSON(&g); err != nil {
		Bad(c, err.Error())
		return
	}
	g.UserID = uid
	database.DB.Create(&g)
	Created(c, g)
}

func DeleteAccountGroup(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.IDRequest
	c.ShouldBindUri(&req)
	database.DB.Where("id = ? AND user_id = ?", req.ID, uid).Delete(&models.AccountGroup{})
	OK(c, nil)
}
