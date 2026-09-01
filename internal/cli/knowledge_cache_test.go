package cli

import (
	"testing"
)

// reflectCacheLen counts the entries in a cache (accessor for the test).
func reflectCacheLen(c *knowledgeCache) int {
	if c == nil {
		return 0
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// ──────────────────────────────────────────────────────────────
// Lookup — cache vazio / query vazia (degradação graciosa)
// ──────────────────────────────────────────────────────────────

// TestKnowledgeCacheLookupEmpty verifica (a): um cache vazio (ou sem dir)
// NUNCA retorna um hit — Lookup devolve ("", false, 0).
func TestKnowledgeCacheLookupEmpty(t *testing.T) {
	// Sem dir (dir vazio) → cache vazio, degrada gracioso.
	c := newKnowledgeCache("")
	if c == nil {
		t.Fatal("newKnowledgeCache(\"\") should not return nil")
	}
	if got := reflectCacheLen(c); got != 0 {
		t.Fatalf("empty cache should have 0 entries, got %d", got)
	}
	// Query não-vazia num cache vazio.
	if ans, ok, score := c.Lookup("o que você vê na tela"); ok || ans != "" || score != 0 {
		t.Fatalf("empty cache Lookup = (%q, %v, %v), want (\"\", false, 0)", ans, ok, score)
	}
	// Query vazia.
	if ans, ok, score := c.Lookup("   "); ok || ans != "" || score != 0 {
		t.Fatalf("blank query Lookup = (%q, %v, %v), want (\"\", false, 0)", ans, ok, score)
	}

	// Dir existente mas sem arquivo → cache vazio também.
	c2 := newKnowledgeCache(t.TempDir())
	if ans, ok, _ := c2.Lookup("oi"); ok || ans != "" {
		t.Fatalf("no-file cache Lookup should miss, got ok=%v ans=%q", ok, ans)
	}
}

// ──────────────────────────────────────────────────────────────
// Lookup — grava e acha (com variação de acentos do STT)
// ──────────────────────────────────────────────────────────────

// TestKnowledgeCacheLookupAnswersAfterStore verifica (b): após gravar
// "o que você vê na tela" → resposta X, um Lookup da MESMA pergunta (com a
// variação sem acentos típica do STT "o que voce ve na tela") retorna X com
// score > 0.5.
func TestKnowledgeCacheLookupAnswersAfterStore(t *testing.T) {
	c := newKnowledgeCache("")
	c.Store("o que você vê na tela", "Vejo uma janela e uma mesa.")

	// Variação de acentos (STT nem sempre acentua) — mesma frase normalizada.
	ans, ok, score := c.Lookup("o que voce ve na tela")
	if !ok {
		t.Fatalf("Lookup of variation should be a hit, got miss (score=%v)", score)
	}
	if ans != "Vejo uma janela e uma mesa." {
		t.Fatalf("Lookup answer = %q, want the stored answer", ans)
	}
	if score <= 0.5 {
		t.Fatalf("Lookup score = %v, want > 0.5", score)
	}
}

// TestKnowledgeCacheLookupDifferentQueryIsMiss verifica (c): uma pergunta bem
// diferente (sem palavras significativas em comum) NÃO deve casar — mesmo com
// entrada no cache, Lookup devolve ("", false, score<threshold).
func TestKnowledgeCacheLookupDifferentQueryIsMiss(t *testing.T) {
	c := newKnowledgeCache("")
	c.Store("o que você vê na tela", "Vejo uma janela e uma mesa.")

	ans, ok, _ := c.Lookup("qual e a capital do brasil")
	if ok || ans != "" {
		t.Fatalf("different query should be a miss, got ok=%v ans=%q", ok, ans)
	}
}

// TestKnowledgeCacheLookupIdenticalScoreOne verifica (d): um Lookup com o mesmo
// texto normalizado que a entrada guardada retorna score === 1.0.
func TestKnowledgeCacheLookupIdenticalScoreOne(t *testing.T) {
	c := newKnowledgeCache("")
	c.Store("o que você vê na tela", "Resposta X.")

	_, ok, score := c.Lookup("o que voce ve na tela") // normalizado == guardado
	if !ok {
		t.Fatalf("identical normalized query should hit, got miss")
	}
	if score != 1.0 {
		t.Fatalf("identical normalized query score = %v, want 1.0", score)
	}
}

// ──────────────────────────────────────────────────────────────
// Store — upsert/sobrescreve a resposta aprendida
// ──────────────────────────────────────────────────────────────

// TestKnowledgeCacheStoreOverwrites verifica (e): gravar a MESMA pergunta
// (variação de acentos) substitui a resposta antiga (upsert), não duplica a
// entrada.
func TestKnowledgeCacheStoreOverwrites(t *testing.T) {
	c := newKnowledgeCache("")
	if got := reflectCacheLen(c); got != 0 {
		t.Fatalf("fresh cache should be empty, got %d entries", got)
	}

	c.Store("o que você vê na tela", "Resposta A.")
	c.Store("o que voce ve na tela", "Resposta B.") // mesma query normalizada

	if got := reflectCacheLen(c); got != 1 {
		t.Fatalf("upsert should keep 1 entry, got %d", got)
	}
	ans, ok, _ := c.Lookup("o que voce ve na tela")
	if !ok || ans != "Resposta B." {
		t.Fatalf("after overwrite Lookup = (%q, %v), want (\"Resposta B.\", true)", ans, ok)
	}
}

// ──────────────────────────────────────────────────────────────
// Persistência — o cache sobrevive a um reload do mesmo dir
// ──────────────────────────────────────────────────────────────

// TestKnowledgeCachePersistsAcrossReload verifica (f): gravar num dir temp e
// recriar o cache do MESMO dir ainda encontra a resposta (persistência em disco).
func TestKnowledgeCachePersistsAcrossReload(t *testing.T) {
	dir := t.TempDir()

	c := newKnowledgeCache(dir)
	c.Store("o que você vê na tela", "Vejo uma janela e uma mesa.")

	// Recria o cache do mesmo dir (lê knowledge.json).
	c2 := newKnowledgeCache(dir)
	if got := reflectCacheLen(c2); got != 1 {
		t.Fatalf("reloaded cache should have 1 entry, got %d", got)
	}
	ans, ok, score := c2.Lookup("o que voce ve na tela")
	if !ok || ans != "Vejo uma janela e uma mesa." {
		t.Fatalf("reloaded cache Lookup = (%q, %v), want the persisted answer", ans, ok)
	}
	if score != 1.0 {
		t.Fatalf("reloaded identical query score = %v, want 1.0", score)
	}
}

// TestKnowledgeCachePersistsUpsertAcrossReload garante que a sobrescrita
// (upsert) também persiste entre reloads.
func TestKnowledgeCachePersistsUpsertAcrossReload(t *testing.T) {
	dir := t.TempDir()

	c := newKnowledgeCache(dir)
	c.Store("o que você vê na tela", "Resposta A.")
	c.Store("o que voce ve na tela", "Resposta B.")

	c2 := newKnowledgeCache(dir)
	if got := reflectCacheLen(c2); got != 1 {
		t.Fatalf("reloaded cache should keep 1 (upsert), got %d", got)
	}
	ans, ok, _ := c2.Lookup("o que voce ve na tela")
	if !ok || ans != "Resposta B." {
		t.Fatalf("reloaded upsert Lookup = (%q, %v), want (\"Resposta B.\", true)", ans, ok)
	}
}
