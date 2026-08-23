package solver

import (
	"fmt"
	"strings"

	"task182-dftimeline/internal/model"
)

// AttributionInput 归因所需的旁证信息。
type AttributionInput struct {
	Artifacts     []*model.Artifact                 // 环上工件
	Sources       map[string]*model.EvidenceSource  // 源快照
	SuspiciousIDs map[string]bool                   // 被标记可疑的工件
}

// Attribute 对冲突环给出归因。判定顺序（优先级从高到低）：
//
//  1. metadata_tamper：环上存在被标记可疑的工件或隔离源工件（内容/元数据异常）；
//  2. clock_skew：环上存在偏差未知的源（时钟模型缺失导致区间错位）；
//  3. insufficient_evidence：环上所有区间都很宽，说明是证据精度不足而非硬矛盾；
//  4. 兜底 clock_skew：多源时钟未对齐的典型取证场景。
//
// 返回主归因 + 人类可读证据说明。
func Attribute(in AttributionInput) (model.Attribution, string) {
	evidence := []string{}
	var wideCount int
	for _, a := range in.Artifacts {
		if in.SuspiciousIDs[a.ID] {
			evidence = append(evidence, fmt.Sprintf("artifact %s is marked suspicious", a.ID))
		}
		if src, ok := in.Sources[a.SourceID]; ok && src.Status == model.SourceUnknownOffset {
			evidence = append(evidence, fmt.Sprintf("artifact %s source %s offset unknown", a.ID, a.SourceID))
		}
		if src, ok := in.Sources[a.SourceID]; ok && src.Status == model.SourceIsolated {
			evidence = append(evidence, fmt.Sprintf("artifact %s source %s isolated", a.ID, a.SourceID))
		}
		if IntervalWidth(a) >= WideIntervalThresholdMS {
			wideCount++
		}
	}

	switch {
	case len(evidence) > 0 && strings.Contains(strings.Join(evidence, "; "), "suspicious"):
		return model.AttributionMetadataTamper, strings.Join(evidence, "; ")
	case len(evidence) > 0 && strings.Contains(strings.Join(evidence, "; "), "offset unknown"):
		return model.AttributionClockSkew, strings.Join(evidence, "; ")
	case len(evidence) > 0 && strings.Contains(strings.Join(evidence, "; "), "isolated"):
		return model.AttributionClockSkew, strings.Join(evidence, "; ")
	case wideCount == len(in.Artifacts) && len(in.Artifacts) > 0:
		return model.AttributionInsufficient, fmt.Sprintf(
			"all %d artifacts in cycle have wide intervals (>= %d ms); precision insufficient",
			len(in.Artifacts), WideIntervalThresholdMS)
	default:
		return model.AttributionClockSkew, "clock skew across sources produces unsatisfiable ordering"
	}
}

// DescribeCycle 生成环的人类可读描述（A -> B -> C -> A）。
func DescribeCycle(nodeIDs []string) string {
	if len(nodeIDs) == 0 {
		return "(empty cycle)"
	}
	parts := make([]string, 0, len(nodeIDs)+1)
	for _, id := range nodeIDs {
		parts = append(parts, id)
	}
	parts = append(parts, nodeIDs[0])
	return strings.Join(parts, " -> ")
}
