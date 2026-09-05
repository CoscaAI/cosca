# Auditoria READ-ONLY — Cluster Cognição / VISÃO — COSCA v1.5.0

**Data:** 2026-09-05
**Escopo:** `internal/vision`, `visionact`, `screen`, `perception`, `sensor`, `grounding`, `compute`, `media`, `nodegraph`, `scene`, `render`, `imageinpaint`, `actiondecode`.
**Regra do auditor:** cada afirmação vem do código-fonte real (path:linha). Nada foi escrito/modificado. Nada foi inflado.

---

## 0. Achado transversal (leia primeiro — muda a leitura do resto)

### 0.1 Há DOIS pacotes `vision` com papéis diferentes

O escopo pedido lista `internal/vision` como "visão: llava (VLM) + tesseract (OCR)". Isso é verdade, mas **o motor de visão que a casa realmente usa ao vivo é OUTRO:**

| Pacote | Papel real | Chamado por |
|---|---|---|
| `internal/vision` | **OCR determinístico (tesseract) + VLM opt-in (llava/Ollama)** | pipeline do cérebro (`brainweb`/`AnalyzeVideo`), CLI `cosca vision` NÃO usa **este** |
| `internal/worldmodel/vision` | **Motor ONNX nativo Go (CLIP + GroundingDINO + Depth + SAM)** — o "olho" real | `perception` loop, `visionact`, `screen`, `vision_detect` |

Prova: o CLI `cosca vision` (`internal/cli/vision.go:14-15`) importa `internal/worldmodel` + `internal/worldmodel/vision` e chama `vision.DetectImageAndRunVisionWithPrompt` (`vision.go:105`). Ou seja: o `cosca vision` **não** é o `internal/vision` do escopo — é o `worldmodel/vision` ONNX.

**Nota honesta:** o rótulo do escopo ("visão: llava + tesseract, backends 100% local") descreve `internal/vision` (que existe e compila), mas a visão que alimenta o Perception Loop, o `visionact`, o `screen` e o CLI `cosca vision` é `internal/worldmodel/vision`. Ambos existem; eles não são a mesma coisa.

### 0.2 Contradição no doc-comment do motor ONNX (resolvida)

O header de `internal/worldmodel/vision/pipeline.go:1-14` declara: *"Each adapter communicates with Python subprocesses via JSON."* **Isso é doc obsoleto.** O código real (`onnx.go`, `adapters.go`, `image.go`) usa `github.com/yalue/onnxruntime_go` — ONNX nativo em Go, **sem Python e sem subprocesso**. A instrumentação (preprocess CHW, normalização ImageNet/CLIP, tensores) é toda em Go. `loadModel`/`NewDynamicAdvancedSession` em `onnx.go:95-198`; adapters em `adapters.go`. O comentário engana quem lê sem ir ao código.

### 0.3 Marcadores de backend usados adiante

- `[NATIVO Go]` — implementação em Go, sem binário externo.
- `[SUBPROCESSO/external]` — invoca programa/binário externo (tesseract, ffmpeg, powershell) ou serviço HTTP externo (Ollama).
- `[no-op Windows]` — organ existe, mas nesta plataforma devolve zero/vazio.

---

## 1. `internal/vision` — 3 src / 2 test

**O que faz:** dá "olhos" ao Kernel via dois backends — `llava` (VLM multimodal local, Ollama) e `tesseract` (OCR imagem→texto) (`vision.go:1-10`).

