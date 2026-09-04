package evidence

import (
	"strings"
	"testing"
	"time"
)

// TestRedact_HidesSecrets valida que os 4 padrões minerados (email, Bearer,
// token, senha) são substituídos pela sentinela e NUNCA vazam o valor.
func TestRedact_HidesSecrets(t *testing.T) {
	input := "contato dev@cosca.ai + token=abc-1234 + Bearer eyJhbGciOiJIUzI1NiJ9 + password=supersecret + Authorization: Bearer abc123"
	out := Redact(input)

	for _, leak := range []string{
		"dev@cosca.ai",
		"abc-1234",
		"eyJhbGciOiJIUzI1NiJ9",
		"supersecret",
		"abc123",
	} {
		if strings.Contains(out, leak) {
			t.Errorf("secret %q vazou após Redact: %q", leak, out)
		}
	}
	if !strings.Contains(out, RedactedEmail) {
		t.Errorf("email deveria ter sentinela %q: %q", RedactedEmail, out)
	}
	if !strings.Contains(out, RedactedBearer) {
		t.Errorf("bearer deveria ter sentinela %q: %q", RedactedBearer, out)
	}
	if !strings.Contains(out, RedactedToken) {
		t.Errorf("token deveria ter sentinela %q: %q", RedactedToken, out)
	}
	if !strings.Contains(out, RedactedPassword) {
		t.Errorf("password deveria ter sentinela %q: %q", RedactedPassword, out)
	}
}

// TestRedact_Idempotent valida que aplicar Redact 2x não corrompe (sentinela
// não casa com os padrões; segredo já trocado não vaza de novo).
func TestRedact_Idempotent(t *testing.T) {
	first := Redact("a@b.com token=xyz perm 123")
	second := Redact(first)
	if first != second {
		t.Errorf("Redact não é idempotente: %q -> %q", first, second)
	}
}

// TestRedact_KeepsBenign verifica que texto sem segredo permanece intacto.
func TestRedact_KeepsBenign(t *testing.T) {
	if got := Redact("hello world, go build ./..."); got != "hello world, go build ./..." {
		t.Errorf("texto benigno foi alterado: %q", got)
	}
	if Redact("") != "" {
		t.Errorf("string vazia deveria permanecer vazia")
	}
}

// TestTraceEntry_ReplayEqual_IgnoresTemporal confirma o contrato do replayer:
// duas entradas idênticas exceto At/Ms são ReplayEqual; um campo não-temporal
// diferente NÃO é.
func TestTraceEntry_ReplayEqual_IgnoresTemporal(t *testing.T) {
	a := TraceEntry{Seq: 1, At: time.Now(), Verb: "step", Detail: "s1", OK: true, Ms: 5, Error: ""}
	b := TraceEntry{Seq: 1, At: time.Now().Add(10 * time.Second), Verb: "step", Detail: "s1", OK: true, Ms: 999, Error: ""}
	if !a.ReplayEqual(b) {
		t.Error("entradas idênticas exceto At/Ms deveriam ser ReplayEqual")
	}
	c := b
	c.OK = false
	if a.ReplayEqual(c) {
		t.Error("entradas com OK diferente NÃO deveriam ser ReplayEqual")
	}
	d := b
	d.Detail = "s2"
	if a.ReplayEqual(d) {
		t.Error("entradas com Detail diferente NÃO deveriam ser ReplayEqual")
	}
}

// TestStepStatus_Valid valida os 4 estados canônicos.
func TestStepStatus_Valid(t *testing.T) {
	for _, s := range []StepStatus{StepOK, StepFailed, StepNotReached, StepSkipped} {
		if !s.Valid() {
			t.Errorf("estado %q deveria ser válido", s)
		}
	}
	if StepStatus("junk").Valid() {
		t.Error("estado inválido não deveria ser válido")
	}
}

// TestStep_ReplayEqual_IgnoresMs valida que Ms (temporal) é excluído da
// comparação de replay dos passos.
func TestStep_ReplayEqual_IgnoresMs(t *testing.T) {
	a := Step{Name: "s", Status: StepOK, OK: true, Ms: 3}
	b := Step{Name: "s", Status: StepOK, OK: true, Ms: 4000}
	if !a.ReplayEqual(b) {
		t.Error("passos iguais exceto Ms deveriam ser ReplayEqual")
	}
	c := b
	c.Status = StepNotReached
	c.OK = false
	if a.ReplayEqual(c) {
		t.Error("passos com Status diferente NÃO deveriam ser ReplayEqual")
	}
	d := b
	d.Needs = []string{"x"}
	if a.ReplayEqual(d) {
		t.Error("passos com Needs diferente NÃO deveriam ser ReplayEqual")
	}
}

// TestReplayHash_DeterministicIgnoresTemporal valida que o hash é estável
// entre execuções com os mesmos campos não-temporais e muda se algo mudar.
func TestReplayHash_DeterministicIgnoresTemporal(t *testing.T) {
	entries1 := []TraceEntry{
		{Seq: 1, At: time.Now(), Verb: "seed", Detail: "w", OK: true, Ms: 1},
		{Seq: 2, At: time.Now(), Verb: "step", Detail: "s1", OK: true, Ms: 2},
	}
	steps1 := []Step{
		{Name: "s1", Status: StepOK, OK: true, Ms: 2},
	}
	entries2 := []TraceEntry{
		{Seq: 1, At: time.Now().Add(time.Hour), Verb: "seed", Detail: "w", OK: true, Ms: 999},
		{Seq: 2, At: time.Now().Add(time.Hour), Verb: "step", Detail: "s1", OK: true, Ms: 777},
	}
	steps2 := []Step{
		{Name: "s1", Status: StepOK, OK: true, Ms: 1234},
	}
	h1 := ReplayHash(entries1, steps1)
	h2 := ReplayHash(entries2, steps2)
	if h1 != h2 {
		t.Errorf("hash deveria ignorar campos temporais: %s != %s", h1, h2)
	}
	if h1 == "" {
		t.Error("hash não pode ser vazio")
	}

	changed := []Step{{Name: "s1", Status: StepFailed, OK: false}}
	if ReplayHash(entries1, changed) == h1 {
		t.Error("hash deveria mudar quando um passo falha")
	}
}

// TestReplayEqualTrace valida a comparação de trilhas completas (ignorando
// tempo) e de tamanhos diferentes.
func TestReplayEqualTrace(t *testing.T) {
	aE := []TraceEntry{{Seq: 1, Verb: "seed", OK: true}}
	aS := []Step{{Name: "s", Status: StepOK, OK: true}}
	bE := []TraceEntry{{Seq: 1, Verb: "seed", OK: true, Ms: 5}}
	bS := []Step{{Name: "s", Status: StepOK, OK: true, Ms: 42}}
	if !ReplayEqualTrace(aE, aS, bE, bS) {
		t.Error("trilhas iguais exceto tempo deveriam ser iguais")
	}
	cE := []TraceEntry{{Seq: 1, Verb: "step", OK: true}}
	if ReplayEqualTrace(aE, aS, cE, bS) {
		t.Error("trilhas com verb diferente NÃO deveriam ser iguais")
	}
	if ReplayEqualTrace(aE, aS, nil, nil) {
		t.Error("tamanhos diferentes NÃO deveriam ser iguais")
	}
}
