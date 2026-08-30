package orchestration

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// ─── Semantic Router Config ──────────────────────────────────────────────────

// SemanticRouterConfig configures the embedding-based agent router.
type SemanticRouterConfig struct {
	// Enabled activates semantic routing. When false, the keyword router
	// is used directly.
	Enabled bool

	// HighThreshold is the cosine similarity score above which an agent
	// match is considered highly confident (no fallback needed).
	// Default: 0.85.
	HighThreshold float64

	// LowThreshold is the minimum cosine similarity score to accept an
	// agent match. Scores below this trigger fallback to keyword routing.
	// Default: 0.72.
	LowThreshold float64

	// MaxCandidates limits how many top agents are compared. Smaller
	// values reduce latency but may miss good matches.
	// Default: 5.
	MaxCandidates int

	// FallbackToKeyword enables automatic fallback to the keyword-based
	// router when embedding similarity is below LowThreshold or the
	// embedder is unavailable.
	// Default: true.
	FallbackToKeyword bool

	// RefreshInterval controls how often agent embeddings are recomputed.
	// Zero means "never refresh after startup".
	// Default: 0 (static after startup).
	RefreshInterval time.Duration
}

// DefaultSemanticRouterConfig returns sensible defaults.
func DefaultSemanticRouterConfig() SemanticRouterConfig {
	return SemanticRouterConfig{
		Enabled:           true,
		HighThreshold:     0.85,
		LowThreshold:      0.72,
		MaxCandidates:     5,
		FallbackToKeyword: true,
		RefreshInterval:   0,
	}
}

// minSemanticMargin is the minimum gap between the best and second-best
// similarity scores required to accept a semantic match (both at the high and
// low threshold bands). Generic queries — e.g. "qual é a capital do brasil" —
// produce near-identical baseline similarity (~0.5-0.65) against every agent,
// so a tiny margin means the top-1 is essentially arbitrary. Requiring a real
// margin avoids delegating such queries to whatever agent happens to rank
// first (historically the Backend team) and lets them fall through to the
// keyword router / CEO fallback instead.
const minSemanticMargin = 0.05

// ─── Agent Embedding ─────────────────────────────────────────────────────────

// agentEmbedding holds a pre-computed embedding for an agent.
type agentEmbedding struct {
	Name      string
	Embedding []float64
}

// ─── Semantic Router ─────────────────────────────────────────────────────────

// SemanticRouter selects agents using vector similarity between the user
// prompt and pre-computed agent description embeddings. It cascades through
// the semantic + keyword routers for maximum coverage. O fallback para
// Full-text Search e CEO é responsabilidade do Router (keyword) — este
// componente implementa apenas o caminho de similaridade semântica e delega
// ao keyword router quando não houver match confiável:
//
//	Semantic (high ≥0.85 + margem) → Semantic (low ≥0.72 + margem) → Keyword (Router) → CEO
//
// The embedding-based approach is language-agnostic — it works equally well
// with prompts in English, Portuguese, Spanish, Japanese, and any other
// language supported by the underlying embedding model.
type SemanticRouter struct {
	agents   AgentResolver
	embedder Embedder
	keyword  *Router

	mu              sync.RWMutex
	agentEmbeddings []agentEmbedding
	config          SemanticRouterConfig
	lastRefresh     time.Time

	// refreshMu é o single-flight lock de recomputação de embeddings. Evita
	// que N requisições concorrentes, ao mesmo tempo que o refresh vence,
	// disparem N computações duplicadas de embeddings (desperdício de tokens
	// no provider — exatamente o que o COSCA mede). Apenas 1 goroutine
	// recalcula; as demais esperam e reutilizam o snapshot publicado.
	refreshMu sync.Mutex

	// sem limits concurrent embedding API calls to prevent overwhelming
	// the provider when falling back to individual calls. Capacity of 5.
	sem chan struct{}
}

