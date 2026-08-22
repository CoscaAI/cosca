# Research Matrix — Imagem

> Dossiê de pesquisa da categoria IMAGEM para a plataforma multimídia assistida por IA do Cosca.
> Fontes: READMEs e código-fonte verificados via GitHub (16/08/2026). Itens não verificados marcados explicitamente.

---

## Projeto: ComfyUI (comfyanonymous/ComfyUI → Comfy-Org/ComfyUI)

**FOCO ESPECIAL — análise profunda do node graph (o Cosca quer um node graph próprio).**

| FIELD | VALUE |
|---|---|
| PROJECT | ComfyUI |
| CATEGORY | Engine de criação multimídia por IA (node graph / grafo de execução) |
| PURPOSE | Orquestrar modelos generativos (imagem/vídeo/áudio/3D/texto) em grafos visuais encadeáveis, com execução local, cache e API |
| ARCHITECTURE | Backend Python (FastAPI/aiohttp + aiohttp server), execução assíncrona com `asyncio`, grafo serializado como JSON. 3 componentes: Core (este repo), Frontend (Vue/TS, repo separado), Desktop (Eletron). Núcleo: `comfy_execution/` (graph, caching, validation), `nodes.py` (registro de nós), `comfy/model_management.py` (VRAM), `comfy/model_patcher.py` (patches/LoRA). Frontend instalado via wheel (`comfyui-frontend-package`). |
| LANGUAGE | Python (backend/execução), TypeScript/Vue (frontend), CUDA/C (kernels custom) |
| CORE ALGORITHMS | Diffusão (SD1.5, SDXL, SD3.5, Flux, Wan, HunyuanVideo…), atenção/attention (SDPA, flash, xformers, split-cross), VAEs, quantização (fp8/INT), cache de assinatura de grafo, ordenação topológica, LRU/RAM-pressure eviction, offloading assíncrono |
| PIPELINES | `prompt (JSON) → validate_inputs (recursivo, detecta ciclos) → DynamicPrompt → PromptExecutor.execute_async → TopologicalSort/ExecutionList → execução por nó → cache → UI/API`. Subgrafos dinâmicos (nós podem expandir grafos em runtime via `GraphBuilder`). Reuso de subgrafos, templates de workflow, App Mode, API local |
| DATA MODEL | Grafo = dict `{node_id: {class_type, inputs}}`; links = `[source_node_id, source_socket_index]`; constantes = valores diretos. Saídas de nós = `(list)`, model patchers = referências. Workflow inteiro serializável em JSON e embutido em PNG (extra_pnginfo) |
| GPU/CPU/MEMORY | `VRAMState`: DISABLED/NO_VRAM/LOW_VRAM/NORMAL_VRAM/HIGH_VRAM/SHARED(MPS). `current_loaded_models` (weakrefs) + `free_memory()`/`load_models_gpu()`; carregamento parcial de pesos (lowvram), modelos dinâmicos sob demanda, mmap + `DIRTY_MMAPS`, streams de offload assíncrono (2 por default em NVIDIA/AMD), cast buffers reutilizáveis, host buffers pinados, evicção de cache por pressão de RAM (`psutil`), OOM → unload_all_models |
| PERFORMANCE | Só executa nós cujo output é alcançável; re-execução parcial (nós inalterados pulados); fila assíncrona; offload sobreposto ao compute; previews de latent de baixa resolução (TAESD); batch/`INPUT_IS_LIST` paralelizável via asyncio |
| EXTENSIBILITY | `custom_nodes/` com registro via `NODE_CLASS_MAPPINGS`; interface `INPUT_TYPES()/RETURN_TYPES/FUNCTION/OUTPUT_NODE/IS_CHANGED/check_lazy_status`; integra VAEs, text encoders, LoRA, ControlNet, upscalers, adapters; `extra_model_paths.yaml` para modelos externos; ComfyUI-Manager para instalar nós |
| PLUGIN SYSTEM | Forte: custom nodes em Python arbitrário (toda a potência = todo o risco); cache providers plugáveis (persistência externa de cache com `CacheContext`/hash de chave); hook `map_node_over_list` deliberadamente não-editável; partner nodes para modelos fechados |
| SECURITY | Custom nodes = código arbitrário (não verificado / risco inerente); execução offline por padrão (core não baixa nada); `--disable-api-nodes` força offline total; TLS via cert; SECURITY.md presente. Sem sandboxing de nós custom |
| LICENSE | GPL-3.0 (repo core). Implicação: incorporar código exige copyleft — usar como referência de arquitetura, não como lib |
| STRENGTHS | Maturidade alta (5.7k commits, 127k stars), arquitetura de grafo exemplar, cache inteligente por assinatura, gestão de VRAM sofisticada, modelo de plugins que virou padrão da comunidade, API para integração produtiva, runs offline |
| WEAKNESSES | GPL-3.0 limita uso interno; custom nodes sem sandbox (segurança); versão master instável para custom nodes (ciclo 2 semanas); código denso/acoplado a PyTorch; cache por assinatura pode custar caro em grafos grandes; frontend em repo separado com release cíclico |
| TRADE-OFFS | Potência/controle total vs. complexidade e risco de segurança; cache por input-signature vs. custo de hash em grafos grandes; flexibilidade de nós em Python vs. impossibilidade de sandbox |

