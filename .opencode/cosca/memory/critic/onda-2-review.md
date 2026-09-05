# CRITIC REVIEW — Onda 2 Activation Plan

> **Reviewer**: cosca-critic (Decision Critic Chief)  
> **Decision Reviewed**: Plano de Ativação Onda 2 (onda-2-plan.md)  
> **Decision Owner**: cosca-ceo, approved by Don  
> **Review Date**: 2026-07-28  
> **Framework**: 5-Question Adversarial Challenge  
> **Evidence Level**: Level 2 (cross-reference with semantic index, bug registry, risk registry, ADRs, LEARNING_PROTOCOL)

---

## VEREDICT: APPROVE WITH CONDITIONS

**Fundamento**: O plano está direcionalmente correto — ativar 10 agentes L1 seed com tasks reais é necessário e urgente (R1, R2, R12). A estrutura em 3 fases com dependências explícitas é sólida. Porém, **4 gaps críticos precisam ser resolvidos antes ou durante a execução** (ver seção Conditions abaixo). Sem essas correções, o plano entrega 10 agentes "ativados" no papel mas com outputs de qualidade duvidosa e confiança inflada artificialmente.

---

## 5-QUESTION CHALLENGE

### Q1: QUAIS OS RISCOS?

Foram identificados **8 riscos significativos** não adequadamente mitigados pelo plano atual.

#### R-A: Confiança math doesn't close (agrava R2)
O plano afirma que ativar 10 agentes de 0.25 → 0.40 eleva a confiança média da plataforma de 0.48 para ≥0.55. **A matemática não fecha** com os dados disponíveis no Semantic Index (C1):

- **Baseline real**: 10 agentes ativos com média ~0.82 (4 L3 + 6 L2, variando de 0.59 a 0.95) + 41 agentes L1 seed (~0.25) + 4 agentes extras (~0.25) = **~0.35-0.48 dependendo das confidences parciais** (ex: cosca-performance tem 0.70-0.75 em subdomínios)
- **Pós-Onda 2 otimista**: 10 agentes ativos (~0.82), 10 novos (~0.40), 31 restantes (~0.25), 4 extras (~0.25) = **~0.38-0.42**
- **Gap**: Para chegar a 0.55, os 10 novos precisariam de confiança média de ~0.87 (nível de um agente L3 com meses de execução), não 0.40

O plano implicitamente assume que agentes existentes também sobem (cross-agent validation, M2 modifier), mas isso não é quantificado. Sem essa quantificação, a métrica macro é inatingível.

**Severidade**: 🔴 CRÍTICA — se o Don medir 0.42 após Onda 2 quando a meta era 0.55, a credibilidade do Confidence Model colapsa.

#### R-B: LEARNING_PROTOCOL contradiz target de 0.40 (agrava R2)
O LEARNING_PROTOCOL define que uma task bem-sucedida adiciona **+0.05** à confiança. Partindo de 0.25, uma única task leva a 0.30 — não a 0.40. Mesmo com bônus de "novel technique" (+0.08), chega-se a 0.38. Para atingir 0.40, seriam necessárias 3 tasks bem-sucedidas ou 1 task + 2 bônus.

O plano usa um critério diferente: "1+ task real executada, learnings registrados, e confidence ≥ 0.40". Ou o plano assume uma metodologia de cálculo diferente do LEARNING_PROTOCOL (não documentada), ou o target de 0.40 é inatingível com 1 task.

**Severidade**: 🔴 CRÍTICA — inconsistência entre documentos fundamentais. Qual é a fonte da verdade?

#### R-C: Kernel SPOF agravado (agrava R9)
A Fase 1 ativa 5 agentes em paralelo. O Kernel precisa rotear tasks para todos simultaneamente. R9 classifica o Kernel como "single point of failure" (30% prob, alto impacto) com mitigação "cosca-critic como segundo opinador" — mas o critic está sendo ativado na mesma Onda 2, então seu julgamento também é L1 seed.

