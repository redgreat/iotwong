<script lang="ts">
	// 顶栏：最左=应用图标+名称（点击 Logo 展开/收起侧边菜单）；最右=主题切换 +
	// 用户下拉（账户信息/用户管理/退出）。
	import AppIcon from '$lib/components/ui/app-icon.svelte';
	import brandIcon from '$lib/assets/brand-icon.svg';
	import { theme, toggleTheme } from '$lib/theme.svelte';
	import { shell } from '$lib/ui-state.svelte';
	import type { Me } from '$lib/api';

	let {
		me,
		onLogout,
		onManageUsers,
		onOpenNav
	}: {
		me: Me;
		onLogout: () => void;
		onManageUsers: () => void;
		onOpenNav: () => void;
	} = $props();

	const isAdmin = $derived(me.tenants[0]?.role === 'admin');
	const roleLabel = $derived(me.tenants[0]?.role === 'admin' ? '管理员' : me.tenants[0]?.role === 'viewer' ? '只读用户' : me.tenants[0]?.role || '—');
	const tenantLabel = $derived(me.tenants.map((t) => t.tenant_name).join('、') || '—');

	let menuOpen = $state(false);
	let menuEl: HTMLDivElement | undefined;

	function onDocDown(e: PointerEvent) {
		if (menuEl && !menuEl.contains(e.target as Node)) menuOpen = false;
	}
	$effect(() => {
		if (menuOpen) document.addEventListener('pointerdown', onDocDown);
		else document.removeEventListener('pointerdown', onDocDown);
		return () => document.removeEventListener('pointerdown', onDocDown);
	});
</script>

<header class="border-border/80 bg-background sticky top-0 z-30 flex h-14 w-full shrink-0 items-center gap-3 border-b px-3 sm:px-4">
	<button
		type="button"
		class="hover:bg-accent text-muted-foreground inline-flex size-9 items-center justify-center rounded-md md:hidden"
		aria-label="打开菜单"
		onclick={onOpenNav}
	>
		<AppIcon name="menu" class="size-5" />
	</button>

	<!-- 应用图标 + 名称（最左上角）；点击 Logo 展开/收起侧栏 -->
	<button
		type="button"
		title={shell.sidebarPinned ? '收起侧边菜单' : '展开侧边菜单'}
		aria-label="展开或收起侧边菜单"
		onclick={() => (shell.sidebarPinned = !shell.sidebarPinned)}
		class="hover:bg-accent group flex min-w-0 items-center gap-2 rounded-md py-1 pr-1.5"
	>
		<!-- 品牌图标（brand-icon.svg：定位针 + 信号弧），点击 Logo 展开/收起侧栏 -->
		<img
			src={brandIcon}
			alt=""
			aria-hidden="true"
			draggable="false"
			class="size-8 shrink-0 rounded-lg shadow-sm"
		/>
		<span class="truncate text-base font-semibold tracking-tight">
			iotwong<span class="text-muted-foreground ml-1.5 hidden text-xs font-normal sm:inline">定位设备管理平台</span>
		</span>
		<AppIcon
			name={shell.sidebarPinned ? 'chevrons-left' : 'chevrons-right'}
			class="text-muted-foreground/60 group-hover:text-muted-foreground hidden size-3.5 transition-transform md:inline-block"
		/>
	</button>

	<div class="flex-1"></div>

	<!-- 主题切换 -->
	<button
		type="button"
		onclick={toggleTheme}
		aria-pressed={theme.dark}
		title={theme.dark ? '切换浅色' : '切换深色'}
		class="border-border text-muted-foreground hover:bg-accent hover:text-accent-foreground inline-flex size-9 items-center justify-center rounded-md border"
	>
		{#if theme.dark}
			<AppIcon name="sun" class="size-4.5" />
			<span class="sr-only">浅色模式</span>
		{:else}
			<AppIcon name="moon" class="size-4.5" />
			<span class="sr-only">深色模式</span>
		{/if}
	</button>

	<!-- 用户下拉 -->
	<div class="relative" bind:this={menuEl}>
		<button
			type="button"
			aria-haspopup="menu"
			aria-expanded={menuOpen}
			onclick={() => (menuOpen = !menuOpen)}
			class="hover:bg-accent flex h-9 items-center gap-2 rounded-md px-1.5"
		>
			<span class="bg-primary text-primary-foreground inline-flex size-7 items-center justify-center rounded-full text-xs font-semibold uppercase">
				{me.user.login.slice(0, 1)}
			</span>
			<span class="hidden text-left text-xs leading-tight sm:block">
				<span class="block max-w-36 truncate font-medium">{me.user.login}</span>
				<span class="text-muted-foreground block text-[10px]">{roleLabel} · {tenantLabel}</span>
			</span>
			<AppIcon name="chevron-down" class="text-muted-foreground size-3.5" />
		</button>

		{#if menuOpen}
			<div
				role="menu"
				class="border-border bg-popover text-popover-foreground absolute right-0 top-full z-40 mt-1.5 w-60 overflow-hidden rounded-lg border shadow-lg"
			>
				<div class="bg-muted/60 flex items-center gap-2.5 px-3 py-2.5">
					<span class="bg-primary text-primary-foreground inline-flex size-8 shrink-0 items-center justify-center rounded-full text-sm font-semibold uppercase">
						{me.user.login.slice(0, 1)}
					</span>
					<span class="min-w-0 text-left">
						<span class="block truncate text-sm font-medium">{me.user.login}</span>
						<span class="text-muted-foreground block truncate text-xs">{tenantLabel} · {roleLabel}</span>
					</span>
				</div>
				{#if isAdmin}
					<button
						type="button"
						role="menuitem"
						class="hover:bg-accent hover:text-accent-foreground flex w-full items-center gap-2.5 px-3 py-2 text-left text-sm"
						onclick={() => { menuOpen = false; onManageUsers(); }}
					>
						<AppIcon name="users-round" class="size-4" />
						用户管理
					</button>
					<div class="bg-muted/40 my-1 h-px"></div>
				{/if}
				<button
					type="button"
					role="menuitem"
					class="hover:bg-accent hover:text-accent-foreground flex w-full items-center gap-2.5 px-3 py-2 text-left text-sm"
					onclick={() => { menuOpen = false; onLogout(); }}
				>
					<AppIcon name="log-out" class="size-4" />
					退出登录
				</button>
			</div>
		{/if}
	</div>
</header>
