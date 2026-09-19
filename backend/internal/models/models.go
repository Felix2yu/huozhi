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
	APIKey       *string   `gorm:"size:64;uniqueIndex" json:"-"` // 外部API访问密钥（NULL=未生成，避免SQLite唯一索引空字符串冲突）
	APIKeyEnabled bool     `gorm:"default:false" json:"api_key_enabled"` // API密钥是否启用

	// 自动备份设置
	AutoBackupEnabled  bool      `gorm:"default:false" json:"auto_backup_enabled"`
	AutoBackupFrequency string  `gorm:"size:20;default:daily" json:"auto_backup_frequency"` // daily / weekly / monthly
	AutoBackupTime     string   `gorm:"size:5;default:03:00" json:"auto_backup_time"`       // HH:MM
	AutoBackupKeepCount int     `gorm:"default:7" json:"auto_backup_keep_count"`            // 保留份数
	AutoBackupLastRun  time.Time `json:"auto_backup_last_run"`

	// 汇率设置。Currency 同时作为「基准货币」：外币流水按 exchange_rate 折算到它。
	FxAutoRefresh  bool `gorm:"default:true" json:"fx_auto_refresh"` // 允许后台定时刷新汇率
	FxRefreshHours int  `gorm:"default:12" json:"fx_refresh_hours"`  // 刷新间隔（小时），0 = 跟随服务端配置
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
	Balance       Money       `gorm:"default:0" json:"balance"`         // 当前余额（分）
	InitialAmount Money       `gorm:"default:0" json:"initial_amount"`  // 初始金额（分）
	Icon          string      `gorm:"size:50" json:"icon"`
	Color         string      `gorm:"size:20" json:"color"`
	BankName        string `gorm:"size:100" json:"bank_name"`          // 银行名
	CardNo4         string `gorm:"size:10" json:"card_no4"`            // 尾号4位
	EncryptedCardNo string `gorm:"size:512" json:"-"`                  // 完整卡号（AES-GCM 加密存，默认不返回前端）
	// 信用卡专属
	CreditLimit     Money   `gorm:"default:0" json:"credit_limit"`       // 额度（分）
	BillDay         int     `gorm:"default:0" json:"bill_day"`           // 账单日
	RepayDay        int     `gorm:"default:0" json:"repay_day"`          // 还款日
	ExpireMonth     int     `gorm:"default:0" json:"expire_month"`       // 卡有效期月份 1-12
	ExpireYear      int     `gorm:"default:0" json:"expire_year"`        // 卡有效期年份 如 27（2027年）
	// EncryptedCVV 已移除：PCI-DSS 明确禁止在授权后以任何形式（含加密）存储 CVV2/CVC2。
	// 记账场景也不需要它，保留只会把「完整卡号 + CVV + 无二次验证」变成一次泄露即全量失守（C15）。
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
	// IsHidden：后端自动生成的隐蔽分类（转账手续费、报销回款等）。
	// 它们不应出现在分类管理页 / 记账表单的选择器 / 流水页的筛选下拉里（原 B-12），
	// 但仍是合法的分类引用——流水列表通过 category_name 直接展示，不会退化成「未分类」。
	IsHidden   bool         `gorm:"default:false" json:"is_hidden"`
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

// 衍生交易类型（Transaction.RelatedType）
const (
	RelatedTransferFee      = "transfer_fee"       // 转账手续费
	RelatedInstallmentRepay = "installment_repay"  // 分期每期还款
	RelatedReimburseReceived = "reimburse_received" // 报销收款
	RelatedSaving           = "saving"             // 存钱计划存入
	RelatedLoan             = "loan"               // 借贷（借出/借入/还款）
)

