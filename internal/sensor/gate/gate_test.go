package gate

import (
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/sensor"
	"github.com/CoscaAI/cosca/internal/sensor/fusion"
)

// goodFuse monta um Result de fusão com um único referente de consenso forte.
// O referente derivado é o conteúdo normalizado (lowercase/trim).
func goodFuse(content string, conf float64, sources ...string) fusion.Result {
	var obs []sensor.Observation
	for _, src := range sources {
		obs = append(obs, sensor.New(sensor.ModalityText, "line", content, sensor.Confidence(conf), src))
	}
	return fusion.Fuse(obs)
}

func ref(content string) fusion.Referent {
	// Normaliza igual ao defaultReferent da fusão: lowercase + trim.
	return fusion.Referent(strings.ToLower(strings.TrimSpace(content)))
}

// TestResolveConfidentConsensus: dois sensores independantes, consenso forte,
// sem contradição → RESOLVE (não escala). O VLM não é chamado à toa.
func TestResolveConfidentConsensus(t *testing.T) {
	res := goodFuse("Las Vegas", 0.91, "ocr", "ocr-2")
	in := Input{Fused: res}
	v := DefaultPolicy().Decide(in, ref("Las Vegas"))
	if v.Action != Resolve {
		t.Errorf("esperava RESOLVE, got %s (%s)", v.Action, v.Reason)
	}
	if v.Escalate {
		t.Errorf("nao deveria escalar em consenso forte")
	}
	if v.Reason != ReasonConfidentConsensus {
		t.Errorf("reason errado: %s", v.Reason)
	}
}

// TestEscalateOnContradiction: contradição no mesmo slot → escala, mesmo com
// confiança alta. O kernel não escolhe vencedor.
func TestEscalateOnContradiction(t *testing.T) {
	region := sensor.Region{X: 0, Y: 0, W: 200, H: 30}
	a := sensor.New(sensor.ModalityText, "line", "Las Vegas", 0.95, "ocr").WithRegion(region)
	b := sensor.New(sensor.ModalityText, "line", "Los Angeles", 0.98, "ocr-2").WithRegion(region)
	res := fusion.Fuse([]sensor.Observation{a, b})
	in := Input{Fused: res}
	v := DefaultPolicy().Decide(in, "las vegas")
	if v.Action != Escalate {
		t.Errorf("contradicao deveria escalar, got %s", v.Action)
	}
	if v.Reason != ReasonContradiction {
		t.Errorf("reason deveria ser contradiction, got %s", v.Reason)
	}
}

// TestEscalateOnLowConfidence: consenso abaixo do limiar → escalar (resposta
// não confiável).
func TestEscalateOnLowConfidence(t *testing.T) {
	res := goodFuse("texto tao pequeno", 0.4, "ocr", "ocr-2")
	in := Input{Fused: res}
	v := DefaultPolicy().Decide(in, ref("texto tao pequeno"))
	if v.Action != Escalate {
		t.Errorf("baixa confianca deveria escalar, got %s", v.Action)
	}
	if v.Reason != ReasonLowConfidence {
		t.Errorf("reason deveria ser low_confidence, got %s", v.Reason)
	}
}

// TestEscalateSingleSourceWeak: uma única fonte INFERRED frágil → escalar.
func TestEscalateSingleSourceWeak(t *testing.T) {
	o := sensor.Inferred(sensor.ModalityVisual, "entity", "objeto", 0.55, "tracker")
	res := fusion.Fuse([]sensor.Observation{o})
	in := Input{Fused: res}
	v := DefaultPolicy().Decide(in, "objeto")
	if v.Action != Escalate {
		t.Errorf("fonte unica fraca deveria escalar, got %s", v.Action)
	}
}

// TestPedidoExplicitoAlwaysEscalates: pedido explícito do Don escala mesmo com
// consenso perfeito. Jamais negar uma análise pedida.
func TestPedidoExplicitoAlwaysEscalates(t *testing.T) {
	res := goodFuse("conteudo", 0.98, "ocr", "ocr-2")
	in := Input{Fused: res, PedidoExplicito: true}
	v := DefaultPolicy().Decide(in, ref("conteudo"))
	if !v.Escalate {
		t.Errorf("pedido explicito deveria sempre escalar")
	}
}

// TestNoInputResolves: sem inputs → nada a decidir, não escala.
func TestNoInputResolves(t *testing.T) {
	in := Input{}
	v := DefaultPolicy().Decide(in, "qualquer")
	if v.Escalate {
		t.Errorf("sem input nao deveria escalar")
	}
	if v.Reason != ReasonNoInput {
		t.Errorf("reason deveria ser no_input, got %s", v.Reason)
	}
}

// TestPolicyOverride: política menos conservadora (contradição não força
// escalada) muda a decisão quando há consenso forte.
func TestPolicyOverride(t *testing.T) {
	region := sensor.Region{X: 0, Y: 0, W: 200, H: 30}
	a := sensor.New(sensor.ModalityText, "line", "Las Vegas", 0.95, "ocr").WithRegion(region)
	b := sensor.New(sensor.ModalityText, "line", "Los Angeles", 0.98, "ocr-2").WithRegion(region)
	res := fusion.Fuse([]sensor.Observation{a, b})
	in := Input{Fused: res}

	// Default: escalada na contradição.
	if DefaultPolicy().Decide(in, "las vegas").Action != Escalate {
		t.Errorf("default deveria escalar na contradicao")
	}
	// Override: contradição não força escalada, consenso alto resolve.
	p := DefaultPolicy()
	p.ForceEscalateOnContradiction = false
	v := p.Decide(in, "las vegas")
	if v.Action != Resolve {
		t.Errorf("com override nao-forcado, consenso alto deveria resolver")
	}
}

// TestNonExistentReferentResolves: referente inexistente → resolve sem escala.
func TestNonExistentReferentResolves(t *testing.T) {
	res := goodFuse("conteudo", 0.9, "ocr")
	in := Input{Fused: res}
	v := DefaultPolicy().Decide(in, ref("nao-existe"))
	if v.Escalate {
		t.Errorf("referente inexistente nao deveria escalar")
	}
}
