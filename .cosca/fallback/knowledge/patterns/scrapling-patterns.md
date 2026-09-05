# Scrapling Patterns — Web Scraping Adaptativo

> **Category**: Agent/Mining | **Repo**: `D4Vinci/Scrapling` (76k★, v0.4.15) | **Owner**: Cosca Kernel | **Date**: 2026-08-27
> **Fonte**: mineração profunda em 4 filões (engines adaptativos, stealth/fingerprint, spiders/crawl, AI-core + agent-skill).

## Resumo do que é

Scrapling é um framework Python de scraping **adaptativo**: uma única API vai de "uma request" até "crawl em escala total", passando por browser dinâmico e stealth anti-bot. A ideia central: **extração como política executável por agente** (o LLM decide o que pegar; a lib resolve o formato e a sanitização de forma determinística).

---

### [Padrão] Engine por Composição de Mixins ("Capabilities via Mixins")
- **O que resolve:** um único tipo de fetcher que opera como HTTP estático OU navegador dinâmico sem duplicar config.
- **Como funciona:** hierarquia por mixin+mixin-base, não herança profunda. `static.py` tem `_ConfigurationLogic(ABC)` como base de defaults (impersonate, stealth, proxies, timeout, retries, headers, redirects, cert, http3) → herdam `_SyncSessionLogic`/`_ASyncSessionLogic`. Browser: `SyncSession`/`AsyncSession` (pool de páginas + Playwright lifecycle); `BaseSessionMixin.__validate_routine__` injeta `color_scheme=dark, device_scale_factor=2`; `StealthySessionMixin` empilha flags anti-detecção. Montagem final: `class DynamicSession(SyncSession, DynamicSessionMixin)`. A escolha estático vs dinâmico **não é automática** — é a classe do fetcher que você instancia.
- **Nível:** 💎💎
- **Aplicação no Cosca:** mapear para os providers: cada provider (HTTP/browser/stealth) = `CapabilitiesMixin` + `EngineBase`. Em Go: `interface embedding + struct composition`. O conflito mutuamente exclusivo `proxy_rotator × proxy` é validação de capacidade a replicar.

### [Padrão] Objeto de Resposta Único: `Response(Selector)`
- **O que resolve:** apagar a diferença entre request `curl_cffi`, página Playwright e XHR capturado, entregando sempre o mesmo objeto consultável.
- **Como funciona:** `class Response(Selector)` (`engines/toolbelt/custom.py:28`) herda do parser e acrescenta metadados de rede: `status`, `reason`, `cookies`, `headers`, `request_headers`, `history`, `meta`, `request`, `captured_xhr`. Expõe `markdown()`, `follow()` (cria Request filho propagando referer + session_kwargs). `ResponseFactory` (`convertor.py:17`) normaliza a partir de http/playwright/async-playwright, resolvendo encoding via regex `charset=`, reconstruindo redirects e anexando XHR. Conteúdo semântico e transport metadata coexistem.
- **Nível:** 💎💎💎
- **Aplicação no Cosca:** contrato único "Documento" onde conteúdo + proveniência (status/headers/url/history/meta) vivem no mesmo objeto serializável — carrier perfeito para o grounding/RAG com fidelidade verificada. `follow()` é modelo de continuidade de contexto entre requests de um mesmo agente no mundo.

