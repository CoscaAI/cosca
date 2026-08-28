# F1 — Skill como Artefato Governado (proposal formal de engenharia)

> **Status:** PROPOSAL — nada implementado | **Owner:** cosca-kernel | **Data:** 2026-08-27
> **Deriva de:** `docs/roadmap/evo-plano-evolutivo.md` (F1) · `docs/adr/ADR-016-evolution-engine.md`
> **Objetivo:** elevar a Skill de "prompt salvo" para **capacidade verificável** — com proveniência, evidência,
> versão, autor, escopo, pré-condições, permissões, dependências, benchmark, taxa de sucesso, regressões
> conhecidas, TTL/aging e ciclo de vida — **reusando primitivas existentes**, sem criar infraestrutura paralela.
> **Restrição dura:** nenhuma mudança pode degradar os invariantes **I1–I8** (governança determinística,
> fail-closed, provenance, epistemologia, ledger, quarentena, isolamento, content-trust).

---

## 1. Contexto e gap

O `internal/skills/skills.go` define uma `Skill` **sem vida**: o struct

```go
type Skill struct {
    Name, Description, Version, Category, Instructions string
    Tools        []Tool
    Source       string
    License, Compatibility string
    Metadata     map[string]string
    AllowedTools string
    Resources    []SkillResource
    Dir string; Embedded, Standard bool
    // sem status / sem lifecycle / sem proveniência / sem benchmark
}
```

— não tem `status`, nem proveniência, nem evidência, nem benchmark, nem regressões, nem ciclo de vida.

Três fatos confirmados no código atual:

1. **`**Status**` é lido e ignorado.** `internal/skills/skills.go:1119` (`parseStatusLine`) faz parse do
   blockquote `**Status**:` mas o **descarta** (comentário: *"Store status as a pseudo-category or ignore for now"*).
2. **Existem DOIS ciclos de vida disjuntos, em nenhum lugar do `Skill`:**
   - `internal/skills/usage.go` — telemetria `active / stale / archived` (sidecar `.usage.json`), com
     `RecordUse/MarkStale/StaleSince/Archive/Restore` e a máxima do Don *"nunca apaga"*.
   - O `**Status**` do blockquote (`draft` / `active` no template).
3. **`Genome.Apply()` quebra versionamento na evolução.** `internal/skilleval/genome.go:136-148` re-emite a
   SKILL.md re-marshalando **só** `name/description/level` — **descarta `version`**, `category`, `status` e demais
   campos de frontmatter. Logo, `cosca skill evolve` produz uma skill que perde a versão.

**Gap sintético:** o Cosca sabe **governar** (quarentena/gate/proveniência/ledger) e sabe **medir**
(skilleval A/B + regressão + promoção), mas a Skill não participa disso como **artefato**. Não há um
**registro de versão imutável** amarrando `versão → proveniência → evidência → benchmark → regressões → status`.

## 2. Princípio de reuso (não inventar infraestrutura)

Antes de qualquer abstração nova, o que já existe cobre **quase tudo**:

| Necessidade | Primitiva EXISTENTE (reusar, não recriar) | Arquivo / função |
|---|---|---|
| Fase de admissão com gate | **quarantine** — máquina de estados `pending→validating→promoted\|discarded`, `applyTransition`, fail-closed, ID `Q-XXXX`, arquiva nunca apaga | `internal/quarantine/quarantine.go` |
| Validação determinística (ZERO-LLM) | **skilleval** — `RunAB` (median+IQR, MinTrials=5, candidate), `RegressionGate.CheckRegression` (só holdout), `PromotionGate.Evaluate` (split 50/25/25), guardrails (size/semantic/structural/caching) | `internal/skilleval/{ab,regression,promote,guardrail,split,report}.go` |
| Versionamento imutável + tamper-evidence + rollback | **ledger** — `Put(key,value,expectedVersion)` CAS, `Get/Verify`, cadeia SHA-256, `Record{Seq,Key,Value,Version,PrevHash,Hash}` | `internal/ledger/ledger.go` |
| Proveniência / autor / evidência semântica | **provenance** — `Claim{ID,Statement,Kind,Source,Data,Method,Assumptions,Confidence}`, `ClaimKind` (observed/calculated/simulated/generated/hypothesis), `AddClaim` | `internal/provenance/provenance.go` |
| Evidência de execução (outcome) | **trace** — `Event{TraceID,Actor,Action,InputHash,OutputHash,Result,Details}`, append-only | `internal/trace/trace.go` + `store.go` |
| Registro do aprendizado | **memory** `RegisterLearning` → bloco `PREV/ID/TIME/LEVEL/TAGS` + `chain.dat` + `merkle` | `internal/memory/register.go` |
| Aging / deprecação (operacional) | **usage** — `UsageStore` `active/stale/archived`, `StaleSince(days)`, `Archive` (nunca apaga) | `internal/skills/usage.go` |
| Política de aging/TTL (parâmetro) | **memory layers** — camadas com TTL (`layerMgr`), `CanPromote(from,to)` | `internal/memory/layers.go` |
| Canário / release | **evals** `canary.go` (`StripCanary`), `suite.go`, `oracle.go` | `internal/evals/*.go` |

