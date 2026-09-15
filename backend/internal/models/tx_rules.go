package models

import "math"

// 交易类型 → 资金方向 / 收支统计口径的**唯一定义**。
//
// 修复背景（原 B-04）：余额引擎把 `reimburse` 当作支出扣减账户余额，
// 但日小计、全站汇总、对账重算各自写了一份 switch 且全部漏掉 reimburse，
// 导致「明细加总 ≠ 日小计 ≠ 余额变动」。这里把映射收敛成单一事实来源，
// 所有口径必须引用它，不允许再各自 switch。

const (
	StatsBucketIncome  = "income"  // 计入收入
	StatsBucketExpense = "expense" // 计入支出
	StatsBucketNone    = ""        // 不计入收支（转账、余额调整）
)

// TxBalanceDirection 该交易类型对「单一账户」的资金方向。
// +1 增加、-1 减少、0 表示余额影响由调用方特殊处理（transfer 双向、adjust 不走余额引擎）。
func TxBalanceDirection(t TransactionType) int64 {
	switch t {
	case TxExpense, TxReimburse:
		return -1
	case TxIncome, TxRefund:
		return 1
	default:
		return 0
	}
}

// TxStatsBucket 该交易类型归入哪个收支统计口径。
// 空串表示不计入收支汇总（转账、余额调整）。
func TxStatsBucket(t TransactionType) string {
	switch t {
	case TxIncome, TxRefund:
		return StatsBucketIncome
	case TxExpense, TxReimburse:
		return StatsBucketExpense
	default:
		return StatsBucketNone
	}
}

// TypesInBucket 返回归入某个统计口径的全部交易类型（用于 SQL 的 type IN ?）。
// 与 TxStatsBucket 同源，避免 SQL 里手写类型列表时漏掉 reimburse。
func TypesInBucket(bucket string) []string {
	out := make([]string, 0, 3)
	for _, t := range []TransactionType{TxExpense, TxIncome, TxRefund, TxReimburse, TxTransfer, TxAdjust} {
		if TxStatsBucket(t) == bucket {
			out = append(out, string(t))
		}
	}
	return out
}

// BaseRate 取折算到基准币种的汇率。
//
// 0 / 负数 / NaN / ±Inf 一律按 1 处理：这类值只可能来自「未填写」或脏数据，
// 按 1 折算等价于「原币金额即基准币金额」，不会把金额放大成 0 或天文数字。
// 此前库里被写入过 0（Go 零值不满足 GORM 默认值触发条件），必须在读取侧兜底。
func (t *Transaction) BaseRate() float64 {
	if t == nil {
		return 1
	}
	if t.ExchangeRate <= 0 || math.IsNaN(t.ExchangeRate) || math.IsInf(t.ExchangeRate, 0) {
		return 1
	}
	return t.ExchangeRate
}

// AmountInBase 折算到基准币种的金额（分）。
//
// 修复背景（原 B-02）：`exchange_rate` 此前从未参与任何算术运算，
// 录入 100 USD / 汇率 7.18 只会从账户余额扣 1.00 元（偏差 718 倍）。
// 凡计算「真实资金变动」的场合（账户余额、预算占用、收支汇总、对账重算）
// 都必须用本函数，禁止直接使用 t.Amount —— 后者是原币金额。
func (t *Transaction) AmountInBase() Money {
	if t == nil {
		return 0
	}
	r := t.BaseRate()
	if r == 1 {
		return t.Amount
	}
	return Money(math.Round(float64(t.Amount) * r))
}

// ToBaseMoney 按本笔交易的汇率把任意附属金额（手续费 / 转账优惠）折算到基准币种。
func (t *Transaction) ToBaseMoney(m Money) Money {
	if t == nil {
		return m
	}
	r := t.BaseRate()
	if r == 1 {
		return m
	}
	return Money(math.Round(float64(m) * r))
}
