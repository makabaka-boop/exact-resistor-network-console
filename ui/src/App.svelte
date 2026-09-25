<script>
  const EXAMPLES = {
    series: {
      nodes: ['A', 'B', 'C'],
      edges: [
        { id: 'R1', from: 'A', to: 'B', milliohms: 1000 },
        { id: 'R2', from: 'B', to: 'C', milliohms: 2000 }
      ],
      source: 'A',
      sink: 'C'
    },
    parallel: {
      nodes: ['A', 'B'],
      edges: [
        { id: 'R1', from: 'A', to: 'B', milliohms: 1000 },
        { id: 'R2', from: 'A', to: 'B', milliohms: 2000 }
      ],
      source: 'A',
      sink: 'B'
    },
    bridge: {
      nodes: ['A', 'B', 'C', 'D'],
      edges: [
        { id: 'R1', from: 'A', to: 'B', milliohms: 1000 },
        { id: 'R2', from: 'A', to: 'C', milliohms: 1000 },
        { id: 'R3', from: 'B', to: 'D', milliohms: 1000 },
        { id: 'R4', from: 'C', to: 'D', milliohms: 1000 },
        { id: 'R5', from: 'B', to: 'C', milliohms: 1000 }
      ],
      source: 'A',
      sink: 'D'
    }
  };

  let input = JSON.stringify(EXAMPLES.bridge, null, 2);
  let result = null;
  let error = '';
  let loading = false;

  function loadExample(name) {
    input = JSON.stringify(EXAMPLES[name], null, 2);
    result = null;
    error = '';
  }

  async function solve() {
    loading = true;
    error = '';
    result = null;
    let payload;
    try {
      payload = JSON.parse(input);
    } catch (e) {
      error = 'Input is not valid JSON: ' + e.message;
      loading = false;
      return;
    }
    try {
      const res = await fetch('/api/solve', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      });
      const data = await res.json();
      if (data.status === 'ERROR') {
        error = data.message || 'input rejected';
      } else {
        result = data;
      }
    } catch (e) {
      error = 'Request failed: ' + e.message;
    } finally {
      loading = false;
    }
  }

  // ---- everything below renders ONLY the solver response ----

  $: ok = result && result.status === 'OK';
  $: potentials = ok ? result.potentials : [];
  $: currents = ok ? result.currents : [];
  $: positions = layout(potentials.map((p) => p.node));
  $: arcs = buildArcs(currents, positions);

  function layout(nodes) {
    const cx = 330;
    const cy = 210;
    const r = 160;
    const pos = {};
    nodes.forEach((n, i) => {
      const angle = (2 * Math.PI * i) / nodes.length - Math.PI / 2;
      pos[n] = { x: cx + r * Math.cos(angle), y: cy + r * Math.sin(angle) };
    });
    return pos;
  }

  // Parallel edges between the same pair fan out onto separate arcs.
  function buildArcs(currents, positions) {
    const groups = new Map();
    for (const c of currents) {
      const key = [c.from, c.to].sort().join('|');
      if (!groups.has(key)) groups.set(key, []);
      groups.get(key).push(c);
    }
    const arcs = [];
    for (const group of groups.values()) {
      group.forEach((c, i) => {
        const offset = (i - (group.length - 1) / 2) * 56;
        arcs.push({ ...c, ...arcPath(positions[c.from], positions[c.to], offset) });
      });
    }
    return arcs;
  }

  function arcPath(a, b, offset) {
    const mx = (a.x + b.x) / 2;
    const my = (a.y + b.y) / 2;
    const dx = b.x - a.x;
    const dy = b.y - a.y;
    const len = Math.hypot(dx, dy) || 1;
    const nx = -dy / len;
    const ny = dx / len;
    const qx = mx + nx * offset;
    const qy = my + ny * offset;
    return { d: `M ${a.x} ${a.y} Q ${qx} ${qy} ${b.x} ${b.y}`, lx: qx, ly: qy };
  }
</script>

<main>
  <h1>Resistor Network Solver</h1>
  <p class="hint">
    Exact rational arithmetic — 1 A is injected at the source and extracted at the sink,
    with the sink held at 0 V. Values are reduced fractions (mΩ, mV, A).
  </p>

  <section class="editor">
    <div class="examples">
      <span>Examples:</span>
      <button on:click={() => loadExample('series')}>Series</button>
      <button on:click={() => loadExample('parallel')}>Parallel</button>
      <button on:click={() => loadExample('bridge')}>Bridge</button>
    </div>
    <textarea bind:value={input} rows="16" spellcheck="false" data-testid="input"></textarea>
    <button class="solve" on:click={solve} disabled={loading}>
      {loading ? 'Solving…' : 'Solve'}
    </button>
  </section>

  {#if error}
    <div class="banner error" data-testid="error">{error}</div>
  {/if}

  {#if result && result.status === 'OPEN'}
    <div class="banner open" data-testid="open">
      <strong>OPEN</strong> — the endpoints are not connected. Connected components:
      <ul>
        {#each result.components as comp}
          <li>{comp.join(', ')}</li>
        {/each}
      </ul>
    </div>
  {/if}

  {#if ok}
    <section class="results">
      <h2>
        Equivalent resistance:
        <span data-testid="equivalent">{result.equivalentMilliohms} mΩ</span>
      </h2>

      <svg viewBox="0 0 660 420" data-testid="graph" role="img">
        <defs>
          <marker
            id="arrow"
            viewBox="0 0 10 10"
            refX="9"
            refY="5"
            markerWidth="7"
            markerHeight="7"
            orient="auto-start-reverse"
          >
            <path d="M 0 0 L 10 5 L 0 10 z" fill="#7a869a" />
          </marker>
        </defs>
        {#each arcs as arc (arc.id)}
          <path d={arc.d} fill="none" stroke="#7a869a" stroke-width="1.5" marker-end="url(#arrow)" />
          <text x={arc.lx} y={arc.ly} class="edge-label">{arc.id} · {arc.amperes} A</text>
        {/each}
        {#each potentials as p (p.node)}
          {@const pos = positions[p.node]}
          <circle cx={pos.x} cy={pos.y} r="18" class="node" />
          <text x={pos.x} y={pos.y + 5} class="node-label">{p.node}</text>
          <text x={pos.x} y={pos.y - 28} class="pot-label">{p.millivolts} mV</text>
        {/each}
      </svg>

      <div class="tables">
        <table data-testid="potentials">
          <thead>
            <tr><th>Node</th><th>Potential</th></tr>
          </thead>
          <tbody>
            {#each potentials as p (p.node)}
              <tr>
                <td>{p.node}</td>
                <td data-testid={`potential-${p.node}`}>{p.millivolts} mV</td>
              </tr>
            {/each}
          </tbody>
        </table>

        <table data-testid="currents">
          <thead>
            <tr><th>Edge</th><th>From</th><th>To</th><th>Current</th></tr>
          </thead>
          <tbody>
            {#each currents as c (c.id)}
              <tr>
                <td>{c.id}</td>
                <td>{c.from}</td>
                <td>{c.to}</td>
                <td data-testid={`current-${c.id}`}>{c.amperes} A</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    </section>
  {/if}
</main>
