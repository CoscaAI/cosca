# ADR-042: GET condicional (ETag/Last-Modified) + helper `optional()` — polling eficiente e enriquecimento opcional resiliente

> **Status:** PROPOSTO (ADOÇÃO: ADOTAR) | **Owner:** cosca-architecture (Architecture Chief)
> **Last Updated:** 2026-09-04
> **Natureza deste ADR:** declara a **doutrina de GET condicional + enriquecimento opcional resiliente**. **NÃO** cria código — é fonte canônica de princípio, análoga ao modo de `ADR-039`/`ADR-040` (doutrina que aponta para a fonte em código quando ela existir). O núcleo da decisão é um **padrão de fetch de fonte/compartilhado** (wrapper), consumível por `architecture + integrations chief + runtime chief + platform`.
> **Mapeamento de adoção:** **integrations + runtime (implementação)** — o helper entra na camada de integração/consumo de fontes e compartilhados, não como órgão periférico.
> **Proveniência:** padrão extraído (**princípio**, não código) de `github.com/simplifaisoul/osiris` (licença MIT), arquivo `src/lib/httpJson.ts`. Ver §7.

---

## 1. Contexto / Problema (por que re-baixar um dump byte-idêntico é o caminho caro de aprender nada)

O COSCA faz **polling/consumo de dumps e datasets** (fontes e compartilhados — índice de ~500KB, dumps multi-megabyte, datasets de conhecimento, web/compartilhados) que **mudam raramente**. Nesses caminhos existe um problema de **classe** (não ruído) que um fetch ingênuo não resolve:

1. **Re-baixar o que é byte-idêntico.** Num poll rápido, o sistema re-busca o **dump inteiro** apenas para descobrir que ele não mudou. Um dataset multi-megabyte que muda raramente é re-baixado por completo (banda + tempo + parse) para **aprender zero** — o caso comum vira o caso caro. **Re-baixar um multi-megabyte num poll rápido para descobrir que é byte-idêntico é o caminho caro de aprender nada.**
2. **Decode de corpo quebrado por suposição.** Alguns hosts servem **JSON pré-comprimido** e setam `Content-Encoding` **independente** do que foi pedido no `Accept-Encoding`. Decodificar **pelo que foi pedido** e não **pelo header de resposta** quebra o corpo — o fetch retorna lixo em vez do dado.
3. **UA genérico atrai rejeição legítima da fonte.** Endpoints **OSM** (Nominatim/Overpass) **REJEITAM browser-UA** com **406/429** e pedem contato. Um `User-Agent` identificador **honesto** (`OSIRIS-OSINT/1.0 (+url)`) é o que a fonte quer ver — e o que o COSCA **deve** enviar. O caminho errado é fingir ser navegador para "passar".
4. **Enriquecimento opcional frágil.** Chamadas de **enriquecimento de baixa prioridade** (dado complementar, não crítico) podem falhar por qualquer motivo transitório (timeout, 5xx, rate-limit, fonte fora). Se essa falha **estourar exceção**, ela **derruba o pipeline principal** — um enriquecimento opcional que **nunca** deveria matar o chamador.

> **Problema em uma frase:** tornar o **caso comum** de polling de dump que mudou pouco/mudou nada **barato** (header em vez de corpo), **decodificar** o corpo corretamente **pelo header de resposta**, **identificar-se honestamente** para não ser rejeitado pela fonte, e envolver **enriquecimento opcional** em um wrapper que **nunca** derruba o pipeline principal.

---

## 2. Decisão — GET condicional + helper `optional()` (fetch de fonte/compartilhado)

**Adotar**, no fetch de fontes/compartilhados, **4 comportamentos combinados** que tornam o polling de dumps que mudam raro *barato por natureza* e o enriquecimento *resiliente por construção*:

### 2.1 GET condicional — replay de validators (`httpConditional`)

- **Replay de `ETag`/`Last-Modified`** no poll seguinte, via **`If-None-Match`** / **`If-Modified-Since`**.
- **O caso comum vira centenas de bytes de header:** quando o upstream responde **`304 Not Modified`**, o corpo é vazio — o sistema aprendeu que **nada mudou** pagando só o custo de header.
- **`304` é resposta VÁLIDA, não erro.** Semântica: `changed:false`, `body:null` — o sistema **não** trata `304` como falha nem como resposta corrompida.
- **Degradação honesta:** sem validators disponíveis (primeira chamada, ou upstream que ignora) degrada para **GET comum** — com **grace** (nunca trata como erro; segue o fluxo normal).
- **Guardar os validators retornados** para o próximo poll — o estado condicional é **persistido entre polls**, é a base de todo o ganho.

