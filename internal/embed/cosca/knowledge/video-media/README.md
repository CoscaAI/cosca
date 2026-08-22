# 19 — VIDEO / AUDIO / MEDIA ENGINEERING

> Stack 19 da Cosca Engineering Intelligence Matrix.
> Objetivo: **Cosca Video/Media Engineering Intelligence** — uma engine de inteligência e engenharia de mídia, não uma "ferramenta que converte vídeo".

## PRINCÍPIO FUNDAMENTAL
**Vídeo NÃO é "uma sequência de imagens" — é DADO TEMPORAL.**
Considerar: motion · tempo · frames · áudio · sincronização · continuidade de cena · identidade de objeto · consistência temporal.

## REGRA FINAL (o fluxo)
```
UNDERSTAND → ANALYZE → PLAN → PROCESS → TRACK → VALIDATE → OPTIMIZE → EXPORT
```
Sempre preservar: **QUALIDADE + SINCRONIZAÇÃO + METADADOS (quando necessário) + SEGURANÇA + ORIGINAL**.

## VISUAL MEDIA ENGINE (arquitetura conceitual)
```
COSCA → MEDIA ANALYZER → FORMAT DETECTOR → CONTENT ANALYZER → TASK CLASSIFIER
     → PIPELINE PLANNER → [VIDEO | AUDIO | IMAGE] → AI/PROCESSING
     → QUALITY VALIDATOR → SECURITY VALIDATOR → EXPORT
```

## FUNDAMENTOS (entender antes de processar)
### Codecs vs Containers
- **Codec ≠ Container**: H.264/H.265·HEVC/AV1/VP8/VP9 (codecs) vs MP4/MKV/MOV/WebM/TS (containers).
- Entender: bitrate, CRF, QP, GOP, I/P/B-frames, keyframes, motion vectors, entropy coding, qualidade×tamanho.
- **Pipeline decision**: "MP4 compatível com celular" → H.264 + AAC + MP4 + bitrate otimizado.

### GPU vs CPU
- GPU: NVENC/NVDEC, VAAPI, AMF, VideoToolbox, Vulkan, CUDA — hardware decode/encode, zero-copy.
- CPU: x264/x265/rav1e (SVT-AV1) — qualidade/controle.
- Decisão por: VRAM, tamanho de frame, FPS, batch, latência.

## PRINCÍPIOS CORE
1. **Temporal consistency** — video segmentation/background removal usa TRACKING + mask propagation, NÃO frames independentes (evita flickering, mask jitter, edge artifacts; preserva cabelo e motion) · UNIVERSAL
2. **Non-destructive** — `SOURCE + EDIT DECISIONS + EFFECTS + TRANSFORMS + KEYFRAMES + AUDIO MIX + SUBTITLES → RENDER`; nunca destruir o original · UNIVERSAL
3. **Nunca prometer informação que não existe** — upscale/restauração: avaliar detail hallucination, artifacts, faces, texto, bordas, motion · UNIVERSAL
4. **Selecionar SÓ os filtros necessários** — restoration pipeline com quality analysis antes (nunca aplicar tudo) · STRONG
5. **Frame interpolation com consciência de artefatos** — optical flow, motion estimation, occlusion, ghosting · STRONG
6. **ASR → legenda → tradução** — Whisper (timestamps+punctuation) → SRT/VTT/ASS; tradução preservando timestamps · STRONG
7. **Mídia é INPUT NÃO CONFIÁVEL** — malformed containers, codec vulns, decompression bombs, oversized, metadata malicioso, path traversal; processar em sandbox · UNIVERSAL (cross-ref Stack 17)
8. **Performance engineering** — CPU/GPU/VRAM/RAM/disk/network; hardware encode/decode; batch/streaming; parallel · UNIVERSAL
9. **Validar SEMPRE** — resolução, FPS, duração, audio/video sync, bitrate, codec, container, corrupção, dropped frames, artifacts, audio clipping, subtitle sync · UNIVERSAL

