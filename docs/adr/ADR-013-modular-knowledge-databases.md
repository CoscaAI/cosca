# ADR-013: Bancos de Dados Modulares — Memória Imutável como Âncora + Módulos Coesos

> **Status:** Proposed | **Owner:** cosca-architecture (Architecture Chief) | **Last Updated:** 2026-08-24
> **Revisão:** aguardando cosca-cto + cosca-security + Don. **Decisão de design — NÃO implementada.**
> **Fonte:** Visão do Don (proposta arquitetural de bancos de dados modulares) + tese do kernel semântico (apreensão holística) + `.opencode/cosca/memory/context/cognitive-state.md` + `ADR-012` (2 zonas) + **Decisões de governança do Don (2026-08-24): Limite 100MB por banco, Zero Redundância de conteúdo (Core → módulos), e residência dos índices pesados em módulos derivados** + **refinamento do professor/advisor técnico (2026-08-24)**: regra anti-monstro (§2.0), modelo de 4 níveis com Core-mapa (§2.1), tabelas de governança `module`/`capability`/`route` (§3.0), router determinístico (§3.2), submódulos (§2.1), gatilho zero-conteúdo (§2.1/§3.0), World Model como linguagem (§1/§7/§9) e busca semântica modular (§3.2).
> **Relação:** especifica o desenho de "bancos por módulo, idempotentes, orquestrados" que hoje vive fragmentado. Não altera código, não migra banco, não roda nada destrutivo.

---

## 0. Mapa objetivo do estado atual (o que JÁ existe — honestidade antes do gap)

Antes de declarar o desenho, o mapeamento real. A premissa central do Don é que "hoje há **um** banco único" — e isso é **verdade para o conhecimento**, mas **falso para o runtime** como um todo. Ser honesto aqui muda a forma do ADR:

| Artefato | Onde reside hoje | Natureza | Mutável? |
|---|---|---|---|
| **knowledge.db (257,6 MB)** | `.cosca/knowledge.db` — **versionado no git** | SQLite, 38 tabelas (incl. shadow tables do FTS) | **Mutável** (estado) |
| Conteúdo indexado | `documents` (2.383), `chunks` (38.742), `headings` (16.501), `code_blocks` (2.903), `tables` (4.135), `symbols` (0) | SQLite | Mutável |
| Grafo | `entities` (36.539), `relationships` (32.535) | SQLite | Atualizável |
| Vetores | `vectors` (28.888 — total, 2.210 docs, 28.888 chunks) | SQLite (`vector` BLOB + `content` + refs `document_id`/`chunk_id`/`entity_id`) | Mutável (reindex) |
| Identidade (auth) | `users` (1 linha) dentro de `knowledge.db` | SQLite | Mutável |
| Busca de texto | `documents_fts`, `chunks_fts`, `code_blocks_fts`, `entities_fts` (FTS5) + triggers para sincronizar | SQLite FTS5 | Mutável |
| **Chain da família (IMUTÁVEL)** | `.cosca/family_chain.dat` + `keys/kernel_public.key` | **Arquivo** append-only, Ed25519, git-anchored (blake3) | **Imutável** (append) |
| **Blocks (memória imutável)** | `memory/agent/{agente}/blocks/<sha256>.md` + `learnings.md` (índice de gatilhos) | **Arquivos** .md assinados via chain | **Immutable** (append) |
| **Cérebro semântico** | `internal/embed/cosca/` (go:embed, read-only) | Arquivos embed | **Imutável** |
| Identidade da casa | `.cosca/framework/` (CONSTITUTION, KERNEL, MEMORY_MODEL...) + `internal/embed/cosca/` | Arquivos | Imutável |
| Ledger de proveniência | `.cosca/provenance.yaml` | YAML | append |
| Runtime (já split!) | `audit.db`, `auth_tokens.db`, `department.db`, `memory/index.db`, `secrets.db`, `trace.db` | **SQLite separados** | Mutáveis |

**Peso medido (real, 25/08/2026) — `knowledge.db` = 257,6 MB (28.888 vetores, 36.539 entidades, 32.535 relações, 38.742 chunks, 2.383 documentos):**

| Bloco | Linhas | Peso estimado | Natureza | Onde viver no alvo |
|---|---|---|---|---|
| `vectors` | 28.888 × 768 dims × 4 B | **≈ 88 MB — o MAIOR bloco** | **Derivado** (reindexável) | **`vector.db`** |
| `entities_fts` (+ `content`/`docsize`) | 36.539 | **grande** (FTS duplica texto) | **Derivado** (reindexável) | **`fts.db`** |
| `chunks` + `chunks_fts` | 38.742 | **grande** (texto dividido + índice) | **Derivado** (reindexável) | **`projects.db`/`fts.db`** |
| `entities` + `relationships` | 36.539 + 32.535 | **considerável** (grafo) | **Derivado** (atualizável) | **`graph.db`** |
| `documents` (manifest de origem) | 2.383 | **pequeno** (metadados + hash + tamanho) | **Fonte** (proveniência) | **Core** |
| `metadata` / `frontmatter` / proveniência | — | **pequeno** | **Fonte** | **Core** |

**Leitura honesta do peso:** os 257,6 MB são dominados por **índices derivados** (vetores ≈ 88 MB, FTS, chunks), **não** por conteúdo-fonte. Isso é o que torna a Decisão 3 viável: mover os **documentos originais (manifest) + proveniência + metadados** para um **Core leve**, e os **índices pesados** para módulos derivados reindexáveis. Assim o Core fica < 100 MB **e** todos os módulos também — nenhum banco do sistema excede os **100 MB** (Decisão 1).

**Conclusão do mapa:** o runtime **já é** multi-banco (audit, auth, department, memory-index, secrets, trace são arquivos SQLite independentes). O **outlier** é o `knowledge.db` que, sozinho, carrega conteúdo + grafo + vetores + FTS + identity — um oceano de 257 MB com tudo misturado. **E o que é imutável hoje** (chain + blocks + embed) **já não está em SQLite** — está em arquivos append-only assinados. Ou seja: a imutabilidade da família **já é arquiteturalmente servida por arquivos**, não por tabela reescritível. O desenho deste ADR transforma essa "meia-verdade" em propriedade explícita.

> **Verdade crítica:** o `knowledge.db` hoje está **no git** (por ordem do Don, 2026-08-22, snapshot consistente) — é exatamente isso que gera o "commit gigante a cada sessão" citado na visão. Este ADR define o **alvo**; a fatia 1 (§6) não pode declarar "módulos ativos" até o `knowledge.db` sair do set de commits ou ser dividido de forma idempotente e verificável.

---

## 1. Contexto / Problema (por que modularizar)

O `knowledge.db` concentra responsabilidades com **ciclos de vida opostos** dentro do mesmo arquivo:

1. **Memória imutável da família** — a inteligência validada, a verdade consolidada. Deve **nunca** mudar.
2. **Memória de curto prazo** (sessões) — volátil, muda a cada interação.
3. **Trilhas/eventos** — append-only, eternamente em crescimento.
4. **Conhecimento por projeto** — mutável, cresce com o código.
5. **Grafo** node/edge — atualizável a cada ingestão.
6. **Vetores** — derivados, reindexáveis, grandes (28.888).
7. **Identidade/auth** — um punhado de linhas, ciclo de vida próprio.

Misturar isso produz os três sintomas citados na visão:

- **Commit gigante a cada sessão** — porque o banco inteiro (257 MB) é versionado junto, e qualquer toque altera o arquivo (mudança disfarçada de "snapshot").
- **Busca semântica com ruído** — porque os vetores e o grafo convivem com conteúdo não-curado; a busca "bermuda" num oceano sem fronteira de domínio.
- **Módulos acoplados** — porque não há fronteira de responsabilidade; tudo lê/grava o mesmo arquivo, sem contrato de interface entre domínios.

**Problema em uma frase:** dividir o `knowledge.db` em **módulos coesos e idempotentes**, com a **memória imutável da família como âncora de comparação** (base estável), e um **índice agregador** para a busca semântica unificada — habilitando a inteligência por compreensão holística por domínio.

**Contexto de domínio (refinamento G — World Model como linguagem):** o Cosca **não constrói uma cidade específica** — ele **possui uma linguagem para representar mundos**. O fluxo é `WORLD → World Model (representação semântica) → GIS | Knowledge | Unreal`. O **World Model é a linguagem**; os **módulos fornecem conteúdo**; a **Unreal apenas representa**. Isso significa que os módulos de conhecimento (nível 2, §2.1) não são "prateleiras de conteúdo de uma cidade", são **expressões da linguagem** — e a modularização deste ADR dá a essa linguagem um **vocabulário coerente** (domínios) e um **espaço de busca disciplinado** (router determinístico → semântica, §3.2), em vez de um oceano ilegível.

