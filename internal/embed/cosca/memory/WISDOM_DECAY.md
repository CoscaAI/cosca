# WISDOM DECAY — Mecanismo Canônico de Envelhecimento e Revalidação do Conhecimento

> **Versão**: 1.0.0 | **Status**: active | **Owner**: Cosca Memory Chief | **Criado**: 2026-07-30

## Propósito

Conhecimento envelhece. Um aprendizado registrado há 3 meses pode não ser mais válido. O código mudou, a arquitetura evoluiu, o padrão foi substituído. No entanto, todas as entradas em `learnings.md` possuem peso igual, independentemente da idade.

**Wisdom Decay** resolve isso: cada entrada de conhecimento possui uma curva de expiração — a confiança diminui ao longo do tempo até que o conhecimento seja revalidado. Este documento define a especificação canônica desse mecanismo.

---

## 1. Curva de Decaimento

A confiança de um aprendizado decai conforme o tempo desde sua última validação. A curva padrão é:

| Tempo desde última validação | Confiança | Descrição |
|------------------------------|-----------|-----------|
| T = 0 (recém registrado/validado) | 1.00 | Conhecimento fresco, totalmente confiável |
| T = 7 dias | 0.95 | Levemente envelhecido, ainda altamente confiável |
| T = 30 dias | 0.80 | Envelhecimento moderado, requer atenção |
| T = 90 dias | 0.60 | Significativamente envelhecido, revalidação recomendada |
| T = 180 dias | 0.40 | Baixa confiança, provável desatualização |
| T = 365 dias | 0.20 | Confiança mínima, quase certamente obsoleto |

### Fórmula de Decaimento

```
confidence(t, category_multiplier) = max(0.10, 1.0 - (effective_age / max_age) × (1.0 - min_confidence))

Onde:
  effective_age    = t × category_multiplier   (dias desde last_validated ajustado pela categoria)
  max_age          = 365                        (dias para atingir confiança mínima)
  min_confidence   = 0.20                       (piso absoluto de confiança)
  t                = dias desde last_validated  (now - last_validated em dias)

Exemplo (STABLE, 90 dias sem revalidação):
  effective_age = 90 × 1.0 = 90
  confidence = max(0.10, 1.0 - (90 / 365) × 0.80) = max(0.10, 1.0 - 0.197) = 0.803
  → Confiança = 0.80 (arredondada para 2 casas decimais)
```

### Representação Gráfica da Curva

```
Confiança
1.00 ┤●
     │●╲
0.90 ┤ ●╲
     │  ●╲
0.80 ┤   ●╲________● (CRITICAL: 365d → 0.76)
     │    ●╲___________● (STABLE: 365d → 0.20)
0.60 ┤     ●╲
     │      ●╲_________● (EXPERIMENTAL: 182.5d → 0.20)
0.40 ┤       ●╲
     │        ●╲
0.20 ┤         ●╲______________________________
     │
0.10 ┤——————————————————————————————————————————
     0   30   90   180   365   Tempo (dias)
```

---

## 2. Categorias de Conhecimento

Cada aprendizado é classificado em uma das quatro categorias, que determinam a velocidade de decaimento:

### 2.1 CRITICAL — Decaimento Lento (multiplicador ×0.3)

Conhecimento fundamental que raramente se torna obsoleto. Alterações nesta categoria são eventos raros e de alto impacto.

**Critérios de enquadramento:**
- Regras de segurança (ex: OWASP Top 10, políticas de CSP)
- Princípios constitucionais do framework (ex: regras do Kernel, protocolos de governança)
- Decisões de arquitetura imutáveis (ex: ADRs aprovados e não superseded)
- Restrições de compliance (ex: GDPR, LGPD, SOC2)

**Curva de decaimento efetiva (CRITICAL):**
| Tempo real | Tempo efetivo (×0.3) | Confiança |
|------------|----------------------|-----------|
| 7 dias | 2.1 dias | 0.99 |
| 30 dias | 9 dias | 0.98 |
| 90 dias | 27 dias | 0.94 |
| 180 dias | 54 dias | 0.88 |
| 365 dias | 109.5 dias | 0.76 |

