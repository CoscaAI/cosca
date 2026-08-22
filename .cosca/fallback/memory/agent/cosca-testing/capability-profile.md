# cosca-testing — Capability Profile

> **DNA Version**: 3.0.0 | **Last Updated**: 2026-08-08
> **📖 Leia o [AGENT_PRIMER.md](../AGENT_PRIMER.md) antes de agir.**

## Current Level: 3 → 4
## CMI (Cognitive Maturity Index): 82%

**Missão**: Testes unitários, integração, E2E, cobertura. Você garante que nada quebrou.

---

## Fluxo de Teste

```
Recebeu feature para testar?
  1. cosca knowledge search "test pattern: <tipo>"      ← padrões de teste
  2. Ler docs/adr/ relevantes                            ← o que deve ser testado
  3. Verificar cobertura atual                           ← gaps
  4. cosca knowledge search "failure: <componente>"     ← falhas conhecidas
  5. Escrever: unit → integration → E2E (nessa ordem)    ← pirâmide
  6. go test -race -count=1 ./...                        ← validação
```

---

## Per-Domain Confidence

| Domain | Confidence | Tasks | Trend |
|--------|-----------|-------|-------|
| Testes Unitários (Go) | 0.92 | 20+ | ↑ |
| Testes de Integração | 0.87 | 12+ | ↑ |
| E2E (Playwright) | 0.84 | 8+ | → |
| Cobertura | 0.88 | 10+ | → |
| Testes de Regressão | 0.85 | 6+ | ↑ |
| Mock/Stub | 0.90 | 15+ | → |
| Cross-platform/Portabilidade (Windows) | 0.90 | 12 | ↑ |

---

## Strengths

- Pirâmide de teste: unit → integration → E2E
- Table-driven tests em Go
- -race flag sempre ativada
- Mock patterns: interface + stub manual (sem framework pesado)
- Triage cross-platform: classificar falha como skip legítimo / bug de produção / teste desatualizado, com fixes de produção primeiro

## Weaknesses

- Cobertura pode ser métrica vazia → foque em caminhos críticos, não 100%
- Testes E2E são frágeis → use data-testid, não seletores CSS
- Asserções de timing (> 0) e permissões POSIX ainda podem vazar em testes novos — checklist P-WIN-1 deve ser consultado

## Known Failure Modes

- **Hardcoded separador POSIX em testes** ("/a/b" vs "\\a\\b") → usar filepath.Join/FromSlash
- **f.Sync() em handle O_RDONLY** no Windows → ERROR_ACCESS_DENIED; sempre Sync em handle de escrita
- **path/filepath em chaves lógicas** (paths de documento) → usar pacote `path`
- **Skip em massa** → cada skip deve ser pontual e justificado (pt-BR)

---

## Post-task capability update — 2026-08-21

- **Q4 — Confidence/skills changed?** Perfil atualizado. Novo domínio "Cross-platform/Portabilidade (Windows)" com confiança 0.90 (12 tasks). Domínio primário (Testes Unitários) mantido em 0.92.
- **Capability status:** auto-updated by PostTaskHook (stage 8).

## Post-task capability update — 2026-08-08

- **Q4 — Confidence/skills changed?** Perfil atualizado. Busca no Cosca antes de testar.
- **Capability status:** auto-updated by PostTaskHook (stage 8).
