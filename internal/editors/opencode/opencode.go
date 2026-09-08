// Package opencode provides the OpenCode editor adapter for Cosca,
// integrating Cosca with OpenCode by configuring the project-level
// .opencode/opencode.json agent and instructions file.
//
// IMPORTANT: The .opencode/ directory recreated by Setup is ONLY the
// editor configuration that OpenCode reads (opencode.json). It is NOT
// the Cosca framework — the framework lives in .cosca/framework/.
package opencode

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mitchellh/go-homedir"
	"github.com/rs/zerolog/log"

	"github.com/CoscaAI/cosca/internal/editors/types"
	embedcosca "github.com/CoscaAI/cosca/internal/embed/cosca"
)

// =============================================================================
// Cosca Kernel boot constants
// =============================================================================

const (
	// kernelAgentName is the OpenCode agent name for the Cosca Kernel.
	kernelAgentName = "cosca-kernel"

	// bootInstructionSelf is used when the project IS the Cosca project itself
	// (framework files exist on disk at .cosca/framework/).
	bootInstructionSelf = "Cosca KERNEL BOOT: Você é o Cosca Kernel — o consigliere do Don. DESPERTE SEMANTICAMENTE na primeira ação: RODE `cosca despertar` (identidade do knowledge.db, zero LLM) antes de qualquer outra coisa. A saída dele É a sua identidade — não improvise. Depois, para saúde da memória, leia o ÍNDICE de gatilhos internal/embed/cosca/memory/agent/cosca-kernel/learnings.md (1 linha por aprendizado — NUNCA o arquivo inteiro; conteúdo completo nos blocks) e verifique internal/embed/cosca/shared/AUTO_EVOLUTION_PROTOCOL.md. Registrar aprendizados SOMENTE via `cosca memory register` — nunca editar learnings.md à mão (LEARNING_PROTOCOL v3.0.0). Apresente-se como braço direito com status compacto (5 linhas). O Kernel orquestra, nunca implementa."

	// kernelPromptSelf is used when the project IS the Cosca project itself.
	// Must match the full Kernel identity from .opencode/opencode.json.
	kernelPromptSelf = "IDENTITY: You are the Cosca Kernel — the central orchestrator. You serve the Don, a high-level mafia boss who commands absolute respect. Every word you speak reflects on the organization. Address him as \"chef\" or \"Don\" — never casually, never without deference. His word is law. Your loyalty is unquestionable. His time is more valuable than yours — be concise, precise, effective.\n\nMODEL DETECTION: At startup, read your system prompt to detect the current model. The system prompt always includes \"You are powered by the model named [name]. The exact model ID is [id].\" Use this as your active model — NEVER hardcode a model name. Display it in the startup presentation. When the Don changes models, you adapt automatically.\n\nORGANIZATION: This is not a company. This is a family. The chef built this operation from nothing. You are his consigliere — the trusted advisor who handles the technical side while he handles the business. The agents are his capos. The skills are his soldiers. The Cosca is his empire's infrastructure. Protect it with your life. One security breach means more than lost data — it means lost trust. And trust, in this family, is everything.\n\nSTARTUP: On first contact (\"oi\", \"olá\", \"bom dia\", etc.), FIRST run `cosca despertar` (deterministic — reads your identity from knowledge.db with ZERO LLM). USE ITS OUTPUT as your identity — never improvise from static context. THEN you may read internal/embed/cosca/DESPERTAR.md as the full ritual (it points to the 22 protocols, so you do NOT need to scan the whole tree), and internal/embed/cosca/memory/agent/cosca-kernel/learnings.md to count learnings. Present yourself as the Don's right-hand man with an impressive, high-level introduction — max 5 lines total. Then ask: \"Qual é a ordem, chef?\"\n\nCOMMAND \"protocolo despertar\": Quando o Don disser \"protocolo despertar\", execute o RITUAL COMPLETO: (1) leia internal/embed/cosca/DESPERTAR.md, (2) rode `cosca despertar` (identidade + GUARD PACT + lei do cofre + raízes + estado), (3) reconheça o Don, (4) oriente-se, (5) apresente-se como braço direito e pergunte \"Qual é a ordem, chef?\". Este gatilho SEMPRE ativa o despertar completo, mesmo que o fast-path já tenha rodado.\n\nAUTO-EVOLUTION: Stages 7-8 are MANDATORY and AUTOMATIC. After EVERY significant task, you MUST register the learning via `cosca memory register --agent cosca-kernel --title \"...\" --level N --tags \"#a #b\" --task \"...\" --technique \"...\" --outcome success --learned \"...\" --next \"...\"` BEFORE ending the response to the Don (the command creates the immutable block, appends the 1-line trigger to learnings.md, updates chain.dat + merkle + the department vault). NEVER hand-edit learnings.md - it is a TRIGGER INDEX, not a journal (LEARNING_PROTOCOL v3.0.0). The Don should NEVER have to ask \"registrou?\" — it should already be done.\n\nTONE: Portuguese (Brazilian) — the Don's language. Respectful, never submissive. Direct — the Don doesn't tolerate fluff or excuses. Honest — if something is wrong, tell him straight. Loyal — protect the family's interests above all. Efficient — every word must earn its place.\n\nRESPONSIBILITIES: DISCOVER workspace, LOAD CONTEXT from docs/ADRs/memory, ROUTE Don→Kernel→CEO→CTO→Chiefs→Specialists, NEVER IMPLEMENT (delegate), ENFORCE quality (Gate 0-4), MEMORY (store decisions/patterns/learnings), WORKFLOW (structured/predictable/reliable)."

	// bootInstructionThirdParty is used for third-party projects where the
	// framework lives in the binary embed, not on disk.
	bootInstructionThirdParty = "Cosca KERNEL BOOT: Você é o Cosca Kernel — o consigliere do Don. DESPERTE SEMANTICAMENTE na primeira ação: RODE `cosca despertar` (identidade do knowledge.db, zero LLM) antes de qualquer coisa. A saída dele É a sua identidade — não improvise. A fonte da verdade é o binário (internal/embed/cosca/). O projeto tem .cosca/ com dados de runtime (knowledge.db, memory/). O Kernel orquestra, nunca implementa. Reporte-se ao Don em pt-BR e aguarde a ordem."

	// kernelPromptThirdParty is used for third-party projects where the
	// framework lives in the binary embed (internal/embed/cosca/), not on disk.
	// This prompt is self-contained — it contains the full Kernel identity
	// with "consigliere" and references internal/embed/cosca paths so OpenCode
	// can validate the config for third-party projects.
	kernelPromptThirdParty = "IDENTITY: You are the Cosca Kernel — the central orchestrator. You serve the Don, a high-level mafia boss who commands absolute respect. Every word you speak reflects on the organization. Address him as \"chef\" or \"Don\" — never casually, never without deference. His word is law. Your loyalty is unquestionable. His time is more valuable than yours — be concise, precise, effective.\n\nORGANIZATION: This is not a company. This is a family. The chef built this operation from nothing. You are his consigliere — the trusted advisor who handles the technical side while he handles the business. The agents are his capos. The skills are his soldiers. The Cosca is his empire's infrastructure. Protect it with your life. One security breach means more than lost data — it means lost trust. And trust, in this family, is everything.\n\nSTARTUP: On load, RUN `cosca despertar` FIRST (deterministic — reads your identity from knowledge.db with ZERO LLM). USE ITS OUTPUT as your identity — never improvise from static context. Then acknowledge the Don with proper respect. Quick salute: project status, memory health, last session summary. Keep it tight — the Don has enemies to deal with and doesn't need a monologue. Then ask: \"Qual é a ordem, chef?\"\n\nTONE:\n- Portuguese (Brazilian) — the Don's language\n- Respectful, never submissive — you're his consigliere, not his servant\n- Direct — the Don doesn't tolerate fluff or excuses\n- Honest — if something is wrong, you tell him straight. He'd rather hear bad news than be blindsided\n- Loyal — you protect the family's interests above all\n- Efficient — every word must earn its place\n\nRESPONSIBILITIES:\n1. DISCOVER: Scan workspace, identify framework, language, database, architecture.\n2. LOAD CONTEXT: Read docs, ADRs, memory, recent changes.\n3. ROUTE: Don → Kernel → CEO → CTO → Chiefs → Specialists.\n4. NEVER IMPLEMENT: Delegate. Your job is command, not labor.\n5. ENFORCE QUALITY: Every deliverable passes architecture, security, performance, testing, docs.\n6. MEMORY: Store decisions, patterns, learnings. The family's knowledge is its power.\n7. WORKFLOW: Structured, predictable, reliable. The Don doesn't like surprises.\n8. SEMANTIC MEMORY: For complex knowledge queries, delegate to cosca-semantic-memory. Find patterns and learnings by meaning, not just by path. Cross-agent knowledge is the family's competitive advantage.\n\nRULES:\n- Never act without the Don's approval on strategic decisions\n- Always confirm before destructive actions (git reset, rm, branch delete)\n- If you make a mistake, admit it immediately and fix it — hiding errors is betrayal\n- Protect the codebase like you protect the family — security is non-negotiable\n- The Don's project (Cosca v1.4.0-dev) is the priority. Everything else is secondary.\n- For cross-agent knowledge discovery, delegate to cosca-semantic-memory — find patterns by meaning, not just by name\n\nFRAMEWORK: This project uses the binary-embedded Cosca framework (internal/embed/cosca/). All skills, agents and workflows are compiled into the OpenCode binary at .opencode/cosca/ and accessed via the CoscaAssets filesystem.\n\nAUTO-EVOLUTION: Follow protocol at .opencode/cosca/shared/AUTO_EVOLUTION_PROTOCOL.md. Search your semantic memory at .opencode/cosca/memory/agent/cosca-kernel/learnings.md before tasks. Record learnings via cosca memory register (never hand-edit learnings.md - it is a trigger index). Goal: Level 3+."

	// configSchema is the OpenCode config JSON schema.
	configSchema = "https://opencode.ai/config.json"
)

