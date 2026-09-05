# Procedural Kernel System (procgen)

> Ordem do Professor (L303): **kernels → métricas → Scene Graph → Visual Compiler**.
> Esta fase implementa os kernels procedurais (famílias **Noise/Math/Pattern**)
> como nós do Node Graph (§21), puro Go determinístico (stdlib apenas), com
> **metadados de performance por operação desde o nascimento** (embrião do
> Performance Memory). **Scene Graph é a PRÓXIMA fase — NÃO foi implementada.**

## O que é

`internal/procgen/` é o **Procedural Kernel System**: funções puras determinísticas
+ nós `nodegraph.Executor` que devolvem a imagem própria do engine (`*RGBA`,
sem depender de `image.RGBA` como tipo principal). Zero dependências novas.

## Determinismo (P1)

- **RNG = splitmix64**: puramente inteiro (um único `uint64` de estado), portável
  **byte a byte** em qualquer máquina. Mesmo seed = mesma sequência.
- Hash de células (`Hash2`) usa a mesma mistura inteira — gradientes e feature
  points determinísticos por seed.
- O seed de um nó resolve-se na ordem: `params["seed"]` (grafo JSON) → campo da
  struct → seed do Render Job (§22, via `render.RenderFromContext`).

## Famílias (as do Professor)

| Família | Funções | Range |
|---------|---------|-------|
| **Noise** | `Perlin2D`, `Simplex2D`, `Worley2D` (F1), `Voronoi2D` (F2-F1, bordas de células), `FBM2D`/`FbmPerlin`/`FbmSimplex` | Perlin/Simplex ~[-1,1]; Worley/Voronoi [0,~1] |
| **Math** | `Add`, `Multiply`, `Remap`, `Clamp`, `Lerp`/`Mix`, `Curve` (smoothstep) | [0,1] quando nós de imagem |
| **Pattern** | `Gradient` (diagonal), `Checker`, `Stripes`, `Cells` (Voronoi por célula), `Rings` | [0,1] |

Decisões documentadas:
- **Voronoi2D** = campo F2-F1 (zero no feature point, pico na borda entre células)
  para atender o uso "bordas de células" e diferenciar de Worley (F1).
- **FBM** avança o RNG só a partir da 2ª octave → `FbmPerlin(1 octave) == Perlin2D`
  exatamente; mais octaves decorrelacionam as camadas.

## Contrato de nós

- `Registry() map[nodegraph.NodeType]nodegraph.Executor` expõe os 16 tipos:
  `perlin`, `simplex`, `worley`, `voronoi`, `fbm`, `gradient`, `checker`,
  `stripes`, `cells`, `rings`, `math_add`, `math_multiply`, `math_remap`,
  `math_clamp`, `math_lerp`, `math_curve`.
- Nós Noise/Pattern geram `*RGBA` via `Map` (nx,ny normalizados 0..1).
- Nós Math recebem 1-2 `*RGBA` — input "a"/"b" (contrato do executor) OU a
  posição `node.Inputs[0]/[1]` (contrato do `nodegraph.Run`, que chaveia por ID).
- Parâmetros: `node.Params` (grafo JSON) com defaults seguros, ou campos da struct.

## Métricas (embrião do Performance Memory)

Todo nó registra uma `OpMetric` ao rodar: operation (tipo do nó), backend
`"cpu-go"`, latência (time.Now), memória estimada (W×H×4), quality. O
`MetricsRecorder` acumula e **despeja JSONL append-only** (`Dump`). `CacheHit`
fica `false` no nó — o cache por assinatura (§23) é decisão do grafo e o
`Stats` de graph/render carrega os `CachedHits` reais. O CLI injeta um recorder
compartilhado via `MetricsSetter`.

## Exemplo de grafo (demo: `cosca ngraph demo-procedural`)

```
fbm(terreno, seed=42, octaves=5) → math_clamp [0,1] → math_lerp(t=0.5)
                                                      ↑ gradient
```

Executado pelo Render Engine (seed 42) → PNGs em `.cosca/procgen-demo/`
(`terrain.png`, `procedural-mix.png`) + `metrics.jsonl`.

## Próxima fase

**Scene Graph** (hierarquia de cenas com transforms/nested) — e depois o Visual
Compiler. Não implementados neste escopo (ordem do Don + Professor).
