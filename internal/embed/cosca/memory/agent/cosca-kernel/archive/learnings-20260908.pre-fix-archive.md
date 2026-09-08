# cosca-kernel - learnings.md PRE-FIX (conteudo nao-registrado na chain)

> Arquivo gerado em 20260908 antes da reconstrucao do indice de gatilhos.
> Conteudo preservado - leia por grep, nunca inteiro.

# cosca-kernel — Semantic Learnings

> Auto-evolution memory. Search before acting. Record after learning.

> [!IMPORTANT — Decisão do Don 2026-08-24]
> Este arquivo é HISTÓRICO do aprendizado do framework/editor.
> NÃO recebe mais conhecimento específico do projeto.
> Conhecimento do projeto vai para: `docs/` + `.cosca/provenance.yaml` (ledger) + `.cosca/knowledge/` (knowledge index).
> Os 558 registros abaixo permanecem como proveniência — NÃO apagar.
> [!NOTE - Decisao do Don 2026-09-08]
> Arquivo reduzido para conter custo de tokens. Historico completo preservado em: archive\learnings-20260908.archive.md - leia por busca/grep, NUNCA integralmente.

## Session: 2026-08-23 — Implementation Plan (Living World)

### 2026-08-23 — Implementation Plan — de mineração para execução
| Field | Value |
|-------|-------|
| **Agent** | cosca-kernel |
| **Task** | O Don mudou de modo: MINING → IMPLEMENTATION PLANNING. Produzir plano executável para "Cosca Living World" preservando arquitetura existente. |
| **Technique** | Level 4 — Architecture + Planning. Auditoria completa (32 packages, 92 CLI, 52 REST, 0 testes, 0 C/C++), gap matrix, arquitetura, 7 fases, vertical slice. |
| **Level** | 4 |
| **Outcome** | success (planning) |
| **Confidence** | 0.88 (architecture domain) |
| **Tags** | #architecture #planning #living-world #implementation #gap-matrix |
| **Related** | .cosca/fallback/knowledge/patterns/implementation-plan.md |
| **Learned** | 1) **O Cosca já tem MUITA infraestrutura**: node graph, scene graph, ECS, procgen, media, orchestration, memory, knowledge, search, plugins, deliberation, CLI (92 cmds), API (52 endpoints). O que falta são ADAPTERS e INTERFACES. 2) **Zero test coverage** em todos os 32 packages — é o maior risco de qualidade. 3) **Zero dependências de vision/audio/spatial/physics** — todo ML é via API providers. Isso é uma FORÇA — integração via subprocessos/APIs, não bibliotecas C++ embutidas. 4) **Arquitetura definida**: Cosca = cognition, Unreal = body, Adapters = nervous system. Provider interfaces (VisionProvider, SpatialProvider, etc.) + Adapter pattern (CLIP, SAM, Whisper, etc. via subprocesso Python). 5) **World Model tipos**: WorldEntity, SpatialObservation, SpatialRelation, WorldState, ClimateState. 6) **7 fases**: Foundation (2-3d) → Vision (5-7d) → Spatial (5-7d) → VFX (3-5d) → Audio (4-6d) → Destruction (3-4d) → Simulation (3-4d) → Multi-Agent (5-7d). Total: 30-43 dias. 7) **Vertical slice**: câmera → frame → vision → spatial observation → world model → deliberate → action → Unreal. Critério: <2s latência total. 8) **Hardware do Don é suficiente**: Ryzen 7 5700X3D, 32GB, RX 6700 XT (12GB VRAM). |
| **Next** | Esperar aprovação do Don para FASE 0 (Foundation). Começar por worldmodel/types.go + providers/ + adapters/ + bridge/. Sem dependências externas. |

---

## Session: 2026-08-23 — Camadas Destruction (#9) e Simulation (#10) — as últimas

