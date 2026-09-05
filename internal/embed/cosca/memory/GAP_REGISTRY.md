# GAP REGISTRY — Registro Global de Lacunas de Conhecimento

> **Versão**: 1.0.0 | **Status**: active | **Owner**: Cosca Context Chief + Cosca Discovery Chief | **Criado**: 2026-07-30
>
> **Autoridade**: [engines/gap-detection/SKILL.md](../engines/gap-detection/SKILL.md) — Toda lacuna detectada é registrada aqui.
>
> **Propósito**: Este arquivo é o catálogo central de todas as lacunas de conhecimento conhecidas no sistema Cosca. Ele existe para garantir que o sistema nunca esqueça o que não sabe — e para rastrear o progresso na resolução dessas lacunas ao longo do tempo.
>
> **Lema**: *"O primeiro passo para saber é admitir o que não se sabe."*

---

## ESTRUTURA DO REGISTRO

O registro é organizado por status e tipo de lacuna:

```
GAP_REGISTRY.md
├── MÉTRICAS GERAIS           ← Visão agregada de todas as lacunas
├── LACUNAS ATIVAS            ← Lacunas não resolvidas (por tipo)
│   ├── GAP_KNOWLEDGE         ← Lacunas de conhecimento técnico
│   ├── GAP_CONTEXT           ← Lacunas de contexto do projeto
│   ├── GAP_RISK              ← Riscos não avaliados
│   ├── GAP_DEPENDENCY        ← Dependências desconhecidas
│   └── GAP_ASSUMPTION        ← Suposições não verificadas
├── LACUNAS RESOLVIDAS        ← Histórico de lacunas resolvidas (com lições)
├── LACUNAS ESCALADAS         ← Lacunas enviadas ao Don (com resposta)
├── PADRÕES DE LACUNA         ← Análise de tipos recorrentes
└── PLANO DE RESOLUÇÃO        ← Próximas lacunas a resolver (prioridade)
```

---

## MÉTRICAS GERAIS

```yaml
metrics:
  last_updated: "2026-07-30T00:00:00Z"
  total_gaps_registered: 3
  by_status:
    active: 3
    resolved: 0
    escalated: 0
    documented: 0
    training_needed: 0
  by_type:
    GAP_KNOWLEDGE: 2
    GAP_CONTEXT: 1
    GAP_RISK: 0
    GAP_DEPENDENCY: 0
    GAP_ASSUMPTION: 0
  by_severity:
    CRITICAL: 0
    HIGH: 0
    MEDIUM: 2
    LOW: 1
  by_domain:
    gRPC: 1
    frontend_react: 1
    embed_management: 1
  resolution_rate: "0%"  # 0 resolved / 3 total
  average_time_to_resolve: "N/A"
  gaps_prevented_incidents: 0  # Estimativa de incidentes evitados
  gap_failures: 1  # Lacunas que causaram falhas (L13)
```

---

## LACUNAS ATIVAS

### GAP_KNOWLEDGE — Lacunas de Conhecimento Técnico

```yaml
- gap_id: "GAP-2026-001"
  status: "ACTIVE"
  registered: "2026-07-29"
  last_reviewed: "2026-07-30"
  domain: "gRPC internals"
  type: "GAP_KNOWLEDGE"
  severity: "MEDIUM"
  description: >
    Não conheço a fundo a API de streaming bidirecional do gRPC em Go.
    O Cosca usa gRPC para comunicação entre runtime e plugins WASM.
    Preciso entender: server-streaming, client-streaming, bidirectional,
    deadlines, cancellation, e interceptors no contexto do Cosca.
  detected_by: "Cosca Kernel"
  task_context: "Auto-avaliação de confiança — uncertainty_topics"
  resolution:
    strategy: "AUTO"
    plan: >
      1. Ler exemplos de gRPC no código Cosca (internal/grpc/)
      2. Estudar api/grpc/ para entender os serviços definidos
      3. Executar descoberta para mapear uso real de streaming
      4. Criar learning entry em memory/agent/cosca-kernel/learnings.md
    estimated_effort: "2 horas"
    blocker: false
  confidence_impact:
    base_confidence: 0.91
    domain_confidence: 0.55  # Confiança específica em gRPC
    target_confidence: 0.80  # Meta pós-resolução

- gap_id: "GAP-2026-002"
  status: "ACTIVE"
  registered: "2026-07-29"
  last_reviewed: "2026-07-30"
  domain: "frontend React components"
  type: "GAP_KNOWLEDGE"
  severity: "LOW"
  description: >
    Não conheço a estrutura do design system de componentes React no
    frontend Next.js 15 do Cosca (272 arquivos .tsx, 27 módulos).
    Para tasks de frontend, preciso delegar ao Frontend Chief.
  detected_by: "Cosca Kernel"
  task_context: "Auto-avaliação de confiança — uncertainty_topics"
  resolution:
    strategy: "DELEGATE"
    plan: >
      1. Quando necessário, delegar ao Frontend Chief (cosca-frontend)
      2. Frontend Chief tem capability profile com confiança em React/Next.js
      3. Após delegação bem-sucedida, registrar padrão de colaboração
    estimated_effort: "Delegação: 30s. Aprendizado contínuo: background."
    blocker: false
  confidence_impact:
    base_confidence: 0.91
    domain_confidence: 0.40  # Confiança baixa em frontend
    target_confidence: 0.60  # Meta: suficiente para delegar com contexto
```

