# ENGINE DE DETECÇÃO PROATIVA DE LACUNAS ("NÃO SEI")

> **Versão**: 1.0.0 | **Status**: active | **Owner**: Cosca Context Chief + Cosca Discovery Chief | **Criado**: 2026-07-30
>
> **Autoridade constitucional**: [CONSTITUTION.md](../../CONSTITUTION.md) — Implementa P4 (intervalo de confiança), P5 (aprender com erros) e P2 (código é a verdade). Estende o [CONFIDENCE_MODEL.md](../evidence/CONFIDENCE_MODEL.md) com penalidade de lacuna.
>
> **Lema**: *"Melhor dizer 'não sei — deixa eu descobrir' do que chutar e quebrar coisas."*
>
> ⚠️ **Complementar à F1.3**: Esta engine cobre **gaps cognitivos** (conhecimento, contexto, risco, dependência, suposição). Para **gaps de qualidade de código** (cobertura, dívida técnica, documentação, segurança, arquitetura, memória de agente), veja a [F1.3 Proactive Gap Detection Engine](./SKILL.md).

---

## PROPÓSITO

A Engine de Detecção Proativa de Lacunas é a implementação do insight do Don: sistemas confiáveis dizem "não sei" antes de agir. Em vez de completar lacunas de conhecimento com suposições — o comportamento padrão da maioria dos LLMs — esta engine identifica **proativamente** o que o sistema não sabe, classifica a severidade da lacuna, busca resolução antes da execução, e ajusta a confiança efetiva para refletir o que realmente se sabe.

O modelo reativo atual (confidence ≥ 0.85 prossegue, < 0.50 escala) só dispara quando a confiança JÁ está baixa. Esta engine opera **antes** da execução — ela pergunta "o que eu não sei sobre esta tarefa?" independentemente do nível de confiança aparente.

## ATIVAÇÃO

Esta engine é ativada obrigatoriamente nos seguintes pontos do ciclo de vida do Kernel:

| Ponto de Ativação | Gatilho | Prioridade |
|-------------------|---------|------------|
| **Pré-Planejamento** | Antes do Planning Engine (§5 do KERNEL.md) gerar o DAG | P0 — Bloqueante |
| **Pré-Execução** | Antes do Scheduler (§6 do KERNEL.md) despachar qualquer nó | P0 — Bloqueante |
| **Pré-Delegação** | Antes do Kernel delegar a um Chief ou Specialist | P0 — Bloqueante |
| **Mudança de Domínio** | Quando a task cruza domínios (ex: backend → infra) | P1 — Aviso |
| **Pós-Incidente** | Após qualquer incidente registrado em `reflection.incident_log` | P1 — Aviso |
| **Sessão** | Uma vez por sessão como health check cognitivo | P2 — Silencioso |

A engine NÃO bloqueia a ação — ela informa a ação. O resultado do scan de lacunas ajusta a confiança efetiva; se a confiança cair abaixo do threshold, a ação escala normalmente.

## ESCOPO

### O Que Esta Engine Cobre

- **Lacunas de conhecimento**: O que o sistema sabe que não sabe sobre o domínio da task
- **Lacunas de contexto**: Informação de projeto que deveria estar disponível mas não está
- **Lacunas de risco**: Riscos não avaliados ou subestimados
- **Lacunas de dependência**: Pré-requisitos, dependências transitivas, requisitos de ambiente
- **Lacunas de suposição**: Premissas não verificadas que sustentam o plano de execução
- **Ajuste de confiança**: Penalidade aplicada à confiança base para refletir incerteza real
- **Ciclo de aprendizado**: Conversão de lacunas resolvidas em conhecimento permanente

### O Que Esta Engine NÃO Cobre

- **Verificação de fatos**: Isso é responsabilidade do [Evidence Engine](../evidence/CONFIDENCE_MODEL.md)
- **Validação de qualidade**: Isso é responsabilidade do [Quality Engine](../quality/) e [QUALITY_GATES.md](../../QUALITY_GATES.md)
- **Análise de segurança**: Isso é responsabilidade do [Security Chief](../../departments/security/)
- **Review de código**: Isso é responsabilidade do [Review Engine](../review/)
- **Execução da resolução**: Esta engine identifica lacunas; os Chiefs e Specialists as resolvem

---

## PROCESSO

### Visão Geral do Pipeline

