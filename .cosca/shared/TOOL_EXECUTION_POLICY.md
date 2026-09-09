# Tool Execution Policy — COSCA

> **Filosofia central:** *rigoroso para segurança, tolerante para execução recuperável.*
> O COSCA **não** afrouxa o rigor. Torna o rigor **observável, recuperável e proporcional ao tipo de falha**.

**Regra de ouro:** *nenhuma tool pode prender o Kernel, o agente ou a sessão indefinidamente.*

Este documento é o **contrato universal de execução de tools**. Todos os agentes do COSCA obedecem a **exatamente o mesmo protocolo** — nenhum agente inventa a própria interpretação de "tool falhou".

---

## 1. Ciclo de vida de toda tool

```
REQUEST
   │
   ▼
CAPABILITY CHECK                    ──❌ sem capability ──► DENY
   │
   ▼
SCHEMA VALIDATION                   ──❌ argumentos inválidos ──► REJECT
   │
   ▼
POLICY / PERMISSION                 ──❌ operação proibida ──► DENY
   │
   ▼
EXECUTION
   │  ├─ transient failure ──► RETRY
   │  ├─ timeout ────────────► ABORT + RECOVER
   │  └─ permanent failure ──► FAIL
   │
   ▼
RESULT VALIDATION                   ──❌ resultado inválido ──► RECOVER / FAIL
   │
   ▼
TOOL_RESULT
   │
   ▼
ORCHESTRATOR
```

Uma falha de tool **nunca** deve deixar o Kernel, o agente ou a sessão presos.

---

## 2. Taxonomia de falha (classifique antes de reagir)

Nem todo erro é fatal. Use uma taxonomia explícita:

| Classe | Retry? | Bloqueia agente? | Recuperação |
|--------|:------:|:----------------:|-------------|
| `invalid_arguments` | ❌ | ❌ | corrigir a chamada |
| `capability_denied` | ❌ | ❌ | escolher outra estratégia |
| `permission_denied` | ❌ | ❌ | reportar |
| `not_found` | ❌ | ❌ | alternativa |
| `timeout` | ✅ limitado | ❌ | abortar a tool |
| `transient` | ✅ limitado | ❌ | retry |
| `unavailable` | ✅ limitado | ❌ | fallback |
| `execution` | talvez | ❌ | diagnosticar |
| `invalid_result` | talvez | ❌ | retry / normalização |
| `internal` | ❌ | ⚠️ | escalar |

**Anti-padrão a evitar:** `permission_denied → retry → retry → retry`. Permissão não muda sozinha. Já `connection reset → retry` é legítimo.

---

## 3. Timeout em TRÊS níveis (nunca só um global)

```
Task   (ex: 30 min)
  │
  ├── Agent   (ex: 10 min)
  │       │
  │       ├── Tool A  (30s)
  │       ├── Tool B  (60s)
  │       └── Tool C  (120s)
```

Se a Tool C morrer: `Tool C timeout → cancel context → libera recursos → retorna ToolError → agente decide → continua ou delega`.

**Nunca** espere o timeout da Task inteira para descobrir que uma única tool morreu.

---

## 4. Retry com política, nunca infinito

```
type RetryPolicy struct {
    MaxAttempts int
    Backoff     time.Duration
}
```

```
attempt 1 → 100ms → attempt 2 → 500ms → attempt 3 → FAIL
```

Não faça retry de tudo. `permission_denied` não muda; `connection reset` pode responder ao retry.

---

## 5. Erro operacional estruturado (nunca "tool failed" genérico)

O agente precisa receber um erro que permita raciocinar:

```json
{
  "status": "failed",
  "tool": "run_tests",
  "failure_class": "timeout",
  "retryable": true,
  "attempt": 2,
  "max_attempts": 3,
  "duration_ms": 30000,
  "message": "tool execution exceeded timeout",
  "recovery": "retry_or_continue"
}
```

Assim o agente **não inventa** o que aconteceu.

---

## 6. Tool cancelável (context.Context)

Contrato ideal:

```go
type Tool interface {
    Name() string
    Execute(ctx context.Context, input ToolInput) (ToolResult, error)
}
```

A implementação **deve** respeitar:

```go
select {
case <-ctx.Done():
    return ToolResult{}, ctx.Err()
case result := <-work:
    return result, nil
}
```

Timeout **sem** cancelamento é só "paramos de esperar" — não "paramos a execução". Sem cancelamento: goroutine leak, processo órfão, lock preso.

---

## 7. Máquina de estados explícita

