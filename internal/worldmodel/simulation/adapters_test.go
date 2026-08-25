package simulation

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

// writeFakeScript cria um script python3 temporário e determinístico que lê o
// request do stdin e devolve um subprocessResponse {ok:true, data:<FAKE_DATA>}.
// É um test double da fronteira externa (subprocesso) — o adapter real executa,
// o marshaling real executa, runSubprocess real executa, e a resposta atravessa
// o parsing real. Não é mock da lógica do adapter.
func writeFakeScript(t *testing.T, dataJSON string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fake_sim.py")
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

func TestTaichiSimAdapterStep_Success(t *testing.T) {
	requirePython(t)
	response := `{
		"step": 42,
		"entities": [
			{"id":"e1","type":"HALTER","position":[1,2,3],"rotation":[0,0,0,1],"label":"halter"}
		]
	}`
	script := writeFakeScript(t, response)
	a := NewTaichiSimAdapter(TaichiConfig{Script: script})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	state := worldmodel.WorldState{Step: 41, Climate: worldmodel.ClimateState{Temperature: 25}}
	step, err := a.Step(ctx, state, nil)
	if err != nil {
		t.Fatalf("Step: %v", err)
	}
	if step.Step != 42 {
		t.Fatalf("step incorreto: %d", step.Step)
	}
	if len(step.State.Entities) != 1 {
		t.Fatalf("entities incorretas: %d", len(step.State.Entities))
	}
	if step.State.Entities[0].ID != "e1" || step.State.Entities[0].Label != "halter" {
		t.Fatalf("entity incorreta: %+v", step.State.Entities[0])
	}
	// a entidade deve preservar o climate do estado de entrada.
	if step.State.Climate.Temperature != 25 {
		t.Fatalf("climate não preservado: %+v", step.State.Climate)
	}
}

func TestTaichiSimAdapterQuery_Success(t *testing.T) {
	requirePython(t)
	response := `{"hit":true,"point":[5,0,0],"normal":[0,1,0],"distance":5.0,"entity_id":"e1"}`
	script := writeFakeScript(t, response)
	a := NewTaichiSimAdapter(TaichiConfig{Script: script})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	q := worldmodel.PhysicsQuery{Type: "ray", MaxDist: 10}
	res, err := a.Query(ctx, q)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if !res.Hit || res.Distance != 5.0 || res.EntityID != "e1" {
		t.Fatalf("query incorreta: %+v", res)
	}
	if res.Point.X != 5 || res.Normal.Y != 1 {
		t.Fatalf("point/normal incorreto: %+v", res)
	}
}

func TestTaichiSimAdapterStep_SubprocessError(t *testing.T) {
	requirePython(t)
	dir := t.TempDir()
	script := filepath.Join(dir, "fail.py")
	content := `import sys, json
json.dump({"ok": False, "data": None, "error": "taichi backend off"}, sys.stdout)
`
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatalf("write fail script: %v", err)
	}
	a := NewTaichiSimAdapter(TaichiConfig{Script: script})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := a.Step(ctx, worldmodel.WorldState{}, nil)
	if err == nil {
		t.Fatal("Step deveria retornar erro quando subprocesso devolve OK=false")
	}
}

func TestMuJoCoAdapterStep_Success(t *testing.T) {
	requirePython(t)
	// MuJoCo espere o step + state completo no data.
	response := `{"step":7,"state":{"step":7,"entity_count":3,"world_time":0.5}}`
	script := writeFakeScript(t, response)
	a := NewMuJoCoAdapter(MuJoCoConfig{Script: script})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	step, err := a.Step(ctx, worldmodel.WorldState{Step: 6}, nil)
	if err != nil {
		t.Fatalf("Step: %v", err)
	}
	if step.Step != 7 {
		t.Fatalf("step incorreto: %d", step.Step)
	}
}

func TestMuJoCoAdapterQuery_MalformedJSON(t *testing.T) {
	requirePython(t)
	dir := t.TempDir()
	script := filepath.Join(dir, "badjson.py")
	content := `import sys
sys.stdout.write("not-json{{{{")
`
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatalf("write badjson script: %v", err)
	}
	a := NewMuJoCoAdapter(MuJoCoConfig{Script: script})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := a.Query(ctx, worldmodel.PhysicsQuery{})
	if err == nil {
		t.Fatal("Query deveria retornar erro ante JSON invalido")
	}
}
