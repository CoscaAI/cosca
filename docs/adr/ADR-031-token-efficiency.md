# ADR-031: Otimização de Tokens — Token Efficiency como métrica-guia (desperdício, não volume)

> **Status:** Proposed (aguardando cosca-cto + cosca-architecture + Don) | **Owner:** cosca-architecture (Architecture Chief) | **Last Updated:** 2026-08-29
> **Revisão:** decisão de design — NÃO implementada de uma vez. Define o norte e prioriza as frentes; o primeiro passo (métrica por execução) é barato.
> **Fonte (ordem do Don + professor, 2026-08-29):** a tese de que o inimigo não é o **volume** de tokens, é o **desperdício**. "Não precisa reduzir a inteligência do agente; precisa reduzir o quanto ele precisa carregar para ser inteligente." E a métrica que importa: **Useful Work / Tokens** — onde "useful work" é DECOMPOSTO em dimensões (knowledge_gain, task_progress, artifact_value, evidence_gain, decision_gain), não uma única métrica (revisão do professor: só `knowledge_gain` ensinaria "só vale aprender").
> **Contexto:** o Don observou uma tarefa ir de **100k → 150k tokens** (custo triplicou, mas ainda barato). O ponto do professor: 100k→150k é OK se for tarefa mais complexa; é desperdício se for contexto repetido/investigação redundante. **Sem decomposição do uso, não se sabe qual.**
> **Relação:** cruza com **ADR-030** (memória modular/versionável — a chave do cache por resultado) e **ADR-029** (knowledge_snapshot — a chave de cache version-safe).

---

## 0. Mapa honesto do estado atual (o que JÁ EXISTE — crítica antes do gap)

> **Nota de veracidade (auditada em código):** o COSCA **já coleta telemetria de tokens** e **já tem cache de busca**. A proposta do professor NÃO é do zero — a fundação existe. Re-implementar seria duplicar. O que falta é **decompor** e **elevar** o que já está coletado.

### 0.1 Telemetria de tokens — JÁ EXISTE e é rica

| Peça | Onde | Estado |
|---|---|---|
| `chat.Usage{PromptTokens, CompletionTokens, TotalTokens}` | `internal/chat/ports.go:159` | ✅ coletado por execução |
| `engine.TokenUsage` (sessão + subagente) | `internal/engine/types.go:44,177` | ✅ acumulado |
| `registry.RecordRequest(usage.TotalTokens)` | `internal/chat/registry.go:351` | ✅ registrado por provider |
| Provider adapter → `ChatEventDone.Usage` | `internal/chat/provider/adapter.go` | ✅ propagado |
| `AgentResponse{TokensUsed, DurationMs}` (por pergunta) | `internal/benchmark/runner.go:139-143` | ✅ medido por agente/task |
| `benchmark.TokensUsed = Input+Output` | `internal/benchmark/runner.go:142` | ✅ |

**Conclusão:** o COSCA **já sabe quantos tokens** cada execução usa. **O que falta** é a **decomposição** (onde estão: base/contexto/ferramentas/histórico/repetição) e o **`knowledge_gain`** (informação útil por execução).

### 0.2 Cache — já existe para BUSCA (não para RESULTADO de agente)

| Peça | Onde | Estado |
|---|---|---|
| **Cache de busca por fingerprint** (multi-tier Mem→SQLite→FS) | `internal/knowledge/cachefingerprint.go` | ✅ |
| Chave determinística de todos os parâmetros (`searchCacheKey`) | `internal/knowledge/knowledge.go:2136` | ✅ |
| Escopo-aware (módulo A nunca serve módulo B) | `internal/knowledge/*_test.go` | ✅ |
| TTL 5min | `cachefingerprint.go:42` | ✅ |

**Conclusão:** o cache de **busca semântica** já é bom e escopo-aware. **O que falta** é o cache de **resultado de agente** — reutilizar a DESCOBERTA feita por um agente em outro agente (ex.: as tarefas em paralelo "40 agentes"). É a frente de maior impacto no custo paralelo.

---

## 1. Contexto / Problema (por que tokens desperdiçados, não volume)

Uma execução que vai de 100k para 150k pode ser:
- **Bom:** tarefa mais complexa — mais informação nova.
- **Ruim:** o agente ficou confortável — contexto repetido, investigação redundante, "já descobri mas vou investigar mais 40k".

**Sem decomposição, o COSCA não consegue distinguir.** O professor identificou exatamente isso e propôs a métrica certa: **Useful Work / Tokens** (com "useful work" decomposto em dimensões). Dois agentes, mesma taxa de sucesso, eficiências diferentes (0.78 vs 1.91) → o sistema aprende qual é melhor para aquele tipo de tarefa.

**Problema em uma frase:** instrumentar o COSCA para medir **onde** tokens são desperdiçados por execução, e usar **Useful Work / Tokens** (decomposto) para decidir (a) se um agente merece mais contexto, (b) qual agente/estratégia é melhor para cada tipo de tarefa, (c) quando o desperdício deve ser podado.

---

