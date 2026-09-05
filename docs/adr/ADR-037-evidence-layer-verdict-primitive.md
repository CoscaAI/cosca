# ADR-037: Evidência Epistêmica — primitiva `Verdict` na Evidence Layer (não-variável, simétrica)

> **Status:** PROPOSTO (ADOTAR — prioridade máxima) | **Owner:** cosca-architecture (Architecture Chief)
> **Last Updated:** 2026-09-04
> **Natureza deste ADR:** declara a **doutrina epistêmica** da primitiva `Verdict`. **NÃO** cria código em `.go` — é fonte canônica de princípio, análoga ao modo de `ADR-036` (doutrina que apenas aponta para a fonte em código quando ela existir). O núcleo da decisão é de **padrão de verificação**, consumível por `architecture + review/critic`.
> **Mapeamento de adoção:** **architecture + review/critic (núcleo)** — a primitiva entra no **núcleo** (Evidence Layer), não como órgão periférico.
> **Proveniência:** padrão extraído (**princípio**, não código) de `github.com/simplifaisoul/osiris` (licença MIT), arquivo `src/lib/sherlock.ts`. Ver §5.

---

## 1. Contexto / Problema (por que bool é epistemicamente insuficiente)

Agentes do COSCA observam o mundo **via ferramentas/verificação** (web, OCR, sensores, detecção de regiões) e hoje reportam o resultado como **booleanos**: `found/not found`, `pass/fail`. Isso confunde o COSCA de duas maneiras que são **erros de classe**, não ruído:

1. **"Ausência de evidência" ≠ "evidência de ausência".** Quando um instrumento é bloqueado (rate-limit, anti-bot, 401/403/407/429/451/503, soft-404), o agente **não sabe** que algo não existe — ele apenas **não conseguiu observar**. Reportar isso como `not_found` é **afirmar uma inexistência que não foi provada**.
2. **Falsos negativos/positivos sob interferência.** Rate-limit, desafios anti-bot e soft-404 produzem EXATAMENTE os dois erros que um bool não sabe distinguir: um `false` porque o recurso não existe, ou um `false` porque o instrumento foi bloqueado. O bool **achata** a diferença num único bit.

### 1.1 Motivação empírica (campanha-001, fine-tune LoRA)

Na **campanha-001** (fine-tune de LoRA), a métrica de eval **regrediu de 0.88 → 0.75**. O **Golden Gate detectou e reprovou corretamente** — ou seja, o COSCA *funcionou* como sistema de guarda. O ponto epistêmico que este ADR captura é mais sutil:

> **O próprio veredito de eval é uma afirmação epistêmica que pode ser NÃO-confiável.** `regrediu 0.88 → 0.75` requer que o instrumento de medição (o benchmark, o harness, o runner) **tenha observado com confiança**. Se o instrumento estava **BLOCKED** (ex.: um harness que recebeu 429/403 durante a coleta) ou **INCONCLUSIVE** (soft-404: o detector reporta um controle aleatório como presente), então o veredito `FAILURE` é **epistemologicamente inválido** — não se pode afirmar regressão a partir de uma medição que não observou.

Em uma frase: **o COSCA precisa distinguir "o mundo disse", "o instrumento disse", e "o instrumento não conseguiu dizer".** Hoje o bool colapsa as três em duas.

---

## 2. Decisão — primitiva `Verdict` de primeira classe na Evidence Layer

**Adotar** uma primitive **`Verdict`** com **6 estados**, substituindo o bool como resultado canônico de observação/verificação:

| Estado | Significado epistêmico | Exemplo típico | **É ausência?** |
|---|---|---|---|
| `FOUND` | O referente **existe/funcionou** e foi **observado com confiança**. | HTML contém o elemento esperado; endpoint respondeu 200. | Não |
| `NOT_FOUND` | **Ausência PROVADA** — verificamos, e a ausência foi **demonstrada**. | Controle negativo confirmado; busca exaustiva sem resultado. | **Sim (provada)** |
| `BLOCKED` | **Desconhecido** — o instrumento **recusou** observar (401/403/407/429/451/503). **NÃO é ausência.** | Rate-limit, anti-bot, auth negada, serviço indisponível. | **Não** — é *incognoscível* agora |
| `INCONCLUSIVE` | **Não-confiável** — o instrumento reportou, mas sua própria calibração está **suspeita** (soft-404: o detector **também** afirma um controle aleatório como presente). | Detector de presença dispara em controle negativo (falso positivo emergente). | **Não** — o sinal é *ruído* |
| `SKIPPED` | A observação **não foi tentada** (fora de escopo, galho sem relevância). | Etapa de uma análise que não se aplica. | **Não** — não houve observação |
| `ERROR` | Falha com **motivo explícito** (nunca implícito). Erro de infraestrutura/tool, exceção capturada. | Timeout de conexão, erro de parse, exceção. | **Não** — a observação quebrou |

Regras invariantes que o `Verdict` carrega:

1. **`BLOCKED` e `INCONCLUSIVE` NUNCA colapsam para `NOT_FOUND`.** São classes distintas. Reduzi-los a bool é reintroduzir o vício que este ADR proíbe.
2. **`NOT_FOUND` é uma afirmação forte** (exigiu PROVA de ausência). Não pode ser fabricado de uma observação fracassada.
3. **`ERROR` carrega motivo explícito** — nunca implícito. Se quebrou, o sistema sabe *por quê*.

### 2.1 Simetria do `Verdict` (a primitiva é única e simétrica)

O `Verdict` é **simétrico**: aplica-se ao **modelo observando o mundo** E ao **avaliador observando o modelo**. Não é um conceito de "ferramenta de coleta" — é um conceito de **conhecimento do que sabemos**. A mesma primitiva qualifica:

- **Observação → mundo:** "o recurso X existe?" → `Verdict` do instrumento de verificação.
- **Avaliação → modelo:** "o modelo Y acertou?" → `Verdict` do avaliador/benchmark sobre o modelo.

Isso torna o `Verdict` um **tipo de primeira classe da Evidence Layer**: ele é a moeda de troca entre camadas, não um detalhe de implementação de uma ferramenta.

### 2.2 Princípio de calibração — DOIS controles independentes

Para um `Verdict` ser confiável, sua calibragem precisa ser **validada por DOIS controles independentes** (não apenas um):

> Um **único** controle deixa o **soft-404** escapar toda vez que esse controle é rejeitado. Exemplo concreto: **Roblox e WordPress** — um veredito que passou num e caiu no outro evidencia que a calibração depende do instrumento, não do mundo. Para `BLOCKED`/`INCONCLUSIVE` serem detectados, a calibração tem que ser **bifurcada**: dois controles que **co-variam com o mundo** (para detectar ausência real) e, ao mesmo tempo, **divergem entre si** (para detectar quando o instrumento mente).

Consequência prática para o COSCA: um detector de presença/verificação **não** pode se auto-calibrar com um único controle. Ele precisa de um **controle positivo** (ou controle negativo) **independente**, de forma que a concordância/discordância entre os dois revele o estado observacional (`BLOCKED`/`INCONCLUSIVE`) — não apenas o estado do mundo (`FOUND`/`NOT_FOUND`).

---

## 3. Matriz dos quadrantes (o eixo exige os DOIS eixos)

A leitura de um veredito é feita em **duas dimensões independentes** — é isso que o bool perde:

| Eixo | Direção | Dimensão | Exemplo |
|---|---|---|---|
| **Eixo A — verdade do mundo** | *Existe? Funcionou?* | o que é verdade **sobre o referente** | o recurso existe, o modelo acertou |
| **Eixo B — observação do instrumento** | *Conseguimos observar com confiança?* | o que é verdade **sobre a medição** | o instrumento estava saudável, calibrado, não-bloqueado |

Os quatro quadrantes:

