# ADR-015: Task Continuation Loop — primitivas neutras de control plane elevadas ao Root

> **Status:** PARTIAL — F1 implementada + Pending Resolution ativa nos 2 motores (2026-09-01, verificado em código) | **Owner:** cosca-architecture | **Last Updated:** 2026-09-01
> **Revisão:** F1 (primitiva neutra) implementada em `internal/task` + `internal/orchestrator` (TaskContinuationLoop, sem persistência — estado em memória). Pending Resolution (`internal/pending`) integrada aos 2 motores de execução — `AgentEngine.MaxTurns` e `orchestration.Executor.MaxToolRounds` — ambos inspecionam o estado antes de parar no limite e resolvem pendências implicadas (sem atalho secreto: registram sinal, nunca executam tool direto). Wiring: serve default ON (`COSCA_PENDING_RESOLUTION=false` desliga). F2–F5 pendentes: `task.TaskRepository` é um contrato sem implementação concreta; `internal/orchestrator` (control plane completo do ADR-015) permanece órfão no bootstrap — a resolução conectada diretamente aos motores foi a decisão pragmática do Don+professor.
> **Referência (campo de prova):** `internal/{task,orchestrator,decision}` do cosca-trader (provado no
> domínio de trading). **F1** eleva as 3 primitivas NEUTRAS para o Cosca Root
> (`github.com/CoscaAI/cosca`), como infraestrutura genérica, sem dependência de domínio.

---

## 0. Contexto

O cosca-trader provou um **Task Continuation Loop** (ADR-001 do trader): o Control Plane NÃO
martela operação a operação — ele só volta quando há um motivo cognitivo
(`INCOMPLETE_OBJECTIVE` / `STEP_LIMIT` / `INPUT_REQUIRED` / `WATCHDOG`). O estado da task
(`TaskState`) é a fonte de verdade entre os two planes; o `TaskOrchestrator` decide
`CONTINUE | COMPLETE`; a memória de decisões (`decision`) é event-sourced e auditável.

Estas 3 primitivas são **genéricas** — não sabem o que é uma posição de trading, um worker,
um editor ou um provider. Elas só precisam de um snapshot chave/valor (`Artifacts`) e de um
contrato de persistência injetável. Por isso a F1 as eleva ao Root como **infraestrutura neutra
de control plane**, prontas para o Kernel do Cosca orquestrar workers/edições/provedores com o
mesmo loop cognitivo.

## 1. O que foi criado (aditivo — nada existente foi alterado)

| Pacote Novo | Conteúdo | Papel |
|---|---|---|
| **`internal/task`** | `TaskState`, `TaskStatus`, `ContinuationReason`, `TaskObjective`, `LegalTransition`, `IsTerminal`, `ValidateObjective`, `Clone`, `TaskRepository` (interface), envelope `Event` + payloads | **CONTRATO** entre os two planes (a linguagem neutra). |
| **`internal/orchestrator`** | `TaskOrchestrator` (`Create/OnEvent/Decide/Continue/Complete/Abort/Status/List/AddCheckpoint/MarkWatchdog/CheckWatchdog`), `DecisionResult` estruturado, `MaxContinue` → `escalate_to_don` | **Task Manager** (thread-safe), decisor do loop. |
| **`internal/decision`** | `Decision`, `DecisionLog` (`Append/Supersede/Redact/ByTask/Latest/List`), `DecisionStore` (interface), `Derive` (função pura latest-winner) | **Memória event-sourced** de decisões (append-only, auditoria). |

Todos são **stdlib-only** (ver §4) e **aditivos**: nada é removido ou importado pelo código
existente do Root.

## 2. A INVARIANTE: primitivas puras (stdlib only), frente `Domain Adapter → Generic Control Plane`

A fronteira é sempre a mesma: a **primitiva** expõe um **contrato** (interface tipada), e a
**borda de domínio** implementa esse contrato. As 3 primitivas NÃO conhecem o domínio:

- `internal/task` define `TaskRepository` (Save/Load/List/Delete) — quem implementa é o adapter.
- `internal/decision` define `DecisionStore` (AppendEvent/Events/Save/ByTask/List) — quem
  implementa é o adapter.
- `internal/orchestrator` só fala com `task.TaskRepository`, `task.Event` e stdlib.

Nenhuma delas importa `internal/workers`, `internal/editors`, `internal/desktop`,
`internal/providers`, `internal/pipeline`, `internal/durable` nem `internal/audit`. Confirmado por
`go list -deps` (ver §4).

## 3. REUSO: o que já existia e a decisão

Antes de criar, o Root foi lido em `internal/pipeline`, `internal/durable`, `internal/orchestration`,
`internal/aitask` e `internal/audit`. Resultado da análise:

### 3.1 Nomes NÃO colidem → criar pacotes novos, mas SEM duplicar conceito

| Root existente | Conceito | Colisão com F1? | Decisão |
|---|---|---|---|
| `internal/aitask` (`Type`, `Catalog` das 18 tarefas de IA) | Catálogo de tarefas de IA | Não | Coexiste. `task` é ciclo de vida de execução; `aitask` é catálogo de modalidades. Coexistem sem conflito de nome (pacotes distintos, semânticas distintas). |
| `internal/orchestration` | Orquestrador simbólico (pipeline/chains) do Root | Não (nome é `orchestration`, não `orchestrator`) | Coexiste. `orchestrator.TaskOrchestrator` é o Task Continuation Loop; `orchestration` é o motor de pipeline do Root. Não usamos `orchestration` para não acoplar a primitiva ao Plan/TaskNode do pipeline. |
| `internal/pipeline` (`Plan`, `TaskNode`, `Checkpoint`, `RecoveryLoop`, `DurableEvent`) | Execução de plano + checkpoint/recuperação de run | **Parcial (checkpoint)** | **Não reusado DENTRO da primitiva** — ver 3.2. |
| `internal/durable` (`Ledger`, fencing/lease/checkpoint/idempotency) | Ledger durável de runs | **Parcial (persistência)** | **Não reusado DENTRO da primitiva** — ver 3.2. |
| `internal/audit` (`DecisionStore` SQLite, D-0001, Record/Get/List) | **Trilha de EXPLICABILIDADE** ("por que fiz isso": laws/evidence/provider/model) | Não (mesmo nome `DecisionStore`, conceito distinto) | **Coexiste, não duplica.** `audit.DecisionStore` = trilha de explicabilidade (SQLite, IDs D-xxxx). `decision.DecisionLog` = memória event-sourced do control plane (latest-winner, append/supersede/redact/ByTask/Latest). Conceitos complementares; sem sobreposição. |

### 3.2 Por que NÃO reusar `durable`/`pipeline` DENTRO das primitivas (justificativa honesta)

A instrução da F1 permite reusar `durable`/`pipeline` **"SE forem infra neutra"** e exige justificar
quando não houver reuso. **Não reuso dentro da primitiva por um motivo de invariante concreta:**

- `internal/durable` importa `internal/sqlite` → **`modernc.org/sqlite`** (dependência externa).
  Importá-lo derrubaria a invariante `go list -deps = stdlib only` (§4), trazendo toda a árvore de
  deps externas para dentro da primitiva. Contra o contrato da F1.
- `internal/pipeline` é **neutro mas não puro** e é orientado a `Plan`/`TaskNode`/`TaskResult`
  (modelo de execução de plano). Acoplar `orchestrator` a `pipeline.TaskNode` contaminaria o
  loop com o modelo de plano do Root e criaria um ciclo conceitual (pipeline já tem seu próprio
  loop/recuperação).

**A decisão de fronteira (a régua da F1):** a **primitiva permanece pura** (stdlib only) e o
**reuso de `durable`/`pipeline` vive na BORDA de domínio — nas implementações concretas dos
contratos**, que NÃO fazem parte desta F1:

- O **`TaskRepository` concreto** pode ser um backend reusando `internal/durable` (ledger) ou
  `internal/pipeline.CheckpointStore` — injetado via `WithRepository`. → **F2.**
- O **`DecisionStore` concreto** pode ser um backend reusando `internal/durable` ou um arquivo
  JSONL append-only — injetado via `NewDecisionLog`. → **F2/F3.**
- O **watchdog/checkpoint** da task (`Checkpoints`) é a-campos-do `TaskState`; a implementação
  concreta pode mapeá-los para `durable.Checkpoint`. → **F2.**

Ou seja: o **reuso é real, mas no lugar certo** (adapter concreto), não na primitiva. Se um
reuso concreto introduzir deps externas (ex.: SQLite p/ `TaskRepository`), isso vaza para a
borda, não para o contrato — exatamente o desenho `Domain Adapter → Generic Control Plane`.

### 3.3 Preparação explícita do reuso na F2/F3

A F1 já deixa os pontos de ancoragem prontos:
- `TaskOrchestrator.WithRepository(task.TaskRepository)` — persiste FORA do lock e re-hidrata
  NÃO-terminais no `New` (retomada idempotente), provado no campo de prova.
- `Option WithEmitter(task.Event)` — emite `TASK_CONTINUE` no transporte real.
- `task.TaskObjective.Deadline` (RFC3339) alimenta `CheckWatchdog` (dead-man switch).
- `decision.DecisionLog` aceita um `DecisionStore` concreto e re-hidrata por replay.

