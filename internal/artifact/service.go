// Package artifact 负责工件登记、内容哈希与时间区间归一化。
//
// 核心算法：把工件本地时间戳 + 来源时钟模型换算为不确定区间
// [earliest, latest]，供偏序约束与时间线求解使用。
package artifact

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"task182-dftimeline/internal/model"
	"task182-dftimeline/internal/store"
)

// UnknownOffsetUncertaintyMS 是源处于“偏差未知”时的默认不确定度（7 天）。
// 偏差未知意味着真实时刻无法精确定位，区间必须足够宽以保持保守。
const UnknownOffsetUncertaintyMS int64 = 7 * 24 * 3600 * 1000

// Service 提供工件业务操作。
type Service struct {
	store *store.Store
}

// NewService 构造工件服务。
func NewService(st *store.Store) *Service { return &Service{store: st} }

// RegisterInput 登记工件的入参。
type RegisterInput struct {
	SourceID     string           `json:"source_id"`
	ArtifactType model.ArtifactType `json:"artifact_type"`
	PathName     string           `json:"path_name"`
	RawTimestamp string           `json:"raw_timestamp"` // RFC3339，必须带时区偏移
	Content      string           `json:"content"`       // 用于计算 SHA-256
	Evidence     string           `json:"evidence"`
}

