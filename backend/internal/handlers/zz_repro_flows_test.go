package handlers_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"huozhi/internal/database"
	"huozhi/internal/handlers"
	"huozhi/internal/models"
)

// 复核用复现测试：
//  1. 报销单未关联交易时，transaction_ids 序列化成 null → 前端 item.transaction_ids.length 崩溃
//  2. 分期创建时前端提交的 monthly_amount 被后端忽略
func TestRepro_ReimbursementNullTransactionIDs(t *testing.T) {
	uid, tok := newUser(t)
	bookID := seedBook(t, uid)

	w := do(authReq("POST", "/api/reimbursements", tok, map[string]interface{}{
		"book_id":      bookID,
		"name":         "差旅费",
		"total_amount": 1000,
	}))
	if w.Code != 201 {
		t.Fatalf("创建报销单失败 %d %s", w.Code, w.Body.String())
	}

	w2 := do(authReq("GET", "/api/reimbursements", tok, nil))
	if w2.Code != http.StatusOK {
		t.Fatalf("列表失败 %d %s", w2.Code, w2.Body.String())
	}
	m := decode(t, w2)
	list := m["data"].([]interface{})
	if len(list) == 0 {
		t.Fatal("列表为空")
	}
	item := list[0].(map[string]interface{})
	raw, _ := json.Marshal(item)
	t.Logf("报销单返回: %s", string(raw))

	if item["transaction_ids"] == nil {
		t.Errorf("transaction_ids 为 null → 前端 `item.transaction_ids.length` 会抛 TypeError，报销列表页崩溃")
	}
}

func TestRepro_InstallmentIgnoresMonthlyAmount(t *testing.T) {
	uid, tok := newUser(t)
	bookID := seedBook(t, uid)
	accID := seedAccount(t, uid, bookID)
	catID := seedCategory(t, uid, bookID, "expense", "分期")

	w := do(authReq("POST", "/api/installments", tok, map[string]interface{}{
		"book_id":          bookID,
		"name":             "手机分期",
		"total_amount":     6000,
		"total_months":     12,
		"monthly_amount":   555.55, // 用户手工指定的月供
		"interest_amount":  0,
		"category_id":      catID,
		"account_id":       accID,
		"first_repay_date": "2026-10-01",
	}))
	if w.Code != 201 {
		t.Fatalf("创建分期失败 %d %s", w.Code, w.Body.String())
	}
	m := decode(t, w)
	ins := m["data"].(map[string]interface{})
	t.Logf("分期返回 monthly_amount=%v (前端提交 555.55)", ins["monthly_amount"])
	if got, _ := ins["monthly_amount"].(float64); got != 555.55 {
		t.Errorf("月供被后端覆盖: got=%v want=555.55（前端「月供金额」输入框形同虚设）", got)
	}
}

// 分期后台任务：服务停机后补录的分期，应在一轮内补齐所有逾期期次
func TestRepro_InstallmentCatchUp(t *testing.T) {
	uid, tok := newUser(t)
	bookID := seedBook(t, uid)
	accID := seedAccount(t, uid, bookID)
	catID := seedCategory(t, uid, bookID, "expense", "分期")

	// 首次还款日 = 3 个月前 + 2 天 → 第 1/2/3 期已到期，第 4 期仍在未来
	first := time.Now().AddDate(0, -3, 0).AddDate(0, 0, 2).Format("2006-01-02")
	w := do(authReq("POST", "/api/installments", tok, map[string]interface{}{
		"book_id": bookID, "name": "补课分期", "total_amount": 3000,
		"total_months": 6, "interest_amount": 0,
		"category_id": catID, "account_id": accID, "first_repay_date": first,
	}))
	if w.Code != 201 {
		t.Fatalf("创建分期失败 %d %s", w.Code, w.Body.String())
	}

	n := handlers.RunInstallmentRepayments(time.Now())
	t.Logf("本轮生成还款笔数=%d", n)
	if n != 3 {
		t.Errorf("逾期 3 期只生成 %d 笔，应为 3 笔（每轮只生成一期会导致进度长期落后）", n)
	}

	var ins models.Installment
	database.DB.Where("user_id = ? AND name = ?", uid, "补课分期").First(&ins)
	if ins.PaidMonths != 3 {
		t.Errorf("已还期数=%d，应为 3", ins.PaidMonths)
	}
	if ins.Status != "active" {
		t.Errorf("状态=%s，6 期只还 3 期应仍为 active", ins.Status)
	}
	// 未到第 4 期，不应被继续生成
	if n2 := handlers.RunInstallmentRepayments(time.Now()); n2 != 0 {
		t.Errorf("重复执行又生成了 %d 笔，幂等失效", n2)
	}
}

