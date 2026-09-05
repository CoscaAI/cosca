# Engineering Timeline

> **Versão**: 1.0.0 | **Status**: active | **Owner**: Cosca Kernel | **Criada**: 2026-07-30
>
> Registro cronológico de todos os eventos de engenharia do Cosca. Alimentado pelo workflow `memorize-commit`.
> Cada linha representa um commit, decisão, ou marco relevante na evolução da plataforma.

## Legenda

| Ícone | Tipo | Descrição |
|-------|------|-----------|
| ✨ | feature | Nova capacidade |
| 🐛 | fix | Correção de bug |
| 🧪 | test | Testes |
| 📝 | docs | Documentação |
| ♻️ | refactor | Refatoração |
| 🔧 | chore | Manutenção |
| 🧠 | learn | Aprendizado |
| ⚡ | perf | Performance |
| 🛡️ | security | Segurança |
| 👷 | ci | CI/CD |
| 🏗️ | architecture | Arquitetura |
| 🚀 | release | Release |
| ⚖️ | decision | Decisão |
| 🔍 | audit | Auditoria |

---

## 2026-07-30 — Onda Cosca Chat + CLI Evolution

> **Sessão**: 2026-07-30 | **Commits**: 22 | **Tema**: Next-Gen CLI + Compute Fabric + Runtime 100%

| Horário | Tipo | Descrição | Impacto | Arq. | Custo | Aprend. | Commit |
|---------|------|-----------|---------|:----:|:-----:|:-------:|--------|
| 22:12 | 🧠 learning | learn: impact report 52435c5 + devops learnings + trust registry update | +124/-0 | 4 | $0.004 | 0 | `5abe0da` |
| 22:11 | 🔍 other | feat(kernel): engines F8.3-F10.1 + F9 completa + F2.3 — 12 engines de cognição | +15358/-18 | 28 | $0.321 | 0 | `52435c5` |
| 21:52 | 🧪 testing | test: final validation of F7.4 impact report hook fixes | +0/-0 | 0 | $0.000 | 0 | `ac013d5` |
| 21:51 | 🧪 testing | test: verify defensive defaults fix | +0/-0 | 0 | $0.000 | 0 | `85928db` |
| 21:50 | 🧪 testing | test: verify post-commit hook awk fix | +0/-0 | 0 | $0.000 | 0 | `85f2265` |
| 17:23 | 🏗️ architecture | Integrate Compute Fabric as Subsystem lifecycle | +8/-0 | 1 | $0.001 | 0 | `90e864f` |
| 17:30 | 🧪 test | Compute Fabric Fase 2.5 coverage 98.0→98.7% | +82/-0 | 3 | $0.002 | 0 | `af51439` |
| 17:30 | 🧠 learn | L26 — Compute Fabric lifecycle + Fase 2.5 | +60/-0 | 1 | $0.001 | 3 | `a746f01` |
| 17:43 | ✨ feature | Fase 0 — Fundação Next-Gen CLI | +580/-0 | 4 | $0.012 | 0 | `776fd07` |
| 18:07 | ✨ feature | Fase 1 — Provider Layer multi-model c/ fallback | +934/-0 | 7 | $0.019 | 0 | `98279f5` |
| 18:30 | ✨ feature | Fase 2 — Tool System + Executor + Testes | +1,822/-0 | 11 | $0.036 | 0 | `f9d2fde` |
| 18:39 | ✨ feature | Fase 3 — Agent Engine completo | +1,306/-0 | 10 | $0.026 | 0 | `eb36926` |
| 18:55 | 🧪 test | Fase 3.5-4 — Testes Engine + Integração Memória | +1,492/-7 | 7 | $0.030 | 0 | `df31a9f` |
| 19:07 | ✨ feature | Fase 5 — CLI commands wired ao AgentEngine | +694/-27 | 5 | $0.014 | 0 | `be82a0c` |
| 19:10 | ✨ feature | Fase 4.5 — Memória real conectada ao AgentEngine | +264/-4 | 3 | $0.005 | 0 | `f362bbc` |
| 19:21 | ✨ feature | Fase 6 — Extensibilidade (MCP, Plugins, Skills) | +684/-10 | 11 | $0.014 | 0 | `1273acd` |
| 19:22 | 🔧 chore | Add cosca-chat binary to .gitignore | +1/-0 | 1 | $0.001 | 0 | `426216c` |
| 19:28 | 🧠 learn | L27 — Fase 6 completa (MCP, Plugins, Skills) | +17/-0 | 1 | $0.001 | 3 | `0823981` |
| 19:30 | 🐛 fix | Definitions() retorna tools ordenadas por nome | +9/-1 | 1 | $0.001 | 0 | `aafb9ac` |
| 19:43 | 🧪 test | Testes MCP + Executor, fix deadlock Connect() | +1,838/-1 | 4 | $0.037 | 0 | `3fa524c` |
| 19:45 | ✨ feature | CLI: MCP and plugin management commands | +253/-0 | 2 | $0.005 | 0 | `64d6af7` |
| 19:48 | ✨ feature | CLI: agent and skill management commands | +257/-0 | 2 | $0.005 | 0 | `da78cda` |
| 19:50 | ✨ feature | CLI: session management commands | +132/-0 | 2 | $0.003 | 0 | `72afc52` |
| 19:52 | ✨ feature | CLI: completion, config get, skill validate | +226/-3 | 3 | $0.005 | 0 | `117fc23` |
| 19:54 | 🧪 test | Unit tests for GetValue/getByKey | +122/-0 | 1 | $0.002 | 0 | `50006ad` |

