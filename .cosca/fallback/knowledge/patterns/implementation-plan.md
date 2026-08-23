# Cosca Living World — Implementation Plan

> **Version**: 1.0.0 | **Confidence**: 0.88 | **Category**: Architecture/Implementation | **Created**: 2026-08-23 | **Source**: Codebase audit (32 packages, 92 CLI commands, 52 REST endpoints, 114 internal packages)

> **Status**: IMPLEMENTATION READY — plano concreto antes de qualquer código.

---

## 1. Auditoria — O que já existe

### 1.1 Infraestrutura existente (Forte)

| Componente | Pacote | O que faz | Status |
|------------|--------|-----------|--------|
| **Node Graph** | `internal/nodegraph/` | DAG serializável (ComfyUI-inspired), executor interface, cache por assinatura | ✅ Maduro |
| **Scene Graph** | `internal/scene/` | Entity tree, transforms, compilação para lista flat | ✅ Maduro |
| **ECS Game Engine** | `internal/gameengine/` | Entity-Component-System, 10 tipos (transform, physics, render, animation, audio, input, AI, collider, health, score) | ✅ Maduro |
| **Procedural Gen** | `internal/procgen/` | Perlin, Simplex, Worley, FBM, Ridged, White noise | ✅ Maduro |
| **Media Engine** | `internal/media/` | ffmpeg/ffprobe, probe/transcode | ✅ Maduro |
| **Orchestration** | `internal/orchestration/` | Pipeline: context builder → router → executor → pipeline | ✅ Maduro |
| **Memory** | `internal/memory/` | 5 camadas (global/workspace/project/session/temp/long-term) | ✅ Maduro |
| **Knowledge** | `internal/knowledge/` | SQLite FTS5 + vector store | ✅ Maduro |
| **Search** | `internal/search/` | Hybrid: FTS5 + vector + graph traversal | ✅ Maduro |
| **Embeddings** | `internal/embeddings/` | Provider interface (pluggable) | ✅ Maduro |
| **Plugin System** | `internal/plugins/` | WASM/Wazero sandbox, lifecycle, allow/deny | ✅ Maduro |
| **Results** | `internal/results/` | Envelope uniforme OK/Fail/Degraded | ✅ Maduro |
| **Policy** | `internal/policy/` | ALLOW/DENY/CONFIRM/ESCALATE | ✅ Maduro |
| **Adapter** | `internal/adapter/` | Conversão de envelope legado | ✅ Maduro |
| **Deliberation** | `internal/deliberate/` | Meta-cognição determinística (ADR-011) | ✅ Maduro |
| **CLI** | `internal/cli/` | 92 subcomandos Cobra | ✅ Maduro |
| **API** | `api/` | REST (52 endpoints) + gRPC (3 services) | ✅ Maduro |
| **SDK** | `pkg/cosca/` | Go SDK com 12 sub-SDKs | ✅ Maduro |
| **Integrity** | `internal/integrity/` | Ed25519, DPAPI, blockchain | ✅ Maduro |

### 1.2 Componentes existentes (Fracos — precisam de extensão)

| Componente | Pacote | O que faz | O que falta |
|------------|--------|-----------|-------------|
| **Vision** | `internal/vision/` | OCR via Tesseract (apenas extração de texto) | Interface de provider, CLIP, SAM, GroundingDINO, Depth, detecção |
| **Voice** | `internal/voice/` | TTS concatenativa PT-BR (G2P, fonemas, díphones) | Interface de provider, Whisper STT, AudioCraft, spatial audio |
| **Game Engine** | `internal/gameengine/` | ECS básico com 10 componentes | Fracture, destruction, ragdoll, vehicle physics |

### 1.3 Componentes inexistentes (Gap total)

| Camada | O que falta |
|--------|-------------|
| **Spatial AI** | SLAM, 3D reconstruction, scene understanding, depth → world coordinates |
| **VFX** | Particle systems, fluid simulation, cloth, procedural effects |
| **Destruction** | Voronoi fracture, ragdoll, soft body, vehicle physics |
| **Simulation** | Climate, ecosystem, economy, crowd, temporal evolution |
| **World Model** | WorldEntity, SpatialObservation, SpatialRelation, WorldState types |
| **Unreal Bridge** | WebSocket/HTTP bridge para comunicação com UE |
| **Provider Interfaces** | VisionProvider, SpatialProvider, AudioProvider, VFXProvider, DestructionProvider, SimulationProvider |

### 1.4 Dependências externas

