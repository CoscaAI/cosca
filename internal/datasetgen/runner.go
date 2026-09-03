package datasetgen

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/CoscaAI/cosca/internal/chat"
	cexecutor "github.com/CoscaAI/cosca/internal/chat/executor"
	"github.com/CoscaAI/cosca/internal/chat/provider"
	"github.com/CoscaAI/cosca/internal/chat/sandbox"
	"github.com/CoscaAI/cosca/internal/chat/tool"
	"github.com/CoscaAI/cosca/internal/chat/tools/filesystem"
	"github.com/CoscaAI/cosca/internal/policy"
)

// ─── Configuração do gerador ─────────────────────────────────────────────────

// GeneratorConfig configura a execução de um exemplo.
type GeneratorConfig struct {
	// Model é o modelo aluno (ex.: "qwen3:4b") que executa a trajetória.
	Model string
	// BaseURL é o endpoint do Ollama. Vazio → default (http://127.0.0.1:11434).
	BaseURL string
	// MaxRounds é o limite de rounds de tool-call por exemplo.
	MaxRounds int
	// Timeout é o limite de tempo por exemplo.
	Timeout time.Duration
	// NumCtx é a janela de contexto (num_ctx) enviada ao Ollama. Em modelo
	// local de 4B, janela GIGANTE (32768+) = processamento de prompt lento =>
	// timeout. Para o datasetgen (tarefas curtas), um valor menor é o ideal.
	// Default: 8192.
	NumCtx int
	// WorkDir é o diretório base para os workspaces temporários isolados.
	WorkDir string
}

// DefaultGeneratorConfig retorna a config padrão (modelo qwen3:4b).
func DefaultGeneratorConfig() GeneratorConfig {
	return GeneratorConfig{
		Model:     "qwen3:4b",
		MaxRounds: 8,
		Timeout:   180 * time.Second,
		NumCtx:    8192,
		WorkDir:   filepath.Join(os.TempDir(), "cosca-datasetgen"),
	}
}

// ─── Runner ──────────────────────────────────────────────────────────────────

// Runner executa um TaskSpec contra o executor canônico e produz um Example
// (trajetória capturada + label classificada).
type Runner struct {
	cfg GeneratorConfig
}

// NewRunner cria um Runner com a config dada.
func NewRunner(cfg GeneratorConfig) *Runner { return &Runner{cfg: cfg} }

// GenerateExample executa um TaskSpec e retorna o Example classificado.
func (r *Runner) GenerateExample(ctx context.Context, spec TaskSpec) (*Example, error) {
	// 1. Workspace isolado temporário.
	ws, err := r.newWorkspace(spec)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(ws)

	// 2. Executor canônico (sandbox + policy + evidence gate) + registry.
	inner, reg := r.newCanonicalExecutor(ws)

	// 3. Loop de tool-call.
	ex := &Example{
		Task:          spec.Task,
		InitialState:  spec.InitialState,
		ExpectedState: spec.ExpectedState,
		Language:      spec.Language,
		Focus:         spec.Focus,
		AgentModel:    r.cfg.Model,
	}
	ex.Trajectory = append(ex.Trajectory, TrajectoryStep{Role: "user", Content: spec.Task})

	chatProvider := buildChatProvider(r.cfg.Model, r.cfg.BaseURL)
	sys := r.buildSystemPrompt(spec)
	messages := []chat.Message{
		{Role: chat.RoleSystem, Content: sys},
		{Role: chat.RoleUser, Content: spec.Task},
	}

	opts := chat.DefaultChatOptions()
	if tools := reg.Definitions(); len(tools) > 0 {
		opts.Tools = tools
	}
	opts.NumCtx = r.cfg.NumCtx // janela configurável (menor = mais rápido p/ 4B local)

	// Timeout total por exemplo (não só por passo): garante que um exemplo
	// travado não pendure o lote inteiro.
	totalCtx, totalCancel := context.WithTimeout(ctx, r.cfg.Timeout*time.Duration(r.cfg.MaxRounds))
	defer totalCancel()

	for round := 0; round < r.cfg.MaxRounds; round++ {
		stepCtx, cancel := context.WithTimeout(totalCtx, r.cfg.Timeout)
		resp, err := chatProvider.Chat(stepCtx, messages, opts)
		cancel()
		if err != nil {
			log.Warn().Err(err).Str("model", r.cfg.Model).Msg("datasetgen: chat call failed")
			ex.LLMError = err.Error()
			break
		}

		content, toolCalls := parseProviderResponse(resp)

		// Registrar passo do assistente.
		step := TrajectoryStep{Role: "assistant", Content: content}
		for _, tc := range toolCalls {
			step.ToolCalls = append(step.ToolCalls, ToolCallJSON{Name: tc.Function.Name, Arguments: json.RawMessage(tc.Function.Arguments)})
		}
		ex.Trajectory = append(ex.Trajectory, step)

		if len(toolCalls) == 0 {
			// Prosa final ou resposta simples — sem mais tools.
			break
		}
		ex.ToolNames = mergeToolNames(ex.ToolNames, toolCalls)

		// Re-anexa a mensagem do assistente (com tool calls) ao histórico.
		messages = append(messages, chat.Message{Role: chat.RoleAssistant, Content: content, ToolCalls: toolCalls})

		// Executar tools no executor canônico.
		execCalls := toExecutorCalls(toolCalls)
		results := inner.ExecuteBatch(ctx, execCalls)
		for _, tr := range results {
			if tr == nil {
				continue
			}
			content := toolResultText(tr)
			trStep := TrajectoryStep{Role: "tool", Content: content}
			ex.Trajectory = append(ex.Trajectory, trStep)
			messages = append(messages, chat.Message{
				Role:       chat.RoleTool,
				Content:    content,
				ToolCallID: tr.ToolCallID,
			})
		}
	}

	// 4. Classificar.
	ex.Label = r.classify(ctx, ws, ex)
	ex.ToolNames = dedupe(ex.ToolNames)
	return ex, nil
}

