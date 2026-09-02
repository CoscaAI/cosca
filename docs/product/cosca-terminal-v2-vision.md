# COSCA TERMINAL v2 — Mission Control (Documento de Visão)

> **Status**: PROPOSTA — visão para aprovação do Don
> **Data**: 2026-09-02
> **Origem**: Don + professor (visão estratégica)
> **Natureza**: Redesign do `cosca terminal` de "UI do agente" para "painel de controle do COSCA"

---

## 1. PRINCÍPIO (o que NÃO é)

**Não é um clone do OpenCode.**

O OpenCode é excelente como *coding agent* — TUI, sessões, MCP, permissões.
O COSCA Terminal v2 é um **operating environment para agentes**: a superfície
do terminal expõe o que o COSCA já construiu por baixo:

```
KERNEL → AGENTES → CAPABILITIES → MEMÓRIA → KNOWLEDGE → EXECUÇÃO → PROVA
```

## 2. ARQUITETURA CONCEITUAL

```
                    COSCA TERMINAL
                         │
          ┌──────────────┼──────────────┐
          ↓              ↓              ↓
       AGENTS          TOOLS         COMPUTER
          │              │              │
     Don/Kernel      capabilities    terminal
     Chiefs          MCP             filesystem
     Specialists     plugins         browser
     parallel        skills          desktop
          │              │              │
          └──────────────┼──────────────┘
                         ↓
                    COSCA RUNTIME
                         │
          ┌──────────────┼──────────────┐
          ↓              ↓              ↓
       MEMORY         KNOWLEDGE       GRAPH
          │              │              │
          └──────────────┼──────────────┘
                         ↓
                    PROJECT STATE
```

## 3. BASELINE (padrão esperado — o que o OpenCode já tem)

| Recurso | Descrição |
|---------|-----------|
| TUI rica | Mouse + teclado, não TUI limitada |
| Sessões | fork/continue, resume, histórico |
| Command Palette | fuzzy search em todos os comandos |
| Temas | escuro premium, trocável em runtime |
| Referências | `@arquivo`, `@agente` no input |
| Undo/Redo | edições reversíveis |
| Serve/Attach | runtime separado da UI (remoto) |
| Editor externo | abrir arquivo no editor do usuário |

## 4. DIFERENCIAIS COSCA (o que o OpenCode não entrega)

### 4.1 COMPUTER MODE 🔥
Agente opera o computador por **capability + permission** — não bash onipotente:

```
COMPUTER
├── terminal
├── filesystem
├── browser
├── clipboard
├── processes
├── windows
├── screenshots
├── keyboard
├── mouse
├── network
└── desktop
```

Com painel de permissões visível:
```
╭─ COMPUTER ─────────────────────────────╮
│ Browser       ✓                        │
│ Terminal      ✓                        │
│ Filesystem    ✓ project                │
│ Desktop       ⚠ approval               │
│ Network       ⚠ approval               │
│ Secrets       ✕                         │
╰────────────────────────────────────────╯
```

### 4.2 Agent Orchestration visível
Em vez de `User → Agent`, mostrar a cadeia real:

```
DON
 ├── KERNEL
 │    ├── CTO
 │    │    ├── Go
 │    │    └── PostgreSQL
 │    ├── Security
 │    └── Verification
 └── MEMORY
```

Task com progresso por agente:
```
TASK #1842 — EXECUTING
Planning       ✓
Research       ✓
Implementation ███████░░
Tests          ░░░░░░░░░
Agents: CTO ACTIVE · Go Specialist ACTIVE · Security WAITING · QA WAITING
```

### 4.3 Memory Explorer 🧠
A carta que o OpenCode não reproduz facilmente. Painel com epistemologia:

```
MEMORY
FACT       ├── PostgreSQL usa River
DECISION   ├── ADR-019
INFERENCE  └── Worker é bottleneck?
EVIDENCE   ├── benchmark #184
CONFLICTS  └── 1
```

Clicar numa memória → **"WHY DOES COSCA BELIEVE THIS?"**:
```
FACT → source → evidence → decision → current state
```

### 4.4 Execution Timeline 🔬
Cada task vira timeline; clique em evento → detalhe:

```
00:00 TASK_START   00:12 SEARCH   00:33 RECOVERY
00:02 PLAN         00:18 EDIT     00:41 EDIT
00:07 AGENT_SPAWN  00:22 TEST     00:53 VERIFY
00:09 READ         00:31 FAILED   00:56 COMMIT
```

### 4.5 Permission Center 🛡️
OpenCode tem allow/ask/deny por ferramenta. COSCA leva a **política por agente**:

| Recurso | Regra |
|---------|-------|
| filesystem project/* | ALLOW |
| shell `go test` | ALLOW |
| shell `rm` | ASK |
| deployment staging | ALLOW |
| deployment production | APPROVAL |
| secrets read | DENY |

E por agente: CTO (fs:allow, net:ask, prod:deny) · Security (fs:read-only, net:deny).

### 4.6 Remote COSCA 🌐
Runtime e UI já são conceitos separados → serve/attach natural.

### 4.7 Workspace (Ctrl+1..9)
`Ctrl+1 Chat · 2 Files · 3 Agents · 4 Terminal · 5 Tasks · 6 Memory · 7 Git · 8 Deploy · 9 Graph`

### 4.8 Command Palette monstruosa (Ctrl+P)
`deploy staging` · `spawn security agent` · `inspect memory` · `open graph` ·
`run tests` · `git diff` · `explain failure` · `compact context` · `fork session` ·
`attach runtime` · `change policy` · `export session` · tudo fuzzy.

### 4.9 Context Inspector (Ctrl+I) — KILLER FEATURE
Quando o agente responde, ver **o que entrou no contexto e por quê**:

```
CONTEXT
Direct: task.go · orchestrator.go
Memory: 7 relevant
Knowledge: 12 chunks
Graph: 19 related nodes
Git: 4 changed files
Context: 18.4k / 128k
```

### 4.10 Agent Replay 💥
Depois da task, reproduzir a execução passo a passo (para desenvolver agentes).

### 4.11 Verification Mode 🧪
Agente não diz "pronto" — mostra prova:
```
Build ✓ · Unit ✓ · Integration ✓ · Git Diff ✓ · Static Analysis ✓ · Policy ✓
RESULT: VERIFIED
```

### 4.12 Graph Mode 🕸️ (Ctrl+G)
2D rápido de Task→Agents→Files→Memory→Decision (3D opcional).

### 4.13 COSCA Mission Control 🏆
```
KERNEL ● HEALTHY · 7 AGENTS
TASKS 12 RUNNING · 43 COMPLETED
TOOLS 37 ACTIVE · 2 WAITING
MEMORY 55,850 ENTITIES · 50,084 RELATIONSHIPS
KNOWLEDGE 62,627 CHUNKS · 14,006 VECTORS
VERIFICATION 98.7%
ACTIVE MISSION: RIZOMAI deployment ████ 82%
```

---

## 5. ESTRATÉGIA DE EXECUÇÃO

**Incremental, como camada de apresentação sobre o runtime existente.**

| Fase | Escopo | Entregável |
|------|--------|-----------|
| **1** | Polimento P1-P6 (tema dark premium, status bar, syntax highlight, painel diff real, spinners) | Terminal bonito no padrão OpenCode |
| **2** | Layout 3 colunas + Command Palette + Workspace (Ctrl+1..9) | Base navegável |
| **3** | Agent Orchestration + Task com progresso | Vantagem COSCA visível |
| **4** | Memory Explorer + Context Inspector | Interface do cérebro |
| **5** | Permission Center + Computer Mode | Capabilities visíveis |
| **6** | Mission Control + Graph Mode + Verification | O cockpit completo |

---

## 6. ESTADO ATUAL DO CÓDIGO (baseline verificado — CLI Chief)

- Base charmbracelet sólida: **8.113 linhas** de UI em `internal/chat/ui/terminal/`
- Temas: Petrol (default), TokyoNight, OpenCode já existem em `theme.go`
- `DiffPanelView` **não é renderizado** no `View()` (código morto — Ctrl+D carrega mas não mostra)
- Glamour v1.0.0: syntax highlight exige registrar estilo chroma **por tema** (não `.Chroma` global)
- Testes fixam o tema Petrol (`#092F33`) — não alterar; adicionar novo tema default
- `go build/vet` ✅ limpos · testes UI ✅ passam

---

*Documento de visão — aguarda aprovação do Don para iniciar a Fase 1.*
