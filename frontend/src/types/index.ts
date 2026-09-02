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
}

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
  flat_list: Transaction[];
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
  status: 'active' | 'paused';
  next_run_at: string;
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
  transaction_ids: number[];
}
