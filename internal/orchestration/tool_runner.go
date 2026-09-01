package orchestration

// tool_runner.go — EXECUTOR DE FERRAMENTAS ÚNICO (Opção B, Etapa 3).
//
// O COSCA tem UM executor de ferramentas canônico: internal/chat/executor
// (com sandbox Gate, policy determinístico, permission allow/ask/deny, level
// gate e anti-hallucination). Antes havia um segundo executor aqui
// (ToolExecutor com tools hardcoded read_file/write_file/execute_command...)
// que rodava SEM nenhum desses gates e anunciava tools que nem executava.
//
// Este arquivo define a interface mínima que o orchestration precisa e o
// adapter que liga o executor canônico a ela — o ÚNICO caminho de execução de
// ferramentas no COSCA.

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/chat/executor"
)

// ToolCallResult is the result of executing a tool call.
type ToolCallResult struct {
	ToolCallID string        `json:"tool_call_id"`
	Name       string        `json:"name"`
	Content    string        `json:"content"`
	Error      string        `json:"error,omitempty"`
	ErrorCode  string        `json:"error_code,omitempty"`
	Duration   time.Duration `json:"duration"`

	// ── Erro operacional estruturado (Tool Execution Policy) ──────────────
	// Estes campos permitem ao agente RACIOCINAR sobre a falha, em vez de
	// receber apenas um "tool failed" genérico. Não são usados para rejeição;
	// apenas para comunicação de recuperação.
	FailureClass string `json:"failure_class,omitempty"` // invalid_arguments|capability_denied|permission_denied|not_found|timeout|transient|unavailable|execution|invalid_result|internal
	Attempt      int    `json:"attempt,omitempty"`       // tentativa em que falhou (1-based)
	Retryable    bool   `json:"retryable,omitempty"`     // se pode ser tentado novamente
	Recovery     string `json:"recovery,omitempty"`      // sugestão de recuperação (retry_or_continue|abort|fallback|delegate|report)
}

// ToolRunner é a interface mínima de execução de ferramentas que o
// orchestration.Executor consome. O adapter executorAdapter a implementa sobre
// o executor canônico (chat/executor).
type ToolRunner interface {
	// ExecuteAll executa um lote de tool calls (chat.ToolCall, o formato que o
	// LLM devolve) e retorna resultados no formato do orchestration.
	ExecuteAll(ctx context.Context, toolCalls []chat.ToolCall) ([]*ToolCallResult, error)

	// ListTools devolve as definições canônicas das ferramentas registradas —
	// o vocabulário REAL que o LLM pode chamar. Substitui o antigo deriveTools
	// (que anunciava nomes hardcoded que nem existiam no executor).
	ListTools(ctx context.Context) ([]chat.ToolDefinition, error)
}

// NewToolRunner cria o ToolRunner sobre o executor canônico (chat/executor).
// É o ÚNICO construtor que os callers (bootstrap/chat/run/serve/terminal) usam
// para ligar a execução de ferramentas ao orchestration.
func NewToolRunner(inner *executor.Executor) ToolRunner {
	if inner == nil {
		return nil
	}
	return newExecutorAdapter(inner)
}

// executorAdapter adapta *executor.Executor (chat/executor, o executor canônico
// com sandbox+policy+permission+level) à interface ToolRunner do orchestration.
type executorAdapter struct {
	inner *executor.Executor
}

// newExecutorAdapter cria o adapter sobre o executor canônico.
func newExecutorAdapter(inner *executor.Executor) *executorAdapter {
	return &executorAdapter{inner: inner}
}

// ListTools delega ao executor canônico (que já filtra tools negadas por
// permission — o LLM nunca vê o que não pode chamar).
func (a *executorAdapter) ListTools(ctx context.Context) ([]chat.ToolDefinition, error) {
	if a.inner == nil {
		return nil, nil
	}
	return a.inner.ListTools(ctx)
}

// ExecuteAll converte chat.ToolCall → executor.ToolCall, delega ao executor
// canônico (que aplica validação, sandbox, policy, permission e level) e
// converte os resultados de volta para ToolCallResult. Argumentos inválidos
// NÃO expõem o texto bruto (segurança — safeError).
func (a *executorAdapter) ExecuteAll(ctx context.Context, toolCalls []chat.ToolCall) ([]*ToolCallResult, error) {
	if a.inner == nil {
		return nil, fmt.Errorf("tool executor not configured")
	}
	if len(toolCalls) == 0 {
		return nil, nil
	}

	converted := make([]executor.ToolCall, 0, len(toolCalls))
	for _, tc := range toolCalls {
		input, err := toolArgsToInput(tc.Function.Arguments)
		if err != nil {
			info := safeError("tool_arguments_invalid", err)
			// Mesmo contrato do antigo ToolExecutor: erro mascarado no RESULT
			// (não no retorno), sem expor o texto bruto dos argumentos.
			return []*ToolCallResult{{
				ToolCallID: tc.ID,
				Name:       tc.Function.Name,
				Error:      safeToolMessage(err),
				ErrorCode:  info.Code,
			}}, nil
		}
		converted = append(converted, executor.ToolCall{
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: input,
		})
	}

	results := a.inner.ExecuteBatch(ctx, converted)
	out := make([]*ToolCallResult, 0, len(results))
	for _, r := range results {
		if r == nil {
			continue
		}
		out = append(out, &ToolCallResult{
			ToolCallID: r.ToolCallID,
			Name:       toolCallNameFromResult(r, toolCalls),
			Content:    toolResultContent(r),
			Error:      r.Error,
			Duration:   time.Duration(r.DurationMs) * time.Millisecond,
		})
	}
	return out, nil
}

// toolArgsToInput converte a string JSON de argumentos (chat.FunctionCall) no
// mapa de input que o executor canônico espera.
func toolArgsToInput(raw string) (map[string]interface{}, error) {
	if raw == "" {
		return map[string]interface{}{}, nil
	}
	var input map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &input); err != nil {
		return nil, err
	}
	return input, nil
}

// toolCallNameFromResult recupera o nome da tool a partir do batch original
// (o executor.ToolResult não carrega o nome).
func toolCallNameFromResult(r *executor.ToolResult, original []chat.ToolCall) string {
	for _, tc := range original {
		if tc.ID == r.ToolCallID {
			return tc.Function.Name
		}
	}
	return ""
}

// toolResultContent extrai o conteúdo textual do resultado (string ou
// serializável).
func toolResultContent(r *executor.ToolResult) string {
	switch v := r.Output.(type) {
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