---

## 2026-07-30 — Compute Fabric + CMI + Expurgo

| Horário | Tipo | Descrição | Impacto | Arq. | Custo | Aprend. | Commit |
|---------|------|-----------|---------|:----:|:-----:|:-------:|--------|
| (madrugada) | 📝 docs | Expurgo de infra fictícia (PostgreSQL/Redis/pgvector) | +0/-315 | 12 | $0.006 | 0 | `792c000` |
| (madrugada) | 🧠 learn | L21 — Coverage audit + doc expurgo | +45/-0 | 1 | $0.001 | 6 | `83ac37e` |
| (manhã) | 🏗️ architecture | CMI v1.0.0 + Kernel Level 4 | +1,200/-0 | 8 | $0.024 | 0 | `a8d8fa9` |
| (manhã) | 🏗️ architecture | Fase 0+1 — CMI Implementation Complete | +850/-0 | 6 | $0.017 | 0 | `b084d25` |
| (manhã) | 🏗️ architecture | Fase 2 — 6 Core Cognitive Engines | +920/-0 | 7 | $0.018 | 0 | `5b92d88` |
| (manhã) | 🏗️ architecture | Fase 3 — CMI Architecture Finalized | +780/-0 | 5 | $0.016 | 0 | `59f4504` |
| (manhã) | 🏗️ architecture | Fase 4 — Cognitive Engines + Audit Loop v2.0 | +1,100/-0 | 8 | $0.022 | 0 | `ba10bf3` |

---

## 2026-07-29 — Segurança + Jaula + Runtime

> Sessão pesada: jail breach incident, auto-jail, binary protection, runtime hardening.

