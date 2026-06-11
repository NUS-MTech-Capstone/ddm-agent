package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/NUS-MTech-Capstone/ddm-agent/internal/execution"
	"github.com/NUS-MTech-Capstone/ddm-agent/internal/model"
	"github.com/NUS-MTech-Capstone/ddm-agent/internal/service"
)

type Server struct {
	mux             *http.ServeMux
	svc             *service.Service
	maxRequestBytes int64
}

type executeRequest struct {
	Command string `json:"command"`
}

func New(svc *service.Service, maxCommandBytes int) *Server {
	s := &Server{
		mux:             http.NewServeMux(),
		svc:             svc,
		maxRequestBytes: int64(maxCommandBytes + 1024),
	}
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	s.mux.HandleFunc("POST /execute", s.execute)
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s %s", r.Method, r.URL.Path)
	s.mux.ServeHTTP(w, r)
}

func (s *Server) execute(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	r.Body = http.MaxBytesReader(w, r.Body, s.maxRequestBytes)
	var req executeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	result, err := s.svc.Execute(r.Context(), model.ExecuteRequest{
		SessionID: r.Header.Get("X-DDM-Session-ID"),
		UserID:    r.Header.Get("X-DDM-User-ID"),
		HostID:    r.Header.Get("X-DDM-Host-ID"),
		CRID:      r.Header.Get("X-DDM-CR-ID"),
		Token:     bearerToken(r.Header.Get("Authorization")),
		Command:   req.Command,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUnauthorized):
			writeError(w, http.StatusForbidden, "session is not authorized")
		case errors.Is(err, execution.ErrTimedOut):
			writeError(w, http.StatusGatewayTimeout, err.Error())
		case errors.Is(err, service.ErrInvalidRequest):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			log.Printf(
				"CommandFailed sessionId=%s userId=%s hostId=%s crId=%s error=%v",
				r.Header.Get("X-DDM-Session-ID"),
				r.Header.Get("X-DDM-User-ID"),
				r.Header.Get("X-DDM-Host-ID"),
				r.Header.Get("X-DDM-CR-ID"),
				err,
			)
			writeError(w, http.StatusInternalServerError, "command execution failed")
		}
		return
	}
	log.Printf(
		"CommandExecuted sessionId=%s userId=%s hostId=%s crId=%s exitCode=%d",
		r.Header.Get("X-DDM-Session-ID"),
		r.Header.Get("X-DDM-User-ID"),
		r.Header.Get("X-DDM-Host-ID"),
		r.Header.Get("X-DDM-CR-ID"),
		result.ExitCode,
	)
	writeJSON(w, http.StatusOK, result)
}

func bearerToken(header string) string {
	const prefix = "Bearer "
	if strings.HasPrefix(header, prefix) {
		return strings.TrimSpace(strings.TrimPrefix(header, prefix))
	}
	return ""
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"message": message}})
}