### [Padrão] Ferramenta Adaptativa: `save / retrieve / relocate / find_similar`
- **O que resolve:** extração que sobrevive a mudanças de layout — elemento achado por similaridade estrutural aprendida, não caminho fixo.
- **Como funciona:** 3 regimes. **1) Aprendizado**: `css(q, auto_save=True)` persiste o elemento via `save(element, identifier)`; fingerprint `{tag, attributes, text, path, parent_*, siblings, children}`. **2) Relocalização**: quando xpath/css retorna vazio, `retrieve(identifier)` + `relocate(element_data, percentage)` varre todos os nós, pontua com `__calculate_similarity_score` e devolve os ≥ `percentage` (default 40). **3) Generalização**: `find_similar(similarity_threshold, ignore_attributes, match_text)` acha "irmãos" via XPath + filtro `__are_alike`. Scoring: média ponderada + `SequenceMatcher.ratio()` sobre tag/texto/atributos + links de identidade (`class/id/href/src`, únicos que sobrevivem a mudança). Storage plugável: `SQLiteStorageSystem` com WAL + RLock, particionado por `url→base_domain`, `INSERT OR REPLACE ... UNIQUE(url, identifier)`, identifiers com sha256+length.
- **Nível:** 💎💎💎
- **Aplicação no Cosca:** maior diamante para grounding/RAG resiliente. O mundo (Unreal) muda — caminhos fixos envelhecem. Guardar fingerprints de entidades e **relocalizar** em vez de re-escrever o extrator. A noção de score + threshold dá fidelidade verificada (retorna score, não certeza cega). `SQLiteStorageSystem` é blueprint do semantic memory indexado por entidade. `find_similar` acha "objetos do mesmo tipo" num mundo.

### [Padrão] Máquina de Estados de Página: `PageInfo` / `PagePool`
- **O que resolve:** compartilhar páginas de browser numa sessão com limite de concurrency e reciclagem segura.
- **Como funciona:** `PageInfo(Generic)` é dataclass com `page`, `state: Literal["ready","busy","error"]`, `url` e métodos `mark_busy/mark_ready/mark_error` (estado no agregado). `PagePool` com `max_pages`, RLock. `_page_generator` recicla em `finally` (marca `ready`) ou remove se `error/is_closed`. Modo rotação cria contexto novo por request (isolamento de fingerprint por proxy); modo normal usa `launch_persistent_context`. Async adiciona backpressure sob `asyncio.Lock` com polling 50ms e `TimeoutError` a 60s. Telemetria: `get_pool_stats()` → `{total_pages, busy_pages, max_pages}`.
- **Nível:** 💎💎
- **Aplicação no Cosca:** resource pool de providers/engines sobre mundo com recursos limitados — limite de concurrency, reciclar vs descartar em erro, métricas `busy/total`. Lógica isolation-by-cost (contexto novo por proxy) para credenciais sensíveis por job.

### [Padrão] Sessão Fábrica com Merge Diferido de Config
- **O que resolve:** uma classe produz sessão sync e async, e uma API one-off (`Fetcher`) sem exigir `with`, tudo com a mesma config.
- **Como funciona:** `FetcherSession` (`static.py:629`) guarda só defaults (via `__slots__`, sem socket). No `__enter__` monta config derivando slots `{k.replace("_default_",""): getattr(self,k)}`, instancia `_SyncSessionLogic`/`_ASyncSessionLogic`. Reentrada proibida. `FetcherClient` (one-off) anula `__enter__/__exit__` (None) e usa sentinela `_NO_SESSION` — "um request só" sem sessão persistente. `_merge_request_args` faz merge por prioridade (defaults ← call kwargs) e remove chaves internas (`retries`, `selector_config`, `http3`). `configure()` seta defaults de classe.
- **Nível:** 💎💎
- **Aplicação no Cosca:** "uma request" vs "uma sessão" sem duplicar API. `FetcherSession` = session factory que resolve sync/async. `FetcherClient` = caminho barato p/ health-checks/discovery. O merge por prioridade é o que os providers do Cosca precisam (defaults globais do mundo, overrides por função).

### [Padrão] Validação de Config com `msgspec.Struct` + Cache de Defaults
- **O que resolve:** validar dezenas de params de engine em hot path sem custo alto.
- **Como funciona:** `PlaywrightConfig`/`StealthConfig` são `msgspec.Struct(kw_only=True)` com constraints anotadas (`Annotated[int, Meta(ge=1, le=50)]`). `__post_init__` valida invariantes (proxy×rotator exclusivos, paths absolutos, `solve_cloudflare` força timeout≥60s). Otimização: `_filter_defaults` compara kwargs contra cache de defaults por modelo (`models_default_values`, lazy) e descarta o que já é default antes de `convert`. `validate_fetch` mergea e só reescreve os campos realmente overrideados.
- **Nível:** 💎💎
- **Aplicação no Cosca:** struct imutável de config com constraints + cache de defaults para pular validação redundante + merge em 2 níveis. Em Go: `struct` com tags + `Validate()` sobre campos != zero + `map[type]defaults` cacheado. Antídoto para config explodida de providers/engine/WASM.

