// Static SPA (docs/02-architecture.md ADR-001): the browser renders every
// route client-side; adapter-static serves a single index.html fallback.
export const ssr = false;
export const prerender = false;
