# ADR-041: Hardening do guard anti-SSRF — validação hop-by-hop e rejeição de IP-confusion

> **Status:** PROPOSTO (ADOÇÃO: ADOTAR — hardening/augment do guard existente) | **Owner:** cosca-architecture (Architecture Chief)
> **Last Updated:** 2026-09-04
> **Natureza deste ADR:** **endurece/refina** o guard anti-SSRF que o COSCA **JÁ TEM** (a ferramenta `cosca_cosca_web` valida *host resolvido + IP público validado*). Este ADR **NÃO** introduz um padrão novo do zero — é um **AUGMENT** que documenta os refinamentos a portar via diff sobre o guard existente. Analogamente ao modo de `ADR-040`/`ADR-037`: fonte canônica de princípio, consumível por `architecture + security chief`.
> **Mapeamento de adoção:** **security chief (implementação)** — o guard anti-SSRF do COSCA é a fronteira de segurança da camada de fetch; o hardening entra ali, como refinamento do que já existe.
> **Proveniência:** padrão extraído (**princípio**, não código) de `github.com/simplifaisoul/osiris` (licença MIT), arquivo `src/lib/ssrf-guard.ts`. Ver §7.

---

## 1. Contexto / Problema (por que "URL inicial segura" não basta)

Uma intuição ingênua de SSRF considera apenas **a URL inicial** e pergunta *"a origem é segura?"*. Mas o caminho real de um fetch é **encadeado e multi-hop**:

> **URL inicial → DNS → IP → redirect → DNS → IP → destino final.**

O COSCA **já tem** um guard anti-SSRF (a ferramenta `cosca_cosca_web` resolve o host e valida que o **IP é público**). Isso é necessário, mas é **insuficiente**, por dois motivos de **classe** (não ruído):

1. **Confusão de representação IPv4 (IP-confusion).** O kernel/stack de rede resolve formas **não-canônicas** de endereços (`2130706433` decimal, `0x7f.0.0.1` hex, `0177.0.0.1` octal, notação mista/curta) e as trata como válidas. Um guard ingênuo que valida uma **string dotted-quad estrita** e nada mais, deixa passar o que o **kernel resolve** — a clássica brecha "o guard diz público, o SO conecta em `127.0.0.1`".
2. **O guard não acompanha o hop.** Se o `redirect` for seguido automaticamente e o host de destino for privado, o guard da **primeira** URL não protege o **destino final** — o ataque via redirect (latente ou direto) fura o guard de primeira linha.

O problema em uma frase: **cada hop** do caminho (`URL → DNS → IP → redirect → DNS → IP`) precisa **continuar** obedecendo ao guard. Um único ponto de validação na URL inicial cria uma janela de evasão (IP-confusion + redirect).

---

## 2. Decisão — Hardening do guard anti-SSRF (augment do existente)

**Adotar**, como **endurecimento do guard existente**, seis refinamentos que fecham as brechas acima. Nada aqui substitui o guard atual — **tudo o adiciona**.

### 2.1 Rejeição de IPv4 não-canônico (anti IP-confusion)

O guard deve aceitar **apenas `dotted-quad` estrito** (quatro octetos decimais `a.b.c.d`, cada um 0–255) e **rejeitar toda forma alternativa** que o kernel resolve:

- **Decimal inteiro:** `2130706433` → `127.0.0.1`.
- **Hexadecimal:** `0x7f.0.0.1`, `0x7f000001`.
- **Octal:** `0177.0.0.1`.
- **Notação mista/curta** e quaisquer outras formas que o **SO** interprete mas a validação ingênua da string não enxergue.

> **Regra:** o guard valida a **forma canônica**; se a representação do IP não é estritamente `dotted-quad`, ela deve ser **rejeitada** — não "normalizada depois". A conversão de formas alternativas **é a brecha**, não um detalhe.

### 2.2 Cobertura IPv6 de ranges reservados

O guard deve **bloquear** (tratar como não-público) os seguintes ranges **IPv6**, cobrindo as classes reservadas e especiais:

- `::` e `::1` — não-especificado / loopback.
- `::ffff:` — IPv4-mapped (o IP vira `127.0.0.1` e a validação IPv4 precisa valer aqui).
- `64:ff9b::` (NAT64) e `64:ff9b:1::`.
- `100::` — discard.
- `2001:db8:` — documentação.
- `fc00::/7` (`fc`/`fd`) — unique-local.
- `fe80::/10` (`fe8`–`feb`) — link-local.
- `fec0::/10` (`fec`–`fef`) — site-local.
- `ff00::/8` (`ff`) — multicast.