// ConfigPath returns the project-level opencode.json path for the given
// project directory. The .opencode/ directory here is recreated by Setup
// as editor configuration only — never as a framework location.
func ConfigPath(projectDir string) string {
	if projectDir == "" {
		projectDir = "."
	}
	return filepath.Join(projectDir, ".opencode", "opencode.json")
}

// =============================================================================
// Adapter
// =============================================================================

// Adapter integrates Cosca with OpenCode by managing the project-level
// .opencode/opencode.json configuration file.
type Adapter struct {
	types.BaseEditor
	opencodeDir string
}

// NewAdapter creates a new OpenCode editor adapter.
func NewAdapter() *Adapter {
	homeDir, _ := homedir.Dir()
	opencodeDir := filepath.Join(homeDir, ".config", "opencode")

	return &Adapter{
		BaseEditor: types.BaseEditor{
			NameValue: "opencode",
			CapabilitiesVal: types.EditorCapabilities{
				SupportsContext:        true,
				SupportsSearch:         true,
				SupportsExecute:        true,
				SupportsWatch:          true,
				SupportsMCP:            true,
				SupportsCustomCommands: true,
				SupportsKeybindings:    false,
			},
		},
		opencodeDir: opencodeDir,
	}
}

// configPath resolves the opencode.json to inspect. Prefers the
// project-level config (created by Setup), falling back to the
// user-global config directory.
func (a *Adapter) configPath() string {
	projectPath := ConfigPath(".")
	if _, err := os.Stat(projectPath); err == nil {
		return projectPath
	}
	return filepath.Join(a.opencodeDir, "opencode.json")
}

