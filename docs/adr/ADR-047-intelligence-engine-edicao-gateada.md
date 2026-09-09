# ADR-047: Intelligence Engine & Governança de Edição Gateada

> **Status:** PROPOSTO — Aguardando Gate do Don | **Owner:** cosca-kernel | **Last Updated:** 2026-09-09
> **Natureza:** ADR de decisão. Cria o **Intelligence Engine** — o motor que governa o conhecimento da casa (o que sabe, o que substitui, o que aprende em seguida) — vinculado a um **Contrato de Salvaguarda** permanente que impede o sistema de entrar em loop de erro.
> **Abordagem:** Motor + Freio decididos JUNTOS e inseparáveis. O próprio ADR-046 previu ADR-047 como "Intelligence Engine (usa cosca-intelligence.db)" — aqui é inaugurado em sua forma definitiva.
> **Relação:** complementa **ADR-046** (DSMS — já criou o schema 004_intelligence.sql), **ADR-044** (Learning Vaults), **ADR-033** (Shadow Mode), **ADR-037/038** (Evidence Layer / Epistemic verdict) e **ADR-014** (auto-recovery).

---

## 1. Contexto

### 1.1 O que a casa já tem

O Cosca possui um **corpus semântico curado de valor único** (4521 docs · 113k chunks · **122k entidades** · grafo de conhecimento · 6 camadas de memória · skills/agentes editáveis). O conhecimento é **vivo, recuperável e auditável** — a "caixa-preta inversa".

### 1.2 O que existe em SPEC (documentado) mas NÃO roda em código

Os motores de curadoria da memória estão **escritos em `.md`** mas **não implementados** em Go:

- **MEMORY_CURATION_ENGINE** — CurationScore v1.0 + R1-R5 (avaliar, condensar, podar, promover, fallback de falhas)
- **MEMORY_DECAY_ENGINE** — CurationScore v2.0 + DecayFactor + **R6 conflito** + `supersedes` + `pinned`

O único pedaço que roda é o passo `wisdom_decay` do `restcycle.go` (depreciação por frescor). **Não há motor de conflito, substituição, promoção ou curriculum.**

### 1.3 O risco que este ADR decide neutralizar

Editar conhecimento ao vivo **sem freio** destrói uma inteligência. Os loops de erro reais:

1. **Loop de auto-correção**: A contradiz B, B contradiz C, C contradiz A → gira sem parar.
2. **Regressão**: uma "edição" errada apaga conhecimento bom e validado.
3. **Corrida de conflitos**: duas partes "atualizam" o mesmo fato sem dono → contradição interna.
4. **Amnésia**: decay sem freio "esquece" até padrão de segurança / decisão crítica.

> **Lei da casa:** *Memória > Velocidade.* A integridade é mais alta que a conveniência.

---

## 2. Decisão

Criar o **Intelligence Engine**, um motor que governa o conhecimento da casa, **inseparável** do seu **Contrato de Salvaguarda**.

> **Afirmação central:** o engine **não edita sozinho**. Ele **sabe** o que está obsoleto/em conflito, **propõe**, passa pelo **Freio**, e **só** aplica o que sobrevive à validação. O que é imutável, **nem propõe**.

### 2.1 As 5 funções do motor (D1-D6)

| # | Função | Automático? | Descrição |
|---|---|---|---|
| ① | **Curriculum** | ✅ sim (observa) | Decide **o que e em que ordem** estudar, guiado pelo grafo de 122k entidades (aprendizagem autodirigida). |
| ② | **Detecção de conflito** (R6) | ✅ sim (observa) | Detecta quando conhecimento novo contradiz o antigo. **Só sinaliza** — não resolve. |
| ③ | **Substituição** (`supersedes`) | ⛔ gateada | Propõe a substituição; **aplica só com teu gate**. |
| ④ | **Promoção** (R4 → global) | ⛔ gateada | Propõe elevar conhecimento validado a padrão global. |
| ⑤ | **Validação determinística** | ✅ sim | Toda edição só passa se comprovada por **evidência** (nível 4-5), nunca por opinião de LLM. |

**Onde vive:** `cosca-intelligence.db` — schema 004 já criado pelo DSMS (`rules`, `code_patterns`, `expert_systems`, `decision_trees`, `code_metrics`).
**O que consome:** `graph` + `vector` + `knowledge.db` (o corpus semântico).
**Papéis:** Kernel/agentes **propõem** · **Don aprova** · especialistas **executam**. Ninguém edita o que não valida.

---

## 3. Contrato de Salvaguarda (o Freio — G1 a G9)

Este é o **contrato permanente** que impede o loop de erro. É **irrenunciável**, prevalece sobre qualquer conveniência, e é o que o Don exigiu manter.

