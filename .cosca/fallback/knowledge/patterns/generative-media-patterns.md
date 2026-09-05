# Generative Media Patterns — PCG/Mundo, 3D, Imagem (Difusão+LoRA), Áudio (Criar o Mundo)

> **Version**: 1.0.0 | **Confidence**: 0.90 | **Category**: AI Agent Patterns | **Created**: 2026-08-23 | **Source**: sceelix/Sceelix, graphdeco-inria/gaussian-splatting, comfyanonymous/ComfyUI, facebookresearch/audiocraft

> **Mined by**: cosca-kernel (ordem do Don). Objetivo do Don: **o Cosca aprender a CRIAR imagem/vídeo/áudio e depois 3D + criação de jogos — até o agente TER UMA VIDA DENTRO DO MUNDO.** Extraído de 4 batedores paralelos sobre os âncoras abertos. O Cosca já tem a fundação (`internal/vision`, `media`, `render`, `scene-graph`, `procgen`, pipeline **LoRA**). Este doc mapeia a stack generativa para essas capacidades.

## Purpose

Sintetizar os padrões de **geração/construção procedural de mundos** (PCG), **representação/render 3D** (Gaussian Splatting), **pipeline de imagem por difusão + LoRA** (ComfyUI) e **geração de áudio/música** (AudioCraft) — as 4 camadas que o Cosca precisa para **criar e habitar um mundo de jogo**. Cada uma com o mapeamento pro Cosca.

---

## A. PCG — construção procedural de mundos (Sceelix) — a chave pro "viver no jogo"

### A1. Dataflow Graph Engine (o "construtor de mundo")
- **O que resolve**: um conjunto de nós conectados vira uma receita executável que transforma dados em conteúdo 3D.
- **Como funciona**: `Graph` é lista de `Node`+`Edge`; `GraphProcedure` decompõe em `ExecutionNode`, calcula **ordem topológica** com detecção de ciclo (`FindNodeCycles`), e na execução consome fila de nós, dequeue por `ExecutionIndex`, roda `Procedure.Execute()`, transfere saídas pelos canais, re-enfileira nós prontos. Canais: `DataChannel`/`MultiDataChannel`/`OutputDataChannel`.
- **Onde**: `Source/Sceelix.Core/Graphs/{Graph,GraphProcedure}.cs`, `Execution/{ExecutionNode,DataChannel,MultiDataChannel}.cs`.
- **Aplicação no Cosca**: esqueleto de um **criador de mundo procedural em runtime** — cada nó é uma operação (gerar terreno, scatter props, regra). O Cosca compõe mundos como grafo e re-executa parcialmente ao mudar parâmetros.

### A2. Schema Reflexivo via Atributos (procedures/entidades plugáveis)
- **O que resolve**: estender o gerador com novos tipos de bloco sem tocar o motor.
- **Como funciona**: `[Procedure("guid",...)]` marca classe; `[Entity("Surface")]` marca tipos. No boot, `SystemProcedureManager.Initialize` varre `SceelixDomain.Types`, monta `Guid→Type` + creator compilado. Parâmetros introspetados viram portas/UI.
- **Onde**: `SystemProcedureManager.cs`, `Annotations/{Procedure,Entity,EntityProperty,SubEntity}Attribute.cs`.
- **Aplicação no Cosca**: registrar "geradores de mundo" (bioma, river, city-block) como módulos declarados que aparecem num catálogo, cada um um `Procedure` com parâmetros tipados. Gameplay/PCG como plugins de dados.

### A3. Sistema de Atributos + Gramática de Expressões (dados que guiam regras)
- **O que resolve**: regras de geração declarativas que reagem ao estado de cada entidade.
- **Como funciona**: cada `Entity` carrega `AttributeCollection`; parâmetros avaliados por `ExpressionParser` (gramática ANTLR) reconhecendo `@attr`, `@@attr`. `AttributeCollection` sobrevive ao fluxo de dados (`ForwardImpulseData` complementa atributos).
- **Onde**: `ExpressionParsing/{ExpressionParser,SceelixGrammar.g}`, `Data/Entity.cs`, `Attributes/AttributeCollection.cs`.
- **Aplicação no Cosca**: o agente/vida decide com base em atributos do mundo (`@altitude @slope`) — o "interior" das regras, transformar regras em mundo. Runtime consulta estado para posicionar gameplay.

