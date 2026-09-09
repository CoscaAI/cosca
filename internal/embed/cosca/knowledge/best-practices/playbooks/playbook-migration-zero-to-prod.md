# Playbook: Migration do Zero à Produção

> **Versão**: 1.0.0 | **Workflow**: migration-execution | **Duração típica**: 2-5 dias

## Cenário
Migrar banco de dados PostgreSQL de `orders` de servidor on-premise para RDS na AWS, com zero downtime e零 data loss.

## Pré-requisitos
- [ ] Acesso SSH ao servidor on-premise
- [ ] Credenciais AWS com permissão RDS
- [ ] string de conexão do banco atual
- [ ] 4h de janela de manutenção aprovada

## Passo a Passo

### 1. Assessment (Dia 1, 2h)
```bash
# 1.1. Levantar tamanho do banco
psql -h $SOURCE_HOST -U $USER -d orders -c "SELECT pg_size_pretty(pg_database_size('orders'));"

# 1.2. Listar todas as tabelas e row counts
psql -h $SOURCE_HOST -U $USER -d orders -c "
  SELECT schemaname, tablename, n_live_tup
  FROM pg_stat_user_tables
  ORDER BY n_live_tup DESC;"

# 1.3. Verificar versão do PostgreSQL
psql -h $SOURCE_HOST -U $USER -d orders -c "SELECT version();"
```

### 2. Schema Migration (Dia 1, 1h)
```bash
# 2.1. Extrair schema
pg_dump -h $SOURCE_HOST -U $USER -d orders --schema-only > schema.sql

# 2.2. Aplicar no target RDS
psql -h $TARGET_HOST -U $USER -d orders -f schema.sql

# 2.3. Verificar schema
psql -h $TARGET_HOST -U $USER -d orders -c "\dt"
```

### 3. Data Migration (Dia 2, 3h)
```bash
# 3.1. Migrar dados (com compressão e paralelo)
pg_dump -h $SOURCE_HOST -U $USER -d orders \
  --data-only \
  --compress=9 \
  --jobs=4 \
  --format=directory \
  -f ./orders_data/

# 3.2. Restaurar no target
pg_restore -h $TARGET_HOST -U $USER -d orders \
  --jobs=4 \
  --data-only \
  ./orders_data/

# 3.3. Verificar row counts
psql -h $TARGET_HOST -U $USER -d orders -c "
  SELECT schemaname, tablename, n_live_tup
  FROM pg_stat_user_tables
  ORDER BY n_live_tup DESC;"
```

### 4. Validação (Dia 2, 1h)
```bash
# 4.1. Comparar row counts
echo "Source:"
psql -h $SOURCE_HOST -U $USER -d orders -c "SELECT count(*) FROM orders;"
echo "Target:"
psql -h $TARGET_HOST -U $USER -d orders -c "SELECT count(*) FROM orders;"

# 4.2. Amostragem aleatória
psql -h $TARGET_HOST -U $USER -d orders -c "
  SELECT * FROM orders ORDER BY random() LIMIT 5;"

# 4.3. Testar aplicação contra target
COSCA_DATABASE_URL=$TARGET_URL npm run test:integration
```

### 5. Rollback Plan
```bash
# Se algo der errado, rollback em 3 passos:
# 1. Parar aplicação
# 2. Reverter DNS/connection string para source
# 3. Reiniciar aplicação
echo "Revertendo para source: $SOURCE_HOST"
```

### 6. Checklist de Sucesso
- [ ] Todos os row counts batem (source == target)
- [ ] Amostragem de dados correta
- [ ] Testes de integração passando
- [ ] Performance aceitável (queries < 100ms p95)
- [ ] Rollback testado e documentado

## Lições Aprendidas
- Sempre testar o rollback ANTES da migração
- Amostragem aleatória pega corrupção silenciosa
- pg_dump com --jobs=4 reduz tempo em 60%
- Manter conexão ssh aberta durante a migração para monitorar

## Related
- [Migration Execution workflow](../../../workflows/migration-execution.md)
- [Migration Chief](../../../engines/tools/SKILL.md)
- [Database Chief](../../../engines/tools/SKILL.md)
