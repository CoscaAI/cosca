# Mineração de Produto/Negócio — Zernio (zernio-dev / zernio.com)

> **Objetivo:** extrair aprendizados de produto, preços e proposta de valor da Zernio para subsidiar a construção da **nossa plataforma concorrente**. Nenhuma propriedade intelectual da Zernio será copiada — apenas padrões de negócio.
> **Data:** 2026-09-01/02 · **Analista:** Product Chief (cosca-product)
> **Epistemologia:** dados coletados via web (site, docs, GitHub, llms.txt) em 2026-09-01. Tudo abaixo é observação de fontes públicas, com nível de confiança marcado onde aplicável.

---

## 1. Sumário executivo

A Zernio não é mais uma "plataforma de agendamento de posts". Ela se reposicionou como **"marketing infrastructure"**: uma API única que cobre social publishing, inbox (DMs/comentários), analytics, ads e comunicação (telefone/SMS/WhatsApp) para **desenvolvedores e AI agents**. É o clássico play "API-first como produto de infraestrutura", executado com rara disciplina de GTM voltada a agentes de IA (MCP server, llms.txt, páginas por ferramenta de agente, CLI com JSON out).

O modelo de preços **atual** é **usage-based por conta conectada** ($6/$3/$1 por conta/mês, graduado; 2 primeiras grátis para sempre, sem cartão) — não os tiers Free/Build/Accelerate/Unlimited descritos no briefing (esses são o modelo **legado**, anterior ao pivô; restos dele ainda aparecem na API, ex.: `planName: "Pro"`). O pivô de "planos com cotas" → "usage-based, tudo incluso, sem seats" é em si uma das maiores lições.

**Números de mercado observados:** 70k+ devs anunciados, 717 followers no GitHub, org criada em 2026-01 (muito jovem), 27 repos públicos, 99.7% uptime anunciado, 2M+ posts entregues (FAQ), 496 tools no MCP, 369 paths na OpenAPI, 16-17 plataformas sociais + 7 redes de ads.

---

## 2. Proposta de valor e posicionamento

### 2.1 Posicionamento (frases reais da marca)

- Homepage title: **"Zernio — Social Media & Messaging API for Developers & AI Agents"**
- Hero: *"The social media and messaging API for developers and AI agents. One API to publish, message, advertise, and talk to your customers across 16 channels."*
- Org GitHub: *"Social & messaging for developers and AI agents."*
- Tagline de dev: **"Ship social features, not sixteen OAuth integrations."**
- Segmento: *"Zernio is built for developers and teams with technical resources. If you can make API calls, you can use Zernio. We also integrate with no-code tools like n8n, Make, and Zapier."*
- Posicionamento de plataforma: *"The API is closed. Everything we build on top of it is open, and it is all here."* (API proprietária; tooling/ecossistema 100% open source)

### 2.2 O que isso significa em linguagem de produto

| Camada | Proposta |
|---|---|
| Dor resolvida | 16 integrações OAuth + token rotation + rate limits + contratos por plataforma |
| Promessa | **Uma integração** cobre social, messaging, voice e ads; mudanças de plataforma são absorvidas pela Zernio |
| Quem compra | Devs, builders, agências técnicas, times com recursos técnicos, e — cada vez mais — **AI agents** |
| Meta | "Go from signup to your first post, message, report, or ad in under 5 minutes." |
| Prova social | Warner Music Group, ClickUp, RE/MAX, HeyMark, Holo, Vibiz; depoimento de Guillermo Rauch (CEO da Vercel); contador vivo "2,587 accounts connected this week" |

**Insight:** a Zernio vende **redução de complexidade integradora** (o "miolo" caro e chato do social), não agendamento. O agendamento é só o gancho de entrada (landing page de postagem). A receita real vem de **accounts + mensageria + ads + telephony**.

---

## 3. Superfície de produto (o que vende / features-gancho)

### 3.1 Plataformas
- **16 sociais:** Twitter/X, Instagram, Facebook, LinkedIn, TikTok, YouTube, Pinterest, Reddit, Bluesky, Threads, Google Business, Telegram, Snapchat, Discord, Slack, WhatsApp (+ Shopify em docs = 17).
- **7 redes de ads:** Meta, Google, TikTok, LinkedIn, Pinterest, X, **OpenAI Ads (ChatGPT Ads)** — ousado e inédito.
- **Telephony:** números (54 países), SMS/MMS, voz, WhatsApp Cloud API (zero markup no Meta).