// NewSemanticRouter creates a SemanticRouter backed by the given AgentResolver,
// Embedder, and keyword Router (used as fallback).
func NewSemanticRouter(agents AgentResolver, embedder Embedder, keyword *Router, config SemanticRouterConfig) *SemanticRouter {
	if config.HighThreshold <= 0 {
		config.HighThreshold = 0.85
	}
	if config.LowThreshold <= 0 {
		config.LowThreshold = 0.72
	}
	// MaxCandidates precisa ser >= 2: com 1 único candidato, o secondScore
	// seria 0 e o margin viraria score superestimado, falsificando a
	// confiança (um match quase-arbitrário pareceria "com margem"). Forçamos
	// o piso aqui para preservar a invariância do margin.
	if config.MaxCandidates < 2 {
		config.MaxCandidates = 5
	}

	sr := &SemanticRouter{
		agents:   agents,
		embedder: embedder,
		keyword:  keyword,
		config:   config,
		sem:      make(chan struct{}, 5),
	}

	return sr
}

// ─── Route ────────────────────────────────────────────────────────────────────

// Route selects the best agent using the cascade:
//
//	Semantic (high threshold) → Semantic (low threshold) → Keyword → Search → CEO
//
// The resolved agent and derived skills are stored in the returned
// PipelineContext.
func (sr *SemanticRouter) Route(ctx context.Context, pc PipelineContext) (PipelineContext, error) {
	logger := log.Ctx(ctx).With().Str("stage", "semantic_router").Str("request_id", pc.RequestID).Logger()

	if !sr.config.Enabled || sr.embedder == nil {
		logger.Debug().Msg("semantic router disabled or no embedder, delegating to keyword router")
		if sr.keyword != nil {
			return sr.keyword.Route(ctx, pc)
		}
		return pc.WithRouterMethod("fallback"), nil
	}

	// 1. Explicit agent hint — bypass all routing.
	if agentName, ok := pc.Data.Extra["agent"].(string); ok && strings.TrimSpace(agentName) != "" {
		agent, err := sr.agents.Get(strings.TrimSpace(agentName))
		if err == nil {
			logger.Debug().Str("agent", agentName).Msg("agent resolved via explicit hint")
			pc = sr.resolveAgent(pc, agent)
			pc = pc.WithRouterMethod("explicit")
			return pc, nil
		}
	}

	// 2. Embed the user prompt.
	promptEmbedding, err := sr.embedPrompt(ctx, pc.Prompt)
	if err != nil {
		info := safeError("embedding_failed", err)
		logger.Warn().Str("error_code", info.Code).Str("error_hash", info.Hash).Int("error_length", info.Length).Msg("failed to embed prompt, falling back to keyword router")
		pc = pc.WithEmbeddingError(info.Code)
		return sr.fallbackToKeyword(ctx, pc)
	}

	// 3. Refresh agent embeddings if needed.
	if err := sr.ensureEmbeddings(ctx); err != nil {
		info := safeError("agent_embeddings_failed", err)
		logger.Warn().Str("error_code", info.Code).Str("error_hash", info.Hash).Int("error_length", info.Length).Msg("failed to compute agent embeddings, falling back to keyword router")
		return sr.fallbackToKeyword(ctx, pc)
	}

	// 4. Find best matching agent. The second-best score is needed to
	// enforce a confidence margin below.
	best, score, secondScore := sr.findBestMatch(promptEmbedding)
	margin := score - secondScore

	// 5. Decision cascade. A semantic match is only accepted when the score
	// clears the threshold AND the best agent beats the runner-up by at
	// least minSemanticMargin. Generic queries produce a near-tie across
	// all agents; without the margin check they would be routed to an
	// arbitrary top-1 instead of falling back to keyword routing.
	switch {
	case score >= sr.config.HighThreshold && margin >= minSemanticMargin:
		agent, err := sr.agents.Get(best)
		if err == nil {
			logger.Debug().
				Str("agent", best).
				Float64("score", score).
				Float64("margin", margin).
				Str("confidence", "high").
				Msg("agent resolved via semantic routing")
			pc = sr.resolveAgent(pc, agent)
			pc = pc.WithRouterMethod("semantic_high")
			pc = pc.WithSemanticScore(score)
			pc = sr.attachSkills(pc, agentSkillTexts(agent))
			return pc, nil
		}

	case score >= sr.config.LowThreshold && margin >= minSemanticMargin:
		agent, err := sr.agents.Get(best)
		if err == nil {
			logger.Debug().
				Str("agent", best).
				Float64("score", score).
				Float64("margin", margin).
				Str("confidence", "low").
				Msg("agent resolved via semantic routing (low confidence)")
			pc = sr.resolveAgent(pc, agent)
			pc = pc.WithRouterMethod("semantic_low")
			pc = pc.WithSemanticScore(score)
			pc = sr.attachSkills(pc, agentSkillTexts(agent))
			return pc, nil
		}
	}

	// 6. No semantic match above thresholds — fallback.
	logger.Debug().
		Str("best_agent", best).
		Float64("best_score", score).
		Float64("second_score", secondScore).
		Float64("margin", margin).
		Float64("low_threshold", sr.config.LowThreshold).
		Msg("semantic score below threshold or insufficient margin, falling back to keyword router")
	return sr.fallbackToKeyword(ctx, pc)
}

