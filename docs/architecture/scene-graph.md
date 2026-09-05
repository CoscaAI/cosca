# Scene Graph (internal/scene)

> Ordem do Professor (L303): kernels → **Scene Graph** → métricas → **Visual Compiler**.
> Esta fase implementa o Scene Graph — o grafo de **ENTIDADES ESPACIAIS**.
> O Visual Compiler (Scene DSL → cena) é a **PRÓXIMA fase — NÃO foi implementado.**

## A separação conceitual (a pedra angular)

| Grafo | O que é | Onde vive |
|-------|---------|-----------|
| **Node Graph** | grafo de **COMPUTAÇÃO** (operações) | `internal/nodegraph` |
| **Scene Graph** | grafo de **ENTIDADES ESPACIAIS** (Camera, Sun, Terrain, Tree, ParticleSystem) | `internal/scene` |

Cada entidade **APONTA** para uma operação do node graph (`Entity.NodeRef`, ex:
`"fbm"`) e carrega os dados que o executor daquele nó consome (`Entity.Params`,
ex: seed, octaves, width). Isso evita transformar o node graph num monolito: a
cena é puramente espacial; a computação fica nos nós.

## Transform (local → mundo)

- `Transform{Position, Rotation, Scale}`, rotação em **GRAUS** (UX da casa),
  convertida internamente para radianos.
- Matriz 4x4 **column-major** (elemento `(r,c)` em `m[c*4+r]`), ponto como
  vetor coluna: `v' = M·v`.
- Composição **T\*R\*S** (decisão da família): escala → rotação → translação
  aplicadas ao ponto. `R = Rz·Ry·Rx` (rotação em graus).
- `LocalMatrix` = T·R·S. `WorldMatrix(parent)` = pai_local · local (hierarquia
  local→mundo pela cadeia de pais). `Apply(v)` leva um ponto local ao espaço do pai.

## A ponte Compile (entidade → nó)

```
Scene → Compile → Graph → Run
```

`Compile(s, g, reg)` transforma a cena em um grafo executável: para cada
entidade com `NodeRef` não vazio, garante um nó `Type=NodeRef` com
`params = cópia(entidade.Params)`. Entidades espaciais puras (Camera, Light,
Group) **não** viram nós. Fail-closed: `NodeRef` desconhecido no registry →
erro explícito.

## Exemplo de cena (demo: `cosca ngraph demo-scene`)

```
Scene "scene-demo"
├─ Terrain "terrain"   (NodeRef=fbm, seed=42, 256×256)
├─ Group  "vegetation"
│   ├─ Procedural "vegetation_perlin"  (NodeRef=perlin, seed=7, 128×128)
│   └─ Procedural "vegetation_worley"  (NodeRef=worley, seed=11, 128×128)
└─ Camera  "camera"    (espacial pura — não vira nó)
```

Compila para 3 nós → executa pelo Render Engine (seed 42) → PNGs em
`.cosca/scene-demo/` (terrain, vegetation-perlin, vegetation-worley) +
`metrics.jsonl`.

## Próxima fase

**Visual Compiler** (Scene DSL → cena): a linguagem que descreve cenas e vira
entidades. Não implementado neste escopo (ordem do Don + Professor) — aqui a
cena ainda é montada em Go puro pelos construtores de `builder.go`.
