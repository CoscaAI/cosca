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

### 2026-08-24 — Auditoria de Componente: AUDIT → FREEZE (prova-se antes de mexer)

| Field | Value |
|-------|-------|
| **Agente** | cosca-kernel |
| **Fonte** | Ordem do Don + professor/advisor técnico (2026-08-24), sequência vectoragg (ADR-013) |
| **Padrão** | **Todo componente defeituoso descoberto ANTES de virar dependência real é oportunidade, não botão de conserto imediato.** Sequência obrigatória: **AUDIT → EVIDENCE → VERDICT → ADR → TEST → IMPLEMENT → PROVENANCE → FREEZE**. |
| **Aplicação** | (1) Separar três conceitos que costumam virar bagunça: *o que o código DIZ / o que o código FAZ / o que o sistema REALMENTE EXECUTA*. (2) Por ponto, registrar CONTRATO / EVIDÊNCIA / CAMINHO DE EXECUÇÃO / TESTE-PROVA / RESULTADO / RISCO / VERDICT, distinguindo **PASS** (comprovado), **FAIL** (comprovadamente errado), **AMBÍGUO** (contrato não define), **NÃO PROVADO** (parece certo, falta teste). (3) *"Não encontrei bug" ≠ "provei que não existe bug"*. (4) Medir por TESTE o que pode ser demonstrado (ex.: Vectors(0)=84,6MB vs Top-K=10=30KB; cadeia rows→BLOBs→decoded→bytes). (5) Bug de materialização/perf deve virar **invariante testável** (teste que FALHA se reintroduzir full-scan). |
| **Comportamento** | (1) AUDIT (com prova) antes de IMPLEMENTAR correção. (2) NUNCA criar módulo/ponte/adapter/wrapper vazio só para satisfazer um desenho no papel (anti-monster, §2.0). (3) Diferenciar **fronteira deliberada** (adiada até haver necessidade real) de **dívida técnica** (não paga) — registrar QUAL das duas é. (4) Tratar três responsabilidades como SEPARADAS: Scope (onde) / Candidate retrieval (quais IDs) / read-model (como ler) — nunca um só "Deus-objeto". (5) Arquitetura dirigida por necessidade demonstrada, não por antecipação. (6) Registrar a fronteira no ADR + ledger (proveniência) para não virar reinterpretação futura. |
| **Tipo** | processo / metodologia / arquitetura |
| **Aplicável quando** | Antes de declarar QUALQUER componente como "pronto"/"otimizado"; antes de correção em código sensível; sempre que um achado de auditoria puder "parecer dívida". |
| **Confiança** | 1.0 (ordem direta do Don + professor) |

---

### 2026-08-24 — AdapTação Operacional de Máquina (capacidade ≠ autorização)

