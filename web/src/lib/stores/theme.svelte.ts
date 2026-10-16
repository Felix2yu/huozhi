/** 主题状态 - Svelte 5 runes */
export type Theme = 'light' | 'dark' | 'system';

const STORAGE_KEY = 'hz_theme';

function getInitialTheme(): Theme {
	if (typeof localStorage !== 'undefined') {
		const stored = localStorage.getItem(STORAGE_KEY) as Theme | null;
		if (stored && ['light', 'dark', 'system'].includes(stored)) {
			return stored;
		}
	}
	return 'system';
}

// Svelte 5 rune
let theme = $state<Theme>(getInitialTheme());

function applyTheme(t: Theme) {
	const root = document.documentElement;
	const isDark =
		t === 'dark' ||
		(t === 'system' &&
			window.matchMedia('(prefers-color-scheme: dark)').matches);
	if (isDark) {
		root.classList.add('dark');
	} else {
		root.classList.remove('dark');
	}
}

// 初始化时应用
if (typeof document !== 'undefined') {
	applyTheme(theme);

	// 监听系统主题变化
	if (theme === 'system') {
		window
			.matchMedia('(prefers-color-scheme: dark)')
			.addEventListener('change', () => {
				applyTheme(theme);
			});
	}
}

let isDark = $derived(
	theme === 'dark' ||
		(theme === 'system' &&
			typeof window !== 'undefined' &&
			window.matchMedia('(prefers-color-scheme: dark)').matches)
);

export const themeStore = {
	get value() {
		return theme;
	},
	set value(t: Theme) {
		theme = t;
		if (typeof localStorage !== 'undefined') {
			localStorage.setItem(STORAGE_KEY, t);
		}
		if (typeof document !== 'undefined') {
			applyTheme(t);
		}
	},
	toggle() {
		this.value = theme === 'dark' ? 'light' : 'dark';
	},
	get isDark() {
		return isDark;
	}
};

/* ============ 界面字体大小 ============
 * Tailwind 的 text-* 与 spacing-* 都基于 rem，改根字号会等比缩放整套界面
 * （等价于浏览器缩放，布局相对关系不变，不会错位）。
 * 值存 localStorage，与主题一样属设备级显示偏好，不进后端。
 */
export type FontSize = 'sm' | 'md' | 'lg' | 'xl';

const FONT_SIZE_KEY = 'hz_font_size';
const FONT_SIZE_PX: Record<FontSize, string> = {
	sm: '14px',
	md: '16px',
	lg: '18px',
	xl: '20px'
};

function getInitialFontSize(): FontSize {
	if (typeof localStorage !== 'undefined') {
		const stored = localStorage.getItem(FONT_SIZE_KEY) as FontSize | null;
		if (stored && stored in FONT_SIZE_PX) return stored;
	}
	return 'md';
}

let fontSize = $state<FontSize>(getInitialFontSize());

function applyFontSize(s: FontSize) {
	if (typeof document !== 'undefined') {
		document.documentElement.style.fontSize = FONT_SIZE_PX[s];
	}
}

// 初始化时应用
if (typeof document !== 'undefined') {
	applyFontSize(fontSize);
}

export const fontSizeStore = {
	get value() {
		return fontSize;
	},
	set value(s: FontSize) {
		fontSize = s;
		if (typeof localStorage !== 'undefined') {
			localStorage.setItem(FONT_SIZE_KEY, s);
		}
		applyFontSize(s);
	}
};
