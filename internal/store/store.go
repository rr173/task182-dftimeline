// Package store 提供基于 SQLite 的持久化层。
//
// 使用 modernc.org/sqlite（纯 Go 驱动，CGO 无关）。整个服务使用单连接，
// 配合互斥锁与事务串行化写入，保证重启后可恢复全部业务数据。
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
)

// Store 封装 SQLite 连接与表结构迁移。
type Store struct {
	db   *sql.DB
	mu   sync.Mutex
	path string
}

// Open 打开（必要时创建）数据库并执行幂等迁移。
// path 为 ":memory:" 时用于测试。
func Open(path string) (*Store, error) {
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
			return nil, fmt.Errorf("create db dir: %w", err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	s := &Store{db: db, path: path}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

// Close 关闭数据库连接。
func (s *Store) Close() error { return s.db.Close() }

// DB 返回底层连接（仅在事务内使用）。
func (s *Store) DB() *sql.DB { return s.db }

// Path 返回数据库文件路径。
func (s *Store) Path() string { return s.path }

// migrate 幂等创建全部业务表与唯一索引。
func (s *Store) migrate() error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS evidence_sources (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			source_type TEXT NOT NULL,
			status TEXT NOT NULL,
			current_calibration_id TEXT NOT NULL DEFAULT '',
			calibration_version INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS clock_calibrations (
			id TEXT PRIMARY KEY,
			source_id TEXT NOT NULL,
			version INTEGER NOT NULL,
			tz_offset_minutes INTEGER NOT NULL,
			bias_milliseconds INTEGER NOT NULL,
			uncertainty_milliseconds INTEGER NOT NULL,
			status TEXT NOT NULL,
			effective_at TEXT NOT NULL,
			created_at TEXT NOT NULL,
			UNIQUE (source_id, version)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_calibration_source_status ON clock_calibrations (source_id, status)`,
		`CREATE TABLE IF NOT EXISTS artifacts (
			id TEXT PRIMARY KEY,
			source_id TEXT NOT NULL,
			artifact_type TEXT NOT NULL,
			path_name TEXT NOT NULL,
			raw_timestamp TEXT NOT NULL,
			raw_tz_offset_minutes INTEGER NOT NULL,
			content_sha256 TEXT NOT NULL,
			status TEXT NOT NULL,
			earliest_ms INTEGER NOT NULL DEFAULT 0,
			latest_ms INTEGER NOT NULL DEFAULT 0,
			evidence TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			UNIQUE (content_sha256)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_artifact_source ON artifacts (source_id)`,
		`CREATE INDEX IF NOT EXISTS idx_artifact_status ON artifacts (status)`,
		`CREATE TABLE IF NOT EXISTS constraints (
			id TEXT PRIMARY KEY,
			timeline_id TEXT NOT NULL DEFAULT '',
			before_artifact_id TEXT NOT NULL,
			after_artifact_id TEXT NOT NULL,
			kind TEXT NOT NULL,
			status TEXT NOT NULL,
			verdict_reason TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_constraint_timeline ON constraints (timeline_id)`,
		`CREATE TABLE IF NOT EXISTS timelines (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			status TEXT NOT NULL,
			artifact_cursor INTEGER NOT NULL DEFAULT 0,
			conflict_count INTEGER NOT NULL DEFAULT 0,
			published_report_id TEXT NOT NULL DEFAULT '',
			superseded_by_id TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS conflict_cores (
			id TEXT PRIMARY KEY,
			timeline_id TEXT NOT NULL,
			artifact_ids TEXT NOT NULL,
			edge_ids TEXT NOT NULL,
			attribution TEXT NOT NULL,
			evidence TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_conflict_timeline ON conflict_cores (timeline_id)`,
		`CREATE TABLE IF NOT EXISTS adjudications (
			id TEXT PRIMARY KEY,
			timeline_id TEXT NOT NULL,
			artifact_id TEXT NOT NULL,
			action TEXT NOT NULL,
			reason TEXT NOT NULL,
			adjudicated_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_adjudication_timeline ON adjudications (timeline_id)`,
		`CREATE TABLE IF NOT EXISTS reports (
			id TEXT PRIMARY KEY,
			timeline_id TEXT NOT NULL,
			version INTEGER NOT NULL,
			status TEXT NOT NULL,
			clock_snapshot TEXT NOT NULL,
			artifact_snapshot TEXT NOT NULL,
			conflict_summary TEXT NOT NULL,
			adjudication_summary TEXT NOT NULL,
			published_at TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			UNIQUE (timeline_id, version)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_report_timeline ON reports (timeline_id)`,
	}
	for _, stmt := range statements {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("exec migration: %w", err)
		}
	}
	return nil
}
