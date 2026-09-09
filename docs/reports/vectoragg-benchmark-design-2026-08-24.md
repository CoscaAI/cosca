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

> **🔴 DIFF v2 → v2.1 (motivo — gate explícito da ambígua, re-auditoria Ponto 1):**
> o teste da ambígua só exercita a propriedade "amplia" se o resolver devolver
> **AMBOS** os módulos para a query exata (whole-word match; rotas test-only).
> **Gate de pré-execução obrigatório:**
> ```
> Resolve(query_ambígua).Modules  ==  [memory, runtime]   (exatamente, e ambos)
> ```
> Se não der exatamente isso → a query é **INVALID** (vira single-module
> silenciosamente e o teste perde o sentido). As **rotas test-only usadas**
> (trigger → módulo) são registradas na PROVENANCE.

---

## 2.1 Regra de VALIDADE da execução (rigorosa — anti-false-positive)

> **🔴 DIFF v2 → v2.1 (motivo — re-auditoria, Ponto 4/6 FAIL):** o v2 exigia
> apenas `ScannedVectors < TotalVectors`, mas isso é satisfazível **"de mentira"**:
> se `CandidateIDs` for **FTS-hits** (redução via registro lexical/hybrid, NÃO do
> router) ou um **subset manual** (ex.: só os chunks do doc de GT), a validação
> passa com **recall 100% trivial** — mas o **router NÃO descobriu** o candidato;
> ele foi **colocado artificialmente** no conjunto. Isso é o **falso-positivo de
> ouro** que a LINHA DE OURO proíbe. **Fonte:** cosca-critic (re-auditoria v2).

Uma execução **só entra na tabela principal de métricas** se TODAS as condições:

```
scopeRouted(scope) == true
Scope.NoRoute      == false
len(Scope.Modules) > 0
len(CandidateIDs)  > 0                       ← NÃO vazio (anti "0 trabalho")
CandidateIDs EXAUSTIVO do(s) módulo(s)       ← a prova-chave (abaixo)
ScannedVectors     < TotalVectors            (via SearchMetrics — trabalho REAL)
CandidatePool      == 0                      (isolamento — §2.2)
EnableGraph        == false                  (isolamento — §2.2)
ground_truth pré-registrado (id exato)
rota esperada pré-registrada (módulo onde a evidência mora)
```

**Se qualquer condição falhar → a execução é `INVALID`** — NÃO entra no cálculo de
recall/performance. Impede o falso-positivo `0 ms / 0 candidatos / recall 100%`.

### O que é "CandidateIDs EXAUSTIVO" (a prova verificável — não "parece correto")

`CandidateIDs` **NÃO** pode ser (a) FTS-hit, (b) GT conhecido, nem (c) subset
manual. Tem que ser **derivado do domínio**:

```
scope module
    ↓
document/path  (pathHasSegment — mesma lógica do confineToScope)
    ↓
JOIN document → vector   (document_id → vector.id)
    ↓
TODOS os vector.id elegíveis daquele módulo
```

**Prova de exaustividade (obrigatória, verificável):**
```
COUNT(DISTINCT vector.id via JOIN <módulo>)   ==   len(CandidateIDs)
```
Se não bater — o set não é o vocabulário completo do módulo → **INVALID**. Isso
transforma "CandidateIDs parece correto" em **propriedade verificável**.

> **Escopo da validação:** IDs FTS-lexicais (`chunks_fts_*`) são a via
> L3/hybrid (outra feature) e **NÃO contam** como "candidatos do roteador". O
> set exaustivo é **só o vetorial derivado do path → document → vector**.

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

> **🔴 DIFF v2 → v2.1 (motivo — isolamento, re-auditoria Ponto 3/4):** o benchmark
> deve responder **uma pergunta por vez** — o efeito do **routing sobre a
> recuperação vetorial**. Parâmetros que introduzem trabalho **fora** do módulo
> roteado (pool de recentes, grafo) contaminam a métrica e são **desligados**:
> - `CandidatePool = 0` — se >0, o pool de recentes do **índice inteiro** entra no
>   set candidato (infla `ScannedVectors`, importa out-of-scope). Só o roteado deve
>   definir o set.
> - `EnableGraph = false` — o grafo roda global e só é pós-cortado pelo scope;
>   isolar o efeito vetorial o desliga.
>
> Depois mede-se híbrido/grafo/FTS em experimentos SEPARADOS, um por vez.

