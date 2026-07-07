package sts

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/NUS-MTech-Capstone/ddm-agent/internal/model"
	"github.com/NUS-MTech-Capstone/ddm-agent/internal/observability"
)

const maxResponseBytes = 64 * 1024

type Client struct {
	baseURL string
	client  *http.Client
}

type validateResponse struct {
	Allowed bool                 `json:"allowed"`
	Reason  string               `json:"reason"`
	Claims  *model.SessionClaims `json:"claims,omitempty"`
}

type automationValidateResponse struct {
	Allowed bool                    `json:"allowed"`
	Reason  string                  `json:"reason"`
	Claims  *model.AutomationClaims `json:"claims,omitempty"`
}

func New(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: timeout},
	}
}

func (c *Client) ValidateSessionToken(ctx context.Context, validation model.SessionTokenValidationRequest) (model.SessionClaims, error) {
	body, err := json.Marshal(map[string]string{
		"sessionId":      validation.SessionID,
		"userId":         validation.UserID,
		"hostId":         validation.HostID,
		"crId":           validation.CRID,
		"connectionType": validation.ConnectionType,
		"action":         validation.Action,
		"mode":           validation.Mode,
		"token":          validation.Token,
	})
	if err != nil {
		return model.SessionClaims{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/tokens/validate", bytes.NewReader(body))
	if err != nil {
		return model.SessionClaims{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	observability.SetRequestHeaders(ctx, req)

	resp, err := c.client.Do(req)
	if err != nil {
		return model.SessionClaims{}, fmt.Errorf("call STS: %w", err)
	}
	defer resp.Body.Close()

	var result validateResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&result); err != nil {
		return model.SessionClaims{}, fmt.Errorf("decode STS response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || !result.Allowed {
		if result.Reason == "" {
			result.Reason = "token rejected"
		}
		return model.SessionClaims{}, errors.New(result.Reason)
	}
	if result.Claims == nil {
		return model.SessionClaims{}, errors.New("token response missing claims")
	}
	return *result.Claims, nil
}

func (c *Client) ValidateAutomationToken(ctx context.Context, runID, token string) (model.AutomationClaims, error) {
	body, err := json.Marshal(map[string]string{
		"runId": runID,
		"token": token,
	})
	if err != nil {
		return model.AutomationClaims{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/tokens/automation/validate", bytes.NewReader(body))
	if err != nil {
		return model.AutomationClaims{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	observability.SetRequestHeaders(ctx, req)

	resp, err := c.client.Do(req)
	if err != nil {
		return model.AutomationClaims{}, fmt.Errorf("call STS: %w", err)
	}
	defer resp.Body.Close()

	var result automationValidateResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&result); err != nil {
		return model.AutomationClaims{}, fmt.Errorf("decode STS response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || !result.Allowed {
		if result.Reason == "" {
			result.Reason = "token rejected"
		}
		return model.AutomationClaims{}, errors.New(result.Reason)
	}
	if result.Claims == nil {
		return model.AutomationClaims{}, errors.New("token response missing claims")
	}
	return *result.Claims, nil
}
