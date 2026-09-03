package models

import (
	"testing"
	"time"
)

func TestComputeNextRunNil(t *testing.T) {
	var r *Recurring
	from := time.Date(2026, 3, 15, 9, 0, 0, 0, time.UTC)
	if r.ComputeNextRun(from) != from {
		t.Fatal("nil receiver should return from")
	}
}

func TestComputeNextRunDaily(t *testing.T) {
	r := &Recurring{RecurringType: RecDaily}
	from := time.Date(2026, 3, 15, 9, 0, 0, 0, time.UTC)
	if next := r.ComputeNextRun(from); !next.Equal(from.AddDate(0, 0, 1)) {
		t.Fatalf("daily mismatch: %v", next)
	}
}

func TestComputeNextRunWeeklyDefault(t *testing.T) {
	r := &Recurring{RecurringType: RecWeekly} // Weekday 0 -> treated as Sunday -> next Sunday (+7)
	from := time.Date(2026, 3, 15, 9, 0, 0, 0, time.UTC) // Sunday
	if next := r.ComputeNextRun(from); !next.Equal(from.AddDate(0, 0, 7)) {
		t.Fatalf("weekly default mismatch: %v", next)
	}
}

func TestComputeNextRunWeeklyTarget(t *testing.T) {
	from := time.Date(2026, 3, 16, 9, 0, 0, 0, time.UTC) // Monday
	r := &Recurring{RecurringType: RecWeekly, Weekday: 3} // Wednesday -> +2 days
	next := r.ComputeNextRun(from)
	if !next.Equal(time.Date(2026, 3, 18, 9, 0, 0, 0, time.UTC)) {
		t.Fatalf("weekly target mismatch: %v", next)
	}
}

func TestComputeNextRunBiWeek(t *testing.T) {
	r := &Recurring{RecurringType: RecBiWeek} // default interval 2
	from := time.Date(2026, 3, 15, 9, 0, 0, 0, time.UTC)
	if next := r.ComputeNextRun(from); !next.Equal(from.AddDate(0, 0, 14)) {
		t.Fatalf("biweek mismatch: %v", next)
	}
}

func TestComputeNextRunMonthly(t *testing.T) {
	r := &Recurring{RecurringType: RecMonthly, MonthDay: 10}
	from := time.Date(2026, 3, 15, 9, 0, 0, 0, time.UTC)
	exp := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	if next := r.ComputeNextRun(from); !next.Equal(exp) {
		t.Fatalf("monthly mismatch: %v", next)
	}
}

func TestComputeNextRunMonthlyClamp(t *testing.T) {
	r := &Recurring{RecurringType: RecMonthly, MonthDay: 31}
	from := time.Date(2026, 1, 31, 9, 0, 0, 0, time.UTC)
	exp := time.Date(2026, 3, 31, 9, 0, 0, 0, time.UTC)
	if next := r.ComputeNextRun(from); !next.Equal(exp) {
		t.Fatalf("monthly clamp mismatch: %v", next)
	}
}

func TestComputeNextRunMonthlyDefaultDay(t *testing.T) {
	r := &Recurring{RecurringType: RecMonthly} // MonthDay 0 -> from.Day()
	from := time.Date(2026, 1, 31, 9, 0, 0, 0, time.UTC)
	exp := time.Date(2026, 3, 31, 9, 0, 0, 0, time.UTC)
	if next := r.ComputeNextRun(from); !next.Equal(exp) {
		t.Fatalf("monthly default day mismatch: %v", next)
	}
}

func TestComputeNextRunYearly(t *testing.T) {
	r := &Recurring{RecurringType: RecYearly}
	from := time.Date(2026, 3, 15, 9, 0, 0, 0, time.UTC)
	if next := r.ComputeNextRun(from); !next.Equal(time.Date(2027, 3, 15, 9, 0, 0, 0, time.UTC)) {
		t.Fatalf("yearly mismatch: %v", next)
	}
}

func TestComputeNextRunCustom(t *testing.T) {
	r := &Recurring{RecurringType: RecCustom, Interval: 5}
	from := time.Date(2026, 3, 15, 9, 0, 0, 0, time.UTC)
	if next := r.ComputeNextRun(from); !next.Equal(from.AddDate(0, 0, 5)) {
		t.Fatalf("custom mismatch: %v", next)
	}
}

func TestComputeNextRunCustomDefaultInterval(t *testing.T) {
	r := &Recurring{RecurringType: RecCustom} // interval 0 -> 1
	from := time.Date(2026, 3, 15, 9, 0, 0, 0, time.UTC)
	if next := r.ComputeNextRun(from); !next.Equal(from.AddDate(0, 0, 1)) {
		t.Fatalf("custom default mismatch: %v", next)
	}
}

func TestComputeNextRunUnknown(t *testing.T) {
	r := &Recurring{RecurringType: "weird"}
	from := time.Date(2026, 3, 15, 9, 0, 0, 0, time.UTC)
	if next := r.ComputeNextRun(from); !next.Equal(from.AddDate(0, 0, 1)) {
		t.Fatalf("unknown mismatch: %v", next)
	}
}
