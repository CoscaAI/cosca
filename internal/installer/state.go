// Package installer — COSCA Environment Provisioner.
//
// O instalador NÃO sabe instalar COSCA — ele ORQUESTRA as capabilities já
// existentes (machine, doctor, health, knowledge, embeddings, index, QGate,
// GOLD). É o maestro: conhece os CONTRATOS, não a implementação.
//
// Cada etapa segue a regra de ouro do professor:
//
//	Detectar → decidir → executar → validar → registrar evidência.
//
// E retorna um StepResult com CHECK/ACTION/RESULT/EVIDENCE/STATE — nunca um
// "500 ifs".
package installer

// State é o Installation State — a máquina de estados do provisionamento.
// Idempotente por design: o instalador persiste o estado e retoma de onde
// parou (fechou no meio → detecta → resume).
type State string

// Estados do ciclo de provisionamento (ordem do professor).
const (
	StateNotReady       State = "NOT_READY"       // máquina não verificada
	StatePreflightOK    State = "PREFLIGHT_OK"    // machine profile + compatibilidade
	StateDepsReady      State = "DEPENDENCIES_READY" // git/go/ollama presentes
	StateAuthReady      State = "AUTH_READY"      // github autenticado
	StateCoscaReady     State = "COSCA_READY"     // fonte/config instaladas
	StateAIReady        State = "AI_READY"        // ollama + modelo embedding + smoke test
	StateKnowledgeReady State = "KNOWLEDGE_READY" // database + corpus
	StateIndexReady     State = "INDEX_READY"     // vetores + índice construído
	StateRuntimeReady   State = "RUNTIME_READY"   // serve/daemon configurados
	StateCertified      State = "CERTIFIED"       // build + tests + health + cert
)

// Order define a ordem canônica dos estados (para retomada e progresso).
var Order = []State{
	StateNotReady,
	StatePreflightOK,
	StateDepsReady,
	StateAuthReady,
	StateCoscaReady,
	StateAIReady,
	StateKnowledgeReady,
	StateIndexReady,
	StateRuntimeReady,
	StateCertified,
}

// Result é o resultado de um CHECK/ACTION (o "veredito" de uma etapa).
type Result string

// Vereditos possíveis de uma etapa.
const (
	ResultPass           Result = "PASS"            // etapa ok
	ResultFail           Result = "FAIL"            // etapa bloqueada
	ResultRebuildRequired Result = "REBUILD_REQUIRED" // detectou defasagem
	ResultSkip           Result = "SKIP"            // não aplicável (ex: sem GPU)
)

// Action é o que a etapa FEZ (ou faria).
type Action string

// StepResult é o contrato de cada etapa do provisionamento: CHECK → ACTION →
// RESULT → EVIDENCE → STATE. É o que torna o instalador declarativo e
// auditável — cada etapa produz evidência verificável, não só um ✓.
type StepResult struct {
	// Check é o ID da capability verificada (ex: "github.repository_access").
	Check string `json:"check"`
	// Action é o que foi feito (ex: "verify", "install", "build_index").
	Action string `json:"action"`
	// Result é o veredito.
	Result Result `json:"result"`
	// Evidence são as provas da etapa (ex: "repository reachable").
	Evidence []string `json:"evidence,omitempty"`
	// State é o estado do installation state APÓS esta etapa.
	State State `json:"state"`
}

// NewStep cria um StepResult tipado (conveniência).
func NewStep(check, action string, result Result, state State, evidence ...string) StepResult {
	return StepResult{
		Check:    check,
		Action:   action,
		Result:   result,
		State:    state,
		Evidence: evidence,
	}
}

// Check é a interface declarativa de uma capability do provisionamento.
// O instalador orquestra Checks — ele não conhece a implementação de cada
// capability, apenas o contrato: Detect → (Install se precisar) → Validate.
type Check interface {
	// ID é o identificador único da capability (ex: "git.installed").
	ID() string
	// Name é o nome legível (ex: "Git").
	Name() string
	// Detect verifica o estado atual SEM instalar nada. Resultado:
	// PASS (ok) | REBUILD_REQUIRED (defasado) | FAIL (ausente).
	Detect() (Result, []string)
	// Install executa a instalação/ativação quando Detect != PASS.
	Install() error
	// Validate prova que a capability FUNCIONA de verdade (não só que o
	// arquivo existe) — o "smoke test" de cada etapa.
	Validate() (Result, []string)
}
