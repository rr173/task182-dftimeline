package httpapi

import (
	"net/http"

	"task182-dftimeline/internal/constraint"
)

func (s *Server) handleCreateConstraint(w http.ResponseWriter, r *http.Request) {
	var in constraint.CreateInput
	if !decodeBody(w, r, &in) {
		return
	}
	c, err := s.app.Constraints.CreateConstraint(in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func (s *Server) handleListConstraints(w http.ResponseWriter, _ *http.Request) {
	cons, err := s.app.Constraints.ListConstraints()
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"constraints": cons, "count": len(cons)})
}

func (s *Server) handleGetConstraint(w http.ResponseWriter, r *http.Request) {
	c, err := s.app.Constraints.GetConstraint(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) handleVerifyConstraint(w http.ResponseWriter, r *http.Request) {
	c, err := s.app.Constraints.VerifyConstraint(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}