### 3.2 Áreas de API (369 paths; amostra da org README)

| Área | Exemplos | Nota de produto |
|---|---|---|
| Publish | `POST /v1/posts`, bulk-upload CSV, retry, media presign (5GB) | 1 chamada = multi-plataforma; per-platform overrides (first comment, board, título de vídeo) |
| Queue | slots, next-slot, preview | "agendamento por slots" sem escolher timestamp manual — recurso que vende para agências |
| Inbox | conversas, comentários, menções, reviews | DMs/comentários de todas as redes numa forma única |
| Analytics | best-time, content-decay, daily-metrics | Analytics como *feature inclusa*, não add-on |
| Ads | campaigns, creatives, audiences, insights, lead-forms | Ads API unificada em 7 redes |
| WhatsApp | templates, flows, phone numbers, calls | WhatsApp é pilar próprio |
| CRM | contacts, broadcasts, sequences, comment-automations | Auto-resposta a comentários (keyword → DM) |
| Plumbing | hosted OAuth, webhooks c/ delivery logs, API keys, usage metering | White-label multi-tenant: `docs.zernio.com/multi-tenant` |

### 3.3 "Apps construídos sobre a API" (ecossistema MIT open source) — jogada estratégica
- **zernflow** — alternativa open source ao ManyChat (flow builder, inbox, CRM, broadcasts, A/B) → mostra capacidade e compete com SaaS adjacente.
- **latewiz** — UI completa de scheduler (calendar, queue, composer), deploy 1-click → prova de que dá para construir UX por cima.
- **unified-inbox** — inbox único (WhatsApp, IG, Messenger, Telegram, X, Reddit, Bluesky), stateless, key nunca chega ao browser.
- **ads-dashboard** — reporting de ads read-only multi-rede.
- **zernio-shopify** — app Shopify que transforma produtos em posts agendados (templates, bulk, UTM).

**Insight:** cada app é um *demo executável* que (a) prova a capacidade da API, (b) gera tráfego para o ecossistema, (c) ocupa espaço de concorrentes pequenos — tudo MIT.

### 3.4 Diferenciadores técnicos que vendem
- `social-media-api` repo: **drop-in replacement para o SDK da Ayrshare** (concorrente direto) — ataque frontal à base instalada.
- SDKs oficiais: **Node, Python, Go, Ruby, PHP, Rust, .NET, Java** + CLI (`@zernio/cli`) — 8 SDKs + CLI.
- OpenAPI 3.1 pública (`/openapi.yaml`) — "o spec é o contrato".
- Integrações no-code: n8n (community node + alias legado `n8n-nodes-late`), Make, Zapier.
- Nome legado/artefatos: chaves `late_...`, plano `"Pro"` na API → produto evoluiu de uma marca/plano anterior ("Late").

---

## 4. Análise da estrutura de preços

### 4.1 Modelo ATUAL (autoritativo — docs.zernio.com/pricing)

**Filosofia: "Pay for what you use. No plans, no seats, no sales calls at any scale."** Um único plano usage-based, tudo incluso, faturado num cartão, invoices itemizados por conta/número/mensagem/chamada.

| Contas conectadas | Preço por conta/mês |
|---|---|
| 1–2 | **Grátis para sempre** (sem cartão) |
| 3–10 | $6 |
| 11–100 | $3 |
| 101–2.000 | $1 |
| 2.001+ | $1, sem cap (self-serve até qualquer escala; enterprise custom só para 2k+ com descontos) |

