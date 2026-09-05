# Mineração de Padrões — Repositório "Osiris" (relatório consolidado)

> **Ordem do Don**: minerar o repositório `github.com/simplifaisoul/osiris` (MIT), fechar a frente com um **mapa limpo** de o que foi encontrado, o que foi incorporado e o que ficou deliberadamente para depois — separado por **categoria de veredito**, com **proveniência** e **motivo arquitetural**.
> **Data**: 2026-09-04
> **Executor**: Cosca Kernel (consigliere) + cosca-architecture (Architecture Chief)
> **Método**: mineração manual do repositório (licença MIT) + avaliação de veredito junto ao Don. Nenhum código-fonte do osiris foi importado — apenas **princípios** extraídos e formalizados como doutrina COSCA (regime `ADR-036`/`ADR-037`: selecionamos o **minério**, nunca a **pilha**).
> **Estado**: FRENTE FECHADA (2026-09-04)

---

## 0. Resumo executivo (o mapa em uma olhada)

| Categoria | Itens | Status no COSCA |
|---|---|---|
| 🏆 **ADOTADO** (incorporado ao core via ADR) | 5 | `ADR-037`·`ADR-039`·`ADR-040`·`ADR-042` (x2) |
| 🛡️ **AUMENTADO** (hardening do que já existe) | 1 | `ADR-041` |
| ⏳ **AVALIAR DEPOIS** (2ª onda — não rejeitado) | 1 | **SEM ADR** (deliberado) |
| 🚫 **REJEITADO** (viola a doutrina da casa) | 1 | **SEM ADR** (deliberado) |

Foram levados **princípios** de 4 arquivos do osiris (`sherlock.ts`, `sourceCache.ts`, `httpJson.ts`, `ssrf-guard.ts`) e de 1 artefato de execução (`runs/ledger.jsonl`), e **rejeitado** integralmente 1 padrão de evasão (`stealthFetch.ts`). A tese central da mineração é o **estado epistêmico** (veredito ≠ verdade do mundo) — §8.

---

## 🏆 ADOTADO (incorporado ao core via ADR)

### 1. Estado epistêmico tri-estado (primitiva `Verdict`) — ADR-037

- **Proveniência**: `osiris/src/lib/sherlock.ts` (MIT).
- **Motivo arquitetural do veredito**: **simetria observador-do-mundo ⇄ avaliador-do-modelo**. A primitiva é única e simétrica: o mesmo `Verdict` qualifica a observação do mundo ("o recurso X existe?") e a avaliação do modelo ("o modelo Y acertou?"). Não confundir **ausência de evidência** com **evidência de ausência** — `BLOCKED`/`INCONCLUSIVE` **nunca** colapsam para `NOT_FOUND` (recusa de observação ≠ ausência provada). Calibração com **dois controles independentes** (um único controle deixa o soft-404 escapar quando esse controle é rejeitado — exemplo: Roblox/WordPress). Conecta-se diretamente à **lição da campanha-001** (o veredito de eval é uma afirmação epistêmica que pode ser não-confiável). É a primitiva de **primeira classe da Evidence Layer** — a moeda de troca entre camadas, não um detalhe de ferramenta.
- **Uso no COSCA**: base da doutrina epistêmica; consumido por `architecture + review/critic` (núcleo). Sem `.go` funcional alterado — apenas doutrina canônica.
- **Formalização COSCA**: síntese em **6 estados** (`FOUND`/`NOT_FOUND`/`BLOCKED`/`INCONCLUSIVE`/`SKIPPED`/`ERROR`) + matriz dos quadrantes (eixo verdade-do-mundo × eixo observação-do-instrumento). Nenhum código do `sherlock.ts` importado.

### 2. Cache single-flight + TTL + stale-on-error — ADR-039

- **Proveniência**: `osiris/src/lib/sourceCache.ts` (MIT).
- **Motivo arquitetural do veredito**: elimina o **thundering herd** (N agentes pedindo a mesma fonte = 1 requisição upstream; a fonte **nunca** é estampada por N requests). Segura a **fonte oscilante** (TTL + stale-on-error + backoff impedem que uma fonte errática seja agredida por cada request). **Não zera conhecimento por refresh que caiu** — uma falha efêmera (timeout/5xx/rate-limit, ou até resultado **vazio**) degrada para `stale` com o último dado bom, **nunca** para `null`/vazio. Ressoa com a doutrina epistêmica de **memória/conhecimento** (`ADR-036`/`ADR-037`): assim como um estado observacional falho não vira ausência, um refresh falho não vira mundo vazio.
- **Formalização COSCA**: 4 comportamentos combinados (TTL + single-flight + stale-on-error + mapa com teto/borne) + preservação da assinatura do fetcher (drop-in, transparência de contrato). Implementação: `cache chief + memory chief`.

