package constraint

import (
	"path/filepath"
	"testing"
	"task182-dftimeline/internal/artifact"
	"task182-dftimeline/internal/model"
	"task182-dftimeline/internal/source"
	"task182-dftimeline/internal/store"
)

func TestTouchingIntervalsAreSatisfied(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "case.db")); if err != nil { t.Fatal(err) }; defer st.Close()
	ss := source.NewService(st); src, err := ss.CreateSource(source.CreateSourceInput{Name:"disk", SourceType:model.SourceFilesystem}); if err != nil { t.Fatal(err) }; if _,err=ss.SubmitCalibration(src.ID,source.SubmitCalibrationInput{}); err!=nil {t.Fatal(err)}
	as:=artifact.NewService(st); a,_,err:=as.RegisterArtifact(artifact.RegisterInput{SourceID:src.ID,ArtifactType:model.ArtifactFile,PathName:"/a",RawTimestamp:"2026-01-01T00:00:00Z",Content:"a"});if err!=nil{t.Fatal(err)}; b,_,err:=as.RegisterArtifact(artifact.RegisterInput{SourceID:src.ID,ArtifactType:model.ArtifactFile,PathName:"/b",RawTimestamp:"2026-01-01T00:00:00Z",Content:"b"});if err!=nil{t.Fatal(err)}
	cs:=NewService(st); c,err:=cs.CreateConstraint(CreateInput{BeforeArtifactID:a.ID,AfterArtifactID:b.ID});if err!=nil{t.Fatal(err)}; got,err:=cs.VerifyConstraint(c.ID);if err!=nil{t.Fatal(err)};if got.Status!=model.ConstraintSatisfied{t.Fatalf("expected satisfied, got %s",got.Status)}
}
