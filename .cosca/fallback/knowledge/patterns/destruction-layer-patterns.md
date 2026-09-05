# Camada 9: 🏗️ Destruction — O Mundo é Modificável

> **Version**: 1.0.0 | **Confidence**: 0.81 | **Category**: Destruction/Physics/Modification | **Created**: 2026-08-23 | **Source**: GitHub API (stars/license) + conhecimento + template do professor

> **Mined by**: cosca-kernel (ordem do Don). **Gap #5 do mining-map-world-vivo.md.** O mundo não é estático — o agente pode destruir, modificar, e reconstruir. Cada projeto abaixo é uma **capacidade de modificação** que o Cosca pode dar ao mundo.

## Purpose

Minar a camada **Destruction** do Mundo Vivo. O agente precisa: (1) destruir o mundo (voronoi fracture, ragdoll), (2) modificar o mundo ( Vehicle physics, soft bodies), (3) reconstruir o mundo (PCG após destruição). Cada projeto abaixo é uma **capacidade de modificação**.

---

## 1. MuJoCo

### REPO
`google-deepmind/mujoco` — ★14.6k — Apache-2.0 — C++/Python

### PAPER/MÉTODO
MuJoCo (Multi-Joint dynamics with Contact) é o **motor de simulação física mais usado em AI research**. Simula: ragdoll, veículos, soft bodies, contact dynamics, articulated bodies. Usado para treinar robôs, agentes, e simulações físicas complexas.

### MODELO
Não é neural — é simulação física de alta fidelidade. Binário/library.

### LICENSE
Apache-2.0 ✅ (tudo liberado)

### DEPENDÊNCIAS
C++, Python bindings. Leve (~50MB). CPU (sem GPU necessária).

### BENCHMARK
State-of-the-art em simulação de multi-joint dynamics. Mais preciso que PhysX em many robotic scenarios.

### INTEGRAÇÃO NO COSCA
MuJoCo é o **motor de simulação do agente** — quando o mundo precisa de física precisa (ragdoll, veículos, soft bodies), MuJoCo é a escolha. Para o Cosca: simular o agente interagindo com o mundo (destruir, empurrar, quebrar).

### POSSÍVEL PLUGIN
`cosca-destruction-mujoco` — plugin que recebe estado do mundo + ação → retorna mundo após física simulada.

### UTILIDADE NO COSCA
**Alta.** MuJoCo é o **padrão da indústria em AI** — se o agente vai interagir fisicamente com o mundo, MuJoCo é o motor mais confiável.

---

## 2. Box2D

### REPO
`erincatto/box2d` — ★10.3k — MIT — C

### PAPER/MÉTODO
Box2D é o motor de **física 2D** mais usado em games. Simula: rigid bodies, joints, contacts, AABB broadphase. Usado em Angry Birds, Cut the Rope, e milhares de jogos indie.

### MODELO
Física 2D (rigid body). Binário/library.

### LICENSE
MIT ✅

### DEPENDÊNCIAS
C puro. Sem dependências. Extremamente leve.

### BENCHMARK
State-of-the-art em 2D game physics. Veloc estável em 60fps com milhares de bodies.

### INTEGRAÇÃO NO COSCA
Box2D é o **motor de física 2D** — para simulações leves (mini-mapa 2D, UI interativa, protótipos). MIT, leve, confiável.

### POSSÍVEL PLUGIN
`cosca-destruction-box2d` — plugin 2D leve para simulações rápidas.

### UTILIDADE NO COSCA
**Média.** Box2D é 2D — para o mundo 3D, MuJoCo/PhysX são melhores. Mas para protótipos e UI, é perfeito.

---

## 3. matter-js

### REPO
`liabru/matter-js` — ★18.4k — MIT — JavaScript

### PAPER/MÉTODO
matter-js é um motor de **física 2D para web** (JavaScript). Simula: rigid bodies, constraints, composites, collision filtering. Foco em elegância e simplicidade de API.

### MODELO
Física 2D (rigid body). JavaScript.

### LICENSE
MIT ✅