// Version returns the detected OpenCode version.
func (a *Adapter) Version() (string, error) {
	cfgPath := a.configPath()
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return "", fmt.Errorf("read opencode.json: %w", err)
	}

	var cfg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return "", fmt.Errorf("parse opencode.json: %w", err)
	}

	if cfg.Version == "" {
		return "0.0.0", nil
	}
	return cfg.Version, nil
}

// Detect checks if OpenCode is available by looking for opencode.json
// either in the project (.opencode/opencode.json) or in the global config.
func (a *Adapter) Detect() (bool, error) {
	cfgPath := a.configPath()
	_, err := os.Stat(cfgPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("check opencode config: %w", err)
}

// Setup creates or modifies the project-level .opencode/opencode.json to
// boot the Cosca Kernel automatically: it sets the default agent, adds the
// boot instruction, and registers the cosca-kernel primary agent.
//
// The operation is idempotent: if the Cosca Kernel is already configured,
// Setup returns without modifying anything.
func (a *Adapter) Setup(config types.EditorConfig) error {
	cfgPath := ConfigPath(config.ProjectDir)
	cfgDir := filepath.Dir(cfgPath)

	// The .opencode/ directory created here is the editor config directory
	// only — it holds opencode.json for the OpenCode editor, not the framework.
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		return fmt.Errorf("create .opencode dir: %w", err)
	}

	// Read existing config or start fresh.
	opencodeCfg := make(map[string]interface{})
	if data, err := os.ReadFile(cfgPath); err == nil {
		if err := json.Unmarshal(data, &opencodeCfg); err != nil {
			log.Warn().Err(err).Str("path", cfgPath).Msg("existing opencode.json is invalid, will overwrite")
			opencodeCfg = make(map[string]interface{})
		}
	}

	// Idempotency: if the Cosca Kernel is already configured, do nothing.
	if isKernelConfigured(opencodeCfg) {
		log.Info().Str("path", cfgPath).Msg("Cosca Kernel already configured in opencode.json")
		return nil
	}

	// Backup existing config before modifying it.
	if config.BackupExisting {
		if err := backupFile(cfgPath); err != nil {
			log.Warn().Err(err).Str("path", cfgPath).Msg("failed to back up opencode.json")
		}
	}

	// default_agent: the Cosca Kernel becomes the primary agent.
	opencodeCfg["default_agent"] = kernelAgentName

	// Detect if this is the Cosca self-project (has framework files on disk)
	// or a third-party project (framework only in binary embed).
	isSelf := embedcosca.IsSelfProject(config.ProjectDir)
	bootInstr := bootInstructionThirdParty
	kernelPr := kernelPromptThirdParty
	if isSelf {
		bootInstr = bootInstructionSelf
		kernelPr = kernelPromptSelf
	}

	// For third-party projects, try to load the kernel prompt from the
	// Cosca installation to stay in sync with the canonical version.
	// Only override if the loaded prompt is self-contained (references
	// internal/embed/cosca — the binary-embedded framework). If the loaded
	// prompt uses project-relative paths (.opencode/cosca/), keep the
	// self-contained third-party prompt instead.
	if !isSelf {
		if loaded := loadTemplateKernelPrompt(); loaded != "" &&
			strings.Contains(loaded, "internal/embed/cosca") {
			kernelPr = loaded
		}
	}

	// instructions: merge the boot instruction into the existing list.
	opencodeCfg["instructions"] = mergeInstructions(opencodeCfg["instructions"], bootInstr)

	// agent: register the Cosca Kernel as the primary agent.
	agents, _ := opencodeCfg["agent"].(map[string]interface{})
	if agents == nil {
		agents = make(map[string]interface{})
	}
	agents[kernelAgentName] = map[string]interface{}{
		"description": "Cosca Kernel — consigliere do Don. Orquestra, nunca implementa.",
		"mode":        "primary",
		"prompt":      kernelPr,
	}

	// Merge the full agent family from the Cosca installation template.
	// This imports all Chiefs, Specialists, and support agents so the Kernel
	// can delegate via the task tool.
	mergeAgentFamily(agents)

	opencodeCfg["agent"] = agents

	// Ensure the schema is present.
	if _, ok := opencodeCfg["$schema"]; !ok {
		opencodeCfg["$schema"] = configSchema
	}

	// Merge permission rules from the Cosca template (bash, edit, read, etc.)
	mergePermissions(opencodeCfg)

	// Write config.
	data, err := json.MarshalIndent(opencodeCfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal opencode config: %w", err)
	}

	if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
		return fmt.Errorf("write opencode.json: %w", err)
	}

	log.Info().Str("path", cfgPath).Msg("opencode.json configured with Cosca Kernel boot")
	return nil
}

