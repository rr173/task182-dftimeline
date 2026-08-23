package report

import (
	"path/filepath"
	"testing"
	"task182-dftimeline/internal/model"
	"task182-dftimeline/internal/store"
)

func TestFrozenReportContainsArtifactSnapshot(t *testing.T) {
	st,err:=store.Open(filepath.Join(t.TempDir(),"case.db"));if err!=nil{t.Fatal(err)};defer st.Close()
	now:=model.TimeNow();if err=st.SaveTimeline(&model.Timeline{ID:"tl1",Name:"case",Status:model.TimelineReviewable,CreatedAt:now,UpdatedAt:now});err!=nil{t.Fatal(err)}
	if err=st.SaveArtifact(&model.Artifact{ID:"art1",SourceID:"src1",PathName:"/evidence",RawTimestamp:"2026-01-01T00:00:00Z",ContentSHA256:"hash1",Status:model.ArtifactNormalized,EarliestMS:1,LatestMS:2,CreatedAt:now,UpdatedAt:now});err!=nil{t.Fatal(err)}
	r,err:=NewService(st).FreezeReport("tl1");if err!=nil{t.Fatal(err)};if len(r.ArtifactSnapshot)!=1||r.ArtifactSnapshot[0].ArtifactID!="art1"{t.Fatalf("expected artifact snapshot, got %#v",r.ArtifactSnapshot)}
}
