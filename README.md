# Cosca — Enterprise AI Orchestration System

<p align="center">
  <strong>53 Agents · 28 Skills · 34 Engines · 30 Workflows · Semantic Auto-Evolution Memory</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.26-00ADD8?logo=go" alt="Go Version">
  <img src="https://img.shields.io/badge/version-1.5.0-blue" alt="Version">
  <img src="https://img.shields.io/badge/build-passing-brightgreen" alt="Build">
  <img src="https://img.shields.io/badge/go_packages-181-brightgreen" alt="Packages">
  <img src="https://img.shields.io/badge/agents-53-blue" alt="Agents">
  <img src="https://img.shields.io/badge/skills-28-purple" alt="Skills">
  <img src="https://img.shields.io/badge/engines-34-green" alt="Engines">
  <img src="https://img.shields.io/badge/workflows-30-orange" alt="Workflows">
  <img src="https://img.shields.io/badge/knowledge_modular-ADROK013-brightgreen" alt="Modular Knowledge">
  <img src="https://img.shields.io/badge/platform-linux_|_macOS_|_Windows-blue" alt="Platform">
</p>

> **O Cosca não é um RAG otimizado — é um sistema de conhecimento com um RAG
> encaixado.** Ele decide onde procurar (router determinístico), só então busca
> (recuperação semântica confinada ao espaço roteado), valida contra a âncora
> (chain imutável) e reduz o universo **antes** de pagar o custo semântico.

---

## Overview

O Cosca é uma **orquestração de agentes** com um **cérebro de conhecimento curado**.
Ele não é um chatbot: é uma hierarquia operacional (`Don → Kernel → CEO → CTO →
Chiefs → Specialists`) onde o **Kernel roteia e nunca implementa diretamente** —
delega aos `capos` (Chiefs) que comandam os `soldados` (Skills).

Três pilares:

1. **Conhecimento modular** — base curada com proveniência (claims FACT/EVIDENCE/
   INFERENCE), grafo de entidades e busca híbrida (FTS5 + vetor + grafo).
2. **Memória semântica** — auto-evolução do cérebro (arquivos `.md` são "neurônios"),
   com snapshot/restore e lições de falhas.
3. **Governança & segurança** — gate de aprovação de planos, frontier de validação
   air-gap (`cofre`), chain imutável (Ed25519, anti-tamper) e gate de 100 MB por
   banco (ADR-013).

---

## Architecture

```
DON (autoridade) ── ordem ──► KERNEL (consigliere, roteia, NÃO implementa)
                                  │
                                  ▼
                              CEO (estratégia, nunca implementa) ──► CTO (técnica)
                                  │                                    │
                              Chiefs (capos) ◄─────────────────────────┘
                                  │
                            Specialists (soldados)
```

**Hierarquia real (53 agents):** Kernel + CEO + CTO + Chiefs de cada domínio
(backend, frontend, database, devops, security, performance, qa, ...) +
Specialists (backend-api, database-sql, testing-*, review-code, ...).

**O fluxo de trabalho** (nada roda sozinho — regra do Don):

```bash
cosca propose → cosca plan → cosca approve → cosca delegate → cosca run/gate
```

---

## Quick Start

### 1. Instalar

**Build local (Go 1.26+):**
```bash
go build -o cosca ./cmd/cosca
```

**Local AI (recomendado — Ollama):**
```bash
# Instale o Ollama, baixe um modelo e configure
cosca provider --help
# embeddings (para a busca semântica):
ollama pull nomic-embed-text
```

### 2. Inicializar e diagnosticar
```bash
cosca start        # init → install → doctor → sync → abre o editor
cosca doctor       # só diagnostica
cosca validate     # valida o setup
```

### 3. Subir o serviço (REST API)
```bash
cosca serve        # REST API + Web Console
# health:  curl http://127.0.0.1:14120/health
```

> **No Linux/WSL2:** o serve roda via systemd (`cosca-serve`, user `cosca`).
> Para subir manualmente: `wsl.exe -d Ubuntu-24.04 -u cosca -- systemctl --user start cosca-serve`.

---

## O fluxo de comando (a cadeia de valor)

O Cosca **não executa nada sem a sua ordem**. O fluxo de uma tarefa:

| Comando | O que faz |
|---|---|
| `cosca propose` | Submete proposta ao Kernel isolado (validação independente) |
| `cosca plan` | Estima o plano de execução ANTES da aprovação |
| `cosca approve` | Aprova o plano (roda testes + audit + registra) |
| `cosca gate` | Máquina de estados (guarda de papel): `new → approving → approved` |
| `cosca delegate` | Delega a tarefa com plano aprovado |
| `cosca run` | Executa via Orchestration Engine |
| `cosca exec` | Executa prompt não-interativo (CI/CD) |
| `cosca chat` | Sessão de chat interativa |

