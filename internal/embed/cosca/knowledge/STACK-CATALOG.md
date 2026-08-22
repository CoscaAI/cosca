# STACK CATALOG — Mapa completo dos Expert Stacks do Cosca

> Índice mestre da Cosca Engineering Intelligence Matrix.
> Cada stack é conhecimento ativável: o orquestrador ativa MÚLTIPLOS stacks conforme a natureza da tarefa.
> Estado: ✅ implementado (doutrina em `knowledge/<dir>/`) · 📌 roadmap (a criar).

## Catálogo (50 stacks)

| # | Stack | Dir | Estado |
|---|---|---|---|
| 01 | 🎨 UI/UX | `design/` | ✅ |
| 02 | 🏗️ Frontend | `frontend/` | ✅ |
| 03 | 📱 Mobile | `mobile/` | ✅ |
| 04 | ⚙️ Backend | `backend/` | ✅ |
| 05 | 🔌 API Engineering | `api/` | ✅ |
| 06 | 🗄️ Database | `database/` | ✅ |
| 07 | 📦 Distributed Systems | `distributed/` | ✅ |
| 08 | 🏛️ Architecture | `architecture/` | ✅ |
| 09 | 🔐 Security | `security/` | ✅ |
| 10 | 🧪 Testing | `testing/` | ✅ |
| 11 | 🐳 DevOps/CI-CD | `devops/` | ✅ |
| 12 | 🚨 SRE/Reliability | `sre/` | ✅ |
| 13 | 🔭 Observability | `observability/` | ✅ |
| 14 | ⚡ Performance | `performance/` | ✅ |
| 15 | 🧠 AI/ML | `ai/` | ✅ |
| 16 | 🛒 E-commerce | `ecommerce/` | ✅ |
| 17 | 👁️ Visual Media (img/PDF/SVG) | `visual-media/` | ✅ |
| 18 | 🎬 Video | `video-media/` | ✅ |
| 19 | 🎧 Audio | `audio/` | ✅ |
| 20 | ☁️ Cloud (AWS/GCP/Azure) | `cloud/` | 📌 |
| 21 | 🌐 Networking | `networking/` | 📌 |
| 22 | 🤖 Agents | `agents/` | ✅ |
| 23 | 📚 RAG/Knowledge | `rag/` | ✅ |
| 24 | 👁️ Computer Vision | `computer-vision/` | 📌 |
| 25 | 📄 Documents | `documents/` | 📌 |
| 26 | 🧬 Scientific Computing | `scientific/` | 📌 |
| 27 | 📊 Data Engineering | `data-engineering/` | 📌 |
| 28 | 📈 Data Visualization | `visualization/` | 📌 |
| 29 | 🎮 Game Development | `game/` | 📌 |
| 30 | 🖥️ Graphics (GL/Vulkan/WebGPU) | `graphics/` | 📌 |
| 31 | 🧱 Systems Programming | `systems/` | 📌 |
| 32 | 🐧 Linux/Kernel | `linux/` | 📌 |
| 33 | 🧪 QA Engineering | `qa/` | 📌 (parcial em `testing/`) |
| 34 | 💳 Fintech | `fintech/` | ✅ |
| 35 | 🏢 Enterprise SaaS | `saas/` | ✅ |
| 36 | 📡 IoT | `iot/` | 📌 |
| 37 | 🔗 Blockchain/Web3 | `web3/` | 📌 |
| 38 | 🛰️ Real-time Systems | `realtime/` | 📌 |
| 39 | 🔄 Workflow Engines | `workflows-engine/` | 📌 |
| 40 | 🧩 Plugin Systems | `plugins/` | 📌 |
| 41 | 🧠 Knowledge Graphs | `knowledge-graph/` | 📌 |
| 42 | 🔎 Search | `search/` | 📌 |
| 43 | 🌍 Internationalization | `i18n/` | 📌 |
| 44 | ♿ Accessibility | `accessibility/` | 📌 (parcial em `design/`) |
| 45 | 🧑💻 Developer Tools | `devtools/` | 📌 |
| 46 | 📝 Compilers | `compilers/` | 📌 |
| 47 | 🖥️ Operating Systems | `os/` | 📌 |
| 48 | 🧰 CLI Engineering | `cli/` | 📌 |
| 49 | 🕸️ Web Scraping | `scraping/` | 📌 |
| 50 | 🗺️ GIS/Maps | `gis/` | 📌 |

## Estrutura padrão de cada stack (a criar quando ativado)

```
STACK/
├── README.md                 ← doutrina (mission, princípios, regra final)
├── mission.md                ← missão e escopo
├── principles.md             ← princípios classificados (universal/contextual/estético)
├── repositories.md           ← referências estudadas + notas
├── architecture.md           ← arquiteturas e decisões
├── patterns.md               ← padrões reutilizáveis
├── anti-patterns.md          ← o que evitar
├── security.md               ← considerações de segurança do domínio
├── performance.md            ← considerações de performance
├── testing.md                ← como testar o domínio
├── decision-matrix.md        ← matriz de decisão (WHEN/WHEN NOT)
├── reference-implementations.md ← exemplos de referência
├── case-studies.md           ← estudos de caso aplicados
└── knowledge/                ← entradas no schema unificado (_SCHEMA.md)
```

> **Prática atual**: cada stack tem `README.md` (doutrina consolidada). Os arquivos especializados são criados conforme o stack é usado e o conhecimento amadurece.

## O PULO DO GATO — Análise → Comparação → Síntese → Decisão

O Cosca NÃO "lê repositórios" — ele processa conhecimento:

```
GitHub Repository
   ↓ ANALYZE (architecture, algorithms, patterns, trade-offs, failures)
   ↓ COMPARE (múltiplos projetos)
   ↓ SYNTHESIZE (princípios próprios)
   ↓ COSCA KNOWLEDGE (schema unificado)
   ↓ DECISION ENGINE (quando usar, quando não)
```

## Ativação multi-stack (por natureza da tarefa)

**"Construa um sistema de pagamentos."** → ativar:
`BACKEND · DATABASE · API · SECURITY · FINTECH · DISTRIBUTED-SYSTEMS · OBSERVABILITY · TESTING · PERFORMANCE · CLOUD`

**"Crie um editor de vídeo online."** → ativar:
`UI/UX · FRONTEND · VIDEO · AUDIO · GRAPHICS · WEBSOCKET · STORAGE · CLOUD · PERFORMANCE · SECURITY · WORKFLOW · AI`

**Regra do orquestrador**: o Kernel identifica a natureza da tarefa e ativa os stacks relevantes — conhecimento especializado acionado sob demanda, não uma pasta gigante de documentação.
