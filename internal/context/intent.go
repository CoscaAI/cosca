package context

import (
	"context"
	"strings"

	"github.com/rs/zerolog"
)

// IntentAnalyzer analyzes user queries to determine intent.
type IntentAnalyzer struct {
	logger zerolog.Logger
}

// NewIntentAnalyzer creates a new intent analyzer.
func NewIntentAnalyzer(logger zerolog.Logger) *IntentAnalyzer {
	return &IntentAnalyzer{
		logger: logger,
	}
}

// AnalyzeIntent analyzes a user query to determine intent and extract entities.
func AnalyzeIntent(ctx context.Context, query string, logger zerolog.Logger) (*IntentResult, error) {
	analyzer := NewIntentAnalyzer(logger)
	return analyzer.Analyze(ctx, query)
}

// Analyze performs intent analysis on the given query.
func (ia *IntentAnalyzer) Analyze(_ context.Context, query string) (*IntentResult, error) {
	result := &IntentResult{
		OriginalQuery: query,
		Intent:        IntentQuestion,
		Confidence:    0.5,
	}

	if query == "" {
		return result, nil
	}

	queryLower := strings.ToLower(query)

	// Determine intent based on keywords and patterns
	result.Intent, result.Confidence = classifyIntent(queryLower)

	// Extract entities (simple heuristic: words that look like identifiers)
	result.Entities = extractEntities(query)

	// Determine required context types
	result.RequiredContext = determineRequiredContext(result.Intent, result.Entities)

	// Build search query
	result.SearchQuery = buildSearchQuery(query, result.Intent, result.Entities)

	ia.logger.Debug().
		Str("intent", string(result.Intent)).
		Float64("confidence", result.Confidence).
		Int("entities", len(result.Entities)).
		Str("search_query", result.SearchQuery).
		Msg("intent analysis complete")

	return result, nil
}

// classifyIntent determines the intent type and confidence from a lowercased query.
func classifyIntent(query string) (IntentType, float64) {
	type intentPattern struct {
		intent   IntentType
		keywords []string
		weight   float64
	}

	patterns := []intentPattern{
		{
			intent:   IntentBug,
			keywords: []string{"bug", "error", "issue", "fix", "broken", "fail", "crash", "exception", "wrong", "incorrect", "not working"},
			weight:   0.7,
		},
		{
			intent:   IntentFeature,
			keywords: []string{"add", "feature", "implement", "new", "create", "build", "support", "enable"},
			weight:   0.6,
		},
		{
			intent:   IntentRefactor,
			keywords: []string{"refactor", "clean", "improve", "optimize", "restructure", "reorganize", "simplify", "extract"},
			weight:   0.7,
		},
		{
			intent:   IntentReview,
			keywords: []string{"review", "audit", "check", "inspect", "verify", "validate", "approve"},
			weight:   0.7,
		},
		{
			intent:   IntentDeploy,
			keywords: []string{"deploy", "release", "publish", "ship", "rollout", "production", "ci", "cd"},
			weight:   0.7,
		},
		{
			intent:   IntentDocs,
			keywords: []string{"doc", "readme", "documentation", "explain", "how", "what is", "guide", "tutorial"},
			weight:   0.6,
		},
		{
			intent:   IntentExplore,
			keywords: []string{"explore", "understand", "architecture", "structure", "overview", "show me", "map"},
			weight:   0.5,
		},
		{
			intent:   IntentSearch,
			keywords: []string{"search", "find", "locate", "where", "lookup", "discover", "show"},
			weight:   0.5,
		},
		{
			intent:   IntentExecute,
			keywords: []string{"run", "execute", "start", "launch", "trigger", "invoke", "call"},
			weight:   0.6,
		},
		{
			intent:   IntentQuestion,
			keywords: []string{"?", "how", "what", "why", "when", "who", "which", "can", "does", "is"},
			weight:   0.5,
		},
	}

	bestScore := 0.0
	bestIntent := IntentQuestion

	for _, pattern := range patterns {
		score := 0.0
		for _, kw := range pattern.keywords {
			if strings.Contains(query, kw) {
				score += pattern.weight
			}
		}
		// Normalize by number of keywords
		if len(pattern.keywords) > 0 {
			score = score / float64(len(pattern.keywords))
		}
		if score > bestScore {
			bestScore = score
			bestIntent = pattern.intent
		}
	}

	// Check for question marks specifically
	if strings.Contains(query, "?") && bestScore < 0.3 {
		return IntentQuestion, 0.8
	}

	return bestIntent, min(bestScore+0.3, 1.0)
}