// ─── Embedding ────────────────────────────────────────────────────────────────

// embedPrompt generates an embedding vector for the user prompt.
func (sr *SemanticRouter) embedPrompt(ctx context.Context, prompt string) ([]float64, error) {
	result, err := sr.embedder.GenerateEmbedding(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("embed prompt: %w", err)
	}
	if result == nil || len(result.Vector) == 0 {
		return nil, fmt.Errorf("empty embedding returned")
	}
	return result.Vector, nil
}

// ensureEmbeddings makes sure agent embeddings are available, computing
// them on first call and refreshing periodically if configured.
//
// Single-flight: usa o refreshMu para impedir que N requisições concorrentes
// (chegando ao mesmo tempo que o refresh vence) disparem N computações
// duplicadas. Somente uma goroutine recalcula; as demais aguardam o lock e
// reutilizam o snapshot já publicado. O routing normal continua lendo sob
// RLock — não há bloqueio de leitura, apenas de recomputação.
func (sr *SemanticRouter) ensureEmbeddings(ctx context.Context) error {
	sr.mu.RLock()
	hasEmbeddings := len(sr.agentEmbeddings) > 0
	needsRefresh := sr.config.RefreshInterval > 0 &&
		time.Since(sr.lastRefresh) > sr.config.RefreshInterval
	sr.mu.RUnlock()

	if hasEmbeddings && !needsRefresh {
		return nil
	}

	// Single-flight: apenas a primeira goroutine que chegar aqui executa a
	// recomputação. As demais esperam liberar o lock e checam de novo.
	sr.refreshMu.Lock()
	defer sr.refreshMu.Unlock()

	// Double-check após adquirir o lock: outra goroutine pode ter concluído
	// a recomputação enquanto esperávamos.
	sr.mu.RLock()
	hasEmbeddings = len(sr.agentEmbeddings) > 0
	needsRefresh = sr.config.RefreshInterval > 0 &&
		time.Since(sr.lastRefresh) > sr.config.RefreshInterval
	sr.mu.RUnlock()

	if hasEmbeddings && !needsRefresh {
		return nil
	}

	return sr.computeEmbeddings(ctx)
}

