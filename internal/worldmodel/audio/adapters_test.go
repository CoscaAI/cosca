package audio

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeFakeScript cria um script python3 temporário e determinístico que
// devolve um subprocessResponse {ok:true, data:<FAKE_DATA>}. É um test double
// da fronteira externa (subprocesso) — o adapter real executa, o parsing real
// executa, runSubprocess real executa.
func writeFakeScript(t *testing.T, dataJSON string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fake_audio.py")
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

func TestWhisperAdapterTranscribe_Success(t *testing.T) {
	requirePython(t)
	script := writeFakeScript(t, `{"text":"máquina com ruído no motor"}`)
	a := NewWhisperAdapter(WhisperConfig{Script: script})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	text, err := a.Transcribe(ctx, []byte{1, 2, 3}, 44100, 2)
	if err != nil {
		t.Fatalf("Transcribe: %v", err)
	}
	if text != "máquina com ruído no motor" {
		t.Fatalf("transcrição incorreta: %q", text)
	}
}

func TestDiffFieldsAdapterAnalyze_Success(t *testing.T) {
	requirePython(t)
	script := writeFakeScript(t, `{"direction":[1,0,0],"distance":2.5,"intensity":0.8,"spatializer":"binaural"}`)
	a := NewDiffFieldsAdapter(DiffFieldsConfig{Script: script})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sa, err := a.Analyze(ctx, []byte{1}, 44100, 2)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if sa.Distance != 2.5 || sa.Intensity != 0.8 || sa.Spatializer != "binaural" {
		t.Fatalf("spatial audio incorreto: %+v", sa)
	}
	if sa.Direction.X != 1 || sa.Direction.Z != 0 {
		t.Fatalf("direction incorreta: %+v", sa.Direction)
	}
}

func TestCoquiAdapterSynthesize_Success(t *testing.T) {
	requirePython(t)
	script := writeFakeScript(t, `{"audio":"//7d","sample_rate":22050}`) // "//7d" = base64 válida
	a := NewCoquiAdapter(CoquiTTSConfig{Script: script})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	audio, sr, err := a.Synthesize(ctx, "olá", "voz-1")
	if err != nil {
		t.Fatalf("Synthesize: %v", err)
	}
	if sr != 22050 || len(audio) == 0 {
		t.Fatalf("synthesize incorreto: sr=%d len=%d", sr, len(audio))
	}
}

func TestAudioCraftAdapterGenerate_Success(t *testing.T) {
	requirePython(t)
	script := writeFakeScript(t, `{"audio":"QkVR","sample_rate":48000}`)
	a := NewAudioCraftAdapter(AudioCraftConfig{Script: script})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	audio, sr, err := a.Generate(ctx, "som de halter caindo", 1.5)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if sr != 48000 || len(audio) == 0 {
		t.Fatalf("generate incorreto: sr=%d len=%d", sr, len(audio))
	}
}

func TestWhisperAdapterTranscribe_SubprocessError(t *testing.T) {
	requirePython(t)
	dir := t.TempDir()
	script := filepath.Join(dir, "fail.py")
	content := `import sys, json
json.dump({"ok": False, "data": None, "error": "whisper model not found"}, sys.stdout)
`
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatalf("write fail script: %v", err)
	}
	a := NewWhisperAdapter(WhisperConfig{Script: script})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := a.Transcribe(ctx, []byte{1}, 44100, 2)
	if err == nil {
		t.Fatal("Transcribe deveria retornar erro quando subprocesso devolve OK=false")
	}
}

func TestCoquiAdapterSynthesize_MalformedJSON(t *testing.T) {
	requirePython(t)
	dir := t.TempDir()
	script := filepath.Join(dir, "badjson.py")
	content := `import sys
sys.stdout.write("not-json{{{{")
`
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatalf("write badjson script: %v", err)
	}
	a := NewCoquiAdapter(CoquiTTSConfig{Script: script})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, _, err := a.Synthesize(ctx, "ola", "v1")
	if err == nil {
		t.Fatal("Synthesize deveria retornar erro ante JSON invalido")
	}
}
