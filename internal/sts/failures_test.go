package sts

import (
	"context"
	"errors"
	"github.com/NUS-MTech-Capstone/ddm-agent/internal/model"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestTokenValidationFailsClosed(t *testing.T) {
	for _, kind := range []string{"session", "automation"} {
		for _, tc := range []struct {
			name, body, want string
			status           int
			transportErr     error
		}{
			{name: "missing claims", body: `{"allowed":true}`, want: "missing claims", status: 200},
			{name: "denied without reason", body: `{"allowed":false}`, want: "token rejected", status: 200},
			{name: "upstream error despite allowed", body: `{"allowed":true,"claims":{}}`, want: "token rejected", status: 503},
			{name: "invalid json", body: `invalid`, want: "decode STS response", status: 200},
			{name: "oversized body", body: strings.Repeat(" ", maxResponseBytes) + `{"allowed":true,"claims":{}}`, want: "decode STS response", status: 200},
			{name: "cancelled", transportErr: context.Canceled, want: "call STS"},
		} {
			t.Run(kind+"/"+tc.name, func(t *testing.T) {
				c := New("http://sts.example", time.Second)
				c.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
					if tc.transportErr != nil {
						return nil, tc.transportErr
					}
					return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body)), Header: make(http.Header)}, nil
				})
				var err error
				if kind == "session" {
					_, err = c.ValidateSessionToken(t.Context(), model.SessionTokenValidationRequest{})
				} else {
					_, err = c.ValidateAutomationToken(t.Context(), "run", "token")
				}
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("error=%v", err)
				}
				if tc.transportErr != nil && !errors.Is(err, tc.transportErr) {
					t.Fatalf("lost cancellation: %v", err)
				}
			})
		}
	}
}
