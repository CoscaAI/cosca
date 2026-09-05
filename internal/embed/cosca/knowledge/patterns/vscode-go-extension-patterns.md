# vscode-go Extension Patterns — LSP Lifecycle, Helper Process & Tool Management, Debug/Test Architecture

> **Version**: 1.0.0 | **Confidence**: 0.93 | **Category**: Enterprise Platform Patterns | **Created**: 2026-08-13 | **Source**: https://github.com/golang/vscode-go.git (MIT License)

> **Mined by**: cosca-kernel (3 agentes explore em paralelo — lifecycle LSP, helper vscgo+IPC+tools, debug/test/commands). Extraído de `extension/src/` (TypeScript) e `internal/vscgo/` (Go). Padrões descritos em linguagem neutra — licença MIT é permissiva, mas copiamos o *design*, não o código.

## Purpose

A extensão VS Code para Go (gopls/gopls + Delve + go test) é a referência canônica de **gestão de ciclo de vida de um processo servidor dependente a partir de um cliente de UI**. Ela resolve os mesmos problemas que o Cosca enfrenta com `serve`/`runtime`/`ollama` (daemons) e o `cosca-desktop` (UI + language server + debug): start/stop/restart sem corrida, crash recovery, atualização de versão, fallback gracioso, e telemetria não-bloqueante.

---

## A. Language Server Lifecycle (gopls)

### A1. Context Object + Orchestrator serializado por mutex
- **O que resolve**: corrida entre múltiplos gatilhos de start/restart; estado espalhado.
- **Como funciona**: um único objeto de contexto (`GoExtensionContext`) guarda referência ao client, última config, contador de crash, histórico de restart, canais de output. Um único ponto de entrada (`startLanguageServer`) faz todo o ciclo start/stop/restart, serializado por um **mutex baseado em promise-chain** (`utils/mutex.ts`).
- **Onde**: `extension/src/context.ts`, `commands/startLanguageServer.ts`, `utils/mutex.ts`.
- **Cosca**: criar um `RuntimeManager` central (referência ao processo/daemon, último config, `crashCount`, `restartHistory`, canais de log) com mutex serializando transições — hoje `serve`/`runtime`/`ollama` são subidos por `setsid nohup` ad hoc (L189/L190), sem orquestrador.

### A2. Stop com timeout client-side
- **O que resolve**: shutdown de servidor que trava (não responde ao request `shutdown`).
- **Como funciona**: `stopLanguageClient` chama `c.stop(2000)` com timeout **no cliente** e `.catch` loga a falha em vez de propagar — nunca fica preso num `await` infinito.
- **Onde**: `language/goLanguageServer.ts:299-315`.
- **Cosca**: todo shutdown de daemon gRPC precisa de timeout + log (nunca bloquear o host). O runtime já trata SIGINT/SIGTERM, mas o *client* (desktop/CLI) não impõe timeout ao aguardar o daemon morrer.

### A3. Circuit breaker de crash (contador + parada definitiva, reset em parada limpa)
- **O que resolve**: loop infinito de restart quando o servidor crasha por causa de config/versão quebrada.
- **Como funciona**: `crashCount` incrementa a cada conexão fechada; `< 5` → restart imediato; `>= 5` → para e chama `suggestActionAfterGoplsStartError` (remediação). Resetado a zero em parada limpa. **Sem backoff** no original (aceitável em UI).
- **Onde**: `language/goLanguageServer.ts:439-481`, `1405-1520`.
- **Cosca**: aplicar ao runtime/ollama — N crashes → parar e diagnosticar (porta ocupada, config inválida, versão incompatível), não reiniciar cegamente. Num daemon autônomo, **adicionar backoff exponencial** (melhoria sobre o original) mantendo o teto de tentativas.