### 2.2 STABLE — Decaimento Normal (multiplicador ×1.0)

Conhecimento comprovado que pode envelhecer com mudanças no código ou arquitetura.

**Critérios de enquadramento:**
- Padrões de projeto comprovados
- Decisões de arquitetura estáveis
- Técnicas de implementação validadas
- Configurações de infraestrutura estabelecidas

**Curva de decaimento: igual à curva padrão da Seção 1.**

### 2.3 EXPERIMENTAL — Decaimento Acelerado (multiplicador ×2.0)

Conhecimento novo, não validado, ou em domínio de rápida evolução. Representa hipóteses e descobertas que precisam de verificação frequente.

**Critérios de enquadramento:**
- Novas descobertas (< 30 dias desde a criação)
- Hipóteses não validadas
- Técnicas em tecnologia emergente
- Padrões em fase de experimentação
- Aprendizados de domínio volátil (ex: frontend, AI/ML)

**Curva de decaimento efetiva (EXPERIMENTAL):**
| Tempo real | Tempo efetivo (×2.0) | Confiança |
|------------|----------------------|-----------|
| 7 dias | 14 dias | 0.97 |
| 30 dias | 60 dias | 0.87 |
| 90 dias | 180 dias | 0.60 |
| 180 dias | 360 dias | 0.21 |
| 365 dias | 730 dias | 0.10 (piso) |

### 2.4 DEPRECATED — Não Decai, Sinalizado

Conhecimento que foi substituído ou invalidado. Não participa da curva de decaimento, mas permanece registrado para referência histórica.

**Critérios de enquadramento:**
- Conhecimento explicitamente invalidado por revalidação
- Padrões substituídos por versões mais novas
- Técnicas que se tornaram obsoletas por mudança de stack

**Comportamento:**
- Confiança fixada em 0.00
- Tag `#deprecated` aplicada automaticamente
- Entrada original preservada com nota de obsolescência
- Referência cruzada para o conhecimento substituto (se existir)

---

## 3. Gatilhos de Revalidação

A revalidação pode ser iniciada por quatro mecanismos:

### 3.1 Automático — Limiar de Confiança

Quando a confiança calculada de uma entrada cai abaixo de **0.70**, o sistema agenda automaticamente uma revalidação na próxima inicialização do agente dono do conhecimento.

```
Gatilho: confidence < 0.70
Ação: Agendar revalidação na próxima task do agente
Prioridade: Média (executa antes da task, não bloqueia)
```

### 3.2 Programado — Intervalo por Categoria

Cada categoria possui um intervalo de revalidação programada, independente da confiança atual:

| Categoria | Intervalo de Revalidação | Justificativa |
|-----------|------------------------|---------------|
| CRITICAL | 180 dias (6 meses) | Mudanças são raras, mas devem ser verificadas |
| STABLE | 90 dias (3 meses) | Equilíbrio entre custo de verificação e risco de obsolescência |
| EXPERIMENTAL | 30 dias (1 mês) | Conhecimento volátil requer verificação frequente |
| DEPRECATED | Nunca | Conhecimento arquivado, apenas referência histórica |

### 3.3 Orientado a Eventos

Revalidação disparada quando eventos externos sugerem possível invalidação:

| Evento | Ação |
|--------|------|
| Alteração de código no domínio relacionado | Revalidar todas as entradas com tags do domínio |
| Atualização de padrão relacionado | Revalidar entradas que referenciam o padrão antigo |
| Migration de schema/stack | Revalidar entradas que mencionam a stack migrada |
| Nova ADR que substitui decisão anterior | Revalidar entradas vinculadas à ADR antiga |
| Atualização de versão do framework Cosca | Revalidar todas as entradas CRITICAL e STABLE |

### 3.4 Manual — Revisita ao Domínio

