# CONFIDENCE MODEL — Evidence Trust Engine

> **Version**: 1.0.0 | **Status**: active | **Owner**: Cosca Kernel | **Created**: 2026-07-28
>
> **Constitutional authority**: [CONSTITUTION.md](../../CONSTITUTION.md) — Implements P2 (Código executado é a verdade absoluta) and Part III (Regras de conflito de informação).

---

## Purpose

When the Kernel or any agent faces conflicting information, this engine determines which source to trust. It assigns a numerical confidence score to every piece of evidence, enabling automated conflict resolution without human intervention for clear cases, and escalation for ambiguous ones.

---

## Evidence Hierarchy

### The 6 Levels

Every piece of information in the Cosca ecosystem belongs to one of these levels. The level determines the base trust weight.

| Level | Source | Base Weight | Verification | Example |
|-------|-------|-------------|-------------|---------|
| **5** | Código executado | **1.00** | `go build` compiles, binary runs | `go.mod` imports, `internal/runtime/runtime.go`, `api/rest/handler/auth.go` |
| **4** | Testes aprovados | **0.90** | `go test` passes, CI green | `internal/cli/cli_coverage_test.go`, integration tests passing |
| **3** | Documentação oficial | **0.60** | Reviewed, version-matched | `docs/api-reference/auth.md`, `README.md` (with correct version) |
| **2** | Memória do agente | **0.50** | learnings.md with evidence tags | `memory/agent/cosca-backend/learnings.md` entry with commit hash |
| **1** | Opinião de agente | **0.30** | Reasoning trace available | Agent's analysis without code reference |
| **0** | Resposta de LLM | **0.20** | External model, not Cosca-verified | LLM provider output, chat response |

### Why These Weights?

| Level | Rationale |
|-------|-----------|
| **5 — Código** | O que está em produção. Não é opinião, é fato executável. Se o código diz `sqlite.Open()`, o banco é SQLite — ponto final. |
| **4 — Testes** | Verificam comportamento. Mas testes podem ter gaps de cobertura, testar cenários errados, ou passar por acidente. |
| **3 — Docs** | Intenção documentada. Mas documentação desatualiza. O `README.md` pode dizer "52 agents" quando o código tem 51. |
| **2 — Memória** | Aprendizado validado. Mas memória pode vir de contexto diferente (ex: memória de outro projeto, stack diferente). |
| **1 — Opinião** | Raciocínio do agente. Valioso, mas não verificável sem evidência de código. |
| **0 — LLM** | Modelos estatísticos. Podem alucinar, confabular, ou gerar respostas baseadas em dados de treinamento desatualizados. Sempre verificar. |

---

## Confidence Modifiers

### The 7 Modifiers

Base weight is adjusted by these modifiers. They reflect the quality, recency, and corroboration of the evidence.

| # | Modifier | Adjustment | Applies When |
|---|----------|-----------|--------------|
| **M1** | Com evidência concreta | **+0.15** | Fonte cita localização exata (arquivo, linha, commit hash) |
| **M2** | Validado cross-agent | **+0.10** | Múltiplos agentes independentes reportam a mesma informação |
| **M3** | Recente (< 7 dias) | **+0.05** | Informação foi atualizada na última semana |
| **M4** | Antiga (> 90 dias) | **-0.15** | Informação não é verificada há mais de 3 meses |
| **M5** | Contradita por fonte superior | **-0.40** | Uma fonte de nível mais alto contradiz esta afirmação |
| **M6** | Fonte única (não corroborada) | **-0.10** | Nenhuma outra fonte independente confirma esta informação |
| **M7** | Marcada como aspirational | **-0.30** | Documento explicitamente marcado como meta futura, não realidade atual |

### Modifier Details

#### M1 — Com evidência concreta (+0.15)

A fonte não apenas afirma algo — ela prova.

```
✅ "A API tem 39 comandos CLI" + evidência: internal/cli/ contém 39 arquivos *_command.go
❌ "A API tem 39 comandos CLI" — sem referência a arquivo
```

#### M2 — Validado cross-agent (+0.10)

Dois ou mais agentes independentes chegaram à mesma conclusão.

```
✅ cosca-discovery + cosca-documentation ambos reportam "357 arquivos .go"
❌ Apenas cosca-discovery reporta "357 arquivos .go"
```

#### M3 — Recente (+0.05)

Informação verificada nos últimos 7 dias.