### 2.2 Decode de `content-encoding` PELO HEADER (não pelo que foi pedido)

- O corpo é decodificado **pelo `Content-Encoding` do header de resposta** (`gzip`/`deflate`/`br`), **não** pelo que foi solicitado no `Accept-Encoding`.
- **Por quê:** alguns hosts servem **JSON pré-comprimido** e setam `Content-Encoding` **independente** do `Accept`. Decodificar por suposição quebra o corpo; decodificar pelo header é a única interpretação **fiel** da resposta.
- **Objetivo:** o fetch nunca retorna **lixo por má-suposição** — a leitura do corpo é sempre a leitura do que o servidor **realmente** enviou.

### 2.3 UA identificador HONESTO (identifier) — nunca spoofing

- `User-Agent` **identificador** claro (ex. `OSIRIS-OSINT/1.0 (+url)`), que declara quem é — **não** um browser-UA genérico de disfarce.
- **Por quê:** endpoints **OSM** (Nominatim/Overpass) **REJEITAM browser-UA** com **406/429** e pedem contato. **Honestidade > spoofing:** o sistema se apresenta, respeita a fonte e colhe o que ela oferece a quem se identifica.
- **Nota axial (contra-tendência):** este é o **oposto** de qualquer **IP/UA-spoofing** para "furar" bloqueio. O **COSCA rejeita** qualquer forma de **disfarce de identidade** — isto viola a **Lei do Cofre / LEALDADE** (ADR-022). O caminho é **identificar-se**, nunca **fingir ser outro**.

### 2.4 Helper `optional()` — enriquecimento opcional que nunca derruba o chamador

- **`optional()` resolve `null`** em vez de **estourar exceção**, para **enriquecimento opcional** que pode falhar **sem derrubar o chamador**.
- **Por quê:** um enriquecimento de **baixa prioridade** (dado complementar) **nunca** deve matar o **pipeline principal**. A falha opcional é **capturada e neutralizada** (vira `null`), permitindo ao chamador seguir com o que já tem.
- **Objetivo:** a robustez do **enriquecimento** é **localizada** — o que é opcional não se propaga como erro fatal.

---

## 3. Por que é valioso para o COSCA

- **Economia massiva em polling de dumps grandes/compartilhados.** O `304` transforma o **caso comum** (nada mudou) em **centenas de bytes de header** em vez de re-baixar um multi-megabyte byte-idêntico — tempo, banda e parse cortados, com ganho de aprendizado **idêntico** (zero).
- **Enriquecimento opcional degrada graceful.** Um enriquecimento de baixa prioridade que falha resultado em `null`, **nunca** derruba o pipeline — o sistema **continua produzindo** sem cegar nem travar.
- **Conteúdo sempre fiel à resposta real.** Decodificar pelo header garante que o corpo **nunca** é `lixo por má-suposição` — mesmo quando o servidor serve dados pré-comprimidos fora do `Accept`.
- **Honestidade estrutural com fontes exigentes.** O UA identificador **abre** a porta que o browser-UA fecha: o COSCA **é aceito** por fontes OSM/Nominatim/Overpass **porque se identifica** — e preserva a **Lei do Cofre/LEALDADE** ao **nunca** fingir identidade.
- **Ressonância com a doutrina de carga resiliente (ADR-039):** o `304` é o **par natural** do TTL/single-flight — um reduz o **custo por request validado**, o outro reduz o **número de requests**. Juntos, o polling de fontes compartilhadas fica **barato por natureza**.

---

## 4. Consequências

### 4.1 O que muda (aditivo)

- **O fetch de fontes/compartilhados passa a ser condicional:** o sistema **replaya** validators (`If-None-Match`/`If-Modified-Since`) e interpreta **`304`** como `changed:false`/`body:null` — **não** como erro.
- **Reconhecimento do `304` como resposta válida (não erro):** o caso comum (nada mudou) vira barato; o estado condicional é **persistido** entre polls.
- **Decode por header de resposta:** `Content-Encoding` lido **da resposta**, levando a **conteúdo fiel** mesmo para JSON pré-comprimido.
- **UA honesto vira a política padrão:** todo fetch levanta um **identificador** claro — a adoção é estrutural, não por-fonte.
- **Enriquecimento opcional via `optional()`:** o que é complementar degrada para `null`, **nunca** exceção — o pipeline principal não é interrompido por dado não-crítico.
- **Degradação honesta sem validators:** primeira chamada/upstream que ignora → GET comum com grace (sem erro).

