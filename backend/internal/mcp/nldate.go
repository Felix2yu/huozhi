package mcp

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// 本文件负责把「自然语言时间」翻译成日期区间。
//
// 为什么要做：账单查询的本质条件是时间范围（"上个月花了多少"、"最近三个月餐饮趋势"），
// 而 LLM 手边只有「今天」这个相对概念，让它自己算日期既不可靠也浪费 token。
// 把解析放在服务端，模型只需原样传递用户原话里的词（"上个月" / "最近30天"），
// 时区与起止边界由服务端统一裁决，避免模型本地时区与服务器不一致导致的差一天。

var (
	// 2026-01-31 / 2026/1/31 / 2026.1.31 / 2026年1月31日 / 2026-01（整月）/ 2026（整年）
	reISO = regexp.MustCompile(`^(\d{4})\s*[-/.年]\s*(\d{1,2})(?:\s*[-/.月]\s*(\d{1,2})\s*日?)?$`)
	// 最近7天 / 近3个月 / 过去一年 / last 30 days
	reRecent = regexp.MustCompile(`^(?:最近|近|过去|过去|past|last|recent)\s*([0-9一二三四五六七八九十百]+)\s*(个)?\s*(天|日|周|星期|月|年|days?|weeks?|months?|years?)$`)
	// 3天前 / 2个月后 / 3 days ago
	reOffset = regexp.MustCompile(`^([0-9一二三四五六七八九十]+)\s*(个)?\s*(天|日|周|星期|月|年|days?|weeks?|months?|years?)\s*(前|后|ago|later)$`)
	// 1月 / 3月份（不带年份 → 取最近的该月）
	reMonthOnly = regexp.MustCompile(`^(\d{1,2})\s*月份?$`)
)

// ParseDate 解析「单个日期」表达式，返回当日 00:00（服务器本地时区）。
//
// 实现上先按单日表达式解析，失败再退到区间表达式取其起始日（"上月" → 上月 1 号）。
// 注意与 ParseRange 的调用方向是单向的：ParseRange 末尾只用不回调 ParseDate 的
// parseSingleDate，否则两者会互相兜底形成无限递归。
func ParseDate(expr string, now time.Time) (time.Time, bool) {
	if d, ok := parseSingleDate(expr, now); ok {
		return d, true
	}
	// 兜底：区间表达式取起始日（"上个月" → 上月 1 号）
	if start, _, ok := ParseRange(expr, now); ok {
		return start, true
	}
	return time.Time{}, false
}

// parseSingleDate 只处理「单日」语义，不回调 ParseRange
func parseSingleDate(expr string, now time.Time) (time.Time, bool) {
	s := normalize(expr)
	if s == "" {
		return time.Time{}, false
	}
	switch s {
	case "今天", "今日", "本日", "当日", "today":
		return startOfDay(now), true
	case "昨天", "昨日", "yesterday":
		return startOfDay(now).AddDate(0, 0, -1), true
	case "前天":
		return startOfDay(now).AddDate(0, 0, -2), true
	case "大前天":
		return startOfDay(now).AddDate(0, 0, -3), true
	case "明天", "明日", "tomorrow":
		return startOfDay(now).AddDate(0, 0, 1), true
	case "后天":
		return startOfDay(now).AddDate(0, 0, 2), true
	}
	if m := reISO.FindStringSubmatch(s); m != nil {
		return fromISO(m, now)
	}
	if m := reOffset.FindStringSubmatch(s); m != nil {
		if n, ok := parseCount(m[1]); ok {
			sign := -1
			if m[4] == "后" || m[4] == "later" {
				sign = 1
			}
			return shiftByUnit(startOfDay(now), n*sign, m[3]), true
		}
	}
	if m := reMonthOnly.FindStringSubmatch(s); m != nil {
		month, _ := strconv.Atoi(m[1])
		if month >= 1 && month <= 12 {
			return time.Date(now.Year(), time.Month(month), 1, 0, 0, 0, 0, now.Location()), true
		}
	}
	return time.Time{}, false
}

