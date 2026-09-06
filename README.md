# Cosca — Enterprise AI Orchestration System

<p align="center">
  <strong>53 Agents · 29 Skills · 30 Workflows · Semantic Auto-Evolution Memory</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.26.7-00ADD8?logo=go" alt="Go Version">
  <img src="https://img.shields.io/badge/version-1.5.0-blue" alt="Version">
  <img src="https://img.shields.io/badge/build-passing-brightgreen" alt="Build">
  <img src="https://img.shields.io/badge/agents-53-blue" alt="Agents">
  <img src="https://img.shields.io/badge/skills-29-purple" alt="Skills">
  <img src="https://img.shields.io/badge/workflows-30-orange" alt="Workflows">
  <img src="https://img.shields.io/badge/security-nohigh%2Fnocritical-brightgreen" alt="Security">
  <img src="https://img.shields.io/badge/platform-linux_|_macOS_|_Windows-blue" alt="Platform">
</p>

> **O Cosca não é um RAG otimizado — é um sistema de conhecimento com um RAG
> encaixado.** Ele decide onde procurar (router determinístico `modlink`), só
> então busca (recuperação semântica confinada ao módulo roteado), valida contra
> a âncora (chain imutável) e reduz o universo **antes** de pagar o custo
> semântico.

---

## Overview

O Cosca é uma **orquestração de agentes** com um **cérebro de conhecimento
curado**. Ele não é um chatbot: é uma hierarquia operacional
(`Don → Kernel → CEO → CTO → Chiefs → Specialists`) onde o **Kernel roteia e
nunca implementa diretamente** — delega aos `capos` (Chiefs) que comandam os
`soldados` (Skills).

Três pilares:

1. **Conhecimento modular (ADR-013)** — base curada com proveniência (claims
   FACT/EVIDENCE/INFERENCE), grafo de entidades e busca híbrida (FTS5 + vetor +
   grafo), particionada em módulos físicos `vector-*.db` (< 100 MB cada).
2. **Memória semântica** — auto-evolução do cérebro (arquivos `.md` são
   "neurônios"), com snapshot/restore e lições de falhas.
3. **Governança & segurança** — gate de aprovação de planos, frontier de
   validação air-gap (`cofre` / WSL2 + bwrap), chain imutável (Ed25519,
   anti-tamper), loopback-only e dependências sem HIGH/CRITICAL.

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

**Hierarquia real (53 agents no framework):** Kernel + CEO + CTO + Chiefs de
cada domínio (backend, frontend, database, devops, security, performance, qa,
...) + Specialists (backend-api, database-sql, testing-*, review-code, ...).

**O fluxo de trabalho** (nada roda sozinho — regra do Don):

```bash
cosca propose → cosca plan → cosca approve → cosca delegate → cosca run/gate
```

---

## Fluxo de Branches (GitHub · CoscaAI/cosca)

O projeto opera com **uma branch de trabalho ativa** e o histórico consolidado:

| Branch | Papel | Estado |
|---|---|---|
| `cosca-database` | **Branch de trabalho ativa** — desenvolvimento e integração | 🟢 atual |
| `master` | **Espelho do estado consolidado** — reflete `cosca-database` | 🟡 sincronizada |

> O auto-update (post-commit) é **agnóstico de branch**: dispara em qualquer
> branch — desde que o diff toque código.

---

## Quick Start

### 1. Instalar

**Build local (Go 1.26.5+):**
```bash
go build -o cosca ./cmd/cosca
```

**Local AI (recomendado — Ollama):**
```bash
cosca provider --help
ollama pull nomic-embed-text        # embeddings para a busca semântica
```

### 2. Inicializar e diagnosticar
```bash
cosca doctor       # só diagnostica (runtime, memória, knowledge, providers, security)
cosca db check --gate   # gate de 100 MB por banco (ADR-013), READ-ONLY
cosca security scan     # scan de vulnerabilidades de dependência
```

### 3. Subir o serviço (REST API)
```bash
cosca serve                 # REST API + Web Console, escuta SÓ loopback
# health:  curl http://127.0.0.1:14120/health
```

> **No Linux/WSL2:** o serve roda via systemd (`cosca-serve`, user `cosca`).
> Para subir manualmente: `wsl.exe -d Ubuntu-24.04 -u cosca -- systemctl --user start cosca-serve`.

> **No Windows:** o serve precisa das envs no ambiente do processo. A forma
> confiável é um `.bat`:
> ```bat
> @echo off
> set "COSCA_ALLOW_NO_ROOT=1"
> set "COSCA_PROVIDER=ollama"
> set "COSCA_OLLAMA_MODEL=cosca-qwen3-4b-lora-001:latest"
> cd /d C:\Users\Henrique\Documents\cosca
> bin\cosca.exe serve
> ```
> O aviso de jail é informativo (opt-in); portas reais: **14120** (REST) /
> **14121** (metrics) / **14122** (gRPC).

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
cosca routes lint                              # valida o registry do router determinístico
cosca db check --gate                          # gate de 100 MB por banco (ADR-013)
```

**A arquitetura de conhecimento (ADR-013, modular):**

```
QUERY → Router determinístico (modlink) → Scope → Candidate IDs → busca confinada
                │
                ▼
        Router responde "ONDE"; Query responde "O QUÊ";
        Retriever responde "QUAIS candidatos"; Reranker responde "QUAIS melhores"
