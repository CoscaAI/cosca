# ADR-036: Continuidade Semântica de Capacidades — Capacidade ≠ Provider; percepção é evidência

> **Status:** CANÔNICO (doutrina documentada — o dogma do Don já refletido em código) | **Owner:** cosca-architecture (Architecture Chief) | **Last Updated:** 2026-09-02
> **Natureza deste ADR:** é a **FONTE CANÔNICA** (doutrina arquitetural). **NÃO** cria código novo, **NÃO** altera `.go` funcional, **NÃO** implementa capability registry. Apenas consolida, em um único documento, o dogma que hoje vive implantado em `internal/sensor` + doc comments.
> **Validação (P2 — o código é a verdade):** lido e verificado nos arquivos citados em §2. O que está em código foi confirmado. Onde o código *ainda não* alcança a doutrina, este ADR registra com honestidade (§2.4, §6) — doutrina não é sinônimo de implementação.
> **Referências:** `ADR-017` (cristalizar o que é provado), `ADR-014` (consciência), `ADR-025` (percepção Go-nativa), `ADR-024` (loop de mundos). Doc comments que são a fonte hoje: `internal/sensor/sensor.go`, `internal/sensor/fusion/fusion.go`, `internal/sensor/gate/gate.go`, `internal/screen/screen.go`, `internal/screen/evidence.go`.

---

## 1. Contexto / Problema (por que esta doutrina é necessária)

O COSCA possui uma gama de motores de percepção (OCR, CLIP, detector de regiões, STT, web, métricas estéticas) e uma gama de providers de modelo (openai, anthropic, local, etc.). Sem uma doutrina, o instinto natural é **escolher primeiro o modelo** ("qual LLM/VLM uso para entender esta imagem?") — e isso inverte a hierarquia correta.

O dogma do Don, já refletido em código, corrige essa inversão:

> **CAPACIDADE ≠ PROVIDER.**
> O COSCA não escolhe primeiro o modelo — escolhe primeiro a **capacidade**. A capacidade escolhe os **sensores/providers disponíveis**. Os sensores produzem **evidência**. O kernel **avalia a evidência**. Somente **INSUFICIÊNCIA** ou **CONTRADIÇÃO** justifica **escalação** (VLM/LLM). *Provider é implementação; capability é significado.*

O problema que este ADR resolve é triplo:

1. **Dispersão da doutrina:** o dogma está espalhado em doc comments de 5 arquivos, sem um documento único que o declare como regra canônica. Sem fonte canônica, alguém pode (por engano) "re-construir a doutrina" do zero em outro lugar — **duplicando o conceito** ou **alterando a semântica sem intenção**.
2. **Risco anti-regressão:** sem a regra registrada, é fácil um provider virar "identidade da capacidade" (o exato oposto do dogma), ou um caminho primário ser quebrado por "ampliar" o recurso para um provider.
3. **Fronteira semântica:** co-existem dois usos da palavra "capacidade" no código — os **níveis cognitivos** (`internal/capability`, L0–L3) e a **capacidade semântica** (que este ADR trata). Confundi-los gera design errado.

---

## 2. Decisão / Doutrina

**Adotar** a continuidade semântica de capacidades como doutrina arquitetural. O COSCA percebe por **capacidade** (significado), não por motor; o motor é uma escolha tardia e substituível; a evidência é o único jeito de o kernel confiar.

### 2.1 O caminho canônico (o fluxo que a arquitetura preserva)

```
INTENÇÃO
  → CAPACIDADE                                  (o significado: "entender esta tela", "ler este texto")
    → SENSORES / PROVIDERS DISPONÍVEIS          (o que a capacidade escolhe)
      → OBSERVATIONS                            (DTO normalizado: sensor.Observation)
        → FUSÃO                                 (consenso + contradição)
          → GATE DE ESCALAÇÃO                   (EVIDENCE GATE)
            ├─ suficiente  → RESOLVE LOCAL      (os sensores bastam; NÃO chama VLM)
            └─ insuficiente / contradição → ESCALAÇÃO → VLM/LLM (componente julgável, não oráculo)
```

Regras que este fluxo carrega:

- **VLM é ESCALADA, NUNCA identidade da capacidade.** O VLM entra no topo, depois do gate. Ele é *consultado* quando os sensores locais não bastam; ele não "é" a capacidade.
- **Adicionar provider novo NUNCA altera o caminho primário** (regra anti-regressão, §3.4).
- **Sensor produz EVIDÊNCIA, não verdade.** A classe epistêmica é preservada (`MEASURED`/`INFERRED`/`EVIDENCE`/`DECISION`); **nunca** promoção automática `INFERRED→FACT`.
- **Contradição é informação: preserva, não apaga.** O kernel não escolhe um vencedor silenciosamente — a fusão sinaliza o conflito e o gate escala.

