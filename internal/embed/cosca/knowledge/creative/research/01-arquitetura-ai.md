# Research Matrix — Arquitetura + IA

> Dossiê de Research Matrix para a plataforma Cosca (criação multimídia assistida por IA).
> Categoria: ARQUITETURA (4 projetos) e IA (5 projetos). Fontes: README/diagramas/licença de cada repo (GitHub), coletados em 2026-08-13.
> Regra de honestidade: campos não verificáveis estão marcados como **não verificado**.

---

## Projeto: CodeBoarding/awesome-architectures

> Nota: `github.com/CodeBoarding/awesome-architectures` redireciona para `CodeBoarding/awesome-architecture-mds` (~159★, 9 forks, 1.669 commits). Projeto pequeno e jovem.

| Campo | Valor |
|-------|-------|
| PROJECT | CodeBoarding/awesome-architectures (→ awesome-architecture-mds) |
| CATEGORY | ARQUITETURA (curadoria de diagramas de arquitetura) |
| PURPOSE | Atlas de diagramas de arquitetura de repos open-source populares, auto-gerados em markdown, "drop-in ready" para coding agents (injetáveis via ARCHITECTURE.md) |
| ARCHITECTURE | Pipeline agentico de análise estática com 6 subsistemas: (1) Orchestration & Lifecycle (CLI, discovery de projeto, registry/persistência), (2) Static Analysis Core (LSP + language adapters, semantic graph engine, clustering/validação), (3) Change Analysis (detecção incremental de mudanças), (4) AI Reasoning Layer (prompt factory + provider abstraction, agentic execution), (5) Health & Documentation (health metrics, geração de docs/diagramas), (6) Persistence & Tooling (DuckDB/SQLite, telemetria) |
| LANGUAGE | Python (inferido por tooling LSP/Node/agentic); **não verificado** explicitamente no README |
| CORE ALGORITHMS | Call-graph construction via LSP, community detection/clustering, análise incremental diff-aware com cache por hash de conteúdo, health metrics |
| PIPELINES | Scan → call graph → clustering → LLM reasoning → docs/diagramas; execução incremental (reusa resultados cacheados para reduzir tokens LLM) |
| DATA MODEL | Grafos (call graph + hierarquias), clusters, cache keyed por hash de arquivo, persistência DuckDB/SQLite |
| GPU/CPU/MEMORY | CPU-only; leve; persistência local embarcada (SQLite/DuckDB) |
| PERFORMANCE | Otimizado para reduzir tokens LLM (incremental + cache); sem benchmarks públicos → **não verificado** |
| EXTENSIBILITY | Registry dinâmico de tools/plugins carregados na inicialização; language adapters via LSP |
| PLUGIN SYSTEM | Sim (registry de tools/plugins) — mas a superfície de plugin é interna, **não verificado** como API pública |
| SECURITY | **não verificado** (análise local de código; nada documentado) |
| LICENSE | **não verificado** (não exibido na página capturada) |
| STRENGTHS | Formato de arquitetura pronto para consumo por agentes; cobertura ampla (dezenas de categorias/repos); incrementalismo para economizar tokens; atualização contínua (1.669 commits) |
| WEAKNESSES | Projeto imaturo (159★); 1 mantenedor/sem comunidade; diagramas auto-gerados podem ficar desatualizados ou imprecisos; pouco testes independentes |
| TRADE-OFFS | Geração automatizada vs. precisão manual; cache/incremental (economia de tokens) vs. latência de análise; amplitude (muitos repos) vs. profundidade por repo |
| LICENSE (resumo) | **não verificado** |

**Princípio para o Cosca:** Trate arquitetura como *artefato vivo* — mantenha um `ARCHITECTURE.md` canônico, atualizado por pipeline (não por esforço manual) e consumível por agentes, com análise incremental para conter custo de tokens.

**Armadilha a evitar:** Diagramas auto-gerados viram "ground truth" não verificado; documentação de arquitetura precisa de revisão humana e de gatilho de atualização (diff) — senão engana mais do que ajuda.

