# A MINA — Cosca Autonomous Discovery Engine

> **Version**: 1.0.0 | **Status**: spec | **Owner**: Cosca Kernel | **Created**: 2026-08-13
>
> A "coisa muito top": um loop fechado onde o Cosca **recebe um problema, explora com gradiente, evolui a própria estratégia e devolve evidência VERIFICADA de uma solução que o Don não tinha previsto**.

## 1. Visão (uma frase)

> Transformar problema em **ouro verificado**: solução + evidência + integridade + prova de que foi DESCOBERTO (não reproduzido) + prova de que o sistema FICOU mais inteligente no processo.

## 2. O núcleo: DOIS loops aninhados

Não é um loop — são dois, e essa é a arquitetura inteira:

```
LOOP EXTERNO — SELF-EVOLUTION (evolui a ESTRATÉGIA)
┌───────────────────────────────────────────────────────────┐
│  LOOP INTERNO — DESCOBERTA (explora o ESPAÇO)             │
│                                                           │
│  ESTRATÉGIA ─► LLM propõe candidatos                      │
│      ▲            │                                       │
│      │            ▼                                       │
│      │        ORÁCULO (gradiente contínuo)                │
│      │            │                                       │
│      └── refina ◄┘  (hipóteses + evidência)               │
│                                                           │
│  ────────────────────────────────────────────             │
│  SOLUÇÃO CANDIDATA ─► VERIFICAÇÃO (refutação + secreto)   │
│         │                                                 │
│         ▼                                                 │
│  EVIDÊNCIA (family chain + ledger — tamper-evident)       │
│         │                                                 │
│         ▼                                                 │
│  SELF-EVOLUTION:  oráculo = fitness                        │
│      → GEPA evolui a estratégia (mutação reflexiva)       │
│      → benchmark-as-gate (NÃO regrediu o resto)           │
│      → ablação (a metacognição agregou?)                  │
│         │                                                 │
│         └──► volta pro loop interno com estratégia melhor  │
└───────────────────────────────────────────────────────────┘
```

**O loop interno encontra UMA solução. O loop externo encontra UMA ESTRATÉGIA melhor.** É a diferença entre "resolveu" e "ficou mais inteligente".

## 3. As 5 peças (tudo já existe — falta só o wiring)

| # | Peça | Componente existente | Papel |
|---|------|---------------------|-------|
| 1 | **Oráculo** | `internal/evals` (SubmitToOracle) + Neural Link `FunctionOracle` | gradiente contínuo → o agente DESCOBRE |
| 2 | **Self-evolution** | `cmd/evolve` (GEPA-equiv) + padrões L244 | fitness → evolui a estratégia |
| 3 | **Ablação** | `internal/evals/ablation.go` + LLMSolver | mede se a memória/metacognição agrega |
| 4 | **Integridade** | `internal/integrity` (family chain) + `internal/ledger` | evidência tamper-evident + MVCC |
| 5 | **Esteira** | `internal/pipeline` (workqueue/taskqueue) | execução paralela com dedup/backoff/recovery |

## 4. O fluxo de uma execução

```
DON: "resolva X, com essas restrições"
  │
  ▼
① ESTRATÉGIA (prompt/skill evolúvel — a "hipótese de como resolver")
  │
  ▼
② DESCOBERTA — LLM propõe → oráculo dá gradiente → refina
  │   (esteira: workqueue com dedup + rate-limit + recovery)
  ▼
③ VERIFICAÇÃO — self-refutation (contraexemplos) + oráculo SECRETO
  │   + F4: solução é KNOWN ou NOVEL?
  ▼
④ EVIDÊNCIA — family chain + ledger registram TUDO (append-only, hash chain)
  │
  ▼
⑤ SELF-EVOLUTION — mede (resolveu? eficiência? discarded?) →
  │   GEPA evolui a estratégia → benchmark-as-gate → ablação
  ▼
  LOOP ② (estratégia melhor) até convergir ou o Don parar
```

## 5. O que a MINA entrega (o dossiê, não uma resposta)

```
Solução:           f(x)=x²  (ou o que descobriu)
Solution class:    NOVEL    (≠ qualquer referência conhecida)
Evidência:         cadeia de proveniência (hipóteses→testes→refutações)
Integridade:       assinada na family chain (tamper-evident)
Descoberta:        discovery_efficiency = X (recursos gastos)
Metacognição:      a memória mudou o caminho? (ablação alone-vs-managed)
Estratégia:        evoluiu N× (score A → B)
Reproduzível:      mesmo problema N× → mesma solução? (repro)
```

## 6. O que é NOVO (o wiring) vs o que já tenho

**✅ 90% pronto** (oráculo, self-evolution, ablação, chain, esteira — tudo testado).

**🆕 O que falta construir (o wiring da MINA):**

| Fase | O que é | Depende de |
|------|---------|-----------|
| **M1** | O **loop fechado**: um orquestrador que conecta descoberta→verificação→evidência→self-evolution num ciclo autônomo | tudo |
| **M2** | **Benchmark-as-gate** (anti-overfit): a variante melhora a task-alvo SEM regredir o resto | L244 (padrão Hermes) |
| **M3** | **Gradiente contínuo** no oráculo (a lição do L243: binário não converge) | L243 |
| **M4** | **F4 — solution_class** (known/novel via hash do reference_solution) | L236 (schema já tem o campo) |
| **M5** | O **dossiê** consolidado (integração com family chain + ledger + metrics) | L241 (ledger) |

## 7. Regras de ouro (herdadas da sessão inteira)

1. **Oráculo com gradiente CONTÍNUO desde o dia zero** — nunca binário (L241/L242/L243).
2. **Benchmarks são GATES, não fitness** — anti-overfit (L244).
3. **"Operates ON, not inside"** — a evolução é desacoplada, abre PRs, não toca no kernel (L244).
4. **Integridade primeiro** — toda evidência passa pela family chain (L186/L188).
5. **Descoberta é relativa** — nunca "ninguém no mundo", só "não estava nas fontes acessíveis" (L199).

## 8. Critério de sucesso (seção 26-style)

A MINA está pronta quando, **num problema que o Don também não sabe resolver**, ela:
1. descobre uma solução que passa num oráculo independente;
2. classifica como NOVEL (não estava nas fontes);
3. prova que EVOLUIU (estratégia melhorou entre rodadas);
4. entrega evidência auditável (chain íntegra);
5. e o Don consegue **reproduzir** o resultado por um caminho independente.
