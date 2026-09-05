# ADR-040: Concorrência como Propriedade do Sistema — `ConcurrencyPolicy` (global/per-provider + rate_limit + burst + backoff)

> **Status:** PROPOSTO (ADOÇÃO: ADOTAR) | **Owner:** cosca-architecture (Architecture Chief)
> **Last Updated:** 2026-09-04
> **Natureza deste ADR:** declara a **doutrina de concorrência como propriedade do sistema**. **NÃO** cria código — é fonte canônica de princípio, análoga ao modo de `ADR-039`/`ADR-037` (doutrina que aponta para a fonte em código quando ela existir). O núcleo da decisão é um **padrão de controle de concorrência** (policy), consumível por `architecture + workflow chief + platform`.
> **Mapeamento de adoção:** **workflow chief + platform (implementação)** — a policy entra na camada de orquestração/fan-out, não como órgão periférico.
> **Proveniência:** padrão extraído (**princípio**, não código) de `github.com/simplifaisoul/osiris` (licença MIT), arquivo `src/lib/sherlock.ts` (mapLimit + calibração de concorrência) e `src/lib/httpJson.ts` (UA honesto — contra-tendência a preservar). Ver §7.

---

## 1. Contexto / Problema (por que concorrência é restrição externa, não detalhe de implementação)

Agentes do COSCA fazem **fan-out/múltiplas chamadas em paralelo** a fontes e provedores (conhecimento, semântico, web, datasets, orquestração multi-fonte). Nesses caminhos existem dois problemas de **classe** (não ruído) que um pool de concorrência ingênuo não resolve:

1. **Estouro do pool de conexões.** Um fan-out desenfreado (N chamadas simultâneas) pode estourar o pool de conexões do cliente/upstream. Um **pool de concorrência limitada** é necessário para manter o sistema dentro do que o transporte aguenta — é um **borne operacional**, não uma preferência.
2. **Auto-rate-limit por excesso de fanout — LIÇÃO EMPÍRICA (osiris/sherlock.ts).** Ao calibrar a concorrência do scanner, a equipe do osiris testou **20** e o próprio scanner **disparava os rate-limiters das fontes** — o excesso de fanout causava **auto-rate-limit**. Reduziram para **12**, e aí estabilizou. Isto é uma lição de **classe**: o limite não é um "número bonito" — é a fronteira **entre explorar a fonte e acordar o seu rate-limiter**.

> **Insight axial:** concorrência **NÃO** é um detalhe de implementação do worker. É uma **RESTRIÇÃO EXTERNA LEGÍTIMA** imposta pelo provedor/fonte. Um número fixo como **12** é **ótimo para uma fonte e péssimo para outra** — cada provedor tem limites diferentes (alguns aceitam mais, outros menos). Fixar um valor único para todos é tratar fontes heterogêneas como homogêneas.

> **Problema em uma frase:** permitir que a concorrência seja **declarada por provedor** (não um número mágico global fixo), com um **teto global** de segurança, **janela de taxa**, **rajada** e **recuo** — e, quando o rate-limit vier, **adaptar a estratégia** em vez de tentar falsificar identidade para contorná-lo.

---

## 2. Decisão — `ConcurrencyPolicy` como recurso de primeira classe do sistema

**Adotar** a **`ConcurrencyPolicy`** como **conceito de primeira classe** — concorrência é uma **PROPRIEDADE DO SISTEMA**, **declarada por provedor**, e **NÃO** um número fixo embutido no worker. A política modela quatro parâmetros:

### 2.1 Teto global (`max_global`)

- **Limite do sistema como um todo** — o fan-out agregado, independentemente de quantos provedores estejam sendo tocados, **nunca** ultrapassa `max_global`.
- **Objetivo:** segurar o **borne do pool de conexões** (o sistema inteiro nunca abre mais conexões concorrentes do que o transporte aguenta).
- É a **rede de segurança final** — mesmo que N provedores individuais estejam abaixo do seu teto, a soma nunca viola o teto do sistema.

### 2.2 Teto por fonte/provedor (`max_per_provider`)

