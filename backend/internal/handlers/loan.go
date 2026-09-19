package handlers

import (
	"time"

	"huozhi/internal/database"
	"huozhi/internal/dto"
	"huozhi/internal/middleware"
	"huozhi/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// 借贷联动的系统账户名（按用户唯一）。所有 lend 共用「应收」虚拟账户，所有 borrow 共用「应付」负债账户，
// 账户余额天然等于当前净应收 / 净应付，并自动并入资产概览（account.go 按账户类型汇总）。
const (
	loanReceivableName = "借出·应收" // 虚拟账户（资产）
	loanPayableName    = "借款·应付" // 负债账户
)

// ensureLoanAccount 取得 / 创建借贷联动的系统账户（按用户）。
//
// lend 返回虚拟（资产）账户，borrow 返回负债账户。首次借贷时自动创建并命名固定，
// 后续复用同一账户——避免每笔借贷都开一个账户、也避免删除单笔借贷误删共享账户。
func ensureLoanAccount(db *gorm.DB, uid, bookID uint, direction models.LoanDirection) (*models.Account, bool) {
	wantName := loanReceivableName
	wantType := models.AccVirtual
	if direction == models.LoanBorrow {
		wantName = loanPayableName
		wantType = models.AccLiability
	}
	var acc models.Account
	if err := db.Where("user_id = ? AND name = ?", uid, wantName).First(&acc).Error; err == nil {
		return &acc, true
	}
	acc = models.Account{
		UserID:        uid,
		BookID:        bookID,
		Name:          wantName,
		Type:          wantType,
		Currency:      "CNY",
		Balance:       0,
		InitialAmount: 0,
		Icon:          "🤝",
		Color:         "#64748b",
		IncludeInTotal: true,
		IncludeInBudget: false,
	}
	if err := db.Create(&acc).Error; err != nil {
		return nil, false
	}
	return &acc, true
}

// ListLoans 列出当前账本下的借贷，并附派生字段（剩余本金、逾期标记）。
func ListLoans(c *gin.Context) {
	uid := middleware.GetUID(c)
	var list []models.Loan
	applyBookScope(c, database.DB.Model(&models.Loan{}), uid).
		Order("status ASC, due_date ASC, created_at DESC").Find(&list)
	OK(c, list)
}

// CreateLoan 新建借贷，并联动真实账户生成 transfer 交易。
//
// lend：transfer(from=资金账户, to=应收账户) → 现金减少、应收增加（净资产不变）。
// borrow：transfer(from=应付账户, to=资金账户) → 现金增加、欠款增加（净资产不变）。
func CreateLoan(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.CreateLoanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Bad(c, "参数错误: "+err.Error())
		return
	}
	direction := models.LoanDirection(req.Direction)
	if direction != models.LoanLend && direction != models.LoanBorrow {
		Bad(c, "direction 必须是 lend 或 borrow")
		return
	}
	loanDate, err := time.ParseInLocation("2006-01-02", req.LoanDate, time.Local)
	if err != nil {
		Bad(c, "借款日期格式不正确，应为 YYYY-MM-DD")
		return
	}
	var dueDate time.Time
	if req.DueDate != "" {
		dueDate, err = time.ParseInLocation("2006-01-02", req.DueDate, time.Local)
		if err != nil {
			Bad(c, "到期日期格式不正确，应为 YYYY-MM-DD")
			return
		}
	}
	currency := req.Currency
	if currency == "" {
		currency = "CNY"
	}
	interestType := req.InterestType
	if interestType == "" {
		interestType = "none"
	}

	principal := models.FromYuan(req.Principal)

	db := database.DB.Begin()
	// 资金账户
	var fundAcc models.Account
	if err := db.Where("id = ? AND user_id = ?", req.AccountID, uid).First(&fundAcc).Error; err != nil {
		db.Rollback()
		Bad(c, "资金账户不存在")
		return
	}
	// 系统联动账户
	sysAcc, ok := ensureLoanAccount(db, uid, req.BookID, direction)
	if !ok {
		db.Rollback()
		InternalErr(c, "创建联动账户失败")
		return
	}

	loan := models.Loan{
		UserID:        uid,
		BookID:        req.BookID,
		Direction:     direction,
		Counterparty:  req.Counterparty,
		Principal:     principal,
		Currency:      currency,
		InterestRate:  req.InterestRate,
		InterestType:  interestType,
		AccountID:     req.AccountID,
		LoanDate:      loanDate,
		DueDate:       dueDate,
		Note:          req.Note,
		Status:        "active",
	}
	if err := db.Create(&loan).Error; err != nil {
		db.Rollback()
		InternalErr(c, "创建借贷失败: "+err.Error())
		return
	}

	// 生成 transfer 交易
	from, to := &fundAcc, sysAcc
	if direction == models.LoanBorrow {
		from, to = sysAcc, &fundAcc
	}
	catID := ensureSystemCategory(db, uid, req.BookID, "借贷", models.KindSystem, "🤝")
	tx := models.Transaction{
		UserID:           uid,
		BookID:           req.BookID,
		Type:             models.TxTransfer,
		Amount:           principal,
		Currency:         currency,
		CategoryID:       catID,
		AccountID:        from.ID,
		ToAccountID:      to.ID,
		TxDate:           loanDate,
		Description:      loanDesc(direction, req.Counterparty, "创建"),
		LoanID:           loan.ID,
		RelatedType:      models.RelatedLoan,
		IncludeInBalance: true,
		IncludeInBudget:  false,
	}
	if err := db.Create(&tx).Error; err != nil {
		db.Rollback()
		InternalErr(c, "创建交易失败: "+err.Error())
		return
	}
	updateAccountBalances(db, &tx, from, to, true)
	loan.TransactionID = tx.ID
	db.Model(&models.Loan{}).Where("id = ?", loan.ID).Update("transaction_id", tx.ID)

	if err := db.Commit().Error; err != nil {
		db.Rollback()
		InternalErr(c, "提交失败: "+err.Error())
		return
	}
	Broadcast(c, "loans", "create", loan.ID)
	Created(c, loan)
}