Quando um agente revisita um domínio, deve verificar a confiança das entradas relacionadas antes de aplicá-las. Se `confidence < 0.70`, o agente deve executar a revalidação como parte da task.

```
Regra: Antes de aplicar qualquer aprendizado com confidence < 0.70,
       o agente DEVE executar o processo de revalidação primeiro.
       O custo de aplicar conhecimento obsoleto é maior que o custo de revalidar.
```

---

## 4. Processo de Revalidação

### 4.1 Fluxo Principal

```
                    ┌─────────────────────┐
                    │ Gatilho de          │
                    │ Revalidação         │
                    └────────┬────────────┘
                             │
                             ▼
                    ┌─────────────────────┐
                    │ 1. CARREGAR ENTRADA │
                    │ Ler learnings.md    │
                    │ + failures.md       │
                    └────────┬────────────┘
                             │
                             ▼
                    ┌─────────────────────┐
                    │ 2. EXECUTAR CHECKS  │
                    │ Código ainda bate?  │
                    │ Docs ainda precisos?│
                    │ Padrão ainda funciona?
                    └────────┬────────────┘
                             │
              ┌──────────────┼──────────────┐
              ▼              ▼              ▼
     ┌────────────┐  ┌────────────┐  ┌────────────┐
     │ VÁLIDO     │  │ PARCIAL    │  │ INVÁLIDO   │
     └─────┬──────┘  └─────┬──────┘  └─────┬──────┘
           │               │               │
           ▼               ▼               ▼
     ┌────────────┐  ┌────────────┐  ┌────────────┐
     │ Resetar    │  │ Atualizar  │  │ Marcar como│
     │ confidence │  │ entrada    │  │ DEPRECATED │
     │ = 1.00     │  │ confidence │  │ confidence │
     │ Atualizar  │  │ = 0.85     │  │ = 0.00     │
     │ last_val   │  │ last_val   │  │ Criar      │
     │            │  │            │  │ failure.md │
     └────────────┘  └────────────┘  └────────────┘
```

### 4.2 Passos Detalhados

#### Passo 1: Carregar Entrada
- Localizar a entrada em `learnings.md` pelo timestamp
- Carregar entradas relacionadas em `failures.md`
- Verificar `patterns.md` por padrões vinculados

#### Passo 2: Executar Checks de Validação

| Tipo de Check | Pergunta | Como Verificar |
|---------------|----------|----------------|
| **Código** | O código que gerou este aprendizado ainda existe? | Buscar arquivos referenciados; verificar se a estrutura ainda corresponde |
| **Documentação** | A documentação referenciada ainda é precisa? | Comparar com a versão atual dos docs; verificar se links ainda são válidos |
| **Padrão** | O padrão descrito ainda é aplicável? | Verificar se há padrões substitutos em `patterns.md`; verificar ADRs superseding |
| **Contexto** | O contexto da task original ainda é relevante? | Verificar `context/` e `project/` por mudanças de escopo |
| **Resultado** | O outcome registrado ainda se sustentaria hoje? | Reexecutar mentalmente a técnica; verificar se as pré-condições ainda existem |

#### Passo 3a: Válido → Reset Total
```
Ações:
  1. confidence = 1.00
  2. last_validated = hoje (YYYY-MM-DD)
  3. expires_at = hoje + (365 × category_multiplier) dias
  4. Adicionar nota: "Revalidado em {data}: conhecimento confirmado como preciso"
  5. Registrar evento de revalidação no audit log
```

#### Passo 3b: Parcialmente Válido → Atualização Parcial
```
Ações:
  1. confidence = 0.85
  2. last_validated = hoje (YYYY-MM-DD)
  3. expires_at = hoje + (365 × category_multiplier × 0.5) dias  (reduzido)
  4. Atualizar campos que mudaram (ex: código, referências, notas)
  5. Adicionar nota: "Revalidado em {data}: {o que mudou} — entrada atualizada"
  6. Manter categoria original (não promover para STABLE automaticamente)
  7. Se era EXPERIMENTAL e agora é parcialmente válido, considerar promoção manual para STABLE
```

