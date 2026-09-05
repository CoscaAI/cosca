# Cosca Chat — Next-Gen CLI Architecture

> **Status**: Design | **Author**: Cosca Kernel | **Date**: 2026-07-30
> **Don's Order**: "Criar o nosso autêntico de última geração" + "Multi core, bem sincronizado, analisar capacidade da máquina e agir de acordo sem gargalar"
> **Source Analysis**: OpenCode/Cosca CLI, Claude Code CLI, Codex CLI
> **Related**: [Compute Fabric Design](./compute-fabric-design.md)

---

## 1. Executive Synthesis

Analisamos 3 CLIs de coding agent e extraímos o DNA de cada um:

| Fonte | O que roubamos |
|-------|---------------|
| **Claude Code** | Agent loop (gather→act→verify), subagent spawning dinâmico, permissões com classificador AI, checkpoint/revert, auto-compaction, multi-superfície |
| **Codex CLI** | Sandbox OS-enforced (não only advisory), Starlark execpolicy, auto-review guardian, AGENTS.md inheritance, progressive disclosure |
| **OpenCode/Cosca** | Hexagonal ports & adapters, pipeline context imutável, MAG memory, agent hierarchy (Kernel→CEO→CTO→Chiefs), self-hosting Markdown framework, AES-256 config crypto |

**O que NÃO copiamos:**
- Complexidade excessiva do Codex (100+ keys TOML, 11K issues)
- Vendor lock-in do Claude Code (só Anthropic)
- Tool system hardcoded do OpenCode atual
- Dual config Cobra/Viper vs Config struct

---

## 2. Visão

Um CLI de coding agent que é:
- **Autêntico** — identidade Cosca, não clone
- **Autocontido** — binário único, sem dependências externas no runtime
- **Seguro por padrão** — sandbox OS-enforced, nunca advisory
- **Extensível** — MCP-first, plugin WASM, skills Markdown
- **Simples de usar** — 3 comandos principais, zero configuração obrigatória
- **Multi-modelo** — qualquer provider (OpenAI, Anthropic, DeepSeek, Ollama local)
- **Multi-agente** — subagents dinâmicos spawnados pelo LLM, com hierarquia Cosca

---

## 3. Arquitetura

