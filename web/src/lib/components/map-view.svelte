<script lang="ts">
	import { onMount } from 'svelte';
	import * as maplibregl from 'maplibre-gl';
	import type { MapLayerMouseEvent } from 'maplibre-gl';
	import type { DeviceItem, Track } from '$lib/api';
	import { resolveBasemap, wgs84ToGcj02, HOME_CENTER, HOME_ZOOM, USER_ZOOM } from '$lib/basemap';

	let {
		devices,
		selectedId,
		track,
		onSelectDevice
	}: {
		devices: DeviceItem[];
		selectedId: string | null;
		track: Track | null;
		onSelectDevice: (id: string) => void;
	} = $props();

	let container: HTMLDivElement;
	let map: maplibregl.Map | null = null;
	let mapError: string | null = $state(null);
	let tileWarn: string | null = $state(null);
	let ready = false;
	let resizeObs: ResizeObserver | undefined;

	// 国内底图（默认高德）：原始坐标 WGS84 只在地图显示层转换为 GCJ-02。
	const basemap = resolveBasemap();
	const toXY = (lng: number, lat: number): [number, number] =>
		basemap.gcj02 ? wgs84ToGcj02(lng, lat) : [lng, lat];
	// 默认视图：本仓库定位数据/eadm 对照中心（青岛），见 basemap.ts HOME_CENTER。
	const [homeLng, homeLat] = toXY(HOME_CENTER[0], HOME_CENTER[1]);

	// 位置源：无任何定位设备选中时，首页先尝试浏览器 geolocation（授权即用），
	// 失败/拒绝/超时自动回落默认中心；右下角“回默认中心”始终可手动复位。
	let geoDone = $state(false);

	function locateMe() {
		if (!map || geoDone) return;
		geoDone = true;
		if (!('geolocation' in navigator)) return;
		navigator.geolocation.getCurrentPosition(
			(pos) => {
				const [lng, lat] = toXY(pos.coords.longitude, pos.coords.latitude);
				map?.flyTo({ center: [lng, lat], zoom: USER_ZOOM, duration: 900 });
			},
			() => {
				/* 拒绝/超时：保持默认中心（可手动回落） */
			},
			{ timeout: 4000, maximumAge: 60_000 }
		);
	}
	function goHome() {
		if (!map) return;
		map.flyTo({ center: [homeLng, homeLat], zoom: HOME_ZOOM, duration: 600 });
	}

	onMount(() => {
		map = new maplibregl.Map({
			container,
			style: basemap.style,
			center: [homeLng, homeLat],
			zoom: HOME_ZOOM,
			attributionControl: basemap.hasTiles ? { compact: true } : false
		});
		map.addControl(new maplibregl.NavigationControl({ showCompass: true }), 'top-right');
		map.on('error', (e: { error?: unknown }) => {
			mapError = e?.error ? String((e.error as Error).message || e.error) : '地图渲染错误';
		});
		map.on('load', () => {
			ready = true;
			// 布局尺寸变化（侧栏收缩/面板折叠）时即时重设画布
			window.setTimeout(() => map?.resize(), 0);
			// 延迟探针：瓦片真实加载失败（Key/域名白名单/网络）时给出可读提示
			window.setTimeout(() => probeBaseTile(), 1500);
			map!.addSource('devices', { type: 'geojson', data: emptyFC() as never });
			map!.addLayer({
				id: 'device-circle',
				type: 'circle',
				source: 'devices',
				paint: {
					'circle-radius': ['case', ['==', ['get', 'selected'], true], 9, 6],
					'circle-color': [
						'case',
						['==', ['get', 'comm'], 'online'], '#16a34a',
						['==', ['get', 'comm'], 'offline'], '#d97706',
						'#6b7280'
					],
					'circle-stroke-width': 2,
					'circle-stroke-color': '#ffffff'
				}
			});
			map!.addSource('track', { type: 'geojson', data: emptyFC() as never });
			map!.addLayer({
				id: 'track-line',
				type: 'line',
				source: 'track',
				paint: { 'line-color': '#2563eb', 'line-width': 3, 'line-opacity': 0.9 }
			});
			map!.on('click', 'device-circle', (ev: MapLayerMouseEvent) => {
				const f = ev.features?.[0];
				if (f?.properties?.id) onSelectDevice(String(f.properties.id));
			});
			map!.on('mouseenter', 'device-circle', () => { if (map) map.getCanvas().style.cursor = 'pointer'; });
			map!.on('mouseleave', 'device-circle', () => { if (map) map.getCanvas().style.cursor = ''; });
			// 默认视图：无选中设备时尝试 geolocation，失败回落默认中心
			locateMe();
		});
		resizeObs = new ResizeObserver(() => map?.resize());
		if (container) resizeObs.observe(container);
		return () => {
			resizeObs?.disconnect();
			resizeObs = undefined;
			map?.remove();
			map = null;
			ready = false;
		};
	});

	function emptyFC(): Record<string, unknown> {
		return { type: 'FeatureCollection', features: [] };
	}

	// 用一个中心瓦片做真实加载探针：加载成功则清除告警，失败则提示
	// （此类错误不触发 maplibre 的 error 事件，需自行检测）。
	function probeBaseTile() {
		if (!map || !basemap.hasTiles) return;
		try {
			const sources = map.getStyle()?.sources as Record<string, { tiles?: string[] }> | undefined;
			const tpl = sources?.base?.tiles?.[0];
			if (!tpl) return;
			const c = map.getCenter();
			const z = Math.max(3, Math.min(18, Math.floor(map.getZoom())));
			const n = 2 ** z;
			const x = Math.min(n - 1, Math.max(0, Math.floor(((c.lng + 180) / 360) * n)));
			const latR = (c.lat * Math.PI) / 180;
			const y = Math.min(
				n - 1,
				Math.max(0, Math.floor(((1 - Math.log(Math.tan(latR) + 1 / Math.cos(latR)) / Math.PI) / 2) * n))
			);
			const url = tpl
				.replace('{x}', String(x))
				.replace('{y}', String(y))
				.replace('{z}', String(z))
				.replace('{r}', '');
			const img = new Image();
			img.onload = () => { if (tileWarn !== null) tileWarn = null; };
			img.onerror = () => {
				tileWarn = '底图瓦片加载失败：请检查高德 Key 是否生效、域名白名单与网络（浏览器 Console 有详情）；设备列表与坐标仍可用。';
			};
			img.src = url;
		} catch {
			/* 忽略探测异常，保持地图可用 */
		}
	}

	function refresh() {
		if (!map || !ready) return;
		const withPos = devices.filter((d) => d.position);
		const features = withPos.map((d) => {
			const [lng, lat] = toXY(d.position!.longitude, d.position!.latitude);
			return {
				type: 'Feature',
				geometry: { type: 'Point', coordinates: [lng, lat] },
				properties: { id: d.id, comm: d.communication_status, selected: d.id === selectedId, name: d.name || d.external_id }
			};
		});
		(map.getSource('devices') as maplibregl.GeoJSONSource | undefined)?.setData({
			type: 'FeatureCollection', features
		} as never);
		const tsrc = map.getSource('track') as maplibregl.GeoJSONSource | undefined;
		if (!tsrc) return;
		if (track && track.segments.length > 0) {
			const lines = track.segments.map((seg) => ({
				type: 'Feature',
				geometry: {
					type: 'LineString',
					coordinates: seg.points.map((p) => toXY(p.coordinates[0], p.coordinates[1]))
				},
				properties: {}
			}));
			tsrc.setData({ type: 'FeatureCollection', features: lines } as never);
			const pts = track.segments.flatMap((s) => s.points.map((p) => toXY(p.coordinates[0], p.coordinates[1])));
			if (pts.length > 0) {
				const lons = pts.map((c) => c[0]);
				const lats = pts.map((c) => c[1]);
				map.fitBounds(
					[[Math.min(...lons), Math.min(...lats)], [Math.max(...lons), Math.max(...lats)]],
					{ padding: 40, maxZoom: 16, duration: 400 }
				);
			}
		} else if (selectedId) {
			const sel = devices.find((d) => d.id === selectedId && d.position);
			if (sel && sel.position) {
				const [lng, lat] = toXY(sel.position.longitude, sel.position.latitude);
				map.flyTo({ center: [lng, lat], zoom: Math.max(map.getZoom(), 13), duration: 400 });
			}
		}
	}

	$effect(() => { refresh(); });
