> **Version**: 1.0.5 | **Status**: active | **Owner**: Voice Engine | **Last Updated**: 2026-08-01

# VOICE ENGINE

## PURPOSE

The Voice Engine provides 100% local speech synthesis (TTS) and speech-to-text transcription (STT) for the Cosca Kernel, enabling natural voice conversations with the Don without any external or cloud services. It is the foundation for the `voice.local` capability (TTS Kokoro + STT faster-whisper).

## WHEN INVOKED

- Voice conversation sessions with the Don
- When the Don activates voice mode
- Reading responses aloud (TTS)
- Transcribing captured audio for processing (STT)

## COMPONENTS

| Component | Role | License | Runtime |
|-----------|------|---------|---------|
| Kokoro-82M | TTS synthesis (PT-BR native) | Apache-2.0 | CPU, real-time, 82M params |
| faster-whisper | STT transcription (CTranslate2, CPU int8) | MIT | CPU, faster than real-time — validated, preferred (pip, no cmake/root) |
| whisper.cpp | STT transcription (original reference) | MIT | CPU — requires cmake build, not available without root on this system |

### Requirements

- Python 3.12+ in a venv with pip (use `get-pip.py` inside the venv; system has PEP 668 and no ensurepip)
- ffmpeg via `imageio-ffmpeg` (static binary, no root) or ffmpeg in PATH
- Kokoro via `pip install kokoro misaki[zh] soundfile`
- STT via `pip install faster-whisper` (validated alternative to whisper.cpp, which requires cmake)
- PipeWire active (audio capture/playback)

## CONVERSATION FLOW

1. Capture audio via PipeWire
2. faster-whisper transcribes speech to text (STT engine: faster-whisper or whisper.cpp)
3. Kernel processes the transcription and generates a text response
4. Kokoro-82M synthesizes speech from the response
5. Playback through PipeWire

Target latency: < 300ms TTS synthesis.

Automated wrapper: `scripts/voice/voice_conversation.py`

## GUARDRAILS

- 100% local processing — zero audio exfiltration, no cloud services
- PT-BR as default language
- Fallback to text when STT/TTS is unavailable
- No audio leaves the machine

## OUTPUT FORMAT

### TTS synthesis (Kokoro)

```python
# Example: synthesize a PT-BR response to WAV (validated E2E)
from kokoro import KPipeline
import soundfile as sf
import numpy as np

pipeline = KPipeline(lang_code='p')  # 'p' = Portuguese
chunks = list(pipeline("Bom dia, Don. O Kernel está acordado e pronto para conversar.",
                       voice='pf_dora'))
audio = np.concatenate([c.audio for c in chunks])
sf.write('response.wav', audio, 24000)  # sample rate is fixed at 24000 Hz
```

```bash
# Playback via PipeWire
pw-play response.wav
```

> Note: sample rate is fixed at 24000 Hz. The pipeline result has no `sr` attribute — do not use `c.audio.sr`.
> Validated PT-BR voices: `pf_dora` (female), `pm_alex` (male), `pm_santa` (male).

### STT transcription (faster-whisper)

```python
# Example: transcribe captured audio (validated E2E)
from faster_whisper import WhisperModel

model = WhisperModel('small', device='cpu', compute_type='int8')
segments, info = model.transcribe('capture.wav', language='pt')
text = ' '.join(seg.text for seg in segments)
# Output: transcribed text consumed by the Kernel
```

```bash
# Alternative (reference): whisper.cpp — requires a cmake build
./whisper-cli -m models/ggml-base.bin -l pt -f capture.wav -otxt
```

## WRAPPER

Automated end-to-end conversation wrapper: `scripts/voice/voice_conversation.py`.
Run it with the validated venv python (default `/tmp/opencode/vtest/bin/python3`).

```bash
# Voice mode (loop): capture -> faster-whisper -> cosca-chat exec -> Kokoro -> playback
scripts/voice/voice_conversation.py

# Text-input mode (fallback when capture is unavailable / forced)
scripts/voice/voice_conversation.py --no-capture

# Test mode: synthesize and play a single phrase, no loop
scripts/voice/voice_conversation.py --once "Bom dia, Don."

# Development mode: print transcription and response to stdout
scripts/voice/voice_conversation.py --dev
```

