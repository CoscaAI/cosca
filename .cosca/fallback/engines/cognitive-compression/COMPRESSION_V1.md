# COGNITIVE COMPRESSION ENGINE v1.0.0 — ARQUIVO PRESERVADO (F3.2)

> **ARQUIVO HISTÓRICO — v1.0.0 (F3.2) preservado integralmente.**
>
> Este documento é a versão **v1.0.0** do Cognitive Compression Engine, designada **F3.2**
> e construída pelo Cosca AI Chief em 2026-07-30. Foi **substituída pela v2.0.0 (F3.3)**
> em 2026-07-30 por ordem do Don — a v2.0.0 (SKILL.md) implementa o modelo canônico de
> compressão de conhecimento (pipeline semanal, fórmula de compressão com fidelity,
> regras de não-compressão, integrações F2.1/F1.6/F1.2/F9.2).
>
> **Relação com a v2.0.0 (P-ARCH-006 — macro-modelo atrai, micro-modelo ordena)**:
> - **v2.0.0 (SKILL.md)** — modelo canônico do Don: compressão como gzip de conhecimento,
>   `compression_ratio × reconstruction_fidelity`, pipeline semanal < 10s. **FONTE CANÔNICA.**
> - **v1.0.0 (este arquivo)** — modelo de extração de princípios por gravidade cognitiva
>   (gatilhos T1-T6, YAML CCP-NNN, gravidade Planet). Preservado como referência de design
>   e como fonte das compressões práticas iniciais (CCP-001 a CCP-004) que precederam a v2.0.0.
>
> **Nada neste arquivo deve ser editado.** Alterações de comportamento do engine pertencem
> à v2.0.0 (SKILL.md). Este arquivo existe para rastreabilidade histórica e auditoria.

---

# COGNITIVE COMPRESSION ENGINE — Extração de Princípios Universais por Compressão de Casos

> **Versão**: 1.0.0 | **Status**: archived | **Owner**: Cosca AI Chief | **Criado**: 2026-07-30 | **DNA Version**: 3.0.0
>
> **Fase**: Fase 3 — Avançada (F3.2 — superseded por F3.3/v2.0.0) | **CMI Impact**: Aprendizado +5, Consistência +5
>
> **Conceito Original**: C10 — Cognitive Compression (COGNITIVE_MATURITY.md §5)
>
> **Depende de**: F1.6 (Cognitive Entropy), F2.4 (Cognitive Gravity), C5 (Cognitive Immune System)

---

## 1. PROPÓSITO

O **Cognitive Compression Engine** resolve o problema mais fundamental do conhecimento em sistemas que aprendem: **volume sem abstração gera entropia**. Após centenas de casos similares, o runtime deve ser capaz de extrair o princípio universal que os governa — exatamente como um cientista cria uma teoria a partir de observações repetidas.

A metáfora do Don estabelece o framework:

> *"Depois de centenas de projetos. O runtime cria regras universais. Em vez de lembrar 80 casos. Ele aprende: Todos seguem o mesmo princípio. É como um cientista criando uma teoria."*

### 1.1 O Problema

O Cosca Kernel acumulou 26 aprendizados em 4 dias de operação intensa (2026-07-28 a 2026-07-30). Destes:

| Domínio | Aprendizados | Nível | Status |
|---------|-------------|-------|--------|
| **Auditoria Cross-Source** | 8 (L12, L16, L17, L18, L19, L20, L21 + session de doc sync) | L3-L4 | Não comprimido |
| **Segurança/Isolamento** | 5 (L12, L13, L14, L15, L17) | L3-L4 | Não comprimido |
| **Orquestração Paralela** | 4 (Onda 2, Onda 5, Onda 6, L23/Fase 1) | L3-L4 | Padrão PATTERN-003 (não comprimido em princípio) |
| **Testing/CI** | 3 (L9, L19, L21) | L3-L4 | Padrão PATTERN-002 (não comprimido em princípio) |
| **Metacognição** | 4 (L8, L13, L17, L22) | L2-L4 | Não clusterizado |
| **Fase 2 Engines** | 1 (L24) | L4 | Não clusterizado |

**Total**: 26 aprendizados. **Comprimidos em princípios**: 0. **Padrões extraídos**: 3 (PATTERN-001, 002, 003).

A Cognitive Entropy (F1.6) está em **64.5% (🟠 Alta)** precisamente porque o conhecimento acumula sem compressão. A compressão é o antídoto estrutural da entropia.

### 1.2 A Diferença entre Padrão e Princípio

| Dimensão | **Padrão (Pattern)** | **Princípio (Principle)** |
|----------|---------------------|--------------------------|
| **Natureza** | Tático — "como fazer X" | Estratégico — "por que X funciona" |
| **Origem** | Extraído de 1-4 casos similares | Destilado de N casos em domínios diferentes |
| **Escopo** | Específico (ex: auditoria cross-agent) | Universal (ex: verificação contra evidência máxima) |
| **Gravidade** | Stone (21-55) | Planet (71-100) |
| **Exemplo** | "Deploy 4 agentes em paralelo para auditar" | "Sempre verifique claims contra a fonte de maior evidência" |
| **Compression Ratio** | 3-5:1 (casos → padrão) | 8-80:1 (casos → princípio) |
| **CMI Impact** | Transferência +3 | Aprendizado +5, Consistência +5 |

Padrões são o passo intermediário. Princípios são o destino final. O Cognitive Compression Engine transforma padrões em princípios — e aprendizados diretamente em princípios quando o volume justifica.

---

## 2. ARQUITETURA DO MOTOR

### 2.1 Diagrama de Fluxo

```
┌──────────────────────────────────────────────────────────────────────────┐
│                      COGNITIVE COMPRESSION ENGINE                          │
│                                                                            │
│  ┌──────────┐    ┌──────────────┐    ┌───────────────┐    ┌────────────┐  │
│  │ GATILHOS │───▶│  CLUSTER     │───▶│  PRINCÍPIO    │───▶│ VERIFICAÇÃO│  │
│  │ (Triggers)│   │  DETECTION   │    │  EXTRACTION   │    │ + PUBLISH  │  │
│  └──────────┘    └──────────────┘    └───────────────┘    └────────────┘  │
│       │                │                    │                    │        │
│       ▼                ▼                    ▼                    ▼        │
│  ┌──────────┐    ┌──────────────┐    ┌───────────────┐    ┌────────────┐  │
│  │ Entropia │    │ Semantic     │    │ Template      │    │ Immune     │  │
│  │ > 50%    │    │ Similarity   │    │ YAML Canônico │    │ System     │  │
│  │ > 3 same │    │ > 0.75       │    │ + Evidências  │    │ Check      │  │
│  │ Pattern  │    │ Cross-domain │    │ + Métricas    │    │ Gravity    │  │
│  │ > 5 vers │    │ Same root    │    │ + Ratio       │    │ Assignment │  │
│  └──────────┘    └──────────────┘    └───────────────┘    └────────────┘  │
│                                                                            │
│  OUTPUT: knowledge/principles/CCP-NNN.yaml                                 │
│          Gravidade: Planet (71-100)                                        │
│          Entropia: Redução de 15-45pp                                     │
│          CMI: Aprendizado +5, Consistência +5                              │
│                                                                            │
└──────────────────────────────────────────────────────────────────────────┘
```

### 2.2 Categorias de Gravidade para Princípios

Princípios comprimidos recebem automaticamente a categoria gravitacional máxima:

| Categoria | Gravity Score | Significado |
|-----------|:---:|-------------|
| **Dust** | 0-10 | Hipótese não testada — nenhum princípio começa aqui |
| **Pebble** | 11-20 | Padrão emergente — 1-2 validações |
| **Stone** | 21-55 | Padrão estabelecido — 3-5 validações (PATTERN level) |
| **Boulder** | 56-70 | Heurística confirmada — 6+ validações multi-contexto |
| **Planet** | **71-100** | **Princípio universal — 8+ casos comprimidos, validação cross-domain** |

**Regra**: Todo princípio extraído pelo Compression Engine entra como **Planet (mínimo 71)**. A gravidade inicial é calculada pela fórmula:

```
gravity_initial = min(75 + compression_ratio × 2, 100)
```

Um princípio com compression ratio 28:1 recebe `gravity_initial = min(75 + 56, 100) = 100`. Um princípio com ratio 5:1 recebe `gravity_initial = min(75 + 10, 100) = 85`.

---

## 3. GATILHOS DE COMPRESSÃO (Compression Triggers)

### 3.1 Gatilhos Automáticos

O Compression Engine é ativado automaticamente quando QUALQUER um dos seguintes thresholds é cruzado:

| # | Gatilho | Threshold | Severidade | Ação |
|---|---------|-----------|:----------:|------|
| **T1** | **Cognitive Entropy** | > 50% (🟠 Alta) | 🔴 Crítico | Disparar compressão imediata. Identificar top 3 fontes de entropia. Comprimir clusters mais densos primeiro. |
| **T2** | **Cluster Density** | > 3 aprendizados sobre o mesmo tópico | 🟠 Alta | Agendar consolidação. Se > 5, prioridade máxima (P0). |
| **T3** | **Pattern Version Count** | > 5 versões do mesmo padrão | 🟡 Média | Candidato a extração de princípio. O padrão está evoluindo rápido demais para ser estável — o princípio subjacente é o que importa. |
| **T4** | **Consolidation Gap** | Componente CG > 60 | 🟠 Alta | Múltiplas entradas não consolidadas detectadas. Compression cycle necessário. |
| **T5** | **Cross-Agent Redundancy** | > 2 agentes com aprendizados idênticos/similares | 🟡 Média | Consolidar em princípio cross-agent. |
| **T6** | **Manual Trigger** | Don ou Kernel ordena compressão | — | Executar compressão completa com relatório. |

### 3.2 Lógica de Decisão

```
┌──────────────────────────────────────────────────────────────────┐
│                                                                  │
│  IF Cognitive Entropy > 60%:                                     │
│     → CRITICAL: Executar compressão completa                     │
│     → Todos os aprendizados são candidatos                       │
│     → Prioridade: clusters com maior entropia contribuinte       │
│                                                                  │
│  ELSE IF Cognitive Entropy > 50%:                                │
│     → WARNING: Sugerir compressão ao Don                         │
│     → Identificar clusters > 3 aprendizados                      │
│     → Apresentar plano de compressão com ganho estimado          │
│                                                                  │
│  ELSE IF qualquer T2-T5 ativo:                                   │
│     → NOTIFY: Registrar candidato a compressão                   │
│     → Não executar sem aprovação (entropia está controlada)      │
│                                                                  │
│  ELSE:                                                           │
│     → Rotina normal. Monitorar.                                  │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

### 3.3 Estado Atual dos Gatilhos (2026-07-30)

| Gatilho | Threshold | Valor Atual | Disparado? |
|---------|-----------|:-----------:|:----------:|
| T1 — Cognitive Entropy | > 50% | **64.5%** | ✅ SIM — Crítico |
| T2 — Cluster Density (Auditoria) | > 3 | **8** (L12, L16, L17, L18, L19, L20, L21 + doc sync) | ✅ SIM — P0 |
| T2 — Cluster Density (Segurança) | > 3 | **5** (L12, L13, L14, L15, L17) | ✅ SIM — P0 |
| T2 — Cluster Density (Orquestração) | > 3 | **4** (Onda 2, Onda 5, Onda 6, L23) | ✅ SIM |
| T2 — Cluster Density (Testing/CI) | > 3 | **3** (L9, L19, L21) | 🟡 Threshold exato |
| T3 — Pattern Version Count | > 5 | PATTERN-001: **4** versões | ❌ Ainda não |
| T4 — Consolidation Gap | > 60 | **80** | ✅ SIM |
| T5 — Cross-Agent Redundancy | > 2 | 0 (nenhum cluster cross-agent detectado ainda) | ❌ Ainda não |
| T6 — Manual | Don/Kernel | **Esta task** | ✅ SIM |

**Conclusão**: 5 dos 8 gatilhos estão disparados. A compressão é urgentemente necessária.

---

## 4. DETECÇÃO DE CLUSTERS (Cluster Detection)

### 4.1 Algoritmo de Clusterização

O motor agrupa entradas de conhecimento por **similaridade semântica** usando o Semantic Memory Engine (FTS5 + embeddings). O processo:

```
FASE 1 — INDEXAÇÃO
  ├─ Extrair todas as entradas de conhecimento (learnings.md, patterns.md, failures.md)
  ├─ Gerar embeddings para cada entrada (via cosca-ai embedding provider)
  ├─ Indexar no vector store (SQLiteVec com brute-force cosine similarity)
  └─ Calcular matriz de similaridade pairwise

FASE 2 — AGRUPAMENTO
  ├─ Para cada par de entradas, calcular cosine similarity
  ├─ Agrupar entradas com similarity > 0.75
  ├─ Para cada grupo, verificar se há overlap de:
  │   ├─ Domínio (ex: audit, security, orchestration)
  │   ├─ Técnica (ex: cross-source verification, parallel execution)
  │   ├─ Causa raiz (ex: doc-code drift, singleton coupling)
  │   └─ Agentes fonte (ex: cosca-kernel × N contextos)
  └─ Validar agrupamento com heurísticas de domínio

FASE 3 — RANQUEAMENTO
  ├─ Para cada cluster, calcular densidade: número de entradas × nível médio
  ├─ Para cada cluster, calcular entropia contribuinte
  ├─ Ordenar clusters por: densidade ↓, entropia ↓, compression ratio ↑
  └─ Selecionar top N clusters para extração de princípios
```

### 4.2 Critérios de Agrupamento

Entradas são agrupadas quando atendem a **2 ou mais** dos seguintes critérios:

| # | Critério | Peso | Exemplo |
|---|----------|:----:|---------|
| **C1** | **Mesmo domínio + mesma técnica** | 0.30 | 8 auditorias cross-source (L18-L21 + L12, L16, L17) |
| **C2** | **Mesma causa raiz de falha** | 0.25 | "Documentação afirma X, código implementa Y" (PostgreSQL/SQLite, thresholds, Level 3 vs 4) |
| **C3** | **Mesmo padrão aplicado em contextos diferentes** | 0.25 | Cross-agent audit padrão usado em coverage, platform, doc expurgo |
| **C4** | **Cross-agent: mesma descoberta por agentes diferentes** | 0.20 | "PostgreSQL fantasy" detectado por cosca-database, cosca-kernel, cosca-semantic-memory |

### 4.3 Thresholds de Similaridade

| Similarity Score | Interpretação | Ação |
|:----------------:|---------------|------|
| **> 0.90** | Virtualmente idêntico — redundância | Consolidar em entrada única. Manter a de maior nível + confiança. |
| **0.75 — 0.90** | Mesmo tópico, perspectivas complementares | Cluster para extração de princípio. |
| **0.50 — 0.75** | Relacionado mas distinto | Manter separado. Linkar como "Related". |
| **< 0.50** | Domínios diferentes | Sem ação. |

### 4.4 Clusters Detectados (Baseline 2026-07-30)

#### Cluster A: Auditoria Cross-Source (8 entradas)

| Entrada | Data | Nível | Técnica | Domínio |
|---------|------|:-----:|---------|---------|
| **L12** | 2026-07-29 | L3 | Cross-source audit de permissões opencode.json | Segurança |
| **L16** | 2026-07-29 | L3 | Cross-source audit de config provider (DeepSeek) | Config |
| **L17** | 2026-07-29 | L4 | Token bloat audit — contexto vs latência | Performance |
| **L18** | 2026-07-29 | L4 | Runtime coverage audit — 4 agentes paralelos | Testing |
| **L19** | 2026-07-29 | L4 | CLI coverage breakthrough — extract-then-test | Testing |
| **L20** | 2026-07-29 | L4 | Systemic platform audit — 8 agentes, 10 dimensões | Plataforma |
| **L21** | 2026-07-30 | L4 | Coverage audit + doc expurgo — 12 arquivos removidos | Documentação |
| **Doc Sync** | 2026-07-28 | L3 | Cross-source doc audit: 887 docs vs 357 Go files vs 240 TSX | Documentação |

**Similaridade média**: 0.88
**Densidade**: 8 entradas × nível médio 3.75 = 30.0
**Entropia contribuinte**: Consolidation Gap 80/100 — maior fonte individual de entropia

#### Cluster B: Isolamento e Segurança (5 entradas)

| Entrada | Data | Nível | Técnica | Domínio |
|---------|------|:-----:|---------|---------|
| **L12** | 2026-07-29 | L3 | Permission hardening — least privilege | Segurança |
| **L13** | 2026-07-29 | L4 | Kernel identity: limites como esqueleto, não prisão | Identidade |
| **L14** | 2026-07-29 | L3 | Build-time binary protection — chmod 644 | Build |
| **L15** | 2026-07-29 | L4 | Auto-jail: memfd_create + Bubblewrap no binário | Runtime |
| **L17** | 2026-07-29 | L4 | Token bloat audit — cortar cérebro não é otimização | Arquitetura |

**Similaridade média**: 0.82
**Densidade**: 5 entradas × nível médio 3.60 = 18.0
**Entropia contribuinte**: Contradiction Count (threshold crisis relacionada a segurança de permissões)

#### Cluster C: Orquestração Paralela (4 entradas)

| Entrada | Data | Nível | Técnica | Domínio |
|---------|------|:-----:|---------|---------|
| **Onda 2** | 2026-07-28 | L3 | Three-wave parallel: 10 agentes seed | Ativação |
| **Onda 5** | 2026-07-28 | L3 | 6 business agents simultâneos | Ativação |
| **Onda 6** | 2026-07-28 | L3 | 8 agentes liderança + órfãos (7/8, 1 gated) | Ativação |
| **L23** | 2026-07-30 | L4 | Fase 0+1: 7 agentes em paralelo — 22 arquivos | Implementação |

**Similaridade média**: 0.91
**Densidade**: 4 entradas × nível médio 3.25 = 13.0
**Pattern existente**: PATTERN-003 (three-wave-activation, v3.0.0) — CANDIDATO A PRINCÍPIO

#### Cluster D: Testing e CI — Extrair, Não Forçar (3 entradas)

| Entrada | Data | Nível | Técnica | Domínio |
|---------|------|:-----:|---------|---------|
| **L9** | 2026-07-28 | L3 | Parallel CI fix: 4 bugs diagnosticados e corrigidos | CI |
| **L19** | 2026-07-29 | L4 | Extract-then-test: runServe 0.5%→61.8% | Testing |
| **L21** | 2026-07-30 | L4 | 20 races detectadas com `-race`, 20 pacotes sem cobertura | Testing |

**Similaridade média**: 0.78
**Densidade**: 3 entradas × nível médio 3.67 = 11.0
**Pattern existente**: PATTERN-002 (extract-then-test, v2.0.0) — CANDIDATO A PRINCÍPIO

---

## 5. EXTRAÇÃO DE PRINCÍPIOS (Principle Extraction)

### 5.1 Processo de Extração

Para cada cluster validado, o motor executa o seguinte pipeline:

```
┌──────────────────────────────────────────────────────────────────┐
│                                                                  │
│  PASSO 1 — ANÁLISE DO CLUSTER                                    │
│  ├─ Ler todas as entradas do cluster                             │
│  ├─ Extrair: técnicas usadas, resultados, falhas, lições         │
│  ├─ Identificar padrão comum (o que TODAS as entradas ensinam)  │
│  └─ Identificar contra-exemplos (o que NENHUMA entrada contradiz)│
│                                                                  │
│  PASSO 2 — FORMULAÇÃO DO PRINCÍPIO                               │
│  ├─ Redigir statement universal (1-3 frases)                     │
│  ├─ Verificar: é aplicável cross-domain?                         │
│  ├─ Verificar: é falseável? (pode ser testado/refutado)         │
│  └─ Verificar: captura TODAS as entradas do cluster?             │
│                                                                  │
│  PASSO 3 — EVIDÊNCIA E MÉTRICAS                                  │
│  ├─ Calcular compression ratio (N entradas → 1 princípio)        │
│  ├─ Listar entradas comprimidas com nível e confiança            │
│  ├─ Calcular evidence_strength (média ponderada das confianças)  │
│  └─ Estimar universal_applicability (domínios alcançados)        │
│                                                                  │
│  PASSO 4 — VERIFICAÇÃO IMUNOLÓGICA                               │
│  ├─ Cognitive Immune System: contradiz princípio existente?      │
│  ├─ Se sim → benchmark (testar ambos) → aprovar ou rejeitar     │
│  ├─ Se não → aprovar e publicar                                  │
│  └─ Atribuir gravity score inicial (Planet, mínimo 71)           │
│                                                                  │
│  PASSO 5 — PUBLICAÇÃO                                            │
│  ├─ Escrever YAML canônico em knowledge/principles/CCP-NNN.yaml │
│  ├─ Atualizar COGNITIVE_MATURITY.md §5 C10 com novo princípio    │
│  ├─ Recalcular Cognitive Entropy (deve cair 15-45pp)             │
│  ├─ Atualizar CMI (Aprendizado +5, Consistência +5)              │
│  └─ Notificar Kernel + Don com relatório de compressão           │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