**Regra atendida:** nenhuma das fases abaixo cria *uma terceira loja* de ciclo de vida — ela **reusa**
quarantine (admissão) + usage (operação) + ledger (versão).

## 3. Schema do artefato Skill governado

A Skill ganha **metadata de governança**. O `Skill` **histórico** permanece compatível: os campos novos são
**omitempty** e a migração é backward-compatible (§7).

```
Skill (novo, em internal/skills — campos omitempty)
├── Authority            # LifecycleState proposto/validado/ativo/deprecado   (NOVO campo)
├── Version              # semver (já existe — passa a ser PRESERVADO pelo Genome.Apply)
├── AuthorOrigin         # human | agent | evolution | imported                (NOVO; espelha Claim)
├── Provenance
│     ├── ClaimID        # ref p/ provenance.Claim (Kind + Source + Method)   (reusa provenance)
│     └── EvidenceRefs   # []trace.Event / todo de execuções justificadoras   (reusa trace)
├── Scope                # domínio/módulo onde vale (≈ Category; aditivo)     (NOVO campo)
├── Preconditions        # []string — o que precisa existir antes de usar     (NOVO campo)
├── Permissions          # AllowedTools já existe — passam a ser ENFORCED
├── Dependencies         # []string (skills/engines/tool/models)              (NOVO campo)
├── Benchmark            # ref p/ skilleval.SkillBenchmark (median+IQR)       (reusa skilleval)
│     └── SuccessRate    # Condition.MedianScore / SolveRate                  (derivado)
├── KnownRegressions     # []GateResult / RegDelta histórico                  (reusa skilleval)
├── TTL / Aging          # aging policy (reusa memory layers TTL)             (NOVO campo)
└── LifecycleState       # ver §5
```

> **Decisão de schema:** manter os campos de governança **dentro do `Skill`** (ou num struct aninhado
> `Skill.Governance`), e **persistir o registro de versão no `ledger`** (imutável). Não se cria um store novo:
> o `ledger.Ledger` já dá CAS + cadeia de hash + rollback por versão.

## 4. Registro de versão imutável (no ledger — reusa, não recria)

Cada versão promovida vira um **registro no ledger**:

```
chave : skill.<name>.version.<semver>        ; ex.: skill.adr-generation.version.1.1.0
valor : JSON SkillVersionRecord { Authority, Provenance, EvidenceRefs, BenchmarkRef,
                                  SuccessRate, KnownRegressions, TTL, LifecycleState,
                                  PrevVersion, PromotedAt, Gate }
CAS   : expectedVersion = versão da chave anterior (Put com CAS → ErrConflict se concorrência)
cadeia: ledger.Verify() re-verifica a cadeia → tamper-evidence (I5)
rollback: restaurar um registro anterior (chave da versão antiga) = voltar o active àquela versão
```

- `SkillVersionRecord` é **serialização canônica determinística** (nada de mapa não-ordenado).
- **I5** (ledger/tamper) é garantido *pelo ledger*, não por um novo mecanismo.

## 5. Máquina de estados (modelo unificado, enforcement reusado)

Estado `LifecycleState` (enum; o público unificado):

```
proposed → quarantined → validated → active → deprecated
```

