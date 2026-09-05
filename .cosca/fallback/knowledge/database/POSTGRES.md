# 06.1 — POSTGRES INTELLIGENCE

> Stack 06.1 da Cosca Engineering Intelligence Matrix (doutrina).
> Relacional + JSONB híbrido — o banco canônico padrão da casa.

## MISSÃO
Ser o default do Cosca para dados canônicos/ACID (ledger, conhecimento, dinheiro = numeric), unindo a integridade relacional à flexibilidade semi-estruturada do JSONB — sempre validado por `EXPLAIN ANALYZE`, a régua P13 do banco.

## PRINCÍPIOS CORE
1. **Normalização 3NF** — integridade relacional primeiro; desnormalização só por medição de leitura quente · UNIVERSAL
2. **JSONB com índice GIN** — `@>`, `?`, `jsonb_path_ops` cobrem consultas sobre campos semi-estruturados · STRONG
3. **JSONB não preserva ordem/whitespace** — se precisar do texto exato, usar `json` · STRONG
4. **UPDATE em JSONB trava a linha inteira** — documentos pequenos, nunca blob gigante no JSONB · UNIVERSAL
5. **Índices compostos/expression/partial** — decisão de query, cobrindo filtros e ordenações reais · UNIVERSAL
6. **MVCC** — leituras não bloqueiam escritas; otimismo por padrão · UNIVERSAL
7. **`EXPLAIN ANALYZE`** — medir antes de mudar; nunca otimizar por intuição · UNIVERSAL

## REGRAS DE DECISÃO
- Postgres é o **default** para dados canônicos; MySQL só se o legado do cliente exigir.
- Dinheiro em `numeric`, nunca `float64`.
- Semi-estruturado vai para JSONB com GIN; estrutura estável vai para colunas normais.
- Transações curtas e pontuais — nada de transação longa segurando lock.
- Driver Go: `github.com/jackc/pgx/v5` (pgxpool, casts `::jsonb`).

## ANTI-PATTERNS
`SELECT *` · `índice sem uso` · `jsonb gigante` · `transação longa segurando lock` · `otimizar por intuição`

## CHECKLIST
- [ ] Schema 3NF; desnormalização justificada por medição
- [ ] Índice GIN + `jsonb_path_ops` em colunas JSONB consultadas com `@>`
- [ ] Documentos JSONB pequenos (UPDATE não toca linha inteira em vão)
- [ ] `EXPLAIN ANALYZE` nas queries quentes
- [ ] Dinheiro em `numeric`
- [ ] Transações curtas, sem lock desnecessário
- [ ] Backup + recovery testado

## A REGRA
Postgres é o default da casa: dados canônicos e ACID nele, JSONB quando o schema é flexível — e nada se muda sem o `EXPLAIN ANALYZE` dizendo por quê.

## REFERÊNCIAS
PostgreSQL docs · pgvector · pgx (jackc/pgx/v5) · PADRAO-COSCA §6 · database/README.md

## CONHECIMENTO (schema _SCHEMA.md)