**Regras-chave:**
- **Graduado, não flat:** 20 contas = $6×8 + $3×10 = **$78/mês** (as 2 primeiras grátis).
- **"No enterprise wall":** nunca precisa falar com vendas para escalar; enterprise é opcional (contratos custom, SSO SAML/OIDC, SCIM, MFA, RBAC, audit logs, Slack dedicado com engenharia, SOC 2 Type II + GDPR via trust portal).
- **Crédito mensal de $12** cobre as 2 primeiras contas (a "gratuidade" é modelada como crédito na fatura).
- **Proração diária** por conta; calc interativo na página replica a matemática do billing engine.
- **Pass-through X/Twitter a custo zero markup:** reads $0.005/req, user reads/follows $0.010, posts & DM sends $0.015, posts com URL $0.200/req — com **cap mensal de gasto X** configurável (aviso a 80%, pausa a 100%).
- **Mensagens outbound:** primeiras 10.000/mês grátis, depois $0.0001/msg ($1/10k); *por workspace* (não por conta); broadcasts contam por destinatário; começa a valer em 01/10/2026 (anunciado com antecedência).
- **Ads gerenciados:** primeiros 500 grátis, depois $0.01/ads ativo/mês (só running/in-review contam). **"No percentage of ad spend, ever."**
- **Telephony:** números $3–21/mês (54 países), chamadas from $0.01/min (+gravação $0.004/min, transcrição $0.03/min), SMS from $0.008/segmento, 10DLC $9 one-time + campanha from $4/mês, WhatsApp com dois billers (Zernio cobra número/carrier leg; Meta cobra templates direto na WABA).
- **Rate limits API:** free 60 req/min → pago 600 → enterprise 1.200. **Volume de posts ilimitado** por conta; limite é de throughput, não de posts.
- **Cobrança:** cartão, charges automáticas por acúmulo (1º charge a partir de $10 — fraud threshold), invoices itemizados, cancelamento/desconexão a qualquer momento, prorata diário. Reembolso: "usage-based, nada pré-pago; erros de billing ou outage → crédito".
- 30-day money-back anunciado no site (contradição leve com FAQ de reembolso — sinal de página em evolução).

### 4.2 Modelo LEGADO (relatado no briefing; consistente com artefatos residuais)

| Plano | Preço | Cotas |
|---|---|---|
| Free | $0 | ~10 posts/mês |
| Build | $13/mo | ~120 posts/mês |
| Accelerate | $33/mo | posts ilimitados |
| Unlimited | $667/mo | escala máxima |

Resquícios que confirmam a existência do modelo antigo: `planName: "Pro"` e `limits: {uploads: 1000, profiles: 10}` no endpoint `/v1/usage-stats`; "subject to plan limits" no README do n8n; conceito de **social sets / account groups** (1 conta por plataforma por profile; para múltiplas contas da mesma plataforma é preciso um profile por conta — limitação estrutural que funciona como quota). **Confiança:** o briefing do usuário é a fonte dos números; não consegui verificar os valores exatos nas fontes atuais (o README do n8n foi truncado antes da seção de planos), mas a *existência* do modelo legado é confirmada por artefatos de API.

### 4.3 O que COPIAR (padrões de pricing de alto valor)

1. **Free "para sempre" sem cartão nas 2 primeiras contas** — remove atrito de trial e cria produto-vírus (o dev usa, o cliente conecta contas). Gratuidade modelada como crédito ($12/mês) facilita contabilidade.
2. **Graduação de preço por volume ($6→$3→$1)** — ancoragem psicológica: $1/conta parece "barato demais" para quem tem 100+ contas; o custo marginal percebido cai 50% a cada faixa.
3. **"Everything included, nothing gated"** — sem feature-tiering elimina a objeção "o recurso X que preciso está no plano Y". O upsell vira **volume**, não feature.
4. **Pass-through transparente (X API, Meta templates)** — "zero markup" vira argumento de confiança e transfere o custo variável real do fornecedor para quem usa.
5. **Invoices itemizados + página de pricing com calculadora interativa replicando o billing engine** — transparência radical gera confiança em compra B2B de infra.
6. **"No enterprise wall" + self-serve até 10k contas** — o concorrente SaaS tradicional força sales call cedo; isso é uma faca no posicionamento deles.
7. **Sem % sobre ad spend** — diferenciador direto contra agências e dashboards que cobram % (o "ever" é uma palavra-chave).
8. **Anúncio antecipado de mudança de preço (meter de mensagens em 01/10)** — comunica previsibilidade; comunidade valoriza.
9. **Cap de gasto X configurável com aviso a 80%** — produto de billing que previne "surprise bills" (dor real de dev).

### 4.4 O que EVITAR

