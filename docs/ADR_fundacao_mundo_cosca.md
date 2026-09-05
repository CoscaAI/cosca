# ADR-XXX — Fundação do Mundo Cosca (World Model Semântico)

> **Autores:** cosca-kernel (Consigliere) + direção do professor
> **Status:** PROPOSTA — aguardando aprovação do Don
> **Contexto:** O professor mandou parar de adicionar features e consolidar as fundações.
> **Tema:** "Nenhum conhecimento do Mundo Cosca deve depender da Unreal."

---

## 0. Por que esta revisão agora

O Cosca já saiu da fase "provar que conversa com Unreal" e entrou na fase
"definir o que é um mundo". O risco arquitetural atual é claro:

> **O Cosca pode virar um "gerador de coisas para Unreal" sem perceber.**

Regras de mundo dentro de código específico da Unreal = dívida que cobra juros
depois. A árvore, o dia/noite e a cidade são substituíveis. **O modelo de
mundo, os contratos, identidade, coordenadas, determinismo e a separação
Cosca↔Unreal NÃO são.** Se estes estiverem errados agora, tudo que vier em
cima deles precisará ser refeito.

---

## 1. Diagnóstico: o que JÁ EXISTE (não duplicar)

O Cosca já tem mais fundação do que parece. Partes boas:

| Camada | Já existe | Arquivo |
|--------|-----------|---------|
| Geometria | `Vec3`, `Quat`, `AABB`, `Pose6DoF` | `internal/worldmodel/types.go` |
| Entidade (percepção) | `WorldEntity` (Type, Position, Rotation, Scale, BBox, Metadata) | `internal/worldmodel/types.go` |
| Relações | `SpatialRelation` (Subject/Object/Relation/Distance) | `internal/worldmodel/types.go` |
| Clima | `ClimateState` (Temperature, Wind, Rain, Snow, Fog, TimeOfDay, Season) | `internal/worldmodel/types.go` |
| Estado do mundo | `WorldState` (Entities, Relations, Climate, Step) | `internal/worldmodel/types.go` |
| Eventos | `WorldEvent` (Type, EntityID, Position, Metadata) | `internal/worldmodel/types.go` |
| Orchestrator | `Orchestrator` (ProcessFrame, AddEntity, InjectEvent, UpdateClimate, GenerateAsset) | `internal/worldmodel/orchestrator.go` |
| Asset | `AssetRequest`, `AssetResult`, `AssetProvenance`, `AssetProvider` | `internal/worldmodel/asset_integration.go` |
| AssetID→Path | `asset_registry.json` (AssetID → UE content path) | UE (Content/Cosca) |
| Bridge | `MessageSpawn/Action/Destroy/ImportMesh/Time/Weather`, `EntitySpec`, Controller | `internal/bridge/*` |

### O problema estrutural

O `WorldEntity` atual é modelado como **PERCEPÇÃO**, não como **ENTIDADE CANÔNICA**:

```go
type WorldEntity struct {
    Confidence float64  // ← só existe para percepção
    Depth      float64  // ← distância da câmera
    Embedding  []float32 // ← feature vector de visão
    LastSeen   time.Time // ← pruning visual (5s)
}
```

Isso significa: a mesma árvore, vista em 2 frames diferentes, **não tem
identidade estável** — é repruning se sair do frame. E não carrega:
`asset_id`, `state`, `parent`, `relationships`, `generation`, `provenance`.

**Consequência:** hoje o Cosca sabe "tem algo parecido com uma árvore aqui",
mas não sabe "existe uma árvore `tree_000184`, carvalho maduro, seed 827361,
versão 0.5.0, que persiste no mundo mesmo sem a câmera olhar".

---

## 2. As 3 Regras de Ouro (topo do documento)

### REGRA 1 — Nenhum conhecimento do Mundo Cosca depende da Unreal
O Cosca sabe que `tree_001` existe. A Unreal decide que é um `AStaticMeshActor`.
Conceito canônico no Cosca, tradução no adapter.

### REGRA 2 — Nenhuma geração procedural sem identidade, versão, seed e proveniência
Tudo que é gerado deve responder: o que criou? com quais dados? qual versão?
qual seed? qual regra? qual asset? quando?

### REGRA 3 — Nenhuma feature nova cria segunda representação do mesmo conceito
Nunca: `tree`, `Tree`, `tree_actor`, `vegetation_object`, `foliage_tree`, `tree_entity`
— 6 nomes para a mesma coisa. **Um conceito canônico por ideia**, adapters traduzem.

---

## 3. Gap Analysis — o que FALTA

