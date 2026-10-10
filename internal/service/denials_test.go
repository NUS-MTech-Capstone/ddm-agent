package service

import (
	"errors"
	"github.com/NUS-MTech-Capstone/ddm-agent/internal/model"
	"strings"
	"testing"
)

func TestInvalidRequestsNeverExecute(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*model.ExecuteRequest)
		want   error
	}{
		{"missing host", func(r *model.ExecuteRequest) { r.HostID = "" }, ErrInvalidRequest},
		{"missing token", func(r *model.ExecuteRequest) { r.Token = "" }, ErrUnauthorized},
		{"empty command", func(r *model.ExecuteRequest) { r.Command = "  " }, ErrInvalidRequest},
		{"oversized command", func(r *model.ExecuteRequest) { r.Command = strings.Repeat("x", 20000) }, ErrInvalidRequest},
		{"missing session", func(r *model.ExecuteRequest) { r.SessionID = "" }, ErrInvalidRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := &fakeValidator{}
			runner := &fakeRunner{}
			req := validRequest()
			tc.mutate(&req)
			_, err := newTestService(v, runner, "host-1").Execute(t.Context(), req)
			if !errors.Is(err, tc.want) || runner.called || v.sessionCalled {
				t.Fatalf("error=%v executed=%v validated=%v", err, runner.called, v.sessionCalled)
			}
		})
	}
}
func TestAutomationDenialsNeverExecute(t *testing.T) {
	for _, tc := range []struct {
		name         string
		claims       model.AutomationClaims
		operation    string
		validatorErr error
		want         error
	}{
		{"STS failure", model.AutomationClaims{}, "health_check", errors.New("STS unavailable"), ErrUnauthorized},
		{"wrong host", model.AutomationClaims{HostID: "other"}, "health_check", nil, ErrUnauthorized},
		{"wrong incident", model.AutomationClaims{IncidentID: "other"}, "health_check", nil, ErrUnauthorized},
		{"missing operation", model.AutomationClaims{IncidentID: "inc-1"}, " ", nil, ErrInvalidRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := &fakeValidator{automation: tc.claims, automationErr: tc.validatorErr}
			r := &fakeRunner{}
			req := validRequest()
			req.RunID = "run-1"
			req.IncidentID = "inc-1"
			req.Operation = tc.operation
			_, err := newTestService(v, r, "host-1").Execute(t.Context(), req)
			if !errors.Is(err, tc.want) || r.called {
				t.Fatalf("error=%v executed=%v", err, r.called)
			}
		})
	}
}
func TestExecutionFailurePreservesCause(t *testing.T) {
	cause := errors.New("runner failed")
	r := &fakeRunner{err: cause}
	_, err := newTestService(&fakeValidator{}, r, "host-1").Execute(t.Context(), validRequest())
	if !errors.Is(err, ErrExecution) || !errors.Is(err, cause) || !r.called {
		t.Fatalf("error=%v called=%v", err, r.called)
	}
}
