package handlers

import (
	"huozhi/internal/database"
	"huozhi/internal/models"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ==================== 系统分类 ====================

// ensureSystemCategory 取（或按需创建）系统分类，返回其 ID。
// 转账手续费、余额调整等后端自动生成的交易需要归属分类，
// 用户手工删掉后也要能自动补回，避免生成 category_id=0 的脏数据。
//
// IsHidden=true：这类分类是**后端自己用**的，不应污染用户的分类管理页、
// 记账表单选择器和流水页筛选下拉（原 B-12）。流水列表通过服务端返回的
// category_name 展示，不受隐藏影响。
func ensureSystemCategory(db *gorm.DB, uid, bookID uint, name string, kind models.CategoryKind, icon string) uint {
	var cat models.Category
	q := db.Where("user_id = ? AND name = ?", uid, name)
	if bookID > 0 {
		q = q.Where("book_id IN ?", []uint{0, bookID})
	}
	if err := q.Order("book_id DESC").First(&cat).Error; err == nil {
		if !cat.IsHidden {
			// 历史数据补标
			db.Model(&models.Category{}).Where("id = ?", cat.ID).Update("is_hidden", true)
		}
		return cat.ID
	}
	cat = models.Category{
		UserID:   uid,
		BookID:   bookID,
		Name:     name,
		Kind:     kind,
		Icon:     icon,
		IsSystem: true,
		IsHidden: true,
	}
	if err := db.Create(&cat).Error; err != nil {
		return 0
	}
	return cat.ID
}

// ==================== 账本权限 ====================

// bookIDsCtx 请求级缓存键：账本成员关系在一次请求内被反复读取
// （列表至少 2 次、写接口 3~5 次），此前每次都单独打库（原 P-03）。
const bookIDsCtx = "hz_book_ids"

// accessibleBookIDs 当前用户可见的账本 ID：自己创建的 + 被邀请加入的（共享账本，C5）
func accessibleBookIDs(uid uint) []uint {
	var ids []uint
	database.DB.Model(&models.BookMember{}).Where("user_id = ?", uid).Pluck("book_id", &ids)
	return ids
}

// bookIDsOf 带请求级缓存的版本。c 为 nil（调度器 / 测试）时退化为直接查询。
func bookIDsOf(c *gin.Context, uid uint) []uint {
	if c == nil {
		return accessibleBookIDs(uid)
	}
	if v, ok := c.Get(bookIDsCtx); ok {
		if ids, ok2 := v.([]uint); ok2 {
			return ids
		}
	}
	ids := accessibleBookIDs(uid)
	c.Set(bookIDsCtx, ids)
	return ids
}

// applyBookScope 把查询范围从「仅本人」扩展为「本人 或 本人可见的共享账本」。
// 这是共享账本真正生效的关键：此前所有列表接口都只按 user_id 过滤，
// 被邀请成员看到的永远是空列表。
// 注意：必须用括号包裹，否则与后续 Where 用 AND 连接时，
// AND 优先级高于 OR 会让范围判断失效（SQL 优先级陷阱）。
func applyBookScope(c *gin.Context, q *gorm.DB, uid uint) *gorm.DB {
	ids := bookIDsOf(c, uid)
	if len(ids) == 0 {
		return q.Where("user_id = ?", uid)
	}
	return q.Where("(user_id = ? OR book_id IN ?)", uid, ids)
}

// canWriteBook 判断用户是否有某账本的写权限（owner 或 editor 成员）
func canWriteBook(c *gin.Context, uid, bookID uint) bool {
	if bookID == 0 {
		return false
	}
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
func ownsOrSharesAccount(c *gin.Context, uid, accountID uint) bool {
	if accountID == 0 {
		return false
	}
	var acc models.Account
	if err := database.DB.Where("id = ? AND user_id = ?", accountID, uid).First(&acc).Error; err == nil {
		return true
	}
	// 共享账本内的账户可能由他人创建
	ids := bookIDsOf(c, uid)
	if len(ids) == 0 {
		return false
	}
	return database.DB.Where("id = ? AND book_id IN ?", accountID, ids).First(&acc).Error == nil
}

// canWriteTx 判断用户是否可写某条已存在的交易。
//
// 修复背景（原 B-05）：此前 ListTransactions 走 applyBookScope（可见即可列），
// 而 Get/Update/Delete/BatchDelete/Recover/ListDeleted 全部只按 user_id 过滤，
// 共享账本成员「看得到但改不了」，且 CreateTransaction 用 canWriteBook 允许 editor 写入
// ——形成「能建不能改」的权限分叉。这里给出唯一的判定入口。
func canWriteTx(c *gin.Context, uid uint, tx *models.Transaction) bool {
	if tx.UserID == uid {
		// 自己记的账永远可改（即使账本已被删除/退出）
		return true
	}
	return canWriteBook(c, uid, tx.BookID)
}

// ==================== 标签 ====================

// adjustTagCounts 同步标签使用次数：delta=+1 关联时增加，delta=-1 解绑/删除时回退。
// 用 GREATEST(count + delta, 0) 兜底，避免历史脏数据把计数减成负数。
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

// adjustTagCountsBatch 批量版本：每个标签只发一条 UPDATE。
// 单次可减 N（如批量删除里同一标签被多笔流水引用）。
func adjustTagCountsBatch(db *gorm.DB, counts map[uint]int, deltaSign int) {
	for tagID, n := range counts {
		if n <= 0 {
			continue
		}
		d := n * deltaSign
		if d > 0 {
			db.Model(&models.Tag{}).Where("id = ?", tagID).
				UpdateColumn("count", gorm.Expr("count + ?", d))
			continue
		}
		db.Model(&models.Tag{}).Where("id = ?", tagID).
			UpdateColumn("count", gorm.Expr("GREATEST(count + ?, 0)", d))
	}
}

// txTagIDs 读取某笔交易当前关联的标签
func txTagIDs(db *gorm.DB, txID uint) []uint {
	var ids []uint
	db.Model(&models.TransactionTag{}).Where("transaction_id = ?", txID).
		Pluck("tag_id", &ids)
	return ids
}

// txTagIDsBatch 一次查询多笔交易的标签关联，避免 N+1（原 B-08）
func txTagIDsBatch(db *gorm.DB, txIDs []uint) map[uint][]uint {
	out := make(map[uint][]uint, len(txIDs))
	if len(txIDs) == 0 {
		return out
	}
	var links []models.TransactionTag
	db.Where("transaction_id IN ?", txIDs).Find(&links)
	for _, l := range links {
		out[l.TransactionID] = append(out[l.TransactionID], l.TagID)
	}
	return out
}

// syncTxTags 把某笔交易的标签集合增量同步到目标集合。
//
// 只处理「差集」：此前先 adjustTagCounts(旧, -1) 再 adjustTagCounts(新, +1)，
// 对**未被改动的标签**也会做一次减再加，计数漂移；而且 Delete-then-Create
// 在相同 (transaction_id, tag_id) 上会撞主键。
func syncTxTags(db *gorm.DB, txID uint, want []uint) {
	old := txTagIDs(db, txID)
	oldSet := make(map[uint]struct{}, len(old))
	for _, id := range old {
		oldSet[id] = struct{}{}
	}
	wantSet := make(map[uint]struct{}, len(want))
	for _, id := range want {
		wantSet[id] = struct{}{}
	}
	var removed, added []uint
	for _, id := range old {
		if _, ok := wantSet[id]; !ok {
			removed = append(removed, id)
		}
	}
	for _, id := range want {
		if _, ok := oldSet[id]; !ok {
			added = append(added, id)
		}
	}
	if len(removed) > 0 {
		db.Where("transaction_id = ? AND tag_id IN ?", txID, removed).
			Delete(&models.TransactionTag{})
		adjustTagCounts(db, removed, -1)
	}
	if len(added) > 0 {
		for _, tid := range added {
			db.Create(&models.TransactionTag{TransactionID: txID, TagID: tid})
		}
		adjustTagCounts(db, added, 1)
	}
}

// ==================== 派生交易 ====================

// revertDerivedTransactions 撤销某笔主交易派生出的子交易（转账手续费）。
// 删除或编辑主交易时必须同步回滚，否则账户余额会与流水对不上。
//
// 这里用**硬删除**（Unscoped）：派生交易是后端自动生成的，
// 此前用软删除会让每编辑一次转账就往回收站里塞一条系统垃圾，
// 把用户真正删除的记录挤出可视范围（原 B-07）。
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
		applyBudgetUsed(db, d.UserID, d.BookID, d.CategoryID, d.TxDate,
			d.AmountInBase(), d.Type, d.IncludeInBudget, -1)
		// 标签计数回退后再硬删关联行
		adjustTagCounts(db, txTagIDs(db, d.ID), -1)
		db.Where("transaction_id = ?", d.ID).Delete(&models.TransactionTag{})
		db.Unscoped().Delete(&models.Transaction{}, d.ID)
	}
}

