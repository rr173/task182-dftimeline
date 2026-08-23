package artifact

import (
	"path/filepath"
	"testing"
	"time"

	"task182-dftimeline/internal/model"
	"task182-dftimeline/internal/source"
	"task182-dftimeline/internal/store"
)

func TestCalibrationBiasIsSubtractedFromTimestamp(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "case.db")); if err != nil { t.Fatal(err) }; defer st.Close()
	ss := source.NewService(st)
	src, err := ss.CreateSource(source.CreateSourceInput{Name: "disk", SourceType: model.SourceFilesystem}); if err != nil { t.Fatal(err) }
	if _, err = ss.SubmitCalibration(src.ID, source.SubmitCalibrationInput{BiasMilliseconds: 1000, UncertaintyMillis: 0}); err != nil { t.Fatal(err) }
	as := NewService(st)
	a, _, err := as.RegisterArtifact(RegisterInput{SourceID: src.ID, ArtifactType: model.ArtifactFile, PathName: "/x", RawTimestamp: "2026-01-01T00:00:00Z", Content: "x"}); if err != nil { t.Fatal(err) }
	raw, _ := time.Parse(time.RFC3339, "2026-01-01T00:00:00Z")
	if a.EarliestMS != raw.UnixMilli()-1000 { t.Fatalf("expected %d, got %d", raw.UnixMilli()-1000, a.EarliestMS) }
}
