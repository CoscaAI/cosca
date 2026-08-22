package orchestration

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/rs/zerolog/log"
)

// wordBoundaryKeywordLen is the maximum keyword length matched as a whole
// word instead of a substring. Short keywords like "api" would otherwise
// match inside unrelated longer words ("capital" contains "api"), which sent
// generic Portuguese queries such as "qual é a capital do brasil" to the
// Backend team. Longer keywords are specific enough that substring matching
// stays useful for plurals and inflections ("teste" matches "testes").
const wordBoundaryKeywordLen = 4

// ─── Keyword → Agent Mapping ────────────────────────────────────────────────

// keywordAgentEntry maps a set of prompt keywords to one or more candidate
// agent names, ordered by preference (specialist first, then chief).
type keywordAgentEntry struct {
	keywords []string
	agents   []string // ordered by preference: specialist, chief, etc.
}

// greetingKeywords are the phrases routed to the Kernel Agent — the family's
// front door — for greetings and small talk. Shared by the keyword routing
// entry and the defensive isGreeting guard so both stay in sync.
var greetingKeywords = []string{"oi", "olá", "ola", "bom dia", "boa tarde", "boa noite", "hello", "hi", "hey", "e aí", "e ai", "tudo bem", "como vai"}

// keywordAgentMap defines the keyword-to-agent routing table.
// Order matters: entries listed first have higher priority when multiple
// keyword groups match.
var keywordAgentMap = []keywordAgentEntry{
	// Greetings / small talk route to the Kernel (the family's front door).
	// Listed first so they take priority over any specialist keyword that
	// happens to co-occur in the greeting.
	{keywords: greetingKeywords, agents: []string{"COSCA KERNEL"}},
	{keywords: []string{"api", "apis", "endpoint", "rest", "graphql", "rota", "rotas", "serviço", "servico", "serviços", "endpoints", "microserviço", "implementar api"}, agents: []string{"Backend API Specialist", "Backend Chief"}},
	{keywords: []string{"component", "ui", "react", "vue", "frontend", "interface", "componente", "tela", "telas", "página", "front", "componentes", "páginas", "paginas"}, agents: []string{"Frontend Component Specialist", "Frontend Chief"}},
	{keywords: []string{"database", "sql", "schema", "migration", "query", "banco", "banco de dados", "migração", "consulta", "tabela"}, agents: []string{"SQL Database Specialist", "Database Chief"}},
	{keywords: []string{"unit test", "integration test", "e2e test", "test", "teste", "testes", "testar", "testando", "cobertura", "unitário", "integração", "testes unitários", "testes de integração", "cobertura de testes"}, agents: []string{"Unit Test Specialist", "Testing Chief"}},
	{keywords: []string{"security", "vulnerability", "auth", "authentication", "authorization", "segurança", "vulnerabilidade", "auditoria", "audite", "autenticação", "autorização", "criptografia", "senha", "invasão", "exploit", "owasp", "pentest", "firewall", "certificado", "tls", "privacidade", "dados sensíveis", "dados sensiveis"}, agents: []string{"Security Chief"}},
	{keywords: []string{"architecture", "design pattern", "adr", "architectural", "design", "arquitetura", "padrão", "padrões"}, agents: []string{"Architecture Chief"}},
	{keywords: []string{"deploy", "ci/cd", "docker", "kubernetes", "pipeline", "implantação", "infra", "ci", "cd", "build", "compilar", "contêiner", "container"}, agents: []string{"DevOps Chief"}},
	{keywords: []string{"readme", "documentation", "docs", "api docs", "documentação", "manuais", "guia", "escrever documentação", "escrever doc"}, agents: []string{"Documentation Chief", "Technical Writer"}},
	{keywords: []string{"code review", "review", "revisão", "revisar", "código", "revisão de código", "revisao de codigo", "auditar código", "auditar codigo"}, agents: []string{"Code Reviewer", "Review Chief"}},
	{keywords: []string{"monitoring", "observability", "alerting", "slo", "monitoramento", "observabilidade", "alerta", "alertas", "logs", "log", "rastreamento", "tracing"}, agents: []string{"Monitoring Chief"}},
	{keywords: []string{"analytics", "dashboard", "metrics", "data analysis", "métrica", "métricas", "análise", "dados", "relatório", "relatorio", "insights"}, agents: []string{"Analytics Chief"}},
	{keywords: []string{"ai", "ml", "machine learning", "rag", "embedding", "llm", "ia", "inteligência artificial", "modelo"}, agents: []string{"AI Chief"}},
	{keywords: []string{"mobile", "ios", "android", "react native", "flutter"}, agents: []string{"Mobile Chief"}},
	{keywords: []string{"release", "version", "changelog", "semver", "versão", "versões"}, agents: []string{"Release Chief"}},
	{keywords: []string{"infrastructure", "cloud", "networking", "scaling", "infraestrutura", "nuvem", "rede", "redes", "escala"}, agents: []string{"Infrastructure Chief"}},
	{keywords: []string{"product", "requirement", "backlog", "user story", "produto", "requisito", "requisitos", "história"}, agents: []string{"Product Chief"}},
	{keywords: []string{"workflow", "automation", "automate", "script", "cli", "generator", "automação", "automatizar"}, agents: []string{"Workflow Chief", "Automation Chief"}},
	{keywords: []string{"context", "environment", "session", "contexto", "ambiente", "sessão", "sessões"}, agents: []string{"Context Chief"}},

	// Registry agents added by the enrichment pass. These come last so they
	// never steal matches from the higher-priority entries above.
	{keywords: []string{"ceo", "estratégia", "estrategia", "estratégico", "roadmap", "negócio", "negocio", "business", "investimento", "prioridade", "direção", "direcao"}, agents: []string{"CEO Agent"}},
	{keywords: []string{"cto", "tecnologia", "technical", "stack"}, agents: []string{"CTO Agent"}},
	{keywords: []string{"kernel", "consigliere", "ordem", "comando", "planejamento", "orquestração", "orquestracao"}, agents: []string{"COSCA KERNEL"}},
	{keywords: []string{"compliance", "lgpd", "gdpr", "conformidade", "regulatório", "regulatorio", "soc2", "jurídico", "juridico"}, agents: []string{"Compliance Chief"}},
	{keywords: []string{"performance", "benchmark", "otimização", "otimizacao", "otimizar", "lentidão", "lentidao", "profiling", "gargalo", "gargalos"}, agents: []string{"Performance Chief"}},
	{keywords: []string{"discovery", "descoberta", "novo projeto", "detectar stack", "análise do workspace", "analise do workspace"}, agents: []string{"Discovery Chief"}},
	{keywords: []string{"memória", "memoria", "aprendizado", "lembrar", "recordar", "memory"}, agents: []string{"Memory Chief"}},
	{keywords: []string{"plugin", "plugins", "extensão", "extensao", "marketplace", "wasm"}, agents: []string{"Plugin Chief"}},
	{keywords: []string{"sdk", "biblioteca", "client library", "api client"}, agents: []string{"SDK Chief"}},
	{keywords: []string{"provider", "provedor", "api key", "chave de api", "provedores"}, agents: []string{"Provider Chief"}},
	{keywords: []string{"governança", "governanca", "política", "politica", "convenção", "convencao", "policy"}, agents: []string{"Governance Chief"}},
	{keywords: []string{"mensageria", "fila", "filas", "evento", "eventos", "pub/sub", "message queue", "event bus", "kafka", "rabbitmq"}, agents: []string{"Messaging Chief"}},
	{keywords: []string{"qa", "qualidade", "garantia de qualidade", "aceite", "critérios de aceite", "criterios de aceite"}, agents: []string{"QA Chief"}},
}

