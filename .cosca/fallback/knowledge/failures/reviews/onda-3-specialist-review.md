## Review: Onda 3 Specialist Code Review

**Date:** 2026-07-28
**Reviewer:** cosca-specialist-review-code
**Scope:** All code areas produced by Wave 3 specialists (database-sql, backend-api, backend-service, testing-unit, testing-integration, frontend-component)
**Files reviewed:** 25 source files, 9 test files (6,319 test LOC)

### Quality Gates

| Gate | Description | Status |
|------|-------------|--------|
| G0   | `go build ./...` | ✅ PASS |
| G1   | `golangci-lint run` | ⚠️ SKIP (version mismatch: v1 vs v2 config) |
| G2   | `go vet ./...` | ✅ PASS (zero warnings) |
| G3   | `go test -race -short` | ✅ PASS (all 6 packages) |
| G9   | Code Review | 📋 This document |

---

### Critical (must fix) — 2 issues

- **[CRITICAL]** `api/rest/handler/websocket.go:74` — `InsecureSkipVerify: true` sempre habilitado.
  WebSocket origin verification está permanentemente desabilitada (`InsecureSkipVerify: true`). O comentário diz "Do NOT skip origin verification in production contexts" mas o código ignora isso. Qualquer domínio pode iniciar uma conexão WebSocket sem validação de origem, permitindo ataques CSRF via WebSocket.
  **Fix:** Tornar `InsecureSkipVerify` configurável via `Config` struct. Em produção (`mode != "development"`), usar `false`. Referência: OWASP ASVS V5.2.1, CWE-346 (Origin Validation Error).

- **[CRITICAL]** `web/src/features/playground/components/OutputArea.tsx:150-153` — XSS via `dangerouslySetInnerHTML`.
  A regex `/\*\*(.+?)\*\*/g` extrai conteúdo entre `**` e insere diretamente via `dangerouslySetInnerHTML` sem sanitização. Um output do LLM contendo `**<img src=x onerror=alert(1)>**` seria renderizado como `<strong><img src=x onerror=alert(1)></strong>`, executando JavaScript malicioso.
  **Fix:** Usar uma biblioteca de sanitização HTML (ex: DOMPurify) antes de passar conteúdo para `dangerouslySetInnerHTML`, ou usar uma abordagem que não envolva HTML raw (ex: splitting em segments React com `<strong>` components). Referência: OWASP XSS Prevention Cheat Sheet.

---

### High (must fix) — 3 issues

- **[HIGH]** `api/rest/handler/knowledge.go:147,191,252` + `api/rest/handler/memory.go:130,172,205,228,267,285` — Uso de `context.Background()` ao invés de `r.Context()`.
  Handlers criam um novo `context.Background()` em vez de propagar o contexto da request HTTP. Isso significa que:
  1. Operações de knowledge/memory não são canceladas quando o cliente desconecta
  2. Deadlines/timeouts da request HTTP não são respeitados
  3. Tracing context é perdido
  **Fix:** Substituir `ctx := context.Background()` por `ctx := r.Context()` em todos os handlers de knowledge e memory. Referência: Go context package docs, ADR-004 (Context Propagation).

- **[HIGH]** `api/mcp/server.go:538,615,1288` — `context.Background()` no MCP server.
  O servidor MCP cria timeouts com `context.Background()` em vez de propagar o contexto da sessão MCP. Operações long-running (sync, workflow) não são canceláveis quando o cliente MCP desconecta.
  **Fix:** Capturar o contexto da sessão MCP no momento da inicialização e derivar timeouts dele (`context.WithTimeout(sessionCtx, ...)`). Se não houver contexto de sessão, documentar explicitamente que MCP operations são fire-and-forget.

