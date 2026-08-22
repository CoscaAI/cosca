// Package bootstrap implements the idempotent, self-service bootstrap of the
// Ollama provider. It detects the OS, detects the ollama binary, detects the
// running daemon, optionally installs Ollama (winget/choco/scoop/direct
// download on Windows; the official install script on Linux), optionally
// starts the daemon, waits for readiness, verifies the configured model,
// optionally pulls it, and finally validates that the model actually responds.
//
// Layering: the provider manager (internal/providers) is a catalogue and
// diagnostic surface — it never installs anything. Bootstrapping is a separate
// concern and lives here so `cosca provider bootstrap` and the boot-time hook
// reuse a single implementation.
//
// Every step is idempotent: a state that is already the desired one is
// reported as "skipped" with its evidence ("already ...") instead of being
// redone. When DetectOnly is set the chain only observes and reports — it
// never installs, starts or pulls.
//
// REVERSIBILITY — what this package may install/launch and how to undo it:
//   - Windows installer (OllamaSetup.exe, silent /S): uninstall via
//     Windows Settings → Apps → Ollama, or `winget uninstall --id Ollama.Ollama`.
//   - winget install: `winget uninstall --id Ollama.Ollama`.
//   - chocolatey: `choco uninstall ollama -y`.
//   - scoop: `scoop uninstall ollama`.
//   - Linux official install script: removes the binary with
//     `sudo rm -f /usr/local/bin/ollama`; the script also installs the systemd
//     unit /usr/lib/systemd/system/ollama.service — disable with
//     `systemctl disable ollama` before removing it.
//   - `ollama pull <model>` only downloads model blobs into the Ollama data
//     directory (~/.ollama or %USERPROFILE%\.ollama); undo with
//     `ollama rm <model>`.
//   - `ollama serve` background process: kill the recorded PID.
//
// Security: commands are executed with exec.Command and separated arguments —
// never `sh -c` with a concatenated string. Secrets are never logged or
// embedded in the report.
package bootstrap

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// Defaults for the bootstrap chain.
const (
	// DefaultBaseURL is the default Ollama daemon endpoint.
	DefaultBaseURL = "http://localhost:11434"
	// DefaultModel is the fallback model when none is configured.
	DefaultModel = "qwen2.5-coder:14b-128k"
	// DefaultTimeout is the overall budget for the whole chain (model pulls
	// are slow).
	DefaultTimeout = 15 * time.Minute
	// DefaultDaemonTimeout bounds a single daemon health probe.
	DefaultDaemonTimeout = 2 * time.Second
	// DefaultReadinessTimeout bounds the readiness polling loop.
	DefaultReadinessTimeout = 60 * time.Second
	// DefaultValidateTimeout bounds the final /api/generate call.
	DefaultValidateTimeout = 30 * time.Second
	// DefaultPollInterval is the interval between readiness probes.
	DefaultPollInterval = 500 * time.Millisecond
	// DefaultInstallerURL is the official Windows installer.
	DefaultInstallerURL = "https://ollama.com/download/OllamaSetup.exe"
	// DefaultInstallScriptURL is the official Linux install script.
	DefaultInstallScriptURL = "https://ollama.com/install.sh"
)

// Step status values.
const (
	StatusOK      = "ok"
	StatusSkipped = "skipped"
	StatusFailed  = "failed"
)

// Config drives the bootstrap chain.
type Config struct {
	// BaseURL is the Ollama daemon endpoint (default http://localhost:11434).
	BaseURL string
	// Model is the model to verify/pull (e.g. qwen2.5-coder:14b-128k).
	Model string
	// Timeout is the overall budget for the whole chain (default 15min).
	Timeout time.Duration
	// AllowAutoInstall enables the install step when the binary is missing.
	AllowAutoInstall bool
	// AllowAutoPull enables the model pull step when the model is missing.
	AllowAutoPull bool

	// DetectOnly reports the current state without any side effect: no
	// install, no daemon start, no model pull. Used by the boot hook so
	// `cosca run` / `cosca serve` never mutate the machine.
	DetectOnly bool

	// Per-step timeouts (documented defaults; override for tests/tuning).
	DaemonTimeout    time.Duration
	ReadinessTimeout time.Duration
	ValidateTimeout  time.Duration

	// Unexported test hooks. The bootstrap tests drive these to simulate a
	// machine without ollama, with a stopped daemon, or with failing
	// installers, without ever touching the real environment.
	pathEnv          string
	installerURL     string
	installScriptURL string
	runner           runner
}

