package orchestration

import (
	"github.com/rs/zerolog/log"
)

// FactoryConfig bundles all dependencies needed to create an orchestration
// Engine. Use NewFactory instead of calling NewEngine directly — it provides
// a single, consistent wiring point that reduces duplication across call sites.
type FactoryConfig struct {
	Knowledge      KnowledgeSearcher
	MemoryRetriever MemoryRetriever
	MemoryStorer   MemoryStorer
	AgentResolver  AgentResolver
	SkillResolver  SkillResolver
	ChatProvider   ChatProvider
	Config         OrchestratorConfig
	Metrics        *OrchestrationMetrics
	// WorkspaceDir is the root directory for tool execution. When set, the
	// engine wires a ToolExecutor (search_codebase, read_file, etc.).
	WorkspaceDir string
}

// NewFactory creates an orchestration Engine from a FactoryConfig struct.
// This is the preferred way to create an Engine — it centralises the wiring
// and makes the dependency list explicit.
func NewFactory(cfg FactoryConfig) *Engine {
	cfg.Config.WorkspaceDir = cfg.WorkspaceDir
	eng := NewEngine(
		cfg.Knowledge, cfg.MemoryRetriever, cfg.MemoryStorer,
		cfg.AgentResolver, cfg.SkillResolver, cfg.ChatProvider,
		cfg.Config, cfg.Metrics,
	)
	if cfg.WorkspaceDir != "" {
		log.Info().Str("workspace_dir", cfg.WorkspaceDir).Msg("factory: tool executor workspace wired")
	}
	return eng
}