// ─── Keyword → Skill Mapping ─────────────────────────────────────────────────

// keywordSkillEntry maps prompt keywords to skill names for tracking which
// skills are relevant to a request.
type keywordSkillEntry struct {
	keywords []string
	skills   []string
}

var keywordSkillMap = []keywordSkillEntry{
	{keywords: []string{"api", "apis", "endpoint", "rest", "graphql", "rota", "rotas", "serviço", "servico", "serviços", "endpoints", "microserviço", "implementar api"}, skills: []string{"api-design", "rest-api", "graphql"}},
	{keywords: []string{"component", "ui", "react", "vue", "frontend", "interface", "componente", "tela", "telas", "página", "front", "componentes", "páginas", "paginas"}, skills: []string{"ui-components", "frontend-development"}},
	{keywords: []string{"database", "sql", "schema", "migration", "banco", "banco de dados", "migração", "consulta", "tabela"}, skills: []string{"sql", "schema-design", "database-migration"}},
	{keywords: []string{"test", "unit test", "integration test", "e2e", "teste", "testes", "testar", "testando", "cobertura", "unitário", "integração", "testes unitários", "testes de integração", "cobertura de testes"}, skills: []string{"unit-testing", "integration-testing"}},
	{keywords: []string{"security", "vulnerability", "auth", "segurança", "vulnerabilidade", "auditoria", "audite", "autenticação", "autorização", "criptografia", "senha", "invasão", "exploit", "owasp", "pentest", "firewall", "certificado", "tls", "privacidade", "dados sensíveis", "dados sensiveis"}, skills: []string{"security-audit", "vulnerability-scanning"}},
	{keywords: []string{"architecture", "design pattern", "adr", "arquitetura", "padrão", "padrões"}, skills: []string{"architecture-review", "design-patterns"}},
	{keywords: []string{"deploy", "ci/cd", "docker", "kubernetes", "implantação", "infra", "ci", "cd", "build", "compilar", "contêiner", "container"}, skills: []string{"ci-cd", "containerization", "deployment"}},
	{keywords: []string{"readme", "documentation", "docs", "documentação", "manuais", "guia", "escrever documentação", "escrever doc"}, skills: []string{"technical-writing", "api-documentation"}},
	{keywords: []string{"review", "code review", "revisão", "revisar", "código", "revisão de código", "revisao de codigo", "auditar código", "auditar codigo"}, skills: []string{"code-review", "architecture-review"}},
	{keywords: []string{"monitoring", "observability", "slo", "monitoramento", "observabilidade", "alerta", "alertas", "logs", "log", "rastreamento", "tracing"}, skills: []string{"monitoring", "alerting"}},
	{keywords: []string{"analytics", "metrics", "data", "métrica", "métricas", "análise", "dados", "relatório", "relatorio", "insights"}, skills: []string{"data-analysis", "metrics"}},
	{keywords: []string{"ai", "ml", "rag", "embedding", "ia", "inteligência artificial", "modelo"}, skills: []string{"machine-learning", "rag", "embeddings"}},
	{keywords: []string{"mobile", "ios", "android"}, skills: []string{"mobile-development", "ios", "android"}},
	{keywords: []string{"release", "version", "changelog", "versão", "versões"}, skills: []string{"release-management", "versioning"}},
	{keywords: []string{"infrastructure", "cloud", "networking", "infraestrutura", "nuvem", "rede", "redes", "escala"}, skills: []string{"cloud-architecture", "infrastructure-as-code"}},
	{keywords: []string{"product", "requirement", "backlog", "produto", "requisito", "requisitos", "história"}, skills: []string{"product-management", "requirements-gathering"}},
	{keywords: []string{"workflow", "pipeline", "automation", "automação", "automatizar"}, skills: []string{"workflow-orchestration", "automation"}},
	{keywords: []string{"context", "environment", "session", "contexto", "ambiente", "sessão", "sessões"}, skills: []string{"context-management", "session-handling"}},

	// Skills mirroring the enriched registry agents, appended last.
	{keywords: []string{"ceo", "estratégia", "estrategia", "estratégico", "roadmap", "negócio", "negocio", "business", "investimento", "prioridade", "direção", "direcao"}, skills: []string{"ceo-strategy"}},
	{keywords: []string{"cto", "tecnologia", "technical", "stack"}, skills: []string{"cto-strategy"}},
	{keywords: []string{"kernel", "consigliere", "ordem", "comando", "planejamento", "orquestração", "orquestracao"}, skills: []string{"kernel-orchestration"}},
	{keywords: []string{"compliance", "lgpd", "gdpr", "conformidade", "regulatório", "regulatorio", "soc2", "jurídico", "juridico"}, skills: []string{"compliance"}},
	{keywords: []string{"performance", "benchmark", "otimização", "otimizacao", "otimizar", "lentidão", "lentidao", "profiling", "gargalo", "gargalos"}, skills: []string{"performance"}},
	{keywords: []string{"discovery", "descoberta", "novo projeto", "detectar stack", "análise do workspace", "analise do workspace"}, skills: []string{"discovery"}},
	{keywords: []string{"memória", "memoria", "aprendizado", "lembrar", "recordar", "memory"}, skills: []string{"memory-management"}},
	{keywords: []string{"plugin", "plugins", "extensão", "extensao", "marketplace", "wasm"}, skills: []string{"plugin-development"}},
	{keywords: []string{"sdk", "biblioteca", "client library", "api client"}, skills: []string{"sdk-development"}},
	{keywords: []string{"provider", "provedor", "api key", "chave de api", "provedores"}, skills: []string{"provider-management"}},
	{keywords: []string{"governança", "governanca", "política", "politica", "convenção", "convencao", "policy"}, skills: []string{"governance"}},
	{keywords: []string{"mensageria", "fila", "filas", "evento", "eventos", "pub/sub", "message queue", "event bus", "kafka", "rabbitmq"}, skills: []string{"messaging"}},
	{keywords: []string{"qa", "qualidade", "garantia de qualidade", "aceite", "critérios de aceite", "criterios de aceite"}, skills: []string{"quality-assurance"}},
}