### A4. Decisor de atualização com guardas + staged rollout por hash
- **O que resolve**: auto-atualização que quebra o usuário (regressão de versão nova).
- **Como funciona**: `shouldUpdateLanguageServer` aplica guardas que retornam `null` (não atualizar): checkForUpdates off, Cloud IDE, versão `(devel)`/`+dirty`, Go base velho demais. Pseudoversions comparadas por **timestamp**. `okForStagedRollout` libera atualização automática **progressivamente por hash de machine-id**: 10% no dia 1, 30% até dia 3, 100% depois — limita blast radius.
- **Onde**: `language/goLanguageServer.ts:1145-1263`, `131-157`.
- **Cosca**: para atualização de runtime/ollama — guardas (não atualizar em dev/local, engine base velho) + staged rollout por hash de instalação.

### A5. Prompt de upgrade com consentimento persistente
- **O que resolve**: re-prompt a cada boot.
- **Como funciona**: `promptForUpdatingTool` oferece `['Always Update', 'Update Once', 'Release Notes']`; "Always Update" grava `toolsManagement.autoUpdate=true` (consentimento persistente); "Update Once" só instala; senão adiciona a `declinedUpdates` (não incomoda de novo).
- **Onde**: `goInstallTools.ts:452-525`.
- **Cosca**: persistir decisão do Don sobre atualização de provider/modelo (nunca repetir prompt).

### A6. Restart por evento tipado (RestartReason) + histórico
- **O que resolve**: observabilidade de *por que* reiniciou.
- **Como funciona**: todo restart carrega enum `RestartReason` (`ACTIVATION`, `MANUAL`, `CONFIG_CHANGE`, `INSTALLATION`), registrado em `restartHistory` (limitado a 10).
- **Onde**: `language/goLanguageServer.ts:111-128`.
- **Cosca**: registrar razão tipada de cada restart do runtime (deploy, config, manual, crash) para diagnóstico.

### A7. Subscrição seletiva de config (debounce por filtro)
- **O que resolve**: restarts em cascata por mudanças irrelevantes.
- **Como funciona**: `addConfigChangeListener` usa `e.affectsConfiguration('go.x')` para reiniciar **somente** quando mudam chaves relevantes ao servidor (`useLanguageServer`, `languageServerFlags`, `alternateTools`, `toolsEnvVars`, `formatTool`). Outras mudanças (cor de coverage) não reiniciam.
- **Onde**: `goMain.ts:252-345`.
- **Cosca**: o desktop deve reiniciar o runtime **só se a chave afeta o daemon**, não por qualquer mudança de config.

### A8. Provider swap (LSP↔legacy) com dispose + fallback granular por feature
- **O que resolve**: degradação graciosa quando o servidor está indisponível, sem "tudo ou nada".
- **Como funciona**: quando gopls está desabilitado/indisponível, registra `LegacyLanguageService` (providers nativos — formatter via gofmt/goimports); ao iniciar o servidor, **descarta o legacy** (`dispose()`) para não haver duplicação. Além disso, um **middleware** pode "roubar" uma feature individual (formatter custom) mesmo com o LSP ativo — fallback **granular por feature**, não binário.
- **Onde**: `registerDefaultProviders.ts:11-24`, `startLanguageServer.ts:52-75`, `language/goLanguageServer.ts:681-693`.
- **Cosca**: o knowledge engine (busca semântica via embedding) deve trocar para fallback lexical (FTS5) quando o provider de embedding está fora, e **descartar o substituto** ao recuperar. Se só o embedding falhar, manter indexação/storage funcionando.

### A9. Status bar dirigida por máquina de estados
- **O que resolve**: refletir o estado real do servidor na UI.
- **Como funciona**: `updateLanguageServerIconGoStatusBar` mapeia `languageClient.state` (Starting→spinner, Running→zap, Stopped→warning) para ícone/cor, e `setContext('go.goplsIsRunning', ...)` condiciona menus.
- **Onde**: `goStatus.ts:161-194`.
- **Cosca**: o desktop deve refletir o estado do runtime/serve (starting/running/crashed/stopped) a partir de uma máquina de estados real, não de flags ad hoc.

---

