<script lang="ts">
import { setContext } from 'svelte';
import { cn } from '$lib/utils/cn';

let {
	class: className = '',
	value = $bindable(''),
	children
}: {
	class?: string;
	value?: string;
	children?: any;
} = $props();

/**
 * 受控组件：选中项完全由外部 value 决定。
 *
 * 此前这里用 `let current = $state(value || '')` 在挂载时做了一次性快照，
 * 再靠 `$effect(() => (value = current))` 单向回写外部。由于该 effect 只依赖
 * current，外部后续对 value 的修改永远不会同步进组件内部——于是「编辑交易」
 * 这种异步回填场景（挂载时 type 还是默认值 expense，接口返回后才变成
 * income/transfer）里，Tab 高亮会永久卡在初始值上，而分类等按真实 type 派生的
 * 逻辑却已经变了，页面自相矛盾。
 *
 * 这里直接读写 bindable prop：外部改 → UI 立刻跟上；点击 → 回写外部。
 * 不再需要任何 effect，也就不存在内外状态打架的问题。
 * （已确认全项目 5 处 <Tabs> 均使用 bind:value，无「非受控」用法。）
 */
setContext('tabs-current', {
	get value() {
		return value ?? '';
	},
	set value(v: string) {
		value = v;
	}
});

const classes = $derived(
	cn(
		'inline-flex h-9 items-center justify-center rounded-lg bg-muted p-1 text-muted-foreground',
		className
	)
);
</script>

<div role="tablist" class={classes}>
	{@render children?.()}
</div>
