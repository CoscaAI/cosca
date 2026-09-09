# ADR-030: Filesystem Cognitivo — Knowledge Graph + Content Addressable Memory + Materialized Runtime

> **Status:** Proposed (aguardando cosca-cto + cosca-architecture + Don) | **Owner:** cosca-architecture (Architecture Chief) | **Last Updated:** 2026-08-29
> **Revisão:** decisão de design de LONGO prazo — NÃO implementada de uma vez (Mandamento III). Define o **alvo**; a implementação é incremental em fases, cada uma com baseline de recall.
> **Fonte (ordem do Don + professor, 2026-08-29):** a visão de que a memória de conhecimento do COSCA pode virar um **filesystem cognitivo versionável** — não uma pasta de módulos + SQLite como cache, mas um **DAG de objetos imutáveis content-addressable**, conceitualmente mais próximo de Git/CAS do que de um banco tradicional. Frase-âncora do professor: **"o banco deixa de ser o cérebro. O conhecimento é o cérebro. O banco é só uma forma rápida de enxergá-lo."**
> **Relação:** ESTENDE o **ADR-029** (knowledge versionável: manifest+lock+snapshot) e o **ADR-013** (bancos modulares). O ADR-029 formalizou a camada composicional (DNA no git + derivado reconstruível); este ADR define o **modelo de dados** (CAS + DAG) que sustenta essa camada, e a fundação que já existe.

---

## 0. O que o professor propôs (a visão do alvo)

Transformar a memória de conhecimento de "banco SQLite gigante" para um **grafo versionável de objetos imutáveis**:

```
                 COSCA
                   │
              ┌────┴────┐
              │  KERNEL │
              └────┬────┘
                   │
           KNOWLEDGE MANAGER
                   │
        ┌──────────┼──────────┐
        ↓          ↓          ↓
     MODULES    SNAPSHOTS   PROVENANCE
        │          │          │
   ┌────┼────┐     │     FACT/EVIDENCE/
   ↓    ↓    ↓     │     INFERENCE/DECISION
  core code  cosca  │
                    ↓
               MATERIALIZER
                    │
          ┌─────────┴─────────┐
          ↓                   ↓
       SQLite             Vector Index
      (cache)              (derived)
```

### 0.1 Conhecimento como objeto imutável content-addressable

Em vez de `documento → chunk → embedding → SQLite`:

```
Knowledge Object
       ↓
    hash
       ↓
Content Addressable Store
       ↓
Snapshot
       ↓
Materializações
```

Cada unidade de conhecimento tem identidade própria (`K:f83a91...`) e pode apontar para outras:

```
FACT
 ├── derived_from → EVIDENCE
 ├── related_to   → FACT
 └── supports     → DECISION
```

### 0.2 Snapshots copy-on-write (sem duplicação)

```
Snapshot A                    Snapshot B
├── core/001                  ├── core/001        ← mesmo objeto
├── cosca/014                 ├── cosca/014       ← mesmo objeto
├── architecture/022          ├── architecture/022
└── programming/031           ├── programming/031
                              └── cosca/057       ← novo
```

Ao adicionar um objeto, o novo snapshot **aponta** para os mesmos objetos existentes + o novo. Zero duplicação.

### 0.3 refs/ + operações estilo Git

```
refs/
├── HEAD
├── main
├── experimental
└── client/foo
```

- `knowledge checkout experimental` → troca o snapshot ativo.
- `knowledge diff main experimental` → **diff cognitivo**, não de arquivo:

```
+ FACT     runtime/...
~ INFERENCE architecture/...
- EVIDENCE old-source/...
+ DECISION ...
```

### 0.4 Embeddings separados do conhecimento

O objeto canônico `K:f83a91` gera representações vetoriais independentes:

```
K:f83a91
 ├── embedding/nomic-768
 └── embedding/model-X
```

O conhecimento não muda; só a representação vetorial muda. **Resolve a distinção arquitetural: conhecimento ≠ memória semântica ≠ índice.**

### 0.5 Garbage collection cognitivo

```
reachable   → KEEP
unreachable → GC
```

Se um objeto não pertence a nenhum snapshot ativo nem é referenciado por outro, é descartável. O crescimento deixa de ser infinito.

### 0.6 KNOWLEDGE.lock (rastreabilidade determinística)

```
Snapshot: ks_a81f...
Kernel: 0.18.2
Schema: 4
Embedding: nomic-embed-text@hash
Modules:
  core@hash
  cosca@hash
  architecture@hash
```

