# CAMPAIGN_PROTOCOL — o método científico da casa (29º protocolo, v3.0)

> **Lema:** "Não procure uma confirmação. Procure uma resposta que sobreviva à
> tentativa de refutá-la."
>
> **Princípio:** Uma campanha é uma investigação estruturada destinada a reduzir
> uma incerteza técnica por meio de evidência reproduzível, preservando
> histórico, erratas, refutações, decisões e estado válido.

## 1. Propósito

Uma CAMPAIGN existe quando uma pergunta não pode ser respondida adequadamente
por uma única operação.

```
TASK: "melhorar performance da busca"  →  CAMPAIGN: "identificar o custo
dominante e quais otimizações preservam qualidade e invariantes"
→ EXPERIMENTS: E1 → E2 → H1 → H2 → H3
```

A campanha **não é uma sequência linear de comandos — é uma árvore de
investigação.**

## 2. Hierarquia da casa

```
               COSCA GOVERNANCE
                      │
        ┌─────────────┴─────────────┐
        │                           │
 CAMPAIGN_PROTOCOL            RECOVERY_PROTOCOL
        │                           │
  ┌─────┴─────┐                     │
  │           │                     │
EXPERIMENT  DECISION            RESTORATION
PROTOCOL    PROTOCOL            VERIFICATION
  │           │
  └─────┬─────┘
        │
    PROVENANCE
        │
     MEMORY
```

A campanha **coordena**. O experimento **mede**. A decisão **autoriza ou
rejeita**. O recovery **restaura**. A provenance **prova o histórico**. A
memória **preserva o aprendizado**.

## 3. Invariante central

A campanha nunca pode afirmar mais do que a evidência suporta:

```
STRENGTH(CONCLUSION) ≤ STRENGTH(EVIDENCE)
```

Se não satisfaz: UNKNOWN → INCONCLUSIVE → STOP. **Nunca:** UNKNOWN → GUESS →
CONFIRMED.

## 4. Estados da campanha

```
PROPOSED → SCOPED → BASELINED → INVESTIGATING → EVIDENCE_REVIEW
   → CONFIRMED | REFUTED | INCONCLUSIVE | BLOCKED | SUPERSEDED
   → DECIDED → AUTHORIZATION_REQUIRED → TRANSFORMATION
   → POST_VERIFICATION → CLOSED
```

**Nenhum estado pode ser pulado sem justificativa registrada.**

## 5. Abertura da campanha

Antes de investigar:

```
CAMPAIGN_ID · TITLE · QUESTION · OBJECTIVE · OWNER · DATE · REPOSITORY ·
COMMIT · ENVIRONMENT · SCOPE · NON_GOALS · BASELINE · INVARIANTS ·
SUCCESS_CRITERIA · STOP_CONDITIONS · MODE
```

Exemplo (a campanha viva):
```yaml
campaign:
  id: PERF-2026-001
  title: Search Performance Investigation
  question: "Onde está o trabalho redundante no caminho de busca?"
  mode: READ_ONLY
  transformation_allowed: false
  baseline: L387
  invariants: [recall, ranking, provider_identity, digest, provenance,
               determinism, recovery, memory_integrity]
```

## 6. Scope lock

A campanha declara o que **não** está investigando. Se surgir algo fora do
escopo: DISCOVERY → REGISTER → NEW HYPOTHESIS → NEW EXPERIMENT. **Nunca**
alterar o escopo silenciosamente.

## 7. Baseline

Reproduzível: git commit · model · provider · digest · hardware · OS ·
configuration · corpus · dataset hash · database state · compiler · runtime ·
benchmark methodology · metrics.

**Sem baseline, não existe comparação.**

## 8. Hipóteses

Cada hipótese é independente e **não contém a própria conclusão**:

```
H1: CLAIM · OBSERVED · RATIONALE · PREDICTION · TEST · METRIC · PASS · FAIL · STATUS
```

## 9. Experimentos

```
EXPERIMENT_ID · QUESTION · HYPOTHESIS · CONTROL · VARIABLE · ENVIRONMENT ·
DATASET · METHOD · METRICS · EXPECTED_RESULT · STOP_CONDITIONS
```

Uma pergunta → uma medição → uma decisão.

## 10. Read-only first

Por padrão: CAMPAIGN → READ_ONLY. Transformação exige transição explícita:
EVIDENCE → DECISION → AUTHORIZATION → TRANSFORMATION. Nenhuma campanha ganha
permissão de alteração simplesmente porque encontrou algo interessante.

## 11. Ambiente de experimento

Toda evidência registra o ambiente (provider, model, digest, config, hardware,
dataset, runtime, flags, env vars). **Se o ambiente experimental não for
semanticamente equivalente ao de produção relevante, a evidência é marcada
LIMITED ou INVALID** conforme o impacto (a lição L368/L389 — o provider
vetorial desligado).

## 12. Resultados

Separar rigorosamente:

```
OBSERVED (0.26ms) · MEASURED (recall 81%) · INFERRED (bounded reduz latência)
· EVIDENCE (recall < threshold) · DECISION (REJECT)
```

