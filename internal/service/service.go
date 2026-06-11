package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/NUS-MTech-Capstone/ddm-agent/internal/model"
)

var ErrUnauthorized = errors.New("session is not authorized")
var ErrInvalidRequest = errors.New("invalid execute request")
var ErrExecution = errors.New("execute command")

type TokenValidator interface {
	ValidateToken(context.Context, string, string, string) error
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
	if req.SessionID == "" || req.UserID == "" || req.HostID == "" || req.CRID == "" {
		return model.ExecuteResult{}, fmt.Errorf("%w: session, user, host, and change request metadata are required", ErrInvalidRequest)
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
	if err := s.sts.ValidateToken(ctx, req.SessionID, req.UserID, req.Token); err != nil {
		return model.ExecuteResult{}, fmt.Errorf("%w: %v", ErrUnauthorized, err)
	}

	commandCtx, cancel := s.commandTimeout(ctx)
	defer cancel()
	result, err := s.runner.Run(commandCtx, req.Command)
	if err != nil {
		return model.ExecuteResult{}, fmt.Errorf("%w: %w", ErrExecution, err)
	}
	return result, nil
}
