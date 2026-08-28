# ADR-016: Evolution Engine — self-improving, evidence-gated (o agente aprende, o cérebro decide)

> **Status:** Proposed | **Owner:** cosca-kernel + cosca-architecture | **Last Updated:** 2026-08-27
> **Revisão:** aguardando cosca-cto + cosca-evolution + Don. **Design aditivo — não quebra o Root.**
> **Referência (benchmark/estudo):** análise comparativa Cosca × Hermes Agent (`docs/cosca-vs-hermes-agent-comparativo.pdf`).

---

## 0. Contexto — por quê agora

O estudo comparativo com o Hermes Agent revelou uma divisão arquitetural clara entre os dois sistemas:

- **Hermes** otimiza o agente para **agir e evoluir** (learning loop: usuário → agente → LLM → tools →
  review → skill/memória → próximo turno).
- **Cosca** otimiza o sistema para **decidir se aquilo pode ser considerado conhecimento/ação válida**
  (proposta → **gate determinístico ZERO-LLM** → decisão → execução → trace/ledger → conhecimento).

A conclusão do estudo é que o Cosca **já possui** a maior parte do maquinário de evolução (gates de
promoção, regressão, A/B, canário, benchmark). O que falta **não é construir** um "Evolution Engine",
mas **wiringar** o que existe num loop ao vivo — e tapar 4 lacunas reais. A tese central a registrar:

> **O agente pode aprender livremente; o cérebro não precisa acreditar livremente.**
> *Self-improving, but evidence-gated.*