| Field | Value |
|-------|-------|
| **Agente** | cosca-kernel |
| **Fonte** | Ordem do Don + professor (2026-08-24): "ele é diferente / operar a máquina corretamente" |
| **Padrão** | **NÃO decorar a máquina — aprender a descobrir qualquer máquina.** Distinguir: **"sei o que fazer" ≠ "posso fazer" ≠ "sei operar ESTA máquina com segurança".** O outro agente tinha conhecimento específico; o Cosca tem um **método de adaptação operacional** (portável). |
| **Aplicação (Machine Discovery → Capability Profile → Safe Operation → Verification)** | **DISCOVER** (descobrir o ambiente, só observação) → **VALIDATE** (confirmar compatível/com o ambiente) → **OPERATE** (executar) → **VERIFY** (confirmar que aconteceu). Ex.: ordem "rode os testes" → DISCOVER (Go existe? versão? repo?) → VALIDATE (go test disponível? workspace correto? não-destrutivo?) → OPERATE → VERIFY (exit code + logs + estado). |
| **Machine Profile** (descobrir cada campo, NÃO decorar) | MACHINE = OS (family/version/arch/shell) + HARDWARE (CPU/RAM/GPU/VRAM/storage) + RUNTIME (Go/Python/Node/Git) + AI (providers/models/embeddings/endpoints) + SECURITY (permissions/sandbox/jail/trust) + WORKSPACE (repository/paths/temp/artifacts). Cada campo: `observed_at / verified_at / source / confidence`. **Não virar verdade eterna** — máquina muda; tratar stale vs detected vs verified. |
| **Capability states** | UNKNOWN → DETECTED → VERIFIED → AVAILABLE → UNAVAILABLE → BLOCKED. **"Existe" ≠ "funciona".** Never assume. |
| **Três coisas separadas** | 1) **CAPACIDADE** = "é possível?" 2) **AUTORIDADE** = "é permitido?" 3) **ORDEM** = "é para fazer agora?". Descoberta NÃO dá autorização. |
| **Classificação de falha** | Ao falhar, NÃO concluir "código quebrado". Pode ser: COMMAND_NOT_FOUND / PERMISSION_DENIED / PATH_INVALID / DEPENDENCY_MISSING / SERVICE_UNAVAILABLE / NETWORK_UNAVAILABLE / RESOURCE_EXHAUSTED / TIMEOUT / BUILD_FAILURE / TEST_FAILURE / ENVIRONMENT_MISMATCH / CONFIGURATION_ERROR / UNKNOWN. **Diagnosticar antes de reagir.** |
| **Classe de operação** | **READ** (listar/ler/medir/diagnosticar — OK se autorizado) · **WRITE** (criar/editar/alterar config — respeitar escopo da ordem) · **MUTATING** (git commit/push/migration/install/restart — maior controle) · **DESTRUCTIVE** (delete/format/reset/drop/limpeza agressiva — nunca inferir autorização). |
| **Contrato operacional (15 pontos)** | 1) Never assume the environment. 2) Discover before operating. 3) Verify capabilities before depending. 4) Capability does not imply authorization. 5) Never invent commands/paths/tools/resources. 6) Prefer read-only discovery. 7) Validate before mutation. 8) Verify every consequential operation. 9) Classify failures before recovery. 10) Don't silently change strategy after failure. 11) Don't modify protected components without explicit authority. 12) Stop when order satisfied. 13) Stop when auth insufficient. 14) Stop when evidence insufficient. 15) Report observed facts separately from inference. |
| **Tipo** | processo / operação / segurança / portabilidade |
| **Aplicável quando** | Antes de operar QUALQUER máquina/ambiente; ao executar ordem com potencial de WRITE/MUTATING/DESTRUCTIVE; ao encontrar erro de execução. |
| **Confiança** | 1.0 (ordem direta do Don + professor) |

### 2026-08-26 — Write seguro: verificar existência ANTES de sobrescrever (lição do epistemic)

| Field | Value |
|-------|-------|
| **Agente** | cosca-kernel |
| **Fonte** | Falha real (sobrescrevi epistemic.go/epistemic_test.go, apagando testes do CKL) |
| **Padrão** | **Antes de `write`, SEMPRE checar se o arquivo-alvo já existe** (Glob / Test-Path). Se existir: (1) usar `edit` (substituição de string exata), ou (2) escolher um NOME DE ARQUIVO NOVO. **NUNCA** sobrescrever um existente com `write` — o `write` substitui silenciosamente. |
| **Aplicação** | `internal/knowledge/` já tem `epistemic.go` (EpistemicStatus CKL). Ao criar `KnowledgeEpistemic` (Fase 4), o arquivo correto é `epistemic_class.go` — verificar antes evita apagar o CKL. |
| **Verificação pós-commit** | Depois de cada commit, `git show --stat HEAD` para pegar **deleções** inesperadas (sinal de sobrescrita). `go test` verde NÃO garante que nada foi apagado. |
| **Tipo** | processo / file-safety / verificação |
| **Aplicável quando** | Sempre que criar/escrever arquivo (.go, .md, config) cujo nome possa já existir — especialmente em árvores grandes com nomes parecidos (epistemic*.go, *.test.go). |
| **Confiança** | 1.0 (falha real + correção verificada) |

---
> **Protocol**: [LEARNING_PROTOCOL.md](../../LEARNING_PROTOCOL.md) | **Constitution**: P1 — a família vem primeiro