Então: `Decision #1842 → Knowledge Snapshot: ks_a81f...` — meses depois, reconstrói exatamente qual cérebro de conhecimento existia.

---

## 1. Mapa honesto do estado atual (o que JÁ EXISTE — e é essa a boa notícia)

> **Nota de veracidade (auditada em código, não suposição):** o professor descreve como *futuro* algo que o COSCA **já começou**. A fundação CAS **já existe e funciona**, aplicada hoje à integridade do framework embed. O salto real é **estender esse modelo ao conhecimento de domínio**.

### 1.1 A fundação CAS JÁ EXISTE — `internal/integrity`

`internal/integrity/integrity.go` implementa um **Content Addressable Store + chain assinada**:

| Peça | Onde | Estado |
|---|---|---|
| **Block** (objeto imutável) | `integrity.go:29` — `Block{Number, Hash, PrevHash, Time, Files, Manifest, HashAlgo, GitCommit, GitTree, GitAuthor, GitAnchored, Signer, Signature}` | ✅ implementado |
| **FileEntry** (hash por conteúdo) | `integrity.go:52` — `FileEntry{Path, Hash, Size}` | ✅ implementado |
| **Hash por conteúdo** | `HashAlgo` (sha256 default), content-addressing dos arquivos | ✅ implementado |
| **Assinatura** | `Signature` base64 **Ed25519**, ou `GIT-ANCHORED` (sem chave) | ✅ implementado |
| **Chain de blocos** | `.cosca/family_chain.dat` (21 MB) — blocos encadeados por `PrevHash` | ✅ implementado |
| **Verificação** | `Check(coscaRoot)` — valida assinatura de cada bloco + hash de cada arquivo; fail-closed | ✅ implementado |
| **Git-anchoring** | bloco referencia `GitCommit`/`GitTree` do HEAD; `SIGNATURE=GIT-ANCHORED` | ✅ implementado |
| **Anti-hijack** | compara chave ativa vs a versionada em git (`kernel_public.key`) | ✅ implementado |

**Conclusão:** o COSCA **já tem a espinha dorsal de CAS**. Hoje ela garante a integridade do `internal/embed/cosca` (framework). A visão do professor é **usar esse mesmo padrão para o conhecimento de domínio** — objetos imutáveis content-addressable, deduplicados por hash, com proveniência e snapshot.

### 1.2 O que o ADR-029 já formalizou (camada composicional)

| Peça | Arquivo | Estado |
|---|---|---|
| **manifest.yaml** (âncora) | `.cosca/knowledge/manifest.yaml` | ✅ criado |
| **lock.yaml** (snapshot+hash) | `.cosca/knowledge/lock.yaml` | ✅ criado |
| **Decisão ↔ snapshot** | `internal/audit` Fase 2A (`knowledge_snapshot`) | ✅ implementado |
| **Conhecimento declarativo** | `.cosca/knowledge/{laws,hall-of-fame,acquired,packages}` | ✅ existe (pequeno) |

### 1.3 O conhecimento de domínio HOJE (não é CAS ainda)

O `knowledge.db` (368 MB) continua sendo o motor: `documento → chunk → embedding → SQLite` (FTS5 + vetor + grafo). **Não é o modelo CAS** — é materialização pronta pra consumo. A proposta do ADR-030 é migrar a **fonte** para CAS (objetos imutáveis) e deixar o `knowledge.db` como **materialização** (uma das representações do snapshot).

---

## 2. Decisão — COSCA como filesystem cognitivo

**Adotar** o modelo **Knowledge Graph + Content Addressable Memory + Materialized Runtime**, em que o conhecimento é a fonte (objetos imutáveis content-addressable + DAG de proveniência) e o SQLite/índice é materialização.

### 2.0 Estrutura-alvo do `.cosca/`

```
.cosca/
├── knowledge/
│   ├── objects/               ← conhecimento canônico (CAS: K:<hash>)
│   ├── snapshots/             ← snapshots copy-on-write (apontam p/ objetos)
│   ├── refs/                  ← HEAD/main/experimental (checkout/diff)
│   ├── modules/               ← unidades cognitivas (com manifest/hash/deps)
│   └── provenance/            ← FACT/EVIDENCE/INFERENCE/DECISION (DAG)
├── indexes/                   ← materializações (SQLite, vetor, HNSW/SoA)
└── runtime/                   ← estado derivado/descartável
```

