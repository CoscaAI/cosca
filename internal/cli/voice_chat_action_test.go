package cli

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/visionact"
	"github.com/CoscaAI/cosca/internal/worldmodel"
	"github.com/CoscaAI/cosca/internal/worldmodel/vision"
)

// ──────────────────────────────────────────────────────────────
// parsePerceptionAction — reconhecimento de ação
// ──────────────────────────────────────────────────────────────

func TestParsePerceptionAction_LookAtScreen(t *testing.T) {
	cases := []string{
		"olha a tela",
		"Olha essa tela",
		"o que você vê?",
		"o que você está vendo?",
		"veja isso",
		"ve o que tem na tela",
	}
	for _, c := range cases {
		act := parsePerceptionAction(c)
		if act.kind != actionLookAtScreen {
			t.Errorf("parse(%q) kind = %v, want actionLookAtScreen", c, act.kind)
		}
	}
}

func TestParsePerceptionAction_RecordAndInterpret(t *testing.T) {
	cases := []string{
		"grava a tela",
		"Grava 30 segundos e interpreta",
		"registra por 10 segundos",
		"grave e interprete",
		"interpreta o que está acontecendo",
		"registre 15s",
	}
	for _, c := range cases {
		act := parsePerceptionAction(c)
		if act.kind != actionRecordAndInterpret {
			t.Errorf("parse(%q) kind = %v, want actionRecordAndInterpret", c, act.kind)
		}
	}
}

func TestParsePerceptionAction_RecordDuration(t *testing.T) {
	act := parsePerceptionAction("grava 30 segundos e interpreta")
	if act.kind != actionRecordAndInterpret {
		t.Fatalf("kind = %v", act.kind)
	}
	if act.duration != 30*time.Second {
		t.Errorf("duration = %v, want 30s", act.duration)
	}

	act = parsePerceptionAction("registrar por 10 segundos")
	if act.duration != 10*time.Second {
		t.Errorf("duration = %v, want 10s", act.duration)
	}

	act = parsePerceptionAction("grava")
	if act.kind != actionRecordAndInterpret {
		t.Fatalf("kind = %v", act.kind)
	}
	if act.duration != visionact.DefaultRecordDuration {
		t.Errorf("default duration = %v, want %v", act.duration, visionact.DefaultRecordDuration)
	}
}

func TestParsePerceptionAction_None(t *testing.T) {
	cases := []string{
		"",
		"bom dia",
		"qual é a previsão do tempo?",
		"conte uma piada",
		"o que é isto",
		"me chama de don",
	}
	for _, c := range cases {
		if act := parsePerceptionAction(c); act.kind != actionNone {
			t.Errorf("parse(%q) kind = %v, want actionNone", c, act.kind)
		}
	}
}

func TestParsePerceptionAction_AccentInsensitive(t *testing.T) {
	// O STT nem sempre acentua: "você" → "voce", "vê" → "ve".
	for _, c := range []string{"o que voce ve", "olha essa tela e me diz"} {
		if act := parsePerceptionAction(c); act.kind != actionLookAtScreen && act.kind != actionRecordAndInterpret {
			t.Errorf("parse(%q) should recognize an action, got %v", c, act.kind)
		}
	}
}

func TestParsePerceptionAction_RecordWinsOverLook(t *testing.T) {
	// "grava" é um sinal mais forte que olhar no meio da frase.
	act := parsePerceptionAction("olha e grava por 5 segundos")
	if act.kind != actionRecordAndInterpret {
		t.Errorf("kind = %v, want actionRecordAndInterpret (record verb wins)", act.kind)
	}
	if act.duration != 5*time.Second {
		t.Errorf("duration = %v, want 5s", act.duration)
	}
}

func TestNormalizePT(t *testing.T) {
	if got := normalizePT("ÁÉÍÓÚÀÂÊÔÃÕÇ âãêõ"); got != "aeiouaaeoaoc aaeo" {
		t.Errorf("normalizePT = %q", got)
	}
}

// ──────────────────────────────────────────────────────────────
// respondWithTool — fallback honesto
// ──────────────────────────────────────────────────────────────

func TestRespondWithTool_NoContext(t *testing.T) {
	got := respondWithTool("", "olha a tela")
	if !strings.Contains(got, "Não consegui perceber nada agora") || !strings.Contains(got, "olha a tela") {
		t.Errorf("respondWithTool empty ctx = %q", got)
	}
}

func TestRespondWithTool_WithContext(t *testing.T) {
	got := respondWithTool("Vision observation: 1 entity(ies).", "olha a tela")
	if !strings.Contains(got, "Vision observation") || !strings.Contains(got, "olha a tela") {
		t.Errorf("respondWithTool ctx = %q", got)
	}
}

