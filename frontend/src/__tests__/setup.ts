import '@testing-library/jest-dom/vitest';

// Mock window.matchMedia (jsdom 不原生支持)
Object.defineProperty(window, 'matchMedia', {
	writable: true,
	value: vi.fn().mockImplementation((query: string) => ({
		matches: false,
		media: query,
		onchange: null,
		addListener: vi.fn(),
		removeListener: vi.fn(),
		addEventListener: vi.fn(),
		removeEventListener: vi.fn(),
		dispatchEvent: vi.fn()
	}))
});

// Mock navigator.onLine
Object.defineProperty(navigator, 'onLine', {
	writable: true,
	value: true
});

// Mock Element.scrollIntoView（jsdom 未实现，选择器用它把当前一级滚入可视区）
if (typeof Element !== 'undefined' && !Element.prototype.scrollIntoView) {
	Element.prototype.scrollIntoView = function scrollIntoView(): void {};
}

// Mock Element.animate（jsdom 未实现 Web Animations API，
// 而 Svelte 5 的 transition:slide 依赖它，缺失会抛 "element.animate is not a function"）
if (typeof Element !== 'undefined' && !Element.prototype.animate) {
	Element.prototype.animate = function animate(): any {
		const animation: any = {
			currentTime: 0,
			startTime: 0,
			playbackRate: 1,
			playState: 'finished',
			pending: false,
			effect: {
				getTiming: () => ({ duration: 0, delay: 0 }),
				getComputedTiming: () => ({ duration: 0, delay: 0 })
			},
			finished: Promise.resolve(),
			onfinish: null,
			oncancel: null,
			cancel: () => {},
			finish: () => {},
			play: () => {},
			pause: () => {},
			reverse: () => {},
			commitStyles: () => {},
			addEventListener: () => {},
			removeEventListener: () => {}
		};
		// 立即结束，让 Svelte 的 transition 回调在下一个微任务里执行
		queueMicrotask(() => animation.onfinish?.());
		return animation;
	} as any;
}
