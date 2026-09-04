# Relatório de Fechamento — Esteira de Mineração "Osiris" (implementação validada)

> **Natureza:** documento de fechamento da frente de mineração → implementação → validação dos padrões do repositório `github.com/simplifaisoul/osiris` (licença MIT) no COSCA.
> **Autor:** cosca-architecture (Architecture Chief)
> **Data:** 2026-09-04
> **Escopo:** consolidação de evidências. **Nenhum arquivo `.go` foi alterado por este relatório.** Documentação apenas.
> **Commit cristalizado:** `431199679290f2546c0a47e5602716960daccf33` — mensagem *"feat(kernel): implementa primitivas validadas derivadas de Osiris (evidence/singleflight/ssrf/conditional/concurrency)"*.
> **Estado da frente:** código validado; working tree **limpa** no que respeita à esteira; pronta para revisão do professor.

---

## 1. Resumo executivo

A esteira de mineração dos padrões do repositório **Osiris** (`github.com/simplifaisoul/osiris`, MIT) foi **fechada em três atos**:

1. **Mineração** (documentação) — ver `docs/reports/mineracao-osiris-padroes-2026-09-04.md`: mapa de veredito por categoria (ADOTADO / AUMENTADO / AVALIAR DEPOIS / REJEITADO), com proveniência por arquivo-fonte e motivo arquitetural.
2. **Formalização em ADR** — seis ADRs emitidos na frente (`ADR-037`…`ADR-042`), consolidando **doutrina canônica** (e, para as 5 primitivas, apontando para a fonte de código quando ela existisse).
3. **Implementação validada** — as **5 primitivas** (ADR-037/039/041/042/040) foram **reimplementadas em Go idiomático** sob os contratos do COSCA, **testadas** (incl. `-race`) e **cristalizadas** no commit `431199679290f2546c0a47e5602716960daccf33`. O `ADR-038` é **doutrina** (documento), não primitiva — ver §3.6.

**Frase-síntese da esteira:** *minerar ideias, não importar código; reimplementar sob os contratos do COSCA.* Nenhuma linha do Osiris foi importada — apenas **princípios** extraídos e expressos como primitivas Go idiomáticas, stdlib-only.

**Estado final:** implementação validada (testes + race limpos, benchmark medido, before×after medido), working tree da esteira limpa, pronta para a **revisão do professor**. O que **não** foi feito (wiring a consumidores, migração de call-sites) está **explícito** na §4 — é o coração deste relatório.

---

## 2. Mapa da esteira (tabela)

| ADR | Primitiva | Pacote | Consumidor integrado | Status (implementado / testado / medido) |
|---|---|---|---|---|
| **ADR-037** | `ObservationState` (6 estados epistêmicos) + `Observation` + `Calibrate` | `internal/evidence` | **NENHUM** (primitiva sem consumidor — ver §4.1) | Implementado · testado · lógica medida (doutrina) |
| **ADR-038** | Doutrina do veredito como estado epistêmico (truth-of-world × observation-of-instrument) | — (doutrina) | Consumido por `architecture + review/critic` (núcleo) | **Documento (não primitiva)** — nenhum `.go` |
| **ADR-039** | `CachedSource[T]` (single-flight + TTL + stale-on-error + mapa com teto) | `internal/cache` | **NENHUM** (sem consumidor — ver §4.2) | Implementado · testado · **medido (benchmark ~47 ns/op)** |
| **ADR-040** | `Policy` / `Limiter` (global + per-provider + rate + burst + backoff) | `internal/concurrency` | **Integrado** em `internal/orchestration/semantic_router.go` (drop-in, `MaxGlobal: 5`) | Implementado · testado · **medido (before×after 4→0 erros)** |
| **ADR-041** | Guard anti-SSRF (`CheckHostname`, `ValidatePublicIP`, `NewSafeClient`, `IsPublicIP`, `DecodeEmbeddedIPv4`) | `internal/security` | **Integrado** em `internal/mcpserver/tools.go` (`handleWeb` / `cosca.web`) | Implementado · testado · lógica média coberta |
| **ADR-042** | `FetchConditional` (ETag/Last-Modified/304) + `Optional[T]` | `internal/acquisition` | **NENHUM** (sem wiring a poller — ver §4.4) | Implementado · testado · lógica média coberta |

