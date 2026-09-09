// 银行主题色映射
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