| Horário | Tipo | Descrição | Impacto | Arq. | Custo | Aprend. | Commit |
|---------|------|-----------|---------|:----:|:-----:|:-------:|--------|
| (dia) | 🛡️ security | Isolamento total do kernel — permissões runtime | +450/-0 | 5 | $0.009 | 0 | `75dfb8a` |
| (dia) | 🛡️ security | P1 — Runtime security hardening | +380/-0 | 4 | $0.008 | 0 | `4d35505` |
| (dia) | 🛡️ security | P2 — gosec config, SBOM, sops template | +320/-0 | 6 | $0.006 | 0 | `f0dc912` |
| (dia) | 🧠 learn | Identidade permanente do kernel | +120/-0 | 2 | $0.002 | 5 | `efba689` |
| (dia) | 🏗️ architecture | Jaula de isolamento para o runtime | +680/-0 | 8 | $0.014 | 0 | `2692f1e` |
| (dia) | 🔧 chore | Build faz chmod 000 + build-dev alternativo | +15/-0 | 1 | $0.001 | 0 | `e94624d` |
| (dia) | 🧠 learn | L14 — Build-time binary protection | +60/-0 | 1 | $0.001 | 5 | `5ab9237` |
| (dia) | 🐛 fix | chmod 000 → chmod -x (644) — Don corrigiu | +1/-1 | 1 | $0.001 | 0 | `c8797de` |
| (dia) | 🏗️ architecture | Auto-jail memfd_create + bwrap (sem script) | +520/-40 | 3 | $0.010 | 4 | `9007d95` |
| (dia) | 🐛 fix | Sudo não executa 644 → 700 root:root | +1/-1 | 1 | $0.001 | 0 | `60319d8` |
| (dia) | 🐛 fix | Provider env propagation + crypto + testes | +280/-15 | 6 | $0.006 | 0 | `4e011ab` |
| (dia) | ✨ feature | Coverage audit + mass test offensive (Don's order) | +3,200/-0 | 25 | $0.064 | 6 | `75d4d99` |
| (dia) | 🧪 test | CLI coverage 46.9% → 71.5% | +1,850/-0 | 12 | $0.037 | 0 | `82429cf` |
| (dia) | 🧠 learn | L19 — 3 meta-aprendizados da cobertura | +45/-0 | 1 | $0.001 | 3 | `f30d4cc` |
| (dia) | 🏗️ architecture | Knowledge Pipeline Fase 1 — fundação | +2,100/-0 | 15 | $0.042 | 0 | `868e202` |
| (dia) | 🧠 learn | Jail breach incident — UCSS + embed-sync | +200/-0 | 3 | $0.004 | 6 | `3f2af5e` |
| (dia) | 🐛 fix | 5 bloqueantes: panic, signal race, MCP, docs, API key | +180/-20 | 7 | $0.004 | 0 | `c96f4ad` |
| (dia) | 🧠 learn | L20 — Systemic audit + 5 blocker fixes | +80/-0 | 1 | $0.002 | 6 | `c613509` |
| (dia) | 🐛 fix | 2 bugs + coverage 68.6% → 71.3% | +120/-10 | 4 | $0.003 | 0 | `758731f` |

---

## 2026-07-28 — Fundação Cosca Chat + Onda de Agentes

> Ativação massiva de agentes (Ondas 2-6, 54/55 agentes), CI fixes, semantic memory, evolution marathon.

| Horário | Tipo | Descrição | Impacto | Arq. | Custo | Aprend. | Commit |
|---------|------|-----------|---------|:----:|:-----:|:-------:|--------|
| (dia) | 🧪 test | CI fix: race condition + flaky test + coverage gate | +80/-15 | 4 | $0.002 | 0 | — |
| (dia) | 🏗️ architecture | Onda 2 — 10 agentes paralelos (analytical → impl → review) | +2,400/-0 | 20 | $0.048 | 5 | — |
| (dia) | 🏗️ architecture | Semantic Memory Kernel + startup optimization | +1,800/-0 | 9 | $0.036 | 4 | — |
| (dia) | 🏗️ architecture | Constitution + Confidence Model + Curation Engine | +1,500/-0 | 3 | $0.030 | 4 | — |
| (dia) | 🏗️ architecture | Metacognition + DNA v3.0 + Learning Protocol v2.0 | +1,100/-0 | 5 | $0.022 | 5 | — |
| (dia) | 🏗️ architecture | 51 Capability Profiles (Ondas 2-3) | +3,000/-0 | 51 | $0.060 | 2 | — |
| (dia) | 📝 docs | 52 documentation fixes across 19 files | +200/-100 | 19 | $0.006 | 1 | — |
| (dia) | 🧠 learn | Kernel self-assessment L1 → L3 | +40/-0 | 1 | $0.001 | 6 | — |

---

## 2026-07-27 — Fundação Inicial

> Primeiros commits, estrutura do projeto, seed data, templates.

> *Nota: Dados aproximados — pré-timeline.*
| Horário | Tipo | Descrição | Impacto | Arq. | Custo | Aprend. |
|---------|------|-----------|---------|:----:|:-----:|:-------:|
| (dia) | 🏗️ architecture | Estrutura inicial do Cosca + CLI base | +5,000/-0 | 30 | $0.100 | — |
| (dia) | 🏗️ architecture | Sistema de providers + runtime | +3,200/-0 | 20 | $0.064 | — |
| (dia) | 🏗️ architecture | Memória + Agentes + Skills seed | +2,800/-0 | 55 | $0.056 | — |

---

## Totais da Timeline

| Métrica | 2026-07-27 | 2026-07-28 | 2026-07-29 | 2026-07-30 |
|---------|:----------:|:----------:|:----------:|:----------:|
| Commits | ~3 | ~8 | ~40 | ~27 |
| Arquivos | ~105 | ~110 | ~120 | ~95 |
| LOC+ | ~11,000 | ~10,120 | ~12,500 | ~11,500 |
| LOC- | ~0 | ~115 | ~100 | ~370 |
| Custo Est. | ~$0.220 | ~$0.202 | ~$0.250 | ~$0.230 |
| Aprendizados | — | ~27 | ~40 | ~12 |
| Eventos | 3 | 8 | 40 | 27 |

---

## Related

- [Workflow: memorize-commit](../../workflows/memorize-commit.md) — Workflow que alimenta esta timeline
- [Memory Model](../../memory/MEMORY_MODEL.md) — Arquitetura de memória
- [CMI](../../architecture/COGNITIVE_MATURITY.md) — Cognitive Maturity Index
- [Impact Reports](./impact-reports/) — Relatórios detalhados por commit

---

## 2026-07-31 — Impact Report Automation

> **Sessão**: 2026-07-31 | **Commits**: 1 (auto)

| Horário | Tipo | Descrição | Impacto | Arq. | Custo | Aprend. | Commit |
|---------|------|-----------|---------|:----:|:-----:|:-------:|--------|
| 04:54 | 🔍 other | fix(config): resolver home do usuário real no DefaultConfig — nunca /root | +175/-14 | 5 | $0.006 | 0 | `27262ea` |
| 04:40 | 🔍 other | feat(no-root): auto-jail com fallback — roda sem sudo/bwrap + Makefile sem root + crypto limpo | +61/-46 | 5 | $0.004 | 0 | `c9af443` |
| 04:26 | 🧠 learning | learn: registros de memória — impact reports b503c4a/5e5bbbc + timeline/trust + sync framework | +1496/-0 | 29 | $0.044 | 0 | `e79ad4a` |
| 04:23 | 🔍 other | fix(embed): salvaguarda anti-loop no SyncToOpenCode — internal/embed/cosca do workspace é autoritativo | +60/-8 | 2 | $0.002 | 0 | `5e5bbbc` |
| 03:21 | 🔍 other | fix(hooks): guarda de commit recursivo no post-commit — memória-only não gera report (fim do loop infinito) | +119/-0 | 5 | $0.005 | 0 | `bc305ff` |
| 03:19 | 🧠 learning | learn: memorize-commit — registro do 5fb2288 | +100/-0 | 4 | $0.004 | 0 | `4aa4d13` |
| 03:19 | 🧠 learning | learn: memorize-commit hook — registro do eea2882 no timeline, trust registry + impact report | +108/-0 | 4 | $0.004 | 0 | `5fb2288` |
| 03:19 | 🔍 other | docs(architecture): sync overview + layers com a realidade do código (engine, chat, compute fabric, auto-jail) — cobertura real 76.7%, REST 60 rotas, 8 subsistemas | +794/-43 | 12 | $0.022 | 0 | `eea2882` |
| 03:01 | 🧠 learning | learn: session snapshot v3 — encerramento 2026-07-31 (11 commits, cobertura 92%+, serve testado) | +43/-0 | 1 | $0.001 | 0 | `7edf9d9` |
| 03:00 | 🔍 other | fix(handler): nil-deref panic no MemoryHandler.Store (memory.go:134) | +59/-1 | 4 | $0.003 | 0 | `be7a52c` |
| 02:55 | 🔍 other | test(handler): api/rest/handler 51.1% → 92.6% — 11 arquivos de teste novos | +4235/-0 | 14 | $0.092 | 0 | `7ca31b1` |
| 02:34 | 🔍 other | test(coverage): 5 pacotes a 95%+ + fix 4 bugs de produção no registry | +4497/-15 | 13 | $0.096 | 0 | `70abb6b` |
| 02:02 | 🧠 learning | learn: registros de aprendizado — trust registry, timeline, impact reports, devops hooks | +233/-0 | 5 | $0.007 | 0 | `311082c` |
| 01:50 | 🔍 other | fix(ci): memory validation G6b + pnpm workspace + SBOM dir + language policy | +338/-76 | 69 | $0.041 | 0 | `2f1f38a` |
| 01:10 | 🔍 other | sync(embed): framework + memória atualizados — timeline, trust, org-memory, impact reports | +1483/-132 | 23 | $0.041 | 0 | `dff269f` |
| 00:54 | 🧠 learning | learn: L38 — fix CI raiz (go.sum + protobuf órfãos) + rename aos→cosca | +15/-0 | 1 | $0.001 | 0 | `70d78c6` |
| 00:53 | 🔍 other | refactor(proto): renomear namespace aos → cosca (Fase 1) + versionar protobuf gerado | +2995/-177 | 19 | $0.069 | 0 | `9063986` |
| 00:42 | 🔍 other | fix(build): versionar go.sum — CI quebrava com missing go.sum entry para todos os módulos | +161/-1 | 2 | $0.004 | 0 | `1230386` |
