// Package config — workspace constraints.
//
// Constraints are project-level safety rails set at init time.
// They are enforced by the jail (network, read-only) and by the
// runtime (agent blocking, file limits, timeouts).
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// ConstraintsFileName is the constraints file inside the .cosca/ directory.
const ConstraintsFileName = "constraints.yaml"

// Constraints define safety rails for a workspace. Set at init time
// and enforced by the jail (network, read-only) and runtime (agent
// blocking, file limits, timeouts).
type Constraints struct {
	// Docs controls whether documentation agents are allowed.
	// When false, cosca-documentation and cosca-specialist-documentation-writer
	// are blocked.
	Docs bool `yaml:"docs"`

	// Network controls whether the jailed process has network access.
	// When false, --unshare-net is added to bwrap.
	Network bool `yaml:"network"`

	// Test controls whether test agents are allowed.
	// When false, cosca-testing and test specialists are blocked.
	Test bool `yaml:"test"`

	// Build controls whether build/exec commands are allowed.
	// When false, bash and exec-based operations are blocked.
	Build bool `yaml:"build"`

	// ReadOnly makes the workspace read-only inside the jail.
	// When true, --ro-bind is used instead of --bind for the workspace.
	ReadOnly bool `yaml:"read_only"`

	// MaxFiles limits the number of files the agent can create (0 = unlimited).
	MaxFiles int `yaml:"max_files"`

	// MaxTime limits the duration per operation (parsed as duration string).
	// Empty or "0s" means unlimited.
	MaxTimeStr string `yaml:"max_time"`
}

// MaxTimeDuration parses MaxTimeStr and returns the duration or 0 (unlimited).
func (c *Constraints) MaxTimeDuration() time.Duration {
	if c.MaxTimeStr == "" || c.MaxTimeStr == "0s" {
		return 0
	}
	d, err := time.ParseDuration(c.MaxTimeStr)
	if err != nil {
		return 0
	}
	return d
}

// DefaultConstraints returns permissive defaults (everything allowed).
// These are the documented default for a workspace WITHOUT a policy file
// (first run). A file that exists but is corrupt must NEVER fall back here.
func DefaultConstraints() *Constraints {
	return &Constraints{
		Docs:    true,
		Network: true,
		Test:    true,
		Build:   true,
	}
}

// RestrictiveConstraints returns fail-closed defaults (everything blocked,
// workspace read-only). Used when a constraints file exists but cannot be
// read or parsed: the jail must CLOSE, not open, when the policy breaks.
func RestrictiveConstraints() *Constraints {
	return &Constraints{
		Docs:     false,
		Network:  false,
		Test:     false,
		Build:    false,
		ReadOnly: true,
	}
}

// LoadConstraints loads constraints from a .cosca/constraints.yaml file
// inside the given workspace directory.
//
// FAIL-CLOSED: "no file" and "corrupt file" are treated differently.
//   - File does not exist → DefaultConstraints() (permissive, documented
//     legitimate default for a first run without a policy).
//   - File exists but is unreadable or corrupt → RestrictiveConstraints()
//     (everything blocked, read-only) plus a descriptive error and an
//     unmistakable alert. The jail closes when the policy breaks.
func LoadConstraints(workspaceDir string) (*Constraints, error) {
	path := filepath.Join(workspaceDir, ".cosca", ConstraintsFileName)
	c := DefaultConstraints()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil // no constraints file = permissive defaults (documented)
		}
		// The file exists but cannot be read. Fail-closed: never fall back
		// to permissive defaults when the policy is broken.
		err = fmt.Errorf("read constraints %s: %w", path, err)
		log.Error().Err(err).Str("path", path).
			Msg("constraints unreadable — FAIL-CLOSED: restrictive defaults applied")
		return RestrictiveConstraints(), err
	}

	if err := yaml.Unmarshal(data, c); err != nil {
		// The file exists but is corrupt. Fail-closed.
		err = fmt.Errorf("parse constraints %s: %w", path, err)
		log.Error().Err(err).Str("path", path).
			Msg("constraints corrupt — FAIL-CLOSED: restrictive defaults applied")
		return RestrictiveConstraints(), err
	}

	return c, nil
}

// SaveConstraints writes the constraints to .cosca/constraints.yaml.
func SaveConstraints(workspaceDir string, c *Constraints) error {
	coscaDir := filepath.Join(workspaceDir, ".cosca")
	// The .cosca directory contains credentials and private knowledge —
	// owner-only (M6b).
	if err := os.MkdirAll(coscaDir, 0700); err != nil {
		return err
	}

	path := filepath.Join(coscaDir, ConstraintsFileName)
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}
