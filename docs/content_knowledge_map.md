# Content Knowledge Map — Environment_Set

> Mineração profunda e sistemática do pacote de referência floresta.
> Objetivo: extrair conhecimento reutilizável pelo Cosca, não copiar implementação.

---

## 1. INVENTÁRIO FÍSICO

### 1.1 Resumo

| Categoria | Itens | Tamanho |
|-----------|-------|---------|
| Texturas | 274 | 2,939 MB |
| Material Instances | 115 | ~20 MB |
| Static Meshes | 88 | ~45 MB |
| Blueprints | 36 | 4 MB |
| Master Materials | 14 | ~2 MB |
| Imposters | 47 textures + 15 MI | ~50 MB |
| Maps | 3 | 158 MB |
| **TOTAL** | **523 arquivos** | **3.04 GB** |

### 1.2 Floresta

```
Trees (15 static meshes)
├── Fir (8 variants)
│   ├── Fir_01_Plant (estágio jovem)
│   ├── Fir_02_Small (pequena)
│   ├── Fir_03_Medium (média)
│   ├── Fir_04_Standalone (solitária)
│   ├── Fir_05_Standalone (variante)
│   ├── Fir_06_Forest (floresta — mais alta, fina)
│   ├── Fir_07_Forest (variante floresta)
│   └── Fir_08_Dead (morta)
│
└── Poplar (7 variants)
    ├── Poplar_01_Standalone
    ├── Poplar_02_Standalone
    ├── Poplar_03_Small
    ├── Poplar_04_Medium
    ├── Poplar_05_Standalone
    ├── Poplar_06_Forest
    └── Poplar_07_Forest
```

### 1.3 Ground Foliage

```
Ground Cover (26+ models)
├── SM_clover_01 (+ _flowery variant)
├── SM_fern_01_0, _01_1, _01_2
├── SM_grass_bush_01-04
├── SM_grass_bush_simple_01-05 (LOD)
├── SM_ground_foliage_01-04
├── SM_nettle_01_0, _01_1
├── SM_forest_heather_01
├── SM_forest_heather_simple_01 (LOD)
├── SM_crooked_root_01-05
├── SM_crooked_roots_small
├── SM_trunk_01, _02
└── SM_road_border_01-04
```

### 1.4 Rocks

```
Rocks (13 static meshes)
├── SM_Big_Rock_01-03
├── SM_Middle_Rock_01-03
├── SM_Small_Rock_01-06
└── SM_rock_01
```

### 1.5 Ground Materials

```
Ground Surfaces (47 Material Instances)
├── Forest
│   ├── forest_grass (6 variants)
│   ├── forest_leaves (3 variants)
│   ├── forest_moss (1 variant)
│   ├── forest_pebbles (1 variant)
│   └── forest_base (2 variants)
├── Rocks
│   └── ground_rocks (8 variants)
├── Dirt
│   └── ground_dirt (4 variants)
├── Sand
│   └── ground_sand (4 variants)
├── Moss
│   └── ground_moss (3 variants)
├── Wet Dirt
│   └── ground_wet_dirt (3 variants)
├── Roots
│   └── ground_roots (1 variant)
└── Big Pebbles
    └── ground_big_pebbles (1 variant)
```

---

## 2. ESTRUTURA DOS ASSETS

### 2.1 Tree Asset Structure

```
Fir_04_Standalone (exemplo)
├── Static Mesh (geometria)
│   ├── Trunk mesh
│   ├── Branch meshes (múltiplos)
│   └── Foliage meshes (múltiplos)
├── Materials
│   ├── M_Bark (tronco)
│   └── M_leaves (folhas)
├── Textures
│   ├── T_fir_bark_BC (base color)
│   ├── T_fir_bark_N (normal)
│   ├── T_fir_bark_MT_R_AO (packed)
│   ├── T_fir_leaves_BC
│   ├── T_fir_leaves_N
│   └── T_fir_leaves_MT_R_AO
└── LODs
    ├── LOD0: Fir_04_Standalone (full)
    ├── LOD1: Fir_04_Simple (reduced)
    └── LOD2: MI_Imposter_Fir_04 (billboard)
```