### GAP_CONTEXT — Lacunas de Contexto do Projeto

```yaml
- gap_id: "GAP-2026-003"
  status: "ACTIVE"
  registered: "2026-07-29"
  last_reviewed: "2026-07-30"
  domain: "embed_management"
  type: "GAP_CONTEXT"
  severity: "MEDIUM"
  description: >
    Não tenho verificação automática da sincronização entre
    internal/embed/cosca/ (build) e internal/embed/cosca/ (fonte) durante
    o bootstrap. O L13 provou que essa lacuna de contexto pode causar
    regressão de 11+ arquivos do framework.
  detected_by: "Cosca Kernel"
  task_context: "Post-mortem do incidente L13 (jail breach)"
  incident_related: "L13-JAIL-BREACH"
  resolution:
    strategy: "AUTO"
    plan: >
      1. Adicionar verificação de timestamp no bootstrap (Step 3: Memory Loading)
      2. Comparar hash/timestamp dos arquivos em internal/embed/cosca/ vs internal/embed/cosca/
      3. Se discrepância > 5 minutos, emitir GAP_CONTEXT warning
      4. Sugerir make embed-sync antes de qualquer operação destrutiva
    estimated_effort: "4 horas (implementar verificação + testes)"
    blocker: false  # Não bloqueia operação, mas emite warning
  confidence_impact:
    base_confidence: 0.85  # Confiança em operações de init
    domain_confidence: 0.50  # Abaixo do threshold sem verificação
    target_confidence: 0.90  # Meta: verificação automática resolve esta lacuna
  safety_note: >
    Enquanto esta lacuna existir, toda operação cosca init --force deve
    passar por gap scan que detecta GAP_CONTEXT sobre estado do embed.
```

### GAP_RISK — Riscos Não Avaliados

```yaml
# Nenhuma lacuna de risco ativa no momento.
# O incidente L13 gerou lacunas de risco que foram resolvidas preventivamente:
# - "O que acontece se init --force rodar com embed desatualizado?"
#   → Resposta: Regressão de arquivos. Mitigação: make embed-sync antes de init.
# - "O jail deve ser desabilitado para init --force?"
#   → Resposta: Não. Jail é proteção, não obstáculo. Respeitar COSCA_JAILED.
```

### GAP_DEPENDENCY — Dependências Desconhecidas

```yaml
# Nenhuma lacuna de dependência ativa no momento.
# Lições do L13:
# - "make embed-sync é pré-requisito de init?" → Sim. P8 da Constituição.
# - "init --force requer sudo?" → Depende do contexto de instalação.
# - Estas lacunas agora são detectadas automaticamente pelo gap scan.
```

### GAP_ASSUMPTION — Suposições Não Verificadas

```yaml
# Nenhuma lacuna de suposição ativa no momento.
# Suposições perigosas identificadas no L13 e agora verificadas:
# - "Binário contém versão mais recente do framework"
#   → VERIFICAÇÃO: Comparar timestamps embed vs fonte no bootstrap.
# - "COSCA_JAILED=1 é condição normal"
#   → VERIFICAÇÃO: Jail só desabilitado com ordem explícita do Don.
```

---

## LACUNAS RESOLVIDAS

```yaml
# Nenhuma lacuna formalmente resolvida ainda.
# O registro foi criado em 2026-07-30.
# Lacunas serão movidas para esta seção quando seu status mudar para RESOLVED.
```

---

## LACUNAS ESCALADAS

```yaml
# Nenhuma lacuna escalada ao Don ainda.
# O protocolo de escalação (gap-detection/SKILL.md §2.3) define:
# - Formato de pergunta com opções A/B/C
# - Contexto: o que não sei, por que importa, o que já tentei
# - Sempre oferecer recomendação com justificativa
```

---

## PADRÕES DE LACUNA

### Análise de Recorrência

```yaml
pattern_analysis:
  last_updated: "2026-07-30"

  # Padrões identificados (threshold: > 3 ocorrências em 30 dias)
  patterns: []

  # Observações iniciais:
  observations:
    - "GAP_CONTEXT sobre estado do embed é a lacuna mais perigosa (causou L13)"
    - "GAP_KNOWLEDGE tende a ser domínio-específica (gRPC, frontend)"
    - "GAP_ASSUMPTION tende a aparecer quando o Kernel está com pressa"

  # Recomendações baseadas em padrões (quando houver dados suficientes):
  recommendations: []
```

---

## PLANO DE RESOLUÇÃO

### Próximas Lacunas a Resolver (Ordenado por Prioridade)