// extractEntities extracts named entities from the query.
func extractEntities(query string) []string {
	var entities []string
	seen := make(map[string]bool)

	// Split into words and look for identifiers
	words := strings.Fields(query)
	for _, word := range words {
		// Clean punctuation
		word = strings.Trim(word, ".,;:!?\"'()[]{}")

		// Skip short words, common words, and stopwords
		if len(word) < 3 || isStopWord(word) {
			continue
		}

		// Check if it looks like an identifier (camelCase, snake_case, etc.)
		if looksLikeIdentifier(word) && !seen[word] {
			entities = append(entities, word)
			seen[word] = true
		}
	}

	return entities
}

// determineRequiredContext determines what context types are needed based on intent.
func determineRequiredContext(intent IntentType, entities []string) []string {
	contextTypes := []string{"code"}

	switch intent {
	case IntentBug:
		contextTypes = append(contextTypes, "stacktrace", "logs", "recent_changes")
	case IntentFeature:
		contextTypes = append(contextTypes, "architecture", "interfaces", "patterns")
	case IntentRefactor:
		contextTypes = append(contextTypes, "architecture", "dependencies", "tests")
	case IntentReview:
		contextTypes = append(contextTypes, "diff", "history", "tests")
	case IntentDeploy:
		contextTypes = append(contextTypes, "config", "infrastructure", "ci_cd")
	case IntentDocs:
		contextTypes = append(contextTypes, "api", "types", "interfaces")
	case IntentExplore:
		contextTypes = append(contextTypes, "architecture", "modules", "graph")
	case IntentSearch:
		contextTypes = append(contextTypes, "index", "symbols")
	case IntentExecute:
		contextTypes = append(contextTypes, "commands", "scripts", "config")
	default:
		contextTypes = append(contextTypes, "code", "docs")
	}

	if len(entities) > 0 {
		contextTypes = append(contextTypes, "symbols")
	}

	return contextTypes
}

// buildSearchQuery constructs an optimized search query from the original.
func buildSearchQuery(query string, intent IntentType, _ []string) string {
	// For search intent, use the query as-is
	if intent == IntentSearch {
		return query
	}

	// Remove common noise words from the query
	noiseWords := []string{"please", "can", "could", "would", "should", "might", "help", "need", "want"}
	words := strings.Fields(query)
	var filtered []string
	for _, word := range words {
		wordLower := strings.ToLower(word)
		isNoise := false
		for _, noise := range noiseWords {
			if wordLower == noise {
				isNoise = true
				break
			}
		}
		if !isNoise {
			filtered = append(filtered, word)
		}
	}

	return strings.Join(filtered, " ")
}

// isStopWord checks if a word is a common stopword.
func isStopWord(word string) bool {
	stopwords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"but": true, "in": true, "on": true, "at": true, "to": true,
		"for": true, "of": true, "with": true, "by": true, "from": true,
		"as": true, "is": true, "was": true, "are": true, "were": true,
		"be": true, "been": true, "being": true, "have": true, "has": true,
		"had": true, "do": true, "does": true, "did": true, "will": true,
		"would": true, "could": true, "should": true, "may": true, "might": true,
		"shall": true, "can": true, "not": true, "no": true, "nor": true,
		"this": true, "that": true, "these": true, "those": true, "it": true,
		"its": true, "i": true, "me": true, "my": true, "we": true,
		"our": true, "you": true, "your": true, "he": true, "she": true,
		"him": true, "her": true, "they": true, "them": true, "their": true,
		"what": true, "which": true, "who": true, "whom": true, "where": true,
		"when": true, "why": true, "how": true, "all": true, "each": true,
		"every": true, "both": true, "few": true, "more": true, "most": true,
		"some": true, "any": true, "none": true, "one": true, "two": true,
		"get": true, "set": true, "use": true, "using": true, "make": true,
		"let": true, "like": true, "just": true, "also": true, "very": true,
		"too": true, "much": true, "many": true, "such": true, "into": true,
		"over": true, "after": true, "before": true, "between": true, "under": true,
		"about": true, "around": true, "up": true, "down": true, "out": true,
		"off": true, "above": true, "below": true, "here": true, "there": true,
	}
	return stopwords[strings.ToLower(word)]
}

// looksLikeIdentifier checks if a word looks like a code identifier.
func looksLikeIdentifier(word string) bool {
	// Check for camelCase
	if strings.Contains(word, "_") {
		return true // snake_case
	}
	if strings.ToLower(word) != word && strings.ToUpper(word[:1]) != word[:1] {
		return true // camelCase (starts lowercase, has uppercase)
	}
	if strings.ToUpper(word) == word && len(word) <= 8 {
		return true // All uppercase acronym
	}
	// Check for mixed case
	hasUpper := false
	hasLower := false
	for _, ch := range word {
		if ch >= 'A' && ch <= 'Z' {
			hasUpper = true
		}
		if ch >= 'a' && ch <= 'z' {
			hasLower = true
		}
	}
	return hasUpper && hasLower
}
