#!/bin/bash
# ============================================================
# validate-conventions.sh
# Verifica se arquivos seguem CONVENTIONS.md
# Uso: bash scripts/validate-conventions.sh [aos_root]
# ============================================================

set -e
COSCA_ROOT="${1:-$(dirname "$(dirname "$(realpath "$0")")")}"
ERRORS=0
TOTAL=0

echo "🔍 Validando CONVENTIONS compliance..."
echo ""

# Verifica department skills
echo "--- Departments ---"
for skill in "$COSCA_ROOT"/departments/*/SKILL.md; do
    TOTAL=$((TOTAL + 1))
    file_errors=0
    name=$(basename "$(dirname "$skill")")
    
    # Metadata block
    if ! grep -q "Version" "$skill" 2>/dev/null; then
        echo "   ❌ $name: missing Version metadata"
        file_errors=$((file_errors + 1))
    fi
    if ! grep -q "Status" "$skill" 2>/dev/null; then
        echo "   ❌ $name: missing Status metadata"
        file_errors=$((file_errors + 1))
    fi
    
    # Required sections
    for section in "PURPOSE" "SCOPE" "OUT OF SCOPE" "RESPONSIBILITIES" "DEPENDENCIES" "INPUTS" "OUTPUTS" "CONSTRAINTS" "QUALITY CRITERIA" "ESCALATION" "FORBIDDEN ACTIONS" "RELATED" "HISTORY"; do
        if ! grep -q "$section" "$skill" 2>/dev/null; then
            echo "   ❌ $name: missing section '$section'"
            file_errors=$((file_errors + 1))
        fi
    done
    
    if [ "$file_errors" -eq 0 ]; then
        echo "   ✅ $name"
    fi
    ERRORS=$((ERRORS + file_errors))
done

echo ""
echo "--- Workflows ---"
for wf in "$COSCA_ROOT"/workflows/*.md; do
    TOTAL=$((TOTAL + 1))
    file_errors=0
    name=$(basename "$wf" .md)
    
    for section in "OBJECTIVE" "INPUTS" "OUTPUTS" "PRECONDITIONS" "POSTCONDITIONS" "STEPS" "SUCCESS CRITERIA" "ERROR HANDLING" "HISTORY"; do
        if ! grep -q "$section" "$wf" 2>/dev/null; then
            echo "   ❌ $name: missing section '$section'"
            file_errors=$((file_errors + 1))
        fi
    done
    
    if [ "$file_errors" -eq 0 ]; then
        echo "   ✅ $name"
    fi
    ERRORS=$((ERRORS + file_errors))
done

echo ""
echo "══════════════════════════════════════"
echo "  RESULTADO: ${TOTAL} arquivos, ${ERRORS} erros"
echo "══════════════════════════════════════"

exit $ERRORS
