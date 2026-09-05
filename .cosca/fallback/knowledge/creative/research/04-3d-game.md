# Research Matrix — 3D/Graphics + Game

**Contexto da plataforma:** Cosca = criação multimídia assistida por IA. Hardware-alvo do dev: GPU AMD ROCm **gfx1030** (RDNA2 / Radeon RX 6000, Vulkan 1.3 completo). Todas as avaliações de compatibilidade Vulkan abaixo levam isso em conta.
**Método:** dados primários obtidos dos README/estrutura dos repositórios em 2026-08-13. "Não verificado" = não confirmado diretamente nas fontes lidas. Estrelas NÃO são proxy de qualidade — avaliou-se maturidade, arquitetura e licença.

---

## Projeto: Blender

| Campo | Valor |
|---|---|
| PROJECT | blender/blender (mirror oficial; upstream projects.blender.org) |
| CATEGORY | 3D/Graphics — DCC (Digital Content Creation) completo |
| PURPOSE | Suite 3D completa: modelagem, rigging, animação, simulação, render, compositing, tracking, edição de vídeo |
| ARCHITECTURE | Núcleo C++ modular: `source/blender/` (blenkernel=DNA/RNA, makesdna, makesrna, editors, render), `intern/` (subsistemas), `extern/` (libs). Data-oriented: **DNA** (estruturas de dados persistidas) + **RNA** (geração de API/reflexão) + UI em `editors/`. Build CMake; SCU build para acelerar compilação |
| LANGUAGE | C, C++ (C++17/20), Python (API/scripts/addons), GLSL/HLSL, CUDA, HIP |
| CORE ALGORITHMS | Subdivisão (OpenSubdiv), remesh (Voxel/Bisector), sculpt brushes, fluid/smoke (Mantaflow), cloth/softbody (posição-BD), simulation nodes, BVH traversal, volume rendering (openVDB), denoising (OIDN via CPU, OptiX/HIP) |
| PIPELINES | Cadeia completa asset→render→composite: geometry nodes → shading nodes → EEVEE (raster, forward) / Cycles (path tracing, BVH, unificado CPU/GPU) → compositor → sequencer |
| DATA MODEL | **DNA** (blocos de memória versionados, persistentes em .blend) + **RNA** (camada reflexiva para Python/API); Scene Graph centrado em objetos + datablocks; nodos em grafos (geometry/shading) |
| GPU/CPU/MEMORY | CPU primário para DCC; GPU via Cycles/EEVEE e viewport (Vulkan/Metal/DX12; OpenGL fallback). Cycles suporta **HIP** (ROCm, AMD) e CUDA/OptiX. EEVEE Vulkan ✓ |
| PERFORMANCE | Forte em simulação/dados; Cycles otimizado com denoise; viewport Vulkan ainda em maturidade; SCU reduz tempo de build |
| EXTENSIBILITY | Muito alta: Python API completa (bpy), addons, nodos custom, engines de render custom (addon), operadores |
| PLUGIN SYSTEM | Addons Python nativos + FFmpeg para mídia; sistema robusto e maduro |
| SECURITY | Modelo: conteúdo de .blend/scripts de terceiros é risco; sem sandbox de execução Python (executa com privilégio do usuário) — risco conhecido |
| LICENSE | GPL v3 (or later per-file), compatível |
| STRENGTHS | Suite mais completa do mercado open-source; Cycles/EEVEE de classe industrial; DNA/RNA = referência em persistência+reflexão de dados; enormes investimento e comunidade |
| WEAKNESSES | GPL contamina embedding (difícil usar como lib interna fechada); base de código gigantesca (~10M LOC) e complexa; viewport/EEVEE ainda atrás de engines dedicadas; UI/arq antiga em partes |
| TRADE-OFFS | Poder total vs. integrabilidade: não é *embedable* como motor, é uma aplicação. Para Cosca: melhor usado como **pipeline externo** (via .blend/Python, batch) do que como núcleo embutido |

**Princípio para o Cosca:** Modelo de dados versionado + camada de reflexão gerada (DNA→RNA) é o padrão-ouro para DCC: manter um "schema canônico" com reflexão automática evita duplicação estado/UI/API. Adotar uma variante leve (schema binário versionado + gerador de binding) para assets e documentos.

**Armadilha a evitar:** Tratar Blender como motor embutível — a licença GPL e o acoplamento UI/aplicação tornam isso inviável para um produto fechado. Use via subprocesso/Python API com contratos de dados estáveis (.blend, glTF, USD) e nunca forke o core.

---

## Projeto: Skia

| Campo | Valor |
|---|---|
| PROJECT | google/skia |
| CATEGORY | 3D/Graphics — renderer 2D vetorial GPU/CPU |
| PURPOSE | Biblioteca 2D completa para desenhar texto, geometria e imagens (base do Chrome/Android/Firefox) |
| ARCHITECTURE | Camadas: API C++ (SkCanvas/SkSurface) → `src/gpu/` com Graphite (modern GPU backend) e Ganesh (legado) → Ganesh/Graphite sobre Vulkan/Metal/D3D/OpenGL; rasterizador CPU (RasterPipeline/Software) via SkRasterPipeline; text/shaping via Skia + Harfbuzz/ICU; dither/AA por ops |
| LANGUAGE | C++ (17), Rust (módulos novos, ex. `rust/`), Python/Go para tooling |
| CORE ALGORITHMS | Rasterização scanline com AA, GPU tessellation de curvas/patches, SDF text rendering, color management (CMS), gradients/patterns, glyph atlas, SkRasterPipeline (SIMD), PathOps (booleanas de paths) |
| PIPELINES | Paint → Atlas/ops → RasterPipeline (CPU) ou Graphite recorder → GPU draw; Culling e reordenamento de ops; texture/page para fontes |
| DATA MODEL | SkPaint (estado), SkPath (geometria), SkImage/SkSurface/SkPicture (retained scene); op buffers; modelo de "recording" (SkPicture) |
| GPU/CPU/MEMORY | Dual: CPU SIMD + GPU. Vulkan ✓ (RDNA2/gfx1030 ok), Metal, D3D12; Graphite usa descriptor pools e pass layout por backend |
| PERFORMANCE | Excelente — otimizado a nível de cache de ops, batching de draw calls, atlas de glyphs; benchmarks próprios (skp/bench) |
| EXTENSIBILITY | Moderada: API de backend novo é um contrato grande; mais fácil extender primitivas de draw do que adicionar backend |
| PLUGIN SYSTEM | Não é plugin-based; integração via classes de backend (Ganesh/Graphite) e fábrica de contexto; forte uso como lib única |
| SECURITY | Foco em safety (Chrome sandbox); fuzzers dedicados (`fuzz/`); memory-safe esperado no consumo — não verificado para todos os backends |
| LICENSE | BSD-3-Clause |
| STRENGTHS | 2D vetorial de altíssima qualidade e performance (referência); suporta Vulkan de primeira classe; maduro (Chrome/Android); text shaping completo |
| WEAKNESSES | Sem API estável/garantias de ABI para consumo externo (feito para uso interno Google); build pesado (GN/Bazel, toolchains); foco 2D — sem 3D |
| TRADE-OFFS | Qualidade máxima 2D vs. complexidade de build/integração e ausência de suporte 3D. Para Cosca: ideal para compositing/canvas 2D dos UI/timeline, não para viewport 3D |

