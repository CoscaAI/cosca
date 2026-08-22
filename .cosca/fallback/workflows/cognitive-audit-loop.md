# Cognitive Audit Loop — Post-Task Enforcement Workflow

> **Version**: 1.0.0 | **Status**: active | **Owner**: Cosca Kernel | **Created**: 2026-07-30
> **DNA Version**: 3.0.0
> **Purpose**: Enforcement mechanism for metacognition pipeline stages 7-8 (EXTRACT PATTERN, UPDATE CAPABILITY MODEL)
> **Trigger**: Após CADA task de QUALQUER agente Cosca
> **Timeout**: 30 segundos (nunca bloqueia execução da task)

---

## Propósito

O Cognitive Audit Loop é o MECANISMO DE ENFORCEMENT que garante que os estágios 7 e 8 do metacognition pipeline NUNCA mais sejam pulados. Ele resolve a falha sistêmica F002 (documentada em `failures.md`): a ausência de um trigger obrigatório fez com que stages 7-8 fossem consistentemente ignorados, causando drift de documentação em todos os arquivos de memória do ecossistema.

Este workflow é **acionado automaticamente** ao final de cada task, seja pelo Kernel, por um post-task hook, ou pelo próprio agente como parte do seu protocolo de auto-evolução.

---

## Arquitetura de Enforcement

```
┌──────────────────────────────────────────────────────────────────────┐
│                 COGNITIVE AUDIT LOOP — EXECUTION FLOW                │
│                                                                      │
│  ┌──────────┐    ┌──────────────┐    ┌──────────────┐    ┌────────┐ │
│  │  TASK    │───►│  STAGE 6     │───►│  AUDIT GATE  │───►│  DONE  │ │
│  │ COMPLETE │    │  CRITIQUE    │    │  ╔════════╗  │    │        │ │
│  │ (stages  │    │  OWN WORK    │    │  ║ Stages  ║  │    │        │ │
│  │  1-5)    │    │  (complete)  │    │  ║ 7 & 8   ║  │    │        │ │
│  └──────────┘    └──────────────┘    │  ║ executed║──┼───►│        │ │
│                                      │  ║  ?      ║  │ YES│        │ │
│                                      │  ╚════╤═══╝  │    │        │ │
│                                      │       │NO     │    │        │ │
│                                      │       ▼       │    │        │ │
│                                      │  ┌─────────┐  │    │        │ │
│                                      │  │ EXECUTE  │  │    │        │ │
│                                      │  │ Stage 7  │  │    │        │ │
│                                      │  │ Stage 8  │──┼───►│        │ │
│                                      │  └─────────┘  │    │        │ │
│                                      └──────────────┘    └────────┘ │
│                                                                      │
│  Se stages 7-8 NÃO executados após 3 tentativas:                     │
│  → Task marcada como INCOMPLETE                                      │
│  → Kernel notificado                                                 │
│  → Audit trail registrado                                            │
│  → Próxima task do agente BLOQUEADA até reconciliação                │
└──────────────────────────────────────────────────────────────────────┘
```

---

## Gatilhos de Ativação

O Cognitive Audit Loop é disparado por qualquer um dos seguintes eventos:

| Gatilho | Descrição | Frequência |
|---------|-----------|------------|
| **Kernel post-task hook** | Kernel verifica audit trail ao finalizar task | Toda task |
| **Agent self-check** | Agente verifica seu próprio completion status | Toda task |
| **Runtime middleware** | Hook no runtime que intercepta task completion | Toda task |
| **Scheduled audit** | Verificação periódica de tasks pendentes (a cada 5 min) | Background |
| **Manual trigger** | Acionado por `cosca audit --agent {name}` | Sob demanda |

---

## Checklist de Verificação (5 Perguntas)

Após CADA task, o agente DEVE responder TODAS as 5 perguntas. O Cognitive Audit Loop verifica cada resposta.

### Q1: Extração de Aprendizado

```
Pergunta: "O que eu aprendi que eu não sabia antes?"
Artefato: learnings.md
Validação: Entrada adicionada com timestamp, técnica, nível, outcome, tags?
Timeout: 30s
```

**Se SIM**: Entrada registrada → Q1 = `true`
**Se NÃO (nada novo)**: Registrar "Task rotineira — técnica {nome} já dominada" → Q1 = `true`
**Se TIMEOUT**: Marcar para async reconciliation → Q1 = `partial`

### Q2: Extração de Padrão

```
Pergunta: "Descobri um padrão reutilizável?"
Artefato: patterns.md
Validação: Padrão documentado com domínio, contexto, solução, confiança, pitfalls?
Timeout: 30s
```

**Se SIM (novo padrão)**: Entrada registrada → Q2 = `true`
**Se SIM (falha)**: Registrar em failures.md + learnings.md → Q2 = `true` (negative memory também é padrão)
**Se NÃO**: Registrar "Nenhum padrão novo — coberto por {padrão existente}" → Q2 = `true`
**Se TIMEOUT**: Marcar para async → Q2 = `partial`

### Q3: Registro de Falha