1. **Modelo legado com cotas de posts e "social sets"** — criar "posts/mês" como quota central estrangula o uso em API-first (o post é o gesto básico); eles mesmos abandonaram.
2. **Tier de $667/mo com "Unlimited"** — preço alto demais para o self-serve e descolado do uso real; criava teto de mercado e empurrava para sales.
3. **"One account per platform per profile"** (limitação estrutural atual) — para agências com 10 contas IG é um incômodo; é um ponto que **podemos atacar** (multi-contas por plataforma num só profile/set).
4. **Complexidade de billing de telephony/WhatsApp de 2 billers** — só copiar se formos entrar em telephony (alto custo operacional; não é o nosso primeiro passo).
5. **Páginas em evolução com contradições** (money-back vs reembolso por uso; 15 vs 16 vs 17 plataformas) — sinal de desordem que podemos evitar com consistência.

---

## 5. Funil / GTM (deduções da observação)

### 5.1 Funil "self-serve + devs"
1. **Aquisição:** SEO por plataforma (páginas `/twitter`, `/instagram`, `/whatsapp`, `/meta-ads`...), conteúdo técnico, **presença GitHub forte** (org verificada, READMEs ricos, MIT), comunidade n8n (community node), app stores (Shopify), afiliados (partners.dub.co + "Refer & earn"), Telegram de anúncios (@zernio_dev), contador social vivo na home.
2. **Ativação:** signup com "Continue with Google" + **2 contas grátis sem cartão** → tempo-para-primeiro-post < 5 min → quickstart com 3 passos (profile → conta → post).
3. **Aprofundamento:** docs com SDKs em 8 linguagens + CLI + curl; **llms.txt + agent-quickstart.md** para agentes; MCP server com OAuth (sem colar chave); multi-tenant guide para builders de produto.
4. **Receita:** usage-based no card; primeira charge em $10+; upgrade natural = conectar mais contas.
5. **Retenção/expansão:** webhooks, status page, changelog, trust portal (SOC 2/GDPR), "Ask AI about the API" no docs (⌘J).
6. **Enterprise (opcional):** SSO/SCIM/audit + Slack dedicado — só para quem pedir.

### 5.2 Estratégia "AI agents first" (o mais sofisticado do GTM)
- **MCP server hospedado** (`mcp.zernio.com/mcp`, Streamable HTTP, OAuth 2.1): 496 tools, 1 endpoint, sem API key no config.
- **Páginas dedicadas por ferramenta de agente:** Claude Code, Cursor, Codex, OpenClaw, Grok bot, Hermes — com comando exato de instalação por cliente.
- **Página /agents é um "instruction set"**: *"Point your agent at it. It fetches this URL, reads the block below, finds the command for the client it is running in, and asks you to approve."* — a própria página é escrita para ser lida por agentes.
- **Docs AI-ready:** `llms.txt` (resumo p/ context window), `llms-full.txt`, OpenAPI 3.1, `agent-quickstart.md`, server-card JSON (`/.well-known/mcp/server-card.json`).
- **CLI com JSON out** — "gives any agent with a shell the same API".
- Plugin oficial do Claude Code (`zernio-claude-plugin`) e do Cursor (`cursor-plugin`).
- **Implicação:** quando um agente precisa publicar nas redes do usuário, a Zernio é a opção que o agente "encontra" sozinho. Eles estão capturando o **canal de distribuição emergente** (agent tooling) antes dos concorrentes clássicos (Buffer/Buffer API, Ayrshare, Postiz...).

### 5.3 Sinais de tração/mercado
- Org GitHub criada **2026-01-23**, com 27 repos públicos e 717 followers em ~7 meses — crescimento orgânico acelerado.
- Claims de confiança: 70k+ devs, 2M+ posts, 99.7% uptime, clientes nominais (Warner Music Group, ClickUp, RE/MAX).
- Contador "accounts connected this week" (ex.: 2.587) — prova social em tempo real na home.

---

## 6. Público-alvo (segmentos e como cada um é atendido)

