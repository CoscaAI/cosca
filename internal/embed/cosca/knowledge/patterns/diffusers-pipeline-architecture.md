---
type: pattern
key: diffusers-pipeline-architecture
tags: [pattern, diffusion, ai-image, pytorch, lora, huggingface, production]
timestamp: 2026-08-08T00:00:00Z
status: active
agent: AI Chief
category: design
confidence: 0.88
times_used: 1
times_succeeded: 1
---

# Diffusers Pipeline Architecture Pattern

## Intent

Construir e servir aplicações comerciais de geração de imagem/vídeo (text-to-image, img2img, inpainting, ControlNet) sobre a biblioteca Diffusers da HuggingFace, com fine-tuning LoRA e otimização para GPUs limitadas.

## Context

- App comercial de geração de imagem (SD1.5, SDXL, Flux)
- Fine-tuning de estilo/identidade com LoRA
- VRAM limitada (GPUs AMD/ROCm ou NVIDIA de entrada)
- Deploy com fila assíncrona (padrão create → poll → download)

## Solution

### 1. Arquitetura do pipeline (4 componentes)
1. **Text encoder** — prompt → embeddings (SDXL: CLIP + OpenCLIP)
2. **Scheduler** — algoritmo do denoising (DDPM, DDIM, Euler, DPM-Solver++)
3. **UNet ou DiT** — o motor; prevê ruído a cada passo (loop de N steps)
4. **VAE** — codifica pixels → latentes comprimidos (o UNet opera no latent space); decodifica no final

### 2. Treino LoRA (padrão Diffusers)
```python
unet_lora_config = LoraConfig(
    r=rank, lora_alpha=rank, init_lora_weights="gaussian",
    target_modules=["to_k", "to_q", "to_v", "to_out.0"],
)
# unet/vae/text_encoder → device com weight_dtype
# Accelerate para device/AMP/distributed
# salvar com get_peft_model_state_dict
```

### 3. Otimização para VRAM limitada
- `torch_dtype=torch.float16` + `variant='fp16'` + `use_safetensors=True`
- `enable_model_cpu_offload()` (em vez de `.to('cuda')`) — UNet na GPU, VAE/encoders em CPU
- `enable_attention_slicing()` como fallback
- Scheduler rápido (DPM-Solver++/Euler, 20-30 steps) para inferência
- Quantização bitsandbytes 4/8-bit para o UNet

### 4. Produção (server)
- Carregar pipeline UMA vez no boot (model warm)
- Fila assíncrona para geração longa: request → taskId → polling
- fp16 + safetensors + rate limiting + cache de prompts
- Health check do modelo

### 5. Stack completa para app comercial
- **Diffusers** = motor (inferência/treino PyTorch)
- **ComfyUI** = pipeline visual (grafos de nodes sem código)
- **TensorArt OpenAPI** = serviço cloud escalável (ver pattern tensorart-openapi-integration)

## Consequences

**Benefits:**
- API simples e consistente (`from_pretrained` + `pipeline(prompt)`)
- Fine-tuning LoRA barato e rápido (adapter pequeno, fusível no modelo)
- Multi-plataforma (CPU/GPU NVIDIA/AMD via Accelerate)
- Comunidade enorme, modelos prontos no HF Hub

**Drawbacks:**
- Default CPU/float32 é lento — otimização é obrigatória para produção
- Baixa abstração (pipelines copiados-colados) gera duplicação no código-fonte
- Modelos grandes exigem offload/quantização em VRAM < 8GB
- Licenças variam por modelo (SDXL permissiva, Flux com restrições)

## Known Uses

- Stable Diffusion / SDXL / Flux pipelines oficiais
- Exemplos text_to_image (LoRA, SDXL) e server-async do repo
- Base do ComfyUI e de serviços cloud como TensorArt

## Related Patterns

- `tensorart-openapi-integration.md` — camada cloud/API
- `comfyui-blueprints.md` — camada visual/pipeline
- API async job pattern (create → poll → download)