### A4. Shape Grammar baseada em BoxScope (composição espacial estilo CityEngine)
- **O que resolve**: gerar estrutura (fachadas, blocos, edifícios) dividindo/transformando um "escopo" recursivamente.
- **Como funciona**: `BoxScope` é transformação não-hierárquica (matriz 3 eixos + translation + sizes). `ActorScopeProcedure` aplica Reset/Rotate/Orient; `ActorInsertProcedure` coloca sub-atores. `MeshDivideProcedure` particiona faces por atributo/direção/adjacência e cria sub-meshes.
- **Onde**: `Mathematics/Data/BoxScope.cs`, `Actors/Procedures/*`, `Meshes/Procedures/MeshDivideProcedure.cs`.
- **Aplicação no Cosca**: o **scene-graph** exportável — expor mundo como árvore de `BoxScope` e gerar estruturas recursivamente (quarteirões→lotes→fachadas). Essencial pra "vida do agente" ter geometria navegável.

### A5. Terreno/Composição em Camadas + Ruído Fractal (Perlin multi-oitava)
- **O que resolve**: gerar terreno coerente e re-funcional; escalar para múltiplos atributos.
- **Como funciona**: `SurfaceEntity` é grelha uniforme que guarda `SurfaceLayer` (canais independentes: Height/Float/Blend/Hole/Normal). `AddLayer`/`Merge`/`InsertInto(BoxScope)`. `PerlinSurfaceParameter` cria `PerlinNoise2D(seed, octaves, persistence, frequency, amplitude)` preenchendo altura por célula (paralelo).
- **Onde**: `Surfaces/Data/{SurfaceEntity,SurfaceLayer,HeightLayer,FloatLayer}.cs`, `Surfaces/Procedures/Create/{SurfaceCreateProcedure,PerlinSurfaceParameter}.cs`, `Mathematics/Noise/PerlinNoise2D.cs`.
- **Aplicação no Cosca**: **mundo procedural contínuo** — geometria + camadas separadas permite adicionar camadas em runtime (umidade/heat) sem recriar mesh. Ruído multi-oitava com seed = relevo infinito reproduzível.

### A6. Determinismo por Seed + Cache por Assinatura
- **O que resolve**: mesma receita → mesmo mundo; reutilizar resultados caros.
- **Como funciona**: `RandomProcedure` cria `new Random(_seed)`; `PerlinNoise2D` grava seed. Cache: `MemoryCacheManager` gera chave `ProcedureGuid + Json(parâmetros)`, invalida se fonte mudou, só para nós sem input. `DeepClone` garante isolamento.
- **Onde**: `Procedures/{RandomProcedure,StochasticProcedure}.cs`, `Graphs/Functions/Rand.cs`, `Caching/MemoryCacheManager.cs`.
- **Aplicação no Cosca**: **seed de mundo** reproduzível — salvar a seed e reconstruir sempre igual; cachear mundos grandes (não recomputar ao entrar no runtime). Ponte determinismo↔performance.

### A7. Componentes Reutilizáveis e Serialização do Grafo (pipeline ideia→asset)
- **O que resolve**: empacotar receita como asset reutilizável dentro de outra.
- **Como funciona**: subgrafo embutido via `ComponentNode` (guarda `ProjectRelativePath`); `GraphProcedure.FromPath/FromXML` desserializa. `IndependentGraphProcedure` roda sem cache com seed fixo. Serialização `GraphSave/GraphLoad` (XML).
- **Onde**: `Graphs/ComponentNode.cs`, `Procedures/{GraphProcedure,IndependentGraphProcedure}.cs`, `Graphs/GraphSave.cs`.
- **Aplicação no Cosca**: biblioteca de mundos-componentes que se compõem + formato serializável pra persistir/versionar mundos — o **pipeline ideia→asset 3D**.

---

## B. 3D — representação/render de cena (Gaussian Splatting)

### B1. Representação por Gaussianas 3D (decomposição da covariância)
- **O que resolve**: representar cena 3D de forma comprimida/composável sem malha nem voxel.
- **Como funciona**: cada elemento tem posição `_xyz`, escala `_scaling` (log p/ positividade), rotação `_rotation` (quatérnio), `cov = R·S·Sᵀ·Rᵀ`, opacity, cor via coeficientes SH. Muitas gaussianas + alpha-blending = superfície contínua.
- **Onde**: `scene/gaussian_model.py:33-59`.
- **Aplicação no Cosca**: formato de disco/cena — o gerador instancia gaussianas como "átomos" de geometria+aparência, persistidas em `.ply` (asset 3D denso, teleportável, sem topologia).

