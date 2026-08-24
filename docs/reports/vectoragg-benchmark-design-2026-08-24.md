# Benchmark FULL-SCAN vs ROTEADO — Design do Experimento (ADR-013 §3.2)

> **DOCUMENTO DE DESIGN** · 2026-08-24 · **NENHUM código de produção alterado.**
> Ordem (professor/Don): **DESENHAR → AUDITAR O BENCHMARK → EXECUTAR → COMPARAR →
> PROVENANCE → só então decidir o próximo passo.**
>
> **A pergunta científica que importa** — NÃO é "ficou mais rápido". É:
> **"O roteador consegue reduzir drasticamente o espaço de recuperação SEM
> destruir a evidência relevante?"** O ganho não vem de otimizar o loop de
> cosine — vem de **não executar 99,9% do trabalho que não precisava existir.**

---

## 0. Estado de produção (o que existe — honestidade antes do design)

| Componente | Estado | Disponibilidade |
|---|---|---|
| `modlink.NewResolver(routes)` | ✅ existe | ✅ |
| Rotas registradas em produção | ❌ **SÓ EM TESTES** (`scope_test.go`, `faseb_pipeline_test.go`) | ⚠️ **falta criar rotas reais** |
| `confineToScope` (path/entity_type) | ✅ existe | ✅ |
| `vectoragg` (read-model, Fase A) | ✅ provado isolado, NÃO conectado | ⚠️ read-model separado |
| `search` com candidate retrieval | ✅ existe (L409-433) | ✅ |
| Corpus real indexado | `docs/`, `internal/`, `internal/embed/cosca/` | ✅ |

**Consequência de design:** para o experimento ROTEADO ter o que morder, é
**necessário um conjunto de rotas reais** (`{Trigger, Module, Capability}`)
mapeando os domínios do corpus. Sem isso, o router retorna `NoRoute` e o
"roteado" cai no full-scan (não mede nada).

**Escopo do design:** o benchmark mede o **caminho de busca** (full-scan vs
scoped/roteado) usando o **corpus real já indexado** + **rotas reais definidas**.
NÃO toca a Fatia 3 (world/vegetation/unreal físicos ficam congelados).

---

## 1. Design em 3 camadas (como o professor especificou)

### Camada 1 — Baseline (FULL SCAN)
Mesmo corpus, mesma query, mesmo embedding, busca no **universo inteiro**.
Mede: quantos candidatos entram, quantos BLOBs decodificados, memória, latência,
top-K final.

### Camada 2 — ROTEADO
`Query → Router → Scope → Candidate IDs → Retrieval → Ranking`.
Mesma query, mesmos dados, mesmo K.
Mede: candidatos do espaço roteado, BLOBs decodificados, memória, latência,
top-K final.

### Camada 3 — Comparação Científica
Não basta latência. As métricas:

| Métrica | O que queremos descobrir |
|---|---|
| **Candidate Reduction %** | quanto o router eliminou (1 − M/N) |
| **BLOBs decodificados** | quanto trabalho deixou de existir de fato |
| **Memória** | redução de materialização |
| **Latência** | ganho real |
| **Recall@K** | se perdeu evidência (a métrica NOBRE) |
| **Precisão / qualidade** | se o resultado ficou mais relevante |
| **Ruído** | quanto conteúdo irrelevante desapareceu |
| **Recall Retention %** | recall_rotedo / recall_fullscan |

> **Recall é obrigatório (o professor foi enfático).** Porque agora existe a
> pergunta: *"o router está estreitando sem eliminar evidência relevante?"*

---

## 2. Conjunto de QUERIES (o teste mais importante)

> **🔴 DIFF v1 → v2 (motivo registrado — a lição da auditoria):**
> A relação **tópico da query ≠ módulo onde a evidência está** é fundamental.
> *"Como funciona o cálculo de similaridade por cosseno?"* **não** significa que
> a evidência está no módulo `vector` — ela pode estar fisicamente em
> `knowledge/`, `architecture/`, `docs/`. O router deve ser avaliado contra a
> **localização real da evidência**, não contra o **nome conceitual do assunto**.
> Sem isso, mediria-se: `router correto → módulo vazio → 0 resultados →
> "router destruiu recall"` — quando na verdade foi `router → módulo sem
> conteúdo` (**erro de fixture/roteamento, não da arquitetura**).
>
> **Fontes: auditoria do cosca-critic (FAIL) + medição própria** — os módulos
> `vector/unreal/world/vegetation/materials` têm **0 docs indexados** no corpus
> real (confirmado por replicar a lógica do `confineToScope`).