### 2.2 Texture Workflow

Pacote usa **packed textures** (3 canais em 1 textura):

| Suffix | Canais | Conteúdo |
|--------|--------|----------|
| `_BC` | RGB(A) | Base Color (+ Alpha opcional) |
| `_N` | RG(B) | Normal Map (tangent space) |
| `_MT_R_AO` | R=Metallic, G=Roughness, B=AO | Packed grayscale |
| `_MT_R_AO_H` | R=Meta, G=Rough, B=AO, A=Height | Packed + height |

**Por que packed?** Reduz número de texturas, diminui VRAM, melhora performance.

### 2.3 Rock Asset Structure

```
SM_Big_Rock_01
├── Static Mesh
├── Materials
│   ├── M_Rocks (padrão)
│   ├── M_UV_Free_Cover (triplanar)
│   └── M_Dynamic_Cover (auto-blend)
├── Blueprint (placement)
│   └── Big_Rock_01_Blueprint
└── Textures
    ├── T_rock_01_BC, _N, _MT_R_AO
    └── T_uvfree_rock_01_BC_H, _N, _MT_R_AO
```

---

## 3. RELAÇÕES

### 3.1 Mapa de Relações

```
Environment_Set
│
├── Tree
│   ├── uses → Bark Material (M_Bark)
│   ├── uses → Foliage Material (M_leaves)
│   ├── uses → LOD Material (M_leaves_LOD)
│   ├── uses → Imposter Material (M_Imposters)
│   ├── belongs → Vegetation Layer
│   ├── placed_on → Landscape
│   ├── affected_by → Lighting (Lumen GI)
│   ├── optimized_by → LOD System
│   ├── optimized_by → Imposters
│   └── varies_by → Species (Fir/Poplar)
│
├── Ground Foliage
│   ├── uses → Foliage Materials (M_Grass, M_leaves)
│   ├── belongs → Vegetation Layer
│   ├── placed_on → Landscape
│   ├── distributed_by → Foliage Tool
│   └── optimized_by → LOD (simple variants)
│
├── Rock
│   ├── uses → Rock Material (M_Rocks)
│   ├── uses → UV-Free Material (M_UV_Free_Cover)
│   ├── uses → Auto-Cover Material (M_Dynamic_Cover)
│   ├── placed_on → Landscape
│   ├── integrated_by → Auto-Cover (blends with terrain)
│   └── distributed_by → Blueprint Placement
│
├── Landscape
│   ├── uses → Terrain Shader (M_Terrain_Shader)
│   ├── uses → 47 Ground Material Instances
│   ├── blended_by → 8 Landscape Layers
│   ├── supports → Tree placement
│   ├── supports → Rock placement
│   ├── supports → Ground Foliage placement
│   └── affected_by → Lighting
│
├── Lighting
│   ├── type → Lumen GI (Dynamic)
│   ├── shadows → Virtual Shadow Maps
│   ├── reflection → Lumen Reflections
│   ├── ray_tracing → Hardware RT
│   └── affects → All materials (shading)
│
└── Performance
    ├── LOD → Tree (3 níveis)
    ├── LOD → Ground Foliage (simple variants)
    ├── Imposters → Trees (billboard fallback)
    ├── Culling → Virtual Shadow Maps (GPU-driven)
    ├── Streaming → World Partition
    └── Instancing → Foliage Tool (implicit)
```

### 3.2 Relações Cruzadas Críticas

```
Tree + Landscape + Material
   │
   ├── Árvore não existe isoladamente
   │   → está sempre placed_on Landscape
   │
   ├── Material da árvore depende de lighting
   │   → bark roughness afeta como luz interage
   │   → foliage alpha afeta transparência
   │
   ├── Landscape controla onde árvores crescem
   │   → layer weights determinam tipo de solo
   │   → solo determina espécie adequada
   │
   └── Integração visual depende de:
       → sombras no terreno
       → Contact shadows na base
       → AO nos galhos
       → scattering nas folhas
```

---

