# Content Knowledge Map — Pacotes Novos (Batch 2)

> Mineração profunda dos pacotes adicionados pelo Don (2026-08-24).
> 6 pacotes, ~19 GB de conteúdo. Foco: o que ENSINA o Cosca a construir um mundo.
> Framework: inventário → estrutura → técnicas → princípios → capacidades.

---

## 1. INVENTÁRIO FÍSICO

| Pacote | Tamanho | Arquivos | Foco |
|--------|---------|----------|------|
| VintageRoom | 6.4 GB | 560 | Ambiente interno + HDRIs |
| AbandonedFactory | 4.1 GB | 1021 | Atmosfera industrial + audio + VFX |
| Brushify | 2.5 GB | 421 | Landscape procedural |
| BrutalistLevelKit | 1.7 GB | 803 | Ambiente modular institucional |
| Isometric_World | 553 MB | 316 | Cidade isométrica grid-based |
| BlinkAndDashVFX | 73 MB | 142 | Niagara VFX (replay) |
| **TOTAL** | **~19 GB** | **3263** | |

---

## 2. ANÁLISE POR PACOTE

### 2.1 Brushify — Landscape procedural (MAIS ALINHADO com Mundo Cosca)

```
Brushify
├── Materials/ (350)
│   ├── M_Landscape          — landscape master
│   ├── MI_Landscape_*       — instâncias por bioma (1017/2041/4081/505)
│   ├── M_Foliage_Master     — foliage master
│   ├── M_Grass_Master       — grass master
│   ├── M_NearMesh_Master    — meshes de perto
│   ├── M_DistanceMesh       — meshes de distância (LOD)
│   └── VirtualTextures: VT_Landscape (RVT streaming)
├── DistanceMeshes/ (17)
│   └── MI_Arctic_*_Snow     — instâncias por bioma (arctic, etc)
├── Procedural/ (20)
│   ├── FT_JuniperBush_*     — Foliage Types (Large/Medium/Small)
│   ├── FT_Rock_*, FT_Cliff_* — Foliage Types (rocks/cliffs)
│   └── Bushes/Cliffs_*      — Foliage Type collections
├── AlphaBrushes/ (1)        — brushes para sculpting
└── Maps/ (15)               — demo por bioma (arctic, desert, grasslands)
```

**TÉCNICAS-OURO:**
- **MF_LandscapePOM** — Parallax Occlusion Mapping (relevo em material, sem geometria)
- **MF_DistanceFade / MF_DistanceShrink** — Fade/encolhimento por distância (LOD em material)
- **MF_LandscapeBlend / Layers / TilingAndDistance** — blending de camadas, tiling
- **MF_RVT_Landscape** — Runtime Virtual Texture (streaming de textura do landscape)
- **Virtual Texture Landscape** — VT_Landscape para grandes áreas sem estourar memória

### 2.2 AbandonedFactory — Atmosfera industrial (mais rico em VFX)

```
AbandonedFactory
├── Environment/ (552)
│   ├── sm_Brick_*       — tijolos (06 variantes)
│   ├── sm_CartonBox_*   — caixas de papelão
│   ├── sm_Chimney_*     — chaminés
│   ├── sm_Curb_*        — calçadas
│   ├── sm_ConcretePlates_*
│   └── (curbs, pipes, ruins)
├── Audio/ (96)
│   ├── S_Ambient_* (+__Cue)   — ambientes (substância)
│   ├── S_AirVent_*, S_CircularSaw_*  — mecânicos
│   └── SA_Ambient_* / SC_Ambient_* (ambient/cue)
├── Effects/ (89)
│   ├── ns_Mist_*           — névoa (LitSmall/LitLarge/UnlitSmall/UnlitLarge)
│   ├── ns_ChimneySmoke_*   — fumaça de chaminé (S/M/L)
│   ├── ns_ManholeSteam_*   — vapor de bueiro
│   ├── ns_Footprint_*_SplashMudWater — pegadas em lama/água
│   ├── ns_InteractiveFoliage_*        — foliage que reage a força
│   └── bp_Mist_*           — blueprints de névoa
├── Decals/ (8)
│   └── decals de sujidade/quebra (broken, dirt)
├── BaseMaterials/ (44)     — base PBR (metais leitosos, etc)
└── Landscape/ (22)
```

**TÉCNICAS-OURO:**
- **Sigilo de escala por VFX**: Mist/Smoke/Steam têm variantes Small/Medium/Large
  → o Cosca deve escalar efeitos por contexto (chaminé grande = fumaça grande)
- **Interactive Foliage com Force** — foliage reage à interação (mundo vivo)
- **Decals de sujidade** — quebra/quebra de ambiente sem novos meshes
- **Audio com Cue separado** — SA (sorcerer)/SC (sonnerie) organização

### 2.3 BrutalistLevelKit — Ambiente modular institucional

