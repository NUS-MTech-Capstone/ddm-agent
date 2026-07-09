package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/NUS-MTech-Capstone/ddm-agent/internal/model"
)

type fakeValidator struct {
	sessionErr       error
	automationErr    error
	sessionCalled    bool
	automationCalled bool
	session          string
	token            string
	automation       model.AutomationClaims
}

func (f *fakeValidator) ValidateSessionToken(_ context.Context, req model.SessionTokenValidationRequest) (model.SessionClaims, error) {
	f.sessionCalled = true
	f.session = req.SessionID
	if req.UserID != "user-1" {
		return model.SessionClaims{}, errors.New("unexpected user")
	}
	if req.Mode != "owner" || req.ConnectionType != "ssh" {
		return model.SessionClaims{}, errors.New("unexpected expected claims")
	}
	f.token = req.Token
	return model.SessionClaims{SessionID: req.SessionID, UserID: req.UserID, HostID: req.HostID, CRID: req.CRID, ConnectionType: req.ConnectionType, Mode: req.Mode}, f.sessionErr
}

func (f *fakeValidator) ValidateAutomationToken(_ context.Context, runID, token string) (model.AutomationClaims, error) {
	f.automationCalled = true
	f.token = token
	if f.automationErr != nil {
		return model.AutomationClaims{}, f.automationErr
	}
	claims := f.automation
	if claims.RunID == "" {
		claims.RunID = runID
	}
	if claims.HostID == "" {
		claims.HostID = "host-1"
	}
	return claims, nil
}

type fakeRunner struct {
	called  bool
	command string
	result  model.ExecuteResult
	err     error
}

func (f *fakeRunner) Run(_ context.Context, command string) (model.ExecuteResult, error) {
	f.called = true
	f.command = command
	return f.result, f.err
}

func TestExecuteValidatesTokenBeforeRunningCommand(t *testing.T) {
	validator := &fakeValidator{}
	runner := &fakeRunner{result: model.ExecuteResult{Stdout: "agent\n"}}
	svc := newTestService(validator, runner, "host-1")

	result, err := svc.Execute(t.Context(), validRequest())
	if err != nil {
		t.Fatal(err)
	}
	if !validator.sessionCalled || validator.session != "sess-1" || validator.token != "token-1" {
		t.Fatalf("validator call = session %q token %q", validator.session, validator.token)
	}
	if !runner.called || runner.command != "whoami" {
		t.Fatalf("runner called = %v command = %q", runner.called, runner.command)
	}
	if result.Stdout != "agent\n" {
		t.Fatalf("stdout = %q", result.Stdout)
	}
}

func TestExecuteDoesNotRunWhenSTSRejectsToken(t *testing.T) {
	validator := &fakeValidator{sessionErr: errors.New("token expired")}
	runner := &fakeRunner{}
	svc := newTestService(validator, runner, "host-1")

	_, err := svc.Execute(t.Context(), validRequest())
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("Execute() error = %v, want unauthorized", err)
	}
	if runner.called {
		t.Fatal("runner was called after STS rejected the token")
	}
}

func TestExecuteRejectsWrongAgentHost(t *testing.T) {
	validator := &fakeValidator{}
	runner := &fakeRunner{}
	svc := newTestService(validator, runner, "other-host")

	_, err := svc.Execute(t.Context(), validRequest())
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("Execute() error = %v, want unauthorized", err)
	}
	if validator.sessionCalled || runner.called {
		t.Fatal("host mismatch should be rejected before STS or command execution")
	}
}

func TestExecuteRunsAllowedAutomationOperation(t *testing.T) {
	validator := &fakeValidator{automation: model.AutomationClaims{
		RunID: "run-1", IncidentID: "inc-1", HostID: "host-1", AllowedOperations: []string{"health_check"},
	}}
	runner := &fakeRunner{result: model.ExecuteResult{Stdout: "ok\n"}}
	svc := newTestService(validator, runner, "host-1")

	req := validRequest()
	req.SessionID = ""
	req.UserID = ""
	req.CRID = ""
	req.RunID = "run-1"
	req.IncidentID = "inc-1"
	req.Operation = "health_check"
	result, err := svc.Execute(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !validator.automationCalled || !runner.called {
		t.Fatal("automation validation and command execution were not called")
	}
	if result.Stdout != "ok\n" {
		t.Fatalf("stdout = %q", result.Stdout)
	}
}

func TestExecuteRejectsAutomationOperationNotAllowedByToken(t *testing.T) {
	validator := &fakeValidator{automation: model.AutomationClaims{
		RunID: "run-1", HostID: "host-1", AllowedOperations: []string{"health_check"},
	}}
	runner := &fakeRunner{}
	svc := newTestService(validator, runner, "host-1")

	req := validRequest()
	req.SessionID = ""
	req.UserID = ""
	req.CRID = ""
	req.RunID = "run-1"
	req.Operation = "restart_service"
	_, err := svc.Execute(t.Context(), req)
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("Execute() error = %v, want unauthorized", err)
	}
	if runner.called {
		t.Fatal("runner was called for a disallowed automation operation")
	}
}

