package model

import "time"

// ReportStatus 报告状态：draft -> frozen。发布后不可变。
type ReportStatus string

const (
	ReportDraft  ReportStatus = "draft"
	ReportFrozen ReportStatus = "frozen"
)

// Report 是时间线的冻结快照。它绑定时钟模型版本：
// ClockSnapshot 记录每个参与源在发布时刻的生效校正版本，
// 后续任何校准更新都只能产生替代报告，不修改本报告。
type Report struct {
	ID                  string       `json:"id"`
	TimelineID          string       `json:"timeline_id"`
	Version             int          `json:"version"`
	Status              ReportStatus `json:"status"`
	ClockSnapshot       []ClockSnapshotEntry `json:"clock_snapshot"`
	ArtifactSnapshot    []ArtifactSnapshotEntry `json:"artifact_snapshot"`
	ConflictSummary     []ConflictSummaryEntry  `json:"conflict_summary"`
	AdjudicationSummary []AdjudicationSummaryEntry `json:"adjudication_summary"`
	PublishedAt         time.Time    `json:"published_at,omitempty"`
	CreatedAt           time.Time    `json:"created_at"`
}

// ClockSnapshotEntry 记录一个证据源在报告发布时的时钟模型版本。
type ClockSnapshotEntry struct {
	SourceID            string `json:"source_id"`
	CalibrationVersion  int    `json:"calibration_version"`
	TZOffsetMinutes     int    `json:"tz_offset_minutes"`
	BiasMilliseconds    int64  `json:"bias_milliseconds"`
	UncertaintyMillis   int64  `json:"uncertainty_milliseconds"`
}

// ArtifactSnapshotEntry 记录工件在报告中的归一化区间快照。
type ArtifactSnapshotEntry struct {
	ArtifactID string `json:"artifact_id"`
	PathName   string `json:"path_name"`
	EarliestMS int64  `json:"earliest_ms"`
	LatestMS   int64  `json:"latest_ms"`
	Status     string `json:"status"`
}

// ConflictSummaryEntry 记录报告包含的冲突核心摘要。
type ConflictSummaryEntry struct {
	ConflictID  string   `json:"conflict_id"`
	ArtifactIDs []string `json:"artifact_ids"`
	Attribution string   `json:"attribution"`
}

// AdjudicationSummaryEntry 记录报告包含的裁决摘要。
type AdjudicationSummaryEntry struct {
	AdjudicationID string `json:"adjudication_id"`
	ArtifactID     string `json:"artifact_id"`
	Action         string `json:"action"`
	Reason         string `json:"reason"`
}
