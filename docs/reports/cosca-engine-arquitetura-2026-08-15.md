# COSCA ENGINE — Arquitetura de Referência (Mineração Profunda)

> **Ordem do Don**: minerar profundamente os repositórios de referência do COSCA Creative/Scientific/Media Ecosystem e projetar a arquitetura própria.
> **Data**: 2026-08-15
> **Método**: 12 frentes paralelas (um capo por categoria), ~71 repositórios minerados no código-fonte (não só README), destilados em Research Matrix + síntese.
> **Artefatos de frente**: `/tmp/opencode/cosca-mining/reports/{categoria}.md` (12 relatórios, ~2.600 linhas).

---

## 1. Resultado das 12 frentes (1 linha por categoria)

| Categoria | Repos | Núcleo extraído |
|---|---|---|
| Architecture | 4 | Ports & Adapters, modular monolith, DAG + cache, content-addressable |
| AI | 5 | Dispatch por capabilities, KV-cache, quantização, scheduler dissociado do modelo |
| Image | 6 | Node graph (ComfyUI), lazy demand-driven (libvips), cache por hash, tile-and-merge |
| Video | 7 | Grafo de filtros negociado, timeline em camadas (MLT), pipeline codec + hwaccel |
| Audio | 9 | Grafo de nós (JUCE), realtime callback + worker pool (Ardour), IA offline (demucs/whisper) |
| 3D/Graphics | 8 | Descrição vs avaliação (USD/depsgraph), GPU em 3 camadas, materiais compilados |
| Game | 4 | ECS data-oriented (Bevy), scene-tree + Server/RID (Godot), immediate-mode UI |
| Scientific | 7 | Array tipado universal, lazy IR (polars/jax), grafo serializável para reprodutibilidade |
| Documents | 6 | Documento em 3 camadas, pipeline OCR por estágios, input hostil |
| Workflow | 3 | Event sourcing + replay (Temporal), DAG + artifacts (Argo), lineage (Dagster) |
| DevTools | 5 | Rope/B-tree document model (Zed), parsing incremental (tree-sitter), extension sandbox |
| Security | 6 | Sandbox multi-camada, gates pré-execução, política declarativa + audit imutável |

---

## 2. Os 12 Princípios Universais (convergem em 3+ categorias)

> Estes são os padrões que aparecem em múltiplos domínios independentes — sinal de padrão maduro. São a fundação do COSCA ENGINE.

### P1 — Node Graph universal
Tudo é um nó com `INPUTS`/`OUTPUTS`/`CONFIG`/`EXECUTION`/`CACHE` tipados. A timeline é um nó com time-mapping (`InputTimeAdjustment`). O mixer é um nó somador. O efeito é um nó filtro. O pipeline IA é um nó.
*Fontes: ComfyUI, GStreamer, MLT, JUCE/AudioProcessorGraph, Blender, Godot, Temporal/Dagster.*

### P2 — Cache por conteúdo (hash), não por posição
Identifique computação pelo **conteúdo** (hash de inputs + modelo/parâmetros), nunca pela posição no grafo. Invalidação upstream. Camadas LRU (RAM) → spill a disco → eviction por pressão.
*Fontes: libvips (op-hash), ComfyUI (input-signature), Olive (InvalidateCache), Dagster (memoize), Argo (Memoize), Git (CAS + CoW).*

### P3 — Abstração por capabilities, não por vendor
O provider declara o que faz; a engine seleciona o melhor backend. Sem `if vendor` no núcleo.
*Fontes: PyTorch Dispatcher/DispatchKey, vLLM Platform + validate_configuration, llama.cpp ggml_backend registry, OpenCV BACKEND×TARGET, wgpu HAL, SDL GPU.*

### P4 — Avaliação lazy / demand-driven
Processa sob demanda, puxa em vez de empurra. Regiões/tiles em vez do arquivo inteiro.
*Fontes: libvips (demand hints/regions), ComfyUI (check_lazy_status), polars (lazy IR), jax (tracing), MLT (get_image pull-stack).*

### P5 — Separar descrição de avaliação
O **"quê"** (grafo/descrição imutável, versionável) separado do **"como"** (engine/avaliação). Permite reexecutar, otimizar, cachear, versionar.
*Fontes: USD (layers vs depsgraph), MLT (metadata-driven), Temporal (event-sourcing vs replay), scientific (grafo vs motor).*

### P6 — Modelo de dados tipado e canônico por domínio
Um modelo canônico, N views. O array é a interface (`dtype+shape+strides`), não a implementação — troque backend por trás.
*Fontes: numpy ndarray, Arrow (polars), Rope/B-tree monoidal (Zed), VipsImage (libvips), SoA archetypes (Bevy).*

### P7 — ECS / composição por componentes
Entidade = ID; comportamento = sistemas que transformam componentes. Scene-tree por cima para ergonomia.
*Fontes: Bevy (archetypes+SoA, ticks, Commands deferred), Godot (Node + Server/RID), Dagster (assets).*

### P8 — Dualidade realtime vs offline
**Realtime**: callback na thread de áudio, blocos fixos, zero alocação, ring buffer. **Offline**: chunks + overlap-add, streaming. Mesmo grafo, dois modos de execução.
*Fontes: Ardour/portaudio/OBS (3 threads) vs demucs/whisper (offline).*

### P9 — Durabilidade via event sourcing + replay
Histórico append-only como fonte única de verdade; estado mutável em cache. Retry/timeout/cache/idempotência como **atributos do nó**.
*Fontes: Temporal (HistoryEvent), Dagster (DataProvenance), Argo (RetryStrategy).*