**Princípio para o Cosca:** Separar "recording de ops" (SkPicture) da execução (CPU/GPU) permite cachear cenas 2D e re-renderizar em múltiplos backends — adotar pipeline de comando gravável para toda UI/2D da plataforma (gravar → validar por IA → executar no backend alvo).

**Armadilha a evitar:** Não assumir que Skia é "drop-in": sem API estável e com build complexo, o custo de integração é alto. Evite embarcar Skia diretamente; prefira usar via Chromium/Firefox stack ou abstrair por trás de uma interface de canvas própria caso precise de troca futura.

---

## Projeto: OpenUSD (Universal Scene Description)

| Campo | Valor |
|---|---|
| PROJECT | PixarAnimationStudios/OpenUSD (URL correta; "AcademySoftwareFoundation/OpenUSD" não existe — 404) |
| CATEGORY | 3D/Graphics — scene description / composição de cena |
| PURPOSE | Sistema escalável para authoring, leitura e streaming de cenas com tempo (time-sampled) para intercâmbio entre aplicações gráficas |
| ARCHITECTURE | Pacotes C++ em `pxr/`: `usd` (core: prim/schema/relationship), `sdf` (camadas persistidas), `usdGeom/usdShade/usdSkel/...` (schemas), `usdImaging` (render), `hydra` (engine de render pluggável); build via CMake/`build_usd.py` |
| LANGUAGE | C++ (17), Python (bindings, ferramentas, plugins), TBB; WASM/iOS/visionOS builds possíveis |
| CORE ALGORITHMS | Composição de camadas (arcs: subLayers, references, payloads, specializes, inherits, variants), time-sampling, instancing/point instancing, Stage cache, **composition arc resolution** (o algoritmo central: resolver overrides em rede de layers) |
| PIPELINES | Author (layers) → Stage → composition → Hydra (render delegate: renderer pluggável) → imagem; **usdview** p/ preview; plugins para formatos (usda/usdc/usdz) |
| DATA MODEL | Prim-based: prims + attributes + relationships; composição via arcs (não cópia!); **scene description é declarativa e extensível por schema**; time-sampled values |
| GPU/CPU/MEMORY | CPU-pesado (composição/traversal); Hydra delega render ao delegate (GPU opcional). Não exige GPU própria; Vulkan compatível via delegate de render de terceiros (não verificado no core) |
| PERFORMANCE | Escala para cenas de filme (billhões de prims) via instancing e culling lazy; carga sob demanda (payloads) |
| EXTENSIBILITY | Muito alta: schemas custom (USD plugin), render delegates (Hydra), formatos de arquivo, asset resolver — arquitetura pluggável de ponta a ponta |
| PLUGIN SYSTEM | Plugin system central e formal: schemas, delegates, resolvers, codecs (usdc/usdz), registrados via manifest |
| SECURITY | Parser de conteúdo externo (arquivos usd) = superfície de ataque; mantém SECURITY.md/Política; parsing de .usd de fontes não confiáveis deve ser tratado como risco |
| LICENSE | Apache-2.0 (licença original da Pixar, com Apache-2.0 para contribuições modernas) |
| STRENGTHS | Padrão de facto da indústria (Pixar, NVIDIA, Apple, USD Workgroup); composição por arcs é única; Hydra separa cena de render; suporta WASM |
| WEAKNESSES | Complexidade conceitual alta (arcs, schemas, hydra); build grande e frágil (deps: TBB, OpenSubdiv, etc.); documentação histórica com lacunas; core ativo mas curva de aprendizado íngreme |
| TRADE-OFFS | Potência/portabilidade de cena vs. complexidade de integração. Para Cosca: é o formato canônico de *scene graph interchange* com IA (gerar USD declarativamente é natural), mas não use para state interno do motor |

**Princípio para o Cosca:** Cenas como **composição declarativa de camadas** (referências/payloads/variants em vez de árvore concreta copiada) é o padrão certo para uma plataforma de IA: a IA edita camadas textuais/diffáveis, e o resultado é resolvido no consumo — habilita undo, A/B e colaboração por composição em vez de mutação.

**Armadilha a evitar:** Usar USD como modelo de runtime do motor (performance e complexidade). USD é camada de *interchange/interop*; o runtime deve ter scene graph próprio e converter via importador. Também: não duplicar "intercâmbio" ad-hoc quando o formato universal já resolve o problema.

---

## Projeto: Vulkan-Docs

