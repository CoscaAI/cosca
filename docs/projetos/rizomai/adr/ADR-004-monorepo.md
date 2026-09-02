# ADR-004: Monorepo único com separação por responsabilidade (API core + workers + spec + web + SDKs gerados)

> **Status:** Proposta 🔵 (aguarda aprovação do Don)
> **Owner:** cosca-architecture (Architecture Chief) | **Last Updated:** 2026-09-01
> **Projeto:** RIZOMAI — plataforma API-first de gestão de redes sociais
> **Referência (base):** `arquitetura-zernio.md` §8 (estrutura de SDKs), §11 R1 (drift de SDKs); `integracoes-zernio.md` §5.2/5.3 (organização de SDKs); ADR-001 (Go).

---

## 0. Contexto

Precisamos organizar: API HTTP (gateway), workers (publish, webhook delivery, oauth refresh,
analytics sync, metering), a spec OpenAPI canônica (fonte de verdade — ADR-005), geradores de SDKs,
dashboard web (consumidor da API), docs (llms.txt, guias) e deploy. A Zernio opera **multi-repo**
(27 repos) com drift comprovado entre SDKs hand-written e gerados (R1) — queremos evitar isso.
Não somos um produto Cosca: o monorepo RIZOMAI é **independente**, mas pode reusar padrões do Cosca.

## 1. Decisão

**Monorepo único `rizomai/`** (Go workspace + `web/` em Next.js via pnpm), com módulos por
componente, deploy de binários separados e a **spec OpenAPI versionada como cidadão de 1ª classe**.

```
rizomai/
├── openapi/                      # ✅ fonte de verdade do contrato (ADR-005)
│   ├── rizomai.yaml              #   spec principal (OpenAPI 3.1)
│   └── parts/                    #   fragmentos por recurso (profiles, posts, media, webhooks, billing…)
│                                 #   → montados por script (evita spec gigante monolítica)
├── api/                          # gateway HTTP (binário 1)
│   ├── cmd/rizomai-api/main.go
│   ├── handlers/                 # 1 arquivo por recurso (espelha openapi/parts)
│   ├── middleware/               # auth (API key), rate-limit, idempotency, recovery
│   └── routes.go
├── internal/                     # código compartilhado (compilado em api + workers)
│   ├── domain/                   # entidades + regras (Team, Profile, Account, Post,
│   │                             #   PostTarget, PublishAttempt, Media, Webhook…)
│   ├── platform/                 # conectores por rede (twitter, linkedin, telegram…) — 1 pkg por rede
│   ├── oauth/                    # OAuth broker (ADR-006)
│   ├── publish/                  # fan-out e agregado de status (ADR-007)
│   ├── media/                    # presign + validador por plataforma (ADR-008)
│   ├── webhook/                  # delivery, HMAC, logs, redelivery (ADR-009)
│   ├── billing/                  # metering + gateway Stripe/PIX (ADR-010)
│   ├── analytics/                # agregados (views SQL + sync)
│   └── store/                    # Postgres: repos por agregado + migrations
├── workers/                      # binários de worker (compartilham internal/)
│   ├── cmd/publish-worker/       #   consome jobs de publicação (River — ADR-003)
│   ├── cmd/webhook-worker/       #   entrega webhooks com retry (ADR-009)
│   ├── cmd/oauth-refresh/        #   refresh de tokens + health check (ADR-006)
│   ├── cmd/analytics-sync/       #   sync de posts nativos (~90 min, opcional MVP+)
│   └── cmd/metering/             #   agregação account-day p/ billing (ADR-010)
├── web/                          # dashboard (Next.js/TS) — consome a API via SDK gerado
├── sdk/                          # SDKs de cliente
│   ├── generators/               #   config do OpenAPI Generator (templates, overrides)
│   └── gen/                      #   saída gerada (typescript/, python/, go/) — artefato de CI,
│                                 #   NUNCA editado à mão (guard no CI: diff limpo)
├── docs/                         # llms.txt, agent-quickstart, guias, changelog
├── deploy/                       # docker-compose, Dockerfile (multi-stage), IaC
├── migrations/                   # SQL versionado (schema public + river)
└── Makefile / Taskfile           # comandos: gen, build, test, lint, dev
```

Regras de ouro:
1. **`openapi/` manda em tudo**: handlers, SDKs, testes de contrato, docs e o dashboard são
   (re)gerados/validados contra a spec — nunca o contrário.
2. **Um binário por responsabilidade** (`api`, `publish-worker`, `webhook-worker`…) compartilhando
   `internal/` — deploy independente, escala independente.
3. **SDK gerado é artefato**: PR que alterar `sdk/gen/` sem alterar `openapi/` falha no CI (diff
   guard). Elimina o drift que matou a Zernio (R1).
4. **`platform/` por rede**: cada conector é um pacote fechado (validação, limites, erros mapeados,
   rate-limit) — a adição de uma rede nova não toca o core.
5. **docs/ ao lado da spec** para llms.txt / agent-quickstart (canal AI agents — produto §5.2).

## 2. Por que monorepo (e não multi-repo)

1. **Versionamento atômico**: mudar contrato + handler + SDK + web de uma vez em um único PR.
2. **CI único e rápido**: build/test por pacote afetado (cache Go/pnpm), guard de contrato em um
   lugar só.
3. **Custo de coordenação zero** para um time pequeno (1–3 devs): multi-repo gastaria tempo com
   releases/versionamento cruzado — exatamente o que causou drift na Zernio.
4. **Espelha o padrão do Cosca** (monorepo Go) → reuso de ferramentas/convenções do Don.

## 3. Consequências

**Prós**
- Contrato → SDK → consumidor sempre coerentes (anti-R1).
- Deploy granular (só o binário afetado) com build compartilhado.
- Adição de plataforma nova = 1 pacote + 1 fragmento de spec, sem tocar core.
- Onboarding curto (tudo num repositório), docs ao lado do código.

**Contras**
- CI cresce (mitigado: build/test seletivo por caminho; cache Go/pnpm).
- Acesso amplo ao código (mitigado: ownership por pacote via CODEOWNERS).
- `sdk/gen/` incha o repo (mitigado: SDKs publicados a partir do CI; a pasta pode virar
  submodule/artifact quando a publicação amadurecer — decisão futura documentada).

## 4. Alternativas consideradas

1. **Multi-repo (padrão Zernio)** — **rejeitado**: 27 repos criam coordenação e drift; R1 provou o
   custo. (Exceção possível: publicar SDKs em repos separados **a partir** do monorepo, via CI —
   mantém o monorepo como fonte e o repo do SDK como vitrine.)
2. **`apps/` + `packages/` (turborepo/nx)** — bom para muitos serviços independentes; aqui só há 1
   API + workers que **compartilham** `internal/` — turborepo adicionaria complexidade sem ganho.
3. **Splitting de SDK por repo gerado com push automático** — aceito como evolução do §3/Contras,
   não como estrutura inicial.

## 5. Referências

- `arquitetura-zernio.md` §8 e §11 R1 (estrutura SDKs; drift = antipadrão a evitar).
- `integracoes-zernio.md` §5.2/5.3 (organização resource × operation que espelha o REST).
- ADR-001 (Go workspace), ADR-005 (spec-first), ADR-007 (fan-out), ADR-009 (webhooks).

---

*ADR de Fase 1 — RIZOMAI. Decisão de stack, aguardando aprovação do Don.*
