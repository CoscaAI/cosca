# SISTEMA IMUNOLÓGICO COGNITIVO — Cognitive Immune System

> **Versão**: 1.0.0 | **Status**: active | **Owner**: Cosca Security Chief | **Criado**: 2026-07-30
>
> **Fase de Implementação**: Fase 2 (Motores) — F2.2 no [cognitive-maturity-implementation.md](../../workflows/cognitive-maturity-implementation.md)
>
> **Conceito Cognitivo**: C5 no [COGNITIVE_MATURITY.md](../../architecture/COGNITIVE_MATURITY.md)
>
> **CMI Impact**: Consistência +8, Autocrítica +5
>
> **Colaboradores**: cosca-qa (validação de benchmarks), cosca-memory-chief (registro de anticorpos)
>
> **Dependência**: F1.4 (Wisdom Decay — TTL de conhecimento necessário para distinguir validade temporal)

---

## 1. PROPÓSITO

O Sistema Imunológico Cognitivo é o motor de defesa proativa da base de conhecimento do Cosca Runtime. Assim como o sistema imunológico biológico não espera o corpo adoecer para agir — ele patrulha, identifica ameaças e as neutraliza antes que causem dano —, o Cognitive Immune System não espera que agentes tomem decisões erradas baseadas em conhecimento contaminado. Ele intercepta, verifica e valida **antes que o conhecimento entre na base**.

Este motor existe porque o Cosca já sofreu contaminação de conhecimento em produção. Os incidentes documentados provam que agentes tomam decisões baseadas no que leem, e se o que leem é ficção, as decisões são erradas. Sem um sistema imunológico, cada documento falso, cada claim não verificada, cada afirmação contraditória — tudo contamina silenciosamente a memória coletiva do runtime.

### O Problema que Resolve

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                     KNOWLEDGE CONTAMINATION LIFE CYCLE                         │
│                          (Sem Sistema Imunológico)                              │
│                                                                                │
│  1. Agente escreve doc     2. Doc afirma          3. Outro agente lê           │
│     "Cosca usa              "PostgreSQL como       o doc e decide              │
│     PostgreSQL..."          banco principal"       usar pgx driver             │
│         │                       │                       │                      │
│         ▼                       ▼                       ▼                      │
│     FICÇÃO                  CONTAMINAÇÃO            DECISÃO ERRADA             │
│                                                                                │
│  4. Terceiro agente         5. A memória coletiva   6. Nova feature             │
│     cita o doc como          agora "sabe" que        construída sobre           │
│     referência               PostgreSQL é real       premissa falsa             │
│         │                       │                       │                      │
│         ▼                       ▼                       ▼                      │
│     PROPAGAÇÃO               FATO FALSO              DÍVIDA TÉCNICA            │
│                             INSTITUÍDO               ACUMULADA                 │
│                                                                                │
│  RESULTADO: 315 linhas de documentação fictícia removidas só no L20.           │
│  A contaminação levou semanas para ser detectada — e só foi porque o Don       │
│  ordenou uma auditoria cross-source explícita.                                  │
│                                                                                │
└──────────────────────────────────────────────────────────────────────────────┘
```

### O Que Muda com o Sistema Imunológico

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                     KNOWLEDGE INOCULATION (Com Sistema Imunológico)            │
│                                                                                │
│  1. Agente propõe novo     2. IMMUNE SYSTEM       3. Contradição?             │
│     conhecimento            intercepta             ┌──── SIM ────▶ Challenge  │
│         │                       │                  │                          │
│         ▼                       ▼                  │                          │
│     CLAIM                   INOCULATION            └──── NÃO ───▶ Aprovado    │
│                              PHASE                                              │
│                                                                                │
│  4. Resultado do Challenge:                                                    │
│     ┌─ Evidência mais forte que conhecimento existente → ATUALIZA             │
│     ├─ Evidência mais fraca → REJEITA (registra como false positive)          │
│     ├─ Evidências iguais → BENCHMARK (cosca-qa testa ambas hipóteses)         │
│     └─ Nenhuma evidência → QUARENTENA (isolado até validação humana)          │
│                                                                                │
│  RESULTADO: Conhecimento falso nunca entra. Verdade é continuamente           │
│  verificada. A base permanece limpa.                                            │
│                                                                                │
└──────────────────────────────────────────────────────────────────────────────┘
```

---

## 2. CASOS DE CONTAMINAÇÃO DOCUMENTADOS (Evidência Histórica)

Antes de definir o sistema, é essencial entender exatamente o que ele precisa prevenir. Estes são os incidentes reais de contaminação que o Cosca sofreu:

### 2.1 Caso A — PostgreSQL Fantasy (Detectado em L20, 2026-07-29)

| Aspecto | Detalhe |
|---------|---------|
| **O que aconteceu** | 3 documentos de arquitetura afirmavam que o Cosca usava PostgreSQL como banco principal. O código real (`go.mod`) usa SQLite via `modernc.org/sqlite`. A documentação também mencionava Redis, pgvector e Kafka — todas infraestruturas inexistentes. |
| **Como contaminou** | Agentes que liam os docs tomavam decisões baseadas em PostgreSQL (ex: sugerir driver `pgx`, planejar migrations com `golang-migrate`). Agentes que liam o código sabiam a verdade, mas não liam os docs. |
| **Tempo até detecção** | Semanas. Só foi descoberto quando o Don ordenou uma auditoria cross-source explícita (docs vs `go.mod`). |
| **Dano** | 315 linhas de documentação removidas. Decisões arquiteturais potencialmente erradas por agentes que confiaram nos docs. |
| **Como o Immune System teria prevenido** | Na Inoculation Phase, qualquer claim sobre banco de dados seria cruzada com `go.mod`. `modernc.org/sqlite` está lá. Nenhum driver PostgreSQL está. A claim seria REJEITADA com severidade CRITICAL. |

### 2.2 Caso B — Coverage Threshold Crisis (Detectado em L18, 2026-07-29)

| Aspecto | Detalhe |
|---------|---------|
| **O que aconteceu** | 4 documentos diferentes definiam thresholds de cobertura diferentes para o mesmo gate de qualidade: 70% (QUALITY_GATES.md), 80% (getting-started.md), 85% (scorecard.md), e 4 valores listados em coverage.md. Nenhum agente sabia qual era o correto. |
| **Como contaminou** | Cada agente que lia um doc diferente aplicava um threshold diferente. O CI falhava ou passava dependendo de qual doc o agente tinha lido. Inconsistência sistêmica. |
| **Tempo até detecção** | Dias. Detectado durante a auditoria cross-agent (L18). |
| **Dano** | 3 documentos ainda têm valores errados (residual após correção). A crise foi resolvida unificando em 70% statement + 60% branch, mas docs residuais persistem. |
| **Como o Immune System teria prevenido** | Na Consistency Check, o sistema detectaria que 4 fontes afirmam valores diferentes para o mesmo gate. Dispararia CONTRADICTION entre as próprias fontes de documentação. Exigiria resolução antes que qualquer agente usasse o valor. |

### 2.3 Caso C — Fictitious Compliance Claims (Detectado em 2026-07-28)

| Aspecto | Detalhe |
|---------|---------|
| **O que aconteceu** | A memória de compliance (`memory/long/compliance-framework.md`) alegava que o Cosca tinha certificações GDPR, SOC2 e ISO 27001 — com datas de certificação fictícias. Nenhuma dessas certificações existe. |
| **Como contaminou** | Agentes de compliance e segurança referenciavam essas certificações como fato. O `cosca-security` chegou a listar GDPR/SOC2 como "implementados" em seu learning (corrigido depois). |
| **Tempo até detecção** | Detectado durante auditoria de memória do cosca-security (2026-07-28). |
| **Dano** | Confiança inflada na postura de compliance. Decisões de negócio baseadas em conformidade inexistente. |
| **Como o Immune System teria prevenido** | Na Source Validation, claims de compliance seriam classificadas como ASSERTION (confiança mais baixa) a menos que acompanhadas de evidência de auditoria externa. O sistema exigiria evidência concreta (certificado, relatório de auditoria). Sem evidência → QUARENTENA. |

### 2.4 Caso D — Capability Profile Drift (Detectado em L22, 2026-07-30)

| Aspecto | Detalhe |
|---------|---------|
| **O que aconteceu** | O `capability-profile.md` do Kernel afirmava Level 3, mas o Kernel estava executando tarefas Level 4 há semanas. A auto-avaliação estava desatualizada em relação à performance real. |
| **Como contaminou** | Agentes que consultavam o capability-profile subestimavam a capacidade do Kernel, resultando em escalações desnecessárias ou planejamento excessivamente conservador. |
| **Tempo até detecção** | Detectado durante auto-avaliação. |
| **Dano** | Desconfiança no sistema de capability profiles como um todo. Se o Kernel mente sobre o próprio nível, quem mais mente? |
| **Como o Immune System teria prevenido** | Na Consistency Check, o sistema cruzaria claims de capability-profile com evidência real de execução (learnings.md com tasks Level 4). Detectaria o drift e exigiria atualização. |

### 2.5 Caso E — Infraestrutura Fictícia (K8s, Kafka) (Detectado em L20, 2026-07-29)

