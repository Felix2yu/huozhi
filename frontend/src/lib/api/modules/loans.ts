import { http } from '../http';
import type { Loan, LoanRepayment } from '$lib/types';

export interface CreateLoanPayload {
  direction: 'lend' | 'borrow';
  counterparty: string;
  principal: number; // 元
  currency?: string;
  interest_rate?: number;
  interest_type?: 'none' | 'simple' | 'monthly';
  account_id: number;
  book_id: number;
  loan_date: string;
  due_date?: string;
  note?: string;
}

export interface RepayLoanPayload {
  amount: number; // 本金（元）
  interest_amount?: number; // 利息（元）
  repay_account_id: number;
  repaid_at: string;
  note?: string;
}

export const loanApi = {
  list: () => http.get<Loan[]>('/loans'),
  create: (data: CreateLoanPayload) => http.post<Loan>('/loans', data),
  repay: (id: number, data: RepayLoanPayload) =>
    http.post<{ id: number; status: string; repaid_principal: number }>(
      `/loans/${id}/repay`,
      data
    ),
  update: (id: number, data: { status: LoanStatus; note?: string }) =>
    http.put<null>(`/loans/${id}`, data),
  remove: (id: number) => http.delete<void>(`/loans/${id}`)
};

export type { LoanRepayment };
