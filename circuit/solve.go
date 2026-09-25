package main

import (
	"fmt"
	"math/big"
)

// ---------- 请求 / 响应类型 ----------

type resistorIn struct {
	ID        string `json:"id"`
	From      string `json:"from"`
	To        string `json:"to"`
	Milliohms int64  `json:"milliohms"`
}

type solveRequest struct {
	Nodes     []string     `json:"nodes"`
	Resistors []resistorIn `json:"resistors"`
	Source    string       `json:"source"`
	Sink      string       `json:"sink"`
}

// fraction 是约分后的分数，num/den 均为十进制整数字符串，den > 0。
// 用字符串而不用浮点，避免把很小但非零的支路电流近似成 0。
type fraction struct {
	Num string `json:"num"`
	Den string `json:"den"`
}

func ratFraction(r *big.Rat) fraction {
	return fraction{Num: r.Num().String(), Den: r.Denom().String()}
}

type nodePotential struct {
	Node      string   `json:"node"`
	Potential fraction `json:"potential"` // 伏特
}

type edgeCurrent struct {
	ID      string   `json:"id"`
	From    string   `json:"from"`
	To      string   `json:"to"`
	Current fraction `json:"current"` // 安培，正值表示 from → to
}

// solvedResponse 单位约定：阻值欧姆、电位伏特、电流安培。
type solvedResponse struct {
	Status               string          `json:"status"` // "SOLVED"
	Source               string          `json:"source"`
	Sink                 string          `json:"sink"`
	EquivalentResistance fraction        `json:"equivalentResistance"`
	Potentials           []nodePotential `json:"potentials"`
	Currents             []edgeCurrent   `json:"currents"`
}

type openResponse struct {
	Status     string     `json:"status"` // "OPEN"
	Components [][]string `json:"components"`
}

// apiError 表示一份被整体拒绝的输入。
type apiError struct {
	msg        string
	components [][]string // 可选：孤立分量等补充信息
}

func (e *apiError) Error() string { return e.msg }

// ---------- 校验 ----------

// isVisibleASCII 要求 1..max 个可见 ASCII 字符（0x21..0x7E）。
func isVisibleASCII(s string, max int) bool {
	if len(s) == 0 || len(s) > max {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < 33 || s[i] > 126 {
			return false
		}
	}
	return true
}

func (req *solveRequest) validate() *apiError {
	if len(req.Nodes) < 2 || len(req.Nodes) > 10 {
		return &apiError{msg: fmt.Sprintf("节点数量必须为 2 至 10 个，当前为 %d", len(req.Nodes))}
	}
	known := make(map[string]bool, len(req.Nodes))
	for _, n := range req.Nodes {
		if !isVisibleASCII(n, 16) {
			return &apiError{msg: fmt.Sprintf("节点名 %q 非法：需为 1-16 个可见 ASCII 字符", n)}
		}
		if known[n] {
			return &apiError{msg: fmt.Sprintf("节点 %q 重复", n)}
		}
		known[n] = true
	}
	if len(req.Resistors) < 1 || len(req.Resistors) > 18 {
		return &apiError{msg: fmt.Sprintf("电阻数量必须为 1 至 18 条，当前为 %d", len(req.Resistors))}
	}
	seenID := make(map[string]bool, len(req.Resistors))
	for _, r := range req.Resistors {
		if !isVisibleASCII(r.ID, 32) {
			return &apiError{msg: fmt.Sprintf("电阻 id %q 非法：需为 1-32 个可见 ASCII 字符", r.ID)}
		}
		if seenID[r.ID] {
			return &apiError{msg: fmt.Sprintf("电阻 id %q 重复", r.ID)}
		}
		seenID[r.ID] = true
		if !known[r.From] {
			return &apiError{msg: fmt.Sprintf("电阻 %s 引用了未知节点 %q", r.ID, r.From)}
		}
		if !known[r.To] {
			return &apiError{msg: fmt.Sprintf("电阻 %s 引用了未知节点 %q", r.ID, r.To)}
		}
		if r.From == r.To {
			return &apiError{msg: fmt.Sprintf("电阻 %s 是自环（%s → %s），不允许", r.ID, r.From, r.To)}
		}
		if r.Milliohms < 1 {
			return &apiError{msg: fmt.Sprintf("电阻 %s 的阻值必须为正整数毫欧，当前为 %d", r.ID, r.Milliohms)}
		}
	}
	if !known[req.Source] {
		return &apiError{msg: fmt.Sprintf("注入端点 %q 不是已知节点", req.Source)}
	}
	if !known[req.Sink] {
		return &apiError{msg: fmt.Sprintf("取走端点 %q 不是已知节点", req.Sink)}
	}
	if req.Source == req.Sink {
		return &apiError{msg: "两个端点必须不同"}
	}
	return nil
}

// ---------- 连通分量 ----------

// components 返回全部节点的连通分量（孤立节点自成分量），
// 分量与其内部节点均按输入顺序排列。
func components(nodes []string, resistors []resistorIn) [][]string {
	parent := make([]int, len(nodes))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(x int) int {
		for parent[x] != x {
			parent[x] = parent[parent[x]]
			x = parent[x]
		}
		return x
	}
	idx := make(map[string]int, len(nodes))
	for i, n := range nodes {
		idx[n] = i
	}
	for _, r := range resistors {
		if a, b := find(idx[r.From]), find(idx[r.To]); a != b {
			parent[a] = b
		}
	}
	groups := make(map[int][]string)
	var order []int
	for i, n := range nodes {
		root := find(i)
		if _, ok := groups[root]; !ok {
			order = append(order, root)
		}
		groups[root] = append(groups[root], n)
	}
	out := make([][]string, 0, len(order))
	for _, root := range order {
		out = append(out, groups[root])
	}
	return out
}

