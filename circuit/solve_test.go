package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------- 辅助 ----------

func rat(num, den string) *big.Rat {
	r, ok := new(big.Rat).SetString(num + "/" + den)
	if !ok {
		panic("bad fraction " + num + "/" + den)
	}
	return r
}

func frac(num, den string) fraction { return fraction{Num: num, Den: den} }

func mustSolve(t *testing.T, req solveRequest) *solvedResponse {
	t.Helper()
	res, aerr := analyze(&req)
	if aerr != nil {
		t.Fatalf("analyze 返回错误: %v", aerr)
	}
	s, ok := res.(*solvedResponse)
	if !ok {
		t.Fatalf("期望 SOLVED，实际为 %+v", res)
	}
	return s
}

func potentialOf(t *testing.T, s *solvedResponse, node string) fraction {
	t.Helper()
	for _, p := range s.Potentials {
		if p.Node == node {
			return p.Potential
		}
	}
	t.Fatalf("节点 %s 不在电位结果中", node)
	return fraction{}
}

func currentOf(t *testing.T, s *solvedResponse, id string) fraction {
	t.Helper()
	for _, c := range s.Currents {
		if c.ID == id {
			return c.Current
		}
	}
	t.Fatalf("电阻 %s 不在电流结果中", id)
	return fraction{}
}

// ---------- 精确值测试：串联 / 并联 / 桥式 ----------

// 串联：A -1k- B -2k- C，A→C 注入 1 A。
// R_eq = 3 Ω；V = 3, 2, 0；各支路 1 A。
func TestSeriesCircuit(t *testing.T) {
	s := mustSolve(t, solveRequest{
		Nodes: []string{"A", "B", "C"},
		Resistors: []resistorIn{
			{ID: "R1", From: "A", To: "B", Milliohms: 1000},
			{ID: "R2", From: "B", To: "C", Milliohms: 2000},
		},
		Source: "A", Sink: "C",
	})
	if got := s.EquivalentResistance; got != frac("3", "1") {
		t.Errorf("等效电阻 = %v，期望 3/1", got)
	}
	for node, want := range map[string]fraction{"A": frac("3", "1"), "B": frac("2", "1"), "C": frac("0", "1")} {
		if got := potentialOf(t, s, node); got != want {
			t.Errorf("V(%s) = %v，期望 %v", node, got, want)
		}
	}
	for _, id := range []string{"R1", "R2"} {
		if got := currentOf(t, s, id); got != frac("1", "1") {
			t.Errorf("I(%s) = %v，期望 1/1", id, got)
		}
	}
}

// 并联：A ⇒ B，1 Ω 与 2 Ω 两条并联边。
// R_eq = 2/3 Ω；I1 = 2/3 A，I2 = 1/3 A。
func TestParallelCircuit(t *testing.T) {
	s := mustSolve(t, solveRequest{
		Nodes: []string{"A", "B"},
		Resistors: []resistorIn{
			{ID: "R1", From: "A", To: "B", Milliohms: 1000},
			{ID: "R2", From: "A", To: "B", Milliohms: 2000},
		},
		Source: "A", Sink: "B",
	})
	if got := s.EquivalentResistance; got != frac("2", "3") {
		t.Errorf("等效电阻 = %v，期望 2/3", got)
	}
	if got := currentOf(t, s, "R1"); got != frac("2", "3") {
		t.Errorf("I(R1) = %v，期望 2/3", got)
	}
	if got := currentOf(t, s, "R2"); got != frac("1", "3") {
		t.Errorf("I(R2) = %v，期望 1/3", got)
	}
}