## 4. Invariante verificada (`go list -deps`)

`go list -deps ./internal/task/... ./internal/orchestrator/... ./internal/decision/...` resolve para:

- **apenas stdlib** (`fmt`, `errors`, `strings`, `sync`, `time`, `os`, `strconv`, `log`,
  `crypto/rand`, `encoding/hex`, ...); e
- `github.com/CoscaAI/cosca/internal/task` (importado legitimamente por `orchestrator`).

**Nenhum** pacote de domínio (`workers`/`editors`/`desktop`/`providers`). **Nenhuma** dependência
externa (`google/uuid`, `shopspring/decimal`, `modernc.org/sqlite`, ...).

Diferenças deliberadas vs o campo de prova (que mantêm a pureza):
- Removida a dependência `github.com/google/uuid` do `orchestrator` → gerador de `TaskID`
  stdlib-only (`crypto/rand` → hex, com fallback timestamp+counter).
- Removida a dependência do pacote `internal/event` do trader (que não existe no Root) → o
  envelope mínimo `task.Event` + constantes `task.EventXxx`/`task.SeverityXxx` foram movidos
  para dentro de `internal/task` (a linguagem dos planes). No Root não criamos um bus genérico
  `internal/event` de propósito — o adapter mapeia `task.Event` para o transporte real.
- Env var renomeada `COSCA_TRADER_TASK_MAX_CONTINUE` → `COSCA_TASK_MAX_CONTINUE` (genérica).

## 5. Impacto no Root / não-regressão

- `go build ./...` → **verde**.
- `go test ./internal/task/... ./internal/orchestrator/... ./internal/decision/...` → **verde**.
- Testes adaptados do campo de prova (`task`, `orchestrator`, `orchestrator_f2`, `decision`).
- **Não** foi tocado código Go existente do Root; os 3 pacotes são novos e nada os importa ainda.

> Nota de regressão (pré-existente, não causada pela F1): `internal/integrity` tem uma falha
> **ambiental** de identidade machine-bound Ed25519 ("cannot load private key (only the kernel
> on this machine can sign)" → `IDENTIDADE-BREACH`). Ela falha **com e sem** os pacotes da F1
> (verificado movendo os 3 pacotes para fora da árvore). Não é causada por esta mudança.

## 6. Decisões e trade-offs

| Decisão | Trade-off / justificativa |
|---|---|
| Criar `internal/task`, `internal/orchestrator`, `internal/decision` (novos) vs estender `orchestration`/`aitask` | Nomes não colidem e os conceitos são distintos. Estender `orchestration`/`aitask` acoplaria o loop ao Plan/TaskNode/catálogo do Root (contra a pureza). Criar pacotes novos é aditivo e cirúrgico. |
| Não reusar `durable`/`pipeline` **dentro** da primitiva | Preserva a invariante stdlib-only (importá-los puxaria `modernc.org/sqlite`). O reuso é delegado ao adapter concreto (F2/F3). |
| `decision` coexiste com `audit.DecisionStore` | São conceitos complementares: `audit` = explicabilidade (SQLite, D-xxxx); `decision` = memória event-sourced do control plane (latest-winner). Sem sobreposição. |
| Envelope `task.Event` dentro de `task` | O Root não tem `internal/event`. Criar um bus genérico é escopo demais; o envelope mínimo mantém a primitiva auto-contida. O adapter mapeia para o transporte real. |

## 7. Ficou para depois

- **F2:** implementações concretas de `TaskRepository` e `DecisionStore` reusando `internal/durable`
  / `pipeline` (borda); ligar `MaxContinue` ao Don escalate real.
- **F3:** compor o `TaskOrchestrator` no Kernel (workers/edições/provedores) + transporte de
  eventos (`task.Event` → bus real); interpretar `Deadline` canonicamente.
- **F4:** `WithObjectiveSatisfied` real (Policy/Risk controller) e integração com
  `internal/deliberate`/`internal/gate` (Plan-only).
- **F5:** telemetria/observabilidade do loop (contadores de continue/escalate/complete) e
  integração de `decision` com `internal/audit` (explicabilidade cross-trace).

## 8. Referências

- Campo de prova: `C:\Users\Henrique\Documents\projects\cosca-trader\internal\{task,orchestrator,decision}`.
- Contrato puro: `internal/task/task.go`, `internal/task/events.go`, `internal/task/repository.go`.
- Primitive: `internal/orchestrator/orchestrator.go`, `internal/decision/decision.go`.
- Reuso futuro: `internal/durable/ledger.go`, `internal/pipeline/checkpoint.go`,
  `internal/pipeline/durable_events.go`, `internal/audit/decision.go`.