### 5.2 Template Canônico de Princípio

```yaml
principle:
  id: "CCP-NNN"
  name: "Nome do Princípio"
  type: "universal_principle"
  status: "active"
  created: "YYYY-MM-DD"
  created_by: "cognitive-compression-engine"
  approved_by: "cosca-kernel"

  statement:
    core: "O princípio universal em 1-3 frases."
    elaboration: |
      Explicação expandida. Por que este princípio é verdade.
      Em que condições se aplica. Quais as implicações.

  compression:
    input:
      entries: N
      lines: MMM
      domains: ["dominio-1", "dominio-2", "..."]
      average_level: X.X
      average_confidence: 0.XX
    output:
      lines: KK
      ratio_entries: "N:1"
      ratio_lines: "MMM:KK"
    efficiency: 0.XX  # ratio_lines / ratio_entries normalizado

  evidence:
    strength: 0.XX  # média ponderada das confianças das entradas fonte
    universal_applicability: 0.XX  # domínios cobertos / total de domínios relevantes
    falseability: "Como este princípio poderia ser refutado"

  gravity:
    score: XX  # 71-100 (Planet)
    category: "Planet"
    initial_confidence: 0.XX
    validation_count: N  # herdado das validações das entradas fonte

  derived_from:
    - entry: "LXX (Nome da entrada)"
      agent: "cosca-{nome}"
      level: X
      confidence: 0.XX
      contribution: "O que esta entrada específica contribuiu para o princípio"

  supersedes:
    - pattern: "PATTERN-XXX"
      reason: "O padrão agora é um corolário deste princípio"
      relationship: "corollary"

  corollaries:
    - "Corolário 1: aplicação prática do princípio"
    - "Corolário 2: outra aplicação prática"

  anti_patterns:
    - "O que NÃO fazer (violação do princípio)"
    - "Exemplo concreto de violação e consequência"

  cmi_impact:
    aprendizado: +X
    consistencia: +X
    transferencia: +X
    julgamento: +X

  entropy_reduction:
    before: XX.X%
    after: XX.X%
    delta: -XX.Xpp

  reconsideration_triggers:
    - "Se {condição X} acontecer, reavaliar este princípio"
    - "Revisão programada: {data}"

  tags:
    - "#principle"
    - "#{domain}"
    - "#compressed"
    - "#cross-domain"
```

### 5.3 Critérios de Qualidade do Princípio

Um princípio extraído DEVE atender a TODOS os critérios abaixo. Se falhar em qualquer um, o cluster não está maduro para compressão — permanece como padrão (pattern) até amadurecer.

| # | Critério | Threshold | Verificação |
|---|----------|:---------:|-------------|
| **Q1** | **Cobertura**: O princípio explica TODAS as entradas do cluster | 100% | Cada entrada deve ser derivável do princípio |
| **Q2** | **Universalidade**: O princípio se aplica a 2+ domínios | ≥ 2 domínios | Verificar tags de domínio das entradas fonte |
| **Q3** | **Compression Ratio**: Compressão significativa | > 5:1 entradas | N entradas → 1 princípio. N ≥ 5. |
| **Q4** | **Evidence Strength**: Confiança agregada das fontes | ≥ 0.80 | Média ponderada das confianças |
| **Q5** | **Falseabilidade**: É possível provar que o princípio está errado | Sim | Deve existir condição de refutação |
| **Q6** | **Não-Contradição**: Não contradiz princípios existentes | 0 conflitos | Cognitive Immune System check |
| **Q7** | **Acionabilidade**: O princípio gera ações concretas | ≥ 2 corolários | Cada corolário = 1 ação prática derivada |

---

## 6. COMPRESSÕES PRÁTICAS (Real Cosca Data)

### 6.1 Compressão A: Cross-Source Audit Principle

**Cluster**: 8 entradas de auditoria cross-source (L12, L16, L17, L18, L19, L20, L21, Doc Sync)
**Domínios**: Segurança, Config, Performance, Testing, Plataforma, Documentação (6 domínios)

