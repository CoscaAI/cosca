# RODADA DE QUEBRA — Queries Cross-Domain (o router mostra os dentes)

> **AUDIT → EVIDENCE → VERDICT** · 2026-08-24 · **NO CODE CHANGED** (leitura +
> instrumentação test-only). Esta é a **próxima rodada de quebra** que o professor
> apontou: *"queries ambíguas/cross-domain é onde um router costuma mostrar os
> dentes."* Objetivo: descobrir se a arquitetura é **propriedade robusta** ou
> **otimização local**.
>
> Resultado: **o teste encontrou uma limitação REAL do modelo de roteamento por
> path** — não um falso positivo. E documenta o comportamento bom (o router amplia)
> junto com a limitação.

---

## 1. O experimento (methodologia)

5 queries **cross-domain/ambíguas** (combinam triggers de módulos DIFERENTES),
pré-registradas, com a MESMA instrumentação do benchmark v2.1 (candidate set
derivado por path-segment, CandidatePool=0, EnableGraph=false).

| # | Query | Triggers que disparam |
|---|---|---|
| 1 | "como o serve configura o runtime como servico systemd" | runtime (serve/runtime/configurar/systemd) |
| 2 | "a memoria da familia e a lei da velocidade entre sessoes" | memory (memoria/lei/velocidade) + runtime (sessoes) |
| 3 | "o gate de integridade da chain depende do worker fabric" | knowledge (gate/integridade/chain) + architecture (worker/fabric) |
| 4 | "como rodar o benchmark do cosca e o fabric gerencia loads" | cli (benchmark) + architecture (fabric/worker) |
| 5 | "a seguranca do runtime usa gate para validar autorizar" | security (seguranca) + runtime (runtime) + knowledge (gate) |

---

## 2. EVIDENCE — o comportamento do router

### 2.1 O router AMPLIA corretamente (comportamento BOM)

| # | Módulos resolvidos | Leitura |
|---|---|---|
| 1 | [runtime] | 1 módulo (query puramente runtime) |
| 2 | [memory, runtime] | ✅ **ampliou** (reconheceu memória + execução) |
| 3 | [architecture, knowledge] | ✅ **ampliou** |
| 4 | [architecture, cli] | ✅ **ampliou** |
| 5 | [knowledge, runtime, security] | ✅ **ampliou** (3 módulos) |

**O router NÃO estreita demais em queries ambíguas — ele AMPLIA para os módulos
relevantes.** Em 4/5 resolve para 2-3 módulos. Isso é o comportamento desejado.

### 2.2 MAS o candidate-set por path não captura a evidência (a limitação)

Comparando o recall do routed vs o doc top-1 do full-scan (a evidência):

| Q | Módulos | Cand | Full-scan top-1 doc | RecallDocRouted@10 |
|---|---|---|---|---|
| 1 | [runtime] | 224 | `df407cef` (PROVIDER_INTERFACE.md) | **0** ⚠️ |
| 2 | [memory, runtime] | 8.565 | `3e30e5ae` | **0** ⚠️ |
| 3 | [architecture, knowledge] | 5.137 | `83c26738` | **0** ⚠️ |
| 4 | [architecture, cli] | 2.102 | `6a03e0f7` | **1** ✅ |
| 5 | [knowledge, runtime, security] | 4.533 | `04d467c6` | **0** ⚠️ |

### 2.3 A CAUSA RAIZ (verificada por medição read-only)

Para cada query onde o routed perdeu, o doc top-1 do full-scan está **FORA do
candidate set** do módulo roteado:

| Q | Doc do GT | Módulo roteado | Doc no path do módulo? | Vetores no candidate set |
|---|---|---|---|---|
| 1 | PROVIDER_INTERFACE.md | runtime | ❌ | 0/36 |
| 2 | PATTERN... | memory | ❌ | 0/26 |
| 3 | mem... | knowledge | ❌ | 0/15 |
| 5 | mem... | security | ❌ | 0/124 |

> **A causa raiz é a mesma e consistente:** o doc que o full-scan acha como top-1
> (a evidência da query) **NÃO está fisicamente no path do módulo que o router
> escolheu.** Ex.: `PROVIDER_INTERFACE.md` fala de serve/runtime (o TÓPICO é
> runtime), mas fisicamente mora em `internal/embed/cosca/` (segmento `cosca`,
> **não** `runtime`). O router acertou o **tema**, mas o **confinamento por
> path-segment** (`pathHasSegment`) não encontra o doc porque a evidência física
> está fora do segmento do módulo roteado.

---

## 3. VERDICT — a fronteira real do modelo de roteamento por path

O professor previu que as queries cross-domain revelariam se a arquitetura é
**robusta** ou **otimização local**. O resultado:

### O que está BOM (comportamento correto)
- ✅ **O router AMPLIA o espaço em queries ambíguas** (resolve 2-3 módulos) — não
  estreita demais no aspecto do roteamento.
- ✅ **O candidate-set é exaustivo** (COUNT(JOIN)==len) e a redução é real.
- ✅ Q4: o routed **mantém** a evidência (recall=1) mesmo cross-domain.

### A LIMITAÇÃO (fronteira real)
- ⚠️ **O `candidate-set` é derivado por `path-segment`**, e a evidência física pode
  morar em um path que **não corresponde ao segmento do módulo do tópico**. Quando
  o tema da query (runtime) e a localização física do doc (`cosca/`) divergem, o
  conflito é o doc **fica fora do candidate set** → recall=0 no routed.

> **Isto é a FONTEIRA do modelo de roteamento por path (auditoria v1 já alertava):
> "módulo = segmento do path onde a evidência mora".** Em queries onde o tópico e a
> localização física divergem, o `pathHasSegment` não captura a evidência. **Não é
> um bug do router** (ele resolveu o tópico certo e ampliou); é o **sinal de domínio
> (path) que é insuficiente** quando a evidência não mora no segmento do módulo.

### Classificação (rigor — não converter em falso bug)
- **Não é "router falhou"**: o router ampliou corretamente.
- **É uma limitação conhecida do `pathHasSegment`** (sinal de domínio por path), que
  já foi documentada na auditoria v1 e que a **Fatia 3** (mapeamento semântico
  módulo→domínio, em vez de path literal) endereçaria.
- Estado: 🟢 **Hipótese principal ainda sustentada** (no corpus/queries originais);
  a rodada cross-domain revelou a **limitação do sinal de path** — uma **fronteira
  arquitetural**, não uma regressão.

---

## 4. PROVENANCE

- **NO CODE CHANGED** — produção intacta; só leitura + instrumentação test-only.
- O teste cross-domain é **evidência de quebra** (instrumento), não mudança de produção.
- A descoberta fica **registrada** como fronteira do modelo de roteamento por path
  (para a Fatia 3 endereçar com mapeamento semântico módulo→domínio).

> **O que a rodada de quebra produziu:** o professor pediu "queries cross-domain
> para ver o router mostrar os dentes". Ele mostrou **dois dentes**: (1) **amplia
> corretamente** (bom); (2) **o sinal de domínio por path é insuficiente quando o
> tópico e a localização física divergem** (a limitação). Isso não derruba a
> arquitetura — **refina o entendimento dela**: o router por path funciona para
> evidência que mora no segmento do módulo; para evidência cross-location, precisa
> do mapeamento semântico (Fatia 3).
