# RIZOMAI

Plataforma API-first de gestão de redes sociais — **"um post → todas as redes"**.

Monorepo Go — **Fase 2**: banco PostgreSQL real, autenticação por API key e
handlers do núcleo validados contra a spec. As decisões de arquitetura estão nos
ADRs em `docs/projetos/rizomai/adr/` (workspace do Cosca). Eles são a lei do projeto.

## Estrutura (ADR-004)

```
rizomai/
├── openapi/
│   └── rizomai.yaml          # fonte única da verdade do contrato (ADR-005)
├── api/                      # gateway HTTP (binário 1)
│   ├── cmd/rizomai-api/      #   entrypoint (migrations + graceful shutdown)
│   ├── handlers/             #   handlers por recurso (profiles, posts…)
│   ├── middleware/           #   auth (API key), rate-limit, logging
│   ├── respond/              #   envelope {data} / {code,error,details} da spec
│   └── routes.go             #   montagem do roteador + chain de middleware
├── internal/
│   ├── domain/               # entidades, validação, IDs prefixados, status (ADR-007)
│   ├── store/                # Postgres: pgxpool + repositórios + migrations EMBEDDED
│   │   └── migrations/       #   SQL versionado (golang-migrate, single-binary)
│   └── queue/                # fila de jobs (stub Fase 2 → River na Fase 3)
├── workers/                  # binários de worker (ADR-004)
│   ├── cmd/publish-worker/   #   fan-out de publicação (scaffold — Fase 3)
│   └── cmd/webhook-worker/   #   delivery de webhooks (scaffold — Fase 3)
├── cmd/seed/                 # bootstrap dev: team + API key + contas fictícias
├── deploy/
│   └── docker-compose.yml    # Postgres 16 local
└── .env.example              # variáveis de ambiente documentadas
```

> Fase 3+: `internal/{platform,oauth,publish,media,webhook,billing,analytics}`,
> `web/` (dashboard), `sdk/` (SDKs gerados), `docs/` — conforme ADR-004.

## Requisitos

- **Go 1.22+** (testado com 1.26)
- **Docker** — para o Postgres local (`make db-up`)
- **Node.js 20+** — apenas para validar o OpenAPI (`make openapi-lint`)

## Como rodar (primeira vez)

```bash
# 1. Suba o banco (Postgres 16)
make db-up

# 2. Copie o env e ajuste se preciso
cp .env.example .env

# 3. Suba a API (aplica migrations automaticamente no boot)
#    no PowerShell: $env:DATABASE_URL="postgres://rizomai:rizomai@localhost:5433/rizomai?sslmode=disable"; go run ./api/cmd/rizomai-api
make run-api

# 4. Bootstrap de dev: cria team + API key + profile + contas fictícias
#    e IMPRIME a API key (sk_live_...) — guarde para os requests abaixo.
make seed
```

Se `DATABASE_URL` não estiver definida, a API para com mensagem clara.

## Como usar

```bash
# health check (público — inclui status do banco)
curl http://localhost:8080/healthz
# → 200 {"status":"ok","db":"ok"}

# autenticado (API key do seed)
KEY="sk_live_..."   # valor impresso pelo make seed

# criar profile
curl -s -X POST http://localhost:8080/v1/profiles \
  -H "Authorization: Bearer $KEY" -H "Content-Type: application/json" \
  -H "Idempotency-Key: $(uuidgen)" \
  -d '{"name":"Minha Agência"}'

# criar post (usa os account_... criados pelo seed)
curl -s -X POST http://localhost:8080/v1/posts \
  -H "Authorization: Bearer $KEY" -H "Content-Type: application/json" \
  -d '{"content":"Olá mundo","platforms":[{"platform":"x","accountId":"account_..."}]}'
# → 201 {data: {id, status, targets:[{platform,status,...}]}}
```

Endpoints implementados na Fase 2: `GET /v1/profiles`, `POST /v1/profiles`,
`GET /v1/profiles/{id}`, `GET /v1/posts`, `POST /v1/posts`, `GET /v1/posts/{id}`.
`GET /v1/connect/{platform}` responde 501 (OAuth broker é Fase 3). Demais rotas
da spec v0.1 (accounts, media, webhooks) ainda não estão registradas.