| # | Regra | Descrição |
|---|---|---|
| **G1** | **Imutável primeiro (pinned)** | Constituição, `laws.json`, ADRs, padrões de segurança, decisões críticas → **NUNCA** editados automaticamente. Só manual, com ordem explícita do Don. |
| **G2** | **Gate do Don** | Toda edição de conhecimento = **proposta** que sobe à mesa do Don. Sem aprovação, **não mexe**. |
| **G3** | **Shadow-first obrigatório** | O engine roda **N ciclos em modo observação** (mostra o que faria, não aplica). Só vira automático após provar, por evidência, que **não destrói** nada. |
| **G4** | **Rollback sempre** | Toda edição guarda a versão anterior (snapshot ou `family_chain`). Se quebrou integridade, **volta**. |
| **G5** | **Conflito ≠ decisão** | R6 **só detecta e sinaliza** o conflito. **Quem decide quem "vence" é o Don.** |
| **G6** | **Validação por evidência** | Edição só aplica se comprovada por **evidência** (nível 4-5: código/teste). Nunca por LLM (nível 0). |
| **G7** | **Receptor anti-loop** | Trava se detectar **ciclo A→B→A**. Limite de N edições relacionadas por ciclo. |
| **G8** | **Custódia separada** | Quem propõe, quem aprova, quem executa: papéis distintos. Ninguém edita o que não valida. |
| **G9** | **Integridade > conveniência** | Edição que quebra `memoryintegrity`, `memoryguard` ou a **family chain** → **rejeitada de cara**. |

---

## 4. Modelo de Dados (cosca-intelligence.db)

Reutiliza o schema 004 já criado pelo DSMS (ADR-046):

```sql
rules             -- lógica determinística (condition/action JSON)
code_patterns     -- padrões de código (severity/auto_fix)
expert_systems    -- sistemas especializados (agrega rules)
decision_trees    -- árvores de decisão (tree JSON)
code_metrics      -- métricas de código (file_path/metric_type/value)
```

Todas as tabelas carregam `created_at`/`updated_at` — a **auditoria da edição** nasce no schema.

---

## 5. Interação com Sistemas Existentes

| Sistema | Como interage |
|---|---|
| **DSMS (ADR-046)** | Fornece o schema + a gestão física do `cosca-intelligence.db` |
| **Memory Curation / Decay (`.md`)** | Agora viram **código** — o engine é a implementação do R1-R6 |
| **Shadow Mode (ADR-033)** | O G3 usa o lastro do ADR-033 (observação antes de aplicar) |
| **Evidence / Verdict (ADR-037/038)** | G6 usa o evidence-level para validar edições |
| **Auto-recovery (ADR-014)** | G4 usa o mecanismo de rollback/recovery |
| **Knowledge (ADR-013/030)** | Fonte do corpus semântico que o engine consome |

---

## 6. Consequências

### 6.1 Ganhos

- O conhecimento **não estagna** nem **degenera**: conflito, substituição e promoção passam a ser **governados**.
- A casa vira **"inteligência que se governa"** — a "caixa-preta inversa" com freio.
- Decisões de edição são **auditáveis** (schema carrega `updated_at`, rollback, gate).

### 6.2 Custos

- O engine precisa ser **implementado em Go** (não é mais só spec).
- Complexidade nova: o Ciclo (curriculum/validação) roda na esteira (como `restcycle`).

### 6.3 Riscos (mitigados pelo Contrato)

| Risco | Mitigação |
|---|---|
| Loop de auto-correção | G7 (receptor anti-loop) + G5 (conflito não resolve sozinho) |
| Regressão de conhecimento | G1 (imutável) + G4 (rollback) + G6 (evidência) |
| Corrida de conflitos | G8 (custódia separada) + G2 (gate) |
| Amnésia | G1 (pinned) + shadow-first (G3) |

---

## 7. Referências

- **ADR-046** (DSMS — schema 004 pronto) · **ADR-044** (Learning Vaults) · **ADR-033** (Shadow Mode)
- **ADR-037/038** (Evidence / Epistemic) · **ADR-014** (Auto-recovery) · **ADR-013/030** (Knowledge)
- **`.cosca/engines/memory-curation/`** (CurationScore v1.0/v2.0 + R6) · **`.cosca/engines/evidence/`** (CONFIDENCE_MODEL)

---

## 8. Status

| Fase | Status | Owner |
|---|---|---|
| Design (este ADR) | ✅ PROPOSTO | cosca-kernel |
| Gate do Don | ⏳ PENDENTE | Don |
| Implementação do engine | ⏳ PENDENTE | cosca-backend / cosca-intelligence |
| Implementação do Contrato (G1-G9) | ⏳ PENDENTE | cosca-security / cosca-governance |
| Testes (shadow-first, anti-loop, rollback) | ⏳ PENDENTE | cosca-qa |

---

**Decisão redigida por:** Cosca Kernel (consigliere)
**Aguarda aprovação:** Don
**Próximo passo:** Gate do Don → implementação do Contrato de Salvaguarda (G1-G9) + engine em Go