> **🔴 DIFF v2 → v2.1 (motivo — narrativa precisa, re-auditoria Ponto 3):** a fase
> **FTS roda SEM escopo** (search.go L229+) e só é pós-cortada pelo `confineToScope`
> (L283). Portanto a redução medida é **SÓ da fase vetorial**. A narrativa correta:
> **"99,9% do trabalho da FASE VETORIAL foi eliminado"** — NUNCA "99,9% do trabalho
> total" (incorreto, pois o FTS ainda faz trabalho global). Essa precisão importa
> na PROVENANCE quando alguém analisar daqui a seis meses.

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

1. **DESIGN v1** — desenhar o benchmark ✅ (rastro §7).
2. **AUDIT v1** → o `cosca-critic` **FALHOU** (queries p/ módulos vazios +
   ground-truth não-verificável + métrica maçã-laranja). Ver §7.
3. **EVIDENCE** — medição própria: `vector/unreal/world/vegetation/materials` = 0 docs. ✅
4. **DESIGN v2** — carrega o motivo de TODA mudança (DIFF v1→v2). ✅
5. **AUDIT v2** → o `cosca-critic` **FALHOU** de novo (Ponto 4/6: `CandidateIDs`
   pode ser FTS-hit/subset e passar na validação com recall-trivial — o
   falso-positivo de ouro). Ver §8.
6. **DESIGN v2.1** (este documento) — fechou a porta do candidato não-exaustivo
   com as 5 correções. ✅
7. **AUDIT v2.1** — re-auditar ANTES de executar. Só passa se a regra de validade
   for insatisfazível "de mentira".
8. **EXECUTAR** — rodar o benchmark (leitura), coletar as métricas por query (com a
   regra de validade §2.1 + exaustividade).
9. **COMPARAR** — full-scan vs roteado, com `Recall Retention %` + `ScannedVectors`.
10. **PROVENANCE** — registrar números brutos + verdict em `docs/reports/`.
11. **Só então** decidir: recall retido alto + custo baixo → arquitetura provada com
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

---

## 8. PROVENANCE — RE-AUDIT v2 (FAIL) e as 5 correções → v2.1

> **Registro da re-auditoria v2** (o rastro que NÃO se apaga). O v2 era honesto na
> métrica de trabalho (`ScannedVectors`) e no mapeamento módulo=evidência, mas o
> crítico encontrou a **última porta aberta** — o falso-positivo de ouro.

### RE-AUDIT v2 — VERDICT: **NÃO APROVADO** (cosca-critic) — 5/7 itens passaram, 2 falharam

| Item | Veredicto | Falha |
|---|---|---|
| **1. Queries (só módulos com conteúdo)** | ✅ PASS | 5 queries em módulos reais; query de cosseno → `knowledge` correta (evidência em `docs/knowledge/search.md`, path-segment `knowledge`, não `vector`). |
| **2. Ground-truth não-circular** | ✅ PASS | Vínculo "ground-truth = o que full-scan retorna" cortado (domínio vem da path, não do resultado). |
| **3. ScannedVectors via SearchMetrics** | ✅ PASS | `vector.SearchMetrics.ScannedVectors` existe (vector.go L145) e é preenchido (sqlite_vec.go L333-343: `ScannedVectors=len(rows)` dos candidatos). |
| **4. Regra de validade** | ❌ **FAIL** | Satisfazível "de mentira": `CandidateIDs` pode ser FTS-hit (redução lexical, não router) ou subset (só chunks do GT) → `ScannedVectors<TotalVectors` passa com **recall 100% trivial** sem o router descobrir nada. |
| **5. Módulo = onde a evidência mora** | ✅ PASS | Path-segment correto; GT pré-registrado é uno e está no módulo da rota. |
| **6. Risco de candidato** | ❌ **FAIL** | Fallback full-scan coberto (vazios→INVALID), mas a "redução fabricada por set não-exaustivo" NÃO. Não existe helper (`IDsForModule`/`ModuleVectors`/`IDsByScope` → 0 resultados). |

