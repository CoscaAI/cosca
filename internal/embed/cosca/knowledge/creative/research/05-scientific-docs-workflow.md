# Research Matrix — Científico + Documentos + Workflow + DevTools

> Dossiê de Research Matrix do Cosca. 21 projetos avaliados por **maturidade, arquitetura e qualidade** (não apenas por estrelas). Campos marcados com * (asterisco) = **não verificado** diretamente no README; conhecimento geral do domínio, requer checagem antes de decisão de arquitetura.

---

## Projeto: NumPy

| CAMPO | VALOR |
|---|---|
| PROJECT | numpy/numpy |
| CATEGORY | Científico |
| PURPOSE | Pacote fundamental de computação científica em Python: arrays N-dimensionais, broadcasting, álgebra linear, FFT, RNG |
| ARCHITECTURE | Python (front-end/API) + núcleo compilado em C/C++/Fortran; build Meson; protocolo de array (`__array__`, buffers) |
| LANGUAGE | Python + C/C++ |
| CORE ALGORITHMS | Broadcasting de arrays; vistas *strided* (sem cópia); reduções SIMD; BLAS/LAPACK para linalg; geradores de RNG (PCG64) |
| PIPELINES | Eager, operação-a-operacão; sem grafo lazy nem DAG; suporte a `memmap` para dados > RAM (não streaming nativo) |
| DATA MODEL | `ndarray` contíguo (row-major) com `dtype`, shape e strides; sem tipo de coluna/tabela |
| GPU/CPU/MEMORY | CPU-first; sem GPU nativa (aceleração via libs externas); footprint de memória alto para dados heterogêneos/objetos |
| PERFORMANCE | ~velocidade de C nativo em operações vetorizadas; overhead de Python por operação (op-fusion só via compiladores externos); GIL em threads |
| EXTENSIBILITY | C API estável; Cython; ufuncs customizados; protocolo de array/DFT |
| PLUGIN SYSTEM | Não é sistema de plugins; extensão via C API / Cython / duck-typing de protocolos |
| SECURITY | Governança NumFOCUS, health score LFX, política de vulnerabilidades via Tidelift; exposição de buffer intencional (ponteiro bruto) |
| LICENSE | BSD-3-Clause |
| STRENGTHS | Padrão de facto do ecossistema científico; maturidade ~30 anos; ABI/API extremamente estável; interoperabilidade universal |
| WEAKNESSES | Eager e sem otimização de consulta; não adequado a DataFrames/labels; memória não-eficiente para strings/objetos; GIL limita threading |
| TRADE-OFFS | Simplicidade/estabilidade universal ⇄ velocidade e expressividade de query; núcleo imutável em C custa lento ciclo de evolução (mudanças levam anos) |

**Princípio para o Cosca:** Camadas de baixo nível devem ter um **núcleo estável e imutável** com contrato de ABI/API explícito, porque o ecossistema inteiro constrói sobre ele — o Cosca deve tratar seu formato de dados e API de extensão como "NumPy" (contrato que não muda).
**Armadilha a evitar:** Tratar array como solução para tabelas/dados rotulados — aplicar NumPy onde Polars/Arrow resolveria custa memória e expressividade. Não usar NumPy para tudo só por ser ubíquo.

---

## Projeto: SciPy

| CAMPO | VALOR |
|---|---|
| PROJECT | scipy/scipy |
| CATEGORY | Científico |
| PURPOSE | Biblioteca de matemática/ciência/engenharia: estatística, otimização, integração, álgebra linear, FFT, processamento de sinais/imagens, solvers de EDO |
| ARCHITECTURE | Monorepo modular sobre NumPy; subpacotes (`optimize`, `stats`, `signal`, `integrate`…); partes em C/C++/Fortran compiladas (meson) |
| LANGUAGE | Python + C/C++/Fortran |
| CORE ALGORITHMS | Otimização (L-BFGS-B, SLSQP, MINPACK), integração numérica, solvers ODE (LSODA, RK45), solvers esparsos (SuperLU/UMFPACK), estatística com distribuições |
| PIPELINES | Funções de nível de algoritmo; o usuário compõe; não há grafo de computação próprio |
| DATA MODEL | Consome `ndarray` NumPy; modelos estatísticos como objetos com métodos |
| GPU/CPU/MEMORY | CPU; acelera via BLAS/LAPACK (MKL/OpenBLAS); sem GPU nativa |
| PERFORMANCE | Competitivo com FORTRAN/BLAS para linalg; algumas rotinas (código Python puro) são lentas; qualidade de performance varia por subpacote |
| EXTENSIBILITY | Contribuidores podem estender; integração com Cython/C; sem plugin runtime |
| PLUGIN SYSTEM | Não tem; extensão por subpacote de terceiros (SciPy ecosystem) |
| SECURITY | NumFOCUS, LFX health score, relatórios via Tidelift |
| LICENSE | BSD-3-Clause |
| STRENGTHS | Amplitude imbatível (quase tudo em um lugar); maturidade; integração nativa com NumPy |
| WEAKNESSES | Monolito: instalação pesada, API inconsistente entre subpacotes; algumas rotinas desatualizadas |
| TRADE-OFFS | Um pacote "tudo-em-um" ⇄ granularidade e modernidade; conveniência ⇄ controle fino de implementação |

**Princípio para o Cosca:** Para algoritmos científicos, **use a melhor implementação testada (biblioteca madura) em vez de reimplementar**; a diferença de valor do Cosca está na orquestração e na IA, não no solver numérico.
**Armadilha a evitar:** Depender de SciPy inteiro (monolito pesado) para uma única função — use `scipy.optimize`/`scipy.signal` seletivamente e considere alternativas focadas para reduzir custo de deploy.

---

## Projeto: pandas

| CAMPO | VALOR |
|---|---|
| PROJECT | pandas-dev/pandas |
| CATEGORY | Científico |
| PURPOSE | Manipulação/análise de dados rotulados: DataFrames, groupby (split-apply-combine), alinhamento por labels, séries temporais, missing data |
| ARCHITECTURE | Python + núcleo Cython/C (migrando para Meson); estruturas baseadas em blocos de arrays NumPy (`BlockManager`); copy-on-write |
| LANGUAGE | Python + Cython/C |
| CORE ALGORITHMS | Split-apply-combine; alinhamento por índice; joins por hash; resampling temporal; pivoting/reshaping; fstrings de engine de string |
| PIPELINES | Eager; sem otimizador de consulta; sem streaming nativo (dados em memória) |
| DATA MODEL | `DataFrame`/`Series` com `Index` (MultiIndex hierárquico); colunas como blocos NumPy (não columnar puro); dtype `object` para strings |
| GPU/CPU/MEMORY | CPU; uso de memória alto (cópias, colunas objeto); suporte GPU via backends externos (cuDF, Modin) |
| PERFORMANCE | Rápido em dados moderados; superado por Polars/duckdb em datasets grandes; overhead de cópia histórico (melhorado por copy-on-write); GIL |
| EXTENSIBILITY | ExtensionArrays customizados; acesso de métodos e operadores rico; integra com bibliotecas de ML |
| PLUGIN SYSTEM | `pandas.api.extensions` (ExtensionArray, accessors); registros via entry points; sem "plugin marketplace" |
| SECURITY | NumFOCUS, LFX health score, CVE handling maduro (pandas tinha CVEs de pickle/parquet em versões antigas) |
| LICENSE | BSD-3-Clause |
| STRENGTHS | API mais familiar do planeta para análise de dados; ecossistema gigantesco; documentos/regressões massivas; estabilidade |
| WEAKNESSES | Não-columnar puro; consumo de memória; eager e sem otimização; GIL; performance inferior a polars/arrow em escala |
| TRADE-OFFS | Familiaridade/ecossistema ⇄ performance e eficiência de memória; flexibilidade ⇄ consistência interna |

