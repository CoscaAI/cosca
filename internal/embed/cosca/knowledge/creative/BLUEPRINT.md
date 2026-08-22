# COSCA ENGINE — BLUEPRINT v1.0

> **Fase 0 — Research Engine · Síntese-mestre · 2026-08-13 · Kernel**
>
> Este documento cruza os **64 projetos de referência** pesquisados (research/01-05) com o
> **inventário existente do Cosca** (65 engines, workflow DAG, providers, cache, security, knowledge,
> cosca-media) e define os **contratos das engines compartilhadas** + o **roadmap da Fase 1**.
>
> **v1.0 (2026-08-13): O ECOSSISTEMA COMPLETO está de pé.** As 9 fases do manifesto §37 foram
> entregues (Engine → Editor → Image → Cinema → Music → 3D → Game → Scientific → Cross-Product),
> o §31 (IA→projeto) está materializado e o editor integra tudo. Este blueprint é o MAPA oficial.
>
> Princípio-guia (manifesto §41): UNDERSTAND → RESEARCH → PLAN → BUILD → TEST → VALIDATE → OPTIMIZE → DELIVER.
> Nunca "prompt → gerar código aleatório".

---

## 1. OS 12 PRINCÍPIOS ABSOLUTOS (extraídos dos 64 projetos)

| # | Princípio | Fonte | Aplicação Cosca |
|---|---|---|---|
| P1 | **Grafo = dados puros, execução derivada e cacheável por assinatura de inputs** | ComfyUI | Node graph do Cosca: serializar como JSON, ordenar topologicamente, executar por demanda, só re-executar o que mudou. Cache por assinatura recursiva (inputs + IS_CHANGED), nunca por node_id |
| P2 | **Durable execution ≠ workflow engine** | Temporal | §40 (resumível): workflows puros + Activities com retry; state = history append-only; replay determinístico; time-skipping em testes |
| P3 | **Centralize a definição, não a execução** | transformers | Contratos canônicos de Model/Task atrás de camada própria; providers plugáveis (local/remote/GPU/CPU/API) |
| P4 | **Separe intenção de mecânica** | diffusers | Camada declarativa de geração (prompt→intenção) desacoplada de schedulers/backends — o usuário pede o resultado, o engine escolhe o pipeline |
| P5 | **I/O e codecs sempre via FFmpeg** | FFmpeg | Nunca reescrever parsing de mídia; IA integra como AVFilter/subprocesso preservando A/V sync |
| P6 | **Timeline sobre motor embutível, UI fina** | MLT/Kdenlive/Shotcut | Embutir motor (não NLE monolítico); LGPL ok para produto comercial; UI própria por cima |
| P7 | **Composição de cena em tempo real na GPU** | OBS | Scene-graph + shaders para preview/streaming (VAAPI/OpenGL/Vulkan, AMD-friendly), separado do render de timeline |
| P8 | **Thread de áudio de tempo real nunca bloqueada por IA** | PortAudio/JUCE | Callbacks só copiam buffers para filas lock-free; DSP pesado (Demucs/Whisper/RIFE) em serviços assíncronos |
| P9 | **Camada gráfica multi-backend segura** | wgpu | Render uma vez → Vulkan/Metal/D3D/WebGPU; nativa na gfx1030 (Vulkan 1.3). Vulkan-Docs como registry de verdade, não API de produto |
| P10 | **Cena = dados declarativos, não árvore imperativa** | OpenUSD/Blender/Godot | IA edita camadas/diffáveis; runtime resolve. OpenUSD para interchange, não runtime |
| P11 | **Durable/columnar/lazy para dados** | Polars/NumPy | Engine columnar Arrow + lazy + streaming; núcleo numérico estável com contrato de API |
| P12 | **Parsing incremental + CST erro-tolerante no editor** | tree-sitter | Não regex; parsing off-thread; semântica via LSP; core de editor exposto via RPC (padrão Neovim) |

## 2. AS 8 ARMADILHAS FATAIS (a evitar sempre)