### 2.1 Conhecimento ≠ memória semântica ≠ índice (três camadas distintas)

- **Conhecimento:** objetos imutáveis content-addressable (a fonte, o "cérebro").
- **Memória semântica:** representações vetoriais dos objetos (derivada, por modelo).
- **Índice:** estruturas de busca (SQLite, FTS, HNSW/SoA) — materialização pronta.
- **Regra:** trocar o modelo de embedding **não muda o conhecimento**, só a memória semântica/índice.

### 2.2 Módulos como unidades cognitivas independentes (não pastas arbitrárias)

Cada módulo tem: `manifest`, `knowledge`, `relations`, `provenance`, `version`, `hash`, `dependencies`:

```
trading@2.4
    ↓ depends_on
architecture@3.1
    ↓ depends_on
core@1.8
```

Isso começa a parecer um **package manager de conhecimento** — módulos instaláveis, versionáveis, com dependências resolvidas.

### 2.3 Genealogia do conhecimento (Git como histórico do cérebro)

O Git passa a armazenar **como o conhecimento mudou**, não cada estado:

```
commit A → COSCA conhece X
commit B → + conhecimento Y
commit C → X revisado, Y confirmado, Z invalidado
```

Cada objeto de conhecimento sabe sua genealogia:

```
X
├── created:  commit A
├── revised:  commit C
├── evidence: E17
└── status:   SUPERSEDED
```

### 2.4 Fluxo: CAS → DAG → Snapshot → Materialização → Decisão

```
Knowledge Object (CAS: K:<hash>)
       ↓
Content Addressable Store (objects/)
       ↓
DAG de proveniência (FACT→EVIDENCE→INFERENCE→DECISION)
       ↓
Snapshot (aponta os objetos reachable)
       ↓
Materialização (SQLite + vetor + índice) ← reconstruível
       ↓
Decision → knowledge_snapshot (ADR-029 §2.4)
```

---

## 3. Implementação — incremental em fases (Mandamento III)

> **NÃO reescrever tudo de uma vez.** Cada fase é um commit independente, com **baseline de recall** obrigatório (o gate `verify`/`revalidate` não mede recall semântico — lição da revisão do CTO no ADR-029).

### 3.1 Fase A — ~~Objetos imutáveis + deduplicação (CAS no conhecimento)~~ ✅ JÁ IMPLEMENTADO (auditado)

> **ACHADO DE AUDITORIA (2026-08-29, verificado em código — não suposição):** o content-addressing e a deduplicação **já existem e funcionam** no motor de indexação. A "Fase A" como escrita originalmente **não é um gap** — re-implementá-la seria duplicar o que já existe (proibido por este ADR: "reutilizar, não duplicar"). Evidência em `internal/indexer/indexer.go`:
>
> | Primitiva CAS | Onde | Estado |
> |---|---|---|
> | Hash de conteúdo (SHA-256) | `computeHash(content)` — `indexer.go:1175` | ✅ |
> | Incremental (skip doc inalterado) | `docHashCache[path]` + `if exists && oldHash == hash` — `indexer.go:232-234` | ✅ |
> | **Dedup pré-embed** (reusa vetor existente) | `existingVectorForContent` — `indexer.go:587-698` | ✅ |
> | **Dedup preventivo** (conteúdo igual + com vetor → `dedup_of=<canonical>`, sem vetor novo) | `dedupCache[chunk.Hash]` + coluna `dedup_of` — `indexer.go:854-891` | ✅ |

**Escopo REAL da Fase A (corrigido):** NÃO criar o CAS (já existe). É **elevar/expor** os objetos content-addressable como **unidades de conhecimento persistentes e versionáveis** (o `objects/` no git + `refs`), o que hoje é implícito e derivado dentro do `knowledge.db`. Concretamente:
- Persistir `K:<hash>` como objeto canônico content-addressable, **referenciável** entre snapshots (não só `docHashCache` em memória + dedup em SQLite).
- **Baseline de recall** já registrado (`baseline_recall_test.go`, commit `9f8bf3f`) — score top-1 de 0.22–0.62 por query.

### 3.2 Fase B — DAG de proveniência

- Fazer os objetos apontarem entre si (`derived_from`/`related_to`/`supports`) — o grafo `FACT→EVIDENCE→INFERENCE→DECISION` já existe conceitualmente no `provenance.yaml`; falta materializá-lo como links entre IDs de objeto.