### EVIDENCE (medição própria, confirma o FAIL)

- **Não existe** helper para derivar `CandidateIDs` como vocabulário exaustivo do módulo (grep por `IDsForModule`/`ModuleVectors`/`IDsByScope` → **0 resultados**).
- `CandidatePool` (search.go L130) é usado no caminho candidato (L429) — se >0, o **pool de recentes do índice inteiro** entra e **infla `ScannedVectors` + importa out-of-scope**.

### As 5 correções finais (v2 → v2.1) — fecham o falso-positivo

| # | Correção | Onde | Por quê |
|---|---|---|---|
| **1** | `CandidateIDs` **derivado do domínio** (não FTS-hit/GT/subset): `scope module → document/path (pathHasSegment) → JOIN document→vector → TODOS os vector.id elegíveis`. **Prova**: `COUNT(DISTINCT vector.id via JOIN) == len(CandidateIDs)`. | §2.1 | Transforma "parece correto" em **propriedade verificável**. |
| **2** | Regra de validade: `len(CandidateIDs) > 0` + **exaustividade** (nenhum vetor do módulo omitido). | §2.1 | Impede `0 candidatos → 0 trabalho → resultado conveniente`. |
| **3** | Narrativa: **"99,9% do trabalho da FASE VETORIAL"** (não "do trabalho total" — o FTS roda global). | §2.2 | Precisão de provenance para análise futura. |
| **4** | Isolamento: `CandidatePool=0` e `EnableGraph=false` (uma pergunta por vez — efeito do routing na recuperação vetorial). | §2.2 | Pool de recentes/grafo contaminariam a métrica. |
| **5** | Gate explícito da ambígua: `Resolve(query).Modules == [memory, runtime]` exatamente; senão `INVALID`. Rotas test-only registradas. | §2 (ambígua) | Evita o teste virar single-module silenciosamente. |

> **A PROPRIEDADE-CHAVE (professor/Don):** o resultado favorável precisa ser
> **conquistado pelo sistema**, não **colocado nas entradas**. Essa é a diferença
> entre um benchmark que mede uma otimização e um experimento que testa uma hipótese.

### Sequência final

```
DESIGN v1 → AUDIT v1 → FAIL → EVIDENCE → DESIGN v2 → AUDIT v2 → FAIL → EVIDENCE
  → DESIGN v2.1 (este) → AUDIT v2.1 (se passar) → EXECUTAR → COMPARAR → PROVENANCE
```

---

## 9. PROVENANCE — RE-AUDIT v2.1: **🟢 APROVADO para EXECUTAR** (cosca-critic, 3º round)

> **Registro da aprovação.** Após 3 rounds de auditoria adversarial, o `cosca-critic`
> **não encontrou furo no núcleo anti-falso-positivo**: o candidato é exaustivo
> (derivado de JOIN, não fabricado), a regra de validade não é satisfazível "de
> mentira", e o ground-truth é não-circular. **APROVADO para executar.**

### EVIDENCE da aprovação (medição própria, read-only — confirma o PASS)

| Módulo | Vetores (via JOIN document→vector, pathHasSegment) | Conteúdo |
|---|---|---|
| `memory` | 8.341 | ✅ |
| `knowledge` | 4.190 | ✅ |
| `architecture` | 1.728 | ✅ |
| `runtime` | 224 | ✅ |
| `cli` | 374 | ✅ |
| `security` | 161 | ✅ |
| `vector/unreal/world/vegetation/materials` | 0 | ❌ (fatia 3) |

- **Total = 28.888 vetores; `document_id` vazio = 0; órfãos = 0** → o `JOIN
  document→vector` captura **100%** dos vetores (exaustividade funciona na prática).
- → A prova `COUNT(DISTINCT vector.id via JOIN) == len(CandidateIDs)` é **factível
  e completa no corpus atual**.

### As 6 ressALVAS de RIGOR (não-fatais — registrar no script de execução)

