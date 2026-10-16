package models

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
	"time"
)

func TestMoneyScanFloatCents(t *testing.T) {
	var amount Money
	if err := amount.Scan(float64(5250411)); err != nil {
		t.Fatalf("扫描金额失败: %v", err)
	}
	if amount != Money(5250411) {
		t.Fatalf("数据库金额应保持分单位，得到 %d，期望 5250411", amount)
	}
}

func TestMoneyConversions(t *testing.T) {
	for _, tt := range []struct {
		name  string
		yuan  float64
		cents float64
		want  Money
	}{
		{"零", 0, 0, 0},
		{"整数", 12, 1200, 1200},
		{"小数", 12.34, 1234, 1234},
		{"向上舍入", 1.236, 123.6, 124},
		{"向下舍入", 1.234, 123.4, 123},
		{"半分", 0.125, 12.5, 13},
		{"负半分", -0.125, -12.5, -13},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := FromYuan(tt.yuan); got != tt.want {
				t.Fatalf("FromYuan = %d，期望 %d", got, tt.want)
			}
			if got := FromCents(tt.cents); got != tt.want {
				t.Fatalf("FromCents = %d，期望 %d", got, tt.want)
			}
			if got := tt.want.Yuan(); got != float64(tt.want)/100 {
				t.Fatalf("Yuan = %v", got)
			}
			value, err := tt.want.Value()
			if err != nil || value != int64(tt.want) {
				t.Fatalf("Value = %v，错误 %v", value, err)
			}
			var scanned Money
			if err := scanned.Scan(value); err != nil || scanned != tt.want {
				t.Fatalf("数据库往返得到 %d，错误 %v", scanned, err)
			}
		})
	}
}

func TestMoneyJSON(t *testing.T) {
	for _, tt := range []struct {
		input   string
		want    Money
		wantErr bool
	}{
		{"0", 0, false},
		{"12.34", 1234, false},
		{`"12.34"`, 1234, false},
		{" -1.236 ", -124, false},
		{"null", 0, false},
		{`""`, 0, false},
		{`"无效"`, 0, true},
		{"true", 0, true},
		{"{}", 0, true},
	} {
		t.Run(tt.input, func(t *testing.T) {
			amount := Money(99)
			err := json.Unmarshal([]byte(tt.input), &amount)
			if (err != nil) != tt.wantErr {
				t.Fatalf("解析错误 = %v，期望错误 %v", err, tt.wantErr)
			}
			if tt.wantErr {
				if amount != 99 {
					t.Fatalf("解析失败修改了原金额: %d", amount)
				}
				return
			}
			if amount != tt.want {
				t.Fatalf("金额 = %d，期望 %d", amount, tt.want)
			}
		})
	}
	for _, tt := range []struct {
		amount Money
		want   string
	}{
		{0, "0"},
		{1234, "12.34"},
		{-1, "-0.01"},
		{5250411, "52504.11"},
	} {
		t.Run(tt.want, func(t *testing.T) {
			data, err := json.Marshal(tt.amount)
			if err != nil || string(data) != tt.want {
				t.Fatalf("序列化 = %s，期望 %s，错误 %v", data, tt.want, err)
			}
			var amount Money
			if err := json.Unmarshal(data, &amount); err != nil || amount != tt.amount {
				t.Fatalf("JSON 往返得到 %d，错误 %v", amount, err)
			}
		})
	}
}

func TestMoneyUnmarshalText(t *testing.T) {
	for _, tt := range []struct {
		input   string
		want    Money
		wantErr bool
	}{
		{"", 0, false},
		{"0", 0, false},
		{"1,234.56", 123456, false},
		{"-12.346", -1235, false},
		{"无效", 0, true},
	} {
		t.Run(tt.input, func(t *testing.T) {
			amount := Money(99)
			err := amount.UnmarshalText([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Fatalf("解析错误 = %v，期望错误 %v", err, tt.wantErr)
			}
			want := tt.want
			if tt.wantErr {
				want = 99
			}
			if amount != want {
				t.Fatalf("金额 = %d，期望 %d", amount, want)
			}
		})
	}
}

func TestMoneyScan(t *testing.T) {
	for _, tt := range []struct {
		name    string
		input   any
		want    Money
		wantErr bool
	}{
		{"空值", nil, 0, false},
		{"整数分", int64(1234), 1234, false},
		{"浮点舍入", 1234.6, 1235, false},
		{"负数舍入", -1234.5, -1235, false},
		{"字符串分", "1234", 1234, false},
		{"带空格和逗号", " 1,234.6 ", 1235, false},
		{"字节分", []byte("1234.5"), 1235, false},
		{"空字符串", "  ", 0, false},
		{"空字节", []byte{}, 0, false},
		{"无效字符串", "无效", 0, true},
		{"无效字节", []byte("无效"), 0, true},
		{"不支持的类型", true, 0, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			amount := Money(99)
			err := amount.Scan(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("扫描错误 = %v，期望错误 %v", err, tt.wantErr)
			}
			want := tt.want
			if tt.wantErr {
				want = 99
			}
			if amount != want {
				t.Fatalf("金额 = %d，期望 %d", amount, want)
			}
		})
	}
}

func TestMoneyString(t *testing.T) {
	for _, tt := range []struct {
		amount Money
		want   string
	}{
		{0, "0.00"},
		{1, "0.01"},
		{-1, "-0.01"},
		{1230, "12.30"},
		{-1234, "-12.34"},
		{5250411, "52504.11"},
	} {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.amount.String(); got != tt.want {
				t.Fatalf("String = %s，期望 %s", got, tt.want)
			}
		})
	}
}