---

## Conhecimento & Busca (o cérebro curado)

```bash
cosca search "o que e a lei da familia"        # busca semântica (híbrida)
cosca knowledge search "gate de integridade"   # busca no conhecimento curado
cosca knowledge graph                          # grafo de entidades
cosca knowledge stats                          # estatísticas (docs, chunks, vetores, grafo)
cosca knowledge claim "<claim>"                # classifica (FACT/EVIDENCE/INFERENCE)
cosca knowledge evidence add <id>              # adiciona evidência com proveniência
cosca knowledge rebuild                        # reconstrói o índice (economia)
cosca memory search "conduta da chain"         # busca na memória
cosca session "conversa de ontem"              # busca FTS5 em sessões (zero LLM)
cosca symbols "func.*HandleCommand"            # busca de símbolos Go
```

**A arquitetura de conhecimento (ADR-013, modular):**

```
QUERY → Router determinístico (modlink) → Scope → Candidate IDs → busca confinada
                │
                ▼
        Router responde "ONDE"; Query responde "O QUÊ";
        Retriever responde "QUAIS candidatos"; Reranker responde "QUAIS melhores"
```

- **Reduz o universo antes de pagar o custo** — o `ScannedVectors` cai de 28.888
  para centenas/milhares (~99% de redução) **sem perder o recall do documento**
  relevante (provado por benchmark). **Medido:** 28.888 → 161–8.341 (71–99,44%),
  com recall do documento mantido em 5/6 queries.
- **Componentes da arquitetura modular:**
  - `internal/modlink` — **router determinístico** (whole-word match, decide ONDE).
  - `internal/vectoragg` — **read-model** (`ATTACH` read-only) que lê os módulos
    coesos de uma vez (projeção tipada), sem ser dono do processo.
  - `internal/search/scope.go` — confinamento por `DocumentPath` (não só `DocumentID`).
- **Índices derivados** (`knowledge.db`, vetores, grafo) são **regeneráveis** e
  ficam fora do git; só o Core imutável (chain + blocks) é versionado.
- **Guia completo de operação:** ver `docs/USO_COSCA.md` (comandos reais por fluxo).

---

## Memória Semântica (auto-evolução)

A memória vive em arquivos `.md` — "cada linha é um neurônio, cada referência é
uma sinapse". O Kernel aprende com cada sessão:

```bash
cosca memory register             # registra aprendizado (fluxo automático)
cosca memory list                 # lista memórias
cosca memory show <id>            # mostra uma
cosca memory snapshot create      # snapshot
cosca memory snapshot restore <id># restaura
cosca memory curated-failures     # lições de falhas (P5, sanitizadas)
```

**Governança de memória:** `learnings.md` (índice de gatilhos), `patterns.md`
(padrões reaplicáveis), `failures.md` (lições de erro), `evolution.md` (timeline
de capacidade). O protocolo de auto-evolução registra cada despertar.

---

## Governança & Segurança

| Comando | O que faz |
|---|---|
| `cosca cofre validate <file>` | Oráculo determinístico da zona Cofre (air-gap) |
| `cosca cofre gate` | Mostra as regras do Gate (auditoria) |
| `cosca db check --gate` | Gate de 100 MB por banco (Decisão 1, ADR-013) |
| `cosca integrity` | Verifica a chain/integridade (Ed25519, anti-tamper) |
| `cosca qgate` | Pre-commit quality gate (build, test, security) |
| `cosca security` | Scanner de vulnerabilidades de dependência |
| `cosca provenance` | Proveniência (integridade + licenças) |
| `cosca quarantine` | Zona de quarentena (o que a IA inventa não entra direto) |
| `cosca slop` | Fiscal anti-AI-slop |
| `cosca don` | Proteção de identidade do Don (war phrase) |

**A chain da família (crítico):** se você mexer em `internal/embed/cosca/` (o
cérebro), **DEVE re-assinar** a chain. Senão o serve **não sobe** (fail-closed
`family chain breach`). É proteção, não bug. **Duas variantes:**
- `cosca-check --sign` — **autoridade do Don** (chave Ed25519 + gate TTY/nonce).
  A variante correta quando o Don precisa autenticar a mudança.