// GenerateBatch processa N TaskSpecs de forma RESILIENTE: um exemplo que falha
// (timeout/erro de provider/panic) NÃO derruba o lote — vira um Example FAILURE
// com a trajetória parcial preservada. Retorna todos os exemplos processados
// e uma lista dos que falharam por razão de provider (para diagnóstico).
func (r *Runner) GenerateBatch(ctx context.Context, specs []TaskSpec) ([]*Example, []error) {
	examples := make([]*Example, 0, len(specs))
	var failReasons []error

	for i, spec := range specs {
		ex, err := r.GenerateExample(ctx, spec)
		if err != nil {
			// Falha de infra (ex.: não montou workspace) → registra.
			failReasons = append(failReasons, fmt.Errorf("spec[%d] (%s): %w", i, spec.Focus, err))
			continue
		}
		if ex.LLMError != "" {
			// Timeout/provider indisponível — já é FAILURE via classify.
			failReasons = append(failReasons, fmt.Errorf("spec[%d] (%s): llm_error=%s", i, spec.Focus, ex.LLMError))
		}
		examples = append(examples, ex)
		fmt.Fprintf(os.Stderr, "datasetgen: spec[%d] focus=%s label=%s tools=%v\n",
			i, spec.Focus, ex.Label, ex.ToolNames)
	}
	return examples, failReasons
}

// ─── Montagem do executor canônico ───────────────────────────────────────────

// buildChatProvider monta o provider Ollama direto (sem auto-detect). O modelo
// é o aluno (ex.: qwen3:4b) que executa a trajetória de tool-call.
func buildChatProvider(model, baseURL string) chat.ChatProvider {
	oll := provider.NewOllama(model, baseURL)
	return provider.NewProviderAdapter(oll, model)
}

// newCanonicalExecutor monta o executor canônico (chat/executor) com o
// EVIDENCE GATE ativo (filesystem) + sandbox + policy. Nenhum atalho.
// Retorna o executor e o registry (para expor as tools ao LLM).
func (r *Runner) newCanonicalExecutor(workspace string) (*cexecutor.Executor, *tool.Registry) {
	reg := tool.NewRegistry()
	filesystem.Register(reg, workspace)

	gate := sandbox.NewGate(workspace, chat.SandboxWorkspace)
	inner := cexecutor.New(reg, gate, workspace)
	inner.SetPolicy(policy.New())
	return inner, reg
}

