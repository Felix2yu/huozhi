package handlers

import (
	"huozhi/internal/config"
	"huozhi/internal/database"
	"huozhi/internal/dto"
	"huozhi/internal/middleware"
	"huozhi/internal/models"
	"huozhi/pkg/auth"
	"huozhi/pkg/crypto"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func encryptCardNo(plain string) string {
	plain = strings.ReplaceAll(plain, " ", "")
	if plain == "" {
		return ""
	}
	key := crypto.KeyFromSecret(config.AppConfig.JWT.Secret)
	ct, _ := crypto.Encrypt([]byte(plain), key)
	return ct
}

func decryptCardNo(cipher string) string {
	if cipher == "" {
		return ""
	}
	key := crypto.KeyFromSecret(config.AppConfig.JWT.Secret)
	pt, _ := crypto.Decrypt(cipher, key)
	return pt
}

func tail4(s string) string {
	s = strings.ReplaceAll(s, " ", "")
	if len(s) <= 4 {
		return s
	}
	return s[len(s)-4:]
}

// ========== 账户 Account ==========

// ListAccounts 账户列表
func ListAccounts(c *gin.Context) {
	uid := middleware.GetUID(c)
	bookID := c.Query("book_id")
	typeFilter := c.Query("type")
	includeArchived := c.Query("include_archived") == "1"

	// C5：共享账本内的账户由成员共享可见
	q := applyBookScope(c, database.DB.Model(&models.Account{}), uid)
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
		TotalAsset  models.Money `json:"total_asset"`
		TotalDebt   models.Money `json:"total_debt"`
		NetAsset    models.Money `json:"net_asset"`
		CashFlow    models.Money `json:"cash_flow"`
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

	// 处理完整卡号：加密存储 + 自动更新尾号
	var encryptedCardNo string
	cardNo4 := req.CardNo4
	if req.FullCardNo != "" {
		encryptedCardNo = encryptCardNo(req.FullCardNo)
		if cardNo4 == "" {
			cardNo4 = tail4(req.FullCardNo)
		}
	}
	acc := models.Account{
		UserID:          uid,
		BookID:          req.BookID,
		Name:            req.Name,
		Type:            models.AccountType(req.Type),
		Currency:        firstNotEmpty(req.Currency, "CNY"),
		Balance:         models.FromYuan(req.InitialAmount),
		InitialAmount:   models.FromYuan(req.InitialAmount),
		Icon:            req.Icon,
		Color:           req.Color,
		BankName:        req.BankName,
		CardNo4:         cardNo4,
		EncryptedCardNo: encryptedCardNo,
		CreditLimit:     models.FromYuan(req.CreditLimit),
		BillDay:         req.BillDay,
		RepayDay:        req.RepayDay,
		ExpireMonth:     req.ExpireMonth,
		ExpireYear:      req.ExpireYear,
		APR:             req.APR,
		IncludeInTotal:  req.IncludeInTotal,
		IncludeInBudget: req.IncludeInBudget,
		IsHidden:        req.IsHidden,
		GroupID:         req.GroupID,
		Sort:            req.Sort,
		Remark:          req.Remark,
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
				Amount:            models.FromYuan(req.InitialAmount),
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

	Broadcast(c, "accounts", "create", acc.ID)
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
		"name":              req.Name,
		"type":              req.Type,
		"currency":          req.Currency,
		"icon":              req.Icon,
		"color":             req.Color,
		"bank_name":         req.BankName,
		"card_no4":          req.CardNo4,
		"credit_limit":      req.CreditLimit,
		"bill_day":          req.BillDay,
		"repay_day":         req.RepayDay,
		"expire_month":      req.ExpireMonth,
		"expire_year":       req.ExpireYear,
		"apr":               req.APR,
		"include_in_total":  req.IncludeInTotal,
		"include_in_budget": req.IncludeInBudget,
		"is_hidden":         req.IsHidden,
		"is_archived":       req.IsArchived,
		"group_id":          req.GroupID,
		"sort":              req.Sort,
		"remark":            req.Remark,
	}
	// 完整卡号：非空才加密覆盖，空则保持原值
	if req.FullCardNo != "" {
		updates["encrypted_card_no"] = encryptCardNo(req.FullCardNo)
		if req.CardNo4 == "" {
			updates["card_no4"] = tail4(req.FullCardNo)
		}
	}
	database.DB.Model(&models.Account{}).Where("id = ? AND user_id = ?", reqUri.ID, uid).Updates(updates)
	var acc models.Account
	database.DB.First(&acc, reqUri.ID)
	Broadcast(c, "accounts", "update", acc.ID)
	OK(c, acc)
}

// GetFullCardNo 按需解密返回完整卡信息。
//
// C15：该接口原先是「登录即可取全卡号」，一旦账号被盗即全量泄露。
// 现要求二次验证登录密码，并把动作降到 POST（避免密码出现在 URL/访问日志里）。
// CVV 字段已整体移除（PCI-DSS 禁止存储授权后的 CVV2/CVC2）。
func GetFullCardNo(c *gin.Context) {
	uid := middleware.GetUID(c)
	var reqUri dto.IDRequest
	if err := c.ShouldBindUri(&reqUri); err != nil {
		Bad(c, "参数错误")
		return
	}

	// 二次验证：必须重新输入登录密码
	var req dto.VerifyPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Bad(c, "请重新输入登录密码以查看完整卡号")
		return
	}
	var user models.User
	if err := database.DB.Where("id = ?", uid).First(&user).Error; err != nil {
		NotFound(c, "用户不存在")
		return
	}
	if !auth.CheckPassword(req.Password, user.PasswordHash) {
		Forbidden(c, "密码错误")
		return
	}

	var acc models.Account
	if err := database.DB.Where("id = ? AND user_id = ?", reqUri.ID, uid).First(&acc).Error; err != nil {
		NotFound(c, "账户不存在")
		return
	}
	if acc.EncryptedCardNo == "" {
		NotFound(c, "未保存完整卡信息")
		return
	}
	log.Printf("[Audit] 用户 %d 查看了账户 %d 的完整卡号", uid, acc.ID)
	OK(c, gin.H{
		"full_card_no": decryptCardNo(acc.EncryptedCardNo),
		"expire_month": acc.ExpireMonth,
		"expire_year":  acc.ExpireYear,
	})
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
	Broadcast(c, "accounts", "delete", req.ID)
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

	// C13：UPDATE balance 与 INSERT 调整记录原本不在同一事务，中途失败会让余额与流水对不上；
	// 且 diff 可为负，会写入 amount < 0 的 adjust 交易（列表/统计展示异常）。
	db := database.DB.Begin()
	var acc models.Account
	if err := db.Where("id = ? AND user_id = ?", reqUri.ID, uid).First(&acc).Error; err != nil {
		db.Rollback()
		NotFound(c, "账户不存在")
		return
	}

	// 计算差额
	newBalance := models.FromYuan(req.Amount)
	diff := newBalance - acc.Balance
	if diff == 0 {
		db.Rollback()
		OK(c, gin.H{"new_balance": acc.Balance, "diff": models.Money(0), "changed": false})
		return
	}

	// 更新余额
	if err := db.Model(&acc).Update("balance", newBalance).Error; err != nil {
		db.Rollback()
		InternalErr(c, "更新余额失败: "+err.Error())
		return
	}

	// 插入调整记录：金额恒为正，方向体现在描述与方向标记上，
	// 保证列表/统计不会出现负数金额，也让对账（B7）能算出正确方向。
	var adjCat models.Category
	db.Where("user_id = ? AND kind = ? AND name = ?", uid, models.KindSystem, "余额调整").First(&adjCat)
	if adjCat.ID == 0 {
		adjCat.ID = ensureSystemCategory(db, uid, acc.BookID, "余额调整", models.KindSystem, "⚙️")
	}
	absDiff := diff
	direction := "调增"
	if absDiff < 0 {
		absDiff = -absDiff
		direction = "调减"
	}
	desc := req.Description
	if desc == "" {
		desc = "余额调整（" + direction + "）"
	}
	tx := models.Transaction{
		UserID:           uid,
		BookID:           acc.BookID,
		Type:             models.TxAdjust,
		Amount:           absDiff,
		Currency:         acc.Currency,
		CategoryID:       adjCat.ID,
		AccountID:        acc.ID,
		TxDate:           req.Date.T(),
		Description:      desc,
		Remark:           direction,
		RelatedType:      "balance_adjust",
		IncludeInBalance: false, // 余额已直接置值，禁止再被余额引擎二次加减
		IncludeInBudget:  false,
	}
	if err := db.Create(&tx).Error; err != nil {
		db.Rollback()
		InternalErr(c, "记录调整流水失败: "+err.Error())
		return
	}
	if err := db.Commit().Error; err != nil {
		InternalErr(c, "提交失败: "+err.Error())
		return
	}

	Broadcast(c, "accounts", "update", acc.ID)
	OK(c, gin.H{"new_balance": newBalance, "diff": diff, "direction": direction, "transaction": tx})
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
	Broadcast(c, "account_groups", "create", g.ID)
	Created(c, g)
}

