<script lang="ts">
	// DataPage — 列表类页面的统一外壳（标题/说明 + 右侧操作区 + 内容区），
	// 供设备管理、围栏等列表页复用统一样式。
	import type { Snippet } from 'svelte';

	interface Props {
		title: string;
		description?: string;
		actions?: Snippet;
		children?: Snippet;
		width?: 'narrow' | 'full';
	}

	let { title, description = '', actions, children, width = 'full' }: Props = $props();
</script>

<section class="space-y-3">
	<div class="border-border bg-card flex flex-wrap items-center justify-between gap-3 rounded-xl border p-4 shadow-sm">
		<div class="min-w-0">
			<h2 class="text-base font-semibold leading-6">{title}</h2>
			{#if description}
				<p class="text-muted-foreground mt-1 text-xs">{description}</p>
			{/if}
		</div>
		{#if actions}
			<div class="flex shrink-0 items-center gap-2">{@render actions()}</div>
		{/if}
	</div>

	<div class={width === 'narrow' ? 'mx-auto w-full max-w-3xl' : ''}>
		{@render children?.()}
	</div>
</section>