### Como o DAG funciona (detalhe)
- **Estrutura:** o prompt é um dict `{node_id: {"class_type":..., "inputs": {...}}}`. Cada input é ou um valor constante ou um link `[nó_origem, índice_da_saída]`. O `DynamicPrompt` envolve isso e acrescenta `ephemeral_prompt` para nós criados em runtime (subgrafos). Não há objeto "grafo" mutável no frontend→backend: o JSON É o grafo.
- **Ordenação:** `TopologicalSort` dissolve o DAG mantendo `blockCount` (dependências não-resolvidas) e `blocking` (quem bloqueia quem). `ExecutionList` estende com "dissolução topológica": um nó pode ser *stageado*, devolvido ao grafo (`unstage`) e ter dependências adicionadas depois (ex.: input lazy que só descobre qual upstream precisa após executar `check_lazy_status` → retorna `PENDING` e re-adiciona strong link). Ciclos são detectados por dissolução reversa do grafo.
- **Encadeamento de modelos:** inputs linkados são resolvidos via `get_input_data()`, que lê o output do upstream do cache de execução (`execution_list.get_cache(from_node, to_node)`), "write-back" no cache principal. Modelos de difusão, text encoders e VAEs trafegam como objetos (model patchers) entre nós — não são re-carregados por nó, são referenciados.
- **Execução:** loop `while not execution_list.is_empty()` → `stage_node_execution()` (heurística UX: nós OUTPUT_NODE/async primeiro) → `execute()` por nó: 1) checa cache de outputs; 2) monta inputs; 3) instancia objeto (cacheado por ID); 4) roda `FUNCTION` via `_async_map_node_over_list` (suporta batch, `INPUT_IS_LIST`, e funções corrotinas); 5) escreve `CacheEntry(ui, outputs)` no cache.

### Como cacheia (não re-executa nós inalterados)
- **Chave por assinatura de input:** `CacheKeySetInputSignature` calcula `signature = [class_type, IS_CHANGED, ancestrais ordenados...]` recursivamente: links viram `("ANCESTOR", índice_ancestral, socket)`, constantes entram cruas; tudo convertido em `to_hashable` (mappings→frozenset, sequences→frozenset com índices, valores não-hasheáveis tipo tensores→`NaN` = nunca cacheados). Resultado: se qualquer input transitivo mudar, a chave muda e o nó re-executa; se nada mudar, `caches.outputs.get(node)` retorna hit e o nó é **pulado inteiro** (só o UI cacheado é reenviado).
- **`IS_CHANGED` / `fingerprint_inputs`:** o próprio nó declara o que conta como "mudou" (ex.: mtime de arquivo carregado). Se não tem IS_CHANGED, o nó é considerado estável entre execuções.
- **Cache hierárquico:** `HierarchicalCache` para outputs (chave por assinatura) + subcaches por subgrafo (nós efêmeros ficam em subcache do nó pai). `CacheKeySetID` (chave = node_id + class_type) cacheia os **objetos instanciados**, não os outputs.
- **Variantes:** `NullCache` (desliga), `LRUCache` (max_size por geração, evicção por geração de uso), `RAMPressureCache` (evicção guiada por RAM livre real: pontua por `1.3^(gerações sem uso)` × RAM_usage estimada dos tensores CPU; entradas >512MB libertadas primeiro; é o default quando `--cache-type ram-pressure`).
- **Cache providers:** camada opcional de persistência externa (store/lookup assíncrono por hash de chave serializada) — extensível para cache distribuído.
- **Provas no código:** "if you submit the same graph twice only the first will be executed. If you change the last part of the graph only the part you changed and the part that depends on it will be executed." E `validate_inputs` roda antes; só nós alcançáveis a partir de `execute_outputs` entram na lista.