| Categoria | Dependências existentes | Dependências faltantes |
|-----------|------------------------|------------------------|
| **CLI** | cobra, viper | — |
| **TUI** | bubbletea, lipgloss, glamour | — |
| **Logging** | zerolog | — |
| **Crypto** | blake3, crypto (stdlib) | — |
| **Storage** | sqlite (modernc), bbolt | — |
| **gRPC** | grpc, protobuf | — |
| **WASM** | wazero | — |
| **HTTP** | nhooyr/websocket | — |
| **Vision** | — | CLIP, SAM, GroundingDINO, Depth (via subprocess/API) |
| **Audio** | — | Whisper, Coqui TTS, AudioCraft (via subprocess/API) |
| **Spatial** | — | ORB-SLAM3, Instant-NGP (via subprocess/API) |
| **Physics** | — | MuJoCo, Box2D (via subprocess/API) |
| **VFX** | — | Taichi (via subprocess/API) |
| **ML Inference** | — | ONNX Runtime ou subprocesso Python |

**Crucial:** O Cosca NÃO tem dependências de vision/audio/spatial/physics. Todo ML é via API providers (Ollama/OpenAI). Isso é uma **força** — significa que a integração deve ser via **subprocessos/APIs**, não bibliotecas C/C++ embutidas.

---

## 2. Implementation Gap Matrix

### Camada 5: Vision

| Campo | Valor |
|-------|-------|
| **Capacidade** | O agente enxerga o mundo |
| **Já existe** | `internal/vision/` (apenas OCR Tesseract — 1 função `Describe()`) |
| **Falta** | VisionProvider interface, CLIP adapter, SAM adapter, GroundingDINO adapter, Depth adapter, frame pipeline |
| **Projeto escolhido** | CLIP (MIT, leve) + SAM2 (Apache, segmentação) + GroundingDINO (Apache, detecção) + Depth Anything V2 (Apache, profundidade) |
| **Alternativas** | YOLO (AGPL — evitar), DINOv2 (Apache — features) |
| **License** | MIT + Apache-2.0 (todas compatíveis) |
| **Dependências** | Python 3.10+, PyTorch, transformers. Execução via subprocesso ou ONNX |
| **CPU** | Mínimo: 4 cores. Recomendado: 8+ cores |
| **GPU** | Opcional (acelera 10x). CLIP-S: 2GB VRAM. SAM2-L: 4GB VRAM |
| **VRAM** | Mínimo: 2GB (CLIP-S). Recomendado: 6GB (CLIP-L + SAM2) |
| **Disco** | ~2GB (modelos CLIP+SAM+GroundingDINO+Depth) |
| **Integração Unreal** | Frame vem da câmera Unreal → WebSocket → Cosca → Vision → resultado volta |
| **Integração Cosca** | Novo pacote `internal/vision/provider.go` (interface) + `internal/vision/clip/` (adapter) |
| **Risco** | Médio — subprocesso Python pode ter latência. Mitigar com ONNX (futuro) |
| **Prioridade** | **1** (mais alto — destrava todo o pipeline) |
| **Critério de aceitação** | Dado um frame PNG, o sistema retorna: objetos detectados (bbox + label + score), máscaras de segmentação, embedding CLIP, mapa de profundidade |

### Camada 6: Spatial AI

| Campo | Valor |
|-------|-------|
| **Capacidade** | O agente entende o espaço |
| **Já existe** | Nada |
| **Falta** | SpatialProvider interface, pose estimation, 3D reconstruction, scene graph neurai, depth → world coords |
| **Projeto escolhido** | ORB-SLAM3 (GPL — usar via subprocesso, não embutir) ou ORB-SLAM3 alternative (OpenVSLAM, BSD) |
| **Alternativas** | NICE-SLAM (Apache), MASt3R-SLAM, Kimera-VIO (BSD) |
| **License** | GPL-3.0 (ORB-SLAM3) ou BSD (OpenVSLAM) — verificar |
| **Dependências** | C++ (ORB-SLAM3), OpenCV, Eigen. Compilação nativa |
| **CPU** | 8+ cores (SLAM é CPU-intensive) |
| **GPU** | Opcional (acelera) |
| **VRAM** | N/A (CPU) ou 2GB (se GPU) |
| **Disco** | ~500MB (ORB-SLAM3 binário) |
| **Integração Unreal** | Pose + point cloud via WebSocket → Cosca |
| **Integração Cosca** | Novo pacote `internal/spatial/provider.go` + `internal/spatial/slam/` |
| **Risco** | Alto — SLAM é complexo, compilação C++ em Windows é difícil |
| **Prioridade** | **2** (destrava navegação) |
| **Critério de aceitação** | Dado sequência de frames, o sistema retorna: pose 6DoF, point cloud 3D, mapa de entidades |

