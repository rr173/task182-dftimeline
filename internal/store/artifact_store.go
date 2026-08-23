package store

import (
	"database/sql"
	"fmt"
	"strings"

	"task182-dftimeline/internal/model"
)

// SaveArtifact 插入工件。content_sha256 唯一；重复哈希返回 ErrConflict。
func (s *Store) SaveArtifact(a *model.Artifact) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`
		INSERT INTO artifacts (id, source_id, artifact_type, path_name, raw_timestamp, raw_tz_offset_minutes, content_sha256, status, earliest_ms, latest_ms, evidence, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.SourceID, string(a.ArtifactType), a.PathName, a.RawTimestamp,
		a.RawTZOffsetMin, a.ContentSHA256, string(a.Status), a.EarliestMS, a.LatestMS,
		a.Evidence, a.CreatedAt.Format(timeFmt), a.UpdatedAt.Format(timeFmt))
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return model.ErrConflict
		}
		return fmt.Errorf("save artifact: %w", err)
	}
	return nil
}

// GetArtifactByHash 按内容哈希读取工件（用于幂等与哈希冲突校验）。
func (s *Store) GetArtifactByHash(hash string) (*model.Artifact, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := s.db.QueryRow(`
		SELECT id, source_id, artifact_type, path_name, raw_timestamp, raw_tz_offset_minutes, content_sha256, status, earliest_ms, latest_ms, evidence, created_at, updated_at
		FROM artifacts WHERE content_sha256 = ?`, hash)
	return scanArtifact(row)
}

// GetArtifact 按 ID 读取工件。
func (s *Store) GetArtifact(id string) (*model.Artifact, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := s.db.QueryRow(`
		SELECT id, source_id, artifact_type, path_name, raw_timestamp, raw_tz_offset_minutes, content_sha256, status, earliest_ms, latest_ms, evidence, created_at, updated_at
		FROM artifacts WHERE id = ?`, id)
	return scanArtifact(row)
}

func scanArtifact(row *sql.Row) (*model.Artifact, error) {
	var a model.Artifact
	var createdAt, updatedAt string
	if err := row.Scan(&a.ID, &a.SourceID, &a.ArtifactType, &a.PathName, &a.RawTimestamp,
		&a.RawTZOffsetMin, &a.ContentSHA256, &a.Status, &a.EarliestMS, &a.LatestMS,
		&a.Evidence, &createdAt, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("scan artifact: %w", err)
	}
	a.CreatedAt = parseTime(createdAt)
	a.UpdatedAt = parseTime(updatedAt)
	return &a, nil
}

// ListArtifacts 列出全部工件（按归一化区间升序，可选按状态过滤）。
func (s *Store) ListArtifacts(status string) ([]*model.Artifact, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	query := `SELECT id, source_id, artifact_type, path_name, raw_timestamp, raw_tz_offset_minutes, content_sha256, status, earliest_ms, latest_ms, evidence, created_at, updated_at
		FROM artifacts`
	args := []any{}
	if status != "" {
		query += ` WHERE status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY earliest_ms ASC, id ASC`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list artifacts: %w", err)
	}
	defer rows.Close()
	out := []*model.Artifact{}
	for rows.Next() {
		var a model.Artifact
		var createdAt, updatedAt string
		if err := rows.Scan(&a.ID, &a.SourceID, &a.ArtifactType, &a.PathName, &a.RawTimestamp,
			&a.RawTZOffsetMin, &a.ContentSHA256, &a.Status, &a.EarliestMS, &a.LatestMS,
			&a.Evidence, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan artifact row: %w", err)
		}
		a.CreatedAt = parseTime(createdAt)
		a.UpdatedAt = parseTime(updatedAt)
		out = append(out, &a)
	}
	return out, rows.Err()
}

// ListArtifactsBySource 列出某证据源的工件。
func (s *Store) ListArtifactsBySource(sourceID string) ([]*model.Artifact, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`
		SELECT id, source_id, artifact_type, path_name, raw_timestamp, raw_tz_offset_minutes, content_sha256, status, earliest_ms, latest_ms, evidence, created_at, updated_at
		FROM artifacts WHERE source_id = ? ORDER BY earliest_ms ASC, id ASC`, sourceID)
	if err != nil {
		return nil, fmt.Errorf("list artifacts by source: %w", err)
	}
	defer rows.Close()
	out := []*model.Artifact{}
	for rows.Next() {
		var a model.Artifact
		var createdAt, updatedAt string
		if err := rows.Scan(&a.ID, &a.SourceID, &a.ArtifactType, &a.PathName, &a.RawTimestamp,
			&a.RawTZOffsetMin, &a.ContentSHA256, &a.Status, &a.EarliestMS, &a.LatestMS,
			&a.Evidence, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan artifact row: %w", err)
		}
		a.CreatedAt = parseTime(createdAt)
		a.UpdatedAt = parseTime(updatedAt)
		out = append(out, &a)
	}
	return out, rows.Err()
}

// UpdateArtifactNormalized 写入归一化区间并推进状态 pending -> normalized。
func (s *Store) UpdateArtifactNormalized(id string, earliest, latest int64, evidence string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.Exec(`
		UPDATE artifacts SET earliest_ms = ?, latest_ms = ?, evidence = ?, status = ?, updated_at = ?
		WHERE id = ? AND status = ?`,
		earliest, latest, evidence, string(model.ArtifactNormalized), model.TimeNow().Format(timeFmt),
		id, string(model.ArtifactPending))
	if err != nil {
		return fmt.Errorf("normalize artifact: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return model.ErrInvalidState
	}
	return nil
}

// UpdateArtifactStatus 更新工件状态（乐观校验 expect）。
func (s *Store) UpdateArtifactStatus(id string, status, expect model.ArtifactStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.Exec(`
		UPDATE artifacts SET status = ?, updated_at = ? WHERE id = ? AND status = ?`,
		string(status), model.TimeNow().Format(timeFmt), id, string(expect))
	if err != nil {
		return fmt.Errorf("update artifact status: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return model.ErrInvalidState
	}
	return nil
}

// MarkArtifactConflict 把参与冲突核心的工件置为 conflict（从 normalized/suspicious 迁移）。
func (s *Store) MarkArtifactConflict(ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin conflict mark: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, id := range ids {
		if _, err := tx.Exec(`
			UPDATE artifacts SET status = ?, updated_at = ?
			WHERE id = ? AND status IN (?, ?, ?)`,
			string(model.ArtifactConflict), model.TimeNow().Format(timeFmt),
			id, string(model.ArtifactNormalized), string(model.ArtifactSuspicious), string(model.ArtifactAdopted)); err != nil {
			return fmt.Errorf("mark artifact conflict: %w", err)
		}
	}
	return tx.Commit()
}

// CountArtifacts 统计工件数量（全量与按状态）。
func (s *Store) CountArtifacts() (total int, byStatus map[model.ArtifactStatus]int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	byStatus = map[model.ArtifactStatus]int{}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM artifacts`).Scan(&total); err != nil {
		return 0, nil, fmt.Errorf("count artifacts: %w", err)
	}
	rows, err := s.db.Query(`SELECT status, COUNT(*) FROM artifacts GROUP BY status`)
	if err != nil {
		return 0, nil, fmt.Errorf("count artifacts by status: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var st string
		var n int
		if err := rows.Scan(&st, &n); err != nil {
			return 0, nil, err
		}
		byStatus[model.ArtifactStatus(st)] = n
	}
	return total, byStatus, rows.Err()
}

