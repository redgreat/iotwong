<script lang="ts">
	// AppIcon — 内嵌描边图标（lucide 图标路径数据，见 $lib/icons.ts）。
	// 不用 lucide-svelte 包：本仓库全局 runes 模式与其 $$props 源码不兼容。
	import { ICONS, type IconName } from '$lib/icons';

	let { name, class: cls = '' }: { name: string; class?: string } = $props();

	const nodes = $derived(ICONS[name as IconName] ?? ICONS.map);
</script>

<svg
	class={cls}
	width="24"
	height="24"
	viewBox="0 0 24 24"
	fill="none"
	stroke="currentColor"
	stroke-width="2"
	stroke-linecap="round"
	stroke-linejoin="round"
	aria-hidden="true"
>
	{#each nodes as n, i (i)}
		<svelte:element this={n[0]} {...n[1]} />
	{/each}
</svg>