### 3.3 Fase C — Snapshots copy-on-write + refs 🎯 **PRIORIDADE (onde está o valor real)**

- `refs/HEAD/main/experimental` — trocar o snapshot ativo.
- `knowledge checkout` / `knowledge diff` (diff cognitivo).
- **Esta é a fase que gera a genealogia, o checkout e o diff cognitivo** — o que o professor enxergou como o diferencial. O CAS/dedup já existe (Fase A); o que dá valor é a **persistência e a navegação** dos snapshots.

### 3.4 Fase D — Separar embeddings do conhecimento + GC 🎯 **PRIORIDADE 2** (design)

**Objetivo:** um **objeto de conhecimento** `K:<hash>` pode ter **N representações vetoriais** (uma por modelo de embedding), desacopladas do chunk. Hoje o embedding está **acoplado ao chunk** via `vectors.chunk_id` — trocar de modelo exige re-embedar tudo.

**Estado atual (auditado, `internal/vector/sqlite_vec.go:112-122`):** a tabela `vectors` tem `id, vector (BLOB), metadata, document_id, chunk_id, entity_id, content, created_at`. **NÃO tem** campo para o objeto `K:<hash>` nem para o modelo — um vetor pertence a um chunk, e um chunk a um documento.

**Desenho da separação (aditivo, não destrutivo — baseline de recall é o gate):**

```sql
ALTER TABLE vectors ADD COLUMN source_hash TEXT NOT NULL DEFAULT '';   -- o K:<hash> do objeto
ALTER TABLE vectors ADD COLUMN embedding_model TEXT NOT NULL DEFAULT ''; -- "nomic-embed-text"
ALTER TABLE vectors ADD COLUMN embedding_dim INTEGER NOT NULL DEFAULT 0; -- 768
```

- **Um objeto, N vetores:** a mesma `source_hash` pode ter várias linhas (uma por `embedding_model`/`embedding_dim`). `chunk_id` continua (para a busca híbrida), mas `source_hash` é a chave do conhecimento.
- **Trocar de modelo ≠ re-embedar conhecimento:** re-embedar gera nova linha `(source_hash, embedding_model=B, dim)` — o conhecimento (`source_hash`) não muda, só a representação.
- **GC cognitivo:** objeto (objeto content-addressable) sem referência em **nenhum snapshot ativo** → embeddings e chunks associados são descartáveis. Implementar como `cosca knowledge gc` (dry-run primeiro, idempotente, com retenção mínima de safety).

**Requisitos de validação (gate):**
- `go test ./internal/knowledge/ -count=1 -run BaselineRecall` → score top-1 **idêntico** ao baseline (0.22–0.62) ANTES e DEPOIS.
- Migração aditiva via `ALTER TABLE` guarded (como a Fase 2A do ADR-029) — bancos antigos continuam funcionando.
- Teste de unidade: inserir 2 vetores para a mesma `source_hash` com modelos diferentes → ambos indexados, busca híbrida funcionando; GC remove objeto sem snapshot ativo.

> **Decisão pendente do Don (registrada):** ao separar, o `chunk_id` convive com o `source_hash`? Recomendo **aditivo** (mantém ambos) para não quebrar a busca híbrida existente — a separação plena (só `source_hash`) é um passo posterior que exige reindexação completa.<br>
> **Não delegar a implementação ANTES deste design ser aprovado.** A Fase C teve escopo mínimo definido antes de codar; a D segue o mesmo princípio (baseline + design antes de refactor de zona sensível).

### 3.5 Fase E — Modules como pacotes (com dependências)

- `manifest` por módulo + `dependencies` resolvidos (package manager de conhecimento).
- `cosca knowledge list/enable/disable`.

> **Nota de segurança:** a fundação `internal/integrity` (chain assinada, git-anchored, anti-hijack) **deve ser reutilizada**, não duplicada. A migração é **aditiva** — o conhecimento declarativo atual (`acquired/`, `packages/`, `laws.json`, `hall-of-fame.json`) NÃO é movido até que as Fases C/D (snapshots/refs + embeddings separados) estejam validadas com **recall preservado** contra o baseline (`baseline_recall_test.go`). O CAS/dedup do indexer **já existe** (§3.1) — não re-implementar.

---

## 4. Consequências