| Campo | Valor |
|---|---|
| PROJECT | KhronosGroup/Vulkan-Docs |
| CATEGORY | 3D/Graphics — especificação de API gráfica |
| PURPOSE | Especificação Vulkan® e Vulkan SC, registry XML (`vk.xml`), geração de headers e referência |
| ARCHITECTURE | Fonte única Asciidoctor (`chapters/`, `appendices/`) + **XML API Registry** (`vk.xml`) que gera headers, docs e tabelas; build com make/asciidoctor; extensões documentadas em `proposals/` e registradas no registry |
| LANGUAGE | Asciidoctor (docs), Python (scripts), XML (registry), C (headers gerados) |
| CORE ALGORITHMS | Não é engine — define o modelo: comandos, pipelines, memory model, synchronization (happens-before / memory barriers), validation layers, extensões |
| PIPELINES | vk.xml → geradores → headers (`vulkan*.h`) + spec; validação via Vulkan Validation Layers (projeto separado) |
| DATA MODEL | Registry formal de tipos/enums/commands/protocols com versionamento (promoção de extensões em versões core: 1.0→1.1→1.2→1.3) |
| GPU/CPU/MEMORY | Modelo explícito: alocação de memória pelo app, barreiras, synchronização manual; **Vulkan 1.3 = RDNA2/gfx1030 suporta (VK_KHR_dynamic_rendering, synchronization2, etc.)** ✓ |
| PERFORMANCE | Baixo overhead + previsível; custo para o desenvolvedor (verbosidade) |
| EXTENSIBILITY | Altíssima: modelo de extensões de API (vendor/ext) com geração automática; registry é a fonte de verdade |
| PLUGIN SYSTEM | Extensões (equivalentes de plugins) — portas para features de vendors; Loader permite camadas/validation dinâmicas |
| SECURITY | Spec define safety de GPU memory/sync; responsabilidade do app; validation layers são a principal ferramenta de segurança/correção |
| LICENSE | Apache-2.0 / MIT (spec e headers, REUSE), dual para conteúdo |
| STRENGTHS | Padrão aberto multi-vendor; cobertura total de HW (inclusive AMD ROCm); geração de bindings de alta qualidade (vulkan.hpp) |
| WEAKNESSES | Não é uma implementação (é spec); complexidade enorme para novatos; sem high-level abstração |
| TRADE-OFFS | Controle máximo vs. esforço. Para Cosca: **usar Vulkan diretamente só onde necessário**; a abstração deve vir de cima (wgpu ou engine) |

**Princípio para o Cosca:** Fonte de verdade única e **geração de código a partir de schema/registry** (XML → headers + docs + bindings) elimina drift entre spec, API e documentação. Para a plataforma: gerar bindings/tipos de eventos, schemas e docs a partir de um único registry versionado.

**Armadilha a evitar:** Programar gráficos diretamente sobre Vulkan raw no projeto — o custo de manutenção (sync, memory, validation) é enorme e há abstrações maduras (wgpu) que já fazem isso com segurança. Vulkan entra como alvo/backend, não como API de produto.

---

## Projeto: wgpu

| Campo | Valor |
|---|---|
| PROJECT | gfx-rs/wgpu |
| CATEGORY | 3D/Graphics — abstração GPU cross-platform |
| PURPOSE | API gráfica segura, cross-platform, pura Rust; base do WebGPU nativo (Firefox, Servo, Deno) |
| ARCHITECTURE | Camadas: `wgpu` (API high-level Rust) → `wgpu-core` (validação/tracking) → `wgpu-hal` (HW abstraction) → backends Vulkan/Metal/D3D12/OpenGL/GLES/WebGPU; shaders via **Naga** (WGSL→SPIR-V/HLSL/GLSL/Metal) |
| LANGUAGE | Rust (100%); MSRV 1.87 (wgpu) / 1.93 (workspace) |
| CORE ALGORITHMS | Resource tracking e lifetime (refcount + queue tracking), validação de pipeline, tradução de shaders (Naga: WGSL frontend + múltiplos backends), command buffer encoding, buffer/memory mapping seguro |
| PIPELINES | WGSL shader → Naga → native shader; API → wgpu-core (validação) → wgpu-hal → Vulkan/Metal/D3D/GL; swapchain/present; CTS (conformance) para WebGPU |
| DATA MODEL | WebGPU object model: Adapter, Device, Queue, Pipeline, BindGroup/Layout, Buffer, Texture; modelo de borrow/track por recurso |
| GPU/CPU/MEMORY | **Vulkan backend: suporte pleno a RDNA2/gfx1030** (Vulkan 1.3), também via MoltenVK em Mac; D3D12/Metal/GL; memória gerenciada (não explícita como Vulkan raw) |
| PERFORMANCE | Bom e em melhora; overhead de tracking vs. controle explícito (trade-off conhecido); CTS rigoroso garante conformidade |
| EXTENSIBILITY | Alta: abstração limpa entre camadas; backends novos (ex.: Vulkan/Vulkan+Ray tracing) adicionáveis via wgpu-hal |
| PLUGIN SYSTEM | Não é plugin-based no sentido de extensões runtime; extensibilidade via traits/interfaces Rust + ecosystem crates |
| SECURITY | Memory-safe por construção (Rust), validação interna (wgpu-core), sandbox-ready (usado no Firefox); grande foco em safety |
| LICENSE | Apache-2.0 OU MIT (dual) |
| STRENGTHS | Única abstração segura cross-GPU moderna (Vulkan+Metal+D3D+GL+Web); backends Vulkan de primeira classe (crítico p/ AMD ROCm); futuro WebGPU; testado por browsers |
| WEAKNESSES | WebGPU API ainda draft (muda frequentemente); recursos low-level (ex.: ray tracing, mesh shaders) limitados em comparação a Vulkan raw; eco jovem em alguns pontos |
| TRADE-OFFS | Segurança/portabilidade vs. controle de baixo nível. Para Cosca: **escolha principal de camada gráfica**: une Vulkan (AMD ROCm), web (WASM) e DX12 num só caminho de render |

