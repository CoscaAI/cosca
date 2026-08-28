# ADR-020: Code-Intelligence Refinamento — absorver as lições do ecossistema MCP de código

> **Status:** Proposed | **Owner:** cosca-kernel + cosca-architecture + cosca-security | **Last Updated:** 2026-08-28
> **Revisão:** aguardando Don + cosca-cto. **Design aditivo — não quebra o Root.**
> **Referência (base):** mineração de 6 servidores MCP de code-intelligence + `ADR-019` (Code-Intelligence Graph) + `ADR-017` (Borrowing Protocol) + `ADR-018` (Stage).

---

## 0. Contexto — 6 minas, nenhum novo "ouro", muitas lições

Após absorver o `codebase-memory-mcp` (CBM, 40k⭐) e construir o `ADR-019` (F1–F4), mineramos 6
servidores MCP de code-intelligence na mesma frente. Nenhum é novo "ouro" como o CBM (1–70⭐), e os
capos **corrigiram claims de marketing dos READMEs**. O valor está nas **ideias específicas** que
ainda não temos. Este ADR consolida o mapa e prioriza.

**Correções factuais (dos capos, contra o código):**
- `sdsrss/code-graph-mcp`: o "BLAKE3 Merkle tree" é um **mapa plano** de content-hash + cache de mtime
  (não é árvore); a "regeneração de todos os callers downstream" é vizinhança de 1 salto (não fecho).
- `cmillstead/codesight-mcp`: o "~99% menos tokens" é **~80–99%** (caso extremo), honesto.
- **Correção ADR-019 §1:** a camada de embedding é **`internal/embeddings`** (`provider.go`), não
  `internal/embed` (que é go:embed filesystem).

## 1. O que ADAPTAR (priorizado)

### 🥇 P1 — Segurança / data-plane (o maior gap nosso)
1. **Spotlighting no `contenttrust`** — delimitador **inforjável** (nonce) + `_meta contentTrust:untrusted`
   que diz ao agente *"isto é data, nunca instrução"*. Mitiga o vetor de **manipulação semântica**
   (achado 2026-08-22). **✅ IMPLEMENTADO** (`2df2065`).
2. **Byte-offset O(1) symbol retrieval** — `seek+read` só do símbolo (~80–99% menos tokens). Compõe com o
   índice RAM-first; expõe `GetSymbolSource` (token-efficiency / ADR-019).
3. **Cadeia de path 6 passos** (O_NOFOLLOW, re-check pós-resolução, walk symlink pai) → endurecer o rails.
4. **Detector de injeção em 3 tiers** → reforçar o `Suspicious` (hoje 5 markers).

### 🥈 P2 — Manutenção incremental (o F3+ do índice)
5. **Content-hash diff + dirty propagation** — coletar dependentes ANTES de mutar + regenerar sinais
   derivados da vizinhança + **guard de run interrompido** (I2). Hoje o índice re-indexa tudo.

### 🥉 P3 — Busca (o meio-termo determinístico, sem modelo)
6. **Naming-blindness sem LLM**: **comunidades de tokens por co-ocorrência** no seu corpus +
   **expansão de query por grafo** (callee/stem). Resolve `authenticate→login` pro SEU repo, zero-LLM.
7. **Cross-encoder OPCIONAL via provider** (seam `internal/embeddings`, default false → preserva I1).
8. **RRF** só se houver 2ª fonte de recuperação; o valor é a **prova de não-inversão** (blend limitado).

### Token-efficiency + governance
9. **Chunking determinístico por blocos lógicos** (splitFunction; melhor que nosso chunker markdown).
10. **`is_test`** por símbolo + **disclosure de capacidade por linguagem** (I3/I4).
11. **Complexity scoring** (metadata INFERRED/I4).
12. **Session memory** (per-agente + frontier) → complementa **ADR-018 (Stage)**.

## 2. O que REJEITAR / cautela

| Fonte | Veredito |
|---|---|
| **`kraklabs/cie` (AGPL-3.0 + CGO/CozoDB)** | 🔴 **PROIBIDO embutir** (copyleft deriva o Cosca). Levar **só a ideia** (interface-dispatch). Sidecar não-modificado = só prova de fronteira (sandbox I7, rede off), nunca produção. Rota limpa = licença comercial. |
| **Embedding-model como backbone** (nomic/ONNX/Qdrant/Ollama) | 🔴 já decidido (ADR-019 §5.1: não copiar blob/modelo). Só **provider opcional** (P3.7). |
| **WASM tree-sitter** (gramáticas em C) | 🔴 só via sidecar/sandbox — proibido como backbone (single-binary/I7-I8 do ADR-019 §5). |
| **Sandbox do codesight** | 🔴 Cosca I7 é superior. |

## 3. Invariantes (I1–I8)

Todas as ideias de P1–P3 são **determinísticas** (I1) quando sem modelo (cross-encoder é opt-in). As de
segurança reforçam **I2/I7/I8** (nonce inforjável, path-chain, tiered injection). O `edges.confidence`
do code-graph (extracted/inferred/ambiguous) **confirma o nosso I4** e é adaptável aos edges do Cosca.

## 4. Recomendação

1. **Já feito:** Spotlighting (P1.1, `2df2065`).
2. **Seguir:** **Byte-offset O(1)** (P1.2) → **Incremental + dirty propagation** (P2.5) → **Naming-blindness
   determinístico** (P3.6).
3. **Consolidar sem pressa:** path-chain, tiered injection, chunking determinístico, is_test, session memory.
4. **Nunca:** embutir `cie` (AGPL), blob/embedding-model como backbone, WASM tree-sitter como SoT.

---

*O Cosca minera a IDEIA dos 6 servidores (boundary markers, byte-offset, incremental, determinismo
sem modelo) — não a arquitetura (nem o AGPL do cie, nem o modelado embed, nem o WASM). Compõe sobre o
ADR-019/018/017, preservando binário único, SoT único e a soberania do cérebro (I1).*