## B. Helper Process Externo (vscgo) + IPC + Tool Management

### B1. Helper process externo (binário Go separado)
- **O que resolve**: operações performance-critical / que reusam bibliotecas do ecossistema Go, isoladas de um processo, sem bloquear o host.
- **Como funciona**: a extensão delega a um binário Go autônomo (`vscgo`), compilado e distribuído junto. `vscgo/main.go` é um `main` fino (~14 linhas) que delega para `internal/vscgo.Main()` — a implementação fica **importável/testável** pelo mesmo módulo. Operações movidas: `inc_counters` (telemetria), `dump-pprof`/`serve-pprof` (parsing binário pprof + HTTP). O que NÃO foi movido (fica como `go` orquestrado pelo TS): resolução de módulos, scan de pacotes.
- **Onde**: `vscgo/main.go`, `internal/vscgo/main.go`, `internal/vscgo/pprof.go`.
- **Cosca**: `cosca-indexer` é o análogo (helper Go), mas hoje é **monolítico** — a lógica de walk/coleta está toda no `main`, sem dispatcher de subcomandos. Refatorar para `cmd/cosca-indexer/main.go` fino + `internal/indexer` com superfície `index/query/search/stats/version`. O surface de operações pesadas (`IndexDocument`/`IndexDirectory`/`RebuildAll` em `internal/knowledge`) deve ser o contrato do helper.

### B2. IPC de três modos (stream line-framed, one-shot JSON, handshake+HTTP)
- **O que resolve**: contrato de comunicação com o helper flexível por tipo de operação.
- **Como funciona**: (1) **stream line-framed** — `inc_counters` lê `"nome valor\n"` do stdin (bufio.Scanner + Sscanf); (2) **one-shot JSON** — `dump-pprof` faz `json.NewEncoder(os.Stdout)`; (3) **handshake+HTTP** — `serve-pprof` escreve `{"Listen": addr}` no stdout e depois serve JSON via HTTP com CORS.
- **Onde**: `internal/vscgo/main.go:166-241`, `pprof.go:31-70`, `goTelemetry.ts:204-240`.
- **Cosca**: padronizar o `cosca-indexer` com JSON-lines no stdout (`index --json` progressivo, `search --json` streaming) e um modo `serve` com handshake para o desktop consultar o indexer incrementalmente. Hoje o desktop não tem como consultar o indexer sem re-inicializar o engine.

### B3. Gestão de ferramentas com catálogo versionado + version tiers por runtime
- **O que resolve**: instalar/atualizar/verificar ferramentas dependentes com compatibilidade de versão.
- **Como funciona**: catálogo central (`allToolsInformation`) com `name/importPath/defaultVersion/latestVersion/minimumGoVersion/replacedByGopls`; `getRequiredTools` deriva o conjunto das settings; `go install <importPath>@<version>`; **version pinning por tier de versão do runtime** (`getImportPathWithVersion`: gopls `@v0.14.2` se go<1.19, `@v0.15.3` se go<1.21); isolamento de ambiente (`tmpDirForToolInstallation` cria `go.mod` sintético para não tocar o workspace).
- **Onde**: `goToolsInformation.ts`, `goTools.ts:135-177,62-103`, `goInstallTools.ts:337-377`.
- **Cosca**: mapear "ferramentas Go" → "providers/modelos". Adicionar ao `internal/models` campos `minimumEngineVersion`/`defaultModelVersion`/`deprecated`; seleção de modelo **por capacidade do provider** (pinar `deepseek-v4-flash` vs `deepseek-chat` conforme a versão do engine, como o vscode-go pina gopls por versão do Go). Faltam no Cosca "missing/outdated" + `suggestUpdates` + `autoUpdate` persistente.