**Princípio para o Cosca:** Uma única camada gráfica segura com múltiplos backends (Vulkan/GL/WebGPU) reduz drasticamente o número de shaders e bug de plataforma — renderizar uma vez, emitir em vários alvos. Adotar wgpu como camada de abstração GPU para todo o viewport e pipelines 2D/3D.

**Armadilha a evitar:** Não tratar a API WebGPU/wgpu como estável final: pinar versão, seguir changelogs e guardar o caminho de migração (breaking releases a cada ~3 meses). Evitar também exigir features Vulkan-raw (ray tracing/mesh shaders) em v1 — fique no subset suportado em todos backends.

---

## Projeto: Filament

| Campo | Valor |
|---|---|
| PROJECT | google/filament |
| CATEGORY | 3D/Graphics — engine de renderização PBR em tempo real |
| PURPOSE | Engine real-time physically based rendering para Android, iOS, Linux, macOS, Windows e WASM (otimizada p/ mobile) |
| ARCHITECTURE | Núcleo C++ mínimo (`filament/`): `backend/` (drivers Vulkan/Metal/OpenGL/WebGPU/WebGL), `Engine/Renderer/View/Scene/Camera`; entidades via EntityManager; materiais compilados offline (matc) para binário; libs: gltfio, ibl, filamat, filamesh, math, filagui (ImGui) |
| LANGUAGE | C++ (17), Java/Kotlin (Android), JS (WASM), shaders GLSL/Metal/SPIR-V gerados por material |
| CORE ALGORITHMS | Clustered forward rendering, Cook-Torrance microfaceta BRDF, PBR (metalness/roughness, clearcoat, sheen, anisotropy, SSS aprox.), IBL pré-filtrado (cmgen), cascaded shadows (PCF/PCSS/EVSM/DPCF), SSAO/SSR/SSR-refração, tone mapping (ACES/AgX/GT7), bloom/DoF, TAA/FXAA/MSAA, dynamic resolution + FSR |
| PIPELINES | Material (.mat) → **matc** (compilador) → pacote binário → MaterialInstance; glTF2.0 → gltfio → render; assets IBL via cmgen; Runtime: Renderer::render(View) por frame |
| DATA MODEL | Entity-Component (EntityManager); Scene (renderables+lights), View (camera+viewport), Camera físico; material packages serializados (filaflat) |
| GPU/CPU/MEMORY | **Vulkan 1.0+ backend completo — compatível com gfx1030/RDNA2 ✓**; GL/ES, Metal, WebGPU, WebGL2; design "o menor e mais eficiente possível no Android" (pequeno footprint, baixa memória) |
| PERFORMANCE | Excelente p/ mobile e desktop; clustered forward = baixo custo de lighting; FSR para upscaling; renderização determinística |
| EXTENSIBILITY | Média: backends plugáveis (driver interface), materiais custom via matc, extensões glTF via KHR_*; core fechado e opinionado (não é motor de jogo completo) |
| PLUGIN SYSTEM | Não é plugin-based no runtime; extensão via materiais, backends e libs auxiliares (gltfio, viewer, filagui) |
| SECURITY | API C++ pura com fronteiras claras; foco em robustez; conteúdo (materiais/glTF) é o risco principal — não verificado sandbox |
| LICENSE | Apache-2.0 |
| STRENGTHS | PBR de qualidade de produto com footprint pequeno; multi-backend (Vulkan/Metal/GL/Web); glTF 2.0 maduro; documentação técnica excepcional (math/BRDF explicados) |
| WEAKNESSES | Engine de render só (sem cena/gameplay/physic); backend WebGPU ainda em evolução; ferramentas host (matc) devem casar versão da lib |
| TRADE-OFFS | Qualidade PBR compacta vs. escopo restrito a render. Para Cosca: bom backend de render "embutível" para viewport 3D e preview PBR, alternativo ao Blender/UE |

**Princípio para o Cosca:** **Compilar materiais offline para pacotes binários** (matc) em vez de shaders em runtime dá: validação prévia, cross-backend único, cache determinística e revisão por IA dos assets. Pipeline de asset = compile-time, não runtime.

**Armadilha a evitar:** Misturar versões entre ferramentas host e runtime (matc/engine) quebra o formato — versionar o pacote de material junto do engine. Evitar também usar Filament para lógica de jogo/scene: ele não tem scene graph rico nem animação de alto nível.

---

## Projeto: Open3D

| Campo | Valor |
|---|---|
| PROJECT | isl-org/Open3D |
| CATEGORY | 3D/Graphics — processamento de dados 3D |
| PURPOSE | Biblioteca moderna p/ processamento de dados 3D: point clouds, meshes, RGBD, registro, reconstrução, visualização, ML 3D |
| ARCHITECTURE | C++ core (`cpp/`) + bindings Python (`python/`); camadas: Geometry (TriangleMesh, PointCloud, VoxelGrid), Pipelines (registration, reconstruction, SLAM/RGBD), Visualization (GUI/visualizer), ML (Open3D-ML: PyTorch/TensorFlow ops), Tensor (tensor op framework próprio) |
| LANGUAGE | C++ (17), Python, CUDA |
| CORE ALGORITHMS | ICP (point-to-point/plane), point cloud registration (global: RANSAC, FGR), RGB-D odometry e integração (TSDF), mesh simplification/reconstruction (Poisson, marching cubes), normals/curvature, RANSAC shape detection, segmentation (clustering), raycasting, BVH/octree, voxel downsampling |
| PIPELINES | RGB-D → odometry → TSDF → mesh → registro → visualização; point cloud → features (FPFH) → registration → alignment; ML: dados → tensors → modelos PyTorch/TF → inferência em 3D |
| DATA MODEL | Geometria typed (PointCloud/TriangleMesh/VoxelGrid/TSDF/Octree) + **Tensor** (unificado CPU/GPU); dados densos (arrays) |
| GPU/CPU/MEMORY | GPU via **CUDA** (principal) — ROCm/HIP **não confirmado**; CPU paralelo (TBB); wheel CPU-only disponível (open3d-cpu); visualização via GUI (delegates GPU) |
| PERFORMANCE | Otimizado p/ pontos e nuvens; GPU acelera registro/reconstrução; grande conjunto de benchmarks — não verificado em detalhe |
| EXTENSIBILITY | Alta: Python-first para pipelines; C++ para perf; Open3D-ML plugável a frameworks |
| PLUGIN SYSTEM | Não é plugin-based formal; extensão por bindings Python e integração com PyTorch/TF |
| SECURITY | SLSA build attestation (novo) + OpenSSF alinhamento (bom sinal); parsing de meshes/point clouds de fontes externas = risco (não verificado sandbox de formatos) |
| LICENSE | MIT |
| STRENGTHS | Suíte mais completa de algoritmos 3D-data open-source; Python-first (ótimo p/ IA); GPU aceleração real; SLSA/supply-chain attestation (raro e bom) |
| WEAKNESSES | **GPU = CUDA-first: sem ROCm/HIP confirmado (crítico para o hardware AMD do Cosca)**; bindings/ABI mudam; dependências pesadas; foco em dados 3D, não em rendering de produto |
| TRADE-OFFS | Velocidade de pipeline (dados 3D) vs. portabilidade GPU (CUDA) e escopo (não é renderer). Para Cosca: excelente para *geometry processing* e integração com modelos de visão 3D, contanto que os pipelines críticos rodem em CPU ou tenham backend alternativo |

