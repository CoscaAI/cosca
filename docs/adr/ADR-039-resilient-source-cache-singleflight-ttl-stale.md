# ADR-039: Cache de Fonte Resiliente — `single-flight` + TTL + `stale-on-error` (fonte com borne)

> **Status:** PROPOSTO (ADOÇÃO: ADOTAR) | **Owner:** cosca-architecture (Architecture Chief)
> **Last Updated:** 2026-09-04
> **Natureza deste ADR:** declara a **doutrina de carga de fonte resiliente**. **NÃO** cria código — é fonte canônica de princípio, análoga ao modo de `ADR-037`/`ADR-036` (doutrina que aponta para a fonte em código quando ela existir). O núcleo da decisão é um **padrão de carga de fonte** (wrapper), consumível por `architecture + cache chief + memory chief`.
> **Mapeamento de adoção:** **cache chief + memory chief (implementação)** — o wrapper entra na camada de conhecimento/semântica, não como órgão periférico.
> **Proveniência:** padrão extraído (**princípio**, não código) de `github.com/simplifaisoul/osiris` (licença MIT), arquivo `src/lib/sourceCache.ts`. Ver §7.

---

## 1. Contexto / Problema (por que carregar a fonte uma vez com borne é necessário)

Agentes do COSCA consultam **fontes upstream** (conhecimento, semântico, web, datasets). Nesses caminhos existem três problemas de **classe** (não ruído) que um fetch ingênuo não resolve:

1. **Efeito manada (thundering herd).** Múltiplos agentes podem pedir a **mesmo fonte ao mesmo tempo**. N acessos concorrentes a um cache-miss disparam N requisições upstream → estoura a fonte e dispara **rate-limit**. O correto é que N concorrentes compartilhem **UMA única requisição**.
2. **Fonte rápida de mudar, lenta de carregar.** Fontes mudam raramente, mas algumas são caras/lentas de buscar. Exemplo empírico (osiris): um índice de ~500KB **re-baixado a cada request** era puro desperdício; fontes `MDOT` (~7s) e `NZTA` (~8s) dominavam o tempo de resposta; um `region=all` batia em **todas** as fontes de uma vez → **~15s por request**. Sem cache com TTL, cada request paga o custo integral.
3. **Falha transitória vira "amnésia artificial".** Uma falha efêmera do upstream (timeout, 5xx, rate-limit) **não deve zerar o conhecimento/dados**. Se o refresh cai, apagar o que já se tinha por causa de uma falha momentânea é apagar o histórico por um acidente. O certo é **servir o último dado bom (stale)** e marcar como **degraded**.

> **Problema em uma frase:** carregar cada fonte **uma vez por janela (TTL)**, deduplicando acessos concorrentes ao mesmo miss (single-flight) e, se o refresh falhar, **servir stale em vez de vazio** — sem martelar o upstream.

---

## 2. Decisão — wrapper de fonte resiliente com 4 comportamentos combinados

**Adotar** um **wrapper de carga de fonte** (drop-in replacement do fetcher, preservando a assinatura) que combina **4 comportamentos** numa única máquina de estados de cache:

### 2.1 TTL — serve da memória até expirar

- O resultado é servido do cache local enquanto a **idade do dado < TTL**.
- Nenhum request upstream é emitido durante a janela. Toda a carga vem da memória.
- **Objetivo:** eliminar o custo integral de re-buscar uma fonte que muda raramente (o índice de ~500KB re-baixado a cada request).

### 2.2 Dedupe `single-flight` — N concorrentes → 1 request

- N acessos concorrentes ao **mesmo cache-miss** compartilham **UMA única requisição upstream**.
- Mecanismo: uma **Promise in-flight compartilhada** (a mesma promoção de `Promise`/`Future` guardada no mapa do cache-miss). O 2º, 3º... Nº chamador **aguarda** a mesma promessa, em vez de abrir outra requisição.
- **Regra central:** **Nunca estampa a fonte com N requests.** A fonte é tocada **exatamente 1 vez** por janela de miss, independente de quantos agentes cheguem juntos.
- **Proteção explícita ao fanout multi-fonte:** um `region=all` que bate em muitas fontes de uma vez fica seguro — cada fonte individual é single-flight.

### 2.3 `stale-on-error` — falha degrada para o último dado bom, nunca para vazio