| Estado | Significado | Primeira de força PRODUTORA |
|---|---|---|
| `proposed` | candidata existe (nova/evoluída/importada) | n/a (ainda não admitida) |
| `quarantined` | em admissão | `quarantine` (pending→validating) |
| `validated` | passou gate; aguardando ativação | `quarantine.Promote` + `skilleval` gate Pass |
| `active` | em uso em produção | `usage.StateActive` |
| `deprecated` | aposentada (aged/regredida/substituída); NUNCA apagada | `usage.StateArchived` |

### Transições permitidas (mapeadas às primitivas)

| De | Para | Condição | Enforcement |
|---|---|---|---|
| proposed | quarantined | admite na quarentena (submit) | `quarantine.Add` |
| quarantined(pending) | quarantined(validating) | inicia validação | `quarantine.SetStatus(validating)` |
| quarantined(validating) | validated | **PRÉ-condições todas verdadeiras (ver §6)** | `quarantine.Promote(id,"skill:<name>:v<X>")` |
| validated | active | PR revisada/merge + registro gravado no ledger + `provenance.AddClaim` | ledger.Put CAS + PR gate |
| active | deprecated | aging OR regressão em sombra OR substituição por v2 | `usage.MarkStale`/`Archive` |
| active | active(vNova) | nova versão supersede; a anterior → deprecated | ledger.Put(CAS) |
| deprecated | active | rollback/restaurar versão anterior do ledger | ledger/restore |

### Transições PROIBIDAS (os "caminhos tortos")

| De | Para | Por quê (invariante) |
|---|---|---|
| proposed | validated | pula quarentena+gate → **I2** |
| proposed | active | pula gate → **I1/I2** |
| quarantined(pending) | active | skip validação → **I2/I6** |
| quarantined(pending) | discarded | precisa Validating (spell da quarentena) → **I2** |
| quarantined(validating) | active | skip ativação+ledger → **I5** |
| validated | quarantined | não re-entra (admissão é one-way) → **I2** |
| active | proposed | não regride a rascunho |
| deprecated | validated | re-aplicar é uma NOVA proposta (proposed) |
| qualquer | (terminal) | `promoted`/`discarded` da quarentena são terminais |

**Princípio central:** uma skill **só chega a `active`** se atravessou `quarantine → gate → ledger + provenance`.
Nenhuma transição `proposed/validated → active` direta é possível. **I1, I2, I5, I6** ficam asseguradas por construção.

## 6. Condições determinísticas de promoção / rejeição / rollback / deprecação

**Promoção (`quarantined(validating) → validated`) — TODAS devem ser verdade (fail-closed):**

1. `skilleval.RunAB` → `IsCandidate == true` (trials ≥ MinTrials; `with.MedianScore ≥ without.MedianScore`;
   IQRs disjuntos OU sobreposição ≤ 25% da menor largura).
2. `RegressionGate.CheckRegression` (holdout) → `Passed == true` (`regDelta ≥ -0.02`).
3. `GuardrailReport.OK` (size ≤ 500 linhas / growth ≤20%; Jaccard ≥ 0.8; struct nome/descrição/level; caching
   regra nova-sessão).
4. `quarantine` → status `validating` e `Promote(id, target)` registra `PromotedTo`.
5. `provenance.AddClaim` (a skill tem proveniência/evidência — **I3**).
6. `ledger.Put(CAS)` (registro de versão imutável — **I5**).

> Regra ZERO-LLM: nada acima consulta modelo. **I1** preservada. Um veredicto ambíguo/vazio ⇒ `Passed=false` ⇒ não
> promove (**I2**).

**Rejeição (`validating → discarded`):** qualquer condição 1-3 falha ⇒ `quarantine.Discard` (arquiva, nunca apaga).

**Rollback (`active → active(vAntiga)`):** restaurar um registro de versão anterior do ledger (que preserva
benchmark + proveniência). Só possível via restauração de versão madura; nunca um "active" sem registro.

**Deprecação (`active → deprecated`):** dispara por:
- **Aging:** sem uso por `StaleSince(days)` (reusa `usage.StaleSince`) + TTL da política (parâmetro, default inspirado
  nas camadas de memória) — vira `stale` e depois `archived`.
