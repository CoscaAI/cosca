# Relatório de Auditoria — Pipeline FROZEN / LIVE / RUNTIME

**Data do relatório:** 2026-09-04
**Autor:** cosca-architecture (documentação apenas; nenhuma alteração de código)
**Escopo:** Documentar o **estado atual** do pipeline de zonas (FROZEN / LIVE / RUNTIME), a direção do *fallback*, a ausência de *sync* Live→Frozen, o *drift* confirmado, e o *boot* lendo FROZEN. **Este relatório não propõe nem altera nada** — é um retrato factual para servir de base ao contrato de autoridade (`docs/specs/frozen-live-authority-contract.md`).

---

## 0. Metodologia e convenções de classificação

Cada afirmação é classificada em um dos quatro níveis epistemológicos, conforme o contrato de autoridade:

| Etiqueta | Significado |
|---|---|
| **`FACT`** | Observável e verificado no código/disco neste relatório. Independe de interpretação. |
| **`MEASURED`** | Valor numérico lido diretamente do disco (tamanho, data, hash, contagem). Factual porém específico a este instante. |
| **`INFERRED`** | Conclusão derivada de fatos, com premissa explícita. Pode mudar se a premissa mudar. |
| **`DECISION`** | Regra de autoridade normativa (vem do contrato aprovado pelo professor/Don). Não é fato observável — é mandato. |

---

## 1. As três zonas do pipeline

### 1.1 FROZEN — `internal/embed/cosca/`

- **`FACT`** É o conjunto embutido no binário via `//go:embed cosca` (`internal/embed/embed.go:20`).
- **`FACT`** É **read-only** do ponto de vista do OpenCode/runtime: o compilador incorpora os arquivos no binário; o Go não permite escrita no FS embutido. Nenhum consumidor escreve de volta no embed.
- **`MEASURED`** Contém **2005 arquivos** (recursivo).
- **`MEASURED`** O subárvore de memória do agente `memory/agent/cosca-architecture/` contém **36 arquivos**, incluindo `blocks/` (28 blocos), `merkle/` (2 arquivos), `chain.dat`, `capability-profile.md`, `evolution.md`, `failures.md`, `INDEX.md`, `learnings.md`, `patterns.md`.
- **`FACT`** Está na origem da cadeia de autoridade: quem o consome de verdade conforme os fatos levantados é o *runtime/init/fallback/ORC/knowledge*.

### 1.2 LIVE — `.opencode/cosca/`

- **`FACT`** É a superfície operacional do OpenCode.
- **`FACT`** `.opencode/opencode.json` contém `command` templates (`docs`, `evolve`, `feature`, `fix`, `init`, `plan`, `refactor`, `review`, `deploy`, `status`, `help-cosca`) que referenciam **paths de `.opencode/cosca/`**: `KERNEL.md`, `engines/<nome>/SKILL.md`, `workflows/<nome>.md`, `HELP.md`. *(Verificado em `.opencode/opencode.json`, linhas 24–77.)*
- **`FACT`** O `instructions` do `opencode.json` referencia `.opencode/cosca/memory/context/cognitive-state.md` e os "neurônios" do `.opencode/cosca/` (linha 72).
- **`FACT`** A política descoberta em `.opencode/AGENTS.md` define `.opencode/cosca/` como a biblioteca oficial de agentes/skills/prompts/workflows/templates e ordens que recurso novo seja criado **dentro** de `.opencode/cosca/`.
- **`MEASURED`** Contém **1026 arquivos** (recursivo, excluindo `node_modules` de `.opencode/`).
- **`MEASURED`** O subárvore de memória do agente `memory/agent/cosca-architecture/` contém **6 arquivos**: `capability-profile.md`, `evolution.md`, `failures.md`, `INDEX.md`, `learnings.md`, `patterns.md`. **Não** contém `blocks/`, `merkle/`, `chain.dat` (esses são derivados/gerados, presentes apenas no FROZEN).
- **`INFERRED`** O LIVE é o único dos três que o OpenCode lê diretamente como superfície de edição/configuração; o `AGENTS.md` o declara como "framework Cosca do editor".

### 1.3 RUNTIME — `.cosca/`