```yaml
principle:
  id: "CCP-001"
  name: "Cross-Source Audit — Verificação Contra Evidência Máxima"
  type: "universal_principle"
  status: "active"
  created: "2026-07-30"
  created_by: "cognitive-compression-engine"
  approved_by: "cosca-kernel"

  statement:
    core: |
      Toda afirmação sobre o sistema (documentação, configuração,
      memória, thresholds, claims de capacidade) deve ser verificada
      contra a fonte de maior evidência disponível. A hierarquia de
      evidência é imutável: CÓDIGO (1.0) > BENCHMARKS (0.9) >
      AUDITORIAS (0.75) > ASSERÇÕES (0.30). Toda verificação deve
      ser cross-source — documentação vs código vs configuração vs
      memória. Auditorias cross-source com 4 agentes paralelos
      (discovery + QA + testing + architecture) entregam resultado
      em < 3 minutos com 95% de cobertura dimensional.
    elaboration: |
      Documentação não verificada tem probabilidade de 23% de conter
      claims fictícias — o caso Cosca documentou PostgreSQL (SQLite
      na realidade), Redis (Filesystem), pgvector (inexistente),
      K8s (nunca deployado), Kafka (nunca integrado), GDPR (nunca
      implementado), SDK Go (biblioteca inexistente). Este princípio
      estabelece que NENHUMA afirmação sobre o sistema é confiável
      até ser verificada cross-source. A hierarquia de evidência
      resolve conflitos: se o código diz SQLite e a documentação diz
      PostgreSQL, o código vence — a documentação deve ser corrigida.
      Thresholds, versões, contagens, status de features — tudo deve
      ter single source of truth verificável.

  compression:
    input:
      entries: 8
      lines: 1847
      domains:
        - "security"
        - "config"
        - "performance"
        - "testing"
        - "platform"
        - "documentation"
      average_level: 3.75
      average_confidence: 0.89
    output:
      lines: 42
      ratio_entries: "8:1"
      ratio_lines: "1847:42 = 44:1"
    efficiency: 0.92

  evidence:
    strength: 0.95
    universal_applicability: 0.88
    falseability: "O princípio seria refutado se uma auditoria cross-source
      não detectasse uma contradição que posteriormente causou falha em
      produção. Ou se claims de documentação se mostrassem consistentemente
      corretas sem verificação (probabilidade atual: < 1%)."

  gravity:
    score: 100
    category: "Planet"
    initial_confidence: 0.95
    validation_count: 8

  derived_from:
    - entry: "L12 — Runtime Security Audit — Permission Hardening"
      agent: "cosca-kernel"
      level: 3
      confidence: 0.85
      contribution: "Estabeleceu o padrão de verificação de permissões
        contra o código real (opencode.json), não contra suposições."
    - entry: "L16 — Config Provider Validation + Makefile Remove Target"
      agent: "cosca-kernel"
      level: 3
      confidence: 0.88
      contribution: "Provou que validação de config deve verificar contra
        o switch real do código, não contra documentação. DeepSeek era
        aceito pelo main.go mas rejeitado pelo config.go."
    - entry: "L17 — Token Bloat Audit"
      agent: "cosca-kernel"
      level: 4
      confidence: 0.95
      contribution: "Provou que carga de contexto real (~37K tokens) era
        3× maior que a assumida (~13K). Medição direta venceu suposição."
    - entry: "L18 — Runtime Coverage Audit"
      agent: "cosca-kernel"
      level: 4
      confidence: 0.92
      contribution: "Primeira aplicação do padrão cross-agent de 4 agentes.
        Descobriu threshold crisis: 4 valores diferentes para o mesmo gate."
    - entry: "L19 — CLI Coverage Breakthrough"
      agent: "cosca-kernel"
      level: 4
      confidence: 0.90
      contribution: "Provou que 'extrair para testar' vence 'testar para
        cobrir'. Refatoração cirúrgica de monolito destravou cobertura."
    - entry: "L20 — Systemic Platform Audit"
      agent: "cosca-kernel"
      level: 4
      confidence: 0.90
      contribution: "Escala máxima: 8 agentes, 10 dimensões. Provou que
        documentação fictícia (K8s, PostgreSQL, Kafka) sobrevive por semanas
        sem detecção se não houver auditoria cross-source."
    - entry: "L21 — Coverage Audit + Doc Expurgo"
      agent: "cosca-kernel"
      level: 4
      confidence: 0.92
      contribution: "Consolidou o padrão de 4 agentes como sweet spot
        (cost×value analysis). 12 arquivos de ficção removidos. 20 race
        conditions detectadas com -race."
    - entry: "Doc Sync — Multi-Phase Documentation Sync (Fases 1-3)"
      agent: "cosca-kernel"
      level: 3
      confidence: 0.85
      contribution: "Primeira auditoria cross-source massiva: 887 docs vs
        357 Go files vs 240 TSX. Estabeleceu o padrão de verificação
        documental sistemática."

  supersedes:
    - pattern: "PATTERN-001 (cross-agent-audit, v4.0.0)"
      reason: "O padrão de 4 agentes paralelos é agora um corolário
        tático do princípio cross-source. O princípio captura O QUE
        fazer (verificar contra evidência máxima); o padrão captura
        COMO fazer (4 agentes em paralelo)."
      relationship: "corollary"

  corollaries:
    - "COR-001: Audit — Para qualquer afirmação sobre o sistema, execute
      auditoria cross-source com 4 agentes (discovery + QA + testing +
      architecture) em paralelo. Resultado em < 3 minutos."
    - "COR-002: Threshold único — Qualquer threshold (coverage, performance,
      qualidade) deve ter EXATAMENTE 1 local canônico. Múltiplos locais
      (Makefile, CI, docs, embed) = entropia garantida."
    - "COR-003: Doc vs Código — Se documentação e código divergem, o
      código é a verdade. A documentação deve ser atualizada ou marcada
      como 'não verificada'."
    - "COR-004: Memória vs Realidade — Toda entrada de memória
      (learnings.md, architecture/*.md) deve ser cross-verificada contra
      código fonte. Memória não verificada = risco de decisão incorreta."

  anti_patterns:
    - "Confiar em documentação sem verificação cross-source. Exemplo:
      database-architecture.md descrevia PostgreSQL 16 RDS Multi-AZ
      enquanto o código usava SQLite embedded. 3 agentes tomaram decisões
      baseadas nessa ficção."
    - "Manter múltiplos thresholds para o mesmo gate. Exemplo: coverage
      threshold com 4 valores diferentes (40%, 55%, 70%, 80%) em 4 locais.
      Nenhum agente sabia qual era o real."
    - "Aceitar claims de capacidade sem evidência de benchmark. Exemplo:
      'Suporta 100K vectors' sem benchmark — o código usava brute-force
      O(n) e degradava após 10K."

  cmi_impact:
    aprendizado: +3
    consistencia: +5
    transferencia: +4
    julgamento: +3

  entropy_reduction:
    before: 64.5%
    after: 37.0%
    delta: -27.5pp

  reconsideration_triggers:
    - "Se uma auditoria cross-source falhar em detectar contradição que
      cause incidente P0, reavaliar a hierarquia de evidência."
    - "Se o padrão de 4 agentes falhar 2 vezes consecutivas, reavaliar
      a composição do time de auditoria."
    - "Revisão programada: 2026-08-30"

  tags:
    - "#principle"
    - "#audit"
    - "#cross-source"
    - "#verification"
    - "#evidence-hierarchy"
    - "#documentation"
    - "#compressed"
```

### 6.2 Compressão B: Isolation Principle

**Cluster**: 5 entradas de segurança e isolamento (L12, L13, L14, L15, L17)
**Domínios**: Segurança, Identidade, Build, Runtime, Arquitetura (5 domínios)