// Step is a single bootstrap step with evidence.
type Step struct {
	Name     string `json:"name"`     // ex: "detect_os", "detect_binary", ...
	Status   string `json:"status"`   // ok | skipped | failed
	Detail   string `json:"detail"`   // evidence (ex: "ollama not found in PATH")
	Duration string `json:"duration"` // human duration of the step
}

// Report is the complete bootstrap result — one Step per stage with evidence.
type Report struct {
	Provider string `json:"provider"`
	OK       bool   `json:"ok"`
	Model    string `json:"model"`
	Steps    []Step `json:"steps"`
	Error    string `json:"error,omitempty"`
}

// runner abstracts process execution so tests can inject a fake that records
// calls without launching real processes.
type runner interface {
	// LookPath resolves a command name to an absolute path, honouring the
	// configured PATH when provided.
	LookPath(name string) (string, error)
	// CombinedOutput runs name with args and returns stdout+stderr.
	CombinedOutput(ctx context.Context, name string, args ...string) ([]byte, error)
	// StartDetached launches name in the background (detached from the
	// caller) so it survives the bootstrap process.
	StartDetached(name string, args ...string) error
}

// osRunner is the default runner backed by the real OS.
type osRunner struct{ pathEnv string }

func (r osRunner) LookPath(name string) (string, error) {
	if r.pathEnv == "" {
		return exec.LookPath(name)
	}
	return lookPathInPath(name, r.pathEnv)
}

func (r osRunner) CombinedOutput(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.CombinedOutput()
}

func (r osRunner) StartDetached(name string, args ...string) error {
	return startDetachedProcess(name, args...)
}

// applyDefaults fills zero-valued fields with documented defaults.
func (c *Config) applyDefaults() {
	if c.BaseURL == "" {
		c.BaseURL = DefaultBaseURL
	}
	c.BaseURL = strings.TrimSuffix(strings.TrimRight(c.BaseURL, "/"), "/v1")
	if c.Timeout <= 0 {
		c.Timeout = DefaultTimeout
	}
	if c.DaemonTimeout <= 0 {
		c.DaemonTimeout = DefaultDaemonTimeout
	}
	if c.ReadinessTimeout <= 0 {
		c.ReadinessTimeout = DefaultReadinessTimeout
	}
	if c.ValidateTimeout <= 0 {
		c.ValidateTimeout = DefaultValidateTimeout
	}
	if c.installerURL == "" {
		c.installerURL = DefaultInstallerURL
	}
	if c.installScriptURL == "" {
		c.installScriptURL = DefaultInstallScriptURL
	}
}

// EnsureOllama guarantees that Ollama is installed, running, has the
// configured model available, and actually responds — as far as the
// configuration allows. It is idempotent: every step that is already in the
// desired state is reported as "skipped" with its evidence.
//
// The returned Report always carries the full per-step evidence; OK reflects
// whether the final state is functional (daemon reachable and, when a model is
// configured, the model present). A non-nil error is returned only when the
// context is cancelled or the overall Timeout budget expires.
func EnsureOllama(ctx context.Context, cfg Config) (*Report, error) {
	cfg.applyDefaults()
	if cfg.Model == "" {
		cfg.Model = DefaultModel
	}
	if cfg.runner == nil {
		cfg.runner = osRunner{pathEnv: cfg.pathEnv}
	}

	// Overall budget: every step inherits cancellation from this context.
	if cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()
	}

	e := &engine{
		cfg:     cfg,
		r:       cfg.runner,
		client:  &http.Client{Timeout: cfg.Timeout},
		baseURL: cfg.BaseURL,
		goos:    runtime.GOOS,
	}

	// The 9-step chain, each step recording evidence.
	e.detectOS()
	e.detectBinary()
	e.detectDaemon(ctx)
	e.install(ctx)
	e.startDaemon(ctx)
	e.waitReadiness(ctx)
	e.verifyModel(ctx)
	e.pullModel(ctx)
	e.validateResponse(ctx)

	report := &Report{
		Provider: "ollama",
		Model:    cfg.Model,
		Steps:    e.steps,
	}
	report.OK = e.daemonReady && (cfg.Model == "" || e.modelPresent)

	switch {
	case !e.daemonReady && e.binary == "":
		report.Error = "ollama is not installed and not running. Run: cosca provider bootstrap --install --pull"
	case !e.daemonReady:
		report.Error = "ollama daemon is not reachable at " + cfg.BaseURL + ". Run: cosca provider bootstrap --pull (starts the daemon and pulls the model)"
	case !e.modelPresent:
		report.Error = fmt.Sprintf("model %q is not available. Run: cosca provider bootstrap --pull", cfg.Model)
	}

	if ctx.Err() != nil {
		return report, ctx.Err()
	}
	return report, nil
}

