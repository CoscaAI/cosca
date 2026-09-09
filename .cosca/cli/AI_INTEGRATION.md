# Cosca — Integração com AI Assistants

## Como o Cosca funciona com AI coding tools

```
AI Assistant                      Cosca                    Cosca File System
(OpenCode/Claude/Codex)           (global)                   (Markdown)
       │                            │                           │
       │  1. Session Start          │                           │
       │  ─────────────────────▶    │                           │
       │                            │  2. cosca init --json      │
       │                            │  ───────────────────────▶│
       │                            │  3. Project context      │
       │                            │  ◀───────────────────────│
       │  4. Context Response ◀─────│                           │
       │                            │                           │
       │  5. User Request           │                           │
       │  (ex: "audit security")    │                           │
       │                            │  6. cosca workflow get     │
       │                            │     security-audit --json│
       │                            │  ───────────────────────▶│
       │                            │  7. Workflow steps       │
       │                            │  ◀───────────────────────│
       │  8. Execute steps ◀─────── │                           │
       │                            │                           │
       │  9. cosca gate 2.3 --json    │                           │
       │  ─────────────────────────▶│  ───────────────────────▶│
       │  10. Gate results ◀─────── │  ◀───────────────────────│
```

## Fluxo Completo

### 1. No início de cada sessão

O AI assistant deve executar:

```bash
# Inicializar Cosca no projeto (cria .cosca/ com contexto)
cosca init

# Descobrir stack tecnológica
STACK=$(cosca discover --json)

# Carregar contexto do projeto
WORKFLOWS=$(cosca workflow list --json)
SKILLS=$(cosca skill list --json)
HEALTH=$(cosca health --json)
```

### 2. Durante a sessão — Resolver requests

Quando o usuário faz um request, o AI assistant:

```bash
# 1. Classificar o request
# feature? bug? refactor? security?

# 2. Encontrar workflow correspondente
cosca workflow get security-audit --json

# 3. Encontrar skills correspondentes
cosca skill search audit --json

# 4. Encontrar chief responsável
cosca agent security --json
```

### 3. Ao final da sessão — Salvar aprendizado

```bash
# Salvar decisões na memória
cosca memory decision

# Salvar padrões identificados
cosca memory pattern
```

## Formatos de Resposta

### Texto (default) — para humanos
```
📋 Workflow: security-audit
   Category: security
   Scopo: Auditoria completa de segurança

   Steps:
   1. Analisar código contra OWASP Top 10
   2. Escanear dependências por CVEs
   3. Verificar hardcoded secrets
```

### JSON (--json) — para AI assistants
```json
{
  "name": "security-audit",
  "category": "security",
  "description": "Auditoria completa de segurança",
  "steps": [
    {
      "chief": "Security Chief",
      "task": "Analisar código contra OWASP Top 10",
      "output": "Relatório de vulnerabilidades"
    }
  ],
  "preconditions": ["Código compilando", "Acesso ao repositório"],
  "successCriteria": ["0 critical/high CVEs"]
}
```

## Plugins para AI Assistants

### OpenCode
Adicione este tool ao `opencode.json`:

```json
{
  "name": "cosca-orchestrator",
  "description": "Cosca Framework — orquestração de agentes, workflows e quality gates",
  "command": "cosca",
  "args": ["--json"],
  "permissions": ["read", "write"],
  "autoInit": true
}
```

### Claude Code
Use como ferramenta no `claude.md`:

```markdown
## Ferramentas
- `cosca init` — Inicializar contexto Cosca
- `cosca discover` — Descobrir stack do projeto
- `cosca workflow list` — Listar workflows disponíveis
- `cosca workflow get <name>` — Obter workflow específico
- `cosca skill search <query>` — Encontrar skills relevantes
- `cosca gate <number>` — Executar quality gate
```

### Codex / Cursor
```bash
# Script de inicialização para Codex
#!/bin/bash
cosca init && cosca discover --json
```

## Variáveis de Ambiente

| Variável | Descrição | Default |
|----------|-----------|---------|
| `COSCA_HOME` | Caminho para pasta Cosca | Auto-detectado |
| `COSCA_FORMAT` | Formato de output | `text` ou `json` |
| `COSCA_PROJECT` | Projeto atual | `cwd` |

## Exemplo de Uso em Sessão

```markdown
# Início da sessão
$ cosca init
✅ Cosca initialized in /projetos/api

$ cosca discover --json
{
  "framework": "nestjs",
  "language": "typescript",
  "database": "postgresql"
}

# Usuário: "Faz um code review no PR #42"
$ cosca workflow get code-review --json
{
  "steps": [
    {"chief": "Review Chief", "task": "Review architecture"},
    {"chief": "Security Chief", "task": "Review security"},
    ...
  ]
}

# AI assistant segue os steps do workflow
# Ao final:
$ cosca gate 2.2 --json
{
  "gate": "2.2",
  "score": 8.5,
  "checks": [...]
}
```
