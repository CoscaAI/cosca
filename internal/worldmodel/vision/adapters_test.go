package vision

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeFakeScript cria um script python3 temporário e determinístico que lê o
// request do stdin e devolve um subprocessResponse {ok:true, data:<FAKE_RESPONSE>} .
// É um TEST DOUBLE da fronteira externa (subprocesso), NÃO um mock da lógica do
// adapter: o adapter real executa, o marshaling real executa, runSubprocess real
// executa, e a resposta atravessa o parsing real.
func writeFakeScript(t *testing.T, dataJSON string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fake_vision.py")
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

func TestClipAdapterClassify_Success(t *testing.T) {
	requirePython(t)
	script := writeFakeScript(t, `{"label":"esteira","confidence":0.87}`)
	a := NewClipAdapter(ClipConfig{Script: script})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	label, conf, err := a.Classify(ctx, []byte("frame"), []string{"esteira", "halter"})
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if label != "esteira" || conf != 0.87 {
		t.Fatalf("classificacao incorreta: label=%q conf=%v", label, conf)
	}
}

func TestClipAdapterEmbed_Success(t *testing.T) {
	requirePython(t)
	script := writeFakeScript(t, `{"embedding":[0.1,0.2,0.3]}`)
	a := NewClipAdapter(ClipConfig{Script: script})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	emb, err := a.Embed(ctx, []byte("frame"))
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(emb) != 3 || emb[0] != 0.1 || emb[2] != 0.3 {
		t.Fatalf("embedding incorreto: %v", emb)
	}
}

func TestSAMAdapterSegment_Success(t *testing.T) {
	requirePython(t)
	script := writeFakeScript(t, `{"masks":[{"bounding_box":{"min":{"x":10,"y":10,"z":0},"max":{"x":50,"y":50,"z":0}},"area":1600,"label":"esteira"}]}`)
	a := NewSAMAdapter(SAMConfig{Script: script})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	masks, err := a.Segment(ctx, []byte("frame"), "esteira")
	if err != nil {
		t.Fatalf("Segment: %v", err)
	}
	if len(masks) != 1 || masks[0].Label != "esteira" || masks[0].Area != 1600 {
		t.Fatalf("masks incorretas: %+v", masks)
	}
}

func TestGroundingAdapterDetect_Success(t *testing.T) {
	requirePython(t)
	script := writeFakeScript(t, `{"detections":[{"bounding_box":{"min":{"x":0,"y":0,"z":0},"max":{"x":10,"y":10,"z":0}},"label":"halter","confidence":0.92}]}`)
	a := NewGroundingAdapter(GroundingConfig{Script: script})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dets, err := a.Detect(ctx, []byte("frame"))
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(dets) != 1 || dets[0].Label != "halter" || dets[0].Confidence != 0.92 {
		t.Fatalf("detections incorretas: %+v", dets)
	}
}

func TestDepthAdapterEstimateDepth_Success(t *testing.T) {
	requirePython(t)
	script := writeFakeScript(t, `{"depth_map":[[1.0,2.0],[3.0,4.0]]}`)
	a := NewDepthAdapter(DepthConfig{Script: script})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	depth, err := a.EstimateDepth(ctx, []byte("frame"))
	if err != nil {
		t.Fatalf("EstimateDepth: %v", err)
	}
	if len(depth) != 2 || len(depth[0]) != 2 || depth[1][1] != 4.0 {
		t.Fatalf("depth map incorreto: %v", depth)
	}
}

func TestClipAdapterClassify_SubprocessError(t *testing.T) {
	requirePython(t)
	dir := t.TempDir()
	script := filepath.Join(dir, "fail.py")
	content := `import sys, json
json.dump({"ok": False, "data": None, "error": "modelo nao carregado"}, sys.stdout)
`
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatalf("write fail script: %v", err)
	}
	a := NewClipAdapter(ClipConfig{Script: script})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, _, err := a.Classify(ctx, []byte("frame"), nil)
	if err == nil {
		t.Fatal("Classify deveria retornar erro quando subprocesso devolve OK=false")
	}
}

func TestDepthAdapterEstimateDepth_MalformedJSON(t *testing.T) {
	requirePython(t)
	dir := t.TempDir()
	script := filepath.Join(dir, "badjson.py")
	content := `import sys
sys.stdout.write("not-json{{{{")
`
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatalf("write badjson script: %v", err)
	}
	a := NewDepthAdapter(DepthConfig{Script: script})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := a.EstimateDepth(ctx, []byte("frame"))
	if err == nil {
		t.Fatal("EstimateDepth deveria retornar erro ante JSON invalido")
	}
}
