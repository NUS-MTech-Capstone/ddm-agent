package execution

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRunnerReturnsOutputAndExitCode(t *testing.T) {
	runner := NewRunner("/bin/sh", 1024)
	result, err := runner.Run(t.Context(), "printf stdout; printf stderr >&2; exit 7")
	if err != nil {
		t.Fatal(err)
	}
	if result.Stdout != "stdout" || result.Stderr != "stderr" || result.ExitCode != 7 {
		t.Fatalf("result = %#v", result)
	}
}

func TestRunnerLimitsOutput(t *testing.T) {
	runner := NewRunner("/bin/sh", 4)
	result, err := runner.Run(t.Context(), "printf 123456")
	if err != nil {
		t.Fatal(err)
	}
	if result.Stdout != "1234\n[output truncated]\n" {
		t.Fatalf("stdout = %q", result.Stdout)
	}
}

func TestRunnerTimesOut(t *testing.T) {
	runner := NewRunner("/bin/sh", 1024)
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()

	_, err := runner.Run(ctx, "sleep 1")
	if !errors.Is(err, ErrTimedOut) {
		t.Fatalf("Run() error = %v, want timeout", err)
	}
	if strings.Contains(err.Error(), "sleep") {
		t.Fatalf("timeout error leaked command: %v", err)
	}
}