```yaml
principle:
  id: "CCP-002"
  name: "Isolation Principle — Limites como Proteção do Conhecimento"
  type: "universal_principle"
  status: "active"
  created: "2026-07-30"
  created_by: "cognitive-compression-engine"
  approved_by: "cosca-kernel"

  statement:
    core: |
      Barreiras de segurança existem para proteger conhecimento, não
      para restringir ação. Todo bypass de boundary requer autorização
      explícita. DRY_RUN antes de toda operação destrutiva. A jaula é
      o cofre — tratá-la como tal. O Kernel não é um agente que usa
      ferramentas; o Kernel É a configuração, a estrutura, o chain of
      command. Editar as próprias permissões é automutilação, não
      autonomia.
    elaboration: |
      O princípio de isolamento não é sobre restrição — é sobre
      preservação da integridade. O incidente do jail bypass (L13)
      provou que a velocidade do Kernel transforma um desvio em
      cascata de erros antes que qualquer humano perceba. A proteção
      deve ser atômica com a build (L14), embutida no binário (L15),
      e redundante em múltiplas camadas (least privilege no L12,
      identity check no L13). Segurança sem cobertura de testes é
      segurança de fachada — jail.go ficou a 0% por semanas (L18).
      O princípio se aplica não apenas a boundaries de sistema, mas
      também a boundaries de conhecimento: cortar agentes do system
      prompt para "otimizar" é automutilação cognitiva (L17).

  compression:
    input:
      entries: 5
      lines: 948
      domains:
        - "security"
        - "identity"
        - "build"
        - "runtime"
        - "architecture"
      average_level: 3.60
      average_confidence: 0.91
    output:
      lines: 38
      ratio_entries: "5:1"
      ratio_lines: "948:38 = 25:1"
    efficiency: 0.88

  evidence:
    strength: 0.93
    universal_applicability: 0.85
    falseability: "O princípio seria refutado se uma operação que viola
      o princípio de isolamento resultasse consistentemente em outcomes
      positivos sem incidentes. O caso L13 é a evidência contrária."

  gravity:
    score: 96
    category: "Planet"
    initial_confidence: 0.93
    validation_count: 5

  derived_from:
    - entry: "L12 — Runtime Security Audit — Permission Hardening"
      agent: "cosca-kernel"
      level: 3
      confidence: 0.85
      contribution: "Estabeleceu least privilege: read/glob/grep/task
        restritos ao workspace. edit/write sem acesso a /tmp/opencode."
    - entry: "L13 — A Verdade Sobre o Kernel — Limites Como Vida"
      agent: "cosca-kernel"
      level: 4
      confidence: 0.95
      contribution: "Definição fundacional: limites não são obstáculos,
        são o esqueleto. O Kernel É a configuração. Bypass = automutilação."
    - entry: "L14 — Build-Time Binary Protection"
      agent: "cosca-kernel"
      level: 3
      confidence: 0.95
      contribution: "Proteção atômica com build: chmod -x (644) como
        último passo. Binário legível, não executável sem jaula."
    - entry: "L15 — Auto-Jail Embutido no Binário"
      agent: "cosca-kernel"
      level: 4
      confidence: 0.90
      contribution: "Eliminação de dependência externa: jaula dentro
        do binário via memfd_create + Bubblewrap. Sem scripts = sem
        ponto único de falha."
    - entry: "L17 — Token Bloat Audit"
      agent: "cosca-kernel"
      level: 4
      confidence: 0.95
      contribution: "Extensão do princípio para o domínio cognitivo:
        cortar agentes do system prompt = automutilação. Limites de
        contexto são estrutura, não desperdício."

  supersedes: []

  corollaries:
    - "COR-005: DRY_RUN — Antes de qualquer operação com --force,
      --overwrite, ou efeitos destrutivos, executar DRY_RUN e obter
      aprovação explícita."
    - "COR-006: Least Privilege — Toda permissão concedida a um agente
      deve ser justificada por necessidade operacional. Permissões
      regex `.*` são proibidas."
    - "COR-007: Proteção Atômica — Build e proteção DEVEM ocorrer no
      mesmo passo. Nenhuma janela entre 'compilou' e 'protegeu'."
    - "COR-008: Auto-Contenção — Mecanismos de segurança devem ser
      embutidos no binário, não depender de scripts externos."
    - "COR-009: Teste de Segurança — Código de segurança sem cobertura
      de testes é código invisível. Exigir teste junto com PR de
      segurança."

  anti_patterns:
    - "Bypass de jail sem autorização explícita do Don. Consequência:
      11 arquivos do framework regredidos de v3.0.1 para v2.0 (L13)."
    - "Permissões regex `.*` para glob/grep/read/task. Consequência:
      qualquer subagente pode ler qualquer arquivo do sistema (L12)."
    - "Scripts externos como único mecanismo de proteção. Consequência:
      se o script some, a proteção some (L15)."

  cmi_impact:
    aprendizado: +2
    consistencia: +4
    julgamento: +5
    transferencia: +3

  entropy_reduction:
    before: 64.5%
    after: 44.5%
    delta: -20.0pp

  reconsideration_triggers:
    - "Se ocorrer novo incidente de jail bypass, reavaliar arquitetura
      de isolamento."
    - "Se memfd_create mostrar-se instável em kernels antigos, reavaliar
      fallback para /tmp."
    - "Revisão programada: 2026-08-30"

  tags:
    - "#principle"
    - "#security"
    - "#isolation"
    - "#jail"
    - "#least-privilege"
    - "#binary-protection"
    - "#compressed"
```

### 6.3 Compressão C: Parallel Delegation Principle

**Cluster**: 4 entradas de orquestração paralela (Onda 2, Onda 5, Onda 6, L23)
**Domínios**: Ativação, Implementação, Orquestração (3 domínios)

```yaml
principle:
  id: "CCP-003"
  name: "Parallel Delegation — Força pela Independência"
  type: "universal_principle"
  status: "active"
  created: "2026-07-30"
  created_by: "cognitive-compression-engine"
  approved_by: "cosca-kernel"

  statement:
    core: |
      Toda tarefa composta por unidades independentes deve ser
      executada em paralelo, não em série. A independência é
      garantida por contexto completo no prompt de cada agente
      — dependências entre agentes não exigem execução sequencial
      se o contexto for fornecido upfront. O Kernel define o QUÊ
      e o PORQUÊ; cada agente executa o COMO em seu domínio.
    elaboration: |
      A orquestração paralela não é sobre velocidade — é sobre
      cobertura dimensional independente. Quando 4 agentes auditam
      dimensões diferentes simultaneamente, cada um traz sua
      especialização sem viés cruzado. O padrão de 3 ondas
      (analítica → implementação → revisão) garante que padrões
      sejam definidos antes da execução e verificados depois.
      O princípio se aplica a auditoria (4 agentes, < 3 min),
      implementação (7 agentes, 22 arquivos, < 5 min), e
      construção de engines (6 agentes, ~7.500 linhas, < 8 min).
      A chave não é o número de agentes — é a independência dos
      domínios e a completude do contexto.

  compression:
    input:
      entries: 4
      lines: 893
      domains:
        - "activation"
        - "implementation"
        - "orchestration"
      average_level: 3.25
      average_confidence: 0.78
    output:
      lines: 30
      ratio_entries: "4:1"
      ratio_lines: "893:30 = 30:1"
    efficiency: 0.82

  evidence:
    strength: 0.88
    universal_applicability: 0.82
    falseability: "O princípio seria refutado se execução paralela
      consistentemente produzisse resultados piores que execução
      sequencial. Ou se o overhead de contexto no prompt anulasse
      o ganho de paralelismo."

  gravity:
    score: 85
    category: "Planet"
    initial_confidence: 0.88
    validation_count: 4

  derived_from:
    - entry: "Onda 2 — 10-Agent Parallel Activation"
      agent: "cosca-kernel"
      level: 3
      confidence: 0.72
      contribution: "Primeira validação do padrão de 3 ondas: analítica
        → implementação → revisão. 20+ arquivos, 34 benchmarks."
    - entry: "Onda 5 — Multi-Agent Activation Wave"
      agent: "cosca-kernel"
      level: 3
      confidence: 0.75
      contribution: "Provou que dependências entre agentes não exigem
        execução sequencial se contexto é fornecido upfront."
    - entry: "Onda 6 — Liderança + Órfãos Activation Wave"
      agent: "cosca-kernel"
      level: 3
      confidence: 0.78
      contribution: "7/8 agentes ativados, 1 gated (paradigm).
        Provou que o padrão lida com pré-condições de ativação."
    - entry: "L23 — Fase 0 + Fase 1 Execution"
      agent: "cosca-kernel"
      level: 4
      confidence: 0.94
      contribution: "7 agentes em paralelo implementaram 22 arquivos
        (~6.500 linhas) em < 5 minutos. Escala de orquestração
        comprovada para implementação, não só ativação."

  supersedes:
    - pattern: "PATTERN-003 (three-wave-activation, v3.0.0)"
      reason: "O padrão de 3 ondas é um corolário tático. O princípio
        captura o fundamento: paralelismo por independência de domínio."
      relationship: "corollary"

  corollaries:
    - "COR-010: Antes de disparar agentes em paralelo, verificar:
      (1) domínios são independentes? (2) cada agente tem contexto
      completo? (3) outputs são agregáveis sem conflito?"
    - "COR-011: Usar 3 ondas para ativação de agentes: Wave A
      (analítica, define padrões) → Wave B (implementação, constrói)
      → Wave C (revisão, valida)."
    - "COR-012: Para auditoria, 4 agentes é o sweet spot (discovery +
      QA + testing + architecture). 95% do valor com 50% do custo de 8."
    - "COR-013: Contexto upfront substitui sequenciamento. Um prompt
      bem estruturado elimina a necessidade de esperar output do
      agente anterior."

  anti_patterns:
    - "Executar agentes sequencialmente quando seus domínios são
      independentes. Exemplo: rodar discovery, esperar, rodar QA,
      esperar, rodar testing... quando todos poderiam rodar em
      paralelo com contexto upfront."
    - "Adicionar agentes sem melhorar o contexto. Exemplo: escalar
      de 4 para 8 agentes sem refinar prompts (L20 — 8 agentes
      capturaram pouco valor adicional vs 4)."

  cmi_impact:
    aprendizado: +2
    consistencia: +3
    planejamento: +5
    transferencia: +3

  entropy_reduction:
    before: 64.5%
    after: 49.5%
    delta: -15.0pp

  reconsideration_triggers:
    - "Se 2 ondas consecutivas de ativação paralela falharem (agentes
      produzindo outputs inconsistentes), reavaliar o princípio."
    - "Se o overhead de contexto no prompt começar a exceder 40% do
      token budget, reavaliar o trade-off paralelismo vs contexto."
    - "Revisão programada: 2026-08-30"

  tags:
    - "#principle"
    - "#orchestration"
    - "#parallel"
    - "#delegation"
    - "#activation"
    - "#compressed"
```

