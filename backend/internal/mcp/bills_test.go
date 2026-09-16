package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"huozhi/internal/database"
	"huozhi/internal/dto"
	"huozhi/internal/handlers"
	"huozhi/internal/models"
	"strings"
	"testing"
	"time"
)

// ==================== 测试脚手架 ====================

func strPtr(s string) *string { return &s }

// newTestUser 建一个带默认账本、分类、账户、标签的用户
func newTestUser(t *testing.T, name string) (uid, bookID uint) {
	t.Helper()
	key := fmt.Sprintf("%064d", time.Now().UnixNano())
	u := models.User{
		Username:      name,
		PasswordHash:  "x",
		Nickname:      name,
		Status:        1,
		APIKey:        strPtr(key),
		APIKeyEnabled: true,
		Currency:      "CNY",
	}
	if err := database.DB.Create(&u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	book := models.Book{UserID: u.ID, Name: "默认账本", Currency: "CNY", IsDefault: true}
	if err := database.DB.Create(&book).Error; err != nil {
		t.Fatalf("create book: %v", err)
	}
	cats := []models.Category{
		{UserID: u.ID, BookID: book.ID, Name: "餐饮", Kind: models.KindExpense, Sort: 1},
		{UserID: u.ID, BookID: book.ID, Name: "交通", Kind: models.KindExpense, Sort: 2},
		{UserID: u.ID, BookID: book.ID, Name: "购物", Kind: models.KindExpense, Sort: 3},
		{UserID: u.ID, BookID: book.ID, Name: "其他", Kind: models.KindExpense, Sort: 4},
		{UserID: u.ID, BookID: book.ID, Name: "工资", Kind: models.KindIncome, Sort: 1},
	}
	if err := database.DB.Create(&cats).Error; err != nil {
		t.Fatalf("create categories: %v", err)
	}
	accs := []models.Account{
		{UserID: u.ID, BookID: book.ID, Name: "现金", Type: models.AccCash, Currency: "CNY"},
		{UserID: u.ID, BookID: book.ID, Name: "招行储蓄卡", Type: models.AccBank, Currency: "CNY"},
	}
	if err := database.DB.Create(&accs).Error; err != nil {
		t.Fatalf("create accounts: %v", err)
	}
	tags := []models.Tag{
		{UserID: u.ID, BookID: book.ID, Name: "出差"},
		{UserID: u.ID, BookID: book.ID, Name: "家庭"},
	}
	if err := database.DB.Create(&tags).Error; err != nil {
		t.Fatalf("create tags: %v", err)
	}
	return u.ID, book.ID
}

func newTestServer(t *testing.T) *Server {
	t.Helper()
	s := New()
	if err := RegisterTools(s); err != nil {
		t.Fatalf("register tools: %v", err)
	}
	return s
}

// call 走一遍完整的 JSON-RPC 分发链路，返回工具结果
func call(t *testing.T, s *Server, uid uint, name string, args map[string]any) (any, error) {
	t.Helper()
	params, err := json.Marshal(CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(Request{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call", Params: params})
	if err != nil {
		t.Fatal(err)
	}
	payload, ok, err := s.HandleBytes(context.Background(), uid, raw)
	if err != nil {
		t.Fatalf("HandleBytes: %v", err)
	}
	if !ok {
		t.Fatal("expected response")
	}
	var resp Response
	if err := json.Unmarshal(payload, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("rpc error: %+v", resp.Error)
	}
	var result CallToolResult
	b, _ := json.Marshal(resp.Result)
	if err := json.Unmarshal(b, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.IsError {
		return nil, fmt.Errorf("%s", result.Content[0].Text)
	}
	return result.StructuredContent, nil
}

// mustMap 把结果断言为 map
func mustMap(t *testing.T, v any) map[string]any {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", v)
	}
	return m
}

// ==================== 协议层 ====================

func TestNegotiateProtocolVersion(t *testing.T) {
	cases := map[string]string{
		"2025-06-18": "2025-06-18",
		"2025-03-26": "2025-03-26",
		"2024-11-05": "2024-11-05",
		"2026-01-01": ProtocolVersionLatest, // 未来的版本回落到服务端最新
		"":           ProtocolVersionLatest,
	}
	for in, want := range cases {
		if got := NegotiateProtocolVersion(in); got != want {
			t.Errorf("NegotiateProtocolVersion(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestInitializeHandshake(t *testing.T) {
	s := newTestServer(t)
	params, _ := json.Marshal(map[string]any{"protocolVersion": "2025-03-26", "clientInfo": map[string]any{"name": "test"}})
	raw, _ := json.Marshal(Request{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "initialize", Params: params})
	payload, ok, err := s.HandleBytes(context.Background(), 1, raw)
	if err != nil || !ok {
		t.Fatalf("initialize failed: %v", err)
	}
	var resp Response
	json.Unmarshal(payload, &resp)
	b, _ := json.Marshal(resp.Result)
	var init InitializeResult
	json.Unmarshal(b, &init)
	if init.ProtocolVersion != "2025-03-26" {
		t.Errorf("protocolVersion = %s", init.ProtocolVersion)
	}
	if init.ServerInfo.Name != ServerName {
		t.Errorf("serverInfo.name = %s", init.ServerInfo.Name)
	}
	if init.Capabilities.Tools == nil {
		t.Error("capabilities.tools missing")
	}
	if !strings.Contains(init.Instructions, "元") {
		t.Error("instructions 应说明金额单位")
	}
}

func TestToolsListCount(t *testing.T) {
	s := newTestServer(t)
	raw, _ := json.Marshal(Request{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/list"})
	payload, _, _ := s.HandleBytes(context.Background(), 1, raw)
	var resp Response
	json.Unmarshal(payload, &resp)
	b, _ := json.Marshal(resp.Result)
	var list ListToolsResult
	json.Unmarshal(b, &list)
	if len(list.Tools) != len(toolHandlers) {
		t.Fatalf("tools = %d, handlers = %d（有工具没有绑定实现）", len(list.Tools), len(toolHandlers))
	}
	for _, tool := range list.Tools {
		if tool.InputSchema.Type != "object" {
			t.Errorf("%s: inputSchema.type = %q", tool.Name, tool.InputSchema.Type)
		}
		if tool.Description == "" {
			t.Errorf("%s: description 为空", tool.Name)
		}
	}
}

func TestNotificationHasNoResponse(t *testing.T) {
	s := newTestServer(t)
	raw := []byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	_, ok, err := s.HandleBytes(context.Background(), 1, raw)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("通知不应产生响应体")
	}
}

func TestUnknownToolReturnsToolError(t *testing.T) {
	s := newTestServer(t)
	// 未知工具必须以 isError 结果返回，让模型看到提示后自行纠正
	params, _ := json.Marshal(CallToolParams{Name: "no_such_tool"})
	raw, _ := json.Marshal(Request{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call", Params: params})
	payload, _, _ := s.HandleBytes(context.Background(), 1, raw)
	var resp Response
	json.Unmarshal(payload, &resp)
	b, _ := json.Marshal(resp.Result)
	var res CallToolResult
	json.Unmarshal(b, &res)
	if !res.IsError {
		t.Error("未知工具应返回 isError=true")
	}
}

// ==================== 自然语言日期 ====================

func TestParseRange(t *testing.T) {
	now := time.Date(2026, 9, 17, 14, 30, 0, 0, time.Local) // 周四
	cases := []struct {
		expr      string
		wantStart string
		wantEnd   string
	}{
		{"今天", "2026-09-17", "2026-09-17"},
		{"昨天", "2026-09-16", "2026-09-16"},
		{"前天", "2026-09-15", "2026-09-15"},
		{"本周", "2026-09-14", "2026-09-20"}, // 周一起
		{"上周", "2026-09-07", "2026-09-13"},
		{"本月", "2026-09-01", "2026-09-30"},
		{"上月", "2026-08-01", "2026-08-31"},
		{"今年", "2026-01-01", "2026-12-31"},
		{"去年", "2025-01-01", "2025-12-31"},
		{"最近7天", "2026-09-11", "2026-09-17"},
		{"最近30天", "2026-08-19", "2026-09-17"},
		{"近3个月", "2026-07-01", "2026-09-17"},
		{"2026-01-31", "2026-01-31", "2026-01-31"},
		{"2026-02", "2026-02-01", "2026-02-28"},
		{"3天前", "2026-09-14", "2026-09-14"},
	}
	for _, c := range cases {
		s, e, ok := ParseRange(c.expr, now)
		if !ok {
			t.Errorf("%s: 解析失败", c.expr)
			continue
		}
		if s.Format("2006-01-02") != c.wantStart || e.Format("2006-01-02") != c.wantEnd {
			t.Errorf("%s: got [%s, %s], want [%s, %s]",
				c.expr, s.Format("2006-01-02"), e.Format("2006-01-02"), c.wantStart, c.wantEnd)
		}
	}
	if _, _, ok := ParseRange("下个世纪", now); ok {
		t.Error("无法识别的表达式应当返回 false")
	}
}

func TestParseDateCnNumbers(t *testing.T) {
	now := time.Date(2026, 9, 17, 0, 0, 0, 0, time.Local)
	if d, ok := ParseDate("十天前", now); !ok || d.Format("2006-01-02") != "2026-09-07" {
		t.Errorf("十天前 = %v ok=%v", d, ok)
	}
	if d, ok := ParseDate("三天前", now); !ok || d.Format("2006-01-02") != "2026-09-14" {
		t.Errorf("三天前 = %v ok=%v", d, ok)
	}
}

func TestArgFloatParsing(t *testing.T) {
	cases := []struct {
		in   any
		want float64
	}{
		{128.5, 128.5},
		{"128.5", 128.5},
		{"¥1,280.50", 1280.5},
		{"35元", 35},
		{float64(12), 12},
	}
	for _, c := range cases {
		got, ok := argFloat(map[string]any{"v": c.in}, "v")
		if !ok || got != c.want {
			t.Errorf("argFloat(%v) = %v, %v; want %v", c.in, got, ok, c.want)
		}
	}
	if _, ok := argFloat(map[string]any{"v": "abc"}, "v"); ok {
		t.Error("非法金额应返回 false")
	}
}

// ==================== 工具：查询 ====================

func TestSearchTransactions(t *testing.T) {
	s := newTestServer(t)
	uid, bookID := newTestUser(t, "u_search")
	r := newResolver(uid)
	cat, err := r.RequireCategory("餐饮", "")
	if err != nil {
		t.Fatal(err)
	}
	acc, err := r.RequireAccount("招行储蓄卡")
	if err != nil {
		t.Fatal(err)
	}
	// 三笔：本月餐饮 35.5、上月餐饮 100、本月收入 8000
	mustCreate(t, uid, dto.CreateTransactionRequest{
		BookID: bookID, Type: "expense", Amount: 35.5, CategoryID: cat, AccountID: acc,
		TxDate: dto.FlexDate{Time: time.Now()}, Description: "午餐 星巴克",
	})
	mustCreate(t, uid, dto.CreateTransactionRequest{
		BookID: bookID, Type: "expense", Amount: 100, CategoryID: cat, AccountID: acc,
		TxDate: dto.FlexDate{Time: time.Now().AddDate(0, -1, 0)}, Description: "上月聚餐",
	})
	mustCreate(t, uid, dto.CreateTransactionRequest{
		BookID: bookID, Type: "income", Amount: 8000, CategoryID: 0, AccountID: acc,
		TxDate: dto.FlexDate{Time: time.Now()}, Description: "工资",
	})

	// 按分类 + 本月
	res, err := call(t, s, uid, "search_transactions", map[string]any{
		"period": "this_month", "category": "餐饮",
	})
	if err != nil {
		t.Fatal(err)
	}
	m := mustMap(t, res)
	if m["total"].(float64) != 1 {
		t.Errorf("本月餐饮应为 1 笔，got %v", m["total"])
	}

	// 自然语言：最近30天全部
	res, err = call(t, s, uid, "search_transactions", map[string]any{"period": "recent_30d"})
	if err != nil {
		t.Fatal(err)
	}
	m = mustMap(t, res)
	if m["total"].(float64) != 2 {
		t.Errorf("最近30天应为 2 笔（本月两笔，上月那笔已过期），got %v", m["total"])
	}

	// 关键词
	res, err = call(t, s, uid, "search_transactions", map[string]any{"keyword": "星巴克"})
	if err != nil {
		t.Fatal(err)
	}
	m = mustMap(t, res)
	if m["total"].(float64) != 1 {
		t.Errorf("关键词搜索 got %v", m["total"])
	}

	// 金额区间
	res, err = call(t, s, uid, "search_transactions", map[string]any{"min_amount": 50})
	if err != nil {
		t.Fatal(err)
	}
	m = mustMap(t, res)
	if m["total"].(float64) != 2 {
		t.Errorf("min_amount=50 应命中 100 与 8000 两笔，got %v", m["total"])
	}
}

func TestSearchAmbiguousCategoryErrors(t *testing.T) {
	s := newTestServer(t)
	uid, _ := newTestUser(t, "u_amb")
	_, err := call(t, s, uid, "search_transactions", map[string]any{"category": "不存在的分类"})
	if err == nil {
		t.Fatal("不存在的分类应报错")
	}
	if !strings.Contains(err.Error(), "未找到分类") {
		t.Errorf("错误信息应给出可读提示，got: %v", err)
	}
}

// ==================== 工具：分析 ====================

func TestAnalyzeSpending(t *testing.T) {
	s := newTestServer(t)
	uid, bookID := newTestUser(t, "u_analyze")
	r := newResolver(uid)
	catFood, _ := r.RequireCategory("餐饮", "")
	catTrans, _ := r.RequireCategory("交通", "")
	acc, _ := r.RequireAccount("现金")
	now := time.Now()
	for i := 0; i < 3; i++ {
		mustCreate(t, uid, dto.CreateTransactionRequest{
			BookID: bookID, Type: "expense", Amount: 30, CategoryID: catFood, AccountID: acc,
			TxDate: dto.FlexDate{Time: now.AddDate(0, 0, -i)}, Description: "吃饭",
		})
	}
	mustCreate(t, uid, dto.CreateTransactionRequest{
		BookID: bookID, Type: "expense", Amount: 10, CategoryID: catTrans, AccountID: acc,
		TxDate: dto.FlexDate{Time: now}, Description: "地铁",
	})

	res, err := call(t, s, uid, "analyze_spending", map[string]any{
		"period": "recent_7d", "kind": "expense", "dimension": "category",
	})
	if err != nil {
		t.Fatal(err)
	}
	m := mustMap(t, res)
	summary := m["summary"].(map[string]any)
	if summary["expense"].(float64) != 100 {
		t.Errorf("支出合计应为 100，got %v", summary["expense"])
	}
	breakdown := m["breakdown"].([]any)
	if len(breakdown) != 2 {
		t.Fatalf("应有 2 个分类，got %d", len(breakdown))
	}
	first := breakdown[0].(map[string]any)
	if first["name"] != "餐饮" {
		t.Errorf("首位应为餐饮，got %v", first["name"])
	}
	if first["amount"].(float64) != 90 {
		t.Errorf("餐饮合计应为 90，got %v", first["amount"])
	}
	if first["percent"].(float64) != 90 {
		t.Errorf("餐饮占比应为 90%%，got %v", first["percent"])
	}
	if _, ok := m["comparison"]; !ok {
		t.Error("默认应带上环比对比")
	}
	if len(m["trend"].([]any)) == 0 {
		t.Error("趋势不应为空")
	}
}

func TestAnalyzeByAccountDimension(t *testing.T) {
	s := newTestServer(t)
	uid, bookID := newTestUser(t, "u_dim")
	r := newResolver(uid)
	cat, _ := r.RequireCategory("餐饮", "")
	acc, _ := r.RequireAccount("现金")
	mustCreate(t, uid, dto.CreateTransactionRequest{
		BookID: bookID, Type: "expense", Amount: 42, CategoryID: cat, AccountID: acc,
		TxDate: dto.FlexDate{Time: time.Now()}, Description: "x",
	})
	res, err := call(t, s, uid, "analyze_spending", map[string]any{
		"period": "this_month", "dimension": "account",
	})
	if err != nil {
		t.Fatal(err)
	}
	bd := mustMap(t, res)["breakdown"].([]any)
	if len(bd) == 0 || bd[0].(map[string]any)["name"] != "现金" {
		t.Errorf("账户维度应解析出「现金」，got %v", bd)
	}
}

// ==================== 工具：写入 ====================

func TestCreateTransactionDryRunThenApply(t *testing.T) {
	s := newTestServer(t)
	uid, _ := newTestUser(t, "u_create")

	args := map[string]any{
		"amount": 66.6, "type": "expense", "category": "餐饮",
		"account": "招行储蓄卡", "description": "晚饭", "date": "今天",
		"tags": []any{"出差"}, "dry_run": true,
	}
	res, err := call(t, s, uid, "create_transaction", args)
	if err != nil {
		t.Fatal(err)
	}
	m := mustMap(t, res)
	if m["applied"] != false {
		t.Error("dry_run 不应落库")
	}
	preview := m["preview"].(map[string]any)
	if preview["amount"].(float64) != 66.6 {
		t.Errorf("preview amount = %v", preview["amount"])
	}
	if preview["category"] != "餐饮" {
		t.Errorf("preview category = %v", preview["category"])
	}

	// 真正创建
	delete(args, "dry_run")
	before := countTx(t, uid)
	res, err = call(t, s, uid, "create_transaction", args)
	if err != nil {
		t.Fatal(err)
	}
	m = mustMap(t, res)
	if m["applied"] != true {
		t.Error("应已落库")
	}
	if countTx(t, uid) != before+1 {
		t.Error("交易数应 +1")
	}
	created := m["transaction"].(map[string]any)
	if created["amount"].(float64) != 66.6 {
		t.Errorf("created amount = %v", created["amount"])
	}
	tags, _ := created["tags"].([]any)
	if len(tags) != 1 || tags[0] != "出差" {
		t.Errorf("标签未正确绑定：%v", created["tags"])
	}
}

func TestCreateTransactionRejectsBadInput(t *testing.T) {
	s := newTestServer(t)
	uid, _ := newTestUser(t, "u_bad")
	if _, err := call(t, s, uid, "create_transaction", map[string]any{"amount": 0}); err == nil {
		t.Error("amount=0 应报错")
	}
	if _, err := call(t, s, uid, "create_transaction", map[string]any{"amount": 10, "type": "乱写"}); err == nil {
		t.Error("非法类型应报错")
	}
	if _, err := call(t, s, uid, "create_transaction", map[string]any{
		"amount": 10, "type": "transfer", "account": "现金",
	}); err == nil {
		t.Error("转账缺 to_account 应报错")
	}
	if _, err := call(t, s, uid, "create_transaction", map[string]any{
		"amount": 10, "category": "不存在的分类", "account": "现金",
	}); err == nil {
		t.Error("不存在的分类应报错")
	}
}

func TestUpdateAndDeleteAndRecover(t *testing.T) {
	s := newTestServer(t)
	uid, bookID := newTestUser(t, "u_udr")
	r := newResolver(uid)
	cat, _ := r.RequireCategory("餐饮", "")
	acc, _ := r.RequireAccount("现金")
	tx, cerr := handlers.CreateTx(uid, dto.CreateTransactionRequest{
		BookID: bookID, Type: "expense", Amount: 20, CategoryID: cat, AccountID: acc,
		TxDate: dto.FlexDate{Time: time.Now()}, Description: "早餐",
	})
	if cerr != nil {
		t.Fatal(cerr)
	}

	// dry_run 修改
	res, err := call(t, s, uid, "update_transaction", map[string]any{
		"id": tx.ID, "amount": 25.5, "description": "早餐+咖啡", "dry_run": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	m := mustMap(t, res)
	if m["applied"] != false {
		t.Error("dry_run 不应落库")
	}

	// 真正修改
	res, err = call(t, s, uid, "update_transaction", map[string]any{
		"id": tx.ID, "amount": 25.5, "description": "早餐+咖啡", "category": "交通",
	})
	if err != nil {
		t.Fatal(err)
	}
	m = mustMap(t, res)
	updated := m["transaction"].(map[string]any)
	if updated["amount"].(float64) != 25.5 {
		t.Errorf("amount = %v", updated["amount"])
	}
	if updated["category"] != "交通" {
		t.Errorf("category = %v", updated["category"])
	}
	if updated["description"] != "早餐+咖啡" {
		t.Errorf("description = %v", updated["description"])
	}

	// 删除（软删除）
	res, err = call(t, s, uid, "delete_transaction", map[string]any{"id": tx.ID})
	if err != nil {
		t.Fatal(err)
	}
	var alive models.Transaction
	if err := database.DB.Where("id = ?", tx.ID).First(&alive).Error; err == nil {
		t.Error("删除后不应还能查到")
	}

	// 回收站可见
	res, err = call(t, s, uid, "search_transactions", map[string]any{"include_deleted": true})
	if err != nil {
		t.Fatal(err)
	}
	if mustMap(t, res)["total"].(float64) != 1 {
		t.Error("回收站应有 1 条")
	}

	// 恢复
	if _, err := call(t, s, uid, "recover_transaction", map[string]any{"id": tx.ID}); err != nil {
		t.Fatal(err)
	}
	if err := database.DB.Where("id = ?", tx.ID).First(&alive).Error; err != nil {
		t.Error("恢复后应能查到")
	}
}

func TestBatchDeleteRequiresConfirm(t *testing.T) {
	s := newTestServer(t)
	uid, bookID := newTestUser(t, "u_batch")
	r := newResolver(uid)
	cat, _ := r.RequireCategory("餐饮", "")
	acc, _ := r.RequireAccount("现金")
	ids := make([]any, 0, 2)
	for i := 0; i < 2; i++ {
		tx, cerr := handlers.CreateTx(uid, dto.CreateTransactionRequest{
			BookID: bookID, Type: "expense", Amount: 10, CategoryID: cat, AccountID: acc,
			TxDate: dto.FlexDate{Time: time.Now()},
		})
		if cerr != nil {
			t.Fatal(cerr)
		}
		ids = append(ids, float64(tx.ID))
	}

	// 未确认 → 拒绝
	if _, err := call(t, s, uid, "batch_delete_transactions", map[string]any{"ids": ids}); err == nil {
		t.Fatal("未传 confirm 应拒绝执行")
	}
	// 预演
	res, err := call(t, s, uid, "batch_delete_transactions", map[string]any{"ids": ids, "dry_run": true})
	if err != nil {
		t.Fatal(err)
	}
	if mustMap(t, res)["count"].(float64) != 2 {
		t.Error("预演应列出 2 笔")
	}
	// 确认执行
	res, err = call(t, s, uid, "batch_delete_transactions", map[string]any{"ids": ids, "confirm": true})
	if err != nil {
		t.Fatal(err)
	}
	if mustMap(t, res)["deleted_count"].(float64) != 2 {
		t.Error("应删除 2 笔")
	}
}

// ==================== 工具：字典 ====================

func TestDictionaryTools(t *testing.T) {
	s := newTestServer(t)
	uid, _ := newTestUser(t, "u_dict")
	if res, err := call(t, s, uid, "list_books", nil); err != nil {
		t.Fatal(err)
	} else if len(mustMap(t, res)["books"].([]any)) == 0 {
		t.Error("books 不应为空")
	}
	if res, err := call(t, s, uid, "list_categories", map[string]any{"kind": "expense"}); err != nil {
		t.Fatal(err)
	} else if len(mustMap(t, res)["categories"].([]any)) != 4 {
		t.Errorf("expense 分类应 4 个，got %v", mustMap(t, res)["categories"])
	}
	if res, err := call(t, s, uid, "list_accounts", nil); err != nil {
		t.Fatal(err)
	} else if len(mustMap(t, res)["accounts"].([]any)) != 2 {
		t.Error("账户应 2 个")
	}
	if res, err := call(t, s, uid, "list_tags", nil); err != nil {
		t.Fatal(err)
	} else if len(mustMap(t, res)["tags"].([]any)) != 2 {
		t.Error("标签应 2 个")
	}
}

func TestBudgetStatus(t *testing.T) {
	s := newTestServer(t)
	uid, bookID := newTestUser(t, "u_budget")
	database.DB.Create(&models.Budget{
		UserID: uid, BookID: bookID, PeriodType: "monthly", CategoryID: 0,
		Amount: models.FromYuan(1000), UsedAmount: models.FromYuan(800),
		StartDate: time.Now().AddDate(0, 0, -5), EndDate: time.Now().AddDate(0, 0, 25),
		AlertRate: 0.8,
	})
	res, err := call(t, s, uid, "get_budget_status", nil)
	if err != nil {
		t.Fatal(err)
	}
	budgets := mustMap(t, res)["budgets"].([]any)
	if len(budgets) != 1 {
		t.Fatalf("预算应 1 条，got %d", len(budgets))
	}
	b := budgets[0].(map[string]any)
	if b["usage_rate"].(float64) != 80 {
		t.Errorf("使用率应为 80，got %v", b["usage_rate"])
	}
	if b["name"] != "总预算" {
		t.Errorf("名称应为总预算，got %v", b["name"])
	}
}

// ==================== 辅助 ====================

func mustCreate(t *testing.T, uid uint, req dto.CreateTransactionRequest) models.Transaction {
	t.Helper()
	tx, cerr := handlers.CreateTx(uid, req)
	if cerr != nil {
		t.Fatalf("CreateTx: %s", cerr.Message)
	}
	return tx
}

func countTx(t *testing.T, uid uint) int64 {
	t.Helper()
	var n int64
	database.DB.Model(&models.Transaction{}).Where("user_id = ?", uid).Count(&n)
	return n
}