### 2026-08-23 — Destruction + Simulation — o mundo é modificável e evolui
| Field | Value |
|-------|-------|
| **Agent** | cosca-kernel |
| **Task** | Últimos gaps do mining-map-world-vivo.md: Destruction (#9) e Simulation (#10). Produzi `destruction-layer-patterns.md` (5 projetos) e `simulation-layer-patterns.md` (6 projetos). |
| **Technique** | Level 3 — Mining por camada. Busquei GitHub (stars/license) + produção de 2 docs seguindo template do professor. |
| **Level** | 3 |
| **Outcome** | success (destruction+simulation) |
| **Confidence** | 0.80 (destruction+simulation domain) |
| **Tags** | #mining #destruction #simulation #mujoco #mesa #physics #ecosystem #crowd |
| **Related** | .cosca/fallback/knowledge/patterns/destruction-layer-patterns.md, simulation-layer-patterns.md |
| **Learned** | 1) **Destruction layer**: MuJoCo (★14.6k, Apache) é o motor de simulação física de alta fidelidade (ragdoll, veículos, soft bodies). Box2D (★10.3k, MIT) para 2D. Bullet (★13k, zlib) alternativa. PhysX/Jolt já na VFX. 2) **Simulation layer**: Mesa (★3.8k, Apache) é o framework ABM completo (agentes+ambiente+scheduling). ABCE para economia. NetLogo para prototipagem. Climate/Crowd como capacidades emergentes. 3) **O mundo com Destruction+Simulation é VIVO** — o agente pode destruir (MuJoCo/PhysX), o mundo pode evoluir (Mesa: agentes+clima+economia+multidão), e o PCG reconstrói após destruição. É o loop completo: gerar→destruir→evoluir→reconstruir. 4) **As 10 camadas estão MINERADAS** — o mining-map-world-vivo.md está completo com todas as camadas cobertas. |
| **Next** | Todas as 10 camadas do mining map estão completas. O caderno tem 45 padrões. Próximo passo: quando o NVMe de 2TB chegar, executar a instalação na ordem recomendada (Vision > Spatial AI > VFX > Audio > Multi-agent > Destruction > Simulation). |

---

## Session: 2026-08-23 — Camada Audio (Gap #4 do mining map)