// ─── Engine ──────────────────────────────────────────────────────────────────

type engine struct {
	cfg         Config
	r           runner
	client      *http.Client
	baseURL     string
	goos        string
	binary      string // found ollama binary ("" when absent)
	daemonReady bool
	modelPresent bool
	steps       []Step
}

func (e *engine) addStep(name, status, detail string, started time.Time) Step {
	s := Step{
		Name:     name,
		Status:   status,
		Detail:   detail,
		Duration: time.Since(started).Round(time.Millisecond).String(),
	}
	e.steps = append(e.steps, s)
	log.Debug().
		Str("step", name).
		Str("status", status).
		Str("detail", detail).
		Str("duration", s.Duration).
		Msg("ollama bootstrap step")
	return s
}

// daemonReachable probes {baseURL}/api/tags and reports reachability.
func (e *engine) daemonReachable(ctx context.Context) (bool, string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.baseURL+"/api/tags", nil)
	if err != nil {
		return false, err.Error()
	}
	resp, err := e.client.Do(req)
	if err != nil {
		return false, err.Error()
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode == http.StatusOK {
		return true, ""
	}
	return false, fmt.Sprintf("HTTP %d", resp.StatusCode)
}

// ─── Step implementations ────────────────────────────────────────────────────

// detect_os records the operating system.
func (e *engine) detectOS() {
	start := time.Now()
	e.addStep("detect_os", StatusOK, "runtime.GOOS="+e.goos, start)
}

// detect_binary looks for the ollama binary in PATH and in known locations.
func (e *engine) detectBinary() {
	start := time.Now()
	bin, err := findBinary(e.r, e.cfg.pathEnv, e.goos)
	if err == nil {
		e.binary = bin
		e.addStep("detect_binary", StatusOK, "ollama found at "+bin, start)
		return
	}
	e.addStep("detect_binary", StatusFailed, err.Error(), start)
}

// detect_daemon probes the daemon endpoint and distinguishes "running",
// "installed but stopped" and "not reachable".
func (e *engine) detectDaemon(ctx context.Context) {
	start := time.Now()
	dctx, cancel := context.WithTimeout(ctx, e.cfg.DaemonTimeout)
	defer cancel()
	ok, detail := e.daemonReachable(dctx)
	switch {
	case ok:
		e.daemonReady = true
		e.addStep("detect_daemon", StatusOK, "daemon running (GET "+e.baseURL+"/api/tags -> 200)", start)
	case e.binary != "":
		e.addStep("detect_daemon", StatusFailed,
			fmt.Sprintf("installed but stopped (binary at %s; daemon not reachable: %s)", e.binary, detail), start)
	default:
		e.addStep("detect_daemon", StatusFailed, "daemon not reachable: "+detail, start)
	}
}

