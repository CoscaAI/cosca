# ADR-029: Memória de Conhecimento Versionável — DNA no Git + Artefato Derivado Reconstruível

> **Status:** Proposed (aguardando cosca-cto + cosca-architecture + Don) | **Owner:** cosca-architecture (Architecture Chief) | **Last Updated:** 2026-08-29
> **Revisão:** decisão de design — implementação parcial (Fase 1 aditiva, segura) concluída; Fase 2 formalizada como alvo.
> **Fonte (ordem do Don + professor, 2026-08-29):** a proposta do professor de transformar o conhecimento de "banco interno do COSCA" em **Knowledge Infrastructure**, separando em three camadas: **🧠 Kernel/código** (git), **📚 Knowledge** (módulos/facts/evidence/inferences/provenance/manifests) e **💾 Runtime State** (índices/caches/embeddings/traces — derivados). A autoridade: **o objetivo NÃO é "diminuir o banco para caber em 100MB" — é mudar a natureza da memória. O Git versiona a definição/proveniência do conhecimento; os artefatos pesados (vetores, índices, cache) são reconstruíveis e NÃO versionados.**
> **Relação:** estende o **ADR-013** (bancos modulares / particionamento por responsabilidade) e o **ADR-002** (knowledge engine). O ADR-013 trata da **divisão física** dos bancos; este ADR trata da **camada composicional versionável** (manifest + lock + snapshot) e da **reprodutibilidade de decisões** — o que faltava para fechar o ciclo.

---

## 0. Mapa objetivo do estado atual (honestidade antes do gap)

> **Nota de veracidade:** o ADR-013 (escrito em 2026-08-24) descrevia o `knowledge.db` como **257,6 MB versionado no git**. **Isso mudou.** Por decisão do Don (2026-08-24, e refletida no `.gitignore`), o `knowledge.db` **já NÃO é versionado** — ele é tratado como **índice derivado reconstruível** ("por decisão do Don 2026-08-24, arquitetura modular ADR-013: índices derivados NÃO são versionados; só o CORE imutável fica no git. Regenerar localmente: `cosca knowledge index`").

| Artefato | Onde reside hoje | Natureza | Versionado no git? | Peso real (2026-08-29) |
|---|---|---|---|---|
| **knowledge.db** | `.cosca/knowledge.db` | **Derivado** (vetores+grafo+FTS+híbrido) | **❌ NÃO** (gitignore) | **368,8 MB** |
| Conteúdo declarativo (fonte) | `.cosca/knowledge/laws.json`, `hall-of-fame.json`, `acquired/*`, `packages/*` | **Fonte** (proveniência) | ✅ | pequeno (~22 KB total) |
| **Manifest (novo)** | `.cosca/knowledge/manifest.yaml` | **ANCORA composicional** | ✅ | **3,2 KB** |
| **Lock (novo)** | `.cosca/knowledge/lock.yaml` | **snapshot reprodutível** | ✅ | **2,3 KB** |
| Ledger de proveniência | `.cosca/provenance.yaml` | append | ✅ | 33,8 KB |
| CORE imutável | `.cosca/family_chain.dat`, `keys/`, `internal/embed/cosca/` | **Imutável** (append-only/assinado) | ✅ | — |
| Runtime (derivado) | `audit.db`, `auth_tokens.db`, `department.db`, `memory/index.db`, `secrets.db`, `trace.db` | **Derivado** | ❌ (exceto .db que somam 0.1 MB) | — |
| Arquivos `.cosca/` rastreados | — | misto | ✅ | 1699 arquivos |

**Leitura honesta do gap (corrigida após revisão cosca-cto + cosca-architecture):** o problema dos **100MB já está contornado** — `knowledge.db` (368 MB) está fora do git; o `.cosca/` rastreado inteiro é **30 MB**, e os `.db` rastreados somam **0,1 MB**. **Porém**, a **estrutura modulada, determinística e versionável** que o professor descreve **não existe como sistema formal**: hoje o conhecimento declarativo é um conjunto solto (`laws.json`, `hall-of-fame.json`, `acquired/`, `packages/`) **sem** âncora composicional, sem lockfile com hashes, sem snapshot identificável, e **sem vínculo decisão↔snapshot**. Foi exatamente essa camada que a Fase 1 adicionou.