### [Padrão] Roteamento de Sessões por ID + `SessionManager`
- **O que resolve:** um único crawl misturar requests HTTP baratos e browsers stealthy, roteando cada URL por ID com lazy start.
- **Como funciona:** `Request(sid, callback, _session_kwargs)`. `SessionManager` registra `dict[str, Session]` com `add(id, session, *, default, lazy)`; sessões lazy só iniciam no primeiro uso (com Lock). `fetch()` resolve `sid` ou default e faz merge de meta. `response.request` e `response.meta` amarrados para proveniência.
- **Nível:** 💎💎
- **Aplicação no Cosca:** um agente que mistura canais de ingestão: `add("http", httpSess)` + `add("browser", stealthSess, lazy=True)`. O lazy start evita pagar custo de browser sem precisar. Proveniência por-agente.

### [Padrão] Gradação de Escala com Backpressure Adaptativa (fingerprint-dedup + AutoThrottle + checkpoints)
- **O que resolve:** continuum "single request → full-scale crawl" com mesma semântica de retry/proxy/parse.
- **Como funciona:** 1) One-off (`Fetcher.get`); 2) Sessão (pool/cookies persistentes); 3) Multi-sessão roteada; 4) Crawl concorrente com `concurrent_requests`, fila por prioridade, **dedup por fingerprint SHA-1** de `{sid, body, method, canonical_url}` e flags `dont_filter/include_headers/include_kwargs`; 5) **AutoThrottle** — `target_delay = latency/target_concurrency`, `new_delay = max((current+target)/2, target)`; em block, `new_delay = min(max(new, floor), max_delay)` garante "um block nunca acelera", punindo com `Retry-After` (parseado, segundos ou data HTTP) ou `current*2`; 6) **Checkpoint atômico** (`.tmp → replace`) para pause/resume.
- **Nível:** 💎💎💎
- **Aplicação no Cosca:** blueprint do orchestration/ingest em escala. 3 peças absorvíveis: (1) fingerprint de request com canonicalize+sha1 no dedup/entidade; (2) AutoThrottle per-domain com "block nunca acelera" + Retry-After; (3) checkpoints atômicos `.tmp→replace` para runs longas do mundo (re-scan, re-grounding).

### [Padrão] Pluggable Strategy & Fingerprint de Rede (rotadores, headers, anti-detecção)
- **O que resolve:** trocar comportamento de rede (proxy, headers "reais", bloqueio) sem tocar no engine.
- **Como funciona:** `ProxyRotator` thread-safe com estratégia plugável `RotationStrategy = Callable[[List[ProxyType], int], Tuple[ProxyType, int]]` (default `cyclic_rotation`). `is_proxy_error` detecta assinaturas multi-engine (curl E Playwright). `generate_headers(browser_mode)` usa browserforge e **alinha a versão do Chrome no UA com a versão real do Chromium que o driver usa** (`driven_browser_version`) — evita mismatch UA vs TLS fingerprint (maior detector de bot). `create_intercept_handler` aborta recursos na lista `EXTRA_RESOURCES` ou domínios bloqueados, com `_is_domain_blocked` buscando por sufixo O(1) em frozenset; `block_ads` injeta AD_DOMAINS (~3500).
- **Nível:** 💎💎
- **Aplicação no Cosca:** `ProxyRotator` (estratégia injetável, thread-safe, chave server|user) vira o proxy pool do Cosca. O alinhamento UA↔driver é o detalhe fino para não quebrar fidelidade de acesso. O bloqueio por sufixo de domínio é ideal para ingestão higienizada — evita ruído e prompt-injection de recursos de terceiros.

