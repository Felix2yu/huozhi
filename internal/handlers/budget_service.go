package handlers

import (
	"fmt"
	"huozhi/internal/database"
	"huozhi/internal/dto"
	"huozhi/internal/middleware"
	"huozhi/internal/models"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ========== 预算周期推导 / 回溯重算 / 自动滚动 ==========
//
// 背景（审查项 C2 / B1 / B2）：
//  1. CreateBudget 曾把 start_date / end_date 设为 binding:"required"，而前端只提交
//     周期类型 → 预算新增 100% 失败；
//  2. used_amount 是纯增量计数器，创建时不回填历史 → 月中建预算进度恒为 0；
//  3. period_type 存了却从不生成下一期 → 用户每月必须手工重建预算。
//
// 本文件提供这三个问题的统一实现。

// budgetPeriodRange 按周期类型推导 [start, end) 区间。
// anchor 为区间内的任意时刻（缺省用「今天」）；monthly/yearly 会被规整到周期首日 00:00。
// 用户的 month_start（自定义账期起始日，1-28）在 monthly 时生效。
func budgetPeriodRange(periodType string, anchor time.Time, monthStart int) (time.Time, time.Time) {
	if anchor.IsZero() {
		anchor = time.Now()
	}
	loc := anchor.Location()
	switch periodType {
	case "yearly":
		start := time.Date(anchor.Year(), 1, 1, 0, 0, 0, 0, loc)
		return start, start.AddDate(1, 0, 0)
	case "custom":
		start := time.Date(anchor.Year(), anchor.Month(), anchor.Day(), 0, 0, 0, 0, loc)
		return start, start.AddDate(0, 1, 0)
	default: // monthly
		day := monthStart
		if day < 1 || day > 28 {
			day = 1
		}
		start := time.Date(anchor.Year(), anchor.Month(), day, 0, 0, 0, 0, loc)
		if anchor.Day() < day {
			// anchor 落在本月账期起始日之前 → 归属上一个账期
			start = start.AddDate(0, -1, 0)
		}
		return start, start.AddDate(0, 1, 0)
	}
}

// nextBudgetPeriod 返回紧接当前区间之后的同一长度区间（用于周期滚动）。
//
// monthStart 为用户的自定义账期起始日（1-28）。monthly / yearly 必须按日历边界重新推导，
// 不能用「上期跨度天数」平移：9 月是 30 天，平移会让 10 月预算变成 10-01~10-31、
// 11 月变成 10-31~11-30，逐月偏离账期首日（预算周期漂移）。
//
// 新区间的起点恒为上期终点（end 是开区间右端点），保证既不重叠也不断档；
// 终点再按日历边界推导，从而把区间长度拉回正确的月/年长度。
func nextBudgetPeriod(periodType string, start, end time.Time, monthStart int) (time.Time, time.Time) {
	switch periodType {
	case "yearly", "monthly":
		_, ne := budgetPeriodRange(periodType, end, monthStart)
		return end, ne
	default: // custom：按实际跨度推进，保持用户自定义的区间长度
		return end, end.AddDate(0, 0, int(end.Sub(start).Hours()/24))
	}
}

// recalcBudgetUsed 用真实流水回填某条预算的 used_amount（B2）。
// 口径与 applyBudgetUsed 保持一致：仅支出、且 include_in_budget = true。
// 口径与 applyBudgetUsed 严格一致：
//  1. 只算统计口径为「支出」的类型 —— 现在包含 reimburse（此前遗漏，导致
//     报销流水真实扣了账户却不算预算占用，原 B-04）；
//  2. 金额按汇率折算到基准币种（baseAmountExpr），此前直接用 SUM(amount)，
//     外币支出的预算占用按原币金额计算（原 B-02）。
func recalcBudgetUsed(db *gorm.DB, b *models.Budget) models.Money {
	var used float64
	q := db.Model(&models.Transaction{}).
		Where("book_id = ? AND include_in_budget = ? AND tx_date >= ? AND tx_date < ? AND type IN ?",
			b.BookID, true, b.StartDate, b.EndDate, models.TypesInBucket(models.StatsBucketExpense))
	if b.CategoryID > 0 {
		q = q.Where("category_id = ?", b.CategoryID)
	}
	q.Select(fmt.Sprintf("COALESCE(SUM(%s), 0)", baseAmountExpr)).Row().Scan(&used)
	newUsed := models.FromCents(used)
	db.Model(&models.Budget{}).Where("id = ?", b.ID).Update("used_amount", newUsed)
	b.UsedAmount = newUsed
	return newUsed
}

// RecalcBudgetHandler POST /budgets/:id/recalc —— 手动重算单条预算（对账入口）
func RecalcBudgetHandler(c *gin.Context) {
	uid := middleware.GetUID(c)
	var reqUri dto.IDRequest
	if err := c.ShouldBindUri(&reqUri); err != nil {
		Bad(c, "参数错误")
		return
	}
	id := reqUri.ID
	var b models.Budget
	if err := database.DB.Where("id = ? AND user_id = ?", id, uid).First(&b).Error; err != nil {
		NotFound(c, "预算不存在")
		return
	}
	used := recalcBudgetUsed(database.DB, &b)
	OK(c, gin.H{"id": b.ID, "used_amount": used, "amount": b.Amount,
		"remaining": b.Amount - used})
}

// RecalcAllBudgetsHandler POST /budgets/recalc —— 重算当前用户全部预算
func RecalcAllBudgetsHandler(c *gin.Context) {
	uid := middleware.GetUID(c)
	var list []models.Budget
	database.DB.Where("user_id = ?", uid).Find(&list)
	for i := range list {
		recalcBudgetUsed(database.DB, &list[i])
	}
	OK(c, gin.H{"recalculated": len(list)})
}

// RollBudgetsForward 预算周期自动滚动（B1）。
// 对所有已过期（end_date <= now）的周期性预算，按 period_type 生成下一期：
//   - 同分类 / 同账本已有下一期预算则跳过（幂等）；
//   - roll_over = true 时把「结余」叠加到下一期额度上（负数结余不滚）。
//
// 返回本次新生成的期数。由 main.go 的调度器每日调用。
func RollBudgetsForward(now time.Time) int {
	if database.DB == nil {
		return 0
	}
	var list []models.Budget
	database.DB.Where("period_type IN ? AND end_date <= ?",
		[]string{"monthly", "yearly"}, now).Find(&list)

	created := 0
	for _, b := range list {
		ns, ne := nextBudgetPeriod(b.PeriodType, b.StartDate, b.EndDate, userMonthStart(b.UserID))
		// 幂等：该账本 + 该分类 + 完全相同区间已存在则跳过
		var exist int64
		database.DB.Model(&models.Budget{}).
			Where("book_id = ? AND category_id = ? AND start_date = ? AND end_date = ?",
				b.BookID, b.CategoryID, ns, ne).Count(&exist)
		if exist > 0 {
			continue
		}
		amount := b.Amount
		if b.RollOver {
			if remain := b.Amount - b.UsedAmount; remain > 0 {
				amount = b.Amount + remain
			}
		}
		nb := models.Budget{
			UserID:     b.UserID,
			BookID:     b.BookID,
			PeriodType: b.PeriodType,
			CategoryID: b.CategoryID,
			Amount:     amount,
			StartDate:  ns,
			EndDate:    ne,
			AlertRate:  b.AlertRate,
			RollOver:   b.RollOver,
		}
		if err := database.DB.Create(&nb).Error; err != nil {
			continue
		}
		// 新期次若已跨入当前时间（服务停机后补滚），区间内可能已有支出，
		// used_amount 必须按流水回填，否则新预算进度恒为 0。
		recalcBudgetUsed(database.DB, &nb)
		created++
	}
	return created
}

// BudgetRolloverRunner 每日滚动一次（挂在 main.go 的 goroutine 上）
func BudgetRolloverRunner() {
	tick := time.NewTicker(1 * time.Hour)
	defer tick.Stop()
	for range tick.C {
		if n := RollBudgetsForward(time.Now()); n > 0 {
			log.Printf("[Cron] 预算周期滚动完成，生成 %d 条新预算", n)
		}
	}
}