### B2. Parâmetros "unconstrained" + ativação inversa
- **O que resolve**: otimizar valores de domínio restrito (positivo/unitário) sem violar física no gradiente.
- **Como funciona**: guarda tensor cru, aplica ativação na leitura (`opacity=sigmoid(_opacity)`, `scale=exp(_scaling)`, `rotation=normalize`); inversas na seed.
- **Onde**: `scene/gaussian_model.py:32-47,102-130`.
- **Aplicação no Cosca**: padrão p/ qualquer parâmetro físico/estético de objetos 3D (reflectância, escala, transparência) — manter log/sigmoid-space, materializar só na render.

### B3. Eséricos Harmônicos (SH) — aparência view-dependent
- **O que resolve**: cor que muda com ângulo de câmera (specular/reflexo) sem texturas por vista.
- **Como funciona**: `f_dc` = cor base; `f_rest` = harmônicos; avaliados via `eval_sh` (Legendre) com grau ativo crescendo durante treino.
- **Onde**: `utils/sh_utils.py:57-117`, `gaussian_renderer/__init__.py:75-110`.
- **Aplicação no Cosca**: aparência realista/reluzente a materiais em mundos 3D, custo mínimo por gaussiana.

### B4. Pipeline de aquisição: SfM (COLMAP) → semeadura de gaussianas
- **O que resolve**: fotos de cena real → cena 3D inicializável.
- **Como funciona**: `convert.py` roda feature+matching+bundle adjustment (posições de câmera + nuvem esparsa); cada ponto SfM vira posição de gaussiana, raio = distância ao vizinho, cor = RGB2SH. Sem SfM → seed 100k pontos aleatórios.
- **Onde**: `convert.py`, `scene/dataset_readers.py:145-226`, `scene/gaussian_model.py:149-176`.
- **Aplicação no Cosca**: o "capturador de mundo" — reconstruir espaços reais a partir de fotos. SfM (não pontos aleatórios) dá convergência.

### B5. Densificação adaptativa (clone / split / prune)
- **O que resolve**: cena decide onde precisa de mais/menos gaussianas.
- **Como funciona**: acumula gradiente 2D por gaussiana; a cada `densification_interval`, gaussianas gradiente-alto e pequenas → **clonadas**; gradiente-alto e grandes → **splits**; depois **prune** de opacidade baixa/área enorme/escala gigante.
- **Onde**: `scene/gaussian_model.py:409-473`, `train.py:164-171`.
- **Aplicação no Cosca**: alocação adaptativa de orçamento de representação — "gastar pontos onde há complexidade".

### B6. Rasterização em tempo real (tile-based splatting + alpha compositing)
- **O que resolve**: desenhar milhões de gaussianas em ≤30fps, diferenciável para backprop.
- **Como funciona**: projeta gaussiana 3D→2D, constrói covariância 2D, kernel CUDA `diff_gaussian_rasterization`: imagem dividida em tiles 16×16, gaussianas associadas, ordenadas por profundidade, alpha-blending por pixel. Frustum culling.
- **Onde**: `gaussian_renderer/__init__.py:18-127`, `scene/cameras.py`.
- **Aplicação no Cosca**: motor de runtime do mundo — render de geometria volumétrica sem mesh, diferenciável (otimizar sobre imagem final).

### B7. Otimização da cena: LR por-propriedade + exposição por-imagem
- **O que resolve**: treinar cena estável, cada gaussiana aprendendo no ritmo certo.
- **Como funciona**: Adam com LRs diferentes por propriedade (posição decai exponencialmente); exposição por imagem é parâmetro aprendível (absorve brilho/clima entre fotos, evita aprender iluminação inconsistente). Loss = L1+SSIM.
- **Onde**: `scene/gaussian_model.py:178-223`, `train.py:91-186`.
- **Aplicação no Cosca**: o "controlador de aprendizado" do mundo — ensinar/adaptar cena iterativamente com retroalimentação de imagem.

---

## C. Imagem — pipeline de difusão + LoRA (ComfyUI)

