# cosca-integrations — Learnings

> Semantic learning journal do Integrations Chief. Cada entrada é uma técnica/descoberta registrada conforme o LEARNING_PROTOCOL v2.0.0.

## Índice
- [2026-09-01 — Mineração de SDKs via API pública do GitHub](#2026-09-01--mineração-de-sdks-via-api-pública-do-github)
- [2026-09-01 — Padrões de integração social multi-rede (Zernio/Late)](#2026-09-01--padrões-de-integração-social-multi-rede-zerniolate)
- [2026-09-05 — Mineração CLI-Anything: protocolo de preview-bundle e harnesses agente-native](#2026-09-05--mineração-cli-anything-protocolo-de-preview-bundle-e-harnesses-agente-native)

---

### 2026-09-01 — Mineração de SDKs via API pública do GitHub

| Field | Value |
|-------|-------|
| **Agent** | cosca-integrations |
| **Task** | Minerar os 27 repos da org zernio-dev (SDKs sociais) sem clonar |
| **Technique** | Mineração read-only: API repos + git trees (recursive) para mapear estrutura + raw.githubusercontent para conteúdo; `cosca.web` para conteúdo (trunca ~8k chars) |
| **Level** | 3 |
| **Outcome** | success |
| **Tags** | #github #research #sdk #mining #api |
| **Related** | GitHub REST API, openapi-generator |
| **Learned** | O JSON de `/orgs/{o}/repos` é pesado (cada repo ~2.5k chars) e trunca no cosca.web → extrair só name/default_branch/language via Invoke-RestMethod. A árvore `git/trees/{branch}?recursive=1` é o jeito mais denso de mapear um repo inteiro (path por linha). `docs/*.md` de SDKs gerados por OpenAPI Generator revelam TODOS os endpoints de graça. Repos-skill (SKILL.md + rules/*.md) são documentação executável de altíssimo valor. |
| **Next** | Para próxima mineração: montar "superárvore" de todos os repos numa única chamada e rankear arquivos por valor (rules/, docs/, src/client) antes de baixar conteúdo. |

---

### 2026-09-01 — Padrões de integração social multi-rede (Zernio/Late)

| Field | Value |
|-------|-------|
| **Agent** | cosca-integrations |
| **Task** | Extrair padrões de OAuth/publicação/mídia/erros dos SDKs zernio-dev para a nossa plataforma multi-rede |
| **Technique** | Reverse-engineering de convenções de API agregadora: auth Bearer sk_ (67 chars), versionamento no path (/v1), modelo Profile→Account→Post, posts multi-plataforma com `platforms[]` + `platformSpecificData`, presign 5GB vs upload direto 25MB/7d, envelope de erro {error, code, details}, rate limit por plano (60-1200 rpm) com headers X-RateLimit-*, webhooks HMAC-SHA256, quota com lastReset+billingAnchorDay |
| **Level** | 3 |
| **Outcome** | success |
| **Tags** | #oauth #social #presign #webhooks #rate-limit #twitter #linkedin #telegram #instagram #tiktok #youtube |
| **Related** | twitter PKCE S256, instagram ig_exchange_token (1h→60d), linkedin X-RestLi/LinkedIn-Version headers, bluesky AT Protocol app-password, tiktok UX compliance audit, instagram 2207051 (blocked-mas-publicou), reddit user-agent, snapchat AES-256-CBC allowlist |
| **Learned** | (1) Cliente nunca faz OAuth — centralizar no servidor; expor só connect URLs + seleção. (2) Erros tipados por status com helpers isAuthError/isRateLimited + RateLimitError com getSecondsUntilReset. (3) Twitter tem 3 níveis de rate limit (app 24h/user 24h/endpoint) via headers próprios. (4) SDK bom = spec-first (OpenAPI gera client) + camada manual (erros, rate limiter, uploader smart <4MB direto / ≥4MB presign, paridade sync/async). (5) Webhook account.disconnected detecta token expirado — essencial p/ saúde de contas. (6) Contagem de chars do Twitter: URL=23, emoji/CJK=2. (7) Hosts problemáticos (Drive/Dropbox/OneDrive) precisam re-host. (8) IDs do IG/FB > 15 dígitos → parse como string. |
| **Next** | Desenhar o conector Twitter/X da nossa plataforma com PKCE + refresh job + 3-tier limits; validar tabela de limites de mídia por plataforma com teste de aceitação. |

---

### 2026-09-05 — Mineração CLI-Anything: protocolo de preview-bundle e harnesses agente-native

| Field | Value |
|-------|-------|
| **Agent** | cosca-integrations |
| **Task** | Minerar HKUDS/CLI-Anything (clone shallow em temp) e extrair harnesses de maior valor para os domínios do COSCA (visão, áudio, 3D, Blender, média, nodegraph, gameengine, conhecimento) |
| **Technique** | Minerar harness "agente-native": clone shallow + PDFs `agent-harness/<NAME>.md` (SOP) + `skills/SKILL.md` + `core/*.py` + `utils/*.py`; extrair 3 eixos (capacidades→comandos, modelo de estado/objetos JSON, preview/feedback ao agente) |
| **Level** | 4 |
| **Outcome** | success |
| **Tags** | #harness #agent-native #json-state #preview-bundle #mcp-backend #blender #comfyui #audacity #knowledge #nodegraph #feedback-loop |
| **Related** | CLI-Anything, DOMShell MCP, ComfyUI REST, Zotero connector, Obsidian REST, bpy/macro-gen, ffmpeg |
| **Learned** | (1) **Formato binário nativo é intocável** → manter estado em JSON e **gerar o artefato** (script bpy / macro FreeCAD / .kra / .tscn / MLT XML / .drawio / SVG). (2) **Preview bundle** (`manifest.json` [artifacts, summary{headline,facts,warnings,next_actions}, metrics, labels, status, cache_key], `_bundle_dir`, `_trajectory_path`) é o contrato de feedback de percepção: renderiza 2 artefatos reais (hero+workbench) por snapshot e guarda `summary.next_actions` para o agente. (3) **Live session** com `poller` + **fingerprint sha256(path,size,mtime)** evita re-render; `trajectory.json` = histórico append-only. (4) ComfyUI: workflow JSON node graph `{id:{class_type,inputs}}` + validador de grafo + ciclo prompt→status→history→outputs[refs de imagem]. (5) **MCP-as-backend** (DOMShell): bridge de acessibilidade do Chrome exposto como filesystem (ls/cd/cat/grep/click) + split-and-check com gates `_is_error`; conceito "accessibility tree = percepção navegável". (6) Conhecimento: Zotero `item context`→`prompt_context` = contrato RAG; separação `item get` (lookup) vs `item find` (descoberta); guardrails `--experimental` + backup + transação + rollback. |
| **Next** | Adotar o protocolo **preview-bundle** (manifest + artifacts + summary.next_actions) como contrato de feedback percepção do COSCA; portar os padrões JSON-state→artifact-gen para Blender/comfyui/audacity; explorar DOMShell MCP como "loop de percepção" (visão/áudio). |

---
