package pipeline

import (
	"fmt"
	"strings"

	"github.com/CoscaAI/cosca/internal/chat"
)

// ─── ModelPreferences ────────────────────────────────────────────────────

type ModelPreferences struct {
	FastModel  string
	SmartModel string
	LocalModel string
	CodeModel  string
}

func DefaultModelPreferences() ModelPreferences {
	return ModelPreferences{
		FastModel:  "gpt-4o-mini",
		SmartModel: "gpt-4o",
		LocalModel: "ollama",
		CodeModel:  "qwen2.5-coder",
	}
}

// ─── ModelChoice ─────────────────────────────────────────────────────────

type ModelChoice struct {
	Provider string  `json:"provider"`
	Model    string  `json:"model"`
	Reason   string  `json:"reason"`
	CostEst  float64 `json:"cost_est"`
}

// ─── ModelRouter ─────────────────────────────────────────────────────────

type ModelRouter struct {
	registry    *chat.ChatRegistry
	preferences ModelPreferences
}

func NewModelRouter(registry *chat.ChatRegistry) *ModelRouter {
	return &ModelRouter{
		registry:    registry,
		preferences: DefaultModelPreferences(),
	}
}

func NewModelRouterWithPrefs(registry *chat.ChatRegistry, prefs ModelPreferences) *ModelRouter {
	return &ModelRouter{
		registry:    registry,
		preferences: prefs,
	}
}

// Select picks the best model based on task characteristics and available
// providers in the registry.
func (r *ModelRouter) Select(task *TaskNode, ctx *GeneralContext) *ModelChoice {
	if r.registry == nil {
		return r.fallback()
	}

	complexity := r.assessComplexity(task)

	switch {
	case r.isCodeTask(task):
		return r.selectByCategory("code", r.preferences.CodeModel, complexity, "code generation task")
	case complexity == "high":
		return r.selectByCategory("smart", r.preferences.SmartModel, complexity, "high complexity task")
	case r.isSensitiveTask(task):
		return r.selectByCategory("local", r.preferences.LocalModel, complexity, "privacy-sensitive task")
	default:
		return r.selectByCategory("fast", r.preferences.FastModel, complexity, "default simple task")
	}
}

// ─── Routing Logic ───────────────────────────────────────────────────────

func (r *ModelRouter) assessComplexity(task *TaskNode) string {
	if task == nil {
		return "low"
	}

	score := 0
	if len(task.Description) > 200 {
		score += 2
	}
	if len(task.DependsOn) > 1 {
		score += 2
	}
	if len(task.InputFiles) > 3 || len(task.OutputFiles) > 3 {
		score += 2
	}
	if task.Priority > 5 {
		score += 1
	}

	switch {
	case score >= 4:
		return "high"
	case score >= 2:
		return "medium"
	default:
		return "low"
	}
}

var codeKeywords = []string{
	"code", "implement", "refactor", "function", "class", "module",
	"create", "generate", "write", "build", "add", "fix",
	"handler", "endpoint", "api", "middleware", "route",
	"component", "test", "migration", "schema", "model",
}

func (r *ModelRouter) isCodeTask(task *TaskNode) bool {
	if task == nil {
		return false
	}
	lower := strings.ToLower(task.Description)
	for _, kw := range codeKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

var sensitiveKeywords = []string{
	"password", "secret", "token", "key", "credential",
	"private", "confidential", "pii", "personal", "sensitive",
	"encrypt", "decrypt", "hash",
}

func (r *ModelRouter) isSensitiveTask(task *TaskNode) bool {
	if task == nil {
		return false
	}
	lower := strings.ToLower(task.Description)
	for _, kw := range sensitiveKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func (r *ModelRouter) selectByCategory(category, preferred string, complexity, reason string) *ModelChoice {
	registered := r.registry.List()

	// Try exact preferred model first
	if preferred != "" {
		if choice := r.matchProviderName(preferred, registered, category, complexity, reason); choice != nil {
			return choice
		}
	}

	// Try category-appropriate fallbacks
	fallbacks := r.categoryFallbacks(category)
	for _, fb := range fallbacks {
		if choice := r.matchProviderName(fb, registered, category, complexity, reason); choice != nil {
			return choice
		}
	}

	// Use first available provider
	for _, name := range registered {
		if provider, ok := r.registry.Get(name); ok {
			model := provider.Model()
			return &ModelChoice{
				Provider: name,
				Model:    model,
				Reason:   fmt.Sprintf("configured model %s (%s, complexity: %s)", model, reason, complexity),
				CostEst:  estimateCost(model),
			}
		}
	}

	return r.fallback()
}

func (r *ModelRouter) matchProviderName(target string, registered []string, category, complexity, reason string) *ModelChoice {
	targetLower := strings.ToLower(target)
	for _, name := range registered {
		if strings.EqualFold(name, target) || strings.Contains(strings.ToLower(name), targetLower) {
			if provider, ok := r.registry.Get(name); ok {
				return &ModelChoice{
					Provider: name,
					Model:    provider.Model(),
					Reason:   fmt.Sprintf("%s (%s, complexity: %s)", reason, category, complexity),
					CostEst:  estimateCost(provider.Model()),
				}
			}
		}
	}

	for _, name := range registered {
		if provider, ok := r.registry.Get(name); ok {
			model := provider.Model()
			if strings.Contains(strings.ToLower(model), targetLower) {
				return &ModelChoice{
					Provider: name,
					Model:    model,
					Reason:   fmt.Sprintf("%s (matched model %s, complexity: %s)", reason, model, complexity),
					CostEst:  estimateCost(model),
				}
			}
		}
	}

	return nil
}

func (r *ModelRouter) categoryFallbacks(category string) []string {
	switch category {
	case "code":
		return []string{"deepseek", "gpt-4o", "claude-sonnet", "gpt-4o-mini"}
	case "smart":
		return []string{"gpt-4o", "claude-sonnet", "deepseek", "gpt-4o-mini"}
	case "local":
		return []string{"ollama", "gpt-4o-mini", "gpt-4o"}
	case "fast":
		return []string{"gpt-4o-mini", "claude-haiku", "deepseek", "gpt-4o"}
	default:
		return []string{"gpt-4o-mini", "gpt-4o"}
	}
}

func (r *ModelRouter) fallback() *ModelChoice {
	return &ModelChoice{
		Provider: "unknown",
		Model:    "gpt-4o-mini",
		Reason:   "no registry available, using default",
		CostEst:  0.01,
	}
}

// ─── Cost Estimation ─────────────────────────────────────────────────────

func estimateCost(model string) float64 {
	lower := strings.ToLower(model)

	switch {
	case strings.Contains(lower, "gpt-4o-mini"):
		return 0.15
	case strings.Contains(lower, "gpt-4o"):
		return 2.50
	case strings.Contains(lower, "gpt-4"):
		return 10.00
	case strings.Contains(lower, "claude-sonnet") || strings.Contains(lower, "claude-3.5"):
		return 3.00
	case strings.Contains(lower, "claude-haiku"):
		return 0.25
	case strings.Contains(lower, "claude-opus"):
		return 15.00
	case strings.Contains(lower, "deepseek"):
		return 0.14
	case strings.Contains(lower, "ollama"):
		return 0.0
	case strings.Contains(lower, "gemini"):
		return 0.50
	default:
		return 1.0
	}
}