```

- **Os vetores NÃO moram mais no monólito.** Desde o fix do CLI (2026-09-04), o
  `status`/`verify`/`search` leem os **módulos físicos** `vector-*.db`
  (particionamento por domínio), não a tabela do `knowledge.db` legado.
  **Medido hoje: 55.453 vetores reais** (antes o CLI reportava `0`).
- **Módulos do banco (ADR-013), cada um < 100 MB:**
  - `vector-code` · `vector-docs` · `vector-embed-core` · `vector-embed-engines`
    · `vector-embed-memory` · `vector-fallback` · `vector-opencode` ·
    `vector-other`
  - `core.db` · `graph.db` · `projects.db` (estrutura/grafo/projetos)
  - `knowledge.db` (585 MB) permanece como **legado** — fora do fluxo modular
    e fora do `gold-modular`.
- **Leitura via:** `PartitionStore` + `vectoragg` (read-model `ATTACH`
  read-only) + `modlink` (router determinístico → scope → busca confinada).
- **Reduz o universo antes de pagar o custo** — o `ScannedVectors` cai de 28.888
  para centenas/milhares (~99% de redução) **sem perder o recall** do documento
  relevante (provado por benchmark).
- **Índices derivados** (vetores, grafo) são **regeneráveis** e ficam fora do
  git; só o Core imutável (chain + blocks) é versionado.
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
| `cosca db check --gate` | Gate de 100 MB por banco (Decisão 1, ADR-013), READ-ONLY |
| `cosca integrity` | Verifica a chain/integridade (Ed25519, anti-tamper) |
| `cosca qgate` | Pre-commit quality gate (build, test, security) |
| `cosca security scan` | Scanner de vulnerabilidades de dependência (osv-scanner) |
| `cosca provenance` | Proveniência (integridade + licenças) |
| `cosca quarantine` | Zona de quarentena (o que a IA inventa não entra direto) |
| `cosca slop` | Fiscal anti-AI-slop |
| `cosca don` | Proteção de identidade do Don (war phrase) |

**Estado de segurança (2026-09-04):**
- **Dependências sem HIGH/CRITICAL.** `grpc v1.83.2` (fecha
  `GHSA-vp52-pcj8-j9qc`, OOM HTTP/2 HIGH) e `x/crypto v0.56.0` (fecha DoS ssh).
  Residual apenas `GO-2026-5932` (openpgp UNKNOWN, **sem uso no código** — usa
  bcrypt).
- **Apenas loopback.** O serve escuta em `127.0.0.1` (porta 14120). Auth
  obrigatória (401 sem token), registro público **desabilitado**, CORS
  **desabilitado**, nenhuma chave de provider externa (só Ollama local — **Lei do
  Cofre** / `MODEL_PROTOCOL §5`). `COSCA_JWT_SECRET` é variável persistida do
  usuário.
- **Sandbox honesto:** no Windows **não há bwrap** → roda sem sandbox
  (`COSCA_ALLOW_NO_ROOT=1`). Código não confiável só deve rodar na zona Cofre /
  WSL2+bwrap. **No WSL2 (Ubuntu-24.04)** o bwrap 0.9.0 funciona no kernel WSL2;
  Go 1.26.7 instalado; `cosca-serve.service` ativo (systemd, user `cosca`).
  `COSCA_BIN` do hook alinhado ao `ExecStart`.

**A chain da família (crítico):** se você mexer em `internal/embed/cosca/` (o
cérebro), **DEVE re-assinar** a chain. Senão o serve **não sobe** (fail-closed
`family chain breach`). É proteção, não bug. **Duas variantes:**
- `cosca-check --sign` — **autoridade do Don** (chave Ed25519 + gate TTY/nonce).
- `cosca-check --sign-auto` — âncora git, **sem** autoridade do Don.

> **Estado atual (2026-09-06):** chain renovada + **ativa** no Windows (blocos
> git-anchored). Após QUALQUER commit que toque `internal/embed/cosca/`, re-assine
> com `cosca-check --sign-auto`.

---

## Auto-Update (post-commit)

O Cosca é auto-atualizante em mudança de **código** (não de docs). Ativado via
`core.hooksPath = .githooks`:

- **Dispara** quando mudam: `cmd/`, `internal/`, `pkg/`, `api/`, `sdk/`,
  `go.mod`, `go.sum`, `Makefile`.
- **Não dispara** quando mudam apenas docs/`.opencode`/`.cosca`/`.githooks`.
- **Windows** (`.cmd`): rebuild `bin\cosca.exe` → mata/relança o serve na porta
  14120.
- **Linux** (bash): rebuild `bin/cosca` → `systemctl --user restart
  cosca-serve.service` (ou relança em background se não houver systemd).
- **Agnóstico de branch** e **nunca trava o commit** (sempre `exit 0`, roda em
  background/setsid). Erros de build preservam o binário anterior, sem restart.

---

## Operação do serviço

```bash
cosca status                        # estado do sistema (inclui Vectors modulares)
cosca health                        # health check
cosca runtime start/stop/restart    # daemon do runtime
cosca runtime logs                  # logs
cosca metrics                       # métricas de orquestração
cosca fabric                        # Compute Fabric (pools, backpressure)
cosca hardware / cosca machine      # probe de hardware / capability profile
```

**Serve no WSL2 (autostart):** `cosca-serve-autostart.bat` (pasta Startup)
invoca `wsl -d Ubuntu-24.04 -u cosca -- systemctl --user start cosca-serve`.
Para operar manualmente:
```bash
# ativo?          wsl -d Ubuntu-24.04 -u cosca -- systemctl --user is-active cosca-serve
# subir:          wsl -d Ubuntu-24.04 -u cosca -- systemctl --user start cosca-serve
# health:         curl http://127.0.0.1:14120/health
# NUNCA `sudo -u cosca` — use `-u cosca`.
```

---

## Tech Stack

| Camada | Tecnologia |
|---|---|
| Backend | Go 1.26.5, cobra CLI, SQLite (`modernc.org/sqlite`) |
| Busca | FTS5 (BM25) + vetor (cosseno, HNSW/brute-force) + grafo (GraphDistance) |
| Conhecimento | Módulos `vector-*.db` (< 100 MB) + `PartitionStore`/`vectoragg`/`modlink` |
| Embeddings | `nomic-embed-text` via Ollama (768-dim), local |
| IA | Providers OpenAI-compatíveis (Ollama, LM Studio, vLLM, llama.cpp) |
| Frontend | Next.js (Web Console) |
| Sandbox | bwrap (Linux/WSL2), auto-jail (memfd_create), seccomp BPF |
| Integridade | Chain Ed25519 + blake3, git-anchored |
| Memória | Auto-evolução semântica em arquivos `.md`, SQLite FTS |

---

## Project Structure

```
.cosca/                  # runtime (core.db, graph.db, projects.db, vector-*.db, memória, snapshots) — fora do git
internal/
  oracle/               # fronteira semântica + gate fail-closed
  search/               # motor híbrido (FTS+vetor+grafo), scope roteado
  modlink/              # router determinístico (ADR-013)
  vectoragg/            # read-model agregador (ATTACH read-only)
  sqlite/               # FTSClient, schema, migrations
  memory/               # memória semântica
  embed/                # cérebro read-only (go:embed) — chain assinada
  cli/                  # comandos cobra
