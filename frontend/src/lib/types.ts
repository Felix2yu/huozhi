// ====== 全部类型定义（与后端对应） ======

export type CategoryKind = 'expense' | 'income' | 'system';
export type AccountType =
  | 'cash' | 'bank' | 'credit' | 'prepaid'
  | 'investment' | 'liability' | 'virtual';
export type TransactionType =
  | 'expense' | 'income' | 'transfer'
  | 'refund' | 'reimburse' | 'adjust';
export type RecurringType = 'daily' | 'weekly' | 'biweekly' | 'monthly' | 'yearly' | 'custom';

export interface ApiResp<T = any> {
  code: number;
  message?: string;
  data?: T;
}

// ====== 用户 ======
export interface User {
  id: number;
  username: string;
  email?: string;
  phone?: string;
  nickname: string;
  avatar?: string;
  locale: string;
  timezone: string;
  month_start: number;
  currency: string;
  is_vip: boolean;
  last_login_at: string;
  created_at: string;
  status: number;
  // 汇率设置（基准货币 = currency，外币流水按 exchange_rate 折算到它）
  fx_auto_refresh?: boolean;
  fx_refresh_hours?: number;
}

// ====== 汇率 ======
/** 汇率快照：rates 的口径为「1 单位币种 = ? 单位基准币」，与 Transaction.exchange_rate 一致 */
export interface FxSnapshot {
  base: string;
  rates: Record<string, number>;
  currencies: string[];
  source?: string;
  fetched_at?: string;
  stale: boolean;
  error?: string;
  enabled: boolean;
  auto_refresh: boolean;
  refresh_hours: number;
}

/** 常用币种清单：记账表单与设置页共用，避免两处各维护一份（新增币种只改这里） */
export const CURRENCIES: { code: string; label: string; symbol: string }[] = [
  { code: 'CNY', label: '人民币', symbol: '¥' },
  { code: 'USD', label: '美元', symbol: '$' },
  { code: 'EUR', label: '欧元', symbol: '€' },
  { code: 'HKD', label: '港元', symbol: 'HK$' },
  { code: 'JPY', label: '日元', symbol: '¥' },
  { code: 'GBP', label: '英镑', symbol: '£' },
  { code: 'SGD', label: '新加坡元', symbol: 'S$' },
  { code: 'AUD', label: '澳元', symbol: 'A$' },
  { code: 'CAD', label: '加元', symbol: 'C$' },
  { code: 'KRW', label: '韩元', symbol: '₩' },
  { code: 'TWD', label: '新台币', symbol: 'NT$' },
  { code: 'MOP', label: '澳门元', symbol: 'MOP$' },
  { code: 'THB', label: '泰铢', symbol: '฿' },
  { code: 'MYR', label: '马来西亚林吉特', symbol: 'RM' },
  { code: 'NZD', label: '新西兰元', symbol: 'NZ$' },
  { code: 'CHF', label: '瑞士法郎', symbol: 'CHF' },
  { code: 'RUB', label: '俄罗斯卢布', symbol: '₽' },
  { code: 'INR', label: '印度卢比', symbol: '₹' },
  { code: 'VND', label: '越南盾', symbol: '₫' },
  { code: 'PHP', label: '菲律宾比索', symbol: '₱' },
  { code: 'IDR', label: '印尼盾', symbol: 'Rp' },
  { code: 'AED', label: '阿联酋迪拉姆', symbol: 'AED' }
];

// ====== 账本 ======
export interface Book {
  id: number;
  user_id: number;
  name: string;
  icon?: string;
  color?: string;
  description?: string;
  currency: string;
  is_default: boolean;
  is_archived: boolean;
  sort: number;
  created_at: string;
}

