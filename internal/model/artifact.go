package model

import "time"

// ArtifactType 描述工件类别。
type ArtifactType string

const (
	ArtifactFile   ArtifactType = "file"
	ArtifactLogLine ArtifactType = "log_line"
	ArtifactDevice ArtifactType = "device_record"
	ArtifactPacket ArtifactType = "network_packet"
)

// ArtifactStatus 工件状态机：
//
//	pending -> normalized（归一化成功，生成不确定区间）
//	normalized -> conflict（参与时间线冲突核心）
//	normalized -> suspicious（人工标记可疑）
//	normalized -> adopted（人工采纳为可信事件）
type ArtifactStatus string

const (
	ArtifactPending    ArtifactStatus = "pending"
	ArtifactNormalized ArtifactStatus = "normalized"
	ArtifactConflict   ArtifactStatus = "conflict"
	ArtifactSuspicious ArtifactStatus = "suspicious"
	ArtifactAdopted    ArtifactStatus = "adopted"
)

// Artifact 是来自某个证据源的带时间戳工件。
// RawTimestamp 与 RawTZOffset 保留原始读数；EarliestMS/LatestMS 是归一化后的不确定区间（Unix 毫秒）。
type Artifact struct {
	ID            string       `json:"id"`
	SourceID      string       `json:"source_id"`
	ArtifactType  ArtifactType `json:"artifact_type"`
	PathName      string       `json:"path_name"`
	RawTimestamp  string       `json:"raw_timestamp"`
	RawTZOffsetMin int         `json:"raw_tz_offset_minutes"`
	ContentSHA256 string       `json:"content_sha256"`
	Status        ArtifactStatus `json:"status"`
	EarliestMS    int64        `json:"earliest_ms,omitempty"`
	LatestMS      int64        `json:"latest_ms,omitempty"`
	Evidence      string       `json:"evidence"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
}

// CanParticipate 判断工件能否参与时间线构建（需已归一化）。
func (a *Artifact) CanParticipate() bool {
	return a.Status == ArtifactNormalized || a.Status == ArtifactConflict || a.Status == ArtifactSuspicious || a.Status == ArtifactAdopted
}
