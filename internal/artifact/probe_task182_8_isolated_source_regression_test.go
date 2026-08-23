package artifact

import (
	"errors"
	"path/filepath"
	"testing"
	"task182-dftimeline/internal/model"
	"task182-dftimeline/internal/source"
	"task182-dftimeline/internal/store"
)

func TestIsolatedSourceCannotRegisterArtifact(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "case.db")); if err != nil { t.Fatal(err) }; defer st.Close()
	ss:=source.NewService(st); src,err:=ss.CreateSource(source.CreateSourceInput{Name:"disk",SourceType:model.SourceFilesystem});if err!=nil{t.Fatal(err)};if _,err=ss.IsolateSource(src.ID, "tamper");err!=nil{t.Fatal(err)}
	_,_,err=NewService(st).RegisterArtifact(RegisterInput{SourceID:src.ID,ArtifactType:model.ArtifactFile,PathName:"/x",RawTimestamp:"2026-01-01T00:00:00Z",Content:"x"})
	if err==nil || !errors.Is(err,model.ErrConflict){t.Fatalf("expected conflict, got %v",err)}
}