---

## 2. Decisão (a arquitetura de módulos)

**Adotar** bancos de dados **por módulo**, separados, **idempotentes**, com a memória principal imutável servindo de **oráculo/âncora**.

**Princípio estrutural:** *a inteligência é a memória imutável; os módulos são os órgãos mutáveis; o índice agregador é a leitura que os une; e o kernel semântico "entende" cada módulo de uma vez ao ler um objeto coeso.* Modularidade não é organização — é a condição de compreensão holística.

### 2.0 Regra anti-monstro: particionar por responsabilidade, nunca por arquivo

> **"Não particionar por arquivo. Particionar por responsabilidade."** — **"Estrutura lógica primeiro; particionamento físico depois."**

Esta é a régua que governa **toda** a granularidade deste ADR, e **corrige** uma tendência da visão original: criar **um banco por função** (`tree.db`, `growth.db`, `lod.db`). Esse instinto é o **erro inverso** do banco único — em vez de um oceano com tudo misturado, teríamos **micro-oceanos**, cada arquivo minúsculo, sem objeto coeso para o kernel apreender, e a fragmentação de busca reintroduzida por outro caminho. **Micro-bancos é anti-padrão proibido**, não uma permuta aceitável.

O que define um módulo é a **responsabilidade** (domínio/capacidade), **não a função**. Um módulo pode agrupar **várias funções relacionadas**; e o físico (um arquivo `.db`) é uma **derivação tardia** que só acontece quando um dos critérios se aplica:

1. **Volume grande** — o módulo cresce e o gate de 100 MB (§2.2.1) exige split;
2. **Ciclo de atualização diferente** — algo muda a cada sessão, algo é imutável;
3. **Permissões diferentes** — um domínio reservado, outro aberto;
4. **Recuperação independente** — falha num domínio não pode derrubar outro;
5. **Busca muito especializada** — índice próprio (ex.: vetores/FTS5 dedicados);
6. **Performance** — isolamento de I/O/página para um hot-path.

**Consequência prática:** a estrutura **lógica** (domínios/capacidades/conhecimento, §2.1) vem **primeiro**; o particionamento **físico** (arquivos `.db`, módulos derivados, §3.x) vem **depois** e **só quando** um critério acima se aplica. A pergunta que sempre precede: *"qual é a responsabilidade?"* → depois *"isso precisa ser um arquivo separado?"*. Nunca o contrário.

### 2.0.1 PRINCÍPIO DE EXTENSIBILIDADE GLOBAL — vale para TODO o Cosca, não só o jogo (ordem do Don + professor, 2026-08-24)

> **"Isso vale pra tudo, não somente o jogo."** — **"O Cosca não é um sistema que sabe fazer jogos. É um sistema que possui capacidades especializadas e sabe determinar quais partes do conhecimento são relevantes para cada problema."**

`module`, `capability` e `route` são **mecanismos GERAIS do Cosca**, não artefatos de floresta/Unreal/jogo. O jogo é apenas **um dos domínios** que usa a arquitetura; a Unreal é **um módulo** dentre muitos.

**O Core NÃO conhece domínios como conceito fundamental.** Ele sabe apenas `module = unreal` porque **existe um módulo registrado** — não porque o sistema foi desenhado para jogos. O mesmo mecanismo serve, sem remodelar o Core, para qualquer domínio futuro:

| Módulo (existente) | Capability (exemplo) |
|---|---|
| `vegetation` | `vegetation.generate_tree` |
| `unreal` | `unreal.spawn_actor` |
| `world` | `world.query_entities` |
| `gis` | `gis.normalize_geometry` |
| `programming` | `go.optimize_algorithm` |

| Módulo (futuro, sem remodelar) | Capability (exemplo) |
|---|---|
| `robotics` | `robotics.path_planning` |
| `finance` | `finance.portfolio_risk` |
| `chemistry` | `chemistry.reaction_balancer` |
| `mathematics` | `mathematics.symbolic_solve` |
| `databases` | `databases.query_planner` |

**A infraestrutura permanece idêntica** para todos:
```
QUERY → ROUTER DETERMINÍSTICO → CAPABILITIES → MODULES → SEARCH SCOPE → SEMANTIC SEARCH → EVIDENCE
```
Só muda o **conhecimento** que está atrás de cada módulo. O `route resolver` (Fatia 1, `internal/modlink`) é **data-driven e genérico** — recebe qualquer conjunto de `Route{Trigger, Module, Capability}` e processa sem saber o que é "jogo", "finanças" ou "robotica". Nenhum domínio está hardcoded.

**O World Model também não se prende ao jogo:** é a representação do **mundo** (linguagem), não da Unreal. `GIS → World Model → Unreal`, mas também `GIS → World Model → Web visualization`, `→ Simulation`, `→ Robot environment`, `→ Reasoning`, `→ Multimodal`. A Unreal é só o **corpo/renderer** de um ambiente; o conhecimento e o raciocínio continuam sendo do Cosca.

**Consequência (o verdadeiro ganho):** ao fazer isso agora, o Cosca **não precisa criar uma arquitetura nova toda vez que aprender uma área diferente** — ele registra novos módulos/capabilities no mesmo mecanismo. Unreal/vegetation é apenas o primeiro ambiente onde isso ficou visualmente evidente (o experimento da cidade: `Google/OSM → INGESTÃO → WORLD MODEL → SEMANTIC KNOWLEDGE → ROUTING → UNREAL`).

### 2.1 Os 4 níveis: o Core é o MAPA do sistema — os módulos são domínios que POSSUEM conhecimento

O refinamento estrutural do professor/advisor técnico (com o Don) organiza o modelo de dados em **4 níveis**. O ponto-chave: **o Core NÃO é o conhecimento — é o MAPA.** É isso que torna o limite de `< 100 MB` (Decisão 1) **possível E significativo**: o conhecimento pode crescer (1 GB / 10 GB / 100 GB) **sem crescer o Core**.

| Nível | O que é | Exemplo |
|---|---|---|
| **1. Core DB** | o **MAPA do sistema** — **não guarda conteúdo** | `identity`, `contracts`, `module_registry`, `capability_registry`, `routing_rules`, `provenance`, `schemas`, `fingerprints`, `recovery metadata` |
| **2. Módulo** | domínio que **POSSUI** conhecimento | `vegetation`, `world`, `unreal`, `gis`, `programming` |
| **3. Capability** | função **lógica** (identificável, **NÃO é banco**) | `vegetation.generate_tree` (ID, descrição, input, output, dependências, owner, evidências, versão, fingerprint) |
| **4. Conhecimento** | conteúdo **pesado** (chunks/embeddings) | `vegetation.db` |

Os módulos abaixo (linha a linha) e os derivados de §3.x encaixam-se nesses níveis: o **Core** é o nível 1; os **módulos de domínio** (`vegetation`, `world`, `unreal`, `gis`, `programming`) e os **módulos infra/derivados** (`memory`, `events`, `projects`, `graph`, `vector`, `fts`) são o nível 2; as **capacidades** (`vegetation.generate_tree`) são o nível 3 (lógica, **não banco**); e os **chunks/embeddings** são o nível 4 (o peso físico).

> **Nota (refinamento F — gatilho imutável → zero conteúdo):** o Core conhece os **contratos, capacidades, versões e integridade** dos módulos — **nunca** o conteúdo interno deles. "Existe capacidade para gerar árvore?" responde-se pelo **mapa** (`vegetation.generate_tree` existe, dono=`vegetation`, module=`vegetation`), **sem carregar o conteúdo**. Invariante reforçada e concretizada pelas tabelas `module`/`capability`/`route` no §3.0.

> **Nota (refinamento E — submódulos, sem pressa de virar banco):** um módulo pode ter **submódulos lógicos** (`vegetation/trees/species`, `vegetation/foliage/grass`). Isso é organização **lógica**; só vira **banco físico separado** quando o volume justificar (os 6 critérios do §2.0). **Nunca** transformar cada subpasta em DB automaticamente.

> **Alinhamento com os 4 níveis (refinamento H):** as linhas abaixo são uma **visão operacional do runtime** (infra + derivados do que hoje está no `knowledge.db`). Os **módulos de conhecimento por domínio** (`vegetation`, `world`, `unreal`, `gis`, `programming`) não aparecem como arquivo fixo: eles **resolvem-se pelo router determinístico (§3.2) + registry (§3.0)**, e seu **conteúdo físico** (nível 4) é particionado pelos mesmos critérios de §2.0.