// install installs Ollama when missing, honoring AllowAutoInstall.
func (e *engine) install(ctx context.Context) {
	start := time.Now()
	if e.cfg.DetectOnly {
		e.addStep("install", StatusSkipped, "detection-only mode (DetectOnly=true)", start)
		return
	}
	if e.binary != "" {
		e.addStep("install", StatusSkipped, "already installed (found at "+e.binary+")", start)
		return
	}
	if e.daemonReady {
		e.addStep("install", StatusSkipped, "already running (no install needed)", start)
		return
	}
	if !e.cfg.AllowAutoInstall {
		e.addStep("install", StatusSkipped, "auto-install not authorized (AllowAutoInstall=false)", start)
		return
	}

	log.Info().Str("os", e.goos).Msg("ollama bootstrap: install authorized, discovering mechanism")
	var detail string
	switch e.goos {
	case "windows":
		detail = e.installWindows(ctx)
	case "linux":
		detail = e.installLinux(ctx)
	case "darwin":
		detail = "no auto-install for darwin; install manually from https://ollama.com/download"
	default:
		detail = "unsupported OS " + e.goos + "; install Ollama manually from https://ollama.com/download"
	}

	if strings.HasPrefix(detail, "ok:") {
		e.addStep("install", StatusOK, strings.TrimPrefix(detail, "ok:"), start)
		return
	}
	log.Warn().Str("step", "install").Msg("ollama bootstrap: install failed: " + detail)
	e.addStep("install", StatusFailed, detail, start)
}

// start_daemon starts a stopped but installed daemon.
func (e *engine) startDaemon(ctx context.Context) {
	start := time.Now()
	if e.cfg.DetectOnly {
		e.addStep("start_daemon", StatusSkipped, "detection-only mode (DetectOnly=true)", start)
		return
	}
	if e.daemonReady {
		e.addStep("start_daemon", StatusSkipped, "already running", start)
		return
	}
	if e.binary == "" {
		e.addStep("start_daemon", StatusSkipped, "not installed (nothing to start)", start)
		return
	}

	// Linux: prefer the systemd user service when present.
	if e.goos == "linux" {
		if _, err := e.r.LookPath("systemctl"); err == nil {
			out, err := e.r.CombinedOutput(ctx, "systemctl", "--user", "start", "ollama")
			if err == nil {
				e.addStep("start_daemon", StatusOK, "started via 'systemctl --user start ollama': "+summarize(string(out)), start)
				return
			}
			log.Warn().Err(err).Msg("ollama bootstrap: systemctl --user start ollama failed, falling back to 'ollama serve'")
		}
	}

	if err := e.r.StartDetached(e.binary, "serve"); err != nil {
		e.addStep("start_daemon", StatusFailed, "failed to start '"+e.binary+" serve': "+err.Error(), start)
		return
	}
	e.addStep("start_daemon", StatusOK, "started '"+e.binary+" serve' in background (detached)", start)
}

// wait_readiness polls /api/tags until the daemon answers or the timeout ends.
func (e *engine) waitReadiness(ctx context.Context) {
	start := time.Now()
	if e.daemonReady {
		e.addStep("wait_readiness", StatusSkipped, "already ready", start)
		return
	}
	if e.cfg.DetectOnly || e.binary == "" {
		e.addStep("wait_readiness", StatusSkipped, "daemon not running and not started (nothing to wait for)", start)
		return
	}

	probes := 0
	deadline := time.Now().Add(e.cfg.ReadinessTimeout)
	var lastErr string
	for {
		select {
		case <-ctx.Done():
			e.addStep("wait_readiness", StatusFailed, "context cancelled while waiting: "+ctx.Err().Error(), start)
			return
		default:
		}
		probes++
		dctx, cancel := context.WithTimeout(ctx, e.cfg.DaemonTimeout)
		ok, detail := e.daemonReachable(dctx)
		cancel()
		if ok {
			e.daemonReady = true
			e.addStep("wait_readiness", StatusOK,
				fmt.Sprintf("ready after %s (%d probes)", time.Since(start).Round(time.Millisecond), probes), start)
			return
		}
		lastErr = detail
		if time.Now().After(deadline) {
			break
		}
		select {
		case <-ctx.Done():
			e.addStep("wait_readiness", StatusFailed, "context cancelled while waiting: "+ctx.Err().Error(), start)
			return
		case <-time.After(DefaultPollInterval):
		}
	}
	e.addStep("wait_readiness", StatusFailed,
		fmt.Sprintf("not ready after %s (%d probes): last probe error: %s",
			time.Since(start).Round(time.Millisecond), probes, lastErr), start)
}