func TestTransactionRules(t *testing.T) {
	for _, tt := range []struct {
		typeName  TransactionType
		direction int64
		bucket    string
	}{
		{TxExpense, -1, StatsBucketExpense},
		{TxReimburse, -1, StatsBucketExpense},
		{TxIncome, 1, StatsBucketIncome},
		{TxRefund, 1, StatsBucketIncome},
		{TxTransfer, 0, StatsBucketNone},
		{TxAdjust, 0, StatsBucketNone},
		{TransactionType("未知"), 0, StatsBucketNone},
	} {
		t.Run(string(tt.typeName), func(t *testing.T) {
			if got := TxBalanceDirection(tt.typeName); got != tt.direction {
				t.Fatalf("资金方向 = %d，期望 %d", got, tt.direction)
			}
			if got := TxStatsBucket(tt.typeName); got != tt.bucket {
				t.Fatalf("统计口径 = %q，期望 %q", got, tt.bucket)
			}
		})
	}
	for _, tt := range []struct {
		bucket string
		want   []string
	}{
		{StatsBucketExpense, []string{string(TxExpense), string(TxReimburse)}},
		{StatsBucketIncome, []string{string(TxIncome), string(TxRefund)}},
		{StatsBucketNone, []string{string(TxTransfer), string(TxAdjust)}},
		{"未知", []string{}},
	} {
		t.Run(tt.bucket, func(t *testing.T) {
			if got := TypesInBucket(tt.bucket); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("交易类型 = %v，期望 %v", got, tt.want)
			}
		})
	}
}

func TestTransactionBaseMoney(t *testing.T) {
	for _, tt := range []struct {
		name       string
		tx         *Transaction
		fee        Money
		wantRate   float64
		wantAmount Money
		wantFee    Money
	}{
		{"空交易", nil, 125, 1, 0, 125},
		{"零汇率", &Transaction{Amount: 10000}, 125, 1, 10000, 125},
		{"负汇率", &Transaction{Amount: 10000, ExchangeRate: -2}, 125, 1, 10000, 125},
		{"非数汇率", &Transaction{Amount: 10000, ExchangeRate: math.NaN()}, 125, 1, 10000, 125},
		{"正无穷", &Transaction{Amount: 10000, ExchangeRate: math.Inf(1)}, 125, 1, 10000, 125},
		{"负无穷", &Transaction{Amount: 10000, ExchangeRate: math.Inf(-1)}, 125, 1, 10000, 125},
		{"同币种", &Transaction{Amount: 10000, ExchangeRate: 1}, 125, 1, 10000, 125},
		{"外币", &Transaction{Amount: 10000, ExchangeRate: 7.18}, 100, 7.18, 71800, 718},
		{"正半分", &Transaction{Amount: 101, ExchangeRate: 0.5}, 3, 0.5, 51, 2},
		{"负半分", &Transaction{Amount: -101, ExchangeRate: 0.5}, -3, 0.5, -51, -2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.tx.BaseRate(); got != tt.wantRate {
				t.Fatalf("基准汇率 = %v，期望 %v", got, tt.wantRate)
			}
			if got := tt.tx.AmountInBase(); got != tt.wantAmount {
				t.Fatalf("基准金额 = %d，期望 %d", got, tt.wantAmount)
			}
			if got := tt.tx.ToBaseMoney(tt.fee); got != tt.wantFee {
				t.Fatalf("基准附属金额 = %d，期望 %d", got, tt.wantFee)
			}
		})
	}
}

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
	r := &Recurring{RecurringType: RecWeekly}            // Weekday 0 -> treated as Sunday -> next Sunday (+7)
	from := time.Date(2026, 3, 15, 9, 0, 0, 0, time.UTC) // Sunday
	if next := r.ComputeNextRun(from); !next.Equal(from.AddDate(0, 0, 7)) {
		t.Fatalf("weekly default mismatch: %v", next)
	}
}

func TestComputeNextRunWeeklyTarget(t *testing.T) {
	from := time.Date(2026, 3, 16, 9, 0, 0, 0, time.UTC)  // Monday
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
	// 1/31 + 1 个月必须落在 2 月（收敛到 2/28），不能因 AddDate 归一化跳过整个 2 月。
	// 旧实现返回 2026-03-31 —— 每月 31 号的周期在 1 月后会直接跳到 3 月。
	exp := time.Date(2026, 2, 28, 9, 0, 0, 0, time.UTC)
	if next := r.ComputeNextRun(from); !next.Equal(exp) {
		t.Fatalf("monthly clamp mismatch: got %v want %v", next, exp)
	}
	// 闰年 2 月应有 29 天
	fromLeap := time.Date(2028, 1, 31, 9, 0, 0, 0, time.UTC)
	expLeap := time.Date(2028, 2, 29, 9, 0, 0, 0, time.UTC)
	if next := r.ComputeNextRun(fromLeap); !next.Equal(expLeap) {
		t.Fatalf("monthly leap clamp mismatch: got %v want %v", next, expLeap)
	}
}

func TestComputeNextRunMonthlyDefaultDay(t *testing.T) {
	r := &Recurring{RecurringType: RecMonthly} // MonthDay 0 -> from.Day()
	from := time.Date(2026, 1, 31, 9, 0, 0, 0, time.UTC)
	exp := time.Date(2026, 2, 28, 9, 0, 0, 0, time.UTC)
	if next := r.ComputeNextRun(from); !next.Equal(exp) {
		t.Fatalf("monthly default day mismatch: got %v want %v", next, exp)
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