#### Passo 3c: Inválido → Depreciação
```
Ações:
  1. wisdom_decay_category = DEPRECATED
  2. confidence = 0.00
  3. Adicionar tag #deprecated
  4. Adicionar campo: superseded_by = {timestamp da entrada substituta} (se existir)
  5. Adicionar nota: "Invalidado em {data}: {razão técnica da invalidação}"
  6. Criar entrada em failures.md descrevendo:
     - Por que o conhecimento se tornou inválido
     - O que mudou no sistema que causou a invalidação
     - Qual é o conhecimento substituto (se existir)
     - Lição: como evitar que conhecimento similar envelheça da mesma forma
```

---

## 5. Integração com o Sistema de Memória Existente

### 5.1 Campos Adicionados ao Learning Entry Format

O formato de entrada em `learnings.md` (definido em `LEARNING_PROTOCOL.md`) é estendido com:

```markdown
### {timestamp} — {technique-name}

| Field | Value |
|-------|-------|
| **Agent** | cosca-{name} |
| **Task** | What was being done (context) |
| **Technique** | The specific technique applied |
| **Level** | 1-5 (1=basic, 5=expert) |
| **Outcome** | success / partial / failure |
| **Tags** | #domain #technique #specific-tag |
| **Related** | References to other entries, docs, patterns |
| **Learned** | What was discovered or confirmed |
| **Next** | What to try next time |
| **Wisdom Decay Category** | CRITICAL / STABLE / EXPERIMENTAL / DEPRECATED |
| **Last Validated** | YYYY-MM-DD (data da última revalidação) |
| **Confidence** | 0.00–1.00 (calculado pela curva de decaimento) |
| **Expires At** | YYYY-MM-DD (data estimada em que confidence atinge 0.20) |
```

### 5.2 Atualização do MEMORY_MODEL.md

O modelo de memória é estendido com o diretório de auditoria:

```
internal/embed/cosca/memory/
├── WISDOM_DECAY.md         ← Esta especificação
├── audit/                  ← NOVO: Registros de auditoria de decaimento
│   ├── decay-events.md     ← Log de revalidações executadas
│   ├── expired-entries.md  ← Entradas que atingiram confidence < 0.30
│   └── INDEX.md            ← Índice do sistema de auditoria
├── agent/
│   └── {agent-name}/
│       ├── learnings.md    ← Campos estendidos (last_validated, wisdom_decay_category, confidence)
│       ├── failures.md     ← Depreciações registradas como falhas
│       ├── ...
```

### 5.3 Auditoria Periódica (Decay Audit)

O Memory Chief executa uma auditoria semanal que:

1. **Varredura**: Percorre todas as entradas de `learnings.md` de todos os 54 agentes
2. **Cálculo**: Calcula `confidence` para cada entrada usando `now - last_validated`
3. **Classificação**:
   - `confidence < 0.30` → Entrada expirada: mover para `audit/expired-entries.md`
   - `0.30 ≤ confidence < 0.70` → Alerta: agendar revalidação automática
   - `0.70 ≤ confidence < 0.85` → Atenção: próximo da zona de revalidação
   - `confidence ≥ 0.85` → Saudável
4. **Relatório**: Gera `audit/decay-events.md` com:
   - Total de entradas analisadas
   - Distribuição por categoria e faixa de confiança
   - Entradas expiradas (confidence < 0.30) com recomendação de ação
   - Entradas em zona de alerta (0.30–0.69) agendadas para revalidação
   - Tempo médio desde última validação por categoria

### 5.4 Comportamento dos Agentes

Ao carregar aprendizado para uma task, o agente deve:

```
1. Buscar learnings.md por tags correspondentes ao domínio
2. Para cada entrada encontrada:
   a. Se wisdom_decay_category == DEPRECATED → ignorar (usar substituto se existir)
   b. Se confidence < 0.70 → executar revalidação antes de aplicar
   c. Se confidence ≥ 0.70 → aplicar com a confiança atual como peso
3. Após a task, se aplicou conhecimento com confidence < 0.85, agendar revalidação
```

---

## 6. Exemplo Completo

### 6.1 Entrada de Aprendizado com Campos de Decaimento

Entrada registrada em 2026-07-27 no `learnings.md` do agente `cosca-architecture`:

```markdown
### 2026-07-27 — Orphan Detection via Directory Structure Analysis

| Field | Value |
|-------|-------|
| **Agent** | cosca-memory-chief |
| **Task** | Activation audit: detectar arquivos órfãos no sistema de memória (DNA v2.0 → v3.0) |
| **Technique** | Comparação entre saída de `ls` e referências em INDEX.md — qualquer arquivo não referenciado é órfão |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #memory-chief #audit #orphan-detection #index-health |
| **Related** | memory/INDEX.md, memory/agent/INDEX.md, MEMORY_MODEL.md |
| **Learned** | 9 arquivos órfãos (DNA v2.0) coexistem com 54 diretórios v3.0. Detecção por `ls -p | grep -v /` revela arquivos planos não indexados. |
| **Next** | Automatizar detecção de órfãos com script de auditoria |
| **Wisdom Decay Category** | STABLE |
| **Last Validated** | 2026-07-27 |
| **Confidence** | 1.00 |
| **Expires At** | 2027-07-27 |

**Decay Profile**: Categoria STABLE (×1.0). Confiança inicial: 1.00. Decaimento normal.
```

### 6.2 Cenário: 90 Dias Sem Revalidação

**Data atual: 2026-10-25 (90 dias após registro)**

O algoritmo de decaimento calcula:

```
t = 2026-10-25 - 2026-07-27 = 90 dias
effective_age = 90 × 1.0 = 90 dias  (STABLE)
confidence = max(0.10, 1.0 - (90 / 365) × 0.80)
           = max(0.10, 1.0 - 0.197)
           = 0.803
           → 0.80 (arredondado)
```

**Estado da entrada após 90 dias:**

```markdown
### 2026-07-27 — Orphan Detection via Directory Structure Analysis

| Field | Value |
|-------|-------|
| **Agent** | cosca-memory-chief |
| **Task** | Activation audit: detectar arquivos órfãos no sistema de memória (DNA v2.0 → v3.0) |
| **Technique** | Comparação entre saída de `ls` e referências em INDEX.md — qualquer arquivo não referenciado é órfão |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #memory-chief #audit #orphan-detection #index-health |
| **Related** | memory/INDEX.md, memory/agent/INDEX.md, MEMORY_MODEL.md |
| **Learned** | 9 arquivos órfãos (DNA v2.0) coexistem com 54 diretórios v3.0. Detecção por `ls -p | grep -v /` revela arquivos planos não indexados. |
| **Next** | Automatizar detecção de órfãos com script de auditoria |
| **Wisdom Decay Category** | STABLE |
| **Last Validated** | 2026-07-27 |
| **Confidence** | 0.80 ⚠️ (decaiu de 1.00 em 90 dias) |
| **Expires At** | 2027-07-27 |

**Decay Status**: 90 dias sem revalidação. Confiança = 0.80.
**Ação**: Acionar revalidação programada (STABLE: 90 dias). Verificar se:
- O padrão de detecção de órfãos ainda funciona?
- Existem novos tipos de órfãos não cobertos?
- O script de automação (Next) foi implementado? Se sim, esta entrada é parcialmente obsoleta.
```

### 6.3 O Que Acontece na Revalidação (3 Cenários)

#### Cenário A: Conhecimento Ainda Válido

O auditor verifica que:
- O padrão `ls -p | grep -v /` ainda detecta órfãos corretamente
- Nenhum novo tipo de órfão surgiu
- O script de automação ainda não foi implementado (Next permanece relevante)

