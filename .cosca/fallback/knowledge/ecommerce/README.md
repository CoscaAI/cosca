# 16 — E-COMMERCE / COMMERCE ENGINEERING INTELLIGENCE

> Stack 16 da Cosca Engineering Intelligence Matrix.
> Um e-commerce NÃO é vitrine + carrinho + pagamento. É um **sistema distribuído de comércio**.

## MISSÃO
Projetar, auditar e evoluir sistemas de comércio digital — sem copiar Medusa/Saleor/Vendure/Shopify. Construir inteligência própria de commerce engineering.

## VISÃO DE DOMÍNIO (a regra de ouro)

| Conceito | É ... (não é) |
|---|---|
| PRODUCT | um **domínio** (não uma tabela) |
| PRICE | uma **regra** (não um campo) |
| INVENTORY | **estado concorrente** (nunca `product.stock = 10`) |
| CART | **estado temporário** com price snapshot |
| CHECKOUT | um **workflow** (máquina de estados) |
| PAYMENT | **transação externa** (tokenização, provedor) |
| ORDER | **agregado operacional** (idempotente) |
| FULFILLMENT | um **processo** (picking/packing/dispatch) |
| SHIPPING | uma **integração** (carrier/zone/rate) |
| RETURN | **workflow reverso** |
| CUSTOMER | um **relacionamento** (não só uma tabela) |
| PROMOTION | um **motor de regras** (conditions→rules→actions→limits) |
| SEARCH | um **índice derivado** (eventual consistency) |
| ANALYTICS | **observabilidade do negócio** (funil) |

## MODELO DE CATÁLOGO
`Product → Variants → Options → Attributes → Media → Categories → Collections`
- Variant = combinação de options (tamanho×cor) — NÃO produto independente
- SKU/EAN/GTIN/MPN por variant; price/cost/inventory por variant

## PRINCÍPIOS CORE
1. **Idempotência OBRIGATÓRIA** em payment, order creation, webhooks, refund, inventory reservation · UNIVERSAL
2. **Inventário = estado concorrente**: on_hand / available / reserved / committed / in_transit — com reservas e atualização atômica · UNIVERSAL
3. **Price snapshot + revalidação**: o carrinho NÃO lê o preço atual cegamente; revalida no checkout · STRONG
4. **Checkout = máquina de estados**: CART→CUSTOMER→ADDRESS→SHIPPING→PAYMENT→ORDER, com retry/expiração/concorrência · UNIVERSAL
5. **Webhook = input NÃO confiável**: assinatura, idempotência, replay, retry, DLQ · UNIVERSAL
6. **Consistência classificada**: payment/order = strong; search/analytics = eventual — justificar cada escolha · STRONG
7. **Event-driven só com benefício real** — não transforme cada evento em microservice · STRONG
8. **Bulk/import/export NUNCA bloqueia HTTP** — `JOB → QUEUE → WORKER → PROGRESS → RESULT` · UNIVERSAL
9. **Back office é produto próprio** (merchant experience), não CRUD genérico · STRONG
10. **Multi-vendor ≠ multi-tenant** — marketplace tem seller/payout/commission/split order · UNIVERSAL

## REGRAS DE DECISÃO
- **Arquitetura**: monólito modular é default; headless quando multi-canal (web/mobile/POS/marketplace); composable exige justificativa (complexidade operacional, latência, boundaries, custo); microservices só com `DOMAIN → TEAM → SCALE → COMPLEXITY → OPERATIONS → COST` justificando.
- **Search**: Postgres FTS até justificar Meilisearch/Typesense/Elasticsearch/Algolia. Não introduza infra sem razão.
- **Tax**: nunca `price * taxRate` cego — jurisdição/região/categoria; provedor especializado quando complexo.
- **Guest checkout** é default; conta obrigatória só quando o domínio exige (B2B).
- **Payment**: direto vs serviço vs orquestrador — pela matriz de risco/mercado; nunca armazenar PAN sem necessidade.

## ANTI-PATTERNS
`product.stock = 10` · `cart lendo preço ao vivo` · `POST /orders sem idempotência (3 cliques = 3 pedidos)` · `checkout acoplado ao provedor de email` · `webhook sem assinatura` · `cache everything sem invalidação` · `import de 100k produtos bloqueando HTTP` · `dark patterns (falsa urgência, opt-out escondido)` · `index explosion de SEO (URLs duplicadas)` · `estoque reservado sem expiração` · `refund sem workflow reverso` · `admin tratado como CRUD genérico`

## MODELO DE MATURIDADE (auditar o projeto)
`0 Prototype → 1 Basic Store → 2 Production → 3 Advanced → 4 Enterprise → 5 Composable/Omnichannel`

## REVIEW ENGINE (22 fases)
`BUSINESS → DOMAIN → ARCHITECTURE → DATA MODEL → API → CATALOG → PRICING → INVENTORY → CART → CHECKOUT → PAYMENT → ORDER → SHIPPING → RETURNS → SECURITY → PERFORMANCE → OBSERVABILITY → UX → SEO → MOBILE → OPERATIONS → RECOMMEND`

## DECISION ENGINE
Para cada recomendação: `PROBLEM → CURRENT STATE → OPTIONS → TRADE-OFFS → RECOMMENDATION → RISK → BENEFIT → VALIDATION`. Nunca "use X" — sempre "use X porque...".

## CHECKLIST (quality gate)
- [ ] Idempotência em POSTs de comércio (order/payment/webhook/refund/reservation)
- [ ] Inventory com estados + reservas + concorrência (último item)
- [ ] Price snapshot + revalidação no checkout
- [ ] Checkout como state machine com recovery (payment failure, retry, expiration)
- [ ] Webhooks com assinatura + replay + DLQ
- [ ] Bulk ops via job/queue (nunca HTTP bloqueante)
- [ ] Consistência classificada (strong vs eventual)
- [ ] Fraude defensiva (risk scoring, rate limits, safe defaults — sem vigilância invasiva)
- [ ] Segredos protegidos; PAN nunca armazenado
- [ ] SEO (canonical, structured data, sitemap, sem index explosion)
- [ ] Observabilidade: checkout_latency, payment_failure_rate, cart_abandonment
- [ ] Performance sob pico (flash sale/campaign) modelada
- [ ] DR testado

## A REGRA
Produto é domínio. Preço é regra. Estoque é estado concorrente. Carrinho é estado temporário. Checkout é workflow. Pagamento é transação externa. Pedido é agregado operacional. Fulfillment é processo. Envio é integração. Devolução é workflow reverso. Cliente é relacionamento. Promoção é motor de regras. Busca é índice derivado. Análise é observabilidade do negócio. **E-commerce é um sistema completo.**

## REFERÊNCIAS
Medusa · Vendure · Saleor · Spree · Sylius · Shopware · WooCommerce · Stripe/PayPal/Adyen · Meilisearch/Typesense/Elasticsearch · Temporal/NATS/Kafka/RabbitMQ · OpenTelemetry/PostHog/Sentry · pgvector
