# ADR-003: Fila de jobs **durável sobre PostgreSQL (River)** — sem Redis/RabbitMQ/SQS no MVP

> **Status:** Proposta 🔵 (aguarda aprovação do Don)
> **Owner:** cosca-architecture (Architecture Chief) | **Last Updated:** 2026-09-01
> **Projeto:** RIZOMAI — plataforma API-first de gestão de redes sociais
> **Referência (base):** `arquitetura-zernio.md` §5 (fluxo de publicação), §10 P0; `integracoes-zernio.md` §4.3 (backoff/retry).

---

## 0. Contexto

O pipeline de publicação precisa de: **fan-out** (1 post → N targets processados em paralelo),
**agendamento** ("publicar 14/09 às 10h"), **retry com backoff por target**, **entrega de webhooks**
com retry, e jobs de infra (refresh de tokens, limpeza de mídia, metering diário). Restrições:
custo inicial baixo (R$1.000/mês), operação mínima, durabilidade (um post agendado **não pode se
perder**). Opções avaliadas: Redis (BullMQ/Asynq), RabbitMQ, SQS, pub/sub nativo.

## 1. Decisão

**A fila de trabalho do RIZOMAI será uma job queue durável sobre PostgreSQL usando a biblioteca Go
`River`** (jobs atômicos via `SKIP LOCKED`, retry com backoff, agendamento com atraso/cron, e
observabilidade via SQL). **Sem Redis, RabbitMQ ou SQS no MVP.** Redis (ou equivalente) entra
depois, apenas como cache/rate-limit/pub-sub — **nunca como fila** — se a telemetria justificar.

Regras:
- **Transacional com o domínio**: criar um Post agendado = mesma transação que enfileira o job de
  publicação. Se a transação commitou, o job existe. Zero janela de perda.
- **Dead-letter + retry**: política por job (ex.: publicação: 5 tentativas, backoff exponencial
  base 5 s cap 5 min; webhook: backoff 5 s × 2^n, janela 24 h, mesmo event id — ADR-009).
- **Workers**: processos separados (`workers/cmd/*`) consumindo a mesma base — escala horizontal
  simples (mais réplicas = mais consumidores) sem broker dedicado.
- **Schemas separados**: `public` (domínio) e `river` (jobs) na mesma instância no MVP; gate para
  separar em instância própria quando houver contenção de IOPS.

### Por que River (e não BullMQ/Asynq)

BullMQ é Node (core é Go — ADR-001) e Asynq (Redis) adiciona **infraestrutura crítica de estado**
(se o Redis cai, a fila para) + custo do managed. River usa Postgres, que já temos (ADR-002):
uma peça a menos, e o job é transacional com os dados que ele processa.

## 2. Comparação das alternativas

| Critério | **River (Postgres)** | Redis/Asynq | RabbitMQ | SQS |
|---|---|---|---|---|
| Infra extra | **nenhuma** | +1 serviço (managed/self-host) | +1 broker (operar) | vendor AWS (managed) |
| Transacional com o domínio | **sim** (mesmo commit) | não | não | não |
| Agendamento "14/09 às 10h" | **nativo** (schedulable) | nativo (delay) | via plugin/DLX | delay máx 15 min → exige scheduler extra |
| Retry/backoff/DeadLetter | **nativo** | nativo | nativo | nativo |
| Custo inicial | **R$ 0** | ~R$ 15–50/mês managed | operação própria ou SaaS | free tier 1M req/mês (mas lock) |
| Escala | boa p/ dezenas de k jobs/dia | excelente | excelente | excelente |
| Lock de cloud | nenhum | nenhum | nenhum | **AWS** |

**Escala esperada do MVP**: poucos milhares de jobs/dia (posts + webhooks + refresh). Postgres
atende com folga; o gate para broker dedicado só se abre com volume real medido (§4).

**Pub/sub nativo** (LISTEN/NOTIFY ou Redis pub/sub): **não é fila durável** — é notificação. Usamos
LISTEN/NOTIFY apenas como *trigger de acordar worker* (ex.: job agendado virou "due"), nunca como
transporte de trabalho.

## 3. Consequências

**Prós**
- **Uma única infraestrutura de estado** (Postgres): menor custo, menor superfície de falha.
- **Zero perda de jobs agendados** (transacional com o domínio) — requisito de produto.
- Retry/backoff/cron nativos sem operar broker; observabilidade via SQL (queries de fila).
- Escala horizontal trivial (workers consumidores) sem mudar de arquitetura.

**Contras**
- Throughput menor que Redis/RabbitMQ (irrelevante na escala de startup; medir).
- Fila e domínio competem por IOPS no mesmo Postgres (mitigado: schema dedicado + pool separado;
  gate de separação).
- Não é stream de eventos (para eventos de domínio, usamos webhooks + LISTEN/NOTIFY; se um dia
  precisarmos de event-streaming de alta taxa, o gate §4 reabre a decisão).
- River é lib jovem (mas em produção em Go — versão 1.x estável; manter pinned).

## 4. Gates para reabrir a decisão (documentados agora)

- **G1**: > 50.000 jobs/dia processados, **ou**
- **G2**: p95 de scheduling (enfileirar → worker pegar) > 30 s, **ou**
- **G3**: necessidade real de event-streaming de alta taxa / replay de eventos (aí avaliar
  Redis Streams ou Kafka — não RabbitMQ), **ou**
- **G4**: contenção de IOPS do Postgres impactando o p95 de escrita transacional (aí: instância
  Postgres dedicada só para River, antes de qualquer broker).

## 5. Referências

- `arquitetura-zernio.md` §5 (fila interna, status scheduled→due, retry por plataforma).
- `integracoes-zernio.md` §4.3 (retry: retryable = 429,500,502,503; backoff 5 s × 2^n cap 30 s).
- ADR-002 (Postgres único), ADR-007 (fan-out por PostTarget), ADR-009 (retry de webhook c/ mesmo id).

---

*ADR de Fase 1 — RIZOMAI. Decisão de stack, aguardando aprovação do Don.*