### P10 — Sandbox + least-privilege por padrão
Todo input externo é hostil. Execução de plugin/script em sandbox (WASM/WASI ou subprocess) com capabilities declaradas no manifest.
*Fontes: Zed (WASM/WASI), Docker Security, ASVS V15, OWASP.*

### P11 — Política declarativa separada do enforcement
Políticas versionáveis (Rego/YAML) aplicadas por gates; audit log imutável, fail-secure (nunca fail-open).
*Fontes: semgrep (patterns), trivy (Rego misconfig), scorecard (checks), ASVS.*

### P12 — Proveniência e reprodutibilidade
Toda saída registra como foi gerada: model/prompt/seed/parâmetros + code_version + data_version. Permite reproduzir, auditar e evoluir.
*Fontes: Dagster (code_version+data_version), creative integrity, scorecard/SLSA.*

---

## 3. Mapeamento para os 16 Engines do COSCA

| COSCA Engine | Projeto (baseado nos princípios) |
|---|---|
| **PROJECT** | Manifest declarativo + content-addressable (CoW) → snapshots/undo/redo/branch de graça |
| **ASSET** | CAS (hash=ID) + derivatives/preview + dependências; nunca duplicar |
| **SCENE GRAPH** | Descrição (layers estilo USD) vs avaliação (depsgraph reativo, time-sampled) |
| **DOCUMENT MODEL** | Rope (B-tree monoidal) + snapshot + anchor; parsing incremental (tree-sitter); 3 camadas (raw/geom/semântico) |
| **MEDIA** | Node graph + demand-driven lazy + stream out-of-core (regiões/tiles) |
| **GPU** | 3 camadas (shader IR → HAL → render graph) + registry de capabilities + scheduler |
| **AI** | Dispatch por capabilities + model registry auto-descritivo + scheduler/cache dissociados do modelo |
| **WORKFLOW** | DAG tipado + event sourcing + provenance + retry/cache/idempotência por nó |
| **PLUGIN** | Sandbox WASM/WASI + capabilities no manifest + extension points |
| **RENDER** | Determinístico + cacheável + resumível + paralelo + observável (preview/draft/final) |
| **TASK** | Filas + backpressure + worker pool (OBS 3-thread, Ardour worker pool) |
| **VERSION** | snapshot/diff/rollback/branch/merge sobre CAS |
| **COLLABORATION** | CRDT/OT sobre o document model |
| **KNOWLEDGE** | Padrões aprendidos (taxonomia por domínio), nunca clones |
| **SECURITY** | Sandbox multi-camada + gates pré-execução (SAST+secrets+SCA+provenance) + audit imutável |
| **OBSERVABILITY** | metrics/logs/traces/audit de TASK/MODEL/GPU/CPU/MEMORY/VRAM |

---

## 4. Decisões arquiteturais de fundo

1. **Modular monolith com Ports & Adapters** — não microserviços prematuros. Núcleo com interfaces estáveis; mídia/GPU/IA atrás de adaptadores.
2. **Tudo é nó** — um único runtime de grafo (tipado, lazy, cacheável) serve editor, imagem, vídeo, áudio, 3D, científico e workflow. É o coração.
3. **CAS (content-addressable) para assets e histórico** — o Git como modelo: dedup, versionamento, undo e colaboração emergem do mesmo primitivo.
4. **Rust no núcleo (ou Go), Python/JS na borda** — performance crítica (media/GPU/DSP) em linguagem de sistemas; produtividade de tooling/IA na borda. (Zed/Bevy/Rust vs ComfyUI/Dagster/Python mostram a fronteira.)

---

## 5. O que NÃO fazer (lições dos trade-offs)

- **Não abstrair demais o media engine** — FFmpeg/GStreamer mostram o custo de abstração vazada; use FFmpeg como backend, não o copie.
- **Não assumir que estrelas = referência** — avaliar maturidade, licença (AGPL/GPL contaminam), manutenção e arquitetura real.
- **Não misturar realtime com IA síncrona** — áudio/vídeo realtime exige callback/blocos; IA roda offline/async.
- **Não confiar em input externo** — PDF, mídia, plugin, script, modelo: tudo hostil até prova em contrário.
- **Não renderizar de novo o que não mudou** — cache por conteúdo é lei.

---

## 6. Estratégia de construção (recomendação)

| Fase | Escopo | Base |
|---|---|---|
| **F1** | PROJECT + ASSET (CAS) + NODE GRAPH + CACHE + SECURITY (sandbox) | Git model, ComfyUI, Zed sandbox |
| **F2** | WORKFLOW + TASK + RENDER (determinístico) | Temporal/Dagster, OBS |
| **F3** | MEDIA + GPU + AI ENGINES | libvips/FFmpeg, wgpu/vLLM |
| **F4** | EDITOR | VSCode/Zed (Rope + extension host) |
| **F5+** | IMAGE → CINEMA → MUSIC → 3D → GAME → SCIENTIFIC | sobre o núcleo F1-F4 |

O segredo: **construir F1-F4 uma única vez** e deixar cada produto ser uma *configuração especializada* de nodes, painéis e knowledge — não um codebase novo.

---

*Relatórios de frente completos: `/tmp/opencode/cosca-mining/reports/` (architecture, ai, image, video, audio, 3d, game, scientific, documents, workflow, devtools, security).*
