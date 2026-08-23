package constraint

import (
	"errors"
	"path/filepath"
	"testing"
	"task182-dftimeline/internal/artifact"
	"task182-dftimeline/internal/model"
	"task182-dftimeline/internal/source"
	"task182-dftimeline/internal/store"
)

func TestSelfLoopConstraintIsRejected(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "case.db")); if err != nil { t.Fatal(err) }; defer st.Close()
	ss := source.NewService(st); src, err := ss.CreateSource(source.CreateSourceInput{Name:"disk", SourceType:model.SourceFilesystem}); if err != nil { t.Fatal(err) }
	if _, err = ss.SubmitCalibration(src.ID, source.SubmitCalibrationInput{}); err != nil { t.Fatal(err) }
	as := artifact.NewService(st); a, _, err := as.RegisterArtifact(artifact.RegisterInput{SourceID:src.ID, ArtifactType:model.ArtifactFile, PathName:"/x", RawTimestamp:"2026-01-01T00:00:00Z", Content:"x"}); if err != nil { t.Fatal(err) }
	_, err = NewService(st).CreateConstraint(CreateInput{BeforeArtifactID:a.ID, AfterArtifactID:a.ID})
	if err == nil || !errors.Is(err, model.ErrBadRequest) { t.Fatalf("expected bad request, got %v", err) }
}