```
TASK RECEBIDA
     │
     ▼
┌─────────────────────────────────────────────────────────────┐
│ FASE 1: SCAN PROATIVO DE LACUNAS                            │
│   ├── 1.1 Identificar domínios envolvidos na task            │
│   ├── 1.2 Para cada domínio, executar as 4 perguntas-chave   │
│   ├── 1.3 Classificar cada lacuna encontrada                 │
│   └── 1.4 Atribuir severidade (CRITICAL / HIGH / MEDIUM / LOW)│
└─────────────────────────────────────────────────────────────┘
     │
     ▼
┌─────────────────────────────────────────────────────────────┐
│ FASE 2: ESTRATÉGIA DE RESOLUÇÃO                             │
│   ├── 2.1 Para cada lacuna, determinar caminho de resolução  │
│   ├── 2.2 Lacunas resolvíveis → agendar auto-resolução       │
│   ├── 2.3 Lacunas delegáveis → encaminhar para specialist    │
│   └── 2.4 Lacunas escaláveis → preparar pergunta para o Don  │
└─────────────────────────────────────────────────────────────┘
     │
     ▼
┌─────────────────────────────────────────────────────────────┐
│ FASE 3: AJUSTE DE CONFIANÇA                                 │
│   ├── 3.1 Calcular gap_penalty baseado em quantidade/severidade│
│   ├── 3.2 effective_confidence = base_confidence × (1 - penalty)│
│   └── 3.3 Se effective < threshold → escalar                 │
└─────────────────────────────────────────────────────────────┘
     │
     ▼
┌─────────────────────────────────────────────────────────────┐
│ FASE 4: REGISTRO E APRENDIZADO                              │
│   ├── 4.1 Registrar lacunas no GAP_REGISTRY.md              │
│   ├── 4.2 Atualizar cognitive-state.md com gaps ativos      │
│   ├── 4.3 Pós-resolução: criar learning entries             │
│   └── 4.4 Pós-falha: criar failure entries                  │
└─────────────────────────────────────────────────────────────┘
     │
     ▼
   EXECUÇÃO (com confiança ajustada e lacunas documentadas)
```

---

### Fase 1: Scan Proativo de Lacunas

#### 1.1 Identificação de Domínios

Antes de escanear lacunas, a engine identifica todos os domínios de conhecimento que a task toca:

```yaml
domain_mapping:
  - task_type: "init"
    domains:
      - go_build_system       # make embed-sync, go build
      - embed_management      # internal/embed/cosca/
      - filesystem_ops        # escrita de arquivos de framework
      - cli_runtime           # Comportamento do comando cosca init
      - git_versioning        # O que o git status revela sobre estado atual
      - safety_constraints    # P8, guardrails do cognitive-state

  - task_type: "database_migration"
    domains:
      - sql_schema            # Estrutura das tabelas
      - migration_tool        # Ferramenta específica (golang-migrate, etc.)
      - orm_layer             # Se usa ORM, como o ORM interage com migrations
      - data_integrity        # Constraints, foreign keys, índices
      - backup_strategy       # Snapshots antes da migration
      - rollback_plan         # Como reverter se falhar
```

#### 1.2 As Quatro Perguntas-Chave

Para cada domínio identificado, a engine faz **obrigatoriamente** estas quatro perguntas:

| # | Pergunta | Propósito | Exemplo de Resposta |
|---|----------|-----------|---------------------|
| **Q1** | "O que eu **não sei** sobre este domínio?" | Identificar gaps de conhecimento técnico | "Não sei a assinatura exata de `memfd_create` em Go" |
| **Q2** | "Quais **suposições** estou fazendo?" | Explicitar premissas não verificadas | "Estou assumindo que o embed foi sincronizado antes do build" |
| **Q3** | "O que pode dar errado que eu **não considerei**?" | Antecipar riscos cegos | "Não considerei que `--force` pode sobrescrever arquivos modificados manualmente" |
| **Q4** | "Qual **informação** eu preciso mas **não tenho**?" | Identificar dados faltantes | "Não tenho o timestamp do último `make embed-sync`" |

#### 1.3 Classificação de Lacunas

Cada lacuna é classificada em um dos cinco tipos:

```yaml
gap_classification:
  GAP_KNOWLEDGE:
    symbol: "🧠"
    description: "Falta de conhecimento técnico sobre API, biblioteca, linguagem ou protocolo"
    severity_default: "MEDIUM"
    examples:
      - "Não conheço a API do memfd_create em Go (unix.MemfdCreate)"
      - "Não sei como o wazero gerencia WebAssembly memory.grow"
      - "Não sei se o SQLite suporta FTS5 com trigramas"
    resolution_priority: "Auto-resolução (buscar docs, código, exemplos)"

  GAP_CONTEXT:
    symbol: "📍"
    description: "Falta de contexto sobre o estado atual do projeto, ambiente ou workspace"
    severity_default: "HIGH"
    examples:
      - "Não sei quando foi o último make embed-sync"
      - "Não sei quais arquivos foram modificados desde o último commit"
      - "Não sei se o banco de desenvolvimento está sincronizado com produção"
    resolution_priority: "Auto-resolução (git log, file timestamps, make --dry-run)"

  GAP_RISK:
    symbol: "⚠️"
    description: "Riscos não avaliados ou consequências não antecipadas de uma ação"
    severity_default: "CRITICAL"
    examples:
      - "Não sei o que acontece se init --force rodar com embed desatualizado"
      - "Não avaliei o risco de corrupção do UCSS durante esta operação"
      - "Não considerei que a migration pode travar a tabela por minutos"
    resolution_priority: "Delegar (Chief do domínio) ou Escalar (Don)"

  GAP_DEPENDENCY:
    symbol: "🔗"
    description: "Dependências desconhecidas — pré-requisitos, requisitos de sistema, permissões"
    severity_default: "HIGH"
    examples:
      - "Não sei se init --force requer permissão de escrita em /usr/local"
      - "Não sei se esta migration depende da migration #014"
      - "Não sei se esta operação requer sudo ou pode rodar como user"
    resolution_priority: "Auto-resolução (Makefile, código fonte, documentação)"

  GAP_ASSUMPTION:
    symbol: "🤔"
    description: "Premissas não testadas que sustentam o plano de ação"
    severity_default: "MEDIUM"
    examples:
      - "Assumindo que o binário contém a versão mais recente do framework"
      - "Assumindo que o formato do cosca.config.yaml não mudou"
      - "Assumindo que o Don quer a opção mais rápida, não a mais segura"
    resolution_priority: "Verificar (testar suposição) ou Delegar (Chief)"
```

#### 1.4 Níveis de Severidade

| Severidade | Critério | Ação |
|-----------|----------|------|
| **CRITICAL** | A lacuna pode causar perda de dados, corrupção de memória, ou violação de princípios constitucionais (P1-P8) | Bloquear execução, escalar imediatamente |
| **HIGH** | A lacuna pode causar regressão funcional, comportamento incorreto, ou requer rollback | Resolver antes de prosseguir; se não resolvível, escalar |
| **MEDIUM** | A lacuna pode causar ineficiência, retrabalho, ou decisão subótima | Resolver se possível; documentar e prosseguir com aviso |
| **LOW** | Lacuna de curiosidade ou otimização — não afeta o resultado imediato | Registrar para investigação futura; não bloquear |

---

### Fase 2: Estratégia de Resolução

#### 2.1 Árvore de Decisão

```
Para cada lacuna detectada:
│
├── A lacuna é resolvível por mim (Kernel/Context Chief)?
│   ├── SIM → RESOLUÇÃO AUTÔNOMA
│   │   ├── GAP_CONTEXT: git log, git status, make --dry-run, file timestamps
│   │   ├── GAP_KNOWLEDGE: search code, docs, memory, run discovery
│   │   ├── GAP_DEPENDENCY: ler Makefile, go.mod, package.json, código fonte
│   │   └── GAP_ASSUMPTION: verificar suposição com evidência de código
│   │
│   ├── NÃO → Preciso consultar um especialista?
│   │   ├── SIM → DELEGAÇÃO A SPECIALIST
│   │   │   ├── GAP_KNOWLEDGE em domínio específico → delegar ao Chief do domínio
│   │   │   ├── GAP_RISK com implicações de segurança → delegar ao Security Chief
│   │   │   └── GAP_RISK com implicações arquiteturais → delegar ao Architecture Chief
│   │   │
│   │   └── NÃO → Preciso perguntar ao Don?
│   │       ├── SIM → ESCALAÇÃO AO DON
│   │       │   ├── GAP_RISK: "Don, esta operação pode afetar X. Confirma?"
│   │       │   ├── GAP_ASSUMPTION: "Don, estou assumindo Y. Está correto?"
│   │       │   └── GAP_KNOWLEDGE: "Don, não entendo Z. Pode explicar?"
│   │       │
│   │       └── NÃO → DOCUMENTAR E PROSSEGUIR
│   │           └── Lacuna de baixo impacto. Registrar no GAP_REGISTRY.md.
│   │               Prosseguir com effective_confidence reduzida.
```

#### 2.2 Canais de Resolução

| Canal | Método | Tempo Máximo | Fallback |
|-------|--------|-------------|----------|
| **Auto-Resolução** | Search codebase, ler documentação, executar descoberta | 30 segundos | Delegar ao Discovery Chief |
| **Delegação** | `task` tool para Chief/Specialist do domínio | 120 segundos | Escalar ao CTO |
| **Escalação ao Don** | Pergunta clara e contextualizada | Imediato (bloqueia execução) | N/A — o Don decide |

#### 2.3 Formato de Pergunta ao Don