| Módulo | Arquivo | Conteúdo | Mutabilidade | Papel | Teto |
|---|---|---|---|---|---|
| **Core (IMUTÁVEL — FONTE DA VERDADE)** | `family_chain.dat` + `blocks/*.md` + `keys/` + `core.db` | chain assinada + blocos de memória validada + embed read-only + **documentos ORIGINAIS** (manifest 2.383) + **proveniência** + **metadados/frontmatter** + identidade | **Append-only / read-only** | **Âncora, oráculo, verdade estável** — base de comparação válida. **Nunca** carrega índice pesado (sem vetores, sem FTS, sem grafo, sem chunks) | **< 100 MB** (leve) |
| **memory.db** | `.cosca/memory/index.db` (+ `memory/*.md` com YAML frontmatter) | memória de curto prazo e camadas (session/short/long/global) | **Mutável** (TTL, prune) | memória de trabalho do runtime | **< 100 MB** |
| **events.db** | `.cosca/events.db` (migrar trilhas do `audit.db`) | trilhas/eventos/logs | **Append-only** (rotação/partição p/ obedecer o teto) | rastreabilidade, auditoria, reprodução | **< 100 MB** |
| **projects.db (derivado)** | `.cosca/projects.db` | `chunks` (texto dividido), `headings`, `code_blocks`, `tables`, `symbols` + FTS por projeto — **os `documents` ORIGINAIS ficam no Core** | **Reindexável** (reindex por projeto) | conhecimento por projeto (projeção derivada) | **< 100 MB** |
| **graph.db (derivado)** | `.cosca/graph.db` | `entities` (36.539) + `relationships` (32.535) | **Atualizável** (derivado do Core) | navegação relacional | **< 100 MB** |
| **vector.db (derivado / read-model)** | `.cosca/vector.db` | **≈ 88 MB de vetores** (768 dims) + `ref{Module,ID,Hash}` | **Reindexável** (`--rebuild`) | índice vetorial (read-model agregador) | **< 100 MB** (≈ 88% do teto → vigiar/particionar) |
| **fts.db (derivado / read-model)** | `.cosca/fts.db` | `chunks_fts` · `entities_fts` · `documents_fts` · `code_blocks_fts` (FTS5 unificado) | **Reindexável** | índice textual BM25 (projeção) | **< 100 MB** |

> **Nota de honestidade sobre o Core:** a memória imutável da família **já é, na prática, arquivos append-only assinados** (chain + blocks) — não forçamos isso para dentro de um SQLite reescritível, porque isso **desfaria** a imutabilidade. O `core.db` nasce **somente-leitura** (WAL off, `PRAGMA query_only`) abrigando o **manifest de documentos** + proveniência + metadados (Decisão 3) e o mínimo de identidade que hoje está no `knowledge.db` — **leve, sem índices pesados (sem vetores/FTS/grafo/chunks)**, para cumprir o teto de 100 MB (Decisão 1). A âncora forte é a chain + blocks.

> **📌 DECISÃO CONGELADA (Fase 0, 2026-08-24 — ordem do Don + professor):**
> **O Core NÃO contém o corpo dos documentos como fonte primária.** O Core é o **MAPA**: mantém **referências, hashes, proveniência e metadados** necessários para localizar E VALIDAR o conteúdo armazenado nos módulos — **não o conteúdo em si**. O **manifest de documentos** no Core é um **mapa de documentos** (`ref + hash + proveniência`), nunca o corpo/chunks/embeddings. O corpo mora nos módulos (nível 4).
>
> **Distinção absoluta:**
> - **CORE** responde: *"onde está? o que é? quem é o dono? qual versão? qual hash? posso confiar?"* — só mapa + contratos + identidade + proveniência + integridade. **NÃO conteúdo.**
> - **MODULE** responde: *"qual é o conteúdo? qual evidência? qual conhecimento? qual embedding?"* — o corpo + conhecimento + índices de busca.
>
> Isso resolve a tensão entre "Core guarda documentos" (versão anterior) e "Core é o mapa" (refinamento do professor): **prevalece MAPA.** O Core fica **leve (< 100 MB)** por construção, e o conteúdo cresce nos módulos sem crescer o Core.

```

  ┌─────────────────────── ZONA COFRE (air-gap, imutável) — TODOS < 100 MB ───────────────┐
  │  CORE (IMUTÁVEL — FONTE DA VERDADE, < 100 MB)                                         │
  │   • family_chain.dat  (chain Ed25519, git-anchored, blake3)                           │
  │   • blocks/<sha256>.md (memória validada da família — a verdade consolidada)          │
  │   • keys/ (chave pública Ed25519)  • internal/embed/cosca/ (cérebro read-only)        │
  │   • core.db — documentos ORIGINAIS (manifest) + proveniência + metadados + identidade │
  │     (SEM vetores, SEM FTS, SEM grafo, SEM chunks — leve por construção)               │
  │               │                                                                       │
  │               ▼  LINKS por ID (refs: Module|ID|Hash, NUNCA cópia de conteúdo)         │
  └───────────────┼─────────────────────────────────────────────────────────────────────────┘
                  │  o Oráculo valida SEMPRE contra a âncora (raízes, L418)
                  ▼
  ┌─────────────────────────── ZONA KERNEL (mutável, internet) — TODOS < 100 MB ───────────┐
  │  MUTÁVEIS:    ┌──────────────┐   ┌──────────────┐   ┌──────────────────────────────┐  │
  │  memory.db    │  memory.db   │   │  events.db   │   │   projects.db (DERIVADO)      │  │
  │  events.db    │ (sessões,    │   │ (append-only│   │   chunks + FTS por projeto     │  │
  │  projects.db  │  TTL/prune)  │   │  c/ rotação) │   │   (reindexável, --rebuild)    │  │
  │  graph.db     └──────┬───────┘   └──────┬───────┘   └──────────────┬───────────────┘  │
  │  vector.db          │                  │                           │                  │
  │  fts.db             └──────────────────┴────────┬──────────────────┘                  │
  │      DERIVADOS:                                 ▼                                     │
  │  graph.db (36.539+32.535) ──▶  ┌──────────────────────────────────────────┐            │
  │                               │  ÍNDICE AGREGADOR (read-models derivados)  │            │
  │                               │  vector.db (~88 MB vetores) + fts.db (FTS5)│            │
  │                               │  + ref(Module|ID|Hash)  — ATTACH read-only │            │
  │                               │  p/ montar; rebuild idempotente (--rebuild)│            │
  │                               └──────────────────────────────┬───────────┘            │
  │                                                              │                         │
  │                                                              ▼                         │
  │              kernel semântico: lê cada módulo COESO e compreende DE UMA VEZ           │
  └────────────────────────────────────────────────────────────────────────────────────────┘
```

### 2.2 Decisões de governança do Don (2026-08-24) — Limite 100MB + Zero Redundância + Residência dos índices pesados

Estas **três** decisões do Don são **regras de arquitetura** que incorporam ao desenho de §2.1 e têm precedência sobre qualquer nuance anterior que as contradiga. São a justificativa de por que os módulos são o que são.

#### 2.2.1 Decisão 1 — LIMITE DE 100 MB POR BANCO (gate/regra)

> "O `knowledge.db` precisa ficar abaixo de 100MB — ele vai ser a fonte da verdade. Os modulares também têm que ficar sempre abaixo [de 100MB]."

- **Regra:** **TODO** banco do sistema — o Core (fonte da verdade) **e cada módulo** (derivado ou mutável) — deve manter-se **abaixo de 100 MB**. É um **gate de arquitetura**, não uma sugestão: nenhum banco pode exceder.
- **Implicação direta:** o Core (fonte da verdade) **não pode** carregar índices pesados. Logo, os pesos dominantes (vetores ≈ 88 MB, FTS, chunks, grafo) **têm** que morar em módulos derivados reindexáveis (Decisão 3). A separação deixou de ser só "organização" — passou a ser **obrigatória** para caber no limite.
- **Estratégias para permanecer abaixo de 100 MB:**
  1. **Separar índices derivados (vetor/FTS)** em módulos reindexáveis — o índice pesado nunca mora em módulo-fonte.
  2. **Particionar / adicionar módulos** se um crescer (ex.: `vector.db` por domínio; `events.db` por ano; `projects.db` por projeto) — melhor mais arquivos coesos do que um arquivo que estoura.
  3. **O Core nunca carrega índice pesado** — por construção (leve).
  4. **Módulos append-only (`events.db`)** que crescem sem fim precisam de **rotação/partição** para obedecer o teto.
