// Package tools registers every Cosca-built tool that ships as a global binary
// installable into ~/.cosca/bin and launchable from anywhere (the Don's
// "one command per tool" pattern — `cosca desktop` is the reference).
//
// A tool is any project built with the Cosca toolchain that deserves a
// first-class global command. Each entry documents how to build it and where
// its production binary is installed. The desktop command in internal/cli
// reads this registry for the canonical install path; future tools (e.g. a
// cosca agent cockpit, a metrics dashboard) add their own entry here and a
// thin cobra launcher in internal/cli.
package tools

import (
	"os"
	"path/filepath"
)

// Tool describes one Cosca-built global tool.
type Tool struct {
	// Name is the command name: `cosca <Name>` (lowercase).
	Name string
	// Description is a one-line human summary.
	Description string
	// Path returns the installed binary path (global, ~/.cosca/bin/<name>-<Name>).
	Path func() string
	// Build returns the function that builds the production binary. Nil means
	// the tool has no self-contained build (e.g. a thin launcher over another
	// binary). The desktop entry wires the wails build.
	Build func() error
}

// installedPath builds ~/.cosca/bin/<name>.
func installedPath(name string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.Getenv("HOME"), ".cosca", "bin", name)
	}
	return filepath.Join(home, ".cosca", "bin", name)
}

// Tools is the registry of Cosca-built global tools, keyed by command name.
var Tools = map[string]Tool{
	"desktop": {
		Name:        "desktop",
		Description: "COSCA Desktop — cockpit enterprise (Wails + React), abre na pasta atual ou na informada",
		Path: func() string {
			return installedPath("cosca-desktop")
		},
		// Build é implementado em internal/cli (buildDesktop) porque envolve o
		// wails CLI, timeout e streaming; o registro documenta o padrão.
		Build: nil,
	},
	"slop": {
		Name:        "slop",
		Description: "slopguard — fiscal do Padrão COSCA §4 (anti-AI-slop)",
		Path: func() string {
			return installedPath("slopguard")
		},
		// Build é implementado em internal/cli (buildSlop): go build do
		// slopguard em ~/Documents/projects/slopguard.
		Build: nil,
	},
	"voice": {
		Name:        "voice",
		Description: "Assistente de voz cosca (projeto independente — torch/whisper/kokoro), sob demanda via cosca voice start/stop",
		Path: func() string {
			home, _ := os.UserHomeDir()
			return filepath.Join(home, "Documents", "projects", "cosca-voice", "voice_conversation.py")
		},
		Build: nil,
	},
}

// InstalledPath returns the installed binary path for a tool by name.
func InstalledPath(name string) string {
	if t, ok := Tools[name]; ok {
		return t.Path()
	}
	return installedPath(name)
}