// 平衡电桥：五条 1 Ω 电阻，R5 为桥臂。
// R_eq = 1 Ω；V_B = V_C = 1/2 V；桥臂电流精确为 0（浮点易误判的关键情形）。
func TestBalancedBridge(t *testing.T) {
	s := mustSolve(t, solveRequest{
		Nodes: []string{"A", "B", "C", "D"},
		Resistors: []resistorIn{
			{ID: "R1", From: "A", To: "B", Milliohms: 1000},
			{ID: "R2", From: "A", To: "C", Milliohms: 1000},
			{ID: "R3", From: "B", To: "D", Milliohms: 1000},
			{ID: "R4", From: "C", To: "D", Milliohms: 1000},
			{ID: "R5", From: "B", To: "C", Milliohms: 1000},
		},
		Source: "A", Sink: "D",
	})
	if got := s.EquivalentResistance; got != frac("1", "1") {
		t.Errorf("等效电阻 = %v，期望 1/1", got)
	}
	if got := potentialOf(t, s, "B"); got != frac("1", "2") {
		t.Errorf("V(B) = %v，期望 1/2", got)
	}
	if got := potentialOf(t, s, "C"); got != frac("1", "2") {
		t.Errorf("V(C) = %v，期望 1/2", got)
	}
	if got := currentOf(t, s, "R5"); got != frac("0", "1") {
		t.Errorf("桥臂电流 I(R5) = %v，期望精确 0/1", got)
	}
}

// 非平衡电桥：R4 = 2 Ω，其余 1 Ω。手算结果：
// V_A = 13/11, V_B = 7/11, V_C = 8/11；R_eq = 13/11 Ω；
// I = 6/11, 5/11, 7/11, 4/11, -1/11（R5 实际流向 C→B）。
func TestUnbalancedBridge(t *testing.T) {
	s := mustSolve(t, solveRequest{
		Nodes: []string{"A", "B", "C", "D"},
		Resistors: []resistorIn{
			{ID: "R1", From: "A", To: "B", Milliohms: 1000},
			{ID: "R2", From: "A", To: "C", Milliohms: 1000},
			{ID: "R3", From: "B", To: "D", Milliohms: 1000},
			{ID: "R4", From: "C", To: "D", Milliohms: 2000},
			{ID: "R5", From: "B", To: "C", Milliohms: 1000},
		},
		Source: "A", Sink: "D",
	})
	if got := s.EquivalentResistance; got != frac("13", "11") {
		t.Errorf("等效电阻 = %v，期望 13/11", got)
	}
	for node, want := range map[string]fraction{
		"A": frac("13", "11"), "B": frac("7", "11"), "C": frac("8", "11"), "D": frac("0", "1"),
	} {
		if got := potentialOf(t, s, node); got != want {
			t.Errorf("V(%s) = %v，期望 %v", node, got, want)
		}
	}
	for id, want := range map[string]fraction{
		"R1": frac("6", "11"), "R2": frac("5", "11"), "R3": frac("7", "11"),
		"R4": frac("4", "11"), "R5": frac("-1", "11"),
	} {
		if got := currentOf(t, s, id); got != want {
			t.Errorf("I(%s) = %v，期望 %v", id, got, want)
		}
	}
}

// ---------- OPEN 与孤立分量 ----------

func TestOpenCircuit(t *testing.T) {
	res, aerr := analyze(&solveRequest{
		Nodes: []string{"A", "B", "C", "D"},
		Resistors: []resistorIn{
			{ID: "R1", From: "A", To: "B", Milliohms: 1000},
			{ID: "R2", From: "C", To: "D", Milliohms: 1000},
		},
		Source: "A", Sink: "D",
	})
	if aerr != nil {
		t.Fatalf("OPEN 情形不应报错: %v", aerr)
	}
	o, ok := res.(openResponse)
	if !ok {
		t.Fatalf("期望 OPEN 响应，实际 %+v", res)
	}
	if o.Status != "OPEN" {
		t.Errorf("status = %q，期望 OPEN", o.Status)
	}
	want := [][]string{{"A", "B"}, {"C", "D"}}
	if fmt.Sprint(o.Components) != fmt.Sprint(want) {
		t.Errorf("连通分量 = %v，期望 %v", o.Components, want)
	}
}

