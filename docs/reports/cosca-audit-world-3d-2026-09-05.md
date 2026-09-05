# Auditoria de Arquitetura — Cluster MUNDO / 3D / UNREAL / BLENDER

> **Tipo:** auditoria read-only (nunca escreveu/modificou código)
> **Escopo:** cluster mundo/3D (worldmodel, world, worldspec, worldloop, worlddebug, bridge, agentbridge,
> gameengine, procgen, render, scene, nodegraph, compute, sensor + CLI)
> **Linguagem:** português brasileiro, objetivo, honesto
> **Data:** 2026-09-05
> **Fato-base (verificado via `go test -count=1`):** `internal/bridge`, `internal/worldmodel`,
> `internal/world` → **TODOS PASS**; `go build ./internal/world/... ./internal/worldmodel/...
> ./internal/bridge/... ./internal/procgen/...` → **BUILD OK (sem erro)**.
> **Regra:** CÓDIGO é a verdade; docs são a intenção. Cada afirmação abaixo cita `path:linha`.

---

## 0. Leitura de resultado em 3 frases

1. **O kernel de mundo é REAL e forte** (`internal/world` + `world/nav|road|city|edit` + `worldspec` +
   `worldloop` + `procgen`/`render`/`nodegraph`): Go puro, determinístico, com proveniência/epistemologia
   I3/I4 e tests verdes — é a fundação sólida do cluster.
2. **A periferia de percepção é "viúva de scripts"**: `vision` é nativo-ONNX real mas NÃO está ligado ao
   `Orchestrator` (que usa stubs), e `spatial`/`vfx`/`destruction`/`simulation` delegam a subprocessos
   Python (`runSubprocess` em `adapters/spatial/slam.py` etc.) cujos **scripts NÃO existem no repo**.
3. **Unreal é metade real, metade contrato**: o cliente WebSocket (`nhooyr.io/websocket`) e o Controller
   são reais e testados, mas o plugin `CoscaRuntime` (UCoscaWorldSubsystem) NÃO está neste repo — só o
   mock (`bridge/mock.go`) e o protocolo documentado. Blender é a única integração external **end-to-end
   real** aqui (scripts em `scripts/blender/*.py` + detecção de `blender.exe`).

---

## 1. `internal/worldmodel` (root — 6 src / 6 test)

### O que FAZ
Define os tipos fundamentais do Living World (`types.go`) e o **orquestrador do pipeline cognitivo**
(`orchestrator.go`), mais interfaces de fornecedores (`providers.go`), o **cache de crença** (`lister.go`),
o registro de adapters de ferramenta (`adapters.go`) e o contrato de asset (`asset_integration.go`).