| Aspecto | Detalhe |
|---------|---------|
| **O que aconteceu** | Documentação de arquitetura mencionava deploy em Kubernetes e integração com Kafka. Nenhum dos dois existe no código ou na configuração. |
| **Como contaminou** | Agentes de DevOps e infraestrutura geravam configurações de deploy para uma plataforma que não é usada. |
| **Tempo até detecção** | Semanas. Parte do mesmo incidente L20. |
| **Dano** | Confusão sobre a arquitetura real de deploy. Esforço desperdiçado em configurações irrelevantes. |
| **Como o Immune System teria prevenido** | Cruzamento com `Dockerfile`, `docker-compose.yml`, e diretório `k8s/`. Se nenhum artefato Kubernetes existe, claim de deploy K8s é rejeitada. |

---

## 3. ARQUITETURA DO SISTEMA IMUNOLÓGICO

O sistema opera em 5 fases, inspiradas no sistema imunológico biológico:

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                    COGNITIVE IMMUNE SYSTEM ARCHITECTURE                         │
│                                                                                │
│  ┌──────────────────────────────────────────────────────────────────────────┐ │
│  │                          FASE 1: INOCULATION                               │ │
│  │                     (Intercepta antes da ingestão)                          │ │
│  │                                                                            │ │
│  │   NOVO CONHECIMENTO ──▶ 1a. Contradiction Check                            │ │
│  │                         1b. Source Validation                               │ │
│  │                         1c. Consistency Check                               │ │
│  │                                                                            │ │
│  │   PASSOU NOS 3? ──▶ APROVADO, entra na base                                │ │
│  │   FALHOU? ──▶ FASE 2 (Challenge)                                           │ │
│  └──────────────────────────────────────────────────────────────────────────┘ │
│                                    │                                           │
│                                    ▼                                           │
│  ┌──────────────────────────────────────────────────────────────────────────┐ │
│  │                          FASE 2: CHALLENGE                                 │ │
│  │                     (Resolve contradições)                                  │ │
│  │                                                                            │ │
│  │   CONTRADIÇÃO DETECTADA ──▶ Evidência existente vs nova evidência          │ │
│  │                                                                            │ │
│  │   ┌─ Nova > Existente ──▶ ATUALIZA conhecimento                            │ │
│  │   ├─ Existente > Nova ──▶ REJEITA nova claim                               │ │
│  │   ├─ Evidências iguais ──▶ BENCHMARK (cosca-qa)                            │ │
│  │   └─ Nenhuma evidência ──▶ QUARENTENA                                      │ │
│  └──────────────────────────────────────────────────────────────────────────┘ │
│                                    │                                           │
│                                    ▼                                           │
│  ┌──────────────────────────────────────────────────────────────────────────┐ │
│  │                          FASE 3: ANTIBODY PRODUCTION                        │ │
│  │                     (Aprende com a contaminação)                            │ │
│  │                                                                            │ │
│  │   CONTAMINAÇÃO RESOLVIDA ──▶ Extrai padrão ──▶ Cria anticorpo             │ │
│  │                                                                            │ │
│  │   Anticorpo = {domain}:{claim_type}:{detection_method}                      │ │
│  │   Ex: docs:database_claim:go_mod_crosscheck                                 │ │
│  └──────────────────────────────────────────────────────────────────────────┘ │
│                                    │                                           │
│                                    ▼                                           │
│  ┌──────────────────────────────────────────────────────────────────────────┐ │
│  │                          FASE 4: IMMUNE MEMORY                             │ │
│  │                     (Registro de anticorpos)                                │ │
│  │                                                                            │ │
│  │   Catálogo de anticorpos ──▶ Auto-evolve ──▶ Promove a gates obrigatórios │ │
│  │   Registro: padrão, severidade, vezes disparado, última ativação           │ │
│  └──────────────────────────────────────────────────────────────────────────┘ │
│                                    │                                           │
│                                    ▼                                           │
│  ┌──────────────────────────────────────────────────────────────────────────┐ │
│  │                          FASE 5: VACCINATION                               │ │
│  │                     (Scan proativo da base existente)                       │ │
│  │                                                                            │ │
│  │   Scanner contínuo ──▶ Cruza toda documentação contra:                     │ │
│  │   • go.mod (dependências reais)                                            │ │
│  │   • Estrutura de diretórios (pastas reais)                                  │ │
│  │   • Código fonte (APIs reais)                                              │ │
│  │   • Configuração runtime (comportamento real)                               │ │
│  │                                                                            │ │
│  │   Output: Contamination Report com severidade e remediação                 │ │
│  └──────────────────────────────────────────────────────────────────────────┘ │
│                                                                                │
└──────────────────────────────────────────────────────────────────────────────┘
```

---

## 4. FASE 1 — INOCULATION (Interceptação Pré-Ingestão)

**Quando ativa**: Toda vez que qualquer agente propõe adicionar, modificar ou registrar conhecimento no sistema. Isso inclui:
- Escrita em `learnings.md` de qualquer agente
- Criação ou atualização de documentação (docs/, README, ADRs)
- Registro de heurísticas (`knowledge/heuristics/`)
- Registro de padrões (`memory/patterns.md`)
- Atualização de capability profiles
- Ingestão de qualquer claim em memória de longo prazo

### 4.1 Check 1: Contradiction Check (Verificação de Contradições)

**Objetivo**: Detectar se a nova claim contradiz conhecimento existente.

**Algoritmo**:

```yaml
contradiction_check:
  step_1_scan:
    action: "Varrer todas as fontes de conhecimento existentes em busca de claims sobre o mesmo domínio"
    sources:
      - "learnings.md de todos os agentes (busca semântica por domínio relacionado)"
      - "Documentação (docs/, README.md, ADRs)"
      - "Padrões e heurísticas registrados"
      - "Memória de decisão (.cosca/memory/decision/)"
      - "Capability profiles de agentes relevantes"
    method: "Semantic similarity search (motor da Semantic Memory Engine)"

  step_2_compare:
    action: "Comparar a claim existente com a nova claim"
    method: "Análise de afirmações opostas"
    threshold: "similaridade > 0.7 + direção oposta da afirmação"

  step_3_classify:
    action: "Classificar a severidade da contradição"
    severity:
      CRITICAL:
        definition: "Contradição direta — as duas claims são mutuamente exclusivas"
        examples:
          - "Claim A: 'O banco de dados é PostgreSQL' vs Claim B: 'O banco de dados é SQLite'"
          - "Claim A: 'Coverage threshold é 70%' vs Claim B: 'Coverage threshold é 85%'"
        action: "Bloquear ingestão imediatamente. Disparar Challenge Phase."

      HIGH:
        definition: "Inconsistência significativa — as claims podem coexistir mas uma está provavelmente errada"
        examples:
          - "Claim A: 'A API suporta OAuth2' vs go.mod sem dependência OAuth2"
          - "Claim A: 'Deploy em Kubernetes' vs ausência de arquivos k8s/"
        action: "Marcar para Challenge Phase. Permitir ingestão com flag 'DISPUTED'."

      MEDIUM:
        definition: "Diferença de versão ou escopo — uma claim pode estar desatualizada"
        examples:
          - "Claim A: 'Versão 1.3.0' vs CHANGELOG: 'Versão 1.4.0-dev'"
          - "Claim A: '34 comandos CLI' vs código: '39 comandos CLI'"
        action: "Alertar. Ingestão permitida com flag 'NEEDS_UPDATE'."

      LOW:
        definition: "Diferença de redação — mesmo significado, palavras diferentes"
        examples:
          - "'Sistema de mensageria' vs 'Message broker' — mesmo conceito"
        action: "Registrar como variação linguística. Sem bloqueio."
```

### 4.2 Check 2: Source Validation (Validação de Fonte)

**Objetivo**: Determinar a confiabilidade da evidência que sustenta a claim.

**Hierarquia de Confiança de Fonte** (alinhada com CONSTITUTION.md P2):

```yaml
source_validation:
  evidence_levels:
    CODE:
      weight: 1.00
      description: "Claims respaldadas por código fonte executável"
      verification: "Cross-reference com go.mod, arquivos .go, imports, testes"
      examples:
        - "Claim: 'Usa SQLite' → go.mod contém modernc.org/sqlite → CONFIRMADO"
        - "Claim: 'Usa PostgreSQL' → go.mod NÃO contém driver PostgreSQL → REFUTADO"

    BENCHMARK:
      weight: 0.90
      description: "Claims respaldadas por medições e benchmarks executados"
      verification: "Resultados de benchmark engine com dados numéricos"
      examples:
        - "Claim: 'Latência < 100ms' → benchmark mostra p95=87ms → CONFIRMADO"
        - "Claim: 'Cobertura 97.9%' → go test -cover mostra 71.3% → REFUTADO"

    AUDIT:
      weight: 0.75
      description: "Claims respaldadas por auditoria sistemática"
      verification: "Relatório de auditoria com metodologia documentada"
      examples:
        - "Claim: '0 CVEs críticos' → govulncheck audit → CONFIRMADO"
        - "Claim: '98.1% DNA compliance' → auditoria de governança → CONFIRMADO"

    ASSERTION:
      weight: 0.30
      description: "Claims sem evidência concreta — baseadas em raciocínio ou memória"
      verification: "Nenhuma — é o nível mais baixo de confiança"
      examples:
        - "Claim: 'GDPR compliant' → sem evidência de auditoria → ASSERTION"
        - "Claim: 'Suporta Redis caching' → sem dependência no go.mod → ASSERTION"

  decision_matrix:
    # Confiança da fonte × threshold mínimo para aprovação
    approval_thresholds:
      critical_claim: 0.90  # Claims sobre infraestrutura, segurança, compliance
      high_claim: 0.75      # Claims sobre arquitetura, performance, versões
      medium_claim: 0.50    # Claims sobre padrões, convenções, recomendações
      low_claim: 0.30       # Claims sobre estilo, opiniões, observações