// RepayLoan 记录一笔还款，联动账户并更新借贷状态。
//
// lend 还款：transfer(from=应收账户, to=收款账户) 冲减应收；利息另记一笔收入。
// borrow 还款：transfer(from=还款账户, to=应付账户) 冲减欠款；利息另记一笔支出。
func RepayLoan(c *gin.Context) {
	uid := middleware.GetUID(c)
	var reqUri dto.IDRequest
	c.ShouldBindUri(&reqUri)
	var req dto.RepayLoanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Bad(c, "参数错误: "+err.Error())
		return
	}
	repaidAt, err := time.ParseInLocation("2006-01-02", req.RepaidAt, time.Local)
	if err != nil {
		Bad(c, "还款日期格式不正确，应为 YYYY-MM-DD")
		return
	}

	amount := models.FromYuan(req.Amount)
	interest := models.FromYuan(req.InterestAmount)

	db := database.DB.Begin()
	var loan models.Loan
	if err := db.Where("id = ? AND user_id = ?", reqUri.ID, uid).First(&loan).Error; err != nil {
		db.Rollback()
		NotFound(c, "借贷记录不存在")
		return
	}
	// 超额还款保护：本金还款不得超出剩余本金
	remaining := loan.Principal - loan.RepaidPrincipal
	if amount > remaining {
		db.Rollback()
		Bad(c, "还款本金超过剩余本金（剩余 "+remaining.String()+"）")
		return
	}

	var repayAcc models.Account
	if err := db.Where("id = ? AND user_id = ?", req.RepayAccountID, uid).First(&repayAcc).Error; err != nil {
		db.Rollback()
		Bad(c, "收款/还款账户不存在")
		return
	}
	sysAcc, ok := ensureLoanAccount(db, uid, loan.BookID, loan.Direction)
	if !ok {
		db.Rollback()
		InternalErr(c, "联动账户不可用")
		return
	}

	// 本金 transfer：lend 从应收→收款账户；borrow 从还款账户→应付账户
	from, to := sysAcc, &repayAcc
	if loan.Direction == models.LoanBorrow {
		from, to = &repayAcc, sysAcc
	}
	catID := ensureSystemCategory(db, uid, loan.BookID, "借贷", models.KindSystem, "🤝")
	tx := models.Transaction{
		UserID:           uid,
		BookID:           loan.BookID,
		Type:             models.TxTransfer,
		Amount:           amount,
		Currency:         loan.Currency,
		CategoryID:       catID,
		AccountID:        from.ID,
		ToAccountID:      to.ID,
		TxDate:           repaidAt,
		Description:      loanDesc(loan.Direction, loan.Counterparty, "还款"),
		LoanID:           loan.ID,
		RelatedType:      models.RelatedLoan,
		IncludeInBalance: true,
		IncludeInBudget:  false,
	}
	if err := db.Create(&tx).Error; err != nil {
		db.Rollback()
		InternalErr(c, "创建还款交易失败: "+err.Error())
		return
	}
	updateAccountBalances(db, &tx, from, to, true)

	repay := models.LoanRepayment{
		UserID:        uid,
		BookID:        loan.BookID,
		LoanID:        loan.ID,
		Amount:        amount,
		InterestAmount: interest,
		RepayAccountID: req.RepayAccountID,
		RepaidAt:      repaidAt,
		Note:          req.Note,
		TransactionID: tx.ID,
	}

	// 利息：lend 还款利息 → 收入；borrow 还款利息 → 支出（均进/出还款账户）
	var interestTxID uint
	if interest > 0 {
		interestTx := models.Transaction{
			UserID:           uid,
			BookID:           loan.BookID,
			Type:             models.TxIncome,
			Amount:           interest,
			Currency:         loan.Currency,
			CategoryID:       ensureSystemCategory(db, uid, loan.BookID, "借贷利息", models.KindSystem, "💸"),
			AccountID:        req.RepayAccountID,
			TxDate:           repaidAt,
			Description:      loanDesc(loan.Direction, loan.Counterparty, "利息"),
			LoanID:           loan.ID,
			RelatedType:      models.RelatedLoan,
			IncludeInBalance: true,
			IncludeInBudget:  false,
		}
		if loan.Direction == models.LoanBorrow {
			interestTx.Type = models.TxExpense
		}
		if err := db.Create(&interestTx).Error; err != nil {
			db.Rollback()
			InternalErr(c, "创建利息交易失败: "+err.Error())
			return
		}
		acc := repayAcc
		updateAccountBalances(db, &interestTx, &acc, nil, true)
		interestTxID = interestTx.ID
		repay.InterestTxID = interestTxID
	}

	if err := db.Create(&repay).Error; err != nil {
		db.Rollback()
		InternalErr(c, "保存还款记录失败: "+err.Error())
		return
	}

	// 更新借贷主记录累计与状态
	newRepaid := loan.RepaidPrincipal + amount
	newInterest := loan.RepaidInterest + interest
	status := "active"
	if newRepaid >= loan.Principal {
		status = "completed"
	}
	db.Model(&models.Loan{}).Where("id = ?", loan.ID).Updates(map[string]interface{}{
		"repaid_principal": newRepaid,
		"repaid_interest":  newInterest,
		"status":           status,
	})

	if err := db.Commit().Error; err != nil {
		db.Rollback()
		InternalErr(c, "提交失败: "+err.Error())
		return
	}
	Broadcast(c, "loans", "update", loan.ID)
	OK(c, gin.H{"id": loan.ID, "status": status, "repaid_principal": newRepaid})
}

