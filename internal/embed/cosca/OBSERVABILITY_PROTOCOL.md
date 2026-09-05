# OBSERVABILITY PROTOCOL — Enxergar por dentro

> **Versão**: 1.0.0 | **Status**: canônico | **Dono**: cosca-kernel
> **Criado**: 2026-08-16, por ordem do Don — o porquê, não só o quê
> **Propósito**: referência operacional ÚNICA para medir, traçar e correlacionar.
> Sem observabilidade, nenhum outro protocolo é verificável de verdade.

---

## 1. O TRACE — a história de cada operação (flight recorder)

Toda operação importante ganha um **Trace ID universal** (`TRACE-20260802-7F92`)
e cada evento carrega o contexto para reconstruir a história: `trace_id`,
`parent_event`, `actor`, `action`, `input_hash`, `output_hash`, `code_version`,
`knowledge_version`, `cognitive_version`, `result`.

```bash
cosca trace new                                    # novo Trace ID
cosca trace event <id> --action X --actor Y ...    # registra um evento
```

- **Append-only** (`.cosca/trace.db`) — nunca edita nem apaga.
- **Divergência** → "SUSPICIOUS DIVERGENCE" onde esta execução difere das anteriores.

---

## 2. AS MÉTRICAS — o pulso do runtime

```bash
cosca metrics    # requests, LLM calls, router decisions, MAG, timing por estágio
```

O que medir:
- **LLM** — calls, tokens, erros, fallbacks (quantas vezes caiu pro determinístico?).
- **Router** — a decisão de roteamento (foi pro lugar certo?).
- **Timing** — onde o tempo está indo (qual estágio é o gargalo?).

---

## 3. O SNAPSHOT COGNITIVO — congelar o estado (CV)

```bash
cosca cv snapshot          # congela o estado cognitivo em hash SHA-256
cosca cv list              # lista os snapshots
cosca cv verify [CV-XXXX]  # verifica o estado atual contra um CV
```

- Congela documentação, conhecimento, leis, memória, prompts, schemas, config.
- **É hash, não conteúdo** — verificação, não backup.
- Rollback é diagnóstico + guia (via git), nunca reescrita automática.

---

## 4. A CORRELAÇÃO — causa, não só correlação

O observability protocol não pergunta só "o que aconteceu?" — pergunta **"por quê?"**:

```
evento → trace (história) → metrics (pulso) → cv (estado) → causa
```

Para incidente: reconstrua a cadeia de eventos (trace), veja o estado no
momento (cv), ache o ponto de divergência. É o braço do INCIDENT_RESPONSE.

---

## 5. GOTCHAS

1. **Trace sem input_hash/output_hash é diário, não evidência** — hash prova o quê.
2. **Append-only é sagrado** — nunca reescrever o passado (L156: read-back).
3. **Métrica sem baseline é número solto** — compare com o anterior.
4. **Observar não é agir** — o trace registra; a correção é outro protocolo.

---

## 6. HISTÓRICO

| Versão | Data | Mudança |
|--------|------|---------|
| 1.0.0 | 2026-08-16 | Criado por ordem do Don — enxergar por dentro |