**Backends reais:**
- **`Describe`** (default = determinístico): `exec.Command(tesseract, ...)` — `[SUBPROCESSO/external]` (`vision.go:130-151`). O default é OCR, NÃO VLM: o backend "llava" é opt-in explícito via `Options.Backend="llava"` (`vision.go:67-78`); quando indisponível, **não** degrada silenciosamente — reporta erro (`vision.go:70-72`).
- **`llava`**: HTTP POST a `OllamaURL` (`http://localhost:11434/api/generate`) — `[external HTTP]` (`vision.go:88-127`).
- **`AnalyzeVideo`** (`pipeline.go:96`): pipeline determinístico frame-a-frame. Usa `ffmpeg` (`exec.LookPath("ffmpeg")`, `pipeline.go:121`) para extrair frames e `tesseract` (`pipeline.go:157`) para OCR — ambos `[SUBPROCESSO/external]`. O **pixel-diff** (`pixelDiff`, `pipeline.go:222`) é `[NATIVO Go]`. `media.Probe`/`ffprobe` — `[SUBPROCESSO/external]`.
- **`observation.go`**: primitiva epistêmica (`Observation`, `EpistemicState`, `EvidenceLevel`) — `[NATIVO Go]`, zero dependência.

**Tamanho:** 3 src (`vision.go`, `pipeline.go`, `observation.go`) / 2 test. Exported: `Describe`, `Options`, `Result` (+`IsEmpty`/`Summary`), `AnalyzeVideo`, `FrameRecord`, `PerceptEvent`, `PipelineResult`, `PipelineOptions`, consts `Backend*`, `Event*`, `Epistemic*`, `Observation`, `EpistemicState`, `EvidenceLevel`.

**O que NÃO está implementado / limites honestos:**
- **Nenhum "entendimento da cena" em Go nativo** na `Describe`: VLM requer serviço Ollama externo em execução; OCR default só lê texto.
- O pipeline derivador de eventos (`deriveEvents`) é heurística simples (mudou número? grande pixel-diff? → evento), sinais `NeedsVLM=true` para o caso semântico (`pipeline.go:195-201`) — mas ele **não chama VLM**, apenas marca.

**Estado:** `build` OK; `go test -count=1` OK.

---

## 2. `internal/visionact` — 1 src / 1 test

**O que faz:** "percepção POR AÇÃO (sob demanda)" — o oposto do loop contínuo (`visionact.go:1-29`). `LookAtScreen` (1 frame agora) e `RecordAndInterpret` (grava N segundos e interpreta).

**Backends reais:** captura via `perception.ScreenCaptor` (kbinani/screenshot GDI — `[NATIVO Go]`); visão via `defaultVision` = `vision.DetectImageAndRunVision` de `internal/worldmodel/vision` (`visionact.go:150-152`) — `[NATIVO Go ONNX]`, mas **depende dos .onnx presentes**; ausência → `Observation` com `Warnings` (degradação, nunca erro, `visionact.go:20-28`).

**Tamanho:** 1 src / 1 test. Exported: `Engine`, `New`, `WithCaptor`, `WithVision`, `WithLogger`, `WithRecordInterval`, `WithMaxFrames`, `LookAtScreen`, `RecordAndInterpret`, `ChangeEvent`, consts `DefaultRecord*`.

**O que NÃO está:** se `internal/worldmodel/vision` não tiver modelos ONNX/Python-export não carregado, a "percepção" vira `Warnings` vazios de entidades (degradação elegante, não falha).

**Estado:** `build` OK; `go test -count=1` OK.

---

## 3. `internal/screen` — 10 src / 6 test

**O que faz:** percepção visual de tela por significado (`screen.go:1-19`): capturar (GDI) → CLIP image-encoder (ONNX) → métricas estéticas determinísticas → Kernel interpreta. `Analyze` (`screen.go:70`) e `AnalyzeMultimodal` (`screen.go:102`) (regiões + estética + CLIP + OCR opcional).

**Backends reais:**
- **Captura**: `github.com/kbinani/screenshot` (GDI no Windows) — `[NATIVO Go]`, zero binário externo (`capture_native.go`).
- **Estética** (`computeAesthetic`, `aesthetic.go:12`) e **detector de regiões de texto** (`detectRegions`, `detect.go:21`, Otsu + projeções, Go puro) — `[NATIVO Go]`.
- **CLIP**: `vision.NewClipAdapter(...)` de `internal/worldmodel/vision` — `[NATIVO Go ONNX]`, exige modelo; ausência → `HasModel=false` (degradação, `screen.go:82-92`).
- **OCR**: interface `OCRProvider` (`ocr.go:15-20`). `NoOPCR` (no-op, `ocr.go:34-38`); `WinRTOCR` → **`[SUBPROCESSO/external]`** via PowerShell (winrt_ocr.go:111-116), com upscale adaptativo 1x→2x→4x (`winrt_ocr.go:45-77`).

