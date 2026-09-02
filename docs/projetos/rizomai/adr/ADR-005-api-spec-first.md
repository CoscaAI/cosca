# ADR-005: API **spec-first com OpenAPI 3.1** — /v1, envelope de erro padrão, idempotência em 2 camadas, SDKs gerados

> **Status:** Proposta 🔵 (aguarda aprovação do Don)
> **Owner:** cosca-architecture (Architecture Chief) | **Last Updated:** 2026-09-01
> **Projeto:** RIZOMAI — plataforma API-first de gestão de redes sociais
> **Referência (base):** `arquitetura-zernio.md` §4 (convenções), §11 R1/R2/R10; `integracoes-zernio.md` §4 (erros) e §5.6 (checklist SDKs); `produto-zernio.md` §5.2 (AI-first).

---

## 0. Contexto

A API é o produto. O público (devs + AI agents) depende de um contrato **estável, consistente e
gerável**: SDKs em várias linguagens, docs sempre atuais, webhooks, testes de contrato. A Zernio
provou o caminho (spec OpenAPI pública, SDKs gerados) **e** os erros (spec de 2,4 MB = endpoint
sprawl R2; envelope de resposta inconsistente `{data}` vs `{posts}` vs `{post}` — R10; drift de SDKs
hand-written × gerados — R1). Nosso objetivo: spec enxuta, consistente e como **única fonte de
verdade** desde o dia 1.

## 1. Decisão

**OpenAPI 3.1 (YAML) em `openapi/rizomai.yaml` é a fonte única de verdade do contrato.** Tudo o
mais deriva dela: handlers validados contra a spec, SDKs gerados, docs, testes de contrato.

1. **Versionamento no path — `/v1`** (imutável). Breaking changes **só** em `/v2`; deprecação com
   `deprecated: true` + changelog + janela mínima antes da remoção. Nunca reusar `/v1`.
2. **Envelope de resposta único**: `{ "data": ... }` em toda operação (sucesso); paginação com
   `{ data, page, limit, total }`. **Envelope de erro único** em todo 4xx/5xx:
   ```json
   { "code": "RATE_LIMITED", "error": "Mensagem humana", "details": {} }
   ```
   - `code` = machine-readable `UPPER_SNAKE` (contrato: `BAD_REQUEST`, `UNAUTHORIZED`,
     `FORBIDDEN`, `NOT_FOUND`, `VALIDATION_ERROR`, `RATE_LIMITED`, `PAYMENT_REQUIRED`,
     `CONFLICT`, `INTERNAL_ERROR`…), `details` = contexto (ex.: `{ "fields": {...} }`).
3. **Idempotência em 2 camadas** (obrigatório em `POST`):
   - (a) Header **`Idempotency-Key`** (UUID do cliente): responde a mesma resposta para a mesma
     key dentro da janela (24 h), armazenada com o request hash.
   - (b) **Content-hash dedup**: mesmo `(team, content + media + targets)` em 24 h → `409` +
     `{ "existingPostId": "post_..." }` (dedup de clique duplo/retry real).
4. **IDs próprios com prefixo estável**: `team_...`, `profile_...`, `account_...`, `post_...`,
   `wh_...`, `key_...` — **nunca** `_id`/ObjectId no contrato (anti-R3).
5. **`platformSpecificData` como union tipada** (oneOf/discriminator por plataforma) no spec —
   validação server-side por plataforma, sem vazar schema interno (anti-R11).
6. **Geração de SDKs via pipeline**: OpenAPI Generator (ou hey-api para TS) no CI (ADR-004 §3) —
   TypeScript, Python e Go primeiro; o resto (Rust/.NET/PHP/Java/Ruby) quando a demanda pedir.
   Nenhum SDK hand-written (anti-R1).
7. **Gates anti-endpoint-sprawl** (anti-R2): spec montada de fragmentos por recurso
   (`openapi/parts/`), regra "1 recurso = 1 fragmento", review obrigatório quando a superfície
   crescer, e limite de operações por recurso (máx. ~15) — quando estourar, é sinal de modelagem errada.

### Modelo de contrato (resumo)

| Recurso | Rotas-chave |
|---|---|
| Profiles | `GET/POST /profiles`, `GET/PUT/DELETE /profiles/{id}` |
| Accounts | `GET /accounts`, `GET/PUT/DELETE /accounts/{id}`, `PATCH /accounts/{id}` (move), `GET /accounts/health` |
| Connect | `GET /connect/{platform}?profileId=`, callback server-side, seleção de página/org, clone-connection |
| Posts | `GET/POST /posts`, `GET/PUT/DELETE /posts/{id}`, `POST /posts/{id}/retry`, `POST /posts/{id}/unpublish`, `GET /posts/{id}/logs` |
| Media | `POST /media/presign`, `POST /media/upload` (direto) |
| Webhooks | `GET/POST/PUT/DELETE /webhooks/settings`, `POST /webhooks/test`, `GET /webhooks/logs`, redelivery |
| Usage/Billing | `GET /usage` (lastReset + billingAnchorDay) |
| API Keys | `GET/POST /api-keys`, `DELETE /api-keys/{keyId}`, `GET /auth/verify` |

## 2. Consequências

**Prós**
- **1 spec → N SDKs sem drift**; docs sempre atuais; testes de contrato em CI.
- Contrato estável + envelope uniforme = base para SDKs multi-linguagem e AI agents (llms.txt
  aponta para a spec — canal de distribuição).
- Erros machine-readable → SDKs com helpers (`isAuthError()`, `isRateLimited()`) como na Zernio.
- Idempotência em 2 camadas previne duplicação real e é auditável (produto confiável).

**Contras**
- **Trabalho inicial**: spec completa antes do código (paradigma novo p/ quem vem de code-first).
- Generators exigem overrides/templates próprios (investimento inicial em `sdk/generators/`).
- Disciplina necessária para manter spec = código (guard no CI: diff check).
- Risco de spec crescer (mitigado pelos gates §1.7 + fragmentos por recurso).

## 3. Alternativas consideradas

1. **Code-first (tsoa/OpenAPI gerada do código)** — docs e SDKs sempre "atrasados" em relação ao
   contrato real; difícil validar quebradores de contrato. **Rejeitado**: spec-first é o coração do
   produto (e o mercado já escolheu — Zernio, Stripe, Twilio).
2. **gRPC/Protobuf** — ótimo internamente, mas contrato público REST é o padrão para SDKs
   multi-linguagem, webhooks e AI agents; gRPC pode ser camada interna futura, não substitui a spec.
3. **SDKs hand-written** (Python/Node como a Zernio fez) — **rejeitado**: R1 provou drift.
4. **GraphQL** — bom p/ frontends, ruim p/ SDKs por linguagem, cache HTTP e webhooks; REST+OpenAPI
   é mais simples para o público-alvo dev/agente. **Rejeitado**.

## 4. Referências

- `arquitetura-zernio.md` §4/§10 (P0/P1), §11 R1/R2/R3/R10/R11.
- `integracoes-zernio.md` §4 (envelope de erro, códigos) e §5.6 (checklist SDK).
- ADR-004 (monorepo: `openapi/` como fonte), ADR-007 (posts multi-target), ADR-009 (webhooks).

---

*ADR de Fase 1 — RIZOMAI. Decisão de stack, aguardando aprovação do Don.*