> Deve-se garantir que **todo o conjunto de ranges reservados IPv6** seja reconhecido pelo guard — não apenas o loopback, porque o `::ffff:` mapeado e os ranges de documentação/discard são igualmente vetores de SSRF.

### 2.3 Blocos IPv4 reservados/não-públicos

O guard deve **bloquear** os seguintes blocos **IPv4**:

- `0.0.0.0/8`
- `10.0.0.0/8`
- `100.64.0.0/10` — CGNAT (Tailscale é uma das aplicações que usa essa faixa).
- `127.0.0.0/8`
- `169.254.0.0/16` — incl. **`169.254.169.254`** (metadata / IMDS).
- `172.16.0.0/12`
- `192.0.0.0/24`
- `192.0.2.0/24` — TEST-NET-1.
- `192.168.0.0/16`
- `198.18.0.0/15`
- `198.51.100.0/24` — TEST-NET-2.
- `203.0.113.0/24` — TEST-NET-3.
- `224.0.0.0/4` — multicast.
- `240.0.0.0/4` — reservado.

> A lista é **exaustiva quanto às classes reservadas/não-públicas** — o guard não deve confiar em "é um IP que não conheço → deixa passar". Qualquer faixa fora do público real é **negada**.

### 2.4 Blocklist de hostnames de metadata (antes do DNS)

O guard deve bloquear **hostnames de metadata** **antes de qualquer resolução DNS**, independentemente do IP final:

- `localhost`, `*.localhost`
- `host.docker.internal`
- `*.local`
- `*.internal`
- `metadata.google.internal`
- (e, por extensão do padrão, hostnames conhecidos de metadata/metadata-services de outras nuvens)

> **Motivo da ordem:** validar o IP final não basta — um hostname de metadata pode resolver para um IP que o guard consideraria "público" (ou que muda via DNS). O nome de host é, ele mesmo, um indicador de intenção **anterior ao DNS**; bloquear o nome antes do lookup remove a classe de ataque antes que o DNS importe.

### 2.5 Revalidação hop-by-hop de redirect

O guard deve **seguir redirects manualmente**, **re-validando o host de cada hop**, em vez de delegar ao cliente o seguimento cego:

- **Revalidar cada hop:** a cada `Location` novo, re-aplicar o guard completo ao host **daquele** hop (IP + hostname).
- **Bloquear redirect público → privado:** um redirect partindo de um host **aparentemente público** que aponta para um host **privado** é **negado** (padrão clássico de SSRF via redirect).
- **Permitir só `http`/`https`:** qualquer outro esquema remetido por `Location` (file, gopher, ftp, etc.) é **rejeitado**.
- **Teto de `maxRedirects`:** um número máximo de saltos para barrar loops/longas cadeias.

> **Nota TOCTOU (defesa por profundidade):** o guard rejeita no *lookup*. A defesa **total** contra time-of-check-time-of-use (THC) exigiria **IP-pinning no socket** (fixar o IP resolvido na conexão). Rejeitar na validação **já bloqueia o ataque de **rebinding não-maligno**; o pinning é o próximo degrau opcional. O guard deste ADR é a **primeira barreira**, não a última palavra.

### 2.6 Rate limiter por-IP + `getClientIp`

Para abuso de proxy (o guard não deve virar um trampolim de fetch), o COSCA deve ter:

- **Rate limiter por-IP** no serviço de fetch — limitando quantas requisições de fetch um mesmo cliente pode disparar numa janela.
- **`getClientIp`** confiável, capaz de ler `x-forwarded-for` / `x-real-ip` de forma **controlada e sanitizada** — registrando a origem real do chamador (para o guard e para o rate-limiter) **sem aceitar** esses cabeçalhos de forma cega/forjada de qualquer origem.

> O objetivo não é bloquear usuário legítimo — é impedir que um ator **abuse do COSCA** como proxy para alcançar alvos internos via fetch (a mesma classe de abuso que o SSRF em si).

---

## 3. Por que é valioso para o COSCA