// Transaction 交易记录
type Transaction struct {
	BaseModel
	UserID        uint            `gorm:"not null;index" json:"user_id"`
	BookID        uint            `gorm:"not null;index" json:"book_id"`
	Type          TransactionType `gorm:"size:20;not null;index" json:"type"`
	Amount        Money           `gorm:"not null;index" json:"amount"` // 金额（分）
	Currency      string          `gorm:"size:10;default:CNY" json:"currency"`
	// 汇率（多币种），amount * ExchangeRate = 账本货币金额
	ExchangeRate  float64         `gorm:"default:1" json:"exchange_rate"`
	CategoryID    uint            `gorm:"not null;index" json:"category_id"`
	AccountID     uint            `gorm:"not null;index" json:"account_id"`
	// 转账相关
	ToAccountID   uint            `gorm:"default:0;index" json:"to_account_id"`
	TransferFee   Money           `gorm:"default:0" json:"transfer_fee"` // 手续费（分）
	TransferDiscount Money         `gorm:"default:0" json:"transfer_discount"` // 优惠（分）
	// 关联退款/报销
	RefundOfID    uint            `gorm:"default:0;index" json:"refund_of_id"`
	// 衍生交易关联：由另一笔交易自动派生出来的记录（转账手续费、分期还款、报销收款、存钱计划）。
	// 删除/修改主交易时据此级联撤销，避免留下与账户余额不符的孤儿流水。
	RelatedTxID   uint            `gorm:"default:0;index" json:"related_tx_id"`
	RelatedType   string          `gorm:"size:30;default:''" json:"related_type"` // transfer_fee, installment_repay, reimburse_received, saving
	ReimburseStatus string        `gorm:"size:20;default:none" json:"reimburse_status"` // none, pending, done
	ReimburseAmount Money         `gorm:"default:0" json:"reimburse_amount"` // 报销金额（分）
	// 报销收款归属的报销单（B5）。仅用于「同一张报销单只入账一次、补差额不重复」的幂等判定，
	// 不参与任何级联删除。此前幂等键只按 (账本, 账户, 金额) 匹配，两张金额相同的报销单
	// 只有第一张能生成收款交易，第二张的钱在资产里凭空消失。
	ReimbursementID uint          `gorm:"default:0;index" json:"reimbursement_id"`
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
	// 借贷（借出/借入/还款由借贷模块统一生成并维护）
	LoanID        uint           `gorm:"default:0;index" json:"loan_id"`
	Remark         string          `gorm:"size:1000" json:"remark"`

	// 只读派生字段（gorm:"-" 不落库），由服务端返回列表/详情前统一填充。
	// 1) category_name / account_name：前端字典是按 currentBookId 加载的，
	//    「全部账本」视图下跨账本流水的分类/账户必然查不到，稳定显示「未分类 / —」（原 P-06）；
	// 2) amount_base：外币流水折算后的基准币金额，避免前端各处重复实现汇率逻辑（原 B-02）。
	AmountBase    Money  `gorm:"-" json:"amount_base,omitempty"`
	CategoryName  string `gorm:"-" json:"category_name,omitempty"`
	AccountName   string `gorm:"-" json:"account_name,omitempty"`
	ToAccountName string `gorm:"-" json:"to_account_name,omitempty"`
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
	Amount      Money        `gorm:"not null" json:"amount"` // 金额（分）
	UsedAmount  Money        `gorm:"default:0" json:"used_amount"` // 已用（分）
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
	TargetAmount  Money     `gorm:"not null" json:"target_amount"` // 目标金额（分）
	CurrentAmount Money     `gorm:"default:0" json:"current_amount"` // 当前金额（分）
	StartDate     time.Time `gorm:"not null" json:"start_date"`
	TargetDate    time.Time `gorm:"not null" json:"target_date"`
	Status        string    `gorm:"size:20;default:active" json:"status"` // active, done, paused
}

// SavingRecord 存钱记录
type SavingRecord struct {
	BaseModel
	UserID      uint      `gorm:"not null;index" json:"user_id"`
	SavingPlanID uint     `gorm:"not null;index" json:"saving_plan_id"`
	Amount      Money     `gorm:"not null" json:"amount"` // 金额（分）
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
	Amount      Money           `gorm:"not null" json:"amount"` // 金额（分）
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

// monthRunAt 在 from 的年月上推进 months 个月，把日号设为 day 并收敛到目标月的最后一天。
//
// 不能用 from.AddDate(0, n, 0) 推进月份：AddDate 会把「1月31日 + 1个月」归一化成
// 3月3日（2月31日不存在），月末账单因此整月跳号（每月31号的周期在 1 月后会直接跳到 3 月）。
func monthRunAt(from time.Time, months int, day int) time.Time {
	loc := from.Location()
	total := int(from.Month()) - 1 + months
	y := from.Year() + total/12
	m := time.Month(total%12 + 1)
	if total < 0 {
		y = from.Year() + (total-11)/12
		m = time.Month((total%12+12)%12 + 1)
	}
	lastDay := time.Date(y, m+1, 0, 0, 0, 0, 0, loc).Day()
	if day < 1 || day > 31 {
		day = from.Day()
	}
	if day > lastDay {
		day = lastDay
	}
	return time.Date(y, m, day, 9, 0, 0, 0, loc)
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
		return monthRunAt(from, 1, r.MonthDay)

	case RecYearly:
		// 同样要把 2 月 29 日这类日期收敛到次年同月的最后一天，
		// 否则 Go 的归一化会把它变成 3 月 1 日，周年周期逐年漂移。
		return monthRunAt(from, 12, from.Day())

	case RecCustom:
		interval := r.Interval
		if interval < 1 {
			interval = 1
		}
		return from.AddDate(0, 0, interval)
	}
	return from.AddDate(0, 0, 1)
}