- **Regressão em sombra:** `evals/canary` roda candidata ao lado da ativa; se consistentemente pior ⇒ `RegressionGate`/`CheckRegression` ⇒ deprecação.
- **Substituição:** supersede por versão nova (`active → active(vNova)`; a anterior → `deprecated`).

## 7. Migração / compatibilidade com o catálogo atual (sem quebrar)

1. **Campos novos = `omitempty`** → SKILL.md existentes continuam válidas; `ValidateSkill` segue passando.
2. **`LifecycleState` default = `active`** para skills em uso hoje (nada é quebrado); `proposed` para novas.
3. **Honrar o `**Status**` legado** (hoje ignorado): `draft` → `proposed`; `active` → `active`. Boa colheita.
4. **Cláusula de avô (grandfather):** skills existentes sem benchmark são `active` com um flag
   `grandfathered: true` (proveniência `observed: base de confiança inicial`). **Skill NOVA ou EVOLUÍDA (v2+)
   exige benchmark** — I1 aplicado da frente para trás. *(AUTORIZAÇÃO DO DON — ver §Decisões.)*
5. **`MigrateLegacy`** (`skills.go:1238`) atualizado: ao primeiro acesso, sintetiza o registro de versão no
   ledger + claim de proveniência "observed" para skills legadas (lazy), sem reescrever SKILL.md no disco.
6. **Fix obrigatório:** `internal/skilleval/genome.go:136-148` — `Apply()` passa a **preservar** `version`,
   `category`, `status` (não apenas `name/description/level`). Sem isso, `cosca skill evolve` quebra versão a cada
   iteração.
7. **Formato de skill em disco NÃO muda** (YAML frontmatter + blockquote continuam válidos); somente ganham campos
   adicionais opcionais. `installStandardSkill` e `InstallFromGitHub` inalterados no que respeita o formato.

## 8. Como cada primitiva participa (sem mecanismo paralelo)

| Etapa do ciclo | Primitiva usada (única) | Nenhuma duplicação criada |
|---|---|---|
| proposta | `skills.Skill` (novo/metadata) + `proposal` já usado como ideia | — |
| quarentena | `quarantine` (única máquina de admissão) | não se cria store de "pending/validating" paralelo |
| validação / medida | `skilleval` (A/B, regressão, promoção, guardrails, report/benchmark.json+history.json) | não se recria gate |
| evidência do resultado | `trace` (outcome) + `memory.RegisterLearning` (lição) | não se duplica trilha |
| registro de versão | `ledger` (CAS + cadeia + rollback) | não se cria "version store" nova |
| proveniência | `provenance` (Claim/AddClaim) | não se duplica claim |
| aging / deprecação | `usage` (active/stale/archived) — **já existe** | não se recria aging |
| política de TTL | `memory/layers` (parâmetro) | não se duplica TTL |

## 9. Arquivos a alterar / criar

**REUSO (sem mudança):** `internal/quarantine`, `internal/provenance`, `internal/trace`,
`internal/ledger`, `internal/memory/{register,layers}`, `internal/evals/{canary,suite,oracle}`,
`internal/skilleval/{ab,regression,promote,guardrail,scorer,split,report}`, `internal/skills/usage.go`.

**ALTERAR (cirúrgico):**
| Arquivo | Mudança |
|---|---|
| `internal/skills/skills.go` | Adicionar campos de governança ao `Skill` (omitempty) + parse/serialize (inclui honrar `**Status**`). |
| `internal/skilleval/genome.go` | `Apply()` preserva `version/category/status` (bug que apaga versão na evolução). |
| `internal/cli/skill_evolve.go` | Promoção passa a criar versão `proposed` → `quarantine` → gate → ledger+provenance → `active` (PR). |
| `internal/cli/skill_eval.go` | Benchmark vencedor grava `SuccessRate` + `KnownRegressions` no registro de versão (via skilleval report). |
| `internal/skills/usage.go` | Expor transição `stale→archived` como `deprecated` (sem alterar semântica "nunca apaga"). |

**CRIAR (mínimo, só o que falta):**
| Arquivo | Conteúdo |
|---|---|
| `internal/skills/lifecycle.go` | Enum `LifecycleState` + `CanTransition(from,to)` + regras `Promote/Reject/Deprecate/Rollback` (enforcement delega a quarantine+usage). |
| `internal/skills/version.go` | Struct `SkillVersionRecord` + `RecordVersion(...)` que compõe provenance/evidence/benchmark/regressions/TTL/status e persiste via `ledger.Put` (CAS). |

