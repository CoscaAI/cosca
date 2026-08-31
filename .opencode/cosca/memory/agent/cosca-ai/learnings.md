# cosca-ai — Semantic Learnings

> Auto-evolution memory. Search before acting. Record after learning.

## Seed Knowledge

### 2026-07-27 — Baseline
| Field | Value |
|-------|-------|
| **Agent** | cosca-ai |
| **Task** | Initial capability establishment |
| **Technique** | Standard ai patterns — project conventions |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #ai #baseline #initialization |
| **Related** | .opencode/cosca/memory/codebase/overview.md |
| **Learned** | Project established. Core ai patterns documented. Ready for Level 2 techniques. |
| **Next** | Level 2: Identify first advanced technique to master |

---

## Active Learnings

### 2026-07-28 — Comprehensive AI Capability Audit
| Field | Value |
|-------|-------|
| **Agent** | cosca-ai |
| **Task** | Onda 5 ativação — auditoria completa das capacidades AI do projeto |
| **Technique** | Full-stack codebase audit with dependency graph analysis, interface completeness checking, and gap identification |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #ai #audit #architecture #embeddings #vector-store #knowledge-engine #semantic-search #rag #gap-analysis |
| **Related** | internal/embeddings/, internal/vector/, internal/search/, internal/ranking/, internal/knowledge/, internal/graph/, internal/orchestration/semantic_router.go |
| **Learned** | CoscaAI possui uma arquitetura AI bem estruturada e funcional: (1) Provider Registry para embeddings com fallback chain e cache, (2) Vector Store SQLite-based com brute-force cosine similarity, (3) Hybrid Search Engine combinando FTS5 + Vector + Graph + Re-ranking, (4) Knowledge Graph com BFS/DFS/Dijkstra e 22 entity types, (5) Ranking multi-fator com BM25 + vector + graph proximity + freshness + popularity, (6) Semantic Router para agent selection via embedding similarity. Gaps: sem pipeline RAG explícito, sem model deployment/monitoring, sem content safety filters, sem prompt management, sem HNSW/IVF para scale >100K vectors, sem ADRs de AI. |
| **Next** | Criar ADR-001 (AI Architecture & RAG Pipeline Decision), priorizar P0 gaps (RAG pipeline, prompt injection safety, embedding provider completions), implementar HNSW index opcional |

