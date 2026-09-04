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
	StateOptions  map[string][]string // path → lista de formas válidas (Golden v2)
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

// GenerateBatch produz N tarefas determinísticas com DISTRIBUIÇÃO EQUILIBRADA
// de linguagens e foco ponderado (mais recovery — o alvo da campanha 002).
//
// LIÇÃO DA CAMPANHA 001: a distribuição anterior não garantia que cada
// linguagem aparecesse em cada foco. O `py-happy-simple` regrediu porque o
// dataset de treino tinha poucos casos Python. Este gerador GARANTE cobertura
// de todas as 6 linguagens em cada foco (round-robin determinístico).
//
// Distribuição de focos (peso-alvo do SPEC §5, ajustado p/ campanha 002):
//   recovery     ~35%  <- alvo da 002 (0.12 no baseline é o gargalo)
//   happy_path   ~30%
//   request_info ~20%
//   search_first ~15%
func GenerateBatch(n int, seed int64) []TaskSpec {
	languages := []string{"go", "python", "typescript", "json", "yaml", "markdown"}
	// Estratégias por foco, cada uma cobrindo TODAS as linguagens (round-robin).
	// Foco -> função que gera um spec para a linguagem dada.
	focusGens := []struct {
		focus Focus
		gen   func(lang string) TaskSpec
		weight int
	}{
		{FocusRecovery, recoverySpec, 35},
		{FocusHappyPath, happyPathSpec, 30},
		{FocusRequestInfo, requestInfoSpec, 20},
		{FocusSearchFirst, searchCoversLangs, 15},
	}

	// Preenche a lista ponderada: para cada foco, gera `weight` exemplos
	// distribuindo as linguagens em round-robin (todas aparecem).
	var weighted []TaskSpec
	for _, fg := range focusGens {
		for j := 0; j < fg.weight; j++ {
			lang := languages[j%len(languages)]
			weighted = append(weighted, fg.gen(lang))
		}
	}

	// Trunca/repete até n (ciclo determinístico dos pesos).
	out := make([]TaskSpec, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, weighted[i%len(weighted)])
	}
	_ = seed // mantido p/ compatibilidade de assinatura; geração é determinística
	return out
}

// searchCoversLangs faz o caso search_first cobrir também uma linguagem
// (além do cenário multi-arquivo), para não restringir o search a uma única
// forma. Usa a linguagem dada para compor o cenário.
func searchCoversLangs(lang string) TaskSpec {
	lk, ok := languageKinds[lang]
	if !ok {
		// fallback para cenário multi default
		return searchFirstSpec()
	}
	return TaskSpec{
		Task:          fmt.Sprintf("Refactor: find all %s files defining the restart symbol, then update each to add a version field. Inspect before editing.", lk.ext),
		InitialState:  map[string]string{"main" + lk.ext: lk.base(), "pkg/other" + lk.ext: lk.base()},
		ExpectedState: map[string]string{},
		Language:      lang,
		Focus:         FocusSearchFirst,
	}
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
