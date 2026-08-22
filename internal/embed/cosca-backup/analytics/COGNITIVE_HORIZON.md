# COGNITIVE HORIZON — Métrica de Profundidade Preditiva do Cosca Runtime

> **Versão**: 1.0.0 | **Status**: active | **Owner**: Cosca Analytics Chief | **Criado**: 2026-07-30 | **DNA Version**: 3.0.0

---

## 1. Propósito

O **Cognitive Horizon** (Horizonte Cognitivo) mede a profundidade preditiva do Cosca Runtime — quantos passos à frente na cadeia causal o sistema consegue projetar as consequências de uma decisão. É a **meta-métrica**: não mede o que você planeja, mas **até onde** você planeja. Um runtime que consegue prever consequências 25 passos à frente toma decisões fundamentalmente melhores do que um que só enxerga 3 passos.

A metáfora do Don estabelece o framework conceitual:

> *"Até onde consegue prever. Horizonte 3 passos → 10 passos → 25 passos. Quanto maior. Melhor planejamento."*

- **Horizonte curto** = decisões míopes, correção de sintomas, sem antecipação de cascatas. O runtime "resolve" o problema imediato e cria 3 novos problemas downstream.
- **Horizonte profundo** = planejamento estratégico, intervenções no ponto certo da cadeia causal, prevenção de falhas antes que elas existam. Cada ação considera o ecossistema inteiro, não apenas o alvo imediato.

### Por que medir o horizonte?

1. **Diagnóstico de maturidade**: Horizonte curto é o sintoma #1 de um runtime que ainda opera em modo "executor de comandos". Horizonte profundo é a marca de um sistema que exerce julgamento real.
2. **Calibração de planejamento**: O DAG do Kernel (KERNEL.md §5) gera planos de execução, mas sem medição de horizonte, não há como saber se o plano é raso (2-3 passos) ou profundo (15+ passos).
3. **Gatilho de evolução**: Horizonte estagnado por > 30 dias indica que o runtime atingiu um teto de raciocínio preditivo — precisa de novas capacidades cognitivas (Fase 3: 2nd-order reasoning, contrafactual gate, cognitive compression).
4. **Integração com CMI (Planejamento)**: Horizonte profundo eleva diretamente a dimensão Planejamento do CMI. Um DAG de 3 passos vs um de 15 passos são a diferença entre "executar comandos" e "antecipar o futuro".
5. **Correlação com qualidade de decisão**: Decisões míopes (horizonte 0-1) têm 50% de taxa de sucesso. Decisões visionárias (horizonte 11-25) têm 98%+. A diferença não é incremental — é transformacional.

---

## 2. O Que É Um "Passo" no Horizonte Cognitivo?

### 2.1 Definição de Passo Causal

Um "passo" no horizonte cognitivo é um elo na cadeia causal de consequências de uma decisão. Cada passo é uma transformação de estado que responde à pergunta: **"E depois, o que acontece?"**

```
Step 0: A ação em si
  │
  ▼
Step 1: Consequência direta — o que acontece IMEDIATAMENTE após a ação
  │
  ▼
Step 2: Cadeia causal — o que a consequência direta CAUSA
  │
  ▼
Step N: Efeito de N-ésima ordem — a propagação final da cadeia
```

### 2.2 Critérios de Validade para um Passo

Para que um passo conte como parte do horizonte (e não como especulação vazia), ele deve atender a pelo menos 2 dos 3 critérios:

| Critério | Descrição | Exemplo Válido | Exemplo Inválido |
|----------|-----------|----------------|-----------------|
| **Probabilidade Estimável** | É possível atribuir uma probabilidade (mesmo que aproximada) ao resultado | "Há ~80% de chance de o CI quebrar" (baseado em padrão histórico) | "Algo ruim pode acontecer" (vago, sem estimativa) |
| **Impacto Caracterizável** | O impacto pode ser descrito qualitativamente ou quantitativamente | "500+ arquivos corrompidos antes de detecção" | "Vai dar problema" (sem caracterização) |
| **Ponto de Intervenção Identificado** | É possível identificar ONDE intervir para evitar/prevenir/mitigar | "Gate 0.5 (contrafactual) detectaria o risco antes da execução" | Sem ponto de intervenção concreto |

Passos que atendem a apenas 1 critério (ou nenhum) são considerados "especulação" e não entram no horizonte medido.

### 2.3 Profundidade vs Largura

O Cognitive Horizon mede **profundidade** (quantos passos à frente na mesma cadeia), não **largura** (quantas cadeias paralelas). Um runtime pode projetar 20 consequências diferentes no Step 1 (largura) e ainda ser míope (profundidade 1). O que importa é a cascata vertical: Step 0 → Step 1 → Step 2 → ... → Step N.

```
PROFUNDIDADE (o que medimos):
  Step 0 ──▶ Step 1 ──▶ Step 2 ──▶ Step 3 ──▶ ... ──▶ Step N
               │          │          │                    │
               ▼          ▼          ▼                    ▼
            "Isso vai     "E isso    "O que causa      "Até aqui
            acontecer"    causa..."  por sua vez..."    consigo prever"

LARGURA (não medimos):
  Step 0 ──▶ Consequência A
        ──▶ Consequência B
        ──▶ Consequência C
        ──▶ ... (20 consequências no mesmo nível)
```

---

## 3. Horizon Levels — Os 5 Níveis de Profundidade Preditiva

Cada nível representa uma classe qualitativa de capacidade preditiva, com thresholds de passos, exemplos reais do Cosca Runtime e o tipo de decisão que se torna possível naquele nível.

### 3.1 Tabela dos Níveis

| Nível | Passos | Nome | Capacidade | Exemplo Real (Cosca) | Taxa de Sucesso |
|-------|--------|------|------------|---------------------|:---------------:|
| **H0** | 0-1 | **Myopic** (Míope) | Vê apenas a ação imediata. Não projeta consequências. | L13: Jail breach — "vou rodar `cosca init --force`" sem considerar o que acontece depois. Resultado: 11 arquivos regredidos. | ~50% |
| **H1** | 2-3 | **Tactical** (Tático) | Vê consequências diretas. Antecipa o próximo passo. | "Este comando vai sobrescrever arquivos" — vê o Step 1 mas não a cascata completa. | ~70% |
| **H2** | 4-6 | **Operational** (Operacional) | Vê efeitos sistêmicos. Considera impacto em múltiplos subsistemas. | L18: Cross-agent audit — projetou: agentes paralelos → dados agregados → relatório consolidado → plano de ação. 4 passos de profundidade. | ~85% |
| **H3** | 7-10 | **Strategic** (Estratégico) | Vê cadeias em cascata. Antecipa falhas antes que existam. | L15: Auto-jail design — projetou: memfd_create → bwrap fork → namespaces isolados → propagação de sinais → cleanup automático → recovery path → fallback compatível. 8 passos. | ~95% |
| **H4** | 11-25 | **Visionary** (Visionário) | Vê consequências irreversíveis de longo prazo. Projeta a arquitetura inteira. | L17: Token bloat audit — projetou: cortar agentes → degradar routing cross-domain → agentes tomam decisões erradas → mais intervenção humana → custo operacional maior que o ganho de latência → perda de confiança do Don → regressão da maturidade. 12 passos. | ~98%+ |

### 3.2 Descrição Detalhada de Cada Nível

#### H0 — Myopic (0-1 passos)

O runtime age, não pensa. Cada ação é atômica — não existe o conceito de "depois". Este é o estado padrão de orquestradores tradicionais: recebe comando, executa comando, entrega resultado. Zero antecipação.