```
✅ go.mod verificado em 2026-07-28 (hoje)
❌ go.mod verificado em 2026-07-01 (27 dias atrás)
```

#### M4 — Antiga (-0.15)

Informação não verificada há mais de 90 dias. Pode estar desatualizada.

```
❌ memory/database-architecture.md — timestamp 2026-07-23, não verificado desde então
```

#### M5 — Contradita por fonte superior (-0.40)

Esta é a penalidade mais severa. Uma fonte de nível superior contradiz diretamente esta afirmação.

```
❌ Memória diz "PostgreSQL" (nível 2, peso base 0.50)
   MAS go.mod importa "modernc.org/sqlite" (nível 5, peso base 1.00)
   → Penalidade M5 aplicada à memória: -0.40
```

#### M6 — Fonte única (-0.10)

Nenhuma outra fonte independente corrobora.

```
⚠️ Apenas um agente reporta "handler X tem bug Y" sem evidência de teste
```

#### M7 — Aspirational (-0.30)

O próprio documento admite que descreve uma meta futura, não o estado atual.

```
❌ compliance-framework.md — status: aspirational
   Descreve GDPR compliance que não existe → penalidade por ser tratado como fato
```

---

## Confidence Calculation

### Formula

```
EvidenceConfidence = clamp(base_weight + sum(modifiers), 0.0, 1.0)

Where:
  base_weight = from Evidence Hierarchy table
  modifiers  = any of M1-M7 that apply
  clamp()    = ensure result is between 0.0 and 1.0
```

### Calculation Examples

#### Example 1: SQLite Reality Check (Fase 1 — July 2026)

```
EVIDENCE A: "Banco de dados é PostgreSQL RDS Multi-AZ"
  Source: memory/architecture/database-architecture.md
  Level: 2 (memória do agente)
  Base weight: 0.50
  Modifiers:
    M4 (antiga > 90 dias): -0.15
    M5 (contradita por nível 5): -0.40
    M6 (fonte única): -0.10
  FINAL CONFIDENCE: 0.50 - 0.15 - 0.40 - 0.10 = 0.00 → clamp(0.00) = 0.00 ❌

EVIDENCE B: "Banco de dados é SQLite (modernc.org/sqlite)"
  Source: go.mod, line 8
  Level: 5 (código executado)
  Base weight: 1.00
  Modifiers:
    M1 (evidência concreta — arquivo, linha): +0.15
  FINAL CONFIDENCE: 1.00 + 0.15 = 1.15 → clamp(1.15) = 1.00 ✅

RESULT: SQLite vence (1.00 vs 0.00). Memória corrigida.
```

#### Example 2: Agent Count (Fase 1 — July 2026)

```
EVIDENCE A: "52 agents"
  Source: README.md (badge)
  Level: 3 (documentação)
  Base weight: 0.60
  Modifiers:
    M7 (contradito por código — internal/ tem 51 agentes): M5 -0.40
  FINAL CONFIDENCE: 0.60 - 0.40 = 0.20 ⚠️

EVIDENCE B: "51 agents"
  Source: Contagem real de capability profiles em memory/agent/
  Level: 5 (diretórios no sistema de arquivos = código executado)
  Base weight: 1.00
  Modifiers:
    M1 (evidência — 51 diretórios): +0.15
  FINAL CONFIDENCE: 1.00 + 0.15 = 1.00 → clamp(1.00) ✅

RESULT: 51 agents vence. README corrigido.
```

#### Example 3: Compliance Claims (Fase 1 — July 2026)

```
EVIDENCE A: "GDPR full compliance achieved 2026-06-01"
  Source: memory/long/compliance-framework.md
  Level: 2 (memória)
  Base weight: 0.50
  Modifiers:
    M7 (documento marcado como aspirational): -0.30
    M6 (fonte única, não corroborada): -0.10
    M5 (contradita por código — sem implementação de GDPR): -0.40
  FINAL CONFIDENCE: 0.50 - 0.30 - 0.10 - 0.40 = 0.00 → clamp(0.00) = 0.00 ❌

RESULT: Compliance claim rejeitada. Documento reescrito como aspirational.
```

---

## Conflict Resolution Algorithm

### When to Trigger

The conflict resolver activates when the Kernel or an agent encounters two or more pieces of evidence that make contradictory claims about the same fact.

### Algorithm

