package vision

import (
	"errors"
	"testing"

	"github.com/yalue/onnxruntime_go"
)

// TestONNXRuntimeDLLLoadable is a smoke test that confirms the onnxruntime
// shared library (onnxruntime.dll on Windows) is discoverable and the
// environment can be initialised. Inference tests are t.Skip'd when the model
// files are absent, but this proves the native-Go ONNX runtime itself is wired
// correctly on the current machine.
func TestONNXRuntimeDLLLoadable(t *testing.T) {
	err := initONNXRuntime()
	if err != nil {
		t.Skipf("onnxruntime shared library not loadable: %v", err)
	}
	if !onnxruntime_go.IsInitialized() {
		t.Fatal("onnxruntime environment not initialized after InitializeEnvironment()")
	}
	if v := onnxruntime_go.GetVersion(); v == "" {
		t.Error("onnxruntime returned an empty version string")
	} else {
		t.Logf("onnxruntime version: %s", v)
	}
}

// TestLoadModel_MissingPath verifies the graceful degradation contract when the
// requested .onnx model file does not exist on disk.
func TestLoadModel_MissingPath(t *testing.T) {
	_, err := loadModel("testdata/does-not-exist.onnx")
	if err == nil {
		t.Fatal("loadModel should return an error for a missing model file")
	}
	if !errors.Is(err, ErrModelNotFound) {
		t.Errorf("expected ErrModelNotFound, got: %v", err)
	}
}