// computeEmbeddings generates embeddings for all registered agents.
// It first attempts a single batch embedding call (dramatically faster
// when the provider supports it), then falls back to individual calls
// with a semaphore-bound concurrency limit of 5. Partial failures are
// tolerated — as long as at least 2 agents are successfully embedded,
// the router can operate.
func (sr *SemanticRouter) computeEmbeddings(ctx context.Context) error {
	agents, err := sr.agents.Search("")
	if err != nil {
		return fmt.Errorf("list agents: %w", err)
	}
	if len(agents) == 0 {
		return fmt.Errorf("no agents available for embedding")
	}

	descriptions := make([]string, len(agents))
	for i, agent := range agents {
		descriptions[i] = buildAgentDescription(agent)
	}

	// ── 1. Try batch embedding ──────────────────────────────────────────

	batchResults, batchErr := sr.embedder.GenerateEmbeddings(ctx, descriptions)
	if batchErr == nil {
		if len(batchResults) != len(descriptions) {
			// Provider retornou menos resultados sem erro: correlacionar
			// por índice (agents[i]) seria fora de range. Tratamos como
			// falha de batch e caimos no fallback individual.
			log.Warn().
				Int("expected", len(descriptions)).
				Int("got", len(batchResults)).
				Msg("batch embedding count mismatch, falling back to individual calls")
			return sr.computeEmbeddingsIndividual(ctx, agents, descriptions)
		}

		var embeddings []agentEmbedding
		var dim int
		for i, result := range batchResults {
			if result == nil || len(result.Vector) == 0 {
				log.Warn().Str("agent", agents[i].Name).Msg("batch embedding nil/empty, skipping")
				continue
			}
			// Valida dimensão uniforme entre todos os embeddings.
			if dim == 0 {
				dim = len(result.Vector)
			} else if len(result.Vector) != dim {
				log.Warn().
					Str("agent", agents[i].Name).
					Int("dimensions", len(result.Vector)).
					Int("expected_dimensions", dim).
					Msg("embedding dimension mismatch, falling back to individual calls")
				return sr.computeEmbeddingsIndividual(ctx, agents, descriptions)
			}
			embeddings = append(embeddings, agentEmbedding{
				Name:      agents[i].Name,
				Embedding: result.Vector,
			})
		}
		if len(embeddings) < 2 {
			return fmt.Errorf("insufficient agent embeddings from batch: got %d, need at least 2", len(embeddings))
		}
		sr.mu.Lock()
		sr.agentEmbeddings = embeddings
		sr.lastRefresh = time.Now()
		sr.mu.Unlock()
		log.Info().Int("agent_count", len(embeddings)).Msg("agent embeddings computed (batch)")
		return nil
	}

	// ── 2. Fall back to individual calls with semaphore ─────────────────

	info := safeError("batch_embedding_failed", batchErr)
	log.Warn().Str("error_code", info.Code).Str("error_hash", info.Hash).Int("error_length", info.Length).Msg("batch embedding failed, falling back to individual calls (max 5 concurrent)")
	return sr.computeEmbeddingsIndividual(ctx, agents, descriptions)
}

// computeEmbeddingsIndividual gera embeddings agente a agente, limitando a
// concorrência a 5 chamadas simultâneas ao provider (semaphore). Tolerante a
// falhas parciais — exige que pelo menos 2 agentes sejam embedados com
// sucesso. Preserva a causa de cancelamento quando o ctx é cancelado.
func (sr *SemanticRouter) computeEmbeddingsIndividual(ctx context.Context, agents []AgentInfo, descriptions []string) error {
	var (
		embeddings []agentEmbedding
		mu         sync.Mutex
		wg         sync.WaitGroup
		firstErr   error
		errMu      sync.Mutex
	)

	for i, agent := range agents {
		wg.Add(1)
		go func(a AgentInfo, desc string) {
			defer wg.Done()

			// Acquire semaphore (blocks if 5 goroutines already active).
			select {
			case sr.sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sr.sem }()

			result, err := sr.embedder.GenerateEmbedding(ctx, desc)
			if err != nil {
				info := safeError("agent_embedding_failed", err)
				log.Warn().Str("error_code", info.Code).Str("error_hash", info.Hash).Int("error_length", info.Length).Str("agent", a.Name).Msg("failed to embed agent description, skipping")
				errMu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				errMu.Unlock()
				return
			}

			mu.Lock()
			embeddings = append(embeddings, agentEmbedding{
				Name:      a.Name,
				Embedding: result.Vector,
			})
			mu.Unlock()
		}(agent, descriptions[i])
	}

	wg.Wait()

	// Se o contexto foi cancelado, preserva a causa (em vez de reportar
	// "insufficient embeddings", que esconde o cancelamento real).
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("embedding context canceled: %w", err)
	}

	// Require at least 2 agents for meaningful routing.
	if len(embeddings) < 2 {
		if firstErr != nil {
			return fmt.Errorf("insufficient agent embeddings (%d): %w", len(embeddings), firstErr)
		}
		return fmt.Errorf("insufficient agent embeddings: got %d, need at least 2", len(embeddings))
	}

	sr.mu.Lock()
	sr.agentEmbeddings = embeddings
	sr.lastRefresh = time.Now()
	sr.mu.Unlock()

	log.Info().
		Int("agent_count", len(embeddings)).
		Msg("agent embeddings computed (individual)")

	return nil
}