- Se o **refresh** falhar (exceção, timeout, 5xx, rate-limit, **ou resultado vazio**), o wrapper **serve o último dado bom (stale)** em vez de zerar/cachear `null`.
- **Resultado vazio também conta como refresh falho.** Um upstream que devolve nada (resposta vazia/soft-empty) não é motivo para apagar o que se tinha.
- **Retry mais cedo que o TTL cheio, mas sem martelar:** após falha, re-tenta em uma janela curta (ex. `+60s`), **não** no TTL completo — mas com **backoff** (nunca inunda a fonte a cada vez que alguém pedir).
- **Semântica de status:** o wrapper expõe um sinalizador de **`degraded`/`stale`** (o dado é armazenado, mas marcado como envelhecido/parcialmente degradado), permitindo à camada superior **escalar de degradado para crítica** quando preciso.

### 2.4 Mapa com teto — evicta a mais antiga, nunca uma in-flight

- **Limite de entradas** (ex. `500`). O cache é um **Mapa com teto**, não um mapa sem fim.
- **Evicta a mais antiga** (ordem de inserção — FCFS / FIFO por inserção).
- **Regra inviolável:** **NUNCA evicta uma entrada com request in-flight.** Se uma entrada está sendo carregada, ela é protegida até a requisição resolver (settle) — evictá-la quebraria a promessa compartilhada (single-flight) e poderia duplicar requests.

### 2.5 Drop-in replacement

- O wrapper **preserva a assinatura do fetcher** (`get()/fetch()` com a mesma entrada/saída), de modo que os chamadores atuais **não mudam** — apenas passam a receber a resiliência embutida. Princípio de **transparência de contrato**.

---

## 3. Por que é valioso para o COSCA

- **Elimina o thundering herd:** N agentes pedindo a mesma fonte = 1 requisição. A fonte **nunca** é estampada por concorrência.
- **Segura a fonte quando ela oscila:** TTL + stale-on-error + backoff impedem que uma fonte errática seja agredida por cada request (o que prolongaria a instabilidade).
- **Não deixa o sistema cego por falha transitória:** falha de refresh → `degraded` com dado bom, não `empty`. O sistema **continua vendo o mundo** mesmo quando o upstream tropeça.
- **Ressonância direta com memória/conhecimento:** o mesmo princípio de **"não apagar conhecimento por um refresh que caiu"** casa com a doutrina epistêmica (ADR-036/037) — um estado observacional falho não vira ausência; aqui, um refresh falho não vira `null`/vazio.

---

## 4. Consequências

### 4.1 O que muda (aditivo)

- **Agentes e a camada de conhecimento/semântico passam a usar o wrapper de fonte resiliente** em vez de fetch direto. Toda leitura de fonte upstream é feita pelo wrapper.
- **Qualquer fanout multi-fonte fica protegido por single-flight:** `region=all`, agregações, harvest de várias fontes → cada fonte individual é deduplicada (1 request por fonte por janela).
- **Falha transitória degrada para `stale`, nunca para vazio:** a camada superior pode detectar `degraded` e escalar, mas **nunca** herdará um "mundo vazio" por causa de um refresh que caiu.
- **Latência em cascata é cortada:** a fonte lenta (ex. `MDOT` ~7s / `NZTA` ~8s) é buscada **uma vez** por TTL, não por request; um `region=all` não paga `N × 15s`, paga `1 × carga` por janela.

### 4.2 O que NÃO muda

- A **assinatura do fetcher** (drop-in replacement) — os chamadores existentes não são refatorados; apenas passam a receber a resiliência embutida.
- A **semântica de dados** do upstream — o wrapper **não altera** o que a fonte retorna; apenas controla **quando/quanto** ela é tocada e **o que serve quando ela falha**.
- A **autoridade de decisão** permanece do kernel/gate (I1) — o wrapper **informa** (`degraded`/`stale`), não decide.
- **Nenhum código funcional** é alterado por este ADR. Ele declara a doutrina; a implementação (ligar o wrapper nas camadas de conhecimento/semântico) é decisão subsequente de cache chief + memory chief.
- A **Fonte em si**: a resiliência é do lado do cliente (wrapper); a fonte upstream continua como é.

### 4.3 Trade-offs / riscos

- **Complexidade de máquina de estados:** o wrapper tem estados (fresh/stale/miss/in-flight) que precisam ser implementados com rigor — o risco é errar o single-flight (deixar escapar 2 requests) ou o stale-on-error (apagar dado por resultado vazio).
- **Custo de memória do mapa com teto:** limitação de entradas é **trade-off de capacidade por simplicidade**. Um mapa infinito seguraria mais, mas cresce sem barreira; o teto (ex. 500) dá **borne** — a troca é: as mais antigas são evictadas (o que é aceitável para fontes que mudam raramente e já foram cacheadas).
- **Risco de `stale` demais:** se a fonte **realmente** mudou e o refresh falha repetidamente, o wrapper serve dado velho. Isso é **intencional** (degradado > vazio), mas a camada superior precisa ser capaz de **escalar/alertar** quando o `degraded` persiste — senão o COSCA pode operar sobre uma imagem velha do mundo sem saber.