| # | Armadilha | Fonte | Defesa Cosca |
|---|---|---|---|
| A1 | GPL/AGPL no núcleo proprietário | libvips(LGPL), PyMuPDF(AGPL), JUCE(GPL), Zed(GPL) | Mapa de licenças por dependência; linkagem dinâmica/processo separado quando necessário |
| A2 | CUDA-only como dependência central | Open3D, SAM2 | GPU = AMD ROCm (gfx1030): validar por projeto; preferir Vulkan/OpenCL/VAAPI + PyTorch-ROCm |
| A3 | Cache ingênuo por node_id (resultado obsoleto) | ComfyUI anti-pattern | Cache por assinatura de inputs + dependências transitivas |
| A4 | Tratar repo de pesquisa como produção | SAM2, Real-ESRGAN | Congelar versões de code/checkpoint/config como artefato único |
| A5 | Not-invented-here / over-engineering | build-your-own-x, system-design-primer | Reusar engine maduro; só microservice com evidência de necessidade |
| A6 | Parser de mídia desprotegido no processo principal | ImageMagick, assimp, SDL | Sandbox/worker para parsers; formatos seguros como primários (glTF/USD) |
| A7 | Efeitos colaterais no workflow (quebra replay) | Temporal | Workflows puros; efeitos reais só em Activities com retry |
| A8 | Lógica não-determinística (clock/RNG/I/O direto) | Temporal | Injetar tempo/semeadura; testar com time-skipping |

## 3. MAPEAMENTO MANIFESTO × EXISTENTE (não reimplementar)

| Manifesto | Engine | Estado no Cosca | Ação Fase 1 |
|---|---|---|---|
| §1 PROJECT | project engine | ⚠️ `.cosca/` ad-hoc; sem schema de manifesto | Criar **Project Manifest** (§34): schema YAML/JSON de projeto |
| §2 ASSET | asset engine | ❌ inexistente | Criar **Asset Registry**: ID/TYPE/HASH/SIZE/METADATA/DEPENDENCIES/VERSION/PREVIEW/DERIVATIVES; content-addressable quando fizer sentido |
| §3 AI | ai engine | ✅ `internal/providers`, `internal/models` | Estender para **Model Registry** (§18) com formatos/quantização/VRAM/capabilities/licença |
| §4 AI-TASK | ai task engine | ⚠️ tasks ad-hoc | Normalizar **18 tipos de tarefa** do manifesto em enum canônico |
| §5 WORKFLOW | workflow engine | ✅ `internal/workflow` (typed-routing DAG) | + durable execution (Temporal-style) para longas tarefas |
| §6-15 EDITOR+PRODUTOS | — | ⚠️ `cosca-desktop` (Wails) | Fase 2+ — Editor universal com modos |
| §16 MEDIA | media engine | ✅ `cosca-media` (visual) + ffmpeg | Estender para vídeo/áudio/3D (decode→process→AI→filter→encode→validate) |
| §17 GPU | gpu engine | ⚠️ rocminfo detectado manualmente | Criar **GPU Probe** (ROCm/Vulkan/VAAPI/OpenCL) + scheduler CPU/GPU/remote |
| §18 MODEL | model registry | ⚠️ `internal/models` | Ver §3 |
| §19 KNOWLEDGE | knowledge engine | ✅ `internal/knowledge` + matriz 23/50 stacks | Estender `knowledge/creative/` (image/video/audio/cinema/game/scientific/3d/animation/documents) |
| §20 PLUGIN | plugin engine | ✅ `internal/plugins` | Estender: tools/models/importers/exporters/filters/effects/providers/codecs/workflows/panels/nodes |
| §21 NODE-GRAPH | node graph | ✅ `internal/workflow` + `internal/graph` | Adicionar **Graph Data Model serializável** (JSON) + cache por assinatura (P1) |
| §22 RENDER | render engine | ❌ | Determinístico+cacheável+resumível+paralelo+observável; preview/draft/final; não re-render o inalterado |
| §23 CACHE | cache engine | ✅ `internal/cache` + knowledge cache | Estender para AI output/render/thumbnail/preview/decoding/embedding; chave = input+params+model-version |
| §24 VERSION | version engine | ⚠️ git | Snapshot/diff/rollback/branch/merge para project/asset/model/workflow/scene/doc/experiment |
| §25 COLLAB | collab engine | ❌ | Arquitetura preparada (multi-user/comments/presence/locking/sharing/permissions) — não implementar ainda |
| §26 SECURITY | security engine | ✅ `internal/security` + Stack 17 | Estender para media/models/plugins/scripts/projects; sandbox de parsers (A6) |
| §27 OBSERVABILITY | observability | ✅ `internal/observability` | Registrar task/model/GPU/CPU/mem/VRAM/time/latency/error/cache/output |
| §28 PERFORMANCE | scheduler | ✅ `internal/scheduler` | Detectar CPU/GPU/IO/network/memory-bound; escolher CPU/GPU/remote por performance/custo/disponibilidade/qualidade |
| §29-31 AI ASSISTANT | unified ai | ✅ engines de chat/agentes | Natural language → decomposição → pipeline de projeto |
| §32-33 INTEGRIDADE | provenance | ❌ | Creative/Scientific integrity: registrar model/prompt/seed/parameters/source/process/edit history |
| §34-35 MANIFEST/LICENÇA | provenance | ❌ | Project manifest + license registry (source/license/version/modifications/attribution) |
| §36 IDENTIDADE | design | ✅ `knowledge/design` + design system | Aplicar Cosca Design Language em cada produto |

