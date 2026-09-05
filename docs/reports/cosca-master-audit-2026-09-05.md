# COSCA — Auditoria Total do Código (Código da Verdade)

> **Data**: 2026-09-05
> **Autor**: Cosca Kernel (consigliere) — por ordem do Don: "analisar todo o código, tudo mesmo, documentar o projeto como código da verdade"
> **Método**: READ-ONLY total — leitura de código-fonte (`internal/*.go`, `api/rest/*.go`, `cmd/*.go`, `laboratory/*.py`), execução de build/test/benchmark, e inventário por contagem. **Nada foi alterado.**
> **Princípio**: P2 — o código executado é a verdade absoluta. Docs são intenção; código é o fato.
> **Escopo**: TODO o repositório — 1.024 arquivos Go (src), 843 test, 234 packages, 165 domínios internos, 33 REST handlers, e o cluster Living World (visão/áudio/3D/Unreal/Blender/scientific).

---

## 0. O projeto em números (medido hoje, 2026-09-05)

| Métrica | Valor |
|---------|------:|
| Arquivos Go (código-fonte, sem test) | **1.024** |
| Arquivos Go (test) | **843** |
| Packages Go (`go list ./...`) | **234** |
| Domínios internos (`internal/`) | **165** |
| Arquivos não-Go de código (py/cpp/cs/ts/tsx/js/sh/lua) | 2.542 |
| Arquivos Markdown | 7.297 |
| Documentos em `docs/` | 150 |
| Framework Cosca em `.opencode/cosca/` | 1.006 .md |
| REST handlers (`api/rest/`) | 33 |
| Arquivos em `laboratory/` (treino/experimentos) | 58 |
| Testes de integração (`test/`) | 9 |
| Total de arquivos no repo (excl. DB/wal/dat) | **13.642** |

**Conclusão do inventário:** o COSCA é um **sistema de verdade, enorme e multifacetado** — não um protótipo. Tem 1.024 fontes Go, 234 packages, e cobre cognitivo, visão, áudio, voz, mundo 3D, ponte Unreal/Blender, engine científica, treino LoRA, realtime e durabilidade.

---

## 1. O que o Don pediu — verificado domínio a domínio

O Don citou explicitamente: **visão, áudio, científico, 3D, unreal, blender, realtime, vision-action**. Abaixo, o veredito honesto de cada um (com os relatórios de auditoria como referência).

### 1.1 Visão (`internal/vision`, `internal/worldmodel/vision`, `internal/screen`)
| Sub-pacote | #src | Implementação | Veredito |
|-----------|-----:|---------------|----------|
| `internal/vision` | 3 | tesseract (OCR) + llava (VLM via Ollama HTTP) — subprocesso/HTTP | ✅ Real |
| `internal/worldmodel/vision` | 7 | ONNX **nativo Go** (CLIP + GroundingDINO + Depth + SAM) | ✅ Motor real, ⚠️ com desvios |
| `internal/screen` | 10 | captura GDI + CLIP + métricas estéticas + OCR (PowerShell) | ✅ Real (Windows) |

**Desvios honestos na visão:**
- `worldmodel/vision/pipeline.go:173-180` — Step CLIP-classify é **no-op** (`_ = candidates`), nunca roda.
- `worldmodel/vision/adapters.go:212-214` — SAM exige `PromptPoint`, que o config não seta; mesmo setado o código ignora (`_ = img`). **O SAM degrada sempre (vazio).**
- Header de `worldmodel/vision` declara "Python subprocesses" mas o motor é ONNX Go — doc desalinhada.

### 1.2 Áudio / Voz (`internal/worldmodel/audio/*`, `internal/voice`, `internal/cli/voice*`)
| Sub-pacote | #src | Implementação | Veredito |
|-----------|-----:|---------------|----------|
| `worldmodel/audio/stt` | — | sherpa-onnx (STT) | ✅ Real, MAS não compila hoje |
| `worldmodel/audio/tts` | — | sherpa-onnx (TTS) | ✅ Real, MAS não compila hoje |
| `worldmodel/audio/mic` | — | **WinMM/WaveIn nativo** (Windows) | ✅ Real |
| `internal/voice` | 8 | TTS **concatenativo Go puro** (dífonos) | ✅ Real |
| `internal/cli/voice_chat_*` | 5 | loop voz | ✅ Real (parcial) |

**Desvio crítico e honesto:** o caminho sherpa (STT/TTS real) **não compila hoje** porque `go.mod:241` faz `replace => C:/Users/Henrique/AppData/Local/Temp/opencode/sherpa-onnx-go-windows`, diretório que **não existe**. Sem DLLs sherpa em `bin/`, o binário default (`Makefile:48-49`, `TAGS=""`, `CGO_ENABLED=0`) só roda os **stubs NOOP** que reportam "desabilitado". **O caminho real está no código mas não está no binário.** O caminho default (sem tag) é no-op.
- O README **não** cita "sherpa" (0 ocorrências) — o doc do Don estava desatualizado; o moto é via WinMM/WaveIn, não WebAudio.

