# Auditoria — Cluster de Voz e Áudio (COSCA v1.5.0)

**Data:** 2026-09-05
**Tipo:** Auditoria de arquitetura, READ-ONLY (nenhum arquivo de código foi escrito/modificado; apenas leitura + build/test read-only para verificação de estado).
**Escopistas:** `internal/voice/`, `internal/worldmodel/audio/` (mic/stt/tts), `internal/cli/` (voice_*, speech_tts_*, mic_*, perception_stt_*), `internal/media/` e `internal/worldmodel/vfx/` como contexto.
**Regra de ouro:** cada afirmação aponta `path:linha`; nada foi inferido fora do código lido. Onde o código não prova algo, isso está declarado explicitamente.

> **Resumo executivo (3 frases):** O cluster tem **duas vidas distintas** — um TTS concatenativo isolado (`internal/voice`, Go puro, “Voz Cosca”, não acoplado ao produto) e um pipeline de percepção/generação (`internal/worldmodel/audio`) cujo caminho **nativo sherpa-onnx** (STT/TTS via cgo ↔ DLL) existe, mas **não compila hoje**, porque o `replace` do módulo aponta para um diretório temporário que não existe. No build **default** (`make build`: `TAGS=""`, `CGO_ENABLED=0`), STT/TTS/mic caem em **stubs NOOP** que reportam “desabilitado”, e o restante da malha de voz acaba dependendo de **processos externos** (systemctl de um projeto Python separado, python3 subprocess, `aplay`). O build default está **verde** (build/vet/testes passam), mas o caminho “real” sherpa está **quebrado** neste ambiente.

---

## 0. Correções honestas de premissas do pedido

| Premissa do solicitante | Verificado no código | Veredito |
|---|---|---|
| `cosca voice` = "Assistente de voz cosca (projeto independente)" | `internal/cli/voice.go:40` — `Short: "Assistente de voz cosca (projeto independente)"` | ✅ confirmado |
| "Há 14+ arquivos `voice_chat_*.go`" | `Get-ChildItem internal/cli -Filter 'voice_chat_*.go'` → **8 arquivos** (5 src: `action`,`deliberate`,`respond`,`noop`,`sherpa`; 3 test: `action_test`,`deliberate_test`,`respond_test`) | ❌ **não confere — são 8, não 14+** |
| "`internal/voice/` 8 src / 2 test" | Diretório do pacote: **7 src** (`g2p`,`synth`,`stress`,`rules`,`prosody`,`phoneme`,`lexicon`) + `cmd/demo/main.go` (separate package `main`) + **2 test** (`g2p_test`,`synth_test`) | ⚠️ **7 src (-1)**, o `cmd/demo` é outro package |
| "O README cita 'sherpa'" | `rg -i "sherpa|voice" README.md` → **0 ocorrências** | ❌ **o README não cita sherpa nem voice**. As menções a sherpa estão em `go.mod:8`/`go.mod:241` e nos headers/comentários dos pacotes + `.cosca/config.yaml:168,176` |
| "capture_winmm = WebAudio/WinMM" | `mic/capture_winmm_windows.go:38` → `syscall.NewLazyDLL("winmm.dll")`; usa **WinMM/WaveIn** (não WebAudio) | ⚠️ é **WinMM/WaveIn**, não WebAudio |

---

## 1. Mecanismo de build tags — sherpa vs noop (núcleo da arquitetura)

O cluster usa **pares mutuamente exclusivos** por build tag. Em cada package relevante existe um stub `_noop/disabled` (`//go:build !tag`) e uma implementação real `_sherpa` (`//go:build tag`). A tag habilita cgo↔DLL; a ausência compila o stub.

