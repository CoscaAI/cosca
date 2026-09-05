# ADR-038: O veredito do instrumento de avaliação é um estado epistêmico — verdade-do-mundo ≠ observação-do-instrumento

> **Status:** ADOTADO como doutrina (2026-09-04) | **Owner:** cosca-architecture (Architecture Chief) | **Last Updated:** 2026-09-04
> **Natureza deste ADR:** é a **FONTE CANÔNICA** (doutrina arquitetural). **NÃO** cria código novo, **NÃO** altera `.go` funcional. Apenas consolida a regra de que **o rótulo de eval é um estado epistêmico** — e nunca uma afirmação incontestável sobre o mundo.
> **Lição que formaliza:** a Campanha-001 (fine-tune LoRA no Qwen3-4B) regrediu o `pass%` de **0.88 (base)** para **0.75 (LoRA)**; o *Golden Gate / PromotionGate* detectou a regressão e **REPROVOU** o modelo — o mecanismo anti-autoengano funcionou e a decisão correta foi **manter `qwen3:4b` base**. A lição mais funda, porém, não foi "o LoRA é pior": foi que **um rótulo `FAILURE`/`FALSE_COMPLETION` é uma ALEGAÇÃO do instrumento**, não a verdade do mundo.
> **Referências:** `ADR-037` (primitiva `Verdict` — companheiro desta doutrina), `ADR-022` (verificação via prova), `ADR-011` (RAG fidelity gates), `ADR-031` (token efficiency), `ADR-036` (capacidade ≠ provider; percepção é evidência). **Proveniência externa:** `github.com/simplifaisoul/osiris` (MIT), `src/lib/sherlock.ts` (calibração com dois controles / tri-estado).

---

## 1. Contexto / Problema (por que esta doutrina é necessária)

A Campanha-001 instalou um fine-tune LoRA no Qwen3-4B. O instrumento de avaliação (o Golden Gate / PromotionGate) mediu `pass%` e registrou queda de **0.88 → 0.75**. O gate fez seu trabalho: detectou regressão e **REPROVOU** o modelo. A decisão que coube ao sistema foi correta — **manter o `qwen3:4b` base**.

Isso prova o **mecanismo anti-autoengano**: o modelo não promoveu a si mesmo; o instrumento julgou e vetou. **Isso é ouro e permanece.**

**Porém, o resultado correto foi, em parte, sorte epistêmica.** O gate conseguiu observar com confiança. A lição que este ADR cristaliza é uma armadilha **latente** que a campanha-001 expôs mas não enfrentou: a confusão entre **dois eixos independentes**.

O eixo do problema é: um rótulo de avaliação se apresenta como um **fato sobre o mundo** ("o modelo falhou", "a operação não retornou o esperado"). Mas um rótulo é, na realidade, **uma ALEGAÇÃO que o instrumento diz ter observado**. Se o instrumento esteve **BLOCKED / INCONCLUSIVE** (não conseguiu observar), então a conclusão "o modelo falhou" é **epistemologicamente inválida** — o modelo pode ter acertado e o instrumento lido errado.

**O análogo: o soft-404 do Sherlock.** Um servidor responde `200 OK` para uma página que não existe (soft-404). Insistir que "a página existe" porque o instrumento respondeu `200` é confundir **a leitura do instrumento** com **a verdade do mundo**. No eval: `FAILURE` pode ser o "soft-404" do instrumento — o mundo estava certo, o instrumento leu errado.

**Problema em uma frase:** o COSCA afirma "o modelo falhou" quando, muitas vezes, a única afirmação garantida é "o instrumento não conseguiu observar". Sem distinguir esses dois eixos, **quebramos o anti-autoengano para entrar no auto-engano reverso**: rejeitamos modelos bons por instrumento cego, ou promovemos modelos ruins por instrumento enviesado.

---

## 2. Decisão / Doutrina

**Adotar** que o veredito do instrumento de avaliação é **ele próprio um estado epistêmico** — pertencente ao **observador**, não ao objeto. E que dois eixos **independentes** governam qualquer veredito:

### 2.1 Os dois eixos (nunca conflacionar)

| Eixo | Pergunta | Exemplos de estado |
|---|---|---|
| **(1) VERDADE DO MUNDO** | O objeto existe? A operação funcionou? O modelo estava certo? | `WORLD_CORRECT` / `WORLD_INCORRECT` |
| **(2) OBSERVAÇÃO DO INSTRUMENTO** | Conseguimos observar isso **com confiança**? Fomos bloqueados? O veredito é confiável? | `OBSERVED` (confiável) / `BLOCKED` · `INCONCLUSIVE` (não confiável) |

Os dois eixos são **ortogonais**: um diz o que é; o outro diz **o que conseguimos saber disso**.

### 2.2 A matriz dos 4 quadrantes (o coração da doutrina)

| | **Instrumento observou bem** (`OBSERVED`) | **Instrumento NÃO conseguiu observar** (`BLOCKED`/`INCONCLUSIVE`) |
|---|---|---|
| **Mundo certo** (`WORLD_CORRECT`) | **FOUND real** — a lesão é confirmada. Veredito válido. | **INCONCLUSIVE** — o modelo pode ter acertado; **`FAILURE` é afirmação não garantida.** |
| **Mundo errado** (`WORLD_INCORRECT`) | **NOT_FOUND real** — a lesão é real. Veredito `FAILURE` **válido**. | **INCONCLUSIVE** — o modelo pode ter errado; **mas não provamos.** |

**A regra que decorre da matriz:** o rótulo de eval **deve ser sempre qualificado pelo estado observacional do instrumento**. Um `FAILURE` **só é válido** quando o instrumento observou **com confiança** (coluna `OBSERVED`). Quando o instrumento não conseguiu observar (coluna `BLOCKED`), o veredito **é `INCONCLUSIVE`, não `FAILURE`** — independentemente de o mundo ter acertado ou errado.

> **Fórmula da casa:** o rótulo diz o que o instrumento **segurou**. O que ele não segurou, ele **não afirma** — vira `INCONCLUSIVE`.

### 2.3 A consequência prática (evolução, não só eval)

Esta doutrina **motiva a adoção da primitiva `Verdict`** (referida no `ADR-037`): o veredito carrega **não apenas** o resultado (`PASS`/`FAIL`/`REJECT`), mas o **estado observacional** que o suporta. O Golden Gate, o CMI, o *review/critic* e a evolução passam a julgar **a confiança da observação antes de emitir um jugamento sobre o mundo**.

### 2.4 A doutrina de evidência generalizada

A mesma regra que distingue *capacidade ≠ provider* (`ADR-036`, *percepção é evidência*) e *verificação só com prova* (`ADR-022`) se **generaliza** aqui: **uma afirmação sobre o mundo só é aceita na medida em que o instrumento a observou.** O princípio único é:

> **Não confundir a leitura do instrumento com a verdade do mundo.** A leitura do instrumento é um **estado epistêmico do observador**; a verdade do mundo é o que queremos saber. Entre os dois, só atravessa quem observou com confiança.

---

## 3. Consequências

### 3.1 O que muda (aditivo, na direção da doutrina)

- **Os rótulos de eval passam a carregar o estado observacional do instrumento.** Um `FAILURE`/`FALSE_COMPLETION` só é emitido quando há observação confiável. Caso contrário, o rótulo é `INCONCLUSIVE`, com o estado `BLOCKED` registrado.
- **O CMI e o Golden Gate não conflacionam erro-de-modelo com lacuna-observacional.** Ferramentas que não responderam, timeouts, malformação de saída, OCR/parse falho, instrumento bloqueado → **lacuna-observacional**, NÃO erro-de-modelo.
- **PromotionGate / evolução (`ADR-022`):** um candidato só é rejeitado por "regressão" quando a regressão é **observada**. Se o instrumento estava `INCONCLUSIVE`, a campanha não pode ser reprovada *por falha* — só *por inconclusão* (outra categoria, com outra ação).
- **Honestidade epistêmica:** um repositório que registra `INCONCLUSIVE` em vez de `FAILURE` mente menos. Esta é a mesma postura da família `.go` canônica (`EVIDENCE` ≠ `FACT`; `DECISION` é classe, não verdade).