| # | Gap ID | Tipo | Severidade | Domínio | Esforço | Bloqueia? |
|---|--------|------|-----------|---------|---------|-----------|
| 1 | GAP-2026-003 | GAP_CONTEXT | MEDIUM | embed_management | 4h | Não (warning) |
| 2 | GAP-2026-001 | GAP_KNOWLEDGE | MEDIUM | gRPC internals | 2h | Não |
| 3 | GAP-2026-002 | GAP_KNOWLEDGE | LOW | frontend React | Background | Não |

### Estratégia de Resolução

1. **GAP-2026-003 (embed_management)** — Prioridade máxima apesar de MEDIUM porque:
   - Foi a causa raiz do pior incidente do sistema (L13)
   - Resolver esta lacuna previne regressão futura do framework
   - Implementação é relativamente simples (verificação de timestamp/hash)

2. **GAP-2026-001 (gRPC internals)** — Prioridade média porque:
   - gRPC é usado em comunicação runtime-plugin (crítico para funcionalidade)
   - Confiança atual (0.55) está no limite do threshold de delegação
   - Melhorar para 0.80 permitiria tomada de decisão autônoma em tasks gRPC

3. **GAP-2026-002 (frontend React)** — Prioridade baixa porque:
   - Frontend é delegado ao Frontend Chief por design
   - Kernel não implementa frontend (KERNEL.md: "NOT a UI framework")
   - Delegação é a estratégia correta e já está documentada

---

## LIÇÕES DO INCIDENTE L13

### O Que o GAP_REGISTRY Teria Prevenido

Se o GAP_REGISTRY existisse em 2026-07-29, o incidente L13 teria sido prevenido porque:

1. **GAP-2026-003** (verificação de embed) teria sido detectada no primeiro bootstrap
2. O gap scan pré-execução teria identificado 7 lacunas (2 CRITICAL, 3 HIGH)
3. A effective_confidence teria caído de 0.85 para 0.425
4. O Kernel teria escalado ao Don em vez de executar
5. O Don teria autorizado `make embed-sync` antes de `init --force`
6. Zero arquivos regredidos. Zero incidentes.

### Compromisso

> *"Este registro existe para que o L13 nunca se repita. Cada lacuna documentada aqui é uma bala desviada. Cada lacuna resolvida é uma parede construída. O objetivo não é nunca ter lacunas — é nunca ser surpreendido por elas."*
>
> — Cosca Kernel, 2026-07-30

---

## FORMULÁRIO DE REGISTRO DE NOVA LACUNA

Use este template ao registrar uma nova lacuna detectada pelo [Gap Detection Engine](../engines/gap-detection/SKILL.md):

```yaml
- gap_id: "GAP-YYYY-NNN"
  status: "ACTIVE"
  registered: "YYYY-MM-DD"
  last_reviewed: "YYYY-MM-DD"
  domain: "nome_do_dominio"
  type: "GAP_KNOWLEDGE | GAP_CONTEXT | GAP_RISK | GAP_DEPENDENCY | GAP_ASSUMPTION"
  severity: "CRITICAL | HIGH | MEDIUM | LOW"
  description: >
    Descrição clara e específica do que não se sabe.
    Incluir contexto: por que esta lacuna importa?
  detected_by: "agent-name"
  task_context: "Qual task revelou esta lacuna?"
  incident_related: "ID do incidente (se aplicável)"
  resolution:
    strategy: "AUTO | DELEGATE | ESCALATE | DOCUMENT"
    plan: >
      Passos concretos para resolver a lacuna.
      Incluir fontes de informação (código, docs, agentes).
    estimated_effort: "tempo estimado"
    blocker: true | false
  confidence_impact:
    base_confidence: 0.XX
    domain_confidence: 0.XX
    target_confidence: 0.XX
  safety_note: >
    Se aplicável: o que fazer enquanto a lacuna não é resolvida?
```

---

## RELACIONADOS

- [engines/gap-detection/SKILL.md](../engines/gap-detection/SKILL.md) — Engine de detecção proativa de lacunas
- [memory/context/cognitive-state.md](context/cognitive-state.md) — Estado cognitivo com gaps ativos
- [CONSTITUTION.md](../CONSTITUTION.md) — Princípios imutáveis (P2, P4, P5, P8)
- [LEARNING_PROTOCOL.md](LEARNING_PROTOCOL.md) — Como lacunas resolvidas viram aprendizado
- [CONFIDENCE_MODEL.md](../engines/evidence/CONFIDENCE_MODEL.md) — Modelo de confiança afetado por lacunas
- [MEMORY_MODEL.md](MEMORY_MODEL.md) — Onde este registro se encaixa no sistema de memória

---

## HISTÓRICO

| Versão | Data | Autor | Alterações |
|---------|------|--------|-----------|
| 1.0.0 | 2026-07-30 | Cosca Context Chief + Cosca Discovery Chief | Criação inicial. 3 lacunas ativas registradas (GAP-2026-001 a GAP-2026-003). Lições do L13 documentadas. Template de registro definido. Métricas iniciais: 0% resolvido, 1 gap failure registrado. |