- **[HIGH]** `api/grpcserver/mapping_memory.go:24-27` — Erro de parse de TTL silenciado.
  Se o cliente enviar um TTL malformado (ex: `"abc"`, `"30"` sem unidade), `time.ParseDuration` falha silenciosamente e o TTL é setado para zero. O cliente nunca sabe que seu input foi ignorado.
  **Fix:** Logar um warning quando o parse falha e retornar `codes.InvalidArgument` para que o cliente seja notificado do erro de input. Exemplo:
  ```go
  if ttl, err := time.ParseDuration(req.GetTtl()); err != nil {
      return memory.MemoryRecord{}, status.Errorf(codes.InvalidArgument, "invalid TTL: %v", err)
  }
  ```
  Nota: Isso exige mudança na assinatura de `pbToMemoryRecord` para retornar error.

---

### Medium (should fix) — 5 issues

- **[MEDIUM]** `api/stream/sse.go:64-82` — TOCTOU race entre WriteEvent/WriteDone/WriteError e Close().
  Cada método de escrita verifica `s.closed` com lock, libera o lock, e depois escreve. Se `Close()` for chamada entre a verificação e a escrita, a escrita procede em um writer marcado como fechado. Embora o comentário diga "NOT safe for concurrent use", `Close()` é tipicamente chamado de outra goroutine (monitor de disconnect).
  **Fix:** Mover a escrita para dentro da seção crítica, ou usar um padrão de "close once" com `sync.Once` para tornar Close idempotente e seguro para concorrência. Opção mais simples: documentar que Close() deve ser chamado apenas após todos os writes terminarem.

- **[MEDIUM]** `api/stream/websocket.go:87-91` — Branches if/else idênticos.
  Ambos os branches produzem exatamente a mesma saída: `logger.With().Str("component", "ws-hub").Logger()`. O mesmo padrão se repete em `NewConnection()` nas linhas 358-362.
  **Fix:** Simplificar removendo a condicional redundante:
  ```go
  logger = logger.With().Str("component", "ws-hub").Logger()
  ```
  Ou, se a intenção é ter um fallback para logger zerado, usar:
  ```go
  if logger.GetLevel() == zerolog.Disabled {
      logger = log.Logger
  }
  logger = logger.With().Str("component", "ws-hub").Logger()
  ```

- **[MEDIUM]** `api/stream/websocket.go:534` — `closeWithReason` ignora erros.
  O método faz `_ = c.conn.Close(status, reason)` descartando o erro. Em casos de rede partida, o erro de close pode indicar um estado inconsistente que deveria ao menos ser logado.
  **Fix:** Adicionar um log de debug: `if err := c.conn.Close(status, reason); err != nil { c.logger.Debug().Err(err).Msg("close frame send failed") }`.

- **[MEDIUM]** `api/metrics/prometheus.go:66-80` — `snapshot()` é chamada com lock já adquirido.
  `writeHTTPMetrics()` adquire `m.mu.Lock()` e depois chama `snapshot()` internamente, que cria novos maps sob o lock. Isso funciona, mas cria acoplamento sutil (snapshot assume que o caller detém o lock). Se outro código chamar `snapshot()` sem lock, há race condition.
  **Fix:** Separar a responsabilidade: snapshot deve adquirir seu próprio lock, ou renomear para `snapshotLocked()` e documentar o pré-requisito.

- **[MEDIUM]** `internal/workflows/workflows.go:1073` — Arquivo muito grande.
  O arquivo workflows.go tem 1073 linhas e contém lógica de parsing de markdown, gerenciamento de workflows, execução com pipeline/fallback, e busca. Diversas responsabilidades em um único arquivo.
  **Fix:** Extrair a lógica de parsing de markdown para `workflows/parser.go` e a lógica de execução para `workflows/executor.go`. Isso melhoraria a manutenibilidade e testabilidade.

---

### Low (optional) — 4 issues

- **[LOW]** `internal/confidence/tracker.go:84-95` — Confidence scores hardcoded.
  Os scores do Wave 2 são inicializados estaticamente no código. Para Wave 3+, scores deveriam ser carregáveis de arquivo de configuração ou banco de dados para permitir atualizações sem recompilação.
  **Sugestão:** Adicionar `LoadFromConfig(path string)` para carregar scores de um YAML/JSON externo.