### 3.2 O que NÃO muda

- O **mecanismo anti-autoengano da campanha-001 continuou correto**: o gate observou bem, e o mundo estava errado → rejeição **válida**. A doutrina NÃO enfraquece o gate bem-observado — ela **veta apenas o gate que não observou**.
- **Nenhum código** é alterado por este ADR.

### 3.3 Riscos / limites

- **`INCONCLUSIVE` pode virar "rejeição de verdade desconfortável"**: se usarmos a doutrina para *sempre* declarar inconclusão quando o resultado é adverso, viramos o auto-engano reverso. O `INCONCLUSIVE` é **honesto**, não **protecionista** — só entra quando o instrumento realmente não observou.
- **Instrumento mal calibrado** continua sendo um bug. A doutrina **não conserta** falha de instrumento; ela **exige que a falha seja sinalizada como tal**, para que o conserto (calibração, dois controles, tri-estado — ver Proveniência) seja prioridade, não mascara.

---

## 4. Alternativas consideradas

| Opção | Veredito |
|---|---|
| **Tratar `FAILURE` como afirmação sobre o mundo** (status quo ingênuo) | ❌ confunde leitura do instrumento com verdade do mundo; reproduz a armadilha do soft-404; arrisca auto-engano reverso. |
| **Descartar o rótulo quando o instrumento falha** (nada registrado) | ❌ perde informação diagnóstica; o estado `BLOCKED` é justamente o dado mais valioso para consertar o instrumento. |
| **Adotar dois eixos separados + matriz de 4 quadrantes** (escolhido) | ✅ distingue verdade-do-mundo de observação-do-instrumento; qualifica o rótulo; preserva o anti-autoengano sem criar o reverso. |
| **Interromper a evolução quando qualquer instrumento reporta falha** | ❌ pior caso: bloqueia promoções válidas por lacuna observacional, sem o conserto do instrumento. |

---

## 5. Teste de aceite (doutrina verificável)

- Lê-se este ADR e sabe-se a regra: *o veredito do instrumento é um estado epistêmico; `FAILURE` só vale quando o instrumento observou com confiança; caso contrário é `INCONCLUSIVE`.*
- Qualquer rótulo de eval novo carrega **dois campos**: resultado **e** estado observacional (`OBSERVED`/`BLOCKED`).
- Em nenhum lugar do design novo, uma lacuna-observacional (timeout, parse falho, ferramenta sem resposta, OCR inválido) é reportada como **erro-de-modelo**.
- A campanha-001 é citada como **prova do mecanismo** (gate observou bem → rejeição válida), não como justificativa para tratar qualquer falha de instrumento como falha de modelo.

---

## 6. Proveniência

- **Padrão de estado epistêmico:** extraído de `github.com/simplifaisoul/osiris` (MIT), `src/lib/sherlock.ts` — **calibração com dois controles** e **tri-estado** (distinguindo "o recurso existe", "não existe", de "não conseguimos dizer"), que é a raiz do "soft-404" citado em §1.
- **Cruzamento com o próprio COSCA:** o resultado da **campanha-001** (LoRA no Qwen3-4B: `pass%` 0.88 base → 0.75 LoRA; regressão detectada pelo Golden Gate/PromotionGate; decisão: manter `qwen3:4b` base). Documentação: `campaign-001/RESULTADO.md` (ou registros de ADR equivalentes da campanha).

---

*Autor: cosca-architecture. Doutrina extraída do padrão de estado epistêmico (osiris/sherlock.ts, MIT), cruzada com a lição do mecanismo anti-autoengano da campanha-001 do COSCA. Escopo: documentação canônica. Nenhum arquivo `.go` funcional foi alterado; nenhuma implementação foi feita.*
