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
			.addEventListener('change', () => applyTheme(theme));
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
