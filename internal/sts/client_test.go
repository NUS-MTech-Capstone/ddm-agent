package sts

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestValidateToken(t *testing.T) {
	client := New("http://sts.example", time.Second)
	client.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost || r.URL.Path != "/tokens/validate" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["sessionId"] != "sess-1" || body["userId"] != "user-1" || body["mode"] != "owner" || body["token"] != "token-1" {
			t.Fatalf("body = %#v", body)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"allowed":true}`)),
		}, nil
	})

	if err := client.ValidateToken(t.Context(), "sess-1", "user-1", "token-1"); err != nil {
		t.Fatal(err)
	}
}

func TestValidateTokenRejectsDeniedResponse(t *testing.T) {
	client := New("http://sts.example", time.Second)
	client.client.Transport = roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusForbidden,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"allowed":false,"reason":"token expired"}`)),
		}, nil
	})
	if err := client.ValidateToken(t.Context(), "sess-1", "user-1", "token-1"); err == nil {
		t.Fatal("ValidateToken() error = nil, want rejection")
	}
}
