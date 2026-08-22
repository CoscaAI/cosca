# MODELO DE MEMORIA — Taxonomia Canonica de Memoria

> **Versao**: 1.4.0-dev | **Status**: active | **Dono**: Memory Chief

## PROPOSITO
Fonte unica de verdade para todos os tipos de memoria, schemas, locais de armazenamento e politicas de ciclo de vida. Referenciado por KERNEL.md, Memory Engine, Memory Chief, Context Engine e Learning Engine.

---

## Arquitetura de Memoria

```
Camada 1: Sessao    (Transiente)      - .cosca/memory/short/
Camada 2: Projeto   (Escopo projeto)  - .cosca/memory/project/, .cosca/memory/long/
Camada 3: Sistema   (Escopo sistema)  - .cosca/memory/architecture/, .cosca/memory/decision/
Camada 4: Sabedoria (Cross-project)   - ${MEMORY_GLOBAL}/pattern/, ${MEMORY_GLOBAL}/bug/
Camada 5: Agente    (Cross-project)   - ${MEMORY_GLOBAL}/agent/
```

---

## Tipos de Memoria

### 1. Memoria Curta — Sessao Transiente
| Atributo | Valor |
|----------|-------|
| Proposito | Contexto de tarefa ativa, decisoes pendentes |
| Escopo | Apenas sessao atual |
| Duracao | Duracao da sessao |
| Local | `.cosca/memory/short/` |
| Limpeza | Limpada no fim da sessao; entradas importantes promovidas para memoria longa |
| Padrao de Acesso | Leitura/Escrita pesada durante sessao |
| Indice | `memory/short/INDEX.md` |

### 2. Memoria Longa — Conhecimento entre Sessoes
| Atributo | Valor |
|----------|-------|
| Proposito | Informacao que persiste entre sessoes |
| Escopo | Duracao do projeto |
| Duracao | Permanente (duracao do projeto) |
| Local | `.cosca/memory/long/` |
| Limpeza | Manual ou via Evolution Engine |
| Padrao de Acesso | Escrita no fim da sessao, leitura no inicio |
| Indice | `memory/long/INDEX.md` |

### 3. Memoria de Projeto — Conhecimento Especifico
| Atributo | Valor |
|----------|-------|
| Proposito | Features, modulos, releases, metricas, issues |
| Escopo | Duracao do projeto |
| Duracao | Duracao do projeto |
| Local | `.cosca/memory/project/` |
| Sub-armazenamentos | `features/`, `modules/`, `releases/`, `metrics/`, `issues/` |
| Padrao de Acesso | Leitura/Escrita continua |
| Indice | `memory/project/INDEX.md` |

### 4. Memoria de Arquitetura — Arquitetura do Sistema
| Atributo | Valor |
|----------|-------|
| Proposito | ADRs, padroes de design, contratos de modulo, pontos de integracao |
| Escopo | Duracao do sistema |
| Duracao | Permanente (duracao do sistema) |
| Local | `.cosca/memory/architecture/` |
| Sub-armazenamentos | `adr/`, `patterns/`, `contracts/`, `integrations/`, `decisions/` |
| Padrao de Acesso | Escrita em decisoes, leitura em planejamento |
| Indice | `memory/architecture/INDEX.md` |

### 5. Memoria de Decisao — Registro de Decisoes
| Atributo | Valor |
|----------|-------|
| Proposito | Todas as decisoes significativas e seu raciocinio |
| Escopo | Permanente |
| Duracao | Para sempre |
| Local | `.cosca/memory/decision/` |
| Padrao de Acesso | Escrita apos cada decisao, leitura no carregamento de contexto |
| Indice | `memory/decision/INDEX.md` |

### 6. Memoria de Padroes — Sabedoria Cross-Project
| Atributo | Valor |
|----------|-------|
| Proposito | Padroes que funcionam, anti-padroes a evitar |
| Escopo | Global (cross-project) |
| Duracao | Permanente |
| Local | `${MEMORY_GLOBAL}/pattern/` |
| Sub-armazenamentos | `architecture/`, `design/`, `code/`, `testing/`, `performance/`, `security/`, `anti-patterns/`, `frameworks/` |
| Padrao de Acesso | Escrito por Evolution/Learning engines, lido globalmente |
| Indice | `memory/pattern/INDEX.md` |

### 7. Memoria de Bug — Catalogo de Bugs
| Atributo | Valor |
|----------|-------|
| Proposito | Bugs encontrados e suas correcoes |
| Escopo | Global (cross-project) |
| Duracao | Permanente |
| Local | `${MEMORY_GLOBAL}/bug/` |
| Padrao de Acesso | Escrito apos correcoes de bugs, lido durante diagnostico |
| Indice | `memory/bug/INDEX.md` |