---

## Projeto: mehdihadeli/awesome-software-architecture

| Campo | Valor |
|-------|-------|
| PROJECT | mehdihadeli/awesome-software-architecture (11.6k★, 640 commits) |
| CATEGORY | ARQUITETURA (curadoria educacional) |
| PURPOSE | Lista curada de artigos/vídeos/recursos para aprender e praticar arquitetura de software, padrões e princípios; site oficial awesome-architecture.com |
| ARCHITECTURE | Repositório de documentação (markdown + mkdocs) organizado em taxonomia: Clean/Hexagonal/Onion/Vertical Slice, Event-Driven, DDD (com táticos/estratégicos), CQRS, Microservices (gateway, observabilidade, resiliência, segurança), Modular Monolith, princípios (SOLID, CAP, DRY, KISS, YAGNI), cloud patterns, design patterns |
| LANGUAGE | Documentação; exemplos com forte presença de .NET (Akka.NET, Orleans, MassTransit, NServiceBus, Dapr, Wolverine) |
| CORE ALGORITHMS | N/A (curadoria de conteúdo) |
| PIPELINES | N/A |
| DATA MODEL | N/A (estrutura de docs por tópico) |
| GPU/CPU/MEMORY | N/A |
| PERFORMANCE | N/A |
| EXTENSIBILITY | Contribuição aberta (CONTRIBUTING.md + mkdocs); muitas seções marcadas TODO |
| PLUGIN SYSTEM | N/A |
| SECURITY | Conteúdo cobre segurança em microservices (key vault, autenticação); a ferramenta em si é estática, sem superfície de ataque |
| LICENSE | CC0-1.0 |
| STRENGTHS | Cobertura muito ampla e bem organizada por tópico; excelente índice de navegação; atualização contínua |
| WEAKNESSES | Muitas descrições ainda "TODO" (qualidade desigual); curadoria majoritariamente de 1 autor (viés); conteúdo linkado externamente (rotatividade de URLs); não é validado por execução/experimento |
| TRADE-OFFS | Amplitude vs. profundidade; links externos (mantém repo leve) vs. estabilidade do conteúdo; atualização contínua vs. consistência editorial |
| LICENSE (resumo) | CC0-1.0 |

**Princípio para o Cosca:** Adote uma *taxonomia de arquitetura* compartilhada (bounded contexts, portas & adaptadores, event sourcing, CQRS) para que decisões de design sejam tomadas com vocabulário e critérios comuns — o vocabulário vale mais que qualquer padrão específico.

**Armadilha a evitar:** Padrões como DDD/CQRS/Event-Driven não são receitas universais; adotá-los sem contexto real gera complexidade acidental (anemic domain models, event-driven sem necessidade de eventos).

---

## Projeto: donnemartin/system-design-primer

| Campo | Valor |
|-------|-------|
| PROJECT | donnemartin/system-design-primer (364k★, 57.8k forks) |
| CATEGORY | ARQUITETURA (design de sistemas em escala) |
| PURPOSE | Aprender a projetar sistemas de larga escala; preparação para entrevistas de system design; coleção organizada de recursos com soluções completas |
| ARCHITECTURE | Documentação + 8 soluções de system design (pastebin, twitter timeline/search, web crawler, mint, social graph, query cache, sales rank, scaling on AWS) + exercícios de OO design; flashcards Anki (spaced repetition); traduções em 20+ idiomas; metodologia de entrevista em 4 passos (use cases → high-level design → core components → scale) |
| LANGUAGE | Markdown; exemplos de código em Python/JS/etc. nas soluções |
| CORE ALGORITHMS | CAP theorem (CP vs AP), consistência (weak/eventual/strong), availability patterns (failover active-passive/active-active, replication), caching (cache-aside, write-through, write-behind, refresh-ahead), sharding/consistent hashing, DNS/CDN/LB, back-of-the-envelope, latency numbers |
| PIPELINES | Metodologia de design em 4 passos (framework mental, não pipeline de software) |
| DATA MODEL | N/A; exemplos de schema relacional/NoSQL nos exercícios |
| GPU/CPU/MEMORY | N/A (trata vertical/horizontal scaling, sharding, replicação) |
| PERFORMANCE | Ensina distinções: performance vs. scalability, latency vs. throughput, availability em "n 9s" |
| EXTENSIBILITY | Contribuições e traduções abertas; conteúdo modular por seção |
| PLUGIN SYSTEM | N/A |
| SECURITY | Seção dedicada a segurança; DNS/SSL/auth abordados como tópicos de design |
| LICENSE | **não verificado** no README capturado (arquivo LICENSE.txt presente; reputa-se Creative Commons) |
| STRENGTHS | Referência canônica; soluções com discussão explícita de prós/contras e trade-offs; flashcards; multi-idioma; comunidade enorme |
| WEAKNESSES | Foco em entrevista → soluções simplificam problemas reais; conteúdo estático com atualização lenta; não é software executável/testável |
| TRADE-OFFS | Breadth vs. depth; "solução de referência" vs. múltiplas soluções igualmente válidas; abstração didática vs. complexidade real |
| LICENSE (resumo) | Creative Commons (variante **não verificada**) |