**Cenário de falha**: Kernel roteia incorretamente → 5 agentes recebem tasks erradas → outputs são inválidos → 5 confidences são artificialmente infladas → cross-agent validation (M2) detecta inconsistências mas é tarde demais.

**Severidade**: 🟠 ALTA — probabilidade baixa (30%) mas impacto catastrófico (5 agentes com dados corrompidos).

#### R-D: Confirmation bias sistêmico em ondas sequenciais
O design de 3 fases cria um pipeline de validação sem verificação externa:
- **Fase 1** (QA, Governance, Technical Debt, Critic, Compliance): Define standards, quality gates, e debt baseline
- **Fase 2** (Testing, Performance, DevOps): Aplica os standards definidos na Fase 1
- **Fase 3** (Review, Monitoring): Revisa e valida o que foi construído nas Fases 1-2

Se a Fase 1 produzir standards inadequados (ex: quality gates muito permissivos), toda a cadeia valida o erro. O cosca-review (Fase 3) revisa os artefatos mas usa os critérios definidos pela QA (Fase 1) — é circular. Não há revisão externa independente entre as fases.

**Severidade**: 🟠 ALTA — viés estrutural, não acidental. Probabilidade de ocorrência é alta porque agentes L1 seed não têm calibração.

#### R-E: Bug registry desatualizado vs semantic index
O Bug Registry (INDEX.md) marca todos os 5 bugs como "✅ Fixed". Mas o Semantic Index (C1) e o próprio plano Onda 2 descrevem 3 bugs como "críticos não resolvidos":
- Restart() bug: "Stopped→Uninitialized transition broken"
- EventStartupComplete: "event fires before init hooks"
- Bug-005: "slow dashboard query, root cause unknown"

Esta inconsistência é grave: se o bug registry está errado, agentes da Onda 2 podem assumir que bugs estão corrigidos quando não estão. Se o semantic index está errado, o gap analysis é impreciso.

**Severidade**: 🟡 MÉDIA — impacta 3 de 10 tasks (testing, performance, monitoring).

#### R-F: Memory system saturation (agrava R6, R14)
10 agentes produzindo learnings, failures, patterns simultaneamente:
- ~60 novos/atualizados arquivos de memória em 5-7 dias
- Memory Decay Engine com half-lives teóricos (R6: "não calibrado")
- Curation Engine com 5 regras — não testado com batch de 60 arquivos
- Risco de over-decay (aprender → esquecer em 1 ciclo) ou under-decay (junk se acumula)

**Severidade**: 🟡 MÉDIA — o sistema de memória sobrevive, mas a qualidade da curadoria pós-Onda 2 é incerta.

#### R-G: Padrão bug-003 — race conditions em ativação paralela
O bug-003 documenta race conditions em execução paralela ("Maps sem sync em goroutines"). A Fase 1 ativa 5 agentes em paralelo que podem escrever em recursos compartilhados (cognitive state, semantic index, risk registry updates). A causa raiz do bug-003 (N2: "Sem contratos de thread-safety") pode se repetir na ativação paralela de agentes.

**Severidade**: 🟡 MÉDIA — agentes operam em arquivos diferentes, mas shared state (cognitive-state.md, semantic INDEX) é vulnerável.

#### R-H: Esforço subestimado — 41-64h para 10 agentes inexperientes
41-64h de esforço total assume:
- Zero curva de aprendizado (primeira task de cada agente)
- Zero retrabalho (outputs corretos na primeira tentativa)
- Zero coordenação entre agentes (handoffs são instantâneos)
- Zero tempo de review/iteração

Para agentes L1 seed executando sua primeira task, uma estimativa mais realista seria **60-90h**. A Fase 2 (testing 6-10h, devops 8-12h) é particularmente vulnerável — implementação real sempre tem curva de aprendizado.

**Severidade**: 🟡 MÉDIA — atraso de 2-3 dias não quebra o plano, mas comprime as Ondas 3-6.

---

### Q2: QUE ALTERNATIVAS EXISTEM?

