// Package evidence — replay determinístico de evidência (mineração ADR-017,
// testkit openwork `evals/packages/testkit/src/spec/{index,runtime,types}.ts`).
//
// Contrato minerado: a evidência de uma execução de eval deve ser um
// REPLAYER DETERMINÍSTICO — idêntica de uma execução para outra, EXCETO pelos
// campos temporais (`At`/`Ms`). Cada ação emite um `TraceEntry{seq, at, verb,
// detail, ok, ms, error}` e cada passo carrega `step{name, ok, ms}` com um
// estado `not-reached` (passos posteriores nem rodam quando um anterior falha)
// e `needs`/`unmetNeeds` (pré-requisitos declarados → `skip` com reason, em vez
// de fail).
//
// PRINCÍPIO de segurança: ao serializar a evidência, a redaction é EMBUTIDA —
// email/Bearer/token/password são substituídos por sentinela, nunca vazam para
// o trace (reforço I8: segredo não é evidência vazada).
//
// stdlib-only, determinístico, zero dependência externa.
package evidence

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"regexp"
	"time"
)

// ──────────────────────────────────────────────────────────────
// Redaction embutida (I8: segredos nunca vazam para a evidência)
// ──────────────────────────────────────────────────────────────

// Sentinela de redaction usada no lugar do segredo vazado.
const (
	RedactedEmail    = "[redacted:email]"
	RedactedBearer   = "[redacted:bearer]"
	RedactedToken    = "[redacted:token]"
	RedactedPassword = "[redacted:password]"
)

var (
	// emailRedactRe captura um endereço de e-mail comum.
	emailRedactRe = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	// bearerRedactRe captura um token Bearer (`Bearer <tok>`); o prefixo é
	// preservado, apenas o token é trocado pela sentinela.
	bearerRedactRe = regexp.MustCompile(`(?i)(\bbearer\s+)[A-Za-z0-9._\-~+/]{4,}(=*)`)
	// passwordRedactRe captura atribuições de senha (`password=...`, `Password: ...`).
	passwordRedactRe = regexp.MustCompile(`(?i)((?:password|passwd|pwd|senha)\s*[=:]\s*)\S+`)
	// tokenRedactRe captura atribuições de token/secret/api_key/authorization.
	tokenRedactRe = regexp.MustCompile(`(?i)((?:access[_-]?token|api[_-]?key|token|secret|authorization)\s*[=:]\s*)\S+`)
)

// Redact substitui, numa string, os padrões de segredo conhecidos pela
// sentinela embutida: e-mail, Bearer token, senha e token/secret/api_key. É
// idempotente (aplicar 2x é seguro: sentinelas não casam com os padrões) e
// nunca fabrica informação. A ordem importa: primeiro o Bearer (que preserva o
// prefixo), depois password/token, para não mascarar o prefixo de contexto.
func Redact(s string) string {
	if s == "" {
		return s
	}
	s = emailRedactRe.ReplaceAllString(s, RedactedEmail)
	s = bearerRedactRe.ReplaceAllString(s, "${1}"+RedactedBearer)
	s = passwordRedactRe.ReplaceAllString(s, "${1}"+RedactedPassword)
	s = tokenRedactRe.ReplaceAllString(s, "${1}"+RedactedToken)
	return s
}

// ──────────────────────────────────────────────────────────────
// TraceEntry — a unidade de evidência de UMA ação
// ──────────────────────────────────────────────────────────────

// TraceEntry é a evidência de uma única ação da execução. Apenas Seq e Verb
// participam do hash/normalização; `At` e `Ms` são campos TEMPORAIS e são
// EXCLUÍDOS da igualdade de replay.
type TraceEntry struct {
	Seq    int       `json:"seq"`              // sequência 1-baseada (determinística)
	At     time.Time `json:"at"`               // TEMPORAL — excluído de ReplayEqual
	Verb   string    `json:"verb"`             // "world" | "seed" | "act" | "step" | "dispose" ...
	Detail string    `json:"detail"`           // já redacted
	OK     bool      `json:"ok"`               // true = ação passou
	Ms     int64     `json:"ms"`               // TEMPORAL — excluído de ReplayEqual
	Error  string    `json:"error,omitempty"`  // já redacted
}

