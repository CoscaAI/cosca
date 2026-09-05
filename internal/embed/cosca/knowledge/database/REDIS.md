# 06.4 — REDIS INTELLIGENCE

> Stack 06.4 da Cosca Engineering Intelligence Matrix (doutrina).
> In-memory data structure server — cache, fila, ranking, sessão, rate-limit.

## MISSÃO
Servir dados quentes do knowledge.db com os tipos certos (String, Hash, List, Set, Sorted Set, Streams, Geo, HLL, Bloom, TimeSeries) e TTL como schema de cache — sempre como camada **derivada**, nunca fonte da verdade.

## PRINCÍPIOS CORE
1. **Tipos com propósito** — String (valor), Hash (record), List (fila), Set (único), Sorted Set (ranking), Streams (append-only + consumer groups) · UNIVERSAL
2. **Streams como ledger de eventos** — append-only com consumer groups substitui a fila broker simples · STRONG
3. **TTL é o schema de cache** — `SET key val EX n` define a vida do dado; sem TTL não é cache · UNIVERSAL
4. **Persistência RDB + AOF** — snapshot + append; nunca confiar só em memória para o que importa · STRONG
5. **Tipos especiais por função** — Geospatial (localização), HyperLogLog (cardinalidade), Bloom (cache dedupe), TimeSeries (métricas) · STRONG
6. **NUNCA fonte da verdade** — cache é derivado do banco canônico; recriável a qualquer momento · UNIVERSAL

## REGRAS DE DECISÃO
- Cache quente do knowledge.db: dado calculado uma vez, servido milhares com TTL.
- Rate limiting: `INCR` + `EXPIRE` atômico por janela.
- Filas/streams: Streams com consumer groups (redelivery, ack) em vez de fila de String.
- Sessão: Hash ou String com TTL.
- Ranking: Sorted Set (`ZADD`/`ZRANGE`).
- Chaves com namespace (`service:entity:id`), nunca chave genérica.
- Driver Go: `github.com/redis/go-redis/v9`.

## ANTI-PATTERNS
`cache como fonte da verdade` · `sem TTL` · `tudo em String` · `chave gigante sem namespace`

## CHECKLIST
- [ ] Todo dado de cache tem TTL explícito
- [ ] Tipo certo por função (Hash para record, Stream para fila, ZSet para ranking)
- [ ] Namespace consistente em todas as chaves
- [ ] RDB/AOF configurado para o que não pode morrer com o processo
- [ ] Nenhum dado canônico vivendo só no Redis
- [ ] Rebuild do cache demonstrado a partir do banco canônico

## A REGRA
Redis é memória servindo o derivado: o dado canônico vive no banco, o TTL define a vida no cache, e tudo que está lá pode morrer sem perder verdade.

## REFERÊNCIAS
Redis docs · go-redis (github.com/redis/go-redis/v9) · database/README.md · PADRAO-COSCA §6

## CONHECIMENTO (schema _SCHEMA.md)