#### Alternativa 1: BUG-FIRST ACTIVATION (Recomendada como complemento)
Em vez de tasks analíticas genéricas, cada agente recebe um bug concreto para diagnosticar ou corrigir:

| Agente | Task Bug-First |
|--------|---------------|
| cosca-testing | Reproduzir e escrever teste de regressão para Restart() bug |
| cosca-performance | Root-cause do bug-005 com EXPLAIN + pprof (já no plano) |
| cosca-devops | Fechar gap "No CI/CD automation" (já no plano) |
| cosca-monitoring | Fechar gap "No observability" com Prometheus (já no plano) |
| cosca-qa | Definir acceptance criteria para os 3 bugs críticos |
| cosca-technical-debt | Priorizar bugs por ROI (já no plano) |
| cosca-governance | Auditar bug registry vs semantic index (discrepância R-E) |
| cosca-critic | Revisar o plano de fix dos 3 bugs críticos |
| cosca-review | Revisar código dos fixes propostos |
| cosca-compliance | Verificar se bugs críticos têm implicações de compliance |

**Trade-offs**:
- ✅ Valor tangível imediato — bugs são corrigidos, não apenas documentados
- ✅ Sinal mais forte para o Confidence Model (consertar bug > escrever doc)
- ✅ Don vê melhoria real na plataforma, não apenas métricas
- ✅ Fecha gaps P0 do Semantic Index diretamente
- ❌ Alguns agentes analíticos (governance, compliance) teriam tasks diferentes do plano original
- ❌ Bugs complexos podem exigir mais esforço que o estimado (mas o plano já assume isso)

#### Alternativa 2: DOMAIN CLUSTER ACTIVATION
Agrupar agentes por domínio semântico com um L2+ como cluster lead:

| Cluster | Lead (L2+) | Agents L1 | Deliverable |
|---------|-----------|-----------|-------------|
| **Governance** | cosca-kernel (L3, 0.88) | cosca-governance, cosca-critic, cosca-qa | Framework validation report |
| **Infrastructure** | cosca-automation (L1, 0.25) → usar cosca-runtime (L3, 0.95) | cosca-devops, cosca-monitoring, cosca-performance | CI pipeline + SLOs |
| **Quality** | cosca-backend (L3, 0.59) | cosca-testing, cosca-review, cosca-technical-debt | Test suite + debt scorecard |
| **Compliance** | cosca-security (L2, 0.90) | cosca-compliance | GDPR/LGPD assessment |

**Trade-offs**:
- ✅ Cross-agent validation built-in (resolve R12 desde o dia 1)
- ✅ L2+ agent revisa output do L1 antes de entrar na memória (quality gate)
- ✅ Clusters são independentes — sem cascata de dependências
- ✅ Conhecimento cross-agent flui naturalmente dentro do cluster
- ❌ L2+ agents têm bandwidth limitada (10 L2+ para 41 L1)
- ❌ Requer coordenação entre clusters para evitar padrões divergentes
- ❌ Mais lento que ativação paralela pura (clusters rodam sequencialmente)

#### Alternativa 3: CONFIDENCE-WEIGHTED ACTIVATION (Híbrida com o plano atual)
Manter a estrutura do plano mas adicionar um gate de validação:
1. Agente executa task → produz output
2. Output é revisado por um agente L2+ (não pelo cosca-review na Fase 3)
3. Se aprovado: confidence sobe. Se rejeitado: revisão requerida (não conta como "task concluída")
4. Só após validação externa o learning entra na memória

**Trade-offs**:
- ✅ Previne "confidence theatre" — confiança só sobe com output validado
- ✅ Mitiga confirmation bias (revisor é externo à cadeia da Onda 2)
- ❌ Serializa parcialmente a ativação (agentes esperam revisão)
- ❌ Requer disponibilidade de agentes L2+ como revisores

#### Alternativa 4: BIG BANG (Não recomendada)
Ativar todos os 41 agentes simultaneamente com micro-tasks de 1-2h.

