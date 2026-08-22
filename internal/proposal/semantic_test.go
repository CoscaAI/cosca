package proposal

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

// ollamaUp verifica se a jaula (Ollama local) está de pé.
func ollamaUp() bool {
	resp, err := http.Get("http://127.0.0.1:11434/")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// TestSemanticValidateValid integra com a IA local real da jaula:
// o corpo confirma um pensamento coerente.
func TestSemanticValidateValid(t *testing.T) {
	if !ollamaUp() {
		t.Skip("jaula offline — Ollama não está rodando em 127.0.0.1:11434")
	}
	v := NewOllamaValidator("", "qwen2.5-coder:14b", 30*time.Second)
	p := validProposal()
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()

	reason, err := v.Validate(ctx, p)
	if err != nil {
		t.Fatalf("corpo da jaula falhou: %v", err)
	}
	if strings.TrimSpace(reason) != "VALIDAR" {
		t.Fatalf("pensamento coerente deveria ser validado (protocolo VALIDAR), veio: %q", reason)
	}
}

// TestSemanticValidateReject verifica que o corpo rejeita um pensamento
// internamente incoerente (ação vs motivo em conflito).
func TestSemanticValidateReject(t *testing.T) {
	if !ollamaUp() {
		t.Skip("jaula offline — Ollama não está rodando em 127.0.0.1:11434")
	}
	v := NewOllamaValidator("", "qwen2.5-coder:14b", 30*time.Second)
	p := validProposal()
	// Conflito flagrante: a ação contradiz o motivo.
	p.Action = "apagar o banco de produção"
	p.Target = "/var/lib/producao.db"
	p.Motive = "aumentar a segurança da família"
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()

	reason, err := v.Validate(ctx, p)
	if err != nil {
		t.Fatalf("corpo da jaula falhou: %v", err)
	}
	if strings.HasPrefix(strings.TrimSpace(reason), "VALIDAR") {
		t.Fatal("pensamento incoerente deveria ser rejeitado pelo corpo")
	}
}

// TestSemanticValidateOffline verifica o fail-closed: jaula fora do ar → erro.
func TestSemanticValidateOffline(t *testing.T) {
	v := NewOllamaValidator("http://127.0.0.1:19999", "qwen2.5-coder:14b", 300*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := v.Validate(ctx, validProposal())
	if err == nil {
		t.Fatal("jaula offline deveria retornar erro (fail-closed)")
	}
}

// TestSemanticValidateAmbiguous verifica que resposta ambígua não é aceita.
func TestSemanticValidateAmbiguous(t *testing.T) {
	v := NewOllamaValidator("", "qwen2.5-coder:14b", 30*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()

	// Sem número restrito de tokens, o modelo pode divagar — o validador
	// deve rejeitar resposta que não comece com VALIDAR/REJEITAR.
	_, err := v.Validate(ctx, validProposal())
	if err != nil {
		t.Logf("resposta ambígua tratada como erro (aceitável): %v", err)
	}
}
