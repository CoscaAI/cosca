# ADR-002: Banco de dados **PostgreSQL** (relacional + JSONB) como banco único

> **Status:** Proposta 🔵 (aguarda aprovação do Don)
> **Owner:** cosca-architecture (Architecture Chief) | **Last Updated:** 2026-09-01
> **Projeto:** RIZOMAI — plataforma API-first de gestão de redes sociais
> **Referência (base):** `arquitetura-zernio.md` §3 (modelo de dados) e §11 R3; `integracoes-zernio.md` §5; ADR-003 (fila).

---

## 0. Contexto

O modelo central é hierárquico e relacional: `Team → Profile → Account → Post → PostTarget →
PublishAttempt`, mais API Keys, Webhooks, Media, Billing, Logs. Simultaneamente, há dados
semi-estruturados por plataforma (`platformSpecificData`, settings por rede) e payloads de webhook,
que num banco relacional puro gerariam esquemas rígidos. A Zernio usa MongoDB `[inferido]` e isso
vazou para o contrato (`_id` no body de webhook — risco R3 que devemos evitar). Precisamos de:
integridade referencial, transações (criar Post + N targets atomicamente), consultas analíticas
(analytics, billing), e flexibilidade para os dados específicos de cada rede.

## 1. Decisão

**PostgreSQL é o banco de dados único do RIZOMAI**, usado para dados transacionais (domínio),
fila de jobs (via River — ADR-003), metering de billing e analytics. Dados flexíveis por plataforma
vão em colunas/objetos **JSONB** com índices GIN/expressões quando necessário.

Regras de uso:
- **Relacional** para o grafo `Team/Profile/Account/Post/PostTarget/PublishAttempt` e entidades de
  suporte (API Keys, Webhooks, Billing, Logs, Media) — com FKs e transações.
- **JSONB** para: `platformSpecificData` de PostTarget (union tipada na API/spec, JSONB no banco),
  `settings` por plataforma em Account, payloads de delivery de webhook, metadados de mídia.
- **Validação na borda**: o JSONB é validado contra o schema OpenAPI na camada de API (spec é a
  fonte de verdade do formato, não o banco) — evita "JSONB bag" sem tipo.
- Extensões planejadas: `pgcrypto`/`pg_uuidv7` (IDs), `pg_cron` (jobs de limpeza/metering) e
  `pgvector` (futuro: analytics/IA de conteúdo) — mantendo o padrão zero-infra extra.

## 2. Por que PostgreSQL (e não NoSQL)

1. **O domínio é relacional por natureza.** `Post → N PostTarget → N PublishAttempt` exige
   integridade referencial e transação atômica: criar um post com 5 targets ou não criar nada.
   Modelar isso em documento único (Mongo) perde consultas, FKs e transações cross-doc.
2. **JSONB dá a flexibilidade NoSQL exatamente onde ela é útil** (dados por plataforma), sem
   sacrificar o resto. A maior "vantagem" do NoSQL (schema-free) vira desvantagem aqui: validar
   `platformSpecificData` no banco é impossível; validar na API + contrato OpenAPI é o correto.
3. **Analytics e billing são consultas agregadas** (`COUNT`/`GROUP BY` por conta/dia, `post.partial`
   por plataforma, funil de tentativas) — SQL/views/materialized views resolvem com um décimo do
   esforço de um aggregation pipeline.
4. **A fila (River) roda no mesmo Postgres** (ADR-003) → **uma única infraestrutura de estado** para
   startup enxuta; menos peças para operar/backupear.
5. **Custo**: Postgres gerenciado (Neon/Supabase free tier generoso) ou Docker self-hosted —
   encaixa em ~R$1.000/mês. Extensões (pgvector) evitam dependências futuras.

## 3. Consequências

**Prós**
- Integridade e transações para o fluxo de publicação (núcleo do produto).
- Uma infra: domínio + fila + metering + analytics no mesmo banco (ADR-003 reforça).
- JSONB para dados flexíveis por plataforma sem rigidez de schema.
- Ecossistema maduro: pgx (Go), migrations versionadas, tooling de backup/point-in-time.
- pgvector futuro para IA/analytics sem novo banco.

**Contras**
- Migrations exigem disciplina (golang-migrate/atlas, review de schema em PR).
- Linhas quentes (`Account`, `Post`) exigem índice/cache quando o volume crescer (gate futuro).
- JSONB sem schema estrito no banco — depende da disciplina de validação na API (regra §1).
- Ponto único de falha (mitigado: managed service com HA barato; self-hosted só no início).

## 4. Alternativas consideradas

1. **MongoDB** (usado pela Zernio) — schema-free e JSON nativo. **Rejeitado**: sem integridade
   referencial/transações cross-doc para o grafo central, analytics via aggregation pipeline mais
   caros de manter, e histórico de vazamento de `_id` no contrato (R3). Com JSONB perdemos ~nada.
2. **MySQL** — relacional equivalente. **Rejeitado**: suporte a JSON e extensões menos maduros
   (pgvector, pg_cron, partial indexes); Postgres vence em features pelo mesmo preço.
3. **DynamoDB** — **rejeitado**: modelagem single-table complexa para hierarquia + analytics,
   vendor lock AWS, custo por leitura/escrita; não cabe em startup LATAM multi-cloud.
4. **Banco único vs banco de fila separado** — ver ADR-003 (decisão: mesma instância no MVP,
   com schema/slot dedicado para jobs e gate de separação).

## 5. Referências

- `arquitetura-zernio.md` §3 (modelo de dados) e §11 R3 (`_id` vazando — contratos desacoplados).
- ADR-003 (River sobre Postgres), ADR-005 (spec-first valida o JSONB na borda).
- ADR-007 (modelo Post → PostTarget → PublishAttempt no relacional).

---

*ADR de Fase 1 — RIZOMAI. Decisão de stack, aguardando aprovação do Don.*
