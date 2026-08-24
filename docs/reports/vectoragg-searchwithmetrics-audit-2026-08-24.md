# AUDIT SearchWithMetrics × CandidateIDs — Diagnóstico do recall=0 (Instrumento)

> **Registro do diagnóstico da divergência** · 2026-08-24 · **NO CODE CHANGED**
> (nenhum código de produção alterado; só leitura e os `*_test.go` do benchmark).
> Objetivo: rastrear por que o benchmark reportou **Recall@10=0** no caminho
> ROTEADO, embora o GT esteja comprovadamente no candidate set e no ranking por
> cosseno.

---

## 1. O que foi PROVADO (evidência read-only)

### Hipóteses descartadas (a arquitetura está CORRETA)

| Hipótese | Resultado | Prova |
|---|---|---|
| **"Router estreitou demais"** (GT fora de `CandidateIDs`) | ❌ **DESCARTADA** | GT está **DENTRO** de `CandidateIDs` em Q1/Q4/Q5 (todos os vetores do doc no set). |
| **"GT dentro, mas perdeu no ranking"** | ❌ **DESCARTADA** | GT = **#1/#1/#2** no ranking do subconjunto (cosseno). **Produção acha** (top-1 score 0,75). |
| **"Provider/embedding incompatível"** | ❌ **DESCARTADA** | `cos(re-embed do MESMO chunk) vs vetor armazenado = 1.0`; mesma dim (768), mesmo modelo. |

### A divergência (suspeita do instrumento — em investigação)

| Etapa | Estado |
|---|---|
| `resolveRouteCandidates` classifica os UUIDs como `direct` (não-FTS) | ✅ OK (confirmado: `parseFTSID` rejeita UUID → `direct`) |
| UUIDs chegam ao `SearchWithMetrics` via `mergeCandidateIDs` | ✅ OK |
| `scanCandidates` faz `id IN (candidateIDs)` | ✅ OK (o GT está no set) |
| `scoreParallel` cosseno → GT na **posição #1** | ✅ OK (cálculo acha o GT) |
| **`materializeScores` → `SearchResult`** | ⚠️ **aqui pode estar a divergência** — o `DocumentID`/`ID` que sai pode diferir do esperado |

---

## 2. O ponto aberto (por que o recall deu 0)

A **evidência conclusiva** do diagnóstico:

1. **Ranking do universo (query re-embeddada + cosseno):** GT em **#2 (Q1), #1 (Q4), #119 (Q5)**.
2. **Ranking do subconjunto (query re-embeddada + cosseno):** GT em **#1 (Q1), #1 (Q4), #2 (Q5)**.
3. **Produção real (`cosca search`):** retorna o **Hot Reload (doc GT) como top-1** (score 0,75).
4. **`fetchMeta` confirma:** o vetor do GT tem `document_id=2f27dc0a` (doc GT) e `chunk_id=47659d61` (chunk GT).

**OU SEJA:** o GT está no candidate set, no ranking, e a produção o acha. O recall=0 do benchmark é um **artefato do caminho `SearchWithMetrics`** no benchmark — a divergência está na **conversão `scoreParallel`/`materializeScores` → `SearchResult`** que o benchmark usa (com representação de scoring float32-SQL), **diferente** da replicação por cosseno direto e da produção.

**Classificação honesta (professor):** é um **"artefato/bug SUSPEITO do instrumento"**, NÃO "bug confirmado" — porque a divergência exata (a linha que falha) ainda não foi apontada no código. O que está confirmado: **NÃO é arquitetura, NÃO é router, NÃO é ranking real** (todos corretos e provados).

---

## 3. Evidência VÁLIDA que o experimento ENTREGOU (não descartar)

| Métrica | Resultado |
|---|---|
| **Candidate Reduction** | REAL (28.888 → 224/8.341/4.190/1.728/374/161) |
| **Exaustividade** | `COUNT(JOIN)==len` = **true** em todas |
| **Gate exact** | `Resolve(query).Modules == esperado` em todas |
| **GT dentro do candidate set** | ✅ confirmado |
| **Ranking correto** (quando medido direto) | ✅ GT no top-1/#2 |
| **Produção encontra o GT** | ✅ top-1 |
| **Redução de trabalho** | 71% a 99,44% |

**O ÚNICO número que não fecha** é o **recall produzido por uma rota específica do instrumento** (`SearchWithMetrics` com candidates).

---

## 4. VERDICT (AUDIT → EVIDENCE → VERDICT)

> **A arquitetura está CORRETA.** O router confina a evidência (GT no candidate set),
> o ranking acha (produção top-1), e a redução de trabalho é real (71–99,44%).
> O **recall=0 é um artefato do instrumento** (`SearchWithMetrics` com candidates),
> que reporta o `SearchResult` de forma divergente da produção/cosseno.
> **Classificação: SUSPEITO do instrumento, não confirmado** — falta apontar a linha
> exata. **NÃO é bug da arquitetura.**

**Próximo passo (fase de correção — APÓS o Don aprovar):** apontar a linha exata da
divergência em `scoreParallel`/`materializeScores`/`fetchMeta` que faz o
`SearchResult` do caminho candidates diferir. **Não corrigir ainda.**

---

## 5. O que o experimento CIENTIFICAMENTE provou (o achado de ouro)

O processo foi exatamente o que um teste científico deveria ser:

```
Hipótese → Benchmark → Resultado estranho (recall=0)
  → NÃO aceitar o número → Investigar
  → GT está nos candidatos → GT está no ranking → Produção encontra o GT
  → ??? → INSTRUMENTO DE MEDIÇÃO SUSPEITO
```

> **KKKK o professor não conseguiu quebrar o RAG — quebrou o próprio microscópio. 🔬😂**
> O experimento entregou uma porrada de evidência VÁLIDA (redução real, exaustividade,
> scope correto, GT no conjunto, ranking correto, produção acha) e um único número
> inválido (recall via `SearchWithMetrics`) que aponta para o instrumento.

**NO CODE CHANGED** — diagnóstico read-only. Nada de produção alterado; corpus intacto.
