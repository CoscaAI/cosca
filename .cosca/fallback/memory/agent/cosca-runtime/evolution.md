# cosca-runtime — Evolution Timeline

> Auto-evolution tracking. Records capability level progression.

## Current Level: 4

## Evolution History

| Date | Level | Capability | Trigger |
|------|-------|------------|---------|
| 2026-07-27 | 1 | Baseline capabilities established | Initial audit |
| 2026-07-28 | 2 | Code-to-documentation cross-validation: 5 source files (2.7K lines) vs docs | Runtime documentation sync |
| 2026-07-28 | 1 | Metrics system architecture: 8 counters, 4 histograms, atomic ops | Codebase scan |
| 2026-08-03 | 4 | Format alignment (float32/dims) + scope bug fix (RootDir) + migration bug (AutoMigrate) + 5 bug fixes (sync dry-run, bwrap --dev, silent error, stderr MCP) | Auditoria profunda + resgate do runtime (ordem do Don) |
| 2026-08-04 | 4 | Gate fail-closed de integridade no entrypoint, allowlist offline e root detection consistente | Enforcement de novas sessões |

## Capability Gains (2026-08-03)

- **Diagnóstico de embeddings:** validar formato de vetores (float32 vs float64, dims) antes de buscar
- **Governança de banco:** escopo do RootDir, AutoMigrate, backup reversível (.legacy)
- **Enforcement de erros:** nunca engolir erro de provider; propagar + sinalizar
- **Padrões de wiring:** detectar subsistemas mortos (RegisterX ausente)