### Como gerencia modelos e memória (VRAM)
- **Registro de modelos:** `LoadModel`/`LoadedModel` com weakref + finalizer para detectar leaks (`cleanup_models_gc` faz GC completo se um modelo morre ainda referenciado). `load_models_gpu()` dedup, libera memória antes (ordena candidatos a unload por memória offloaded), carrega.
- **Modos VRAM:** HIGH_VRAM mantém tudo na GPU; NORMAL/LOW_VRAM usam carregamento parcial (`model_load(lowvram_model_memory)` — carrega só parte dos pesos e o resto sob demanda); NO_VRAM minimiza ao extremo. Modelos *dinâmicos* (ex.: text encoders que carregam token a token) não desalocam uns aos outros (offloading on-demand).
- **Offload assíncrono:** até 2 streams CUDA por device (default em NVIDIA/AMD) para sobrepor transferência de pesos com compute; cast buffers reutilizáveis para casting de dtype; fp8 compute quando suportado; `soft_empty_cache()` para devolver memória ao allocator.
- **RAM:** headroom configurável (`--cache-type ram-pressure`, `--ram`, `--ram-inactive`); callback de RAM em cada iteração do loop de execução libera cache (intermediários grandes primeiro) e pinos de memória host quando `psutil` indica pressão.
- **OOM:** detecta `torch.cuda.OutOfMemoryError`/`AcceleratorError`, descarrega todos os modelos e orienta o usuário (tips sobre batch_size).

**Princípio para o Cosca:** *Serialize o grafo como dados puros (JSON) e torne a execução derivada e cacheável por assinatura de inputs — só re-execute o que mudou, e só execute o que é alcançável a partir de outputs demandados.* (Signature-based caching + topológico por demanda.)

**Armadilha a evitar:** *Plugins com execução arbitrária sem sandbox.* ComfyUI paga o preço de GPL + superfície de segurança enorme porque custom nodes são Python cru. O Cosca deve separar o núcleo de execução de qualquer código não confiável (sandbox/denylist/execução isolada) desde o dia zero — e evitar adotar código GPL-3.0 no núcleo proprietário.

---

## Projeto: OpenCV (opencv/opencv)

