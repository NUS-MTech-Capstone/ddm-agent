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
)

const maxResponseBytes = 64 * 1024

type Client struct {
	baseURL string
	client  *http.Client
}

type validateResponse struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason"`
}

func New(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: timeout},
	}
}

func (c *Client) ValidateToken(ctx context.Context, sessionID, userID, token string) error {
	body, err := json.Marshal(map[string]string{
		"sessionId": sessionID,
		"userId":    userID,
		"mode":      "owner",
		"token":     token,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/tokens/validate", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("call STS: %w", err)
	}
	defer resp.Body.Close()

	var result validateResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&result); err != nil {
		return fmt.Errorf("decode STS response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || !result.Allowed {
		if result.Reason == "" {
			result.Reason = "token rejected"
		}
		return errors.New(result.Reason)
	}
	return nil
}