> **Leitura da tabela:** 3 das 5 primitivas (037/039/042) estão **implementadas, testadas e medidas**, mas **sem consumidor ligado**; 2 (040/041) já têm **consumidor integrado** (040 de forma **parcial/opt-in**, 041 de forma **completa no `cosca.web` mais pendência deliberada no `acquisition`**). Cada pendência é detalhada na §4.

---

## 3. Evidências por ADR (implementado + testado + medido + gate)

> Para cada uma das **5 primitivas implementadas** (no mesmo bloco) e, ao final, a **observação sobre o ADR-038**. Em cada caso: o que entrou (tipo/assinatura), testes (`go test` e `-race`), medição (números), Golden Gate / PromotionGate e proveniência.

### 3.1 ADR-037 — `ObservationState` (Evidence Layer / primitiva `Verdict`)

**O que entrou** (`internal/evidence/state.go`):

- `type ObservationState string` — primitiva de **6 estados**: `found`, `not_found`, `blocked`, `inconclusive`, `skipped`, `error` (o `Verdict` formalizado pela doutrina ADR-037/ADR-038).
- Métodos que **carregam o lema central** do ADR em código:
  - `IsAbsence() bool` → `true` **só** para `not_found` (**ausência provada**); `blocked`/`inconclusive` **nunca** são ausência.
  - `IsUncertain() bool` → `true` para `blocked`/`inconclusive` (incognoscível agora).
  - `IsConfident() bool` → `true` para `found`/`not_found` (observação confiável).
  - `IsKnown() bool` → `true` quando o estado do mundo foi estabelecido.
- `type Observation struct { State ObservationState; Reason string; Evidence string }` — carrega **motivo explícito** (nunca implícito).
- `func (o Observation) Validate() error` — estado canônico; `ObservationError` **exige** `Reason` (não-engolimento implícito).
- `func Calibrate(primary ObservationState, controlsReported int) ObservationState` — aplica o **princípio dos DOIS controles independentes**: qualquer controle aleatório "encontrado" → `found` vira `inconclusive` (soft-404).

**Testes** (`internal/evidence/state_test.go`, 7 testes):

- `TestObservationStatesCanonical`, `TestIsAbsence_OnlyProvenAbsence` (lema central), `TestIsUncertain`, `TestIsConfident`, `TestObservation_Validate`, `TestCalibrate_TwoControls`, `TestNeverCollapseBlockedToAbsence`.
- Resultado: **PASS** ( `go test ./internal/evidence/...` → `ok`; `go test -race ./internal/evidence/…` → `ok`).

**Medição (lógica):** sem benchmark de tempo (não é caminho quente); a "medição" é a **prova da doutrina** no lema central — o teste `TestIsAbsence_OnlyProvenAbsence` fixa que **apenas** `not_found` é ausência, e `TestNeverCollapseBlockedToAbsence` garante o guarda-corpo (blocked ≠ ausência ≠ confiável). A **lógica** é validada por tabela (map de casos, determinístico).

**Golden Gate / PromotionGate:** **aprovado** (primitiva aditiva no pacote `internal/evidence`, zero quebra de contrato — `evidence` é pacote novo). Verifica o lema `blocked/inconclusive ≠ not_found` expresso em código, não só em doutrina.

**Proveniência:** princípio minerado de `osiris/src/lib/sherlock.ts` (MIT) — distinção estado-do-mundo × estado-da-observação, simetria observador-do-mundo ⇄ avaliador-do-modelo, calibração bifurcada por **dois controles independentes** (Roblox/WordPress). **Nenhum código/campo/estrutura do `sherlock.ts` importado** — a síntese em 6 estados é formalização COSCA.

---

### 3.2 ADR-039 — `CachedSource[T]` (cache de fonte resiliente)

**O que entrou** (`internal/cache/sourcecache.go`):

