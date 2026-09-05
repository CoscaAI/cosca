package engine

import (
	"context"
	"math"
	"strings"
	"unicode"

	"github.com/CoscaAI/cosca/internal/chat"
)

// ─── Routing Constants ────────────────────────────────────────────────────────

const (
	// methodMention is used when the user explicitly mentions an agent via @name or /agent name.
	methodMention = "mention"

	// methodKeyword is used when keyword/capability matching selects the agent.
	methodKeyword = "keyword"

	// methodSemantic is used when semantic (embedding) matching selects the agent.
	methodSemantic = "semantic"

	// methodFallback is used when no match is found; routes to the kernel agent.
	methodFallback = "fallback"

	// defaultAgent is the fallback agent name when routing cannot determine the target.
	// cosca-kernel exists in .cosca/framework/agents/ (cosca-general does not).
	defaultAgent = "cosca-kernel"

	// keywordConfidenceThreshold is the minimum score for keyword routing to trigger.
	keywordConfidenceThreshold = 5

	// maxKeywordScore caps the raw keyword match score for confidence calculation.
	maxKeywordScore = 20

	// semanticMinScore is the minimum cosine similarity for Stage 3 semantic
	// routing to trigger. Below this, routing falls through to the fallback.
	semanticMinScore = 0.4
)

// ─── Router ───────────────────────────────────────────────────────────────────

// Router routes user requests to the most appropriate agent using a three-stage
// strategy: direct mention, keyword/capability matching, and semantic matching
// (with stub fallback). This implements the routing stage of the Agent Loop
// described in next-gen-cli-design.md §5.
type Router struct {
	registry *AgentRegistry
}

// NewRouter creates a Router backed by the given agent registry.
func NewRouter(registry *AgentRegistry) *Router {
	return &Router{
		registry: registry,
	}
}

// Route analyzes the user input and conversation history to select the best agent.
// The routing strategy is:
//
//  1. Mention: If the input contains @agent-name or /agent agent-name, use that agent.
//  2. Keyword: Match words in the input against agent capabilities, descriptions,
//     and names. The agent with the highest cumulative score wins.
//  3. Semantic: Cosine similarity over token-frequency vectors of the input vs.
//     each agent's description and capabilities. Used only when keyword matching
//     is insufficient; the best agent above semanticMinScore wins.
//  4. Fallback: Return "cosca-kernel" at low confidence.
//
// The returned RouteResult includes the selected agent name, a confidence score
// (0.0–1.0), and the method used for resolution.
func (r *Router) Route(ctx context.Context, userInput string, history []chat.Message) RouteResult {
	// Stage 1: Direct agent mention (@name or /agent name).
	if mention := r.ExtractAgentMention(userInput); mention != "" {
		if def := r.registry.Get(mention); def != nil {
			return RouteResult{
				Agent:      def.Name,
				Confidence: 1.0,
				Method:     methodMention,
			}
		}
	}

	// Stage 2: Keyword / capability matching.
	if result, ok := r.matchByKeyword(userInput); ok {
		return result
	}

	// Stage 3: Semantic matching — cosine similarity over token-frequency vectors.
	if result, ok := r.matchBySemantic(userInput); ok {
		return result
	}

	// Stage 4: Fallback.
	return RouteResult{
		Agent:      defaultAgent,
		Confidence: 0.3,
		Method:     methodFallback,
	}
}

// matchByKeyword scores agents by matching the user input against their name,
// capabilities, and description. Returns the best match if it meets the minimum
// confidence threshold.
func (r *Router) matchByKeyword(userInput string) (RouteResult, bool) {
	bestAgent := ""
	bestScore := 0
	inputLower := strings.ToLower(userInput)
	inputWords := tokenize(inputLower)

	for _, def := range r.registry.List() {
		score := 0

		// Exact name match (highest weight).
		if strings.Contains(strings.ToLower(def.Name), inputLower) {
			score += 10
		}

		// Capability matches.
		for _, cap := range def.Capabilities {
			if strings.Contains(inputLower, strings.ToLower(cap)) {
				score += 5
			}
		}

		// Description match.
		if strings.Contains(strings.ToLower(def.Description), inputLower) {
			score += 3
		}

		// Per-word matching against capabilities.
		for _, word := range inputWords {
			if len(word) < 3 {
				continue
			}
			for _, cap := range def.Capabilities {
				if strings.Contains(strings.ToLower(cap), word) {
					score += 2
				}
			}
		}

		if score > bestScore {
			bestScore = score
			bestAgent = def.Name
		}
	}

	if bestScore < keywordConfidenceThreshold {
		return RouteResult{}, false
	}

	// Normalise score to 0.0–1.0 confidence (capped at 0.9 for keyword matches).
	confidence := float64(bestScore) / float64(maxKeywordScore)
	if confidence > 0.9 {
		confidence = 0.9
	}

	return RouteResult{
		Agent:      bestAgent,
		Confidence: confidence,
		Method:     methodKeyword,
	}, true
}

