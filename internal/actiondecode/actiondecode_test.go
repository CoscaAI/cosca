package actiondecode

import "testing"

// TestDecode_StructuredPacket: o LLM devolve um instruction packet JSON válido
// → StatusStructured com a decisão decodificada.
func TestDecode_StructuredPacket(t *testing.T) {
	raw := `{"decision":"EDIT","target":"internal/mcpserver/registry.go","action":"align_registry","confidence":0.94,"reason":"single source of truth is internal/mcpserver","verification":["go test ./internal/mcpserver/..."]}`
	p := Decode(raw)
	if p.Status != StatusStructured {
		t.Fatalf("status = %q, esperava structured", p.Status)
	}
	if p.Structured == nil {
		t.Fatal("Structured nil em packet estruturado")
	}
	if p.Structured.Decision != "EDIT" || p.Structured.Target != "internal/mcpserver/registry.go" {
		t.Fatalf("decisão errada: %+v", p.Structured)
	}
	if len(p.Structured.Verification) != 1 || p.Structured.Verification[0] != "go test ./internal/mcpserver/..." {
		t.Fatalf("verification errada: %+v", p.Structured.Verification)
	}
}

// TestDecode_MarkdownWrapped: o LLM embrulha o JSON em ```json ... ``` — o
// decoder extrai o bloco e devolve estruturado.
func TestDecode_MarkdownWrapped(t *testing.T) {
	raw := "Aqui está a decisão:\n```json\n{\"decision\":\"RUN\",\"action\":\"run_tests\",\"confidence\":0.9}\n```"
	p := Decode(raw)
	if p.Status != StatusStructured {
		t.Fatalf("status = %q, esperava structured (markdown-wrapped)", p.Status)
	}
	if p.Structured.Decision != "RUN" {
		t.Fatalf("decisão = %q, esperava RUN", p.Structured.Decision)
	}
}

// TestDecode_ProseFailOpen: o LLM devolve prosa normal → StatusText com a
// resposta crua preservada (NUNCA quebra).
func TestDecode_ProseFailOpen(t *testing.T) {
	raw := "Vou analisar o problema e voltar com a resposta completa."
	p := Decode(raw)
	if p.Status != StatusText {
		t.Fatalf("status = %q, esperava text (fail-open)", p.Status)
	}
	if p.Text != raw {
		t.Fatalf("text = %q, esperava a resposta crua preservada", p.Text)
	}
	if p.Structured != nil {
		t.Fatal("Structured deveria ser nil em fail-open")
	}
}

// TestDecode_EmptyResponse: resposta vazia → text (nunca structured).
func TestDecode_EmptyResponse(t *testing.T) {
	p := Decode("")
	if p.Status != StatusText {
		t.Fatalf("status = %q, esperava text para resposta vazia", p.Status)
	}
}

// TestDecode_InvalidJSON: JSON malformado → text (fail-open).
func TestDecode_InvalidJSON(t *testing.T) {
	p := Decode(`{"decision":"EDIT","target":}`)
	if p.Status != StatusText {
		t.Fatalf("status = %q, esperava text para JSON inválido", p.Status)
	}
}

// TestDecode_EmptyPacket: JSON válido mas sem decision/action → text (não é
// uma decisão).
func TestDecode_EmptyPacket(t *testing.T) {
	p := Decode(`{"confidence":0.9,"reason":"sem ação"}`)
	if p.Status != StatusText {
		t.Fatalf("status = %q, esperava text para packet sem decision/action", p.Status)
	}
}

// TestDecode_LowConfidenceFailOpen: confiança abaixo do piso → text (o runtime
// não executa decisão de baixa confiança sem contexto).
func TestDecode_LowConfidenceFailOpen(t *testing.T) {
	p := Decode(`{"decision":"DELETE","action":"remove_everything","confidence":0.3}`)
	if p.Status != StatusText {
		t.Fatalf("status = %q, esperava text para confiança baixa (fail-open)", p.Status)
	}
}

// TestDecode_ProseWithJSONInside: prosa com um JSON parcial dentro → o decoder
// extrai o bloco se balanceado, senão text. Aqui o JSON é completo no meio.
func TestDecode_ProseWithJSONInside(t *testing.T) {
	raw := "Contexto: {\"decision\":\"SEARCH\",\"action\":\"find_endpoint\",\"confidence\":0.88} fim."
	p := Decode(raw)
	if p.Status != StatusStructured {
		t.Fatalf("status = %q, esperava structured (JSON no meio da prosa)", p.Status)
	}
	if p.Structured.Decision != "SEARCH" {
		t.Fatalf("decisão = %q, esperava SEARCH", p.Structured.Decision)
	}
}

// TestExtractJSON_Balanced: o extrator casa chaves balanceadas mesmo com
// strings aninhadas contendo chaves.
func TestExtractJSON_Balanced(t *testing.T) {
	raw := `antes {"a":"{nested}","b":{"c":1}} depois`
	got := extractJSON(raw)
	if got != `{"a":"{nested}","b":{"c":1}}` {
		t.Fatalf("extractJSON = %q", got)
	}
}