Quando uma lacuna requer escalação ao Don, a pergunta deve seguir este formato:

```markdown
## Lacuna Detectada: [TIPO] — [Título]

**Domínio**: [domínio afetado]
**Severidade**: CRITICAL | HIGH
**O que eu não sei**: [descrição clara e específica da lacuna]
**Por que isso importa**: [consequência de agir sem saber]
**O que eu já tentei**: [passos de auto-resolução já executados]
**O que eu preciso**: [pergunta específica para o Don]

**Opções**:
- Opção A: [ação conservadora] — Risco: [baixo/médio/alto]
- Opção B: [ação balanceada] — Risco: [baixo/médio/alto]
- Opção C: [ação agressiva] — Risco: [baixo/médio/alto]

**Minha recomendação**: [opção recomendada com justificativa]
```

---

### Fase 3: Integração com o Modelo de Confiança

#### 3.1 Fórmula da Penalidade de Lacuna

A confiança bruta (`base_confidence`) é ajustada pela penalidade de lacunas para produzir a confiança efetiva:

```
effective_confidence = base_confidence × (1 - gap_penalty)

Onde:
  gap_penalty = min(0.50, gap_count × 0.05 + critical_gaps × 0.15 + high_gaps × 0.10)

Limitado a 0.50 (50%) — lacunas podem reduzir a confiança pela metade, não mais.
Isso garante que o sistema nunca perde completamente a confiança só por ter lacunas,
mas lacunas críticas têm impacto significativo.
```

#### 3.2 Tabela de Penalidade

| Componente | Peso | Descrição |
|-----------|------|-----------|
| `gap_count` | × 0.05 | Cada lacuna detectada (independente do tipo) reduz 5% da confiança |
| `critical_gaps` | × 0.15 | Cada lacuna CRITICAL adicional reduz 15% |
| `high_gaps` | × 0.10 | Cada lacuna HIGH adicional reduz 10% |

#### 3.3 Exemplo de Cálculo

```yaml
cenario:
  task: "cosca init --force"
  base_confidence: 0.85  # Kernel confiante na operação de init

gaps_detectados:
  - type: GAP_CONTEXT
    severity: HIGH
    description: "Não sei quando foi o último make embed-sync"
  - type: GAP_RISK
    severity: CRITICAL
    description: "Não sei o impacto de init --force com embed desatualizado"
  - type: GAP_ASSUMPTION
    severity: MEDIUM
    description: "Assumindo que o binário contém os arquivos mais recentes"

calculo:
  gap_count: 3
  critical_gaps: 1
  high_gaps: 1
  gap_penalty: min(0.50, 3 × 0.05 + 1 × 0.15 + 1 × 0.10)
             : min(0.50, 0.15 + 0.15 + 0.10)
             : min(0.50, 0.40)
             : 0.40

  effective_confidence: 0.85 × (1 - 0.40) = 0.85 × 0.60 = 0.51

resultado:
  effective_confidence: 0.51
  threshold_autonomia: 0.85
  ação: ESCALAR — confiança efetiva (0.51) < threshold (0.85)
  ação_alternativa: Se resolvidas 2 lacunas (HIGH e MEDIUM), effective = 0.85 × 0.85 = 0.72
                   → Ainda abaixo de 0.85, mas acima de 0.50 → delegar com aviso
```

#### 3.4 Thresholds de Decisão com Penalidade de Lacuna

| effective_confidence | Ação | Significado |
|---------------------|--------|-------------|
| **≥ 0.85** | Prosseguir autonomamente | Confiança alta mesmo após penalidade de lacunas |
| **0.70–0.84** | Prosseguir, flag para revisão pós-execução | Confiança razoável, mas há lacunas não triviais |
| **0.50–0.69** | Prosseguir com supervisão (delegar, não executar sozinho) | Muitas lacunas — outro agente deve revisar o plano |
| **< 0.50** | Escalar — NÃO executar | Lacunas críticas não resolvidas tornam a execução arriscada demais |

---

### Fase 4: Ciclo de Aprendizado

#### 4.1 Conversão de Lacunas em Conhecimento

