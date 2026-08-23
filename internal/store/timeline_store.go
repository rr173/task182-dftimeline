package store

import (
	"database/sql"
	"fmt"

	"task182-dftimeline/internal/model"
)

// ---------------------------------------------------------------------------
// Timeline
// ---------------------------------------------------------------------------

// SaveTimeline 插入或更新时间线。
func (s *Store) SaveTimeline(t *model.Timeline) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`
		INSERT INTO timelines (id, name, status, artifact_cursor, conflict_count, published_report_id, superseded_by_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			status = excluded.status,
			artifact_cursor = excluded.artifact_cursor,
			conflict_count = excluded.conflict_count,
			published_report_id = excluded.published_report_id,
			superseded_by_id = excluded.superseded_by_id,
			updated_at = excluded.updated_at`,
		t.ID, t.Name, string(t.Status), t.ArtifactCursor, t.ConflictCount,
		t.PublishedReportID, t.SupersededByID, t.CreatedAt.Format(timeFmt), t.UpdatedAt.Format(timeFmt))
	if err != nil {
		return fmt.Errorf("save timeline: %w", err)
	}
	return nil
}

// GetTimeline 按 ID 读取时间线。
func (s *Store) GetTimeline(id string) (*model.Timeline, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := s.db.QueryRow(`
		SELECT id, name, status, artifact_cursor, conflict_count, published_report_id, superseded_by_id, created_at, updated_at
		FROM timelines WHERE id = ?`, id)
	return scanTimeline(row)
}

func scanTimeline(row *sql.Row) (*model.Timeline, error) {
	var t model.Timeline
	var createdAt, updatedAt string
	if err := row.Scan(&t.ID, &t.Name, &t.Status, &t.ArtifactCursor, &t.ConflictCount,
		&t.PublishedReportID, &t.SupersededByID, &createdAt, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("scan timeline: %w", err)
	}
	t.CreatedAt = parseTime(createdAt)
	t.UpdatedAt = parseTime(updatedAt)
	return &t, nil
}

// ListTimelines 列出全部时间线。
func (s *Store) ListTimelines() ([]*model.Timeline, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`
		SELECT id, name, status, artifact_cursor, conflict_count, published_report_id, superseded_by_id, created_at, updated_at
		FROM timelines ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list timelines: %w", err)
	}
	defer rows.Close()
	out := []*model.Timeline{}
	for rows.Next() {
		var t model.Timeline
		var createdAt, updatedAt string
		if err := rows.Scan(&t.ID, &t.Name, &t.Status, &t.ArtifactCursor, &t.ConflictCount,
			&t.PublishedReportID, &t.SupersededByID, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan timeline row: %w", err)
		}
		t.CreatedAt = parseTime(createdAt)
		t.UpdatedAt = parseTime(updatedAt)
		out = append(out, &t)
	}
	return out, rows.Err()
}

// UpdateTimelineCursor 原子推进构建游标并更新状态。
func (s *Store) UpdateTimelineCursor(id string, cursor int, status model.TimelineStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`
		UPDATE timelines SET artifact_cursor = ?, status = ?, updated_at = ? WHERE id = ?`,
		cursor, string(status), model.TimeNow().Format(timeFmt), id)
	if err != nil {
		return fmt.Errorf("update timeline cursor: %w", err)
	}
	return nil
}

// UpdateConflictCount 累加时间线的冲突核心计数。
func (s *Store) UpdateConflictCount(id string, count int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`
		UPDATE timelines SET conflict_count = conflict_count + ?, updated_at = ? WHERE id = ?`,
		count, model.TimeNow().Format(timeFmt), id)
	if err != nil {
		return fmt.Errorf("update conflict count: %w", err)
	}
	return nil
}

// SupersedeTimeline 把旧时间线置为 superseded 并指向新时间线。
func (s *Store) SupersedeTimeline(id, supersededByID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.Exec(`
		UPDATE timelines SET status = ?, superseded_by_id = ?, updated_at = ?
		WHERE id = ? AND status = ?`,
		string(model.TimelineSuperseded), supersededByID, model.TimeNow().Format(timeFmt),
		id, string(model.TimelinePublished))
	if err != nil {
		return fmt.Errorf("supersede timeline: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return model.ErrInvalidState
	}
	return nil
}

// CountTimelines 统计时间线数量。
func (s *Store) CountTimelines() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM timelines`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count timelines: %w", err)
	}
	return n, nil
}

// ---------------------------------------------------------------------------
// ConflictCore
// ---------------------------------------------------------------------------