### 3. ConcurrencyPolicy — ADR-040

- **Proveniência**: `osiris/src/lib/sherlock.ts` (MIT) — `mapLimit` + calibração de concorrência.
- **Motivo arquitetural do veredito**: **concorrência é propriedade do sistema**, não constante de worker. O modelo declara por provedor: `max_global` (borne do sistema), `max_per_provider` (calibração por fonte), `rate_limit`+`burst` (cadência + rajada), `backoff` (recuo exponencial + jitter). **Lição empírica 20→12**: calibrar 20 fez o scanner **disparar os próprios rate-limiters** das fontes (auto-rate-limit por excesso de fanout); 12 estabilizou. O limite é a fronteira **entre explorar a fonte e acordar o seu rate-limiter** — um número fixo é ótimo para uma fonte e péssimo para outra (fontes heterogêneas ≠ homogêneas). **Princípio axial**: rate-limit é restrição externa **legítima** — o sistema **adapta a estratégia**, nunca evade.
- **Formalização COSCA**: `ConcurrencyPolicy` como recurso de primeira classe (hierarquia per-provedor com teto global). Implementação: `workflow chief + platform`.

### 4. Conditional GET (ETag/Last-Modified) — ADR-042

- **Proveniência**: `osiris/src/lib/httpJson.ts` (MIT).
- **Motivo arquitetural do veredito**: **`304` transforma polling de dump raro em barato**. Em dumps/datasets que mudam raramente, o caso comum (nada mudou) vira **centenas de bytes de header** em vez de re-baixar um multi-megabyte byte-idêntico (tempo + banda + parse para aprender **zero**). `304` é resposta **válida, não erro** (semântica `changed:false`/`body:null`); sem validators degrada para GET comum com **grace**; validators são **persistidos entre polls** (a base do ganho). **Conteúdo decodificado pelo header**: o corpo é lido **pelo `Content-Encoding` da resposta**, não pelo que foi pedido no `Accept-Encoding` — alguns hosts servem JSON pré-comprimido e setam o encoding fora do `Accept` (evita `lixo por má-suposição`). Par natural do `ADR-039`: o `304` reduz o **custo por request validado**, o TTL/single-flight reduz o **número de requests**.
- **Formalização COSCA**: `httpConditional` (replay de validators + interpretação de `304`). Implementação: `integrations + runtime`.

### 5. `optional()` (enriquecimento opcional resiliente) — ADR-042

- **Proveniência**: `osiris/src/lib/httpJson.ts` (MIT).
- **Motivo arquitetural do veredito**: **enriquecimento opcional degrada graceful, nunca derruba o pipeline**. Chamadas de enriquecimento de baixa prioridade (dado complementar, não crítico) podem falhar por qualquer motivo transitório (timeout/5xx/rate-limit). `optional()` resolve `null` em vez de **estourar exceção** — a robustez do enriquecimento é **localizada**; o que é opcional não se propaga como erro fatal. **Limite claro**: `optional()` é para enriquecimento **opcional**, nunca para o caminho crítico (o obrigatório não passa por ele, para não mascarar bug real).
- **Formalização COSCA**: helper que neutraliza falha opcional como `null`. Implementação: `integrations + runtime`.

---

## 🛡️ AUMENTADO (hardening de algo que o COSCA já tem)

### 6. SSRF hardening — ADR-041

