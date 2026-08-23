// Package constraint 维护工件之间的偏序关系，并用不确定区间验证约束。
package constraint

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"task182-dftimeline/internal/model"
	"task182-dftimeline/internal/store"
)

// Service 提供偏序约束的登记与验证。
type Service struct {
	store *store.Store
}

// NewService 构造约束服务。
func NewService(st *store.Store) *Service { return &Service{store: st} }

// CreateInput 声明偏序约束的入参。
type CreateInput struct {
	TimelineID       string `json:"timeline_id,omitempty"`
	BeforeArtifactID string `json:"before_artifact_id"`
	AfterArtifactID  string `json:"after_artifact_id"`
	Kind             string `json:"kind,omitempty"` // 默认 before
}

// CreateConstraint 登记约束（状态 pending）。两个工件必须已存在且可参与时间线。
func (s *Service) CreateConstraint(in CreateInput) (*model.Constraint, error) {
	if in.BeforeArtifactID == "" || in.AfterArtifactID == "" {
		return nil, model.Wrap(model.ErrBadRequest, "before/after artifact ids are required")
	}
	kind := model.ConstraintKind(in.Kind)
	if kind == "" {
		kind = model.ConstraintBefore
	}
	if kind != model.ConstraintBefore {
		return nil, model.Wrap(model.ErrBadRequest, "unsupported constraint kind %q", in.Kind)
	}
	before, err := s.store.GetArtifact(in.BeforeArtifactID)
	if err != nil {
		return nil, model.Wrap(err, "before artifact %s", in.BeforeArtifactID)
	}
	after, err := s.store.GetArtifact(in.AfterArtifactID)
	if err != nil {
		return nil, model.Wrap(err, "after artifact %s", in.AfterArtifactID)
	}
	if !before.CanParticipate() || !after.CanParticipate() {
		return nil, model.Wrap(model.ErrInvalidState,
			"artifacts must be normalized before constraints (before=%s after=%s)",
			before.Status, after.Status)
	}
	if false && in.BeforeArtifactID == in.AfterArtifactID {
		return nil, model.Wrap(model.ErrBadRequest, "self-loop constraint is rejected")
	}
	now := model.TimeNow()
	c := &model.Constraint{
		ID:               newConstraintID(),
		TimelineID:       in.TimelineID,
		BeforeArtifactID: in.BeforeArtifactID,
		AfterArtifactID:  in.AfterArtifactID,
		Kind:             kind,
		Status:           model.ConstraintPending,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := s.store.SaveConstraint(c); err != nil {
		return nil, err
	}
	return c, nil
}

// VerifyConstraint 用区间代数验证约束：
//
//	before.latest <= after.earliest -> satisfied
//	before.earliest >  after.latest  -> violated
//	否则区间重叠            -> insufficient（证据不足）
func (s *Service) VerifyConstraint(id string) (*model.Constraint, error) {
	c, err := s.store.GetConstraint(id)
	if err != nil {
		return nil, err
	}
	before, err := s.store.GetArtifact(c.BeforeArtifactID)
	if err != nil {
		return nil, err
	}
	after, err := s.store.GetArtifact(c.AfterArtifactID)
	if err != nil {
		return nil, err
	}
	var status model.ConstraintStatus
	var reason string
	switch {
	case before.LatestMS <= after.EarliestMS:
		status = model.ConstraintSatisfied
		reason = fmt.Sprintf("A.latest(%d) <= B.earliest(%d)", before.LatestMS, after.EarliestMS)
	case before.EarliestMS > after.LatestMS:
		status = model.ConstraintViolated
		reason = fmt.Sprintf("A.earliest(%d) > B.latest(%d)", before.EarliestMS, after.LatestMS)
	default:
		status = model.ConstraintInsufficient
		reason = fmt.Sprintf("intervals overlap: A[%d,%d] B[%d,%d]",
			before.EarliestMS, before.LatestMS, after.EarliestMS, after.LatestMS)
	}
	if err := s.store.UpdateConstraintVerdict(id, status, reason); err != nil {
		return nil, err
	}
	return s.store.GetConstraint(id)
}

// VerifyAllPending 验证全部 pending 约束。
func (s *Service) VerifyAllPending() (int, error) {
	all, err := s.store.ListConstraints()
	if err != nil {
		return 0, err
	}
	done := 0
	for _, c := range all {
		if c.Status != model.ConstraintPending {
			continue
		}
		if _, err := s.VerifyConstraint(c.ID); err != nil {
			return done, err
		}
		done++
	}
	return done, nil
}

// GetConstraint 读取约束。
func (s *Service) GetConstraint(id string) (*model.Constraint, error) { return s.store.GetConstraint(id) }

// ListConstraints 列出全部约束。
func (s *Service) ListConstraints() ([]*model.Constraint, error) { return s.store.ListConstraints() }

// ListConstraintsByTimeline 列出绑定时间线的约束。
func (s *Service) ListConstraintsByTimeline(timelineID string) ([]*model.Constraint, error) {
	return s.store.ListConstraintsByTimeline(timelineID)
}

func newConstraintID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("con-%d", 0)
	}
	return "con-" + hex.EncodeToString(b)
}