> **⚠️ Delimitação de honestidade (essencial):** o `knowledge.db` de **368,8 MB NÃO materializa só as ~22 KB declarativas do DNA.** Ele indexa o **corpus inteiro do projeto** (2.383 documentos, 38.742 chunks, 36.539 entidades / 40.889 relações, 87.552 nós — código + markdown + `fallback/knowledge`), via `Engine.Rebuild` → `indexer.RebuildAll(RootDir=projeto)` + reindexação de `acquired/`. **O manifesto/lock que a Fase 1 cria é um DNA PARCIAL — ele congela a fração declarativa nomeável, não a proveniência do corpus que reconstrói os 368 MB.** Consequência honesta:
> - **"Apagar `knowledge.db` não perde conhecimento"** → verdade para **conteúdo-fonte** (o corpus já está no repo/git); **falso para estado** — *perde* `tier/lifecycle` (`expires_at`, long/medium decidido pelo Don), `snapshots` (backups) e `sync_log`, que **não** são reconstituíveis pelo DNA atual.
> - A **reprodutibilidade plena do corpus inteiro** (todo o processo de ingestão/reindexação) é objetivo da **Fase 2** — não é algo que o manifest/lock de hoje garante.

---

## 1. Contexto / Problema (por que mudar a natureza, não só o tamanho)

O modelo atual trata o SQLite como **fonte de verdade do conhecimento** — o que o cérebro "sabe" é o que está materializado no `knowledge.db`. Isso tem três consequências estruturais:

1. **Não reprodutível:** dada uma decisão do COSCA, é impossível reconstruir "qual conhecimento o cérebro tinha quando decidiu" — o banco mutável já avançou.
2. **Não versionável semanticamente:** um commit captura `binary database changed` (ou um inventário flat de 2387 hashes), nunca um **diff semântico** do conhecimento.
3. **Acoplado ao artefato:** embeddings, índices e cache derivados vivem junto do conteúdo-fonte; qualquer mudança "suja" o artefato inteiro.

**Problema em uma frase:** separar **Knowledge Source** (fonte declarativa, versionável, com proveniência + manifest + lock) de **Knowledge Materialization** (SQLite, vetor, índice, cache — reconstruíveis), de modo que o Git versione o **DNA** da memória e o cérebro reconstrua o derivado — e que cada decisão seja **reproduzível** contra um snapshot identificável.

**Alinhamento com o que já existe:** o COSCA já tem a epistemologia (`FACT`/`MEASURED`/`EVIDENCE`/`INFERRED`/`RULE`/`DECISION`/`PROFILE` — as **7 classes** de `epistemic_class.go`), a proveniência (`P0–P5`), o knowledge engine híbrido (FTS5+vetor+grafo), snapshots, auditoria, recovery, a chain de commits da família e o pipeline `cosca knowledge {rebuild, manifest, verify, revalidate, resolve, status, claim}`. **Este ADR não duplica isso** — ele adiciona a **camada composicional** que falta e amarra as peças num ciclo reprodutível.

---

## 2. Decisão — Knowledge como sistema modular, determinístico e versionável

**Adotar** o modelo de três camadas, em que o Git versiona o **Knowledge Source** (DNA) e os **derivados** são reconstruíveis:

### 2.0 Arquitetura em camadas (o "pulo do gato")

```
COSCA
├── 🧠 Kernel / código            → versionado no Git (ADR-013)
├── 📚 Knowledge (SOURCE = DNA)
│    ├── manifest.yaml            → âncora composicional (já criado)
│    ├── lock.yaml                → snapshot reprodutível + embedding (já criado)
│    ├── modules/*                → módulos nomeados (facts/evidence/inferences/relations)
│    └── provenance.yaml          → ledger de proveniência (já existe)
└── 💾 Runtime State (DERIVADO)
     ├── knowledge.db             → materialização (reconstruível) — NÃO versionado
     ├── vector.db / fts.db       → índices pesados (ADR-013) — NÃO versionados
     └── caches, traces           → descartáveis
```

### 2.1 O SQLite DEIXA de ser a fonte de verdade

```
Git
 ↓
Knowledge Source (manifest + modules + lock)
 ↓  resolver
Module/Path Resolver             [Fase 2 — refactor do motor]
 ↓  materializar
Materializer
 ↓
SQLite (knowledge.db)             → CACHE MATERIALIZADO, não fonte
 ↓
Vector Index / FTS                → derivados, reindexáveis
```

