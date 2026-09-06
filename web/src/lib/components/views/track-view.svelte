<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import Button from '$lib/components/ui/button.svelte';
	import MapView from '$lib/components/map-view.svelte';
	import { apiDevices, apiTrack, ApiError, type DeviceItem, type Track } from '$lib/api';

	// 轨迹视图：设备选择 + 时间窗 + 查询/回放（UI-REF-05 信息组织方式，R07）。
	let devices = $state<DeviceItem[] | null>(null);
	let loading = $state(true);
	let listError = $state<string | null>(null);

	let selectedId = $state<string | null>(null);
	let from = $state('');
	let to = $state('');
	let track = $state<Track | null>(null);
	let trackError = $state<string | null>(null);
	let trackLoading = $state(false);

	const commLabel: Record<string, string> = { online: '在线', offline: '离线', unknown: '未知' };
	const selectedDevice = $derived(devices?.find((d) => d.id === selectedId) ?? null);

	function toLocalInput(t: Date): string {
		const p = (n: number) => String(n).padStart(2, '0');
		return `${t.getFullYear()}-${p(t.getMonth() + 1)}-${p(t.getDate())}T${p(t.getHours())}:${p(t.getMinutes())}`;
	}
	function toUtc(local: string): string {
		return new Date(local).toISOString();
	}
	function defaultWindow(d: DeviceItem) {
		const end = d.last_position_at ? new Date(d.last_position_at) : new Date();
		const start = new Date(end.getTime() - 30 * 60 * 1000);
		return { from: toLocalInput(start), to: toLocalInput(end) };
	}

	async function loadDevices() {
		loading = true;
		listError = null;
		try {
			const page = await apiDevices({ limit: '200' });
			devices = page.items;
			if (devices.length > 0) {
				const first = devices.find((d) => d.last_position_at) ?? devices[0];
				selectedId = first.id;
				const w = defaultWindow(first);
				from = w.from;
				to = w.to;
			} else {
				selectedId = null;
			}
		} catch (e) {
			devices = null;
			listError = e instanceof ApiError ? e.message : String(e);
		} finally {
			loading = false;
		}
	}

	onMount(() => void loadDevices());

	function pick(id: string) {
		selectedId = id;
		const d = devices?.find((x) => x.id === id);
		if (d) {
			const w = defaultWindow(d);
			from = w.from;
			to = w.to;
		}
		track = null;
		trackError = null;
	}

	async function queryTrack() {
		if (!selectedId) return;
		trackLoading = true;
		trackError = null;
		try {
			track = await apiTrack(selectedId, toUtc(from), toUtc(to), 5000);
			stopPlay();
			playIdx = 0;
		} catch (e) {
			track = null;
			trackError = e instanceof ApiError ? e.message : String(e);
		} finally {
			trackLoading = false;
		}
	}

	const emptyTrack = $derived(!track || track.segments.length === 0);
	const trackPts = $derived(track ? track.segments.flatMap((seg) => seg.points) : []);
	let playIdx = $state(0);
	let playSpeed = $state(10);
	let playing = $state(false);
	let playTimer: ReturnType<typeof setInterval> | undefined;

	function stopPlay() {
		if (playTimer) {
			clearInterval(playTimer);
			playTimer = undefined;
		}
		playing = false;
	}
	function togglePlay() {
		if (trackPts.length === 0) return;
		if (playing) {
			stopPlay();
			return;
		}
		if (playIdx >= trackPts.length) playIdx = 0;
		playing = true;
		playTimer = setInterval(() => {
			playIdx += 1;
			if (playIdx >= trackPts.length) stopPlay();
		}, Math.max(40, Math.round(1000 / playSpeed)));
	}
	onDestroy(stopPlay);
	const curPt = $derived(trackPts[Math.min(playIdx, Math.max(trackPts.length - 1, 0))] ?? null);
	const selectCls =
		'border-input bg-background h-9 rounded-md border px-2 text-sm';
</script>