// ─── Router ──────────────────────────────────────────────────────────────────

// Router selects the most appropriate agent for a request by examining the
// prompt and any explicit agent hints in the pipeline context.
type Router struct {
	agents AgentResolver
}

// NewRouter creates a Router backed by the given AgentResolver.
func NewRouter(agents AgentResolver) *Router {
	return &Router{agents: agents}
}

// Route selects the best agent based on the prompt and pipeline context. It
// applies heuristics in this order:
//  1. Explicit "agent" key in Data.Extra
//  2. Keyword matching against the prompt
//  3. Full-text search against the agent registry
//  4. Fallback to "CEO Agent"
//
// The resolved agent and derived skills are stored in the returned
// PipelineContext.
func (r *Router) Route(ctx context.Context, pc PipelineContext) (PipelineContext, error) {
	logger := log.Ctx(ctx).With().Str("stage", "router").Str("request_id", pc.RequestID).Logger()

	// 1. Check Data.Extra["agent"] — caller explicitly requested an agent.
	if agentName, ok := pc.Data.Extra["agent"].(string); ok && strings.TrimSpace(agentName) != "" {
		agentName = strings.TrimSpace(agentName)
		agent, err := r.agents.Get(agentName)
		if err == nil {
			logger.Debug().Str("agent", agentName).Str("role", agent.Role).Msg("agent resolved via explicit hint")
			pc = r.resolveAgent(pc, agent)
			pc = r.attachSkills(pc, []string{agentName, agent.Role, agent.Department, agent.Description})
			pc = pc.WithRouterMethod("explicit")
			return pc, nil
		}
		logger.Warn().Str("agent", agentName).Msg("explicit agent not found, falling back to keyword matching")
	}

	// 2. Keyword matching against the prompt. The prompt is normalized
	// (accent-stripped and lowercased) so PT-BR queries typed without
	// diacritics still route correctly ("seguranca" == "segurança").
	promptLower := normalizeText(pc.Prompt)
	matchedEntries := r.matchKeywords(promptLower)

	for _, entry := range matchedEntries {
		for _, candidateName := range entry.agents {
			agent, err := r.agents.Get(candidateName)
			if err == nil {
				logger.Debug().
					Str("agent", candidateName).
					Str("role", agent.Role).
					Strs("matched_keywords", entry.keywords).
					Msg("agent resolved via keyword matching")
				pc = r.resolveAgent(pc, agent)
				pc = r.attachSkills(pc, []string{agent.Name, agent.Role, agent.Department, agent.Description})
				pc = pc.WithRouterMethod("keyword")
				return pc, nil
			}
		}
	}

	// 3. Full-text search against the agent registry.
	//
	// Defensive greeting guard: a greeting can reach this point when the
	// keyword entry above was skipped because Kernel Agent was not resolvable
	// at keyword time. Never let "Oi" full-text match a random specialist —
	// route it straight to the Kernel (the family's front door) instead.
	if isGreeting(pc.Prompt) {
		if kernel, err := r.agents.Get("Kernel Agent"); err == nil {
			logger.Debug().
				Str("agent", kernel.Name).
				Str("role", kernel.Role).
				Msg("agent resolved via greeting → Kernel Agent")
			pc = r.resolveAgent(pc, kernel)
			pc = r.attachSkills(pc, []string{kernel.Name, kernel.Role, kernel.Department, kernel.Description})
			pc = pc.WithRouterMethod("keyword")
			return pc, nil
		}
	}

	results, err := r.agents.Search(pc.Prompt)
	if err == nil && len(results) > 0 {
		first := &results[0]
		logger.Debug().
			Str("agent", first.Name).
			Str("role", first.Role).
			Int("total_results", len(results)).
			Msg("agent resolved via full-text search")
		pc = r.resolveAgent(pc, first)
		pc = r.attachSkills(pc, []string{first.Name, first.Role, first.Department, first.Description})
		pc = pc.WithRouterMethod("search")
		return pc, nil
	}

	// 4. No agent matched — return a clear error with available agents.
	available := r.buildAvailableAgentList()
	// The prompt and registry listing are untrusted/user-controlled data. Keep
	// the long diagnostic in logs only; callers receive the stable category.
	return pc, safePublicError("router", "routing_failed", fmt.Errorf("no agent found for task: %s. Available agents: %s", pc.Prompt, available))
}