// RegisterArtifact 登记工件：校验源状态与时间戳格式，计算内容哈希，
// 执行同哈希幂等/冲突检查，随后立即归一化为不确定区间。
func (s *Service) RegisterArtifact(in RegisterInput) (*model.Artifact, bool, error) {
	src, err := s.store.GetSource(in.SourceID)
	if err != nil {
		return nil, false, err
	}
	if src.Status == model.SourceIsolated {
		return nil, false, model.Wrap(model.ErrConflict, "source %s is isolated", in.SourceID)
	}
	if in.PathName == "" {
		return nil, false, model.Wrap(model.ErrBadRequest, "path_name is required")
	}
	// 拒绝无时区的时间戳：必须带 ±HH:MM 偏移或 Z。
	rawMS, tzOffsetMin, err := parseTimestampWithZone(in.RawTimestamp)
	if err != nil {
		return nil, false, err
	}
	hash := ContentSHA256([]byte(in.Content))

	// 同哈希幂等与内容一致性检查。
	if existing, err := s.store.GetArtifactByHash(hash); err == nil {
		if existing.SourceID == in.SourceID && existing.RawTimestamp == in.RawTimestamp {
			return existing, true, nil // 完全一致 -> 幂等返回
		}
		return nil, false, model.Wrap(model.ErrHashMismatch,
			"artifact hash %s already registered with different content", shortHash(hash))
	} else if err != model.ErrNotFound {
		return nil, false, err
	}

	now := model.TimeNow()
	art := &model.Artifact{
		ID:            newArtifactID(),
		SourceID:      in.SourceID,
		ArtifactType:  in.ArtifactType,
		PathName:      in.PathName,
		RawTimestamp:  in.RawTimestamp,
		RawTZOffsetMin: tzOffsetMin,
		ContentSHA256: hash,
		Status:        model.ArtifactPending,
		Evidence:      in.Evidence,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	// rawMS 仅用于校验，未持久化原始毫秒；区间由归一化计算。
	_ = rawMS
	if err := s.store.SaveArtifact(art); err != nil {
		return nil, false, err
	}
	if err := s.NormalizeArtifact(art.ID); err != nil {
		return nil, false, err
	}
	normalized, err := s.store.GetArtifact(art.ID)
	return normalized, false, err}

// NormalizeArtifact 把 pending 工件归一化为不确定区间。
// 使用源当前 active 校准；源偏差未知时采用最大不确定度。
func (s *Service) NormalizeArtifact(id string) error {
	art, err := s.store.GetArtifact(id)
	if err != nil {
		return err
	}
	if art.Status != model.ArtifactPending {
		return model.Wrap(model.ErrInvalidState, "artifact %s is not pending", id)
	}
	src, err := s.store.GetSource(art.SourceID)
	if err != nil {
		return err
	}
	rawMS, _, err := parseTimestampWithZone(art.RawTimestamp)
	if err != nil {
		return err
	}
	var bias, uncertainty int64
	var offsetMinutes int
	var note string
	switch src.Status {
	case model.SourceCalibrated:
		cal, err := s.store.GetActiveCalibration(art.SourceID)
		if err != nil {
			if err == model.ErrNotFound {
				return model.Wrap(model.ErrInvalidState, "source %s marked calibrated but has no active calibration", art.SourceID)
			}
			return err
		}
		bias, uncertainty = cal.BiasMilliseconds, cal.UncertaintyMillis
		offsetMinutes = cal.TZOffsetMinutes
		note = fmt.Sprintf("clock model v%d (bias=%dms, uncertainty=%dms, tz=%dmin)",
			cal.Version, cal.BiasMilliseconds, cal.UncertaintyMillis, cal.TZOffsetMinutes)
	case model.SourceUnknownOffset:
		bias, uncertainty = 0, UnknownOffsetUncertaintyMS
		offsetMinutes = art.RawTZOffsetMin
		note = "clock offset unknown; conservative max uncertainty applied"
	default:
		return model.Wrap(model.ErrInvalidState, "source %s in status %s cannot produce timestamps", art.SourceID, src.Status)
	}

	// adjusted = raw(UTC ms) + tz_offset*60000 - bias
	adjusted := rawMS + int64(offsetMinutes)*60_000 + bias
	earliest, latest := adjusted-uncertainty, adjusted+uncertainty
	if earliest > latest {
		return model.Wrap(model.ErrIntervalReversed, "artifact %s interval [%d, %d]", id, earliest, latest)
	}
	if art.Evidence != "" {
		note = art.Evidence + "; " + note
	}
	return s.store.UpdateArtifactNormalized(id, earliest, latest, note)
}

// NormalizeAllPending 归一化全部 pending 工件（幂等；供启动恢复与批量操作）。
func (s *Service) NormalizeAllPending() (int, error) {
	arts, err := s.store.ListArtifacts(string(model.ArtifactPending))
	if err != nil {
		return 0, err
	}
	done := 0
	for _, a := range arts {
		if err := s.NormalizeArtifact(a.ID); err != nil {
			return done, err
		}
		done++
	}
	return done, nil
}

// MarkSuspicious 标记工件可疑（normalized -> suspicious）。
func (s *Service) MarkSuspicious(id string) (*model.Artifact, error) {
	art, err := s.store.GetArtifact(id)
	if err != nil {
		return nil, err
	}
	expect := model.ArtifactStatus(art.Status)
	if expect == model.ArtifactAdopted {
		return nil, model.Wrap(model.ErrInvalidState, "adopted artifact %s cannot be marked suspicious", id)
	}
	if err := s.store.UpdateArtifactStatus(id, model.ArtifactSuspicious, expect); err != nil {
		return nil, err
	}
	return s.store.GetArtifact(id)
}

// AdoptArtifact 采纳工件（normalized/suspicious/conflict -> adopted）。
func (s *Service) AdoptArtifact(id string) (*model.Artifact, error) {
	art, err := s.store.GetArtifact(id)
	if err != nil {
		return nil, err
	}
	if err := s.store.UpdateArtifactStatus(id, model.ArtifactAdopted, model.ArtifactStatus(art.Status)); err != nil {
		return nil, err
	}
	return s.store.GetArtifact(id)
}

// GetArtifact 读取工件。
func (s *Service) GetArtifact(id string) (*model.Artifact, error) { return s.store.GetArtifact(id) }

// ListArtifacts 列出工件（可选按状态过滤）。
func (s *Service) ListArtifacts(status string) ([]*model.Artifact, error) {
	return s.store.ListArtifacts(status)
}

// ListArtifactsBySource 列出某源的工件。
func (s *Service) ListArtifactsBySource(sourceID string) ([]*model.Artifact, error) {
	return s.store.ListArtifactsBySource(sourceID)
}

// ---------------------------------------------------------------------------
// 时间与哈希工具
// ---------------------------------------------------------------------------

// parseTimestampWithZone 解析 RFC3339 时间戳，要求必须携带时区偏移（±HH:MM 或 Z）。
// 返回 Unix 毫秒与分钟级时区偏移。
func parseTimestampWithZone(raw string) (unixMS int64, tzOffsetMinutes int, err error) {
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return 0, 0, model.Wrap(model.ErrTimeZoneMissing, "invalid or missing timezone in %q", raw)
	}
	_, offsetSec := t.Zone()
	return t.UnixMilli(), offsetSec / 60, nil
}

// ContentSHA256 计算内容的 SHA-256 摘要。
func ContentSHA256(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func shortHash(h string) string {
	if len(h) > 12 {
		return h[:12]
	}
	return h
}

func newArtifactID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("art-%d", time.Now().UnixNano())
	}
	return "art-" + hex.EncodeToString(b)
}
