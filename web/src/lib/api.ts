/**
 * Typed client for the self-hosted HTTP v1 API (docs/04-contracts.md).
 * All calls are same-origin; the deployment Nginx / dev Vite proxies /api
 * to the Go backend. Sessions are HttpOnly cookies.
 */

export interface ApiEnvelope<T> {
	data?: T;
	error?: { code: string; message: string; details?: unknown };
	request_id: string;
}

export type HealthLive = { status: 'ok' };
export type ReadyComponent = { name: string; status: 'ok' | 'error'; error?: string };
export type HealthReady = { ready: boolean; components: ReadyComponent[] };

export interface Tenant { tenant_id: string; tenant_name: string; role: string }
export interface Me {
	user: { id: string; login: string };
	tenants: Tenant[];
	timezone: string;
}
export interface GeoPoint { longitude: number; latitude: number; crs: string }
export interface DeviceItem {
	id: string;
	source: string;
	external_id: string;
	name: string;
	project_id: string;
	communication_status: 'online' | 'offline' | 'unknown';
	last_seen_at: string | null;
	last_position_at: string | null;
	position: GeoPoint | null;
	location_source: string | null;
	accuracy_m: number | null;
	battery_pct: number | null;
	speed_mps: number | null;
	is_demo: boolean;
	sync_status: string | null;
}
export interface DeviceList { items: DeviceItem[]; page: { next_cursor: string | null; total: null } }

export interface TrackPoint { recorded_at: string; coordinates: [number, number] }
export interface TrackSegment { points: TrackPoint[] }
export interface Track {
	segments: TrackSegment[];
	raw_count: number;
	returned_count: number;
	simplified: boolean;
}
export interface PositionItem {
	recorded_at: string;
	longitude: number;
	latitude: number;
	speed_mps: number | null;
	heading_deg: number | null;
	accuracy_m: number | null;
}
export interface PositionList { items: PositionItem[]; page: { next_cursor: string | null; total: null } }

export class ApiError extends Error {
	code: string;
	requestId: string;
	status: number;
	constructor(status: number, code: string, message: string, requestId: string) {
		super(message);
		this.name = 'ApiError';
		this.status = status;
		this.code = code;
		this.requestId = requestId;
	}
}

// Single 401 hook: the auth module registers it so any expired session
// (any view) falls back to the login gate instead of lingering on an error.
let unauthorizedHandler: (() => void) | null = null;
export function setUnauthorizedHandler(fn: (() => void) | null): void {
	unauthorizedHandler = fn;
}

// 请求级超时（毫秒）：后端偶发卡顿/断连时，前端必须从“一直 loading”收敛到
// 显式错误 + 重试，而不是无限期等待（设备地图/设备管理反馈“一直加载中”的根因）。
const REQUEST_TIMEOUT_MS = 15000;

async function request<T>(path: string, init?: RequestInit, timeoutMs = REQUEST_TIMEOUT_MS): Promise<T> {
	const ctrl = new AbortController();
	const timer = setTimeout(() => ctrl.abort(), timeoutMs);
	let res: Response;
	try {
		res = await fetch(path, { headers: { accept: 'application/json' }, signal: ctrl.signal, ...init });
	} catch {
		if (ctrl.signal.aborted) {
			throw new ApiError(0, 'timeout', '请求超时，请重试', '');
		}
		throw new ApiError(0, 'network_error', '无法连接后端服务', '');
	} finally {
		clearTimeout(timer);
	}
	let body: ApiEnvelope<T> | null = null;
	try {
		body = (await res.json()) as ApiEnvelope<T>;
	} catch {
		body = null;
	}
	if (!body) {
		throw new ApiError(res.status, 'invalid_response', '后端返回了无法解析的响应', '');
	}
	if (!res.ok || body.error) {
		if (res.status === 401) unauthorizedHandler?.();
		throw new ApiError(
			res.status,
			body.error?.code ?? 'unknown',
			body.error?.message ?? '请求失败',
			body.request_id
		);
	}
	return body.data as T;
}

const json = (body: unknown): RequestInit => ({
	method: 'POST',
	headers: { 'content-type': 'application/json' },
	body: JSON.stringify(body)
});

export function getHealthLive(): Promise<HealthLive> {
	return request<HealthLive>('/api/v1/health/live');
}
export function getHealthReady(): Promise<HealthReady> {
	return request<HealthReady>('/api/v1/health/ready');
}