| Segmento | Como a Zernio os atende |
|---|---|
| **Devs individuais / indie hackers** | Free 2 contas, SDKs, docs 5-min, CLI, curl |
| **AI agents / agent platforms** | MCP, llms.txt, páginas por cliente, CLI JSON, OAuth sem chave |
| **Agencias e multi-tenant** | Profiles por cliente, scoped keys, webhooks por routing, guia multi-tenant, billing centralizado no workspace |
| **Times internos (equipe)** | Invites, API keys por membro, users/roles, analytics, inbox unificado |
| **Produtos SaaS que querem social embutido** | White-label OAuth, presigned upload, drop-in Ayrshare SDK, apps exemplo MIT |
| **Empresas (compliance)** | SOC 2 Type II, GDPR, SSO/SCIM/MFA, audit, enterprise track |

---

## 7. API-first e "limites como estratégia"

- **Volume de posts ilimitado** em todas as contas → o post não é o meter; **a conta conectada** é o meter. Isso alinha incentivo: querem que você conecte muitas contas e automatize (custo deles é infraestrutura, receita é account+msg+ads).
- **Rate limits por plano (60/600/1.200 req/min)** — limite de throughput vira gate de upgrade "invisível" para bots/agentes (um agente faz MUITAS chamadas).
- **Limitações estruturais funcionam como cotas:** 1 conta por plataforma por profile; perfis; account groups. É a versão "social sets" do modelo novo.
- **Contador de uso exposto na API** (`/v1/usage-stats`) — transparência que reduz churn por surpresa e educa o upsell.
- **Cap de gasto X** — limite preventivo que gera confiança (não é restrição, é proteção).

---

## 8. Oportunidades de diferenciação para a NOSSA plataforma

> Premissa: não vamos copiar a Zernio; vamos competir onde ela é fraca ou cara demais.

### 8.1 Onde a Zernio é fraca (ataque direto)
1. **"1 conta por plataforma por profile"** — agências sofrem com múltiplas contas da mesma plataforma. → **Nosso diferencial #1: multi-contas da mesma plataforma por set de contas**, com billing por conta do mesmo jeito. É a dor exata da agência de social media.
2. **Custo X/Twitter pass-through ($0.015-0.20/req)** — caro para quem automatiza X. → Podemos **subsidiar X** no início (perder margem X para ganhar contas), ou **empacotar leitura de X mais barata** via estratégias alternativas.
3. **Sem UI nativa forte** — o dashboard existe, mas a proposta é API-first; a UX do produto final (latewiz é um app separado open source!) é terceirizada. → **UI excelente nativa (PT-BR)** como nosso diferencial de produto completo.
4. **Suporte em inglês, empresa nova nos EUA** → **Suporte PT-BR + LATAM + preços em R$** (moeda local, PIX, boleto) é um oceano azul: nenhum player desse tipo fala português de verdade.
5. **Billing de telephony complexo** — não precisamos copiar; **evite** telephony no v1.
6. **Sem planos com desconto anual / sem foco SMB** — modelo deles é self-serve tech; podemos fazer **planos para agências SMB** (assinatura anual, suporte humano, onboarding).
7. **Contradições/desordem de site** (15 vs 16 vs 17 plataformas; money-back vs refund) → consistência de comunicação como vantagem.
8. **Sem módulo de "conteúdo/IA para criar posts"** — eles são pura infraestrutura; podemos oferecer **editor + sugestão por IA + calendário editorial** como camada de valor (eles competem com Ayrshare/Buffer; nós competimos com "infra + produto").

### 8.2 O que manter igual (padrões vencedores)
- API REST simples + OpenAPI pública + SDKs (começar com Node + Python + Go; depois o resto).
- Free para sempre sem cartão (2 contas) — mas podemos ir além: **free com 3 contas + 1 set de agência** para capturar o "agente que testa".
- Usage-based por conta com gradação — mas com **teto de preço por conta menor no tier de entrada** (ex.: $4/contas 3-10) para undercut.
- Tudo incluso (sem feature tiers), webhooks, health check de contas, queue por slots, analytics.
- MCP server + llms.txt + agent-quickstart — **barato de copiar (é só documentação + servidor MCP) e essencial** para o canal de agentes.
- Drop-in SDK do concorrente (como eles fizeram com a Ayrshare) — podemos publicar um **adapter drop-in para a própria Zernio** ("migre da Zernio sem reescrever código") — jogada ofensiva legítima de mercado.

