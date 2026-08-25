package vfx

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/worldmodel"
)

// writeFakeScript cria um script python3 temporário e determinístico que
// devolve subprocessResponse {ok:true, data:<FAKE_DATA>}. Test double da
// fronteira externa (subprocesso) — o adapter real executa, o parsing real
// executa, runSubprocess real executa.
func writeFakeScript(t *testing.T, dataJSON string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fake_vfx.py")
	content := `import sys, json
req = json.load(sys.stdin)
resp = {"ok": True, "data": json.loads("""FAKE_DATA""")}
json.dump(resp, sys.stdout)
`
	content = strings.Replace(content, "FAKE_DATA", dataJSON, 1)
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write fake script: %v", err)
	}
	return path
}

func requirePython(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 não disponível; adapters World/City exigem subprocesso")
	}
}

func TestTaichiAdapterSimulate_Success(t *testing.T) {
	requirePython(t)
	// Data é []byte -> base64. "//7d" é base64 válida ("\xff\xfe\xdd").
	response := `{"frames":[{"step":1,"width":64,"height":64,"data":"//7d","metadata":{"luminance":0.5}}]}`
	script := writeFakeScript(t, response)
	a := NewTaichiAdapter(TaichiConfig{Script: script})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg := worldmodel.VFXConfig{Resolution: 64, Seed: 42, Duration: 1, FrameRate: 30}
	frames, err := a.Simulate(ctx, cfg)
	if err != nil {
		t.Fatalf("Simulate: %v", err)
	}
	if len(frames) != 1 || frames[0].Step != 1 || frames[0].Width != 64 {
		t.Fatalf("frames incorretas: %+v", frames)
	}
	if len(frames[0].Data) == 0 || frames[0].Metadata["luminance"] != 0.5 {
		t.Fatalf("frame data/metadata incorreto: %+v", frames[0])
	}
}

func TestTaichiAdapterStep_Success(t *testing.T) {
	requirePython(t)
	response := `{"step":5,"width":32,"height":32,"data":"AB==","metadata":{"particles":100}}`
	script := writeFakeScript(t, response)
	a := NewTaichiAdapter(TaichiConfig{Script: script})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	frame, err := a.Step(ctx)
	if err != nil {
		t.Fatalf("Step: %v", err)
	}
	if frame.Step != 5 || frame.Width != 32 || frame.Metadata["particles"] != 100 {
		t.Fatalf("frame incorreto: %+v", frame)
	}
}

func TestTaichiAdapterReset_Success(t *testing.T) {
	requirePython(t)
	// reset espera resposta ok:true com data vazio.
	response := `{}`
	script := writeFakeScript(t, response)
	a := NewTaichiAdapter(TaichiConfig{Script: script})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := a.Reset(ctx); err != nil {
		t.Fatalf("Reset: %v", err)
	}
}

func TestTaichiAdapterStep_SubprocessError(t *testing.T) {
	requirePython(t)
	dir := t.TempDir()
	script := filepath.Join(dir, "fail.py")
	content := `import sys, json
json.dump({"ok": False, "data": None, "error": "vfx backend off"}, sys.stdout)
`
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatalf("write fail script: %v", err)
	}
	a := NewTaichiAdapter(TaichiConfig{Script: script})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := a.Step(ctx)
	if err == nil {
		t.Fatal("Step deveria retornar erro quando subprocesso devolve OK=false")
	}
}

func TestTaichiAdapterSimulate_MalformedJSON(t *testing.T) {
	requirePython(t)
	dir := t.TempDir()
	script := filepath.Join(dir, "badjson.py")
	content := `import sys
sys.stdout.write("not-json{{{{")
`
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatalf("write badjson script: %v", err)
	}
	a := NewTaichiAdapter(TaichiConfig{Script: script})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := a.Simulate(ctx, worldmodel.VFXConfig{})
	if err == nil {
		t.Fatal("Simulate deveria retornar erro ante JSON invalido")
	}
}
