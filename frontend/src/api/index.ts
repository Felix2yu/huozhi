import axios, { AxiosError, AxiosInstance } from 'axios';
import { toast } from 'sonner';
import type { ApiResp } from '@/types';

const http: AxiosInstance = axios.create({
  baseURL: '/api',
  timeout: 30_000,
  headers: { 'Content-Type': 'application/json' },
});

// 自动加token
http.interceptors.request.use((config) => {
  const token = localStorage.getItem('hz_token');
  if (token) config.headers.Authorization = `Bearer ${token}`;
  return config;
});

// 统一业务错误处理
http.interceptors.response.use(
  (resp) => {
    const data = resp.data as ApiResp;
    if (data && typeof data === 'object' && 'code' in data) {
      if (data.code !== 0) {
        toast.error(data.message || '请求失败');
        return Promise.reject(new Error(data.message || '业务错误'));
      }
      resp.data = data.data;
    }
    return resp;
  },
  (err: AxiosError<ApiResp>) => {
    if (err.response?.status === 401) {
      localStorage.removeItem('hz_token');
      if (!location.pathname.startsWith('/login')) {
        location.href = '/login?redirect=' + encodeURIComponent(location.pathname);
      }
    }
    const msg = (err.response?.data as any)?.message || err.message || '网络错误';
    toast.error(msg);
    return Promise.reject(err);
  },
);

export default http;

// ====================== 各模块 API 封装 ======================

import type {
  User, Book, Account, AccountSummary, Category, Tag,
  Transaction, TransactionListData, BudgetView, Budget,
  StatisticsData, AssetOverview, AssetPoint,
  SavingPlan, Recurring, Installment, Reimbursement,
} from '@/types';

// 认证
export const authApi = {
  register: (data: any) => http.post<any, User>('/auth/register', data),
  login:    (data: any) => http.post<any, { token: string; expire_in: number; user: User }>('/auth/login', data),
  me:       () => http.get<any, User>('/auth/me'),
  updateMe: (data: any) => http.put<any, User>('/auth/me', data),
  changePwd:(data: any) => http.post<any, void>('/auth/password', data),
  logout:   () => http.post<any, void>('/auth/logout'),
};

// 账本
export const bookApi = {
  list:     () => http.get<any, Book[]>('/books'),
  get:      (id: number) => http.get<any, Book>(`/books/${id}`),
  create:   (data: Partial<Book>) => http.post<any, Book>('/books', data),
  update:   (id: number, data: Partial<Book>) => http.put<any, Book>(`/books/${id}`, data),
  remove:   (id: number) => http.delete<any, void>(`/books/${id}`),
  listMembers: (id: number) => http.get<any, any[]>(`/books/${id}/members`),
  inviteMember: (id: number, data: any) => http.post<any, any>(`/books/${id}/members`, data),
};

// 账户
export const accountApi = {
  list: (params?: any) => http.get<any, { accounts: Account[]; summary: AccountSummary }>('/accounts', { params }),
  get:  (id: number) => http.get<any, Account>(`/accounts/${id}`),
  create: (data: any) => http.post<any, Account>('/accounts', data),
  update: (id: number, data: any) => http.put<any, Account>(`/accounts/${id}`, data),
  remove: (id: number) => http.delete<any, void>(`/accounts/${id}`),
  adjust: (id: number, data: any) => http.post<any, any>(`/accounts/${id}/adjust`, data),
  listGroups: () => http.get<any, any[]>('/accounts/groups'),
  createGroup: (data: any) => http.post<any, any>('/accounts/groups', data),
  removeGroup: (id: number) => http.delete<any, void>(`/accounts/groups/${id}`),
};

// 分类
export const categoryApi = {
  list: (params?: any) =>
    http.get<any, { expense: Category[]; income: Category[]; system: Category[] }>('/categories', { params }),
  create: (data: any) => http.post<any, Category>('/categories', data),
  update: (id: number, data: any) => http.put<any, Category>(`/categories/${id}`, data),
  remove: (id: number) => http.delete<any, void>(`/categories/${id}`),
};

