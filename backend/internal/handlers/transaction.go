package handlers

import (
	"encoding/json"
	"fmt"
	"huozhi/internal/database"
	"huozhi/internal/dto"
	"huozhi/internal/middleware"
	"huozhi/internal/models"
	"huozhi/internal/ws"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ========== 交易 Transaction ==========

// CreateTransaction 创建交易（核心：同步更新账户余额）
func CreateTransaction(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.CreateTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Bad(c, "参数错误: "+err.Error())
		return
	}

	// 转账类型必须有目标账户
	if req.Type == "transfer" && req.ToAccountID == 0 {
		Bad(c, "转账需要指定目标账户")
		return
	}
	// 汇率兜底：0 / 负数一律按 1（1 单位原币 = 1 单位基准币）。
	// 放任 0 入库会让后续任何折算都得到 0。
	if req.ExchangeRate <= 0 || math.IsNaN(req.ExchangeRate) || math.IsInf(req.ExchangeRate, 0) {
		req.ExchangeRate = 1
	}
	if req.ReimburseStatus == "" {
		req.ReimburseStatus = "none"
	}

	dbtx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			dbtx.Rollback()
		}
	}()

	// 读取账户（加乐观锁，余额操作要谨慎）。
	// 共享账本里的账户可能由他人创建，不能只按 user_id 校验（C5）。
	var fromAcc, toAcc models.Account
	ids := bookIDsOf(c, uid)
	accQuery := func(id uint, out *models.Account) error {
		if len(ids) > 0 {
			return dbtx.Where("id = ? AND (user_id = ? OR book_id IN ?)", id, uid, ids).First(out).Error
		}
		return dbtx.Where("id = ? AND user_id = ?", id, uid).First(out).Error
	}
	if err := accQuery(req.AccountID, &fromAcc); err != nil {
		dbtx.Rollback()
		Bad(c, "来源账户不存在")
		return
	}
	if req.ToAccountID > 0 {
		if err := accQuery(req.ToAccountID, &toAcc); err != nil {
			dbtx.Rollback()
			Bad(c, "目标账户不存在")
			return
		}
	}
	// 归属校验：必须对该账本有写权限（本人账本或共享账本的 editor/owner）
	if !canWriteBook(c, uid, req.BookID) {
		dbtx.Rollback()
		Forbidden(c, "对该账本无写入权限")
		return
	}

	// 构建交易
	newTx := models.Transaction{
		UserID:           uid,
		BookID:           req.BookID,
		Type:             models.TransactionType(req.Type),
		Amount:           models.FromYuan(req.Amount),
		Currency:         firstNotEmpty(req.Currency, "CNY"),
		ExchangeRate:     req.ExchangeRate,
		CategoryID:       req.CategoryID,
		AccountID:        req.AccountID,
		ToAccountID:      req.ToAccountID,
		TransferFee:      models.FromYuan(req.TransferFee),
		TransferDiscount: models.FromYuan(req.TransferDiscount),
		RefundOfID:       req.RefundOfID,
		TxDate:           req.TxDate.T(),
		Description:      req.Description,
		Images:           req.Images,
		Merchant:         req.Merchant,
		Location:         req.Location,
		// C3/C12：未显式传值时默认 true，显式传 false 时尊重客户端
		// （旧实现强制改回 true，导致 DTO 暴露的字段永远无效）
		IncludeInBalance: req.BalanceFlag(),
		IncludeInBudget:  req.BudgetFlag(),
		RecurringID:      req.RecurringID,
		InstallmentID:    req.InstallmentID,
		Remark:           req.Remark,
		ReimburseStatus:  req.ReimburseStatus,
	}
	if err := dbtx.Create(&newTx).Error; err != nil {
		dbtx.Rollback()
		InternalErr(c, "创建交易失败: "+err.Error())
		return
	}

	// 标签关联
	if len(req.TagIDs) > 0 {
		for _, tid := range req.TagIDs {
			dbtx.Create(&models.TransactionTag{TransactionID: newTx.ID, TagID: tid})
		}
		adjustTagCounts(dbtx, req.TagIDs, 1)
		var tags []models.Tag
		dbtx.Where("id IN ?", req.TagIDs).Find(&tags)
		tagPtrs := make([]*models.Tag, len(tags))
		for i := range tags {
			tagPtrs[i] = &tags[i]
		}
		newTx.Tags = tagPtrs
	}

	// 更新账户余额（内部按 AmountInBase() 折算到基准币种）
	updateAccountBalances(dbtx, &newTx, &fromAcc, &toAcc, true)

	// C8：转账手续费生成独立支出交易，保证「每笔资金变动都有流水」
	syncTransferFeeDerived(dbtx, &newTx, &fromAcc)

	applyBudgetUsed(dbtx, uid, newTx.BookID, newTx.CategoryID, newTx.TxDate,
		newTx.AmountInBase(), newTx.Type, newTx.IncludeInBudget, 1)

	if err := dbtx.Commit().Error; err != nil {
		dbtx.Rollback()
		InternalErr(c, "提交失败")
		return
	}

	// 预算提醒：只针对本次支出真正相关的预算，且只在跨过阈值那一刻推送（原 B-11）
	broadcastBudgetAlerts(c, uid, newTx)

	// 重新加载完整信息（同样要走切片，否则派生字段填不到返回值上）
	database.DB.Preload("Tags").First(&newTx, newTx.ID)
	enriched := []models.Transaction{newTx}
	fillTxViewFields(enriched)
	Broadcast(c, "transactions", "create", newTx.ID)
	Created(c, enriched[0])
}

