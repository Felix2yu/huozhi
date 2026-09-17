package dto

import (
	"database/sql/driver"
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestFlexDateUnmarshalJSONDate(t *testing.T) {
	var d FlexDate
	if err := json.Unmarshal([]byte(`"2026-03-15"`), &d); err != nil {
		t.Fatal(err)
	}
	if d.Time.Format("2006-01-02") != "2026-03-15" {
		t.Fatalf("got %s", d.Time)
	}
}

func TestFlexDateUnmarshalJSONRFC(t *testing.T) {
	var d FlexDate
	if err := json.Unmarshal([]byte(`"2026-03-15T10:20:30Z"`), &d); err != nil {
		t.Fatal(err)
	}
	if d.Time.Year() != 2026 || d.Time.Hour() != 10 {
		t.Fatalf("got %v", d.Time)
	}
}

func TestFlexDateUnmarshalJSONSlash(t *testing.T) {
	var d FlexDate
	if err := json.Unmarshal([]byte(`"2026/03/15"`), &d); err != nil {
		t.Fatal(err)
	}
	if d.Time.Format("2006-01-02") != "2026-03-15" {
		t.Fatalf("got %s", d.Time)
	}
}

func TestFlexDateUnmarshalJSONNullString(t *testing.T) {
	var d FlexDate
	if err := json.Unmarshal([]byte(`"null"`), &d); err != nil {
		t.Fatal(err)
	}
	if !d.Time.IsZero() {
		t.Fatal("expected zero time")
	}
}

func TestFlexDateUnmarshalInvalid(t *testing.T) {
	var d FlexDate
	// number is not a time -> parseString fails
	if err := json.Unmarshal([]byte(`12345`), &d); err == nil {
		t.Fatalf("expected error, got %v", d.Time)
	}
}

func TestFlexDateMarshalNull(t *testing.T) {
	d := FlexDate{}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "null" {
		t.Fatalf("got %s", b)
	}
}

func TestFlexDateMarshalDate(t *testing.T) {
	d := FlexDate{Time: time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)}
	b, _ := json.Marshal(d)
	if string(b) != `"2026-03-15"` {
		t.Fatalf("got %s", b)
	}
}

func TestFlexDateUnmarshalParam(t *testing.T) {
	var d FlexDate
	if err := d.UnmarshalParam("2026-03-15"); err != nil {
		t.Fatal(err)
	}
	if d.Time.Format("2006-01-02") != "2026-03-15" {
		t.Fatal("param parse failed")
	}
}

func TestFlexDateString(t *testing.T) {
	d := FlexDate{Time: time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)}
	if d.String() != "2026-03-15" {
		t.Fatal("String failed")
	}
	var z FlexDate
	if z.String() != "" {
		t.Fatal("zero String should be empty")
	}
	if !z.T().IsZero() {
		t.Fatal("T should be zero")
	}
}

func TestFlexDateParseStringEmpty(t *testing.T) {
	var d FlexDate
	if err := d.UnmarshalParam(""); err != nil {
		t.Fatal(err)
	}
	if !d.Time.IsZero() {
		t.Fatal("empty should be zero")
	}
	if err := d.UnmarshalParam("null"); err != nil {
		t.Fatal(err)
	}
}

func TestFlexDateParseStringFallback(t *testing.T) {
	// len > 10, first formats fail, s[:10] is a valid date
	var d FlexDate
	if err := d.UnmarshalParam("2026-03-15extra"); err != nil {
		t.Fatal(err)
	}
	if d.Time.Format("2006-01-02") != "2026-03-15" {
		t.Fatal("fallback parse failed")
	}
}

func TestFlexDateParseStringInvalid(t *testing.T) {
	var d FlexDate
	if err := d.UnmarshalParam("notadate"); err == nil {
		t.Fatal("expected error")
	}
}

