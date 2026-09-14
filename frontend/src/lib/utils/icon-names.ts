// 图标文件名到中文名称的映射
export const iconNames: Record<string, string> = {
	// 国有大行
	'icbc': '工商银行',
	'abchina': '农业银行',
	'boc': '中国银行',
	'ccb': '建设银行',
	'bankcomm': '交通银行',
	'psbc': '邮储银行',

	// 股份制银行
	'cmbchina': '招商银行',
	'citicbank': '中信银行',
	'spdb': '浦发银行',
	'cmbc': '民生银行',
	'cib': '兴业银行',
	'cebbank': '光大银行',
	'pingan': '平安银行',
	'hxb': '华夏银行',
	'cgbchina': '广发银行',
	'czbank': '浙商银行',
	'cbhb': '渤海银行',
	'hfbank': '恒丰银行',
	'bosc': '上海银行',
	'bankofbeijing': '北京银行',
	'njcb': '南京银行',
	'nbcb': '宁波银行',
	'hzbank': '杭州银行',

	// 互联网银行
	'webank': '微众银行',
	'mybank': '网商银行',

	// 外资银行
	'citibank': '花旗银行',
	'hsbc': '汇丰银行',
	'standardchartered': '渣打银行',
	'hangseng': '恒生银行',
	'hkbea': '东亚银行',
	'dbs': '星展银行',

	// 支付机构
	'alipay': '支付宝',
	'yuebao': '余额宝',
	'yulibao': '余利宝',
	'xiaohebao': '小荷包',
	'wechat-pay': '微信支付',
	'wechat-lingqiantong': '微信零钱通',
	'wechat-fenfu': '微信分付',
	'jd-pay': '京东支付',
	'jd-jinrong': '京东金融',
	'jd-baitiao': '京东白条',
	'meituan': '美团',
	'huabei': '花呗',
	'jiebei': '借呗',
	'douyin': '抖音',
	'unionpay': '银联',
	'qq-wallet': 'QQ钱包',
	'paypal': 'PayPal',
	'huawei-pay': '华为钱包',
	'duoduo-pay': '多多钱包',
	'digital-rmb': '数字人民币',
	'gongjijin': '公积金',
	'medical-insurance': '医保',

	// 充值账户
	'huafie': '话费',
	'shuidian': '水电',
	'fanka': '饭卡',
	'yajin': '押金',
	'gongjiao': '公交卡',
	'huiyuan': '会员卡',
	'jiayouka': '加油卡',
	'shihua': '石化钱包',
	'apple': 'Apple',
	'chongzhika': '充值卡',

	// 投资理财
	'stocks': '股票',
	'funds': '基金',
	'gold': '黄金',
	'forex': '外汇',
	'futures': '期货',
	'bonds': '债券',
	'fixed-income': '固定收益',
	'crypto': '加密货币',

	// 通用
	'cash': '现金',
	'credit-card': '信用卡',
	'bank-card': '银行卡',
};

export function getIconName(id: string): string {
	return iconNames[id] || id;
}
