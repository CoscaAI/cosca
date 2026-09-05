package gate

import (
	"testing"

	"github.com/CoscaAI/cosca/internal/sensor"
	"github.com/CoscaAI/cosca/internal/sensor/fusion"
)

// ─────────────────────────────────────────────────────────────────────────────
// Caso D — Continuidade semântica de capacidades após restarte (anti-regressão).
//
// A doutrina (ADR-036, §2.4 #2/#3) declara duas invariantes que o restart não
// pode quebrar:
//   (a) "Adicionar provider novo NUNCA altera o caminho primário" — o veredito do
//       gate é uma função PURA da evidência, jamais do conjunto de providers.
//   (b) "Escalação não pode subir 'por padrão'" — só sobe para o VLM quando o
//       gate diz INSUFICIÊNCIA/CONTRADIÇÃO; nunca "porque o provider existe".
//
// Estes testes NÃO duplicam gate_test.go (que cobre os branches individuais do
// Decide com inputs pré-fundidos) nem fusion_test.go (que cobre a matemática da
// fusão). Eles validam o INVARIANTE DE INTEGRAÇÃO do caminho completo
// sensor→fusion→gate, com um conjunto de providers VLM (registrados no ambiente)
// presente como fato externo — e provam que esse fato é INERTE à decisão.
// ─────────────────────────────────────────────────────────────────────────────

// vlmSet representa os providers VLM registrados no ambiente (fato externo).
// A existência deles é um dado do ecossistema, NÃO um insumo da decisão.
type vlmSet []string

// registeredVLM é o "leque de escalada" disponível à capacidade (o que vira
// ALVO se o gate escalar), jamais a identidade da capacidade.
var registeredVLM = vlmSet{"openai-vlm-x", "local-clip-z", "docker-vlm-y"}

// outcome é a saída do roteamento canônico: o veredito do gate + o sinal de
// "consultar VLM". O VLM é consultado SOMENTE quando o GATE pede Escalate E há
// um alvo registrado — exatamente a ordem da doutrina (escalada vem DEPOIS do
// gate), nunca como identidade da capacidade.
type outcome struct {
	v   Verdict
	vlm bool
}

// route é o caminho canônico per ADR-036, em versão de teste (sem runtime
// externo): sensores→fusion.Fuse→gate.Decide→(resolve | escala VLM).
// O conjunto de providers NÃO entra na decisão — só mina se existe um ALVO de
// escalada quando o gate pede. Retorna um mapa por referente fundido.
func route(obs []sensor.Observation, providers vlmSet, p Policy) map[fusion.Referent]outcome {
	res := fusion.Fuse(obs)
	out := make(map[fusion.Referent]outcome, len(res.Consensus))
	for _, c := range res.Consensus {
		v := p.Decide(Input{Fused: res}, c.Referent)
		out[c.Referent] = outcome{v: v, vlm: v.Escalate && len(providers) > 0}
	}
	return out
}

// assertVerdictInertToProviders verifica a invariante (a): para o MESMO conjunto
// de evidências, o veredito do gate é IDÊNTICO com e sem providers VLM
// registrados. Adicionar provider NUNCA altera o caminho primário.
func assertVerdictInertToProviders(t *testing.T, obs []sensor.Observation, p Policy) {
	t.Helper()
	without := route(obs, nil, p)
	with := route(obs, registeredVLM, p)
	if len(without) != len(with) {
		t.Fatalf("o nº de referentes fundidos mudou com providers: %d vs %d (anti-regressão violada)",
			len(without), len(with))
	}
	for ref, e := range without {
		w, ok := with[ref]
		if !ok {
			t.Fatalf("referente %q sumiu quando providers VLM foram registrados", ref)
		}
		if e.v != w.v {
			t.Errorf("referente %q: provider registrado alterou o VEREDITO do gate (%+v vs %+v) — caminho primário quebrado",
				ref, e.v, w.v)
		}
	}
}