### [Padrão] Camaleão de Fingerprint de Navegador (Patchright + flags)
- **O que resolve:** fazer Chromium headless parecer humano para WAFs (Cloudflare, Akamai, DataDome).
- **Como funciona:** estratos integrados. **1)** `patchright` (fork do Playwright que remove leaks de automação via CDP — espinha dorsal). **2)** `STEALTH_ARGS`: `--disable-blink-features=AutomationControlled`, `--start-maximized`+`--window-position=0,0`, `--blink-settings=primaryHoverType=2...` (finge hover/touch), etc.; `HARMFUL_ARGS` (`--enable-automation`, `--disable-extensions`) são **removidos** via `ignore_default_args`. **3)** UA coerente via browserforge + `driven_browser_version()`. **4)** Contexto: `color_scheme=dark`, `device_scale_factor=2`, screen 1920×1080, permissions, `is_mobile=False`. **5)** locale forçado por flag de launch (`--lang`) porque `context.locale` só patchia a main-thread — Web Workers mantêm o idioma real e o Cloudflare flagra o mismatch.
- **Nível:** 💎💎💎
- **Aplicação no Cosca:** "identidade coerente" não é um item, é uma matriz de camadas (motor→flags→UA→JS→workers→locale). Modelar como um "fingerprint contract" do collector. **Limite honesto:** fingerprint não é mágico; e mascarar identidade para evasão é zona cinzenta de compliance.

### [Padrão] Detector + Solver de Cloudflare como Classificador de Qualidade
- **O que resolve:** passar por páginas protegidas detectando o tipo de challenge. *(uso defensável: classificar qualidade de fonte, não resolver captcha)*
- **Como funciona:** `_detect_cloudflare(page_content)` procura literais `cType: 'non-interactive'|'managed'|'interactive'` e detecta turnstile embutido. `_challenge_cleared()` re-analisa. O solver é heurístico (clica com jitter humano, `randint(26,28)`, delay 100-200ms), loopa até 3 tentativas.
- **Nível:** 💎💎 (valor real)
- **Aplicação no Cosca:** o padrão **detect → cleared** é um classificador determinístico de "gate de qualificação": marcar `trust=low` e não positivar dados coletados de challenge não resolvido. Absorver como *signal de quality gate* para o grounding, **não** como solver automático de CAPTCHA.

### [Padrão] Spiders / Crawl em Escala Confiável
- **O que resolve:** orquestrar milhares de requests concorrentes com politeness, retomada e dedup.
- **Como funciona:** `CrawlerEngine` roda loop assíncrono com `CapacityLimiter` global + `_domain_limiters`. **Fingerprint SHA1 idempotente** (`update_fingerprint`): `sha1(orjson.dumps({sid, body, method, canonicalize_url(url)}, OPT_SORT_KEYS), usedforsecurity=False)` cacheado. **Scheduler** de prioridade com tie-break monotônico + espelho `_pending`/`_inflight` p/ snapshot/restore. **Checkpoint/retomada** serializa `CheckpointData{requests, seen}` atomicamente (`.tmp→replace`), reidrata callbacks por **nome** (`_callback_name`) já que bound methods não são pickláveis, e em resume **pula** `start_requests()`. **Cache replayable** chaveado por fingerprint (JSON + body base64) p/ determinismo de dev. **AutoThrottle + Crawl-delay/Request-rate** por domínio. **robots.txt** via Protego (`can_fetch`, `get_delay_directives`), com `robots_disallowed_count` como métrica de governança. **Templates prontos**: `CrawlSpider` (rules), `SitemapSpider` (gzip com cap 64MiB anti-bomb), `XMLFeedSpider`/`CSVFeedSpider`, `ShopifySpider` (collections/products.json + dedup por variant id). **SiteToMarkdownSpider**: converte site inteiro em `.md` limpo + `docs.jsonl`.
- **Nível:** 💎💎💎
- **Aplicação no Cosca:** núcleo de ingestão auditável. Shallow `internal/ingest`: dedup por fingerprint idempotente, checkpoint/cursor no `durable` (retomável sem duplicata), cache replayable (determinismo de dev/debug/auditoria), AutoThrottle (polidez autônoma), robots.txt como gate de governança, templates por plataforma.