**Tamanho:** 10 src / 6 test. Exported: `Result`, `Aesthetic`, `Screen`, `Region`, `Rect`, `RegionKind`, `OCRProvider`, `OCRResult`, `NoOPCR`, `WinRTOCR`, `NewWinRTOCR`, `Analyze`, `AnalyzeMultimodal`, `Screen.Evidence`.

**O que NÃO está / limites honestos:**
- O sensor "text-region" só diz *"aqui provavelmente há texto"* — **não é OCR** (`detect.go:19-20`); o comportamento é declarado e correto.
- `Screen.Evidence` (`evidence.go:17`) mapeia só `RegionText` — regiões gráficas não viram observação canônica (comportamento intencional).

**Estado:** `build` OK; `go test -count=1` OK (teste de OCR de tela real é de natureza live).

---

## 4. `internal/perception` — 5 src raiz + 6 src `bus` (= 11 src) / 3 test

**O que faz:** o loop contínuo de presença (`perception.go:1-24`): capturar tela → `vision.DetectImageAndRunVision` (**worldmodel/vision** ONNX) → WorldState perceptual → SSE (`/v1/perception/stream`) + read endpoint.

**Backends reais:**
- **Captura**: `ScreenCaptor` / `CaptureScreen` (kbinani/screenshot, GDI) — `[NATIVO Go]` (`capture.go:61`).
- **Visão**: `worldmodel/vision` (CLIP + GroundingDINO + Depth) — `[NATIVO Go ONNX]` (`perception.go:14-15, 249-257`). **Não** é o `internal/vision`.
- **Change-detect** (`change_detect.go`): Go puro, grid 16×16, mean-abs-delta — `[NATIVO Go]`.
- **Memory-watchdog** (`memory_watchdog.go`): `runtime.ReadMemStats` — `[NATIVO Go]`.
- **Métricas** (`metrics.go`): `runtime/metrics` — `[NATIVO Go]`.

**Tamanho:** raiz 5 src / 1 test + `bus` 6 src / 2 test. Exported raiz: `Service`, `NewService`, `Start`, `Stop`, `State`, `Subscribe`, `Unsubscribe`, `Config`, `ChangeDetectionConfig`, `MemoryWatchdogConfig`, `Captor`, `VisionRunner`, `NewChangeDetector`, `DefaultChangeDetectionThreshold`, `CaptureScreen`, `ScreenCaptor`, `State`, `Metrics`. `bus`: `Modality`, `Observation`, `Payload`, `AudioPayload`, `Token`, `WorldState`, `MultiRel`, `WindowRef`, `Metrics`.

**O que NÃO está / limites honestos:**
- **Fase A (agora) é só visão.** O canal de áudio do bus é a preparação para a "Fase B" (**sherpa**); em Fase A a fonte de áudio é no-op e raramente populada (`bus/types.go:78-80`). Há alternates NO-OP vs SHERPA no CLI (`mic_noop.go`, `mic_sherpa.go`, `perception_stt_noop.go`, `perception_stt_sherpa.go`, `speech_tts_noop.go`, `speech_tts_sherpa.go`).
- O subpacote `bus` é um **adaptador** — nunca duplica/alterta o loop de visão.

**Estado:** `build` OK; `go test -count=1` OK (raiz + bus).

---

## 5. `internal/sensor` — 1 src raiz / 2 test (+ `fusion`, `gate`)

**O que faz:** peça (1) do dogma percepção-como-evidência — DTO sensorial normalizado `{tipo, conteúdo, confiança, fonte, estilo_epistêmico, timestamp, trace_id}` (`sensor.go:1-19`).

