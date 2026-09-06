<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import Button from '$lib/components/ui/button.svelte';
	import { cn } from '$lib/utils';
	import { apiAlarms, apiAckAlarm, apiDevices, ApiError, type AlarmItem } from '$lib/api';

	// 报警视图：viewer 只读（不渲染确认按钮），admin 行内“确认”= POST /alarms/{id}/ack。
	let { role }: { role: string } = $props();
	const isAdmin = $derived(role === 'admin');

	let items = $state<AlarmItem[] | null>(null);
	let loading = $state(true);
	let viewError = $state<string | null>(null);
	let notice = $state<string | null>(null);
	let ackingId = $state<string | null>(null);
	// alarm 只带 device_id，设备名从设备列表交叉引用（不做假数据）。
	let deviceNames = $state<Record<string, string>>({});
	let namesDone = $state(false);
	let noticeTimer: ReturnType<typeof setTimeout> | undefined;

	const transitionLabel: Record<string, string> = { enter: '进入', exit: '离开' };

	function showNotice(text: string) {
		notice = text;
		if (noticeTimer) clearTimeout(noticeTimer);
		noticeTimer = setTimeout(() => (notice = null), 4500);
	}
	onDestroy(() => {
		if (noticeTimer) clearTimeout(noticeTimer);
	});

	async function loadDeviceNames() {
		if (namesDone) return;
		try {
			const page = await apiDevices({ limit: '200' });
			const map: Record<string, string> = {};
			for (const d of page.items) map[d.id] = d.name || d.external_id;
			deviceNames = map;
		} catch {
			// 名字交叉引用失败时回退显示设备短 id（仍为真实数据）。
		}
		namesDone = true;
	}

	async function load() {
		loading = true;
		viewError = null;
		try {
			const page = await apiAlarms(200);
			items = page.items ?? [];
		} catch (e) {
			items = null;
			viewError = e instanceof ApiError ? e.message : String(e);
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		void load();
		void loadDeviceNames();
	});

	async function ack(id: string) {
		ackingId = id;
		viewError = null;
		try {
			await apiAckAlarm(id);
			showNotice('已确认该条报警');
			await load();
		} catch (e) {
			viewError = e instanceof ApiError ? e.message : String(e);
		} finally {
			ackingId = null;
		}
	}

	function deviceLabel(a: AlarmItem): { text: string; title?: string } {
		const name = deviceNames[a.device_id];
		if (name) return { text: name };
		return { text: `设备 ${a.device_id.slice(0, 8)}…`, title: '该设备不在当前设备列表中（可能已移除）' };
	}
</script>

<section aria-label="报警列表" class="border-border bg-card space-y-3 rounded-lg border p-3">
	<div class="flex flex-wrap items-start justify-between gap-2">
		<div>
			<h2 class="text-base font-semibold">报警{isAdmin ? '确认' : '（只读）'}</h2>
			<p class="text-muted-foreground mt-0.5 text-xs">
				{#if isAdmin}
					点击行内“确认”标记已处理（POST /alarms/{'{id}'}/ack）。
				{:else}
					查看者只读：确认报警需要管理员权限，因此这里不提供确认按钮。
				{/if}
			</p>
		</div>
		<Button size="sm" variant="outline" onclick={() => void load()} disabled={loading}>
			{loading ? '刷新中…' : '刷新'}
		</Button>
	</div>

	{#if notice}
		<p role="status" class="border-success/40 text-success-foreground bg-success/10 rounded-md border px-3 py-2 text-xs">{notice}</p>
	{/if}
	{#if viewError}
		<div role="alert" class="bg-destructive/10 text-destructive rounded-md border border-destructive/30 px-3 py-2 text-xs">
			{viewError}
		</div>
	{/if}

	{#if loading}
		<p class="text-muted-foreground py-8 text-center text-sm">加载报警中…</p>
	{:else if viewError}
		<div class="text-muted-foreground py-6 text-center text-sm">
			<p>报警加载失败，请稍后重试。</p>
			<div class="mt-3"><Button size="sm" variant="outline" onclick={() => void load()}>重试</Button></div>
		</div>
	{:else if !items || items.length === 0}
		<p class="text-muted-foreground py-8 text-center text-sm">
			还没有报警记录：设备进出围栏时会在这里生成报警。
		</p>
	{:else}
		<div class="border-border overflow-hidden rounded-md border">
			<!-- 列头（桌面） -->
			<div class="border-b bg-muted/40 hidden grid-cols-[150px_minmax(0,1fr)_minmax(0,1fr)_72px_96px_110px] gap-2 px-3 py-2 text-xs font-medium sm:grid" aria-hidden="true">
				<span>时间</span>
				<span>围栏</span>
				<span>设备</span>
				<span>方向</span>
				<span>确认状态</span>
				<span>{isAdmin ? '操作' : ''}</span>
			</div>
			<ul class="divide-y">
				{#each items as a (a.id)}
					{@const dev = deviceLabel(a)}
					<li class="grid grid-cols-1 gap-x-2 gap-y-1 px-3 py-2.5 text-sm sm:grid-cols-[150px_minmax(0,1fr)_minmax(0,1fr)_72px_96px_110px] sm:items-center">
						<span class="text-muted-foreground tabular-nums sm:text-xs">
							<span class="text-muted-foreground sm:hidden">时间 </span>
							{new Date(a.occurred_at).toLocaleString('zh-CN', { hour12: false })}
						</span>
						<span class="truncate font-medium" title={a.fence_name}>
							<span class="text-muted-foreground sm:hidden">围栏 </span>{a.fence_name}
						</span>
						<span class="truncate" title={dev.title}>
							<span class="text-muted-foreground sm:hidden">设备 </span>{dev.text}
						</span>
						<span>
							<span class="text-muted-foreground sm:hidden">方向 </span>
							<span
								class={cn(
									'inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs',
									a.transition === 'enter' && 'bg-primary/10 text-primary',
									a.transition !== 'enter' && 'bg-warning/15 text-warning-foreground'
								)}
							>
								{transitionLabel[a.transition] ?? a.transition}
							</span>
						</span>
						<span>
							<span class="text-muted-foreground sm:hidden">确认状态 </span>
							<span
								class={cn(
									'inline-flex rounded-full px-2 py-0.5 text-xs',
									a.acked && 'bg-muted text-muted-foreground',
									!a.acked && 'bg-warning/15 text-warning-foreground'
								)}
							>
								{a.acked ? '已确认' : '未确认'}
							</span>
						</span>
						<span class="min-h-8 sm:flex sm:items-center">
							{#if isAdmin}
								{#if a.acked}
									<span class="text-muted-foreground text-xs" title="该报警已确认">—</span>
								{:else}
									<Button
										size="sm"
										variant="outline"
										disabled={ackingId === a.id}
										onclick={() => void ack(a.id)}
									>
										{ackingId === a.id ? '确认中…' : '确认'}
									</Button>
								{/if}
							{/if}
						</span>
					</li>
				{/each}
			</ul>
		</div>
	{/if}
</section>