---

## 5. Alternativas consideradas

| Alternativa | Veredito |
|---|---|
| **Fetch direto sem cache (status quo)** | ❌ Paga o custo integral a cada request (~15s para `region=all`), estampa a fonte com N requests (thundering herd → rate-limit), e uma falha transitória zera a informação. |
| **Só cache TTL (sem single-flight, sem stale)** | ⚠️ Resolve o custo repetido, mas N concorrentes a um miss ainda disparam N requests (herd persiste) e uma falha de refresh ainda apaga o dado (amnésia artificial). |
| **Só single-flight (sem TTL)** | ⚠️ Deduplica concorrência, mas religa o mesmo upstream a cada evento de miss sem janela — fonte lenta é tocada com frequência, e não há proteção contra falha transitória. |
| **Retorno `null`/vazio na falha (deixar a camada superior decidir)** | ⚠️ Empurra o problema para cima e reintroduz a "amnésia artificial": a camada superior tende a tratar vazio como ausência real. |
| **Cache de fonte resiliente: single-flight + TTL + stale-on-error + mapa com teto (ESTE ADR)** | ✅ Combina os 4 comportamentos: 1 request por fonte por janela (herd eliminado), custo pago uma vez por TTL (fonte lenta barata), falha transitória degrada para stale (sem cegueira), e o mapa tem borne (sem crescimento sem barreira). |

---

## 6. Teste de aceite (doutrina verificável)

- **Single-flight:** N agentes pedem a mesma fonte simultaneamente → upstream é tocado **exatamente 1 vez** por miss (verificável por contador de requests no fetcher real; N-1 chegam via a mesma Promise in-flight).
- **TTL (custo repetido):** dentro da janela, um segundo request da mesma fonte **não** toca o upstream — serve da memória (o índice de ~500KB não é re-baixado).
- **stale-on-error:** um refresh que **falha** (exceção/timeout/5xx) **não** zera o dado — serve o último bom e marca `degraded`. Um resultado **vazio** também é tratado como refresh falho (não cacheia `null`).
- **Backoff sem martelar:** após falha, o retry é mais cedo que o TTL cheio (ex. `+60s`) mas com **backoff** — não inunda o upstream a cada pedido durante a janela de erro.
- **Mapa com teto:** o cache não ultrapassa o limite de entradas; a mais antiga é evictada (ordem de inserção); **nenhuma entrada com request in-flight é evictada**.
- **Drop-in:** a assinatura do fetcher não muda — uma fonte nova só precisa trocar o fetch direto pelo wrapper, sem alterar os chamadores.

---

## 7. Proveniência

**Padrão extraído (princípio, NÃO código)** do repositório **`github.com/simplifaisoul/osiris`** (licença **MIT**), arquivo **`src/lib/sourceCache.ts`**.

O que foi levado como **princípio** (não como código):

- **TTL** para servir de memória até expirar, evitando re-buscar uma fonte que muda raramente.
- **Single-flight (dedupe de concorrência)** para que N acessos ao mesmo cache-miss compartilhem UMA única requisição upstream via Promise in-flight compartilhada — a fonte nunca é estampada por N requests.
- **stale-on-error** para que uma falha de refresh (inclusive resultado vazio) sirva o último dado bom + `degraded` em vez de zerar, com retry mais cedo que o TTL cheio mas com backoff.
- **Mapa com teto** (limite de entradas, evicta a mais antiga por ordem de inserção, **nunca** evicta uma in-flight).
- **Preservação da assinatura do fetcher** (drop-in replacement) — transparência de contrato.

O que **NÃO** foi levado:

- **Nenhum código, tipo, função ou estrutura do `sourceCache.ts`.** Nenhum import, nenhuma assinatura, nenhuma classe/Map real. A síntese em **4 comportamentos combinados** (TTL + single-flight + stale-on-error + mapa com teto) e os **exemplos concretos** (índice ~500KB, `MDOT` ~7s, `NZTA` ~8s, `region=all` ~15s) são a *formalização COSCA* do princípio, não a reprodução do arquivo-fonte.
- A **pilha/implementação** do repositório (stack TS/JS, estrutura de arquivos, APIs específicas) — **REJEITADA** a título de estilo, seguindo ADR-036/022 (selecionamos o minério, nunca a pilha).

---

*Autor: cosca-architecture. Padrão extraído (princípio) de osiris (`src/lib/sourceCache.ts`, MIT); formalização COSCA (4 comportamentos combinados) — nenhum código do repositório-fonte importado. Escopo: documentação canônica. Nenhum arquivo funcional alterado; nenhuma implementação feita. Adoção: ADOTAR — cache chief + memory chief (implementação).*
