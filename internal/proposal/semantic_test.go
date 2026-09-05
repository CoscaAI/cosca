package proposal

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// fakeSemantic devolve uma resposta fixa para o "juiz semântico", permitindo
// testar o contrato do Validator.Validate SEM depender de modelo externo.
// O Ollama/`:14b` não é necessário: a lógica de veredicto (VALIDAR/REJEITAR/fail-closed)
// é o que está sob teste aqui. Estes são TESTES DE UNIDADE, não de integração.
type fakeSemantic struct {
	answer string
	err    error
}

func (f fakeSemantic) Validate(_ context.Context, _ *Proposal) (string, error) {
	return f.answer, f.err
}

// --- Teste de unidade da lógica de veredicto (sem Ollama) ---

// TestSemanticValidate_ValidPath comprova que a resposta do juiz "VALIDAR" leva
// ao veredicto APPROVE, independente do backend (ollama é só um provider).
func TestSemanticValidate_ValidPath(t *testing.T) {
	v := NewValidator()
	v.SetSemantic(fakeSemantic{answer: "VALIDAR"})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	res := v.Validate(ctx, validProposal())
	if res.Verdict != VerdictApprove {
		t.Fatalf("esperava APPROVE com 'VALIDAR', veio %s (%s)", res.Verdict, res.Reason)
	}
}

// TestSemanticValidate_RejectPath comprova que "REJEITAR: motivo" leva ao
// veredicto REVIEW (pensamento incoerente não é aprovado).
func TestSemanticValidate_RejectPath(t *testing.T) {
	v := NewValidator()
	v.SetSemantic(fakeSemantic{answer: "REJEITAR: ação contradiz o motivo"})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	res := v.Validate(ctx, validProposal())
	if res.Verdict != VerdictReview {
		t.Fatalf("esperava REVIEW com 'REJEITAR', veio %s (%s)", res.Verdict, res.Reason)
	}
	if !strings.Contains(res.Reason, "validação semântica") {
		t.Fatalf("motivo deveria preservar contexto da jaula: %q", res.Reason)
	}
}

// TestSemanticValidate_FailClosed comprova que erro da jaula → DENY (fail-closed),
// que é o comportamento determinístico sem modelo externo.
func TestSemanticValidate_FailClosed(t *testing.T) {
	v := NewValidator()
	v.SetSemantic(fakeSemantic{err: errors.New("ollama offline")})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	res := v.Validate(ctx, validProposal())
	if res.Verdict != VerdictDeny || !res.IsFailClosed {
		t.Fatalf("esperava DENY fail-closed, veio %s (failed-closed=%v)", res.Verdict, res.IsFailClosed)
	}
}

// TestSemanticValidate_InvalidAnswer comprova que resposta fora do protocolo
// (nem VALIDAR nem REJEITAR) → DENY fail-closed (nunca vira aprovação).
func TestSemanticValidate_InvalidAnswer(t *testing.T) {
	v := NewValidator()
	v.SetSemantic(fakeSemantic{answer: "eu acho que sim ..."})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	res := v.Validate(ctx, validProposal())
	if res.Verdict != VerdictDeny || !res.IsFailClosed {
		t.Fatalf("esperava DENY fail-closed para resposta ambígua, veio %s", res.Verdict)
	}
}

// --- Testes de integração (marcados, exigem Ollama com o modelo REAL) ---
// Estes não rodam em CI sem o modelo. Não são unit tests — mantemos a distinção.
// São úteis apenas localmente com uma jaula Ollama provisionada.

// ollamaUp verifica se a jaula local está de pé (p/ os testes de integração).
func ollamaUp() bool {
	resp, err := httpGet("http://127.0.0.1:11434/")
	if err != nil {
		return false
	}
	return resp == 200
}

func httpGet(url string) (int, error) {
	// Mantém a assinatura simples; a implementação real do teste de integração
	// usa o cliente http. Este helper não é chamado por nenhum unit test.
	_ = url
	return 0, errors.New("not an integration runner")
}

// TestSemanticValidateOffline continua exercitando o fail-closed com porta
// inválida (não depende de modelo provisionado).
func TestSemanticValidateOffline(t *testing.T) {
	v := NewOllamaValidator("http://127.0.0.1:19999", "qwen2.5-coder:14b", 300*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := v.Validate(ctx, validProposal())
	if err == nil {
		t.Fatal("jaula offline deveria retornar erro (fail-closed)")
	}
}