## 4. MATERIAIS

### 4.1 Master Materials

| Material | Tamanho | Função | Parâmetros-chave |
|----------|---------|--------|------------------|
| M_Bark | — | Casca de árvore | Base Color, Normal, Roughness |
| M_Bark_Blended | 22.9 KB | Casca com blending | + blend mask, + variation |
| M_leaves | — | Folhas | Base Color, Normal, Opacity, SSS |
| M_leaves_LOD | 156 KB | Folhas simplificadas | Versão reduzida para LOD |
| M_Grass_Material | — | Grama | Base Color, Wind, Opacity |
| M_Grass_Tick_Material | — | Grama densa | + densidade |
| M_Roots | — | Raízes | Base Color, Normal, Roughness |
| M_Rocks | — | Pedras | Base Color, Normal, Roughness, AO |
| M_rock_01 | — | Pedra variante | Parâmetros similares |
| M_UV_Free_Cover | 223 KB | Triplanar projection | World-aligned UVs |
| M_Dynamic_Cover | 215 KB | Auto-blend com terreno | + Distance blend, + projection |
| M_Imposters | — | Billboard trees | Two-sided, masked |
| M_Terrain_Shader | 176 KB | Landscape master | 8 layers, blending |
| M_Ground_1 | — | Ground variant | Layer-specific |

### 4.2 Técnicas de Material Encontradas

#### A. Packed Textures
```
OBSERVADO: 3 canais em 1 textura (MT_R_AO)
PADRÃO: Reduzir número de texturas
PRINCÍPIO: Texture Packing — optimization through channel compression
COSCA CAPABILITY: material.texture_packing
```

#### B. Triplanar/UV-Free Projection
```
OBSERVADO: M_UV_Free_Cover projeta textura sem UVs
PADRÃO: World-aligned texture projection
PRINCÍPIO: UV-Free Texturing — funciona em geometria complexa sem UV mapping
COSCA CAPABILITY: material.triplanar_projection
```

#### C. Auto-Cover / Dynamic Blend
```
OBSERVADO: M_Dynamic_Cover mistura rocha com terreno
PADRÃO: Distance-based material blending
PRINCÍPIO: Context-Aware Material — adapta aparência ao contexto
COSCA CAPABILITY: material.context_blend
```

#### D. Landscape Layer Blending
```
OBSERVADO: M_Terrain_Shader mistura 8 camadas
PADRÃO: Multi-layer material painting
PRINCÍPIO: Surface Composition — múltiplas superfícies em 1 terreno
COSCA CAPABILITY: landscape.layer_blending
```

#### E. Imposter/Billboard
```
OBSERVADO: 2D billboard para distância
PADRÃO: Distance-based representation swap
PRINCÍPIO: Hierarchical Visual Representation
COSCA CAPABILITY: rendering.impostor_system
```

---

## 5. LANDSCAPE

### 5.1 Arquitetura do Terreno

```
Landscape
├── M_Terrain_Shader (Master Material)
│   ├── 8 Landscape Layers
│   │   ├── Layer_1 (forest floor)
│   │   ├── Layer_2 (rocks)
│   │   ├── Layer_3 (dirt)
│   │   ├── Layer_4 (sand)
│   │   ├── Layer_5 (moss)
│   │   ├── Layer_6 (wet)
│   │   ├── Layer_7 (roots)
│   │   └── Layer_8 (pebbles)
│   │
│   └── 47 Material Instances
│       ├── MI_Ground_01-47
│       └── Cada uma = combinação de texturas
│
├── Heightmap (geometria do terreno)
├── Layer Info (pesos de blending)
└── Built Data (iluminação baked)
```

### 5.2 Técnicas de Landscape

#### A. Multi-Layer Blending
```
OBSERVADO: 8 camadas misturadas via painting
PADRÃO: Landscape layer system
PRINCÍPIO: Surface Composition — terreno não é uniforme
COSCA CAPABILITY: landscape.surface_composition
```