// ParseRange 解析「时间范围」表达式，返回闭区间的起止日（均为当日 00:00）。
//
// 支持：今天/昨天/本周/上周/本月/上月/本季度/上季度/今年/去年/最近N天（周/月/年）、
// ISO 日期（单日 / 年-月 → 整月 / 年 → 整年）。
func ParseRange(expr string, now time.Time) (start, end time.Time, ok bool) {
	s := normalize(expr)
	if s == "" {
		return time.Time{}, time.Time{}, false
	}
	today := startOfDay(now)

	switch s {
	case "今天", "今日", "本日", "当日", "today":
		return today, today, true
	case "昨天", "昨日", "yesterday":
		d := today.AddDate(0, 0, -1)
		return d, d, true
	case "前天":
		d := today.AddDate(0, 0, -2)
		return d, d, true
	case "明天", "明日", "tomorrow":
		d := today.AddDate(0, 0, 1)
		return d, d, true
	case "本周", "这周", "本星期", "这个星期", "这星期", "thisweek", "this week":
		return startOfWeek(today), endOfWeek(today), true
	case "上周", "上个星期", "上星期", "lastweek", "last week":
		prev := today.AddDate(0, 0, -7)
		return startOfWeek(prev), endOfWeek(prev), true
	case "下周", "下星期", "nextweek", "next week":
		next := today.AddDate(0, 0, 7)
		return startOfWeek(next), endOfWeek(next), true
	case "本月", "这月", "这个月", "thismonth", "this month":
		return startOfMonth(today), endOfMonth(today), true
	case "上月", "上个月", "lastmonth", "last month":
		prev := startOfMonth(today).AddDate(0, -1, 0)
		return prev, endOfMonth(prev), true
	case "下月", "下个月", "nextmonth", "next month":
		next := startOfMonth(today).AddDate(0, 1, 0)
		return next, endOfMonth(next), true
	case "本季度", "这季度", "这个季度":
		return startOfQuarter(today), endOfQuarter(today), true
	case "上季度", "上个季度":
		prev := startOfQuarter(today).AddDate(0, -3, 0)
		return prev, endOfQuarter(prev), true
	case "今年", "本年", "thisyear", "this year":
		return time.Date(today.Year(), 1, 1, 0, 0, 0, 0, today.Location()),
			time.Date(today.Year(), 12, 31, 0, 0, 0, 0, today.Location()), true
	case "去年", "上年", "lastyear", "last year":
		y := today.Year() - 1
		return time.Date(y, 1, 1, 0, 0, 0, 0, today.Location()),
			time.Date(y, 12, 31, 0, 0, 0, 0, today.Location()), true
	case "明年", "nextyear", "next year":
		y := today.Year() + 1
		return time.Date(y, 1, 1, 0, 0, 0, 0, today.Location()),
			time.Date(y, 12, 31, 0, 0, 0, 0, today.Location()), true
	case "全部", "所有", "全部时间", "all", "alltime", "all time":
		return time.Date(1970, 1, 1, 0, 0, 0, 0, today.Location()), today, true
	}

	if m := reISO.FindStringSubmatch(s); m != nil {
		d, ok2 := fromISO(m, now)
		if !ok2 {
			return time.Time{}, time.Time{}, false
		}
		if m[3] == "" {
			// 只有年-月 → 整月；只有年 → 整年
			if m[2] == "" {
				return time.Time{}, time.Time{}, false
			}
			return startOfMonth(d), endOfMonth(d), true
		}
		return d, d, true
	}

	if m := reRecent.FindStringSubmatch(s); m != nil {
		n, ok2 := parseCount(m[1])
		if !ok2 || n <= 0 {
			return time.Time{}, time.Time{}, false
		}
		// "最近N天" 含今天：起点 = 今天 - (N-1) 天
		switch unitKey(m[3]) {
		case "day":
			return today.AddDate(0, 0, -(n - 1)), today, true
		case "week":
			return today.AddDate(0, 0, -(7*n - 1)), today, true
		case "month":
			return startOfMonth(today).AddDate(0, -(n - 1), 0), today, true
		case "year":
			return time.Date(today.Year()-n+1, 1, 1, 0, 0, 0, 0, today.Location()), today, true
		}
	}

	// 单日表达式（如 "3天前"）也可作为区间：起止同一天。
	// 这里只调用 parseSingleDate，绝不回调 ParseDate，避免循环兜底导致无限递归。
	if d, ok2 := parseSingleDate(s, now); ok2 {
		return d, d, true
	}
	return time.Time{}, time.Time{}, false
}