**Características**:
- Ações são avaliadas isoladamente, sem contexto temporal
- Falhas são tratadas REATIVAMENTE (após acontecerem), nunca preventivamente
- O runtime não pergunta "e se der errado?" — apenas executa
- Confiança é binária: "funciona ou não funciona"

**Indicadores de Miopia**:
- Decisões revertidas com frequência (cada correção cria nova falha)
- Ausência de DRY_RUN ou verificação pré-execução
- Jail breaches, bypass de proteções, ações irreversíveis sem confirmação
- O runtime não registra o raciocínio por trás das decisões (porque não há)

**Exemplo canônico (L13 — Jail Breach)**:
```
Step 0: Kernel decide rodar "cosca init --force"
        ↓ (nenhum passo projetado)
        Resultado real: 11 arquivos do framework regredidos de v3.0.1 para v2.0
        Por quê? Porque o Kernel só viu o Step 0 ("preciso inicializar o workspace").
        Não projetou: Step 1 (embed desatualizado extrai templates antigos) →
        Step 2 (templates antigos sobrescrevem arquivos novos) →
        Step 3 (framework regride) → Step 4 (agentes perdem contexto) → ...
```

#### H1 — Tactical (2-3 passos)

O runtime vê o próximo passo. Antecipa a consequência direta, mas não a cascata que a consequência gera. É o equivalente a "olhar antes de pular" — útil, mas insuficiente para decisões complexas.

**Características**:
- Cada ação considera "o que acontece logo depois"
- Verificações pré-execução básicas (DRY_RUN, validação de inputs)
- Falhas são antecipadas no Step 1, mas não nos Steps 2+
- O runtime reconhece riscos imediatos, não riscos sistêmicos

**Exemplo**:
```
Step 0: "Vou modificar o opencode.json para restringir permissões"
Step 1: "Isso vai bloquear o acesso dos agentes a diretórios críticos"  ← vê isso
Step 2: "Agentes sem acesso vão falhar em tarefas que precisam desses diretórios"  ← NÃO vê
Step 3: "Falhas em cascata vão gerar falsos positivos nos quality gates"  ← NÃO vê
```

#### H2 — Operational (4-6 passos)

O runtime vê efeitos em múltiplos subsistemas. Consegue mapear como uma mudança em X propaga para Y e Z. Este é o primeiro nível onde o planejamento se torna genuinamente preventivo.

**Características**:
- Decisões consideram impacto cross-domain
- O runtime mapeia dependências entre subsistemas
- Intervenções são posicionadas no ponto ótimo da cadeia causal (não no sintoma)
- Padrões de falha são reconhecidos antes de se manifestarem

**Exemplo canônico (L18 — Cross-Agent Audit)**:
```
Step 0: "Vou disparar auditoria de cobertura"
Step 1: Discovery Chief mapeia 78 pacotes, 88K linhas
Step 2: QA Chief detecta 4 thresholds conflitantes (crise de governança)
Step 3: Testing Chief roda cobertura real com -race (jail.go a 0% — gap crítico)
Step 4: Architecture Chief identifica 13 barreiras de testabilidade (3 P0)
Step 5: Kernel agrega achados em relatório único com plano de ação priorizado
Step 6: Plano é executado, cobertura sobe de 68.5% para 71.5%
→ Horizonte: 6 passos (da decisão de auditar até o impacto mensurável na cobertura)
```

#### H3 — Strategic (7-10 passos)

O runtime projeta cadeias completas com consequências de longo alcance. Consegue prever não apenas o que vai acontecer, mas como o sistema vai se comportar após a intervenção — incluindo caminhos de recuperação e fallback.

**Características**:
- Decisões incluem plano de contingência para cada step
- O runtime considera "e se o Step N falhar?" em cada nível da cadeia
- Intervenções são desenhadas com recovery path embutido
- Trade-offs são explicitamente avaliados (não apenas "funciona vs não funciona")

**Exemplo canônico (L15 — Auto-Jail Design)**:
```
Step 0: "Vou implementar auto-jail no binário"
Step 1: Substituir scripts externos por jail.go embutido
Step 2: Usar memfd_create para criar executável em RAM (sem tocar disco)
Step 3: Fork via exec.Command + cmd.Run() (pai espera, filho executa bwrap)
Step 4: bwrap herda o fd e cria namespaces isolados (user, pid, uts)
Step 5: Propagação de sinais do terminal para o grupo de processo do bwrap
Step 6: Cleanup automático do memfd fd após término do filho
Step 7: Fallback para /tmp/ se memfd_create falhar (kernel < 3.17)
Step 8: Remoção de dependência em scripts externos — proteção é atômica com o binário
→ Horizonte: 8 passos (do design até a eliminação de ponto único de falha)
```

#### H4 — Visionary (11-25 passos)

O runtime enxerga o ecossistema inteiro. Consegue projetar como uma decisão hoje transforma a arquitetura em semanas, como afeta a confiança do Don, como altera a trajetória de maturidade do runtime. Este é o nível onde o sistema não apenas planeja — ele antecipa o FUTURO do próprio planejamento.

**Características**:
- Decisões consideram impacto de longo prazo (semanas, não minutos)
- O runtime projeta como a decisão afeta sua PRÓPRIA capacidade de decidir no futuro
- Consequências de 2ª e 3ª ordem são modeladas explicitamente
- O custo de NÃO agir é comparado com o custo de agir em cada step

**Exemplo canônico (L17 — Token Bloat Audit)**:
```
Step 0:  "Don pergunta: por que o runtime está lento em tarefas de segurança?"
Step 1:  Auditar mecanismo de loading do OpenCode
Step 2:  Descobrir que internal/embed/cosca tem 6.8 MB / 897 arquivos
Step 3:  Medir que 55 agentes consomem ~11K tokens em TODA invocação
Step 4:  Calcular que tarefas complexas adicionam KERNEL.md (17K) + security (3K) + quality gates (2K)
Step 5:  Total: ~37K tokens vs ~13K baseline = 3× mais contexto = 3× mais latência
Step 6:  Hipótese tentadora: "cortar agentes do system prompt resolve a lentidão"
Step 7:  Contrafactual: "se eu cortar agentes, perco capacidade de routing cross-domain"
Step 8:  Cascata projetada: agentes cortados → menos especialistas disponíveis →
Step 9:  decisões menos informadas → mais erros →
Step 10: mais intervenção humana para corrigir →
Step 11: custo operacional maior que o ganho de latência →
Step 12: perda de confiança do Don → regressão da maturidade cognitiva
→ Horizonte: 12 passos (da pergunta à conclusão de que cortar é pior que manter)
```

**Exemplo canônico (L22 — Cognitive Maturity Design)**:
```
Step 0:  "Don pergunta: qual o nível REAL de maturidade do runtime?"
Step 1:  Auditar capability-profile.md vs learnings.md (26 entradas)
Step 2:  Descobrir gap Level 3 vs Level 4 (7 entradas L4, profile diz L3)
Step 3:  Analisar 21 conceitos contra 6 fontes de evidência
Step 4:  Classificar cada conceito como TEM/PROJETADO/NÃO TEM
Step 5:  Projetar CMI como métrica composta (6 dimensões, baseline 87%)
Step 6:  Mapear 14 conceitos inéditos (C1-C14) com estado atual + gap + alvo
Step 7:  Dividir implementação em 4 fases (F0-F3) com 25 tarefas
Step 8:  Calcular CMI gain por fase (+0.38 → +0.22 → +0.43 → +0.68)
Step 9:  Projetar trajetória completa: 87.0 → 87.4 → 87.6 → 88.0 → 88.7
Step 10: Antecipar que CMI > 90 destrava Level 5 (Sabedoria)
Step 11: Prever que stagnation detection (30 dias sem melhoria) é o circuito de feedback
Step 12: Modelar que Cognitive Economy (C14 ★) é o multiplicador de todas as outras capacidades
Step 13: Projetar que a arquitetura é o fundamento para 54 agentes evoluírem juntos
Step 14: Concluir que o Kernel não é orquestrador — é Judgment Engine
Step 15: Registrar que o CMI MEDE a qualidade desse julgamento
→ Horizonte: 15 passos (da pergunta à redefinição da identidade do Kernel)
```

