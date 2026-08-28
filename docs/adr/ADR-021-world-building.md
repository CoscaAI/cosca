# ADR-021: World Building — o sistema decide o mundo; o LLM propõe (I1)

> **Status:** Proposed | **Owner:** cosca-kernel + cosca-architecture | **Last Updated:** 2026-08-28
> **Revisão:** aguardando Don + cosca-cto. **Design aditivo — não quebra o Root.**
> **Referência (base):** mineração `SimWorld-AI/SimWorld` + `bilawalsidhu/gods-eye-view` +
> `bilawalsidhu/see-through-walls` + `ADR-017` (Borrowing) + Living World (Cosca=cognição,
> Unreal=corpo, Adapters=nervos).

---

## 0. Contexto — 3 minas, 1 tese validada

Minamos 3 projetos de construção de mundo. **Eles validam empiricamente a arquitetura do Cosca:**
*o "cérebro propõe, o sistema decide" (I1) é a arquitetura certa.* O SimWorld prova que delegar
low-level ao LLM degrada segurança; a razão de ser do orquestrador determinístico é dar **autoridade
de mundo ao sistema**.

**Correções factuais:** SimWorld NÃO mistura LLM na geração base (citygen é determinístico com seed);
o "language-based procedural generation" é só depois, no AssetsRP. gods-eye-view é visualização (Cesium)
de dados ao vivo — o Cosca deve preferir snapshots offline + `INFERRED`.

## 1. A tese — 3 camadas do mundo

1. **O SISTEMA gera o mundo** (determinístico, zero-LLM): citygen procedural, física, modelo temporal,
   frame-de-mundo compartilhado, geodésia.
2. **O LLM só PROPÕE** intenção estruturada (destino, "colocar X à frente de Y"); o sistema resolve
   espaço/rota/placement.
3. **A PERCEPÇÃO/EPISTEMIA** distingue "saber" de "ver" (oclusão é fato epistêmico), com proveniência
   por entidade.

## 2. O que ADAPTAR (mapeado ao Cosca)

| # | Ideia | Fonte | Vai para | Implementação |
|---|---|---|---|---|
| **1** | **Local Action Planner rule-based** (LLM propõe destino → sistema valida + A* + executa) | SimWorld | `internal/world/nav` | ✅ **IMPLEMENTADO** (gate I1/I2 + A*) |
| **2** | **Language-driven scene editing** (LLM extrai campos → sistema retrieval+placement) | SimWorld (AssetsRP) | `internal/proposal`+`gate`+`world` | 🔵 próximo |
| **3** | **Frame-de-mundo compartilhado + saber≠ver** (ancoragem VPS; oclusão epistêmica) | see-through-walls | `internal/worldmodel/spatial` + `WorldEntity` | 🔵 |
| **4** | **Road-network topológica** (priority-queue growth + merges + interseção + quadtree) | SimWorld | `internal/world/city` | 🔵 |
| **5** | **Temporal determinístico** (bracketing/dead-reckon/coast p/ feed ruidoso) | gods-eye-view | `internal/worldmodel/temporal` | 🔵 |
| **6** | **Per-entity provenance + FeedState** (`Source/asOf/nominal|degraded|stale`) | gods-eye-view | `WorldEntity` (I3/I4) | 🔵 |
| **7** | **Geodetic datum** (WGS84→ECEF + EGM96 + ENU) | gods-eye-view | `internal/world/geo` | 🔵 (só se globo) |
| **8** | **Gym-like percepção estruturada + feedback MEASURED** | SimWorld | `internal/gameengine`+`orchestrator` | 🔵 |
| **9** | **Grafo de navegação (calçada/crossing)** | SimWorld | `internal/world` | 🔵 |

## 3. O que REJEITAR

- **UE5/UnrealCV/`/Game/`/Blueprint** (SimWorld acopla layout ao engine; Cosca já usa `AssetID`
  semântico — não recuar).
- **Cesium/WebGL/JS + agente OpenAI Realtime** (gods-eye-view) — I7/I1.
- **Unity/iOS/glasses + servidor externo autoritativo** (see-through-walls) — I7/I8 (Cosca é dono
  do frame).
- **Dados ao vivo como fonte de verdade** → snapshots offline + `INFERRED` explícito (I4/I8).
- **Controle low-level por VLM** (viola I1); **regex de pose**; **stack Python/PyQt**.
- **Depender de UE5**: o Cosca "Unreal=corpo" já existe como adapter; não embutir UnrealCV.

## 4. Invariantes

Todas as ideas de §2 preservam I1 (determinísticas; o LLM só propõe). O planner item 1 **materializa
o gate I1 no mundo**: destino/gate I2 (fail-closed), A* determinístico. Proveniência por entidade
reforça I3/I4; frame-de-mundo próprio reforça I7/I8 (não alugar a verdade espacial).

## 5. Recomendação (prioridade)

1. **✅ Local Action Planner** (`internal/world/nav`, commit desta sessão) — o I1 materializado.
2. **🔵 Language-driven scene editing** (AssetsRP) — destrava "construir o mundo por linguagem".
3. **🔵 Frame-de-mundo + saber≠ver** — o gap central do multiagente espacial.
4. **🔵 Road-network topológica** — estradas orgânicas determinísticas.

Cada fase: **aditiva + avaliada** (benchmark antes/depois). O Cosca minera a IDEIA, não a plataforma.

---

*Os 3 projetos validam a tese do Cosca: o mundo é do sistema (determinístico, gate I1); o LLM propõe
intenção, nunca coordenada. O Cosca compõe sobre `world`/`worldmodel`/`procgen`/`proposal`/`gate` e
continua distinto — seleciona, não copia.*
