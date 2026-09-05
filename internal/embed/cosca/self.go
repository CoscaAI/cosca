// self.go implements the SELF IDENTITY: detection of the Cosca project
// itself. This package is READ-ONLY at runtime — the compiled binary
// MUST NEVER write to internal/embed/cosca/ (P8).
package cosca

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Self identity constants.
const (
	SelfProjectName  = "Cosca"
	SelfIdentity     = "cosca-kernel"
	SelfModule       = "github.com/CoscaAI/cosca"
	SelfVersion      = "1.0.0"
	ManifestFileName = "manifest.yaml"
)

// IsSelfProject reports whether projectDir is the Cosca project itself.
func IsSelfProject(projectDir string) bool {
	goModPath := filepath.Join(projectDir, "go.mod")
	goModData, err := os.ReadFile(goModPath)
	if err == nil {
		return moduleOfGoMod(goModData) == SelfModule
	}
	if !os.IsNotExist(err) {
		return false
	}
	manifest, mErr := ReadManifest(filepath.Join(projectDir, ".cosca"))
	if mErr != nil {
		return false
	}
	return manifest["identity"] == SelfIdentity && manifest["module"] == SelfModule
}

func moduleOfGoMod(data []byte) string {
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if !strings.HasPrefix(line, "module ") {
			continue
		}
		mod := strings.TrimSpace(strings.TrimPrefix(line, "module "))
		if idx := strings.Index(mod, "//"); idx >= 0 {
			mod = strings.TrimSpace(mod[:idx])
		}
		return mod
	}
	return ""
}

// WriteManifest writes .cosca/manifest.yaml inside coscaDir.
func WriteManifest(coscaDir string) error {
	manifest := map[string]string{
		"project_name": SelfProjectName,
		"identity":     SelfIdentity,
		"module":       SelfModule,
		"version":      SelfVersion,
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
	}
	data, err := yaml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	if err := os.MkdirAll(coscaDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", coscaDir, err)
	}
	path := filepath.Join(coscaDir, ManifestFileName)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// ReadManifest reads .cosca/manifest.yaml from coscaDir.
func ReadManifest(coscaDir string) (map[string]string, error) {
	path := filepath.Join(coscaDir, ManifestFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var manifest map[string]string
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return manifest, nil
}
