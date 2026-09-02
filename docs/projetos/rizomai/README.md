# RIZOMAI — Certidão de Nascimento

> **Projeto**: RIZOMAI — Plataforma multi-rede social via API
> **Natureza**: Projeto INDEPENDENTE (não faz parte do CLARO, nem é produto Cosca)
> **Batismo**: 2026-09-02 (decisão do Don)
> **Status**: 🔵 ESTRUTURA CRIADA — aguardando Fase 1 (stack + ADRs)

---

## 1. O QUE É

Uma plataforma **API-first** de gestão de redes sociais: **um post → todas as redes**.
Posicionada para desenvolvedores e agentes de IA (inspirada na mineração da Zernio —
`docs/reports/zernio-mineracao/`), mas com identidade própria e foco **LATAM/PT-BR**.

## 2. NOME

- **RIZOMAI** — rizoma (conexões sem centro) + AI (agentes/automação).
- GitHub org/user `rizomai`: ✅ LIVRES (verificado 2026-09-02).
- Domínio: `rizomai.com` ocupado por terceiro — escolher alternativo (`.dev`, `.app`,
  `.com.br`) quando o projeto exigir.

## 3. POR QUE (dor e oportunidade)

- Ferramentas existentes (Zernio, Sprout, Buffer) ignoram o mercado LATAM: sem PT-BR,
  sem preço em R$, sem suporte humano.
- Demanda confirmada: mineração Zernio revelou modelo de negócio validado
  (usage-based por conta conectada) e gaps estruturais a explorar
  (1 conta por plataforma por profile — dor real de agência).

## 4. O QUE VAI ENTREGAR (blueprint resumido)

1. **API central** versionada `/v1`: Profiles → Accounts → Posts (fan-out
   `Post → PostTarget(status) → PublishAttempt`).
2. **Conectores**: Twitter/X, LinkedIn, Telegram (MVP); depois Instagram, YouTube,
   TikTok, Facebook, Bluesky, Reddit, Pinterest, Google Business, Snapchat.
3. **Publicação**: agendamento, fila, retry, status por plataforma, webhooks
   (HMAC-SHA256, event-id, delivery logs).
4. **Mídia**: upload direto (≤25MB) + presign (até 5GB).
5. **Analytics**: views, followers, engajamento por plataforma.
6. **Gestão**: equipe, API keys escopadas, quotas, billing usage-based.

## 5. PRINCÍPIOS (herdados da mineração)

- Spec-first: OpenAPI como fonte única de verdade; SDKs gerados.
- Idempotência em 2 camadas (Idempotency-Key + content-hash 24h).
- OAuth broker server-side centralizado (cliente nunca toca token).
- Health check de contas (alarme precoce de token expirado).
- Pricing usage-based por conta conectada (graduado) — sem cotas de posts.
- AI agents como canal de distribuição (MCP, llms.txt).

## 6. STATUS DOS INSUMOS

| Insumo | Local | Status |
|--------|-------|--------|
| Blueprint de arquitetura | `docs/reports/zernio-mineracao/arquitetura-zernio.md` | ✅ pronto |
| Padrões de integração | `docs/reports/zernio-mineracao/integracoes-zernio.md` | ✅ pronto |
| Análise de produto/pricing | `docs/reports/zernio-mineracao/produto-zernio.md` | ✅ pronto |
| Stack + ADRs | `docs/projetos/rizomai/adr/` | ⏳ Fase 1 |
| Scaffold | — | ⏳ após ADRs |

## 7. PRÓXIMO PASSO (aguardando ordem do Don)

**Fase 1**: delegar ao CTO/Architecture a decisão de stack + ADRs
(linguagem backend, fila, banco, estrutura de monorepo) com base nos relatórios.
