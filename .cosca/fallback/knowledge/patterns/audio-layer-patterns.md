# Camada 8: 🎵 Audio — O Mundo Tem Som

> **Version**: 1.0.0 | **Confidence**: 0.84 | **Category**: Audio/Speech/Music | **Created**: 2026-08-23 | **Source**: GitHub API (stars/license) + conhecimento + template do professor

> **Mined by**: cosca-kernel (ordem do Don). **Gap #4 do mining-map-world-vivo.md.** O mundo precisa de som: o agente fala, ouve, e o ambiente tem música adaptativa e sons espaciais. Template: REPO→PAPER→MODEL→LICENSE→DEPS→BENCHMARK→INTEGRATION→PLUGIN→UTILITY.

## Purpose

Minar a camada **Audio** do Mundo Vivo. O agente precisa: (1) ouvir o mundo (STT), (2) falar (TTS), (3) gerar música/ambiente, (4) entender sons. Cada projeto abaixo é uma **capacidade de áudio** que o Cosca pode dar ao agente e ao mundo.

---

## 1. Whisper (speech recognition)

### REPO
`openai/whisper` — ★28k — MIT — Python

### PAPER/MÉTODO
Whisper é um modelo de **reconhecimento de fala** (STT) treinado em 680k horas de dados multilíngues. Arquitetura: encoder-decoder transformer. Input: áudio (16kHz) → Output: texto transcrito. Suporta: transcrição, tradução, detecção de idioma.

### MODELO
tiny/base/small/medium/large-v3. Pesos open-source.

### LICENSE
MIT ✅ (tudo liberado)

### DEPENDÊNCIAS
PyTorch, numpy, ffmpeg. Leve (tiny: ~1GB GPU, large: ~10GB GPU).

### BENCHMARK
State-of-the-art em STT (CommonVoice, LibriSpeech). Large-v3: 2-5% WER em múltiplos idiomas.

### INTEGRAÇÃO NO COSCA
Whisper é os **ouvidos do agente** — ele ouve o mundo e entende o que dizem. Para o Cosca: input de voz do usuário, conversação com NPCs, comandos de voz. Pode rodar localmente (privacidade) ou via API.

### POSSÍVEL PLUGIN
`cosca-audio-whisper` — plugin que recebe áudio (PCM/WAV) → retorna texto transcrito + idioma + timestamps. Input: stream de áudio. Output: texto + metadata.

### UTILIDADE NO COSCA
**Muito Alta.** Sem STT, o agente é surdo. Whisper é o **padrão open-source** — MIT, multilíngue, preciso. Essencial para conversação e comandos de voz.

---

## 2. Whisper.cpp / faster-whisper (STT rápido)

### REPO
- `ggml-org/whisper.cpp` — ★53k — MIT — C++
- `SYSTRAN/faster-whisper` — ★25k — MIT — Python (CTranslate2)

### PAPER/MÉTODO
whisper.cpp é uma **porta C++ puro** do Whisper (sem PyTorch) — roda em CPU, Raspberry Pi, mobile. faster-whisper usa CTranslate2 para ser 4x mais rápido que o original com mesma qualidade.

### MODELO
Mesmos pesos do Whisper (tiny→large-v3).

### LICENSE
MIT ✅

### DEPENDÊNCIAS
whisper.cpp: ggml (C, sem dependências externas). faster-whisper: CTranslate2, PyTorch.

### BENCHMARK
whisper.cpp: real-time em CPU (medium em laptops). faster-whisper: 4x mais rápido que whisper original.

### INTEGRAÇÃO NO COSCA
whisper.cpp é o **STT que roda em qualquer lugar** — sem GPU, sem PyTorch, sem dependências pesadas. Para o Cosca: se o Don quiser rodar em hardware leve (Raspberry Pi, laptop antigo), whisper.cpp é a escolha. faster-whisper é o melhor para server (GPU).

### POSSÍVEL PLUGIN
`cosca-audio-whisper-cpp` — plugin leve que roda STT em CPU (sem GPU).

### UTILIDADE NO COSCA
**Alta.** whisper.cpp é o **STT portátil** — garante que o agente possa ouvir em qualquer hardware.

---

## 3. Coqui TTS (text-to-speech + voice cloning)

### REPO
`coqui-ai/TTS` — ★37k — MPL-2.0 — Python

### PAPER/MÉTODO
Coqui TTS é um framework de **text-to-speech** com suporte a voice cloning. Modelos: VITS, VITS2, XTTS (cross-lingual voice cloning), Tacotron2, FastPitch. O XTTS pode clonar uma voz a partir de 6 segundos de áudio.