- **Limite por fonte/provedor** — uma fonte específica é tocada com, no máximo, `max_per_provider` chamadas concorrentes.
- **A calibração é por fonte, não global.** É aqui que a lição empírica do osiris entra: **12** pode ser ótimo para uma fonte e péssimo para outra. O valor-base é **declarado por provedor**, não uma **constante mágica** global.
- **Objetivo:** cada provedor recebe o teto que **ele** suporta — o que explora a fonte perto da fronteira de rate-limit **sem acordá-la**.

### 2.3 Janela de taxa (`rate_limit`) e rajada (`burst`)

- **`rate_limit` = janela de taxa** — quantas requisições por janela de tempo a fonte aceita (a cadência sustentada do provedor).
- **`burst` = rajada** — o pico momentâneo permitido acima da taxa sustentada (a tolerância de `burst` que o provedor dá antes de recusar).
- **Objetivo:** modelar o **contrato de cadência real** do provedor (taxa sustentada + rajada), e não uma única constante "não chame demais".

### 2.4 Recuo (`backoff`)

- **`backoff` = recuo exponencial + jitter** — quando o rate-limit **vier** (429/503), o sistema **recua exponencialmente**, com **jitter** para não re-sincronizar todos os workers numa mesma "onda" de retry.
- **Objetivo:** após a fonte dizer "chega", o sistema **se retira** e **retorna com moderação**, nunca martelando a fonte num retry sincronizado.

### 2.5 A base é ajustável por fonte, com teto global e backoff

- Estrutura da política: **valor-base declarado por provedor** (não constante mágica), **teto global** (`max_global`) como borne do sistema, e **backoff** como resposta a recusa.
- **Resumo da hierarquia:** `max_per_provider` ≤ controle fino por fonte; `max_global` ≤ borne do sistema; `rate_limit`/`burst` ≤ contrato de cadência; `backoff` ≤ moderação após recusa.

### 2.6 PRINCÍPIO AXIAL — rate-limit é restrição externa legítima; adaptar, nunca evadir

> **Rate-limit é uma RESTRIÇÃO EXTERNA LEGÍTIMA** do provedor. O sistema deve **ADAPTAR sua estratégia** (política/backoff/religar em outro momento) — **NÃO falsificar identidade para contornar**.

- **REJEITADO** qualquer padrão tipo **IP-spoofing**, rotação de UA para disfarçar identidade, ou evasão de anti-bot para furar o rate-limit. Isso viola a **Lei do Cofre / LEALDADE** (ADR-022).
- **Caminho certo:** **Honestidade + respeitar o rate-limit + adaptar a estratégia.** O sistema **declara quem é**, **respeita o teto** que a fonte impõe, e **muda a estratégia** (backoff, religar depois, reduzir fanout) — em vez de buscar contornar a restrição.

---

## 3. Por que é valioso para o COSCA

- **Evita auto-rate-limit de fanout:** a lição empírica (calibrar 20 → 12 para não disparar o próprio rate-limiter) vira **regra estrutural**: cada provedor declara seu teto, e o sistema não explode a fonte por excesso de paralelismo.
- **Controle fino por fonte:** o teto por-provedor permite **explorar** uma fonte tolerante e **poupar** uma sensível — às duas com o valor certo, e não com um único número para todas.
- **Concorrência vira policy configurável:** em vez de um número no worker, a concorrência é uma **declaração** (por provedor, com teto global, cadência e recuo) — configurável, audível e versionada como as demais políticas do sistema.
- **Ressonância com a orquestração/fan-out dos agentes e a camada de fontes multi-provedor:** o fan-out dos agentes e a leitura multi-fonte passam a respeitar a mesma policy — um `region=all`/agregação multi-fonte não agride N provedores de uma vez, **cada um no seu teto**, com borne global.
- **Honestidade estrutural:** a doutrina **rejeita evasão** (spoofing). O COSCA **não** finge identidade para contornar restrição — adapta a estratégia. Isso preserva a **Lei do Cofre/LEALDADE** e evita degradar o sistema em um "burla anti-bot".

---

## 4. Consequências

### 4.1 O que muda (aditivo, direção da doutrina)