// ─── Similarity Matching ─────────────────────────────────────────────────────

// findBestMatch compares the prompt embedding against all agent embeddings
// and returns the best agent name, its similarity score, and the
// second-best score. The second-best score backs the margin requirement in
// Route: generic queries yield near-identical similarity across all agents,
// and returning the runner-up lets the router reject the arbitrary top-1 and
// fall back to keyword routing.
func (sr *SemanticRouter) findBestMatch(promptEmbedding []float64) (string, float64, float64) {
	sr.mu.RLock()
	embeddings := sr.agentEmbeddings
	sr.mu.RUnlock()

	if len(embeddings) == 0 {
		return "", 0, 0
	}

	type scored struct {
		name  string
		score float64
	}

	var scores []scored
	for _, ae := range embeddings {
		s := cosineSimilarity(promptEmbedding, ae.Embedding)
		// -1 sinaliza dimensão de embedding divergente (candidato inválido).
		// Descartamos em vez de comparar entre vetores incompatíveis.
		if s < 0 {
			continue
		}
		scores = append(scores, scored{name: ae.Name, score: s})
	}

	if len(scores) == 0 {
		// Todos os candidatos tinham embedding de dimensão incompatível.
		return "", 0, 0
	}

	// Sort descending by score.
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	// Limit to top candidates.
	if len(scores) > sr.config.MaxCandidates {
		scores = scores[:sr.config.MaxCandidates]
	}

	if len(scores) == 0 {
		return "", 0, 0
	}

	second := 0.0
	if len(scores) > 1 {
		second = scores[1].score
	}
	return scores[0].name, scores[0].score, second
}

// ─── Fallback ─────────────────────────────────────────────────────────────────

// fallbackToKeyword delegates to the keyword router, or returns an error
// when no keyword router is configured or fallback is disabled.
func (sr *SemanticRouter) fallbackToKeyword(ctx context.Context, pc PipelineContext) (PipelineContext, error) {
	if sr.keyword != nil && sr.config.FallbackToKeyword {
		return sr.keyword.Route(ctx, pc)
	}
	// No fallback available — return an honest error.
	available := sr.buildAvailableAgentList()
	return pc, safePublicError("semantic router", "routing_failed", fmt.Errorf("semantic routing: no agent matched prompt %q with thresholds [%.2f, %.2f]. Available agents: %s",
		truncateString(pc.Prompt, 100), sr.config.HighThreshold, sr.config.LowThreshold, available))
}

// ─── Agent Resolution ────────────────────────────────────────────────────────

// resolveAgent stores the resolved agent's information in the PipelineContext.
func (sr *SemanticRouter) resolveAgent(pc PipelineContext, agent *AgentInfo) PipelineContext {
	pc = pc.WithResolvedAgent(agent.Name)
	pc = pc.WithAgentRole(agent.Role)
	pc = pc.WithAgentDepartment(agent.Department)
	pc = pc.WithAgentDescription(agent.Description)
	pc = pc.WithAgentCapabilities(agent.Capabilities)
	pc = pc.WithAgentResponsibilities(agent.Responsibilities)
	return pc
}