```

**Importância da Claim vs Threshold**:

| Tipo de Claim | Threshold Mínimo | Exemplo |
|---------------|-----------------|---------|
| **Infraestrutura** (banco, cache, mensageria) | 0.90 (CODE ou BENCHMARK) | "Usamos PostgreSQL" → exige `go.mod` |
| **Segurança** (compliance, certificações) | 0.90 (AUDIT) | "SOC2 certified" → exige certificado |
| **Performance** (latência, cobertura, throughput) | 0.75 (BENCHMARK) | "Cobertura 97.9%" → exige `go test -cover` |
| **Arquitetura** (padrões, decisões) | 0.75 (AUDIT ou CODE) | "Arquitetura hexagonal" → exige evidência |
| **Convenções** (naming, estilo) | 0.50 (ASSERTION ok) | "Usamos camelCase" → baixa criticidade |

### 4.3 Check 3: Consistency Check (Verificação de Consistência)

**Objetivo**: Verificar se a claim é consistente com a realidade observável do código, arquivos e configuração.

```yaml
consistency_check:
  cross_references:
    go_mod:
      description: "Claims sobre dependências devem ser verificadas contra go.mod"
      rules:
        - "Claim menciona banco de dados X → go.mod deve conter driver X"
        - "Claim menciona cache Y → go.mod deve conter cliente Y"
        - "Claim menciona mensageria Z → go.mod deve conter cliente Z"
        - "Claim menciona framework W → go.mod deve conter framework W"

    directory_structure:
      description: "Claims sobre estrutura devem ser verificadas contra o sistema de arquivos"
      rules:
        - "Claim menciona deploy K8s → deve existir diretório k8s/ ou helm/"
        - "Claim menciona Docker → deve existir Dockerfile"
        - "Claim menciona CI/CD específico → deve existir .github/workflows/ ou equivalente"
        - "Claim menciona N comandos CLI → count deve bater com código"

    runtime_behavior:
      description: "Claims sobre comportamento devem ser verificadas contra o runtime"
      rules:
        - "Claim menciona N agentes ativos → verificar contra capability registry"
        - "Claim menciona cobertura X% → verificar com go test -cover"
        - "Claim menciona latency < Xms → verificar com benchmark"
        - "Claim menciona nível de capability Y → verificar contra evolution.md"

    numerical_consistency:
      description: "Números em documentação devem ser verificados contra realidade"
      rules:
        - "Contagem de arquivos/pacotes/agentes → verificar com find/go list"
        - "Versão do projeto → verificar contra git tags e CHANGELOG"
        - "Datas de release → verificar contra git log"
        - "Thresholds e métricas → verificar contra configuração real"
```

---

## 5. FASE 2 — CHALLENGE (Resolução de Contradições)

**Quando ativa**: Quando a Fase 1 (Inoculation) detecta uma contradição de severidade CRITICAL ou HIGH.

### 5.1 Algoritmo de Resolução

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                          CHALLENGE RESOLUTION FLOW                              │
│                                                                                │
│                         CONTRADIÇÃO DETECTADA                                   │
│                                │                                               │
│                                ▼                                               │
│                    ┌──────────────────────┐                                    │
│                    │ COMPARAR EVIDÊNCIAS  │                                    │
│                    │                      │                                    │
│                    │ Evidência existente  │                                    │
│                    │ vs                   │                                    │
│                    │ Evidência nova       │                                    │
│                    └──────────┬───────────┘                                    │
│                               │                                               │
│              ┌────────────────┼────────────────┐                              │
│              │                │                │                              │
│              ▼                ▼                ▼                              │
│     ┌────────────┐   ┌────────────┐   ┌────────────┐                          │
│     │ NOVA >     │   │ EXISTENTE  │   │ EVIDÊNCIAS │                          │
│     │ EXISTENTE  │   │ > NOVA     │   │ IGUAIS     │                          │
│     └─────┬──────┘   └─────┬──────┘   └─────┬──────┘                          │
│           │                │                │                                 │
│           ▼                ▼                ▼                                 │
│     ┌────────────┐   ┌────────────┐   ┌────────────┐                          │
│     │  ATUALIZAR │   │  REJEITAR  │   │ BENCHMARK  │                          │
│     │conhecimento│   │ nova claim │   │ (cosca-qa) │                          │
│     │ existente  │   │            │   │            │                          │
│     └─────┬──────┘   └─────┬──────┘   └─────┬──────┘                          │
│           │                │                │                                 │
│           │                │                ▼                                 │
│           │                │         ┌────────────┐                           │
│           │                │         │  TESTAR    │                           │
│           │                │         │  AMBAS     │                           │
│           │                │         │  HIPÓTESES │                           │
│           │                │         └─────┬──────┘                           │
│           │                │               │                                  │
│           │                │     ┌─────────┴─────────┐                        │
│           │                │     │                   │                        │
│           │                │     ▼                   ▼                        │
│           │                │  ┌──────────┐    ┌──────────────┐                │
│           │                │  │ Hipótese │    │  Hipótese    │                │
│           │                │  │ Nova     │    │  Existente   │                │
│           │                │  │ VENCE    │    │  VENCE       │                │
│           │                │  └────┬─────┘    └──────┬───────┘                │
│           │                │       │                 │                         │
│           ▼                ▼       ▼                 ▼                         │
│     ┌─────────────────────────────────────────────────────────────────┐       │
│     │                       REGISTRAR DECISÃO                          │       │
│     │                                                                  │       │
│     │  • O que foi decidido                                            │       │
│     │  • Qual evidência prevaleceu                                      │       │
│     │  • Por que a alternativa foi rejeitada                            │       │
│     │  • Data da decisão                                                │       │
│     │  • Reconsideration triggers (quando reavaliar)                    │       │
│     └─────────────────────────────────────────────────────────────────┘       │
│                                                                                │
└──────────────────────────────────────────────────────────────────────────────┘
```

### 5.2 Regras de Resolução

```yaml
challenge_rules:
  rule_1_nova_mais_forte:
    condition: "Evidência da nova claim tem peso maior que a evidência existente"
    action: "ATUALIZAR"
    details:
      - "Substituir conhecimento existente pela nova versão"
      - "Versionar conhecimento antigo (não deletar — preservar histórico)"
      - "Registrar no changelog do conhecimento: versão, data, motivo"
      - "Notificar agentes que referenciaram o conhecimento antigo"
    example:
      existing: "Coverage threshold: 80% (fonte: ASSERTION, doc antigo)"
      new: "Coverage threshold: 70% (fonte: CODE, go test -cover config)"
      resolution: "ATUALIZAR para 70%. Versão antiga marcada como deprecated."

  rule_2_existente_mais_forte:
    condition: "Evidência existente tem peso maior que a evidência da nova claim"
    action: "REJEITAR"
    details:
      - "Nova claim é rejeitada"
      - "Registrar como 'false positive' no immune memory"
      - "Se a fonte da claim rejeitada for um agente, notificar o agente"
      - "Se reincidência do mesmo agente com claims falsas, degradar confidence do agente"
    example:
      existing: "Banco de dados: SQLite (fonte: CODE, go.mod)"
      new: "Banco de dados: PostgreSQL (fonte: ASSERTION, sem evidência)"
      resolution: "REJEITAR. Claim falsa detectada. Registrar anticorpo."

  rule_3_evidencias_iguais:
    condition: "Ambas as claims têm evidências de mesmo peso"
    action: "BENCHMARK"
    details:
      - "Disparar benchmark engine (cosca-qa coordena)"
      - "Testar ambas as hipóteses com dados reais"
      - "Resultado do benchmark decide qual claim prevalece"
      - "Se benchmark inconclusivo, escalar para o Don (decisão humana)"
    example:
      existing: "Estratégia de cache A é mais rápida (fonte: BENCHMARK antigo)"
      new: "Estratégia de cache B é mais rápida (fonte: BENCHMARK novo)"
      resolution: "Re-executar ambos os benchmarks em condições controladas."

  rule_4_nenhuma_evidencia:
    condition: "Nenhuma das claims tem evidência concreta (ambas ASSERTION)"
    action: "QUARENTENA"
    details:
      - "Ambas as claims são movidas para quarentena"
      - "Nenhuma entra na base de conhecimento ativa"
      - "Registrar no quarantine log: claims, fontes, data"
      - "Escalar para investigação: agendar auditoria ou consultar o Don"
      - "Claims em quarentena NÃO são carregadas pelo Memory Loading (§10.3)"
    example:
      existing: "Futuro: planejamos migrar para microservices (fonte: ASSERTION)"
      new: "Futuro: planejamos manter monolith (fonte: ASSERTION)"
      resolution: "AMBAS EM QUARENTENA. Investigar intenção real do Don."
```

