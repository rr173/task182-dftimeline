package solver

import (
	"testing"

	"task182-dftimeline/internal/model"
)

func mkArtifact(id string, earliest, latest int64) *model.Artifact {
	return &model.Artifact{ID: id, EarliestMS: earliest, LatestMS: latest, Status: model.ArtifactNormalized}
}

func TestTopologicalOrderAcyclic(t *testing.T) {
	a, b, c := mkArtifact("a", 100, 200), mkArtifact("b", 300, 400), mkArtifact("c", 500, 600)
	g := NewGraph([]*model.Artifact{a, b, c}, []*model.Constraint{
		{ID: "e1", Kind: model.ConstraintBefore, BeforeArtifactID: "a", AfterArtifactID: "b"},
		{ID: "e2", Kind: model.ConstraintBefore, BeforeArtifactID: "b", AfterArtifactID: "c"},
	})
	order, cyclic := g.TopologicalOrder()
	if cyclic {
		t.Fatal("expected acyclic")
	}
	if len(order) != 3 || order[0] != "a" || order[2] != "c" {
		t.Fatalf("unexpected order: %v", order)
	}
}

func TestMinConflictCycleFindsShortest(t *testing.T) {
	// 环 A->B->D->A（3 个节点）与 A->B->C->D->A（4 个节点）并存，应返回 3 节点环。
	a, b, c, d := mkArtifact("a", 0, 10), mkArtifact("b", 0, 10), mkArtifact("c", 0, 10), mkArtifact("d", 0, 10)
	cons := []*model.Constraint{
		{ID: "e1", Kind: model.ConstraintBefore, BeforeArtifactID: "a", AfterArtifactID: "b"},
		{ID: "e2", Kind: model.ConstraintBefore, BeforeArtifactID: "b", AfterArtifactID: "d"},
		{ID: "e3", Kind: model.ConstraintBefore, BeforeArtifactID: "d", AfterArtifactID: "a"},
		{ID: "e4", Kind: model.ConstraintBefore, BeforeArtifactID: "b", AfterArtifactID: "c"},
		{ID: "e5", Kind: model.ConstraintBefore, BeforeArtifactID: "c", AfterArtifactID: "d"},
	}
	g := NewGraph([]*model.Artifact{a, b, c, d}, cons)
	nodes, edges := g.MinConflictCycle()
	if len(nodes) != 3 {
		t.Fatalf("expected minimal cycle of 3 nodes, got %v (edges=%v)", nodes, edges)
	}
	// 环必须包含 a、b、d。
	seen := map[string]bool{}
	for _, n := range nodes {
		seen[n] = true
	}
	if !seen["a"] || !seen["b"] || !seen["d"] {
		t.Fatalf("cycle nodes wrong: %v", nodes)
	}
}

func TestCycleKeyDedup(t *testing.T) {
	k1 := CycleKey([]string{"a", "b", "c"})
	k2 := CycleKey([]string{"c", "a", "b"})
	if k1 != k2 {
		t.Fatalf("cycle keys should be canonical: %s vs %s", k1, k2)
	}
}

func TestAttributePrioritizesTamper(t *testing.T) {
	arts := []*model.Artifact{
		mkArtifact("a", 0, 1000),
		mkArtifact("b", 2000, 3000),
	}
	arts[1].Status = model.ArtifactSuspicious
	attr, evidence := Attribute(AttributionInput{
		Artifacts:     arts,
		Sources:       map[string]*model.EvidenceSource{},
		SuspiciousIDs: map[string]bool{"b": true},
	})
	if attr != model.AttributionMetadataTamper {
		t.Fatalf("expected metadata_tamper, got %s (%s)", attr, evidence)
	}
}

func TestAttributeInsufficientForWideIntervals(t *testing.T) {
	arts := []*model.Artifact{
		mkArtifact("a", 0, WideIntervalThresholdMS+1),
		mkArtifact("b", WideIntervalThresholdMS+2, WideIntervalThresholdMS*2+3),
	}
	attr, _ := Attribute(AttributionInput{Artifacts: arts, Sources: map[string]*model.EvidenceSource{}})
	if attr != model.AttributionInsufficient {
		t.Fatalf("expected insufficient_evidence, got %s", attr)
	}
}
