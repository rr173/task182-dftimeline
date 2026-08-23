package httpapi

import (
	"net/http"

	"task182-dftimeline/internal/source"
)

func (s *Server) handleCreateSource(w http.ResponseWriter, r *http.Request) {
	var in source.CreateSourceInput
	if !decodeBody(w, r, &in) {
		return
	}
	src, err := s.app.Sources.CreateSource(in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, src)
}

func (s *Server) handleListSources(w http.ResponseWriter, _ *http.Request) {
	srcs, err := s.app.Sources.ListSources()
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sources": srcs, "count": len(srcs)})
}

func (s *Server) handleGetSource(w http.ResponseWriter, r *http.Request) {
	src, err := s.app.Sources.GetSource(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, src)
}

func (s *Server) handleSubmitCalibration(w http.ResponseWriter, r *http.Request) {
	var in source.SubmitCalibrationInput
	if !decodeBody(w, r, &in) {
		return
	}
	cal, err := s.app.Sources.SubmitCalibration(r.PathValue("id"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, cal)
}

func (s *Server) handleListCalibrations(w http.ResponseWriter, r *http.Request) {
	cals, err := s.app.Sources.ListCalibrations(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"calibrations": cals, "count": len(cals)})
}

func (s *Server) handleMarkUnknown(w http.ResponseWriter, r *http.Request) {
	src, err := s.app.Sources.MarkUnknownOffset(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, src)
}

func (s *Server) handleIsolateSource(w http.ResponseWriter, r *http.Request) {
	src, err := s.app.Sources.IsolateSource(r.PathValue("id"), "")
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, src)
}

func (s *Server) handleReactivateSource(w http.ResponseWriter, r *http.Request) {
	src, err := s.app.Sources.ReactivateSource(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, src)
}

func (s *Server) handleListSourceArtifacts(w http.ResponseWriter, r *http.Request) {
	arts, err := s.app.Artifacts.ListArtifactsBySource(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"artifacts": arts, "count": len(arts)})
}
