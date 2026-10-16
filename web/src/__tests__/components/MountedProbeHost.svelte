<script lang="ts">
	/**
	 * 仅用于测试的宿主：把 useMounted() 的「脚本执行期」与「挂载后」两个取值都渲染出来。
	 *
	 * 图表组件的守卫从 `{#if mounted && …}` 改成 `{#if mounted() && …}`。
	 * `useMounted()` 返回的是一个函数，旧写法判断的是函数引用本身 —— 恒为真，
	 * 于是 SSR / 预渲染阶段也会去渲染 Chart.js，撞上「canvas 不可用」。
	 * 这里锁住修复后的语义：初始化期（等价于 SSR 看到的值）为 false，挂载后才为 true。
	 */
	import { useMounted } from '#lib/components/charts/chartSetup.svelte';

	const mounted = useMounted();
	// 脚本体在 onMount 之前执行，取到的就是 SSR / 预渲染阶段会看到的值
	const duringInit = mounted();
</script>

<span data-testid="during-init">{String(duringInit)}</span>
<span data-testid="after-mount">{String(mounted())}</span>
<span data-testid="is-function">{String(typeof mounted === 'function')}</span>
<span data-testid="fn-truthy">{String(Boolean(mounted))}</span>
