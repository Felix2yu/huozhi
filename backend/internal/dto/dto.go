package dto

import (
	"database/sql/driver"
	"encoding/json"
	"strings"
	"time"
)

// FlexDate 兼容多种日期格式的类型
// 支持: "2006-01-02", "2006-01-02T15:04:05Z07:00", "2006-01-02T15:04:05.000Z"
// 时间部分会被忽略（截断到日期当日 00:00:00）
type FlexDate struct {
	time.Time
}

func (d *FlexDate) UnmarshalJSON(data []byte) error {
	// 先尝试标准方式
	var t time.Time
	if err := json.Unmarshal(data, &t); err == nil {
		d.Time = t
		return nil
	}
	// 再尝试字符串
	s := strings.Trim(string(data), `"`)
	return d.parseString(s)
}

func (d *FlexDate) parseString(s string) error {
	s = strings.TrimSpace(s)
	if s == "" || s == "null" {
		d.Time = time.Time{}
		return nil
	}
	// 按优先级尝试多种格式
	formats := []string{
		"2006-01-02",
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006/01/02",
		"2006/01/02 15:04:05",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			d.Time = t
			return nil
		}
	}
	// 最后兜底：只取前 10 个字符按 "2006-01-02" 解析
	if len(s) >= 10 {
		if t, err := time.Parse("2006-01-02", s[:10]); err == nil {
			d.Time = t
			return nil
		}
	}
	return &time.ParseError{Layout: "2006-01-02|2006-01-02T15:04:05Z07:00", Value: s}
}

// UnmarshalQuery 兼容 form/query 绑定
func (d *FlexDate) UnmarshalParam(param string) error {
	return d.parseString(param)
}

// MarshalJSON 序列化为 "YYYY-MM-DD"（纯日期）
func (d FlexDate) MarshalJSON() ([]byte, error) {
	if d.Time.IsZero() {
		return []byte("null"), nil
	}
	return []byte(`"` + d.Time.Format("2006-01-02") + `"`), nil
}

// String 方便日志和调试
// T 取出底层 time.Time
func (d FlexDate) T() time.Time { return d.Time }

// Value 实现 driver.Valuer，使 FlexDate 能直接作为查询参数使用
// （例如 db.Where("tx_date >= ?", req.StartDate)）。否则 SQL 驱动无法把
// 这个 Embed time.Time 的结构体转换为参数，导致带日期筛选的查询静默失败、
// 返回空结果（账单流水页为空、收入支出结余为 0）。
func (d FlexDate) Value() (driver.Value, error) {
	if d.Time.IsZero() {
		return nil, nil
	}
	return d.Time, nil
}

func (d FlexDate) String() string {
	if d.Time.IsZero() {
		return ""
	}
	return d.Time.Format("2006-01-02")
}

// ====== 通用 ======

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

type Pagination struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

type PagedResponse struct {
	List       interface{} `json:"list"`
	Pagination Pagination  `json:"pagination"`
}

type IDRequest struct {
	ID uint `uri:"id" binding:"required"`
}

// ====== 认证 ======

type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Email    string `json:"email" binding:"omitempty,email"`
	Phone    string `json:"phone" binding:"omitempty,min=6,max=20"`
	Password string `json:"password" binding:"required,min=6,max=128"`
	Nickname string `json:"nickname" binding:"omitempty,max=50"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token    string      `json:"token"`
	ExpireIn int         `json:"expire_in"` // 秒
	User     interface{} `json:"user"`
}

type UpdateUserRequest struct {
	Nickname   string `json:"nickname" binding:"omitempty,max=50"`
	Avatar     string `json:"avatar" binding:"omitempty,max=255"`
	Email      string `json:"email" binding:"omitempty,email"`
	Phone      string `json:"phone" binding:"omitempty,max=20"`
	Locale     string `json:"locale" binding:"omitempty,oneof=zh-CN en"`
	Timezone   string `json:"timezone" binding:"omitempty,max=50"`
	MonthStart int    `json:"month_start" binding:"omitempty,min=1,max=28"`
	Currency   string `json:"currency" binding:"omitempty,max=10"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6,max=128"`
}

// ====== 账本 ======

type CreateBookRequest struct {
	Name        string `json:"name" binding:"required,max=100"`
	Icon        string `json:"icon" binding:"omitempty,max=50"`
	Color       string `json:"color" binding:"omitempty,max=20"`
	Description string `json:"description" binding:"omitempty,max=500"`
	Currency    string `json:"currency" binding:"omitempty,max=10"`
	IsDefault   bool   `json:"is_default"`
	Sort        int    `json:"sort"`
}

type UpdateBookRequest = CreateBookRequest

type InviteMemberRequest struct {
	Username string `json:"username" binding:"required"`
	Role     string `json:"role" binding:"required,oneof=editor viewer"`
}

// ====== 账户 ======