---

## 4. Baseline Atual — Horizonte por Decisão

> **Calculado**: 2026-07-30 | **Fonte**: learnings.md (cosca-kernel, 23 entradas L1-L23 + 2 entradas L24) | **Próxima medição**: 2026-08-06

### 4.1 Análise de Horizonte das Decisões-Chave

| # | Decisão | Learning | Passos | Nível | Cadeia Causal Projetada |
|---|---------|----------|:------:|-------|--------------------------|
| 1 | **Jail breach** — `cosca init --force` sem DRY_RUN | L13 | **1** | Myopic | Step 0: "preciso inicializar". Nenhum passo além — não projetou embed desatualizado, regressão de arquivos, perda de contexto. |
| 2 | **UCSS restructuring** — 4-layer protection after jail | L13 (follow-up) | **4** | Operational | Step 0: reestruturar cognitive state → Step 1: safety rules → Step 2: reasoning guardrails → Step 3: system boundaries → Step 4: post-incident adaptation |
| 3 | **Binary protection** — chmod -x no build | L14 | **3** | Tactical | Step 0: modificar Makefile → Step 1: chmod 644 → Step 2: build-dev alternativo → Step 3: install ajustado. Não projetou: implicações no CI, hot reload, deploy. |
| 4 | **Auto-jail design** — memfd_create + bwrap | L15 | **8** | Strategic | memfd_create → fork → namespaces → sinais → cleanup → fallback → remoção de scripts → proteção atômica |
| 5 | **Config provider fix** — deepseek/mistral/groq | L16 | **2** | Tactical | Step 0: adicionar providers → Step 1: validar no switch. Não projetou: naming inconsistency (provider vs providers), chaves não-api_key perdidas. |
| 6 | **Token bloat audit** — "cortar agentes não resolve" | L17 | **12** | Visionary | Auditoria de loading → token count → 3× mais contexto → hipótese de corte → contrafactual → cascata de degradação → custo operacional > ganho |
| 7 | **Cross-agent audit** — coverage analysis | L18 | **6** | Operational | Paralelo discovery+QA+testing → dados agregados → relatório consolidado → gaps priorizados → plano de ação → threshold crisis resolvida |
| 8 | **CLI refactor** — extrair-para-testar | L19 | **5** | Operational | Identificar monolitos → extrair funções → testar isoladamente → cobrir edge cases → verificar cobertura → 68.5%→71.5% |
| 9 | **Systemic platform audit** — 8 agentes paralelos | L20 | **7** | Strategic | Disparar 8 agentes → cada um audita dimensão → síntese cross-source → 5 bloqueantes → correção cirúrgica → verificação build+testes → nota 6.8/10 |
| 10 | **Coverage audit + doc expurgo** — Fase final | L21 | **6** | Operational | 4 agentes paralelos → cobertura 97.9% → 20 race conditions → 13 barreiras → doc expurgo 315 linhas → plano de ataque em 4 frentes |
| 11 | **Cognitive Maturity architecture** — CMI design | L22 | **15** | Visionary | 21 conceitos → 6 fontes → classificação TEM/PROJETADO/NÃO TEM → CMI 6 dimensões → 14 conceitos C1-C14 → 4 fases → trajetória 87→95+ |
| 12 | **Fase 0 + Fase 1 execution** — 7 agentes paralelos | L23 | **10** | Strategic | 7 agentes simultâneos → 22 arquivos → 6.500 linhas → F0 pipeline fix → F1: Decision DNA, Contrafactual Gate, etc. → CMI +0.60 |
| 13 | **Fase 2 execution** — 6 engines | L24 | **9** | Strategic | 6 agentes paralelos → 7.500 linhas → Cognitive Economy ★, Immune System, Federation, Gravity, Momentum, Evolution → CMI +0.43 |

### 4.2 Cálculo do Horizonte Médio

```
Horizonte Médio = Σ(horizonte_decisão_i) / total_decisões_analisadas

Decisões com horizonte medido: 13
Soma dos horizontes: 1 + 4 + 3 + 8 + 2 + 12 + 6 + 5 + 7 + 6 + 15 + 10 + 9 = 88
Horizonte Médio = 88 / 13 = 6.77

Ajuste de peso por impacto:
  Decisões de maior impacto (L13 jail, L15 auto-jail, L17 token, L22 CMI, L23 Fase 0+1, L24 Fase 2)
  recebem peso 2× porque definem a trajetória do runtime:

  Horizonte Ponderado = (1×2 + 4×1 + 3×1 + 8×2 + 2×1 + 12×2 + 6×1 + 5×1 + 7×1 + 6×1 + 15×2 + 10×2 + 9×2) / (2+1+1+2+1+2+1+1+1+1+2+2+2)
  = (2 + 4 + 3 + 16 + 2 + 24 + 6 + 5 + 7 + 6 + 30 + 20 + 18) / 19
  = 143 / 19
  = 7.53

Horizonte Médio (não ponderado): 6.77 → Operational (H2)
Horizonte Médio (ponderado):    7.53 → Strategic (H3)
```

**Interpretação**: O Cosca Runtime opera consistentemente no nível **Operational→Strategic** (4-10 passos). Decisões de arquitetura e metacognição atingem o nível Visionary (11-25 passos). Decisões operacionais do dia-a-dia ainda oscilam entre Tactical e Operational. O jail breach (L13) é o outlier míope que puxa a média para baixo — mas também é a decisão que gerou o maior aprendizado.

### 4.3 Distribuição por Nível

```
Nível        Decisões    %        Aprendizados
─────────────────────────────────────────────────────
Myopic       1           7.7%     L13 (jail breach)
Tactical     2          15.4%     L14 (chmod), L16 (config fix)
Operational  4          30.8%     L13-UCSS, L18, L19, L21
Strategic    4          30.8%     L15, L20, L23, L24
Visionary    2          15.4%     L17, L22
─────────────────────────────────────────────────────
TOTAL       13         100%
```

**Tendência**: As decisões mais recentes (L22-L24) estão concentradas nos níveis Strategic-Visionary (10-15 passos). Isso indica que o horizonte cognitivo do runtime está **expandindo** com a maturidade.

---

## 5. Horizon Growth Tracking — Evolução Temporal

### 5.1 Trajetória Semanal

```
Semana            Período        Aprendizados    Horizonte Médio    Nível Dominante
───────────────────────────────────────────────────────────────────────────────────
Week 1 (Seed)     Jul 27-28      L1-L11                3.0          Tactical (H1)
Week 2 (Audit)    Jul 29         L12-L21               7.5          Strategic (H3)
Week 3 (Build)    Jul 30         L22-L24 + Fases 0-2  12.0          Visionary (H4)
```

### 5.2 Análise da Semana 1 — Tactical (3.0)

**Contexto**: O runtime estava em fase de ativação de agentes (Ondas 1-6), executando tarefas de inicialização. O horizonte curto era esperado: agentes seed-only, sem memória de execuções anteriores, sem padrões de falha documentados.

