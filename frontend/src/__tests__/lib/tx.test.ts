import { describe, it, expect } from 'vitest';
import {
	statsBucket,
	baseAmount,
	amountDisplay,
	toneClass,
	typeLabel,
	recomputeDaySubtotal,
	round2,
	highlightSegments,
	firstGrapheme,
	clampMoneyInput
} from '$lib/utils/tx';

describe('statsBucket', () => {
	it('收入类归入income', () => {
		expect(statsBucket('income')).toBe('income');
		expect(statsBucket('refund')).toBe('income');
	});

	it('支出类归入expense', () => {
		expect(statsBucket('expense')).toBe('expense');
		expect(statsBucket('reimburse')).toBe('expense');
	});

	it('其他类型返回空字符串', () => {
		expect(statsBucket('transfer')).toBe('');
		expect(statsBucket('adjust')).toBe('');
		expect(statsBucket('unknown')).toBe('');
	});
});

describe('baseAmount', () => {
	it('优先使用amount_base', () => {
		expect(baseAmount({ id: 1, type: 'expense', amount: 100, amount_base: 50 })).toBe(50);
	});

	it('无amount_base时使用汇率折算', () => {
		expect(baseAmount({ id: 1, type: 'expense', amount: 100, exchange_rate: 7.5 })).toBe(750);
	});

	it('汇率为1时直接返回amount', () => {
		expect(baseAmount({ id: 1, type: 'expense', amount: 100, exchange_rate: 1 })).toBe(100);
	});

	it('无汇率时直接返回amount', () => {
		expect(baseAmount({ id: 1, type: 'expense', amount: 100 })).toBe(100);
	});
});

describe('amountDisplay', () => {
	it('收入显示+号和income色调', () => {
		const result = amountDisplay({ id: 1, type: 'income', amount: 100 });
		expect(result.sign).toBe('+');
		expect(result.abs).toBe(100);
		expect(result.tone).toBe('income');
	});

	it('支出显示-号和expense色调', () => {
		const result = amountDisplay({ id: 1, type: 'expense', amount: 100 });
		expect(result.sign).toBe('-');
		expect(result.abs).toBe(100);
		expect(result.tone).toBe('expense');
	});

	it('转账显示空符号和muted色调', () => {
		const result = amountDisplay({ id: 1, type: 'transfer', amount: 100 });
		expect(result.sign).toBe('');
		expect(result.abs).toBe(100);
		expect(result.tone).toBe('muted');
	});
});

describe('toneClass', () => {
	it('income返回收入颜色', () => {
		expect(toneClass('income')).toBe('text-[var(--color-income)]');
	});

	it('expense返回支出颜色', () => {
		expect(toneClass('expense')).toBe('text-[var(--color-expense)]');
	});

	it('muted返回灰色', () => {
		expect(toneClass('muted')).toBe('text-muted-foreground');
	});
});

describe('typeLabel', () => {
	it('返回正确的中文名', () => {
		expect(typeLabel('income')).toBe('收入');
		expect(typeLabel('expense')).toBe('支出');
		expect(typeLabel('transfer')).toBe('转账');
		expect(typeLabel('refund')).toBe('退款');
		expect(typeLabel('reimburse')).toBe('报销');
		expect(typeLabel('adjust')).toBe('余额调整');
	});

	it('未知类型返回原值', () => {
		expect(typeLabel('unknown')).toBe('unknown');
	});
});

describe('recomputeDaySubtotal', () => {
	it('计算日小计', () => {
		const day = {
			day_income: 0,
			day_expense: 0,
			day_balance: 0,
			transactions: [
				{ id: 1, type: 'income', amount: 100 },
				{ id: 2, type: 'expense', amount: 30 },
				{ id: 3, type: 'expense', amount: 20 }
			]
		};
		recomputeDaySubtotal(day);
		expect(day.day_income).toBe(100);
		expect(day.day_expense).toBe(50);
		expect(day.day_balance).toBe(50);
	});

	it('忽略exclude_in_balance交易', () => {
		const day = {
			day_income: 0,
			day_expense: 0,
			day_balance: 0,
			transactions: [
				{ id: 1, type: 'income', amount: 100 },
				{ id: 2, type: 'expense', amount: 30, include_in_balance: false }
			]
		};
		recomputeDaySubtotal(day);
		expect(day.day_income).toBe(100);
		expect(day.day_expense).toBe(0);
		expect(day.day_balance).toBe(100);
	});
});

describe('round2', () => {
	it('四舍五入到两位小数', () => {
		expect(round2(1.235)).toBe(1.24);
		expect(round2(1.234)).toBe(1.23);
		expect(round2(1.1)).toBe(1.1);
	});

	it('处理非数字', () => {
		expect(round2(NaN)).toBe(0);
		expect(round2(undefined as any)).toBe(0);
	});
});

describe('highlightSegments', () => {
	it('空关键词返回原文', () => {
		const result = highlightSegments('hello', '');
		expect(result).toEqual([{ text: 'hello', hit: false }]);
	});

	it('空文本返回空数组', () => {
		expect(highlightSegments('', 'test')).toEqual([]);
	});

	it('高亮匹配的文本', () => {
		const result = highlightSegments('Hello World', 'world');
		expect(result).toEqual([
			{ text: 'Hello ', hit: false },
			{ text: 'World', hit: true }
		]);
	});

	it('大小写不敏感', () => {
		const result = highlightSegments('Hello World', 'HELLO');
		expect(result[0].hit).toBe(true);
	});

	it('无匹配返回原文', () => {
		const result = highlightSegments('Hello World', 'xyz');
		expect(result).toEqual([{ text: 'Hello World', hit: false }]);
	});
});

describe('firstGrapheme', () => {
	it('返回首字符', () => {
		expect(firstGrapheme('Hello')).toBe('H');
		expect(firstGrapheme('你好')).toBe('你');
	});

	it('空字符串返回默认值', () => {
		expect(firstGrapheme('')).toBe('¥');
	});

	it('自定义默认值', () => {
		expect(firstGrapheme('', '$')).toBe('$');
	});
});

describe('clampMoneyInput', () => {
	it('截断到两位小数', () => {
		expect(clampMoneyInput('1.234')).toBe('1.23');
		expect(clampMoneyInput('1.23')).toBe('1.23');
	});

	it('处理负数', () => {
		expect(clampMoneyInput('-1.234')).toBe('-1.23');
	});

	it('空字符串返回空', () => {
		expect(clampMoneyInput('')).toBe('');
	});

	it('移除非数字字符', () => {
		expect(clampMoneyInput('abc123')).toBe('123');
	});
});