// broadcastBudgetAlerts 预算超限提醒。
//
// 此前该逻辑查询账本下**所有** used_amount > amount 的预算并推送，既不检查
// 是哪一类预算被突破，也不使用 budget 自身的 alert_rate：用户每记一笔账都会
// 收到一条与自己无关的「你有 N 个预算超支」。改为：
//  1. 只看本次支出的分类预算 + 该账本总预算；
//  2. 按 alert_rate 判断；
//  3. 只在「使用率由 below → above 跨过阈值」那一刻提醒，后续同类消费不再打扰。
func broadcastBudgetAlerts(c *gin.Context, uid uint, t models.Transaction) {
	if t.Type != models.TxExpense || !t.IncludeInBudget || t.BookID == 0 {
		return
	}
	var budgets []models.Budget
	database.DB.Where(
		"user_id = ? AND book_id = ? AND category_id IN ? AND start_date <= ? AND end_date >= ?",
		uid, t.BookID, []uint{0, t.CategoryID}, t.TxDate, t.TxDate,
	).Find(&budgets)
	if len(budgets) == 0 {
		return
	}
	delta := float64(t.AmountInBase())
	var hit []models.Budget
	for _, b := range budgets {
		if b.Amount <= 0 {
			continue
		}
		rate := b.AlertRate
		if rate <= 0 {
			rate = 0.8
		}
		after := float64(b.UsedAmount) / float64(b.Amount)
		before := after - delta/float64(b.Amount)
		// 跨越阈值的那一刻才提醒；此前已超阈值的不重复打扰
		if before < rate && after >= rate {
			hit = append(hit, b)
		}
	}
	if len(hit) == 0 {
		return
	}
	names := budgetNames(hit)
	data, _ := json.Marshal(map[string]interface{}{
		"count":     len(names),
		"budgets":   names,
		"tx_amount": t.AmountInBase(),
	})
	ws.DefaultHub.BroadcastWithData(uid, ws.Message{
		Type:   "alert",
		Table:  "budgets",
		Action: "over_budget",
		Data:   data,
	})
}

