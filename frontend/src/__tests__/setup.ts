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