### 8.3 Diferenciais de nicho que a Zernio não cobre
- **LATAM/BR primeiro**: docs em PT-BR, suporte humano em PT-BR, preços em R$/BRL, PIX/boleto, LGPD.
- **Agências de influência/micro-agências**: multi-tenant com revenda (sub-orgs, billing por cliente, white-label com marca do cliente).
- **Verticalização**: nichos com casos prontos — e-commerce (Shopify/Mercado Livre + posts), restaurantes/horários, imobiliárias (GMB + posts).
- **Conformidade/confiança local**: SOC 2 é caro; mas LGPD + transparência de preço + status page já dão confiança inicial.

---

## 9. Recomendação: modelo de preços inicial para NÓS

### 9.1 Proposta: **Freemium estrutural (free para sempre) + usage-based por conta, com 2 camadas de produto: Dev (self-serve) e Agência (assinatura)** — mas com uma diferença tática vs Zernio.

| Faixa de contas | Preço por conta/mês (USD) | Preço em BRL sugerido |
|---|---|---|
| 1–3 | **Grátis para sempre** (sem cartão) | Grátis |
| 4–10 | **$4** (undercut: $6 deles) | ~R$ 24/mês |
| 11–100 | $2 | ~R$ 12/mês |
| 101+ | $1 | ~R$ 6/mês |
| Agência (assinatura) | **$49/mês base** = até 10 contas + 3 sets + 5 usuários + white-label básico + suporte PT-BR prioritário | ~R$ 290/mês |

**Aditivos metered (copiar o padrão "primeiro X grátis"):**
- Mensagens outbound: 10k/mês grátis, depois $1/10k (igual — padrão de mercado).
- Ads: primeiro 100 grátis, $0.01/ads ativo (mais generoso no início p/ atrair).
- X/Twitter: **subsidiado no free** (grátis 300 req/mês; depois pass-through) — ataque direto ao custo deles.

### 9.2 Justificativa
1. **Freemium com free "para sempre"** é o padrão-ouro de API-first (validado pela própria Zernio e por Twilio/Stripe): o custo marginal de 2-3 contas é baixo, e o dev/agente que adota vira distribuidor (e o agente de IA recomenda a ferramenta que ele "sabe" usar — vencedor é quem estiver nos MCP/llms.txt).
2. **Undercut de $6→$4 no tier de entrada** ataca exatamente o preço ancorado da Zernio (preço-âncora do concorrente é o nosso melhor material de marketing: "mesma coisa por 33% menos, em português").
3. **Camada Agência separada** captura o segmento que a Zernio atende mal (multi-contas por plataforma, UI, suporte humano) — é onde está o dinheiro recorrente e o churn mais baixo.
4. **Sem cotas de posts** (lição do legado): meter = contas + mensagens + ads; post ilimitado sempre.
5. **Meter X subsidiado no free** é o cavalo de Troia: o custo de X é a maior dor de quem automatiza social; se o free absorve isso, a migração da Zernio (que repassa 100%) é óbvia.
6. **Billing PT-BR (PIX/boleto, R$)** remove a barreira de cartão internacional — sozinho já diferencia no mercado brasileiro.
7. **Rate limits 60/600/1200** (copiar) + **cap de gasto X** (copiar) + invoices itemizados (copiar) — confiança de infra.

### 9.3 Métricas de validação (North Stars iniciais)
- Ativação: tempo até 1º post < 5 min; % signup→1 conta conectada.
- Conversão: % que passa de 3 contas (free→pago) e tempo médio para isso.
- Distribuição por agente: nº de setups via MCP/llms.txt e nº de "agent-driven" signups.
- Agência: nº de sets ativos, contas por agência, churn mensal.

---

## 10. Top 10 lições para a nossa plataforma

