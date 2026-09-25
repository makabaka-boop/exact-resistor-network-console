# 电阻网络求解器

任意接线（含桥式等无法串并联化简的网络）的直流电阻网络求解器：
Svelte 接线页 + Go 求解服务，Compose 一键联调。
所有结果以**精确有理数（约分分数）**返回与展示——浮点近似会把很小但非零的
支路电流显示成 0，这里分子分母都是整数字符串，绝无舍入。

## 组成

| 目录 | 说明 |
| --- | --- |
| `circuit/` | Go 求解服务（仅标准库），`POST /api/solve`，精确有理数解节点方程 |
| `ui/` | SvelteKit 接线页：表单录入、SVG 接线图、电位/电流表格；服务端 action 经 HTTP 调用 `circuit` |
| `compose.yml` | `ui` + `circuit` 两容器联调（ui 通过 `CIRCUIT_URL=http://circuit:8080` 发起真实 HTTP 请求） |

## 快速开始

```bash
docker compose up --build
# 打开 http://localhost:3000 （页面预填了一个平衡电桥示例）
```

若对外地址/端口不同，请修改 `compose.yml` 中 ui 的 `ORIGIN`
（adapter-node 用它校验表单 Origin，做 CSRF 防护）。

本地开发（不用 Docker）：

```bash
cd circuit && go run .                 # 求解服务 :8080
cd ui && npm install && npm run dev    # 页面 :5173，CIRCUIT_URL 默认 http://localhost:8080
```

## 输入规则

- 2–10 个唯一节点，节点名为 1–16 个可见 ASCII 字符
- 1–18 条电阻：唯一 id、正整数毫欧；**允许并联边，不允许自环**
- 两个不同的端点（注入端 / 取走端）
- 引用未知节点、出现多余字段、或任何一条不合法 → **整份拒绝**（HTTP 400）
- 端点不连通 → 响应 `OPEN` 与全部连通分量
- 端点连通但存在完全无关的孤立分量 → 输入错误（HTTP 400，附连通分量）

## API

`POST /api/solve`，Content-Type: application/json

```json
{
  "nodes": ["A", "B", "C", "D"],
  "resistors": [
    {"id": "R1", "from": "A", "to": "B", "milliohms": 1000},
    {"id": "R2", "from": "A", "to": "C", "milliohms": 1000},
    {"id": "R3", "from": "B", "to": "D", "milliohms": 1000},
    {"id": "R4", "from": "C", "to": "D", "milliohms": 2000},
    {"id": "R5", "from": "B", "to": "C", "milliohms": 1000}
  ],
  "source": "A",
  "sink": "D"
}
```

求解时以 `sink` 为零电位，在 `source` 注入 1 A、`sink` 取走 1 A，
用 `math/big.Rat` 精确解节点电导方程。响应（200）：

```json
{
  "status": "SOLVED",
  "source": "A", "sink": "D",
  "equivalentResistance": {"num": "13", "den": "11"},
  "potentials": [{"node": "A", "potential": {"num": "13", "den": "11"}}, "..."],
  "currents": [{"id": "R5", "from": "B", "to": "C", "current": {"num": "-1", "den": "11"}}, "..."]
}
```

单位：等效电阻欧姆、电位伏特、电流安培；分数均已约分、分母为正，
电流正值表示 from → to。端点不连通时（200）：

```json
{"status": "OPEN", "components": [["A", "B"], ["C", "D"]]}
```

输入被拒绝时（400）：`{"error": "...", "components": [[...]]?}`

页面的 SVG 与表格只读取上述响应，不在浏览器内做任何电路计算。

## 测试

```bash
cd circuit && go test ./...     # 精确值（串/并/桥）+ 校验拒绝 + OPEN/孤立分量
                                # + 50 组随机网络的 KCL/欧姆定律/等效电阻核对 + HTTP 层

cd ui && npx playwright install chromium
docker compose up -d --build
cd ui && npx playwright test    # 一条浏览器主流程：桥式网络提交并核对精确分数
```

Playwright 默认用 `docker compose up --build` 起栈（`ui/playwright.config.js` 的
`webServer`）；若服务已在运行，会复用 `http://localhost:3000`，也可用
`PLAYWRIGHT_BASE_URL` 指定其他地址。