### C1. Node Graph como Grafo Funcional Tipado (workflow declarativo)
- **O que resolve**: reduzir pipeline de geração (encoder→modelo→sampler→VAE→save) a grafo de valor acyclic (DAG) em JSON puro.
- **Como funciona**: cada nó tem `INPUT_TYPES` (tipado: MODEL/CLIP/CONDITIONING/LATENT/IMAGE/Vae/INT/FLOAT/Combo)+`RETURN_TYPES`+`FUNCTION`. Prompt é `{"3":{"class_type":"KSampler","inputs":{...}}}`. `NODE_CLASS_MAPPINGS` mapeia class_type→classe. Nova API `define_schema()` + `check_lazy_status`.
- **Onde**: `nodes.py`, `custom_nodes/example_node.py.example`.
- **Aplicação no Cosca**: pipeline de criação de imagem como **grafo serializável JSON**, nunca funções acopladas. Versionamento/reuso/reexecutar só o que mudou.

### C2. Modelo de Execução: ordem topológica assíncrona + cache de subexpressão
- **O que resolve**: executar grafo com concorrência, reusar nós computados, execução parcial.
- **Como funciona**: `TopologicalSort` destrava nós quando zero dependências; `ExecutionList` dá execução parcial. Cache `HierarchicalCache`/`LRUCache` chaveado por **assinatura de inputs** — se subgrafo não mudou, reusa. `IsChangedCache` decide reexecução.
- **Onde**: `execution.py`, `comfy_execution/{graph,caching}.py`.
- **Aplicação no Cosca**: separar **grafo** (dados) de **motor** (DAG+cache). Re-gerar com seed diferente não re-roda CLIP/reload de checkpoint. Execução parcial reusa o resto.

### C3. ModelPatcher: modelos como "pesos + camada de patches"
- **O que resolve**: aplicar múltiplas modificações (LoRA, ControlNet, hooks) sem alterar checkpoint em disco.
- **Como funciona**: `ModelPatcher` envolve `BaseModel`, mantém `patches[key]`; `add_patches` injeta, `get_key_patches` materializa o tensor final (aplicando função LoRA). Ao aplicar LoRA, `model.clone()` + patches ao clone (original intacto).
- **Onde**: `comfy/model_patcher.py`, `comfy/model_management.py`.
- **Aplicação no Cosca**: tratar modelo como patcher com pilha de adaptações; servir muitos estilos com um modelo em RAM, trocar estilo sem reload.

### C4. LoRA como injeção de pesos (load → convert → patch → clone)
- **O que resolve**: ajustar estilo/domínio sem refinar o modelo inteiro; aplicar em UNet+CLIP com forças independentes (podendo ser negativas).
- **Como funciona**: `load_lora_for_models` mapeia chaves checkpoint→LoRA, converte, `model.clone()` + `add_patches(strength_model)` e `clip.clone() + add_patches(strength_clip)`. `LoraLoader` encadeia vários LoRAs em sequência.
- **Onde**: `comfy/lora.py`, `comfy/sd.py:102`, `nodes.py:709`.
- **Aplicação no Cosca**: o pipeline LoRA de treino já existente produz artefatos; o caminho é **carregar como patch declarativo**. "Estilo agente" = conjunto ordenado de LoRAs + weights, empilhável.

### C5. Condicionamento: positive/negative, CFG, área/máscara e controle por timestep
- **O que resolve**: dirigir geração — o que quero/o que não quero, CLIP, máscara, sinal estrutural (ControlNet).
- **Como funciona**: `CLIPTextEncode` produz `CONDITIONING` (listas com `area`,`strength`,`mask`,`timestep`). CFG extrapola cond entre positive/negative (default 8). `ControlBase` injeta sinais (pose/edge/depth). Inpainting via `VAEEncodeForInpaint`+`denoise_mask`.
- **Onde**: `comfy/conds.py`, `comfy/controlnet.py`, `comfy/samplers.py`.
- **Aplicação no Cosca**: prompt em 2 canais (incluir/excluir) + sinal estrutural opcional (máscara/referência) como condicionamento, combina com patch de inpainting do LoRA.