### 5.3 Challenge Decision DNA

Toda resolução de challenge gera um registro no formato Decision DNA (C4), integrável com o motor de decisão:

```yaml
immune_challenge_decision:
  id: "IMM-2026-07-30-001"
  timestamp: "2026-07-30T14:30:00Z"

  claims:
    existing:
      content: "Banco de dados principal: SQLite"
      source_type: CODE
      source_location: "go.mod → modernc.org/sqlite v1.29.0"
      confidence: 1.00

    new:
      content: "Banco de dados principal: PostgreSQL"
      source_type: ASSERTION
      source_location: "docs/architecture/database.md"
      confidence: 0.30

  contradiction_severity: CRITICAL

  resolution:
    action: REJECT_NEW
    reason: "Código executado é a verdade absoluta (CONSTITUTION.md P2). go.mod comprova SQLite."
    evidence_winner: EXISTING
    evidence_weight_existing: 1.00
    evidence_weight_new: 0.30

  antibody_generated:
    id: "AB-001"
    signature: "docs:database_claim:go_mod_crosscheck"
    detection_rule: "Toda claim sobre tecnologia de banco de dados em documentação deve ser cruzada com go.mod"

  reconsideration_triggers:
    - "Se go.mod passar a incluir driver PostgreSQL, reavaliar esta decisão"
    - "Se nova evidência (ex: configuration file) mencionar PostgreSQL, reavaliar"
```

---

## 6. FASE 3 — ANTIBODY PRODUCTION (Produção de Anticorpos)

**Quando ativa**: Após a resolução bem-sucedida de uma contaminação (Fase 2).

### 6.1 O Que é um Anticorpo

Um anticorpo é uma regra de detecção que impede que o mesmo tipo de contaminação ocorra novamente. Assim como o sistema imunológico biológico produz anticorpos específicos para patógenos que já enfrentou, o Cognitive Immune System produz anticorpos para padrões de contaminação que já detectou.

### 6.2 Estrutura do Anticorpo

```yaml
antibody_schema:
  id: "AB-NNN"
  signature: "{domain}:{claim_type}:{detection_method}"

  metadata:
    created: "ISO8601"
    created_from: "IMM-challenge-id"
    severity: "CRITICAL | HIGH | MEDIUM | LOW"
    times_triggered: 0
    last_triggered: null
    status: "active | promoted_to_gate | deprecated"

  pattern:
    domain: "docs | memory | capability_profile | heuristics | patterns"
    claim_type: "database_claim | dependency_claim | compliance_claim | threshold_claim | infrastructure_claim | version_claim"
    contamination_vector: "Como a contaminação entrou no sistema"

  detection:
    rule: "Descrição da regra de detecção"
    cross_reference: "Quais fontes cruzar para verificar"
    false_positive_check: "Como evitar falsos positivos nesta regra"

  evolution:
    promoted_from: null  # ID do anticorpo predecessor, se evoluiu
    promoted_to: null    # ID do anticorpo sucessor, se foi substituído
```

### 6.3 Catálogo Inicial de Anticorpos (Seed)

Baseado nos 5 casos de contaminação documentados:

```yaml
antibodies_seed:
  - id: "AB-001"
    signature: "docs:database_claim:go_mod_crosscheck"
    metadata:
      created: "2026-07-30"
      created_from: "Caso A — PostgreSQL Fantasy (L20)"
      severity: CRITICAL
      times_triggered: 1
      status: active
    pattern:
      domain: docs
      claim_type: database_claim
      contamination_vector: "Documentação afirma uso de tecnologia X. Código não contém dependência de X."
    detection:
      rule: "Toda menção a tecnologia de banco de dados em documentação deve ser verificada contra go.mod"
      cross_reference: "go.mod (dependências diretas + indiretas)"
      false_positive_check: "Verificar se a tecnologia pode ser usada via driver indireto (ex: database/sql + driver)"
    evolution:
      promoted_from: null
      promoted_to: null

  - id: "AB-002"
    signature: "docs:threshold_claim:multi_source_consistency"
    metadata:
      created: "2026-07-30"
      created_from: "Caso B — Coverage Threshold Crisis (L18)"
      severity: CRITICAL
      times_triggered: 1
      status: active
    pattern:
      domain: docs
      claim_type: threshold_claim
      contamination_vector: "Múltiplos documentos definem valores diferentes para o mesmo parâmetro crítico"
    detection:
      rule: "Toda menção a threshold numérico deve ser consistente em todas as fontes. Discrepância > 0 = contradição."
      cross_reference: "Todos os arquivos .md que mencionam o mesmo parâmetro (busca full-text)"
      false_positive_check: "Verificar se thresholds diferentes são para contextos diferentes (ex: statement vs branch coverage)"
    evolution:
      promoted_from: null
      promoted_to: null

  - id: "AB-003"
    signature: "memory:compliance_claim:certification_evidence"
    metadata:
      created: "2026-07-30"
      created_from: "Caso C — Fictitious Compliance Claims"
      severity: CRITICAL
      times_triggered: 1
      status: active
    pattern:
      domain: memory
      claim_type: compliance_claim
      contamination_vector: "Memória ou documentação alega certificação de compliance sem evidência de auditoria externa"
    detection:
      rule: "Toda claim de certificação (GDPR, SOC2, HIPAA, ISO 27001, PCI-DSS) deve ser acompanhada de evidência de auditoria externa"
      cross_reference: "Verificar existência de certificados, relatórios de auditoria, ou datas de certificação reais"
      false_positive_check: "Distinguir entre 'aspiracional' (planejado) e 'implementado' (real). Claims aspiracionais devem ser explicitamente marcadas como tal."
    evolution:
      promoted_from: null
      promoted_to: null

  - id: "AB-004"
    signature: "capability:level_claim:evolution_crosscheck"
    metadata:
      created: "2026-07-30"
      created_from: "Caso D — Capability Profile Drift (L22)"
      severity: HIGH
      times_triggered: 1
      status: active
    pattern:
      domain: capability_profile
      claim_type: level_claim
      contamination_vector: "Capability profile afirma nível X, mas histórico de execução mostra performance de nível Y"
    detection:
      rule: "Nível declarado em capability-profile.md deve ser consistente com as tasks registradas em learnings.md"
      cross_reference: "learnings.md (tasks executadas + níveis) vs capability-profile.md (nível declarado)"
      false_positive_check: "Agente pode ter executado tasks de nível superior recentemente. Verificar data de atualização do profile."
    evolution:
      promoted_from: null
      promoted_to: null

  - id: "AB-005"
    signature: "docs:infrastructure_claim:filesystem_crosscheck"
    metadata:
      created: "2026-07-30"
      created_from: "Caso E — Infraestrutura Fictícia (K8s, Kafka)"
      severity: HIGH
      times_triggered: 1
      status: active
    pattern:
      domain: docs
      claim_type: infrastructure_claim
      contamination_vector: "Documentação alega uso de infraestrutura X, mas nenhum artefato de configuração de X existe"
    detection:
      rule: "Toda claim de infraestrutura deve ser verificada contra artefatos de configuração correspondentes"
      cross_reference: "k8s/ → Kubernetes, docker-compose.yml → Docker, terraform/ → IaC, go.mod → serviços"
      false_positive_check: "Infraestrutura pode ser gerenciada externamente. Verificar com cosca-infrastructure."
    evolution:
      promoted_from: null
      promoted_to: null
```