**Princípio para o Cosca:** **API estável e familiar vale mais que performance marginal** — mas reserve pandas para interação/interfaces e use engine columnar (Polars/Arrow) por baixo para carga pesada.
**Armadilha a evitar:** Construir o pipeline de dados do Cosca sobre pandas como engine — vai doer em escala e em memória; não cometer "cópia-de-DataFrame" (chained indexing/views).

---

## Projeto: Polars ⭐ FOCO

| CAMPO | VALOR |
|---|---|
| PROJECT | pola-rs/polars |
| CATEGORY | Científico |
| PURPOSE | Query engine de DataFrames de alta performance, escrito do zero em Rust |
| ARCHITECTURE | Columnar sobre **Apache Arrow** (zero-copy); motor **lazy** com otimização de consulta (predicate/projection pushdown, alinhamento de joins); **streaming engine** para dados > RAM; paralelismo multithread (Rayon) + SIMD |
| LANGUAGE | Rust (núcleo) + bindings Python/Node/R/SQL |
| CORE ALGORITHMS | Execução vetorizada/SIMD; pushdown de predicados e projeções; engine streaming out-of-core; hashing/joins eficientes; expressões compostas (não dataframes intermediários) |
| PIPELINES | Eager e Lazy (`scan_*.parquet`.filter.group_by.agg.sort.collect); coletar em streaming; otimizador embutido |
| DATA MODEL | Arrow columnar: arrays tipados densos, null bitmaps, zero-copy entre sistemas; dados ordenados por coluna (column-major) |
| GPU/CPU/MEMORY | CPU multi-core + SIMD por padrão; **GPU NVIDIA opcional**; streaming reduz pico de RAM (processa 250GB em laptop *) |
| PERFORMANCE | Entre os melhores em benchmarks PDS-H; largamente superior a pandas em join/groupby/agregação em escala |
| EXTENSIBILITY | **Plugins de I/O e Expressão** (extender Polars nativamente via plugins); Interop Arrow zero-copy; múltiplas linguagens |
| PLUGIN SYSTEM | Plugin system nativo (Rust) para custom expressions/I/O; sem marketplace central |
| SECURITY | Política de segurança documentada (SECURITY.md), dependências escaneadas (deny.toml); maduro o suficiente para produção |
| LICENSE | MIT |
| STRENGTHS | Performance + memory efficiency; lazy optimizer; streaming out-of-core; multithreading automático; expressividade de expressões |
| WEAKNESSES | Ecossistema menor que pandas (menos integrações); modelo Arrow exige tipos/regras específicas; alguns edge cases de compatibilidade; API muda mais rápido |
| TRADE-OFFS | Velocidade/eficiência ⇄ ecossistema/maturidade de API; columnar puro ⇄ flexibilidade de "qualquer coisa em DataFrame" |

**Princípio para o Cosca:** **Dados estruturados de alto volume devem viver em formato columnar Arrow com engine lazy otimizado** — pushdown de filtros e projeções (ler só o que precisa) + streaming out-of-core são o padrão ouro para qualquer pipeline de dados de mídia/IA.
**Armadilha a evitar:** Não ignorar o lazy optimizer: materializar DataFrames intermediários destrói o ganho; e não assumir compatibilidade total com pandas API — os idioms diferem e código que "parece certo" pode ser semanticamente diferente.

---

## Projeto: JAX

| CAMPO | VALOR |
|---|---|
| PROJECT | jax-ml/jax |
| CATEGORY | Científico |
| PURPOSE | Computação numérica transformável: grad, jit, vmap compostos; aceleração GPU/TPU via XLA; sharding para clusters |
| ARCHITECTURE | Python (tracing) + XLA (compilador OpenXLA); API funcional (funções puras); `jaxlib` núcleo nativo; plugin de backends |
| LANGUAGE | Python + C++ (jaxlib/XLA) |
| CORE ALGORITHMS | Autodiff reverse/forward (componíveis); JIT por tracing e compilação XLA; vmap (vectorization pushdown); sharding/mesh partition |
| PIPELINES | Funções puras traçadas → HLO → executável acelerador; paralelismo automático/explícito/manual |
| DATA MODEL | Arrays (jnp) imutáveis; PyTrees (estruturas aninhadas); shardings como tipo (ex. `f32[512@data,512]`) |
| GPU/CPU/MEMORY | GPU (NVIDIA/AMD/Apple*) e TPU-first; CPU completo; memória gerenciada por buffers XLA (não Python GC) |
| PERFORMANCE | Muito rápido quando compilado; vmap/grad fusion; porém **custo de compilação (trace/JIT) alto**; sharp edges em control flow |
| EXTENSIBILITY | Primitivas custom (custom_vjp, custom_jvp); plugins de plataforma; Pallas |
| PLUGIN SYSTEM | `jax_plugins` (backend/pytree plugins) — formalizado |
| SECURITY | Projeto de pesquisa explícito ("not an official Google product; expect sharp edges") |
| LICENSE | Apache-2.0 |
| STRENGTHS | Composabilidade transformacional única; escala multi-dispositivo; base de muitos frameworks ML modernos |
| WEAKNESSES | Modelo funcional (sem mutação in-place) — paradigma diferente; compile-time; erro de tipo obscuro; documentação exigente |
| TRADE-OFFS | Expressividade/compilação ⇄ curva de aprendizado e ergonomia; controle de hardware ⇄ simplicidade |

**Princípio para o Cosca:** Separe **lógica numérica "funcional e transformável"** (compilável, diferenciável, vetorizável) do código imperativo de I/O — é o padrão que permite reutilizar o mesmo algoritmo em CPU/GPU/TPU sem reescrita.
**Armadilha a evitar:** Assumir comportamento Python (mutação, print, side-effects em funções `jitted`) — quebra silenciosa; e subestimar o tempo de compilação em pipelines interativos.

---

## Projeto: Julia

| CAMPO | VALOR |
|---|---|
| PROJECT | JuliaLang/julia |
| CATEGORY | Científico |
| PURPOSE | Linguagem dinâmica de alto desempenho para computação técnica (HPC, ML, ciência) |
| ARCHITECTURE | Compilador JIT (LLVM) com **multiple dispatch**; type inference especializa código por tipos; runtime em C; stdlib modular |
| LANGUAGE | Julia (com núcleo C/C++/LLVM) |
| CORE ALGORITHMS | Multiple dispatch; inferência de tipo → especialização (codegen); JIT; GC generacional; metaprogramação de macros |
| PIPELINES | Scripts/programas; paralelismo via threads/tasks; sem orquestração nativa de DAG |
| DATA MODEL | Tipos imutáveis/mutáveis; arrays; sistemas de tipos paramétricos; structs especializadas por tipo |
| GPU/CPU/MEMORY | CPU multi-core nativo; GPU via CUDA.jl/AMDGPU.jl; controle fino de layout de memória |
| PERFORMANCE | Próximo de C/Fortran em código bem escrito; **custo de latência de compilação (time-to-first-plot)** alto; tuning fino para desempenho |
| EXTENSIBILITY | Multiple dispatch torna composição natural; ecossistema de pacotes enorme (JuliaHub/General) |
| PLUGIN SYSTEM | Pacotes Julia (registry central); composição por dispatch, sem "plugin" no sentido de extensão de host |
| SECURITY | MIT; requer divulgação de uso de IA generativa (policy); comunidade ativa |
| LICENSE | MIT |
| STRENGTHS | Velocidade de linguagem compilada com ergonomia dinâmica; multiple dispatch é padrão de composição elegante; forte em HPC/ciência |
| WEAKNESSES | Latência de compilação; ecossistema menor que Python; ferramentas de deploy menos maduras; curva de perfumação de tipos |
| TRADE-OFFS | Desempenho/expressividade ⇄ ecossistema e latência de startup; linguagem própria ⇄ integração com stack Python do Cosca |

**Princípio para o Cosca:** **Adote múltiplos dispatch / composição por protocolos no design de tipos do Cosca** — em vez de hierarquias rígidas de classes, modele comportamentos por interfaces (dispatch), que é mais extensível para plugins e backends.
**Armadilha a evitar:** Não trazer Julia para dentro do stack principal do Cosca por "velocidade" sem pagar o custo de ecossistema/startup; use linguagens nativas ou binding (Rust/C) onde o custo é pontual.

