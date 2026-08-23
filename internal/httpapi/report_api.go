package httpapi

import "net/http"

func (s *Server) handleListReports(w http.ResponseWriter, _ *http.Request) {
	rpts, err := s.app.Reports.ListReports()
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reports": rpts, "count": len(rpts)})
}

func (s *Server) handleGetReport(w http.ResponseWriter, r *http.Request) {
	rpt, err := s.app.Reports.GetReport(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rpt)
}