- **Como medir e alertar (gate):** verificação de tamanho **por banco** (via SQLite: `$(page_count * page_size)`; idealmente `dbstat`/`PRAGMA page_count` para on-disk real). Um comando de governança (ex.: `cosca db check --gate`) deve:
  - reportar o tamanho de cada módulo e o **% do teto** de 100 MB;
  - **alertar (warn)** quando um módulo cruza um limiar (ex.: 80% do teto = ~80 MB); e
  - **bloquear (fail)** quando um módulo tenta **exceder** 100 MB (recusar a operação que estouraria, pedindo split/otimização antes).
  - Ao se aproximar do limite → **ação**: dividir/particionar ou otimizar (VACUUM, rebuild do índice derivado), nunca "deixar passar".

#### 2.2.2 Decisão 2 — ZERO REDUNDÂNCIA DE CONTEÚDO (Core → módulos)

> "O gatilho imutável pro módulo tem redundância?" — **Resposta: NÃO DEVE TER.**

- **Regra:** o Core (imutável, fonte da verdade) **NÃO duplica o conteúdo dos módulos**. Ele **referencia por ID + hash** (`Ref{Module, ID, Hash}`) e **nunca copia** o texto/conteúdo do módulo.
- **Exemplo:** o Core guarda a memória **"K-01"**; o `memory.db` referencia K-01 por **ID** — **sem cópia**. O dado vive **no módulo dono**; o Core só guarda o **link + hash** (para detectar drift — §3.1).
- **Redundância NÃO permitida** para **conteúdo de verdade**: nunca duplicar texto/fonte **entre Core e módulos**. O Core guarda `Ref{Module, ID, Hash}`, não o corpo.
- **Escopo da regra:** incide sobre a **fonte de verdade** (Core). Índices derivados/reindexáveis (FTS, vetores) podem materializar lexemas/embeddings por natureza **do índice** — mas isso é projeção reindexável, **não** cópia de fonte no Core; a autoridade (o conteúdo original) pertence a uma única casa (o manifest no Core ou o modulo dono), e o resto referencia por ID+hash.

#### 2.2.3 Decisão 3 — ONDE VIVEM os ~88 MB de vetores + grafo (para o Core ficar leve)

Como o Core (fonte da verdade) precisa ficar **< 100 MB**, os **índices pesados** migram para módulos **derivados / reindexáveis** (nunca fonte primária):

| Índice pesado | Reside em | Reindexável? | Por quê |
|---|---|---|---|
| **~88 MB de vetores** | **`vector.db`** | Sim (`--rebuild`) | Índice derivado, reconstruível; nunca fonte primária |
| **66.000+ registros de grafo** (`entities` 36.539 + `relationships` 32.535) | **`graph.db`** | Sim (atualizável/derivado) | Derivado do Core; atualizável a cada ingestão |
| **FTS** (`chunks_fts`, `entities_fts`, `documents_fts`, `code_blocks_fts`) | **`fts.db`** | Sim | Índice reindexável |
| **Chunks** (texto dividido) | **`projects.db`** (ou camada derivada) | Sim (reindex por projeto) | Projeção derivada dos originais |
| **Core (fonte da verdade)** | **`core.db`** + chain + blocks | **Não** (imutável) | Chain + blocks + **documentos ORIGINAIS** + proveniência + metadados — **leve, < 100 MB** |

**Corolário de tamanho (que este ADR torna explícito):** sozinhos ≈ 88 MB, os vetores **consomem ~88% do teto** de 100 MB. Portanto `vector.db` **não pode** coabitar com o FTS unificado — por isso o índice textual BM25 ganha **`fts.db` próprio** e o agregador passa a ser a **leitura conjunta** (`ATTACH` read-only) sobre `vector.db` + `fts.db` + `graph.db`, mantendo cada arquivo < 100 MB.

## 3. Mecanismos e detalhes

### 3.0 Modelo de dados do Core (registry) — o Core é o MAPA, não o conhecimento

O Core (nível 1, §2.1) é um **registry de governança**. Ele responde **"existe capacidade para gerar árvore?"** procurando no **mapa**, **nunca** no conteúdo. As três estruturas de governança (refinamento C):

**`module`** — identifica um módulo de domínio/capacidade (nível 2, §2.1):

```sql
-- module: um domínio que POSSUI conhecimento (nível 2)
id              TEXT PRIMARY KEY,   -- "vegetation"
name            TEXT NOT NULL,      -- "vegetation"
version         TEXT NOT NULL,      -- versão semântica do módulo
schema_version  INTEGER NOT NULL,   -- versão do schema do módulo
status          TEXT NOT NULL,      -- active | deprecated | experimental
fingerprint     TEXT NOT NULL,      -- blake3 do schema/contratos (integridade)
storage_ref     TEXT NOT NULL,      -- Ref{Module,ID,Hash} → local físico do conteúdo (nível 4)
embedding_version TEXT,             -- qual modelo gerou os embeddings do módulo
created_at      TEXT, updated_at TEXT
```

**`capability`** — função lógica (nível 3), **NUNCA um banco**:

```sql
-- capability: função lógica identificável dentro de um módulo. É lógica, não DB.
id              TEXT PRIMARY KEY,   -- "vegetation.generate_tree"
module_id       TEXT NOT NULL REFERENCES module(id),
name            TEXT NOT NULL,      -- "generate_tree"
description     TEXT,               -- o que a capacidade faz
input_schema    TEXT,               -- schema dos dados de entrada
output_schema   TEXT,               -- schema da saída (o que a capacidade produz)
dependencies    TEXT,               -- outras capabilities/módulos de que depende
status          TEXT NOT NULL,      -- active | deprecated | experimental
version         TEXT NOT NULL       -- versão da capacidade (integridade)
```

**`route`** — o que o **router determinístico** (§3.2) usa para decidir o espaço de busca (refinamento D):

```sql
-- route: gatilho → capacidade(s) → módulo(s) → prioridade/condições
trigger         TEXT PRIMARY KEY,   -- o gatilho imutável (ex.: "árvore urbana no terreno")
capability      TEXT NOT NULL,      -- capacidade(s) acionada(s)
priority        TEXT,               -- ordem/prioridade do roteamento
module          TEXT NOT NULL,      -- módulo dono da capacidade
conditions      TEXT                -- condições/contexto p/ a rota valer
```

**Invariante (refinamento F — gatilho imutável → zero conteúdo):** o Core **nunca** materializa o conteúdo interno (nível 4) nem o corpo das capacidades. Guarda apenas `id`, `name`, `version`, `schema_version`, `status`, `fingerprint` e `storage_ref`/contratos — **o gatilho sabe o que existe, onde, versão, fingerprint e status; nunca o conteúdo**. Verificar "existe capacidade para gerar árvore?" é uma leitura de **mapa** (`capability`/`route`/`module`) **sem carregar** `vegetation.db` (nível 4). Em coerência, os **"documentos ORIGINAIS (manifest)"** listados no §2.1 no Core são um **mapa de documentos** (ref + hash + proveniência), **não o corpo dos arquivos** — o corpo/chunks reside nos módulos (nível 4).

### 3.1 Links entre módulos — honestidade técnica (SQLite)

O Don descreve **"gatilhos de banco + relações entre módulos"**, e é preciso ser direto: **SQLite NÃO tem trigger nem FK cross-database.** Um trigger (`CREATE TRIGGER`) e uma constraint `REFERENCES` só operam **dentro** de um único arquivo `.db`. Não existe `FOREIGN KEY ... REFERENCES outros.db.tabela(col)` nem gatilho que dispare em outra conexão.

Portanto, os links entre módulos são **na camada de aplicação**, por **referência por ID + resolução em código**. Especificação:

```go
// Ref identifica um registro em outro módulo. NUNCA guarda dados duplicados
// além do hash — o dado vive no módulo dono. (Decisão 2: ZERO REDUNDÂNCIA.)
// O Core imutável guarda apenas o LINK + HASH, nunca o corpo do registro.
type Ref struct {
    Module string `json:"module"` // "memory" | "events" | "projects" | "graph" | "vector" | "fts"
    ID     string `json:"id"`     // chave primária no módulo dono (ex. chunk_id, entity_id)
    Hash   string `json:"hash"`   // blake3/sha256 do registro no momento do link (âncora)
}
```

Regras de resolução (em `internal/modlink`):

