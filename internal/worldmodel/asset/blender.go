// Package asset implements the Asset/World Pipeline for the Living World.
//
// The asset pipeline connects Cosca to external 3D tools (Blender, etc.)
// for asset creation, validation, and delivery to Unreal Engine.
//
// Flow:
//
//	AssetRequest (type, params, seed, format)
//	  → BlenderAdapter (generate, validate, export)
//	  → AssetResult (path, hash, metadata, provenance)
//	  → Unreal Bridge (import, spawn, update WorldState)
package asset

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// ──────────────────────────────────────────────────────────────
// Asset types
// ──────────────────────────────────────────────────────────────

// AssetType classifies asset types.
type AssetType string

const (
	AssetTree       AssetType = "tree"
	AssetTerrain    AssetType = "terrain"
	AssetBuilding   AssetType = "building"
	AssetDebris     AssetType = "debris"
	AssetVehicle    AssetType = "vehicle"
	AssetProp       AssetType = "prop"
	AssetCharacter  AssetType = "character"
	AssetCustom     AssetType = "custom"
)

// ExportFormat is the output format.
type ExportFormat string

const (
	FormatGLB ExportFormat = "glb"
	FormatFBX ExportFormat = "fbx"
	FormatUSD ExportFormat = "usd"
)

// ──────────────────────────────────────────────────────────────
// Request/Response
// ──────────────────────────────────────────────────────────────

// AssetRequest is a request to generate an asset.
type AssetRequest struct {
	Type     AssetType       `json:"type"`
	Params   map[string]any  `json:"params,omitempty"`
	Seed     int64           `json:"seed,omitempty"`
	Format   ExportFormat    `json:"format,omitempty"`
	Output   string          `json:"output,omitempty"`   // output directory
	Validate bool            `json:"validate,omitempty"` // validate after generation
}

// AssetResult is the result of asset generation.
type AssetResult struct {
	ID         string            `json:"id"`
	Type       AssetType         `json:"type"`
	Path       string            `json:"path"`       // output file path
	Format     ExportFormat      `json:"format"`
	Hash       string            `json:"hash"`       // sha256 hash
	Valid      bool              `json:"valid"`
	Issues     []string          `json:"issues,omitempty"`
	Metadata   AssetMetadata     `json:"metadata"`
	Provenance AssetProvenance   `json:"provenance"`
	Latency    time.Duration     `json:"latency"`
}

// AssetMetadata holds asset statistics.
type AssetMetadata struct {
	Vertices     int        `json:"vertices"`
	Faces        int        `json:"faces"`
	Materials    []string   `json:"materials,omitempty"`
	Textures     []string   `json:"textures,omitempty"`
	BoundingBox  *AABB      `json:"bounding_box,omitempty"`
	LODLevels    int        `json:"lod_levels"`
	HasCollision bool       `json:"has_collision"`
}

// AABB is an axis-aligned bounding box.
type AABB struct {
	Min [3]float64 `json:"min"`
	Max [3]float64 `json:"max"`
}

// AssetProvenance tracks the origin of an asset.
type AssetProvenance struct {
	Tool      string    `json:"tool"`      // "blender"
	Version   string    `json:"version"`   // "4.2.0"
	Script    string    `json:"script"`    // "generate_tree.py"
	Seed      int64     `json:"seed"`
	Timestamp time.Time `json:"timestamp"`
}

// ──────────────────────────────────────────────────────────────
// Validation
// ──────────────────────────────────────────────────────────────

// ValidationResult holds validation results.
type ValidationResult struct {
	Valid  bool     `json:"valid"`
	Issues []string `json:"issues,omitempty"`
	Stats  AssetMetadata `json:"stats"`
}

// ──────────────────────────────────────────────────────────────
// BlenderProvider interface
// ──────────────────────────────────────────────────────────────

