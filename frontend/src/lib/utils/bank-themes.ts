// 银行主题色映射
import type { AccountType } from '$lib/types';

export interface BankTheme {
	color: string;
}

const bankThemes: Record<string, BankTheme> = {
	// 国有大行
	'中国工商银行': { color: '#C41230' },
	'工商银行': { color: '#C41230' },
	'ICBC': { color: '#C41230' },
	'中国农业银行': { color: '#009944' },
	'农业银行': { color: '#009944' },
	'ABC': { color: '#009944' },
	'中国银行': { color: '#C10A27' },
	'中行': { color: '#C10A27' },
	'BOC': { color: '#C10A27' },
	'中国建设银行': { color: '#004EA2' },
	'建设银行': { color: '#004EA2' },
	'CCB': { color: '#004EA2' },
	'交通银行': { color: '#1B3F8B' },
	'交行': { color: '#1B3F8B' },
	'BOCOM': { color: '#1B3F8B' },
	'中国邮政储蓄银行': { color: '#007B3E' },
	'邮储银行': { color: '#007B3E' },
	'PSBC': { color: '#007B3E' },

	// 股份制银行
	'招商银行': { color: '#E60012' },
	'招行': { color: '#E60012' },
	'CMB': { color: '#E60012' },
	'中信银行': { color: '#E4002B' },
	'中信': { color: '#E4002B' },
	'CITIC': { color: '#E4002B' },
	'浦发银行': { color: '#0066CC' },
	'浦发': { color: '#0066CC' },
	'SPDB': { color: '#0066CC' },
	'民生银行': { color: '#00A0E9' },
	'民生': { color: '#00A0E9' },
	'CMBC': { color: '#00A0E9' },
	'兴业银行': { color: '#003DA5' },
	'兴业': { color: '#003DA5' },
	'CIB': { color: '#003DA5' },
	'光大银行': { color: '#7D2027' },
	'光大': { color: '#7D2027' },
	'CEB': { color: '#7D2027' },
	'平安银行': { color: '#FF6600' },
	'平安': { color: '#FF6600' },
	'PAB': { color: '#FF6600' },
	'华夏银行': { color: '#C10A27' },
	'华夏': { color: '#C10A27' },
	'HXB': { color: '#C10A27' },
	'广发银行': { color: '#D4232E' },
	'广发': { color: '#D4232E' },
	'CGB': { color: '#D4232E' },

	// 互联网银行
	'微众银行': { color: '#07C160' },
	'微众': { color: '#07C160' },
	'WeBank': { color: '#07C160' },
	'网商银行': { color: '#1677FF' },
	'网商': { color: '#1677FF' },
	'MYbank': { color: '#1677FF' },

	// 外资银行
	'花旗银行': { color: '#003DA5' },
	'花旗': { color: '#003DA5' },
	'Citi': { color: '#003DA5' },
	'汇丰银行': { color: '#DB0011' },
	'汇丰': { color: '#DB0011' },
	'HSBC': { color: '#DB0011' },
	'渣打银行': { color: '#007A33' },
	'渣打': { color: '#007A33' },
	'Standard Chartered': { color: '#007A33' },

	// 信用卡品牌
	'VISA': { color: '#1A1F71' },
	'Visa': { color: '#1A1F71' },
	'Mastercard': { color: '#EB001B' },
	'万事达': { color: '#EB001B' },
	'银联': { color: '#E21836' },
	'UnionPay': { color: '#E21836' },
	'American Express': { color: '#006FCF' },
	'运通': { color: '#006FCF' },
	'JCB': { color: '#0B4EA2' },
};

// 默认主题（找不到匹配时使用）
const defaultTheme: BankTheme = { color: '#6B7280' };

export function getBankTheme(bankName?: string): BankTheme {
	if (!bankName) return defaultTheme;
	const trimmed = bankName.trim();

	// 精确匹配
	if (bankThemes[trimmed]) return bankThemes[trimmed];

	// 模糊匹配（包含关键词）
	for (const [key, theme] of Object.entries(bankThemes)) {
		if (trimmed.includes(key) || key.includes(trimmed)) {
			return theme;
		}
	}

	return defaultTheme;
}

// 生成卡片渐变背景
export function getCardGradient(bankName?: string): string {
	const theme = getBankTheme(bankName);
	const color = theme.color;
	return `linear-gradient(135deg, ${color}, ${adjustBrightness(color, -30)})`;
}