**Princípio para o Cosca:** *"Everything is a trade-off"* — para toda decisão de arquitetura da plataforma (consistência, cache, filas, particionamento) exija a documentação explícita do trade-off (CAP, latência vs. throughput, custo) antes de escolher.

**Armadilha a evitar:** Designs "ideais" de livro-texto (microservices, sharding, cache em camadas) viram over-engineering em produtos reais; comece com o design mínimo que atende a demanda atual e escale com evidência.

---

## Projeto: codecrafters-io/build-your-own-x

| Campo | Valor |
|-------|-------|
| PROJECT | codecrafters-io/build-your-own-x (539k★, 50.9k forks) |
| CATEGORY | ARQUITETURA (aprendizado por reconstrução) |
| PURPOSE | Compilação de guias passo-a-passo para recriar tecnologias do zero (3D renderers, LLMs, databases, Docker, emuladores, compiladores, shells, web servers, OS, redes, etc.) — "What I cannot create, I do not understand" (Feynman) |
| ARCHITECTURE | Repositório de curadoria de tutoriais organizado por tecnologia (30+ categorias); links externos de alta qualidade por categoria; também direciona para o produto comercial CodeCrafters |
| LANGUAGE | Multi-linguagem: C, C++, Go, Rust, Python, JavaScript/TypeScript, Ruby, Java, Haskell, etc. |
| CORE ALGORITHMS | N/A (tutoriais cobrem: B+tree/SQL, TCP/IP stack, regex NFA/DFA, KV store estilo Redis, garbage collectors, interpreters/compilers, ray tracing, quantização de LLM) |
| PIPELINES | N/A |
| DATA MODEL | N/A |
| GPU/CPU/MEMORY | N/A (tutoriais incluem renderers GPU e neural nets, mas é conteúdo, não software) |
| PERFORMANCE | N/A |
| EXTENSIBILITY | Contribuições abertas (novas categorias/links); ISSUE_TEMPLATE |
| PLUGIN SYSTEM | N/A |
| SECURITY | N/A (conteúdo educacional estático) |
| LICENSE | **não verificado** (não exibido na página capturada) |
| STRENGTHS | Estratégia pedagógica comprovada (construir para entender); curadoria de links excelentes; cobertura gigante e organizada |
| WEAKNESSES | Não é projeto de software (é uma lista); links externos podem quebrar; profundidade depende do tutorial individual; sem garantia de qualidade uniforme entre entradas |
| TRADE-OFFS | Tutoriais "toy" vs. sistemas de produção; construir do zero (aprendizado profundo) vs. reutilizar libs (velocidade); manter repo pequeno (links) vs. conter o conteúdo (inline) |
| LICENSE (resumo) | **não verificado** |