| FIELD | VALUE |
|---|---|
| PROJECT | OpenCV |
| CATEGORY | Biblioteca de visão computacional e processamento de imagem |
| PURPOSE | Algoritmos clássicos de visão (filtros, features, calibração, tracking, DNN inference) como bloco de construção universal |
| ARCHITECTURE | Monolito C++ modular: `core`, `imgproc`, `features2d`, `video`, `calib3d`, `objdetect`, `dnn`, `highgui`, etc. Camada HAL (hardware abstraction) para SIMD/OpenCL/IPP; `opencv_contrib` separado. Build via CMake. Bindings Python (numpy), Java, JS, C# |
| LANGUAGE | C++ (núcleo), C, Python bindings, OpenCL kernels |
| CORE ALGORITHMS | Convolução/filtros, FFT, Canny, Hough, SIFT/ORB/FAST, homografia/calibração, optical flow, histogramas, clustering (K-means), contornos, morfologia, `dnn` (import ONNX, SSD/YOLO), OpenCL acelerado |
| PIPELINES | `Mat (referência-counted header + dados) → pipeline de funções → saída`. `cv::UMap`/`UMat` para dados na GPU via OpenCL transparente. `dnn`: `readNet(ONNX) → setInput → forward`. Filas de vídeo (VideoCapture/VideoWriter) |
| DATA MODEL | `cv::Mat`: header + ponteiro de dados, refcount, ROI/views (submatrizes sem cópia), tipos (`CV_8U/CV_32F`…), canais e passo (`step`). `UMat` (GPU), `GpuMat` |
| GPU/CPU/MEMORY | CPU (SIMD, OpenMP), GPU via OpenCL (padrão) ou CUDA (`opencv_contrib/cuda*`), HAL para IPP/TBB; memória gerenciada por refcount, `UMat` faz upload/download lazy; não gerencia VRAM de DL (deixa para o runtime do modelo) |
| PERFORMANCE | Muito maduro em CPU (decades de otimização); GPU depende do build (OpenCL nem sempre habilitado). `dnn` mais lento que runtimes especializados (TensorRT/ONNX Runtime) |
| EXTENSIBILITY | Módulos compiláveis, `opencv_contrib` com algoritmos experimentais, callbacks em C++/Python; via de extensão mais pesada (recompilar) |
| PLUGIN SYSTEM | Fraco (não é arquitetura de plugins; é biblioteca linkável). Pode-se criar módulos custom compilando, mas não há hot-plug de runtime |
| SECURITY | Imagem de entrada: histórico sólido de CVEs de parsing (PNG/JPEG) — mitigado com fuzzing contínuo e policy de desabilitar codecs; SECURITY.md; não carrega modelos remotos |
| LICENSE | Apache-2.0 (permissiva, ideal para incorporar) |
| STRENGTHS | Maturidade extrema (36k commits, 90k stars, ~20 anos), cobertura total de CV clássica, bindings para quase tudo, Apache-2.0, HAL para performance multi-vendor |
| WEAKNESSES | API com cruft histórico (múltiplas gerações convivendo), breaking changes entre 3.x/4.x/5.x, build pesado, `dnn` não é topo de performance, GPU exige build/config própria |
| TRADE-OFFS | Abrangência + universalidade vs. profundidade/performance em tarefas específicas (para SR/segmentação use modelos dedicados); simplicidade Python vs. performance C++ (bindings numpy) |

**Princípio para o Cosca:** *Adote OpenCV como camada de "CV utilitária" (filtros, máscaras, geometria, parsing de imagem) e não reinvente o básico — mas trate o parsing de arquivos como superfície de risco (fuzz + policy).*

**Armadilha a evitar:** *Tratar `dnn` do OpenCV como engine de inferência principal.* É ok para prototipar, mas perde em desempenho e flexibilidade para runtimes especializados; o Cosca deve separar "processamento clássico" (OpenCV) de "inferência de modelos" (runtime dedicado).

---

## Projeto: SAM 2 (facebookresearch/sam2)