// ====== 账户 / 资产 ======
export interface Account {
  id: number;
  user_id: number;
  book_id: number;
  name: string;
  type: AccountType;
  currency: string;
  balance: number;
  initial_amount: number;
  icon?: string;
  color?: string;
  bank_name?: string;
  card_no4?: string;
  credit_limit?: number;
  bill_day?: number;
  repay_day?: number;
  expire_month?: number;
  expire_year?: number;
  apr?: number;
  include_in_total: boolean;
  include_in_budget: boolean;
  is_hidden: boolean;
  is_archived: boolean;
  group_id: number;
  sort: number;
  remark?: string;
  created_at: string;
}
export interface AccountSummary {
  total_asset: number;
  total_debt: number;
  net_asset: number;
  cash_flow: number;
}

// 信用卡还款倒计时项（/accounts/credit-summary 返回）
export interface CreditRepayItem {
  id: number;
  name: string;
  bank_name: string;
  card_no4: string;
  repay_day: number;
  bill_day: number;
  balance: number;
  credit_limit: number;
  days_left: number;
  repay_date: string;
  bill_amount: number;
  overdue: boolean;
}

// ====== 分类 ======
export interface Category {
  id: number;
  user_id: number;
  book_id: number;
  parent_id: number;
  name: string;
  kind: CategoryKind;
  icon?: string;
  color?: string;
  sort: number;
  is_system: boolean;
  is_archived: boolean;
  need_tag: boolean;
}
export interface CategoryTree extends Category {
  children: Category[];
}

// ====== 标签 ======
export interface Tag {
  id: number;
  user_id: number;
  book_id: number;
  name: string;
  color?: string;
  sort: number;
  count: number;
}

// ====== 交易 ======
export interface Transaction {
  id: number;
  user_id: number;
  book_id: number;
  type: TransactionType;
  amount: number;
  currency: string;
  exchange_rate: number;
  category_id: number;
  account_id: number;
  to_account_id?: number;
  transfer_fee?: number;
  transfer_discount?: number;
  refund_of_id?: number;
  reimburse_status: 'none' | 'pending' | 'done';
  reimburse_amount: number;
  tx_date: string;
  description?: string;
  tags?: Tag[];
  images?: string[];
  merchant?: string;
  location?: string;
  include_in_balance: boolean;
  include_in_budget: boolean;
  is_recurring: boolean;
  recurring_id?: number;
  installment_id?: number;
  installment_index: number;
  installment_total: number;
  remark?: string;
  created_at: string;
  // 服务端填充的只读派生字段（展示用，不回传）。
  // 「全部账本」视图下本地字典覆盖不到全部账本，必须依赖服务端给出的名称，
  // 否则分类稳定显示「未分类」、账户稳定显示「—」。
  amount_base?: number;
  category_name?: string;
  account_name?: string;
  to_account_name?: string;
}

export interface DayGroup {
  date: string;
  day_income: number;
  day_expense: number;
  day_balance: number;
  transactions: Transaction[];
}
export interface TransactionListData {
  grouped: DayGroup[];
  summary: { total_income: number; total_expense: number; net: number };
  // flat_list 已移除：它与 grouped 是同一批数据、被序列化两次（原 P-02）。
  // 需要平铺列表时用 grouped.flatMap((g) => g.transactions)。
  // A1：保留后端分页信息，供「加载更多」判断是否还有下一页
  pagination?: { page: number; page_size: number; total: number };
}

// ====== 预算 ======
export interface Budget {
  id: number;
  user_id: number;
  book_id: number;
  period_type: 'monthly' | 'yearly' | 'custom';
  category_id: number;
  amount: number;
  used_amount: number;
  start_date: string;
  end_date: string;
  alert_rate: number;
  roll_over: boolean;
  created_at: string;
}
export interface BudgetView extends Budget {
  remaining: number;
  usage_rate: number;
  is_over_budget: boolean;
  daily_budget: number;
}

