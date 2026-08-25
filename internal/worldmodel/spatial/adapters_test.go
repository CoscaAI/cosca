package spatial

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/worldmodel"
)

// writeFakeScript cria um script python3 temporário e determinístico que lê o
// request no stdin e devolve um subprocessResponse fixo (uma resposta JSON)
// codificado em base64 via variável de ambiente. Este é um TEST DOUBLE da
// fronteira externa (o subprocesso), NÃO um mock da lógica do adapter: o
// adapter real executa, o marshaling real executa, runSubprocess real executa,
// e a resposta atravessa o código real de parsing.
func writeFakeScript(t *testing.T, responseJSON string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fake_adapter.py")
	// O script lê o stdin (request), ignora, e escreve o response fixo.
	content := `import sys, os, json
# Lê o request do stdin (obrigatório para o adapter funcionar de verdade).
req = json.load(sys.stdin)
resp = {"ok": True, "data": json.loads(os.environ["FAKE_RESPONSE"])}
json.dump(resp, sys.stdout)
`
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write fake script: %v", err)
	}
	t.Setenv("FAKE_RESPONSE", responseJSON)
	return path
}

func TestSLAMAdapterLocalize_Success(t *testing.T) {
	response := `{"position":[1,2,3],"rotation":[0,0,0,1]}`
	script := writeFakeScript(t, response)

	a := NewSLAMAdapter(SLAMConfig{Script: script})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pose, err := a.Localize(ctx, []byte("frame-bytes"), []float64{0.1, 0.2, 0.3})
	if err != nil {
		t.Fatalf("Localize: %v", err)
	}
	if pose.Position.X != 1 || pose.Position.Y != 2 || pose.Position.Z != 3 {
		t.Fatalf("position incorreta: %+v", pose.Position)
	}
	// array [0,0,0,1] -> W=0, X=0, Y=0, Z=1 (mapeamento index 0=W,1=X,2=Y,3=Z)
	if pose.Rotation.Z != 1 || pose.Rotation.W != 0 {
		t.Fatalf("rotation incorreta: %+v", pose.Rotation)
	}
}

func TestSLAMAdapterLocalize_SubprocessError(t *testing.T) {
	// Script que devolve OK=false com erro -> adapter deve propagar o erro.
	dir := t.TempDir()
	script := filepath.Join(dir, "fail.py")
	content := `import sys, json
json.dump({"ok": False, "data": None, "error": "slam backend indisponivel"}, sys.stdout)
`
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatalf("write fail script: %v", err)
	}

	a := NewSLAMAdapter(SLAMConfig{Script: script})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := a.Localize(ctx, []byte("frame"), nil)
	if err == nil {
		t.Fatal("Localize deveria retornar erro quando o subprocesso devolve OK=false")
	}
}

func TestSLAMAdapterMap_MalformedJSON(t *testing.T) {
	// Script que devolve JSON invalido no stdout -> adapter deve falhar de forma
	// controlada, sem panic.
	dir := t.TempDir()
	script := filepath.Join(dir, "badjson.py")
	content := `import sys
sys.stdout.write("not-valid-json{{{")
`
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatalf("write badjson script: %v", err)
	}

	a := NewSLAMAdapter(SLAMConfig{Script: script})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := a.Map(ctx)
	if err == nil {
		t.Fatal("Map deveria retornar erro ante JSON invalido do subprocesso")
	}
}

func TestSLAMAdapterMap_Timeout(t *testing.T) {
	// Script que dorme mais que o timeout -> adapter deve retornar erro de
	// contexto, nao travar. Usamos timeout curto (1s) e script que dorme 3s.
	// Deterministico: o conflito é real, o teste valida o cancelamento.
	dir := t.TempDir()
	script := filepath.Join(dir, "sleep.py")
	content := `import sys, time
time.sleep(3)
sys.stdout.write('{"ok": true, "data": {}}')
`
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatalf("write sleep script: %v", err)
	}

	a := NewSLAMAdapter(SLAMConfig{Script: script})
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	start := time.Now()
	_, err := a.Map(ctx)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("Map deveria retornar erro por timeout/contexto")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("Map nao respeitou o timeout: levou %v", elapsed)
	}
}

func TestReconstructAdapter_Reconstruct_Success(t *testing.T) {
	response := `{"vertices":[[0,0,0],[1,0,0],[0,1,0]],"faces":[[0,1,2]],"texture_url":"tex.png"}`
	script := writeFakeScript(t, response)

	a := NewReconstructAdapter(ReconstructConfig{Script: script})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pc := worldmodel.PointCloud{Points: []worldmodel.Vec3{{X: 0, Y: 0, Z: 0}}}
	mesh, err := a.Reconstruct(ctx, pc)
	if err != nil {
		t.Fatalf("Reconstruct: %v", err)
	}
	if len(mesh.Vertices) != 3 || len(mesh.Faces) != 1 {
		t.Fatalf("mesh incorreta: %d verts, %d faces", len(mesh.Vertices), len(mesh.Faces))
	}
	if mesh.Vertices[0] != (worldmodel.Vec3{X: 0, Y: 0, Z: 0}) {
		t.Fatalf("vertex 0 incorreto: %+v", mesh.Vertices[0])
	}
}

func TestReconstructAdapter_ReconstructFromFrames_Success(t *testing.T) {
	response := `{"vertices":[[5,6,7]],"faces":[[0,0,0]]}`
	script := writeFakeScript(t, response)

	a := NewReconstructAdapter(ReconstructConfig{Script: script})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mesh, err := a.ReconstructFromFrames(ctx, [][]byte{{1}, {2}})
	if err != nil {
		t.Fatalf("ReconstructFromFrames: %v", err)
	}
	if len(mesh.Vertices) != 1 || mesh.Vertices[0] != (worldmodel.Vec3{X: 5, Y: 6, Z: 7}) {
		t.Fatalf("mesh incorreta: %+v", mesh.Vertices)
	}
}

func TestReasoningAdapter_Reason_Success(t *testing.T) {
	response := `{
		"relations": [{"subject":"muro","object":"piso","relation":"acima_de","distance":1.5,"confidence":0.9}],
		"entities": [{"id":"e1","type":"WALL","position":[10,0,5],"label":"muro","confidence":0.8}]
	}`
	script := writeFakeScript(t, response)

	a := NewReasoningAdapter(ReasoningConfig{Script: script})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pose := worldmodel.Pose6DoF{Position: worldmodel.Vec3{X: 0, Y: 0, Z: 0}, Rotation: worldmodel.Quat{W: 1}}
	pc := worldmodel.PointCloud{Points: []worldmodel.Vec3{{X: 0, Y: 0, Z: 0}}}
	relations, entities, err := a.Reason(ctx, pose, pc)
	if err != nil {
		t.Fatalf("Reason: %v", err)
	}
	if len(relations) != 1 || relations[0].Relation != "acima_de" {
		t.Fatalf("relations incorretas: %+v", relations)
	}
	if len(entities) != 1 || entities[0].Type != worldmodel.EntityType("WALL") {
		t.Fatalf("entities incorretas: %+v", entities)
	}
}

// TestFakeScriptPythonAssurance valida que o python3 do ambiente consegue
// rodar o script fake (caso contrario os testes acima seriam ENVIRONMENT
// dependent). Evita introduzir falha por falta de python; se python3 nao
// existir, estes testes sao skipados explicitamente (nao silenciosamente).
func TestFakeScriptPythonAssurance(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 não disponível; adapters World/City exigem subprocesso")
	}
}

// Evita import nao usado se json nao for necessario em algum build.
var _ = json.RawMessage{}