### [Padrão] AI-Sanitized-Markdown-Ingestion (anti prompt-injection)
- **O que resolve:** transformar página → Markdown limpo, removendo conteúdo `hidden` usado para injetar instruções no LLM, com custo zero de tokens.
- **Como funciona:** `Convertor._sanitize_for_ai` (`shell.py:604-619`): `_HIDDEN_XPATH` remove `display:none|visibility:hidden|opacity:0|font-size:0|height/width:0`, `aria-hidden='true'`, `<template>`; `_ZWC_PATTERN` (zero-width) e `_CONTROL_CHARS_PATTERN` limpam texto/tail; `keep_comments=False`. `_strip_noise_tags`: `script/style/noscript/svg`. `_extract_content`: `main_content_only` corta no `<body>`, `css_selector` estreita. `Response.markdown()` reusa os mesmos passos.
- **Nível:** 💎💎💎
- **Aplicação no Cosca:** contrato `sanitize-content` (remover oculto + estreitar + converter em md). Crítico para qualquer RAG/semantic-memory/embeddings — o texto chega limpo sem custo de LLM na ingestão. Alinhado ao pilar anti-prompt-injection.

### [Padrão] MCP-Tool-Surface (desacoplar lib core de tool surface)
- **O que resolve:** expor uma lib inteira como ferramentas MCP consumíveis por qualquer agente, com sessão persistente e auth.
- **Como funciona:** `ScraplingMCPServer` (`ai.py:182`) expõe 13 tools (`make_request`, `bulk_get`, `fetch`, `stealthy_fetch`, `open_session`, `session_fetch`, `close_session`, `screenshot`...). Modelos pydantic (`ResponseModel{status, content, url}`, `SessionInfo`). `_translate_response` usa `Convertor._extract_content` + limpa CONTROL_CHARS. `_session_settings` devolve os settings efetivos da sessão "para o agente". Transportes stdio e Streamable HTTP (com auth Bearer token, DNS-rebinding protection). **Não chama LLM nenhum** — é o oposto: expõe a capacidade de scraping como ferramentas para um LLM consumir.
- **Nível:** 💎
- **Aplicação no Cosca:** referência de como expor a capacidade do Cosca (hooks, search, memory) como ferramentas MCP com models versionados, sessões reutilizáveis e auth por token. A lição: **desacoplar tool-surface ↔ lib core**. `_session_settings` (tool devolve settings efetivos) é um padrão raro que o Cosca deve replicar.

### [Padrão] AgentSkill-SKILL.md (formato de skill portátil — o maior diamante)
- **O que resolve:** unidade portátil de conhecimento que um agente carrega, com ativação por intenção e progressive disclosure.
- **Como funciona** (`agent-skill/Scrapling-Skill/SKILL.md:1-416`): **frontmatter YAML** com `name`, `description` (frase longa terminando em gatilhos "**Use when**..."), `version` (sincronizada com a lib), `license`, `metadata.{homepage, openclaw.requires.bins: [python3], anyBins: [pip, pip3]}` (declaração de pré-requisitos de bins). **Corpo progressivo**: intro, Requires, bloco `> Notes for AI scanners`, `IMPORTANT` sobre prompt-injection (`--ai-targeted`), `## Setup`, `## CLI Usage`, `## Code overview`, `## References` (lista de caminhos relativos carregados quando precisar), `## Guardrails (Always)`. Estrutura multi-arquivo: `references/{fetching,parsing,spiders,integrations,mcp-server,building-rag-systems}.md` + `examples/01-04`.
- **Nível:** 💎💎💎
- **Aplicação no Cosca:** o Cosca usa formato diferente (`INDEX.md` catálogo + `NAME.md` por tópico com metadados em markdown, sem frontmatter `description`-disparo). Para absorver: (a) frontmatter com `description`-disparo para o motor de skills indexar/ativar por intenção (não por catálogo manual); (b) unidades portáteis (`SKILL.md` + `references/` + `examples/` + LICENSE) versionáveis; (c) `metadata.openclaw.requires.bins` como declaração de pré-requisitos checável no bootstrap; (d) `version` sincronizada. Resolve o problema real de contexto e onboarding do agente.