Options: `--venv-python`, `--cosca-chat`, `--voice` (default `pf_dora`),
`--model` (default `small`), `--lang` (default `pt`), `--sys-prompt`,
`--turn-duration` (default 10s), `--no-capture`, `--dev`, `--service`,
`--session` (default `cosca-voice`), `--clear-session`, `--log`. See `--help`.

Conversation memory is permanent: every turn is persisted to
`.cosca/sessions/cosca-voice.jsonl` (via `cosca-chat exec --session cosca-voice`).
The brain remembers the whole conversation from the start. Reset with
`cosca-chat exec --session cosca-voice --clear-session` or
`voice_conversation.py --clear-session`.

> The conversation is pinned to the `cosca-kernel` agent (`--agent cosca-kernel`,
> default) — the wrapper always speaks with the Kernel/consigliere and the router
> NEVER diverts the turn to another agent (e.g. dashboard/painel questions stay
> with the Kernel instead of being routed to cosca-analytics / cosca-monitoring).

### ALWAYS-ON (systemd service)

The wrapper runs as a persistent always-on service (user unit
`~/.config/systemd/user/cosca-voice.service`) that listens continuously with
silero VAD (faster-whisper.vad): it only captures/transcribes when human voice
is detected, responds via Kokoro, and goes back to listening. Errors never kill
the service; it stops only via SIGTERM (systemctl stop). The word "sair"/"exit"
does NOT stop the service — it replies that the service stays active.

```bash
systemctl --user daemon-reload
systemctl --user start cosca-voice
systemctl --user enable cosca-voice
systemctl --user status cosca-voice
journalctl --user -u cosca-voice -f
```

Notes:
- Linger is enabled: the service survives logout (`loginctl enable-linger cosca`).
- Conversation log: `~/.local/share/cosca/voice-conversation.log`
  (timestamp ISO + `[DON]`/`[KERNEL]` role + text; override with `--log`).
- Manual run (foreground, dev): `scripts/voice/voice_conversation.py --service --dev`.

## DEPENDENCIES

| File | Why |
|------|-----|
| [PROVIDER_INTERFACE.md](../../PROVIDER_INTERFACE.md) | Provider abstraction for the `audio` modality |
| [Provider Discovery](../../skills/ai/PROVIDER_DISCOVERY.md) | Provider evaluation for voice capabilities |
| Kernel | Session initialization and orchestration |

## RELATED

- [PROVIDER_INTERFACE.md](../../PROVIDER_INTERFACE.md) — Provider abstraction, `audio` modality
- [Provider Discovery](../../skills/ai/PROVIDER_DISCOVERY.md) — Kokoro / faster-whisper provider docs
- [skills/ai](../../skills/ai/) — AI skills (embedding, prompt, provider)
- [KERNEL.md](../../KERNEL.md) — Orchestration entry point

## HISTORY

| Version | Date | Changes |
|---------|------|---------|
| 1.0.5 | 2026-08-01 | Voice conversation pinned to cosca-kernel agent (--agent cosca-kernel) — router no longer diverts dashboard/painel questions to other agents. |
| 1.0.4 | 2026-08-01 | Added permanent conversation memory: exec --session flag persists history to .cosca/sessions/{id}.jsonl; voice wrapper uses fixed session cosca-voice. |
| 1.0.3 | 2026-08-01 | Added always-on service mode: VAD-based smart listening, --service flag, systemd user unit (cosca-voice.service), conversation log. |
| 1.0.2 | 2026-08-01 | Added voice conversation wrapper (scripts/voice/voice_conversation.py): capture → faster-whisper → cosca-chat exec → Kokoro → playback; text-input fallback. |
| 1.0.1 | 2026-08-01 | Corrected with validated E2E data: real PT-BR voices `pf_dora`/`pm_alex`/`pm_santa` (replaces non-existent `p_pt_br_f_luzia`), faster-whisper validated as STT (whisper.cpp kept as reference), sample rate fixed at 24000 Hz, ffmpeg via imageio-ffmpeg. |
| 1.0.0 | 2026-08-01 | Initial version. Voice capability design, TTS/STT components, conversation flow, guardrails. |
