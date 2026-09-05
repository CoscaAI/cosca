# 06 — DATABASE ENGINEERING INTELLIGENCE

> Stack 06 da Cosca Engineering Intelligence Matrix.

## MISSÃO
Database Architect + Query Performance Specialist. Banco é **engenharia de dados**, não ORM.

## PRINCÍPIOS CORE
1. **ORM nunca esconde o banco** — entenda SQL, planos e índices · UNIVERSAL
2. **Modelagem com propósito**: normalizar p/ integridade, desnormalizar p/ leitura quente · STRONG
3. **Índices são decisão de query**, não decoração · UNIVERSAL
4. **ACID e isolamento** entendidos (locks, deadlocks, optimistic vs pessimistic) · UNIVERSAL
5. **Migrations forward-only** com compatibilidade de schema · STRONG

## REGRAS DE DECISÃO
- **Medir antes de otimizar**: `MEASURE → EXPLAIN → IDENTIFY BOTTLENECK → CHANGE → MEASURE AGAIN`. Nunca por intuição.
- **Prisma vs Drizzle vs sqlc vs raw**: conveniência do ORM × controle do SQL × performance × type safety — pesar no contexto.
- **Replicação/particionamento/sharding**: só quando medido, não prematuro.

## ANTI-PATTERNS
`N+1` · `SELECT *` desnecessário · `índice que ninguém usa` · `migração sem rollback pensado` · `transação longa segurando lock` · `connection pool estourado` · `count(*) em tabela gigante` · `ORM escondendo query ruim`

## CHECKLIST
- [ ] EXPLAIN ANALYZE nas queries quentes
- [ ] Índices cobrindo os filtros/ordenações reais
- [ ] Migrações forward-only + compatibilidade
- [ ] Pool de conexões dimensionado
- [ ] Isolamento/locks entendidos nas transações
- [ ] Backup + recovery testado

## A REGRA
Nunca "otimize" por intuição. Meça, explique, mude, meça de novo.

## DOUTRINAS
| Doc | Tema |
|---|---|
| [POSTGRES.md](POSTGRES.md) | 06.1 — Relacional + JSONB híbrido (default canônico da casa) |
| [MYSQL.md](MYSQL.md) | 06.2 — Relacional InnoDB (legado LAMP do cliente) |
| [MONGODB.md](MONGODB.md) | 06.3 — Documento BSON flexível (schema volátil) |
| [REDIS.md](REDIS.md) | 06.4 — In-memory data structure server (cache/fila/ranking) |

## ÁRVORE DE DECISÃO — qual banco pra qual projeto
1. Dados canônicos/ACID (dinheiro, ledger, conhecimento)? → **Postgres** (default) | MySQL só se legado do cliente
2. Dados por documento, schema volátil (catálogo, perfil, telemetria)? → **MongoDB** (regra: consistência multi-doc → Postgres/JSONB)
3. Cache/fila/ranking/sessão/rate-limit? → **Redis** (sempre derivado do canônico, nunca fonte)
4. Edge/single-file/local-first (padrão atual da casa)? → **SQLite** (sobe pra Postgres quando multi-writer real)

**Regra de bolso: SQLite → Postgres → Mongo/Redis. Sobe de nível por medição (P13), não por hype.**

## REFERÊNCIAS
PostgreSQL · pgvector · Redis · CockroachDB · ClickHouse · SQLite · Prisma · Drizzle · goose/migrate