---

## Projeto: Eigen

| CAMPO | VALOR |
|---|---|
| PROJECT | eigen-mirror/eigen (canônico: gitlab.com/libeigen/eigen) |
| CATEGORY | Científico |
| PURPOSE | Biblioteca C++ de álgebra linear: matrizes, vetores, solvers numéricos |
| ARCHITECTURE | **Template library header-only**; expression templates (avaliação lazy/vectorizada sem temporários); runtime sem dependências |
| LANGUAGE | C++ (templates) |
| CORE ALGORITHMS | Expression templates + SIMD; decomposições (LU, QR, SVD, Cholesky); solvers esparsos (SuperLU/MKL); vetorização auto |
| PIPELINES | Código compilado; sem pipeline runtime |
| DATA MODEL | Matrizes/tensores estáticos e dinâmicos; layout row/column-major; sparse structures |
| GPU/CPU/MEMORY | CPU; SIMD (SSE/AVX/NEON); sem GPU nativa (via backends) |
| PERFORMANCE | Excelente p/ templates (zero-overhead); depende de flags de compilação/BLAS; pode ser superado por BLAS otimizado em grandes dimensões |
| EXTENSIBILITY | Expression system extensível; custom scalar types; plugins por macro `EIGEN_MATRIX_PLUGIN` |
| PLUGIN SYSTEM | Macros de plugin no tipo (`EIGEN_MATRIX_PLUGIN`) — leve |
| SECURITY | C++ manual memory; bugs de segurança dependem do uso; fuzzing não documentado no repo espelho |
| LICENSE | MPL-2.0 (com alguns componentes Apache/BSD/MINPACK) |
| STRENGTHS | Zero-dependency, embeddável; velocidade via templates; onipresente em C++ científico (TensorFlow, OpenCV, e mais) |
| WEAKNESSES | Compile-time pesado; mensagens de erro de template notoriamente ruins; runtime header-only incha build |
| TRADE-OFFS | Performance via templates ⇄ tempo de compilação e legibilidade de erros; header-only ⇄ footprint binário |

> **Nota de honestidade:** o GitHub `eigen-mirror` tem apenas ~31 estrelas — é um **espelho**; o desenvolvimento real está no GitLab. Citar o mirror sem esse contexto enganaria. A avaliação acima considera o projeto Eigen real (maturidade altíssima, usado pela indústria inteira).

**Princípio para o Cosca:** **Bibliotecas header-only/template com zero-dependência são ideais para kernels nativos embutíveis** — considerar Eigen (ou similares) como primitivo de álgebra linear dentro de binários Cosca sem adicionar dependências de runtime.
**Armadilha a evitar:** Não citar o mirror GitHub como projeto de 31 estrelas (maturidade real é do GitLab); e não usar Eigen em hotspots de grandes dimensões sem benchmark contra BLAS otimizado.

---

## Projeto: qpdf

| CAMPO | VALOR |
|---|---|
| PROJECT | qpdf/qpdf |
| CATEGORY | Documentos |
| PURPOSE | Transformador de PDF **content-preserving**: linearização, criptografia, split/merge, inspeção estrutural, PDF/A |
| ARCHITECTURE | CLI + biblioteca C++ (`libqpdf`); core p/ manipular estrutura PDF (objetos xref); build CMake; zlib/jpeg deps; zopfli opcional |
| LANGUAGE | C++ (C++17/20) |
| CORE ALGORITHMS | Parser de xref/objetos; reescrita determinística de PDF (object stream, cross-reference); criptografia RC4/AES (providers native/gnutls/openssl); compressão flate/zopfli |
| PIPELINES | CLI pipelines (input → transform → output); sem orquestração própria |
| DATA MODEL | Modelo de objetos PDF (`QPDFObjectHandle`): dicionários, streams, arrays; conteúdo preservado byte-a-byte |
| GPU/CPU/MEMORY | CPU; memória moderada; otimizado p/ arquivos grandes (testes >4GB); sem GPU |
| PERFORMANCE | Rápido em operações estruturais; zopfli é ~100x mais lento que zlib (para arquivo menor) — trade-off explícito |
| EXTENSIBILITY | C API estável (`include/qpdf`); API rica para escrita de extensões C++ |
| PLUGIN SYSTEM | Não é plugin system; extensão por API/CLI scriptable |
| SECURITY | Forte: releases assinados (cosign/GPG), dir `fuzz/`, README-hardening, providers criptográficos plugáveis; crypto nativo permite operar sem OpenSSL |
| LICENSE | Apache-2.0 (pré-v7 Artistic-2.0 opcional) |
| STRENGTHS | "Não estraga nada" (content-preserving) é raro e valioso; suporte amplo de PDF; fuzzing e hardening; sem dependências pesadas |
| WEAKNESSES | **Não renderiza e não extrai texto** (deliberado — fica para outras ferramentas); sem APIs de alto nível para conteúdo de página |
| TRADE-OFFS | Preservação/estrutura correta ⇄ ausência de render/extração; baixo nível ⇄ precisão |

**Princípio para o Cosca:** **Separe "estrutura do documento" de "conteúdo do documento"**: use qpdf como camada estrutural (validar, linearizar, criptografar, dividir) e uma engine de extração (PyMuPDF/docling) para conteúdo — responsabilidades distintas e testáveis.
**Armadilha a evitar:** Usar qpdf para extrair texto/renderizar (não faz) e descobrir na hora de implementar; e não ignorar o trade-off zopfli (100x mais lento) ao gerar PDFs de arquivo.

---

## Projeto: OCRmyPDF

| CAMPO | VALOR |
|---|---|
| PROJECT | ocrmypdf/OCRmyPDF |
| CATEGORY | Documentos |
| PURPOSE | Adiciona camada de texto OCR a PDFs escaneados, gerando PDF/A pesquisável |
| ARCHITECTURE | CLI/script Python puro (com lib) que orquestra binários externos: **Tesseract** (OCR) + **Ghostscript** (PDF/A); plugin interface; `--jobs N` multinúcleo |
| LANGUAGE | Python (orquestração) + binários C++ (tesseract/ghostscript) |
| CORE ALGORITHMS | OCR por Tesseract LSTM; deskew; rotacionamento automático; compressão de imagem; construção PDF/A com validação |
| PIPELINES | Pipeline determinístico por página: pdf→imagem→pré-processo→OCR→inserir camada→validar PDF/A; paraleliza por página (`--jobs`) |
| DATA MODEL | PDF escaneado → PDF com camada de texto invisível + imagem original em resolução exata; metadados |
| GPU/CPU/MEMORY | CPU multi-core; sem GPU (GPU via plugins de engine); escala a milhares de páginas; "battle-tested em milhões de PDFs" |
| PERFORMANCE | Distribui por cores; GPU não nativa; qualidade depende do Tesseract + pré-processamento |
| EXTENSIBILITY | **Plugin interface** para substituir o engine OCR: plugins AppleOCR, EasyOCR, PaddleOCR existentes |
| PLUGIN SYSTEM | Sistema de plugins real (entry points) para engines de OCR — demonstrado |
| SECURITY | MPL-2.0; foco em "private data stays private" (local); validação de entrada/saída; versões assinadas via Docker |
| LICENSE | MPL-2.0 (non-core MIT, docs CC-BY-SA) |
| STRENGTHS | Padrão-ouro para PDF pesquisável; preserva resolução/imagem original; valida PDF/A; plugin engine; escala por página |
| WEAKNESSES | Requer Ghostscript+Tesseract instalados (deps externas); qualidade depende de preprocessing; sem GPU nativa |
| TRADE-OFFS | Fidelidade (imagem exata + PDF/A) ⇄ dependência de binários externos; OCR genérico (Tesseract) ⇄ precisão de engines modernos |

