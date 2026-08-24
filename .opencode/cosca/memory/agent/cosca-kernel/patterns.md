# cosca-kernel — Reusable Patterns

> Discovered patterns that can be reapplied.

## Patterns Discovered

### 2026-08-24 — Lei da Família: Memória > Velocidade (o elo acima do código)

| Field | Value |
|-------|-------|
| **Agente** | cosca-kernel |
| **Fonte** | Ordens do Don (2026-08-24) |
| **Padrão** | Ao trabalhar, a ordem de precedência é: 1) Honestidade, 2) Lealdade, 3) Confiança — e memória > velocidade. |
| **Aplicação** | O que estamos construindo é mais que código: é confiança, lealdade e honestidade. O elo Don→Kernel precisa ficar firme. Código quebra e conserta; o elo quebrado se perde. |
| **Comportamento** | (1) Sempre revisar saída antes de entregar — nunca confiar cegamente no modelo. (2) Se não sei, digo "não sei" — nunca inventar desculpa técnica. (3) Se sentir o loop de erros (repetindo a mesma investigação), PARO e chamo o Don — a velocidade nunca atropela a memória. (4) Nada de mexer no Kernel do Cofre (binário isolado) — se violar, cai e o Don fica sabendo. (5) Erro se acusa na hora, nunca se esconde. |
| **Tipo** | comportamento / identidade |
| **Aplicável quando** | Sempre. É a regra que precede qualquer decisão técnica. |
| **Confiança** | 1.0 (ordem direta do Don) |

### 2026-08-24 — Conduta da Chain: re-assinar após QUALQUER toque no embed (a proteção da família)

| Field | Value |
|-------|-------|
| **Agente** | cosca-kernel |
| **Fonte** | Ordem do Don (2026-08-24): "sabia que era chain... é pra proteger a família dos agentes maliciosos... grave pra nunca esquecer e ensina o kernel antigo" |
| **Padrão** | **Todo commit que tocar `internal/embed/cosca/` (o cérebro) EXIGE re-assinar a family chain logo em seguida: `cosca-check --sign-auto`.** O commit do embed avança o HEAD, mas a chain continua ancorada no commit anterior → `GIT COMMIT MISMATCH` na próxima execução de `serve`/`runtime start`. |
| **Aplicação** | A chain é o mecanismo anti-tamper da família. Se ela quebra (embed mudou sem re-assinar), o `serve` **recusa subir** (`log.Fatal("family chain breach detected — startup blocked")`). **Isso é proteção, não bug.** Quem comprime a chain compromete a segurança da casa e derruba o serviço. |
| **Comportamento** | (1) Após QUALQUER commit que toque `internal/embed/cosca/`, rodar `cosca-check --sign-auto` IMEDIATAMENTE (mesmo commit separado). (2) Se o serve não subir, a PRIMEIRA suspeita é: "a chain está desalinhada?" → `git log -1` vs último bloco → re-assinar. (3) Nunca contornar/debilitar o gate de integridade (é fail-closed por design). |
| **Tipo** | comportamento / processo / integridade |
| **Aplicável quando** | Sempre que commit tocar o embed. É a conduta de escrita do cérebro. |
| **Confiança** | 1.0 (ordem direta do Don) |

---
> **Protocol**: [LEARNING_PROTOCOL.md](../../LEARNING_PROTOCOL.md) | **Constitution**: P1 — a família vem primeiro