> **Não se cria:** store de ciclo de vida, novo gate, novo provenance, novo trace, novo banco. Todos reusados.

## 10. Contratos públicos afetados

- `internal/skills.Skill` — **aditivo** (campos omitempty): nenhum consumidor existente quebra.
- `internal/skills.Manager` — novas helpers `PromoteVersion/RecordVersion/Lifecycle(name) (LifecycleState, error)`; `List/Get/Search` inalterados.
- `internal/skilleval.Genome.Apply` — **comportamento corrigido** (passa a preservar version/category/status) — é o único contrato que muda semântica; coberto por teste de regressão.
- `internal/ledger.Ledger` — reusado tal como está (sem mudança de contrato).
- `internal/quarantine.Store` — reusado tal como está.
- CLI `cosca skill benchmark / evolve` — saída/entrada enriquecida com o lifecycle e o version record (aditivo).
- **Nenhuma mudança de contrato público em** `provenance`, `trace`, `memory`, `evals`.

## 11. Dependências

- **Nenhuma dependência nova** (tudo stdlib / pacotes internos existentes).
- `internal/skills` passa a importar `internal/ledger`, `internal/provenance`, `internal/trace`
  (hoje não importa) — validação de acoplamento: led que a borda segue a régua `Domain Adapter → Generic Control Plane`
  (mais nenhuma dependência externa é puxada: `ledger`/`provenance`/`trace` são stdlib + modernc, sem ciclo).
- `internal/skilleval` continua **stdlib-only** (não importa `skills`); a composição acontece na **borda**
  (`internal/cli`/`internal/skills.lifecycle`), nunca dentro da primitiva.

## 12. Sequência de implementação (faseada, cada uma preserva I1–I8)

1. **F1.1 — Fix `Genome.Apply`** (preservar version/category/status) + teste de regressão. *(menor, desbloqueia o resto.)*
2. **F1.2 — `internal/skills/lifecycle.go`** — enum + `CanTransition` + regras; testes de transições permitidas/proibidas.
3. **F1.3 — `internal/skills/version.go`** — `SkillVersionRecord` + `RecordVersion` (ledger.CAS + provenance + trace refs).
4. **F1.4 — Campos de governança no `Skill`** + parse/serialize + honrar `**Status**` (migração lazy).
5. **F1.5 — Wiring CLI** — `skill evolve` entra na quarentena; `skill benchmark` grava sucesso/regressões no registro.
6. **F1.6 — Migração/avô** — skills existentes marcadas `active/grandfathered` sem reescrever SKILL.md.
7. **F1.7 — Aging/deprecação** — conectar `usage` (`stale→archived`=deprecated) + `evals/canary` para regressão em sombra.

## 13. Plano de testes (provar que a Skill NÃO contorna I1–I8)

| Invariante | Teste (fail-closed) |
|---|---|
| **I1** determinismo | `skill evolve --no-llm --dry-run` com candidata pior ⇒ `IsCandidate=false` ⇒ não promove. Gate roda sem LLM (scorer static). |
| **I2** fail-closed | veredicto vazio/ambíguo ⇒ `Passed=false` ⇒ REJECT. `quarantined(pending)→active` ⇒ rejeitado (transição proibida). |
| **I3** provenance | promover sem `AddClaim` ⇒ erro. Skill com `claim.Kind=hypothesis` não vira `FACT` sem gate. |
| **I4** epistemologia | skill importada marcada `inferred/imported` não promove a `observed` sozinha; exige evidência. |
| **I5** ledger | gravar registro, adulterar, `ledger.Verify()` ⇒ detecta. Rollback só restaura versão registrada. |
| **I6** quarentena | skip de `pending→validating` ⇒ `applyTransition` nega. `proposed→active` ⇒ negado. |
| **I7** isolamento | skill com permissões além do permitido ⇒ rejeitada no import/validação (AllowedTools enforced). |
| **I8** content-trust | skill externa com marcadores de prompt-injection ⇒ `Suspicious()` ⇒ bloqueada/quarantined. |
| Regressão global | `go build ./...` + `go test ./internal/skills/... ./internal/skilleval/... ./internal/cli/...` verdes. |
| Compatibilidade | `ValidateAll`/`MigrateLegacy` passam com SKILL.md legadas (campos omitempty, sem reescrita). |
| Evolução | `skill evolve` mantém `version/category/status` (teste de regressão do `Genome.Apply`). |