// ──────────────────────────────────────────────────────────────
// runPerceptionAction — instrumenta a ferramenta e delibera
// ──────────────────────────────────────────────────────────────

// TestRunPerceptionAction_LookAtScreen verifies that the action dispatches the
// LookAtScreen tool and (with a fake brain) returns the deliberate response.
func TestRunPerceptionAction_LookAtScreen(t *testing.T) {
	fp := &fakeBrainProvider{response: "Vejo uma janela à frente."}
	brain := newVoiceBrain(fp, 0, 0, nopLogger())
	SetVoiceBrain(brain)
	defer SetVoiceBrain(nil)

	// Engine with a fake captor/vision (no onnxruntime).
	e := newActionTestEngine(t)

	act := perceptionAction{kind: actionLookAtScreen}
	resp, err := runPerceptionAction(context.Background(), act, e, "olha a tela", nil, nopLogger())
	if err != nil {
		t.Fatalf("runPerceptionAction error: %v", err)
	}
	if !strings.Contains(resp, "janela") {
		t.Fatalf("response = %q, want the brain's smart reply", resp)
	}
	// A resposta do brain não deve falhar no caminho da ferramenta.
	if !strings.Contains(fp.gotMsg.Content, "olha a tela") {
		t.Errorf("brain prompt should include the Don's utterance, got:\n%s", fp.gotMsg.Content)
	}
}

// TestRunPerceptionAction_LookAtScreen_NoBrain verifies the graceful fallback:
// no brain → template (respondWithTool) relays what the tool saw.
func TestRunPerceptionAction_LookAtScreen_NoBrain(t *testing.T) {
	SetVoiceBrain(nil)
	defer SetVoiceBrain(nil)
	e := newActionTestEngine(t)
	act := perceptionAction{kind: actionLookAtScreen}

	resp, err := runPerceptionAction(context.Background(), act, e, "olha a tela", nil, nopLogger())
	if err != nil {
		t.Fatalf("runPerceptionAction error: %v", err)
	}
	if !strings.Contains(resp, "janela") {
		t.Fatalf("no-brain fallback should relay the tool result, got %q", resp)
	}
}

// TestRunPerceptionAction_Record verifies the record path feeds the tool report
// to the brain.
func TestRunPerceptionAction_Record(t *testing.T) {
	fp := &fakeBrainProvider{response: "A tela mudou — apareceu um objeto novo."}
	brain := newVoiceBrain(fp, 0, 0, nopLogger())
	SetVoiceBrain(brain)
	defer SetVoiceBrain(nil)

	e := newActionTestEngine(t)
	act := perceptionAction{kind: actionRecordAndInterpret, duration: 10 * time.Millisecond}

	resp, err := runPerceptionAction(context.Background(), act, e, "grava e interpreta", nil, nopLogger())
	if err != nil {
		t.Fatalf("runPerceptionAction record error: %v", err)
	}
	if !strings.Contains(resp, "mudou") {
		t.Fatalf("response = %q, want the brain's record reply", resp)
	}
}

// newActionTestEngine wires a visionact.Engine with deterministic fakes for the
// CLI-level tests (no onnxruntime).
func newActionTestEngine(t *testing.T) *visionact.Engine {
	t.Helper()
	cap := &fakeActionCaptor{frame: solidActionPNG(t)}
	vis := &fakeActionVision{}
	return visionact.New(
		visionact.WithCaptor(cap),
		visionact.WithVision(vis.run),
		visionact.WithRecordInterval(time.Millisecond),
		visionact.WithMaxFrames(20),
	)
}

// fakeActionCaptor is a deterministic Captor that returns a fixed PNG frame.
type fakeActionCaptor struct {
	frame []byte
}

func (c *fakeActionCaptor) Capture(context.Context) ([]byte, int, int, error) {
	return c.frame, 1, 1, nil
}

// fakeActionVision returns a deterministic Observation with a couple of
// entities every time (simulating the native ONNX vision without onnxruntime).
type fakeActionVision struct{}

func (f *fakeActionVision) run(_ context.Context, _ []byte, _, _ int) (*vision.Observation, error) {
	return &vision.Observation{
		Entities: []worldmodel.WorldEntity{
			{ID: "e1", Type: worldmodel.EntityObject, Label: "janela", Confidence: 0.91},
			{ID: "e2", Type: worldmodel.EntityObject, Label: "texto", Confidence: 0.80},
		},
	}, nil
}

// solidActionPNG returns a small deterministic PNG image (a solid colour), used
// as a fake capture frame in the CLI-level action tests.
func solidActionPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			img.Set(x, y, color.RGBA{R: 34, G: 177, B: 76, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png encode: %v", err)
	}
	return buf.Bytes()
}