> **Nomenclatura (correção após revisão):** este ADR chama a peça de **Module/Path Resolver** (resolve a composição do manifest em paths/consultas ao motor). **NÃO é o "Knowledge Resolver"** que já existe em `internal/knowledge/resolver.go` + `cosca knowledge resolve <task>` (tarefa→lacuna de conhecimento / gap detection). Evitar sobreposição de nome.

- **Apagar `.cosca/knowledge.db` NÃO perde conteúdo-fonte.** Recupera com `cosca knowledge rebuild`. **Perde estado** (tier/lifecycle, snapshots, sync_log) que o DNA atual não reconstrói — ver §0 delimitação de honestidade.
- Mesmo conhecimento + **mesmo embedding model** + mesma fonte → **mesmo índice**.
- **Mesmo conhecimento + embedding model diferente ≠ mesmo índice** — por isso o `lock.yaml` registra o `model_hash` (ponto técnico crítico do professor).
- **Drift sem trocar modelo:** o índice também depende do **chunker/parser de frontmatter, tokenizer, config do indexer** e do pre-embed dedup. O `lock.yaml` deve registrar `indexer_version`/`chunker_version` para cobrir drift silencioso de recall.

### 2.2 Manifest.yaml — a âncora (Fase 1 já cria)

Composição declarativa do que o cérebro conhece:

```yaml
schema_version: 1
knowledge:
  id: cosca-knowledge
  version: 0.8.0
modules:
  - id: cosca-core        # path: knowledge/laws.json
  - id: pkg-nest          # path: knowledge/acquired/nest
embedding:
  provider: ollama
  model: nomic-embed-text
  dimensions: 768
  normalization: cosine
index:
  type: hybrid
  rebuild_cmd: cosca knowledge rebuild
```

### 2.3 Lock.yaml — o snapshot reprodutível (Fase 1 já cria)

Análogo ao `package-lock.json` — congela exatamente o que está carregado:

```yaml
knowledge_snapshot: ks_2026_08_29_001
modules:
  cosca-core: { version: 1.0.0, hash: sha256:6097b55d8065fed0 }
  pkg-nest:   { version: 1.0.0, hash: sha256:3558e1bb4ae16193 }
embedding:
  provider: ollama
  model: nomic-embed-text
  model_hash: sha256:<digest-do-provedor>     # amarrar ao EmbeddingDigest REAL (fail-closed), não a um pseudo-hash
  dimensions: 768
  normalization: cosine
indexer:
  indexer_version: <versão do indexer>         # cobre drift sem troca de modelo (chunker/parser/config)
  chunker_version: <versão do chunker>
```

> **Correção (após revisão CTO):** o `model_hash` deve ser o **digest de identidade do embedding provider** (`EmbeddingDigest`, já usado em `internal/embeddings/provider.go` + `ErrEmbeddingIdentityMismatch` em `knowledge.go:238` — fail-closed) — **não** um pseudo-hash do nome do modelo. E adicionar `indexer_version`/`chunker_version` para o drift do ponto 3 do §2.1. O lock também deve registrar **data de geração/validade** para distinguir snapshots quando o modelo mudar.

**Responde à pergunta:** *"Qual conhecimento o cérebro tinha quando tomou esta decisão?"* → ler o `lock.yaml` apontado pela decisão (proveniência).

### 2.4 Decisão → snapshot (reprodutibilidade) — Fase 2

Cada decisão passa a registrar seu contexto congelado:

```yaml
decision_id: dec_0192
knowledge_snapshot: { id: ks_2026_08_29_001, hash: sha256:... }
memory_snapshot:    { hash: sha256:... }
runtime:            { version: 0.14.2 }
```

---

## 3. Implementação — em duas fases

### 3.1 Fase 1 ✅ CONCLUÍDA (aditiva, segura — não quebra o motor)

| Entrega | Arquivo | Estado |
|---|---|---|
| Manifest (âncora composicional) | `.cosca/knowledge/manifest.yaml` (3,2 KB) | ✅ criado, validado YAML |
| Lock (snapshot + embedding) | `.cosca/knowledge/lock.yaml` (2,3 KB) | ✅ criado, validado YAML |
| Registro no ledger | `provenance.yaml` claim `knowledge-fase1-manifest-lock` | ✅ |

