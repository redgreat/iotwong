<script lang="ts">
	// devices-view — 设备地图（主内容区满幅地图 + 右上角半透明设备列表/筛选卡片）。
	// 与 TrackView 共用 MapView；原始数据 WGS84，仅地图显示层转 GCJ-02（见 basemap.ts）。
	import { onDestroy, onMount } from 'svelte';
	import Button from '$lib/components/ui/button.svelte';
	import MapView from '$lib/components/map-view.svelte';
	import AppIcon from '$lib/components/ui/app-icon.svelte';
	import { apiDevices, apiTrack, ApiError, type DeviceItem, type Track } from '$lib/api';

// 视图在菜单切换时重建；缓存最近一次设备列表，保证回到本页时立即有内容
//（避免先空屏再加载的闪烁/竞态）。
let cachedDevices: DeviceItem[] = [];

	let devices = $state<DeviceItem[]>([]);
	let devicesError = $state<string | null>(null);
	let devicesLoading = $state(true);
	let filterSource = $state('');
	let filterStatus = $state('');
	let filterQ = $state('');
	let loadSeq = $state(0);

	let selectedId = $state<string | null>(null);
	let selectedDevice = $state<DeviceItem | null>(null);
	let from = $state('');
	let to = $state('');
	let track = $state<Track | null>(null);
	let trackError = $state<string | null>(null);
	let trackLoading = $state(false);

	// 面板折叠（移动端更实用）
	let panelCollapsed = $state(false);

	const commLabel: Record<string, string> = { online: '在线', offline: '离线', unknown: '未知' };
	const commDot: Record<string, string> = { online: 'bg-success', offline: 'bg-warning', unknown: 'bg-muted-foreground' };
	const dotCls = (s: string) => `inline-block size-2 shrink-0 rounded-full ${commDot[s] ?? 'bg-muted-foreground'}`;
	function shortTs(v: string): string {
		return new Date(v).toLocaleTimeString('zh-CN', { hour12: false }).slice(0, 5);
	}

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

	async function reloadDevices() {
		const seq = ++loadSeq;
		devicesLoading = true;
		devicesError = null;
		try {
			const params: Record<string, string> = { limit: '200' };
			if (filterSource) params.source = filterSource;
			if (filterStatus) params.status = filterStatus;
			if (filterQ) params.q = filterQ;
			const page = await apiDevices(params);
			if (seq !== loadSeq) return; // stale
			devices = page.items;
			cachedDevices = page.items;
			// 选中项失效时清理
			if (selectedId && !devices.some((d) => d.id === selectedId)) selectDevice(null);
		} catch (e) {
			if (seq === loadSeq) devicesError = e instanceof ApiError ? e.message : String(e);
		} finally {
			if (seq === loadSeq) devicesLoading = false;
		}
	}

	onMount(() => {
		if (cachedDevices.length > 0) {
			devices = cachedDevices;
			devicesLoading = false;
		}
		void reloadDevices();
	});

	function selectDevice(id: string | null) {
		selectedId = id;
		selectedDevice = id ? (devices.find((d) => d.id === id) ?? null) : null;
		if (selectedDevice) {
			const w = defaultWindow(selectedDevice);
			from = w.from;
			to = w.to;
		}
		resetTrack();
	}

	async function loadTrack() {
		if (!selectedId) return;
		trackLoading = true;
		trackError = null;
		try {
			track = await apiTrack(selectedId, toUtc(from), toUtc(to), 5000);
			stopPlay();
			playIdx = 0;
		} catch (e) {
			trackError = e instanceof ApiError ? e.message : String(e);
		} finally {
			trackLoading = false;
		}
	}
	function resetTrack() {
		track = null;
		trackError = null;
		stopPlay();
		playIdx = 0;
	}

	const emptyTrack = $derived(!track || track.segments.length === 0);
	const trackPts = $derived(track ? track.segments.flatMap((seg) => seg.points) : []);
	let editingWindow = $state(false);
	const windowText = $derived(`${from} ~ ${to}`);

	// 回放
	let playIdx = $state(0);
	let playSpeed = $state(10);
	let playing = $state(false);
	let playTimer: ReturnType<typeof setInterval> | undefined;

	function stopPlay() {
		if (playTimer) { clearInterval(playTimer); playTimer = undefined; }
		playing = false;
	}
	function togglePlay() {
		if (trackPts.length === 0) return;
		if (playing) { stopPlay(); return; }
		if (playIdx >= trackPts.length) playIdx = 0;
		playing = true;
		playTimer = setInterval(() => {
			playIdx += 1;
			if (playIdx >= trackPts.length) stopPlay();
		}, Math.max(40, Math.round(1000 / playSpeed)));
	}
	onDestroy(stopPlay);
	const curPt = $derived(trackPts[Math.min(playIdx, Math.max(trackPts.length - 1, 0))] ?? null);

	const onlineCount = $derived(devices.filter((d) => d.communication_status === 'online').length);
	const totalPos = $derived(devices.filter((d) => d.position).length);