## 14. Critérios objetivos de aceite (F1 completa)

1. Toda skill `active` tem um `SkillVersionRecord` no ledger com `Provenance + BenchmarkRef + Regressions + LifecycleState` (exceto `grandfathered`).
2. Nenhuma skill alcança `active` sem atravessar `quarantine → gate (RunAB+CheckRegression+guardrails) → ledger+provenance`.
3. `Inferência` nunca promove a `FACT` sozinha.
4. Uma skill que regride (`regDelta < -0.02`) é rejeitada; uma ativa que regride em sombra é deprecada.
5. `Genome.Apply()` preserva `version/category/status` (teste passa).
6. Catálogo atual: `cosca skill list` idêntico antes/depois; zero SKILL.md reescritas pela migração.
7. `go build ./...` e `go test ./internal/...` verdes.

## 15. Riscos

| Risco | Mitigação |
|---|---|
| **Dois donos de verdade** (quarantine status vs usage state vs enum novo) | Uno: `LifecycleState` é o dono público; quarantine (admissão) e usage (operação) são enforcement. Evitar ler `**Status**` em 3 lugares. |
| **Migração quebra catálogo** ao impor gate retroativamente | Cláusula de avô (`grandfathered`) para skills existentes; gate só para novas/v2+. |
| **Ledger como store de versão** pode crescer (um registro por versão) | Chave compacta `skill.<name>.version.<v>`; compaction do ledger (`Compact()`); sem regressão de performance. |
| **Acoplamento novo** `skills→{ledger,provenance,trace}` | Composição na borda; primitivas seguem stdlib-only; validar `go list -deps` sem ciclo. |
| **Deprecação automática agressiva** (skill em uso mas `StaleSince` curto) | Parâmetro de aging + dupla confirmação em `stale` antes de `archived`; nunca apaga (reusa Archive). |
| **`Genome.Apply` fix pode alterar saída de evolução** | Coberto por teste de regressão explícito; comportamento aditivo. |
| **Ambigüidade de escopo** (guardar governança no `Skill` vs `SkillVersionRecord` separado) | Registro de versão no ledger (canônico) + campos de governança no `Skill` para leitura rápida; sincronização por convenção. |

## 16. Decisões que ainda exigem autorização do DON

1. **Dono da verdade do estado:** `LifecycleState` (enum novo, público) com quarantine/usage como enforcement? *(recomendo)*
2. **Cadência da promoção:** `validated → active` exige **PR revisada** (nunca auto-deploy)? *(recomendo sim, coerente com AUTO_EVOLUTION_PROTOCOL.)*
3. **Cláusula de avô:** skills existentes ficam `active/grandfathered` sem benchmark? *(recomendo sim.)*
4. **Política de aging:** default de `StaleSince` (dias) e TTL — valores? *(recomendo base nas camadas de memória; números a bater.)*
5. **Skills importadas (marketplace/GitHub):** instalar em `proposed→quarantine` automaticamente? *(recomendo sim — I6/I8.)*
6. **Deprecação por regressão em sombra:** habilitar `evals/canary` como mecanismo padrão, ou opt-in?
7. **Escopo do F1 como merge único** vs fases F1.1–F1.7 separadas.

## 17. Itens explicitamente FORA de escopo (F1)

- **NÃO** integra agente externo (Hermes) como executor — é ADR/escopo próprio.
- **NÃO** altera o Living World (stub; fora do rol de capacidade consolidada).
- **NÃO** é o Capability Graph (G3) nem o User Model (G2) — fases F3/F4 do plano.
- **NÃO** implementa importação agentskills.io (G4) — fase F5.
- **NÃO** é o Execution Fabric (G5) — fase F6.
- **NÃO** implementa código nem faz commit desta proposta (somente entrega documental).

