# ADR-025: Percepção/visão 100% Go-nativa via ONNX Runtime (resolver Python→Go)

> **Status:** Accepted (aprovado pelo Don, 2026-08-28) | **Owner:** cosca-kernel + cosca-architecture | **Last Updated:** 2026-08-28
> **Revisão:** aprovado. **Design aditivo — não quebra o Root.**
> **Referência (base):** auditoria `internal/media` + `internal/vision` + `internal/worldmodel/vision`
> (adapters GroundingDINO/SAM/CLIP/Depth); o gap de portabilidade `python3`/Windows; a POC de percepção
> deterministic-first (`E:\cosca-tmp\video-percept`); `ADR-017` (cristalização do que é provado) +
> `ADR-014` (consciência) + a primitiva epistêmica `internal/vision/observation.go`.

---

## 0. Contexto — a dúvida

O Don perguntou: **"tudo em Python pode ser feito em Go para ser nativo?"**

A auditoria da sessão revelou:
- `internal/media` (ffmpeg/ffprobe) e `internal/vision` (tesseract OCR) são **Go-nativos operacionais**.
- `internal/worldmodel/vision` (GroundingDINO/SAM2/CLIP/Depth) são **adapters Python**: chamam `runSubprocess("python3", script)`. **No Windows quebram**: `python3` não resolve (é `python`), os scripts `adapters/vision/*.py` **não existem** no repo, e as dependências (torch/transformers) não estão instaladas. **= "código existe ≠ capacidade operacional existe".**
- O pipeline deterministico (probe→extractFrame→OCR→pixel-diff→eventos) foi provado **em Go** na POC.

**A causa raiz da quebra não é a visão; é a dependência Python+torch nos adapters de modelo.**

---

## 1. Decisão

**SIM** — a percepção/visão pode e deve ser **100% Go-nativa**, incluindo a inferência de modelos ML, via **ONNX Runtime** (`github.com/yalue/onnxruntime_go`, v1.35.0 disponível), convertendo os modelos (YOLO/CLIP/SAM/GroundingDINO/Depth) para **artefatos ONNX** e carregando-os **in-process** em Go.

- **Pipeline deterministico** (media/ffmpeg + OCR/tesseract + pixel-diff + tracker + epistemologia): **Go puro** (já provado).
- **Inferência ML** (detect/segment/classify/embed/depth): **Go + ONNX Runtime**, sem Python/torch.

---

## 2. Racional

1. **Monolith do COSCA (I1, single binary, stdlib):** subtools Python quebram o ethos de binário único e introduzem runtime/dependências.
2. **A evidência da quebra:** os únicos componentes que se mostraram **não-operacionais no Windows** foram os adapters Python (`python3` + torch + scripts ausentes). Go-native elimina a classe inteira desse gap.
3. **ONNX Runtime é maduro em Go** (`onnxruntime_go` v1.35.0; modelos YOLO/CLIP têm export ONNX público). É o caminho padrão para inferência Go.
4. **A primitiva cerebral já é nativa e generaliza** (`Observation`/`OBSERVED/INFERRED/UNKNOWN` + `Corroborate`/`Level`). Ela não depende de Python — é o foco da "consciência" (ADR-014).

---

## 3. Alternativas consideradas

| Opção | Veredito |
|---|---|
| **Manter adapters Python** (status quo) | ❌ quebra single-binary, depende de Python+torch, gap `python3`/Windows medido. |
| **Python em subprocesso controlado** (deps gerenciadas) | ⚠️ resolveria portabilidade mas mantém runtime externo + 2 runtimes (Go+Python). |
| **Pure-Go ML** (`onnx-go`) | ⚠️ menos maduro/lento; `onnxruntime_go` é o binding canônico. |
| **Go + ONNX Runtime** (escolhido) | ✅ nativo single-binary; modelos como artefatos; resolve a dúvida. |

---

## 4. Consequências

- **Pipeline de percepção** vira **nativo single-binary**: `media + OCR + pixel-diff + YOLO/CLIP(ONNX) + tracker + epistemologia`.
- **Modelos** viram **artefatos ONNX** (content-addressable, `internal/asset`), não dependências de runtime.
- **Dependência**: `onnxruntime_go` (CGO; lib ONNX Runtime embutida/bundled) — decidir no momento da implementação se mantemos o binding CGO ou empacotamos o runtime.
- **NÃO cristaliza arquitetura universal de percepção.** O escopo é **apenas resolver Python→Go** para o caminho de percepção. A `Observation` epistêmica é a peça que generaliza (não um sistema de 14 interfaces).
- **Risco mitigado**: só portar modelo a modelo conforme necessário (primeiro **YOLO** para detecção de entidade — o que a evidência mostrou ser o gap). CLIP/SAM/Depth **ficam fora até existir necessidade** (regra do professor: não adicionar porque parece útil).

---

## 5. Teste de aceite

- `go build ./...` limpo sem Python.
- Rodar um detector (YOLO via ONNX) num frame → `Observation{Object:'person', Epistemic:'OBSERVED', Confidence>0.6}`.
- Pipe `media.ExtractFrame → vision.Describe/Detect → Observation → Level()` sem subprocesso Python.
- `go vet` + testes passam.

---

*Autor: cosca-kernel. Decisão fundamentada em evidência medida (gap python3/Windows + maturidade do ONNX Runtime em Go). Resolve a dúvida: **SIM, tudo pode ser Go-nativo.***
