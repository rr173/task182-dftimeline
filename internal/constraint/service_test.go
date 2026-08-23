package constraint_test

import (
	"testing"

	"task182-dftimeline/internal/constraint"
	"task182-dftimeline/internal/model"
	"task182-dftimeline/internal/store"
)

// newServiceWithIntervalArtifacts 构造一个内存存储并直接写入两个已归一化的工件，
// 区间由调用方指定（单位：Unix 毫秒）。绕过 Register/Normalize 流程，便于精确
// 构造首尾相接 / 重叠 / 反转等边界关系。
func newServiceWithIntervalArtifacts(t *testing.T, aEarliest, aLatest, bEarliest, bLatest int64) (*constraint.Service, *model.Artifact, *model.Artifact) {
	t.Helper()
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	now := model.TimeNow()
	src := &model.EvidenceSource{
		ID: "src1", Name: "src", SourceType: model.SourceFilesystem,
		Status: model.SourceCalibrated, CreatedAt: now, UpdatedAt: now,
	}
	if err := st.SaveSource(src); err != nil {
		t.Fatal(err)
	}
	artA := &model.Artifact{
		ID: "art-a", SourceID: "src1", ArtifactType: model.ArtifactFile, PathName: "a.txt",
		RawTimestamp: "2026-08-20T10:00:00Z", ContentSHA256: "aaaa",
		Status: model.ArtifactNormalized, EarliestMS: aEarliest, LatestMS: aLatest,
		CreatedAt: now, UpdatedAt: now,
	}
	artB := &model.Artifact{
		ID: "art-b", SourceID: "src1", ArtifactType: model.ArtifactFile, PathName: "b.txt",
		RawTimestamp: "2026-08-20T10:00:30Z", ContentSHA256: "bbbb",
		Status: model.ArtifactNormalized, EarliestMS: bEarliest, LatestMS: bLatest,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := st.SaveArtifact(artA); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveArtifact(artB); err != nil {
		t.Fatal(err)
	}
	a, _ := st.GetArtifact("art-a")
	b, _ := st.GetArtifact("art-b")
	return constraint.NewService(st), a, b
}

// verify 声明 before->after 约束并验证，返回最终状态。
func verify(t *testing.T, svc *constraint.Service, beforeID, afterID string) model.ConstraintStatus {
	t.Helper()
	con, err := svc.CreateConstraint(constraint.CreateInput{BeforeArtifactID: beforeID, AfterArtifactID: afterID})
	if err != nil {
		t.Fatal(err)
	}
	verified, err := svc.VerifyConstraint(con.ID)
	if err != nil {
		t.Fatal(err)
	}
	return verified.Status
}

// TestVerifyConstraintAbuttingIntervalsSatisfied 是回归测试：
// 两个半开区间恰好首尾相接（A.latest == B.earliest）时，before 约束
// 必须判定为 satisfied，而不是落入 insufficient（证据不足）。
func TestVerifyConstraintAbuttingIntervalsSatisfied(t *testing.T) {
	// A = [100, 200)，B = [200, 300)：半开区间下 A.latest == B.earliest == 200。
	svc, before, after := newServiceWithIntervalArtifacts(t, 100, 200, 200, 300)
	if before.LatestMS != after.EarliestMS {
		t.Fatalf("precondition: expected abutting intervals, got A.latest=%d != B.earliest=%d",
			before.LatestMS, after.EarliestMS)
	}
	if got := verify(t, svc, before.ID, after.ID); got != model.ConstraintSatisfied {
		t.Fatalf("abutting half-open intervals must be satisfied, got %s", got)
	}
}

// TestVerifyConstraintFullyBeforeSatisfied 验证严格分离（A.latest < B.earliest）仍 satisfied。
func TestVerifyConstraintFullyBeforeSatisfied(t *testing.T) {
	svc, before, after := newServiceWithIntervalArtifacts(t, 100, 150, 200, 300)
	if before.LatestMS >= after.EarliestMS {
		t.Fatalf("precondition: expected strictly separated intervals")
	}
	if got := verify(t, svc, before.ID, after.ID); got != model.ConstraintSatisfied {
		t.Fatalf("strictly-separated intervals must be satisfied, got %s", got)
	}
}

// TestVerifyConstraintOverlappingInsufficient 验证区间真重叠仍判为 insufficient。
func TestVerifyConstraintOverlappingInsufficient(t *testing.T) {
	// A = [100, 250)，B = [200, 300)：A 的右端越过 B 的左端，真重叠。
	svc, before, after := newServiceWithIntervalArtifacts(t, 100, 250, 200, 300)
	if got := verify(t, svc, before.ID, after.ID); got != model.ConstraintInsufficient {
		t.Fatalf("overlapping intervals must be insufficient, got %s", got)
	}
}

// TestVerifyConstraintReversedViolated 验证 A 完全在 B 之后时判为 violated。
func TestVerifyConstraintReversedViolated(t *testing.T) {
	// A = [400, 500)，B = [200, 300)：A.earliest(400) > B.latest(300)。
	svc, before, after := newServiceWithIntervalArtifacts(t, 400, 500, 200, 300)
	if before.EarliestMS <= after.LatestMS {
		t.Fatalf("precondition: expected reversed intervals")
	}
	if got := verify(t, svc, before.ID, after.ID); got != model.ConstraintViolated {
		t.Fatalf("reversed intervals must be violated, got %s", got)
	}
}