1. **Zero-Redundant (Decisão 2):** cada módulo é a **única fonte de verdade** dos seus registros. O Core imutável guarda **somente** `Ref`s (`Module|ID|Hash`) — **nunca copia** o conteúdo dos módulos. É **proibido** duplicar texto/fonte entre o Core e os módulos: o dado vive **no módulo dono**; o Core referencia por ID+hash.
2. **`Resolve(ref)`** abre a conexão do módulo correto (catálogo de datasources: `module → path`) e busca por ID.
3. **Verificação de derivação:** se o `Hash` do registro atual ≠ `ref.Hash`, o resolvedor sinaliza `Ref.Drift` — o alvo **mudou depois** que o Core imutável o referenciou. Esse sinal é **exatamente o valor da âncora**: a memória imutável dá a comparação estável; quando o órgão mutável "flutua", o kernel **sabe** e reavalia contra a âncora, em vez de aceitar cegamente.
4. **Nunca** resolvemos por `JOIN` cross-db (não funciona); resolvemos por **dois passos** (buscar `Ref` no módulo A → buscar o registro no módulo B). A **transação cross-módulo** que o SQLite não suporta é substituída por **compensação** (idempotência + reindex do agregador), ver §3.3.

**Uma ferramenta real e legítima: `ATTACH DATABASE`.** Embora não dê FK/trigger cross-db, o `ATTACH` permite **abrir outros arquivos .db na mesma conexão e fazer `SELECT`/`JOIN` entre eles** (leitura). Usamos `ATTACH` somente-leitura no índice agregador para **construir/consultar** o modelo unificado. Isso é diferente de links: `ATTACH` serve à **leitura agregada**; os **links semânticos** (a intenção de "referenciar") ficam na camada de aplicação, porque a decisão de "isso aponta pra aquilo" é semântica, não um constraint de banco.

### 3.2 Router determinístico + índice agregador — a busca REFINA o espaço já roteado

**Correção crítica (refinamento D):** *"Não deixar o router depender da busca semântica."* O router é **determinístico** e roda **antes** da busca semântica. A busca semântica **refina** um espaço já escolhido; ela **nunca escolhe** o espaço. Isso elimina o erro clássico "a IA pesquisou errado": a IA pode errar o refinamento, mas o **espaço** já foi fixado por regra determinística. O agregador **opera dentro dos módulos/capabilities já roteados** — ele não escolhe o espaço, ele **materializa e consulta a projeção do espaço escolhido**.

**Fluxo completo (D + H):**

```
QUERY
  → IMMUTABLE TRIGGER      (o gatilho imutável — sabe o que existe, onde, versão, fingerprint; §3.0/F)
  → CAPABILITY ROUTER      (determinístico — resolve contra route/capability/module; §3.0)
  → MODULE SELECTION       (escolhe os módulos de domínio: world, vegetation, unreal, ...)
  → CAPABILITY SELECTION   (escolhe as capacidades: generate_tree, placement, ...)
  → SEMANTIC SEARCH        (vetor/FTS/grafo — SÓ dentro do espaço já roteado)
  → EVIDENCE
```

**Busca semântica modular (refinamento H) — reduzir o espaço de busca:** em vez de `QUERY → TODOS OS CHUNKS → VECTOR SEARCH → RUÍDO`, o fluxo é `QUERY → TRIGGER → ROUTER → MODULE SELECTION → CAPABILITY SELECTION → SEMANTIC SEARCH → EVIDENCE`. Exemplo: *"árvore urbana no terreno"* roteia para `world (spatial/entity)` + `vegetation (generate_tree/placement)` + `unreal (asset/actor)` + `materials (foliage/ground integration)` — **não** pesquisa programação/bancos/OSM. A busca semântica fica confinada ao subespaço relevante, eliminando o ruído de domínios alheios.

O conjunto `{vector.db, fts.db}` (mais a leitura de `graph.db`) é o **read-model agregador** (projeção/queries materializadas e derivadas). Ele é **reconstruível** — nada dele é fonte primária, tudo é derivável dos módulos. **Por que dois arquivos?** Porque os ~88 MB de vetores consomem ~88% do teto de 100 MB (Decisão 1); coabitar vetores + FTS no mesmo arquivo **estouraria** o limite. Cada read-model fica assim < 100 MB. Função:

1. **Construção (rebuild idempotente):** para cada módulo, o agregador **lê** (`ATTACH` read-only) e materializa na sua própria estrutura: `vector.db` → `vectors` (id, `vector` BLOB, `content`, `module`, `ref_id`, `ref_hash`); `fts.db` → FTS unificado (`chunks_fts` · `entities_fts` · `documents_fts` · `code_blocks_fts`); e `entity_index`/`edge_index` (visão achatada do grafo para re-ranking, lida de `graph.db`). Rebuild idempotente via `--rebuild` preserva o **gate de tamanho** (cada arquivo < 100 MB).
2. **Consulta:** um único `SearchRequest` (reusar `internal/oracle/search.go`) resolve o modelo unificado, **na ordem**:
   - **Fase 0 — Roteamento determinístico:** resolve `trigger → route → capability → module` (§3.0), fixando o **espaço de busca** (conjunto de módulos/capacidades). Esta fase é **imutável e determinística** — não é IA, não é semântica.
   - **Fase 1 — FTS exato** (BM25) sobre o FTS agregado (`fts.db`), **restringido ao subespaço roteado**.
   - **Fase 2 — vetor semântico** (similaridade cosseno sobre `vector.db`), **restringido** aos vetores do espaço roteado.
   - **Fase 3 — grafo** (re-ranking por `GraphDistance` real, como já ativado) para navegar entidades/relações **dentro do espaço roteado**.
   - **Fase 4 — âncora:** para cada hit, `Ref`+`Hash` é verificada contra o Core imutável. Resultados de módulos mutáveis são **etiquetados** com a classe de proveniência (`EvidenceClass{FACT, MEASURED, EVIDENCE, INFERRED, HYPOTHESIS}`) e **re-classificados** se o `ref_hash` divergir (deriva o "confiabilidade temporal" da resposta).
   - **Fase 5 — dedup + merge** por `ref_id` (o mesmo chunk pode vir por FTS e por vetor; consolida e pontua).
3. **Resultado:** busca **inteligente E segura** — inteligente porque une FTS+vetor+grafo **dentro de um espaço já roteado**; segura porque a âncora imutável qualifica o que é verdade estável versus o que é órgão mutável.

**Por que agregador e não buscar direto nos módulos?** Porque busca semântica exige **milhões de vetores comparáveis num único espaço** — se cada módulo tivesse seu próprio índice vetorial, a busca não seria unificada e voltaria o "ruído" da busca fragmentada. O agregador centraliza a comparação **do espaço já roteado**; os módulos mantêm a **fonte**; o Core mantém a **verdade**.

### 3.3 Idempotência e migrações por módulo

Cada módulo tem seu **próprio schema versionado** e seu **próprio `migration_history`**. Reusar o mecanismo existente (`internal/sqlite/migrations.go`):

- **Migrações versionadas com checksum**: `Version` + `Name` + `UpSQL`/`DownSQL` + `Checksum`. O `Up()` verifica a história (`getCurrentVersion`), detecta **drift** fail-closed (`validateAppliedMigrations` — se uma migração aplicada mudou de nome/checksum, para) e aplica as pendentes. `CREATE IF NOT EXISTS` garante **repetibilidade** (rodar de novo não quebra).
- **Verificação estrutural estrita** (reusar padrão `verifyDurableSchema` / `verifyRegisteredObjects`): para o schema do `vector.db` e do `core.db`, validar colunas/índices/FKs **por nome**, não apenas "existe a tabela" — porque `CREATE IF NOT EXISTS` não conserta tabela incompatível.
- **Idempotência de dados**, além de schema: `INSERT OR REPLACE`/`UPSERT` por chave natural (hash) nos módulos mutáveis; o `vector.db` é **sempre truncado e reconstruído** (é derivado), nunca migrado incrementando.
- **Fonte de verdade do estado**: alguns módulos usam `migration_history` (como hoje); o `PRAGMA user_version` **não** é confiável para isso (hoje `knowledge.db` está com `user_version=0` apesar de 6 migrações aplicadas).

### 3.4 Imutabilidade do Core — como garantir (garantia real, não só regra)

| Camada | Mecanismo | Como garante |
|---|---|---|
| **Chain assinada** | `family_chain.dat`, Ed25519, git-anchored, blake3 | Cada bloco assina o manifest; `integrity.Check()` no boot (fail-closed: "family chain breach — startup blocked") |
| **Blocks** | `blocks/<sha256>.md` nomeado pelo hash do conteúdo | Conteúdo endereçado por conteúdo; alterar o arquivo muda o hash → aponta arrefado/inexistente → `VerifyOrFatal` falha |
| **Cérebro read-only** | `internal/embed/cosca/` via `go:embed` | Sem caminho de escrita; quem edita a si mesmo para se consertar morre |
| **core.db read-only** | `PRAGMA query_only = 1`, WAL off | A conexão não aceita escrita; o módulo Core é lido por qualquer um, escrito por **ninguém** (só identidade nova via chain) |
| **Verificação seletiva** (honestidade) | `Check()` confere **apenas o último** bloco contra o disco (estado atual); blocos históricos = assinatura somente | Multi-bloco não gera falso-positivo em chains longas |

