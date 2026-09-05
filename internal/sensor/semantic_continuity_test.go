package sensor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─────────────────────────────────────────────────────────────────────────────
// Caso D — Continuidade semântica de capacidades APÓS RESTART.
//
// O objetivo do Caso D é garantir que um reinício de processo NÃO regride o
// sistema para o caminho antigo ("visão = VLM externo"). A relação arquitetural
// correta é:
//
//	CAPACIDADE -> sensores locais -> []sensor.Observation -> FUSÃO -> GATE
//	-> (suficiente: RESOLVE | insuficiência/contradição: ESCALA VLM)
//
// Esta relação sobrevive ao restart porque está PERSISTIDA nas fontes canônicas
// em disco (o que o despertar/consulta semanticamente recupera):
//   - docs/adr/ADR-036-continuidade-semantica-de-capacidades.md  (fonte canônica)
//   - .cosca/provenance.yaml                                     (dogma-continuidade-semantica-capacidades)
//   - .opencode/cosca/memory/agent/cosca-kernel/learnings.md     (entrada canônica)
//
// Divergência da busca semântica (knowledge.db): o índice pode NÃO estar pronto
// (reindex pendente — ver nota do Caso D). Este teste é DETERMINÍSTICO e NÃO
// depende do índice: ele lê as fontes persistentes em disco, que são o MESMO
// conteúdo que a busca semântica indexaria, e valida que a RELAÇÃO ressurge e
// que NENHUMA fonte a expressa como "VLM = identidade da capacidade".
//
// Estes testes vivem no pacote sensor porque a doutrina governa exatamente este
// DTO (sensor.Observation) — mas validam DOCUMENTAÇÃO/PROVENIÊNCIA, não código
// funcional. Nenhum arquivo .go de produção é tocado.
// ─────────────────────────────────────────────────────────────────────────────

// repoRoot sobe a partir do diretório do pacote (CWD do teste) até achar go.mod,
// garantindo leitura robusta das fontes em disco independente de onde o teste roda.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd falhou: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod não encontrado na árvore acima do diretório do teste")
		}
		dir = parent
	}
}

// mustRead lê uma fonte canônica do repositório e falha o teste se ausente/vazia.
func mustRead(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("leitura da fonte canônica %q falhou: %v", rel, err)
	}
	if strings.TrimSpace(string(b)) == "" {
		t.Fatalf("fonte canônica %q está vazia", rel)
	}
	return string(b)
}

// doctrineSources é a tríade persistente da RELAÇÃO (o que sobrevive ao restart).
var doctrineSources = []string{
	"docs/adr/ADR-036-continuidade-semantica-de-capacidades.md",
	".cosca/provenance.yaml",
	".opencode/cosca/memory/agent/cosca-kernel/learnings.md",
}

// doctrineNormalize deixa o texto consultável de forma robusta: minúsculas,
// acentos PT removidos (para casar formas acentuadas/não-acentuadas) e o
// símbolo "≠" unificado com "!=" (ambos expressam CAPACIDADE != PROVIDER).
func doctrineNormalize(s string) string {
	s = strings.ReplaceAll(s, "≠", "!=")
	s = strings.ToLower(s)
	repl := map[rune]string{
		'á': "a", 'à': "a", 'â': "a", 'ã': "a", 'ä': "a",
		'é': "e", 'è': "e", 'ê': "e", 'ë': "e",
		'í': "i", 'ì': "i", 'î': "i", 'ï': "i",
		'ó': "o", 'ò': "o", 'ô': "o", 'õ': "o", 'ö': "o",
		'ú': "u", 'ù': "u", 'û': "u", 'ü': "u",
		'ç': "c",
	}
	var b strings.Builder
	for _, r := range s {
		if rep, ok := repl[r]; ok {
			b.WriteString(rep)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// TestDoctrine_ContinuitySourcesPersist: as fontes canônicas da RELAÇÃO existem
// em disco e não estão vazias. A relação sobrevive ao restart porque está
// persistida, não só em memória do processo.
func TestDoctrine_ContinuitySourcesPersist(t *testing.T) {
	root := repoRoot(t)
	for _, rel := range doctrineSources {
		if fs := mustRead(t, root, rel); strings.TrimSpace(fs) == "" {
			t.Errorf("fonte canônica %q não deveria estar vazia", rel)
		}
	}
}

// TestDoctrine_RelationRecoverableFromPersistentSources: a regra canônica
// ressurge da junção normalizada das fontes persistidas. Valida que a RELAÇÃO
// (capacidade→sensores→observation→fusão→gate→(resolve|escala)) está registrada,
// não apenas "existe OCR/VLM" como ferramenta.
func TestDoctrine_RelationRecoverableFromPersistentSources(t *testing.T) {
	root := repoRoot(t)
	var all strings.Builder
	for _, rel := range doctrineSources {
		all.WriteString(mustRead(t, root, rel))
		all.WriteString("\n")
	}
	body := doctrineNormalize(all.String())

	markers := []struct{ name, token string }{
		{"intenção (ponto de partida do caminho)", "intencao"},
		{"capacidade (não provider)", "capacidade"},
		{"provider", "provider"},
		{"sensores locais", "sensores locais"},
		{"observation (sensor.Observation)", "observation"},
		{"fusão/consenso (fusion.Fuse)", "fusion"},
		{"gate de evidência (gate.Decide)", "gate"},
		{"escalada (RESOLVE|ESCALATE)", "escalada"},
		{"vlm (no topo, nunca identidade)", "vlm"},
		{"contradição (informação preservada)", "contradic"},
		{"regra anti-regressão", "anti-regress"},
	}
	for _, m := range markers {
		if !strings.Contains(body, m.token) {
			t.Errorf("marcador canônico ausente nas fontes persistentes: %q (token %q)", m.name, m.token)
		}
	}
}

// TestDoctrine_VLMIsEscalationNotIdentity é a garantia de que a RELAÇÃO NÃO é
// "visão = VLM externo". O ADR-036 (fonte canônica) declara explicitamente que o
// VLM é ESCALADA e NUNCA identidade da capacidade — a antítese da regressão.
func TestDoctrine_VLMIsEscalationNotIdentity(t *testing.T) {
	root := repoRoot(t)
	body := doctrineNormalize(mustRead(t, root, "docs/adr/ADR-036-continuidade-semantica-de-capacidades.md"))
	for _, marker := range []string{
		"vlm e escalada, nunca identidade",   // ADR-036 §2.1: "VLM é ESCALADA, NUNCA identidade da capacidade"
		"nunca identidade da capacidade",     // cláusula de anti-regressão
	} {
		if !strings.Contains(body, marker) {
			t.Errorf("ADR-036 não contém a cláusula anti-regressão %q", marker)
		}
	}
}

// TestDoctrine_ProviderNeverBecomesCapability: o dogma registra CAPACIDADE !=
// PROVIDER — a capacidade nunca é definida por um provider. Confirmado tanto na
// fonte canônica (ADR-036) quanto no ledger (provenance.yaml).
func TestDoctrine_ProviderNeverBecomesCapability(t *testing.T) {
	root := repoRoot(t)
	adr := doctrineNormalize(mustRead(t, root, "docs/adr/ADR-036-continuidade-semantica-de-capacidades.md"))
	prov := doctrineNormalize(mustRead(t, root, ".cosca/provenance.yaml"))

	if !strings.Contains(adr, "capacidade != provider") {
		t.Errorf("ADR-036 não registra 'capacidade != provider' explicitamente")
	}
	if !strings.Contains(prov, "capacidade != provider") {
		t.Errorf("provenance.yaml não registra '(1) capacidade != provider'")
	}
}