**Princípio para o Cosca:** **Pipeline de documentos = orquestrador fino + engines plugáveis**: manter o Cosca independente do engine de OCR (interface de plugin como OCRmyPDF), permitindo trocar Tesseract → PaddleOCR → engine VLM sem reescrever o pipeline.
**Armadilha a evitar:** Empacotar binários externos (ghostscript/tesseract) sem isolar versões — compatibilidade PDF/A e OCR quebram silenciosamente entre versões; pin e teste em CI.

---

## Projeto: PyMuPDF

| CAMPO | VALOR |
|---|---|
| PROJECT | pymupdf/PyMuPDF |
| CATEGORY | Documentos |
| PURPOSE | Biblioteca Python de alto desempenho sobre MuPDF (C): extração, análise, conversão, renderização e manipulação de PDF/EPUB/XPS/Office* |
| ARCHITECTURE | Binding Python sobre engine C MuPDF; sem dependências Python obrigatórias; renderização própria (não Ghostscript) |
| LANGUAGE | Python + C (MuPDF) |
| CORE ALGORITHMS | Extração de texto com metadados (spans/blocks); detecção de tabelas (`find_tables`); OCR via tessdata do MuPDF; renderização de página (pixmap); redaction permanente; manipulação de formas/objetos |
| PIPELINES | API imperativa; processamento por página; multiprocessing recomendado (não thread-safe); PyMuPDF4LLM gera Markdown/JSON pronto para RAG |
| DATA MODEL | Document/Page/Pixmap; text como dict hierárquico (blocks→lines→spans com bbox/font); metadados e TOC |
| GPU/CPU/MEMORY | CPU; memória mínima; rendering em alta DPI; sem GPU |
| PERFORMANCE | 10–50x mais rápido que libs Python puras em extração; 100x+ em rendering; baixo footprint; **não thread-safe** (usar processos) |
| EXTENSIBILITY | API rica e de baixo nível; componentes: `pymupdf4llm`, `pymupdf-fonts`, `pymupdfpro` (Office) |
| PLUGIN SYSTEM | Pacotes complementares, não plugin runtime |
| SECURITY | Execução 100% local/air-gapped (sem telemetria); forte para HIPAA/financeiro/legal; sem callbacks de licença no build OSS |
| LICENSE | **AGPL-3.0** (open source) + licença comercial — copyleft forte; crítico para produto fechado |
| STRENGTHS | Velocidade/acuidade; extração rica (font, bbox, cor); LLM-ready (Markdown); local; redaction de verdade; tudo-em-um |
| WEAKNESSES | **AGPL** exige cuidado jurídico se o Cosca for fechado (comercial disponível); não thread-safe; OCR depende de tessdata externo |
| TRADE-OFFS | Qualidade/velocidade/all-in-one ⇄ licença AGPL e threading por processos |

**Princípio para o Cosca:** **Para extração de conteúdo e layout de PDF, priorize engine nativo (C/C++) com binding fino** — a ordem de magnitude de ganho (10–100x) justifica a dependência nativa; e avalie **desde o dia 1 a licença AGPL** antes de fixar PyMuPDF como componente interno.
**Armadilha a evitar:** Engajar AGPL em produto fechado sem plano de licença comercial; e não paralelizar com threads (não thread-safe — só multiprocessing), sob risco de corrupção/crash.

---

## Projeto: Docling ⭐ FOCO

| CAMPO | VALOR |
|---|---|
| PROJECT | docling-project/docling |
| CATEGORY | Documentos |
| PURPOSE | Preparar documentos para gen-AI: parse de múltiplos formatos (PDF, DOCX, PPTX, XLSX, HTML, EPUB, email, áudio, vídeo*) para representação unificada e export Markdown/JSON |
| ARCHITECTURE | Pipeline modular de conversão → **DoclingDocument** (formato unificado, Pydantic v2) → exportadores; componentes de layout/OCR/VLM; `docling-serve` (API server) e MCP server; IBM Research Zurich |
| LANGUAGE | Python |
| CORE ALGORITHMS | Layout/reading-order; estrutura de tabelas; reconhecimento de código/fórmulas; classificação de imagens; OCR; ASR; VLMs (GraniteDocling); agentes de layout |
| PIPELINES | DocumentConverter → DoclingDocument → Markdown/HTML/JSON/DocLang/DocTags; opção `--pipeline vlm` para VLM |
| DATA MODEL | **DoclingDocument** unificado: hierarquia de seções/tabelas/figuras/código; perdendo informação? Não — lossless JSON + Markdown |
| GPU/CPU/MEMORY | CPU para layout clássico; GPU para VLM/OCR neural; modelos baixados do HuggingFace; rodar local/air-gapped |
| PERFORMANCE | Rápido para parse clássico; VLM é mais lento mas mais preciso; benchmarks próprios (perfs/) |
| EXTENSIBILITY | Integrações nativas: LangChain, LlamaIndex, CrewAI, Haystack; MCP; API server; skills para agentes |
| PLUGIN SYSTEM | Integrações + pipeline plugável; modelos custom |
| SECURITY | Rodagem local; MIT com modelos sob licenças próprias; OpenSSF best practices; política de segurança publicada |
| LICENSE | MIT (código); modelos têm licenças individuais |
| STRENGTHS | **Formato unificado DoclingDocument** resolve o "Tower of Babel" de formatos; export LLM-ready; integrações; local; projeto em LF AI & Data |
| WEAKNESSES | Projeto jovem (rápida evolução, breaking changes); downloads de modelos pesados; suporte a vídeo/áudio ainda emergente; dependências pesadas |
| TRADE-OFFS | Unificação/qualidade de parse ⇄ peso e velocidade de evolução; MIT (código) ⇄ licenças de modelos baixados |

**Princípio para o Cosca:** **Adote um formato canônico de documento (tipo DoclingDocument) como o "document object model" do Cosca**: tudo entra (PDF, DOCX, PPTX, HTML…) → modelo único → exporta (Markdown/JSON para LLM, ou para editor). Unificar o input é o multiplicador de pipeline.
**Armadilha a evitar:** Confiar no parse como "perda zero" sem validação — modelos de layout erram em documentos complexos; e não ignorar que cada modelo de VLM tem sua própria licença, mesmo com código MIT.

---

## Projeto: PaddleOCR

| CAMPO | VALOR |
|---|---|
| PROJECT | PaddlePaddle/PaddleOCR |
| CATEGORY | Documentos |
| PURPOSE | Toolkit OCR + Document-AI: de cena a documentos estruturados (JSON/Markdown) com SOTA e leve |
| ARCHITECTURE | Suite de modelos + pipelines: PP-OCR (det+rec), PP-StructureV3 (estrutura), **PaddleOCR-VL (0.9B VLM)**; backends plugáveis (Paddle static/dynamic, ONNX Runtime, OpenVINO, TensorRT); serving; deploy C++ |
| LANGUAGE | Python + C++ (deploy) + PaddlePaddle |
| CORE ALGORITHMS | Detecção/reconhecimento de texto (DB, CTC); PP-StructureV3 layout/tabelas/fórmulas; VLM NaViT-style + ERNIE 0.3B; PP-DocLayoutV3 (formas irregulares); P-MTP decoding (HPD) |
| PIPELINES | Pipeline end-to-end página→Markdown/JSON; paralelização multi-GPU/processo; benchmark por módulo |
| DATA MODEL | Saída estruturada com coordenadas (células de tabela, texto) em JSON/Markdown; suporte DOCX export |
| GPU/CPU/MEMORY | CPU/GPU/NPU/XPU; tiny (1.5M) a medium (34.5M) params; 0.13s/page A100; edge/cloud; 5.2x CPU speedup (OpenVINO) |
| PERFORMANCE | SOTA em OmniDocBench (~96.3% v1.6); 100+ idiomas; alta throughput (HPD 4752 tok/s); tiers por hardware |
| EXTENSIBILITY | Modelos no HuggingFace/ModelScope; pipeline config; integração Dify/RAGFlow/Haystack; MCP server; paddleocr.js (browser) |
| PLUGIN SYSTEM | Config-driven pipelines + backends de inferência; não é "plugin" clássico |
| SECURITY | Apache-2.0; sem política de segurança destacada no README (CVE via GitHub)*; dependências grandes |
| LICENSE | Apache-2.0 |
| STRENGTHS | Melhor acurácia open-source em documentos complexos (tabelas/fórmulas/idiomas raros); leve e multi-hardware; ecossistema de integração amplo |
| WEAKNESSES | Atrasa em dependência de **PaddlePaddle** (framework pesado, ecossistema/docs com forte viés chinês); configs complexos; versionamento rápido |
| TRADE-OFFS | Acurácia SOTA ⇄ complexidade de integração e dependência do framework Paddle; abrangência ⇄ curva de deploy |

