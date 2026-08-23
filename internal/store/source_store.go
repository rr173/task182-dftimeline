package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"task182-dftimeline/internal/model"
)

const timeFmt = "2006-01-02T15:04:05.000Z07:00"

// ---------------------------------------------------------------------------
// EvidenceSource
// ---------------------------------------------------------------------------

// SaveSource 插入或更新证据源。
func (s *Store) SaveSource(src *model.EvidenceSource) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`
		INSERT INTO evidence_sources (id, name, source_type, status, current_calibration_id, calibration_version, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			source_type = excluded.source_type,
			status = excluded.status,
			current_calibration_id = excluded.current_calibration_id,
			calibration_version = excluded.calibration_version,
			updated_at = excluded.updated_at`,
		src.ID, src.Name, string(src.SourceType), string(src.Status),
		src.CurrentCalibrationID, src.CalibrationVersion,
		src.CreatedAt.Format(timeFmt), src.UpdatedAt.Format(timeFmt))
	if err != nil {
		return fmt.Errorf("save source: %w", err)
	}
	return nil
}

// GetSource 按 ID 读取证据源。
func (s *Store) GetSource(id string) (*model.EvidenceSource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := s.db.QueryRow(`
		SELECT id, name, source_type, status, current_calibration_id, calibration_version, created_at, updated_at
		FROM evidence_sources WHERE id = ?`, id)
	return scanSource(row)
}

func scanSource(row *sql.Row) (*model.EvidenceSource, error) {
	var src model.EvidenceSource
	var createdAt, updatedAt string
	if err := row.Scan(&src.ID, &src.Name, &src.SourceType, &src.Status,
		&src.CurrentCalibrationID, &src.CalibrationVersion, &createdAt, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("scan source: %w", err)
	}
	src.CreatedAt = parseTime(createdAt)
	src.UpdatedAt = parseTime(updatedAt)
	return &src, nil
}

// ListSources 列出全部证据源。
func (s *Store) ListSources() ([]*model.EvidenceSource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`
		SELECT id, name, source_type, status, current_calibration_id, calibration_version, created_at, updated_at
		FROM evidence_sources ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("list sources: %w", err)
	}
	defer rows.Close()
	out := []*model.EvidenceSource{}
	for rows.Next() {
		var src model.EvidenceSource
		var createdAt, updatedAt string
		if err := rows.Scan(&src.ID, &src.Name, &src.SourceType, &src.Status,
			&src.CurrentCalibrationID, &src.CalibrationVersion, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan source row: %w", err)
		}
		src.CreatedAt = parseTime(createdAt)
		src.UpdatedAt = parseTime(updatedAt)
		out = append(out, &src)
	}
	return out, rows.Err()
}

// UpdateSourceStatus 更新源状态与时间戳（乐观校验：期望当前状态）。
func (s *Store) UpdateSourceStatus(id string, status model.SourceStatus, expect model.SourceStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.Exec(`
		UPDATE evidence_sources SET status = ?, updated_at = ?
		WHERE id = ? AND status = ?`,
		string(status), model.TimeNow().Format(timeFmt), id, string(expect))
	if err != nil {
		return fmt.Errorf("update source status: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return model.ErrInvalidState
	}
	return nil
}

// SetSourceCalibration 绑定源的生效校正（版本号同步）。
func (s *Store) SetSourceCalibration(id, calibrationID string, version int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`
		UPDATE evidence_sources SET current_calibration_id = ?, calibration_version = ?, status = ?, updated_at = ?
		WHERE id = ?`,
		calibrationID, version, string(model.SourceCalibrated), model.TimeNow().Format(timeFmt), id)
	if err != nil {
		return fmt.Errorf("bind calibration: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// ClockCalibration
// ---------------------------------------------------------------------------

// SaveCalibration 插入时钟校正（同源版本号唯一，冲突时返回 ErrConflict）。
func (s *Store) SaveCalibration(cal *model.ClockCalibration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`
		INSERT INTO clock_calibrations (id, source_id, version, tz_offset_minutes, bias_milliseconds, uncertainty_milliseconds, status, effective_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		cal.ID, cal.SourceID, cal.Version, cal.TZOffsetMinutes, cal.BiasMilliseconds,
		cal.UncertaintyMillis, string(cal.Status), cal.EffectiveAt.Format(timeFmt), cal.CreatedAt.Format(timeFmt))
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return model.ErrConflict
		}
		return fmt.Errorf("save calibration: %w", err)
	}
	return nil
}

// GetCalibration 按 ID 读取校正。
func (s *Store) GetCalibration(id string) (*model.ClockCalibration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := s.db.QueryRow(`
		SELECT id, source_id, version, tz_offset_minutes, bias_milliseconds, uncertainty_milliseconds, status, effective_at, created_at
		FROM clock_calibrations WHERE id = ?`, id)
	return scanCalibration(row)
}

func scanCalibration(row *sql.Row) (*model.ClockCalibration, error) {
	var cal model.ClockCalibration
	var effectiveAt, createdAt string
	if err := row.Scan(&cal.ID, &cal.SourceID, &cal.Version, &cal.TZOffsetMinutes,
		&cal.BiasMilliseconds, &cal.UncertaintyMillis, &cal.Status, &effectiveAt, &createdAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("scan calibration: %w", err)
	}
	cal.EffectiveAt = parseTime(effectiveAt)
	cal.CreatedAt = parseTime(createdAt)
	return &cal, nil
}