**Princípio para o Cosca:** Para cada componente crítico da plataforma (ex.: pipeline de geração, orquestração de jobs), crie *protótipos "from scratch"* como exercício de domínio da equipe — o entendimento profundo que nasce disso melhora a arquitetura do produto real.

**Armadilha a evitar:** Not-invented-here: aprenda construindo o protótipo, mas sirva o produto com componentes maduros (PyTorch, vLLM, etc.); não transforme o protótipo educacional em produção sem forte justificativa.

---

## Projeto: huggingface/transformers

| Campo | Valor |
|-------|-------|
| PROJECT | huggingface/transformers (164k★, 23.6k commits) |
| CATEGORY | IA (framework de modelos pré-treinados) |
| PURPOSE | Framework de *definição de modelos* state-of-the-art para texto, visão, áudio, vídeo e multimodal — inferência e treinamento; é o "pivô" entre frameworks e engines do ecossistema |
| ARCHITECTURE | Biblioteca Python com poucas abstrações públicas: `pipeline()` (alta) + `AutoModel`/`Tokenizer`/`Processor`. Modelos como arquivos autocontidos (config + safetensors). Integração nativa com Hub (1M+ checkpoints). Suporte PyTorch/JAX/TF. `transformers serve` (servidor OpenAI-compatível). Por design, NÃO refatora cada arquitetura em abstrações (iteração rápida de pesquisa) |
| LANGUAGE | Python (núcleo), C++/CUDA via PyTorch, JAX/TF |
| CORE ALGORITHMS | Arquiteturas Transformer (attention, MoE, multimodal encoders), tokenização (BPE, WordPiece, SentencePiece), pipelines por task (text-generation, ASR, image-classification, VQA), safetensors |
| PIPELINES | `pipeline(task=...)`: preprocess → model → postprocess; treino via Trainer (otimizado para PyTorch); chat via pipeline com histórico |
| DATA MODEL | Checkpoints serializados (config.json + safetensors/bin); 1M+ checkpoints no Hub; conversão de pesos entre frameworks |
| GPU/CPU/MEMORY | dtype bf16/fp16, `device_map="auto"` (offload multi-dispositivo), CPU-first suportado, quantização via integrações |
| PERFORMANCE | Prioriza compatibilidade/simplicidade; para throughput produtivo delega a vLLM/SGLang/TGI/llama.cpp; `transformers serve` oferece decodificação especulativa |
| EXTENSIBILITY | Nova arquitetura = novo arquivo de model autocontido (+ `add_custom_model`); exemplos de reprodução por arquitetura; compatível com a maioria dos training frameworks (DeepSpeed, FSDP, Unsloth...) |
| PLUGIN SYSTEM | Não plugin-based; extensão por arquivos de modelo e custom code (via `trust_remote_code`) |
| SECURITY | SECURITY.md; políticas de Hub p/ checkpoints; `trust_remote_code` como superfície de risco |
| LICENSE | Apache-2.0 |
| STRENGTHS | Padrão de facto do ecossistema; interoperabilidade total (treinamento + inferência + vizinhos); documentação extensa; API unificada simples; enorme comunidade |
| WEAKNESSES | Deliberadamente não-modular (duplicação de código entre arquiteturas); pacote grande/pesado; exemplos podem não rodar out-of-the-box; API evolui com quebras (MIGRATION_GUIDE_V5) |
| TRADE-OFFS | Simplicidade de API vs. modularidade interna; cobertura de modelos vs. manutenção; ser o "definidor canônico" vs. não otimizar a execução (delega a engines) |
| LICENSE (resumo) | Apache-2.0 |

**Princípio para o Cosca:** *Centralize a definição, não a execução* — defina contratos canônicos de modelos/tasks (entrada/saída/checkpoint) como espinha dorsal do Cosca, e mantenha os engines de execução plugáveis atrás de uma camada própria.

**Armadilha a evitar:** Não importe toda a biblioteca para um único modelo; isole o uso em camada própria com versões pinadas, porque o tamanho e a volatilidade de API de `transformers` geram dívida técnica se vazarem por todo o código.

---

