package reaper

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// recordedExec captures the command/args and returns a scripted outcome, so the
// docker reapers can be tested without a real Docker Engine.
type recordedExec struct {
	command  string
	args     []string
	result   ExecResult
	err      error
	called   bool
}

func (r *recordedExec) Run(ctx context.Context, command string, args []string, timeout time.Duration) (ExecResult, error) {
	r.called = true
	r.command = command
	r.args = append([]string(nil), args...)
	return r.result, r.err
}

func TestDockerReaperScenarios(t *testing.T) {
	base := Entry{Kind: KindDocker, ID: "srv-1"}
	cases := []struct {
		name       string
		result     ExecResult
		execErr    error
		wantStatus Status
		wantArg    []string
		wantPrefix string
	}{
		{"reaped", ExecResult{Code: 0}, nil, StatusReaped, []string{"rm", "--force", "--volumes", "srv-1"}, ""},
		{"missing-container", ExecResult{Code: 1, Stderr: "Error: No such container: srv-1"}, nil, StatusMissing, nil, ""},
		{"generic-failure", ExecResult{Code: 1, Stderr: "connect: permission denied"}, nil, StatusSkipped, nil, "docker rm failed"},
		{"docker-unavailable", ExecResult{Code: 127, Stderr: ""}, errors.New("docker CLI not found"), StatusSkipped, nil, "docker unavailable"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			re := &recordedExec{result: c.result, err: c.execErr}
			out, err := DockerReaper().Reap(context.Background(), base, ReapContext{Exec: re.Run})
			if err != nil {
				t.Fatalf("Reap error: %v", err)
			}
			if out.Status != c.wantStatus {
				t.Fatalf("status = %s, want %s (%s)", out.Status, c.wantStatus, out.Reason)
			}
			if !re.called {
				t.Fatalf("exec was not called")
			}
			if re.command != "docker" {
				t.Fatalf("command = %q, want docker", re.command)
			}
			if c.wantArg != nil {
				if strings.Join(re.args, " ") != strings.Join(c.wantArg, " ") {
					t.Fatalf("args = %v, want %v", re.args, c.wantArg)
				}
			}
			if c.wantPrefix != "" && !strings.HasPrefix(out.Reason, c.wantPrefix) {
				t.Fatalf("reason = %q, want prefix %q", out.Reason, c.wantPrefix)
			}
		})
	}
}

func TestDockerVolumeReaperScenarios(t *testing.T) {
	base := Entry{Kind: KindDockerVolume, ID: "vol-1"}
	cases := []struct {
		name       string
		result     ExecResult
		execErr    error
		wantStatus Status
		wantArg    []string
		wantPrefix string
	}{
		{"reaped", ExecResult{Code: 0}, nil, StatusReaped, []string{"volume", "rm", "--force", "vol-1"}, ""},
		{"missing-volume", ExecResult{Code: 1, Stderr: "Error: No such volume: vol-1"}, nil, StatusMissing, nil, ""},
		{"generic-failure", ExecResult{Code: 1, Stderr: "network error"}, nil, StatusSkipped, nil, "docker volume rm failed"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			re := &recordedExec{result: c.result, err: c.execErr}
			out, err := DockerVolumeReaper().Reap(context.Background(), base, ReapContext{Exec: re.Run})
			if err != nil {
				t.Fatalf("Reap error: %v", err)
			}
			if out.Status != c.wantStatus {
				t.Fatalf("status = %s, want %s (%s)", out.Status, c.wantStatus, out.Reason)
			}
			if re.command != "docker" {
				t.Fatalf("command = %q, want docker", re.command)
			}
			if c.wantArg != nil {
				if strings.Join(re.args, " ") != strings.Join(c.wantArg, " ") {
					t.Fatalf("args = %v, want %v", re.args, c.wantArg)
				}
			}
			if c.wantPrefix != "" && !strings.HasPrefix(out.Reason, c.wantPrefix) {
				t.Fatalf("reason = %q, want prefix %q", out.Reason, c.wantPrefix)
			}
		})
	}
}

func TestDockerReaperNoExec(t *testing.T) {
	out, err := DockerReaper().Reap(context.Background(), Entry{Kind: KindDocker, ID: "x"}, ReapContext{})
	if err != nil {
		t.Fatalf("Reap error: %v", err)
	}
	if out.Status != StatusSkipped {
		t.Fatalf("status = %s, want skipped (no exec)", out.Status)
	}
}
