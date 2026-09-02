# RIZOMAI

Plataforma API-first de gestão de redes sociais — **"um post → todas as redes"**.

Monorepo Go (Fase 1 — scaffold). As decisões de arquitetura estão nos ADRs em
`docs/projetos/rizomai/adr/` (workspace do Cosca). Eles são a lei do projeto.

## Estrutura (ADR-004)

```
rizomai/
├── openapi/
│   └── rizomai.yaml          # fonte única da verdade do contrato (ADR-005)
├── api/                      # gateway HTTP (binário 1)
│   ├── cmd/rizomai-api/      #   entrypoint
│   ├── handlers/             #   handlers por recurso (espelha a spec)
│   ├── middleware/           #   logging (auth/rate-limit/idempotency: Fase 2)
│   └── routes.go             #   montagem do roteador
├── internal/                 # código compartilhado (api + workers)
│   ├── domain/               # entidades centrais (Profile, Account, Post…)
│   └── store/                # Postgres: repos por agregado + migrations (Fase 2)
├── workers/                  # binários de worker (compartilham internal/)
│   ├── cmd/publish-worker/   #   fan-out de publicação via River (Fase 2)
│   └── cmd/webhook-worker/   #   delivery de webhooks (Fase 2)
└── migrations/               # SQL versionado (schema public + river) — Fase 2
```

> Fase 2+: `internal/{platform,oauth,publish,media,webhook,billing,analytics}`,
> `web/` (dashboard), `sdk/` (SDKs gerados), `deploy/`, `docs/` — conforme ADR-004.

## Requisitos

- **Go 1.22+** (testado com 1.26)
- **Node.js 20+** — apenas para validar o OpenAPI (`make openapi-lint`)

## Como rodar

```bash
make run-api      # gateway HTTP em http://localhost:8080 (env PORT opcional)
                  # → health check: GET /healthz → 200 {"status":"ok"}
make run-worker   # publish-worker (scaffold — fila River chega na Fase 2)
```

## Qualidade

```bash
make test         # go test ./...
make vet          # go vet ./...
make lint         # vet (lint mínimo sem ferramentas globais)
make build        # compila os 3 binários em bin/
make openapi-lint # valida a spec com @redocly/cli (spec-first — ADR-005)
make gen          # (Fase 2) geração de SDKs a partir da spec
```

CI (`.github/workflows/ci.yml`): `go build` + `go vet` + `go test` para o Go e
validação do OpenAPI com `@redocly/cli lint` em todo push/PR.

## Contrato (ADR-005)

`openapi/rizomai.yaml` (OpenAPI 3.1, base path `/v1`) é a **fonte única da
verdade**. Convenções obrigatórias:

- Envelope de sucesso `{ "data": ... }`; paginação `{ data, page, limit, total }`
- Envelope de erro `{ "code", "error", "details" }` (`code` = UPPER_SNAKE:
  `BAD_REQUEST`, `UNAUTHORIZED`, `FORBIDDEN`, `NOT_FOUND`, `VALIDATION_ERROR`,
  `CONFLICT`, `RATE_LIMITED`, `PAYMENT_REQUIRED`, `INTERNAL_ERROR`)
- Header `Idempotency-Key` (UUID) nas mutações — mesma chave = mesma resposta em 24h
- IDs com prefixo estável: `profile_`, `account_`, `post_`, `wh_`, `media_`
- Auth: API key `sk_...` via `Authorization: Bearer` (ADR-006)

## Modelo de domínio (ADR-007)

```
Post ──1:N──> PostTarget ──1:N──> PublishAttempt
```

O status agregado de `Post` (`scheduled|publishing|published|partial|failed|cancelled`)
é **derivado** do status dos targets — nunca editado à mão. `partial` (alguns
targets publicados, outros não) é um recurso de 1ª classe. Ver `internal/domain/`.

## Notas de Fase 1 → Fase 2

- Banco (pgx + migrations `public`/`river`) e fila (River) são deliberadamente
  **ausentes** do scaffold; a ordem de implementação está nos `main.go`.
- A spec v0.1 é monolítica; quando a superfície crescer, fatiar em
  `openapi/parts/` (gate anti-endpoint-sprawl — ADR-005 §1.7).
