package database

import (
	"huozhi/internal/config"
)

// IsPostgres 判断当前数据库是否为 PostgreSQL。
// SQLite 与 PostgreSQL 在日期/字符串函数上差异很大（strftime / DATE() 为 SQLite 专有），
// 涉及原生 SQL 表达式的查询必须按方言分支，否则切换 driver 后直接报错。
func IsPostgres() bool {
	if config.AppConfig == nil {
		return false
	}
	return config.AppConfig.Database.Driver == "postgres"
}

// LikeExpr 返回按方言生成的 LIKE 表达式。
//   - SQLite:     LIKE（默认不区分大小写）
//   - PostgreSQL: ILIKE（大小写不敏感）
func LikeExpr() string {
	if IsPostgres() {
		return "ILIKE"
	}
	return "LIKE"
}

// DateGroupExpr 返回按给定粒度对日期列分组的 SQL 表达式。
// grain 支持 "day" | "week" | "month"。
//   - SQLite:     strftime('%Y-%m-%d', col)
//   - PostgreSQL: to_char(col, 'YYYY-MM-DD')
func DateGroupExpr(col string, grain string) string {
	if IsPostgres() {
		switch grain {
		case "month":
			return "to_char(" + col + ", 'YYYY-MM')"
		case "week":
			// ISO 周：IYYY-IW，与 SQLite 的 %G-W%V 语义一致
			return "to_char(" + col + ", 'IYYY-IW')"
		default:
			return "to_char(" + col + ", 'YYYY-MM-DD')"
		}
	}
	switch grain {
	case "month":
		return "strftime('%Y-%m', " + col + ")"
	case "week":
		return "strftime('%G-W%V', " + col + ")"
	default:
		return "strftime('%Y-%m-%d', " + col + ")"
	}
}
