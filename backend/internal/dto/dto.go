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
	IsArchived  bool   `json:"is_archived"` // 归档：停用但保留数据，列表默认不展示
	Sort        int    `json:"sort"`
}

type UpdateBookRequest = CreateBookRequest

// InviteMemberRequest 邀请成员。identifier 同时接受用户名或邮箱 —— 此前前端传 email
// 而后端只认 username，导致 `WHERE username = ''` 匹配不到，邀请 100% 失败（C5）。
type InviteMemberRequest struct {
	Username   string `json:"username"`
	Email      string `json:"email"`
	Identifier string `json:"identifier"` // 用户名或邮箱（推荐）
	Role       string `json:"role" binding:"omitempty,oneof=editor viewer"`
}

// Who 返回最终用于查找被邀请人的标识
func (r *InviteMemberRequest) Who() string {
	if r.Identifier != "" {
		return r.Identifier
	}
	if r.Username != "" {
		return r.Username
	}
	return r.Email
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
	APR             float64 `json:"apr"`
	IncludeInTotal  bool    `json:"include_in_total"`
	IncludeInBudget bool    `json:"include_in_budget"`
	IsHidden        bool    `json:"is_hidden"`
	IsArchived      bool    `json:"is_archived"`
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

// UpdateCategoryRequest 为独立的更新结构：所有字段均可选，
// 未提供的字段不会被覆盖为类型零值（例如更新名称时无需重复传 kind）。
type UpdateCategoryRequest struct {
	BookID   uint   `json:"book_id"`
	ParentID uint   `json:"parent_id"`
	Name     string `json:"name" binding:"omitempty,max=50"`
	Kind     string `json:"kind" binding:"omitempty,oneof=expense income system"`
	Icon     string `json:"icon" binding:"omitempty,max=50"`
	Color    string `json:"color" binding:"omitempty,max=20"`
	Sort     int    `json:"sort"`
	NeedTag  *bool  `json:"need_tag"`
}

// ====== 交易 ======

type CreateTransactionRequest struct {
	BookID           uint      `json:"book_id" binding:"required"`
	Type             string    `json:"type" binding:"required,oneof=expense income transfer refund reimburse adjust"`
	Amount           float64   `json:"amount" binding:"required,gt=0"`
	Currency         string    `json:"currency" binding:"omitempty,max=10"`
	// ExchangeRate：1 单位原币 = ? 单位基准币。未传时后端按 1 兜底。
	// 显式传值必须为正数——0 只会来自脏数据（Go 零值），会让折算结果变成 0。
	ExchangeRate     float64   `json:"exchange_rate" binding:"omitempty,gt=0"`
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
	// IncludeInBalance / IncludeInBudget 用指针：未传（nil）时由后端按业务默认值兜底
	// （均为 true）。此前用 bool，客户端不传即落 false，导致预算进度恒为 0、
	// 且后端又把 false 强制改回 true，API 语义自相矛盾。
	IncludeInBalance *bool     `json:"include_in_balance"`
	IncludeInBudget  *bool     `json:"include_in_budget"`
	RecurringID      uint      `json:"recurring_id"`
	InstallmentID    uint      `json:"installment_id"`
	InstallmentIndex int       `json:"installment_index"`
	Remark           string    `json:"remark" binding:"omitempty,max=1000"`
	ReimburseStatus  string    `json:"reimburse_status" binding:"omitempty,oneof=none pending done"`
}

// BalanceFlag 取「是否计入余额」的最终值：显式传值尊重客户端，未传（nil）默认 true
func (r *CreateTransactionRequest) BalanceFlag() bool {
	if r.IncludeInBalance == nil {
		return true
	}
	return *r.IncludeInBalance
}

// BudgetFlag 取「是否计入预算」的最终值：显式传值尊重客户端，未传（nil）默认 true
func (r *CreateTransactionRequest) BudgetFlag() bool {
	if r.IncludeInBudget == nil {
		return true
	}
	return *r.IncludeInBudget
}

// UpdateTransactionRequest 更新交易：补丁语义（PATCH semantics）。
//
// 每个字段用指针区分「未传」与「传了零值」：只有非 nil 的字段才会写库。
// 此前这里直接 `= CreateTransactionRequest`（全字段必填且一律覆盖），
// 而前端表单只提交十来个字段，导致一次「改备注」式的无害编辑就把
// transfer_fee / refund_of_id / reimburse_status / images 全部清零（原 B-01），
// 并且已回滚的转账手续费派生交易因 fee=0 不再重建，造成账实永久不符。
type UpdateTransactionRequest struct {
	BookID           *uint      `json:"book_id"`
	Type             *string    `json:"type" binding:"omitempty,oneof=expense income transfer refund reimburse adjust"`
	Amount           *float64   `json:"amount" binding:"omitempty,gt=0"`
	Currency         *string    `json:"currency" binding:"omitempty,max=10"`
	ExchangeRate     *float64   `json:"exchange_rate" binding:"omitempty,gt=0"`
	CategoryID       *uint      `json:"category_id"`
	AccountID        *uint      `json:"account_id"`
	ToAccountID      *uint      `json:"to_account_id"`
	TransferFee      *float64   `json:"transfer_fee" binding:"omitempty,gte=0"`
	TransferDiscount *float64   `json:"transfer_discount" binding:"omitempty,gte=0"`
	RefundOfID       *uint      `json:"refund_of_id"`
	TxDate           *FlexDate  `json:"tx_date"`
	Description      *string    `json:"description" binding:"omitempty,max=500"`
	TagIDs           *[]uint    `json:"tag_ids"`
	Images           *[]string  `json:"images"`
	Merchant         *string    `json:"merchant" binding:"omitempty,max=200"`
	Location         *string    `json:"location" binding:"omitempty,max=255"`
	// 与创建接口一致：未传（nil）保持旧值；显式传才覆盖。
	IncludeInBalance *bool      `json:"include_in_balance"`
	IncludeInBudget  *bool      `json:"include_in_budget"`
	Remark           *string    `json:"remark" binding:"omitempty,max=1000"`
	ReimburseStatus  *string    `json:"reimburse_status" binding:"omitempty,oneof=none pending done"`
}

// TagIDsOrNil 取出标签集合：显式传 null（清空）与未传要能区分。
// 未传时返回 nil，调用方据此保持旧标签不变。
func (r *UpdateTransactionRequest) TagIDsOrNil() []uint {
	if r.TagIDs == nil {
		return nil
	}
	return *r.TagIDs
}

// HasTagIDs 判断是否显式提交了 tag_ids（含空数组 = 清空标签）
func (r *UpdateTransactionRequest) HasTagIDs() bool {
	return r.TagIDs != nil
}

// NormalizedReimburseStatus 报销状态兜底：空串写库会得到非法值，
// 使该笔流水在「按 reimburse_status 筛选」时永远查不到。
func (r *UpdateTransactionRequest) NormalizedReimburseStatus(old string) string {
	if r.ReimburseStatus == nil {
		if old == "" {
			return "none"
		}
		return old
	}
	if *r.ReimburseStatus == "" {
		return "none"
	}
	return *r.ReimburseStatus
}

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
	ReimburseStatus string `form:"reimburse_status"`
	Page       int       `form:"page,default=1"`
	PageSize   int       `form:"page_size,default=20"`
	// 游标分页（原 F-04）：与此同时 offset 分页在「加载更多」时会被数据位移
	// 打乱（新插入一条 → 下一页首条与上一页末条重复 → 前端去重后永远加载不到新数据）。
	// 传了 cursor_* 时忽略 page，改为从「上一页最后一条的 (tx_date, id)」继续往前取。
	CursorDate FlexDate `form:"cursor_date"`
	CursorID   uint     `form:"cursor_id"`
}

