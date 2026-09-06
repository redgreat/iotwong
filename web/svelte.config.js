import adapter from '@sveltejs/adapter-static';

/** @type {import('@sveltejs/kit').Config} */
const config = {
	compilerOptions: {
		// Whole project uses Svelte 5 runes mode.
		runes: true
	},
	kit: {
		// Static SPA: a single index.html fallback serves every route; /api/*
		// is proxied by Nginx (prod) / Vite (dev) to the Go backend.
		adapter: adapter({ fallback: 'index.html' })
	}
};

export default config;
