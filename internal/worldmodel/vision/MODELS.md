# Vision ONNX Models (native Go — no Python, no PyTorch)

The vision pipeline (`internal/worldmodel/vision`) runs **entirely in Go** via
[`github.com/yalue/onnxruntime_go`](https://github.com/yalue/onnxruntime_go).
It loads `.onnx` models and runs inference through the ONNX Runtime C library.
There are **no Python subprocesses** and **no PyTorch**.

> The pipeline is **best-effort by design**: if a model `.onnx` file is not
> present in the models directory, the corresponding adapter degrades
> gracefully — it returns a non-fatal warning and the agent keeps running. It
> never crashes on a missing model.

---

## Where the models live (sovereignty: never a fixed path)

The engine resolves the models directory in this order:

1. `$COSCA_MODELS` — explicit environment override.
2. `~/.cosca/models/vision/` — user home (cross-platform).
3. `./.cosca/models/vision` — last resort, CWD relative.

Check the status on your machine with:

```bash
cosca model vision            # human-readable table
cosca model vision --json     # machine-readable
cosca model vision --dir ~/.cosca/models/vision
```

The directory is mirrored in code by `vision.DefaultModelsDir()` /
`vision.ModelsDirFor(dir)`.

---

## The four models and where to download them

| Canonical file            | Model                          | Task                    | Get it from |
|---------------------------|--------------------------------|-------------------------|-------------|
| `clip_vitb32.onnx`        | OpenAI **CLIP ViT-B/32**       | Classification + image embeddings | [HF: openai/clip-vit-base-patch32 ONNX](https://huggingface.co/openai/clip-vit-base-patch32) (export as ONNX) |
| `sam2_hiera_large.onnx`   | Meta **SAM2** (hiera large)    | Prompt-based segmentation | [HF: facebook/sam2-hiera-large](https://huggingface.co/facebook/sam2-hiera-large) or [github.com/ChaoningZhang/... onnx exports](https://github.com) (export as ONNX) |
| `groundingdino_swint.onnx`| **GroundingDINO** (swin-t)     | Text-prompted object detection | [HF: IDEA-Research/grounding-dino-tiny](https://huggingface.co/IDEA-Research/grounding-dino-tiny) (export detector as ONNX) |
| `depth_anything_v2_vitl.onnx` | **Depth Anything V2** (ViT-La) | Monocular depth estimation | [HF: depth-anything/Depth-Anything-V2-Large](https://huggingface.co/depth-anything/Depth-Anything-V2-Large) (export as ONNX) |

### Steps

1. Create the models directory:
   ```bash
   mkdir -p ~/.cosca/models/vision
   ```
2. Export each model to ONNX (PyTorch → ONNX via `torch.onnx.export`) with the
   **input/output names kept stable** (the adapters introspect the graph's I/O
   names and shapes, so slightly different exports are tolerated). Place the
   result under the canonical filename above.
3. (Optional) Set `$COSCA_MODELS` to point the engine at a different directory.
4. Verify:
   ```bash
   cosca model vision
   ```

> **Note on export contracts.** The detectors/segmenters (GroundingDINO, SAM2)
> have many possible input/output signatures. The adapters read them via
> `onnxruntime_go.GetInputOutputInfo` and heuristically match output tensors by
> name ("box", "score", "mask", "depth", "embed"). If your export names differ,
> map them in the corresponding decoder (`decode.go`) or provide a
> matching export.

---

## Runtime prerequisite: ONNX Runtime shared library

The Go binary loads the ONNX Runtime C library on demand. On Windows it
resolves `onnxruntime.dll` through the default DLL search path.

**Important (verified on the target machine):** the `onnxruntime.dll` shipped
with Windows (`C:\Windows\System32\onnxruntime.dll`) is version **1.17.1**
(API version 17), but `onnxruntime_go v1.35.0` requires **API version 29**
(ONNX Runtime ≥ 1.20). The current smoke test
(`TestONNXRuntimeDLLLoadable`) detects this mismatch and **skips** (degradation),
but to run **real inference** you must supply a newer DLL, e.g.:

- Drop a newer `onnxruntime.dll` (≥ 1.20) next to the `cosca` binary, **or**
- set the DLL search path to a newer ONNX Runtime distribution, **or**
- downgrade `github.com/yalue/onnxruntime_go` to a version that matches
  ONNX Runtime 1.17 (e.g. the `v1.12.x` line) if you must stay on the
  shipped DLL.

The smoke test logs the mismatch:

```
The requested API version [29] is not available, only API versions [1, 17] are
supported in this build. Current ORT Version is: 1.17.1
```

---

## Automatic image detection

When an image (PNG/JPEG) enters the agent runtime, call
`vision.DetectImageAndRunVision(ctx, data, contentType)` in
`internal/worldmodel/vision/vision_detect.go`. It sniffs the content type,
runs the pipeline and returns an `Observation` (entities + relations) whose
`SummaryText()` is a ready-to-inject semantic string. It degrades gracefully
when models are absent.

**Wired (implemented):** the hook lives in `internal/cli/agent.go`
(`runAgentToolLoop`). Just before the conversation is sent to `provider.Chat`,
`internal/cli/agent_vision.go` (`enrichMessagesWithVision`) scans the messages
for `chat.Message.ContentParts[].ImageURL` data-URIs, decodes via
`vision.DecodeDataURI`, runs `DetectImageAndRunVision`, and appends the
`Observation.SummaryText()` as an extra text content part on the SAME message —
so the model receives "image + semantic understanding" in one turn, no LLM-vision
required. It is opt-in via `vision.enabled: true` (default `false`, no
regression) or `COSCA_VISION__ENABLED=true`, and never breaks the tool-calling
flow when no image is present or the pipeline degrades.
