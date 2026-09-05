# 02 — GSAP INTELLIGENCE

> Stack 02 da Cosca Engineering Intelligence Matrix.
> Doutrina de animação da casa: Tween seta, Timeline sequencia, useGSAP limpa.

## MISSÃO
Animar interfaces com GSAP dominando a arquitetura mental Tween/Timeline e a integração correta com React — transform/opacity no GPU, cleanup automático e respeito a `prefers-reduced-motion`.

## PRINCÍPIOS CORE
1. **Tween seta, Timeline sequencia** — `gsap.to/from/fromTo` anima propriedades; `gsap.timeline()` orquestra sequências aninháveis · UNIVERSAL
2. **Transform sem reflow** — `x/y/rotation/scale`/opacity são atalhos de transform (GPU, sem layout) · UNIVERSAL
3. **Cleanup é lei em React** — `useGSAP()` + `gsap.context()` revertem tudo no unmount · UNIVERSAL
4. **A melhor animação passa despercebida** — duração curta, easing certo, exit mais sutil que enter · STRONG
5. **`prefers-reduced-motion` inegociável** — no mesmo código, não em follow-up · UNIVERSAL

## REGRAS DE DECISÃO
- `position parameter`: `2` = posição absoluta, `"+=2"` = gap após o fim, `"-=2"` = overlap com o anterior.
- `tl.addLabel("nome")` + `tl.seek("nome")` para marcar e pular pontos da timeline.
- React: `useGSAP(() => { ... }, { dependencies })` dentro do componente; contexto isola seletores e reverte.
- `gsap.matchMedia()` para animações responsivas (mobile vs desktop), respeitando reduced motion.
- Plugins sob demanda: ScrollTrigger (scroll), ScrollSmoother (smooth scroll), SplitText (texto), Flip (layout FLIP), Draggable, MotionPath.
- Só `transform`/`opacity`/`clip-path` — nunca animar layout (width/height/top/left).
- Frequência decide intensidade: raro = expressivo; frequente = sutil; 100s/dia = nenhuma (PADRAO-COSCA §3).

## ANTI-PATTERNS
`stagger em tudo` · `spring com bounce em ação utilitária` · `animar width/height/top/left (reflow)` · `timeline sem cleanup no React (vazamento de tweens)` · `motion on mount para conteúdo estático` · `ignorar prefers-reduced-motion` · `cap universal de duração em vez de frequency gate` · `1000 tweens soltos sem timeline`

## CHECKLIST
- [ ] `useGSAP()` com dependências e cleanup automático
- [ ] Só transform/opacity/clip-path (nada de reflow)
- [ ] Timeline para sequências; position parameter consciente
- [ ] `prefers-reduced-motion` no mesmo código da animação
- [ ] Duração por frequency gate (<300ms produtividade; 200-500ms produção)
- [ ] Easing: ease-out chegando, ease-in saindo, exit mais sutil que enter
- [ ] `gsap.matchMedia()` quando animação difere por viewport

## A REGRA
GSAP é um property setter de alta performance com um sequenciador: Tween seta valores, Timeline ordena o tempo, e em React todo tween nasce com um death warrant — o cleanup.

## REFERÊNCIAS
GSAP docs (gsap.com) · useGSAP · ScrollTrigger · SplitText · Flip · design-motion-principles · PADRAO-COSCA §3

## CONHECIMENTO (schema _SCHEMA.md)