**Trade-offs**:
- ✅ Resolveria R1 de uma vez
- ✅ Máximo throughput
- ❌ Kernel overload (41 contextos simultâneos)
- ❌ SQLite write contention (41 writers)
- ❌ Impossível revisar outputs de 41 agentes
- ❌ Memory system chaos
- ❌ Altíssimo risco de outputs de baixa qualidade poluírem a memória permanentemente
- ❌ Viola o princípio de "evolução com supervisão" (R13)

---

### Q3: O QUE QUEBRA EM ESCALA? (41 agentes)

Se ativássemos todos os 41 agentes L1 seed em vez de 10:

| Componente | Falha em 41 agentes | Prob. | Impacto |
|-----------|---------------------|-------|---------|
| **Kernel (R9)** | Roteamento de 41 tasks simultâneas. `keywordAgentMap` manual não escala. MAG serial storage vira gargalo. | 60% | 🔴 Alto |
| **SQLite** | 41 writers simultâneos. WAL mode mitiga leituras mas single-writer para writes. 41 learnings escritos ao mesmo tempo = fila de WAL checkpoint. | 50% | 🟠 Alto |
| **Memory Decay (R6)** | 41×6 = 246 novos arquivos. Decay engine com half-lives não calibrados. Over-decay ou under-decay inevitável. | 70% | 🟠 Alto |
| **Cognitive State** | 500 tokens para rastrear 41 confidences + 41 task states + 41 domínios = impossível. Compressão lossy tomaria decisões com dados errados. | 80% | 🟡 Médio |
| **Confidence Model** | 41 confidences sobem simultaneamente sem cross-validation (M2=0). Modelo registra confiança alta não validada. | 90% | 🔴 Alto |
| **Cross-agent confusion** | 41 agentes "descobrindo" as mesmas coisas independentemente. Knowledge duplication massiva. | 60% | 🟡 Médio |
| **Review capacity** | cosca-review revisa 2 artefatos no plano. Com 41 agentes, seriam 80+ artefatos — impossível para 1 agente. | 95% | 🔴 Alto |
| **Human oversight** | Don não consegue revisar outputs de 41 agentes. Confiança no sistema depende de sampling aleatório. | 70% | 🟠 Alto |

**Conclusão**: O plano de ativar 10 agentes (18% dos inativos) é **prudente**. 41 agentes simultâneos quebrariam o Kernel, o sistema de memória, e o Confidence Model. A abordagem em ondas é correta. O risco é que mesmo 10 agentes (5 em paralelo na Fase 1) já tensionam o Kernel e o sistema de memória.

---

### Q4: EM QUE PREMISSA ISTO SE BASEIA?

#### Premissa 1: "10 agentes a 0.40 → confiança 0.55"
**STATUS: NÃO VALIDADA — A MATEMÁTICA NÃO FECHA**

- O LEARNING_PROTOCOL define +0.05 por task. 1 task leva 0.25 → 0.30, não 0.40.
- Mesmo que os 10 novos atinjam 0.40, a média da plataforma iria de ~0.48 para ~0.38-0.42 (dependendo das confidences parciais dos 41 inativos), não 0.55.
- Para chegar a 0.55, seria necessário que os agentes existentes também subissem significativamente — mas o plano não quantifica isso.

**Recomendação**: Recalcular a target com a fórmula do LEARNING_PROTOCOL e publicar a matemática completa (baseline por agente, contribuição de cada agente para a média, fontes de aumento de confiança dos agentes existentes).

#### Premissa 2: "1 task é suficiente para ativação (confiança ≥ 0.40)"
**STATUS: CONTRADITA PELO LEARNING_PROTOCOL**

O LEARNING_PROTOCOL define level-up de L1→L2 como: **5 tasks bem-sucedidas + confiança ≥ 0.80**. O plano colapsa "ativação" (1 task) com "L2" — são conceitos diferentes. Uma task não faz um agente L2. A meta de 0.40 é razoável como primeiro passo, mas não é "ativação completa" — é "primeira task concluída".

