# KNOWLEDGE PROTOCOL — Como buscar conhecimento na família

> **Versão**: 1.0.0 | **Status**: canônico | **Dono**: cosca-kernel
> **Criado**: 2026-08-16, por ordem do Don — consolidar como BUSCAR conhecimento
> **Propósito**: referência operacional ÚNICA para achar o conhecimento certo na
> fonte certa, sem varrer tudo. Complementa o `MEMORY_ACCESS_PROTOCOL.md`.

---

## 1. As FONTES — onde o conhecimento vive

| Fonte | Onde | O que tem |
|-------|------|-----------|
| **Knowledge base (local)** | `.cosca/knowledge.db` | docs, código, ADRs, patterns (FTS5 + vetores) |
| **Knowledge base (global)** | `~/.config/cosca/knowledge.db` | conhecimento compartilhado entre projetos |
| **Sessões** | `.cosca/session.db` | conversas passadas (FTS5, zero LLM) |
| **Símbolos de código** | `knowledge.db` (code_symbols) | funções/tipos Go pelo que FAZEM |
| **Grafo** | knowledge graph | relações entre entidades |
| **Memória** | `.cosca/memory/` | short/long/project/decision |
| **Learnings** | `learnings.md` + `blocks/` | a memória do kernel (gatilhos) |

---

## 2. Os COMANDOS — como buscar cada fonte

```bash
# Conhecimento (FTS5 + vetores, local + global)
cosca knowledge search "database schema"
cosca knowledge search --limit 20 "error handling"
cosca knowledge search --global "cobra"        # só a base global
cosca knowledge search --json "architecture"

# Por que aquele resultado? (explicabilidade)
cosca knowledge explain <result-id>

# Lacuna de conhecimento (o que eu AINDA não sei)
cosca knowledge resolve <tarefa>

# Sessões passadas (FTS5 determinística, zero LLM)
cosca session index                             # reindexa primeiro
cosca session search "CKL"
cosca session search --limit 20 "tatuagem"

# Símbolos de código (achar função pelo que FAZ)
cosca symbols index                             # indexa primeiro
cosca symbols search "o que calcula PnL"

# Ranking multi-sinal (por que X venceu)
cosca ranking explain "backup"
cosca ranking explain "backup" --index 1

# Memória
cosca memory search "database schema"
cosca memory list

# Grafo
cosca graph
```

---

## 3. A ÁRVORE DE DECISÃO — o que usar para o quê

| Pergunta | Use |
|----------|-----|
| "Onde está documentado X?" | `cosca knowledge search "X"` |
| "Já conversamos sobre X?" | `cosca session search "X"` |
| "Que função faz Y?" | `cosca symbols search "Y"` |
| "Por que esse resultado apareceu?" | `cosca knowledge explain <id>` / `cosca ranking explain` |
| "O que eu ainda não sei sobre X?" | `cosca knowledge resolve X` |
| "O que a família aprendeu sobre X?" | `learnings.md` (grep por tag) → `blocks/` |
| "Como X se relaciona com Y?" | `cosca graph` |

---

## 4. EXPLICABILIDADE — nunca confiar no "porque sim"

O ranking do Cosca é **multi-sinal com pesos fixos** (BM25, vetor, grafo,
frescor, popularidade) — não uma nota opaca. Cada resultado pode ser auditado:

```bash
cosca ranking explain "<query>"        # breakdown: quanto cada sinal contribuiu
```

**Regra (P13)**: antes de agir com base num resultado, saiba POR QUE ele
venceu. "Apareceu no topo" não é evidência.

---

## 5. A ORDEM — buscar ANTES de agir

O ciclo de metacognição manda: **RETRIEVE MEMORY antes de PLAN e EXECUTE**.

```
1. BUSCAR   → knowledge search + session search + learnings (tags)
2. VERIFICAR → explain/ranking (por que esse resultado?)
3. RESOLVER  → se lacuna, knowledge resolve (o que falta?)
4. AGIR      → só depois de achar + entender
```

Nunca implementar antes de buscar — a família já pode ter resolvido isso
(L256: "não ficar escaniando tudo até entender").

---

## 6. GOTCHAS

1. **Índice desatualizado** — `cosca session search` e `cosca symbols search`
   exigem `index` antes (a indexação é explícita, não automática).
2. **Base local vs global** — sem `--global`, a busca é só local + grep nos docs
   globais adquiridos. Quer tudo? Use `--global`.
3. **Vetor órfão** — se o search devolve resultado morto, `cosca knowledge verify`
   acha o mismatch (L254).
4. **Conteúdo do learnings vive no block** — o `learnings.md` só tem o gatilho;
   o detalhe está em `blocks/<hash>.md` (MEMORY_ACCESS_PROTOCOL §2).
5. **Explicabilidade é parte da busca** — não aceite "porque sim" (P13).

---

## 7. HISTÓRICO

| Versão | Data | Mudança |
|--------|------|---------|
| 1.0.0 | 2026-08-16 | Criado por ordem do Don — consolida como buscar conhecimento |