```
Pergunta: "Algo falhou?"
Artefato: failures.md
Validação: Falha documentada com causa raiz, consequência, lição, padrão de evitação?
Timeout: 30s
```

**Se SIM**: Entrada registrada → Q3 = `true`
**Se NÃO**: Registrar "Task concluída sem falhas" → Q3 = `true`
**Se TIMEOUT**: Marcar para async → Q3 = `partial`

### Q4: Atualização de Capacidade

```
Pergunta: "Minha confiança / habilidades mudaram?"
Artefato: capability-profile.md
Validação: Confidence score recalculado? Strengths/weaknesses atualizados?
Timeout: 30s
```

**Se SIM (mudou)**: Perfil atualizado com delta documentado → Q4 = `true`
**Se NÃO (estável)**: Registrar "Confidence estável para {domínio}" → Q4 = `true`
**Se TIMEOUT**: Marcar para async → Q4 = `partial`

### Q5: Verificação de Level-Up

```
Pergunta: "Alcancei um threshold de level-up?"
Artefato: evolution.md
Validação: Threshold verificado? Contagem atualizada? Level-up registrado se aplicável?
Timeout: 30s
```

**Se SIM (level-up)**: evolution.md atualizado com data, nível, tasks acumuladas → Q5 = `true`
**Se NÃO**: Registrar contagem: "{X}/{Y} tasks para próximo nível" → Q5 = `true`
**Se TIMEOUT**: Marcar para async → Q5 = `partial`

---

## Audit Trail (Formato de Saída)

Cada execução do Cognitive Audit Loop produz um registro no formato:

```yaml
audit_entry:
  id: "audit-{timestamp}-{agent}-{task_hash}"
  task_id: "{identificador da task}"
  agent: "{nome do agente}"
  timestamp: "2026-07-30T14:30:00Z"
  pipeline_stages:
    stage_7_extract_pattern:
      executed: true
      duration_ms: 450
      findings: "pattern|failure|none"
      artifact_updated: "patterns.md"
    stage_8_update_capability:
      executed: true
      duration_ms: 320
      confidence_delta: +0.05
      level_up: false
      artifact_updated: "capability-profile.md"
  checklist:
    q1_learnings: true
    q2_pattern: true
    q3_failure: true
    q4_capability: true
    q5_evolution: true
  completion:
    status: "complete"       # complete | incomplete | partial
    blocked: false            # true se próxima task do agente está bloqueada
    reconciliation_needed: false  # true se algum Q = partial
  metadata:
    total_duration_ms: 980
    timeout_triggered: false
    retry_count: 0
```

---

## Estados de Completion

| Estado | Significado | Ação |
|--------|-------------|------|
| `complete` | Todas as 5 perguntas respondidas, todos os artefatos atualizados | Task concluída. Próxima task liberada. |
| `incomplete` | Uma ou mais perguntas NÃO respondidas após 3 tentativas | Task marcada como incompleta. Kernel notificado. Próxima task BLOQUEADA. |
| `partial` | Uma ou mais perguntas em timeout — registradas para async | Task concluída com ressalva. Async reconciliation agendada. Próxima task liberada. |

---

## Regras de Bloqueio

```
┌─────────────────────────────────────────────────────────────────┐
│                    BLOCKING RULES                                │
│                                                                  │
│  IF audit_status == "incomplete":                                │
│    → Próxima task do agente é BLOQUEADA                          │
│    → Agente recebe: "Task anterior incompleta — stages 7-8      │
│      não executados. Complete o audit loop antes de prosseguir." │
│    → Kernel é notificado                                         │
│    → Após 3 tasks bloqueadas consecutivas → review humano        │
│                                                                  │
│  IF audit_status == "partial":                                   │
│    → Próxima task NÃO é bloqueada                                │
│    → Async reconciliation job é agendado (background)            │
│    → Se reconciliation falhar por 3 ciclos → escala para Kernel  │
│                                                                  │
│  IF audit_status == "complete":                                  │
│    → Nenhuma ação. Fluxo normal.                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## Integração com o Ecossistema

| Componente | Como Integra |
|------------|-------------|
| **metacognition-pipeline.md** | Audit loop é o enforcement dos estágios 7-8 marcados como MANDATORY |
| **AUTO_EVOLUTION_PROTOCOL.md** | Protocolo referencia o audit loop como mecanismo de compliance |
| **Kernel** | Kernel verifica audit trail antes de marcar task como concluída |
| **Runtime** | Middleware intercepta `task.complete` e dispara audit loop |
| **Memory Engine** | Artefatos atualizados pelo audit loop (learnings, patterns, failures) |
| **Evolution Engine** | evolution.md e capability-profile.md atualizados pelo audit loop |
| **Monitoring** | Métrica: `audit_completion_rate` (tasks com audit completo / total tasks) |

---

## Comandos

```bash
# Executar audit loop manualmente para um agente específico
cosca audit --agent cosca-evolution

# Verificar status de audit de todas as tasks pendentes
cosca audit --status

# Forçar reconciliação de tasks com status "partial"
cosca audit --reconcile