## 13. Errata

Erro experimental **não pode ser apagado**:

```
ERRATA_ID · ORIGINAL_EXPERIMENT · ORIGINAL_RESULT · ERROR · CAUSE · IMPACT ·
VALIDITY (INVALID/PARTIAL) · CORRECTIVE_ACTION · RE_RUN · SUPERSEDES
```

(L388 → provider não registrado → INVALID → L389 re-medição. A errata vira
parte da evidência.)

## 14. Refutação

Uma hipótese refutada registra: WHY · EVIDENCE · CONDITIONS · LIMITATIONS ·
REMAINING_QUESTION — e **nunca é apagada** (conhecimento negativo).

## 15. Descobertas colaterais

```
DISCOVERY_ID · SOURCE_EXPERIMENT · OBSERVATION · IMPACT · NEW_QUESTION
→ NEW HYPOTHESIS
```

Assim nasce a árvore (ex.: a descoberta de que o provider estava desligado
gerou a re-medição; o FTS rebuild dominante gerou E-006).

## 16. Árvore de investigação

```
CAMPAIGN-001
 ├── E001 Baseline
 ├── H1 ── E002 ── REFUTED
 ├── H2 ── E003 ── DISCARDED
 ├── H3 ── E004 ── CONFIRMED
 └── DISCOVERY-001 ── H4 ── E005 ── INCONCLUSIVE
```

Melhor que lista cronológica.

## 17. P13 dentro da campanha

Toda conclusão passa pela régua (medição válida? ambiente correto? controle?
provider esperado? digest esperado? mistura? o teste mede a hipótese?
confounder? amostra suficiente?). **Se alguma resposta crítica for UNKNOWN →
INCONCLUSIVE.**

## 18. Invariantes

Antes e depois de todo experimento relevante: memory integrity · provenance ·
provider identity · model digest · recall · ranking · determinism · recovery ·
security · data integrity.

**Uma otimização que quebra uma invariante não é otimização — é regressão.**

## 19. Decisão

A campanha não termina no experimento:

```
DECISION · SUPPORTED_BY (E001, E002…) · OPTIONS · SELECTED · RATIONALE ·
RISKS · UNKNOWN · AUTHORIZATION_REQUIRED
```

Separa "descobrimos" de "decidimos fazer".

## 20. Transformação

Somente após autorização:

```
PRE_STATE → SNAPSHOT → CHANGE → TEST → VERIFY → POST_STATE
commit_before · commit_after · files_changed · tests ·
metrics_before · metrics_after · invariants
```

## 21. Recovery

Se a transformação sair do controle:

```
ANOMALY → CAMPAIGN PAUSE → RECOVERY_PROTOCOL → IDENTIFY LAST VALID STATE →
VERIFY IDENTITY → RESTORE → PROVE → RESUME / ABORT
```

**Nunca** `rollback HEAD~1` automático — porque
LAST COMMIT ≠ LAST VALID COMMIT ≠ LAST KNOWN-GOOD STATE.

## 22. Interrupção e retomada

Estados: PAUSED · BLOCKED · RECOVERY · WAITING_AUTHORIZATION. A retomada
carrega o estado do REGISTRO (campaign state, hypotheses, evidence, errata,
decisions, open questions, environment) — **a compactação de contexto não
destrói a campanha.**

## 23. Conclusão

```
QUESTION · ANSWER · CONFIDENCE · EVIDENCE · LIMITATIONS ·
REFUTED_HYPOTHESES · CONFIRMED_HYPOTHESES · OPEN_QUESTIONS · DECISIONS ·
TRANSFORMATIONS · RECOVERY_EVENTS · PROVENANCE
```

Uma campanha não é obrigada a produzir uma solução — é obrigada a produzir
clareza sobre o que foi descoberto.

## 24. Fechamento

Antes de CLOSED: pergunta respondida · baseline preservado · evidências
registradas · hipóteses classificadas · erratas registradas · invariantes
verificadas · transformações verificadas · recovery verificado (se usado) ·
provenance válida · limitações documentadas · decisões registradas · memória
atualizada.

```
CAMPAIGN → CLOSED → MEMORY → KNOWLEDGE
```

## 25. A regra mais importante

> Uma campanha não existe para provar que o Cosca estava certo. Ela existe para
> descobrir o que é verdadeiro dentro do escopo, das condições e da evidência
> disponível. Refutar uma hipótese é progresso. Corrigir uma medição é
> progresso. Descobrir que não sabemos é progresso. O único fracasso real é
> transformar uma hipótese em fato sem evidência suficiente.

## Referências

- Baseline congelado: `knowledge/patterns/PERFORMANCE_BASELINE_L387_L390.md`
- Campanha viva: `knowledge/patterns/CAMPAIGN_PERF-2026-001.md`
- Histórico: L387-L394 (a campanha demonstrou o comportamento antes do
  protocolo existir — o protocolo institucionaliza o emergente).
