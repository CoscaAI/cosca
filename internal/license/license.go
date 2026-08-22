// Package license implements the Cosca security-key (chave de segurança)
// verification mechanism defined in LICENSE.md Section 2.
//
// The security key that authenticates the Software is the official
// CoscaAI repository itself (https://github.com/CoscaAI). Authenticity is
// verified by three cumulative factors:
//
//   - F1 module   — go.mod declares EXACTLY "github.com/CoscaAI/cosca"
//     (never just a prefix — github.com/CoscaAI/cosca-client must NOT match)
//   - F2 manifest — .cosca/manifest.yaml declares identity "cosca-kernel"
//     AND module "github.com/CoscaAI/cosca"
//   - F3 build    — the binary embeds a traceable version + commit hash
//
// A Software that fails any factor is NOT authenticated and enjoys no
// license to use.
package license

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"

	embedcosca "github.com/CoscaAI/cosca/internal/embed/cosca"
	pkgcosca "github.com/CoscaAI/cosca/pkg/cosca"
)

// AuthFactor identifies one of the three security-key factors.
type AuthFactor string

const (
	// FactorModule is F1: go.mod module == github.com/CoscaAI/cosca
	// (exact match, never prefix).
	FactorModule AuthFactor = "module"

	// FactorManifest is F2: .cosca/manifest.yaml identity == cosca-kernel
	// AND module == github.com/CoscaAI/cosca.
	FactorManifest AuthFactor = "manifest"

	// FactorBuild is F3: the binary embeds a traceable version + commit hash.
	FactorBuild AuthFactor = "build"
)

// AuthStatus holds the result of the security-key verification.
type AuthStatus struct {
	Authenticated bool                `json:"authenticated"`
	Factors       map[AuthFactor]bool `json:"factors"`
	ModulePath    string              `json:"module_path,omitempty"`
	Version       string              `json:"version"`
	Detail        string              `json:"detail"`
}

// VerifyAuthenticity verifies the security key (the 3 factors) for the
// project rooted at projectDir. It returns the factor-by-factor status.
// A nil error does NOT mean authenticated — always check
// AuthStatus.Authenticated (all 3 factors must hold).
func VerifyAuthenticity(projectDir string) (*AuthStatus, error) {
	fi, err := os.Stat(projectDir)
	if err != nil {
		return nil, fmt.Errorf("verify license: cannot access %s: %w", projectDir, err)
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("verify license: %s is not a directory", projectDir)
	}

	status := &AuthStatus{
		Factors: map[AuthFactor]bool{
			FactorModule:   false,
			FactorManifest: false,
			FactorBuild:    false,
		},
		Version: pkgcosca.Version,
	}
	var failures []string

	// F1 — module: exact match on go.mod.
	modPath, modErr := moduleOf(projectDir)
	status.ModulePath = modPath
	switch {
	case modErr != nil:
		failures = append(failures, fmt.Sprintf("F1 módulo Go: %v", modErr))
	case modPath != embedcosca.SelfModule:
		failures = append(failures, fmt.Sprintf(
			"F1 módulo Go: go.mod declara %q, esperado %q (match exato, nunca prefixo)",
			modPath, embedcosca.SelfModule))
	default:
		status.Factors[FactorModule] = true
	}

	// F2 — manifest: .cosca/manifest.yaml must carry the self identity.
	manifest, mErr := embedcosca.ReadManifest(filepath.Join(projectDir, ".cosca"))
	switch {
	case mErr != nil:
		failures = append(failures, fmt.Sprintf("F2 manifesto: %v", mErr))
	case manifest["identity"] != embedcosca.SelfIdentity || manifest["module"] != embedcosca.SelfModule:
		failures = append(failures, fmt.Sprintf(
			"F2 manifesto: identity=%q module=%q, esperado identity=%q module=%q",
			manifest["identity"], manifest["module"],
			embedcosca.SelfIdentity, embedcosca.SelfModule))
	default:
		status.Factors[FactorManifest] = true
	}

	// F3 — build: the binary embeds a traceable version + commit hash.
	if pkgcosca.Version == "" || pkgcosca.CommitHash == "" {
		failures = append(failures,
			"F3 build: binário sem versão/commit rastreáveis (ldflags ausentes)")
	} else {
		status.Factors[FactorBuild] = true
	}

	status.Authenticated = status.Factors[FactorModule] &&
		status.Factors[FactorManifest] &&
		status.Factors[FactorBuild]
	switch {
	case status.Authenticated:
		status.Detail = "chave de segurança válida — os 3 fatores confirmam origem em https://github.com/CoscaAI"
	case len(failures) > 0:
		status.Detail = "chave de segurança INVÁLIDA — " + strings.Join(failures, "; ")
	default:
		status.Detail = "chave de segurança INVÁLIDA — software não autenticado"
	}

	return status, nil
}

// moduleOf returns the module path declared by the project's go.mod, or an
// error when the file is missing or malformed. Parsing is done with the
// canonical golang.org/x/mod/modfile parser (same as internal/discovery).
func moduleOf(projectDir string) (string, error) {
	goModPath := filepath.Join(projectDir, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return "", fmt.Errorf("go.mod ilegível: %v", err)
	}
	f, err := modfile.Parse(goModPath, data, nil)
	if err != nil {
		return "", fmt.Errorf("go.mod inválido: %v", err)
	}
	if f.Module == nil {
		return "", fmt.Errorf("go.mod sem diretiva module")
	}
	return f.Module.Mod.Path, nil
}
