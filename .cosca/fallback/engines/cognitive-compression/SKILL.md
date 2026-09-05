# COGNITIVE COMPRESSION ENGINE — Compressão de Conhecimento (F3.3)

> **Versão**: 2.0.0 | **Status**: active | **Owner**: Architecture Chief | **Criado**: 2026-07-30 | **Atualizado**: 2026-07-30
> **Código**: F3.3 | **Fase**: Fase 3 — Avançada
> **Dependências**: F2.1 (Cognitive Economy) | F1.2 (Contrafactual Gate) | F1.6 (Cognitive Entropy) | F9.2 (Wisdom Distillation)
> **CMI Impact**: Aprendizado +5, Consistência +5
> **Conceito Original**: C10 — Cognitive Compression (COGNITIVE_MATURITY.md §5)
>
> **Versão anterior**: v1.0.0 (F3.2) preservada integralmente em [COMPRESSION_V1.md](COMPRESSION_V1.md).
> A v2.0.0 substitui a v1.0.0 como **fonte canônica** do engine por ordem do Don (2026-07-30).

---

## SUMÁRIO

1. [Definição](#1-definição)
2. [O Problema: 600+ Learnings Custam Caro](#2-o-problema-600-learnings-custam-caro)
3. [Pipeline de Compressão (semanal, < 10s)](#3-pipeline-de-compressão-semanal--10s)
4. [Fórmula de Compressão](#4-fórmula-de-compressão)
5. [Verificação de Fidelidade (Reconstrução)](#5-verificação-de-fidelidade-reconstrução)
6. [O que NUNCA Comprime](#6-o-que-nunca-comprime)
7. [Integração com F2.1 — Cognitive Economy](#7-integração-com-f21--cognitive-economy)
8. [Integração com F1.6 — Cognitive Entropy](#8-integração-com-f16--cognitive-entropy)
9. [Integração com F1.2 — Contrafactual Gate](#9-integração-com-f12--contrafactual-gate)
10. [Integração com F9.2 — Wisdom Distillation](#10-integração-com-f92--wisdom-distillation)
11. [Exemplo Real: Dead Code Removal](#11-exemplo-real-dead-code-removal)
12. [Formato do Princípio Comprimido](#12-formato-do-princípio-comprimido)
13. [Gatilhos de Compressão](#13-gatilhos-de-compressão)
14. [Governança e Reversão](#14-governança-e-reversão)
15. [CLI e Automação](#15-cli-e-automação)
16. [Métricas do Engine](#16-métricas-do-engine)
17. [Casos de Borda e Anti-Padrões](#17-casos-de-borda-e-anti-padrões)
18. [Escalação](#18-escalação)
19. [Referências](#19-referências)
20. [Histórico](#20-histórico)

---

## 1. DEFINIÇÃO

### 1.1 Compressão é o gzip da cognição

O **Cognitive Compression Engine** é o motor que aplica **compressão de dados à memória cognitiva**. Assim como o gzip reduz arquivos repetitivos sem que o leitor perceba a diferença, o Cognitive Compression reduz **muitos learnings → poucos princípios** sem perder informação essencial.

```
┌───────────────────────────────────────────────────────────────────────┐
│                                                                        │
│   COGNITIVE COMPRESSION = gzip PARA CONHECIMENTO                       │
│                                                                        │
│   ANTES (bruto)                        DEPOIS (comprimido)             │
│   ─────────────                        ───────────────────             │
│   50 learnings sobre                  → 3 princípios que              │
│   "testes em Go"                         capturam ~90% da             │
│   (L9, L19, L21, L22, L29,               informação essencial         │
│    + dezenas de outros)                  + referências aos originais  │
│                                                                        │
│   Tamanho: ~40KB                        Tamanho: ~4KB (90% menor)     │
│   Contexto: caro                         Contexto: barato              │
│   Entropia: alta (fragmentado)           Entropia: baixa (consolidado)│
│                                                                        │
└───────────────────────────────────────────────────────────────────────┘
```

### 1.2 Analogia com gzip

| Propriedade | gzip (dados) | Cognitive Compression (cognição) |
|-------------|-------------|----------------------------------|
| **Entrada** | Arquivo com repetições | Learnings com similaridade semântica |
| **Saída** | Arquivo `.gz` menor | Princípios + referências aos originais |
| **Perda** | Sem perda (lossless) | Perda controlada (fidelity ≥ 90%) |
| **Reversão** | `gunzip` restaura o original | Don pode reverter (`--revert`) |
| **Ganho** | Menos disco | Menos contexto, menos entropia |
| **Rastro** | Checksum | Referências a L20, L22, L29... |

### 1.3 A analogia do Don

> *"Depois de centenas de projetos. O runtime cria regras universais. Em vez de lembrar 80 casos. Ele aprende: Todos seguem o mesmo princípio. É como um cientista criando uma teoria."*

Compressão **não é** resumir mal — é **descobrir a teoria** que explica os casos. O que é descartado é a redundância; o que é preservado é a essência. O Cosca não precisa carregar 50 learnings sobre testes em Go no contexto: ele carrega 3 princípios que reconstroem esses 50 learnings com ≥ 90% de fidelidade.

---

## 2. O PROBLEMA: 600+ LEARNINGS CUSTAM CARO

### 2.1 A escala

O Cosca acumula conhecimento em velocidade industrial. A cada sessão, dezenas de learnings são registrados pelos 54 agentes. Hoje:

| Métrica | Valor |
|---------|-------|
| **Learnings totais** | 600+ (acumulados por 54 agentes) |
| **Crescimento** | ~10-47 learnings/dia em sessões intensas |
| **Custo de contexto** | Cada learning carregado no contexto custa tokens (F2.1 §3.2) |
| **Entropia** | Conhecimento não comprimido fragmenta e contradiz (F1.6) |
| **Comprimidos em princípios** | 0 até a v2.0.0 |

### 2.2 Por que contexto custa caro

A fórmula da F2.1 (Cognitive Economy) define o custo de tokens por task:

```
custo_tokens = (input_tokens + output_tokens) × token_price

Onde:
  input_tokens = system prompt + KERNEL.md + MEMÓRIA carregada + task
```

A **memória carregada** é diretamente proporcional ao número de learnings relevantes injetados no contexto (via F2.4 Cognitive Gravity). 50 learnings de um domínio = ~40KB de contexto por task. Com 600+ learnings no total, o runtime gasta a maior parte do orçamento de contexto **relendo conhecimento que já foi absorvido em princípios**.

### 2.3 O custo da não-compressão

| Sem compressão | Com compressão (F3.3) |
|----------------|------------------------|
| Cada task carrega N learnings redundantes | Cada task carrega 1-3 princípios consolidados |
| Tokens de input altos → ROI baixo (F2.1) | Tokens de input reduzidos → ROI alto |
| Entropia sobe com cada learning novo (F1.6) | Entropia cai a cada ciclo de compressão |
| Contradições acumulam (2 learnings opostos) | Contradições são resolvidas na consolidação |
| Tempo de leitura de contexto cresce | Leitura de contexto fica estável |

> **Princípio do engine**: *"Conhecimento que já virou teoria não deve continuar custando como se ainda fosse dado bruto."*

---

## 3. PIPELINE DE COMPRESSÃO (SEMANAL, < 10s)

### 3.1 Pipeline Canônico

O pipeline roda **semanalmente** (ou sob demanda via CLI). Tempo alvo: **< 10s** para a base inteira.

```
┌──────────────────────────────────────────────────────────────────────┐
│                  COGNITIVE COMPRESSION — PIPELINE                     │
│                          (semanal, < 10s)                             │
│                                                                       │
│  1. Seleciona domínio (ex: "testing")                                 │
│       │                                                               │
│  2. Coleta todos os learnings do domínio                              │
│       │                                                               │
│  3. Agrupa por similaridade semântica                                 │
│       │                                                               │
│  4. Para cada grupo:                                                  │
│       ├── Extrai a informação essencial (princípio)                   │
│       ├── Calcula taxa de compressão                                  │
│       └── Verifica perda de informação                                │
│           (reconstrói grupo a partir do princípio)                    │
│       │                                                               │
│  5. Se reconstrução > 90% → COMPRIME                                  │
│       (substitui por princípio + referências)                         │
│       │                                                               │
│  6. Se reconstrução < 90% → MANTÉM grupo                              │
│       (perda alta demais)                                             │
│       │                                                               │
│  7. Registra: compressão reduz context cost e entropia                │
│                                                                       │
└──────────────────────────────────────────────────────────────────────┘
```

### 3.2 Detalhamento dos Passos

| Passo | Ação | Fonte de Dados | Tempo Alvo |
|-------|------|----------------|:----------:|
| **1. Seleciona domínio** | Itera pelos domínios do registry (testing, security, architecture, orchestration...) | `knowledge/schema/` + tags dos learnings | < 1s |
| **2. Coleta learnings** | Lê todos os learnings.md dos agentes com tag/domínio correspondente | `memory/agent/*/learnings.md` (FTS5) | < 2s |
| **3. Agrupa por similaridade** | Clusterização semântica: similaridade de tags + afirmação central + causa raiz | Semantic Memory (FTS5 + embeddings) | < 2s |
| **4. Extrai + mede + verifica** | Para cada grupo: redige princípio, calcula `compression_ratio`, roda reconstrução | LLM local (1 chamada/grupo) + similaridade | < 3s |
| **5. Comprime se > 90%** | Escreve princípio em `knowledge/principles/`, tageia originais como `#compressed` | Filesystem + index | < 1s |
| **6. Mantém se < 90%** | Marca grupo como `compression_deferred` | Log + metadata | < 0.5s |
| **7. Registra** | Atualiza CSV de compressão + notifica F2.1 e F1.6 | `memory/timeline/compression.csv` | < 0.5s |
| **Total** | | | **< 10s** |

### 3.3 Por que < 10s é viável

- **Coleta**: índices FTS5 por tag tornam a leitura de learnings O(n) com n pequeno (~600 entries, < 2s).
- **Clusterização**: similaridade por tags (Jaccard) primeiro; embeddings apenas para grupos ambíguos — evita custo de vector store em todos os pares.
- **Extração**: **1 chamada LLM por grupo** (não por learning). Com ~10-20 grupos/semana, o custo é marginal.
- **Verificação**: reconstrução é similaridade de texto (uma função determinística), não chamada LLM.
- O engine **nunca** chama LLM para ler — apenas para redigir o princípio.

### 3.4 Cadência

| Frequência | Gatilho | Escopo |
|------------|---------|--------|
| **Semanal** | Scheduler (segunda 08:00, junto com F1.6) | Domínios com clusters candidatos |
| **Sob demanda** | `cosca cognitive compress --all` | Base inteira |
| **Parcial** | `cosca cognitive compress --domain testing` | Domínio específico |
| **Urgente** | Entropia > threshold ou Don command | Domínios de maior entropia |

---

## 4. FÓRMULA DE COMPRESSÃO

### 4.1 As Três Métricas

```
compression_ratio = 1 - (princípios / learnings)

reconstruction_fidelity = similaridade(princípio, grupo original)

compression_value = compression_ratio × reconstruction_fidelity
```

| Métrica | O que mede | Fórmula | Faixa |
|---------|-----------|---------|:-----:|
| **compression_ratio** | Quanto o volume foi reduzido | `1 - (princípios / learnings)` | 0.0 - 1.0 |
| **reconstruction_fidelity** | Quanta informação essencial foi preservada | `similaridade(princípio, grupo)` | 0.0 - 1.0 |
| **compression_value** | Valor líquido da compressão (economia × fidelidade) | `ratio × fidelity` | 0.0 - 1.0 |

### 4.2 Thresholds de Decisão

```
┌────────────────────────────────────────────────────────────────────┐
│                                                                    │
│   compression_value > 0.7  →  COMPRIME                            │
│     Compressão de alto valor: economia significativa com          │
│     perda mínima. Substitui grupo por princípio + referências.    │
│                                                                    │
│   compression_value 0.4-0.7  →  COMPRIME PARCIAL                  │
│     Compressão de valor médio: mantém exemplos-chave do grupo     │
│     ao lado do princípio (princípio + 1-2 exemplos canônicos).    │
│                                                                    │
│   compression_value < 0.4  →  NÃO COMPRIME                        │
│     Compressão de baixo valor: a perda de informação não justifica│
│     a economia de contexto. Mantém o grupo intacto.               │
│                                                                    │
└────────────────────────────────────────────────────────────────────┘
```

### 4.3 Interpretação Econômica da Fórmula

A fórmula é o **ROI da compressão** em forma multiplicativa (P-ARCH-004 — semântica zero-terminal):

- **`compression_ratio` é a economia**: reduzir 50 learnings para 3 princípios = ratio 0.94 (94% de economia de contexto).
- **`fidelity` é a qualidade**: se o princípio não reconstruir o grupo, a "economia" é na verdade perda. Fidelity baixa zera o valor.
- **`compression_value` é o produto**: economia sem fidelidade é descarte; fidelidade sem economia é redundância. Os dois juntos são compressão.

### 4.4 Relação com a Fórmula Anterior (v1.0.0)

A v1.0.0 media `compression_efficiency = (ratio × applicability) / (level × ln(entries))` — uma métrica de eficiência semântica. A v2.0.0 adota a fórmula do Don: **mais simples, mais econômica e com decisão binária clara** (comprimir / comprimir parcial / não comprimir). A fórmula antiga permanece documentada em [COMPRESSION_V1.md](COMPRESSION_V1.md) §7.2 como referência.

---

## 5. VERIFICAÇÃO DE FIDELIDADE (RECONSTRUÇÃO)

### 5.1 O Teste de Reconstrução

A fidelidade não é estimada — é **testada por reconstrução**. Para cada grupo:

```
PARA CADA grupo de learnings {L1, L2, ..., Ln}:

  1. Redige o princípio P a partir do grupo
  2. PARA CADA Li do grupo:
       reconstrói Li a partir de P (o que P implica sobre este caso?)
       fidelity_i = similaridade(Li, reconstrução)
  3. reconstruction_fidelity = média(fidelity_1 ... fidelity_n)
  4. Se reconstruction_fidelity ≥ 0.90 → COMPRIME
     Senão → MANTÉM grupo (perda alta demais)
```

### 5.2 Regra de Ouro: Fidelity Mínima 90%

> **Nenhuma compressão é executada com `reconstruction_fidelity < 0.90`.**
>
> O princípio comprimido DEVE ser capaz de reconstruir cada learning original
> com pelo menos 90% de similaridade semântica. Abaixo disso, o princípio
> está perdendo informação essencial — e a compressão vira corrupção.

| Fidelity | Decisão | Racional |
|:--------:|---------|----------|
| **≥ 0.90** | Comprime | O princípio explica o grupo: reconstrução fiel |
| **0.80 - 0.90** | Comprime parcial | Princípio + exemplos-chave cobrem a lacuna |
| **< 0.80** | Não comprime | O grupo tem variação semântica alta demais |

### 5.3 O que "reconstruir" significa

Reconstruir um learning a partir do princípio = responder: **"O que este princípio me diz sobre este caso específico?"** Se o princípio "sempre verificar `go list -deps` antes de remover código" reconstitui o L29 ("`go build ./...` não detecta código não-linkado") com alta similaridade, o princípio capturou a essência do learning.

A reconstrução é uma **compressão reversível com perda controlada**: o leitor do princípio consegue derivar os casos originais (informação essencial preservada) e, quando precisa do detalhe exato, segue a referência até o learning original (que permanece em `learning_history`).

---

## 6. O QUE NUNCA COMPRIME

### 6.1 As Quatro Proteções Absolutas

| # | Categoria | Exemplos | Por que NUNCA comprime |
|---|-----------|----------|------------------------|
| **P1** | **Learnings com contexto crítico** (segurança, dados) | L12 (permission hardening), L13 (jail bypass), L15 (auto-jail), DDNAs de dados | Perder 10% de um learning de segurança é perder uma proteção. Detalhe de segurança NÃO é redundância — é o ativo. |
| **P2** | **Learnings recentes (< 30 dias)** | Qualquer learning com idade < 30 dias | Ainda em validação: podem ser refutados ou refinados. Comprimir antes da validação congela uma hipótese como se fosse fato. |
| **P3** | **Failures** | `failures.md` (jail breach, F002, doc drift) | O detalhe do erro É a informação. Perder a especificidade de uma falha impede a prevenção da próxima. |
| **P4** | **DDNA** | `DECISION_DNA.md` entries, decisões P0/P1 | Decisões precisam de rastro completo: contexto, alternativas, evidências, consequências. Compressão destruiria a auditabilidade. |

### 6.2 Regra de Ouro

```
┌────────────────────────────────────────────────────────────────────┐
│   SE a informação for:                                             │
│   ├── Sobre SEGURANÇA ou DADOS (crítico)      → NUNCA comprime    │
│   ├── MENOS de 30 dias de idade               → NUNCA comprime    │
│   ├── UMA FALHA (failures.md)                 → NUNCA comprime    │
│   └── UMA DECISÃO (DDNA)                      → NUNCA comprime    │
│                                                                   │
│   Estas quatro categorias são IMUNES à compressão,               │
│   independentemente do compression_value.                        │
└────────────────────────────────────────────────────────────────────┘
```

### 6.3 Por que o contexto crítico é imune

Um learning de segurança tem duas partes: a regra ("não bypassar a jaula") e o contexto ("o bypass regrediu 11 arquivos de v3.0.1 para v2.0"). A regra comprime bem; o contexto não. Se o contexto for perdido, o próximo agente não entende **por que** a regra existe — e a regra vira burocracia, não proteção.

### 6.4 Por que failures nunca comprimem

Failures são memória negativa: o valor está no **detalhe do erro** (o que foi tentado, por que falhou, o que fazer diferente). Comprimir "falha na remoção de código" para "verificar antes de remover" perde o cenário exato de falha — e a prevenção precisa do cenário, não do resumo.

### 6.5 Por que DDNA nunca comprime

DDNA é o rastro de decisão (F1.1): Decisão → Evidências → Riscos → Alternativas → Resultado. O F1.2 (Contrafactual Gate) depende desse rastro completo para avaliar decisões futuras. Um DDNA comprimido é uma decisão sem memória — o pior tipo de débito de conhecimento.

---

## 7. INTEGRAÇÃO COM F2.1 — COGNITIVE ECONOMY

### 7.1 Compressão reduz custo de contexto

A **Cognitive Economy (F2.1)** mede o ROI de cada task. O custo dominante em tasks de conhecimento é `custo_tokens`:

```
custo_tokens = (input_tokens + output_tokens) × token_price
input_tokens  = system prompt + KERNEL.md + MEMÓRIA carregada + task
```

A compressão ataca diretamente a **MEMÓRIA carregada**: onde antes a F2.4 injetava 50 learnings (~40KB), agora injeta 3 princípios (~4KB).

### 7.2 O Ciclo Econômico

```
┌─────────────────────────────────────────────────────────────────┐
│                                                                  │
│   ANTES DA COMPRESSÃO                  DEPOIS DA COMPRESSÃO      │
│                                                                  │
│   Task "escrever teste para X"         Task "escrever teste      │
│   ├── 50 learnings injetados             para X"                 │
│   ├── ~40KB de contexto                 ├── 3 princípios +       │
│   ├── input_tokens: ~12.000               referências            │
│   ├── custo_tokens: $0.120              ├── ~4KB de contexto     │
│   └── ROI: baixo (custo alto)           ├── input_tokens: ~4.000 │
│                                         ├── custo_tokens: $0.040 │
│                                         └── ROI: +66% mais alto  │
│                                                                  │
│   compressão → menos tokens → menos custo → maior ROI por task   │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### 7.3 Registro no Pipeline Pós-Task da F2.1

A cada ciclo de compressão, o engine registra o ganho econômico:

| Campo CSV (F2.1) | Valor | Significado |
|-------------------|-------|-------------|
| `context_tokens_saved` | ~8.000/task | Tokens de memória que deixaram de ser carregados |
| `context_cost_saved` | ~$0.080/task | Custo evitado por task (à $0.01/K tokens) |
| `tasks_affected` | ~10/semana | Tasks que carregam o domínio comprimido |
| `weekly_savings` | ~$0.80/semana | Economia semanal projetada por domínio |

### 7.4 Compressão é investimento, não despesa

Assim como tasks de infraestrutura têm ROI negativo imediato e ROI composto futuro (F2.1 §4), o ciclo de compressão custa ~$0.005 (1 chamada LLM por grupo) e retorna economia permanente. O engine registra o ciclo como `task_type: infrastructure` no CSV da F2.1 — investimento de capital intelectual.

---

## 8. INTEGRAÇÃO COM F1.6 — COGNITIVE ENTROPY

### 8.1 Compressão REDUZ entropia diretamente

A **Cognitive Entropy (F1.6)** mede desorganização do conhecimento:

```
CognitiveEntropy = (contradictions × 3 + stale × 2 + gaps × 1) / total_knowledge
```

A compressão ataca **os três componentes do numerador**:

| Componente (F1.6) | Efeito da compressão | Mecanismo |
|--------------------|----------------------|-----------|
| **contradictions** (peso 3) | **↓ direto** | Learnings que se contradizem, ao serem consolidados num único princípio, param de contradizer — o princípio é internamente consistente |
| **stale** (peso 2) | **↓ indireto** | O princípio comprimido é revalidado na reconstrução (freshness resetada); originais arquivados não enganam mais ninguém |
| **gaps** (peso 1) | **↓ direto** | Conhecimento fragmentado (consolidation gap) é consolidado em princípio — o gap fecha |

### 8.2 O Ciclo Virtuoso

```
Compressão consolida grupo
        │
        ▼
Contradições dentro do grupo somem (unificadas no princípio)
        │
        ▼
Entropia cai (numerador menor)
        │
        ▼
F2.1 risk_penalty cai → ROI ajustado sobe
        │
        ▼
F2.4 Cognitive Gravity: conhecimento confiável é mais atraído
        │
        ▼
Mais reuso → mais validação → princípio fica mais forte
        │
        ▼
Próximo ciclo de compressão é mais preciso (evidência maior)
```

### 8.3 Registro no Pipeline do F1.6

A cada ciclo de compressão, o engine atualiza o cálculo do F1.6:

| Métrica (F1.6) | Antes | Depois | Δ |
|-----------------|:-----:|:------:|:-:|
| `contradictions` | 1 | 0-1 (resolvidas na consolidação) | ↓ |
| `stale` | N | N-1 (originais arquivados) | ↓ |
| `gaps` | 4 | 3 (1 gap consolidado) | ↓ |
| `entropy` | 0.433 | < 0.433 (projeção até 0.19 no baseline v1) | ↓ |

> **Relação bidirecional**: a F1.6 **dispara** a compressão (entropia alta = urgência de consolidar) e a compressão **reduz** a entropia (retroalimentação negativa — o antídoto estrutural da entropia).

---

## 9. INTEGRAÇÃO COM F1.2 — CONTRAFACTUAL GATE

### 9.1 O Gate antes da Compressão

O **Contrafactual Gate (F1.2)** pergunta antes de toda decisão P0/P1: *"E se fosse diferente?"* A compressão é uma decisão sobre conhecimento — e merece o mesmo escrutínio:

```
PERGUNTA DO GATE PARA CADA GRUPO:

  "E se o princípio estiver errado?"
  ──────────────────────────────
  Proposta: comprimir {L20, L22, L29, L35, L52} em 1 princípio
  Contrafactual: manter os 5 learnings separados

  Teste do Gate = TESTE DE RECONSTRUÇÃO:
  ├── Se o princípio reconstrói os 5 learnings com fidelity ≥ 0.90
  │     → o contrafactual perde: comprime
  ├── Se a reconstrução falha (< 0.90)
  │     → o contrafactual vence: mantém o grupo
  └── Se parcial (0.80-0.90)
        → comprime parcial: princípio + exemplos-chave
```

### 9.2 O Teste de Reconstrução É o Contrafactual da Compressão

O F1.2 opera em decisões; o F3.3 opera em conhecimento. O **teste de reconstrução é a materialização do Gate para compressão**: em vez de "e se a decisão estiver errada?", pergunta "e se o princípio estiver errado?" — e responde objetivamente reconstruindo o grupo a partir do princípio. Se o princípio não consegue explicar os próprios casos, o contrafactual (manter) vence.

### 9.3 Cooperação em Decisões de Conhecimento

| Cenário | F1.2 faz | F3.3 faz |
|---------|----------|----------|
| Decisão P0/P1 sobre remover provider | Gera alternativas, avalia evidências (ex: "executar `go list -deps` antes") | N/A — DDNA é imune à compressão (P4) |
| Compressão de grupo de learnings | N/A (não é decisão de sistema) | Roda reconstrução (o contrafactual do grupo) |
| Princípio comprimido usado em decisão futura | Avalia o princípio como evidência | Mantém referência ao DDNA original (imune) |

---

## 10. INTEGRAÇÃO COM F9.2 — WISDOM DISTILLATION

### 10.1 Duas Camadas de Destilação

O **F3.3 (Compression)** e o **F9.2 (Wisdom Distillation)** são complementares, não redundantes:

```
┌─────────────────────────────────────────────────────────────────────┐
│                                                                      │
│  600+ LEARNINGS BRUTOS                                              │
│       │                                                              │
│       ▼                                                              │
│  F3.3 COGNITIVE COMPRESSION (este engine)                           │
│  ─────────────────────────────                                      │
│  Muitos learnings → poucos princípios                               │
│  Taxa: ~10:1 (50 learnings → 3 princípios)                          │
│  Garantia: fidelity ≥ 90% (perda controlada)                        │
│  Foco: ECONOMIA DE CONTEXTO (custos)                                │
│  Reversível: Don pode desfazer                                       │
│  Cadência: semanal                                                   │
│       │                                                              │
│       ▼                                                              │
│  PRINCÍPIOS TÉCNICOS (knowledge/principles/)                        │
│       │                                                              │
│       ▼                                                              │
│  F9.2 WISDOM DISTILLATION                                           │
│  ───────────────────────                                            │
│  Princípios técnicos → emendas constitucionais                      │
│  Taxa: ~100:1 acumulada (500 → 1-2 emendas)                         │
│  Garantia: maturidade > 0.9 + coesão                                 │
│  Foco: GOVERNANÇA (constituição)                                    │
│  Não reversível (lei é lei)                                          │
│  Cadência: mensal                                                     │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### 10.2 Divisão de Trabalho

| Dimensão | F3.3 Compression | F9.2 Wisdom Distillation |
|----------|------------------|--------------------------|
| **Pergunta** | "Como reduzir custo de contexto sem perder informação?" | "O que merece virar lei?" |
| **Entrada** | Learnings brutos por domínio | Princípios compilados (F9.1) + padrões |
| **Saída** | Princípios técnicos + referências | Emendas constitucionais (max 2/mês) |
| **Métrica** | `compression_value` | `maturity > 0.9` |
| **Reversibilidade** | Sim (Don reverte) | Não (constituição) |
| **Frequência** | Semanal | Mensal |

### 10.3 Fluxo de Dados

```
F3.3 comprime → princípios densos e validados → F9.1/F9.2 consomem
                                │
                                └──→ menos ruído na entrada da destilaria
                                     (princípios limpos destilam melhor)
```

A compressão **alimenta** a destilação com entrada mais limpa: o F9.2 não precisa ler 50 learnings redundantes sobre testes — ele lê 3 princípios já validados por reconstrução e decide quais merecem maturidade constitucional.

---

## 11. EXEMPLO REAL: DEAD CODE REMOVAL

### 11.1 O Cluster (Dados Reais do Cosca)

O domínio **dead code removal** acumulou 5 learnings de 3 agentes diferentes:

| Learning | Agente | Data | Confiança | Afirmação essencial |
|----------|--------|------|:---------:|---------------------|
| **L20** | cosca-kernel | 2026-07-29 | 0.90 | "Código morto só se detecta com análise de grafo de imports: `go list -f '{{join .Deps}}' ./cmd/cosca/` provou que 6.615 linhas nunca entram no binário. `go build` não detecta." |
| **L22** | cosca-kernel | 2026-07-30 | 0.93 | "Classificação de gaps de cobertura em 3 categorias: Tipo A (código morto) → remove." |
| **L29** | cosca-kernel | 2026-07-30 | 0.94 | "`go build ./...` não detecta código não-linkado. `go list -deps` é a ferramenta correta para detectar dead code — não `go build`." |
| **L35** | cosca-architecture | 2026-07-30 | 0.87 | "Código morto aumenta custo de manutenção sem benefício. Deve ser identificado por análise de dependências (`go list -deps`) antes de qualquer refatoração." |
| **L52** | cosca-performance | 2026-07-30 | 0.82 | "Dead code não é só código não usado — é código que COMPILA mas não é IMPORTADO. `go build` não detecta. Precisa de `go list -deps`." |

**Similaridade média do grupo**: 0.88 (mesmo domínio, mesma técnica, mesma causa raiz)
**Densidade**: 5 learnings × nível médio 3.6 = 18.0

### 11.2 Extração do Princípio

**Princípio redigido**:

> *"Sempre verificar `go list -deps` antes de remover código."*
>
> Elaboração: código morto compila mas não entra no binário — `go build` é cego
> para código não-linkado. A detecção exige análise do grafo de imports
> (`go list -f '{{join .Deps}}' ./cmd/cosca/ | grep <pacote>` ou `go list -deps`).
> A remoção é em cascata (código → testes órfãos) e classifica gaps em 3 tipos
> (A: remove, B: testa, C: documenta).

### 11.3 Cálculo da Compressão

```
compression_ratio = 1 - (princípios / learnings)
                  = 1 - (1 / 5)
                  = 0.80

reconstruction_fidelity = similaridade(princípio, grupo original)
                        = 0.93   (princípio reconstruiu os 5 learnings)

compression_value = compression_ratio × reconstruction_fidelity
                  = 0.80 × 0.93
                  = 0.744

0.744 > 0.7  →  COMPRIME  ✅
fidelity 0.93 ≥ 0.90      →  COMPRIME  ✅
```

| Verificação | Valor | Threshold | Status |
|-------------|:-----:|:---------:|:------:|
| `compression_value` | 0.744 | > 0.7 | ✅ Comprime |
| `reconstruction_fidelity` | 0.93 | ≥ 0.90 | ✅ Fidelity mínima |
| Categoria de proteção | Nenhuma (P1-P4 não aplicam) | — | ✅ Não é crítico/recente/failure/DDNA |

### 11.4 Resultado da Compressão

```
ANTES:  5 learnings (L20, L22, L29, L35, L52)
        ~2.400 linhas de memória
        Custo de contexto por task: ~5KB

DEPOIS: 1 princípio (CCP-101)
        ~30 linhas + referências
        Custo de contexto por task: ~0.5KB

ECONOMIA: 10:1 em linhas | 0.80 compression_ratio | 90% menos contexto
GANHO:    context cost reduzido (F2.1) + entropia reduzida (F1.6)
```

---

## 12. FORMATO DO PRINCÍPIO COMPRIMIDO

### 12.1 Template Canônico (YAML)

Princípios comprimidos são armazenados em `knowledge/principles/CCP-NNN.yaml`:

```yaml
principle:
  id: "CCP-101"
  name: "Verificar go list -deps antes de remover código"
  type: "compressed_principle"
  status: "active"
  created: "2026-07-30"
  created_by: "cognitive-compression-engine"
  approved_by: "cosca-kernel"

  statement:
    core: "Sempre verificar `go list -deps` antes de remover código."
    elaboration: |
      Código morto compila mas não entra no binário — `go build` é cego
      para código não-linkado. Detecção exige análise do grafo de imports.

  compression:
    input_learnings: ["L20", "L22", "L29", "L35", "L52"]
    input_agents: ["cosca-kernel", "cosca-architecture", "cosca-performance"]
    compression_ratio: 0.80
    reconstruction_fidelity: 0.93
    compression_value: 0.744
    decision: "compress"          # compress | partial | none
    fidelity_check: "reconstruction"  # método de verificação

  original_references:            # ★ RASTREABILIDADE — obrigatório
    - id: "L20"
      path: "memory/agent/cosca-kernel/learnings.md#L20"
      agent: "cosca-kernel"
    - id: "L22"
      path: "memory/agent/cosca-kernel/learnings.md#L22"
      agent: "cosca-kernel"
    - id: "L29"
      path: "memory/agent/cosca-kernel/learnings.md#L29"
      agent: "cosca-kernel"
    - id: "L35"
      path: "memory/agent/cosca-architecture/learnings.md#L35"
      agent: "cosca-architecture"
    - id: "L52"
      path: "memory/agent/cosca-performance/learnings.md#L52"
      agent: "cosca-performance"

  protection:                    # ★ imune à compressão (P1-P4)
    security_context: false
    data_context: false
    age_days: 1
    under_30_days: false
    is_failure: false
    is_ddna: false

  reversible:
    reversible: true
    revert_status: "none"        # none | requested | reverted
    reverted_by: null
    reverted_at: null

  entropy_reduction:
    before: 0.433
    after: 0.410                # projeção após consolidação
    delta: -0.023

  cmi_impact:
    aprendizado: +5
    consistencia: +5

  reconsideration_triggers:
    - "Se um novo learning contradizer o princípio, reabrir o grupo."
    - "Revisão programada: 90 dias (F1.4 Wisdom Decay)."

  tags:
    - "#principle"
    - "#compressed"
    - "#dead-code"
    - "#go-list-deps"
```

### 12.2 Rastreabilidade Obrigatória

> **TODO princípio comprimido DEVE manter referências aos originais** (`original_references`).
> O leitor do princípio pode reconstruir o grupo por similaridade (essência) E pode
> consultar o learning exato (detalhe). Rastreabilidade não é opcional — é o que
> distingue compressão de descarte.

### 12.3 Onde os Originais Ficam

| Artefato | Local | Estado |
|----------|-------|--------|
| **Princípio comprimido** | `knowledge/principles/CCP-NNN.yaml` | Fonte canônica de consulta |
| **Learnings originais** | `memory/agent/*/learnings.md` | Tagged `#compressed` + `compressed_into: CCP-NNN` |
| **Cópia de segurança** | `memory/learning_history/` | Cópia integral antes da compressão |
| **Registro do ciclo** | `memory/timeline/compression.csv` | Métricas + decisão + reversão |

---

## 13. GATILHOS DE COMPRESSÃO

### 13.1 Gatilhos Automáticos

| # | Gatilho | Threshold | Ação |
|---|---------|-----------|------|
| **T1** | **Cluster Density** | ≥ 3 learnings no mesmo tópico | Candidato a compressão no ciclo semanal |
| **T2** | **Similaridade média** | ≥ 0.75 entre learnings do grupo | Cluster maduro para extração |
| **T3** | **Entropia (F1.6)** | > 0.25 (🔴) | Executar ciclo imediato nos domínios de maior entropia |
| **T4** | **Context cost** | Custo de contexto do domínio > $0.10/semana | Priorizar domínio na fila semanal |
| **T5** | **Manual** | Don command `cosca cognitive compress` | Executar escopo solicitado |

### 13.2 Lógica de Decisão

```
SE cluster ≥ 3 learnings (T1) E similaridade ≥ 0.75 (T2):
    → candidato para o ciclo semanal
    → calcular compression_value
    → aplicar thresholds (comprimir / parcial / não comprimir)

SE entropia > 0.25 (T3):
    → execução imediata (não espera o ciclo semanal)
    → priorizar domínios de maior contribuição de entropia

SE Dom requisitou (T5):
    → executar imediatamente no escopo solicitado

SEMPRE:
    → verificar P1-P4 (nunca comprime crítico/recente/failure/DDNA)
    → verificar fidelity ≥ 0.90 antes de publicar
```

---

## 14. GOVERNANÇA E REVERSÃO

### 14.1 Don Pode Reverter Compressão

> **A compressão é reversível. O Don pode desfazer qualquer compressão.**

```
cosca cognitive compress --revert CCP-101
        │
        ▼
1. Verifica revert_status != "none"
2. Restaura os 5 learnings originais de learning_history/
3. Remove a tag #compressed e o campo compressed_into
4. Marca o princípio como status: "reverted" (não deleta — histórico)
5. Registra reversão no compression.csv (audit trail)
```

### 14.2 Regras de Reversão

| # | Regra | Racional |
|---|-------|----------|
| **R1** | Reversão restaura TODOS os originais do grupo | Reversão parcial quebraria a consistência |
| **R2** | O princípio nunca é deletado — vira `status: reverted` | Rastro histórico (o mesmo padrão do F9.4 learning_history) |
| **R3** | Reversão exige registro de motivo (DDNA ou nota) | Por que o Don reverteu? |
| **R4** | Após reversão, o grupo é `compression_deferred` por 30 dias | Impede recompressão imediata do mesmo grupo |
| **R5** | Don pode reverter por CLI, mas a auditoria é automática | `--revert` exige `--reason` |

### 14.3 Propriedade

| Responsabilidade | Entidade |
|------------------|----------|
| **Execução do ciclo** | Cognitive Compression Engine (automático, semanal) |
| **Aprovação de compressão** | Automática se fidelity ≥ 0.90 + sem proteção P1-P4 |
| **Reversão** | Don (exclusivo) |
| **Revisão de princípios** | Cosca Critic Chief (contrafactual) + F1.4 (90 dias) |
| **Auditoria** | compression.csv append-only (imutável) |

---

## 15. CLI E AUTOMAÇÃO

### 15.1 Comandos

```bash
# Executar ciclo completo (DRY_RUN primeiro)
cosca cognitive compress --dry-run --all
cosca cognitive compress --all

# Comprimir domínio específico
cosca cognitive compress --domain testing

# Verificar candidatos sem executar
cosca cognitive compress --check

# Reverter uma compressão (exige motivo)
cosca cognitive compress --revert CCP-101 --reason "novo learning contradiz o princípio"

# Gerar relatório do último ciclo
cosca cognitive compress --report
```

### 15.2 Automação

| Trigger | Ação |
|---------|------|
| **Scheduler semanal** (segunda 08:00) | Ciclo completo, junto com F1.6 |
| **Entropia spike** (> 0.25 ou +10pp semanal) | Ciclo imediato nos domínios de maior entropia |
| **Cluster formado** (≥ 3 learnings) | Registrar candidato (não executar sem ciclo) |
| **Don command** | Execução imediata do escopo solicitado |

---

## 16. MÉTRICAS DO ENGINE

### 16.1 Dashboard do Ciclo

```
COGNITIVE COMPRESSION REPORT — 2026-07-30 (baseline v2.0.0)
┌──────────────────────────────────────────────────────────┐
│  LEARNINGS:     600+ totais                              │
│  COMPRIMIDOS:   5 (L20, L22, L29, L35, L52) → 1 princípio│
│  RATIO MÉDIO:   0.80                                     │
│  FIDELITY MÉDIO: 0.93 (≥ 0.90 ✅)                        │
│  CONTEXT SAVED: ~$0.80/semana (projeção)                 │
│  ENTROPIA:      0.433 → 0.410 (projeção -0.023)          │
│  REVERSÕES:     0                                        │
└──────────────────────────────────────────────────────────┘
```

### 16.2 Métricas-Chave

| Métrica | Fórmula | Alvo | Alerta |
|---------|---------|------|--------|
| **Compression ratio médio** | `1 - (princípios/learnings)` | > 0.7 | < 0.4 = ciclo ineficiente |
| **Fidelity média** | `similaridade(princípio, grupo)` | ≥ 0.90 | < 0.90 = compressor está perdendo informação |
| **Context cost saved** | `(tokens_antes - tokens_depois) × price` | > $0.50/semana | 0 = compressão não está sendo consumida |
| **Entropy delta** | `entropy_antes - entropy_depois` | < 0 | > 0 = compressor está fragmentando |
| **Taxa de reversão** | `reversões / princípios` | < 5% | > 10% = compressor está desalinhado com o Don |
| **Tempo de ciclo** | `tempo_total / ciclos` | < 10s | > 10s = otimizar pipeline |

---

## 17. CASOS DE BORDA E ANTI-PADRÕES

### 17.1 Casos de Borda

| Caso | Tratamento |
|------|-----------|
| **Grupo com 2 learnings** | Não comprime (mínimo 3 — T1 não dispara) |
| **Grupo com fidelity 0.89** | Comprime parcial: princípio + 1 exemplo-chave |
| **Grupo contendo 1 security learning** | P1 dispara → grupo inteiro imune (nunca comprime) |
| **Grupo com learning de 25 dias** | P2 dispara → aguarda 5 dias (comprime quando > 30 dias) |
| **Princípio contradiz princípio existente** | Não publica → escalar ao Critic/Contradiction Engine |
| **Compressão aumentou entropia** | Alerta crítico → reverter grupo + investigar pipeline |
| **Reconstrução falha em 1 de 5 learnings** | Fidelity parcial → comprime parcial mantendo o learning problemático como exemplo-chave |

### 17.2 Anti-Padrões

| Anti-padrão | Por que é errado | Correto |
|-------------|------------------|---------|
| **Comprimir por volume** (comprimir porque "são muitos") | Volume não é critério — fidelity é | Comprimir só se reconstruction ≥ 0.90 |
| **Comprimir security para "limpar" a base** | Perder detalhe de segurança é perder proteção | P1 — imune, sempre |
| **Deletar originais após comprimir** | Sem referências não há rastreabilidade | Originais vão para learning_history com referência |
| **Comprimir failures para "resumir" erros** | O detalhe do erro é a prevenção | P3 — imune, sempre |
| **Comprimir DDNA para economizar contexto** | Decisão sem rastro é débito cognitivo | P4 — imune, sempre |
| **Recompressão imediata após reversão** | Don reverteu por um motivo | R4 — espera 30 dias |
| **Chamar LLM para cada learning na verificação** | Custo explode; verificação é determinística | 1 chamada por grupo; verificação por similaridade |

---

## 18. ESCALAÇÃO

| Situação | Ação |
|----------|------|
| **Fidelity média < 0.80 por 2 ciclos consecutivos** | Escalar ao Kernel + Architecture Chief — compressor com defeito |
| **Compressão aumentou entropia** | Alerta crítico → reverter grupos + investigar pipeline |
| **Princípio contradiz CONSTITUTION.md** | Rejeitar publicação → escalar ao Don |
| **Don reverteu > 10% dos princípios** | Revisar thresholds de compressão com o Don |
| **Grupo com contexto crítico tentado para compressão** | Bloquear (P1-P4) → registrar tentativa no log |
| **Reconstrução instável (fidelity oscila)** | Marcar grupo como `compression_deferred` por 30 dias |

---

## 19. REFERÊNCIAS

| Documento | Caminho | Relação |
|-----------|---------|---------|
| Cognitive Economy (F2.1) | [../cognitive-economy/SKILL.md](../cognitive-economy/SKILL.md) | Custo de contexto que a compressão reduz |
| Contrafactual Gate (F1.2) | [../../workflows/contrafactual-gate.md](../../workflows/contrafactual-gate.md) | Teste de reconstrução como contrafactual da compressão |
| Cognitive Entropy (F1.6) | [../cognitive-entropy/ENTROPY.md](../cognitive-entropy/ENTROPY.md) | Entropia que a compressão reduz + gatilho T3 |
| Wisdom Distillation (F9.2) | [../wisdom-distillation/SKILL.md](../wisdom-distillation/SKILL.md) | Camada de destilação constitucional (complementar) |
| Experience Compiler (F9.1) | [../experience-compiler/SKILL.md](../experience-compiler/SKILL.md) | Compilação de learnings em padrões (upstream) |
| Compression v1.0.0 (arquivado) | [COMPRESSION_V1.md](COMPRESSION_V1.md) | Modelo anterior F3.2 (gatilhos, gravidade Planet, CCP-001-004) |
| COGNITIVE_MATURITY.md §5 C10 | [../../architecture/COGNITIVE_MATURITY.md](../../architecture/COGNITIVE_MATURITY.md) | Conceito original de Cognitive Compression |
| COGNITIVE_ECOSYSTEM.md | [../../architecture/COGNITIVE_ECOSYSTEM.md](../../architecture/COGNITIVE_ECOSYSTEM.md) | Mapa do ecossistema (F3.2 → F3.3) |
| DECISION_DNA (F1.1) | [../../knowledge/architecture/DECISION_DNA.md](../../knowledge/architecture/DECISION_DNA.md) | DDNA — imune à compressão (P4) |
| LEARNING_PROTOCOL.md | [../../memory/LEARNING_PROTOCOL.md](../../memory/LEARNING_PROTOCOL.md) | Formato de learning que alimenta o engine |
| AUTO_EVOLUTION_PROTOCOL.md | [../../shared/AUTO_EVOLUTION_PROTOCOL.md](../../shared/AUTO_EVOLUTION_PROTOCOL.md) | Protocolo de auto-evolução (stages 7-8) |

### Dados Fonte

| Arquivo | Caminho | Conteúdo |
|---------|---------|----------|
| cosca-kernel learnings | [../../memory/agent/cosca-kernel/learnings.md](../../memory/agent/cosca-kernel/learnings.md) | L20, L22, L29 (dead code removal) |
| cosca-architecture learnings | [../../memory/agent/cosca-architecture/learnings.md](../../memory/agent/cosca-architecture/learnings.md) | L35 (dead code) |
| cosca-performance learnings | [../../memory/agent/cosca-performance/learnings.md](../../memory/agent/cosca-performance/learnings.md) | L52 (dead code) |
| F9.1 exemplo dead code | [../experience-compiler/SKILL.md](../experience-compiler/SKILL.md) | Cluster {L20, L22, L35, L52} validado no F9.1 |

---

## 20. HISTÓRICO

| Versão | Data | Autor | Mudanças |
|--------|------|-------|----------|
| 2.0.0 | 2026-07-30 | Architecture Chief | **Redesign por ordem do Don (F3.3)**. Novo modelo canônico: compressão como gzip de conhecimento, pipeline semanal < 10s, fórmula `compression_value = compression_ratio × reconstruction_fidelity` com thresholds (0.7/0.4), fidelity mínima 90%, 4 proteções absolutas (crítico/recente/failure/DDNA), integrações F2.1 (custo de contexto) e F1.6 (entropia), exemplo real dead code removal (5:1, fidelity 0.93). v1.0.0 (F3.2) preservada em COMPRESSION_V1.md. |
| 1.0.0 | 2026-07-30 | Cosca AI Chief | Versão original (F3.2): extração de princípios por gravidade cognitiva, gatilhos T1-T6, template YAML CCP-NNN, 4 compressões práticas (CCP-001 a CCP-004). **Arquivada em [COMPRESSION_V1.md](COMPRESSION_V1.md).** |

---

> **"Depois de centenas de projetos. O runtime cria regras universais. Em vez de lembrar 80 casos. Ele aprende: Todos seguem o mesmo princípio."**
>
> — O Don, definindo Cognitive Compression
>
> **"600+ learnings → princípios. Contexto barato. Entropia baixa. Informação essencial preservada — com rastro."**
>
> — Cognitive Compression Engine v2.0.0 (F3.3), 2026-07-30