// budgetNames 把预算对象转成人类可读的名字，用于提醒文案（一次批量查询）
func budgetNames(list []models.Budget) []string {
	catIDs := make([]uint, 0, len(list))
	for _, b := range list {
		if b.CategoryID > 0 {
			catIDs = append(catIDs, b.CategoryID)
		}
	}
	nameMap := make(map[uint]string, len(catIDs))
	if len(catIDs) > 0 {
		var cats []models.Category
		database.DB.Select("id", "name").Where("id IN ?", catIDs).Find(&cats)
		for _, c := range cats {
			nameMap[c.ID] = c.Name
		}
	}
	out := make([]string, 0, len(list))
	for _, b := range list {
		if b.CategoryID == 0 {
			out = append(out, "总预算")
			continue
		}
		out = append(out, orDefault(nameMap[b.CategoryID], fmt.Sprintf("分类#%d", b.CategoryID)))
	}
	return out
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// isDebtAccount 信用卡 / 负债类账户：balance 正数表示「欠款」，与 statistics 资产概览口径一致。
// 这类账户的余额方向与普通资产账户相反，记账时需要对消增减方向。
func isDebtAccount(a *models.Account) bool {
	return a != nil && (a.Type == models.AccCredit || a.Type == models.AccLiability)
}

// debtSign 返回账户余额方向系数：普通资产账户为 +1（正数=我拥有的钱）；
// 信用卡/负债账户为 -1（正数=欠款，增减方向与资产相反）。
func debtSign(a *models.Account) int64 {
	if isDebtAccount(a) {
		return -1
	}
	return 1
}

// updateAccountBalances 按整数分运算更新余额（金额列为 bigint，无浮点误差）。
//
// 资金方向统一取自 models.TxBalanceDirection（单一事实来源），
// 金额统一取自 AmountInBase()（乘以汇率折算到基准币种）。
func updateAccountBalances(db *gorm.DB, t *models.Transaction, from, to *models.Account, isAdd bool) {
	if !t.IncludeInBalance {
		return
	}
	factor := int64(1)
	if !isAdd {
		factor = -1
	}
	// ds = 账户方向系数，债务类账户取反
	dsFrom := debtSign(from)
	dsTo := debtSign(to)
	amount := int64(t.AmountInBase())
	discount := int64(t.ToBaseMoney(t.TransferDiscount))

	switch models.TxBalanceDirection(t.Type) {
	case -1:
		// 支出 / 报销：from 账户余额减少（信用卡则欠款增加）
		db.Model(from).Update("balance", gorm.Expr("balance - ?", amount*factor*dsFrom))
	case 1:
		// 收入 / 退款：from 账户余额增加（信用卡则欠款减少）
		db.Model(from).Update("balance", gorm.Expr("balance + ?", amount*factor*dsFrom))
	default:
		if t.Type == models.TxTransfer {
			// 转账：from 减少 amount、增加 discount（信用卡侧方向取反）；to 增加 amount。
			// 手续费（C8）不再在此处静默扣减，而是生成一条独立的「转账手续费」支出交易。
			db.Model(from).Update("balance", gorm.Expr("balance - ? + ?", amount*factor*dsFrom, discount*factor*dsFrom))
			if to != nil && to.ID > 0 {
				db.Model(to).Update("balance", gorm.Expr("balance + ?", amount*factor*dsTo))
			}
		}
		// TxAdjust：余额由 AdjustAccount 直接置值，流水 IncludeInBalance=false，
		// 余额引擎不得二次加减，否则会把账户余额算双倍。
	}
}

// accumulateBalanceDelta 把单笔交易对各账户余额的影响聚合成 map，
// 供批量删除一次性 UPDATE，避免 O(N) 次逐账户回写（原 B-08）。
// mult=+1 施加该交易的影响，mult=-1 撤销。
func accumulateBalanceDelta(t *models.Transaction, from, to *models.Account, deltas map[uint]int64, mult int64) {
	if !t.IncludeInBalance {
		return
	}
	amount := int64(t.AmountInBase())
	discount := int64(t.ToBaseMoney(t.TransferDiscount))
	dsFrom := debtSign(from)
	dsTo := debtSign(to)
	switch models.TxBalanceDirection(t.Type) {
	case -1:
		if from != nil && from.ID > 0 {
			deltas[from.ID] -= amount * mult * dsFrom
		}
	case 1:
		if from != nil && from.ID > 0 {
			deltas[from.ID] += amount * mult * dsFrom
		}
	default:
		if t.Type == models.TxTransfer {
			if from != nil && from.ID > 0 {
				deltas[from.ID] += (-amount + discount) * mult * dsFrom
			}
			if to != nil && to.ID > 0 {
				deltas[to.ID] += amount * mult * dsTo
			}
		}
	}
}

// flushBalanceDeltas 将聚合好的余额增量一次性写回（每个账户一条 UPDATE）。
func flushBalanceDeltas(db *gorm.DB, deltas map[uint]int64) {
	accIDs := make([]uint, 0, len(deltas))
	for id := range deltas {
		accIDs = append(accIDs, id)
	}
	sort.Slice(accIDs, func(i, j int) bool { return accIDs[i] < accIDs[j] })
	for _, id := range accIDs {
		d := deltas[id]
		if d == 0 {
			continue
		}
		db.Model(&models.Account{}).Where("id = ?", id).
			Update("balance", gorm.Expr("balance + ?", d))
	}
}

// applyBudgetUsed 应用预算 used_amount 变更（支持所有 period_type，通过 start/end_date 范围匹配）
// factor=+1 增加（创建/修改为新值），factor=-1 减少（删除/撤销旧值）
//
// 注意：amount 必须是**已折算到基准币种**的金额（tx.AmountInBase()），
// 否则外币支出的预算占用会按原币金额计算（原 B-02 的一部分）。
func applyBudgetUsed(db *gorm.DB, uid, bookID, catID uint, date time.Time, amount models.Money, txType models.TransactionType, includeInBudget bool, factor int64) {
	if models.TxStatsBucket(txType) != models.StatsBucketExpense || !includeInBudget || amount <= 0 {
		return
	}
	delta := int64(amount) * factor
	// 总预算（category_id=0）
	db.Model(&models.Budget{}).
		Where("user_id = ? AND book_id = ? AND category_id = 0 AND start_date <= ? AND end_date >= ?",
			uid, bookID, date, date).
		Update("used_amount", gorm.Expr("used_amount + ?", delta))
	// 分类预算
	if catID > 0 {
		db.Model(&models.Budget{}).
			Where("user_id = ? AND book_id = ? AND category_id = ? AND start_date <= ? AND end_date >= ?",
				uid, bookID, catID, date, date).
			Update("used_amount", gorm.Expr("used_amount + ?", delta))
	}
}

// ========== 派生视图字段 ==========

// fillTxViewFields 批量补齐列表/详情返回体里的只读派生字段。
//
// 1) amount_base：外币流水折算后的基准币种金额（原 B-02 的前端可见部分）；
// 2) category_name / account_name / to_account_name：此前前端靠本地字典反查，
//    而字典是按 currentBookId 加载的，「全部账本」视图下跨账本流水必然查不到，
//    稳定显示「未分类」和「—」（原 P-06）。改为服务端用两次批量查询补齐。
func fillTxViewFields(list []models.Transaction) {
	if len(list) == 0 {
		return
	}
	catSet := make(map[uint]struct{}, len(list))
	accSet := make(map[uint]struct{}, len(list)*2)
	for i := range list {
		list[i].AmountBase = list[i].AmountInBase()
		if list[i].CategoryID > 0 {
			catSet[list[i].CategoryID] = struct{}{}
		}
		if list[i].AccountID > 0 {
			accSet[list[i].AccountID] = struct{}{}
		}
		if list[i].ToAccountID > 0 {
			accSet[list[i].ToAccountID] = struct{}{}
		}
	}
	if len(catSet) > 0 {
		var cats []models.Category
		database.DB.Select("id", "name").Where("id IN ?", idSetKeys(catSet)).Find(&cats)
		names := make(map[uint]string, len(cats))
		for _, c := range cats {
			names[c.ID] = c.Name
		}
		for i := range list {
			if n, ok := names[list[i].CategoryID]; ok {
				list[i].CategoryName = n
			}
		}
	}
	if len(accSet) > 0 {
		var accs []models.Account
		database.DB.Select("id", "name").Where("id IN ?", idSetKeys(accSet)).Find(&accs)
		names := make(map[uint]string, len(accs))
		for _, a := range accs {
			names[a.ID] = a.Name
		}
		for i := range list {
			if n, ok := names[list[i].AccountID]; ok {
				list[i].AccountName = n
			}
			if n, ok := names[list[i].ToAccountID]; ok {
				list[i].ToAccountName = n
			}
		}
	}
}

// idSetKeys 取出 ID 集合的键并排序（顺序稳定 -> SQL 稳定，便于命中缓存）
func idSetKeys(m map[uint]struct{}) []uint {
	out := make([]uint, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// isFiniteFloat 判断浮点数是否为有限值（非 NaN、非 ±Inf）。
// Go 标准库没有 math.IsFinite，这里统一提供，避免各处用不同写法校验。
func isFiniteFloat(f float64) bool {
	return !math.IsNaN(f) && !math.IsInf(f, 0)
}

// GetTransaction 获取单条
func GetTransaction(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.IDRequest
	if err := c.ShouldBindUri(&req); err != nil {
		Bad(c, "参数错误")
		return
	}
	// 读权限与列表一致：本人 或 可见共享账本（原 B-05）。
	// 此前只按 user_id 过滤，共享账本成员能看见却读不到详情。
	var tx models.Transaction
	if err := applyBookScope(c, database.DB.Model(&models.Transaction{}), uid).
		Preload("Tags").Where("id = ?", req.ID).First(&tx).Error; err != nil {
		NotFound(c, "交易不存在")
		return
	}
	// 注意：必须先把 tx 放进切片再填充 —— 直接传 []T{tx} 是传值副本，
	// 填充的是副本，返回值里拿不到 category_name / account_name。
	list := []models.Transaction{tx}
	fillTxViewFields(list)
	OK(c, list[0])
}

// ListTransactions 交易列表（分页+按日分组）
func ListTransactions(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.QueryTransactionRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		Bad(c, err.Error())
		return
	}
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 || req.PageSize > 200 {
		req.PageSize = 20
	}

	// buildBase 是列表 / 计数 / 汇总**共用的唯一**筛选构造器。
	// 此前汇总查询单独搭了一条只有 book_id + 日期的 SQL，其余 7 类筛选全丢，
	// 顶部「收支结余」与筛选后的列表完全对不上（原 B-03）。
	// withBalanceOnly=true 时额外加 include_in_balance 过滤，
	// 使汇总口径与统计页（statistics.go）一致。
	buildBase := func(withBalanceOnly bool) *gorm.DB {
		q := applyBookScope(c, database.DB.Model(&models.Transaction{}), uid)
		if req.BookID > 0 {
			q = q.Where("book_id = ?", req.BookID)
		}
		if req.Type != "" {
			q = q.Where("type = ?", req.Type)
		}
		if req.CategoryID > 0 {
			q = q.Where("category_id = ?", req.CategoryID)
		}
		if req.AccountID > 0 {
			q = q.Where("account_id = ? OR to_account_id = ?", req.AccountID, req.AccountID)
		}
		if !req.StartDate.IsZero() {
			q = q.Where("tx_date >= ?", req.StartDate.T())
		}
		// 半开区间 [start, end+1d)：此前用 `<= end+1d`，会把次日 00:00:00 的记录
		// 一并纳入结果，与统计页的 `<` 口径不一致（原 B-09）。
		if !req.EndDate.IsZero() {
			q = q.Where("tx_date < ?", req.EndDate.T().AddDate(0, 0, 1))
		}
		if req.Keyword != "" {
			k := "%" + req.Keyword + "%"
			like := database.LikeExpr()

			args := []interface{}{k, k, k, k, k}
			amountCond := ""
			// 数字关键词：按 ±10% 区间近似匹配，而不是精确等值。
			// 精确等值会让房间号「302」把所有恰好 302.00 元的流水翻出来（原 B-10）；
			// ParseFloat 会接受 NaN / ±Inf，必须先做有限性校验。
			// 数字关键词的健壮性校验：ParseFloat 接受 NaN / ±Inf，
			// 直接参与金额比较会得到无意义谓词，先做有限性校验。
			if n, err := strconv.ParseFloat(req.Keyword, 64); err == nil && isFiniteFloat(n) && n > 0 {
				amountCond = " OR (amount >= ? AND amount <= ?)"
				args = append(args, models.FromYuan(n*0.9), models.FromYuan(n*1.1))
			}
			q = q.Where(
				"(description "+like+" ? OR merchant "+like+" ? OR remark "+like+
					" ? OR location "+like+" ? OR category_id IN (SELECT id FROM categories WHERE name "+
					like+" ?)"+amountCond+")",
				args...,
			)
		}
		if req.MinAmount > 0 {
			q = q.Where("amount >= ?", models.FromYuan(req.MinAmount))
		}
		if req.MaxAmount > 0 {
			q = q.Where("amount <= ?", models.FromYuan(req.MaxAmount))
		}
		if req.TagID > 0 {
			q = q.Joins("JOIN transaction_tags tt ON transactions.id = tt.transaction_id").
				Where("tt.tag_id = ?", req.TagID)
		}
		if req.ReimburseStatus != "" {
			q = q.Where("reimburse_status = ?", req.ReimburseStatus)
		}
		if withBalanceOnly {
			q = q.Where("include_in_balance = ?", true)
		}
		return q
	}

	var total int64
	buildBase(false).Count(&total)

	var list []models.Transaction
	q := buildBase(false).Preload("Tags")
	if req.UseCursor() {
		// 游标分页：从上一页最后一条的 (tx_date, id) 继续，天然免疫插入位移。
		// offset 分页在「已加载第 1 页 → 后台新插入一笔」的场景下会整页错位，
		// 前端去重后 hasMore 恒真、按钮永远加载不到新数据（原 F-04）。
		cd := req.CursorDate.T()
		q = q.Where("(tx_date < ? OR (tx_date = ? AND id < ?))", cd, cd, req.CursorID)
	} else {
		q = q.Offset((req.Page - 1) * req.PageSize)
	}
	q.Order("tx_date DESC, id DESC").Limit(req.PageSize).Find(&list)

	fillTxViewFields(list)

	// 按日分组
	dayMap := make(map[string][]models.Transaction)
	var dayOrder []string
	for _, t := range list {
		day := t.TxDate.Format("2006-01-02")
		if _, ok := dayMap[day]; !ok {
			dayOrder = append(dayOrder, day)
		}
		dayMap[day] = append(dayMap[day], t)
	}
	type dayGroup struct {
		Date         string               `json:"date"`
		DayIncome    models.Money         `json:"day_income"`
		DayExpense   models.Money         `json:"day_expense"`
		DayBalance   models.Money         `json:"day_balance"`
		Transactions []models.Transaction `json:"transactions"`
	}
	// 用非 nil 空切片初始化：数据量 0 时 JSON 序列化为 [] 而非 null，
	// 否则前端 group.length 会因 null 抛错。
	grouped := make([]dayGroup, 0, len(dayOrder))
	for _, d := range dayOrder {
		g := dayGroup{Date: d, Transactions: dayMap[d]}
		for i := range dayMap[d] {
			t := dayMap[d][i]
			if !t.IncludeInBalance {
				continue
			}
			// 收支口径与资金方向同源：models.TxStatsBucket。
			// 此前这里漏掉 reimburse —— 它真实扣了余额却不进任何小计，
			// 造成「当日明细加总 ≠ 当日小计」（原 B-04）。
			switch models.TxStatsBucket(t.Type) {
			case models.StatsBucketIncome:
				g.DayIncome += t.AmountInBase()
			case models.StatsBucketExpense:
				g.DayExpense += t.AmountInBase()
			}
		}
		g.DayBalance = g.DayIncome - g.DayExpense
		grouped = append(grouped, g)
	}

	// 汇总 —— 只在首页计算。
	// 「加载更多」每翻一页都重复一遍全表聚合、结果完全相同，白白多一条 SQL（原 P-01）。
	var sumIn, sumOut models.Money
	summary := gin.H{"total_income": sumIn, "total_expense": sumOut, "net": sumIn - sumOut}
	if !req.UseCursor() && req.Page == 1 {
		type sumRow struct {
			Type string  `gorm:"column:type"`
			Amt  float64 `gorm:"column:amt"`
		}
		var sums []sumRow
		// 外币笔设在 SQL 层折算，避免先累加再乘导致精度失真
		err := buildBase(true).
			Select("type, SUM(CASE WHEN exchange_rate > 0 THEN amount * exchange_rate ELSE amount END) as amt").
			Group("type").Scan(&sums).Error
		if err == nil {
			for _, s := range sums {
				switch models.TxStatsBucket(models.TransactionType(s.Type)) {
				case models.StatsBucketIncome:
					sumIn += models.FromCents(s.Amt)
				case models.StatsBucketExpense:
					sumOut += models.FromCents(s.Amt)
				}
			}
		}
		summary = gin.H{"total_income": sumIn, "total_expense": sumOut, "net": sumIn - sumOut}
	}

	// flat_list 已移除：它与 grouped 承载完全相同的数据，整份交易被 JSON 序列化两次，
	// 响应体与序列化 CPU 都翻倍（原 P-02）。前端如需平铺列表请用 grouped.flatMap。
	PagedOK(c, gin.H{
		"grouped": grouped,
		"summary": summary,
	}, req.Page, req.PageSize, total)
}

// UpdateTransaction 更新交易（补丁语义）
func UpdateTransaction(c *gin.Context) {
	uid := middleware.GetUID(c)
	var reqUri dto.IDRequest
	if err := c.ShouldBindUri(&reqUri); err != nil {
		Bad(c, "参数错误")
		return
	}
	var req dto.UpdateTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Bad(c, "参数错误: "+err.Error())
		return
	}

	db := database.DB.Begin()

	// 读范围与列表一致（原 B-05）
	var old models.Transaction
	if err := applyBookScope(c, db.Model(&models.Transaction{}), uid).
		Where("id = ?", reqUri.ID).First(&old).Error; err != nil {
		db.Rollback()
		NotFound(c, "交易不存在")
		return
	}
	if !canWriteTx(c, uid, &old) {
		db.Rollback()
		Forbidden(c, "对该账本无写入权限")
		return
	}

	// 目标状态 = 旧值 + 补丁。未出现在请求体里的字段保持原样。
	target := old
	if req.BookID != nil && *req.BookID > 0 {
		target.BookID = *req.BookID
	}
	if req.Type != nil {
		target.Type = models.TransactionType(*req.Type)
	}
	if req.Amount != nil {
		target.Amount = models.FromYuan(*req.Amount)
	}
	if req.Currency != nil && *req.Currency != "" {
		target.Currency = *req.Currency
	}
	if req.ExchangeRate != nil {
		target.ExchangeRate = *req.ExchangeRate
	}
	if target.ExchangeRate <= 0 || math.IsNaN(target.ExchangeRate) || math.IsInf(target.ExchangeRate, 0) {
		target.ExchangeRate = 1
	}
	if req.CategoryID != nil {
		target.CategoryID = *req.CategoryID
	}
	if req.AccountID != nil {
		target.AccountID = *req.AccountID
	}
	if req.ToAccountID != nil {
		target.ToAccountID = *req.ToAccountID
	}
	if req.TransferFee != nil {
		target.TransferFee = models.FromYuan(*req.TransferFee)
	}
	if req.TransferDiscount != nil {
		target.TransferDiscount = models.FromYuan(*req.TransferDiscount)
	}
	if req.RefundOfID != nil {
		target.RefundOfID = *req.RefundOfID
	}
	if req.TxDate != nil {
		target.TxDate = req.TxDate.T()
	}
	if req.Description != nil {
		target.Description = *req.Description
	}
	if req.Images != nil {
		target.Images = *req.Images
	}
	if req.Merchant != nil {
		target.Merchant = *req.Merchant
	}
	if req.Location != nil {
		target.Location = *req.Location
	}
	if req.IncludeInBalance != nil {
		target.IncludeInBalance = *req.IncludeInBalance
	}
	if req.IncludeInBudget != nil {
		target.IncludeInBudget = *req.IncludeInBudget
	}
	if req.Remark != nil {
		target.Remark = *req.Remark
	}
	// 报销状态：空串是非法值，会让该笔流水在按状态筛选时永远查不到
	target.ReimburseStatus = req.NormalizedReimburseStatus(old.ReimburseStatus)

	// C14：类型白名单
	switch target.Type {
	case models.TxExpense, models.TxIncome, models.TxTransfer,
		models.TxRefund, models.TxReimburse, models.TxAdjust:
	default:
		db.Rollback()
		Bad(c, "非法交易类型: "+string(target.Type))
		return
	}
	// C14：归属校验 —— 账户/目标账户/分类必须属于当前用户（或共享账本）
	if !ownsOrSharesAccount(c, uid, target.AccountID) {
		db.Rollback()
		Bad(c, "来源账户不存在或无权限")
		return
	}
	if target.ToAccountID > 0 && !ownsOrSharesAccount(c, uid, target.ToAccountID) {
		db.Rollback()
		Bad(c, "目标账户不存在或无权限")
		return
	}
	if target.CategoryID > 0 {
		var cnt int64
		db.Model(&models.Category{}).
			Where("id = ? AND (user_id = ? OR book_id IN ?)",
				target.CategoryID, uid, append(bookIDsOf(c, uid), 0)).Count(&cnt)
		if cnt == 0 {
			db.Rollback()
			Bad(c, "分类不存在或无权限")
			return
		}
	}
	if target.Type == models.TxTransfer && target.ToAccountID == 0 {
		db.Rollback()
		Bad(c, "转账需要指定目标账户")
		return
	}
	if !canWriteBook(c, uid, target.BookID) && old.UserID != uid {
		db.Rollback()
		Forbidden(c, "对该账本无写入权限")
		return
	}

	// 撤销该交易派生出的子交易（手续费）
	revertDerivedTransactions(db, old.ID)

	// 先撤销原交易对余额的影响
	var from, to models.Account
	db.First(&from, old.AccountID)
	if old.ToAccountID > 0 {
		db.First(&to, old.ToAccountID)
	}
	updateAccountBalances(db, &old, &from, &to, false)
	// 撤销旧预算 used_amount
	applyBudgetUsed(db, uid, old.BookID, old.CategoryID, old.TxDate,
		old.AmountInBase(), old.Type, old.IncludeInBudget, -1)

	// 只写本次真正提交的字段（补丁语义），其余字段保持库里的原值。
	// 此前用一张 20 键的固定 map 无条件覆盖，把所有没传的字段一律写零值，
	// 静默销毁手续费 / 退款关联 / 凭证图片 / 报销状态（原 B-01）。
	updates := map[string]interface{}{}
	if req.BookID != nil {
		updates["book_id"] = target.BookID
	}
	if req.Type != nil {
		updates["type"] = target.Type
	}
	if req.Amount != nil {
		updates["amount"] = target.Amount
	}
	if req.Currency != nil {
		updates["currency"] = target.Currency
	}
	if req.ExchangeRate != nil || old.ExchangeRate != target.ExchangeRate {
		updates["exchange_rate"] = target.ExchangeRate
	}
	if req.CategoryID != nil {
		updates["category_id"] = target.CategoryID
	}
	if req.AccountID != nil {
		updates["account_id"] = target.AccountID
	}
	if req.ToAccountID != nil {
		updates["to_account_id"] = target.ToAccountID
	}
	if req.TransferFee != nil {
		updates["transfer_fee"] = target.TransferFee
	}
	if req.TransferDiscount != nil {
		updates["transfer_discount"] = target.TransferDiscount
	}
	if req.RefundOfID != nil {
		updates["refund_of_id"] = target.RefundOfID
	}
	if req.TxDate != nil {
		updates["tx_date"] = target.TxDate
	}
	if req.Description != nil {
		updates["description"] = target.Description
	}
	if req.Images != nil {
		updates["images"] = target.Images
	}
	if req.Merchant != nil {
		updates["merchant"] = target.Merchant
	}
	if req.Location != nil {
		updates["location"] = target.Location
	}
	if req.IncludeInBalance != nil {
		updates["include_in_balance"] = target.IncludeInBalance
	}
	if req.IncludeInBudget != nil {
		updates["include_in_budget"] = target.IncludeInBudget
	}
	if req.Remark != nil {
		updates["remark"] = target.Remark
	}
	if req.ReimburseStatus != nil || old.ReimburseStatus != target.ReimburseStatus {
		updates["reimburse_status"] = target.ReimburseStatus
	}
	if len(updates) > 0 {
		if err := db.Model(&models.Transaction{}).Where("id = ?", old.ID).Updates(updates).Error; err != nil {
			db.Rollback()
			InternalErr(c, "更新失败: "+err.Error())
			return
		}
	}

	// 标签按差集同步（此前整删再增，对未变更的标签也做一次「减再加」，计数漂移）
	if req.HasTagIDs() {
		syncTxTags(db, old.ID, req.TagIDsOrNil())
	}

	// 应用新余额 / 预算影响
	var newTx models.Transaction
	db.Preload("Tags").First(&newTx, old.ID)
	var from2, to2 models.Account
	db.First(&from2, newTx.AccountID)
	if newTx.ToAccountID > 0 {
		db.First(&to2, newTx.ToAccountID)
	}
	updateAccountBalances(db, &newTx, &from2, &to2, true)
	applyBudgetUsed(db, uid, newTx.BookID, newTx.CategoryID, newTx.TxDate,
		newTx.AmountInBase(), newTx.Type, newTx.IncludeInBudget, 1)

	// 转账手续费派生交易：旧的已在前面回滚，这里统一按最新状态重建
	syncTransferFeeDerived(db, &newTx, &from2)

	if err := db.Commit().Error; err != nil {
		InternalErr(c, "提交失败: "+err.Error())
		return
	}

	database.DB.Preload("Tags").First(&newTx, old.ID)
	updated := []models.Transaction{newTx}
	fillTxViewFields(updated)
	Broadcast(c, "transactions", "update", old.ID)
	OK(c, updated[0])
}

// DeleteTransaction 删除交易（软删除+回滚余额）
func DeleteTransaction(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.IDRequest
	if err := c.ShouldBindUri(&req); err != nil {
		Bad(c, "参数错误")
		return
	}

	db := database.DB.Begin()
	var tx models.Transaction
	if err := applyBookScope(c, db.Model(&models.Transaction{}), uid).
		Where("id = ?", req.ID).First(&tx).Error; err != nil {
		db.Rollback()
		NotFound(c, "交易不存在")
		return
	}
	if !canWriteTx(c, uid, &tx) {
		db.Rollback()
		Forbidden(c, "对该账本无写入权限")
		return
	}

	// 撤销派生交易（手续费）
	revertDerivedTransactions(db, tx.ID)

	var from, to models.Account
	db.First(&from, tx.AccountID)
	if tx.ToAccountID > 0 {
		db.First(&to, tx.ToAccountID)
	}
	updateAccountBalances(db, &tx, &from, &to, false)
	applyBudgetUsed(db, uid, tx.BookID, tx.CategoryID, tx.TxDate,
		tx.AmountInBase(), tx.Type, tx.IncludeInBudget, -1)
	adjustTagCounts(db, txTagIDs(db, tx.ID), -1)

	// 注意：**不再删除 transaction_tags 关联行**。
	// 此前这里硬删了关联，而 RecoverTransaction 又依赖 txTagIDs 读回标签 → 必然返回空切片，
	// 回收站恢复后标签绑定永久丢失（原 B-06）。保留关联不影响任何查询，
	// 因为主记录本身已被软删除，只在回收站里带上 Tags 展示。
	db.Delete(&tx)
	if err := db.Commit().Error; err != nil {
		InternalErr(c, "删除失败: "+err.Error())
		return
	}
	Broadcast(c, "transactions", "delete", req.ID)
	OK(c, nil)
}

// ========== 批量操作 ==========

type BatchDeleteRequest struct {
	IDs []uint `json:"ids"`
}

// maxBatchDelete 单次批量删除上限。
// 无上限时 `id IN (?)` 的参数个数不受控，大量 ID 会触发 SQLite 默认 999 参数上限，
// 且每条都跑一轮完整回滚，全部在同一事务里持有锁（原 B-08）。
const maxBatchDelete = 200

func BatchDeleteTransactions(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req BatchDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Bad(c, err.Error())
		return
	}
	if len(req.IDs) == 0 {
		Bad(c, "请选择要删除的交易")
		return
	}
	// 去重 + 限流
	seen := make(map[uint]struct{}, len(req.IDs))
	ids := make([]uint, 0, len(req.IDs))
	for _, id := range req.IDs {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) > maxBatchDelete {
		Bad(c, fmt.Sprintf("单次最多删除 %d 笔，请分批操作", maxBatchDelete))
		return
	}

	db := database.DB.Begin()

	var txs []models.Transaction
	applyBookScope(c, db.Model(&models.Transaction{}), uid).Where("id IN ?", ids).Find(&txs)
	// 写权限过滤（原 B-05）：共享账本里他人所记、且自己只有 viewer 角色的流水跳过
	writable := make([]models.Transaction, 0, len(txs))
	writableIDs := make([]uint, 0, len(txs))
	for i := range txs {
		if !canWriteTx(c, uid, &txs[i]) {
			continue
		}
		writable = append(writable, txs[i])
		writableIDs = append(writableIDs, txs[i].ID)
	}
	if len(writable) == 0 {
		db.Rollback()
		Forbidden(c, "没有可删除的交易")
		return
	}

	// ---- 一次性预取：标签关联 / 账户（含方向系数）/ 派生交易 ----
	tagMap := txTagIDsBatch(db, writableIDs)

	accSet := make(map[uint]struct{}, len(writable)*2)
	for i := range writable {
		accSet[writable[i].AccountID] = struct{}{}
		if writable[i].ToAccountID > 0 {
			accSet[writable[i].ToAccountID] = struct{}{}
		}
	}
	accMap := make(map[uint]models.Account, len(accSet))
	if len(accSet) > 0 {
		var accs []models.Account
		db.Where("id IN ?", idSetKeys(accSet)).Find(&accs)
		for i := range accs {
			accMap[accs[i].ID] = accs[i]
		}
	}

	// 派生交易回滚（每条单独处理，但派生交易只有转账手续费，数量极少）
	for i := range writable {
		revertDerivedTransactions(db, writable[i].ID)
	}

	// ---- 聚合增量，最后统一落库 ----
	type budgetKey struct {
		bookID uint
		catID  uint
		date   string
	}
	balanceDeltas := make(map[uint]int64, len(accSet))
	budgetDeltas := make(map[budgetKey]models.Money, len(writable))
	tagCount := make(map[uint]int, len(writable))
	for i := range writable {
		t := writable[i]
		for _, tid := range tagMap[t.ID] {
			tagCount[tid]++
		}
		from := accMap[t.AccountID]
		var to *models.Account
		if t.ToAccountID > 0 {
			if a, ok := accMap[t.ToAccountID]; ok {
				to = &a
			}
		}
		accumulateBalanceDelta(&t, &from, to, balanceDeltas, -1)
		// 预算只汇总真正计入的部分，避免把转账/退款也算进去
		if models.TxStatsBucket(t.Type) == models.StatsBucketExpense && t.IncludeInBudget {
			k := budgetKey{bookID: t.BookID, catID: t.CategoryID, date: t.TxDate.Format("2006-01-02")}
			budgetDeltas[k] += t.AmountInBase()
		}
	}
	flushBalanceDeltas(db, balanceDeltas)
	if len(budgetDeltas) > 0 {
		keys := make([]budgetKey, 0, len(budgetDeltas))
		for k := range budgetDeltas {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return keys[i].date < keys[j].date })
		for _, k := range keys {
			d, err := time.Parse("2006-01-02", k.date)
			if err != nil {
				continue
			}
			applyBudgetUsed(db, uid, k.bookID, k.catID, d, budgetDeltas[k], models.TxExpense, true, -1)
		}
	}
	adjustTagCountsBatch(db, tagCount, -1)

	db.Where("id IN ?", writableIDs).Delete(&models.Transaction{})
	if err := db.Commit().Error; err != nil {
		InternalErr(c, "批量删除失败: "+err.Error())
		return
	}
	Broadcast(c, "transactions", "delete", 0)
	OK(c, gin.H{"deleted_count": len(writable)})
}

