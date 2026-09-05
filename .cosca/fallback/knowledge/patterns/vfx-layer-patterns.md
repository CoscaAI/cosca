# Camada 7: ✨ VFX — O Mundo é Vivo

> **Version**: 1.0.0 | **Confidence**: 0.80 | **Category**: VFX/Simulation/Particles | **Created**: 2026-08-23 | **Source**: GitHub API (stars/license) + conhecimento + template do professor

> **Mined by**: cosca-kernel (ordem do Don). **Gap #3 do mining-map-world-vivo.md.** O agente não só vê e entende o mundo — ele precisa SENTIR que o mundo é vivo: vento, fogo, água, poeira, destruição, partículas. Template: REPO→PAPER→MODEL→LICENSE→DEPS→BENCHMARK→INTEGRATION→PLUGIN→UTILITY.

## Purpose

Minar a camada **VFX** do Mundo Vivo. O mundo precisa ter "vida" visual: partículas (poeira, faíscas), fluidos (água, fumaça), física (destruição, cloth), e efeitos procedurais (vento, chuva, clima). Cada projeto abaixo é uma **capacidade de simulação** que o Cosca pode dar ao mundo.

---

## 1. Taichi

### REPO
`taichi-dev/taichi` — ★26k — Apache-2.0 — C++/Python

### PAPER/MÉTODO
Taichi é uma **linguagem e runtime para computação gráfica e simulação física**. Permite escrever simulações (fluidos, partículas, cloth, destruição) em Python com performance de C++ (compila para GPU via LLVM/SPIR-V). Arquitetura: frontend Python → IR otimizado → backend (CUDA/Metal/Vulkan/DX12). Suporta: SPH fluids, PBD, partículas, MPM (Material Point Method), fractura.

### MODELO
Não é um modelo ML — é uma linguagem/runtime. Pesos não se aplicam. O código é o produto.

### LICENSE
Apache-2.0 ✅ (tudo liberado — uso comercial, modificação, distribuição)

### DEPENDÊNCIAS
Python, LLVM, GPU (CUDA/Metal/Vulkan). Leve para development (~500MB). Runtime precisa de GPU para performance.

### BENCHMARK
SPH fluids: real-time em resoluções moderadas. MPM: state-of-the-art em simulação de materiais (areia, neve, comida). Performance: 10-100x mais rápido que implementações Python puras.

### INTEGRAÇÃO NO COSCA
Taichi é o **motor de simulação do mundo** — o Cosca pode escrever simulações em Python (que o Don domina) e rodar com performance de GPU. Exemplos: água corrente, fumaça, poeira, vento, cloth (bandeiras, roupas), destruição (voronoi fracture). Liga ao PCG (mundos gerados precisam de "vida") e ao Unreal (simulações que rodam no Cosca podem alimentar VFX no UE).

### POSSÍVEL PLUGIN
`cosca-vfx-taichi` — plugin que recebe parâmetros de simulação (tipo, resolução, seed) → retorna frames de simulação (partículas, mesh, velocities). Input: config de simulação. Output: sequência de frames + metadados.

### UTILIDADE NO COSCA
**Muito Alta.** Taichi é o **coração da "vida" do mundo** — sem simulação, o mundo é estático. Com Taichi, ele tem fluidos, partículas, destruição, cloth. E é Apache-20 (o Don pode usar sem restrições).

---

## 2. NVIDIA PhysX

### REPO
`NVIDIA-Omniverse/PhysX` — ★4.7k — BSD-3-Clause — C++

### PAPER/MÉTODO
PhysX é o **motor de física mais usado em games** (Unreal, Unity). BSD-3-Clause (open-source). Suporta: ragdoll, vehicles, cloth, fluids (APB), destructible meshes, joints, triggers. PhysX 5: GPU-accelerated physics (massive parallelism).

### MODELO
Não é neural — é física clássica (rigid body, soft body, fluid). Binário/library.

### LICENSE
BSD-3-Clause ✅ (uso comercial, modificação, distribuição — tudo liberado)

### DEPENDÊNCIAS
C++, GPU (Omniverse/PhysX 5). Moderado.

### BENCHMARK
State-of-the-art em game physics. PhysX 5 GPU: milhões de bodies em tempo real.

### INTEGRAÇÃO NO COSCA
PhysX é o **motor de física do mundo** — colisões, ragdoll, veículos, cloth, destruição. Para o Cosca, pode ser integrado como lib C++ ou via Omniverse (se o Don quiser). Para o Unreal, PhysX já é o backend do UE — o Cosca pode se comunicar com ele via API.

### POSSÍVEL PLUGIN
`cosca-vfx-physx` — plugin que recebe estado do mundo → retorna física simulada (colisões, ragdoll, cloth).