### 2.2 Mapa das peças concretas (P2 — código verificado)

| Peça | Papel | Código (fonte canônica) |
|---|---|---|
| **(1) DTO sensorial normalizado** | O "DNA" que todo sensor emite: `{id, modality, kind, content, confidence, source, epistemic, timestamp, trace_id, region}`. Construtores `New`(MEASURED)/`Inferred`(INFERRED)/`Corroborated`(EVIDENCE); `IsTrustworthy(threshold)`; `Modality` (visual/text/audio/code/web/state); `EpistemicState` (MEASURED/INFERRED/EVIDENCE/DECISION). | `internal/sensor/sensor.go` |
| **(2) FUSÃO + DETECÇÃO DE CONTRADIÇÃO** | Agrupa por **Referent** (identidade) combinando confiança por **log-odds** (não média); separa **Referent** vs **Predicate**; detecta **Contradiction** entre referentes concorrentes no **mesmo Slot**. | `internal/sensor/fusion/fusion.go` |
| **(3) GATE DE ESCALAÇÃO** | Decide `Action` (Resolve/Escalate) com `Reason` (`no_input`/`confident_consensus`/`contradiction`/`low_confidence`/`single_source_weak`). `Policy` (`MinConsensus=0.70`, `ForceEscalateOnContradiction=true`, `MinEvidenceSources=1`). `Decide()` aplica: pedido explícito→escalate; sem input→resolve; contradição→escalate; consenso alto+corroborado→resolve; fonte única fraca→escalate; confiança baixa→escalate. | `internal/sensor/gate/gate.go` |
| **Ponte sensor → DTO** | `Screen.Evidence(source)` converte a percepção da tela em `[]sensor.Observation`. Regra epistêmica: texto **LIDO** pelo OCR → `MEASURED`; região detectada mas **NÃO lida** → `INFERRED`. | `internal/screen/evidence.go` |

**Distinção obrigatória (evita confusão de design):**

> `internal/capability/capability.go` modela **níveis cognitivos** (L0–L3: determinístico / retrieval / raciocínio / autônomo — derivados do provider ativo + runtime). Isso **NÃO é** o mapa capacidade→sensores→escalação. São duas coisas diferentes: um é "qual altura cognitiva o runtime alcança hoje"; o outro é "a capacidade X usa quais sensores e escala para qual VLM". **Este ADR trata do segundo.** Confundi-los é exatamente o erro que a doutrina proíbe.

### 2.3 O que é fonte canônica — e por que NÃO duplicar o conceito

**Fonte canônica = o código + os doc comments inerentes a ele.**

- A **forma** do DTO, a **semântica** da fusão e a **lógica** do gate vivem, de verdade, em `internal/sensor/sensor.go`, `fusion/fusion.go`, `gate/gate.go` (+ `internal/screen/evidence.go` para a ponte). Documentos NÃO devem re-descrever a implementação linha a linha — isso cria **risco de divergência** (o ADR diz uma coisa, o código faz outra).
- O papel deste ADR é **declarar a regra** (o porquê, a inviolabilidade) e **apontar** para as fontes. Ele é uma *bússola*, não o *mapa*. Se o código mudar, a fonte muda; o ADR descreve a doutrina **em vigor**, não um design concorrente.

**Regra anti-duplicação:** não criar um novo "Observation", um novo "fusion", um novo "gate" ou um novo "registry de contradiction" que espelhe o que já existe em `internal/sensor`. Qualquer necessidade deve **consumir** `sensor.Observation` + `fusion.Fuse` + `gate.Policy.Decide`. Duplicar a forma é duplicar o conceito — e a doutrina morre quando há duas versões de "evidência".

### 2.4 Registrar a regra anti-regressão

A anti-regressão é **a invariante que domina a evolução**:

1. **Provider nunca vira capacidade.** Um provider novo (`openai-gpt-x`, `docker-vlm-y`, `local-clip-z`) é um **motor** que a capacidade pode escolher; ele não define o significado. A capacidade "ler texto" permanece "ler texto"; o provider é só um candidato a sensor.
2. **Caminho primário intocável.** `INTENÇÃO → CAPACIDADE → SENSORES → OBS → FUSÃO → GATE` é o caminho determinístico que **sempre** roda primeiro e independente de modelo. Adicionar um provider **não** adiciona um passo ao caminho primário; adicionar é só mais um sensor candidato no leque da capacidade.
3. **Escalação não pode subir "por padrão".** Só sobe para VLM quando o gate diz *insuficiente/contradição* (ou pedido explícito). Nunca "porque o provider existe".
4. **Classe epistêmica não é promovida automaticamente.** A fusão combina e o gate julga; ninguém promove `INFERRED→FACT`. `EVIDENCE` ≠ `FACT` — fato é decisão do kernel, e até `DECISION` é uma classe, não a verdade.