| Package | Real (`go:build`) | Stub (`go:build`) | Artefato real |
|---|---|---|---|
| `stt` | `stt_sherpa.go:1` `//go:build stt_sherpa` | `stt.go:1` `//go:build !stt_sherpa` | `NewOnlineRecognizer` (`stt_sherpa.go:171`) |
| `tts` | `tts_sherpa.go:1` `//go:build tts_sherpa` | `tts.go:1` `//go:build !tts_sherpa` | `NewOfflineTts` (`tts_sherpa.go:225`) |
| `mic` (bus) | `mic_sherpa.go:1` `//go:build stt_sherpa` | `mic_noop.go:1` `//go:build !stt_sherpa` | `mic.NewCapture` (`mic_sherpa.go:40`) |
| `speech_tts` | `speech_tts_sherpa.go:1` `//go:build tts_sherpa` | `speech_tts_noop.go:1` `//go:build !tts_sherpa` | `tts.NewSpeaker` (`speech_tts_sherpa.go:41`) |
| `voice speak` | `voice_speak_sherpa.go:1` `//go:build tts_sherpa` | `voice_speak_noop.go:1` `//go:build !tts_sherpa` | `spk.SpeakToFile` (`voice_speak_sherpa.go:53`) |
| `voice listen` | `voice_listen_sherpa.go:1` `//go:build stt_sherpa` | `voice_listen_noop.go:1` `//go:build !stt_sherpa` | `mic.NewCapture` (`voice_listen_sherpa.go:68`) |
| `voice chat` | `voice_chat_sherpa.go:1` `//go:build stt_sherpa` | `voice_chat_noop.go:1` `//go:build !stt_sherpa` | loop completo (`voice_chat_sherpa.go:44`) |
| `perception_stt` | `perception_stt_sherpa.go:1` `//go:build stt_sherpa` | `perception_stt_noop.go:1` `//go:build !stt_sherpa` | `buildSttAudioSource` (`perception_stt_sherpa.go:63`) |

**Como o binário é montado (Makefile):**
- `Makefile:48` — `TAGS :=` (vazio por default) e `Makefile:49` — `CGO_ENABLED ?= 0`.
- `Makefile:71-73` — `build` usa `-tags "$(TAGS)"` e `CGO_ENABLED=$(CGO_ENABLED)`. Logo, **o binário default sai com `TAGS=""` → todas as tags sherpa desligadas → stubs NOOP**.
- Para ativar o caminho real seria `CGO_ENABLED=1 TAGS="stt_sherpa tts_sherpa" make build`, exigindo mingw + DLLs sherpa + o módulo `github.com/k2-fsa/sherpa-onnx-go-windows`.

**Dependência e replace (ponto crítico):**
- `go.mod:8` — `github.com/k2-fsa/sherpa-onnx-go-windows v1.13.6`.
- `go.mod:241` — `replace github.com/k2-fsa/sherpa-onnx-go-windows => C:/Users/Henrique/AppData/Local/Temp/opencode/sherpa-onnx-go-windows`.
- Verificação: `Test-Path 'C:/Users/Henrique/AppData/Local/Temp/opencode/sherpa-onnx-go-windows'` → **False**.
- **Consequência medível:** `go build -tags stt_sherpa,tts_sherpa ./internal/worldmodel/audio/stt/... ./internal/worldmodel/audio/tts/...` → **erro**: `replacement directory ... does not exist` (`SHERPA_BUILD_EXIT=1`).

> **Honestidade total:** o código do motor sherpa está **presente e completo** (STT streaming + TTS offline, com VAD/endpoint/watchdog), mas **o caminho sherpa não compila neste ambiente** por causa do `replace` aponte para um diretório temporário apagado. Não há DLLs sherpa/onnx em `bin/` (só `cosca.exe`). Portanto **o COSCA "nativo" que fala/ouve não está operacional hoje**; o que roda é o stub NOOP.

---

## 2. Componentes read-only (o que faz / backend / plataforma / stubs)

### 2.1 `internal/voice` — TTS concatenativo "Voz Cosca" (isolado)

