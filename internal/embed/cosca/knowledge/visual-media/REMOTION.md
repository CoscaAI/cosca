# 17 — REMOTION INTELLIGENCE

> Stack 17 da Cosca Engineering Intelligence Matrix.
> Doutrina de vídeo programático da casa: vídeo como função pura do frame, determinístico e renderizável em servidor.

## MISSÃO
Gerar vídeo por código com Remotion — declarando composições e sequências como funções puras do frame, com render headless em Node, controle por CLI e o compositor nativo em Rust.

## PRINCÍPIOS CORE
1. **Vídeo é função pura do frame** — dado o frame, o output é determinístico; `useCurrentFrame()` + `useVideoConfig()` · UNIVERSAL
2. **Composição declara a mídia** — `<Composition>` define id/fps/durationInFrames/width/height; `<Sequence>` time-shift · UNIVERSAL
3. **Interpolação e spring** — `interpolate(frame,[in],[out])` e `spring({frame,fps,config})`/`Easing` movem valores de forma determinística · UNIVERSAL
4. **Render é uma API** — Node renderer (`bundle` → `selectComposition` → `renderMedia`/`renderStill`) roda em servidor · STRONG
5. **Schema é contrato de edição** — Zod no Composition habilita visual editing e validação dos props · STRONG

## REGRAS DE DECISÃO
- `registerRoot` como entrada; o root registra as `<Composition>`s disponíveis.
- `<Sequence from={n}>` desloca o tempo; `durationInFrames` limita a janela; aninhar sequências para sobrepor camadas.
- Metadados dinâmicos (duração/tamanho calculados) vão em `calculateMetadata`, não em defaultProps gigantes.
- Assets em `public/` acessados com `staticFile()`.
- Playback no browser via `@remotion/player`; render final via CLI `npx remotion render` ou API Node.
- `npx remotion studio` para iterar visualmente com hot reload.
- GPU/headless: compositor nativo em Rust cuida do decode/encode; pensar em chunks quando o vídeo é longo.

## ANTI-PATTERNS
`defaultProps gigantes (usar calculateMetadata)` · `frame-dependent code com estado mutável (quebra o determinismo)` · `números mágicos de timing em vez de spring/interpolate` · `render de vídeo longo em um único process` · `buscar dados dentro do componente de frame (deve vir de props)` · `Composition sem schema Zod perdendo o visual editing` · `depender de window/document (SSR quebra)`

## CHECKLIST
- [ ] Root registrado com `registerRoot`
- [ ] `<Composition>` com id/fps/durationInFrames/width/height + schema Zod
- [ ] Sequências com `from`/`durationInFrames` explícitos
- [ ] Movimento via `spring`/`interpolate`/`Easing`, sem números mágicos
- [ ] Metadados dinâmicos em `calculateMetadata`
- [ ] Assets via `staticFile()`
- [ ] Render testado via CLI e, se servidor, via API Node headless
- [ ] Determinismo: mesmo frame → mesmo pixel

## A REGRA
Remotion transforma vídeo em software: se você consegue descrever o frame como função pura do tempo, consegue gerar, iterar e renderizar vídeo como código — com a CPU, o servidor ou o CI.

## REFERÊNCIAS
Remotion docs (remotion.dev) · @remotion/player · @remotion/cli · @remotion/bundler · PADRAO-COSCA §6 · visual-media/README.md

## CONHECIMENTO (schema _SCHEMA.md)