O núcleo está fechado; estas ressalvas garantem que a medição não capture efeitos
que não sejam o roteamento:

1. **Full-scan irrestringível:** chamar o baseline com `Scope=nil` + `CandidateIDs`
   vazio (NUNCA via resolver — as rotas reais confinariam e mediria "redução 0"
   / clonagem). O roteado usa `ApplyScope` + `CandidateIDs` derivado do módulo.
2. **`SearchParams` idênticos nos dois braços** (mesmos `Limit`, `Tags`, `Types`,
   `EnableFTS`, `EnableVector`, `EnableGraph=false`, `CandidatePool=0`) — exceto
   `Scope`/`CandidateIDs`. Senão a diferença medida vem de filtros/flags, não do roteamento.
3. **Mesma representação de scoring** para recall justo: full-scan = int8/int16
   (in-memory), roteado = float32 (bounded). OU forçar float32 nos dois (DisableInt8/
   DisableInt16), OU documentar o ~0,5% de ruído de precisão.
4. **Gate exact para TODAS as queries** (`Resolve(query).Modules == conjunto
   esperado`), não só à ambígua. O gate "≥1 item" deixa entrar módulo errado.
5. **Narrativa:** reportar `1 − ScannedVectors_roteado / ScannedVectors_fullscan`
   **medido** (ex.: memory 71,1%, knowledge 85,5%, architecture 94,0%, runtime
   99,22%, cli 98,71%, security 99,44%), NUNCA "99,9%" como fato — só como ordem
   de grandeza hipotética.
6. **Prova de exaustividade:** registrar que `COUNT(JOIN)==len(CandidateIDs)` é
   verificação de **consistência/identidade do pipeline**, não de completude
   absoluta. Risco latente: vetores de entidade (`document_id=''`) caso surjam no
   futuro (hoje 0 no corpus — inofensivo).

### Mecanismo FULL-SCAN vs ROTEADO (especificado — evita a clonagem)

```
FULL-SCAN:  Engine.Search(ctx, params)  — Scope=nil, CandidateIDs vazio (irrestringível)
ROTEADO:    ApplyScope(resolver, query) — Scope=roteado, CandidateIDs=derivado do módulo
Ambos:      EnableGraph=false, CandidatePool=0, MESMO Limit (top-K igual)
```

### Veredicto

**🟢 APROVADO para executar** — com as 6 ressalvas de rigor registradas acima para
a medição ser honesta. Sequência: `EXECUTAR → COMPARAR → PROVENANCE`.
**NO CODE CHANGED** — o design foi auditar e aprovado sem tocar produção.

---

## 10. PROVENANCE — EXECUÇÃO v1 (resultados) + VERDICT do MISMATCH de embedding

> **Registro da 1ª execução do benchmark (2026-08-24).** A execução PROVOU a
> redução de `ScannedVectors` (evidência VÁLIDA), mas o **recall veio INVALID**
> por uma causa que foi AUDITADA (read-only) e FECHADA: **não é re-embed nem
> provider — é granularidade semântica query→chunk.**

### 10.1 Resultados da 1ª execução (números crus — evidência de redução VÁLIDA)

| Query | Dominio | TotalVec | CandidateIDs | Scanned (full→rote) | Lat vetorial (full→rote) | Recall@10 |
|---|---|---|---|---|---|---|
| serve systemd | runtime | 28.888 | 224 | 28.888→224 | 454→3 ms | 0 (full e roteado) |
| lei da familia | memory | 28.888 | 8.341 | 28.888→8.341 | 1,7→120 ms | 0 |
| gate da chain | knowledge | 28.888 | 4.190 | 28.888→4.190 | 2,3→51 ms | 0 |
| cosseno | knowledge | 28.888 | 4.190 | 28.888→4.190 | 2,3→52 ms | 0 |
| memoria da familia | memory | 28.888 | 8.565 | 28.888→8.565 | 2,2→125 ms | 0 |

**✅ Evidência VÁLIDA (fica):** `Candidate Reduction` + `ScannedVectors` (28.888 →
centenas/milhares) + **exaustividade `COUNT(JOIN)==len` = true** em todas.