- `type SourceFetcher[T any] func(ctx context.Context, key string) (T, error)` — assinatura do fetcher preservada (drop-in).
- `type SourceResult[T any] { Value T; Stale bool }` — sinalizador `degraded/stale`.
- `type SourceConfig { TTL; RetryTTL; MaxEntries }` (defaults: 30min / 60s / 500).
- `type CachedSource[T any]` — concorrente-safe (`sync.Mutex`), com relógio injetável (`now` test seam).
- `func NewSourceCache[T any](fetcher SourceFetcher[T], cfg SourceConfig) *CachedSource[T]`.
- `func (s *CachedSource[T]) Load(ctx, key) (SourceResult[T], error)` — **single-flight** (N callers ao mesmo miss compartilham 1 fetch via `sourceCall` future), **TTL** (hit serve da memória sem tocar a fonte), **stale-on-error** (refresh falho → serve último dado bom + `stale=true`; sem valor anterior → erro real), **mapa com teto** (evicta a mais antiga; **nunca** evicta uma in-flight).
- `func (s *CachedSource[T]) Clear()`; `evictIfNeededLocked()`.

**Testes** (`internal/cache/sourcecache_test.go` — `BenchmarkSourceCache_ParallelHit` + 11 testes): `TestSourceCache_Hit`, `TestSourceCache_TTL`, `TestSourceCache_SingleFlight`, `TestSourceCache_SharedResult`, `TestSourceCache_ErrorNoPrior`, `TestSourceCache_ErrorWithStale`, `TestSourceCache_AfterExpirationGetsFresh`, `TestSourceCache_RaceSafety`, `TestSourceCache_EvictionOldest`, `TestSourceCache_EvictionSkipsInflight`, `TestSourceCache_Clear`.

- Resultado: **PASS** (`go test ./internal/cache/...` → `ok`; `go test -race ./internal/cache/…` → `ok`).

**Medição (benchmark):**

- `BenchmarkSourceCache_ParallelHit-16` — **hit-path sob paralelismo** (single-flight amortizado no hit; 0 B/op, 0 allocs/op).
- Número medido nesta máquina (AMD Ryzen 7 5700X3D): **~46.97 ns/op** (1ª corrida) / **~60.37 ns/op** (corrida com `-benchmem`), **0 allocs/op**. O valor oscila ~47–60 ns/op conforme carga do host; o número de referência citado na esteira (~47.79 ns/op) é a mesma métrica, hit-path sob paralelismo.

**Golden Gate / PromotionGate:** **aprovado** (pacote `internal/cache` recebeu a primitiva **aditiva**; `cache.Cache` — value-store — **não** foi alterado; ver §3.7 contratos preservados).

**Proveniência:** princípio minerado de `osiris/src/lib/sourceCache.ts` (MIT) — TTL + single-flight (N→1, nunca N requests na fonte) + stale-on-error (falha/vazio → último dado bom, não `null`) + mapa com teto (nunca evicta in-flight) + drop-in (preserva assinatura). **Nenhum código do `sourceCache.ts` importado.**

---

### 3.3 ADR-040 — `Policy` / `Limiter` (concorrência como propriedade do sistema)

**O que entrou** (`internal/concurrency/policy.go`):

- `type ProviderLimit { MaxConcurrency; RatePerWindow; Window; Burst }`.
- `type Backoff { Initial; Max }`.
- `type Policy { MaxGlobal int; PerProvider map[string]ProviderLimit; Backoff Backoff }` — **teto global** (borne do sistema), **tetos por provedor** (calibração por fonte), **rate/burst** (cadência), **backoff** (recuo exponencial, adapta, nunca evade).
- `type Limiter` com `Acquire(ctx, provider) error`, `Release(provider)`, `ReportRateLimited(provider)`, e `waitPolicy`/`reserve` (token bucket cancelável).
- `func NewLimiter(p Policy) (*Limiter, error)` — falha se `MaxGlobal <= 0` (sem constante mágica; quem usa declara o teto do sistema). `Acquire` rollback em erro (não vaza slot); ordem fixa global→provider (anti-deadlock).

**Consumidor integrado** (`internal/orchestration/semantic_router.go`):

