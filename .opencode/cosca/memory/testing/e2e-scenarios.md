# E2E Test Scenarios — Cosca v1.4.0-dev

> **Document Type**: Test Planning (E2E)  
> **Version**: 1.0.0  
> **Status**: draft  
> **Owner**: cosca-specialist-testing-e2e  
> **Created**: 2026-07-28  
> **Related Risk**: [R10 — Sem E2E tests](../risk/RISK_REGISTRY.md#R10)  
> **Cobertura atual**: ~78% unit + integration, 0% E2E de jornada completa  

---

## Visão Geral

Este documento define 5 cenários de teste End-to-End que cobrem as jornadas críticas de usuário da plataforma Cosca. Cada cenário é uma jornada completa — do início ao fim — validando a integração real entre CLI, REST API, Knowledge Engine, Memory System, Orchestration Engine, Quality Gates e CI/CD pipeline.

### Mapa de Cobertura

| Cenário | Domínios Cobertos | Portas/APIs | CLI Commands | Tipo |
|---------|-------------------|-------------|--------------|------|
| C1 — Criar Projeto | init, config, provider, orchestration | N/A (CLI) | init, provider list/set/test, run | CLI E2E |
| C2 — Knowledge Sync | knowledge, search, memory | /v1/knowledge/* | sync, knowledge search, memory search | API + CLI |
| C3 — Agent Task | orchestration, agents, memory, execution | /v1/run, /v1/executions, /v1/knowledge/search | run, agent list | API E2E |
| C4 — Review Cycle | quality gates, review, agents | N/A (CI pipeline) | N/A (CI) | CI E2E |
| C5 — Deploy | CI/CD, docker, release | /health, /ready, /v1/status | version, status, health | Deploy E2E |

---

## Cenário 1 — Criar Projeto (CLI E2E)

### Jornada

**Usuário novo chega** → **inicializa Cosca** → **configura provider LLM** → **executa primeira task** → **verifica resultado**

### Pré-condições

| Condição | Como Garantir |
|----------|---------------|
| Binário `cosca` compilado e no PATH | `make build && export PATH=$PWD/bin:$PATH` |
| Diretório de projeto limpo (vazio ou sem `.cosca/`) | `t.TempDir()` ou `mktemp -d` |
| Nenhum processo `cosca serve` rodando na porta 14120 | Verificar `lsof -i :14120` |
| Provider configurável (não requer API key real — aceita mock/dry-run) | Usar `--dry-run` ou provider local como Ollama |
| Permissão de escrita no diretório de projeto | Garantido por `t.TempDir()` |

### Passos

#### Passo 1: Init

**Comando**:
```bash
cosca init --json
```

**Entrada**: Nenhuma (auto-detecta diretório corrente)

**Resultado Esperado**:
```json
{
  "project_dir": "/tmp/test-project",
  "cosca_dir": "/tmp/test-project/.cosca",
  "config_file": "/tmp/test-project/.cosca/config/config.yaml",
  "found_env": {
    "framework": "go" | "node" | "unknown",
    "language": "go" | "typescript" | "unknown",
    "runtime": "go1.25" | "node20"
  },
  "next_steps": ["cosca provider list", "cosca run <prompt>"],
  "directories": [
    "/tmp/test-project/.cosca",
    "/tmp/test-project/.cosca/config",
    "/tmp/test-project/.cosca/memory",
    "/tmp/test-project/.cosca/memory/short",
    "/tmp/test-project/.cosca/memory/long",
    "/tmp/test-project/.cosca/memory/project",
    "/tmp/test-project/.cosca/memory/architecture",
    "/tmp/test-project/.cosca/memory/decision",
    "/tmp/test-project/.cosca/cache",
    "/tmp/test-project/.cosca/plugins",
    "/tmp/test-project/.cosca/runtime",
    "/tmp/test-project/.cosca/index",
    "/tmp/test-project/.cosca/vectors",
    "/tmp/test-project/.cosca/graph",
    "/tmp/test-project/.cosca/sessions",
    "/tmp/test-project/.cosca/audit"
  ]
}
```

**Critérios de Sucesso**:
- Exit code = 0
- Todos os 16 diretórios criados em `.cosca/`
- `config.yaml` existe e é parseable
- Detection do ambiente funciona (framework + language + runtime != "unknown")

**Condições de Falha**:
- Exit code ≠ 0
- `.cosca/` não existe ou está incompleto
- `config.yaml` vazio ou inválido
- Erro "already initialized" sem `--force` em dir limpo (falso positivo)

#### Passo 2: Provider List

**Comando**:
```bash
cosca provider list --json
```

**Entrada**: Nenhuma

**Resultado Esperado**:
```json
[
  {"name": "openai", "status": "available", "model": "gpt-4o"},
  {"name": "anthropic", "status": "available", "model": "claude-3-opus"},
  {"name": "ollama", "status": "available", "model": "llama3"},
  ...
]
```

**Critérios de Sucesso**:
- Exit code = 0
- Lista contém ≥ 1 provider com `status: "available"`
- JSON válido e parseable

**Condições de Falha**:
- Exit code ≠ 0
- Lista vazia (nenhum provider registrado)
- JSON inválido

#### Passo 3: Provider Set + Test

**Comando**:
```bash
cosca provider set ollama --json
cosca provider test ollama --json
```

**Entrada**: Nome do provider (ex: `ollama`)

**Resultado Esperado (set)**:
```json
{"status": "ok", "provider": "ollama"}
```

**Resultado Esperado (test)**:
```json
{
  "provider": "ollama",
  "status": "ok",
  "latency_ms": 150,
  "model": "llama3"
}
```

**Critérios de Sucesso**:
- Exit code = 0 em ambos
- Provider ativo confirmado
- Test retorna `status: "ok"` com latência mensurável

**Condições de Falha**:
- Provider não encontrado → mensagem de erro clara
- Test falha (provider offline) → `status: "error"` ou timeout
- Provider não persiste entre chamadas → verificar `cosca config get`

#### Passo 4: Primeira Task (run --dry-run)

**Comando**:
```bash
cosca run --dry-run --json "Explain what Cosca is in one sentence"
```

**Entrada**: Prompt "Explain what Cosca is in one sentence"

**Resultado Esperado**:
```json
{
  "request_id": "run-<uuid>",
  "agent": "cosca-ai" | "cosca-backend" | "auto",
  "prompt": "Explain what Cosca is in one sentence",
  "dry_run": true,
  "status": "ready"
}
```

**Critérios de Sucesso**:
- Exit code = 0
- Agente selecionado automaticamente (pelo menos 1 candidato)
- Nenhum erro de provider (dry-run não chama LLM)

**Condições de Falha**:
- "failed to select chat provider" → provider não configurado
- "execution failed" → erro no orchestration engine
- Timeout (>30s) → engine travado

#### Passo 4b: Task Real (opcional, requer provider real)

**Comando**:
```bash
cosca run --json "Hello, Cosca. Respond with 'OK' to confirm you are working."
```

**Entrada**: Prompt simples de health-check

**Resultado Esperado**:
```json
{
  "id": "exec-<uuid>",
  "agent": "cosca-ai",
  "response": "...OK...",
  "duration_ms": "<number>",
  "skills_used": [],
  "memory_id": ""
}
```

**Critérios de Sucesso**:
- Exit code = 0
- `response` contém "OK" (ou variante)
- `duration_ms` > 0
- `id` é UUID válido

**Condições de Falha**:
- Provider retorna erro → mensagem clara com status code
- Timeout → contexto expira, erro capturado
- Resposta vazia → engine retornou sem conteúdo

### Dependências

| Dependência | Impacto da Falha |
|-------------|-----------------|
| Binário `cosca` | Teste nem inicia — build falhou |
| Sistema de arquivos | Init falha ao criar diretórios |
| Provider (real) | Task real falha — usar apenas dry-run como fallback |
| Rede (provider test) | Test de provider falha — aceitar como skip condicional |

### Estimativa de Tempo
- **Init**: 2-3s
- **Provider list/set/test**: 2-5s
- **Task dry-run**: 1-2s
- **Task real (opcional)**: 5-30s (depende do provider)
- **Total**: 10-40s

---

## Cenário 2 — Knowledge Sync (API + CLI E2E)

### Jornada

**Projeto existente** → **sync knowledge base** → **indexar documentos** → **buscar (FTS5 + vector)** → **verificar resultados**

### Pré-condições

| Condição | Como Garantir |
|----------|---------------|
| Servidor Cosca rodando (`cosca serve --port 14120`) | Iniciar como processo filho, verificar `/health` |
| Projeto inicializado com `cosca init` | Executar init antes do teste |
| Arquivos de teste para indexar (≥ 3 markdown files) | Criar em `t.TempDir()` com conteúdo conhecido |
| Usuário autenticado (JWT token) | `POST /v1/auth/login` com credenciais de teste |
| Token JWT armazenado para requests subsequentes | Header `Authorization: Bearer <token>` |

### Passos

#### Passo 0: Setup — Iniciar Servidor

```bash
# Iniciar servidor em background
./bin/cosca serve --port 14120 &
COSCA_PID=$!

# Aguardar health check
for i in $(seq 1 30); do
  if curl -s http://localhost:14120/health | grep -q '"healthy":true'; then
    break
  fi
  sleep 1
done
```

**Resultado Esperado**:
- `/health` retorna `{"healthy": true}` em < 10s
- Processo servidor rodando (PID existe)

#### Passo 0b: Autenticação

```http
POST /v1/auth/login
Content-Type: application/json

{
  "username": "admin",
  "password": "admin123"
}
```

**Resultado Esperado**:
```json
{
  "access_token": "eyJ...",
  "refresh_token": "eyJ...",
  "expires_in": 86400
}
```

#### Passo 1: Criar Arquivos de Teste

```bash
# Criar 3 documentos markdown com conteúdo distinto
cat > /tmp/test-knowledge/doc1.md << 'EOF'
# Cosca Architecture
Cosca uses a mafia-family organizational model with a Don, Kernel, Chiefs, and Specialists.
The Kernel acts as consigliere, routing tasks to the appropriate Chief.
EOF

cat > /tmp/test-knowledge/doc2.md << 'EOF'
# Memory System
Cosca's semantic auto-evolution memory stores learnings, failures, patterns, and capability profiles.
Agents progress through 5 levels: Basic, Intermediate, Advanced, Expert, Master.
EOF

cat > /tmp/test-knowledge/doc3.md << 'EOF'
# Quality Gates
Cosca enforces 10 quality gates (G0-G9) before any deliverable is accepted.
Gate G0 ensures build passes. Gate G4 requires security scan. Gate G9 requires review approval.
EOF
```

#### Passo 2: CLI Sync

```bash
cosca sync --full --json
```

**Resultado Esperado**:
```json
{
  "project_dir": "/tmp/test-knowledge",
  "full_sync": true,
  "files_scanned": 3,
  "files_added": 3,
  "files_updated": 0,
  "files_deleted": 0,
  "duration": "<time>",
  "errors": 0
}
```

**Critérios de Sucesso**:
- Exit code = 0
- `files_scanned` ≥ 3
- `files_added` = 3 (para sync inicial)
- `errors` = 0

#### Passo 3: API Knowledge Index

```http
POST /v1/knowledge/index
Authorization: Bearer <token>
Content-Type: application/json

{
  "path": "/tmp/test-knowledge",
  "recursive": true
}
```

**Resultado Esperado**:
```json
{
  "indexed": 3,
  "chunks": 5,
  "duration_ms": 200,
  "errors": []
}
```

**Critérios de Sucesso**:
- HTTP 200
- `indexed` ≥ 3
- `errors` vazio

#### Passo 4: Busca FTS5

```http
POST /v1/knowledge/search
Authorization: Bearer <token>
Content-Type: application/json

{
  "query": "quality gates security",
  "limit": 5
}
```

**Resultado Esperado**:
```json
{
  "results": [
    {
      "document": "doc3.md",
      "path": "/tmp/test-knowledge/doc3.md",
      "score": 0.85,
      "snippet": "...quality gates (G0-G9)...Gate G4 requires security scan...",
      "match_type": "fts5"
    },
    ...
  ],
  "total": 1,
  "duration_ms": 15
}
```

**Critérios de Sucesso**:
- HTTP 200
- `results` não vazio
- `doc3.md` aparece no topo (contém "quality gates" e "security")
- `score` > 0 (FTS5 relevância)
- `match_type` inclui "fts5"

#### Passo 5: Busca Semântica (Vector)

```http
POST /v1/knowledge/search
Authorization: Bearer <token>
Content-Type: application/json

{
  "query": "how does the system remember things over time",
  "search_type": "hybrid",
  "limit": 5
}
```

**Resultado Esperado**:
```json
{
  "results": [
    {
      "document": "doc2.md",
      "score": 0.72,
      "match_type": "vector",
      "snippet": "...semantic auto-evolution memory stores learnings..."
    },
    ...
  ],
  "total": 1,
  "duration_ms": 80
}
```

**Critérios de Sucesso**:
- HTTP 200
- `doc2.md` (sobre memory system) aparece no topo para query semântica sobre "remember"
- `match_type` = "vector" ou "hybrid" (se vector store disponível)

#### Passo 6: Knowledge Stats

```http
GET /v1/knowledge/stats
Authorization: Bearer <token>
```

**Resultado Esperado**:
```json
{
  "documents": 3,
  "chunks": 5,
  "db_size_bytes": 45000,
  "graph_nodes": 12,
  "graph_edges": 8,
  "uptime_seconds": 120
}
```

**Critérios de Sucesso**:
- HTTP 200
- `documents` = 3 (refletindo dados indexados)
- `db_size_bytes` > 0 (banco populado)

### Condições de Falha por Passo

| Passo | Falha | Detecção |
|-------|-------|----------|
| Serve startup | Porta em uso, DB corrompido | `/health` retorna não-200 ou timeout |
| Auth | Credenciais inválidas, DB sem seed | HTTP 401 |
| Sync | File system read-only, permissão | `errors` > 0 |
| Index | Path não existe, arquivo binário | HTTP 400, `errors` não vazio |
| Search FTS5 | Índice FTS5 vazio, query malformada | `results` vazio |
| Search Vector | Embedding provider offline | `match_type` = "fts5" apenas (fallback) |

### Dependências

| Dependência | Tipo | Impacto |
|-------------|------|---------|
| Processo `cosca serve` | Servidor local | Bloqueante — sem servidor, nada funciona |
| SQLite (modernc.org) | Embedded DB | Bloqueante — FTS5 requer índice |
| Vector store (sqlite-vec) | Embedded DB | Degradado — busca cai para FTS5-only |
| Embedding provider | Serviço externo | Degradado — vector search usa fallback |

### Cleanup

```bash
kill $COSCA_PID 2>/dev/null
rm -rf /tmp/test-knowledge
```

### Estimativa de Tempo
- **Serve startup**: 5-10s
- **Auth**: 500ms
- **Criar arquivos**: 100ms
- **Sync**: 2-5s (depende do tamanho do projeto)
- **Index**: 1-3s
- **FTS5 Search**: 50-200ms
- **Vector Search**: 200ms-2s (depende do embedding provider)
- **Stats**: 50ms
- **Total**: 15-25s

---

## Cenário 3 — Agent Task (API E2E)

### Jornada

**Kernel recebe task** → **seleciona agente automaticamente** → **agente executa com skills** → **retorna resultado** → **verifica learnings registrados**

### Pré-condições

| Condição | Como Garantir |
|----------|---------------|
| Servidor Cosca rodando | Igual C2 — `cosca serve` background |
| Usuário autenticado (JWT) | `POST /v1/auth/login` |
| Provider configurado e testado | `cosca provider set <name>` + `cosca provider test <name>` |
| Knowledge base populada (opcional) | Sync executado previamente |

### Passos

#### Passo 1: Listar Agentes Disponíveis

```http
GET /v1/agents
Authorization: Bearer <token>
```

**Resultado Esperado**:
```json
[
  {
    "name": "cosca-ai",
    "role": "Chief",
    "department": "AI",
    "description": "AI Chief — ML models, RAG pipelines, embeddings, AI features",
    "tools": ["llm", "embeddings", "rag"],
    "status": "active"
  },
  {
    "name": "cosca-backend",
    "role": "Chief",
    "department": "Backend",
    "description": "Backend Chief — API design, business logic, services",
    ...
  },
  ...
]
```

**Critérios de Sucesso**:
- HTTP 200
- Array com ≥ 40 agentes
- Cada agente tem `name`, `role`, `department`, `description`

#### Passo 2: Executar Task com Auto-Routing

```http
POST /v1/run
Authorization: Bearer <token>
Content-Type: application/json

{
  "prompt": "Create a simple Go function that checks if a number is prime. Return only the code and a brief explanation.",
  "context": {}
}
```

**Resultado Esperado**:
```json
{
  "id": "exec-<uuid>",
  "agent": "cosca-backend",
  "status": "completed",
  "response": "...func isPrime(n int) bool {...",
  "duration_ms": 3500,
  "skills_used": ["go-coding"],
  "memory_id": "mem-<uuid>",
  "tokens": {
    "input": 150,
    "output": 200
  }
}
```

**Critérios de Sucesso**:
- HTTP 200
- `status` = "completed" (não "error" nem "timeout")
- `agent` não é vazio (auto-routing funcionou)
- `response` contém código Go (string "func" ou "package")
- `duration_ms` > 0
- `id` é UUID válido
- `tokens.input` > 0 e `tokens.output` > 0

**Condições de Falha**:
- HTTP 400: `prompt` vazio ou malformado
- HTTP 503: Provider offline → mensagem clara
- Timeout: Task > 60s → contexto cancela
- `status` = "error": Provider retornou erro → `response` contém mensagem de erro

#### Passo 3: Verificar Execução no Histórico

```http
GET /v1/executions?limit=5&status=success
Authorization: Bearer <token>
```

**Resultado Esperado**:
```json
{
  "executions": [
    {
      "id": "exec-<uuid>",
      "agent": "cosca-backend",
      "status": "completed",
      "duration_ms": 3500,
      "timestamp": "2026-07-28T..."
    },
    ...
  ],
  "total": 5,
  "limit": 5,
  "offset": 0
}
```

**Critérios de Sucesso**:
- HTTP 200
- `executions` contém a execução do passo 2 (`id` coincide)
- `status` = "completed" para a execução relevante

#### Passo 4: Verificar Learnings (Memory Store)

```http
GET /v1/memory/search?query=prime+number+function&types=learning
Authorization: Bearer <token>
```

**Resultado Esperado**:
```json
{
  "results": [
    {
      "id": "mem-<uuid>",
      "type": "learning",
      "content": "...Go prime number function...",
      "tags": ["go", "coding", "prime"],
      "layer": "session",
      "timestamp": "2026-07-28T..."
    }
  ],
  "total": 1
}
```

**Critérios de Sucesso**:
- HTTP 200
- Se `memory_id` do passo 2 não é vazio, busca deve retornar o registro
- `type` = "learning"
- `content` contém referência ao prompt ou resposta

**Nota**: Se MAG (Memory-Augmented Generation) estiver desabilitado ou memory_id vazio, este passo é opcional.

#### Passo 5: Executar com Agente Específico

```http
POST /v1/run
Authorization: Bearer <token>
Content-Type: application/json

{
  "prompt": "Name three design patterns for Go microservices",
  "agent": "cosca-architecture"
}
```

**Resultado Esperado**:
```json
{
  "id": "exec-<uuid>",
  "agent": "cosca-architecture",
  "status": "completed",
  "response": "...Circuit Breaker...CQRS...Saga...",
  "skills_used": ["architecture-patterns", "system-design"]
}
```

**Critérios de Sucesso**:
- `agent` = "cosca-architecture" (agente específico foi usado)
- `response` contém nomes de patterns (Circuit Breaker, CQRS, Saga, etc.)
- `status` = "completed"

#### Passo 6: Busca Cross-Agent Knowledge

```http
POST /v1/knowledge/search
Authorization: Bearer <token>
Content-Type: application/json

{
  "query": "#go #coding #prime",
  "search_type": "tag"
}
```

**Resultado Esperado**:
```json
{
  "results": [
    {
      "document": "mem-<uuid>",
      "score": 0.95,
      "match_type": "tag",
      "snippet": "...prime number function..."
    }
  ],
  "total": 1
}
```

**Critérios de Sucesso**:
- Se conhecimento foi indexado (C2 executado), busca por tags retorna resultados
- `match_type` = "tag" (busca por tags FTS5)

### Condições de Falha Globais

| Condição | Sintoma | Recovery |
|----------|---------|----------|
| Provider offline | Todas as tasks falham com HTTP 503 | Tentar provider alternativo ou skip com aviso |
| Rate limit | HTTP 429 após várias requests | Esperar 1 minuto, retry com backoff |
| Engine crash | Servidor cai durante execução | `/health` falha → reiniciar servidor |
| Memory leak | Latência cresce a cada request | Verificar métricas, reiniciar se > 2x baseline |

### Dependências

| Dependência | Obrigatória? | Fallback |
|-------------|-------------|----------|
| LLM Provider | Sim (para execução real) | Usar `--dry-run` ou mock provider |
| Agent Manager | Sim (para routing) | N/A — core da plataforma |
| Memory Engine | Não | MAG desabilitado, memory_id vazio |
| Knowledge Engine | Não | Search retorna vazio, execução prossegue |
| Skill Manager | Não | skills_used vazio |

### Estimativa de Tempo
- **List agents**: 200ms
- **Executar task (auto-route)**: 3-30s (depende do provider)
- **Verificar histórico**: 100ms
- **Verificar learnings**: 100ms
- **Executar com agente específico**: 3-30s
- **Cross-agent search**: 100ms
- **Total**: 10-70s

---

## Cenário 4 — Review Cycle (CI E2E)

### Jornada

**Código produzido** → **PR aberto** → **quality gates G0-G6 executam** → **review por agente Chief** → **aprovação/rejeição** → **merge**

### Pré-condições

| Condição | Como Garantir |
|----------|---------------|
| Repositório Git com branch `main` e `feature/*` | Clonar repo de teste ou usar repo real em fork |
| CI configurada (`.github/workflows/ci.yml`) | Verificar que o arquivo existe |
| Go 1.25 instalado no ambiente CI | `go version` retorna ≥ 1.25 |
| Ferramentas de lint/security disponíveis | `golangci-lint`, `govulncheck`, `gosec` |

### Passos

#### Passo 1: Simular Criação de Feature Branch

```bash
git checkout -b feature/test-code-change
```

#### Passo 2: Criar Mudança de Código (Go)

```go
// internal/testcode/e2e_feature.go
package testcode

// IsPrime checks if a number is prime.
func IsPrime(n int) bool {
    if n < 2 {
        return false
    }
    for i := 2; i*i <= n; i++ {
        if n%i == 0 {
            return false
        }
    }
    return true
}
```

```bash
mkdir -p internal/testcode
cat > internal/testcode/e2e_feature.go << 'EOF'
package testcode

func IsPrime(n int) bool {
    if n < 2 {
        return false
    }
    for i := 2; i*i <= n; i++ {
        if n%i == 0 {
            return false
        }
    }
    return true
}
EOF
```

#### Passo 3: Criar Teste Unitário

```go
// internal/testcode/e2e_feature_test.go
package testcode

import "testing"

func TestIsPrime(t *testing.T) {
    tests := []struct {
        n    int
        want bool
    }{
        {0, false}, {1, false}, {2, true}, {3, true},
        {4, false}, {5, true}, {97, true}, {100, false},
    }
    for _, tt := range tests {
        if got := IsPrime(tt.n); got != tt.want {
            t.Errorf("IsPrime(%d) = %v, want %v", tt.n, got, tt.want)
        }
    }
}
```

#### Passo 4: Executar Quality Gates Sequencialmente

**G0 — Build**:
```bash
go build ./...
```
- **Sucesso**: Exit code 0
- **Falha**: Erro de compilação → teste aborta

**G1 — Lint**:
```bash
golangci-lint run --timeout=5m ./...
```
- **Sucesso**: Exit code 0, zero issues
- **Falha**: Issues encontradas → reportar e abortar

**G2 — Vet**:
```bash
go vet ./...
```
- **Sucesso**: Exit code 0
- **Falha**: Warnings encontrados

**G3 — Test (com race)**:
```bash
go test -race -count=1 ./internal/testcode/...
```
- **Sucesso**: Exit code 0, todos os testes passam, zero race conditions
- **Falha**: Test falha ou race detectada

**G4 — Security**:
```bash
govulncheck ./...
gosec -quiet ./internal/testcode/...
```
- **Sucesso**: Zero CVEs, zero issues High/Critical
- **Falha**: CVE encontrada ou gosec reporta High/Critical

**G5 — Coverage**:
```bash
go test -coverprofile=coverage.out ./internal/testcode/...
go tool cover -func=coverage.out | grep total
```
- **Sucesso**: Coverage ≥ 70% (ou ≥ 80% no diff)
- **Falha**: Coverage abaixo do threshold

**G6 — Docs (opcional para feature code)**:
```bash
# Doc-code validator
.github/workflows/scripts/doc-validator.sh
```
- **Sucesso**: Zero broken references
- **Falha**: Referências quebradas

#### Passo 5: Simular Review por Agente Chief

**Critérios de Review (G9 checklist)**:
```yaml
review:
  reviewer: "cosca-review"
  checklist:
    - name: "SOLID principles"
      status: "pass"
    - name: "Error handling"
      status: "pass"
    - name: "Tests present"
      status: "pass"
      detail: "8 test cases covering edge cases (0, 1, 2, 3, 4, 5, 97, 100)"
    - name: "Naming conventions"
      status: "pass"
    - name: "No security issues"
      status: "pass"
    - name: "Documentation"
      status: "pass"
  verdict: "APPROVED"
  comments: "Clean implementation. Good test coverage. No issues found."
```

**Critérios de Sucesso**:
- Todos os items do checklist marcados como "pass"
- Veredito = "APPROVED" ou "APPROVED_WITH_SUGGESTIONS"
- Comentários não vazios

**Condições de Rejeição**:
- Qualquer item marcado como "fail" → REJECTED
- Veredito = "REJECTED" com razão documentada
- Issues de segurança (P1) → rejeição automática sem exceções

#### Passo 6: Simular Merge

```bash
git checkout main
git merge --no-ff feature/test-code-change -m "feat: add IsPrime utility function"
```

**Critérios de Sucesso**:
- Merge sem conflitos
- Commit message segue convenção (conventional commits)

### Validação Completa do Ciclo

| Gate | Função | Status Final | Bloqueia Merge? |
|------|--------|-------------|-----------------|
| G0 | Build | ✅ pass | Sim |
| G1 | Lint | ✅ pass | Sim |
| G2 | Vet | ✅ pass | Sim |
| G3 | Test | ✅ pass (8/8) | Sim |
| G4 | Security | ✅ pass (0 CVEs) | Sim |
| G5 | Coverage | ✅ 85% | Warning only |
| G6 | Docs | ✅ pass (N/A) | Condicional |
| G9 | Review | ✅ APPROVED | Sim |

### Condições de Falha por Gate

| Gate | Modo de Falha | Comportamento Esperado |
|------|--------------|----------------------|
| G0 | Erro de sintaxe | `go build` falha com mensagem clara |
| G1 | Função sem comentário | golangci-lint reporta issue |
| G2 | Unreachable code | `go vet` reporta warning |
| G3 | Test flaky | `go test` falha intermitentemente |
| G3 | Race condition | `-race` detecta e reporta stack trace |
| G4 | CVE em dependência | `govulncheck` reporta CVE ID + severidade |
| G5 | Cobertura < 70% | Warning, ticket de technical debt |
| G9 | Review rejeitada | Merge bloqueado, PR fechado ou revisão solicitada |

### Dependências

| Dependência | Versão | Propósito |
|-------------|--------|-----------|
| Go | ≥ 1.25 | Compilação e testes |
| golangci-lint | v2.1+ | Lint (G1) |
| govulncheck | latest | Security scan (G4) |
| gosec | v2+ | Code security (G4) |
| Git | ≥ 2.40 | Version control |

### Estimativa de Tempo
- **Criar código + teste**: 30s
- **G0 (Build)**: 10-30s
- **G1 (Lint)**: 20-60s
- **G2 (Vet)**: 5-15s
- **G3 (Test)**: 5-15s
- **G4 (Security)**: 10-30s
- **G5 (Coverage)**: 5-10s
- **G9 (Review)**: manual/agent — 30s-2min
- **Total sequencial**: 1-5min

---

## Cenário 5 — Deploy (CI/CD E2E)

### Jornada

**Tag de release criada** → **CI verification completa** → **Docker build multi-platform** → **security scan** → **release artifacts** → **deploy** → **smoke test**

### Pré-condições

| Condição | Como Garantir |
|----------|---------------|
| Tag git no formato `v*` (ex: `v1.4.0`) | `git tag v1.4.0-test && git push origin v1.4.0-test` |
| Docker Buildx disponível | `docker buildx version` |
| Acesso ao registry (GHCR ou local) | `docker login` ou registry local |
| CI/CD workflow configurado (`.github/workflows/cd.yml`) | Verificar arquivo existe |
| Binário compilado para smoke test | Build stage concluído |

### Passos

#### Passo 1: Criar Tag de Release (Simulado)

```bash
git tag v1.4.0-e2e-test
# O trigger do CD é git push --tags, mas em E2E test podemos simular:
echo "Simulating tag trigger for v1.4.0-e2e-test"
```

**Resultado Esperado**:
- Tag criada no repositório local
- CD pipeline seria triggerado (simulado no E2E)

#### Passo 2: CI Verification (Re-run All Gates)

**Equivalente a re-executar `ci.yml` no tag**:

```bash
# G0-G6 sequencial
go build ./...                        # G0
golangci-lint run --timeout=5m ./...  # G1
go vet ./...                          # G2
go test -race -count=1 ./...          # G3
govulncheck ./...                     # G4a
gosec -quiet ./...                    # G4b
go test -coverprofile=coverage.out ./...  # G5
```

**Critérios de Sucesso**:
- Todos os gates G0-G5 passam com exit code 0
- G5: coverage ≥ 70%

**Condições de Falha**:
- Qualquer gate falhar → release abortada
- CI verification é blocking para todo o pipeline CD

#### Passo 3: Docker Build Multi-Platform

```bash
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  --tag cosca:v1.4.0-e2e-test \
  --load \
  .
```

**Resultado Esperado**:
- Build bem-sucedido para ambas plataformas
- Imagem disponível localmente (`docker images cosca:v1.4.0-e2e-test`)

**Critérios de Sucesso**:
- Exit code 0
- Imagem existe em `docker images`
- Manifest suporta multi-platform

**Condições de Falha**:
- Build falha por dependência faltando
- Platform específica não suportada (ex: ARM64 sem emulador)
- Dockerfile mal configurado

#### Passo 4: Security Scan da Imagem

```bash
# Scan da imagem Docker
docker scout quickview cosca:v1.4.0-e2e-test 2>/dev/null || echo "docker scout not available — skipping"
# Alternativa: Trivy
trivy image cosca:v1.4.0-e2e-test --severity HIGH,CRITICAL 2>/dev/null || echo "trivy not available — skipping"
```

**Resultado Esperado**:
- Zero vulnerabilidades HIGH ou CRITICAL
- Se scanner indisponível, skip com aviso (não bloqueante para E2E)

**Critérios de Sucesso**:
- Scan conclui sem encontrar CVEs críticas

#### Passo 5: Smoke Test do Container

```bash
# 5a: Versão
docker run --rm cosca:v1.4.0-e2e-test version

# 5b: Health check
docker run --rm -d --name cosca-smoke -p 14121:14120 cosca:v1.4.0-e2e-test serve --port 14120
sleep 5
curl -s http://localhost:14121/health

# 5c: Status
docker run --rm cosca:v1.4.0-e2e-test status
```

**Resultado Esperado (5a — version)**:
```
cosca v1.4.0-e2e-test (commit: abc1234, built: 2026-07-28, go1.25)
```

**Resultado Esperado (5b — health)**:
```json
{"healthy": true}
```

**Resultado Esperado (5c — status)**:
```
Runtime: healthy
Agents: 55 registered
Knowledge: available
Memory: available
```

**Critérios de Sucesso**:
- `version` retorna a tag correta
- `/health` retorna `{"healthy": true}`
- `status` reporta runtime healthy
- Container pode ser parado e removido sem leaks

**Condições de Falha**:
- Container não inicia (porta em uso, permissão)
- `/health` retorna erro ou timeout
- `status` reporta componentes degradados
- Container não para (`docker stop` timeout)

#### Passo 5d: Teste de Readiness

```bash
curl -s http://localhost:14121/ready
```

**Resultado Esperado**:
```json
{
  "ready": true,
  "subsystems": {
    "knowledge": "available",
    "memory": "available",
    "runtime": "healthy"
  }
}
```

**Critérios de Sucesso**:
- HTTP 200
- Todos os subsistemas reportam "available" ou "healthy"

#### Passo 6: Cleanup + Verificação de Ausência de Leaks

```bash
# Parar container
docker stop cosca-smoke
docker rm cosca-smoke

# Verificar que porta foi liberada
! curl -s --connect-timeout 2 http://localhost:14121/health
```

**Critérios de Sucesso**:
- Container para em < 10s (SIGTERM)
- Porta 14121 liberada (conexão recusada)
- Nenhum processo zumbi (`docker ps` não lista cosca-smoke)

#### Passo 7: Release Artifacts (Simulado para E2E)

```bash
# Build dos binários multi-plataforma
GOOS=linux GOARCH=amd64 go build -o dist/cosca_linux_amd64 ./cmd/cosca/
GOOS=linux GOARCH=arm64 go build -o dist/cosca_linux_arm64 ./cmd/cosca/
GOOS=darwin GOARCH=amd64 go build -o dist/cosca_darwin_amd64 ./cmd/cosca/
GOOS=darwin GOARCH=arm64 go build -o dist/cosca_darwin_arm64 ./cmd/cosca/

# Verificar binários
file dist/cosca_linux_amd64 | grep -q "ELF 64-bit"
file dist/cosca_darwin_arm64 | grep -q "Mach-O 64-bit"
```

**Critérios de Sucesso**:
- 4 binários gerados (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64)
- Binários são executáveis do tipo correto (ELF, Mach-O)
- Tamanho razoável (> 5MB, binário Go compilado)

### Validação Completa do Pipeline CD

| Stage | Função | Status Final | Bloqueia Release? |
|-------|--------|-------------|-------------------|
| CI Verify | Re-run G0-G6 | ✅ pass | Sim |
| Docker Build | Multi-platform image | ✅ pass (linux/amd64 + arm64) | Sim |
| Security Scan | Trivy/Scout | ✅ pass (0 HIGH/CRITICAL) | Sim |
| Smoke Test | Container health | ✅ pass | Sim |
| Readiness | Subsystem check | ✅ pass | Sim |
| Artifacts | Multi-platform binaries | ✅ pass (4 OS/arch) | Não (Docker é primário) |

### Condições de Falha por Stage

| Stage | Modo de Falha | Comportamento Esperado |
|-------|--------------|----------------------|
| CI Verify | G3 falha (teste flaky) | Release abortada, investigação necessária |
| Docker Build | Emulador ARM64 falha | Fallback para linux/amd64 apenas |
| Security Scan | CVE HIGH encontrada | Release bloqueada (P1 — Segurança acima de funcionalidade) |
| Smoke Test | Container não inicia | Erro de configuração — Dockerfile ou entrypoint |
| Smoke Test | Health check falha | Subsystem degraded — verificar logs |
| Readiness | knowledge "unavailable" | DB corrompido ou migration pendente |

### Dependências

| Dependência | Versão | Propósito |
|-------------|--------|-----------|
| Docker | ≥ 24 | Container build e smoke test |
| Docker Buildx | ≥ 0.12 | Multi-platform builds |
| Go | ≥ 1.25 | Cross-compilation |
| Trivy (opcional) | latest | Container security scan |
| curl | any | Health check HTTP |

### Estimativa de Tempo
- **CI Verify (G0-G5)**: 3-8min
- **Docker Build (multi-platform)**: 2-5min
- **Security Scan**: 30s-2min
- **Smoke Test**: 15-30s
- **Readiness Check**: 5s
- **Artifacts Build**: 1-2min
- **Total**: 7-17min

---

## Recomendações de Ferramentas para Implementação

### Stack de E2E Testing

| Camada | Ferramenta | Justificativa | Status |
|--------|-----------|---------------|--------|
| **CLI E2E** | `os/exec` + `testing` (Go nativo) | Zero dependências externas, consistente com o resto da stack Go. Já usado nos testes unitários. | ✅ Recomendado |
| **API E2E** | `httptest` + `testing` (Go nativo) | Servidor real em processo, sem necessidade de deploy externo. Já usado em `handler/*_test.go`. | ✅ Recomendado |
| **Web Console E2E** | Playwright (`@playwright/test`) | Multi-browser (Chromium, Firefox, WebKit), screenshots, trace viewer, CI integration nativa. Já configurado em `web/playwright.config.ts`. | ✅ Recomendado (já existe) |
| **CI/CD E2E** | `act` (local GitHub Actions) + Go testing | Simula CI pipeline localmente antes do push. Evita "commit and pray". | 🟡 Recomendado |
| **Performance/Load** | k6 (Grafana) | Testes de carga para API endpoints. Integra com Prometheus. Essencial para validar R3 (soak test). | 🟡 Recomendado |
| **Security E2E** | ZAP (OWASP) | DAST — Dynamic Application Security Testing. Complementa os scans estáticos (govulncheck, gosec). | ⚪ Opcional |
| **Contract Testing** | Pact ou schema validation | Garante que API contracts não quebram entre versões. OpenAPI spec já existe — validar respostas contra ela. | ⚪ Opcional |
| **Chaos Engineering** | chaos-mesh ou custom scripts | Injetar falhas (network partition, DB corruption, kill processes) e verificar resiliência. Para R9 (Kernel SPOF). | ⚪ Futuro |

### Configuração Recomendada

```yaml
# .github/workflows/e2e.yml (a ser criado)
name: E2E Tests

on:
  push:
    branches: [main]
  schedule:
    - cron: '0 6 * * *'  # Diário às 6am
  workflow_dispatch:      # Manual trigger

jobs:
  cli-e2e:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.25' }
      - name: Build binary
        run: make build
      - name: Run CLI E2E tests
        run: go test -v -timeout 5m ./test/e2e/cli/...

  api-e2e:
    runs-on: ubuntu-latest
    services:
      ollama:
        image: ollama/ollama:latest
        options: --gpus all
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.25' }
      - name: Run API E2E tests
        run: go test -v -timeout 10m ./test/e2e/api/...

  web-e2e:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with: { node-version: '20' }
      - name: Install pnpm
        run: npm install -g pnpm
      - name: Run Playwright E2E
        run: |
          cd web
          pnpm install
          npx playwright test
      - uses: actions/upload-artifact@v4
        if: failure()
        with:
          name: playwright-report
          path: web/playwright-report/
```

### Estrutura de Diretórios Proposta

```
test/e2e/
├── README.md                    # E2E testing guide
├── cli/                         # Cenário 1 — CLI jornadas
│   ├── e2e_cli_test.go          # Test runner principal
│   ├── fixtures/                # Fixtures de projeto
│   │   └── empty_project/
│   └── testdata/                # Dados de teste
│       └── prompts.txt
├── api/                         # Cenários 2 + 3 — API jornadas
│   ├── e2e_api_test.go          # Test runner principal
│   ├── e2e_knowledge_test.go    # Cenário 2: Knowledge Sync
│   ├── e2e_agent_test.go        # Cenário 3: Agent Task
│   └── testdata/
│       ├── docs/                # Documentos para indexar
│       │   ├── architecture.md
│       │   ├── memory.md
│       │   └── quality_gates.md
│       └── responses/           # Golden responses
│           ├── isprime.json
│           └── patterns.json
├── ci/                          # Cenários 4 + 5 — CI/CD jornadas
│   ├── e2e_review_test.go       # Cenário 4: Review Cycle
│   ├── e2e_deploy_test.go       # Cenário 5: Deploy
│   └── docker-compose.yml       # Stack local de teste
└── shared/                      # Helpers compartilhados
    ├── server.go                # Start/stop cosca serve
    ├── auth.go                  # Login helper
    ├── cleanup.go               # Resource cleanup
    └── assertions.go            # Custom assertions
```

---

## Estimativa de Esforço para Implementação

### Breakdown por Cenário

| Cenário | Complexidade | Dependências Externas | Esforço (dias) | Riscos |
|---------|-------------|----------------------|----------------|--------|
| **C1 — Criar Projeto** | Baixa | Binário cosca, filesystem | 1-2d | Nenhum — é CLI-only, zero deps externas |
| **C2 — Knowledge Sync** | Média | Servidor cosca, SQLite, (opcional: provider para embeddings) | 2-3d | Vector store pode não estar disponível → fallback para FTS5-only |
| **C3 — Agent Task** | Alta | LLM Provider real, Agent Manager, Memory Engine | 3-5d | **Provider dependency é o maior risco** — requer API key ou Ollama local. Timeout e rate-limit handling complexos |
| **C4 — Review Cycle** | Média | Repositório Git, golangci-lint, gosec | 2-3d | Depende de ferramentas externas (lint, security) instaladas |
| **C5 — Deploy** | Alta | Docker, Docker Buildx, registry | 3-4d | Multi-platform build requer QEMU em CI. Container smoke test é frágil (timing) |
| **Shared Helpers** | Baixa | N/A | 1d | Server start/stop, auth, cleanup — código compartilhado |
| **CI Integration** | Média | GitHub Actions, secrets | 1-2d | Configurar workflow, secrets para providers, artifact upload |

### Total Estimado

| Fase | Esforço | Duração |
|------|---------|---------|
| Shared helpers + infra | 1-2 dias | Semana 1 |
| C1 + C2 (menor risco) | 3-5 dias | Semanas 1-2 |
| C4 (CI pipeline) | 2-3 dias | Semana 2 |
| C3 (Agent tasks — maior risco) | 3-5 dias | Semanas 2-3 |
| C5 (Deploy) | 3-4 dias | Semanas 3-4 |
| CI Integration + docs | 1-2 dias | Semana 4 |
| **Total** | **14-21 dias** | **3-4 semanas** |

### Fatores que Aumentam Esforço

1. **Provider instability**: Se o provider de LLM for instável ou tiver rate limits baixos, C3 pode exigir mock provider ou Ollama local → +2d
2. **Flaky tests**: Timing issues em startup de servidor e smoke tests → +1-2d para retry logic e health checks robustos
3. **Docker in CI**: Buildx + QEMU em GitHub Actions é notoriamente lento e frágil → +1d para otimização
4. **Cross-platform**: Se C5 exigir testes reais em ARM64 (não emulação) → +2d para runners auto-hospedados

### Dependências Bloqueantes

| Bloqueador | Status | Ação |
|-----------|--------|------|
| Provider configurável para C3 | Provider precisa estar disponível | Configurar Ollama como provider padrão de teste (zero custo, sem API key) |
| C3 depende de C1 (init) e C2 (knowledge) | C1 e C2 devem ser implementados primeiro | Ordenar implementação: C1 → C2 → C3 |
| Docker Buildx no CI | Já configurado no `ci.yml` — reutilizar | Usar mesmo setup do CI job `docker-build` |
| `.cosca/` state entre testes | Isolamento necessário | `t.TempDir()` garante isolamento |

---

## Apêndice: Critérios de Aceitação

### Definição de "Done" para E2E Tests

- [ ] Todos os 5 cenários implementados e passando em ambiente local
- [ ] Todos os 5 cenários passando no CI (GitHub Actions)
- [ ] Testes são determinísticos (zero flaky tests em 10 execuções consecutivas)
- [ ] Cobertura de branches de erro ≥ 50% (testar paths de falha, não só happy path)
- [ ] Documentação de cada cenário atualizada (este documento)
- [ ] Relatório de execução gerado automaticamente (JUnit XML ou similar)
- [ ] Timeout total da suite E2E < 30 minutos (para CI viável)

### Métricas de Sucesso

| Métrica | Baseline Atual | Target |
|---------|---------------|--------|
| Cobertura E2E (jornadas) | 0% (0/5) | 100% (5/5) |
| Tempo de execução da suite | N/A | < 30min |
| Flaky test rate | N/A | < 5% |
| Bugs encontrados por E2E | 0 (sem testes) | ≥ 1 (esperado em primeira execução) |
| Cobertura total (unit+int+e2e) | ~78% | ≥ 80% |

---

> **Related**: [R10 — Risk Registry](../risk/RISK_REGISTRY.md) | [Testing Strategy](strategy.md) | [Quality Gates](../qa/quality-gates.md) | [CI Pipeline](../../../../.github/workflows/ci.yml) | [CD Pipeline](../../../../.github/workflows/cd.yml)