## Projeto: huggingface/diffusers

| Campo | Valor |
|-------|-------|
| PROJECT | huggingface/diffusers (34.3k★, 6.8k commits) |
| CATEGORY | IA (geração por difusão — imagem/vídeo/áudio/3D) |
| PURPOSE | Biblioteca state-of-the-art de modelos de difusão pré-treinados para geração de imagens, áudio, vídeo e moléculas 3D; toolbox modular para inferência e treinamento |
| ARCHITECTURE | Três componentes centrais: (1) diffusion pipelines (alto nível, prontas p/ inferência), (2) noise schedulers intercambiáveis (velocidade/qualidade), (3) modelos pré-treinados como building blocks combináveis. Filosofia explícita: "usability over performance", "simple over easy", "customizability over abstractions". Baseado em PyTorch |
| LANGUAGE | Python (PyTorch); integrações com keras, CompVis, etc. |
| CORE ALGORITHMS | Denoising diffusion (DDPM, DDIM), score-based, latent diffusion (VAE + UNet), schedulers (DPMSolver, Euler, LMS), text-to-image, img2img, inpainting, ControlNet, InstructPix2Pix, upscaling, unCLIP |
| PIPELINES | Loop de difusão explícito: `scheduler.set_timesteps` → ruído → `model(input, t)` → `scheduler.step` → decode VAE; ou `DiffusionPipeline.from_pretrained(...)` |
| DATA MODEL | Checkpoints do Hub (30k+); configs separadas para pipeline/scheduler/modelo |
| GPU/CPU/MEMORY | CUDA + float16, Apple Silicon (MPS); otimizações fp16 e offload; guias de memória/latência |
| PERFORMANCE | Prioriza usabilidade (filosofia declarada); benchmarks existem, mas runtimes dedicados são mais rápidos |
| EXTENSIBILITY | Componentes intercambiáveis (scheduler/model/pipeline); criar pipeline própria é cidadão de primeira classe; contribuição de novos modelos/pipelines/schedulers |
| PLUGIN SYSTEM | Não plugin-based; extensão por composição de componentes e novas pipelines |
| SECURITY | SECURITY.md; atenção a conteúdo nocivo em modelos generativos; filtros em alguns pipelines |
| LICENSE | Apache-2.0 |
| STRENGTHS | Componentização limpa e pedagogia ótima (pipelines vs. toolbox); cobertura dos modelos modernos (SD, Flux, video, audio); customização incentivada |
| WEAKNESSES | Performance abaixo de runtimes especializados; superfície de API extensa e mutável; dependência de PyTorch |
| TRADE-OFFS | Usabilidade/customização vs. performance; generalidade (multi-modalidade) vs. foco; abstrações mínimas vs. consistência de API |
| LICENSE (resumo) | Apache-2.0 |

**Princípio para o Cosca:** Separe *intenção* (pipeline de alto nível: "gerar imagem a partir de prompt") de *mecânica* (schedulers/modelos plugáveis) — no Cosca, a camada de geração deve ser declarativa e desacoplada do backend de difusão.

**Armadilha a evitar:** Pipelines prontas são rígidas e sedutoras; para fluxos de criação multimídia customizados, compose modelos+schedulers atrás de uma interface própria — e não engesse o produto em pipelines prontas que mudam entre versões.

---

## Projeto: pytorch/pytorch