### 1.3 Científico (`internal/sciengine`)
- `sciengine.go:49-72` — Scientific Engine é um **registry YAML passivo** (CRUD sobre `experiments.yaml`).
- **Reproducibilidade NÃO é garantida**: `CodeVersion`, `ModelVersion`, `Environment` são strings livres digitadas pelo chamador. Não há captura automática de commit/cmd, nem hash de entrada, nem re-run de verificação. `Timestamp` gravado em `New()` (:86).
- **Não há comando CLI ligando o sciengine** — só template em `cli/project_examples.go:81` e re-export via `pkg/engine`.

> **Se o valor central é científico, este é o gap nº 1.** O sciengine é um cadastro, não um verificador.

### 1.4 3D / Unreal / Blender (`internal/world*`, `internal/bridge`, `internal/procgen`)
| Sub-pacote | #src | Implementação | Veredito |
|-----------|-----:|---------------|----------|
| `internal/world` (root) | 14 | GeoToWorld, serialize fingerprint, validate — Go puro | ✅ Real, forte |
| `internal/world/nav` | — | plan A* com gate I1/I2 | ✅ Real |
| `internal/world/road` | — | gerador determinístico | ✅ Real |
| `internal/world/city` | — | city generator | ✅ Real |
| `internal/world/edit` | — | edição de mundo | ✅ Real |
| `internal/worldspec` | 1 | Validate fail-closed | ✅ Real |
| `internal/worldloop` | 1 | loop de mundo | ✅ Real |
| `internal/procgen` | 7 | procgen | ✅ Real |
| `internal/render` | 1 | render | ✅ Real |
| `internal/nodegraph` | 2 | node graph | ✅ Real |
| `internal/bridge` | 5 | **WebSocket real (nhooyr.io/websocket)** pro Unreal | ✅ Real (cliente) |
| `internal/worldmodel/vision` | 7 | ONNX nativo | ✅ Motor real |

**Veredito honesto do cluster mundo/3D:**
1. **Kernel de mundo é REAL e forte** — Go puro, determinístico, com invariantes I1-I5, testes verdes.
2. **A periferia de percepção é "viúva de scripts"** — os subdomínios `spatial`/`vfx`/`destruction`/`simulation` delegam a subprocessos Python (`runSubprocess(... adapters/spatial/slam.py ...)`) cujos **scripts NÃO existem no repo** (só `scripts/blender/*.py` existe). São contratos de subprocesso que falham em runtime.
3. **Unreal é metade real, metade contrato** — o cliente WebSocket e o Controller são reais e testados, mas o plugin `UCoscaWorldSubsystem`/`CoscaRuntime` **NÃO está neste repo** (só no docs + mock `bridge/mock.go`). **Blender é a única integração external end-to-end real** — `asset/blender.go` + `scripts/blender/*.py` presentes.

### 1.5 Realtime / Vision-Action (`internal/stallwatch`, `durable`, `concurrency`, `visionact`)
| Componente | Real? | Nota |
|------------|-------|------|
| `internal/stallwatch` | ✅ Real, wired | timeout+retry+backoff jitter+fallback; wired em `orchestration/executor.go:1072` |
| `internal/durable` | ✅ Real, wired | fencing/lease/idempotency/migração |
| `internal/concurrency` | ⚠️ 1 ponto só | só `semantic_router.go:139` |
| `internal/compute` | ✅ Real (17 src/15 test) | Compute Fabric, scheduler GPU, backpressure |
| `internal/visionact` | ⚠️ Parcial | `LookAtScreen` real, mas "agir" = **só TTS**. Não há ação física no mundo. |
| `internal/grounding` | ✅ Real | grounding linguagem→mundo |

**Loop perceber→decidir→agir:** existe **parcialmente**. Perceber (visionact real) → deliberar (LLM) → "agir" = **gerar uma fala (TTS)**. Não há ação sobre o mundo/ferramentas. O `agir` físico requer a ponte Unreal/Blender.

---

## 2. O cluster Cognitivo (o cérebro) — resumo da auditoria

| Componente | #src | Veredito |
|-----------|-----:|----------|
| `internal/knowledge` | — | ✅ Real — 17.738 entries, 50.014 vetores, busca híbrida |
| `internal/memory` | — | ✅ Real — 6 camadas |
| `internal/orchestration` | 18 src / 25 test | ✅ Real — multi-stage pipeline |
| `internal/engine` | 13 src / 16 test | ✅ Real — agent loop |
| `internal/pipeline` | 48 src / 41 test | ✅ Real — DAG + durable events |
| `internal/deliberate` | 6 src | ✅ Real — gates determinísticos zig-LLM |
| `internal/search`/`vector` | — | ✅ Real — híbrido FTS5+vetor+grafo |
| `internal/agents` | — | ✅ Real — 61 agentes |