### 2.5 Mapeamento vision → sensores → escalada

Exemplo canônico (o Don já citou): **"visão" é uma capability.**

```
CAPACIDADE: vision ("entender o que está na tela/cena")
  SENSORES DISPONÍVEIS (todos locais degradáveis, best-effort, nunca crash — ADR-025):
    • screen capture          (captura nativa, GDI no Windows)
    • text-region detector    (detectRegions — "aqui provavelmente há texto")
    • WinRT OCR               (OCR nativo do Windows — internal/screen/winrt_ocr.go)
    • tesseract OCR           (Go-nativo — internal/vision)
    • CLIP                    (image-encoder ONNX nativo — internal/worldmodel/vision)
    • aesthetic metrics       (determinísticos — cor, harmonia, contraste, simetria…)
    • compostos futuros       (qualquer sensor novo entra no leque, NUNCA como nova capacidade)
  ESCALAÇÃO (VLM externo):
    • VLM (ex.: um modelo generativo de imagem) → SOMENTE quando o gate diz
      insuficiente/contradição. NUNCA é a identidade da vision.
```

**Propriedade-chave (anti-regressão prática):** se amanhã o COSCA ganhar um re-ranker visual novo, ele vira **mais um sensor** no leque de `vision` — **não** muda o caminho, **não** vira a "vision". E se a `vision` deixar de ter OCR (falha), ela **degrada** (regiões + estética + CLIP) em vez de quebrar — a capacidade sobrevive à ausência de um motor específico.

---

## 3. Consequências

### 3.1 O que muda (aditivo, na direção da doutrina)

- **Nada de código** — este ADR não altera `internal/sensor`, `internal/screen` nem `internal/capability`.
- A documentação passa a ter **uma única fonte canônica** a citar quando o assunto é capacidade semântica; o que hoje é "doc comment espalhado" ganha um porta-voz.

### 3.2 O que NÃO muda (a fundação já está posta)

- `internal/sensor/sensor.go` (DTO) — intocado.
- `internal/sensor/fusion` (fusão/contradição) — intocado.
- `internal/sensor/gate` (gate de escalação) — intocado.
- `internal/screen/evidence.go` (ponte epistêmica) — intocado.
- `internal/screen/screen.go` (pipeline de percepção multimodal) — intocado.

### 3.3 Honestidade: onde o código AINDA NÃO alcança a doutrina (P2 verificado)

Este ADR é doutrina; **não é sinônimo de implementação completa**. Verificado no código:

- **`sensor/fusion` é importado apenas por `internal/sensor/gate/gate.go` e seu teste** (`gate_test.go`). Não há consumidor de produção.
- **`sensor/gate` não é importado por NINGUÉM** no codebase (só os próprios testes). Ou seja: o gate de escalação está **construído e testado, mas NÃO ligado em nenhum pipeline de runtime**.
- **`Screen.Observations` é produzido** (`AnalyzeMultimodal` via `Screen.Evidence("screen")`) **mas o único consumidor da tela** (`internal/cli/screen.go`) **não o lê**: ele lê `res.Regions`, `res.Aesthetic`, `res.VisualEmbedding` diretamente, **bypassando** fusion/gate.
- **`internal/worldloop/worldloop.go` (linha ~147) usa `gate.Decide(results)`** — mas o `gate` ali é **`evalgo.Gate`** (juiz da engine de mundos, ADR-024), **NÃO** o gate sensorial. São "gates" diferentes; a menção a este arquivo como exemplo do gate sensorial em runtime é uma **imprecisão** e deve ser lida como o loop *evalgo*, não como Peça (3) sensorial.

**Leitura honesta:** a "capacidade semântica" está bem **construída em peças** (DTO + fusão + gate + ponte), mas **o caminho canônico inteiro (`INTENÇÃO→…→GATE→ESCALAÇÃO`) não está fechado em runtime** — falta a ligação de `screen.Observations → fusion.Fuse → gate.Decide`, e um destino de escalação efetivo. Isso é exatamente o que a avaliação do capability registry (§6) expõe.

---

## 4. Alternativas consideradas

