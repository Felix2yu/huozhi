package models

import (
	"time"

	"gorm.io/gorm"
)

// BaseModel 所有模型的基础
type BaseModel struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// ==================== 用户与认证 ====================

// User 用户
type User struct {
	BaseModel
	Username     string    `gorm:"size:50;uniqueIndex;not null" json:"username"`
	Email        *string   `gorm:"size:100;uniqueIndex" json:"email"` // 指针类型：空值写 NULL，避免 SQLite 唯一索引把 '' 当值冲突
	Phone        *string   `gorm:"size:20;uniqueIndex" json:"phone"`
	PasswordHash string    `gorm:"size:255;not null" json:"-"`
	Nickname     string    `gorm:"size:50" json:"nickname"`
	Avatar       string    `gorm:"size:255" json:"avatar"`
	Locale       string    `gorm:"size:10;default:zh-CN" json:"locale"`
	Timezone     string    `gorm:"size:50;default:Asia/Shanghai" json:"timezone"`
	MonthStart   int       `gorm:"default:1" json:"month_start"` // 自定义账期起始日 1-28
	Currency     string    `gorm:"size:10;default:CNY" json:"currency"`
	IsVIP        bool      `gorm:"default:false" json:"is_vip"`
	LastLoginAt  time.Time `json:"last_login_at"`
	Status       int       `gorm:"default:1" json:"status"` // 1正常 0禁用
}

// ==================== 账本 ====================

// Book 账本
type Book struct {
	BaseModel
	UserID      uint   `gorm:"not null;index" json:"user_id"`
	Name        string `gorm:"size:100;not null" json:"name"`
	Icon        string `gorm:"size:50" json:"icon"`
	Color       string `gorm:"size:20" json:"color"`
	Description string `gorm:"size:500" json:"description"`
	Currency    string `gorm:"size:10;default:CNY" json:"currency"`
	IsDefault   bool   `gorm:"default:false" json:"is_default"`
	IsArchived  bool   `gorm:"default:false" json:"is_archived"`
	Sort        int    `gorm:"default:0" json:"sort"`
}

// BookMember 账本成员（共享账本）
type BookMember struct {
	BaseModel
	BookID     uint   `gorm:"not null;uniqueIndex:idx_book_user" json:"book_id"`
	UserID     uint   `gorm:"not null;uniqueIndex:idx_book_user" json:"user_id"`
	Role       string `gorm:"size:20;default:viewer" json:"role"` // owner, editor, viewer
	Permission string `gorm:"size:255" json:"permission"`
	JoinedAt   time.Time `json:"joined_at"`
}

// ==================== 账户 / 资产 ====================

// AccountType 账户类型
type AccountType string

const (
	AccCash      AccountType = "cash"       // 现金
	AccBank      AccountType = "bank"       // 储蓄卡
	AccCredit    AccountType = "credit"     // 信用卡
	AccPrepaid   AccountType = "prepaid"    // 储值卡/公交卡/饭卡
	AccInvest    AccountType = "investment" // 投资账户（股票/基金等）
	AccLiability AccountType = "liability"  // 负债（花呗/借呗/贷款）
	AccVirtual   AccountType = "virtual"    // 虚拟（支付宝余额/微信零钱等）
)

