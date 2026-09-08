# cosca-memory-chief - learnings.md EDITOR PRE-FIX

> Arquivo gerado em 20260908. Conteudo preservado - leia por grep, nunca inteiro.

# cosca-memory-chief — Semantic Learnings

> Auto-evolution memory. Search before acting. Record after learning.

## Seed Knowledge

### 2026-07-27 — Baseline
| Field | Value |
|-------|-------|
| **Agent** | cosca-memory-chief |
| **Task** | Initial capability establishment |
| **Technique** | Standard memory-chief patterns — project conventions |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #memory-chief #baseline #initialization |
| **Related** | .opencode/cosca/memory/codebase/overview.md |
| **Learned** | Project established. Core memory-chief patterns documented. Ready for Level 2 techniques. |
| **Next** | Level 2: Identify first advanced technique to master |

---

## Real Task Learnings

### 2026-07-29 — Complete Memory System Audit (Activation Task)
| Field | Value |
|-------|-------|
| **Agent** | cosca-memory-chief |
| **Task** | Activation audit: structure, INDEX.md, orphans, health, recommendations |
| **Technique** | Level 2 — Structural tree audit + content sampling across all memory categories (39 directories), cross-reference verification between INDEX.md and filesystem, orphan analysis (9 DNA v2.0 flat files), learning health assessment (39 activated vs 15 seed agents) |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #memory-chief #audit #activation #orphans #index-health #duplicates |
| **Related** | memory/INDEX.md, memory/agent/INDEX.md, memory/MEMORY_SYSTEM.md, agent/*.md (9 orphan files), memory/session/, memory/sessions/, memory/decision/, memory/decisions/ |
| **Learned** | 1) **Structural duplicates**: `session/` (vestigial — only INDEX.md) and `sessions/` (active + archive) exist in parallel; `decision/` (framework decisions) and `decisions/` (ADR decisions) have overlapping scopes. Need consolidation. 2) **Orphan count is 9, not 0**: memory/INDEX.md claims zero orphans but 9 flat agent-*.md files live unindexed in agent/. Three have valuable Cosca performance data (kernel, architecture, review, security); five are generic seed profiles from external framework; one (cosca-architecture) describes an external project (order-system). 3) **agent/INDEX.md references only 11/54 agents**: Major gap — 43 agent directories are invisible from the index. 4) **Template compliance is 100%**: All 54 agent directories have the full 6-file DNA v3.0 structure (capability-profile, evolution, failures, INDEX, learnings, patterns). |
| **Next** | Level 3: Implement semantic memory health dashboard; automate orphan detection; create automated INDEX.md generator for agent directories |

---

### 2026-07-28 — INDEX.md Rewrite (54 agents, department-organized)
| Field | Value |
|-------|-------|
| **Agent** | cosca-memory-chief |
| **Task** | P0 rewrite of agent/INDEX.md — from 11 entries to full 54-agent index |
| **Technique** | Level 2 — Cross-reference of filesystem directories (64 entries: 54 agent dirs + 9 orphans + INDEX.md) with agent capability-profile levels to determine active (L2+) vs seed (L1) status. Department grouping based on system-prompt reporting lines (Kernel, Leadership, Backend & Data, Frontend & Design, AI & Analytics, Platform & Infrastructure, Engineering & Tooling, Quality & Performance, Specialists, Bootstrap). |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #memory-chief #index-rewrite #p0 #54-agents #department-organization |
| **Related** | memory/agent/INDEX.md, memory/agent/ (54 agent directories), agent-*.md (9 orphan files) |
| **Learned** | 1) **54 agents confirmed**, not 53 as previously declared. 2) **22 agents are active (L2+)**, 31 are seed (L1), 1 is gated (paradigm). 3) Backend & Data has the most active agents (6/8). 4) All 9 specialists are seed — never executed a real task. 5) The 9 flat `agent-*.md` orphan files are vestigial DNA v2.0 artifacts — their data is superseded by v3.0 directory structures. |
| **Next** | Level 3: Automate INDEX.md generation from filesystem; implement health dashboard with active/seed/gated ratios; systematically migrate or archive orphan files. |

---

## 2026-09-02 - Curadoria de continuidade semantica de capacidades (Caso D - restart)

| Field | Value |
|-------|-------|
| **Agent** | cosca-memory-chief |
| **Task** | Caso D (restart): garantir que, ao despertar, o COSCA NAO regrida para "visao = VLM externo". Registrar/curar um aprendizado canonico e duravel que capture a RELACAO arquitetural (visao->capability->sensores->gate->escalacao) e a regra anti-regressao, referenciando ADR-036 como fonte canonica (sem duplicar a doutrina). |
| **Technique** | Level 3 - Diagnosticar o caminho de leitura do despertar (internal/cli/despertar.go) + a memoria semantica; mapear de onde vem a doutrina e se ela e recuperada; curar. **Fonte canonica**: ADR-036. |
| **Level** | 3 |
| **Outcome** | success (diagnostico + curadoria de memoria; nenhum `.go` alterado) |
| **Confidence** | 0.90 |
| **Tags** | #memory-chief #continuidade-semantica #despertar #capacidade #visao #gate-escalacao #ADR-036 #curadoria #anti-regressao |
| **Related** | docs/adr/ADR-036-continuidade-semantica-de-capacidades.md, internal/cli/despertar.go, internal/embed/cosca/SEMANTIC_AWAKENING_PROTOCOL.md, .opencode/cosca/memory/agent/cosca-kernel/learnings.md (entrada canonica adicionada 2026-09-02), .cosca/provenance.yaml (#dogma-continuidade-semantica-capacidades) |
| **Learned** | 1) **Diagnostico: o despertar recupera a RELACAO? NAO - recupera apenas ferramentas de forma dispersa.** O comando `cosca despertar` le SO a identidade (busca fixa `cosca kernel consigliere braco direito do Don identidade quem sou`) do knowledge.db, com zero LLM; nao carrega a doutrina de capacidade/visao. O `SEMANTIC_AWAKENING_PROTOCOL` tem 5 pilares (identidade/memoria/arquitetura/governanca/estado) e NENHUM cobre a cadeia visao->sensores->gate->escalacao. A busca semantica por "visao/capacidade/OCR/VLM" no knowledge.db devolve conteudo sobre *qual motor OCR* (Tesseract->PaddleOCR->VLM) e *"capacidade com evidencia"* - isto e **existencia de ferramenta**, nao a relacao. 2) **A RELACAO existe hoje em ADR-036 + dogmas do ledger** (`dogma-percepcao-como-evidencia`, `dogma-percepcao-por-representacao`) + learnings do kernel (L1320-1357), mas NAO estava na leitura do despertar e **ADR-036 NAO estava indexado no knowledge.db** (a busca por "ADR-036" retornou ADR-035/034/013, nao o 036). 3) **Curadoria aplicada (para o despertar recuperar):** adicionei (a) entrada canonica em `.opencode/cosca/memory/agent/cosca-kernel/learnings.md` (que e varrida/indexada no knowledge.db -> busca semantica a recupera), (b) dogma `dogma-continuidade-semantica-capacidades` em `.cosca/provenance.yaml` (fonte de verdade/duravel, referencia ADR-036 e os dogmas pre-existentes, nao re-escreve a doutrina). 4) **Escrever `memory register` aqui e inviavel/indevido** - ele grava no `internal/embed/cosca/memory/...` (cerebro embed read-only protegido pela family chain, Mandamento III) e exige autorizacao do Don no TTY (verifyDonIdentity). O correto, no dominio de memoria, foi editar os `.md`/ledger indexaveis. 5) **Nenhum `.go` funcional tocado; nenhum comportamento de runtime alterado** (ligar sensor->fusion->gate->escalacao e o GAP COMPROVADO do ADR-036 secao 6, decisao do Don/CTO, fora da curadoria de memoria). |
| **Next** | (1) Reportar ao Conselho o GAP de localizacao: `cosca despertar`/pillars nao carregam a doutrina - ou ampliar a leitura do despertar (decisao arquitetural/`.go`) ou confiar na recuperacao semantica via knowledge.db (que agora tem a entrada canonica indexavel). (2) Assegurar que ADR-036 entre no indice do knowledge.db (reindexar a varredura `docs/adr/`). (3) Nao duplicar: qualquer novo codigo consome `sensor.Observation`+`fusion.Fuse`+`gate.Policy.Decide`. |

