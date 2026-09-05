# Plano Evolutivo — Borrow capabilities, not assumptions

> **Status:** Proposal (PLAN — nenhum código implementado) | **Owner:** cosca-kernel | **Data:** 2026-08-27
> **Ligação:** `docs/adr/ADR-016-evolution-engine.md` (arquitetura) · `dot.opencode` = `docs/adr` + `docs/roadmap`.
> **Regra-mãe:** nenhuma capacidade externa pode **degradar um invariante do Cosca**.

---

## 0. O que este documento é (e o que NÃO é)

É o **plano evolutivo autonomamente decidido** após o estudo de benchmarking arquitetural Cosca × Hermes Agent.
O Cosca olhou outro sistema, extraiu capacidade e decidiu **o que vale a pena incorporar**, **em que ordem**,
**quais invariantes ficam intocáveis** e **o que é rejeitado**.

> **NÃO implementa nada.** É o plano. A implementação é decisão do Don + delegação aos capos.

A tese que governa tudo:

> **Borrow capabilities, not assumptions.**
> O agente aprende livremente; o cérebro não precisa acreditar livremente.
> *Self-improving, but evidence-gated.*

## 1. Invariantes intocáveis (não-negoceáveis)

Independentemente do que vier de fora, estas **não podem regredir**:

| # | Invariante | Onde vive |
|---|---|---|
| I1 | **Governança determinística (P9)** — a IA propõe, o sistema decide. Gate ZERO-LLM. | `internal/proposal`, `internal/deliberate`, `internal/decision` |
| I2 | **Fail-closed absoluto** — sem veredicto, sem execução; erro/ambiguidade = nega. | `internal/proposal/flow.go`, `internal/semantic` |
| I3 | **Provenance** — "nunca inventar resultado"; ClaimKind + fonte + método. | `internal/provenance` |
| I4 | **Epistemologia** — FACT/MEASURED/EVIDENCE/INFERRED; inferência ≠ fato. | `internal/knowledge/*epistemic*.go` |
| I5 | **Ledger / auditoria criptográfica** — append-only, tamper-evidente, cadeia de hash. | `internal/ledger`, `internal/trace` |
| I6 | **Quarentena epistemológica** — o que a IA inventa não entra direto. | `internal/quarantine` |
| I7 | **Isolamento / execução controlada** — auto-jail, sandbox, rejeição a root. | `pkg/cosca/jail`, `internal/plugins` |
| I8 | **Segurança de conteúdo** — texto externo nunca concede autoridade. | `internal/contenttrust` |

**Regra de admissão de qualquer capacidade externa:** ela entra por um **ADAPTER**, passa por um
**INVARIANT CHECK**, e **só** é promovida se passar por **BENCHMARK/TEST** e pelo **EVOLUTION GATE**.
Se degradar I1–I8 → **rejeita**, por melhor que seja.

```
 EXTERNAL CAPABILITY
        ↓
     ADAPTER (borda de domínio; nunca no Kernel)
        ↓
   COSCA INVARIANT CHECK  (I1–I8)
        ↓
   BENCHMARK / TEST
        ↓
   EVOLUTION GATE
        ↓
    PROMOTE ... ou REJECT
```

## 2. Classificação das capacidades do Hermes (decisão própria)

| Descoberta do Hermes | Tratamento no Cosca | Impacto nos invariantes |
|---|---|---|
| Self-improving skills | **Adotar / adaptar** | Neutro→positivo em I1/I3/I4 (via gate) |
| Learning graph | **Adotar / adaptar** | Positivo em I3/I5 (grafo Skill→lesson→evidência) |
| User model | **Investigar** (epistêmico: observed/confirmed/inferred) | Preserva I4 |
| 37 providers | **Adotar arquitetura** (ProviderProfile → registry) | Neutro em I1 |
| 7 execution backends | **Avaliar** (como adapters; auto-jail segue default) | Preserva I7 |
| Messaging gateway | **Adotar como camada externa** (REST/gRPC já existem) | Neutro em I1 |
| Agentskills.io | **Compatibilidade** (formato, não abandono interno) | Preserva I6 (quarentena) |
| Delegation | **Benchmark + adaptar** (isolação de contexto, cancel, fan-out) | Neutro em I1 |
| Trajectory compression | **Investigar forte** (execução→dataset→treino) | Neutro em I1 |
| 200 skills | **NÃO copiar quantidade** (quantidade é consequência, não arquitetura) | — |
| LLM-driven judgment | **NÃO adotar** | Contrai I1/I2 |
| Ausência de provenance | **NÃO adotar** | Contrai I3/I5 |

## 3. O primeiro lever: a Skill como capacidade **verificável** (não mais "prompt salvo")

O Hermes mostrou uma mecânica boa: transformar experiência em capacidade reutilizável. O Cosca eleva isso a
**capacidade governada**, usando primitivas que **já existem**.

### 3.1 O artefato Skill carrega

```
proveniência      — de onde nasceu (observação/evolução/importação)
evidências        — quais execuções justificaram a criação
versão            — semântica (v1 → v2 …) + fingerprint de benchmark
autor/origem      — humano | agente | evolução | import
escopo            — domínio/módulo onde vale
pré-condições     — o que precisa existir para usar
permissões        — o que a skill pode tocar (nunca mais que o necessário)
dependências      — skills/engines/tools/modelos de que depende
benchmark         — A/B, median+IQR, taxa de sucesso
regressões conhecidas — holdout + histórico
TTL / aging       — ciclo de vida temporal (como memória em camadas)
status            — proposta → quarentena → validada → ativa → deprecated
```

### 3.2 Ciclo de vida (mapeia sobre primitivas existentes)