// Account 账户/资产
type Account struct {
	BaseModel
	UserID        uint        `gorm:"not null;index" json:"user_id"`
	BookID        uint        `gorm:"default:0;index" json:"book_id"` // 0=所有账本通用
	Name          string      `gorm:"size:100;not null" json:"name"`
	Type          AccountType `gorm:"size:20;not null;index" json:"type"`
	Currency      string      `gorm:"size:10;default:CNY" json:"currency"`
	Balance       float64     `gorm:"default:0" json:"balance"`         // 当前余额
	InitialAmount float64     `gorm:"default:0" json:"initial_amount"`  // 初始金额
	Icon          string      `gorm:"size:50" json:"icon"`
	Color         string      `gorm:"size:20" json:"color"`
	BankName        string `gorm:"size:100" json:"bank_name"`          // 银行名
	CardNo4         string `gorm:"size:10" json:"card_no4"`            // 尾号4位
	EncryptedCardNo string `gorm:"size:512" json:"-"`                  // 完整卡号（AES-GCM 加密存，默认不返回前端）
	// 信用卡专属
	CreditLimit     float64 `gorm:"default:0" json:"credit_limit"`       // 额度
	BillDay         int     `gorm:"default:0" json:"bill_day"`           // 账单日
	RepayDay        int     `gorm:"default:0" json:"repay_day"`          // 还款日
	ExpireMonth     int     `gorm:"default:0" json:"expire_month"`       // 卡有效期月份 1-12
	ExpireYear      int     `gorm:"default:0" json:"expire_year"`        // 卡有效期年份 如 27（2027年）
	EncryptedCVV    string  `gorm:"size:128" json:"-"`                   // CVV2/CVC2（AES-GCM 加密存）
	// 负债专属
	APR           float64 `gorm:"default:0" json:"apr"`                // 年化利率
	// 通用设置
	IncludeInTotal bool   `gorm:"default:true" json:"include_in_total"` // 计入资产总计
	IncludeInBudget bool  `gorm:"default:true" json:"include_in_budget"`
	IsHidden       bool   `gorm:"default:false" json:"is_hidden"`
	IsArchived     bool   `gorm:"default:false" json:"is_archived"`
	GroupID        uint   `gorm:"default:0" json:"group_id"`           // 资产分组
	Sort           int    `gorm:"default:0" json:"sort"`
	Remark         string `gorm:"size:500" json:"remark"`
}

// AccountGroup 资产分组
type AccountGroup struct {
	BaseModel
	UserID uint   `gorm:"not null;index" json:"user_id"`
	Name   string `gorm:"size:100;not null" json:"name"`
	Icon   string `gorm:"size:50" json:"icon"`
	Sort   int    `gorm:"default:0" json:"sort"`
}

// ==================== 分类 ====================

// CategoryKind 收支种类
type CategoryKind string

const (
	KindExpense CategoryKind = "expense" // 支出
	KindIncome  CategoryKind = "income"  // 收入
	KindSystem  CategoryKind = "system"  // 系统（转账等）
)

// Category 分类
type Category struct {
	BaseModel
	UserID     uint         `gorm:"not null;index" json:"user_id"`
	BookID     uint         `gorm:"default:0;index" json:"book_id"`
	ParentID   uint         `gorm:"default:0;index" json:"parent_id"` // 0=一级
	Name       string       `gorm:"size:50;not null" json:"name"`
	Kind       CategoryKind `gorm:"size:10;not null;index" json:"kind"`
	Icon       string       `gorm:"size:50" json:"icon"`
	Color      string       `gorm:"size:20" json:"color"`
	Sort       int          `gorm:"default:0" json:"sort"`
	IsSystem   bool         `gorm:"default:false" json:"is_system"` // 系统内置不可删
	IsArchived bool         `gorm:"default:false" json:"is_archived"`
	NeedTag    bool         `gorm:"default:false" json:"need_tag"` // 是否强制标签
}

// ==================== 交易/账单 ====================

// TransactionType 交易类型
type TransactionType string

const (
	TxExpense    TransactionType = "expense"    // 支出
	TxIncome     TransactionType = "income"     // 收入
	TxTransfer   TransactionType = "transfer"   // 转账
	TxRefund     TransactionType = "refund"     // 退款（关联原支出）
	TxReimburse  TransactionType = "reimburse"  // 报销（支出可被报销）
	TxAdjust     TransactionType = "adjust"     // 余额调整
)

