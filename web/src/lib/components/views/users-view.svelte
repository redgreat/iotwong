<script lang="ts">
	import { onMount } from 'svelte';
	import Button from '$lib/components/ui/button.svelte';
	import Modal from '$lib/components/ui/modal.svelte';
	import Input from '$lib/components/ui/input.svelte';
	import { apiUsers, apiCreateUser, apiResetUserPassword, ApiError, type UserItem } from '$lib/api';

	// 用户管理（仅 admin 菜单可见）：GET/POST /users 与
	// POST /users/{id}/reset-password。登录后下拉“用户管理”进入本视图。
	let items = $state<UserItem[] | null>(null);
	let loading = $state(true);
	let viewError = $state<string | null>(null);
	let notice = $state<string | null>(null);
	let noticeTimer: ReturnType<typeof setTimeout> | undefined;

	// 新建用户弹窗
	let createOpen = $state(false);
	let cLogin = $state('');
	let cPassword = $state('');
	let cRole = $state('viewer');
	let cError = $state<string | null>(null);
	let creating = $state(false);

	// 重置密码弹窗
	let resetTarget = $state<UserItem | null>(null);
	let rPassword = $state('');
	let rError = $state<string | null>(null);
	let resetting = $state(false);

	const roleLabel: Record<string, string> = { admin: '管理员', viewer: '只读用户' };

	function flash(text: string) {
		notice = text;
		if (noticeTimer) clearTimeout(noticeTimer);
		noticeTimer = setTimeout(() => (notice = null), 4500);
	}

	async function load() {
		loading = true;
		viewError = null;
		try {
			const page = await apiUsers();
			items = page.items ?? [];
		} catch (e) {
			items = null;
			viewError = e instanceof ApiError ? e.message : String(e);
		} finally {
			loading = false;
		}
	}
	onMount(() => void load());

	function openCreate() {
		cLogin = '';
		cPassword = '';
		cRole = 'viewer';
		cError = null;
		createOpen = true;
	}
	function closeCreate() {
		createOpen = false;
	}
	async function submitCreate() {
		const login = cLogin.trim();
		if (!login || cPassword.length < 6) {
			cError = '登录名与至少 6 位密码必填';
			return;
		}
		creating = true;
		cError = null;
		try {
			await apiCreateUser({ login, password: cPassword, role: cRole });
			createOpen = false;
			flash(`已创建用户 ${login}`);
			await load();
		} catch (e) {
			cError = e instanceof ApiError ? e.message : String(e);
		} finally {
			creating = false;
		}
	}

	function openReset(u: UserItem) {
		resetTarget = u;
		rPassword = '';
		rError = null;
	}
	function closeReset() {
		resetTarget = null;
	}
	async function submitReset() {
		if (!resetTarget) return;
		if (rPassword.length < 6) {
			rError = '新密码至少 6 位';
			return;
		}
		resetting = true;
		rError = null;
		try {
			await apiResetUserPassword(resetTarget.id, rPassword);
			flash(`已重置 ${resetTarget.login} 的密码`);
			closeReset();
		} catch (e) {
			rError = e instanceof ApiError ? e.message : String(e);
		} finally {
			resetting = false;
		}
	}
</script>