// ComputeFirstRun 计算首次执行时间（创建周期任务时初始化 next_run_at 用）。
//
// 语义：首次执行不得早于 start_date。此前实现统一用 ComputeNextRun(start_date - 1天)，
// 只对「每天 / 间隔 1」恰好成立，其余全部错位：
//   - monthly：起始月被整月跳过（09-19 起始 + 每月 25 号 → 10-25，应为 09-25）；
//   - yearly：被推到次年且偏 1 天（2026-09-19 → 2027-09-18）；
//   - daily/biweekly/custom 间隔 N：偏 N-1 天。
func (r *Recurring) ComputeFirstRun() time.Time {
	if r == nil {
		return time.Time{}
	}
	sd := r.StartDate
	if sd.IsZero() {
		sd = time.Now()
	}
	loc := sd.Location()
	// start_date 只精确到日，规整到当日 00:00
	base := time.Date(sd.Year(), sd.Month(), sd.Day(), 0, 0, 0, 0, loc)

	switch r.RecurringType {
	case RecMonthly:
		day := r.MonthDay
		if day < 1 || day > 31 {
			// 未指定每月几号 → 以 start_date 的日期为准，首次即 start_date 当天
			return base
		}
		if day >= base.Day() {
			return monthRunAt(base, 0, day)
		}
		return monthRunAt(base, 1, day)

	case RecWeekly:
		target := r.Weekday
		if target < 1 || target > 7 {
			return base
		}
		fromWd := int(base.Weekday())
		if fromWd == 0 {
			fromWd = 7
		}
		diff := target - fromWd
		if diff < 0 {
			diff += 7
		}
		return base.AddDate(0, 0, diff).Add(9 * time.Hour)

	case RecYearly:
		return time.Date(base.Year(), base.Month(), base.Day(), 9, 0, 0, 0, loc)
	}

	// daily / biweekly / custom：首次执行即 start_date 当天
	return base
}

// Installment 分期
type Installment struct {
	BaseModel
	UserID          uint      `gorm:"not null;index" json:"user_id"`
	BookID          uint      `gorm:"not null;index" json:"book_id"`
	Name            string    `gorm:"size:100;not null" json:"name"`
	TotalAmount     Money     `gorm:"not null" json:"total_amount"` // 总额（分）
	TotalMonths     int       `gorm:"not null" json:"total_months"`
	PaidMonths      int       `gorm:"default:0" json:"paid_months"`
	MonthlyAmount   Money     `gorm:"not null" json:"monthly_amount"` // 月供（分）
	InterestAmount  Money     `gorm:"default:0" json:"interest_amount"` // 利息（分）
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
	TotalAmount   Money     `gorm:"not null" json:"total_amount"` // 总额（分）
	ReceivedAmount Money     `gorm:"default:0" json:"received_amount"` // 已收（分）
	Status        string    `gorm:"size:20;default:pending" json:"status"` // pending, received, partial
	SubmittedAt   time.Time `json:"submitted_at"`
	ReceivedAt    time.Time `json:"received_at"`
	Remark        string    `gorm:"size:1000" json:"remark"`
	// 关联交易
	TransactionIDs []uint  `gorm:"serializer:json" json:"transaction_ids"`
}

// ==================== 借贷 ====================

// LoanDirection 借贷方向
type LoanDirection string

const (
	LoanLend   LoanDirection = "lend"   // 我借出（别人欠我）
	LoanBorrow LoanDirection = "borrow" // 我借入（我欠别人）
)