- **`FACT`** É estado derivado (regenerado pelo runtime).
- **`MEASURED`** `.cosca/fallback/` contém **15343 arquivos** (materializados do embed). Subárvores presentes: `engines/`, `knowledge/`, `memory/`, `workflows/` (conforme `FallbackDirs`).
- **`MEASURED`** `.cosca/framework/` contém **11 arquivos** (stub antigo: `AGENT_DNA.md`, `CONSTITUTION.md`, `CONVENTIONS.md`, `KERNEL.md`, `MEMORY_MODEL.md`, `QUALITY_GATES.md`, `knowledge/`, `memory/`, `shared/`).
- **`MEASURED`** `.cosca/knowledge.db` = **613.675.008 bytes** (≈585 MB), gravado em 04/09/2026 16:47.
- **`FACT`** Contém também vários bancos derivados (`audit.db`, `bug.db`, `semantic.db`, `vector-*.db`, `trace.db`, etc.), todos regeneráveis.

---

## 2. Direção do fallback: embed → disco (NÃO o contrário)

### 2.1 O comando e a função

- **`FACT`** `internal/cli/fallback.go:8-9` define `materializeFallback(root)` que apenas delega: `return embed.MaterializeFallback(root)`.
- **`FACT`** `internal/embed/embed.go:107-148` implementa `MaterializeFallback(root string) error`, que escreve em `<root>/.cosca/fallback/`.
- **`FACT`** Ele itera `FallbackDirs = []string{"knowledge", "memory", "workflows", "engines"}` (`embed.go:100`).
- **`FACT`** A fonte dos dados é o **FS embutido** (`ReadFile`, `WalkDir` do `embed.FS`) — ou seja, **FROZEN → `.cosca/fallback/`**.

### 2.2 Semântica (contrato explícito no código)

- **`FACT`** `embed.go:102-106` documenta: "copies the canonical framework trees from the embedded filesystem into `<root>/.cosca/fallback/`... It is **idempotent**: embedded files are (re)written on every run, and **stale extra files are left untouched**."
- **`FACT`** O comentário afirma explicitamente que a operação é idempotente e **não** apaga arquivos "stale" extras. Isso o torna um `upsert` aditivo, não um `copy-all` destrutivo.

> **Conclusão `INFERRED`:** A única materialização automática existente no pipeline é **FROZEN → RUNTIME(`.cosca/fallback/`)**. Não existe, no código, fluência **do LIVE em direção ao FROZEN**. O LIVE é editável e vive *fora* do binário; o FROZEN é a autoridade.

---

## 3. Ausência de sync/promoção Live → FROZEN automático

- **`FACT`** Nenhum `.go` no repositório escreve a partir de `.opencode/cosca/` em `internal/embed/cosca/`. Não há chamada de `WriteFile`/`Copy` apontando para `internal/embed/cosca`.
- **`FACT`** A única publicação documentada do LIVE ao FROZEN foi **manual**, em **2026-09-01**: 692 edições mais novas do LIVE foram publicadas no embed, com backup em `Temp/opencode/embed-backup-20260901`, preservando **1274 exclusivos do embed** e deixando **39 arquivos** onde o embed é mais novo. *(Fonte: `docs/reports/embed-audit-ingestao-2026-09-01.md`, §3.)*
- **`FACT`** Esse mesmo relatório (§5), como "fluxo futuro", descreve um procedimento **manual**: editar no LIVE → publicar no embed → rebuild → `cosca embed audit` → commit.
- **`DECISION`** (normativa, vinda do contrato aprovado) Não há promoção automática; promoção Live→Frozen exigirá **processo explícito e verificável** (ver `docs/specs/frozen-live-authority-contract.md`).

---

## 4. DRIFT confirmado

> **Objeto comparado (a amostra escolhida nesta auditoria):** `memory/agent/cosca-architecture/learnings.md` nos dois cérebros — a memória semântica deste próprio agente.

### 4.1 Métricas `MEASURED`

| Métrica | FROZEN (`internal/embed/cosca/...`) | LIVE (`.opencode/cosca/...`) |
|---|---|---|
| Tamanho (bytes) | **49.853** | **53.244** |
| Última gravação | **01/09/2026 00:52:18** | **01/09/2026 21:44:30** |
| SHA-256 | **FDAAEF...** (prefixo `FDAAEFA3829D6C14ACBAF393762C966091842370E3D6650F0E7C87549D3EAE9C`) | **14C57F...** (prefixo `14C57F1D0E4C521728448979A3E133BC3A39F8671E97DA695DAE0D4D328C17EB`) |
| Linhas | **217** | **230** |
| Marcadores de aprendizado (`### 20...`) | **16** | **17** |