// SaveConflictCore 保存冲突核心（工件与边的 ID 列表以 JSON 存储）。
func (s *Store) SaveConflictCore(c *model.ConflictCore) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`
		INSERT INTO conflict_cores (id, timeline_id, artifact_ids, edge_ids, attribution, evidence, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.TimelineID, EncodeList(c.ArtifactIDs), EncodeList(c.EdgeIDs),
		string(c.Attribution), c.Evidence, c.CreatedAt.Format(timeFmt))
	if err != nil {
		return fmt.Errorf("save conflict core: %w", err)
	}
	return nil
}

// GetConflictCore 按 ID 读取冲突核心。
func (s *Store) GetConflictCore(id string) (*model.ConflictCore, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := s.db.QueryRow(`
		SELECT id, timeline_id, artifact_ids, edge_ids, attribution, evidence, created_at
		FROM conflict_cores WHERE id = ?`, id)
	return scanConflictCore(row)
}

func scanConflictCore(row *sql.Row) (*model.ConflictCore, error) {
	var c model.ConflictCore
	var artifactIDs, edgeIDs, createdAt string
	if err := row.Scan(&c.ID, &c.TimelineID, &artifactIDs, &edgeIDs, &c.Attribution,
		&c.Evidence, &createdAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("scan conflict core: %w", err)
	}
	if err := DecodeList(artifactIDs, &c.ArtifactIDs); err != nil {
		return nil, fmt.Errorf("decode artifact ids: %w", err)
	}
	if err := DecodeList(edgeIDs, &c.EdgeIDs); err != nil {
		return nil, fmt.Errorf("decode edge ids: %w", err)
	}
	c.CreatedAt = parseTime(createdAt)
	return &c, nil
}

// ListConflictCoresByTimeline 列出某时间线的全部冲突核心。
func (s *Store) ListConflictCoresByTimeline(timelineID string) ([]*model.ConflictCore, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`
		SELECT id, timeline_id, artifact_ids, edge_ids, attribution, evidence, created_at
		FROM conflict_cores WHERE timeline_id = ? ORDER BY created_at ASC`, timelineID)
	if err != nil {
		return nil, fmt.Errorf("list conflict cores: %w", err)
	}
	defer rows.Close()
	out := []*model.ConflictCore{}
	for rows.Next() {
		var c model.ConflictCore
		var artifactIDs, edgeIDs, createdAt string
		if err := rows.Scan(&c.ID, &c.TimelineID, &artifactIDs, &edgeIDs, &c.Attribution,
			&c.Evidence, &createdAt); err != nil {
			return nil, fmt.Errorf("scan conflict row: %w", err)
		}
		_ = DecodeList(artifactIDs, &c.ArtifactIDs)
		_ = DecodeList(edgeIDs, &c.EdgeIDs)
		c.CreatedAt = parseTime(createdAt)
		out = append(out, &c)
	}
	return out, rows.Err()
}

// CountConflictCores 统计冲突核心数量。
func (s *Store) CountConflictCores() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM conflict_cores`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count conflict cores: %w", err)
	}
	return n, nil
}

// ---------------------------------------------------------------------------
// Adjudication
// ---------------------------------------------------------------------------

// SaveAdjudication 保存裁决记录。
func (s *Store) SaveAdjudication(a *model.Adjudication) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`
		INSERT INTO adjudications (id, timeline_id, artifact_id, action, reason, adjudicated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		a.ID, a.TimelineID, a.ArtifactID, string(a.Action), a.Reason, a.AdjudicatedAt.Format(timeFmt))
	if err != nil {
		return fmt.Errorf("save adjudication: %w", err)
	}
	return nil
}

// ListAdjudicationsByTimeline 列出某时间线的裁决。
func (s *Store) ListAdjudicationsByTimeline(timelineID string) ([]*model.Adjudication, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`
		SELECT id, timeline_id, artifact_id, action, reason, adjudicated_at
		FROM adjudications WHERE timeline_id = ? ORDER BY adjudicated_at ASC`, timelineID)
	if err != nil {
		return nil, fmt.Errorf("list adjudications: %w", err)
	}
	defer rows.Close()
	out := []*model.Adjudication{}
	for rows.Next() {
		var a model.Adjudication
		var adjudicatedAt string
		if err := rows.Scan(&a.ID, &a.TimelineID, &a.ArtifactID, &a.Action, &a.Reason,
			&adjudicatedAt); err != nil {
			return nil, fmt.Errorf("scan adjudication row: %w", err)
		}
		a.AdjudicatedAt = parseTime(adjudicatedAt)
		out = append(out, &a)
	}
	return out, rows.Err()
}
