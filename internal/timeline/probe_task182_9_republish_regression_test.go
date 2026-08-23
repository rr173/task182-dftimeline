package timeline

import (
	"errors"
	"path/filepath"
	"testing"
	"task182-dftimeline/internal/constraint"
	"task182-dftimeline/internal/model"
	"task182-dftimeline/internal/report"
	"task182-dftimeline/internal/store"
)

func TestPublishedTimelineCannotBePublishedAgain(t *testing.T) {
	st,err:=store.Open(filepath.Join(t.TempDir(),"case.db"));if err!=nil{t.Fatal(err)};defer st.Close()
	tl:=&model.Timeline{ID:"tl-published",Name:"case",Status:model.TimelinePublished,CreatedAt:model.TimeNow(),UpdatedAt:model.TimeNow()};if err=st.SaveTimeline(tl);err!=nil{t.Fatal(err)}
	ts:=NewService(st,constraint.NewService(st),report.NewService(st));_,err=ts.PublishTimeline(tl.ID);if err==nil||!errors.Is(err,model.ErrInvalidState){t.Fatalf("expected invalid state, got %v",err)}
}
