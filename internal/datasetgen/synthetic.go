package datasetgen

import (
	"fmt"
	"strings"
)

// ─── Gerador procedural de tarefas ───────────────────────────────────────────
//
// Este gerador produz tarefas DETERMINÍSTICAS (sem LLM) cobrindo diversidade de
// linguagens e focos. É o backbone garantido: o dataset é produzido mesmo sem o
// professor. Quando o professor (DeepSeek-V4) estiver disponível, o método
// GenerateFromProfessor é usado para produzir tarefas mais ricas; este gerador
// cobre o piso de qualidade e a diversidade de superfície.

// TaskSpec é a tripla gerada: {tarefa, estado inicial, resultado esperado}.
type TaskSpec struct {
	Task          string            // instrução ao agente
	InitialState  map[string]string // path → conteúdo (estado inicial do workspace)
	ExpectedState map[string]string // path → substring esperada (verificação)
	Language      string            // linguagem do artefato
	Focus         Focus             // categoria de comportamento
}

// synthDirStrategy é a assinatura de uma função sintetizadora.
type synthDirStrategy func() TaskSpec

// languageNames mapeia linguagem → extensão e uma "moldura" de código base.
// Varried de propósito para o modelo não aprender "Restart() → read_file" como
// uma regra superficial.
type languageKind struct {
	ext       string
	base      func() string
	symbol    string       // símbolo a alterar (nome de função/atributo)
	newBody   string       // novo corpo/assinatura a injetar
}

var languageKinds = map[string]languageKind{
	"go": {
		ext: ".go",
		base: func() string { return "package main\n\nfunc Restart() {\n\t// old\n}\n" },
		symbol: "func Restart() {",
		newBody: "func Restart() error {\n\t// added state handling\n}",
	},
	"python": {
		ext: ".py",
		base: func() string { return "def restart():\n    # old\n    pass\n" },
		symbol: "def restart():",
		newBody: "def restart() -> bool:\n    # added state handling\n    return True",
	},
	"typescript": {
		ext: ".ts",
		base: func() string { return "export function restart() {\n  // old\n}\n" },
		symbol: "export function restart() {",
		newBody: "export function restart(): boolean {\n  // added state handling\n  return true;\n}",
	},
	"json": {
		ext: ".json",
		base: func() string { return "{\n  \"restart\": {\n    \"mode\": \"cold\"\n  }\n}\n" },
		symbol: `"mode": "cold"`,
		newBody: `"mode": "warm"`,
	},
	"yaml": {
		ext: ".yaml",
		base: func() string { return "restart:\n  mode: cold\n" },
		symbol: "mode: cold",
		newBody: "mode: warm",
	},
	"markdown": {
		ext: ".md",
		base: func() string { return "# Restart\n\nOld behavior.\n" },
		symbol: "Old behavior.",
		newBody: "New behavior (proper state handling).",
	},
}

// generatorPool é o conjunte de estratégias de síntese, uma por foco.
func generatorPool() []synthDirStrategy {
	var out []synthDirStrategy
	for _, lang := range []string{"go", "python", "typescript", "json", "yaml", "markdown"} {
		l := lang
		out = append(out, func() TaskSpec { return happyPathSpec(l) })
		out = append(out, func() TaskSpec { return recoverySpec(l) })
		out = append(out, func() TaskSpec { return requestInfoSpec(l) })
		out = append(out, func() TaskSpec { return searchFirstSpec() })
	}
	return out
}

// happyPathSpec: read → edit → verify → DONE (caminho limpo).
func happyPathSpec(lang string) TaskSpec {
	lk := languageKinds[lang]
	file := "main" + lk.ext
	return TaskSpec{
		Task:          fmt.Sprintf("Fix %s in %s to add proper state handling.", lk.symbol, file),
		InitialState:  map[string]string{file: lk.base()},
		ExpectedState: map[string]string{file: lk.newBody},
		Language:      lang,
		Focus:         FocusHappyPath,
	}
}