type CreateAccountRequest struct {
	BookID          uint    `json:"book_id"`
	Name            string  `json:"name" binding:"required,max=100"`
	Type            string  `json:"type" binding:"required,oneof=cash bank credit prepaid investment liability virtual"`
	Currency        string  `json:"currency" binding:"omitempty,max=10"`
	InitialAmount   float64 `json:"initial_amount"`
	Icon            string  `json:"icon" binding:"omitempty,max=50"`
	Color           string  `json:"color" binding:"omitempty,max=20"`
	BankName        string  `json:"bank_name" binding:"omitempty,max=100"`
	CardNo4         string  `json:"card_no4" binding:"omitempty,max=10"`
	FullCardNo      string  `json:"full_card_no" binding:"omitempty,max=32"` // 完整卡号，后端加密存
	CreditLimit     float64 `json:"credit_limit"`
	BillDay         int     `json:"bill_day" binding:"omitempty,min=1,max=31"`
	RepayDay        int     `json:"repay_day" binding:"omitempty,min=1,max=31"`
	ExpireMonth     int     `json:"expire_month" binding:"omitempty,min=1,max=12"`
	ExpireYear      int     `json:"expire_year" binding:"omitempty,min=0,max=99"`
	CVV             string  `json:"cvv" binding:"omitempty,max=10"` // CVV2/CVC2，后端加密存
	APR             float64 `json:"apr"`
	IncludeInTotal  bool    `json:"include_in_total"`
	IncludeInBudget bool    `json:"include_in_budget"`
	IsHidden        bool    `json:"is_hidden"`
	GroupID         uint    `json:"group_id"`
	Sort            int     `json:"sort"`
	Remark          string  `json:"remark" binding:"omitempty,max=500"`
}

type UpdateAccountRequest = CreateAccountRequest

type AdjustAccountRequest struct {
	Amount      float64   `json:"amount" binding:"required"`
	Description string    `json:"description" binding:"omitempty,max=500"`
	Date        FlexDate `json:"date" binding:"required"`
}

// ====== 分类 ======

type CreateCategoryRequest struct {
	BookID     uint   `json:"book_id"`
	ParentID   uint   `json:"parent_id"`
	Name       string `json:"name" binding:"required,max=50"`
	Kind       string `json:"kind" binding:"required,oneof=expense income"`
	Icon       string `json:"icon" binding:"omitempty,max=50"`
	Color      string `json:"color" binding:"omitempty,max=20"`
	Sort       int    `json:"sort"`
	NeedTag    bool   `json:"need_tag"`
}

type UpdateCategoryRequest = CreateCategoryRequest

// ====== 交易 ======

type CreateTransactionRequest struct {
	BookID           uint      `json:"book_id" binding:"required"`
	Type             string    `json:"type" binding:"required,oneof=expense income transfer refund reimburse adjust"`
	Amount           float64   `json:"amount" binding:"required,gt=0"`
	Currency         string    `json:"currency" binding:"omitempty,max=10"`
	ExchangeRate     float64   `json:"exchange_rate"`
	CategoryID       uint      `json:"category_id"`
	AccountID        uint      `json:"account_id" binding:"required"`
	ToAccountID      uint      `json:"to_account_id"`
	TransferFee      float64   `json:"transfer_fee"`
	TransferDiscount float64   `json:"transfer_discount"`
	RefundOfID       uint      `json:"refund_of_id"`
	TxDate           FlexDate `json:"tx_date" binding:"required"`
	Description      string    `json:"description" binding:"omitempty,max=500"`
	TagIDs           []uint    `json:"tag_ids"`
	Images           []string  `json:"images"`
	Merchant         string    `json:"merchant" binding:"omitempty,max=200"`
	Location         string    `json:"location" binding:"omitempty,max=255"`
	IncludeInBalance bool      `json:"include_in_balance"`
	IncludeInBudget  bool      `json:"include_in_budget"`
	RecurringID      uint      `json:"recurring_id"`
	InstallmentID    uint      `json:"installment_id"`
	Remark           string    `json:"remark" binding:"omitempty,max=1000"`
}

type UpdateTransactionRequest = CreateTransactionRequest

type QueryTransactionRequest struct {
	BookID     uint      `form:"book_id"`
	Type       string    `form:"type"`
	CategoryID uint      `form:"category_id"`
	AccountID  uint      `form:"account_id"`
	TagID      uint      `form:"tag_id"`
	StartDate  FlexDate `form:"start_date"`
	EndDate    FlexDate `form:"end_date"`
	Keyword    string    `form:"keyword"`
	MinAmount  float64   `form:"min_amount"`
	MaxAmount  float64   `form:"max_amount"`
	Page       int       `form:"page,default=1"`
	PageSize   int       `form:"page_size,default=20"`
}

// ====== 标签 ======