// ReplayEqual compara duas entradas IGNORANDO os campos temporais (At e Ms):
// é a igualdade do replayer determinístico. Duas execuções do mesmo script
// produzem entradas ReplayEqual umas às outras, exceto pelo tempo.
func (t TraceEntry) ReplayEqual(o TraceEntry) bool {
	return t.Seq == o.Seq &&
		t.Verb == o.Verb &&
		t.Detail == o.Detail &&
		t.OK == o.OK &&
		t.Error == o.Error
}

// ──────────────────────────────────────────────────────────────
// Step — um passo nomeado com estado replayable
// ──────────────────────────────────────────────────────────────

// StepStatus é o estado de um passo no replay.
type StepStatus string

const (
	// StepOK: o passo rodou e passou.
	StepOK StepStatus = "ok"
	// StepFailed: o passo rodou e falhou.
	StepFailed StepStatus = "failed"
	// StepNotReached: um passo anterior falhou → este NÃO rodou (short-circuit).
	StepNotReached StepStatus = "not-reached"
	// StepSkipped: um pré-requisito (needs) não foi satisfeito → skip com reason.
	StepSkipped StepStatus = "skipped"
)

// Valid devolve true para os 4 estados canônicos.
func (s StepStatus) Valid() bool {
	switch s {
	case StepOK, StepFailed, StepNotReached, StepSkipped:
		return true
	}
	return false
}

// Step é um passo nomeado da execução: `{name, ok, ms}` com estado replayable,
// motivo explícito (Reason) e pré-requisitos declarados (Needs). O `Error` e o
// `Reason` já são redacted pelo harness.
type Step struct {
	Name   string     `json:"name"`
	Status StepStatus `json:"status"`
	OK     bool       `json:"ok"`
	Ms     int64      `json:"ms"`
	Error  string     `json:"error,omitempty"`
	Reason string     `json:"reason,omitempty"`
	Needs  []string   `json:"needs,omitempty"`
}

// ReplayEqual compara dois passos IGNORANDO o campo temporal (Ms). Um passo
// not-reached nunca carrega Ms por construção (não rodou), então a comparação
// exclui Ms para ser estável entre execuções.
func (s Step) ReplayEqual(o Step) bool {
	return s.Name == o.Name &&
		s.Status == o.Status &&
		s.OK == o.OK &&
		s.Error == o.Error &&
		s.Reason == o.Reason &&
		equalStrings(s.Needs, o.Needs)
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ──────────────────────────────────────────────────────────────
// ReplayHash — assinatura determinística da evidência
// ──────────────────────────────────────────────────────────────

// ReplayHash computa uma assinatura SHA-256 determinística da evidência,
// usando APENAS os campos não-temporais (Seq/Verb/Detail/OK/Error das entradas
// e Name/Status/OK/Error/Reason/Needs dos passos). Duas execuções do mesmo
// script produzem o mesmo hash, o que permite comparar replay sem ruído de
// tempo. Reusa o substrato de hashing idiossincrático do ledger (SHA-256).
func ReplayHash(entries []TraceEntry, steps []Step) string {
	h := sha256.New()
	var b8 [8]byte
	write := func(s string) {
		binary.BigEndian.PutUint64(b8[:], uint64(len(s)))
		h.Write(b8[:])
		h.Write([]byte(s))
	}
	writeByte := func(b byte) { h.Write([]byte{b}) }

	for _, e := range entries {
		write(e.Verb)
		write(e.Detail)
		write(e.Error)
		binary.BigEndian.PutUint64(b8[:], uint64(e.Seq))
		h.Write(b8[:])
		writeByte(byte(boolInt(e.OK)))
	}
	// termina a seção de entradas
	write("|steps|")
	for _, s := range steps {
		write(s.Name)
		write(string(s.Status))
		write(s.Error)
		write(s.Reason)
		writeByte(byte(boolInt(s.OK)))
		for _, n := range s.Needs {
			write("need:" + n)
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

// ReplayEqualTrace compara duas listas de entradas e passos ignorando os
// campos temporais, como um "diff" de replay. Cobre os arranjos mais comuns:
// entradas e/ou passos podem diferir de tamanho.
func ReplayEqualTrace(aEntries []TraceEntry, aSteps []Step, bEntries []TraceEntry, bSteps []Step) bool {
	if len(aEntries) != len(bEntries) || len(aSteps) != len(bSteps) {
		return false
	}
	for i := range aEntries {
		if !aEntries[i].ReplayEqual(bEntries[i]) {
			return false
		}
	}
	for i := range aSteps {
		if !aSteps[i].ReplayEqual(bSteps[i]) {
			return false
		}
	}
	return true
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
