# Research Matrix — Vídeo + Áudio

> Dossiê de pesquisa para o **Cosca** (plataforma de criação multimídia assistida por IA).
> Hardware-alvo do Cosca: **GPU AMD ROCm, gfx1030** (RX 6700 XT / 6800 / 6900 XT). Todo campo GPU observa ROCm/OpenCL/Vulkan vs CUDA-only.
> Fontes: README, estrutura de repositório, licenças e documentação pública de cada projeto (verificadas em ago/2026). Campos marcados com *(não verificado)* não foram confirmáveis por leitura direta.

---

## Projeto: FFmpeg

| PROJECT | FFmpeg/FFmpeg |
|---|---|
| **CATEGORY** | Vídeo + Áudio — biblioteca/CLI de processamento multimídia (codecs, containers, filtros) |
| **PURPOSE** | Decodificar, codificar, transcode, filtrar, mux/demux e transmitir áudio/vídeo. É o backend de codec/filtro de praticamente todo o ecossistema (OBS, MLT, Kdenlive, Olive, Whisper via ffmpeg) |
| **ARCHITECTURE** | 7 bibliotecas C com APIs públicas estáveis: `libavcodec` (codecs), `libavformat` (containers/IO), `libavutil` (utils), `libavfilter` (grafo de filtros), `libavdevice` (captura/dispositivos), `libswresample` (mix/resample), `libswscale` (cor/escala). Ferramentas `ffmpeg`, `ffplay`, `ffprobe` |
| **LANGUAGE** | C (+ trechos em ASM/SIMD por arquitetura) |
| **CORE ALGORITHMS** | Codecs (H.264/HEVC/AV1/VP9/AAC/Opus/MP3, ~100+ codecs), grafo de filtros dirigido (DAG), escalonamento/correção de cores (libswscale), resample de áudio (libswresample), threading por frame/slice/pacote |
| **PIPELINES** | CLI `ffmpeg -i in -vf "filtergraph" -c:v codec out`. Grafo de filtros como DAG de filtros conectados; cada nó recebe/enfileira frames (AVFrame) |
| **DATA MODEL** | `AVFrame` (vídeo decodificado/carregável em memória), `AVPacket` (bitstream comprimido), `AVFormatContext`, `AVCodecContext`, `AVFilterGraph` — estruturas empilháveis a partir de C |
| **GPU/CPU/MEMORY** | CPU-first. Aceleração via hwaccel/filters: **VAAPI** (padrão em ROCm Linux), **VDPAU**, **Vulkan**, **OpenCL**, e CUDA/NVENC apenas se compilado com SDK NVIDIA. Nenhuma dependência obrigatória de CUDA |
| **PERFORMANCE** | Reference-grade em compatibilidade; throughput depende do codec escolhido (libx264 SIMD otimizado; AV1 via SVT-AV1/libaom). Threading pthreads/Win32 |
| **EXTENSIBILITY** | API pública em C; novos filtros/codecs/muxers registrados por `AVFilter`, `AVCodec`, `AVOutputFormat`. Filtros customizados viáveis para pipeline de IA do Cosca |
| **PLUGIN SYSTEM** | Não é plugin em runtime — extensão por compilação/registro estático ou libs dinâmicas de codec externas (libx264 etc.) |
| **SECURITY** | Programa fuzzing ativo (oss-fuzz), histórico de CVEs em demuxers/decoders; superfície de ataque é o parsing de mídia (grande, de origem não confiável) |
| **LICENSE** | LGPL-2.1 (base) com componentes opcionais GPL-2/3 — verifique `--enable-gpl` vs LGPL ao distribuir |
| **STRENGTHS** | Padrão de facto; cobertura de codecs/formatos incomparável; filtros de vídeo (ass, fade, overlay, scale) e áudio maduros; interoperável com tudo |
| **WEAKNESSES** | API C de baixo nível e verbosa; documentação de filtros fragmentada; codecs patenteados (AAC/HEVC) exigem atenção de licenciamento |
| **TRADE-OFFS** | Onipresença vs controle fino: para o Cosca, FFmpeg resolve I/O/codecs, mas o filtro de "IA" precisa ser um nó externo do grafo (ex.: `hwupload` → Vulkan/OpenCL, ou subprocesso) — não espere DSP/IA embutido |

**Princípio para o Cosca:** Adotar FFmpeg como camada de I/O/codec (demux/decode/preview/encode) e injetar a IA como filtro customizado no grafo (`AVFilter`), preservando A/V sync e reutilizando o ecossistema.

**Armadilha a evitar:** Escrever parsing de mídia próprio ou duplicar codecs "por performance". A superfície de parsing é grande e fuzzada — delegue ao FFmpeg e **não deixe seus filtros de IA receberem bitstream bruto de entrada não confiável** (decode em processo separado/sandbox se servir arquivo arbitrário).

---

## Projeto: GStreamer

| PROJECT | GStreamer/gstreamer |
|---|---|
| **CATEGORY** | Vídeo + Áudio — framework de pipeline/grafo de mídia (sources → transforms → sinks) |
| **PURPOSE** | Construir pipelines de processamento de mídia em tempo real ou offline com negociação dinâmica de formato; base do GNOME/WebKit e de muitos players/streamers |
| **ARCHITECTURE** | Core (GObject/GLib) + plugins (`gst-plugins-base/good/bad/ugly`). Elementos (source/transform/sink) conectados por pads com negociação de **caps** (formato/resolução/framerate). Meson build, monorepo com subprojects |
| **LANGUAGE** | C (GObject) no core; plugins em C/C++; bindings GIR para Python (PyGObject) |
| **CORE ALGORITHMS** | Grafo/pipeline com escalonamento dinâmico de threads, negociação de caps, typefinding automático de formatos; integra FFmpeg, x264, VAAPI, Vulkan, OpenCL como elementos |
| **PIPELINES** | `gst-launch-1.0` com sintaxe de pipeline; pipeline = grafo de elementos; `queue`/`tee`/`mixer` controlam fluxo/paralelismo |
| **DATA MODEL** | `GstBuffer` (amostras/frames), `GstCaps` (formato negociado), `GstPad`/`GstElement`/`GstBin`/`GstPipeline` |
| **GPU/CPU/MEMORY** | CPU-first; aceleração por elementos opcionais: **VAAPI** (video), **Vulkan**, **OpenCL** (`gst-plugins-opencl`) — todos compatíveis com ROCm/GPU AMD via drivers Linux |
| **PERFORMANCE** | Bom paralelismo por elemento/queue; overhead de GObject baixo mas não-zero; adequado a streaming/realtime e offline |
| **EXTENSIBILITY** | Modelo de plugin de primeira classe: novo `GstElement` = novo `.so` registrado por typefind/caps — ideal para plugar nós de IA |
| **PLUGIN SYSTEM** | Forte e formal: cada elemento é um plugin descoberto em runtime; build com GPL só com `-Dgpl=enabled` |
| **SECURITY** | Advisories publicados em `security-advisories/`; parsing de mídia fuzzado; modelo de caps reduz erros de formato, mas elementos de parsing continuam a ser a superfície de risco |
| **LICENSE** | LGPL-2.1 core; plugins individuais podem ser GPL/AGPL (opt-in explícito) |
| **STRENGTHS** | Arquitetura de pipeline de mídia mais expressiva do open source; extensão via plugins trivial; ecossistema GNOME maduro; adequado tanto a tempo real quanto a lote |
| **WEAKNESSES** | Curva de aprendizado do modelo de caps/negociação; menos "universal" que FFmpeg em codecs isolados; docs espalhadas entre freedesktop/GitLab |
| **TRADE-OFFS** | Abstração de pipeline ganha expressividade e perde controle fino de baixo nível (vs FFmpeg). Para o Cosca: usar GStreamer **se** a IA for um elemento no grafo; caso contrário FFmpeg basta |