- **O COSCA já tem um guard anti-SSRF — este ADR o torna defensável de verdade.** A validação de "host resolvido + IP público" existente é **necessária mas não suficiente**; o hardening fecha exatamente os vetores que o guard de primeira linha deixa passar (IP-confusion, IPv6-mapped, ranges reservados, hostname de metadata, redirect).
- **Fecho da brecha mais comum:** IP-confusion e redirect público→privado são as duas formas **mais exploradas** de SSRF. Rejeitar representações não-canônicas e revalidar cada hop ataca a **causa**, não o sintoma.
- **Blindagem de metadata (IMDS).** Bloquear `169.254.169.254` **e** `metadata.google.internal` cobre os dois vetores mais críticos em cloud — o IP de metadata e o hostname que resolve para ele.
- **Defesa em profundidade sobre o caminho multi-hop.** "Cada hop obedece ao guard" transforma o fetch de "confia na primeira URL" em "confia em **todos** os pontos do caminho" — a única leitura correta de um SSRF em cadeia.
- **Proteção de abuso de proxy.** O rate-limiter por-IP + `getClientIp` impede que a ferramenta de fetch do COSCA seja usada como **trampolim** para alcançar alvos que o próprio COSCA estaria proibido de tocar.
- **Princípio preservado (sem custo de evasão):** o guard continua **declarando** e **respeitando** os limites — não há mecanismo de contorno. Alinha-se à Lei do Cofre / LEALDADE (ADR-022).

---

## 4. Consequências

### 4.1 O que muda (aditivo, sobre o guard existente)

- **O guard passa a rejeitar IP-confusion:** formas **não-canônicas** de IPv4 (decimal/hex/octal/mista) que o kernel resolve são **negadas** — só `dotted-quad` estrito passa.
- **O guard passa a cobrir IPv6 reservado:** ranges de loopback, IPv4-mapped, discard, doc, unique-local, link-local, site-local, multicast e NAT64 são **bloqueados** — não mais apenas o loopback.
- **O guard passa a validar o conjunto completo de blocos IPv4 reservados/não-públicos** (incl. CGNAT e `169.254.169.254`), em vez de apenas os clássicos.
- **O guard passa a bloquear hostnames de metadata antes do DNS** (`localhost`, `*.local`, `*.internal`, `metadata.google.internal`, etc.) — independente do IP final.
- **O guard passa a revalidar hop-by-hop o redirect:** segue o redirect manualmente, re-aplica o guard a cada hop, **nega** público→privado, **limita a `http`/`https`** e impõe um **teto de `maxRedirects`**.
- **O serviço de fetch passa a ter rate limiter por-IP + `getClientIp`** — originando a requisição auditável e limitando abuso de proxy.

### 4.2 O que NÃO muda (invariantes preservadas)

- **O guard anti-SSRF existente NÃO é removido nem substituído.** Este ADR é **AUGMENT** — a validação de "host resolvido + IP público" existente permanece; é **ampliada** pelos refinamentos.
- **A autoridade de decisão** permanece do kernel/gate (I1). O guard **informa/limita**; ele não substitui o gate nem decide por conta própria sobre o que o COSCA pode alcançar.
- **A Lei do Cofre / LEALDADE (ADR-022)** permanece como lei superior: o guard protege **declarando e respeitando** limites — **sem evasão**, sem falsificar origem, sem contornar a restrição.
- **Permitido apenas `http`/`https`** — o guard **não** expande o conjunto de esquemas aceitos. O escopo de protocolo do fetch não muda.
- **Nenhum `.go` funcional é alterado por este ADR.** Ele declara a doutrina; a portagem via diff para o guard existente é decisão subsequente de **security chief (implementação)**.

### 4.3 Trade-offs / riscos

- **Falso positivo por representação legítima:** ao rejeitar **toda** forma não-canônica, o guard pode negar URL escritas "incomuns" mas genuinamente públicas. O trade-off é aceito: **precisão de segurança > tolerância a URL exótica** — a forma canônica é a única aceita.
- **Tratamento de IPv6-mapped precisa ser consistente:** se o guard normalizar `::ffff:a.b.c.d` para IPv4 sem **então** aplicar a regra IPv4, o `::ffff:127.0.0.1` escapa. O risco é de dupla-lógica divergente — o guard deve aplicar **as duas camadas**.
- **Redirect manual aumenta custo:** seguir redirect hop-a-hop e revalidar é mais caro que o seguimento cego do cliente. O custo é o preço de não pular a validação do destino final.
- **Limite de `maxRedirects` pode cortar cadeia legítima:** um valor baixo negaria cadeias válidas. O teto precisa ser calibrado — suficiente para a web real, baixo demais para abuso.
- **`getClientIp` não pode aceitar cabeçalho forjado:** aceitar `x-forwarded-for`/`x-real-ip` de forma cega permite ao ator **forjar sua origem**. O `getClientIp` precisa de uma cadeia de confiança (o proxy/edge que escreve o cabeçalho é confiável) — senão vira um vetor de evasão (trade-off com a honestidade do ADR-022).
- **TOCTOU residual:** a rejeição no *lookup* bloqueia rebinding **não-maligno**, mas a janela *check→use* (time-of-check-time-of-use) só é **completamente** fechada por **IP-pinning no socket** (no tempo de conexão). O ADR posiciona o guard como **primeira barreira**, mas **não** a última. A defesa total é decisão subsequente/opcional.

