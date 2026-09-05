# 11 — VERCEL INTELLIGENCE

> Stack 11 da Cosca Engineering Intelligence Matrix.
> Doutrina de deploy da casa: git é a fonte da verdade, preview por push, rollback instantâneo.

## MISSÃO
Deployar frontends (e serverless) com Vercel dominando o modelo git-driven: production branch + preview por push, env vars por environment, framework preset e rollback instantâneo — com segurança sobre quem tem poder de deploy.

## PRINCÍPIOS CORE
1. **Git é a fonte da verdade** — push em preview branches gera previews; merge na production branch faz deploy de produção · UNIVERSAL
2. **URL única por preview** — cada push gera uma URL de preview própria, testável antes de merge · UNIVERSAL
3. **Env vars por environment** — production/preview/development têm conjuntos de variáveis independentes · UNIVERSAL
4. **Framework preset** — o build é detectado automaticamente (Next.js, etc.) sem config manual · STRONG
5. **Rollback instantâneo** — qualquer deploy anterior pode ser restaurado em um clique; a plataforma é imutável e versionada · STRONG

## REGRAS DE DECISÃO
- `vercel.json` para ajustes (rewrites, headers, region, cron, limits) quando o preset não cobre.
- Monorepo: `rootDirectory` aponta para o app a ser deployado; o resto fica fora do build.
- Segurança: nunca fazer deploy de commit de autor não autorizado — revisar quem tem permissão de escrita no branch de produção.
- CI/CD: integrar ao pipeline da casa (COMMIT → TEST → BUILD → DEPLOY → VERIFY → ROLLBACK do devops/README).
- Preview é o ambiente de QA real: testar a URL antes de abrir o merge request.
- Env vars sensíveis nunca no repo; configurar por environment no dashboard/CLI.

## ANTI-PATTERNS
`deploy manual de bundle local` · `produção deployada por push de branch não-principal` · `env vars de produção usadas em preview` · `commit de autor não autorizado indo a produção sem gate` · `rollback feito no código em vez de na plataforma (lento)` · `ignorar o framework preset e configurar build na mão` · `monorepo sem rootDirectory (build do app errado)`

## CHECKLIST
- [ ] Push gera preview com URL testável
- [ ] Merge na production branch faz deploy automático
- [ ] Env vars separadas por environment; nenhum segredo no repo
- [ ] Framework preset detectado (ou rootDirectory correto no monorepo)
- [ ] `vercel.json` documentado quando usado
- [ ] Rollback testado em staging antes de depender dele em prod
- [ ] Gate de autorização sobre o branch de produção

## A REGRA
Na Vercel, o deploy é consequência do git: quem controla o branch de produção controla o que vai ao ar — e o rollback é uma decisão de plataforma, não uma re-escrita de código.

## REFERÊNCIAS
Vercel docs (vercel.com/docs) · vercel.json reference · Vercel CLI · devops/README.md · PADRAO-COSCA §6

## CONHECIMENTO (schema _SCHEMA.md)

```
ID: devops-001
DOMAIN: devops
TITLE: Git como fonte da verdade do deploy
PROBLEM: Deploys manuais divergem do código; ambiente se torna "snowflake" sem rastreio
CONTEXT: Vercel, frontend/serverless
PRINCIPLE: O estado do deploy deriva do git: branch de produção = produção; push = preview
RECOMMENDATION: Adotar merge-to-deploy; proibir deploy manual de bundle local
WHEN_TO_USE: Qualquer app frontend/serverless
WHEN_NOT_TO_USE: Deploy de artefato binário/legado sem pipeline git
TRADE_OFFS: Amarração ao fluxo git vs rastreabilidade e consistência total
EXAMPLE: Merge de `main` em produção sobe o app; o commit é o registro do deploy
COUNTER_EXAMPLE: `vercel deploy --prod` com bundle gerado localmente sem passar por review
FAILURE_MODES: Commit ruim mergeado direto — precisa de gate de testes antes do merge
REFERENCES: Vercel git docs
CONFIDENCE: UNIVERSAL
SOURCE: Vercel + devops/README.md
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: devops-002
DOMAIN: devops
TITLE: Preview por push com URL única
PROBLEM: QA/teste de mudança exige ambiente isolado e reproduzível
CONTEXT: Pull requests em projetos Vercel
PRINCIPLE: Cada push gera um ambiente efêmero com URL própria — testável antes de merge
RECOMMENDATION: Usar a URL de preview como porta de QA em todo PR; nunca aprovar sem ela
WHEN_TO_USE: Todo PR com mudança de frontend
WHEN_NOT_TO_USE: Mudança puramente de backend/libs sem impacto na página
TRADE_OFFS: Um deploy por push vs ambientes de teste sempre frescos
EXAMPLE: Push `feat/x` gera `feat-x-abc123.vercel.app` testável
COUNTER_EXAMPLE: Testar a mudança só após o merge (prod já está "quebrada")
FAILURE_MODES: Preview divergindo de prod por env vars diferentes — manter environments sincronizados
REFERENCES: Vercel preview deployments docs
CONFIDENCE: UNIVERSAL
SOURCE: Vercel + devops/README.md
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: devops-003
DOMAIN: devops
TITLE: Env vars por environment (production/preview/development)
PROBLEM: Segredo de produção vazado para preview/desenvolvimento; config misturada
CONTEXT: Vercel, múltiplos ambientes
PRINCIPLE: Environment define o conjunto de variáveis; o mesmo código, dados diferentes
RECOMMENDATION: Configurar env vars por environment; secrets nunca no repo nem em preview
WHEN_TO_USE: Qualquer app com dados/credenciais distintos por ambiente
WHEN_NOT_TO_USE: App stateless sem variáveis sensíveis
TRADE_OFFS: Três conjuntos para gerenciar vs isolamento real de segredo
EXAMPLE: `DATABASE_URL` aponta para prod em production, para staging em preview
COUNTER_EXAMPLE: `VERCEL_ENV` usado no código para vazar segredo de prod em preview
FAILURE_MODES: Preview quebrando por env ausente — documentar o conjunto completo
REFERENCES: Vercel environment variables docs
CONFIDENCE: UNIVERSAL
SOURCE: Vercel + security/README.md
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: devops-004
DOMAIN: devops
TITLE: Rollback instantâneo na plataforma
PROBLEM: Reverter produção via código é lento e arriscado no momento do incidente
CONTEXT: Vercel, deploys imutáveis
PRINCIPLE: Deploys são imutáveis e versionados; reverter é escolher o deploy anterior, não re-escrever
RECOMMENDATION: Usar o rollback de plataforma (último deploy bom) e então corrigir o código com calma
WHEN_TO_USE: Incidente em produção
WHEN_NOT_TO_USE: Correção que pode ir direto para o mesmo commit (ainda assim rollback é mais rápido)
TRADE_OFFS: Instantaneidade vs voltar funcionalidade nova legítima que veio no meio
EXAMPLE: Deploy ruim detectado → rollback para o deploy anterior em segundos
COUNTER_EXAMPLE: Criar hotfix e re-deployar (minutos) enquanto o site está fora
FAILURE_MODES: Rollback sem testes de que o "bom" antigo funciona com o banco atual
REFERENCES: Vercel rollback docs
CONFIDENCE: STRONG
SOURCE: Vercel + devops/README.md
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```
