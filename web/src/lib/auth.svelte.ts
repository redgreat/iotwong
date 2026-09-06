import { apiLogin, apiLogout, apiMe, ApiError, setUnauthorizedHandler } from './api';
import type { Me } from './api';

type Status = 'checking' | 'anon' | 'signed';

export const auth = $state<{ status: Status; me: Me | null; error: string | null }>({
	status: 'checking',
	me: null,
	error: null
});

// 任何已登录页面的 API 返回 401（会话过期/被注销）时，立即回到登录门，
// 不让工作台停留在“请求失败”的错误态（验收：401 自动登出回登录页）。
function expireSession(): void {
	auth.status = 'anon';
	auth.me = null;
	auth.error = null;
}
setUnauthorizedHandler(expireSession);

export async function boot(): Promise<void> {
	auth.status = 'checking';
	auth.error = null;
	try {
		auth.me = await apiMe();
		auth.status = 'signed';
	} catch (e) {
		if (e instanceof ApiError && (e.code === 'unauthorized' || e.status === 401)) {
			auth.status = 'anon';
		} else {
			auth.status = 'anon';
			auth.error = e instanceof ApiError ? e.message : String(e);
		}
	}
}

export async function login(loginName: string, password: string): Promise<void> {
	await apiLogin(loginName, password);
	auth.me = await apiMe();
	auth.status = 'signed';
	auth.error = null;
}

export async function logout(): Promise<void> {
	try {
		await apiLogout();
	} finally {
		auth.status = 'anon';
		auth.me = null;
	}
}