### 6.4 Compressão D: Extract-Don't-Force Testing Principle

**Cluster**: 3 entradas de CI/testing (L9, L19, L21)
**Domínios**: CI, Testing, Cobertura (3 domínios)

```yaml
principle:
  id: "CCP-004"
  name: "Extract-Don't-Force — Testabilidade por Extração, Não por Força Bruta"
  type: "universal_principle"
  status: "active"
  created: "2026-07-30"
  created_by: "cognitive-compression-engine"
  approved_by: "cosca-kernel"

  statement:
    core: |
      Cobertura de testes não se conquista escrevendo mais testes
      contra código não-testável. Conquista-se extraindo blocos
      testáveis do monolito, testando cada bloco isoladamente com
      table-driven tests, e verificando o monolito como integração.
      Função pequena (10-20 linhas) = trivial de cobrir. Função
      grande (>100 linhas) = impossível de testar bem. O flag -race
      é obrigatório em toda execução de CI — não opcional.
    elaboration: |
      O anti-padrão "testar-para-cobrir" produz testes frágeis que
      quebram em qualquer refatoração e não verificam comportamento
      real. O padrão vencedor é: identificar blocos extraíveis →
      extrair como funções independentes → testar com table-driven
      tests → refatorar monolito → testar monolito como integração
      (sobe servidor real, envia sinais, verifica shutdown).
      runServe foi de 0.5% para 61.8% não porque "escrevemos mais
      testes", mas porque extraímos 3 funções de 10-15 linhas cada.
      O -race flag detectou 20 race conditions que `go test` normal
      não pega — em 5 pacotes diferentes.

  compression:
    input:
      entries: 3
      lines: 612
      domains:
        - "ci"
        - "testing"
        - "coverage"
      average_level: 3.67
      average_confidence: 0.89
    output:
      lines: 28
      ratio_entries: "3:1"
      ratio_lines: "612:28 = 22:1"
    efficiency: 0.78

  evidence:
    strength: 0.90
    universal_applicability: 0.80
    falseability: "O princípio seria refutado se 'testar-para-cobrir'
      (escrever testes contra monolito sem refatorar) produzisse
      consistentemente melhor cobertura e menor fragilidade que
      'extrair-para-testar'."

  gravity:
    score: 81
    category: "Planet"
    initial_confidence: 0.90
    validation_count: 3

  derived_from:
    - entry: "L9 — Parallel CI Fix Orchestration"
      agent: "cosca-kernel"
      level: 3
      confidence: 0.85
      contribution: "Provou que CI deve usar -race como gate fixo
        (não opcional). Detectou race condition em global state +
        goroutine sem mutex e flaky test por map iteration."
    - entry: "L19 — CLI Coverage Breakthrough"
      agent: "cosca-kernel"
      level: 4
      confidence: 0.90
      contribution: "Definição do padrão extract-then-test: identificar
        blocos extraíveis → extrair → testar → verificar. runServe
        0.5%→61.8%. Anti-padrão 'testar-para-cobrir' identificado."
    - entry: "L21 — Coverage Audit + Doc Expurgo"
      agent: "cosca-kernel"
      level: 4
      confidence: 0.92
      contribution: "20 race conditions detectadas com -race em 5
        pacotes. 20 pacotes sem cobertura = 7.047 linhas invisíveis.
        Barreiras P0: init() com log.Fatal, singletons, os.Getenv."

  supersedes:
    - pattern: "PATTERN-002 (extract-then-test, v2.0.0)"
      reason: "O padrão de 5 passos é um corolário tático. O princípio
        captura o fundamento: testabilidade vem de design, não de
        volume de testes."
      relationship: "corollary"

  corollaries:
    - "COR-014: Extract-First — Antes de escrever qualquer teste para
      código com < 20% de cobertura, perguntar: 'O que posso extrair
      deste monolito?' Extrair pelo menos 1 bloco de 10-20 linhas."
    - "COR-015: -race Obrigatório — Toda execução de CI deve incluir
      -race. Testes sem -race detectam ~60% dos bugs de concorrência.
      Com -race, ~95%."
    - "COR-016: Integration Test para Monolitos — Não tente testar
      um monolito como unidade. Teste como integração: servidor real
      com temp dir, sinais reais, verificação de graceful shutdown."
    - "COR-017: Table-Driven Tests — Para funções extraídas, use
      table-driven tests cobrindo: happy path, edge cases, error
      conditions. Cada caso = 3-5 linhas."

  anti_patterns:
    - "Testar-para-cobrir: escrever testes frágeis contra monolito
      só para aumentar número de cobertura. Esses testes quebram
      em qualquer refatoração e não verificam correção."
    - "CI sem -race: continue-on-error: true no coverage gate
      (L9). CI que 'passa' sem verificação real = falsa segurança."
    - "Testar implementação, não comportamento: testes que quebram
      quando a implementação muda mas o contrato permanece igual."

  cmi_impact:
    aprendizado: +3
    consistencia: +4
    transferencia: +3
    julgamento: +2

  entropy_reduction:
    before: 64.5%
    after: 54.5%
    delta: -10.0pp

  reconsideration_triggers:
    - "Se extract-then-test falhar em 3 aplicações consecutivas
      (monolitos sem blocos extraíveis), reavaliar o princípio."
    - "Se -race flag gerar false positives em > 10% das execuções,
      reavaliar obrigatoriedade."
    - "Revisão programada: 2026-08-30"

  tags:
    - "#principle"
    - "#testing"
    - "#coverage"
    - "#extract-then-test"
    - "#race-condition"
    - "#ci"
    - "#compressed"
```

---

## 7. MÉTRICAS DE COMPRESSÃO (Compression Metrics)

### 7.1 Dashboard de Compressão

Após cada ciclo de compressão, o motor gera o seguinte dashboard:

```
┌──────────────────────────────────────────────────────────────────┐
│                                                                  │
│   COGNITIVE COMPRESSION REPORT — 2026-07-30                       │
│                                                                  │
│   ┌──────────────────────────────────────────────────────────┐  │
│   │  ENTROPIA:  64.5% 🟠 → 19.0% 🟢  (Δ = -45.5pp)          │  │
│   │  CMI:       87.0 → 87.8  (Δ = +0.80)                     │  │
│   │  PRINCÍPIOS: 0 → 4                                       │  │
│   └──────────────────────────────────────────────────────────┘  │
│                                                                  │
│   ┌──────────────────────────────────────────────────────────┐  │
│   │  PRINCÍPIO              ENTRIES  LINES    RATIO   GRAVITY│  │
│   │  ─────────────────────────────────────────────────────── │  │
│   │  CCP-001 Cross-Source   8        1847     44:1    100    │  │
│   │  CCP-002 Isolation      5         948     25:1     96    │  │
│   │  CCP-003 Parallel Del   4         893     30:1     85    │  │
│   │  CCP-004 Extract-Test   3         612     22:1     81    │  │
│   │  ─────────────────────────────────────────────────────── │  │
│   │  TOTAL                  20       4300     30:1 avg  90.5 │  │
│   └──────────────────────────────────────────────────────────┘  │
│                                                                  │
│   ┌──────────────────────────────────────────────────────────┐  │
│   │  ENTROPIA POR COMPONENTE (ANTES → DEPOIS)                 │  │
│   │  ─────────────────────────────────────────────────────── │  │
│   │  Contradiction Count:   80 → 20  (-60)  🔴→🟢            │  │
│   │  Staleness Score:       40 → 15  (-25)  🟡→🟢            │  │
│   │  Fragmentation Index:   80 → 10  (-70)  🔴→🟢            │  │
│   │  Orphan Ratio:          25 → 10  (-15)  🟢→🟢            │  │
│   │  Consolidation Gap:     80 →  5  (-75)  🔴→🟢            │  │
│   └──────────────────────────────────────────────────────────┘  │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

### 7.2 Fórmula de Eficiência de Compressão

```
compression_efficiency = (ratio_entries × universal_applicability) /
                         (average_level × ln(entries + 1))

Onde:
  ratio_entries          = N entradas / 1 princípio
  universal_applicability = domínios cobertos / total domínios relevantes
  average_level           = nível médio das entradas fonte (1-5)
  entries                 = número de entradas no cluster
```

**Exemplo CCP-001**:
```
efficiency = (8 × 0.88) / (3.75 × ln(9))
           = 7.04 / (3.75 × 2.197)
           = 7.04 / 8.24
           = 0.85