**Backends reais:** puro Go, **zero external** (`[NATIVO Go]`). `fusion` (consenso log-odds + contradição) e `gate` (gate de escalação — VLM no topo) são também Go puro.

**Tamanho:** raiz 1 src / 2 test; `fusion` 1 src / 1 test; `gate` 1 src / 2 test (semântica: `semantic_continuity_test.go`, `continuity_test.go`). Exported: `EpistemicState`, `Modality`, `Region`, `Confidence`, `Observation`, `New`, `Inferred`, `Corroborated`, `WithRegion`, `WithID`, `WithTrace`, `IsTrustworthy` (+ fusion `Fuse`, `Fused`, `Contradiction`; gate `Policy`, `Decide`, `Verdict`).

**O que NÃO está:** nenhuma inferência própria — é o "DNA" dos sensores. As outras 3 peças (fusão, gate, ledger) consomem esse DTO.

**Estado:** `build` OK; `go test -count=1` OK.

---

## 6. `internal/grounding` — 9 src / 7 test

**O que faz:** gates de fidelidade anti-alucinação do RAG (ADR-011, fatia G1): extração de claims, verificação por sobreposição com chunks, fidelidade (veredito), recall@K (MRR), qrels, cascata de gating, e `BuildAnswer` ("sem evidência não gera").

**Backends reais:** puro Go. O LLM é **injetado** via `LLMFunc` (`answer.go:18`) — nenhuma inferência nativa aqui. `buildGroundingInputs`/tokenizer não pertencem a este pacote (são de `worldmodel/vision`).

**Tamanho:** 9 src / 7 test. Exported: `Claim`, `SourceChunk`, `ClaimVerdict`, `VerifyOptions`, `EffectiveVerifyOptions`, `VerifyClaim`, `ExtractClaims`, `FidelityReport`, `Verdict`, `Verify`, `RecallGate`, `NewRecallGate`, `Retriever`, `Floor`, `DefaultFloor`, `RecallReport`, `Qrels`, `QrelsFile`, `EmbeddingSig`, `LoadQrels`, `LoadQrelsFile`, `ValidateQrels`, `RealBenchmarkQrels`, `RealBaselineFile`, `RealBenchmarkDocuments`, `LLMFunc`, `BuildAnswer`, `CascadeVerdict`, `Gater`, `Config`, `DefaultConfig`, `EnvConfig`, `HeuristicGater`, `NewHeuristicGater`, `HHEMGate`, `Choose`, `WithConfig`.

**Estado:** `build` OK; `go test -count=1` OK.

---

## 7. `internal/compute` — 17 src / 15 test

**O que faz:** o Compute Fabric — engine de execução multi-core adaptativa: detecta hardware e escala pools automaticamente (`fabric.go:1-24`, `hardware.go:1-4`).

**Backends reais:**
- **Pools, circuit-breaker, rate-limiter, memory-budget** (`pool.go`, `backpressure.go`) — `[NATIVO Go]`.
- **GPU executor (OllamaExecutor)**: HTTP POST a servidor Ollama local (`/api/generate`) — `[external HTTP]` (`gpu.go:153-219`). Setado via `SetGPUExecutor`; pool "gpu" nasce dormindo (`MinWorkers 0`, `fabric.go:142-145`).
- **Probes de hardware**: maioria lê **`/proc` do Linux** — `hardware.go` (`readLoadAvg`/`readMemInfo`, `hardware.go:244-279`), `cpuprobe.go` (`/proc/cpuinfo`), `gpuprobe.go` (`sysfs /sys/class/drm`, `rocm-smi`, `rocminfo`, `nvidia-smi`, `vulkaninfo`, `clinfo`, `lspci`), `topologyprobe.go`, `environmentprobe.go`. `memoryprobe_windows.go` e `storageprobe_windows.go` têm variante Windows.

