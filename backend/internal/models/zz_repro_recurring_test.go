package models

import (
	"testing"
	"time"
)

// 复核回归测试：首次执行时间（ComputeFirstRun）不得早于 start_date。
func TestRepro_FirstRun(t *testing.T) {
	loc := time.Local
	sd := time.Date(2026, 9, 19, 0, 0, 0, 0, loc) // 周六

	cases := []struct {
		name    string
		r       Recurring
		wantStr string
	}{
		{
			name:    "monthly 每月25号，起始 09-19 → 首次应为 09-25",
			r:       Recurring{RecurringType: RecMonthly, MonthDay: 25, StartDate: sd},
			wantStr: "2026-09-25",
		},
		{
			name:    "monthly 每月5号，起始 09-19 → 首次应为 10-05",
			r:       Recurring{RecurringType: RecMonthly, MonthDay: 5, StartDate: sd},
			wantStr: "2026-10-05",
		},
		{
			name:    "monthly 未指定几号，起始 09-19 → 首次即 09-19",
			r:       Recurring{RecurringType: RecMonthly, StartDate: sd},
			wantStr: "2026-09-19",
		},
		{
			name:    "daily 间隔3天，起始 09-19 → 首次应为 09-19",
			r:       Recurring{RecurringType: RecDaily, Interval: 3, StartDate: sd},
			wantStr: "2026-09-19",
		},
		{
			name:    "yearly 起始 2026-09-19 → 首次应为 2026-09-19（不是 2027）",
			r:       Recurring{RecurringType: RecYearly, StartDate: sd},
			wantStr: "2026-09-19",
		},
		{
			name:    "biweekly 起始 09-19 → 首次应为 09-19",
			r:       Recurring{RecurringType: RecBiWeek, Interval: 2, StartDate: sd},
			wantStr: "2026-09-19",
		},
		{
			name:    "weekly 每周六(6)，起始 09-19(周六) → 首次应为 09-19",
			r:       Recurring{RecurringType: RecWeekly, Weekday: 6, StartDate: sd},
			wantStr: "2026-09-19",
		},
		{
			name:    "weekly 每周一(1)，起始 09-19(周六) → 首次应为 09-21",
			r:       Recurring{RecurringType: RecWeekly, Weekday: 1, StartDate: sd},
			wantStr: "2026-09-21",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.r.ComputeFirstRun()
			if got.Format("2006-01-02") != c.wantStr {
				t.Errorf("首次执行时间错误: got=%s want=%s",
					got.Format("2006-01-02"), c.wantStr)
			} else {
				t.Logf("OK 首次执行=%s", got.Format("2006-01-02"))
			}
		})
	}
}

// 后续周期计算回归保护（含月末收敛）
func TestRepro_SubsequentRun(t *testing.T) {
	loc := time.Local
	from := time.Date(2026, 9, 25, 9, 0, 0, 0, loc)

	r := Recurring{RecurringType: RecMonthly, MonthDay: 25}
	if got := r.ComputeNextRun(from); got.Format("2006-01-02") != "2026-10-25" {
		t.Errorf("monthly 后续周期错误: got=%s want=2026-10-25", got.Format("2006-01-02"))
	}

	r2 := Recurring{RecurringType: RecDaily, Interval: 3}
	if got := r2.ComputeNextRun(from); got.Format("2006-01-02") != "2026-09-28" {
		t.Errorf("daily interval=3 后续周期错误: got=%s want=2026-09-28", got.Format("2006-01-02"))
	}

	// 月末收敛：1/31 起，每月 31 号 → 应落在 2/28，而不是跳到 3 月
	r3 := Recurring{RecurringType: RecMonthly, MonthDay: 31}
	if got := r3.ComputeNextRun(time.Date(2026, 1, 31, 9, 0, 0, 0, loc)); got.Format("2006-01-02") != "2026-02-28" {
		t.Errorf("月末收敛错误: got=%s want=2026-02-28", got.Format("2006-01-02"))
	}
	// 闰年 2 月：2028-01-31 → 2028-02-29
	if got := r3.ComputeNextRun(time.Date(2028, 1, 31, 9, 0, 0, 0, loc)); got.Format("2006-01-02") != "2028-02-29" {
		t.Errorf("闰年月末收敛错误: got=%s want=2028-02-29", got.Format("2006-01-02"))
	}
	// 12 月跨年：2026-12-15 → 2027-01-15
	r4 := Recurring{RecurringType: RecMonthly, MonthDay: 15}
	if got := r4.ComputeNextRun(time.Date(2026, 12, 15, 9, 0, 0, 0, loc)); got.Format("2006-01-02") != "2027-01-15" {
		t.Errorf("跨年错误: got=%s want=2027-01-15", got.Format("2006-01-02"))
	}
	// 年度：2028-02-29 → 2029-02-28（不能归一化成 3 月 1 日）
	r5 := Recurring{RecurringType: RecYearly}
	if got := r5.ComputeNextRun(time.Date(2028, 2, 29, 9, 0, 0, 0, loc)); got.Format("2006-01-02") != "2029-02-28" {
		t.Errorf("年度闰日收敛错误: got=%s want=2029-02-28", got.Format("2006-01-02"))
	}
}