```
ID: visual-media-001
DOMAIN: visual-media
TITLE: Vídeo como função pura do frame
PROBLEM: Vídeo por código vira estado e mutação; impossível de depurar e renderizar de forma previsível
CONTEXT: Remotion, vídeo programático
PRINCIPLE: Dado `frame`, o render é determinístico — componente é função de (frame, props, config)
RECOMMENDATION: Tudo derivado de `useCurrentFrame()` + `useVideoConfig()`; nada de estado mutável dentro do frame
WHEN_TO_USE: Qualquer composição Remotion
WHEN_NOT_TO_USE: Edição de timeline interativa tradicional (editor de vídeo manual)
TRADE_OFFS: Modelo determinístico rígido vs renderização confiável e testável
EXAMPLE: `const opacity = interpolate(frame, [0,30],[0,1])` — mesmo frame, mesmo pixel
COUNTER_EXAMPLE: Contador mutável fora do frame que muda a saída dependendo de quando foi chamado
FAILURE_MODES: Acessar window/document (SSR) quebrando o render headless
REFERENCES: Remotion determinism docs
CONFIDENCE: UNIVERSAL
SOURCE: Remotion + PADRAO-COSCA §6
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: visual-media-002
DOMAIN: visual-media
TITLE: Composition + Sequence para estruturar a mídia
PROBLEM: Vídeos com múltiplas cenas precisam de estrutura de tempo explícita
CONTEXT: Remotion
PRINCIPLE: Composition declara o vídeo (id/fps/duração/dimensão); Sequence desloca e limita janelas de tempo
RECOMMENDATION: Aninhar `<Sequence from durationInFrames>` para cenas; Composition como contrato raiz com schema Zod
WHEN_TO_USE: Todo vídeo com mais de uma cena
WHEN_NOT_TO_USE: Frame único/imagem estática (renderStill direto)
TRADE_OFFS: Hierarquia de Sequence vs timing aditivo que exige disciplina
EXAMPLE: `Composition id="intro"` com duas `<Sequence from={0}>` e `<Sequence from={90}>`
COUNTER_EXAMPLE: Uma Composition gigante com tudo no mesmo frame timeline implícito
FAILURE_MODES: `from` fora do alcance ou sobreposição indevida cortando cenas
REFERENCES: Remotion Sequence docs
CONFIDENCE: UNIVERSAL
SOURCE: Remotion
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: visual-media-003
DOMAIN: visual-media
TITLE: spring/interpolate como linguagem de movimento
PROBLEM: Timing com números mágicos produz movimento artificial e não-editável
CONTEXT: Remotion, motion
PRINCIPLE: Movimento determinístico se expressa com `spring` (física) e `interpolate` (mapeamento) — não com hardcode
RECOMMENDATION: `spring({frame, fps, config})` para entradas/saídas; `interpolate` + `Easing` para mapeamentos explícitos
WHEN_TO_USE: Qualquer animação de valores
WHEN_NOT_TO_USE: Corte seco por edição (hard cut) onde não há movimento
TRADE_OFFS: Aprender API de física vs movimento natural e ajustável
EXAMPLE: `const p = spring({frame, fps, config: {damping: 12}}); opacity = interpolate(p,[0,1],[0,1])`
COUNTER_EXAMPLE: `if (frame > 20) return 1` — timing quebradiço e sem easing
FAILURE_MODES: Spring com `fps` errado (config do Composition) desincronizando
REFERENCES: Remotion spring/interpolate docs
CONFIDENCE: UNIVERSAL
SOURCE: Remotion
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: visual-media-004
DOMAIN: visual-media
TITLE: calculateMetadata em vez de defaultProps gigantes
PROBLEM: defaultProps enormes duplicam dados, poluem o Composition e trazem a duração para o front
CONTEXT: Remotion, composições com dados externos
PRINCIPLE: Metadados calculados (duração, dimensão) devem ser calculados, não declarados em props estáticas
RECOMMENDATION: Computar `durationInFrames`/`width`/`height` em `calculateMetadata` a partir dos props essenciais
WHEN_TO_USE: Duração/tamanho dependente de conteúdo (dados vindos de API)
WHEN_NOT_TO_USE: Vídeo com forma fixa conhecida em build time
TRADE_OFFS: Um hook de metadata vs centralizar tudo no componente
EXAMPLE: `calculateMetadata: ({props}) => ({durationInFrames: props.items.length * 30})`
COUNTER_EXAMPLE: `defaultProps={{items: [...300 itens], durationInFrames: 9000}}`
FAILURE_MODES: calculateMetadata chamado sem await de dados externos corretamente
REFERENCES: Remotion calculateMetadata docs
CONFIDENCE: STRONG
SOURCE: Remotion + PADRAO-COSCA §6
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: visual-media-005
DOMAIN: visual-media
TITLE: Render headless via Node API (bundle → selectComposition → renderMedia)
PROBLEM: Render precisa rodar em servidor/CI sem browser manual
CONTEXT: Remotion em backend/pipeline
PRINCIPLE: O pipeline de render é uma API: `bundle()` o projeto, `selectComposition()`, `renderMedia()`/`renderStill()`
RECOMMENDATION: Expor render como job (fila) usando a Node API; CLI para desenvolvimento
WHEN_TO_USE: Geração de vídeo em servidor, CI, sob demanda
WHEN_NOT_TO_USE: Iteração local — use `remotion studio`/`render`
TRADE_OFFS: API mais verbosa vs automação e isolamento do processo
EXAMPLE: `bundle()` → `selectComposition({id})` → `renderMedia({composition, serveUrl})`
COUNTER_EXAMPLE: Abrir o studio em produção para "gerar vídeo"
FAILURE_MODES: Concorrência de renders de vídeo longo estourando memória (chunk/serialize)
REFERENCES: Remotion renderer docs (@remotion/bundler, renderMedia)
CONFIDENCE: STRONG
SOURCE: Remotion
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```