#### B. Texture Variation
```
OBSERVADO: 47 variants do mesmo tipo de superfície
PADRÃO: Multiple texture sets per surface type
PRINCÍPIO: Visual Variety — evita repetição
COSCA CAPABILITY: landscape.texture_variation
```

---

## 6. FOLIAGE

### 6.1 Sistema de Distribuição

```
Foliage Distribution
├── Tree Placement
│   ├── Fir: 8 variantes (plant → dead)
│   ├── Poplar: 7 variantes (standalone → forest)
│   └── Cada variante = contexto diferente
│
├── Ground Cover
│   ├── Clover (2 variants)
│   ├── Fern (3 variants)
│   ├── Grass (9 variants, 5 simple LOD)
│   ├── Ground Foliage (4 variants)
│   ├── Nettle (2 variants)
│   ├── Heather (2 variants, 1 simple LOD)
│   └── Roots (6 variants)
│
└── Distribution Method
    ├── UE5 Foliage Tool (paint-based)
    ├── Density per type
    ├── Radius constraints
    └── Slope/height filtering
```

### 6.2 Técnicas de Foliage

#### A. Growth Stage Variation
```
OBSERVADO: Fir tem 8 estágios (plant → dead)
PADRÃO: Multiple maturity levels per species
PRINCÍPIO: Life Cycle Representation — mesma espécie, múltiplas idades
COSCA CAPABILITY: vegetation.growth_stages
```

#### B. Context-Aware Placement
```
OBSERVADO: "Standalone" vs "Forest" variants
PADRÃO: Environment-dependent morphology
PRINCÍPIO: Context Shapes Form — árvore solita vs competição
COSCA CAPABILITY: vegetation.context_morphology
```

#### C. LOD Simplification
```
OBSERVADO: _simple_ variants para distância
PADRÃO: Geometry reduction per distance
PRINCÍPIO: Progressive Simplification
COSCA CAPABILITY: vegetation.lod_system
```

---

## 7. BLUEPRINTS

### 7.1 Rock Placement System

```
36 Blueprints
├── Big_Rock_01-03 × 3 versions = 9
├── Middle_Rock_01-03 × 3 versions = 9
├── Small_Rock_01-06 × 1 version = 6
└── Road_Border_01-04 × 3 versions = 12

Cada Blueprint:
├── Static Mesh (referência)
├── Randomização de:
│   ├── Scale (uniform aleatório)
│   ├── Rotation (Z-axis)
│   └── Position (offset mínimo)
└── Material override (opcional)
```

### 7.2 Padrão Extraído

```
OBSERVADO: Blueprints envolvem meshes com randomização
PADRÃO: Placement Blueprint Pattern
PRINCÍPIO: Controlled Randomness — variação dentro de limites
COSCA CAPABILITY: placement.controlled_randomness
```

---

## 8. OTIMIZAÇÃO

### 8.1 Pipeline de Rendering

```
Configuração (DefaultEngine.ini):
├── DX12 + SM6
├── Lumen GI (Dynamic Global Illumination)
├── Lumen Reflections
├── Virtual Shadow Maps
├── Hardware Ray Tracing
├── Mesh Distance Fields
├── Substrate Materials (UE5.4+)
└── World Partition (streaming)
```

### 8.2 Sistemas de LOD

```
LOD Pipeline (3 níveis):
│
├── LOD0: Full Detail
│   ├── Trunk: full geometry
│   ├── Branches: full geometry
│   ├── Foliage: full geometry
│   └── Materials: full quality
│
├── LOD1: Simplified
│   ├── _simple_ variants
│   ├── Reduced polygon count
│   └── Simplified materials
│
└── LOD2: Imposter
    ├── Billboard 2D
    ├── M_Imposters material
    ├── 2 textures (BC + N)
    └── Two-sided, masked
```

### 8.3 Técnicas de Performance

#### A. Virtual Shadow Maps
```
OBSERVADO: r.Shadow.Virtual.Enable=1
PADRÃO: GPU-driven shadow mapping
PRINCÍPIO: Scalable Shadow Quality — shadows que escalam com cena
COSCA CAPABILITY: rendering.virtual_shadows
```