**Princípio para o Cosca:** Para dados 3D "de verdade" (nuvens, RGBD, meshes para reconstrução/modalidades de IA), usar bibliotecas especializadas em vez de reinventar geometria computacional — integrar Open3D como *geometry toolkit* atrás de uma interface própria de dados.

**Armadilha a evitar:** Assumir aceleração ROCm: Open3D usa CUDA e **não há confirmação de HIP/ROCm** — em gfx1030 os pipelines GPU de Open3D podem não rodar. Planejar fallback CPU (open3d-cpu) e validar com `usecuda=False` antes de prometer aceleradores AMD. Evitar também acoplar formatos de asset proprietários à ABI do Open3D.

---

## Projeto: assimp

| Campo | Valor |
|---|---|
| PROJECT | assimp/assimp |
| CATEGORY | 3D/Graphics — importador/exportador de assets 3D |
| PURPOSE | Carregar 40+ formatos de arquivos 3D em um modelo único em memória (importação + exportação + pós-processamento de mesh) |
| ARCHITECTURE | C++ core (`code/`): `AssetLib/` (um importador/exportador por formato), `Common/`, `PostProcessing/`, `CApi/` (C API); cada formato = módulo pluggável registrado por extensão/magic; pipeline: file→importer→scene (aiScene)→post-process steps→app |
| LANGUAGE | C++ (17), C API; bindings: C#, Java, Python, Rust (russimp), JVM, etc. |
| CORE ALGORITHMS | Pós-processamento: geração de normals/tangents (MikkTSpace), triangulação, otimização de vertex cache, remoção de primitivas degeneradas/duplicados, ordenação por primitive type, fusão de materiais; mesh decimation |
| PIPELINES | Importação: detector de formato → importer → `aiScene` normalizado → post-process chain configurável (flags aiProcess_*) → dados do app; exportação simétrica |
| DATA MODEL | **`aiScene`** unificado: aiNode (hierarquia), aiMesh, aiMaterial, aiAnimation, aiTexture, aiCamera, aiLight — modelo neutro entre formatos |
| GPU/CPU/MEMORY | CPU-only (parsing); leve e sem GPU; uso de memória proporcional ao mesh — não verificado em perf extremo |
| PERFORMANCE | Bom para asset loading offline; post-processing custo linear; threads opcional — não verificado |
| EXTENSIBILITY | Alta: adicionar um formato = novo módulo AssetLib seguindo interface Import/Export; post-process steps custom |
| PLUGIN SYSTEM | Módulos de formato registrados por extensão (build-time/CMake flags); não é plugin runtime dinâmico |
| SECURITY | Parsing de arquivos arbitrários = risco clássico; projeto tem fuzz (`fuzz/`, Google fuzzer) e SECURITY.md; **histórico de CVEs em parsers (ex.: FBX)** — atenção |
| LICENSE | BSD-3-Clause (estático permitido) |
| STRENGTHS | Cobertura de formatos enorme (40+, FBX/Collada/glTF/OBJ/STL/3MF/IFC); modelo `aiScene` neutro; pós-processamento pronto para game; licença permissiva |
| WEAKNESSES | Parsers com histórico de bugs de segurança; alguns formatos são "import-only" com fidelidade variável; projeto de comunidade com manutenção espinhosa em partes |
| TRADE-OFFS | Amplitude de formatos vs. risco de parsing/fidelidade. Para Cosca: entrada de assets é caso de uso ideal (upload de .obj/.fbx/.stl) — mas sanitize e rode em processo isolado |

**Princípio para o Cosca:** Normalizar TODOS os formatos de entrada para um **modelo canônico em memória** (tipo `aiScene`) na fronteira da plataforma — o resto da stack (IA, preview, export) conversa apenas com o modelo neutro, não com cada formato.

**Armadilha a evitar:** Processar arquivos 3D não confiáveis no processo principal — parsers de FBX/Collada já tiveram CVEs; rodar assimp em worker/sandbox isolado (e preferir glTF/USD como formatos primários, que são mais simples e seguros de parsear).

---

## Projeto: Godot