// syncTransferFeeDerived 统一的「转账手续费」派生交易生成器。
// Create / Update / Recover 三处共用，避免逻辑漂移（此前 update 分支重建条件依赖
// 前端传来的 TransferFee，一旦被清零就静默丢钱，见原 B-01 的连锁反应）。
func syncTransferFeeDerived(db *gorm.DB, parent *models.Transaction, from *models.Account) {
	if parent.Type != models.TxTransfer || parent.TransferFee <= 0 {
		return
	}
	feeCat := ensureSystemCategory(db, parent.UserID, parent.BookID, "转账手续费", models.KindExpense, "💸")
	feeTx := models.Transaction{
		UserID:           parent.UserID,
		BookID:           parent.BookID,
		Type:             models.TxExpense,
		Amount:           parent.TransferFee,
		Currency:         parent.Currency,
		ExchangeRate:     parent.ExchangeRate,
		CategoryID:       feeCat,
		AccountID:        parent.AccountID,
		TxDate:           parent.TxDate,
		Description:      "转账手续费",
		RelatedTxID:      parent.ID,
		RelatedType:      models.RelatedTransferFee,
		IncludeInBalance: true,
		IncludeInBudget:  true,
	}
	if err := db.Create(&feeTx).Error; err != nil {
		return
	}
	feeFrom := *from
	updateAccountBalances(db, &feeTx, &feeFrom, nil, true)
	applyBudgetUsed(db, feeTx.UserID, feeTx.BookID, feeCat, feeTx.TxDate,
		feeTx.AmountInBase(), feeTx.Type, true, 1)
}

