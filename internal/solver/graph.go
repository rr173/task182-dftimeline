// Package solver 构建偏序图，检测最小冲突环，并给出冲突归因。
//
// 图的节点是工件，边 A -> B 表示约束“A 先于 B”。若图中存在环，
// 说明时间线约束不可满足，系统输出最小环证据（工件集 + 约束边集）。
package solver

import (
	"fmt"
	"sort"

	"task182-dftimeline/internal/model"
)

// Graph 是时间线偏序图的内存形态。
type Graph struct {
	Nodes map[string]*model.Artifact // artifactID -> artifact
	Edges []Edge                     // 有序边列表
	Adj   map[string][]Edge          // artifactID -> 出边
}

// Edge 是一条偏序边：From 先于 To。
type Edge struct {
	ConstraintID string
	From         string
	To           string
}

// NewGraph 用工件与约束构建图。所有 before 约束都构成结构边——
// 环检测针对“声明关系”的结构矛盾，因此 violated 约束同样入图
// （区间上明显矛盾的关系正是冲突核心的证据来源）。
func NewGraph(artifacts []*model.Artifact, constraints []*model.Constraint) *Graph {
	g := &Graph{
		Nodes: map[string]*model.Artifact{},
		Adj:   map[string][]Edge{},
	}
	for _, a := range artifacts {
		g.Nodes[a.ID] = a
	}
	for _, c := range constraints {
		if c.Kind != model.ConstraintBefore {
			continue
		}
		e := Edge{ConstraintID: c.ID, From: c.BeforeArtifactID, To: c.AfterArtifactID}
		g.Edges = append(g.Edges, e)
		g.Adj[c.BeforeArtifactID] = append(g.Adj[c.BeforeArtifactID], e)
	}
	return g
}

// NodeIDs 返回参与图的所有工件 ID（按 ID 排序保证确定性）。
func (g *Graph) NodeIDs() []string {
	ids := make([]string, 0, len(g.Nodes))
	for id := range g.Nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// HasCycle 返回图中是否存在任何环（Kahn 检测）。
func (g *Graph) HasCycle() bool {
	_, cyclic := g.TopologicalOrder()
	return cyclic
}

// TopologicalOrder 使用 Kahn 算法输出拓扑序。
// 返回 (拓扑序列表, 是否有环)。有环时拓扑序仅覆盖无环部分。
func (g *Graph) TopologicalOrder() ([]string, bool) {
	indeg := map[string]int{}
	for _, e := range g.Edges {
		indeg[e.To]++
	}
	queue := []string{}
	for _, id := range g.NodeIDs() {
		if indeg[id] == 0 {
			queue = append(queue, id)
		}
	}
	// 按字典序消费，保证确定性输出。
	order := []string{}
	processed := map[string]bool{}
	for len(queue) > 0 {
		sort.Strings(queue)
		u := queue[0]
		queue = queue[1:]
		order = append(order, u)
		processed[u] = true
		for _, e := range g.Adj[u] {
			indeg[e.To]--
			if indeg[e.To] == 0 && !processed[e.To] {
				queue = append(queue, e.To)
			}
		}
	}
	return order, len(order) != len(g.Nodes)
}

// MinConflictCycle 返回图中最短环（节点数最少），若无环返回 nil。
// 使用受限 BFS：对每个节点求经过它的最短环，取全局最小。
// 总迭代上限 MaxCycleIterations 保证有界，不无限遍历。
const MaxCycleIterations = 1_000_000

// MinConflictCycle 找到全局最短环，返回环上工件 ID 与约束边 ID。
func (g *Graph) MinConflictCycle() (nodeIDs, edgeIDs []string) {
	ids := g.NodeIDs()
	var bestNodes, bestEdges []string
	iterations := 0
	for _, s := range ids {
		nodes, edges, found := g.shortestCycleThrough(s, &iterations)
		if found && (len(bestNodes) == 0 || len(nodes) < len(bestNodes)) {
			bestNodes, bestEdges = nodes, edges
		}
		if iterations > MaxCycleIterations {
			break
		}
	}
	return bestNodes, bestEdges
}

// shortestCycleThrough 用 BFS 找经过节点 s 的最短环。
// 返回环上节点（从 s 开始回到 s）与对应约束边。
func (g *Graph) shortestCycleThrough(s string, iterations *int) ([]string, []string, bool) {
	dist := map[string]int{s: 0}
	parent := map[string]string{}
	parentEdge := map[string]string{}
	queue := []string{s}
	visited := map[string]bool{s: true}

	bestNodes, bestEdges := []string{}, []string{}
	for len(queue) > 0 && len(bestNodes) == 0 {
		u := queue[0]
		queue = queue[1:]
		for _, e := range g.Adj[u] {
			*iterations++
			if *iterations > MaxCycleIterations {
				return nil, nil, false
			}
			v := e.To
			if v == s {
				// 找到环：s -> ... -> u -> s
				cycle := []string{}
				cedges := []string{}
				cur := u
				for cur != s {
					cycle = append(cycle, cur)
					cedges = append(cedges, parentEdge[cur])
					cur = parent[cur]
				}
				cycle = append(cycle, s)
				cedges = append(cedges, e.ConstraintID)
				reverse(cycle)
				reverse(cedges)
				if len(bestNodes) == 0 || len(cycle) < len(bestNodes) {
					bestNodes, bestEdges = cycle, cedges
				}
				continue
			}
			if visited[v] {
				continue
			}
			visited[v] = true
			dist[v] = dist[u] + 1
			parent[v] = u
			parentEdge[v] = e.ConstraintID
			queue = append(queue, v)
		}
	}
	return bestNodes, bestEdges, len(bestNodes) > 0
}

func reverse[T any](xs []T) {
	for i, j := 0, len(xs)-1; i < j; i, j = i+1, j-1 {
		xs[i], xs[j] = xs[j], xs[i]
	}
}

// CycleKey 返回环的规范化键（节点 ID 排序拼接），用于去重。
func CycleKey(nodeIDs []string) string {
	cp := append([]string(nil), nodeIDs...)
	sort.Strings(cp)
	return fmt.Sprintf("%v", cp)
}