## 4. CONTRATOS DAS ENGINES COMPARTILHADAS (Fase 1)

### 4.1 Project Engine
```yaml
# cosca.project.yaml — Project Manifest (§34)
project:
  name: string
  version: semver
  engine_version: string
  type: editor|image|cinema|music|game|scientific|3d|animation|document|lab
  models: [{id, provider, version}]
  assets: [asset_id]
  workflows: [workflow_id]
  dependencies: [{id, version, license}]
  render_settings: {}
  plugins: []
  licenses: []
```

### 4.2 Asset Engine
```go
type Asset struct {
  ID           string            // content-addressable (hash) quando aplicável
  Type         AssetType         // IMAGE|VIDEO|AUDIO|3D|FONT|TEXT|DATA|MODEL|MATERIAL|SCRIPT|DOCUMENT
  Hash         string            // sha256 do conteúdo
  Size         int64
  Metadata     map[string]any    // format, codec, dimensions, duration, exif...
  Dependencies []string          // asset IDs
  Version      string
  Source       string            // provenance (§33/§35)
  Preview      string            // caminho do derivative de preview
  Derivatives  []string          // thumbnails, transcodes, resized...
}
```
Regras: nunca duplicar; derivatives derivam do hash; edição não-destrutiva (ORIGINAL + OPS).

### 4.3 AI Engine — Model Registry (§18)
```go
type Model struct {
  ID           string
  Provider     string            // local|remote|openai|anthropic|hf|ollama|rocm...
  Version      string
  Format       string            // safetensors|gguf|onnx|torch...
  Quantization string            // fp16|int8|int4...
  VRAM         int64             // requisito em bytes
  Capabilities []TaskType        // que tarefas §4 ele executa
  License      string            // §35
}
```

### 4.4 Node Graph — Graph Data Model (P1)
```json
{
  "nodes": [
    {"id": "n1", "type": "load_image", "params": {"path": "a.png"}},
    {"id": "n2", "type": "segment", "params": {"model": "sam2"}},
    {"id": "n3", "type": "remove_bg", "inputs": ["n1", "n2"]},
    {"id": "n4", "type": "upscale", "inputs": ["n3"], "params": {"model": "realesrgan"}},
    {"id": "n5", "type": "export", "inputs": ["n4"], "params": {"format": "png"}}
  ]
}
```
Execução: ordenação topológica → demanda → cache por assinatura (inputs+params+model-version).
Serializável = workflows versionáveis, compartilháveis e reproduzíveis (§33).