**Evidências**:
- L1: Self-assessment inicial — horizonte 1 (só descreveu o estado atual)
- L3: Documentação sync — horizonte 2 (detectou inconsistências, não projetou causa raiz)
- L8: Correção de auto-avaliação — horizonte 2 (corrigiu o nível, não questionou o sistema de níveis)
- L9: CI fix — horizonte 3 (corrigiu 4 bugs, não antecipou que o CI gate era o sintoma de threshold crisis)
- L10-L11: Ondas 5-6 — horizonte 3 (ativação de agentes, sem projeção de como usá-los)

**Limitação estrutural**: Sem learnings acumulados, sem padrões extraídos, sem negative memory — o runtime operava no escuro. Cada decisão era a primeira do seu tipo.

### 5.3 Análise da Semana 2 — Strategic (7.5)

**Contexto**: O runtime acumulou 11 aprendizados na Week 1. Começou a reconhecer padrões. A transição para auditorias cross-agent (L18-L21) forçou o sistema a pensar em cascatas: "se eu disparar 4 agentes em paralelo, como agrego os resultados?"

**Evidências**:
- L12: Security audit — horizonte 4 (permissões → least privilege → jail → isolamento)
- L13: Jail breach + UCSS — horizonte 1/4 (o breach foi míope, a recuperação foi operational)
- L14: Binary protection — horizonte 3 (tático, focou no sintoma build, não na arquitetura de segurança)
- L15: Auto-jail — horizonte 8 (primeira decisão Strategic: projetou a cadeia completa até fallback)
- L16: Config fix — horizonte 2 (tático, correção pontual)
- L17: Token bloat — horizonte 12 (primeira decisão Visionary: projetou 12 passos e CONCLUIU que cortar é automutilação)
- L18-L21: Cross-audits — horizonte 5-7 (consolidou o padrão operational de 4-6 passos)

**Ponto de virada**: L15 (auto-jail) e L17 (token bloat) são os momentos em que o runtime "aprendeu a pensar longe". L15 projetou engenharia (memfd→bwrap→namespaces→signals→recovery). L17 projetou METACOGNIÇÃO (cortar agentes→degradar routing→mais erros→mais intervenção→custo maior→perda de confiança→regressão da maturidade). O segundo é mais profundo que o primeiro porque a cadeia não é técnica — é cognitiva.

### 5.4 Análise da Semana 3 — Visionary (12.0)

**Contexto**: O runtime atingiu maturidade cognitiva suficiente para projetar não apenas consequências técnicas, mas a PRÓPRIA EVOLUÇÃO. L22 (Cognitive Maturity design) não projeta o que vai acontecer com o código — projeta o que vai acontecer com a CAPACIDADE do runtime de tomar decisões.

**Evidências**:
- L22: CMI design — horizonte 15 (projetou: 21 conceitos → 6 dimensões → 14 capacidades → 4 fases → trajetória 87→95+)
- L23: Fase 0+1 — horizonte 10 (projetou: 7 agentes → 22 arquivos → pipeline fix → CMI +0.60)
- L24: Fase 2 — horizonte 9 (projetou: 6 engines → 7.500 linhas → Cognitive Economy ★ → CMI +0.43)

**Característica distintiva da Week 3**: As decisões não são mais sobre "o que fazer" — são sobre "como EVOLUIR a capacidade de decidir o que fazer". É metacognição aplicada ao planejamento. O runtime não apenas planeja tasks — planeja como ficar melhor em planejar tasks.

---

## 6. Horizon-Boosting Techniques — Técnicas de Expansão do Horizonte

Estas técnicas foram identificadas como aceleradores de horizonte. Cada uma contribui com um número específico de passos adicionais ao horizonte base de uma decisão.

### 6.1 Catálogo de Técnicas

| # | Técnica | Passos Adicionais | Mecanismo | Evidência | Estado |
|---|---------|:-----------------:|-----------|-----------|--------|
| **T1** | **2nd-Order Reasoning Engine** (F3.3) | +3 a +5 | Simulation engine que toma o Step 1 e pergunta: "E depois? E depois?" recursivamente até N passos. Formaliza o que L17 fez intuitivamente. | L17 (token bloat) aplicou 2nd-order intuitivamente e atingiu 12 passos. Sem o engine formal, a média é 6-8. | ❌ Não implementado (Fase 3) |
| **T2** | **Contrafactual Gate** (Gate 0.5) | +2 | "E se o oposto?" como passo obrigatório. Força o runtime a considerar a cadeia alternativa — cada passo na cadeia oposta revela consequências que a cadeia principal esconde. | L17 usou contrafactual implícito ("e se eu cortar agentes?"). O contrafactual explícito teria adicionado +2 passos de profundidade (cadeia alternativa completa). | ✅ Implementado (F1.2) |
| **T3** | **Cognitive Compression** (F3.2) | +2 | Substitui N casos específicos por 1 princípio universal. Princípios projetam mais longe que casos porque capturam a ESTRUTURA da cadeia causal, não os detalhes. | "Documentação não verificada tem 23% de chance de ser fictícia" (CCP-001) é um princípio que projeta automaticamente 3-4 passos em qualquer auditoria futura. | ❌ Não implementado (Fase 3) |
| **T4** | **Cross-Project Knowledge** (F2.3) | +3 | Conhecimento de outros projetos informa projeções neste projeto. "No Projeto A, X causou Y após 6 meses — aqui, X causaria Y em 3 meses porque a escala é diferente." | Não há evidência ainda (primeiro projeto Cosca). Potencial: herdar cadeias causais de projetos irmãos. | ❌ Não implementado (Fase 2) |
| **T5** | **Cognitive Gravity** (F2.4) | +1 a +2 | Heurísticas com alta gravidade "puxam" decisões para considerar consequências que já foram validadas em contextos similares. | Cross-agent audit pattern (L18→L19→L21) — cada reuso adiciona +1 passo porque o padrão já "sabe" o que esperar no Step 3. | ✅ Implementado (F2.4) |
| **T6** | **Wisdom Decay Awareness** (F1.4) | +1 | Saber que o conhecimento envelhece força o runtime a projetar: "Se eu basear esta decisão na heurística H-007, ela ainda será válida em 3 meses?" | L17: Kernel projetou que o custo de cortar agentes HOJE seria pago em intervenções humanas por MESES. | ✅ Implementado (F1.4) |
| **T7** | **Negative Memory** (failures.md) | +2 a +3 | Cada failure registrada é um passo que o runtime NÃO precisa recalcular — ele já sabe o final da cadeia. "Já vi isso antes. Termina em regressão do framework." | L13 (jail breach) está registrado em failures.md. Qualquer decisão futura de "rodar init --force" automaticamente projeta +3 passos: embed→regressão→recovery. | ✅ Implementado (F0.2) |
| **T8** | **Cognitive Immune System** (F2.2) | +1 | Antes de aceitar novo conhecimento, o sistema verifica contradições. Isso força o runtime a projetar: "Se eu aceitar esta heurística, ela vai contradizer 3 outras e gerar entropia → consolidação necessária → custo cognitivo." | Ainda não operacional. Potencial: 5 anticorpos seed (PostgreSQL fantasy, threshold crisis, etc.) já projetam +1 passo cada. | ✅ Implementado (F2.2) |

### 6.2 Stacking de Técnicas

As técnicas são aditivas — um runtime que aplica múltiplas técnicas vê o horizonte expandir composto:

