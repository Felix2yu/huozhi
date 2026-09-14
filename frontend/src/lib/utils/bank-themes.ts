// 银行主题色映射
import type { AccountType } from '$lib/types';

export interface BankTheme {
	color: string;
	icon?: string;
}

const bankThemes: Record<string, BankTheme> = {
	// 国有大行
	'中国工商银行': { color: '#C41230', icon: 'icbc' },
	'工商银行': { color: '#C41230', icon: 'icbc' },
	'ICBC': { color: '#C41230', icon: 'icbc' },
	'中国农业银行': { color: '#009944', icon: 'abchina' },
	'农业银行': { color: '#009944', icon: 'abchina' },
	'ABC': { color: '#009944', icon: 'abchina' },
	'中国银行': { color: '#C10A27', icon: 'boc' },
	'中行': { color: '#C10A27', icon: 'boc' },
	'BOC': { color: '#C10A27', icon: 'boc' },
	'中国建设银行': { color: '#004EA2', icon: 'ccb' },
	'建设银行': { color: '#004EA2', icon: 'ccb' },
	'CCB': { color: '#004EA2', icon: 'ccb' },
	'交通银行': { color: '#1B3F8B', icon: 'bankcomm' },
	'交行': { color: '#1B3F8B', icon: 'bankcomm' },
	'BOCOM': { color: '#1B3F8B', icon: 'bankcomm' },
	'中国邮政储蓄银行': { color: '#007B3E', icon: 'psbc' },
	'邮储银行': { color: '#007B3E', icon: 'psbc' },
	'PSBC': { color: '#007B3E', icon: 'psbc' },

	// 股份制银行
	'招商银行': { color: '#E60012', icon: 'cmbchina' },
	'招行': { color: '#E60012', icon: 'cmbchina' },
	'CMB': { color: '#E60012', icon: 'cmbchina' },
	'中信银行': { color: '#E4002B', icon: 'citicbank' },
	'中信': { color: '#E4002B', icon: 'citicbank' },
	'CITIC': { color: '#E4002B', icon: 'citicbank' },
	'浦发银行': { color: '#0066CC', icon: 'spdb' },
	'浦发': { color: '#0066CC', icon: 'spdb' },
	'SPDB': { color: '#0066CC', icon: 'spdb' },
	'民生银行': { color: '#00A0E9', icon: 'cmbc' },
	'民生': { color: '#00A0E9', icon: 'cmbc' },
	'CMBC': { color: '#00A0E9', icon: 'cmbc' },
	'兴业银行': { color: '#003DA5', icon: 'cib' },
	'兴业': { color: '#003DA5', icon: 'cib' },
	'CIB': { color: '#003DA5', icon: 'cib' },
	'光大银行': { color: '#7D2027', icon: 'cebbank' },
	'光大': { color: '#7D2027', icon: 'cebbank' },
	'CEB': { color: '#7D2027', icon: 'cebbank' },
	'平安银行': { color: '#FF6600', icon: 'pingan' },
	'平安': { color: '#FF6600', icon: 'pingan' },
	'PAB': { color: '#FF6600', icon: 'pingan' },
	'华夏银行': { color: '#C10A27', icon: 'hxb' },
	'华夏': { color: '#C10A27', icon: 'hxb' },
	'HXB': { color: '#C10A27', icon: 'hxb' },
	'广发银行': { color: '#D4232E', icon: 'cgbchina' },
	'广发': { color: '#D4232E', icon: 'cgbchina' },
	'CGB': { color: '#D4232E', icon: 'cgbchina' },

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

	// 支付机构
	'支付宝': { color: '#1677FF', icon: 'alipay' },
	'余额宝': { color: '#1677FF', icon: 'yuebao' },
	'余利宝': { color: '#1677FF', icon: 'yulibao' },
	'小荷包': { color: '#1677FF', icon: 'xiaohebao' },
	'微信': { color: '#07C160', icon: 'wechat-pay' },
	'微信支付': { color: '#07C160', icon: 'wechat-pay' },
	'微信零钱通': { color: '#10B981', icon: 'wechat-lingqiantong' },
	'微信分付': { color: '#07C160', icon: 'wechat-fenfu' },
	'财付通': { color: '#07C160', icon: 'wechat-pay' },
	'京东': { color: '#E4393C', icon: 'jd-pay' },
	'京东金融': { color: '#E4393C', icon: 'jd-jinrong' },
	'京东白条': { color: '#E4393C', icon: 'jd-baitiao' },
	'美团': { color: '#FFD100', icon: 'meituan' },
	'美团月付': { color: '#FFD100', icon: 'meituan' },
	'花呗': { color: '#1677FF', icon: 'huabei' },
	'借呗': { color: '#1677FF', icon: 'jiebei' },
	'抖音': { color: '#000000', icon: 'douyin' },
	'抖音月付': { color: '#000000', icon: 'douyin' },
	'云闪付': { color: '#E21836', icon: 'unionpay' },
	'银联': { color: '#E21836', icon: 'unionpay' },
	'UnionPay': { color: '#E21836', icon: 'unionpay' },
	'QQ钱包': { color: '#E21836', icon: 'qq-wallet' },
	'Paypal': { color: '#003087', icon: 'paypal' },
	'PayPal': { color: '#003087', icon: 'paypal' },
	'华为钱包': { color: '#000000', icon: 'huawei-pay' },
	'多多钱包': { color: '#E4393C', icon: 'duoduo-pay' },
	'数字人民币': { color: '#1677FF', icon: 'digital-rmb' },
	'公积金': { color: '#009944', icon: 'gongjijin' },
	'医保': { color: '#009944', icon: 'medical-insurance' },

	// 充值账户
	'话费': { color: '#10B981', icon: 'huafie' },
	'水电': { color: '#3B82F6', icon: 'shuidian' },
	'饭卡': { color: '#F59E0B', icon: 'fanka' },
	'押金': { color: '#6B7280', icon: 'yajin' },
	'公交卡': { color: '#10B981', icon: 'gongjiao' },
	'会员卡': { color: '#8B5CF6', icon: 'huiyuan' },
	'加油卡': { color: '#EF4444', icon: 'jiayouka' },
	'石化钱包': { color: '#DC2626', icon: 'shihua' },
	'Apple': { color: '#000000', icon: 'apple' },
	'苹果账户': { color: '#000000', icon: 'apple' },

	// 投资理财
	'股票': { color: '#EF4444', icon: 'stocks' },
	'基金': { color: '#3B82F6', icon: 'funds' },
	'黄金': { color: '#F59E0B', icon: 'gold' },
	'外汇': { color: '#10B981', icon: 'forex' },
	'期货': { color: '#8B5CF6', icon: 'futures' },
	'债券': { color: '#06B6D4', icon: 'bonds' },
	'固定收益': { color: '#10B981', icon: 'fixed-income' },
	'加密货币': { color: '#F59E0B', icon: 'crypto' },

	// 信用卡品牌
	'VISA': { color: '#1A1F71' },
	'Visa': { color: '#1A1F71' },
	'Mastercard': { color: '#EB001B' },
	'万事达': { color: '#EB001B' },
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
	// 支付机构
	{ names: ['支付宝'], short: '支' },
	{ names: ['余额宝'], short: '余' },
	{ names: ['余利宝'], short: '余' },
	{ names: ['小荷包'], short: '荷' },
	{ names: ['微信', '微信支付', '财付通'], short: '微' },
	{ names: ['微信零钱通'], short: '微' },
	{ names: ['微信分付'], short: '微' },
	{ names: ['花呗', '借呗', '蚂蚁'], short: '蚂' },
	{ names: ['京东', '京东金融'], short: '京' },
	{ names: ['京东白条'], short: '京' },
	{ names: ['美团', '美团支付', '美团月付'], short: '美' },
	{ names: ['抖音', '抖音月付'], short: '抖' },
	{ names: ['云闪付', '银联', 'UnionPay'], short: '银' },
	{ names: ['QQ钱包', 'QQ'], short: 'Q' },
	{ names: ['Paypal', 'PayPal'], short: 'P' },
	{ names: ['华为钱包', '华为'], short: '华' },
	{ names: ['多多钱包'], short: '多' },
	{ names: ['数字人民币'], short: '数' },
	{ names: ['公积金'], short: '公' },
	{ names: ['医保'], short: '医' },
	// 充值账户
	{ names: ['话费'], short: '话' },
	{ names: ['水电'], short: '水' },
	{ names: ['饭卡'], short: '饭' },
	{ names: ['押金'], short: '押' },
	{ names: ['公交卡'], short: '公' },
	{ names: ['会员卡'], short: '会' },
	{ names: ['加油卡'], short: '油' },
	{ names: ['石化钱包', '石化'], short: '石' },
	{ names: ['Apple', '苹果', '苹果账户'], short: '' },
	// 投资理财
	{ names: ['股票'], short: '股' },
	{ names: ['基金'], short: '基' },
	{ names: ['黄金'], short: '金' },
	{ names: ['外汇'], short: '汇' },
	{ names: ['期货'], short: '期' },
	{ names: ['债券'], short: '债' },
	{ names: ['固定收益'], short: '固' },
	{ names: ['加密货币'], short: '币' },
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
	if (/支付宝|微信|财付通|京东|美团|云闪付|Apple|苹果|网商|微众|花呗|借呗|抖音|QQ|Paypal|PayPal|华为|多多|数字人民币|公积金|医保/.test(t)) return 'virtual';
	if (/信用卡|花呗|借呗|京东白条|美团月付|抖音月付|微信分付/.test(t)) return 'credit';
	if (/储蓄卡|借记卡|银行卡|银行/.test(t)) return 'bank';
	if (/话费|水电|饭卡|押金|公交卡|会员卡|加油卡|石化|充值/.test(t)) return 'prepaid';
	if (/股票|基金|黄金|外汇|期货|债券|固定收益|加密货币/.test(t)) return 'investment';
	return '';
}

// 获取银行/支付机构的Logo文件名（不含扩展名）
export function getBankIcon(bankName?: string): string | null {
	const theme = getBankTheme(bankName);
	return theme.icon || null;
}