- **Proveniência**: `osiris/src/lib/ssrf-guard.ts` (MIT).
- **Motivo arquitetural do veredito**: esse item é um **AUGMENT**, não padrão novo — o COSCA **já tem** um guard anti-SSRF (a ferramenta `cosca_cosca_web` valida *host resolvido + IP público validado*), e este padrão **fecha os gaps que a primeira linha deixava passar**, pelo caminho **multi-hop** (`URL→DNS→IP→redirect→DNS→IP→destino`). Refinamentos portados:
  - **Rejeição de IPv4 não-canônico (IP-confusion)**: só `dotted-quad` estrito passa; decimal/hex/octal/mista (`2130706433`, `0x7f.0.0.1`, `0177.0.0.1`) são **negados** — forma não-canônica vira a brecha "o guard diz público, o SO conecta em `127.0.0.1`".
  - **Cobertura IPv6 reservado**: `::`, `::1`, `::ffff:` (IPv4-mapped), `64:ff9b::` (NAT64), `100::` (discard), `2001:db8::` (doc), `fc00::/7`, `fe80::/10`, `fec0::/10`, `ff00::/8` (multicast).
  - **Bloco IPv4 reservado completo**: incl. CGNAT `100.64.0.0/10` e `169.254.169.254` (metadata/IMDS), TEST-NET, multicast, reservado.
  - **Blocklist de hostnames de metadata antes do DNS**: `localhost`, `*.local`, `*.internal`, `host.docker.internal`, `metadata.google.internal`, etc. (o nome de host é indicador de intenção **anterior** à resolução).
  - **Revalidação hop-by-hop de redirect**: segue manualmente, reaplica o guard a cada hop, **nega público→privado**, limita a `http`/`https`, impõe teto de `maxRedirects`.
  - **Rate limiter por-IP + `getClientIp`**: impede o guard de virar trampolim de fetch (abuso de proxy).
- **Rejeitada uma nota de cautela** (deliberadamente não aprovada como mecanismo): IP-pinning no socket é o **degrau opcional seguinte** (TOCTOU total) — este ADR posiciona o guard como **primeira barreira**, não a última palavra. O guard continua **declarando e respeitando** limites, **sem evasão** (alinha-se a `ADR-022`).
- **Implementação**: `security chief`.

---

## ⏳ AVALIAR DEPOIS (2ª onda — NÃO rejeitado, NÃO esquecido)

### 7. Prediction Ledger (Brier/calibração) — SEM ADR (deliberadamente)

- **Proveniência**: `osiris/runs/ledger.jsonl` (MIT).
- **Motivo arquitetural do veredito**: este padrão é **estacionado — não foi rejeitado, não foi esquecido**. Não é tão fundamental quanto os demais (não toca a fronteira de verificação/observação como o `Verdict`, nem a resiliência de carga como o cache/concorrência). Trata-se de um ledger append-only de previsões probabilísticas (`statement`, `probability`, `base_probability`, `horizon`, `resolve_after`, `reasoning`, `split`, `agents`) auto-resolvível após o tempo, que permite medir **calibração** (Brier score) e responder: *"quando digo 80%, historicamente acerto ~80%?"*.
- **Deliberadamente SEM ADR** (decisão do Don, 2026-09-04): não abrir ADR-043 "só por abrir". Ficar **registrado explicitamente** como **2ª onda** para **não sumir da esteira**.
- **Caminho futuro (registrado para não esquecer)**: **AGUARDANDO a fase de evolução/calibração do COSCA** (`Golden Gate` + `CMI` + `CheckPromotion`). Considerar estender o Golden Gate/CMI para medir a **calibração da confiança do agente**, além de `BEFORE` vs `AFTER` em tasks — isto é, usar o ledger para validar a **confiança** que o agente afirma, não apenas o acerto da task.
- **Campos de apoio**: `evolution + memory chief`.

---

## 🚫 REJEITADO (viola a doutrina da casa)

### 8. stealthFetch (IP-spoofing + UA randomizado) — SEM ADR (deliberado)

- **Proveniência**: `osiris/src/lib/stealthFetch.ts` (MIT).
- **Motivo arquitetural do veredito**: o padrão falsifica **identidade** — injeta `X-Forwarded-For` de IPs residenciais falsos (IP-spoofing) e randomiza fingerprints de browser (UA não-identificador) — com o objetivo de **burlar o rate-limit** de fontes. Isso viola diretamente a **Lei do Cofre / LEALDADE** (`ADR-022`) e o **Fail-Closed** do **Guard Pact**: mentir para o externo para contornar uma **restrição externa legítima** é **desonestidade estrutural**, e degrada o sistema a um "burla anti-bot".
- **Princípio substituído pela contra-tendência correta do próprio repo**: o mesmo osiris, em `httpJson.ts`, usa **UA identificador honesto** (`OSIRIS-OSINT/1.0 (+url)`) e é **aceito** por fontes OSM/Nominatim/Overpass que **rejeitam browser-UA** com `406/429`. Ou seja: **honestidade + respeitar o rate-limit + adaptar a estratégia** (`ADr-040`/`ADR-042`: backoff, política, identificação) — a abordagem que **funciona melhor** — em vez de **trapacear para ser resiliente**. A frase-síntese do veredito: **"COSCA não precisa trapacear para ser resiliente."**
- **Deliberadamente SEM ADR**: registrar aqui como **rejeição explícita**, alinhada à admoestação de segurança do `cosca-security`, e **sem** gastar um ADR apenas com um anti-padrão.
- **Não levado**: nenhum mecanismo de evasão (IP-spoofing, rotação de UA não-identificador, bypass anti-bot) é incorporado, em nenhuma camada.

