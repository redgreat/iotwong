<script lang="ts">
	import { cn } from '$lib/utils';
	import type { HTMLInputAttributes } from 'svelte/elements';

	let {
		class: klass,
		value = $bindable(''),
		type = 'text',
		...rest
	}: { class?: string; value?: string; type?: string } & HTMLInputAttributes = $props();
</script>

<input
	type={type}
	value={value}
	oninput={(e) => (value = (e.currentTarget as HTMLInputElement).value)}
	class={cn(
		'border-input bg-transparent h-9 w-full rounded-md border px-3 py-1 text-sm shadow-xs transition-colors placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50',
		klass
	)}
	{...rest}
/>