#### B. Impostor System
```
OBSERVADO: 15 materials de billboard para árvores
PADRÃO: 2D fallback for distant 3D objects
PRINCÍPIO: Distance-Based Representation — trocar representação por distância
COSCA CAPABILITY: rendering.impostor_system
```

#### C. World Partition
```
OBSERVADO: WorldPartition settings ativos
PADRÃO: Automatic streaming grid
PRINCÍPIO: Load-on-Demand — só carrega o que está visível
COSCA CAPABILITY: world.partition_streaming
```

#### D. Texture Packing
```
OBSERVADO: MT_R_AO (3 canais em 1 textura)
PADRÃO: Channel packing for VRAM optimization
PRINCÍPIO: Memory Efficiency through compression
COSCA CAPABILITY: material.texture_packing
```

---

## 9. DEMO SCENE

### 9.1 Maps

| Map | Tamanho | Função |
|-----|---------|--------|
| Environment_Set_Map | 52 MB | Floresta principal |
| Environment_Set_Map_Overview | 56 MB | Vista aérea |
| Park_Overview_Map1 | 50 MB | Variação parque |

### 9.2 Composição da Cena

```
Forest Composition
│
├── Terrain (base)
│   ├── Heightmap
│   ├── 8 Material Layers
│   └── Painted weights
│
├── Trees (vertical elements)
│   ├── Fir (8 variants)
│   ├── Poplar (7 variants)
│   └── Distributed via Foliage Tool
│
├── Ground Cover (horizontal fill)
│   ├── Grass (dense)
│   ├── Fern (scattered)
│   ├── Clover (patches)
│   └── Heather (clusters)
│
├── Rocks (accents)
│   ├── Big Rocks (focal points)
│   ├── Middle Rocks (transition)
│   ├── Small Rocks (detail)
│   └── Auto-Cover (terrain blend)
│
├── Lighting
│   ├── Lumen GI (ambient)
│   ├── VSM (shadows)
│   └── RT reflections
│
└── Composition Rules
    ├── Trees clustered (não uniformes)
    ├── Rocks near tree bases
    ├── Ground cover denser near trees
    ├── Clearings for visual rest
    └── Depth through layering
```

### 9.3 Por que a Cena Parece Integrada

```
INTEGRATION FACTORS:
│
├── Material Continuity
│   ├── Terrain → ground materials
│   ├── Tree bark → earth tones
│   └── Foliage → ground color harmony
│
├── Shadow Integration
│   ├── Trees cast shadows on terrain
│   ├── Rocks cast shadows
│   └── Ground cover receives shadows
│
├── Scale Coherence
│   ├── Trees: 8-20m
│   ├── Rocks: 0.5-3m
│   ├── Ground cover: 0.1-0.5m
│   └── All proportional to real world
│
├── Density Gradient
│   ├── Dense near camera
│   ├── Sparse far away
│   └── Clearings for composition
│
└── Auto-Cover
    ├── Rocks blend into terrain
    ├── Roots emerge from ground
    └── No hard edges between objects
```

---

## 10. GENERALIZAÇÃO

### 10.1 De Observação para Capacidade

| Observado no Pacote | Padrão | Princípio | Capacidade Cosca |
|---------------------|--------|-----------|------------------|
| 8 estágios de crescimento | Growth stages | Life Cycle Representation | vegetation.growth_stages |
| Variante "Forest" vs "Standalone" | Context morphology | Environment Shapes Form | vegetation.context_morphology |
| Packed textures (MT_R_AO) | Channel packing | Memory Efficiency | material.texture_packing |
| Triplanar projection | UV-free texturing | Universal Surface Texturing | material.triplanar_projection |
| Auto-Cover blend | Terrain integration | Context-Aware Materials | material.context_blend |
| 8 landscape layers | Multi-layer blending | Surface Composition | landscape.layer_blending |
| 3-tier LOD | Progressive simplification | Distance-Based Representation | rendering.lod_system |
| Billboard imposters | 2D fallback | Hierarchical Visuals | rendering.impostor_system |
| Rock blueprints | Placement randomization | Controlled Randomness | placement.controlled_randomness |
| Virtual Shadow Maps | GPU-driven shadows | Scalable Shadow Quality | rendering.virtual_shadows |
| World Partition | Automatic streaming | Load-on-Demand | world.partition_streaming |
| 47 ground variants | Texture variety | Visual Diversity | landscape.texture_variation |