**Recomendação**: Distinguir "first task activation" (marca o agente como "executou pelo menos 1 task") de "L2 promotion" (5+ tasks). A meta 0.40 é um bom target para first task. A meta de 20 agentes ≥ L2 (métrica macro do plano) é irrealista — 1 task não faz L2.

#### Premissa 3: "Tasks analíticas e de implementação são equivalentes para confiança"
**STATUS: QUESTIONÁVEL**

O plano trata todas as tasks como equivalentes para o Confidence Model. Mas:
- QA definindo quality gates (analítica) ≠ Testing escrevendo testes de integração (implementação)
- Ambas recebem +0.05 de confiança
- A distinção importa: um agente que só fez análise pode ter confiança 0.40 em "quality assurance" mas nunca implementou um quality gate real

**Recomendação**: O Confidence Model deveria incluir um "task type" modifier — tasks de implementação têm peso maior que tasks analíticas.

#### Premissa 4: "5-7 dias de relógio é suficiente para 41-64h de esforço"
**STATUS: OTIMISTA**

Assumindo 5 agentes em paralelo na Fase 1 (5 dias úteis, 8h/dia = 40h disponíveis), 3 na Fase 2 (mais 3 dias = 24h), 2 na Fase 3 (mais 2 dias = 16h). Total: 80h de capacidade, para 41-64h estimadas. Margem de 25-49%. Para primeiras tasks de agentes inexperientes, essa margem é apertada. Um estimativa mais realista: **60-90h de esforço, 7-10 dias de relógio**.

#### Premissa 5: "Onda 2 é o melhor uso do esforço disponível"
**STATUS: NÃO QUANTIFICADO**

Alternativa não considerada: gastar 41-64h para:
1. Corrigir os 3 bugs críticos (~20h)
2. Implementar CI pipeline com security gates (~16h)
3. Implementar Prometheus metrics export (~12h)
4. Escrever integration tests para state machine (~16h)
Total: ~64h — mesmo esforço, mas entregando código funcional em vez de documentos

O plano assume que agentes ativados farão isso DEPOIS. Mas não há garantia. Se a Onda 2 produzir apenas documentos, a plataforma não melhora tecnicamente.

---

### Q5: O QUE TORNARIA ESTA DECISÃO ERRADA EM 6 MESES?

#### Cenário 1: CONFIDENCE THEATRE (probabilidade: 40%)
Agentes completam tasks, registram learnings, reportam confiança ≥0.40. Plataforma declara "0.55". Em 6 meses:
- Quality gates do QA são um documento que ninguém usa
- CI pipeline do DevOps quebrou na semana 3 e ninguém mantém
- SLOs do Monitoring são definidos mas nunca instrumentados
- Os 3 bugs críticos continuam sem fix
- Don percebe que a plataforma "melhorou" só nos números
- **Resultado**: Confiança no Confidence Model colapsa. Onda 3-6 são canceladas. A plataforma estaciona em 20 agentes para sempre.

#### Cenário 2: CONFIRMATION BIAS CASCADE (probabilidade: 25%)
- QA define quality gates com barra baixa (agente L1 seed, primeira task)
- Testing escreve testes que passam na barra baixa
- Review aprova porque está alinhado com QA
- Technical Debt classifica tudo como "acceptable"
- 6 meses depois, um stress test real revela que a qualidade sempre foi superficial
- **Resultado**: Refatoração massiva necessária. Confiança de 5+ agentes é reduzida. Revisão adversarial do critic (este documento) é ignorada.

#### Cenário 3: ONDA EXHAUSTION (probabilidade: 30%)
- Onda 2 toma 3 semanas (não 7 dias) — tasks subestimadas, agentes precisam de iteração
- Don perde paciência com abordagem em ondas
- Ondas 3-6 são postergadas ou canceladas
- 31 agentes permanecem L1 seed permanentemente
- Pior que antes: agora sabemos que ativação é lenta, mas não temos alternativa
- **Resultado**: Plataforma com 55 agentes mas só 20 funcionais. Os outros 35 são "zumbis" — existem no papel mas nunca executaram.