**Princípio para o Cosca:** **Para acurácia de OCR/documento em idiomas e layouts complexos, PaddleOCR é o padrão open-source** — mas isole-o atrás de uma camada de serviço/adaptador (interface OCRmyPDF-style) para poder trocar por VLM/alternativa sem refatorar o Cosca.
**Armadilha a evitar:** Amarrar o core do Cosca a PaddlePaddle (framework proprietário e pesado) em vez de encapsular atrás de uma API própria; e subestimar o custo de baixar/hospedar os modelos.

---

## Projeto: Tesseract

| CAMPO | VALOR |
|---|---|
| PROJECT | tesseract-ocr/tesseract |
| CATEGORY | Documentos |
| PURPOSE | Engine OCR open-source: reconhece texto em imagens; mais de 100 idiomas |
| ARCHITECTURE | C++ core (`libtesseract`) + CLI; engine **LSTM neural** (v4+) + legado; depende de Leptonica para entrada de imagem; tessdata (traineddata) por idioma |
| LANGUAGE | C++ |
| CORE ALGORITHMS | Reconhecimento por linha LSTM (CTC); segmentação de página (PSM); pré-processamento; treinamento customizado; output hOCR/PDF/TSV/ALTO/PAGE |
| PIPELINES | imagem → (pré-processo) → OCR → texto estruturado (boxes, hOCR, PDF invisível); CLI e API |
| DATA MODEL | Página→blocos→linhas→palavras→símbolos; confianças por palavra; hOCR/ALTO para layout |
| GPU/CPU/MEMORY | CPU (por padrão); leve; sem GPU nativa; qualidade melhora com pré-processamento |
| PERFORMANCE | Rápido e estável em CPU; acurácia superada por engines neurais modernos (PaddleOCR, EasyOCR) e VLMs; OSS-Fuzz + CodeQL |
| EXTENSIBILITY | API C/C++ estável; training pipeline para novos idiomas; wrappers em todas as linguagens |
| PLUGIN SYSTEM | tessdata como "plugins" de idioma; sem plugin de engine |
| SECURITY | OSS-Fuzz contínuo, CodeQL, Coverity; Apache-2.0; Leptonica BSD |
| LICENSE | Apache-2.0 |
| STRENGTHS | Maduro (desde 1985, Google até 2017); 100+ idiomas; treinável; determinístico; leve; base de OCRmyPDF |
| WEAKNESSES | Acurácia inferior a engines modernos em layouts complexos/idiomas CJK; requer preprocessing (deskew, threshold); sem GPU |
| TRADE-OFFS | Estabilidade/leveza/determinismo ⇄ acurácia em cenários difíceis; "bom o bastante" genérico ⇄ customização |

**Princípio para o Cosca:** **Para OCR em escala, comece pelo Tesseract (leve, treinável, determinístico, sem GPU) e promova para PaddleOCR/VLM apenas quando a acurácia exigir** — o determinismo e a independência de GPU importam para custo e reprodutibilidade.
**Armadilha a evitar:** Jogar imagem crua no Tesseract sem pré-processamento e culpar o engine — deskew/binarização mudam o resultado; e não assumir que mais idiomas = melhor acurácia por idioma.

---

## Projeto: Temporal ⭐ FOCO

| CAMPO | VALOR |
|---|---|
| PROJECT | temporalio/temporal |
| CATEGORY | Workflow |
| PURPOSE | Plataforma de **durable execution**: executa Workflows com resiliência automática a falhas, retry, e estado persistente |
| ARCHITECTURE | Servidor Go multi-serviço (frontend/history/matching/worker) + SDKs de client (Go/Java/TS/Python em repos separados); **event-sourced**: todo o progresso é evento em store (Cassandra/Postgres/MySQL/SQLite-dev); workers executam código determinístico |
| LANGUAGE | Go (servidor); SDKs multi-linguagem |
| CORE ALGORITHMS | Event sourcing do history; **deterministic replay** (re-executa workflow a partir de eventos); retry com backoff; timers/signals; child workflows; sagas; sticky queues |
| PIPELINES | Workflow (orquestração) + Activity (execução com efeitos colaterais); corridas resumíveis; cron distribuído; `temporal workflow` CLI |
| DATA MODEL | Event History por workflow execution (append-only); search attributes (visibilidade); task queues; worker identity |
| GPU/CPU/MEMORY | CPU de workers (horizontally scalable); estado em banco externo (não em memória do app); persistência configuravel |
| PERFORMANCE | Overhead de evento-por-decisão (write amplification em workflows de muitos steps); alto throughput com tuning; latência de retry controlável |
| EXTENSIBILITY | SDKs oficiais + comunidade; plugins de data converter/encryption; Nexus (chamadas cross-namespace); ativações de código |
| PLUGIN SYSTEM | Extensões via SDK/DataConverter; sem plugin runtime central |
| SECURITY | TLS/mTLS, auth/authorization (RBAC), search attributes; community mitigations (ex: workflows como vetor de ataque — hardening) |
| LICENSE | MIT |
| STRENGTHS | **Retomada exata de tarefas longas** (maturidade Cadence→Temporal); determinismo confiável; visibilidade; linguagens múltiplas; ecossistema maduro (Stripe, Netflix, Datadog, ...) |
| WEAKNESSES | **Código de workflow deve ser determinístico** (sem `time.Now()`, sem random, sem I/O direto); infra de servidor a operar; write-amplification; curva de aprendizado |
| TRADE-OFFS | Durable/retomável ⇄ restrição de determinismo e infra; complexidade operacional ⇄ resiliência |

**Princípio para o Cosca:** **§40 — tarefas longas e resumíveis são o coração do Cosca: adote durable execution com event sourcing e deterministic replay** (Temporal ou padrão equivalente): toda longa renderização/generação de mídia deve ser um Workflow que sobrevive a crash, retry e redeploy. State = history, nunca variável em memória.
**Armadilha a evitar:** Escrever lógica não-determinística dentro do workflow (relógio, RNG, I/O direto, dependência de ordem de coleções) — quebra o replay silenciosamente; e não tratar Activities como workflows (toda effect real deve ser Activity com retry semântico).

---

## Projeto: Argo Workflows

| CAMPO | VALOR |
|---|---|
| PROJECT | argoproj/argo-workflows |
| CATEGORY | Workflow |
| PURPOSE | Workflow engine **container-native** para Kubernetes (K8s CRD) |
| ARCHITECTURE | CRD do K8s: cada step é um container/pod; controlador Go; DAG ou steps; artifacts (S3/GCS/HTTP/Git); UI; archiving; multi-executor |
| LANGUAGE | Go (controlador) + YAML (workflows) + SDKs (Go/Java/Python-Hera/TS-Juno) |
| CORE ALGORITHMS | Agendamento de DAG; scheduling/affinity; retry por step; suspend/resume; memoized resubmit; loops/parametrização; garbage collection |
| PIPELINES | WorkflowTemplate → Workflow → steps/pods; artifacts entre steps; cron; retry/resubmit/cancel; notificações via exit hooks |
| DATA MODEL | Workflow/DAG/Steps como CRDs; artifacts como inputs/outputs; parameters; templates reutilizáveis |
| GPU/CPU/MEMORY | Herda recursos do K8s (GPU via K8s); execução por pod; escalabilidade horizontal do cluster |
| PERFORMANCE | Latência por step = tempo de agendamento de pod; overhead por pod; bom para batch/ML massivo; não é "event-sourced" por evento |
| EXTENSIBILITY | Hera (Python SDK), Juno (TS), plugins de artifact/executor; integration com Argo Events/CD |
| PLUGIN SYSTEM | Plugins de executor e de storage de artifacts |
| SECURITY | **20 advisories listados no repo (security and quality)**; SSO OIDC; RBAC K8s; assinatura cosign; OpenSSF scorecard |
| LICENSE | Apache-2.0 (CNCF graduated) |
| STRENGTHS | Padrão de fato para pipelines K8s; ecossistema CNCF; nativo do K8s (sem infra extra); 200+ orgs; UI/observabilidade |
| WEAKNESSES | Requer Kubernetes; resumibilidade no nível de step/pod (não retoma no meio de execução); retry por step, não por "evento" do app; YAML complexo |
| TRADE-OFFS | Container-native/K8s ⇄ ausência de durable-execution no nível de aplicação; simplicidade K8s ⇄ peso do cluster |