### 10.2 Princípios Generalizáveis

#### PRINCÍPIO 1: Hierarchical Visual Representation
```
OBJECTO NÃO É UM NÍVEL ÚNICO
→ LOD0 (full) → LOD1 (simplified) → LOD2 (imposter) → culled
→ Cada distância tem representação adequada
→ Cosca deve gerar múltiplos níveis de representação
```

#### PRINCÍPIO 2: Context-Aware Materiality
```
MATERIAL NÃO É ESTÁTICO
→ Adapta-se ao contexto (distância, vizinhos, terreno)
→ Auto-Cover: rocha se funde com terreno
→ Blended bark: casca varia com altura
→ Cosca deve gerar materiais contextuais
```

#### PRINCÍPIO 3: Controlled Randomness
```
VARIAÇÃO NÃO É CAOS
→ Dentro de limites específicos da espécie
→ Mesma seed = mesmo resultado
→ Cosca deve controlar variação via parâmetros
```

#### PRINCÍPIO 4: Surface Composition
```
TERRENO NÃO É UNIFORME
→ Múltiplas superfícies misturadas
→ Pintura manual ou procedural
→ Cosca deve compor superfícies
```

#### PRINCÍPIO 5: Life Cycle Representation
```
OBJETOS TÊM IDADE
→ Mesma espécie, múltiplas aparências
→ Seedling ≠ Mature ≠ Ancient
→ Cosca deve modelar envelhecimento
```

#### PRINCÍPIO 6: Integration Through Material Continuity
```
INTEGRAÇÃO VISUAL = CONTINUIDADE DE MATERIAL
→ Mesma paleta de cores
→ Sombras conectam objetos
→ Cosca deve manter harmonia visual
```

---

## 11. CAPACIDADES COSCA IDENTIFICADAS

### 11.1 Capacidades Primárias

| Capacidade | Input | Output | Dependencies |
|------------|-------|--------|--------------|
| `vegetation.generate_tree` | species, age, seed | Tree asset | — |
| `vegetation.growth_stages` | species, age_range | Multiple assets | generate_tree |
| `vegetation.context_morphology` | species, context | Context variant | generate_tree |
| `landscape.compose_terrain` | heightmap, layers | Terrain asset | — |
| `landscape.layer_blending` | layers, weights | Material | — |
| `landscape.texture_variation` | surface_type, count | Texture set | — |
| `material.texture_packing` | channels | Packed texture | — |
| `material.triplanar_projection` | textures | Projection material | — |
| `material.context_blend` | materials, rules | Blend material | — |
| `placement.controlled_randomness` | objects, rules | Placed objects | — |
| `rendering.lod_system` | asset, levels | LOD chain | — |
| `rendering.impostor_system` | 3D asset | Billboard | — |
| `rendering.virtual_shadows` | config | Shadow setup | — |
| `world.partition_streaming` | config | Streaming setup | — |

### 11.2 Capacidades Derivadas

| Capacidade | Derivada de | Função |
|------------|-------------|--------|
| `world.forest_generation` | vegetation + landscape + placement | Gerar floresta completa |
| `world.environment_composition` | terrain + vegetation + rocks + lighting | Compor cena integrada |
| `world.performance_optimization` | LOD + instancing + streaming | Otimizar cena grande |

### 11.3 Dependências entre Capacidades

```
vegetation.generate_tree
    │
    ├──→ vegetation.growth_stages
    │        │
    │        └──→ world.forest_generation
    │
    ├──→ vegetation.context_morphology
    │        │
    │        └──→ world.forest_generation
    │
    └──→ rendering.lod_system
             │
             └──→ rendering.impostor_system
                      │
                      └──→ world.performance_optimization

landscape.compose_terrain
    │
    ├──→ landscape.layer_blending
    │        │
    │        └──→ world.environment_composition
    │
    └──→ landscape.texture_variation
             │
             └──→ world.environment_composition

material.texture_packing
    │
    └──→ (todas as capacidades de material)

placement.controlled_randomness
    │
    └──→ world.forest_generation
```