| Campo | Valor |
|---|---|
| PROJECT | godotengine/godot |
| CATEGORY | Game — engine de jogo 2D/3D multi-plataforma |
| PURPOSE | Engine de jogo completo (editor + runtime) 2D/3D, export 1-clique para desktop, mobile, web e consoles |
| ARCHITECTURE | C++ core: `core/` (Variant, Object, RefCounted, String, math), `scene/` (SceneTree/Node tree), `servers/` (RenderingServer, PhysicsServer, AudioServer — API de baixo nível), `modules/` (GDExtension, rendering, plugins), `platform/` (OS-specific), `editor/`; build SCons |
| LANGUAGE | C++ (core), GDScript (linguagem principal), C# (via .NET), GDExtension (C++), Godot 4 também Rust via GDExtension |
| CORE ALGORITHMS | Scene tree traversal, node lifecycle (enter/ready/process/physics), physics (Bullet/Godot Physics 3D, Jolt opcional), rendering (forward+, mobile, compatibility via Vulkan/GLES3), animation (AnimationPlayer, blend trees), ECS-like p/ internals, RID-based servers |
| PIPELINES | Projeto (scenes .tscn/.scn + scripts .gd) → editor → export templates → binário por plataforma; render via RenderingServer (Vulkan/GLES3/GLES2) |
| DATA MODEL | **Scene Tree de Nodes** (hierárquica, composicional) + SceneTree singletons; cenas = templates instanciáveis; recursos (Resource) serializáveis; GDScript atua como data/scripts |
| GPU/CPU/MEMORY | **Vulkan 1.0+ (RenderingMethod forward+) e GLES3 — compatível com gfx1030/RDNA2 ✓**; mobile/compatibility p/ baixo-end; memória gerenciada por refcounting (Object) |
| PERFORMANCE | Bom p/ escopo indie; rendering menos maduro que UE/Unity em AAA; physics via Jolt opcional melhora; GDScript mais lento que C#/GDExtension (mas mais rápido de desenvolver) |
| EXTENSIBILITY | Muito alta: GDScript/C#/GDExtension, módulos nativos, plugins de editor, asset library |
| PLUGIN SYSTEM | Forte: Addons/plugins (asset lib), GDExtension para C++, sistema de mods p/ games; editor extensível |
| SECURITY | MIT puro; scripts GDScript executados confiam no conteúdo do jogo; export templates; fuzz/CI — risco de scripts de terceiros é conhecido (executam com permissão do usuário) |
| LICENSE | MIT |
| STRENGTHS | Editor e runtime open-source completos; curva de aprendizado gentil; Vulkan moderno com fallback; comunidade gigante; MIT = liberdade total p/ produto |
| WEAKNESSES | Engine de jogo, não de DCC/render profissional; rendering AAA limitado (VFX/geometry high-end); GDScript não escala para equipes grandes de perf crítica; console export requer licenças pagas |
| TRADE-OFFS | Produtividade/liberdade vs. teto gráfico. Para Cosca: forte candidato para **runtime 3D interativo / demos de IA / prototipagem de jogos** gerados por IA, com GDScript gerável |

**Princípio para o Cosca:** **Cenas como templates instanciáveis** (cenas-como-assets) é o modelo certo para geração procedural por IA: uma IA gera grafos de cena (scripts+cenas) que se compõem e reutilizam — a hierarquia de nodes é o *documento* editável, não código monolítico.

**Armadilha a evitar:** Assumir Godot como motor universal de render/editing profissional — seu teto é indie/mobile e o rendering é inferior a UE/Unity p/ AAA. Não escrever GDScript como se fosse a linguagem do produto: use-o como superfície, com lógica pesada em GDExtension/C# ou código gerado.

---

## Projeto: Bevy

| Campo | Valor |
|---|---|
| PROJECT | bevyengine/bevy |
| CATEGORY | Game — engine de jogo ECS em Rust |
| PURPOSE | Engine de jogo data-driven "simples" em Rust, baseada em Entity Component System |
| ARCHITECTURE | 100% Rust, workspace `crates/`: `bevy_ecs` (ECS core), `bevy_app` (App/Schedule), `bevy_render`/`bevy_wgpu` (sobre wgpu), `bevy_pbr`, `bevy_ui`, `bevy_asset`, `bevy_math`; **App = plugins; mundo = ECS; sistemas = funções**; render via wgpu (Vulkan/Metal/D3D/GL) |
| LANGUAGE | Rust (100%); MSRV ≈ stable recente |
| CORE ALGORITHMS | ECS scheduling (systems, parallel executor, ambiguity detection), archetype-based storage (SoA), Query filtering/indexing, change detection, component storage sparse/dense, asset loader pipeline (AssetServer), pipelined renderer |
| PIPELINES | App → plugins → schedule de sistemas → ECS world → query/sistemas → wgpu render; assets async via AssetServer → loaders → stores; parallelização automática de sistemas |
| DATA MODEL | **ECS puro**: Entities + Components (data) + Systems (logic); World = store; Resources (singletons); **sem scene tree** — a cena é dados em componentes, com hierarquia via `Parent/Child` opcional |
| GPU/CPU/MEMORY | **wgpu/Vulkan → compatível gfx1030/RDNA2 ✓** (herdado do wgpu); data-oriented: cache-friendly, parallelism automático; memória controlada por Rust |
| PERFORMANCE | Muito bom em CPU-heavy (data-oriented, multicore); render decente mas menos polido que engines maduras; compilação lenta (melhorada com config "fast compiles") |
| EXTENSIBILITY | Altíssima: tudo é plugin; feature flags Cargo; crates externos; API estável em evolução |
| PLUGIN SYSTEM | Plugins via trait `Plugin` (add_systems/add_plugins); cargo features; 3rd-party crates (bevy_ecs_ldtk etc.) |
| SECURITY | Rust memory-safe; asset loading de fontes não confiáveis é risco (não verificado sandbox); fuzz não tão maduro quanto outros |
| LICENSE | MIT OU Apache-2.0 (dual) |
| STRENGTHS | ECS puro = paralelismo e data-locality excelentes; tudo modular; wgpu por baixo (multi-GPU, ROCm ok); comunidade Rust gamedev ativa; segurança de Rust |
| WEAKNESSES | **Advertência oficial: "early stages, features missing, sparse docs, breaking changes a cada ~3 meses"**; sem editor visual maduro; curva para não-Rustistas; compilações lentas |
| TRADE-OFFS | Limpeza arquitetural/performance vs. maturidade e instabilidade de API. Para Cosca: se a base for Rust e a IA gerar sistemas ECS declarativamente, Bevy é excelente; mas pinar versão e preparar migração |