### DEPENDÊNCIAS
JavaScript puro (browser ou Node.js). Extremamente leve.

### BENCHMARK
Bom para web. Não tão rápido quanto Box2D para simulações pesadas.

### INTEGRAÇÃO NO COSCA
matter-js é o **motor de física para web** — se o Cosca precisar de simulação 2D em interface web (visualização de destruição, protótipos interativos), matter-js é a escolha.

### POSSÍVEL PLUGIN
`cosca-destruction-matter` — simulação 2D para visualização web.

### UTILIDADE NO COSCA
**Baixa-Média.** Útil para visualização web. Para simulação real, usar MuJoCo/PhysX.

---

## 4. Bullet Physics

### REPO
`bulletphysics/bullet3` — ★13k — zlib — C++

### PAPER/MÉTODO
Bullet é um motor de **física 3D** open-source (mais antigo que PhysX). Suporta: rigid body, soft body, ragdoll, vehicle, cloth. Usado em muitos jogos e filmes.

### MODELO
Física 3D (rigid+soft body). Binário/library.

### LICENSE
zlib ✅ (permissivo, uso comercial)

### DEPENDÊNCIAS
C++. Moderado.

### BENCHMARK
Bom em geral, mas MuJoCo/PhysX são mais modernos e rápidos em muitos cenários.

### INTEGRAÇÃO NO COSCA
Bullet é uma **alternativa open-source ao PhysX** — se o Cosca precisa de física 3D sem as restrições do PhysX (BSD) ou da complexidade do MuJoCo, Bullet é uma opção madura.

### POSSÍVEL PLUGIN
`cosca-destruction-bullet` — física 3D open-source alternativa.

### UTILIDADE NO COSCA
**Média.** Bullet é mais antigo que MuJoCo/PhysX. Para novos projetos, MuJoCo é melhor. Para compatibilidade com assets existentes, Bullet é útil.

---

## 5. PhysX / Jolt (já na camada VFX)

PhysX (★4.7k, BSD) e Jolt (★2.5k, MIT) já foram minerados na camada VFX (#7). Aqui confirmamos que eles também servem para **destruição**: PhysX tem `destructible meshes` (voronoi fracture), Jolt tem soft body e destruction. São a **física production** do mundo.

---

## Síntese da Camada Destruction

| # | Projeto | ★ | License | Capacidade no Cosca |
|---|---------|---|---------|---------------------|
| 1 | **MuJoCo** | 14.6k | Apache-2.0 | Simulação física de alta fidelidade (ragdoll, veículos, soft bodies) |
| 2 | Box2D | 10.3k | MIT | Física 2D leve (protótipos, UI) |
| 3 | matter-js | 18.4k | MIT | Física 2D para web |
| 4 | Bullet | 13k | zlib | Física 3D open-source alternativa |
| 5 | PhysX/Jolt | 4.7k/2.5k | BSD/MIT | Física production (já na VFX layer) |

### O pipeline de destruição/modificação

```
Mundo (PCG gerado)
    │
    ├─→ MuJoCo (simulação) → o agente interage fisicamente
    │
    ├─→ PhysX (destructible meshes) → voronoi fracture, ragdoll
    │
    ├─→ PCG (reconstrução) → mundo regenerado após destruição
    │
    └─→ Loop: destruir → reconstruir → destruir (o mundo é vivo)
```

### Prioridade de instalação
1. **MuJoCo** (Apache, Python, alta fidelidade) — motor principal
2. **PhysX** (BSD, já na VFX) — destructible meshes
3. **Box2D** (MIT, 2D) — protótipos leves
4. **Jolt** (MIT, já na VFX) — física leve
5. **Bullet** (zlib, alternativa) — compatibilidade

## Related Patterns

- [`vfx-layer-patterns.md`](vfx-layer-patterns.md) — PhysX/Jolt/Taichi (física e simulação)
- [`generative-media-patterns.md`](generative-media-patterns.md) — PCG (o mundo que é destruído e reconstruído)
- [`unreal-integration-patterns.md`](unreal-integration-patterns.md) — Chaos Destruction (física do UE)