### UTILIDADE NO COSCA
**Alta.** PhysX é o **padrão da indústria** — se o mundo vai ter física (e vai), PhysX é a escolha madura. Para o Cosca, é mais uma lib C++ que um plugin Python.

---

## 3. Jolt Physics

### REPO
`godot-jolt/godot-jolt` — ★2.5k — MIT — C++

### PAPER/MÉTODO
Jolt Physics é um motor de física **leve e rápido**, criado por Jan Bosch (ex-Activision). Foco em performance e determinismo. Usado no Godot Engine como plugin. Suporta: rigid body, soft body, vehicle, ragdoll, joints.

### MODELO
Física clássica. Binário/library.

### LICENSE
MIT ✅ (tudo liberado)

### DEPENDÊNCIAS
C++. Leve (~100KB lib).

### BENCHMARK
Mais rápido que PhysX em许多 cenarios (benchmarks do Godot). Determinístico (importante para replay/sync).

### INTEGRAÇÃO NO COSCA
Jolt é uma **alternativa leve ao PhysX** — se o Cosca precisa de física sem o overhead do PhysX/Omniverse, Jolt é a escolha. MIT, leve, rápido.

### POSSÍVEL PLUGIN
`cosca-vfx-jolt` — plugin leve de física para simulações onde PhysX é pesado demais.

### UTILIDADE NO COSCA
**Média-Alta.** Jolt é o **física leve** — para protótipos e simulações rápidas. PhysX é para produção pesada.

---

## 4. Position Based Dynamics (PBD)

### REPO
`InteractiveComputerGraphics/PositionBasedDynamics` — ★2.2k — MIT — C++

### PAPER/MÉTODO
PBD (Position Based Dynamics) é um método de simulação que ajusta posições diretamente (não forças). Resultado: estável, fast, e fácil de controlar. Suporta: cloth, soft bodies, ragdoll, fluids (PBF). O método é o padrão para cloth em games (Unreal, Unity).

### MODELO
Física baseada em posição. Binário/library.

### LICENSE
MIT ✅

### DEPENDÊNCIAS
C++. Leve.

### BENCHMARK
Real-time para cloth e soft bodies. Mais estável que métodos force-based.

### INTEGRAÇÃO NO COSCA
PBD é o **método de simulação para cloth e soft bodies** — bandeiras, roupas, cabelos, elementos flexíveis. Essencial para dar "vida" aos personagens e ao mundo.

### POSSÍVEL PLUGIN
`cosca-vfx-pbd` — plugin que recebe config de cloth/soft body → retorna simulação frame-a-frame.

### UTILIDADE NO COSCA
**Média-Alta.** PBD é o **cloth/soft body do agente** — sem ele, tudo é rígido.

---

## 5. SPlisHSPlasH (SPH Fluids)

### REPO
`InteractiveComputerGraphics/SPlisHSPlasH` — ★1.9k — MIT — C++

### PAPER/MÉTODO
SPlisHSPlasH é um framework de **simulação de fluidos SPH** (Smoothed Particle Hydrodynamics). Implementa múltiplos métodos: WCSPH, IISPH, PBF, DFSPH, APSFL. Suporta: água, fumaça, deformáveis. Benchmark comparisons inclusos.

### MODELO
SPH (partículas). Binário/library.

### LICENSE
MIT ✅

### DEPENDÊNCIAS
C++, GPU (OpenCL/CUDA). Moderado.

### BENCHMARK
DFSPH: state-of-the-art em SPH fluids. Comparações com outros métodos inclusas.

### INTEGRAÇÃO NO COSCA
SPlisHSPlasH é o **framework de fluidos** — água corrente, fumaça, lava, poeira líquida. Para o Cosca, pode ser integrado como lib C++ ou via Taichi (que já tem SPH).

### POSSÍVEL PLUGIN
`cosca-vfx-sph` — plugin que recebe config de fluido → retorna simulação de partículas.

### UTILIDADE NO COSCA
**Média.** Se o Cosca já usa Taichi (que tem SPH), SPlisHSPlasH é um **backend alternativo** mais maduro para fluidos de alta qualidade.

---

## 6. bevy_hanabi (GPU Particles)

### REPO
`djeedai/bevy_hanabi` — ★1.4k — Apache-2.0 — Rust

### PAPER/MÉTODO
bevy_hanabi é um **sistema de partículas GPU** para o Bevy Engine. Partículas rodam na GPU (compute shaders), com efeitos: spawn, lifetime, velocity, size, color, force fields. Data-driven (efeitos definidos por config, não código).

