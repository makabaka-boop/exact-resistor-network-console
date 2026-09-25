<script>
	import { enhance } from '$app/forms';
	import ResultView from '$lib/ResultView.svelte';

	let { form } = $props();

	// 预填一个平衡电桥作为示例
	let nodes = $state('A,B,C,D');
	let source = $state('A');
	let sink = $state('D');
	let rows = $state([
		{ id: 'R1', from: 'A', to: 'B', moh: '1000' },
		{ id: 'R2', from: 'A', to: 'C', moh: '1000' },
		{ id: 'R3', from: 'B', to: 'D', moh: '1000' },
		{ id: 'R4', from: 'C', to: 'D', moh: '1000' },
		{ id: 'R5', from: 'B', to: 'C', moh: '1000' }
	]);

	function addRow() {
		rows.push({ id: `R${rows.length + 1}`, from: '', to: '', moh: '1000' });
	}
	function removeRow(i) {
		rows.splice(i, 1);
	}
</script>

<svelte:head>
	<title>电阻网络求解器</title>
</svelte:head>

<main>
	<h1>电阻网络求解器</h1>
	<p class="hint">
		2–10 个唯一 ASCII 节点、1–18 条正整数毫欧电阻（允许并联，不允许自环）。
		在两个端点间注入 / 取走 1 A，后端以精确有理数求解，分数原样展示。
	</p>

	<form method="POST" use:enhance>
		<section class="panel">
			<label class="nodes-label">
				节点（逗号或空格分隔）
				<input name="nodes" bind:value={nodes} placeholder="A,B,C,D" required />
			</label>
			<div class="terminals">
				<label>注入端点 <input name="source" bind:value={source} required /></label>
				<label>取走端点 <input name="sink" bind:value={sink} required /></label>
			</div>
		</section>

		<section class="panel">
			<table class="edit">
				<thead>
					<tr><th>id</th><th>from</th><th>to</th><th>毫欧</th><th></th></tr>
				</thead>
				<tbody>
					{#each rows as row, i (row)}
						<tr>
							<td><input name="rid" bind:value={row.id} placeholder="R1" /></td>
							<td><input name="rfrom" bind:value={row.from} placeholder="A" /></td>
							<td><input name="rto" bind:value={row.to} placeholder="B" /></td>
							<td><input name="rmoh" bind:value={row.moh} inputmode="numeric" /></td>
							<td>
								<button type="button" class="del" onclick={() => removeRow(i)} aria-label="删除该行">×</button>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
			<button type="button" class="add" onclick={addRow}>+ 添加电阻</button>
		</section>

		<button class="solve" type="submit">求解</button>
	</form>

	{#if form?.error}
		<div class="error" role="alert">
			<strong>输入被拒绝：</strong>{form.error}
			{#if form.components}
				<ul>
					{#each form.components as comp}
						<li>[{comp.join(', ')}]</li>
					{/each}
				</ul>
			{/if}
		</div>
	{/if}

	{#if form?.result}
		<ResultView result={form.result} />
	{/if}
</main>

<style>
	:global(body) {
		font-family: system-ui, 'PingFang SC', 'Microsoft YaHei', sans-serif;
		margin: 0;
		background: #ffffff;
		color: #0f172a;
	}
	main {
		max-width: 760px;
		margin: 0 auto;
		padding: 1.5rem 1rem 3rem;
	}
	h1 {
		font-size: 1.4rem;
		margin-bottom: 0.2rem;
	}
	.hint {
		color: #64748b;
		font-size: 0.85rem;
	}
	.panel {
		border: 1px solid #e2e8f0;
		border-radius: 8px;
		padding: 0.8rem 1rem;
		margin: 0.8rem 0;
	}
	label {
		display: block;
		font-size: 0.85rem;
		color: #334155;
	}
	input {
		font: inherit;
		padding: 0.3rem 0.5rem;
		border: 1px solid #cbd5e1;
		border-radius: 6px;
		margin-top: 0.2rem;
	}
	.nodes-label input {
		width: 100%;
		box-sizing: border-box;
	}
	.terminals {
		display: flex;
		gap: 1rem;
		margin-top: 0.6rem;
	}
	.terminals input {
		width: 6rem;
	}
	table.edit {
		border-collapse: collapse;
	}
	table.edit th {
		font-size: 0.8rem;
		color: #64748b;
		text-align: left;
		padding: 0.2rem 0.4rem;
	}
	table.edit td {
		padding: 0.15rem 0.4rem 0.15rem 0;
	}
	table.edit input {
		width: 5.5rem;
	}
	button {
		font: inherit;
		cursor: pointer;
		border-radius: 6px;
		border: 1px solid #cbd5e1;
		background: #f8fafc;
		padding: 0.3rem 0.8rem;
	}
	button.del {
		padding: 0.1rem 0.5rem;
		color: #b91c1c;
	}
	button.add {
		margin-top: 0.5rem;
	}
	button.solve {
		background: #2563eb;
		border-color: #2563eb;
		color: #fff;
		font-weight: 600;
		padding: 0.45rem 2rem;
	}
	.error {
		border: 1px solid #ef4444;
		background: #fef2f2;
		color: #991b1b;
		border-radius: 8px;
		padding: 0.7rem 1rem;
		margin-top: 1rem;
	}
</style>
