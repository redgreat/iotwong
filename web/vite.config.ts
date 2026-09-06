import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
	plugins: [tailwindcss(), sveltekit()],
	build: {
		// 品牌图标等小型 SVG 也作为真实文件产出：favicon 需要可下载的 URL，
		// 而不是 data: URI —— 浏览器标签页与 headless 探针才能 200 校验/缓存。
		assetsInlineLimit: 0
	},
	server: {
		// Development proxy: browsers only ever call the same origin; in
		// deployment Nginx performs this proxy and the SPA history fallback
		// (docs/06-deployment.md, delivered in T07).
		// changeOrigin:false keeps the browser Host header, so the backend CSRF
		// check (Origin host == Host) passes for same-origin write requests.
		proxy: {
			'/api': {
				target: process.env.API_ORIGIN || 'http://127.0.0.1:8080',
				changeOrigin: false
			}
		}
	}
});