**O que NÃO está / `[no-op Windows]` (honesto, relevante):**
- `hardware.go:181-188` lê `/proc/loadavg` + `/proc/meminfo` — **no Windows TotalRAM/AvailableRAM/Load voltam 0**, logo `IsMemoryPressured` devolve `false` (`TotalRAM==0`, `hardware.go:147-151`) e `IsOverloaded` devolve `false` (load=0). RAM/uso são invisíveis no Windows.
- `gpuprobe.go`: `sysfsVendor` (Linux-only, `gpuprobe.go:280-304`) **não detecta AMD/Intel no Windows**; AMD depende de `rocm-smi`/`rocminfo`/`lspci` (Linux). Só `nvidia-smi` existe no Windows → **GPU AMD no Windows é reportada como `GPUNone`** (ou só model via lspci, que também não existe). 
- `cpuprobe`, `topologyprobe`, `environmentprobe` **não têm variante Windows** → degradam.

**Tamanho:** 17 src / 15 test. Exported (principais): `Fabric`, `NewFabric`, `Start`, `Stop`, `Submit`, `SubmitGPU`, `SetGPUExecutor`, `FanOut`, `Pool`, `Snapshot`, `StatusReport`, `FabricConfig`, `DefaultFabricConfig`, `LoadFabricConfig`, `WorkerPool`, `NewWorkerPool`, `PoolConfig`, `Task`, `TaskResult`, `CircuitBreaker`, `RateLimiter`, `MemoryBudget`, `CapabilityProfile`, `BuildCapabilityProfile`, `MachineStore`, `NewMachineStore`, `DiffProfiles`, `HardwareProbe`, `ProbeHardware`, `ProbeCPU`, `ProbeGPU`, `GPUInfo`, `GPUVendor`, `CPUInfo`, `HardwareSnapshot`, `CapabilityChangedWarning`.

**Estado:** `build` OK; `go test -count=1` OK (89s — muitos casos com sleeps de scheduler).

---

## 8. `internal/media` — 2 src / 2 test

**O que faz:** Media Engine §16 do manifesto (Fase 1, etapa 1.8) — probe e pipeline de mídia (vídeo/áudio). Princípio P5: *"I/O e codecs sempre via FFmpeg, nunca reescrever parsing"* (`media.go:1-14`).

**Backends reais:** **`[SUBPROCESSO/external]`** — `ffprobe` (`media.go:117`), `ffmpeg` (`media.go:189`). Envolve só os pipes; a Media Engine **não** é nativa de codecs.

**Tamanho:** 2 src / 2 test. Exported: `Stream`, `Format`, `Info`, `VideoStream`, `AudioStream`, `DurationSeconds`, `Probe`, `PipeOptions`, `ExtractAudio`, `Transcode`, `ExtractFrame`, `AudioPipeOptions`, `ConvertAudio`, `Validate`, `Executor`, consts `NodeLoad/Probe/ExtractAudio/Transcode/ExtractFrame/ConvertAudio/Validate`. O executor (`executor.go:41`) implementa `nodegraph.Executor` e encadeia os pipes.

**Estado:** `build` OK; `go test -count=1` OK.

---

## 9. `internal/nodegraph` — 2 src / 2 test

**O que faz:** Node Graph §21 (Fase 1, etapa 1.6) — DAG serializável de nós, ordenado topologicamente, executável por demanda, cacheadável por assinatura (P1 do ComfyUI) (`nodegraph.go:1-16`).

**Backends reais:** `[NATIVO Go]`, zero external. Execução é `graph.Run(ctx, exec, opts)` — o executor é injetado.

**Tamanho:** 2 src / 2 test. Exported: `Graph`, `Node`, `NodeType`, `New`, `AddNode`, `Validate`, `TopoOrder`, `Signature`, `Marshal`, `MarshalIndent`, `Unmarshal`, `Build`, `Executor`, `ExecutorFunc`, `Cache`, `NewCache`, `RunOptions`, `Stats`.

**Estado:** `build` OK; `go test -count=1` OK.