- **[LOW]** `api/grpcserver/server.go:56-57` — Comentário sobre ordenação de interceptors.
  O comentário "recovery first (outermost)" está conceitualmente invertido. No gRPC, interceptors em `ChainUnaryInterceptor` são executados na ordem da lista (primeiro = mais externo). O código está correto (Recovery primeiro = mais externo = captura panics de todos os handlers internos), mas o comentário "outermost" é ambíguo.
  **Sugestão:** "recovery first (outermost — catches panics from all downstream interceptors and handlers)".

- **[LOW]** `api/stream/types.go:19` — Constantes de evento SSE.
  `EventToken` é um alias para `EventResponse` com o mesmo valor `"response"`, o que pode causar confusão. Se ambos tem o mesmo propósito, manter apenas um.
  **Sugestão:** Remover `EventToken` ou documentar explicitamente por que o alias existe (backward compatibility).

- **[LOW]** `api/rest/handler/run.go:201` — `LogEvent` com `audit.DetailsJSON` aceita `interface{}`.
  O logging de auditoria usa `interface{}` para details, o que perde type safety. Se o `audit.DetailsJSON` espera JSON-serializável, deveria ter um type constraint.
  **Sugestão:** Restringir com `audit.DetailsJSON[T any](data T)` ou criar tipos específicos.

---

### Pontos Positivos

1. **Excelente cobertura de testes:** 6,319 linhas de código de teste cobrindo gRPC, SSE, runtime lifecycle, state machine transitions (21 transições testadas), benchmarks de FTS, search e vector. Todos passam com `-race`.

2. **Arquitetura limpa nos serviços gRPC:** Separação clara entre mapping (pbToDomain/domainToPb), service logic, e server setup. Cada arquivo tem responsabilidade única.

3. **WebSocket Hub bem projetado:** Event-loop pattern com canais, slow consumer drop, shutdown graceful com drain de conexões, e suporte a subscribe/unsubscribe por tópico.

4. **Tratamento consistente de erros gRPC:** Uso apropriado de `codes.InvalidArgument`, `codes.NotFound`, `codes.FailedPrecondition`, `codes.Internal` em todos os serviços gRPC.

5. **RecoveryInterceptor:** Proteção contra panics no gRPC server — essencial para produção.

6. **AAA pattern nos testes:** Arrange/Act/Assert em todos os testes de lifecycle e state machine (runtime_lifecycle_test.go, state_integration_test.go).

7. **SSE com suporte a structured data:** O `formatSSEEvent` lida inteligentemente com strings e objetos JSON, mantendo compatibilidade com o formato legacy.

8. **Prometheus sem dependência externa:** Implementação própria do formato text/plain sem bibliotecas externas, seguindo o princípio de dependências mínimas.

9. **Frontend componente AgentConfidenceCard:** Bom uso de `memo`, estados de loading/error/empty, progressbar acessível com `role="progressbar"`, e estilização responsiva com `useMobile()`.

10. **Flow charts nos testes MCP:** 1,167 linhas de testes para o servidor MCP, cobrindo todos os tools registrados.

---

### Resumo

| Severidade | Quantidade |
|------------|------------|
| 🔴 Critical | 2 |
| 🟡 High | 3 |
| 🟠 Medium | 5 |
| 🟢 Low | 4 |
| **Total** | **14** |

**Status geral de aprovação:** ⚠️ CONDITIONAL — Os 2 issues críticos e 3 issues high devem ser resolvidos antes do merge. Os 5 issues medium devem ser endereçados no próximo sprint. O código está bem estruturado, bem testado, e segue a maioria dos padrões Go idiomáticos e princípios SOLID.

**Code owners para cada issue:**
- Critical #1 (WebSocket InsecureSkipVerify): `cosca-specialist-frontend-component` + `cosca-specialist-backend-api`
- Critical #2 (XSS dangerouslySetInnerHTML): `cosca-specialist-frontend-component`
- High #1 (context.Background REST): `cosca-specialist-backend-api`
- High #2 (context.Background MCP): `cosca-specialist-backend-api`
- High #3 (TTL silent error): `cosca-specialist-backend-api`