**Princípio para o Cosca:** Se o Cosca precisar de composição de fluxos em tempo real (câmera + microfone + preview + stream), GStreamer é o framework que dá "pipeline IA como elemento" sem reinventar buffering/negociação.

**Armadilha a evitar:** Usar GStreamer para offline transcoding simples em que FFmpeg já resolve — ganha-se abstração, perde-se controle de codec e ganha-se complexidade de build (meson + muitos subprojects).

---

## Projeto: OBS Studio

| PROJECT | obsproject/obs-studio |
|---|---|
| **CATEGORY** | Vídeo — captura, composição de cenas, encoding e streaming em tempo real |
| **PURPOSE** | Live streaming e gravação com composição de cenas em tempo real sobre GPU |
| **ARCHITECTURE** | Core `libobs` (frontend e plugins); renderers backend plugáveis: `libobs-opengl` (Linux), `libobs-d3d11` (Windows), `libobs-metal` (macOS), `libobs-winrt`. Fontes/saídas/encoders como plugins registrados em libobs |
| **LANGUAGE** | C/C++ |
| **CORE ALGORITHMS** | Composição de cenas (grafo de sources/transitions/filters renderizado na GPU), captura (screen/window/game via APIs de SO), mixing de áudio, encoders em hardware/software (NVENC, AMF, VAAPI, QSV, x264) |
| **PIPELINES** | Scene → sources (camadas) → transitions → output (RTMP/HLS/record) — tudo em loop de render de ~60fps; filtros de fonte e de áudio no caminho |
| **DATA MODEL** | `obs_source`, `obs_output`, `obs_encoder`, `obs_scene`/`obs_scene_item`, `obs_filter` — API C pública em `libobs` |
| **GPU/CPU/MEMORY** | GPU-heavy por natureza (composição em GPU). **Linux: OpenGL + VAAPI** (funciona com drivers AMD/ROCm); sem dependência de CUDA no core; encoders NVIDIA são opcionais |
| **PERFORMANCE** | Otimizado para latência de tempo real (dezenas de ms); menos relevante para render offline |
| **EXTENSIBILITY** | API de plugin documentada (`docs/obsproject.com/docs`); filtros de vídeo (shader) e de áudio plugáveis — um filtro de "IA" pode ser um plugin |
| **PLUGIN SYSTEM** | Forte: milhares de plugins (OBS-websocket, shaders, etc.); compilados contra libobs |
| **SECURITY** | `SECURITY.md` presente; SAST (PVS-Studio) no CI; foco de risco: captura de conteúdo e parsing de fontes de terceiros |
| **LICENSE** | GPL-2.0-or-later |
| **STRENGTHS** | Referência em composição de cena em tempo real na GPU; plugin API madura; prova que composição de cenas ≠ render de timeline (modelo de grafo + GPU) |
| **WEAKNESSES** | Foco em live/streaming, não em edição não-linear; curva para contribuir no core; encoders de hardware AMD (AMF) historicamente melhores no Windows do que VAAPI no Linux |
| **TRADE-OFFS** | Tempo real/GPU dedicado vs capacidade de edição/offline. Para o Cosca: OBS é o "motor de composição de cena e streaming", não o "editor de timeline" |

**Princípio para o Cosca:** Separar **composição de cenas em tempo real (GPU)** de **edição não-linear (timeline)** — adotar a arquitetura de grafo de cenas + render GPU do OBS para preview/streaming, não para timeline.

**Armadilha a evitar:** Assumir que "scene graph com shaders" basta para um NLE. OBS não tem timeline, busca/seek, nem render escalável offline — se o Cosca precisar disso, use MLT/FFmpeg como núcleo e OBS-style scene-graph apenas para o estágio de composição live.

---

## Projeto: Olive

| PROJECT | olive-editor/olive |
|---|---|
| **CATEGORY** | Vídeo — editor não-linear (NLE) |
| **PURPOSE** | Editor de vídeo não-linear livre e gratuito com pipeline de render baseado em nós |
| **ARCHITECTURE** | Node-graph: efeitos/composição como nós conectados por arestas avaliados por frame; UI em Qt; render via OpenGL/GLSL |
| **LANGUAGE** | C++17 |
| **CORE ALGORITHMS** | Avaliação de grafo de nós por frame (render DAG), shaders GLSL para efeitos, codecs via FFmpeg, áudio via Qt/PortAudio (e VST para efeitos de áudio) |
| **PIPELINES** | Clipe → cadeia de nós de efeito → saída; render de quadro acionado por necessidade (tanto preview quanto export) |
| **DATA MODEL** | Grafo de nós (Node/NodeGraph), projeto serializado em arquivo próprio; usa FFmpeg para demux/decode |
| **GPU/CPU/MEMORY** | OpenGL para render de preview (compatível com GPUs AMD); encode offline via FFmpeg (x264 etc.) |
| **PERFORMANCE** | *(não verificado com rigor — projeto em alpha)*; render depende da cadeia de shaders e do codec de origem |
| **EXTENSIBILITY** | Nós definidos no código; sem SDK de plugin de terceiros estável até o momento |
| **PLUGIN SYSTEM** | Fraca/ausente (sem API pública estável de plugins hoje) |
| **SECURITY** | Projeto alpha, sem política de segurança explícita *(não verificado)* |
| **LICENSE** | GPL-3.0 |
| **STRENGTHS** | Arquitetura de render por node-graph é conceitualmente a mais próxima de "pipeline de IA por nós"; ideal como inspiração de design |
| **WEAKNESSES** | **Alpha e "highly unstable" (README oficial)**; sem lançamento estável 1.0; pouco uso em produção; contribuição concentrada |
| **TRADE-OFFS** | Inspiração arquitetural excelente vs risco de basear o Cosca em código instável — use como referência de design, não como base |