```
ID: database-001
DOMAIN: database
TITLE: JSONB + índice GIN para semi-estruturado
PROBLEM: Dados com schema volátil precisam de consulta flexível sem migração a cada mudança
CONTEXT: Postgres, colunas de payload variável
PRINCIPLE: JSONB indexado com GIN é o meio-termo entre relacional e documental dentro do mesmo banco
RECOMMENDATION: Coluna `jsonb` + índice `GIN (coluna jsonb_path_ops)`; consultar com `@>` e `?`
WHEN_TO_USE: Atributos variáveis, metadados, filtros dinâmicos
WHEN_NOT_TO_USE: Texto exato (ordem/whitespace) — aí use `json`; ou estrutura estável — aí use colunas normais
TRADE_OFFS: Flexibilidade de schema vs checks de integridade relacional
EXAMPLE: `CREATE INDEX ... USING GIN (metadata jsonb_path_ops)` + `WHERE metadata @> '{"tier":"vip"}'`
COUNTER_EXAMPLE: Guardar o documento inteiro num JSONB e filtrar por ele sem índice
FAILURE_MODES: Índice GIN sem `jsonb_path_ops` desperdiçando espaço; documento gigante em UPDATE travando a linha
REFERENCES: PostgreSQL JSONB docs
CONFIDENCE: UNIVERSAL
SOURCE: database/POSTGRES.md
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: database-002
DOMAIN: database
TITLE: JSONB não preserva ordem e whitespace
PROBLEM: Aplicação precisa de assinatura ou reprodução exata do payload original
CONTEXT: Postgres, coluna jsonb
PRINCIPLE: jsonb é um formato binário de armazenamento; ele normaliza o documento ao gravar
RECOMMENDATION: Usar o tipo `json` (texto) quando o byte-exato importa; `jsonb` para consulta
WHEN_TO_USE: Assinaturas, digests, logs de payload a serem reproduzidos
WHEN_NOT_TO_USE: Consulta sobre campos internos — aí `json` é ineficiente
TRADE_OFFS: Fidelidade de texto vs capacidade de indexação/consulta
EXAMPLE: `data jsonb` para filtro + `data_raw json` para o original a assinar
COUNTER_EXAMPLE: Confiar no `::jsonb::text` para reproduzir o JSON enviado pelo cliente
FAILURE_MODES: Assinatura quebrando porque o conteúdo foi reordenado na gravação
REFERENCES: PostgreSQL JSONB vs JSON docs
CONFIDENCE: STRONG
SOURCE: database/POSTGRES.md
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: database-003
DOMAIN: database
TITLE: UPDATE em JSONB trava a linha inteira
PROBLEM: Documentos JSONB grandes atualizados com frequência causam contenção e degradação
CONTEXT: Postgres MVCC, coluna jsonb
PRINCIPLE: JSONB é uma única coluna — qualquer UPDATE de um de seus campos reescreve a linha inteira
RECOMMENDATION: Manter documentos JSONB pequenos; campos quentes atualizados com frequência viram colunas normais
WHEN_TO_USE: Metadados de baixo volume de escrita
WHEN_NOT_TO_USE: Contador/estado que muda toda requisição
TRADE_OFFS: Simplicidade do documento vs custo de reescrita sob escrita quente
EXAMPLE: Perfil com `settings jsonb` (escrita rara) + colunas separadas para `status` (escrita frequente)
COUNTER_EXAMPLE: Session state inteiro em JSONB sendo UPDATE a cada request
FAILURE_MODES: Bloqueio/contenda em escritas concorrentes; bloat de versões MVCC
REFERENCES: PostgreSQL MVCC docs
CONFIDENCE: UNIVERSAL
SOURCE: database/POSTGRES.md
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: database-004
DOMAIN: database
TITLE: EXPLAIN ANALYZE como régua P13 do banco
PROBLEM: Otimização por intuição troca queries corretas por queries "parecidas mais rápidas" sem evidência
CONTEXT: Postgres, performance de query
PRINCIPLE: Nenhum fato sem verificação, nenhuma métrica sem medição — planos reais > achismo
RECOMMENDATION: Rodar `EXPLAIN ANALYZE` antes e depois de qualquer mudança de query/índice
WHEN_TO_USE: Query quente, índice novo, suspeita de seq scan, mudança de cardinalidade
WHEN_NOT_TO_USE: Query única de manutenção que roda uma vez por mês
TRADE_OFFS: Tempo de análise vs decisões baseadas em dado real
EXAMPLE: Seq scan de 40s vira index scan de 2ms após índice covering apontado pelo EXPLAIN
COUNTER_EXAMPLE: Criar índice "por garantia" e nunca olhar o plano de execução
FAILURE_MODES: EXPLAIN em dados de teste com distribuição diferente da produção enganando a decisão
REFERENCES: PostgreSQL EXPLAIN docs
CONFIDENCE: UNIVERSAL
SOURCE: database/POSTGRES.md
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```
