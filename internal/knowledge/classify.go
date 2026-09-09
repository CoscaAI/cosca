package knowledge

import (
	"regexp"
	"strings"
)

// ClassifiedDoc é o veredito determinístico (sem LLM) do classificador para um
// candidato a documento. Persistent=false significa que o documento NÃO deve
// ser indexado — "não conseguir classificar ≠ conhecimento" (fail-closed).
type ClassifiedDoc struct {
	Persistent bool    `json:"persistent"`
	Kind       string  `json:"kind"`
	Scope      string  `json:"scope"`
	Reason     string  `json:"reason"`
	Confidence float64 `json:"confidence"`
}

// Escopos. scope = jurisdição semântica; path = proveniência física.
const (
	// ScopeGlobal é reservado ao cérebro embarcado (internal/embed/cosca/**).
	// É o ÚNICO caminho determinístico a scope=global nesta fase — nunca um
	// conteúdo de origem de projeto (regra de ouro: NUNCA auto-promover
	// PROJECT→GLOBAL).
	ScopeGlobal = "global"
	// ScopeProject é o escopo de TODO conteúdo de origem de projeto
	// (.cosca/memory/agent/**, .cosca/fallback/memory/agent/**,
	// docs/**). Default de qualquer path não-embarcado.
	ScopeProject = "project"
)

// Kinds aceitos no conhecimento persistente.
const (
	KindLearning     = "learning"
	KindDecision     = "decision"
	KindPattern      = "pattern"
	KindRule         = "rule"
	KindADR          = "adr"
	KindTechnique    = "technique"
	KindArchitecture = "architecture"
)

// minClassifyConfidence é o limiar de confiança. Abaixo dele o candidato é
// descartado (fail-closed): não indexar o que não foi classificado com
// confiança.
const minClassifyConfidence = 0.5

// transientPathTokens são componentes de path que marcam artefato transitório
// NÃO persistente: sessões, logs, traces, dumps, backups, testes/gold-test,
// fixtures, temporários.
var transientPathTokens = []string{
	"session", "sessions",
	"log", "logs",
	"trace", "traces",
	"dump", "dumps",
	"backup", "backups",
	"tmp", "temp",
	"testdata", "fixtures", "gold-test", "golden", "snapshots",
}

// kindPathSegments mapeiam componentes de path a um kind. A ordem importa
// apenas para a prioridade de detecção (a primeira match vence).
var kindPathSegments = []struct {
	seg  string
	kind string
}{
	{"learnings", KindLearning},
	{"learning", KindLearning},
	{"lessons", KindLearning},
	{"patterns", KindPattern},
	{"pattern", KindPattern},
	{"failures", KindTechnique},
	{"adr", KindADR},
	{"adrs", KindADR},
	{"architecture", KindArchitecture},
	{"decisions", KindDecision},
	{"decision", KindDecision},
	{"rules", KindRule},
	{"rule", KindRule},
	{"techniques", KindTechnique},
}

// positiveTypeKinds mapeiam o frontmatter type a um kind persistente.
var positiveTypeKinds = map[string]string{
	"learning":     KindLearning,
	"learned":      KindLearning,
	"lesson":       KindLearning,
	"decision":     KindDecision,
	"pattern":      KindPattern,
	"rule":         KindRule,
	"adr":          KindADR,
	"architecture": KindArchitecture,
	"technique":    KindTechnique,
}

// negativeTypes são frontmatter types que marcam artefato transitório.
var negativeTypes = map[string]bool{
	"session":   true,
	"log":       true,
	"trace":     true,
	"test":      true,
	"fixture":   true,
	"dump":      true,
	"gold-test": true,
	"transient": true,
}

// contentKindMarkers detectam estruturas de conteúdo consolidadas. A ordem
// importa: marcadores de learning block vêm primeiro.
var contentKindMarkers = []struct {
	re   *regexp.Regexp
	kind string
}{
	// Learning block (formato register.go: buildBlock).
	{regexp.MustCompile(`(?m)^## L\d+ \|`), KindLearning},
	{regexp.MustCompile(`\*\*Learned\*\*`), KindLearning},
	{regexp.MustCompile(`\*\*Outcome\*\*`), KindLearning},
	{regexp.MustCompile(`\*\*Technique\*\*`), KindLearning},
	// ADR / decisão.
	{regexp.MustCompile(`(?mi)^#+ (ADR|Decision)\b`), KindDecision},
	{regexp.MustCompile(`(?mi)\*\*Status\*\*[:=]?\s*(accepted|approved|proposed|rejected|accepted)`), KindDecision},
	{regexp.MustCompile(`(?m)^#+ (Context|Consequence|Consequences)\b`), KindDecision},
	// Padrão.
	{regexp.MustCompile(`(?mi)^#+ Pattern\b`), KindPattern},
	{regexp.MustCompile(`(?mi)\*\*Applicability\*\*`), KindPattern},
	// Regra.
	{regexp.MustCompile(`(?mi)^#+ Rule\b`), KindRule},
	// Arquitetura.
	{regexp.MustCompile(`(?mi)^#+ Architecture\b`), KindArchitecture},
	{regexp.MustCompile(`(?mi)\*\*Architecture\*\*`), KindArchitecture},
	// Técnica (heading, não a forma de learning block).
	{regexp.MustCompile(`(?mi)^#+ Technique\b`), KindTechnique},
}