---

## 10. `internal/scene` — 4 src / 1 test

**O que faz:** Scene Graph — dados puros (Entity + hierarchy + Transform). `NewScene`/`AddRoot`/`AddChild`/`Find`/`Validate` (`scene.go`); construtores (`builder.go`); ponte `Entity→NodeGraph` via `Compile` (`compiler.go:24`).

**Backends reais:** `[NATIVO Go]` — somente estruturas + validação. `Compile` gera um `nodegraph.Graph` mas **exige registro de executors injetado** (`reg map[NodeType]Executor`, `compiler.go:24`); o pacote em si **não rasteriza**.

**Tamanho:** 4 src / 1 test. Exported: `Scene`, `Entity`, `EntityType`, `NewScene`, `AddRoot`, `AddChild`, `Find`, `Validate`, `Marshal`, `MarshalIndent`, `Unmarshal`, `NewCamera`, `NewLight`, `NewTerrain`, `NewProceduralEntity`, `NewGroup`, `Compile`.

**O que NÃO está:** não há renderer de Scene Graph (o rasterizador é "futuro" — `builder.go:8-9` declara DSL futuro).

**Estado:** `build` OK; `go test -count=1` OK.

---

## 11. `internal/render` — 1 src / 1 test

**O que faz:** Render Engine §22 (Fase 1, etapa 1.7) — determinístico, cacheável, resumível (checkpoint), paralelo, observável. Estende o `nodegraph` (`render.go:1-15`).

**Backends reais:** `[NATIVO Go]` — é orquestração sobre `nodegraph.Graph.Run`. O renderizador de verdade é o **`nodegraph.Executor` injetado** em `Renderer.Render(ctx, j, exec)` (`render.go:157`); este pacote não faz rasterização própria.

**Tamanho:** 1 src / 1 test. Exported: `Quality` (`Preview`/`Draft`/`Final`), `Job`, `New`, `Result`, `JobKey`, `Renderer`, `NewRenderer`, `Render`, `RenderParams`, `WithRenderContext`, `RenderFromContext`, `QualitiesList`, `SortedNodeIDs`.

**Estado:** `build` OK; `go test -count=1` OK.

---

## 12. `internal/imageinpaint` — 1 src / 1 test

**O que faz:** inpainting determinístico em Go puro (Telea/FMM — propagação de cor da fronteira) (`inpaint.go:1-8`).

**Backends reais:** `[NATIVO Go]`, sem CGO/OpenCV/IA.

**Tamanho:** 1 src / 1 test. Exported: `Inpaint(src, mask) *image.RGBA`.

**O que NÃO está (honesto):** o **doc claim** Telea/FMM **não reflete o algoritmo**. O código é uma **difusão por média ponderada de vizinhos 3×3** com loop de até 2000 iterações (`inpaint.go:44-83`) — aproximação simplificada, **sem** campo de distância (Fast Marching) nem prioridade/oitava da fronteira real. Funciona para regiões pequenas ("calçada em grama"), mas não é o FMM completo. A função `inpaintFMMVar` mencionada (`inpaint.go:87-90`) é declarada mas **não é chamada** (mantida como referência).

**Estado:** `build` OK; `go test -count=1` OK.

---

## 13. `internal/actiondecode` — 1 src / 1 test

**O que faz:** Action Decoder (ADR-035, F1) — transforma a resposta do LLM em `instruction packet` estruturado (`DECISION/TARGET/ACTION/ARGUMENTS/CONFIDENCE/VERIFICATION`). **FAIL-OPEN obrigatório** (`actiondecode.go:8-12`).

**Backends reais:** `[NATIVO Go]` — parse de string/JSON. O LLM é externo (fornece o texto). `Decode(raw)` (`actiondecode.go:69`).

**Tamanho:** 1 src / 1 test. Exported: `Decision`, `Status`, `Packet`, `Decode`.

**Estado:** `build` OK; `go test -count=1` OK.

---

## 14. Voz / Percepção — alternates NO-OP vs SHERPA

