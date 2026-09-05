# Cosca Knowledge Base — Schema Migrations

> **Database:** `cosca.db` (materializado em runtime)  
> **Localização:** `internal/embed/cosca/knowledge/schema/`  
> **Status:** active

## Visão Geral

Este diretório contém as migrations que definem o schema da Knowledge Base física.
A KB armazena heurísticas, padrões e playbooks — conhecimento estruturado que
os agentes consultam durante a operação.

O schema é versionado via tabela `schema_version`, que rastreia quais migrations
foram aplicadas, quando, com qual checksum e em qual status.

## Lista de Migrations

| Versão | Nome                    | Data       | Status   | Descrição                            |
|--------|-------------------------|------------|----------|--------------------------------------|
| V001   | initial                 | 2026-07-29 | pending  | Schema inicial: heuristics, patterns, playbooks, FTS5 |

## Como Aplicar Migrations

```bash
# Aplica todas as migrations pendentes
bash internal/embed/cosca/knowledge/schema/migrate.sh [caminho-do-cosca.db]

# Exemplo (usando default: .cosca/cosca.db)
bash internal/embed/cosca/knowledge/schema/migrate.sh

# Ou manualmente (uma por uma):
sqlite3 cosca.db < internal/embed/cosca/knowledge/schema/schema_version.sql
sqlite3 cosca.db < internal/embed/cosca/knowledge/schema/V001__initial.sql
```

## Como Verificar Versão Atual

```bash
sqlite3 cosca.db "SELECT MAX(version) FROM schema_version WHERE status='applied';"
```

Ou listar todas as migrations aplicadas:

```bash
sqlite3 cosca.db "SELECT version, name, applied_at, status FROM schema_version ORDER BY version;"
```

## Política de Migrations

1. **Forward-only**: migrations são irreversíveis. Rollback = criar uma nova
   migration corretiva (ex: V002 corrigindo um problema introduzido em V001).

2. **Idempotentes**: todas as migrations usam `CREATE TABLE IF NOT EXISTS` e
   `CREATE TRIGGER IF NOT EXISTS`. Re-executar uma migration já aplicada é seguro
   (não causa erros), mas o `migrate.sh` pula migrations já registradas.

3. **Checksum**: cada migration tem um checksum SHA-256 armazenado na
   `schema_version`. O `migrate.sh` calcula o checksum no momento da aplicação.
   Se uma migration for alterada após ser aplicada, o checksum armazenado
   servirá como trilha de auditoria.

4. **Nomenclatura**: `V<NNN>__<nome>.sql` — três dígitos, snake_case descritivo.

5. **Transacional**: migrations que falham são registradas com `status='failed'`
   no `schema_version`. O `migrate.sh` interrompe a execução ao encontrar uma
   falha.

## Relação com o Knowledge Engine

Este schema é **independente** do schema interno do Knowledge Engine
(`internal/sqlite/schema.go`). O Knowledge Engine gerencia seu próprio SQLite
em `~/.cosca/knowledge.db` com seu migration manager em Go. A KB (`cosca.db`)
é materializada separadamente e populada via sync pipeline.

## Estrutura de Diretórios

```
internal/embed/cosca/knowledge/schema/
├── INDEX.md              # Este arquivo
├── schema_version.sql    # Tabela de controle de versão (pré-requisito)
├── V001__initial.sql     # Migration inicial
├── VNNN__*.sql           # Migrations subsequentes
└── migrate.sh            # Script helper para aplicar migrations
```