var (
	timestampLineRe = regexp.MustCompile(`(?m)^\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}`)
	testOutputRe    = regexp.MustCompile(`(?m)^(=== RUN|--- PASS|--- FAIL|ok\s+\S+|FAIL\s*$|panic:|goroutine\s+\d+\s+\[running\]|runtime error:)`)
	ansiEscapeRe    = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	logPrefixRe     = regexp.MustCompile(`(?m)^\[(INFO|DEBUG|WARN|ERROR|TRACE)\]`)
)

// ClassifyDoc é a função pura do classificador determinístico. Sem LLM. Decide,
// por path + frontmatter type + heurística de conteúdo, se o documento é
// conhecimento persistente e com qual kind/scope. Fail-closed: se não
// classificar com confiança >= minClassifyConfidence, Persistent=false.
//
// Nomeada ClassifyDoc para não colidir com Classify (thinking.go), que tem
// outra assinatura no mesmo pacote.
func ClassifyDoc(path string, frontmatter map[string]any, content string, agent string) ClassifiedDoc {
	scope := classifyScope(path)

	// 1. Artefatos transitórios por path — excluídos SEMPRE.
	for _, tok := range transientPathTokens {
		if pathContainsToken(path, tok) {
			return ClassifiedDoc{Persistent: false, Scope: scope, Reason: "transient path segment: " + tok, Confidence: 0.2}
		}
	}

	// 2. Frontmatter type transitório — excluído SEMPRE.
	if t, ok := frontmatter["type"].(string); ok {
		tl := strings.ToLower(strings.TrimSpace(t))
		if negativeTypes[tl] {
			return ClassifiedDoc{Persistent: false, Scope: scope, Kind: tl, Reason: "transient frontmatter type: " + t, Confidence: 0.2}
		}
	}

	// 3. Conteúdo explicitamente transitório (log/test output).
	if isTransientContent(content) {
		return ClassifiedDoc{Persistent: false, Scope: scope, Reason: "transient content (log/test)", Confidence: 0.2}
	}

	// 4. Determinação de kind + confiança.
	kind, confidence := classifyKind(path, frontmatter, content)

	// 5. Fail-closed: sem kind OU confiança abaixo do limiar → não indexar.
	if kind == "" || confidence < minClassifyConfidence {
		return ClassifiedDoc{Persistent: false, Scope: scope, Kind: kind, Reason: "falha em classificar (confiança < 0.5)", Confidence: confidence}
	}

	return ClassifiedDoc{Persistent: true, Kind: kind, Scope: scope, Reason: "classificado", Confidence: confidence}
}

// classifyScope define o escopo. scope=global é reservado ao cérebro
// embarcado — tanto em `internal/embed/cosca/**` (a fonte) quanto em
// `.cosca/fallback/**` (a cópia materializada do mesmo cérebro via
// MaterializeFallback). TUDO mais (conteúdo único de projeto: docs/**,
// .cosca/memory/agent/**) → scope=project.
//
// Isso preserva a regra de ouro (origem de projeto NUNCA vira scope=global) E
// evita poluir o escopo do projeto com conteúdo de framework: a cópia fallback
// é o MESMO conteúdo do embed, então só faria sentido tratá-la como
// framework/global — nunca como conhecimento único do projeto.
func classifyScope(path string) string {
	norm := slashPath(path)
	if strings.HasPrefix(norm, "internal/embed/cosca/") || strings.Contains(norm, "/internal/embed/cosca/") ||
		strings.HasPrefix(norm, ".cosca/fallback/") || strings.Contains(norm, "/.cosca/fallback/") {
		return ScopeGlobal
	}
	return ScopeProject
}

// classifyKind determina o kind persistente e a confiança, na ordem de
// autoridade: frontmatter type > path > conteúdo.
func classifyKind(path string, fm map[string]any, content string) (string, float64) {
	if t, ok := fm["type"].(string); ok {
		tl := strings.ToLower(strings.TrimSpace(t))
		if k, ok := positiveTypeKinds[tl]; ok {
			return k, 0.9
		}
	}
	if k, ok := kindFromPath(path); ok {
		return k, 0.85
	}
	if k, ok := kindFromContent(content); ok {
		return k, 0.8
	}
	return "", 0.3
}

func kindFromPath(p string) (string, bool) {
	for _, kps := range kindPathSegments {
		if pathContainsToken(p, kps.seg) {
			return kps.kind, true
		}
	}
	return "", false
}

func kindFromContent(content string) (string, bool) {
	for _, mk := range contentKindMarkers {
		if mk.re.MatchString(content) {
			return mk.kind, true
		}
	}
	return "", false
}

func isTransientContent(content string) bool {
	if len(timestampLineRe.FindAllString(content, -1)) >= 3 {
		return true
	}
	if testOutputRe.MatchString(content) {
		return true
	}
	if ansiEscapeRe.MatchString(content) {
		return true
	}
	if logPrefixRe.MatchString(content) {
		return true
	}
	return false
}

// pathContainsToken verifica se path possui um componente igual a token (ou um
// arquivo "token.ext") — matching por componente, não por substring, para não
// confundir "log" com "catalogo".
func pathContainsToken(p, token string) bool {
	norm := slashPath(p)
	for _, seg := range strings.Split(norm, "/") {
		if seg == token {
			return true
		}
		if strings.HasPrefix(seg, token+".") {
			return true
		}
	}
	return false
}

// slashPath normaliza separadores de path para "/", tornando o matching
// determinístico entre sistemas (Windows usa "\").
func slashPath(p string) string {
	return strings.ReplaceAll(p, "\\", "/")
}