// RecoverTransaction 撤销软删除（回收站恢复）。
// 后端早已是软删除（BaseModel.DeletedAt），这里提供按 ID 恢复，并回滚余额与预算。
func RecoverTransaction(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.IDRequest
	if err := c.ShouldBindUri(&req); err != nil {
		Bad(c, "参数错误")
		return
	}
	db := database.DB.Begin()
	var tx models.Transaction
	if err := applyBookScope(c, db.Unscoped().Model(&models.Transaction{}), uid).
		Where("id = ? AND deleted_at IS NOT NULL", req.ID).First(&tx).Error; err != nil {
		db.Rollback()
		NotFound(c, "未找到已删除的交易")
		return
	}
	if !canWriteTx(c, uid, &tx) {
		db.Rollback()
		Forbidden(c, "对该账本无写入权限")
		return
	}
	if err := db.Unscoped().Model(&models.Transaction{}).Where("id = ?", req.ID).
		Update("deleted_at", nil).Error; err != nil {
		db.Rollback()
		InternalErr(c, "恢复失败: "+err.Error())
		return
	}
	var from, to models.Account
	db.First(&from, tx.AccountID)
	if tx.ToAccountID > 0 {
		db.First(&to, tx.ToAccountID)
	}
	updateAccountBalances(db, &tx, &from, &to, true)
	applyBudgetUsed(db, uid, tx.BookID, tx.CategoryID, tx.TxDate,
		tx.AmountInBase(), tx.Type, tx.IncludeInBudget, 1)
	// 标签关联在删除时被刻意保留，这里一定能读回来（原 B-06）
	adjustTagCounts(db, txTagIDs(db, tx.ID), 1)
	// 重建派生交易（转账手续费）：删除时被硬删以保证回收站不留系统垃圾（原 B-07）
	syncTransferFeeDerived(db, &tx, &from)
	if err := db.Commit().Error; err != nil {
		InternalErr(c, "恢复失败: "+err.Error())
		return
	}
	Broadcast(c, "transactions", "recover", req.ID)
	OK(c, gin.H{"recovered": req.ID})
}