```
                      EIXO B: INSTRUMENTO CONSEGUIU OBSERVAR?
                              CONFIANÇA       |       NÃO-CONFIANÇA
                              (observou prós) |  (bloqueado / inconclusivo)
        ┌─────────────────────────────────────┼──────────────────────────────┐
  EIXO A│   FOUND      (existe + vi)          │  BLOCKED / INCONCLUSIVE      │
  VERDADE│   NOT_FOUND (não existe + vi)      │  → o MUNDO é incognoscível   │
  O MUNDO│                                    │    agora; NÃO é "não existe"  │
  EXISTE?│                                    │    nem "existe"               │
        └─────────────────────────────────────┴──────────────────────────────┘
```

**A regra central deste ADR (vale para AMBOS os lados da simetria):**

> **Um veredito de `FAILURE`/`FALSE_COMPLETION` (da avaliação) quando o instrumento estava `BLOCKED`/`INCONCLUSIVE` (da observação) é EPISTEMOLOGICAMENTE INVÁLIDO.**

Ou seja: **o mesmo vício de `403 != not_found`, aplicado à avaliação.** Da mesma forma que `BLOCKED` não pode ser lido como `NOT_FOUND`, o veredito de eval **não pode ser lido como verdade de mundo** quando o instrumento que mediu estava bloqueado/inconclusivo. Um rótulo `FAILURE` de eval **precisa** ser qualificado pelo estado observacional do instrumento — senão o COSCA "aprende" regressões que nunca ocorreram, e reprova modelos por causa de um harness que mentiu.

---

## 4. Consequências

### 4.1 O que muda (aditivo, direção da doutrina)

- **Ferramentas de verificação e agentes passam a expor `Verdict`** (não `bool`) como resultado de observação. `found/not_found` e `pass/fail` deixam de ser o contrato canônico em favor dos 6 estados.
- **O review/critic e o Golden Gate passam a CONSUMIR `Verdict`**: quando jutarem um resultado de eval, eles exigem o estado observacional do instrumento. `FAILURE` sem qualificador observacional perde o direito de ser usado como evidência de reprovação.
- **Um rótulo de eval (`FAILURE`/`FALSE_COMPLETION`) precisa ser qualificado pelo estado do instrumento** (`OBSERVED` vs `BLOCKED`/`INCONCLUSIVE`) antes de ser aceito como `EVIDENCE`. Isso dá ao Golden Gate a capacidade de distinguir "o modelo regrediu" de "o instrumento não conseguiu medir".

### 4.2 O que NÃO muda

- A **autoridade de decisão** permanece do kernel/gate (I1). O `Verdict` **informa**; ele **não substitui** o gate — é a classe epistêmica que o gate consome.
- A **separação entre verdade e medição** já é a fundação do COSCA (ADR-025 percepção Go-nativa, ADR-036 capacidade semântica, ADR-022 proof). Este ADR **integra** essa separação no contrato de verificação, não a cria do zero.
- **Nenhum `.go` funcional** é alterado por este ADR. Ele declara a doutrina; a implementação (ligar os 6 estados nos instrumentos e no evaluator) é decisão subsequente.

### 4.3 Trade-offs / riscos

- **Mais estados, mais complexidade de contrato.** O `Verdict` é mais caro que um bool — exige que cada instrumento saiba distinguir `BLOCKED` de `NOT_FOUND`, e que cada avaliador preserve o estado observacional. É o preço de não mentir.
- **Risco de degenerar para `UNKNOWN` genérico.** Se `BLOCKED`/`INCONCLUSIVE` forem tratados como "não sei", perdem o poder de distinguir *recusa de observação* vs *observação não-calibrada*. `SKIPPED` vs `BLOCKED` vs `INCONCLUSIVE` devem permanecer **classes separadas**, nunca colapsadas.
- **Custo de calibração dupla.** A regra dos dois controles independentes (Roblox/WordPress) exige infraestrutura de controle — mas é o único jeito de detectar soft-404 de forma **estrutural** (não heurística).

---

## 5. Proveniência

