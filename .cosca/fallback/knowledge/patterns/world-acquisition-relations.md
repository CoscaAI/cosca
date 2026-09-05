# World Acquisition Relations — public-apis × Scrapling × Cosca

> **Category**: Agent/Mining | **Owner**: Cosca Kernel | **Date**: 2026-08-27
> **Fonte**: mineração cruzada de `public-apis` (onde estão as fontes) + `scrapling` (como adquirir) contra o código real do Cosca.

## Resumo executivo

O esqueleto do caminho "adquirir o mundo" **já está montado** nos módulos internos do Cosca (`acquisition → knowledge → grounding → providers → world`). O que **public-apis + scrapling juntos** entregam e o Cosca **não tem** são exatamente três elos:

1. Um **acquirer que consiga qualquer página do mundo** (render/stealth/HTML→md) alimentado pelo próprio `acquisition`.
2. Um **registro de fontes do mundo real** (`public-apis` como `DataSourceRegistry`).
3. Um **chunker+sanitizador** que liga o markdown limpo ao `SourceChunk` do grounding.

---

## Achado-chave que muda o diagnóstico

O Cosca **não está vazio**: já tem `internal/acquisition` (fetch SSRF-safe), `internal/knowledge` (ingestão com proveniência), `internal/grounding` (portão de fidelidade zero-LLM) e `internal/providers` (LLMs multi-provedor). **O que falta é o elo de conversão + descoberta de fontes.**

---

## RELAÇÃO 1 — Skill-to-Skill (formato Scrapling vs Skills do Cosca)

- **Fronteira:** contrato de Skill de agente. `agent-skill/Scrapling-Skill/SKILL.md` vs `.opencode/cosca/{departments,engines,skills}/**/SKILL.md`.
- **Como se encaixa hoje:** o Cosca tem **três** famílias: (1) `departments/*/SKILL.md` + `engines/*/SKILL.md` — contratos de posse/org; (2) `skills/*/*.md` — skills operacionais (how-to), o análogo exato da skill do Scrapling, via `INDEX.md`; (3) `SKILL_TEMPLATE.md` + `CONVENTIONS.md`.
- **O que falta (gap):** frontmatter do Cosca (`name/description/level`) **não tem** `version`, `license`, `metadata.openclaw.requires.bins: [python3]` (declaração de pré-requisito de tooling) nem convenção de `references/` acoplada a uma skill executável. Falta seção anti-prompt-injection.
- **Valor:** 💎💎
- **Proposta concreta:** adicionar ao template do Cosca os campos opcionais `prereqs: {bins, anyBins}` e `references:`. Reusar `skills/automation`, `departments/integrations`, `engines/tools` antes de criar (política de descoberta do `AGENTS.md`).

## RELAÇÃO 2 — Fonte → Aquisição (public-apis diz ONDE, scrapling diz COMO)

- **Fronteira:** catálogo de fontes (public-apis) → aquisição segura/stealth (scrapling) → `internal/acquisition`/`internal/knowledge`.
- **Como se encaixa hoje (o Cosca JÁ tem):**
  - `internal/acquisition/acquisition.go` — fetch endurecido (`Client.FetchAll`), validação SSRF (`validateTarget`: IPs privados/loopback/metadata, DNS fallback), `ProvenanceUntrusted="UNTRUSTED"`, SHA256, MIME, `AcquisitionTracker` de orçamento, body gravado em quarentena.
  - `internal/knowledge/acquire.go` — pipeline `manifest → source URL → fetch → markdown → FTS5`, gate de saúde do repo, fail-closed `--allow-remote`, dedup por SHA256.
- **O que falta (gap):**
  - `acquire.go:184-200` (`buildAcquireURL`) só constrói `api.github.com/repos/<org>/<repo>/readme` — **preso a GitHub README**. Sem aquisição de página web arbitrária/API do mundo real.
  - `acquisition.Client` é **só GET HTTP** — sem render de JS/SPA, sem stealth/impersonate TLS, sem bypass de Cloudflare, sem rotação de proxy, sem selectores adaptativos.
  - **Sem conversão HTML→Markdown.** `writeAcquiredMarkdown` grava o body como `.md` assumindo que já é markdown. `internal/markdown` só **parseia** `.md`, não **converte** HTML.
  - `internal/chat/tool/web_fetch.go:47-51` está com **`TODO ... not implemented (stub) — integrate before use`** (Jina Reader). A ferramenta de percepção web do agente está **morta**.