### Camada 7: VFX

| Campo | Valor |
|-------|-------|
| **Capacidade** | O mundo é vivo (partículas, fluidos, cloth) |
| **Já existe** | Nada |
| **Falta** | VFXProvider interface, particle system, fluid simulation, cloth |
| **Projeto escolhido** | Taichi (Apache-2.0, Python, motor completo) |
| **Alternativas** | PBD (MIT, cloth), SPlisHSPlasH (MIT, fluidos) |
| **License** | Apache-2.0 |
| **Dependências** | Python 3.10+, Taichi (pip install taichi). Execução via subprocesso |
| **CPU** | 4+ cores |
| **GPU** | Recomendado (Taichi roda em GPU) |
| **VRAM** | 2-4GB (partículas moderadas) |
| **Disco** | ~500MB (Taichi runtime) |
| **Integração Unreal** | Partículas/simulação via WebSocket → frames → Niagara/UE |
| **Integração Cosca** | Novo pacote `internal/vfx/provider.go` + `internal/vfx/taichi/` |
| **Risco** | Médio — Taichi é Python, subprocesso é simples |
| **Prioridade** | **3** (destrava "vida" visual) |
| **Critério de aceitação** | Dado config (tipo, resolução, seed), retorna sequência de frames de partículas + metadata |

### Camada 8: Audio

| Campo | Valor |
|-------|-------|
| **Capacidade** | O mundo tem som, o agente fala/ouve |
| **Já existe** | `internal/voice/` (TTS concatenativa PT-BR — limitado) |
| **Falta** | AudioProvider interface, STT (Whisper), TTS moderno (Coqui), spatial audio, music gen |
| **Projeto escolhido** | whisper.cpp (MIT, CPU, ★53k) + Coqui TTS (MPL) + AudioCraft (MIT) |
| **Alternativas** | faster-whisper (MIT), Bark (MIT), SenseVoice (MIT) |
| **License** | MIT + MPL-2.0 (compatíveis) |
| **Dependências** | whisper.cpp (C, binário), Coqui TTS (Python), AudioCraft (Python) |
| **CPU** | whisper.cpp: 2+ cores (CPU). AudioCraft: 4+ cores |
| **GPU** | Opcional para whisper.cpp. Recomendado para Coqui/AudioCraft |
| **VRAM** | whisper.cpp: 0 (CPU). Coqui: 2GB. AudioCraft: 4-8GB |
| **Disco** | ~1GB (whisper.cpp binário + modelos) |
| **Integração Unreal** | Áudio via WebSocket → Cosca → processamento → resultado |
| **Integração Cosca** | Extensão de `internal/voice/` + novo `internal/audio/provider.go` |
| **Risco** | Baixo — whisper.cpp é C puro, fácil de integrar |
| **Prioridade** | **4** (destrava comunicação) |
| **Critério de aceitação** | STT: dado WAV, retorna texto. TTS: dado texto, retorna WAV. Ambiente: dado descrição, retorna áudio |

### Camada 9: Destruction

| Campo | Valor |
|-------|-------|
| **Capacidade** | O mundo é modificável (destruição, ragdoll) |
| **Já existe** | `internal/gameengine/` (ECS com physics component — básico) |
| **Falta** | DestructionProvider interface, fracture, ragdoll, soft body, vehicle |
| **Projeto escolhido** | MuJoCo (Apache-2.0, Python bindings) via subprocesso |
| **Alternativas** | Box2D (MIT, 2D), Jolt (MIT, leve) |
| **License** | Apache-2.0 |
| **Dependências** | MuJoCo (pip install mujoco). Binário C++ embutido |
| **CPU** | 4+ cores |
| **GPU** | Opcional |
| **VRAM** | N/A |
| **Disco** | ~100MB (MuJoCo) |
| **Integração Unreal** | Simulação via WebSocket → resultado → Apply no UE |
| **Integração Cosca** | Novo pacote `internal/destruction/provider.go` |
| **Risco** | Médio — MuJoCo é maduro, mas integração é nova |
| **Prioridade** | **5** |
| **Critério de aceitação** | Dado estado + ação, retorna estado após simulação física |

### Camada 10: Simulation