// newWorkspace cria diretório temporário e grava o initial_state.
func (r *Runner) newWorkspace(spec TaskSpec) (string, error) {
	ws := filepath.Join(r.cfg.WorkDir, fmt.Sprintf("ex-%d", time.Now().UnixNano()))
	if err := os.MkdirAll(ws, 0o755); err != nil {
		return "", err
	}
	for path, content := range spec.InitialState {
		full := filepath.Join(ws, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			return "", err
		}
	}
	return ws, nil
}

// buildSystemPrompt monta o prompt operacional cirúrgico.
func (r *Runner) buildSystemPrompt(spec TaskSpec) string {
	var sb strings.Builder
	sb.WriteString("You are a coding agent working in a workspace. ")
	sb.WriteString("You MUST use the available tools to accomplish the task. ")
	sb.WriteString("Always read_file before edit_file. old_string must be an exact substring you observed. ")
	sb.WriteString("If a tool errors, read again and retry. Execute by CALLING tools, not describing.\n\n")
	sb.WriteString("TASK: " + spec.Task)
	return sb.String()
}

// ─── Classificação ───────────────────────────────────────────────────────────

func (r *Runner) classify(ctx context.Context, ws string, ex *Example) Label {
	// Erro de provider/timeout: não é uma ação agêntica — é indisponibilidade.
	// Trata como FAILURE (nunca entra como demo positiva).
	if ex.LLMError != "" {
		return LabelFailure
	}

	expectedOk := r.expectedReached(ws, ex)

	switch {
	case !ex.anyToolCall():
		if ex.hasFinalProseOnly() {
			return LabelProseInsteadOfAction
		}
		return LabelFailure

	case ex.hasToolResultError() && expectedOk:
		return LabelRecoverySuccess

	case expectedOk:
		return LabelSuccess

	case ex.hasToolResultError():
		return LabelFailure

	default:
		if r.declaredDone(ex) {
			return LabelFalseCompletion
		}
		return LabelFailure
	}
}

func (r *Runner) expectedReached(ws string, ex *Example) bool {
	if len(ex.ExpectedState) == 0 {
		return ex.anyToolCall()
	}
	for path, needle := range ex.ExpectedState {
		data, err := os.ReadFile(filepath.Join(ws, path))
		if err != nil {
			return false
		}
		if needle != "" && !strings.Contains(string(data), needle) {
			return false
		}
	}
	return true
}

func (r *Runner) declaredDone(ex *Example) bool {
	last := ex.lastStep()
	if last == nil {
		return false
	}
	lc := strings.ToLower(last.Content)
	return strings.Contains(lc, "done") || strings.Contains(lc, "conclu") || strings.Contains(lc, "complet")
}

// ─── Helpers de conversão ────────────────────────────────────────────────────

// parseProviderResponse extrai conteúdo e tool calls da resposta do provider.
func parseProviderResponse(resp *chat.ChatResponse) (string, []chat.ToolCall) {
	if resp == nil || len(resp.Choices) == 0 {
		return "", nil
	}
	c := resp.Choices[0]
	return c.Message.Content, c.Message.ToolCalls
}

// toExecutorCalls converte chat.ToolCall → executor.ToolCall (com Input JSON).
func toExecutorCalls(tcs []chat.ToolCall) []cexecutor.ToolCall {
	out := make([]cexecutor.ToolCall, 0, len(tcs))
	for _, tc := range tcs {
		input := map[string]interface{}{}
		if tc.Function.Arguments != "" {
			_ = json.Unmarshal([]byte(tc.Function.Arguments), &input)
		}
		out = append(out, cexecutor.ToolCall{ID: tc.ID, Name: tc.Function.Name, Input: input})
	}
	return out
}

// toolResultText converte executor.ToolResult em texto para a trajetória.
func toolResultText(tr *cexecutor.ToolResult) string {
	if tr == nil {
		return ""
	}
	if tr.Error != "" {
		return "ERROR: " + tr.Error
	}
	switch v := tr.Output.(type) {
	case string:
		return v
	case nil:
		return ""
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(b)
	}
}

// mergeToolNames acrescenta nomes únicos de tool às tool names existentes.
func mergeToolNames(existing []string, tcs []chat.ToolCall) []string {
	for _, tc := range tcs {
		existing = append(existing, tc.Function.Name)
	}
	return existing
}

// dedupe remove duplicatas preservando a ordem.
func dedupe(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