### B4. Descoberta de ambiente com degradação (não bloqueio)
- **O que resolve**: ambiente ausente/quebrado não derruba o resto.
- **Como funciona**: `updateGoVarsFromConfig` roda `go env -json` e mescla; GOROOT inválido → avisa e **ignora** (com "Don't Show Again"); Go ausente → mensagem mas a ativação **continua**; `getBinPathWithPreferredGopathGorootWithExplanation` busca em ordem (alternateTools → GOBIN → toolsGopath/bin → GOROOT/bin → PATH) com cache e `clearCacheForTools()`.
- **Onde**: `goInstallTools.ts:527-599`, `goEnv.ts:93-131`, `pathUtils.ts:60-133`.
- **Cosca**: trocar o `log.Fatal()` do `cosca-indexer` (quando `.cosca` não existe) por degradação (avisar e continuar); tratar provider ausente como status `no_key`/`not_running` (já em `detectStatus`). Adicionar **cache de resolução com invalidação** ao `desktopIsStale` (hoje re-walkeia os sources a cada chamada).

### B5. Single-flight / fan-out (dedupe de execuções concorrentes)
- **O que resolve**: N execuções caras idênticas simultâneas.
- **Como funciona**: `getAllPackagesNoCache` deduplica `go list std all` usando um `Set` (`goListPkgsRunning`) + `Map` de assinantes — só uma execução por workDir, todos aguardam a mesma promise. `TelemetryReporter` faz batch (acumula contadores, flush 60s) com máquina de estados para impedir spawns concorrentes.
- **Onde**: `goPackages.ts:109-134`, `goTelemetry.ts:132-249`.
- **Cosca**: o `cosca-indexer` é CPU-bound e roda em loop síncrono; aplicar single-flight no `internal/knowledge`/`internal/indexer` para deduplicar `IndexDocument` concorrentes.

### B6. Cache em camadas com invalidação + TTL adaptativo
- **O que resolve**: subprocessos/leitura de disco repetidos.
- **Como funciona**: `binPathCache` (resolução de binário, invalidado por `clearCacheForTools`); `allPkgsCache` com `cacheTimeout` adaptativo (ajustado à duração da última execução, min 5s); `cachedGoVersion`; `getLatestGoVersions` em globalState com TTL 24h; short-circuit sem subprocesso (`getModuleCache` deriva sem chamar `go env`).
- **Onde**: `pathUtils.ts:17`, `goPackages.ts:36,141-163`, `util.ts`, `goEnvironmentStatus.ts:549-574`.
- **Cosca**: o `Lookup` do `internal/models/cache.go` relê o JSON a cada chamada (sem cache em memória); adicionar um mapa em memória modelID→Model com invalidação quando `SyncFromAPI` grava. Usar watcher/dirty-flag (já existe `WatchEnabled`) para indexar só mudanças (`IndexChanged`/`IndexRemoved`).

---

## C. Debug Adapter (Delve) + Test Runner + Command Dispatch

### C1. Adapter dual + Proxy de processo (dlv-dap)
- **O que resolve**: complexidade de reimplementar o protocolo de debug.
- **Como funciona**: dois caminhos intercambiáveis — (a) adapter **legacy** implementa DAP traduzindo cada request para JSON-RPC ao Delve; (b) **dlv-dap** não implementa DAP: spawna `dlv dap` e faz **proxy de socket duplex**, parseando só o framing `Content-Length: ...\r\n\r\n`. Decisão via factory de descriptor. `restart` é orquestrado pelo host (termina e relança), não pelo adapter.
- **Onde**: `goDebugFactory.ts:43`, `debugAdapter/goDebug.ts:431`.
- **Cosca**: o `DebugStart` do desktop deve spawnar `dlv dap` e fazer proxy de socket (muito mais simples que o legacy), em vez de reimplementar DAP.

### C2. Ciclo de vida do processo depurado (halt→detach→killProcessTree + timeout)
- **O que resolve**: vazamento de processo na teardown.
- **Como funciona**: `close()` distingue cenários: launch-local → `halt` + `Detach{Kill}` + `killProcessTree` + remove binário temp; attach-local → detach sem matar; attach-remote → só fecha RPC. `dispose` tem token de 1s antes de matar o dlv.
- **Onde**: `debugAdapter/goDebug.ts:834`, `goDebugFactory.ts:301`.
- **Cosca**: o `DebugStart`/`KillTerminal` do desktop precisa do mesmo shutdown com fallback de kill após timeout, tratando launch/attach/remote.