// Transaction 交易记录
type Transaction struct {
	BaseModel
	UserID        uint            `gorm:"not null;index" json:"user_id"`
	BookID        uint            `gorm:"not null;index" json:"book_id"`
	Type          TransactionType `gorm:"size:20;not null;index" json:"type"`
	Amount        float64         `gorm:"not null;index" json:"amount"`
	Currency      string          `gorm:"size:10;default:CNY" json:"currency"`
	// 汇率（多币种），amount * ExchangeRate = 账本货币金额
	ExchangeRate  float64         `gorm:"default:1" json:"exchange_rate"`
	CategoryID    uint            `gorm:"not null;index" json:"category_id"`
	AccountID     uint            `gorm:"not null;index" json:"account_id"`
	// 转账相关
	ToAccountID   uint            `gorm:"default:0;index" json:"to_account_id"`
	TransferFee   float64         `gorm:"default:0" json:"transfer_fee"`
	TransferDiscount float64      `gorm:"default:0" json:"transfer_discount"`
	// 关联退款/报销
	RefundOfID    uint            `gorm:"default:0;index" json:"refund_of_id"`
	ReimburseStatus string        `gorm:"size:20;default:none" json:"reimburse_status"` // none, pending, done
	ReimburseAmount float64       `gorm:"default:0" json:"reimburse_amount"`
	// 记账者（协作账本中记录是谁记的账）
	RecordedBy    string          `gorm:"size:100" json:"recorded_by"`
	// 账单标记（信用卡账单归属标记，如某笔消费归属的账单月份）
	BillMarker    string          `gorm:"size:100" json:"bill_marker"`
	// 外部来源 ID（钱迹原始交易 ID）：不替换内部自增主键 id，仅作业务外部引用，
	// 用于导入幂等去重与「关联账单」外键解析。跨导出/跨用户不保证全局唯一，故仅按 (user_id, book_id, external_id) 查询。
	ExternalID    string          `gorm:"size:100;index" json:"external_id"`
	// 关联账单：钱迹「关联账单」列记录的原交易外部 ID；入库后解析为 RefundOfID（内部字段，不对外暴露）
	RefundOfExternalID string     `gorm:"size:100" json:"-"`
	// 记账日期（重要：可以与创建时间不同）
	TxDate        time.Time       `gorm:"not null;index" json:"tx_date"`
	Description   string          `gorm:"size:500" json:"description"`
	Tags          []*Tag          `gorm:"many2many:transaction_tags" json:"tags,omitempty"`
	// 图片附件
	Images        []string        `gorm:"serializer:json" json:"images"`
	// 商家/地点
	Merchant      string          `gorm:"size:200" json:"merchant"`
	Location      string          `gorm:"size:255" json:"location"`
	// 设置
	IncludeInBalance bool          `gorm:"default:true" json:"include_in_balance"`
	IncludeInBudget  bool          `gorm:"default:true" json:"include_in_budget"`
	IsRecurring    bool           `gorm:"default:false" json:"is_recurring"`
	RecurringID    uint           `gorm:"default:0;index" json:"recurring_id"`
	InstallmentID  uint           `gorm:"default:0;index" json:"installment_id"`
	// 分期
	InstallmentIndex int         `gorm:"default:0" json:"installment_index"` // 第几期
	InstallmentTotal int         `gorm:"default:0" json:"installment_total"`
	Remark         string          `gorm:"size:1000" json:"remark"`
}

// TransactionTag 交易标签多对多
type TransactionTag struct {
	TransactionID uint `gorm:"primaryKey" json:"transaction_id"`
	TagID         uint `gorm:"primaryKey" json:"tag_id"`
}

// ==================== 标签 ====================

// Tag 标签
type Tag struct {
	BaseModel
	UserID uint   `gorm:"not null;index" json:"user_id"`
	BookID uint   `gorm:"default:0;index" json:"book_id"`
	Name   string `gorm:"size:50;not null;index" json:"name"`
	Color  string `gorm:"size:20" json:"color"`
	Sort   int    `gorm:"default:0" json:"sort"`
	Count  int    `gorm:"default:0" json:"count"` // 使用次数
}

// ==================== 预算 ====================

// Budget 预算
type Budget struct {
	BaseModel
	UserID      uint         `gorm:"not null;index" json:"user_id"`
	BookID      uint         `gorm:"not null;index" json:"book_id"`
	PeriodType  string       `gorm:"size:20;not null" json:"period_type"` // monthly, yearly, custom
	CategoryID  uint         `gorm:"default:0;index" json:"category_id"` // 0=总预算
	Amount      float64      `gorm:"not null" json:"amount"`
	UsedAmount  float64      `gorm:"default:0" json:"used_amount"`
	StartDate   time.Time    `gorm:"not null" json:"start_date"`
	EndDate     time.Time    `gorm:"not null" json:"end_date"`
	AlertRate   float64      `gorm:"default:0.8" json:"alert_rate"` // 超80%提醒
	RollOver    bool         `gorm:"default:false" json:"roll_over"` // 结余滚入下月
}

// ==================== 存钱计划 ====================