// buildAvailableAgentList returns a comma-separated list of agent names from
// the registry for use in error messages.
func (sr *SemanticRouter) buildAvailableAgentList() string {
	if sr.agents == nil {
		return "(none)"
	}
	agents, err := sr.agents.Search("")
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

// attachSkills derives skill names from the agent information and the prompt.
func (sr *SemanticRouter) attachSkills(pc PipelineContext, agentTexts []string) PipelineContext {
	skills := DeriveSkills(pc.Prompt, agentTexts)
	pc = pc.WithSkillsUsed(skills)
	return pc
}

// ─── Builder ─────────────────────────────────────────────────────────────────

// buildAgentDescription concatenates agent metadata into a single string
// optimized for embedding generation. The format is designed to produce
// dense semantic representations that capture the agent's identity,
// domain, and capabilities.
func buildAgentDescription(agent AgentInfo) string {
	var sb strings.Builder
	sb.WriteString(agent.Name)
	sb.WriteString(". ")
	sb.WriteString(agent.Role)
	if agent.Department != "" {
		sb.WriteString(". Department: ")
		sb.WriteString(agent.Department)
	}
	if agent.Description != "" {
		sb.WriteString(". ")
		sb.WriteString(agent.Description)
	}
	// Inclui capabilities e responsibilities: a identidade operacional
	// completa do agente (nao apenas nome/role/desc). Isso alinha o perfil
	// usado para ESCOLHER o agente com o perfil usado para derivar skills.
	if len(agent.Capabilities) > 0 {
		sb.WriteString(". Capabilities: ")
		sb.WriteString(strings.Join(agent.Capabilities, ", "))
	}
	if len(agent.Responsibilities) > 0 {
		sb.WriteString(". Responsibilities: ")
		sb.WriteString(strings.Join(agent.Responsibilities, ", "))
	}
	return sb.String()
}

// agentSkillTexts monta o conjunto de textos do agente usados para derivar
// skills (attachSkills), incluindo capabilities e responsibilities — não só
// identity básica. Assim a derivação de skills enxerga a mesma identidade
// operacional usada na escolha do agente (ponto 7 do review).
func agentSkillTexts(agent *AgentInfo) []string {
	texts := []string{agent.Name, agent.Role, agent.Department, agent.Description}
	texts = append(texts, agent.Capabilities...)
	texts = append(texts, agent.Responsibilities...)
	return texts
}

// ─── Math ────────────────────────────────────────────────────────────────────

// cosineSimilarity computes the cosine similarity between two vectors.
//
// INVARIANT: os dois vetores DEVEM ter a mesma dimensão. Truncar/pad
// silenciosamente produziria um score numericamente válido mas
// semanticamente inválido (por ex. 768 × 1024 viraria 768 × 768 e pareceria
// uma comparação legítima). Para o COSCA, dimensões divergentes indicam um
// erro de configuração do embedder — sinalizamos retornando -1, que é
// impossível para um cosseno válido (intervalo [0,1]) e faz o router
// descartar o candidato em vez de tomar uma decisão aparentemente válida.
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return -1
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// DeriveSkills is a package-level function shared between Router and
// SemanticRouter. It matches prompt keywords against the skill keyword map
// to produce a deduplicated list of relevant skill names.
func DeriveSkills(prompt string, agentTexts []string) []string {
	promptLower := strings.ToLower(prompt)
	seen := make(map[string]struct{})
	var skills []string

	for _, entry := range keywordSkillMap {
		for _, kw := range entry.keywords {
			if strings.Contains(promptLower, kw) {
				for _, sk := range entry.skills {
					if _, ok := seen[sk]; !ok {
						seen[sk] = struct{}{}
						skills = append(skills, sk)
					}
				}
				break
			}
		}
	}

	combinedAgentText := strings.ToLower(strings.Join(agentTexts, " "))
	for _, entry := range keywordSkillMap {
		for _, kw := range entry.keywords {
			if strings.Contains(combinedAgentText, kw) {
				for _, sk := range entry.skills {
					if _, ok := seen[sk]; !ok {
						seen[sk] = struct{}{}
						skills = append(skills, sk)
					}
				}
				break
			}
		}
	}

	return skills
}