**Regra de ouro da imutabilidade:** nenhuma escrita ao Core passa sem **prova criptográfica** (chain assinada). Ajuste de identidade **não** reescreve o Core — é **append** de um novo bloco assinado, e a verdade "atual" é o último bloco. Isso preserva a imutabilidade (só cresce) e dá a **base de comparação estável**.

---

## 4. Impacto e o que NÃO muda

**Muda (aditivo, na direção do alvo):**
- Novo `internal/modlink` (Ref + catálogo de datasources + `Resolve` + detecção de drift).
- Novo `internal/vectoragg` (read-model agregador; `ATTACH` read-only + rebuild idempotente).
- Divisão conceitual e física de `knowledge.db` → **`core.db` (fonte da verdade, leve)** + **derivados** `projects.db` (chunks/FTS por projeto) + `graph.db` + `vector.db` (~88 MB vetores) + `fts.db`.
- **Os `documents` ORIGINAIS + proveniência + metadados migram para o Core** (Decisão 3); os índices pesados (vetores/FTS/grafo/chunks) ficam nos módulos derivados reindexáveis.
- Novo **gate de tamanho** `cosca db check --gate` (Decisão 1): mede % do teto de 100 MB por banco, alerta em ~80% e recusa operação que estoure.
- Migração idempotente de 257 MB (a parte mais arriscada — ver §5).
- Os módulos mutáveis (memory/events) saem do snapshot versionado do git; só as **projeções derivadas** são regeneráveis.

**NÃO muda (o que já é a fundação):**
- `internal/oracle` (semântica + gate fail-closed) — intocado; é quem valida contra a âncora.
- `internal/sqlite/migrations.go` — reusado por módulo; contrato já está correto.
- `internal/integrity` (chain, Ed25519, git-anchor) — intocado; é a **âncora** do ADR.
- `internal/memory/store.go` (`memory/index.db` + `memory/*.md` FTS) — já é um "módulo" de fato; formalizamos.
- Os runtime DBs já separados (`audit.db`, `auth_tokens.db`, `secrets.db`, `trace.db`) — permanecem; `events.db` migra as trilhas do `audit.db`.
- O **desenho de 2 zonas** do `ADR-012` — este ADR o reconhece e encaixa os módulos dentro das zonas (Core na Cofre; módulos mutáveis + agregador no Kernel).

**Impacto operacional:**
- A busca semântica passa a depender do agregador **em sincronia** (eventual consistency). Se o agregador está atrás, os resultados são "do instante do último rebuild" — honestos, não falsos.
- Split de banco = mais conexões, mais arquivos, mas cada um menor e coeso (menos I/O de página, menos commit gigante).
- O `knowledge.db` grande deixa de ser um commit a cada sessão; só a chain é imutável e versionada.

---

## 5. Trade-offs / Riscos

| Risco | Mitigação |
|---|---|
| **Custo da divisão (complexidade de orquestração).** Passar de 1 arquivo para 5 torne as operações multi-arquivo: mais conexões a gerenciar, mais a migrar, mais caso de estado. | Bounded: a fatia 1 (§6) entrega valor isolado. Manter o `internal/sqlite` como camada única para abrir cada módulo (mesmo `Open`), e só exportar datasources via `modlink`. Não espalhar lógica de conexão. |
| **Sem transação cross-módulo (SQLite não suporta).** Uma operação que toca 2 módulos não é atômica. | Idempotência por chave natural + compensação: cada módulo é fonte única; o agregador é reconstruível; se um passo falha, o rebuild refaz (eventual consistency). Nunca prometer atomicidade cross-db. |
| **O agregador vetorial cross-módulo é o ponto único de falha de busca.** Se o `vector.db` corrompe ou está desatualizado, a busca semântica degrada para FTS-somente. | O agregador é **derivado** — sempre reconstruível com `--rebuild`. FTS por módulo continua servindo como fallback. Teste: `cosca index rebuild --verify` valida projeção contra fontes. |
| **A migração do `knowledge.db` (257 MB) é a etapa mais perigosa.** Dividir **no vivo** pode corromper o runtime (e há `family_chain` no caminho, `integrity.Check` no boot). | Migração **offline** com backup (`VACUUM INTO` já suporta) e validação por contagem/checksum. Nunca migrar com o serve rodando. Fatia 1 faz **espelho** (split de leitura) antes de qualquer escrita. |
| **Perda das FTS5 triggers por tabela.** Hoje `schema.go` tem triggers que mantêm os FTS em sincronia com as tabelas base. Espalhando por módulo, o agregador não tem esses triggers para a base mutável. | O agregador **não** depende de trigger: é **projeção** com rebuild explícito (foi por isso que o escolhemos como read-model). Os módulos que **precisam** de FTS próprio mantêm seus triggers locais. |
| **Git versionamento do `knowledge.db` continua pesado na migração.** Enquanto o arquivo grande está no git, cada toque muda o snapshot. | Alvo: só o **Core imutável** (chain + blocks) é versionado como snapshot consistente; `projects/graph/vector` são derivados e **regeneráveis** → passarão para `.gitignore` (com `--rebuild` para recompor). Comunicação com o Don é obrigatória antes de mudar a política de versionamento. |
| **Eventual consistency pode confundir a busca** (uma resposta aponta para dado mutável já alterado). | A âncora imutável resolve: `ref_hash` divergente → etiqueta `Drift` → reavalia contra a verdade estável. A busca é **segura porque sabe quando o alvo flutou**. |
| **`vector.db` (≈88 MB) está a ~88% do teto de 100 MB (Decisão 1).** É o módulo mais próximo do limite; qualquer crescimento de embeddings pode estourar. | Particionar `vector.db` por domínio/projeto; `cosca db check --gate` alerta em ~80 MB; `--rebuild` regenera sem deixar lixo; se atingir o teto, dividir em mais módulos antes de escrever (gate fail). |
| **`events.db` é append-only (cresce sem fim) e o 100 MB vira um teto rígido (Decisão 1).** | Rotação/partição por período (ex.: `events-YYYY.db`); o gate de tamanho **recusa** a operação que estouraria e sinaliza "particione antes"; arquivos antigos arquivados (sempre < 100 MB por arquivo). |
| **Risco de redundância de conteúdo entre Core e módulos (Decisão 2).** Reindexar/agregar pode, por descuido, "copiar" o conteúdo no lugar de referenciar por ID+hash. | Regra invariante em `modlink`: `Resolve`/agregador **nunca** grava o corpo no Core; só `Ref{Module,ID,Hash}`. Teste de regressão: nenhum caminho de código permite que o Core guarde texto de um módulo. |
| **O content-materializado no `fts.db`/agregador é duplicação inerente de índice.** A Decisão 2 proíbe duplicar **conteúdo de verdade entre Core e módulos**, mas FTS duplica lexemas por natureza. | Registrar explicitamente que o read-model é **projeção reindexável**, não fonte de verdade; a autoridade do texto está **numa única casa** (manifest no Core / módulo dono), e o FTS/agregador é reconstruível e descartável. A zero-redundância vale para **fonte**, não para o índice derivado. |
| **Risco de micro-bancos (o erro inverso do banco único).** Granular por **função** (`tree.db`, `growth.db`, `lod.db`) reduz cada arquivo, mas fragmenta o objeto coeso que o kernel precisa apreender de uma vez (§7) e reintroduz o ruído cross-domínio — é a mesma doença do banco único, só que em escala microscópica. | **Regra anti-monstro (§2.0) como gate contrário:** particionar por **responsabilidade** (domínio/capacidade), nunca por função; **estrutura lógica primeiro, particionamento físico depois**. Um módulo agrupa funções relacionadas; só vira arquivo separado quando aplicável um dos 6 critérios (volume, ciclo, permissões, recuperação, busca especializada, performance). Teste de coerência: cada `*.db` deve corresponder a uma **responsabilidade identificável**, não a uma função. A **busca semântica modular** (§3.2) já confina o espaço por roteamento, sem precisar de micro-arquivos. |

---

## 6. Fronteira do incremento — Fatia 1 recomendada (valor isolado primeiro)

**Escopo bounded:**