func TestExecuteRejectsInvalidRequestsBeforeAuthorization(t *testing.T) {
	tests := []struct {
		name string
		req  model.ExecuteRequest
	}{
		{name: "missing host", req: model.ExecuteRequest{Token: "token", Command: "whoami"}},
		{name: "missing token", req: model.ExecuteRequest{HostID: "host-1", Command: "whoami"}},
		{name: "empty command", req: model.ExecuteRequest{HostID: "host-1", Token: "token", Command: "  "}},
		{name: "oversized command", req: model.ExecuteRequest{HostID: "host-1", Token: "token", Command: "too-long"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := &fakeValidator{}
			runner := &fakeRunner{}
			svc := New(validator, runner, func(ctx context.Context) (context.Context, context.CancelFunc) {
				return context.WithTimeout(ctx, time.Second)
			}, len("short"), "")

			_, err := svc.Execute(t.Context(), tt.req)
			if err == nil {
				t.Fatal("Execute() error = nil")
			}
			if validator.sessionCalled || validator.automationCalled || runner.called {
				t.Fatalf("validator=%v/%v runner=%v", validator.sessionCalled, validator.automationCalled, runner.called)
			}
		})
	}
}

func TestExecuteWrapsRunnerFailure(t *testing.T) {
	validator := &fakeValidator{}
	runnerErr := errors.New("boom")
	runner := &fakeRunner{err: runnerErr}
	svc := newTestService(validator, runner, "host-1")

	_, err := svc.Execute(t.Context(), validRequest())
	if !errors.Is(err, ErrExecution) || !errors.Is(err, runnerErr) {
		t.Fatalf("Execute() error = %v, want ErrExecution wrapping runner error", err)
	}
}

func TestAutomationAuthorizationRejectsHostIncidentAndMissingOperation(t *testing.T) {
	tests := []struct {
		name   string
		claims model.AutomationClaims
		req    model.ExecuteRequest
		want   error
	}{
		{
			name:   "host mismatch",
			claims: model.AutomationClaims{RunID: "run-1", HostID: "other-host", AllowedOperations: []string{"health_check"}},
			req:    automationRequest("health_check", "inc-1"),
			want:   ErrUnauthorized,
		},
		{
			name:   "incident mismatch",
			claims: model.AutomationClaims{RunID: "run-1", HostID: "host-1", IncidentID: "other-inc", AllowedOperations: []string{"health_check"}},
			req:    automationRequest("health_check", "inc-1"),
			want:   ErrUnauthorized,
		},
		{
			name:   "missing operation",
			claims: model.AutomationClaims{RunID: "run-1", HostID: "host-1", IncidentID: "inc-1", AllowedOperations: []string{"health_check"}},
			req:    automationRequest(" ", "inc-1"),
			want:   ErrInvalidRequest,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := &fakeValidator{automation: tt.claims}
			runner := &fakeRunner{}
			svc := newTestService(validator, runner, "")
			_, err := svc.Execute(t.Context(), tt.req)
			if !errors.Is(err, tt.want) {
				t.Fatalf("Execute() error = %v, want %v", err, tt.want)
			}
			if runner.called {
				t.Fatal("runner called for rejected automation request")
			}
		})
	}
}

func TestSelectAuthorizationStrategyUsesAutomationWhenRunIDPresent(t *testing.T) {
	strategy := selectAuthorizationStrategy(model.ExecuteRequest{RunID: "run-1"})
	if _, ok := strategy.(AutomationAuthorizationStrategy); !ok {
		t.Fatalf("strategy = %T, want AutomationAuthorizationStrategy", strategy)
	}
}

func automationRequest(operation, incidentID string) model.ExecuteRequest {
	req := validRequest()
	req.SessionID = ""
	req.UserID = ""
	req.CRID = ""
	req.RunID = "run-1"
	req.IncidentID = incidentID
	req.Operation = operation
	return req
}

func TestSelectAuthorizationStrategyDefaultsToSession(t *testing.T) {
	strategy := selectAuthorizationStrategy(model.ExecuteRequest{SessionID: "sess-1"})
	if _, ok := strategy.(SessionAuthorizationStrategy); !ok {
		t.Fatalf("strategy = %T, want SessionAuthorizationStrategy", strategy)
	}
}

func newTestService(validator TokenValidator, runner CommandRunner, hostID string) *Service {
	return New(
		validator,
		runner,
		func(ctx context.Context) (context.Context, context.CancelFunc) {
			return context.WithTimeout(ctx, time.Second)
		},
		1024,
		hostID,
	)
}

func validRequest() model.ExecuteRequest {
	return model.ExecuteRequest{
		SessionID: "sess-1",
		UserID:    "user-1",
		HostID:    "host-1",
		CRID:      "cr-1",
		Token:     "token-1",
		Command:   "whoami",
	}
}
