# 06.2 — MYSQL INTELLIGENCE

> Stack 06.2 da Cosca Engineering Intelligence Matrix (doutrina).
> Relacional InnoDB — para infra já existente do cliente (legado LAMP).

## MISSÃO
Manter sistemas MySQL/InnoDB corretos e previsíveis quando a casa herda a infra do cliente — com chaves primárias clustered, utf8mb4 e `EXPLAIN` como régua — sem reinventar o que Postgres faria melhor.

## PRINCÍPIOS CORE
1. **InnoDB** — transações, FKs e row locks; nada de MyISAM · UNIVERSAL
2. **PK clustered** — a chave primária define a ordenação física; preferir coluna monotônica (auto_increment/bigint) · UNIVERSAL
3. **utf8mb4** — charset padrão para texto completo (emoji e multibyte) · UNIVERSAL
4. **Índices B-tree** — cobrir filtros e ordenações reais, validados por `EXPLAIN` · UNIVERSAL
5. **Evitar ENUM** — novo valor exige ALTER de tabela · STRONG
6. **Sharding raro** — delegar a Vitess/ProxySQL quando for inevitável, nunca por intuição · STRONG

## REGRAS DE DECISÃO
- MySQL só para **legado LAMP / infra já existente do cliente** — nunca do zero se Postgres disponível.
- Postgres é estritamente mais capaz: tipos, JSONB, partial indexes, CTEs.
- Driver Go: `database/sql` + `github.com/go-sql-driver/mysql` com `interpolateParams=true` e `multiStatements=false` (fail-closed em segurança).
- PK monotônica evita page split e fragmentação da clustered index.
- `DATETIME` sempre com fuso explícito.

## ANTI-PATTERNS
`ENUM mutável` · `PK UUID aleatória em tabela grande` · `DATETIME sem fuso` · `MyISAM` · `N+1`

## CHECKLIST
- [ ] Engine InnoDB em todas as tabelas
- [ ] PK monotônica; UUID aleatória justificada e medida
- [ ] utf8mb4 no schema e nas conexões
- [ ] `EXPLAIN` nas queries quentes; índices B-tree cobrindo
- [ ] `interpolateParams=true`, `multiStatements=false`
- [ ] `DATETIME` com fuso explícito
- [ ] Sem ENUM em campos que mudam; sem MyISAM

## A REGRA
MySQL é dívida herdada, não escolha: o legado do cliente se mantém com InnoDB correto, PK monotônica e `EXPLAIN` — e o novo, quando possível, vai para Postgres.

## REFERÊNCIAS
MySQL docs · InnoDB docs · go-sql-driver/mysql · database/README.md · PADRAO-COSCA §6

## CONHECIMENTO (schema _SCHEMA.md)

```
ID: database-005
DOMAIN: database
TITLE: PK clustered exige coluna monotônica
PROBLEM: UUID aleatória como PK em tabela grande causa fragmentação e page split na inserção
CONTEXT: MySQL InnoDB, tabelas grandes
PRINCIPLE: No InnoDB a PK é a ordem física; valores aleatórios quebram a localidade de escrita
RECOMMENDATION: `bigint auto_increment` (ou outra coluna monotônica) como PK; UUID como coluna única com índice, não PK
WHEN_TO_USE: Tabela grande com escrita contínua
WHEN_NOT_TO_USE: Tabela pequena/longa onde o custo de escrita é irrelevante e a exposição da sequência importa
TRADE_OFFS: Escrita sequencial vs PK "previsível" (vazamento de ordem)
EXAMPLE: `id BIGINT AUTO_INCREMENT PRIMARY KEY` + `uuid CHAR(36) UNIQUE` para exposição
COUNTER_EXAMPLE: `PRIMARY KEY (uuid)` aleatório em tabela com milhões de linhas e inserção constante
FAILURE_MODES: Fragmentação da clustered index, page split, writes lentos sob carga
REFERENCES: InnoDB clustered index docs
CONFIDENCE: UNIVERSAL
SOURCE: database/MYSQL.md
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: database-006
DOMAIN: database
TITLE: ENUM mutável exige ALTER
PROBLEM: Novo valor válido de um campo enum quebra o deploy (precisa ALTER de tabela no meio da mudança)
CONTEXT: MySQL, colunas ENUM
PRINCIPLE: ENUM congela o domínio no schema; mudá-lo é uma operação de DDL acoplada ao release
RECOMMENDATION: Usar VARCHAR + tabela de domínio (ou CHECK) quando o conjunto pode crescer
WHEN_TO_USE: Domínio estável e pequeno (sexo binário clássico)
WHEN_NOT_TO_USE: Status/estados que evoluem com o produto
TRADE_OFFS: Compacidade de armazenamento vs flexibilidade de evolução
EXAMPLE: `status VARCHAR(20)` + valores conhecidos por aplicação em vez de ENUM com 3 estados
COUNTER_EXAMPLE: `ENUM('pending','active','blocked')` e o produto precisa de 'suspended' sem downtime
FAILURE_MODES: Release que falha porque esqueceu o ALTER; lock de tabela durante o ALTER
REFERENCES: MySQL enum docs
CONFIDENCE: STRONG
SOURCE: database/MYSQL.md
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: database-007
DOMAIN: database
TITLE: DATETIME sem fuso corrompe a verdade temporal
PROBLEM: Horários interpretados em timezone diferente do gravado geram registros errados
CONTEXT: MySQL, colunas temporais
PRINCIPLE: Tempo sem fuso é uma mentira que se descobre em produção; fuso deve ser explícito
RECOMMENDATION: Armazenar UTC (ou `TIMESTAMP`/`DATETIME` com fuso declarado) e converter na apresentação
WHEN_TO_USE: Qualquer coluna temporal em sistema multi-timezone
WHEN_NOT_TO_USE: Sistema estritamente local single-TZ onde o fuso nunca muda
TRADE_OFFS: Normalização temporal vs conversões a cada leitura
EXAMPLE: Gravar `created_at` em UTC e exibir no fuso do usuário na camada de apresentação
COUNTER_EXAMPLE: `DATETIME` gravado com hora local do servidor, servidor em timezone errado
FAILURE_MODES: DST corrompendo cálculos; auditorias com horários divergentes
REFERENCES: MySQL datetime/timestamp docs
CONFIDENCE: UNIVERSAL
SOURCE: database/MYSQL.md
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: database-008
DOMAIN: database
TITLE: Postgres sobre MySQL para projeto novo
PROBLEM: Projeto do zero em MySQL repete as limitações do legado sem nenhuma vantagem herdada
CONTEXT: MySQL, decisão de stack
PRINCIPLE: Postgres é estritamente mais capaz (tipos, JSONB, partial indexes, CTEs) — MySQL perde por habilidade, não por gosto
RECOMMENDATION: Postgres como default; MySQL apenas quando o cliente já tem infra/equipe/ops MySQL
WHEN_TO_USE: Já existe DBA de MySQL, ferramentas, ou requisito explícito do cliente
WHEN_NOT_TO_USE: Projeto verde sem amarras
TRADE_OFFS: Compatibilidade com a operação do cliente vs capacidade do engine
EXAMPLE: Migração de legado LAMP mantendo MySQL até o custo de migração ser medido; novo serviço já em Postgres
COUNTER_EXAMPLE: Escolher MySQL "porque é o que eu conheço" num projeto sem infra pré-existente
FAILURE_MODES: Pagar caro depois por JSONB/partial index/CTE que o banco não faz
REFERENCES: database/README.md (árvore de decisão)
CONFIDENCE: STRONG
SOURCE: database/MYSQL.md
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```
