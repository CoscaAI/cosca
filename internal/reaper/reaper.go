// Package reaper implements a deterministic, zero-LLM "resource reaper": given
// the ledger of resources recorded during a run {kind, id, match, retain, at},
// it dispatches each entry to the per-kind reaper and disposes only what it can
// positively verify. This materializes invariant I7 — containment / clean
// environment as a verifiable mechanism, not hope.
//
// The safety contract has two non-negotiable guarantees:
//
//   - Identity-match. A reaper acts on a resource only if the live resource
//     still matches the identity marker recorded when it was created. A process
//     reaper never kills a PID whose command line does not contain the recorded
//     `match` (anti-hijack: it does not kill a process that is not its own); a
//     docker reaper never removes a container/volume blindly — it only acts on
//     what actually exists and reports "missing" otherwise (no silent no-op on
//     a reused name).
//
//   - Containment. A temporary-directory reaper only removes a path that is
//     canonically below one of the allowed roots, after resolving symlinks
//     (realpath). It never `rm -rf`s an arbitrary path (anti-escape).
//
// Entries are the resource records (the "ledger of resources"). They are meant
// to be persisted in the append-only, tamper-evident Cosca ledger
// (internal/ledger) — the same durable store used across the kernel — and the
// reaper is iterated over that record. Each Entry maps onto a ledger key via
// Entry.LedgerKey(), so a resource registry can be reaped deterministically.
//
// The package is stdlib-only. Platform-specific process introspection and
// termination live behind build tags (unix / windows / other), mirroring the
// cross-platform conventions used elsewhere in the repository.
package reaper

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Kind is the resource class a reaper handles.
const (
	// KindProcess reaps a running process (with identity-match on its command
	// line and SIGTERM→SIGKILL escalation).
	KindProcess = "process"
	// KindDocker reaps a Docker container (docker rm --force --volumes).
	KindDocker = "docker"
	// KindDockerVolume reaps a Docker volume (docker volume rm --force).
	KindDockerVolume = "docker-volume"
	// KindTmpDir reaps a temporary directory, only if contained inside an
	// allowed root (realpath containment).
	KindTmpDir = "tmpdir"
)

// Entry is a single recorded resource to reap.
type Entry struct {
	// Kind is the reaper class (one of the Kind* constants). Unknown kinds are
	// skipped unless a custom reaper is registered for them.
	Kind string `json:"kind"`
	// ID identifies the resource: a PID (process), a container/volume name
	// (docker*), or an absolute path (tmpdir).
	ID string `json:"id"`
	// Match is the identity marker the resource must still carry to be reaped.
	// For process it is a fragment of the command line; for docker/tmpdir it may
	// be empty (existence + containment are the identity checks).
	Match string `json:"match,omitempty"`
	// Retain protects the entry from disposal forever, unless a Purge reaps it.
	Retain bool `json:"retain,omitempty"`
	// At is the time the resource was recorded (UTC), for provenance.
	At time.Time `json:"at,omitempty"`
}

// LedgerKey returns the canonical key used to persist this entry in the Cosca
// ledger (internal/ledger) as a resource record. It is namespaced under
// "reaper:" so a search for the token "reaper" and per-kind/id survives
// collision with unrelated keys.
func (e Entry) LedgerKey() string {
	return "reaper:" + e.Kind + ":" + e.ID
}

// Status is the semantic outcome of reaping one entry.
type Status string

const (
	// StatusReaped: the resource existed, matched identity/containment and was
	// disposed.
	StatusReaped Status = "reaped"
	// StatusMissing: the resource did not exist at reap time.
	StatusMissing Status = "missing"
	// StatusSkipped: the resource was intentionally not touched (identity
	// mismatch, outside allowed roots, no reaper, no identity marker, still
	// alive after escalation).
	StatusSkipped Status = "skipped"
	// StatusRetained: the entry is Retain-protected and the run was not a purge.
	StatusRetained Status = "retained"
	// StatusError: an infrastructure failure occurred while reaping.
	StatusError Status = "error"
)

// Outcome is the result of reaping a single entry.
type Outcome struct {
	Status Status
	Reason string
}

// SkippedEntry pairs an entry with the reason it was left untouched.
type SkippedEntry struct {
	Entry  Entry  `json:"entry"`
	Reason string `json:"reason"`
}

// ErroredEntry pairs an entry with the infrastructure error that aborted reaping.
type ErroredEntry struct {
	Entry Entry `json:"entry"`
	Err   error `json:"-"`
}