- **Valor:** 💎💎💎 (fecha o elo de percepção — win mais rápido).
- **Proposta concreta:** implementar o stub `web_fetch.go` chamando a CLI/MCP do scrapling (`scrapling extract <url> <out>.md [--css-selector] [--ai-targeted]`), roteando por `acquisition.Client` p/ herdar `--allow-remote` + proveniência UNTRUSTED + quarentena + orçamento. Acoplar como provider/plugin em `internal/acquisition` (novo `scrapling.go`) e um **sidecar** (scrapling roda separado, servido via MCP HTTP com token — `ai.py:170`). O `--ai-targeted`/ad-block já mitiga prompt-injection — alinhado à postura "EXTERNAL ≠ TRUSTED".

## RELAÇÃO 3 — Aquisição → Grounding (markdown → conhecimento com fidelidade)

- **Fronteira:** `page.markdown()`/`SiteToMarkdownSpider` (scrapling) → `internal/grounding` → `internal/knowledge` (Mundo).
- **Como se encaixa hoje (o Cosca JÁ tem o "fim do funil"):**
  - `internal/grounding/` — portões de fidelidade determinísticos (zero-LLM): `ExtractClaims`, `VerifyClaim` (token-overlap+bigrama+número), `SourceChunk{ChunkID, DocumentID, DocumentPath, Content, MatchingTerms, Score}`, gates `recall/cascade/answer/qrels/golden/fidelity`. Fail-open se o scorer não estiver disponível.
  - `internal/knowledge/` — `IngestLearningBlock` (classifica fail-closed → dedup SHA256 → indexa com proveniência semântica), `epistemic.go`, `provenance.go`, `entity_vectors.go`, `compiler.go` (FTS5).
  - `internal/providers/openaicompat/embeddings.go` + `internal/search` — retrieval vector/BM25 que alimenta `SourceChunk`.
- **O que falta (gap):**
  - **O chunker/conversor a montante.** `AcquiredArtifact` **não carrega `Content`** (só `Notes`=exerp de 500 chars via `shortExcerpt`); o body HTML bruto é descartado. Não há elo `HTML → markdown limpo → chunks → SourceChunk → vector`.
  - **Sanitização anti-modelo.** A remoção de `script/style/aria-hidden/template/comentários/zero-width` está **só no scrapling**. O `IngestLearningBlock` tem proveniência e classifier fail-closed, mas **não** o passo de "strip conteúdo oculto" antes de indexar — conteúdo hostil pode chegar ao LLM.
  - **Mapeamento de citação.** Sscrapling emite `{url,title,markdown}`; Cosca usa `[Chunk N]`/`[K-xxxx]`. Falta mapear `url → DocumentID` e `chunk → ChunkID`.
- **Valor:** 💎💎 (a fidelidade já existe; falta o transformador a montante + sanitização).
- **Proposta concreta:** inserir um conversor em `internal/acquisition`/`internal/knowledge`: após `FetchAll`, se `Content-Type: text/html`, transformar via scrapling `.markdown(main_content_only=True)` e gravar como `writeAcquiredMarkdown`; depois chunker → `SourceChunk` com `DocumentID=<url>` e `ChunkID` estável. Adicionar `sanitize` (strip oculto) em `IngestLearningBlock` e expor `--strip-hidden`. Conecta `SiteToMarkdownSpider` ao pipeline `knowledge`.

## RELAÇÃO 4 — LLM-directed scraping (ai.py vs internal/providers)