---

## 5. Alternativas consideradas

| Alternativa | Veredito |
|---|---|
| **Somente guard atual (host resolvido + IP público, sem refinamentos — status quo)** | ⚠️ Necessário, mas **insuficiente**. Não rejeita IP-confusion (decimal/hex/octal resolve no kernel), não cobre IPv6 reservado além do loopback, não bloqueia hostname de metadata antes do DNS e não revalida redirect — as brechas de classe que este ADR fecha. |
| **Validação somente na URL inicial (ignora o caminho multi-hop)** | ❌ Confia que o redirect é seguro por ser "a mesma coisa" que a URL inicial. Um redirect público→privado fura o guard de primeira linha — é o vetor clássico que a **revalidação hop-by-hop** ataca. |
| **Confiar no guard do cliente seguindo redirects** | ❌ Delega o seguimento cego ao cliente de rede, que não re-valida o host de cada hop. Sem **revalidação manual + bloqueio público→privado + teto de redirects**, o destino final escapa. |
| **Bloquear só os endereços "óbvios de metadata" (`127`, `169.254`)** | ⚠️ Cobre parte do problema, mas não o IP-confusion (o mesmo IP "obscuro" escrito em decimal/hex), nem o IPv6-mapped (`::ffff:127.0.0.1`), nem o bloqueio do hostname de metadata antes do DNS. |
| **Hardening completo do guard existente (ADOTAR — rejeitar não-canônico + IPv6 reservado + IPv4 reservado completo + blocklist de metadata + revalidação hop-by-hop + rate limiter/`getClientIp`)** | ✅ Endurece o guard **existente** (não o substitui); fecha IP-confusion; cobre IPv6/IPv4 reservados de forma **exaustiva**; bloqueia hostname de metadata **antes** do DNS; revalida **cada hop** de redirect com restrição a `http/https` + teto; e protege contra abuso de proxy. Ao mesmo tempo, preserva a honestidade (ADR-022) e não expande o escopo de protocolos. |
| **IP-pinning no socket como única defesa (ignorar o guard de validação)** | ⚠️ Fecha o TOCTOU, mas por si só não trata IP-confusion/representação, nem a intenção do hostname de metadata, nem a cadeia de redirects. O pinning é o **último** degrau de defesa em profundidade — **complementa**, não substitui, a validação. |

---

## 6. Teste de aceite (doutrina verificável)

- **IP-confusion rejeitado:** `cosca_cosca_web`/o guard **recusa** `0x7f.0.0.1`, `2130706433`, `0177.0.0.1` e qualquer forma de `127.0.0.1` não-canônica — retorna/classifica como **não-público** sem resolver.
- **IPv6 reservado bloqueado:** o guard **rejeita** `::`, `::1`, `::ffff:127.0.0.1`, `64:ff9b::1`, `100::1`, `2001:db8::1`, `fd00::1`, `fe80::1`, `fec0::1`, `ff02::1` — não apenas `::1`.
- **Bloco IPv4 reservado coberto:** o guard **rejeita** `10.0.0.1`, `100.64.0.1` (CGNAT), `169.254.169.254` (metadata), `192.0.0.1`, `192.0.2.1` (TEST-NET-1), `198.18.0.1`, `198.51.100.1` (TEST-NET-2), `203.0.113.1` (TEST-NET-3), `224.0.0.1` (multicast), `240.0.0.1` (reservado).
- **Hostname de metadata bloqueado antes do DNS:** `localhost`, `foo.localhost`, `host.docker.internal`, `foo.local`, `foo.internal`, `metadata.google.internal` são **recusados** sem resolução — não importa para onde o mock/DNS apontaria.
- **Redirect revalidado hop-by-hop:** uma resposta `301`/`302` para um host **privado** (mesmo vindo de um host **público** aparente) é **negada**; `Location` com esquema não-`http`/`https` (ex. `file://`, `gopher://`, `ftp://`) é **rejeitado**; a cadeia é refreada por **`maxRedirects`**.
- **Rate limiter por-IP operante:** o serviço de fetch limita requisições por cliente numa janela, e o log/guardo identifica a origem via **`getClientIp`** (lendo `x-forwarded-for`/`x-real-ip` de forma sanitizada, sem aceitar forja cega).
- **Escopo de protocolo preservado:** o guard **não** passa a aceitar esquemas além de `http`/`https`.
- **`grep` por representação não-canônica / bypass:** nenhum caminho no guard valida IPv4 de forma que deixe passar decimal/hex/octal; nenhum caminho deixa o redirect ser seguido sem revalidação; nenhum hostname de metadata escapa da blocklist.