```
BrutalistLevelKit
├── Models/ (254)
│   ├── SM_Fol_* (Fern, Grass_Set)  — foliage
│   ├── SM_Hallway1_* (15)          — corredores modulares
│   ├── SM_Stairs_* (13)            — escadas
│   ├── SM_Ground_* (12)            — pisos
│   ├── SM_Pipe/Scaf/Pillar/Block1  — infraestrutura modular
│   ├── SM_Debris/Part/File/Sign    — detalhes
│   └── SM_Door/Office/Room1/Industrial1
├── Blueprints/ (84)
├── Materials/ (125) — decals, emissive, volumetric fog, fabric, outline
└── Maps/Demo (126)
```

**TÉCNICAS-OURO:**
- **Construção modular** — combinação de peças (corredor + escada + porta)
- **SM_Part/Block1** — peças genéricas compactáveis (procedural)
- **M_Outline / M_EmissiveSurface** — estilo + emissive
- **M_VolumetricFog** — névoa volumétrica

### 2.4 Isometric_World — Cidade isométrica grid-based

```
Isometric_World
├── Core/
│   └── SM_Temple_* (Cube_01-04, Floor_Tile 1m/2m, etc)
├── Sky_Temple/ (templo modular)
└── Demo/ (313 assets)
```

**TÉCNICAS-OURO:**
- **Grid-based tiles** (1m/2m) — construção por célula (ideal para cidade em grid)
- **Cube modular** — blocos empilháveis em grid
- **Isométrico** — perspectiva fixa (render eficiente, sem LOD complexo)

### 2.5 VintageRoom — Ambiente interno + HDRI

```
VintageRoom
├── HDRIs/ — HDR_01, solitude_night_4k (iluminação HDRI)
├── Materials, Meshes, Textures, Blueprint, Maps
└── + FBX/TGA/BMP (assets de origem)
```

**TÉCNICAS-OURO:**
- **HDRI de iluminação** — environment map 360° (4k) para luz ambiente realista
- **Assets de origem FBX/TGA** — pipeline de importação de assets reais

### 2.6 BlinkAndDashVFX — Niagara replay

```
BlinkAndDashVFX
├── VFX_Niagara, Meshes, Materials, Textures
├── BP_Niagara_Replay — replay de Niagara systems
└── Overview.umap
```

**TÉCNICAS-OURO:**
- **Niagara Replay** — gravar/reproduzir VFX (para cinemática/determinismo)

---

## 3. GENERALIZAÇÃO

### 3.1 Técnicas → Princípios → Capacidades

| Observado | Padrão | Princípio | Capacidade Cosca |
|-----------|--------|-----------|------------------|
| Brushify M_Landscape + MI_* | Bioma por instância | Biome Material Instancing | landscape.biome_material |
| Brushify MF_LandscapePOM | Relevo em shader | Displacement-Driven Detail | landscape.parallax_occlusion |
| Brushify MF_DistanceFade/Shrink | LOD por material | Distance-Based Representation | rendering.distance_fade |
| Brushify VT_Landscape (RVT) | Streaming de textura | Virtual Texture Streaming | landscape.virtual_texture |
| Brushify MF_LandscapeBlend/Layers | Camadas misturadas | Surface Composition | landscape.layer_blending |
| AbandonedFactory ns_Mist/Smoke S/M/L | Escala de VFX por contexto | Context-Scaled Effects | vfx.scale_by_context |
| AbandonedFactory InteractiveFoliage Force | Foliage reativo | Reactive Environment | vegetation.reactive_foliage |
| AbandonedFactory Audio Ambient + Cue | Atmosfera via som | Ambience via Audio | audio.environment_ambience |
| AbandonedFactory Decals | Sujidade sem geometria | Surface Detail Overlay | rendering.decal_overlay |
| Brutalist SM_Part/Block/Corridor | Peças modulares | Modular Composition | building.modular_kit |
| Isometric SM_Temple tiles 1m/2m | Grid construction | Grid-Based Building | building.grid_construction |
| VintageRoom HDRI | Iluminação environment | IBL Lighting | lighting.hdri_environment |
| BlinkAndDash Niagara Replay | VFX gravável | Deterministic VFX Replay | vfx.replay |

### 3.2 O que os pacotes ENSINAM o Cosca

**Brushify** → como construir um **terreno procedural** com biomas, camadas, RVT, POM. É a peça que faltava para o Cosca gerar terrain real.

**AbandonedFactory** → como dar **atmosfera viva** a um mundo: névoa, fumaça, vapor, som ambiente, sujidade, foliage reativo. Transforma mundo estático em mundo que respira.

**BrutalistLevelKit** → como **compor estruturas modulares** (corredor + escada + porta). Fundamental para gerar edifícios COMO ENTIDADES compostas.