- **Fronteira:** `scrapling/core/ai.py` (MCP tools dirigidas por LLM) ↔ `internal/providers` + `internal/chat/tool`.
- **Como se encaixa hoje:** o Cosca **tem a metade LLM** (`internal/providers/` com openai, anthropic, bedrock, azure, google, groq, deepseek, mistral, ollama, local, openaicompat + circuitbreaker/ratelimit/transport) e **a metade tool** (`internal/chat/tool`).
- **O que falta (gap):** o `ai.py` expõe o "input dirigido" que um agente precisa: `make_request`/`fetch`/`stealthy_fetch`/`session_fetch`/`screenshot` com `extraction_type="markdown|html|text"`, `css_selector`, `main_content_only`, sessões persistentes com session_id, `cdp_url`, auth por bearer, redirects SSRF-safe, e **strip de prompt-injection** antes do modelo ver. **A única tool web do Cosca é um stub.** A extração do scrapling é determinística (sem LLM no loop) — o LLM só decide o que buscar — o que casa perfeitamente com `internal/grounding` (zero-LLM), mas o Cosca não tem a "ponte MCP".
- **Valor:** 💎💎
- **Proposta concreta:** expor o **servidor MCP do scrapling como fonte de tools** no Cosca. Opções: (a) client MCP nativo em Go em `internal/chat/tool/scrapling.go` (HTTP + token `SCRAPLING_MCP_AUTH_TOKEN`); (b) empacotar como **plugin WASM** no runtime de plugins 14120. Tool `acquire(url, extraction_type, css_selector, main_only)` roteada via `internal/providers`, com execução determinística no scrapling. `--ai-targeted` como default para salvar tokens.

## RELAÇÃO 5 — Catálogo de capacidades (onde o Cosca está cego)

- **Fronteira:** `public-apis` (catálogo de fontes do mundo real) vs `CAPABILITY_CATALOG.md` + `internal/skills/catalog.go`.
- **Como se encaixa hoje:** `CAPABILITY_CATALOG.md` mapeia **64 capacidades internas** (CAP-ARCH, CAP-ENG, CAP-AI-002 RAG, CAP-AI-003 Embeddings, CAP-INT-001 Third-Party API Integration). `internal/skills/catalog.go` busca skills via GitHub `topic:skills`. `knowledge.PackageStore` é para repos de documentação GitHub. Nenhum descobrindo **fontes de dados do mundo** (weather, câmbio, geolocalização, notícias...).
- **O que falta (gap):** **nenhuma capacidade de "World Data-Source Discovery"**. O Cosca está **cego ao que existe no mundo**. O public-apis é exatamente isso (~2100 linhas, 52 categorias, tabelas com `Auth/HTTPS/CORS`). E o Cosca **já tem o parser** para importá-lo: `internal/markdown.Parse` extrai `Table{Headers,Rows}` — dá para ingerir o README do public-apis direto no knowledge DB.
- **Valor:** 💎💎💎 (a porta de entrada da percepção de mundo).
- **Proposta concreta:** nova capacidade `DataSourceRegistry` (em `internal/knowledge`/novo `internal/sources`):
  1. `cosca sources add --from public-apis` — usa `markdown.Parse` para importar as tabelas; cada `| Name | Description | Auth | HTTPS | CORS |` vira `DataSource{ID, Category, Auth, HTTPS, CORS, URL}` persistida com `ProvenanceUntrusted`.
  2. Liga cada fonte a um **acquirer**: `scrapling` (web/HTML — stealth), `acquisition.Client` (GET JSON), OAuth/apiKey (via `internal/providers`/secrets).
  3. Aquisição disparada por **intenção/percepção** (ex.: "o que está no céu?") → resolve `DataSource` por categoria → `acquire()` → grounding. Registrar como `CAP-SRC-001/002` no catálogo (regra: "nenhuma responsabilidade fora do catálogo").

---

## AS RELAÇÕES MAIS VALIOSAS (Top 3)

