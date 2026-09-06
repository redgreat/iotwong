<script lang="ts">
	import '../app.css';
	import brandIcon from '$lib/assets/brand-icon.svg';
	import { onMount } from 'svelte';
	import { initTheme } from '$lib/theme.svelte';

	let { children } = $props();

	onMount(() => initTheme());
</script>

<svelte:head>
	<link rel="icon" type="image/svg+xml" href={brandIcon} />
</svelte:head>

<div class="bg-background text-foreground min-h-dvh w-full antialiased">
	{@render children()}
</div>