**O que faz (header/API):**
- `synth.go:26` — `SampleRate = 24000`.
- `synth.go:31` — `Banco` de dífonos (transições f1→f2) carregado de um diretório; `synth.go:38` — `AbrirBanco(dir)` lê os `.wav` (via `lerWAV16` PCM16 mono → `[]float32`).
- `synth.go:121` — `Sintetizar(texto, estilo)` faz G2P → prosódia → concatenação com crossfade/resample/energia.
- `g2p.go:196` — `G2P(texto)` pipeline grafema→fonema (Tokenize→Lexicon→Rules→Stress→Fonologia).
- `phoneme.go:36` — `Phoneme` com `Evidence` (FACT/LEXICON/INFERRED/FALLBACK) e `DurClass`.
- `lexicon.go:19` — léxico hardcoded de pronúncias PB (ex.: `"cosca": {tokens("k 'o S k a")}`).
- `rules.go`, `stress.go`, `prosody.go` — regras ortográficas, tonicidade e planejador prosódico (`ProsodyPlan`, `VoiceStyle`).

**Backend real: [NATIVO Go]** — todo o DSP/síntese é Go puro. **Exceção honesta:** `synth.go:262` — `TocarPCM` faz `exec.Command("aplay", ...)` (player Linux). `synth.go:232` — `EscreverWAV16` é Go puro.

**Plataforma:** sintetizador cross-plataforma; **playback só Linux** (`aplay`).

**Não implementado / dependências:**
- Requer **banco de dífonos** externo; se vazio → `synth.go:55` `errors.New("banco de dífonos vazio em " + dir)`.
- `cmd/demo/main.go:16` — path hardcoded `/home/cosca/.cosca/voice/bank/diphones` (Linux).
- **Não acoplado ao produto:** `rg "internal/voice"` retorna **apenas** `internal/voice/cmd/demo/main.go:8`. O binário principal (`cmd/cosca`) **não importa** este pacote.

**Estado:** build OK, `go vet` OK, `go test ./internal/voice/...` → `ok (cached)`.

### 2.2 `internal/worldmodel/audio` — Pipeline de percepção/generação (subprocess)

**O que faz (header/API):**
- `pipeline.go:29` — `PipelineConfig` (`Whisper`,`DiffFields`,`CoquiTTS`,`AudioCraft`); `pipeline.go:37` — `DefaultPipelineConfig`.
- `pipeline.go:90` — `Perceive` (STT + análise espacial → `AudioEvent`); `pipeline.go:161` — `GenerateTTS`; `pipeline.go:180` — `GenerateSFX`.

**Backend real: [EXTERNAL / subprocess python3]**
- `adapters.go:37` — `runSubprocess` exec `exec.CommandContext(ctx, "python3", scriptPath)`.
- `adapters.go:77` — `"adapters/audio/whisper.py"`; `:126` — `"adapters/audio/difffields.py"`; `:184` — `"adapters/audio/coqui_tts.py"`; `:233` — `"adapters/audio/audiocraft.py"`.

**Não implementado (dead contract):**
- `Test-Path adapters` → **False**; `Get-ChildItem -Recurse -Filter *.py | match whisper|coqui|taichi|audiocraft|difffields` → **0 resultados**. Os scripts referenciados **não existem** no repo. Logo `Perceive`/`GenerateTTS`/`GenerateSFX` **falhariam** em runtime (o subprocess python3 não encontra o script).

> **Honestidade:** este pipeline é um **contrato de subprocesso com scripts ausentes** — não há implementação Go real ou binário sherpa aqui; é o "jeito antigo" (whisper.cpp via python). Disto o caminho sherpa (2.4/2.5) é separado.

**Estado:** build OK, `go test ./internal/worldmodel/audio/...` → `ok`.

### 2.3 `internal/worldmodel/audio/mic` — captura de microfone

**O que faz (header/API):** `source.go:1` — captura PCM do mic em tempo real, **nativamente**, via WinMM/WaveIn: "no external library, no Python, no PortAudio — raw syscalls". `source.go:91` — `Source` interface; `source.go:112` — `NewCapture(cfg)` → `newCapture`; `source.go:99` — `DeviceInfo`/`ListDevices`.

