package execution

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"syscall"
	"time"

	"github.com/NUS-MTech-Capstone/ddm-agent/internal/model"
)

var ErrTimedOut = errors.New("command timed out")

type Runner struct {
	shell          string
	maxOutputBytes int
}

func NewRunner(shell string, maxOutputBytes int) *Runner {
	return &Runner{shell: shell, maxOutputBytes: maxOutputBytes}
}

func (r *Runner) Run(ctx context.Context, command string) (model.ExecuteResult, error) {
	cmd := exec.CommandContext(ctx, r.shell, "-c", command) // #nosec G204 -- command execution is the agent feature and is guarded by STS auth, size limits, timeout, and bounded output.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = 2 * time.Second
	stdout := newLimitedBuffer(r.maxOutputBytes)
	stderr := newLimitedBuffer(r.maxOutputBytes)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err := cmd.Run()
	result := model.ExecuteResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}
	if ctx.Err() != nil {
		return model.ExecuteResult{}, ErrTimedOut
	}
	if err == nil {
		return result, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	}
	return model.ExecuteResult{}, err
}

type limitedBuffer struct {
	buffer    bytes.Buffer
	remaining int
	truncated bool
}

func newLimitedBuffer(limit int) *limitedBuffer {
	return &limitedBuffer{remaining: limit}
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	originalLength := len(p)
	if len(p) > b.remaining {
		p = p[:b.remaining]
		b.truncated = true
	}
	if len(p) > 0 {
		_, _ = b.buffer.Write(p)
		b.remaining -= len(p)
	}
	return originalLength, nil
}

func (b *limitedBuffer) String() string {
	if !b.truncated {
		return b.buffer.String()
	}
	return b.buffer.String() + "\n[output truncated]\n"
}