// BlenderProvider is the interface for Blender-based asset operations.
type BlenderProvider interface {
	// GenerateAsset creates an asset from parameters.
	GenerateAsset(ctx context.Context, req AssetRequest) (*AssetResult, error)

	// ValidateAsset checks if an asset file is valid.
	ValidateAsset(ctx context.Context, filepath string) (*ValidationResult, error)

	// ExportAsset exports the current scene.
	ExportAsset(ctx context.Context, scene string, format ExportFormat) (string, error)

	// GetVersion returns the Blender version.
	GetVersion(ctx context.Context) (string, error)
}

// ──────────────────────────────────────────────────────────────
// BlenderAdapter (subprocess implementation)
// ──────────────────────────────────────────────────────────────

// BlenderAdapterConfig configures the Blender adapter.
type BlenderAdapterConfig struct {
	BlenderPath string `json:"blender_path"` // path to blender executable
	WorkDir     string `json:"work_dir"`     // working directory for scripts
	Timeout     int    `json:"timeout"`      // seconds (0 = no timeout)
	ScriptsDir  string `json:"scripts_dir"`  // path to Python scripts
}

// DefaultBlenderAdapterConfig returns sensible defaults.
func DefaultBlenderAdapterConfig() BlenderAdapterConfig {
	return BlenderAdapterConfig{
		BlenderPath: detectBlenderPath(),
		WorkDir:     os.TempDir(),
		Timeout:     60,
		ScriptsDir:  "scripts/blender",
	}
}

// detectBlenderPath finds the Blender executable on the system.
// Falls back to "blender" (relying on PATH) if not found in standard locations.
func detectBlenderPath() string {
	// Check PATH first
	if _, err := exec.LookPath("blender"); err == nil {
		return "blender"
	}

	// Windows standard install locations
	bases := []string{
		os.Getenv("ProgramFiles"),
		os.Getenv("ProgramFiles(x86)"),
	}
	for _, base := range bases {
		if base == "" {
			continue
		}
		found := findBlenderInDir(base)
		if found != "" {
			return found
		}
	}

	return "blender"
}

// findBlenderInDir recursively searches a base directory for blender.exe.
func findBlenderInDir(base string) string {
	root := filepath.Join(base, "Blender Foundation")
	entries, err := os.ReadDir(root)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		exe := filepath.Join(root, entry.Name(), "blender.exe")
		if _, err := os.Stat(exe); err == nil {
			return exe
		}
	}
	return ""
}

// BlenderAdapter wraps Blender for asset operations.
type BlenderAdapter struct {
	config BlenderAdapterConfig
}

// NewBlenderAdapter creates a Blender adapter.
func NewBlenderAdapter(config BlenderAdapterConfig) *BlenderAdapter {
	return &BlenderAdapter{config: config}
}

// GenerateAsset creates an asset using Blender.
func (a *BlenderAdapter) GenerateAsset(ctx context.Context, req AssetRequest) (*AssetResult, error) {
	start := time.Now()

	if req.Format == "" {
		req.Format = FormatGLB
	}
	if req.Params == nil {
		req.Params = make(map[string]any)
	}
	req.Params["seed"] = req.Seed
	req.Params["type"] = string(req.Type)

	// Determine output path
	outputPath := req.Output
	if outputPath == "" {
		outputPath = filepath.Join(a.config.WorkDir, fmt.Sprintf("asset_%d.%s", time.Now().UnixNano(), req.Format))
	}

	// Build script args
	scriptPath := a.scriptPath("generate.py")
	args := []string{
		"--background",
		"--factory-startup",
		"--python", scriptPath,
		"--",
		"--type", string(req.Type),
		"--output", outputPath,
		"--format", string(req.Format),
	}

	// Add params as args
	for k, v := range req.Params {
		args = append(args, "--"+k, fmt.Sprintf("%v", v))
	}

	// Run Blender
	timeout := time.Duration(a.config.Timeout) * time.Second
	if timeout == 0 {
		timeout = 120 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, a.config.BlenderPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("blender generate: %w, stderr: %s", err, stderr.String())
	}

	// Compute hash
	hash, err := computeFileHash(outputPath)
	if err != nil {
		return nil, fmt.Errorf("compute hash: %w", err)
	}

	// Validate if requested
	var valid bool
	var issues []string
	if req.Validate {
		validation, err := a.ValidateAsset(ctx, outputPath)
		if err != nil {
			return nil, fmt.Errorf("validate generated asset: %w", err)
		}
		valid = validation.Valid
		issues = validation.Issues
	} else {
		valid = true
	}

	return &AssetResult{
		ID:     fmt.Sprintf("asset_%d", time.Now().UnixNano()),
		Type:   req.Type,
		Path:   outputPath,
		Format: req.Format,
		Hash:   hash,
		Valid:  valid,
		Issues: issues,
		Provenance: AssetProvenance{
			Tool:      "blender",
			Version:   "4.2.0",
			Script:    "generate.py",
			Seed:      req.Seed,
			Timestamp: time.Now(),
		},
		Latency: time.Since(start),
	}, nil
}