---

## 7. Proveniência

**Padrão extraído (princípio, NÃO código)** do repositório **`github.com/simplifaisoul/osiris`** (licença **MIT**), arquivo **`src/lib/ssrf-guard.ts`**.

O que foi levado como **princípio** (não como código):

- **A visão hop-by-hop do SSRF:** o caminho real é **URL → DNS → IP → redirect → DNS → IP → destino final**, e **cada hop** deve continuar obedecendo ao guard — não só a URL inicial.
- **A rejeição de IP-confusion:** aceitar apenas a **forma canônica** (`dotted-quad` estrito) e rejeitar decimal/hex/octal/mista — formas que o kernel resolve mas a validação ingênua deixa passar.
- **A cobertura exaustiva de ranges reservados IPv6 e blocos IPv4** (incl. CGNAT, IPv4-mapped, TEST-NET, multicast, reservado, `169.254.169.254`/metadata e classes especiais de doc/discard/link-local/site-local).
- **O bloqueio de hostnames de metadata antes do DNS** — o nome de host é indicador de intenção **anterior** à resolução; bloqueá-lo no topo remove a classe antes que o IP importe.
- **A revalidação de redirect hop a hop** com restrição a `http`/`https` e teto de `maxRedirects`, e a **consciência do TOCTOU** (rejeitar no lookup bloqueia rebinding não-maligno; defesa total exigiria IP-pinning).
- **A proteção contra abuso de proxy** (rate limiter por-IP + origem via `getClientIp` lendo `x-forwarded-for`/`x-real-ip` de forma sanitizada) — o guard não deve virar trampolim de fetch.

O que **NÃO** foi levado:

- **Nenhum código, tipo, função, regex, lista ou estrutura do `ssrf-guard.ts`.** Nenhum import, nenhuma assinatura, nenhuma implementação de `isPrivate`/`isReserved`/`parse` real. A síntese em **6 refinamentos** (rejeição de não-canônico, IPv6 reservado, blocos IPv4, blocklist de metadata, revalidação hop-by-hop, rate limiter/`getClientIp`) e a **nota TOCTOU** são a *formalização COSCA* do princípio, não a reprodução do arquivo-fonte.
- A **pilha/implementação** do repositório (stack TS/JS, estrutura de arquivos, APIs específicas) — **REJEITADA** a título de estilo, seguindo ADR-036/022 (selecionamos o minério, nunca a pilha).
- **Qualquer mecanismo de evasão** (falsificar origem, aceitar `x-forwarded-for` forjado de qualquer origem, contornar o propio rate-limiter) — **REJEITADO** como princípio, pois viola a Lei do Cofre / LEALDADE (ADR-022).

---

*Autor: cosca-architecture. Padrão extraído (princípio) de osiris (`src/lib/ssrf-guard.ts`, MIT); formalização COSCA (6 refinamentos de hardening do guard anti-SSRF existente: rejeição de IPv4 não-canônico + cobertura IPv6 reservado + blocos IPv4 reservados + blocklist de hostnames de metadata + revalidação hop-by-hop de redirect + rate limiter por-IP e `getClientIp`) — nenhum código do repositório-fonte importado. Este ADR é AUGMENT do guard já existente (a ferramenta `cosca_cosca_web` valida host resolvido + IP público validado); não é padrão novo. Escopo: documentação canônica. Nenhum arquivo `.go` funcional alterado; nenhuma implementação feita. Adoção: ADOTAR (hardening/augment) — security chief (implementação).*
