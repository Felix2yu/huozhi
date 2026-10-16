package mcp

import "huozhi/internal/mcp/schema"

// 本文件声明对外暴露的 MCP 工具（名称、描述、入参 JSON Schema）与服务端指令。
// 具体实现见 bills.go；工具注册集中在一处，便于审计「AI 到底能碰什么数据」。

// serverInstructions 随 initialize 返回，会被客户端注入模型上下文。
// 写得具体一点，模型才不会把「金额」当成「分」、把「上月」算错。
const serverInstructions = `你是「货殖」个人记账系统的助手，可以通过工具查询、分析和修改用户的账单数据。

【单位与格式】
1. 所有金额一律使用「元」（如 35.8 表示 35 元 8 角），不要用「分」。
2. 日期支持自然语言：今天、昨天、前天、本周、上周、本月、上月、本季度、今年、去年、最近7天、最近3个月，也支持 2026-01-31 / 2026-01 这样的写法。
3. 分类、账户、账本、标签都可以直接给名称（如「餐饮」「招行信用卡」），服务端会做模糊匹配；匹配到多个时会返回候选，请你挑一个更精确的名称或直接改用 id 重试。

【推荐流程】
1. 查询/分析：先用 search_transactions 或 analyze_spending 取数，再基于返回的数据回答，不要凭空估算。
2. 修改：先用 search_transactions 定位到具体条目拿到 id，再调用 update_transaction / delete_transaction。
3. 拿不准时：create_transaction / update_transaction / delete_transaction 都支持 dry_run=true，会返回「将要发生的变更」而不落库，可先预演再执行。

【安全边界】
1. 删除是软删除，误删可用 recover_transaction 按 id 恢复。
2. batch_delete_transactions 一次影响多条记录，必须先取得用户明确同意，并显式传 confirm=true。
3. 不要为了「完成用户指令」而编造账单数据；查不到就如实说明。`

// RegisterTools 注册全部工具
func RegisterTools(s *Server) error {
	for _, t := range toolSpecs() {
		h, ok := toolHandlers[t.Name]
		if !ok {
			continue
		}
		if err := s.Register(t, h); err != nil {
			return err
		}
	}
	return nil
}

// ==================== 工具清单 ====================