1. **R2 — Fonte→Aquisição (💎💎💎):** o elo de percepção. `web_fetch.go` é stub; `acquisition.Client` não converte HTML→md nem faz stealth; `acquire.go` preso a GitHub README. Implementar via scrapling CLI/MCP através do gate `--allow-remote` + proveniência UNTRUSTED resolve a percepção hoje.
2. **R5 — Catálogo de capacidades (💎💎💎):** o Cosca está **cego à existência do mundo**. public-apis é o índice; um `DataSourceRegistry` ingerido pelo `markdown.Parse` é a porta de entrada do "perceber um mundo".
3. **R3 — Aquisição→Grounding (💎💎):** o funil de fidelidade já existe (`internal/grounding` zero-LLM), mas falta o **chunker/conversor HTML→markdown→SourceChunk** e a **sanitização anti-conteúdo-oculto** a montante.

---

## DIAGRAMA DO CAMINHO PONTA-A-PONTA

```
public-apis (ONDE está a fonte)          Scrapling (COMO pegar)                 COSCA (Mundo)
──────────────────────────────────      ────────────────────────────            ───────────────────────────────
[catálogo: 52 categorias,               [Fetchers: static/dynamic/stealth]      ① internal/acquisition
 Auth/HTTPS/CORS, tables, ~2100 linhas]  [Spiders: Crawl/Sitemap/Shopify/          · client.FetchAll    [TÊM]
      │                                     SiteToMarkdownSpider]                   · SSRF guard        [TÊM]
      │  R5: markdown.Parse importa     [page.markdown(main_content_only,          · proveniência UNTRUSTED [TÊM]
      ▼    as tables do README              css_selector)]  HTML→Markdown            · orçamento/tracker  [TÊM]
 [DataSourceRegistry: ID/Categoria/      [adaptive: save/retrieve/find_similar]     · body HTML→markdown  [GAP]
  Auth/HTTPS/CORS/URL]  ──[GAP]──►     [MCP server ai.py:                          → render/stealth/proxy [GAP]
      |                                    extraction_type, css_selector,         ②  internal/markdown
      | R4: LLM decide o que buscar       sessions, screenshot, auth, SSRF-safe]      · Parser (.md→AST)  [TÊM]
      └──────────S1──┐                         │                                       · HTML→MD converter [GAP]
                     │ R2: acquire via        │ R2: wire web_fetch tool / plugin     ②b internal/knowledge
                     │   scrapling CLI/MCP    │                                        · AcquirePackage    [TÊM]
                     └──────────►┌─────────┴──────────┐                                  · IngestLearningBlock [TÊM]
                  acq via AllowRemote + UNTRUSTED     │ R3: chunker → SourceChunk          · dedup SHA256      [TÊM]
                                        │             │    (DocumentID=url, ChunkID) [GAP]· anti-hidden-strip [GAP]
                                        ▼             ▼                                        │
                                  [chunks texto limpo]──►─ ③ internal/grounding ④ internal/providers
                                                S3: fidelity gate [TÊM]          (LLM dirige, extração é det)
                                                ExtractClaims + VerifyClaim       OpenAI/Anthropic/Ollama/Bedrock…
                                                (token+bigram+number, zero-LLM)   circuit breaker/ratelimit [TÊM]
                                                          │                                │
                                                          ▼                                ▼
                                                  [veredito suportado/fiel] →  knowledge.db  →  Mundo Unreal
                                                   (SourceChunk↔[K-xxxx])           FTS5/VECTOR   (world/adapter/unreal)
                                                                                    (epistemic/provenance) [TÊM]
```

Legenda: `[TÊM]` = já existe no Cosca · `[GAP]` = elo faltante (prioridades: R2, R5, R3).

## ⚠️ CONSIDERAÇÃO DE CONFORMIDADE

Ao implementar o R2, adotar o **acquirer respeitoso** do scrapling: `robots_txt_obey` real, AutoThrottle de polidez (nunca acelerar após block), e a camada stealth/captcha-solver como **opt-in auditado**, não default. O `--ai-targeted`/sanitização anti-prompt-injection é seguro e deve ser default. Evitar o `solve_cloudflare` automático (circumvention de proteção) — em vez disso, usar o classificador de "fonte bloqueada" para marcar `trust=low` e não positivar dados de challenge não resolvido.
