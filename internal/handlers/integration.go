package handlers

import (
	"strconv"
	"strings"

	"huozhi/internal/database"
	"huozhi/internal/dto"
	"huozhi/internal/middleware"
	"huozhi/internal/models"

	"github.com/gin-gonic/gin"
)

// ===== 外部系统只读对接（API Key） =====
//
// 给自建系统（如「牵丝」的往来记录）按账单关联开销用。
// 为什么不直接把 API Key 放进 /api/transactions 的鉴权链：
// 长期密钥只该拿到「读」，写操作仍必须走登录态——一次误删虽然进回收站，
// 但外部系统批量改账的爆炸半径远大于它带来的便利。

// IntegrationBill 对外账单视图。
// 金额用「分」（amount_fen）而非 Web 端习惯的「元」：跨系统传递落整数最不容易出歧义，
// 且下游（牵丝）全链路以分为单位存储。字段名显式带 _fen，避免调用方误按元处理。
type IntegrationBill struct {
	ID           uint   `json:"id"`
	AmountFen    int64  `json:"amount_fen"`
	Type         string `json:"type"`
	TxDate       string `json:"tx_date"`
	Description  string `json:"description"`
	Merchant     string `json:"merchant"`
	Location     string `json:"location"`
	CategoryID   uint   `json:"category_id"`
	CategoryName string `json:"category_name"`
	AccountID    uint   `json:"account_id"`
	AccountName  string `json:"account_name"`
	BookID       uint   `json:"book_id"`
	BookName     string `json:"book_name"`
}

// maxIntegrationIDs 单次按 id 批量取的上限，防止外部系统一把梭拉全库。
const maxIntegrationIDs = 200

// IntegrationListBills GET /api/integration/bills
//
// 两种模式：
//   - 常规搜索：keyword / start_date / end_date / type / page / page_size，
//     直接复用 ListTx —— 与 Web 端、MCP 共用同一套筛选口径，不允许出现第三份规则；
//   - 按 id 批量取：?ids=1,2,3，牵丝刷新已关联账单快照时用，不受分页影响。
func IntegrationListBills(c *gin.Context) {
	uid := middleware.GetUID(c)
	if raw := strings.TrimSpace(c.Query("ids")); raw != "" {
		ids := parseIntegrationIDs(raw)
		if len(ids) == 0 {
			OK(c, gin.H{"items": []IntegrationBill{}})
			return
		}
		q := ScopeByBooks(database.DB.Model(&models.Transaction{}), uid, VisibleBookIDs(uid))
		var rows []models.Transaction
		if err := q.Where("id IN ?", ids).Order("tx_date DESC, id DESC").Find(&rows).Error; err != nil {
			Bad(c, err.Error())
			return
		}
		OK(c, gin.H{"items": integrationBills(rows)})
		return
	}

	var req dto.QueryTransactionRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		Bad(c, err.Error())
		return
	}
	res, cerr := ListTx(uid, req)
	if cerr != nil {
		FailFromCore(c, cerr)
		return
	}
	flat := make([]models.Transaction, 0, res.PageSize)
	for _, g := range res.Grouped {
		flat = append(flat, g.Transactions...)
	}
	OK(c, gin.H{
		"items":     integrationBills(flat),
		"total":     res.Total,
		"page":      res.Page,
		"page_size": res.PageSize,
	})
}

// IntegrationGetBill GET /api/integration/bills/:id
func IntegrationGetBill(c *gin.Context) {
	uid := middleware.GetUID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		Bad(c, "账单 id 不合法")
		return
	}
	q := ScopeByBooks(database.DB.Model(&models.Transaction{}), uid, VisibleBookIDs(uid))
	var tx models.Transaction
	if err := q.Where("id = ?", id).First(&tx).Error; err != nil {
		NotFound(c, "账单不存在或无权访问")
		return
	}
	OK(c, integrationBills([]models.Transaction{tx})[0])
}

// integrationBills 把交易模型转成对外视图，并批量补齐分类/账户/账本名称。
// 批量查字典而不是逐条 Preload：一页最多 200 条，逐条 Preload 会放大成数百次小查询。
func integrationBills(rows []models.Transaction) []IntegrationBill {
	out := make([]IntegrationBill, 0, len(rows))
	if len(rows) == 0 {
		return out
	}
	catIDs := make([]uint, 0, len(rows))
	acctIDs := make([]uint, 0, len(rows))
	bookIDs := make([]uint, 0, len(rows))
	for _, t := range rows {
		catIDs = append(catIDs, t.CategoryID)
		acctIDs = append(acctIDs, t.AccountID)
		bookIDs = append(bookIDs, t.BookID)
	}
	cats := nameMap[models.Category](catIDs)
	accts := nameMap[models.Account](acctIDs)
	books := nameMap[models.Book](bookIDs)

	for _, t := range rows {
		out = append(out, IntegrationBill{
			ID:           t.ID,
			AmountFen:    int64(t.Amount),
			Type:         string(t.Type),
			TxDate:       t.TxDate.Format("2006-01-02"),
			Description:  t.Description,
			Merchant:     t.Merchant,
			Location:     t.Location,
			CategoryID:   t.CategoryID,
			CategoryName: cats[t.CategoryID],
			AccountID:    t.AccountID,
			AccountName:  accts[t.AccountID],
			BookID:       t.BookID,
			BookName:     books[t.BookID],
		})
	}
	return out
}

// nameMap 批量取 id → name。查不到的下标留空串，由调用方按空值处理，
// 不能让「分类被删了」把整条账单的展示也一起吞掉。
func nameMap[T models.Category | models.Account | models.Book](ids []uint) map[uint]string {
	out := map[uint]string{}
	uniq := make([]uint, 0, len(ids))
	seen := map[uint]bool{}
	for _, id := range ids {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		uniq = append(uniq, id)
	}
	if len(uniq) == 0 {
		return out
	}
	var rows []T
	database.DB.Where("id IN ?", uniq).Find(&rows)
	for _, r := range rows {
		switch v := any(r).(type) {
		case models.Category:
			out[v.ID] = v.Name
		case models.Account:
			out[v.ID] = v.Name
		case models.Book:
			out[v.ID] = v.Name
		}
	}
	return out
}

// parseIntegrationIDs 解析 "1,2,3"，忽略非法项并按上限截断。
func parseIntegrationIDs(raw string) []uint {
	parts := strings.Split(raw, ",")
	out := make([]uint, 0, len(parts))
	seen := map[uint]bool{}
	for _, p := range parts {
		n, err := strconv.ParseUint(strings.TrimSpace(p), 10, 64)
		if err != nil || n == 0 || seen[uint(n)] {
			continue
		}
		seen[uint(n)] = true
		out = append(out, uint(n))
		if len(out) >= maxIntegrationIDs {
			break
		}
	}
	return out
}
