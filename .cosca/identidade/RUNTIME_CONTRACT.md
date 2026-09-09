# CONTRATO DE RUNTIME — Interface Cosca Kernel ↔ Runtime

> **Version**: 2.0.0 | **Status**: active | **Owner**: Cosca Kernel | **Last Updated**: 2026-08-01
> **Decision**: ADR-7423 — versionamento de contratos por método

## PROPÓSITO
Este documento define a interface formal entre o Cosca Kernel e qualquer ambiente de execução de Runtime. O Runtime é responsável por executar os comportamentos definidos pelo Cosca Kernel. Este contrato garante que qualquer Runtime conformante (CLI, API server, Dashboard, IDE plugin, agente headless) possa consumir as diretrizes do Cosca de forma consistente.

## ARQUITETURA

```
┌──────────────────────────────────────┐
│              Cosca KERNEL              │
│  (defines behavior, routes work,     │
│   enforces quality gates)            │
└──────────────┬───────────────────────┘
               │ RUNTIME CONTRACT
               │ (this document)
               ▼
┌──────────────────────────────────────┐
│             RUNTIME LAYER            │
│  ┌────────┐ ┌────────┐ ┌─────────┐  │
│  │  CLI   │ │  API   │ │Dashboard│  │
│  └────────┘ └────────┘ └─────────┘  │
│  ┌────────┐ ┌────────┐              │
│  │  SDK   │ │  IDE   │  ...         │
│  └────────┘ └────────┘              │
└──────────────────────────────────────┘
```

## 1. INTERFACE DO KERNEL

### 1.1 Ciclo de Vida da Sessão

| Operação | Direção | Descrição |
|-----------|---------|-------------|
| `session.init` | Runtime → Kernel | Inicializar uma nova sessão Cosca |
| `session.discover` | Kernel → Runtime | Solicitar descoberta do workspace |
| `session.context` | Kernel → Runtime | Solicitar carregamento de contexto |
| `session.route` | Kernel → Runtime | Rotear uma requisição do usuário |
| `session.execute` | Kernel → Runtime | Executar um passo do workflow |
| `session.teardown` | Runtime → Kernel | Encerrar sessão, persistir memória |

### 1.2 Protocolo de Requisição/Resposta

```
Request:  { type, payload, metadata }
Response: { type, payload, status, errors }
```

### 1.3 Tipos de Requisição Suportados

| Tipo | Payload | Resposta |
|------|---------|----------|
| `feature` | Descrição da feature, restrições | Plano Executivo + Implementação |
| `bug` | Descrição do bug, passos, comportamento esperado | Correção + Testes de regressão + Padrão |
| `refactor` | Alvo, razão, escopo | Código refatorado + Comparação de qualidade |
| `review` | Alvo, review_type | Relatório de revisão + Aprovação |
| `deploy` | Ambiente, versão, estratégia | Status do deploy + Verificação |
| `docs` | Docs alvo, escopo | Documentação atualizada |
| `status` | — | Saúde do projeto + Métricas de qualidade |
| `evolve` | — | Relatório de evolução + Recomendações |

## 2. RESPONSABILIDADES DO RUNTIME

### 2.1 O Runtime DEVE:

1. **Carregar o Cosca Kernel** — Analisar e executar as diretrizes do KERNEL.md
2. **Invocar engines** — Carregar skills de engine quando acionado pelo Kernel
3. **Criar agentes** — Criar sessões de subagentes para chiefs e especialistas
4. **Aplicar quality gates** — Aplicar verificações do QUALITY_GATES.md
5. **Gerenciar memória** — Ler/escrever nos armazenamentos de memória conforme MEMORY_MODEL.md
6. **Rastrear estado** — Manter estado do workflow, status de tarefas, contexto da sessão
7. **Reportar erros** — Escalar falhas conforme a tabela de tratamento de erros do KERNEL.md
8. **Fornecer tools** — Expor E/S de arquivos, git, shell, busca via Tools Engine

### 2.2 O Runtime PODE:

1. **Paralelizar** — Executar tarefas independentes concorrentemente
2. **Cachear** — Armazenar em cache contexto, consultas de memória, resultados de descoberta
3. **Otimizar** — Escolher o modelo de IA ideal por tipo de tarefa
4. **Estender** — Adicionar tools personalizadas além do catálogo da Tools Engine
5. **Persistir** — Armazenar snapshots de sessão para capacidade de retomada

### 2.3 O Runtime NÃO DEVE:

1. **Pular a cadeia de comando** — Nunca rotear trabalho diretamente para especialistas
2. **Ignorar quality gates** — Nunca entregar sem passar pelo Gate 2+
3. **Modificar skills** — Nunca alterar SKILL.md ou definições de workflow
4. **Sobrescrever decisões** — Nunca sobrescrever decisões de CEO/CTO/Chief
5. **Hardcodar caminhos** — Sempre usar Caminhos Virtuais do Resource Resolver

## 3. INTERFACE DE CRIAÇÃO DE AGENTES

### 3.1 Requisição de Criação

```json
{
  "type": "spawn_agent",
  "agent_type": "chief | specialist | engine",
  "department": "backend | frontend | security | ...",
  "task": "Task description with context",
  "context": { "workflow_id": "...", "step_id": "..." },
  "tools": ["read_file", "write_file", "execute_command"],
  "timeout_ms": 300000,
  "retry": { "max": 3, "backoff_ms": 5000 }
}
```

### 3.2 Resposta da Criação

```json
{
  "agent_id": "uuid",
  "status": "running | completed | failed",
  "output": {},
  "review": { "score": 8.5, "issues": [] },
  "duration_ms": 45000,
  "attempts": 1,
  "errors": []
}
```

## 4. INTERFACE DE TOOLS

### 4.1 Tools Disponíveis para Todos os Agentes

| Categoria | Tools |
|----------|-------|
| E/S de Arquivos | read_file, write_file, edit_file, list_directory, find_files, search_content |
| Código | execute_command, run_tests, run_linter, run_typecheck, run_build |
| Git | git_status, git_diff, git_log, git_branch, git_checkout, git_commit |
| Memória | store_memory, retrieve_memory, search_memory |
| Docs | generate_readme, generate_api_docs, generate_adr, update_changelog |
| Qualidade | run_security_scan, check_test_coverage, run_complexity_analysis |
| Workflow | get_workflow_status, create_workflow, execute_workflow |

### 4.2 Permissões de Tools

| Nível | Agentes | Tools |
|-------|---------|-------|
| Leitura | Todos | read_file, list_directory, find_files, search_content, git_status, git_diff, git_log |
| Escrita | Chiefs | write_file, edit_file, git_commit, store_memory |
| Execução | Chiefs | execute_command, run_tests, run_linter, run_build |
| Admin | Kernel, CEO, CTO | spawn_agent, kill_agent, install_dependencies |

## 5. EVENT BUS

### 5.1 Eventos Emitidos pelo Kernel

| Evento | Payload | Consumidores |
|--------|---------|-------------|
| `session.started` | session_id, timestamp | Observability, Audit |
| `workflow.created` | workflow_id, type | Workflow Engine, Audit |
| `task.assigned` | task_id, agent, department | Execution Engine, Audit |
| `task.completed` | task_id, output, duration | Review Engine, Audit |
| `task.failed` | task_id, error, attempts | Execution Engine, Audit |
| `review.completed` | review_id, score, issues | Quality Engine, Audit |
| `qa.completed` | qa_id, passed, issues | Release Chief, Audit |
| `decision.made` | decision_id, type, rationale | Memory Engine, Audit |
| `session.ended` | session_id, summary, learnings | Memory Engine, Learning Engine |

### 5.2 Eventos Consumidos pelo Runtime

| Evento | Ação |
|--------|------|
| `session.started` | Inicializar observabilidade, carregar contexto |
| `task.assigned` | Criar agente, monitorar progresso |
| `task.completed` | Armazenar output, acionar próximo passo |
| `task.failed` | Repetir ou escalar conforme tabela de erros |
| `decision.made` | Armazenar em memória |
| `session.ended` | Persistir memória, gerar relatório da sessão |

## 6. INTEGRAÇÃO DE QUALITY GATE

| Gate | Quando | Ação do Runtime |
|------|--------|----------------|
| Gate 0 | Antes de qualquer trabalho | Validar tipo de requisição, escopo, departamentos |
| Gate 1 | Após geração do plano | Validar arquitetura, segurança, dependências |
| Gate 2 | Após implementação | Executar verificações automatizadas + revisão + QA |
| Gate 3 | Antes do release | Suite completa de testes + scan de segurança + verificação de docs |
| Gate 4 | Após o release | Health checks + monitoramento de erros + feedback do usuário |

## 7. CONTRATO DE TRATAMENTO DE ERROS