| Campo | Valor |
|-------|-------|
| PROJECT | pytorch/pytorch (102k★, 109k commits) |
| CATEGORY | IA (framework de deep learning) |
| PURPOSE | Biblioteca Python de tensores com forte aceleração GPU e redes neurais profundas baseadas em autograd de tape; imperativo, Python-first |
| ARCHITECTURE | Componentes: `torch` (tensor), `torch.autograd` (diferenciação automática reverse-mode em tempo real), `torch.jit` (TorchScript), `torch.nn`, `torch.multiprocessing` (memória compartilhada), `torch.utils` (DataLoader). Núcleo C++ (aten, c10) + dispatcher de ops; `torch.compile` (trace → Inductor codegen + Triton); backends CUDA/ROCm/Intel XPU/MPS |
| LANGUAGE | Python (frontend), C++ (núcleo), CUDA/HIP kernels |
| CORE ALGORITHMS | Reverse-mode autograd (tape), operações de tensor, convolução via cuDNN/MKL, comunicação distribuída NCCL, TorchScript, torch.compile (graph tracing/lowering/codegen), memory allocators GPU custom |
| PIPELINES | DataLoader pipeline (multiprocessing + shm); training loop; compile pipeline (trace → fusions → codegen) |
| DATA MODEL | Tensores (compartilháveis via shared memory), `nn.Module` e estado, checkpoints serializados |
| GPU/CPU/MEMORY | CUDA/ROCm/Intel/Apple; alocadores de memória GPU custom (eficiência de VRAM); MKL; shared memory para data loading |
| PERFORMANCE | Focado em performance com flexibilidade; integra cuDNN/NCCL/MKL; CI contínuo (hud.pytorch.org); compile para produção |
| EXTENSIBILITY | Extensões em C/C++ sem boilerplate (API de custom ops); camadas em Python puro; registro de ops no dispatcher |
| PLUGIN SYSTEM | Não plugin-based; extensão via custom ops/modules (dispatcher de ops) |
| SECURITY | SECURITY.md; AI_POLICY.md (política de uso de IA no desenvolvimento) |
| LICENSE | BSD-3-Clause (reputado; README capturado não exibiu o texto) |
| STRENGTHS | Dominância no ecossistema; flexibilidade imperativa; extensões simples; ecossistema de bibliotecas imenso; custom allocators e compile para produção |
| WEAKNESSES | Complexidade/superfície enorme; build e footprint grandes; compatibilidade de versões (CUDA, glibc) é trabalho contínuo; sobrecarga cognitiva da API vasta |
| TRADE-OFFS | Dinamicidade/imperatividade vs. otimização estática; Python-first vs. performance máxima (compile); interoperabilidade ampla vs. controle fino |
| LICENSE (resumo) | BSD-3-Clause |

**Princípio para o Cosca:** *Eager por padrão, compile quando precisar* — separe camada de pesquisa/prototipagem (flexível, imperativa) da camada de produção (compilada/otimizada, com kernels testados); não otimize o caminho de dev antes de existir evidência de bottleneck.

**Armadilha a evitar:** Acoplar o produto inteiro a um framework monolítico sem camada de abstração própria; o custo de atualizações profundas (CUDA, kernels, versões) escala e trava a plataforma com o tempo.

---

## Projeto: vllm-project/vllm