### 6.4 Ciclo de Vida do Anticorpo

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                         ANTIBODY LIFE CYCLE                                     │
│                                                                                │
│   ┌──────────┐     ┌──────────┐     ┌──────────┐     ┌──────────────┐        │
│   │  ACTIVE  │────▶│ TRIGGERED│────▶│ VALIDATED│────▶│ PROMOTED TO  │        │
│   │ (vigilant│     │(detectou │     │(confirma │     │ MANDATORY    │        │
│   │  standby)│     │contaminaç│     │ que era   │     │ GATE         │        │
│   └──────────┘     └──────────┘     │contaminação│    └──────────────┘        │
│                          │          └─────┬─────┘                             │
│                          │                │                                    │
│                          │ false          │ true positive                      │
│                          │ positive       │                                    │
│                          ▼                ▼                                    │
│                    ┌──────────┐    ┌──────────────┐                           │
│                    │ REFINED  │    │ REINFORCED   │                           │
│                    │(ajustar  │    │(confidence +)│                           │
│                    │ regra)   │    └──────┬───────┘                           │
│                    └──────────┘           │                                    │
│                                          ▼                                    │
│                                   ┌──────────────┐                           │
│                                   │ ≥ 5 TRUE     │                           │
│                                   │ POSITIVES?   │                           │
│                                   └──────┬───────┘                           │
│                                          │                                    │
│                                     ┌────┴────┐                              │
│                                   SIM│        │NÃO                           │
│                                     ▼        ▼                               │
│                              ┌──────────┐ ┌──────────┐                       │
│                              │PROMOTED  │ │ACTIVE    │                       │
│                              │TO GATE   │ │(continua │                       │
│                              │(mandatory│ │vigilante)│                       │
│                              │ check)   │ └──────────┘                       │
│                              └──────────┘                                    │
│                                                                                │
│   REGRA DE PROMOÇÃO:                                                           │
│   Anticorpos que detectam ≥ 5 true positives são promovidos a                  │
│   MANDATORY GATES — sua verificação é executada em TODA ingestão              │
│   de conhecimento, incondicionalmente.                                          │
│                                                                                │
└──────────────────────────────────────────────────────────────────────────────┘
```

---

## 7. FASE 4 — IMMUNE MEMORY (Memória Imunológica)

### 7.1 Registro de Anticorpos

O Immune Memory é um catálogo persistente de todos os anticorpos ativos, seus históricos de acionamento e suas métricas de efetividade.

```yaml
immune_memory:
  storage: "internal/embed/cosca/engines/cognitive-immune-system/antibodies/registry.yaml"
  backup: ".cosca/immune/antibodies.db (SQLite, FTS5 para busca)"

  registry_schema:
    antibody_id: "AB-NNN"
    signature: "string"
    status: "active | promoted_to_gate | deprecated"
    severity: "CRITICAL | HIGH | MEDIUM | LOW"
    times_triggered: 0
    true_positives: 0
    false_positives: 0
    precision: 0.0  # true_positives / (true_positives + false_positives)
    last_triggered: "ISO8601 | null"
    created_from: "IMM-challenge-id"
    promoted_to_gate_at: "ISO8601 | null"
    deprecated_at: "ISO8601 | null"

  metrics:
    total_antibodies: 5
    active: 5
    promoted_to_gate: 0
    deprecated: 0
    total_detections: 5
    contamination_prevented: 5
    avg_precision: 1.00  # Seed antibodies validated against known cases
```

### 7.2 Auto-Evolução de Anticorpos

Anticorpos não são estáticos. Eles evoluem com base em sua efetividade real:

```yaml
antibody_evolution:
  promotion_rule:
    condition: "true_positives >= 5 AND precision >= 0.80"
    action: "Promover anticorpo a MANDATORY GATE"
    effect: "Verificação executada em toda ingestão de conhecimento, sem exceção"

  demotion_rule:
    condition: "precision < 0.50 AND times_triggered >= 10"
    action: "Reverter para ACTIVE com flag 'needs_refinement'"
    effect: "Anticorpo volta a ser vigilante, mas não é gate obrigatório"

  refinement_rule:
    condition: "false_positives > true_positives"
    action: "Analisar falsos positivos e ajustar regra de detecção"
    effect: "Nova versão do anticorpo (v2) com regra refinada"

  deprecation_rule:
    condition: "Nenhum acionamento em 90 dias E não é MANDATORY GATE"
    action: "Marcar como deprecated"
    effect: "Anticorpo removido da verificação ativa, preservado no histórico"

  merging_rule:
    condition: "Dois anticorpos detectam padrões com sobreposição > 80%"
    action: "Fundir em um anticorpo composto com ambas as regras"
    effect: "Reduz duplicação no catálogo"
```

---

## 8. FASE 5 — VACCINATION (Vacinação — Scan Proativo)

**Quando ativa**: Periodicamente (a cada sessão, ou sob demanda via comando do Don).

A Vacinação é o scan proativo da base de conhecimento existente. Diferente da Inoculation (que intercepta conhecimento novo), a Vaccination examina o conhecimento que já está na base — procurando contaminações que entraram antes do immune system existir, ou que escaparam por qualquer razão.

### 8.1 Algoritmo de Vacinação

```yaml
vaccination_scan:
  frequency:
    light_scan: "A cada inicialização do Kernel (§10.2 Context Discovery)"
    deep_scan: "A cada 10 sessões ou sob comando explícito do Don"
    full_audit: "Mensal, coordenado pelo cosca-security"

  light_scan_steps:
    - step: "Cross-reference de claims em documentação contra go.mod"
      check: "Toda dependência mencionada em docs deve existir em go.mod"
      severity: CRITICAL

    - step: "Cross-reference de contagens em docs contra filesystem"
      check: "Números de arquivos, pacotes, agentes em docs devem bater com find/go list"
      severity: HIGH

    - step: "Cross-reference de versões em docs contra CHANGELOG/git tags"
      check: "Toda menção de versão em docs deve ser consistente com git"
      severity: MEDIUM

  deep_scan_steps:
    - step: "Verificação de consistência entre todos os arquivos de memória"
      check: "learnings.md vs capability-profile.md vs evolution.md"
      severity: HIGH

    - step: "Verificação de claims de compliance contra evidência real"
      check: "Toda claim de certificação deve ter artefato de evidência"
      severity: CRITICAL

    - step: "Verificação de claims de performance contra benchmarks"
      check: "Toda claim numérica de performance deve ter benchmark correspondente"
      severity: HIGH

    - step: "Verificação de capacidade de agentes contra tasks reais"
      check: "Capability level declarado vs tasks executadas (nível)"
      severity: HIGH

  full_audit_steps:
    - step: "Auditoria completa de todos os 426+ arquivos do framework"
      check: "Cada claim em cada arquivo é verificada contra código, go.mod, filesystem"
      severity: VARIED

    - step: "Geração de Contamination Report completo"
      check: "Lista todas as discrepâncias com severidade, fonte, evidência e remediação"
      severity: N/A
```

### 8.2 Contamination Report

O output de uma Vacinação é o Contamination Report — documento estruturado que lista toda contaminação detectada:

```yaml
contamination_report:
  header:
    generated: "ISO8601"
    scan_type: "light | deep | full_audit"
    scanned_files: 426
    total_claims_verified: 0
    contaminations_found: 0

  summary:
    critical: 0
    high: 0
    medium: 0
    low: 0
    total: 0
    cognitive_entropy_delta: 0  # Mudança no índice C2

  findings:
    - id: "CONTAM-001"
      severity: CRITICAL
      location: "docs/architecture/database.md:42"
      claim: "PostgreSQL é o banco de dados principal"
      evidence_against: "go.mod importa modernc.org/sqlite, não contém driver PostgreSQL"
      source_confidence: 0.30  # ASSERTION
      evidence_confidence: 1.00  # CODE
      antibody_triggered: "AB-001"
      remediation: "Substituir 'PostgreSQL' por 'SQLite (modernc.org/sqlite)' em todo o documento"
      status: "pending_fix"

  metrics:
    cognitive_entropy_before: 0
    cognitive_entropy_after: 0
    false_positives: 0
    scan_duration_ms: 0
```

### 8.3 Gatilhos de Vacinação

```yaml
vaccination_triggers:
  automatic:
    - event: "Kernel Bootstrap (§10.1)"
      scan: light
      description: "Scan rápido ao iniciar — verifica claims críticas"

    - event: "Session Start"
      scan: light
      description: "Verificação de integridade antes de cada sessão"

    - event: "10 sessions completed"
      scan: deep
      description: "Scan profundo a cada 10 sessões"

    - event: "Cognitive Entropy > 25"
      scan: deep
      description: "Entropia alta dispara scan profundo automático (C2)"

    - event: "Monthly cron"
      scan: full_audit
      description: "Auditoria completa mensal"

  on_demand:
    - command: "Don: 'executar vacinação completa'"
      scan: full_audit

    - command: "cosca-security: 'verificar claims de banco de dados'"
      scan: targeted (domínio específico)

    - command: "Qualquer agente pode solicitar scan se detectar inconsistência"
      scan: targeted
```

---

## 9. EXEMPLOS PRÁTICOS DE OPERAÇÃO

### 9.1 Exemplo A — PostgreSQL Contamination (Como Teria Sido Prevenida)

```
┌──────────────────────────────────────────────────────────────────────────────┐
│         EXEMPLO A: PostgreSQL Fantasy — Com Sistema Imunológico                │
│                                                                                │
│  TIMELINE:                                                                     │
│                                                                                │
│  T0: Agente cosca-documentation escreve em docs/architecture/database.md:      │
│      "O Cosca utiliza PostgreSQL como banco de dados principal..."             │
│                                                                                │
│  T1: INOCULATION PHASE intercepta a escrita.                                   │
│                                                                                │
│  T2: CONTRADICTION CHECK                                                       │
│      • Busca semântica por claims sobre "banco de dados"                       │
│      • Encontra em go.mod: modernc.org/sqlite v1.29.0                          │
│      • Encontra em internal/database/: imports de sqlite                       │
│      • NÃO encontra driver PostgreSQL em go.mod                                │
│      • Resultado: CONTRADIÇÃO CRITICAL                                         │
│        "Claim diz PostgreSQL. Código usa SQLite."                              │
│                                                                                │
│  T3: SOURCE VALIDATION                                                         │
│      • Claim nova (PostgreSQL): fonte = ASSERTION (sem evidência)              │
│        Confidence weight: 0.30                                                 │
│      • Evidência existente (SQLite): fonte = CODE (go.mod + imports)            │
│        Confidence weight: 1.00                                                 │
│      • Threshold para claims de infraestrutura: 0.90                           │
│      • Nova claim (0.30) < threshold (0.90) → REPROVADA                        │
│                                                                                │
│  T4: CONSISTENCY CHECK                                                         │
│      • go.mod cross-reference: sem driver PostgreSQL → FAIL                    │
│      • Directory structure: sem migrations/postgres/ → FAIL                    │
│      • Runtime behavior: sqlite.Open(), não pgx.Connect() → FAIL               │
│                                                                                │
│  T5: CHALLENGE PHASE                                                           │
│      • Evidência existente (CODE, 1.00) > Evidência nova (ASSERTION, 0.30)   │
│      • RESOLUÇÃO: REJEITAR nova claim                                          │
│      • Documentação NÃO é publicada                                            │
│                                                                                │
│  T6: ANTIBODY PRODUCTION                                                       │
│      • Anticorpo AB-001 criado: docs:database_claim:go_mod_crosscheck          │
│      • Regra: "Toda claim sobre banco de dados deve ser cruzada com go.mod"    │
│      • Status: ACTIVE                                                          │
│                                                                                │
│  T7: NOTIFICAÇÃO                                                               │
│      • cosca-documentation notificado: "Sua claim sobre PostgreSQL foi         │
│        rejeitada. Evidência mostra SQLite. Corrija a documentação."            │
│      • Registro no immune memory: false positive detectado                     │
│                                                                                │
│  RESULTADO: A ficção PostgreSQL nunca entra na base de conhecimento.            │
│  O documento é corrigido ANTES de ser publicado.                                │
│  Nenhum agente toma decisão baseada em infraestrutura inexistente.              │
│                                                                                │
└──────────────────────────────────────────────────────────────────────────────┘
```

### 9.2 Exemplo B — Coverage Threshold Crisis (Como Teria Sido Resolvida)

```
┌──────────────────────────────────────────────────────────────────────────────┐
│      EXEMPLO B: Coverage Threshold Crisis — Com Sistema Imunológico            │
│                                                                                │
│  SITUAÇÃO INICIAL:                                                             │
│  • QUALITY_GATES.md: "coverage threshold: 70% statement"                       │
│  • getting-started.md: "test coverage minimum: 80%"                            │
│  • scorecard.md: "coverage target: 85%"                                        │
│  • coverage.md: lista 4 valores diferentes (60%, 70%, 80%, 85%)               │
│                                                                                │
│  T0: VACCINATION SCAN (deep scan periódico)                                    │
│                                                                                │
│  T1: CONSISTENCY CHECK cross-document                                          │
│      • Scan detecta 4 fontes mencionando "coverage" + número                   │
│      • Extrai valores: 70%, 80%, 85%, e lista de 4 valores                    │
│      • Detecta: 4 fontes, valores inconsistentes                               │
│      • Classificação: CONTRADIÇÃO CRITICAL (threshold afeta CI gate)          │
│                                                                                │
│  T2: CHALLENGE PHASE                                                           │
│      • Múltiplas claims conflitantes — qual é a verdadeira?                    │
│      • SOURCE VALIDATION de cada claim:                                         │
│        - QUALITY_GATES.md (70%): fonte = CODE (é o arquivo de configuração    │
│          canônico, referenciado por KERNEL.md §10.10)                           │
│        - scorecard.md (85%): fonte = ASSERTION (documento descritivo)          │
│        - getting-started.md (80%): fonte = ASSERTION (guia de introdução)      │
│        - coverage.md (lista): fonte = ASSERTION (documento informativo)       │
│                                                                                │
│  T3: RESOLUÇÃO                                                                 │
│      • QUALITY_GATES.md é a fonte canônica (CODE weight: 0.90)                 │
│      • Demais docs são descritivos (ASSERTION weight: 0.30)                    │
│      • Evidência mais forte: 70% (QUALITY_GATES.md)                            │
│      • RESOLUÇÃO: ATUALIZAR todos os docs para refletir 70%                    │
│                                                                                │
│  T4: REMEDIATION                                                                │
│      • scorecard.md: 85% → 70% statement                                       │
│      • getting-started.md: 80% → 70% statement                                 │
│      • coverage.md: remover lista de 4 valores, referenciar QUALITY_GATES.md   │
│                                                                                │
│  T5: ANTIBODY PRODUCTION                                                       │
│      • Anticorpo AB-002 criado: docs:threshold_claim:multi_source_consistency  │
│      • Regra: "Toda menção a threshold numérico deve ser consistente           │
│        em todas as fontes de documentação"                                      │
│                                                                                │
│  RESULTADO: 4 valores conflitantes → 1 valor canônico.                          │
│  CI nunca mais falha ou passa com threshold errado.                             │
│  Agentes sempre consultam o mesmo valor.                                        │
│                                                                                │
└──────────────────────────────────────────────────────────────────────────────┘
```

### 9.3 Exemplo C — Claim sobre Redis (Cenário Hipotético)

```
┌──────────────────────────────────────────────────────────────────────────────┐
│           EXEMPLO C: Claim "Cosca suporta Redis" — Com Sistema Imunológico     │
│                                                                                │
│  CENÁRIO: Um novo agente (ou LLM) sugere adicionar à documentação:            │
│  "O Cosca suporta Redis como cache distribuído para sessões de alta            │
│   performance e invalidação de cache cross-instance."                           │
│                                                                                │
│  T0: INOCULATION PHASE intercepta                                               │
│                                                                                │
│  T1: CONTRADICTION CHECK                                                       │
│      • Busca semântica: "Redis", "cache", "distribuído"                        │
│      • go.mod: NÃO contém go-redis, redigo, nem qualquer cliente Redis         │
│      • internal/cache/: NÃO existe diretório                                   │
│      • internal/runtime/: sem imports de cache distribuído                     │
│      • Resultado: NENHUMA evidência de Redis no código                         │
│                                                                                │
│  T2: SOURCE VALIDATION                                                         │
│      • Claim: "Cosca suporta Redis" → fonte = ASSERTION (sem evidência)        │
│        Confidence weight: 0.30                                                 │
│      • Threshold para claims de infraestrutura: 0.90                           │
│      • Claim (0.30) < threshold (0.90) → REPROVADA                             │
│                                                                                │
│  T3: CONSISTENCY CHECK                                                         │
│      • go.mod: sem cliente Redis → FAIL                                        │
│      • Directory: sem internal/cache/ → FAIL                                    │
│      • Config: cosca.config.yaml sem seção redis → FAIL                         │
│      • Tests: sem testes de integração Redis → FAIL                             │
│                                                                                │
│  T4: CHALLENGE RESOLUTION                                                      │
│      • Nova claim: ASSERTION (0.30) vs Realidade: CODE (1.00)                  │
│      • RESOLUÇÃO: REJEITAR                                                      │
│      • Claim rejeitada com explicação:                                          │
│        "A afirmação de que Cosca suporta Redis não tem evidência no             │
│         código. go.mod não contém cliente Redis. Não há diretório de            │
│         cache. Se esta capacidade for desejada, crie uma feature request        │
│         e marque a documentação como 'planejado', não como 'existente'."        │
│                                                                                │
│  T5: ANTIBODY UPDATE                                                           │
│      • AB-005 (infrastructure_claim:filesystem_crosscheck) é acionado           │
│      • AB-005.times_triggered += 1                                              │
│                                                                                │
│  RESULTADO: Ficção Redis nunca entra.                                            │
│  Documentação permanece honesta sobre capacidades reais.                         │
│                                                                                │
└──────────────────────────────────────────────────────────────────────────────┘
```

---

## 10. INTEGRAÇÃO COM O ECOSSISTEMA COSCA

### 10.1 Posição no Pipeline do Kernel (§10)

O Cognitive Immune System se integra ao pipeline de inicialização do KERNEL.md §10 entre dois estágios críticos:

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                KERNEL INITIALIZATION SEQUENCE (§10)                             │
│                    Com Cognitive Immune System                                  │
│                                                                                │
│  Step 1: Bootstrap                                                             │
│  Step 2: Context Discovery                                                     │
│                                                                                │
│  Step 3: Memory Loading (§10.3)                                                │
│          │                                                                     │
│          ▼                                                                     │
│  ┌─────────────────────────────────────────────┐                              │
│  │ ★ COGNITIVE IMMUNE SYSTEM — VACCINATION     │                              │
│  │                                              │                              │
│  │  • Light scan em toda memória carregada      │                              │
│  │  • Verifica claims críticas antes de usar    │                              │
│  │  • Conhecimento em quarentena NÃO é carregado│                              │
│  │  • Conhecimento com flag DISPUTED é carregado│                              │
│  │    com alerta                                │                              │
│  │  • Gera Contamination Report se detectar     │                              │
│  │    inconsistências                           │                              │
│  └─────────────────────────────────────────────┘                              │
│          │                                                                     │
│          ▼                                                                     │
│  Step 4: Company Initialization                                                │
│  Step 5: Request Analysis                                                      │
│                                                                                │
│  Step 6: Capability Resolution (§10.6)                                         │
│          │                                                                     │
│          ▼                                                                     │
│  ┌─────────────────────────────────────────────┐                              │
│  │ ★ COGNITIVE IMMUNE SYSTEM — INOCULATION     │                              │
│  │                                              │                              │
│  │  • Verifica claims nos capability profiles   │                              │
│  │  • Detecta drift entre nível declarado e     │                              │
│  │    performance real (AB-004)                  │                              │
│  │  • Se capability profile mente → alerta      │                              │
│  └─────────────────────────────────────────────┘                              │
│          │                                                                     │
│          ▼                                                                     │
│  Step 7+: Planning, Execution, Review, etc.                                    │
│                                                                                │
│  EM TODAS AS ETAPAS:                                                           │
│  ┌─────────────────────────────────────────────┐                              │
│  │ ★ COGNITIVE IMMUNE SYSTEM — INOCULATION     │                              │
│  │   (Intercepta qualquer escrita de knowledge) │                              │
│  │                                              │                              │
│  │  • learnings.md update → check contradictions │                             │
│  │  • patterns.md update → check consistency    │                              │
│  │  • documentation update → check source       │                              │
│  │  • heuristic registration → check evidence   │                              │
│  └─────────────────────────────────────────────┘                              │
│                                                                                │
└──────────────────────────────────────────────────────────────────────────────┘
```

### 10.2 Ownership e Responsabilidades

```yaml
immune_system_governance:
  primary_owner:
    agent: cosca-security
    role: "Security Chief"
    reason: "Sistema imunológico é defesa — security é o guardião natural. Contaminação de conhecimento é uma vulnerabilidade de segurança cognitiva."
    responsibilities:
      - "Manter o registry de anticorpos atualizado"
      - "Coordenar Vaccination Scans (light, deep, full)"
      - "Decidir sobre promoção de anticorpos a MANDATORY GATES"
      - "Aprovar ou rejeitar novas claims em caso de impasse"
      - "Reportar saúde do immune system ao CTO"

  collaborators:
    - agent: cosca-qa
      role: "Validação de benchmarks"
      reason: "Quando Challenge Phase exige BENCHMARK (evidências iguais), cosca-qa coordena a execução dos testes comparativos"
      responsibilities:
        - "Executar benchmarks quando duas claims têm evidências de peso igual"
        - "Documentar metodologia e resultados do benchmark"
        - "Reportar qual hipótese prevaleceu"

    - agent: cosca-memory-chief
      role: "Registro e curadoria de anticorpos"
      reason: "Immune memory é uma extensão da memory architecture"
      responsibilities:
        - "Manter o schema do registry de anticorpos"
        - "Garantir que anticorpos são persistidos corretamente"
        - "Coordenar deduplicação e merging de anticorpos"
        - "Implementar evolução automática de anticorpos"

    - agent: cosca-discovery
      role: "Verificação de claims contra código real"
      reason: "CONSTITUTION.md P2 define código como verdade absoluta. Discovery Chief é o verificador."
      responsibilities:
        - "Cross-reference claims contra go.mod, filesystem, estrutura de diretórios"
        - "Fornecer evidência de nível CODE para o Challenge Phase"

    - agent: cosca-critic
      role: "Revisão adversarial de decisões do immune system"
      reason: "O immune system pode cometer erros (falsos positivos). O Critic Chief revisa decisões de rejeição."
      responsibilities:
        - "Revisar decisões de REJECT que tenham impacto alto (critical/high severity)"
        - "Validar que anticorpos não estão gerando falsos positivos excessivos"
        - "Sugerir refinamento de regras de detecção"
```

### 10.3 Alimentação do CMI (Cognitive Maturity Index)

O Cognitive Immune System alimenta diretamente a dimensão **Consistência** do CMI:

```yaml
cmi_consistencia_feed:
  metric: "consistency_score"
  weight: 0.10  # 10% do CMI total

  immune_system_contribution:
    sub_metrics:
      contamination_free_rate:
        definition: "% de claims verificadas que passaram na Inoculation sem contradição"
        formula: "claims_approved / total_claims_ingested"
        target: "> 95%"
        current_baseline: 0.95  # Estimado (5 contaminações conhecidas / ~100 claims)

      contamination_resolution_time:
        definition: "Tempo médio entre detecção de contaminação e resolução"
        formula: "SUM(resolution_time) / num_contaminations"
        target: "< 1 hora"
        current_baseline: 0  # Ainda não medido

      antibody_precision:
        definition: "Precisão média dos anticorpos ativos"
        formula: "AVG(true_positives / (true_positives + false_positives))"
        target: "> 0.80"
        current_baseline: 1.00  # Seed antibodies validados contra casos conhecidos

      cognitive_entropy_reduction:
        definition: "Redução no índice de entropia cognitiva (C2) atribuível ao immune system"
        formula: "entropy_before_vaccination - entropy_after_vaccination"
        target: "> 0 por scan"
        current_baseline: 0  # Ainda não medido

    cmi_impact:
      description: "Menos contaminação → maior Consistência → CMI sobe"
      baseline: 92  # Consistência atual (COGNITIVE_MATURITY.md §2.3)
      target_with_immune_system: 96
      delta: +4
```

### 10.4 Integração com Sistemas Existentes

```yaml
integration_points:
  metacognition_pipeline:
    stage_2_retrieve_memory:
      description: "Stage 2 do pipeline de metacognição — antes de recuperar memória para uma task"
      immune_system_role: "Verificar se o conhecimento recuperado está em quarentena. Se sim, NÃO carregar."

    stage_7_extract_pattern:
      description: "Stage 7 do pipeline — após extrair padrão de uma task"
      immune_system_role: "Inoculation: verificar se o padrão extraído contradiz conhecimento existente ANTES de registrá-lo."

  quality_gates:
    gate_0_pre_work:
      description: "Gate 0 — validação antes de começar trabalho"
      immune_system_role: "Verificar se o domínio da task tem conhecimento não contaminado. Se C2 entropy > 25, bloquear e disparar deep scan."

    gate_2_post_implementation:
      description: "Gate 2 — validação pós-implementação"
      immune_system_role: "Verificar se novo código/documentação introduziu contaminação. Inoculation em qualquer artefato novo."

  semantic_memory:
    description: "Motor de busca semântica usado para Contradiction Check"
    immune_system_role: "Usar o mesmo motor de similaridade semântica (cosine) para detectar claims contraditórias"

  evidence_confidence_model:
    description: "Modelo de confiança de evidência (engines/evidence/CONFIDENCE_MODEL.md)"
    immune_system_role: "Usar os mesmos níveis de evidência (CODE, BENCHMARK, AUDIT, ASSERTION) para Source Validation"

  wisdom_decay:
    description: "Decaimento de conhecimento (F1.4)"
    immune_system_role: "Conhecimento antigo tem peso reduzido no Challenge Phase. Claims com TTL expirado são tratadas como ASSERTION."
```

### 10.5 Status no Feature Flag System

```yaml
feature_flag:
  name: "immune.cognitive-system"
  phase: experimental
  metadata:
    owner: "Security Chief"
    created: "2026-07-30"
    description: "Cognitive Immune System — validação proativa de conhecimento"
  defaults:
    development: true
    staging: false
    production: false
  dependencies:
    - flag: "memory.vector-search"
      required_value: true
    - flag: "knowledge.graph"
      required_value: true
  rollout:
    percentage: 0
    target_phase: stable
    expected_stable: "2026-09"  # Fase 2 (Setembro 2026)
```

---

## 11. MÉTRICAS DO SISTEMA IMUNOLÓGICO

```yaml
immune_system_metrics:
  health:
    immune_system_active:
      type: Gauge
      description: "1 se o immune system está ativo, 0 se inativo"

    antibodies_active:
      type: Gauge
      description: "Número de anticorpos ativos"

    antibodies_promoted:
      type: Gauge
      description: "Número de anticorpos promovidos a MANDATORY GATES"

  detection:
    claims_ingested:
      type: Counter
      description: "Total de claims interceptadas na Inoculation Phase"

    claims_approved:
      type: Counter
      description: "Claims aprovadas após os 3 checks"

    claims_rejected:
      type: Counter
      description: "Claims rejeitadas (REJECT no Challenge Phase)"

    claims_quarantined:
      type: Counter
      description: "Claims em quarentena (sem evidência)"

    contradictions_detected:
      type: Counter
      labels: [severity]
      description: "Contradições detectadas por severidade"

  antibodies:
    antibody_triggers:
      type: Counter
      labels: [antibody_id, result]
      description: "Acionamentos de anticorpos (true_positive, false_positive)"

    antibody_precision:
      type: Gauge
      labels: [antibody_id]
      description: "Precisão de cada anticorpo"

  vaccination:
    vaccination_scans:
      type: Counter
      labels: [scan_type]
      description: "Scans de vacinação executados (light, deep, full)"

    contaminations_found:
      type: Counter
      labels: [scan_type, severity]
      description: "Contaminações encontradas por scan"

    scan_duration_ms:
      type: Histogram
      labels: [scan_type]
      description: "Duração dos scans de vacinação"
```

---

## 12. CONSTRAINTS E LIMITAÇÕES

```yaml
constraints:
  performance:
    - "Inoculation Phase não deve adicionar > 500ms de latência à ingestão de conhecimento"
    - "Light Vaccination Scan não deve adicionar > 2s ao bootstrap do Kernel"
    - "Deep Vaccination Scan pode levar até 30s (executado em background)"

  false_positives:
    - "Taxa de falsos positivos deve ser < 10% (precisão > 90%)"
    - "Falsos positivos devem ser registrados e analisados para refinamento de anticorpos"
    - "Nunca rejeitar automaticamente sem registrar o motivo (auditability)"

  knowledge_safety:
    - "Conhecimento rejeitado NUNCA é deletado — é versionado e movido para histórico"
    - "Conhecimento em quarentena é preservado para auditoria futura"
    - "Decisões do immune system são sempre reversíveis (audit trail)"

  authority:
    - "O Don sempre tem autoridade final — pode override de qualquer decisão do immune system"
    - "Overrides do Don são registrados como 'DON_OVERRIDE' no immune memory"
    - "O immune system NÃO pode bloquear comandos diretos do Don"

  scope:
    - "O immune system valida CONHECIMENTO, não código executável"
    - "Claims em código fonte são domínio do compilador/testes, não do immune system"
    - "O immune system complementa — não substitui — testes, linting e code review"
```

---

## 13. WORKFLOW DE ESCALAÇÃO

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                         ESCALATION WORKFLOW                                     │
│                                                                                │
│  IMPASSE NO CHALLENGE                                                          │
│  (Evidências iguais e benchmark inconclusivo)                                   │
│       │                                                                        │
│       ▼                                                                        │
│  ┌─────────────┐                                                               │
│  │ cosca-critic │  Revisão adversarial da decisão                              │
│  │ review       │  "O immune system está certo em ficar em impasse?"           │
│  └──────┬──────┘                                                               │
│         │                                                                      │
│    ┌────┴────┐                                                                 │
│    │         │                                                                 │
│  SIM        NÃO                                                                │
│    │         │                                                                 │
│    ▼         ▼                                                                 │
│  ┌──────┐  ┌──────────┐                                                        │
│  │DON   │  │cosca-qa  │  Re-executar benchmark                                 │
│  │DECIDE│  │re-test   │  com metodologia revisada                               │
│  └──────┘  └──────────┘                                                        │
│                                                                                │
│  ANTICORPO COM ALTA TAXA DE FALSO POSITIVO (>30%)                              │
│       │                                                                        │
│       ▼                                                                        │
│  ┌─────────────┐                                                               │
│  │ cosca-critic │  Analisar falsos positivos                                   │
│  │ analysis     │  Sugerir refinamento da regra                                │
│  └──────┬──────┘                                                               │
│         │                                                                      │
│         ▼                                                                      │
│  ┌─────────────┐                                                               │
│  │ cosca-       │  Atualizar anticorpo (v2)                                    │
│  │ security     │  com regra refinada                                          │
│  └─────────────┘                                                               │
│                                                                                │
│  CONTAMINAÇÃO SISTÊMICA DETECTADA (>10 claims contaminadas)                     │
│       │                                                                        │
│       ▼                                                                        │
│  ┌─────────────┐                                                               │
│  │ DON ALERT   │  Notificação imediata                                         │
│  │ + CTO       │  "Contaminação sistêmica detectada.                           │
│  │             │   Entropia cognitiva: XX.                                     │
│  │             │   Ação recomendada: full audit +                              │
│  │             │   congelar decisões automáticas."                             │
│  └─────────────┘                                                               │
│                                                                                │
└──────────────────────────────────────────────────────────────────────────────┘
```

---

## 14. COMANDOS DO SISTEMA IMUNOLÓGICO

```yaml
immune_system_commands:
  scan:
    - name: "immune scan light"
      description: "Executar light vaccination scan"
      handler: "cosca-security"

    - name: "immune scan deep"
      description: "Executar deep vaccination scan"
      handler: "cosca-security"

    - name: "immune scan full"
      description: "Executar full audit de contaminação"
      handler: "cosca-security"

    - name: "immune scan domain <domain>"
      description: "Scan focado em domínio específico (ex: database, compliance, coverage)"
      handler: "cosca-security"

  antibodies:
    - name: "immune antibodies list"
      description: "Listar todos os anticorpos ativos"
      handler: "cosca-security"

    - name: "immune antibodies show <AB-ID>"
      description: "Mostrar detalhes de um anticorpo específico"
      handler: "cosca-security"

    - name: "immune antibodies promote <AB-ID>"
      description: "Promover anticorpo a MANDATORY GATE"
      handler: "cosca-security (requer aprovação do CTO)"

    - name: "immune antibodies deprecate <AB-ID>"
      description: "Depreciar anticorpo"
      handler: "cosca-security"

  quarantine:
    - name: "immune quarantine list"
      description: "Listar claims em quarentena"
      handler: "cosca-memory-chief"

    - name: "immune quarantine review <claim-id>"
      description: "Revisar e decidir sobre claim em quarentena"
      handler: "cosca-security + cosca-critic"

    - name: "immune quarantine release <claim-id>"
      description: "Liberar claim da quarentena para a base"
      handler: "cosca-security (requer justificativa)"

  report:
    - name: "immune report"
      description: "Gerar Contamination Report completo"
      handler: "cosca-security"

    - name: "immune health"
      description: "Mostrar saúde do immune system (métricas)"
      handler: "cosca-security"
```

---

## 15. REFERÊNCIAS

### Documentos do Framework

| Documento | Caminho | Relação com Immune System |
|-----------|---------|--------------------------|
| COGNITIVE_MATURITY.md | [../../architecture/COGNITIVE_MATURITY.md](../../architecture/COGNITIVE_MATURITY.md) | C5 — Conceito original do Immune System. CMI dimensão Consistência. |
| cognitive-maturity-implementation.md | [../../workflows/cognitive-maturity-implementation.md](../../workflows/cognitive-maturity-implementation.md) | F2.2 — Especificação de implementação e critérios de aceitação |
| KERNEL.md | [../../KERNEL.md](../../KERNEL.md) | §10 — Pipeline de inicialização onde o Immune System se integra |
| CONSTITUTION.md | [../../CONSTITUTION.md](../../CONSTITUTION.md) | P2 — Código executado é verdade absoluta (fundamento do Source Validation) |
| SECURITY_ARCHITECTURE.md | [../../SECURITY_ARCHITECTURE.md](../../SECURITY_ARCHITECTURE.md) | Domínio de segurança — immune system é defesa de segurança cognitiva |
| MEMORY_MODEL.md | [../../MEMORY_MODEL.md](../../MEMORY_MODEL.md) | Modelo de memória — onde o immune system patrulha |
| CONFIDENCE_MODEL.md | [../evidence/CONFIDENCE_MODEL.md](../evidence/CONFIDENCE_MODEL.md) | Níveis de evidência (CODE, BENCHMARK, AUDIT, ASSERTION) |
| QUALITY_GATES.md | [../../QUALITY_GATES.md](../../QUALITY_GATES.md) | Gates 0 e 2 integram verificações do immune system |
| AUTO_EVOLUTION_PROTOCOL.md | [../../shared/AUTO_EVOLUTION_PROTOCOL.md](../../shared/AUTO_EVOLUTION_PROTOCOL.md) | Protocolo de auto-evolução que rege a evolução de anticorpos |
| metacognition-pipeline.md | [../../workflows/metacognition-pipeline.md](../../workflows/metacognition-pipeline.md) | Pipeline de 8 estágios integrado com Inoculation e Quarentena |

### Learnings Relevantes (Evidência dos Casos de Contaminação)

| Agente | Learning | Caso |
|--------|----------|------|
| cosca-kernel | L20 — Systemic Platform Audit (PostgreSQL/K8s/Kafka fantasy) | Casos A, E |
| cosca-kernel | L18 — Coverage Threshold Crisis (4 valores conflitantes) | Caso B |
| cosca-kernel | L22 — Capability Profile Drift (Level 3 vs Level 4) | Caso D |
| cosca-security | 2026-07-28 — Memory Compliance Fix (fictitious GDPR/SOC2) | Caso C |
| cosca-kernel | 2026-07-28 — Documentation Integrity Fix (52 issues em 19 arquivos) | Casos A, B, C, E |
| cosca-kernel | 2026-07-28 — Multi-Phase Documentation Sync (PostgreSQL fantasy pattern) | Casos A, E |

---

## 16. HISTÓRICO

| Versão | Data | Autor | Mudanças |
|---------|------|--------|----------|
| 1.0.0 | 2026-07-30 | Cosca Security Chief | Especificação completa do Cognitive Immune System. 5 fases: Inoculation (3 checks), Challenge (4 resoluções), Antibody Production (5 anticorpos seed), Immune Memory (registry + auto-evolução), Vaccination (3 níveis de scan). 3 exemplos práticos (PostgreSQL, Coverage, Redis). Integração com KERNEL.md §10, CMI, metacognition pipeline, quality gates. Ownership: cosca-security (primary), cosca-qa (benchmarks), cosca-memory-chief (registry). Baseado em 5 casos reais de contaminação documentados. |

---

> **"O sistema imunológico não pergunta 'isso parece verdade?'. Ele pergunta 'isso contradiz algo que já sabemos ser verdade?'."**
>
> — Cosca Security Chief, 2026-07-30
