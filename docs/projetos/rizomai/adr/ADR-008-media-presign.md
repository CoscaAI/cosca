# ADR-008: Mídia — **upload direto ≤25 MB** + **presign até 5 GB** (S3/R2) com streaming, sem buffer em memória

> **Status:** Proposta 🔵 (aguarda aprovação do Don)
> **Owner:** cosca-architecture (Architecture Chief) | **Last Updated:** 2026-09-01
> **Projeto:** RIZOMAI — plataforma API-first de gestão de redes sociais
> **Referência (base):** `arquitetura-zernio.md` §9 (uploads), §11 R6; `integracoes-zernio.md` §3 (media, limites por plataforma, streaming).

---

## 0. Contexto

Posts carregam mídia (imagens, vídeos) com limites por plataforma que chegam a **5 GB** (LinkedIn) e
a formatos/aspectos específicos por rede (TikTok 9:16, IG 8 MB imagem…). Trafegar binário pelo
gateway em upload grande = timeout, OOM, custo de banda no servidor. A lição Zernio: **presign
próprio (S3-style) desde o início** (eles migraram de Vercel Blob → presign) + **upload direto para
arquivos pequenos** (retenção curta). Streaming (nunca carregar arquivo inteiro em memória) é regra
de ouro para workers que baixam mídia antes de publicar.

## 1. Decisão

**Dois fluxos de upload, separados por tamanho, com storage S3-compatível:**

| Fluxo | Endpoint | Limite | Retenção | Uso |
|---|---|---|---|---|
| **Direto** | `POST /v1/media/upload` (multipart) | **≤ 25 MB** | 7 dias (auto-delete por job `pg_cron`/River) | inbox/mensagens, thumbnails, uploads pequenos |
| **Presign** | `POST /v1/media/presign` `{filename, contentType}` → `{uploadUrl, publicUrl}` | **até 5 GB** | permanente (mídia do post) | vídeos e arquivos grandes |

Fluxo presign (o cliente faz o PUT direto no storage, sem passar pelo gateway):
```
1. POST /v1/media/presign {filename, contentType} → {uploadUrl, publicUrl}
2. PUT  uploadUrl  (body = arquivo, header Content-Type)   ← streaming, direto no bucket
3. Usar publicUrl em mediaUrls[] do createPost
```

1. **Storage: Cloudflare R2** (S3-compatível) como escolha padrão — **zero egress fee** (relevante
   para LATAM e para workers que re-servem mídia às plataformas), custo por armazenamento baixo;
   **S3 (AWS) como fallback** via interface S3-compatível (troca de endpoint/config, sem mudança de
   código). Acesso por **credenciais com escopo mínimo** (presign gerado server-side; o cliente só
   recebe URL temporária).
2. **Streaming obrigatório**: qualquer download/processamento de mídia (worker de publicação,
   re-host de URLs externas) usa `io.Copy`/streaming por chunk — **nunca** `ReadAll` do arquivo em
   memória. Protege workers de OOM com arquivos de 5 GB.
3. **Validador por plataforma centralizado** (em `internal/media/`): tabela de limites
   (imagem/vídeo/formato/aspecto/duração por rede — minerada no relatório) validada no `createPost`,
   com erro tipado (`VALIDATION_ERROR` + `details.fields`). Ex.: TikTok vídeo 4 GB/9:16, IG imagem
   8 MB, LinkedIn PDF 100 MB, YouTube thumbnail 2 MB.
4. **Re-host de URLs externas problemáticas**: antes de publicar, mídia vinda de hosts não
   publicáveis (Google Drive, Dropbox/1drv.ms) é baixada (streaming) e re-servida do nosso storage;
   nunca publicar direto de host que devolve HTML.
5. **Uploads das plataformas**: seguir o fluxo nativo de cada rede (Twitter chunked
   INIT/APPEND/FINALIZE + poll; YouTube resumable; IG/TikTok direto) na camada `internal/platform/`
   — sempre com streaming e polling de `processing_info` (timeout ~5 min).

### Retenção e custo

- Upload direto (7 dias) cobre uso transitório sem custo de armazenamento permanente.
- Presign (permanente) é o que vai no post; `publicUrl` é o contrato com as redes.
- Job de limpeza de mídia expirada (River/pg_cron) roda diariamente — idempotente.

## 2. Consequências

**Prós**
- **Escala sem proxy**: arquivos de 5 GB nunca atravessam o gateway (sem timeout/OOM/custo de banda).
- **Custo previsível**: R2 paga por storage, sem egress; LATAM se beneficia (tráfego para redes
  brasileiras e regionais).
- Streaming seguro para workers; validador centralizado evita erro por plataforma no runtime.
- Interface S3-compatível = portabilidade (R2 ↔ S3 ↔ MinIO) sem lock.

**Contras**
- **R2 = dependência da Cloudflare** (mitigado: interface S3-compatível + fallback documentado).
- Upload direto de 7 dias exige job de limpeza confiável (e documentar a política no contrato —
  anti-surpresa; Zernio teve exatamente essa pegadinha).
- Geração de presign exige segurança: bucket privado, URLs assinadas com TTL curto, rotação de
  credenciais (review de segurança obrigatória).
- Validador por plataforma precisa de manutenção contínua (limites mudam) — tabela versionada.

## 3. Alternativas consideradas

1. **Upload multipart até 5 GB no gateway** — **rejeitado**: OOM/timeout/custo de banda; o servidor
   de API não deve trafegar binário grande (lição Zernio §9).
2. **Vercel Blob / objeto gerenciado de terceiro** — **rejeitado**: lock + custo por operação;
   a Zernio migrou dele para presign próprio.
3. **S3 AWS como única escolha** — aceito como alternativa, **não** padrão: egress fee custa caro no
   cenário LATAM com tráfego de mídia constante; R2 vence em custo total.
4. **Sem validação de mídia no servidor (deixa a rede rejeitar)** — **rejeitado**: péssima UX
   (falha só no publish) e gera `post.partial` evitável.

## 4. Referências

- `arquitetura-zernio.md` §9 (presign até 5 GB; upload direto; `publicUrl`) e §10 P1.
- `integracoes-zernio.md` §3 (fluxo presign/direto; limites por plataforma; regra "stream, nunca
  buffer"; re-host de Drive/Dropbox; uploads Twitter/YouTube).
- ADR-004 (`internal/media/`), ADR-005 (contrato `presign/upload`), ADR-007 (mediaUrls no createPost).

---

*ADR de Fase 1 — RIZOMAI. Decisão de stack, aguardando aprovação do Don.*