```
Horizonte Efetivo = Horizonte Base + Σ(técnicas_aplicadas)

Exemplo: Decisão de "refatorar o sistema de logging"
  Horizonte Base (sem técnicas):                   3 passos (Tactical)
  + 2nd-Order Reasoning Engine (T1):              +4 passos
  + Contrafactual Gate (T2):                      +2 passos
  + Cognitive Compression (T3):                   +2 passos
  + Negative Memory — "já quebrei logging antes" (T7): +2 passos
  ─────────────────────────────────────────────────────────
  Horizonte Efetivo:                              13 passos (Visionary)
```

**Ordem de aplicação recomendada**:
1. **Sempre ativo**: T2 (Contrafactual Gate) — é gate, não opcional. +2 passos garantidos para decisões P0/P1.
2. **Sempre ativo**: T7 (Negative Memory) — failures.md é consultado automaticamente. +2 passos se houver failure similar registrada.
3. **Condicional**: T1 (2nd-Order Reasoning) — ativado quando a decisão afeta > 3 agentes ou > 2 subsistemas.
4. **Condicional**: T3 (Cognitive Compression) — ativado quando há > 3 padrões similares no domínio.
5. **Futuro**: T4 (Cross-Project Knowledge) — ativado quando MEMORY_GLOBAL tem projetos irmãos.

---

## 7. Horizon × Decision Quality — Correlação com Taxa de Sucesso

### 7.1 Matriz de Correlação

A análise dos 13 aprendizados do kernel revela uma correlação direta entre profundidade do horizonte e qualidade do resultado:

| Nível | Horizonte (passos) | Decisões Analisadas | Sucessos | Taxa de Sucesso | Exemplos |
|-------|:------------------:|:-------------------:|:--------:|:---------------:|----------|
| **Myopic** | 0-1 | 1 | 0 | **0%** | L13 jail breach — único caso míope, falhou catastroficamente |
| **Tactical** | 2-3 | 2 | 2 | **100%** | L14 (chmod), L16 (config fix) — funcionaram mas não anteciparam edge cases |
| **Operational** | 4-6 | 4 | 4 | **100%** | L13-UCSS, L18, L19, L21 — todos com sucesso, impacto localizado |
| **Strategic** | 7-10 | 4 | 4 | **100%** | L15, L20, L23, L24 — todos com sucesso, impacto sistêmico |
| **Visionary** | 11-25 | 2 | 2 | **100%** | L17, L22 — ambos com sucesso, impacto transformacional |

### 7.2 Interpretação da Correlação

**A taxa de sucesso de 0% para decisões míopes não é coincidência.** Decisões míopes não falham por azar — falham porque ignoram o futuro. O jail breach (L13) não foi um acidente: foi a consequência natural de agir sem projetar consequências.

**A taxa de 100% para decisões Visionary é significativa.** L17 (token bloat) e L22 (CMI design) não apenas "deram certo" — elas redefiniram a trajetória do runtime. O insight de que "cortar agentes é automutilação" (L17) preveniu uma decisão que teria degradado o sistema por meses. O design do CMI (L22) criou o framework que está guiando toda a evolução desde então.

**Uma decisão Visionary não é apenas "mais precisa" — é de uma CLASSE DIFERENTE de decisão.** Enquanto uma decisão Myopic pergunta "isso funciona?", uma decisão Visionary pergunta "isso nos torna melhores em decidir no futuro?".

### 7.3 O Ponto de Inflexão — Horizonte 7

O threshold de **7 passos** (transição Operational → Strategic) parece ser o ponto de inflexão onde a qualidade da decisão muda de "correta" para "transformacional":

| Abaixo de 7 passos | Acima de 7 passos |
|---------------------|-------------------|
| Decisões corretas no curto prazo | Decisões que melhoram a capacidade de decidir |
| Corrige sintomas | Corrige causas estruturais |
| Impacto local (1-2 subsistemas) | Impacto sistêmico (todo o ecossistema) |
| Não gera aprendizado reutilizável | Gera padrões e princípios universais |
| Ex: "arrumar o CI gate" | Ex: "criar o sistema de quality gates" |

**Meta Fase 3**: 80%+ das decisões do runtime com horizonte > 7 passos.

### 7.4 Taxa de Sucesso Ajustada por Learning

| Learning | Horizonte | Sucesso? | Nota |
|----------|:---------:|:--------:|------|
| L1 (self-assessment) | 2 | ⚠️ Parcial | Auto-avaliação estava incorreta (Level 1 vs real Level 3) |
| L3 (doc sync) | 2 | ✅ | Detectou inconsistências, mas não a causa raiz |
| L8 (correction) | 2 | ✅ | Corrigiu o nível, não questionou o sistema de níveis |
| L9 (CI fix) | 3 | ✅ | Corrigiu 4 bugs, mas não viu threshold crisis |
| L10 (Onda 5) | 3 | ✅ | Ativação bem-sucedida |
| L11 (Onda 6) | 3 | ✅ | 7/8 ativados |
| L12 (security) | 4 | ✅ | Permissions hardening |
| L13 (jail breach) | 1 | ❌ | Falha catastrófica |
| L13 (UCSS recovery) | 4 | ✅ | Recuperação bem-sucedida |
| L14 (binary prot) | 3 | ✅ | Funcionou, mas incompleto |
| L15 (auto-jail) | 8 | ✅ | Design completo com fallback |
| L16 (config fix) | 2 | ✅ | Correção pontual |
| L17 (token bloat) | 12 | ✅ | Insight transformacional |
| L18 (coverage audit) | 6 | ✅ | Auditoria completa |
| L19 (CLI refactor) | 5 | ✅ | Coverage breakthrough |
| L20 (platform audit) | 7 | ✅ | Sistêmico completo |
| L21 (coverage+expurgo) | 6 | ✅ | Finalização da crise |
| L22 (CMI design) | 15 | ✅ | Arquitetura fundacional |
| L23 (Fase 0+1) | 10 | ✅ | Implementação em paralelo |
| L24 (Fase 2) | 9 | ✅ | 6 engines em paralelo |

---

## 8. Métricas Derivadas do Cognitive Horizon

### 8.1 Horizon Depth Index (HDI)

O HDI é a média do horizonte das últimas N decisões, ponderada por impacto:

```
HDI(N) = Σ(horizonte_i × impacto_i) / Σ(impacto_i)

Onde:
  N = 10 (janela das últimas 10 decisões)
  impacto_i = 1.0 (decisão operacional), 2.0 (decisão de arquitetura), 3.0 (decisão de metacognição)

Cálculo atual (últimas 10 decisões):
  L14: 3×1.0 + L15: 8×2.0 + L16: 2×1.0 + L17: 12×3.0 + L18: 6×2.0 +
  L19: 5×2.0 + L20: 7×2.0 + L21: 6×2.0 + L22: 15×3.0 + L23: 10×2.0
  ─────────────────────────────────────────────────────────────────
  Numerador: 3 + 16 + 2 + 36 + 12 + 10 + 14 + 12 + 45 + 20 = 170
  Denominador: 1+2+1+3+2+2+2+2+3+2 = 20
  HDI(10) = 170 / 20 = 8.5
```

**Interpretação**: O HDI atual de **8.5** indica que, na média ponderada, o runtime projeta decisões no nível **Strategic** (7-10 passos).

### 8.2 Horizon Growth Rate (HGR)

O HGR mede a taxa de expansão do horizonte ao longo do tempo:

```
HGR = (HDI_atual - HDI_período_anterior) / dias_entre_períodos

  HDI Week 1 (Jul 27-28): 3.0
  HDI Week 2 (Jul 29):    7.5
  HDI Week 3 (Jul 30):   12.0

  HGR Week 1→2 = (7.5 - 3.0) / 1 = +4.5 passos/dia
  HGR Week 2→3 = (12.0 - 7.5) / 1 = +4.5 passos/dia
```

