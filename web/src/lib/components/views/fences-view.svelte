<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import Button from '$lib/components/ui/button.svelte';
	import Input from '$lib/components/ui/input.svelte';
	import Modal from '$lib/components/ui/modal.svelte';
	import AppIcon from '$lib/components/ui/app-icon.svelte';
	import { cn } from '$lib/utils';
	import {
		apiFences,
		apiCreateFence,
		apiDeleteFence,
		apiDevices,
		ApiError,
		type DeviceItem,
		type Fence
	} from '$lib/api';

	// 围栏管理（仅 admin 菜单可见）：列表 + 弹窗新增（公共 Modal 组件）。
	let fences = $state<Fence[] | null>(null);
	let loading = $state(true);
	let listError = $state<string | null>(null);
	let notice = $state<string | null>(null);
	let viewError = $state<string | null>(null);
	let deletingId = $state<string | null>(null);
	let noticeTimer: ReturnType<typeof setTimeout> | undefined;

	// 新建圆形围栏（弹窗内表单）
	let createOpen = $state(false);
	let formName = $state('');
	let formLng = $state('');
	let formLat = $state('');
	let formRadius = $state('');
	let formError = $state<string | null>(null);
	let creating = $state(false);
	let bindIds = $state<string[]>([]);

	// 可绑定设备（source=local）
	let localDevices = $state<DeviceItem[] | null>(null);
	let localLoading = $state(true);
	let localError = $state<string | null>(null);

	const kindLabel: Record<string, string> = { circle: '圆形', polygon: '多边形' };

	function showNotice(text: string) {
		notice = text;
		if (noticeTimer) clearTimeout(noticeTimer);
		noticeTimer = setTimeout(() => (notice = null), 4500);
	}
	onDestroy(() => {
		if (noticeTimer) clearTimeout(noticeTimer);
	});

	async function loadFences() {
		loading = true;
		listError = null;
		try {
			const page = await apiFences();
			fences = page.items ?? [];
		} catch (e) {
			fences = null;
			listError = e instanceof ApiError ? e.message : String(e);
		} finally {
			loading = false;
		}
	}

	async function loadLocalDevices() {
		localLoading = true;
		localError = null;
		try {
			const page = await apiDevices({ source: 'local', limit: '200' });
			localDevices = page.items;
		} catch (e) {
			localDevices = null;
			localError = e instanceof ApiError ? e.message : String(e);
		} finally {
			localLoading = false;
		}
	}

	onMount(() => {
		void loadFences();
		void loadLocalDevices();
	});

	function polygonPoints(f: Fence): number {
		return f.polygon && f.polygon.length > 0 ? f.polygon.length : 0;
	}

	function geoSummary(f: Fence): string {
		if (f.kind === 'circle') {
			if (f.radius_m != null) return `半径 ${f.radius_m} 米`;
			return '圆形';
		}
		return `${polygonPoints(f)} 个顶点`;
	}

	function boundNames(f: Fence): string {
		const names = f.device_ids
			.map((id) => (localDevices ? localDevices.find((d) => d.id === id) : undefined))
			.filter((d): d is DeviceItem => !!d)
			.map((d) => d.name || d.external_id);
		return names.join('、');
	}

	async function remove(f: Fence) {
		const sure = window.confirm(`确定删除围栏「${f.name}」？删除后该围栏不再触发报警（设备不受影响）。`);
		if (!sure) return;
		deletingId = f.id;
		viewError = null;
		try {
			await apiDeleteFence(f.id);
			showNotice(`已删除围栏「${f.name}」`);
			await loadFences();
		} catch (e) {
			viewError = e instanceof ApiError ? e.message : String(e);
		} finally {
			deletingId = null;
		}
	}

	function openCreate() {
		resetForm();
		createOpen = true;
		void loadLocalDevices();
	}
	function closeCreate() {
		if (!creating) createOpen = false;
	}

	function resetForm() {
		formName = '';
		formLng = '';
		formLat = '';
		formRadius = '';
		bindIds = [];
		formError = null;
	}

	async function create() {
		const name = formName.trim();
		const lng = Number(formLng);
		const lat = Number(formLat);
		const radius = Number(formRadius);
		if (!name) return (formError = '请填写围栏名称');
		if (formLng === '' || !Number.isFinite(lng) || lng < -180 || lng > 180)
			return (formError = '中心经度需为 -180～180 之间的数字');
		if (formLat === '' || !Number.isFinite(lat) || lat < -90 || lat > 90)
			return (formError = '中心纬度需为 -90～90 之间的数字');
		if (formRadius === '' || !Number.isFinite(radius) || radius <= 0)
			return (formError = '半径需为大于 0 的数字（米）');
		creating = true;
		formError = null;
		viewError = null;
		try {
			await apiCreateFence({
				name,
				kind: 'circle',
				enabled: true,
				center_longitude: lng,
				center_latitude: lat,
				radius_m: radius,
				device_ids: bindIds
			});
			showNotice(`已创建围栏「${name}」`);
			createOpen = false;
			await loadFences();
		} catch (e) {
			formError = e instanceof ApiError ? e.message : String(e);
		} finally {
			creating = false;
		}
	}
