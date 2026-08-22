#!/bin/bash
# ============================================================================
# Cosca Knowledge Base — Migration Script
# ============================================================================
# Aplica migrations pendentes no cosca.db.
# Uso: ./migrate.sh [caminho-do-cosca.db]
#
# Política: forward-only. Rollback = nova migration corretiva.
# ============================================================================

set -euo pipefail

DB="${1:-.cosca/cosca.db}"
SCHEMA_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "=== Cosca KB Migration ==="
echo "Database: $DB"

# Garante que o diretório do banco existe
DB_DIR="$(dirname "$DB")"
if [ ! -d "$DB_DIR" ]; then
    mkdir -p "$DB_DIR"
    echo "  Created directory: $DB_DIR"
fi

# Cria a tabela schema_version se não existir (pré-requisito para todas as migrations)
echo "---"
echo "Initializing schema_version table..."
if sqlite3 "$DB" < "$SCHEMA_DIR/schema_version.sql"; then
    echo "  ✓  schema_version ready"
else
    echo "  ✗  Failed to create schema_version table"
    exit 1
fi

# Detecta se sha256sum está disponível; fallback para shasum -a 256
if command -v sha256sum &>/dev/null; then
    CHECKSUM_CMD="sha256sum"
elif command -v shasum &>/dev/null; then
    CHECKSUM_CMD="shasum -a 256"
else
    CHECKSUM_CMD=""
    echo "  ⚠  No sha256sum/shasum found — checksums will be empty"
fi

FAILED=0
APPLIED=0
SKIPPED=0

# Aplica migrations em ordem (V001, V002, ...)
for migration in "$SCHEMA_DIR"/V*.sql; do
    # Pula se não houver arquivos V*.sql
    if [ ! -f "$migration" ]; then
        break
    fi

    filename=$(basename "$migration")
    name="${filename%.sql}"
    version=$(echo "$filename" | grep -oP 'V\K\d+')

    if [ -z "$version" ]; then
        echo "  ⚠  Could not parse version from $filename — skipping"
        continue
    fi

    # Força integer (remove leading zeros)
    version=$((10#$version))

    # Verifica se já foi aplicada com sucesso
    applied=$(sqlite3 "$DB" \
        "SELECT COUNT(*) FROM schema_version WHERE version = $version AND status = 'applied';" \
        2>/dev/null || echo "0")

    if [ "$applied" -gt 0 ]; then
        echo "  ✓  $name (already applied)"
        SKIPPED=$((SKIPPED + 1))
        continue
    fi

    # Calcula checksum
    if [ -n "$CHECKSUM_CMD" ]; then
        checksum=$($CHECKSUM_CMD "$migration" | cut -d' ' -f1)
    else
        checksum=""
    fi

    echo -n "  …  $name ... "

    # Aplica migration
    if sqlite3 "$DB" < "$migration" 2>/tmp/cosca-migrate-err.log; then
        sqlite3 "$DB" \
            "INSERT INTO schema_version (version, name, checksum) \
             VALUES ($version, '$name', '$checksum');"
        echo "✓ (applied)"
        APPLIED=$((APPLIED + 1))
    else
        # Registra falha
        sqlite3 "$DB" \
            "INSERT INTO schema_version (version, name, checksum, status) \
             VALUES ($version, '$name', '$checksum', 'failed');" \
            2>/dev/null || true
        echo "✗ FAILED"
        echo "  Error log:"
        sed 's/^/    /' /tmp/cosca-migrate-err.log
        FAILED=$((FAILED + 1))
        exit 1
    fi
done

echo "---"
echo "=== Migration complete ==="
echo "  Applied: $APPLIED"
echo "  Skipped: $SKIPPED"
echo "  Failed:  $FAILED"

# Exibe versão atual
current=$(sqlite3 "$DB" \
    "SELECT MAX(version) FROM schema_version WHERE status = 'applied';" \
    2>/dev/null || echo "0")
echo "  Current version: $current"