**Princípio para o Cosca:** Modelar o pipeline de render como **grafo de nós avaliado por frame** (como o Olive), para que etapas de IA (upscale, interpolação, VFX) sejam nós no mesmo grafo dos efeitos tradicionais.

**Armadilha a evitar:** Forkar ou depender de Olive em produção — projeto em alpha sem estabilidade de API. Extraia o *design* (node graph), não o *código*.

---

## Projeto: Kdenlive

| PROJECT | KDE/kdenlive |
|---|---|
| **CATEGORY** | Vídeo — editor não-linear (NLE) de produção |
| **PURPOSE** | NLE completo para consumidores/prosumers sobre o framework MLT |
| **ARCHITECTURE** | UI em Qt + KDE Frameworks 6; núcleo de edição é o **MLT** (producers/consumers/filters/transitions); efeitos via frei0r (vídeo) e LADSPA (áudio) |
| **LANGUAGE** | C++ |
| **CORE ALGORITHMS** | Timeline com clips/trilhas sobre MLT; preview via MLT/OpenGL; efeitos frei0r; render offline via MLT→FFmpeg |
| **PIPELINES** | Projeto XML (kdenlive) → MLT profiles → clips em trilhas → filtros → consumer (preview/export) |
| **DATA MODEL** | Documento XML de projeto (Kdenlive) que referencia clips e filtros MLT; timeline multi-trilha |
| **GPU/CPU/MEMORY** | CPU para efeitos (frei0r/OpenCV); preview via OpenGL; sem pipeline GPU/ROCm de primeira classe para efeitos |
| **PERFORMANCE** | NLE "bom o suficiente" para uso prosumer; render depende de FFmpeg/x264; efeitos pesados são CPU-bound |
| **EXTENSIBILITY** | Filtros frei0r são plugins binários (compartilhados com outros NLEs); scripts via Python? *(não verificado)*; API de efeitos herdada do MLT |
| **PLUGIN SYSTEM** | Indireta: via frei0r/LADSPA/MLT, não por SDK próprio do Kdenlive |
| **SECURITY** | KDE segue política de segurança coordenada (KDE security); fuzzing de projeto XML presente (validação `validate-xml-files.py`, suíte `fuzzer/`) |
| **LICENSE** | GPL-3.0 (KDE) |
| **STRENGTHS** | NLE open source mais usado em desktop Linux; prova a viabilidade de **NLE = UI + MLT**; boa maturidade de produto |
| **WEAKNESSES** | Dependência forte de KDE Frameworks dificulta reuso como lib; efeitos de IA só entram via frei0r/MLT, sem pipeline GPU/ROCm |
| **TRADE-OFFS** | Maturidade de produto vs reusabilidade como biblioteca — para o Cosca, Kdenlive é *referência de UX de NLE*, não componente embutível |

**Princípio para o Cosca:** Copiar o modelo **"UI fina + núcleo de edição reutilizável (MLT) + efeitos via plugin (frei0r)"** — não embutir o editor na UI.

**Armadilha a evitar:** Seguir Kdenlive cegamente e herdar a limitação de efeitos CPU-only; o Cosca precisa de efeitos de IA com aceleração ROCm — projete o ponto de plugue de efeito para aceitar shader/modelo, não só filtro frei0r.

---

## Projeto: MLT (Meltytech)

| PROJECT | mltframework/mlt |
|---|---|
| **CATEGORY** | Vídeo + Áudio — framework de edição multimídia |
| **PURPOSE** | Framework para NLEs: timeline, producers/consumers/filters/transitions, render offline e preview, programável |
| **ARCHITECTURE** | Modelo `producer → filter/transition → consumer` dirigido por `Mlt::Service`; módulos para FFmpeg, SDL2, Qt/OpenGL, frei0r, LADSPA, Python (binding `libmlt`) |
| **LANGUAGE** | C (core `src/`) + C++ (bindings/`qt`); CLI `melt` |
| **CORE ALGORITHMS** | Composição de timeline (tracks/clips), render em `Mlt::Frame`, efeitos/transições como serviços, busca por frame exato, profiles |
| **PIPELINES** | `melt video.mp4 -attach filter -consumer avformat` — pipelines descritíveis e componíveis; a base do Kdenlive/Shotcut |
| **DATA MODEL** | `Mlt::Producer/Filter/Transition/Consumer/Playlist/Tractor`; XML de projeto padrão |
| **GPU/CPU/MEMORY** | CPU + FFmpeg; OpenGL preview; sem modelo GPU/ROCm de primeira classe para efeitos (frei0r usa OpenCV) |
| **PERFORMANCE** | Suficiente para NLE prosumer; render offline escalável em CPUs |
| **EXTENSIBILITY** | Filtros/transições/producers são serviços registrados — fácil plugar "filter de IA"; bindings Python oficiais (Python é o idioma do Cosca para IA) |
| **PLUGIN SYSTEM** | Sim, via módulos/serviços registrados + bindings (Python) |
| **SECURITY** | Dev container com modo restrito por padrão; nenhuma política formal de segurança encontrada *(não verificado)* |
| **LICENSE** | LGPL-2.1 |
| **STRENGTHS** | É o "núcleo de NLE embutível" do open source (usado por Kdenlive, Shotcut); timeline/composição prontas; LGPL permite uso em produto proprietário; binding Python nativo |
| **WEAKNESSES** | Docs históricas/espalhadas; comunidade pequena; efeitos GPU/IA ficam por conta do Cosca (não existe) |
| **TRADE-OFFS** | Embute-se um núcleo de timeline maduro (MLT) em troca de um modelo de efeito que exige adaptação para IA/GPU |

**Princípio para o Cosca:** **MLT é o candidato nº1 de núcleo de timeline** do Cosca: timeline, composição, preview e bindings Python já resolvidos, com LGPL permissiva para uso comercial.