**Backend real: [NATIVO Go] (só win32 amd64/arm64); [NOOP/ErrUnsupported] nas demais:**
- `capture_winmm_windows.go:1` — `//go:build windows && (amd64 || arm64)`. `:38` — `syscall.NewLazyDLL("winmm.dll")` + procs WaveIn. `:106` — `newCapture` real. `:270` — `ListDevices` enumera via `waveInGetDevCapsW`.
- `capture_windows_other.go:1` — `//go:build windows && !(amd64 || arm64)` → `ErrUnsupported` (`:12`).
- `capture_other.go:1` — `//go:build !windows` → `ErrUnsupported` (`:12`).

**Plataforma (honesto):** captura real **apenas Windows/amd64+arm64**. Em **WSL/Linux/macOS → ErrUnsupported** (sem captura). Não há backend WebAudio nem PortAudio.

**Não implementado:** enumeração/captura fora de win32 amd64/arm64; 32-bit Windows (stub). `source_stt_sherpa.go:1` (`//go:build stt_sherpa`) — `MicrophoneAudioSource` que liga mic→stt (só compila sob a tag).

**Estado:** build/test OK (`go test ./internal/worldmodel/audio/mic/...` → `ok`).

### 2.4 `internal/worldmodel/audio/stt` — reconhecimento de fala (sherpa)

**O que faz (header/API):** `types.go:1` — STT nativo Go, ONNX local. `types.go:122` — `Engine` interface; `types.go:134` — `Stream` (AcceptWaveform/Decode/Result/IsEndpoint/Reset). `types.go:32` — `DefaultMaxSegmentDuration=30s` (watchdog de memória).

**Backend real: [NATIVO Go via cgo↔DLL], não exec de binário externo**
- `stt_sherpa.go:20` — `import sherpa "github.com/k2-fsa/sherpa-onnx-go-windows"`. `:171` — `sherpa.NewOnlineRecognizer`. `:189` — `NewStream`. `:209` — `AcceptWaveform` → `st.s.AcceptWaveform`. **Isto é ligação cgo à DLL sherpa-onnx (in-process), NÃO `exec` de um binário.**
- `audiosource_sherpa.go:66` — `NewAudioSource` implementa `bus.AudioSource`; `:115` — `PushPCM` (não-bloqueante, dropa em buffer cheio); `:225` — watchdog finaliza segmento e chama `Reset`.

**Default (sem tag) = [NOOP]:** `stt.go:25` — `Enabled() false`; `:27-38` — todos os métodos retornam `ErrDisabled`.

**Não implementado / pendência:**
- **Sem testes** no pacote: `Get-ChildItem internal/worldmodel/audio/stt -Filter *_test.go` → **vazio**.
- Motor real **não compila hoje** (replace ausente, ver §1).
- VAD (`VADConfig`) é opcional e não ligado por padrão.

**Estado:** build default OK; sherpa build **quebrado**.

### 2.5 `internal/worldmodel/audio/tts` — síntese de fala (sherpa)

**O que faz (header/API):** `types.go:1` — TTS nativo Go (FASE C). `types.go:157` — `Engine` (Open/Close/Synthesize/SampleRate/NumSpeakers). `types.go:31` — `ModelType` (vits/kokoro/matcha/zipvoice). `speaker.go:18` — `Speaker`; `:65` — `Synthesize`; `:81` — `SpeakToFile`; `:117` — `SynthesisQueue` (fila assíncrona). `wav.go:16` — `WriteWAV16File` (Go puro).

**Backend real: [NATIVO Go via cgo↔DLL], não exec de binário externo**
- `tts_sherpa.go:21` — `import sherpa "github.com/k2-fsa/sherpa-onnx-go-windows"`. `:225` — `sherpa.NewOfflineTts`. `:246`/`:260` — `Synthesize` → `e.tts.Generate`. `:237` — `DeleteOfflineTts`. **In-process via cgo↔DLL.**
- `tts_sherpa.go:62` — `discoverModels` resolve paths por arquivo OU varre `*.onnx` (`scanOnnx`); `:126` — `validate`.

**Default (sem tag) = [NOOP]:** `tts.go:22` — `Enabled() false`; `:24-31` — métodos retornam `ErrDisabled`.