</script>

<section aria-label="围栏管理" class="space-y-3">
	<div class="border-border bg-card flex flex-wrap items-center justify-between gap-3 rounded-xl border p-4 shadow-sm">
		<div>
			<h2 class="text-base font-semibold">围栏管理</h2>
			<p class="text-muted-foreground mt-0.5 text-xs">
				设备进出围栏由服务端判断（T08），关闭网页仍会触发报警。本页仅管理员可见。
			</p>
		</div>
		<div class="flex items-center gap-2">
			<Button size="sm" variant="outline" onclick={() => void loadFences()} disabled={loading}>
				{loading ? '刷新中…' : '刷新'}
			</Button>
			<Button size="sm" onclick={openCreate}>
				<AppIcon name="fence" class="size-4" />
				新建围栏
			</Button>
		</div>
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
		<div class="border-border bg-card text-muted-foreground rounded-xl border p-10 text-center text-sm">加载围栏中…</div>
	{:else if listError}
		<div class="border-border bg-card rounded-xl border p-8 text-center text-sm">
			<p role="alert" class="text-destructive">围栏加载失败：{listError}</p>
			<div class="mt-3"><Button size="sm" variant="outline" onclick={() => void loadFences()}>重试</Button></div>
		</div>
	{:else if !fences || fences.length === 0}
		<div class="border-border bg-card text-muted-foreground rounded-xl border p-10 text-center text-sm">
			还没有围栏，点击右上角“新建围栏”创建第一个圆形围栏。
		</div>
	{:else}
		<div class="border-border bg-card overflow-hidden rounded-xl border shadow-sm">
			<ul class="divide-border divide-y">
				{#each fences as f (f.id)}
					<li class="hover:bg-accent/40 flex flex-wrap items-center gap-x-4 gap-y-2 px-4 py-3 transition-colors">
						<span class="from-primary/15 to-primary/5 bg-gradient-to-br text-primary inline-flex size-10 shrink-0 items-center justify-center rounded-lg">
							<AppIcon name="fence" class="size-5" />
						</span>
						<div class="min-w-0 flex-1">
							<div class="flex flex-wrap items-center gap-2">
								<span class="font-medium">{f.name}</span>
								<span class={cn('rounded-full px-2 py-0.5 text-xs', 'bg-primary/10 text-primary')}>
									{kindLabel[f.kind] ?? f.kind}
								</span>
								<span
									class={cn(
										'rounded-full px-2 py-0.5 text-xs',
										f.enabled && 'bg-success/15 text-success-foreground',
										!f.enabled && 'bg-muted text-muted-foreground'
									)}
								>
									{f.enabled ? '启用' : '已停用'}
								</span>
							</div>
							<p class="text-muted-foreground mt-1 truncate text-xs" title={boundNames(f) || undefined}>
								{geoSummary(f)} · 绑定 {f.device_ids.length} 台设备{boundNames(f) ? `（${boundNames(f)}）` : ''}
							</p>
						</div>
						<Button
							size="sm"
							variant="destructive"
							disabled={deletingId === f.id}
							onclick={() => void remove(f)}
						>
							{deletingId === f.id ? '删除中…' : '删除'}
						</Button>
					</li>
				{/each}
			</ul>
		</div>
	{/if}
</section>

<!-- 新建围栏（公共弹窗） -->
{#if createOpen}
	<Modal
		title="新建圆形围栏"
		description="名称 + 中心经纬度（WGS84）+ 半径（米），可勾选要绑定的 local 设备。"
		size="md"
		onClose={closeCreate}
	>
			<form
				id="fence-create-form"
				onsubmit={(e) => {
					e.preventDefault();
					void create();
				}}
				class="flex flex-col gap-4"
			>
				<label class="flex flex-col gap-1.5 text-xs">
					<span class="text-muted-foreground">围栏名称（必填）</span>
					<Input bind:value={formName} placeholder="例如：厂区东门" required maxlength={200} autofocus />
				</label>
				<div class="grid grid-cols-2 gap-3">
					<label class="flex flex-col gap-1.5 text-xs">
						<span class="text-muted-foreground">中心经度（-180～180）</span>
						<Input bind:value={formLng} type="number" step="any" placeholder="116.4020" required />
					</label>
					<label class="flex flex-col gap-1.5 text-xs">
						<span class="text-muted-foreground">中心纬度（-90～90）</span>
						<Input bind:value={formLat} type="number" step="any" placeholder="39.9010" required />
					</label>
				</div>
				<label class="flex flex-col gap-1.5 text-xs">
					<span class="text-muted-foreground">半径（米，&gt;0）</span>
					<Input bind:value={formRadius} type="number" step="any" min={1} placeholder="100" required />
				</label>

				<div class="flex flex-col gap-1.5 text-xs">
					<p class="text-muted-foreground">绑定设备（local 自建，可空）</p>
					{#if localLoading}
						<p class="text-muted-foreground border-border bg-muted/30 rounded-lg border py-4 text-center">加载 local 设备中…</p>
					{:else if localError}
						<div class="text-destructive border-border bg-destructive/5 rounded-lg border px-3 py-3">
							<p role="alert">设备加载失败：{localError}</p>
							<Button class="mt-2" size="sm" variant="outline" onclick={() => void loadLocalDevices()}>重试</Button>
						</div>
					{:else if !localDevices || localDevices.length === 0}
						<p class="text-muted-foreground border-border bg-muted/30 rounded-lg border px-3 py-4 text-center">
							没有可绑定的 local 设备（可先创建空围栏，之后再绑定）。
						</p>
					{:else}
						<ul class="scrollbar-thin border-border bg-muted/20 max-h-44 overflow-y-auto rounded-lg border p-1.5">
							{#each localDevices as d (d.id)}
								<li>
									<label class="hover:bg-accent flex cursor-pointer items-center gap-2.5 rounded-md px-2 py-1.5">
										<input type="checkbox" value={d.id} bind:group={bindIds} class="accent-primary size-4" />
										<span class="min-w-0 flex-1">
											<span class="block truncate text-[13px] font-medium">{d.name || d.external_id}</span>
											<span class="text-muted-foreground block text-[11px]">{d.external_id}</span>
										</span>
									</label>
								</li>
							{/each}
						</ul>
					{/if}
				</div>

				{#if formError}
					<p role="alert" class="text-destructive text-xs">{formError}</p>
				{/if}
			</form>
		{#snippet footer()}
			<Button type="button" variant="outline" onclick={closeCreate} disabled={creating}>取消</Button>
			<Button type="submit" form="fence-create-form" disabled={creating || localLoading}>
				{creating ? '创建中…' : '创建围栏'}
			</Button>
		{/snippet}
	</Modal>
{/if}