### MODELO
XTTS-v2, VITS, Tacotron2, FastPitch, HiFi-GAN (vocoder). Pesos open-source.

### LICENSE
MPL-2.0 ✅ (modificações devem ser open-source; uso comercial permitido)

### DEPENDÊNCIAS
PyTorch, transformers. Moderado (~2GB GPU para XTTS).

### BENCHMARK
XTTS: state-of-the-art em voice cloning cross-lingual. VITS: naturalness state-of-the-art.

### INTEGRAÇÃO NO COSCA
Coqui TTS é a **voz do agente** — ele fala com o mundo. O XTTS pode clonar vozes (o agente pode falar como alguém específico). Para o Cosca: vozes de NPCs, narração, feedback de voz.

### POSSÍVEL PLUGIN
`cosca-audio-tts` — plugin que recebe texto + voz (opcional) → retorna áudio falado. Input: texto + voice sample. Output: WAV/PCM.

### UTILIDADE NO COSCA
**Muito Alta.** Sem TTS, o agente é mudo. Coqui é o **TTS open-source mais completo** — voice cloning, multilíngue, natural.

---

## 4. Bark (TTS expressivo)

### REPO
`suno-ai/bark` — ★36k — MIT — Python

### PAPER/MÉTODO
Bark é um modelo de **TTS expressivo** que gera áudio realista com emoção, risadas, música de fundo, e sons não-verbais. Arquitetura: transformer autoregressivo (GPT-like). Suporta: fala multilíngue, efeitos sonoros, música.

### MODELO
Bark small. Pesos open-source.

### LICENSE
MIT ✅

### DEPENDÊNCIAS
PyTorch, transformers. Pesado (~5GB GPU).

### BENCHMARK
Mais expressivo que Coqui TTS (emoção, efeitos). Menos preciso em pronúncia.

### INTEGRAÇÃO NO COSCA
Bark é o **TTS para emoção e expressividade** — quando o agente precisa não só falar, mas EXPRESSAR (alegria, surpresa, risada). Para NPCs vivos, Bark é melhor que Coqui.

### POSSÍVEL PLUGIN
`cosca-audio-bark` — plugin que recebe texto + emoção → retorna áudio expressivo.

### UTILIDADE NO COSCA
**Média-Alta.** Bark é o **TTS emocional** — complementa Coqui (que é mais preciso). Para NPCs vivos, Bark é essencial.

---

## 5. AudioCraft (music + audio generation)

### REPO
`facebookresearch/audiocraft` — ★22k — MIT — Python

### PAPER/MÉTODO
AudioCraft gera **música, áudio e sons** a partir de texto. Modelos: MusicGen (música), AudioGen (sons), EnCodec (codec neural). MusicGen: transformer que gera áudio a partir de descrição textual + melodia + harmonic. Já minerado em `generative-media-patterns.md`.

### MODELO
MusicGen (small/medium/large), AudioGen, EnCodec. Pesos open-source.

### LICENSE
MIT ✅

### DEPENDÊNCIAS
PyTorch, transformers. Pesado (~4-8GB GPU).

### BENCHMARK
MusicGen: state-of-the-art em music generation from text. AudioGen: state-of-the-art em sound effects.

### INTEGRAÇÃO NO COSCA
AudioCraft é o **som ambiente do mundo** — música de fundo adaptativa, sons de ambiente (pássaros, vento, cidade), efeitos sonoros. Para o Cosca: gera o "som do mundo" dinamicamente.

### POSSÍVEL PLUGIN
`cosca-audio-musicgen` — plugin que recebe descrição → retorna música/áudio ambiente. Input: "calm forest ambient music". Output: WAV.

### UTILIDADE NO COSCA
**Alta.** AudioCraft é o **compositor do mundo** — o mundo tem som porque AudioCraft gera. Já minerado; este doc confirma sua posição na camada Audio.

---

## 6. SenseVoice (audio understanding)

### REPO
`QwenAudio/SenseVoice` — ★9k — MIT — C/Python

### PAPER/MÉTODO
SenseVoice é um modelo de **entendimento de áudio** — classifica sons, detecta emoção em fala, identifica eventos sonoros. Multilíngue (50+ idiomas). Pode ser usado para: "o que esse som é?", "a pessoa está feliz ou triste?", "tem vidro quebrando?".

### MODELO
SenseVoice-small/large. Pesos open-source.

