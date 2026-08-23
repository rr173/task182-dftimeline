package store_test

import (
	"testing"

	"task182-dftimeline/internal/model"
	"task182-dftimeline/internal/store"
)

func TestStoreCRUDAndUniqueHash(t *testing.T) {
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	now := model.TimeNow()
	src := &model.EvidenceSource{ID: "src1", Name: "phone", SourceType: model.SourceDevice,
		Status: model.SourcePendingCalibration, CreatedAt: now, UpdatedAt: now}
	if err := st.SaveSource(src); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetSource("src1")
	if err != nil || got.Name != "phone" {
		t.Fatalf("get source: %v %+v", err, got)
	}

	art := &model.Artifact{ID: "a1", SourceID: "src1", ArtifactType: model.ArtifactFile,
		PathName: "f.txt", RawTimestamp: "2026-08-20T10:00:00+08:00", ContentSHA256: "deadbeef",
		Status: model.ArtifactPending, CreatedAt: now, UpdatedAt: now}
	if err := st.SaveArtifact(art); err != nil {
		t.Fatal(err)
	}
	dup := *art
	dup.ID = "a2"
	if err := st.SaveArtifact(&dup); err != model.ErrConflict {
		t.Fatalf("expected conflict on duplicate hash, got %v", err)
	}
}

func TestCalibrationActivationSingleActive(t *testing.T) {
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	now := model.TimeNow()
	cal1 := &model.ClockCalibration{ID: "c1", SourceID: "src1", Version: 1, TZOffsetMinutes: 480,
		BiasMilliseconds: 0, UncertaintyMillis: 1000, Status: model.CalibrationDraft,
		EffectiveAt: now, CreatedAt: now}
	cal2 := &model.ClockCalibration{ID: "c2", SourceID: "src1", Version: 2, TZOffsetMinutes: 480,
		BiasMilliseconds: 500, UncertaintyMillis: 2000, Status: model.CalibrationDraft,
		EffectiveAt: now, CreatedAt: now}
	if err := st.SaveCalibration(cal1); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveCalibration(cal2); err != nil {
		t.Fatal(err)
	}
	if err := st.ActivateCalibration("c1"); err != nil {
		t.Fatal(err)
	}
	active, err := st.GetActiveCalibration("src1")
	if err != nil || active.ID != "c1" {
		t.Fatalf("expected c1 active, got %+v err=%v", active, err)
	}
	if err := st.ActivateCalibration("c2"); err != nil {
		t.Fatal(err)
	}
	active, err = st.GetActiveCalibration("src1")
	if err != nil || active.ID != "c2" {
		t.Fatalf("expected c2 active after re-activation, got %+v err=%v", active, err)
	}
	all, err := st.ListCalibrationsBySource("src1")
	if err != nil {
		t.Fatal(err)
	}
	activeCount := 0
	for _, c := range all {
		if c.Status == model.CalibrationActive {
			activeCount++
		}
	}
	if activeCount != 1 {
		t.Fatalf("expected exactly one active calibration, got %d", activeCount)
	}
}

func TestTimelinePersistenceRoundTrip(t *testing.T) {
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	now := model.TimeNow()
	tl := &model.Timeline{ID: "tl1", Name: "case", Status: model.TimelineBuilding, CreatedAt: now, UpdatedAt: now}
	if err := st.SaveTimeline(tl); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateTimelineCursor("tl1", 50, model.TimelineReviewable); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetTimeline("tl1")
	if err != nil || got.Status != model.TimelineReviewable || got.ArtifactCursor != 50 {
		t.Fatalf("timeline round trip failed: %+v err=%v", got, err)
	}

	rpt := &model.Report{ID: "r1", TimelineID: "tl1", Version: 1, Status: model.ReportFrozen,
		ClockSnapshot: []model.ClockSnapshotEntry{{SourceID: "s1", CalibrationVersion: 1}},
		PublishedAt: now, CreatedAt: now}
	if err := st.SaveReport(rpt); err != nil {
		t.Fatal(err)
	}
	gotRpt, err := st.GetReport("r1")
	if err != nil || len(gotRpt.ClockSnapshot) != 1 || gotRpt.ClockSnapshot[0].SourceID != "s1" {
		t.Fatalf("report round trip failed: %+v err=%v", gotRpt, err)
	}
}