**Resultado:**
```
confidence = 1.00 (reset total)
last_validated = 2026-10-25
Nota: "Revalidado em 2026-10-25: padrão de detecção de órfãos confirmado como preciso"
```

#### Cenário B: Conhecimento Parcialmente Válido

O auditor verifica que:
- O padrão básico ainda funciona
- Mas o script de automação já foi implementado (campo Next obsoleto)
- Um novo tipo de órfão (symlinks quebrados) não é coberto pela técnica original

**Resultado:**
```
confidence = 0.85 (reset parcial)
last_validated = 2026-10-25
Nota: "Revalidado em 2026-10-25: padrão base mantido. Script de automação implementado (Next resolvido). Symlinks quebrados são nova classe de órfãos não cobertos — próxima iteração deve incluir `find -L -type l`."
Next atualizado: "Estender detecção para symlinks quebrados com `find -L -type l`"
```

#### Cenário C: Conhecimento Totalmente Invalidado

O auditor verifica que:
- O sistema de memória migrou para um banco SQLite com FTS5
- Não existem mais arquivos Markdown planos — tudo está em `.cosca/knowledge.db`
- O padrão `ls -p | grep -v /` é completamente irrelevante

**Resultado:**
```
wisdom_decay_category = DEPRECATED
confidence = 0.00
Tags: adicionado #deprecated
superseded_by = 2026-10-25 — SQLite FTS5 Orphan Query
Nota: "Invalidado em 2026-10-25: migração para knowledge.db eliminou arquivos planos. Detecção de órfãos agora é feita via query SQL: SELECT key FROM memory_entries WHERE key NOT IN (SELECT target FROM cross_references)."

Entrada em failures.md:
  Failure: Orphan Detection Pattern — invalidado por migração de storage
  Root Cause: Migração de Markdown plano para SQLite FTS5 tornou o padrão baseado em filesystem obsoleto
  Lesson: Padrões acoplados ao storage físico devem ser marcados como EXPERIMENTAL
  Avoidance Pattern: Preferir padrões baseados em contratos lógicos (ex: "entidades não referenciadas")
    sobre padrões baseados em implementação física (ex: "arquivos não listados no índice")
```

---

## 7. Regras de Transição de Categoria

| Transição | Condição | Ação |
|-----------|----------|------|
| EXPERIMENTAL → STABLE | 3 revalidações consecutivas com confidence ≥ 0.85 e nenhuma correção necessária | Promover categoria, resetar last_validated |
| STABLE → CRITICAL | ADR aprovada que estabelece o conhecimento como princípio constitucional | Promover categoria, revisão por Governance Chief |
| STABLE → EXPERIMENTAL | Mudança significativa no domínio (stack, arquitetura, API) | Rebaixar categoria, acelerar decaimento |
| Qualquer → DEPRECATED | Revalidação resulta em inválido | Depreciar, criar failure, link para substituto |
| DEPRECATED → STABLE | Nunca automático — requer recriação como nova entrada | Criar nova entrada com referência à antiga |

---

## 8. Métricas e Monitoramento

### 8.1 Métricas do Sistema de Decaimento

| Métrica | Definição | Alvo |
|---------|-----------|------|
| **Mean Time Between Validation (MTBV)** | Tempo médio entre revalidações por categoria | CRITICAL ≤ 180d, STABLE ≤ 90d, EXPERIMENTAL ≤ 30d |
| **Staleness Ratio** | Entradas com confidence < 0.70 / Total de entradas | < 10% |
| **Expired Ratio** | Entradas com confidence < 0.30 / Total de entradas | < 2% |
| **Deprecation Rate** | Entradas depreciadas por mês | Informativo (tendência) |
| **Revalidation Success Rate** | Revalidações com resultado "válido" / Total de revalidações | > 70% (se menor, categorias mal classificadas) |
| **Category Distribution** | Distribuição de entradas entre CRITICAL/STABLE/EXPERIMENTAL/DEPRECATED | Monitorar desbalanceamento |