**Isometric_World** → como **construir em grid** (tiles 1m/2m). Perfeito para cidades procedurais determinísticas e renderização eficiente.

**VintageRoom** → **iluminação HDRI** e pipeline de assets de origem.

**BlinkAndDashVFX** → **replay determinístico de VFX**.

---

## 4. CAPACIDADES COSCA NOVAS IDENTIFICADAS

```
landscape.biome_material      — instância de material por bioma
landscape.parallax_occlusion  — relevo via POM
landscape.virtual_texture     — streaming RVT para grandes áreas
rendering.distance_fade       — fade/shrink por distância (LOD material)
rendering.decal_overlay       — sujidade/quebra via decals
vegetation.reactive_foliage   — foliage que reage a força
audio.environment_ambience    — som ambiente como camada de atmosfera
vfx.scale_by_context          — efeitos escalados ao contexto
vfx.replay                    — replay determinístico de VFX
building.modular_kit          — composição por peças modulares
building.grid_construction    — construção em grid (tiles)
lighting.hdri_environment     — iluminação via HDRI (IBL)
```

### Sinergia com o Mundo Cosca
```
landscape.biome_material
    + landscape.parallax_occlusion
    + landscape.virtual_texture
    + rendering.distance_fade
    = landscape.procedural (o Cosca gera terrain real)

vfx.scale_by_context
    + audio.environment_ambience
    + vegetation.reactive_foliage
    + rendering.decal_overlay
    = world.atmosphere (mundo que respira)

building.modular_kit
    + building.grid_construction
    = building.procedural (edifícios como entidades compostas)
```

---

## 5. LACUNAS DE CONHECIMENTO

| Lacuna | Fonte que responde | Prioridade |
|--------|--------------------|------------|
| Como gerar terrain procedural com biomas? | Brushify | Alta |
| Como aplicar POM para relevo? | Brushify MF_LandscapePOM | Alta |
| Como usar RVT para grandes landscapes? | Brushify VT_Landscape | Alta |
| Como compor edifícios modulares? | BrutalistLevelKit | Alta |
| Como dar atmosfera viva (névoa/fumaça/steam)? | AbandonedFactory | Alta |
| Como construir cidades em grid? | Isometric_World | Média |
| Como usar HDRI para iluminação? | VintageRoom | Média |
| Como fazer replay de VFX determinístico? | BlinkAndDashVFX | Média |

---

## 6. O QUE ISSO HABILITA NO COSCA

Com Brushify (terrain) + AbandonedFactory (atmosfera) + Brutalist (modular) +
Isometric (grid) + VintageRoom (HDRI) + BlinkAndDash (replay), o Cosca agora
tem as peças para o pipeline completo:

```
Dados reais (OSM)
    ↓
World Model (entidades semânticas)      ✓ PRONTO
    ↓
Terrain procedural (Brushify)           ← NEW
    ↓
Atmosfera viva (AbandonedFactory)       ← NEW
    ↓
Edifícios modulares (Brutalist)         ← NEW
    ↓
Cidades em grid (Isometric)             ← NEW
    ↓
Iluminação (VintageRoom HDRI)           ← NEW
    ↓
Unreal → Mundo Cosca
```

O Don adicionou exatamente as peças que faltavam: **como representar visualmente
o que o Cosca semanticamente já entende.**

---

## 7. EVIDÊNCIA vs INFERÊNCIA

### FACT (confirmado nos assets)
- Brushify: 350 materiais, landscape master, RVT, POM, distance fade, 6 biomas
- AbandonedFactory: 552 env, 96 audio, 89 effects (mist/smoke/steam/foliage), decals
- BrutalistLevelKit: 254 models modulares, 84 blueprints, 125 materials
- Isometric_World: tiles 1m/2m, cubes modulares, temple
- VintageRoom: HDRIs (4k), FBX/TGA de origem
- BlinkAndDashVFX: Niagara replay

### MEASURED
- 6 pacotes, ~19 GB, 3263 arquivos
- Brushify 15 maps de demo (biomas)

### INFERRED
- Brushify usa RVT para streaming (baseado em VT_Landscape files)
- Brutalist modular kit combina peças via blueprints
- AbandonedFactory VFX têm variantes (S/M/L) para LOD
- VintageRoom HDRI para IBL (env map 360°)

### PROFILE
- Engine: UE 5.8 (Nanite, RVT, POM, Niagara)
- Landscape: procedural + RVT streaming
- Atmosfera: VFX Niagara + audio atmosférico
- Construção: modular + grid
- Iluminação: HDRI/IBL

### DECISION
- Usar RVT para grandes landscapes (não textures carregadas)
- Usar POM para detalhe (não geometria extra)
- Escalar VFX por contexto (S/M/L)
- Compor edifícios modularmente (não meshes únicos)
- Construir cidades em grid (determinístico)
- Iluminar via HDRI (IBL)