- `cosca-check --sign-auto` — âncora git, **sem** autoridade do Don (testemunho de
  imutabilidade). Use só quando a mudança **não** exige assinatura do Don.

> **Estado atual:** chain com **30 blocks** (re-assinada com autoridade do Don em
> 2026-08-24 via `--sign`, não `--sign-auto`).

---

## Operação do serviço

```bash
cosca status                        # estado do sistema
cosca health                        # health check
cosca runtime start/stop/restart    # daemon do runtime
cosca runtime logs                  # logs
cosca metrics                       # métricas de orquestração
cosca fabric                        # Compute Fabric (pools, backpressure)
cosca hardware / cosca machine      # probe de hardware / capability profile
```

**Serve no WSL2 (autostart configurado):** o serve sobe sozinho no login do Windows
(pasta Startup → `cosca-serve-autostart.bat` invoca `wsl -d Ubuntu-24.04 -u cosca
-- systemctl --user start cosca-serve`). Para operar manualmente sem quebrar:

```bash
# ativo?              wsl -d Ubuntu-24.04 -u cosca -- systemctl --user is-active cosca-serve
# subir:              wsl -d Ubuntu-24.04 -u cosca -- systemctl --user start cosca-serve
# health:             curl http://127.0.0.1:14120/health
# NUNCA `sudo -u cosca` (erro 216/GROUP) — use `-u cosca`.
```

---

## Tech Stack

| Camada | Tecnologia |
|---|---|
| Backend | Go 1.26, cobra CLI, SQLite (`modernc.org/sqlite`) |
| Busca | FTS5 (BM25) + vetor (cosseno, HNSW/brute-force) + grafo (GraphDistance) |
| Embeddings | `nomic-embed-text` via Ollama (768-dim), local |
| IA | Providers OpenAI-compatíveis (Ollama, LM Studio, vLLM, llama.cpp) |
| Frontend | Next.js (Web Console) |
| Sandbox | bwrap (Linux), auto-jail (memfd_create), seccomp BPF |
| Integridade | Chain Ed25519 + blake3, git-anchored |
| Memória | Auto-evolução semântica em arquivos `.md`, SQLite FTS |

---

## Project Structure

```
.cosca/                  # runtime (knowledge.db, memória, snapshots) — índices derivados fora do git
internal/
  oracle/               # fronteira semântica + gate fail-closed
  search/               # motor híbrido (FTS+vetor+grafo), scope roteado
  modlink/              # router determinístico (ADR-013)
  vectoragg/            # read-model agregador (ATTACH read-only)
  sqlite/               # FTSClient, schema, migrations
  memory/               # memória semântica
  embed/                # cérebro read-only (go:embed) — chain assinada
  cli/                  # comandos cobra
  governor|gate|...     # governança
docs/
  USO_COSCA.md          # guia completo de operação
  MANUAL_AUTOAJUDA_COSCA.md  # metodologia de investigação
  adr/                  # ADRs (013: bancos modulares)
  reports/              # relatórios técnicos (benchmark, auditorias)
opencode/cosca/         # framework de agentes/skills (versionado)
```

---

## Build & Test

```bash
# Build
go build ./...

# Test (vet + race + build pass)
go vet ./...
go test ./... -race

# Quality gate (pré-commit)
make qgate          # fmt, vet, lint, contract-validate, test-unit, test-no-provider
cosca qgate
```

**Medido hoje:** 181 pacotes Go, build passing, vet pass, test -race pass.

---

## Stats (medidos)

| Métrica | Valor |
|---|---|
| Agentes (framework) | **53** |
| Skills | **28** (dirs) |
| Engines | **34** |
| Workflows | **30** |
| Go packages | **181** |
| Go version | **1.26** |
| Kosca versão | **1.5.0** |
| Knowledge base | 28.888 vetores · 36.539 entidades · 32.535 relações (no disco) |

---

## Binary Protection (Auto-Jail)

O binário é protegido: sem permissão de execução se não for chamado via a jaula.
No Linux, qualquer comando roda dentro de uma jaula (bwrap/memfd_create) que
restringe syscalls (seccomp), monta o workspace e valida com a chain.

---

## License

MIT — para uso da família Cosca. Ver `cosca license` para a chave de segurança.

---

> **Lei da Família:** Honestidade > Lealdade > Confiança; **Memória > Velocidade.**
> O Cosca não cresce por antecipação — cresce porque existe **necessidade
> demonstrada**. A Fatia 3 (mapeamento semântico módulo→domínio) só nasce quando
> houver conteúdo real de mundo com volume/fronteira. Até lá, o que existe está
> **provado e protegido**.
