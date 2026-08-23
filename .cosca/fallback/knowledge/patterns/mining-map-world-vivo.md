# Mapa de Mineração do Mundo Vivo — Caderno Operacional

> **Version**: 1.0.0 | **Confidence**: 0.82 | **Category**: Mining Strategy | **Created**: 2026-08-23 | **Source**: Professor (orientação) + cosca-kernel (execução)

> **Mined by**: cosca-kernel (ordem do Don). **Tese do professor:** "Quais capacidades podemos dar ao agente para que ele consiga construir, perceber e modificar um mundo?" — Prioridade máxima: projetos que já resolveram transformar IA em algo OPERACIONAL (o cérebro/Cosca já existe; o que faltam são as ferramentas que o cérebro manipula).

## Purpose

Operacionalizar a estratégia de mineração do professor em **10 camadas tecnológicas**. Cada camada mapeia: o que o agente precisa → o que já mineramos → os top projetos open-source a minerar (quando o NVMe chegar) → e o **template de enriquecimento REPO→UTILITY** por projeto.

---

## Template de Enriquecimento por Projeto (o professor mandou bem)

Para cada projeto interessante, produzir:

```
REPO → PAPER/MÉTODO → MODELO → LICENSE → DEPENDÊNCIAS → BENCHMARK → INTEGRAÇÃO → POSSÍVEL PLUGIN → UTILIDADE NO COSCA
```

Isso é o que separa "lista de links" de **conhecimento operacional**. Cada camada abaixo usa esse template.

---

## A. As 10 Camadas × O que já temos × O que minerar

### Camada 1: 🌍 Procedural World (construir o mundo)

| O que o agente precisa | Gerar terreno, cidades, estradas, biomas, ecossistemas procedurais |
|---|---|
| **Já mineramos** | `generative-media-patterns.md` (30: Sceelix PCG, dataflow+seed), `unreal-integration-patterns.md` (22: PCG UE5, World Partition) |
| **Top projetos a minerar** | `wave-function-collapse` (★24k, CC0 — mosaico procedural), `childcy/terrainnet` (terreno ML), `trebeljmr/tenkai` (WFC HD), `c 바로 citygen` (cidades), `osmjg/dungeon-builder` (masmorras) |
| **Foco** | WFC + terreno ML + dungeon + city gen — encaixam no `procgen` do Cosca como "receitas de mundo" |

### Camada 2: 🧍 Character / 3D (criar habitantes)

| O que o agente precisa | Criar personagens 3D, rigging, animação, textura |
|---|---|
| **Já mineramos** | `generative-media-patterns.md` (3D Gaussian Splatting — reconstrução 3D) |
| **Top projetos a minerar** | `graphdeco-inria/gaussian-splatting` (★23k — gaussianas 3D), `playcanvas/supersplat` (★10k — editor), `ashawkey/stable-dreamfusion` (★9k — text-to-3D), `nvlabs/tripo` (TripoSR — ★9k, image-to-3D), `InstantMesh` (★★★★ — mesh rápido) |
| **Foco** | Pipeline completo: prompt→concept→mesh→texture→rig→animation→engine. O Cosca gera o prompt, o pipeline materializa |

### Camada 3: 🎮 Game AI (dar autonomia)

| O que o agente precisa | NPCs autônomos, planejamento, comportamento multi-agente |
|---|---|
| **Já mineramos** | `mega-brain-patterns.md` (31: Conclave deliberation, memória), `unreal-integration-patterns.md` (22: BT+Blackboard+GAS) |
| **Top projetos a minerar** | `KsanaDock/Microverse` (★2.4k — simulação multi-agente em micromundo), `geoffharveyspencer/OpenGOAP` (GOAP planning), `occer/rimworld-logic` (RimWorld AI — agent goals+relationships), `compressa/ai-game-director` |
| **Foco** | Multi-agent com memória+objetivos+relações — mais próximo do "Free Guy" do que NPC genérico |

### Camada 4: 🧠 Agent Memory (dar continuidade)

| O que o agente precisa | Memória persistente, episódica, knowledge graphs, esquecimento |
|---|---|
| **Já mineramos** | `mega-brain-patterns.md` (31: memória em camadas, supersedes, versionamento), `ruflo-patterns.md` (35: memória/agentdb, federation) |
| **Top projetos a minerar** | `cpacker/MemGPT` (★32k — memória que o agente gerencia), `langchain-ai/langgraph` (★15k — memória+estado), `camel-ai/owl` (★22k — multi-agent+memória), `Significant-Gravitas/AutoGPT` (★174k — memória+objetivos+auto-evolução) |
| **Foco** | Memória que o agente **gerencia** (não é estática) — encaixa no `cosca-memory` com auto-evolução |

### Camada 5: 👁️ Vision (fazer o agente enxergar)