// SavingPlan 存钱计划
type SavingPlan struct {
	BaseModel
	UserID        uint      `gorm:"not null;index" json:"user_id"`
	BookID        uint      `gorm:"default:0;index" json:"book_id"`
	AccountID     uint      `gorm:"default:0;index" json:"account_id"`
	Name          string    `gorm:"size:100;not null" json:"name"`
	Icon          string    `gorm:"size:50" json:"icon"`
	Color         string    `gorm:"size:20" json:"color"`
	TargetAmount  float64   `gorm:"not null" json:"target_amount"`
	CurrentAmount float64   `gorm:"default:0" json:"current_amount"`
	StartDate     time.Time `gorm:"not null" json:"start_date"`
	TargetDate    time.Time `gorm:"not null" json:"target_date"`
	Status        string    `gorm:"size:20;default:active" json:"status"` // active, done, paused
}

// SavingRecord 存钱记录
type SavingRecord struct {
	BaseModel
	UserID      uint      `gorm:"not null;index" json:"user_id"`
	SavingPlanID uint     `gorm:"not null;index" json:"saving_plan_id"`
	Amount      float64   `gorm:"not null" json:"amount"`
	RecordDate  time.Time `gorm:"not null" json:"record_date"`
	TransactionID uint    `gorm:"default:0;index" json:"transaction_id"`
	Note        string    `gorm:"size:500" json:"note"`
}

// ==================== 周期记账 / 分期 ====================

// RecurringType 周期类型
type RecurringType string

const (
	RecDaily   RecurringType = "daily"
	RecWeekly  RecurringType = "weekly"
	RecBiWeek  RecurringType = "biweekly"
	RecMonthly RecurringType = "monthly"
	RecYearly  RecurringType = "yearly"
	RecCustom  RecurringType = "custom" // 按指定天数间隔
)

// Recurring 周期记账模板
type Recurring struct {
	BaseModel
	UserID      uint            `gorm:"not null;index" json:"user_id"`
	BookID      uint            `gorm:"not null;index" json:"book_id"`
	Name        string          `gorm:"size:100;not null" json:"name"`
	Type        TransactionType `gorm:"size:20;not null" json:"type"`
	Amount      float64         `gorm:"not null" json:"amount"`
	CategoryID  uint            `gorm:"not null" json:"category_id"`
	AccountID   uint            `gorm:"not null" json:"account_id"`
	ToAccountID uint            `gorm:"default:0" json:"to_account_id"`
	Description string          `gorm:"size:500" json:"description"`
	TagIDs      []uint          `gorm:"serializer:json" json:"tag_ids"`
	// 周期设置
	RecurringType RecurringType `gorm:"size:20;not null" json:"recurring_type"`
	Interval      int           `gorm:"default:1" json:"interval"` // custom类型时隔N天
	Weekday       int           `gorm:"default:0" json:"weekday"`  // weekly时：1-7
	MonthDay      int           `gorm:"default:1" json:"month_day"`
	// 范围
	StartDate     time.Time     `gorm:"not null" json:"start_date"`
	EndDate       time.Time     `json:"end_date"`
	MaxTimes      int           `gorm:"default:0" json:"max_times"` // 0=无限
	RunCount      int           `gorm:"default:0" json:"run_count"`
	LastRunAt     time.Time     `json:"last_run_at"`
	NextRunAt     time.Time     `gorm:"index" json:"next_run_at"`
	Status        string        `gorm:"size:20;default:active" json:"status"`
}

// ComputeNextRun 根据周期类型计算下一次执行时间（基于 from 时刻）
func (r *Recurring) ComputeNextRun(from time.Time) time.Time {
	if r == nil {
		return from
	}
	switch r.RecurringType {
	case RecDaily:
		interval := r.Interval
		if interval < 1 {
			interval = 1
		}
		return from.AddDate(0, 0, interval)

	case RecWeekly:
		target := r.Weekday
		if target < 1 || target > 7 {
			target = int(from.Weekday())
			if target == 0 {
				target = 7
			}
		}
		fromWeekday := int(from.Weekday())
		if fromWeekday == 0 {
			fromWeekday = 7
		}
		diff := target - fromWeekday
		if diff <= 0 {
			diff += 7
		}
		return time.Date(from.Year(), from.Month(), from.Day(), 9, 0, 0, 0, from.Location()).AddDate(0, 0, diff)

	case RecBiWeek:
		interval := r.Interval
		if interval < 1 {
			interval = 2
		}
		return from.AddDate(0, 0, 7*interval)

	case RecMonthly:
		day := r.MonthDay
		if day < 1 || day > 31 {
			day = from.Day()
		}
		nextMonth := from.AddDate(0, 1, 0)
		lastDay := time.Date(nextMonth.Year(), nextMonth.Month()+1, 0, 0, 0, 0, 0, from.Location()).Day()
		if day > lastDay {
			day = lastDay
		}
		return time.Date(nextMonth.Year(), nextMonth.Month(), day, 9, 0, 0, 0, from.Location())

	case RecYearly:
		return time.Date(from.Year()+1, from.Month(), from.Day(), 9, 0, 0, 0, from.Location())

	case RecCustom:
		interval := r.Interval
		if interval < 1 {
			interval = 1
		}
		return from.AddDate(0, 0, interval)
	}
	return from.AddDate(0, 0, 1)
}