### Queries v2 (só módulos COM conteúdo real no corpus)

Módulos validados por path-segment (medição): `memory`=1.664, `knowledge`=183,
`runtime`=17, `architecture`=49, `security`=7, `cli`=5 docs.

1. **Runtime** — ex.: "como configurar o serve como serviço systemd" → módulo `runtime`
2. **Memory** — ex.: "o que e a lei da familia memoria velocidade" → módulo `memory`
3. **Knowledge** — ex.: "como funciona o gate de integridade da chain" → módulo `knowledge`
4. **Cosseno (novo mapeamento)** — ex.: "como funciona o cálculo de similaridade por cosseno" → módulo **`knowledge`** (evidência em `docs/knowledge/search.md`, `docs/architecture/PIPELINE_TECNICO.md` — NÃO `vector`).
5. **Ambígua REAL (reescrita)** — ex.: "como o cosca mantém a memoria da familia entre sessões" → módulos **`memory` + `runtime`** (ambos com conteúdo). Testa se o router **amplia** o espaço para 2 módulos reais sem estreitar demais.

**POR QUE SEM vector/unreal/world:** a query de cosseno não é `vector` (evidência
está em `knowledge`/`architecture`); Unreal/world são Fatia 3 (sem conteúdo). As
rotas para módulos vazios **não entram** — seriam fixture inválida, não medida.

### Justificativa da ambígua (v2)

O teste realmente perigoso é: **"quando há ambiguidade, o router consegue ampliar o
espaço adequadamente ou estreita demais?"** Com `memory + runtime` (ambos com
conteúdo), conseguimos testar isso de verdade — em vez de rotas para módulos vazios.

- Full-scan acha a evidência correta em Top-10 **e** roteado também → **excelente**.
- Full-scan acha **e** roteado não → **router estreitou demais** (ampliar scope).
- **Ambos acham, roteado usa muito menos candidatos** → a propriedade a provar.

---

## 2.1 Regra de VALIDADE da execução (rigorosa — anti-false-positive)

Uma execução **só entra na tabela principal de métricas** se TODAS as condições:

```
scopeRouted(scope) == true
Scope.NoRoute      == false
len(Scope.Modules) > 0
ScannedVectors     < TotalVectors   (via SearchMetrics — trabalho REAL fez)
ground_truth pré-registrado (id exato)
rota esperada pré-registrada (módulo onde a evidência mora)
```

**Se qualquer condição falhar → a execução é `INVALID`** — NÃO entra no cálculo de
recall/performance. Isso impede que alguém olhe depreço para um `0 ms, 0
candidatos` e chame aquilo de otimização. (Regra do professor.)

---

## 2.2 Métrica de trabalho correta (ScannedVectors, não contagem pós-filtro)

> **DIFF v1 → v2 (motivo):** `BLOBs decodificados` e "contagem de resultados
> pós-`confineToScope`" são **enganosas** — podem mascarar o trabalho que JÁ
> aconteceu. O `confineToScope` roda DEPOIS da fase vetorial (search.go L283), então
> contar o resultado final não mede o custo real. E no caminho default
> (NewSQLiteVec in-memory int8), o full-scan decodifica 0 BLOBs (usa índice).

**A cadeia certa a medir:**
```
TotalVectors → ScannedVectors → Candidates → Decoded BLOBs → Top-K
```
- `ScannedVectors` via `vector.SearchMetrics` (métrica de trabalho REAL).
- `Candidates` = IDs que entraram no retriever confinado.
- `Decoded BLOBs` = só no caminho SQL-scan (contextualizar com `Int8Enabled`).
- **NUNCA** usar a contagem pós-`confineToScope` como "redução de trabalho".