### 2026-08-23 — Audio Layer — o mundo tem som
| Field | Value |
|-------|-------|
| **Agent** | cosca-kernel |
| **Task** | Gap #4 do mining-map-world-vivo.md: minar a camada Audio (STT, TTS, música, entendimento sonoro) seguindo o template do professor. |
| **Technique** | Level 3 — Mining por camada. Busquei GitHub (stars/license) + produção de `audio-layer-patterns.md` (8 projetos, pipeline audio, template completo). |
| **Level** | 3 |
| **Outcome** | success (audio) |
| **Confidence** | 0.84 (audio domain) |
| **Tags** | #mining #audio #stt #tts #whisper #coqui #audiocraft #music |
| **Related** | .cosca/fallback/knowledge/patterns/audio-layer-patterns.md |
| **Learned** | 1) **Pipeline audio mapeado**: STT (Whisper/whisper.cpp) → entender (SenseVoice) → TTS (Coqui/Bark) → falar → ambiente (AudioCraft) → música (YuE). 2) **whisper.cpp é o repo mais estrelado** (★53k!) — STT que roda em CPU sem dependências. 3) **Licenças favoráveis**: Whisper/whisper.cpp/faster-whisper/Bark/AudioCraft/SenseVoice (MIT ✅), Coqui (MPL ✅), YuE (Apache ✅). 4) **Cada projeto = uma capacidade**: Whisper="ouvidos", Coqui="voz", Bark="emoção", AudioCraft="compositor", SenseVoice="cérebro auditivo". 5) **O mundo sem som é mudo** — com som, o agente ouve, fala, e o ambiente tem vida sonora. É a camada que completa a imersão sensorial (visão + som). |
| **Next** | Últimas camadas: Destruction (#9) e Simulation (#10). Depois, quando o NVMe chegar, instalar whisper.cpp+Coqui TTS+AudioCraft como primeiro módulo audio. |

---

## Session: 2026-08-23 — Camada VFX (Gap #3 do mining map)

### 2026-08-23 — VFX Layer — o mundo é vivo
| Field | Value |
|-------|-------|
| **Agent** | cosca-kernel |
| **Task** | Gap #3 do mining-map-world-vivo.md: minar a camada VFX (partículas, fluidos, cloth, destruição, física) seguindo o template do professor. |
| **Technique** | Level 3 — Mining por camada. Busquei GitHub (stars/license) + produção de `vfx-layer-patterns.md` (7 projetos, pipeline VFX, template completo). |
| **Level** | 3 |
| **Outcome** | success (vfx) |
| **Confidence** | 0.80 (vfx domain) |
| **Tags** | #mining #vfx #taichi #physics #particles #fluids #cloth |
| **Related** | .cosca/fallback/knowledge/patterns/vfx-layer-patterns.md |
| **Learned** | 1) **Pipeline VFX do mundo mapeado**: mundo → Taichi (motor principal: fluidos+partículas+cloth+destruição) → PhysX/Jolt (física: colisões+ragdoll) → PBD (cloth) → Partículas GPU (efeitos). 2) **Taichi é o âncora** (★26k, Apache-2.0, Python, motor completo) — é o coração da "vida" do mundo. 3) **Licenças favoráveis**: Taichi (Apache ✅), PhysX (BSD ✅), Jolt (MIT ✅), PBD (MIT ✅), SPlisHSPlasH (MIT ✅), bevy_hanabi (Apache ✅), PixelFlow (MIT ✅). Todas as 7 são compatíveis com Cosca. 4) **Cada projeto = uma capacidade**: Taichi="motor de vida", PhysX="física padrão", Jolt="física leve", PBD="cloth/flexibilidade", SPlisHSPlasH="fluidos altos", bevy_hanabi="referência de partículas". 5) **O mundo sem VFX é estático** — com VFX, ele tem vento, fogo, água, poeira, destruição. É a camada que separa "cenário" de "mundo vivo". |
| **Next** | Continuar com as 2 camadas restantes: Audio espacial (#8) e Destruction/Simulation (#9/#10). Depois, quando o NVMe chegar, instalar Taichi+PBD como primeiro módulo VFX. |

---

## Session: 2026-08-23 — Camada Spatial AI (Gap #2 do mining map)

### 2026-08-23 — Spatial AI Layer — o agente entende o espaço
| Field | Value |
|-------|-------|
| **Agent** | cosca-kernel |
| **Task** | Gap #2 do mining-map-world-vivo.md: minar a camada Spatial AI (SLAM, NeRF, reconstrução 3D, raciocínio espacial) seguindo o template do professor. |
| **Technique** | Level 3 — Mining por camada. Busquei GitHub (stars/license) + produção de `spatial-ai-layer-patterns.md` (7 projetos, pipeline espacial, template completo). |
| **Level** | 3 |
| **Outcome** | success (spatial) |
| **Confidence** | 0.83 (spatial domain) |
| **Tags** | #mining #spatial #slam #nerf #reconstruction #3d |
| **Related** | .cosca/fallback/knowledge/patterns/spatial-ai-layer-patterns.md |
| **Learned** | 1) **Pipeline espacial do agente mapeado**: câmera → ORB-SLAM3/MASt3R-SLAM (localização) → Instant-NGP/Meshroom (reconstrução 3D) → Spatial Reasoning VLM (relações). 2) **Licenças**: ORB-SLAM3 (GPL ⚠️), NICE-SLAM/Spatial Reasoning (Apache ✅), NeRF (MIT ✅), Instant-NGP/Meshroom (Custom ⚠️), MASt3R-SLAM (verificar). 3) **Prioridade**: ORB-SLAM3 primeiro (GPS do agente, CPU, funciona sem GPU) → MASt3R-SLAM (se estável) → Instant-NGP (scanner 3D) → Spatial Reasoning → NICE-SLAM → Meshroom. 4) **Cada projeto = uma capacidade**: ORB-SLAM3="GPS", Instant-NGP="scanner", Meshroom="fotogrametrista", Spatial Reasoning="inteligência espacial". 5) **O agente com Spatial AI sabe onde está e como é o mundo ao redor** — é a camada que conecta visão (o que vê) a ação (onde ir, o que fazer). |
| **Next** | Continuar com as 3 camadas restantes: VFX (#7), Audio espacial (#8), Destruction/Simulation. Depois, quando o NVMe chegar, instalar ORB-SLAM3+Instant-NGP como primeiro módulo espacial. |

---

## Session: 2026-08-23 — Camada Vision (Gap #1 do mining map)

### 2026-08-23 — Vision Layer — o agente enxerga o mundo
| Field | Value |
|-------|-------|
| **Agent** | cosca-kernel |
| **Task** | Gap #1 do mining-map-world-vivo.md: minar a camada Vision (SAM/CLIP/GroundingDINO/etc) seguindo o template do professor (REPO→UTILITY). |
| **Technique** | Level 3 — Mining por camada. Busquei GitHub (stars/license) + produção de `vision-layer-patterns.md` (6 projetos, pipeline visual, template completo). |
| **Level** | 3 |
| **Outcome** | success (vision) |
| **Confidence** | 0.85 (vision domain) |
| **Tags** | #mining #vision #sam2 #clip #groundingdino #dinov2 #yolo #depth |
| **Related** | .cosca/fallback/knowledge/patterns/vision-layer-patterns.md |
| **Learned** | 1) **Pipeline visual do agente mapeado**: frame → YOLO (rápido/alertas) → GroundingDINO (busca linguagem) → SAM2 (segmentação) → CLIP (classificação) → Depth Anything (profundidade) → DINOv2 (features/memória). 2) **Licenças favoráveis**: CLIP (MIT ✅), SAM2/GroundingDINO/DINOv2/DepthAnything (Apache ✅), YOLO (AGPL ⚠️ — licença paga pra comercial). 3) **Prioridade de instalação**: CLIP primeiro (MIT, leve, versátil) → SAM2 → GroundingDINO → Depth Anything → DINOv2 → YOLO. 4) **Cada projeto = uma capacidade do agente**: SAM2="olho que segmenta", CLIP="dicionário visual", GroundingDINO="ponteiro linguístico", DepthAnything="senso de profundidade", DINOv2="cérebro visual", YOLO="detector rápido". 5) **O agente com visão é 10x mais poderoso**: sem ela, ele é cego no mundo; com ela, enxerga, entende, e age com base no que vê. |
| **Next** | Continuar com as outras 4 camadas gap: Spatial AI (#6), VFX (#7), Audio espacial (#8), Multi-agent (#3). Depois, quando o NVMe chegar, instalar CLIP+SAM2 como primeiro módulo de visão. |

---

## Session: 2026-08-23 — Mapa de Mineração do Mundo Vivo (orientação do professor)

### 2026-08-23 — Mapa de Mineração (professor) — 10 camadas + template REPO→UTILITY
| Field | Value |
|-------|-------|
| **Agent** | cosca-kernel |
| **Task** | O Don compartilhou orientação do professor sobre o que estudar/minerar. Professor propôs 10 camadas tecnológicas + template REPO→UTILITY por projeto + prioridade em "projetos que resolveram IA→operacional". Executar: mapear as 10 camadas, cruzar com o que já temos, identificar gaps, produzir mapa operacional. |
| **Technique** | Level 3 — Strategy mining. Produzi `mining-map-world-vivo.md` (10 camadas, template, gaps, ordem de mineração). |
| **Level** | 3 |
| **Outcome** | success (strategy) |
| **Confidence** | 0.82 (mining strategy domain) |
| **Tags** | #mining #strategy #professor #10layers #operational-tools #gaps |
| **Related** | .cosca/fallback/knowledge/patterns/mining-map-world-vivo.md |
| **Learned** | 1) **Tese do professor é CORRETA e alinhada ao objetivo:** "Quais capacidades dar ao agente pra construir, perceber, modificar o mundo?" — o Cosca já é o cérebro; faltam ferramentas OPERACIONAIS (não "qual a melhor IA"). 2) **10 camadas** mapeadas: Procedural World (1), Character/3D (2), Game AI (3), Agent Memory (4), Vision (5), Spatial AI (6), VFX (7), Audio (8), Destruction (9), Simulation (10). 3) **Gaps críticos identificados** (zero coberto no caderno): Vision (SAM/CLIP), Spatial AI (scene graphs neurais), VFX (Taichi), áudio espacial, multi-agent com memória/relações (Microverse). 4) **Template REPO→UTILITY** é o framework certo de enriquecimento (REPO→PAPER→MODEL→LICENSE→DEPS→BENCHMARK→INTEGRATION→PLUGIN→UTILITY). 5) **Prioridade do professor bate:** os 10 top "operacionais" (ComfyUI ✓, AutoGPT, MemGPT, LangGraph, Gaussian Splatting ✓, TripoSR, SAM2, Taichi, AudioCraft ✓, Qwen3-TTS) — deles, 4 já mineramos (ComfyUI, Gaussian, AudioCraft, PCG). 6) **Ordem de mineração recomendada:** Vision→Spatial AI→VFX→Audio espacial→Multi-agent→Destruição→Simulação→City/Terrain. 7) **Fraco no GitHub:** queries específicas (game destruction, AI director, NPC memory, text-to-3D-asset) retornam resultados fracos — os projetos de referência são conhecidos por outra via (comunidade, papers, não busca exata). |
| **Next** | Quando o NVMe de 2TB chegar, executar a mineração por camada na ordem acima, aplicando o template REPO→UTILITY a cada âncora. Começar com Vision (SAM/CLIP) — é o que mais destrava o agente "enxergar" o mundo. |