```
┌─────────────────────────────────────────────────────────────┐
│                     USER INTERFACE                           │
│  ┌─────────┐  ┌─────────┐  ┌──────────┐  ┌─────────────┐  │
│  │  chat   │  │  exec   │  │  serve   │  │  review     │  │
│  │  (TUI)  │  │(CI/CD)  │  │ (API)    │  │ (code rev)  │  │
│  └────┬────┘  └────┬────┘  └────┬─────┘  └──────┬──────┘  │
│       └────────────┴────────────┴─────────────────┘         │
│                        │  CLI Layer (Cobra)                  │
├────────────────────────┼────────────────────────────────────┤
│                        ▼                                     │
│  ┌──────────────────────────────────────────────────────┐   │
│  │              AGENT ENGINE (Core Loop)                 │   │
│  │                                                       │   │
│  │  ┌──────────┐  ┌───────────┐  ┌─────────────┐       │   │
│  │  │ Context  │→ │  Router   │→ │  Provider   │       │   │
│  │  │ Builder  │  │ (agent+)  │  │  Call (LLM) │       │   │
│  │  └──────────┘  └───────────┘  └──────┬──────┘       │   │
│  │                           ┌──────────┘              │   │
│  │                           ▼                          │   │
│  │  ┌──────────┐  ┌───────────┐  ┌─────────────┐      │   │
│  │  │ Response │← │   Tool    │← │  Sandbox    │      │   │
│  │  │ Parser   │  │ Executor  │  │  Gate       │      │   │
│  │  └──────────┘  └───────────┘  └─────────────┘      │   │
│  └──────────────────────────────────────────────────────┘   │
│            │              │              │                  │
│      ┌─────┴──┐    ┌──────┴────┐   ┌────┴─────┐           │
│      │ Memory │    │ Knowledge │   │  Agent   │           │
│      │ Engine │    │  Engine   │   │ Registry │           │
│      └────────┘    └───────────┘   └──────────┘           │
├─────────────────────────────────────────────────────────────┤
│                   PROVIDER LAYER                             │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌───────────┐  │
│  │ OpenAI   │  │Anthropic │  │ DeepSeek │  │ Ollama    │  │
│  │ (native) │  │ (native) │  │ (native) │  │ (local)   │  │
│  └──────────┘  └──────────┘  └──────────┘  └───────────┘  │
│         ↕ Registry pattern (primary + fallback)             │
├─────────────────────────────────────────────────────────────┤
│                   SECURITY LAYER                             │
│  ┌──────────────┐  ┌──────────────┐  ┌───────────────┐     │
│  │ Sandbox      │  │ Permission   │  │ Crypto        │     │
│  │ bwrap+seccomp│  │ Classifier   │  │ AES-256-GCM   │     │
│  │ (OS-enforced)│  │ (AI-powered) │  │ (config+keys) │     │
│  └──────────────┘  └──────────────┘  └───────────────┘     │
├─────────────────────────────────────────────────────────────┤
│                   COMPUTE FABRIC (Multi-Core)                │
│  ┌──────────────┐  ┌──────────────┐  ┌───────────────┐     │
│  │ Hardware     │  │ Worker Pools │  │ Backpressure  │     │
│  │ Probe        │  │ (CPU/IO)     │  │ (breaker,     │     │
│  │ (CPU+RAM+GPU)│  │ Work Stealing│  │  limiter,     │     │
│  │              │  │              │  │  memory budget)│    │
│  └──────────────┘  └──────────────┘  └───────────────┘     │
├─────────────────────────────────────────────────────────────┤
│                   EXTENSIBILITY                              │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌───────────┐  │
│  │ MCP      │  │ Plugin   │  │ Skills   │  │ Hooks     │  │
│  │ Servers  │  │ WASM     │  │ Markdown │  │ Lifecycle │  │
│  └──────────┘  └──────────┘  └──────────┘  └───────────┘  │
├─────────────────────────────────────────────────────────────┤
│                   INFRASTRUCTURE                             │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌───────────┐  │
│  │ SQLite   │  │ Vector   │  │ REST     │  │ gRPC      │  │
│  │ FTS5+vec │  │ Store    │  │ API      │  │ API       │  │
│  └──────────┘  └──────────┘  └──────────┘  └───────────┘  │
└─────────────────────────────────────────────────────────────┘
```

---

## 4. Comandos

```
cosca chat              # TUI interativo (modo principal)
cosca chat "prompt"     # One-shot inline (sem TUI)
cosca chat --model deepseek  # Seleciona modelo
cosca chat --resume     # Retoma última sessão
cosca chat --worktree feature-x  # Git worktree isolado

cosca exec "prompt"     # Non-interactive (CI/CD, scripts)
cosca exec --json       # Saída JSONL para automação
cosca exec --schema schema.json  # Structured output
cosca exec --max-turns 10  # Limite de turnos
cosca exec --ephemeral  # Não persiste sessão

cosca review            # Code review do diff atual
cosca review --uncommitted  # Apenas não commitado
cosca review --base main    # Diff contra branch

cosca serve             # Inicia API server (REST + gRPC + MCP)
cosca serve --port 8080
cosca serve --mcp       # Modo MCP server (stdio)

cosca config            # Gerencia configuração
cosca config set providers.deepseek.api_key <key>
cosca config get
cosca config list

cosca doctor            # Diagnóstico do ambiente
cosca init              # Inicializa projeto com .cosca/
cosca plugin            # Gerencia plugins WASM
cosca mcp               # Gerencia servidores MCP
```

**3 comandos principais, zero config obrigatória:**
1. `cosca chat` — começa a usar imediatamente
2. `cosca exec` — automação
3. `cosca serve` — servidor

---

## 5. O Agent Loop (Coração)