### 8. Memoria de Agente — Performance do Agente
| Atributo | Valor |
|----------|-------|
| Proposito | Metricas de performance, preferencias, historico de aprendizado |
| Escopo | Global (cross-project) |
| Duracao | Permanente |
| Local | `${MEMORY_GLOBAL}/agent/` |
| Padrao de Acesso | Escrito por Learning Engine, lido por Kernel |
| Indice | `memory/agent/INDEX.md` |

---

## Schema do Registro de Memoria

Todos os registros de memoria seguem este schema YAML frontmatter:

```yaml
---
type: short | long | project | architecture | decision | pattern | bug | agent
key: identificador-unico
tags: [tag1, tag2]
timestamp: ISO8601
status: active | archived | superseded
related: [chave1, chave2]
agent: nome-do-agente
session: id-da-sessao
---
```

### Extensoes por Tipo

#### Registros de Decisao
```yaml
decided_by: nome-do-agente
confidence: 0.0-1.0
alternatives_considered: [alt1, alt2]
```

#### Registros de Padrao
```yaml
category: architecture | design | code | testing | performance | security | anti-pattern
confidence: 0.0-1.0
times_used: N
times_succeeded: N
```

#### Registros de Bug
```yaml
severity: critical | high | medium | low
fix_commit: hash
related_patterns: [padrao-chaves]
```

#### Registros de Agente
```yaml
agent_type: chief | specialist | engine
department: nome-do-departamento
metric_type: performance | preference | learning
```

---

## Operacoes de Memoria

| Operacao | Descricao | Gatilho |
|----------|-----------|---------|
| **Armazenar** | Escrever registro no armazenamento apropriado | Automatico (decisoes, bugs, padroes) ou manual (agentes) |
| **Recuperar** | Ler registros por chave, tags ou periodo | Inicio de sessao, carregamento de contexto |
| **Buscar** | Busca full-text nos armazenamentos | Consultas de agente, correspondencia de padroes |
| **Indexar** | Reconstruir metadados de busca | Apos escritas em lote |
| **Podar** | Arquivar registros antigos/irrelevantes | Periodico (Evolution Engine) |
| **Promover** | Mover registro de memoria curta para longa | Fim da sessao |

---

## Ciclo de Vida da Memoria — 4 Niveis (decisao do Don, 2026-08-17)

A regra de ouro: **"coloca tudo em medio, o longo a gente vai ver o que coloca"** — nada nasce longo. O que sobrevive a janela de 7 dias so sobrevive por promocao manual.

### Os 4 Niveis

| Nivel | Janela | Onde vive | Como entra |
|-------|--------|-----------|------------|
| **Permanente** | pra sempre | blockchain (`chain.dat` + `blocks/` + `merkle/`) | fluxo de registro L (MEMORY_ACCESS_PROTOCOL sec 3) |
| **Longo** | 1 ano | `knowledge.db` — `tier = long` | **promocao EXPLICITA** (`cosca knowledge promote` / `cosca memory promote`) |
| **Medio** | 7 dias | `knowledge.db` — `tier = medium` | **DEFAULT de tudo** (acquire, compile, index, note) |
| **Curto** | sessao / 24h | `session.db` + `LayerSession` (24h) + `LayerTemp` (1h) | gravacao de sessao |

### Ciclo de Vida

```
escrita ──► medium (7d) ──► GC automatico apaga
                │
                └── promote (explicito) ──► long (1y) ──► GC apaga em 1 ano
```

### Fases

| Fase | Acao |
|------|------|
| Criar | Escrever registro com `status: active` |
| Ativo | Disponivel para recuperacao e busca |
| Promover | Mover de curto para longo no fim da sessao |
| Arquivar | Marcar `status: archived`, mover para subdiretorio de arquivo |
| Podar | Deletar registros mais antigos que o periodo de retencao |

### Comandos

```bash
cosca knowledge gc                          # roda o ciclo de vida (expiracao)
cosca knowledge gc --json
cosca knowledge promote <doc-id> --tier long    # promove p/ 1 ano
cosca knowledge promote --all --tier long       # promove todos do medio
cosca memory promote <id> --tier long           # record do MemoryEngine
cosca memory prune --dry-run                    # preview do prune
cosca knowledge verify --fix                    # re-embed + limpa dangling
```

### Integridade e Vetores