**Padrão extraído (princípio, NÃO código)** do repositório **`github.com/simplifaisoul/osiris`** (licença **MIT**), arquivo **`src/lib/sherlock.ts`**.

O que foi levado como **princípio** (não como código):

- A distinção entre **estado do mundo** e **estado da observação** — a ideia de que um veredito de ausência só é válido quando o instrumento **provou** a ausência, e que recusa/inconclusão do instrumento são **classes próprias**, não `not_found`.
- A **simetria**: a mesma primitiva de julgamento vale para o observador do mundo e para o avaliador do modelo — ambos reportam um `Verdict`, não uma "verdade".
- A **calibração bifurcada por controles independentes** para detectar quando o instrumento mente (soft-404 / falso positivo do detector).

O que **NÃO** foi levado:

- **Nenhum código, tipo, função ou estrutura do `sherlock.ts`.** Nenhum import, nenhuma assinatura, nenhum enum/union real. A síntese em **6 estados** (`FOUND`/`NOT_FOUND`/`BLOCKED`/`INCONCLUSIVE`/`SKIPPED`/`ERROR`) e a **matriz dos quadrantes** são a *formalização COSCA* do princípio, não a reprodução do arquivo-fonte.
- A **pilha/implementação** do repositório (stack TS/JS, estrutura de arquivos, APIs específicas) — **REJEITADA** a título de estilo, seguindo ADR-036/022 (selecionamos o minério, nunca a pilha).

---

## 6. Teste de aceite (doutrina verificável)

- Lê-se este ADR e sabe-se a regra: **`BLOCKED`/`INCONCLUSIVE` ≠ `NOT_FOUND`; `FAILURE` de eval sem estado observacional é inválido; calibração exige dois controles independentes.**
- Nenhuma ferramenta de verificação nova reporta apenas `bool` (found/not_found, pass/fail) como resultado canônico — reporta `Verdict`.
- O review/critic e o Golden Gate, ao lerem um resultado de eval, exigem/conservam o **estado observacional** do instrumento antes de emitir um veredito de aprovação/reprovação.
- `grep` por `not_found`/`fail` em código novo de verificação NÃO deve ser tratado como ausência real quando o estado observacional for `BLOCKED`/`INCONCLUSIVE`.

---

## 7. Alternativas consideradas

| Alternativa | Veredito |
|---|---|
| **Manter `bool` (status quo)** | ❌ Conflaciona "ausência de evidência" com "evidência de ausência"; produz falsos negativos/positivos sob rate-limit/anti-bot/soft-404; impede o Golden Gate de distinguir regressão real de medição quebrada. |
| **Usar `UNKNOWN`/`nil` como terceiro estado (`tri-state`)** | ⚠️ Insuficiente: colapsa `BLOCKED` (recusa do instrumento) com `INCONCLUSIVE` (medição não-calibrada) e perde o motivo explícito do `ERROR`. Não separa "não sei porque não consegui" de "não sei porque o detector mente". |
| **Tratar itens não-encontrados como `NOT_FOUND` mas registrar `warn` separado** | ⚠️ Heurístico e não-estrutural: a informação de que o instrumento estava bloqueado vive num log paralelo, não no contrato; o avaliador não é *forçado* a consumi-la. |
| **`Verdict` de 6 estados como primitiva de primeira classe da Evidence Layer (ESCOLHIDO)** | ✅ Separa verdade do mundo × confiança do instrumento de forma **estrutural**; é simétrica (observador do mundo = avaliador do modelo); dá ao Golden Gate um contrato para não reprovar baseado em medição quebrada. |

---

*Autor: cosca-architecture. Padrão extraído (princípio) de osiris (`src/lib/sherlock.ts`, MIT); formalização COSCA (6 estados + matriz dos quadrantes) — nenhum código do repositório-fonte importado. Escopo: documentação canônica. Nenhum arquivo `.go` funcional alterado; nenhuma implementação feita. Adoção: ADOTAR (prioridade máxima) — architecture + review/critic (núcleo).*
