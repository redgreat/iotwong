<script lang="ts">
	// device-manage-view — 设备管理（列表页）：展示全部设备的详细静态信息。
	// 与“设备地图”页共用同一份 API 数据；只读列表，样式走公共 DataPage。
	import { onMount } from 'svelte';
	import Button from '$lib/components/ui/button.svelte';
	import DataPage from '$lib/components/ui/data-page.svelte';
		import { apiDevices, ApiError, type DeviceItem } from '$lib/api';

	let devices = $state<DeviceItem[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let filterSource = $state('');
	let filterStatus = $state('');
	let filterQ = $state('');
	let seq = $state(0);

	const commLabel: Record<string, string> = { online: '在线', offline: '离线', unknown: '未知' };
	const sourceLabel: Record<string, string> = { local: 'local 自建', racebox_demo: 'RaceBox 演示', luatos: 'luatos' };
	const locLabel: Record<string, string> = { gnss: 'GNSS', lbs: '基站', wifi: 'Wi-Fi', unknown: '未知' };
	const kmh = (v: number | null) => (v == null ? '—' : `${(v * 3.6).toFixed(1)} km/h`);
	const pct = (v: number | null) => (v == null ? '—' : `${Math.round(v)}%`);
	const fmt = (v: string | null) => (v ? new Date(v).toLocaleString('zh-CN', { hour12: false }) : '—');

	async function load() {
		const my = ++seq;
		loading = true;
		error = null;
		try {
			const params: Record<string, string> = { limit: '200' };
			if (filterSource) params.source = filterSource;
			if (filterStatus) params.status = filterStatus;
			if (filterQ) params.q = filterQ;
			const page = await apiDevices(params);
			if (my !== seq) return;
			devices = page.items;
		} catch (e) {
			if (my === seq) error = e instanceof ApiError ? e.message : String(e);
		} finally {
			if (my === seq) loading = false;
		}
	}
	onMount(() => void load());

	const stat = $derived({
		total: devices.length,
		online: devices.filter((d) => d.communication_status === 'online').length,
		offline: devices.filter((d) => d.communication_status === 'offline').length,
		positioned: devices.filter((d) => d.position).length
	});
	const selCls = 'border-input bg-background h-9 rounded-md border px-2 text-sm';
</script>

<DataPage title="设备管理" description="当前租户全部数据源的设备静态信息（只读）；定位状态与时间来自最近一次上报。">
	{#snippet actions()}
		<Button size="sm" variant="outline" onclick={() => void load()} disabled={loading}>
			{loading ? '刷新中…' : '刷新'}
		</Button>
	{/snippet}

		<div class="border-border bg-card overflow-hidden rounded-xl border shadow-sm">
			<div class="flex flex-wrap items-center gap-3 border-b border-white/10 px-4 py-3">
				<span class="text-muted-foreground flex items-center gap-3 text-xs">
					<span>共 <b class="text-foreground">{stat.total}</b> 台</span>
					<span class="flex items-center gap-1"><span class="bg-success inline-block size-2 rounded-full"></span>{stat.online} 在线</span>
					<span class="flex items-center gap-1"><span class="bg-warning inline-block size-2 rounded-full"></span>{stat.offline} 离线</span>
					<span class="flex items-center gap-1"><span class="bg-primary inline-block size-2 rounded-full"></span>{stat.positioned} 有定位</span>
				</span>
				<span class="ml-auto flex flex-wrap items-center gap-2">
					<select aria-label="数据源" bind:value={filterSource} class={selCls} onchange={() => void load()}>
						<option value="">全部数据源</option>
						<option value="local">local 自建</option>
						<option value="racebox_demo">RaceBox 演示</option>
						<option value="luatos">luatos</option>
					</select>
					<select aria-label="通信状态" bind:value={filterStatus} class={selCls} onchange={() => void load()}>
						<option value="">全部状态</option>
						<option value="online">在线</option>
						<option value="offline">离线</option>
						<option value="unknown">未知</option>
					</select>
					<input
						aria-label="搜索"
						placeholder="按名称/标识搜索…"
						bind:value={filterQ}
						oninput={() => void load()}
						class="border-input bg-background h-9 w-48 rounded-md border px-2.5 text-sm outline-none"
					/>
				</span>
			</div>

			{#if loading && devices.length === 0}
				<p class="text-muted-foreground p-10 text-center text-sm">加载设备中…</p>
			{:else if error}
				<div class="p-10 text-center text-sm">
					<p role="alert" class="text-destructive">{error}</p>
					<Button class="mt-3" size="sm" variant="outline" onclick={() => void load()}>重试</Button>
				</div>
			{:else if devices.length === 0}
				<p class="text-muted-foreground p-10 text-center text-sm">没有符合条件的设备</p>
			{:else}
				<div class="overflow-x-auto">
					<table class="w-full min-w-[880px] text-left text-sm">
						<thead>
							<tr class="text-muted-foreground border-b bg-muted/30 text-xs">
								<th class="px-4 py-2.5 font-medium">设备</th>
								<th class="px-4 py-2.5 font-medium">数据源</th>
								<th class="px-4 py-2.5 font-medium">状态</th>
								<th class="px-4 py-2.5 font-medium">最近定位时间</th>
								<th class="px-4 py-2.5 font-medium">坐标（WGS84）</th>
								<th class="px-4 py-2.5 font-medium">速度</th>
								<th class="px-4 py-2.5 font-medium">电量</th>
								<th class="px-4 py-2.5 font-medium">定位来源</th>
							</tr>
						</thead>
						<tbody class="divide-border divide-y">
							{#each devices as d (d.id)}
								<tr class="hover:bg-accent/40 transition-colors">
									<td class="px-4 py-2.5">
										<span class="flex items-center gap-2">
											<span
												class="inline-block size-2 shrink-0 rounded-full"
												class:bg-success={d.communication_status === 'online'}
												class:bg-warning={d.communication_status === 'offline'}
												class:bg-muted-foreground={d.communication_status === 'unknown'}
											></span>
											<span class="min-w-0">
												<span class="block max-w-56 truncate font-medium">{d.name || d.external_id}</span>
												<span class="text-muted-foreground block text-[11px]">{d.external_id}</span>
											</span>
											{#if d.is_demo}
												<span class="bg-warning/25 text-warning-foreground shrink-0 rounded px-1.5 py-0.5 text-[10px]">历史回放</span>
											{/if}
										</span>
									</td>
									<td class="px-4 py-2.5">
										<span class="border-border bg-background inline-flex rounded-md border px-2 py-0.5 text-xs">{sourceLabel[d.source] ?? d.source}</span>
									</td>
									<td class="px-4 py-2.5">
										<span
											class={[
												'rounded-full px-2 py-0.5 text-xs',
												d.communication_status === 'online' && 'bg-success/15 text-success-foreground',
												d.communication_status === 'offline' && 'bg-warning/15 text-warning-foreground',
												d.communication_status === 'unknown' && 'bg-muted text-muted-foreground'
											].filter(Boolean).join(' ')}
										>
											{commLabel[d.communication_status]}
										</span>
									</td>
									<td class="text-muted-foreground px-4 py-2.5 tabular-nums">{fmt(d.last_position_at)}</td>
									<td class="px-4 py-2.5">
										{#if d.position}
											<span class="text-muted-foreground font-mono text-xs tabular-nums">
												{d.position.latitude.toFixed(6)}, {d.position.longitude.toFixed(6)}
											</span>
										{:else}
											<span class="text-muted-foreground">无定位（不放 0,0）</span>
										{/if}
									</td>
									<td class="px-4 py-2.5 tabular-nums">{kmh(d.speed_mps)}</td>
									<td class="px-4 py-2.5 tabular-nums">{pct(d.battery_pct)}</td>
									<td class="text-muted-foreground px-4 py-2.5">
										{#if d.location_source}
											{locLabel[d.location_source] ?? d.location_source}
										{:else}—{/if}
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/if}
		</div>

</DataPage>
