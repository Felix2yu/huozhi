package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Money 金额，单位：分（最小货币单位）。
// 全部金额字段使用整数分存储与运算，杜绝 float64 二进制浮点累加误差
// （如 52504.10999999991）。JSON 对外仍序列化为「元」的浮点数
// （整数分 /100 得到的是精确两位小数的最近浮点表示，序列化输出即为
// "52504.11"，前端无需改动）；数据库落 bigint 整数列，SQLite/PostgreSQL 通用。
type Money int64

// FromYuan 由「元」的浮点值构造 Money（四舍五入到分）。
func FromYuan(f float64) Money {
	return Money(math.Round(f * 100))
}

// FromCents 将数据库聚合结果（如 SUM(amount)）返回的浮点分数值收敛为整数分。
// SQLite 的 SUM 返回整数、PostgreSQL 的 SUM(bigint) 返回 numeric，
// 扫描到 float64 后用此函数取整，不能再走 FromYuan（那是元->分）。
func FromCents(f float64) Money {
	return Money(math.Round(f))
}

// Yuan 转回「元」的浮点值（仅用于展示/序列化与比率计算，不参与累加存储）。
func (m Money) Yuan() float64 {
	return float64(m) / 100
}

// MarshalJSON 序列化为元（浮点数字段，兼容既有前端类型 number）。
func (m Money) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.Yuan())
}

// UnmarshalJSON 兼容数字与字符串形式的元金额。
func (m *Money) UnmarshalJSON(b []byte) error {
	s := strings.Trim(strings.TrimSpace(string(b)), `"`)
	if s == "" || s == "null" {
		*m = 0
		return nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fmt.Errorf("金额格式错误: %s", s)
	}
	*m = FromYuan(f)
	return nil
}

// UnmarshalText 兼容表单/query 绑定（如 min_amount=12.34）。
func (m *Money) UnmarshalText(s []byte) error {
	t := strings.ReplaceAll(string(s), ",", "")
	if t == "" {
		*m = 0
		return nil
	}
	f, err := strconv.ParseFloat(t, 64)
	if err != nil {
		return fmt.Errorf("金额格式错误: %s", t)
	}
	*m = FromYuan(f)
	return nil
}

// Value 实现 driver.Valuer：数据库存整数分。
func (m Money) Value() (driver.Value, error) {
	return int64(m), nil
}

// Scan 实现 sql.Scanner。注意：迁移完成后库里存的统一是整数分，
// SQLite 的 REAL 亲和性列会把整数分读回为 float（如 5250411.0），
// 因此 float 走 FromCents 取整，而不是按元再乘 100。
func (m *Money) Scan(v interface{}) error {
	if v == nil {
		*m = 0
		return nil
	}
	switch x := v.(type) {
	case int64:
		*m = Money(x)
	case float64:
		*m = FromCents(x)
	case []byte:
		return m.scanString(string(x))
	case string:
		return m.scanString(x)
	default:
		return fmt.Errorf("无法将 %T 扫描为 Money", v)
	}
	return nil
}

func (m *Money) scanString(s string) error {
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", "")
	if s == "" {
		*m = 0
		return nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fmt.Errorf("金额格式错误: %s", s)
	}
	*m = FromCents(f)
	return nil
}

// String 输出元的两位小数字符串（如 1234.5 分 -> "12.35"），不经过浮点。
func (m Money) String() string {
	v := int64(m)
	neg := v < 0
	if neg {
		v = -v
	}
	s := fmt.Sprintf("%d.%02d", v/100, v%100)
	if neg {
		s = "-" + s
	}
	return s
}