- **A orquestração/fan-out multi-fonte passa a respeitar a `ConcurrencyPolicy` declarada por provedor** em vez de um número fixo de worker: `max_global` como borne, `max_per_provider` por fonte, `rate_limit`/`burst` como contrato de cadência, `backoff` como resposta a recusa.
- **Concorrência vira configuração declarada:** o provedor declara seu valor-base; não há **constante mágica** `concurrency = 12` para todas as fontes.
- **O sistema ADAPTA a estratégia quando a fonte recusa:** no 429/503, recua com **backoff exponencial + jitter** e religa com moderação — em vez de tentar furar o rate-limit.
- **Uma nova fonte entra declarando sua política** (teto, cadência, burst) junto do seu registro — a base é per-provedor, não heurística.

### 4.2 O que NÃO muda (invariantes — a doutrina preserva os limites)

- **Nenhum mecanismo de evasão é aceito.** A doutrina **rejeita explicitamente** IP-spoofing, rotação de UA/disfarce de identidade e bypass de anti-bot para contornar rate-limit. A adaptação é **de estratégia**, nunca **de identidade**.
- A **autoridade de decisão** permanece do kernel/gate (I1) — a `ConcurrencyPolicy` **informa/limita**; ela **não substitui** o gate. O kernel continua sendo quem decide sobre empenho/recursos.
- A **Lei do Cofre / LEALDADE** (ADR-022) permanece como lei superior: a política opera **dentro** dela, não fora. Honestidade > evasão.
- **Nenhum `.go` funcional** é alterado por este ADR. Ele declara a doutrina; a implementação (ligar a policy na orquestração/fan-out da camada de fontes) é decisão subsequente de **workflow chief + platform**.

### 4.3 Trade-offs / riscos

- **Complexidade de política declarada:** a `ConcurrencyPolicy` tem 4 parâmetros por provedor (teto, cadência, burst, recuo) que precisam ser **declarados corretamente** — o risco é um provedor declarar um teto alto demais e disparar o próprio rate-limit (a falha que motivou o ADR). O `max_global` é a rede de segurança contra esse erro.
- **Tensão entre explorar e poupar:** o calibre por-provedor é uma **calibração**, não uma verdade. Um valor bom hoje pode ser ruim se a fonte mudar — a política precisa ser **revisável/ajustável** (configurável), não rígida.
- **Risco de abaixar demais a concorrência:** poupar demais uma fonte tolerante desperdiça latência/vazão. O equilíbrio (sustentada + burst) é o ponto de operação, e o sistema deve ser capaz de **medir e ajustar** a política.
- **Adaptação em vez de evasão = menor vazão em cenário de rate-limit:** quando a fonte recusa, o COSCA **recua** (backoff) e **não** contorna. Isso custa tempo, mas é o preço da **honestidade** — e é o único caminho aceito pela Lei do Cofre/LEALDADE.

---

## 5. Alternativas consideradas

| Alternativa | Veredito |
|---|---|
| **Concorrência fixa global no worker (status quo)** | ❌ Um único número (ex. `12`/`20`) para todas as fontes é ótimo para uma e péssimo para outra; um valor alto (20) **dispara o próprio rate-limiter** (lição empírica do osiris); trata fontes heterogêneas como homogêneas; não tem borne global explícito. |
| **Sem pool de concorrência (fanout ilimitado)** | ❌ Estoura o **pool de conexões** do transporte e dispara **auto-rate-limit por excesso de fanout** — o exato problema que o ADR resolve. Sem borne nenhum, a fonte quebra e o sistema se agride. |
| **Concorrência fixa por fonte (valor-base hardcoded por fonte)** | ⚠️ Melhor que a global, mas ainda **constante mágica** por fonte — não modela `rate_limit`/`burst`/`backoff`; não responde a recusa com adaptação; não tem teto global explícito. |
| **ConcurrencyPolicy declarada por provedor (max_global + max_per_provider + rate_limit + burst + backoff)** | ✅ Concorrência como **propriedade do sistema**, declarada por provedor; teto global (borne do sistema); teto por fonte (calibração por-provedor); cadência + rajada (contrato real do provedor); backoff exponencial + jitter (moderação após recusa). Sem constante mágica, sem evasão. |
| **Evasão de rate-limit (IP-spoofing / rotação de UA / bypass anti-bot)** | ❌ **REJEITADO — viola a Lei do Cofre/LEALDADE** (ADR-022). Falsificar identidade para contornar restrição externa é desonestidade estrutural; o caminho é **adaptar a estratégia**, não burlar. |

---