// UseCursor 是否走游标分页
func (r *QueryTransactionRequest) UseCursor() bool {
	return r.CursorID > 0 && !r.CursorDate.IsZero()
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
	CategoryID uint      `json:"category_id"` // 0 = 总预算
	Amount     float64   `json:"amount" binding:"required,gt=0"`
	// start_date / end_date 不再强制必填：前端只让用户选周期类型时，
	// 后端按 period_type 从 start_date（缺省为今天所属周期首日）推导区间。
	StartDate  FlexDate `json:"start_date"`
	EndDate    FlexDate `json:"end_date"`
	AlertRate  float64  `json:"alert_rate" binding:"omitempty,min=0,max=1"`
	RollOver   bool     `json:"roll_over"`
}

// UpdateBudgetRequest 独立的更新结构：金额以「元」为单位传入（与创建接口一致），
// 未传的字段不覆盖旧值，避免整包覆盖把 period_type / 日期清空。
type UpdateBudgetRequest struct {
	PeriodType string    `json:"period_type" binding:"omitempty,oneof=monthly yearly custom"`
	CategoryID *uint     `json:"category_id"`
	Amount     *float64  `json:"amount" binding:"omitempty,gt=0"`
	StartDate  FlexDate  `json:"start_date"`
	EndDate    FlexDate  `json:"end_date"`
	AlertRate  *float64  `json:"alert_rate" binding:"omitempty,min=0,max=1"`
	RollOver   *bool     `json:"roll_over"`
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

// VerifyPasswordRequest 敏感操作的二次验证（C15）
type VerifyPasswordRequest struct {
	Password string `json:"password" binding:"required"`
}

type AddSavingRecordRequest struct {
	Amount      float64   `json:"amount" binding:"required,gt=0"`
	RecordDate  FlexDate `json:"record_date" binding:"required"`
	TransactionID uint    `json:"transaction_id"`
	// AccountID 资金来源账户（B6）。不传时取用户第一个非负债账户。
	// 存钱会真实扣减该账户余额并生成交易，使计划进度与资产一致。
	AccountID   uint      `json:"account_id"`
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
	// AccountID 收款账户（B5）。报销到账时按 received_amount 生成一条收入交易，
	// 否则钱在资产里凭空消失、净资产失真。
	AccountID      uint    `json:"account_id"`
	ReceivedDate   FlexDate `json:"received_date"`
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
