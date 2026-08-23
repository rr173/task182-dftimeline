package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"task182-dftimeline/internal/artifact"
	"task182-dftimeline/internal/constraint"
	"task182-dftimeline/internal/model"
	"task182-dftimeline/internal/source"
	"task182-dftimeline/internal/store"
)

// SelfCheck 是 --smoke-test 的执行体：真实创建数据、调用核心闭环、
// 关闭并重开数据库验证持久化恢复，最后返回人类可读摘要。
// 任何断言失败都返回错误，进程以非零退出码结束。
func (a *App) SelfCheck(dbPath string) (string, error) {
	if err := a.Recover(); err != nil {
		return "", fmt.Errorf("recover failed: %w", err)
	}
	var sb strings.Builder
	write := func(format string, args ...any) { _, _ = fmt.Fprintf(&sb, format+"\n", args...) }

	// ---- 场景 A：两台设备偏差校正后，“下载先于打开”成立 ----
	phone, err := a.Sources.CreateSource(source.CreateSourceInput{Name: "phone-device", SourceType: model.SourceDevice})
	if err != nil {
		return "", err
	}
	laptop, err := a.Sources.CreateSource(source.CreateSourceInput{Name: "laptop", SourceType: model.SourceFilesystem})
	if err != nil {
		return "", err
	}
	write("created sources: %s (phone), %s (laptop)", phone.ID, laptop.ID)

	if _, err := a.Sources.SubmitCalibration(phone.ID, source.SubmitCalibrationInput{
		TZOffsetMinutes: 480, BiasMilliseconds: 0, UncertaintyMillis: 5000}); err != nil {
		return "", err
	}
	// laptop 时钟偏快 30 秒。
	if _, err := a.Sources.SubmitCalibration(laptop.ID, source.SubmitCalibrationInput{
		TZOffsetMinutes: 480, BiasMilliseconds: 30_000, UncertaintyMillis: 5000}); err != nil {
		return "", err
	}
	write("calibrated phone (bias 0ms) and laptop (bias +30000ms)")

	dl, _, err := a.Artifacts.RegisterArtifact(artifact.RegisterInput{
		SourceID: phone.ID, ArtifactType: model.ArtifactFile, PathName: "download_report.pdf",
		RawTimestamp: "2026-08-20T10:00:00+08:00", Content: "downloaded report bytes v1"})
	if err != nil {
		return "", err
	}
	op, _, err := a.Artifacts.RegisterArtifact(artifact.RegisterInput{
		SourceID: laptop.ID, ArtifactType: model.ArtifactFile, PathName: "open_report.pdf",
		RawTimestamp: "2026-08-20T10:00:45+08:00", Content: "opened report at 10:00:45 laptop local"})
	if err != nil {
		return "", err
	}
	write("registered artifacts: %s (download), %s (open)", dl.ID, op.ID)
	if dl.EarliestMS >= dl.LatestMS {
		return "", fmt.Errorf("download interval invalid: [%d,%d]", dl.EarliestMS, dl.LatestMS)
	}
	if dl.LatestMS > op.EarliestMS {
		return "", fmt.Errorf("expected download before open after calibration, got dl.latest=%d > op.earliest=%d",
			dl.LatestMS, op.EarliestMS)
	}

	con, err := a.Constraints.CreateConstraint(constraint.CreateInput{BeforeArtifactID: dl.ID, AfterArtifactID: op.ID})
	if err != nil {
		return "", err
	}
	verified, err := a.Constraints.VerifyConstraint(con.ID)
	if err != nil {
		return "", err
	}
	if verified.Status != model.ConstraintSatisfied {
		return "", fmt.Errorf("expected satisfied constraint, got %s (%s)", verified.Status, verified.VerdictReason)
	}
	write("constraint download-before-open verified: %s", verified.Status)

	tl, err := a.Timelines.CreateTimeline("case-20260820")
	if err != nil {
		return "", err
	}
	for {
		res, err := a.Timelines.BuildTimeline(tl.ID)
		if err != nil {
			return "", err
		}
		if res.Done {
			break
		}
	}
	curTL, err := a.Timelines.GetTimeline(tl.ID)
	if err != nil {
		return "", err
	}
	if curTL.Status != model.TimelineReviewable {
		return "", fmt.Errorf("expected reviewable timeline, got %s", curTL.Status)
	}
	write("timeline built: %s (status=%s)", tl.ID, curTL.Status)

	rpt1, err := a.Timelines.PublishTimeline(tl.ID)
	if err != nil {
		return "", err
	}
	if len(rpt1.ClockSnapshot) != 2 {
		return "", fmt.Errorf("expected 2 clock models in report, got %d", len(rpt1.ClockSnapshot))
	}
	// 冻结报告必须包含发布时刻的工件快照，不能遗漏证据内容。
	if len(rpt1.ArtifactSnapshot) != 2 {
		return "", fmt.Errorf("expected 2 artifact snapshots in report, got %d", len(rpt1.ArtifactSnapshot))
	}
	if !hasArtifactSnapshot(rpt1.ArtifactSnapshot, dl.ID) || !hasArtifactSnapshot(rpt1.ArtifactSnapshot, op.ID) {
		return "", fmt.Errorf("report artifact snapshot missing registered artifacts: %+v", rpt1.ArtifactSnapshot)
	}
	write("published report v%d bound to %d clock models and %d artifacts",
		rpt1.Version, len(rpt1.ClockSnapshot), len(rpt1.ArtifactSnapshot))

	// ---- 重启恢复：关闭并重开数据库，验证数据仍在 ----
	if err := a.Store.Close(); err != nil {
		return "", err
	}
	st2, err := store.Open(dbPath)
	if err != nil {
		return "", fmt.Errorf("reopen db: %w", err)
	}
	defer st2.Close()
	a2 := New(st2)
	if err := a2.Recover(); err != nil {
		return "", fmt.Errorf("recover after reopen: %w", err)
	}
	restoredTL, err := a2.Timelines.GetTimeline(tl.ID)
	if err != nil {
		return "", fmt.Errorf("timeline lost after reopen: %w", err)
	}
	if restoredTL.Status != model.TimelinePublished {
		return "", fmt.Errorf("timeline status changed after reopen: %s", restoredTL.Status)
	}
	restoredArt, err := a2.Artifacts.GetArtifact(dl.ID)
	if err != nil {
		return "", fmt.Errorf("artifact lost after reopen: %w", err)
	}
	if restoredArt.Status != model.ArtifactNormalized {
		return "", fmt.Errorf("artifact status changed after reopen: %s", restoredArt.Status)
	}
	restoredRpt, err := a2.Reports.GetReport(rpt1.ID)
	if err != nil {
		return "", fmt.Errorf("report lost after reopen: %w", err)
	}
	if restoredRpt.Status != model.ReportFrozen || restoredRpt.Version != rpt1.Version {
		return "", fmt.Errorf("report changed after reopen: %s v%d", restoredRpt.Status, restoredRpt.Version)
	}
	if len(restoredRpt.ArtifactSnapshot) != len(rpt1.ArtifactSnapshot) {
		return "", fmt.Errorf("report artifact snapshot lost after reopen: got %d want %d",
			len(restoredRpt.ArtifactSnapshot), len(rpt1.ArtifactSnapshot))
	}
	write("reopened db: timeline/artifacts/report all restored")

	// ---- 场景 B：加入伪造的创建时间，约束形成环，返回三个工件 ----
	forged, _, err := a2.Artifacts.RegisterArtifact(artifact.RegisterInput{
		SourceID: phone.ID, ArtifactType: model.ArtifactFile, PathName: "create_report.pdf",
		RawTimestamp: "2026-08-20T10:01:00+08:00", Content: "forged: claims creation after download"})
	if err != nil {
		return "", err
	}
	// B before forged（打开先于创建，异常），forged before A（伪造创建先于下载）=> 环 A->B->forged->A
	if _, err := a2.Constraints.CreateConstraint(constraint.CreateInput{BeforeArtifactID: op.ID, AfterArtifactID: forged.ID}); err != nil {
		return "", err
	}
	if _, err := a2.Constraints.CreateConstraint(constraint.CreateInput{BeforeArtifactID: forged.ID, AfterArtifactID: dl.ID}); err != nil {
		return "", err
	}
	tl2, err := a2.Timelines.CreateTimeline("case-20260820-forged")
	if err != nil {
		return "", err
	}
	for {
		res, err := a2.Timelines.BuildTimeline(tl2.ID)
		if err != nil {
			return "", err
		}
		if res.Done {
			break
		}
	}
	curTL2, err := a2.Timelines.GetTimeline(tl2.ID)
	if err != nil {
		return "", err
	}
	if curTL2.Status != model.TimelineConflicted {
		return "", fmt.Errorf("expected conflicted timeline, got %s", curTL2.Status)
	}
	cores, err := a2.Timelines.ListConflictCores(tl2.ID)
	if err != nil || len(cores) == 0 {
		return "", fmt.Errorf("expected at least one conflict core, got %d (err=%v)", len(cores), err)
	}
	if len(cores[0].ArtifactIDs) != 3 {
		return "", fmt.Errorf("expected 3 artifacts in minimal conflict cycle, got %v", cores[0].ArtifactIDs)
	}
	write("conflict cycle detected: %d artifacts, attribution=%s", len(cores[0].ArtifactIDs), cores[0].Attribution)

	// ---- 场景 C：发布后更新源校准，只产生替代报告，旧报告不可变 ----
	rpt2, err := a2.Timelines.PublishTimeline(tl2.ID)
	if err != nil {
		return "", err
	}
	if rpt2.Version != 1 { // tl2 是独立时间线，版本从 1 开始
		return "", fmt.Errorf("expected report v1 for second timeline, got v%d", rpt2.Version)
	}
	// 冲突报告同样必须冻结全部工件快照证据。
	if len(rpt2.ArtifactSnapshot) != 3 {
		return "", fmt.Errorf("expected 3 artifact snapshots in conflict report, got %d", len(rpt2.ArtifactSnapshot))
	}
	if _, err := a2.Sources.SubmitCalibration(laptop.ID, source.SubmitCalibrationInput{
		TZOffsetMinutes: 480, BiasMilliseconds: 0, UncertaintyMillis: 2000}); err != nil {
		return "", err
	}
	// 同名案件重建时间线并发布：替代报告产生，旧时间线被替代。
	tl3, err := a2.Timelines.CreateTimeline("case-20260820")
	if err != nil {
		return "", err
	}
	for {
		res, err := a2.Timelines.BuildTimeline(tl3.ID)
		if err != nil {
			return "", err
		}
		if res.Done {
			break
		}
	}
	if _, err := a2.Timelines.PublishTimeline(tl3.ID); err != nil {
		return "", err
	}
	if err := a2.SupersedeOnPublish(tl3.ID, "case-20260820"); err != nil {
		return "", err
	}
	// 旧报告 rpt1 仍冻结且未变化。
	checkRpt, err := a2.Reports.GetReport(rpt1.ID)
	if err != nil {
		return "", err
	}
	if checkRpt.Status != model.ReportFrozen {
		return "", fmt.Errorf("old report mutated: %s", checkRpt.Status)
	}
	oldTL, err := a2.Timelines.GetTimeline(tl.ID)
	if err != nil {
		return "", err
	}
	if oldTL.Status != model.TimelineSuperseded {
		return "", fmt.Errorf("expected old timeline superseded, got %s", oldTL.Status)
	}
	write("recalibration produced replacement report; old report v%d stays immutable", checkRpt.Version)

	// ---- 汇总 ----
	st, err := a2.ComputeStats()
	if err != nil {
		return "", err
	}
	write("stats: sources=%d artifacts=%d constraints=%d timelines=%d conflicts=%d reports=%d",
		st.Sources, st.Artifacts, st.Constraints, st.Timelines, st.Conflicts, st.Reports)
	write("SELFCHECK OK")
	return sb.String(), nil
}

// hasArtifactSnapshot 报告快照是否包含指定工件的证据记录。
func hasArtifactSnapshot(snap []model.ArtifactSnapshotEntry, artifactID string) bool {
	for _, e := range snap {
		if e.ArtifactID == artifactID {
			return true
		}
	}
	return false
}

// RunSelfCheck 以独立数据库文件执行自检（供 main --smoke-test 调用）。
func RunSelfCheck(dbPath string) (string, error) {
	if dbPath == "" || dbPath == ":memory:" {
		dir, err := os.MkdirTemp("", "dftimeline-smoke-")
		if err != nil {
			return "", err
		}
		defer os.RemoveAll(dir)
		dbPath = filepath.Join(dir, "smoke.db")
	}
	st, err := store.Open(dbPath)
	if err != nil {
		return "", err
	}
	defer st.Close()
	return New(st).SelfCheck(dbPath)
}
