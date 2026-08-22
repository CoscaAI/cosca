---
type: pattern
key: tensorart-openapi-integration
tags: [pattern, ai-image, api, comfyui, tensorart, integration]
timestamp: 2026-08-08T00:00:00Z
status: active
agent: API Chief
category: integration
confidence: 0.85
times_used: 1
times_succeeded: 1
---

# TensorArt OpenAPI Integration Pattern

## Intent

Integrar agentes e pipelines (ComfyUI, skills de IA) com a geração de imagens/vídeos via TensorArt OpenAPI, de forma assíncrona e sem expor credenciais no prompt.

## Context

- Agente precisa gerar imagem/vídeo por IA externa (TensorArt/Tusi)
- Upload de mídia de referência local
- Tarefas longas com polling
- Integração com ComfyUI (custom nodes) ou skills de agente

## Solution

### 1. Autenticação e multi-host
- Header `Echo-Access-Key` com a chave (lida de `~/.tensor_access_key`)
- Chave com prefixo define o host: `ak_tusi` → `openapi.tusiart.cn`, `ak_tensor` → `openapi.tensor.art`
- Fallback `DEFAULT_BASE_URL`

### 2. Fluxo de geração (5 passos)
1. **Listar tools** — descobrir inputs/outputs/custo estimado (schema dinâmico, nada hardcoded)
2. **Upload** (se FILE input) — POST `file/upload` → URL presignada Cloudflare R2 → PUT direto (nunca enviar binário pela API principal)
3. **Criar task** — POST `task` com `{toolName, inputs}` → `taskId`
4. **Polling** — POST `task/query` com `taskIds[]`, intervalo 3s, máx 60 iterações; estados terminais: `FINISH`/`EXCEPTION`/`CANCELED`
5. **Download** — resultado pela URL retornada

### 3. Contrato de scripts (padrão skill)
- Um script por responsabilidade: `list_tools.py`, `create_task.py`, `query_task.py`, `upload_file.py`, `download_result.py`
- JSON em stdout (contrato), logs em stderr
- Códigos de saída distintos: 0=FINISH, 1=erro/cancelado, 2=timeout
- Stdlib-only (urllib) — zero dependências externas

### 4. Integração ComfyUI (custom nodes)
- `NODE_CLASS_MAPPINGS` + `NODE_DISPLAY_NAME_MAPPINGS` no `__init__.py`
- Node de Settings (baseUrl/apiKey) via manager singleton
- Node AITools faz discovery e render dinâmico de parâmetros (image/combo especiais)
- Node Execute + TA_Websocket para progresso em tempo real
- `server.py` registra rotas auxiliares no PromptServer

### 5. Conversão tensor→PIL (ComfyUI)
- `image.detach().cpu().numpy()`; tratar [B,C,H,W]→primeiro item; [C,H,W]→transpor
- Modo pelo canal: 1→'L', 3→'RGB', 4→'RGBA'; multiplicar por 255 (float→uint8)

## Consequences

**Benefits:**
- Geração de imagem/vídeo sem expor credenciais no prompt do LLM
- Upload presignado não bloqueia o servidor
- Schema dinâmico (tools) dispensa atualização do cliente quando a API evolui
- Skill autocontido, distribuível via `npx skills add`

**Drawbacks:**
- Polling adiciona latência (vs push)
- Multi-host por prefixo é convenção proprietária
- Custom nodes exigem reinício do ComfyUI para carregar

## Known Uses

- `tensorart-skills` (tensorart-generate) — skill de agente
- `ComfyUI_TENSOR_ART` — custom nodes (AITools, Execute, UploadImage)
- `image-mcp-server` — servidor MCP de geração de imagem

## Related Patterns

- `api-patterns.md` (error envelope, versioning)
- `comfyui-blueprints.md` (node-based diffusion)
- Upload presignado (S3-style) — padrão geral de mídia
