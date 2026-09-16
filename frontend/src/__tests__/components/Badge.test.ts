import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import Badge from '$lib/components/ui/Badge.svelte';

describe('Badge', () => {
	it('渲染默认样式', () => {
		const { container } = render(Badge, { props: {} });
		expect(container.querySelector('div')).toBeTruthy();
	});

	it('渲染secondary变体', () => {
		const { container } = render(Badge, { props: { variant: 'secondary' } });
		expect(container.querySelector('div')).toBeTruthy();
	});

	it('渲染destructive变体', () => {
		const { container } = render(Badge, { props: { variant: 'destructive' } });
		expect(container.querySelector('div')).toBeTruthy();
	});

	it('渲染outline变体', () => {
		const { container } = render(Badge, { props: { variant: 'outline' } });
		expect(container.querySelector('div')).toBeTruthy();
	});
});
