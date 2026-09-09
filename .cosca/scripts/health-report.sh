#!/bin/bash
# ============================================================
# health-report.sh
# Gera relatório completo de saúde do framework Cosca
# Uso: bash scripts/health-report.sh [aos_root]
# ============================================================

COSCA_ROOT="${1:-$(dirname "$(dirname "$(realpath "$0")"))"}"

echo "═══════════════════════════════════════════════════════════"
echo "  Cosca FRAMEWORK — RELATÓRIO DE SAÚDE"
echo "  $(date '+%Y-%m-%d %H:%M')"
echo "═══════════════════════════════════════════════════════════"
echo ""

# 1. Inventário
echo "📊 INVENTÁRIO"
echo "   Arquivos .md: $(find "$COSCA_ROOT" -name '*.md' ! -path '*/node_modules/*' | wc -l)"
echo "   Departments: $(find "$COSCA_ROOT"/departments -name SKILL.md | wc -l)"
echo "   Skills: $(find "$COSCA_ROOT"/skills -name '*.md' ! -name SKILLS_CATALOG.md 2>/dev/null | wc -l)"
echo "   Workflows: $(find "$COSCA_ROOT"/workflows -name '*.md' | wc -l)"
echo "   Templates: $(find "$COSCA_ROOT"/templates -name TEMPLATE.md | wc -l)"
echo "   Engines: $(find "$COSCA_ROOT"/engines -name SKILL.md | wc -l)"
echo "   Councils: $(grep -c '### .* COUNCIL' "$COSCA_ROOT"/councils/COUNCILS.md 2>/dev/null || echo 0)"
echo "   Memory Records: $(find "$COSCA_ROOT"/memory -name '*.md' ! -name INDEX.md | wc -l)"
echo "   ADRs: $(find "$COSCA_ROOT"/memory/architecture/adr -name '*.md' 2>/dev/null | wc -l)"
echo ""

# 2. Cross-references
echo "🔗 CROSS-REFERENCES"
broken=0
find "$COSCA_ROOT" -name "*.md" ! -path "*/node_modules/*" | while read -r file; do
    grep -oP '\[.*?\]\(\K[^)]+' "$file" 2>/dev/null | while read -r link; do
        if echo "$link" | grep -qE '^(http|https|#|mailto:)'; then
            continue
        fi
        base_dir=$(dirname "$file")
        target_file=$(echo "$base_dir/$link" | sed 's/#.*//')
        if [ ! -f "$target_file" ] && [ ! -d "$target_file" ]; then
            echo "   ❌ Link quebrado: $link (em $file)"
        fi
    done
done 2>/dev/null

# 3. Orphans
echo ""
echo "📄 ARQUIVOS ÓRFÃOS"
find "$COSCA_ROOT" -name "*.md" ! -path "*/node_modules/*" ! -name "INDEX.md" | while read -r file; do
    basename_file=$(basename "$file" .md)
    refs=$(grep -rl "$basename_file" "$COSCA_ROOT" --include="*.md" ! -path "*/node_modules/*" 2>/dev/null | grep -v "$file" | wc -l)
    if [ "$refs" -eq 0 ]; then
        echo "   ❌ $file"
    fi
done 2>/dev/null

# 4. Saída
echo ""
echo "═══════════════════════════════════════════════════════════"
echo "  FIM DO RELATÓRIO"
echo "═══════════════════════════════════════════════════════════"