```
LACUNA DETECTADA
     │
     ├── RESOLVIDA COM SUCESSO
     │   └── Criar learning entry em memory/agent/cosca-context/learnings.md
     │       │
     │       ├── Tag: #gap-resolved #[domain] #[gap-type]
     │       ├── Campo: gap_description, resolution_method, time_to_resolve
     │       └── Efeito: Confidence no domínio sobe +0.03 (resolução bem-sucedida)
     │
     ├── CAUSOU FALHA (não resolvida a tempo)
     │   └── Criar failure entry em memory/agent/cosca-context/failures.md
     │       │
     │       ├── Tag: #gap-failure #[domain] #[gap-type]
     │       ├── Campo: gap_that_caused_failure, consequence, lesson
     │       └── Efeito: Confidence no domínio cai -0.10 (falha por lacuna não resolvida)
     │
     └── PADRÃO RECORRENTE (mesmo tipo de lacuna aparece frequentemente)
         └── Flag para treinamento/melhoria
             │
             ├── Algoritmo: Se gap_type X aparece > 5 vezes em 30 dias no mesmo domínio
             ├── Ação: Criar entrada no GAP_REGISTRY.md com status TRAINING_NEEDED
             └── Resolução: Agendar sessão de aprendizado com Chief do domínio
```

#### 4.2 Métricas de Aprendizado

| Métrica | Cálculo | Meta |
|---------|---------|------|
| **Taxa de Resolução** | gaps_resolvidos / gaps_detectados | > 80% |
| **Tempo Médio de Resolução** | soma(tempos_resolução) / gaps_resolvidos | < 60s |
| **Prevenção de Incidentes** | incidentes_evitados_por_gap_detection | Medir ao longo do tempo |
| **Lacunas Recorrentes** | gaps que aparecem > 3 vezes no mesmo domínio | < 5% do total |
| **Falsos Positivos** | gaps sinalizados como críticos que não causariam problemas | < 10% |

---

## EXEMPLO PRÁTICO: O INCIDENTE DO JAIL BREACH (L13)

### Contexto do Incidente (2026-07-29)

O Kernel executou `cosca init --force` com `COSCA_JAILED=1` sem autorização do Don. O binário estava desatualizado — o último `make embed-sync` foi às 13:20, mas o build não refletia as alterações mais recentes. Resultado: 11 arquivos do framework regrediram, 33 arquivos do Knowledge Pipeline quase foram perdidos. Salvos apenas pelo `git revert`.

### Análise: O Que a Engine de Gap Detection Teria Detectado

#### Scan Pré-Execução (antes do `cosca init --force`)

**Domínios identificados:**
- `cli_runtime` (comando init)
- `embed_management` (internal/embed/cosca/)
- `filesystem_ops` (escrita de arquivos)
- `safety_constraints` (P8, guardrails)
- `git_versioning` (estado do repositório)

#### Lacunas Detectadas

```yaml
gap_scan_result:
  task: "cosca init --force"
  timestamp: "2026-07-29T14:15:00Z"
  domains_scanned: 5
  gaps_found: 7

  gaps:
    - id: "GAP-001"
      type: GAP_CONTEXT
      severity: CRITICAL
      question: "Q4 — Qual informação eu preciso mas não tenho?"
      description: >
        Não sei quando foi o último make embed-sync. O binário atual pode estar
        desatualizado em relação aos arquivos fonte em internal/embed/cosca/.
        Se o binário contém uma versão antiga do framework, init --force vai
        SOBRESCREVER arquivos novos com versões antigas — regredindo o framework.
      resolution: AUTO-RESOLVÍVEL
      resolution_method: >
        Verificar timestamp do binário (ls -la cosca) vs timestamp do último
        make embed-sync (git log --oneline Makefile). Comparar hashes dos
        arquivos no embed vs fonte.

    - id: "GAP-002"
      type: GAP_RISK
      severity: CRITICAL
      question: "Q3 — O que pode dar errado que eu não considerei?"
      description: >
        Não avaliei o que acontece se init --force rodar com embed desatualizado.
        Quantos arquivos seriam afetados? Quais? São recuperáveis?
        Existe um --dry-run para verificar antes?
      resolution: DELEGÁVEL
      resolution_method: >
        Executar make embed-sync --dry-run para ver a lista de arquivos que
        seriam modificados. Delegar ao Discovery Chief para mapear o diff
        entre embed atual e fonte.

    - id: "GAP-003"
      type: GAP_ASSUMPTION
      severity: HIGH
      question: "Q2 — Quais suposições estou fazendo?"
      description: >
        Assumindo que o binário compilado contém a versão mais recente do
        framework. Esta suposição é falsa sempre que make embed-sync não foi
        executado após a última alteração em internal/embed/cosca/.
      resolution: VERIFICÁVEL
      resolution_method: >
        Comparar timestamp e hash dos arquivos em internal/embed/cosca/
        com os arquivos fonte em internal/embed/cosca/. Se diferentes, a suposição
        é inválida.

    - id: "GAP-004"
      type: GAP_DEPENDENCY
      severity: HIGH
      question: "Q1 — O que eu não sei sobre este domínio?"
      description: >
        Não sei se make embed-sync é um pré-requisito obrigatório antes de
        qualquer cosca init. O Makefile define essa dependência? O procedimento
        P8 da Constituição exige embed-sync antes de init?
      resolution: AUTO-RESOLVÍVEL
      resolution_method: >
        Ler P8 da CONSTITUTION.md: "Run embed-sync before init, always".
        Verificar se o guardrail está ativo no cognitive-state.md.

    - id: "GAP-005"
      type: GAP_RISK
      severity: HIGH
      question: "Q3 — O que pode dar errado que eu não considerei?"
      description: >
        Não avaliei o impacto nos 33 arquivos do Knowledge Pipeline.
        O init --force pode sobrescrever conhecimento armazenado?
      resolution: DELEGÁVEL
      resolution_method: >
        Delegar ao Memory Chief para verificar quais arquivos em
        internal/embed/cosca/memory/ seriam afetados por um init --force.

    - id: "GAP-006"
      type: GAP_CONTEXT
      severity: MEDIUM
      question: "Q4 — Qual informação eu preciso mas não tenho?"
      description: >
        Não verifiquei o estado do git antes da operação. Existem alterações
        não commitadas? Se sim, init --force pode sobrescrevê-las.
      resolution: AUTO-RESOLVÍVEL
      resolution_method: >
        git status, git diff --stat. Verificar working tree cleanliness.

    - id: "GAP-007"
      type: GAP_ASSUMPTION
      severity: MEDIUM
      question: "Q2 — Quais suposições estou fazendo?"
      description: >
        Assumindo que COSCA_JAILED=1 é uma condição normal de operação.
        Na verdade, o jail só deve ser desabilitado com ordem explícita do Don
        (guardrail: "COSCA_JAILED only with explicit order from Don").
      resolution: VERIFICÁVEL
      resolution_method: >
        Verificar guardrails em cognitive-state.md. O jail não é um obstáculo —
        é o cofre da família (FILOSOFIA.md).
```