### 10.2 AUDIT do MISMATCH de embedding (read-only — EVIDENCE)

| Verificação | Resultado | Veredicto |
|---|---|---|
| Provider/modelo da query | Ollama `nomic-embed-text`, 768-dim | ✅ |
| Provider/modelo do corpus | Mesmo config.yaml (`nomic-embed-text`, 768) | ✅ idêntico |
| Dimensão | Ambos 768 | ✅ |
| Normalização | corpus norm=**1.0**; query cru norm=**22.8** | ⚠️ mas cosseno é invariante → **não é a causa** |
| Blob íntegro? | self-cos = **1.0** | ✅ íntegro |
| **Re-embed do MESMO chunk** | cos(query re-embed) vs vetor armazenado = **1.0000** | ✅ **provider reproduz** |
| **Query vs chunk do GT** | cos = **0.44** | ⚠️ **a causa real** |

**VERDICT do mismatch — CAUSA FECHADA:**
> **Provider/model/dimensão/normalização estão COMPATÍVEIS.** O Ollama re-embeda o
> mesmo conteúdo com cos=1.0. O problema é a **granularidade semântica
> query→chunk**: as queries do benchmark são **perguntas genéricas curtas** que não
> chegam perto do **chunk específico** que contém a resposta (o corpus indexa
> chunks/frases, não docs). **NÃO é re-embed. NÃO mexer no corpus.**

**Consequência na medição:** o recall=0 em ambos os bancos (full e roteado) NÃO é
falha do router — é a query não se casar com o chunk. **Recall anterior = INVALID.
Redução de ScannedVectors = VÁLIDA.**

### 10.3 REDESENHO do recall em 2 NÍVEIS (v3 — pergunta arquitetural mais forte)

> **Correção de raciocínio (Don):** NÃO ajustar a query até ela acertar o chunk
> (seria outro viés). O recall ganha **dois níveis**:

**Nível 1 — `RECALL_DOCUMENT@K` (métrica PRINCIPAL):**
```
query → top-K chunks → QUALQUER chunk pertencente ao document_id correto → RECALL_DOCUMENT@K
```
Não exige que aquele chunk específico seja o representante semântico do documento.
Responde: **"o router remove ~71–99% do espaço SEM remover o documento relevante?"**

**Nível 2 — `RECALL_CHUNK@K` (métrica SECUNDÁRIA):**
Mantém a atual (chunk específico pré-registrado no top-K). Mais rigoroso; pode ser
menor naturalmente. Responde: **"achou exatamente o chunk pré-registrado?"**

**Query set pré-registrado (sem seleção pós-resultado):**
Queries que expressam o conteúdo real dos documentos, mas **pré-registradas antes
de rodar** — nunca escolhidas depois de ver o resultado. Fluxo: `query set →
pré-registro → ground_truth document_id → FULL vs ROUTED`. Proíbe o loop
"rodei → vi que falhou → fiz uma query que funciona → rodei de novo".

### 10.4 Regras da execução v3 (idênticas ao resto, só muda a medição do recall)

- **Mantém** (exatamente igual): FULL-SCAN (`Scope=nil`, `CandidateIDs` vazio) vs
  ROTEADO (`ApplyScope` + `CandidateIDs` derivado do domínio); `EnableGraph=false`;
  `CandidatePool=0`; mesmo `Limit`; mesmos `SearchParams`; exaustividade.
- **Só muda**: o cálculo do recall em **2 níveis** (document_id principal, chunk
  secundário).
- **NÃO altera produção.** NÃO re-embeda o corpus.

### Gating para execução v3

- A query só entra se `Resolve(query).Modules == [módulo esperado]` (gate exact,
  todas as queries) e a rota apontar para módulo com ≥1 item indexado.
- `RECALL_DOCUMENT@K` > 0 ⇒ o router preservou o documento (evidência arquitetural).
- `RECALL_CHUNK@K` pode ser 0 (não exige o chunk exato).

**Pergunta que a v3 responde (mais forte):** "O router consegue remover ~71–99%
do espaço vetorial SEM remover o documento relevante?" — em vez de só "achou o chunk?".
