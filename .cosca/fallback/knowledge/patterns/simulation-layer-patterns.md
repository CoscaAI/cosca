# Camada 10: 🌦️ Simulation — O Mundo Evolui

> **Version**: 1.0.0 | **Confidence**: 0.79 | **Category**: Simulation/Ecosystem/Multi-agent | **Created**: 2026-08-23 | **Source**: GitHub API (stars/license) + conhecimento + template do professor

> **Mined by**: cosca-kernel (ordem do Don). **Gap #6 do mining-map-world-vivo.md.** O mundo não é estático — ele evolui: clima muda, ecossistemas se desenvolvem, economia flutua, multidões se movem. Template: REPO→PAPER→MODEL→LICENSE→DEPS→BENCHMARK→INTEGRATION→PLUGIN→UTILITY.

## Purpose

Minar a camada **Simulation** do Mundo Vivo. O mundo precisa evoluir mesmo quando o agente não age: (1) clima/estações, (2) ecossistemas (plantas+animais), (3) economia (recursos+comércio), (4) multidões (NPCs autônomos). Cada projeto abaixo é uma **capacidade de simulação emergente**.

---

## 1. Mesa (agent-based simulation)

### REPO
`mesa/mesa` — ★3.8k — Apache-2.0 — Python

### PAPER/MÉTODO
Mesa é um framework Python para **simulação baseada em agentes** (ABM). Estrutura: `Model` (ambiente) + `Agent` (indivíduos com comportamento) + `Space` (grid/continuous). Suporta: scheduling (Random, Simultaneous, Staged), coletas de dados, visualização (built-in). Usado em pesquisa acadêmica (epidemiologia, economia, ecologia).

### MODELO
Não é neural — é framework ABM. Agentes com regras simples → comportamento emergente.

### LICENSE
Apache-2.0 ✅

### DEPENDÊNCIAS
Python puro. Sem dependências externas. Extremamente leve.

### BENCHMARK
Padrão acadêmico em ABM. Não é o mais rápido, mas é o mais maduro e extensível.

### INTEGRAÇÃO NO COSCA
Mesa é o **motor de simulação multi-agente do Cosca** — o agente do Cosca pode interagir com dezenas/centenas de NPCs autônomos, cada um com seus próprios objetivos e comportamento. É a base para: ecossistemas, economia, sociedades.

### POSSÍVEL PLUGIN
`cosca-sim-mesa` — plugin que recebe config de agentes + regras → retorna simulação step-a-step com coleta de dados.

### UTILIDADE NO COSCA
**Muito Alta.** Mesa é o **framework de simulação do mundo** — sem ele, o mundo é um cenário estático. Com ele, o mundo tem vida emergente.

---

## 2. MuJoCo (já na camada Destruction)

MuJoCo (★14.6k, Apache) também serve para simulação — não só destruição. Pode simular: ecossistemas físicos (animais se movendo), veículos, robôs interagindo com o ambiente. É o **motor de simulação física de alta fidelidade**.

---

## 3. NetLogo

### REPO
`NetLogo/NetLogo` — ★1.2k — GPL-3.0 — Scala/Java

### PAPER/MÉTODO
NetLogo é a **plataforma mais usada para simulação multi-agente** em educação e pesquisa. Interface gráfica, linguagem de programação simples, milhares de modelos prontos (ecossistemas, economia, epidemias, tráfego). É o "Hello World" da simulação deagentes.

### MODELO
ABM com interface gráfica. Linguagem proprietária (NetLogo).

### LICENSE
GPL-3.0 ⚠️ (requer open-source do código que usa; para Cosca interno OK)

### DEPENDÊNCIAS
Java (JVM). Moderado (~200MB).

### BENCHMARK
Não é para performance — é para prototipagem e educação. Modelos prontos são o valor.

### INTEGRAÇÃO NO COSCA
NetLogo é o **prototipador rápido de simulações** — o Don pode testar ideias de simulação (ecossistema, economia) usando os modelos prontos do NetLogo antes de implementar no Cosca/Mesa.

### POSSÍVEL PLUGIN
`cosca-sim-netlogo` — plugin que carrega modelos NetLogo (.nlogo) e roda simulações.

### UTILIDADE NO COSCA
**Média.** NetLogo é mais para prototipagem/educação que para produção. Mas os modelos prontos são valiosos como referência.

---

## 4. ABCE (economic simulation)

### REPO
`AB-CE/abce` — ★225 — Python

### PAPER/MÉTODO
ABCE (Agent-Based Computational Economics) é um framework para **simulação econômica** multi-agente. Agentes: consumidores, firmas, governo. Simula: comércio, produção, consumo, mercado. Built-in: coleta de dados, gráficos.

### MODELO
ABM econômico. Agentes com objetivos econômicos.

### LICENSE
Não especificada (verificar)

### DEPENDÊNCIAS
Python, numpy. Leve.

### BENCHMARK
Específico para economia — não é generalista como Mesa.

