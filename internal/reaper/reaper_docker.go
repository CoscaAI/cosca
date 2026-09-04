package reaper

import (
	"context"
	"strings"
	"time"
)

// Timeouts for tool-based reapers (docker), matching the openwork reaper.
const (
	dockerTimeout = 20 * time.Second
)

// dockerReaper disposes a Docker container only after confirming it exists. It
// never touches a container whose name was reused — a not-found is reported as
// "missing" instead of a blind no-op.
type dockerReaper struct{}

// Kind returns KindDocker.
func (dockerReaper) Kind() string { return KindDocker }

// DockerReaper returns the built-in container reaper.
func DockerReaper() Reaper { return dockerReaper{} }

// Reap runs `docker rm --force --volumes <id>` and classifies the outcome.
// It is cross-platform in that it shells out to the `docker` CLI; if docker is
// not present (e.g. the host has no Docker Engine or Desktop) it reports
// "docker unavailable" and is skipped — it never fabricates a reaped result.
// Note: on Windows this requires Docker Desktop; otherwise it is gated off by
// the CLI being absent.
func (dockerReaper) Reap(ctx context.Context, e Entry, rc ReapContext) (Outcome, error) {
	if rc.Exec == nil {
		return Outcome{Status: StatusSkipped, Reason: "no exec"}, nil
	}
	res, err := rc.Exec(ctx, "docker", []string{"rm", "--force", "--volumes", e.ID}, dockerTimeout)
	if err != nil {
		return Outcome{Status: StatusSkipped, Reason: "docker unavailable: " + err.Error()}, nil
	}
	if res.Code == 0 {
		return Outcome{Status: StatusReaped}, nil
	}
	if strings.Contains(strings.ToLower(res.Stderr), "no such container") {
		return Outcome{Status: StatusMissing}, nil
	}
	return Outcome{Status: StatusSkipped, Reason: "docker rm failed: " + firstLine(res.Stderr)}, nil
}

// dockerVolumeReaper disposes a Docker volume with the same existence-first
// discipline: it only reports reaped for a real removal and "missing" for a
// volume that no longer exists.
type dockerVolumeReaper struct{}

// Kind returns KindDockerVolume.
func (dockerVolumeReaper) Kind() string { return KindDockerVolume }

// DockerVolumeReaper returns the built-in volume reaper.
func DockerVolumeReaper() Reaper { return dockerVolumeReaper{} }

// Reap runs `docker volume rm --force <id>` and classifies the outcome.
func (dockerVolumeReaper) Reap(ctx context.Context, e Entry, rc ReapContext) (Outcome, error) {
	if rc.Exec == nil {
		return Outcome{Status: StatusSkipped, Reason: "no exec"}, nil
	}
	res, err := rc.Exec(ctx, "docker", []string{"volume", "rm", "--force", e.ID}, dockerTimeout)
	if err != nil {
		return Outcome{Status: StatusSkipped, Reason: "docker unavailable: " + err.Error()}, nil
	}
	if res.Code == 0 {
		return Outcome{Status: StatusReaped}, nil
	}
	if strings.Contains(strings.ToLower(res.Stderr), "no such volume") {
		return Outcome{Status: StatusMissing}, nil
	}
	return Outcome{Status: StatusSkipped, Reason: "docker volume rm failed: " + firstLine(res.Stderr)}, nil
}

// firstLine returns the first line of a multi-line message, trimmed of CR.
func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return strings.TrimSpace(line)
}
