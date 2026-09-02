# ADR-007: Modelo de publicação **fan-out** — `Post → PostTarget(status) → PublishAttempt` com agregado `partial`

> **Status:** Proposta 🔵 (aguarda aprovação do Don)
> **Owner:** cosca-architecture (Architecture Chief) | **Last Updated:** 2026-09-01
> **Projeto:** RIZOMAI — plataforma API-first de gestão de redes sociais
> **Referência (base):** `arquitetura-zernio.md` §3/§5 (modelo e fluxo) e §10 P0; `integracoes-zernio.md` §4.5 (falhas parciais).

---

## 0. Contexto

O coração do produto é "1 post → N redes com status por plataforma". O cliente precisa saber
**exatamente o que falhou onde**: `post.partial` (algumas redes publicaram, outras não) é o recurso
de 1ª classe que diferencia uma plataforma de integração de um "posta e reza". Isso exige modelar
desde o dia 1: um `Post` (conteúdo) → N `PostTarget` (cada um com `{accountId, platform}` e status
próprio) → N `PublishAttempt` (log append-only de cada tentativa). Retry e unpublish são **por
target**, nunca do post inteiro.

## 1. Decisão

**Modelar o pipeline de publicação como agregação de 3 entidades relacionais** (Postgres — ADR-002):

```
Post ──1:N──> PostTarget ──1:N──> PublishAttempt
```

- **`Post`** — o conteúdo agendado/único: `content`, `mediaUrls[]`, `platforms[]` (referência aos
  targets), `scheduledFor`, `timezone`, `status` (agregado), `createdBy`, `contentHash`
  (idempotência — ADR-005). **Status agregado derivado** dos targets:
  - `scheduled` → ainda não due; `publishing` → fan-out em andamento;
  - `published` → **todos** os targets publicados;
  - `partial` → **alguns** publicados, outros falharam (o caso mais comum na prática!);
  - `failed` → todos falharam; `cancelled` → cancelado antes da publicação.
- **`PostTarget`** — 1 linha por `(post, accountId, platform)`: `status`
  (`pending|scheduled|publishing|published|failed|skipped`), `platformSpecificData` (JSONB —
  union tipada na spec), `publishedUrl`, `externalPostId`, `lastError`. **Regra de ouro: o status do
  Post é uma função do status dos targets** (materializado para leitura, atualizado transacionalmente).
- **`PublishAttempt`** — log **append-only** de cada tentativa: `targetId`, `attempt#`, `startedAt`,
  `finishedAt`, `outcome` (`success|failed|timeout`), `error` (code + detalhe), `httpStatus`,
  `requestId`. Fonte da auditoria (`GET /posts/{id}/logs`).

### Comportamento

1. **Fan-out paralelo por target** (goroutines — ADR-001): cada target publica de forma **isolada**;
   falha em X não bloqueia Y. Se X falhar, X vira `failed` + PublishAttempt com erro; Y segue.
2. **Retry por target**: `POST /posts/{id}/retry` reprocessa **só os targets `failed`/`skipped`**
   (nunca re-publica os já `published` — idempotente via unique `(target_id, attempt)` e consulta ao
   estado atual antes de cada tentativa).
3. **Backoff**: retry automático de erros transitórios (`429, 500, 502, 503`) com backoff
   exponencial (5 s × 2^n, cap 5 min, máx 3 tentativas automáticas) — depois disso, `failed` aguardando
   retry manual ou webhook. Erros definitivos (`invalid_grant`, `duplicate`, `NO_LINKS`, quota) →
   **sem retry automático**, erro tipado no log.
4. **Verificação pós-erro**: para plataformas com "falha ambígua" (ex.: Instagram `2207051`
   "bloqueado mas pode ter publicado"), fazer **check de existência** antes de marcar `failed`
   (anti-duplicado no retry).
5. **Unpublish por plataforma**: `POST /posts/{id}/unpublish {platform, accountId}` — remove/deleta
   na rede, marca o target `unpublished` (quando a plataforma suporta; documentar por conector).
6. **Webhooks por evento** (ADR-009): `post.scheduled`, `post.partial`, `post.published`,
   `post.failed` — o `partial` é o mais importante (o cliente precisa agir: retry, alerta, seguir).

### Contrato (resumo — espelha ADR-005)

```jsonc
// POST /v1/posts
{
  "content": "Olá mundo",
  "platforms": [
    { "platform": "x", "accountId": "account_123" },
    { "platform": "linkedin", "accountId": "account_456",
      "platformSpecificData": { "firstComment": "...", "visibility": "PUBLIC" } }
  ],
  "mediaUrls": ["https://cdn.rizomai.app/media_..."]
}
// resposta: { "data": { "id": "post_...", "status": "scheduled", "targets": [ ... ] } }

// GET /v1/posts/{id} → { "data": { "id": "...", "status": "partial",
//   "targets": [ { "platform": "x", "status": "published", "publishedUrl": "..." },
//                 { "platform": "linkedin", "status": "failed", "lastError": { "code": "...", "error": "..." } } ] } }
```

## 2. Consequências

**Prós**
- **Transparência total** ("o que falhou, onde e por quê") — o recurso que fideliza dev/agência.
- Retry seguro e idempotente por target; unpublish granular.
- Logs append-only = auditoria e suporte sem "chute".
- Base natural para analytics por plataforma e billing (account-day usa o estado real das contas).
- `partial` como cidadão de 1ª classe → webhook `post.partial` dispara ação no cliente.

**Contras**
- 3 entidades + estados a gerenciar (mais complexo que um "array de status no documento").
- Status agregado precisa ser atualizado transacionalmente com os targets (mitigado: recomputar por
  query/trigger no fim do fan-out; materialização para leitura).
- Eventual-consistency entre fan-out e estado (expor no contrato: `publishing` e webhooks de
  progresso; nunca prometer atomicidade cross-rede — impossível).
- Verificação pós-erro por plataforma é custo extra por conector (regra §1.4).

## 3. Alternativas consideradas

1. **Post com array de status embutido** (documento único, estilo Mongo) — **rejeitado**: perde
   consultas/transações, retry por target vira manipulação de array, logs por tentativa explodem o
   documento (ADR-002 já resolve com relacional + JSONB).
2. **Publicar tudo-ou-nada (transação cross-rede)** — **rejeitado**: impossível e indesejável
   (rede X pode estar fora sem impedir Y); o `partial` é o valor, não o bug.
3. **Sem logs por tentativa (só status final)** — **rejeitado**: depuração impossível; P0 da
   mineração exige log por tentativa.
4. **Status agregado como campo manual** — **rejeitado**: drifta; o status agregado é **derivado**
   dos targets (materializado, nunca editado à mão).

## 4. Referências

- `arquitetura-zernio.md` §3 (entidades), §5 (fluxo: fila → fan-out → agregado → logs → webhooks), §10 P0.
- `integracoes-zernio.md` §4.4/4.5 (códigos de erro por plataforma; falhas parciais; 2207051).
- ADR-002 (relacional + JSONB), ADR-003 (River p/ jobs do fan-out), ADR-005 (contrato), ADR-009 (webhooks).

---

*ADR de Fase 1 — RIZOMAI. Decisão de stack, aguardando aprovação do Don.*