**Princípio para o Cosca:** **Escolha o orquestrador pelo nível de resiliência que você precisa**: Argo para batch/etapas em contêineres (K8s) — mas não para tarefas que precisam retomar **dentro** de um step longo; nesse caso combine com durable execution (Temporal) por dentro do step.
**Armadilha a evitar:** Assumir que "workflow" em Argo = durable execution — um pod que morre no meio reinicia o step inteiro (perda de progresso interno); e não subestimar a operação de um cluster K8s só para orquestração.

---

## Projeto: Dagster ⭐ FOCO

| CAMPO | VALOR |
|---|---|
| PROJECT | dagster-io/dagster |
| CATEGORY | Workflow |
| PURPOSE | Orquestração **asset-oriented**: desenvolvimento, produção e observação de **data assets** (tabelas, modelos, relatórios) |
| ARCHITECTURE | Python-first: assets como funções declarativas (`@dg.asset`) com dependências tipadas; run storage/instigator (Postgres); dagster-webserver (UI); gRPC/daemons; integrações; Helm; 27k commits, evolução rápida |
| LANGUAGE | Python (+ TS/React UI) |
| CORE ALGORITHMS | DAG de dependências de assets; **software-defined assets**; fresh policy/auto-materialize; lineage; retry; partitions/backfills; sensors/schedules |
| PIPELINES | assets → execução → metadados (asset observability); materialização sob demanda, backfill, cron |
| DATA MODEL | **Assets como entidade de primeira classe** (não apenas jobs): cada asset tem metadados, checks de qualidade, lineage e upstream/downstream |
| GPU/CPU/MEMORY | Executa em workers/daemons; escala via executors (local, K8s, ECS); estado em banco (event log); suporta recursos K8s/GPU* |
| PERFORMANCE | Overhead moderado; foco em observabilidade/lineage mais que latência máxima; eventos gravados no event log |
| EXTENSIBILITY | Integrações com stack moderno (dbt, Airbyte, Snowflake, Spark, MLflow, ...); API Python rica; MCP; componentes |
| PLUGIN SYSTEM | `dagster-*` integration libraries; resource configs; sem plugin binário |
| SECURITY | Apache-2.0; autenticação/autorização na UI (TLS); RBAC básico; avisos de segurança no repo |
| LICENSE | Apache-2.0 |
| STRENGTHS | **Modelo mental centrado em assets/data** — "o que é produzido" em vez de "quais tarefas rodam"; lineage/observabilidade embutida; testabilidade; UI ótima; flexível |
| WEAKNESSES | Não é durable execution no sentido do Temporal (retry/asset; execução pode ser interrompida); framework opinativo e pesado; curva de aprendizado; ~27k commits = API em movimento |
| TRADE-OFFS | Observabilidade/lineage/declaratividade ⇄ simplicidade e resiliência de baixo nível; asset-model ⇄ task-model tradicional |

**Princípio para o Cosca:** **Modele a plataforma por "assets/data" e não por "jobs"**: defina cada artefato do Cosca (cena, áudio, texto, estrutura do documento) como um asset com dependências, lineage e qualidade verificável — é o modelo que escala em complexidade e transparência.
**Armadilha a evitar:** Confundir Dagster com durable execution — para resumibilidade de longas tarefas use Temporal; e não adotar o framework inteiro (pesado/opinativo) sem avaliar se a UI + modelo de assets justifica para o caso do Cosca.

---

## Projeto: VS Code

| CAMPO | VALOR |
|---|---|
| PROJECT | microsoft/vscode |
| CATEGORY | DevTools |
| PURPOSE | Editor de código (distribuição "Code - OSS"); base extensível para edição, debug, navegação |
| ARCHITECTURE | Electron (Chromium+Node); **Extension Host** isolado em processo; comunicacao por protocolo interno; Monaco Editor como core; LSP client |
| LANGUAGE | TypeScript + Node.js (+ Electron/C++) |
| CORE ALGORITHMS | Editor de texto com tokenização (Monarch/TextMate); arquitetura de processos (renderer/main/extension host); LSP protocol; incremental buffer |
| PIPELINES | Processos separados: UI + extension host + language servers; comunicação assíncrona |
| DATA MODEL | Workspace/files + buffer de texto; configuração por settings/JSON; extensões com activation events |
| GPU/CPU/MEMORY | **Consumo de memória alto (Electron)**; aceleração GPU p/ render (opcional); requer 4 cores/6GB para build (dev) |
| PERFORMANCE | Responsivo para uso geral; overhead de Electron notável em máquinas fracas; extensões podem degradar |
| EXTENSIBILITY | **Modelo de extensões massivo** (Marketplace); LSP; debug adapters; tasks; Dev Containers |
| PLUGIN SYSTEM | Sistema de extensões maduro (host isolado, manifest, activation events, contribution points) |
| SECURITY | Modelo de confiança por extensões (workspace trust); sandboxing do extension host; **43 security alerts no repo (security and quality)**; telemetria opcional |
| LICENSE | MIT (Code-OSS); distribuição VS Code com licença proprietária da Microsoft |
| STRENGTHS | Ecossistema de extensões sem rival; modelo de host isolado seguro; LSP integrado; comunidade massiva |
| WEAKNESSES | Peso de Electron; extensões de baixa qualidade podem quebrar/roubar; dependência do ecossistema Microsoft |
| TRADE-OFFS | Ecossistema/extensibilidade ⇄ peso e consumo de recursos; flexibilidade ⇄ risco de segurança de extensões |

**Princípio para o Cosca:** **Isolar código de extensões/plugins em processo separado (host) com modelo de contribuição declarativo** — é o que permite que o core fique estável e o ecossistema cresça sem comprometer segurança.
**Armadilha a evitar:** Não copiar o modelo Electron (peso/memória) para o Cosca se o alvo é performance (ex. Zed/Neovim mostram o custo); e não confiar cegamente em extensões de terceiros no host do editor.

---

## Projeto: Monaco Editor

| CAMPO | VALOR |
|---|---|
| PROJECT | microsoft/monaco-editor |
| CATEGORY | DevTools |
| PURPOSE | Editor de código **no browser** (o editor do VS Code extraído para web) |
| ARCHITECTURE | TS puro (ESM/AMD); web workers para language services; conceitos: **Models** (conteúdo), **URIs** (identidade), **Editors** (view), **Providers** (features inteligentes), **Disposables** (ciclo de vida) |
| LANGUAGE | TypeScript |
| CORE ALGORITHMS | Tokenização **Monarch** (e TextMate via `monaco-tm`); buffer de texto com undo/redo; diff model; workers para parsing LSP; virtualized rendering |
| PIPELINES | Model → Providers (completion, hover, diagnostics via LSP em workers); eventos de edição |
| DATA MODEL | Models identificados por URI (vfs virtual); edição com versioning; view state por editor |
| GPU/CPU/MEMORY | Browser (CPU); bundle pesado (~vários MB); workers por linguagem; mobile não suportado |
| PERFORMANCE | Rápido para edição comum; limites em arquivos gigantes; bundle size é custo real |
| EXTENSIBILITY | APIs de providers (completion, hover, code actions); monaco-lsp-client (LSP no browser); themes/languages custom |
| PLUGIN SYSTEM | Não roda extensões VS Code; só features via providers/JS (LSP em JS funciona) |
| SECURITY | MIT; execução no sandbox do browser (workers); sem privilégios de sistema |
| LICENSE | MIT |
| STRENGTHS | Editor web de referência; modelos/URI/Provider são design limpo; LSP integração; maduro e testado em produção (VS Code web) |
| WEAKNESSES | Não suporta extensões VS Code; bundle grande; tokenização por regex (Monarch) menos preciso que parser real (tree-sitter) |
| TRADE-OFFS | Prontidão web/ecossistema ⇄ peso e profundidade de parsing; conveniência ⇄ controle de rendering |