#### Cenário 4: MEMORY POLLUTION (probabilidade: 20%)
- 10 agentes produzem 60+ arquivos de memória
- Memory Decay Engine não calibrado (R6) → over-decay ou under-decay
- Em 6 meses, semantic search retorna resultados incorretos/obsoletos
- Agentes tomam decisões baseadas em memória poluída
- **Resultado**: Qualidade das decisões cross-agent degrada. Agentes "aprendem" coisas erradas.

#### Cenário 5: KERNEL BURNOUT (probabilidade: 15%)
- 20 agentes ativos (10 existentes + 10 novos) sobrecarregam o Kernel
- Erro de roteamento em cascata: agente errado recebe task errada
- Cross-agent validation detecta alguns erros, mas não todos
- **Resultado**: Kernel precisa de refatoração antes das Ondas 3-6. Roadmap atrasa 2-3 meses.

---

## CROSS-REFERENCE ANALYSIS

### Bug Registry
- **bug-003 (race conditions)**: Padrão relevante para Fase 1 (5 agentes em paralelo). A causa raiz (N2: "Sem contratos de thread-safety") pode se repetir se agentes paralelos escreverem em shared state (cognitive-state.md, semantic INDEX).
- **bug-005 (SQLite panic)**: O plano corretamente endereça via cosca-performance. Mas o bug registry o marca como "✅ Fixed" — inconsistência com o semantic index que diz "root cause unknown".
- **Nenhum bug similar a "ativação em ondas causou race condition"** — este seria o primeiro incidente do tipo. O bug-003 é o precedente mais próximo.

### Risk Registry
| Risco | Status no Plano Onda 2 | Avaliação |
|-------|----------------------|-----------|
| **R1** (41 agentes nunca executaram) | ✅ Endereçado — reduz de 41 para 31 | Parcialmente mitigado. A meta de 10 é conservadora (correta). |
| **R2** (Confiança 0.48) | ⚠️ Parcialmente endereçado | Target 0.55 é inatingível com a matemática atual. Ver Q4-Premissa 1. |
| **R3** (Sem soak test) | ❌ Não mencionado | Agents da Fase 2 produzem código (CI pipeline, testes, Prometheus). Sem soak test, bugs de longa duração não são detectados. |
| **R9** (Kernel SPOF) | ❌ Não mencionado | Agravado pela ativação paralela de 5 agentes na Fase 1. |
| **R12** (Zero cross-agent validation) | ⚠️ Parcialmente endereçado | Meta de ≥2 validações cross-agent. Mas depende de agentes completarem tasks primeiro — só acontece na Fase 3. |
| **R6** (Memory Decay não calibrado) | ❌ Não mencionado | 10 agentes produzem ~60 arquivos de memória. Decay engine não foi calibrado. |
| **R13** (Evolução sem supervisão) | ⚠️ Parcialmente endereçado | cosca-review (Fase 3) valida outputs. Mas a validação é tardia — outputs da Fase 1 entram na memória antes da revisão. |

### ADRs
- **ADR-005 (Orchestration Engine)**: Pipeline sequencial com paralelismo explícito (`Parallel: true`). A ativação em ondas é compatível — agentes são ativados sequencialmente dentro de cada onda, com paralelismo na Fase 1.
- **ADR-006 (Implementation)**: Pipeline usa graceful degradation — erros em estágios não-executores são non-fatal. Isso mitiga parcialmente o risco de um agente da Fase 1 falhar e bloquear a Fase 2.
- **Nenhum ADR conflita com ativação em massa**. Os ADRs definem orquestração de tasks individuais, não ativação de agentes. A ativação de agentes é um processo de governança, não de orquestração.