**Não implementado / pendência:**
- Motor real **não compila hoje** (replace ausente, §1). Sem DLLs sherpa em `bin/`.
- Testes que cobrem o motor real (`tts_test.go:74` `TestSynthesizeReal`) **pulam** sob `!Enabled()` (`:75`). `tts_test.go:37` `TestConfigValidation` pula na build default.

**Estado:** build default OK; sherpa build **quebrado**.

### 2.6 `internal/cli/voice*` — comandos de voz

**`voice.go` (gerente do projeto externo):** `:40` — Short confirmado. `:22` — `voiceProjectRoot()` → `~/Documents/projects/cosca-voice`. `:31` — `voiceServiceName = "cosca-voice.service"`. `:99` — `voiceControl` chama `systemctl --user <action> cosca-voice.service` (`:105` exec `systemctl`). **Backend: EXTERNAL (systemd de um projeto Python torch+faster-whisper+kokoro).** Falharia em Windows (systemctl é Linux).

**`voice_devices.go` (sem tag):** `:22` — `mic.ListDevices()`; em não-Windows → `ErrUnsupported` (mensagem limpa).

**`voice_speak_sherpa.go` / `voice_speak_noop.go`:** sherpa ⇒ `spk.SpeakToFile(...)` e grava `.wav` (`:53`); noop ⇒ imprime "TTS nativo desabilitado" (`:20-22`). Caso `spk == nil` ⇒ erro pedindo `-tags tts_sherpa` (`voice_speak_sherpa.go:49`).

**`voice_listen_sherpa.go` / noop:** sherpa ⇒ `mic.NewCapture` + `MicrophoneAudioSource` + `signal.NotifyContext` (`:68-84`); noop ⇒ "STT desabilitado" (`:22-24`).

**`voice_chat_sherpa.go` (o "teste absurdo"):** `:44` — loop de diálogo (mic→STT→bus→visão por ação→brain→TTS). `:130` — `bus.NewBus`. `:157` — `buildVoiceBrain` (Ollama/qwen3 via `internal/chat`). `:301` — `speakVoiceChat`. `:377-380` — `playVoiceChatWAV` usa **PowerShell `System.Media.SoundPlayer.PlaySync()`** (player do Windows). `:392` — `voiceEchoCooldown=4s` (supressão de eco). **Backend do loop: NATIVO Go + cgo↔DLL(sherpa) + subprocess PowerShell (player)**; noop (`voice_chat_noop.go:22`) → mensagem "STT/TTS desabilitado".

**`voice_chat_action.go` / `voice_chat_deliberate.go` / `voice_chat_respond.go` (SEM build tag, sempre compilam):**
- `action.go:85` — `parsePerceptionAction` (regex "olha/grava/interpreta"); `:159` — `buildActionEngine` → `visionact.New(...)` (visão por ação); `:175` — `runPerceptionAction`; `:249` — `shouldRespond` (gate anti-ruído).
- `deliberate.go:48` — `voiceBrain` (provider injetável via `SetVoiceBrain`); `:228`/`:274` — `respondWithDeliberation[Tool]` com memória-first (knowledge cache) e degradação graciosa para template; `:387` — `buildPerceptualPrompt`.
- `respond.go:24` — `respondFromWorldState` (template puro: "A visão não está ativa / Não estou vendo objetos / Estou vendo N objetos").
- **Backend destes 3: NATIVO Go** (lógica pura, sem cgo). São a parte testável da malha.

**Não implementado / honesto (CLI):** no build default, `speak/listen/chat` **apenas imprimem "desabilitado"**; o loop real de voz (falar/ouvir/ver ao vivo) é **inacessível** porque exige `stt_sherpa`/`tts_sherpa` + DLLs + módulo, e o módulo não compila. Toda a resposta inteligente por voz depende do `buildVoiceBrain` (LLM via Ollama), cuja disponibilidade não está garantida (degrada para template).

### 2.7 `internal/cli/perception_stt_*` — fiação STT→bus