// ---------- 求解 ----------

// analyze 校验输入，区分 OPEN / 孤立分量错误 / 正常求解三种情况。
func analyze(req *solveRequest) (any, *apiError) {
	if err := req.validate(); err != nil {
		return nil, err
	}
	comps := components(req.Nodes, req.Resistors)
	connected := false
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
			connected = true
		}
	}
	if !connected {
		return openResponse{Status: "OPEN", Components: comps}, nil
	}
	if len(comps) > 1 {
		return nil, &apiError{msg: "存在与端点完全无关的孤立分量，整份输入被拒绝", components: comps}
	}
	return solve(req), nil
}

// solve 以 sink 为零电位，在 source 注入 1 A、sink 取走 1 A，
// 用精确有理数解节点电导方程。阻值按 欧姆 = 毫欧 / 1000 换算。
func solve(req *solveRequest) *solvedResponse {
	n := len(req.Nodes)
	idx := make(map[string]int, n)
	for i, name := range req.Nodes {
		idx[name] = i
	}
	sink, source := idx[req.Sink], idx[req.Source]

	// 节点间电导（并联边电导相加），单位西门子。
	g := make([][]*big.Rat, n)
	for i := range g {
		g[i] = make([]*big.Rat, n)
		for j := range g[i] {
			g[i][j] = new(big.Rat)
		}
	}
	conductance := make([]*big.Rat, len(req.Resistors))
	for k, r := range req.Resistors {
		res := new(big.Rat).SetFrac64(r.Milliohms, 1000) // 欧姆
		c := new(big.Rat).Inv(res)
		conductance[k] = c
		a, b := idx[r.From], idx[r.To]
		g[a][b].Add(g[a][b], c)
		g[b][a].Add(g[b][a], c)
	}

	// 变量：除 sink 外的所有节点（sink 固定 0 V）。
	m := n - 1
	row := make([]int, n)
	r := 0
	for i := 0; i < n; i++ {
		if i == sink {
			continue
		}
		row[i] = r
		r++
	}
	mat := make([][]*big.Rat, m)
	for i := range mat {
		mat[i] = make([]*big.Rat, m)
		for j := range mat[i] {
			mat[i][j] = new(big.Rat)
		}
	}
	rhs := make([]*big.Rat, m)
	for i := range rhs {
		rhs[i] = new(big.Rat)
	}
	for i := 0; i < n; i++ {
		if i == sink {
			continue
		}
		ri := row[i]
		diag := new(big.Rat)
		for j := 0; j < n; j++ {
			if j == i {
				continue
			}
			diag.Add(diag, g[i][j])
			if j != sink {
				mat[ri][row[j]].Sub(mat[ri][row[j]], g[i][j])
			}
		}
		mat[ri][ri] = diag
		if i == source {
			rhs[ri].SetInt64(1) // 注入 1 A
		}
	}
	v := gaussJordan(mat, rhs)

	pot := make([]*big.Rat, n)
	for i := 0; i < n; i++ {
		if i == sink {
			pot[i] = new(big.Rat)
		} else {
			pot[i] = v[row[i]]
		}
	}

	resp := &solvedResponse{Status: "SOLVED", Source: req.Source, Sink: req.Sink}
	// 注入 1 A，故 source 相对 sink 的电压数值上即等效电阻（欧姆）。
	resp.EquivalentResistance = ratFraction(new(big.Rat).Sub(pot[source], pot[sink]))
	for i, name := range req.Nodes {
		resp.Potentials = append(resp.Potentials, nodePotential{Node: name, Potential: ratFraction(pot[i])})
	}
	for k, ri := range req.Resistors {
		d := new(big.Rat).Sub(pot[idx[ri.From]], pot[idx[ri.To]])
		d.Mul(d, conductance[k])
		resp.Currents = append(resp.Currents, edgeCurrent{ID: ri.ID, From: ri.From, To: ri.To, Current: ratFraction(d)})
	}
	return resp
}

// gaussJordan 用精确有理数做高斯-若尔当消元，返回解向量。
// 连通网络约化后的节点电导矩阵对称正定，主元必非零。
func gaussJordan(a [][]*big.Rat, b []*big.Rat) []*big.Rat {
	n := len(a)
	for col := 0; col < n; col++ {
		piv := -1
		for r := col; r < n; r++ {
			if a[r][col].Sign() != 0 {
				piv = r
				break
			}
		}
		if piv < 0 {
			panic("节点方程矩阵奇异（图应连通却不可逆）")
		}
		a[col], a[piv] = a[piv], a[col]
		b[col], b[piv] = b[piv], b[col]
		inv := new(big.Rat).Inv(a[col][col])
		for c := 0; c < n; c++ {
			a[col][c].Mul(a[col][c], inv)
		}
		b[col].Mul(b[col], inv)
		for r := 0; r < n; r++ {
			if r == col {
				continue
			}
			f := new(big.Rat).Set(a[r][col])
			if f.Sign() == 0 {
				continue
			}
			for c := 0; c < n; c++ {
				a[r][c].Sub(a[r][c], new(big.Rat).Mul(f, a[col][c]))
			}
			b[r].Sub(b[r], new(big.Rat).Mul(f, b[col]))
		}
	}
	return b
}