> ⚠️ O caminho roteado só reduz trabalho REAL quando usa `params.CandidateIDs` +
> `scopeRouted` (L546/L425) — **não** apenas `SearchWithRoute` (que só seta `Scope`
> e faz a fase vetorial full-scan). O benchmark deve garantir que o caminho roteado
> passe `CandidateIDs` para ativar o scan bounded.

---

## 3. Formato do resultado (comparação visual)

```
TotalVectors → ScannedVectors → Candidates → Decoded BLOBs → Top-K

FULL     28.888 total → ScannedVectors → N candidatos → top-K
ROUTED   28.888 total → ScannedVectors → M candidatos → top-K
```

> **DIFF v1 → v2 (motivo):** o "exemplo alvo" (`Routed: 10 candidatos`) era
> **irrealista** — só o domínio `memory` tem 14.367 chunks; uma rota memory real
> produz **milhares** de candidatos, não 10. Substituir por **faixas realistas por
> domínio** (full-scan ≈ dezenas de milhares → scoped ≈ milhares), medindo a
> **razão** (Candidate Reduction %), não um alvo arbitrário.

**O que isso provaria:** o ganho **NÃO veio** de otimizar o loop de cosine —
veio de **não executar 99,9% do trabalho que não precisava ser executado**.

---

## 4. Escopo / limites do experimento (honestidade)

- **Rotas reais (test-only):** o benchmark DEFINE um conjunto de rotas `modlink`
  para os domínios reais do corpus **que têm conteúdo** (runtime/memory/knowledge/
  architecture/security/cli). Isso é INSTRUMENTAÇÃO (test-only), **não** mudança de
  produção. **As rotas para módulos vazios (vector/unreal/world/vegetation/materials)
  NÃO entram** — seriam fixture inválida (módulo sem conteúdo), não medida.
- **Não toca produção:** nenhum caminho de escrita; só leitura do `knowledge.db`
  (`mode=ro`) e do corpus. Nada indexa/reindexa/apaga.
- **Fatia 3 congelada:** NÃO cria módulos físicos de mundo.
- **Ground-truth NÃO-circular (regra do professor):** para CADA query, registrar
  ANTECIPADAMENTE (pré-execução, sem consultar a busca): (a) `document.id` exato;
  (b) segmento de path/domínio onde a evidência mora; (c) rota canônica esperada
  (módulo). (d) e (e) (full-scan e routed acham?) são coletados DEPOIS. **NUNCA**
  definir ground-truth como "o que o full-scan retorna" (circular — o recall diria
  100% trivialmente).
- **Gate de pré-execução:** a query só entra no benchmark se a rota apontar para um
  módulo com **≥1 item indexado** (`COUNT` por path-segment). Se 0 → query
  descartada (não "falha"), rota corrigida.

---

## 5. Fronteira (o que o benchmark NÃO é)

> **DIFF v1 → v2 (motivo):** o v1 tratava `vector/unreal/world` como módulos
> auditáveis. A auditoria provou que são **módulos sem conteúdo** (0 docs) — logo,
> **não são o que o experimento mede**. O benchmark v2 mede a arquitetura sobre os
> módulos com conteúdo real; os módulos vazios são Fatia 3 (congelada).

Este experimento é **medida de arquitetura**, não feature. Ele NÃO é:
- A Fatia 3 (não cria módulos físicos de mundo).
- A conexão do vectoragg ao pipeline (isso é Fatia 3).
- Um "benchmark de performance" genérico (foca em **recall retention** + candidate
  reduction — a propriedade arquitetural).
- Uma medição sobre módulos sem conteúdo (seria fixture inválida).

**O que ele decide:** se a arquitetura (router→scope→candidate→scoped retrieval)
mantém **recall** com **custo drasticamente menor** no corpus real. Se garantido,
é a **evidência experimental** de que o design funciona fora do caso sintético —
antes de tocar na Fatia 3.

---

## 6. Próximos passos (ordem do professor — atualizada)