```
PENDING
   │
   ▼
VALIDATING
   │
   ▼
AUTHORIZED
   │
   ▼
RUNNING
   │  ├── SUCCESS
   │  ├── RETRYING  → RUNNING
   │  ├── TIMEOUT   → RECOVERING
   │  └── FAILED    → RECOVERING
```

Nenhuma transição implícita.

---

## 8. Trace obrigatório de tool

Cada execução gera (pelo menos) os eventos:

`TOOL_REQUESTED → TOOL_VALIDATING → TOOL_AUTHORIZED → TOOL_STARTED → TOOL_RETRY | TOOL_TIMEOUT | TOOL_FAILED | TOOL_SUCCEEDED | TOOL_CANCELLED | TOOL_RECOVERED`

Cada evento com: `trace_id · task_id · agent_id · tool_id · attempt · timestamp · duration · failure_class`.

Exemplo:
```
Task:    T-123
Agent:   backend
Tool:    run_tests

09:41:02 REQUESTED
09:41:02 AUTHORIZED
09:41:02 STARTED
09:41:32 TIMEOUT
09:41:32 CANCELLED
09:41:32 RECOVERED

Cause:  subprocess exceeded 30s
Action: agent continued
```

Aí acaba o achismo.

---

## 9. Erro de agente ≠ erro de tool

```
Agent
  └── Tool execution
        ├── success
        ├── timeout
        └── failure
```

Se a **tool** falha, **não** significa que o **agente** falhou. O agente pode:

```
Tool A → FAIL → analisar → Tool B → SUCCESS → continuar
```

Marque o agente como falho **somente** quando a tarefa dele realmente não puder continuar.

---

## 10. O Orchestrator decide a recuperação

```
Backend Agent → tool X → timeout
```

Comportamento **errado**: `TASK FAILED` automático.

Comportamento **certo**:
```
timeout
  ↓
agent reports blocked capability
  ↓
orchestrator evaluates
  ↓
retry?  alternative tool?  delegate?  continue without tool?  abort?
```

A tool reporta. O **Orchestrator** decide.

---

## 11. Agente NUNCA contorna capability / permissão

```
Frontend Agent → precisa de database_admin → não possui
  → não contorna por caminho escondido
  → CAPABILITY_MISSING
      ↓
    report
      ↓
    orchestrator
      ↓
    delegate para agente apropriado
```

Isso preserva a arquitetura capability-based.

---

## 12. Conectar ao ADR-031 e ao Auto-Audit

Métrica encadeada:
```
Task → Agent → Tools → Tokens → Useful Work → Failures → Retries
```

**Auto-Audit enxerga padrões:**
```
Tool:    run_tests
Calls:   182
Success: 176
Timeout: 6
Timeout rate: 3.3%   ⚠️ acima do baseline
```

```
Agent:   backend
Tool failures: permission 0 · schema 1 · timeout 18 · execution 2
Finding: 18/21 failures são timeout → problema de EXECUÇÃO, não de capability
```

Muito melhor do que simplesmente aumentar o timeout.

---

## 13. Regra que todo agente memoriza

> **Agentes devem usar somente capabilities e tools explicitamente concedidas pelo COSCA para o projeto e a tarefa atual.**
> Uma falha de tool **não** deve ser interpretada automaticamente como falha da tarefa.
> O agente deve classificar o resultado, respeitar retry/timeout/cancellation e devolver um erro operacional estruturado ao Orchestrator.
> Agentes **nunca** contornam restrições de capability, permissão ou política.
> Tools devem ser canceláveis, ter timeout definido e **nunca** bloquear indefinidamente o Kernel ou o Orchestrator.
> O Orchestrator é responsável pela decisão de recuperação: retry, fallback, delegação, continuação ou abort.
> Toda execução de tool deve ser observável no trace.

---

## 14. Resumo na forma de tabela operacional

| Camada | Falhou? | Resposta | Recuperação |
|--------|---------|----------|-------------|
| Capability | não possui | `DENY` | delegate / outra estratégia |
| Schema | argumento inválido | `REJECT` | corrigir chamada |
| Permission | operação proibida | `DENY` | reportar |
| Execução | transient | `RETRY` | retry limitado |
| Execução | timeout | `ABORT` | cancelar + recover |
| Execução | permanente | `FAIL` | fallback / diagnosticar |
| Resultado | malformado | `RECOVER` | retry / normalizar |
| Agente | não respondeu | `TIMEOUT` | recovery |

**Frase-chave para o agente:**
> "Não vou executar porque X não está exatamente conforme o contrato" → **governança** (correto).
> "Não consegui executar e fiquei pendurado" → **bug de execução** (corrigir sem sacrificar o rigor).
