// Package service 编排各业务包，暴露聚合操作、启动恢复与统计。
package service

import (
	"task182-dftimeline/internal/artifact"
	"task182-dftimeline/internal/constraint"
	"task182-dftimeline/internal/model"
	"task182-dftimeline/internal/report"
	"task182-dftimeline/internal/source"
	"task182-dftimeline/internal/store"
	"task182-dftimeline/internal/timeline"
)

// App 聚合全部业务服务。
type App struct {
	Store      *store.Store
	Sources    *source.Service
	Artifacts  *artifact.Service
	Constraints *constraint.Service
	Timelines  *timeline.Service
	Reports    *report.Service
}

// New 组装 App。
func New(st *store.Store) *App {
	srcSvc := source.NewService(st)
	artSvc := artifact.NewService(st)
	conSvc := constraint.NewService(st)
	rptSvc := report.NewService(st)
	tlSvc := timeline.NewService(st, conSvc, rptSvc)
	return &App{
		Store:       st,
		Sources:     srcSvc,
		Artifacts:   artSvc,
		Constraints: conSvc,
		Timelines:   tlSvc,
		Reports:     rptSvc,
	}
}

// Recover 启动恢复：归一化遗留 pending 工件、验证遗留 pending 约束、
// 把处于 building 的时间线维持为 building（游标已持久化，可继续构建）。
func (a *App) Recover() error {
	if _, err := a.Artifacts.NormalizeAllPending(); err != nil {
		return err
	}
	if _, err := a.Constraints.VerifyAllPending(); err != nil {
		return err
	}
	return nil
}

// SupersedeOnPublish 在发布新时间线时，把同名的已发布时间线标记为被替代。
// 旧报告保持不可变，符合“发布报告后更新校准只产生替代报告”的不变量。
func (a *App) SupersedeOnPublish(newTimelineID, name string) error {
	tls, err := a.Store.ListTimelines()
	if err != nil {
		return err
	}
	for _, t := range tls {
		if t.ID == newTimelineID || t.Status != model.TimelinePublished {
			continue
		}
		if t.Name != name {
			continue
		}
		if err := a.Timelines.Supersede(t.ID, newTimelineID); err != nil {
			return err
		}
	}
	return nil
}

// Stats 聚合统计视图。
type Stats struct {
	Sources        int                       `json:"sources"`
	SourceStatus   map[model.SourceStatus]int `json:"source_status"`
	Artifacts      int                       `json:"artifacts"`
	ArtifactStatus map[model.ArtifactStatus]int `json:"artifact_status"`
	Constraints    int                       `json:"constraints"`
	ConstraintStatus map[model.ConstraintStatus]int `json:"constraint_status"`
	Timelines      int                       `json:"timelines"`
	Conflicts      int                       `json:"conflicts"`
	Reports        int                       `json:"reports"`
}

// ComputeStats 计算统计视图。
func (a *App) ComputeStats() (*Stats, error) {
	st := &Stats{
		SourceStatus:     map[model.SourceStatus]int{},
		ArtifactStatus:   map[model.ArtifactStatus]int{},
		ConstraintStatus: map[model.ConstraintStatus]int{},
	}
	srcs, err := a.Store.ListSources()
	if err != nil {
		return nil, err
	}
	st.Sources = len(srcs)
	for _, s := range srcs {
		st.SourceStatus[s.Status]++
	}
	artTotal, artByStatus, err := a.Store.CountArtifacts()
	if err != nil {
		return nil, err
	}
	st.Artifacts = artTotal
	st.ArtifactStatus = artByStatus
	conTotal, conByStatus, err := a.Store.CountConstraints()
	if err != nil {
		return nil, err
	}
	st.Constraints = conTotal
	st.ConstraintStatus = conByStatus
	if st.Timelines, err = a.Store.CountTimelines(); err != nil {
		return nil, err
	}
	if st.Conflicts, err = a.Store.CountConflictCores(); err != nil {
		return nil, err
	}
	reports, err := a.Store.ListReports()
	if err != nil {
		return nil, err
	}
	st.Reports = len(reports)
	return st, nil
}