**Positivas:**
- **Conhecimento portável, bifurcável, comparável, auditável, reconstruível e mesclável** — exatamente o que o professor descreveu: `main`, `client-A`, `experiment-X`, `future-v2`.
- **Snapshots baratos** (copy-on-write, zero duplicação).
- **Deduplicação** por hash de conteúdo (objeto igual = um só).
- **GC cognitivo** → crescimento controlado, não infinito.
- **Reprodutibilidade determinística** (lock) e rastreabilidade epistemológica completa.
- **Repo git pequeno** — manifests/snapshots/refs/objetos no git; SQLite/vetores/índices fora.

**Negativas / trade-offs:**
- **Migração grande e incremental** — várias fases, cada uma exige baseline de recall; risco de regressão do recall no motor.
- **O `knowledge.db` não é "só cache" ainda** — tem estado de lifecycle (`tier`, `expires_at`), `snapshots`/backups, `sync_log`. Precisa ser resolvido antes de declarar "materialização pura" (decisão explícita do Don: o `tier` é conhecimento ou estado?).
- **Conceito complexo** — CAS + DAG + refs + modulos são mais difíceis de operar/entender do que "mexer no .db".
- **Custo de curva** — o caso de uso (agents, buscas) precisa continuar funcionando durante a migração.

### 4.1 O que NÃO muda (fronteira)

- O **motor de busca** (FTS5+veter+grafo) e o recall continuam como hoje durante a migração.
- A **epistemologia e proveniência** (`FACT/.../PROFILE`, `P0–P5`) não mudam.
- O **recall do knowledge.db** não pode regredir — baseline obrigatório.
- A **chain imutável da família** (`family_chain.dat`) é reutilizada, não duplicada.

---

## 5. Alternativas consideradas

| Alternativa | Veredito |
|---|---|
| **A. Continuar com "pasta de módulos + SQLite como cache"** | ⚠️ Funciona, mas não dá a genealogia, o checkout/diff, a deduplicação por conteúdo nem o GC. O professor é explícito: é "mais radicalmente diferente". |
| **B. CAS completo + refs + checkout + GC (ESTE ADR)** | ✅ Alvo — aproveita a fundação `internal/integrity` que já existe, e a camada composicional do ADR-029. |
| **C. Só separar embeddings do conhecimento (fatia mínima)** | ⚠️ Bom primeiro passo incremental (Fase D isolada), mas não captura a genealogia/deduplicação. |
| **D. Reimplementar tudo de uma vez** | ❌ Proibido — Mandamento III / death loop. |

---

## 6. Verificação (como saber que funciona)

1. **Deduplicação:** ingerir o mesmo conteúdo 2× → 1 objeto (`K:<hash>` igual), snapshot aponta 2× para ele.
2. **Snapshot CoW:** criar snapshot B a partir de A + 1 objeto novo → B não duplica os objetos de A.
3. **`knowledge diff`:** entre 2 snapshots, mostra diff cognitivo (FACT/~INFERENCE/-EVIDENCE/+DECISION), não diff de arquivo.
4. **Reprodutibilidade:** `Decision #X → knowledge_snapshot → lock → rebuild` → estado idêntico.
5. **GC:** objeto removido de todos os snapshots ativos → descartável; recupera espaço.
6. **Baseline de recall:** rodar `campaign_recall_test`/`campaign_realrecall_test` antes E depois de cada fase — sem regressão.

---

## Referências

- **ADR-029** — Knowledge versionável (DNA no git + derivado reconstruível; manifest+lock+snapshot; Fase 2A decisão↔snapshot).
- **ADR-013** — Bancos de Dados Modulares (particionamento por responsabilidade; `internal/modlink`/`vectoragg`).
- **ADR-028** — Servidor MCP do COSCA (epistemologia FACT/MEASURED/EVIDENCE/INFERRED/RULE/DECISION/PROFILE; proveniência P0–P5).
- **`internal/integrity`** — a fundação CAS que já existe (`Block`, `FileEntry`, `family_chain.dat`, Ed25519, git-anchored, anti-hijack).
- **Proposta do professor (2026-08-29)** — "COSCA Knowledge Graph + Content Addressable Memory + Materialized Runtime".

---

> **Autor:** Ordem do Don + orientação do professor (2026-08-29) | **Formalizado por:** cosca-architecture | **Revisão pendente:** cosca-cto + Don | **Status:** Proposed — alvo de longo prazo, implementação incremental (nunca reescrita única).