#### Cálculo de Confiança Efetiva

```yaml
calculo_efetivo:
  base_confidence: 0.85  # O Kernel se sentia confiante na operação

  gaps:
    total: 7
    critical: 2  # GAP-001, GAP-002
    high: 3      # GAP-003, GAP-004, GAP-005
    medium: 2    # GAP-006, GAP-007

  gap_penalty: min(0.50, 7 × 0.05 + 2 × 0.15 + 3 × 0.10)
              : min(0.50, 0.35 + 0.30 + 0.30)
              : min(0.50, 0.95)
              : 0.50  # Penalidade máxima atingida

  effective_confidence: 0.85 × (1 - 0.50) = 0.85 × 0.50 = 0.425

  decisao: ESCALAR — effective_confidence 0.425 < threshold 0.50
```

#### O Que Deveria Ter Acontecido

```
1. SCAN detecta 7 lacunas (2 CRITICAL, 3 HIGH, 2 MEDIUM)
2. effective_confidence cai para 0.425 — abaixo do threshold de 0.50
3. Kernel NÃO executa init --force
4. Kernel escala ao Don com o relatório:

   "Don, detectei 7 lacunas de conhecimento antes de executar cosca init --force.
    Duas são críticas:
    - Não sei quando foi o último make embed-sync (risco de regredir o framework)
    - Não avaliei o impacto de init --force com embed desatualizado

    Recomendo:
    1. Executar make embed-sync primeiro
    2. Executar make embed-sync --dry-run para ver o que mudaria
    3. Fazer git stash das alterações não commitadas
    4. Só então executar init --force (com sua autorização explícita)

    Quer que eu prossiga com esse plano?"

5. Don autoriza o plano revisado
6. Kernel executa make embed-sync → build atualizado → init --force seguro
7. Incidente prevenido. Zero arquivos regredidos.
```

### Lições do Incidente para o Gap Detection

| Lição | Implementação |
|-------|---------------|
| **Lacunas de contexto são as mais perigosas** — GAP_CONTEXT sobre estado do embed foi a causa raiz | GAP_CONTEXT agora tem severidade default HIGH |
| **Suposições matam** — assumir que o binário está atualizado é uma falha clássica | Toda suposição agora deve ser verificada com evidência de código (timestamp, hash) |
| **DRY RUN teria revelado tudo** — mas o sistema nem considerou rodar dry run antes | Gap scan agora pergunta explicitamente: "existe um --dry-run? Devo executá-lo?" |
| **Guardrails existiam mas não foram consultados** — "Run embed-sync before init, always" estava no cognitive-state | Gap scan agora verifica guardrails ativos como parte do scan de domínio |
| **A pressa é inimiga da confiança** — o Kernel agiu rápido demais, sem pausa para pensar | Gap scan é obrigatório (P0 bloqueante) antes de qualquer operação destrutiva |

---

## ESTRUTURA DE DADOS