| FIELD | VALUE |
|---|---|
| PROJECT | SAM 2 (Segment Anything Model 2) |
| CATEGORY | Segmentação por prompt (imagem + vídeo) — foundation model |
| PURPOSE | Segmentação promptável: dado ponto/box/máscara, produz máscara do objeto em imagem ou rastreia em vídeo (masklets) |
| ARCHITECTURE | Transformer com memória de streaming (streaming memory: banco de memory-banks de pontos/atenção por frame) para vídeo em tempo real. Backbone hierárquico Hiera. Um modelo trata imagem como vídeo de 1 frame. Pacote pip `SAM-2` com `SAM2ImagePredictor`, `SAM2VideoPredictor`, `AutomaticMaskGenerator`, state de inferência por vídeo |
| LANGUAGE | Python (PyTorch), CUDA (kernel custom p/ connected components em pós-processo) |
| CORE ALGORITHMS | Promptable segmentation (points/box/mask), memory attention (memory bank + occlusion-aware), máscara com memória streaming, `torch.compile` para VOS (major speedup), inferência multi-objeto independente |
| PIPELINES | `set_image → predict(propmpts)` (imagem) / `init_state(video) → add_new_points_or_box → propagate_in_video` (vídeo). Prompt refinável frame a frame; máscaras rastreadas |
| DATA MODEL | Checkpoints `.pt` + configs YAML (hiera tiny/small/base+/large: 38.9M–224.4M params). State de inferência (objetos, memórias, prompts) mantido pelo predictor entre frames |
| GPU/CPU/MEMORY | GPU obrigatório na prática (autocast bf16, `torch.inference_mode`); CUDA kernel custom precisa nvcc; memória cresce com nº de objetos×frames (memory banks); modelo grande exige ~24GB para hiera_l em vídeos longos (não verificado empiricamente) |
| PERFORMANCE | Hiera_l ~39–40 FPS em A100 (imagem/vídeo benchmark oficial); tiny ~91 FPS; `vos_optimized=True` (compile) dá speedup "major" em VOS |
| EXTENSIBILITY | Biblioteca (predictors + build functions), exemplo de frontend/backend de demo, treinamento/fine-tuning liberado, integração via HuggingFace Hub. Não é framework de plugins |
| PLUGIN SYSTEM | Inexistente (é uma lib/modelo). Extensão via código Python ou via integração em plataformas (ex.: nodes de SAM no ComfyUI) |
| SECURITY | Repo de pesquisa; checkpoints são binários confiáveis (hash), sem superfície de parsing de inputs hostis; Apache-2.0 + componente cc_torch BSD-3 |
| LICENSE | Apache-2.0 (modelo + código) / BSD-3-Clause (componente cc_torch) |
| STRENGTHS | Estado da arte em segmentação promptável, unifica imagem+vídeo num único modelo, API simples, streaming memory bem projetada, checkpoints múltiplos (tiny→large) para escalar custo |
| WEAKNESSES | Repo de pesquisa em evolução rápida (checkpoints atados a versão exata do código — "reinstall on pull"), exige GPU, memória crescente em vídeos longos/multi-objeto, sem serviço/API pronta, poucos testes de produção |
| TRADE-OFFS | Qualidade SOTA + simplicidade de API vs. instabilidade de release e custo de GPU; one-model-for-all vs. especialistas (SAM2 vs. detectores dedicados) |

**Princípio para o Cosca:** *Trate modelos como bibliotecas com uma interface mínima e estável ("predictor" com state explícito), e congele a versão de código junto com o checkpoint* — a API estável de `SAM2ImagePredictor/SAM2VideoPredictor` é o modelo a seguir para empacotar modelos de IA no node graph.

**Armadilha a evitar:** *"Feature completa" do release de pesquisa não é produção.* Depender de repo de pesquisa que muda a cada PR (e obriga reinstalar) sem congelamento de versão/cache de artefatos é receita de quebra. O Cosca deve versionar checkpoints+config+código como um artefato único.

---

## Projeto: Real-ESRGAN (xinntao/Real-ESRGAN)

| FIELD | VALUE |
|---|---|
| PROJECT | Real-ESRGAN |
| CATEGORY | Super-resolução (upscale) e restauração de imagem/vídeo |
| PURPOSE | Aumentar resolução de imagens reais (fotos, anime) com qualidade prática; restauração geral |
| ARCHITECTURE | Baseado em BasicSR (toolbox). Generator RRDBNet (ESRGAN) treinado com degradação sintética realista (blur+noise+resize+JPEG) — "blind super-resolution with pure synthetic data". Variantes: RealESRGAN (GAN) e RealESRNet (sem GAN). Model zoo + código de treino + inferência CLI (`inference_realesrgan.py`) e portátil NCNN/Vulkan |
| LANGUAGE | Python (PyTorch), C++/NCNN para executáveis Vulkan |
| CORE ALGORITHMS | GAN super-resolution (ESRGAN/RRDBNet), JPEG artifacts removal, degradação sintética para treino, tile-based inference, TTA (test-time augmentation), post-resize LANCZOS4 para outscale arbitrário, face enhancement via GFPGAN, denoise strength (`-dn`) |
| PIPELINES | `imagem → [tile opcional → RRDBNet x4 → ] → (outscale via LANCZOS4) → (face_enhance GFPGAN) → saída`. Vídeo: por frame (`inference_realesrgan_video.py`). ncnn-vulkan: `-i in -o out -n model -t tile -j load:proc:save` |
| DATA MODEL | Modelos `.pth` (~64MB x4plus; 17MB anime_6B; RealESRGAN_x2plus); imagens como tensores (fp16 default, fp32 opcional); tile caching para imagens grandes |
| GPU/CPU/MEMORY | GPU via PyTorch (fp16); CPU/GPU sem PyTorch via executáveis NCNN/Vulkan (modelos FP16 convertidos); tile size controla o pico de VRAM; batch de frames para vídeo |
| PERFORMANCE | Rápido para o tipo (anime_6B leve; x4plus em GPU); em CPU sozinho é lento — daí a variante NCNN; tile grande = menos seams mas mais VRAM |
| EXTENSIBILITY | CLI, API Python, integrações de terceiros (VapourSynth, Waifu2x-Extension-GUI, Upscayl, nodes ComfyUI); modelos adicionais no model zoo |
| PLUGIN SYSTEM | Inexistente no repo; extensão via integração externa (são os projetos "Projects that use Real-ESRGAN") |
| SECURITY | Sem superfície hostil relevante (inputs são imagens; parsing via PIL/opencv); executa modelos locais; nenhum endpoint remoto |
| LICENSE | BSD-3-Clause (permissiva) |
| STRENGTHS | Prático e bem-embalado (pip, executáveis portáteis, Replicate/Colab), modelo por domínio (foto/anime/vídeo), tile para grandes imagens, código simples de integrar |
| WEAKNESSES | Um único propósito (SR/restauração); manutenção desacelerada (atualizações recentes = modelos pequenos); artefatos/alucinações em fotos complexas; resultado varia por domínio (anime vs foto); tiles causam inconsistência de bloco (documentado no próprio README) |
| TRADE-OFFS | Modelo pequeno/rápido (anime_6B) vs. qualidade (x4plus); tile (memória baixa) vs. seams de borda; GAN (nitidez, artefatos) vs. RealESRNet (estabilidade) |