// ValidateAsset checks if an asset file is valid.
func (a *BlenderAdapter) ValidateAsset(ctx context.Context, assetPath string) (*ValidationResult, error) {
	scriptPath := a.scriptPath("validate.py")

	timeout := 30 * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, a.config.BlenderPath,
		"--background", "--factory-startup",
		"--python", scriptPath,
		"--", "--input", assetPath,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("blender validate: %w, stderr: %s", err, stderr.String())
	}

	// Parse validation output
	// Blender prints log lines (INFO:, version, "Blender quit") around the JSON
	// payload, so extract the balanced JSON object first.
	data := extractJSON(stdout.Bytes())
	if data == nil {
		return nil, fmt.Errorf("no JSON found in validation output: %s", stdout.String())
	}
	var result ValidationResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse validation: %w", err)
	}

	return &result, nil
}

// ExportAsset exports the current Blender scene.
func (a *BlenderAdapter) ExportAsset(ctx context.Context, scene string, format ExportFormat) (string, error) {
	outputPath := filepath.Join(a.config.WorkDir, fmt.Sprintf("export_%d.%s", time.Now().UnixNano(), format))

	scriptPath := a.scriptPath("export.py")

	timeout := 60 * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, a.config.BlenderPath,
		"--background", "--factory-startup",
		"--python", scriptPath,
		"--", "--input", scene, "--output", outputPath, "--format", string(format),
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("blender export: %w, stderr: %s", err, stderr.String())
	}

	return outputPath, nil
}

// GetVersion returns the Blender version.
func (a *BlenderAdapter) GetVersion(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, a.config.BlenderPath, "--version")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("blender version: %w", err)
	}

	// Parse "Blender 4.2.0"
	output := stdout.String()
	if len(output) > 0 {
		return output, nil
	}
	return "unknown", nil
}

// ──────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────

// scriptPath resolves the absolute path to a Blender Python script.
// It tries the configured ScriptsDir, then falls back to searching
// upward from the process working directory until it finds the
// project root (containing go.mod) or the scripts/ directory.
func (a *BlenderAdapter) scriptPath(name string) string {
	// Try configured ScriptsDir directly
	if a.config.ScriptsDir != "" {
		p := filepath.Join(a.config.ScriptsDir, name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
		if abs, err := filepath.Abs(p); err == nil {
			if _, err := os.Stat(abs); err == nil {
				return abs
			}
		}
	}

	// Search upward for project root (go.mod) or scripts/ dir
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		candidates := []string{
			filepath.Join(dir, "scripts", "blender", name),
			filepath.Join(dir, "go.mod"),
		}
		for _, cand := range candidates {
			if _, err := os.Stat(cand); err == nil {
				if filepath.Base(cand) == "go.mod" {
					return filepath.Join(dir, "scripts", "blender", name)
				}
				return cand
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// extractJSON extracts the first valid JSON object from raw output.
// Blender prints log lines (INFO:, version, "Blender quit") around the
// JSON payload, so we must find the balanced JSON object.
func extractJSON(raw []byte) []byte {
	start := bytes.IndexByte(raw, '{')
	if start < 0 {
		return nil
	}
	depth := 0
	for i := start; i < len(raw); i++ {
		switch raw[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return raw[start : i+1]
			}
		}
	}
	return nil
}

func computeFileHash(filepath string) (string, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", hash), nil
}