### 4.2 O que NÃO muda (invariantes)

- **Nenhum mecanismo de evasão é aceito.** A doutrina **rejeita explicitamente** IP-spoofing, rotação de UA não-identificador e disfarce de identidade para contornar bloqueio/rate-limit. **Identificar-se > fingir ser outro** — a **Lei do Cofre / LEALDADE** (ADR-022) permanece lei superior.
- A **autoridade de decisão** permanece do kernel/gate (I1) — o wrapper **informa** (`changed:false`, `body:null`, `null` de `optional()`), não decide.
- A **semântica de dados** do upstream — o wrapper **não altera** o que a fonte retorna; apenas controla **quando/quanto** ela é tocada e **como** o corpo é lido e o erro opcional é neutralizado.
- **Nenhum código funcional** é alterado por este ADR. Ele declara a doutrina; a implementação (ligar condicional + `optional()` no fetch de fontes/compartilhados na camada de integração/runtime) é decisão subsequente de **integrations + runtime**.

### 4.3 Trade-offs / riscos

- **Complexidade de decodificação por múltiplos encodings** (`gzip`/`deflate`/`br`): o decode precisa cobrir os encodings que a fonte pode real-utilizar — erro aqui produz corpo corrompido. O decode **pelo header** é a única defesa correta.
- **Dependência da cooperação do upstream com `ETag`/`Last-Modified`:** a adoção assume que a fonte expõe validators. Upstream que **não** os fornece degrada para GET comum (com grace) — o ganho é **por-fonte**, não garantido globalmente.
- **Falso `304` por cache intermediário:** se um intermediário devolver `304` indevidamente (por validator mal-persistido), o sistema pode saltar uma mudança real. Mitigação: manter o estado condicional **fiel**, e aceitar que a janela de mudança é coberta pelo TTL/stale (ADR-039).
- **UA honesto pode ter políticas distintas por fonte:** fontes OSM podem **exigir contato/cadastro** — o UA honesto é pré-requisito, mas **não** garante acesso; a política de **backoff/rate_limit** (ADR-040) segue sendo a parceira de moderação.
- **`optional()` pode mascarar erro real:** tragar **toda** exceção vira `null` pode esconder um bug de programação (não só falha transitória). Mitigação: `optional()` é **para enriquecimento opcional**, não para o caminho crítico — o que é obrigatório não passa por `optional()`.

---

## 5. Alternativas consideradas

| Alternativa | Veredito |
|---|---|
| **Fetch direto sem condicional (status quo)** | ❌ Re-baixa o dump inteiro a cada poll para aprender que é byte-idêntico — pagar o corpo multi-megabyte para aprender nada é o caminho caro. Tempo, banda e parse desperdiçados no caso comum. |
| **Só decode por `Accept-Encoding` (o que foi pedido)** | ❌ Quebra o corpo quando o host serve JSON pré-comprimido e seta `Content-Encoding` independente do `Accept` — o fetch retorna lixo por má-suposição. |
| **UA browser-genérico / rotação de UA para "passar"** | ❌ **REJEITADO — viola a Lei do Cofre/LEALDADE** (ADR-022). Disfarce de identidade é desonestidade estrutural; e fontes OSM **rejeitam** browser-UA mesmo assim (406/429). |
| **Fetch com exceção propagada no enriquecimento (deixar o chamador tratar)** | ⚠️ Empurra o problema para cima: um enriquecimento opcional que estoura exceção **derruba o pipeline principal** — o não-crítico mata o caminho crítico. |
| **GET condicional (ETag/Last-Modified) + decode por header + UA honesto + `optional()` (ESTE ADR)** | ✅ Combina os 4 comportamentos: o caso comum vira **barato via `304`** (header em vez de corpo); o corpo é lido **fielmente** (decode pelo header); o UA honesto **abre** fontes exigentes sem evasão; e o enriquecimento opcional **degrade para `null`**, nunca derrubando o pipeline. |

---