// ====== 统计 ======
export interface StatsItem {
  id: number;
  name: string;
  icon?: string;
  color?: string;
  kind: string;
  amount: number;
  count: number;
  percent: number;
  parent_id: number;
}
export interface TrendPoint {
  date: string;
  income: number;
  expense: number;
  net: number;
}
export interface AssetPoint {
  month: string;
  total_asset: number;
  total_debt: number;
  net_asset: number;
}
export interface StatisticsData {
  range: { start: string; end: string; days: number };
  summary: {
    total_income: number;
    total_expense: number;
    net: number;
    income_count: number;
    expense_count: number;
    transaction_count: number;
    avg_daily_expense: number;
    avg_daily_income: number;
  };
  by_category_expense: StatsItem[];
  by_category_income: StatsItem[];
  by_account: Record<string, { account_id: number; income: number; expense: number }>;
  by_book?: Record<string, { book_id: number; book_name?: string; icon?: string; income: number; expense: number }>;
  trend: TrendPoint[];
  top_expense: {
    id: number; amount: number; description?: string;
    tx_date: string; category_id: number; merchant?: string;
  }[];
  asset_snapshots: any[];
}
export interface AssetOverview {
  total_asset: number;
  total_debt: number;
  net_asset: number;
  cash_on_hand: number;
  by_type: Record<string, number>;
  month_income: number;
  month_expense: number;
  month_net: number;
  account_count: number;
}

// ====== 存钱计划 ======
export interface SavingPlan {
  id: number;
  user_id: number;
  book_id: number;
  account_id: number;
  name: string;
  icon?: string;
  color?: string;
  target_amount: number;
  current_amount: number;
  start_date: string;
  target_date: string;
  status: 'active' | 'done' | 'paused';
  created_at: string;
}

// ====== 周期记账 / 分期 / 报销 ======
export interface Recurring {
  id: number;
  user_id: number;
  book_id: number;
  name: string;
  type: TransactionType;
  amount: number;
  category_id: number;
  account_id: number;
  to_account_id: number;
  description: string;
  tag_ids: number[];
  recurring_type: RecurringType;
  interval: number;
  weekday: number;
  month_day: number;
  start_date: string;
  end_date: string;
  max_times: number;
  run_count: number;
  status: 'active' | 'paused';
  // 达到最大次数 / 已过结束日期时后端会把 next_run_at 置为 NULL，
  // 类型上必须允许 null，否则会得到 "Invalid Date"。
  next_run_at: string | null;
  created_at: string;
}
export interface Installment {
  id: number;
  user_id: number;
  book_id: number;
  name: string;
  total_amount: number;
  total_months: number;
  paid_months: number;
  monthly_amount: number;
  interest_amount: number;
  category_id: number;
  account_id: number;
  first_repay_date: string;
  next_repay_date: string;
  status: 'active' | 'done';
  created_at: string;
}
export interface Reimbursement {
  id: number;
  user_id: number;
  book_id: number;
  name: string;
  total_amount: number;
  received_amount: number;
  status: 'pending' | 'received' | 'partial';
  submitted_at: string;
  received_at: string;
  remark: string;
  // 后端是 nil 切片时会序列化成 null（历史数据/未关联交易的报销单），
  // 类型上必须允许 null，调用方一律用 `?? []` 兜底，否则 `.length` 直接抛 TypeError。
  transaction_ids: number[] | null;
  created_at: string;
}

export type LoanDirection = 'lend' | 'borrow';
export type LoanStatus = 'active' | 'completed';
export type LoanInterestType = 'none' | 'simple' | 'monthly';

export interface Loan {
  id: number;
  user_id: number;
  book_id: number;
  direction: LoanDirection; // lend=借出（别人欠我），borrow=借入（我欠别人）
  counterparty: string; // 对方姓名/备注
  principal: number; // 本金（分）
  currency: string;
  interest_rate: number; // 年化/月利率，百分比数值
  interest_type: LoanInterestType;
  account_id: number; // 资金账户
  loan_date: string;
  due_date: string; // 可能为空
  note: string;
  status: LoanStatus;
  repaid_principal: number; // 已还本金（分）
  repaid_interest: number; // 已还利息（分）
  transaction_id: number;
  created_at: string;
}

export interface LoanRepayment {
  id: number;
  user_id: number;
  book_id: number;
  loan_id: number;
  amount: number; // 本次还本金（分）
  interest_amount: number; // 本次利息（分）
  repay_account_id: number;
  repaid_at: string;
  note: string;
  transaction_id: number;
  interest_tx_id: number;
  created_at: string;
}