**Decisão de segurança (Mandamento III):** a Fase 1 foi **aditiva** — NÃO moveu `acquired/`, `packages/`, `laws.json`, `hall-of-fame.json`, porque o motor lê esses **paths fixos**; mover quebraria `acquire`/snapshot/recall. O manifesto referencia esses paths por caminho, sem alterá-los.

### 3.2 Fase 2 🎯 ALVO — separatória em dois trilhos (após revisão CTO + Arquiteto)

**Trilho 2A — recomendado JÁ (barato, alto valor, não toca o motor):**

1. **Decisão ↔ snapshot** — vincular `knowledge_snapshot`/`memory_snapshot` ao `provenance` de cada decisão (o ciclo de reprodutibilidade do §2.4). É a única parte da Fase 2 que desbloqueia algo **realmente novo** (reproduzir o contexto no qual uma decisão foi tomada) e não refatora o motor.

**Trilho 2B — adiado, exige ADR de refactor DEDICADO + baseline de recall:**

2. **Module/Path Resolver** — resolve a composição do manifest em paths/consultas ao motor (refactor de `internal/knowledge`).
3. **Módulos nomeados** — `modules/core/`, `modules/architecture/`, `modules/programming/`, etc., com `facts/`, `evidence/`, `inferences/`, `relations/`, cada um com seu `manifest.yaml` próprio e cadeia de proveniência.
4. **`cosca knowledge list / enable / disable`** — sistema de "pacotes cognitivos" ativáveis.
5. **Snapshot automático** — regenerar `lock.yaml` a cada mudança de fonte; amarrar ao `EmbeddingDigest` real.

> **Fronteira com o ADR-013 (correção após revisão Arquiteto):** o "Module/Path Resolver" **sobrepõe** a zona de código do ADR-013 §6 Fatia 1 (`internal/modlink` Ref+Resolve+Drift + `internal/vectoragg` agregador via `ATTACH`). Antes de implementar o 2B, **cruzamos as fatias**: ou o Module/Path Resolver é o MESMO mecanismo de `modlink`/`vectoragg` do ADR-013 (e definimos a precedência de quem implementa primeiro), ou explicitamos por que é diferente. **Não podem ser dois refactors concorrentes do mesmo engine.**
>
> **Requisito prévio do 2B (não negociável):** registrar **baseline de recall** antes (os `campaign_recall_test.go`/`campaign_realrecall_test.go` já medem recall real — precisa de um baseline versionado). O gate `verify`/`revalidate` confere **integridade de hash/estrutura**, NÃO recall semântico; sem baseline, "mitigado por verify" é insuficiente para um refactor que mexe em `compiler`/`resolver`.
>
> **Nota:** mover os paths fixos para módulos nomeados exige refactor do motor (`internal/knowledge`) para resolver paths via manifest, e **migração idempotente e verificável** dos arquivos existentes — nunca destrutiva. `packages/*` hoje é **runtime/gitignored** (`knowledge.go:1813`), então não é fonte no git — precisamos decidir se passa a ser fonte (e vai pro DNA) ou permanece derivado.

---

## 4. Consequências

**Positivas:**
- **Git limpo e semântico:** diffs de conhecimento (não "binary db changed"); `.cosca/` rastreado fica leve (~30 MB, dominado por DDL declarativa, não por binário).
- **Reprodutibilidade total:** decisões reproduzíveis contra snapshots identificáveis; conversa direto com a auditoria/recovery já existente no Kernel.
- **Conhecimento sobrevive ao banco:** `knowledge.db` é descartável; `cosca knowledge rebuild` reconstrói a partir do DNA.
- **Escalável:** embeddings/indexes podem chegar a 500 MB–5 GB sem afetar o repositório; artefatos distribuíveis separadamente.