// 标签
export const tagApi = {
  list:   () => http.get<any, Tag[]>('/tags'),
  create: (data: any) => http.post<any, Tag>('/tags', data),
  update: (id: number, data: any) => http.put<any, Tag>(`/tags/${id}`, data),
  remove: (id: number) => http.delete<any, void>(`/tags/${id}`),
};

// 交易
export const txApi = {
  list: (params?: any) => http.get<any, TransactionListData>('/transactions', { params }),
  get:  (id: number) => http.get<any, Transaction>(`/transactions/${id}`),
  create: (data: any) => http.post<any, Transaction>('/transactions', data),
  update: (id: number, data: any) => http.put<any, Transaction>(`/transactions/${id}`, data),
  remove: (id: number) => http.delete<any, void>(`/transactions/${id}`),
  batchRemove: (ids: number[]) => http.post<any, any>('/transactions/batch-delete', { ids }),
};

// 预算
export const budgetApi = {
  list:   (params?: any) => http.get<any, BudgetView[]>('/budgets', { params }),
  create: (data: Partial<Budget>) => http.post<any, Budget>('/budgets', data),
  update: (id: number, data: Partial<Budget>) => http.put<any, Budget>(`/budgets/${id}`, data),
  remove: (id: number) => http.delete<any, void>(`/budgets/${id}`),
};

// 统计
export const statsApi = {
  overview: (params: any) => http.get<any, StatisticsData>('/statistics', { params }),
  assets:   () => http.get<any, AssetOverview>('/statistics/assets'),
  timeline: (params?: any) => http.get<any, AssetPoint[]>('/statistics/assets/timeline', { params }),
};

// 存钱计划
export const savingApi = {
  list:   () => http.get<any, SavingPlan[]>('/saving-plans'),
  create: (data: any) => http.post<any, SavingPlan>('/saving-plans', data),
  update: (id: number, data: any) => http.put<any, SavingPlan>(`/saving-plans/${id}`, data),
  remove: (id: number) => http.delete<any, void>(`/saving-plans/${id}`),
  addRecord: (id: number, data: any) => http.post<any, any>(`/saving-plans/${id}/records`, data),
};

// 周期记账
export const recurringApi = {
  list:   () => http.get<any, Recurring[]>('/recurring'),
  create: (data: any) => http.post<any, Recurring>('/recurring', data),
  toggle: (id: number) => http.post<any, Recurring>(`/recurring/${id}/toggle`),
  remove: (id: number) => http.delete<any, void>(`/recurring/${id}`),
};

// 分期
export const installmentApi = {
  list:   () => http.get<any, Installment[]>('/installments'),
  create: (data: any) => http.post<any, Installment>('/installments', data),
  remove: (id: number) => http.delete<any, void>(`/installments/${id}`),
};

// 报销
export const reimbApi = {
  list:   () => http.get<any, Reimbursement[]>('/reimbursements'),
  create: (data: any) => http.post<any, Reimbursement>('/reimbursements', data),
  update: (id: number, data: any) => http.put<any, Reimbursement>(`/reimbursements/${id}`, data),
  remove: (id: number) => http.delete<any, void>(`/reimbursements/${id}`),
};

// 导入导出
export const ioApi = {
  template: () => window.open('/api/io/template'),
  exportCSV: (params?: any) => {
    const q = new URLSearchParams(params as any).toString();
    window.open('/api/io/export?' + q);
  },
  import: async (source: string, book_id: number, file: File) => {
    const fd = new FormData();
    fd.append('file', file);
    return http.post<any, { created: number; skipped: number; total: number }>(
      `/io/import?source=${source}&book_id=${book_id}`, fd,
      { headers: { 'Content-Type': 'multipart/form-data' } },
    );
  },
};