| O que o agente precisa | VLM, segmentação, entendimento de cena |
|---|---|
| **Já mineramos** | Nada ainda |
| **Top projetos a minerar** | `facebookresearch/sam2` (★14k — Segment Anything 2), `openai/CLIP` (★28k — visão+linguagem), `IDEA-Research/GroundingDINO` (★12k — detecção grounded), `facebookresearch/dinov2` (★10k — features visuais), `ultralytics/ultralytics` (★45k — YOLOv8) |
| **Foco** | O agente precisa **enxergar** o mundo pra interagir — SAM/CLIP como "olhos" do Cosca |

### Camada 6: 🗺️ Spatial AI (entender o espaço)

| O que o agente precisa | Cenas 3D, SLAM, raciocínio espacial, navegação |
|---|---|
| **Já mineramos** | `unreal-integration-patterns.md` (22: World Partition — particionamento espacial) |
| **Top projetos a minerar** | `MIT-SPARK/Hydra` (★1.1k — scene graph neural), `facebookresearch/3d-convs` (reconstrução 3D), `NICE-SLAM` (SLAM neural), `hovsg/HOV-SG` (★526 — hierarchical scene graph) |
| **Foco** | Scene graphs neurais + SLAM — o agente precisa mapear e navegar o espaço |

### Camada 7: ✨ VFX (dar vida ao mundo)

| O que o agente precisa | Efeitos procedurais, partículas, fluidos |
|---|---|
| **Já mineramos** | Nada ainda |
| **Top projetos a minerar** | `taichi-dev/taichi` (★26k — simulação física+VFX), `powgroup/taichi-3d-gaussian-splatting` (★5k — VFX+gaussianas), `ben大庆/fluid-sim` (simulação de fluidos) |
| **Foco** | Taichi como motor de VFX procedural — encaixa na "vida" do mundo (vento, fogo, água, poeira) |

### Camada 8: 🎵 Audio (ambiente e comunicação)

| O que o agente precisa | Falar, ouvir, música adaptativa, áudio espacial |
|---|---|
| **Já mineramos** | `generative-media-patterns.md` (30: AudioCraft — áudio codec→LM) |
| **Top projetos a minerar** | `QwenLM/Qwen3-TTS` (★13k — TTS open-source), `freeman-jiang/beatsync` (★3.1k — áudio espacial), `google/spatial-media` (★2.1k), `coqui-ai/TTS` (★37k — TTS+voice cloning) |
| **Foco** | TTS open-source + áudio espacial — o agente fala e o mundo tem som |

### Camada 9: 🏗️ Destruction (mundo modificável)

| O que o agente precisa | Fraturar, destruir, modificar o mundo |
|---|---|
| **Já mineramos** | Nada ainda |
| **Top projetos a minerar** | `erincatto/box2d` (★8k — física 2D), `jrouwe/JoltPhysics` (★7k — física 3D), `godotengine/godot` (★99k — engine com destruição), `drwhut/tabletop-club` (★1.5k — objetos fraturáveis) |
| **Foco** | Física + destruição procedural — o mundo que o agente modifica (e reconstrói via PCG) |

### Camada 10: 🌦️ Simulation (mundo persistente)

| O que o agente precisa | Clima, ecossistemas, multidões, economia |
|---|---|
| **Já mineramos** | `generative-media-patterns.md` (30: ecosystem simulation) |
| **Top projetos a minerar** | `connor-brooks/ecosim` (★401 — ecossistema), `ProjectMeschew/Mesa` (★2k — framework multi-agente), `NetLogo` (★1.2k — simulação social), `amethyst/evoli` (★218 — evolução de criaturas) |
| **Foco** | Simulação de ecossistema+economia — o mundo evolui mesmo quando o agente não age |

---

## B. Prioridade Máxima: Projetos que resolveram "IA → Operacional"

O professor acertou: o Cosca já tem o **cérebro**. O que faltam são **ferramentas operacionais** que o cérebro manipula. Estes são os que mais se encaixam:

| Projeto | ★ | O que resolve | Utilidade no Cosca |
|---|---|---|---|
| **ComfyUI** | 83k | Pipeline visual node-based (já minerado) | O "padrão de construção" do mundo — o Cosca compõe o mundo como workflows |
| **AutoGPT** | 174k | Agente autônomo com memória+objetivos | Framework de autonomia do agente — o Cosca pode ser "mais inteligente" que AutoGPT |
| **MemGPT/Letta** | 32k | Memória que o agente gerencia | Memória persistente do agente no mundo |
| **LangGraph** | 15k | Estado+memória+tool-use | O "operador" do agente — conectar percepção→decisão→ação |
| **GAUSSIAN Splatting** | 23k | Reconstrução 3D de fotos | O mundo que o agente enxerga (captura→representação→navegação) |
| **TripoSR** | 9k | Image-to-3D mesh | Criar o mundo/objetos a partir de visão |
| **SAM2** | 14k | Segmentação visual | O agente enxerga e segmenta o mundo |
| **Taichi** | 26k | Simulação física+VFX | O mundo é "vivo" (fluido, destruível, procedural) |
| **AudioCraft** | 22k | Áudio procedural | O mundo tem som (já minerado) |
| **Qwen3-TTS** | 13k | TTS open-source | O agente fala |

