# B4 — Conhecimento Reutilizado

> **Métrica**: B4 | **Owner**: cosca-monitoring | **Criado**: 2026-07-30
> **Schema**: `internal/embed/cosca/metrics/CMI_REAL_METRICS.md` — Seção 3.4
> **Atualização**: A cada tarefa que referencia conhecimento prévio (padrão, heurística, learning)

---

## Baseline (2026-07-30)

### Resumo

| Indicador | Valor |
|-----------|-------|
| **Total de reusos registrados** | 12 |
| **Padrões com reuso documentado** | 3 |
| **Média de reusos por padrão** | 4.0 |
| **Padrão mais reutilizado** | parallel-orchestration (5 reusos) |
| **Taxa de sucesso dos reusos** | 100% |
| **Meta Fase 1** | ≥ 15 reusos totais |

---

## Catálogo de Padrões com Reuso

### PADRÃO 1: Cross-Agent Audit (Auditoria Cross-Agent)

| Campo | Valor |
|-------|-------|
| **ID** | cross-agent-audit |
| **Nome** | Padrão de Auditoria Cross-Agent |
| **Descrição** | Deploy de múltiplos agentes especialistas em paralelo, cada um auditando uma dimensão independente, com síntese centralizada pelo Kernel. O padrão escala linearmente: 3 agentes (básico) → 8 agentes (massivo). Tempo de execução constante (~3-5 min) independente do número de agentes devido ao paralelismo. |
| **Descoberto em** | L18 (2026-07-29) — Runtime Coverage Audit |
| **Versão atual** | v4 (octa-agent) |
| **Contagem de reusos** | 3 |
| **Taxa de sucesso** | 100% (3/3) |

#### Histórico de Reusos

##### B4-001 — Primeira Reaplicação: CLI Coverage Breakthrough (L19)
| Campo | Valor |
|-------|-------|
| **Data** | 2026-07-29 |
| **Contexto** | Refatoração do monolito runServe (699 linhas) para quebrar barreira dos 70% no CLI |
| **Adaptação Necessária?** | Sim — foco em extrair-para-testar em vez de mapeamento puro |
| **Adaptação** | Mesmo padrão de 3 agentes (discovery + QA + testing), mas com objetivo diferente: discovery identificou blocos extraíveis, QA verificou thresholds, testing executou cobertura e validou extrações |
| **Outcome** | ✅ success — runServe 0.5%→61.8%, CLI total 68.5%→71.5% |
| **Lição do Reuso** | O padrão de auditoria cross-agent é adaptável a diferentes objetivos (cobertura, refatoração, qualidade). A estrutura base (deploy paralelo + síntese) permanece; o que muda é a pergunta que cada agente responde. |

