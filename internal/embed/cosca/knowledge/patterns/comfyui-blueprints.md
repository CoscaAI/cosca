# ComfyUI — Knowledge Package

> **Fonte**: https://github.com/Comfy-Org/ComfyUI/tree/master/blueprints (89 blueprints analisados)
> **Data**: 2026-08-02 | **Análise**: cosca-kernel | **Ecosystem**: python | **Repository**: comfy-org/comfyui
> **Status**: validated (blueprints oficiais) | **License**: GPL-3.0

## Resumo

ComfyUI é o motor de difusão node-based (Stable Diffusion, Flux, Wan, LTX) em Python. Os **blueprints** são workflows JSON reutilizáveis que o Cosca pode carregar, inspecionar e adaptar. Este pacote documenta a estrutura de blueprint (v0.4), os nodes mais usados e os fluxos canônicos — o conhecimento necessário para o Cosca "utilizar o ComfyUI".

## Estrutura de um Blueprint (formato v0.4)

```json
{
  "revision": 0,
  "last_node_id": 14,
  "last_link_id": 0,
  "nodes": [...],          // nós da UI (UUID type → definitions)
  "links": [...],          // conexões [id, from_node, from_slot, to_node, to_slot, type]
  "version": 0.4,
  "definitions": {
    "subgraphs": [ {       // o GRAPH real com os nodes executáveis
      "id": "<uuid>",
      "name": "Text to Image",
      "nodes": [ { "id", "type", "pos", "size", "order", "mode",
                   "inputs", "outputs", "widgets_values" } ],
      "links": [...],
      "category": "...",
      "description": "..."
    } ]
  },
  "extra": {}
}
```

**Padrão chave**: o `type` dos nodes na UI é um **UUID** que resolve para o subgraph; dentro do subgraph, `type` é o **nome real do node** (`CLIPLoader`, `KSampler`, etc.). O Cosca deve ler `definitions.subgraphs[].nodes[].type` para entender o workflow real.

## Nodes mais usados (89 blueprints — 182 distintos)

### Core de difusão (text-to-image/video)
| Node | Uso | Inputs | Outputs |
|---|---|---|---|
| `UNETLoader` | Carrega o modelo de difusão | unet_name, weight_dtype | MODEL |
| `CLIPLoader` / `DualCLIPLoader` | Text encoder (clip_l, t5xxl) | clip_name1/2, type | CLIP |
| `VAELoader` | Autoencoder | vae_name | VAE |
| `VAEDecode` / `VAEEncode` | Latente ↔ imagem | samples, vae | IMAGE / LATENT |
| `EmptySD3LatentImage` | Latente inicial | width, height, batch_size | LATENT |
| `CLIPTextEncode` | Prompt → condicionamento | clip, text | CONDITIONING |
| `KSampler` | Amostragem | model, positive, negative, latent_image, seed, steps, cfg, sampler, scheduler | LATENT |
| `KSamplerSelect` | Escolhe sampler | sampler_name | SAMPLER |
| `SamplerCustomAdvanced` | Amostragem avançada | model, add_noise, noise_seed, cfg, positive, negative, sampler, sigmas, latent_image | LATENT |
| `RandomNoise` / `CFGGuider` | Noise/guia para avançado | — | NOISE / GUIDER |
| `LoraLoaderModelOnly` | LoRA no modelo | model, lora_name, strength | MODEL |
| `CheckpointLoaderSimple` | Checkpoint completo | ckpt_name | MODEL, CLIP, VAE |
| `ModelSamplingAuraFlow` | Ajuste de sampling | model, shift | MODEL |

### Utilitários (primitivas e lógica)
| Node | Uso |
|---|---|
| `PrimitiveInt` / `PrimitiveFloat` / `PrimitiveBoolean` | Valores primitivos (inputs de UI) |
| `ComfySwitchNode` | Switch de fluxo |
| `ComfyMathExpression` | Expressão matemática |
| `GetImageSize` / `GetVideoComponents` | Metadados de mídia |
| `StringReplace` | Substituição de string |
| `CreateVideo` | Monta vídeo de frames |
| `ResizeImageMaskNode` | Redimensiona máscara |
| `GLSLShader` | Shader custom |
| `MarkdownNote` | Anotação no workflow |

### Edição de mídia (blueprints de pós-processamento)
Canny/Depth → image/video (Z-Image-Turbo, LTX 2.0), Character Replacement (SCAIL-2), ControlNet, Color Adjustment/Balance/Curves, Crop 2x2/3x3, Edge-Preserving Blur, Film Grain, First-Last-Frame, Unsharp Mask, Video Inpainting (Wan2.1 VACE, VOID), Video Segmentation (SAM3), Video to Pose (SDPose), Video Upscale (GAN x4).

## Fluxos canônicos

### Text to Image (Flux.1 Dev — padrão)
```
VAELoader ─┐
UNETLoader ─┤→ KSampler → VAEDecode → IMAGE
DualCLIPLoader ─┤  (model, positive, negative, latent)
EmptySD3LatentImage ─┘
CLIPTextEncode (prompt) → positive/negative
```

### Text to Video (LTX-2.3 / Wan 2.2)
```
UNETLoader (video model) → sampler → CreateVideo → frames
GetVideoComponents → ... (manipulação de vídeo)
```

## Como o Cosca utiliza

1. **`cosca knowledge add https://github.com/comfy-org/comfyui`** → manifesto com repository (já feito)
2. **Carregar blueprint**: ler `definitions.subgraphs[].nodes[]` — o `type` real dos nodes
3. **Validar modelo**: o Cosca detecta via `cosca hardware` (GPU/VRAM) qual fluxo usar (Flux.1 Dev precisa de ~24GB; Z-Image-Turbo roda em menos)
4. **Adaptar**: trocar `widgets_values` (modelo, tamanho, steps) conforme a máquina
5. **Executar**: submeter o JSON ao servidor ComfyUI (`POST /prompt`)

## Riscos / Notas

- Blueprint é **dados, não código** — seguro para adquirir (nunca executa sozinho)
- O Cosca **não instala** ComfyUI automaticamente (KNOWLEDGE ≠ DEPENDENCY ≠ CODE); fornece o conhecimento para o agente orientar a instalação
- Modelos: Flux.1 Dev (grande), Z-Image-Turbo (rápido), LTX/Wan (vídeo) — escolha conforme VRAM