func TestFlexDateSQLValue(t *testing.T) {
	for _, tt := range []struct {
		name string
		date FlexDate
		want driver.Value
	}{
		{"零值", FlexDate{}, nil},
		{"日期", FlexDate{Time: time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)}, time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)},
		{"带时区时间", FlexDate{Time: time.Date(2026, 9, 17, 23, 59, 58, 123, time.FixedZone("UTC+8", 8*3600))}, time.Date(2026, 9, 17, 23, 59, 58, 123, time.FixedZone("UTC+8", 8*3600))},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.date.Value()
			if err != nil || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("SQL值 = %v，期望 %v，错误 %v", got, tt.want, err)
			}
			converted, err := driver.DefaultParameterConverter.ConvertValue(tt.date)
			if err != nil || !reflect.DeepEqual(converted, tt.want) {
				t.Fatalf("SQL参数转换 = %v，期望 %v，错误 %v", converted, tt.want, err)
			}
		})
	}
}

func TestFlexDateParamFormats(t *testing.T) {
	for _, input := range []string{
		"2026-09-17T12:34:56",
		"2026-09-17 12:34:56",
		"2026/09/17 12:34:56",
		" 2026-09-17T12:34:56Z ",
	} {
		t.Run(input, func(t *testing.T) {
			var date FlexDate
			if err := date.UnmarshalParam(input); err != nil {
				t.Fatal(err)
			}
			want := time.Date(2026, 9, 17, 12, 34, 56, 0, time.UTC)
			if !date.T().Equal(want) {
				t.Fatalf("日期 = %v，期望 %v", date.T(), want)
			}
		})
	}
	for _, input := range []string{"2026-02-30", "2026-13-17", "不是有效日期"} {
		t.Run(input, func(t *testing.T) {
			original := time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)
			date := FlexDate{Time: original}
			if err := date.UnmarshalParam(input); err == nil {
				t.Fatal("非法日期应返回错误")
			}
			if !date.T().Equal(original) {
				t.Fatal("解析失败不应修改已有日期")
			}
		})
	}
}

func TestInviteMemberWho(t *testing.T) {
	for _, tt := range []struct {
		name string
		req  InviteMemberRequest
		want string
	}{
		{"标识优先", InviteMemberRequest{Identifier: "指定成员", Username: "用户名", Email: "member@example.test"}, "指定成员"},
		{"用户名回退", InviteMemberRequest{Username: "用户名", Email: "member@example.test"}, "用户名"},
		{"邮箱回退", InviteMemberRequest{Email: "member@example.test"}, "member@example.test"},
		{"空请求", InviteMemberRequest{}, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.req.Who(); got != tt.want {
				t.Fatalf("成员标识 = %q，期望 %q", got, tt.want)
			}
		})
	}
}

func TestCreateTransactionFlags(t *testing.T) {
	for _, tt := range []struct {
		input       string
		wantBalance bool
		wantBudget  bool
	}{
		{`{}`, true, true},
		{`{"include_in_balance":null,"include_in_budget":null}`, true, true},
		{`{"include_in_balance":false}`, false, true},
		{`{"include_in_budget":false}`, true, false},
		{`{"include_in_balance":true,"include_in_budget":true}`, true, true},
		{`{"include_in_balance":false,"include_in_budget":false}`, false, false},
	} {
		t.Run(tt.input, func(t *testing.T) {
			var req CreateTransactionRequest
			if err := json.Unmarshal([]byte(tt.input), &req); err != nil {
				t.Fatal(err)
			}
			if got := req.BalanceFlag(); got != tt.wantBalance {
				t.Fatalf("余额标记 = %v，期望 %v", got, tt.wantBalance)
			}
			if got := req.BudgetFlag(); got != tt.wantBudget {
				t.Fatalf("预算标记 = %v，期望 %v", got, tt.wantBudget)
			}
		})
	}
}

func TestUpdateTransactionTags(t *testing.T) {
	for _, tt := range []struct {
		input string
		want  []uint
		has   bool
	}{
		{`{}`, nil, false},
		{`{"tag_ids":[]}`, []uint{}, true},
		{`{"tag_ids":[2,7]}`, []uint{2, 7}, true},
	} {
		t.Run(tt.input, func(t *testing.T) {
			var req UpdateTransactionRequest
			if err := json.Unmarshal([]byte(tt.input), &req); err != nil {
				t.Fatal(err)
			}
			if got := req.HasTagIDs(); got != tt.has {
				t.Fatalf("标签提交标记 = %v，期望 %v", got, tt.has)
			}
			if got := req.TagIDsOrNil(); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("标签 = %#v，期望 %#v", got, tt.want)
			}
		})
	}
}