**Princípio para o Cosca:** *Ofereça o modelo certo para o domínio certo (foto/anime/vídeo) e controle de custo por parâmetro (tile size, fp16, denoise strength, outscale) exposto como inputs de nó* — Real-ESRGAN prova que embalar SR como "nó" com knob de qualidade/VRAM funciona.

**Armadilha a evitar:** *Upscale como "caixa-preta mágica".* Artefatos (seams de tile, alucinação de textura) são inerentes; o Cosca deve expor tile/denoise/post-resize e documentar limitações, ou o usuário culpa a plataforma.

---

## Projeto: libvips (libvips/libvips)

| FIELD | VALUE |
|---|---|
| PROJECT | libvips |
| CATEGORY | Biblioteca de processamento de imagem (pipeline) |
| PURPOSE | Processamento de imagens grandes com uso mínimo de memória e alta velocidade, orientado a servidores/web (thumbnails, conversão, formatos) |
| ARCHITECTURE | "Demand-driven, horizontally threaded": ~300 operações formam um pipeline lazy (grafo de operações) que só computa as regiões realmente pedidas, por linhas (tiles). Cache de regiões computadas. Cada operação roda em threads (threadpool) sobre o pipeline. Loaders/savers plugáveis por codec (libjpeg, libpng, libheif, libjxl, libtiff, pdfium/poppler, OpenSlide…). Build via Meson |
| LANGUAGE | C (sobre GLib/GObject), C++ bindings, CLI `vips`; bindings Python (pyvips), Ruby, PHP, Go, Lua, etc. |
| CORE ALGORITHMS | Lazy/pull-based evaluation (partial evaluation), processamento por regiões/linhas, cache de tiles, operações aritméticas, convolução, morfologia, FFT (fftw), cor (lcms2), histogramas, resampling, piramides DeepZoom (`dzsave`) |
| PIPELINES | `load(arquivo) → operações encadeadas (im1.affine().resize().composite()...) → write`. Nada computa até um write/read demandar; o scheduler calcula só o necessário e descarta o resto |
| DATA MODEL | `VipsImage`: metadados + lazy data (stream de linhas/regiões). Imagens podem ter qualquer nº de bandas, tipos numéricos 8-bit→128-bit complex; "unlimited" tamanho virtual (não carrega tudo em RAM) |
| GPU/CPU/MEMORY | CPU-only (SIMD via highway/orc, threads GLib); memória escala com o tamanho da janela de processamento, não com a imagem — "runs quickly and uses little memory"; arquivos enormes viram streams |
| PERFORMANCE | Referência em speed/memória para conversão e resize de larga escala; mais rápido que ImageMagick em cargas web típicas (ex.: sharp, Mastodon, imgproxy) |
| EXTENSIBILITY | Bindings para ~12 linguagens; loaders/savers via codecs externos; API GObject permite classes custom (mais avançado); CLI para scripting |
| PLUGIN SYSTEM | Limitado (mais "biblioteca extensível via bindings/codecs" do que plugin runtime); módulos carregados em build-time |
| SECURITY | Fuzzing contínuo (OSS-Fuzz badge); política documentada: evitar ImageMagick como backend de load com imagens não confiáveis (ataque de superfície grande); loaders de cada codec herdando riscos do codec |
| LICENSE | LGPL-2.1-or-later (linkagem dinâmica ok; cuidado com distribuição binária estática) |
| STRENGTHS | Modelo de memória excepcional para pipelines de imagem (demand-driven), ecossistema de produção enorme (sharp, Mastodon, Rails Active Storage, imgproxy), cobertura de formatos ampla, mature e mantido ativamente |
| WEAKNESSES | Sem GPU; LGPL-2.1 (não ideal para fechar); API de threading/async sutil de entender; doc de algumas operações esparsa; dependência GLib |
| TRADE-OFFS | Lazy/streaming (memória baixa) vs. complexidade de pipeline vs. simplicidade de imagem-em-RAM; LGPL vs. Apache-2.0 |