### C3. Runner de teste único + dual-parser (JSON/linhas)
- **O que resolve**: classe de bugs de "parse de output de teste".
- **Como funciona**: um runner central (`goTest`) monta `go test -test.fullpath=true` (+ `-timeout`, `-coverprofile=<tmp>`, `-run ^Nome$`). Parsing **dual**: modo JSON (`go test -json` via test2json, modelado por `GoTestOutput{Action,Output,Package,Test,Elapsed}`) com fallback regex para saída não-JSON (erros de build no stderr). Expande paths relativos via `pkgMap` (package→dir de `go list`).
- **Onde**: `testUtils.ts` (`goTest`, `computeTestCommand:529`, `processTestResultLineInJSONMode:617`, `processTestResultLineInStandardMode:651`).
- **Cosca**: o TestTool da esteira deve **sempre** usar `-json` + `-test.fullpath=true` e consumir eventos estruturados, com fallback regex — resolve a classe de bug de parse de output.

### C4. Descoberta de testes por símbolos+regex (não `go test -list`)
- **O que resolve**: evita build só para listar testes.
- **Como funciona**: usa document symbols do gopls + regexes (`testFuncRegex`, `benchmarkRegex`, `testMethodRegex` para suites testify `(*X).TestY`), sem `go test -list`.
- **Onde**: `testUtils.ts:47-52`, `goDocumentSymbols.ts`.
- **Cosca**: o TestTool pode gerar a lista de testes a partir de símbolos/AST, alimentando o StepRunner sem build extra.

### C5. Consumer de eventos com estado por TestItem (subtests/benchmarks)
- **O que resolve**: correlação output↔teste, incluindo subtests e benchmarks sem evento terminal.
- **Como funciona**: `consumeGoTestEvent` mapeia `Action` (run→started, pass→passed, fail→failed, skip→skipped, output→buffer por test id); benchmarks não emitem run/pass — deduz estado do output e marca incompletos como passed ao final (`markComplete`); subtests resolvidos por `/`.
- **Onde**: `goTest/run.ts`, `goTest/test_events.md`.
- **Cosca**: modelo direto para o StepRunner reportar progresso por teste (subtests/benchmarks) com máquina de estados por item.

### C6. ID codificado em URI (query=kind, fragment=name)
- **O que resolve**: mapear nós de árvore ↔ (tipo, nome) sem tabelas laterais.
- **Como funciona**: o `id` do TestItem é a URI do arquivo com `query=kind` (test/benchmark/file/package/...) e `fragment=functionName` — round-trip barato id↔(kind,name).
- **Onde**: `goTest/utils.ts:40`.
- **Cosca**: útil para correlacionar eventos de output com o item de teste no StepRunner/desktop.

### C7. CommandFactory + registerCommand (closure com deps injetadas)
- **O que resolve**: registro uniforme de comandos testável sem o host.
- **Como funciona**: `createRegisterCommand(ctx, goCtx)` retorna `registerCommand(name, fn)`; todo comando é `CommandFactory<T> = (ctx, goCtx) => CommandCallback<T>` (factory de 2 estágios). **Partial application**: `testAtCursor('test')` vs `('debug')` vs `('benchmark')` reusam a mesma função. **Namespace por ponto** agrupado por domínio (`go.test.*`, `go.debug.*`, `go.lint.*`). **Barrel module** (`commands/index.ts`) centraliza re-export.
- **Onde**: `commands/index.ts:32`, `goMain.ts:133-234`, `goTest.ts:182`.
- **Cosca**: registrar cada binding desktop↔CLI como factory que recebe (session, config) e retorna o handler, com **um único ponto de registro** que cuida do lifecycle (dispose/cancelamento). Namespace por ponto (`cosca.test.run`, `cosca.debug.start`) dá taxonomia grep-ável.