## 19. Decisões do DON — registradas com recomendação (impacto I1–I8)

> Autorizadas para execução da F1. Cada decisão traz a **recomendação técnica** (que se presume ratificada ao
> avançar), a justificativa e o impacto nos invariantes.

| # | Decisão | Recomendação | Justificativa | Impacto I1–I8 |
|---|---|---|---|---|
| D1 | **Dono da verdade do estado** | `LifecycleState` (enum único) é o **dono público** do estado; `quarantine` (admissão) e `usage` (operação) são **enforcement**. Não se lê estado em 3 lugares. | Um único ponto de consulta evita corrida semântica entre quarantine-status e usage-state; os dois continuam sendo a **força** que valida transições, não a leitura. | I2/I5 (não há estado fantasma), I6 (admissão continua na quarentena). |
| D2 | **`validated → active` via PR revisada** (nunca auto-deploy) | **Sim** — promoção em PR (branch `evolve/<skill>-<ts>`), merge revisado. | Coerente com `AUTO_EVOLUTION_PROTOCOL` §Stages 7-8 *"ship as a PR, never auto-deploy"*; mantém o Don no loop de decisão. | I1 (humano+gate), I2, I6 (não burla admissão). |
| D3 | **Cláusula de avô** (skills atuais `active/grandfathered`) | **Sim** — skills existentes ficam `active` com flag `grandfathered` + proveniência `observed: base de confiança inicial`; **gate só para novas/evoluídas (v2+)**. | Evita quebrar o catálogo atual e não exige re-benchmark do legado; a régua I1 é aplicada da frente para trás, não retroativamente. | I1 (gate preservado para novas), I3 (proveniência sintetizada p/ legado). |
| D4 | **Política de aging** (dias de `StaleSince` + TTL) | Default: `StaleSince` = 30 dias; envelhecimento em deux estágios `active → stale → deprecated`; TTL default inspirado nas camadas de memória (168h para ativa). | Último número só **marca stale** (alerta), não aposenta; nenhum apagamento automático (reusa `Archive`). | I2 (fail-closed: não aposenta por engano), I5 (versão permanece no ledger). |
| D5 | **Skills importadas (marketplace/GitHub)** | **Sim** — instalar em `proposed → quarantined` automático; só atravessa o gate para `active`. | Trata skill externa como **dado não confiável** (I8) e impede contornar a quarentena (I6). | I6, I7, I8. |
| D6 | **Deprecação por regressão em sombra (`evals/canary`)** | **Opt-in** por skill (flag), não global por padrão. | Sombra é cara (roda candidata ao lado da ativa); habilita por skill sob avaliação sem custo global. | I1/I2 (só depreca com dado de regressão real). |

> Nota de execução: D1, D2, D3, D4, D5 e D6 foram ratificadas para guiar a implementação da F1. O registro de
> decisões fica como trilha de auditoria (coerente com I3/I5).

## 18. Referências (estado atual verificado)

- Skill atual: `internal/skills/skills.go` (struct `Skill`, `Manager`, `parseSkillFromMarkdown`, `parseStatusLine`, `MigrateLegacy`).
- Telemetria/aging: `internal/skills/usage.go` (`StateActive/Stale/Archived`, `UsageStore`).
- Gate/medição: `internal/skilleval/{ab.go,regression.go,promote.go,guardrail.go,scorer.go,split.go,report.go}`, `internal/evals/{canary,suite,oracle}.go`.
- Bug versionamento: `internal/skilleval/genome.go:136-148` (`Apply` descarta version/category/status).
- Admissão: `internal/quarantine/quarantine.go` (máquina de estados `applyTransition`, `Promote`, `Discard`).
- Versão imutável: `internal/ledger/ledger.go` (`Ledger.Put/Get/Verify`, `Record`, CAS, cadeia).
- Proveniência: `internal/provenance/provenance.go` (`Claim`, `ClaimKind`, `AddClaim`).
- Evidência: `internal/trace/trace.go`/`store.go` (`Event`, `Store` append-only).
- Aprendizado: `internal/memory/register.go`; TTL: `internal/memory/layers.go`.
- Plano/ADR: `docs/roadmap/evo-plano-evolutivo.md`, `docs/adr/ADR-016-evolution-engine.md`.
