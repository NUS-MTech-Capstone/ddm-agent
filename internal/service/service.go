package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/NUS-MTech-Capstone/ddm-agent/internal/model"
)

var ErrUnauthorized = errors.New("session is not authorized")
var ErrInvalidRequest = errors.New("invalid execute request")
var ErrExecution = errors.New("execute command")

type TokenValidator interface {
	ValidateSessionToken(context.Context, model.SessionTokenValidationRequest) (model.SessionClaims, error)
	ValidateAutomationToken(context.Context, string, string) (model.AutomationClaims, error)
}

type CommandRunner interface {
	Run(context.Context, string) (model.ExecuteResult, error)
}

type Service struct {
	sts             TokenValidator
	runner          CommandRunner
	commandTimeout  func(context.Context) (context.Context, context.CancelFunc)
	maxCommandBytes int
	hostID          string
}

func New(sts TokenValidator, runner CommandRunner, timeoutContext func(context.Context) (context.Context, context.CancelFunc), maxCommandBytes int, hostID string) *Service {
	return &Service{
		sts:             sts,
		runner:          runner,
		commandTimeout:  timeoutContext,
		maxCommandBytes: maxCommandBytes,
		hostID:          hostID,
	}
}

func (s *Service) Execute(ctx context.Context, req model.ExecuteRequest) (model.ExecuteResult, error) {
	if req.HostID == "" {
		return model.ExecuteResult{}, fmt.Errorf("%w: host metadata is required", ErrInvalidRequest)
	}
	if req.Token == "" {
		return model.ExecuteResult{}, ErrUnauthorized
	}
	if s.hostID != "" && req.HostID != s.hostID {
		return model.ExecuteResult{}, fmt.Errorf("%w: target host mismatch", ErrUnauthorized)
	}
	if strings.TrimSpace(req.Command) == "" {
		return model.ExecuteResult{}, fmt.Errorf("%w: command is required", ErrInvalidRequest)
	}
	if len(req.Command) > s.maxCommandBytes {
		return model.ExecuteResult{}, fmt.Errorf("%w: command exceeds %d bytes", ErrInvalidRequest, s.maxCommandBytes)
	}

	if req.RunID != "" {
		claims, err := s.sts.ValidateAutomationToken(ctx, req.RunID, req.Token)
		if err != nil {
			return model.ExecuteResult{}, fmt.Errorf("%w: %v", ErrUnauthorized, err)
		}
		if claims.HostID != req.HostID {
			return model.ExecuteResult{}, fmt.Errorf("%w: token host mismatch", ErrUnauthorized)
		}
		if req.IncidentID != "" && claims.IncidentID != req.IncidentID {
			return model.ExecuteResult{}, fmt.Errorf("%w: token incident mismatch", ErrUnauthorized)
		}
		if strings.TrimSpace(req.Operation) == "" {
			return model.ExecuteResult{}, fmt.Errorf("%w: operation is required for automation execution", ErrInvalidRequest)
		}
		if !slices.Contains(claims.AllowedOperations, req.Operation) {
			return model.ExecuteResult{}, fmt.Errorf("%w: operation is not allowed", ErrUnauthorized)
		}
	} else {
		if req.SessionID == "" || req.UserID == "" || req.CRID == "" {
			return model.ExecuteResult{}, fmt.Errorf("%w: session, user, host, and change request metadata are required", ErrInvalidRequest)
		}
		if _, err := s.sts.ValidateSessionToken(ctx, model.SessionTokenValidationRequest{
			SessionID:      req.SessionID,
			UserID:         req.UserID,
			HostID:         req.HostID,
			CRID:           req.CRID,
			ConnectionType: "ssh",
			Mode:           "owner",
			Token:          req.Token,
		}); err != nil {
			return model.ExecuteResult{}, fmt.Errorf("%w: %v", ErrUnauthorized, err)
		}
	}

	commandCtx, cancel := s.commandTimeout(ctx)
	defer cancel()
	result, err := s.runner.Run(commandCtx, req.Command)
	if err != nil {
		return model.ExecuteResult{}, fmt.Errorf("%w: %w", ErrExecution, err)
	}
	return result, nil
}