</script>

<div class="relative h-[calc(100dvh-3.5rem)] w-full overflow-hidden">
	<!-- 满幅地图 -->
	<div class="absolute inset-0" aria-label="设备地图底图">
		<MapView devices={devices} selectedId={selectedId} track={track} onSelectDevice={(id) => selectDevice(id)} />
	</div>

	<!-- 右上角设备列表/筛选悬浮卡片（半透明玻璃态；折叠后仅留图标） -->
	<section
		aria-label="设备列表"
		class="border-border/40 bg-card/60 backdrop-blur-xl absolute right-3 top-3 z-10 flex flex-col overflow-hidden rounded-2xl border shadow-2xl transition-all duration-300"
		class:bottom-3={!panelCollapsed}
		class:w-[300px]={!panelCollapsed}
		class:xl:w-[340px]={!panelCollapsed}
		class:w-12={panelCollapsed}
	>
		<!-- 面板头 -->
		{#if panelCollapsed}
			<!-- 折叠态：仅留“设备列表”图标 -->
			<button
				type="button"
				class="hover:bg-accent/60 text-muted-foreground mx-auto my-1.5 flex size-9 items-center justify-center rounded-xl"
				aria-label="展开设备列表"
				title="展开设备列表"
				onclick={() => { panelCollapsed = false; stopPlay(); }}
			>
				<AppIcon name="list" class="size-5" />
			</button>
		{:else}
			<div class="flex items-center justify-between gap-2 border-b border-white/10 px-3 py-2.5">
				<h2 class="flex min-w-0 items-center gap-1.5 whitespace-nowrap text-[13px] font-semibold leading-none">
					<AppIcon name="map" class="text-primary size-4 shrink-0" />
					<span>设备</span>
					<span class="text-muted-foreground min-w-0 truncate text-[11px] font-normal">
						{devices.length} 台 · {onlineCount} 在线 · {totalPos} 有定位
					</span>
				</h2>
				<button
					type="button"
					class="hover:bg-accent text-muted-foreground inline-flex size-7 shrink-0 items-center justify-center rounded-md"
					aria-label="收起设备面板"
					onclick={() => { panelCollapsed = true; stopPlay(); }}
				>
					<AppIcon name="chevrons-right" class="size-4" />
				</button>
			</div>
		{/if}

		{#if !panelCollapsed}
			<!-- 查询/播放器内容随“播放中”压缩：回放时只保留播放卡片 -->
			<div class="min-h-0 flex-1 overflow-y-auto">
				{#if !playing}
					<!-- 筛选（数据源/状态 + 搜索） -->
					<div class="flex flex-col gap-2 px-3 pt-2.5">
						<div class="grid grid-cols-2 gap-2">
							<select
								aria-label="数据源"
								bind:value={filterSource}
								onchange={() => { editingWindow = false; void reloadDevices(); }}
								class="border-white/30 bg-white/50 text-foreground dark:bg-black/30 h-8 w-full rounded-lg border px-2 text-xs outline-none backdrop-blur focus:border-primary/60"
							>
								<option value="">全部数据源</option>
								<option value="local">local 自建</option>
								<option value="racebox_demo">RaceBox 演示</option>
								<option value="luatos">luatos</option>
							</select>
							<select
								aria-label="通信状态"
								bind:value={filterStatus}
								onchange={() => { editingWindow = false; void reloadDevices(); }}
								class="border-white/30 bg-white/50 text-foreground dark:bg-black/30 h-8 w-full rounded-lg border px-2 text-xs outline-none backdrop-blur focus:border-primary/60"
							>
								<option value="">全部状态</option>
								<option value="online">在线</option>
								<option value="offline">离线</option>
								<option value="unknown">未知</option>
							</select>
						</div>
						<input
							aria-label="搜索"
							bind:value={filterQ}
							placeholder="按名称/标识搜索…"
							oninput={() => { editingWindow = false; void reloadDevices(); }}
							class="border-white/30 bg-white/50 placeholder:text-muted-foreground/70 text-foreground dark:bg-black/30 h-8 w-full rounded-lg border px-2.5 text-xs outline-none backdrop-blur focus:border-primary/60"
						/>
					</div>

					<!-- 设备列表 -->
					<div class="mt-2.5 min-h-0 flex-1 overflow-y-auto px-1.5">
						{#if devicesLoading && devices.length === 0}
							<p class="text-muted-foreground py-6 text-center text-xs">加载设备中…</p>
						{:else if devicesError}
							<div role="alert" class="px-3 py-4 text-center">
								<p class="text-destructive text-xs">{devicesError}</p>
								<Button class="mt-2" size="sm" variant="outline" onclick={() => void reloadDevices()}>重试</Button>
							</div>
						{:else if devices.length === 0}
							<p class="text-muted-foreground py-6 text-center text-xs">没有符合条件的设备</p>
						{:else}
							<ul class="flex flex-col gap-1">
								{#each devices as d (d.id)}
									<!-- “历史回放”徽章放在按钮之外，避免与回放播放按钮的文案冲突 -->
									<li class="relative">
										{#if d.is_demo}
											<span class="bg-warning/25 text-warning-foreground pointer-events-none absolute right-1.5 top-1.5 z-[1] rounded px-1.5 py-0.5 text-[10px]">历史回放</span>
										{/if}
										<button
											type="button"
											onclick={() => selectDevice(d.id)}
											class={[
												'w-full rounded-xl border px-2.5 py-2 pr-14 text-left transition-colors',
												d.id === selectedId
													? 'bg-primary/15 border-primary/50'
													: 'border-white/10 bg-white/40 hover:bg-accent/70 dark:bg-black/20'
											].join(' ')}
										>
											<span class="flex items-center gap-2 text-[13px] font-medium">
												<span class={dotCls(d.communication_status)}></span>
												<span class="min-w-0 flex-1 truncate">{d.name || d.external_id}</span>
											</span>
											<span class="text-muted-foreground mt-0.5 flex flex-wrap gap-x-2.5 text-[11px]">
												<span>{commLabel[d.communication_status]}</span>
												{#if d.position}
													<span class="tabular-nums">
														{d.position.latitude.toFixed(4)}, {d.position.longitude.toFixed(4)}
													</span>
													{#if d.last_position_at}
														<span class="tabular-nums">{shortTs(d.last_position_at)}</span>
													{/if}
												{:else}
													<span>无定位（不放 0,0）</span>
												{/if}
											</span>
										</button>
									</li>
								{/each}
							</ul>
						{/if}
					</div>

					<!-- 时间窗查询（仅当未加载轨迹时出现） -->
					{#if track === null}
						<div class="border-t border-white/10 px-3 py-2.5">
							{#if selectedDevice}
								<p class="mb-1.5 flex items-center justify-between text-[11px] text-muted-foreground">
									<span class="truncate font-medium text-foreground/90">{selectedDevice.name || selectedDevice.external_id}</span>
									<span class="shrink-0">查询轨迹时间窗</span>
								</p>
								<div class="grid grid-cols-2 gap-1.5">
									<label class="flex flex-col gap-1 text-[10px] text-muted-foreground">
										起
										<input type="datetime-local" bind:value={from} class="glass-dt" />
									</label>
									<label class="flex flex-col gap-1 text-[10px] text-muted-foreground">
										止
										<input type="datetime-local" bind:value={to} class="glass-dt" />
									</label>
								</div>
								<Button class="mt-2 w-full" size="sm" onclick={() => void loadTrack()} disabled={trackLoading}>
									{trackLoading ? '查询中…' : '查询轨迹'}
								</Button>
							{:else}
								<p class="text-muted-foreground py-1 text-center text-[11px]">从上方设备列表选择一台设备</p>
							{/if}
							{#if trackError}
								<p role="alert" class="text-destructive mt-2 text-xs">{trackError}</p>
							{/if}
						</div>
					{/if}

					<!-- 已加载轨迹：摘要 + 回放条（默认隐藏日期弹窗，仅一行摘要；可展开时间窗） -->
					{#if track !== null}
						<div class="border-t border-white/10 px-3 py-2.5">
							<p class="mb-1.5 text-[11px] text-muted-foreground">
								{#if emptyTrack}
									该时间窗内没有轨迹数据（点选无定位设备只显示状态）。
								{:else}
									原始点 {track.raw_count}，返回 {track.returned_count}{track.simplified ? '（已抽稀）' : ''}，{track.segments.length} 段
									· {windowText}
								{/if}
							</p>
							{#if emptyTrack}
								<Button size="sm" variant="outline" onclick={() => resetTrack()}>返回选择</Button>
							{:else}
								{#if !editingWindow}
									<div class="flex flex-wrap items-center gap-2 rounded-xl border border-white/10 bg-white/30 p-1.5 dark:bg-black/20">
										<Button
											size="sm"
											variant={playing ? 'destructive' : 'default'}
											onclick={togglePlay}
											disabled={trackPts.length === 0}
										>
											{playing ? '停止' : '回放'}
										</Button>
										<label class="flex items-center gap-1 text-[11px] text-muted-foreground">
											倍速
											<select bind:value={playSpeed} aria-label="回放倍速" class="glass-dt h-7 w-auto px-1">
												<option value={1}>1×</option>
												<option value={10}>10×</option>
												<option value={60}>60×</option>
											</select>
										</label>
										<input
											type="range"
											min={0}
											max={Math.max(trackPts.length - 1, 0)}
											step={1}
											bind:value={playIdx}
											aria-label="播放进度"
											class="min-w-16 flex-1 accent-primary"
											disabled={playing}
										/>
										<span class="text-muted-foreground tabular-nums text-[11px]">
											{Math.min(playIdx + 1, trackPts.length)}/{trackPts.length}
										</span>
									</div>
								{:else}
									<div class="grid grid-cols-2 gap-1.5">
										<label class="flex flex-col gap-1 text-[10px] text-muted-foreground">
											起
											<input type="datetime-local" bind:value={from} class="glass-dt" />
										</label>
										<label class="flex flex-col gap-1 text-[10px] text-muted-foreground">
											止
											<input type="datetime-local" bind:value={to} class="glass-dt" />
										</label>
									</div>
									<div class="mt-2 flex gap-2">
										<Button size="sm" variant="outline" onclick={() => (editingWindow = false)}>取消</Button>
										<Button size="sm" onclick={() => { editingWindow = false; void loadTrack(); }}>重新查询</Button>
									</div>
								{/if}
								<button
									type="button"
									class="text-muted-foreground hover:text-accent-foreground mt-2 inline-flex items-center gap-1 text-[11px]"
									onclick={() => (editingWindow = !editingWindow)}
								>
									<AppIcon name="route" class="size-3.5" />
									{editingWindow ? '收起时间窗' : '修改时间窗'}
								</button>
							{/if}
						</div>
					{/if}
				{:else}
					<!-- 回放中：只保留播放卡片 -->
					<div class="flex min-h-[150px] flex-col justify-center gap-3 px-4 py-5">
						<p class="flex items-center gap-2 text-sm font-medium">
							<span class={dotCls(selectedDevice?.communication_status ?? 'unknown')}></span>
							<span class="truncate">{selectedDevice?.name || selectedDevice?.external_id || '设备'}</span>
							<span class="bg-primary/15 text-primary rounded-full px-2 py-0.5 text-[10px]">回放中</span>
						</p>
						{#if curPt}
							<p class="text-muted-foreground text-[11px]">
								{new Date(curPt.recorded_at).toLocaleString('zh-CN', { hour12: false })}
								· {curPt.coordinates[1].toFixed(5)}, {curPt.coordinates[0].toFixed(5)}
							</p>
						{/if}
						<div class="flex items-center gap-2">
							<Button size="sm" variant="destructive" onclick={stopPlay}>停止</Button>
							<input
								type="range"
								min={0}
								max={Math.max(trackPts.length - 1, 0)}
								step={1}
								bind:value={playIdx}
								aria-label="播放进度"
								class="min-w-0 flex-1 accent-primary"
							/>
							<span class="text-muted-foreground tabular-nums text-[11px]">
								{Math.min(playIdx + 1, trackPts.length)}/{trackPts.length}
							</span>
						</div>
					</div>
				{/if}
			</div>
		{/if}
	</section>
</div>