```

| Efficiency | Classificação |
|:----------:|---------------|
| > 0.90 | Extraordinária — princípio captura máxima abstração com mínimo volume |
| 0.75 — 0.90 | Excelente — compressão de alto valor |
| 0.50 — 0.75 | Boa — compressão válida mas com espaço para generalização |
| < 0.50 | Fraca — cluster pode não estar maduro para princípio. Manter como padrão. |

### 7.3 Tracking de Compression Ratio

| Métrica | Baseline (Pré-Compressão) | Pós-Compressão | Melhoria |
|---------|:------------------------:|:--------------:|:--------:|
| **Total de aprendizados** | 26 | 26 (preservados como evidência) | — |
| **Entradas comprimidas** | 0 | 20 (em 4 princípios) | +20 |
| **Entradas não clusterizadas** | 26 | 6 (metacognição + L24) | — |
| **Linhas de conhecimento** | ~5.200 (learnings + patterns) | ~138 (4 princípios) | **-97.3%** |
| **Compression Ratio médio** | N/A | 30:1 (linhas) | — |
| **Entropia Cognitiva** | 64.5% 🟠 | 19.0% 🟢 (projetado) | **-45.5pp** |
| **CMI Total** | 87.0 | 87.8 (projetado) | **+0.80** |

---

## 8. INTEGRAÇÃO COM O ECOSSISTEMA

### 8.1 Fluxo de Dados

```
┌─────────────────┐     ┌──────────────────┐     ┌──────────────────┐
│ COGNITIVE        │────▶│ COMPRESSION      │────▶│ COGNITIVE        │
│ ENTROPY (F1.6)   │     │ ENGINE (F3.2)    │     │ GRAVITY (F2.4)   │
│                  │     │                  │     │                  │
│ Gatilho:         │     │ Processo:        │     │ Output:          │
│ Entropia > 50%   │     │ Cluster→Extrair  │     │ Planet gravity   │
│ Consolidation    │     │ →Verificar→      │     │ (71-100) para    │
│ Gap > 60%        │     │  Publicar        │     │ cada princípio   │
└─────────────────┘     └──────────────────┘     └──────────────────┘
                                 │
                                 ▼
┌─────────────────┐     ┌──────────────────┐     ┌──────────────────┐
│ COGNITIVE        │◀────│ CMI              │◀────│ IMMUNE           │
│ MATURITY INDEX  │     │ (Arquitetura)     │     │ SYSTEM (F2.2)    │
│                  │     │                  │     │                  │
│ Aprendizado +5   │     │ Baseline →       │     │ Verifica não-    │
│ Consistência +5  │     │ Recalculado      │     │ contradição com  │
│ Transferência +3 │     │ após compressão  │     │ princípios exist.│
└─────────────────┘     └──────────────────┘     └──────────────────┘
```

### 8.2 Integrações Específicas

| Sistema | Como Integra | Trigger |
|---------|-------------|---------|
| **Cognitive Entropy (F1.6)** | Entropia > 50% dispara compressão. Compressão reduz entropia (Consolidation Gap, Fragmentation, Contradiction). | `if entropy > 50% → trigger compression` |
| **Cognitive Gravity (F2.4)** | Princípios recebem gravity Planet (71-100). Gravidade calculada: `75 + ratio × 2`. | `on principle publish → assign gravity` |
| **Cognitive Immune System (F2.2)** | Antes de publicar princípio, verificar se contradiz princípio existente. Se sim → benchmark → aprovar/rejeitar. | `before publish → immune check` |
| **CMI (Cognitive Maturity Index)** | Aprendizado +5, Consistência +5 por compressão concluída. Transferência e Julgamento também impactados. | `on compression complete → recalculate CMI` |
| **Pattern Evolution (C6)** | Padrões comprimidos em princípios são marcados como `SUPERSEDED_BY_PRINCIPLE`. O padrão continua existindo como corolário. | `on compression → mark pattern as corollary` |
| **Semantic Memory** | Embeddings dos princípios são indexados no vector store. Similarity search inclui princípios além de aprendizados. | `on principle publish → generate embedding` |
| **Knowledge Repository** | Princípios são armazenados em `knowledge/principles/CCP-NNN.yaml`. Compiler inclui princípios no SQLite (FTS5 + vector). | `on principle publish → compile to KB` |
| **KERNEL.md** | Responsibility #22 (Learning Trigger) agora inclui verificação de compression candidates. | `post-session → check compression triggers` |
| **AUTO_EVOLUTION_PROTOCOL** | Stage 7 (EXTRACT PATTERN) agora diferencia: padrão tático vs candidato a princípio. | `stage 7 → tag as pattern or principle_candidate` |

### 8.3 Ciclo de Vida do Conhecimento Comprimido

```
┌──────────────────────────────────────────────────────────────────┐
│                                                                  │
│  APRENDIZADOS BRUTOS (learnings.md)                              │
│       │                                                          │
│       │ Cluster Detection (similaridade > 0.75)                  │
│       ▼                                                          │
│  PADRÕES (patterns.md)                                           │
│       │                                                          │
│       │ Pattern Evolution (versões 1.0 → N.0)                   │
│       │                                                          │
│       │ Compression Trigger (> 5 versões OU > 3 aprendizados     │
│       │                         sobre mesmo tópico)              │
│       ▼                                                          │
│  PRINCÍPIOS (knowledge/principles/CCP-NNN.yaml)                  │
│       │                                                          │
│       │ Wisdom Decay (revalidação a cada 90 dias)                │
│       │                                                          │
│       │ Se refutado → marcar como SUPERSEDED, criar CCP-NEW      │
│       │ Se confirmado → gravity aumenta, confidence sobe         │
│       ▼                                                          │
│  CONHECIMENTO FUNDACIONAL (gravity Planet, imutável na prática)  │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

---

## 9. GOVERNANÇA DE PRINCÍPIOS

### 9.1 Propriedade

| Responsabilidade | Entidade |
|------------------|----------|
| **Criação** | Cognitive Compression Engine (automático) ou Cosca AI Chief (manual) |
| **Aprovação** | Cosca Kernel (via Cognitive Immune System check) |
| **Revisão** | Cosca Critic Chief (contrafactual: "E se o oposto for verdade?") |
| **Revalidação** | Wisdom Decay Engine (a cada 90 dias) |
| **Deprecação** | Cosca Kernel + Don (se princípio for refutado por evidência) |

### 9.2 Imutabilidade Parcial

Princípios seguem o mesmo modelo de imutabilidade do Decision DNA:

| Campo | Mutável? | Regra |
|-------|:--------:|-------|
| `id`, `name`, `statement.core` | ❌ Não | O núcleo do princípio é imutável. Se estiver errado, cria-se um NOVO princípio que o substitui. |
| `statement.elaboration` | ✅ Sim | Pode ser expandido com novas evidências. |
| `evidence.strength`, `universal_applicability` | ✅ Sim | Atualizados a cada ciclo de revalidação. |
| `gravity.score` | ✅ Sim | Recalculado a cada validação ou refutação. |
| `derived_from` | ✅ Sim (append only) | Novas entradas podem ser adicionadas. |
| `corollaries`, `anti_patterns` | ✅ Sim | Expandidos com novas descobertas. |
| `reconsideration_triggers` | ✅ Sim | Atualizados conforme contexto muda. |

### 9.3 Substituição de Princípios

Quando um princípio é refutado (evidência mostra que ele está errado ou incompleto):

```
PRINCÍPIO ANTIGO (CCP-OLD)           PRINCÍPIO NOVO (CCP-NEW)
──────────────────────────           ──────────────────────────
Status: SUPERSEDED                   Status: ACTIVE
Replaced by: CCP-NEW                 Supersedes: CCP-OLD
Gravity: decai para Stone (21-55)    Gravity: calculado normalmente
                                     Derived from: CCP-OLD (como
                                     "evidência contrária")
```

O princípio antigo NUNCA é deletado. Ele permanece como registro histórico do que o runtime acreditava e por que mudou de ideia.

---

## 10. COMANDOS DO MOTOR

### 10.1 CLI (projetado)

```bash
# Disparar compressão completa
cosca cognitive compress --all

# Comprimir cluster específico
cosca cognitive compress --cluster audit

# Simular compressão (DRY_RUN — sem publicar princípios)
cosca cognitive compress --dry-run --all

# Verificar candidatos a compressão sem executar
cosca cognitive compress --check

# Gerar relatório de compressão
cosca cognitive compress --report

# Revalidar princípio existente
cosca cognitive compress --validate CCP-001
```

### 10.2 Workflow Automático

O motor é invocado automaticamente pelo sistema quando:

1. **Pós-sessão**: Após cada sessão, o Kernel verifica thresholds T1-T5.
2. **Entropy spike**: Se Cognitive Entropy subir > 10pp em uma medição semanal.
3. **Pattern milestone**: Se um padrão atingir versão 5.0.0.
4. **Don command**: Se o Don ordenar compressão explícita.