export function apiLogin(login: string, password: string): Promise<{ login: string }> {
	return request('/api/v1/auth/login', json({ login, password }));
}
export function apiLogout(): Promise<{ ok: boolean }> {
	return request('/api/v1/auth/logout', { method: 'POST' });
}
export function apiMe(): Promise<Me> {
	return request<Me>('/api/v1/auth/me');
}

export function apiDevices(params: Record<string, string> = {}): Promise<DeviceList> {
	const q = new URLSearchParams(params).toString();
	return request<DeviceList>(`/api/v1/devices${q ? '?' + q : ''}`);
}

export function apiTrack(deviceId: string, from: string, to: string, maxPoints = 5000): Promise<Track> {
	return request<Track>(
		`/api/v1/devices/${encodeURIComponent(deviceId)}/track?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}&max_points=${maxPoints}`
	);
}

export function apiPositions(
	deviceId: string,
	from: string,
	to: string,
	limit = 200
): Promise<PositionList> {
	return request<PositionList>(
		`/api/v1/devices/${encodeURIComponent(deviceId)}/positions?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}&limit=${limit}`
	);
}

// ---- P1 fences & alarms (docs/04-contracts.md; write requires admin) ----

export interface PageRef {
	next_cursor: string | null;
	total: null;
}

export interface AlarmItem {
	id: string;
	fence_id: string;
	fence_name: string;
	device_id: string;
	/** enter = 设备进入围栏；exit = 设备离开围栏 */
	transition: 'enter' | 'exit';
	occurred_at: string;
	acked: boolean;
}

export interface Fence {
	id: string;
	name: string;
	kind: 'circle' | 'polygon';
	enabled: boolean;
	center_longitude?: number | null;
	center_latitude?: number | null;
	radius_m?: number | null;
	/** 闭合 [lng,lat] 环（polygon 围栏） */
	polygon?: [number, number][] | null;
	device_ids: string[];
}

export interface FenceInput {
	name: string;
	kind: 'circle';
	enabled: boolean;
	center_longitude: number;
	center_latitude: number;
	radius_m: number;
	device_ids: string[];
}

export function apiAlarms(limit = 200): Promise<{ items: AlarmItem[] | null; page: PageRef }> {
	return request<{ items: AlarmItem[] | null; page: PageRef }>(`/api/v1/alarms?limit=${limit}`);
}
export function apiAckAlarm(alarmId: string): Promise<{ ok: boolean }> {
	return request<{ ok: boolean }>(`/api/v1/alarms/${encodeURIComponent(alarmId)}/ack`, { method: 'POST' });
}
export function apiFences(): Promise<{ items: Fence[] | null; page: PageRef }> {
	return request<{ items: Fence[] | null; page: PageRef }>('/api/v1/fences');
}
export function apiCreateFence(input: FenceInput): Promise<{ id: string }> {
	return request<{ id: string }>('/api/v1/fences', json(input));
}
export function apiDeleteFence(fenceId: string): Promise<{ ok: boolean }> {
	return request<{ ok: boolean }>(`/api/v1/fences/${encodeURIComponent(fenceId)}`, { method: 'DELETE' });
}
/** R10 本地别名改名（admin；仅 name，200 字符内）。 */
export function apiPatchDeviceName(deviceId: string, name: string): Promise<{ ok: boolean }> {
	return request<{ ok: boolean }>(`/api/v1/devices/${encodeURIComponent(deviceId)}`, {
		method: 'PATCH',
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify({ name })
	});
}

// ---- 用户管理（admin；docs/04-contracts.md）----

export interface UserItem {
	id: string;
	login: string;
	tenant_name: string;
	role: string;
	created_at: string;
}

export function apiUsers(): Promise<{ items: UserItem[] | null; page: PageRef }> {
	return request<{ items: UserItem[] | null; page: PageRef }>('/api/v1/users');
}
export function apiCreateUser(input: {
	login: string;
	password: string;
	role: string;
}): Promise<{ id: string }> {
	return request<{ id: string }>('/api/v1/users', json(input));
}
export function apiResetUserPassword(userId: string, password: string): Promise<{ ok: boolean }> {
	return request<{ ok: boolean }>(
		`/api/v1/users/${encodeURIComponent(userId)}/reset-password`,
		json({ password })
	);
}