// NextArtifactChunk 按游标返回下一批未构建工件（用于时间线构建的断点续传）。
// limit 为批大小；返回的切片为空表示已全部处理。
func (s *Store) NextArtifactChunk(offset, limit int) ([]*model.Artifact, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`
		SELECT id, source_id, artifact_type, path_name, raw_timestamp, raw_tz_offset_minutes, content_sha256, status, earliest_ms, latest_ms, evidence, created_at, updated_at
		FROM artifacts
		WHERE status IN (?, ?, ?, ?)
		ORDER BY earliest_ms ASC, id ASC
		LIMIT ? OFFSET ?`,
		string(model.ArtifactNormalized), string(model.ArtifactConflict),
		string(model.ArtifactSuspicious), string(model.ArtifactAdopted), limit, offset)
	if err != nil {
		return nil, fmt.Errorf("next artifact chunk: %w", err)
	}
	defer rows.Close()
	out := []*model.Artifact{}
	for rows.Next() {
		var a model.Artifact
		var createdAt, updatedAt string
		if err := rows.Scan(&a.ID, &a.SourceID, &a.ArtifactType, &a.PathName, &a.RawTimestamp,
			&a.RawTZOffsetMin, &a.ContentSHA256, &a.Status, &a.EarliestMS, &a.LatestMS,
			&a.Evidence, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan artifact row: %w", err)
		}
		a.CreatedAt = parseTime(createdAt)
		a.UpdatedAt = parseTime(updatedAt)
		out = append(out, &a)
	}
	return out, rows.Err()
}