### C8. Facade de config com scope por recurso + listener reativo + validação cruzada
- **O que resolve**: acesso disperso a settings; reações em cascata; conflitos.
- **Como funciona**: `getGoConfig(uri?)` encapsula `workspace.getConfiguration('go', uri)` (scope por recurso, stubbável em teste); `validateConfig` detecta conflitos (`go` vs `gopls` lint duplicado) e avisa **uma vez** (state guard); listener reativo usa `affectsConfiguration(ns)` para reagir seletivamente.
- **Onde**: `config.ts:36,87`, `goMain.ts:252`.
- **Cosca**: centralizar `cosca.*` settings num facade com overrides por workspace; um único subscriber que, conforme a chave, reinicia o runtime/recarrega env — em vez de N listeners acoplados; validação cruzada entre esteira vs desktop com aviso-única-vez.

### C9. Telemetria em memória + flush batch para processo externo
- **O que resolve**: instrumentação sem bloquear UI/esteira.
- **Como funciona**: `TelemetryReporter` acumula contadores em memória (`_counters`, add síncrono não-bloqueante), flush 60s spawnando `vscgo inc_counters` e escrevendo `"key value\n"` no stdin; `dispose` força flush final. `TelemetryKey` enum com buckets de latência. Delegação ao servidor via capability detection (`gopls.maybe_prompt_for_telemetry`). Anonimização por hash de machine-id.
- **Onde**: `goTelemetry.ts:132-328`.
- **Cosca**: acumular métricas de build/test/debug em memória e descarregar em lote via processo/daemon separado; definir um `TelemetryKey` equivalente; empurrar coleta pesada para o daemon/CLI.

---

## Synthesis — padrões priorizados para o Cosca

| # | Padrão | Aplicação |
|---|--------|-----------|
| 1 | Context Object + Orchestrator (A1) | `RuntimeManager` central para serve/runtime/ollama |
| 2 | Circuit breaker + remediação (A3) | Resiliência do runtime (parar + diagnosticar após N crashes) |
| 3 | Provider swap + dispose (A8) | Fallback do knowledge engine (semântico→lexical) |
| 4 | Helper externo + subcomandos (B1) | Refatorar `cosca-indexer` (main fino + `internal/indexer`) |
| 5 | IPC de três modos (B2) | JSON-lines no stdout do indexer + modo `serve` com handshake |
| 6 | Catálogo versionado + version tiers (B3) | Gestão de providers/modelos (missing/outdated/autoUpdate) |
| 7 | Adapter dual + Proxy de processo (C1) | `DebugStart` do desktop spawna `dlv dap` + proxy de socket |
| 8 | Runner único + dual-parser (C3) | TestTool da esteira sempre `-json` + fallback regex |
| 9 | CommandFactory + namespace (C7) | Registro de bindings desktop↔CLI com lifecycle |
| 10 | Telemetria batch para processo externo (C9) | Instrumentação não-bloqueante do desktop/esteira |

## Known Uses (referência)

- `golang/vscode-go` (extensão VS Code para Go, produção, milhões de instalações) — gestão de gopls (LSP), Delve (DAP), go test, e helper `vscgo`.

## Related Patterns

- [`temporal-workflow-engine-patterns.md`](temporal-workflow-engine-patterns.md) — durable execution / event sourcing (complementa o circuit breaker A3)
- [`backstage-plugin-platform-patterns.md`](backstage-plugin-platform-patterns.md) — DI + extension points (complementa o CommandFactory C7)
- [`argocd-gitops-reconciliation-patterns.md`](argocd-gitops-reconciliation-patterns.md) — reconciliação declarativa (complementa o single-flight B5)
- [`cosca-product-pattern.md`](cosca-product-pattern.md) — o padrão oficial de produto (helper `cosca-indexer` é o `vscgo` do Cosca)
