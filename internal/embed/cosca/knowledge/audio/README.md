# 20 — AUDIO ENGINEERING / DSP / MUSIC / SPEECH

> Stack 20 da Cosca Engineering Intelligence Matrix.
> Objetivo: **Cosca Audio Engineering Intelligence** — DSP + DAW + codecs + speech + música + restauração + separação + TTS/STT + MIDI + tempo real + IA.

## PRINCÍPIO FUNDAMENTAL
**Áudio é um sinal no domínio do tempo** — sample rate, canais, bit depth e **sincronização** são a base. Tempo real ≠ offline: latência é a moeda do tempo real.

## REGRA FINAL
```
ANALYZE → PLAN → PROCESS → VALIDATE → PRESERVE
```
De "ferramenta que edita áudio" para **AUDIO ENGINEERING INTELLIGENCE** — entender o sinal antes de processar, aplicar SÓ o necessário, validar e preservar o original.

## PIPELINES INTELLIGENTES (os 3 do Don)
### 1. Áudio ruim → restaurado
`ANALYZE (noise/hum/clipping/voice detection) → DSP PLAN → DENOISE → DEHUM → DECLICK → EQ → DYNAMIC RANGE → LOUDNESS NORMALIZATION (LUFS) → VALIDATION → EXPORT`
**Nunca aplicar todos os filtros** — o plano nasce da análise.

### 2. Vídeo → legendas com falantes
`AUDIO EXTRACTION → WHISPER (ASR) → TRANSCRIPT → SPEAKER DIARIZATION (pyannote) → TIMESTAMPS → SUBTITLE (SRT/VTT/ASS)`

### 3. Música → stems
`DETECT STEMS (demucs) → VOCAL / DRUMS / BASS / OTHER → EDIT / MIX / EXPORT`

## FUNDAMENTOS
### Codecs & formatos
- **Lossless**: WAV/PCM · FLAC — preservação total.
- **Lossy**: MP3 · AAC · Opus · Vorbis — qualidade×tamanho (Opus = melhor latência+qualidade para fala).
- Sample rate, canais, bit depth: escolher pelo destino, não converter às cegas.

### Loudness (LUFS) — o padrão
- Normalização por **LUFS** (percepção), não por pico — broadcast/streaming exigem consistência de loudness.
- Ferramentas: librosa, ffmpeg `loudnorm`.

### Tempo real vs offline
- **Baixa latência** (gravação/efeitos ao vivo): miniaudio, PortAudio, JUCE — buffer size, latency budget, threads de áudio dedicadas.
- **Offline** (restauração/separacão): processamento pesado sem deadline.

## PRINCÍPIOS CORE
1. **Analisar antes de processar** — noise/hum/clipping/voice detection dirige o DSP plan (nunca filtros automáticos) · UNIVERSAL
2. **LUFS como unidade de loudness** (não pico) · STRONG
3. **ASR + diarização** separa quem falou (Whisper + pyannote/silero) · STRONG
4. **Stem separation** (vocal/drums/bass/other) habilita edição/mix profissional · STRONG
5. **Non-destructive** — processar cópia, preservar original + metadados · UNIVERSAL
6. **Áudio é input NÃO CONFIÁVEL** — malformed files, decompression bombs, metadados maliciosos; sandbox · UNIVERSAL (cross-ref Stack 17)
7. **Escolher codec pelo destino** (lossless para arquivo, Opus para fala/realtime, AAC para compat) · STRONG
8. **Sincronização sempre** (áudio↔vídeo, timestamps de legenda) · UNIVERSAL

## ÁREAS DE CONHECIMENTO
- **Captura/reprodução**: PortAudio · miniaudio · libsndfile · SDL
- **DSP**: EQ · compressor · limiter · reverb · delay · denoise · dehum · declick (sox, pedalboard)
- **Análise**: waveform · spectrogram · librosa
- **STT**: Whisper (whisper.cpp/faster-whisper) · vosk · silero-VAD (detecção de voz)
- **TTS**: coqui TTS · piper (local, leve)
- **Música/MIDI**: FluidSynth · MuseScore · LMMS · MIDI
- **DAW/editing**: Ardour · Audacity · Reaper SDK · non-destructive
- **Separação/IA**: demucs · AudioLDM · audiocraft (referência técnica)
- **Plugins/real-time**: JUCE · VST3 SDK · AudioKit

## ANTI-PATTERNS
`aplicar todos os filtros sem análise (restoration soup)` · `normalizar por pico em vez de LUFS` · `converter formato sem destino claro (perde qualidade)` · `processar áudio não-confiável sem sandbox` · `destruir o original` · `ignorar sync (áudio/vídeo ou legenda)` · `baixa latência sem budget de buffer` · `TTS/STT sem avaliação de qualidade`

## CHECKLIST (quality gate)
- [ ] Análise antes do plano (noise/hum/clipping/voz)
- [ ] Só os filtros necessários aplicados
- [ ] Loudness normalizado por LUFS (quando aplicável)
- [ ] Codec/formato pelo destino
- [ ] Original intacto + metadados preservados
- [ ] Sync validado (áudio↔vídeo, timestamps)
- [ ] Sandbox + limites para arquivo não-confiável
- [ ] STT/TTS avaliados (qualidade, latência, custo)

## A REGRA
O Cosca deve entender o sinal (análise), planejar o DSP (só o necessário), processar, validar (sync/loudness/artefatos) e preservar o original — uma **Audio Engineering Intelligence**, não um conjunto de comandos de conversão.

## REFERÊNCIAS
**Core**: FFmpeg · GStreamer · PortAudio · libsndfile · miniaudio
**Codecs**: Opus · Vorbis · FLAC
**DSP/Análise**: sox · pedalboard · librosa · python-soundfile
**STT**: Whisper · whisper.cpp · faster-whisper · whisperX · vosk · silero-VAD
**TTS**: coqui TTS · piper
**Separação/IA**: demucs · ultimatevocalremovergui · AudioLDM · audiocraft
**Diarização**: pyannote-audio
**Música/MIDI/DAW**: FluidSynth · MuseScore · LMMS · Ardour · Audacity · Reaper SDK
**Plugins/Realtime**: JUCE · VST3 SDK · iPlug2 · AudioKit