## PIPELINES-CHAVE
### Background removal em vídeo
`frame analysis → subject detection → segmentation → temporal tracking → mask propagation → matte refinement → compositing → encode` (consistência temporal).

### Restauração de vídeo antigo
`quality analysis → denoise → deblock → deblur → super resolution → frame restoration → color restoration → audio restoration → validation` (só o necessário).

### Legendas automáticas
`extract audio → speech recognition → timestamps → punctuation → subtitle generation (SRT/VTT/ASS)`.

### Tradução de vídeo
`audio → ASR → language detection → translation → timestamp preservation → subtitle generation` (+ dubbing opcional).

### Thumbnail inteligente
`scene detection → frame extraction → quality scoring → face/object analysis → composition score → best frame`.

### Vídeo OCR
`frame sampling → text detection → OCR → temporal tracking → structured text`.

## MEDIA ANALYSIS ENGINE (capacidade de resposta)
Quantas pessoas/objetos? Quando aparece a cena X? Duração? Onde há fala? Que palavras? Que textos? Qual idioma? Cenas semelhantes? Melhores frames? Silêncio? Onde há cortes?

## MEDIA CONVERSION MATRIX
`video→video · video→audio · video→image · video→GIF · video→WebP · video→subtitles · audio→audio · image→video · subtitle→video · video→structured data · video→transcript`

## ANTI-PATTERNS
`processar frames independentes (sem consistência temporal → flicker)` · `upscale prometendo detalhe inexistente (hallucination)` · `aplicar TODOS os filtros (restoration soup)` · `escolher codec/container às cegas` · `processar vídeo não-confiável sem sandbox` · `destruir o original` · `ignorar audio/video sync no pós` · `ignorar trade-off CPU vs GPU` · `metadados perdidos sem necessidade`

## CHECKLIST (quality gate)
- [ ] Pipeline escolhido pelo conteúdo/tarefa (não default)
- [ ] Consistência temporal (tracking/mask propagation em segmentação)
- [ ] Original intacto (non-destructive)
- [ ] Filtros mínimos necessários (nada de restoration soup)
- [ ] Audio/video sync validado
- [ ] Codec/container/resolução/FPS/bitrate apropriados
- [ ] Sandbox + limites para mídia não-confiável
- [ ] Metadados preservados quando necessário
- [ ] Validação pós-processamento (artifacts, dropped frames, clipping)

## A REGRA
O Cosca deve evoluir de "ferramenta que converte vídeo" para **ENGINE DE INTELIGÊNCIA E ENGENHARIA DE MÍDIA** — entendendo vídeo como dado temporal e preservando qualidade, sincronização, metadados, segurança e o original.

## REFERÊNCIAS
**Core**: FFmpeg · GStreamer · OBS · mpv/VLC
**Codecs**: x264 · x265 · SVT-AV1 · rav1e · libde265 · libavif
**Containers**: Bento4 · Matroska · SRT
**Edição**: Olive · MLT · Kdenlive · Shotcut
**GPU**: NVIDIA video-sdk · oneVPL · AMF · Vulkan
**Restauração/upscale**: Real-ESRGAN · GFPGAN · CodeFormer · SwinIR · BasicVSR++
**Interpolação**: RIFE · DAIN · google frame-interpolation
**Visão**: ultralytics · Detectron2 · MMDetection/MMTracking/MMAction2 · SAM/SAM2/3 · GroundingDINO · rembg/BiRefNet · ByteTrack · BoT-SORT · RAFT · MediaPipe · OpenPose · InsightFace
**Áudio**: Opus · librosa · pedalboard · sox · LUFS
**ASR/Legenda**: Whisper · whisper.cpp · faster-whisper · SubtitleEdit · CCExtractor
**Streaming**: HLS.js · Shaka · dash.js · MediaMTX · SRS · pion/aiortc (WebRTC)
**AI/Orquestração**: ComfyUI · diffusers · transformers · Wan · LTX-Video