- `limiter, _ := concurrency.NewLimiter(concurrency.Policy{MaxGlobal: 5})` — **drop-in**: `MaxGlobal=5` == comportamento anterior do semáforo cru; 0 regressão.
- `sr.limiter.Acquire(ctx, "embedding")` / `sr.limiter.Release("embedding")` — ligado ao caminho de **fallback individual de embeddings** (per-provider `"embedding"`, global 5). O single-flight de `refreshMu` já existia; a primitiva substitui o semáforo cru pela **policy**.

**Testes** (`internal/concurrency/policy_test.go`, 8 testes): `TestNewLimiter_RequiresMaxGlobal`, `TestMaxGlobal`, `TestPerProvider`, `TestRateLimitBurst`, `TestBackoff`, `TestCancel`, `TestNoDeadlock`, `TestUnderLoad`.

- Resultado: **PASS** (`go test ./internal/concurrency/...` → `ok`; `go test -race ./internal/concurrency/…` → `ok`).

**Medição (before × after, rate-limit):**

- `TestBeforeAfter_AdaptsToRateLimitedProvider` (`internal/concurrency/beforeafter_test.go`): provider que **rate-limita a 1 req / 20ms**; 5 requisições em paralelo.
  - **ANTES** (semáforo cru, cap 5): **4 erros** de rate-limit.
  - **DEPOIS** (policy com rate+backoff): **0 erros** de rate-limit.
  - Log confirmado: `ANTES (semáforo cru): erro de rate-limit = 4` / `DEPOIS (policy): erro de rate-limit = 0`. O sistema **ADApta** (rate/backoff) — **não evada** (princípio axial ADR-040).

**Golden Gate / PromotionGate:** **aprovado** (pacote `internal/concurrency` novo, aditivo; `semantic_router.go` mudou `25 +-` com drop-in de teto = 5, zero regressão). Verifica o princípio: concorrência é propriedade do sistema; rate-limit é restrição externa legítima → adaptar, nunca evadir.

**Proveniência:** princípio minerado de `osiris/src/lib/sherlock.ts` (mapLimit + calibração de concorrência, lição empírica **20→12**) e `osiris/src/lib/httpJson.ts` (UA honesto — contra-tendência a preservar). **Nenhum código importado** — síntese em 4 parâmetros (global/per-provider/rate+burst/backoff) é formalização COSCA.

---

### 3.4 ADR-041 — Hardening do guard anti-SSRF

**O que entrou** (`internal/security/netguard.go` + `safehttp.go`):

- `func CheckHostname(host string) error` — **blocklist de hostnames de metadata ANTES do DNS** (`localhost`, `*.local`, `*.internal`, `host.docker.internal`, `metadata.google.internal`). Fail-closed.
- `func IsPublicIP(addr netip.Addr) bool` — fail-closed: qualquer faixa não-pública → `false`; **IPv6 de transição decodificado** (6to4 `2002::/16`, NAT64 `64:ff9b::/96`) antes de revalidar.
- `func DecodeEmbeddedIPv4(addr netip.Addr) (netip.Addr, bool)` — fecho da brecha do cloud metadata service.
- `func ValidatePublicIP(ip netip.Addr) error` — mensagem descritiva (instrutiva), inclui o alvo decodificado.
- `func NewSafeClient(timeout, maxRedirects) *http.Client` — **revalidação hop-by-hop de redirect**: `CheckRedirect` re-aplica o guard a **cada salto**, **nega público→privado** (via `validateURLHost`), **limita a `http`/`https`**, impõe **teto de `maxRedirects`**.
- Blocos IPv4 reservados (incl. **CGNAT** `100.64.0.0/10`, `169.254.169.254`/IMDS, TEST-NET, multicast, reservado) e IPv6 reservados (loopback, IPv4-mapped, NAT64, discard, doc, ULA, link-local, site-local, multicast) — §2.2/2.3 do ADR.

**Consumidor integrado** (`internal/mcpserver/tools.go`, `handleWeb` → tool `cosca.web`):

- `security.CheckHostname(u.Hostname())` (antes do DNS) → `net.DefaultResolver.LookupIP` → `security.ValidatePublicIP(addr)` para **todas** as IPs → `security.NewSafeClient(15*time.Second, 3)` para o GET (redirects revalidados).

**Testes** (`netguard_test.go` 5 testes + `safehttp_test.go` 6 testes):