1. **DESIGN v1** — desenhar o benchmark ✅ (rastro abaixo).
2. **AUDIT v1** → o `cosca-critic` **FALHOU** o design (queries p/ módulos vazios +
   ground-truth não-verificável + métrica maçã-laranja). Ver §7.
3. **EVIDENCE** — confirmação por medição própria (replicar `confineToScope`):
   `vector/unreal/world/vegetation/materials` = 0 docs; `memory`=1.664 etc.
4. **DESIGN v2** (este documento) — carrega o motivo de TODA mudança (DIFF v1→v2). ✅
5. **AUDIT v2** — re-auditar antes de executar. Só passa se o design estiver honesto.
6. **EXECUTAR** — rodar o benchmark (leitura), coletar as métricas por query (com a
   regra de validade §2.1).
7. **COMPARAR** — full-scan vs roteado, com `Recall Retention %` + `ScannedVectors`.
8. **PROVENANCE** — registrar números brutos + verdict em `docs/reports/`.
9. **Só então** decidir: recall retido alto + custo baixo → arquitetura provada com
   mundo real; router corta evidência → ajustar rotas/ampliar scope.

> **LINHA DE OURO (professor/Don):** NÃO mexer no runtime para fazer o benchmark
> passar. Se o benchmark v2 quebrar, **ótimo** — descobriu outra coisa antes de virar
> uma conclusão falsa.

---

## 7. PROVENANCE — AUDIT v1 (FAIL) e EVIDENCE (o rastro que NÃO se apaga)

> **Este é o registro da auditoria v1.** O design v2 carrega estas correções, mas a
> auditoria original NÃO é apagada — é a *prova de que não se aceitou um falso
> positivo*.

### AUDIT v1 — VERDICT: **NÃO APROVADO para executar** (cosca-critic)

| Item | Veredicto | Falha encontrada |
|---|---|---|
| **1. Ground-truth** | ❌ FAIL | Design só prometeia "cada query terá a resposta conhecida" — sem procedimento. Não definia (a) id exato (b) domínio (c) rota esperada (d) full-scan acha (e) routed acha. Risco de **circular** (definir ground-test como "o que full-scan retorna" → recall 100% trivial). |
| **2. Queries** | ❌ FAIL | 4/7 queries roteiam p/ módulos **sem conteúdo** (`vector`/`unreal`/`world`/`vegetation`/`materials` = 0 docs). Resultado seria recall-trivial-0 em módulo vazio (artefato de rota, não achado). |
| **3. Rotas reais** | ❌ FAIL | 5 dos 8 módulos não existem no corpus. Mapeamento **rota→evidência invertido** (query de cosseno → `vector`, mas evidência está em `knowledge`/`architecture`). Regra: módulo = onde a evidência realmente mora. |
| **4. Medição de recall** | ❌ FAIL | `confineToScope` roda DEPOIS do vetorial (L283) → o v1 poderia reportar redução que não existiu em trabalho. Fallback NoRoute → risco de full-scan disfarçado. |
| **5. Métricas** | ⚠️ Parcial FAIL | `BLOBs decodificados` enganosa (in-memory int8 usa índice → 0 BLOBs decode). "Memória" indefinida (índice residente vs materializado). "Exemplo alvo 10 candidatos" irrealista. |

### EVIDENCE (medição própria, read-only — replica exata do `confineToScope`)

| Módulo | Docs (path-segment match) | Conteúdo |
|---|---|---|
| `memory` | 1.664 | ✅ |
| `knowledge` | 183 | ✅ |
| `runtime` | 17 | ✅ |
| `architecture` | 49 | ✅ |
| `security` | 7 | ✅ |
| `cli` | 5 | ✅ |
| `vector` | **0** | ❌ vazio |
| `unreal` | **0** | ❌ vazio |
| `world` | **0** | ❌ vazio |
| `vegetation` | **0** | ❌ vazio |
| `materials` | **0** | ❌ vazio |

> Confirma a falha da auditoria: as queries sobre `vector/unreal/world/
> vegetation/materials` mediriam **módulo vazio**, não arquitetura. O design v2
> **remove essas queries** e mapeia cada rota pelo **path onde a evidência mora**. 
