package model

import "time"

// SourceType 描述证据来源的工件类别。
type SourceType string

const (
	SourceFilesystem SourceType = "filesystem"
	SourceLog        SourceType = "log"
	SourceDevice     SourceType = "device"
	SourceNetwork    SourceType = "network"
)

// SourceStatus 证据源状态机：
//
//	pending_calibration -> calibrated        （存在生效时钟校正）
//	pending_calibration -> unknown_offset    （标记为偏差未知）
//	pending_calibration -> isolated          （隔离，禁止参与新时间线）
//	isolated           -> pending_calibration（重新激活）
type SourceStatus string

const (
	SourcePendingCalibration SourceStatus = "pending_calibration"
	SourceCalibrated         SourceStatus = "calibrated"
	SourceUnknownOffset      SourceStatus = "unknown_offset"
	SourceIsolated           SourceStatus = "isolated"
)

// EvidenceSource 是一个可产生带时间戳工件的证据来源（文件系统、日志、设备或网络）。
type EvidenceSource struct {
	ID                    string       `json:"id"`
	Name                  string       `json:"name"`
	SourceType            SourceType   `json:"source_type"`
	Status                SourceStatus `json:"status"`
	CurrentCalibrationID  string       `json:"current_calibration_id,omitempty"`
	CalibrationVersion    int          `json:"calibration_version"`
	CreatedAt             time.Time    `json:"created_at"`
	UpdatedAt             time.Time    `json:"updated_at"`
}

// CalibrationStatus 时钟校正状态：draft -> active -> superseded。
type CalibrationStatus string

const (
	CalibrationDraft      CalibrationStatus = "draft"
	CalibrationActive     CalibrationStatus = "active"
	CalibrationSuperseded CalibrationStatus = "superseded"
)

// ClockCalibration 描述一个证据源在某版本下的时钟模型：
// 时区偏移、固定偏差（毫秒）与不确定度（毫秒）。
type ClockCalibration struct {
	ID                string            `json:"id"`
	SourceID          string            `json:"source_id"`
	Version           int               `json:"version"`
	TZOffsetMinutes   int               `json:"tz_offset_minutes"`
	BiasMilliseconds  int64             `json:"bias_milliseconds"`
	UncertaintyMillis int64             `json:"uncertainty_milliseconds"`
	Status            CalibrationStatus `json:"status"`
	EffectiveAt       time.Time         `json:"effective_at"`
	CreatedAt         time.Time         `json:"created_at"`
}

// CanEmitTimestamp 判断源是否处于可产生可信时间戳的状态。
func (s *EvidenceSource) CanEmitTimestamp() bool {
	return s.Status == SourceCalibrated || s.Status == SourceUnknownOffset
}

// SourceTransitions 返回源允许的状态迁移表。
var SourceTransitions = map[SourceStatus][]SourceStatus{
	SourcePendingCalibration: {SourceCalibrated, SourceUnknownOffset, SourceIsolated},
	SourceIsolated:           {SourcePendingCalibration},
}

// CanTransition 判断源状态迁移是否合法。
func CanSourceTransition(from, to SourceStatus) bool {
	for _, next := range SourceTransitions[from] {
		if next == to {
			return true
		}
	}
	return false
}
