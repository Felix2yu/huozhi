package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"huozhi/internal/models"
)

// ===== 密钥 scope 对 MCP 的影响 =====
//
// 需求：MCP 必须保留写能力（AI 客户端要能记账），但牵丝、幕间这类消费方
// 只该取数。两者共用同一端点，唯一可区分的依据就是密钥自身的 scope。
// 这里锁住三件事：只读密钥看不到写工具、调不动写工具、缺省按只读处理。

// rpc 走完整分发链路并返回原始结果。
func rpc(t *testing.T, s *Server, uid uint, scope string, method string, params any) Response {
	t.Helper()
	var rawParams json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			t.Fatal(err)
		}
		rawParams = b
	}
	raw, err := json.Marshal(Request{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: method, Params: rawParams})
	if err != nil {
		t.Fatal(err)
	}
	payload, ok, err := s.HandleBytes(WithScope(context.Background(), scope), uid, raw)
	if err != nil || !ok {
		t.Fatalf("HandleBytes: ok=%v err=%v", ok, err)
	}
	var resp Response
	if err := json.Unmarshal(payload, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("rpc error: %+v", resp.Error)
	}
	return resp
}

// listTools 取 tools/list 里的工具名。
func listTools(t *testing.T, s *Server, uid uint, scope string) map[string]bool {
	t.Helper()
	resp := rpc(t, s, uid, scope, "tools/list", nil)
	b, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatal(err)
	}
	var out ListToolsResult
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	names := make(map[string]bool, len(out.Tools))
	for _, tl := range out.Tools {
		names[tl.Name] = true
	}
	return names
}

// callToolErr 调工具并返回 (是否工具级错误, 错误文本)。
func callToolErr(t *testing.T, s *Server, uid uint, scope, name string, args map[string]any) (bool, string) {
	t.Helper()
	resp := rpc(t, s, uid, scope, "tools/call", CallToolParams{Name: name, Arguments: args})
	b, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatal(err)
	}
	var res CallToolResult
	if err := json.Unmarshal(b, &res); err != nil {
		t.Fatal(err)
	}
	text := ""
	for _, c := range res.Content {
		text += c.Text
	}
	return res.IsError, text
}

func TestScopeReadOnlyHidesWriteTools(t *testing.T) {
	s := newTestServer(t)
	uid, _ := newTestUser(t, "scope_list")

	readTools := listTools(t, s, uid, models.ScopeRead)
	writeTools := listTools(t, s, uid, models.ScopeWrite)

	// 写工具：只读清单里不能出现，读写清单里必须有
	for _, name := range []string{"create_transaction", "update_transaction", "delete_transaction", "batch_delete_transactions"} {
		if readTools[name] {
			t.Fatalf("只读密钥的 tools/list 不应包含写工具 %q", name)
		}
		if !writeTools[name] {
			t.Fatalf("读写密钥的 tools/list 应包含写工具 %q", name)
		}
	}
	// 只读工具两种密钥都能看到——分权只针对写
	for _, name := range []string{"search_transactions", "get_transaction", "list_books", "analyze_spending"} {
		if !readTools[name] || !writeTools[name] {
			t.Fatalf("只读工具 %q 应对两种密钥都可见（read=%v write=%v）", name, readTools[name], writeTools[name])
		}
	}
	if len(readTools) >= len(writeTools) {
		t.Fatalf("只读清单应少于读写清单：read=%d write=%d", len(readTools), len(writeTools))
	}
}

func TestScopeReadOnlyCannotWrite(t *testing.T) {
	s := newTestServer(t)
	uid, _ := newTestUser(t, "scope_gate")

	// 即使客户端硬调（工具没列出来也不代表它不会试），也必须被拦下
	isErr, text := callToolErr(t, s, uid, models.ScopeRead, "create_transaction", map[string]any{
		"type": "expense", "amount": 10,
	})
	if !isErr {
		t.Fatal("只读密钥调写工具应返回工具错误")
	}
	if !strings.Contains(text, "只读") {
		t.Fatalf("错误信息应说明权限不足，实际: %s", text)
	}

	// 只读工具照常可用
	isErr, text = callToolErr(t, s, uid, models.ScopeRead, "search_transactions", map[string]any{"limit": 5})
	if isErr {
		t.Fatalf("只读密钥应能用读工具，实际报错: %s", text)
	}
}

// 缺省（未注入 scope）按只读处理：这是刻意选择的失败方向——
// 新增传输层忘记注入时，代价是写工具不可用，而不是悄悄拿到删账权限。
func TestScopeDefaultsToReadOnly(t *testing.T) {
	if got := scopeFrom(context.Background()); got != models.ScopeRead {
		t.Fatalf("缺省 scope = %q, want read", got)
	}
	if canWrite(context.Background()) {
		t.Fatal("缺省不应具备写权限")
	}
	if canWrite(WithScope(context.Background(), "乱写")) {
		t.Fatal("未知 scope 应按只读处理")
	}
	// 未知 scope 不能在归一化时意外升权
	if got := scopeFrom(WithScope(context.Background(), "READ")); got != models.ScopeRead {
		t.Fatalf("大小写不匹配应归为只读，实际 %q", got)
	}
}

// 写工具判定以 ReadOnlyHint 为准，注解缺失按写处理（失败关闭）。
func TestToolRequiresWrite(t *testing.T) {
	readTool := Tool{Name: "x", Annotations: &ToolAnnotations{ReadOnlyHint: true}}
	writeTool := Tool{Name: "y", Annotations: &ToolAnnotations{ReadOnlyHint: false}}
	unknown := Tool{Name: "z"}

	if toolRequiresWrite(readTool) {
		t.Fatal("只读标注的工具不应要求写权限")
	}
	if !toolRequiresWrite(writeTool) {
		t.Fatal("非只读标注的工具应要求写权限")
	}
	if !toolRequiresWrite(unknown) {
		t.Fatal("缺注解的工具应按写处理（失败关闭）")
	}
}