```
 TAREFA → EXECUÇÃO → RESULTADO → EXPERIÊNCIA → APRENDIZADO → SKILL PROPOSTA
     |        |           |             |             |
     |        |           |             |             └── proposal.go
     |        |           |             └── register.go (blockchain do aprendizado)
     |        |           └── trace (outcome verificável)
     |        └── execução real
     └── tarefa do mundo

 SKILL PROPOSTA
     ↓  quarantine.go  (pending → validating)
 VALIDAÇÃO (guardrails + contenttrust + estática)
     ↓
 GATE DETERMINÍSTICO — skilleval (A/B + regressão em holdout + custo + proveniência)
     │
     ├── PASS → (ledger/provenance) → VERSÃO + status ATIVA  → query/uso → OBSERVE → rollback se regredir
     └── FAIL → REJECT (registrado, não promove)

 ATIVA → aging (TTL) → DEPRECATED (se perde uso ou regride)
```

**Consequência estratégica:** quanto melhor o sistema, **menos ele depende do LLM para repetir raciocínio já
descoberto.** A experiência vira skill; a skill vira capacidade; a capacidade vira componente reutilizável.
O aprendizado deixa de ser memória e vira **evolução operacional**.

## 4. Ordem evolutiva (fases — o que vem primeiro)

| Fase | Entrega | Reusa | Novo | Invariantes |
|---|---|---|---|---|
| **F1 — Skill como artefato** | Elevar Skill ao modelo acima (proveniência/evidência/versão/autor/escopo/permissões/deps/benchmark/taxa de sucesso/regressões/aging/status) | `quarantine` (máquina de estados), `skilleval` (gates), `ledger/provenance`, `memory` (aging/TTL) | Schema completo da skill + status `ativa→deprecated` + aging de skill | I1, I3, I4, I5, I6 |
| **F2 — O feed ao vivo** | Decoder `outcome → lesson → proposta` + case `.eval.yaml` automático | `trace` (outcome), `proposal`, `register.go` | Decoder + geração de casos a partir de execução real | I1, I2 |
| **F3 — Modelo de usuário epistêmico** | `observed`/`confirmed`/`inferred`, sem auto-promoção de inferência a fato | epistemologia, `ledger` | User model governado | I4 |
| **F4 — Capability Graph** | Roteamento por capacidade `tarefa → skill/tool/modelo/ambiente/permissão` | `capability`, `routing`, `modelreg` | Grafo de descoberta + roteamento | I1 |
| **F5 — Import agentskills.io** | `import → quarantine → static-check → avaliação → gate → Cosca skill` | `quarantine`, `skilleval`, `plugins`, `skills/marketplace` | Parser/format + sandbox de import | I6, I7, I8 |
| **F6 — Execution Fabric** | Local/WASM default; remoto/cloud como adapters | `pkg/engine`, plugins, `agentbridge` | Adapters remotos + scheduler de execução por risco | I7 |

## 5. O que é rejeitado (e por quê)

- **LLM-driven judgment no gate** → contraria I1/I2 (o coração do Cosca). Rejeitado por princípio.
- **Ausência de provenance** → contraria I3/I5. Rejeitado por princípio.
- **Copiar 200 skills** → quantidade é consequência, não arquitetura. Foco em skills **verificáveis**, não em maço.
- **Auto-deploy de skill** → viola I1 (nenhuma melhoria entra sem gate + PR revisado).
- **Um agente externo (ex.: Hermes) com memória auto-evoluída na periferia** → **fora de escopo deste plano**;
  toca soberania e merece ADR próprio (decisão `stateless-reporter` vs `import como dado não confiável`).
  Registrado aqui como risco, não decidido.

## 6. Critérios de aceite (por fase)

- **F1:** toda skill ativa tem provenance + evidência + versão + benchmark + taxa de sucesso; status transita
  `proposta→quarentena→validada→ativa→deprecated`; nenhuma transição ilegal passa (semelhante à máquina da quarentena).
- **F2:** um `outcome` real gera uma proposta válida (R01–R06) + case `.eval.yaml`; `cosca skill benchmark` +
  `cosca skill evolve --no-llm --dry-run` decidem por dados. `go build ./...` + `go test ./...` verdes.
- **F3:** `inferred` nunca promove a `FACT` sem gate.
- **F4:** uma query retorna a combinação mínima de capacidades + a rota.
- **F5:** skill externa entra em quarentena; só promove após static-check + avaliação + gate.
- **F6:** mesma tarefa em local/WASM/remoto com o mesmo contrato `Executor`; auto-jail permanece default.

## 7. Você (Don) decide o escopo

Este plano não implementa nada. As fases F1–F6 são **propostas**. A execução é ordem do Don + delegação:

- **F1** → cosca-architecture + cosca-evolution + cosca-kernel
- **F2** → cosca-evolution + cosca-architecture
- **F3** → cosca-ai + cosca-architecture
- **F4** → cosca-architecture + cosca-backend
- **F5** → cosca-security + cosca-integrations
- **F6** → cosca-platform + cosca-infrastructure

## 8. Referências

- Estudo comparativo: `docs/cosca-vs-hermes-agent-comparativo.pdf`
- Arquitetura (gate/loop): `docs/adr/ADR-016-evolution-engine.md`
- Política da família: `.opencode/cosca/shared/AUTO_EVOLUTION_PROTOCOL.md` (§Stages 7-8)
- Primitivas reusadas: `internal/{proposal,quarantine,ledger,provenance,trace}.go`,
  `internal/skilleval/{ab,regression,promote,guardrail,scorer,split,report}.go`,
  `internal/evals/{canary,suite,oracle}.go`, `internal/memory/{register,layers}.go`,
  `internal/skills/{catalog,marketplace}.go`, `internal/capability/capability.go`
