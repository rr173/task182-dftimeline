package httpapi

import (
	"net/http"

	"task182-dftimeline/internal/report"
)

func (s *Server) handleCreateTimeline(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	tl, err := s.app.Timelines.CreateTimeline(body.Name)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, tl)
}

func (s *Server) handleListTimelines(w http.ResponseWriter, _ *http.Request) {
	tls, err := s.app.Timelines.ListTimelines()
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"timelines": tls, "count": len(tls)})
}

func (s *Server) handleGetTimeline(w http.ResponseWriter, r *http.Request) {
	tl, err := s.app.Timelines.GetTimeline(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tl)
}

func (s *Server) handleBuildTimeline(w http.ResponseWriter, r *http.Request) {
	res, err := s.app.Timelines.BuildTimeline(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleListConflicts(w http.ResponseWriter, r *http.Request) {
	cores, err := s.app.Timelines.ListConflictCores(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"conflicts": cores, "count": len(cores)})
}

func (s *Server) handleGetConflict(w http.ResponseWriter, r *http.Request) {
	core, err := s.app.Timelines.GetConflictCore(r.PathValue("cid"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, core)
}

func (s *Server) handleListAdjudications(w http.ResponseWriter, r *http.Request) {
	adjs, err := s.app.Reports.ListAdjudications(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"adjudications": adjs, "count": len(adjs)})
}

func (s *Server) handleAdjudicate(w http.ResponseWriter, r *http.Request) {
	var in report.AdjudicateInput
	if !decodeBody(w, r, &in) {
		return
	}
	ad, err := s.app.Reports.Adjudicate(r.PathValue("id"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, ad)
}

func (s *Server) handlePublishTimeline(w http.ResponseWriter, r *http.Request) {
	rpt, err := s.app.Timelines.PublishTimeline(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, rpt)
}

func (s *Server) handleListTimelineReports(w http.ResponseWriter, r *http.Request) {
	rpts, err := s.app.Reports.ListReportsByTimeline(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reports": rpts, "count": len(rpts)})
}
