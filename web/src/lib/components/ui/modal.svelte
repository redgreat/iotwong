<script lang="ts">
	// Modal — 公共弹窗外壳（围栏新增/用户管理等复用）。
	// 固定样式：居中卡片 + 遮罩 + 标题栏（可关闭）+ 内容插槽 + 底部操作插槽；
	// 支持 Esc / 点击遮罩关闭；内容区出现时焦点限制在对话框内。
	import { onMount, onDestroy } from 'svelte';
	import type { Snippet } from 'svelte';
	import AppIcon from '$lib/components/ui/app-icon.svelte';

	interface Props {
		title: string;
		description?: string;
		size?: 'sm' | 'md' | 'lg';
		onClose: () => void;
		children?: Snippet;
		footer?: Snippet;
	}

	let { title, description = '', size = 'md', onClose, children, footer }: Props = $props();

	const panelW = $derived({ sm: 'max-w-sm', md: 'max-w-lg', lg: 'max-w-2xl' }[size]);

	let panelEl: HTMLDivElement | undefined;
	let lastFocus: HTMLElement | null = null;

	function onKey(e: KeyboardEvent) {
		if (e.key === 'Escape') onClose();
	}

	onMount(() => {
		lastFocus = document.activeElement as HTMLElement | null;
		document.addEventListener('keydown', onKey);
		// 焦点进入弹窗
		panelEl?.querySelector<HTMLElement>('input,select,button,textarea')?.focus();
	});
	onDestroy(() => {
		document.removeEventListener('keydown', onKey);
		lastFocus?.focus?.();
	});
</script>

<div class="fixed inset-0 z-50 flex items-center justify-center p-4">
	<button
		type="button"
		class="bg-black/45 backdrop-blur-[2px] absolute inset-0 animate-in fade-in-0"
		aria-label="关闭弹窗"
		onclick={onClose}
	></button>

	<div
		bind:this={panelEl}
		role="dialog"
		aria-modal="true"
		aria-label={title}
		class="border-border bg-card text-card-foreground relative z-10 flex max-h-[88dvh] w-full flex-col overflow-hidden rounded-2xl border shadow-2xl animate-in zoom-in-95 fade-in-0 {panelW}"
	>
		<!-- 标题栏 -->
		<div class="border-border/70 flex items-start justify-between gap-3 border-b px-5 py-3.5">
			<div class="min-w-0">
				<h3 class="text-base font-semibold leading-6">{title}</h3>
				{#if description}
					<p class="text-muted-foreground mt-0.5 text-xs">{description}</p>
				{/if}
			</div>
			<button
				type="button"
				class="text-muted-foreground hover:bg-accent hover:text-accent-foreground -mr-1 -mt-1 inline-flex size-8 shrink-0 items-center justify-center rounded-lg"
				aria-label="关闭弹窗"
				onclick={onClose}
			>
				<AppIcon name="x" class="size-4.5" />
			</button>
		</div>

		<!-- 内容 -->
		{#if children}
			<div class="min-h-0 flex-1 overflow-y-auto px-5 py-4">
				{@render children()}
			</div>
		{/if}

		<!-- 底部操作 -->
		{#if footer}
			<div class="border-border/70 flex items-center justify-end gap-2 border-t px-5 py-3">
				{@render footer()}
			</div>
		{/if}
	</div>
</div>