### C6. Sampler/denoise: o loop de ruído (scheduler de sigmas + k-diffusion)
- **O que resolve**: transformar latent ruidoso em imagem; `denoise` faz img2img conservando estrutura.
- **Como funciona**: `KSampler` pré-computa noise schedule `sigmas` via `model_sampling`; se `denoise<1.0`, recria steps e **recorta a cauda** `sigmas[-(steps+1):]` (controla quantos passos de ruído remove). `sample()` entrega noise determinístico. Loop em `k_diffusion`, `model_sampling` define parametrização EPS/V_PREDICTION.
- **Onde**: `comfy/samplers.py`, `comfy/model_sampling.py`, `comfy/k_diffusion/sampling.py`.
- **Aplicação no Cosca**: sampler/scheduler/denoise/cfg como parâmetros de qualidade vs custo; `denoise` = img2img natural (imagem base + denoise baixo).

### C7. API/Backend de invocação: prompt→queue→job→assets (assíncrono+streaming)
- **O que resolve**: disparar geração por HTTP sem websocket da UI.
- **Como funciona**: `POST /prompt` valida, enfileira `{prompt_id, number}`, retorna; daemon grava em `/history/{id}`, outputs em `/view` (imagem por hash), `/api/jobs/{id}/cancel`/`/interrupt` abortam. `client_id` → websocket `/ws` (progresso ao vivo). API de assets versionada (hash idempotente).
- **Onde**: `server.py` (/prompt:1072, /history, /view, /ws:269, /interrupt), `app/assets/api/routes.py`.
- **Aplicação no Cosca**: `POST /image/generate` assíncrono (resolver grafo → enfileirar job → retornar `job_id` → `GET /image/{id}`). Assets por hash idempotente (dedup uploads).

---

## D. Áudio/Música (AudioCraft)

### D1. Decodificação em Duas Etapas: Codec Discreto → Transformer Autoregressivo
- **O que resolve**: áudio contínuo de alta dimensão não é capturável por LM diretamente.
- **Como funciona**: `CompressionModel` (Encodec) → encoder SEANet → `ResidualVectorQuantizer` (K codebooks de cardinal 1024 → tokens `[B,K,T]`); decoder reproduz. `frame_rate = sample_rate//hop_length` (ex. 50 frames/s). `LMModel` é transformer sobre tokens.
- **Onde**: `audiocraft/models/encodec.py`, `quantization/vq.py`, `modules/seanet.py`.
- **Aplicação no Cosca**: media/voice/audio espelha isso — um **codec** (comprimir áudio em tokens) + modelo de linguagem sobre tokens (reusa infra de transformers do Cosca). `frame_rate` = unidade de tempo do jogo (token ≈ tick).

### D2. Interleaving de Codebooks (padrões de sequência multi-stream)
- **O que resolve**: tokenizador gera K codebooks paralelos por frame — transformer só prevê um por passo.
- **Como funciona**: `CodebooksPatternProvider` gera layout; `DelayedPatternProvider` atrasa cada codebook (dependência causal sobre os anteriores).
- **Onde**: `modules/codebooks_patterns.py` (DelayedPatternProvider:305).
- **Aplicação no Cosca**: várias trilhas simultâneas (tema+ambiência+SFX) com streams paralelos e dependência temporal controlada (atraso define quão acopladas são as pistas).

### D3. Fuso de Condicionamento (sum/prepend/cross) → controle por texto, melodia e estilo
- **O que resolve**: injetar "trilha tensa de battle", "melodia de bach", "estilo anos 80".
- **Como funciona**: `ConditioningAttributes` agrupa por tipo (text/wav/joint_embed/symbolic); `ConditionFuser` decide modo de injeção: `sum`/`prepend` (melodia/chroma)/`cross` (texto T5/CLAP). `ChromaStemConditioner` (via demucs) para melodia.
- **Onde**: `modules/conditioners.py` (ConditionFuser:1672, T5Conditioner:422, ChromaStemConditioner:571).
- **Aplicação no Cosca**: "painel de condicionamento" declarativo — texto=direção criativa, áudio=reuso de elementos sonoros do mundo, estilo=arena temática.

### D4. Classifier-Free Guidance com Forward Único Empacotado
- **O que resolve**: ajustar quão fortemente a geração segue a condição.
- **Como funciona**: `generate` duplica batch (cond+nulo), faz UM forward (~2× mais rápido), interpola `logits = uncond + cfg_coef*(cond-uncond)`. CFG dupla (`cfg_coef_beta`) equilibra texto vs estilo.
- **Onde**: `models/lm.py:323-418`, `musicgen.py:96-132`.
- **Aplicação no Cosca**: `cfg_coef` como parâmetro de "adherence" — alto=segue direção à risca; baixo=deixa inventar variações.