- `perception_stt_sherpa.go:22` — `newSherpaSTTSource` exige `stt.provider=="sherpa"`, senão `nil`. `:63` — `buildSttAudioSource`. **Backend: NATIVO Go via cgo↔DLL (só sob `stt_sherpa`).**
- `perception_stt_noop.go:16` — retorna `nil`.
- `serve.go:609` — `buildPerceptionBus`: `:621` `buildSttAudioSource`; `:630` `buildMicAudioSource`; `:637` — se `nil` → `bus.NoopAudioSource{}`. **Conclusão honesta:** no build default o bus de percepção de áudio usa **NoopAudioSource** (vazio).

### 2.8 Contexto — `internal/media` e `internal/worldmodel/vfx`

- `media/media.go:1` — Media Engine (§16): "I/O e codecs sempre via FFmpeg" (P5). **Backend: EXTERNAL (`ffprobe`/`ffmpeg`).** Contexto de mídia.
- `worldmodel/vfx/pipeline.go:1` — pipeline VFX procedural; `:39` — `DefaultPipelineConfig` → `Script: "adapters/vfx/taichi.py"`. **Backend: EXTERNAL (python3/taichi).** Os scripts `adapters/*` **não existem** (ver §2.2). Contexto.

---

## 3. O que NÃO está implementado / é stub / no-op (lista honesta)

1. **STT real** (sherpa) — só sob `stt_sherpa`; default = stub `ErrDisabled` (`stt/stt.go:25`). **Não compila hoje** (replace quebrado, §1).
2. **TTS real** (sherpa) — só sob `tts_sherpa`; default = stub `ErrDisabled` (`tts/tts.go:22`). **Não compila hoje.**
3. **Captura de mic em Linux/WSL/macOS** e **Windows 32-bit** — `ErrUnsupported` (`capture_other.go:12`, `capture_windows_other.go:12`).
4. **Pipeline `Perceive`/`GenerateTTS`/`GenerateSFX`** (`worldmodel/audio`) — contrato de subprocesso para scripts **ausentes** (whisper/difffields/coqui/audiocraft `*.py`).
5. **Player de voz** — `internal/voice` usa `aplay` (só Linux); `voice_chat` usa PowerShell `SoundPlayer` (só Windows). Não há cross-plataforma uniforme.
6. **VAD opcional** (`stt/types.go:87`) — presente no config, não é ligado por padrão.
7. **`cosca voice start/stop/status`** — depende do projeto externo `~/Documents/projects/cosca-voice` + `systemctl --user` (Linux-only). Em Windows falha.
8. **Endpoint HTTP `/v1/voice/speaks`** — `api/rest/handler/voice.go:55-56` devolve **503** se `!speaker.Enabled()` (default build = sempre 503).

---

## 4. Estado de build/test HOJE (verificado, read-only)

| Comando | Resultado |
|---|---|
| `go build ./internal/voice/... ./internal/worldmodel/audio/...` | ✅ `BUILD_EXIT=0` |
| `go vet ./internal/voice/... ./internal/worldmodel/audio/...` | ✅ `VET_EXIT=0` |
| `go test ./internal/voice/...` | ✅ `ok (cached)` |
| `go test -count=1 ./internal/worldmodel/audio/...` | ✅ `ok` (audio, mic, tts; `stt` = "no test files") |
| `go build -tags stt_sherpa,tts_sherpa ./internal/worldmodel/audio/stt/... ./internal/worldmodel/audio/tts/...` | ❌ **`replacement directory ... does not exist`** (`SHERPA_BUILD_EXIT=1`) |

**Síntese honesta:** **default está verde; caminho sherpa está quebrado.** A camada "real" de voz (STT/TTS nativos) é código completo, mas inalcançável neste ambiente por causa do `replace` do módulo e da ausência de DLLs.

---

## 5. Tabela-resumo

