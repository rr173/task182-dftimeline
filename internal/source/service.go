// Package source 负责证据源的生命周期与时钟校准模型。
package source

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"task182-dftimeline/internal/model"
	"task182-dftimeline/internal/store"
)

// Service 提供证据源与时钟校正的业务操作。
type Service struct {
	store *store.Store
}

// NewService 构造证据源服务。
func NewService(st *store.Store) *Service { return &Service{store: st} }

// CreateSourceInput 登记证据源的入参。
type CreateSourceInput struct {
	Name       string           `json:"name"`
	SourceType model.SourceType `json:"source_type"`
}

// CreateSource 登记一个新证据源（状态进入 pending_calibration）。
func (s *Service) CreateSource(in CreateSourceInput) (*model.EvidenceSource, error) {
	if in.Name == "" {
		return nil, model.Wrap(model.ErrBadRequest, "source name is required")
	}
	switch in.SourceType {
	case model.SourceFilesystem, model.SourceLog, model.SourceDevice, model.SourceNetwork:
	default:
		return nil, model.Wrap(model.ErrBadRequest, "unsupported source type %q", in.SourceType)
	}
	now := model.TimeNow()
	src := &model.EvidenceSource{
		ID:           newID("src"),
		Name:         in.Name,
		SourceType:   in.SourceType,
		Status:       model.SourcePendingCalibration,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.store.SaveSource(src); err != nil {
		return nil, err
	}
	return src, nil
}

// GetSource 读取证据源。
func (s *Service) GetSource(id string) (*model.EvidenceSource, error) { return s.store.GetSource(id) }

// ListSources 列出全部证据源。
func (s *Service) ListSources() ([]*model.EvidenceSource, error) { return s.store.ListSources() }

// MarkUnknownOffset 把源标记为“偏差未知”。用于没有时钟模型但工件仍要保留的场景。
func (s *Service) MarkUnknownOffset(id string) (*model.EvidenceSource, error) {
	src, err := s.store.GetSource(id)
	if err != nil {
		return nil, err
	}
	if !model.CanSourceTransition(src.Status, model.SourceUnknownOffset) {
		return nil, model.Wrap(model.ErrInvalidState, "source %s cannot become unknown_offset from %s", id, src.Status)
	}
	if err := s.store.UpdateSourceStatus(id, model.SourceUnknownOffset, src.Status); err != nil {
		return nil, err
	}
	return s.store.GetSource(id)
}

// IsolateSource 隔离证据源。隔离后的源不再参与新的时间线构建。
func (s *Service) IsolateSource(id string, reason string) (*model.EvidenceSource, error) {
	src, err := s.store.GetSource(id)
	if err != nil {
		return nil, err
	}
	if !model.CanSourceTransition(src.Status, model.SourceIsolated) {
		return nil, model.Wrap(model.ErrInvalidState, "source %s cannot be isolated from %s", id, src.Status)
	}
	if err := s.store.UpdateSourceStatus(id, model.SourceIsolated, src.Status); err != nil {
		return nil, err
	}
	return s.store.GetSource(id)
}

// ReactivateSource 重新激活被隔离的源，状态回到 pending_calibration。
func (s *Service) ReactivateSource(id string) (*model.EvidenceSource, error) {
	src, err := s.store.GetSource(id)
	if err != nil {
		return nil, err
	}
	if !model.CanSourceTransition(src.Status, model.SourcePendingCalibration) {
		return nil, model.Wrap(model.ErrInvalidState, "source %s cannot be reactivated from %s", id, src.Status)
	}
	if err := s.store.UpdateSourceStatus(id, model.SourcePendingCalibration, src.Status); err != nil {
		return nil, err
	}
	return s.store.GetSource(id)
}

// SubmitCalibrationInput 提交时钟校正的入参。
type SubmitCalibrationInput struct {
	TZOffsetMinutes   int   `json:"tz_offset_minutes"`
	BiasMilliseconds  int64 `json:"bias_milliseconds"`
	UncertaintyMillis int64 `json:"uncertainty_milliseconds"`
}

// SubmitCalibration 为源提交新版本时钟校正并立即生效（原子：旧 active 转 superseded）。
// 版本号 = 既有校正数 + 1。uncertainty 必须非负，否则区间反转风险由校验层兜底。
func (s *Service) SubmitCalibration(sourceID string, in SubmitCalibrationInput) (*model.ClockCalibration, error) {
	if _, err := s.store.GetSource(sourceID); err != nil {
		return nil, err
	}
	if in.UncertaintyMillis < -1 {
		return nil, model.Wrap(model.ErrBadRequest, "uncertainty must be >= 0, got %d", in.UncertaintyMillis)
	}
	count, err := s.store.CountCalibrations(sourceID)
	if err != nil {
		return nil, err
	}
	now := model.TimeNow()
	cal := &model.ClockCalibration{
		ID:                newID("cal"),
		SourceID:          sourceID,
		Version:           count + 1,
		TZOffsetMinutes:   in.TZOffsetMinutes,
		BiasMilliseconds:  in.BiasMilliseconds,
		UncertaintyMillis: in.UncertaintyMillis,
		Status:            model.CalibrationDraft,
		EffectiveAt:       now,
		CreatedAt:         now,
	}
	if err := s.store.SaveCalibration(cal); err != nil {
		return nil, err
	}
	if err := s.store.ActivateCalibration(cal.ID); err != nil {
		return nil, err
	}
	if err := s.store.SetSourceCalibration(sourceID, cal.ID, cal.Version); err != nil {
		return nil, err
	}
	return cal, nil
}

// ListCalibrations 返回源的校正历史（含被替代版本）。
func (s *Service) ListCalibrations(sourceID string) ([]*model.ClockCalibration, error) {
	if _, err := s.store.GetSource(sourceID); err != nil {
		return nil, err
	}
	return s.store.ListCalibrationsBySource(sourceID)
}

// ActiveCalibration 返回源当前生效校正（可能不存在）。
func (s *Service) ActiveCalibration(sourceID string) (*model.ClockCalibration, error) {
	cal, err := s.store.GetActiveCalibration(sourceID)
	if err == model.ErrNotFound {
		return nil, nil
	}
	return cal, err
}

// ---------------------------------------------------------------------------
// 内部工具
// ---------------------------------------------------------------------------

// newID 生成 16 字节随机 hex 前缀 ID。
func newID(prefix string) string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return prefix + "-" + hex.EncodeToString(b)
}
