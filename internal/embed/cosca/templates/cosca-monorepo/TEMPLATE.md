# COSCA MONOREPO TEMPLATE — Blueprint-mãe do Padrão COSCA

> **Nível:** Template | **Origem:** PADRAO-COSCA.md (a doutrina) + stack intelligence matrix
> **Propósito:** monorepo canônico da casa para projetos fullstack completos — web + api + studio + brand, tokens únicos, tudo conversa.
> **Doutrinas que referenciam:** `knowledge/PADRAO-COSCA.md` · `knowledge/frontend/NEXTJS.md` · `TAILWIND.md` · `SHADCN.md` · `GSAP.md` · `knowledge/backend/NESTJS.md` · `knowledge/visual-media/REMOTION.md` · `knowledge/devops/VERCEL.md` · `knowledge/database/README.md`

## Domain
Monorepo fullstack no padrão COSCA: um único repositório (turborepo/pnpm workspaces) orquestrando apps que compartilham identidade visual e lógica — frontend, backend e produção de vídeo programática. Próprio para projetos que precisam de mais de uma superfície (site + API + conteúdo em vídeo) com uma fonte única de design tokens.

**Quando usar:**
- Projeto completo: frontend público/autenticado + backend próprio + conteúdo em mídia (vídeo/imagem programática).
- Vários apps que precisam compartilhar identidade visual (mesma cor, tipografia, tokens).
- Backend NestJS separado do front (borda de segurança, escala, tipos de workload distintos).

**Quando NÃO usar (use o template simples):**
- Projeto pequeno com uma superfície só — um `web` estático sem backend: template `landing`.
- API pura sem frontend: template `api`.
- Backend + frontend mínimos em um app único (API routes do Next cobrem): template `saas`/`landing`, não o monorepo completo.
- CLI, SDK, mobile standalone: templates `cli`, `sdk`, `mobile`.

## Recommended Stack
- **Web** (`apps/web/`): Next.js App Router + Tailwind v4 (`@theme`) + shadcn/ui — Server Components por padrão, `"use client"` só onde interage; file-conventions (`layout`/`page`/`loading`/`error`/`route`) como API declarativa.
- **API** (`apps/api/`): NestJS — módulos por feature (`users/`, `orders/`, `payments/`), DI por constructor, `ValidationPipe` global fail-closed, Guards/Pipes/Interceptors/Filters no papel certo.
- **Studio** (`apps/studio/`): Remotion — vídeo como função pura do frame (`useCurrentFrame` + `Composition`/`Sequence`), `spring()`/`interpolate()`, determinístico; GSAP para animação in-page (Tween seta, Timeline sequencia, `useGSAP()` + cleanup).
- **Tokens**: `packages/brand/theme.css` — design tokens ÚNICOS em `@theme` (Tailwind v4), fonte da verdade consumida por TODOS os apps via `@import`.
- **Dados**: pela árvore de decisão (`knowledge/database/README.md`) — SQLite (edge/single-file/local-first, padrão atual) → Postgres (canônico/ACID: dinheiro, ledger, conhecimento; `money = numeric`) → Mongo (schema volátil por documento; consistência multi-doc sobe pra Postgres/JSONB) → Redis (cache/fila/ranking — SEMPRE derivado do canônico, nunca fonte da verdade). Migrations forward-only + teste de banco em container.
- **Deploy**: web no Vercel (`vercel.json` com `rootDirectory` apontando pra `apps/web`; git é a fonte da verdade, preview por push); api em Docker (build+test+deploy no CI).

## Module Structure
```
projeto/
├── apps/
│   ├── web/                  # Next.js App Router + Tailwind v4 (@theme) + shadcn/ui
│   │   ├── app/              # file-conventions (layout/page/loading/error/route)
│   │   ├── components/ui/    # shadcn (copy-paste, Radix + cva — código é SEU)
│   │   └── styles/globals.css# @import "tailwindcss" + @import do brand/theme.css
│   ├── api/                  # NestJS (monólito modular)
│   │   ├── src/modules/      # feature modules (users/, orders/, payments/)
│   │   ├── src/common/       # guards, decorators, filters, interceptors, pipes
│   │   └── prisma/           # schema + migrations (forward-only)
│   └── studio/               # Remotion (vídeo programático) + GSAP (animação in-page)
│       ├── src/compositions/ # <Composition> + <Sequence> + schema Zod
│       └── public/           # assets via staticFile()
├── packages/
│   └── brand/
│       └── theme.css         # design tokens ÚNICOS (@theme) — fonte da verdade
├── docs/
│   ├── DESIGN.md             # fonte de verdade visual do projeto (PADRAO-COSCA §2)
│   └── adr/                  # Architecture Decision Records
├── CI/
│   ├── vercel.json           # deploy do web (rootDirectory: apps/web)
│   └── docker/               # build+test+deploy do api
└── package.json              # workspaces (turborepo/pnpm)
```