// ParseDateOrRange 同时给出区间的两种读法：调用方按需取 start 或 end。
func ParseDateOrRange(expr string, now time.Time) (start, end time.Time, ok bool) {
	return ParseRange(expr, now)
}

// ==================== 内部工具 ====================

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func startOfWeek(t time.Time) time.Time {
	d := startOfDay(t)
	// Go 的 Weekday：Sunday=0。换算成周一=0
	offset := int(d.Weekday()+6) % 7
	return d.AddDate(0, 0, -offset)
}

func endOfWeek(t time.Time) time.Time {
	return startOfWeek(t).AddDate(0, 0, 6)
}

func startOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}

func endOfMonth(t time.Time) time.Time {
	return startOfMonth(t).AddDate(0, 1, -1)
}

func startOfQuarter(t time.Time) time.Time {
	q := (int(t.Month())-1)/3*3 + 1
	return time.Date(t.Year(), time.Month(q), 1, 0, 0, 0, 0, t.Location())
}

func endOfQuarter(t time.Time) time.Time {
	return startOfQuarter(t).AddDate(0, 3, -1)
}

func fromISO(m []string, now time.Time) (time.Time, bool) {
	year, err := strconv.Atoi(m[1])
	if err != nil || year < 1970 || year > 9999 {
		return time.Time{}, false
	}
	if m[2] == "" {
		return time.Date(year, 1, 1, 0, 0, 0, 0, now.Location()), true
	}
	month, err := strconv.Atoi(m[2])
	if err != nil || month < 1 || month > 12 {
		return time.Time{}, false
	}
	day := 1
	if m[3] != "" {
		day, err = strconv.Atoi(m[3])
		if err != nil || day < 1 || day > 31 {
			return time.Time{}, false
		}
	}
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, now.Location()), true
}

func unitKey(s string) string {
	switch s {
	case "天", "日", "day", "days":
		return "day"
	case "周", "星期", "week", "weeks":
		return "week"
	case "月", "month", "months":
		return "month"
	case "年", "year", "years":
		return "year"
	}
	return ""
}

func shiftByUnit(t time.Time, n int, unit string) time.Time {
	switch unitKey(unit) {
	case "day":
		return t.AddDate(0, 0, n)
	case "week":
		return t.AddDate(0, 0, 7*n)
	case "month":
		return t.AddDate(0, n, 0)
	case "year":
		return t.AddDate(n, 0, 0)
	}
	return t
}

// parseCount 兼容阿拉伯数字与中文数字（一到九十九）
func parseCount(s string) (int, bool) {
	if n, err := strconv.Atoi(s); err == nil {
		return n, true
	}
	return cnNum(s)
}

var cnDigits = map[rune]int{'零': 0, '一': 1, '二': 2, '两': 2, '三': 3, '四': 4, '五': 5, '六': 6, '七': 7, '八': 8, '九': 9}

// cnNum 解析 1~99 的中文数字：十 / 十一 / 二十 / 二十一
func cnNum(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	runes := []rune(s)
	// 纯数字：一 / 二 / ... / 九 / 十
	if len(runes) == 1 {
		if runes[0] == '十' {
			return 10, true
		}
		if v, ok := cnDigits[runes[0]]; ok {
			return v, true
		}
		return 0, false
	}
	total := 0
	i := 0
	// 十位
	if runes[0] == '十' {
		total = 10
		i = 1
	} else if v, ok := cnDigits[runes[0]]; ok {
		if runes[1] == '十' {
			total = v * 10
			i = 2
		} else {
			return 0, false
		}
	} else {
		return 0, false
	}
	// 个位
	if i < len(runes) {
		v, ok := cnDigits[runes[i]]
		if !ok || i != len(runes)-1 {
			return 0, false
		}
		total += v
	}
	return total, true
}

// normalize 归一化输入：去空白、英文转小写、剔除常见冗余后缀
func normalize(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.ReplaceAll(s, " ", "")
	// 中文量词统一：一个月 → 一月
	s = strings.ReplaceAll(s, "一个月", "一月")
	s = strings.ReplaceAll(s, "一周", "一星期")
	replacer := strings.NewReplacer(
		"号", "日",
		"今日", "今天",
		"昨日", "昨天",
		"当月", "本月",
		"本月", "本月",
	)
	return replacer.Replace(s)
}
