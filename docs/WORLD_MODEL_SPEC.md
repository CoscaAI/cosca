# WORLD_MODEL_SPEC.md — Cosca World Model

> **Versão:** 1.0 (SchemaVersion 1)
> **Autor:** cosca-kernel | **Data:** 2026-08-24
> **Status:** ENTREGUE — FASE A (per professor)
> **Local:** `internal/world/`

---

## 1. REGRA FUNDAMENTAL

> **O World Model NÃO conhece Unreal.**
> NÃO conhece AActor, UStaticMesh, UWorld, UBlueprint, NEM Google Maps.
> Ele representa o mundo. Renderers o adaptam.

Isso é a separação entre **"como o mundo é descrito"** e **"como o mundo é renderizado"**.

---

## 2. Os 9 Componentes

### 2.1 World Entity Model (`entity.go` no world.go)
- `EntityID` estável (ex: `tree_000184`) — independe do renderer
- `Class` (terrain/vegetation/structure/water/road/vehicle)
- `Type` específico (ex: `tree.oak`)
- `Transform` (Position/Rotation/Scale)
- `BoundingBox`
- `Parent` + `Children` → hierarquia
- `Properties` → propriedades semânticas (species, age, width, height)
- `State` → alive/health/condition/age
- `Provenance` → de onde veio
- `Version` → schema version da entidade

### 2.2 Coordinate System (`coordinate.go` em world.go)
- `GeoCoordinates` → WGS84 (lat/lon/alt)
- `CoordinateSystem` → origem + unidades + escala
- `GeoToWorld` / `WorldToGeo` → **um único conversor** (equirectangular local)
- `Origin` → anchor geográfica
- Documentado: precisão ok para escala cidade

### 2.3 World Graph (`graph.go`)
- Entity → Entity com relações semânticas:
  - `contains`, `located_at`, `connected_to`, `adjacent_to`, `belongs_to`, `derived_from`
- Navegação: `ChildrenOf`, `ParentOf`, `RelationsFrom`, `RelatedTo`
- `EntitiesInRegion` (spatial containment)

### 2.4 Spatial Model (`spatial.go`)
- `Point`, `Line` (polyline + Length), `Polygon` (Area), `Volume`
- `BoundingBox`
- `TerrainRegion` (polígono + classificação + elevação)
- `CoordinateTransform`

### 2.5 Temporal Model
- `SimulationTime` (Day/Hour/TimeScale/Paused) — determinístico
- `WeatherState` (type/intensity/wind/temp)
- `Season` (spring/summer/autumn/winter)

### 2.6 Provenance (`world.go` Provenance)
Cada entidade responde:
- `Class` → FACT / MEASURED / INFERRED / GENERATED / HYPOTHESIS / UNKNOWN
- `Source` → dataset + versão + hash
- `Generation` → generator + version + seed + input_hash + parameters
- `Accuracy` → precisão 0..1
- `CreatedAt`

### 2.7 Serialization (`serialize.go`)
- Formato canônico determinístico (JSON, ordem de campos estável)
- `schema_version` embutido
- **Fingerprint SHA-256** → mesmo mundo = mesmo hash
- Round-trip validado

### 2.8 World → Renderer Contract (`renderer.go`)
- `RendererAdapter` interface (MaterializeEntity, UpdateEntity, RemoveEntity, ApplyWorldState)
- `RendererCommand` / `RendererEvent`
- O World Model conhece SÓ esta interface — nunca AActor/UStaticMesh

### 2.9 Teste Mínimo (`world_test.go`)
- Mundo sintético: Terrain + Road + Tree + Building + Rock
- 7 testes determinísticos passando

---

## 3. FINGERPRINT (determinismo)

```
WorldFingerprint(w) = SHA-256( MarshalWorldCanonical(w) )
```

Mesmo mundo construído de formas diferentes → mesmo hash. Verificado por
`TestFingerprintStableAcrossConstructors`.

---

## 4. Limites da Conversão Geográfica

O `GeoToWorld` usa projeção equirectangular local (tangent plane).
- **Preciso para:** escala de cidade/região (~dezenas de km).
- **Limitado para:** escala continental/global (erro cresce com distância da origem).
- **Documentado no código:** trocar por UTM ou projeção adequada se o mundo crescer.

---

## 5. O QUE FOI ENTREGUE (checklist do professor)

- [x] 1. World Entity Model (ID/tipos/geo/transform/hierarquia/relações/props/provenance/versão/estado)
- [x] 2. Coordinate System (WGS84/local/conversão/origem/unidades/altitude/precisão)
- [x] 3. World Graph (contains/located_at/connected_to/adjacent_to/belongs_to/derived_from)
- [x] 4. Spatial Model (Point/Line/Polygon/Volume/BBox/TerrainRegion/transform)
- [x] 5. Temporal Model (timestamp/dia-noite/weather/estado-ao-longo-do-tempo/sim-time)
- [x] 6. Provenance (fonte/quando/precisão/FACT-MEASURED-INFERRED-GENERATED/versão)
- [x] 7. Serialization (formato canônico/determinístico/versionado/hashável/reproduzível)
- [x] 8. World → Renderer Contract (adapter abstrato)
- [x] 9. Teste mínimo (mundo sintético sem Unreal)

---

## 6. PRÓXIMA TRAVA (regra do professor)

> **Antes de qualquer integração Google Maps / OSM / GIS / DEM...**
> World Model deve ter: MODELO_SEM + schema + testes determinísticos + mundo
> sintético pequeno + adapter Unreal funcional.

**Status dos pré-requisitos:**
- ✅ WORLD_MODEL_SPEC.md (este arquivo)
- ✅ schema (SchemaVersion 1)
- ✅ testes determinísticos (7/7)
- ✅ mundo sintético pequeno (Terrain/Road/Tree/Building/Rock)
- ⏳ adapter Unreal funcional (bridge já existe — falta ligar ao RendererAdapter)

> ⚠️ **Ainda NÃO** conectar Google Maps até o adapter Unreal estar funcional.

---

## 7. Regras de Implementação

1. **Nenhuma regra de mundo em código de adapter.** Adapter só traduz.
2. **Nenhuma conversão geográfica espalhada.** Um único conversor (`GeoToWorld`).
3. **Nenhuma geração sem seed/versão/ID/proveniência.**
4. **Schemas sempre com `schema_version`.**
5. **Nenhuma segunda representação de conceito já existente.**