// Loan 借贷台账（个人借贷：借给朋友 / 向朋友借）
//
// 财务口径：借贷通过 transfer 联动真实账户，保证净资产正确——
//   - lend（借出）：钱从「资金账户」转出到系统「借出·应收」虚拟账户，现金减少、应收增加；
//   - borrow（借入）：钱从系统「借款·应付」负债账户转入「资金账户」，现金增加、欠款增加。
// 系统账户（应收 virtual / 应付 liability）余额天然代表当前净应收/应付，
// 自动并入资产概览的总资产/总负债（account.go 按账户类型汇总），无需改统计逻辑。
//
// 派生字段（不落库）：剩余本金、是否逾期，由列表出口统一计算。
type Loan struct {
	BaseModel
	UserID      uint          `gorm:"not null;index" json:"user_id"`
	BookID      uint          `gorm:"not null;index" json:"book_id"`
	Direction   LoanDirection `gorm:"size:10;not null;index" json:"direction"` // lend / borrow
	Counterparty string       `gorm:"size:100;not null" json:"counterparty"`  // 对方姓名/备注
	Principal   Money         `gorm:"not null" json:"principal"`               // 本金（分）
	Currency    string        `gorm:"size:10;default:CNY" json:"currency"`
	InterestRate float64      `gorm:"default:0" json:"interest_rate"`         // 年利率（%）
	InterestType string        `gorm:"size:10;default:none" json:"interest_type"` // none / simple / compound
	AccountID   uint          `gorm:"not null" json:"account_id"`             // 资金账户（借出时的出款账户 / 借入时的入账账户）
	LoanDate    time.Time     `gorm:"not null" json:"loan_date"`              // 借款日
	DueDate     time.Time     `json:"due_date"`                               // 到期日（可空）
	Note        string        `gorm:"size:1000" json:"note"`
	Status      string        `gorm:"size:20;default:active" json:"status"` // active / completed
	RepaidPrincipal Money      `gorm:"default:0" json:"repaid_principal"`    // 已还本金（分）
	RepaidInterest  Money      `gorm:"default:0" json:"repaid_interest"`     // 已还利息（分）
	TransactionID   uint       `gorm:"default:0" json:"transaction_id"`      // 创建时生成的 transfer 交易
}

// LoanRepayment 还款记录
type LoanRepayment struct {
	BaseModel
	UserID      uint      `gorm:"not null;index" json:"user_id"`
	BookID      uint      `gorm:"not null;index" json:"book_id"`
	LoanID      uint      `gorm:"not null;index" json:"loan_id"`
	Amount      Money     `gorm:"not null" json:"amount"`   // 还本金（分）
	InterestAmount Money   `gorm:"default:0" json:"interest_amount"` // 还利息（分）
	RepayAccountID uint    `gorm:"not null" json:"repay_account_id"` // 收款/还款账户
	RepaidAt    time.Time `gorm:"not null" json:"repaid_at"` // 还款日
	Note        string    `gorm:"size:1000" json:"note"`
	TransactionID uint     `gorm:"default:0" json:"transaction_id"`  // 本金 transfer 交易
	InterestTxID  uint     `gorm:"default:0" json:"interest_tx_id"`  // 利息 income/expense 交易
}

// ==================== 资产快照 ====================

// AssetSnapshot 资产快照（每日/每月）
type AssetSnapshot struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	UserID    uint      `gorm:"not null;uniqueIndex:idx_user_date" json:"user_id"`
	SnapDate  time.Time `gorm:"not null;uniqueIndex:idx_user_date;index" json:"snap_date"`
	TotalAsset Money    `gorm:"default:0" json:"total_asset"` // 总资产（分）
	TotalDebt  Money    `gorm:"default:0" json:"total_debt"` // 总负债（分）
	NetAsset   Money    `gorm:"default:0" json:"net_asset"` // 净资产（分）
	Currency   string   `gorm:"size:10;default:CNY" json:"currency"`
	Detail     string   `gorm:"type:text" json:"detail"` // JSON 各账户余额快照
	CreatedAt  time.Time `json:"created_at"`
}

// ==================== 汇率 ====================

// ExchangeRate 汇率快照（按「基准货币 + 目标币种」唯一）。
//
// Rate 的语义与 Transaction.ExchangeRate 保持一致：
// **1 单位 Currency = Rate 单位 Base**，即 `amount * rate = 基准币金额`。
// 上游汇率站大多给出反向口径（1 单位基准币 = N 单位外币），入库前统一取倒数，
// 避免调用方每处都要自己换算方向（方向搞反会让金额放大 N² 倍）。
type ExchangeRate struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	Base      string    `gorm:"size:10;not null;uniqueIndex:idx_fx_base_cur" json:"base"`
	Currency  string    `gorm:"size:10;not null;uniqueIndex:idx_fx_base_cur" json:"currency"`
	Rate      float64   `gorm:"not null" json:"rate"` // 1 单位 Currency = ? 单位 Base
	Source    string    `gorm:"size:50" json:"source"`
	FetchedAt time.Time `gorm:"index" json:"fetched_at"` // 上游数据的发布时间（非本机拉取时间）
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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
