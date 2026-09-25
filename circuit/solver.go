package main

import (
	"fmt"
	"math/big"
	"regexp"
	"sort"
)

const (
	minNodes     = 2
	maxNodes     = 10
	minEdges     = 1
	maxEdges     = 18
	maxMilliohms = 1_000_000_000 // sanity cap: 1e9 mΩ = 1 MΩ

	StatusOK    = "OK"
	StatusOpen  = "OPEN"
	StatusError = "ERROR"
)

// Node and edge ids: short printable ASCII tokens.
var nameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,15}$`)

type Edge struct {
	ID        string `json:"id"`
	From      string `json:"from"`
	To        string `json:"to"`
	Milliohms int64  `json:"milliohms"`
}

type SolveRequest struct {
	Nodes  []string `json:"nodes"`
	Edges  []Edge   `json:"edges"`
	Source string   `json:"source"`
	Sink   string   `json:"sink"`
}

type Potential struct {
	Node       string `json:"node"`
	Millivolts string `json:"millivolts"` // reduced fraction, e.g. "500" or "2000/3"
}

type Current struct {
	ID      string `json:"id"`
	From    string `json:"from"`
	To      string `json:"to"`
	Amperes string `json:"amperes"` // reduced fraction, signed in the from→to direction
}

type SolveResponse struct {
	Status              string      `json:"status"`                        // OK | OPEN | ERROR
	EquivalentMilliohms string      `json:"equivalentMilliohms,omitempty"` // OK only
	Potentials          []Potential `json:"potentials,omitempty"`          // OK only, sorted by node
	Currents            []Current   `json:"currents,omitempty"`            // OK only, input edge order
	Components          [][]string  `json:"components,omitempty"`          // OPEN only
	Message             string      `json:"message,omitempty"`             // ERROR only
}

func (req *SolveRequest) validate() error {
	if len(req.Nodes) < minNodes || len(req.Nodes) > maxNodes {
		return fmt.Errorf("nodes: need %d..%d unique names, got %d", minNodes, maxNodes, len(req.Nodes))
	}
	known := make(map[string]bool, len(req.Nodes))
	for _, n := range req.Nodes {
		if !nameRE.MatchString(n) {
			return fmt.Errorf("node %q: must be 1-16 ASCII chars from [A-Za-z0-9_-], starting with a letter or digit", n)
		}
		if known[n] {
			return fmt.Errorf("duplicate node %q", n)
		}
		known[n] = true
	}

	if len(req.Edges) < minEdges || len(req.Edges) > maxEdges {
		return fmt.Errorf("edges: need %d..%d, got %d", minEdges, maxEdges, len(req.Edges))
	}
	ids := make(map[string]bool, len(req.Edges))
	for _, e := range req.Edges {
		if !nameRE.MatchString(e.ID) {
			return fmt.Errorf("edge id %q: must be 1-16 ASCII chars from [A-Za-z0-9_-], starting with a letter or digit", e.ID)
		}
		if ids[e.ID] {
			return fmt.Errorf("duplicate edge id %q", e.ID)
		}
		ids[e.ID] = true
		if !known[e.From] {
			return fmt.Errorf("edge %q: unknown node %q", e.ID, e.From)
		}
		if !known[e.To] {
			return fmt.Errorf("edge %q: unknown node %q", e.ID, e.To)
		}
		if e.From == e.To {
			return fmt.Errorf("edge %q: self-loops are not allowed", e.ID)
		}
		if e.Milliohms < 1 || e.Milliohms > maxMilliohms {
			return fmt.Errorf("edge %q: milliohms must be a positive integer (1..%d)", e.ID, maxMilliohms)
		}
	}

	if !known[req.Source] {
		return fmt.Errorf("unknown source node %q", req.Source)
	}
	if !known[req.Sink] {
		return fmt.Errorf("unknown sink node %q", req.Sink)
	}
	if req.Source == req.Sink {
		return fmt.Errorf("source and sink must be two different nodes")
	}
	return nil
}

// components returns the connected components of the undirected graph,
// each sorted, and the list sorted by first member (deterministic output).
func components(nodes []string, edges []Edge) [][]string {
	parent := make(map[string]string, len(nodes))
	for _, n := range nodes {
		parent[n] = n
	}
	var find func(x string) string
	find = func(x string) string {
		if parent[x] != x {
			parent[x] = find(parent[x])
		}
		return parent[x]
	}
	for _, e := range edges {
		a, b := find(e.From), find(e.To)
		if a != b {
			parent[a] = b
		}
	}
	groups := map[string][]string{}
	for _, n := range nodes {
		r := find(n)
		groups[r] = append(groups[r], n)
	}
	out := make([][]string, 0, len(groups))
	for _, g := range groups {
		sort.Strings(g)
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i][0] < out[j][0] })
	return out
}

// solve runs the full pipeline: connectivity classification, then exact
// rational nodal analysis with 1 A injected at Source and extracted at Sink,
// Sink held at 0 V. Units: resistances in mΩ, potentials in mV, currents in A
// (1 A × 1 mΩ = 1 mV, so the numbers stay consistent).
func solve(req *SolveRequest) *SolveResponse {
	comps := components(req.Nodes, req.Edges)

	endpointsConnected := false
	for _, c := range comps {
		hasSource, hasSink := false, false
		for _, n := range c {
			if n == req.Source {
				hasSource = true
			}
			if n == req.Sink {
				hasSink = true
			}
		}
		if hasSource && hasSink {
			endpointsConnected = true
		}
	}
	if !endpointsConnected {
		return &SolveResponse{Status: StatusOpen, Components: comps}
	}
	if len(comps) > 1 {
		return &SolveResponse{
			Status:  StatusError,
			Message: "input has isolated components unrelated to the endpoints",
		}
	}

	// Unknowns: every node except the sink (ground). Sorted for determinism.
	nodes := append([]string(nil), req.Nodes...)
	sort.Strings(nodes)
	idx := make(map[string]int, len(nodes)-1)
	for _, n := range nodes {
		if n == req.Sink {
			continue
		}
		idx[n] = len(idx)
	}
	n := len(idx)

	a := make([][]*big.Rat, n)
	for i := range a {
		a[i] = make([]*big.Rat, n)
		for j := range a[i] {
			a[i][j] = new(big.Rat)
		}
	}
	b := make([]*big.Rat, n)
	for i := range b {
		b[i] = new(big.Rat)
	}

	// KCL at every non-ground node: Σ (V_node − V_neighbor) / R = injected current.
	for _, e := range req.Edges {
		g := new(big.Rat).SetFrac(big.NewInt(1), big.NewInt(e.Milliohms))
		i, iOK := idx[e.From]
		j, jOK := idx[e.To]
		if iOK {
			a[i][i].Add(a[i][i], g)
			if jOK {
				a[i][j].Sub(a[i][j], g)
			}
		}
		if jOK {
			a[j][j].Add(a[j][j], g)
			if iOK {
				a[j][i].Sub(a[j][i], g)
			}
		}
	}
	b[idx[req.Source]].SetInt64(1) // 1 A injected at source, extracted at ground

	x := gaussEliminate(a, b)

	v := map[string]*big.Rat{req.Sink: new(big.Rat)}
	for node, i := range idx {
		v[node] = x[i]
	}

	resp := &SolveResponse{Status: StatusOK}
	resp.EquivalentMilliohms = new(big.Rat).Sub(v[req.Source], v[req.Sink]).RatString()
	for _, node := range nodes {
		resp.Potentials = append(resp.Potentials, Potential{Node: node, Millivolts: v[node].RatString()})
	}
	for _, e := range req.Edges {
		g := new(big.Rat).SetFrac(big.NewInt(1), big.NewInt(e.Milliohms))
		d := new(big.Rat).Sub(v[e.From], v[e.To])
		current := new(big.Rat).Mul(d, g)
		resp.Currents = append(resp.Currents, Current{
			ID: e.ID, From: e.From, To: e.To, Amperes: current.RatString(),
		})
	}
	return resp
}

// gaussEliminate solves A x = b exactly over the rationals, in place.
// The conductance matrix of a connected resistor network is strictly
// diagonally dominant, so a nonzero pivot always exists on the diagonal
// after a row search; no column pivoting is needed.
func gaussEliminate(a [][]*big.Rat, b []*big.Rat) []*big.Rat {
	n := len(a)
	for col := 0; col < n; col++ {
		pivot := -1
		for r := col; r < n; r++ {
			if a[r][col].Sign() != 0 {
				pivot = r
				break
			}
		}
		if pivot < 0 {
			panic("singular conductance matrix") // unreachable for a connected network
		}
		a[col], a[pivot] = a[pivot], a[col]
		b[col], b[pivot] = b[pivot], b[col]

		for r := col + 1; r < n; r++ {
			if a[r][col].Sign() == 0 {
				continue
			}
			f := new(big.Rat).Quo(a[r][col], a[col][col])
			for c := col; c < n; c++ {
				a[r][c].Sub(a[r][c], new(big.Rat).Mul(f, a[col][c]))
			}
			b[r].Sub(b[r], new(big.Rat).Mul(f, b[col]))
		}
	}

	x := make([]*big.Rat, n)
	for i := n - 1; i >= 0; i-- {
		s := new(big.Rat).Set(b[i])
		for j := i + 1; j < n; j++ {
			s.Sub(s, new(big.Rat).Mul(a[i][j], x[j]))
		}
		x[i] = new(big.Rat).Quo(s, a[i][i])
	}
	return x
}