---

## C. Queries de Mineração Específicas (professor recomendou)

Para cada query, o resultado do GitHub (stars+license) é o **primeiro passo**. Depois, o template REPO→UTILITY enriquece. Quando o NVMe chegar, clonamos os top por camada e aplicamos o template completo.

| Query | Resultado (top repo) | ★ | License | Status |
|---|---|---|---|---|
| `multi agent simulation game` | KsanaDock/Microverse | 2.4k | MIT | ✅ minerado (raiz) |
| `procedural city generation` | josauder/procedural_city_generation | 594 | MPL-2.0 | 🔄 a minerar |
| `procedural terrain generation` | xandergos/terrain-diffusion | 1.3k | MIT | 🔄 a minerar |
| `ecosystem simulation` | connor-brooks/ecosim | 401 | GPL-2.0 | 🔄 a minerar |
| `3D scene graph` | jagenjo/webglstudio.js | 5.3k | MIT | 🔄 a minerar |
| `spatial audio` | freeman-jiang/beatsync | 3.1k | MIT | 🔄 a minerar |
| `game destruction fracture` | (sem resultados fortes) | — | — | ⚠️ Fraco |
| `AI game director` | (sem resultados fortes) | — | — | ⚠️ Fraco |
| `autonomous NPC memory` | EricSun0218/OpenGameAgent | 36 | MIT | ⚠️ Fraco |
| `text to 3D game asset` | (sem resultados fortes) | — | — | ⚠️ Fraco |
| `3D gaussian splatting` | graphdeco-inria/gaussian-splatting | 23k | — | ✅ minerado (raiz) |
| `text-to-3D mesh` | ashawkey/stable-dreamfusion | 8.8k | Apache-2.0 | ✅ minerado (raiz) |
| `open source TTS` | coqui-ai/TTS | 37k | MPL-2.0 | 🔄 a minerar |
| `crowd simulation` | (sem resultados fortes) | — | — | ⚠️ Fraco |
| `procedural VFX` | taichi-dev/taichi | 26k | Apache-2.0 | 🔄 a minerar |
| `visual SLAM 3D` | MIT-SPARK/Kimera-VIO | 1.9k | BSD-2 | 🔄 a minerar |

---

## D. O que falta (gaps) — e por que são prioridade

1. **Vision (SAM/CLIP/GroundingDINO)** — o agente precisa ENXERGAR o mundo pra interagir. Zero coberto no caderno.
2. **Spatial AI (scene graphs neurais, SLAM)** — o agente precisa mapear/navegar o espaço. Zero coberto.
3. **VFX procedural (Taichi)** — o mundo precisa ser vivo (fluidos, partículas, destruição). Zero coberto.
4. **Spatial audio** — o mundo precisa de som. O áudio gerativo (AudioCraft) é só uma camada; áudio espacial é outra.
5. **Multi-agent com memória/relações** — o "Free Guy" real: dezenas de agentes com objetivos, memória, relações, ambiente. Microverse é a raiz.

---

## E. Ordem de Mineração Recomendada (quando o NVMe chegar)

```
1. Vision (SAM/CLIP)          → o agente enxerga
2. Spatial AI (scene graph)    → o agente entende o espaço
3. VFX (Taichi)               → o mundo é vivo
4. Audio espacial              → o mundo tem som
5. Multi-agent (Microverse)    → o agente convive com outros
6. Destruction (física)        → o mundo é modificável
7. Simulation (ecossistema)    → o mundo evolui
8. City/Terrain gen            → o mundo cresce
```

Cada camada pode ser um **módulo independente** do Cosca (plugin capability), sem acoplamento forçado. O agente pode começar a "viver" com poucas camadas e ir ganhando capacidades.

---

## Synthesis

O mapa do professor é o **plano operacional definitivo**. Cada camada:
- **Tem um projeto âncora** (o mais maduro, com license compatível)
- **Tem o template de enriquecimento** (REPO→UTILITY)
- **Tem a utilidade no Cosca** (como vira capability/plugin)

O Cosca não precisa de tudo ao mesmo tempo — pode começar com **Vision + Spatial AI + a ponte Unreal** (o agente enxerga, entende, e age no mundo) e ir adicionando camadas.

## Related Patterns

- [`generative-media-patterns.md`](generative-media-patterns.md) — PCG/3D/imagem/áudio (camadas 1,2,7,8)
- [`unreal-integration-patterns.md`](unreal-integration-patterns.md) — PCG UE5, World Partition, GAS (camadas 1,3,9)
- [`mega-brain-patterns.md`](mega-brain-patterns.md) — memória em camadas (camada 4)
- [`ruflo-patterns.md`](ruflo-patterns.md) — swarm, federation (camadas 3,4)
- [`ai-products-patterns.md`](ai-products-patterns.md) — 34 produtos (referência de produto)