**Negativas / trade-offs:**
- **Fase 2 (trilho 2B) exige refactor** do `internal/knowledge` + migração idempotente; risco de regressão do recall — **exige baseline de recall** (o `verify`/`revalidate` não mede recall). Segurança: não implementar 2B sem baseline versionado.
- **Perda de estado no rebuild** — apagar `knowledge.db` e reindexar **perde** `tier/lifecycle` (long/medium, `expires_at`), `snapshots` (backups) e `sync_log`. O conteúdo-fonte volta; o **estado não**. Se o Don decidir que o tier É conhecimento (fonte), ele deve migrar para o DNA — decisão explícita necessária.
- **Lockfile precisa ser mantido** — se o embedding model/chunker mudar, o snapshot anterior fica obsoleto (o `lock.yaml` precisa registrar data/modelo/versões para distinguir snapshots).
- **Curva de aprendizado** — a operação deixa de ser "mexer no .db" e passa a ser "editar fonte + reindexar".

### 4.1 O que NÃO muda (fronteira deste ADR)

- O **motor de busca híbrida** (FTS5 + sqlite-vec + grafo) permanece o mesmo — este ADR não altera o recall, só a **camada composicional** (manifest/lock/snapshot) que o governa.
- O **pipeline de ingestão/reindexação** (`cosca knowledge rebuild`) continua sendo o mecanismo de materialização — não substituímos o materializer, só o tornamos **reproduzível** a partir de um DNA identificável.
- A **epistemologia e a proveniência** (`FACT/.../PROFILE`, `P0–P5`) não mudam — o ADR apenas as **amarra** a snapshots.
- A **chain imutável da família** (`family_chain.dat`, blocks assinados) permanece a âncora de verdade imutável — este ADR opera na camada **acima** dela (conhecimento derivado versionável).

---

## 5. Alternativas consideradas

| Alternativa | Veredito |
|---|---|
| **A. Manter `knowledge.db` no git como snapshot** | ❌ Rejeitada — commit gigante a cada sessão; já superado (fora do git desde 2026-08-24). |
| **B. Apenas diminuir o banco (poda/vacuum)** | ❌ Rejeitada — não resolve a natureza; o conteúdo cresce de volta. O professor é explícito: não é "diminuir", é "mudar a natureza". |
| **C. Módulos como pastas, sem manifest/lock** | ⚠️ Parcial — organização visual, mas sem determinismo nem reprodutibilidade (o professor: "não basta quebrar a pasta em módulos"). |
| **D. Knowledge Source + lock + manifest + snapshot (ESTE ADR)** | ✅ Escolhida — alinha com ADR-013/002 e com a epistemologia já existente. |

---

## 6. Verificação (como saber que funciona)

1. `cosca knowledge verify` → integridade do índice após rebuild.
2. `cosca knowledge revalidate` → hashes da fonte batem com o `lock.yaml`.
3. **Teste de reprodutibilidade (conteúdo):** apagar `knowledge.db` → `cosca knowledge rebuild` → `cosca knowledge verify` → conteúdo-fonte idêntico. (Atenção: o **estado** tier/snapshot/sync_log NÃO volta — ver §4 trade-off.)
4. **Teste de recall (baseline):** rodar `campaign_recall_test`/`campaign_realrecall_test` antes E depois de qualquer refactor (2B) e comparar contra um baseline versionado — o gate real de não-regressão.
5. `git status .cosca/knowledge/` → só `manifest.yaml`/`lock.yaml` (e a fonte) mudam; **nenhum binário**.

---

## Referências

- **ADR-013** — Bancos de Dados Modulares (particionamento por responsabilidade; `internal/modlink`/`vectoragg` Fatia 1).
- **ADR-002** — Knowledge Engine (motor de busca híbrida, ingestão, reindexação).
- **ADR-028** — Servidor MCP do COSCA (epistemologia FACT/MEASURED/EVIDENCE/INFERRED/RULE/DECISION/PROFILE; proveniência P0–P5).
- **`.gitignore`** — regra 2026-08-24: `knowledge.db`/`*.db-wal`/`*.db-shm`/`logs`/`cache` fora do git.
- **`internal/knowledge/{acquire.go, knowledge.go, resolver.go, knowledge_manifest.go}`** — estados e nomenclatura atuais (delimitação de honestidade).
- **Proposta do professor (2026-08-29)** — "Knowledge Source versionável; SQLite = materialização; snapshot identificável; decisões reproduzíveis".

---

> **Autor:** Ordem do Don + orientação do professor (2026-08-29) | **Formalizado por:** cosca-architecture | **Revisado por:** cosca-cto + cosca-architecture (pareceres capturados) | **Status:** Proposed — aguardando Don.