<section aria-label="轨迹查询" class="space-y-3">
	<div class="border-border bg-card rounded-lg border p-3">
		<h2 class="text-base font-semibold">轨迹</h2>
		<p class="text-muted-foreground mt-0.5 text-xs">
			按设备与时间窗查询历史轨迹（原始点/抽稀由服务端决定），支持回放。
		</p>

		{#if loading}
			<p class="text-muted-foreground py-6 text-center text-sm">加载设备中…</p>
		{:else if listError}
			<div class="text-muted-foreground py-4 text-center text-sm">
				<p role="alert" class="text-destructive">{listError}</p>
				<div class="mt-2"><Button size="sm" variant="outline" onclick={() => void loadDevices()}>重试</Button></div>
			</div>
		{:else if !devices || devices.length === 0}
			<p class="text-muted-foreground py-6 text-center text-sm">没有设备可查询轨迹。</p>
		{:else}
			<div class="mt-3 flex flex-wrap items-end gap-2">
				<label class="flex flex-col gap-1 text-xs">
					<span class="text-muted-foreground">设备</span>
					<select bind:value={selectedId} aria-label="选择设备" class={selectCls} onchange={() => { if (selectedId) pick(selectedId); }}>
						{#each devices as d (d.id)}
							<option value={d.id}>{d.name || d.external_id}（{d.external_id}）</option>
						{/each}
					</select>
				</label>
				<label class="flex flex-col gap-1 text-xs">
					<span class="text-muted-foreground">起</span>
					<input type="datetime-local" bind:value={from} class="border-input bg-background h-9 rounded-md border px-2 text-sm" />
				</label>
				<label class="flex flex-col gap-1 text-xs">
					<span class="text-muted-foreground">止</span>
					<input type="datetime-local" bind:value={to} class="border-input bg-background h-9 rounded-md border px-2 text-sm" />
				</label>
				<Button onclick={() => void queryTrack()} disabled={trackLoading || !selectedId}>
					{trackLoading ? '查询中…' : '查询轨迹'}
				</Button>
			</div>
		{/if}
	</div>

	<div class="border-border bg-card min-h-[420px] rounded-lg border p-2">
		<div class="mb-2 flex flex-wrap items-center gap-2 text-sm">
			{#if selectedDevice}
				<span class="font-medium">{selectedDevice.name || selectedDevice.external_id}</span>
				<span class="text-muted-foreground text-xs">
					{selectedDevice.source} · {commLabel[selectedDevice.communication_status]}
					{#if selectedDevice.position}
						· {selectedDevice.position.latitude.toFixed(5)}, {selectedDevice.position.longitude.toFixed(5)}
					{:else}· 无定位（不放 0,0）{/if}
				</span>
			{:else}
				<span class="text-muted-foreground">选择设备后查询其历史轨迹</span>
			{/if}
		</div>

		<div class="relative flex-1">
			<div class="h-[46vh] min-h-[300px] overflow-hidden rounded-md">
				<MapView devices={devices ?? []} selectedId={selectedId} track={track} onSelectDevice={(id) => pick(id)} />
			</div>

			{#if trackError}
				<p role="alert" class="text-destructive mt-2 text-xs">{trackError}</p>
			{:else if track}
				<div class="text-muted-foreground mt-2 text-xs">
					{#if emptyTrack}
						该时间窗内没有轨迹数据（无定位设备只显示状态）。
					{:else}
						原始点 {track.raw_count}，返回 {track.returned_count}{track.simplified ? '（已抽稀）' : ''}，{track.segments.length} 段
						（5 分钟以上空洞自动断段）
					{/if}
				</div>
				{#if !emptyTrack}
					<div class="mt-2 flex flex-wrap items-center gap-2 rounded-md border p-2 text-xs">
						<Button size="sm" variant={playing ? 'destructive' : 'default'} onclick={togglePlay} disabled={trackPts.length === 0}>
							{playing ? '停止' : '回放'}
						</Button>
						<label class="text-muted-foreground flex items-center gap-1">
							倍速
							<select bind:value={playSpeed} aria-label="回放倍速" class="border-input bg-background h-7 rounded-md border px-1 text-xs">
								<option value={1}>1×</option>
								<option value={10}>10×</option>
								<option value={60}>60×</option>
							</select>
						</label>
						<input
							class="min-w-32 flex-1"
							type="range" min={0} max={Math.max(trackPts.length - 1, 0)} step={1}
							bind:value={playIdx} aria-label="播放进度"
							disabled={playing}
						/>
						<span class="text-muted-foreground tabular-nums">
							{Math.min(playIdx + 1, trackPts.length)}/{trackPts.length}
						</span>
						{#if curPt}
							<span class="text-muted-foreground">
								{new Date(curPt.recorded_at).toLocaleString('zh-CN', { hour12: false })}
								· {curPt.coordinates[1].toFixed(5)}, {curPt.coordinates[0].toFixed(5)}
							</span>
						{/if}
					</div>
				{/if}
			{/if}
		</div>
	</div>
</section>