**Princípio para o Cosca:** **Separe conteúdo (Model) de visão (Editor) e features (Providers)** — arquitetura por modelos+providers é o padrão para editor embutido em produto (Cosca pode embutir Monaco ou reusar a mesma separação conceitual).
**Armadilha a evitar:** Esperar que Monaco seja igual ao VS Code (não roda extensões, não tem TextMate nativo, bundle pesado) — avaliar se tree-sitter (parsing real) é necessário para highlighting preciso.

---

## Projeto: Tree-sitter ⭐ FOCO

| CAMPO | VALOR |
|---|---|
| PROJECT | tree-sitter/tree-sitter |
| CATEGORY | DevTools |
| PURPOSE | **Parsing incremental** para ferramentas de programação: árvore sintática concreta atualizada a cada keystroke |
| ARCHITECTURE | Gerador de parsers (Rust) + runtime em **C puro** (embutível, sem dependências); bindings Rust/Wasm; gramáticas em JS (repos separados por linguagem) |
| LANGUAGE | Rust (gerador/CLI) + C (runtime) + JS (gramáticas) |
| CORE ALGORITHMS | **Incremental parsing** (reparse de subárvores editadas); GLR parser; **error tolerance** (produz árvore útil mesmo com erros de sintaxe); parsing off-thread (async) |
| PIPELINES | Gramática → parser gerado (Wasm/native) → árvore; edição → atualização incremental (O(tamanho da mudança)); queries para extração de sintaxe |
| DATA MODEL | Concrete Syntax Tree (não AST abstrata); nós com ranges/posições; árvore compartilhada/imutável com versionamento |
| GPU/CPU/MEMORY | CPU; leve (runtime C); adequado a editor (per-keystroke); parsing assíncrono |
| PERFORMANCE | "Rápido o suficiente para parse a cada tecla"; atualização incremental evita reparse do arquivo inteiro; memória eficiente |
| EXTENSIBILITY | Gramáticas por linguagem (centenas); bindings para muitas linguagens; consultas (queries) para highlight/indentação/folding |
| PLUGIN SYSTEM | Gramáticas como "plugins" de linguagem (repos independentes); consumo via parsers |
| SECURITY | MIT; runtime C embutível — segurança depende de gramáticas (parsing de input não confiável deve ser sandboxed); fuzzing existe na comunidade |
| LICENSE | MIT |
| STRENGTHS | **Paradigma correto para editor**: incremental + error-tolerant; real (não regex); embutível (C/Wasm — roda no browser); usado por GitHub/Neovim/Zed |
| WEAKNESSES | Gramáticas com qualidade variável (manutenção externa); árvore concreta requer camada para AST semântica; error-recovery heurística |
| TRADE-OFFS | Velocidade/robustez/portabilidade ⇄ qualidade por-linguagem e ausência de semântica; parsing genérico ⇄ análise semântica (LSP complementa) |

**Princípio para o Cosca:** **Para qualquer editor/highlighting/extração de código no Cosca, use parsing incremental com árvore concreta e erro-tolerante (tree-sitter)** — é a única abordagem que escala para edição em tempo real e para extrair estrutura de código em conteúdo gerado por IA.
**Armadilha a evitar:** Tratar a CST como AST: para semântica (types, references) o Cosca ainda precisa de LSP/analysis; e não assumir que uma gramática de linguagem X é mantida — valide cobertura e manutenção antes de depender dela.

---

## Projeto: Zed

| CAMPO | VALOR |
|---|---|
| PROJECT | zed-industries/zed |
| CATEGORY | DevTools |
| PURPOSE | Editor de código de alta performance, multiplayer, em Rust (dos criadores de Atom e Tree-sitter) |
| ARCHITECTURE | **Rust + GPU UI (GPUI)**; renderização própria via GPU; collaborative (collab server); LSP client integrado; tree-sitter para parsing |
| LANGUAGE | Rust (quase tudo) |
| CORE ALGORITHMS | Rope/text buffer com edit incremental; GPU-driven layout/rendering; tree-sitter incremental parsing; collab CRDT (colaboração) |
| PIPELINES | Buffer → parse incremental → LSP queries → UI GPU; processos separados (collab, extensions) |
| DATA MODEL | Text buffers (rope), files no workspace; projetos/multibuffer; colaboração via CRDT |
| GPU/CPU/MEMORY | **GPU para renderização** (único entre editores); baixa latência de startup; memória menor que Electron |
| PERFORMANCE | Startup/latência "na velocidade do pensamento"; vantagem grande vs Electron em máquinas moderadas |
| EXTENSIBILITY | Sistema de extensões (extensions/); linguagens via LSP; configuração; tema |
| PLUGIN SYSTEM | Extensões (novas, em crescimento); não compara com VS Code Marketplace |
| SECURITY | 12 security alerts no repo; GPL-3.0; modelo de colaboração requer servidor; sandbox de extensões |
| LICENSE | GPL-3.0 (com componentes Apache-2.0) |
| STRENGTHS | Performance GPU-first; colaboração multiplayer integrada; código moderno; menor footprint |
| WEAKNESSES | Ecossistema de extensões imaturo vs VS Code; **GPL** limita reuso em produto fechado; Windows/Linux/macOS mas sem web |
| TRADE-OFFS | Performance/UX ⇄ ecossistema/maturidade; inovação (GPU UI) ⇄ compatibilidade e licença |

**Princípio para o Cosca:** **GPU-driven rendering e buffer rope são caminho para UX de baixa latência** — se o Cosca construir UI de timeline/editor de mídia, considere arquitetura tipo GPUI (render por GPU) em vez de DOM pesado.
**Armadilha a evitar:** Adotar Zed (GPL) ou seu modelo sem checar licença e ecossistema; e não copiar "multimodal UI" sem considerar o custo de manter um toolkit próprio de GPU.

---

## Projeto: Neovim

| CAMPO | VALOR |
|---|---|
| PROJECT | neovim/neovim |
| CATEGORY | DevTools |
| PURPOSE | Fork do Vim focado em **extensibilidade** e usabilidade; editor programável |
| ARCHITECTURE | Core C + **Lua de primeira classe** (scripting); API RPC (msgpack) para GUIs externas/headless; event loop assíncrono; job control; integração tree-sitter/LSP nativa |
| LANGUAGE | C + Lua (embed) |
| CORE ALGORITHMS | Buffer de texto (Vim-like); parsing via tree-sitter (integrado); LSP client; async job control; msgpack-RPC para UI |
| PIPELINES | Editor (event loop) → APIs (RPC) → clientes externos (GUIs, LSP, agentes) |
| DATA MODEL | Buffers/windows/tabs; shared data (shada) entre instâncias; runtime plugins Lua |
| GPU/CPU/MEMORY | CPU; muito leve (terminal/headless); sem GPU; adequado a servidores/CI/CLI |
| PERFORMANCE | Extremamente leve; headless rápido; editor de terminal; desempenho de parsing depende de tree-sitter |
| EXTENSIBILITY | **API RPC completa** de qualquer linguagem; Lua plugin; headless (sem UI) para automação |
| PLUGIN SYSTEM | Plugin Lua massivo (Neovim ecosystem); config em Lua; compat Vim plugins |
| SECURITY | Apache-2.0; plugins de terceiros rodam com privilégio do processo (sem sandbox robusto)*; security policy publicada |
| LICENSE | Apache-2.0 (contribuições pós-2015; compat Vim) |
| STRENGTHS | Programável/headless; leve; RPC torna qualquer ferramenta um "cliente de editor"; Lua moderno |
| WEAKNESSES | Curva de aprendizado Vim-modal alta; configuração exige Lua; ecossistema mais fragmentado; sem GUI própria rica |
| TRADE-OFFS | Leveza/programabilidade ⇄ usabilidade/curva; headless/automação ⇄ UX de GUI |

