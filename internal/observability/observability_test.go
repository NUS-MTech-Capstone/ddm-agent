package observability

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestIDMiddlewareUsesIncomingRequestID(t *testing.T) {
	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id := RequestID(r.Context()); id != "req-1" {
			t.Fatalf("RequestID() = %q, want req-1", id)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Request-ID", "req-1")

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted || rec.Header().Get("X-Request-ID") != "req-1" {
		t.Fatalf("status/header = %d/%q", rec.Code, rec.Header().Get("X-Request-ID"))
	}
}

func TestRequestIDMiddlewareGeneratesRequestID(t *testing.T) {
	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if RequestID(r.Context()) == "" {
			t.Fatal("RequestID() = empty")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Header().Get("X-Request-ID") == "" {
		t.Fatal("X-Request-ID response header is empty")
	}
}

func TestShouldSkipRequestLogSuppressesOnlySuccessfulProbeRequests(t *testing.T) {
	cases := []struct {
		name   string
		method string
		path   string
		status int
		want   bool
	}{
		{name: "successful healthz", method: http.MethodGet, path: "/healthz", status: http.StatusOK, want: true},
		{name: "successful readyz", method: http.MethodGet, path: "/readyz", status: http.StatusNoContent, want: true},
		{name: "failed healthz", method: http.MethodGet, path: "/healthz", status: http.StatusServiceUnavailable, want: false},
		{name: "business route", method: http.MethodPost, path: "/execute", status: http.StatusOK, want: false},
		{name: "non-get probe", method: http.MethodPost, path: "/healthz", status: http.StatusOK, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			if got := shouldSkipRequestLog(req, tc.status); got != tc.want {
				t.Fatalf("shouldSkipRequestLog() = %v, want %v", got, tc.want)
			}
		})
	}
}