**Armadilha a evitar:** Adicionar efeitos de IA apenas como "filtro CPU síncrono" no MLT — isso trava o render. Projete os nós de IA como serviços MLT assíncronos (ou externalize via FFmpeg/GPU) para não bloquear o preview.

---

## Projeto: RIFE (video frame interpolation)

| PROJECT | hzwer/ECCV2022-RIFE (antigo MCG-NJU/RIFE; há mirror megvii-research/ECCV2022-RIFE) |
|---|---|
| **CATEGORY** | Vídeo — IA generativa/restauração: interpolação de quadros (frame interpolation / slow-mo) |
| **PURPOSE** | Interpolar quadros intermediários entre dois frames (arbitrary timestep), para slow-motion, up-framerate e suavização de vídeo gerado por IA |
| **ARCHITECTURE** | Rede em PyTorch: fluxo óptico (IFNet) + módulo de warp/fusão; pesos pré-treinados baixados à parte; scripts `inference_video.py` / `inference_img.py` |
| **LANGUAGE** | Python (PyTorch); ports nativos em C++/Vulkan (rife-ncnn-vulkan, vs-mlrt) |
| **CORE ALGORITHMS** | Estimação de fluxo intermediário (intermediate flow) — IFNet leve + síntese de quadro no tempo arbitrário (RIFEm); perda L1/laplaciano; distilação privilegiada |
| **PIPELINES** | `inference_video.py --exp=1/2 --video=...` → frames PNG → modelo → frames interpolados → FFmpeg re-encoda |
| **DATA MODEL** | Imagens (tensores NHWC/RGB em PyTorch); vídeo tratado como sequência de frames |
| **GPU/CPU/MEMORY** | **PyTorch CUDA-first** (30+ FPS 2×720p em 2080Ti). No ROCm: PyTorch com builds ROCm (gfx1030) executa; alternativa robusta é o port **rife-ncnn-vulkan** (Vulkan → roda em GPUs AMD/ROCm sem CUDA) |
| **PERFORMANCE** | Estado da arte em speed/qualidade para VFI em tempo quase real; modelos v4.x adicionam suporte a anime |
| **EXTENSIBILITY** | Reutilizável como módulo PyTorch (pip/import); portas Vulkan para integração nativa |
| **PLUGIN SYSTEM** | Não há (é um modelo, não um framework) — integra-se por código |
| **SECURITY** | Pesos baixados de drive externo (Google Drive) — risco de supply-chain se mal escaneados; código MIT sem auditoria formal |
| **LICENSE** | MIT |
| **STRENGTHS** | SOTA prático em VFI; MIT; fácil de plugar como estágio de pós-processamento de vídeo (incl. vídeos de diffusion models); port Vulkan para AMD sem CUDA |
| **WEAKNESSES** | Manutenção está concentrada (README admite modelo rejeitado 4× em CVPR antes de aceite ECCV); artefatos em cenas complexas; áudio é descartado no pipeline slomo (re-sync manual) |
| **TRADE-OFFS** | Qualidade/velocidade excelente vs dependência de PyTorch (peso) e comportamento de artefatos; use o port Vulkan se o Cosca quiser rodar nativo em ROCm |

**Princípio para o Cosca:** Adotar **RIFE como estágio de interpolação/slow-mo** no pipeline de vídeo, invocável por processo (CLI/port Vulkan) para não acoplar o core do Cosca ao PyTorch.

**Armadilha a evitar:** Rodar RIFE com pesos baixados sem pinning/verificação e sem pinning de versão do PyTorch; e **não esquecer de remixar o áudio** após interpolação (o pipeline oficial remove o áudio).

---

## Projeto: libsndfile

| PROJECT | libsndfile/libsndfile |
|---|---|
| **CATEGORY** | Áudio — I/O de arquivos de som amostrados |
| **PURPOSE** | Ler/escrever arquivos de áudio amostrado (WAV, AIFF, FLAC, OGG/Vorbis, Opus, MP3 via libs externas) de forma portável |
| **ARCHITECTURE** | Biblioteca C pura, API `sf_open/sf_read/sf_write`; mapeia formatos via código-fonte, com codecs opcionais externos (FLAC/OGG/Opus/MP3) |
| **LANGUAGE** | C (C99) |
| **CORE ALGORITHMS** | Descodificação/codificação PCM float/int, reamostragem/dither básicos, suporte a formatos de 8/16/24/32-bit e float32/64 |
| **PIPELINES** | Arquivo → stream de frames (float) → (processamento no app) → stream → arquivo |
| **DATA MODEL** | `SF_INFO` (rate/channels/format) + buffers de amostras float/interleaved |
| **GPU/CPU/MEMORY** | CPU puro; uso de memória mínimo |
| **PERFORMANCE** | Muito rápida para I/O PCM; overhead desprezível |
| **EXTENSIBILITY** | API C estável; pode-se adicionar formatos; bindings disponíveis (soundfile/pysoundfile para Python) |
| **PLUGIN SYSTEM** | Não possui plugin system — extensão via codecs linkados |
| **SECURITY** | `SECURITY.md` + fuzzing (ossfuzz); projeto liderado por time dedicado desde 1.0.30 |
| **LICENSE** | LGPL-2.1 |
| **STRENGTHS** | Padrão de facto para I/O de áudio em C/Python; API simples; formatos principais cobertos; segura (fuzzed) |
| **WEAKNESSES** | Sem streaming longo com metadados ricos; sem suporte a codecs lossy premium (AAC, WMA) além de MP3 |
| **TRADE-OFFS** | Simplicidade/solidez vs escopo: para áudio de alta fidelidade PCM/FLAC é perfeito; para AAC/formatos fechados precisará do FFmpeg |

**Princípio para o Cosca:** Usar **libsndfile (via `soundfile`/`pysoundfile` em Python) para I/O de áudio amostrado** no pipeline de IA — leitura/escrita float nativa e determinística.

**Armadilha a evitar:** Usar libsndfile para decode de formatos que ele não cobre (AAC/AC3/feeds de streaming) — o Cosca terá pipeline duplo: libsndfile para PCM/FLAC e FFmpeg para o resto.

---

## Projeto: PortAudio