// ─── Keyword Matching ────────────────────────────────────────────────────────

// normalizeText lowercases s and strips all combining diacritical marks by
// decomposing to NFD and dropping Mn (nonspacing mark) runes. PT-BR users
// routinely omit accents when typing ("seguranca" instead of "segurança",
// "autenticacao" instead of "autenticação"), so keyword matching must treat
// accented and unaccented forms as equivalent — otherwise valid queries miss
// their intended agent and fall through to full-text search or an error.
func normalizeText(s string) string {
	decomposed := norm.NFD.String(strings.ToLower(s))
	var b strings.Builder
	b.Grow(len(decomposed))
	for _, r := range decomposed {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// matchKeywords scans the prompt against each keyword group and returns
// matching entries in priority order. An entry matches if any of its keywords
// is found in the prompt. The prompt is normalized (accent-stripped and
// lowercased) here so callers passing raw text, e.g. tests, get the same
// accent-insensitive behavior as the router itself.
func (r *Router) matchKeywords(prompt string) []keywordAgentEntry {
	promptNorm := normalizeText(prompt)
	var matched []keywordAgentEntry
	for _, entry := range keywordAgentMap {
		for _, kw := range entry.keywords {
			if matchesKeyword(promptNorm, kw) {
				matched = append(matched, entry)
				break
			}
		}
	}
	return matched
}

// matchesKeyword reports whether kw occurs in the normalized (accent-stripped,
// lowercased) prompt. The keyword is normalized before comparing, so accented
// ("segurança") and unaccented ("seguranca") forms both resolve to the same
// token. Keywords of length <= wordBoundaryKeywordLen must appear as whole
// words so that short substrings like "api" don't match inside unrelated words
// ("capital" contains "api", which wrongly routed generic Portuguese queries to
// the Backend team). Longer keywords match as substrings, preserving matches
// for plurals and inflections ("teste" matches "testes", "documentação"
// matches "documentações").
func matchesKeyword(normalizedPrompt, kw string) bool {
	kwNorm := normalizeText(kw)
	if len(kwNorm) > wordBoundaryKeywordLen {
		return strings.Contains(normalizedPrompt, kwNorm)
	}
	return containsWordBoundary(normalizedPrompt, kwNorm)
}

// isGreeting reports whether the prompt is a greeting or small talk. Greeting
// prompts are routed to the Kernel Agent (the family's front door) and must
// never be full-text matched against a random specialist by chance ("Oi"
// used to match the Testing Integration Specialist).
func isGreeting(prompt string) bool {
	promptNorm := normalizeText(prompt)
	for _, kw := range greetingKeywords {
		if matchesKeyword(promptNorm, kw) {
			return true
		}
	}
	return false
}

// containsWordBoundary reports whether kw appears in s with a non-word
// character on both sides (or the string edges). Word characters are
// ASCII letters and digits.
func containsWordBoundary(s, kw string) bool {
	start := 0
	for {
		i := strings.Index(s[start:], kw)
		if i < 0 {
			return false
		}
		pos := start + i
		beforeOK := pos == 0 || !isWordByte(s[pos-1])
		after := pos + len(kw)
		afterOK := after >= len(s) || !isWordByte(s[after])
		if beforeOK && afterOK {
			return true
		}
		start = pos + 1
	}
}

// isWordByte reports whether b is an ASCII word character.
func isWordByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9')
}

// ─── Agent Resolution ────────────────────────────────────────────────────────

// resolveAgent stores the resolved agent's information in the PipelineContext.
func (r *Router) resolveAgent(pc PipelineContext, agent *AgentInfo) PipelineContext {
	pc = pc.WithResolvedAgent(agent.Name)
	pc = pc.WithAgentRole(agent.Role)
	pc = pc.WithAgentDepartment(agent.Department)
	pc = pc.WithAgentDescription(agent.Description)
	return pc
}

// buildAvailableAgentList returns a comma-separated list of agent names from the
// registry for use in error messages. Returns "(none)" when the resolver is nil
// or the registry is empty, or when the list operation fails.
func (r *Router) buildAvailableAgentList() string {
	if r.agents == nil {
		return "(none)"
	}
	agents, err := r.agents.Search("")
	if err != nil || len(agents) == 0 {
		return "(none)"
	}
	names := make([]string, 0, len(agents))
	for _, a := range agents {
		names = append(names, a.Name)
	}
	return strings.Join(names, ", ")
}

// ─── Skill Derivation ────────────────────────────────────────────────────────

// attachSkills derives skill names from the agent information and the prompt,
// then stores them in Data.SkillsUsed.
func (r *Router) attachSkills(pc PipelineContext, agentTexts []string) PipelineContext {
	skills := DeriveSkills(pc.Prompt, agentTexts)
	pc = pc.WithSkillsUsed(skills)
	return pc
}
