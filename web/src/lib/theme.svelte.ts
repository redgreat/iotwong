// Shared theme state (light/dark) used by the app shell and header.
// Persisted in localStorage; falls back to the OS preference.
const THEME_KEY = 'iotwong-theme';

export const theme = $state<{ dark: boolean; ready: boolean }>({ dark: false, ready: false });

function apply(): void {
	if (typeof document !== 'undefined') {
		document.documentElement.classList.toggle('dark', theme.dark);
	}
}

export function initTheme(): void {
	let saved: string | null = null;
	try {
		saved = localStorage.getItem(THEME_KEY);
	} catch {
		/* private mode */
	}
	theme.dark = saved ? saved === 'dark' : window.matchMedia('(prefers-color-scheme: dark)').matches;
	theme.ready = true;
	apply();
}

export function toggleTheme(): void {
	theme.dark = !theme.dark;
	apply();
	try {
		localStorage.setItem(THEME_KEY, theme.dark ? 'dark' : 'light');
	} catch {
		/* keep in-memory only */
	}
}