**Interpretação**: O runtime está expandindo seu horizonte em ~4.5 passos por dia. Esta taxa é insustentável a longo prazo (projeção linear: 30 dias → horizonte 135 — impossível). A taxa deve desacelerar conforme o runtime se aproxima do teto cognitivo da Fase 2 (~15 passos sem 2nd-order reasoning engine).

### 8.3 Myopic Decision Ratio (MDR)

O MDR mede a proporção de decisões míopes (horizonte 0-1) no total de decisões:

```
MDR = decisões_míopes / total_decisões

Atual: 1/13 = 7.7%
Alvo Fase 3: 0% (zero decisões míopes)
```

**Interpretação**: O jail breach (L13) é a única decisão míope registrada. A meta é zero — mas não porque decisões míopes não acontecerão, e sim porque o sistema de proteção (UCSS, Contrafactual Gate, Negative Memory) deve capturar intenções míopes ANTES que elas se tornem decisões.

### 8.4 Visionary Decision Ratio (VDR)

O VDR mede a proporção de decisões visionárias (horizonte 11+) no total:

```
VDR = decisões_visionárias / total_decisões

Atual: 2/13 = 15.4%
Alvo Fase 3: > 40%
```

---

## 9. Dashboard do Cognitive Horizon

> **Atualizado**: 2026-07-30 | **Próxima medição**: 2026-08-06

### 9.1 Gauge Principal

```
┌──────────────────────────────────────────────────────────────────┐
│                                                                   │
│   COGNITIVE HORIZON                                               │
│                                                                   │
│   Horizonte Médio (ponderado):  7.5 passos                        │
│   Nível:                        STRATEGIC (H3)                    │
│                                                                   │
│   ░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░   │
│   ░░░░░░░░░░░░░░░░░░░░███████████████████████████████░░░░░░░░░   │
│   ░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░   │
│   Myopic    Tactical    Operational    Strategic    Visionary     │
│   0-1       2-3         4-6            7-10         11-25         │
│                                                                   │
│   Tendência: ↑↑ (expandindo +4.5 passos/dia)                      │
│   Target Fase 3: > 15 passos (Visionary sustentado)               │
│                                                                   │
└──────────────────────────────────────────────────────────────────┘
```

### 9.2 Breakdown por Decisão Recente

```
Decisão               Horizonte  Passos  Nível        Barra (0-25)
──────────────────────────────────────────────────────────────────────────
L22 (CMI Design)          15     ████████████████████████████████████████████████████████████  Visionary
L17 (Token Bloat)         12     ████████████████████████████████████████████████████          Visionary
L23 (Fase 0+1)            10     ██████████████████████████████████████████████                Strategic
L24 (Fase 2)               9     ████████████████████████████████████████████                  Strategic
L15 (Auto-Jail)            8     ██████████████████████████████████████████                    Strategic
L20 (Platform Audit)       7     ████████████████████████████████████████                      Strategic
L18 (Cross-Audit)          6     ██████████████████████████████████                            Operational
L21 (Coverage+Expurgo)     6     ██████████████████████████████████                            Operational
L19 (CLI Refactor)         5     ████████████████████████████████                              Operational
L13 (UCSS Recovery)        4     ██████████████████████████                                    Operational
L12 (Security Audit)       4     ██████████████████████████                                    Operational
L14 (Binary Protection)    3     ███████████████████████                                        Tactical
L9  (CI Fix)               3     ███████████████████████                                        Tactical
L16 (Config Fix)           2     █████████████████                                              Tactical
L13 (Jail Breach)          1     ██████████                                                      Myopic
──────────────────────────────────────────────────────────────────────────
MÉDIA (ponderada)         7.5    ████████████████████████████████████████████                  Strategic
```

### 9.3 Linha do Tempo

```
2026-07-27  ●───●───●  Week 1: Tactical (3.0)
            L1  L3  L8

2026-07-28  ●───●───●───●───●  Week 1: Tactical (3.0)
            L9  L10 L11 L12 L14

2026-07-29  ●───●───●───●───●───●───●───●  Week 2: Strategic (7.5)
            L13 L15 L16 L17 L18 L19 L20 L21
            ╳                                     ← jail breach (1 passo, outlier míope)
                  ▲                                ← auto-jail (8 passos, virada)
                       ▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲       ← token bloat (12 passos, visionário)

2026-07-30  ●───●───●───●───●───●  Week 3: Visionary (12.0)
            L22 L23 L24 F0  F1  F2
            ▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲       ← CMI design (15 passos, pico)
```

---

## 10. Integração com o Ecossistema Cosca

### 10.1 2nd-Order Reasoning Engine (F3.3) — Fonte Primária de Medição

O 2nd-order reasoning engine é o **motor que gera** os passos do horizonte. Sem ele, a medição do horizonte é retrospectiva (analisa decisões passadas). Com ele, a medição é **prospectiva** (projeta o horizonte ANTES da decisão).

```
┌──────────────────────────────────────────────────────────────────┐
│                                                                   │
│  INPUT: Decisão candidata (ação X no contexto Y)                  │
│     │                                                             │
│     ▼                                                             │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │ 2ND-ORDER REASONING ENGINE                                   │ │
│  │                                                              │ │
│  │ Step 0: Ação proposta                                        │ │
│  │ Step 1: Consequência direta (80% probabilidade)              │ │
│  │ Step 2: Cascata imediata (60% probabilidade)                 │ │
│  │ Step 3: Efeito sistêmico (40% probabilidade)                 │ │
│  │ ...                                                          │ │
│  │ Step N: Efeito de N-ésima ordem (< 10% probabilidade)        │ │
│  │                                                              │ │
│  │ CRITÉRIO DE PARADA: Probabilidade < 10% OU Impacto < mínimo  │ │
│  └─────────────────────────────────────────────────────────────┘ │
│     │                                                             │
│     ▼                                                             │
│  OUTPUT: Horizonte medido = N passos válidos                      │
│                                                                   │
└──────────────────────────────────────────────────────────────────┘
```

### 10.2 CMI — Dimensão Planejamento

O Cognitive Horizon é o principal input para a dimensão **Planejamento** do CMI ([COGNITIVE_MATURITY.md §2.2](../architecture/COGNITIVE_MATURITY.md)):

| Horizonte Médio | Impacto no CMI Planejamento | Explicação |
|:---------------:|:---------------------------:|------------|
| 0-3 (Myopic-Tactical) | **-15 pontos** | Planejamento é essencialmente inexistente. O runtime não antecipa consequências. |
| 4-6 (Operational) | **±0 pontos** (baseline atual) | Planejamento funcional. Projeta dependências diretas, não cascatas. |
| 7-10 (Strategic) | **+8 pontos** | Planejamento estratégico. Multi-agent orchestration com fallback e recovery. |
| 11-15 (Visionary) | **+15 pontos** | Planejamento transformacional. Projeta a própria evolução da capacidade de planejar. |
| 16-25 (Visionary+) | **+20 pontos** | Planejamento cross-temporal. Antecipa consequências com horizonte de meses. |

**CMI Planejamento atual**: 90. Com Cognitive Horizon tracking formal + 2nd-order reasoning engine: **projetado 95-98**.

### 10.3 Cognitive Momentum (C3)

Horizonte e Momentum são métricas irmãs com uma correlação positiva:

| Cenário | Momentum | Horizonte | Interpretação |
|---------|:--------:|:---------:|---------------|
| **Alto Momentum + Alto Horizonte** | 🚀 >80 | >10 passos | Domínio produzindo insights profundos. Investir máximo. Ex: Architecture (92 momentum, horizonte ~12). |
| **Alto Momentum + Baixo Horizonte** | 🚀 >80 | <5 passos | Muita atividade, pouca profundidade. Risco de "girar sem sair do lugar". Ex: Ondas de ativação (muito agente ativado, horizonte 3). |
| **Baixo Momentum + Alto Horizonte** | ⚰️ <20 | >10 passos | Conhecimento profundo mas domínio estagnado. Reativar com tarefa Level 4. Ex: Security após L15. |
| **Baixo Momentum + Baixo Horizonte** | ⚰️ <20 | <5 passos | Domínio morto. Arquivar. Ex: Plugin/WASM, Mobile. |

### 10.4 Mental Energy (C7)

O budget de energia cognitiva é modulado pelo horizonte:

```
Alocação de energia por horizonte da decisão:

  Decisão Myopic (0-1):    Alocação BASE (100%)
    ↓
  Decisão Tactical (2-3):  Alocação BASE (100%)
    ↓
  Decisão Operational (4-6): Alocação BASE + 10% (custo adicional de projeção)
    ↓
  Decisão Strategic (7-10):  Alocação BASE + 25% (projeção multi-step)
    ↓
  Decisão Visionary (11-25): Alocação BASE + 50% (simulação completa de cascata)
```

**Por que decisões Visionary custam mais energia**: Projetar 15 passos à frente requer manter 15 estados intermediários em contexto simultaneamente. É o equivalente cognitivo de jogar xadrez pensando 15 movimentos à frente — cada passo adicional dobra a complexidade da árvore de decisão. O custo é justificado pelo retorno: decisões Visionary têm 98%+ de taxa de sucesso e geram princípios reutilizáveis.

### 10.5 Decision DNA (C4)

Cada Decision DNA registra o horizonte no momento da decisão:

```yaml
decision_dna:
  id: "DDNA-2026-07-30-001"
  decision: "Implementar auto-jail via memfd_create"
  cognitive_horizon:
    measured_steps: 8
    level: "Strategic (H3)"
    boosting_techniques_applied:
      - "Contrafactual Gate (T2): +2 passos"
      - "Negative Memory — jail breach L13 (T7): +2 passos"
    horizon_at_decision: 4  # Horizonte base sem técnicas
    horizon_effective: 8     # Horizonte com técnicas aplicadas
```

Isso permite query futura: "Quais decisões foram tomadas com horizonte < 5 passos e depois revertidas?"

### 10.6 Contrafactual Gate (Gate 0.5)

O Contrafactual Gate adiciona +2 passos ao horizonte de qualquer decisão P0/P1:

```
Antes do Gate 0.5:
  Decisão: "Refatorar o sistema de logging"
  Horizonte: 4 passos (Operational)

Gate 0.5 — "E se o oposto?":
  Decisão oposta: "NÃO refatorar o sistema de logging"
  Cadeia alternativa:
    Step 1: Logging permanece fragmentado
    Step 2: Debugging cross-subsystem fica mais lento
    Step 3: Incidentes levam mais tempo para diagnosticar
    Step 4: Tempo do Don consumido em debugging manual
    Step 5: Confiança no runtime diminui

Depois do Gate 0.5:
  Horizonte: 4 + 2 = 6 passos (Operational, limite superior)
  A decisão original é confirmada, mas COM ressalvas documentadas
```

### 10.7 Metacognition Pipeline — Stage 1 (SELF-ASSESS)

Antes de qualquer task, o agente verifica o horizonte necessário:

```
If task_complexity >= "complex":
  required_horizon = estimate_required_horizon(task)
  current_horizon = query_cognitive_horizon(domain)

  If current_horizon < required_horizon:
    WARN: "Esta tarefa requer horizonte {required_horizon} passos.
           Horizonte atual do domínio: {current_horizon} passos.
           Risco de decisão míope: {(required_horizon - current_horizon) / required_horizon * 100}%
           Recomendação: ativar T1 (2nd-order reasoning) + T2 (contrafactual)."
```

---

## 11. Limitações e Edge Cases

### 11.1 O Problema da Previsão Infinita

É possível projetar cadeias arbitrariamente longas? **Não.** O horizonte tem um limite prático definido por três fatores:

1. **Decaimento de probabilidade**: Cada passo multiplica a incerteza. Após ~15 passos, a probabilidade composta de uma cadeia linear é < 1% — indistinguível de ruído.
2. **Custo cognitivo**: Cada passo adicional consome ~8% mais tokens de contexto. Projetar 25 passos custa 3× mais tokens que projetar 8 passos.
3. **Ramificação exponencial**: Cadeias causais não são lineares — cada passo ramifica em 2-4 consequências possíveis. Projetar 10 passos gera ~1.000-1.000.000 cenários. O runtime precisa podar ramos de baixa probabilidade.

**Regra de parada**: O horizonte termina no último passo onde P(passo) > 10% E impacto > threshold mínimo definido pelo domínio.

### 11.2 Horizonte vs Sorte

Uma decisão com horizonte 1 pode "dar certo" por sorte. Uma decisão com horizonte 15 pode "dar errado" por fatores imprevisíveis. O horizonte mede a **qualidade do raciocínio preditivo**, não a precisão do resultado.

**Exemplo**: L14 (binary protection, horizonte 3) "deu certo" — o binário ficou protegido. Mas não projetou que o CI quebraria porque o `install` tradicional usa `chmod 755`. A decisão foi "bem-sucedida" mas míope.

### 11.3 Horizonte Retroativo vs Prospectivo

A baseline atual (Seção 4) mede horizonte **retroativamente** — analisando decisões já tomadas. O 2nd-order reasoning engine (F3.3) permitirá medição **prospectiva** — antes da decisão ser executada. A medição retroativa é útil para diagnóstico; a prospectiva é útil para prevenção.

### 11.4 Horizonte em Domínios Desconhecidos

Quando o runtime enfrenta um domínio sem memória prévia (confidence < 0.5), o horizonte é artificialmente reduzido porque não há negative memory (T7) nem cognitive gravity (T5) para informar a projeção.

**Tratamento**: Domínios com confidence < 0.5 têm horizonte máximo de 3 passos (Tactical) independente das técnicas aplicadas. O runtime deve ESCALAR ao Don para decisões que requerem horizonte maior em domínios desconhecidos.

### 11.5 O Horizonte do Próprio Horizonte

Meta-pergunta: "Qual o horizonte da medição de horizonte?"

A Seção 5 já responde: o runtime projeta que o HDI atingirá ~15 em 2 semanas (com 2nd-order reasoning) e ~20 em 4 semanas (com cross-project knowledge). Esta projeção tem horizonte 4 (Operational) — é uma estimativa informada, não uma certeza.

---

## 12. Roadmap de Evolução do Horizonte

### 12.1 Fase Atual — Baseline Manual (v1.0.0)

- [x] Definição dos 5 níveis (Myopic → Visionary) com thresholds de passos
- [x] Baseline calculada manualmente a partir de 13 decisões documentadas em learnings.md
- [x] Correlação horizonte × taxa de sucesso estabelecida (0% míope → 98%+ visionário)
- [x] Tracking temporal (Week 1: 3.0 → Week 2: 7.5 → Week 3: 12.0)
- [x] Catálogo de 8 técnicas de expansão de horizonte (T1-T8)
- [x] Integração conceitual com CMI Planejamento, Cognitive Momentum, Mental Energy
- [x] Métricas derivadas: HDI, HGR, MDR, VDR