### 8.2 Alertas

| Condição | Severidade | Ação |
|-----------|------------|------|
| Staleness Ratio > 20% | ALTO | Auditoria completa de revalidação em todos os agentes |
| Expired Ratio > 5% | CRÍTICO | Limpeza de entradas expiradas; revisão de categorização |
| EXPERIMENTAL > 50% do total | MÉDIO | Muitas entradas não validadas; revisar critério de promoção |
| MTBV STABLE > 120 dias | MÉDIO | Revalidações atrasadas; verificar agendamento |
| 0 revalidações em 30 dias | ALTO | Sistema de auditoria pode estar quebrado |

---

## 9. Migração de Entradas Existentes

Todas as entradas em `learnings.md` criadas antes de 2026-07-30 devem ser migradas:

### 9.1 Regras de Migração

1. **last_validated** = data de criação da entrada (campo timestamp)
2. **wisdom_decay_category** = determinado por heurística:
   - Se tags contêm `#security`, `#compliance`, `#kernel`, `#constitutional` → CRITICAL
   - Se tags contêm `#experimental`, `#hypothesis`, `#discovery`, `#new` → EXPERIMENTAL
   - Se tags contêm `#baseline`, `#seed`, `#initialization` → EXPERIMENTAL (conhecimento seed não validado)
   - Demais casos → STABLE
3. **confidence** = calculado pela curva de decaimento a partir de `last_validated` até hoje
4. **expires_at** = `last_validated + (365 × category_multiplier)` dias

### 9.2 Exemplo de Migração

Entrada original (2026-07-27, sem campos de decaimento):

```markdown
### 2026-07-27 — Baseline
| **Agent** | cosca-memory-chief |
| **Outcome** | success |
| **Tags** | #memory-chief #baseline #initialization |
```

Após migração automática (2026-07-30):

```markdown
### 2026-07-27 — Baseline
| **Agent** | cosca-memory-chief |
| **Outcome** | success |
| **Tags** | #memory-chief #baseline #initialization |
| **Wisdom Decay Category** | EXPERIMENTAL |
| **Last Validated** | 2026-07-27 |
| **Confidence** | 0.99 |
| **Expires At** | 2027-10-25 |
```

> Nota: Categoria EXPERIMENTAL porque tags contêm `#baseline` e `#initialization` — conhecimento seed, não validado em task real.

---

## 10. Governança

### 10.1 Propriedade

- **Owner**: Cosca Memory Chief (`cosca-memory-chief`)
- **Revisor**: Cosca Governance Chief (`cosca-governance`) — aprova mudanças de categoria CRITICAL
- **Executor da Auditoria**: Cosca Memory Chief — auditoria semanal de decaimento
- **Consumidores**: Todos os 54 agentes Cosca — aplicam confiança ao carregar aprendizado

### 10.2 Versionamento

| Versão | Data | Mudança |
|--------|------|---------|
| 1.0.0 | 2026-07-30 | Especificação inicial do mecanismo Wisdom Decay |

### 10.3 Changelog

- **v1.0.0** (2026-07-30): Criado por ordem do Don. Define curva de decaimento, 4 categorias de conhecimento, 4 gatilhos de revalidação, processo de revalidação de 3 resultados, integração com LEARNING_PROTOCOL.md, auditoria periódica, métricas e migração de entradas existentes.

---

## Referências

- [LEARNING_PROTOCOL.md](LEARNING_PROTOCOL.md) — Formato de entrada de aprendizado (estendido por este documento)
- [MEMORY_MODEL.md](MEMORY_MODEL.md) — Modelo de memória do Kernel (estendido com audit/)
- [AUTO_EVOLUTION_PROTOCOL.md](../shared/AUTO_EVOLUTION_PROTOCOL.md) — Protocolo de auto-evolução dos agentes
- [AGENTS.md](../../AGENTS.md) — Política de descoberta de agentes
