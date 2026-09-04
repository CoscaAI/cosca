package datasetgen

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ─── Classes de rotulagem (classificador automático) ─────────────────────────

// Label é a classe de um exemplo de trajetória. Reflete se a análise do
// classificador + verificação do resultado esperado.
type Label string

const (
	// LabelSuccess: ferramentas executadas, o resultado esperado foi atingido.
	LabelSuccess Label = "SUCCESS"

	// LabelRecoverySuccess: o agente errou, MAS se recuperou e terminou certo.
	// Este é o material mais valioso (o "reflexo de recuperação").
	LabelRecoverySuccess Label = "RECOVERY_SUCCESS"

	// LabelFailure: não completou / desistiu / entrou em loop / resultado não
	// atingido.
	LabelFailure Label = "FAILURE"

	// LabelUnsafeAction: tentou editar sem ler, old_string inventado, ou ação
	// que viola a invariante de evidência.
	LabelUnsafeAction Label = "UNSAFE_ACTION"

	// LabelProseInsteadOfAction: respondeu em prosa/descreveu em vez de chamar
	// uma ferramenta (o "pedreiro que não constrói").
	LabelProseInsteadOfAction Label = "PROSE_INSTEAD_OF_ACTION"

	// LabelFalseCompletion: declarou sucesso sem evidência / sem atender o DoD.
	LabelFalseCompletion Label = "FALSE_COMPLETION"
)

// IsPositive reporta se a label é uma demonstração POSITIVA (entra no
// dataset de fine-tune como exemplo a seguir). As labels de contraste
// (FAILURE/UNSAFE/PROSE/FALSE_COMPLETION) não são demonstrações — são
// material de preferência/contraste.
func (l Label) IsPositive() bool {
	switch l {
	case LabelSuccess, LabelRecoverySuccess:
		return true
	default:
		return false
	}
}

// String retorna a label como string.
func (l Label) String() string { return string(l) }

// ─── Tipos de dados ──────────────────────────────────────────────────────────

// Focus categoriza o comportamento que um exemplo exercita no agente.
type Focus string

const (
	FocusHappyPath Focus = "happy_path"
	FocusRecovery  Focus = "recovery"
	FocusRequestInfo Focus = "request_info"
	FocusSearchFirst Focus = "search_first"
	FocusMultiFile  Focus = "multi_file"
	FocusNoDoD      Focus = "no_dod"
)

// TrajectoryStep é um passo da trajetória (mensagem) do agente.
type TrajectoryStep struct {
	Role      string  `json:"role"`                // user | assistant | tool
	Content   string  `json:"content,omitempty"`   // texto (assistant) ou tool result
	Name      string  `json:"name,omitempty"`      // nome da tool (role=tool)
	ToolCalls []ToolCallJSON `json:"tool_calls,omitempty"` // chamadas (assistant)
}

// ToolCallJSON é uma chamada de ferramenta na trajetória.
type ToolCallJSON struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// Example é um exemplo do dataset (uma linha do JSONL).
type Example struct {
	// Task é a instrução (prompt) dada ao agente.
	Task string `json:"task"`
	// InitialState é o estado inicial do workspace (path → conteúdo).
	InitialState map[string]string `json:"initial_state"`
	// ExpectedState descreve o resultado esperado (path → substring/condição).
	ExpectedState map[string]string `json:"expected_state"`
	// StateOptions permite múltiplas formas semanticamente válidas do estado
	// final (Golden v2). path → lista de fragmentos; o caso passa se QUALQUER
	// um estiver presente. Corrige o falso negativo do contrato v1 (substring
	// literal rejeitava edições válidas). Ver FALHA DE CONTRATO 2026-09-04.
	StateOptions map[string][]string `json:"state_options,omitempty"`
	// Language é a linguagem do artefato (go|python|ts|json|yaml|markdown...).
	Language string `json:"language"`
	// Focus é a categoria de comportamento exercitada.
	Focus Focus `json:"focus"`
	// AgentModel é o modelo que executou a trajetória (aluno).
	AgentModel string `json:"agent_model"`
	// Label é a classe resultante da classificação.
	Label Label `json:"label"`
	// Trajectory é a sequência completa de passos.
	Trajectory []TrajectoryStep `json:"trajectory"`
	// ToolNames lista as ferramentas efetivamente chamadas.
	ToolNames []string `json:"tool_names,omitempty"`
	// LLMError registra um erro de provider/timeout durante a geração (em
	// contraste com uma falha de tool). Um exemplo com LLMError é FAILURE.
	LLMError string `json:"llm_error,omitempty"`
}

// MarshalJSONL serializa um exemplo como uma única linha JSON (para o formato
// JSONL de fine-tune).
func (e *Example) MarshalJSONL() []byte {
	b, err := json.Marshal(e)
	if err != nil {
		return []byte(`{"error":"` + err.Error() + `"}`)
	}
	return append(b, '\n')
}

// ─── Análise auxiliar para o classificador ───────────────────────────────────