### 4.5 Render Engine (§22)
- Determinístico (seed/semeadura explícita) · Cacheável (assinatura) · Resumível (checkpoint) · Paralelo (tiles/frames) · Observável.
- Qualidade: PREVIEW (rápido) < DRAFT < FINAL (completo).
- Não re-renderizar partes inalteradas (hash de entrada de cada nó).

### 4.6 GPU Engine Probe (§17)
```yaml
gpu:
  vendor: amd
  family: gfx1030
  model: "Radeon RX 6700 XT"
  vram_gib: 12
  rocm: true          # ROCk loaded, runtime 1.18
  vulkan: 1.3
  vaapi: true
  opencl: true
  cuda: false
```
Scheduler decide CPU/GPU/remote por: performance · custo · disponibilidade · qualidade (§28).

### 4.7 Security invariantes (§26)
1. Todo input externo (mídia/modelos/plugins/scripts/projetos/imports/exports) é UNTRUSTED.
2. Parsers de formato rodam sandboxed/worker (nunca no processo principal).
3. Plugins/código arbitrário de projeto nunca executa com privilégio do sistema.
4. Nunca executar código de projeto com privilégio do sistema (manifesto §26 literal).
5. Provenance registrada para toda geração (model/prompt/seed/params/history — §33).

## 5. ROADMAP FASE 1 (ordem de implementação)

| Etapa | Engine | Escopo mínimo | Status |
|---|---|---|---|
| 1.1 | **Project Manifest** | Schema YAML + validação + scaffold `cosca new` | ✅ `internal/project` + `cosca project --type` |
| 1.2 | **Asset Registry** | Modelo Asset + hash + derivatives + API básica | ✅ `internal/asset` (content-addressable) + `cosca asset` |
| 1.3 | **AI Task Engine** | Enum canônico dos 18 task types + dispatch | ✅ `internal/aitask` + `cosca task` |
| 1.4 | **Model Registry** | Modelo Model + detect de models locais + integração providers | ✅ `internal/modelreg` + `cosca model` |
| 1.5 | **GPU Probe** | Detecção ROCm/Vulkan/VAAPI/OpenCL + scheduler | ✅ `internal/compute` + `internal/sched` + `cosca gpu` |
| 1.6 | **Node Graph v1** | Grafo serializável JSON + ordenação topológica + cache por assinatura | ✅ `internal/nodegraph` + `cosca ngraph` |
| 1.7 | **Render Engine v1** | Preview/draft/final + cache por nó + resumível | ✅ `internal/render` + `cosca render` |
| 1.8 | **Media Engine estendida** | vídeo (ffmpeg pipeline) + áudio + 3D import | ✅ `internal/media` + `cosca media` |
| 1.9 | **Durable Execution** | workflow resumível (replay + activities com retry) | ✅ `internal/dflow` + `cosca flow` |
| 1.10 | **Provenance + Licenças** | integridade criativa/científica (§32-33) + license registry (§35) | ✅ `internal/provenance` + `cosca provenance` |

**Critério de pronto por etapa:** unit + integration tests verdes · esteira CI · contrato documentado · exemplo executável. — ✅ todas atendidas.

## 6. DECISÕES ESTRATÉGICAS (aprovadas pelo Don em 2026-08-13)

1. **Linguagem das engines:** Go (consistente com o Cosca) + Python (venv) para camada IA/media pesada — o padrão já usado (cosca-media). Rust adiado (wgpu só se necessário). ✅
2. **Editor:** Wails (Go+Web) já existente no cosca-desktop — base da Fase 2; interface adaptativa por tipo de projeto. (Fase 2)
3. **GPU:** AMD ROCm gfx1030 como alvo primário de aceleração; CUDA fora do caminho crítico. ✅ (probe detecta RX 6700 XT: 12 GiB, gfx1030, ROCm 7.2.4, VA-API, 40 CUs)
4. **Licenças:** mapa obrigatório antes de integrar qualquer dependência de terceiros; AGPL/GPL fora do núcleo. ✅ (provenance §35 operacional)
5. **Durabilidade:** Temporal é referência de padrão, não dependência — implementado como `internal/dflow` (workflow puro + activities + replay). ✅