## 2. Decisão — Token Efficiency como métrica-guia, instrumentação por execução

**Adotar** o modelo de **medir trabalho útil por token consumido** e usar isso para guiar as frentes de otimização, na ordem de maior retorno. A métrica transversa:

```
Useful Work / Tokens = (dimensões de valor) / tokens_consumed
```

> **Correção (após revisão do professor, 2026-08-29):** definir "useful" como **uma ÚNICA dimensão** (ex: só `knowledge_gain`) ensina o agente a lição errada — *"só vale a pena trabalhar se eu gravar algo na memória"*. Falso: corrigir um bug pode dar `knowledge_gain=0` mas `task_success=1`, `artifact=1`, `regression_fixed=1`. **Trabalho útil às vezes é resolver, não aprender.** Por isso "useful work" é DECOMPOSTO em dimensões (Frente 1), com `knowledge_gain` sendo UMA delas, não a única.

### 2.0 Frente 1 (primeiro ataque — barato, instrumentação) 🎯

**Registrar por execução** (o COSCA já coleta tokens; falta o vetor de valor + agregação):

```
agent_id
task_id
input_tokens
output_tokens
tool_tokens
context_tokens
cached_tokens
delegated_tokens
duration
result
# Useful Work — DECOMPOSTO em dimensões (não uma só métrica):
knowledge_gain      ← aprendizado (delta de conhecimento antes/depois)
task_progress       ← sucesso na resolução (0..1)
artifact_value      ← artefato produzido (código/docs/evidência)
evidence_gain       ← evidência validada (P0-P5)
decision_gain       ← decisão tomada (D-XXXX)
```

- **Barato:** aproveita a telemetria que já existe; adiciona o vetor de "useful work" (múltiplas dimensões, não só `knowledge_gain`).
- **Objetivo:** responder "quem está queimando tokens e por quê" — sem isso, otimizar é chute.
- **Saída:** seleção como `cosca cost` (relatório por agente/task de Useful Work / Tokens, com as dimensões decompostas).
- **Anti-padrão que evita:** NÃO priorizar exclusivamente `knowledge_gain` — senão o COSCA aprende "só vale aprender", e ignora resolução/correção que é o trabalho útil mais comum.

### 2.1 Frente 2 — Cache por resultado de agente (maior impacto no paralelismo) 🎯

Reutilizar a DESCOBERTA de um agente em outro, com chave **version-safe** (combina com ADR-029/030):

```
cache_key = hash(task) + knowledge_snapshot + agent_capability
```

- A `knowledge_snapshot` (ADR-029) garante que o cache **nunca mistura versões diferentes** de conhecimento (o ponto do professor: "cache não fica perigoso por misturar versões").
- **Maior impacto** nas tarefas em paralelo (o "40 agentes" que o professor brincou).
- Diferente do cache de busca já existente: este cacheia o **resultado da tarefa** (a resposta/descoberta), não os resultados de busca.

### 2.2 Frente 3 — Progressive disclosure (contexto em camadas) 

```
L0 — Task
L1 — Summary
L2 — Knowledge relevante
L3 — Evidence
L4 — Source
L5 — Full history
```

Regra: **comece na menor camada capaz de resolver; se L2 resolve, nunca chega em L4.** O agente pode pedir expansão ("preciso da evidência original do item X"). Diferente do retriever atual (que joga contexto "inteiro"), isso injeta apenas o necessário.

### 2.3 Frente 4 — Token Budget Governor dinâmico

Ao invés de `max_tokens` fixo, um **orçamento dinâmico**:

```
budget = 30k
  ↓ produtivo  → 50k
  ↓ contínuo   → 80k
  ↓ começa a repetir → STOP
```

Deixa o agente **provar** que precisa de mais recursos, em vez de limitar cedo ou deixar gastar à toa. Orçamentos por complexidade: simple=10k, medium=30k, complex=75k, research=150k.

### 2.4 Frente 5 — Early-stop por suficiência (não só max_tokens)

Condição de suficiência, não apenas limite:

```
new_information_rate → 0 ✓
confidence           → high ✓
unresolved_questions → 0 ✓
→ encerra
```

Evita o "já descobri, mas vou investigar mais 40k".

### 2.5 Frente 6 — Resumo operacional de agente filho (não a narrativa)

O filho pode pensar com contexto grande, mas devolve **resultado operacional** ao pai (não a "novela"):

```
result: { status, answer, facts[], evidence[], decisions[], uncertainty[], artifacts[], knowledge_delta[] }
```

---

## 3. Implementação — prioridade e ordem

### 3.1 Fase 0 — Instrumentação (Frente 1) 🎯 **PRIMEIRO** (barato, não toca o recall)

- Adicionar `knowledge_gain` + agregação por execução (telemetria ja existe).
- Relatório `cosca cost` (Token Efficiency por agente/task).
- **Gate:** nenhum refactor de motor; só registro. NÃO toca recall.

### 3.2 Fase 1 — Cache por resultado de agente (Frente 2)

