/**
 * 银行/机构 SVG 图标资源表。
 *
 * 三个组件（AccountIcon / IconPicker / 银行卡页）此前各自写了一份
 * `import.meta.glob` + 手工拼 key（`/src/lib/assets/bank-icons/<id>.svg`），
 * 一旦拼错或传入的 icon 不是图标 id（例如历史数据里的 emoji 💳/💵），
 * 就会静默回退到「首字头像」，表现为「选了图标也不生效」。
 *
 * 这里统一收口：
 *  - 用 basename 建索引，不再依赖 glob key 的具体前缀（dev / build / 不同 Vite 版本都稳）；
 *  - 提供 resolveAccountIcon()，明确区分「SVG 图标」「字面量字形（emoji）」「无图标」三种结果。
 */
import { getBankIcon } from './bank-themes';

const rawIcons = import.meta.glob<string>('$lib/assets/bank-icons/*.svg', {
	eager: true,
	query: '?url',
	import: 'default'
});

/** id（文件名去扩展名）→ 打包后的 URL */
export const bankIconUrls: Record<string, string> = {};
for (const [key, url] of Object.entries(rawIcons)) {
	const id = key.split('/').pop()?.replace(/\.svg$/, '') ?? '';
	if (id) bankIconUrls[id] = url;
}

/** 取图标 URL；id 不是有效图标时返回 null */
export function bankIconUrl(id?: string | null): string | null {
	if (!id) return null;
	return bankIconUrls[id] ?? null;
}

export function isBankIconId(id?: string | null): boolean {
	return bankIconUrl(id) !== null;
}

export interface IconInput {
	icon?: string | null;
	bankName?: string | null;
	name?: string | null;
	type?: string | null;
}

export type ResolvedIcon =
	| { kind: 'svg'; id: string; url: string }
	| { kind: 'glyph'; text: string }
	| null;

/** 按账户类型给的通用图标，避免「自动」完全无结果时退化成首字 */
const TYPE_FALLBACK: Record<string, string> = {
	bank: 'bank-card',
	credit: 'credit-card'
};

/** 图标 id 的形态：纯 ASCII 的 slug；emoji / 中文等一律不是 id */
const ICON_ID_RE = /^[A-Za-z0-9._-]+$/;

/**
 * 解析账户最终要显示的图标，优先级：
 *   1. 手动 icon —— 命中 SVG 表则用图标；
 *      非 id 形态的值（emoji 💳/💵 等历史数据）按字面量渲染；
 *      id 形态但表中已不存在（改名/下线的旧 id）视为失效，继续往下走自动识别；
 *   2. 自动识别（bank_name → name）
 *   3. 账户类型兜底（银行卡 / 信用卡）
 *   4. null —— 交给调用方渲染首字头像
 */
export function resolveAccountIcon(input: IconInput): ResolvedIcon {
	const manual = (input.icon ?? '').trim();
	if (manual) {
		const url = bankIconUrl(manual);
		if (url) return { kind: 'svg', id: manual, url };
		if (!ICON_ID_RE.test(manual)) return { kind: 'glyph', text: manual };
	}

	const src = (input.bankName ?? '').trim() || (input.name ?? '').trim();
	const autoId = getBankIcon(src);
	const autoUrl = bankIconUrl(autoId);
	if (autoUrl) return { kind: 'svg', id: autoId as string, url: autoUrl };

	const typeFallback = TYPE_FALLBACK[(input.type ?? '').trim()];
	const fallbackUrl = bankIconUrl(typeFallback);
	if (fallbackUrl) return { kind: 'svg', id: typeFallback, url: fallbackUrl };

	return null;
}