### LICENSE
MIT ✅

### DEPENDÊNCIAS
PyTorch, transformers. Moderado (~2GB GPU).

### BENCHMARK
State-of-the-art em audio understanding (emotion detection, sound classification).

### INTEGRAÇÃO NO COSCA
SenseVoice é o **cérebro auditivo do agente** — ele não só ouve (Whisper transcreve), mas ENTENDE (classifica sons, detecta emoção). Para o Cosca: o agente sabe "isso é um grito" ou "isso é uma risada".

### POSSÍVEL PLUGIN
`cosca-audio-understand` — plugin que recebe áudio → retorna classificação de som + emoção + confiança. Input: WAV. Output: sound class + emotion + score.

### UTILIDADE NO COSCA
**Alta.** SenseVoice é a **inteligência auditiva** — complementa Whisper (transcrição) com entendimento semântico.

---

## 7. YuE (music generation)

### REPO
`multimodal-art-projection/YuE` — ★6.4k — Apache-2.0 — Python

### PAPER/MÉTODO
YuE é um modelo de **geração de música** open-source que suporta letras + melodia. Gera músicas completas (intro, verso, refrão) a partir de texto+letra. Multilíngue.

### MODELO
YuE base/instage. Pesos open-source.

### LICENSE
Apache-2.0 ✅

### DEPENDÊNCIAS
PyTorch, transformers. Pesado.

### BENCHMARK
Competitivo com MusicGen em qualidade. Melhor em estrutura musical (verso/refrão).

### INTEGRAÇÃO NO COSCA
YuE é uma **alternativa ao AudioCraft para música com letra** — se o mundo precisa de música com vocal (canções de taverna, himnos), YuE é melhor que MusicGen.

### POSSÍVEL PLUGIN
`cosca-audio-yue` — plugin que recebe letra + estilo → retorna música completa.

### UTILIDADE NO COSCA
**Média.** YuE é para música com letra específica — complementa AudioCraft (que é mais para ambiente).

---

## Síntese da Camada Audio

| # | Projeto | ★ | License | Capacidade no Cosca |
|---|---------|---|---------|---------------------|
| 1 | **whisper.cpp** | 53k | MIT | STT rápido (roda em qualquer hardware) |
| 2 | **Coqui TTS** | 37k | MPL-2.0 | TTS + voice cloning (a voz do agente) |
| 3 | **Bark** | 36k | MIT | TTS expressivo (emoção, risadas) |
| 4 | **Whisper** | 28k | MIT | STT padrão (multilíngue, preciso) |
| 5 | **faster-whisper** | 25k | MIT | STT 4x mais rápido |
| 6 | **AudioCraft** | 22k | MIT | Música + sons ambiente |
| 7 | **SenseVoice** | 9k | MIT | Entendimento de áudio (emoção, classificação) |
| 8 | **YuE** | 6.4k | Apache-2.0 | Música com letra |

### O pipeline audio do agente e do mundo

```
Mundo
    │
    ├─→ STT (Whisper/whisper.cpp) → "o que dizem?" (transcrição)
    │
    ├─→ SenseVoice → "o que esse som é?" (classificação + emoção)
    │
    ├─→ TTS (Coqui/Bark) → "o agente fala" (voz + expressividade)
    │
    ├─→ AudioCraft → "som ambiente" (música + sons)
    │
    └─→ YuE → "música com letra" (taverna, hino)
```

### Prioridade de instalação (quando o NVMe chegar)
1. **Whisper.cpp** (MIT, CPU, roda em qualquer lugar) — os ouvidos do agente
2. **Coqui TTS** (MPL, a voz do agente) — o agente fala
3. **AudioCraft** (MIT, som ambiente) — o mundo tem som
4. **SenseVoice** (MIT, entendimento) — o agente entende sons
5. **Bark** (MIT, expressividade) — NPCs vivos
6. **faster-whisper** (MIT, server) — STT otimizado
7. **YuE** (Apache, música com letra) — complemento

## Related Patterns

- [`generative-media-patterns.md`](generative-media-patterns.md) — AudioCraft (mineração detalhada já feita)
- [`vision-layer-patterns.md`](vision-layer-patterns.md) — CLIP (visão→linguagem, complementa audio→linguagem)
- [`unreal-integration-patterns.md`](unreal-integration-patterns.md) — Perception (o agente "ouve" no UE)
- [`mining-map-world-vivo.md`](mining-map-world-vivo.md) — Gap #4 (Audio) é esta camada