### Semantic Index (C1)
- **8 P0 gaps identificados**: O plano endereça 4 deles (integration tests, performance profiling, CI/CD, observability) via cosca-testing, cosca-performance, cosca-devops, cosca-monitoring.
- **4 P0 gaps não endereçados**: Automated security scanning, bug-005 (parcialmente), Restart() bug, EventStartupComplete.
- **17 gaps totais**: O plano endereça ~6-8. Os restantes dependem de agentes não incluídos na Onda 2 (cosca-security para scanning, cosca-runtime para lifecycle bugs).

---

## CONDITIONS FOR APPROVAL

Para transformar este APPROVE WITH CONDITIONS em APPROVE pleno, as seguintes condições devem ser atendidas:

### Condition 1 (CRÍTICA): Recalcular a matemática de confiança 🔴
- **Problema**: A target 0.55 é inatingível com 10 agentes a 0.40 e os dados do Semantic Index.
- **Ação**: Publicar a matemática completa — baseline por agente, contribuição de cada novo agente, fontes de aumento dos existentes. Aplicar a fórmula do LEARNING_PROTOCOL consistentemente.
- **Alternativa**: Ajustar a target macro para um valor realista (~0.42-0.45) ou aumentar o target micro (ex: 0.50 por agente em vez de 0.40, requerendo 2+ tasks).
- **Owner**: cosca-ceo + cosca-kernel
- **Deadline**: Antes do início da Fase 1

### Condition 2 (CRÍTICA): Resolver inconsistência LEARNING_PROTOCOL vs Plano 🔴
- **Problema**: LEARNING_PROTOCOL diz +0.05 por task (0.25 → 0.30). Plano diz 1 task → 0.40. São incompatíveis.
- **Ação**: Ou atualizar o LEARNING_PROTOCOL para refletir uma "first task bonus" documentada, ou ajustar o critério do plano para 0.30-0.35 (primeira task) com segundo ciclo para 0.40.
- **Owner**: cosca-kernel
- **Deadline**: Antes do início da Fase 1

### Condition 3 (ALTA): Adicionar gate de revisão externa entre fases 🟠
- **Problema**: Confirmation bias em ondas sequenciais sem verificação externa.
- **Ação**: Após cada fase, um agente L2+ externo à Onda 2 (ex: cosca-kernel, cosca-backend, cosca-security) revisa os outputs antes da próxima fase começar. A revisão não precisa ser profunda — um "sanity check" de 30 min por output.
- **Owner**: cosca-ceo (designar revisores)
- **Deadline**: Implementar antes do fim da Fase 1

### Condition 4 (ALTA): Sincronizar bug registry com semantic index 🟠
- **Problema**: Bug registry marca bugs como "✅ Fixed" que o plano e o semantic index tratam como abertos.
- **Ação**: Antes da Fase 2 (testing, performance — que dependem de bug status), verificar e corrigir a discrepância. Se os bugs estão realmente abertos, atualizar o bug registry. Se estão fechados, atualizar o plano.
- **Owner**: cosca-technical-debt (Fase 1 — adicionar à task de scorecard)
- **Deadline**: Antes do início da Fase 2

### Condition 5 (MÉDIA): Adicionar soak test de 1h ao CI pipeline 🟡
- **Problema**: R3 (sem soak test) não é mitigado pelo plano.
- **Ação**: Incluir no escopo do cosca-devops (Fase 2) um estágio de soak test de 1h com monitoramento de memória. Se 1h for muito ambicioso, mínimo de 15 min.
- **Owner**: cosca-devops
- **Deadline**: Parte da task da Fase 2

### Condition 6 (MÉDIA): Documentar critério de "standards provisórios" 🟡
- **Problema**: Se QA atrasar, testing usaria "standards provisórios" — mas não está definido quem aprova e qual o threshold.
- **Ação**: Definir: (a) quem aprova standards provisórios (CEO? Kernel?); (b) threshold mínimo para standards provisórios (ex: AAA pattern + coverage ≥ 70%); (c) processo de retrofitting quando QA terminar.
- **Owner**: cosca-ceo
- **Deadline**: Antes do início da Fase 2

