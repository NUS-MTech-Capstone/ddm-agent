package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/NUS-MTech-Capstone/ddm-agent/internal/model"
)

type fakeValidator struct {
	err     error
	called  bool
	session string
	token   string
}

func (f *fakeValidator) ValidateToken(_ context.Context, sessionID, userID, token string) error {
	f.called = true
	f.session = sessionID
	if userID != "user-1" {
		return errors.New("unexpected user")
	}
	f.token = token
	return f.err
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
	if !validator.called || validator.session != "sess-1" || validator.token != "token-1" {
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
	validator := &fakeValidator{err: errors.New("token expired")}
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
	if validator.called || runner.called {
		t.Fatal("host mismatch should be rejected before STS or command execution")
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
