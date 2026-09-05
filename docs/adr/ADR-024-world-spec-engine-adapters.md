# ADR-024: World Spec & Engine Adapters — o COSCA autorando mundos independente do motor (o "sistema nervoso")

> **Status:** Proposed | **Owner:** cosca-kernel + cosca-architecture | **Last Updated:** 2026-08-28
> **Revisão:** aguardando Don + cosca-cto. **Design aditivo — não quebra o Root.**
> **Referência (base):** auditoria do substrato (`internal/world`, `scene`, `gameengine`, `procgen`,
> `tdengine`, `asset`, `worldmodel`, `sciengine`, `evalgo`, `provenance`, `evolution`, `gate`) +
> `ADR-017` (híridização/cristalização, ratificado) + `ADR-021` (World Building) + `ADR-022` (Verificação & Proof).

---

## 0. Contexto — a correção que muda a ambição

A mineração do Roblox + a auditoria do substrato revelaram uma distinção importante, refinada na
conversa com o professor: **o COSCA não tem "80% de um World Studio pronto como produto".** Ele tem
**peças de infraestrutura desconectadas** (órgãos), mas **não tem o sistema nervoso** que as conecta.

A ambição correta **não é** "fazer um concorrente do Unreal". É:

> **fazer o COSCA entender e autorar mundos independentemente do motor que vai executá-los.**

E nesse desenho, Unreal/Roblox/Blender tornam-se **órgãos periféricos** (corpos), não o centro.

**O ponto de solda é o `WorldSpec`** — a língua comum. O COSCA emite "este é o mundo que quero
representar"; o adapter materializa aquele mundo no corpo.

---

## 1. O que o COSCA JÁ TEM (auditoria — órgãos existentes)

| Órgão (existe) | Papel no World Studio | Estado |
|---|---|---|
| `internal/world` (+ `world/nav`) | World Kernel semântico (entidades, terrain, relations, A*) | ✅ |
| `internal/worldmodel` | Living World (orquestrador, crença I4, TrustState) + **Adapter de FERRAMENTA** | ✅ |
| `internal/scene` | Scene graph (entidades, câmera, Visual Compiler) | ✅ |
| `internal/gameengine` | **ECS Go puro** (Scene→Entity→Component→System, JSON serializável) | ✅ |
| `internal/procgen` | Gerador procedural determinístico | ✅ |
| `internal/tdengine` | **3D** (parsers puros OBJ/glTF → MODELING/MATERIALS/.../RENDER) | ✅ |
| `internal/asset` | Asset Registry (content-addressable, provenance, não-destrutivo) | ✅ |
| `internal/sciengine` + `evalgo` | Avaliação (experimentos reprodutíveis + juiz=medição + gate) | ✅ |
| `internal/provenance` + `evolution` | Integridade (OBSERVED/CALCULATED/SIMULATED/GENERATED) + aprendizado | ✅ |
| `internal/gate` | Aprovação (máquina de estados por papel/transições) | ✅ |

**Atenção (subsídio):** o `worldmodel.Adapter` atual é **adapter de FERRAMENTA** (CLIP/SAM/Whisper —
percepção/subprocesso). O **Engine Adapter** (Unreal/Roblox/Blender como corpo) é OUTRA abstração,
**nova e fina** — não reusar a semântica de ferramenta.

---

## 2. A decisão — o sistema nervoso (WorldSpec + pipeline + adapters + loop)

### 2.1 O contrato canônico `WorldSpec`
- JSON, **versionado** (`schemaVersion`, padrão `internal/contracts`: bump aditivo).
- **Determinístico** (`seed` → mesma spec → mesmo mundo). **I1**.
- **Declarativo** (intenção estrutural, não vértice a vértice) — casa com o `gameengine`.
- **Com proveniência** (`source`/`class`, tool, prompt, model, seed, params). **I3/I4**.
- **Decisão de projeto:** **estender o `gameengine.Scene`** (já declarativo/JSON/ECS) com
  `terrain`/`navigation`/`environment`/`simulation`/`provenance`. **Não criar um tipo novo gigante.**

### 2.2 O pipeline de autoria (como um mundo nasce)
```
LLM (Intent)  →  Proposal  →  WorldPlanner  →  WorldSpec  →  Gate  →  Apply  →  Provenance
   propõe         struct          resolve         contrato     aprova     (commit      prova
   "cidade        (validada)      intenção→        canônico     (papel+    idempotente  (I3/I4)
    costeira"                     estado válido                transição) com ledger/  + evolution
                                                                I1/I2      version)
```
- **Regra de ouro (ADR-017 aplicado ao mundo):** o LLM **NUNCA escreve no mundo**. Emite `Intent`;
  o `WorldPlanner` (determinístico) **constrói o estado válido**; o `Gate` aprova; o `Provenance` prova;
  o engine executa; a simulação produz evidência.