- `TestIsPublicIP_PrivateBlocked`, `TestDecodeEmbeddedIPv4_NAT64_SixTo4`, `TestValidatePublicIP_MetadataServiceBlocked`, `TestIsPublicIP_ReservedIPv6Blocked`, `TestCheckHostname`; `TestSafeClient_BlocksRedirectToPrivate`, `TestSafeClient_BlocksRedirectToMetadataHost`, `TestSafeClient_MaxRedirects`, `TestSafeClient_BlocksNonHTTPSchema`, `TestSafeClient_InitialRequestAllowed`, `TestSafeClient_BlockedErrorIsDescriptive`.
- Resultado: **PASS** (`go test ./internal/security/...` → `ok`; `go test -race ./internal/security/…` → `ok`).

**Medição (lógica):** cobertura das classes reservadas por **tabelas determinísticas** (IP-confusion, IPv6-mapped, metadata, redirect) — validada por testes unitários, não por benchmark de tempo. Não há benchmark (não é caminho quente) — a medição é a **prova da classificação fail-closed** de cada faixa.

**Golden Gate / PromotionGate:** **aprovado** (guard `netguard.go` foi **aumentado** `39 +-`; `NewSafeClient`/`CheckHostname` novos; `handleWeb` endurecido `12 +-`). Verifica: cada hop obedece ao guard; honestidade (sem evasão) preservada (ADR-022).

**Proveniência:** princípio minerado de `osiris/src/lib/ssrf-guard.ts` (MIT) — visão hop-by-hop `URL→DNS→IP→redirect→DNS→IP→destino`, rejeição de IP-confusion (só `dotted-quad` estrito), cobertura exaustiva IPv6/IPv4 reservado, blocklist de metadata antes do DNS, revalidação por hop, rate limiter/`getClientIp`. **Nenhum código/lista/expressão do `ssrf-guard.ts` importado.**

> **Atenção (pendência deliberada — §4.3):** o `internal/acquisition` **ainda usa** guard mais fraco (`net.ParseIP` + `blockedCIDRs` sem CGNAT/multicast/TEST-NET). NÃO migrado por escopo mínimo. TOCTOU/IP-pinning não coberto.

---

### 3.5 ADR-042 — GET condicional + `optional()`

**O que entrou** (`internal/acquisition/conditional.go` + `optional.go`):

- `type Validators { ETag; LastModified }` / `type ConditionalResult { Changed bool; Body []byte; Validators Validators }`.
- `func (c *Client) FetchConditional(ctx, rawURL, v Validators) (ConditionalResult, error)` — **replay de `If-None-Match`/`If-Modified-Since`**; **`304` = resposta VÁLIDA** (`changed:false`, `body:nil`, sem erro); **degradação honesta** sem validators (GET comum com grace); **decode do corpo PELO `Content-Encoding` do header** (gzip/deflate; desconhecido → raw, sem dependência brotli); **User-Agent identificador honesto** (`Cosca-Acquisition/1.0 (+https://github.com/CoscaAI/cosca)`); teto de corpo/redirects herdado do cliente endurecido.
- `func Optional[T any](fn func() (T, error)) (T, bool)` — enriquecimento opcional: resolve erro para zero + `ok=false`; `ok=false` é distinto de "zero legítimo".
- `validatorsFrom(h, prev)` — cai de volta para os validators anteriores quando o header não traz novo (upstream que ignora ainda funciona).

**Testes** (`conditional_test.go` 5 + `optional_test.go` 3):

- `TestFetchConditional_304`, `TestFetchConditional_LastModified`, `TestFetchConditional_DecodesGzip`, `TestFetchConditional_SendsHonestUA`, `TestFetchConditional_Non200IsError`; `TestOptional_Success`, `TestOptional_Error`, `TestOptional_DistinguishesZeroFromUnavailable`.
- Resultado: **PASS** (`go test ./internal/acquisition/...` → `ok`; `go test -race ./internal/acquisition/…` → `ok`).

**Medição (lógica):** sem benchmark de tempo; a medição é a **prova da semântica do `304`** (`changed:false` sem erro, corpo não re-baixado) e do **decode por header** (gzip) e da **distinção zero-legítimo × indisponível** (`ok=false`). Validada por testes unitários determinísticos.

