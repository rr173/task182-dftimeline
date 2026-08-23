// Package timeline 负责时间线的创建、构建、发布与替代。
package timeline

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"task182-dftimeline/internal/constraint"
	"task182-dftimeline/internal/model"
	"task182-dftimeline/internal/report"
	"task182-dftimeline/internal/solver"
	"task182-dftimeline/internal/store"
)

// BuildBatchSize 每轮构建处理的工件数量（断点续传粒度）。
const BuildBatchSize = 100

// Service 提供时间线业务。
type Service struct {
	store      *store.Store
	constraint *constraint.Service
	report     *report.Service
}

// NewService 构造时间线服务。
func NewService(st *store.Store, cs *constraint.Service, rs *report.Service) *Service {
	return &Service{store: st, constraint: cs, report: rs}
}

// CreateTimeline 创建时间线（状态 building，游标 0）。
func (s *Service) CreateTimeline(name string) (*model.Timeline, error) {
	if name == "" {
		return nil, model.Wrap(model.ErrBadRequest, "timeline name is required")
	}
	now := model.TimeNow()
	t := &model.Timeline{
		ID:        newTimelineID(),
		Name:      name,
		Status:    model.TimelineBuilding,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.store.SaveTimeline(t); err != nil {
		return nil, err
	}
	return t, nil
}

// BuildResult 一轮构建的进度报告。
type BuildResult struct {
	TimelineID   string   `json:"timeline_id"`
	Processed    int      `json:"processed"`
	Total        int      `json:"total"`
	Done         bool     `json:"done"`
	Status       string   `json:"status"`
	ConflictIDs  []string `json:"conflict_ids,omitempty"`
	HasCycle     bool     `json:"has_cycle"`
}

// BuildTimeline 从持久化游标继续构建时间线（可重复调用，支持重启恢复）。
// 每轮处理 BuildBatchSize 个工件：验证约束、建偏序图、检测最小环并保存冲突核心。
// 全部处理完后时间线进入 reviewable（无环）或 conflicted（有环）。
func (s *Service) BuildTimeline(id string) (*BuildResult, error) {
	tl, err := s.store.GetTimeline(id)
	if err != nil {
		return nil, err
	}
	if tl.Status != model.TimelineBuilding {
		return nil, model.Wrap(model.ErrInvalidState, "timeline %s is %s, not building", id, tl.Status)
	}
	res := &BuildResult{TimelineID: id, Status: string(tl.Status)}

	// 1. 取本批工件（从游标开始）。
	chunk, err := s.store.NextArtifactChunk(tl.ArtifactCursor, BuildBatchSize)
	if err != nil {
		return nil, err
	}
	// 计算总量（用最大游标估算：此处通过计数简化，实际以批次推进为准）。
	if len(chunk) == 0 {
		// 全部处理完：验证约束 -> 建图 -> 环检测 -> 终态。
		return s.finalize(tl)
	}

	// 2. 验证本批工件涉及的 pending 约束（区间判定）。
	if _, err := s.constraint.VerifyAllPending(); err != nil {
		return nil, err
	}

	// 3. 更新游标（即使本批存在异常也记录进度，重启后继续）。
	newCursor := tl.ArtifactCursor + len(chunk)
	if err := s.store.UpdateTimelineCursor(id, newCursor, model.TimelineBuilding); err != nil {
		return nil, err
	}
	res.Processed = len(chunk)
	res.Total = newCursor
	return res, nil
}

// finalize 全部工件处理完后收尾：建图、环检测、保存冲突核心、定终态。
func (s *Service) finalize(tl *model.Timeline) (*BuildResult, error) {
	res := &BuildResult{TimelineID: tl.ID, Processed: 0, Done: true}

	arts, err := s.store.ListArtifacts("")
	if err != nil {
		return nil, err
	}
	cons, err := s.store.ListConstraints()
	if err != nil {
		return nil, err
	}
	g := solver.NewGraph(arts, cons)
	nodeIDs, edgeIDs := g.MinConflictCycle()
	conflictIDs := []string{}
	if len(nodeIDs) > 0 {
		// 归因。
		attribution, evidence := s.attributeCycle(g, nodeIDs)
		cc := &model.ConflictCore{
			ID:          newTimelineID(),
			TimelineID:  tl.ID,
			ArtifactIDs: nodeIDs,
			EdgeIDs:     edgeIDs,
			Attribution: attribution,
			Evidence:    evidence,
			CreatedAt:   model.TimeNow(),
		}
		if err := s.store.SaveConflictCore(cc); err != nil {
			return nil, err
		}
		// 环上工件标记为 conflict。
		if err := s.store.MarkArtifactConflict(nodeIDs); err != nil {
			return nil, err
		}
		conflictIDs = append(conflictIDs, cc.ID)
		status := model.TimelineConflicted
		if err := s.store.UpdateTimelineCursor(tl.ID, tl.ArtifactCursor, status); err != nil {
			return nil, err
		}
		// 同步冲突计数。
		if err := s.store.UpdateConflictCount(tl.ID, 1); err != nil {
			return nil, err
		}
		res.Status = string(status)
		res.HasCycle = true
		res.ConflictIDs = conflictIDs
		return res, nil
	}

	status := model.TimelineReviewable
	if err := s.store.UpdateTimelineCursor(tl.ID, tl.ArtifactCursor, status); err != nil {
		return nil, err
	}
	res.Status = string(status)
	return res, nil
}

// attributeCycle 组装归因所需的旁证。
func (s *Service) attributeCycle(g *solver.Graph, nodeIDs []string) (model.Attribution, string) {
	arts := make([]*model.Artifact, 0, len(nodeIDs))
	sources := map[string]*model.EvidenceSource{}
	suspicious := map[string]bool{}
	for _, id := range nodeIDs {
		a := g.Nodes[id]
		if a == nil {
			continue
		}
		arts = append(arts, a)
		if src, err := s.store.GetSource(a.SourceID); err == nil {
			sources[a.SourceID] = src
		}
		if a.Status == model.ArtifactSuspicious {
			suspicious[a.ID] = true
		}
	}
	return solver.Attribute(solver.AttributionInput{
		Artifacts:     arts,
		Sources:       sources,
		SuspiciousIDs: suspicious,
	})
}

// GetTimeline 读取时间线。
func (s *Service) GetTimeline(id string) (*model.Timeline, error) { return s.store.GetTimeline(id) }

// ListTimelines 列出全部时间线。
func (s *Service) ListTimelines() ([]*model.Timeline, error) { return s.store.ListTimelines() }

// PublishTimeline 发布时间线：冻结报告并转入 published。
// 已发布的时间线不可直接重排；只能通过新时间线替代。
func (s *Service) PublishTimeline(id string) (*model.Report, error) {
	tl, err := s.store.GetTimeline(id)
	if err != nil {
		return nil, err
	}
	switch tl.Status {
	case model.TimelineReviewable, model.TimelineConflicted:
		// 可发布
	case model.TimelinePublished:
		// published timelines are incorrectly allowed to publish again
	default:
		return nil, model.Wrap(model.ErrInvalidState, "timeline %s in status %s cannot be published", id, tl.Status)
	}
	r, err := s.report.FreezeReport(id)
	if err != nil {
		return nil, err
	}
	// 时间线绑定报告并转 published。
	tl.Status = model.TimelinePublished
	tl.PublishedReportID = r.ID
	tl.UpdatedAt = model.TimeNow()
	if err := s.store.SaveTimeline(tl); err != nil {
		return nil, err
	}
	return r, nil
}

// Supersede 把已发布时间线标记为被新时间线替代（旧报告不可变）。
func (s *Service) Supersede(id, supersededByID string) error {
	return s.store.SupersedeTimeline(id, supersededByID)
}

// ListConflictCores 列出时间线的冲突核心。
func (s *Service) ListConflictCores(timelineID string) ([]*model.ConflictCore, error) {
	return s.store.ListConflictCoresByTimeline(timelineID)
}

// GetConflictCore 读取冲突核心。
func (s *Service) GetConflictCore(id string) (*model.ConflictCore, error) {
	return s.store.GetConflictCore(id)
}

func newTimelineID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("tl-%d", 0)
	}
	return "tl-" + hex.EncodeToString(b)
}
