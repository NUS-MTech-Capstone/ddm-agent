package sts

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/NUS-MTech-Capstone/ddm-agent/internal/model"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestValidateSessionToken(t *testing.T) {
	client := New("http://sts.example", time.Second)
	client.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost || r.URL.Path != "/tokens/validate" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("x-api-version") != "1" {
			t.Fatalf("api version = %q", r.Header.Get("x-api-version"))
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["sessionId"] != "sess-1" || body["userId"] != "user-1" ||
			body["hostId"] != "host-1" || body["crId"] != "cr-1" ||
			body["connectionType"] != "ssh" || body["mode"] != "owner" || body["token"] != "token-1" {
			t.Fatalf("body = %#v", body)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"allowed":true,"claims":{"sessionId":"sess-1","userId":"user-1","hostId":"host-1","crId":"cr-1","connectionType":"ssh","mode":"owner"}}`)),
		}, nil
	})

	_, err := client.ValidateSessionToken(t.Context(), model.SessionTokenValidationRequest{
		SessionID: "sess-1", UserID: "user-1", HostID: "host-1", CRID: "cr-1",
		ConnectionType: "ssh", Mode: "owner", Token: "token-1",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidateSessionTokenRejectsDeniedResponse(t *testing.T) {
	client := New("http://sts.example", time.Second)
	client.client.Transport = roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusForbidden,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"allowed":false,"reason":"token expired"}`)),
		}, nil
	})
	if _, err := client.ValidateSessionToken(t.Context(), model.SessionTokenValidationRequest{
		SessionID: "sess-1", UserID: "user-1", Token: "token-1",
	}); err == nil {
		t.Fatal("ValidateSessionToken() error = nil, want rejection")
	}
}

func TestValidateAutomationToken(t *testing.T) {
	client := New("http://sts.example", time.Second)
	client.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost || r.URL.Path != "/tokens/automation/validate" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("x-api-version") != "1" {
			t.Fatalf("api version = %q", r.Header.Get("x-api-version"))
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["runId"] != "run-1" || body["token"] != "token-1" {
			t.Fatalf("body = %#v", body)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"allowed":true,"claims":{"runId":"run-1","incidentId":"inc-1","hostId":"host-1","phase":"diagnose","allowedOperations":["health_check"]}}`)),
		}, nil
	})

	claims, err := client.ValidateAutomationToken(t.Context(), "run-1", "token-1")
	if err != nil {
		t.Fatal(err)
	}
	if claims.HostID != "host-1" || len(claims.AllowedOperations) != 1 || claims.AllowedOperations[0] != "health_check" {
		t.Fatalf("claims = %#v", claims)
	}
}