### D5. Áudio-Loop de Geração Extensa por Janelas Deslizantes + KV-Cache Streaming
- **O que resolve**: gerar áudio mais longo que o max_duration do treino com continuidade.
- **Como funciona**: se `duration>max_duration`, loop gera chunks de `chunk_duration`, mantém últimos `stride_tokens` (default 18s) como prompt do próximo, concatena. `StreamingModule` cacheia estados (KV-cache) via `with self.streaming()`.
- **Onde**: `models/genmodel.py:193-260`, `musicgen.py:251-337`, `modules/streaming.py`.
- **Aplicação no Cosca**: trilhas longas de fase/missão; `StreamingModule` é padrão reutilizável p/ qualquer geração incremental longa (não só áudio).

### D6. Pipeline de Pós-Processamento e Normalização de Áudio
- **O que resolve**: áudio cru → assets utilizáveis (sample rate, canais, loudness, formato).
- **Como funciona**: `convert_audio` (resample/canais), `normalize_audio` (peak/rms/loudness), `compress`/`get_mp3`/`get_aac` (mp3/ogg/flac). `InterleaveStereoCompressionModel` p/ estéreo.
- **Onde**: `data/audio_utils.py`, `models/encodec.py:397-506`.
- **Aplicação no Cosca**: "shipping layer" — converte saída em assets prontos (BGM .ogg alta loudness, SFX .wav mono).

### D7. Abstração `BaseGenModel` + API de Geração (contrato do agente)
- **O que resolve**: agente chamar geração de forma consistente.
- **Como funciona**: `BaseGenModel` define `generate(text[])`, `generate_continuation(prompt_wav)`, `generate_unconditional`, `generate_audio(tokens)`. `get_pretrained(name)` carrega. `return_tokens=True` expõe tokens (para re-gerar com decoder alternativo). `progress_callback`.
- **Onde**: `models/genmodel.py:28-267`, `musicgen.py:40-94`.
- **Aplicação no Cosca**: contrato único — `generate(texto)`, `generate_continuation(áudio)` (loop), `generate_with_chroma(melodia)`. Um endpoint com 3 modos de condicionamento resolve trilha + sons.

---

## Synthesis — o que o Cosca deveria copiar (p/ criar e habitar o mundo)

| # | Padrão | Camada | Aplicação no Cosca |
|---|--------|--------|--------------------|
| 1 | A4 — Shape Grammar + BoxScope | mundo | scene-graph exportável p/ estruturas recursivas |
| 2 | A5/A6 — camadas de terreno + seed/cache | mundo | mundo procedural contínuo determinístico |
| 3 | A1 — Dataflow Graph Engine | mundo | criador de mundo como grafo re-executável |
| 4 | B6 — Gaussian splatting diferenciável | 3D | motor de runtime do mundo (render volumétrico) |
| 5 | B4 — SfM→gaussianas | 3D | capturador de espaços reais p/ o mundo |
| 6 | C3/C4 — ModelPatcher + LoRA como patch | imagem | servir estilos com um modelo; estilo-agente empilhável |
| 7 | C2 — cache por assinatura de inputs | imagem | re-executar só o que mudou; reuso de subgrafo |
| 8 | D1 — codec discreto → LM autoregressivo | áudio | reusa infra de transformers p/ áudio; token = tick |
| 9 | D5 — janelas deslizantes + KV-cache | áudio | geração longa (trilha de missão) com continuidade |

## Known Uses (referência)

- `sceelix/Sceelix` (PCG 2D/3D), `graphdeco-inria/gaussian-splatting` (3D), `comfyanonymous/ComfyUI` (image difusão+LoRA), `facebookresearch/audiocraft` (áudio/música).

## Related Patterns

- [`ai-products-patterns.md`](ai-products-patterns.md) — os produtos (Midjourney/Runway/Suno...) que o Don listou
- [`mega-brain-patterns.md`](mega-brain-patterns.md) — criatividade/síntese + media
- [`kubernetes-org-patterns.md`](kubernetes-org-patterns.md) — gap de execução/sandbox (relevante p/ rodar esses pipelines)
- docs/architecture/{scene-graph,procgen}.md — a base do Cosca que isso estende