### 4.2 Dimensão do drift (`MEASURED`)

Comparação linha-a-linha (`Compare-Object`):
- **20 linhas** existem **somente no LIVE**.
- **7 linhas** existem **somente no FROZEN**.
- **Total de linhas divergentes: 27** (20 + 7).

### 4.3 Conteúdo do drift (`FACT` sobre o diff)

O **LIVE** está à frente e contém aprendizados que o **FROZEN não tem**:
- **`### 2026-08-31 — ADR-033: Cognitive Shadow Mode`** (observação da deliberação ADR-032).
- **`### 2026-09-01 — Mineração zernio-dev`** (blueprint de plataforma de gestão de redes sociais via API; org GitHub `zernio-dev`, 27 repos; nível 3; resultado success).

O **FROZEN** contém linhas que o **LIVE não tem** (7 linhas), correspondentes a conteúdo ausente no arquivo Live — o que confirma um drift **bidirecional** (não é apenas "Live mais novo").

### 4.4 Interpretação (`INFERRED`)

- **`INFERRED`** O LIVE tem **1 aprendizado a mais** (17 vs 16 marcadores) e é **mais novo** (21:44 vs 00:52, mesmo dia).
- **`INFERRED`** O drift é o sintoma da ausência de um processo de promoção: o agente evoluiu o conhecimento no LIVE (superfície de edição), e o FROZEN (autoridade) ficou para trás. Conforme o modelo de autoridade, **FROZEN continua sendo a autoridade semântica** durante qualquer drift — o LIVE não pode ser tratado como segunda verdade.

---

## 5. Boot lê o FROZEN

- **`FACT`** `internal/editors/opencode/opencode.go` monta o prompt de boot do Kernel referenciando exclusivamente paths **FROZEN**:
  - `internal/embed/cosca/memory/agent/cosca-kernel/learnings.md`
  - `internal/embed/cosca/shared/AUTO_EVOLUTION_PROTOCOL.md`
  - `internal/embed/cosca/DESPERTAR.md`
  - A variante "*third-party*" usa literalmente a expressão "A fonte da verdade é o binário (`internal/embed/cosca/`)" (`opencode.go:42`).
- **`FACT`** `opencode.go:21` importa `embedcosca "github.com/CoscaAI/cosca/internal/embed/cosca"`, e `opencode.go:187` chama `embedcosca.IsSelfProject(...)` para escolher entre o prompt *self* (que também referencia `internal/embed/cosca`) e o *third-party*.
- **`DECISION`** Este comportamento está **correto** e deve ser **preservado** (o OpenCode não deve passar a ler o LIVE para o boot do Kernel).

> **Observação de coerência (`INFERRED`):** O `opencode.go:5-7` tem um comentário que diz que o "framework lives in `.cosca/framework/`". Isso está em tensão com a realidade: o framework real consumido é `internal/embed/cosca/` (FROZEN), e `.cosca/framework/` é só um stub de 11 arquivos. A documentação interna do `opencode.go` está **defasada em relação ao código que ele mesmo executa** — anotado como observação, não como mudança.

---

## 6. Observações adicionais relevantes (FACT)

### 6.1 Conflito de semântica sobre o LIVE (legado × atual)

- **`FACT`** `internal/cli/upgrade.go:173-208` classifica `.opencode/cosca/` como "Framework legado" e recomenda **`remover`**, com a justificativa "framework agora vive em `internal/embed/cosca/`".
- **`FACT`** `internal/editors/opencode/opencode_test.go:81` contém a asserção `.opencode/cosca must NOT exist — the framework lives in .cosca/framework`.
- **`FACT`** Apesar disso, `.opencode/opencode.json` (que é lido pelo OpenCode no boot) **referencia** `.opencode/cosca/KERNEL.md` e `.opencode/cosca/engines/...` em dezenas de templates (ver §1.2). Remover `.opencode/cosca/` quebraria o OpenCode.
- **`INFERRED`** Existe uma **contradição interna** entre (a) o modelo de "migração" assumido por `upgrade.go`/`opencode_test.go` (de que o LIVE é legado e deve sumir) e (b) o estado operacional real (o LIVE é superfície viva que o OpenCode consome). O contrato de autoridade deve **resolver** isso não destruindo o LIVE, mas colocando um **lado da autoridade** (FROZEN manda; LIVE é superfície; RUNTIME é derivado).

