// recipe.go — recipes de rebuild por classe de banco derivado.
//
// Cada classe de banco alvo tem UMA recipe canônica descoberta no código
// existente do Cosca (a regeneração real que o sistema usa — nunca inventada
// aqui) e é executada como re-execução do PRÓPRIO binário
// (`exec.Command(os.Executable(), args...)`, padrão já usado em
// internal/cli/despertar.go checkChain e internal/cli/start.go findCoscaBin):
//
//	classe      arquivo(s)                recipe canônica (comando real)
//	-------     ------------------------  -----------------------------------------
//	session     .cosca/session.db         cosca session index   (FTS5 das sessions)
//	index       .cosca/memory/index.db    cosca memory reindex  (FTS5 da memória)
//	vector      .cosca/vector-*.db        cosca db build        (módulos p/ knowledge.db)
//	knowledge   .cosca/knowledge.db       cosca knowledge compile + knowledge rebuild
//	                                       + knowledge vectors-backfill
//
// A classe `knowledge` NÃO tem recipe 100% offline/determinística: depende do
// corpus presente (.cosca/knowledge/acquired, fallback) e de um provider de
// embeddings para o backfill. Por isso o Repair registra a falha e devolve
// INSTRUÇÕES MANUAIS explícitas (nunca meio-repair — ADR-014 regra 4).
//
// INVARIANTE DE SEGURANÇA (ADR-014 regra 3; ADR-043 §8): chain
// (family_chain.dat), embed (internal/embed/cosca) e identidade/chaves NÃO
// são classes reparáveis — SupportedKinds não os contém e nenhuma recipe os
// referencia (verificado por teste).
package staterepair

import (
	"fmt"
	"strings"
)

// DBKind identifica uma classe de banco derivado/regenerável reparável.
type DBKind string

// Classes de banco reparáveis (escopo ADR-043 §8 / decisão do Don).
const (
	// KindSession é `.cosca/session.db` — índice FTS5 das conversas.
	KindSession DBKind = "session"
	// KindIndex é `.cosca/memory/index.db` — índice FTS5 da memória.
	KindIndex DBKind = "index"
	// KindKnowledge é `.cosca/knowledge.db` — o conhecimento (fonte do split).
	KindKnowledge DBKind = "knowledge"
	// KindVector é o conjunto `.cosca/vector-*.db` — módulos vetoriais.
	KindVector DBKind = "vector"
	// KindAll seleciona todas as classes reparáveis.
	KindAll DBKind = "all"
)

// SupportedKinds devolve as classes concretas reparáveis (SEM all). A lista
// NÃO contém chain/embed/identidade por construção — a fonte da verdade
// nunca tem caminho automático de repair.
func SupportedKinds() []DBKind {
	return []DBKind{KindSession, KindIndex, KindKnowledge, KindVector}
}

// String devolve o identificador da classe.
func (k DBKind) String() string { return string(k) }

// IsConcrete devolve true para uma classe única reparável (não "all").
func (k DBKind) IsConcrete() bool {
	for _, c := range SupportedKinds() {
		if c == k {
			return true
		}
	}
	return false
}

// ParseKind valida um seletor de classe vindo da CLI (`--db`). Aceita também
// "all". Vazio devolve KindAll (default da CLI).
func ParseKind(s string) (DBKind, error) {
	switch DBKind(strings.ToLower(strings.TrimSpace(s))) {
	case "":
		return KindAll, nil
	case KindAll:
		return KindAll, nil
	case KindSession:
		return KindSession, nil
	case KindIndex:
		return KindIndex, nil
	case KindKnowledge:
		return KindKnowledge, nil
	case KindVector:
		return KindVector, nil
	default:
		return "", fmt.Errorf("classe de banco inválida %q (use session|index|knowledge|vector|all)", s)
	}
}

// Step é um passo da recipe: re-execução do próprio binário com Args.
type Step struct {
	// Args são os argumentos após o executável (ex.: {"session", "index"}).
	Args []string
	// What descreve o passo em linguagem humana.
	What string
}

// Recipe é a regeneração canônica de uma classe de banco.
type Recipe struct {
	// Kind é a classe de banco.
	Kind DBKind
	// Label é um rótulo curto da recipe.
	Label string
	// Steps são os passos executados em ordem (binário atual + Args).
	Steps []Step
	// Offline indica se a recipe é determinística e offline (não depende de
	// provider de embeddings/corpus externo). false → Repair falha com
	// ManualInstructions quando a recipe não consegue regenerar.
	Offline bool
	// ManualInstructions é o texto exibido quando a recipe não é offline
	// determinística ou falha: comandos manuais explícitos, nunca meio-repair.
	ManualInstructions string
}

// commandString renderiza um passo como linha de comando para relatório.
func (s Step) commandString() string {
	return "cosca " + strings.Join(s.Args, " ")
}

// recipeFor devolve a recipe canônica da classe (descoberta no código do
// Cosca — ver cabeçalho do arquivo). KindAll não tem recipe própria.
func recipeFor(kind DBKind) (Recipe, error) {
	switch kind {
	case KindSession:
		return Recipe{
			Kind:    KindSession,
			Label:   "reindex de sessão (FTS5)",
			Offline: true,
			Steps: []Step{
				{Args: []string{"session", "index"}, What: "Reindexa .cosca/sessions/*.jsonl em .cosca/session.db (FTS5 dedicado)"},
			},
		}, nil

	case KindIndex:
		return Recipe{
			Kind:    KindIndex,
			Label:   "reindex de memória (FTS5)",
			Offline: true,
			Steps: []Step{
				{Args: []string{"memory", "reindex"}, What: "Reconstrói o índice FTS5 da memória (memory/index.db) a partir dos arquivos .md em disco"},
			},
		}, nil

	case KindVector:
		return Recipe{
			Kind:    KindVector,
			Label:   "db build (módulos físicos do split)",
			Offline: true,
			Steps: []Step{
				{Args: []string{"db", "build"}, What: "Reconstrói os módulos físicos derivados (vector-*.db, core/graph/projects/fts) a partir do knowledge.db, que permanece intacto (ADR-013 Fase C)"},
			},
		}, nil

	case KindKnowledge:
		return Recipe{
			Kind:    KindKnowledge,
			Label:   "pipeline de conhecimento (compile + rebuild + backfill)",
			Offline: false,
			Steps: []Step{
				{Args: []string{"knowledge", "compile"}, What: "Compila o conhecimento do repositório (.cosca/fallback/knowledge) no knowledge.db"},
				{Args: []string{"knowledge", "rebuild"}, What: "Reconstrói o Knowledge Base (FTS5) a partir dos documentos adquiridos (knowledge/acquired)"},
				{Args: []string{"knowledge", "vectors-backfill"}, What: "Embebe os chunks sem vetor (idempotente) — requer provider de embeddings configurado"},
			},
			ManualInstructions: "knowledge.db é a fonte do split (ADR-013) e sua regeneração NÃO é 100% offline: depende do corpus presente e do provider de embeddings. Para recuperar manualmente, rode em ordem: (1) cosca knowledge compile; (2) cosca knowledge rebuild; (3) cosca knowledge index <dirs-do-corpus> para cada fonte do projeto; (4) cosca knowledge vectors-backfill; (5) cosca knowledge index-entities. Confira antes: cosca doctor (provider de embeddings) e que o .cosca/knowledge/acquired não foi perdido. O arquivo doente está preservado em quarentena + backup forense (não foi destruído).",
		}, nil

	default:
		return Recipe{}, fmt.Errorf("classe %q não tem recipe de rebuild", kind)
	}
}
