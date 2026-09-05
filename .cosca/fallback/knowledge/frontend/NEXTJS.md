# 02 — NEXT.JS APPS ROUTER INTELLIGENCE

> Stack 02 da Cosca Engineering Intelligence Matrix.
> Doutrina do framework React canônico da casa: App Router, file-conventions e Server Components.

## MISSÃO
Construir aplicações React com Next.js dominando o sistema de file-conventions como API declarativa — rotas, layouts, boundaries de carregamento/erro — e os Server Components como default de arquitetura.

## PRINCÍPIOS CORE
1. **File-conventions são a API** — `layout`/`page`/`loading`/`error`/`route`/`not-found`/`template` declaram comportamento; não há config central · UNIVERSAL
2. **URL = pastas + arquivos especiais** — a pasta só vira rota quando contém `page`/`route` (colocation) · UNIVERSAL
3. **Server Components por padrão** — `"use client"` é a exceção, reservada a interatividade real · UNIVERSAL
4. **Boundaries aninhadas** — hierarquia `layout → template → error → loading → not-found → page` isola falha e carregamento por nível · STRONG
5. **Organização é livre** — o framework não é opinionado sobre estrutura; as convenções são declarativas e composicionais · OPINION

## REGRAS DE DECISÃO
- Rotas dinâmicas: `[slug]` (uma posição), `[...slug]` (catch-all), `[[...slug]]` (opcional).
- Route groups `(group)`: agrupam sem mudar a URL; cada grupo permite `layout` próprio.
- Pasta privada `_folder`: fora do roteamento; acessível apenas via import.
- Parallel routes `@slot` renderizam múltiplas views na mesma rota; intercepting `(.)` sobrepõe rotas (modais/detail com URL mantida).
- `loading.tsx` = skeleton por segmento; `error.tsx`/`global-error.tsx` = boundaries de recuperação com recovery.
- `"use client"` no menor componente interativo possível; nunca em página inteira por preguiça.
- API via `route.ts` (Route Handlers) quando o objetivo é endpoint, não página.

## ANTI-PATTERNS
`"use client" em tudo` · `página inteira client quando só um botão interage` · `fetch em useEffect em conteúdo server-renderable` · `componente solto na pasta de rotas virando rota fantasma` · `erro sem boundary (crash total da tela)` · `estrutura de pastas ditada por terceiros em vez de convenção` · `duplicar layout em vez de usar route group`

## CHECKLIST
- [ ] Página navegável tem `page`; API tem `route`
- [ ] `loading`/`error`/`not-found` presentes nos segmentos críticos
- [ ] `"use client"` só onde há interatividade real
- [ ] Route groups usados para layouts próprios sem poluir a URL
- [ ] Pasta privada `_` para módulos internos de um segmento
- [ ] Dados buscados no servidor (RSC/Server Actions), não no client

## A REGRA
No Next.js App Router, URL e arquitetura são declaradas por convenções de arquivos, não por config — o framework dá a forma, o capo preenche com componentes e dados.

## REFERÊNCIAS
Next.js App Router (vercel/next.js) · React Server Components · React 19 · Vercel Agent Skills (nextjs-optimizations, composition-patterns) · PADRAO-COSCA §6

## CONHECIMENTO (schema _SCHEMA.md)

