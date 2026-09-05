# @cosca/cli — Cosca Runtime CLI

Global CLI para o **Cosca (AI Orchestration System)** — usado por AI assistants como OpenCode, Claude Code, Codex, Cursor e outros para orquestrar desenvolvimento.

## Instalação Global

```bash
npm install -g @cosca/cli

# Verificar instalação
cosca health

# Inicializar Cosca no projeto atual
cosca init
```

## Uso com AI Assistants

### OpenCode / Claude Code / Codex

Quando iniciar o AI assistant, execute:

```bash
# Inicializar contexto do projeto
cosca init

# Descobrir stack tecnológica
cosca discover --json

# Ver workflows disponíveis
cosca workflow list --json

# Pegar um workflow específico  
cosca workflow get security-audit --json

# Executar quality gate
cosca gate 2.2 --json
```

Use `--json` para output estruturado que o AI pode processar.

## Comandos

| Comando | Descrição | Exemplo |
|---------|-----------|---------|
| `cosca init` | Inicializa Cosca no projeto | `cosca init ./my-project` |
| `cosca discover` | Escaneia stack tecnológica | `cosca discover --json` |
| `cosca workflow list` | Lista workflows | `cosca workflow list security` |
| `cosca workflow get <name>` | Detalha workflow | `cosca workflow get security-audit` |
| `cosca workflow run <name>` | Executa workflow | `cosca workflow run migration` |
| `cosca skill list` | Lista skills | `cosca skill list security` |
| `cosca skill get <name>` | Detalha skill | `cosca skill get SECURITY_AUDIT` |
| `cosca skill search <q>` | Busca skills | `cosca skill search audit` |
| `cosca gate <number>` | Quality gate | `cosca gate 2.2` |
| `cosca agent [name]` | Lista chiefs | `cosca agent` ou `cosca agent backend` |
| `cosca memory [store]` | Acessa memória | `cosca memory decision` |
| `cosca health` | Saúde do framework | `cosca health` |

## Integração com AI Assistants

### OpenCode
Adicione ao `opencode.json` do projeto:
```json
{
  "tools": [
    {
      "name": "cosca",
      "command": "cosca",
      "args": ["--json"]
    }
  ]
}
```

### Claude Code
```bash
# Claude Code pode chamar comandos Cosca diretamente
claude "Execute cosca discover --json e me diga qual framework estamos usando"
```

### Codex / Cursor
```bash
# Via terminal integrado
cosca workflow get code-review --json
```

## JSON Output

Todos os comandos suportam `--json` para output processável:

```json
{
  "name": "security-audit",
  "category": "security",
  "steps": [
    { "chief": "Security Chief", "task": "Audit code for vulnerabilities" }
  ]
}
```

## Desenvolvimento

```bash
git clone <repo>
cd cosca/cli
npm install
npm run build
npm link   # instala globalmente para desenvolvimento
```
