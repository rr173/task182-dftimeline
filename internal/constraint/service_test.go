package constraint_test

import (
	"errors"
	"testing"

	"task182-dftimeline/internal/constraint"
	"task182-dftimeline/internal/model"
	"task182-dftimeline/internal/store"
)

func newSvc(t *testing.T) (*constraint.Service, *store.Store) {
	t.Helper()
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	now := model.TimeNow()
	if err := st.SaveSource(&model.EvidenceSource{
		ID: "src1", Name: "phone", SourceType: model.SourceDevice,
		Status: model.SourcePendingCalibration, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("save source: %v", err)
	}
	return constraint.NewService(st), st
}

// saveArtifact 写入一个可参与约束的归一化工件。
func saveArtifact(t *testing.T, st *store.Store, id, sha string) {
	t.Helper()
	now := model.TimeNow()
	if err := st.SaveArtifact(&model.Artifact{
		ID: id, SourceID: "src1", ArtifactType: model.ArtifactFile,
		PathName: id + ".txt", RawTimestamp: "2026-08-20T10:00:00+08:00",
		ContentSHA256: sha, Status: model.ArtifactNormalized,
		EarliestMS: 1000, LatestMS: 2000, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("save artifact %s: %v", id, err)
	}
}

func TestCreateConstraintRejectsSelfLoop(t *testing.T) {
	svc, st := newSvc(t)
	saveArtifact(t, st, "a1", "dead0000")

	c, err := svc.CreateConstraint(constraint.CreateInput{
		BeforeArtifactID: "a1",
		AfterArtifactID:  "a1",
	})
	if err == nil {
		t.Fatalf("expected self-loop constraint to be rejected, got %+v", c)
	}
	if !errors.Is(err, model.ErrBadRequest) {
		t.Fatalf("expected bad-request error for self-loop, got %v", err)
	}
	if c != nil {
		t.Fatalf("expected nil constraint on rejection, got %+v", c)
	}
}

func TestCreateConstraintAcceptsDistinctArtifacts(t *testing.T) {
	svc, st := newSvc(t)
	saveArtifact(t, st, "a1", "dead0001")
	saveArtifact(t, st, "a2", "dead0002")

	c, err := svc.CreateConstraint(constraint.CreateInput{
		BeforeArtifactID: "a1",
		AfterArtifactID:  "a2",
	})
	if err != nil {
		t.Fatalf("expected distinct-artifact constraint to be created, got %v", err)
	}
	if c.BeforeArtifactID != "a1" || c.AfterArtifactID != "a2" {
		t.Fatalf("unexpected constraint ids: %+v", c)
	}
	if c.IsSelfLoop() {
		t.Fatalf("non-self-loop constraint reported as self-loop: %+v", c)
	}
}