1. **Venda infraestrutura, não ferramenta**: "uma API para 16 canais" vence "um app de agendamento". Posicione como camada de redução de complexidade integradora.
2. **Freemium sem cartão e "para sempre"** é o motor de adoção API-first (2 contas grátis; gratuidade como crédito contábil).
3. **Usage-based por conta conectada, graduado ($6→$3→$1), tudo incluso, sem feature tiers** — o upsell é volume, não feature; posts ilimitados sempre (cota de post foi o erro do modelo legado).
4. **"No enterprise wall"** — self-serve até qualquer escala + enterprise opcional só para compliance/volume. Não force sales call.
5. **AI agents são um canal de distribuição real**: MCP server, llms.txt, página /agents como instruction set, CLI com JSON out, páginas por cliente (Claude/Cursor/Codex). Barato de implementar, caro de ignorar.
6. **Ecossistema open source ao redor da API fechada**: apps de exemplo MIT (scheduler, inbox, dashboards) provam capacidade, geram tráfego e ocupam espaço de concorrentes.
7. **Drop-in adapter para a Zernio** (como eles fizeram com a Ayrshare) — ataque frontal à base instalada com custo de migração ~zero.
8. **Transparência radical de billing**: invoices itemizados, calculadora de preço pública, pass-through a zero markup, cap de gasto com aviso — confiança é o moeda em infra B2B.
9. **Limites estruturais funcionam como quotas**: rate limit por throughput (60/600/1200), 1 conta por plataforma por profile (para nós: desenhar melhor, multi-contas por set) e usage-stats exposto na API.
10. **Nicho/LATAM é o oceano azul**: suporte e docs em PT-BR, preços em R$ com PIX/boleto, foco em agências SMB — a Zernio não fala com esse mercado.

---

## 11. Anexos

### 11.1 Fontes coletadas (2026-09-01)
| Fonte | Status | Uso |
|---|---|---|
| zernio.com (home) via r.jina.ai | ✓ renderizado | proposta de valor, nav, social proof |
| zernio.com/pricing via r.jina.ai | ✓ renderizado | rate card completo, FAQ |
| docs.zernio.com/pricing via r.jina.ai | ✓ | pricing autoritativo, math de billing |
| docs.zernio.com (quickstart) | ✓ | funil de ativação, plataformas |
| zernio.com/agents via r.jina.ai | ✓ | estratégia AI agents, MCP, llms.txt |
| zernio.com/llms.txt | ✓ | resumo oficial API, rate limits, conceitos |
| raw.githubusercontent.com/zernio-dev/n8n-nodes-zernio/master/README.md | ✓ (truncado em 8k chars) | superfície de API, operações n8n |
| raw.githubusercontent.com/zernio-dev/.github/main/profile/README.md | ✓ completo | SDKs, apps do ecossistema, repos |
| api.github.com/orgs/zernio-dev (+ repos) | ✓ (truncado) | metadados org (717 followers, 27 repos, verificada) |
| github.com/zernio-dev via r.jina.ai | ✓ | org README renderizado |
| web.archive.org (wayback) | ✓ disponível (2026-08-28) | confirma pricing atual recente |

### 11.2 Limitações e avisos epistêmicos
- **Modelo legado (Free/Build/Accelerate/Unlimited) NÃO verificado nas fontes atuais** — veio do briefing e de artefatos residuais (`planName: "Pro"`, `limits: {uploads: 1000, profiles: 10}`, "subject to plan limits", alias `n8n-nodes-late` + prefixo de chave `late_`). Tratar como histórico, não como oferta atual.
- Claims de tração (70k devs, 2M posts, clientes) são **autodeclarados** — não auditados.
- O README do n8n foi truncado (limite do fetcher) — seções finais (possíveis detalhes de planos legados) não capturadas.
- Preços capturados em 2026-09-01; a Zernio anunciou meter de mensagens a partir de 01/10/2026 — modelo em movimento.

### 11.3 Repos da org (parcial — listagem truncada na API, complementada pela org README)
n8n-nodes-zernio · zernio-python · zernio-node · zernio-go · zernio-ruby · zernio-php · zernio-rust · zernio-dotnet · zernio-java · zernio-cli · zernio-claude-plugin · cursor-plugin · chat-sdk-adapter · zernflow · latewiz · unified-inbox · ads-dashboard · zernio-shopify · openapi-specs · zernio-api (Claude skill) · social-media-api (drop-in Ayrshare) · crisp-mcp · .github — ~27 públicos no total.

---
*Fim do relatório. Próximos passos sugeridos: (1) validar o modelo de pricing proposto com 5-10 agências PT-BR; (2) priorizar construção: API REST + OpenAPI + SDK Node/Python + MCP + llms.txt antes de UI; (3) monitorar changelog/preços da Zernio mensalmente.*
