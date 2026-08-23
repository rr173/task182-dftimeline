package source

import (
	"path/filepath"
	"testing"
	"task182-dftimeline/internal/model"
	"task182-dftimeline/internal/store"
)

func TestFirstCalibrationStartsAtVersionOne(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "case.db")); if err != nil { t.Fatal(err) }; defer st.Close()
	svc:=NewService(st); src,err:=svc.CreateSource(CreateSourceInput{Name:"disk",SourceType:model.SourceFilesystem});if err!=nil{t.Fatal(err)}
	cal,err:=svc.SubmitCalibration(src.ID,SubmitCalibrationInput{});if err!=nil{t.Fatal(err)};if cal.Version!=1{t.Fatalf("expected version 1, got %d",cal.Version)}
}
