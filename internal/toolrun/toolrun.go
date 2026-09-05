// Package toolrun monta o EXECUTOR DE FERRAMENTAS CANÔNICO do COSCA
// (chat/executor) com o registry real de tools, sandbox gate e permission
// ruleset — o ÚNICO ponto de montagem usado por todos os caminhos que precisam
// executar ferramentas (serve/chat/run/terminal/exec).
//
// Opção B (Etapa 3): antes havia dois executores — o chat/executor (com
// sandbox+policy+permission+level) e um ToolExecutor interno do orchestration
// (com tools hardcoded read_file/execute_command... SEM gates de segurança).
// Este pacote é a resposta "um executor, um vocabulário, todos os gates".
package toolrun

import (
	"strings"

	"github.com/CoscaAI/cosca/internal/chat"
	chatconfig "github.com/CoscaAI/cosca/internal/chat/config"
	"github.com/CoscaAI/cosca/internal/chat/executor"
	"github.com/CoscaAI/cosca/internal/chat/sandbox"
	"github.com/CoscaAI/cosca/internal/chat/tool"
	"github.com/CoscaAI/cosca/internal/chat/tools/filesystem"
	"github.com/CoscaAI/cosca/internal/orchestration"
	"github.com/CoscaAI/cosca/internal/policy"
	"github.com/rs/zerolog/log"
)

// Config configura a montagem do executor canônico.
type Config struct {
	// Workspace é o root do projeto (rails de path + sandbox).
	Workspace string
	// CoscaDir é o data dir (<workspace>/.cosca) — reservado para tools que
	// dependam dele (plugins/MCP no futuro).
	CoscaDir string
	// Gate é o sandbox gate explícito. Nil = monta o gate default do workspace
	// a partir do sandbox mode da config de chat.
	Gate *sandbox.Gate
}

// Build monta o executor canônico (chat/executor) com o registry real de
// tools, o sandbox gate (explícito ou derivado da config) e a permission
// ruleset. Devolve um orchestration.ToolRunner pronto para injetar no
// orchestration (config.ToolRunner). Devolve nil quando a montagem não é
// possível (o orchestration roda sem execução de tools, sem quebrar).
func Build(cfg Config) orchestration.ToolRunner {
	chatCfg, err := loadChatConfig(cfg.Workspace)
	if err != nil {
		log.Warn().Err(err).Msg("toolrun: failed to load chat config, using defaults")
	}

	// 1. Registry canônico + sandbox gate.
	toolRegistry := tool.NewRegistry()
	rails := sandbox.NewRails(cfg.Workspace)

	var sbGate *sandbox.Gate
	if cfg.Gate != nil {
		sbGate = cfg.Gate
	} else {
		sbGate = sandbox.NewGate(cfg.Workspace, ParseSandboxMode(chatCfg.Sandbox.Mode))
	}

	// 2. Tools reais (o vocabulário ÚNICO do COSCA).
	filesystem.Register(toolRegistry, cfg.Workspace)
	toolRegistry.Register(tool.NewShellTool(cfg.Workspace, sbGate))
	toolRegistry.Register(tool.NewSearchTool(cfg.Workspace, rails, sbGate))
	toolRegistry.Register(tool.NewGitTool(cfg.Workspace, sbGate))
	toolRegistry.Register(tool.NewBuildTool(cfg.Workspace, sbGate))
	toolRegistry.Register(tool.NewTestTool(cfg.Workspace, sbGate))

	// 3. Executor canônico com permission ruleset (allow/ask/deny).
	ex := executor.New(toolRegistry, sbGate, cfg.Workspace)
	ex.SetPermission(chatCfg.PermissionRuleset())

	// 4. Guard de política determinístico (L366 / GOVERNANCE_PROTOCOL §1):
	// as regras padrão da casa (anti-exfiltração, argument-aware deny) são
	// ANEXADAS ao executor canônico. Antes (R3, relatório 08-17) o SetPolicy
	// existia mas NENHUM caminho de produção o chamava — o guard ficava
	// desligado. Agora TODO executor montado pelo toolrun o tem.
	ex.SetPolicy(policy.New())

	_ = cfg.CoscaDir // reservado para tools que dependam do data dir

	log.Debug().Strs("tools", toolRegistry.List()).Msg("toolrun: canonical executor wired")
	return orchestration.NewToolRunner(ex)
}

// loadChatConfig carrega a config de chat do projeto (sandbox mode, permission
// ruleset). Fallback para defaults quando a config não existe.
func loadChatConfig(workspace string) (*chatconfig.Config, error) {
	cfg, err := chatconfig.Load(".")
	if err != nil {
		d := chatconfig.DefaultConfig()
		return &d, nil
	}
	return cfg, nil
}

// ParseSandboxMode converte a string de sandbox mode da config para o enum
// canônico chat.SandboxMode (default: workspace).
func ParseSandboxMode(mode string) chat.SandboxMode {
	switch strings.ToLower(mode) {
	case "read-only", "readonly":
		return chat.SandboxReadOnly
	case "full":
		return chat.SandboxFull
	default:
		return chat.SandboxWorkspace
	}
}