1. **Formalizar o ADR** (este documento) — o desenho deixa de ser só intenção.
2. **`internal/modlink`** — `Ref{Module,ID,Hash}` + catálogo de datasources + `Resolve` + detecção de `Drift`. Testes provam que link quebrado/derivado é detectado **e** que o Core **nunca** grava o corpo de um registro (zero-redundância, Decisão 2).
3. **Split de *leitura* (espelho)** — abrir `projects.db`/`graph.db`/`vector.db` como **views/derived do mesmo `knowledge.db`** (via `ATTACH` read-only), sem escrever ainda. Isso prova que o agregador consegue ler de módulos separados sem migrar.
4. **`cosca index rebuild --verify`** — reconstrói `vector.db` + `fts.db` (read-models) e valida projeção vs. fontes; **idempotente** (rodar 2x = mesmo resultado).
5. **`core.db` somente-leitura** — `query_only=1`, contendo os **documentos ORIGINAIS + proveniência + metadados** (Decisão 3) e o mínimo de identidade; **nenhuma** escrita de dados de conhecimento no Core, **nenhum** índice pesado.
6. **`cosca db check --gate`** (Decisão 1) — mede o tamanho por banco e o **% do teto de 100 MB**; subiu ao alerta de ~80% e recusa escrita que estoure. É a prova do gate.

**Critério de saída da fatia 1:** `go test ./internal/modlink/...` e `./internal/vectoragg/...` verdes; `cosca index rebuild --verify` passa a rodar 2x sem divergência; `cosca db check --gate` reporta cada módulo < 100 MB (e alerta corretamente para módulo em ~80%); teste prova que o Core guarda só `Ref`s (sem cópia de conteúdo); zero regressão em `go test ./...`; o `knowledge.db` **ainda** intocado (a migração destrutiva fica para a fatia 2).

**Gate de custo explícito (o que está FORA da fatia 1):**
- A **migração física/destrutiva** dos 257 MB para os arquivos-alvo (Core leve + `memory`/`events` + derivados `projects`/`graph`/`vector`/`fts`) — fatia 2+, com backup e validação, jamais com o serve rodando.
- Alterar a política de versionamento do git (mover `projects/graph/vector/fts` para `.gitignore`) — **exige aprovação do Don**, porque hoje é snapshot consistente versionado por ordem expressa.
- Mudar `internal/sqlite/migrations.go` — intocado; reusado.
- O **air-gap** do Core (ADR-012) — escopo do ADR-012, não deste.

### 📌 FATIA 2 — BUSCA OBEDECE AO SCOPE (implementado 2026-08-24) + FATIA 3 CONDICIONADA

**Fatia 2 (implementada, bounded):** `SearchParams.Scope *modlink.SearchScope` (campo opcional, `nil` = busca atual intacta). O fluxo é o professor definiu: `QUERY → modlink.Resolver.ResolveRoute → SearchScope → SearchParams.Scope → Engine.Search` — a **busca semântica REFINA o espaço já roteado** (não escolhe o espaço). Em `internal/search/scope.go`: extração determinística de módulo do `path` (segmento igual, case-insensitive) + `entity_type` como sinal secundário + `confineToScope`. **Retrocompatível.**

**⚠️ LIMITAÇÃO HONESTA + CONDIÇÃO DA FATIA 3 (regra do professor — NÃO criar módulo sem volume/fronteira):**
- O schema atual **NÃO tem coluna `domain`/`module`**; a única pista de domínio é `documents.path`/`entity_type`.
- **NÃO existe conteúdo de mundo** (`vegetation`, `world`, `unreal`, `gis`) indexado ainda: o `knowledge.db` de hoje é ~2002 docs em `internal/embed/cosca/...` + ~334 em `.cosca/fallback/memory/...`. Logo, um escopo `{vegetation, world}` retorna **vazio** (correto — nunca recai em "pesquisar tudo").
- **A Fatia 3 (coluna `domain`/`module` no schema e/ou indexar módulos de mundo) fica CONDICIONADA** a: *"quando os módulos de mundo tiverem conteúdo indexado com volume/fronteira próprios."* O professor foi explícito: **"Não criar uma tabela/módulo só porque apareceu uma nova categoria. Primeiro provar que existe responsabilidade, volume e fronteira próprios. Senão trocamos o 'banco monstro' por um 'zoológico de microbancos'."**
- **Nem todo módulo precisa virar banco imediatamente**: domínio com pouco conteúdo continua como armazenamento simples dentro da fronteira; só ganha estrutura própria quando houver volume/indexação/necessidade operacional.

**Estado das fatias:** Fatia 1 (`internal/modlink`, route resolver determinístico) ✅ commitado · Fatia 2 (busca obedece ao scope) ✅ implementado+validado · Fatia 3 (coluna de domínio / conteúdo de mundo) ⏳ **condicionada a volume/fronteira**.

---



## 7. A tese do Don — por que módulos = inteligência (o coração da decisão)

O insight não-óbvio que sustenta todo este ADR é a **tese da apreensão holística**:

> **"Se você ler cada módulo, você entende o banco de cada módulo de uma vez."**

O kernel semântico opera **horizontal + vertical**: ele **compreende um objeto de uma vez só** (apreensão holística/simultânea), não lê linha a linha como a abertura vertical do OpenCode. **Mas essa compreensão só é possível se o objeto for coeso e compreensível.**

- Um banco de **257 MB** é um **oceano com ruído** — fragmentado, poluído, com ciclos de vida opostos (memória imutável + sessões voláteis + grafo + vetores) na mesma sopa. O kernel **não consegue** entender isso "de uma vez": a holística se quebra na fronteira do domínio, porque não há fronteira — é tudo misturado.
- Um **módulo pequeno e coerente** — "projects", "graph", "events", "memory" — é um objeto **que o kernel pode ler e entender POR COMPLETO**. A leitura holística, o "apreender de uma vez", só se realiza quando há um objeto apreensível. Modularizar é **condição da inteligência**, não apenas organização.

E há a segunda metade: a **memória imutável** dá a **verdade estável** (âncora/oráculo). Sob essa luz:

- **Inteligência** (compreensão holística por domínio) veio da **modularização** — o kernel apreende cada módulo coeso de uma vez.
- **Segurança / verdade** veio da **imutabilidade** — a âncora não flutua, então o kernel sempre tem base de comparação válida.

**Os dois juntos = busca semântica inteligente E segura.** Sem módulos, não há objeto coeso para apreender (só oceano fragmentado). Sem âncora imutável, não há verdade estável (só flutuação). A modularização não é um capricho de organização: é o que **habilita a inteligência** do kernel semântico, e a imutabilidade é o que **garante que essa inteligência não esteja perseguindo uma verdade que muda a cada sessão**.

As **decisões de governança** (§2.2) estão em serviço da mesma tese, não contra ela:

- **Limite de 100 MB (Decisão 1)** é o que **força** o objeto a ser coeso e apreensível. Um banco de 257 MB é oceano com ruído; um banco **< 100 MB** é um objeto que o kernel pode ler "de uma vez". O teto não é burocracia — é a **condição prática da apreensão holística**. Se um módulo cresce, ele deixa de ser apreensível; o gate nos obriga a dividi-lo, mantendo-o coeso.
- **Zero-redundância (Decisão 2)** mantém o Core **leve e lê-lo de uma vez**. Se o Core copiasse o conteúdo dos módulos, ele viraria um segundo oceano — a âncora deixaria de ser uma verdade enxuta para virar uma duplicata ao lado da fonte. Referenciar por ID+hash é o que **preserva o Core como âncora compreensível**.
- **Índices pesados em módulos derivados (Decisão 3)** é o mecanismo que permite ao Core ser **ao mesmo tempo** a fonte da verdade e um objeto apreensível: a verdade (originais + proveniência + metadados) fica leve; os **óculos de busca** (vetores/FTS/grafo) ficam fora, reindexáveis, onde não poluem a âncora.

**O World Model é a linguagem, não o conteúdo (refinamento G):** a tese não para em "módulos coesos". O Cosca **não constrói uma cidade específica** — ele **possui uma linguagem para representar mundos**. O fluxo é `WORLD → World Model (representação semântica) → GIS | Knowledge | Unreal`. O **World Model é a linguagem**; os **módulos fornecem conteúdo**; a **Unreal só representa**. Cada domínio (`vegetation`, `world`, `unreal`, `gis`) é um **termo do vocabulário**; cada capacidade (`vegetation.generate_tree`) é uma **expressão**; e o conteúdo pesado (nível 4, §2.1) é a **instância** do que a linguagem descreve. É por isso que modularizar importa também aqui: para a linguagem ser **apreensível** — um domínio de cada vez, um mundo representável de cada vez — e não um oceano de conteúdo ilegível. Esta é a mesma decisão do Don que este ADR preserva: o Cosca tem uma **linguagem** para representar mundos; os módulos são o **vocabulário coerente** dessa linguagem, e o Core é o **mapa** que diz o que essa linguagem sabe expressar.