docs/
  USO_COSCA.md          # guia completo de operação
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

---

## Stats (medidos 2026-09-04)

**Fonte canônica: framework versionado `.opencode/cosca/`** (o runtime pode
reportar um superset — embutido + fallback + registrados).

| Métrica | Valor |
|---|---|
| Agentes (framework) | **53** |
| Skills (framework) | **29** |
| Workflows (framework) | **30** |
| Go version | **1.26.7** |
| Cosca versão | **1.5.0** |
| Knowledge (fonte da verdade) | `knowledge.db` (índice) |
| Provider LLM | Ollama (`cosca-qwen3-4b-lora-001:latest`) |
| Serve | Windows (14120 REST / 14121 metrics / 14122 gRPC) · WSL2 (systemd) |
| Deps segurança | sem HIGH/CRITICAL |

---

## Binary Protection (Auto-Jail)

O binário é protegido: sem permissão de execução se não for chamado via a jaula.
No Linux/WSL2, qualquer comando roda dentro de uma jaula (bwrap/memfd_create)
que restringe syscalls (seccomp), monta o workspace e valida com a chain. No
Windows a jaula não existe (sem bwrap), então a execução sem sandbox é opt-in
explícito via `COSCA_ALLOW_NO_ROOT=1`.

---

## License

Licença **proprietária CoscaAI — v1.1.0 (2026-09-04)**. Ver `LICENSE` para os
termos completos e a chave de segurança (fatores F1–F3). Verificação:
```bash
cosca license verify
```

> **Nota:** O **código/núcleo** e o **conhecimento/dados** são regimes distintos
> sob a licença — rodar o binário não confere direito sobre o conteúdo do
> conhecimento, e o repo público direciona a distribuição apenas conforme os
> termos da licença proprietária (não é OSS).

---

> **Lei da Família:** Honestidade > Lealdade > Confiança; **Memória > Velocidade.**
> O Cosca não cresce por antecipação — cresce porque existe **necessidade
> demonstrada**. A Fatia 3 (mapeamento semântico módulo→domínio) só nasce quando
> houver conteúdo real de mundo com volume/fronteira. Até lá, o que existe está
> **provado e protegido**.
