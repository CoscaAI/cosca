# Cosca Frontend Enterprise — Arquitetura de Redesign

> **Status**: PROPOSED | **Author**: cosca-architecture | **Target**: v1.4.0
> **Don's Order**: "quero melhorar o design frontend por completo, utilizar o que tem de mais avancado pra deixar nivel enterprise, design elegante bonito com graficos pra monitorar tudo em tempo real sem faltar nada"

---

## 1. Resumo Executivo (para o Don)

**O que vamos fazer**: Transformar o Cosca Console de um painel funcional para um **cockpit enterprise de comando em tempo real** — equivalente ao que um Datadog + Grafana + Linear fariam para uma plataforma de AI orchestration, mas unificado em uma única interface.

**Stack mantida**: Next.js 15 App Router + React 19 + shadcn/ui + Tailwind + @tanstack/react-query. **Zero rewrite**. Evolução progressiva.

**Três pilares da transformação**:

| Pilar | De | Para |
|-------|-----|------|
| **Visual** | Tokyo Night dark funcional | Tokyo Night **evoluído** com glassmorphism, micro-interações, hierarquia visual de 4 níveis |
| **Tempo real** | Polling a cada 5-15s + SSE básico | **SSE + WebSocket híbrido**: métricas críticas via WS push, eventos via SSE, dados históricos via react-query |
| **Dashboards** | 40+ páginas isoladas | **Cockpit unificado** de 6 telas principais com drill-down, cada uma consumindo múltiplos endpoints sincronizados |

**Tempo estimado**: 3 fases, ~6-8 semanas com time de 2-3 devs frontend.

**Risco**: Baixo. A arquitetura atual já é sólida (feature modules, API client tipado, EventClient, chart components). A evolução é incremental sobre o existente.

---

## 2. Diagnóstico do Estado Atual

### 2.1 O que já funciona bem (não mexer)

| Componente | Status | Nota |
|-----------|--------|------|
| `ApiClient` (api.ts) | ✅ Excelente | CSRF, httpOnly cookies, tipagem genérica, tratamento de 401 |
| `EventClient` (event-client.ts) | ✅ Excelente | Refcount de EventSource, backoff 1s/2s/5s/10s, Last-Event-ID |
| `QueryProvider` (react-query) | ✅ Sólido | staleTime 60s, gcTime 5min, retry 1 |
| Feature module pattern | ✅ Sólido | `features/<name>/hooks/use-<name>.ts` + types.ts + index.ts |
| Layout system | ✅ Sólido | Sidebar colapsável, mobile bottom nav, tablet drawer, animações framer-motion |
| Chart components | ✅ Base ok | ChartContainer com loading/error/empty states, BarChart, CHART_COLORS theme-aware |
| Auth flow | ✅ Sólido | ProtectedRoute, httpOnly cookies, auto-redirect on 401 |
| Accessibility | ✅ Bom | SkipNav, focus-visible rings, contraste verificado, touch targets 44px |

### 2.2 Gaps identificados

| Gap | Severidade | Impacto |
|-----|-----------|---------|
| Falta **dashboard unificado** com visão cross-domain | 🔴 Critical | Don precisa navegar 6+ páginas para ver estado completo |
| SSE/WebSocket **não usado para dashboards** — só para run/stream | 🔴 Critical | Métricas são polling a cada 5-15s, não tempo real |
| Biblioteca de gráficos **subutilizada** — só BarChart implementado | 🟡 High | Sem line charts, area charts, gauges, heatmaps, sankey |
| Falta **design system documentado** formalmente | 🟡 High | Cores, tipografia, espaçamento existem mas sem spec central |
| Tema Tokyo Night **genérico** — falta identidade Cosca | 🟡 Medium | shadcn/ui default colors, brand só no logo |
| Server Components **não usados** — tudo é "use client" | 🟡 Medium | Perda de performance SSR e streaming RSC |
| Falta **storyboard/layout de grid** para dashboards | 🟡 Medium | Páginas têm layouts ad-hoc, sem grid system |
| Responsividade **funcional mas datada** | 🟢 Low | Breakpoints só md/lg, falta otimização para telas 1440p+ |

---

## 3. Arquitetura de Dados — Como o Frontend Consome os 86 Endpoints

### 3.1 Estratégia de 3 camadas

```
┌─────────────────────────────────────────────────────────┐
│ CAMADA 1: REACT QUERY (dados históricos e estruturados) │
│ • staleTime: 30s-5min por query                        │
│ • Cache compartilhado entre páginas                     │
│ • ~70 endpoints                                         │
├─────────────────────────────────────────────────────────┤
│ CAMADA 2: SERVER-SENT EVENTS (streams de processo)      │
│ • POST /v1/run/stream, /v1/knowledge/sync/stream       │
│ • GET /v1/status/stream, /v1/agentbridge/.../stream    │
│ • EventClient existente (refcount, backoff)             │
│ • ~5 endpoints                                          │
├─────────────────────────────────────────────────────────┤
│ CAMADA 3: WEBSOCKET (métricas em tempo real)            │
│ • GET /v1/ws com topic subscription                     │
│ • system.cpu, system.memory, agents.events, etc.        │
│ • Push do servidor a cada 1-2s para gráficos ao vivo    │
│ • ~10 tópicos                                           │
└─────────────────────────────────────────────────────────┘
```

### 3.2 Mapa completo: Feature → Endpoint → Estratégia

#### 🟢 CAMADA 1 — React Query (dados estruturados, cache, refetchInterval)