| Campo | Valor |
|-------|-------|
| PROJECT | vllm-project/vllm (89k★, 20k commits; originário do Sky Computing Lab, UC Berkeley) |
| CATEGORY | IA (serving/inferência de LLMs) |
| PURPOSE | Engine de inferência e serving de LLMs com alto throughput e eficiência de memória; API compatível com OpenAI |
| ARCHITECTURE | PagedAttention (gestão de memória KV em blocos paginados) + continuous batching + chunked prefill + prefix caching + CUDA/HIP graphs + quantização (FP8, MXFP, INT8/4, GPTQ/AWQ, GGUF, compressed-tensors...) + kernels otimizados (FlashAttention, FlashInfer, Triton) + speculative decoding (EAGLE, n-gram) + parallelismo (tensor/pipeline/data/expert/context) + prefill/decode/encode disgregados. Servidor OpenAI-compatible + Anthropic API + gRPC. Python (orquestração) + C++/CUDA (kernels) + Rust (componentes recentes) |
| LANGUAGE | Python, C++/CUDA, Rust, Triton |
| CORE ALGORITHMS | PagedAttention, continuous batching, chunked prefill, prefix caching, speculative decoding, MoE expert parallelism, KV cache management, quantização |
| PIPELINES | Request scheduling → prefill → decode (streaming); fases prefill/decode/encode desagregadas; multi-LoRA |
| DATA MODEL | KV cache paginado (blocks), requisições em batch dinâmico, adapters LoRA |
| GPU/CPU/MEMORY | NVIDIA/AMD/Intel GPUs + CPUs x86/ARM/PPC + plugins (TPU, Gaudi, Ascend, Apple Silicon...); eficiência de VRAM via PagedAttention + quantização |
| PERFORMANCE | Throughput de serving state-of-the-art (benchmarks documentados); contrabalança com latência (batching) |
| EXTENSIBILITY | 200+ arquiteturas de modelo; plugins de hardware (VLLM_PLUGINS); kernels plugáveis |
| PLUGIN SYSTEM | Sim — plugins de hardware/backend e registro de novas arquiteturas de modelo |
| SECURITY | SECURITY.md; DCO; security advisories; 62 findings de segurança reportados (superfície ativa) |
| LICENSE | Apache-2.0 |
| STRENGTHS | Throughput líder; memória eficiente (PagedAttention); compatibilidade ampla (hardware e modelos); API OpenAI-compatível; comunidade de 2000+ contribuidores |
| WEAKNESSES | Alta complexidade; requer GPUs com VRAM adequada p/ modelos grandes; kernels especializados exigem manutenção; curva de configuração |
| TRADE-OFFS | Throughput/batching vs. latência individual (p99); compatibilidade de hardware vs. kernels especializados; flexibilidade de plugins vs. estabilidade |
| LICENSE (resumo) | Apache-2.0 |

**Princípio para o Cosca:** *Memória como recurso de primeira classe* — aloque recursos de inferência por blocos/páginas (como PagedAttention), não por requisição; qualquer pipeline de IA server-side do Cosca deve ser desenhado com gestão explícita de KV/memória e batching contínuo.

**Armadilha a evitar:** Otimizações profundas de serving acoplam o sistema a kernels/hardware específicos e exigem manutenção contínua; adote um engine maduro (vLLM) só acima de um volume real — para prototipagem, engines simples bastam e custam menos.

---

## Projeto: ggerganov/llama.cpp

> Nota: `github.com/ggerganov/llama.cpp` redireciona para `ggml-org/llama.cpp` (123.8k★, MIT).

| Campo | Valor |
|-------|-------|
| PROJECT | ggerganov/llama.cpp (→ ggml-org/llama.cpp) |
| CATEGORY | IA (inferência local/edge) |
| PURPOSE | Inferência de LLM/VLM em C/C++ puro, sem dependências, com performance state-of-the-art em hardware amplo — localmente e na nuvem |
| ARCHITECTURE | Construído sobre ggml (biblioteca de tensores). Núcleo C/C++ + backends de hardware plugáveis (BLAS/BLIS, CUDA, HIP, Metal, Vulkan, SYCL, OpenCL, CANN, MUSA, WebGPU, RPC...). Quantização inteira (1.5-bit a 8-bit). Inferência híbrida CPU+GPU (modelos maiores que a VRAM). Ferramentas: `llama cli`, `llama serve` (OpenAI-compatível), GBNF grammars (saída estruturada), built-in web UI. Formato de modelo: GGUF |
| LANGUAGE | C/C++ (núcleo + kernels CUDA/Metal/Vulkan), Python (conversão de modelos — gguf-py) |
| CORE ALGORITHMS | Operações de tensor (ggml), quantização inteira (IQ/Q), kernels de atenção custom, constrained decoding via grammars GBNF, CPU+GPU offload, memory mapping (mmap) |
| PIPELINES | Conversão HF→GGUF → carregamento (mmap) → prefill/decode → (opcional) geração estruturada por grammar |
| DATA MODEL | GGUF (formato de modelo quantizado; 1.5–8 bit), vocab, embeddings |
| GPU/CPU/MEMORY | Primeira classe: Apple Silicon (NEON/Accelerate/Metal), x86 (AVX/AVX2/AVX512/AMX), RISC-V (RVV); NVIDIA (CUDA), AMD (HIP), Vulkan/SYCL/WebGPU; hybrid CPU+GPU |
| PERFORMANCE | SOTA em local/edge; roda em laptop/desktop; quantização reduz VRAM/RAM drasticamente |
| EXTENSIBILITY | Backends de hardware compilados condicionalmente; suporte contínuo a novas arquiteturas de modelo |
| PLUGIN SYSTEM | Backends de hardware (em tempo de compilação), não plugin runtime |
| SECURITY | SECURITY.md; risco de carregar GGUF de fontes não confiáveis |
| LICENSE | MIT |
| STRENGTHS | Zero dependências; portabilidade máxima (qualquer hardware); quantização eficiente; CPU-first sem sacrificar performance; MIT; comunidade enorme |
| WEAKNESSES | Foco em inferência (treinamento limitado); ecossistema GGUF próprio (conversão necessária); paridade incompleta entre arquiteturas; kernels por hardware exigem manutenção |
| TRADE-OFFS | Portabilidade/simplicidade vs. features completas de serving em escala (isso fica com vLLM); quantização (memória) vs. qualidade de saída; edge/CPU vs. data center/GPU |
| LICENSE (resumo) | MIT |