| PROJECT | PortAudio/portaudio |
|---|---|
| **CATEGORY** | Áudio — I/O de áudio em tempo real (captura/reprodução) |
| **PURPOSE** | Camada portável de áudio I/O cross-platform com callback de tempo real |
| **ARCHITECTURE** | API C única sobre múltiplos host APIs: `src/hostapi/{alsa,pulseaudio,jack,asio,coreaudio,wasapi,dsound,wmme,oss,sndio,...}`; núcleo comum em `src/common` |
| **LANGUAGE** | C |
| **CORE ALGORITHMS** | Callback-driven I/O com buffers de tamanho fixo (blocos de 128–1024 frames); blocking read/write; conversão de formato interno p/ nativo |
| **PIPELINES** | Callback de áudio (`PaStreamCallback`) → buffer de frames → processamento do app (efeitos/IA) → buffer de saída |
| **DATA MODEL** | `PaStream`, `PaStreamParameters`, buffers float32/16-bit |
| **GPU/CPU/MEMORY** | CPU; overhead baixíssimo; adequado a thread de áudio dedicada |
| **PERFORMANCE** | Latência configurável (pa_minlat); CPU load mensurável via `Pa_GetCPULoad()` |
| **EXTENSIBILITY** | API C estável há 20+ anos; usado por Audacity, etc.; bindings em várias linguagens |
| **PLUGIN SYSTEM** | Não possui plugin system (é biblioteca) |
| **SECURITY** | Sem política formal de segurança destacada; superfície pequena (I/O de dispositivo) |
| **LICENSE** | Licença permissiva estilo MIT (modificada, com cláusula de atribuição) |
| **STRENGTHS** | Portabilidade real (Linux/macOS/Win); callback de tempo real testado em produção (Audacity); latência baixa |
| **WEAKNESSES** | Sem gerenciamento automático de dispositivos (enumerar/abrir); ALSA no Linux é instável sob hotplug; sem suporte a MIDI |
| **TRADE-OFFS** | Controle explícito e latência baixa vs conveniência: para o Cosca, PortAudio é o caminho para captura/playback de baixa latência, não para processamento batch |

**Princípio para o Cosca:** Para **captura/playback de áudio em tempo real** (monitoramento, gravação, streaming), usar PortAudio com callback de baixa latência — mantendo o processamento de IA fora da thread de áudio (fila lock-free).

**Armadilha a evitar:** Colocar inferência de IA síncrona dentro do callback de áudio — quebras de tempo real e glitches. Projete o callback apenas para copiar buffers para uma fila de processamento.

---

## Projeto: Pedalboard (Spotify)

| PROJECT | spotify/pedalboard |
|---|---|
| **CATEGORY** | Áudio — biblioteca Python de efeitos/processamento (DSP + host de plugins) |
| **PURPOSE** | Aplicar efeitos de áudio (EQ, compressão, reverb, pitch shift, VST3/AU) a partir de Python; usado pela Spotify para data augmentation de ML |
| **ARCHITECTURE** | Binding pybind11 sobre **JUCE** (núcleo DSP) + VST3/AU host; `Pedalboard` = cadeia de `Plugin`; `AudioFile` para I/O; `AudioStream` para tempo real |
| **LANGUAGE** | C++ (núcleo) + Python (API) |
| **CORE ALGORITHMS** | Filtros (Biquad/Ladder), dinâmica (Compressor/Limiter), espaciais (Convolution/Reverb/Delay), PitchShift (Rubber Band), lossy (MP3/GSM), resample |
| **PIPELINES** | Cadeia sequencial de plugins (ou árvores paralelas com `Mix`); processamento em chunks com estado reset-ável |
| **DATA MODEL** | NumPy arrays float32 (n_channels × n_samples) |
| **GPU/CPU/MEMORY** | CPU multithread; **libera o GIL** para usar múltiplos cores; sem dependência de GPU |
| **PERFORMANCE** | Até ~300× mais rápido que pySoX; 2–5× vs sox bindings; I/O até 4× mais rápido que librosa.load |
| **EXTENSIBILITY** | Carrega plugins VST3/AU externos via `load_plugin`; biblioteca de efeitos nativos extensível |
| **PLUGIN SYSTEM** | Suporta VST3/AU de terceiros (host); efeitos nativos novos exigem compilação |
| **SECURITY** | Depende de JUCE/VST3 SDK (GPLv3); sem política de segurança formal encontrada *(não verificado)* |
| **LICENSE** | **GPL-3.0** (devido ao JUCE GPLv3 — forte copyleft; risco para produto proprietário) |
| **STRENGTHS** | Qualidade de DSP de estúdio em Python; integra VST3/AU (ecossistema enorme); performance e thread-safety excelentes |
| **WEAKNESSES** | **GPL-3.0 vaza para o código que o usa** (JUCE obriga); wheels manylinux não cobrem todas as distros; sem suporte oficial para efeitos IA/GPU |
| **TRADE-OFFS** | Qualidade+velocidade+ecossistema VST vs copyleft GPLv3 — para o Cosca (possivelmente proprietário), use com cuidado de licença ou isole em serviço separado |

**Princípio para o Cosca:** Modelar a **cadeia de efeitos de áudio como "pedalboard"** (lista de plugins com estado reset-ável, processamento chunkado em float32) — ótimo para data augmentation de treino de IA.

**Armadilha a evitar:** Incorporar pedalboard diretamente num produto fechado (GPL-3.0 via JUCE). Prefira isolar como serviço/contêiner ou substituir efeitos nativos por implementação própria.

---

## Projeto: Librosa

| PROJECT | librosa/librosa |
|---|---|
| **CATEGORY** | Áudio — análise de áudio/música (MIR), extração de features |
| **PURPOSE** | Extração de features de áudio (STFT, mel, chroma, MFCC, beats, onset, pitch, HPSS) para MIR e pré-processamento de ML |
| **ARCHITECTURE** | Python sobre numpy/scipy/soundfile/num_ba; módulos por feature; foco em composição e documentação científica |
| **LANGUAGE** | Python (com kernels num_ba JIT) |
| **CORE ALGORITHMS** | STFT/DCT (FFT), mel-scale, chroma, MFCC, beat tracking (temporal autocorrelation), onset strength, HPSS (harmonic/percussive), de/composição, estiramento de tempo simples |
| **PIPELINES** | `librosa.load` (via soundfile) → feature (ex.: `stft`, `melspectrogram`) → tensor → modelo ML |
| **DATA MODEL** | NumPy arrays; convenções sample_rate, hop_length, n_fft |
| **GPU/CPU/MEMORY** | CPU (numpy/num_ba); sem aceleração GPU própria — feats alimentam modelos em GPU externamente |
| **PERFORMANCE** | Rápida para análise em batch; mais lenta que C nativo em STFT de alta resolução, mas suficiente para MIR |
| **EXTENSIBILITY** | Biblioteca Python fácil de importar/estender; funções composíveis |
| **PLUGIN SYSTEM** | Não possui plugin system (biblioteca) |
| **SECURITY** | Código científico com CI/coverage; sem política de segurança formal destacada *(não verificado)* |
| **LICENSE** | ISC (permissiva) |
| **STRENGTHS** | Padrão de facto para features de áudio em ML; API ergonômica; documentação e citação científica; permissiva |
| **WEAKNESSES** | Leitura de áudio lenta vs pedalboard (I/O próprio); features em CPU podem virar gargalo em batch grande |
| **TRADE-OFFS** | Conveniência científica vs performance de I/O — use `soundfile`/`pedalboard` para load pesado e librosa só para features |

