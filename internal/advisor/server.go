package advisor

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"
)

type Server struct {
	svc *Service
	mux *http.ServeMux
}

func NewServer(svc *Service) *Server {
	s := &Server{svc: svc, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) routes() {
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("POST /advisor/sessions/register", s.handleRegister)
	s.mux.HandleFunc("POST /advisor/sessions/{id}/finalize", s.handleFinalize)
	s.mux.HandleFunc("GET /advisor/sessions/{id}/analyses", s.handleListAnalyses)
	s.mux.HandleFunc("GET /advisor/sessions/{id}/analyses/{reportID}", s.handleGetAnalysis)
	s.mux.HandleFunc("GET /advisor/sessions/{id}/strategy", s.handleStrategy)
	s.mux.HandleFunc("GET /advisor/knowledge", s.handleKnowledge)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := s.svc.Register(r.Context(), req); err != nil {
		writeErr(w, statusFromErr(err), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "registered"})
}

func (s *Server) handleFinalize(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.svc.Finalize(r.Context(), id); err != nil {
		writeErr(w, statusFromErr(err), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "finalizing"})
}

func (s *Server) handleListAnalyses(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tf := Timeframe(r.URL.Query().Get("tf"))
	if tf == "" {
		tf = TF5m
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	reports, err := s.svc.store.ListReports(r.Context(), id, tf, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if reports == nil {
		reports = []AnalysisReport{}
	}
	writeJSON(w, http.StatusOK, reports)
}

func (s *Server) handleGetAnalysis(w http.ResponseWriter, r *http.Request) {
	rep, err := s.svc.store.GetReport(r.Context(), r.PathValue("reportID"))
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	if rep.SessionID != r.PathValue("id") {
		writeErr(w, http.StatusNotFound, errors.New("not found"))
		return
	}
	writeJSON(w, http.StatusOK, rep)
}

func (s *Server) handleStrategy(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	syn, err := s.svc.store.GetSynthesis(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	reports, _ := s.svc.store.ListReports(r.Context(), id, TFStrategy, 5)
	resp := map[string]any{
		"synthesis": syn,
		"reports":   reports,
	}
	if syn == nil && len(reports) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"synthesis": nil, "reports": []AnalysisReport{}})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleKnowledge(w http.ResponseWriter, r *http.Request) {
	uid := r.URL.Query().Get("instrument_uid")
	if uid == "" {
		writeErr(w, http.StatusBadRequest, errBadRequest("instrument_uid required"))
		return
	}
	from := time.Now().UTC().Add(-7 * 24 * time.Hour)
	to := time.Now().UTC()
	if v := r.URL.Query().Get("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			from = t.UTC()
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			to = t.UTC()
		}
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	reports, err := s.svc.store.SearchKnowledge(r.Context(), uid, from, to, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if reports == nil {
		reports = []AnalysisReport{}
	}
	writeJSON(w, http.StatusOK, reports)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, err error) {
	msg := "error"
	if err != nil {
		msg = err.Error()
	}
	writeJSON(w, code, map[string]string{"error": msg})
}

func statusFromErr(err error) int {
	if IsBadRequest(err) {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

// ListenAndServe starts HTTP server until ctx cancelled.
func ListenAndServe(ctx context.Context, addr string, svc *Service) error {
	srv := &http.Server{
		Addr:    addr,
		Handler: NewServer(svc).Handler(),
	}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	go svc.Run(ctx)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