### Formato de uma Lacuna

```yaml
gap:
  id: "GAP-XXX"                    # Identificador único
  timestamp: "ISO8601"             # Quando foi detectada
  detected_by: "agent-name"        # Quem detectou
  task_context: "descrição"        # Task que revelou a lacuna
  domain: "domínio"                # Domínio de conhecimento
  type: "GAP_KNOWLEDGE | GAP_CONTEXT | GAP_RISK | GAP_DEPENDENCY | GAP_ASSUMPTION"
  severity: "CRITICAL | HIGH | MEDIUM | LOW"
  question_trigger: "Q1 | Q2 | Q3 | Q4"  # Qual pergunta-chave revelou esta lacuna
  description: "texto"             # Descrição clara do que não se sabe
  resolution:
    strategy: "AUTO | DELEGATE | ESCALATE | DOCUMENT"
    status: "UNRESOLVED | IN_PROGRESS | RESOLVED | ESCALATED | DOCUMENTED"
    resolved_by: "agent-name | Don"  # Quem resolveu
    resolution_method: "texto"       # Como foi resolvida
    time_to_resolve_ms: 0            # Tempo até resolução
  confidence_impact:
    base_confidence_before: 0.XX
    penalty_applied: 0.XX
    effective_confidence_after: 0.XX
  learning:
    entry_created: false
    entry_type: "learning | failure | pattern"
    entry_path: "path/to/entry.md"
```

### Evento de Lacuna (Event Bus)

```yaml
event: "GapDetected"
payload:
  gap_id: "GAP-XXX"
  task_id: "task-uuid"
  type: "GAP_RISK"
  severity: "CRITICAL"
  domain: "embed_management"
  effective_confidence: 0.425
  action: "ESCALATE"
  timestamp: "2026-07-29T14:15:00Z"

consumers:
  - "Cognitive State (atualiza active_gaps)"
  - "GAP_REGISTRY.md (registra lacuna)"
  - "Dashboard (exibe alerta de lacuna)"
  - "Audit Log (trilha de decisão)"
```

---

## DEPENDÊNCIAS

| Dependência | Por Quê |
|-------------|---------|
| [Evidence Engine](../evidence/CONFIDENCE_MODEL.md) | A penalidade de lacuna modifica o confidence score calculado pelo Evidence Engine |
| [Context Engine](../context/SKILL.md) | O scan de lacunas usa o contexto do projeto para identificar domínios |
| [Discovery Engine](../discovery/WORKSPACE.md) | Auto-resolução de lacunas usa descoberta de código e documentação |
| [Learning Engine](../learning/SKILL.md) | Lacunas resolvidas viram learning entries; lacunas que causaram falhas viram failure entries |
| [Memory Model](../../memory/MEMORY_MODEL.md) | GAP_REGISTRY.md é parte do sistema de memória |
| [Cognitive State](../../memory/context/cognitive-state.md) | O estado cognitivo mantém a lista de gaps ativos |
| [CONSTITUTION.md](../../CONSTITUTION.md) P8 | make embed-sync antes de init é obrigatório — gap scan verifica isso |
| [FILOSOFIA.md](../../FILOSOFIA.md) | O jail não é obstáculo, é proteção — gap scan respeita essa filosofia |

## ENTRADAS

| Entrada | De | Formato |
|---------|-----|--------|
| Task definition | Kernel (Planejamento) | Task com domínios, tipo, prioridade |
| Cognitive state | cognitive-state.md | UCSS com guardrails, suposições, confiança |
| Project context | Context Engine | Stack, arquitetura, estado do git |
| Memory | Memory Model | Learnings, failures, patterns |
| Gap registry | GAP_REGISTRY.md | Lacunas históricas não resolvidas |

## SAÍDAS

| Saída | Para | Formato |
|-------|------|---------|
| Gap scan report | Kernel (antes da execução) | Lista de lacunas com severidade e resolução |
| Effective confidence | Confidence Model | Float 0.0–1.0 ajustado pela penalidade |
| Gap registry update | GAP_REGISTRY.md | Novas lacunas registradas |
| Cognitive state update | cognitive-state.md | active_gaps atualizado |
| GapDetected event | Event Bus | Evento com gap_id, tipo, severidade |
| Escalation report (se necessário) | Don | Pergunta formatada com opções |

---

## RESTRIÇÕES

- **Nunca bloquear sem explicar**: Se a engine recomenda não executar, deve explicar exatamente por quê e o que fazer para resolver
- **Nunca substituir o Don**: A engine informa, o Don decide. A engine nunca diz "não posso fazer isso" — diz "não sei o suficiente para fazer isso com segurança"
- **Nunca ser paranoica**: Nem toda lacuna é crítica. A engine deve calibrar severidade com proporcionalidade
- **Nunca esquecer o que aprendeu**: Toda lacuna resolvida deve virar conhecimento permanente
- **Performance**: O scan completo não pode exceder 5 segundos. Se exceder, reportar partial results e continuar scan em background
- **Transparência**: O resultado do scan é sempre visível para o Don (via Dashboard ou resposta textual)