### MODELO
Compute shaders (WGSL). Partículas como dados GPU.

### LICENSE
Apache-2.0 ✅

### DEPENDÊNCIAS
Rust, Bevy Engine. GPU obrigatória.

### BENCHMARK
Milhões de partículas em tempo real (GPU-accelerated).

### INTEGRAÇÃO NO COSCA
bevy_hanabi é o **padrão de referência para partículas GPU** — o conceito (efeitos data-driven em GPU) pode ser adaptado para o Cosca. Se o Don quiser partículas no Cosca (poeira, faíscas, chuva), o modelo de bevy_hanabi é o que seguir.

### POSSÍVEL PLUGIN
`cosca-vfx-particles` — sistema de partículas GPU data-driven (inspirado em bevy_hanabi). Input: config de efeito. Output: frames de partículas.

### UTILIDADE NO COSCA
**Média.** bevy_hanabi é Rust/Bevy — não integra direto com Python/Go do Cosca. Mas o **conceito** (partículas GPU data-driven) é o que o Cosca deve replicar.

---

## 7. PixelFlow

### REPO
`diwi/PixelFlow` — ★1.3k — MIT — Java

### PAPER/MÉTODO
PixelFlow é uma biblioteca de **simulação e visualização de fluidos** baseada em Navier-Stokes (GPU via GLSL). Fluidos 2D/3D, smoke, fire, divergence-free noise. Simples de usar (Processing/Java).

### MODELO
Navier-Stokes (fluid dynamics). GPU (GLSL).

### LICENSE
MIT ✅

### DEPENDÊNCIAS
Java, OpenGL/GLSL. Leve.

### BENCHMARK
Real-time para fluidos 2D. Fluidos 3D mais pesados.

### INTEGRAÇÃO NO COSCA
PixelFlow é o **fluid dynamics simples** — para efeitos 2D (UI, mini-mapa,background) ou protótipos rápidos. Não é production-grade como Taichi/SPlisHSPlasH.

### POSSÍVEL PLUGIN
`cosca-vfx-pixelflow` — fluid dynamics leve para efeitos 2D/protótipos.

### UTILIDADE NO COSCA
**Baixa-Média.** Útil para protótipos e efeitos 2D. Para 3D production, usar Taichi.

---

## Síntese da Camada VFX

| # | Projeto | ★ | License | Capacidade no Cosca |
|---|---------|---|---------|---------------------|
| 1 | **Taichi** | 26k | Apache-2.0 | Motor de simulação completo (fluidos+partículas+cloth+destruição) |
| 2 | PhysX | 4.7k | BSD-3 | Física padrão da indústria (colisões+ragdoll+veículos) |
| 3 | Jolt Physics | 2.5k | MIT | Física leve e rápida |
| 4 | PBD | 2.2k | MIT | Cloth e soft bodies |
| 5 | SPlisHSPlasH | 1.9k | MIT | Fluidos SPH de alta qualidade |
| 6 | bevy_hanabi | 1.4k | Apache-2.0 | Partículas GPU data-driven (referência conceitual) |
| 7 | PixelFlow | 1.3k | MIT | Fluid dynamics simples (2D/protótipos) |

### O pipeline VFX do mundo

```
Mundo (PCG/Unreal)
    │
    ├─→ Taichi (motor principal) → fluidos + partículas + cloth + destruição
    │
    ├─→ PhysX/Jolt (física) → colisões + ragdoll + veículos
    │
    ├─→ PBD (cloth) → roupas + bandeiras + cabelos
    │
    └─→ Partículas GPU → poeira + faíscas + chuva + vento
```

### Prioridade de instalação (quando o NVMe chegar)
1. **Taichi** (Apache, Python, motor completo) — o coração da "vida"
2. **PBD** (MIT, cloth) — o que dá flexibilidade ao mundo
3. **Jolt** (MIT, física leve) — para protótipos rápidos
4. **PhysX** (BSD, física production) — para o mundo final
5. **SPlisHSPlasH** (MIT, fluidos) — se Taichi não for suficiente
6. **bevy_hanabi** (conceito de partículas GPU)
7. **PixelFlow** (protótipos 2D)

## Related Patterns

- [`generative-media-patterns.md`](generative-media-patterns.md) — PCG (o mundo gerado que o VFX anima)
- [`unreal-integration-patterns.md`](unreal-integration-patterns.md) — Niagara (VFX do UE — o Cosca pode alimentar via Taichi)
- [`vision-layer-patterns.md`](vision-layer-patterns.md) — Depth Anything (profundidade para VFX 3D)
- [`mining-map-world-vivo.md`](mining-map-world-vivo.md) — Gap #3 (VFX) é esta camada
