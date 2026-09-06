<script lang="ts">
	// 左侧功能菜单：
	//  - 桌面：默认折叠为图标窄栏；鼠标悬停临时展开；点击任一菜单后固定为
	//    展开态、不再自动收缩。固定态由顶栏最左 Logo 切换（shell.sidebarPinned）。
	//  - 移动端：抽屉（由顶栏汉堡键开关），点击菜单后自动关闭。
	//  - 左下角：copyright 图标 + wangcw（链接 GitHub 开源仓库）；折叠时保持简短、不换行。
	import AppIcon from '$lib/components/ui/app-icon.svelte';
	import { shell } from '$lib/ui-state.svelte';

	export interface SidebarItem {
		id: string;
		label: string;
	}

	let {
		items,
		active,
		onSelect,
		mobileOpen,
		onCloseMobile
	}: {
		items: SidebarItem[];
		active: string;
		onSelect: (id: string) => void;
		mobileOpen: boolean;
		onCloseMobile: () => void;
	} = $props();

	let hovered = $state(false);
	const expanded = $derived(shell.sidebarPinned || hovered);

	function choose(id: string) {
		shell.sidebarPinned = true; // 点击后不再自动收缩
		onSelect(id);
	}
	function fireResizeSoon() {
		// 通知地图容器尺寸变化（maplibre 监听 window resize）
		window.setTimeout(() => window.dispatchEvent(new Event('resize')), 240);
	}

	const GITHUB_URL = 'https://github.com/redgreat/iotwong';
	const START_YEAR = 2026;
	const NOW_YEAR = new Date().getFullYear();
	const copyShort = $derived('wangcw');
	const copyFull = $derived(
		NOW_YEAR > START_YEAR ? `wangcw ${START_YEAR}-${NOW_YEAR}` : `wangcw ${START_YEAR}`
	);
</script>

{#snippet menuIcon(id: string)}
	<AppIcon
		name={id === 'devices' ? 'map' : id === 'manage' ? 'list' : id === 'alarms' ? 'bell-ring' : id === 'fences' ? 'fence' : id === 'rename' ? 'pencil-line' : id === 'users' ? 'users-round' : 'map'}
		class="size-5 shrink-0"
	/>
{/snippet}

<!-- 桌面端侧栏 -->
<aside
	class="border-border/80 bg-background hidden shrink-0 flex-col border-r transition-[width] duration-200 ease-out md:flex md:w-16"
	class:md:w-64={expanded}
	onmouseenter={() => { hovered = true; fireResizeSoon(); }}
	onmouseleave={() => { hovered = false; fireResizeSoon(); }}
	aria-label="功能侧栏"
>
	<nav aria-label="功能菜单" class="flex min-h-0 flex-1 flex-col gap-1 overflow-x-hidden overflow-y-auto p-2">
		{#each items as item (item.id)}
			<button
				type="button"
				title={item.label}
				aria-current={active === item.id ? 'page' : undefined}
				onclick={() => choose(item.id)}
				class="flex h-11 w-full shrink-0 items-center gap-3 rounded-lg px-2.5 text-sm font-medium transition-colors"
				class:bg-primary={active === item.id}
				class:text-primary-foreground={active === item.id}
				class:hover:bg-accent={active !== item.id}
				class:hover:text-accent-foreground={active !== item.id}
				class:text-muted-foreground={active !== item.id}
			>
				{@render menuIcon(item.id)}
				<span
					class="whitespace-nowrap transition-all duration-200"
					class:w-auto={expanded}
					class:w-0={!expanded}
					class:opacity-100={expanded}
					class:opacity-0={!expanded}
					class:overflow-hidden={!expanded}
				>
					{item.label}
				</span>
			</button>
		{/each}
	</nav>
	<!-- 左下角：copyright 图标 + wangcw（简短、不换行），链接 GitHub -->
	<div class="shrink-0 border-t p-1.5">
		<a
			href={GITHUB_URL}
			target="_blank"
			rel="noreferrer noopener"
			title={copyFull}
			class="text-muted-foreground hover:text-accent-foreground flex h-8 w-full items-center justify-center gap-1 overflow-hidden rounded-md text-[11px] leading-none transition-colors"
		>
			<AppIcon name="copyright" class="size-3 shrink-0" />
			<span class="truncate whitespace-nowrap">{expanded ? copyFull : copyShort}</span>
		</a>
	</div>
</aside>

<!-- 移动端抽屉 -->
{#if mobileOpen}
	<button
		type="button"
		class="bg-black/40 fixed inset-0 z-40 md:hidden"
		aria-label="关闭菜单"
		onclick={onCloseMobile}
	></button>
	<aside class="border-border bg-background fixed inset-y-0 left-0 z-50 flex w-64 flex-col border-r md:hidden">
		<div class="border-border/70 flex h-14 shrink-0 items-center justify-between border-b px-3">
			<span class="text-sm font-semibold">功能菜单</span>
			<button type="button" class="hover:bg-accent text-muted-foreground inline-flex size-8 items-center justify-center rounded-md" aria-label="关闭菜单" onclick={onCloseMobile}>
				<AppIcon name="x" class="size-4.5" />
			</button>
		</div>
		<nav aria-label="功能菜单" class="flex flex-1 flex-col gap-1 overflow-y-auto p-2">
			{#each items as item (item.id)}
				<button
					type="button"
					aria-current={active === item.id ? 'page' : undefined}
					onclick={() => { onSelect(item.id); onCloseMobile(); }}
					class="flex h-11 shrink-0 items-center gap-3 rounded-lg px-3 text-sm font-medium transition-colors"
					class:bg-primary={active === item.id}
					class:text-primary-foreground={active === item.id}
					class:hover:bg-accent={active !== item.id}
					class:text-muted-foreground={active !== item.id}
				>
					{@render menuIcon(item.id)}
					{item.label}
				</button>
			{/each}
		</nav>
		<div class="border-border/70 flex h-9 shrink-0 items-center justify-center gap-1 border-t px-2">
			<a href={GITHUB_URL} target="_blank" rel="noreferrer noopener" class="text-muted-foreground hover:text-accent-foreground flex min-w-0 items-center gap-1 truncate text-[11px]">
				<AppIcon name="copyright" class="size-3 shrink-0" />
				<span class="truncate">{copyFull}</span>
			</a>
		</div>
	</aside>
{/if}
