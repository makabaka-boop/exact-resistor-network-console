<script>
	import { fmt } from '$lib/fraction.js';

	// result 为求解服务的完整响应；本组件（SVG 与表格）只读取它，不做任何电路计算。
	let { result } = $props();

	const W = 640;
	const H = 460;
	const CX = W / 2;
	const CY = H / 2;
	const RADIUS = 160;
	const NODE_R = 24;

	// 由响应派生的布局：节点按输入顺序均匀排在圆周上，
	// 并联边按组横向错开成弧线。
	let layout = $derived.by(() => {
		if (result.status !== 'SOLVED') return null;
		const pots = result.potentials;
		const n = pots.length;
		const pos = {};
		pots.forEach((p, i) => {
			const a = -Math.PI / 2 + (2 * Math.PI * i) / n;
			pos[p.node] = { x: CX + RADIUS * Math.cos(a), y: CY + RADIUS * Math.sin(a) };
		});
		const groups = new Map();
		for (const c of result.currents) {
			const key = [c.from, c.to].sort().join('|');
			if (!groups.has(key)) groups.set(key, []);
			groups.get(key).push(c);
		}
		const edges = [];
		for (const list of groups.values()) {
			list.forEach((c, k) => {
				const p1 = pos[c.from];
				const p2 = pos[c.to];
				const dx = p2.x - p1.x;
				const dy = p2.y - p1.y;
				const len = Math.hypot(dx, dy);
				const nx = -dy / len;
				const ny = dx / len;
				const off = (k - (list.length - 1) / 2) * 34;
				const ctrl = { x: (p1.x + p2.x) / 2 + nx * off, y: (p1.y + p2.y) / 2 + ny * off };
				const B = (t) => ({
					x: (1 - t) ** 2 * p1.x + 2 * (1 - t) * t * ctrl.x + t ** 2 * p2.x,
					y: (1 - t) ** 2 * p1.y + 2 * (1 - t) * t * ctrl.y + t ** 2 * p2.y
				});
				const a = B(NODE_R / len);
				const b = B(1 - NODE_R / len);
				const mid = B(0.5);
				const zero = c.current.num === '0';
				const neg = c.current.num.startsWith('-');
				// 二次贝塞尔中点切线方向即弦方向；负电流则箭头反转
				const angle = (Math.atan2(dy, dx) * 180) / Math.PI + (neg ? 180 : 0);
				const side = off >= 0 ? 1 : -1;
				edges.push({
					c,
					a,
					b,
					ctrl,
					mid,
					zero,
					angle,
					label: { x: mid.x + nx * side * 18, y: mid.y + ny * side * 18 }
				});
			});
		}
		return { pos, edges, pots };
	});
</script>

{#if result.status === 'OPEN'}
	<section class="open" data-testid="open-view">
		<h2>开路（OPEN）：两个端点不连通</h2>
		<p>两个端点分属不同连通分量，等效电阻为无穷大。连通分量：</p>
		<ul>
			{#each result.components as comp}
				<li data-testid="component">[{comp.join(', ')}]</li>
			{/each}
		</ul>
	</section>
{:else if layout}
	<section class="solved">
		<h2>
			等效电阻 R<sub>{result.source}{result.sink}</sub> =
			<span data-testid="eq-resistance">{fmt(result.equivalentResistance)} Ω</span>
		</h2>
		<p class="caption">
			在 {result.source} 注入 1 A、{result.sink} 取走 1 A，{result.sink} 为零电位；箭头为电流实际方向，数值为精确分数。
		</p>
		<svg viewBox="0 0 {W} {H}" role="img" aria-label="电路接线图">
			{#each layout.edges as e}
				<path
					d="M {e.a.x} {e.a.y} Q {e.ctrl.x} {e.ctrl.y} {e.b.x} {e.b.y}"
					fill="none"
					stroke="#94a3b8"
					stroke-width="1.6"
				/>
				{#if !e.zero}
					<g transform="translate({e.mid.x} {e.mid.y}) rotate({e.angle})">
						<path d="M 8 0 L -6 -5.5 L -6 5.5 Z" fill="#dc2626" />
					</g>
				{/if}
				<text x={e.label.x} y={e.label.y} text-anchor="middle" class="edge-label">
					{e.c.id} {fmt(e.c.current)} A
				</text>
			{/each}
			{#each layout.pots as p}
				{@const q = layout.pos[p.node]}
				<g>
					<circle class="node" cx={q.x} cy={q.y} r={NODE_R} />
					<text x={q.x} y={q.y + 5} text-anchor="middle" class="node-name">{p.node}</text>
					<text x={q.x} y={q.y + NODE_R + 17} text-anchor="middle" class="node-v">
						{fmt(p.potential)} V
					</text>
				</g>
			{/each}
		</svg>

		<div class="tables">
			<table>
				<caption>节点电位（{result.sink} = 0 V）</caption>
				<thead>
					<tr><th>节点</th><th>电位</th></tr>
				</thead>
				<tbody>
					{#each layout.pots as p}
						<tr>
							<td>{p.node}</td>
							<td data-testid="potential-{p.node}">{fmt(p.potential)} V</td>
						</tr>
					{/each}
				</tbody>
			</table>
			<table>
				<caption>支路电流（正值表示 from → to）</caption>
				<thead>
					<tr><th>电阻</th><th>方向</th><th>电流</th></tr>
				</thead>
				<tbody>
					{#each result.currents as c}
						<tr>
							<td>{c.id}</td>
							<td>{c.from} → {c.to}</td>
							<td data-testid="current-{c.id}">{fmt(c.current)} A</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	</section>
{/if}

<style>
	.open {
		border: 1px solid #f59e0b;
		background: #fffbeb;
		border-radius: 8px;
		padding: 0.8rem 1.2rem;
	}
	h2 {
		font-size: 1.15rem;
	}
	.caption {
		color: #64748b;
		font-size: 0.85rem;
		margin: 0.2rem 0 0.6rem;
	}
	svg {
		width: 100%;
		max-width: 640px;
		background: #f8fafc;
		border: 1px solid #e2e8f0;
		border-radius: 8px;
	}
	:global(.node) {
		fill: #eff6ff;
		stroke: #2563eb;
		stroke-width: 2;
	}
	.node-name {
		font-weight: 700;
		font-size: 15px;
		fill: #1e3a8a;
	}
	.node-v {
		font-size: 12px;
		fill: #475569;
	}
	.edge-label {
		font-size: 12px;
		fill: #b91c1c;
	}
	.tables {
		display: flex;
		gap: 1.5rem;
		flex-wrap: wrap;
		margin-top: 1rem;
	}
	table {
		border-collapse: collapse;
	}
	caption {
		font-weight: 600;
		text-align: left;
		margin-bottom: 0.3rem;
	}
	th,
	td {
		border: 1px solid #cbd5e1;
		padding: 0.3rem 0.8rem;
		text-align: left;
	}
	th {
		background: #f1f5f9;
	}
</style>