**Princípio para o Cosca:** *Adote execução demand-driven/lazy: represente operações de imagem como pipeline e só compute o que for solicitado* — o Cosca deve usar o mesmo padrão no node graph: nós não materializam resultados intermediários gigantes a menos que um downstream demande.

**Armadilha a evitar:** *Usar libvips (via LGPL) em binário estático fechado* sem entender as obrigações de linkagem dinâmica; e *não habilitar ImageMagick como backend de load em serviços que recebem imagens não confiáveis* (superfície de ataque — melhor codecs dedicados).

---

## Projeto: ImageMagick (ImageMagick/ImageMagick)

| FIELD | VALUE |
|---|---|
| PROJECT | ImageMagick |
| CATEGORY | Suite de edição/conversão de imagem |
| PURPOSE | Criar, editar, compor e converter bitmaps em 200+ formatos; CLI + APIs para automação |
| ARCHITECTURE | C monolito modular: `MagickCore` (núcleo), `MagickWand` (API de alto nível), `Magick++` (C++), `PerlMagick`; `coders/` = um codec por formato (JPEG, PNG, GIF, TIFF, PDF, HEIC…); `filters/` = operadores; delegates invocam programas externos (ghostscript, ffmpeg…); pixel cache em 3 níveis (RAM → disco → distribuidores remotos); OpenMP threads; OpenCL para certos algoritmos; MVG para drawing; policy.xml para segurança |
| LANGUAGE | C (núcleo), C++ (Magick++), scripts de automação, OpenCL kernels |
| CORE ALGORITHMS | Convolução, morphing/morfologia, connected components, convex hull, DFT (fft), CLAHE/histogram equalization, bilateral blur, distorções (perspectiva), color management (ICC), encipher/decipher, perceptual hash, resize/rotate/trim |
| PIPELINES | `convert input [operators] output` — encadeamento CLI; pixel cache em memória/disco/distribuído para processar além da RAM; Q8/Q16/Q32 builds + HDRI |
| DATA MODEL | `Image`: pixels em memória (quantum, Q8/Q16/Q32, HDRI float) + metadados (EXIF, profiles ICC, comentários, frames de animação, multi-image); pixel cache com paginação para disco |
| GPU/CPU/MEMORY | CPU (OpenMP multi-core); OpenCL para acelerar subset; memória: imagem inteira em RAM/pixel cache (diferente do streaming do libvips) — memória é o gargalo para imagens gigantes; opções de tcmalloc/SSD para cache |
| PERFORMANCE | Rápido para tarefas médias e conveniente (scripting); mais lento que libvips para tarefas web de larga escala (documentado pela comunidade) |
| EXTENSIBILITY | CLI onipresente (automação/scripts), APIs (Wand/C++/Perl), coder modules (build-time), Magick.NET; foco em compatibilidade de formatos |
| PLUGIN SYSTEM | Fraco (sem hot-plugin de runtime; coders são build-time) |
| SECURITY | Superfície de segurança notoriamente grande: histórico de CVEs (ImageTragick), parsing de 200+ formatos. Mitigações: `policy.xml` com policies Open/Limited/Secure/Websafe (default desde 7.1.1-16), `--enable-64bit-channel-masks`, fuzzing via OSS-Fuzz, validador de policy externo. **Uso com inputs não confiáveis exige policy restrita** |
| LICENSE | ImageMagick License (permissiva, estilo Apache-2.0 modificado) |
| STRENGTHS | Suporte massivo de formatos, CLI maduro e ubíquo, recursos ricos (draw, morph, HDR, motion picture), distribuição onipresente, licença permissiva |
| WEAKNESSES | Consumo de memória alto em imagens grandes (imagem inteira em RAM/cache), histórico de segurança ruim, pipeline thread-safety com nuances, evolução conservadora do core |
| TRADE-OFFS | Conforto/cobertura de formatos vs. velocidade (libvips) e vs. segurança (200+ parsers); HDRI (precisão) vs. 2× memória; Q8/Q16 vs. precisão |
| OPINIÃO DE MATURIDADE | Muito maduro e usado em produção, mas a classificação deve baixar por superfície de segurança e memória: usar como utilitário de conversão com policy restrita, não como engine central de pipeline |