# Listar tasks bloqueadas por audit incompleto
cosca audit --blocked

# Executar audit loop para a última task de todos os agentes
cosca audit --all

# Ver métrica de completion rate
cosca audit --metrics
```

---

## Anti-Padrões Detectados

O Cognitive Audit Loop identifica e sinaliza estes anti-padrões:

| Anti-Padrão | Detecção | Ação |
|-------------|----------|------|
| **Silent Skip** | Agente completa task sem registro de audit | Task marcada INCOMPLETE. Alerta ao Kernel. |
| **Empty Extraction** | Q1-Q3 respondidas com string vazia ou placeholder | Rejeitado. Exige resposta substantiva. |
| **Copy-Paste Audit** | Audit trail idêntico ao anterior (hash match) | Alerta: possível extração automatizada sem reflexão real. |
| **Confidence Inflation** | Confidence score sobe >0.20 sem evidência de sucesso excepcional | Flag para revisão. |
| **Level-Up Rush** | Agente reporta level-up sem atingir threshold documentado | Rejeitado. Exige evidência. |
| **Timeout Abuse** | Agente consistentemente atinge timeout em stages 7-8 (>3 tasks) | Escalado para diagnóstico de performance. |

---

## Métricas de Saúde

O Cognitive Audit Loop expõe estas métricas para o Monitoring Engine:

| Métrica | Descrição | Target |
|---------|-----------|--------|
| `audit_completion_rate` | % de tasks com audit trail completo | ≥ 95% |
| `audit_partial_rate` | % de tasks com audit parcial (timeout) | ≤ 5% |
| `audit_incomplete_rate` | % de tasks sem audit (bloqueadas) | ≤ 1% |
| `audit_avg_duration_ms` | Tempo médio do audit loop | ≤ 2000ms |
| `extraction_yield` | % de tasks que geraram novo conhecimento (learnings/patterns/failures) | ≥ 30% |
| `drift_score` | Medida de desatualização dos arquivos de memória (0 = tudo atualizado) | ≤ 0.05 |

---

## Exemplo de Execução

```
TASK: "Otimizar query de busca semântica" (cosca-backend)

[Stage 6 — CRITIQUE OWN WORK] ✓
  → Colaboração com cosca-database foi eficiente. Índice FTS5 reduziu 340ms → 12ms.

═══════════════════════════════════════════
⚡ COGNITIVE AUDIT LOOP INICIADO
═══════════════════════════════════════════

Q1: O que aprendi?
  → Técnica: FTS5 content= option + composite index on (type, created_at)
  → Nível: 2 → aprendizado registrado em learnings.md
  → Tags: #sqlite #fts5 #query-optimization #performance
  ✓ Q1 = true (450ms)

Q2: Padrão reutilizável?
  → Novo padrão: "sqlite-fts5-optimization" — EXPLAIN QUERY PLAN antes de indexar
  → Registrado em patterns.md com domínio=database, confiança=0.85
  ✓ Q2 = true (320ms)

Q3: Algo falhou?
  → Não. Task concluída sem falhas.
  → Registro: "Nenhum novo negative memory"
  ✓ Q3 = true (80ms)

Q4: Confiança mudou?
  → Performance confidence: 0.45 → 0.50 (+0.05)
  → Backend confidence: 0.92 → 0.92 (estável)
  → capability-profile.md atualizado
  ✓ Q4 = true (210ms)

Q5: Level-up?
  → Backend: 14/15 tasks L3 completadas para L4 (falta 1)
  → evolution.md atualizado
  ✓ Q5 = true (150ms)

═══════════════════════════════════════════
AUDIT RESULT: complete (1210ms)
  → learnings.md: +1 entrada
  → patterns.md: +1 entrada
  → failures.md: sem alteração
  → capability-profile.md: atualizado
  → evolution.md: atualizado
═══════════════════════════════════════════

TASK STATUS: complete ✓
PRÓXIMA TASK: liberada ✓
```

---

## Relação com F002 (Failure Documentado)

Este workflow é a correção direta da falha F002 documentada em `failures.md`:

> **F002**: Metacognition Pipeline Stages 7-8 Não Executados (Sistêmico)
> **Causa raiz**: Pipeline projetado com 8 estágios, mas stages 7-8 sem mecanismo de enforcement.
> **Consequência**: Drift de documentação em TODOS os arquivos de memória.
> **Correção**: Cognitive Audit Loop + MANDATORY flag no pipeline + Post-Task Checklist.

Com este workflow ativo, a falha F002 é considerada **mitigada** e o risco de drift de documentação é reduzido a ≤ 1% (métrica `audit_incomplete_rate`).

---

> **Related**: [metacognition-pipeline.md](metacognition-pipeline.md) | [AUTO_EVOLUTION_PROTOCOL.md](../shared/AUTO_EVOLUTION_PROTOCOL.md) | [LEARNING_PROTOCOL.md](../memory/LEARNING_PROTOCOL.md) | [failures.md — F002](../memory/agent/cosca-kernel/failures.md)