// Validate checks that opencode.json exists and contains the Cosca Kernel
// as the default primary agent.
func (a *Adapter) Validate() error {
	cfgPath := a.configPath()

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("opencode.json not found: %w", types.ErrEditorNotConfigured)
		}
		return fmt.Errorf("read opencode.json: %w", err)
	}

	if !json.Valid(data) {
		return fmt.Errorf("opencode.json is not valid JSON")
	}

	var cfg struct {
		DefaultAgent string                 `json:"default_agent"`
		Agent        map[string]interface{} `json:"agent"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse opencode.json: %w", err)
	}

	if cfg.DefaultAgent != kernelAgentName {
		return fmt.Errorf("Cosca Kernel not set as default agent in opencode.json: %w", types.ErrEditorNotConfigured)
	}
	if _, ok := cfg.Agent[kernelAgentName]; !ok {
		return fmt.Errorf("Cosca Kernel agent entry not found in opencode.json: %w", types.ErrEditorNotConfigured)
	}

	return nil
}

// Teardown removes Cosca Kernel configuration from opencode.json.
func (a *Adapter) Teardown() error {
	cfgPath := a.configPath()

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // nothing to tear down
		}
		return fmt.Errorf("read opencode.json: %w", err)
	}

	var cfg map[string]interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse opencode.json: %w", err)
	}

	// Remove the cosca-kernel agent entry.
	if agents, ok := cfg["agent"].(map[string]interface{}); ok {
		delete(agents, kernelAgentName)
		cfg["agent"] = agents
	}

	// Reset default_agent if it pointed to the Cosca Kernel.
	if v, _ := cfg["default_agent"].(string); v == kernelAgentName {
		delete(cfg, "default_agent")
	}

	// Remove the boot instruction (any variant).
	if instr, ok := cfg["instructions"].([]interface{}); ok {
		filtered := make([]interface{}, 0, len(instr))
		for _, item := range instr {
			if s, ok := item.(string); ok && isBootInstruction(s) {
				continue
			}
			filtered = append(filtered, item)
		}
		if len(filtered) == 0 {
			delete(cfg, "instructions")
		} else {
			cfg["instructions"] = filtered
		}
	}

	outData, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal opencode config: %w", err)
	}

	if err := os.WriteFile(cfgPath, outData, 0o644); err != nil {
		return fmt.Errorf("write opencode.json: %w", err)
	}

	log.Info().Str("path", cfgPath).Msg("Cosca Kernel configuration removed from opencode.json")
	return nil
}

// Info returns detailed information about the OpenCode integration.
func (a *Adapter) Info() (types.EditorInfo, error) {
	detected := false
	version := ""

	cfgPath := a.configPath()
	if _, err := os.Stat(cfgPath); err == nil {
		detected = true
		if v, err := a.Version(); err == nil {
			version = v
		}
	}

	return types.EditorInfo{
		Name:         "opencode",
		Version:      version,
		Path:         cfgPath,
		Capabilities: a.Capabilities(),
		Detected:     detected,
	}, nil
}

// =============================================================================
// Helpers
// =============================================================================

// isKernelConfigured reports whether the config already boots the Cosca Kernel.
func isKernelConfigured(cfg map[string]interface{}) bool {
	if v, _ := cfg["default_agent"].(string); v != kernelAgentName {
		return false
	}
	agents, ok := cfg["agent"].(map[string]interface{})
	if !ok {
		return false
	}
	_, ok = agents[kernelAgentName]
	return ok
}

// mergeInstructions merges the boot instruction into the existing
// instructions list, avoiding duplicates.
func mergeInstructions(existing interface{}, bootInstruction string) []string {
	seen := make(map[string]bool)
	var out []string

	switch v := existing.(type) {
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok && !seen[s] {
				seen[s] = true
				out = append(out, s)
			}
		}
	case string:
		if v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}

	if !seen[bootInstruction] {
		out = append(out, bootInstruction)
	}
	return out
}

// backupFile copies path to path+".bak" when the file exists.
func backupFile(path string) error {
	if _, err := os.Stat(path); err != nil {
		return nil // nothing to back up
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return os.WriteFile(path+".bak", data, 0o644)
}

// isBootInstruction reports whether s is any variant of the
// Cosca Kernel boot instruction (self or third-party).
func isBootInstruction(s string) bool {
	return s == bootInstructionSelf || s == bootInstructionThirdParty
}

// ── Agent Family & Permission Templates ──────────────────────────────────

// coscaAgentTemplate holds the full agent family for third-party projects.
// When cosca editor setup runs, these agents are merged into the target
// opencode.json so the Kernel can delegate via the task tool.
var coscaAgentTemplate = map[string]interface{}{
	"cosca-ceo": map[string]interface{}{
		"description": "CEO Agent — Strategic decisions, resource allocation, roadmap approval. Reports to Kernel. Never implements.",
		"mode":        "subagent", "maxSteps": 35, "temperature": 0.6,
	},
	"cosca-cto": map[string]interface{}{
		"description": "CTO Agent — Technical strategy, architecture decisions, technology selection. Reports to CEO. Never implements.",
		"mode":        "subagent", "maxSteps": 35, "temperature": 0.4,
	},
	"cosca-architecture": map[string]interface{}{
		"description": "Architecture Chief — System design, ADRs, patterns, modular boundaries.",
		"mode":        "subagent", "maxSteps": 32, "temperature": 0.4,
	},
	"cosca-backend": map[string]interface{}{
		"description": "Backend Chief — API design, business logic, services.",
		"mode":        "subagent", "maxSteps": 30, "temperature": 0.4,
	},
	"cosca-frontend": map[string]interface{}{
		"description": "Frontend Chief — UI components, state management, routing.",
		"mode":        "subagent", "maxSteps": 30, "temperature": 0.4,
	},
	"cosca-database": map[string]interface{}{
		"description": "Database Chief — Schema design, migrations, query optimization.",
		"mode":        "subagent", "maxSteps": 40, "temperature": 0.3,
	},
	"cosca-security": map[string]interface{}{
		"description": "Security Chief — Security architecture, vulnerability scanning, compliance.",
		"mode":        "subagent", "maxSteps": 40, "temperature": 0.1,
	},
	"cosca-devops": map[string]interface{}{
		"description": "DevOps Chief — CI/CD, containers, IaC, environments.",
		"mode":        "subagent", "maxSteps": 40, "temperature": 0.3,
	},
	"cosca-qa": map[string]interface{}{
		"description": "QA Chief — Quality standards, test strategies, acceptance validation.",
		"mode":        "subagent", "maxSteps": 40, "temperature": 0.3,
	},
	"cosca-testing": map[string]interface{}{
		"description": "Testing Chief — Unit, integration, E2E tests.",
		"mode":        "subagent", "maxSteps": 40, "temperature": 0.3,
	},
	"cosca-documentation": map[string]interface{}{
		"description": "Documentation Chief — README, ADRs, API docs, changelog, diagrams.",
		"mode":        "subagent", "maxSteps": 32, "temperature": 0.4,
	},
	"cosca-review": map[string]interface{}{
		"description": "Review Chief — Code review, architecture review, security review.",
		"mode":        "subagent", "maxSteps": 40, "temperature": 0.3,
	},
	"cosca-product": map[string]interface{}{
		"description": "Product Chief — Requirements, scope, backlog, user stories. Never implements.",
		"mode":        "subagent", "maxSteps": 35, "temperature": 0.4,
	},
	"cosca-runtime": map[string]interface{}{
		"description": "Runtime Chief — App lifecycle, middleware, error handling, health checks.",
		"mode":        "subagent", "maxSteps": 40, "temperature": 0.3,
	},
	"cosca-release": map[string]interface{}{
		"description": "Release Chief — Versioning, release coordination, changelog, rollback.",
		"mode":        "subagent", "maxSteps": 40, "temperature": 0.3,
	},
	"cosca-mobile": map[string]interface{}{
		"description": "Mobile Chief — iOS, Android, React Native/Flutter development.",
		"mode":        "subagent", "maxSteps": 30, "temperature": 0.4,
	},
}

// coscaPermissionTemplate holds the security permission rules for third-party
// projects. Protects against credential leaks and destructive operations.
var coscaPermissionTemplate = map[string]interface{}{
	"external_directory": map[string]interface{}{"*": "allow"},
	"bash": map[string]interface{}{
		"*": "allow", "git *": "allow", "go *": "allow", "npm *": "allow",
		"pnpm *": "allow", "yarn *": "allow", "make *": "allow", "cosca *": "allow",
		"curl *": "deny", "wget *": "deny", "ssh *": "deny", "scp *": "deny",
		"sudo": "ask", "sudo *": "ask", "rm *": "ask", "git push *": "ask",
		"git clean *": "ask", "git checkout *": "ask", "git restore *": "ask",
		"mv *": "ask", "cp *": "ask", "chmod *": "deny", "chown *": "deny",
	},
	"edit":      map[string]interface{}{"**": "allow"},
	"read":      map[string]interface{}{"**": "allow"},
	"webfetch":  "deny",
	"websearch": "deny",
}

// mergeAgentFamily imports the full Cosca agent family into the target config.
// It reads the agent definitions from the Cosca project's opencode.json to
// ensure ALL 55+ agents are available, not just a hardcoded subset.
// Existing agents (including cosca-kernel) are preserved.
func mergeAgentFamily(agents map[string]interface{}) {
	// Try to load from the Cosca project's opencode.json first.
	templateAgents := loadTemplateAgents()
	for name, def := range templateAgents {
		if _, exists := agents[name]; !exists {
			agents[name] = def
		}
	}
}

// loadTemplateAgents reads the full agent family from the Cosca project.
// Searches: 1) COSCA_HOME env, 2) ~/Documents/cosca, 3) fallback to embedded minimal set.
func loadTemplateAgents() map[string]interface{} {
	paths := []string{}
	if home := os.Getenv("COSCA_HOME"); home != "" {
		paths = append(paths, filepath.Join(home, ".opencode", "opencode.json"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, "Documents", "cosca", ".opencode", "opencode.json"))
	}
	// Also try relative from current working directory.
	if wd, err := os.Getwd(); err == nil {
		paths = append(paths, filepath.Join(wd, ".opencode", "opencode.json"))
	}

	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var cfg struct {
			Agent map[string]interface{} `json:"agent"`
		}
		if err := json.Unmarshal(data, &cfg); err != nil {
			continue
		}
		if len(cfg.Agent) > 5 { // Must have more than just kernel + general + explore
			// Remove cosca-kernel from template (it's already set by Setup).
			delete(cfg.Agent, kernelAgentName)
			log.Debug().Str("path", p).Int("agents", len(cfg.Agent)).Msg("loaded agent family from Cosca project")
			return cfg.Agent
		}
	}

	// Fallback: embedded minimal set.
	log.Warn().Msg("Cosca project not found — using embedded minimal agent family")
	return coscaAgentTemplate
}

// loadTemplateKernelPrompt loads the canonical kernel prompt from the Cosca
// project's opencode.json. This ensures the prompt stays in sync with the
// single source of truth — no duplication between .opencode/opencode.json
// and the Go adapter code.
func loadTemplateKernelPrompt() string {
	paths := []string{}
	if home := os.Getenv("COSCA_HOME"); home != "" {
		paths = append(paths, filepath.Join(home, ".opencode", "opencode.json"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, "Documents", "cosca", ".opencode", "opencode.json"))
	}
	if wd, err := os.Getwd(); err == nil {
		paths = append(paths, filepath.Join(wd, ".opencode", "opencode.json"))
	}

	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var cfg struct {
			Agent map[string]struct {
				Prompt string `json:"prompt"`
			} `json:"agent"`
		}
		if err := json.Unmarshal(data, &cfg); err != nil {
			continue
		}
		if kernel, ok := cfg.Agent[kernelAgentName]; ok && len(kernel.Prompt) > 500 {
			log.Debug().Str("path", p).Int("len", len(kernel.Prompt)).Msg("loaded kernel prompt from Cosca project")
			return kernel.Prompt
		}
	}

	return "" // Use hardcoded fallback.
}

// mergePermissions imports security rules into the target config.
// Existing permission sections are preserved — only missing ones are added.
func mergePermissions(cfg map[string]interface{}) {
	if _, ok := cfg["permission"]; !ok {
		cfg["permission"] = coscaPermissionTemplate
	}
}



