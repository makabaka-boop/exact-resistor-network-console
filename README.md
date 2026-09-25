# Resistor Network Solver

Exact-rational solver for resistor networks (including bridges that cannot be
reduced by series/parallel rules), with a Svelte wiring page and a Go solver
service. The `ui` and `circuit` containers talk over real HTTP via Docker
Compose.

## Layout

```
circuit/   Go solver service (stdlib only, exact big.Rat nodal analysis)
ui/        Svelte + Vite page; SVG and tables render only the solver response
docker-compose.yml
```

## Run

```sh
docker compose up --build
# open http://localhost:5173  (ui container; nginx proxies /api → circuit:8080)
```

The `circuit` image build runs `go vet` and `go test` — an image only builds
if the circuit-equation tests pass.

## Test

```sh
# Go: circuit equations, validation, OPEN/ERROR flows
cd circuit && go test ./...

# Browser main flow (balanced bridge) — needs the stack running
cd ui && npm ci && npx playwright install chromium
docker compose up --build -d
npm run e2e            # BASE_URL=http://localhost:5173 is the default
```

For local development without Docker: run the solver (`go run .` in `circuit/`,
listens on `:8080`) and `npm run dev` in `ui/` (Vite proxies `/api` to
`localhost:8080`).

## API

`POST /api/solve`

```json
{
  "nodes": ["A", "B", "C", "D"],
  "edges": [
    { "id": "R1", "from": "A", "to": "B", "milliohms": 1000 }
  ],
  "source": "A",
  "sink": "D"
}
```

Input rules (violations reject the whole request with `400`):

- 2–10 unique node names; 1–18 edges with unique ids. Names are short ASCII
  tokens (`[A-Za-z0-9_-]`, starting alphanumeric).
- `milliohms` is a positive integer; parallel edges allowed, self-loops not.
- `source` ≠ `sink`, both must be known nodes; edges may only reference known
  nodes. Unknown nodes or extra fields (anywhere in the payload) reject the
  whole request.

Responses:

- **Solved** — `200`:
  ```json
  {
    "status": "OK",
    "equivalentMilliohms": "1000",
    "potentials": [{ "node": "A", "millivolts": "1000" }],
    "currents": [{ "id": "R1", "from": "A", "to": "B", "amperes": "1/2" }]
  }
  ```
- **Endpoints not connected** — `200`:
  `{ "status": "OPEN", "components": [["A", "B"], ["C", "D"]] }`
- **Endpoints connected but unrelated isolated components exist** — `400`
  input error: `{ "status": "ERROR", "message": "..." }`

## Method

1 A is injected at the source and extracted at the sink; the sink is held at
0 V. Node equations are solved with exact rational arithmetic
(`math/big.Rat`, Gaussian elimination), so a balanced bridge's mid-branch
current is exactly `0` — never floating-point dust — and every value is
returned as a reduced fraction string (`"2000/3"`, `"1/2"`, `"0"`).

Units: resistances in mΩ, potentials in mV, currents in A (1 A × 1 mΩ = 1 mV,
so numbers stay consistent). `equivalentMilliohms` equals the source
potential at 1 A. Currents are signed in each edge's `from`→`to` direction;
`potentials` are sorted by node, `currents` follow input edge order.