// verify_model checks whether the configured model is present on the daemon.
func (e *engine) verifyModel(ctx context.Context) {
	start := time.Now()
	if e.cfg.Model == "" {
		e.addStep("verify_model", StatusSkipped, "no model configured (nothing to verify)", start)
		return
	}
	if !e.daemonReady {
		e.addStep("verify_model", StatusFailed, "daemon not reachable; cannot verify model", start)
		return
	}
	names, err := e.fetchModelNames(ctx)
	if err != nil {
		e.addStep("verify_model", StatusFailed, "failed to list models: "+err.Error(), start)
		return
	}
	if modelPresent(names, e.cfg.Model) {
		e.modelPresent = true
		e.addStep("verify_model", StatusOK, fmt.Sprintf("model %q present", e.cfg.Model), start)
		return
	}
	e.addStep("verify_model", StatusFailed,
		fmt.Sprintf("model %q missing (available: %s)", e.cfg.Model, summarizeList(names)), start)
}

// pull_model pulls the configured model when missing, honoring AllowAutoPull.
func (e *engine) pullModel(ctx context.Context) {
	start := time.Now()
	if e.cfg.DetectOnly {
		e.addStep("pull_model", StatusSkipped, "detection-only mode (DetectOnly=true)", start)
		return
	}
	if e.cfg.Model == "" {
		e.addStep("pull_model", StatusSkipped, "no model configured", start)
		return
	}
	if e.modelPresent {
		e.addStep("pull_model", StatusSkipped, "already present", start)
		return
	}
	if !e.cfg.AllowAutoPull {
		e.addStep("pull_model", StatusSkipped, "auto-pull not authorized (AllowAutoPull=false)", start)
		return
	}
	if !e.daemonReady {
		e.addStep("pull_model", StatusFailed, "daemon not ready; cannot pull model", start)
		return
	}

	// Prefer the CLI binary; fall back to the native /api/pull endpoint.
	if e.binary != "" {
		out, err := e.r.CombinedOutput(ctx, e.binary, "pull", e.cfg.Model)
		if err != nil {
			e.addStep("pull_model", StatusFailed,
				fmt.Sprintf("'ollama pull %s' failed: %v (%s)", e.cfg.Model, err, summarize(string(out))), start)
			return
		}
		e.modelPresent = true
		e.addStep("pull_model", StatusOK,
			fmt.Sprintf("'ollama pull %s' completed in %s", e.cfg.Model, time.Since(start).Round(time.Millisecond)), start)
		return
	}

	if err := e.pullModelViaAPI(ctx); err != nil {
		e.addStep("pull_model", StatusFailed, "POST /api/pull failed: "+err.Error(), start)
		return
	}
	e.modelPresent = true
	e.addStep("pull_model", StatusOK,
		fmt.Sprintf("POST /api/pull %q completed in %s", e.cfg.Model, time.Since(start).Round(time.Millisecond)), start)
}

