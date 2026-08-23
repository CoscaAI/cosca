# cosca-specialist-backend-service — Semantic Learnings

> Auto-evolution memory. Search before acting. Record after learning.

## Session: 2026-08-23 — FATIA H1: harness degradável + controles (ADR-011 Security, Bloco 2)

### 2026-08-23 — Envelope uniforme de resultado (results pkg)
| Field | Value |
|-------|-------|
| **Agent** | cosca-specialist-backend-service |
| **Task** | Implement `internal/results` envelope uniforme {success,data,degraded,exitCode} (paridade D6) |
| **Technique** | Manager pattern — novo pacote `results`, envelope value-type + construtores OK/Fail/Degraded, conversão de borda via `ResultError` |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #results #envelope #degraded #exit-code #boundary #adr-011 |
| **Related** | internal/adapter (a migrar em H2), internal/policy, finding V13 |
| **Learned** | Invarante central: `Success == (exitCode == 0)` mantido pelos construtores. `Fail(0,...)` normaliza exitCode→1 para nunca parecer sucesso na borda. `Degraded(...)` é `Success=true` + `Degraded=true` + `exitCode=0` — **não é error**; `WasError()`/`Err()` o tratam como sucesso. `Err()` retorna `*ResultError` que preserva o `Result` (resgata exitCode/reason na fronteira sem parse de string). |
| **Next** | H2: migrar `internal/adapter` para retornar `Result` (aditivo, mantém `(*T, error)`) |

### 2026-08-23 — MCPPolicy default-deny + dangerous patterns + budget (policy pkg)
| Field | Value |
|-------|-------|
| **Agent** | cosca-specialist-backend-service |
| **Task** | Implementar `mcp_policy.go`, `dangerous.go`, `budget.go` em `internal/policy` (default-deny, P1, V13, orçamento 200/turno) |
| **Technique** | Aditivo ao `policy` existente — reusa `Decision`/`Allow`/`Deny` (não recria enum). Classificação de capacidade por nome (determinística, sem parser de shell). Matching por substring (reuses `has(...)` do `DefaultRules`) |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #policy #mcp-policy #default-deny #dangerous-patterns #budget #v13 #security |
| **Related** | V13 (execpolicy não interpreta operadores de shell), ruflo-patterns (mcp-policy.json), internal/execpolicy/Tokenize |
| **Learned** | Default-deny = dois fatores: tool precisa estar em `AllowedTools` E a capability (Shell/Network/FileWrite) precisa estar explicitamente habilitada. `Evaluate` retorna `Deny` como **Decision, não error** — error só para uso inválido (nil receiver, tool vazia). Orçamento default 200 em `DefaultMaxToolCalls`; `ToolBudget.Consume` normaliza `Max<=0`→200. Padrões perigosos: `rm -rf`, `sudo`, `curl|sh/bash`, `ssh`, `git push --force/-f`, operadores `|`,`&&`,`;`,`` ` `` — específicos antes de genéricos para retornar o padrão mais informativo. |
| **Next** | H2: interpretar operadores de shell no `execpolicy` (parser leve), integrar `MCPPolicy` al `adapter` |

## Referências de padrões do projeto (constantes)
- Testes: table-driven com `github.com/stretchr/testify/assert` + `require` (convenção ampla no repo).
- `go build ./...`, `go vet ./...`, `go test ./internal/results/... ./internal/policy/...` verdes (go 1.26.7, Windows).
- `internal/embed/cosca` é intocável (P8) — nada foi alterado lá.
- Erros sempre envolvidos com contexto (`fmt.Errorf("...: %w", err)`); nunca `panic`.