// ==================== 导出给调度器 ====================

// UpdateAccountBalances 导出给 cmd/huozhi-server 的调度器复用。
// 此前周期记账在 main.go 里维护了一份残缺副本 updateRecurBalances：
// 没有负债账户方向处理、也不更新预算，与手工记账口径不一致（C7）。
func UpdateAccountBalances(db *gorm.DB, tx *models.Transaction, from, to *models.Account, isAdd bool) {
	updateAccountBalances(db, tx, from, to, isAdd)
}

// ApplyBudgetUsed 导出给调度器复用（同上）。
// amount 必须是已折算到基准币种的金额（tx.AmountInBase()）。
func ApplyBudgetUsed(db *gorm.DB, uid, bookID, catID uint, date time.Time,
	amount models.Money, txType models.TransactionType, includeInBudget bool, factor int64) {
	applyBudgetUsed(db, uid, bookID, catID, date, amount, txType, includeInBudget, factor)
}

// countBookChildren 账本下是否已有交易（删除账本前的孤儿数据校验用）
func countBookChildren(db *gorm.DB, uid, bookID uint) (txs, accounts, categories int64) {
	db.Model(&models.Transaction{}).Where("user_id = ? AND book_id = ?", uid, bookID).Count(&txs)
	db.Model(&models.Account{}).Where("user_id = ? AND book_id = ?", uid, bookID).Count(&accounts)
	db.Model(&models.Category{}).Where("user_id = ? AND book_id = ?", uid, bookID).Count(&categories)
	return
}