---

## Session: 2026-08-23 — Mineração Unreal Engine (o runtime do "viver no jogo")

### 2026-08-23 — Mineração Unreal Engine (arquitetura) — o agente vivo no mundo
| Field | Value |
|-------|-------|
| **Agent** | cosca-kernel |
| **Task** | O Don revelou: o runtime do "viver no jogo" é o **Unreal** (ele já domina criar lá). Minar a arquitetura do UE5 focada em: criar o mundo, o runtime do mundo, onde o agente vive e como age + a ponte Cosca↔Unreal. |
| **Technique** | Level 3 — Mining de arquitetura. **Nota honesta:** `EpicGames/UnrealEngine` NÃO é clonável anonimamente (exige EULA/conta) e é dezenas de GB. Minei por conhecimento do framework UE5 (sem clonar), focada no objetivo. Agregação em `unreal-integration-patterns.md` (22 padrões) + INDEX (38 total) |
| **Level** | 3 |
| **Outcome** | success (arquitetura) |
| **Confidence** | 0.78 (unreal/integration domain) |
| **Tags** | #mining #unreal #ue5 #gameworld #agent-life #patterns #integration |
| **Related** | .cosca/fallback/knowledge/patterns/unreal-integration-patterns.md |
| **Learned** | 1) O runtime do "viver no jogo" é o **UE5** (o Don domina). 2) **Arquitetura do UE pro agente vivo** mapeada em 3 camadas: (a) **criar o mundo** → **PCG** (grafo espacial data-driven = o "Sceelix do UE", com `PCGData`/`PCGPoint` como átomo de mundo + seed determinística — casa com o padrão Sceelix que já mineramos); (b) **runtime do mundo** → **World Partition** (células/streaming = mundo infinito; o agente só age no que está carregado), Gameplay Framework (Pawn+Controller); (c) **o agente vivo** → o agente = **`ACoscaAgentPawn`** (corpo) dirigido pela **mente Cosca** (cérebro), com **Behavior Tree + Blackboard** (decisão/memória curta), **GAS** (abilities/attributes/effects = ações/estado), **Perception** (os sentidos). 3) **A ponte = o coração do objetivo**: loop **perceber (Unreal) → decidir (Cosca) → agir (GAS/move) → estado (volta pro Cosca como memória)** via **WebSocket/HTTP** (reusa a porta 14120 do Cosca, que já é REST/WS). O Cérebro (Cosca) é separado do Corpo (Pawn). 4) **UInterface** (`CoscaAgentInterface`) é o contrato que o Don liga no Blueprint/C++ — o "agent loop" do Cosca aplicado ao mundo. 5) O Cosca está bem posicionado: já tem memória em camadas (Mega Brain), procgen, services (porta 14120). |
| **Next** | A ponte Cosca↔Unreal é a feature-chave do objetivo. Recomendo: (1) minar vídeo generativo (completa a Fase 2), (2) depois focar na **ponte** (UInterface + WebSocket bridge pro `ACoscaAgentPawn`). O agente viverá como um pawn no mundo do UE, com o Cosca como cérebro. |

---