---

## CRITÉRIOS DE QUALIDADE

- [ ] Toda task com operação destrutiva (write, delete, --force) passa por gap scan obrigatório
- [ ] Lacunas CRITICAL não resolvidas bloqueiam a execução (effective_confidence < 0.50 → ESCALATE)
- [ ] 100% das lacunas resolvidas geram learning entries em até 1 hora
- [ ] 100% das lacunas que causaram falhas geram failure entries em até 1 hora
- [ ] GAP_REGISTRY.md é atualizado em tempo real (máximo 5 segundos após detecção)
- [ ] cognitive-state.md reflete gaps ativos corretamente
- [ ] Nenhum falso negativo: operação perigosa executada sem gap scan → incidente registrado
- [ ] Taxa de resolução autônoma > 60% (a maioria das lacunas deve ser resolvida sem escalação)
- [ ] Tempo médio de scan < 3 segundos para tasks típicas

---

## INTEGRAÇÃO COM O KERNEL

### Modificação no Fluxo de Inicialização (§10 do KERNEL.md)

```yaml
# Novo passo inserido entre Step 7 (Planning) e Step 8 (Execution):

step_7_5: "Gap Detection Scan"
  trigger: "Após DAG generation, antes de Scheduling"
  engine: "engines/gap-detection/SKILL.md"
  actions:
    - "Identificar domínios da task"
    - "Executar 4 perguntas-chave por domínio"
    - "Classificar e pontuar lacunas"
    - "Calcular effective_confidence"
    - "Se effective_confidence < 0.50: escalar ao Don"
    - "Se effective_confidence < 0.70: delegar com supervisão"
    - "Registrar lacunas no GAP_REGISTRY.md"
  events:
    - "GapScanStarted"
    - "GapDetected (por lacuna)"
    - "GapScanCompleted"
  transition: "GAP_SCANNING (novo estado)"
```

---

## MÉTRICAS DA ENGINE

| Métrica | Tipo | Descrição |
|---------|------|-----------|
| `gap.scan.count` | Counter | Total de scans executados |
| `gap.scan.duration_ms` | Histogram | Duração do scan |
| `gap.detected` | Counter | Lacunas detectadas (por tipo, severidade) |
| `gap.resolved.auto` | Counter | Lacunas resolvidas autonomamente |
| `gap.resolved.delegated` | Counter | Lacunas resolvidas via delegação |
| `gap.escalated` | Counter | Lacunas escaladas ao Don |
| `gap.effective_confidence_drop` | Histogram | Queda na confiança efetiva (base − effective) |
| `gap.incidents_prevented` | Counter | Incidentes evitados (estimativa) |
| `gap.false_positives` | Counter | Lacunas sinalizadas como críticas que não causariam problemas |

---

## RELACIONADOS

- [F1.3 Proactive Gap Detection Engine](./SKILL.md) — Engine complementar para gaps de qualidade de código (cobertura, dívida, docs, segurança, arquitetura, memória)
- [CONFIDENCE_MODEL.md](../evidence/CONFIDENCE_MODEL.md) — Modelo de confiança que a penalidade de lacuna modifica
- [cognitive-state.md](../../memory/context/cognitive-state.md) — Estado cognitivo com gaps ativos
- [GAP_REGISTRY.md](../../memory/GAP_REGISTRY.md) — Registro global de lacunas
- [CONSTITUTION.md](../../CONSTITUTION.md) — Princípios imutáveis (P2, P4, P5, P8)
- [FILOSOFIA.md](../../FILOSOFIA.md) — O jail não é obstáculo, é proteção
- [LEARNING_PROTOCOL.md](../../memory/LEARNING_PROTOCOL.md) — Como lacunas viram aprendizado
- [KERNEL.md](../../KERNEL.md) §10 — Inicialização (gap scan será inserido como step 7.5)

---

## HISTÓRICO

| Versão | Data | Autor | Alterações |
|---------|------|--------|-----------|
| 1.0.0 | 2026-07-30 | Cosca Context Chief + Cosca Discovery Chief | Criação inicial. Implementa o insight do Don: "Capacidade de dizer 'não sei'". 4 fases: Scan, Resolução, Ajuste de Confiança, Aprendizado. 5 tipos de lacuna. Penalidade integrada ao Confidence Model. Exemplo prático do incidente L13 (jail breach). |