## 7. O ECOSSISTEMA COMPLETO (Fases 2-9 — entregue em 2026-08-13)

```
                    COSCA (v1.0 — ecossistema de pé)
                       |
        +--------------+--------------+
        |              |              |
      CREATE        ENGINEER       DISCOVER
        |              |              |
     IMAGE (▦)      CODE           SCIENCE (Σ)
     CINEMA (▶)     SYSTEMS        DATA
     MUSIC (♪)      SECURITY       MODELS
     3D (◈)         NETWORK        SIMULATION
     GAME (▣)                      RESEARCH
        |
        +-----------------------------+
                                      |
                           NODE GRAPH (≋)
                           — UM grafo, UM cache (§23)
                           — roteamento por domínio (§30)
                                      |
                              AI TASK ENGINE (18 tasks)
                                      |
                          ASSET REGISTRY (content-addressable)
                                      |
                          RENDER ENGINE (preview/draft/final)
                                      |
                                  GPU / RUNTIME
                                      |
                                  PROJECT MANIFEST
```

### 7.1 Estado por fase (§37 do manifesto)

| Fase | Produto | Status | Onde vive | Provado por |
|---|---|---|---|---|
| 1 | **COSCA ENGINE** (10 engines) | ✅ | `internal/{project,asset,aitask,modelreg,sched,nodegraph,render,media,dflow,provenance}` | ~90 testes (L218-227) |
| 2 | **COSCA EDITOR** | ✅ | `projects/cosca-code` (Wails + React) | 10 painéis, port discovery, event bus (L228) |
| 3 | **COSCA IMAGE** | ✅ | `mediaexec` (cosca-media) | WebP real via pipeline (L229) |
| 4 | **COSCA CINEMA** | ✅ | `cinexec` (ffmpeg) | a.wav + f.png reais (L230) |
| 5 | **COSCA MUSIC** | ✅ | `musexec` (ffmpeg) | MP3 real 56kbps (L231) |
| 6 | **COSCA 3D** | ✅ | `tdengine` (parser puro) | cube.obj importado (L232) |
| 7 | **COSCA GAME** | ✅ | `gameengine` (ECS puro) | level-1 com player/enemy/coin (L233) |
| 8 | **COSCA SCIENTIFIC** | ✅ | `sciengine` (experimento) | baseline vs sweep persistido (L234) |
| 9 | **CROSS-PRODUCT** | ✅ | `xexec` (orquestrador) | cinema→music gerou MP3 (L235) |
| — | **§31 IA→PROJETO** | ✅ | `project_examples` (kit por tipo) | `cosca project new --type game` (L236) |

### 7.2 Arquitetura do editor (padrão executor+endpoint+painel)

```
frontend (React) ──HTTP──► server.Handler() ──roteia──► executor do domínio
  ▼ painel                                             mediaexec/cinexec/musexec/
  node graph ≋                                        td3dexec/gameexec/sciexec
  (JSON §21)                                          cada um usa a engine do
  ▼                                                   núcleo (cosca/pkg/engine)
  cache por assinatura (§23) ──► mesmo grafo executado
```

O padrão se provou 9 vezes: **cada produto é um executor do node graph** —
nunca código separado. O editor é cliente fino; toda lógica vive no núcleo.

### 7.3 Próximos (visão §42)

1. **IA preenchendo os kits (§31 real)**: prompt → level.json/experiment.json gerado.
2. **Modo automático**: `project_type` decide o canvas default (2.1 já detecta).
3. **Dogfooding**: usar o próprio ecossistema para gerar conteúdo.
4. **WASM/GPU** nos pipelines pesados quando o Don aprovar libs (L211).

---

*Blueprint v1.0 — o ecossistema completo documentado. Evolui com o uso real.*
