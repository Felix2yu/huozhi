package handlers

import (
	"testing"
	"time"
)

// 复核用复现测试：预算周期滚动的区间推导。
func TestRepro_BudgetPeriodDrift(t *testing.T) {
	loc := time.Local

	// 月度预算：2026-09-01 ~ 2026-10-01
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, loc)
	end := time.Date(2026, 10, 1, 0, 0, 0, 0, loc)

	// 连续滚动 4 期，检查是否始终落在账期首日
	curS, curE := start, end
	for i := 1; i <= 4; i++ {
		ns, ne := nextBudgetPeriod("monthly", curS, curE, 1)
		t.Logf("第%d次滚动: %s ~ %s", i,
			ns.Format("2006-01-02"), ne.Format("2006-01-02"))
		if ns.Day() != 1 || ne.Day() != 1 {
			t.Errorf("月度预算边界漂移: 第%d期 %s ~ %s，应为每月 1 号",
				i, ns.Format("2006-01-02"), ne.Format("2006-01-02"))
		}
		// 区间必须首尾相接，不能重叠或断档
		if !ns.Equal(curE) {
			t.Errorf("第%d期起点 %s 未接上上期终点 %s", i,
				ns.Format("2006-01-02"), curE.Format("2006-01-02"))
		}
		curS, curE = ns, ne
	}

	// 年度预算：2026-01-01 ~ 2027-01-01 → 2027-01-01 ~ 2028-01-01
	ys, ye := nextBudgetPeriod("yearly",
		time.Date(2026, 1, 1, 0, 0, 0, 0, loc),
		time.Date(2027, 1, 1, 0, 0, 0, 0, loc), 1)
	if ys.Format("2006-01-02") != "2027-01-01" || ye.Format("2006-01-02") != "2028-01-01" {
		t.Errorf("年度滚动错误: got %s ~ %s, want 2027-01-01 ~ 2028-01-01",
			ys.Format("2006-01-02"), ye.Format("2006-01-02"))
	}

	// 自定义账期起始日 = 5：10-05 ~ 11-05 → 11-05 ~ 12-05
	cs, ce := nextBudgetPeriod("monthly",
		time.Date(2026, 10, 5, 0, 0, 0, 0, loc),
		time.Date(2026, 11, 5, 0, 0, 0, 0, loc), 5)
	if cs.Format("2006-01-02") != "2026-11-05" || ce.Format("2006-01-02") != "2026-12-05" {
		t.Errorf("自定义账期滚动错误: got %s ~ %s, want 2026-11-05 ~ 2026-12-05",
			cs.Format("2006-01-02"), ce.Format("2006-01-02"))
	}
}

// budgetPeriodRange 在自定义账期起始日下的归属判断
func TestRepro_BudgetPeriodRange(t *testing.T) {
	loc := time.Local
	// 账期起始日 = 5，今天是 9/3 → 归属上一个账期 8/5 ~ 9/5
	s, e := budgetPeriodRange("monthly", time.Date(2026, 9, 3, 0, 0, 0, 0, loc), 5)
	if s.Format("2006-01-02") != "2026-08-05" || e.Format("2006-01-02") != "2026-09-05" {
		t.Errorf("账期归属错误: got %s ~ %s want 2026-08-05 ~ 2026-09-05",
			s.Format("2006-01-02"), e.Format("2006-01-02"))
	}
}
