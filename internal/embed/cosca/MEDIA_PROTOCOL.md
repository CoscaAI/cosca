# MEDIA PROTOCOL — O cinema da casa (vídeo, áudio, imagem — §16)

> **Versão**: 1.0.0 | **Status**: canônico | **Dono**: cosca-kernel
> **Criado**: 2026-08-16, por ordem do Don — *"analise todos os comandos, falta algum protocolo?"*
> **Propósito**: referência operacional ÚNICA para a Media Engine — o motor de vídeo/áudio
> da casa (ffmpeg, §16) e o ecossistema multimídia em construção (TTS, vídeo generativo,
> cosca-cinema). Nasceu das fases A–B: mineração de padrões (L329), stack de áudio (L331),
> voz (L333), benchmark TTS (L334).

---

## 1. O QUE É a Media Engine

A **Media Engine** (`cosca media`) é o motor de mídia da casa: vídeo/áudio via
**ffmpeg** — sem depender de GUI, sem GUI, headless e determinístico (P7 do L329).

```
cosca media probe <file>          # info completa (ffprobe)
cosca media extract-audio <in> <out>   # trilha de áudio (wav/mp3/aac)
cosca media frame <in> <out> <time>    # frame no instante t
cosca media transcode <in> <out>       # re-encode (crf por qualidade)
cosca media validate <file>            # integridade do media
```

**Regras da casa:**
- **Nunca destruir o original** — todo comando produz um NOVO arquivo.
- Determinismo: mesmo input + mesmo comando = mesmo output (para pipeline).
- FFmpeg é a régua (P13): `ffprobe` antes, `validate` depois de qualquer transform.
- Integridade: `media validate` verifica o arquivo após qualquer processamento.

---

## 2. O ECOSSISTEMA MULTIMÍDIA — o que a casa já tem

| Camada | Stack | Status |
|--------|-------|--------|
| **Áudio DSP** | librosa 1.0.0, soundfile 0.14.0, torchaudio 2.11.0+rocm7.2, ffmpeg-python | ✅ operacional (L331) |
| **Voz (TTS)** | Kokoro 0.9.4 (af_bella en, pf_dora pt-BR) | ✅ operacional (L333) |
| **STT** | faster-whisper 1.2.1 (medium) | ✅ operacional (L333) |
| **TTS daemon** | `~/.cosca/bin/cosca-tts` (porta 14324, systemd user) | 🟡 RTF 2.2x em thread HTTP (pendente) |
| **Vídeo generativo** | Wan2.1 1.3B / LTX-Video 2B (planejado) | ⏳ Fase C |
| **Render** | `cosca render` (deterministic, cacheable, resumable) | ✅ engine existe |

**Pipeline da casa (visão):** *texto → TTS (voz) → vídeo generativo (imagens) →
montagem (ffmpeg filter_complex) → cosca-cinema*. A voz já fala (Fase B); a
Fase C é o vídeo.

---

## 3. ARQUITETURA DE ÁUDIO — lições duras (L331–L334)

| Lição | Detalhe |
|-------|---------|
| **torchcodec NÃO existe p/ ROCm** | o build do PyPI é CUDA (libnvrtc.so.13) — NÃO instalar; I/O WAV via soundfile, DSP GPU via torchaudio.transforms |
| **torchaudio do índice ROCm** | `pip install --index-url https://download.pytorch.org/whl/rocm7.2` — o PyPI entrega build CUDA que quebra |
| **float32 obrigatório** | `sf.read` retorna float64 por default → `dtype='float32'` (senão "double != float" no torchaudio) |
| **ROCm gfx1030** | `HSA_OVERRIDE_GFX_VERSION=10.3.0` obrigatória — sem ela, SEGV (o systemd NÃO herda env do shell) |
| **fp16 direto no Kokoro** | `parameter types mismatch` no LSTM — usar fp32 |
| **autocast fp16 no ROCm** | PIORA (17.5s vs 8.8s) — não usar |
| **Warmup domina** | fria 6.0s (RTF 0.91x) vs quente 0.79s (RTF 0.12x) — modelo RESIDENTE, nunca carregar por chamada |

---

## 4. O DAEMON TTS — a voz da casa

Arquitetura correta (daemon quente): modelo residente em memória, API HTTP.

```bash
# endpoints
GET  /health          # vivo?
GET  /voices          # vozes disponíveis (af_bella, pf_dora...)
POST /tts             # texto → wav (saída em ~/.cosca/audio/tts_*.wav)
```

```bash
systemctl --user status cosca-tts       # unit systemd user
systemctl --user restart cosca-tts
```

**Unit do systemd** exige (env NÃO herdado do shell):
```
Environment=HSA_OVERRIDE_GFX_VERSION=10.3.0
Environment=LD_LIBRARY_PATH=/home/cosca/.local/lib
Environment=CUDA_VISIBLE_DEVICES=0
```

**Pendência conhecida**: síntese em thread do HTTP handler roda RTF ~2.2x vs
0.12x na main thread — investigar warmup na main thread no startup (L335 radar).

---

## 5. PADRÕES DE MÍDIA MINERADOS (L329) — o mapa do futuro

| Padrão | Fonte | Aplicação cosca |
|--------|-------|-----------------|
| **P1 timeline-como-dado** | MLT/Kdenlive | a edição é um JSON serializável, não um estado |
| **P2 nó tipado** | ComfyUI | gráfico de nós com contratos de entrada/saída |
| **P3 cache content-addressed** | ComfyUI | assinatura do nó = chave de cache (ver `cosca ngraph sig`) |
| **P4 caps negotiation** | GStreamer | negociar formatos entre nós antes de conectar |
| **P6 plugins abertos** | LADSPA/LV2/FAUST | DSP plugável sem recompilar |
| **P7 engine headless** | FFmpeg/MLT | tudo roda sem GUI (é o §16) |
| **P8 qualidade contratual** | proxy + métrica | contrato de qualidade na saída (SSIM/VMAF p/ vídeo, MOS p/ voz) |

---

## 6. COMO TRABALHAR COM MÍDIA — o fluxo da casa

1. **Probe primeiro** (P13): `cosca media probe <file>` — codec, resolução, taxa.
2. **Transforma sem destruir**: sempre novo arquivo de saída.
3. **Valida depois**: `cosca media validate <out>` — o que saiu é íntegro.
4. **GPU para DSP**: torchaudio transforms em cuda:0 (float32), nunca CPU se der.
5. **Daemon quente para voz**: TTS residente (warmup pago uma vez).
6. **Registrar**: qualquer mudança de stack/benchmark → aprendizado (L331–L334 são os exemplos).

---

## 7. RELAÇÃO COM OUTROS PROTOCOLOS

| Protocolo | Complemento |
|-----------|-------------|
| `MODEL_PROTOCOL.md` | modelos/tarefas/GPU — onde roda cada peça de mídia |
| `PERFORMANCE_PROTOCOL.md` | "fritar" a Media Engine (RTF, warmup, gargalos) |
| `GRAPH_PROTOCOL.md` | assets de mídia como entidades no grafo |
| `EVIDENCE_PROTOCOL.md` | conteúdo externo de mídia passa pela quarentena |