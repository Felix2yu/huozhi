package handlers

import (
	"strconv"
	"strings"

	"huozhi/internal/database"
	"huozhi/internal/middleware"
	"huozhi/internal/models"

	"github.com/gin-gonic/gin"
)

// ===== 对外只读账单接口（API Key） =====
//
// 这一组是货殖对外的**唯一**账单契约，目前有两个消费方：
//   - 幕间（演出记录）：按 id 取单张账单，展示门票/其他开销；
//   - 牵丝（往来记录）：搜索并勾选多张账单，把往来开销挂到账单上。
//
// 双方都只需要「读」，所以鉴权只认 X-API-Key，没有写接口。
// 新增字段一律**向后兼容**（只加不改），老消费方忽略未知字段即可。

// PublicBill 对外账单视图。
//
// 金额同时给两种口径：
//   - amount：元（浮点），历史字段，幕间在用；
//   - amount_fen：分（整数），跨系统传整数不会出现浮点误差，牵丝在用。
type PublicBill struct {
	ID          uint         `json:"id"`
	Description string       `json:"description"`
	Amount      models.Money `json:"amount"`
	AmountFen   int64        `json:"amount_fen"`
	Currency    string       `json:"currency"`
	Type        string       `json:"type"`
	TxDate      string       `json:"tx_date"`
	Merchant    string       `json:"merchant"`
	Remark      string       `json:"remark"`
	// 以下为补充字段（可空）：幕间用 category 判定门票/其他开销，
	// 牵丝在关联列表里展示分类与账户。查不到的字典项留空串。
	Category string `json:"category"`
	Account  string `json:"account"`
	Book     string `json:"book"`
}

// maxPublicBillIDs 按 id 批量取的上限，防止外部系统一把梭拉全库。
const maxPublicBillIDs = 200

// GetPublicBill 获取单张账单（供外部系统查询），需要有效的 API key。
func GetPublicBill(c *gin.Context) {
	uid := middleware.GetUID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		Bad(c, "无效的账单ID")
		return
	}
	var tx models.Transaction
	if err := database.DB.Where("id = ? AND user_id = ?", uint(id), uid).First(&tx).Error; err != nil {
		NotFound(c, "账单不存在")
		return
	}
	OK(c, publicBills([]models.Transaction{tx})[0])
}

// ListPublicBills 获取账单列表（供外部系统查询）。
//
// 支持：
//   - 分页：page / page_size（≤100）
//   - 时间范围：start_date / end_date（按 tx_date，含当日）
//   - 类型：type（expense / income / transfer …）
//   - 关键词：keyword（备注、商家、地点、附加说明）
//   - 精确取：ids=1,2,3（刷新已关联账单用，忽略分页）
//
// 历史行为保持不变：只按 user_id 过滤，按 tx_date 倒序。
func ListPublicBills(c *gin.Context) {
	uid := middleware.GetUID(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	q := database.DB.Where("user_id = ?", uid)

	if ids := parsePublicBillIDs(c.Query("ids")); len(ids) > 0 {
		// ids 模式忽略分页与其它筛选：调用方要的就是这几张的现状
		var list []models.Transaction
		if err := q.Where("id IN ?", ids).Order("tx_date DESC, id DESC").Find(&list).Error; err != nil {
			Bad(c, err.Error())
			return
		}
		PagedOK(c, publicBills(list), 1, len(list), int64(len(list)))
		return
	}

	if startDate := c.Query("start_date"); startDate != "" {
		q = q.Where("tx_date >= ?", startDate)
	}
	if endDate := c.Query("end_date"); endDate != "" {
		q = q.Where("tx_date <= ?", endDate+" 23:59:59")
	}
	if typ := strings.TrimSpace(c.Query("type")); typ != "" {
		q = q.Where("type = ?", typ)
	}
	if kw := strings.TrimSpace(c.Query("keyword")); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("description LIKE ? OR merchant LIKE ? OR location LIKE ? OR remark LIKE ?",
			like, like, like, like)
	}

	var total int64
	q.Model(&models.Transaction{}).Count(&total)

	var list []models.Transaction
	offset := (page - 1) * pageSize
	q.Order("tx_date DESC, id DESC").Offset(offset).Limit(pageSize).Find(&list)

	PagedOK(c, publicBills(list), page, pageSize, total)
}

// publicBills 把交易模型转成对外视图，并批量补齐分类/账户/账本名称。
// 批量查字典而不是逐条 Preload：一页最多 100 条，逐条 Preload 会放大成数百次小查询。
func publicBills(rows []models.Transaction) []PublicBill {
	out := make([]PublicBill, 0, len(rows))
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
	cats := dictName[models.Category](catIDs)
	accts := dictName[models.Account](acctIDs)
	books := dictName[models.Book](bookIDs)

	for _, t := range rows {
		out = append(out, PublicBill{
			ID:          t.ID,
			Description: t.Description,
			Amount:      t.Amount,
			AmountFen:   int64(t.Amount),
			Currency:    t.Currency,
			Type:        string(t.Type),
			// 格式保持历史原样（幕间按此解析），需要日期的话调用方取前 10 位
			TxDate:   t.TxDate.Format("2006-01-02 15:04:05"),
			Merchant: t.Merchant,
			Remark:   t.Remark,
			Category: cats[t.CategoryID],
			Account:  accts[t.AccountID],
			Book:     books[t.BookID],
		})
	}
	return out
}

// dictName 批量取 id → name。查不到的留空串，
// 不能让「分类被删了」把整条账单的展示也一起吞掉。
func dictName[T models.Category | models.Account | models.Book](ids []uint) map[uint]string {
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

// parsePublicBillIDs 解析 "1,2,3"，忽略非法项并按上限截断。
func parsePublicBillIDs(raw string) []uint {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
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
		if len(out) >= maxPublicBillIDs {
			break
		}
	}
	return out
}