// TestContinuity_ProviderPresenceNeverChangesVerdict percorre os cenários de
// evidência do Caso A/B/C da doutrina e afirma que a decisão é função DA
// EVIDÊNCIA, não da presença do provider: com e sem um leque de VLM registrado,
// o veredito é o mesmo, e o VLM só é consultado onde o gate manda.
func TestContinuity_ProviderPresenceNeverChangesVerdict(t *testing.T) {
	p := DefaultPolicy()
	vegasRegion := sensor.Region{X: 0, Y: 0, W: 200, H: 30}

	cases := []struct {
		name           string
		obs            []sensor.Observation
		expectEscalate bool // com VLM disponível, VLM é consultado sse o gate escala
	}{
		{
			name: "evidencia suficiente resolve (nao escala, mesmo com VLM dispoivel)",
			obs: []sensor.Observation{
				sensor.New(sensor.ModalityText, "line", "Botao continuar", 0.92, "ocr"),
				sensor.New(sensor.ModalityText, "line", "Botao continuar", 0.91, "ocr-2"),
			},
			expectEscalate: false,
		},
		{
			name: "contradicao no mesmo slot escala (sem ganhador silencioso)",
			obs: []sensor.Observation{
				sensor.New(sensor.ModalityText, "line", "Las Vegas", 0.95, "ocr").WithRegion(vegasRegion),
				sensor.New(sensor.ModalityText, "line", "Los Angeles", 0.96, "ocr-2").WithRegion(vegasRegion),
			},
			expectEscalate: true,
		},
		{
			name: "confianca baixa escala (resposta nao confiavel)",
			obs: []sensor.Observation{
				sensor.New(sensor.ModalityText, "line", "texto tao pequeno", 0.35, "ocr"),
				sensor.New(sensor.ModalityText, "line", "texto tao pequeno", 0.40, "ocr-2"),
			},
			expectEscalate: true,
		},
		{
			name: "fonte unica fraca/inferida escala",
			obs: []sensor.Observation{
				sensor.Inferred(sensor.ModalityVisual, "entity", "objeto", 0.55, "tracker"),
			},
			expectEscalate: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Invariante (a): providers são inertes ao veredito.
			assertVerdictInertToProviders(t, tc.obs, p)

			// Invariante (b): com VLM disponível, VLM só é consultado quando o
			// gate escala — a escalada é decisão por evidência, não "por existir".
			with := route(tc.obs, registeredVLM, p)
			for ref, o := range with {
				if o.v.Escalate != tc.expectEscalate {
					t.Errorf("referente %q: gate deveria escalar=%v (por evidência), got %v (presente do provider alterou decisão?)",
						ref, tc.expectEscalate, o.v.Escalate)
				}
				if o.vlm != tc.expectEscalate {
					t.Errorf("referente %q: esperava consultar VLM=%v (decisão do gate), got %v",
						ref, tc.expectEscalate, o.vlm)
				}
				if o.vlm && !o.v.Escalate {
					t.Errorf("referente %q: VLM consultado sem o gate ter escalado (viola 'VLM no topo, nunca na base')", ref)
				}
			}
		})
	}
}

// TestContinuity_VLMPresenceDoesNotEscalateSufficientEvidence é a sentinela
// anti-regressão mais explícita do Caso D: mesmo com um leque de VLM registrado
// no ambiente, se a evidência local é suficiente o caminho RESOLVE (não escala).
// Escalação NUNCA sobe "por padrão" — só por insuficiência/contradição.
func TestContinuity_VLMPresenceDoesNotEscalateSufficientEvidence(t *testing.T) {
	obs := []sensor.Observation{
		sensor.New(sensor.ModalityText, "line", "Continuar", 0.93, "ocr"),
		sensor.New(sensor.ModalityText, "line", "Continuar", 0.91, "ocr-2"),
	}
	with := route(obs, registeredVLM, DefaultPolicy())
	if len(with) == 0 {
		t.Fatalf("evidência suficiente deveria produzir 1 referente fundido")
	}
	for ref, o := range with {
		if o.v.Action != Resolve || o.v.Escalate || o.vlm {
			t.Errorf("referente %q: evidência suficiente + VLM disponível NÃO pode escalar; got action=%s escalate=%v vlm=%v",
				ref, o.v.Action, o.v.Escalate, o.vlm)
		}
		if o.v.Reason != ReasonConfidentConsensus {
			t.Errorf("referente %q: reason deveria ser confident_consensus, got %s", ref, o.v.Reason)
		}
	}
}

// TestContinuity_VLMInvokedOnlyWhenGateEscalates confirma a ordem da doutrina:
// o VLM (ou qualquer ALVO de escalada) só é consultado quando o gate já
// decidiu Escalate. Com evidência suficiente, existir um alvo VLM NÃO dispara
// consulta alguma.
func TestContinuity_VLMInvokedOnlyWhenGateEscalates(t *testing.T) {
	obs := []sensor.Observation{
		sensor.New(sensor.ModalityText, "line", "Pomar", 0.99, "ocr"),
		sensor.New(sensor.ModalityText, "line", "Pomar", 0.98, "ocr-2"),
	}
	for _, o := range route(obs, registeredVLM, DefaultPolicy()) {
		if o.vlm {
			t.Errorf("VLM não deveria ser consultado com consenso forte (got vlm=%v)", o.vlm)
		}
	}
}
