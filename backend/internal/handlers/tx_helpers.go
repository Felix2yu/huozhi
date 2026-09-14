package handlers

import (
	"huozhi/internal/database"
	"huozhi/internal/models"
	"time"

	"gorm.io/gorm"
)

// ensureSystemCategory 取（或按需创建）系统分类，返回其 ID。
// 转账手续费、余额调整等后端自动生成的交易需要归属分类，
// 用户手工删掉后也要能自动补回，避免生成 category_id=0 的脏数据。
func ensureSystemCategory(db *gorm.DB, uid, bookID uint, name string, kind models.CategoryKind, icon string) uint {
	var cat models.Category
	q := db.Where("user_id = ? AND name = ?", uid, name)
	if bookID > 0 {
		q = q.Where("book_id IN ?", []uint{0, bookID})
	}
	if err := q.Order("book_id DESC").First(&cat).Error; err == nil {
		return cat.ID
	}
	cat = models.Category{
		UserID:   uid,
		BookID:   bookID,
		Name:     name,
		Kind:     kind,
		Icon:     icon,
		IsSystem: true,
	}
	if err := db.Create(&cat).Error; err != nil {
		return 0
	}
	return cat.ID
}

// adjustTagCounts 同步标签使用次数：delta=+1 关联时增加，delta=-1 解绑/删除时回退。
// 用 GREATEST(count + delta, 0) 兜底，避免历史脏数据把计数减成负数。
// 此前只有创建时 +1，删除/编辑从不回退，标签中心「使用次数」只增不减（C10）。
func adjustTagCounts(db *gorm.DB, tagIDs []uint, delta int) {
	if len(tagIDs) == 0 {
		return
	}
	if delta > 0 {
		db.Model(&models.Tag{}).Where("id IN ?", tagIDs).
			UpdateColumn("count", gorm.Expr("count + ?", delta))
		return
	}
	// PostgreSQL 与 SQLite 都支持 GREATEST
	db.Model(&models.Tag{}).Where("id IN ?", tagIDs).
		UpdateColumn("count", gorm.Expr("GREATEST(count + ?, 0)", delta))
}

// txTagIDs 读取某笔交易当前关联的标签
func txTagIDs(db *gorm.DB, txID uint) []uint {
	var ids []uint
	db.Model(&models.TransactionTag{}).Where("transaction_id = ?", txID).
		Pluck("tag_id", &ids)
	return ids
}

// revertDerivedTransactions 撤销某笔主交易派生出的子交易（手续费/分期/报销收款/存钱）。
// 删除或编辑主交易时必须同步回滚，否则账户余额会与流水对不上。
func revertDerivedTransactions(db *gorm.DB, txID uint) {
	var derived []models.Transaction
	db.Where("related_tx_id = ?", txID).Find(&derived)
	for i := range derived {
		d := derived[i]
		var from, to models.Account
		db.First(&from, d.AccountID)
		if d.ToAccountID > 0 {
			db.First(&to, d.ToAccountID)
		}
		updateAccountBalances(db, &d, &from, &to, false)
		applyBudgetUsed(db, d.UserID, d.BookID, d.CategoryID, d.TxDate, d.Amount, d.Type, d.IncludeInBudget, -1)
		adjustTagCounts(db, txTagIDs(db, d.ID), -1)
		db.Where("transaction_id = ?", d.ID).Delete(&models.TransactionTag{})
		db.Delete(&d)
	}
}

// UpdateAccountBalances 导出给 cmd/huozhi-server 的调度器复用。
// 此前周期记账在 main.go 里维护了一份残缺副本 updateRecurBalances：
// 没有负债账户方向处理、也不更新预算，与手工记账口径不一致（C7）。
func UpdateAccountBalances(db *gorm.DB, tx *models.Transaction, from, to *models.Account, isAdd bool) {
	updateAccountBalances(db, tx, from, to, isAdd)
}

// ApplyBudgetUsed 导出给调度器复用（同上）
func ApplyBudgetUsed(db *gorm.DB, uid, bookID, catID uint, date time.Time,
	amount models.Money, txType models.TransactionType, includeInBudget bool, factor int64) {
	applyBudgetUsed(db, uid, bookID, catID, date, amount, txType, includeInBudget, factor)
}

// hasTransactionsForBook 账本下是否已有交易（删除账本前的孤儿数据校验用）
func countBookChildren(db *gorm.DB, uid, bookID uint) (txs, accounts, categories int64) {
	db.Model(&models.Transaction{}).Where("user_id = ? AND book_id = ?", uid, bookID).Count(&txs)
	db.Model(&models.Account{}).Where("user_id = ? AND book_id = ?", uid, bookID).Count(&accounts)
	db.Model(&models.Category{}).Where("user_id = ? AND book_id = ?", uid, bookID).Count(&categories)
	return
}

// accessibleBookIDs 当前用户可见的账本 ID：自己创建的 + 被邀请加入的（共享账本，C5）
func accessibleBookIDs(uid uint) []uint {
	var ids []uint
	database.DB.Model(&models.BookMember{}).Where("user_id = ?", uid).Pluck("book_id", &ids)
	return ids
}

// applyBookScope 把查询范围从「仅本人」扩展为「本人 或 本人可见的共享账本」。
// 这是共享账本真正生效的关键：此前所有列表接口都只按 user_id 过滤，
// 被邀请成员看到的永远是空列表。
// 注意：必须用括号包裹，否则与后续 Where 用 AND 连接时，
// AND 优先级高于 OR 会让范围判断失效（SQL 优先级陷阱）。
func applyBookScope(q *gorm.DB, uid uint) *gorm.DB {
	ids := accessibleBookIDs(uid)
	if len(ids) == 0 {
		return q.Where("user_id = ?", uid)
	}
	return q.Where("(user_id = ? OR book_id IN ?)", uid, ids)
}

// canWriteBook 判断用户是否有某账本的写权限（owner 或 editor 成员）
func canWriteBook(uid, bookID uint) bool {
	var book models.Book
	if err := database.DB.Where("id = ? AND user_id = ?", bookID, uid).First(&book).Error; err == nil {
		return true
	}
	var mem models.BookMember
	if err := database.DB.Where("book_id = ? AND user_id = ? AND role IN ?",
		bookID, uid, []string{"owner", "editor"}).First(&mem).Error; err == nil {
		return true
	}
	return false
}

// ownsOrSharesAccount 校验账户归属：本人所有，或属于本人可见的共享账本
func ownsOrSharesAccount(uid, accountID uint) bool {
	if accountID == 0 {
		return false
	}
	var acc models.Account
	if err := database.DB.Where("id = ? AND user_id = ?", accountID, uid).First(&acc).Error; err == nil {
		return true
	}
	// 共享账本内的账户可能由他人创建
	ids := accessibleBookIDs(uid)
	if len(ids) == 0 {
		return false
	}
	return database.DB.Where("id = ? AND book_id IN ?", accountID, ids).First(&acc).Error == nil
}