---

## 3. Mapa de veredito unificado (o "semáforo" do COSCA)

### 🟢 VERDE — real, funciona, testado (código da verdade)
- **Cérebro**: knowledge, memory, search, vector, orchestration, engine, pipeline, gate
- **Mundo**: `world` (kernel), `nav`, `road`, `city`, `edit`, `worldspec`, `worldloop`, `procgen`, `render`, `nodegraph`
- **Realtime**: stallwatch, durable, compute, grounding
- **Visão**: `internal/vision` (OCR+VLM), `worldmodel/vision` (ONNX, com desvios), `screen`
- **Voz**: mic nativo WinMM, TTS concatenativo Go
- **Unreal**: cliente WebSocket (real), **Blender** (end-to-end real)
- **Governança**: Constituição, cert GOLD, chain Ed25519+DPAPI, quality gates

### 🟡 AMARELO — real no código, mas com desvio / não ligado / não compila no default
- **worldmodel/vision**: CLIP-classify no-op, SAM degrada (motor existe, pipeline incompleto)
- **sciengine**: reproduzibilidade não garantida (registro, não verificador)
- **spatial/vfx/destruction/simulation**: subprocessos Python cujos scripts não existem (contrato)
- **visionact**: percebe+delibera, mas "agir"=só TTS
- **deliberate**: `Enabled=false` por padrão (fail-closed)
- **voice̶̶̶ sherpa**: real mas não compila (replace aponta pra dir inexistente); default=noop
- **decision**: standalone (sem persistência wired)
- **concurrency**: wired em 1 ponto
- **training**: scripts LoRA reais mas o Go não orquestra

### 🔴 VERMELHO — o que quebra hoje (itens que NÃO rodam no estado atual)
- **`go.mod:241`** → `replace sherpa-onnx-go-windows` aponta para diretório inexistente. **STT/TTS sherpa não compila** (só noop).
- **Subprocessos Python de percepção** (`spatial`, `vfx`, `destruction`, `simulation`) → scripts **não existem** no repo → falham em runtime.
- **Plugin Unreal** (`UCoscaWorldSubsystem`) → **não está no repo** (só mock).
- **README desatualizado** → diz ~46 comandos / 53 agents / 29 skills; o real é **112 comandos / 61 agents / 88 skills** (gap 2.4×).
- **`plugin search`** → `pluginsAdapter.Search()` retorna `nil,nil` (stub) → sempre "No plugins found".

---

## 4. Os 5 relatórios de auditoria (anexos)

| Relatório | Caminho | Foco |
|-----------|---------|------|
| Prova de capacidade | `docs/reports/cosca-capability-proof-2026-09-05.md` | Inventário geral + certificação GOLD |
| Auditoria CLI | `docs/reports/cosca-cli-audit-2026-09-05.md` | 112 comandos vs estado real |
| Auditoria Cognição+Visão | `docs/reports/audit-cognition-vision-2026-09-05.md` | visão, screen, perception, sensor, compute |
| Auditoria Áudio | `docs/reports/cosca-audit-audio-2026-09-05.md` | voz, STT/TTS, mic |
| Auditoria Mundo/3D | `docs/reports/cosca-audit-world-3d-2026-09-05.md` | world, Unreal, Blender, procgen |
| Auditoria Científico/Realtime | `docs/reports/cosca-audit-sci-rt-2026-09-05.md` | sciengine, visionact, stallwatch, durable, training |

---

## 5. Conclusão honesta (o Don deve saber)

O COSCA é **um sistema real, grande e multifacetado** — 1.024 fontes Go, 234 packages, cognitivo completo, mundo 3D determinístico, ponte Unreal real, Blender end-to-end, realtime com stallwatch/durable, e um pipeline de treino LoRA. **Não é um protótipo.**

Mas, dito com a verdade que o Don exige:

1. **O núcleo (cérebro + mundo) é sólido e testado** — é o "ouro" do sistema.
2. **A periferia de percepção é uma corda com buracos** — vários subprocessos Python (`spatial`, `vfx`, `destruction`, `simulation`) apontam para scripts que **não existem** no repo. São contratos, não código vivo.
3. **O sherpa (STT/TTS real) não compila no estado atual** — aponta para um diretório temporário inexistente. O binário default roda no-op.
4. **A reproducibilidade científica não está garantida** — o sciengine registra, não verifica.
5. **Vision-action não age sobre o mundo** — só percebe e fala.
6. **Documentação desatualizada** — README subdimensiona o sistema em 2.4×.

**Este é o estado real do COSCA, sem enfeite.** Os verdes são fortes; os amarelos são promessa de código existente mas não ligado ou não compilável; os vermelhos são o que impede o "tudo funcionando" hoje.
