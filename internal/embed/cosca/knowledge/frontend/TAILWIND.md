# 02 — TAILWIND v4 INTELLIGENCE

> Stack 02 da Cosca Engineering Intelligence Matrix.
> Doutrina do sistema de estilos CSS-first da casa: `@theme`, tokens que geram utilities.

## MISSÃO
Estilizar interfaces com Tailwind v4 dominando a virada CSS-first: design tokens declarados em `@theme` que geram utilities automaticamente, sem `tailwind.config.js`.

## PRINCÍPIOS CORE
1. **CSS-first, zero config** — `@import "tailwindcss"` injeta theme → preflight → utilities; não existe `tailwind.config.js` · UNIVERSAL
2. **`@theme {}` declara tokens que GERAM utilities** — `--color-mint-500` vira `bg-mint-500` automaticamente · UNIVERSAL
3. **Theme vars ≠ `:root`** — vars de theme instruem o Tailwind a criar classes utilitárias; `:root` é só CSS global · STRONG
4. **Tokens compartilháveis** — o `theme.css` do brand pode ser importado por qualquer app do monorepo · STRONG
5. **Variação por breakpoint como variante** — breakpoints viram variants (`3xl:*`), sem config extra · UNIVERSAL

## REGRAS DE DECISÃO
- `@theme inline` quando o valor referencia outra var por valor (não se resolve em CSS var runtime).
- `@keyframes` dentro de `@theme` alimentam `--animate-*` e geram a utility `animate-*`.
- Tokens da casa ficam em `packages/brand/theme.css` (fonte da verdade única) e são importados com `@import "../brand/theme.css"`.
- Escalas e cores: OKLCH; tokens Primitive → Semantic → Component, conforme PADRAO-COSCA §2.
- Dark mode re-mapeia a camada semântica, nunca redeclara tokens crus.

## ANTI-PATTERNS
`CSS vars em :root onde @theme deveria estar` · `tailwind.config.js recriado à mão` · `hex cru no componente em vez de token` · `repetir tokens por app em vez de importar o theme.css` · `@theme inline usado sem entender a diferença (var por valor vs referência)` · `breakpoints inventados fora da escala do design system`

## CHECKLIST
- [ ] Zero `tailwind.config.js`; tudo em CSS
- [ ] Todos os tokens de design em `@theme`, não em `:root`
- [ ] Utilities geradas usadas (`bg-mint-500`), não `bg-[var(--color-mint-500)]`
- [ ] Tokens de brand importados de um único `theme.css`
- [ ] Dark mode por re-mapeamento semântico
- [ ] Animações com `--animate-*` + `@keyframes` em `@theme`

## A REGRA
No Tailwind v4, o tema É o CSS: `@theme` é a fronteira entre "valor" e "utility" — declara o token uma vez e o framework cria a classe para você.

## REFERÊNCIAS
Tailwind CSS v4 docs · Tailwind v4 upgrade guide · PADRAO-COSCA §2 e §6 · Vercel Agent Skills (tailwind-css)

## CONHECIMENTO (schema _SCHEMA.md)

```
ID: frontend-006
DOMAIN: frontend
TITLE: @theme gera utilities a partir de design tokens
PROBLEM: Design tokens declarados não viravam classes úteis; exigia config JS e repetição
CONTEXT: Tailwind v4, CSS-first
PRINCIPLE: Toda var de tema em `@theme` instrui o Tailwind a criar a utility correspondente
RECOMMENDATION: Declarar `--color-mint-500` no @theme e usar `bg-mint-500` no markup
WHEN_TO_USE: Qualquer estilo novo derivado de token
WHEN_NOT_TO_USE: Valor descartável de um componente (inline/utility avulsa)
TRADE_OFFS: Convenção forte (nome da var define a classe) vs menos liberdade arbitrária
EXAMPLE: `--color-mint-500: #14B8A6;` → `bg-mint-500`, `text-mint-500`, `border-mint-500`
COUNTER_EXAMPLE: `--color-mint-500` declarado em `:root` — vira só CSS var, sem utility gerada
FAILURE_MODES: Token fora de @theme que o dev espera que gere classe (silencioso)
REFERENCES: Tailwind v4 docs (Theme variables)
CONFIDENCE: UNIVERSAL
SOURCE: Tailwind v4 + PADRAO-COSCA §6
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: frontend-007
DOMAIN: frontend
TITLE: CSS-first sem tailwind.config.js
PROBLEM: Config em JS fragmentava tema e exigia rebuild; CSS-first centraliza tudo no stylesheet
CONTEXT: Tailwind v4
PRINCIPLE: `@import "tailwindcss"` injeta a ordem canônica: theme → preflight → utilities
RECOMMENDATION: Manter todo tema no CSS (camadas/`@theme`); config JS só para exceções reais
WHEN_TO_USE: Novo projeto ou migração v4
WHEN_NOT_TO_USE: Projeto com v3 legado e config enorme já madura (migrar com plano)
TRADE_OFFS: Tudo em CSS (portável, versionável) vs quebrar hábitos de config JS
EXAMPLE: Projeto vazio com `globals.css` contendo `@import "tailwindcss"` + `@theme`
COUNTER_EXAMPLE: Migração v3→v4 que mantém `tailwind.config.js` inteiro por inércia
FAILURE_MODES: Ordem de import errada sobrescrevendo utilities
REFERENCES: Tailwind v4 upgrade guide
CONFIDENCE: UNIVERSAL
SOURCE: Tailwind v4
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: frontend-008
DOMAIN: frontend
TITLE: Tokens de brand compartilhados via @import entre apps
PROBLEM: Tokens duplicados por app divergem; o design system perde a fonte única
CONTEXT: Monorepo (turborepo/nx) com vários apps
PRINCIPLE: O theme é código; importa-se o `theme.css` do brand, não se reescreve token
RECOMMENDATION: Manter `packages/brand/theme.css` e `@import "../brand/theme.css"` em cada app
WHEN_TO_USE: Monorepo com web/api/studio compartilhando identidade
WHEN_NOT_TO_USE: App único isolado (o theme.css local basta)
TRADE_OFFS: Acoplamento ao pacote brand vs consistência garantida
EXAMPLE: web/ e studio/ importam o mesmo theme.css; mudança de cor propaga
COUNTER_EXAMPLE: Cada app redeclara `--color-*` com hex divergente
FAILURE_MODES: Import relativo quebrado por path errado; ordem de cascata entre themes
REFERENCES: PADRAO-COSCA §6
CONFIDENCE: STRONG
SOURCE: PADRAO-COSCA + Tailwind v4
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```