// 端点连通，但节点 C 自成一个完全无关的孤立分量 → 输入错误。
func TestIsolatedComponentRejected(t *testing.T) {
	_, aerr := analyze(&solveRequest{
		Nodes: []string{"A", "B", "C"},
		Resistors: []resistorIn{
			{ID: "R1", From: "A", To: "B", Milliohms: 1000},
		},
		Source: "A", Sink: "B",
	})
	if aerr == nil {
		t.Fatal("存在孤立分量时应返回输入错误")
	}
	if !strings.Contains(aerr.msg, "孤立分量") {
		t.Errorf("错误信息应提及孤立分量: %q", aerr.msg)
	}
	if fmt.Sprint(aerr.components) != "[[A B] [C]]" {
		t.Errorf("错误应附带连通分量，实际 %v", aerr.components)
	}
}

// ---------- 校验：整份拒绝 ----------

func TestValidationRejections(t *testing.T) {
	okResistors := []resistorIn{{ID: "R1", From: "A", To: "B", Milliohms: 1000}}
	cases := []struct {
		name string
		req  solveRequest
		want string
	}{
		{"节点太少", solveRequest{Nodes: []string{"A"}, Resistors: okResistors, Source: "A", Sink: "B"}, "2 至 10"},
		{"节点太多", solveRequest{Nodes: []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K"}, Resistors: okResistors, Source: "A", Sink: "B"}, "2 至 10"},
		{"节点重复", solveRequest{Nodes: []string{"A", "A"}, Resistors: okResistors, Source: "A", Sink: "A"}, "重复"},
		{"节点名非ASCII", solveRequest{Nodes: []string{"甲", "B"}, Resistors: okResistors, Source: "甲", Sink: "B"}, "ASCII"},
		{"节点名为空", solveRequest{Nodes: []string{"", "B"}, Resistors: okResistors, Source: "", Sink: "B"}, "非法"},
		{"电阻为零条", solveRequest{Nodes: []string{"A", "B"}, Source: "A", Sink: "B"}, "1 至 18"},
		{"电阻超过18条", solveRequest{Nodes: []string{"A", "B"}, Resistors: manyResistors(19), Source: "A", Sink: "B"}, "1 至 18"},
		{"电阻id重复", solveRequest{Nodes: []string{"A", "B"}, Resistors: []resistorIn{
			{ID: "R1", From: "A", To: "B", Milliohms: 1000},
			{ID: "R1", From: "B", To: "A", Milliohms: 2000},
		}, Source: "A", Sink: "B"}, "重复"},
		{"自环", solveRequest{Nodes: []string{"A", "B"}, Resistors: []resistorIn{{ID: "R1", From: "A", To: "A", Milliohms: 1000}}, Source: "A", Sink: "B"}, "自环"},
		{"未知from节点", solveRequest{Nodes: []string{"A", "B"}, Resistors: []resistorIn{{ID: "R1", From: "A", To: "Z", Milliohms: 1000}}, Source: "A", Sink: "B"}, "未知节点"},
		{"阻值为零", solveRequest{Nodes: []string{"A", "B"}, Resistors: []resistorIn{{ID: "R1", From: "A", To: "B", Milliohms: 0}}, Source: "A", Sink: "B"}, "正整数毫欧"},
		{"阻值为负", solveRequest{Nodes: []string{"A", "B"}, Resistors: []resistorIn{{ID: "R1", From: "A", To: "B", Milliohms: -5}}, Source: "A", Sink: "B"}, "正整数毫欧"},
		{"端点相同", solveRequest{Nodes: []string{"A", "B"}, Resistors: okResistors, Source: "A", Sink: "A"}, "必须不同"},
		{"端点未知", solveRequest{Nodes: []string{"A", "B"}, Resistors: okResistors, Source: "A", Sink: "Z"}, "不是已知节点"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, aerr := analyze(&tc.req)
			if aerr == nil {
				t.Fatalf("应拒绝但未拒绝")
			}
			if !strings.Contains(aerr.msg, tc.want) {
				t.Errorf("错误信息 %q 不含 %q", aerr.msg, tc.want)
			}
		})
	}
}

func manyResistors(n int) []resistorIn {
	out := make([]resistorIn, n)
	for i := range out {
		out[i] = resistorIn{ID: fmt.Sprintf("R%d", i+1), From: "A", To: "B", Milliohms: 1000}
	}
	return out
}

// ---------- 电路方程核对（性质测试） ----------

// 对一批确定性伪随机连通网络，核对响应满足：
//  1. 每条边欧姆定律 I = (V_from - V_to) / R
//  2. 每个节点 KCL：流出电流之和 = 注入电流（source 1 A，sink -1 A，其余 0）
//  3. 等效电阻 = V_source - V_sink
//  4. 所有分数已约分且分母为正
func TestCircuitEquationsHold(t *testing.T) {
	rng := rand.New(rand.NewSource(20260925))
	for trial := 0; trial < 50; trial++ {
		n := 2 + rng.Intn(9) // 2..10 个节点
		nodes := make([]string, n)
		for i := range nodes {
			nodes[i] = string(rune('A' + i))
		}
		// 先生成一条随机链保证连通，再随机加边（允许并联）。
		var edges [][2]int
		perm := rng.Perm(n)
		for i := 0; i+1 < n; i++ {
			edges = append(edges, [2]int{perm[i], perm[i+1]})
		}
		for len(edges) < 18 && rng.Intn(2) == 0 {
			a, b := rng.Intn(n), rng.Intn(n)
			if a != b {
				edges = append(edges, [2]int{a, b})
			}
		}
		var resistors []resistorIn
		for i, e := range edges {
			resistors = append(resistors, resistorIn{
				ID: fmt.Sprintf("R%d", i+1), From: nodes[e[0]], To: nodes[e[1]],
				Milliohms: int64(1 + rng.Intn(9000)),
			})
		}
		src, snk := nodes[rng.Intn(n)], nodes[rng.Intn(n)]
		for snk == src {
			snk = nodes[rng.Intn(n)]
		}
		req := solveRequest{Nodes: nodes, Resistors: resistors, Source: src, Sink: snk}
		s := mustSolve(t, req)

		pot := map[string]*big.Rat{}
		for _, p := range s.Potentials {
			pot[p.Node] = rat(p.Potential.Num, p.Potential.Den)
			assertReduced(t, p.Potential)
		}
		if pot[snk].Sign() != 0 {
			t.Fatalf("trial %d: sink 电位应为 0，实际 %v", trial, pot[snk])
		}
		injection := map[string]*big.Rat{src: big.NewRat(1, 1), snk: big.NewRat(-1, 1)}
		for i, c := range s.Currents {
			assertReduced(t, c.Current)
			cur := rat(c.Current.Num, c.Current.Den)
			// 欧姆定律
			ohm := new(big.Rat).SetFrac64(req.Resistors[i].Milliohms, 1000)
			lhs := new(big.Rat).Mul(cur, ohm)
			rhs := new(big.Rat).Sub(pot[c.From], pot[c.To])
			if lhs.Cmp(rhs) != 0 {
				t.Fatalf("trial %d: 边 %s 违反欧姆定律: I·R=%v, ΔV=%v", trial, c.ID, lhs, rhs)
			}
			// KCL 累计流出量
			for node, sign := range map[string]int64{c.From: 1, c.To: -1} {
				injection[node] = new(big.Rat).Sub(defaultRat(injection[node]), new(big.Rat).Mul(big.NewRat(sign, 1), cur))
			}
		}
		for node, rest := range injection {
			if rest.Sign() != 0 {
				t.Fatalf("trial %d: 节点 %s 违反 KCL，残差 %v", trial, node, rest)
			}
		}
		// 等效电阻 = 端电压 / 1 A
		eq := rat(s.EquivalentResistance.Num, s.EquivalentResistance.Den)
		if eq.Cmp(new(big.Rat).Sub(pot[src], pot[snk])) != 0 {
			t.Fatalf("trial %d: 等效电阻与端电压不符", trial)
		}
		if eq.Sign() <= 0 {
			t.Fatalf("trial %d: 连通正电阻网络的等效电阻应为正，实际 %v", trial, eq)
		}
	}
}

func defaultRat(r *big.Rat) *big.Rat {
	if r == nil {
		return new(big.Rat)
	}
	return r
}

func assertReduced(t *testing.T, f fraction) {
	t.Helper()
	num, _ := new(big.Int).SetString(f.Num, 10)
	den, ok := new(big.Int).SetString(f.Den, 10)
	if num == nil || !ok {
		t.Fatalf("分数 %s/%s 不是合法整数", f.Num, f.Den)
	}
	if den.Sign() <= 0 {
		t.Fatalf("分母必须为正: %s/%s", f.Num, f.Den)
	}
	g := new(big.Int).GCD(nil, nil, new(big.Int).Abs(num), den)
	if g.Cmp(big.NewInt(1)) != 0 {
		t.Fatalf("分数 %s/%s 未约分", f.Num, f.Den)
	}
}

// ---------- HTTP 层 ----------

func postBody(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/solve", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	handleSolve(rec, req)
	return rec
}

func TestHTTPSolved(t *testing.T) {
	rec := postBody(t, `{
		"nodes": ["A", "B"],
		"resistors": [{"id": "R1", "from": "A", "to": "B", "milliohms": 500}],
		"source": "A", "sink": "B"
	}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("状态码 = %d，期望 200；body=%s", rec.Code, rec.Body)
	}
	var s solvedResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatalf("响应不是合法 JSON: %v", err)
	}
	if s.Status != "SOLVED" || s.EquivalentResistance != frac("1", "2") {
		t.Errorf("响应内容异常: %+v", s)
	}
}

func TestHTTPOpen(t *testing.T) {
	rec := postBody(t, `{
		"nodes": ["A", "B"],
		"resistors": [{"id": "R1", "from": "A", "to": "A", "milliohms": 500}],
		"source": "A", "sink": "B"
	}`)
	// 自环应先被校验拒绝
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("自环应 400，实际 %d", rec.Code)
	}
	rec = postBody(t, `{
		"nodes": ["A", "B", "C"],
		"resistors": [{"id": "R1", "from": "A", "to": "B", "milliohms": 500}],
		"source": "A", "sink": "C"
	}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("OPEN 应 200，实际 %d", rec.Code)
	}
	var o openResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &o); err != nil {
		t.Fatalf("响应不是合法 JSON: %v", err)
	}
	if o.Status != "OPEN" || len(o.Components) != 2 {
		t.Errorf("OPEN 响应异常: %+v", o)
	}
}