| Campo | Valor |
|-------|-------|
| **Capacidade** | O mundo evolui (clima, ecossistema, economia) |
| **Já existe** | Nada |
| **Falta** | SimulationProvider interface, climate rules, agent-based sim, economy |
| **Projeto escolhido** | Mesa (Apache-2.0, Python, framework ABM completo) |
| **Alternativas** | NetLogo (GPL), ABCE (economia) |
| **License** | Apache-2.0 |
| **Dependências** | Mesa (pip install mesa). Python puro, sem dependências externas |
| **CPU** | 2+ cores |
| **GPU** | N/A |
| **VRAM** | N/A |
| **Disco** | ~10MB |
| **Integração Unreal** | Estado do mundo via WebSocket → UE renderiza |
| **Integração Cosca** | Novo pacote `internal/simulation/provider.go` |
| **Risco** | Baixo — Mesa é Python puro, leve |
| **Prioridade** | **6** |
| **Critério de aceitação** | Dado config de agentes + regras, retorna N steps de simulação com dados coletados |

---

## 3. Arquitetura Proposta

### 3.1 Princípio fundamental

**Cosca = cognition/runtime**
**Unreal = body/rendering/physics/world**
**Adapters = nervous system**

Módulos externos devem ser **substituíveis**. Se CLIP for substituído, o Cosca não reescreve.

### 3.2 Diagrama de arquitetura

```
┌─────────────────────────────────────────────────────────┐
│                    UNREAL ENGINE                         │
│  Camera → Frame → Render → Physics → Audio → VFX       │
│      ↑                                    ↓              │
│      │         WebSocket/HTTP Bridge      │              │
│      │         (port 14120)               │              │
└──────┼───────────────────────────────────┼──────────────┘
       │                                   │
       ↓                                   ↓
┌──────────────────────────────────────────────────────────┐
│                     COSCA CORE                           │
│                                                          │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │
│  │  Perception  │  │ World Model │  │  Decision    │     │
│  │  (input)     │→ │ (spatial+   │→ │  (deliberate │     │
│  │             │  │  temporal)   │  │   + plan)    │     │
│  └─────────────┘  └─────────────┘  └─────────────┘     │
│       ↑                                   ↓              │
│  ┌─────────────┐                   ┌─────────────┐     │
│  │  Memory      │                   │  Action      │     │
│  │  (5 layers)  │←──────────────────│  (output)    │     │
│  └─────────────┘                   └─────────────┘     │
│                                                          │
│  ┌────────────────────────────────────────────────────┐ │
│  │              PROVIDER INTERFACES                     │ │
│  │  VisionProvider  SpatialProvider  AudioProvider     │ │
│  │  VFXProvider  DestructionProvider  SimulationProvider│ │
│  └────────────────────────────────────────────────────┘ │
│       ↑           ↑           ↑           ↑              │
└───────┼───────────┼───────────┼───────────┼──────────────┘
        │           │           │           │
        ↓           ↓           ↓           ↓
┌──────────────────────────────────────────────────────────┐
│                    ADAPTERS                               │
│  CLIP/SAM/GroundingDINO  ORB-SLAM3  Whisper/Coqui       │
│  Taichi  MuJoCo  Mesa                                   │
│  (cada adapter = subprocesso Python ou binário C++)      │
└──────────────────────────────────────────────────────────┘
```

### 3.3 Tipos fundamentais (World Model)

```go
// internal/worldmodel/types.go

// WorldEntity representa uma entidade no mundo
type WorldEntity struct {
    ID          string
    Type        string            // "object", "npc", "structure", "terrain"
    Position    Vec3              // coordenadas 3D
    Rotation    Quat              // orientação
    Scale       Vec3              // tamanho
    BoundingBox AABB              // caixa delimitadora
    Label       string            // "red car", "tree"
    Confidence  float64           // 0-1
    Depth       float64           // distância ao agente
    Embedding   []float32         // feature vector (CLIP/DINOv2)
    Metadata    map[string]string // dados extras
    LastSeen    time.Time
    Persistent  bool              // sobrevive entre frames
}

// SpatialObservation é uma observação completa do mundo
type SpatialObservation struct {
    Timestamp   time.Time
    AgentPose   Pose6DoF          // posição+orientação do agente
    Entities    []WorldEntity     // entidades detectadas
    DepthMap    [][]float32       // mapa de profundidade
    PointCloud  []Vec3            // pontos 3D (do SLAM)
    Relations   []SpatialRelation // relações espaciais
    AudioEvents []AudioEvent      // eventos acústicos
}

// SpatialRelation é uma relação entre duas entidades
type SpatialRelation struct {
    Subject   string  // ID da entidade
    Object    string  // ID da entidade
    Relation  string  // "left_of", "behind", "near", "on_top_of"
    Distance  float64 // distância entre elas
    Confidence float64
}

// WorldState é o estado completo do mundo
type WorldState struct {
    Entities    []WorldEntity
    Relations   []SpatialRelation
    Climate     ClimateState
    Time        time.Time
    Step        int64
}

// ClimateState é o estado do clima
type ClimateState struct {
    Temperature float64
    Wind        Vec3
    Rain        float64  // 0-1
    TimeOfDay   float64  // 0-24
    Season      string   // "spring", "summer", "autumn", "winter"
}
```

