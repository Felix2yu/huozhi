/**
 * 常见银行 / 消费金融机构品牌标识。
 * 无真实 logo 资源，采用业界通行做法：品牌主色圆标 + 品牌短字
 * （如工商银行红色「工」、建设银行蓝色「建」），用于银行卡卡面与账户列表图标。
 * keys 按包含关系匹配，更具体的关键词须排在前面（如「工商银行」在「中国银行」之前）。
 */
export interface BankBrand {
  short: string;
  color: string;
  name: string;
  /** @icongo/bl 图标组件名（BankMark 静态注册表里需有对应项），缺省时回退短字圆标 */
  icon?: string;
}

const BANK_BRANDS: (BankBrand & { keys: string[] })[] = [
  { keys: ['工商银行', '工行', 'ICBC'], short: '工', color: '#C8000F', name: '中国工商银行', icon: 'BLIcbc' },
  { keys: ['建设银行', '建行', 'CCB'], short: '建', color: '#0066B3', name: '中国建设银行', icon: 'BLCcb' },
  { keys: ['农业银行', '农行', 'ABC'], short: '农', color: '#009A44', name: '中国农业银行', icon: 'BLAbchina' },
  { keys: ['交通银行', '交行', 'BOCOM'], short: '交', color: '#003E7E', name: '交通银行', icon: 'BLBankcomm' },
  { keys: ['中国银行', '中行', 'BOC'], short: '中', color: '#AE1F24', name: '中国银行', icon: 'BLBoc' },
  { keys: ['招商银行', '招行', 'CMB'], short: '招', color: '#C8102E', name: '招商银行', icon: 'BLCmbchina' },
  { keys: ['邮储', '邮政储蓄', '邮政', 'PSBC'], short: '邮', color: '#007A3D', name: '中国邮政储蓄银行', icon: 'PSBC' },
  { keys: ['中信银行', '中信', 'CITIC'], short: '信', color: '#CC0000', name: '中信银行', icon: 'BLCiticbank' },
  { keys: ['浦发银行', '浦发', 'SPDB'], short: '浦', color: '#005BAC', name: '浦发银行' },
  { keys: ['民生银行', '民生', 'CMBC'], short: '民', color: '#008B8B', name: '民生银行', icon: 'BLCmbc' },
  { keys: ['光大银行', '光大', 'CEB'], short: '光', color: '#6A1B86', name: '中国光大银行', icon: 'BLCebbank' },
  { keys: ['平安银行', '平安', 'PAB'], short: '平', color: '#F58220', name: '平安银行', icon: 'PINGANBANK' },
  { keys: ['华夏银行', '华夏', 'HXB'], short: '华', color: '#C8000F', name: '华夏银行', icon: 'BLHxb' },
  { keys: ['广发银行', '广发', 'CGB'], short: '广', color: '#C8102E', name: '广发银行', icon: 'BLCgbchina' },
  { keys: ['兴业银行', '兴业', 'CIB'], short: '兴', color: '#00539F', name: '兴业银行', icon: 'BLCib' },
  { keys: ['浙商银行', '浙商'], short: '浙', color: '#1B4B9B', name: '浙商银行', icon: 'BLCzbank' },
  { keys: ['渤海银行', '渤海'], short: '渤', color: '#00A0E9', name: '渤海银行' },
  { keys: ['恒丰银行', '恒丰'], short: '恒', color: '#C7000B', name: '恒丰银行' },
  { keys: ['北京银行'], short: '北', color: '#C8102E', name: '北京银行', icon: 'BLBankofbeijing' },
  { keys: ['上海银行'], short: '上', color: '#005BAC', name: '上海银行' },
  { keys: ['宁波银行'], short: '甬', color: '#C8102E', name: '宁波银行' },
  { keys: ['江苏银行'], short: '苏', color: '#C8102E', name: '江苏银行', icon: 'BLJshbank' },
  { keys: ['南京银行'], short: '宁', color: '#C8102E', name: '南京银行' },
  { keys: ['杭州银行'], short: '杭', color: '#C8102E', name: '杭州银行' },
  { keys: ['微众银行', '微众', 'WeBank'], short: '微', color: '#0052D9', name: '微众银行' },
  { keys: ['网商银行', '网商', 'MYbank'], short: '网', color: '#FF6A00', name: '网商银行' },
  { keys: ['新网银行', '新网'], short: '新', color: '#E60027', name: '新网银行' },
  // 消费金融 / 支付平台
  { keys: ['花呗'], short: '花', color: '#6C5CE7', name: '花呗' },
  { keys: ['借呗'], short: '借', color: '#1677FF', name: '借呗' },
  { keys: ['白条', '京东', 'JD'], short: '京', color: '#E1251B', name: '京东', icon: 'JDFINANCE' },
  { keys: ['美团'], short: '美', color: '#FFD100', name: '美团', icon: 'MEITUAN' },
  { keys: ['支付宝', '余额宝'], short: '支', color: '#1677FF', name: '支付宝', icon: 'ALIPAY' },
  { keys: ['微信', '零钱', 'WeChat'], short: '微', color: '#07C160', name: '微信支付', icon: 'WECHAT' },
  { keys: ['银联', '云闪付', 'UnionPay'], short: '联', color: '#D10429', name: '中国银联', icon: 'UNIONPAY' },
  { keys: ['携程'], short: '携', color: '#2577E3', name: '携程' },
  { keys: ['滴滴'], short: '滴', color: '#FF7E33', name: '滴滴' },
  { keys: ['抖音', '字节'], short: '抖', color: '#161823', name: '抖音' },
  { keys: ['度小满'], short: '度', color: '#2932E1', name: '度小满' },
];

/**
 * 从账户名 / 银行名中识别银行品牌，未命中返回 null。
 */
export function resolveBankBrand(text?: string): BankBrand | null {
  if (!text) return null;
  const t = text.trim();
  if (!t) return null;
  const lower = t.toLowerCase();
  for (const b of BANK_BRANDS) {
    if (b.keys.some(k => lower.includes(k.toLowerCase()))) {
      return { short: b.short, color: b.color, name: b.name, icon: b.icon };
    }
  }
  return null;
}

// 品牌表未收录时的兜底色板（柔和色，按名称哈希取色保证稳定）
const FALLBACK_COLORS = ['#475569', '#6366F1', '#0EA5E9', '#14B8A6', '#F59E0B', '#EC4899', '#8B5CF6', '#64748B'];

/**
 * 银行徽标：优先品牌表（品牌短字 + 品牌色），
 * 未收录的机构取名称首字 + 哈希色兜底，保证所有银行卡都有标识。
 */
export function bankBadge(text?: string): { short: string; color: string; name: string } | null {
  const brand = resolveBankBrand(text);
  if (brand) return brand;
  const t = (text || '').trim();
  if (!t) return null;
  let h = 0;
  for (let i = 0; i < t.length; i++) h = (h * 31 + t.charCodeAt(i)) >>> 0;
  return { short: t.slice(0, 1), color: FALLBACK_COLORS[h % FALLBACK_COLORS.length], name: t };
}