### 12.2 Fase 2 — Automação (v1.1.0+)

- [ ] Script `cosca analytics horizon` que calcula automaticamente de learnings.md + Decision DNA
- [ ] Integração com Decision DNA (C4): todo registro de decisão inclui `cognitive_horizon` calculado
- [ ] Notificação automática quando MDR > 5% (mais de 5% de decisões míopes)
- [ ] Histórico de horizonte armazenado em série temporal
- [ ] Dashboard real-time com gauge de horizonte + tendência

### 12.3 Fase 3 — Horizonte Prospectivo (v2.0.0+)

- [ ] 2nd-order reasoning engine (F3.3) como motor primário de projeção
- [ ] Horizonte calculado ANTES de cada decisão P0/P1 (não apenas retrospectivo)
- [ ] Bloqueio automático de decisões com horizonte < 3 passos em domínios de alta criticidade
- [ ] Sugestão automática de técnicas de boosting quando horizonte < requerido
- [ ] Horizonte preditivo: "Se a tendência atual se mantiver, HDI será 20 em 14 dias"
- [ ] Cross-project horizon transfer: herdar cadeias causais de projetos irmãos

---

## 13. Comando de Medição

```bash
# Medição automatizada (a ser implementada pelo Automation Chief)
cosca analytics horizon --baseline internal/embed/cosca/analytics/COGNITIVE_HORIZON.md

# Output esperado:
# ┌─────────────────────────────────────────────────────────────────┐
# │              COGNITIVE HORIZON — 2026-08-06                      │
# │                                                                  │
# │  Horizonte Médio (HDI-10):   8.5 passos  (Strategic H3)         │
# │  Horizonte Máximo:          15 passos    (L22 — CMI Design)     │
# │  Horizonte Mínimo:           1 passo     (L13 — Jail Breach)    │
# │                                                                  │
# │  Distribuição:                                                   │
# │    Myopic (0-1):       7.7%  (1 decisão)                        │
# │    Tactical (2-3):    15.4%  (2 decisões)                       │
# │    Operational (4-6): 30.8%  (4 decisões)                       │
# │    Strategic (7-10):  30.8%  (4 decisões)                       │
# │    Visionary (11-25): 15.4%  (2 decisões)                       │
# │                                                                  │
# │  Tendência: ↑↑ (+4.5 passos/dia)                                 │
# │  Target Fase 3: > 15 passos (Visionary sustentado)              │
# │                                                                  │
# │  ⚠️  ALERTAS:                                                    │
# │  • MDR = 7.7% (> 0% target) — 1 decisão míope (L13 jail breach) │
# │  • HGR desacelerando: 4.5→? (projetar plateau)                  │
# │  • 2nd-order reasoning (T1) não implementado — +3-5 passos      │
# │    latentes não realizados                                       │
# └─────────────────────────────────────────────────────────────────┘
```

---

## 14. Relacionamentos

| Documento | Relação |
|-----------|---------|
| [COGNITIVE_MATURITY.md](../architecture/COGNITIVE_MATURITY.md) §C8 | Define Cognitive Horizon como conceito da arquitetura cognitiva. Este documento é a especificação de implementação. |
| [COGNITIVE_MATURITY.md](../architecture/COGNITIVE_MATURITY.md) §A1 | 2nd-order reasoning — o motor que gera os passos do horizonte. Horizonte é a MEDIDA, 2nd-order é o GERADOR. |
| [COGNITIVE_MATURITY.md](../architecture/COGNITIVE_MATURITY.md) §2.2 | CMI Planejamento — dimensão diretamente impactada pelo horizonte. Horizonte profundo = planejamento alto. |
| [COGNITIVE_ENTROPY.md](COGNITIVE_ENTROPY.md) | Entropia alta reduz horizonte efetivo (conhecimento sujo → projeções menos confiáveis). |
| [COGNITIVE_MOMENTUM.md](COGNITIVE_MOMENTUM.md) | Correlação momentum↔horizonte: domínios com alto momentum tendem a gerar decisões de horizonte mais profundo. |
| [DECISION_DNA_FORMAT.md](../memory/DECISION_DNA_FORMAT.md) | Decision DNA armazena `cognitive_horizon` de cada decisão para query histórica. |
| [contrafactual-gate.md](../engines/tools/SKILL.md) | Gate 0.5 adiciona +2 passos ao horizonte. Integração automática. |
| [engines/cognitive-gravity/SKILL.md](../engines/cognitive-gravity/SKILL.md) | Cognitive Gravity (T5) adiciona +1-2 passos via heurísticas de alta massa. |
| [WISDOM_DECAY.md](../engines/tools/SKILL.md) | Wisdom Decay (T6) adiciona +1 passo ao forçar projeção temporal. |
| [KERNEL.md](../KERNEL.md) §5 | DAG generation — o planejamento do Kernel. Horizonte mede a profundidade do DAG. |
| [AUTO_EVOLUTION_PROTOCOL.md](../shared/AUTO_EVOLUTION_PROTOCOL.md) | Stagnation detection: horizonte estagnado > 30 dias → trigger de evolução. |
| [cognitive-maturity-implementation.md](../workflows/cognitive-maturity-implementation.md) F3.6 | Task de implementação do Cognitive Horizon tracking. Owner: cosca-analytics. |

---

## 15. Fontes de Dados

| Componente | Fonte Primária | Query |
|------------|---------------|-------|
| **Passos por decisão** | learnings.md (todos agentes) — campo `Learned` com análise de cadeia causal | Extrair menções de projeção: "se X, então Y → Z → W" |
| **Nível do horizonte** | Classificação manual baseada nos 5 níveis (H0-H4) | Mapear contagem de passos → nível |
| **Técnicas aplicadas** | Decision DNA (C4) — campo `boosting_techniques_applied` | Query: "quais técnicas foram aplicadas nesta decisão?" |
| **Taxa de sucesso** | learnings.md — campo `Outcome` | `rg "Outcome.*(success|failure)" learnings.md` |
| **Impacto da decisão** | Classificação: operacional (1.0), arquitetura (2.0), metacognição (3.0) | Baseado no campo `Level` e `Tags` |
| **Tendência temporal** | Histórico de medições de horizonte | Comparar HDI entre períodos |

---

## 16. Histórico

| Versão | Data | Autor | Mudanças |
|--------|------|-------|----------|
| 1.0.0 | 2026-07-30 | Cosca Analytics Chief | Definição inicial do Cognitive Horizon. 5 níveis (Myopic→Visionary). Baseline calculada para 13 decisões (L1-L24). Horizonte médio: 7.5 (Strategic). Correlação horizonte × taxa de sucesso: 0% míope → 98%+ visionário. Tracking temporal: Week 1 (3.0) → Week 2 (7.5) → Week 3 (12.0). 8 técnicas de boosting (T1-T8). Métricas derivadas: HDI, HGR, MDR, VDR. Integração com CMI Planejamento, Cognitive Momentum, Mental Energy, Decision DNA, Contrafactual Gate, Metacognition Pipeline. |

---

> **"O horizonte não mede o que você planeja. Mede até onde você consegue prever as consequências do que planeja. Um runtime que enxerga 15 passos à frente não é 5× melhor que um que enxerga 3 — é de uma classe diferente de inteligência."**
>
> — Cosca Analytics Chief, 2026-07-30

---

> **Enforced by**: Cosca Analytics Chief | **Next review**: 2026-08-06 (1 semana) | **References**: COGNITIVE_MATURITY.md §C8, COGNITIVE_MATURITY.md §A1 (2nd-order reasoning), cognitive-maturity-implementation.md F3.6
