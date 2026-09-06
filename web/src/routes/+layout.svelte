<script lang="ts">
	import '../app.css';
	import favicon from '$lib/assets/favicon.svg';
	import { onMount } from 'svelte';
	import { initTheme } from '$lib/theme.svelte';

	let { children } = $props();

	onMount(() => initTheme());
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

<div class="bg-background text-foreground min-h-dvh w-full antialiased">
	{@render children()}
</div>
