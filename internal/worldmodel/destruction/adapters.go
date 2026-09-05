package destruction

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/CoscaAI/cosca/internal/worldmodel"
)

// ──────────────────────────────────────────────────────────────
// Subprocess helper
// ──────────────────────────────────────────────────────────────

type subprocessRequest struct {
	Command string          `json:"command"`
	Payload json.RawMessage `json:"payload"`
}

type subprocessResponse struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data,omitempty"`
	Error string          `json:"error,omitempty"`
}

func runSubprocess(ctx context.Context, scriptPath string, req subprocessRequest, timeout time.Duration) (json.RawMessage, error) {
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	reqBytes, _ := json.Marshal(req)
	cmd := exec.CommandContext(ctx, "python3", scriptPath)
	cmd.Stdin = bytes.NewReader(reqBytes)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("subprocess failed: %w, stderr: %s", err, stderr.String())
	}

	var resp subprocessResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if !resp.OK {
		return nil, fmt.Errorf("subprocess error: %s", resp.Error)
	}
	return resp.Data, nil
}

// ──────────────────────────────────────────────────────────────
// Fracture Adapter (Voronoi / Markov / Cutoff)
// ──────────────────────────────────────────────────────────────

// FractureConfig configures the fracture adapter.
type FractureConfig struct {
	Method   string `json:"method"`   // "voronoi", "markov", "cutoff"
	MaxFrags int    `json:"max_frags"`
	Seed     int64  `json:"seed"`
	Script   string `json:"script"`
}

// FractureAdapter wraps fracture simulation tools.
type FractureAdapter struct {
	config FractureConfig
}

// NewFractureAdapter creates a fracture adapter.
func NewFractureAdapter(config FractureConfig) *FractureAdapter {
	if config.Script == "" {
		config.Script = "adapters/destruction/fracture.py"
	}
	return &FractureAdapter{config: config}
}

// Fracture breaks a mesh into fragments based on an impact.
func (a *FractureAdapter) Fracture(ctx context.Context, mesh worldmodel.Mesh, material worldmodel.Material, action worldmodel.PhysicalAction) ([]worldmodel.Fragment, error) {
	vertices := make([][3]float64, len(mesh.Vertices))
	for i, v := range mesh.Vertices {
		vertices[i] = [3]float64{v.X, v.Y, v.Z}
	}

	faces := make([][3]int, len(mesh.Faces))
	copy(faces, mesh.Faces)

	payload, _ := json.Marshal(map[string]any{
		"vertices":  vertices,
		"faces":     faces,
		"material": map[string]any{
			"name":         material.Name,
			"density":      material.Density,
			"hardness":     material.Hardness,
			"fracture_type": material.FractureType,
			"toughness":    material.Toughness,
		},
		"impact_point": [3]float64{action.Impulse.X, action.Impulse.Y, action.Impulse.Z},
		"force":        [3]float64{action.Force.X, action.Force.Y, action.Force.Z},
		"method":       a.config.Method,
		"max_frags":    a.config.MaxFrags,
		"seed":         a.config.Seed,
	})

	data, err := runSubprocess(ctx, a.config.Script, subprocessRequest{
		Command: "fracture",
		Payload: payload,
	}, 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("fracture: %w", err)
	}

	var result struct {
		Fragments []struct {
			ID         string    `json:"id"`
			Position   [3]float64 `json:"position"`
			Rotation   [4]float64 `json:"rotation"`
			Velocity   [3]float64 `json:"velocity"`
			AngularVel [3]float64 `json:"angular_velocity"`
			Mass       float64   `json:"mass"`
			Volume     float64   `json:"volume"`
			Radius     float64   `json:"bounding_radius"`
			Vertices   [][3]float64 `json:"vertices"`
			Faces      [][3]int     `json:"faces"`
		} `json:"fragments"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	fragments := make([]worldmodel.Fragment, len(result.Fragments))
	for i, f := range result.Fragments {
		frag := worldmodel.Fragment{
			ID:         f.ID,
			Position:   worldmodel.Vec3{X: f.Position[0], Y: f.Position[1], Z: f.Position[2]},
			Rotation:   worldmodel.Quat{W: f.Rotation[0], X: f.Rotation[1], Y: f.Rotation[2], Z: f.Rotation[3]},
			Velocity:   worldmodel.Vec3{X: f.Velocity[0], Y: f.Velocity[1], Z: f.Velocity[2]},
			AngularVel: worldmodel.Vec3{X: f.AngularVel[0], Y: f.AngularVel[1], Z: f.AngularVel[2]},
			Mass:       f.Mass,
			Volume:     f.Volume,
			BoundingSphereRadius: f.Radius,
			Timestamp:  time.Now(),
		}

		if len(f.Vertices) > 0 {
			mesh := worldmodel.Mesh{
				Vertices: make([]worldmodel.Vec3, len(f.Vertices)),
				Faces:    make([][3]int, len(f.Faces)),
			}
			for j, v := range f.Vertices {
				mesh.Vertices[j] = worldmodel.Vec3{X: v[0], Y: v[1], Z: v[2]}
			}
			copy(mesh.Faces, f.Faces)
			frag.Mesh = &mesh
		}

		fragments[i] = frag
	}

	return fragments, nil
}