- Chave `hash(task) + knowledge_snapshot + capability` (version-safe, ADR-029).
- Cache do resultado da tarefa (não da busca).
- **Gate:** bateria de testes + baseline de recall (o cache não pode servir resultado errado de outra versão de conhecimento).

### 3.3 Fase 2+ — Progressive disclosure + Budget Governor + early-stop + resumo operacional

- Refactors de orquestração — maiores, dependem das métricas da Fase 0 mostrarem onde está o desperdício real.

> **Nota de segurança/rigor:** as Frentes 3-6 (progressive disclosure, budget governor, early-stop, resumo) **não devem** ser implementadas ANTES da Fase 0 — sem a decomposição de onde estão os tokens, otimiza-se a parte errada. O professor é explícito: "meça onde estão os 150k antes de otimizar."
>
> **Método de decisão (o "onde está o 150k" do professor):** após a Fase 0, o COSCA deve primeiro responder **onde estão os tokens**, e SÓ ENTÃO escolher a frente:
>
> ```
> Task #4821 — 151,203 tokens
> Contexto       61,220  40.4%
> Tools          34,881  23.1%
> History        28,430  18.8%
> Output         14,921   9.9%
> System          7,204   4.8%
> Repetition      4,547   3.0%
> ```
>
> - Se **60% em contexto** → progressive disclosure (Frente 3) é o tiro.
> - Se **40% em ferramentas repetidas** → o problema é outro (reuso de tool/resultado — cache de resultado, Frente 2).
> - Se **o vilão é delegação em cascata** → mexe no orchestrator (budget governor, Frente 4).
>
> **Não decidir a frente antes de medir.** A Fase 0 é o pré-requisito de tudo.

---

## 4. Consequências

**Positivas:**
- **Token Efficiency** como métrica de resultado (não volume) — o sistema aprende "para este tipo de tarefa, agente B é melhor".
- Cache version-safe = paralelismo barato sem risco de misturar versões de conhecimento.
- Progressive disclosure = contexto mínimo suficiente, sem perder inteligência.
- Budget dinâmico = agente não limitado cedo, mas também não desperdiça.

**Negativas / trade-offs:**
- Instrumentação é trabalho contínuo (manter a telemetria correta).
- Cache precisa de invalidação cuidadosa (nunca servir resultado de outra versão de conhecimento).
- Progressive disclosure pode adicionar latência de round-trip (agente pede expansão).
- Budget governor é heurístico — risco de cortar um agente que precisava de mais.

### 4.1 O que NÃO muda

- A **inteligência do agente** — não se reduz o modelo, só o que ele carrega.
- O **motor de busca/recall** (as Frentes 3-6 são de orquestração, não de busca; apenas a Frente 2 adiciona cache, sem tocar o recall do índice).
- A **telemetria de tokens existente** (é reaproveitada, não duplicada).

---

## 5. Alternativas consideradas

| Alternativa | Veredito |
|---|---|
| **A. Reduzir tokens simplesmente** (max_tokens menor) | ❌ Rejeitada — não resolve o desperdício; limita o agente e pode degradar qualidade. O professor: "não é diminuir tokens, é diminuir tokens desperdiçados". |
| **B. Trocar por modelo menor** | ⚠️ Reduz custo mas reduz inteligência — não é o objetivo. |
| **C. Token Efficiency + instrumentação + cache + progressive disclosure (ESTE ADR)** | ✅ Escolhida — ataca o desperdício, preserva a inteligência, usa a fundação que já existe. |
| **D. Só cache de busca (já existe)** | ⚠️ Insuficiente para o paralelismo; o gap é cache de RESULTADO de agente. |

---

## 6. Verificação (como saber que funciona)

1. **Fase 0:** `cosca cost` por agente/task mostra Token Efficiency; as execuções que gastaram 150k aparecem decompostas (base/contexto/ferramentas/histórico/repetição).
2. **Fase 1:** duas tarefas iguais em versões diferentes de conhecimento → cache não mistura (teste com `knowledge_snapshot`).
3. **Token Efficiency:** agente que mantém sucesso com menos tokens (e.g. 1.91) é preferido para aquele tipo de tarefa.
4. **Baseline de recall** (ADR-030) preservado — nenhuma frente toca o recall do índice sem o gate.

---

## Referências

- **ADR-029** — knowledge_snapshot (chave do cache version-safe); manifest/lock; Fase 2A decisão↔snapshot.
- **ADR-030** — memória modular; cache por resultado se alinha ao modelo de objetos `K:<hash>`.
- **Telemetria de tokens existente** — `internal/chat/ports.go`, `internal/engine/types.go`, `internal/benchmark/runner.go`.
- **Cache de busca existente** — `internal/knowledge/cachefingerprint.go`.
- **Proposta do professor (2026-08-29)** — "Token Efficiency = informação útil / tokens"; o inimigo é o token desperdiçado.

---

> **Autor:** Ordem do Don + orientação do professor (2026-08-29) | **Formalizado por:** cosca-architecture | **Revisão pendente:** cosca-cto + Don | **Status:** Proposed — Fase 0 (instrumentação) recomendada primeiro.
