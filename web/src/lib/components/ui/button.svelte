<script lang="ts">
	import { cn } from '$lib/utils';
	import type { Snippet } from 'svelte';
import type { HTMLButtonAttributes } from 'svelte/elements';

	type Variant = 'default' | 'destructive' | 'outline' | 'ghost';
	type Size = 'default' | 'sm' | 'icon';

	let {
		variant = 'default',
		size = 'default',
		class: klass = '',
		children,
		...rest

	}: { variant?: Variant; size?: Size; class?: string; children?: Snippet } & HTMLButtonAttributes = $props();

	const variants: Record<Variant, string> = {
		default:
			'bg-primary text-primary-foreground hover:bg-primary/90 shadow-sm inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50',
		destructive:
			'bg-destructive text-destructive-foreground hover:bg-destructive/90 shadow-sm inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50',
		outline:
			'border border-border bg-background hover:bg-accent hover:text-accent-foreground inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50',
		ghost:
			'hover:bg-accent hover:text-accent-foreground inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50'
	};
	const sizes: Record<Size, string> = {
		default: 'h-9 px-4 py-2 text-sm',
		sm: 'h-8 rounded-md px-3 text-xs',
		icon: 'size-9'
	};
</script>

<button class={cn(variants[variant], sizes[size], klass)} {...rest}>
	{@render children?.()}
</button>