### [Padrão] Escalation-Ladder de Extração (decisão do agente)
- **O que resolve:** regra determinística de qual fetcher usar, evitando gastar tokens em browser quando HTTP resolve.
- **Como funciona:** `get` (simples) → `fetch` (dynamic/SPA) → `stealthy-fetch` (Cloudflare/anti-bot), com a nota "speed is nearly the same". No MCP: `make_request → fetch → stealthy_fetch`, mais `bulk_*` e sessões para múltiplas páginas.
- **Nível:** 💎💎
- **Aplicação no Cosca:** mapear para workflows/routers de capacidade (ex. `cosca-discovery` escolhe fetch estático vs dinâmico vs stealth), e expor como tabela de "guia de seleção de ferramenta" no SKILL.md/prompt do agente — estabiliza custo (tokens/recursos) e a rota.

---

## 💎 DIAMANTES (para o Cosca)

1. **SKILL.md portátil + progressive disclosure** — copiar integralmente: frontmatter `description:"Use when..."` + `references/` sob demanda + `examples/` + `requires.bins`. Transforma conhecimento de pasta em unidade carregável que o motor de skills do Cosca ativa por intenção.
2. **Ferramenta Adaptativa (`save/retrieve/relocate` + score)** — grounding/RAG resiliente: quando o mundo muda, **reloca** em vez de re-escrever; **nunca afirma certeza sem score**. É a diferença entre "scraping" e "grounding resiliente".
3. **AI-Sanitized-Markdown (anti prompt-injection)** — contrato determinístico `sanitize-content`: remove oculto/zero-width/scripts → LLM recebe texto limpo sem custo de modelo. Todo pipeline RAG do Cosca que comer web fica mais barato e mais seguro.
4. **Gradação de Escala com Backpressure** — fingerprint-dedup determinístico + AutoThrottle per-domain ("block nunca acelera") + checkpoints atômicos. É o blueprint do orchestration/ingest em escala confiável e auditável.
5. **Objeto `Response(Selector)` unificado** — conteúdo semântico + proveniência (status/headers/history/meta) no mesmo objeto. Alimenta o RAG com fidelidade verificada e a trilha de proveniência.
6. **MCP-Tool-Surface desacoplada** — `ai.py` não chama LLM, só expõe a capacidade como ferramentas. Modelo para expor o Cosca (search/memory/hooks) como MCP consumível.

## ⚠️ LIMITES / CONFORMIDADE

- O repo declara (README, ~10 línguas): "apenas para fins educacionais/pesquisa — cumpra leis locais de scraping/privacy, respeite ToS e robots.txt". As camadas Spider têm `robots_txt_obey` real e `SKILL.md` diz "não burle paywalls ou autenticação sem permissão".
- **Ponto vermelho para o Cosca (com governance):** o `solve_cloudflare=True` resolve **CAPTCHA automaticamente** = circumvention de medida de proteção, quase sempre viola ToS e fragiliza LGPD/GDPR. Mesmo mascarar fingerprint para evasão é zona ambígua.
- **Recomendação:** absorva os padrões **neutros** (pool de sessão com reset, normalização/sanitização p/ RAG, classificação de "fonte bloqueada", respeito a robots.txt, rotação como *resilience* com proxy industrial/público não residencial) e **não** o auto-camaleão/captcha-solver como default. Coleta de dados públicos defensável = robots respeitado + rate-limit politeness + rota que exija evasão como **opt-in auditado**, não default.