// Report aggregates the disposition of a Reap/Purge pass over a resource ledger.
type Report struct {
	Reaped   []Entry       `json:"reaped,omitempty"`
	Missing  []Entry       `json:"missing,omitempty"`
	Skipped  []SkippedEntry `json:"skipped,omitempty"`
	Retained []Entry       `json:"retained,omitempty"`
	Errors   []ErroredEntry `json:"errors,omitempty"`
}

// SummaryLine renders a one-line readable summary, mirroring the report style
// used elsewhere in the repository.
func (r *Report) SummaryLine() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("reaper: %d reaped, %d missing, %d skipped, %d retained, %d errors",
		len(r.Reaped), len(r.Missing), len(r.Skipped), len(r.Retained), len(r.Errors)))
	return b.String()
}

// Reaper disposes a single recorded resource of a specific kind.
type Reaper interface {
	// Kind returns the resource class this reaper handles.
	Kind() string
	// Reap disposes entry (after its own identity/containment verification),
	// returning the outcome. An error is reserved for infrastructure failures.
	Reap(ctx context.Context, entry Entry, rc ReapContext) (Outcome, error)
}

// ExecResult is the outcome of running a command through an ExecFn.
type ExecResult struct {
	Code   int    // exit code (0 = success, 124 = timed out, 127 = not found)
	Stdout string
	Stderr string
}

// ExecFn runs a command with a bounded timeout. It is injected so the docker
// reaper (and any external-tool reaper) can be tested with a fake and does not
// depend on the external binary being present.
type ExecFn func(ctx context.Context, command string, args []string, timeout time.Duration) (ExecResult, error)

// ReapContext carries per-pass capabilities that are shared by every reaper.
type ReapContext struct {
	// Cwd is the working directory used to derive default allowed tmp roots.
	Cwd string
	// AllowedTmpRoots are the canonical roots inside which a tmpdir resource may
	// be removed. Entries outside any of them are skipped.
	AllowedTmpRoots []string
	// Exec is the command runner used by tool-based reapers (docker*).
	Exec ExecFn
}

// containedByAllowedRoot reports whether canonical is strictly below one of the
// allowed roots (never equal, never an escape via `..`).
func (rc ReapContext) containedByAllowedRoot(canonical string) bool {
	for _, root := range rc.AllowedTmpRoots {
		if belowRoot(canonical, root) {
			return true
		}
	}
	return false
}

// ReapOptions controls a whole Reap/Purge pass.
type ReapOptions struct {
	// Purge bypasses the Retain protection (dispose even retained entries).
	Purge bool
	// Reapers augments/overrides the registry per kind. When a kind is present
	// here it takes precedence over the registry default.
	Reapers map[string]Reaper
	// Exec overrides the default command runner (injectable for tests).
	Exec ExecFn
	// AllowedTmpRoots overrides the default tmp roots. When empty, the defaults
	// (os.TempDir() plus WorkspaceRoot/evals/results) are canonicalized and used.
	AllowedTmpRoots []string
	// WorkspaceRoot is used to derive the default evals/results tmp root.
	WorkspaceRoot string
}

// buildContext derives the shared ReapContext from the options.
func (o ReapOptions) buildContext() ReapContext {
	roots := o.AllowedTmpRoots
	if len(roots) == 0 {
		roots = defaultAllowedTmpRoots(o.WorkspaceRoot)
	}
	execFn := o.Exec
	if execFn == nil {
		execFn = defaultExec
	}
	return ReapContext{
		Cwd:             o.WorkspaceRoot,
		AllowedTmpRoots: roots,
		Exec:            execFn,
	}
}

// defaultExec runs a command via the OS (os/exec) with a context timeout and
// captures stdout/stderr. It maps ExitError to its code and infra errors
// (ENOENT, timeout) to 127/124 so tool-based reapers can report them
// conservatively.
func defaultExec(ctx context.Context, command string, args []string, timeout time.Duration) (ExecResult, error) {
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cctx, command, args...)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	res := ExecResult{Stdout: out.String(), Stderr: errb.String()}
	if err == nil {
		res.Code = 0
		return res, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		res.Code = exitErr.ExitCode()
		return res, nil
	}
	if errors.Is(cctx.Err(), context.DeadlineExceeded) {
		res.Code = 124
		return res, nil
	}
	res.Code = 127
	return res, err
}