### 3.4 Provider Interfaces

```go
// internal/providers/vision.go

// VisionProvider é a interface para percepção visual
type VisionProvider interface {
    // Detect retorna objetos detectados na imagem
    Detect(ctx context.Context, frame []byte) ([]Detection, error)
    
    // Segment retorna máscaras de segmentação
    Segment(ctx context.Context, frame []byte, prompt string) ([]Mask, error)
    
    // Classify retorna a classe mais provável
    Classify(ctx context.Context, frame []byte, candidates []string) (string, float64, error)
    
    // Embed retorna o embedding visual
    Embed(ctx context.Context, frame []byte) ([]float32, error)
    
    // Depth retorna o mapa de profundidade
    Depth(ctx context.Context, frame []byte) ([][]float32, error)
}

// internal/providers/spatial.go

// SpatialProvider é a interface para entendimento espacial
type SpatialProvider interface {
    // Localize retorna a pose do agente
    Localize(ctx context.Context, frame []byte, imu []float64) (Pose6DoF, error)
    
    // Map retorna o mapa 3D ao redor
    Map(ctx context.Context) (PointCloud, error)
    
    // Reconstruct reconstrói cena 3D a partir de fotos
    Reconstruct(ctx context.Context, frames [][]byte) (Mesh, error)
    
    // SpatialReasoning responde perguntas espaciais
    SpatialReasoning(ctx context.Context, question string, observation SpatialObservation) (string, error)
}

// internal/providers/audio.go

// AudioProvider é a interface para percepção/acção de áudio
type AudioProvider interface {
    // Transcribe transcreve áudio em texto
    Transcribe(ctx context.Context, audio []byte) (string, error)
    
    // Synthesize gera áudio a partir de texto
    Synthesize(ctx context.Context, text string, voice string) ([]byte, error)
    
    // ClassifySound classifica um som
    ClassifySound(ctx context.Context, audio []byte) (string, float64, error)
    
    // GenerateAmbient gera áudio ambiente
    GenerateAmbient(ctx context.Context, description string) ([]byte, error)
}

// internal/providers/vfx.go

// VFXProvider é a interface para efeitos visuais
type VFXProvider interface {
    // Simulate roda uma simulação (partículas, fluido, cloth)
    Simulate(ctx context.Context, config VFXConfig) ([]Frame, error)
    
    // Step avança a simulação um passo
    Step(ctx context.Context) ([]Frame, error)
    
    // Reset reseta a simulação
    Reset(ctx context.Context) error
}

// internal/providers/destruction.go

// DestructionProvider é a interface para destruição/modificação
type DestructionProvider interface {
    // Apply aplica uma ação física ao mundo
    Apply(ctx context.Context, action PhysicalAction) (WorldState, error)
    
    // Query consulta o estado físico
    Query(ctx context.Context, query PhysicsQuery) (PhysicsResult, error)
}

// internal/providers/simulation.go

// SimulationProvider é a interface para simulação emergente
type SimulationProvider interface {
    // Step avança a simulação N passos
    Step(ctx context.Context, n int) ([]SimulationStep, error)
    
    // GetState retorna o estado atual
    GetState(ctx context.Context) (WorldState, error)
    
    // Inject injeta um evento no mundo
    Inject(ctx context.Context, event WorldEvent) error
}
```

### 3.5 Dependency Graph

```
FASE 0 (Foundation)
  ├── worldmodel/types.go (tipos fundamentais)
  ├── providers/ (interfaces)
  ├── adapters/ (bridge pattern)
  └── config (feature flags)

FASE 1 (Vision)
  ├── providers/vision.go (interface)
  ├── adapters/clip/ (CLIP adapter)
  ├── adapters/sam/ (SAM adapter)
  ├── adapters/groundingdino/ (GroundingDINO adapter)
  ├── adapters/depth/ (Depth Anything adapter)
  └── internal/vision/ (extensão do existente)

FASE 2 (Spatial)
  ├── providers/spatial.go (interface)
  ├── adapters/slam/ (ORB-SLAM3 adapter)
  └── worldmodel/spatial.go (tipos espaciais)

FASE 3 (VFX)
  ├── providers/vfx.go (interface)
  └── adapters/taichi/ (Taichi adapter)

FASE 4 (Audio)
  ├── providers/audio.go (interface)
  ├── adapters/whisper/ (whisper.cpp adapter)
  ├── adapters/tts/ (Coqui TTS adapter)
  └── adapters/audiocraft/ (AudioCraft adapter)

FASE 5 (Destruction)
  ├── providers/destruction.go (interface)
  └── adapters/mujoco/ (MuJoCo adapter)

FASE 6 (Simulation)
  ├── providers/simulation.go (interface)
  └── adapters/mesa/ (Mesa adapter)

FASE 7 (Multi-Agent)
  ├── worldmodel/agent.go (agent identity)
  └── agents/ (extensão do existente)
```

