package source

import (
	"errors"
	"path/filepath"
	"testing"

	"task182-dftimeline/internal/model"
	"task182-dftimeline/internal/store"
)

func TestNegativeUncertaintyIsRejected(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "case.db"))
	if err != nil { t.Fatal(err) }
	defer st.Close()
	svc := NewService(st)
	src, err := svc.CreateSource(CreateSourceInput{Name: "disk", SourceType: model.SourceFilesystem})
	if err != nil { t.Fatal(err) }
	_, err = svc.SubmitCalibration(src.ID, SubmitCalibrationInput{UncertaintyMillis: -1})
	if err == nil || !errors.Is(err, model.ErrBadRequest) { t.Fatalf("expected bad request, got %v", err) }
}
