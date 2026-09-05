# 06.3 — MONGODB INTELLIGENCE

> Stack 06.3 da Cosca Engineering Intelligence Matrix (doutrina).
> Documento BSON flexível — schema volátil, catálogos heterogêneos e telemetria.

## MISSÃO
Modelar dados por documento BSON seguindo "dado que se acessa junto, se armazena junto" — com aggregation pipeline bem pensado e `$jsonSchema` sempre presente — mantendo longe qualquer coisa que exija consistência multi-documento (isso é Postgres/JSONB).

## PRINCÍPIOS CORE
1. **"Dado que se acessa junto, se armazena junto"** — embed > join por padrão · UNIVERSAL
2. **1:1/1:N: embed ou referência** — decidir por frequência de acesso e crescimento ilimitado · STRONG
3. **Aggregation pipeline** — `$match` cedo; `$lookup` caro e **não atravessa shard** · UNIVERSAL
4. **`$jsonSchema` SEMPRE** — schema validation obrigatório mesmo em banco schema-less · UNIVERSAL
5. **NUNCA para consistência multi-documento** — transações multi-doc são exceção, não design · UNIVERSAL

## REGRAS DE DECISÃO
- Embed quando o dado acompanha o documento e cresce pouco; referência quando cresce ilimitado ou é acessado de muitos lugares.
- `$lookup` é caro e não atravessa shard — modelar para evitá-lo, nunca como desculpa de JOIN.
- `$match`/filtros mais seletivos primeiro no aggregation pipeline.
- Schema validation via `$jsonSchema` na criação das collections.
- Driver Go: `go.mongodb.org/mongo-driver/v2`.
- Consistência multi-documento (duas operações atômicas no mesmo evento) → sobe para Postgres/JSONB.

## ANTI-PATTERNS
`$lookup em todo lugar` · `documento gigante aninhado` · `sem schema validation` · `join multi-shard`

## CHECKLIST
- [ ] Collections com `$jsonSchema` de validation
- [ ] Embed/referência decididos por frequência de acesso e crescimento
- [ ] `$match` primeiro no pipeline; `$lookup` raro e justificado
- [ ] Documentos com profundidade/volume controlados
- [ ] Sem operações que exigem atomicidade multi-documento
- [ ] Índices cobrindo os filtros reais do pipeline

## A REGRA
MongoDB serve o dado que se lê inteiro e cresce de forma imprevisível — e a partir do momento em que você precisa de consistência entre documentos, ele para de ser o banco daquilo.

## REFERÊNCIAS
MongoDB docs · mongo-driver (go.mongodb.org/mongo-driver/v2) · database/README.md · PADRAO-COSCA §6

## CONHECIMENTO (schema _SCHEMA.md)

```
ID: database-009
DOMAIN: database
TITLE: Embed vs referência no documento BSON
PROBLEM: Aninhar ou referenciar? A escolha errada explode o documento ou multiplica queries
CONTEXT: MongoDB, modelagem 1:1/1:N
PRINCIPLE: O que se acessa junto se armazena junto — embed é o default; referência é para o que cresce ou se compartilha
RECOMMENDATION: Embed quando o subdado acompanha o documento e é lido junto; referência quando cresce ilimitado ou vive em muitos pais
WHEN_TO_USE: Itens de pedido, endereços, atributos de perfil
WHEN_NOT_TO_USE: Histórico que cresce sem fim, entidades compartilhadas entre milhares de documentos
TRADE_OFFS: Uma leitura atômica vs duplicação e tamanho do documento
EXAMPLE: Pedido embute `items[]` (lê-se junto) mas referência o `customerId` (entidade compartilhada)
COUNTER_EXAMPLE: Comentários de post embutidos sem limite até o documento estourar os 16MB
FAILURE_MODES: Documento gigante em reads/updates; duplicação divergindo sem reconciliação
REFERENCES: MongoDB data modeling docs
CONFIDENCE: STRONG
SOURCE: database/MONGODB.md
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: database-010
DOMAIN: database
TITLE: $jsonSchema validation sempre
PROBLEM: Banco schema-less recebe documento malformado que só explode na query de produção
CONTEXT: MongoDB, criação de collections
PRINCIPLE: Schema-less não significa schema sem contrato; a validation é a fronteira fail-closed do documento
RECOMMENDATION: Definir `$jsonSchema` (tipos, required, enum) no validator de toda collection
WHEN_TO_USE: Toda collection com entrada de usuário ou integração externa
WHEN_NOT_TO_USE: Coleção de staging/log descartável onde o custo do contrato não compensa
TRADE_OFFS: Rigidez de contrato vs liberdade de evolução do documento
EXAMPLE: `validator: { $jsonSchema: { bsonType: "object", required: ["email"], properties: {...} } }`
COUNTER_EXAMPLE: Collection criada na hora com insert e campo `email` string/object/number conforme o cliente
FAILURE_MODES: Campo `required` faltando em produção; type mismatch detectado só no consumo
REFERENCES: MongoDB $jsonSchema docs
CONFIDENCE: UNIVERSAL
SOURCE: database/MONGODB.md
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: database-011
DOMAIN: database
TITLE: $match cedo, $lookup caro e que não atravessa shard
PROBLEM: Pipeline mal ordenado varre tudo com $lookup caro e degrade sob volume
CONTEXT: MongoDB aggregation
PRINCIPLE: Reduzir o conjunto antes de caro: $match/seletividade primeiro, $lookup por último e raro
RECOMMENDATION: Colocar filtros mais seletivos no início; evitar $lookup em pipelines quentes; não modelar cross-shard
WHEN_TO_USE: Relatórios e agregações com volume real
WHEN_NOT_TO_USE: Busca simples que o índice resolve sozinho
TRADE_OFFS: Pipeline eficiente vs modelagem que força join no app
EXAMPLE: `$match { status: "paid", ... }` antes de qualquer `$group`/`$lookup`
COUNTER_EXAMPLE: `$lookup` no primeiro estágio varrendo coleção inteira antes de qualquer filtro
FAILURE_MODES: Shard: $lookup não atravessa shard e retorna conjunto incompleto/errado
REFERENCES: MongoDB aggregation pipeline docs
CONFIDENCE: UNIVERSAL
SOURCE: database/MONGODB.md
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: database-012
DOMAIN: database
TITLE: Consistência multi-documento sobe para Postgres/JSONB
PROBLEM: Operação que precisa atomicidade entre dois documentos vira transação de contorcionismo no Mongo
CONTEXT: MongoDB, design de consistência
PRINCIPLE: Mongo garante atomicidade no documento; consistência entre documentos é exigência de banco relacional
RECOMMENDATION: Quando duas escritas precisam ser atômicas no mesmo evento, modelar em Postgres (ou JSONB)
WHEN_TO_USE: Ledger, saldo de conta, reserva + débito, qualquer invariante entre registros
WHEN_NOT_TO_USE: Catálogo/perfil/telemetria onde cada documento é autônomo
TRADE_OFFS: Flexibilidade documental vs garantias ACID entre documentos
EXAMPLE: Transação financeira com saldo e lançamento em dois registros → Postgres
COUNTER_EXAMPLE: Implementar double-spending com updateIfCurrent e reza no Mongo
FAILURE_MODES: Transações multi-doc com retry compensatório; invariante quebrada silenciosamente
REFERENCES: database/README.md (árvore de decisão)
CONFIDENCE: UNIVERSAL
SOURCE: database/MONGODB.md
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```
