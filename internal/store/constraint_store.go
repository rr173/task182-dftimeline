package store

import (
	"database/sql"
	"fmt"

	"task182-dftimeline/internal/model"
)

// SaveConstraint 插入偏序约束。
func (s *Store) SaveConstraint(c *model.Constraint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`
		INSERT INTO constraints (id, timeline_id, before_artifact_id, after_artifact_id, kind, status, verdict_reason, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.TimelineID, c.BeforeArtifactID, c.AfterArtifactID, string(c.Kind),
		string(c.Status), c.VerdictReason, c.CreatedAt.Format(timeFmt), c.UpdatedAt.Format(timeFmt))
	if err != nil {
		return fmt.Errorf("save constraint: %w", err)
	}
	return nil
}

// GetConstraint 按 ID 读取约束。
func (s *Store) GetConstraint(id string) (*model.Constraint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := s.db.QueryRow(`
		SELECT id, timeline_id, before_artifact_id, after_artifact_id, kind, status, verdict_reason, created_at, updated_at
		FROM constraints WHERE id = ?`, id)
	return scanConstraint(row)
}

func scanConstraint(row *sql.Row) (*model.Constraint, error) {
	var c model.Constraint
	var createdAt, updatedAt string
	if err := row.Scan(&c.ID, &c.TimelineID, &c.BeforeArtifactID, &c.AfterArtifactID,
		&c.Kind, &c.Status, &c.VerdictReason, &createdAt, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("scan constraint: %w", err)
	}
	c.CreatedAt = parseTime(createdAt)
	c.UpdatedAt = parseTime(updatedAt)
	return &c, nil
}

// ListConstraints 列出全部约束。
func (s *Store) ListConstraints() ([]*model.Constraint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`
		SELECT id, timeline_id, before_artifact_id, after_artifact_id, kind, status, verdict_reason, created_at, updated_at
		FROM constraints ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("list constraints: %w", err)
	}
	defer rows.Close()
	return scanConstraintRows(rows)
}

// ListConstraintsByTimeline 列出绑定到某时间线的约束。
func (s *Store) ListConstraintsByTimeline(timelineID string) ([]*model.Constraint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`
		SELECT id, timeline_id, before_artifact_id, after_artifact_id, kind, status, verdict_reason, created_at, updated_at
		FROM constraints WHERE timeline_id = ? ORDER BY created_at ASC`, timelineID)
	if err != nil {
		return nil, fmt.Errorf("list timeline constraints: %w", err)
	}
	defer rows.Close()
	return scanConstraintRows(rows)
}

// ListConstraintsForArtifact 返回涉及指定工件的约束（供求解器构建邻接表）。
func (s *Store) ListConstraintsForArtifact(artifactID string) ([]*model.Constraint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`
		SELECT id, timeline_id, before_artifact_id, after_artifact_id, kind, status, verdict_reason, created_at, updated_at
		FROM constraints WHERE before_artifact_id = ? OR after_artifact_id = ?
		ORDER BY created_at ASC`, artifactID, artifactID)
	if err != nil {
		return nil, fmt.Errorf("list artifact constraints: %w", err)
	}
	defer rows.Close()
	return scanConstraintRows(rows)
}

func scanConstraintRows(rows *sql.Rows) ([]*model.Constraint, error) {
	out := []*model.Constraint{}
	for rows.Next() {
		var c model.Constraint
		var createdAt, updatedAt string
		if err := rows.Scan(&c.ID, &c.TimelineID, &c.BeforeArtifactID, &c.AfterArtifactID,
			&c.Kind, &c.Status, &c.VerdictReason, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan constraint row: %w", err)
		}
		c.CreatedAt = parseTime(createdAt)
		c.UpdatedAt = parseTime(updatedAt)
		out = append(out, &c)
	}
	return out, rows.Err()
}

// UpdateConstraintVerdict 写入约束验证结论。
func (s *Store) UpdateConstraintVerdict(id string, status model.ConstraintStatus, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`
		UPDATE constraints SET status = ?, verdict_reason = ?, updated_at = ?
		WHERE id = ?`,
		string(status), reason, model.TimeNow().Format(timeFmt), id)
	if err != nil {
		return fmt.Errorf("update constraint verdict: %w", err)
	}
	return nil
}

// CountConstraints 统计约束数量与各状态分布。
func (s *Store) CountConstraints() (total int, byStatus map[model.ConstraintStatus]int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	byStatus = map[model.ConstraintStatus]int{}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM constraints`).Scan(&total); err != nil {
		return 0, nil, fmt.Errorf("count constraints: %w", err)
	}
	rows, err := s.db.Query(`SELECT status, COUNT(*) FROM constraints GROUP BY status`)
	if err != nil {
		return 0, nil, fmt.Errorf("count constraints by status: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var st string
		var n int
		if err := rows.Scan(&st, &n); err != nil {
			return 0, nil, err
		}
		byStatus[model.ConstraintStatus(st)] = n
	}
	return total, byStatus, rows.Err()
}
