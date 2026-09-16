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
	tx, cerr := CreateTx(uid, req)
	if cerr != nil {
		FailFromCore(c, cerr)
		return
	}
	Created(c, tx)
}

// broadcastBudgetAlerts 预算超限提醒。
//
// 此前该逻辑查询账本下**所有** used_amount > amount 的预算并推送，既不检查
// 是哪一类预算被突破，也不使用 budget 自身的 alert_rate：用户每记一笔账都会
// 收到一条与自己无关的「你有 N 个预算超支」。改为：
//  1. 只看本次支出的分类预算 + 该账本总预算；
//  2. 按 alert_rate 判断；
//  3. 只在「使用率由 below → above 跨过阈值」那一刻提醒，后续同类消费不再打扰。
func broadcastBudgetAlerts(uid uint, t models.Transaction) {
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
	tx, cerr := GetTx(uid, req.ID)
	if cerr != nil {
		FailFromCore(c, cerr)
		return
	}
	OK(c, tx)
}
// ListTransactions 交易列表（分页+按日分组）
func ListTransactions(c *gin.Context) {
	uid := middleware.GetUID(c)
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
	PagedOK(c, gin.H{
		"grouped": res.Grouped,
		"summary": res.Summary,
	}, res.Page, res.PageSize, res.Total)
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
	tx, cerr := UpdateTx(uid, reqUri.ID, req)
	if cerr != nil {
		FailFromCore(c, cerr)
		return
	}
	OK(c, tx)
}
// DeleteTransaction 删除交易（软删除+回滚余额）
func DeleteTransaction(c *gin.Context) {
	uid := middleware.GetUID(c)
	var req dto.IDRequest
	if err := c.ShouldBindUri(&req); err != nil {
		Bad(c, "参数错误")
		return
	}
	if cerr := DeleteTx(uid, req.ID); cerr != nil {
		FailFromCore(c, cerr)
		return
	}
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
	n, cerr := BatchDeleteTx(uid, req.IDs)
	if cerr != nil {
		FailFromCore(c, cerr)
		return
	}
	OK(c, gin.H{"deleted_count": n})
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
	if _, cerr := RecoverTx(uid, req.ID); cerr != nil {
		FailFromCore(c, cerr)
		return
	}
	OK(c, gin.H{"recovered": req.ID})
}
// ListDeletedTransactions 回收站：列出软删除的交易
func ListDeletedTransactions(c *gin.Context) {
	uid := middleware.GetUID(c)
	page, pageSize := GetPageParams(c)
	list, cerr := ListDeletedTx(uid, page, pageSize)
	if cerr != nil {
		FailFromCore(c, cerr)
		return
	}
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