##### B4-002 — Segunda Reaplicação: Coverage Audit + Doc Expurgo (L21)
| Campo | Valor |
|-------|-------|
| **Data** | 2026-07-30 |
| **Contexto** | Auditoria completa de cobertura + expurgo de documentação fictícia (Don's order) |
| **Adaptação Necessária?** | Sim — expandido para 4 agentes |
| **Adaptação** | Adicionado architecture como 4º agente (identificou 13 barreiras de testabilidade). Demais agentes (discovery, QA, testing) mantiveram funções originais do padrão. |
| **Outcome** | ✅ success — 78 pacotes mapeados, 20 race conditions detectadas, thresholds unificados, 12 arquivos expurgados |
| **Lição do Reuso** | O padrão escala de 3→4→8 agentes sem degradação. A adição de architecture trouxe a dimensão de barreiras estruturais (P0: init() com log.Fatal, singletons globais, os.Getenv) que não seriam detectadas pelos 3 agentes originais. |

##### B4-003 — Terceira Reaplicação: Systemic Platform Audit (L20)
| Campo | Valor |
|-------|-------|
| **Data** | 2026-07-29 |
| **Contexto** | Avaliação sistêmica completa da plataforma em 10 dimensões |
| **Adaptação Necessária?** | Sim — escalado massivamente para 8 agentes |
| **Adaptação** | Padrão expandido para cobertura máxima: discovery, architecture, qa, security, devops, documentation, memory-chief, technical-debt, runtime — cada um em uma dimensão independente. Síntese consolidada em nota única 6.8/10. |
| **Outcome** | ✅ success — auditoria completa em < 5 min (8 agentes paralelos) + 5 correções em < 30 min |
| **Lição do Reuso** | O padrão escala de 3→8 agentes com eficiência: tempo constante (~5 min), qualidade crescente (mais dimensões = diagnóstico mais completo). O limite prático parece ser ~10 agentes (sobreposição de domínios começa a ocorrer). |

#### Evolução do Padrão

```
cross-agent-audit v1 (L18)
  └─ 3 agentes: discovery + QA + testing
     └─ cross-agent-audit v2 (L19)
        └─ 3 agentes com foco em extrair-para-testar
           └─ cross-agent-audit v3 (L21)
              └─ 4 agentes: + architecture (barreiras estruturais)
                 └─ cross-agent-audit v4 (L20)
                    └─ 8 agentes: todas as dimensões da plataforma
```

---

### PADRÃO 2: Parallel Orchestration (Orquestração Paralela em Ondas)

| Campo | Valor |
|-------|-------|
| **ID** | parallel-orchestration |
| **Nome** | Padrão de Orquestração Paralela em Ondas |
| **Descrição** | Ativação progressiva de agentes em ondas (analytical → implementation → review), com dependências entre agentes resolvidas via contexto fornecido no prompt (não via execução sequencial). Cada onda herda o contexto da anterior, mas agentes dentro da mesma onda executam em paralelo. |
| **Descoberto em** | Onda 2 (2026-07-28) — 10-Agent Parallel Activation |
| **Versão atual** | v5 (refinada com cross-audit synthesis) |
| **Contagem de reusos** | 5 |
| **Taxa de sucesso** | 100% (5/5) |

#### Histórico de Reusos

| ID | Data | Contexto | Onda | Agentes | Outcome |
|----|------|----------|------|---------|---------|
| B4-004 | 2026-07-28 | Onda 2 — 10 agentes em 3 ondas (A: analytical, B: implementation, C: review) | 2 | 10 | ✅ success |
| B4-005 | 2026-07-28 | Onda 3 — 9 especialistas com tasks de implementação concreta | 3 | 9 | ✅ success |
| B4-006 | 2026-07-28 | Onda 5 — 6 agentes de negócio (AI, Analytics, Infrastructure, Provider, Mobile, Platform) | 5 | 6 | ✅ success |
| B4-007 | 2026-07-28 | Onda 6 — 8 agentes (liderança + órfãos) | 6 | 8 | ✅ success (7/8, 1 gated) |
| B4-008 | 2026-07-28 | Semantic Memory Deploy — 6 agentes em 2 fases | Fase C | 6 | ✅ success |

#### Lições Agregadas do Padrão

1. **Contexto no prompt resolve dependências**: Agentes não precisam executar sequencialmente se o contexto da onda anterior for incluído no prompt da onda seguinte.
2. **Três ondas é o sweet spot**: analytical (define padrões) → implementation (constrói) → review (valida). Mais de 3 ondas gera diminishing returns.
3. **Cross-audit synthesis emerge naturalmente**: Agentes em ondas diferentes correlacionam descobertas (ex: platform correlacionou achados de infra + provider na Onda 5).
4. **Gates de ativação são saudáveis**: O paradigm (Onda 6) foi corretamente gated — isso não é falha do padrão, é o padrão funcionando.

---

### PADRÃO 3: Heuristics Applied (Heurísticas Aplicadas)

| Campo | Valor |
|-------|-------|
| **ID** | heuristics-applied |
| **Nome** | Aplicação de Heurísticas Extraídas |
| **Descrição** | Heurísticas (H-001 a H-020) extraídas da execução de 17 agentes, formalizadas e aplicadas em decisões subsequentes para evitar repetição de erros ou acelerar diagnósticos. |
| **Descoberto em** | Fase C-D Evolution (2026-07-28) |
| **Versão atual** | v1 (extração inicial) |
| **Contagem de reusos** | 4 |
| **Taxa de sucesso** | 100% (4/4) |

#### Histórico de Reusos

| ID | Heurística | Contexto | Outcome |
|----|-----------|----------|---------|
| B4-009 | H-001 (jail.go 0% coverage) | Priorização de teste de segurança no gap analysis pós-coverage audit. Heurística: "Código de segurança sem cobertura = código invisível." | ✅ Aplicada em L18 → jail.go priorizado para teste |
| B4-010 | H-002 (threshold único) | Unificação dos 4 valores conflitantes de coverage gate (40%, 55%, 70%, 80%). Heurística: "O threshold que vale é o que o CI executa." | ✅ Aplicada em L18/L19/L21 → threshold unificado 70% |
| B4-011 | H-009 (PostgreSQL fantasy) | Cross-source doc audit para detectar claims de infraestrutura inexistente. Heurística: "Sempre verificar docs contra go.mod + estrutura de diretórios." | ✅ Aplicada em L20 → 3 docs fictícios detectados |
| B4-012 | extract-then-test | Refatoração de monolitos para melhorar cobertura. Heurística: "Identificar blocos extraíveis → extrair → testar. Função pequena = trivial de cobrir." | ✅ Aplicada em L19 → runServe refatorado com sucesso |

---

## Dashboard

### Reuso por Padrão (Gráfico de Barras)

```
Parallel Orchestration  ████████████████████████ 5
Cross-Agent Audit       ████████████ 3
Heuristics Applied      ████████████████ 4
                        ────────────────────
                        TOTAL               12
```

### Taxa de Crescimento de Reuso

```
Reusos acumulados ao longo do tempo:

12 ┤                                          ● (2026-07-30)
   │                                    ▄▄▄▄
 9 ┤                              ▄▄▄▄▄
   │                        ▄▄▄▄▄
 6 ┤                  ▄▄▄▄▄
   │            ▄▄▄▄▄
 3 ┤      ▄▄▄▄▄
   │ ▄▄▄▄▄
 0 ┼─────────────────────────────────────────────►
   28/Jul manhã   28/Jul tarde   29/Jul   30/Jul
```

### Eficiência de Reuso

| Padrão | Descoberto | Reusos | Dias desde descoberta | Reusos/dia |
|--------|-----------|--------|----------------------|------------|
| parallel-orchestration | 28/Jul | 5 | 2 | 2.5/dia |
| cross-agent-audit | 29/Jul | 3 | 1 | 3.0/dia |
| heuristics-applied | 28/Jul | 4 | 2 | 2.0/dia |

---

## Schema de Entrada (para novos reusos)

```yaml
# Template para registrar novo reuso
novo_reuso:
  id: "B4-YYYY-MM-DD-NNN"
  data_reuso: YYYY-MM-DD
  conhecimento_fonte:
    tipo: "pattern | heuristic | learning | principle"
    id: "identificador da fonte"
    learning_origem: "LXX"
  contexto_reuso:
    task: "Descrição da tarefa onde foi reaplicado"
    learning_destino: "LXX"
    dominio: "domínio da aplicação"
  resultado_reuso:
    outcome: "success | partial_success | failure"
    adaptacao_necessaria: true | false
    adaptacao_descricao: "Descrição da adaptação (se houve)"
    licao_reuso: "O que este reuso ensinou sobre o padrão"
  contagem_acumulada: N   # Quantas vezes este conhecimento foi reusado até agora
```

---

## Metas

| Meta | Valor | Status |
|------|-------|--------|
| Curto prazo: identificar ≥ 2 novos reusos | 2+ novos | ⬜ 0/2 nesta sessão |
| Médio prazo: total de reusos | ≥ 20 | ⬜ 12/20 (60%) |
| Fase 1: reusos totais + padrões documentados | ≥ 15 reusos, ≥ 4 padrões | ⬜ 12/15, 3/4 padrões |
| Fase 3: reusos + padrões versionados | ≥ 50 reusos, ≥ 10 padrões | ⬜ 12/50 (24%) |

---

> **Última atualização**: 2026-07-30 | **Próxima revisão**: após próximo reuso documentado
> **Princípio**: Conhecimento reutilizado é ROI cognitivo — o custo de aprender uma vez é amortizado sobre N aplicações. Esta métrica separa um sistema que "executa" de um sistema que "aprende".