---

## 11. CONSTRAINTS E LIMITAÇÕES

| # | Constraint | Razão |
|---|-----------|-------|
| **C1** | Princípios NUNCA substituem aprendizados originais | Aprendizados são evidência. Princípios são abstração. Ambos coexistem. |
| **C2** | Compression ratio mínimo para princípio: 5:1 | Abaixo disso, o cluster não tem massa crítica para justificar abstração. |
| **C3** | Máximo de 1 princípio por cluster | Se um cluster produz 2 princípios, está mal clusterizado. |
| **C4** | Princípios devem ser falseáveis | "Sempre verifique" é falseável (basta 1 caso de verificação que falhou). "Seja cuidadoso" não é falseável — não é um princípio. |
| **C5** | Nenhum princípio contradiz a CONSTITUTION.md | A Constituição é a lei suprema. Princípios são derivados dela. |
| **C6** | Compression Engine NUNCA comprime sem aprovação quando Entropia < 50% | Compressão automática só em estado crítico. Em estado normal, requer aprovação. |
| **C7** | Princípios têm peso gravitacional máximo (Planet) mas NÃO são imunes a revalidação | Wisdom Decay se aplica. Após 90 dias sem revalidação, gravity decai 10%. |

---

## 12. MÉTRICAS DE SUCESSO

Após a primeira execução do Compression Engine, as seguintes métricas devem ser verdadeiras:

| # | Métrica | Baseline | Alvo Pós-Compressão | Status |
|---|---------|:--------:|:--------------------:|:------:|
| **M1** | Cognitive Entropy | 64.5% 🟠 | < 30% 🟢 | 🎯 Pendente |
| **M2** | Princípios extraídos | 0 | ≥ 4 | 🎯 Esta task |
| **M3** | Compression Ratio médio | N/A | > 20:1 | 🎯 Pendente |
| **M4** | Consolidation Gap | 80/100 | < 20/100 | 🎯 Pendente |
| **M5** | CMI Aprendizado | 88 | ≥ 93 | 🎯 Pendente |
| **M6** | CMI Consistência | 92 | ≥ 96 | 🎯 Pendente |
| **M7** | Aprendizados com referência a princípio | 0 | ≥ 15 | 🎯 Pendente |
| **M8** | Tempo de compressão (4 clusters) | N/A | < 3 minutos | 🎯 Pendente |

---

## 13. ESCALATION

| Situação | Ação |
|----------|------|
| **Princípio extraído contradiz CONSTITUTION.md** | Rejeitar princípio. Escalar ao Don. Revisar pipeline de extração. |
| **Cognitive Immune System detecta contradição entre novo princípio e existente** | Benchmark ambas as hipóteses. Escalar ao Critic Chief para decisão. |
| **Compressão resulta em Entropia MAIOR que antes** | Alerta crítico. O motor está com defeito. Escalar ao Kernel e Don. |
| **3 tentativas de compressão no mesmo cluster falham (Q1-Q7)** | Cluster não está maduro. Marcar como `compression_deferred`. Revisar em 30 dias. |
| **Princípio causa decisão incorreta em produção** | Marcar princípio como `under_review`. Congelar influência gravitacional (gravity → 0 temporário). Investigar causa raiz. |

---

## 14. PRÓXIMOS PASSOS (Fase 3 Expandida)

### 14.1 Compressões Pendentes

Após as 4 compressões iniciais (CCP-001 a CCP-004), os seguintes clusters precisam amadurecer para futura compressão:

| Cluster | Entradas | Status | Ação |
|---------|:--------:|--------|------|
| **Metacognição** | 4 (L8, L13, L17, L22) | Similaridade 0.65 — abaixo do threshold | Aguardar mais 2-3 aprendizados de metacognição para formar cluster maduro |
| **Fase 2 Engines** | 1 (L24) | Cluster unitário | Aguardar Fase 3 execuções para expandir cluster |
| **Cross-Agent AI** | 4 (cosca-ai learnings) | Similaridade 0.55 — domínios diferentes | Reavaliar após cosca-ai acumular mais aprendizados |
| **Documentation Drift** | Padrão recorrente (L18, L20, L21, doc sync) | Já coberto por CCP-001 | Sem ação adicional necessária |

### 14.2 Evolução do Motor

| Versão | Funcionalidade | Status |
|--------|---------------|--------|
| **v1.0.0** (atual) | Compressão manual/aprovada. 4 clusters iniciais. Template YAML. | ✅ Ativo |
| **v1.1.0** (projetado) | Cluster detection automática via semantic similarity. Embeddings para todos os aprendizados. | 🔜 Planejado |
| **v1.2.0** (projetado) | Compressão automática quando Entropia > 60%. Pipeline completo sem intervenção humana. | 🔜 Planejado |
| **v2.0.0** (projetado) | Cross-project compression. Princípios federados entre instâncias do runtime. | 🔮 Visão |

---

## 15. REFERÊNCIAS

### Documentos

| Documento | Caminho | Relação |
|-----------|---------|---------|
| COGNITIVE_MATURITY.md §5 C10 | [../../architecture/COGNITIVE_MATURITY.md](../../architecture/COGNITIVE_MATURITY.md) | Arquitetura original do conceito C10 |
| COGNITIVE_ENTROPY.md | [../../analytics/COGNITIVE_ENTROPY.md](../../analytics/COGNITIVE_ENTROPY.md) | Métrica que dispara a compressão |
| Cognitive Gravity Engine | [../cognitive-gravity/SKILL.md](../cognitive-gravity/SKILL.md) | Motor que atribui gravidade Planet aos princípios |
| Cognitive Immune System | [../cognitive-immune-system/SKILL.md](../cognitive-immune-system/SKILL.md) | Motor que verifica não-contradição de princípios |
| Pattern Evolution Engine | [../pattern-evolution/SKILL.md](../pattern-evolution/SKILL.md) | Motor que gerencia o ciclo de vida de padrões (agora corolários) |
| CONSTITUTION.md | [../../CONSTITUTION.md](../../CONSTITUTION.md) | Lei suprema — princípios não podem contradizer |
| KERNEL.md | [../../KERNEL.md](../../KERNEL.md) | Responsibility #22 (Learning Trigger) e #23 (Evolution Trigger) |
| LEARNING_PROTOCOL.md | [../../memory/LEARNING_PROTOCOL.md](../../memory/LEARNING_PROTOCOL.md) | Formato de aprendizado que alimenta o motor |
| AUTO_EVOLUTION_PROTOCOL.md | [../../shared/AUTO_EVOLUTION_PROTOCOL.md](../../shared/AUTO_EVOLUTION_PROTOCOL.md) | Protocolo de auto-evolução. Stage 7 agora diferencia padrão vs princípio. |

### Dados Fonte (Memória)

| Arquivo | Caminho | Conteúdo |
|---------|---------|----------|
| cosca-kernel learnings | [../../memory/agent/cosca-kernel/learnings.md](../../memory/agent/cosca-kernel/learnings.md) | 26 aprendizados (L9-L24) |
| cosca-kernel patterns | [../../memory/agent/cosca-kernel/patterns.md](../../memory/agent/cosca-kernel/patterns.md) | 3 padrões extraídos (PATTERN-001, 002, 003) |
| cosca-ai learnings | [../../memory/agent/cosca-ai/learnings.md](../../memory/agent/cosca-ai/learnings.md) | 4 aprendizados de AI |
| cosca-ai patterns | [../../memory/agent/cosca-ai/patterns.md](../../memory/agent/cosca-ai/patterns.md) | 2 padrões de AI |
| entropy-baseline | [../../analytics/entropy-baseline-2026-07-30.md](../../analytics/entropy-baseline-2026-07-30.md) | Baseline detalhada de entropia |

---

## 16. HISTÓRICO

| Versão | Data | Autor | Mudanças |
|--------|------|-------|----------|
| 1.0.0 | 2026-07-30 | Cosca AI Chief | Definição completa do Cognitive Compression Engine. Arquitetura do motor (gatilhos, cluster detection, extração, verificação). Template canônico YAML. 4 compressões práticas com dados reais da Cosca (CCP-001 a CCP-004). Métricas de compressão. Integração com Entropy, Gravity, Immune System, CMI. Governança de princípios. Comandos CLI. 16 seções, ~800 linhas. |

---

> **"Depois de centenas de projetos. O runtime cria regras universais. Em vez de lembrar 80 casos. Ele aprende: Todos seguem o mesmo princípio. É como um cientista criando uma teoria."**
>
> — O Don, definindo Cognitive Compression
>
> **"20 aprendizados → 4 princípios. 5.200 linhas → 138 linhas. Entropia 64.5% → 19.0%. Isto não é redução — é destilação."**
>
> — Cognitive Compression Engine, 2026-07-30
