package task

// TaskRepository persiste e re-hidrata TaskState de forma durável (F2, ADR-015).
//
// É o contrato do ADAPTER de persistência: a PRIMITIVA (internal/task e
// internal/orchestrator) permanece pura — stdlib only — e NÃO importa
// `internal/durable`, `internal/pipeline`, `internal/sqlite` nem qualquer
// fronteira de domínio: essas são o reuso legítimo da implementação concreta.
// Quem implementa este contrato é uma BORDA de domínio (ex.: um repositório
// SQLite apoiado em `internal/durable`, ou um arquivo JSONL apoiado em
// `internal/pipeline`), injetada no TaskOrchestrator via WithRepository.
//
// Convenção do contrato:
//   - Save  upserta (idempotente pela TaskID) um TaskState;
//   - Load  devolve UM TaskState (nil se não existir);
//   - List  devolve TODOS (o orquestrador filtra os NÃO-terminais na retomada);
//   - Delete remove pela TaskID.
//
// As implementações devem retornar uma cópia em Load/List (não expor estado
// interno mutável) — o TaskState.Clone() cobre a leitura segura.
type TaskRepository interface {
	Save(*TaskState) error
	Load(taskID TaskID) (*TaskState, error)
	List() ([]*TaskState, error)
	Delete(taskID TaskID) error
}
