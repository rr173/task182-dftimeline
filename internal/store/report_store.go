package store

import (
	"database/sql"
	"fmt"

	"task182-dftimeline/internal/model"
)

// SaveReport 插入报告。同时间线的版本号唯一。
func (s *Store) SaveReport(r *model.Report) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`
		INSERT INTO reports (id, timeline_id, version, status, clock_snapshot, artifact_snapshot, conflict_summary, adjudication_summary, published_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.TimelineID, r.Version, string(r.Status),
		EncodeList(r.ClockSnapshot), EncodeList(r.ArtifactSnapshot),
		EncodeList(r.ConflictSummary), EncodeList(r.AdjudicationSummary),
		r.PublishedAt.Format(timeFmt), r.CreatedAt.Format(timeFmt))
	if err != nil {
		return fmt.Errorf("save report: %w", err)
	}
	return nil
}

// GetReport 按 ID 读取报告。
func (s *Store) GetReport(id string) (*model.Report, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := s.db.QueryRow(`
		SELECT id, timeline_id, version, status, clock_snapshot, artifact_snapshot, conflict_summary, adjudication_summary, published_at, created_at
		FROM reports WHERE id = ?`, id)
	return scanReport(row)
}

func scanReport(row *sql.Row) (*model.Report, error) {
	var r model.Report
	var clockSnap, artifactSnap, conflictSum, adjSum, publishedAt, createdAt string
	if err := row.Scan(&r.ID, &r.TimelineID, &r.Version, &r.Status, &clockSnap, &artifactSnap,
		&conflictSum, &adjSum, &publishedAt, &createdAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("scan report: %w", err)
	}
	_ = DecodeList(clockSnap, &r.ClockSnapshot)
	_ = DecodeList(artifactSnap, &r.ArtifactSnapshot)
	_ = DecodeList(conflictSum, &r.ConflictSummary)
	_ = DecodeList(adjSum, &r.AdjudicationSummary)
	r.PublishedAt = parseTime(publishedAt)
	r.CreatedAt = parseTime(createdAt)
	return &r, nil
}

// ListReports 列出全部报告（按版本倒序）。
func (s *Store) ListReports() ([]*model.Report, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`
		SELECT id, timeline_id, version, status, clock_snapshot, artifact_snapshot, conflict_summary, adjudication_summary, published_at, created_at
		FROM reports ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list reports: %w", err)
	}
	defer rows.Close()
	out := []*model.Report{}
	for rows.Next() {
		var r model.Report
		var clockSnap, artifactSnap, conflictSum, adjSum, publishedAt, createdAt string
		if err := rows.Scan(&r.ID, &r.TimelineID, &r.Version, &r.Status, &clockSnap, &artifactSnap,
			&conflictSum, &adjSum, &publishedAt, &createdAt); err != nil {
			return nil, fmt.Errorf("scan report row: %w", err)
		}
		_ = DecodeList(clockSnap, &r.ClockSnapshot)
		_ = DecodeList(artifactSnap, &r.ArtifactSnapshot)
		_ = DecodeList(conflictSum, &r.ConflictSummary)
		_ = DecodeList(adjSum, &r.AdjudicationSummary)
		r.PublishedAt = parseTime(publishedAt)
		r.CreatedAt = parseTime(createdAt)
		out = append(out, &r)
	}
	return out, rows.Err()
}

// ListReportsByTimeline 列出某时间线的报告。
func (s *Store) ListReportsByTimeline(timelineID string) ([]*model.Report, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`
		SELECT id, timeline_id, version, status, clock_snapshot, artifact_snapshot, conflict_summary, adjudication_summary, published_at, created_at
		FROM reports WHERE timeline_id = ? ORDER BY version DESC`, timelineID)
	if err != nil {
		return nil, fmt.Errorf("list timeline reports: %w", err)
	}
	defer rows.Close()
	out := []*model.Report{}
	for rows.Next() {
		var r model.Report
		var clockSnap, artifactSnap, conflictSum, adjSum, publishedAt, createdAt string
		if err := rows.Scan(&r.ID, &r.TimelineID, &r.Version, &r.Status, &clockSnap, &artifactSnap,
			&conflictSum, &adjSum, &publishedAt, &createdAt); err != nil {
			return nil, fmt.Errorf("scan timeline report row: %w", err)
		}
		_ = DecodeList(clockSnap, &r.ClockSnapshot)
		_ = DecodeList(artifactSnap, &r.ArtifactSnapshot)
		_ = DecodeList(conflictSum, &r.ConflictSummary)
		_ = DecodeList(adjSum, &r.AdjudicationSummary)
		r.PublishedAt = parseTime(publishedAt)
		r.CreatedAt = parseTime(createdAt)
		out = append(out, &r)
	}
	return out, rows.Err()
}

// NextReportVersion 返回某时间线的下一个报告版本号（当前最大 + 1）。
func (s *Store) NextReportVersion(timelineID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var max int
	err := s.db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM reports WHERE timeline_id = ?`, timelineID).Scan(&max)
	if err != nil {
		return 0, fmt.Errorf("next report version: %w", err)
	}
	return max + 1, nil
}