---

## 12. ANÁLISE CRUZADA

### 12.1 Por que a Vegetação Parece Integrada ao Terreno

```
FATOR 1: Material Continuity
├── Bark usa tons terrosos (mesma paleta do ground)
├── Foliage usa verdes que harmonizam com grass
└── Resultado: árvore "pertence" ao terreno

FATOR 2: Shadow Integration
├── Árvores projetam sombras no terreno (VSM)
├── Sombras conectam visualmente objetos
└── Resultado: árvore "está no" terreno

FATOR 3: Auto-Cover
├── Rocks se fundem com terreno (distance blend)
├── Roots emergem do solo
└── Resultado: sem bordas duras

FATOR 4: Density Gradient
├── Ground cover mais denso perto das árvores
├── Espaços abertos para descanso visual
└── Resultado: distribuição natural
```

### 12.2 Como a Escala é Mantida

```
REFERÊNCIA: Human eye level ≈ 1.7m

Trees: 8-20m (5-12× human height)
Rocks: 0.5-3m (0.3-2× human height)
Ground cover: 0.1-0.5m (0.06-0.3× human height)

TODOS proporcionais ao mundo real.
NENHUM objeto com escala absurda.
Result: world feels real-scale.
```

### 12.3 Como a Densidade é Controlada

```
DENSITY RULES:
├── Trees: clustered (não uniformes)
│   ├── Forest variant: mais denso
│   └── Standalone: mais espaçado
│
├── Ground cover: hierarchical
│   ├── Grass: very dense
│   ├── Fern: medium
│   ├── Clover: patches
│   └── Heather: clusters
│
├── Rocks: strategic placement
│   ├── Big: focal points
│   ├── Middle: transition
│   └── Small: detail
│
└── Clearings: composição visual
    ├── Evita monotonia
    └── Guia o olhar
```

### 12.4 Como Rocks Não Parecem "Colocados"

```
AUTO-COVER SYSTEM:
├── M_Dynamic_Cover
│   ├── Proximity blend: rocha mistura com terreno
│   ├── Distance: 0-2m da base
│   └── Result: rocha "emerge" do solo
│
├── UV-Free Projection
│   ├── M_UV_Free_Cover
│   ├── Textura projetada (não UV-mapped)
│   ├── Funciona em qualquer geometria
│   └── Result: textura consistente
│
├── Placement
│   ├── Rocks perto de tree bases
│   ├── Partially buried (scale Y < 1)
│   └── Result: rocha parece natural
│
└── Material Match
    ├── Rock tones = ground tones
    ├── Same roughness range
    └── Result: visual continuity
```

### 12.5 Como a Cena Mantém Performance

```
OPTIMIZATION STACK:
│
├── LOD System (3 levels)
│   ├── LOD0: Full detail (close)
│   ├── LOD1: Simplified (medium)
│   └── LOD2: Imposter billboard (far)
│
├── Virtual Shadow Maps
│   ├── GPU-driven culling
│   ├── Only visible shadows computed
│   └── Scales with scene complexity
│
├── World Partition
│   ├── Automatic streaming grid
│   ├── Only loads visible cells
│   └── Memory-efficient
│
├── Texture Packing
│   ├── 3 channels per texture
│   ├── Less VRAM usage
│   └── Fewer texture binds
│
├── Impostor System
│   ├── 2D billboard for far trees
│   ├── 2 textures vs full mesh
│   └── Massive polygon reduction
│
└── Lumen GI
    ├── Screen-space + ray-traced
    ├── No baked lighting needed
    └── Dynamic but GPU-efficient
```

---

## 13. O QUE DEVE SER TESTADO PRIMEIRO