**Golden Gate / PromotionGate:** **aprovado** (aditivo ao pacote `internal/acquisition` — `conditional.go`/`optional.go` novos; `acquisition.go` alterado apenas p/ expor `readDecodedBody` e o cliente endurecido; nenhuma API pública existente quebrada).

**Proveniência:** princípio minerado de `osiris/src/lib/httpJson.ts` (MIT) — GET condicional (`304` barato, válido, não erro), decode por header (não por `Accept-Encoding`), UA honesto (contra-tendência: endpoints OSM rejeitam browser-UA com 406/429), `optional()` (falha opcional → `null`, nunca derruba o pipeline). **Nenhum código do `httpJson.ts` importado.**

> **Atenção (§4.4):** primitiva criada, mas **sem wiring a um poller que persiste validators**; `br` (brotli) passa como **raw** (sem dependência).

---

### 3.6 Observação sobre ADR-038 (doutrina — documento, não primitiva)

O **ADR-038** (`O veredito do instrumento de avaliação é um estado epistêmico`) **não gera primitiva de código** — é **doutrina canônica** que formaliza a lição da Campanha-001 (LoRA no Qwen3-4B regrediu `0.88 → 0.75`; o Golden Gate/PromotionGate **detectou e reprovou** — o mecanismo anti-autoengano funcionou). Ele **motiva** o ADR-037 (`Verdict`) e é **consumido** por `architecture + review/critic` como regra de leitura: *`FAILURE` de eval só é válido quando o instrumento observou com confiança; caso contrário é `INCONCLUSIVE`.* Nenhum arquivo `.go` foi alterado por este ADR — **zero diff** (verificação §3.7).

---

### 3.7 Contratos preservados (contexto; verificação de integridade)

Os seguintes arquivos foram confirmados como **intactos** (zero diff entre o commit `4311996` e o pai `4311996^`):

- `internal/cache/cache.go` — o `cache.Cache` (value-store) permanece; `CachedSource` é primitiva **aditiva** que não o altera.
- `internal/level/gate.go` — `level.Verdict` (decisão de **autorização** allow/deny) permanece; o `evidence.ObservationState` é classe distinta (ver nota no doc-comment de `state.go`).
- `internal/compute/capability_model.go` — `CapabilityStatus` (estado de uma **capacidade**) permanece.

`git diff --stat 4311996^ 4311996 -- internal/cache/cache.go internal/level/gate.go internal/compute/capability_model.go` → **vazio** (confirmado). Ou seja: **nenhum contrato anterior foi quebrado** pela esteira — as 5 primitivas são todas **aditivas**. Este também é o motivo pelo qual `internal/evidence` e `internal/concurrency` são **pacotes novos** e `internal/cache`/`internal/security`/`internal/acquisition` receberam **somente adição/aumento**.

---

## 4. Decisões de wiring / REVISÃO PENDENTES (o coração do relatório)

> A esteira **implementou e validou primitivas**, mas **o wiring é decisão de arquitetura/revisão** — e é **deliberadamente não-resolvido** onde não havia valor imediato ou onde havia risco de mexer em consumidores sem medir. Cada item abaixo é o que **ainda depende de decisão do professor / donos de camada**. Nada foi esquecido; tudo está registrado para decisão.

### 4.1 ADR-037 — `ObservationState`: **primitiva criada, SEM consumidor ligado**

- **O que está feito:** a classe epistêmica (`ObservationState`/`Observation`/`Calibrate`) existe e é validada.
- **O que NÃO foi feito:** **nenhuma ferramenta/verificação atual expõe `ObservationState`** como resultado canônico de observação — `cosca.web`, OCR, sensores e o avalador ainda usam primitivas de observação mais simples (`bool`/status).
- **Onde ligar (decisão pendente — Evidence Layer):** em quais ferramentas/verificação aplicar `Verdict`/`ObservationState` é decisão da **Evidence Layer** (`review/critic` + Kernel). O ponto de maior valor: o **Golden Gate** consumir `ObservationState` para distinguir *"o modelo regrediu"* de *"o instrumento não conseguiu medir"* (a regra do ADR-038). Deixa **explícito**: a primitiva está pronta; **o contrato que a consumir ainda não foi decidido**, e isso é o gap arquitetural de maior impacto.
- **Por que não foi feito:** ligar `Verdict` às ferramentas muda o **contrato de retorno** de cada instrumento — mexer em consumidores sem antes desenhar a Evidence Layer seria refatoração sem direção. Escopo mínimo respeitado.

