// 银行主题色 + logo 映射
export interface BankTheme {
	color: string;
	logo: string; // emoji logo
}

const bankThemes: Record<string, BankTheme> = {
	// 国有大行
	'中国工商银行': { color: '#C41230', logo: '🔴' },
	'工商银行': { color: '#C41230', logo: '🔴' },
	'ICBC': { color: '#C41230', logo: '🔴' },
	'中国农业银行': { color: '#009944', logo: '🟢' },
	'农业银行': { color: '#009944', logo: '🟢' },
	'ABC': { color: '#009944', logo: '🟢' },
	'中国银行': { color: '#C10A27', logo: '🔴' },
	'中行': { color: '#C10A27', logo: '🔴' },
	'BOC': { color: '#C10A27', logo: '🔴' },
	'中国建设银行': { color: '#004EA2', logo: '🔵' },
	'建设银行': { color: '#004EA2', logo: '🔵' },
	'CCB': { color: '#004EA2', logo: '🔵' },
	'交通银行': { color: '#1B3F8B', logo: '🔵' },
	'交行': { color: '#1B3F8B', logo: '🔵' },
	'BOCOM': { color: '#1B3F8B', logo: '🔵' },
	'中国邮政储蓄银行': { color: '#007B3E', logo: '🟢' },
	'邮储银行': { color: '#007B3E', logo: '🟢' },
	'PSBC': { color: '#007B3E', logo: '🟢' },

	// 股份制银行
	'招商银行': { color: '#E60012', logo: '🔴' },
	'招行': { color: '#E60012', logo: '🔴' },
	'CMB': { color: '#E60012', logo: '🔴' },
	'中信银行': { color: '#E4002B', logo: '🔴' },
	'中信': { color: '#E4002B', logo: '🔴' },
	'CITIC': { color: '#E4002B', logo: '🔴' },
	'浦发银行': { color: '#0066CC', logo: '🔵' },
	'浦发': { color: '#0066CC', logo: '🔵' },
	'SPDB': { color: '#0066CC', logo: '🔵' },
	'民生银行': { color: '#00A0E9', logo: '🔵' },
	'民生': { color: '#00A0E9', logo: '🔵' },
	'CMBC': { color: '#00A0E9', logo: '🔵' },
	'兴业银行': { color: '#003DA5', logo: '🔵' },
	'兴业': { color: '#003DA5', logo: '🔵' },
	'CIB': { color: '#003DA5', logo: '🔵' },
	'光大银行': { color: '#7D2027', logo: '🟤' },
	'光大': { color: '#7D2027', logo: '🟤' },
	'CEB': { color: '#7D2027', logo: '🟤' },
	'平安银行': { color: '#FF6600', logo: '🟠' },
	'平安': { color: '#FF6600', logo: '🟠' },
	'PAB': { color: '#FF6600', logo: '🟠' },
	'华夏银行': { color: '#C10A27', logo: '🔴' },
	'华夏': { color: '#C10A27', logo: '🔴' },
	'HXB': { color: '#C10A27', logo: '🔴' },
	'广发银行': { color: '#D4232E', logo: '🔴' },
	'广发': { color: '#D4232E', logo: '🔴' },
	'CGB': { color: '#D4232E', logo: '🔴' },

	// 互联网银行
	'微众银行': { color: '#07C160', logo: '🟢' },
	'微众': { color: '#07C160', logo: '🟢' },
	'WeBank': { color: '#07C160', logo: '🟢' },
	'网商银行': { color: '#1677FF', logo: '🔵' },
	'网商': { color: '#1677FF', logo: '🔵' },
	'MYbank': { color: '#1677FF', logo: '🔵' },

	// 外资银行
	'花旗银行': { color: '#003DA5', logo: '🔵' },
	'花旗': { color: '#003DA5', logo: '🔵' },
	'Citi': { color: '#003DA5', logo: '🔵' },
	'汇丰银行': { color: '#DB0011', logo: '🔴' },
	'汇丰': { color: '#DB0011', logo: '🔴' },
	'HSBC': { color: '#DB0011', logo: '🔴' },
	'渣打银行': { color: '#007A33', logo: '🟢' },
	'渣打': { color: '#007A33', logo: '🟢' },
	'Standard Chartered': { color: '#007A33', logo: '🟢' },

	// 信用卡品牌
	'VISA': { color: '#1A1F71', logo: '💳' },
	'Visa': { color: '#1A1F71', logo: '💳' },
	'Mastercard': { color: '#EB001B', logo: '💳' },
	'万事达': { color: '#EB001B', logo: '💳' },
	'银联': { color: '#E21836', logo: '💳' },
	'UnionPay': { color: '#E21836', logo: '💳' },
	'American Express': { color: '#006FCF', logo: '💳' },
	'运通': { color: '#006FCF', logo: '💳' },
	'JCB': { color: '#0B4EA2', logo: '💳' },
};

// 默认主题（找不到匹配时使用）
const defaultTheme: BankTheme = { color: '#6B7280', logo: '💳' };

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