func TestHTTPRejections(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"多余字段", `{"nodes":["A","B"],"resistors":[{"id":"R1","from":"A","to":"B","milliohms":1}],"source":"A","sink":"B","extra":1}`},
		{"电阻内多余字段", `{"nodes":["A","B"],"resistors":[{"id":"R1","from":"A","to":"B","milliohms":1,"foo":2}],"source":"A","sink":"B"}`},
		{"尾部垃圾", `{"nodes":["A","B"],"resistors":[{"id":"R1","from":"A","to":"B","milliohms":1}],"source":"A","sink":"B"} {}`},
		{"非法JSON", `{"nodes":`},
		{"非整数毫欧", `{"nodes":["A","B"],"resistors":[{"id":"R1","from":"A","to":"B","milliohms":1.5}],"source":"A","sink":"B"}`},
		{"孤立分量", `{"nodes":["A","B","C"],"resistors":[{"id":"R1","from":"A","to":"B","milliohms":1}],"source":"A","sink":"B"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := postBody(t, tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("状态码 = %d，期望 400；body=%s", rec.Code, rec.Body)
			}
			var e errorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil || e.Error == "" {
				t.Errorf("错误响应应含 error 字段: %s", rec.Body)
			}
		})
	}
}

func TestHTTPMethodNotAllowed(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/solve", handleSolve)
	req := httptest.NewRequest(http.MethodGet, "/api/solve", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /api/solve 应 405，实际 %d", rec.Code)
	}
}
