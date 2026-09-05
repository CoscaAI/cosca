# KNOWLEDGE PIPELINE F2-F3 — Validate, Hot Reload, Auto-commit & Review (F9.3)

> **Workflow**: `cosca-knowledge-pipeline-f2-f3`
> **Versão**: 1.0.0 | **Status**: active | **Código**: F9.3
> **Owner**: Architecture Chief (design), Memory Chief (execução), Review Chief (validação)
> **Triggers**: Post-learning (após F1 Source/Parse), Don's order, commit hook, cron (5 min)
> **Dependências**: F9.1 Experience Compiler (`engines/experience-compiler/SKILL.md`), F9.2 Wisdom Distillation (`engines/wisdom-distillation/SKILL.md`), memorize-commit workflow (`workflows/memorize-commit.md`), F7.4 Impact automation (`scripts/hooks/post-commit`)

---

## SUMÁRIO

1. [Objetivo](#1-objetivo)
2. [Pipeline F2-F3 Completo](#2-pipeline-f2-f3-completo)
3. [F2 — Validate + Hot Reload](#3-f2--validate--hot-reload)
4. [F3 — Auto-commit + Review](#4-f3--auto-commit--review)
5. [Regras de Validação (F2.3)](#5-regras-de-validação-f23)
6. [Regras de Hot Reload (F2.4)](#6-regras-de-hot-reload-f24)
7. [Regras de Auto-commit (F3.1)](#7-regras-de-auto-commit-f31)
8. [Regras de Review (F3.2)](#8-regras-de-review-f32)
9. [Integração com F9.1 Experience Compiler](#9-integração-com-f91-experience-compiler)
10. [Integração com F9.2 Wisdom Distillation](#10-integração-com-f92-wisdom-distillation)
11. [Integração com memorize-commit](#11-integração-com-memorize-commit)
12. [Integração com CONSTITUIÇÃO](#12-integração-com-constituição)
13. [CLI e Automação](#13-cli-e-automação)
14. [Métricas do Pipeline](#14-métricas-do-pipeline)
15. [Casos de Borda e Anti-Padrões](#15-casos-de-borda-e-anti-padrões)

---

## 1. OBJETIVO

### 1.1 O que é o Knowledge Pipeline F2-F3

O pipeline F2-F3 completa o ciclo de processamento de conhecimento que faltava entre a fonte bruta (F1 — Source/Parse) e os consumidores de alto nível (F9.1 Experience Compiler, F9.2 Wisdom Distillation):

| Fase | Nome | O que faz | Status |
|------|------|-----------|--------|
| **F1** | Source/Parse | Captura learning entries do filesystem, faz parse do formato markdown, extrai campos | ✅ Existe (`internal/knowledge/`, `sync_source.go`, `sync_parse.go`) |
| **F2** | Validate + Hot Reload | Valida consistência, domínios, não-contradição com CONSTITUIÇÃO; notifica agents em runtime | 🔄 **NOVO** |
| **F3** | Auto-commit + Review | Commita mudanças automaticamente, revisa, merge/push se aprovado | 🔄 **NOVO** |

O pipeline resolve três problemas críticos:

1. **Conhecimento não validado polui a base**: Aprendizados sem validação de tags, domínio, e consistência constitucional entram no `knowledge.db` e são consumidos por agentes como verdade.
2. **Agents operam com dados stale**: Mudanças no `knowledge.db` ou em `learnings.md` não são propagadas para agents em runtime sem restart.
3. **Conhecimento não versionado se perde**: Alterações em `learnings.md` que não são commitadas perdem o rastro de quando e por que foram feitas.

### 1.2 Pipeline em 1 Minuto

```
F1 ─── F2 ─── gate ─── F3 ─── F9.1 / F9.2
│       │               │
Source  Validate        Auto-commit
Parse   Hot Reload      Review
                        Merge/Push
```

Cada learning entry percorre: **captura (F1) → validação (F2.3) → hot reload (F2.4) → auto-commit (F3.1) → review (F3.2) → merge/push (F3.3)** antes de ser consumido pelo Experience Compiler ou Wisdom Distillation.

### 1.3 Analogia: Controle de Qualidade + Versionamento

```
┌─────────────────────────────────────────────────────────────────────────┐
│               KNOWLEDGE PIPELINE — ANALOGIA INDUSTRIAL                    │
│                                                                          │
│  F1 (Source/Parse)    →  Matéria-prima extraída da mina                │
│  F2.1/F2.2 (Source)   →  Já existe (esteira de extração)                │
│  F2.3 (Validate)      →  Controle de qualidade: peça passa no teste?   │
│  F2.4 (Hot Reload)    →  Prateleira atualizada em tempo real           │
│  ─── gate ───          →  Só peça aprovada segue para expedição        │
│  F3.1 (Auto-commit)   →  Carimbo de versão no registro                 │
│  F3.2 (Review)        →  Auditoria de qualidade final                  │
│  F3.3 (Merge/Push)    →  Produto final expedido                        │
│                                                                          │
│  Peça reprovada (F2.3) → Retorna para correção → nunca chega ao F9.1  │
│  Peça aprovada (F3.2)  → Consumida por F9.1 e F9.2                     │
│                                                                          │
│  Tempo alvo total: < 5s (F2) + < 10s (F3) = < 15s                      │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 2. PIPELINE F2-F3 COMPLETO

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                    KNOWLEDGE PIPELINE F2-F3 — FLUXO COMPLETO                    │
│                                                                                │
│  ENTRADA: Output do F1 (learning entries parseados)                            │
│  ┌──────────────────────────────────────────────────────────────────────────┐  │
│  │ F2.1 — SOURCE (já existe)                                                │  │
│  │                                                                           │  │
│  │  File watcher + hash detecta mudanças em:                                 │  │
│  │  ├── memory/agent/*/learnings.md                                          │  │
│  │  ├── memory/agent/*/failures.md                                           │  │
│  │  ├── knowledge/**/*.md (patterns, heuristics, etc.)                       │  │
│  │  └── knowledge.db (SQLite)                                               │  │
│  │                                                                           │  │
│  │  Gera evento `knowledge.source.changed` com diff rows                    │  │
│  │                                                                           │  │
│  │  ⏱ < 1s | Trigger: fsnotify, polling 5s                                 │  │
│  └──────────────────────────────────┬───────────────────────────────────────┘  │
│                                     ▼                                          │
│  ┌──────────────────────────────────────────────────────────────────────────┐  │
│  │ F2.2 — PARSE (já existe)                                                 │  │
│  │                                                                           │  │
│  │  Extrai campos do learning entry format (LEARNING_PROTOCOL.md):           │  │
│  │  ├── Agent: cosca-{name}                                                  │  │
│  │  ├── Task: o que estava sendo feito (contexto)                           │  │
│  │  ├── Technique: técnica específica aplicada                              │  │
│  │  ├── Level: 1-5                                                          │  │
│  │  ├── Outcome: success / partial / failure                                │  │
│  │  ├── Tags: lista de #tags                                                │  │
│  │  ├── Learned: descoberta central                                         │  │
│  │  ├── Domain: inferido das tags + conteúdo                                │  │
│  │  ├── Freshness: do Wisdom Decay (F1.4)                                   │  │
│  │  └── Related: referências                                                 │  │
│  │                                                                           │  │
│  │  Saída: LearningEntry{...} estruturado                                    │  │
│  │  ⏱ < 1s | Trigger: pós F2.1                                             │  │
│  └──────────────────────────────────┬───────────────────────────────────────┘  │
│                                     ▼                                          │
│  ┌──────────────────────────────────────────────────────────────────────────┐  │
│  │ F2.3 — VALIDATE ★ NOVO                                                   │  │
│  │                                                                           │  │
│  │  Valida cada learning entry contra 4 gates:                              │  │
│  │                                                                           │  │
│  │  GATE 1 — Integridade do Formato:                                        │  │
│  │  ├── Todos os campos obrigatórios preenchidos?                            │  │
│  │  ├── Timestamp válido?                                                    │  │
│  │  └── Level entre 1-5?                                                    │  │
│  │                                                                           │  │
│  │  GATE 2 — Consistência de Tags:                                          │  │
│  │  ├── Tags seguem o padrão #kebab-case?                                   │  │
│  │  ├── Pelo menos 1 tag de domínio (ex: #architecture, #security)?         │  │
│  │  └── Sem tags duplicadas no mesmo entry?                                 │  │
│  │                                                                           │  │
│  │  GATE 3 — Domínio Válido:                                                │  │
│  │  ├── Domínio inferido existe no registry?                                │  │
│  │  └── Domínio é conhecido?                                                 │  │
│  │                                                                           │  │
│  │  GATE 4 — Não Contradiz CONSTITUIÇÃO:                                    │  │
│  │  ├── Learning não viola P1-P8?                                           │  │
│  │  ├── Se contradiz → rejeitado com justificativa                          │  │
│  │  └── Se complementa → aprovado com tag #constitutional-checked           │  │
│  │                                                                           │  │
│  │  Saída: VALID (segue) ou INVALID (rejeitado com causa)                   │  │
│  │  ⏱ < 1s por learning | Ver §5 para regras detalhadas                    │  │
│  └──────────────────────────────────┬───────────────────────────────────────┘  │
│                                     ▼                                          │
│  ┌──────────────────────────────────────────────────────────────────────────┐  │
│  │ F2.4 — HOT RELOAD ★ NOVO                                                 │  │
│  │                                                                           │  │
│  │  Se knowledge.db ou learnings.md mudaram:                                │  │
│  │                                                                           │  │
│  │  1. Gera evento `knowledge.updated` no barramento interno                │  │
│  │  2. Notifica agents inscritos no tópico `knowledge`                      │  │
│  │  3. Agents com cache de conhecimento invalidam cache                     │  │
│  │  4. Agents em runtime recarregam referências afetadas                    │  │
│  │  5. NÃO restart — atualização em memória                                 │  │
│  │                                                                           │  │
│  │  Saída: Evento publicado + agents notificados                            │  │
│  │  ⏱ < 2s | Ver §6 para regras detalhadas                                 │  │
│  └──────────────────────────────────┬───────────────────────────────────────┘  │
│                                     ▼                                          │
│           ┌──────────────────────────────────────────────────┐                 │
│           │            GATE F2 → F3                          │                 │
│           │                                                  │                 │
│           │  Só passa para F3 SE:                            │                 │
│           │  ├── Pelo menos 1 learning VALID naᵃ batch       │                 │
│           │  └── Hot reload concluído com sucesso            │                 │
│           │                                                  │                 │
│           │  Se F2.3 rejeitou todos → pipeline para          │                 │
│           │  (erro reportado, nada a commitar)               │                 │
│           └──────────────────┬───────────────────────────────┘                 │
│                              ▼                                                 │
│  ┌──────────────────────────────────────────────────────────────────────────┐  │
│  │ F3.1 — AUTO-COMMIT ★ NOVO                                                │  │
│  │                                                                           │  │
│  │  Para batch de learnings validados:                                      │  │
│  │                                                                           │  │
│  │  1. Verifica se há mudança REAL (diff learnings.md)                      │  │
│  │     ├── Se diff vazio → skip (nada a commitar)                          │  │
│  │     └── Se diff real → continua                                           │  │
│  │                                                                           │  │
│  │  2. Gera mensagem de commit padronizada:                                 │  │
│  │     └── `learn: {domain} — {summary} ({count} learning(s))`             │  │
│  │                                                                           │  │
│  │  3. Executa:                                                              │  │
│  │     ├── git add {arquivo(s) afetados}                                    │  │
│  │     └── git commit -m "{mensagem}"                                       │  │
│  │                                                                           │  │
│  │  4. Se branch for `main` ou `master`:                                    │  │
│  │     └── Cria branch `learn/{timestamp}` e commita lá                     │  │
│  │     └── (para permitir review antes de merge)                            │  │
│  │  5. Se branch for de feature/learning:                                   │  │
│  │     └── Commita direto na branch                                         │  │
│  │                                                                           │  │
│  │  Saída: Commit hash ou skip                                               │  │
│  │  ⏱ < 3s | Ver §7 para regras detalhadas                                 │  │
│  └──────────────────────────────────┬───────────────────────────────────────┘  │
│                                     ▼                                          │
│  ┌──────────────────────────────────────────────────────────────────────────┐  │
│  │ F3.2 — REVIEW ★ NOVO                                                     │  │
│  │                                                                           │  │
│  │  Cosca-review valida o commit de learning:                               │  │
│  │                                                                           │  │
│  │  1. Verifica mensagem do commit (formato `learn: ...`)                  │  │
│  │  2. Verifica diff dos learnings.md para consistência:                    │  │
│  │     ├── Novos entries seguem o formato LEARNING_PROTOCOL?                │  │
│  │     ├── Entries alterados mantêm rastro de edição?                      │  │
│  │     └── Nenhuma remoção não autorizada?                                  │  │
│  │  3. Verifica impacto no knowledge.db:                                    │  │
│  │     ├── Schema compatível?                                                │  │
│  │     └── Nenhuma query FTS5 quebrada?                                     │  │
│  │  4. Gera score de review (0-100)                                         │  │
│  │                                                                           │  │
│  │  Saída: APPROVED, REJECTED (com causas), ou SKIP (se auto-review off)   │  │
│  │  ⏱ < 5s | Ver §8 para regras detalhadas                                │  │
│  └──────────────────────────────────┬───────────────────────────────────────┘  │
│                                     ▼                                          │
│  ┌──────────────────────────────────────────────────────────────────────────┐  │
│  │ F3.3 — MERGE/PUSH ★ NOVO                                                 │  │
│  │                                                                           │  │
│  │  Se F3.2 = APPROVED:                                                     │  │
│  │                                                                           │  │
│  │  1. Se branch for `learn/{timestamp}`:                                   │  │
│  │     ├── git push origin {branch}                                         │  │
│  │     ├── Cria PR automaticamente (se configurado)                         │  │
│  │     └── Ou merge direto para main (se auto-merge configurado)            │  │
│  │  2. Se branch for feature:                                               │  │
│  │     └── git push origin (já está na branch correta)                     │  │
│  │  3. Registra commit no Engineering Timeline (memorize-commit)            │  │
│  │  4. Dispara webhook de notificação (se configurado)                      │  │
│  │                                                                           │  │
│  │  Saída: Push realizado ou PR criado                                      │  │
│  │  ⏱ < 5s | Depende de latência de rede                                   │  │
│  └──────────────────────────────────────────────────────────────────────────┘  │
│                                                                                │
│  SAÍDA: Learning entries commitados + revisados + disponíveis para            │
│         F9.1 Experience Compiler e F9.2 Wisdom Distillation                   │
│                                                                                │
└──────────────────────────────────────────────────────────────────────────────┘
```

### 2.1 Algoritmo Central

```python
def run_knowledge_pipeline_f2f3():
    """
    Pipeline principal F2-F3.
    Executado após F1 (Source/Parse) detectar mudanças.
    """
    # === F2.1: SOURCE (já existe) ===
    changes = detect_file_changes(
        paths=["memory/agent/*/learnings.md",
               "memory/agent/*/failures.md",
               "knowledge/**/*.md"]
    )
    if not changes:
        return {"status": "no_changes", "message": "Nenhuma mudança detectada"}

    # === F2.2: PARSE (já existe) ===
    parsed_entries = []
    for change in changes:
        entry = parse_learning_entry(change.file_path, change.diff)
        if entry:
            parsed_entries.append(entry)

    if not parsed_entries:
        return {"status": "no_entries", "message": "Nenhum entry parseado"}

    # === F2.3: VALIDATE ===
    valid_entries = []
    rejected_entries = []
    for entry in parsed_entries:
        result = validate_learning_entry(entry)  # Ver §5
        if result.valid:
            valid_entries.append(entry)
            entry.validation_status = "valid"
        else:
            rejected_entries.append({
                "entry": entry,
                "reasons": result.reasons
            })
            entry.validation_status = "rejected"

    if not valid_entries:
        return {
            "status": "all_rejected",
            "rejected": rejected_entries,
            "message": f"{len(rejected_entries)} entry(s) rejeitado(s)"
        }

    # === F2.4: HOT RELOAD ===
    reload_result = hot_reload_knowledge(valid_entries)  # Ver §6
    if not reload_result.success:
        log.warning(f"Hot reload parcial: {reload_result.errors}")

    # === GATE F2 → F3 ===
    # Se não há entries válidas, não commita

    # === F3.1: AUTO-COMMIT ===
    commit_result = auto_commit_learnings(valid_entries)  # Ver §7
    if commit_result.skipped:
        return {"status": "no_changes_to_commit", "valid_entries": len(valid_entries)}

    # === F3.2: REVIEW ===
    if is_review_enabled():  # Don pode desligar
        review_result = review_learning_commit(commit_result)  # Ver §8
        if review_result.status == "REJECTED":
            return {
                "status": "review_rejected",
                "commit": commit_result,
                "review": review_result,
                "message": "Commit rejeitado na review"
            }
    else:
        review_result = {"status": "SKIPPED", "reason": "Review desligado pelo Don"}

    # === F3.3: MERGE/PUSH ===
    if commit_result.created_branch:
        push_result = merge_push_learning(commit_result, review_result)

    # Gera relatório
    report = generate_f2f3_report(
        parsed=len(parsed_entries),
        valid=len(valid_entries),
        rejected=rejected_entries,
        commit=commit_result,
        review=review_result
    )

    return report
```

### 2.2 Performance

| Operação | Complexidade | Tempo Estimado |
|----------|-------------|----------------|
| F2.1 Source (file hash + diff) | O(F) onde F = files monitorados | < 1s |
| F2.2 Parse (markdown extraction) | O(E) onde E = entries | < 1s |
| F2.3 Validate (4 gates) | O(E × G) onde G = gates (4) | < 1s (por entry) |
| F2.4 Hot Reload (event + notify) | O(A) onde A = agents inscritos | < 2s |
| F3.1 Auto-commit (git add + commit) | O(D) onde D = diff size | < 3s |
| F3.2 Review (diff analysis) | O(D + E) | < 5s |
| F3.3 Merge/Push | O(1) + latência de rede | < 5s |
| **Total** | | **< 15s** |

> **E = entries na batch, F = files monitorados, G = gates de validação, A = agents, D = diff size**

---

## 3. F2 — VALIDATE + HOT RELOAD

### 3.1 Gatilhos de Execução

| Gatilho | Descrição | Comportamento |
|---------|-----------|---------------|
| **Post-F1** | Após F1 Source/Parse processar mudanças | Pipeline completo F2+F3 |
| **fsnotify** | Mudança em tempo real em learnings.md | F2.3 + F2.4 apenas (sem F3) |
| **Cron 5min** | Polling periódico | Detecta mudanças que watcher perdeu |
| **Don order** | `cosca knowledge pipeline --f2-f3` | Execução explícita |
| **Post-commit** | Após hook post-commit (se houver learnings) | F2.3 + F2.4 para validar o que foi commitado |

### 3.2 Fluxo Detalhado F2.3 — Validate

```
                    ENTRADA: Learning entry parseado
                              │
                              ▼
              ┌─────────────────────────────────┐
              │ GATE 1: Integridade do Formato   │
              │                                 │
              │ [1.1] Campos obrigatórios:       │
              │       Agent, Task, Technique,    │
              │       Level, Outcome, Tags,      │
              │       Learned (ou Learned)       │
              │                                 │
              │ [1.2] Timestamp: formato ISO?   │
              │ [1.3] Level: 1-5?               │
              │ [1.4] Outcome: success/partial/  │
              │        failure?                  │
              └─────────────┬───────────────────┘
                            │
                    ┌───────┴───────┐
                    ▼               ▼
              ┌──────────┐    ┌──────────┐
              │ VÁLIDO   │    │ INVÁLIDO │──→ REJEITADO + causa
              └────┬─────┘    └──────────┘
                   ▼
              ┌─────────────────────────────────┐
              │ GATE 2: Consistência de Tags     │
              │                                 │
              │ [2.1] Tags em #kebab-case?      │
              │ [2.2] Pelo menos 1 tag de       │
              │       domínio?                  │
              │ [2.3] Sem tags duplicadas?      │
              │ [2.4] Tags existem no registry? │
              └─────────────┬───────────────────┘
                            │
                    ┌───────┴───────┐
                    ▼               ▼
              ┌──────────┐    ┌──────────┐
              │ VÁLIDO   │    │ INVÁLIDO │──→ REJEITADO + correção
              └────┬─────┘    └──────────┘
                   ▼
              ┌─────────────────────────────────┐
              │ GATE 3: Domínio Válido           │
              │                                 │
              │ [3.1] Domínio inferido existe   │
              │       no domain registry?        │
              │ [3.2] Domínio é conhecido?       │
              │ [3.3] Se domínio novo → flag     │
              └─────────────┬───────────────────┘
                            │
                    ┌───────┴───────┐
                    ▼               ▼
              ┌──────────┐    ┌──────────┐
              │ VÁLIDO   │    │ INVÁLIDO │──→ REJEITADO + flag
              └────┬─────┘    └──────────┘
                   ▼
              ┌─────────────────────────────────┐
              │ GATE 4: Contradição com          │
              │         CONSTITUIÇÃO             │
              │                                 │
              │ [4.1] Learning viola P1-P8?     │
              │ [4.2] Se contradiz → rejeitado  │
              │       com justificativa          │
              │ [4.3] Se complementa → tag      │
              │       #constitutional-checked   │
              │ [4.4] Se neutro → aprovado      │
              └─────────────┬───────────────────┘
                            │
                    ┌───────┴───────┐
                    ▼               ▼
              ┌──────────┐    ┌──────────┐
              │ VÁLIDO   │    │ INVÁLIDO │──→ REJEITADO + causa
              └────┬─────┘    └──────────┘
                   ▼
              SAÍDA: Learning entry VALID
                     (ready for hot reload + commit)
```

### 3.3 Fluxo Detalhado F2.4 — Hot Reload

```
                    ENTRADA: Batch de entries VALID
                              │
                              ▼
              ┌─────────────────────────────────┐
              │ [1] Verificar mudança real      │
              │                                 │
              │ Compara hash atual do           │
              │ knowledge.db com último hash    │
              │ conhecido.                      │
              │                                 │
              │ Se igual → SKIP (nada a         │
              │   recarregar)                   │
              │ Se diferente → continua         │
              └─────────────┬───────────────────┘
                            ▼
              ┌─────────────────────────────────┐
              │ [2] Publicar evento             │
              │     `knowledge.updated`         │
              │                                 │
              │ Payload:                        │
              │ - type: "learning" | "failure"  │
              │   | "pattern"                   │
              │ - action: "create" | "update"   │
              │   | "delete"                    │
              │ - entries: [{id, domain, tags,  │
              │   summary}]                     │
              │ - timestamp: ISO8601            │
              │ - source: "f2.4-hot-reload"     │
              └─────────────┬───────────────────┘
                            ▼
              ┌─────────────────────────────────┐
              │ [3] Notificar agents inscritos  │
              │                                 │
              │ Agents que se inscreveram no    │
              │ tópico "knowledge":             │
              │                                 │
              │ cosca-semantic-memory           │
              │   → Atualiza índice de busca    │
              │ cosca-memory-chief              │
              │   → Verifica integridade        │
              │ cosca-evolution                 │
              │   → Reavalia padrões            │
              │ cosca-experience-compiler       │
              │   → Prepara entrada p/ F9.1     │
              │ cosca-wisdom-distillation       │
              │   → Prepara entrada p/ F9.2     │
              │ cosca-review                    │
              │   → Se pending review            │
              └─────────────┬───────────────────┘
                            ▼
              ┌─────────────────────────────────┐
              │ [4] Invalidar caches            │
              │                                 │
              │ Agents que têm cache de         │
              │ conhecimento:                   │
              │                                 │
              │ - cosca-kernel: context cache   │
              │ - cosca-discovery: codebase     │
              │   cache                         │
              │ - cosca-cli: knowledge cache    │
              │ - Qualquer agente com           │
              │   confidence baseada em         │
              │   learnings                     │
              └─────────────┬───────────────────┘
                            ▼
              SAÍDA: Agents notificados + caches invalidados
                     (sem restart, sem downtime)
```

---

## 4. F3 — AUTO-COMMIT + REVIEW

### 4.1 Fluxo Detalhado F3.1 — Auto-Commit

```
                    ENTRADA: Batch de entries VALID + hot reload OK
                              │
                              ▼
              ┌─────────────────────────────────┐
              │ [1] Verificar diff real         │
              │                                 │
              │ git diff learnings.md           │
              │ (ou arquivo afetado)            │
              │                                 │
              │ Se diff vazio → SKIP            │
              │   (nada a commitar)             │
              │ Se diff real → continua         │
              └─────────────┬───────────────────┘
                            ▼
              ┌─────────────────────────────────┐
              │ [2] Gerar mensagem padronizada  │
              │                                 │
              │ Formato:                        │
              │ learn: {domain} — {summary}     │
              │   ({N} learning(s))             │
              │                                 │
              │ Exemplos:                       │
              │ learn: architecture —           │
              │   Cognitive Economy ROI         │
              │   formula (3 learnings)         │
              │ learn: security — XSS           │
              │   detection via CSP headers     │
              │   (1 learning)                  │
              │ learn: testing — Race           │
              │   condition patterns            │
              │   (2 learnings)                 │
              │                                 │
              │ Se múltiplos domínios:          │
              │ learn: multi-domain —           │
              │   {summary} ({N} learnings      │
              │   across {M} domains)           │
              └─────────────┬───────────────────┘
                            ▼
              ┌─────────────────────────────────┐
              │ [3] Executar git                │
              │                                 │
              │ BRANCH ACTUAL = main/master?    │
              │   ├── SIM                        │
              │   │   Cria branch:               │
              │   │   learn/{timestamp}-         │
              │   │   {short-hash}               │
              │   │   git checkout -b {branch}   │
              │   │   git add {arquivos}         │
              │   │   git commit -m "{msg}"      │
              │   │                             │
              │   └── NÃO (já em feature/       │
              │        learning branch)          │
              │       git add {arquivos}         │
              │       git commit -m "{msg}"      │
              │                                 │
              │ Se erro: aborta, loga, notifica │
              └─────────────┬───────────────────┘
                            ▼
              ┌─────────────────────────────────┐
              │ [4] Registrar metadados         │
              │                                 │
              │ - commit_hash: HEAD             │
              │ - branch: learn/{timestamp}...  │
              │ - entries_count: N              │
              │ - domains: [architecture, ...]  │
              │ - timestamp: ISO8601            │
              │ - trigger: "auto-f2f3"          │
              └─────────────┬───────────────────┘
                            ▼
              SAÍDA: Commit realizado (ou skip)
```

### 4.2 Fluxo Detalhado F3.2 — Review

```
                    ENTRADA: Commit realizado
                              │
                              ▼
              ┌─────────────────────────────────┐
              │ [0] Review habilitada?          │
              │                                 │
              │ Se Don desligou review → SKIP   │
              │   (pula para F3.3)              │
              │ Se habilitada → continua        │
              └─────────────┬───────────────────┘
                            ▼
              ┌─────────────────────────────────┐
              │ [1] Validar mensagem do commit  │
              │                                 │
              │ Regex: ^learn: .+ — .+ \(\d+   │
              │   learning(s)\)$                │
              │                                 │
              │ Se inválido → -10 pontos        │
              │ Se ausente → REJECT             │
              └─────────────┬───────────────────┘
                            ▼
              ┌─────────────────────────────────┐
              │ [2] Validar diff                │
              │                                 │
              │ Verifica:                       │
              │ - Novos entries seguem          │
              │   LEARNING_PROTOCOL.md?         │
              │ - Entries alterados têm         │
              │   `**Updated**: {data}`?        │
              │ - Nenhuma remoção sem           │
              │   justificativa?                 │
              │ - Formato markdown válido?      │
              │                                 │
              │ Cada violação: -15 pontos       │
              │ Múltiplas violações: REJECT     │
              └─────────────┬───────────────────┘
                            ▼
              ┌─────────────────────────────────┐
              │ [3] Validar impacto knowledge   │
              │                                 │
              │ Verifica:                       │
              │ - Schema do knowledge.db        │
              │   compatível?                    │
              │ - Nenhuma FTS5 query quebrada?  │
              │ - Índices intactos?              │
              │                                 │
              │ Se problema: REJECT             │
              └─────────────┬───────────────────┘
                            ▼
              ┌─────────────────────────────────┐
              │ [4] Calcular score final        │
              │                                 │
              │ score = max(0, 100 - penalties) │
              │                                 │
              │ score ≥ 70 → APPROVED           │
              │ 50 ≤ score < 70 → APPROVED      │
              │   com ressalvas                 │
              │ score < 50 → REJECTED           │
              │                                 │
              │ Se REJECTED:                    │
              │ ├── Desfaz o commit             │
              │ │   (git reset HEAD~1)          │
              │ └── Reporta causas ao Don       │
              └─────────────────────────────────┘
```

### 4.3 Fluxo Detalhado F3.3 — Merge/Push

```
                    ENTRADA: Commit + Review APPROVED
                              │
                              ▼
              ┌─────────────────────────────────┐
              │ Branch é learn/{timestamp}?     │
              │                                 │
              ├── SIM                            │
              │   ┌──────────────────────────┐  │
              │   │ git push origin {branch} │  │
              │   └──────────┬───────────────┘  │
              │              ▼                   │
              │   ┌──────────────────────────┐  │
              │   │ Auto-merge configurado?  │  │
              │   ├── SIM: git checkout main │  │
              │   │        git merge {branch}│  │
              │   │        git push origin   │  │
              │   │        main              │  │
              │   │                         │  │
              │   └── NÃO: Cria PR via       │  │
              │        gh pr create          │  │
              │        --title "{msg}"       │  │
              │        --body "Auto PR:      │  │
              │         {count} learnings   │  │
              │         em {domain}"        │  │
              │        --base main           │  │
              │                             │  │
              ├── NÃO (já em feature/       │  │
              │     learning branch)         │  │
              │   └── git push origin       │  │
              │                             │  │
              └─────────────────────────────┘  │
                            ▼
              ┌─────────────────────────────────┐
              │ Registrar na Timeline           │
              │                                 │
              │ Chama memorize-commit workflow  │
              │ (parcial: só registro, sem      │
              │  análise completa de impacto)   │
              │                                 │
              │ Entrada na timeline:            │
              │ learn {emoji} {summary}         │
              │   +N/-0 {files} ${cost}         │
              └─────────────┬───────────────────┘
                            ▼
              SAÍDA: Conhecimento commitado + revisado + disponível
```

---

## 5. REGRAS DE VALIDAÇÃO (F2.3)

### 5.1 Gate 1 — Integridade do Formato

```
GATE 1: Integridade do Formato
════════════════════════════════

Regras:
  [1.1] Todos os campos obrigatórios devem estar preenchidos
  [1.2] Timestamp deve ser ISO 8601 válido (YYYY-MM-DD)
  [1.3] Level deve ser 1, 2, 3, 4, ou 5
  [1.4] Outcome deve ser "success", "partial", ou "failure"
  [1.5] Tags deve conter pelo menos 1 tag
  [1.6] Learned (ou Learned) não pode estar vazio

Campos Obrigatórios (LEARNING_PROTOCOL.md v3.1.0):
  - Agent (string)
  - Task (string, contexto)
  - Technique (string, técnica aplicada)
  - Level (int, 1-5)
  - Outcome (enum: success|partial|failure)
  - Tags (list, min 1)
  - Learned (string, descoberta central)

Campos Opicionais (não bloqueiam, mas geram warning):
  - Related (string, referências)
  - Next (string, próximo passo)
  - Wisdom Decay Category (CRITICAL|STABLE|EXPERIMENTAL|DEPRECATED)
  - Last Validated (date)
  - Confidence (float, 0.00-1.00)
  - Expires At (date)

Campos Proibidos:
  - Conteúdo vazio em qualquer campo obrigatório
  - Tags com caracteres especiais (exceto # e -)
  - Level fora do range 1-5

Ações:
  ✅ Válido: Segue para Gate 2
  ❌ Inválido: REJEITADO com lista de campos ausentes/inválidos
     O learning não entra no knowledge.db até ser corrigido
```

### 5.2 Gate 2 — Consistência de Tags

```
GATE 2: Consistência de Tags
════════════════════════════

Regras:
  [2.1] Tags devem seguir o padrão #kebab-case
        ✅ #architecture, #security, #xss-detection, #go-test-race
        ❌ #Architecture, #XSS_Detection, #Go test race, #minhadica

  [2.2] Pelo menos 1 tag de domínio do registry
        Domínios válidos: architecture, security, testing, devops,
        performance, frontend, backend, ai, database, cache,
        messaging, monitoring, analytics, cli, sdk, platform,
        documentation, infrastructure, compliance, governance,
        evolution, runtime, memory, knowledge, integration,
        automation, quality, mobile, plugin, workflow

  [2.3] Sem tags duplicadas no mesmo entry
        Se uma tag aparece duas vezes → warning (não bloqueante)

  [2.4] Tags desconhecidas geram warning (não bloqueante)
        Tags não encontradas no registry são aceitas mas sinalizadas
        para revisão manual periódica

Ações:
  ✅ Válido: Segue para Gate 3
  ❌ Inválido: REJEITADO com sugestão de correção
     ⚠ Warning: Tag não encontrada no registry (não bloqueia)
```

### 5.3 Gate 3 — Domínio Válido

```
GATE 3: Domínio Válido
════════════════════════

Regras:
  [3.1] Domínio inferido das tags + conteúdo DEVE ser um domínio conhecido
        A inferência usa: tag de domínio explícita OU similaridade com
        descrição de domínio conhecido

  [3.2] Domínios válidos (mesmo registry do Gate 2):
        architecture, security, testing, devops, performance,
        frontend, backend, ai, database, cache, messaging,
        monitoring, analytics, cli, sdk, platform, documentation,
        infrastructure, compliance, governance, evolution, runtime,
        memory, knowledge, integration, automation, quality, mobile,
        plugin, workflow

  [3.3] Domínio novo (não no registry):
        Se o domínio não existe no registry:
          → ACCEPT com flag `#new-domain`
          → Notifica Kernel para avaliar inclusão no registry
          → Learning entra, mas fica em "domínio não oficial" até
            confirmação

  [3.4] Multi-domínio:
        Se o learning pertence a múltiplos domínios:
          → Tags multi-domínio são permitidas
          → Domínio primário é o primeiro tag de domínio explícito
          → Learning é indexado em TODOS os domínios relevantes

Ações:
  ✅ Válido: Segue para Gate 4
  ❌ Inválido (domínio não reconhecido sem tags de domínio):
     REJEITADO — adicionar tag de domínio explícita
  ⚠ Flag #new-domain (não bloqueia, notifica Kernel)
```

### 5.4 Gate 4 — Não Contradiz CONSTITUIÇÃO

```
GATE 4: Não Contradiz CONSTITUIÇÃO
══════════════════════════════════

Regras:
  [4.1] Verificação de contradição com P1-P8:
        Para cada learning, verificar se a afirmação central conflita
        com algum princípio imutável da CONSTITUIÇÃO.

  Algoritmo de verificação:
    Para cada princípio P (P1 a P8):
      learning.violates(P) = contradição_direta(learning, P) OR
                             violação_implicada(learning, P)

  [4.2] Contradição direta:
        Se learning afirma o OPOSTO de um princípio → VIOLAÇÃO
        Exemplo: Learning "Segurança pode ser sacrificada por
        performance" contradiz P1 (Segurança acima de funcionalidade)

  [4.3] Violação implicada:
        Se learning, se seguido, levaria a violação de princípio
        → VIOLAÇÃO
        Exemplo: Learning "Remover arquivos do embed diretamente
        acelera deploy" contradiz P8 (Integridade do Embed)

  [4.4] Complemento (não contradiz):
        Se learning adiciona detalhe ou extensão a um princípio
        → APROVADO com tag #constitutional-checked
        Exemplo: Learning "P2 também se aplica a configurações de
        runtime" complementa P2

  [4.5] Neutro:
        Se learning não tem relação com princípios → APROVADO

  [4.6] Matriz de verificação rápida:

    | Princípio | Tema | O que verificar |
    |-----------|------|-----------------|
    | P1 | Segurança | Learning sugere ignorar segurança por velocidade? |
    | P2 | Código é verdade | Learning contradiz hierarquia de fontes? |
    | P3 | Rastro | Learning omite decisão que deveria ser registrada? |
    | P4 | Don veto | Learning sugere ignorar ordem do Don? |
    | P5 | Aprender c/ erros | Learning sugere esconder falha? |
    | P6 | Evolução s/ regressão | Learning sugere técnica inferior? |
    | P7 | Memória s/ poluição | Learning é duplicata ou obsoleto? |
    | P8 | Integridade embed | Learning sugere remoção sem procedimento? |

  [4.7] Ações:

    ✅ Aprovado (neutro): Segue para hot reload
    ✅ Aprovado (complementa): Tag #constitutional-checked + segue
    ❌ Rejeitado (contradiz): REJEITADO com justificativa:
       "O learning '{summary}' contradiz {Px} ({principio}).
        Justificativa: {detalhe}. O learning não será indexado
        até ser corrigido ou removido."
    ❌ Rejeitado (viola): REJEITADO — mesma ação

Ações:
  ✅ Válido: Segue para F2.4 Hot Reload
  ❌ Inválido: REJEITADO com justificativa constitucional
```

---

## 6. REGRAS DE HOT RELOAD (F2.4)

### 6.1 Arquitetura de Eventos

```
┌─────────────────────────────────────────────────────────────────┐
│                    BARRAVENTO DE EVENTOS                          │
│                                                                   │
│  Event: knowledge.updated                                         │
│  ┌──────────────────────────────────────────────┐                │
│  │ {
│  │   "type": "knowledge.updated",
│  │   "version": "1.0.0",
│  │   "id": "evt-2026-07-30-001",
│  │   "timestamp": "2026-07-30T15:30:00Z",
│  │   "source": "f2.4-hot-reload",
│  │   "changes": [
│  │     {
│  │       "action": "create",          // create | update | delete
│  │       "file": "memory/agent/cosca-architecture/learnings.md",
│  │       "entries": [
│  │         {
│  │           "id": "2026-07-30",
│  │           "domain": "architecture",
│  │           "tags": ["#f9.3", "#knowledge-pipeline"],
│  │           "summary": "Knowledge Pipeline F2-F3 design"
│  │         }
│  │       ]
│  │     }
│  │   ],
│  │   "hash": "sha256:abc123...",
│  │   "agents_notified": ["cosca-semantic-memory", ...]
│  │ }                                                                     │
│  └──────────────────────────────────────────────┘                │
│                                                                   │
│  Inscritos no tópico "knowledge":                                 │
│  ┌─────────────────────────────┬──────────────────────────────┐  │
│  │ Agent                       │ Ação no evento               │  │
│  ├─────────────────────────────┼──────────────────────────────┤  │
│  │ cosca-semantic-memory       │ Reindexar buscas             │  │
│  │ cosca-memory-chief          │ Verificar integridade        │  │
│  │ cosca-evolution             │ Reavaliar padrões            │  │
│  │ cosca-experience-compiler   │ Marcar novos inputs          │  │
│  │ cosca-wisdom-distillation   │ Marcar novos inputs          │  │
│  │ cosca-review                │ Preparar review (se pending) │  │
│  │ cosca-kernel                │ Atualizar context cache      │  │
│  │ cosca-discovery             │ Invalidar codebase cache     │  │
│  │ cosca-cli                   │ Invalidar knowledge cache    │  │
│  └─────────────────────────────┴──────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

### 6.2 Regras de Notificação

| Regra | Descrição |
|-------|-----------|
| **Notificação seletiva** | Só notifica agents que se inscreveram no tópico `knowledge` |
| **Payload mínimo** | Evento contém apenas: tipo, ação, entries (id + domínio + tags + summary) |
| **Sem blocking** | Notificação é assíncrona — pipeline F3 não espera resposta dos agents |
| **Idempotência** | Mesmo evento pode ser publicado múltiplas vezes sem efeito colateral |
| **TTL do evento** | Evento expira após 5 minutos (não replay de eventos velhos) |
| **Log obrigatório** | Todo evento é logado para auditoria |

### 6.3 Invalidação de Cache por Agent

| Agent | Cache | Estratégia de Invalidação |
|-------|-------|--------------------------|
| cosca-semantic-memory | Índice de busca semântica | Reindexar entries afetados (delta) |
| cosca-kernel | Contexto de conhecimento | Invalidar cache de learned patterns |
| cosca-discovery | Mapa de código → conhecimento | Remover entradas obsoletas |
| cosca-cli | Cache de knowledge queries | Invalidar query cache do domínio |
| cosca-evolution | Padrões detectados | Reprocessar padrões no próximo ciclo |
| cosca-experience-compiler | Inputs de compilação | Marcar novos learnings como pending |
| cosca-wisdom-distillation | Inputs de destilação | Marcar novos inputs como pending |

### 6.4 Performance

| Operação | Tempo Alvo | Pior Caso |
|----------|-----------|-----------|
| Gerar evento | < 10ms | < 50ms |
| Notificar 1 agent | < 50ms | < 200ms |
| Notificar 9 agents (paralelo) | < 200ms | < 500ms |
| Invalidar cache (por agent) | < 50ms | < 100ms |
| **Total F2.4** | **< 2s** | **< 5s** |

---

## 7. REGRAS DE AUTO-COMMIT (F3.1)

### 7.1 Formato da Mensagem de Commit

```
Formato canônico:

  learn: {domain} — {summary} ({count} learning(s))

Onde:
  {domain}  = domínio primário (architecture, security, testing, etc.)
              Se multi-domínio: "multi-domain"
  {summary} = resumo curto (< 80 chars) do que foi aprendido
  {count}   = número de learning entries neste commit

Exemplos:

  learn: architecture — Cognitive Economy ROI formula (3 learnings)
  learn: security — XSS detection via CSP headers (1 learning)
  learn: testing — Race condition patterns in concurrent Go (2 learnings)
  learn: multi-domain — Pipeline de auto-evolution (4 learnings across 3 domains)
  learn: devops — Post-commit hook optimization (1 learning)

Comprimento máximo: 100 caracteres (limite git + clareza)
```

### 7.2 Regras para Evitar Commits Vazios

| Regra | Descrição |
|-------|-----------|
| **Só commita se diff real** | `git diff --stat` deve mostrar mudanças |
| **Ignora whitespace** | `git diff --ignore-all-space` — mudanças só de whitespace não contam |
| **Ignora timestamp auto** | Se só mudou a data de `Last Validated`, não commita |
| **Ignora metadata de confidence** | Se só mudou o confidence score (decay automático), não commita |
| **Batch de 5 segundos** | Mudanças dentro de 5s são agrupadas em 1 commit |
| **Máximo de 50 entries por commit** | Acima disso, divide em múltiplos commits |

### 7.3 Tratamento de Branch

```
BRANCH ACTUAL:
├── main / master
│   ├── Cria branch: learn/{timestamp}-{short-hash}
│   ├── Commita na branch temporária
│   ├── F3.2 Review valida
│   └── F3.3 Merge/Push (ou PR)
│
├── learn/{timestamp} (já em branch de learning)
│   ├── Commita direto
│   ├── F3.2 Review valida
│   └── F3.3 Push (ou PR para main)
│
├── feature/* (branch de feature)
│   ├── Commita direto (learning é parte da feature)
│   ├── F3.2 Review valida
│   └── F3.3 Push (já está na branch correta)
│
└── outra branch não reconhecida
    ├── Commita direto
    ├── F3.2 Review valida
    └── F3.3 Push (sem merge automático)
```

### 7.4 Segurança

| Regra | Descrição |
|-------|-----------|
| **Nunca força push** | `git push` normal — sem `--force` em nenhuma circunstância |
| **Nunca commita fora do escopo** | Só os arquivos de learnings/failures afetados |
| **Nunca commita secrets** | Verificação automática de secrets no diff |
| **Aborta se conflito** | Se `git add` falhar ou `git commit` tiver conflito, aborta tudo |
| **Log de auditoria** | Todo auto-commit é registrado com agent responsável + trigger |

---

## 8. REGRAS DE REVIEW (F3.2)

### 8.1 Score de Review

```
Score = max(0, 100 - penalties)

Penalties:
  - Mensagem de commit fora do padrão: -10
  - Learning entry fora do formato LEARNING_PROTOCOL: -15 (cada)
  - Remoção sem justificativa: -20 (cada)
  - Formato markdown inválido: -10
  - Schema knowledge.db incompatível: -25
  - FTS5 query quebrada: -25
  - Índices corrompidos: -30

Thresholds:
  score ≥ 70  → ✅ APPROVED
  50 ≤ score < 70 → ⚠️ APPROVED WITH RESERVATIONS
  score < 50  → ❌ REJECTED

Se REJECTED:
  1. git reset HEAD~1 (desfaz o commit)
  2. Registra review com causas
  3. Notifica Don
  4. Learning volta para pool (não é perdido)
```

### 8.2 Don Pode Desligar

```
Review pode ser desligado pelo Don de duas formas:

1. Configuração global (opencode.json ou cosca.config.yaml):
   knowledge_pipeline:
     auto_review: false   # Desliga review automático

2. Flag por execução:
   cosca knowledge pipeline --f2-f3 --no-review

3. Variável de ambiente:
   COSCA_KNOWLEDGE_REVIEW_DISABLED=true

Quando review está desligado:
  - F3.2 é SKIPPED
  - F3.3 executa imediatamente após F3.1
  - Log: "Review skipped by Don configuration"
  - Aviso: "Learning entries committed without review"
```

### 8.3 Modo Silencioso vs Completo

| Modo | Quando usar | Ações |
|------|-------------|-------|
| **Completo** | Padrão (review habilitado) | Valida mensagem, diff, knowledge.db |
| **Silencioso** | Don desligou review | Skip — apenas log |
| **Parcial** | Apenas validação de diff | Só verifica formato, sem knowledge.db |

---

## 9. INTEGRAÇÃO COM F9.1 EXPERIENCE COMPILER

### 9.1 Contrato de Integração

```
O Experience Compiler (F9.1) é CONSUMIDOR do output do pipeline F2-F3.
A relação é UNIDIRECIONAL: F2-F3 entrega, F9.1 consome.

┌─────────────────────────┐  learnings validados   ┌─────────────────────────┐
│  F2-F3 Knowledge        │  + commitados           │  Experience Compiler    │
│  Pipeline               │ ─────────────────────▶ │  (F9.1)                 │
│                         │                         │                         │
│  - Learnings VALID      │  - Freshness > 0.5      │  - Fase 1: Coleta      │
│  - Learnings COMMITTED  │  - Format validated     │  - Fase 2: Agrupamento  │
│  - Metadata por entry   │  - Constitutional safe  │  - Fase 3: Extração    │
│                         │                         │  - Fase 4: Validação   │
│                         │                         │  - Fase 5: Compilação  │
└─────────────────────────┘                         └─────────────────────────┘
```

### 9.2 Interface de Dados

O F9.1 consome do diretório `memory/agent/*/learnings.md` (já validado pelo F2.3).
Além disso, recebe metadados do pipeline F2-F3:

| Campo | Tipo | Origem | Descrição |
|-------|------|--------|-----------|
| `validation_status` | enum | F2.3 | "valid" \| "rejected" |
| `validation_gates` | object | F2.3 | Resultado de cada gate (1-4) |
| `constitutional_check` | object | F2.3 | Resultado do Gate 4 |
| `commit_hash` | string | F3.1 | Hash do commit que versionou o learning |
| `commit_timestamp` | ISO8601 | F3.1 | Quando foi commitado |
| `review_status` | enum | F3.2 | "approved" \| "rejected" \| "skipped" |
| `review_score` | int | F3.2 | Score de review (0-100) se aplicável |

### 9.3 Pipeline Integrado (F2-F3 + F9.1)

```
F2.1 Source ──┐
F2.2 Parse  ──┤
F2.3 Validate ─┤
F2.4 Hot Reload ┤
               │
               ▼
┌─────────────────────────────────────────────────┐
│              GATE F2-F3 → F9.1                   │
│                                                  │
│  F9.1 Experience Compiler SÓ processa            │
│  learnings que passaram pelo F2-F3:              │
│                                                  │
│  ├── validation_status = "valid"                 │
│  ├── commit_hash != null (versionado)            │
│  └── (review_status = "approved" ou "skipped")   │
│                                                  │
│  Learnings REJEITADOS no F2.3 são EXCLUÍDOS      │
│  da compilação (F9.1 nem os vê).                 │
│                                                  │
│  Learnings não commitados (ainda em edição)      │
│  são IGNORADOS (F9.1 só processa versionado).   │
└─────────────────────────────────────────────────┘
```

### 9.4 Regras de Consumo

| Regra | Descrição |
|-------|-----------|
| **Só consome validado** | F9.1 só processa learnings com `validation_status = "valid"` |
| **Só consome versionado** | F9.1 só processa learnings com `commit_hash` definido |
| **Freshness contínua** | Freshness ainda é gerenciado por F1.4 Wisdom Decay |
| **Feedback de compilação** | F9.1 marca learnings como `#compiled` após processamento |
| **Learnings rejeitados** | F9.1 nunca vê learnings rejeitados no F2.3 |

---

## 10. INTEGRAÇÃO COM F9.2 WISDOM DISTILLATION

### 10.1 Contrato de Integração

```
A Wisdom Distillation (F9.2) é CONSUMIDORA do output do F9.1 Experience Compiler,
que por sua vez consumiu o output do F2-F3. A relação é INDIRETA:

F2-F3 → F9.1 → F9.2

O F9.2 herda todas as garantias de qualidade do F2-F3:
  - Se um learning passou pelo F2-F3, ele é válido, consistente e constitucionalmente seguro
  - Se um learning foi rejeitado no F2.3, ele nunca chega ao F9.2
  - Se um learning foi commitado (F3.1), ele é rastreável até o commit
```

### 10.2 Garantias do F2-F3 para F9.2

| Garantia | Origem | Impacto no F9.2 |
|----------|--------|-----------------|
| **Formato válido** | F2.3 Gate 1 | F9.2 não precisa re-validar formato |
| **Tags consistentes** | F2.3 Gate 2 | F9.2 pode confiar nas tags para agrupamento |
| **Domínio conhecido** | F2.3 Gate 3 | F9.2 não precisa inferir domínio |
| **Constitutional safe** | F2.3 Gate 4 | F9.2 não precisa verificar contradição com CONSTITUIÇÃO |
| **Versionado** | F3.1 | F9.2 pode referenciar commit hash como evidência |
| **Revisado** | F3.2 | F9.2 sabe que o learning passou por revisão |

### 10.3 Pipeline Integrado Completo (F1 → F9.2)

```
Pipeline completo de conhecimento:

F1 SOURCE/PARSE
  │  (File watcher + markdown parser)
  ▼
F2-F3 VALIDATE + COMMIT ★ NOVO
  │  (4 gates + hot reload + auto-commit + review)
  ▼
F9.1 EXPERIENCE COMPILER
  │  (5 fases: coleta → agrupamento → extração → validação → compilação)
  ▼
F9.2 WISDOM DISTILLATION
  │  (5 níveis: coleção → grupos → padrões → princípios → emendas)
  ▼
CONSTITUIÇÃO / PRINCIPLES.md
  (Conhecimento organizado como leis e guias)
```

---

## 11. INTEGRAÇÃO COM MEMORIZE-COMMIT

### 11.1 Relação entre os Workflows

```
memorize-commit.md                    knowledge-pipeline-f2-f3.md
═══════════════════                    ══════════════════════════

Pós-commit de QUALQUER mudança        Pipeline focado em learnings.md
Gera Impact Report completo (10       Valida, commita e revisa aprendimentos
  steps)                                específicos
Registra na Engineering Timeline      Usa git como mecanismo de versionamento
Extrai aprendizados do diff           Gera commits com prefixo "learn:"
Foco: métricas de engenharia          Foco: qualidade e rastreabilidade do
                                         conhecimento

Como se complementam:
  memorize-commit → registra TODO commit na timeline
  knowledge-pipeline-f2-f3 → COMMITA especificamente learnings
  Ambos → alimentam a base de conhecimento
```

### 11.2 Pontos de Integração

| Ponto | memorize-commit | F2-F3 |
|-------|-----------------|-------|
| **Trigger** | Pós-commit (qualquer) | Pós-F1, fsnotify, cron |
| **Arquivos afetados** | Qualquer arquivo no repo | Apenas learnings.md e failures.md |
| **Mensagem** | Mantém mensagem original | Prefixo `learn:` obrigatório |
| **Timeline** | Registra entrada completa | Registra entrada resumida |
| **Review** | Não faz (é pós-commit) | Faz (F3.2) antes do push |
| **Impact Report** | Gera completo | Gera resumido |

### 11.3 Fluxo Combinado

```
1. Learning entry é editado em memory/agent/*/learnings.md
2. F1 detecta (file watcher)
3. F2.1/F2.2 faz Source/Parse
4. F2.3 valida (4 gates)
5. F2.4 hot reload
6. F3.1 auto-commit → commit com prefixo "learn:"
7. F3.2 review (se habilitado)
8. F3.3 merge/push
9. Hook post-commit detecta o commit "learn:"
10. memorize-commit gera Impact Report
11. Engineering Timeline atualizada
```

---

## 12. INTEGRAÇÃO COM CONSTITUIÇÃO

### 12.1 Pipeline F2.3 Gate 4 Referencia CONSTITUIÇÃO

O Gate 4 do F2.3 verifica se o learning contradiz a CONSTITUIÇÃO. Esta é a
primeira barreira constitucional no pipeline de conhecimento — learnings que
violam P1-P8 são barrados antes de entrar no sistema.

### 12.2 Alinhamento com P3 (Rastro)

```
P3 — NENHUM AGENTE AGE SEM RASTRO

O pipeline F2-F3 implementa P3 através de:

├── F2.3: Toda validação é registrada (aprovado/rejeitado com causa)
├── F2.4: Todo evento de hot reload é logado
├── F3.1: Todo commit tem mensagem padronizada + autor = pipeline
├── F3.2: Toda review é documentada (score, causas de rejeição)
└── F3.3: Todo push é registrado na timeline
```

### 12.3 Alinhamento com P5 (Aprender com Erros)

```
P5 — A FAMÍLIA APRENDE COM ERROS

O pipeline F2-F3 implementa P5 através de:

├── F2.3: Learnings rejeitados são registrados com causa
│   (não descartados silenciosamente)
├── F3.2: Commits rejeitados são desfeitos com justificativa
│   (o learning não é perdido — volta para edição)
└── Métricas: Taxa de rejeição é monitorada
    (se > 30%, algo está errado no processo de criação de learnings)
```

### 12.4 Alinhamento com P7 (Memória sem Poluição)

```
P7 — MEMÓRIA SEM POLUIÇÃO

O pipeline F2-F3 implementa P7 através de:

├── F2.3 Gate 2: Tags consistentes evitam poluição semântica
├── F2.3 Gate 3: Domínios conhecidos evitam categorização errada
├── F2.3 Gate 4: Learnings que contradizem CONSTITUIÇÃO são barrados
└── F3.2: Review verifica se learning já existe (evita duplicatas)
```

---

## 13. CLI E AUTOMAÇÃO

### 13.1 Comandos

```bash
# Executar pipeline completo F2-F3
cosca knowledge pipeline --f2-f3

# Executar apenas F2 (validate + hot reload, sem commit)
cosca knowledge pipeline --f2

# Executar apenas F2.3 (validate, sem hot reload)
cosca knowledge pipeline --validate

# Executar apenas F3 (auto-commit + review, assumindo F2 já feito)
cosca knowledge pipeline --f3

# Executar em modo dry-run (não modifica arquivos, não commita)
cosca knowledge pipeline --f2-f3 --dry-run

# Executar sem review (Don override)
cosca knowledge pipeline --f2-f3 --no-review

# Ver relatório do último pipeline
cosca knowledge pipeline report

# Ver histórico de pipelines
cosca knowledge pipeline history --days 30

# Ver learnings rejeitados (pendentes de correção)
cosca knowledge pipeline rejected

# Ver health do pipeline (métricas, erros, warnings)
cosca knowledge pipeline health
```

### 13.2 Configuração

```yaml
# internal/embed/cosca/cosca.config.yaml ou opencode.json

knowledge_pipeline:
  # F2 — Validate
  validate:
    gates:
      integrity: true      # Gate 1
      tags: true           # Gate 2
      domain: true         # Gate 3
      constitution: true   # Gate 4
    max_rejection_rate: 0.3  # Se > 30% rejeitado, alerta

  # F2 — Hot Reload
  hot_reload:
    enabled: true
    notify_agents:
      - cosca-semantic-memory
      - cosca-memory-chief
      - cosca-evolution
      - cosca-experience-compiler
      - cosca-wisdom-distillation
      - cosca-review
      - cosca-kernel
      - cosca-discovery
      - cosca-cli
    event_ttl_seconds: 300

  # F3 — Auto-Commit
  auto_commit:
    enabled: true
    batch_window_seconds: 5      # Janela para agrupar mudanças
    max_entries_per_commit: 50   # Máximo de entries por commit
    branch_prefix: "learn/"      # Prefixo para branches de learning
    create_pr: true              # Criar PR (vs merge direto)
    auto_merge: false            # Merge automático para main
    skip_if_empty_diff: true     # Não commitar se diff vazio

  # F3 — Review
  review:
    enabled: true                # Don pode desligar
    min_score_for_approval: 70   # Score mínimo para aprovar
    reject_below: 50             # Score abaixo disso rejeita
    auto_revert_on_reject: true  # Desfaz commit se rejeitar
```

### 13.3 Automação (Cron)

```cron
# A cada 5 minutos — verifica mudanças em learnings.md, valida, hot reload
*/5 * * * * cosca knowledge pipeline --f2

# A cada hora — se houver learnings não commitados, commita
0 * * * * cosca knowledge pipeline --f3

# Diariamente — relatório de saúde do pipeline
0 6 * * * cosca knowledge pipeline health
```

---

## 14. MÉTRICAS DO PIPELINE

### 14.1 Métricas Primárias

| Métrica | Descrição | Alvo | Ferramenta |
|---------|-----------|------|------------|
| **F2.3 Tempo de validação** | Tempo médio por learning | < 1s | Log de performance |
| **F2.4 Tempo de hot reload** | Tempo para notificar todos agents | < 2s | Log de performance |
| **F3.1 Tempo de auto-commit** | Tempo do git add + commit | < 3s | Git hook timing |
| **F3.2 Tempo de review** | Tempo para validar o commit | < 5s | Log de performance |
| **Pipeline total** | F2 + F3 completo | < 15s | Log de performance |
| **Taxa de rejeição F2.3** | % de learnings rejeitados | < 20% | Dashboard |
| **Taxa de rejeição F3.2** | % de commits rejeitados | < 10% | Dashboard |
| **Commits vazios evitados** | Nº de vezes que F3.1 skippou | > 90% das tentativas | Dashboard |
| **Agentes notificados** | Nº médio de agents no hot reload | 9/9 | Log de eventos |

### 14.2 Métricas de Qualidade

| Métrica | Descrição | Alvo |
|---------|-----------|------|
| **Learnings válidos/total** | Proporção de entries que passam F2.3 | > 80% |
| **Commits aprovados/total** | Proporção de commits aprovados no F3.2 | > 90% |
| **Tempo médio F1→F3.3** | Desde detecção até push | < 30s |
| **Cobertura de validação** | % de entries que passam pelos 4 gates | 100% |
| **Falsos positivos F2.3** | Entries válidos rejeitados incorretamente | < 1% |
| **Falsos negativos F2.3** | Entries inválidos aprovados incorretamente | < 0.1% |

### 14.3 Dashboard

```
┌─────────────────────────────────────────────────────────────────────┐
│                    KNOWLEDGE PIPELINE HEALTH                          │
│                    Última atualização: {timestamp}                     │
├─────────────────────────────────────────────────────────────────────┤
│                                                                       │
│  F2.3 Validate                                                        │
│  ┌──────────────────────────────────────────────────────────────┐    │
│  │ Total: 245 entries │ ✅ 201 (82%) │ ❌ 44 (18%) │ ⚠️ 12 (5%) │    │
│  │ Gate 1: 98% ✅ │ Gate 2: 95% ✅ │ Gate 3: 92% ✅ │ Gate 4: 99% ✅ │
│  │ Tempo médio: 0.4s/entry │ P99: 0.9s                          │    │
│  └──────────────────────────────────────────────────────────────┘    │
│                                                                       │
│  F2.4 Hot Reload                                                     │
│  ┌──────────────────────────────────────────────────────────────┐    │
│  │ Eventos: 89 │ ✅ 87 (98%) │ ❌ 2 (2%)                         │    │
│  │ Agents notificados: 9/9 │ Tempo médio: 0.3s                  │    │
│  │ Caches invalidados: 7/7                                       │    │
│  └──────────────────────────────────────────────────────────────┘    │
│                                                                       │
│  F3.1 Auto-Commit                                                    │
│  ┌──────────────────────────────────────────────────────────────┐    │
│  │ Commits: 67 │ ✅ 65 (97%) │ ❌ 2 (3%) │ SKIP: 34 (34%)       │    │
│  │ Tempo médio: 1.2s │ P99: 2.8s                                │    │
│  │ Branch criada: 12 │ Commit direto: 55                         │    │
│  └──────────────────────────────────────────────────────────────┘    │
│                                                                       │
│  F3.2 Review                                                         │
│  ┌──────────────────────────────────────────────────────────────┐    │
│  │ Reviews: 67 │ ✅ 63 (94%) │ ❌ 4 (6%) │ SKIP (Don): 0        │    │
│  │ Score médio: 87/100 │ Tempo médio: 2.1s                      │    │
│  └──────────────────────────────────────────────────────────────┘    │
│                                                                       │
│  F3.3 Merge/Push                                                     │
│  ┌──────────────────────────────────────────────────────────────┐    │
│  │ Push: 67 │ PR criado: 12 │ Merge direto: 55                   │    │
│  │ Erros: 0 │ Tempo médio: 1.5s                                 │    │
│  └──────────────────────────────────────────────────────────────┘    │
│                                                                       │
│  Pipeline Total: 67 execuções │ Tempo médio: 6.8s │ P99: 14.2s      │
│  Learnings processados: 245 │ Rejeitados (F2.3+F3.2): 48 (19.6%) │
│                                                                       │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 15. CASOS DE BORDA E ANTI-PADRÕES

### 15.1 Casos de Borda

| Cenário | Problema | Solução |
|---------|----------|---------|
| **Learning editado e depois revertido** | Diff existe mas conteúdo final é igual ao original | F3.1 detecta diff vazio → skip |
| **Múltiplos agents editam o mesmo arquivo simultaneamente** | Conflito de merge no git | F3.1 aborta se houver conflito → notifica agents |
| **Learning com 200+ caracteres na mensagem** | Violação de formato | F2.3 Gate 1 rejeita → learning volta para edição |
| **Tag #golang ao invés de #go** | Tag não padronizada | F2.3 Gate 2: warning (não bloqueante), sugere #go |
| **Learning que sugere violação de P1** | Contradição constitucional | F2.3 Gate 4 rejeita com justificativa |
| **Fsnotify perde evento** | Mudança não detectada | Cron 5min como fallback |
| **Git push falha por rede** | F3.3 não completa | Retry 3× com backoff exponencial (1s, 3s, 9s) |
| **Review rejeita commit** | Learning não é versionado | git reset HEAD~1, learning volta para pool |
| **Knowledge.db corrompido** | F2.4 não consegue atualizar | Fallback para notificação manual, alerta Don |
| **Don faz override manual no learnings.md** | Mudança fora do pipeline | F1 detecta, F2-F3 processa normalmente |

### 15.2 Anti-Padrões

| Anti-Padrão | Por que é problema | O que fazer em vez |
|--------------|-------------------|-------------------|
| **Validar depois de commitar** | Conhecimento poluído já versionado | Validar ANTES de commitar (F2.3 antes de F3.1) |
| **Hot reload sem validação** | Agents recebem conhecimento inválido | Sempre validar antes de notificar |
| **Commit sem review** | Conhecimento não revisado vira verdade | Review habilitado por padrão (Don pode desligar) |
| **Review bloqueante para mudanças urgentes** | Conhecimento crítico preso no pipeline | Don override (`--no-review`) para emergências |
| **Notificar agents em sequência** | Lento (soma latência de cada agent) | Notificar em paralelo (goroutines) |
| **Validar tudo de novo no F9.1** | Duplicação de trabalho | F9.1 confia no F2.3 (não re-valida formato) |
| **Commit por entry individual** | Poluição do git history com micro-commits | Batch de 5s + máximo 50 entries por commit |
| **Ignorar learnings rejeitados** | Perda de conhecimento valioso | Rejeitados voltam para pool com justificativa |

### 15.3 Estratégia de Erro e Recuperação

| Falha | Impacto | Ação |
|-------|---------|------|
| F2.3 falha em 1 entry | Entry não passa | Continua com os demais, rejeitado logado |
| F2.3 falha em > 50% da batch | Batch inteira suspeita | Aborta pipeline, alerta Don |
| F2.4 evento não publicado | Agents não notificados | Retry 3×, se persistir → alerta |
| F3.1 git commit falha | Learning não versionado | Log do erro, learning retorna para F1 |
| F3.2 review rejeita | Commit desfeito | git reset, learning volta para pool |
| F3.3 git push falha | Learning local mas não remoto | Retry, se falhar → alerta (não perde dados) |
| Pipeline inteiro falha | Nada processado | Log completo, Don notificado |

---

## HISTÓRICO

| Versão | Data | Autor | Alterações |
|--------|------|-------|------------|
| 1.0.0 | 2026-07-30 | Architecture Chief | Design inicial do pipeline F2-F3 completo: validação (4 gates), hot reload, auto-commit, review, integrações com F9.1, F9.2, memorize-commit, CONSTITUIÇÃO |