### 4.2 ADR-039 — `CachedSource`: **primitiva criada, SEM consumidor**

- **O que está feito:** o wrapper de fonte resiliente existe e é validado (benchmark hit ~47 ns/op, 0 allocs).
- **O que NÃO foi feito:** **nenhum ponto da camada de conhecimento/semântica** usa `CachedSource` — o fetch de fonte ainda é direto.
- **Onde ligar (decisão pendente — cache chief + memory chief):** plugar o wrapper **onde o conhecimento/semântico lê fontes upstream** (conteúdo, semântica, web, datasets). Candidatos: leituras de agregados/`region=all`, harvest multi-fonte, refreshes de corpo grande.
- **Benefício ainda NÃO medido em produção:** o ganho (thundering herd eliminado, custo por TTL, stale-on-error) é **teórico/medido em teste unitário** — **não medido em produção** (é o motivo do "drop-in sem consumidor").

### 4.3 ADR-041 — SSRF: **JÁ integrado no `handleWeb`**, MAS pendência deliberada no `acquisition`

- **O que está feito:** `cosca.web` (`handleWeb`) usa `CheckHostname` + `ValidatePublicIP` (todas as IPs) + `NewSafeClient` (redirect hop-by-hop) — guard **completo** para o tool.
- **PENDÊNCIA DELIBERADA (escopo mínimo)** — `internal/acquisition` **ainda usa guard mais fraco**: `net.ParseIP` + `blockedCIDRs` (sem **CGNAT**, **multicast**, **TEST-NET**, sem **IP-confusion** anti-`dotted-quad`, sem `DecodeEmbeddedIPv4`). A migração do `acquisition` para `netip` + `CheckHostname` **NÃO foi feita** nesta esteira — por **escopo mínimo** (evitar um diff grande num caminho de aquisição já existente e validado).
- **TOCTOU / IP-pinning:** **não coberto** — a rejeição no *lookup* bloqueia rebinding **não-maligno**; a defesa **total** (IP-pinning no socket) é degrau **opcional** (anotado no ADR-041). Decisão pendente: **não implementado**.

### 4.4 ADR-042 — Conditional GET / `optional()`: **primitiva criada; wiring a poller pendente**

- **O que está feito:** `FetchConditional` + `Optional` existem e são validados (semântica `304`, decode por header, UA honesto, `ok=false` distinto de zero).
- **O que NÃO foi feito — wiring (decisão pendente):** **ligar a um poller que persiste os validators** (`ETag`/`Last-Modified`) entre polls é decisão de **integrations + runtime**. Sem esse persistência/loop, o ganho do `304` **não se materializa** — a primitiva está pronta, mas o **estado condicional não é mantido por um poller** ainda.
- **brotli (`br`):** passa como **raw**, sem dependência — decisão de incluir brotli fica para quando houver fonte que realmente o exija (evita adicionar dependência sem real-consumidor).

### 4.5 ADR-040 — `ConcurrencyPolicy`: **integrada só no `semantic_router` (opt-in)**, 5 call-sites NÃO migrados

- **O que está feito:** integrado em `internal/orchestration/semantic_router.go` como **drop-in** (`MaxGlobal: 5` == comportamento anterior, 0 regressão).
- **O que NÃO foi feito — 5 call-sites não migrados (decisão pendente / opt-in):** os demais pontos de concorrência do sistema **continuam** com semáforos/pools próprios, **não** usando a `Policy`:
  - `pipeline/StepRunner` (orquestração de passos),
  - subprocess (worldmodel: `vfx`/`spatial`/`simulation`/`destruction` `runSubprocess`),
  - `chat/executor`,
  - workflow (orquestração de workflows),
  - (e outros pontos de fan-out).