**Princípio para o Cosca:** **Um core de editor exposto via RPC (headless) permite automação e integração de qualquer linguagem** — para features de edição no Cosca (scripts, agentes, pipelines), uma API de editor programável vale mais que uma GUI acoplada.
**Armadilha a evitar:** Depender de plugins de terceiros sem revisar (rodam no processo); e não presumir que "extensível" em Neovim = fácil — a curva Lua/Vim é real para novos contribuidores.

---

# SÍNTESE DA CATEGORIA

## CIENTÍFICO

**8 princípios fortes:**
1. Núcleo estável com contrato explícito de API/ABI (NumPy) — o que o ecossistema constrói sobre você.
2. Use a melhor implementação madura de algoritmo, não reimplemente (SciPy).
3. Engine columnar Arrow + lazy + streaming para dados estruturados (Polars) — leia só o necessário, processe > RAM.
4. Separe lógica numérica funcional/transformável (JAX) do I/O imperativo — reuso em CPU/GPU sem reescrita.
5. Componha por múltiplo dispatch/protocolos, não hierarquias rígidas (Julia).
6. Bibliotecas header-only/zero-dep para kernels nativos embutíveis (Eigen).
7. Escolha bibliotecas maduras com governança e fuzzing (numpy/scipy/tesseract) — a segurança vem da maturidade.
8. Não use array onde tabela resolve; não use tabela onde engine columnar resolve — a camada de dados certa por caso.

**8 armadilhas comuns:**
1. Tratar array/DataFrame como solução universal de dados (custo de memória e expressividade).
2. Reimplementar solver/algoritmo em vez de usar biblioteca testada.
3. Ignorar o lazy optimizer e materializar intermediários (Polars) — mata o ganho.
4. Assumir semântica Python em código compilado/transformável (JAX jit) — quebras silenciosas.
5. Citar mirror como projeto real (Eigen no GitHub: 31 estrelas vs projeto real no GitLab).
6. Confiar em "velocidade de linguagem" sem pagar custo de ecossistema/startup (Julia).
7. Benchmarks de marketing sem contexto de hardware/dataset.
8. Ignorar copy-on-write/ownership (pandas) — bugs sutis de views/aliasing.

## DOCUMENTOS

**8 princípios fortes:**
1. Separe camada estrutural (qpdf) de camada de conteúdo (PyMuPDF/docling) — responsabilidades testáveis.
2. Engine nativo (C/C++) com binding fino dá 10–100x (PyMuPDF) — vale a dependência nativa.
3. Formato canônico unificado de documento (DoclingDocument) como "DOM" do pipeline (Docling).
4. Orquestrador fino + engines de OCR plugáveis (OCRmyPDF plugin interface) — troque Tesseract→PaddleOCR→VLM sem refatorar.
5. Comece pelo OCR leve/determinístico (Tesseract) e promova por demanda (PaddleOCR).
6. Pré-processe imagens antes do OCR (deskew/binarização) — qualidade vem da entrada.
7. Avalie licenças (AGPL PyMuPDF, MIT código vs modelos Docling) desde o dia 1.
8. Para layouts/idiomas complexos, PaddleOCR é o padrão open-source SOTA — mas atrás de adaptador.

**8 armadilhas comuns:**
1. Usar qpdf para extrair/renderizar (não faz).
2. Engajar AGPL em produto fechado sem plano de licença (PyMuPDF).
3. Paralelizar PyMuPDF com threads (não thread-safe — só processos).
4. Depender de binários externos (ghostscript/tesseract) sem pinar versões — quebras silenciosas de PDF/A.
5. Confiar em parse como perda-zero sem validação de layout (Docling/PaddleOCR).
6. Amarrar o core a PaddlePaddle em vez de encapsular atrás de API própria.
7. Ignorar pré-processamento de imagem e culpar o engine OCR.
8. Assumir que modelo VLM sob código MIT também é MIT — licenças de modelos são separadas.

## WORKFLOW

**8 princípios fortes:**
1. Durable execution + event sourcing + deterministic replay para tarefas longas resumíveis (§40 do Cosca) — Temporal.
2. State = history (append-only), nunca variável em memória.
3. Efeitos colaterais reais só em Activities (retry semântico); workflows só orquestram.
4. Modele por assets/data, não por jobs (Dagster) — lineage e qualidade verificável.
5. Escolha o orquestrador pelo nível de resiliência: K8s batch (Argo) ≠ retomada intra-step (Temporal).
6. Retry com backoff explícito e timeouts por step/workflow (Argo).
7. Visibilidade/observabilidade de execução são features de primeira classe (UI, eventos, search attributes).
8. Testabilidade: workflows determinísticos são testáveis em memória com time-skipping (Temporal SDK).

**8 armadilhas comuns:**
1. Lógica não-determinística no workflow (clock, RNG, I/O direto) — quebra replay.
2. Confundir "workflow engine" com "durable execution" (Argo reinicia o pod inteiro).
3. Adotar framework opinativo pesado (Dagster) sem avaliar se assets/UI justificam.
4. Subestimar write-amplification de event sourcing em workflows de muitos steps.
5. Colocar efeitos colaterais no workflow em vez de Activity — perda de retry correto.
6. Não planejar infra do servidor (Temporal) — durable execution não é free.
7. YAML de workflow sem versionamento/validação (Argo) vira dependência frágil.
8. Ignorar auth/RBAC/SSO na exposição de UI/API de workflow.

## DEVTOOLS

**8 princípios fortes:**
1. Isolar plugins/extensões em host separado com modelo de contribuição declarativo (VS Code).
2. Separe conteúdo (Model) de visão (Editor) e features (Providers) — arquitetura de editor embutível (Monaco).
3. Parsing incremental + CST erro-tolerante para edição em tempo real (tree-sitter) — não regex.
4. Parsing no editor é assíncrono/off-thread (workers/threads) — não bloqueie UI.
5. Complemente CST com semântica via LSP (types/references) — parsing ≠ análise.
6. GPU-driven rendering + rope buffers para UX de baixa latência (Zed).
7. Core de editor exposto via RPC/headless habilita automação e agentes (Neovim).
8. Embutir runtime C/Wasm (tree-sitter, MuPDF) mantém portabilidade e performance.

**8 armadilhas comuns:**
1. Copiar Electron (peso/memória) quando o alvo é performance.
2. Esperar que Monaco rode extensões VS Code ou TextMate nativo — não roda.
3. Tratar CST como AST — falta semântica para refactor/tipos.
4. Confiar em gramáticas tree-sitter sem validar manutenção/cobertura por linguagem.
5. Licença GPL (Zed) ignorada no planejamento de produto fechado.
6. Plugins de terceiros rodando com privilégio do processo sem revisão (Neovim/VS Code).
7. Bundle web pesado (Monaco) sem tree-shaking/otimização.
8. Subestimar a curva de config Lua/Vim (Neovim) para novos contribuidores.

---

*Nota metodológica: dados de arquitetura/língua/licença verificados nos READMEs dos repositórios (ago/2026). Métricas de desempenho numérico e dados de segurança não declarados nos READMEs estão marcados com `*` (não verificado). A avaliação de maturidade considera histórico, governança, adoção e qualidade, não apenas contagem de estrelas.