### INTEGRAÇÃO NO COSCA
ABCE é o **motor econômico do mundo** — se o mundo tem economia (comércio, recursos, mercado), ABCE fornece a base. Pode ser integrado ao Mesa para simulações econômicas.

### POSSÍVEL PLUGIN
`cosca-sim-economy` — plugin que simula economia (agentes, comércio, mercado).

### UTILIDADE NO COSCA
**Média.** ABCE é específico para economia — útil se o mundo tem comércio. Para simulação geral, Mesa é melhor.

---

## 5. Weather / Climate Simulation

### REPO
- `igarciad/weather_simulation` — ★39 — C++ (weather patterns)
- `mustartt/hydraulic-erosion` — ★10 — MIT (erosão hidráulica para terreno)
- Conhecimento: UE5 Weather System (não open-source), Unity Houdini (não open-source)

### PAPER/MÉTODO
Simulação de clima: padrões de vento, chuva, neve, temperaturas por estação. Erosão hidráulica: como água molda o terreno (rios, vales, canyons). É o **clima e a erosão** que dão "vida temporal" ao mundo.

### LICENSE
Variável (verificar cada repo)

### DEPENDÊNCIAS
C++/Python. Variável.

### BENCHMARK
Específico para cada cenário.

### INTEGRAÇÃO NO COSCA
Clima/erosão é o **tempo do mundo** — o mundo não é só estático, ele muda com o tempo (estações, erosão, enchentes). Pode ser simulado via Taichi (que tem MPM para erosão) ou via regras simples no Mesa.

### POSSÍVEL PLUGIN
`cosca-sim-climate` — plugin que simula clima (estações, vento, chuva) e erosão temporal.

### UTILIDADE NO COSCA
**Média-Alta.** Clima é o que separa "mundo estático" de "mundo que muda com o tempo".

---

## 6. Crowd Simulation

### REPO
Fraco no GitHub (projetos de crowd simulation são geralmente proprietários ou acadêmicos). Conhecimento: RVO2 (Reciprocal Velocity Obstacles), Unity Local Avoidance, UE Crowd Manager.

### PAPER/MÉTODO
Crowd simulation: dezenas/centenas de NPCs se movendo autonomamente com colisão, formação de fila, comportamento de grupo. Algoritmos: RVO (velocity obstacles), social forces, flow fields.

### INTEGRAÇÃO NO COSCA
Crowd simulation é o **mundo com pessoas** — NPCs caminhando, formando fila, reagindo ao agente. Pode ser integrado ao Mesa (agentes com movimentação) ou via algoritmo RVO2 (open-source, C++).

### POSSÍVEL PLUGIN
`cosca-sim-crowd` — plugin que simula multidão (agentes com colisão+comportamento de grupo).

### UTILIDADE NO COSCA
**Média-Alta.** Multidão é o que dá "vida social" ao mundo — sem ela, o mundo é vazio.

---

## Síntese da Camada Simulation

| # | Projeto | ★ | License | Capacidade no Cosca |
|---|---------|---|---------|---------------------|
| 1 | **Mesa** | 3.8k | Apache-2.0 | Framework ABM (agentes+ambiente+ scheduling) |
| 2 | MuJoCo | 14.6k | Apache-2.0 | Simulação física (já na Destruction) |
| 3 | NetLogo | 1.2k | GPL-3.0 | Prototipagem rápida de simulações |
| 4 | ABCE | 225 | ? | Economia multi-agente |
| 5 | Climate/Weather | ~50 | Variável | Clima + erosão temporal |
| 6 | Crowd Sim | Fraco | — | Multidão (NPCs autônomos) |

### O pipeline de simulação do mundo

```
Mundo (PCG gerado + VFX + Destruction)
    │
    ├─→ Mesa (motor principal) → agentes + regras → comportamento emergente
    │
    ├─→ MuJoCo (física) → interações físicas de alta fidelidade
    │
    ├─→ Climate → clima muda → estações → erosão temporal
    │
    ├─→ Economy → comércio → recursos → mercado
    │
    └─→ Crowd → NPCs se movem → multidão → vida social
```

### Prioridade de instalação
1. **Mesa** (Apache, Python, framework completo) — o coração da simulação
2. **Climate rules** (regras simples no Mesa ou Taichi) — o tempo do mundo
3. **Crowd (RVO2)** — o mundo com pessoas
4. **Economy (ABCE)** — o mundo com comércio
5. **NetLogo** (prototipagem) — testar ideias

## Related Patterns

- [`mega-brain-patterns.md`](mega-brain-patterns.md) — memória + agentes (o "cérebro" dos NPCs simulados)
- [`ruflo-patterns.md`](ruflo-patterns.md) — swarm, federation (simulação multi-agente)
- [`mining-map-world-vivo.md`](mining-map-world-vivo.md) — Gap #6 (Simulation) é esta camada
- [`unreal-integration-patterns.md`](unreal-integration-patterns.md) — World Partition (mundo streaming enquanto simula)