<section aria-label="用户管理" class="space-y-3">
	<div class="border-border bg-card flex flex-wrap items-center justify-between gap-3 rounded-xl border p-4 shadow-sm">
		<div>
			<h2 class="text-base font-semibold">用户管理</h2>
			<p class="text-muted-foreground mt-0.5 text-xs">
				当前租户下的本地账号（登录名全局唯一）；新建账号或重置密码后请告知对方新密码。
			</p>
		</div>
		<Button size="sm" onclick={openCreate}>新建用户</Button>
	</div>

	{#if notice}
		<p role="status" class="border-success/40 bg-success/10 text-success-foreground rounded-md border px-3 py-2 text-xs">{notice}</p>
	{/if}

	{#if loading}
		<div class="border-border bg-card text-muted-foreground rounded-xl border p-10 text-center text-sm">加载用户中…</div>
	{:else if viewError}
		<div class="border-border bg-card rounded-xl border p-8 text-center text-sm">
			<p role="alert" class="text-destructive">{viewError}</p>
			<div class="mt-3"><Button size="sm" variant="outline" onclick={() => void load()}>重试</Button></div>
		</div>
	{:else if !items || items.length === 0}
		<div class="border-border bg-card text-muted-foreground rounded-xl border p-10 text-center text-sm">还没有用户。</div>
	{:else}
		<div class="border-border bg-card overflow-x-auto rounded-xl border shadow-sm">
			<table class="w-full min-w-[560px] text-left text-sm">
				<thead>
					<tr class="text-muted-foreground border-b text-xs">
						<th class="px-4 py-2.5 font-medium">登录名</th>
						<th class="px-4 py-2.5 font-medium">角色</th>
						<th class="px-4 py-2.5 font-medium">租户</th>
						<th class="px-4 py-2.5 font-medium">创建时间</th>
						<th class="px-4 py-2.5 text-right font-medium">操作</th>
					</tr>
				</thead>
				<tbody>
					{#each items as u (u.id)}
						<tr class="hover:bg-accent/40 border-b last:border-b-0">
							<td class="px-4 py-2.5 font-medium">{u.login}</td>
							<td class="px-4 py-2.5">
								<span
									class="inline-flex rounded px-1.5 py-0.5 text-xs"
									class:bg-primary={u.role === 'admin'}
									class:text-primary-foreground={u.role === 'admin'}
									class:bg-muted={u.role !== 'admin'}
									class:text-muted-foreground={u.role !== 'admin'}
								>
									{roleLabel[u.role] ?? u.role}
								</span>
							</td>
							<td class="text-muted-foreground px-4 py-2.5">{u.tenant_name}</td>
							<td class="text-muted-foreground px-4 py-2.5 tabular-nums">{new Date(u.created_at).toLocaleString('zh-CN', { hour12: false })}</td>
							<td class="px-4 py-2.5 text-right">
								<Button size="sm" variant="outline" onclick={() => openReset(u)}>重置密码</Button>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</section>


<!-- 新建用户 -->
{#if createOpen}
	<Modal title="新建用户" description="在当前租户下创建本地账号（登录名全局唯一，初始密码仅本次显示）。" size="sm" onClose={closeCreate}>
		<form id="user-create-form" class="flex flex-col gap-3" onsubmit={(e) => { e.preventDefault(); void submitCreate(); }}>
			<label class="flex flex-col gap-1 text-xs text-muted-foreground">
				登录名
				<Input bind:value={cLogin} placeholder="如 ops-viewer" autocomplete="off" required />
			</label>
			<label class="flex flex-col gap-1 text-xs text-muted-foreground">
				初始密码（≥6 位）
				<Input bind:value={cPassword} type="password" placeholder="••••••••" autocomplete="new-password" required />
			</label>
			<label class="flex flex-col gap-1 text-xs text-muted-foreground">
				角色
				<select bind:value={cRole} class="border-input bg-background h-9 rounded-md border px-2 text-sm">
					<option value="viewer">viewer（只读）</option>
					<option value="admin">admin（管理员）</option>
				</select>
			</label>
			{#if cError}<p role="alert" class="text-destructive text-xs">{cError}</p>{/if}
		</form>
		{#snippet footer()}
			<Button type="button" variant="outline" onclick={closeCreate} disabled={creating}>取消</Button>
			<Button type="submit" form="user-create-form" disabled={creating}>{creating ? '创建中…' : '创建'}</Button>
		{/snippet}
	</Modal>
{/if}

<!-- 重置密码 -->
{#if resetTarget}
	<Modal title="重置密码 — {resetTarget.login}" description="重置后原密码立即失效；新密码仅本次输入，不落库明文。" size="sm" onClose={closeReset}>
		<form id="user-reset-form" class="flex flex-col gap-3" onsubmit={(e) => { e.preventDefault(); void submitReset(); }}>
			<label class="flex flex-col gap-1 text-xs text-muted-foreground">
				新密码（≥6 位）
				<Input bind:value={rPassword} type="password" placeholder="••••••••" autocomplete="new-password" required />
			</label>
			{#if rError}<p role="alert" class="text-destructive text-xs">{rError}</p>{/if}
		</form>
		{#snippet footer()}
			<Button type="button" variant="outline" onclick={closeReset} disabled={resetting}>取消</Button>
			<Button type="submit" form="user-reset-form" disabled={resetting}>{resetting ? '提交中…' : '确认重置'}</Button>
		{/snippet}
	</Modal>
{/if}