### Prioridade 1: Vegetação
1. **generate_tree** com 3 espécies (oak, pine, birch)
2. **growth_stages** para 1 espécie
3. **LOD system** básico (2 níveis)
4. **Imposter** para distância

### Prioridade 2: Materiais
1. **Texture packing** (Packed textures)
2. **Triplanar projection** para rocks
3. **Context blend** para auto-cover

### Prioridade 3: Landscape
1. **Layer blending** (2-3 camadas)
2. **Texture variation** para forest floor

### Prioridade 4: Composition
1. **Forest generation** (20-50 árvores)
2. **Rock placement** com randomização
3. **Ground cover** density control

---

## 14. MENOR VERTICAL SLICE RECOMENDADO

```
VS#1: Single Tree + Ground
├── 1 árvore procedural (oak, mature)
├── 1 ground material (forest_grass)
├── 1 rock com auto-cover
├── LOD básico (2 níveis)
└── Critério: árvore parece integrada ao terreno

VS#2: Tree Cluster
├── 5-10 árvores (2 espécies)
├── Ground cover (grass + fern)
├── 3-5 rocks
├── Density variation
└── Critério: cluster parece natural

VS#3: Forest Section
├── 50-100 árvores (3 espécies)
├── Full ground cover
├── Rocks distributed
├── LOD chain completa
├── Imposters
└── Performance: 30+ FPS em RX 6700 XT
```

---

## 15. LACUNAS DE CONHECIMENTO

| Lacuna | Impacto | Prioridade |
|--------|---------|------------|
| Como gerar normal maps procedurais? | Materiais sem normal = chatos | Alta |
| Como criar imposters automaticamente? | LOD manual não escala | Alta |
| Como implementar HISM para foliage? | Performance em florestas | Alta |
| Como fazer auto-cover procedural? | Integração rocha-terreno | Média |
| Como gerar heightmaps procedurais? | Terreno manual não escala | Média |
| Como implementar World Partition? | Streaming automático | Média |
| Como criar material instances dinâmicas? | Variação em runtime | Média |
| Como implementar wind animation? | Folhas estáticas = mortas | Baixa |

---

## 16. EVIDÊNCIA vs INFERÊNCIA

### FACT (confirmado no pacote)
- 15 tree meshes (8 Fir + 7 Poplar)
- 26+ ground foliage meshes
- 13 rock meshes
- 47 ground material instances
- 8 landscape layers
- 36 rock blueprints
- 3 maps
- Packed textures (MT_R_AO)
- Triplanar materials (M_UV_Free_Cover)
- Auto-cover materials (M_Dynamic_Cover)
- Imposter system (15 billboard materials)
- 3-tier LOD naming (_simple_, _NoBillboard)
- Lumen + VSM + RT config
- World Partition enabled

### MEASURED (quantificável)
- 523 arquivos, 3.04 GB
- 274 texturas, 2.939 GB
- 88 static meshes
- 115 material instances
- 14 master materials

### INFERRED (deduzido, não confirmado)
- Foliage Tool usado para distribuição (não PCG)
- Blueprints usam randomização de scale/rotation
- LOD pipeline é 3 níveis (baseado em naming)
- HISM pode estar em uso via Foliage Tool (não visível em arquivos)
- World Partition usa grid automático

### EVIDENCE (base para inferências)
- Naming patterns (_simple_, _01, _Forest, _Standalone)
- File structure (LOD variants próximos)
- Config settings (WorldPartition, VSM, Lumen)
- Blueprint naming (placement system)

### PROFILE (perfil técnico do pacote)
- Engine: UE 5.4+ (Substrate materials)
- Renderer: DX12 + SM6
- Lighting: Lumen GI + RT
- Shadows: Virtual Shadow Maps
- Streaming: World Partition
- Materials: Substrate (Strata)
- Target: Desktop high-end

### DECISION (decisões de design do pacote)
- Usar packed textures (não textures separadas)
- Usar imposters (não HLOD)
- Usar Foliage Tool (não PCG)
- Usar auto-cover (não manual placement)
- Usar Lumen (não baked lighting)
- Usar 3-tier LOD (não multi-LOD)
