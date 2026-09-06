/**
 * basemap.ts — 国内底图与坐标显示转换。
 *
 * 产品需求：可配置国内厂商底图；默认离线（合法瓦片由部署方显式配置）。原始数据始终为 WGS84
 * （docs/03-data-design.md，接口输出 WGS84）；高德栅格瓦片为 GCJ-02，
 * 因此仅在地图“显示层”把点/线坐标转换为 GCJ-02，列表与接口文本仍显示
 * WGS84 原始值，避免“二次坐标偏移”（A15）。
 *
 * 瓦片供应商与 Key 由构建期环境注入（静态 SPA）：
 *   VITE_MAP_PROVIDER   offline（默认）| amap
 *   VITE_MAP_KEY        高德开放平台 Web端 Key；仅 provider=amap 且提供 Key 时启用
 *                       （AGENTS/ADR-004：不默认调用未授权第三方瓦片）
 */
import type { StyleSpecification } from 'maplibre-gl';

export type BasemapProvider = 'offline' | 'amap';

export interface Basemap {
	provider: BasemapProvider;
	label: string;
	/** 栅格瓦片是否基于 GCJ-02（叠加层需要转换显示坐标） */
	gcj02: boolean;
	/** 底图是否由真实瓦片提供（false = 内联灰底，仅开发降级） */
	hasTiles: boolean;
	style: StyleSpecification;
	attribution: string;
}

const AMAP_HOSTS = ['webrd01.is.autonavi.com', 'webrd02.is.autonavi.com', 'webrd03.is.autonavi.com', 'webrd04.is.autonavi.com'];

/**
 * 默认视图（无用户定位授权时的回落点）。
 * 采用本仓库定位数据集/内部对照项目 eadm 轨迹页的中心（青岛 [120.527112, 36.411864]，
 * 初始 zoom 11；eadm 为官方 AMap JS API + amap://styles/fresh，本仓库为 maplibre 栅格
 * 等效实现）。坐标按 WGS84 记，显示层与其它叠加物一致走 GCJ-02 转换。
 */
export const HOME_CENTER: [number, number] = [120.527112, 36.411864];
export const HOME_ZOOM = 11;
/** geolocation 授权成功后定位到当前位置的级别（对齐 eadm setZoomAndCenter(13,…)） */
export const USER_ZOOM = 13;

function env(): Record<string, string | undefined> {
	return (import.meta as unknown as { env?: Record<string, string | undefined> }).env ?? {};
}

function amapStyle(key: string | undefined): StyleSpecification {
	// 高德开放栅格瓦片（webrd0X 路网子域，style=8 标准配色）：无需 JS API 域白名单，
	// 服务端返回 access-control-allow-origin:* 且 key 可省略（实测 200 image/png，
	// z2..20 均可用）；仍按配置注入 key/保留归属。GCJ-02 坐标系 → 叠加层必须转坐标。
	const q = 'lang=zh_cn&size=1&scale=1&style=8' + (key ? `&key=${encodeURIComponent(key)}` : '');
	const tiles = AMAP_HOSTS.map((h) => `https://${h}/appmaptile?${q}&x={x}&y={y}&z={z}`);
	return {
		version: 8,
		name: 'amap-raster',
		sources: {
			base: {
				type: 'raster',
				tiles,
				tileSize: 256,
				minzoom: 3,
				maxzoom: 18,
				attribution: '© 高德地图'
			}
		},
		layers: [{ id: 'base-raster', type: 'raster', source: 'base' }]
	};
}

function offlineBasemap(): Basemap {
	return {
		provider: 'offline',
		label: '离线开发底图',
		gcj02: false,
		hasTiles: false,
		style: {
			version: 8,
			name: 'iotwong-offline',
			sources: {},
			layers: [{ id: 'bg', type: 'background', paint: { 'background-color': '#eef1f4' } }]
		},
		attribution: ''
	};
}

export function resolveBasemap(): Basemap {
	const provider = (env().VITE_MAP_PROVIDER || 'offline').toLowerCase() as BasemapProvider;
	const key = env().VITE_MAP_KEY;
	if (provider === 'amap' && key) {
		// 仅在用户显式提供合法高德 Web 端 Key（并按其服务条款使用）时启用
		return {
			provider: 'amap',
			label: '高德地图',
			gcj02: true,
			hasTiles: true,
			style: amapStyle(key),
			attribution: '© 高德地图'
		};
	}
	// 默认离线内联底图（AGENTS/ADR-004：不默认调用未授权第三方瓦片）。
	// 显式 amap 但缺 Key 时同样回退离线，不发起未授权请求。
	return offlineBasemap();
}

// ---- WGS84 -> GCJ-02（显示层坐标转换；标准实现，误差约米级）----
const A = 6378245.0;
// eslint-disable-next-line no-loss-of-precision -- WGS84→GCJ-02 标准系数（取 double 可表达精度）
const EE = 0.00669342162296594323;

function outOfChina(lng: number, lat: number): boolean {
	return lng < 72.004 || lng > 137.8347 || lat < 0.8293 || lat > 55.8271;
}
function transformLat(x: number, y: number): number {
	let ret = -100.0 + 2.0 * x + 3.0 * y + 0.2 * y * y + 0.1 * x * y + 0.2 * Math.sqrt(Math.abs(x));
	ret += ((20.0 * Math.sin(6.0 * x * Math.PI) + 20.0 * Math.sin(2.0 * x * Math.PI)) * 2.0) / 3.0;
	ret += ((20.0 * Math.sin(y * Math.PI) + 40.0 * Math.sin((y / 3.0) * Math.PI)) * 2.0) / 3.0;
	ret += ((160.0 * Math.sin((y / 12.0) * Math.PI) + 320 * Math.sin((y * Math.PI) / 30.0)) * 2.0) / 3.0;
	return ret;
}
function transformLng(x: number, y: number): number {
	let ret = 300.0 + x + 2.0 * y + 0.1 * x * x + 0.1 * x * y + 0.1 * Math.sqrt(Math.abs(x));
	ret += ((20.0 * Math.sin(6.0 * x * Math.PI) + 20.0 * Math.sin(2.0 * x * Math.PI)) * 2.0) / 3.0;
	ret += ((20.0 * Math.sin(x * Math.PI) + 40.0 * Math.sin((x / 3.0) * Math.PI)) * 2.0) / 3.0;
	ret += ((150.0 * Math.sin((x / 12.0) * Math.PI) + 300.0 * Math.sin((x / 30.0) * Math.PI)) * 2.0) / 3.0;
	return ret;
}
/** 返回 GCJ-02 [lng, lat]；中国境外坐标原样返回。 */
export function wgs84ToGcj02(lng: number, lat: number): [number, number] {
	if (outOfChina(lng, lat)) return [lng, lat];
	let dLat = transformLat(lng - 105.0, lat - 35.0);
	let dLng = transformLng(lng - 105.0, lat - 35.0);
	const radLat = (lat / 180.0) * Math.PI;
	let magic = Math.sin(radLat);
	magic = 1 - EE * magic * magic;
	const sqrtMagic = Math.sqrt(magic);
	dLat = (dLat * 180.0) / (((A * (1 - EE)) / (magic * sqrtMagic)) * Math.PI);
	dLng = (dLng * 180.0) / ((A / sqrtMagic) * Math.cos(radLat) * Math.PI);
	return [lng + dLng, lat + dLat];
}