## 6. Teste de aceite (doutrina verificável)

- **GET condicional:** um poll subsequente do mesmo dump replay a `If-None-Match`/`If-Modified-Since` e, diante de **`304`**, retorna `changed:false`/`body:null` **sem re-baixar o corpo** — verificável por contador de bytes transferidos (custo do caso comum ≈ centenas de bytes de header).
- **`304` como resposta válida, não erro:** o sistema **não** registra `304` como falha; classifica como `changed:false` e segue o fluxo normal.
- **Degradação sem validators:** primeira chamada (ou upstream que não expõe `ETag`/`Last-Modified`) degrada para **GET comum com grace** — sem erro, sem quebra.
- **Decode por header:** um host que serve JSON **pré-comprimido** com `Content-Encoding` independente do `Accept` é decodificado **pelo header de resposta** — o corpo **não** sai como lixo.
- **UA honesto:** o fetch envia **identificador** claro (ex. `OSIRIS-OSINT/1.0 (+url)`), não browser-UA genérico — e é **aceito** por fonte OSM (`Nomimatin/Overpass`) que rejeitaria browser-UA (406/429).
- **`optional()`:** um enriquecimento de baixa prioridade que **falha** (timeout/5xx/rate-limit) resolve `null` **sem estourar exceção** — o pipeline principal **continua** seu fluxo.
- **`grep` por spoofing:** **nenhum** mecanismo de **IP-spoofing ou rotação de UA não-identificador** para contornar bloqueio/rate-limit — a doutrina proíbe evasão; o sistema se **identifica**, nunca **se disfarça**.

---

## 7. Proveniência

**Padrão extraído (princípio, NÃO código)** do repositório **`github.com/simplifaisoul/osiris`** (licença **MIT**), arquivo **`src/lib/httpJson.ts`**.

O que foi levado como **princípio** (não como código):

- **GET condicional (`httpConditional`)** — replay de `ETag`/`Last-Modified` (via `If-None-Match`/`If-Modified-Since`) em polling de dumps grandes que mudam raro; **`304` com corpo vazio** vira o caso comum barato; **`304` é resposta válida (não erro)**, com `changed:false`/`body:null`; sem validators degrada para GET comum com grace; validators guardados para o próximo poll.
- **Decode de `Content-Encoding` pelo HEADER** — (`gzip`/`deflate`/`br`) lido da **resposta**, não do que foi pedido — porque alguns hosts servem JSON pré-comprimido e setam `Content-Encoding` independente do `Accept`.
- **UA identificador HONESTO** (ex. `OSIRIS-OSINT/1.0 (+url)`) — porque endpoints OSM (Nomimatin/Overpass) **rejeitam browser-UA** com 406/429 e pedem contato. **Honestidade > spoofing** — a contra-tendência a preservar.
- **`optional()`** — resolve `null` em vez de **estourar exceção**, para **enriquecimento opcional** que pode falhar **sem derrubar o chamador** (enriquecimento de baixa prioridade nunca mata o pipeline principal).

O que **NÃO** foi levado:

- **Nenhum código, tipo, função ou estrutura do `httpJson.ts`.** Nenhum import, nenhuma assinatura, nenhuma classe/função real. A síntese em **4 comportamentos** (`httpConditional` + decode por header + UA honesto + `optional()`) e a **formalização COSCA** desses princípios são a síntese canônica, não a reprodução do arquivo-fonte.
- A **pilha/implementação** do repositório (stack TS/JS, estrutura de arquivos, APIs específicas) — **REJEITADA** a título de estilo, seguindo ADR-036/022 (selecionamos o minério, nunca a pilha).
- **Qualquer mecanismo de evasão** (IP-spoofing, rotação de UA não-identificador, bypass anti-bot) como forma de "resolver" rejeição/rate-limit — **REJEITADO** como princípio, pois viola a Lei do Cofre/LEALDADE (ADR-022).

---

*Autor: cosca-architecture. Padrão extraído (princípio) de osiris (`src/lib/httpJson.ts`, MIT) — GET condicional (ETag/Last-Modified), decode de content-encoding pelo header, UA identificador honesto e helper `optional()`; formalização COSCA dos 4 comportamentos combinados — nenhum código do repositório-fonte importado. Escopo: documentação canônica. Nenhum arquivo funcional alterado; nenhuma implementação feita. Adoção: ADOTAR — integrations + runtime (implementação).*
