#!/bin/bash
# ============================================================
# detect-orphans.sh
# Detecta arquivos .md não referenciados por nenhum outro
# Uso: bash scripts/detect-orphans.sh [aos_root]
# ============================================================

set -e
COSCA_ROOT="${1:-$(dirname "$(dirname "$(realpath "$0")"))"}"
ERRORS=0
TOTAL=0

echo "🔍 Detectando arquivos órfãos..."
echo ""

# Lista todos os arquivos .md (exceto INDEX.md e memória)
find "$COSCA_ROOT" -name "*.md" ! -path "*/node_modules/*" ! -name "INDEX.md" | while read -r file; do
    TOTAL=$((TOTAL + 1))
    relative_path=$(realpath --relative-to="$COSCA_ROOT" "$file")
    
    # Extrai o nome base sem extensão
    basename_file=$(basename "$file" .md)
    
    # Procura referências a este arquivo em outros .md
    # Procura pelo caminho relativo ou nome do arquivo
    refs=$(grep -rl "$basename_file" "$COSCA_ROOT" --include="*.md" ! -path "*/node_modules/*" 2>/dev/null | grep -v "$file" | wc -l)
    
    if [ "$refs" -eq 0 ]; then
        echo "   ❌ ÓRFÃO: $relative_path (0 referências)"
        ERRORS=$((ERRORS + 1))
    fi
done 2>/dev/null

echo ""
echo "══════════════════════════════════════"
echo "  RESULTADO: ${ERRORS} arquivos órfãos"
echo "══════════════════════════════════════"

exit $ERRORS