// ListDeletedTransactions 回收站：列出软删除的交易
func ListDeletedTransactions(c *gin.Context) {
	uid := middleware.GetUID(c)
	page, pageSize := GetPageParams(c)
	if pageSize > 200 {
		pageSize = 200
	}
	var list []models.Transaction
	applyBookScope(c, database.DB.Unscoped().Model(&models.Transaction{}), uid).
		Preload("Tags").
		Where("deleted_at IS NOT NULL").
		Order("deleted_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&list)
	fillTxViewFields(list)
	OK(c, list)
}

// ========== 预算 Budget ==========

func ListBudgets(c *gin.Context) {
	uid := middleware.GetUID(c)
	bookID := c.Query("book_id")
	// C5：共享账本的预算对受邀成员可见
	q := applyBookScope(c, database.DB.Model(&models.Budget{}), uid)
	// book_id=0 表示「全部账本」：与流水/分类/账户列表保持一致，不加账本过滤。
	// 否则传 0 会变成 `book_id = 0` 的字面过滤，预算列表恒为空。
	if bookID != "" && bookID != "0" {
		q = q.Where("book_id = ?", bookID)
	}
	var list []models.Budget
	q.Order("start_date DESC, id DESC").Find(&list)
	// 计算剩余
	type budgetView struct {
		models.Budget
		Remaining    models.Money `json:"remaining"`
		UsageRate    float64      `json:"usage_rate"`
		IsOverBudget bool         `json:"is_over_budget"`
		DailyBudget  float64      `json:"daily_budget"`
	}
	out := make([]budgetView, 0, len(list))
	for _, b := range list {
		v := budgetView{Budget: b, Remaining: b.Amount - b.UsedAmount, UsageRate: 0}
		if b.Amount > 0 {
			v.UsageRate = math.Round(b.UsedAmount.Yuan()/b.Amount.Yuan()*1000) / 1000
		}
		v.IsOverBudget = b.UsedAmount > b.Amount
		days := b.EndDate.Sub(b.StartDate).Hours()/24 + 1
		if days > 0 {
			v.DailyBudget = math.Round((b.Amount - b.UsedAmount).Yuan()/days*100) / 100
		}
		out = append(out, v)
	}
	OK(c, out)
}

func CreateBudget(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.CreateBudgetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Bad(c, "参数错误: "+err.Error())
		return
	}
	// 起止日期：客户端未传或区间非法时，按 period_type + 用户账期起始日自动推导（C2）
	monthStart := userMonthStart(uid)
	start := req.StartDate.T()
	end := req.EndDate.T()
	if start.IsZero() {
		s, e := budgetPeriodRange(req.PeriodType, time.Now(), monthStart)
		start, end = s, e
	} else if end.IsZero() || !end.After(start) {
		_, e := budgetPeriodRange(req.PeriodType, start, monthStart)
		start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
		end = e
	}

	b := models.Budget{
		UserID:     uid,
		BookID:     req.BookID,
		PeriodType: req.PeriodType,
		CategoryID: req.CategoryID,
		Amount:     models.FromYuan(req.Amount),
		StartDate:  start,
		EndDate:    end,
		AlertRate:  req.AlertRate,
		RollOver:   req.RollOver,
	}
	if b.AlertRate <= 0 {
		b.AlertRate = 0.8
	}
	if err := database.DB.Create(&b).Error; err != nil {
		InternalErr(c, "创建预算失败: "+err.Error())
		return
	}
	// 回溯回填：按区间内已有支出重算 used_amount，避免月中建预算进度恒为 0（B2）
	recalcBudgetUsed(database.DB, &b)
	Created(c, b)
}

