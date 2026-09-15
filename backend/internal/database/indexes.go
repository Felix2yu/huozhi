package database

import (
	"log"

	"gorm.io/gorm"
)

// EnsureIndexes 创建 AutoMigrate 无法表达的复合索引。
//
// 流水列表的主排序是 `ORDER BY tx_date DESC, id DESC`（ListTransactions），
// 深翻页时 OFFSET N 需要全量排序扫描，开销随页数线性增长（原 P-06）。
// （user_id, book_id, tx_date DESC, id DESC）让「按可见范围 + 日期倒序」
// 完全走索引，配合游标分页后扫描行数恒等于 page_size。
//
// SQLite 与 PostgreSQL 都支持 CREATE INDEX IF NOT EXISTS，因此幂等可重复执行。
func EnsureIndexes(db *gorm.DB) {
	if db == nil {
		return
	}
	stmts := []struct {
		name string
		sql  string
	}{
		{
			name: "transactions 列表主查询",
			sql:  "CREATE INDEX IF NOT EXISTS idx_tx_list_scope ON transactions (user_id, book_id, tx_date DESC, id DESC)",
		},
		{
			name: "transactions 账本+日期",
			sql:  "CREATE INDEX IF NOT EXISTS idx_tx_book_date ON transactions (book_id, tx_date DESC, deleted_at)",
		},
		{
			name: "transactions 类型+日期（统计/分类型汇总）",
			sql:  "CREATE INDEX IF NOT EXISTS idx_tx_type_date ON transactions (type, tx_date DESC)",
		},
		{
			name: "transactions 回收站",
			sql:  "CREATE INDEX IF NOT EXISTS idx_tx_deleted ON transactions (user_id, deleted_at)",
		},
		{
			name: "transactions 派生交易定位",
			sql:  "CREATE INDEX IF NOT EXISTS idx_tx_related ON transactions (related_tx_id, related_type)",
		},
	}
	for _, s := range stmts {
		if err := db.Exec(s.sql).Error; err != nil {
			// 索引创建失败不影响服务启动：退化为全表扫描，但功能可用
			log.Printf("[DB] 索引创建失败（%s）: %v", s.name, err)
		}
	}
}