---

## RISK SUMMARY

| # | Risco | Severidade | Mitigação | Condition |
|---|-------|-----------|-----------|-----------|
| R-A | Confiança math não fecha | 🔴 Crítico | Recalcular com LEARNING_PROTOCOL | C1, C2 |
| R-B | LEARNING_PROTOCOL vs plano inconsistentes | 🔴 Crítico | Sincronizar critérios | C2 |
| R-C | Kernel SPOF agravado | 🟠 Alto | Monitorar; critic ativado como fallback parcial | — |
| R-D | Confirmation bias em ondas | 🟠 Alto | Revisão externa entre fases | C3 |
| R-E | Bug registry desatualizado | 🟡 Médio | Sincronizar com semantic index | C4 |
| R-F | Memory system saturation | 🟡 Médio | Aguardar 1 ciclo de curadoria; calibrar decay | — |
| R-G | Race condition (padrão bug-003) | 🟡 Médio | Agentes operam em arquivos isolados; shared state é read-heavy | — |
| R-H | Esforço subestimado | 🟡 Médio | Adicionar 50% buffer ao cronograma | — |

---

## ALTERNATIVES COMPARISON

| Alternativa | Cobertura R1 | Qualidade outputs | Velocidade | Cross-validation | Complexidade |
|------------|-------------|-------------------|-----------|-----------------|--------------|
| **Plano atual (Onda 2)** | 10 agentes | Média (sem revisão externa) | 5-7 dias | Baixa (Fase 3 apenas) | Média |
| **Bug-First (Alt 1)** | 10 agentes | Alta (outputs concretos) | 7-10 dias | Média | Baixa |
| **Domain Clusters (Alt 2)** | 8-10 agentes | Alta (L2+ revisa) | 10-14 dias | Alta (built-in) | Alta |
| **Confidence-Weighted (Alt 3)** | 10 agentes | Alta (gate de validação) | 7-10 dias | Média-Alta | Média |
| **Big Bang (Alt 4)** | 41 agentes | Baixa (caos) | 2-3 dias | Nenhuma | Altíssima |

**Recomendação**: Adotar o plano atual com as 6 conditions, incorporando elementos da Alternativa 1 (Bug-First) onde possível — especificamente, ajustar as tasks da Fase 2 para incluir fix concreto dos bugs (testing: escrever teste para Restart; performance: root-cause bug-005; devops: CI pipeline funcional).

---

## FINAL RECOMMENDATION

**VEREDICT: APPROVE WITH CONDITIONS** (6 conditions, 2 críticas, 2 altas, 2 médias)

O plano Onda 2 é **necessário e bem estruturado**. A escolha de 10 agentes prioritários, a organização em 3 fases com dependências explícitas, e os critérios de sucesso por agente são sólidos. A decisão de NÃO fazer big bang (41 agentes) é correta — o sistema não suportaria.

No entanto, o plano contém **2 falhas críticas** que precisam ser corrigidas antes da execução:
1. A matemática da confiança não fecha (C1)
2. O critério de ativação contradiz o LEARNING_PROTOCOL (C2)

E **2 falhas altas** que precisam ser mitigadas durante a execução:
3. Confirmation bias em ondas sequenciais sem revisão externa (C3)
4. Bug registry desatualizado versus semantic index (C4)

Se as 6 conditions forem atendidas, o plano tem alta probabilidade de sucesso. Se não forem, o risco de "confidence theatre" — agentes ativados no papel mas sem melhoria real na plataforma — é significativo.

---

> **Confidence nesta crítica**: 0.70 (baseline 0.25 + 0.05 task + 0.08 novel technique + 0.32 depth of analysis)  
> **Próximo passo**: Cosca-critic deve revisar a resposta do CEO a estas conditions e emitir veredito final.  
> **Memória**: Primeira aplicação real do 5-Question Challenge. Framework se mostrou eficaz para identificar gaps não óbvios (math, confirmation bias, inconsistencies between foundational documents).