**Validação da tese (como mostrar que estamos certos):** o critério operacional é o **índice de compreensão** — medir se, para uma consulta, o kernel consegue localizar e explicar **o objeto completo por domínio** sem ruído cruzado. Módulo coeso (valor) → compreensão de uma vez (medida). Banco único (ruído) → compreensão fragmentada (medida). É o que a fatia 2+ pode instrumentar com os já existentes `GraphDistance`/re-ranking e `SemanticScore`.

---

## 8. Alternativas Consideradas

| Alternativa | Veredito |
|---|---|
| **Um único banco, mas "organizado"** (adicionar mais tabelas/partição lógica no `knowledge.db`). | **Rejeitado como alvo.** É o status quo. "Partição lógica" não cria objeto coeso para o kernel apreender — o oceano continua no mesmo arquivo, o commit continua gigante, o ruído continua. Não resolve a tese. |
| **Postgres/arquitetura multi-schema com FKs reais** (quase o que o Don descreve, mas com um RDBMS de verdade). | **Rejeitado como alvo imediato.** Viola local-first / zero-dependência (ADR-002) e a Lei do cofre (contexto nunca à nuvem). O custo de spin-up e a quebra de compatibilidade (modernc + FTS5 + sqlite-vec) não se justificam agora. Fica como **fase posterior** se o modelo de links/app-layer se provar insuficiente. |
| **Um banco por módulo, mas sem agregador** (cada módulo com seu índice vetorial). | **Rejeitado.** A busca semântica "unificada" se fragmentaria; reintroduz o ruído cross-módulo que estamos eliminando. O agregador é o que **une** — sem ele, é N bancos isolados, não N módulos orquestrados. |
| **Apenas mover o `knowledge.db` para `.gitignore`** (resolver só o "commit gigante", sem dividir). | **Rejeitado como suficiente.** Resolve um sintoma, não a tese. Sem modularizar + imutabilizar a âncora, o ruído de busca e o acoplamento permanecem. |
| **Módulos com FKs/triggers cross-db (como o Don descreveu literalmente).** | **Inviável em SQLite.** É a honestidade do §3.1: não existe trigger/FK cross-database. Adotamos **links na camada de aplicação** (Ref por ID + resolução + detecção de drift), que preserva a intenção do Don sem fingir que o SQLite tem um recurso que não tem. |
| **Um único banco (ou o Core) copiando o conteúdo dos módulos (redundância para "facilitar" a busca).** | **Rejeitado — Decisão 2.** Duplicar conteúdo de verdade entre Core e módulos transforma a âncora numa duplicata ao lado da fonte: quebra a zero-redundância, cria drift duplo (duas cópias a reconciliar) e infla o Core além dos 100 MB. A unificação de busca já é servida pelo **read-model derivado** (que é reindexável e descartável, não cópia de fonte). |
| **Sem limite de tamanho por banco (deixar crescer e "otimizar depois").** | **Rejeitado — Decisão 1.** Sem o teto de 100 MB, o `knowledge.db` continua a virar um oceano que quebra a apreensão holística e o commit gigante. O limite é um **gate de arquitetura** que força coesão e objeto apreensível; sem ele, a tese do Don se desfaz. |
| **Um banco por função (micro-bancos: `tree.db`, `growth.db`, `lod.db`).** | **Rejeitado — Regra anti-monstro (§2.0).** É o **erro inverso** do banco único. Reduz cada arquivo, mas fragmenta o objeto coeso que o kernel precisa compreender de uma vez (§7) e reintroduz o ruído cross-domínio. O módulo é por **responsabilidade** (domínio/capacidade), nunca por função; a **busca semântica modular** (§3.2) já confina o espaço por roteamento determinístico, sem precisar de micro-arquivos. Estrutura lógica primeiro; particionamento físico só quando justificado. |

---

## 9. Related

- `.opencode/cosca/memory/context/cognitive-state.md` — fonte da tese (apreensão holística, camadas de memória, distinção vertical × horizontal).
- `ADR-012-2-zone-cofre-kernel.md` — as 2 zonas; este ADR encaixa os módulos dentro delas (Core na Cofre, módulos mutáveis + agregador no Kernel).
- `ADR-002-knowledge-engine.md` — hybrid search (FTS5 + vetor + grafo); o agregador reusa esse modelo unificado.
- `internal/integrity/*` (chain, Ed25519, git-anchor, `SignAfterLearning`, `Check` fail-closed) — a **âncora** imutável.
- `internal/oracle/oracle.go` + `internal/oracle/search.go` — a fronteira semântica que valida contra a âncora (reuso no agregador).
- `internal/sqlite/schema.go` + `internal/sqlite/migrations.go` — contrato de schema/migração por módulo (reuso; `verifyDurableSchema` como padrão estrito).
- `internal/memory/store.go` — o `memory/index.db` (memória mutável) já é um módulo de fato.
- `.cosca/knowledge.db` (257 MB) — o estado atual; fonte da migração (§5, §6).
- `.cosca/provenance.yaml` — proveniência/ledger (move para o Core na Decisão 3).
- `internal/embed/cosca/SECURITY_PROTOCOL.md` — o data-vault (Cofre de dados); contexto de onde os módulos se encaixam.
- `AGENTS.md` (raiz) — política de versionamento do `.cosca` (o snapshot consistente versionado por ordem do Don; contexto para o risco de versionamento).
- **Modelo de domínio "linguagem de mundos"** (`WORLD → World Model → GIS | Knowledge | Unreal`) — a tese de que o Cosca **possui uma linguagem para representar mundos** (não constrói uma cidade específica); os módulos `vegetation`/`world`/`unreal`/`gis` são a expressão dessa linguagem (nível 2, §2.1; refinamento G).

---

> **Decisão pendente de revisão (cosca-cto + cosca-security + Don).** O desenho captura a visão do Don (bancos modulares, imutáveis no Core, orquestrados, idempotentes, com índice agregador) e a tese técnica ("ler cada módulo = entender de uma vez"), **com as 3 decisões de governança do Don incorporadas fielmente**: (1) **Limite de 100 MB por banco** — todo banco (Core e módulos) < 100 MB, como gate com medição/alerta/recusa; (2) **Zero Redundância** — o Core referencia por `Ref{Module,ID,Hash}` e nunca copia conteúdo dos módulos; (3) **Residência dos índices pesados** — os ~88 MB de vetores vão para `vector.db`, o grafo para `graph.db`, o FTS para `fts.db`, e o **Core (fonte da verdade)** fica leve (chain + blocks + documentos ORIGINAIS + proveniência + metadados). Com a honestidade de que: (a) a memória imutável da família **já é** arquivo append-only assinado (não um SQLite reescritível), e (b) SQLite **não suporta** trigger/FK cross-db — os links são na camada de aplicação (Ref + resolução + detecção de drift). A fatia 1 é bounded e **não** migra nada destrutivo: formaliza, cria `modlink`, prova o agregador por espelho de leitura, entrega o `core.db` read-only e adiciona o gate de tamanho `cosca db check --gate`. A migração dos 257 MB fica para a fatia 2+, com backup e validação, jamais com o serve rodando.

> **Refinamento do professor/advisor técnico (2026-08-24) incorporado:** (A) **Regra anti-monstro** — particionar por **responsabilidade**, nunca por arquivo; **estrutura lógica primeiro, particionamento físico depois**; proibido um banco por função (micro-bancos), §2.0/§5/§8; (B) **4 níveis** — Core = MAPA (não o conhecimento), Módulo = domínio que possui conhecimento, Capability = função lógica (não é banco), Conhecimento = conteúdo pesado; <100 MB é possível **e** significativo porque o conhecimento cresce sem crescer o Core, §2.1; (C) **Tabelas de governança do Core** (`module`, `capability`, `route`), §3.0; (D) **Router determinístico** — a busca semântica **refina** o espaço já roteado, nunca escolhe o espaço, §3.2; (E) **Submódulos** lógicos só viram banco quando o volume justificar, §2.1; (F) **Gatilho imutável → zero conteúdo** — o Core conhece contratos/capacidades/versões/integridade, nunca o conteúdo, reforçado nas tabelas `module`/`capability`/`route`, §2.1/§3.0; (G) **World Model como linguagem** — Cosca possui uma linguagem para representar mundos (`WORLD → World Model → GIS | Knowledge | Unreal`), §1/§7/§9; (H) **Busca semântica modular** — `QUERY → TRIGGER → ROUTER → MODULE SELECTION → CAPABILITY SELECTION → SEMANTIC SEARCH → EVIDENCE`, reduzindo o espaço de busca, §3.2.