func UpdateBudget(c *gin.Context) {
	uid := middleware.GetUID(c)
	var reqUri dto.IDRequest
	c.ShouldBindUri(&reqUri)
	var req dto.UpdateBudgetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Bad(c, "参数错误: "+err.Error())
		return
	}
	var old models.Budget
	if err := database.DB.Where("id = ? AND user_id = ?", reqUri.ID, uid).First(&old).Error; err != nil {
		NotFound(c, "预算不存在")
		return
	}

	updates := map[string]interface{}{}
	if req.PeriodType != "" {
		updates["period_type"] = req.PeriodType
	}
	if req.CategoryID != nil {
		updates["category_id"] = *req.CategoryID
	}
	if req.Amount != nil {
		updates["amount"] = models.FromYuan(*req.Amount)
	}
	if req.AlertRate != nil {
		updates["alert_rate"] = *req.AlertRate
	}
	if req.RollOver != nil {
		updates["roll_over"] = *req.RollOver
	}
	if !req.StartDate.IsZero() {
		updates["start_date"] = req.StartDate.T()
	}
	if !req.EndDate.IsZero() {
		updates["end_date"] = req.EndDate.T()
	}
	// 改了 period_type 却没给日期 → 重新推导区间
	if _, ok := updates["period_type"]; ok {
		if _, okS := updates["start_date"]; !okS {
			if _, okE := updates["end_date"]; !okE {
				s, e := budgetPeriodRange(req.PeriodType, old.StartDate, userMonthStart(uid))
				updates["start_date"] = s
				updates["end_date"] = e
			}
		}
	}
	if len(updates) > 0 {
		if err := database.DB.Model(&models.Budget{}).
			Where("id = ? AND user_id = ?", reqUri.ID, uid).Updates(updates).Error; err != nil {
			InternalErr(c, "更新失败: "+err.Error())
			return
		}
	}

	var nb models.Budget
	database.DB.First(&nb, reqUri.ID)
	// 区间/分类/金额任一变化都会影响已用金额口径，统一重算（B2）
	recalcBudgetUsed(database.DB, &nb)
	OK(c, nb)
}

// userMonthStart 读取用户的自定义账期起始日（1-28），未设置时为 1
func userMonthStart(uid uint) int {
	var u models.User
	if err := database.DB.Select("month_start").Where("id = ?", uid).First(&u).Error; err != nil {
		return 1
	}
	if u.MonthStart < 1 || u.MonthStart > 28 {
		return 1
	}
	return u.MonthStart
}

func DeleteBudget(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.IDRequest
	c.ShouldBindUri(&req)
	database.DB.Where("id = ? AND user_id = ?", req.ID, uid).Delete(&models.Budget{})
	OK(c, nil)
}