```
┌──────────────────────────────────────────────────┐
│              AGENT LOOP (por turno)               │
│                                                   │
│  1. CONTEXT BUILD                                 │
│     ├─ Carrega .cosca/config.yaml                 │
│     ├─ Carrega AGENTS.md (cadeia de herança)      │
│     ├─ MAG: busca memória relevante               │
│     ├─ Knowledge: busca documentação relevante    │
│     ├─ Skills: expõe nomes (progressive)          │
│     └─ Tools: registra disponíveis (MCP + builtin)│
│                                                   │
│  2. ROUTE                                         │
│     ├─ Keyword match → Agent Registry             │
│     ├─ Semantic search (embeddings)               │
│     └─ Fallback: general agent                    │
│                                                   │
│  3. LLM CALL                                      │
│     ├─ System: identidade + contexto + tools      │
│     ├─ Messages: histórico + user prompt          │
│     ├─ Streaming: tokens em tempo real            │
│     └─ Parse: content + tool_calls                │
│                                                   │
│  4. TOOL EXECUTION                                │
│     ├─ Se tool_call → Sandbox Gate                │
│     │   ├─ Read ops → auto (within workspace)     │
│     │   ├─ Write ops → classifier check           │
│     │   └─ Network ops → approval required        │
│     ├─ Se subagent spawn → novo context isolado   │
│     ├─ Executa tool no sandbox                    │
│     └─ Feedback resultado ao LLM                  │
│                                                   │
│  5. VERIFY & LOOP                                 │
│     ├─ Se mais tool_calls → volta ao passo 4     │
│     ├─ Se resposta final → apresenta ao usuário   │
│     └─ Auto-compaction se contexto > 80%          │
│                                                   │
│  6. MEMORY STORE (post-turn)                      │
│     ├─ MAG: armazena decisão                      │
│     ├─ Learnings: extrai padrão (se novo)         │
│     └─ Session: salva transcript                  │
└──────────────────────────────────────────────────┘
```

---

## 6. Sistema de Tools (MCP-First)

Todas as tools são registradas via MCP. Tools built-in são implementadas como MCP servers internos.

```
Tool Registry
├── builtin/ (MCP servers internos)
│   ├── filesystem (read, write, edit, glob)
│   ├── shell (bash, com sandbox)
│   ├── search (grep, semantic)
│   ├── web (fetch, search)
│   ├── agent (spawn subagent)
│   ├── git (commit, diff, branch, PR)
│   └── memory (store, retrieve, forget)
├── mcp/ (servidores externos)
│   ├── databases (PostgreSQL, MySQL, etc.)
│   ├── APIs (GitHub, Jira, Slack, etc.)
│   └── custom (qualquer MCP server)
└── plugin/ (plugins WASM)
    └── marketplace
```

**Princípio**: Toda tool é MCP. Isso significa:
- Descoberta dinâmica (não hardcoded)
- Schema padronizado (JSON Schema)
- Sandbox uniforme (toda tool passa pelo mesmo gate)
- Hot-reload (adiciona/remove MCP servers sem reiniciar)

---

## 7. Sandbox & Segurança

### Camadas (inspirado no Codex + aprimorado)

| Camada | Mecanismo | O que protege |
|--------|-----------|---------------|
| **1. OS Sandbox** | bwrap + seccomp (Linux), Seatbelt (macOS) | Acesso a arquivos, rede, processos fora do workspace |
| **2. Permission Gate** | Classificador AI-powered (como Claude Code) | Revisa ações antes de executar |
| **3. Workspace Rails** | Path validation + symlink prevention | Impede escape do workspace |
| **4. Crypto** | AES-256-GCM para config/keys | Protege credenciais em disco |
| **5. Auto-Jail** | memfd_create + reexec | Binário se auto-enjaula |

### Modos de Sandbox

| Modo | Arquivos | Rede | Comandos |
|------|----------|------|----------|
| `read-only` | Read workspace | Off | Sem shell |
| `workspace` | Read/Write workspace (exceto .git/) | Off (opt-in) | Shell dentro do workspace |
| `full` | Irrestrito | On | Shell irrestrito |