---

## 4. Fases de Implementação

### FASE 0 — FOUNDATION (Sem dependências externas)

**Objetivo:** Preparar a infraestrutura para todos os módulos.

**O que fazer:**
1. Criar `internal/worldmodel/types.go` — tipos fundamentais (WorldEntity, SpatialObservation, SpatialRelation, WorldState, ClimateState, Pose6DoF, Vec3, Quat, AABB)
2. Criar `internal/providers/` — interfaces (VisionProvider, SpatialProvider, AudioProvider, VFXProvider, DestructionProvider, SimulationProvider)
3. Criar `internal/adapters/` — bridge pattern (adapter registry, config, lifecycle)
4. Criar `internal/config/flags.go` — feature flags (vision.enabled, audio.enabled, etc.)
5. Criar `internal/bridge/` — WebSocket client para Unreal
6. Criar testes para todos os tipos e interfaces
7. Documentar arquitetura

**Dependências:** Nenhuma externa. Apenas stdlib Go.

**Custo:** ~2-3 dias de trabalho.

**Critério de aceitação:** Todos os tipos compilam, interfaces são清晰, testes passam.

### FASE 1 — PERCEPTION (Vision)

**Objetivo:** O agente enxerga o mundo.

**O que fazer:**
1. Implementar `internal/adapters/clip/` — adapter CLIP (subprocesso Python)
2. Implementar `internal/adapters/sam/` — adapter SAM2 (subprocesso Python)
3. Implementar `internal/adapters/groundingdino/` — adapter GroundingDINO (subprocesso Python)
4. Implementar `internal/adapters/depth/` — adapter Depth Anything V2 (subprocesso Python)
5. Criar `internal/vision/pipeline.go` — frame → detect → segment → classify → depth → observation
6. Integrar com `internal/scene/` (scene graph existente)
7. Testes + benchmark

**Dependências:** Python 3.10+, PyTorch, transformers (para os adapters)

**Custo:** ~5-7 dias. ~2GB disco (modelos).

**Critério de aceitação:** Dado um frame PNG, o pipeline retorna: objetos detectados (bbox + label + score), máscaras de segmentação, embedding CLIP, mapa de profundidade. Latência < 500ms (GPU) ou < 2s (CPU).

### FASE 2 — SPATIAL WORLD MODEL

**Objetivo:** O agente entende o espaço.

**O que fazer:**
1. Implementar `internal/adapters/slam/` — adapter ORB-SLAM3 (subprocesso C++ ou Go port)
2. Implementar `internal/worldmodel/spatial.go` — WorldState, SpatialObservation
3. Integrar com `internal/scene/` (scene graph existente)
4. Integrar com `internal/memory/` (persistência do estado)
5. Testes + benchmark

**Dependências:** ORB-SLAM3 (C++ binário) ou OpenVSLAM (BSD)

**Custo:** ~5-7 dias. ~500MB disco.

**Critério de aceitação:** Dado sequência de frames, retorna: pose 6DoF, point cloud 3D, mapa de entidades com relações espaciais.

### FASE 3 — VFX / WORLD RESPONSE

**Objetivo:** O mundo é visualmente vivo.

**O que fazer:**
1. Implementar `internal/adapters/taichi/` — adapter Taichi (subprocesso Python)
2. Criar `internal/vfx/particles.go` — particle system
3. Criar `internal/vfx/fluids.go` — fluid simulation
4. Integrar com `internal/nodegraph/` (node graph existente)
5. Testes + benchmark

**Dependências:** Python 3.10+, Taichi

**Custo:** ~3-5 dias. ~500MB disco.

**Critério de aceitação:** Dado config de efeito, retorna sequência de frames de partículas/fluido com metadata.

### FASE 4 — AUDIO

**Objetivo:** O mundo tem som, o agente fala/ouve.