```
ID: frontend-013
DOMAIN: frontend
TITLE: Tween seta, Timeline sequencia
PROBLEM: Múltiplas animações avulsas viram caos de timing impossível de orquestrar
CONTEXT: Animações coordenadas em UI
PRINCIPLE: Tween = property setter isolado; Timeline = sequenciador aninhável e encadeável
RECOMMENDATION: Tween para uma transição; Timeline (`tl.to().to()`) para sequências que dependem de ordem
WHEN_TO_USE: Qualquer animação multi-passo ou com overlap
WHEN_NOT_TO_USE: Transição única de um elemento só
TRADE_OFFS: Uma camada a mais vs controle de tempo/overlap e pause/seek
EXAMPLE: `gsap.timeline().to(el,{x:100}).to(el,{opacity:0,"-=0.3"})`
COUNTER_EXAMPLE: Três `gsap.to` independentes com setTimeout mágicos
FAILURE_MODES: Timelines sem fim (loop) segurando o GC
REFERENCES: GSAP Timeline docs
CONFIDENCE: UNIVERSAL
SOURCE: GSAP + design-motion-principles
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: frontend-014
DOMAIN: frontend
TITLE: useGSAP + gsap.context para cleanup no React
PROBLEM: Tweens criados em componentes React vazam e animam nós desmontados
CONTEXT: GSAP dentro de componentes React (App Router)
PRINCIPLE: Todo tween deve ter morte certa no unmount; context reverte em lote
RECOMMENDATION: Envolver animações em `useGSAP(() => {...}, {dependencies})`; nunca em `useEffect` puro com seletores globais
WHEN_TO_USE: Qualquer animação em componente React
WHEN_NOT_TO_USE: Scripts fora do React (SSR de astro/animação standalone)
TRADE_OFFS: Hook proprietário vs garantia de cleanup e isolamento
EXAMPLE: `useGSAP(() => gsap.to(".card", {x: 100}), {dependencies: [id]})`
COUNTER_EXAMPLE: `gsap.to(".card", ...)` num useEffect sem revert (cards antigos seguem animando)
FAILURE_MODES: Esquecer de listar dependência e a animação não re-criar no state change
REFERENCES: useGSAP docs (gsap/react)
CONFIDENCE: UNIVERSAL
SOURCE: GSAP + PADRAO-COSCA §3
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: frontend-015
DOMAIN: frontend
TITLE: Position parameter para controlar timing na timeline
PROBLEM: Sequenciar animações com acúmulo de atrasos fica frágil e ilegível
CONTEXT: Timelines multi-tween
PRINCIPLE: O position parameter expressa quando o tween começa: absoluto, gap ou overlap
RECOMMENDATION: `"+=2"` (gap) e `"-=2"` (overlap) em vez de `delay` acumulado; número puro = posição absoluta
WHEN_TO_USE: Qualquer timeline com 3+ tweens
WHEN_NOT_TO_USE: Duas animações simples consecutivas (default basta)
TRADE_OFFS: Aprender o terceiro argumento vs timing explícito e editável
EXAMPLE: `tl.to(a,{x:50},0).to(b,{x:50},"-=0.5").to(c,{opacity:0},"+=1")`
COUNTER_EXAMPLE: `delay: 0, delay: 0.5, delay: 1.2...` — quebra ao inserir um tween no meio
FAILURE_MODES: Valor absoluto que aponta para antes do começo da timeline
REFERENCES: GSAP position parameter docs
CONFIDENCE: UNIVERSAL
SOURCE: GSAP docs
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: frontend-016
DOMAIN: frontend
TITLE: x/y/rotation como atalhos de transform (GPU)
PROBLEM: Animar propriedades de layout (width/height/top/left) causa reflow e jank
CONTEXT: Animações de performance
PRINCIPLE: `x/y/rotation/scale`/opacity são transform/opacity — compositor-only, sem reflow
RECOMMENDATION: Sempre via atalhos GSAP de transform; nunca animar geometria de layout
WHEN_TO_USE: Qualquer movimento/entrada/saída
WHEN_NOT_TO_USE: Mudança real de layout que deve fluir (expansão de altura com reflow consciente)
TRADE_OFFS: Reduzir flexibilidade de animação estrutural vs 60fps estáveis
EXAMPLE: `gsap.to(card, {x: 100, rotation: 5, opacity: 0.5})`
COUNTER_EXAMPLE: `gsap.to(card, {left: 100, top: 40})` — repaints a cada frame
FAILURE_MODES: Blur/box-shadow animados em paralelo derrubando a composição
REFERENCES: GSAP transform docs · design-motion-principles
CONFIDENCE: UNIVERSAL
SOURCE: GSAP + design-motion-principles
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```