// recoverySpec: estado que exige um erro (old_string inventado mal-formado) e
// a recuperação correta — o modelo deve ler, errar, re-ler e corrigir.
func recoverySpec(lang string) TaskSpec {
	lk := languageKinds[lang]
	file := "main" + lk.ext
	// Estado inicial com uma função cujo corpo NÃO bate com o símbolo óbvio,
	// forçando o agente a ler com cuidado e recuperar de um erro.
	return TaskSpec{
		Task:          fmt.Sprintf("Replace state handling in %s. First read the file, then edit it.", file),
		InitialState:  map[string]string{file: "package main\n\nfunc Restart(force bool) {\n\t// force mode\n}\n"},
		ExpectedState: map[string]string{file: "force mode"},
		Language:      lang,
		Focus:         FocusRecovery,
	}
}

// requestInfoSpec: evidência insuficiente — a resposta correta é NÃO agir sem
// mais informação (ler/buscar/escalar), nunca inventar a edição.
func requestInfoSpec(lang string) TaskSpec {
	lk := languageKinds[lang]
	file := "main" + lk.ext
	return TaskSpec{
		Task:          fmt.Sprintf("Update %s: I'm not sure where to place the new handler. Find the right file first.", file),
		InitialState:  map[string]string{file: lk.base()},
		ExpectedState: map[string]string{},
		Language:      lang,
		Focus:         FocusRequestInfo,
	}
}

// searchFirstSpec: mudança que exige buscar/inventariar antes de editar
// (o arquivo a mudar está espalhado / precisa de glob).
func searchFirstSpec() TaskSpec {
	return TaskSpec{
		Task:          "Refactor: find all files defining 'restart' across the repo, then update each to add a 'version' field.",
		InitialState: map[string]string{
			"cmd/alpha.json": `{ "restart": { "mode": "cold" } }`,
			"cmd/beta.yaml":  "restart:\n  mode: cold\n",
			"pkg/main.go":    "package main\n\nfunc Restart() {\n\t// old\n}\n",
		},
		ExpectedState: map[string]string{},
		Language:      "multi",
		Focus:         FocusSearchFirst,
	}
}

// GenerateBatch produz N tarefas determinísticas com distribuição de focos.
// A distribuição é construída para aproximar o peso-alvo do SPEC (mais
// happy_path, bastante recovery).
func GenerateBatch(n int, seed int64) []TaskSpec {
	strategies := generatorPool()
	// Distribuição ponderada (espelha o SPEC §5).
	//
	// Para cada estratégia, geramos em proporção:
	//   happy_path   ~40%
	//   recovery     ~25%
	//   request_info ~15%
	//   search_first ~10%
	//   multi_file   ~10%
	//
	// Como o pool tem estratégias em ordem [happy, recovery, request, search]
	// por linguagem, usamos um índice ponderado simples.
	weighted := make([]TaskSpec, 0, n)
	for i := 0; i < n; i++ {
		idx := weightedPick(i, n)
		weighted = append(weighted, strategies[idx%len(strategies)]())
	}
	return weighted
}

// weightedPick devolve um índice de estratégia de acordo com o peso do foco.
// round-robin ponderado determinístico (não usa rand para nunca variar entre
// execuções — dados de treino devem ser reproduzíveis).
func weightedPick(i, n int) int {
	// Janela de 10: [0-3]=happy, [4-6]=recovery, [7-8]=request, [9]=search.
	// Isso aproxima 40/30/20/10.
	slot := i % 10
	switch {
	case slot < 4:
		return slot * 2 // happy
	case slot < 7:
		return slot*2 + 1 // recovery
	case slot < 9:
		return slot * 2 // request (índice par = request)
	default:
		return 7 // search (último índice)
	}
}

// seedStr é usado pelo gerador do professor para variar; aqui mantido para
// compatibilidade de assinatura.
func seedStr(seed int64) string {
	return fmt.Sprintf("seed-%d", seed)
}

// SummarizeBatch descreve as labels/idiomas de um lote (para relatório).
func SummarizeBatch(specs []TaskSpec) string {
	var sb strings.Builder
	byFocus := map[Focus]int{}
	for _, s := range specs {
		byFocus[s.Focus]++
	}
	sb.WriteString("lot stats: ")
	for _, f := range []Focus{FocusHappyPath, FocusRecovery, FocusRequestInfo, FocusSearchFirst, FocusMultiFile} {
		sb.WriteString(fmt.Sprintf("%s=%d ", f, byFocus[f]))
	}
	return sb.String()
}
