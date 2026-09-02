# ADR-006: OAuth broker **server-side centralizado** — cliente usa só API key; tokens criptografados em repouso; clone de conexão

> **Status:** Proposta 🔵 (aguarda aprovação do Don)
> **Owner:** cosca-architecture (Architecture Chief) | **Last Updated:** 2026-09-01
> **Projeto:** RIZOMAI — plataforma API-first de gestão de redes sociais
> **Referência (base):** `arquitetura-zernio.md` §6 (auth) e §10 P1; `integracoes-zernio.md` §1.2/§1.3 (OAuth por plataforma) e §6 (lições X/LinkedIn/Telegram).

---

## 0. Contexto

O maior custo de quem integra redes sociais é o **OAuth por plataforma**: 16 fluxos diferentes
(PKCE, token exchange em 2 passos, seleção de página/org/board, App Password, bot token), refresh,
rate limits, health. A Zernio resolve isso centralizando: o cliente **nunca toca token** — só troca
uma API key por operações. O fluxo de conexão vira `GET /connect/{platform}?profileId= → authUrl` →
callback server-side. Queremos o mesmo, com segurança de tokens como requisito LGPD (repouso
criptografado) e suporte a multi-tenancy de agência (clone de conexão entre profiles).

## 1. Decisão

**O RIZOMAI opera um OAuth broker server-side centralizado.** O cliente autentica na API com API
key (`sk_...`, Bearer) e **nunca recebe nem armazena tokens de rede social**.

1. **Fluxo padrão de conexão**:
   - `GET /v1/connect/{platform}?profileId=...` → `{ "authUrl": "..." }` (redirect do usuário;
     `code_verifier` embutido no `state` para PKCE — lição X/Twitter).
   - Callback **no servidor RIZOMAI** (`/v1/connect/{platform}/callback`); troca de `code` por token;
     fluxos de seleção (`select-page/org/board/...`) com `pendingDataToken` (uso único, expira em
     10 min, sem auth — protegido por header `X-Connect-Token` quando via API key).
   - Fluxos **não-OAuth** na mesma abstração: Bluesky (App Password), Telegram (bot token + código),
     Google Business (verificação de localização).
2. **Armazenamento seguro de tokens**:
   - Criptografia **AES-256-GCM em repouso** (chave mestre via secret manager/KMS ou env criptografado;
     nunca em plaintext no banco). Campo `encrypted_token` + `token_status` + metadados de escopo.
   - **Nunca expor o token na API**: apenas `tokenStatus` (`ok|expired|revoked|needs_attention`) e
     permissões via `GET /v1/accounts/{id}/health`.
   - **Refresh automático**: job `oauth-refresh` (River — ADR-003) renova tokens expiráveis (X 2h,
     IG 60d, LI 60d/365d) e aciona webhook `account.disconnected` quando irreversível.
3. **Health check de conta** (alarme precoce): `GET /v1/accounts/health` e `/accounts/{id}/health` —
   token vs permissões vs link vivo (ex.: WhatsApp `platformConnection` probe). Base para billing
   preciso (account-day) e para o usuário saber que a conta morreu antes de tentar publicar.
4. **Clone de conexão** (diferencial de agência): `POST /profiles/{profileId}/clone-connection` com
   `sourceAccountId` (+ `targetPageId`/`targetOrganizationId`) — **reusa autorização OAuth entre
   profiles/páginas/orgs sem re-autenticar** (multi-contas por plataforma por profile = dor real da
   agência que a Zernio não resolve — produto §8.1).
5. **Move**: `PATCH /v1/accounts/{id}` move conta entre profiles (operacionalidade de agência).

### Escopo por plataforma no MVP

| Plataforma | Fluxo | Detalhe crítico |
|---|---|---|
| X/Twitter | OAuth 2.0 PKCE (S256) | verifier no state; refresh 2h; 3 níveis de rate limit |
| LinkedIn | OAuth 2.0 + seleção org | headers `X-RestLi-Protocol-Version: 2.0.0` + `LinkedIn-Version` por endpoint |
| Telegram | Bot token + código | sem OAuth; `chatId` direto p/ power user; sanitizar HTML |

(Fase 2+: IG 2-pass, TikTok UX-compliance, YouTube `access_type=offline`, Bluesky App Password,
Reddit `duration=permanent` + UA descritivo, etc. — cada um documentado no pacote `internal/platform/`.)

## 2. Consequências

**Prós**
- **Contrato trivial para o cliente** ("conecte com 1 chamada") — o valor que vendemos.
- **Segurança concentrada**: token só no servidor, criptografado em repouso (LGPD), rotação
  centralizada, least-privilege por scope.
- **Multi-tenancy limpo**: `accountId → profile`, move + clone para agências.
- Health check proativo reduz falhas de publicação e suporte.
- Compatível com AI agents (fluxo headless com `pendingDataToken`).

**Contras**
- **Trabalho inicial alto por plataforma** (cada OAuth é um mini-projeto; mitigado: pacotes isolados
  em `internal/platform/` + specs oficiais mineradas).
- **Responsabilidade de segurança concentrada** (mitigado: vault, rotação, revisão de segurança;
  compensa pelo modelo de negócio).
- Callbacks precisam de roteamento estável e retry (jobs de refresh; webhook `account.disconnected`).
- Seleção de página/org é fluxo de UI server-side (mais uma superfície a construir).

## 3. Alternativas consideradas

1. **OAuth no cliente (SDK guarda o token)** — modelo antigo (Ayrshare). **Rejeitado**: vaza token,
   refresh complexo por app cliente, multi-tenancy quebrado, impossível para AI agents/headless.
2. **Depender de libs open-source por plataforma** — **rejeitado**: manutenção externa incerta;
   preferimos pacotes próprios finos contra specs oficiais (documentadas por conector).
3. **Terceirizar o OAuth para um agregador (Zernio/Ayrshare)** — **rejeitado**: é o concorrente;
   o OAuth broker é exatamente o "miolo" que dá valor ao nosso produto (e à margem).
4. **Guardar token plaintext** — **rejeitado** por LGPD/segurança; sem discussão.

## 4. Referências

- `arquitetura-zernio.md` §6/§10-P1 e §11 R5 (risco de wrappers não-oficiais — só integração oficial).
- `integracoes-zernio.md` §1.2/1.3/§6 (fluxos por plataforma, PKCE, LinkedIn, Telegram).
- `produto-zernio.md` §8.1 (multi-contas por plataforma = diferencial de agência).
- ADR-003 (jobs de refresh via River), ADR-004 (`internal/oauth` + `internal/platform/`).

---

*ADR de Fase 1 — RIZOMAI. Decisão de stack, aguardando aprovação do Don.*