**Princípio para o Cosca:** Usar **librosa como biblioteca de features de áudio** (mel/MFCC/chroma/onset) para alimentar modelos de IA de MIR e análise — código científico estável e permissivo.

**Armadilha a evitar:** Usar `librosa.load` como I/O de áudio padrão do Cosca (é o elo lento). Carregue com `soundfile`/FFmpeg e deixe librosa só para feature extraction.

---

## Projeto: Audacity

| PROJECT | audacity/audacity |
|---|---|
| **CATEGORY** | Áudio — editor de áudio (aplicativo) |
| **PURPOSE** | Editor/gravador de áudio multi-trilha para desktop |
| **ARCHITECTURE** | Aplicativo C++/wxWidgets; núcleo em transição: **Audacity 4 (master)** reescreve UI e backend (submódulo **MuseScore Muse**); Audacity 3.x mantém o clássico |
| **LANGUAGE** | C++ |
| **CORE ALGORITHMS** | Edição não-destrutiva de áudio, efeitos (LV2/VST3/VST/AU/LADSPA/Nyquist), espectrograma, análise |
| **PIPELINES** | Track → efeito (destrutivo em 3.x; não-destrutivo em 4) → render/export |
| **DATA MODEL** | Projeto próprio (`.aup3`, SQLite-based em 3.x); samples em float |
| **GPU/CPU/MEMORY** | CPU para edição/efeitos; sem pipeline GPU/ROCm |
| **PERFORMANCE** | Bom para edição interativa; efeitos pesados CPU-bound |
| **EXTENSIBILITY** | Efeitos via formatos padrão (LV2/VST3/etc.) — mais de um modo de plugin; Nyquist (Lisp) para scripts |
| **PLUGIN SYSTEM** | Forte via efeitos LV2/VST/AU; **master (4.x) está em reestruturação** e menos amigável a contribuidores |
| **SECURITY** | GPL-3.0; repositório em mudança estrutural, sem política destacada *(não verificado)* |
| **LICENSE** | GPL-3.0 (maioria GPLv2+; VST3 code com exceções) |
| **STRENGTHS** | Referência de UX de editor de áudio; suporte multi-efeito; ecossistema de plugins |
| **WEAKNESSES** | **Mudança estrutural grande (4.x)** — master instável para terceiros; monólito desktop difícil de embutir; sem GPU/IA |
| **TRADE-OFFS** | Aplicativo completo vs componente embutível: Audacity é referência de UX, não biblioteca |

**Princípio para o Cosca:** Extrair de Audacity a **UX e o fluxo de trabalho de edição de áudio** (não-destrutivo, espectrograma, plugins), não o código.

**Armadilha a evitar:** Embasar o Cosca no master do Audacity durante a reescrita 4.x — a API está em fluxo; se adotar algo, fixe num release 3.x estável e isole o backend (ex.: Muse).

---

## Projeto: Ardour

| PROJECT | Ardour/ardour |
|---|---|
| **CATEGORY** | Áudio — DAW (workstation de áudio/MIDI em tempo real) |
| **PURPOSE** | DAW profissional para gravação, mixagem e edição de áudio + MIDI com qualidade de estúdio |
| **ARCHITECTURE** | C++ (GTK/Qt? core + `gtk2_ardour` legacy); núcleo `libs/` (audio engine, mixbus, session); backends de áudio **JACK/ALSA/PulseAudio**; **Lua** embutido para scripts |
| **LANGUAGE** | C++ |
| **CORE ALGORITHMS** | Engine de áudio de tempo real (processamento em blocos), mixbus/summing, tempo/MIDI, automação, session management, plugins LV2/VST/VST3/AU/LADSPA |
| **PIPELINES** | Tracks/buses → inserts (plugins) → faders/pan → master bus → master; tudo em loop de tempo real |
| **DATA MODEL** | Session (projeto) serializado; áudio em arquivos + `libs/` internos; automação por controle |
| **GPU/CPU/MEMORY** | CPU estritamente de tempo real; JACK permite shared-memory para IA processar fora do engine *(não verificado em detalhe)* |
| **PERFORMANCE** | Engine de tempo real maduro (20+ anos); latência baixa via JACK |
| **EXTENSIBILITIES** | **Lua scripting** oficial (sessão/automação); plugins via formatos padrão; JACK como interface de interop (qualquer app pode se conectar) |
| **PLUGIN SYSTEM** | LV2/VST/VST3/AU/LADSPA |
| **SECURITY** | GPL-2.0+; sem política formal destacada; superfície = parsing de áudio/MIDI e scripts Lua *(não verificado)* |
| **LICENSE** | GPL-2.0-or-later (com partes GPL-3 em alguns plugins) |
| **STRENGTHS** | DAW de tempo real mais maduro do open source; JACK interop (o Cosca pode "plugarse" via JACK); Lua para automação |
| **WEAKNESSES** | Aplicação desktop monolítica; aprendizado/UX complexo; sem pipeline de IA/GPU |
| **TRADE-OFFS** | Profundidade profissional vs embarcabilidade — use Ardour/JACK como *interoperabilidade de tempo real* (se o Cosca fizer live), não como núcleo |

**Princípio para o Cosca:** Se o Cosca fizer **áudio em tempo real**, aproveitar o modelo **JACK/ALSA (como Ardour)** — um nó de IA pode ser um "app JACK" conectável ao engine sem tocar no DAW.

**Armadilha a evitar:** Assumir JACK/ALSA como stack de áudio default de todos os usuários — no Linux desktop moderno, PipeWire é o padrão; desenhe uma camada de abstração de backend (PortAudio cobre isso).

---

## Projeto: JUCE