// Installment 分期
type Installment struct {
	BaseModel
	UserID          uint      `gorm:"not null;index" json:"user_id"`
	BookID          uint      `gorm:"not null;index" json:"book_id"`
	Name            string    `gorm:"size:100;not null" json:"name"`
	TotalAmount     float64   `gorm:"not null" json:"total_amount"`
	TotalMonths     int       `gorm:"not null" json:"total_months"`
	PaidMonths      int       `gorm:"default:0" json:"paid_months"`
	MonthlyAmount   float64   `gorm:"not null" json:"monthly_amount"`
	InterestAmount  float64   `gorm:"default:0" json:"interest_amount"`
	CategoryID      uint      `gorm:"not null" json:"category_id"`
	AccountID       uint      `gorm:"not null" json:"account_id"`
	FirstRepayDate  time.Time `gorm:"not null" json:"first_repay_date"`
	NextRepayDate   time.Time `gorm:"index" json:"next_repay_date"`
	Description     string    `gorm:"size:500" json:"description"`
	Status          string    `gorm:"size:20;default:active" json:"status"`
}

// ==================== 报销 ====================

// Reimbursement 报销单
type Reimbursement struct {
	BaseModel
	UserID        uint      `gorm:"not null;index" json:"user_id"`
	BookID        uint      `gorm:"not null;index" json:"book_id"`
	Name          string    `gorm:"size:100;not null" json:"name"`
	TotalAmount   float64   `gorm:"not null" json:"total_amount"`
	ReceivedAmount float64  `gorm:"default:0" json:"received_amount"`
	Status        string    `gorm:"size:20;default:pending" json:"status"` // pending, received, partial
	SubmittedAt   time.Time `json:"submitted_at"`
	ReceivedAt    time.Time `json:"received_at"`
	Remark        string    `gorm:"size:1000" json:"remark"`
	// 关联交易
	TransactionIDs []uint  `gorm:"serializer:json" json:"transaction_ids"`
}

// ==================== 资产快照 ====================

// AssetSnapshot 资产快照（每日/每月）
type AssetSnapshot struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	UserID    uint      `gorm:"not null;uniqueIndex:idx_user_date" json:"user_id"`
	SnapDate  time.Time `gorm:"not null;uniqueIndex:idx_user_date;index" json:"snap_date"`
	TotalAsset float64  `gorm:"default:0" json:"total_asset"`
	TotalDebt  float64  `gorm:"default:0" json:"total_debt"`
	NetAsset   float64  `gorm:"default:0" json:"net_asset"`
	Currency   string   `gorm:"size:10;default:CNY" json:"currency"`
	Detail     string   `gorm:"type:text" json:"detail"` // JSON 各账户余额快照
	CreatedAt  time.Time `json:"created_at"`
}

// ==================== 同步 ====================

// SyncLog 同步日志
type SyncLog struct {
	BaseModel
	UserID       uint   `gorm:"not null;index" json:"user_id"`
	DeviceID     string `gorm:"size:100;index" json:"device_id"`
	DeviceType   string `gorm:"size:20" json:"device_type"`
	Operation    string `gorm:"size:20" json:"operation"`
	TableName    string `gorm:"size:50" json:"table_name"`
	RecordID     uint   `gorm:"index" json:"record_id"`
	SyncStatus   string `gorm:"size:20" json:"sync_status"`
	Version      int64  `json:"version"`
}