**Princípio para o Cosca:** *Edge-first quando fizer sentido* — ofereça modo de inferência local (privacidade, latência, custo zero de API) com fallback transparente para cloud; a portabilidade "sem dependências" de llama.cpp mostra que IA local é viável como camada da plataforma.

**Armadilha a evitar:** Quantização agressiva degrada qualidade *silenciosamente*, e o formato próprio GGUF cria lock-in de conversão; o Cosca deve testar/calibrar qualidade dos modelos quantizados antes de servir e manter o caminho de conversão como pipeline automatizado.

---

# SÍNTESE DA CATEGORIA

## 5 princípios mais fortes para o Cosca

1. **Tudo é um trade-off** (system-design-primer) — decida com documentação explícita de trade-offs (CAP, latência vs. throughput, custo), não por modismo.
2. **Centralize a definição, não a execução** (transformers) — contratos canônicos de modelos/tasks como espinha dorsal; engines plugáveis atrás de camada própria.
3. **Separe intenção de mecânica** (diffusers) — camada declarativa de geração desacoplada de schedulers/modelos/backends.
4. **Eager por padrão, compile quando precisar** (pytorch) — pesquisa flexível, produção otimizada; otimize com evidência.
5. **Memória como recurso de primeira classe + modo edge** (vLLM/llama.cpp) — gestão explícita de KV/memória no serving e oferecer inferência local com fallback cloud.

## 5 armadilhas mais comuns a evitar

1. **Over-engineering prematuro** — adotar microservices/DDD/CQRS/sharding sem necessidade real (system-design-primer, awesome-software-architecture).
2. **Not-invented-here** — transformar protótipo "from scratch" em produção, reinventando componentes maduros (build-your-own-x).
3. **Acoplamento a framework monolítico** — importar bibliotecas gigantes sem camada de abstração própria → dívida de API/versões (transformers, pytorch).
4. **Lock-in de formato/runtime + degradação silenciosa** — formatos próprios (GGUF) e quantização que corroem qualidade sem alerta (llama.cpp).
5. **"Ground truth" não verificado** — diagramas/docs de arquitetura auto-gerados e padrões de referência tratados como verdade absoluta sem revisão humana (CodeBoarding, system-design-primer).

---

## Notas metodológicas

- Fontes: páginas dos repositórios no GitHub (README, estrutura de arquivos, badges de licença/CI, stats de stars/commits). Diagrama arquitetural do CodeBoarding extraído do próprio README.
- Campos marcados "não verificado": licenças não exibidas no HTML capturado (CodeBoarding, codecrafters), detalhe de licença do system-design-primer, linguagem do CodeBoarding, e qualquer métrica de performance sem benchmark público.
- Os números de stars/forks/commits são os exibidos nas páginas capturadas em 2026-08-13 e servem apenas como proxy de adoção, não de qualidade (conforme regra da tarefa).