## Qualidade

```bash
make test         # go test ./...  (auth, rate-limit, validação, status, store/pgxmock)
make vet          # go vet ./...
make lint         # vet (lint mínimo sem ferramentas globais)
make build        # compila os binários em bin/
make openapi-lint # valida a spec com @redocly/cli (spec-first — ADR-005)
make gen          # (Fase 3) geração de SDKs a partir da spec
```

CI (`.github/workflows/ci.yml`): `go build` + `go vet` + `go test` para o Go e
validação do OpenAPI com `@redocly/cli lint` em todo push/PR.

## Variáveis de ambiente

| Variável | Default | Descrição |
|---|---|---|
| `DATABASE_URL` | — (obrigatória) | DSN Postgres (`postgres://user:pass@host:5432/db?sslmode=disable`) |
| `PORT` | `8080` | Porta do gateway |
| `API_KEY_PEPPER` | `dev-pepper` | Pepper do hash de API keys (trocar fora de dev) |
| `RATE_LIMIT_PER_MIN` | `60` | Token bucket por tenant (req/min) |
| `SKIP_MIGRATIONS` | `0` | `1` pula migrations no boot (gerência externa) |

## Contrato (ADR-005)

`openapi/rizomai.yaml` (OpenAPI 3.1, base path `/v1`) é a **fonte única da
verdade**. Convenções obrigatórias:

- Envelope de sucesso `{ "data": ... }`; paginação `{ data, page, limit, total }`
- Envelope de erro `{ "code", "error", "details" }` (`code` = UPPER_SNAKE:
  `BAD_REQUEST`, `UNAUTHORIZED`, `FORBIDDEN`, `NOT_FOUND`, `VALIDATION_ERROR`,
  `CONFLICT`, `RATE_LIMITED`, `PAYMENT_REQUIRED`, `NOT_IMPLEMENTED`, `INTERNAL_ERROR`)
- Header `Idempotency-Key` (UUID) nas mutações — mesma chave = mesma resposta em 24h
- Dedup por content-hash: mesmo `(team, content+media+targets)` em 24h → `409` + `existingPostId`
- IDs com prefixo estável: `profile_`, `account_`, `post_`, `wh_`, `media_`
- Auth: API key `sk_...` via `Authorization: Bearer` (ADR-006); rate limit com
  headers `X-RateLimit-Limit/Remaining/Reset` e `Retry-After`

## Modelo de domínio (ADR-007)

```
Post ──1:N──> PostTarget ──1:N──> PublishAttempt
```

O status agregado de `Post` (`scheduled|publishing|published|partial|failed|cancelled`)
é **derivado** do status dos targets — nunca editado à mão (`domain.DerivePostStatus`).
`partial` (alguns targets publicados, outros não) é um recurso de 1ª classe.
Criação de post é **transacional**: Post + N Targets no mesmo commit (ADR-002/003).

## Segurança (ADR-006)

- A API key é armazenada apenas como **SHA-256(pepper ‖ key)** — nunca plaintext.
- O cliente nunca recebe tokens de rede social (OAuth broker é Fase 3).
- Pepper via env `API_KEY_PEPPER`; chaves com prefixo `sk_live_` (identificação
  visual via `key_prefix`).

## Notas de Fase 2 → Fase 3

- **Fila**: `internal/queue` é um stub síncrono (loga "publicando em {platform}"
  e marca target `published`). Fase 3: River sobre Postgres, transacional com o
  domínio (ADR-003) e conectores reais em `internal/platform/`.
- **Migrations** ficam em `internal/store/migrations/` (não na raiz) por
  exigência do `//go:embed` (single-binary — ADR-001); mesmo conteúdo SQL.
- A spec v0.1 é monolítica; quando a superfície crescer, fatiar em
  `openapi/parts/` (gate anti-endpoint-sprawl — ADR-005 §1.7).