**Default: `workspace`**. O usuário não precisa configurar nada — seguro por padrão.

---

## 8. Sistema de Agentes (Hierarquia Cosca)

Mantemos e evoluímos o sistema de agentes Markdown-native:

```
Kernel (cosca-kernel)
├── CEO (cosca-ceo) — estratégia
├── CTO (cosca-cto) — técnica
├── Chiefs (15+ agentes)
│   ├── Architecture, Backend, Frontend, Database
│   ├── Security, DevOps, Testing, QA
│   └── AI, Analytics, Documentation, etc.
└── Specialists (20+ agentes)
    ├── API, Service, SQL, Component, Testing
    └── Review, Documentation, etc.
```

**Novidade**: Subagent spawning dinâmico. O LLM pode criar um subagente com:
```
<tool name="agent">
  <agent>cosca-database</agent>
  <task>Otimizar esta query</task>
  <context>schema.sql + query atual</context>
</tool>
```

O subagente roda em contexto isolado (como Claude Code) e retorna apenas o summary.

---

## 9. Contexto & Memória

### Context Pipeline (ordem de carga)

1. **System Identity** — Quem sou (agent DNA)
2. **Project Context** — AGENTS.md (cadeia de herança como Codex)
3. **MAG Memory** — Memórias relevantes (nosso padrão)
4. **Knowledge** — Documentação relevante (FTS5 + vector)
5. **Skills** — Progressive disclosure (nomes → conteúdo)
6. **Tools** — Schema das tools disponíveis
7. **Conversation History** — Últimos N turnos
8. **User Request** — O prompt atual

### Auto-Compaction (como Claude Code)

Quando o contexto atinge 80% da janela:
1. Tool outputs antigos são limpos primeiro
2. Conversa é resumida (LLM call de sumarização)
3. Se thrashing (recompacta imediatamente após compactar), para com erro

### Persistent Sessions

Sessões salvas em `.cosca/sessions/` como JSONL:
- Resume: `cosca chat --resume`
- Fork: mid-session branch
- Search: `cosca chat --search-sessions "bug fix authentication"`

---

## 10. Provider Architecture

```
ChatRegistry (interface)
├── primary: configurado pelo usuário
├── fallback: automático se primary falhar
├── hot-reload: detecta mudanças de config
└── models:
    ├── openai: gpt-5, gpt-4o
    ├── anthropic: claude-sonnet-5, claude-opus-4
    ├── deepseek: deepseek-v4
    ├── ollama: qualquer modelo local
    └── custom: qualquer OpenAI-compatible endpoint
```

**Model switching mid-session** (como Claude Code):
```
/model deepseek
/model claude-sonnet-5
/model ollama/codellama
```

---

## 11. Extensibilidade

| Mecanismo | O que faz | Exemplo |
|-----------|-----------|---------|
| **MCP Servers** | Adiciona tools externas | PostgreSQL, GitHub API |
| **Plugins WASM** | Extensões sandboxed | Linters customizados, formatadores |
| **Skills Markdown** | Instruções reutilizáveis | "deploy-to-aws", "create-migration" |
| **Hooks** | Intercepta eventos do ciclo de vida | PreToolUse, PostToolUse, SessionStart |
| **Custom Agents** | Agentes especializados | `.cosca/agents/` |

---

## 12. Stack Tecnológica