```
function resolve_conflict(claims):
    1. For each evidence source, calculate EvidenceConfidence
    2. Group sources by claim (what they assert)
    3. For each group, take MAX(EvidenceConfidence) as the group's confidence
    4. Sort groups by confidence (descending)
    5. Calculate difference = group[0].confidence - group[1].confidence
    
    IF difference > 0.30:
        RETURN group[0]  // Clear winner
    ELIF difference > 0.10:
        RETURN group[0] WITH WARNING "low confidence margin"
        // Winner exists but close — flag for review
    ELSE:
        ESCALATE to Kernel  // Too close to call automatically
    
    IF Kernel cannot resolve:
        ESCALATE to Don
```

### Decision Table

| Difference | Action | Example |
|-----------|--------|---------|
| > 0.30 | Winner accepted automatically | PostgreSQL (0.00) vs SQLite (1.00) → dif 1.00 |
| 0.10–0.30 | Winner accepted with review flag | Agent count 52 (0.20) vs 51 (1.00) → dif 0.80 |
| < 0.10 | Escalate to Kernel | Two agents with similar confidence disagree |
| Kernel unresolved | Escalate to Don | Complex architectural decision |

### Conflict Resolution Examples

#### Clear Win (difference > 0.30)

```
CLAIM A: "36 REST endpoints" (docs/api-reference/overview.md, conf: 0.60)
CLAIM B: "36 REST endpoints" (api/rest/handler/ directory count, conf: 1.00)
CLAIM C: "34 REST endpoints" (README.md old version, conf: 0.15)

Group A+B confidence: 1.00 (max of agreeing claims)
Group C confidence: 0.15
Difference: 1.00 - 0.15 = 0.85 > 0.30

RESULT: 36 endpoints confirmed. README corrected.
```

#### Ambiguous (difference < 0.10)

```
CLAIM A: "Architecture should be microservices" (CTO opinion, conf: 0.45)
CLAIM B: "Architecture should be modular monolith" (Architecture Chief, conf: 0.42)

Difference: 0.45 - 0.42 = 0.03 < 0.10

RESULT: Escalate to Kernel → Kernel evaluates both arguments → escalates to Don if unresolved.
```

---

## Integration Points

### With Metacognition Pipeline

```
SELF-ASSESS stage:
  → Use EvidenceConfidence to evaluate capability match

RETRIEVE MEMORY stage:
  → Weight search results by EvidenceConfidence
  → Higher confidence memories ranked higher

PLAN STRATEGY stage:
  → Reject plans based on evidence with confidence < 0.30

VERIFY RESULT stage:
  → Validate claims against code (level 5) evidence
```

### With Memory Curation Engine

```
CurationScore calculation uses EvidenceConfidence:
  → Memories backed by level 4-5 evidence get higher CurationScore
  → Memories with M5 (contradicted) penalty are flagged for deprecation
```

### With Agent Capability Profiles

```
Per-domain confidence scores are EvidenceConfidence applied to agent performance:
  → "cosca-backend has confidence 0.92 in REST API architecture"
  → Based on 12 successful tasks (level 4-5 evidence: commits, tests passing)
```

---

## Confidence Thresholds for Action

| Confidence | Action | Applies To |
|-----------|--------|------------|
| **≥ 0.85** | Autonomous execution | Agent capability for task domain |
| **≥ 0.70** | Accept as fact | Information used in decision-making |
| **≥ 0.50** | Consider with caution | Planning, strategy selection |
| **< 0.50** | Escalate / verify | Task exceeds capability, or information unreliable |
| **< 0.30** | Reject / ignore | Information too unreliable to use |
| **0.00** | Actively contradict | Actively replace with correct information |

---

## Automated Checks

The Evolution Engine periodically verifies:

- [ ] All active memory entries have EvidenceConfidence ≥ 0.30
- [ ] No documentation contradicts code without being flagged
- [ ] Memories with M5 (contradicted) penalty are resolved within 7 days
- [ ] Cross-agent validations (M2) are recorded and tracked
- [ ] Information marked as aspirational (M7) is not used in automated decisions

---

> **Related**: [CONSTITUTION.md](../../CONSTITUTION.md) P2 | [LEARNING_PROTOCOL.md](../../memory/LEARNING_PROTOCOL.md) | [MEMORY_CURATION_ENGINE.md](../memory-curation/MEMORY_CURATION_ENGINE.md) | [metacognition-pipeline.md](../../workflows/metacognition-pipeline.md)