```
ID: database-013
DOMAIN: database
TITLE: TTL é o schema de cache
PROBLEM: Cache sem expiração vira dado fantasma que não reflete mais a fonte canônica
CONTEXT: Redis como cache
PRINCIPLE: Cache é projeção derivada; TTL define o quão velha a projeção pode ser antes de morrer
RECOMMENDATION: Todo SET de cache com `EX`/TTL coerente com a volatilidade do dado; `EXPIRE` em quem não definiu
WHEN_TO_USE: Resultado de query/calculo caro com janela de validade
WHEN_NOT_TO_USE: Fila/contador atômico que é estado transiente de negócio, não cache
TRADE_OFFS: Custo de recomputar vs frescor do dado servido
EXAMPLE: `SET top_courses:2026 "..." EX 300` recomputado da fonte a cada cache miss
COUNTER_EXAMPLE: Cachear perfil e nunca expirar; perfil continua velho meses após o usuário mudar
FAILURE_MODES: Stale read servindo dado errado; pico de thundering herd na expiração
REFERENCES: Redis EXPIRE/TTL docs
CONFIDENCE: UNIVERSAL
SOURCE: database/REDIS.md
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: database-014
DOMAIN: database
TITLE: Redis nunca é fonte da verdade
PROBLEM: Salvar dado canônico só no Redis e perdê-lo no restart expõe a fraqueza do in-memory
CONTEXT: Redis, arquitetura de dados
PRINCIPLE: Memória é derivada e recriável; o banco canônico é o dono da verdade
RECOMMENDATION: Persistir o dado no banco canônico; Redis serve a projeção quente e é reconstruído do banco
WHEN_TO_USE: Cache, sessão, contador de janela, ranking derivado
WHEN_NOT_TO_USE: Registro que não pode ser perdido (transação, conhecimento, ledger)
TRADE_OFFS: Latência de memória vs risco de perda em restart/failover
EXAMPLE: knowledge.db guarda o conteúdo; Redis guarda o top-N quente com TTL
COUNTER_EXAMPLE: Gravar só no Redis "para ser rápido" sem AOF e perder tudo no OOM
FAILURE_MODES: Restart limpa cache; app tenta ler e encontra vazio sem fallback para a fonte
REFERENCES: Redis persistence docs
CONFIDENCE: UNIVERSAL
SOURCE: database/REDIS.md
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: database-015
DOMAIN: database
TITLE: INCR + EXPIRE para rate limiting
PROBLEM: Permitir abuso em endpoint sem contagem atômica por janela de tempo
CONTEXT: Redis, rate limiting distribuído
PRINCIPLE: Contador de janela precisa ser atômico e auto-expirante — Redis resolve em uma instrução
RECOMMENDATION: `INCR key` + `EXPIRE key <janela>` na primeira vez; comparar com limite; 429 quando estourar
WHEN_TO_USE: Endpoints de auth, webhooks, APIs de terceiros
WHEN_NOT_TO_USE: Abuso de baixa frequência que um contador de banco já resolve
TRADE_OFFS: Granularidade por janela vs precisão por segundo (sliding window é mais caro)
EXAMPLE: `INCR ratelimit:{userId}:{minute}` com EXPIRE 60 — 60 requisições/min por usuário
COUNTER_EXAMPLE: Contador sem EXPIRE acumulando para sempre e bloqueando o usuário eternamente
FAILURE_MODES: Race entre INCR e EXPIRE em clientes separados (usar EVAL/MULTI para atomicidade)
REFERENCES: Redis INCR/EXPIRE docs
CONFIDENCE: UNIVERSAL
SOURCE: database/REDIS.md
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: database-016
DOMAIN: database
TITLE: Streams como ledger de eventos de cache/fila
PROBLEM: Fila de String (LPUSH/BRPOP) perde mensagens e não dá ack/redelivery
CONTEXT: Redis, filas e eventos
PRINCIPLE: Para eventos que precisam de rastreio e reprocessamento, Streams trazem append-only + consumer groups
RECOMMENDATION: Usar Streams com consumer groups (XADD/XREADGROUP/ACK) para filas de trabalho e eventos
WHEN_TO_USE: Eventos a processar com redelivery, múltiplos consumers, histórico recente
WHEN_NOT_TO_USE: Ledger canônico do negócio — isso é Postgres, não Redis
TRADE_OFFS: Semântica de fila real (ack, consumer groups) vs simplicidade do LPUSH/BRPOP
EXAMPLE: `XADD events:* ...` consumido por grupo com `XACK` após processar
COUNTER_EXAMPLE: Emitir evento financeiro em Redis Stream esperando que ele vire fonte de verdade
FAILURE_MODES: Stream sem trim crescendo sem limite; consumer sem ack reprocessando para sempre
REFERENCES: Redis Streams docs
CONFIDENCE: STRONG
SOURCE: database/REDIS.md
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```