**O que fazer:**
1. Implementar `internal/adapters/whisper/` — adapter whisper.cpp (binário C)
2. Implementar `internal/adapters/tts/` — adapter Coqui TTS (subprocesso Python)
3. Implementar `internal/adapters/audiocraft/` — adapter AudioCraft (subprocesso Python)
4. Extensão de `internal/voice/` (TTS existente)
5. Testes + benchmark

**Dependências:** whisper.cpp (binário), Python 3.10+, Coqui TTS, AudioCraft

**Custo:** ~4-6 dias. ~1GB disco.

**Critério de aceitação:** STT: dado WAV, retorna texto. TTS: dado texto, retorna WAV. Ambiente: dado descrição, retorna áudio.

### FASE 5 — DESTRUCTION

**Objetivo:** O mundo é modificável.

**O que fazer:**
1. Implementar `internal/adapters/mujoco/` — adapter MuJoCo (subprocesso Python)
2. Extensão de `internal/gameengine/` (ECS existente)
3. Testes + benchmark

**Dependências:** Python 3.10+, MuJoCo

**Custo:** ~3-4 dias. ~100MB disco.

**Critério de aceitação:** Dado estado + ação, retorna estado após simulação física.

### FASE 6 — SIMULATION

**Objetivo:** O mundo evolui.

**O que fazer:**
1. Implementar `internal/adapters/mesa/` — adapter Mesa (subprocesso Python)
2. Criar `internal/simulation/climate.go` — climate rules
3. Criar `internal/simulation/economy.go` — economy rules
4. Testes + benchmark

**Dependências:** Python 3.10+, Mesa

**Custo:** ~3-4 dias. ~10MB disco.

**Critério de aceitação:** Dado config, retorna N steps de simulação com dados coletados.

### FASE 7 — MULTI-AGENT

**Objetivo:** O mundo tem NPCs autônomos.

**O que fazer:**
1. Extensão de `internal/agents/` (agent management existente)
2. Criar `internal/worldmodel/agent.go` — agent identity, perception, memory, goals
3. Integrar com Mesa (multi-agent sim)
4. Integrar com `internal/deliberate/` (meta-cognição)
5. Testes + benchmark

**Dependências:** Mesa (já na FASE 6)

**Custo:** ~5-7 dias.

**Critério de aceitação:** NPCs autônomos com memória, objetivos, relações, comportamento emergente.

---

## 5. Primeiro Vertical Slice

### Objetivo

O Cosca deve conseguir:
1. **Observar** um objeto no mundo (câmera Unreal → frame)
2. **Identificar** o objeto (CLIP/GroundingDINO)
3. **Obter posição/distância** (Depth + Spatial)
4. **Registrar** no World Model
5. **Tomar decisão** simples (deliberate)
6. **Executar ação** correspondente no Unreal

### Pipeline

```
UNREAL CAMERA
    ↓
FRAME (PNG via WebSocket)
    ↓
VISION PIPELINE
    ├── YOLO/Detect → "o que tem aqui?"
    ├── GroundingDINO → "onde está o X?"
    ├── SAM → "qual a forma?"
    ├── CLIP → "isso é o quê?"
    └── Depth → "quão longe?"
    ↓
SPATIAL OBSERVATION
    ├── Pose 6DoF (onde estou?)
    ├── WorldEntity[] (o que vejo?)
    └── SpatialRelation[] (onde está cada coisa?)
    ↓
WORLD MODEL
    ├── WorldState atualizado
    └── Memória persistida
    ↓
COSCA (deliberate + decide)
    ├── "Devo agir?"
    ├── "Qual ação?"
    └── "Qual o objetivo?"
    ↓
ACTION (via WebSocket → Unreal)
    ├── Mover para objeto
    ├── Interagir com objeto
    └── Reportar observação
```

### Critérios objetivos de sucesso

| Critério | Métrica |
|----------|---------|
| Frame recebido | < 100ms do Unreal para Cosca |
| Detecção de objetos | ≥ 3 objetos detectados em cena típica |
| Classificação | ≥ 80% accuracy em top-3 |
| Profundidade | Erro < 10% em distâncias até 10m |
| Pose estimation | Erro < 10cm em posição, < 5° em orientação |
| World Model atualizado | < 500ms frame → world state |
| Decisão tomada | < 100ms world state → action |
| Ação executada | < 200ms decision → Unreal action |
| **Latência total** | **< 2s frame → ação no mundo** |

### Hardware mínimo para vertical slice