| Camada | Tecnologia | Por quê |
|--------|-----------|---------|
| **Linguagem** | Go 1.23+ | Performance, binário único, nosso ecossistema |
| **CLI Framework** | Cobra | Padrão Cosca, maduro |
| **TUI** | Bubble Tea (charmbracelet) | Melhor TUI framework Go |
| **LLM Clients** | Go OpenAI, Go Anthropic | Nativos, sem wrapper |
| **Sandbox** | bwrap + seccomp | OS-enforced, zero-trust |
| **MCP** | mcp-go | Implementação Go do protocolo |
| **Database** | SQLite (mattn/go-sqlite3) | FTS5, WAL mode, zero-config |
| **Vector** | SQLite + custom extension | Sem dependência externa |
| **Config** | YAML (gopkg.in/yaml.v3) | Simples, legível |
| **Crypto** | crypto/aes + crypto/cipher | Standard library, sem dependência |
| **Plugin** | Wazero (WASM runtime) | Zero dependencies, puro Go |
| **Logging** | Zerolog | Performance, estruturado |
| **API** | Chi router (REST) + gRPC | Leve, idiomático |
| **Compute** | errgroup + semaphore + channels (stdlib) | Concorrência formal, zero dependência |
| **Scheduling** | Work stealing + adaptive scaling | Mesmo algoritmo do Go scheduler |

**Princípio**: Zero dependências externas no runtime. Tudo que o binário precisa está dentro dele. SQLite é embedded. WASM é embedded. bwrap é a única dependência de sistema (e é opcional — fallback para modo sem sandbox).

---

## 13. DNA Único — O Que Nos Torna "Autênticos"

| Característica | Por que é único |
|----------------|-----------------|
| **Agent Hierarchy** | Nenhum outro CLI tem Kernel→CEO→CTO→Chiefs como sistema de agentes |
| **Self-Hosting** | O CLI se constrói com seus próprios agentes (`internal/embed/cosca/`) |
| **MAG Memory** | Memória semântica cross-agent com scoring de confiança |
| **Auto-Jail** | O binário se copia pra RAM e se executa em sandbox — ninguém mais faz isso |
| **Markdown-Native Agents** | Agentes definidos em Markdown, versionados no Git, sem database |
| **AES-256 Config Crypto** | API keys criptografadas em disco com machine-id binding |
| **Cognitive Maturity** | Auto-evolution, metacognition pipeline, cognitive engines — o CLI aprende |
| **Compute Fabric** | Multi-core com work stealing, scaling adaptativo, backpressure — sente a máquina e age |
| **Simplicidade Radical** | 3 comandos principais. Zero config. `cosca chat` e já está usando. |

---

## 14. Métricas de Sucesso

| Métrica | Target |
|---------|--------|
| Tempo até primeiro prompt útil | < 30 segundos (instalação + `cosca chat "hello"`) |
| Cobertura de testes | 100% statement coverage |
| Binary size | < 40 MB |
| Memory footprint idle | < 50 MB |
| Subagent spawn latency | < 500ms |
| MCP tool discovery | < 100ms |
| Cold start (first prompt) | < 2 segundos |
| Sandbox enforcement | 100% (zero escapes em audit) |

---

## 15. Comparação Final

| Dimensão | Claude Code | Codex CLI | OpenCode Atual | **Cosca Chat (nosso)** |
|----------|-------------|-----------|----------------|------------------------|
| **Open Source** | ❌ | ✅ | ✅ | ✅ |
| **Multi-Model** | ❌ (só Anthropic) | ⚠️ (OSS mode limitado) | ✅ | ✅ |
| **Sandbox OS** | ⚠️ (parcial) | ✅ (Seatbelt+bwrap) | ✅ (bwrap) | ✅ (bwrap+seccomp) |
| **Agent Hierarchy** | ❌ | ❌ | ✅ | ✅ (aprimorado) |
| **Subagent Spawn** | ✅ | ✅ | ❌ | ✅ |
| **MCP-First** | ✅ | ✅ | ❌ | ✅ |
| **Auto-Compaction** | ✅ | ✅ | ❌ | ✅ |
| **Persistent Sessions** | ✅ | ✅ | ❌ | ✅ |
| **Config Crypto** | ❌ | ❌ | ✅ | ✅ |
| **Self-Hosting** | ❌ | ❌ | ✅ | ✅ |
| **Cognitive Evolution** | ❌ | ❌ | ✅ | ✅ |
| **Compute Fabric** | ❌ | ❌ | ❌ | ✅ (único!) |
| **Simplicidade** | ⚠️ (muita config) | ❌ (100+ keys) | ⚠️ | ✅ (3 comandos) |
