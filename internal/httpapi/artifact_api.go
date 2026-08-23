package httpapi

import (
	"net/http"

	"task182-dftimeline/internal/artifact"
)

func (s *Server) handleRegisterArtifact(w http.ResponseWriter, r *http.Request) {
	var in artifact.RegisterInput
	if !decodeBody(w, r, &in) {
		return
	}
	art, replayed, err := s.app.Artifacts.RegisterArtifact(in)
	if err != nil {
		writeError(w, err)
		return
	}
	status := http.StatusCreated
	if replayed {
		status = http.StatusOK
	}
	writeJSON(w, status, art)
}

func (s *Server) handleListArtifacts(w http.ResponseWriter, r *http.Request) {
	arts, err := s.app.Artifacts.ListArtifacts(r.URL.Query().Get("status"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"artifacts": arts, "count": len(arts)})
}

func (s *Server) handleGetArtifact(w http.ResponseWriter, r *http.Request) {
	art, err := s.app.Artifacts.GetArtifact(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, art)
}

func (s *Server) handleMarkSuspicious(w http.ResponseWriter, r *http.Request) {
	art, err := s.app.Artifacts.MarkSuspicious(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, art)
}

func (s *Server) handleAdoptArtifact(w http.ResponseWriter, r *http.Request) {
	art, err := s.app.Artifacts.AdoptArtifact(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, art)
}