type CreateTagRequest struct {
	BookID uint   `json:"book_id"`
	Name   string `json:"name" binding:"required,max=50"`
	Color  string `json:"color" binding:"omitempty,max=20"`
	Sort   int    `json:"sort"`
}

// ====== 预算 ======

type CreateBudgetRequest struct {
	BookID     uint      `json:"book_id" binding:"required"`
	PeriodType string    `json:"period_type" binding:"required,oneof=monthly yearly custom"`
	CategoryID uint      `json:"category_id"`
	Amount     float64   `json:"amount" binding:"required,gt=0"`
	StartDate  FlexDate `json:"start_date" binding:"required"`
	EndDate    FlexDate `json:"end_date" binding:"required"`
	AlertRate  float64   `json:"alert_rate" binding:"omitempty,min=0,max=1"`
	RollOver   bool      `json:"roll_over"`
}

// ====== 存钱计划 ======

type CreateSavingPlanRequest struct {
	BookID        uint      `json:"book_id"`
	AccountID     uint      `json:"account_id"`
	Name          string    `json:"name" binding:"required,max=100"`
	Icon          string    `json:"icon" binding:"omitempty,max=50"`
	Color         string    `json:"color" binding:"omitempty,max=20"`
	TargetAmount  float64   `json:"target_amount" binding:"required,gt=0"`
	CurrentAmount float64   `json:"current_amount"`
	StartDate     FlexDate `json:"start_date" binding:"required"`
	TargetDate    FlexDate `json:"target_date" binding:"required"`
}

type AddSavingRecordRequest struct {
	Amount      float64   `json:"amount" binding:"required,gt=0"`
	RecordDate  FlexDate `json:"record_date" binding:"required"`
	TransactionID uint    `json:"transaction_id"`
	Note        string    `json:"note" binding:"omitempty,max=500"`
}

// ====== 周期记账 ======

type CreateRecurringRequest struct {
	BookID        uint   `json:"book_id" binding:"required"`
	Name          string `json:"name" binding:"required,max=100"`
	Type          string `json:"type" binding:"required,oneof=expense income transfer"`
	Amount        float64 `json:"amount" binding:"required,gt=0"`
	CategoryID    uint   `json:"category_id" binding:"required"`
	AccountID     uint   `json:"account_id" binding:"required"`
	ToAccountID   uint   `json:"to_account_id"`
	Description   string `json:"description" binding:"omitempty,max=500"`
	TagIDs        []uint `json:"tag_ids"`
	RecurringType string `json:"recurring_type" binding:"required,oneof=daily weekly biweekly monthly yearly custom"`
	Interval      int    `json:"interval" binding:"omitempty,min=1"`
	Weekday       int    `json:"weekday" binding:"omitempty,min=1,max=7"`
	MonthDay      int    `json:"month_day" binding:"omitempty,min=1,max=31"`
	StartDate     string `json:"start_date" binding:"required"`
	EndDate       string `json:"end_date"`
	MaxTimes      int    `json:"max_times"`
}

// ====== 分期 ======

type CreateInstallmentRequest struct {
	BookID         uint   `json:"book_id" binding:"required"`
	Name           string `json:"name" binding:"required,max=100"`
	TotalAmount    float64 `json:"total_amount" binding:"required,gt=0"`
	TotalMonths    int    `json:"total_months" binding:"required,min=1"`
	InterestAmount float64 `json:"interest_amount"`
	CategoryID     uint   `json:"category_id" binding:"required"`
	AccountID      uint   `json:"account_id" binding:"required"`
	FirstRepayDate string `json:"first_repay_date" binding:"required"`
	Description    string `json:"description" binding:"omitempty,max=500"`
}

// ====== 报销 ======

type CreateReimbursementRequest struct {
	BookID         uint   `json:"book_id" binding:"required"`
	Name           string `json:"name" binding:"required,max=100"`
	TotalAmount    float64 `json:"total_amount" binding:"required,gt=0"`
	Remark         string `json:"remark" binding:"omitempty,max=1000"`
	TransactionIDs []uint `json:"transaction_ids"`
}

type UpdateReimbursementRequest struct {
	Status         string  `json:"status" binding:"required,oneof=pending received partial"`
	ReceivedAmount float64 `json:"received_amount"`
	Remark         string  `json:"remark" binding:"omitempty,max=1000"`
}

// ====== 统计 ======

type StatisticsRequest struct {
	BookID    uint      `form:"book_id"`
	StartDate FlexDate `form:"start_date" binding:"required"`
	EndDate   FlexDate `form:"end_date" binding:"required"`
	Dimension string    `form:"dimension,default=category"` // category, account, tag, month, week, day
	Kind      string    `form:"kind"` // expense, income, all
}

// ====== 导入导出 ======

type ImportRequest struct {
	Source   string `form:"source" binding:"required,oneof=wechat alipay qianji template"`
	BookID   uint   `form:"book_id" binding:"required"`
	Password string `form:"password"` // 微信/支付宝账单解压密码
}