func TestNormalizedReimburseStatus(t *testing.T) {
	for _, tt := range []struct {
		input string
		old   string
		want  string
	}{
		{`{}`, "", "none"},
		{`{}`, "pending", "pending"},
		{`{"reimburse_status":null}`, "done", "done"},
		{`{"reimburse_status":""}`, "done", "none"},
		{`{"reimburse_status":"none"}`, "pending", "none"},
		{`{"reimburse_status":"pending"}`, "none", "pending"},
		{`{"reimburse_status":"done"}`, "pending", "done"},
	} {
		t.Run(tt.input+tt.old, func(t *testing.T) {
			var req UpdateTransactionRequest
			if err := json.Unmarshal([]byte(tt.input), &req); err != nil {
				t.Fatal(err)
			}
			if got := req.NormalizedReimburseStatus(tt.old); got != tt.want {
				t.Fatalf("报销状态 = %q，期望 %q", got, tt.want)
			}
		})
	}
}

func TestQueryTransactionUseCursor(t *testing.T) {
	date := FlexDate{Time: time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)}
	for _, tt := range []struct {
		name string
		req  QueryTransactionRequest
		want bool
	}{
		{"无游标", QueryTransactionRequest{}, false},
		{"仅ID", QueryTransactionRequest{CursorID: 1}, false},
		{"仅日期", QueryTransactionRequest{CursorDate: date}, false},
		{"完整游标", QueryTransactionRequest{CursorID: 1, CursorDate: date}, true},
		{"游标优先于页码", QueryTransactionRequest{CursorID: 1, CursorDate: date, Page: 3}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.req.UseCursor(); got != tt.want {
				t.Fatalf("游标启用 = %v，期望 %v", got, tt.want)
			}
		})
	}
}

func TestUpdateTransactionPartialJSON(t *testing.T) {
	description := "只改备注"
	var req UpdateTransactionRequest
	if err := json.Unmarshal([]byte(`{"description":"只改备注"}`), &req); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(req, UpdateTransactionRequest{Description: &description}) {
		t.Fatalf("局部更新不应填充未提交字段: %+v", req)
	}
	if err := json.Unmarshal([]byte(`{"transfer_fee":0,"refund_of_id":0,"images":[],"include_in_balance":false,"include_in_budget":false}`), &req); err != nil {
		t.Fatal(err)
	}
	if req.TransferFee == nil || *req.TransferFee != 0 || req.RefundOfID == nil || *req.RefundOfID != 0 {
		t.Fatal("显式零值应保留为非空指针")
	}
	if req.Images == nil || !reflect.DeepEqual(*req.Images, []string{}) {
		t.Fatal("显式空图片数组应支持清空")
	}
	if req.IncludeInBalance == nil || *req.IncludeInBalance || req.IncludeInBudget == nil || *req.IncludeInBudget {
		t.Fatal("显式false不能退化为未提交")
	}
}

func TestUpdateUserPartialJSON(t *testing.T) {
	var req UpdateUserRequest
	if err := json.Unmarshal([]byte(`{"auto_backup_enabled":false}`), &req); err != nil {
		t.Fatal(err)
	}
	disabled := false
	if !reflect.DeepEqual(req, UpdateUserRequest{AutoBackupEnabled: &disabled}) {
		t.Fatalf("只更新备份不应修改其他资料: %+v", req)
	}
	var fxReq UpdateUserRequest
	if err := json.Unmarshal([]byte(`{"nickname":"","fx_auto_refresh":false,"fx_refresh_hours":0}`), &fxReq); err != nil {
		t.Fatal(err)
	}
	empty, zero := "", 0
	want := UpdateUserRequest{Nickname: &empty, FxAutoRefresh: &disabled, FxRefreshHours: &zero}
	if !reflect.DeepEqual(fxReq, want) {
		t.Fatalf("显式空值、false和零值应保留: %+v", fxReq)
	}
}