### Pipeline real
`orchestrator.go:209` `ProcessFrame(ctx, FrameInput)` desenha o fluxo:
`FRAME → Vision → Spatial → Audio → State snapshot → Multi-agent → Actions → Step++`.
**PORÉM** cada etapa de percepção é um **STUB**:
- `orchestrator.go:263` `processVision` → devolve `VisionResult` **vazio** (só comentário "delegates to
  internal/worldmodel/vision").
- `orchestrator.go:273` `processSpatial` → devolve `SpatialResult{}` **vazio**.
- `orchestrator.go:283` `processAudio` → devolve `AudioPerception{}` **vazio**.
- `orchestrator.go:397` `coordinateAgents` → **`return nil, nil`** (placeholder).
- `orchestrator.go:249` Step 6 (ações) → `make([]ActionResult, 0)` (placeholder).

O que é REAL no orchestrator: `mergeEntities` (297), `listerUpsert` (336), `ResyncWorld` (355),
`InjectEvent` (427), `AddEntity` (455), `GenerateAsset` (464), `registerAssetEntity` (484).

### Integração Unreal
O orchestrator **não fala com Unreal** diretamente. A ponte é `internal/bridge` (ver §5).

### Blender
`GenerateAsset` (464) delega ao `AssetProvider` configurado — a implementação concreta é
`asset.BlenderAdapter` (ver §6). **Contrato, mas o Orchestrator só funciona se alguém injetar o provider.**

### Destroy/Simulation/Multiagent/VFX
O orchestrator **não chama** os subdomínios — nem planeja. `coordinateAgents` (multiagent) é stub; as
etapas VFX/Destruction/Simulation **não aparecem no `ProcessFrame`**.

### O que NÃO está implementado
- Pipeline de percepção (Vision/Spatial/Audio) do orchestrator: **stubs** (retornam vazio).
- Multi-agent no orchestrator: `return nil,nil`.
- `providers.go`: apenas **interfaces** (`VisionProvider`, `SpatialProvider`, `AudioProvider`,
  `VFXProvider`, `DestructionProvider`, `SimulationProvider`) — **nenhuma implementação concreta**.

### Estado
- `types.go` (486 linhas) — RICO e real: `Vec3/Quat/AABB/Pose6DoF`, `WorldEntity` (epistemologia
  I4: `Visibility` current|stale|inferred + `KnownPosition()` 172, `OccludedBy`), `SpatialObservation`
  (TrustState/Source I3/I4 274-303), `WorldState`, `VFXConfig`, `Material/Fragment/Debris`,
  `WorldEvent/SimulationStep`.
- `lister.go` (314 linhas) — REAL e substancial: padrão k8s informer→lister (ADR-023 item 5), a crença
  com selo epistêmico (`EntityStamp` com AsOf/Source/Trust/Revision), **fencing** anti-regressão (126),
  `Resync` (233), `ResyncIfDue` (291), `Stale` (303). Este é o ativo epistemológico real.
- **Testes verdes** (`internal/worldmodel` 0.321s).

---

## 2. `internal/worldmodel` subdomínios

### 2.1 vision (7 src / 4 test) — **NATIVO-Go ONNX real, mas não ligado**
- `onnx.go`: runtime ONNX via `github.com/yalue/onnxruntime_go` (resolvido em `go.mod`), sessão lazy,
  DirectML p/ RX 6700 XT (186), **degradação graciosa** (`ErrModelUnavailable`/`ErrModelNotFound`).
- `adapters.go`: `ClipAdapter` (CLIP ViT-B/32), `SAMAdapter` (SAM2 hiera), `GroundingAdapter`
  (GroundingDINO swint/Tiny — 5 inputs, tokenizer BERT nativo `bert_tokenizer.go`),
  `DepthAdapter` (Depth Anything V2). **Zero subprocesso.**
- `pipeline.go`: `Process` (146) com modo sequential/parallel real (grounding/depth em goroutines 261).
- **LIMITAÇÃO HONESTA:** exige `.onnx` presentes (`ModelsDir` → `~/.cosca/models/vision/`); sem modelo,
  degrada. CLIP zero-shot exige `TextEmbeddings` pré-computadas (112), SAM exige `PromptPoint` (212).
- **NÃO é ligado ao Orchestrator** (que usa stub). **Nenhum `vision.NewPipeline` é chamado no repo**
  (grep vazio) — é biblioteca "pronta mas órfã".

### 2.2 spatial (2 src / 2 test) — **subprocesso (scripts ausentes) + fallback Go**
- `pipeline.go:89` `Process` = Localize→Map→Reconstruct→Reasoning→fallback. Orquestração Go real.
- `adapters.go`: `SLAMAdapter.Localize/Map` (37/66), `ReconstructAdapter` (124/165),
  `ReasoningAdapter.Reason` (223) — **todos via `runSubprocess(... "python3", "adapters/spatial/slam.py")`**
  (`pipeline.go:282`, default scripts `adapters/spatial/slam.py`|`reconstruct.py`|`reasoning.py`).
- **Os scripts `adapters/spatial/*.py` NÃO existem no repo** (só `scripts/blender/*.py` existe).
  → o SLAM/Reconstr/Reasoning são **contrato de subprocesso que irá falhar em runtime** (python3 + path).
- Fallback nativo: `computeSpatialRelations` (183) + `clusterPoints` (239) — **Go puro, real** (mas só
  quando o subprocesso NÃO retorna relations).

### 2.3 vfx (2 src / 2 test) — **subprocesso (script ausente) + presets Go**
- `pipeline.go:73` `Simulate` → `TaichiAdapter` via `runSubprocess("python3","adapters/vfx/taichi.py")`.
- `adapters.go` (47/84/114) mesmos subprocessos. **`adapters/vfx/taichi.py` NÃO existe.**
- Presets reais em Go: `DustParticles` (119), `WindEffect` (138), `FireEffect` (155), `SmokeEffect` (173),
  `FluidEffect` (191) — só configs; não simulam nada sozinhos.

### 2.4 destruction (2 src / 1 test) — **subprocesso + cálculo Go**
- `pipeline.go:74` `Destroy` → `FractureAdapter.Fracture` (subprocesso `adapters/destruction/fracture.py`)
  → depois **Go real** `generateDebris` (106) e `computeStructuralImpact` (139).
- `adapters.go` (110) subprocesso. **`adapters/destruction/fracture.py` NÃO existe.**

### 2.5 simulation (2 src / 2 test) — **subprocesso (scripts ausentes)**
- `pipeline.go:63` `Simulate`/`Query` → `Taichi`/`MuJoCo` adapters (subprocesso
  `adapters/simulation/taichi.py`|`mujoco.py`). **Scripts NÃO existem.**

### 2.6 multiagent (1 src / 1 test) — **Go puro REAL**
- `multiagent.go`: `SharedWorld` thread-safe (Estados/agents/tasks/conflicts), `AssignTask` (190)
  com `scoreAgent` (238), `DetectConflicts` (322), `ResolveConflict` (358). **Zero subprocesso, real.**
  **NÃO ligado ao Orchestrator** (o uso do orchestrator é stub).

### 2.7 audio (16 src / 6 test)
- `pipeline.go:60` `NewPipeline` → adapters Whisper/DiffFields/Coqui/AudioCraft **subprocesso conceitual**.
- **STT real atrás de build tag:** `stt_sherpa.go` (``//go:build stt_sherpa``) usa
  `k2-fsa/sherpa-onnx-go-windows` (binding windows cgo) com `OnlineRecognizer` real. **Default build**
  (`stt.go`/`types.go`) = **stub disabled (no-op que reporta ErrDisabled)**.
- **TTS real atrás de build tag:** `tts_sherpa.go` (``//go:build tts_sherpa``); default `tts.go` = no-op.
- Mic: `mic/capture_winmm_windows.go` (Windows winmm REAL) + stubs cross-platform (`capture_other.go`).
- **Nota:** o mundo audio é o subdomínio com mais "vida" real (mic winmm + STT/TTS sherpa nativo), mas
  **fora do caminho do Orchestrator**.

### 2.8 asset (2 src / 1 test) — **Blender REAL end-to-end**
- `blender.go`: `BlenderAdapter` gera/valida/exporta via **subprocesso a `blender.exe`** (`exec.CommandContext`
  258/314/352) com scripts `generate.py`/`validate.py`/`export.py` que **EXISTEM** em `scripts/blender/`.
- `detectBlenderPath` (161) procura PATH + `ProgramFiles/Blender Foundation/*/blender.exe`.
- `DefaultBlenderAdapterConfig` (150). `GetVersion` (369). `extractJSON` (441) parseia saída do Blender.
- **Quem chama:** `bridge/runtime.GenerateAndSpawn` (via `AssetGenerator`), `cli/bridge.go` demo (212),
  `cli/asset.go` `gen` (333). → **integração real e wired.**

### 2.9 worldframe (1 src / 1 test) — tipos de frame de mundo (valor).

---

## 3. `internal/world` (14 src) + subdomínios — o CORAÇÃO REAL

### O que FAZ
World Model canônico, **puro Go stdlib, zero conhecimento de Unreal/Google Maps** (regra de fronteira).
`world.go` define `Vec3/Quat/Transform/BoundingBox`, `CoordinateSystem` (`GeoToWorld`/`WorldToGeo`
equiretangular 113/124), `Source/Generation/Provenance` (I3), `EntityClass/EntityType`, `Entity`
(com `Hash` 237), `Relation`, `WeatherState/Season/SimulationTime`, `World`.

### Pipeline / contrato
- `renderer.go:36` `RendererAdapter` (MaterializeEntity/UpdateEntity/RemoveEntity/ApplyWorldState) —
  a abstração que desacopla o mundo do renderizador. **`world` NUNCA referencia Unreal.**
- `serialize.go` — serialização **canônica/order-independent** (`buildCanonicalWorld` 46 sorteia entities e
  relations), `WorldFingerprint` (112): mesma seed+data = mesmo hash.
- `validate.go` `Validate` (35) — invariantes: ID único, relations não-órfãs, parent existe, sem NaN,
  sem escala negativa (5 checks).
- `query.go` — queries semânticas (CountByClass/ByType/TallestBuilding/EntitiesNear/EntitiesInside/
  ConnectBetween + pointInPolygon 70).
- `graph.go` — ChildrenOf/ParentOf/RelationsFrom/RelatedTo/EntitiesInRegion.
- `factory.go` — builders ergonômicos (TerrainEntity/RoadEntity/TreeEntity/BuildingEntity/RockEntity).

### Subdomínios — TODOS reais e determinísticos
- `city/city.go` (304): gerador de cidade sintética **determinística** (`Generate` 55, seed→mesma cidade),
  hierarquia City→District→Streets→Buildings/Trees/Vehicles→Square→River, proveniência `ClassGENERATED`.
- `nav/nav.go` (174): **A*** real (`Plan` 97, `aStar` 118) com **gate I1/I2** (`ValidateDestination` 87
  fail-closed) — "LLM propõe destino, sistema decide rota".
- `road/road.go` (200): gerador de rede viária **topológica** determinística (priority-queue de pontas,
  snap/merge interseção, highways) — `Generate` (77) fail-closed (BranchChance>1 → erro).
- `edit/edit.go` (92): **placement determinístico** — `Resolve` (54) converte intenção estruturada
  ({asset,reference,relation,offset}) em posição/orientação no mundo, sem LLM.
- `reconstruct/reconstruct.go` (85): `Reconstructor.Reconstruct` (47) materializa um `World` inteiro via
  `RendererAdapter` (cap por classe/quantidade + aplica mundo não-entidade).
- `adapter/unreal.go` (173): **tradução semântica → comandos bridge** (`MaterializeEntity` 38 usa
  `ctrl.ImportMesh`, `entityAssetID` 102 mapeia class→AssetID, `classToBridgeType` 146). **Ressalva
  honesta:** conversões WGS84→ECEF/EGM96/ENU **NÃO existem** (é só GeoToWorld equiretangular local —
  o ADR-021 item 7 marca isso como "só se globo").

### Estado
**REAL e maduro.** Testes verdes.

---

## 4. `internal/worldspec` + `internal/worldloop` + `internal/worlddebug`

- `worldspec/worldspec.go` (236): contrato canônico ADR-024 — `WorldSpec` reusa `gameengine.Entity` ECS
  (`FromWorld` 126, `FromGameEngine` 149, `FromScene` 164), com `terrain/navigation/environment/
  simulation/provenance`, `JSON()` 200, `Hash()` 205, `Validate()` 217 (fail-closed I2: seed obrigatório,
  provenance.class válida). **Real.**
- `worldloop/worldloop.go` (171): loop cognitivo **AUTHOR→SIMULATE→OBSERVE→EVALUATE→MODIFY** (ADR-024
  §2.4). `Run` (140) com `Simulator` (`AccessibilitySimulator` 36 => grau de cada nav-node),
  `Apply` (`ConnectIsolated` 98 => liga nó isolado ao hub), e gate `evalgo.Gate` (`DefaultGate` 165).
  Composição real sobre `evalgo`/`worldspec`.
- `worlddebug/{inspector,replay,browser,helpers}.go`: inspector/explicação por entidade + replay — real.

---

## 5. `internal/bridge` (5 src / 4 test) — a ponte Unreal (metade real, metade contrato)

### O que FAZ
"Sistema nervoso" Cosca↔Unreal: recebe frames/estado, despacha spawn/move/destroy/import_mesh/weather/time.

### WebSocket real
- `websocket.go:48` `WebSocketClient.Connect` usa **`nhooyr.io/websocket`** (`Dial` + `DialOptions`
  CompressionDisabled). `readLoop` (160) lê frames BINÁRIOS (comentário 171: o plugin UE
  WebSocketNetworking devolve binary) e roteia por handler ou enfileira no `inbox` (backpressure 200).
- **Contratos de mensagem:** `client.go:26-46` `MessageType` — frame|audio|event|state_sync (Unreal→Cosca);
  action|spawn|destroy|modify|import_mesh|weather|time (Cosca→Unreal); ping|pong|error.
- **`seq`** para correlação request/response (`websocket.go:30,97`).
- **`reconnectDelay`/`pingInterval`** (`websocket.go:33-34`) **definidos mas SEM loop de auto-reconnect** —
  no fim da conexão o `readLoop` apenas marca `connected=false` e `close(inbox)` (177-181). (Honestidade:
  o GRACE de reconexão não está implementado.)

### Controller
- `controller.go:75` `Controller` — real: `Spawn` (169), `Move` (183), `Destroy` (195),
  `ImportMesh` (210), `SetTimeOfDay` (242), `SetWeather` (252), reconciliação de `StateSnapshot`
  (OnMessage StateSync 137), ack por ID (`acks`).

### Runtime (seam)
- `runtime.go:72` `StartFrameLoop` liga frame→`FrameObserver.Observe`→`orchestrator.AddEntity`.
- `runtime.go:93` `GenerateAndSpawn` liga Blender→spawn Unreal→registra no orchestrator.
- `FrameObserver`/`AssetGenerator` são interfaces (20/31) — **o demo CLI usa um observer STUB** (`cli/bridge.go:226`
  devolve uma `WorldEntity` hardcoded).

### Mock server
- `mock.go`: `Listen` + `ServeMock` (accepta subprotocolo `cosca`, ecoa `pong`) — **real**, permite
  desenvolver sem Unreal.

### O que NÃO está implementado / ausente
- **O plugin Unreal `CoscaRuntime`/`UCoscaWorldSubsystem` NÃO está neste repo** (grep `.uplugin|CoscaRuntime`
  → só docs). A fronteira define um **repo separado "Cosca-Unreal (PROPOSTO)"** (`fronteira_cosca_unreal.md:107`).
  O lado Go tem cliente+controller+mock reais; o lado Unreal é **contrato documentado + mock**.
- Sem auto-reconnect ativo; sem `asset_registry.json` gerado aqui.

### Estado
Testes verdes (1.243s). Build ok.

---

## 6. `internal/agentbridge` + `internal/gameengine`

- `gameengine/gameengine.go` (215): **ECS Go puro** real — `Scene`→`Entity`→`Component`→`System`
  (ComponentType valid 37, AddEntity 90 valida ID único, UnmarshalScene 180 valida).
- `agentbridge/{bridge,events,session}.go`: ponte de agentes (sessão/eventos) — real.

---

## 7. `internal/procgen` (7 src) + `internal/render` + `internal/scene` + `internal/nodegraph`

### procgen — núcleo procedural NATIVO real
- `noise.go` (198): `Perlin2D`, `Simplex2D`, `Worley2D` (F1), `Voronoi2D` (F2-F1, bordas de células),
  `FBM2D`/`FbmPerlin`/`FbmSimplex` — determinísticos (`Hash2` splitmix64 portável 8, `rng.state`).
- `nodes.go` (704): `nodegraph.Executor` para Perlin/Simplex/Worley/Voronoi/Fbm/Gradient/Checker/Stripes/
  Cells/Rings + Math (Add/Multiply/Remap/Clamp/Lerp/Curve), resolução de params grafo→struct→render ctx
  (`seedFor` 156), métricas `OpMetric` (`recordMetric` 169).
- `rng.go`/`math.go`/`pattern.go`/`image.go`/`metrics.go`: RNG splitmix64, math puro, patterns, RGBA próprio.
- **Zero dependência externa, puro determinístico.**

### render / nodegraph / scene
- `nodegraph/executor.go` (137): `Graph.Run` em ordem topológica com **cache por assinatura recursiva**
  (`Cache` 33, `Signature` — §23), `Stats` (Executed/CachedHits).
- `nodegraph/nodegraph.go`: grafo (TopoOrder/Signature/Validate/Marshal).
- `render/render.go` (363): Render Engine QUALITY (Preview<Draft<Final), `Job.JobKey` (131), `Renderer.Render`
  (157) com **checkpoint resumível** (`loadCheckpoint`/`saveCheckpoint` 287/307), contexto determinístico
  `WithRenderContext` (251).
- `scene/{scene,builder,compiler,transform}.go`: scene graph real (camera/luz/terreno/procedural entities).

**Nota:** este stack (procgen+nodegraph+render+scene) é a via de **geração procedural 2D/visual** — separado
do mundo 3D Unreal. É real e testável.

---

## 8. `internal/compute` + `internal/sensor` (pipeline espacial)

- `compute/*`: **capacidade de hardware** (caps, cpuprobe, gpuprobe, memoryprobe, storageprobe, topologyprobe,
  fabric, pool) + `gpu.go` `OllamaExecutor` (154) — cliente HTTP a um servidor Ollama ROCm/CUDA local
  (probe de GPU lazy/cacheado, guard de VRAM, override ROCmGfx1031). **Infra paralela**, não é o pipeline
  espacial SLAM, mas suporta a inferência GPU local.
- `sensor/sensor.go` + `sensor/fusion/fusion.go` + `sensor/gate/gate.go`: **fusão real** — consenso por
  log-odds (evidência independente) e **detecção de contradição** entre referentes concorrentes no mesmo
  slot (`fusion.go` 1-60), com gate "saber vs ver" (não fabrica percepção). **Real.** Atende I4.

---

## 9. CLI (`internal/cli`)

- `world.go`: `inspect` (OSM→Inventory→Validate→Fingerprint→Golden), `spawn` (OSM→**Unreal** via
  bridge+reconstruct), `explain` (Inspector por entidade), `build` (WorldSpec→**Rojo**), `loop`.
- `world_spec.go`: `build` (Rojo, `engineadapter.NewRobloxAdapter`), `loop` (worldloop).
- `bridge.go`: `serve` (mock WS), `connect` (handshake ping/pong), `demo` (**vertical slice c/ observer
  STUB**), `import-mesh`, `time`, `weather`.
- `asset.go`: `add/list/info/gen` — `gen` chama **Blender real** (subprocesso), valida e registra.
- `render.go` ("render <workflow.json>"), `ngraph.go`/`ngraph_demo.go` (nodegraph), `scene_demo.go`,
  plus `task`, `gpu`.

**Nota honesta sobre `engineadapter`:** tem `RobloxAdapter` (real, gera Rojo) mas **NÃO tem
`UnrealAdapter`/`BlenderAdapter`** ali — Unreal é via `world/adapter` (bridge) e Blender via
`worldmodel/asset` (subprocesso). São três caminhos distintos de materialização.

---

## 10. Tabela-resumo

| Componente | #src | #test | Implementação real | Estado | Nota honesta |
|---|--:|--:|---|---|---|
| `worldmodel` (root) | 6 | 6 | Native Go (tipos + lister) | ✅ build/test | `types.go`+`lister.go` são fortes; `orchestrator` só desenha o pipeline com **stubs** de percepção |
| `worldmodel/vision` | 7 | 4 | **Nativo-Go ONNX** | ✅ test | Clipe/SAM/GroundingDINO/Depth **nativos**, mas **não ligado** ao orchestrator; exige `.onnx` pré-carregados |
| `worldmodel/spatial` | 2 | 2 | Subprocesso (scripts ausentes) | ⚠️ | SLAM/Reconstr/Reasoning via `python3 adapters/spatial/*.py` — **scripts não existem**; fallback `computeSpatialRelations` é Go real |
| `worldmodel/vfx` | 2 | 2 | Subprocesso (script ausente) | ⚠️ | `taichi.py` ausente; presets (Dust/Fire/…) são só configs Go |
| `worldmodel/destruction` | 2 | 1 | Subprocesso + Go real | ⚠️ | `fracture.py` ausente; debris/impacto são **Go real** (`pipeline.go:106,139`) |
| `worldmodel/simulation` | 2 | 2 | Subprocesso (scripts ausentes) | ⚠️ | `taichi.py`/`mujoco.py` ausentes; orquestração Go real (`pipeline.go:63`) |
| `worldmodel/multiagent` | 1 | 1 | **Go puro REAL** | ✅ | `SharedWorld` real (assign/conflict), mas **não ligado** (orchestrator usa stub) |
| `worldmodel/audio` | 16 | 6 | Nativo (winmm + sherpa STT/TTS) | ⚠️→✅ | mic winmm real; STT/TTS sherpa **só atrás de build tag**; default = no-op stub |
| `worldmodel/asset` | 2 | 1 | **Blender real (subprocesso)** | ✅ | `blender.exe` + `scripts/blender/*.py` **existem**; wired via `bridge/runtime` e CLI |
| `worldmodel/worldframe` | 1 | 1 | Go puro | ✅ | tipos de frame de mundo |
| `world` (root) | 8 | 1 | **Go puro REAL** | ✅ | World canônico; serialização order-independent; `RendererAdapter` desacopla da Unreal |
| `world/city` | 1 | 1 | **Nativo determinístico** | ✅ | cidade sintética seed→mesma cidade; proveniência GENERATED |
| `world/nav` | 1 | 1 | **Nativo A*** | ✅ | gate I1/I2 fail-closed; "LLM propõe, sistema decide rota" |
| `world/road` | 1 | 1 | **Nativo determinístico** | ✅ | rede viária topológica (snap/merge interseção) |
| `world/edit` | 1 | 1 | **Nativo determinístico** | ✅ | placement por intenção estruturada (sem LLM) |
| `world/reconstruct` | 1 | 1 | Nativo (RendererAdapter) | ✅ | materializa o mundo inteiro via adapter |
| `world/adapter` | 1 | 2 | **Nativo → bridge** | ✅ | traduz `world.Entity`→comandos bridge; `AssetID` semântico (nunca `/Game/`) |
| `worldspec` | 1 | 1 | **Nativo REAL** | ✅ | contrato canônico ADR-024; reusa `gameengine` ECS; `Validate` fail-closed |
| `worldloop` | 1 | 1 | **Nativo REAL** | ✅ | loop AUTHOR→SIMULATE→OBSERVE→EVALUATE→MODIFY sobre `evalgo` |
| `worlddebug` | 4 | 1 | Nativo | ✅ | inspector/replay/browser |
| `bridge` | 5 | 4 | **WebSocket real + mock** | ✅ | `nhooyr.io/websocket` real, Controller real, mock server real; **plugin Unreal ausente** |
| `agentbridge` | 3 | 1 | Nativo | ✅ | ponte de agentes |
| `gameengine` | 1 | 1 | **Nativo ECS** | ✅ | Scene→Entity→Component→System Go puro |
| `procgen` | 7 | 1 | **Nativo determinístico** | ✅ | Perlin/Simplex/Worley/Voronoi/FBM; zero dep externa |
| `render` | 1 | 1 | **Nativo REAL** | ✅ | Quality + cache + checkpoint resumível |
| `nodegraph` | 2 | 2 | **Nativo REAL** | ✅ | executor topo-order + cache por assinatura |
| `scene` | 4 | 1 | Nativo | ✅ | scene graph + builders |
| `compute` | mul. | mul. | **Nativo REAL** (Ollama executor) | ✅ | capacidade de hardware + GPU exec local (infra paralela) |
| `sensor`+`fusion` | 3+ | 3+ | **Nativo REAL** | ✅ | fusão log-odds + contradição + gate I4 |

---

## 11. Respostas diretas às perguntas do auditor

**Q1 (O que FAZ):** kernel de mundo canônico determinístico (`world`) + pipeline cognitivo desenhado
(`worldmodel.Orchestrator`) + contrato de autoria (`worldspec`) + loop de experimentação (`worldloop`) +
ponte WebSocket Unreal (`bridge`) + geração procedural (`procgen/render/nodegraph`) + percepção nativa
(`vision` ONNX, `audio` sherpa, `sensor/fusion`).

**Q2 (Pipeline real FRAME→Localize→Map→Reconstruct→Reasoning→SpatialObservation):**
- **Nativo-Go real:** só o **fallback** `computeSpatialRelations`+`clusterPoints` (`spatial/pipeline.go:183,239`)
  e a orquestração Go do `Pipeline.Process` (89).
- **Subprocesso (contrato):** Localize/Map/Reconstruct/Reasoning (`adapters/spatial/*.py`) — **scripts
  AUSENTES no repo** → não executa de fato.
- **Stub:** o `Orchestrator.ProcessFrame` **nem chama** o pipeline espacial — `processSpatial` (273) devolve vazio.
- **Veredito:** o pipeline espacial **NÃO está operacional end-to-end em Go hoje.**

**Q3 (Integração Unreal):** WebSocket **real** (`nhooyr.io/websocket`, `websocket.go:52`), contratos
`MessageType` (`client.go:26`), `seq` (correlação) e `reconnectDelay`/`pingInterval` (definidos; **sem
loop de auto-reconnect**). Controller real. **MAS** o `UCoscaWorldSubsystem`/plugin `CoscaRuntime`
**não existe neste repo** — é contrato em docs (`fronteira_cosca_unreal.md`) + **mock** (`bridge/mock.go`).

**Q4 (Blender):** **integração REAL** (`blender.go` subprocesso a `blender.exe` + scripts em
`scripts/blender/{generate,validate,export}.py`). **Quem chama:** `bridge/runtime.GenerateAndSpawn` (93),
`cli/bridge.go` demo (212), `cli/asset.go` `gen` (333), `worldmodel.Orchestrator.GenerateAsset` (464).
É a única integração external **real e ligada** do cluster.

**Q5 (Destroy/Simulation/Multiagent/VFX):** todos são **adapters** (subprocesso Python com scripts
ausentes) exceto **multiagent** (Go puro real) — mas nenhum está ligado ao `Orchestrator` (que os ignora
ou usa stub).

**Q6 (NÃO implementado / stub / no-op — honestidade total):**
- `orchestrator.processVision/processSpatial/processAudio` = **stubs vazios** (`orchestrator.go:263/273/283`).
- `orchestrator.coordinateAgents` = **`return nil,nil`** (`orchestrator.go:399`); Step 6 ações = placeholder.
- `providers.go` = só interfaces, sem implementação.
- Scripts `adapters/spatial/*.py`, `adapters/vfx/taichi.py`, `adapters/destruction/fracture.py`,
  `adapters/simulation/taichi.py|mujoco.py` = **não existem no repo** (só `scripts/blender/*.py`).
- `engineadapter`: só `RobloxAdapter`; sem Unreal/Blender adapter ali.
- `audio`: STT/TTS sherpa reais só atrás de build tag; default = no-op (`stt.go`,`tts.go`).
- `vision`: CLIP zero-shot exige `TextEmbeddings`; SAM exige `PromptPoint`; tudo degrada sem `.onnx`.
- `bridge`: sem auto-reconnect; sem o plugin Unreal no repo.
- CLI `bridge demo`: `FrameObserver` é **stub** (retorna entidade hardcoded, `cli/bridge.go:226`).
- WGS84→ECEF/EGM96/ENU (ADR-021 item 7): **não implementado** (só equiretangular local em `world.go:113`).

**Q7 (Estado):** `go build` dos pacotes alvo → **OK (sem erro)**. `go test -count=1 ./internal/bridge/
./internal/worldmodel/ ./internal/world/` → **todos PASS** (bridge 1.243s, worldmodel 0.321s, world 0.310s).

---

## 12. Síntese honesta

O cluster é **assimétrico**: o "cérebro/mundo" (kernel `world`, `worldspec`, `worldloop`, `nav/road/city/
edit`, `procgen/render/nodegraph`) é **maduro, determinístico, testado e aderente a I1–I5**. Já a
"percepção/execução" (o que alimenta/vive o mundo) está em **estado de contrato**: `vision` é nativo mas
órfão; `spatial/vfx/destruction/simulation` são subprocessos cujos scripts não existem; e o `Orchestrator`
que deveria amarrar tudo usa **stubs**. Unreal é cliente+mock reais (plugin e loop de reconexão ausentes);
**Blender é o único corpo external real e ligado**; multiagent é Go-puro real mas desconectado.

*A presente auditoria é somente leitura; nenhum arquivo foi modificado.*