### 6.2 Detector de drift pré-existente (mas não Frozen↔Live)

- **`FACT`** `internal/cli/catalog_gate.go` implementa `cosca gate catalog` (`--check`/`--generate`/`--audit`), que verifica **drift do próprio LIVE** contra um snapshot canônico `.opencode/cosca/catalog.manifest`.
- **`FACT`** Esse detector compara **LIVE vs snapshot do LIVE**, estados `MATCH`/`DRIFT` (1 tipo de violação: `KindManifestDrift`) e os invariantes A (INDEX.md), B (frontmatter), C (cross-refs), D (mojibake).
- **`INFERRED`** **Não existe** hoje um detector que compare **FROZEN ↔ LIVE** por caminho+hash com os estados `MATCH`/`LIVE_NEWER`/`FROZEN_NEWER`/`ONLY_FROZEN`/`ONLY_LIVE`. O `cosca gate catalog` cobre *integridade interna do LIVE*, não *divergência de autoridade Frozen↔Live*. Este é exatamente o gap que o contrato de autoridade preenche.

### 6.3 Zoneamento de domínio no indexador

- **`FACT`** `internal/cli/db_build.go:430-455` (`vectorDomain`) classifica os documentos por path em três zonas distintas:
  - `internal/embed/cosca` → `embed-memory` / `embed-engines` / `embed-core`
  - `.opencode/cosca` → `opencode`
  - `.cosca/fallback` → `fallback`
  - (além de `docs`, `code`, `other`).
- **`INFERRED`** O indexador de conhecimento (runtime) já enxerga as três zonas como **domínios separados**, reforçando que são entidades distintas e não uma árvore única sincronizada.

---

## 7. Modelo de autoridade (referência normativa)

Conforme aprovado e estabelecido como mandato (`DECISION`):

```
FROZEN (internal/embed/cosca/)  →  ÚNICA autoridade semântica
LIVE  (.opencode/cosca/)        →  superfície operacional necessária ao OpenCode (NÃO é segunda autoridade)
RUNTIME (.cosca/)               →  estado derivado

Regra de precedência:  FROZEN > LIVE > RUNTIME.
```

- **`DECISION`** FROZEN manda. LIVE pode mudar → vira **DRIFT** → FROZEN continua autoridade → promoção Live→Frozen exige **processo explícito e verificável**.
- **`DECISION`** Não matar a evolução; colocar **fronteira de autoridade**. NÃO fazer materialização destrutiva automática.

---

## 8. Resumo executivo

| Item | Estado | Classificação |
|---|---|---|
| FROZEN como autoridade | **Confirmado** (boot/fallback/ORC/knowledge o consomem) | `FACT` |
| Direção do fallback | **FROZEN → `.cosca/fallback/`** (idempotente, aditivo, não destrutivo) | `FACT` |
| Sync Live→Frozen automático | **Inexistente** (só houve publicação manual em 2026-09-01) | `FACT` |
| Drift em `learnings.md` | **Confirmado** — LIVE 53.244B/21:44/14C57F; FROZEN 49.853B/00:52/FDAAEF; 27 linhas (20+7); 1 aprendizado a mais no LIVE | `MEASURED` |
| Boot lendo FROZEN | **Confirmado e correto** | `FACT` |
| Contradição upgrade.go/test sobre LIVE | **Existente** (LIVE declarado "legado", mas consumido pelo OpenCode) | `INFERRED` |
| Detector Frozen↔Live | **Inexistente** (existe `cosca gate catalog`, mas é LIVE vs snapshot do LIVE) | `INFERRED` |
| Materialização destrutiva automática | **Inexistente** (`MaterializeFallback` é idempotente e preserva extras) | `FACT` |

---

*Fim do relatório. Nenhum arquivo de código, embed ou boot foi alterado — apenas a criação deste documento e do contrato de autoridade (`docs/specs/frozen-live-authority-contract.md`).*
