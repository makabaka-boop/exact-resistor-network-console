package main

import (
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func mustSolve(t *testing.T, req SolveRequest) SolveResponse {
	t.Helper()
	if err := req.validate(); err != nil {
		t.Fatalf("validate() rejected a valid fixture: %v", err)
	}
	resp := solve(&req)
	if resp.Status != StatusOK {
		t.Fatalf("solve() status = %s, want OK (resp=%+v)", resp.Status, resp)
	}
	return *resp
}

func potentialMap(t *testing.T, resp SolveResponse) map[string]string {
	t.Helper()
	m := map[string]string{}
	for _, p := range resp.Potentials {
		m[p.Node] = p.Millivolts
	}
	return m
}

func currentMap(t *testing.T, resp SolveResponse) map[string]string {
	t.Helper()
	m := map[string]string{}
	for _, c := range resp.Currents {
		m[c.ID] = c.Amperes
	}
	return m
}

func seriesRequest() SolveRequest {
	return SolveRequest{
		Nodes: []string{"A", "B", "C"},
		Edges: []Edge{
			{ID: "R1", From: "A", To: "B", Milliohms: 1000},
			{ID: "R2", From: "B", To: "C", Milliohms: 2000},
		},
		Source: "A",
		Sink:   "C",
	}
}

func parallelRequest() SolveRequest {
	return SolveRequest{
		Nodes: []string{"A", "B"},
		Edges: []Edge{
			{ID: "R1", From: "A", To: "B", Milliohms: 1000},
			{ID: "R2", From: "A", To: "B", Milliohms: 2000},
		},
		Source: "A",
		Sink:   "B",
	}
}

func bridgeRequest() SolveRequest {
	return SolveRequest{
		Nodes: []string{"A", "B", "C", "D"},
		Edges: []Edge{
			{ID: "R1", From: "A", To: "B", Milliohms: 1000},
			{ID: "R2", From: "A", To: "C", Milliohms: 1000},
			{ID: "R3", From: "B", To: "D", Milliohms: 1000},
			{ID: "R4", From: "C", To: "D", Milliohms: 1000},
			{ID: "R5", From: "B", To: "C", Milliohms: 1000},
		},
		Source: "A",
		Sink:   "D",
	}
}

func TestSeriesExactValues(t *testing.T) {
	resp := mustSolve(t, seriesRequest())

	if got, want := resp.EquivalentMilliohms, "3000"; got != want {
		t.Errorf("equivalent = %s, want %s", got, want)
	}
	wantV := map[string]string{"A": "3000", "B": "2000", "C": "0"}
	if got := potentialMap(t, resp); !reflect.DeepEqual(got, wantV) {
		t.Errorf("potentials = %v, want %v", got, wantV)
	}
	wantI := map[string]string{"R1": "1", "R2": "1"}
	if got := currentMap(t, resp); !reflect.DeepEqual(got, wantI) {
		t.Errorf("currents = %v, want %v", got, wantI)
	}
}

func TestParallelExactFractions(t *testing.T) {
	resp := mustSolve(t, parallelRequest())

	// 1000 mΩ ∥ 2000 mΩ = 2000/3 mΩ; 1 A splits 2/3 and 1/3.
	if got, want := resp.EquivalentMilliohms, "2000/3"; got != want {
		t.Errorf("equivalent = %s, want %s", got, want)
	}
	wantV := map[string]string{"A": "2000/3", "B": "0"}
	if got := potentialMap(t, resp); !reflect.DeepEqual(got, wantV) {
		t.Errorf("potentials = %v, want %v", got, wantV)
	}
	wantI := map[string]string{"R1": "2/3", "R2": "1/3"}
	if got := currentMap(t, resp); !reflect.DeepEqual(got, wantI) {
		t.Errorf("currents = %v, want %v", got, wantI)
	}
}

func TestFractionsAreReduced(t *testing.T) {
	req := SolveRequest{
		Nodes: []string{"A", "B"},
		Edges: []Edge{
			{ID: "R1", From: "A", To: "B", Milliohms: 1000},
			{ID: "R2", From: "A", To: "B", Milliohms: 1000},
		},
		Source: "A",
		Sink:   "B",
	}
	resp := mustSolve(t, req)
	// 1000 ∥ 1000 = 500 exactly — must be "500", not "1000/2".
	if got, want := resp.EquivalentMilliohms, "500"; got != want {
		t.Errorf("equivalent = %s, want %s", got, want)
	}
	for _, c := range resp.Currents {
		if c.Amperes != "1/2" {
			t.Errorf("current %s = %s, want 1/2", c.ID, c.Amperes)
		}
	}
}

func TestBalancedBridgeExactZero(t *testing.T) {
	resp := mustSolve(t, bridgeRequest())

	// Balanced Wheatstone bridge: equivalent 1000 mΩ, mid-branch current
	// is exactly zero — no floating-point dust masquerading as zero.
	if got, want := resp.EquivalentMilliohms, "1000"; got != want {
		t.Errorf("equivalent = %s, want %s", got, want)
	}
	wantV := map[string]string{"A": "1000", "B": "500", "C": "500", "D": "0"}
	if got := potentialMap(t, resp); !reflect.DeepEqual(got, wantV) {
		t.Errorf("potentials = %v, want %v", got, wantV)
	}
	wantI := map[string]string{"R1": "1/2", "R2": "1/2", "R3": "1/2", "R4": "1/2", "R5": "0"}
	if got := currentMap(t, resp); !reflect.DeepEqual(got, wantI) {
		t.Errorf("currents = %v, want %v", got, wantI)
	}
}

// checkCircuitEquations verifies the returned solution against the circuit
// equations themselves: Ohm's law on every edge, KCL at every node, and
// consistency of the equivalent resistance with the terminal potentials.
func checkCircuitEquations(t *testing.T, req SolveRequest, resp SolveResponse) {
	t.Helper()

	v := map[string]*big.Rat{}
	for _, p := range resp.Potentials {
		r, ok := new(big.Rat).SetString(p.Millivolts)
		if !ok {
			t.Fatalf("potential of node %s is not a fraction: %q", p.Node, p.Millivolts)
		}
		if _, dup := v[p.Node]; dup {
			t.Fatalf("node %s listed twice in potentials", p.Node)
		}
		v[p.Node] = r
	}
	if len(v) != len(req.Nodes) {
		t.Fatalf("potentials cover %d nodes, want %d", len(v), len(req.Nodes))
	}
	for _, n := range req.Nodes {
		if _, ok := v[n]; !ok {
			t.Fatalf("node %s missing from potentials", n)
		}
	}
	if v[req.Sink].Sign() != 0 {
		t.Errorf("sink potential = %s, want 0", v[req.Sink].RatString())
	}

	// Equivalent resistance must equal the terminal voltage at 1 A.
	eq, ok := new(big.Rat).SetString(resp.EquivalentMilliohms)
	if !ok {
		t.Fatalf("equivalent is not a fraction: %q", resp.EquivalentMilliohms)
	}
	term := new(big.Rat).Sub(v[req.Source], v[req.Sink])
	if eq.Cmp(term) != 0 {
		t.Errorf("equivalent = %s, but V(source)-V(sink) = %s", eq.RatString(), term.RatString())
	}

	// Ohm's law per edge, and accumulate KCL outflows per node.
	outflow := map[string]*big.Rat{}
	for _, n := range req.Nodes {
		outflow[n] = new(big.Rat)
	}
	if len(resp.Currents) != len(req.Edges) {
		t.Fatalf("got %d currents for %d edges", len(resp.Currents), len(req.Edges))
	}
	for i, e := range req.Edges {
		c := resp.Currents[i]
		if c.ID != e.ID || c.From != e.From || c.To != e.To {
			t.Fatalf("current[%d] = %+v, want edge %+v (input order)", i, c, e)
		}
		cur, ok := new(big.Rat).SetString(c.Amperes)
		if !ok {
			t.Fatalf("current of edge %s is not a fraction: %q", e.ID, c.Amperes)
		}
		// I·R must equal V(from) − V(to).
		lhs := new(big.Rat).Mul(cur, big.NewRat(e.Milliohms, 1))
		rhs := new(big.Rat).Sub(v[e.From], v[e.To])
		if lhs.Cmp(rhs) != 0 {
			t.Errorf("edge %s: I·R = %s, but ΔV = %s", e.ID, lhs.RatString(), rhs.RatString())
		}
		outflow[e.From].Add(outflow[e.From], cur)
		outflow[e.To].Sub(outflow[e.To], cur)
	}

	// KCL: net outflow is +1 A at the source, −1 A at the sink, 0 elsewhere.
	for _, n := range req.Nodes {
		want := new(big.Rat)
		switch n {
		case req.Source:
			want.SetInt64(1)
		case req.Sink:
			want.SetInt64(-1)
		}
		if outflow[n].Cmp(want) != 0 {
			t.Errorf("KCL at node %s: net outflow = %s, want %s", n, outflow[n].RatString(), want.RatString())
		}
	}
}

func TestUnbalancedBridgeSatisfiesEquations(t *testing.T) {
	// A bridge that cannot be reduced by series/parallel rules, with
	// awkward coprime values to force nontrivial fractions.
	req := SolveRequest{
		Nodes: []string{"A", "B", "C", "D"},
		Edges: []Edge{
			{ID: "R1", From: "A", To: "B", Milliohms: 1000},
			{ID: "R2", From: "A", To: "C", Milliohms: 2000},
			{ID: "R3", From: "B", To: "D", Milliohms: 3000},
			{ID: "R4", From: "C", To: "D", Milliohms: 4000},
			{ID: "R5", From: "B", To: "C", Milliohms: 5000},
		},
		Source: "A",
		Sink:   "D",
	}
	resp := mustSolve(t, req)
	checkCircuitEquations(t, req, resp)

	// The mid-branch current must be a small nonzero fraction — exactly
	// representable, never rounded to zero.
	mid := currentMap(t, resp)["R5"]
	r, ok := new(big.Rat).SetString(mid)
	if !ok || r.Sign() == 0 {
		t.Errorf("mid-branch current = %q, want a nonzero fraction", mid)
	}
}

func TestFixturesSatisfyEquations(t *testing.T) {
	for name, req := range map[string]SolveRequest{
		"series":   seriesRequest(),
		"parallel": parallelRequest(),
		"bridge":   bridgeRequest(),
	} {
		t.Run(name, func(t *testing.T) {
			checkCircuitEquations(t, req, mustSolve(t, req))
		})
	}
}

func TestOpenEndpoints(t *testing.T) {
	req := SolveRequest{
		Nodes: []string{"A", "B", "C", "D"},
		Edges: []Edge{
			{ID: "R1", From: "A", To: "B", Milliohms: 1000},
			{ID: "R2", From: "C", To: "D", Milliohms: 1000},
		},
		Source: "A",
		Sink:   "D",
	}
	resp := solve(&req)
	if resp.Status != StatusOpen {
		t.Fatalf("status = %s, want OPEN", resp.Status)
	}
	want := [][]string{{"A", "B"}, {"C", "D"}}
	if !reflect.DeepEqual(resp.Components, want) {
		t.Errorf("components = %v, want %v", resp.Components, want)
	}
}

func TestIsolatedComponentIsInputError(t *testing.T) {
	// Endpoints are connected, but C–D is a completely unrelated component.
	req := SolveRequest{
		Nodes: []string{"A", "B", "C", "D"},
		Edges: []Edge{
			{ID: "R1", From: "A", To: "B", Milliohms: 1000},
			{ID: "R2", From: "C", To: "D", Milliohms: 1000},
		},
		Source: "A",
		Sink:   "B",
	}
	resp := solve(&req)
	if resp.Status != StatusError {
		t.Fatalf("status = %s, want ERROR", resp.Status)
	}
	if resp.Message == "" {
		t.Error("ERROR response must carry a message")
	}
}

func TestValidation(t *testing.T) {
	valid := bridgeRequest()

	cases := []struct {
		name   string
		mutate func(*SolveRequest)
	}{
		{"too few nodes", func(r *SolveRequest) { r.Nodes = []string{"A"} }},
		{"too many nodes", func(r *SolveRequest) { r.Nodes = []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K"} }},
		{"duplicate node", func(r *SolveRequest) { r.Nodes = []string{"A", "A", "C", "D"} }},
		{"bad node charset", func(r *SolveRequest) { r.Nodes = []string{"A", "B", "C", "节"} }},
		{"no edges", func(r *SolveRequest) { r.Edges = nil }},
		{"too many edges", func(r *SolveRequest) {
			r.Edges = nil
			for i := 0; i < 19; i++ {
				r.Edges = append(r.Edges, Edge{ID: "E" + strings.Repeat("x", 0) + string(rune('a'+i)), From: "A", To: "B", Milliohms: 1})
			}
		}},
		{"duplicate edge id", func(r *SolveRequest) { r.Edges[1].ID = r.Edges[0].ID }},
		{"unknown from node", func(r *SolveRequest) { r.Edges[0].From = "Z" }},
		{"unknown to node", func(r *SolveRequest) { r.Edges[0].To = "Z" }},
		{"self loop", func(r *SolveRequest) { r.Edges[0].To = r.Edges[0].From }},
		{"zero milliohms", func(r *SolveRequest) { r.Edges[0].Milliohms = 0 }},
		{"negative milliohms", func(r *SolveRequest) { r.Edges[0].Milliohms = -5 }},
		{"unknown source", func(r *SolveRequest) { r.Source = "Z" }},
		{"unknown sink", func(r *SolveRequest) { r.Sink = "Z" }},
		{"same endpoints", func(r *SolveRequest) { r.Sink = r.Source }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := valid
			// deep-copy the slices so mutations don't leak between cases
			req.Nodes = append([]string(nil), valid.Nodes...)
			req.Edges = append([]Edge(nil), valid.Edges...)
			tc.mutate(&req)
			if err := req.validate(); err == nil {
				t.Errorf("validate() accepted invalid request %+v", req)
			}
		})
	}

	if err := valid.validate(); err != nil {
		t.Errorf("validate() rejected the valid fixture: %v", err)
	}
}

func postBody(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/solve", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handleSolve(rec, req)
	return rec
}

func TestHandlerValidFlow(t *testing.T) {
	rec := postBody(t, `{
		"nodes": ["A", "B"],
		"edges": [{"id": "R1", "from": "A", "to": "B", "milliohms": 1000}],
		"source": "A", "sink": "B"
	}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body)
	}
	var resp SolveResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if resp.Status != StatusOK || resp.EquivalentMilliohms != "1000" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestHandlerOpenFlow(t *testing.T) {
	rec := postBody(t, `{
		"nodes": ["A", "B", "C", "D"],
		"edges": [
			{"id": "R1", "from": "A", "to": "B", "milliohms": 1000},
			{"id": "R2", "from": "C", "to": "D", "milliohms": 1000}
		],
		"source": "A", "sink": "D"
	}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("OPEN must still be HTTP 200, got %d", rec.Code)
	}
	var resp SolveResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if resp.Status != StatusOpen || len(resp.Components) != 2 {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestHandlerRejections(t *testing.T) {
	cases := map[string]string{
		"extra top-level field": `{"nodes":["A","B"],"edges":[{"id":"R1","from":"A","to":"B","milliohms":1}],"source":"A","sink":"B","bogus":1}`,
		"extra edge field":      `{"nodes":["A","B"],"edges":[{"id":"R1","from":"A","to":"B","milliohms":1,"color":"red"}],"source":"A","sink":"B"}`,
		"unknown node":          `{"nodes":["A","B"],"edges":[{"id":"R1","from":"A","to":"Z","milliohms":1}],"source":"A","sink":"B"}`,
		"non-integer milliohms": `{"nodes":["A","B"],"edges":[{"id":"R1","from":"A","to":"B","milliohms":1.5}],"source":"A","sink":"B"}`,
		"trailing garbage":      `{"nodes":["A","B"],"edges":[{"id":"R1","from":"A","to":"B","milliohms":1}],"source":"A","sink":"B"} {}`,
		"not json":              `hello`,
		"isolated component":    `{"nodes":["A","B","C","D"],"edges":[{"id":"R1","from":"A","to":"B","milliohms":1},{"id":"R2","from":"C","to":"D","milliohms":1}],"source":"A","sink":"B"}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			rec := postBody(t, body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body)
			}
			var resp SolveResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("error response is not JSON: %v", err)
			}
			if resp.Status != StatusError || resp.Message == "" {
				t.Errorf("unexpected error response: %+v", resp)
			}
		})
	}
}
