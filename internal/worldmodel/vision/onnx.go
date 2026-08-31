package vision

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/yalue/onnxruntime_go"
)

// ──────────────────────────────────────────────────────────────
// ONNX Runtime — native Go vision engine (no Python / no PyTorch)
// ──────────────────────────────────────────────────────────────

// Sentinel errors returned by the ONNX-backed adapters. They are NOT fatal to
// the pipeline: a missing model simply degrades perception to "best effort"
// and the agent keeps running (graceful degradation).
var (
	ErrModelUnavailable = errors.New("vision model unavailable")
	ErrModelNotFound    = errors.New("vision model file not found")
)

// Canonical .onnx filenames expected inside the vision models directory. These
// are the conventions surfaced by `cosca model status` and documented in
// MODELS.md (the exporters must emit models with these names).
const (
	ClipModelFile      = "clip_vitb32.onnx"
	SAMModelFile       = "sam2_hiera_large.onnx"
	GroundingModelFile = "groundingdino_swint.onnx"
	DepthModelFile     = "depth_anything_v2_vitl.onnx"
)

// filenameForModel maps a human-facing model name (e.g. "ViT-B/32") to the
// canonical .onnx filename. Unknown names fall back to a sanitised "<name>.onnx".
func filenameForModel(model string) string {
	switch model {
	case "ViT-B/32", "clip", "clip_vitb32", "clip-vit-b32":
		return ClipModelFile
	case "sam2_hiera_large", "sam", "sam2", "sam2-hiera-large":
		return SAMModelFile
	case "groundingdino_swint", "groundingdino", "grounding", "groundingdino-swint":
		return GroundingModelFile
	case "depth_anything_v2_vitl", "depth_anything", "depth", "depth-anything-v2-vitl":
		return DepthModelFile
	default:
		return model + ".onnx"
	}
}

// DefaultModelsDir returns the directory where the ONNX vision models are
// expected to be stored. Resolution order (sovereignty-first over a fixed
// path):
//
//  1. $COSCA_MODELS            (explicit environment override)
//  2. ~/.cosca/models/vision/  (user home, cross-platform)
//  3. ./.cosca/models/vision   (last resort, CWD-relative)
func DefaultModelsDir() string {
	if v := os.Getenv("COSCA_MODELS"); v != "" {
		return v
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".cosca", "models", "vision")
	}
	return filepath.Join(".cosca", "models", "vision")
}

// ModelsDirFor returns the models directory for a pipeline configuration,
// honouring an explicit override (sovereignty) before falling back to the
// default resolution.
func ModelsDirFor(dir string) string {
	if dir != "" {
		return dir
	}
	return DefaultModelsDir()
}

// ──────────────────────────────────────────────────────────────
// Environment lifecycle (single, process-wide, lazy)
// ──────────────────────────────────────────────────────────────

var (
	onnxInitOnce sync.Once
	onnxInitErr  error
)

// initONNXRuntime initialises the process-wide onnxruntime environment exactly
// once. It is safe to call repeatedly. If the shared library (onnxruntime.dll
// on Windows / onnxruntime.so elsewhere) cannot be loaded, the error is cached
// and returned so callers can degrade gracefully instead of crashing.
//
// On Windows the library is resolved through the default DLL search path
// (C:\Windows\System32 is on it), so a stock onnxruntime.dll drops straight in.
func initONNXRuntime() error {
	onnxInitOnce.Do(func() {
		if onnxruntime_go.IsInitialized() {
			return
		}
		onnxInitErr = onnxruntime_go.InitializeEnvironment()
	})
	return onnxInitErr
}

// ──────────────────────────────────────────────────────────────
// Model session (load + introspect + run)
// ──────────────────────────────────────────────────────────────

// modelSession wraps an onnxruntime session plus its introspected input/output
// metadata (names, shapes, dtypes). Adapters use it to build inputs in the
// exact order the exported model expects and to read results by name, so a
// differently-exported model (different names/shapes) is still tolerated.
type modelSession struct {
	sess    *onnxruntime_go.DynamicAdvancedSession
	inputs  []onnxruntime_go.InputOutputInfo
	outputs []onnxruntime_go.InputOutputInfo
	path    string
}

// inputNames returns the ordered list of input tensor names.
func (m *modelSession) inputNames() []string { return infoNames(m.inputs) }

// outputNames returns the ordered list of output tensor names.
func (m *modelSession) outputNames() []string { return infoNames(m.outputs) }

// run executes the session with the given inputs, in the order of the model's
// input names. The returned slice of Values maps 1:1 to outputNames and must be
// Destroy()'d by the caller.
func (m *modelSession) run(inputs []onnxruntime_go.Value) ([]onnxruntime_go.Value, error) {
	if m.sess == nil {
		return nil, ErrModelUnavailable
	}
	if len(inputs) != len(m.inputs) {
		return nil, fmt.Errorf("model %s expects %d inputs, got %d",
			m.path, len(m.inputs), len(inputs))
	}
	outputs := make([]onnxruntime_go.Value, len(m.outputs))
	if err := m.sess.Run(inputs, outputs); err != nil {
		return nil, err
	}
	return outputs, nil
}

// destroy releases the session resources.
func (m *modelSession) destroy() {
	if m.sess != nil {
		_ = m.sess.Destroy()
		m.sess = nil
	}
}

func infoNames(infos []onnxruntime_go.InputOutputInfo) []string {
	names := make([]string, len(infos))
	for i := range infos {
		names[i] = infos[i].Name
	}
	return names
}

// loadModel loads an ONNX model from path. It returns ErrModelNotFound if the
// file is absent, ErrModelUnavailable if the onnxruntime environment could not
// be initialised, or the introspected session otherwise.
func loadModel(path string) (*modelSession, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrModelNotFound, path)
		}
		return nil, fmt.Errorf("stat model %s: %w", path, err)
	}
	if err := initONNXRuntime(); err != nil {
		return nil, fmt.Errorf("%w: onnxruntime init: %v", ErrModelUnavailable, err)
	}
	in, out, err := onnxruntime_go.GetInputOutputInfo(path)
	if err != nil {
		return nil, fmt.Errorf("introspect model %s: %w", path, err)
	}
	sess, err := onnxruntime_go.NewDynamicAdvancedSession(path,
		infoNames(in), infoNames(out), nil)
	if err != nil {
		return nil, fmt.Errorf("create session %s: %w", path, err)
	}
	return &modelSession{sess: sess, inputs: in, outputs: out, path: path}, nil
}

// imageInputDims returns the (height, width) a single-image model expects, from
// its first input tensor's shape. If the dimension is dynamic (<=0) it falls
// back to hw.
func imageInputDims(m *modelSession, hw int) (int, int) {
	if len(m.inputs) == 0 || len(m.inputs[0].Dimensions) < 4 {
		return hw, hw
	}
	d := m.inputs[0].Dimensions
	h, w := d[len(d)-2], d[len(d)-1]
	if h <= 0 || w <= 0 {
		return hw, hw
	}
	return int(h), int(w)
}