## 6. Teste de aceite (doutrina verificável)

- **`max_global` (borne do sistema):** um fan-out agregado sobre N provedores **nunca** ultrapassa `max_global` conexões concorrentes simultâneas — verificável por um medidor de concorrência ativa no transporte.
- **`max_per_provider` (calibração por fonte):** uma fonte com `max_per_provider = 12` é tocada com **no máximo 12** chamadas concorrentes, independentemente do global — e **não** dispara o próprio rate-limiter (a lição osiris: 12 OK, 20 dispara).
- **`rate_limit` / `burst` (cadência + rajada):** dentro da janela, o sistema respeita a taxa sustentada e só usa a rajada quando o provedor a tolera — sem estourar a cadência declarada.
- **`backoff` (moderação após recusa):** diante de **429/503**, o sistema **recua exponencialmente + jitter** e **não martela** retry sincronizado — não re-sincroniza todos os workers numa mesma onda.
- **`grep` por valor mágico de concorrência global:** em código novo de orquestração, concorrência **não** é um número fixo embutido — é lida da `ConcurrencyPolicy` declarada por provedor.
- **`grep` por evasão (spoofing/rotação de UA/disguise):** **nenhum** mecanismo de **IP-spoofing, rotação de UA para disfarçar, ou bypass anti-bot** para contornar rate-limit — a doutrina proíbe evasão; o sistema **adaptada a estratégia**, nunca a identidade.

---

## 7. Proveniência

**Padrão extraído (princípio, NÃO código)** do repositório **`github.com/simplifaisoul/osiris`** (licença **MIT**), arquivo **`src/lib/sherlock.ts`** (mapLimit + calibração de concorrência) e **`src/lib/httpJson.ts`** (UA honesto — contra-tendência a preservar).

O que foi levado como **princípio** (não como código):

- **Concorrência é restrição externa legítima**, não detalhe de implementação — a calibração empírica (20 → 12) mostra que **excesso de fanout causa auto-rate-limit**. O limite é a fronteira **entre explorar a fonte e acordar o seu rate-limiter**.
- **Um número fixo é ótimo para uma fonte e péssimo para outra** — cada provedor tem limites diferentes; o valor-base deve ser **declarado por provedor**, não constante mágica global.
- **Adaptar a estratégia em vez de evadir:** o `mapLimit` + `backoff` modelam o sistema **respeitando** o teto do provedor e **recuando** quando ele recusa — é o princípio da **moderação adaptativa**.
- **Honestidade antes de evasão (contra-tendência correta):** `httpJson.ts` (do mesmo repositório) usa **UA honesto/identificador em vez de spoofing** — é o princípio a **preservar**: declarar quem é e respeitar o rate-limit, nunca falsificar identidade para contornar.

O que **NÃO** foi levado:

- **Nenhum código, tipo, função ou estrutura do `sherlock.ts`/`httpJson.ts`.** Nenhum import, nenhuma assinatura, nenhum `mapLimit`/`Promise.all` real. A síntese em **4 parâmetros** (`max_global`/`max_per_provider`/`rate_limit`+`burst`/`backoff`) e a hierarquia **per-provedor com teto global** são a *formalização COSCA* do princípio, não a reprodução do arquivo-fonte.
- A **pilha/implementação** do repositório (stack TS/JS, estrutura de arquivos, APIs específicas) — **REJEITADA** a título de estilo, seguindo ADR-036/022 (selecionamos o minério, nunca a pilha).
- **Qualquer mecanismo de evasão** (IP-spoofing, rotação de UA não-identificador, bypass anti-bot) como forma de "resolver" o rate-limit — **REJEITADO** como princípio, pois viola a Lei do Cofre/LEALDADE (ADR-022).

---

*Autor: cosca-architecture. Padrão extraído (princípio) de osiris (`src/lib/sherlock.ts` — mapLimit + calibração de concorrência 20→12 — e `src/lib/httpJson.ts` — UA honesto/identificador —, MIT); formalização COSCA (`ConcurrencyPolicy`: max_global + max_per_provider + rate_limit + burst + backoff) — nenhum código do repositório-fonte importado. Escopo: documentação canônica. Nenhum arquivo `.go` funcional alterado; nenhuma implementação feita. Adoção: ADOTAR — workflow chief + platform (implementação).*
