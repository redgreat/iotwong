<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import Button from '$lib/components/ui/button.svelte';
	import Input from '$lib/components/ui/input.svelte';
	import { apiDevices, apiPatchDeviceName, ApiError, type DeviceItem } from '$lib/api';

	// 设备改名（仅 admin 菜单可见）：R10 本地别名，PATCH /devices/{id} {name}。
	let { onRenamed }: { onRenamed?: () => void } = $props();

	let devices = $state<DeviceItem[] | null>(null);
	let loading = $state(true);
	let listError = $state<string | null>(null);
	let selectedId = $state<string | null>(null);
	let nameDraft = $state('');
	let saving = $state(false);
	let formError = $state<string | null>(null);
	let notice = $state<string | null>(null);
	let noticeTimer: ReturnType<typeof setTimeout> | undefined;

	const selected = $derived(devices?.find((d) => d.id === selectedId) ?? null);
	const nameChanged = $derived(!!selected && nameDraft.trim() !== '' && nameDraft.trim() !== selected.name);

	function showNotice(text: string) {
		notice = text;
		if (noticeTimer) clearTimeout(noticeTimer);
		noticeTimer = setTimeout(() => (notice = null), 4500);
	}
	onDestroy(() => {
		if (noticeTimer) clearTimeout(noticeTimer);
	});

	async function load() {
		loading = true;
		listError = null;
		try {
			const page = await apiDevices({ source: 'local', limit: '200' });
			devices = page.items;
			// 若当前选中设备已不存在则取消选择
			if (selectedId && !devices.some((d) => d.id === selectedId)) {
				selectedId = null;
				nameDraft = '';
			}
		} catch (e) {
			devices = null;
			listError = e instanceof ApiError ? e.message : String(e);
		} finally {
			loading = false;
		}
	}

	onMount(() => void load());

	function pick(d: DeviceItem) {
		selectedId = d.id;
		nameDraft = d.name || d.external_id;
		formError = null;
	}

	async function save() {
		if (!selected) return;
		const name = nameDraft.trim();
		if (name === '' || name === selected.name) return;
		saving = true;
		formError = null;
		try {
			await apiPatchDeviceName(selected.id, name);
			showNotice(`已保存别名「${name}」（本地别名，仅提示成功）`);
			if (devices) devices = devices.map((d) => (d.id === selected.id ? { ...d, name } : d));
			onRenamed?.();
		} catch (e) {
			formError = e instanceof ApiError ? e.message : String(e);
		} finally {
			saving = false;
		}
	}
</script>

<section aria-label="设备改名" class="border-border bg-card flex flex-col gap-3 rounded-lg border p-3 xl:grid xl:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
	<div>
		<h2 class="text-base font-semibold">设备改名</h2>
		<p class="text-muted-foreground mt-0.5 text-xs">
			修改的是平台内本地别名（R10，PATCH /devices/{'{id}'}，仅 name），不会向设备或上游发送任何修改。
		</p>

		{#if notice}
			<p role="status" class="border-success/40 text-success-foreground bg-success/10 mt-3 rounded-md border px-3 py-2 text-xs">{notice}</p>
		{/if}

		<div class="mt-3">
			<p class="text-muted-foreground mb-1 text-xs">选择 local 设备</p>
			{#if loading}
				<p class="text-muted-foreground py-6 text-center text-sm">加载设备中…</p>
			{:else if listError}
				<div class="text-muted-foreground py-4 text-center text-sm">
					<p role="alert" class="text-destructive">{listError}</p>
					<div class="mt-2"><Button size="sm" variant="outline" onclick={() => void load()}>重试</Button></div>
				</div>
			{:else if !devices || devices.length === 0}
				<p class="text-muted-foreground py-6 text-center text-sm">没有可改名的 local 设备（改名仅适用于自建设备）。</p>
			{:else}
				<ul class="border-border max-h-[40vh] divide-y overflow-y-auto rounded-md border">
					{#each devices as d (d.id)}
						<li>
							<button
								type="button"
								onclick={() => pick(d)}
								class="hover:bg-accent/60 w-full px-3 py-2 text-left text-sm"
								class:bg-accent={d.id === selectedId}
							>
								<span class="flex items-center justify-between gap-2">
									<span class="truncate font-medium">{d.name || d.external_id}</span>
									<span class="text-muted-foreground shrink-0 text-xs">{d.external_id}</span>
								</span>
							</button>
						</li>
					{/each}
				</ul>
			{/if}
		</div>
	</div>

	{#if selected}
		<div class="border-border rounded-md border p-3">
			<h3 class="text-sm font-semibold">改名：{selected.name || selected.external_id}</h3>
			<dl class="text-muted-foreground mt-2 grid grid-cols-2 gap-x-3 gap-y-1 text-xs">
				<div><dt class="inline">来源 </dt><dd class="inline">{selected.source}</dd></div>
				<div><dt class="inline">标识 </dt><dd class="inline break-all">{selected.external_id}</dd></div>
				<div class="col-span-2"><dt class="inline">设备 id </dt><dd class="inline break-all">{selected.id}</dd></div>
			</dl>
			<form
				onsubmit={(e) => {
					e.preventDefault();
					void save();
				}}
				class="mt-3 flex flex-col gap-2"
			>
				<label class="flex flex-col gap-1 text-xs">
					<span class="text-muted-foreground">新名称（1-200 字符）</span>
					<Input bind:value={nameDraft} maxlength={200} required placeholder="本地显示名称" />
				</label>
				{#if formError}
					<p role="alert" class="text-destructive text-xs">{formError}</p>
				{/if}
				<div class="flex items-center gap-2">
					<Button
						type="submit"
						disabled={saving || !nameChanged}
						title={!nameChanged ? '名称未变化或为空时不可保存' : undefined}
					>
						{saving ? '保存中…' : '保存'}
					</Button>
					<Button type="button" variant="ghost" size="sm" onclick={() => pick(selected)} disabled={saving}>
						重置
					</Button>
				</div>
			</form>
		</div>
	{:else}
		<div class="text-muted-foreground flex min-h-24 items-center justify-center rounded-md border border-dashed text-xs">
			从左侧选择要改名的 local 设备
		</div>
	{/if}
</section>