// matchBySemantic scores agents by cosine similarity between the token-frequency
// vectors of the user input and each agent's description plus capabilities. It
// returns the best match when its score is at or above semanticMinScore; this is
// used as a fallback when keyword matching is insufficient.
func (r *Router) matchBySemantic(userInput string) (RouteResult, bool) {
	inputVec := tokenFrequencyVector(tokenize(userInput))

	bestAgent := ""
	bestScore := 0.0

	for _, def := range r.registry.List() {
		agentText := def.Description
		for _, cap := range def.Capabilities {
			agentText += " " + cap
		}
		agentVec := tokenFrequencyVector(tokenize(agentText))
		if score := cosineSimilarity(inputVec, agentVec); score > bestScore {
			bestScore = score
			bestAgent = def.Name
		}
	}

	if bestAgent == "" || bestScore < semanticMinScore {
		return RouteResult{}, false
	}

	return RouteResult{
		Agent:      bestAgent,
		Confidence: bestScore,
		Method:     methodSemantic,
	}, true
}

// tokenFrequencyVector builds a term-frequency map from a token slice. Repeated
// tokens accumulate counts, giving common vocabulary a higher weight.
func tokenFrequencyVector(tokens []string) map[string]float64 {
	vec := make(map[string]float64, len(tokens))
	for _, tok := range tokens {
		vec[tok]++
	}
	return vec
}

// cosineSimilarity computes the cosine similarity between two term-frequency
// vectors. It returns 0.0 when either vector is empty.
func cosineSimilarity(a, b map[string]float64) float64 {
	var dot, normA, normB float64
	for k, va := range a {
		dot += va * b[k]
		normA += va * va
	}
	for _, vb := range b {
		normB += vb * vb
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

// ExtractAgentMention extracts agent names from input text in two forms:
//   - @mention: e.g. "@cosca-database" yields "cosca-database"
//   - /agent command: e.g. "/agent cosca-database" yields "cosca-database"
//
// Returns an empty string if no mention is found.
func (r *Router) ExtractAgentMention(input string) string {
	// Check for @mention pattern.
	if idx := strings.Index(input, "@"); idx >= 0 {
		if name := extractAgentName(input[idx+1:]); name != "" {
			return name
		}
	}

	// Check for /agent command pattern.
	const agentPrefix = "/agent "
	if idx := strings.Index(strings.ToLower(input), agentPrefix); idx >= 0 {
		if name := extractAgentName(input[idx+len(agentPrefix):]); name != "" {
			return name
		}
	}

	return ""
}

// extractAgentName reads the agent identifier starting at the given string.
// Agent names consist of alphanumeric characters and hyphens (kebab-case).
// Stops at the first invalid character or end of string.
func extractAgentName(s string) string {
	var name strings.Builder
	for _, ch := range s {
		if ch == '-' || unicode.IsLetter(ch) || unicode.IsDigit(ch) {
			name.WriteRune(ch)
		} else {
			break
		}
	}
	return name.String()
}

// tokenize splits a lowercased string into individual words, removing short
// tokens and common noise words that would degrade routing quality.
func tokenize(s string) []string {
	fields := strings.Fields(s)
	// Common stop words to ignore in routing.
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"may": true, "might": true, "can": true, "shall": true, "to": true,
		"of": true, "in": true, "for": true, "on": true, "with": true,
		"at": true, "by": true, "from": true, "as": true, "into": true,
		"about": true, "like": true, "through": true, "after": true,
		"over": true, "between": true, "out": true, "against": true,
		"during": true, "without": true, "before": true, "under": true,
		"around": true, "among": true, "and": true, "but": true, "or": true,
		"if": true, "because": true, "so": true, "than": true, "that": true,
		"this": true, "these": true, "those": true, "it": true, "its": true,
		"i": true, "me": true, "my": true, "we": true, "our": true,
		"you": true, "your": true, "he": true, "she": true, "they": true,
		"them": true, "their": true, "what": true, "which": true, "who": true,
		"how": true, "when": true, "where": true, "why": true,
		"not": true, "no": true, "nor": true, "never": true,
		"just": true, "only": true, "very": true, "too": true, "also": true,
		"please": true, "help": true, "need": true, "want": true,
	}

	result := make([]string, 0, len(fields))
	for _, f := range fields {
		word := strings.TrimFunc(strings.ToLower(f), func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r)
		})
		if len(word) >= 3 && !stopWords[word] {
			result = append(result, word)
		}
	}
	return result
}