| Feature | Endpoint(s) | refetchInterval | staleTime | Notas |
|---------|------------|-----------------|-----------|-------|
| **Health** | `GET /health`, `GET /ready` | 10s | 5s | Kubernetes probes, lightweight |
| **System Status** | `GET /v1/system/status`, `GET /v1/system/hardware` | 5s | 2s | CPU, mem, disk, uptime — dashboard crítico |
| **Runtime** | `GET /v1/status` | 5s | 2s | Engine health + uptime |
| **Stats (agregado)** | `GET /v1/stats` | 15s | 10s | Agentes ativos, skills, providers, workflows |
| **Agents** | `GET /v1/agents`, `GET /v1/agents/search`, `GET /v1/agents/{name}` | 30s | 15s | Lista + busca + detalhe |
| **Skills** | `GET /v1/skills`, `GET /v1/skills/search`, `GET /v1/skills/{name}` | 30s | 15s | Catálogo de skills |
| **Providers** | `GET /v1/providers`, `GET /v1/providers/{name}` | 30s | 15s | Lista + detalhe + status |
| **Workflows** | `GET /v1/workflows`, `GET /v1/workflows/search`, `GET /v1/workflows/{name}` | 30s | 15s | Lista + busca + detalhe |
| **Executions** | `GET /v1/executions`, `GET /v1/executions/{id}` | 30s | 20s | Histórico de execuções |
| **Traces** | `GET /v1/traces`, `GET /v1/traces/{id}` | 60s | 30s | Timeline de execução |
| **Traces Detail** | `GET /v1/traces/{id}/replay`, `GET /v1/traces/{id}/causal` | on-demand | 30s | Replay + causal chain |
| **Knowledge** | `GET /v1/knowledge/stats`, `GET /v1/knowledge/epistemology`, `GET /v1/knowledge/epistemology/{status}` | 30s | 15s | Stats + epistemologia por status |
| **Knowledge Search** | `POST /v1/knowledge/search` | on-demand | 30s | Mutação de busca |
| **Memory** | `GET /v1/memory/stats`, `GET /v1/memory/search`, `GET /v1/memory/get`, `GET /v1/memory/{id}` | 30s | 15s | Stats + busca + registro |
| **Departments** | `GET /v1/departments`, `GET /v1/departments/threads/{thread}` | 30s | 15s | Lista + thread |
| **Analytics** | `GET /v1/analytics` | 60s | 30s | Admin-only aggregated data |
| **Plugins** | `GET /v1/plugins` | 30s | 15s | Catálogo WASM |
| **API Keys** | `GET /v1/api-keys` | 60s | 30s | Admin only |
| **Audit Logs** | `GET /v1/audit/logs`, `GET /v1/audit/logs/{id}` | 60s | 30s | Admin only |
| **Secrets** | `GET /v1/secrets`, `GET /v1/secrets/{key}` | 60s | 30s | Admin only — keys masked |
| **Users** | `GET /v1/users` | 60s | 30s | Admin only |
| **Kernel Emergency** | `GET /v1/kernel/emergency` | 30s | 15s | Admin only — kill switch status |
| **AgentBridge** | `GET /v1/agentbridge/status`, `GET /v1/agentbridge/sessions`, `GET /v1/agentbridge/sessions/{id}/events` | 10s | 5s | Bridge status + sessions |
| **Auth** | `GET /v1/auth/me` | on-mount | 60s | Current user info |

#### 🟡 CAMADA 2 — SSE Streams (processos contínuos)

| Stream | Endpoint | Direção | Uso no Frontend |
|--------|----------|---------|-----------------|
| **Run Stream** | `POST /v1/run/stream` | POST SSE | Playground + Orchestration — tokens, fases, agentes em tempo real |
| **Status Stream** | `GET /v1/status/stream` | GET SSE | Health dashboard — push de mudanças de status |
| **Sync Stream** | `POST /v1/knowledge/sync/stream` | POST SSE | Knowledge page — progresso de indexação |
| **Session Events Stream** | `GET /v1/agentbridge/sessions/{id}/events/stream` | GET SSE | Agent Bridge — eventos por sessão |
| **Workflow Run Stream** | `POST /v1/workflows/{name}/run/stream` | POST SSE | Workflow execution — output incremental |

#### 🔴 CAMADA 3 — WebSocket (métricas em tempo real, push do servidor)

| Tópico | Fonte no Backend | Frequência | Gráfico no Frontend |
|--------|-----------------|------------|---------------------|
| `system.cpu` | System handler + WS Hub | 1-2s | CPU gauge/sparkline no Command Center |
| `system.memory` | System handler + WS Hub | 2s | Memory usage area chart |
| `system.disk` | System handler + WS Hub | 10s | Disk usage bar |
| `agents.events` | Agents + WS Hub | on-event | Event timeline, agent status |
| `runtime.requests` | Runtime middleware metrics | 5s | Request rate line chart |
| `runtime.latency` | Runtime middleware metrics | 5s | Latency histogram/heatmap |
| `knowledge.indexing` | Knowledge handler event | on-event | Indexing progress bar |
| `workflows.executions` | Workflows handler | on-event | Execution live feed |
| `security.alerts` | Security middleware | on-event | Security Center alert feed |
| `system.uptime` | System handler + WS Hub | 30s | Uptime counter |

---

## 4. Design System Enterprise — Cosca Design Language

### 4.1 Filosofia: "Tokyo Night Evoluído"

O tema atual (hsl 240° 10% 3.9%) é um dark mode funcional mas genérico. A evolução mantém a base escura e adiciona **identidade Cosca**:

- **Background layers**: não um só preto, mas 3 camadas de profundidade
- **Glass morphism**: cards com backdrop-blur em overlays e modais
- **Brand accent**: o brand-600 (#444ce7) vira a cor de destaque primária, substituindo o primary genérico
- **Status semântico**: 5 cores de status com significado fixo (healthy, warning, critical, inactive, neutral)

### 4.2 Paleta de Cores (CSS Variables)

```css
:root {
  /* Base — Tokyo Night Evolved */
  --background: 222 15% 7%;        /* #0f1117 — mais azulado que preto puro */
  --foreground: 210 15% 90%;       /* #e1e4e9 — texto primário */

  /* Surface layers — profundidade visual */
  --card: 222 15% 10%;             /* #161820 — cards e containers */
  --card-hover: 222 15% 13%;       /* #1e2029 — hover state */
  --popover: 222 15% 12%;          /* #1a1c24 — dropdowns, popovers */

  /* Semantic — identidade Cosca */
  --primary: 235 70% 60%;          /* #5162f0 — Cosca brand blue */
  --primary-foreground: 0 0% 100%;

  /* Status colors — semântica fixa */
  --success: 160 84% 45%;          /* #34d399 — healthy/active */
  --warning: 38 96% 58%;           /* #f59e0b — degraded/warning */
  --destructive: 0 72% 55%;        /* #ef4444 — error/critical */
  --info: 217 91% 60%;             /* #3b82f6 — informational */
  --neutral: 220 10% 45%;          /* #6b7280 — inactive/unknown */

  /* Chart palette — 8 cores vibrantes para gráficos */
  --chart-1: 235 70% 60%;          /* Brand blue */
  --chart-2: 160 84% 45%;          /* Emerald */
  --chart-3: 38 96% 58%;           /* Amber */
  --chart-4: 280 65% 60%;          /* Purple */
  --chart-5: 340 82% 60%;          /* Rose */
  --chart-6: 195 80% 50%;          /* Cyan */
  --chart-7: 25 90% 55%;           /* Orange */
  --chart-8: 140 50% 55%;          /* Green */

  /* Glass effect */
  --glass-bg: 222 15% 7% / 0.7;
  --glass-border: 222 15% 20% / 0.5;
  --glass-blur: 12px;
}
```

### 4.3 Tipografia

| Token | Font | Size | Weight | Uso |
|-------|------|------|--------|-----|
| `--font-display` | Inter | clamp(2rem, 3vw, 2.75rem) | 700 | Títulos de página |
| `--font-h1` | Inter | clamp(1.5rem, 2vw, 2rem) | 600 | Section headers |
| `--font-h2` | Inter | clamp(1.25rem, 1.5vw, 1.5rem) | 600 | Card titles |
| `--font-h3` | Inter | 1.125rem | 600 | Sub-section titles |
| `--font-body` | Inter | 0.9375rem (15px) | 400 | Body text |
| `--font-small` | Inter | 0.8125rem (13px) | 400 | Secondary text, labels |
| `--font-caption` | Inter | 0.75rem (12px) | 500 | Captions, badges, chart labels |
| `--font-mono` | JetBrains Mono | 0.8125rem | 400 | Code, IDs, tokens, timestamps |
| `--font-mono-small` | JetBrains Mono | 0.75rem | 400 | Inline code, keyboard shortcuts |
| `--font-stat` | Inter | clamp(1.75rem, 2.5vw, 2.5rem) | 700 | Stat cards, KPIs |
| `--font-stat-label` | Inter | 0.75rem | 500 | KPI labels |

### 4.4 Grid System

```
┌──────────────────────────────────────────────────────┐
│ Layout: 12-column grid, 24px gutter, 16px padding    │
│                                                      │
│ Dashboard Full-Width (default):                      │
│ ┌──────────┬──────────┬──────────┬──────────┐       │
│ │  span 3  │  span 3  │  span 3  │  span 3  │       │
│ ├──────────┴──────────┼──────────┴──────────┤       │
│ │      span 6         │      span 6         │       │
│ ├─────────────────────┴─────────────────────┤       │
│ │               span 12                     │       │
│ └───────────────────────────────────────────┘       │
│                                                      │
│ Detail Page (content + aside):                       │
│ ┌────────────────────────┬──────────┐               │
│ │       span 8-9         │ span 3-4 │               │
│ └────────────────────────┴──────────┘               │
│                                                      │
│ Breakpoints:                                         │
│   sm:  640px  (mobile)                               │
│   md:  768px  (tablet portrait)                      │
│   lg:  1024px (tablet landscape / small desktop)     │
│   xl:  1280px (desktop)                              │
│   2xl: 1536px (large desktop — 1440p+)               │
│   3xl: 1920px (ultrawide — 1080p fullscreen)         │
└──────────────────────────────────────────────────────┘
```

### 4.5 Design Tokens Adicionais

```css
:root {
  /* Spacing scale (4px base) */
  --space-1: 0.25rem;   /* 4px */
  --space-2: 0.5rem;    /* 8px */
  --space-3: 0.75rem;   /* 12px */
  --space-4: 1rem;      /* 16px */
  --space-5: 1.25rem;   /* 20px */
  --space-6: 1.5rem;    /* 24px */
  --space-8: 2rem;      /* 32px */
  --space-10: 2.5rem;   /* 40px */
  --space-12: 3rem;     /* 48px */
  --space-16: 4rem;     /* 64px */

  /* Border radius */
  --radius-sm: 0.375rem;   /* 6px — badges, tags, small buttons */
  --radius-md: 0.5rem;     /* 8px — cards, inputs, buttons */
  --radius-lg: 0.75rem;    /* 12px — modals, large cards */
  --radius-xl: 1rem;       /* 16px — main containers */
  --radius-full: 9999px;   /* Pills, avatars */

  /* Shadows (dark mode) */
  --shadow-sm: 0 1px 2px 0 rgb(0 0 0 / 0.3);
  --shadow-md: 0 4px 6px -1px rgb(0 0 0 / 0.4), 0 2px 4px -2px rgb(0 0 0 / 0.3);
  --shadow-lg: 0 10px 15px -3px rgb(0 0 0 / 0.5), 0 4px 6px -4px rgb(0 0 0 / 0.3);
  --shadow-xl: 0 20px 25px -5px rgb(0 0 0 / 0.6), 0 8px 10px -6px rgb(0 0 0 / 0.3);

  /* Glass */
  --glass-bg: hsl(222 15% 7% / 0.7);
  --glass-border: hsl(222 15% 25% / 0.3);
  --glass-blur: 12px;

  /* Transitions */
  --transition-fast: 150ms cubic-bezier(0.4, 0, 0.2, 1);
  --transition-base: 200ms cubic-bezier(0.4, 0, 0.2, 1);
  --transition-slow: 300ms cubic-bezier(0.4, 0, 0.2, 1);
}
```

---

## 5. Stack de Visualização Recomendada

### 5.1 Biblioteca principal: Recharts (já instalada)

**Por que manter Recharts:**
- Já está no projeto (v3.10.1) com ChartContainer, BarChart, CHART_COLORS implementados
- Suporta todos os tipos de gráfico necessários
- Theme-aware (CSS variables)
- Animação nativa
- ResponsiveContainer built-in
- Bundle tree-shakeable (~45KB gzip para o subset usado)

### 5.2 Tipos de gráfico por feature

| Feature | Gráfico | Tipo Recharts | Descrição |
|---------|---------|---------------|-----------|
| **Command Center** | CPU/Memory real-time | `AreaChart` + sparkline | Gráfico de área com gradiente, atualização via WebSocket |
| **Command Center** | Request rate | `LineChart` | Linha temporal de requisições/min |
| **Command Center** | System health | `PieChart` (donut) | Distribuição healthy/degraded/down |
| **Command Center** | Latency distribution | `BarChart` (histogram) | P50/P95/P99 em barras agrupadas |
| **Security Center** | Alert timeline | `ScatterChart` | Eventos de segurança no tempo |
| **Security Center** | Threat categories | `Treemap` (custom) ou `BarChart` horizontal | Distribuição por tipo de ameaça |
| **Metrics** | Token usage | `AreaChart` stacked | Tokens in/out ao longo do tempo |
| **Metrics** | Agent activity | `BarChart` horizontal | Atividade por agente (barras) |
| **Metrics** | Provider performance | `RadarChart` | Comparação multi-dimensional |
| **Analytics** | Search trends | `LineChart` multi-series | Tendências de busca |
| **Analytics** | Top queries | `BarChart` horizontal | Top 10 queries |
| **Analytics** | Zero results | `BarChart` ou métrica simples | Taxa de zero results |
| **Traces** | Causal chain | `Sankey` (custom D3 ou @nivo/sankey) | Grafo de causalidade entre eventos |
| **Traces** | Execution timeline | Gantt-like horizontal bars | Linha do tempo de execução |
| **Workflows** | DAG visualization | `@xyflow/react` (já instalado) | Visualização de workflow como grafo |
| **Departments** | Conversation graph | Force-directed graph (D3/react-force-graph) | Grafo de conversas inter-departamento |
| **Providers** | Comparison matrix | Heatmap table | Matriz providers × métricas |
| **Knowledge** | Document distribution | `PieChart` (donut) | Distribuição por status epistemológico |
| **Memory** | Memory growth | `AreaChart` | Crescimento de registros ao longo do tempo |

### 5.3 Bibliotecas adicionais (leves)

```json
{
  "dependencies": {
    "recharts": "^3.10.1",
    "@xyflow/react": "^12.11.2",
    "d3-scale": "^4.0.2",
    "d3-shape": "^3.2.0"
  },
  "devDependencies": {
    "@types/d3-scale": "^4.0.0",
    "@types/d3-shape": "^3.0.0"
  }
}
```

**D3-scale + D3-shape apenas**: ~12KB gzip combinados, usados para escalas de cor e formas customizadas que Recharts não cobre (treemap, sankey). **Não importar D3 inteiro** (≈150KB).

### 5.4 Componentes de visualização a implementar

```
src/components/shared/charts/
├── chart-container.tsx          ✅ existente — refinar
├── bar-chart.tsx                ✅ existente — refinar
├── area-chart.tsx               🆕 Real-time area com gradiente
├── line-chart.tsx               🆕 Multi-series line
├── donut-chart.tsx              🆕 Donut/pie com labels
├── sparkline.tsx                🆕 Mini chart inline (para stat cards)
├── heatmap-table.tsx            🆕 Tabela com gradiente de cor
├── radar-chart.tsx              🆕 Radar/spider multi-eixo
├── timeline-chart.tsx           🆕 Gantt-like horizontal
├── sankey-diagram.tsx           🆕 Fluxo entre nós (traces causais)
├── stat-card-sparkline.tsx      🆕 StatCard + sparkline integrado
├── chart-tooltip.tsx            ✅ existente — refinar
├── chart-legend.tsx             🆕 Legenda interativa com toggle
└── types.ts                     ✅ existente — expandir
```

---

## 6. Dashboard Unificado — O Cockpit

### 6.1 As 6 telas principais (reduzindo de 40+ para 6 hubs)

```
COSCA CONSOLE
├── 🏠 COMMAND CENTER         /command-center     [Overview + System Health]
│   ├── System vitals (CPU, RAM, Disk, Uptime) — WS real-time
│   ├── Request rate & latency — WS real-time
│   ├── Active agents & sessions — react-query 10s
│   ├── Recent executions feed — WS events
│   └── Quick actions (Run, Emergency, Search)
│
├── 🔒 SECURITY CENTER        /security           [Security + Compliance]
│   ├── Threat dashboard — WS alerts
│   ├── Audit log viewer — react-query
│   ├── API key manager — react-query + mutations
│   ├── Secrets vault (masked) — react-query
│   ├── Kernel emergency panel — react-query + mutations
│   └── User management (admin) — react-query
│
├── 📊 ANALYTICS HUB          /analytics          [Métricas + Insights]
│   ├── Token usage over time — AreaChart (react-query)
│   ├── Agent performance leaderboard — BarChart
│   ├── Provider comparison — RadarChart + Heatmap
│   ├── Search analytics — LineChart
│   ├── Knowledge health — DonutChart
│   └── Memory growth — AreaChart
│
├── 🔄 ORCHESTRATION HUB      /orchestration      [Execuções + Workflows]
│   ├── Execution history — Table + timeline
│   ├── Workflow designer — @xyflow/react DAG
│   ├── Trace viewer — Timeline + Sankey causal
│   ├── Pipeline status — react-query
│   ├── Run playground — SSE stream
│   └── Context builder — react-query
│
├── 🧠 KNOWLEDGE HUB          /knowledge          [Knowledge + Memory]
│   ├── Document explorer — Table + search
│   ├── Epistemology dashboard — DonutChart + tree
│   ├── Memory browser — Search + detail
│   ├── Indexing status — SSE stream progress
│   ├── Unknowns queue — List
│   └── Understanding graph — react-query
│
└── 🤖 AI TOOLS HUB           /ai-tools           [Agents + Skills + Plugins]
    ├── Agent catalog — Cards + detail
    ├── Skill marketplace — Grid
    ├── Plugin manager — List
    ├── Provider config — Forms + test
    ├── Template library — Cards
    ├── Prompt manager — Grid + editor
    └── AgentBridge monitor — WS sessions
```

### 6.2 Layout do Cockpit

```
┌─────────────────────────────────────────────────────────────┐
│ TOPBAR: [🍔] Cosca  │  🔍 Global Search  │  🔔 Notifs  │  👤 │
├─────────────────────────────────────────────────────────────┤
│ SIDEBAR │                                                   │
│ (6 hubs)│  ┌─────────────────────────────────────────────┐  │
│         │  │  HEADER: Page Title + Breadcrumb + Actions  │  │
│         │  ├─────────────────────────────────────────────┤  │
│         │  │                                             │  │
│         │  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌────┐ │  │
│         │  │  │ Stat    │ │ Stat    │ │ Stat    │ │Stat│ │  │
│         │  │  │ Card    │ │ Card    │ │ Card    │ │Card│ │  │
│         │  │  │ w/Spark │ │ w/Spark │ │ w/Spark │ │    │ │  │
│         │  │  └─────────┘ └─────────┘ └─────────┘ └────┘ │  │
│         │  │                                             │  │
│         │  │  ┌──────────────────┐ ┌──────────────────┐  │  │
│         │  │  │   Main Chart     │ │   Side Panel     │  │  │
│         │  │  │   (6-8 columns)  │ │   (4-6 columns)  │  │  │
│         │  │  │                  │ │                  │  │  │
│         │  │  └──────────────────┘ └──────────────────┘  │  │
│         │  │                                             │  │
│         │  │  ┌──────────────────────────────────────┐   │  │
│         │  │  │   Full-width Table / Timeline        │   │  │
│         │  │  └──────────────────────────────────────┘   │  │
│         │  └─────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

---

## 7. Arquitetura Técnica — Server Components vs Client Components

### 7.1 Nova estratégia de boundary

```
┌──────────────────────────────────────────┐
│ SERVER COMPONENTS (por padrão)           │
│ • Layouts, página shell, metadata        │
│ • Pré-renderização estática de estrutura │
│ • Streaming SSR para conteúdo pesado     │
├──────────────────────────────────────────┤
│ CLIENT COMPONENTS ("use client")         │
│ • Apenas folhas interativas              │
│ • Charts, forms, real-time hooks         │
│ • Tudo que precisa de estado/browser API │
└──────────────────────────────────────────┘
```

### 7.2 Padrão de composição

```tsx
// app/(dashboard)/command-center/page.tsx — SERVER COMPONENT
import { Suspense } from "react";
import { CommandCenterShell } from "./command-center-shell";
import { SystemVitals } from "./system-vitals";
import { RecentActivity } from "./recent-activity";
import { Skeleton } from "@/components/shared/skeleton";

export default function CommandCenterPage() {
  return (
    <CommandCenterShell>
      <Suspense fallback={<Skeleton variant="rect" height={120} />}>
        <SystemVitals />      {/* Client — uses useQuery + WS */}
      </Suspense>
      <Suspense fallback={<Skeleton variant="rect" height={400} />}>
        <RecentActivity />    {/* Client — uses useCoscaEvents */}
      </Suspense>
    </CommandCenterShell>
  );
}
```

### 7.3 Code splitting e lazy loading

```tsx
// Lazy load gráficos pesados (só carrega quando scrolla para a seção)
const AgentPerformanceChart = dynamic(
  () => import("./agent-performance-chart").then(m => m.AgentPerformanceChart),
  { ssr: false, loading: () => <Skeleton variant="rect" height={350} /> }
);

const TraceViewer = dynamic(
  () => import("./trace-viewer").then(m => m.TraceViewer),
  { ssr: false }
);
```

### 7.4 Bundle budget

| Chunk | Limite | Estratégia |
|-------|--------|------------|
| Framework (Next/React) | ~120KB | Shared, cacheável |
| shadcn/ui components | ~45KB | Tree-shaken, só componentes usados |
| Recharts | ~45KB | Tree-shaken, só tipos usados |
| @xyflow/react | ~50KB | Lazy loaded, só na página de workflows |
| D3 helpers | ~12KB | Só scale + shape |
| App code (features) | ~80KB por rota | Route-based splitting |
| **Total por página** | **~200KB** | Abaixo do limite de 250KB |

---

## 8. Real-time Monitoring — Arquitetura SSE/WebSocket

### 8.1 WebSocket Hook (novo)

```tsx
// hooks/use-cosca-websocket.ts — NOVO
"use client";

import { useEffect, useRef, useState, useCallback } from "react";
import { API_BASE_URL } from "@/lib/constants";

type WSTopic =
  | "system.cpu" | "system.memory" | "system.disk"
  | "agents.events" | "runtime.requests" | "runtime.latency"
  | "knowledge.indexing" | "workflows.executions"
  | "security.alerts" | "system.uptime";

interface WSMessage {
  type: string;
  topic?: string;
  data?: unknown;
}

export function useCoscaWebSocket<T = unknown>(
  topics: WSTopic[],
  onMessage: (topic: string, data: T) => void,
) {
  const wsRef = useRef<WebSocket | null>(null);
  const [connected, setConnected] = useState(false);
  const topicsRef = useRef(topics);
  topicsRef.current = topics;

  useEffect(() => {
    const token = document.cookie
      .split("; ")
      .find(row => row.startsWith("access_token="))
      ?.split("=")[1];

    const ws = new WebSocket(
      `${API_BASE_URL.replace("http", "ws")}/v1/ws?token=${token}`
    );
    wsRef.current = ws;

    ws.onopen = () => {
      setConnected(true);
      // Subscribe to all topics
      for (const topic of topics) {
        ws.send(JSON.stringify({ type: "subscribe", topic }));
      }
    };

    ws.onmessage = (event) => {
      const msg: WSMessage = JSON.parse(event.data);
      if (msg.topic && msg.data !== undefined) {
        onMessage(msg.topic, msg.data as T);
      }
    };

    ws.onclose = () => setConnected(false);

    return () => {
      ws.close();
    };
  }, []);

  // Re-subscribe when topics change
  useEffect(() => {
    if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) return;
    for (const topic of topics) {
      wsRef.current.send(JSON.stringify({ type: "subscribe", topic }));
    }
  }, [topics]);

  return { connected };
}
```

### 8.2 Hook composto: React Query + WebSocket

```tsx
// pattern: useLiveMetric — combina valor inicial de react-query com updates WS
function useLiveMetric<T>(
  queryKey: string[],
  queryFn: () => Promise<T>,
  wsTopic: WSTopic,
) {
  // Base data from react-query (cache, refetch, stale-while-revalidate)
  const query = useQuery({ queryKey, queryFn, refetchInterval: 30_000 });

  // Live updates from WebSocket
  const [liveData, setLiveData] = useState<T | null>(null);

  useCoscaWebSocket([wsTopic], (_, data) => {
    setLiveData(data as T);
  });

  // WS data wins when available, fallback to query
  return {
    data: liveData ?? query.data,
    isLoading: query.isLoading,
    isLive: liveData !== null,
    error: query.error,
  };
}
```

---

## 9. Responsividade

### 9.1 Breakpoints e comportamentos

| Breakpoint | Largura | Sidebar | Grid | Charts |
|-----------|---------|---------|------|--------|
| **Mobile** | < 768px | Bottom nav bar | 1 coluna | Stacked, sem labels |
| **Tablet** | 768-1023px | Slide-in drawer (Sheet) | 2 colunas | Reduzidos, tooltips simplificados |
| **Desktop** | 1024-1279px | Colapsável (68px/260px) | 3 colunas | Padrão |
| **Large** | 1280-1535px | Colapsável | 4 colunas | Full detail |
| **Ultrawide** | ≥ 1536px | Fixo expandido (280px) | 4-6 colunas | Estendidos, sparklines visíveis |

### 9.2 Mobile-first para componentes críticos

- **Stat cards**: horizontal stack no mobile, grid no desktop
- **Charts**: altura reduzida (200px mobile, 300-400px desktop), toggle fullscreen
- **Tables**: horizontal scroll com primeira coluna sticky
- **Modals**: fullscreen sheet no mobile, dialog centrado no desktop
- **Topbar**: ações colapsam em menu "..." no mobile

---

## 10. Acessibilidade (WCAG 2.1 AA)

### 10.1 Checklist de conformidade

| Critério | Status atual | Ação |
|----------|-------------|------|
| **1.1.1 Non-text Content** | ✅ Parcial | Adicionar aria-label em todos os gráficos com descrição textual dos dados |
| **1.3.1 Info and Relationships** | ✅ Bom | Manter heading hierarchy (h1→h2→h3) |
| **1.4.1 Use of Color** | ⚠️ A melhorar | Status badges já usam ícones + cor. Gráficos precisam de patterns ou text labels redundantes |
| **1.4.3 Contrast (Minimum)** | ✅ Verificado | 4.5:1+ para todos os pares de cor |
| **1.4.10 Reflow** | ⚠️ A testar | Garantir que dashboards funcionam em 320px largura sem horizontal scroll |
| **2.1.1 Keyboard** | ✅ Bom | Focus-visible rings, SkipNav |
| **2.4.2 Page Titled** | ✅ | Metadata.title com template |
| **2.4.3 Focus Order** | ✅ | DOM order = visual order |
| **2.4.7 Focus Visible** | ✅ | Ring-2 ring-ring ring-offset-2 |
| **3.3.2 Labels or Instructions** | ✅ | Labels em todos os inputs |
| **4.1.2 Name, Role, Value** | ⚠️ A melhorar | Gráficos precisam de role="img" + aria-label |
| **4.1.3 Status Messages** | ✅ | Sonner toasts + live regions |

### 10.2 Gráficos acessíveis

```tsx
// pattern: chart com descrição para screen readers
<Card>
  <CardHeader>
    <CardTitle id="cpu-chart-title">CPU Usage</CardTitle>
  </CardHeader>
  <CardContent>
    {/* Visual chart */}
    <AreaChart data={data} aria-labelledby="cpu-chart-title">
      {/* ... */}
    </AreaChart>
    {/* Screen reader table (visually hidden) */}
    <div className="sr-only" role="region" aria-label="CPU usage data table">
      <table>
        <caption>CPU usage over time</caption>
        <thead>
          <tr><th>Time</th><th>CPU %</th></tr>
        </thead>
        <tbody>
          {data.map(d => (
            <tr key={d.time}><td>{d.time}</td><td>{d.value}%</td></tr>
          ))}
        </tbody>
      </table>
    </div>
  </CardContent>