// 报销入账：同一账本 + 同一账户 + 相同金额的两张报销单，必须各自生成收款交易
func TestRepro_ReimbursementPayoutNotLost(t *testing.T) {
	uid, tok := newUser(t)
	bookID := seedBook(t, uid)
	accID := seedAccount(t, uid, bookID)

	var ids []uint
	for i := 0; i < 2; i++ {
		w := do(authReq("POST", "/api/reimbursements", tok, map[string]interface{}{
			"book_id": bookID, "name": "同额报销", "total_amount": 500,
		}))
		if w.Code != 201 {
			t.Fatalf("创建报销单失败 %d %s", w.Code, w.Body.String())
		}
		ids = append(ids, uint(decode(t, w)["data"].(map[string]interface{})["id"].(float64)))
	}

	for _, id := range ids {
		w := do(authReq("PUT", "/api/reimbursements/"+itoa(id), tok, map[string]interface{}{
			"status": "received", "received_amount": 500, "account_id": accID,
			"received_date": time.Now().Format("2006-01-02"),
		}))
		if w.Code != http.StatusOK {
			t.Fatalf("更新报销单失败 %d %s", w.Code, w.Body.String())
		}
	}

	var cnt int64
	database.DB.Model(&models.Transaction{}).
		Where("user_id = ? AND related_type = ?", uid, models.RelatedReimburseReceived).
		Count(&cnt)
	if cnt != 2 {
		t.Errorf("收款交易数=%d，应为 2（第二张同额报销单的钱会凭空消失）", cnt)
	}

	// 部分到账后再收齐：只补差额，不能重复入账
	var bal models.Account
	database.DB.First(&bal, accID)
	before := bal.Balance
	w := do(authReq("PUT", "/api/reimbursements/"+itoa(ids[0]), tok, map[string]interface{}{
		"status": "received", "received_amount": 500, "account_id": accID,
		"received_date": time.Now().Format("2006-01-02"),
	}))
	_ = w
	database.DB.First(&bal, accID)
	if bal.Balance != before {
		t.Errorf("重复提交导致余额变化: %v → %v，幂等失效", before, bal.Balance)
	}
}

// 周期记账：首次执行时间不得早于 start_date
func TestRepro_RecurringFirstRun(t *testing.T) {
	uid, tok := newUser(t)
	bookID := seedBook(t, uid)
	accID := seedAccount(t, uid, bookID)
	catID := seedCategory(t, uid, bookID, "expense", "房租")

	// 每月 25 号，起始日设为「本月 1 号」→ 首次执行必须是本月 25 号
	start := time.Now().AddDate(0, 0, -time.Now().Day()+1).Format("2006-01-02")
	w := do(authReq("POST", "/api/recurring", tok, map[string]interface{}{
		"book_id": bookID, "name": "房租", "type": "expense", "amount": 3000,
		"category_id": catID, "account_id": accID,
		"recurring_type": "monthly", "month_day": 25, "start_date": start,
	}))
	if w.Code != 201 {
		t.Fatalf("创建周期任务失败 %d %s", w.Code, w.Body.String())
	}
	d := decode(t, w)["data"].(map[string]interface{})
	next := d["next_run_at"].(string)
	t.Logf("start=%s next_run_at=%s", start, next)
	if next[:10] != nextMonthDay25(t, start) {
		t.Errorf("首次执行时间错误: next=%s，应为当月（或顺延下月）25 号", next)
	}
}

// nextMonthDay25 计算 start 所在月（或次月）的 25 号
func nextMonthDay25(t *testing.T, start string) string {
	t.Helper()
	sd, err := time.Parse("2006-01-02", start)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(sd.Year(), sd.Month(), 25, 0, 0, 0, 0, sd.Location())
	if want.Day() < sd.Day() || want.Before(sd) {
		want = want.AddDate(0, 1, 0)
	}
	return want.Format("2006-01-02")
}