Confirmado em `internal/cli/` (não no escopo, mas relevante ao cluster): existem pares NO-OP e SHERPA — `mic_noop.go`/`mic_sherpa.go`, `perception_stt_noop.go`/`perception_stt_sherpa.go`, `speech_tts_noop.go`/`speech_tts_sherpa.go`, `voice_chat_noop.go`/`voice_chat_sherpa.go`, `voice_listen_*`, `voice_speak_*`. Ou seja: há **caminho NO-OP** (ausência do serviço = não quebra, best-effort) **e caminho SHERPA** (ASR/TTS local). A áudio no bus é Fase B (sherpa); em Fase A a fonte é no-op.

---

## 15. Tabela de resumo

| Pacote | #src | #test | Backend real | Estado | Nota honesta |
|---|---|---|---|---|---|
| `internal/vision` | 3 | 2 | tesseract/subproc + llava/HTTP Ollama | **implementado** | Default = OCR determinístico; VLM é opt-in. Não é o motor ONNX vivo. |
| `internal/visionact` | 1 | 1 | captura GDI nativo + `worldmodel/vision` ONNX | **implementado** | Percepção sob demanda; degrada a Warnings sem modelos. |
| `internal/screen` | 10 | 6 | captura GDI nativo + CLIP ONNX + métricas Go puro; OCR WinRT via powershell | **implementado** | Estética/regiões Go puro; OCR = subprocesso; CLIP degrade p/ `HasModel=false`. |
| `internal/perception` | 11 (5+6) | 3 | captura GDI nativo + `worldmodel/vision` ONNX; change/mem/metrics Go puro | **implementado** | Loop contínuo real. Audáudio é Fase B (sherpa), Fase A no-op. |
| `internal/sensor` | 1 (+fusion,+gate) | 2 (+1,+2) | **NATIVO Go** | **implementado** | DTO puro; fusão/gate Go puro. Nenhuma inferência. |
| `internal/grounding` | 9 | 7 | **NATIVO Go** + LLM injetado | **implementado** | Fidelity/recall/anti-alucinação; LLM externo via `LLMFunc`. |
| `internal/compute` | 17 | 15 | pools/backpressure Go puro; GPU via HTTP Ollama; probes **Linux-biased** | **implementado** | **[no-op Windows]**: /proc load/mem=0; GPU AMD/Intel não detectada no Win; cpu/topo/env sem variante Win. |
| `internal/media` | 2 | 2 | **ffprobe/ffmpeg subprocesso** | **implementado** | I/O e codecs via FFmpeg (por princípio). Não é nativo de codec. |
| `internal/nodegraph` | 2 | 2 | **NATIVO Go** | **implementado** | DAG serializável + cache por assinatura + executor injetado. |
| `internal/scene` | 4 | 1 | **NATIVO Go** (dados puros) | **implementado** | Só estruturas/validação; rasterizador é futuro; `Compile` exige registry. |
| `internal/render` | 1 | 1 | **NATIVO Go** (orquestração) | **implementado** | Orquestra `nodegraph`; render real depende do Executor injetado. |
| `internal/imageinpaint` | 1 | 1 | **NATIVO Go** | **implementado** | Doc diz Telea/FMM; código é difusão média 3×3 (aproximação). |
| `internal/actiondecode` | 1 | 1 | **NATIVO Go** | **implementado** | Parse JSON + fail-open; LLM fornece o texto. |

---

## 16. Estado geral (verificado por execução no Windows)

- `go build` de todos os 13 pacotes do cluster: **exit 0**.
- `go test -count=1` de todos (incluindo `worldmodel/vision` e `compute`): **todos `ok`**.
- `compute` é o mais lento (89s) por causa de testes do scheduler com sleeps reais.
- O motor de visão vivo (`worldmodel/vision`) também compila e testa OK (0.4s).

---

*Documento gerado por auditoria READ-ONLY. Nenhum arquivo do repositório foi modificado.*
