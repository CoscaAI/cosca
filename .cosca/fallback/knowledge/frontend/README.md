# 02 — FRONTEND ENGINEERING INTELLIGENCE

> Stack 02 da Cosca Engineering Intelligence Matrix.

## MISSÃO
Construir frontend escalável, performático, acessível, testável e previsível — e saber explicar **por que** a arquitetura é apropriada.

## PRINCÍPIOS CORE
1. **Estado vive no lugar certo** (server state no cache, client state no componente, URL state no router) · UNIVERSAL
2. **Composição > props booleanas** · STRONG
3. **Boundaries de servidor/cliente explícitas** (Server Components vs Client Components) · STRONG
4. **Code splitting e lazy loading por rota** · UNIVERSAL
5. **Efeitos são a exceção** (derivar > efeito; menos `useEffect` desnecessário) · STRONG

## REGRAS DE DECISÃO
- Estado global SÓ quando múltiplos ramos da árvore precisam — senão, local/lifted.
- Cache de server state (TanStack Query) > fetch manual em `useEffect`.
- Virtualizar listas longas; nunca renderizar 10k linhas.
- Memoizar com intenção (profiling antes, não depois).

## ANTI-PATTERNS
`giant components` · `prop drilling` · `estado global desnecessário` · `efeitos desnecessários` · `client-side everything` · `waterfall fetching` · `bundle gigante` · `API logic duplicada` · `design system prematuro`

## CHECKLIST (antes de entregar)
- [ ] Boundaries de estado corretas
- [ ] Sem waterfall no fetch
- [ ] Bundle auditado (nenhuma dep pesada desnecessária)
- [ ] Teclado/contraste/reduced-motion
- [ ] Listas grandes virtualizadas
- [ ] Erros tratados com recovery

## A REGRA
Frontend bom não é o mais "avançado" — é o que equilibra performance, acessibilidade e previsibilidade para o usuário real.

## DOUTRINAS
- `NEXTJS.md` — Next.js App Router: file-conventions como API, rotas dinâmicas, route groups, RSC
- `TAILWIND.md` — Tailwind v4: CSS-first, `@theme` que gera utilities, tokens compartilhados
- `SHADCN.md` — shadcn/ui: código é SEU, Radix + cva + Tailwind, dark mode por `data-theme`
- `GSAP.md` — GSAP: Tween/Timeline, `useGSAP()` + cleanup, transform no GPU, reduced-motion

## REFERÊNCIAS
React · Next.js · TanStack Query/Router · React Aria · Testing Library · Vercel Agent Skills (react-best-practices, composition-patterns, view-transitions)