| PROJECT | juce-framework/JUCE |
|---|---|
| **CATEGORY** | Áudio — framework C++ para aplicações/plugins de áudio (VST/AU) e tempo real |
| **PURPOSE** | Desenvolver plugins e apps de áudio cross-platform (VST, VST3, AU, AUv3, LV2, AAX) com DSP e UI |
| **ARCHITECTURE** | Módulos (`juce_audio_processors`, `juce_dsp`, `juce_audio_devices`, `juce_graphics`, ...); **`AudioProcessor::processBlock()`** — callback de tempo real por buffer; build via CMake ou Projucer |
| **LANGUAGE** | C++17 (C++20 para Windows MIDI Services) |
| **CORE ALGORITHMS** | DSP buffer-based (filtros Biquad, FFT `juce::dsp::FFT`, convolução, resample), processamento por blocos com latência/estado por parâmetros; hosting de plugins |
| **PIPELINES** | Audio buffer → `processBlock` (efeitos/geração) → output; execução em thread de áudio dedicada de tempo real |
| **DATA MODEL** | Buffers `juce::AudioBuffer<float>`, `AudioProcessorParameter` (parametrizável/automatável) |
| **GPU/CPU/MEMORY** | CPU; tempo real rigoroso (sem alocação/bloqueio no callback); sem dependência GPU |
| **PERFORMANCE** | Benchmark de referência em DSP C++; custo zero de abstração razoável |
| **EXTENSIBILITIES** | Plugins de terceiros via formato padrão; módulos para UIs; integração com CMake |
| **PLUGIN SYSTEM** | É o framework que *gera* plugins (VST/AU/LV2/AAX); pode também *hostar* |
| **SECURITY** | Madura (empresa JUCE), mas licença é o ponto: GPLv3 OU comercial |
| **LICENSE** | **Dual: GPL-3.0 OU licença comercial** — qualquer produto proprietário precisa da licença comercial |
| **STRENGTHS** | Padrão da indústria para plugins de áudio; DSP tempo real sólido; ecossistema VST/AU completo; suporte a todas as plataformas |
| **WEAKNESSES** | C++ não é o stack do Cosca (Python); custo de licença comercial; curva alta |
| **TRADE-OFFS** | Potência de tempo real/ecossistema de plugins vs linguagem/licença: para o Cosca, JUCE é útil apenas se houver necessidade de **componente nativo de áudio** — caso contrário fique em Python |

**Princípio para o Cosca:** Se precisar de um **processador de áudio em tempo real nativo ou plugins VST/AU próprios**, JUCE é a arquitetura de referência — mas avalie custo/licença (GPLv3 ou comercial) e a separação **thread de áudio vs thread de IA**.

**Armadilha a evitar:** Tratar a thread de áudio como lugar para IA pesada. JUCE (e todo áudio tempo real) exige código sem alocação/locks no callback — a inferência deve rodar em outra thread com filas lock-free.

---

## Projeto: Demucs (hybrid transformer)

| PROJECT | facebookresearch/demucs (**arquivado** — fork ativo: adefossez/demucs) |
|---|---|
| **CATEGORY** | Áudio — IA: separação de stems (vocais, drums, bass, other) |
| **PURPOSE** | Separar fontes musicais (stem separation) com qualidade SOTA — base para vocal isolation, karaoke, remix |
| **ARCHITECTURE** | PyTorch; **Hybrid Transformer (htdemucs v4)**: U-Net dupla — um ramo waveform, um espectral — com Transformer cross-domain no gargalo; separação por janelas sobrepostas |
| **LANGUAGE** | Python (PyTorch); `torchaudio`/FFmpeg para I/O |
| **CORE ALGORITHMS** | U-Net convolucional temporal + STFT branch + cross-attention de domínios; "shift trick"; overlap-add; SDR 9.0 dB (MUSDB-HQ) com fine-tuning; modelos 4-stem e 6-stem (experimental) |
| **PIPELINES** | `demucs audio` → resample 44.1k → janelas com overlap → modelo → stems (drums/bass/other/vocals) → wav (int16/24/float32) |
| **DATA MODEL** | Tensores waveform (C×T) e espectro; I/O via torchaudio/FFmpeg |
| **GPU/CPU/MEMORY** | **CUDA-first**; 3–7 GB VRAM (configurar `--segment`); **ROCm**: builds PyTorch ROCm (gfx1030) executam Demucs — mesma arquitetura; CPU é ~1.5× o tempo real da música |
| **PERFORMANCE** | SOTA em qualidade (melhor que Spleeter/Open-Unmix); trade-off: mais pesado que Spleeter |
| **EXTENSIBILITIES** | API Python simples (`demucs.separate.main`); modelos plugáveis por checkpoint; Integração em plugins reais (Neutone) |
| **PLUGIN SYSTEM** | Não possui plugin system (pacote Python) |
| **SECURITY** | MIT; repo **arquivado (jan/2025)** — manutenção parada no original; fork adefossez/demucs recebe correções |
| **LICENSE** | MIT |
| **STRENGTHS** | Qualidade de separação de referência; MIT; uso fácil via pip/CLI/API |
| **WEAKNESSES** | Repositório oficial arquivado (risco de supply-chain/abandono); modelo 6-stem tem `piano` com artefatos; pesado para batch grande sem GPU |
| **TRADE-OFFS** | Qualidade SOTA vs peso computacional e manutenção: fixe a versão/pin dos checkpoints e prefira o fork mantido; avalie Spleeter se precisar de leveza |

**Princípio para o Cosca:** Adotar **Demucs (htdemucs) como motor de separação de stems**, executado em PyTorch-ROCm (gfx1030) ou CPU, como serviço assíncrono — nunca síncrono na thread de áudio.

**Armadilha a evitar:** Depender do repo original arquivado para correções/segurança — **pinar versão e checkpoints com hash** e considerar o fork mantido (adefossez/demucs); cuidado com vazamento de memória em GPU em lote (use `--segment`).

---

## Projeto: Whisper (ASR)