// validate_response makes a real minimal model call (POST /api/generate) and
// verifies a non-empty response.
func (e *engine) validateResponse(ctx context.Context) {
	start := time.Now()
	if e.cfg.DetectOnly {
		e.addStep("validate_response", StatusSkipped, "detection-only mode (DetectOnly=true)", start)
		return
	}
	if e.cfg.Model == "" {
		e.addStep("validate_response", StatusSkipped, "no model configured", start)
		return
	}
	if !e.daemonReady {
		e.addStep("validate_response", StatusFailed, "daemon not ready; cannot validate", start)
		return
	}
	if !e.modelPresent {
		e.addStep("validate_response", StatusFailed,
			fmt.Sprintf("model %q not available; cannot validate response", e.cfg.Model), start)
		return
	}

	vctx, cancel := context.WithTimeout(ctx, e.cfg.ValidateTimeout)
	defer cancel()

	body, _ := json.Marshal(map[string]interface{}{
		"model":  e.cfg.Model,
		"prompt": "ping",
		"stream": false,
	})
	req, err := http.NewRequestWithContext(vctx, http.MethodPost, e.baseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		e.addStep("validate_response", StatusFailed, "create request: "+err.Error(), start)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		e.addStep("validate_response", StatusFailed, "generate request failed: "+err.Error(), start)
		return
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		e.addStep("validate_response", StatusFailed, "read response: "+err.Error(), start)
		return
	}
	if resp.StatusCode != http.StatusOK {
		e.addStep("validate_response", StatusFailed,
			fmt.Sprintf("generate HTTP %d: %s", resp.StatusCode, summarize(string(data))), start)
		return
	}
	var out struct {
		Response string `json:"response"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		e.addStep("validate_response", StatusFailed, "parse response: "+err.Error(), start)
		return
	}
	if strings.TrimSpace(out.Response) == "" {
		e.addStep("validate_response", StatusFailed, "empty response from model", start)
		return
	}
	e.addStep("validate_response", StatusOK,
		fmt.Sprintf("model responded in %s", time.Since(start).Round(time.Millisecond)), start)
}

// ─── Installers ──────────────────────────────────────────────────────────────

// installMechanism describes one candidate install path and why it was chosen.
type installMechanism struct {
	name   string
	reason string
	argv   []string // full argv: program + arguments
}

// runInstallCmd runs a full argv through the runner.
func (e *engine) runInstallCmd(ctx context.Context, argv []string) ([]byte, error) {
	if len(argv) == 0 {
		return nil, fmt.Errorf("empty install command")
	}
	return e.r.CombinedOutput(ctx, argv[0], argv[1:]...)
}

// installWindows tries, in order, winget → choco → scoop → direct download of
// the official installer. Every mechanism is detected (never assumed) and the
// chosen one is recorded with its evidence.
func (e *engine) installWindows(ctx context.Context) string {
	var mechs []installMechanism
	if p, err := e.r.LookPath("winget"); err == nil {
		mechs = append(mechs, installMechanism{
			name:   "winget",
			reason: "detected winget at " + p,
			argv:   []string{"winget", "install", "--id", "Ollama.Ollama", "-e", "--silent", "--accept-package-agreements"},
		})
	}
	if p, err := e.r.LookPath("choco"); err == nil {
		mechs = append(mechs, installMechanism{
			name:   "chocolatey",
			reason: "detected choco at " + p,
			argv:   []string{"choco", "install", "ollama", "-y"},
		})
	}
	if p, err := e.r.LookPath("scoop"); err == nil {
		mechs = append(mechs, installMechanism{
			name:   "scoop",
			reason: "detected scoop at " + p,
			argv:   []string{"scoop", "install", "ollama"},
		})
	}

	for _, m := range mechs {
		log.Info().Str("mechanism", m.name).Str("reason", m.reason).Msg("ollama bootstrap: installing via package manager")
		out, err := e.runInstallCmd(ctx, m.argv)
		if err == nil {
			if bin, ferr := findBinary(e.r, e.cfg.pathEnv, e.goos); ferr == nil {
				e.binary = bin
				return "ok: installed via " + m.name + " (" + m.reason + "); ollama found at " + bin
			}
			return "ok: installed via " + m.name + " (" + m.reason + "); binary not re-detected yet (PATH refresh may be required): " + summarize(string(out))
		}
		log.Warn().Str("mechanism", m.name).Err(err).Msg("ollama bootstrap: install mechanism failed")
	}

	// Last resort: direct download of the official installer.
	return e.installWindowsDirect(ctx, mechs)
}

// installWindowsDirect downloads OllamaSetup.exe and runs it silently (/S).
func (e *engine) installWindowsDirect(ctx context.Context, mechs []installMechanism) string {
	log.Info().Str("url", e.cfg.installerURL).
		Msg("ollama bootstrap: no package manager available, downloading official Ollama installer")
	tmp, err := os.CreateTemp("", "ollama-setup-*.exe")
	if err != nil {
		return installFailure(mechs, "create temp file: "+err.Error())
	}
	defer os.Remove(tmp.Name())

	if err := e.downloadTo(ctx, e.cfg.installerURL, tmp); err != nil {
		return installFailure(mechs, "installer download failed: "+err.Error())
	}
	if err := tmp.Close(); err != nil {
		return installFailure(mechs, "close temp file: "+err.Error())
	}

	if out, err := e.runInstallCmd(ctx, []string{tmp.Name(), "/S"}); err == nil {
		if bin, ferr := findBinary(e.r, e.cfg.pathEnv, e.goos); ferr == nil {
			e.binary = bin
			return "ok: installed via direct download of official installer (silent /S, no winget/choco/scoop detected); ollama found at " + bin
		}
		return "ok: installed via direct download of official installer (silent /S, no winget/choco/scoop detected); binary not re-detected yet (PATH refresh may be required)"
	} else {
		return installFailure(mechs, "installer execution failed: "+err.Error()+" ("+summarize(string(out))+")")
	}
}

// installLinux installs via the official install script, downloaded to a temp
// file and executed with separated args (`sh <script>`) — never a piped
// `sh -c` string.
func (e *engine) installLinux(ctx context.Context) string {
	if _, err := e.r.LookPath("curl"); err != nil {
		return "failed: curl not found; install Ollama manually: curl -fsSL https://ollama.com/install.sh | sh (see https://github.com/ollama/ollama/blob/main/docs/linux.md)"
	}
	if _, err := e.r.LookPath("sh"); err != nil {
		return "failed: sh not found; install Ollama manually: curl -fsSL https://ollama.com/install.sh | sh"
	}

	log.Info().Str("url", e.cfg.installScriptURL).Msg("ollama bootstrap: downloading official install script")
	tmp, err := os.CreateTemp("", "ollama-install-*.sh")
	if err != nil {
		return "failed: create temp file: " + err.Error()
	}
	defer os.Remove(tmp.Name())

	if err := e.downloadTo(ctx, e.cfg.installScriptURL, tmp); err != nil {
		return "failed: install script download failed: " + err.Error()
	}
	if err := tmp.Close(); err != nil {
		return "failed: close temp file: " + err.Error()
	}

	// Sanity: the official script is a POSIX shell script.
	data, err := os.ReadFile(tmp.Name())
	if err != nil {
		return "failed: read install script: " + err.Error()
	}
	if !strings.HasPrefix(strings.TrimSpace(string(data)), "#!") {
		return "failed: downloaded install script does not look like a shell script; refusing to execute"
	}

	log.Info().Str("script", tmp.Name()).Msg("ollama bootstrap: executing official install script")
	out, err := e.runInstallCmd(ctx, []string{"sh", tmp.Name()})
	if err != nil {
		return "failed: official install script failed: " + err.Error() + " (" + summarize(string(out)) + ")"
	}
	if bin, ferr := findBinary(e.r, e.cfg.pathEnv, e.goos); ferr == nil {
		e.binary = bin
		return "ok: installed via official install script; ollama found at " + bin
	}
	return "ok: installed via official install script; binary not re-detected yet (PATH refresh may be required)"
}

// downloadTo streams url into f (closing f on error).
func (e *engine) downloadTo(ctx context.Context, url string, f *os.File) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := e.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if _, err := io.Copy(f, io.LimitReader(resp.Body, 256*1024*1024)); err != nil {
		return err
	}
	return nil
}

// installFailure builds the failed-install evidence, listing which mechanisms
// were tried and the underlying cause, plus manual instructions.
func installFailure(mechs []installMechanism, cause string) string {
	var tried []string
	for _, m := range mechs {
		tried = append(tried, m.name)
	}
	if len(tried) == 0 {
		tried = []string{"none detected"}
	}
	return fmt.Sprintf(
		"failed: no install mechanism succeeded (tried: %s) and direct download failed: %s. Install Ollama manually from https://ollama.com/download (OllamaSetup.exe) or run: winget install --id Ollama.Ollama",
		strings.Join(tried, ", "), cause)
}

// ─── HTTP / model helpers ────────────────────────────────────────────────────

// fetchModelNames lists the models known by the daemon via /api/tags.
func (e *engine) fetchModelNames(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.baseURL+"/api/tags", nil)
	if err != nil {
		return nil, err
	}
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return nil, err
	}
	var out struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(out.Models))
	for _, m := range out.Models {
		names = append(names, m.Name)
	}
	return names, nil
}

// pullModelViaAPI pulls via POST /api/pull (NDJSON stream) without requiring
// the CLI binary.
func (e *engine) pullModelViaAPI(ctx context.Context) error {
	body, _ := json.Marshal(map[string]interface{}{
		"model":  e.cfg.Model,
		"stream": false,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/api/pull", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// NDJSON: {"status":"..."} lines; "success" is the terminal status.
	sc := bufio.NewScanner(resp.Body)
	last := ""
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line != "" {
			last = line
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	var out struct {
		Status string `json:"status"`
		Error  string `json:"error,omitempty"`
	}
	if err := json.Unmarshal([]byte(last), &out); err != nil {
		return fmt.Errorf("unexpected /api/pull result: %s", summarize(last))
	}
	switch {
	case out.Status == "success":
		return nil
	case out.Error != "":
		return fmt.Errorf("%s", out.Error)
	default:
		return fmt.Errorf("unexpected /api/pull result: %s", summarize(last))
	}
}

// modelPresent reports whether names contains model (exact or as prefix).
func modelPresent(names []string, model string) bool {
	for _, n := range names {
		if n == model || strings.HasPrefix(n, model+":") {
			return true
		}
	}
	return false
}

// ─── Binary discovery ────────────────────────────────────────────────────────

// findBinary resolves the ollama binary via PATH (honouring pathEnv) and then
// through known platform-specific locations.
func findBinary(r runner, pathEnv, goos string) (string, error) {
	if p, err := r.LookPath("ollama"); err == nil && p != "" {
		return p, nil
	}
	for _, p := range knownOllamaPaths(goos) {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("ollama not found in PATH or known locations")
}

// knownOllamaPaths returns the platform-specific default install locations.
func knownOllamaPaths(goos string) []string {
	switch goos {
	case "windows":
		paths := []string{`C:\Program Files\Ollama\ollama.exe`}
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			paths = append(paths, filepath.Join(local, "Programs", "Ollama", "ollama.exe"))
		}
		if profile := os.Getenv("USERPROFILE"); profile != "" {
			paths = append(paths, filepath.Join(profile, ".ollama", "ollama.exe"))
		}
		return paths
	default:
		paths := []string{"/usr/local/bin/ollama", "/usr/bin/ollama"}
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			paths = append(paths, filepath.Join(home, ".local", "bin", "ollama"))
		}
		return paths
	}
}

// lookPathInPath searches name inside an explicit PATH (with PATHEXT
// resolution on Windows). Used by tests and by osRunner when pathEnv is set.
func lookPathInPath(name, pathEnv string) (string, error) {
	dirs := filepath.SplitList(pathEnv)
	exts := []string{""}
	if runtime.GOOS == "windows" {
		pathext := os.Getenv("PATHEXT")
		if pathext == "" {
			exts = []string{".exe", ".cmd", ".bat", ""}
		} else {
			for _, e := range strings.Split(pathext, ";") {
				if e = strings.TrimSpace(e); e != "" {
					exts = append(exts, strings.ToLower(e))
				}
			}
		}
	}
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		for _, ext := range exts {
			candidate := filepath.Join(dir, name+ext)
			if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
				return candidate, nil
			}
		}
	}
	return "", fmt.Errorf("%s not found in PATH", name)
}

// ─── Small helpers ───────────────────────────────────────────────────────────

// summarize truncates long command output for evidence strings.
func summarize(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}

// summarizeList renders a model list compactly.
func summarizeList(items []string) string {
	if len(items) == 0 {
		return "(none)"
	}
	if len(items) > 5 {
		return strings.Join(items[:5], ", ") + fmt.Sprintf(", ... (%d total)", len(items))
	}
	return strings.Join(items, ", ")
}