// defaultAllowedTmpRoots canonicalizes the permitted tmp roots: the OS temp dir
// plus the evals/results directory under the workspace, mirroring the openwork
// reaper's notion of allowed roots and the Cosca eval workspace layout.
func defaultAllowedTmpRoots(workspaceRoot string) []string {
	roots := []string{os.TempDir()}
	if workspaceRoot != "" {
		roots = append(roots, filepath.Join(workspaceRoot, "evals", "results"))
	}
	var out []string
	seen := map[string]struct{}{}
	for _, r := range roots {
		c, err := canonicalPath(r)
		if err != nil {
			continue
		}
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		out = append(out, c)
	}
	return out
}

// Registry holds the per-kind reapers and orchestrates a Reap/Purge pass.
type Registry struct {
	mu      sync.RWMutex
	reapers map[string]Reaper
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{reapers: map[string]Reaper{}}
}

// DefaultRegistry returns a Registry with the built-in reapers for process,
// docker, docker-volume and tmpdir registered.
func DefaultRegistry() *Registry {
	reg := NewRegistry()
	reg.Register(ProcessReaper())
	reg.Register(DockerReaper())
	reg.Register(DockerVolumeReaper())
	reg.Register(TmpDirReaper())
	return reg
}

// Register adds (or replaces) a reaper for its kind.
func (r *Registry) Register(rp Reaper) {
	if rp == nil {
		return
	}
	k := rp.Kind()
	if k == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.reapers == nil {
		r.reapers = map[string]Reaper{}
	}
	r.reapers[k] = rp
}

// Lookup returns the registered reaper for kind, or nil.
func (r *Registry) Lookup(kind string) Reaper {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.reapers[kind]
}

// Reap iterates over the resource ledger and disposes each entry through its
// reaper. Retained entries are preserved unless opts.Purge is set. It returns a
// Report grouping every disposition; it never throws on a reaper failure — a
// failure lands in Report.Errors.
func (r *Registry) Reap(ctx context.Context, entries []Entry, opts ReapOptions) (*Report, error) {
	rc := opts.buildContext()
	report := &Report{}
	for _, e := range entries {
		if e.Retain && !opts.Purge {
			report.Retained = append(report.Retained, e)
			continue
		}

		rp := opts.Reapers[e.Kind]
		if rp == nil {
			rp = r.Lookup(e.Kind)
		}
		if rp == nil {
			report.Skipped = append(report.Skipped, SkippedEntry{Entry: e, Reason: "no reaper for kind"})
			continue
		}

		out, err := rp.Reap(ctx, e, rc)
		if err != nil {
			report.Errors = append(report.Errors, ErroredEntry{Entry: e, Err: err})
			continue
		}
		switch out.Status {
		case StatusReaped:
			report.Reaped = append(report.Reaped, e)
		case StatusMissing:
			report.Missing = append(report.Missing, e)
		case StatusRetained:
			report.Retained = append(report.Retained, e)
		default:
			report.Skipped = append(report.Skipped, SkippedEntry{Entry: e, Reason: out.Reason})
		}
	}
	return report, nil
}

// Purge is Reap with opts.Purge forced to true: Retain protection is bypassed.
func (r *Registry) Purge(ctx context.Context, entries []Entry, opts ReapOptions) (*Report, error) {
	opts.Purge = true
	return r.Reap(ctx, entries, opts)
}

// belowRoot reports whether path is a strict descendant of root after cleaning.
// It is separator-aware and rejects `..` escapes and equality with the root, so
// a tmpdir resource can never be an arbitrary path or the allowed root itself.
func belowRoot(path, root string) bool {
	if filepath.Clean(path) == filepath.Clean(root) {
		return false
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	if rel == "." || rel == "" {
		return false
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return false
	}
	return true
}

// canonicalPath resolves p to its canonical absolute path, following symlinks on
// the longest existing prefix and appending the (possibly non-existent) suffix.
// This is the Go equivalent of the openwork reaper's realpath-with-suffix and
// is what makes tmpdir containment immune to symlink/`..` escapes.
func canonicalPath(p string) (string, error) {
	candidate, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	suffix := []string{}
	for {
		resolved, err := filepath.EvalSymlinks(candidate)
		if err == nil {
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return filepath.Clean(resolved), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(candidate)
		if parent == candidate {
			return "", fmt.Errorf("reaper: cannot canonicalize %q: %w", p, err)
		}
		suffix = append(suffix, filepath.Base(candidate))
		candidate = parent
	}
}