| PROJECT | openai/whisper |
|---|---|
| **CATEGORY** | Áudio — IA: reconhecimento de fala (ASR) / transcrição / tradução |
| **PURPOSE** | Transcrever fala em múltiplos idiomas, traduzir fala→inglês, identificar idioma; multitasking num único modelo seq2seq |
| **ARCHITECTURE** | Transformer encoder-decoder (seq2seq) multitarefa: tarefas são tokens especiais no decoder (ASR, translate, language ID, VAD); janela deslizante de 30 s; log-Mel spectrogram de entrada |
| **LANGUAGE** | Python (PyTorch) + tiktoken; requer **FFmpeg** para decodificação |
| **CORE ALGORITHMS** | Transformer (encoder/decoder com attention), tokenização BPE (tiktoken), janela de 30 s com padding, decoding autogressivo; modelos tiny→large + turbo (otimizado, ~8× mais rápido que large) |
| **PIPELINES** | `whisper audio` → ffmpeg p/ float32 16 kHz → log-Mel 80 bins → encoder → decoder autoregressivo (30 s chunks) → texto/timestamps |
| **DATA MODEL** | Tensores log-Mel (80 mels × 3000 frames p/ 30 s); texto + segmentos com timestamps |
| **GPU/CPU/MEMORY** | **PyTorch CUDA-first** (VRAM ~1–10 GB conforme tamanho). **ROCm**: builds PyTorch ROCm (gfx1030) rodam Whisper sem alteração de código; alternativa leve: **whisper.cpp** (CPU/Vulkan → roda em qualquer GPU AMD) |
| **PERFORMANCE** | SOTA em robustez multi-idioma; turbo ≈ 8× more rápido que large com perda mínima; CPU viável para tiny/base |
| **EXTENSIBILITIES** | API Python (`load_model`, `transcribe`); modelos por tamanho/idioma; ports nativos (whisper.cpp, faster-whisper); finetunable |
| **PLUGIN SYSTEM** | Não possui plugin system (pacote/modelo) |
| **SECURITY** | MIT (código + pesos); sem política de segurança formal; pesos públicos — risco de prompt/entrada não confiável baixo, mas transcrição de conteúdo sensível requer cuidado de privacidade |
| **LICENSE** | MIT |
| **STRENGTHS** | Referência em ASR robusto multilíngue; MIT incluindo pesos; roda em ROCm via PyTorch ou via whisper.cpp (Vulkan) |
| **WEAKNESSES** | Janela fixa 30 s complica streaming de baixa latência; decodificação autogressiva lenta sem GPU; timestamps aproximados (hallucinação em silêncio/música em alguns casos) |
| **TRADE-OFFS** | Robustez/qualidade vs latência e custo de GPU — para legendagem/transcrição batch, use turbo em ROCm; para streaming de baixa latência, avalie faster-whisper/whisper.cpp |

**Princípio para o Cosca:** Adotar **Whisper (turbo/large-v3) em PyTorch-ROCm (gfx1030) como pipeline ASR** para legendagem automática e busca de conteúdo — MIT cobre pesos, ideal para produto.

**Armadilha a evitar:** Usar a janela de 30 s para streaming em tempo real sem planejamento — e **fixar o modelo/versão** (o repo evolui; turbo não traduz). Para latência crítica, não rode o decoder autogressivo na thread de UI.

---

## SÍNTESE DA CATEGORIA

### 8 princípios fortes para o Cosca

1. **I/O e codecs sempre via FFmpeg** (demux/decode/encode, mux, preview) — nunca reescrever parsing; integrar IA como `AVFilter` ou subprocesso, preservando A/V sync.
2. **Timeline sobre MLT** (ou equivalente): composição de trilhas, preview, busca por frame e render offline já resolvidos; LGPL permite produto comercial; binding Python nativo.
3. **"Pipeline IA = grafo de nós avaliado por frame"** (design do Olive): upscale/interpolação/VFX como nós do mesmo grafo dos efeitos tradicionais, sem forkar código alpha.
4. **UI fina + núcleo embutível** (padrão Kdenlive/Shotcut): separar UX (Qt/web) do motor de edição — o Cosca deve embutir motor, não um NLE monolítico.
5. **Composição de cena em tempo real na GPU** (padrão OBS): scene-graph + shaders para preview/streaming, separado do render de timeline; rodar em VAAPI/OpenGL/Vulkan (AMD-friendly).
6. **Thread de áudio de tempo real nunca bloqueada por IA**: callbacks (PortAudio) só copiam buffers para filas lock-free; DSP pesado (Demucs/Whisper/RIFE) roda em serviços assíncronos separados.
7. **ROCM/OpenCL/Vulkan como primeira escolha de aceleração** (gfx1030): preferir VAAPI/Vulkan/OpenCL (FFmpeg, GStreamer, OBS, rife-ncnn-vulkan, whisper.cpp) e builds PyTorch-ROCm para modelos (RIFE, Demucs, Whisper) — **evitar CUDA-only como dependência central**.
8. **Feature extraction e I/O de áudio determinísticos**: `soundfile`/libsndfile para PCM/FLAC, librosa só para features, pedalboard-style chain para data augmentation (com atenção à licença).

### 8 armadilhas comuns

1. **Embutir NLE/DAW monolítico** (Audacity 4.x, Kdenlive, Ardour) em vez de usar o núcleo embutível (MLT) — APIs em reescrita ou aplicações desktop não-componentizáveis.
2. **Forkar projeto alpha/arquivado** (Olive "highly unstable"; Demucs arquivado) — use como referência de design ou pin version/checkpoints com hash; prefira forks mantidos.
3. **Presumir GPU = CUDA**: modelos PyTorch "CUDA-first" (RIFE, Demucs, Whisper) rodam em ROCm com builds apropriados — mas valide `gfx1030` antes de prometer; senão use ports Vulkan (ncnn, whisper.cpp).
4. **Rodar IA no callback de áudio ou na thread de UI** — glitches, perda de tempo real e UI travada; desenhe filas lock-free e serviços.
5. **Perder o sync áudio/vídeo em pós-processamento IA**: RIFE descarta áudio, interpolação muda duração — pipeline deve re-encodar e remuxar com áudio correto.
6. **Subestimar licenças**: JUCE (GPLv3 ou comercial), pedalboard (GPLv3), GStreamer plugins GPL/AGPL, FFmpeg `--enable-gpl` — mapeie licença por componente antes de definir modelo de distribuição do Cosca.
7. **I/O lento como gargalo**: `librosa.load` é elo lento vs `soundfile`/pedalboard; e libsndfile não cobre AAC/AC3 — defina pipeline duplo (libsndfile + FFmpeg) desde o início.
8. **Aceitar bitstream não confiável em parsers próprios**: parsing de mídia é a maior superfície de CVEs (FFmpeg fuzza); sandboxize decode e valide pesos pré-treinados baixados (supply-chain).

---

*Nota metodológica: informações coletadas por leitura dos READMEs, estruturas de repositório e licenças em ago/2026. RIFE encontrado em `hzwer/ECCV2022-RIFE` (repo original `MCG-NJU/RIFE` redireciona/404; mirrors em `megvii-research/ECCV2022-RIFE`). Kdenlive em `KDE/kdenlive` (endereço dado no briefing redireciona para repo vazio). Campos sem confirmação documental estão marcados como "(não verificado)".*