- **Edição semântica:** "essa rua conecta o porto ao centro" → `Intent` relacional → o planner altera a
  **intenção estrutural**, não coordena vértice a vértice.

### 2.3 Engine Adapters (o corpo) — abstração nova e fina
```go
type EngineAdapter interface {
    Target() string                                   // "unreal" | "roblox" | "blender"
    Materialize(ctx context.Context, spec WorldSpec) (ExecutionHandle, error)
    // RunSimulate / Observe / Health... conforme o corpo
}
```
- `roblox-adapter` (1º alvo; design pronto no BIBLE): `AuthorityMode=Server`, rojo/wally, DataStore
  anti-dupe, validação de RemoteEvent.
- `unreal-adapter` / `blender-adapter` (posteriores): `internal/tdengine` já é o **ponto de entrada
  do Blender** (OBJ/glTF → malha).

### 2.4 O loop (a engine cognitiva de mundos)
```
AUTHOR → SIMULATE → OBSERVE → EVALUATE → MODIFY → (loop)
```
O que separa "gerar conteúdo" de "**experimentar mundos**":
- **OBSERVE** = `worldmodel` (crença com `TrustState`/`Uncertainty`, **I4**) — observação *qualificada*.
- **EVALUATE** = `evalgo`/`sciengine` — métrica pré-definida + significância + **gate** (nunca "parece melhor").
- **MODIFY** = `proposal`→`gate`→`apply`→`provenance` — **proof-gated**.

---

## 3. Invariantes (I1–I8) — veredito

- **I1 (determinístico):** `WorldSpec` com `seed` + `WorldPlanner` determinístico → mesmo mundo.
  **I1 reforçado.**
- **I2 (fail-closed):** `Gate` aprova/nega; `Apply` idempotente (ledger/version); divergência/dupe
  nunca "consertado" em silêncio. **I2 reforçado.**
- **I3/I4 (proveniência/epistemia):** `WorldSpec.provenance` + `OBSERVE` qualificado (`TrustState`).
  O mundo sabe o que sabe; nada entra por declaração. **I3/I4 reforçado.**
- **I5 (ledger):** `Apply` comita via `internal/durable/ledger` (fencing). **I5 preservado.**
- **I7 (isolamento):** motores são **corpos externos** (adapters); nada de runtime novo no núcleo.
  **I7 preservado.**
- **I8 (externo nunca autoridade):** o mundo pertence ao COSCA (`WorldSpec`); Unreal/Roblox/Blender
  são **targets**, não fontes de verdade. **I8 reforçado.**

---

## 4. O que REJEITAR (com convicção)

- **Editor 3D gigante / "Unreal clone".** O equivalente ao Blender/Unreal é uma **camada de autoria
  universal** (intenção→spec), não um editor de vértice.
- **Reimplementar física/render/nav.** Motores e `tdengine` já fazem; consumir via adapter.
- **LLM como editor.** O LLM propõe; o `WorldPlanner` constrói o estado válido. Nunca o LLM escreve
  no mundo. (Anti-I1.)
- **Cristalização automática.** Só cristalizar capacidade se passar nos 4 critérios do ADR-017.
- **Fazer do adapter de ferramenta o adapter de engine.** São abstrações distintas.

---

## 5. Recomendação (faseada, com evidência)

1. **Fase 1 (XS — contrato):** definir o `WorldSpec` canônico (estender `gameengine.Scene` +
   terrain/nav/env/simulation/provenance), documentado e com validação de schema/versionamento.
2. **Fase 2 (S — prova do nervo):** `WorldPlanner` para UM domínio (cidade costeira) + **1º adapter
   (`roblox-adapter`)** — até "colocar uma estrada ligando o porto ao centro" virar `WorldSpec` e
   materializar. Prova o pipeline sem construir a engine.
3. **Fase 3 (M — loop):** compor `SIMULATE→OBSERVE→EVALUATE→MODIFY` (sciengine+worldmodel+evalgo+proposal)
   num fluxo único, com medição + sign-off antes de promover.
4. Cada fase: **aditiva + avaliada** (régua `internal/performance`/`evalgo`), com o **gate do COSCA**
   (ADR-016/017) — evidência antes de promover.

---

*O que parecia "fazer um concorrente do Unreal" é, na verdade, "fazer o COSCA entender e autorar
mundos independentemente do motor". WorldSpec é a língua comum; os motores viram órgãos periféricos;
e o loop AUTHOR→SIMULATE→OBSERVE→EVALUATE→MODIFY transforma o COSCA num sistema que **experimenta**
mundos, não só gera conteúdo. O substrato já existe — o trabalho é o sistema nervoso.*