func toolSpecs() []Tool {
	return []Tool{
		{
			Name:        "search_transactions",
			Description: "搜索/筛选账单流水。支持按时间范围（自然语言）、类型、分类、账户、账本、标签、关键词、金额区间组合筛选，返回按日期倒序的流水列表与合计。",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"start_date":      schema.Str("起始日期（含）。支持自然语言：今天、昨天、本周、上月、最近30天、2026-01-01 等；留空表示不限。"),
					"end_date":        schema.Str("结束日期（含）。写法同 start_date。"),
					"period":          schema.Enum("时间范围快捷词，与 start_date/end_date 二选一；同时给出时以 start_date/end_date 为准。", "today", "yesterday", "this_week", "last_week", "this_month", "last_month", "this_quarter", "last_quarter", "this_year", "last_year", "recent_7d", "recent_30d", "recent_90d"),
					"type":            schema.Enum("交易类型；不传表示全部类型。", "expense", "income", "transfer", "refund", "reimburse", "adjust"),
					"category":        schema.Str("分类名称或 id，如「餐饮」；支持模糊匹配。"),
					"account":         schema.Str("账户名称或 id，如「招行信用卡」。转账的收付款双方命中任一即算匹配。"),
					"book":            schema.Str("账本名称或 id；不传表示跨全部账本。"),
					"tag":             schema.Str("标签名称或 id。"),
					"keyword":         schema.Str("关键词，匹配备注/商家/地点/分类名；纯数字会按金额 ±10% 近似匹配。"),
					"min_amount":      schema.Num("最小金额（元，含）。"),
					"max_amount":      schema.Num("最大金额（元，含）。"),
					"limit":           schema.Int("每页条数，1-100，默认 20。"),
					"page":            schema.Int("页码，从 1 开始，默认 1。"),
					"include_deleted": schema.Bool("是否只查回收站里已删除的流水，默认 false。"),
				},
				AdditionalProperties: false,
			},
			Annotations: &ToolAnnotations{Title: "搜索账单", ReadOnlyHint: true, OpenWorldHint: false},
		},
		{
			Name:        "get_transaction",
			Description: "按 id 获取单条账单的完整信息（含分类名、账户名、标签）。",
			InputSchema: ToolInputSchema{
				Type:                 "object",
				Properties:           map[string]any{"id": schema.Str("账单 id（必填）。")},
				Required:             []string{"id"},
				AdditionalProperties: false,
			},
			Annotations: &ToolAnnotations{Title: "获取账单详情", ReadOnlyHint: true},
		},
		{
			Name:        "analyze_spending",
			Description: "消费/收入分析：给出区间汇总（收入、支出、结余、日均、笔数）、指定维度的占比排行（分类/账户/标签/商家/按日/按周/按月趋势）、Top 大额流水，并可选与上一个等长周期对比，用于回答「花了多少」「哪类花最多」「比上月涨了还是降了」「消费趋势如何」。",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"start_date":       schema.Str("起始日期（含），支持自然语言；默认本月 1 号。"),
					"end_date":         schema.Str("结束日期（含），支持自然语言；默认今天。"),
					"period":           schema.Enum("时间范围快捷词，与 start_date/end_date 二选一。", "today", "yesterday", "this_week", "last_week", "this_month", "last_month", "this_quarter", "last_quarter", "this_year", "last_year", "recent_7d", "recent_30d", "recent_90d"),
					"kind":             schema.Enum("统计口径：支出 / 收入 / 全部，默认 expense。", "expense", "income", "all"),
					"dimension":        schema.Enum("拆分维度，默认 category。trend 维度指按时间看趋势，会同时给出按分类占比。", "category", "account", "tag", "merchant", "day", "week", "month"),
					"book":             schema.Str("账本名称或 id；不传表示跨全部账本。"),
					"category":         schema.Str("只统计某个分类（名称或 id）。"),
					"account":          schema.Str("只统计某个账户（名称或 id）。"),
					"top_n":            schema.Int("占比排行返回前 N 项，1-50，默认 10。"),
					"compare_previous": schema.Bool("是否对比上一个等长周期（环比），默认 true。"),
				},
				AdditionalProperties: false,
			},
			Annotations: &ToolAnnotations{Title: "消费分析", ReadOnlyHint: true},
		},
		{
			Name:        "get_budget_status",
			Description: "查看预算执行情况：每个预算的周期、额度、已用、剩余、使用率与是否超支。",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"book":            schema.Str("账本名称或 id；不传表示跨全部账本。"),
					"only_over_alert": schema.Bool("只看使用率达到预警线的预算，默认 false。"),
				},
				AdditionalProperties: false,
			},
			Annotations: &ToolAnnotations{Title: "预算执行情况", ReadOnlyHint: true},
		},
		{
			Name:        "list_books",
			Description: "列出全部账本（含共享账本），返回 id、名称、币种、是否默认。记账前用它确认账本。",
			InputSchema: ToolInputSchema{Type: "object", Properties: map[string]any{}, AdditionalProperties: false},
			Annotations: &ToolAnnotations{Title: "账本列表", ReadOnlyHint: true},
		},
		{
			Name:        "list_categories",
			Description: "列出分类字典（id、名称、收支类型、父分类）。记账选分类前先查它，不要臆造分类名。",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"kind":    schema.Enum("按收支类型过滤；不传返回全部。", "expense", "income", "system"),
					"book":    schema.Str("账本名称或 id；不传表示跨全部账本。"),
					"flatten": schema.Bool("是否平铺返回（默认 true，便于模型阅读）；传 false 返回两级树。"),
					"keyword": schema.Str("按名称过滤。"),
				},
				AdditionalProperties: false,
			},
			Annotations: &ToolAnnotations{Title: "分类列表", ReadOnlyHint: true},
		},
		{
			Name:        "list_accounts",
			Description: "列出账户/资产（id、名称、类型、当前余额、币种）。记账选账户前先查它。",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"type":             schema.Str("按类型过滤：cash/bank/credit/prepaid/investment/liability/virtual。"),
					"book":             schema.Str("账本名称或 id；不传表示跨全部账本。"),
					"include_archived": schema.Bool("是否包含已归档账户，默认 false。"),
				},
				AdditionalProperties: false,
			},
			Annotations: &ToolAnnotations{Title: "账户列表", ReadOnlyHint: true},
		},
		{
			Name:        "list_tags",
			Description: "列出标签及其使用次数。",
			InputSchema: ToolInputSchema{
				Type:                 "object",
				Properties:           map[string]any{"book": schema.Str("账本名称或 id；不传表示跨全部账本。")},
				AdditionalProperties: false,
			},
			Annotations: &ToolAnnotations{Title: "标签列表", ReadOnlyHint: true},
		},
		{
			Name:        "create_transaction",
			Description: "新增一笔账单。会自动同步账户余额、占用预算、按补丁规则生成转账手续费流水。不确定参数时可先传 dry_run=true 预演。",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"amount":        schema.Num("金额（元，必填，必须大于 0）。"),
					"type":          schema.Enum("类型，默认 expense。", "expense", "income", "transfer", "refund", "reimburse", "adjust"),
					"category":      schema.Str("分类名称或 id。不传时尝试从 description 里匹配已有分类名，再退到「其他」。"),
					"account":       schema.Str("账户名称或 id。不传时若只有一个账户则用它，否则报错。"),
					"to_account":    schema.Str("目标账户名称或 id，type=transfer 时必填。"),
					"book":          schema.Str("账本名称或 id。不传时用默认账本。"),
					"date":          schema.Str("记账日期，支持自然语言（今天/昨天/上月5号…），默认今天。"),
					"description":   schema.Str("备注，如「午餐 星巴克」。"),
					"merchant":      schema.Str("商家。"),
					"location":      schema.Str("地点。"),
					"remark":        schema.Str("备注说明。"),
					"tags":          schema.ArrStr("标签名数组，如 [\"出差\"]。不存在的标签会被跳过。"),
					"transfer_fee":  schema.Num("转账手续费（元），仅 type=transfer 时有效。"),
					"currency":      schema.Str("币种，默认 CNY。"),
					"exchange_rate": schema.Num("汇率：1 单位原币 = ? 单位基准币，默认 1。"),
					"dry_run":       schema.Bool("预演：只返回将要创建的账单与影响，不落库。"),
				},
				Required:             []string{"amount"},
				AdditionalProperties: false,
			},
			Annotations: &ToolAnnotations{Title: "新增账单", ReadOnlyHint: false, DestructiveHint: false},
		},
		{
			Name:        "update_transaction",
			Description: "修改一笔已有账单（补丁语义：只改你传的字段，未传的保持原值）。改金额/账户会自动重算余额与预算。",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"id":           schema.Str("账单 id（必填）。"),
					"amount":       schema.Num("新金额（元）。"),
					"type":         schema.Enum("新类型。", "expense", "income", "transfer", "refund", "reimburse", "adjust"),
					"category":     schema.Str("新分类名称或 id。"),
					"account":      schema.Str("新账户名称或 id。"),
					"to_account":   schema.Str("新目标账户名称或 id。"),
					"book":         schema.Str("新账本名称或 id。"),
					"date":         schema.Str("新记账日期，支持自然语言。"),
					"description":  schema.Str("新备注。"),
					"merchant":     schema.Str("新商家。"),
					"location":     schema.Str("新地点。"),
					"remark":       schema.Str("新备注说明。"),
					"tags":         schema.ArrStr("新标签名数组（会整体替换原有标签）。"),
					"transfer_fee": schema.Num("转账手续费（元）。"),
					"dry_run":      schema.Bool("预演：返回修改前后的对照，不落库。"),
				},
				Required:             []string{"id"},
				AdditionalProperties: false,
			},
			Annotations: &ToolAnnotations{Title: "修改账单", ReadOnlyHint: false},
		},
		{
			Name:        "delete_transaction",
			Description: "删除一笔账单（软删除，进回收站，可用 recover_transaction 恢复）。会自动回滚账户余额与预算占用。",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"id":      schema.Str("账单 id（必填）。"),
					"dry_run": schema.Bool("预演：只返回将被删除的账单与影响，不落库。"),
				},
				Required:             []string{"id"},
				AdditionalProperties: false,
			},
			Annotations: &ToolAnnotations{Title: "删除账单", ReadOnlyHint: false, DestructiveHint: true},
		},
		{
			Name:        "recover_transaction",
			Description: "从回收站恢复一笔已删除的账单，并补回账户余额与预算占用。",
			InputSchema: ToolInputSchema{
				Type:                 "object",
				Properties:           map[string]any{"id": schema.Str("账单 id（必填）。")},
				Required:             []string{"id"},
				AdditionalProperties: false,
			},
			Annotations: &ToolAnnotations{Title: "恢复账单", ReadOnlyHint: false},
		},
		{
			Name:        "batch_delete_transactions",
			Description: "批量删除多笔账单（软删除）。影响面大：必须先取得用户明确同意，并显式传 confirm=true（可配合 dry_run 先预演）。单次最多 200 笔。",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"ids":     schema.ArrNum("账单 id 数组（必填）。"),
					"confirm": schema.Bool("确认执行，必须显式传 true。"),
					"dry_run": schema.Bool("预演：返回将被删除的账单清单与合计，不落库。"),
				},
				Required:             []string{"ids"},
				AdditionalProperties: false,
			},
			Annotations: &ToolAnnotations{Title: "批量删除账单", ReadOnlyHint: false, DestructiveHint: true},
		},
	}
}