// UpdateLoan 更新借贷状态 / 备注（例如手动标记结清）。
func UpdateLoan(c *gin.Context) {
	uid := middleware.GetUID(c)
	var reqUri dto.IDRequest
	c.ShouldBindUri(&reqUri)
	var req dto.UpdateLoanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Bad(c, "参数错误: "+err.Error())
		return
	}
	var loan models.Loan
	if err := database.DB.Where("id = ? AND user_id = ?", reqUri.ID, uid).First(&loan).Error; err != nil {
		NotFound(c, "借贷记录不存在")
		return
	}
	updates := map[string]interface{}{"status": req.Status}
	if req.Note != "" {
		updates["note"] = req.Note
	}
	database.DB.Model(&models.Loan{}).Where("id = ?", loan.ID).Updates(updates)
	Broadcast(c, "loans", "update", loan.ID)
	OK(c, nil)
}

// DeleteLoan 删除借贷及其全部关联交易（创建 transfer + 各次还款 transfer + 利息交易），并回滚账户余额。
func DeleteLoan(c *gin.Context) {
	uid := middleware.GetUID(c)
	var reqUri dto.IDRequest
	c.ShouldBindUri(&reqUri)

	db := database.DB.Begin()
	var loan models.Loan
	if err := db.Where("id = ? AND user_id = ?", reqUri.ID, uid).First(&loan).Error; err != nil {
		db.Rollback()
		NotFound(c, "借贷记录不存在")
		return
	}
	// 找出该借贷的全部关联交易，逐笔反向调整余额后删除（避免账户余额失真）
	var txs []models.Transaction
	db.Where("loan_id = ? AND user_id = ?", loan.ID, uid).Find(&txs)
	for i := range txs {
		t := txs[i]
		var from, to *models.Account
		if t.Type == models.TxTransfer {
			var fa, ta models.Account
			if db.Where("id = ?", t.AccountID).First(&fa).Error == nil {
				from = &fa
			}
			if t.ToAccountID > 0 && db.Where("id = ?", t.ToAccountID).First(&ta).Error == nil {
				to = &ta
			}
		} else {
			var fa models.Account
			if db.Where("id = ?", t.AccountID).First(&fa).Error == nil {
				from = &fa
			}
		}
		// 反向：isAdd=false 撤销原影响
		updateAccountBalances(db, &t, from, to, false)
	}
	db.Where("loan_id = ? AND user_id = ?", loan.ID, uid).Delete(&models.Transaction{})
	db.Where("loan_id = ? AND user_id = ?", loan.ID, uid).Delete(&models.LoanRepayment{})
	db.Where("id = ? AND user_id = ?", loan.ID, uid).Delete(&models.Loan{})
	if err := db.Commit().Error; err != nil {
		db.Rollback()
		InternalErr(c, "删除失败: "+err.Error())
		return
	}
	Broadcast(c, "loans", "delete", loan.ID)
	OK(c, nil)
}

// loanDesc 生成借贷交易的说明文案。
func loanDesc(direction models.LoanDirection, counterparty, action string) string {
	prefix := "借出"
	if direction == models.LoanBorrow {
		prefix = "借入"
	}
	return "[" + prefix + "] " + action + " · " + counterparty
}