| # | Fundação | Existe? | Lacuna |
|---|----------|---------|--------|
| 1 | **Entity canônica** | ⚠️ parcial | `WorldEntity` é percepção; falta `Entity` persistente com asset/state/generation |
| 2 | **GeoCoordinates** | ❌ | Só `Vec3` (engine); falta lat/lon/alt + conversão explícita |
| 3 | **Region/Chunk** | ❌ | Sem `Region` (terrain, entities, roads, vegetation) nem chunks |
| 4 | **Command vs Event** | ⚠️ parcial | `WorldEvent` genérico; falta contrato explícito Command(→) e Event(←) |
| 5 | **Schema version** | ❌ | Sem `world_schema_version` |
| 6 | **Determinism formal** | ❌ | `AssetProvenance` parcial; falta `input_hash` + entidade de generation |
| 7 | **World State consolidado** | ⚠️ parcial | `ClimateState` existe; falta world_id + coordenar Time/Weather/Environment/Rules |
| 8 | **Coordinate/Scale strategy** | ❌ | Sem camada Geo→World→Engine, sem origem/escala/precisão |
| 9 | **Provenance de ENTITY** | ⚠️ parcial | só de ASSET; falta `source`/`generation` na entidade |

Já existe (não mexer): Vec3, SpatialRelation, ClimateState, AssetProvenance, AssetID, MessageTime/Weather.

---

## 4. Contratos Canônicos PROPOSTOS

*(Schemas em Go — puros, stdlib, independentes da Unreal. Proposta para revisão.)*

### 4.1 Entity (cérebro, canônica)

```go
type Entity struct {
    ID          string            `json:"id"`              // "tree_000184"
    Type        string            `json:"type"`            // "vegetation.tree" (namespace canônico)
    Transform   Transform         `json:"transform"`
    Asset       AssetRef          `json:"asset"`           // asset_id, not path
    State       EntityState       `json:"state"`           // alive, age, etc.
    Properties  map[string]any    `json:"properties"`
    Parent      string            `json:"parent,omitempty"` // ID do bloco/região
    Relations   []EntityRelation  `json:"relations"`
    Provenance  Provenance        `json:"provenance"`
    Version     int               `json:"version"`          // schema/entity version
}

type AssetRef struct {
    AssetID string `json:"asset_id"` // "oak_mature_01" — NUNCA um path
}

type EntityState struct {
    Alive     bool   `json:"alive"`
    Age       string `json:"age"`       // "seedling", "mature", "ancient"
    Condition string `json:"condition"` // "intact", "damaged"
}

type Transform struct {
    Position Vec3 `json:"position"`
    Rotation Quat `json:"rotation"`
    Scale    Vec3 `json:"scale"`
}

type EntityRelation struct {
    To       string `json:"to"`
    Relation string `json:"relation"` // "belongs_to", "near", "entrance_of", "contains"
}

type Provenance struct {
    Source     GenerationSource `json:"source"`
    Generation GenerationMeta   `json:"generation"`
    Runtime    RuntimeMeta      `json:"runtime"`
}

type GenerationSource struct {
    Dataset   string `json:"dataset,omitempty"`  // "osm", "procedural", "asset"
    Version   string `json:"version"`
    Timestamp string `json:"timestamp"`
    Hash      string `json:"hash,omitempty"`
}

type GenerationMeta struct {
    Generator    string         `json:"generator"`     // "vegetation.generate_tree"
    Version      string         `json:"version"`
    Seed         int64          `json:"seed"`
    InputHash    string         `json:"input_hash"`
    Parameters   map[string]any `json:"parameters"`
}

type RuntimeMeta struct {
    CreatedAt    string `json:"created_at"`
    ModifiedAt   string `json:"modified_at"`
    LastEvent    string `json:"last_event,omitempty"`
}
```

### 4.2 Coordenadas (camada explícita, NUNCA espalhar conversão)

```go
type GeoCoordinates struct {
    Latitude  float64 `json:"latitude"`
    Longitude float64 `json:"longitude"`
    Altitude  float64 `json:"altitude"`
}

type WorldCoordinates struct {   // espaço contínuo do mundo
    X float64 `json:"x"`
    Y float64 `json:"y"`
    Z float64 `json:"z"`
}

type EngineCoordinates struct {  // o que o adapter manda pro renderer
    X float64 `json:"x"`
    Y float64 `json:"y"`
    Z float64 `json:"z"`
}

// Um único conversor. Contrato de unidades/origem/escala/precisão fica OBRIGATÓRIO.
func GeoToWorld(geo GeoCoordinates, origin GeoCoordinates, scale float64) WorldCoordinates
func WorldToEngine(w WorldCoordinates, origin WorldCoordinates) EngineCoordinates
```

### 4.3 Region / Chunk (para mundos grandes)

```go
type Region struct {
    ID         string        `json:"id"`        // "chunk_2_3"
    Chunk      ChunkCoord    `json:"chunk"`
    Terrain    TerrainRef    `json:"terrain"`
    Entities   []string      `json:"entities"`  // entity IDs
    Roads      []string      `json:"roads"`
    Buildings  []string      `json:"buildings"`
    Vegetation []string      `json:"vegetation"`
    Water      []string      `json:"water"`
    Metadata   map[string]any `json:"metadata"`
}

type ChunkCoord struct {
    X int `json:"x"`
    Y int `json:"y"`
}
```

### 4.4 Command vs Event (coisas DIFERENTES)

