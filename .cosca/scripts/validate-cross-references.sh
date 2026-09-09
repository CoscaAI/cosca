#!/bin/bash
# ============================================================
# validate-cross-references.sh
# Verifica se todos os links internos .md são válidos
# Uso: bash scripts/validate-cross-references.sh [aos_root]
# ============================================================

set -e
COSCA_ROOT="${1:-$(dirname "$(dirname "$(realpath "$0")")")}"
ERRORS=0
WARNINGS=0
TOTAL_LINKS=0

echo "🔍 Validando cross-references em $COSCA_ROOT ..."
echo ""

# Encontra todos os links .md em arquivos .md
find "$COSCA_ROOT" -name "*.md" ! -path "*/node_modules/*" | while read -r file; do
    # Extrai links do tipo [texto](caminho)
    # Filtra apenas links relativos .md
    while IFS= read -r link; do
        TOTAL_LINKS=$((TOTAL_LINKS + 1))
        
        # Pula links externos (http, https, #)
        if echo "$link" | grep -qE '^(http|https|#|mailto:)'; then
            continue
        fi
        
        # Resolve o caminho relativo ao diretório do arquivo
        base_dir=$(dirname "$file")
        target="$base_dir/$link"
        
        # Remove âncora (#section) para verificar o arquivo
        target_file=$(echo "$target" | sed 's/#.*//')
        
        if [ ! -f "$target_file" ] && [ ! -d "$target_file" ]; then
            echo "❌ BROKEN LINK: $link"
            echo "   📄 in: $file"
            echo "   🎯 target: $target_file"
            ERRORS=$((ERRORS + 1))
        fi
    done < <(grep -oP '\[.*?\]\(\K[^)]+' "$file" 2>/dev/null | grep -v '^http' | grep -v '^#' | grep -v '^mailto:' | sed 's/\.md.*/.md/' | sort -u)
done

echo ""
echo "══════════════════════════════════════"
echo "  RESULTADO: ${ERRORS} erros, ${WARNINGS} warnings, ${TOTAL_LINKS} links"
echo "══════════════════════════════════════"

exit $ERRORS
