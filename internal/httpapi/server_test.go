package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/NUS-MTech-Capstone/ddm-agent/internal/model"
	"github.com/NUS-MTech-Capstone/ddm-agent/internal/service"
)

type validatorFunc func(context.Context, model.SessionTokenValidationRequest) (model.SessionClaims, error)

func (f validatorFunc) ValidateSessionToken(ctx context.Context, req model.SessionTokenValidationRequest) (model.SessionClaims, error) {
	return f(ctx, req)
}

func (f validatorFunc) ValidateAutomationToken(context.Context, string, string) (model.AutomationClaims, error) {
	return model.AutomationClaims{}, errors.New("unexpected automation validation")
}

type runnerFunc func(context.Context, string) (model.ExecuteResult, error)

func (f runnerFunc) Run(ctx context.Context, command string) (model.ExecuteResult, error) {
	return f(ctx, command)
}

func TestExecuteRequiresSTSAuthorization(t *testing.T) {
	runnerCalled := false
	svc := testService(
		validatorFunc(func(_ context.Context, _ model.SessionTokenValidationRequest) (model.SessionClaims, error) {
			return model.SessionClaims{}, errors.New("token expired")
		}),
		runnerFunc(func(_ context.Context, _ string) (model.ExecuteResult, error) {
			runnerCalled = true
			return model.ExecuteResult{}, nil
		}),
	)
	server := New(svc, 1024)

	response := httptest.NewRecorder()
	server.ServeHTTP(response, newExecuteHTTPRequest(`{"command":"whoami"}`, "token-1"))

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
	if runnerCalled {
		t.Fatal("runner was called for an unauthorized request")
	}
}

func TestExecuteReturnsStructuredCommandResult(t *testing.T) {
	svc := testService(
		validatorFunc(func(_ context.Context, req model.SessionTokenValidationRequest) (model.SessionClaims, error) {
			if req.SessionID != "sess-1" || req.UserID != "user-1" || req.Token != "token-1" ||
				req.HostID != "host-1" || req.CRID != "cr-1" || req.Mode != "owner" || req.ConnectionType != "ssh" {
				t.Fatalf("STS validation got %#v", req)
			}
			return model.SessionClaims{SessionID: req.SessionID, UserID: req.UserID, HostID: req.HostID}, nil
		}),
		runnerFunc(func(_ context.Context, command string) (model.ExecuteResult, error) {
			if command != "whoami" {
				t.Fatalf("command = %q", command)
			}
			return model.ExecuteResult{Stdout: "ddm\n", ExitCode: 0}, nil
		}),
	)
	server := New(svc, 1024)

	response := httptest.NewRecorder()
	server.ServeHTTP(response, newExecuteHTTPRequest(`{"command":"whoami"}`, "token-1"))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	var result model.ExecuteResult
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Stdout != "ddm\n" || result.ExitCode != 0 {
		t.Fatalf("result = %#v", result)
	}
}

func testService(validator service.TokenValidator, runner service.CommandRunner) *service.Service {
	return service.New(
		validator,
		runner,
		func(ctx context.Context) (context.Context, context.CancelFunc) {
			return context.WithTimeout(ctx, time.Second)
		},
		1024,
		"host-1",
	)
}

func newExecuteHTTPRequest(body, token string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/execute", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("X-DDM-Session-ID", "sess-1")
	request.Header.Set("X-DDM-User-ID", "user-1")
	request.Header.Set("X-DDM-Host-ID", "host-1")
	request.Header.Set("X-DDM-CR-ID", "cr-1")
	return request
}
