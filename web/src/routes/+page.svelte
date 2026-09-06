<script lang="ts">
	import { onMount } from 'svelte';
	import Button from '$lib/components/ui/button.svelte';
	import Input from '$lib/components/ui/input.svelte';
	import AppHeader from '$lib/components/app-header.svelte';
	import AppSidebar, { type SidebarItem } from '$lib/components/app-sidebar.svelte';
	import DevicesView from '$lib/components/views/devices-view.svelte';
	import AlarmsView from '$lib/components/views/alarms-view.svelte';
	import FencesView from '$lib/components/views/fences-view.svelte';
	import RenameView from '$lib/components/views/rename-view.svelte';
	import DeviceManageView from '$lib/components/views/device-manage-view.svelte';
	import UsersView from '$lib/components/views/users-view.svelte';
	import { ApiError } from '$lib/api';
	import { auth, boot, login, logout } from '$lib/auth.svelte';
	import brandIcon from '$lib/assets/brand-icon.svg';

	onMount(() => { void boot(); });

	// ---------- 登录 ----------
	let loginName = $state('');
	let loginPassword = $state('');
	let loginError = $state<string | null>(null);
	let loginBusy = $state(false);

	async function doLogin() {
		loginBusy = true;
		loginError = null;
		try {
			await login(loginName, loginPassword);
		} catch (e) {
			loginError = e instanceof ApiError ? e.message : String(e);
		} finally {
			loginBusy = false;
		}
	}

	// ---------- 角色驱动菜单（每个菜单项即一个页面视图） ----------
	type ViewId = 'devices' | 'manage' | 'alarms' | 'fences' | 'rename' | 'users';
	let currentView = $state<ViewId>('devices');
	let mobileNavOpen = $state(false);

	const menuItems = $derived.by<SidebarItem[]>(() => {
		const role = auth.me?.tenants[0]?.role;
		const items: SidebarItem[] = [];
		if (role === 'admin') {
			items.push(
				{ id: 'devices', label: '设备地图' },
				{ id: 'manage', label: '设备管理' },
				{ id: 'alarms', label: '报警确认' },
				{ id: 'fences', label: '围栏管理' },
				{ id: 'rename', label: '设备改名' },
				{ id: 'users', label: '用户管理' }
			);
		} else if (role === 'viewer') {
			items.push(
				{ id: 'devices', label: '设备地图' },
				{ id: 'manage', label: '设备管理' },
				{ id: 'alarms', label: '报警（只读）' }
			);
		} else {
			items.push(
				{ id: 'devices', label: '设备地图' },
				{ id: 'manage', label: '设备管理' }
			);
		}
		return items;
	});
	const allowedViews = $derived(new Set(menuItems.map((m) => m.id)));
	const currentLabel = $derived(menuItems.find((m) => m.id === currentView)?.label ?? '工作台');
	$effect(() => {
		if (auth.status === 'signed' && !allowedViews.has(currentView)) currentView = 'devices';
	});

	async function doLogout() {
		currentView = 'devices';
		mobileNavOpen = false;
		await logout();
	}
	function manageUsers() {
		currentView = 'users';
		mobileNavOpen = false;
	}
</script>

<svelte:head>
	<title>{auth.status === 'signed' ? `${currentLabel} — iotwong` : '登录 — iotwong'}</title>
</svelte:head>

{#if auth.status === 'checking'}
	<div class="flex min-h-dvh items-center justify-center">
		<div class="text-center text-sm text-muted-foreground">
			<div class="border-primary mx-auto mb-3 size-6 animate-spin rounded-full border-2 border-t-transparent"></div>
			正在检查会话…
		</div>
	</div>
{:else if auth.status === 'anon'}
	<div class="flex min-h-dvh flex-col">
		<div class="flex h-14 w-full items-center gap-2 px-4">
			<img src={brandIcon} alt="" aria-hidden="true" draggable="false" class="size-8 rounded-lg" />
			<span class="text-base font-semibold tracking-tight">iotwong</span>
		</div>
		<div class="flex flex-1 items-start justify-center px-4 pt-10 sm:pt-16">
			<div class="border-border bg-card w-full max-w-sm rounded-2xl border p-6 shadow-lg">
				<h1 class="text-lg font-semibold">登录 iotwong</h1>
				<p class="text-muted-foreground mt-1 mb-4 text-xs">本地账号会话（HttpOnly Cookie）</p>
				<form onsubmit={(e) => { e.preventDefault(); void doLogin(); }} class="flex flex-col gap-3">
					<Input bind:value={loginName} placeholder="登录名" autocomplete="username" required />
					<Input bind:value={loginPassword} type="password" placeholder="密码" autocomplete="current-password" required />
					{#if loginError}<p role="alert" class="text-destructive text-xs">{loginError}</p>{/if}
					<Button type="submit" disabled={loginBusy} class="w-full">{loginBusy ? '登录中…' : '登录'}</Button>
				</form>
			</div>
		</div>
	</div>
{:else}
	<div class="flex h-dvh flex-col overflow-hidden">
		<AppHeader
			me={auth.me!}
			onLogout={() => void doLogout()}
			onManageUsers={manageUsers}
			onOpenNav={() => (mobileNavOpen = true)}
		/>
		<div class="flex min-h-0 flex-1">
			<AppSidebar
				items={menuItems}
				active={currentView}
				onSelect={(id: string) => (currentView = id as ViewId)}
				mobileOpen={mobileNavOpen}
				onCloseMobile={() => (mobileNavOpen = false)}
			/>
			{#if currentView === 'devices'}
				<main class="min-w-0 flex-1">
					<DevicesView />
				</main>
			{:else}
				<main class="bg-muted/40 min-w-0 flex-1 overflow-y-auto">
					<div class="mx-auto w-full max-w-[1600px] space-y-4 p-3 sm:p-5">
						{#if currentView === 'manage'}
							<DeviceManageView />
						{:else if currentView === 'alarms'}
							<AlarmsView role={auth.me!.tenants[0]!.role} />
						{:else if currentView === 'fences'}
							<FencesView />
						{:else if currentView === 'rename'}
							<RenameView />
						{:else if currentView === 'users'}
							<UsersView />
						{/if}
					</div>
				</main>
			{/if}
		</div>
	</div>
{/if}