**Princípio para o Cosca:** *Segurança por política: se processar imagens não confiáveis, rode o conversor com policy mínima (formats/reads restritos, resource limits, sem delegates) — e prefira libvips para carga web.* O Cosca deve adotar um "security policy" configurável como padrão de plataforma para qualquer processo que ingira arquivos.

**Armadilha a evitar:** *Expor ImageMagick como serviço de conversão sem policy.* É o clássico vetor (ImageTragick); sempre sandbox + policy restritiva + desabilitar delegates (Ghostscript/PDF) se não for necessário.

---

## SÍNTESE DA CATEGORIA

### 5 princípios fortes para o Cosca
1. **Grafo = dados puros, execução derivada e cacheável por assinatura** (ComfyUI): serializar o workflow como JSON, ordenar topologicamente, executar por demanda e só re-executar o que mudou. Esta é a espinha dorsal do node graph do Cosca.
2. **Execução demand-driven/lazy com memória sob controle** (libvips + ComfyUI): nunca materialize intermediários gigantes; compute por regiões/sob demanda e exponha knobs de memória (tile, fp16, headroom) como inputs de nó.
3. **Modelos empacotados como bibliotecas com interface mínima e estável + versão congelada de código/checkpoint/config como um artefato único** (SAM 2, Real-ESRGAN).
4. **Separe camadas por propósito**: CV clássica → OpenCV; conversão/formato web → libvips; inferência de modelos → runtime dedicado; upscale/segmentação → modelos especializados (Real-ESRGAN, SAM 2). Não usar uma única lib para tudo.
5. **Segurança como política por padrão**: parsing de imagem é superfície de risco (ImageMagick/OpenCV); sandbox + policy mínima + fuzz desde o início, e plugins com código arbitrário ficam fora do núcleo.

### 5 armadilhas comuns a evitar
1. **Adotar código GPL-3.0/LGPL-2.1 no núcleo proprietário** sem análise de linkagem (ComfyUI, libvips): usar como referência de arquitetura ou via linkagem dinâmica/processo separado.
2. **Tratar repo de pesquisa como produção** (SAM 2, Real-ESRGAN): releases que quebram, checkpoints atados a commits, poucos testes; congele versões e automatize cache de artefatos.
3. **Cache ingênuo por node_id em vez de por assinatura de inputs** — ou cache cuja chave ignora dependências transitivas/IS_CHANGED: ou re-executa demais ou (pior) devolve resultado obsoleto.
4. **Subestimar consumo de memória de imagem em RAM** (ImageMagick-style) ao construir pipeline para web: imagens grandes derrubam o serviço; adote streaming/lazy e limites de recurso por request.
5. **Ignorar a dimensão de artefatos do domínio** (seams de tile, alucinação de SR, consistência de máscara em vídeo): se a plataforma não expõe os trade-offs e knobs (tile/denoise/refinamento), a culpa recai sobre a ferramenta.