```go
// COMMAND: Cosca → Unreal ("faça isso")
type Command struct {
    Cmd       string         `json:"cmd"`       // "spawn_entity", "set_time", "set_weather", "destroy_entity"
    EntityID  string         `json:"entity_id,omitempty"`
    Params    map[string]any `json:"params"`
    ID        string         `json:"id"`        // correlation
    Timestamp string         `json:"timestamp"`
}

// EVENT: Unreal → Cosca ("isso aconteceu")
type Event struct {
    Type      string         `json:"type"`     // "entity_spawned", "tree_destroyed", "rain_started", "collision"
    EntityID  string         `json:"entity_id,omitempty"`
    Playload  map[string]any `json:"payload"`
    Timestamp string         `json:"timestamp"`
}
```

### 4.5 World State consolidado

```go
type WorldStateSchema struct {
    WorldID        string       `json:"world_id"`
    SchemaVersion  int          `json:"schema_version"`    // ← IMPORTANTE
    SimulationTime SimulationTime `json:"simulation_time"`
    Weather        WeatherState `json:"weather"`
    Environment    Environment  `json:"environment"`
    Entities       []string     `json:"entities"`
    Regions        []string     `json:"regions"`
    Version        int          `json:"version"`
}

type SimulationTime struct {
    Day        int     `json:"day"`
    Hour       float64 `json:"hour"`
    TimeScale  float64 `json:"time_scale"`
    Paused     bool    `json:"paused"`
}

type WeatherState struct {
    Type      string  `json:"type"`     // "clear", "rain", "snow", "fog", "storm"
    Intensity float64 `json:"intensity"`
    Wind      float64 `json:"wind"`     // m/s
    Temp      float64 `json:"temperature"`
}
```

---

## 5. A Regra de Threading (onde cada camada vive)

```
                    COSCA (cérebro)
                       │
                WORLD MODEL (semântico)
                       │
        ┌──────────────┼──────────────┐
        ↓              ↓              ↓
     CONTENT       SIMULATION      RULES
        │              │              │
        └──────────────┼──────────────┘
                       │
                WORLD COMMAND BUS
                       │
                  UE ADAPTER (tradutor)
                       │
                  UNREAL 5.8 (corpo)
                       │
            rendering / physics
                       │
                    EVENTS
                       │
                       └──────────→ COSCA
```

**Cosca NÃO sabe que `tree_001` é um `AStaticMeshActor`.**
Isso é responsabilidade exclusiva do UE Adapter.

---

## 6. Sequência PROPOSTA (Fase A–E)

### FASE A — Contratos (documento primeiro)
- [ ] `WorldState` schema + version
- [ ] `Entity` schema
- [ ] `Asset` schema
- [ ] `Region` schema
- [ ] `Command` schema
- [ ] `Event` schema
- [ ] `world_schema_version`

### FASE B — Identidade
- [ ] AssetID (já existe — consolidar)
- [ ] EntityID canônico
- [ ] RegionID
- [ ] IDs determinísticos
- [ ] Provenance + generation metadata (completo)

### FASE C — Espaço
- [ ] GeoCoordinates
- [ ] WorldCoordinates
- [ ] EngineCoordinates
- [ ] origem / escala / precisão
- [ ] chunk/region system

### FASE D — Bridge
- [ ] command validation
- [ ] command routing
- [ ] UE adapter (tradutor puro)
- [ ] event return
- [ ] error protocol
- [ ] acknowledgement

### FASE E — Mundo (SÓ DEPOIS)
- [ ] terrain
- [ ] vegetation
- [ ] roads
- [ ] buildings
- [ ] water
- [ ] weather
- [ ] NPCs

---

## 7. O que NÃO fazer agora (segurar)

❌ armas complexas
❌ multiplayer
❌ inventário gigante
❌ NPC superinteligente
❌ animações avançadas
❌ cidade procedural completa
❌ quests
❌ economia
❌ milhares de assets

Todos dependem das fundações acima.

---

## 8. Regras de implementação (para os capos)

1. **Nenhuma regra de mundo em código de adapter.** Adapter só traduz.
2. **Nenhuma conversão geográfica espalhada.** Um único conversor.
3. **Nenhum procedimento gera conteúdo sem seed/versão/ID/proveniência.**
4. **Nenhuma segunda representação de conceito já existente.** Reusa `AssetID`.
5. **Schemas sempre com `schema_version`.** Formato nunca implícito no código.

---

## 9. Decisão solicitada ao Don

1. Aprovar a direção: **fundações primeiro** (Fase A–C) antes de mais conteúdo?
2. Aprovar os contratos canônicos propostos (Entity, Region, Command/Event, WorldState)?
3. Aprovar a criação de `internal/world/schema` como pacote novo (separado do
   `worldmodel` de percepção)? — Proposta: manter percepção vs mundo separados.

> **Nota:** Nenhum código foi escrito. Esta é proposta para revisão antes de
> gerar dívida arquitetural. A decisão de avançar para Fase A é do Don.
