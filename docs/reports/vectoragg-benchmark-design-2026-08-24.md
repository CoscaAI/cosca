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

Várias queries conhecidas, com **resposta relevante que sabemos existir**:

1. **Runtime** — ex.: "como configurar o serve como serviço systemd"
2. **Memory** — ex.: "o que e a lei da familia memoria velocidade"
3. **Knowledge** — ex.: "como funciona o gate de integridade da chain"
4. **Vector search** — ex.: "como o ranking de similaridade de cosseno funciona"
5. **Unreal** — ex.: "como configurar o S1_TestMap com atmosfera e meio-dia"
6. **World Model** — ex.: "o que e o world model como linguagem do cosca"
7. **Ambígua (a mais importante)** — ex.: "como gerar uma árvore no terreno"
   (dispara `vegetation`/`world`/`materials` — e testa o caso onde o scope pode
   ser estreito demais)

**Justificativa da ambígua:** uma arquitetura parece maravilhosa enquanto o
router recebe consultas perfeitamente classificáveis. O teste real é:
*"quando a query é ambígua, o router estreita demais o espaço?"*

- Full-scan acha a evidência correta em Top-10 **e** roteado também → **excelente**.
- Full-scan acha **e** roteado não → **NÃO é falha do vector search**; é
  evidência de que **o router precisa ampliar o scope**. Separa router ≠
  semantic search ≠ candidate retrieval ≠ ranking.

---

## 3. Formato do resultado (comparação visual)

```
FULL     80.000+ universo → N candidatos → K resultados
ROUTED   80.000+ universo → M candidatos → K resultados

Exemplo alvo (o que a história contaria):
  Universo:          80.000
  Full scan:         28.888 BLOBs decodificados
  Routed:               10 candidatos
  Recall@10:           100%
```

**O que isso provaria:** o ganho **NÃO veio** de otimizar o loop de cosine —
veio de **não executar 99,9% do trabalho que não precisava ser executado**.

---

## 4. Escopo / limites do experimento (honestidade)

- **Rotas reais:** o benchmark DEFINE um conjunto de rotas `modlink` para os
  domínios reais do corpus (runtime/memory/knowledge/vector/unreal/world). Isso é
  parte do experimento (INSTRUMENTAÇÃO), não mudança de produção — ou fica em
  `internal/*/bench` test-only, ou em fixture de benchmark.
- **Não toca produção:** nenhum caminho de escrita; só leitura do `knowledge.db`
  (`mode=ro`) e do corpus. Nada indexa/reindexa/apaga.
- **Fatia 3 congelada:** NÃO cria `vegetation.db`/`world.db` físicos. As rotas
  `vegetation`/`world` apontam para escopos, não para módulos físicos.
- **Recall ground-truth:** cada query terá a resposta-relevante conhecida
  (o documento/ID que devemos achar). Para o recall ser medido honestamente.

---

## 5. Fronteira (o que o benchmark NÃO é)

Este experimento é **medida de arquitetura**, não feature. Ele NÃO é:
- A Fatia 3 (não cria módulos físicos de mundo).
- A conexão do vectoragg ao pipeline (isso é Fatia 3).
- Um "benchmark de performance" genérico (foca em **recall retention** + candidate
  reduction — a propriedade arquitetural).

**O que ele decide:** se a arquitetura (router→scope→candidate→scoped retrieval)
mantém **recall** com **custo drasticamente menor** no corpus real. Se garantido,
é a **evidência experimental** de que o design funciona fora do caso sintético —
antes de tocar na Fatia 3.

---

## 6. Próximos passos (ordem do professor)

1. **DESENHAR** (este doc) ✅
2. **AUDITAR o benchmark** — revisar as rotas reais + queries + ground-truth
   (garantir que medem recall de verdade, não latência só).
3. **EXECUTAR** — rodar o benchmark (leitura), coletar as 8 métricas por query.
4. **COMPARAR** — full-scan vs roteado, com Recall Retention %.
5. **PROVENANCE** — registrar o resultado em `docs/reports/`.
6. **Só então** decidir: se recall retido alto + custo baixo → arquitetura provada
   com mundo real; se router corta evidência → ajustar rotas/ampliar scope.