Convenções:
- **Design tokens UMA vez**: `packages/brand/theme.css` com `@theme {}` (gera utilities: `--color-mint-500` → `bg-mint-500`). Web e studio importam `@import "../../../packages/brand/theme.css"`; GSAP e Remotion leem as mesmas vars; shadcn consome as vars (componentes em `components/ui/` usam token, nunca hex cru).
- **Nunca redeclarar token por app** (drift) e nunca `CSS vars em :root onde @theme deveria estar` — anti-pattern da casa.
- **Dark mode** re-mapeia a camada semântica (`html[data-theme]`), nunca redeclara tokens crus.

## Design
Seguir `knowledge/PADRAO-COSCA.md` na íntegra:
- **`docs/DESIGN.md` por projeto = fonte de verdade** — leia antes de todo pixel; em QA, flague código que não bate com ele.
- **Tokens em 3 camadas**: Primitive (valores crus) → Semantic (papel) → Component (botão/input). OKLCH; base 4px; escala tipográfica única; dark mode = re-mapear a semântica. **Nunca hex cru em componente.** Validar por script.
- **Cor**: máx 1 acento, saturação <80%; neutros quentes OU frios, nunca mistos; contraste ≥4.5:1 texto / ≥3:1 não-texto; cor nunca é o único sinal de estado.
- **Tipografia**: 2 faces + mono funcional; corpo 16px; medida 65-75ch; tracking ≥ -0.04em.
- **Motion**: frequency gate; UI de produtividade <300ms (180ms ideal); easing ease-out chegando / ease-in saindo; exit mais sutil que enter; só transform/opacity/clip-path; **`prefers-reduced-motion` inegociável** (no mesmo código).
- **Estados completos**: hover, disabled, loading, error, empty — sempre.
- **Portão de entrega**: o checklist do PADRÃO-COSCA aplicado ao scaffold (abaixo).

## Decision Matrix
| Situação | Template | Motivo |
|----------|----------|--------|
| Frontend + backend + vídeo/mídia, tokens compartilhados | **cosca-monorepo** | precisa de web+api+studio; tokens únicos em packages/brand |
| Frontend + backend leves, um app só | saas / landing (API routes do Next) | monorepo é peso desnecessário |
| Só frontend estático/marketing | landing | uma superfície, sem backend |
| Só API pura | api | sem frontend nem mídia |
| CLI ferramenta | cli | single surface, zero web |
| SDK / lib reutilizável | sdk | entrega como pacote, não como app |
| Aplicativo nativo | mobile | fora do monorepo web; tokens via design system se preciso |

Regra de ouro: **monorepo quando há ≥2 superfícies que compartilham identidade.** Uma superfície só = template leve. Subir de nível por medição, não por hype (P13).

## Checklist
O portão do PADRÃO-COSCA (`knowledge/PADRAO-COSCA.md`) aplicado ao scaffold:
- [ ] `docs/DESIGN.md` criado e lido antes de qualquer pixel
- [ ] Tokens 3 camadas validados por script (contraste, escalas) — todos em `packages/brand/theme.css`
- [ ] `apps/web` + `apps/api` + `apps/studio` importam o MESMO `theme.css`; nenhum token redeclarado
- [ ] Estados: hover/disabled/loading/error/empty em toda superfície
- [ ] `prefers-reduced-motion` presente no mesmo código da animação (GSAP e Remotion)
- [ ] Contraste ≥4.5:1 / ≥3:1 medido (não chutado)
- [ ] Monorepo montado (web+api+studio+shared), tokens únicos
- [ ] Banco escolhido pela árvore de decisão, não por hábito (SQLite → Postgres → Mongo/Redis)
- [ ] Migrations forward-only + teste de banco em container
- [ ] Copy com fórmula (headline + CTA + hook 3 componentes)
- [ ] Output direto: lead with the action, zero preamble
- [ ] Evidência fresca de cada "está pronto" (P13)
- [ ] Zero slop: bans verificados (Inter, 3 cards, purple, em-dash)

## REFERÊNCIAS
PADRAO-COSCA.md · NEXTJS.md · TAILWIND.md · SHADCN.md · GSAP.md · NESTJS.md · REMOTION.md · VERCEL.md · database/README.md

**Criado:** 2026-08-16, como blueprint-mãe do Padrão COSCA — "tokens uma vez, tudo conversa".