</script>

<div class="relative h-full w-full">
	{#if mapError}
		<div role="alert" class="bg-destructive/5 text-destructive absolute left-4 top-4 z-10 max-w-[80%] rounded-md border border-destructive/30 px-3 py-2 text-xs">
			底图渲染失败：{mapError} —— 仍可使用设备列表与坐标
		</div>
	{/if}
	{#if tileWarn}
		<div role="alert" class="bg-warning/10 text-warning-foreground absolute bottom-9 left-3 z-10 max-w-[70%] rounded-md border border-warning/40 px-2.5 py-1.5 text-[11px] shadow-sm backdrop-blur">
			{tileWarn}
		</div>
	{/if}
	<button
		type="button"
		aria-label="回到默认中心"
		title="回到默认中心（青岛）"
		class="bg-background/70 text-muted-foreground hover:bg-accent/80 hover:text-accent-foreground absolute bottom-8 left-3 z-10 rounded border border-white/20 px-2 py-1 text-[10px] shadow-sm backdrop-blur"
		onclick={goHome}
	>
		回默认中心
	</button>
	<div class="bg-background/70 text-muted-foreground absolute bottom-3 left-3 z-10 rounded px-1.5 py-0.5 text-[10px]">
		WGS84 入库 · {basemap.hasTiles ? `${basemap.label} GCJ-02 显示` : '无瓦片降级底图'}
	</div>
	<div bind:this={container} class="h-full min-h-64 w-full"></div>
</div>