| Componente | Mínimo | Recomendado |
|------------|--------|-------------|
| CPU | 4 cores | 8+ cores |
| RAM | 8GB | 16GB+ |
| GPU | Não obrigatória | GTX 1660 / RX 6600 |
| VRAM | 0 (CPU) | 4GB+ |
| Disco | 5GB livres | 10GB+ |
| OS | Windows 10/11 | Windows 11 |

---

## 6. Riscos

| Risco | Probabilidade | Impacto | Mitigação |
|-------|---------------|---------|-----------|
| Latência de subprocessos Python | Alta | Alto | ONNX runtime (futuro), batch processing |
| Compilação ORB-SLAM3 em Windows | Alta | Alto | Usar OpenVSLAM (BSD) ou binário pré-compilado |
| Tamanho dos modelos (VRAM) | Média | Médio | Modelos pequenos (CLIP-S, SAM2-tiny), CPU fallback |
| Breaking changes em APIs externas | Baixa | Médio | Versionamento de adapters, testes de integração |
| Complexidade de integração Unreal | Média | Alto | WebSocket simples, protocolo bem definido |
| zero test coverage existente | Alta | Alto | Escrever testes para novos módulos (não modificar existentes) |
| Dependências Python conflitantes | Média | Médio | virtualenv por adapter, containers |

---

## 7. Recursos Necessários

### Por fase

| Fase | Dias | Disco | GPU | Python |
|------|------|-------|-----|--------|
| FASE 0 | 2-3 | 0 | Não | Não |
| FASE 1 | 5-7 | 2GB | Recomendada | Sim |
| FASE 2 | 5-7 | 500MB | Opcional | Não (C++) |
| FASE 3 | 3-5 | 500MB | Recomendada | Sim |
| FASE 4 | 4-6 | 1GB | Recomendada | Sim |
| FASE 5 | 3-4 | 100MB | Opcional | Sim |
| FASE 6 | 3-4 | 10MB | Não | Sim |
| FASE 7 | 5-7 | 10MB | Não | Sim |
| **Total** | **30-43 dias** | **~4GB** | Variável | Sim |

### Por hardware

| Hardware | Mínimo | Recomendado |
|----------|--------|-------------|
| CPU | 4 cores | 8+ cores (Ryzen 7 5700X3D ✓) |
| RAM | 8GB | 16GB+ (32GB ✓) |
| GPU | Não | RX 6700 XT ✓ |
| VRAM | 0 | 12GB (RX 6700 XT ✓) |
| Disco | 5GB | 10GB+ (NVMe 2TB em compra ✓) |
| OS | Windows | Windows 11 ✓ |

---

## 8. IMPLEMENTATION READY

### Checklist

- [x] Auditoria completa do codebase (32 packages, 92 CLI commands, 52 REST endpoints)
- [x] Gap matrix para cada camada (6 gaps identificados)
- [x] Arquitetura proposta (Cosca = cognition, Unreal = body, Adapters = nervous system)
- [x] Dependency graph (7 fases, sem ciclos)
- [x] Ordem das fases (Foundation → Vision → Spatial → VFX → Audio → Destruction → Simulation → Multi-Agent)
- [x] Riscos identificados (7 riscos com mitigação)
- [x] Recursos necessários (30-43 dias, ~4GB disco)
- [x] Primeiro vertical slice definido (câmera → vision → spatial → world model → decide → action)
- [x] Critérios objetivos de sucesso (9 critérios mensuráveis)
- [x] Hardware compatível (Don tem: Ryzen 7 5700X3D, 32GB, RX 6700 XT)

### Próximo passo

**FASE 0 — Foundation.** Começar por `internal/worldmodel/types.go` + `internal/providers/` + `internal/adapters/` + `internal/bridge/`. Sem dependências externas. Sem instalações. Apenas código Go puro.

**Quando o Don aprovar, eu inicio a FASE 0.**

---

## Related Patterns

- [`mining-map-world-vivo.md`](mining-map-world-vivo.md) — O plano de mineração que originou este plano
- [`vision-layer-patterns.md`](vision-layer-patterns.md) — Detalhes técnicos da camada Vision
- [`spatial-ai-layer-patterns.md`](spatial-ai-layer-patterns.md) — Detalhes técnicos da camada Spatial
- [`vfx-layer-patterns.md`](vfx-layer-patterns.md) — Detalhes técnicos da camada VFX
- [`audio-layer-patterns.md`](audio-layer-patterns.md) — Detalhes técnicos da camada Audio
- [`destruction-layer-patterns.md`](destruction-layer-patterns.md) — Detalhes técnicos da camada Destruction
- [`simulation-layer-patterns.md`](simulation-layer-patterns.md) — Detalhes técnicos da camada Simulation