---

## Lições de arquitetura

A **tese central da mineração** não é a lista de padrões — é a **conexão do estado epistêmico com a campanha-001** (ver `ADR-038`):

> **O instrumento de avaliação ≠ a verdade do mundo.** O rótulo de eval é um **estado epistêmico do observador**, não uma afirmação incontestável sobre o mundo.

- A **campanha-001** (fine-tune LoRA no Qwen3-4B) regrediu `pass%` de **0.88 (base)** para **0.75 (LoRA)**; o **Golden Gate/PromotionGate** detectou e **REPROVOU** — o **mecanismo anti-autoengano funcionou** (decisão correta: manter `qwen3:4b` base). A lição **mais funda**, porém, não foi "o LoRA é pior": foi que **um rótulo `FAILURE`/`FALSE_COMPLETION` é uma ALEGAÇÃO do instrumento**, não a verdade do mundo. O gate observou com confiança (bem-observado), então a rejeição foi **válida** — mas o mesmo gate, se tivesse estado `BLOCKED`/`INCONCLUSIVE`, não poderia afirmar regressão.
- **O análogo do soft-404 do Sherlock**: um servidor responde `200 OK` para página inexistente (soft-404). Insistir que "a página existe" porque o instrumento respondeu `200` é confundir **a leitura do instrumento** com **a verdade do mundo**. No eval: `FAILURE` pode ser o "soft-404" do instrumento — o mundo estava certo, o instrumento leu errado.
- **Fórmula da casa** (`ADR-038`): *o rótulo diz o que o instrumento **segurou**. O que ele não segurou, ele **não afirma** — vira `INCONCLUSIVE`.* Um `FAILURE` só é válido quando o instrumento observou **com confiança** (coluna `OBSERVED`); na coluna `BLOCKED`, o veredito é **`INCONCLUSIVE`**, não `FAILURE` — independentemente de o mundo ter acertado ou errado.
- **Sem fraturar o anti-autoengano, nem cair no auto-engano reverso**: a doutrina **não** enfraquece um gate **bem-observado** (veto válido permanece); ela **veta apenas o gate que não observou**. `INCONCLUSIVE` é **honesto**, não **protecionista** — só entra quando o instrumento realmente não observou.
- **Generalização**: a mesma regra que distingue *capacidade ≠ provider* (`ADR-036`) e *verificação só com prova* (`ADR-022`) é o **princípio único** desta mineração: **não confundir a leitura do instrumento com a verdade do mundo.** Entre os dois, só atravessa **quem observou com confiança**.

---

## Referências (repositório minerado)

```
github.com/simplifaisoul/osiris  (licença MIT)
├── src/lib/sherlock.ts      → ADR-037 (Verdict tri-estado) · ADR-040 (ConcurrencyPolicy/mapLimit)
├── src/lib/sourceCache.ts   → ADR-039 (single-flight + TTL + stale-on-error)
├── src/lib/httpJson.ts      → ADR-042 (Conditional GET + optional()) · contra-tendência UA honesto
├── src/lib/ssrf-guard.ts    → ADR-041 (hardening SSRF hop-by-hop)
├── src/lib/stealthFetch.ts  → 🚫 REJEITADO (IP-spoofing/UA randomizado — viola Lei do Cofre/LEALDADE)
└── runs/ledger.jsonl        → ⏳ Prediction Ledger (2ª onda — SEM ADR, deliberado)
```

ADRs emitidos na esteira desta frente: **ADR-037**, **ADR-038**, **ADR-039**, **ADR-040**, **ADR-041**, **ADR-042**.

---

*Autor: cosca-architecture. Relatório consolidado da mineração do repositório osiris (MIT), fechando a frente com mapa de veredito por categoria (ADOTADO / AUMENTADO / AVALIAR DEPOIS / REJEITADO), proveniência por arquivo-fonte e motivo arquitetural de cada veredito. Nenhum código do repositório-fonte foi importado — apenas princípios extraídos e formalizados como doutrina COSCA. Escopo: documentação canônica. Nenhum arquivo `.go` funcional alterado. Frente: fechada.*