| Componente | #src | #test | Backend (nativo/external/noop) | Plataforma | Estado | Nota honesta |
|---|---|---|---|---|---|---|
| `internal/voice` (Voz Cosca) | 7 (+1 demo) | 2 | NATIVO Go (playback `aplay` subprocess) | Cross-plataforma (`aplay` só Linux) | build/test OK (cached) | Isolado, **fora do produto**; exige banco de dífonos externo (`synth.go:55`); demo path hardcoded Linux (`demo/main.go:16`) |
| `worldmodel/audio` (Pipeline) | 2 | 2 | EXTERNAL (python3 subprocess) | Cross-plataforma (precisa python3) | build OK | **dead contract**: scripts `adapters/audio/*.py` não existem |
| `worldmodel/audio/mic` | 5 | 3 | NATIVO WinMM (amd64/arm64) / NOOP (`ErrUnsupported`) | win32 amd64+arm64 | build/test OK | Real só no Windows; Linux/WSL/32-bit → `ErrUnsupported` (não é WebAudio) |
| `worldmodel/audio/stt` | 4 | 0 | NATIVO Go cgo↔DLL (sherpa) / NOOP default | windows cgo | default OK; **sherpa quebrado** | **sem testes** (0); real engine inalcançável (replace ausente) |
| `worldmodel/audio/tts` | 5 | 1 | NATIVO Go cgo↔DLL (sherpa) / NOOP default | windows cgo | default OK; **sherpa quebrado** | `TestSynthesizeReal`/`TestConfigValidation` pulam no default (`tts_test.go:75,39`) |
| `cli/voice.go` (manager) | 1 | 0 | EXTERNAL (systemctl --user) | Linux/systemd | compila | governa o projeto **independente** `cosca-voice` (Python) |
| `cli/voice_speak` | 2 | 0 | NATIVO cgo↔DLL / NOOP | windows cgo | default NOOP; sherpa quebrado | default imprime "TTS desabilitado" (`voice_speak_noop.go:20`) |
| `cli/voice_listen` | 2 | 0 | NATIVO cgo↔DLL / NOOP | windows cgo | default NOOP | default imprime "STT desabilitado" (`voice_listen_noop.go:22`) |
| `cli/voice_chat` | 5 | 3 | NATIVO Go + cgo↔DLL + PS player / NOOP | windows cgo (real) | build/test OK (lógica); loop real quebrado | lógica pura testável; `voice_chat_sherpa` é a única via real, e não compila |
| `cli/perception_stt` | 2 | 0 | NATIVO cgo↔DLL / NOOP | windows cgo | default NOOP | `serve.go:637` cai em `bus.NoopAudioSource{}` no default |
| `api/rest/handler/voice.go` | 1 | n/d | REST → sherpa TTS | windows cgo | default **503** | `Speaks` devolve 503 sem `tts_sherpa` (`voice.go:55`) |
| `internal/media` (contexto) | 4 | 2 | EXTERNAL (ffmpeg/ffprobe) | cross | build OK | P5 — nunca reescreve parsing de mídia |
| `internal/worldmodel/vfx` (contexto) | 2 | 2 | EXTERNAL (python taichi) | cross | build OK | script `adapters/vfx/taichi.py` não existe no repo |

---

## 6. Conclusão do auditor (honesta)

- O cluster é **arquiteturalmente bifurcado**: um "modo legado/externo" (python3, systemd, ffmpeg, `aplay`/PowerShell) e um "modo nativo/sherpa" claramente desenhado (build tags, `Enabled()`, degradação graciosa). O design de degradação é **bom e coerente** (default silencioso, nunca crasha).
- Porém **o estado operacional é decepcionante**: o caminho que daria ao COSCA "falar e ouvir em Go nativo" — o **sherpa** — **não compila neste ambiente** (replace para diretório temporário inexistente, sem DLLs em `bin/`), e o pipeline antigo aponta para **scripts que não existem**. Então, na prática, hoje **não há voz funcional no binário default**: só stubs que dizem "desabilitado".
- **Pendências críticas a resolver** (para o nível 3+ de honestidade técnica): (1) reparar o `replace`/módulo do sherpa (usar versionamento real, não path temporário); (2) commitment das DLLs sherpa/onnx ou build via CI; (3) decidir se o pipeline `worldmodel/audio` (scripts python) é legado a ser removido ou possui scripts a restaurar; (4) unificar player cross-plataforma (hoje `aplay`/PowerShell).