**Princípio para o Cosca:** **ECS (data-oriented) como modelo de runtime** separa dados de lógica de forma ideal para IA: a IA gera/edita *components e systems* (declarativos, testáveis) em vez de grafos imperativos; mudanças de comportamento = mudança de schedule de sistemas.

**Armadilha a evitar:** Usar Bevy em produção antes de amadurecer (o próprio README avisa: breaking releases trimestrais, features faltando). Não começar projeto crítico na versão não-pinada; e não forçar ECS em times sem experiência — o custo de onboard de arquitetura data-oriented é real.

---

## Projeto: Dear ImGui

| Campo | Valor |
|---|---|
| PROJECT | ocornut/imgui |
| CATEGORY | Game — GUI de modo imediato (immediate mode) |
| PURPOSE | Biblioteca GUI C++ "bloat-free" p/ ferramentas de conteúdo, debug e visualização (não p/ UI de usuário final) |
| ARCHITECTURE | Self-contained: `imgui*.cpp/h` no root (sem deps externas); API C++ pura; **renderer-agnostic** (emite vertex buffers + command lists); backends oficiais em `backends/` (GL, D3D9-12, Vulkan, Metal, SDL_GPU, WebGPU, SDL_Renderer); frame loop: Begin/Widgets/End |
| LANGUAGE | C++ (11+); bindings para ~20+ linguagens (C, C#, Rust, Python, etc.) |
| CORE ALGORITHMS | **Immediate mode UI**: regenera UI a cada frame (sem árvore de widgets retida); text layout/raster (stb_truetype), font atlas, text shaping básico (não completa i18n/RTL), batching de draw calls, clipping, tables/windows docking, **test engine** (automação) |
| PIPELINES | Código → ImGui::Begin/Widgets → vertex buffers + command lists → app renderiza via backend (Vulkan/D3D/etc.) → tela |
| DATA MODEL | Estado mínimo de UI (posição de janelas, foco); **sem modelo de dados de UI retido**; dados são refletidos direto do app (callbacks com ponteiros para vars) |
| GPU/CPU/MEMORY | Renderizado via backend do app (Vulkan/Metal/D3D/GL) — precisa de pipeline de triângulos texturizados; **Vulkan backend oficial ✓ (gfx1030 ok)**; uso de memória mínimo |
| PERFORMANCE | Excelente: batching de draw calls, baixo overhead por frame; não toca GPU diretamente |
| EXTENSIBILITY | Alta: custom widgets, backends custom, extensões (ImPlot, ImPlot3d, ImNodes, docking branch) |
| PLUGIN SYSTEM | Não é plugin runtime; extensões por código (backend/widget) — modelo "compile-time" |
| SECURITY | Sem i18n/RTL/accessibility (limitações declaradas); input handling simples; fuzz/CI; não é projetado para UI pública de produto |
| LICENSE | MIT |
| STRENGTHS | Iteração instantânea (perfeito p/ tools de conteúdo); renderer-agnostic (funciona em qualquer pipeline); footprint mínimo; padrão de facto da indústria de game tools |
| WEAKNESSES | Não é para UI de usuário final (falta i18n, accessibility, acessibilidade); estado de UI não-serializável de forma trivial; API C-style verbosa; longe de ser "design system" |
| TRADE-OFFS | Velocidade de desenvolvimento de tools vs. polimento de UX final. Para Cosca: **UI interna de ferramentas/editor e debug de pipelines IA** — ideal; para UI de produto, outra camada |

**Princípio para o Cosca:** Para ferramentas de conteúdo e debug, **immediate mode** vence por velocidade de iteração e reflete o estado dinâmico do dado em tempo real — usar Dear ImGui como UI de tooling interno (inspector de grafos de IA, viewport debug), não como UI de produto.

**Armadilha a evitar:** Deixar Dear ImGui vazar para a UI do usuário final (i18n/accessibility ausentes). Separar claramente "ferramentas" (ImGui) de "produto" (UI de plataforma própria). Cuidar também do sync estado app/UI (o paradigma imediato exige que o estado viva no app, não na UI).

---

## Projeto: SDL

| Campo | Valor |
|---|---|
| PROJECT | libsdl-org/SDL |
| CATEGORY | Game — camada de plataforma/mídia |
| PURPOSE | Camada cross-platform para mídia: janela, render 2D, input (teclado/mouse/controle), áudio, vídeo, HID — "Simple DirectMedia Layer" |
| ARCHITECTURE | C core (`src/`) com `include/` headers públicos; backends por plataforma: X11/Wayland, Windows (Win32), macOS (Cocoa), Android/iOS, consoles; **SDL_GPU API nova** (abstração GPU cross-platform 2D/3D); eventos unificados (SDL_Event); subsistemas: video, audio, joystick, sensor, filesystem, timer, haptics |
| LANGUAGE | C (C17-ish), bindings p/ muitas linguagens; SDL3 é a versão atual |
| CORE ALGORITHMS | Event loop/queue, window/swapchain management, input mapping (SDL_GameController API), mixer de áudio, renderer 2D (SDL_Renderer), **SDL_GPU** (render pass, pipelines, textures) |
| PIPELINES | App → SDL_Init → window → renderer/GPU → event loop (SDL_PollEvent) → present; áudio via SDL_Audio |
| DATA MODEL | Subsistemas singletons; janelas/surfaces/GPUs como handles; eventos estruturados (SDL_Event); controller mapping |
| GPU/CPU/MEMORY | SDL3 inclui **SDL_GPU (backends Vulkan/Metal/D3D12/OpenGL) — Vulkan ✓ gfx1030/RDNA2 ok**; CPU: input/audio/filesystem; memória mínima |
| PERFORMANCE | Overhead mínimo (camada fina); renderer 2D para casos simples; para 3D, app usa sua própria GPU API (SDL fornece só janela/surface p/ Vulkan/GL) |
| EXTENSIBILITY | Alta: backend por plataforma fácil de estender; API C estável; ecossistema de bindings enorme |
| PLUGIN SYSTEM | Não é plugin-based; extensibilidade por backends e API |
| SECURITY | Código C sem safety automática; parsing de formatos (SDL_image/mixer) é risco (CVEs históricos em libs auxiliares) |
| LICENSE | zlib (permissivo) |
| STRENGTHS | Padrão de facto para janelas/input/áudio cross-platform (usado por Godot, engines); SDL3 moderno (SDL_GPU); licença ultra-permissiva; suporte total a consoles |
| WEAKNESSES | É camada de plataforma, não engine; SDL_GPU é nova (menos maturo que wgpu p/ abstração completa); renderer 2D limitado; libs auxiliares (image/mixer) separadas |
| TRADE-OFFS | Simplicidade/portabilidade vs. escopo (sem render 3D próprio). Para Cosca: **janela + input + áudio + troca de superfície Vulkan** — essencial para app desktop e editor |

**Princípio para o Cosca:** Usar SDL como **fronteira de plataforma** (janelas, eventos, input unificado, áudio) e delegar render a uma camada GPU própria (wgpu/Vulkan) — separar "sistema operacional" (SDL) de "pipeline gráfico" (wgpu) mantém portabilidade e evita lock-in de uma lib monolítica.

**Armadilha a evitar:** Usar SDL_Renderer/SDL_GPU como se fossem engine de render completo — para viewport 3D e pipelines PBR eles são insuficientes. Não acoplar lógica de negócio a SDL (a API de eventos C é rígida); manter o core da plataforma agnóstico de UI e manter libs auxiliares (image/mixer) versionadas/sanitizadas por histórico de CVEs.

---

# SÍNTESE DA CATEGORIA — 3D/Graphics + Game

## 6 princípios fortes para o Cosca

1. **Uma camada gráfica segura, multi-backend.** wgpu (Rust) ou abstração equivalente sobre Vulkan/Metal/D3D/WebGPU renderiza uma vez e emite para todos os alvos — e é nativa na GPU ROCm/gfx1030 (Vulkan 1.3). Vulkan-Docs entra como *registry de verdade*, não como API de produto. (wgpu, Vulkan-Docs, SDL_GPU)
2. **Cena = dados declarativos, não árvore imperativa.** OpenUSD (arcs/composição) para *interchange* e cenas para IA; Blender DNA/RNA para persistência+reflexão; Godot cenas-como-templates para runtime interativo. A IA edita camadas/diffáveis e o runtime resolve. (OpenUSD, Blender, Godot)
3. **Asset pipeline em compile-time, não runtime.** Materiais compilados offline (Filament `matc`), shaders pré-validados, schema→geração de código (vk.xml→headers). Assets gerados por IA passam por compile com validação, cache e revisão. (Filament, Vulkan-Docs, Blender)
4. **Modelo canônico de dados na fronteira.** Todos os formatos de entrada normalizados para um schema único em memória (`aiScene`-like), com parsers isolados/sandbox (assimp). A plataforma conversa com o modelo neutro, não com 40 formatos. (assimp, Open3D, OpenUSD)
5. **Runtime data-oriented para lógica gerada por IA.** ECS puro (Bevy) ou scene-tree (Godot) — IA gera systems/components declarativos e paralelizáveis; comportamento = schedule, não código monolítico. (Bevy, Godot)
6. **Ferramentas vs. produto separados.** UI interna de tooling em immediate-mode (Dear ImGui) + plataforma via SDL + render via wgpu; produto final tem UI própria com i18n/accessibility. (ImGui, SDL, Filament)

## 6 armadilhas comuns a evitar

1. **Prometer GPU ROCm sem validar.** Open3D é CUDA-first (HIP/ROCm não confirmado); SKIA 2D é CPU/GPU-independente; Blender Cycles HIP ok. Validar *por projeto* o suporte a gfx1030 antes de prometer aceleração AMD. (Open3D)
2. **Tratar engines/apps como embutíveis.** Blender é GPL e acoplado a UI; OpenUSD é interchange, não runtime; Filament é render-only. Usar cada um na sua camada certa (externo vs. lib) evita reescrita. (Blender, OpenUSD, Filament)
3. **Confiar em API/ABI instável como fundamento.** wgpu/WebGPU (draft), Bevy (breaking a cada ~3 meses), SDL_GPU (novo): pinar versões, seguir changelogs, guardar migração. (wgpu, Bevy, SDL)
4. **Processar conteúdo não confiável no processo principal.** Parsers FBX/Collada (assimp), .blend, glTF, SDL_image/mixer têm histórico de CVEs. Sandbox/worker + preferir formatos seguros (glTF/USD) como primários. (assimp, Open3D, SDL, Blender)
5. **Criar modelos de dados duplicados.** DNA/RNA vs. aiScene vs. scene-graph de engine: se cada camada mantém seu próprio "schema canônico", drift e re-sync geram bugs (como a piada do ImGui sobre estado duplicado). Um registry único versionado resolve. (Blender, Vulkan-Docs, ImGui)
6. **Vazamento de camadas.** ImGui no produto final (sem i18n/accessibility); SDL como engine de render; Vulkan raw como API de produto — cada lib tem um papel; misturar aumenta custo e quebra portabilidade. (ImGui, SDL, Vulkan-Docs)

---
*Notas de verificação:* Stars coletadas dos repositórios em 2026-08-13 (ex.: Godot 115.7k, ImGui 75.6k, Bevy 47.6k, Filament 20.4k, Blender 19.7k, wgpu 17.8k, SDL 16.3k, Open3D 13.9k, assimp 13.1k, Skia 10.9k, OpenUSD 7.4k, Vulkan-Docs 3.3k) — não usadas como critério de qualidade. "Não verificado" marca itens sem confirmação nas fontes lidas. URL correto do USD: PixarAnimationStudios/OpenUSD.