```
ID: frontend-001
DOMAIN: frontend
TITLE: File-conventions como API declarativa
PROBLEM: Roteamento, layouts, loading e erro precisam ser declarados sem config central que se degrada
CONTEXT: Qualquer app Next.js App Router
PRINCIPLE: A pasta é o roteador; arquivos especiais (layout/page/loading/error/route/not-found/template) são as instruções
RECOMMENDATION: Declarar comportamento com file-conventions; não duplicar em config
WHEN_TO_USE: Todo projeto Next.js App Router
WHEN_NOT_TO_USE: App simples que não usa React/Next (Vite)
TRADE_OFFS: Convenções implícitas e fortes vs legibilidade para quem não conhece o framework
EXAMPLE: `app/blog/[slug]/page.tsx` vira rota `/blog/:slug` sem roteador manual
COUNTER_EXAMPLE: Framework sem convenções (React puro + react-router manual) onde a estrutura é escolha 100% sua
FAILURE_MODES: Arquivo no lugar errado criando rota fantasma; esquecer `loading`/`error` e crash total
REFERENCES: Next.js App Router docs
CONFIDENCE: UNIVERSAL
SOURCE: vercel/next.js + PADRAO-COSCA §6
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: frontend-002
DOMAIN: frontend
TITLE: Server Components por padrão, "use client" como exceção
PROBLEM: Todo componente client desnecessário infla o JS no browser e move trabalho do servidor para o usuário
CONTEXT: RSC no Next.js App Router
PRINCIPLE: Server Components são o default; client é reservado a interatividade, estado e efeitos
RECOMMENDATION: Marcar `"use client"` apenas no menor componente que precisa de interatividade
WHEN_TO_USE: Qualquer conteúdo renderizável no servidor
WHEN_NOT_TO_USE: Componente com estado/efeito/handlers — precisa ser client
TRADE_OFFS: Menos JS no client + menos ida ao servidor vs limite de onde o server boundary termina
EXAMPLE: Página de produto server; botão "adicionar" isolado como client
COUNTER_EXAMPLE: SPA de dashboard real-time onde quase tudo é client legítimo
FAILURE_MODES: `"use client"` vazando para a árvore inteira (todo filho vira client)
REFERENCES: React Server Components docs
CONFIDENCE: UNIVERSAL
SOURCE: vercel/next.js + PADRAO-COSCA §6
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: frontend-003
DOMAIN: frontend
TITLE: Hierarquia de boundaries aninhadas
PROBLEM: Falha ou carregamento em um segmento derruba a experiência inteira
CONTEXT: App Router com layouts aninhados
PRINCIPLE: Cada nível da árvore pode ter sua própria boundary (layout → template → error → loading → not-found → page)
RECOMMENDATION: Cobrir segmentos críticos com `loading.tsx` (skeleton) e `error.tsx`/`global-error.tsx` (recovery)
WHEN_TO_USE: Qualquer rota com dados async ou alto valor de resiliência
WHEN_NOT_TO_USE: Página estática trivial sem estado de falha
TRADE_OFFS: Mais arquivos/boundaries vs isolamento granular de falha
EXAMPLE: `app/dashboard/loading.tsx` mantém header renderizado enquanto dados carregam
COUNTER_EXAMPLE: Aplicação que usa `try/catch` e estado global para simular boundaries
FAILURE_MODES: Error boundary sem recovery (usuário preso na tela de erro)
REFERENCES: Next.js error handling docs
CONFIDENCE: STRONG
SOURCE: vercel/next.js
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: frontend-004
DOMAIN: frontend
TITLE: Route groups + colocation para organizar sem poluir a URL
PROBLEM: Organizar rotas com layouts próprios sem que pastas de organização vazem para a URL
CONTEXT: App Router com muitos segmentos
PRINCIPLE: Route group `(group)` agrupa sem mudar URL e permite layout próprio; pasta sem `page`/`route` não vira rota
RECOMMENDATION: Usar `(group)` para layouts alternativos e colocation para manter módulos junto da rota
WHEN_TO_USE: Apps com áreas distintas (auth vs app), componentes co-localizados
WHEN_NOT_TO_USE: Estruturas onde a URL já reflete a organização desejada
TRADE_OFFS: Organização limpa vs dois caminhos para a mesma coisa (grupo vs pasta privada)
EXAMPLE: `app/(marketing)` com layout próprio e `app/(app)` com sidebar, sem `/marketing` na URL
COUNTER_EXAMPLE: Colocar um componente dentro da pasta de rotas sem `page` e esperar que seja ignorado (é colocation correto; vira rota só com `page`/`route`)
FAILURE_MODES: Route group vira rota se você colocar `page.tsx` direto no grupo sem filho
REFERENCES: Next.js route groups docs
CONFIDENCE: STRONG
SOURCE: vercel/next.js + PADRAO-COSCA §6
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: frontend-005
DOMAIN: frontend
TITLE: Rotas dinâmicas e pastas privadas
PROBLEM: Necessidade de URLs dinâmicas e de esconder módulos internos do roteador
CONTEXT: Conteúdo por slug, módulos de implementação por segmento
PRINCIPLE: `[slug]`/`[...slug]`/`[[...slug]]` expressam graus de dinamismo; `_folder` esconde do roteamento
RECOMMENDATION: Escolher o tipo de rota dinâmica pelo cardinalidade do parâmetro; `_` para internos
WHEN_TO_USE: Blog/produto por slug, módulos compartilhados por segmento
WHEN_NOT_TO_USE: Rotas estáticas conhecidas (não usar catch-all por preguiça)
TRADE_OFFS: Flexibilidade de catch-all vs perda de tipo/estrutura explícita
EXAMPLE: `app/blog/[...slug]/page.tsx` para caminhos aninhados; `app/_components/` para internos
COUNTER_EXAMPLE: Catch-all quando só um nível existe (diminui segurança de tipagem)
FAILURE_MODES: Catch-all capturando rotas que deveriam ter handler específico
REFERENCES: Next.js dynamic routes docs
CONFIDENCE: UNIVERSAL
SOURCE: vercel/next.js
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```
