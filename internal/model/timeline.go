package model

import "time"

// TimelineStatus 时间线状态机：
//
//	building -> reviewable   （偏序图构建完成且无环）
//	building -> conflicted   （构建时发现冲突核心）
//	reviewable/conflicted -> published（发布冻结报告）
//	published -> superseded  （源校准更新后产生替代时间线）
type TimelineStatus string

const (
	TimelineBuilding   TimelineStatus = "building"
	TimelineReviewable TimelineStatus = "reviewable"
	TimelineConflicted TimelineStatus = "conflicted"
	TimelinePublished  TimelineStatus = "published"
	TimelineSuperseded TimelineStatus = "superseded"
)

// Timeline 是一次时间线构建会话。构建游标记录已归一化工件的处理进度，
// 重启后从游标继续，避免重复构建。
type Timeline struct {
	ID                 string        `json:"id"`
	Name               string        `json:"name"`
	Status             TimelineStatus `json:"status"`
	ArtifactCursor     int           `json:"artifact_cursor"`
	ConflictCount      int           `json:"conflict_count"`
	PublishedReportID  string        `json:"published_report_id,omitempty"`
	SupersededByID     string        `json:"superseded_by_id,omitempty"`
	CreatedAt          time.Time     `json:"created_at"`
	UpdatedAt          time.Time     `json:"updated_at"`
}

// Attribution 冲突归因类别。
type Attribution string

const (
	AttributionClockSkew     Attribution = "clock_skew"
	AttributionMetadataTamper Attribution = "metadata_tamper"
	AttributionInsufficient  Attribution = "insufficient_evidence"
)

// ConflictCore 是一次构建发现的最小冲突环：一组工件在偏序约束下无法满足，
// 系统输出最小环证据并给出归因。
type ConflictCore struct {
	ID           string      `json:"id"`
	TimelineID   string      `json:"timeline_id"`
	ArtifactIDs  []string    `json:"artifact_ids"`
	EdgeIDs      []string    `json:"edge_ids"`
	Attribution  Attribution `json:"attribution"`
	Evidence     string      `json:"evidence"`
	CreatedAt    time.Time   `json:"created_at"`
}

// AdjudicationAction 人工裁决动作。
type AdjudicationAction string

const (
	AdjudicationAdopt      AdjudicationAction = "adopt"
	AdjudicationSuspicious AdjudicationAction = "mark_suspicious"
	AdjudicationIsolate    AdjudicationAction = "isolate_source"
	AdjudicationCorrectClock AdjudicationAction = "correct_clock"
)

// Adjudication 是对冲突工件的裁决记录。
type Adjudication struct {
	ID           string             `json:"id"`
	TimelineID   string             `json:"timeline_id"`
	ArtifactID   string             `json:"artifact_id"`
	Action       AdjudicationAction `json:"action"`
	Reason       string             `json:"reason"`
	AdjudicatedAt time.Time          `json:"adjudicated_at"`
}