// ListCalibrationsBySource 列出某源的校正历史（按版本倒序）。
func (s *Store) ListCalibrationsBySource(sourceID string) ([]*model.ClockCalibration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`
		SELECT id, source_id, version, tz_offset_minutes, bias_milliseconds, uncertainty_milliseconds, status, effective_at, created_at
		FROM clock_calibrations WHERE source_id = ? ORDER BY version DESC`, sourceID)
	if err != nil {
		return nil, fmt.Errorf("list calibrations: %w", err)
	}
	defer rows.Close()
	out := []*model.ClockCalibration{}
	for rows.Next() {
		var cal model.ClockCalibration
		var effectiveAt, createdAt string
		if err := rows.Scan(&cal.ID, &cal.SourceID, &cal.Version, &cal.TZOffsetMinutes,
			&cal.BiasMilliseconds, &cal.UncertaintyMillis, &cal.Status, &effectiveAt, &createdAt); err != nil {
			return nil, fmt.Errorf("scan calibration row: %w", err)
		}
		cal.EffectiveAt = parseTime(effectiveAt)
		cal.CreatedAt = parseTime(createdAt)
		out = append(out, &cal)
	}
	return out, rows.Err()
}

// GetActiveCalibration 返回某源当前生效的校正；不存在则返回 ErrNotFound。
func (s *Store) GetActiveCalibration(sourceID string) (*model.ClockCalibration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := s.db.QueryRow(`
		SELECT id, source_id, version, tz_offset_minutes, bias_milliseconds, uncertainty_milliseconds, status, effective_at, created_at
		FROM clock_calibrations WHERE source_id = ? AND status = ? ORDER BY version DESC LIMIT 1`,
		sourceID, string(model.CalibrationActive))
	return scanCalibration(row)
}

// ActivateCalibration 把某校正置为 active，并把同源其他 active 置为 superseded。
// 在单事务内完成，保证任一时刻最多一个 active。
func (s *Store) ActivateCalibration(calID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin activate: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var sourceID string
	if err := tx.QueryRow(`SELECT source_id FROM clock_calibrations WHERE id = ?`, calID).Scan(&sourceID); err != nil {
		if err == sql.ErrNoRows {
			return model.ErrNotFound
		}
		return fmt.Errorf("locate calibration: %w", err)
	}
	if _, err := tx.Exec(`UPDATE clock_calibrations SET status = ? WHERE source_id = ? AND status = ?`,
		string(model.CalibrationSuperseded), sourceID, string(model.CalibrationActive)); err != nil {
		return fmt.Errorf("supersede old calibrations: %w", err)
	}
	if _, err := tx.Exec(`UPDATE clock_calibrations SET status = ? WHERE id = ?`,
		string(model.CalibrationActive), calID); err != nil {
		return fmt.Errorf("activate calibration: %w", err)
	}
	return tx.Commit()
}

// CountCalibrations 统计某源的校正条数（用于计算下一版本号）。
func (s *Store) CountCalibrations(sourceID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM clock_calibrations WHERE source_id = ?`, sourceID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count calibrations: %w", err)
	}
	return n, nil
}

// ListCalibrationVersions 返回某源全部校正（含被替代版本），供报告快照核对。
func (s *Store) ListCalibrationVersions(sourceID string) ([]int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`SELECT version FROM clock_calibrations WHERE source_id = ? ORDER BY version ASC`, sourceID)
	if err != nil {
		return nil, fmt.Errorf("list calibration versions: %w", err)
	}
	defer rows.Close()
	out := []int{}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// SnapshotClockModels 返回报告发布时刻的时钟模型快照（所有源当前 active 校正）。
func (s *Store) SnapshotClockModels() ([]model.ClockSnapshotEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`
		SELECT c.source_id, c.version, c.tz_offset_minutes, c.bias_milliseconds, c.uncertainty_milliseconds
		FROM clock_calibrations c
		JOIN evidence_sources src ON src.id = c.source_id
		WHERE c.status = ?
		ORDER BY c.source_id ASC`, string(model.CalibrationActive))
	if err != nil {
		return nil, fmt.Errorf("snapshot clock models: %w", err)
	}
	defer rows.Close()
	out := []model.ClockSnapshotEntry{}
	for rows.Next() {
		var e model.ClockSnapshotEntry
		if err := rows.Scan(&e.SourceID, &e.CalibrationVersion, &e.TZOffsetMinutes,
			&e.BiasMilliseconds, &e.UncertaintyMillis); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// parseTime 把存储的时间字符串解析为 time.Time。
func parseTime(s string) time.Time {
	t, err := time.Parse(timeFmt, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// EncodeList 把 JSON 可序列化对象编码为字符串列。
func EncodeList(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// DecodeList 把字符串列解码为 JSON 对象。
func DecodeList(s string, out any) error {
	if s == "" {
		return nil
	}
	return json.Unmarshal([]byte(s), out)
}