</Card>
```

---

## 11. Alinhamento com o Backend

### 11.1 Princípio: "Cada endpoint do backend deve ter representação visual no frontend"

| Módulo Backend | Endpoints | Feature Frontend | Status |
|---------------|-----------|------------------|--------|
| `handler/health` | 2 (health, ready) | Command Center | ✅ Implementado (useDashboard) |
| `handler/system` | 2 (hardware, status) | Command Center | ✅ Implementado (useSystemStatus, useHardware) |
| `handler/stats` | 1 (stats) | Command Center | ⚠️ Parcial — só usado em /metrics |
| `handler/runtime` | 3 (status, health, status/stream) | Command Center + Runtime | ⚠️ SSE stream não usado no frontend |
| `handler/agents` | 3 (list, search, get) | AI Tools Hub | ✅ Implementado (useAgents) |
| `handler/skills` | 4 (list, search, get, install) | AI Tools Hub | ⚠️ Install não tem UI |
| `handler/providers` | 4 (list, get, test, setActive) | AI Tools Hub | ✅ Implementado (useProviders) |
| `handler/workflows` | 5 (list, search, get, run, run/stream) | Orchestration Hub | ⚠️ Run/stream sem UI |
| `handler/run` | 2 (execute, stream) | Orchestration Hub | ✅ Implementado (useOrchestrationStream) |
| `handler/executions` | 2 (list, get) | Orchestration Hub | ✅ Implementado (useExecutions) |
| `handler/traces` | 4 (list, get, replay, causal) | Orchestration Hub | ⚠️ Replay e causal sem UI |
| `handler/departments` | 3 (list, thread, ask) | Orchestration Hub | ✅ Implementado (useDepartments) |
| `handler/knowledge` | 7 (search, index, stats, epistemology, etc) | Knowledge Hub | ⚠️ Index e sync sem UI |
| `handler/memory` | 6 (store, search, get, delete, promote, stats) | Knowledge Hub | ✅ Parcial (useMemory*) |
| `handler/agentbridge` | 8 (status, sessions, events, CRUD) | AI Tools Hub | ⚠️ Sem UI completa |
| `handler/analytics` | 1 (analytics) | Analytics Hub | ✅ Implementado (useAnalytics) |
| `handler/auth` | 6 (login, register, refresh, logout, me, changePassword, csrf) | Auth | ✅ Implementado |
| `handler/users` | 4 (list, create, delete, updateRole) | Security Center | ✅ Implementado (useUsers) |
| `handler/apikeys` | 3 (list, create, revoke) | Security Center | ✅ Implementado |
| `handler/audit` | 3 (list, get, prune) | Security Center | ✅ Implementado (useAuditLogs) |
| `handler/secrets` | 4 (list, create, get, delete) | Security Center | ✅ Implementado (useSecrets) |
| `handler/emergency` | 3 (status, stop, halt) | Security Center | ✅ Implementado (useKernelEmergency) |
| `handler/plugins` | 1 (list) | AI Tools Hub | ✅ Implementado (usePlugins) |
| `handler/websocket` | 1 (ws) | Global | ⚠️ Cliente WS não implementado |
| **TOTAL** | **~86 endpoints** | **6 hubs** | **~75% coberto, ~25% gaps** |

---

## 12. Plano de Implementação em Fases

### FASE 1: Foundation (Semanas 1-2) — "Design System + Core Cockpit"

| # | Tarefa | Estimativa | Dependências |
|---|--------|------------|-------------|
| 1.1 | Criar `design-tokens.css` com paleta evoluída, tipografia, grid, glass | 2d | — |
| 1.2 | Migrar `globals.css` para usar tokens, atualizar :root/.dark | 1d | 1.1 |
| 1.3 | Criar componentes de chart ausentes: AreaChart, LineChart, DonutChart, Sparkline | 3d | 1.1 |
| 1.4 | Criar `StatCardSparkline` (stat card com mini gráfico inline) | 1d | 1.3 |
| 1.5 | Implementar `useCoscaWebSocket` hook | 2d | — |
| 1.6 | Redesenhar Command Center com grid 4-2-1, stat cards com sparklines, WS real-time | 3d | 1.3, 1.4, 1.5 |
| 1.7 | Criar layout de grid reutilizável (`DashboardGrid`, `DashboardRow`, `DashboardPanel`) | 1d | 1.1 |
| 1.8 | Atualizar Sidebar para 6 hubs (reduzir de 26 itens para 6 principais + submenu) | 1d | — |

**Milestone**: Command Center redesenhado com métricas em tempo real via WebSocket.

### FASE 2: Visualization (Semanas 3-4) — "Gráficos e Dashboards Completos"

| # | Tarefa | Estimativa | Dependências |
|---|--------|------------|-------------|
| 2.1 | Implementar Security Center completo (threat dash, audit viewer, kernel panel) | 3d | 1.7 |
| 2.2 | Implementar Analytics Hub (token usage, agent perf, provider comparison, search) | 3d | 1.3, 1.7 |
| 2.3 | Implementar Trace Viewer com Sankey causal diagram + Timeline | 3d | 1.7 |
| 2.4 | Workflow designer com @xyflow/react DAG visual | 2d | 1.7 |
| 2.5 | Knowledge Hub: epistemology dashboard + document tree + indexing progress | 2d | 1.7 |
| 2.6 | Adicionar HeatmapTable, RadarChart, TimelineChart | 2d | 1.3 |

**Milestone**: Todos os 6 hubs com visualizações ricas. 80% dos endpoints com representação visual.

### FASE 3: Polish (Semanas 5-6) — "Enterprise Finish"

| # | Tarefa | Estimativa | Dependências |
|---|--------|------------|-------------|
| 3.1 | Server Components: converter layouts e shells para RSC, streaming SSR | 2d | — |
| 3.2 | Code splitting: lazy load todos os gráficos e páginas secundárias | 1d | — |
| 3.3 | Responsividade completa: testar todos os breakpoints, ajustar layouts | 2d | — |
| 3.4 | Acessibilidade: screen reader tables para gráficos, keyboard nav em dashboards | 2d | — |
| 3.5 | Tema claro: garantir contraste e legibilidade no light mode | 1d | — |
| 3.6 | Animações: micro-interações, transições entre hubs, page transitions | 1d | — |
| 3.7 | Storybook: documentar todos os componentes de chart e dashboard | 2d | — |
| 3.8 | Performance: bundle analysis, otimização de re-renders, memo onde necessário | 1d | — |
| 3.9 | Testes E2E: Playwright para fluxos críticos (login → dashboard → drill-down) | 2d | — |

**Milestone**: Frontend enterprise completo. WCAG 2.1 AA verificado. Bundle < 200KB por página.

---

## 13. Decisões de Arquitetura (ADR-like)

### ADR-001: Recharts sobre Nivo/visx/Tremor

**Decisão**: Manter Recharts como biblioteca principal de gráficos.

**Rationale**:
- Já está instalado e em uso (BarChart, ChartContainer)
- Cobre 90% dos tipos de gráfico necessários
- Theme-aware via CSS variables
- Bundle tree-shakeable (~45KB)
- Comunidade ativa, documentação sólida
- Nivo teria curva de migração alta; visx é muito baixo nível; Tremor é opinionated demais

**Exceções**: D3-scale + D3-shape (~12KB) para Sankey e escalas avançadas.

### ADR-002: WebSocket para métricas em tempo real, não substituir react-query

**Decisão**: WebSocket complementa, não substitui, react-query.

**Rationale**:
- react-query provê cache, stale-while-revalidate, retry, dedup — essencial para dados estruturados
- WebSocket provê push de baixa latência — essencial para gráficos ao vivo
- Padrão `useLiveMetric`: query inicial → WS updates → fallback ao query no reconnect
- Backend já tem WebSocket Hub implementado com topic pub/sub

### ADR-003: 6 hubs, não 40+ páginas

**Decisão**: Consolidar a navegação em 6 hubs principais com drill-down.

**Rationale**:
- 26 itens de menu atuais causam cognitive overload
- Dashboards cross-domain (ex: Security mostra audit + API keys + kernel juntos)
- Drill-down preserva acesso a todas as features sem poluir a navegação principal
- Sidebar colapsável com submenu para acesso rápido a páginas internas

### ADR-004: Server Components como shell, Client Components como folhas

**Decisão**: Layouts e wrappers são RSC. Apenas componentes interativos são "use client".

**Rationale**:
- Aproveita streaming SSR do Next.js 15
- Reduz JS enviado ao cliente
- Layouts pré-renderizados no servidor
- Charts/forms/real-time hooks são client boundary natural

---

## 14. Dependências Novas (mínimas)

```json
{
  "dependencies": {
    "d3-scale": "^4.0.2",
    "d3-shape": "^3.2.0"
  }
}
```

**Total de novas dependências: 2** (~12KB gzip combinadas).

**Zero breaking changes** nas dependências existentes.

---

## 15. Riscos e Mitigações

| Risco | Probabilidade | Impacto | Mitigação |
|-------|-------------|---------|-----------|
| WebSocket Hub sobrecarregar com muitos subscribers | Baixa | Médio | Rate limiting no backend, buffer por connection (64 msg), drop para slow consumers já implementado |
| Bundle size crescer com novos charts | Baixa | Baixo | Tree-shaking do Recharts, lazy loading por rota, budget de 250KB |
| Migração de CSS quebrar temas existentes | Média | Médio | Novos tokens em arquivo separado, fallback para variáveis antigas, migração gradual |
| Complexidade de 6 hubs vs 40 páginas | Baixa | Baixo | Cada hub é composição de features existentes, não rewrite |
| Acessibilidade de gráficos complexos | Média | Médio | Screen-reader tables como fallback, testar com axe-core e VoiceOver |

---

## Apêndice A: Estrutura de Arquivos Proposta

```
web/src/
├── app/
│   ├── layout.tsx                          # RSC — Root layout
│   ├── globals.css                         # Tokens + base styles
│   ├── (auth)/login/page.tsx               # Login
│   └── (dashboard)/
│       ├── layout.tsx                      # RSC shell — sidebar + topbar
│       ├── command-center/
│       │   ├── page.tsx                    # RSC — composition
│       │   ├── system-vitals.tsx           # CC — WS real-time vitals
│       │   ├── request-metrics.tsx         # CC — WS + react-query
│       │   ├── agent-status.tsx            # CC — react-query + WS
│       │   └── quick-actions.tsx           # CC — mutations
│       ├── security/
│       │   ├── page.tsx
│       │   ├── threat-dashboard.tsx
│       │   ├── audit-viewer.tsx
│       │   ├── api-key-manager.tsx
│       │   └── kernel-emergency.tsx
│       ├── analytics/
│       │   ├── page.tsx
│       │   ├── token-usage-chart.tsx
│       │   ├── agent-performance.tsx
│       │   ├── provider-comparison.tsx
│       │   └── search-analytics.tsx
│       ├── orchestration/
│       │   ├── page.tsx
│       │   ├── execution-history.tsx
│       │   ├── workflow-designer.tsx
│       │   ├── trace-viewer.tsx
│       │   └── playground.tsx
│       ├── knowledge/
│       │   ├── page.tsx
│       │   ├── document-explorer.tsx
│       │   ├── epistemology-dashboard.tsx
│       │   ├── memory-browser.tsx
│       │   └── indexing-status.tsx
│       └── ai-tools/
│           ├── page.tsx
│           ├── agent-catalog.tsx
│           ├── skill-marketplace.tsx
│           ├── plugin-manager.tsx
│           └── agentbridge-monitor.tsx
├── components/
│   ├── shared/
│   │   ├── charts/
│   │   │   ├── chart-container.tsx         # ✅ Refinado
│   │   │   ├── bar-chart.tsx               # ✅ Refinado
│   │   │   ├── area-chart.tsx              # 🆕
│   │   │   ├── line-chart.tsx              # 🆕
│   │   │   ├── donut-chart.tsx             # 🆕
│   │   │   ├── sparkline.tsx               # 🆕
│   │   │   ├── heatmap-table.tsx           # 🆕
│   │   │   ├── radar-chart.tsx             # 🆕
│   │   │   ├── timeline-chart.tsx          # 🆕
│   │   │   ├── sankey-diagram.tsx          # 🆕
│   │   │   ├── stat-card-sparkline.tsx     # 🆕
│   │   │   ├── chart-tooltip.tsx           # ✅ Refinado
│   │   │   ├── chart-legend.tsx            # 🆕
│   │   │   └── types.ts                    # ✅ Expandido
│   │   └── layout/
│   │       ├── dashboard-grid.tsx          # 🆕 Grid system
│   │       ├── dashboard-row.tsx           # 🆕
│   │       └── dashboard-panel.tsx         # 🆕
│   └── ui/                                 # ✅ shadcn/ui (manter)
├── hooks/
│   ├── use-cosca-websocket.ts              # 🆕
│   ├── use-live-metric.ts                  # 🆕 React Query + WS pattern
│   └── use-responsive.ts                   # 🆕 Breakpoint hook unificado
├── lib/
│   ├── api.ts                              # ✅ Manter
│   ├── event-client/                       # ✅ Manter
│   ├── ws-client.ts                        # 🆕 WebSocket client singleton
│   └── design-tokens.ts                    # 🆕 Token constants
└── styles/
    └── design-tokens.css                   # 🆕 CSS custom properties
```

---

## Apêndice B: Definição de "Pronto" (Definition of Done)

Cada fase é considerada pronta quando:

1. **Todos os testes passam**: vitest (unit), Playwright (E2E), Storybook (visual)
2. **Bundle size**: < 250KB por rota (medido com @next/bundle-analyzer)
3. **Lighthouse**: Performance ≥ 90, Accessibility = 100
4. **WCAG 2.1 AA**: axe-core scan limpo, testado com VoiceOver/NVDA
5. **Responsivo**: Verificado em 5 breakpoints (320px, 768px, 1024px, 1440px, 1920px)
6. **Dark + Light**: Ambos os temas funcionais e com contraste verificado
7. **Alinhamento backend**: Cada endpoint do backend tem representação visual no frontend

---

> **Documento aprovado?** Aguardando revisão do Don e CTO.
> **Próximo passo**: Iniciar FASE 1 — Foundation, começando pelos design tokens e Command Center redesign.