| Opção | Veredito |
|---|---|
| **Manter o dogma apenas nos doc comments** (status quo) | ❌ doutrina dispersa, sem fonte única; não dá para citar nem evoluir com segurança; regra anti-regressão não é registrada. |
| **Criar um "banco de capacidades" que re-descreva sensor/fusion/gate** | ❌ viola a regra anti-duplicação (§2.3): cria duas fontes de verdade para "evidência". |
| **Transformar `internal/capability` (L0–L3) em o mapa capacidade→sensores→escalação** | ❌ são conceitos distintos; forçar um no outro corrompe ambos (ver §2.2). |
| **Escrever um ADR canônico que declare a doutrina e aponte para a fonte** (escolhido) | ✅ fonte única, sem duplicação, sem alterar código, registra anti-regressão e o fluxo inteiro. |

---

## 5. Teste de aceite (doutrina verificável)

- Lê-se este ADR e sabe-se, sem abrir 5 arquivos, qual é a regra: *capacidade ≠ provider; evidência → fusão → gate → (resolve | escala)*.
- `grep` por `sensor.Observation` em qualquer código novo deve **consumir** `fusion.Fuse`/`gate.Decide`, nunca re-criar a forma.
- Nenhum provider aparece como "nome da capacidade" em design novo (a `internal/capability` L0–L3 é cognitiva, não semântica).
- O doc `internal/screen/evidence.go` continua sendo a única ponte `screen→sensor.Observation`.

---

## 6. Avaliação anexa — capability registry (GAP COMPROVADO, sem implementar)

*Análise textual (veredito + justificativa), nenhum código alterado. Ver também o PRU da tarefa original (não entregue aqui por ser análise).*

**PERGUNTA:** `internal/capability` modela L0–L3, não o mapa capacidade→sensores→escalada. Há **lacuna comprovada** para um registry explícito de capacidade→sensores→escalada? Ou o mapeamento implícito (screen/evidence→sensor→fusion→gate) já é suficiente?

**VEREDITO: GAP COMPROVADO** (com qualificação honesta sobre a natureza do gap).

**Justificativa baseada em código:**

1. **O mapeamento implícito NÃO está conectado em runtime.** `sensor/gate` tem **zero importers** de produção; `sensor/fusion` só é usado por `gate.go` e testes; `cli/screen.go` lê regiões/estética diretamente e **nunca consome** `Screen.Observations`. Logo a cadeia `screen/evidence→sensor→fusion→gate` **não existe como pipeline vivo** — ela é um conjunto de peças testadas, não um caminho operacional. Portanto a alegação "o mapeamento implícito já é suficiente" é **falsa** como afirmação de viabilidade arquitetural: o *caminho* que a doutrina define não é alcançável em runtime hoje.

2. **Não há mapa consultável de capacidade→sensores→escalada.** A capacidade "visão" e seu leque de sensores (capture, text-region, WinRT OCR, tesseract, CLIP, estética) e seu alvo de escalação (VLM) existem como **doc comments** (`screen.go`, `sensor.go`, `gate.go`) e como a "câmera de sensores" hardcoded no pacote `screen`. Não há **registry data-driven** que (a) responda "para a capacidade X, quais sensores + qual alvo de escalação?", (b) permita registrar uma capacidade nova **sem tocar código**, (c) **force estruturalmente** a regra anti-regressão (provider não vira capacidade).

3. **`internal/capability` não preenche o gap** — ele mede nível cognitivo por provider+runtime. É um conceito **ortogonal**. Nada nele diz "vision usa {sensores} e escala para {VLM}". Esta distinção é exatamente a do §2.2.

4. **A natureza do gap é dupla, e a ORDEM importa:** (a) **primeiro** o pipeline sensor→fusion→gate precisa ser **ligado em runtime** (senão o registry seria papel); (b) **depois** o mapa capacidade→sensores→escalada pode virar **registry explícito** para dar descobribilidade + anti-regressão estrutural + adição de capacidades sem tocar código.

**Por que "GAP COMPROVADO" e não "SEM GAP":** a doutrina é **plausível** hoje (todas as peças existem), mas não **vigente** — o caminho canônico não roda, e a regra anti-regressão não tem nenhum ponto estrutural que a faça valer. Isso é uma lacuna comprovada por **código** (importadores ausentes + bypass do `Screen.Observations` + ausência de mapa), não uma preferência de design.

**Nota de escopo:** este ADR **não implementa** nada disso. A doutrina e a avaliação ficam registradas; a implementação (ligar o pipeline + registry explícito) é decisão futura do Don/CTO.

---

*Autor: cosca-architecture. Doutrina fundamentada no dogma do Don + código verificado (P2). Escopo: documentação canônica. Nenhum arquivo `.go` funcional foi alterado; nenhuma implementação foi feita.*