### 2026-07-28 — Embedding Provider Registration Analysis
| Field | Value |
|-------|-------|
| **Agent** | cosca-ai |
| **Task** | Verificar quais provedores de embedding estão registrados e quais estão faltando |
| **Technique** | Codebase grep for RegisterEmbedding + provider init() analysis |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #embeddings #providers #registration #gap-analysis |
| **Related** | internal/providers/openaicompat/embeddings.go, internal/providers/*/init() |
| **Learned** | Apenas 4 provedores registram embeddings (Mistral, Groq, DeepSeek, Anthropic). Faltam: OpenAI nativo, Google/Gemini, Azure nativo, Ollama (local), Bedrock, Local/TF-IDF. O auto-detection no ProviderRegistry é naïf — apenas lista todos registrados sem testar conectividade/env vars. O provedor "local" (sempre disponível como fallback) NÃO implementa embeddings — é apenas para chat. |
| **Next** | Registrar provedores de embedding faltantes (OpenAI, Google, Azure, Ollama, Bedrock). Melhorar autoDetectProviders() para checar env vars e conectividade. |

### 2026-07-28 — Knowledge Graph Code Import Extraction Limitation
| Field | Value |
|-------|-------|
| **Agent** | cosca-ai |
| **Task** | Identificar limitações do extractor de imports do knowledge graph |
| **Technique** | Regex pattern analysis and language coverage assessment |
| **Level** | 2 |
| **Outcome** | partial |
| **Tags** | #graph #code-imports #language-support #limitation |
| **Related** | internal/graph/builder.go:ExtractCodeImports() |
| **Learned** | O ExtractCodeImports é Go-only (regex para `import ( ... )`). Não suporta Python (`import x` / `from x import y`), TypeScript/JavaScript (`import ... from ...`), Rust (`use ...`), Java (`import ...`), etc. Isso limita a construção do knowledge graph para projetos multi-linguagem. |
| **Next** | Implementar multi-language import extraction com strategy pattern (GoExtractor, PythonExtractor, TSExtractor, etc.) ou generalizar com regex multi-linguagem. |

### 2026-07-28 — Vector Store Scalability Ceiling
| Field | Value |
|-------|-------|
| **Agent** | cosca-ai |
| **Task** | Analisar escalabilidade do vector store SQLite |
| **Technique** | Complexity analysis of brute-force search vs ANN alternatives |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #vector-store #performance #scalability #hnsw #ann |
| **Related** | internal/vector/sqlite_vec.go, internal/vector/vector.go |
| **Learned** | O SQLiteVec usa brute-force cosine similarity (O(n) por query). Adequado para ~100K vetores. Para projetos com milhões de documentos, precisa de ANN: HNSW (hierarchical navigable small world), IVF (inverted file), ou FAISS integration. O schema SQLite atual usa BLOBs — não permite index acceleration nativa. Alternativas: pgvector (PostgreSQL), sqlite-vec extension, ou switch para Qdrant/Weaviate/Milvus para escala enterprise. |
| **Next** | Criar opção de backend HNSW (via gonum ou FAISS CGo binding) como alternativa ao brute-force. Manter SQLiteVec como default para <100K e habilitar HNSW para >100K. |

---

### 2026-08-31 — Automatic Vision Recognition Hook in the Agent Loop
| Field | Value |
|-------|-------|
| **Agent** | cosca-ai |
| **Task** | Fazer a visão ser reconhecida automaticamente no loop do agente (imagem → rodar pipeline Go ONNX → injetar entendimento semântico no contexto, sem depender do LLM ver a imagem) |
| **Technique** | Additive pre-Chat hook with dependency-injected runner (testable seam), graceful degradation, opt-in config gate |
| **Level** | 3 |
| **Outcome** | success |
| **Tags** | #vision #automatic-recognition #agent-loop #multimodal #onnx #graceful-degradation #dependency-injection #config-gate |
| **Related** | internal/cli/agent.go, internal/cli/agent_vision.go, internal/cli/agent_vision_test.go, internal/worldmodel/vision/vision_detect.go, internal/config/config.go |
| **Learned** | (1) Ponto de integração real: `runAgentToolLoop` em internal/cli/agent.go monta `messages []chat.Message` e chama `provider.Chat`. O hook plugado enriquece a conversa base (System+User) UMA vez antes do loop, não por iteração — evita re-injeção duplicada a cada turno já que `messages` cresce no loop. (2) `chat.Message` guarda imagens em `ContentParts[].ImageURL.URL` (data-URI); `Message.HasImage()` só checa image_url; o hook percorre ContentParts e decodifica só prefixo `data:image/` (sem fetch de http). (3) `vision.DecodeDataURI` + `vision.DetectImageAndRunVision` + `(*Observation).SummaryText()` são reutilizados (não recriados). (4) **Degradação graciosa**: se `runner` erro ou retorna nil, a imagem é pulada e a mensagem passa intacta; observação degradada (warnings, zero entidades) ainda gera SummaryText → o modelo é informado que viu a imagem mas a percepção degradou. (5) Anexar o resumo como ContentPart de TEXTO na MESMA mensagem que carrega a imagem (AddText) é aditivo e provider-agnostic — preserva o turno do usuário; Avoid adicionar um system message no meio da conversa. (6) `visionRunner` como seam injetável (default = `vision.DetectImageAndRunVision`) permite teste determinístico com Observation fake, sem carregar onnxruntime; `go vet` rejeita comparar função com nil (sempre false) — não usar teste guard com função. (7) Config gate: adicionado `Config.Vision.Enabled` (opt-in, default false via `DefaultEnableVision`), env `COSCA_VISION__ENABLED`; passar `projectCfg.Vision.Enabled` para `runAgentToolLoop` (assinatura mudou, único caller). |
| **Next** | (Pendência honesta) (a) SAM2/seguir para segmentação de máscara por entidade na injection — hoje só SummaryText; (b) tratar http(s) URLs de imagem (precisa de HTTP client/baixa de bytes) — fora do escopo atual; (c) decidir se enriquecimento deve reaparecer em tool-call results que devolvem imagem (ex. read de screenshot); (d) considerar reusar `AgentRequest.Context` para visão quando o runtime usar o engine (não só agent run CLI). |

---

### 2026-08-31 — Sincronização Multimodal (áudio + visão): mineração sherpa-onnx-go + pion/webrtc e PLANO do Perception Bus
| Field | Value |
|-------|-------|
| **Agent** | cosca-ai |
| **Task** | Mineração a fundo de sherpa-onnx-go (STT streaming + VAD, cgo/lib nativa Windows) e pion/webrtc (sync RTP/NTP via RTCP SR) e produzir o PLANO DE IMPLEMENTAÇÃO do Perception Bus multimodal (clock monotônico + temporal buffer 2-5s + WorldState multimodal + fusão temporal visão↔áudio) para o Don delegar |
| **Technique** | Cross-codebase mining of cgo bindings + WebRTC media-sync pattern → extract reusable Go data model (Observation multimodal, monotonic clock, ring buffer, RTP-tick↔wall-clock mapping) mapped onto existing COSCA seams (capture.go/VisionRunner/perception.Service/serve.go/config) |
| **Level** | 3 |
| **Outcome** | success |
| **Tags** | #multimodal #perception-bus #synchronization #monotonic-clock #temporal-buffer #stt #sherpa-onnx #webrtc #rtp-ntp #ring-buffer #episodic-memory #transducer #vad #onnxruntime #cgo #windows-dll |
| **Related** | internal/perception/{perception,capture}.go, internal/worldmodel/types.go (AudioEvent/SpatialObservation/WorldState), internal/worldmodel/vision/{pipeline,onnx,vision_detect}.go, internal/worldmodel/audio/{pipeline,adapters}.go (WhisperAdapter→substituir), internal/config/config.go (PerceptionConfig/VisionConfig), internal/cli/serve.go (buildPerceptionService), api/rest/{server,handler/perception}.go; repos: sherpa-onnx-go(+sherpa_onnx_windows.go), sherpa-onnx-go-windows(sherpa_onnx.go/build_windows_amd64.go/lib/x86_64-pc-windows-gnu/*.dll), pion-webrtc(track_local_static.go:WriteSample; stats.go; go.mod) |
| **Learned** | (1) **sherpa-onnx-go** é wrapper fino que só delega (`sherpa_onnx_windows.go` → `github.com/k2-fsa/sherpa-onnx-go-windows`). O binding cgo real está em `sherpa_onnx.go` e exige **3 DLLs GNU/mingw** em `lib/x86_64-pc-windows-gnu/` (`sherpa-onnx-c-api.dll`, `sherpa-onnx-cxx-api.dll`, `onnxruntime.dll`); `build_windows_amd64.go` linha 5: `#cgo LDFLAGS: -L ${SRCDIR}/lib/x86_64-pc-windows-gnu -lsherpa-onnx-c-api -lonnxruntime`. (2) **STT streaming**: `NewOnlineRecognizer(config)` → `NewOnlineStream(recognizer)` → `AcceptWaveform(sampleRate, []float32)` (PCM normalizado [-1,1], re-sample interno) → loop `for IsReady(s){Decode(s)}` → `GetResult(s)` retorna transcript **parcial** (Text/Tokens/**Timestamps []float32 em segundos** — ouro para timing multimodal) → `IsEndpoint(s)`/`Reset(s)` p/ segmentos; config `OnlineRecognizerConfig{FeatConfig{SampleRate:16000,FeatureDim:80}, ModelConfig{Transducer/Paraformer/Zipformer2Ctc/NemoCtc/ToneCtc, Tokens, NumThreads, Provider}, DecodingMethod: greedy_search|modified_beam_search, EnableEndpoint}`; 3 famílias: Transducer (encoder+decoder+joiner), Paraformer (encoder+decoder), CTC (1 onnx). (3) **VAD**: `VadModelConfig{SileroVad/...}`, `NewVoiceActivityDetector(config, bufferSizeSeconds)`, `AcceptWaveform`, `IsSpeech()`, `Front()*SpeechSegment{Start, Samples}`. (4) **pion/webrtc sync**: cada track tem `clockRate` (audio 48k/Opus, video 90k) — `track_local_static.go:328`; `WriteSample` converte `sample.Duration.Seconds()*clockRate` em RTP ticks (linha 361) acumulando `remainder` p/ drift fracionário (370-372). O RTCP **Sender Report** mapeia RTP timestamp ↔ NTP (relógio global comum) — `github.com/pion/rtcp` (SR: NTPTime+RTPTime) e o receiver combina para alinhar áudio/vídeo; `RemoteOutboundRTPStreamStats` "reported in an RTCP Sender Report (SR)" (stats.go:1032-33). **Padrão a replicar sem WebRTC**: clock monotônico Go é o "NTP"; tick media-local (sample/frame) mapeia pro clock via âncora base+tick/clockRate → fusão temporal em timeline comum. (5) **COSCA**: `perception.Service`/`Config`/`State` já existem (perception.go), com seam `VisionRunner` e `Subscribe`/`publish` non-blocking; `State.Timestamp` é `time.Time` UTC (não monotônico). `worldmodel` já tem `AudioEvent`, `SpatialObservation.AudioEvents`, `EntityAudio` — vocabulário multimodal parcialmente presente. Visão é ONNX nativa via `yalue/onnxruntime_go` (onnx.go: `DefaultModelsDir`, `initONNXRuntime` singleton, `modelSession`). Áudio atual é Python subprocess (`WhisperAdapter.Transcribe`). (6) **CRÍTICO (MODELS.md)**: na máquina alvo `C:\Windows\System32\onnxruntime.dll`=1.17.1 (API 17) mas `onnxruntime_go v1.35.0` exige API 29 (≥1.20) → visão está **degradada**; e o `onnxruntime.dll` do sherpa (build mingw) pode **colidir** com o DLL do onnxruntime_go → externalidade nº1. |
| **Next** | Fase A: `internal/perception` estendido p/ `internal/perception/bus` (Observation multimodal + MonotonicClock + temporal ring buffer 2-5s + fusão por janela) NÃO substituindo o State atual (aditivo; State passa a ser uma projeção do bus). Fase B: STT nativo via sherpa-onnx — novo `internal/worldmodel/audio/stt` (OnlineRecognizer+OnlineStream+VAD, Provider cpu) usando o MESMO padrão `DefaultModelsDir`/`initONNXRuntime`/adapter do vision/onnx.go, com config `audio.stt` e gate por build tag (evita forçar cgo nas builds CI); substituir `WhisperAdapter.Transcribe` (Python) no caminho STT. Fase C: memória episódica multimodal (persistir observações sincronizadas; reusar `internal/memory`/vector/embeddings já auditados). Resolver ONNX DLL API-version mismatch ANTES de medir a visão (senão não há ground-truth visual p/ validar a fusão). |

---

### 2026-08-31 — FASE A do Perception Bus IMPLEMENTADA (bus + handler + rotas + testes)
| Field | Value |
|-------|-------|
| **Agent** | cosca-ai |
| **Task** | Codificar a FASE A do Perception Bus (sincronização multimodal visão+áudio no runtime): package `internal/perception/bus`, config, wiring em serve.go, rotas `/v1/perception/bus` + handler, testes e verificação build/vet/test |
| **Technique** | Native-Go additive seam over existing Perception Loop: MonotonicClock (route `time.Since(epoch)`, NO `//go:linkname`), O(1) circular Ring (eviction por window+maxObs), temporal overlap matcher (`tol` default 300ms), Bus as pure consumer/adapter (never mutates perception.Service), lazy closure registration of routes (avoids nil-capture bug) |
| **Level** | 3 |
| **Outcome** | success |
| **Tags** | #multimodal #perception-bus #fase-a #monotonic-clock #ring-buffer #overlap #sse #bus-handler #worldstate #aditive #opt-in #golang |
| **Related** | internal/perception/bus/{clock,types,ring,match,bus}.go, internal/perception/bus/bus_test.go, api/rest/handler/perception_bus.go(+_test.go), api/rest/server.go (SetBusService + rotas), internal/cli/serve.go (buildPerceptionBus), internal/config/{config,defaults}.go (AudioConfig) |
| **Learned** | (1) **Clock**: escolhi `time.Since(epoch)` (via `sync.Once` + `time.Now()` epoch) em vez de `//go:linkname runtime.nanotime` — linkname é unsafe/sem contrato e quebra entre GO versions & build modes; `time.Since` usa a leitura monotônica embutida no `time.Time`, então é monotônico, sem syscall/alloc. `MonotonicTime int64` ns + `Add/Sub/Since/Duration`; `ToMonotonic` converte `time.Duration`. Unity de clock: todo o bus referenceia um único processo. (2) **Ring**: buffer circular O(1) com eviction por window (baseada no `newest` timestamp, não wall-clock — determinístico e testável) e por capacidade `maxObs` (cap hard). `Snapshots` ordenado por timestamp, `Since`, `WindowAround` (o primitivo "o que estava vendo quando ouviu X"). (3) **Overlap**: `match` liga cada obs de áudio às obs de visão via intervalo `[Ts, Ts+Duration]` com `tol` (0→intersecção estrita). `overlap` normaliza pra `aStart<=bStart`, gap<=tol, `overlapEnd=min(aEnd,bEnd)`, `ov`. Confiança = fração do áudio coberta. `MultiRel` = audio_seg + text_hint + WindowRefs(sequence/timestamp/entities) + conf. (4) **Bus**: adapter puro sobre `perception.Service.Subscribe()` — reconstrói `vision.Observation` a partir de `perception.State` (o State NÃO carrega o Observation cru; usei Entities/Relations/Warnings/LatencyMS). `Duration` da visão = LatencyMS. Áudio via `AudioSource.Stream() <-chan AudioSample` (seam injetável; `NoopAudioSource` Fase A). Ingest stamps `Now()`; publish non-blocking. `copyWorldState` defensiva. (5) **Bug latente que NÃO toquei**: em `registerRoutes`, `NewPerceptionHandler(s.perceptionSvc)` captura `s.perceptionSvc` no momento de `New()` (quando é nil — `SetPerceptionService` roda depois). Para o bus usei CLOSURE lento (`func(w,r){ handler.NewBusHandler(s.busSvc).State(w,r) }`) que lê `s.busSvc` a cada request — robusto a ordenação. (6) **Config**: `AudioConfig` (Enabled/Window/Tolerance/ModelsDir/SampleRate/ChunkMS) sob `PerceptionConfig.Audio`; defaults 5s/300ms/16000/100; env `COSCA_PERCEPTION__AUDIO__*`; validação `window/tolerance/sample_rate/chunk_ms > 0`. (7) **Testes**: bus_test.go usa seams injetáveis SEM onnxruntime — `ingestVision`/`ingestAudio` diretos (package bus) para projeção determinística; e um teste de integração real (`perception.NewService` + `WithCaptor/WithVision/WithLogger` + `Interval` curto) provando que o bus assina o loop. Handler test usa `httptest.NewServer` + `http.NewRequestWithContext` p/ cancelar o SSE (senão a Stream loop prende o `Close()`). |