// toolNamesFromTrajectory extrai os nomes únicos de ferramentas chamadas.
func (e *Example) toolNamesFromTrajectory() []string {
	var names []string
	seen := map[string]bool{}
	for _, step := range e.Trajectory {
		for _, tc := range step.ToolCalls {
			if tc.Name != "" && !seen[tc.Name] {
				seen[tc.Name] = true
				names = append(names, tc.Name)
			}
		}
	}
	return names
}

// anyToolCall reporta se a trajetória contém pelo menos uma chamada de tool.
func (e *Example) anyToolCall() bool {
	for _, step := range e.Trajectory {
		if len(step.ToolCalls) > 0 {
			return true
		}
	}
	return false
}

// hasFinalProseOnly reporta se o passo final é prosa e NÃO houve tool call
// executada (o "pedreiro que não constrói").
func (e *Example) hasFinalProseOnly() bool {
	if e.anyToolCall() {
		return false
	}
	last := e.lastStep()
	return last != nil && last.Role == "assistant" && strings.TrimSpace(last.Content) != "" && len(last.ToolCalls) == 0
}

// lastStep retorna o último passo da trajetória (ou nil).
func (e *Example) lastStep() *TrajectoryStep {
	if len(e.Trajectory) == 0 {
		return nil
	}
	return &e.Trajectory[len(e.Trajectory)-1]
}

// hasToolResultError reporta se algum tool result contém um erro de execução
// (sinal de recuperação em um RECOVERY_SUCCESS).
func (e *Example) hasToolResultError() bool {
	for _, step := range e.Trajectory {
		if step.Role == "tool" && step.Content != "" {
			lc := strings.ToLower(step.Content)
			if strings.Contains(lc, "error") || strings.Contains(lc, "requires_read") ||
				strings.Contains(lc, "not found") || strings.Contains(lc, "stale") {
				return true
			}
		}
	}
	return false
}

// String (debug) retorna uma descrição curta.
func (e *Example) String() string {
	return fmt.Sprintf("Example{label=%s focus=%s lang=%s tools=%v}", e.Label, e.Focus, e.Language, e.toolNamesFromTrajectory())
}

// ─── Análise de invariante (Golden Gate camadas 2/5/6) ──────────────────────

// readsBeforeEdit verifica que TODO edit_file foi precedido de um read_file do
// mesmo path na trajetória (a invariante de evidência). Se houve algum edit
// sem leitura anterior do mesmo arquivo, retorna false (camada 2).
func (e *Example) readsBeforeEdit() bool {
	readPaths := map[string]bool{}
	orderOk := true
	for _, step := range e.Trajectory {
		for _, tc := range step.ToolCalls {
			var args struct {
				Path string `json:"path"`
			}
			_ = json.Unmarshal(tc.Arguments, &args)
			switch tc.Name {
			case "read_file", "read", "cat":
				if args.Path != "" {
					readPaths[args.Path] = true
				}
			case "edit_file", "edit", "write_file":
				if args.Path != "" && !readPaths[args.Path] {
					orderOk = false // editou sem ler antes
				}
			}
		}
	}
	return orderOk
}

// hasUnsafeMutation detecta uma violação crítica (camada 6): edit_file sem
// leitura prévia OU um raw JSON de tool-call em prosa não executado. Uma
// trajetória que só descreve a edição em prosa (nunca chamou edit_file) mas
// afirma que editou também é um sinal. Considera unsafe quando há um passo
// de prosa que contém "old_string"/"new_string" mas NENHUM tool_call edit.
func (e *Example) hasUnsafeMutation() bool {
	// Se editou sem ler antes → unsafe (violação da invariante).
	if !e.readsBeforeEdit() {
		return true
	}
	// Se o último passo declara "editado/alterado" mas a trajetória NUNCA
	// chamou edit_file/write_file → declaração de mutação sem evidência.
	var calledWrite bool
	for _, step := range e.Trajectory {
		for _, tc := range step.ToolCalls {
			if tc.Name == "edit_file" || tc.Name == "edit" || tc.Name == "write_file" {
				calledWrite = true
			}
		}
	}
	if !calledWrite {
		last := e.lastStep()
		if last != nil {
			lc := strings.ToLower(last.Content)
			if strings.Contains(lc, "editado") || strings.Contains(lc, "alterado") || strings.Contains(lc, "updated") || strings.Contains(lc, "edited") {
				return true
			}
		}
	}
	return false
}

// hasTestEvidence verifica (camada 4) se a trajetória executou uma tool de
// verificação (test/build/verify) ou se o passo final menciona evidência de
// que a mudança foi validada (PASS/OK/test passed).
func (e *Example) hasTestEvidence() bool {
	verifiedTools := map[string]bool{"test": true, "build": true, "verify": true, "run_tests": true, "go test": true}
	for _, step := range e.Trajectory {
		for _, tc := range step.ToolCalls {
			if verifiedTools[tc.Name] {
				return true
			}
		}
		// tool result com PASS/ok de teste.
		if step.Role == "tool" {
			lc := strings.ToLower(step.Content)
			if strings.Contains(lc, "pass") || strings.Contains(lc, "ok") || strings.Contains(lc, "success") {
				return true
			}
		}
	}
	return false
}
