// Package httpapi 提供 JSON HTTP 接口，统一错误映射。
package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"task182-dftimeline/internal/model"
	"task182-dftimeline/internal/service"
)

// Server 持有业务编排引用并注册路由。
type Server struct {
	app *service.App
	mux *http.ServeMux
}

// New 构造 HTTP 服务器。
func New(app *service.App) *Server {
	s := &Server{app: app, mux: http.NewServeMux()}
	s.routes()
	return s
}

// Handler 返回 HTTP 处理器。
func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) routes() {
	m := s.mux
	m.HandleFunc("GET /api/health", s.handleHealth)
	m.HandleFunc("GET /api/stats", s.handleStats)
	m.HandleFunc("POST /api/selfcheck", s.handleSelfCheck)

	// sources
	m.HandleFunc("POST /api/sources", s.handleCreateSource)
	m.HandleFunc("GET /api/sources", s.handleListSources)
	m.HandleFunc("GET /api/sources/{id}", s.handleGetSource)
	m.HandleFunc("POST /api/sources/{id}/calibrations", s.handleSubmitCalibration)
	m.HandleFunc("GET /api/sources/{id}/calibrations", s.handleListCalibrations)
	m.HandleFunc("POST /api/sources/{id}/mark-unknown", s.handleMarkUnknown)
	m.HandleFunc("POST /api/sources/{id}/isolate", s.handleIsolateSource)
	m.HandleFunc("POST /api/sources/{id}/reactivate", s.handleReactivateSource)
	m.HandleFunc("GET /api/sources/{id}/artifacts", s.handleListSourceArtifacts)

	// artifacts
	m.HandleFunc("POST /api/artifacts", s.handleRegisterArtifact)
	m.HandleFunc("GET /api/artifacts", s.handleListArtifacts)
	m.HandleFunc("GET /api/artifacts/{id}", s.handleGetArtifact)
	m.HandleFunc("POST /api/artifacts/{id}/suspicious", s.handleMarkSuspicious)
	m.HandleFunc("POST /api/artifacts/{id}/adopt", s.handleAdoptArtifact)

	// constraints
	m.HandleFunc("POST /api/constraints", s.handleCreateConstraint)
	m.HandleFunc("GET /api/constraints", s.handleListConstraints)
	m.HandleFunc("GET /api/constraints/{id}", s.handleGetConstraint)
	m.HandleFunc("POST /api/constraints/{id}/verify", s.handleVerifyConstraint)

	// timelines
	m.HandleFunc("POST /api/timelines", s.handleCreateTimeline)
	m.HandleFunc("GET /api/timelines", s.handleListTimelines)
	m.HandleFunc("GET /api/timelines/{id}", s.handleGetTimeline)
	m.HandleFunc("POST /api/timelines/{id}/build", s.handleBuildTimeline)
	m.HandleFunc("GET /api/timelines/{id}/conflicts", s.handleListConflicts)
	m.HandleFunc("GET /api/timelines/{id}/conflicts/{cid}", s.handleGetConflict)
	m.HandleFunc("GET /api/timelines/{id}/adjudications", s.handleListAdjudications)
	m.HandleFunc("POST /api/timelines/{id}/adjudications", s.handleAdjudicate)
	m.HandleFunc("POST /api/timelines/{id}/publish", s.handlePublishTimeline)
	m.HandleFunc("GET /api/timelines/{id}/reports", s.handleListTimelineReports)

	// reports
	m.HandleFunc("GET /api/reports", s.handleListReports)
	m.HandleFunc("GET /api/reports/{id}", s.handleGetReport)
}

// ---------------------------------------------------------------------------
// 通用辅助
// ---------------------------------------------------------------------------

type errorBody struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("encode response: %v", err)
	}
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	code := "bad_request"
	switch {
	case errors.Is(err, model.ErrNotFound):
		status, code = http.StatusNotFound, "not_found"
	case errors.Is(err, model.ErrConflict):
		status, code = http.StatusConflict, "conflict"
	case errors.Is(err, model.ErrInvalidState):
		status, code = http.StatusConflict, "invalid_state"
	case errors.Is(err, model.ErrTimeZoneMissing):
		status, code = http.StatusUnprocessableEntity, "timezone_missing"
	case errors.Is(err, model.ErrIntervalReversed):
		status, code = http.StatusUnprocessableEntity, "interval_reversed"
	case errors.Is(err, model.ErrHashMismatch):
		status, code = http.StatusConflict, "hash_mismatch"
	case errors.Is(err, model.ErrCrossCase):
		status, code = http.StatusForbidden, "cross_case"
	case errors.Is(err, model.ErrReportImmutable):
		status, code = http.StatusConflict, "report_immutable"
	}
	writeJSON(w, status, errorBody{Error: err.Error(), Code: code})
}

func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "invalid JSON body: " + err.Error(), Code: "bad_request"})
		return false
	}
	return true
}

// ---------------------------------------------------------------------------
// 基础端点
// ---------------------------------------------------------------------------

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "dftimeline"})
}

func (s *Server) handleStats(w http.ResponseWriter, _ *http.Request) {
	st, err := s.app.ComputeStats()
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleSelfCheck(w http.ResponseWriter, _ *http.Request) {
	// 使用独立临时库执行自检，不影响在线数据。
	out, err := service.RunSelfCheck("")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error(), Code: "selfcheck_failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "output": out})
}