// 调整颜色亮度
function adjustBrightness(hex: string, percent: number): string {
	const num = parseInt(hex.replace('#', ''), 16);
	const amt = Math.round(2.55 * percent);
	const R = Math.max(0, Math.min(255, (num >> 16) + amt));
	const G = Math.max(0, Math.min(255, ((num >> 8) & 0x00FF) + amt));
	const B = Math.max(0, Math.min(255, (num & 0x0000FF) + amt));
	return `#${(0x1000000 + R * 0x10000 + G * 0x100 + B).toString(16).slice(1)}`;
}

// ============ 品牌简称 / 识别（用于账户头像、名称自动填充） ============

interface BankMeta {
	names: string[]; // 匹配关键词（含别名/英文），顺序：最具体在前
	short: string; // 头像显示的 1 字简称
}

// 覆盖主流银行 + 支付宝 + 微信 + 常见互联网/消费金融机构
const BANK_META: BankMeta[] = [
	{ names: ['招商银行', '招行', 'CMB'], short: '招' },
	{ names: ['工商银行', '工行', 'ICBC'], short: '工' },
	{ names: ['建设银行', '建行', 'CCB'], short: '建' },
	{ names: ['农业银行', '农行', 'ABC'], short: '农' },
	{ names: ['中国银行', '中行', 'BOC'], short: '中' },
	{ names: ['交通银行', '交行', 'BOCOM'], short: '交' },
	{ names: ['邮储银行', '邮政储蓄', 'PSBC'], short: '邮' },
	{ names: ['中信银行', '中信', 'CITIC'], short: '信' },
	{ names: ['浦发银行', '浦发', 'SPDB'], short: '浦' },
	{ names: ['民生银行', '民生', 'CMBC'], short: '民' },
	{ names: ['兴业银行', '兴业', 'CIB'], short: '兴' },
	{ names: ['光大银行', '光大', 'CEB'], short: '光' },
	{ names: ['平安银行', '平安', 'PAB'], short: '平' },
	{ names: ['华夏银行', '华夏', 'HXB'], short: '华' },
	{ names: ['广发银行', '广发', 'CGB'], short: '广' },
	{ names: ['浙商银行'], short: '浙' },
	{ names: ['渤海银行'], short: '渤' },
	{ names: ['北京银行'], short: '京' },
	{ names: ['上海银行'], short: '沪' },
	{ names: ['南京银行'], short: '宁' },
	{ names: ['宁波银行'], short: '甬' },
	{ names: ['杭州银行'], short: '杭' },
	{ names: ['微众银行', '微众', 'WeBank'], short: '微' },
	{ names: ['网商银行', '网商', 'MYbank'], short: '网' },
	{ names: ['花旗银行', '花旗', 'Citi'], short: '花' },
	{ names: ['汇丰银行', '汇丰', 'HSBC'], short: '汇' },
	{ names: ['渣打银行', '渣打', 'Standard Chartered'], short: '渣' },
	{ names: ['支付宝'], short: '支' },
	{ names: ['微信', '微信支付', '财付通'], short: '微' },
	{ names: ['花呗', '借呗', '蚂蚁'], short: '蚂' },
	{ names: ['京东', '京东金融'], short: '京' },
	{ names: ['美团', '美团支付'], short: '美' },
	{ names: ['云闪付', '银联', 'UnionPay'], short: '银' },
	{ names: ['Apple', '苹果'], short: '' }
];

export interface BankBrand {
	short: string; // 头像简称，空串时由调用方兜底
	color: string; // 品牌色
}

// 根据银行名/账户名识别品牌（颜色复用 bank-themes 的主题色）
export function getBankBrand(text?: string): BankBrand {
	const t = (text || '').trim();
	if (!t) return { short: '?', color: defaultTheme.color };
	for (const b of BANK_META) {
		if (b.names.some((n) => t.includes(n))) {
			const color = getBankTheme(b.names[0]).color;
			return { short: b.short || t.slice(0, 1), color };
		}
	}
	// 兜底：用首字 + 主题色（能识别则取色，否则灰色）
	return { short: t.slice(0, 1), color: getBankTheme(t).color };
}

// 从账户名称推断银行/机构名称（例如「招商银行信用卡」→「招商银行」）
export function detectBankName(text?: string): string {
	const t = (text || '').trim();
	if (!t) return '';
	for (const b of BANK_META) {
		if (b.names.some((n) => t.includes(n))) return b.names[0];
	}
	return '';
}

// 从账户名称推断账户类型（仅对明显的虚拟/信用/银行卡生效，其余留空让用户选）
export function detectAccountType(text?: string): AccountType | '' {
	const t = (text || '').trim();
	if (/支付宝|微信|财付通|京东|美团|云闪付|Apple|苹果|网商|微众/.test(t)) return 'virtual';
	if (/信用卡/.test(t)) return 'credit';
	if (/储蓄卡|借记卡|银行卡|银行/.test(t)) return 'bank';
	return '';
}