O Cosca já declara isso como política no `AUTO_EVOLUTION_PROTOCOL` (etapas 7-8: *"measure, then
promote — evidence-gated"*). Este ADR transforma essa política de **manual** em **loop automatizado** que
flui de execução real para skill versionada, passando pelo mesmo gate determinístico que rege tudo.

## 1. O que JÁ EXISTE (não reinventar — o maquinário de gate está pronto)

| Capacidade | Onde está | Prova (arquivo/função) |
|---|---|---|
| **Gate de promoção** (split 50/25/25 + holdout, fail-closed, determinístico) | `internal/skilleval` | `promote.go` → `NewPromotionGate(threshold, splitRatio, seed)`, `Evaluate(...)` |
| **Gate de regressão em holdout** (só holdout; rejeita regressão) | `internal/skilleval` | `regression.go` → `RegressionGate.CheckRegression(skill, baseline, candidate, holdout, scorer)` |
| **A/B / modo sombra** (candidato vs baseline) | `internal/skilleval` | `ab.go` → `RunAB(ctx, scorer, with, without, cases, trials, gate)` |
| **Evolução por mutação (GEPA/genome)** | `internal/skilleval` | `gepa.go`, `genome.go`, `mutate.go`, `ablation.go` |
| **Canário / release gradativa** | `internal/evals` | `canary.go` + `suite.go`, `runner.go`, `metrics.go`, `oracle.go`, `verify.go` |
| **Benchmark** | `internal/benchmark` | `benchmark.go`, `judge.go`, `scorer.go`, `runner.go` |
| **CLI de medição** (Roda A/B e persiste benchmark + histórico) | `internal/cli` | `skill_eval.go` → `cosca skill benchmark <name>` (persiste `.benchmark.json` + `.history.json`, `--dry-run`) |
| **CLI de promoção via PR** (branch `evolve/<skill>-<ts>`, guardrails + `CheckRegression.Passed`) | `internal/cli` | `skill_evolve.go` → `cosca skill evolve <name> --no-llm --dry-run` |
| **Intake governado** (proposta → quarentena → classificação epistêmica → veredicto) | `internal/proposal`, `internal/quarantine`, `internal/knowledge/epistemic*` | fluxo `propose → quarantine → validate → evidence → promote` |
| **Proveniência + ledger + trace** (versionamento e auditoria) | `internal/provenance`, `internal/ledger`, `internal/trace` | `Ledger` P1-P8, `ClaimKind`, `TraceID` + hash de entrada/saída |
| **Registro de aprendizado** (blockchain + Merkle) | `internal/memory` | `register.go` (bloco PREV/ID/TIME/LEVEL/TAGS + `chain.dat` + `merkle/epoch_*.json`) |
| **Nível de capacidade** (L0-L3) | `internal/capability` | `capability.go` |

**Ponto-chave:** o gate já é **ZERO-LLM** (métricas determinísticas testáveis com `go test`, sem redenção
de modelo) e o protocolo da família já manda *"a skill é promovida só quando os dados dizem — opinião nunca
promove uma skill"*. A invariante do Cosca está preservada.

## 2. O que FALTA (as lacunas reais)

A política está certa; o **circuito que a alimenta** não existe de forma automática. Hoje é manual: um agente
precisa lembrar de rodar `cosca skill benchmark` e `cosca skill evolve`. As lacunas:

| Lacuna | Descrição | Prioridade |
|---|---|---|
| **G1 — O feed ao vivo** | Execução real → `outcome` → `lesson` → **proposta** → case `.eval.yaml` → gate. Não há um "decoder" que transforme o resultado de uma tarefa real numa proposta de evolução + casos de avaliação. É a lacuna nº 1. | Alta |
| **G2 — Modelo de usuário epistêmico** | Perfil do Don/consumidor distinguindo `observed` / `confirmed` / `inferred`. Inferência ≠ fato (não vira conclusão automaticamente). | Alta |
| **G3 — Capability Graph (roteamento por capacidade)** | Descobrir a combinação mínima `tarefa → capacidade → skill/tool/modelo/ambiente/permissão`. Hoje `capability.go` é só L0-L3, sem grafo de descoberta. | Média |
| **G4 — Import de skill externa (agentskills.io)** | `import → quarantine → static-check → evaluation → gate → Cosca skill`. Skill externa = **dado não confiável** até promoção. | Média |
| **G5 — Execution Fabric (adapter remoto/cloud)** | Local/WASM como default; remoto/cloud como adapters opcionais. Auto-jail continua sendo o padrão de segurança. | Média |

## 3. A INVARIANTE — o Evolution Loop determinístico

Todo o loop é **aditivo** e preserva a fronteira `Domain Adapter → Generic Control Plane`. O gate permanece
**ZERO-LLM**. O LLM só participa na geração da *proposta* (a "ideia"), nunca na *decisão*.

```
 EXECUÇÃO (trace real)
     │
     ▼
 OUTCOME  ──────────► (G1) DECODER: outcome → lesson → PROPOSAL
     │                            (proposal.go — "a IA propõe")
     ▼
 CLASSIFICAÇÃO EPISTÊMICA  (FACT/MEASURED/EVIDENCE/INFERRED — nunca inferência virando fato)
     │
     ▼
 QUARENTENA (quarantine.go — a IA inventou? não entra direto)
     │
     ▼
 GATE DETERMINÍSTICO (ZERO-LLM — skilleval)
     │   ├── proveniência (provenance.go)          │
     │   ├── segurança (guardrail.go + contenttrust)│
     │   ├── regressão em holdout (regression.go)  │
     │   ├── benchmark A/B (ab.go, median+IQR)     │
     │   └── custo (skilleval/scorer.go)           │
     │
     ├── PASS →  VERSÃO + LEDGER (ledger/provenance) → OBSERVE → ROLLBACK se regredir
     └── FAIL →  REJECT (registrado, não promove)
```

- O **LLM** gera a candidata; o **sistema** decide se ela entra.
- Nada é auto-deployado: a promoção é via PR (`evolve/<skill>-<timestamp>`), revisada.
- Cada skill versionada carrega **fingerprint de benchmark + hash de proveniência** → rastreável até a
  origem (`Skill → lesson → execução → evidência`).

## 4. Reuso vs novo (régua da fronteira)

| Recurso | Reuso? | Justificativa |
|---|---|---|
| `skilleval` (AB/regressão/promoção/guardrails/GEPA) | **Reusar** | É o gate. Não duplicar. |
| `internal/evals` (canário/suite/oracle) + `internal/benchmark` | **Reusar** | O "shadow mode" e o benchmark já existem. |
| `proposal` + `quarantine` + `epistemic` | **Reusar** | É o intake governado. |
| `ledger` + `provenance` + `trace` | **Reusar** | Versionamento e auditoria. |
| `memory/register.go` | **Reusar** | Registro do aprendizado. |
| **Decoder outcome→proposta** (G1) | **NOVO** | Não existe. O coração do loop. |
| **Capability Graph** (G3) | **NOVO** | Não existe (capability.go é só níveis). |
| **User Model epistêmico** (G2) | **NOVO** | Não existe. |
| **Import agentskills.io** (G4) | **NOVO** (reusa quarantine) | Caminho de import não existe. |
| **Adapters remotos** (G5) | **NOVO** (reusa executor) | Não existe. |

## 5. Decisões e trade-offs

| Decisão | Trade-off / justificativa |
|---|---|
| **Wiringar o que existe** (reusar skilleval/evals/benchmark) em vez de criar um Engine novo | Evita duplicação e re-inventar gates que já são determinísticos e testados. O esforço vai para o feed (G1), que é o que falta. |
| **LLM só propõe; gate ZERO-LLM decide** (preservar P9) | Mantém o diferencial do Cosca: veredicto auditável e testável sem modelo. Copiar o `background_review` do Hermes (que auto-aplica) destruiria a soberania. |
| **Promoção via PR, nunca auto-deploy** (já é protocolo) | Garante revisão humana/Da gate antes de virar skill. Consistente com `AUTO_EVOLUTION_PROTOCOL` §Stages 7-8. |
| **Skill externa = dado não confiável** (G4 via quarentena) | Evita "baixei 200 skills = 200 pontos de ataque". |
| **Inferência ≠ fato** (G2 via epistemologia) | Só `observed`/`confirmed` viram fato; `inferred` fica marcado. É o que diferencia o Cosca de um `USER.md` ingênuo. |
| **Modelo como recurso, não identidade** (G3) | Roteamento por capacidade habilita "preciso coding + 128k + tool-calling + vision" → o Cosca escolhe o provider/modelo. |
| **Execução por risco** (G5) | baixo→local · isolado→WASM · pesado→GPU remoto · não confiável→sandbox forte. |

## 6. Verificação (critérios de aceite por lacuna)

- **G1:** dado um `outcome` real, o decoder produz uma `proposal` válida (R01-R06 satisfeitas) e um case
  `.eval.yaml`; ao rodar `cosca skill benchmark` e `cosca skill evolve --no-llm --dry-run` o gate decide
  `PASS`/`FAIL` apenas por dados. `go build ./...` + `go test ./internal/skilleval/... ./internal/evals/...` → **verde**.
- **G2:** `observed`/`confirmed`/`inferred` distintos; nenhuma `inferred` promove a `FACT` sem gate.
- **G3:** uma query retorna a combinação mínima de capacidades (skill/tool/modelo/ambiente/permissão) e a rota
  correspondente.
- **G4:** skill externa importada entra em quarentena; só promove após static-check + avaliação + gate.
- **G5:** executar mesma tarefa em local/WASM/remoto com o mesmo contrato `Executor`; auto-jail permanece default.

## 7. Ficou para depois (fora de escopo deste ADR)

- **Integração de um agente externo (ex.: Hermes) como executor da periferia** — toca soberania e merece ADR
  próprio (a decisão `stateless-reporter` vs `import como dado não confiável` é delicada). **Fora do escopo
  "melhorar o Cosca somente".**
- **Living World como capacidade operacional** — continua **arquitetura promissora, stub na execução**. Não
  entra no rol de diferencial consolidado (registrado como gap honesto).
- **Trajectory generation/compression para dataset/treino** — linha separada (G-futuro).

## 8. Referências

- Estudo comparativo: `docs/cosca-vs-hermes-agent-comparativo.pdf`.
- Política da família: `.opencode/cosca/shared/AUTO_EVOLUTION_PROTOCOL.md` (§Stages 7-8).
- Gate: `internal/skilleval/{promote,regression,ab,gepa,genome,mutate,ablation,guardrail,scorer,split,report}.go`.
- Canário/evals: `internal/evals/{canary,suite,runner,metrics,oracle,verify}.go`; `internal/benchmark/*.go`.
- CLI: `internal/cli/{skill_eval,skill_evolve}.go`.
- Intake governado: `internal/proposal`, `internal/quarantine`, `internal/knowledge/*epistemic*.go`.
- Auditoria: `internal/{provenance,ledger,trace}.go`; `internal/memory/register.go`.
- Contrato primitivo (controle de tasks): `internal/{task,orchestrator,decision}` (ADR-015).
