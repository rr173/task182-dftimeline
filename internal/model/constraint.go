package model

import "time"

// ConstraintKind 偏序关系类型，目前支持“先于”（before）。
type ConstraintKind string

const (
	ConstraintBefore ConstraintKind = "before"
)

// ConstraintStatus 约束验证结果状态机：
//
//	pending -> satisfied（A.latest <= B.earliest）
//	pending -> violated （A.earliest > B.latest）
//	pending -> insufficient（区间重叠，证据不足）
type ConstraintStatus string

const (
	ConstraintPending      ConstraintStatus = "pending"
	ConstraintSatisfied    ConstraintStatus = "satisfied"
	ConstraintViolated     ConstraintStatus = "violated"
	ConstraintInsufficient ConstraintStatus = "insufficient"
)

// Constraint 声明一条偏序关系：BeforeArtifactID 发生在 AfterArtifactID 之前。
type Constraint struct {
	ID               string         `json:"id"`
	TimelineID       string         `json:"timeline_id,omitempty"`
	BeforeArtifactID string         `json:"before_artifact_id"`
	AfterArtifactID  string         `json:"after_artifact_id"`
	Kind             ConstraintKind `json:"kind"`
	Status           ConstraintStatus `json:"status"`
	VerdictReason    string         `json:"verdict_reason,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

// IsSelfLoop 判断约束是否指向同一工件。
func (c *Constraint) IsSelfLoop() bool { return c.BeforeArtifactID == c.AfterArtifactID }