func DeleteAccountGroup(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.IDRequest
	c.ShouldBindUri(&req)
	database.DB.Where("id = ? AND user_id = ?", req.ID, uid).Delete(&models.AccountGroup{})
	Broadcast(c, "account_groups", "delete", req.ID)
	OK(c, nil)
}

// GetCreditSummary 信用卡还款倒计时汇总（Dashboard 提醒用）
func GetCreditSummary(c *gin.Context) {
	uid := middleware.GetUID(c)
	now := time.Now()
	loc := now.Location()

	var cards []models.Account
	database.DB.Where("user_id = ? AND type = ? AND is_archived = ? AND repay_day > 0",
		uid, models.AccCredit, false).Find(&cards)

	type repayOut struct {
		ID          uint         `json:"id"`
		Name        string       `json:"name"`
		BankName    string       `json:"bank_name"`
		CardNo4     string       `json:"card_no4"`
		RepayDay    int          `json:"repay_day"`
		BillDay     int          `json:"bill_day"`
		Balance     models.Money `json:"balance"`
		CreditLimit models.Money `json:"credit_limit"`
		DaysLeft    int          `json:"days_left"`     // 距下一个还款日还剩几天
		RepayDate   string       `json:"repay_date"`    // 下次还款日
		BillAmount  models.Money `json:"bill_amount"`   // 已出账金额（本月支出）
		Overdue     bool         `json:"overdue"`        // 是否已逾期
	}
	var out []repayOut

	for _, card := range cards {
		// 计算下一个还款日
		// C17：原实现「过了 00:00 就算逾期」，还款日当天一早就显示「还剩 -0 天 / 逾期」，
		// 前端直接渲染负数 → 出现「还剩 -3 天」。改为：
		//   - 当天为还款日：不算逾期，还剩 0 天（今天要还）
		//   - 已过还款日：days_left 为正的已逾期天数（文案为「已逾期 N 天」）
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
		curMonthRepay := time.Date(now.Year(), now.Month(), card.RepayDay, 0, 0, 0, 0, loc)
		var nextRepay time.Time
		var daysLeft int
		var overdue bool

		switch {
		case curMonthRepay.Equal(today):
			nextRepay = curMonthRepay
			daysLeft = 0
			overdue = false
		case curMonthRepay.Before(today):
			nextRepay = curMonthRepay.AddDate(0, 1, 0)
			daysLeft = int(today.Sub(curMonthRepay).Hours() / 24) // 正数：已逾期天数
			overdue = true
		default:
			nextRepay = curMonthRepay
			daysLeft = int(curMonthRepay.Sub(today).Hours() / 24)
			overdue = false
		}

		// 计算已出账金额（本月支出到该信用卡的交易）
		billDay := card.BillDay
		if billDay == 0 {
			billDay = card.RepayDay - 20 // 默认：还款日前 20 天为账单日
		}
		var billStart time.Time
		if now.Day() >= billDay {
			billStart = time.Date(now.Year(), now.Month(), billDay, 0, 0, 0, 0, loc)
		} else {
			billStart = time.Date(now.Year(), now.Month()-1, billDay, 0, 0, 0, 0, loc)
		}
		billEnd := nextRepay
		var billAmount float64
		database.DB.Model(&models.Transaction{}).
			Where("user_id = ? AND account_id = ? AND tx_date >= ? AND tx_date < ? AND type = ?",
				uid, card.ID, billStart, billEnd, string(models.TxExpense)).
			Select("COALESCE(SUM(amount), 0)").Row().Scan(&billAmount)

		out = append(out, repayOut{
			ID: card.ID, Name: card.Name, BankName: card.BankName, CardNo4: card.CardNo4,
			RepayDay: card.RepayDay, BillDay: card.BillDay,
			Balance: card.Balance, CreditLimit: card.CreditLimit,
			DaysLeft: daysLeft, RepayDate: nextRepay.Format("2006-01-02"),
			BillAmount: models.FromCents(billAmount), Overdue: overdue,
		})
	}

	// 按距还款日天数排序（最紧急的排前面）
	sort.Slice(out, func(i, j int) bool {
		if out[i].Overdue != out[j].Overdue {
			return out[i].Overdue // 逾期的排最前面
		}
		return out[i].DaysLeft < out[j].DaysLeft
	})

	OK(c, out)
}
