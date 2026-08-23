package solver

import (
	"sort"

	"task182-dftimeline/internal/model"
)

// SortableEvents 表示可排序事件序列：拓扑序 + 按区间 earliest 稳定排序。
type SortableEvents struct {
	ArtifactIDs []string `json:"artifact_ids"`
	// Chronological 为按 earliest 升序的拓扑序（若存在环，环内工件被剔除）。
	Chronological []string `json:"chronological"`
	Cyclic        bool     `json:"cyclic"`
}

// ComputeSortable 计算可排序事件：
// 1. 对全图做 Kahn 拓扑排序，得到拓扑序；
// 2. 若无环，按区间 earliest（其次 ID）稳定排序输出时间线顺序；
// 3. 若有环，拓扑序仅覆盖无环子图，环内工件单独列入 CyclicSet。
func ComputeSortable(g *Graph) (*SortableEvents, map[string]bool) {
	order, cyclic := g.TopologicalOrder()
	cyclicSet := map[string]bool{}
	if cyclic {
		all := g.NodeIDs()
		inOrder := map[string]bool{}
		for _, id := range order {
			inOrder[id] = true
		}
		for _, id := range all {
			if !inOrder[id] {
				cyclicSet[id] = true
			}
		}
	}
	chrono := append([]string(nil), order...)
	sort.SliceStable(chrono, func(i, j int) bool {
		a, b := g.Nodes[chrono[i]], g.Nodes[chrono[j]]
		if a.EarliestMS != b.EarliestMS {
			return a.EarliestMS < b.EarliestMS
		}
		return a.ID < b.ID
	})
	return &SortableEvents{
		ArtifactIDs:   order,
		Chronological: chrono,
		Cyclic:        cyclic,
	}, cyclicSet
}

// ArtifactIntervalWide 判断工件区间是否过宽（超过阈值毫秒），用于归因判断。
const WideIntervalThresholdMS int64 = 6 * 3600 * 1000 // 6 小时

// IntervalWidth 返回工件区间宽度。
func IntervalWidth(a *model.Artifact) int64 {
	if a.LatestMS < a.EarliestMS {
		return 0
	}
	return a.LatestMS - a.EarliestMS
}

// SortByEarliest 按区间 earliest 升序排序工件。
func SortByEarliest(arts []*model.Artifact) {
	sort.SliceStable(arts, func(i, j int) bool {
		if arts[i].EarliestMS != arts[j].EarliestMS {
			return arts[i].EarliestMS < arts[j].EarliestMS
		}
		return arts[i].ID < arts[j].ID
	})
}