- A migracao V7 (`memory_tiers_medium_default`) adicionou `tier` + `expires_at` em `documents` e backfillou tudo para `medium` (7 dias).
- **Vetores de entidade (`entity_id != ''`) NUNCA sao dangling** — o `CleanupDanglingVectors` preserva `entity_id` (L338); o `verify` conta so vetores de chunk no match.
- `verify --fix` re-embedda chunks sem vetor; `index-entities` vetoriza os nos do grafo.

### Politicas de Retencao por Tipo

| Tipo de Memoria | Retencao |
|-----------------|----------|
| Curto | Apenas sessao atual |
| Longo | Duracao do projeto |
| Projeto | Duracao do projeto |
| Arquitetura | Para sempre (duracao do projeto) |
| Decisao | Para sempre |
| Padrao | Para sempre (revisao periodica) |
| Bug | Para sempre (revisao periodica) |
| Agente | Ultimos 12 meses (janela rolling) |

---

## RELACIONADOS

- [Memory Engine](engines/memory/SKILL.md) — Operacoes de memoria e regras de auto-captura
- [Memory Chief](departments/memory/SKILL.md) — Gerenciamento de memoria
- [Context Engine](engines/context/SKILL.md) — Construcao de contexto a partir da memoria
- [Learning Engine](engines/learning/SKILL.md) — Atualizacoes de memoria de agentes
- [Evolution Engine](engines/evolution/SKILL.md) — Atualizacoes de memoria de padroes e bugs
- [KERNEL.md](KERNEL.md) — Carregamento de memoria no inicio da sessao

---

## HISTORICO

| Versao | Data | Autor | Mudancas |
|--------|------|-------|----------|
| 1.0.0 | 2026-07-10 | Memory Chief | Taxonomia canonica inicial de memoria |

---

> **Executado por**: Memory Engine | **Ultima revisao**: 2026-07-10

---

## Memoria Auto-Evolucao Semantica (v4.0.0 — 2026-07-27)

### Conceito
Cada agente mantem uma memoria semantica auto-evolutiva que cresce com a experiencia. Agentes progridem por 5 niveis de capacidade aprendendo com cada tarefa e aplicando tecnicas cada vez mais avancadas.

### Arquitetura
```
internal/embed/cosca/memory/agent/{nome-do-agente}/
├── learnings.md    ← Diario semantico (indexado FTS5, buscavel por vetor)
├── evolution.md    ← Rastreamento de nivel de capacidade
├── patterns.md     ← Padroes de solucao reutilizaveis
└── INDEX.md        ← Referencia cruzada para recuperacao rapida
```

### O Loop de Evolucao
1. **RECUPERAR**: Agente busca em learnings.md por #tags correspondentes a tarefa atual
2. **APLICAR**: Agente usa a tecnica de nivel mais alto encontrada (nunca regredir)
3. **EXECUTAR**: Agente realiza a tarefa com a tecnica selecionada
4. **APRENDER**: Agente registra resultado, tecnica, nivel e direcao de melhoria
5. **EVOLUIR**: Com o tempo, agente progride Nivel 1-2-3-4-5

### Progressao de Nivel
| Nivel | Nome | Gatilho | Exemplo (Security Chief) |
|-------|------|---------|--------------------------|
| 1 | Basico | Inicializacao | Checklist OWASP Top 10 |
| 2 | Intermediario | 5 tarefas L1 bem-sucedidas | govulncheck automatizado + revisao manual |
| 3 | Avancado | 10 tarefas L2 bem-sucedidas | Modelagem de ameacas STRIDE por subsistema |
| 4 | Especialista | 15 tarefas L3 bem-sucedidas | Descoberta de vetores de ataque novos, padroes zero-day |
| 5 | Mestre | 20 tarefas L4 bem-sucedidas | Contribuicao de novas tecnicas OWASP, treinamento de outros agentes |

### Aprendizado Cross-Agent
Todos os learnings sao indexados no knowledge engine (SQLite FTS5 + embeddings vetoriais). O Knowledge Engine indexa internal/embed/cosca/memory/agent/ recursivamente. O padrao de seguranca do Agente A pode ser recuperado semanticamente pelo Agente B quando enfrenta uma tarefa relacionada.

### Busca Semantica
Antes de qualquer tarefa, agentes executam: `cosca knowledge search "#security #xss"` para encontrar learnings relevantes. Resultados ordenados por: nivel (maior = melhor), recencia (mais recente = mais relevante), resultado (sucesso > parcial > falha).

### Saude da Memoria
- Maximo de learnings por agente: ilimitado (diario append-only)
- Indexacao: automatica via FTS5 a cada escrita
- Integridade de referencia cruzada: validada pelo Memory Chief semanalmente
- Expiracao: learnings nunca expiram (conhecimento acumulativo)