- **Critério para migrar (decisão pendente):** **só onde houver provider com rate-limit real** (para não adicionar política sem valor). A migração é **opt-in** e **medida** — a política é invocada (`Acquire`/`Release`/`ReportRateLimited`) pelo caller.
- **Throughput em produção NÃO medido:** o ganho foi **medido em carga simulada** (`TestBeforeAfter_AdaptsToRateLimitedProvider` → 4→0 erros), **não em produção** — que é o motivo do opt-in.

---

## 5. Fora de escopo / não tocado

- **Prediction Ledger (Brier/calibração):** **estacionado** — **não rejeitado, não esquecido**. Sem ADR (deliberado, decisão do Don 2026-09-04). Agente: **2ª onda**, quando a fase de **evolução/calibração** entrar (Golden Gate / CMI / CheckPromotion para medir a **calibração da confiança do agente**, não só o acerto da task). Registrado em `runs/ledger.jsonl` do Osiris → **minerado como princípio**, não importado.
- **stealthFetch (IP-spoofing + UA randomizado):** **REJEITADO** — viola a **Lei do Cofre / LEALDADE** (ADR-022) e o Fail-Closed do Guard Pact. Sem ADR (deliberado). **Nenhum mecanismo de evasão incorporado em nenhuma camada.** Frase-síntese: *"COSCA não precisa trapacear para ser resiliente."*
- **`level.Verdict`** (autorização allow/deny) e **`CapabilityStatus`** (estado de capacidade): **intactos** — **zero diff** (verificação §3.7). São classes **distintas** de `evidence.ObservationState` (classe epistêmica), como declara o doc-comment de `state.go`.
- **Nenhum teste/contrato anterior removido.** Nenhum segredo. **Nenhum código do Osiris importado** — apenas princípios, reimplementados como primitivas Go idiomáticas setdlib-only.

---

## 6. Próximos passos sugeridos (para o professor aprovar)

1. **Revisar a implementação** (commit `431199679290f2546c0a47e5602716960daccf33`) e o mapa da §2.
2. **Decidir o wiring da Evidence Layer (ADR-037)** — **ponto de maior valor arquitetural**: onde `ObservationState`/`Verdict` passa a ser o contrato de retorno das ferramentas/verificação, e onde o Golden Gate passa a exigir o **estado observacional** antes de emitir `FAILURE` (regra ADR-038).
3. **Decidir onde plugar `CachedSource`** em conhecimento/semântico (ADR-039) — cache chief + memory chief; e **medir** o benefício em produção.
4. **Decidir a migração do guard do `acquisition`** para `netip` + `CheckHostname` (remanescente do ADR-041) — fechar CGNAT/multicast/TEST-NET/IP-confusion naquele caminho; avaliar a necessidade de IP-pinning (TOCTOU).
5. **Segunda onda (Prediction Ledger)** — quando a fase de **evolução/calibração** entrar; extender o Golden Gate/CMI para validar a **calibração da confiança** do agente.

---

## Referência / proveniência cruzada

- Relatório de mineração (frente fechada): `docs/reports/mineracao-osiris-padroes-2026-09-04.md`.
- ADRs emitidos: `ADR-037` … `ADR-042` (`docs/adr/`).
- Commit de implementação: `431199679290f2546c0a47e5602716960daccf33`.
- Repositório minerado: `github.com/simplifaisoul/osiris` (MIT) — `src/lib/sherlock.ts` (ADR-037/040), `src/lib/sourceCache.ts` (ADR-039), `src/lib/ssrf-guard.ts` (ADR-041), `src/lib/httpJson.ts` (ADR-042), `src/lib/stealthFetch.ts` (REJEITADO), `runs/ledger.jsonl` (2ª onda).

---

*Autor: cosca-architecture (Architecture Chief). Relatório de fechamento da esteira de mineração→implementação→validação dos padrões do repositório Osiris (MIT). Nenhum arquivo `.go` foi alterado por este relatório — apenas consolidação de evidências. As primitivas implementadas (ADR-037/039/041/042/040) estão testadas (+race limpo) e medidas; o que NÃO foi feito (wiring, migração de call-sites, TOCTOU, brotli) está explícito na §4 para decisão do professor. A tese da esteira permanece: minerar ideias, nunca importar código.*