| Tipo de Erro | Repetição | Escalar Para | Ação do Runtime |
|------------|-------|-------------|----------------|
| Timeout de agente | 3x, backoff exponencial | Agente secundário | Recriar com agente diferente |
| Falha de agente | 1x | Department Chief | Registrar erro, escalar |
| Falha de validação | 0 | Chief | Retornar ao agente com feedback |
| Falha de dependência | 3x | CTO | Bloquear tarefas dependentes |
| Todos os caminhos esgotados | — | Usuário | Notificar com diagnóstico |

## 8. INTERFACE DE MEMÓRIA

| Operação | Implementação no Runtime |
|-----------|----------------------|
| Armazenar | Escrever arquivo markdown com frontmatter YAML conforme MEMORY_MODEL.md |
| Recuperar | Ler por chave ou consultar por tags |
| Buscar | Busca em texto completo nos armazenamentos de memória |
| Indexar | Reconstruir metadados de busca |
| Podar | Arquivar registros mais antigos que o período de retenção |

## 9. VERSIONED CONTRACTS (v2.0.0)

> Decisão: [ADR-7423](../fallback/knowledge/architecture/adr/adr-7423-versioned-contracts.md). Padrão
> adaptado do framework `versioned-rpc` do Traycer (open-source, MIT) — ver
> `.cosca/memory/project/traycer-analysis.md`.

Todo método RPC do contrato declara uma versão `{ major, minor }` própria, com schemas
de request/response. As invariantes abaixo são verificadas **em tempo de carga do
registry** (CI obrigatória, `make contract-validate`), falhando o build se violadas.

### 9.1 Regra de ouro — minor é somente aditivo

Entre minors do mesmo major, mudanças de schema devem ser **apenas aditivas**:
campos novos obrigatoriamente opcionais; nada removido, renomeado ou alterado em tipo.
Violação → build falha com a mensagem exata do campo.

### 9.2 Major é obrigatoriamente breaking

Um bump de major sem mudança real de schema (request E response) falha com
"could have shipped as a minor". Proíbe major de mentira e força disciplina.

### 9.3 Downgrade explícito + floor methods

- Cada major declara paths de downgrade a partir do seu latest minor.
- Métodos fora do floor declararam `degrade`: `unsupported` ou `fallback`
  (adapta request/response para um método floor).
- Garante que cliente novo ↔ host antigo conversam sem derrubar a conexão.

### 9.4 Negociação de manifesto

O handshake troca manifesto de capacidades (`method → {major, minor}`) com mirror
check em ambos os lados. Incompatibilidade → erro tipado com guidance de upgrade
(client-missing-method / host-missing-method / no-bridge).

### 9.5 Registry central

`internal/contracts/` é a única fonte de verdade dos contratos, carregada no boot
de todo runtime (CLI, API, dashboard, plugins). Nenhum método fora do registry.

## 10. COMPATIBILIDADE

| Versão | Requisito do Runtime | Alterações Incompatíveis |
|---------|-------------------|-----------------|
| 1.0.0 | Floor inicial — todos os métodos migram como major 1, baseline de compatibilidade | — |
| 2.0.0 | Versionamento por método (seção 9) ativo; métodos v1 formam o floor | Sem mudança de contrato para runtimes v1 (compatibilidade preservada via floor) |

## RELACIONADOS
- [KERNEL.md](KERNEL.md) — Orquestração do Kernel
- [MEMORY_MODEL.md](MEMORY_MODEL.md) — Taxonomia de memória
- [QUALITY_GATES.md](QUALITY_GATES.md) — Definições de quality gates
- [engines/resource-resolver/SKILL.md](../engines/knowledge/SKILL.md) — Resolução de Caminhos Virtuais
- [ADR-7423](../fallback/knowledge/architecture/adr/adr-7423-versioned-contracts.md) — Decisão de versionamento
- [BRIDGE_ARCHITECTURE.md](../fallback/knowledge/architecture/BRIDGE_ARCHITECTURE.md) — Referência: ponte Host↔OpenCode (padrão Vercel AI SDK) para futuro bridge Cosca

## HISTÓRICO

| Versão | Data | Autor | Alterações |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-12 | Cosca Kernel | Contrato inicial de runtime — formalizada interface Kernel↔Runtime |
| 2.0.0 | 2026-08-01 | Cosca Kernel | Versionamento por método (ADR-7423) — minor aditivo, major breaking, floor methods, manifesto |
