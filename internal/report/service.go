// Package report 负责冲突裁决与时间线报告的冻结发布。
package report

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"task182-dftimeline/internal/model"
	"task182-dftimeline/internal/store"
)

// Service 提供裁决与报告业务。
type Service struct {
	store *store.Store
}

// NewService 构造报告服务。
func NewService(st *store.Store) *Service { return &Service{store: st} }

// AdjudicateInput 提交裁决的入参。
type AdjudicateInput struct {
	ArtifactID string                   `json:"artifact_id"`
	Action     model.AdjudicationAction `json:"action"`
	Reason     string                   `json:"reason"`
}

// Adjudicate 为时间线内的冲突工件提交裁决。裁决动作会同步工件/源状态：
//
//	adopt             -> 工件 adopted
//	mark_suspicious   -> 工件 suspicious
//	isolate_source    -> 工件所在源隔离
//	correct_clock     -> 仅记录裁决，由后续时钟校正闭环完成
func (s *Service) Adjudicate(timelineID string, in AdjudicateInput) (*model.Adjudication, error) {
	art, err := s.store.GetArtifact(in.ArtifactID)
	if err != nil {
		return nil, err
	}
	if in.Reason == "" {
		return nil, model.Wrap(model.ErrBadRequest, "adjudication reason is required")
	}
	switch in.Action {
	case model.AdjudicationAdopt:
		if err := s.store.UpdateArtifactStatus(art.ID, model.ArtifactAdopted, model.ArtifactStatus(art.Status)); err != nil {
			return nil, err
		}
	case model.AdjudicationSuspicious:
		if err := s.store.UpdateArtifactStatus(art.ID, model.ArtifactSuspicious, model.ArtifactStatus(art.Status)); err != nil {
			return nil, err
		}
	case model.AdjudicationIsolate:
		src, err := s.store.GetSource(art.SourceID)
		if err != nil {
			return nil, err
		}
		if err := s.store.UpdateSourceStatus(art.SourceID, model.SourceIsolated, src.Status); err != nil {
			return nil, err
		}
	case model.AdjudicationCorrectClock:
		// 记录意图；具体校正通过 POST /api/sources/{id}/calibrations 闭环。
	default:
		return nil, model.Wrap(model.ErrBadRequest, "unsupported adjudication action %q", in.Action)
	}
	ad := &model.Adjudication{
		ID:            newReportID(),
		TimelineID:    timelineID,
		ArtifactID:    in.ArtifactID,
		Action:        in.Action,
		Reason:        in.Reason,
		AdjudicatedAt: model.TimeNow(),
	}
	if err := s.store.SaveAdjudication(ad); err != nil {
		return nil, err
	}
	return ad, nil
}

// ListAdjudications 列出时间线的裁决。
func (s *Service) ListAdjudications(timelineID string) ([]*model.Adjudication, error) {
	return s.store.ListAdjudicationsByTimeline(timelineID)
}

// FreezeReport 冻结时间线的发布报告。报告快照包含：
//   - 时钟模型快照（发布时刻各源生效校正版本，之后更新只产生替代报告）；
//   - 工件快照（参与时间线的工件归一化区间）；
//   - 冲突摘要与裁决摘要。
//
// 冻结后的报告不可修改（ErrReportImmutable 由服务层保证不更新）。
func (s *Service) FreezeReport(timelineID string) (*model.Report, error) {
	version, err := s.store.NextReportVersion(timelineID)
	if err != nil {
		return nil, err
	}
	clockSnap, err := s.store.SnapshotClockModels()
	if err != nil {
		return nil, err
	}
	arts, err := s.store.ListArtifacts("")
	if err != nil {
		return nil, err
	}
	artifactSnap := make([]model.ArtifactSnapshotEntry, 0, len(arts))
	for _, a := range arts {
		artifactSnap = append(artifactSnap, model.ArtifactSnapshotEntry{
			ArtifactID: a.ID,
			PathName:   a.PathName,
			EarliestMS: a.EarliestMS,
			LatestMS:   a.LatestMS,
			Status:     string(a.Status),
		})
	}
	cores, err := s.store.ListConflictCoresByTimeline(timelineID)
	if err != nil {
		return nil, err
	}
	conflictSum := make([]model.ConflictSummaryEntry, 0, len(cores))
	for _, c := range cores {
		conflictSum = append(conflictSum, model.ConflictSummaryEntry{
			ConflictID:  c.ID,
			ArtifactIDs: c.ArtifactIDs,
			Attribution: string(c.Attribution),
		})
	}
	adjs, err := s.store.ListAdjudicationsByTimeline(timelineID)
	if err != nil {
		return nil, err
	}
	adjSum := make([]model.AdjudicationSummaryEntry, 0, len(adjs))
	for _, a := range adjs {
		adjSum = append(adjSum, model.AdjudicationSummaryEntry{
			AdjudicationID: a.ID,
			ArtifactID:     a.ArtifactID,
			Action:         string(a.Action),
			Reason:         a.Reason,
		})
	}
	now := model.TimeNow()
	r := &model.Report{
		ID:                  newReportID(),
		TimelineID:          timelineID,
		Version:             version,
		Status:              model.ReportFrozen,
		ClockSnapshot:       clockSnap,
		ArtifactSnapshot:    artifactSnap,
		ConflictSummary:     conflictSum,
		AdjudicationSummary: adjSum,
		PublishedAt:         now,
		CreatedAt:           now,
	}
	if err := s.store.SaveReport(r); err != nil {
		return nil, err
	}
	return r, nil
}

// GetReport 读取报告。
func (s *Service) GetReport(id string) (*model.Report, error) { return s.store.GetReport(id) }

// ListReports 列出全部报告。
func (s *Service) ListReports() ([]*model.Report, error) { return s.store.ListReports() }

// ListReportsByTimeline 列出时间线的报告。
func (s *Service) ListReportsByTimeline(timelineID string) ([]*model.Report, error) {
	return s.store.ListReportsByTimeline(timelineID)
}

func newReportID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("rpt-%d", 0)
	}
	return "rpt-" + hex.EncodeToString(b)
}
