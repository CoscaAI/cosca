# Phase 1 MVP — Web Console Product Backlog

> **Status**: planning | **Owner**: Product Chief | **Sprint**: 4 weeks
> **Target**: 5-page MVP web console consuming the existing REST API
> **Dependency**: EPIC-006 (API Layer) REST endpoints must be functional

---

## Table of Contents

- [1. Epic Description](#1-epic-description)
- [2. Scope & Boundaries](#2-scope--boundaries)
- [3. User Stories](#3-user-stories)
  - [3.1 Dashboard](#31-dashboard-us-001-to-us-003)
  - [3.2 Knowledge Explorer](#32-knowledge-explorer-us-004-to-us-007)
  - [3.3 Memory Viewer](#33-memory-viewer-us-008-to-us-011)
  - [3.4 Runtime Monitor](#34-runtime-monitor-us-012-to-us-014)
  - [3.5 Settings](#35-settings-us-015-to-us-017)
- [4. API Contract](#4-api-contract)
- [5. Shared Components](#5-shared-components)
- [6. Route Map](#6-route-map)
- [7. Non-Functional Requirements](#7-non-functional-requirements)
- [8. Out of Scope (Explicitly)](#8-out-of-scope-explicitly)
- [9. Success Metrics](#9-success-metrics)
- [10. Weekly Sprint Plan](#10-weekly-sprint-plan)
- [11. Risks & Mitigations](#11-risks--mitigations)
- [12. Dependencies on New API Endpoints](#12-dependencies-on-new-api-endpoints)

---

## 1. Epic Description

### What Is the MVP Web Console?

The **Cosca Web Console** (Phase 1 MVP) is a read-only web frontend that provides visual access to the Cosca Platform. It surfaces the existing Knowledge Engine, Memory Engine, and Runtime subsystems through a browser-based interface. The console consumes the REST API already defined in the `api/rest/` package and documented in `api/rest/openapi.yaml`.

### Who Is It For?

**Primary persona**: Developers who use Cosca in their projects and want a visual alternative to the terminal for monitoring, searching, and inspecting their Cosca instance. They are already comfortable with the CLI but need a faster way to browse indexed knowledge, inspect memory records, and verify system health.

**Secondary persona**: Technical leads who need to understand system state at a glance before delegating work to agents (Phase 2).

### What Value Does It Deliver?

| Value | Current Gap | MVP Solution |
|-------|-------------|--------------|
| **Visual monitoring** | CLI-only (`cosca status`, `cosca health`) — hard to scan | Dashboard with health cards, component grid, live status badges |
| **Knowledge search** | CLI text output — hard to browse, no snippets | Search with facets, result cards with highlighted excerpts, stats sidebar |
| **Memory inspection** | CLI text output — no layer visualization | Layer browser, type filters, record detail cards |
| **System context** | Scattered across multiple commands | Single-page overview of runtime state, subsystem health, uptime |

### Success Statement

> A developer can open the web console and within 10 seconds understand: (1) whether Cosca is running and healthy, (2) how many documents are indexed, (3) how to search knowledge or browse memory, and (4) which subsystems are operational.

---

## 2. Scope & Boundaries

### In Scope (Phase 1 MVP)

| Category | Scope |
|----------|-------|
| **Pages** | Dashboard, Knowledge Explorer, Memory Viewer, Runtime Monitor, Settings — 5 pages total |
| **Operations** | Read-only: search, list, view, stats, status. No mutations (create/update/delete) except search queries |
| **API consumption** | `GET /v1/health`, `GET /v1/status`, `POST /v1/knowledge/search`, `GET /v1/knowledge/stats`, `GET /v1/memory/search`, `GET /v1/memory/stats`, plus 2 new endpoints |
| **Components** | Nav sidebar (from existing layout scaffold), stat cards, search bars, result lists, detail modals, status badges, empty states |
| **Themes** | Dark + light mode via `next-themes` (already configured) |
| **Responsiveness** | Mobile (320px), tablet (768px), desktop (1024px+) |
| **Accessibility** | WCAG 2.1 AA on all pages |

### Out of Scope (Explicitly)

See [Section 8](#8-out-of-scope-explicitly) for the complete list.

### Boundaries

- No backend changes required beyond 2 new GET endpoints (see Section 12)
- No new database tables or schema changes
- No authentication — the console assumes the REST API is on localhost or a trusted network
- No SSR/SSG reliance — all data is fetched client-side via React Query

---

## 3. User Stories

### 3.1 Dashboard (US-001 to US-003)

The Dashboard is the landing page (`/dashboard`). It provides an at-a-glance health summary and quick stats for the Cosca platform. It consumes `GET /v1/health`, `GET /v1/status`, `GET /v1/knowledge/stats`, and `GET /v1/memory/stats`.

---

#### US-001: System Health Overview

**As a** developer,  
**I want** to see system health at a glance with component status indicators,  
**so that** I know immediately whether Cosca is running properly without running CLI commands.

**Acceptance Criteria:**

- [ ] **Given** the runtime is healthy, **when** the dashboard loads, **then** a prominent health badge shows "Healthy" with a green indicator.
- [ ] **Given** the runtime is degraded, **when** the dashboard loads, **then** the health badge shows "Degraded" with an amber indicator and lists affected components.
- [ ] **Given** the runtime is unhealthy or unreachable, **when** the dashboard loads, **then** the health badge shows "Unhealthy" with a red indicator and a "Troubleshoot" link to the Runtime Monitor.
- [ ] **Error state**: When `GET /v1/health` returns a non-2xx response, display a "Connection Error" message with a retry button and the last known status timestamp.
- [ ] **Loading state**: Show skeleton placeholders (pulsing rectangles) for health badge and stat cards while API calls are in flight.
- [ ] **Empty state**: Not applicable — the health endpoint always returns a response.
- [ ] **Accessibility**: Health badge uses `role="status"` and `aria-live="polite"` so screen readers announce status changes. Color is not the sole indicator; text labels accompany every color.

**Definition of Done:**

- [ ] Unit tests for the health badge component (all 3 states + error)
- [ ] Component in Storybook with controls for health/state/healthy
- [ ] Dark + light theme renders correctly
- [ ] Responsive: badge resizes and stacks on mobile
- [ ] Lighthouse Accessibility audit score ≥ 95

---

#### US-002: Quick Stats Cards

**As a** developer,  
**I want** to see quick stats (indexed documents, memory records, plugins count, uptime),  
**so that** I understand system usage and scale at a glance.

**Acceptance Criteria:**

- [ ] **Given** the dashboard loads successfully, **when** all API calls resolve, **then** four stat cards are displayed: Knowledge Docs (from `GET /v1/knowledge/stats`), Memory Records (from `GET /v1/memory/stats`), Plugins (from `GET /v1/status` components), and Uptime (from `GET /v1/status`).
- [ ] **Given** a stat value is zero, **when** the stat card renders, **then** it displays "0" with the appropriate icon and a subdued color (not an error).
- [ ] **Given** a stat API call fails, **when** the card renders, **then** it displays "—" with a tooltip "Data unavailable" and a retry icon.
- [ ] **Loading state**: Each card independently shows a skeleton (pulsing number placeholder) while its specific API call is pending.
- [ ] **Empty state**: Cards show "0" for countable metrics (docs, records, plugins) and a dash for non-countable (uptime when not running).
- [ ] **Accessibility**: Each card has an `aria-label` describing the metric and its value. Cards are keyboard-navigable in a logical order.

**Definition of Done:**

- [ ] Unit tests for the StatCard component (all states)
- [ ] Each card variant in Storybook
- [ ] Dark + light theme
- [ ] Responsive: 2-column grid on tablet, 4-column on desktop
- [ ] Zero prop-type warnings in console

---

#### US-003: Provider Status Section

**As a** developer,  
**I want** to see which AI/embedding providers are available,  
**so that** I know which services are configured and can be used by agents.

**Acceptance Criteria:**

- [ ] **Given** providers are reported in the component map of `GET /v1/status`, **when** the dashboard renders, **then** a "Providers" section lists each provider with a status indicator (active/inactive/error) and its type (e.g., OpenAI, Anthropic, Local).
- [ ] **Given** no providers are configured, **when** the providers section renders, **then** it shows "No providers configured" with a link to the Settings page.
- [ ] **Given** provider status is unavailable (missing from response), **when** the section renders, **then** it shows a "Provider status unavailable" message.
- [ ] **Loading state**: Show skeleton rows (3-4 lines) for the provider list.
- [ ] **Accessibility**: Provider list uses a `<ul>` with `<li>` items. Status indicators use both color and text.

**Definition of Done:**

- [ ] Unit tests for provider status list component
- [ ] Component in Storybook with mock provider data
- [ ] Dark + light theme
- [ ] Responsive: full-width on mobile, fits within dashboard grid
- [ ] Provider badge uses accessible color contrast ratios

---

### 3.2 Knowledge Explorer (US-004 to US-007)

The Knowledge Explorer page (`/knowledge`) enables searching and browsing the indexed project knowledge base. It consumes `POST /v1/knowledge/search` and `GET /v1/knowledge/stats`.

---

#### US-004: Knowledge Search

**As a** developer,  
**I want** to search my project's knowledge base with a text query,  
**so that** I can find relevant documentation, code, and entities without using the CLI.

**Acceptance Criteria:**

- [ ] **Given** the user types a query and submits, **when** `POST /v1/knowledge/search` returns results, **then** results are displayed as cards with title, snippet (truncated to 200 chars with highlighting), score percentage, type badge, and file path.
- [ ] **Given** the search returns zero results, **when** results render, **then** an empty state shows "No results found for 'query'" with suggestions: "Try different keywords", "Check spelling", "Browse knowledge stats instead".
- [ ] **Given** the user submits an empty query, **when** the form validates, **then** the search button is disabled and a helper text says "Enter at least 2 characters".
- [ ] **Given** the search API returns an error, **when** the error is caught, **then** a toast notification (via Sonner) shows "Search failed: {message}" and results area shows the last successful results with a warning banner.
- [ ] **Loading state**: Show a spinner in the search bar and skeleton result cards while the API call is pending. Debounce aggressively (300ms) to avoid flicker on fast typists. Show query term in the loading indicator: "Searching for 'query'...".
- [ ] **Empty state** (before first search): Show a centered prompt: "Search your knowledge base" with an example query placeholder (e.g., "Try: authentication flow") and a keyboard shortcut hint (`Ctrl+K`).
- [ ] **Accessibility**: Search input has `role="searchbox"` and `aria-label="Search knowledge base"`. Results list uses `role="listbox"`. Screen reader announces result count on each search.

**Definition of Done:**

- [ ] Unit tests for search form, result card, empty state, error state
- [ ] Integration test: mock API, type query, verify results render
- [ ] Search bar + result cards in Storybook
- [ ] Dark + light theme (code snippets must be readable in both)
- [ ] Responsive: search bar full-width on mobile, results stack vertically
- [ ] Keyboard accessible: Enter to submit, Escape to clear, Tab through results

---

#### US-005: Search Facets and Explanation

**As a** developer,  
**I want** to see search facets (type, language) and result explanations,  
**so that** I understand *why* certain results were returned and can filter effectively.

**Acceptance Criteria:**

- [ ] **Given** search results include facets, **when** search completes, **then** a sidebar shows facet groups: "Type" (document, chunk, entity, code_block) and "Language" (from metadata) with counts per facet value. Clicking a facet value filters results client-side.
- [ ] **Given** a result has an explanation (score breakdown), **when** the user clicks "Why this result?", **then** a tooltip or popover shows the FTS5 score, vector similarity, graph boost, and combined score.
- [ ] **Given** no facets are returned (API returns empty facets object), **when** the sidebar renders, **then** the facets section is hidden (not shown as empty).
- [ ] **Loading state**: Facets sidebar shows skeleton chips while API is loading.
- [ ] **Accessibility**: Facet chips are toggle buttons with `aria-pressed`. Screen reader announces "Filter applied: Type = document" when a facet is selected.

**Definition of Done:**

- [ ] Unit tests for facet sidebar, explanation popover
- [ ] Components in Storybook with mock facet/explanation data
- [ ] Dark + light theme
- [ ] Responsive: facets collapse to a horizontal scrollable chip row on mobile, expand to sidebar on desktop
- [ ] Focus management: selecting a facet does not lose keyboard focus

---

#### US-006: Knowledge Stats Panel

**As a** developer,  
**I want** to view knowledge engine statistics (indexed docs, chunks, entities, vectors, graph nodes, DB size, last indexed time),  
**so that** I understand the size and freshness of the knowledge base.

**Acceptance Criteria:**

- [ ] **Given** the Knowledge Explorer page loads, **when** `GET /v1/knowledge/stats` returns data, **then** a stats panel at the top of the page (collapsible) shows: document count, chunk count, entity count, vector count, graph nodes, graph edges, database size (formatted as KB/MB), and last indexed timestamp (relative: "2h ago").
- [ ] **Given** the stats API returns a partial response (e.g., missing graph_stats), **when** the panel renders, **then** missing stats show "N/A" with no error state.
- [ ] **Given** the stats API fails, **when** the panel renders, **then** it shows a collapsed panel with a "Stats unavailable" message and a retry button.
- [ ] **Loading state**: Stats panel shows skeleton values.
- [ ] **Empty state**: All zeros (fresh install) — shown as "0" with a note: "No documents indexed yet. Use `cosca knowledge index` to start."
- [ ] **Accessibility**: Stats are presented in a `<dl>` (description list) with `<dt>` for labels and `<dd>` for values.

**Definition of Done:**

- [ ] Unit tests for stats panel component
- [ ] Component in Storybook with varied mock data (full, partial, empty)
- [ ] Dark + light theme
- [ ] Responsive: stats reflow from horizontal to stacked on mobile
- [ ] Collapsible state persists via `localStorage`

---

#### US-007: Search Pagination and Sorting

**As a** developer,  
**I want** to paginate through search results and sort by relevance or recency,  
**so that** I can explore large result sets without overwhelming the UI.

**Acceptance Criteria:**

- [ ] **Given** search returns more results than the page size (default 20), **when** results render, **then** pagination controls appear at the bottom showing "Page X of Y (Z total results)" with Previous/Next buttons.
- [ ] **Given** the user clicks "Next" or changes the sort order, **when** the API re-fetches, **then** the page scrolls to the top of results and the new results are announced to screen readers.
- [ ] **Given** there are 0 results or fewer results than page size, **when** results render, **then** pagination controls are hidden.
- [ ] **Given** the user is on page 1 and clicks "Previous", **when** the button is disabled, **then** it shows as disabled with `aria-disabled="true"`.
- [ ] **Loading state**: Pagination buttons show a subtle spinner while re-fetching.
- [ ] **Accessibility**: Pagination uses `<nav aria-label="Search results pagination">`. Current page is marked with `aria-current="page"`.

**Definition of Done:**

- [ ] Unit tests for pagination component (edge cases: page 1, last page, single page)
- [ ] Component in Storybook
- [ ] Dark + light theme
- [ ] Responsive: pagination compacts on mobile (shows "Page X of Y" only, no numbered buttons)

---

### 3.3 Memory Viewer (US-008 to US-011)

The Memory Viewer page (`/memory`) enables browsing and searching memory records across layers. It consumes `GET /v1/memory/search`, `GET /v1/memory/stats`, and the new `GET /v1/memory/layers`.

---

#### US-008: Memory Layer Browser

**As a** developer,  
**I want** to browse memory records by layer (Global, Workspace, Project, Session, Temp),  
**so that** I can inspect stored knowledge at the appropriate scope.

**Acceptance Criteria:**

- [ ] **Given** the Memory Viewer loads, **when** `GET /v1/memory/layers` returns layer data, **then** a tab bar or vertical sidebar shows the 5 memory layers, each with a record count badge. The Session layer is selected by default.
- [ ] **Given** the user selects a layer, **when** records are fetched for that layer, **then** records are displayed as cards showing type badge, truncated content (150 chars), priority indicator, created date (relative), and metadata tags.
- [ ] **Given** a layer has zero records, **when** the layer is selected, **then** an empty state shows: "No records in {layer} layer" with a description of what that layer is for (e.g., "Session layer stores ephemeral context for the current session").
- [ ] **Given** the layers API fails, **when** the page renders, **then** a static fallback shows the 5 layer tabs (hardcoded) with "—" for counts and an error banner: "Could not load layer stats."
- [ ] **Loading state**: Record cards show skeleton placeholders when switching layers.
- [ ] **Accessibility**: Layer tabs are in a `role="tablist"` with `role="tab"` and `aria-selected`. Record cards are keyboard-focusable.

**Definition of Done:**

- [ ] Unit tests for layer tabs, record cards, empty state
- [ ] Components in Storybook with mock layer and record data
- [ ] Dark + light theme
- [ ] Responsive: tabs scroll horizontally on mobile, switch to vertical on desktop
- [ ] Keyboard navigation: left/right arrows switch tabs

---

#### US-009: Memory Record Search

**As a** developer,  
**I want** to search memory records by keyword, type, and layer filters,  
**so that** I can find specific decisions, patterns, or bug reports.

**Acceptance Criteria:**

- [ ] **Given** the user types a query and optionally selects type filters (Decision, Pattern, Bug, Agent, Project, Architecture, Session) and layer filters, **when** `GET /v1/memory/search` is called, **then** filtered results are displayed as record cards.
- [ ] **Given** the search returns zero results, **when** results render, **then** an empty state with the active filters is shown: "No records matching '{query}' in {types} types" with a "Clear filters" button.
- [ ] **Given** no search query is entered but type/layer filters are active, **when** the search triggers, **then** results are fetched without a query string (browse mode) and sorted by recency.
- [ ] **Given** the search API returns an error, **when** the error is caught, **then** a toast shows the error message and the previous results remain visible.
- [ ] **Loading state**: Skeleton record cards while fetching. Filter chips show a subtle pulse.
- [ ] **Accessibility**: Filter checkboxes are labeled and grouped in `<fieldset>` with `<legend>`. Screen reader announces filter changes.

**Definition of Done:**

- [ ] Unit tests for search form, filter chips, record cards
- [ ] Integration test: mock search API, apply filters, verify request params
- [ ] Components in Storybook
- [ ] Dark + light theme
- [ ] Responsive: filters collapse to a drawer/sheet on mobile
- [ ] URL query params sync with search state (shareable search URLs)

---

#### US-010: Memory Stats Overview

**As a** developer,  
**I want** to view memory statistics per layer (record count, size),  
**so that** I understand memory usage and storage footprint.

**Acceptance Criteria:**

- [ ] **Given** the Memory Viewer loads, **when** `GET /v1/memory/stats` returns data, **then** a stats bar shows per-layer breakdown: layer name, record count, size (formatted), and a simple horizontal bar visualizing proportional size.
- [ ] **Given** a layer has 0 records, **when** the bar renders, **then** it shows "0 records" with an empty bar segment.
- [ ] **Given** the stats API fails, **when** the stats bar renders, **then** it shows "Stats unavailable" with a retry button.
- [ ] **Loading state**: Stats bar shows skeleton segments.
- [ ] **Empty state**: All zero — shows "No memory records stored yet" with a CLI tip.
- [ ] **Accessibility**: Stats bar is an accessible chart using `role="img"` with a text alternative describing the data.

**Definition of Done:**

- [ ] Unit tests for stats bar component
- [ ] Component in Storybook
- [ ] Dark + light theme
- [ ] Responsive: bars scale down proportionally on mobile
- [ ] Color contrast ratio ≥ 4.5:1 for bar segments

---

#### US-011: Memory Record Detail View

**As a** developer,  
**I want** to click a memory record to see its full content with metadata,  
**so that** I can read the complete record without truncation.

**Acceptance Criteria:**

- [ ] **Given** the user clicks a memory record card, **when** the detail view opens, **then** a modal or slide-out panel shows: full content (Markdown rendered or preformatted), type badge, layer badge, priority, created date, updated date, TTL, all metadata key-value pairs, and tags as chips.
- [ ] **Given** the record content is long (> 500 chars), **when** the detail view renders, **then** content is scrollable within the panel.
- [ ] **Given** the record has a TTL, **when** the detail view renders, **then** TTL is shown as a human-readable duration (e.g., "24 hours" rather than "24h0m0s").
- [ ] **Given** the user presses Escape or clicks the backdrop, **when** the event fires, **then** the detail view closes and focus returns to the triggering record card.
- [ ] **Loading state**: Not applicable — record data is already loaded in the card list.
- [ ] **Empty state**: Not applicable — detail view only opens from an existing record.
- [ ] **Accessibility**: Modal traps focus. Opened with `aria-modal="true"` and `aria-labelledby`. Close button labeled "Close record detail".

**Definition of Done:**

- [ ] Unit tests for detail modal/panel component
- [ ] Component in Storybook with mock record data (short, long, with/without metadata)
- [ ] Dark + light theme
- [ ] Responsive: full-screen panel on mobile, side panel on desktop
- [ ] Keyboard: Escape closes, Tab cycles within modal

---

### 3.4 Runtime Monitor (US-012 to US-014)

The Runtime Monitor page (`/runtime`) provides detailed system status, state machine visualization, and subsystem health. It consumes `GET /v1/status` and `GET /v1/health`.

---

#### US-012: Runtime Status Overview

**As a** developer,  
**I want** to see runtime state, health, uptime, and version,  
**so that** I know the exact status of the Cosca runtime.

**Acceptance Criteria:**

- [ ] **Given** the Runtime Monitor loads, **when** `GET /v1/status` returns data, **then** a status header shows: state badge (e.g., "Running" with green), health badge (e.g., "Healthy"), uptime (e.g., "3h 42m"), version (e.g., "0.1.0"), and started-at timestamp (absolute + relative).
- [ ] **Given** the runtime is in an error state, **when** the status renders, **then** the status header shows a prominent error banner with the error message and a "View Details" link.
- [ ] **Given** the runtime is in a transitional state (initializing, stopping, recovering), **when** the status renders, **then** a progress indicator is shown with the current state name and an animated pulse.
- [ ] **Given** the status API fails (connection refused), **when** the page renders, **then** it shows a full-page error: "Cannot connect to Cosca Runtime" with the API base URL displayed and a retry button.
- [ ] **Loading state**: Status header shows skeleton badges.
- [ ] **Accessibility**: State and health badges use `role="status"`. Screen reader announces "Runtime state: running. Health: healthy."

**Definition of Done:**

- [ ] Unit tests for status header (all 8 runtime states mapped to UI)
- [ ] Component in Storybook with controls for state/health/uptime
- [ ] Dark + light theme
- [ ] Responsive: status badges stack vertically on mobile
- [ ] Color indicators pass AA contrast with text labels

---

#### US-013: Subsystem Health Grid

**As a** developer,  
**I want** to see the health of each subsystem (knowledge, memory, plugins, discovery, cache, watcher, editors),  
**so that** I can diagnose which subsystem is causing issues.

**Acceptance Criteria:**

- [ ] **Given** `GET /v1/status` includes a components map, **when** the Runtime Monitor renders, **then** a grid of subsystem cards shows each component name, its status (healthy/degraded/unhealthy/unknown), uptime, and status message.
- [ ] **Given** a component is healthy, **when** its card renders, **then** it shows a green check icon and "Healthy".
- [ ] **Given** a component is degraded, **when** its card renders, **then** it shows an amber warning icon and the warning message from the API.
- [ ] **Given** a component is unhealthy, **when** its card renders, **then** it shows a red error icon with the error message.
- [ ] **Given** the components map is empty or missing, **when** the grid renders, **then** it shows "No subsystem data available. Ensure the runtime is running and reporting component status."
- [ ] **Loading state**: Cards show skeleton placeholders.
- [ ] **Accessibility**: Each card has `aria-label="{name}: {status}"`. Cards are in a `<section>` with `aria-label="Subsystem health"`.

**Definition of Done:**

- [ ] Unit tests for subsystem card component (healthy, degraded, unhealthy states)
- [ ] Component grid in Storybook with varied mock data
- [ ] Dark + light theme
- [ ] Responsive: 1 column on mobile, 2 on tablet, 3-4 on desktop
- [ ] Status colors have text fallbacks for colorblind users

---

#### US-014: State Machine Visualization (Static)

**As a** developer,  
**I want** to see a visual representation of the Cosca state machine with the current state highlighted,  
**so that** I understand the runtime lifecycle and where the system is in it.

**Acceptance Criteria:**

- [ ] **Given** the Runtime Monitor loads, **when** the current state is known, **then** a static diagram shows the 8 runtime states (uninitialized → initializing → ready → running → stopping → stopped; plus error and recovering) with the current state highlighted (filled with brand color) and valid transitions shown as arrows.
- [ ] **Given** the current state is "running", **when** the diagram renders, **then** the "running" node is filled with the success color, all other nodes are outlined, and the "stopping" arrow is shown as the next valid transition.
- [ ] **Given** the current state is "error", **when** the diagram renders, **then** the "error" node is filled with the error color, and the "recovering" arrow is shown.
- [ ] **Error state**: If the state is unknown, highlight nothing and show a note: "Current state unknown."
- [ ] **Loading state**: Diagram shows skeleton nodes (empty circles) until the state is known.
- [ ] **Accessibility**: Diagram is an SVG with `<title>` and `<desc>` elements. State nodes are keyboard-focusable with tooltips describing the state.

**Definition of Done:**

- [ ] Unit tests for state machine mapping logic (state → node highlight, valid transitions)
- [ ] Component in Storybook with control for current state
- [ ] Dark + light theme (diagram colors adapt)
- [ ] Responsive: diagram scales down on mobile using SVG viewBox
- [ ] Diagram is static (not animated) for Phase 1 — animation is Phase 4

---

### 3.5 Settings (US-015 to US-017)

The Settings page (`/settings`) displays the current Cosca configuration and installed plugins. Initially read-only. It consumes the new `GET /v1/status/config` endpoint and existing `GET /v1/status`.

---

#### US-015: Configuration Viewer

**As a** developer,  
**I want** to view the current Cosca configuration as structured data,  
**so that** I understand how the system is configured without opening config files.

**Acceptance Criteria:**

- [ ] **Given** the Settings page loads, **when** `GET /v1/status/config` returns configuration data, **then** configuration is displayed in organized sections: General (version, data directory, log level), API (REST port, API key masked, MCP enabled), Storage (database path, cache settings), and any provider-specific config (with API keys masked as `****`).
- [ ] **Given** a config value is sensitive (contains "key", "secret", "token", "password"), **when** the value renders, **then** it is displayed as `••••••••` with a toggle button to reveal/hide.
- [ ] **Given** the config API fails or the endpoint is not available (501 Not Implemented), **when** the page renders, **then** it shows a message: "Configuration viewer is not available. Use `cosca config get` to view settings."
- [ ] **Loading state**: Config sections show skeleton key-value rows.
- [ ] **Empty state**: If config is empty (no custom config), show default config values with a note: "Using default configuration."
- [ ] **Accessibility**: Config sections use `<dl>` structure. Sensitive toggles have `aria-label="Reveal API key"` and `aria-pressed`.

**Definition of Done:**

- [ ] Unit tests for config viewer, sensitive value masking
- [ ] Component in Storybook with mock config data
- [ ] Dark + light theme
- [ ] Responsive: config sections stack vertically
- [ ] Copy-to-clipboard button on non-sensitive values

---

#### US-016: Installed Plugins List

**As a** developer,  
**I want** to see a list of installed plugins with their status, version, and runtime,  
**so that** I know what extensions are available in the system.

**Acceptance Criteria:**

- [ ] **Given** plugins data is available in `GET /v1/status` components, **when** the Settings page renders, **then** a "Plugins" section shows each plugin with: name, version, status (active/inactive/error), runtime type (Go/WASM/External/SharedLib), and a brief description.
- [ ] **Given** no plugins are installed, **when** the section renders, **then** it shows "No plugins installed" with a hint: "Use `cosca plugin install` to add plugins."
- [ ] **Given** a plugin has an error status, **when** its card renders, **then** it shows the error message in red text beneath the plugin name.
- [ ] **Loading state**: Plugin list shows skeleton rows.
- [ ] **Accessibility**: Plugin list is a `<ul>`. Each item uses `aria-label="{name}: {status}"`.

**Definition of Done:**

- [ ] Unit tests for plugin list component (active, inactive, error states)
- [ ] Component in Storybook with mock plugin data
- [ ] Dark + light theme
- [ ] Responsive: plugin cards stack vertically
- [ ] Status badges use accessible color contrast

---

#### US-017: System Information Footer

**As a** developer,  
**I want** to see system-level information (Go version, OS, architecture, build info),  
**so that** I can include accurate environment details when reporting issues.

**Acceptance Criteria:**

- [ ] **Given** the Settings page loads, **when** `GET /v1/status` returns runtime info, **then** a "System Information" section at the bottom shows: Cosca version, Go runtime version, OS/Arch, build commit hash (if available), and build timestamp.
- [ ] **Given** any system info field is missing from the API, **when** the section renders, **then** the missing field shows "Unknown".
- [ ] **Loading state**: System info shows skeleton rows.
- [ ] **Accessibility**: System info uses a `<dl>` structure. Values can be selected and copied.

**Definition of Done:**

- [ ] Unit tests for system info section
- [ ] Component in Storybook
- [ ] Dark + light theme
- [ ] Responsive: simple key-value layout, stacks on mobile
- [ ] Copy-to-clipboard on version/commit fields

---

## 4. API Contract

Each page consumes specific REST API endpoints. The table below maps pages to endpoints and identifies which are existing and which are new.

### Endpoint-to-Page Matrix

| Page | Endpoint | Method | Status | Purpose |
|------|----------|--------|--------|---------|
| **Dashboard** | `/v1/health` | `GET` | ✅ Existing | Health check for overall health badge |
| **Dashboard** | `/v1/status` | `GET` | ✅ Existing | Runtime state, components, providers, uptime |
| **Dashboard** | `/v1/knowledge/stats` | `GET` | ✅ Existing | Document count for stat cards |
| **Dashboard** | `/v1/memory/stats` | `GET` | ✅ Existing | Record count for stat cards |
| **Knowledge** | `/v1/knowledge/search` | `POST` | ✅ Existing | Hybrid search with facets |
| **Knowledge** | `/v1/knowledge/stats` | `GET` | ✅ Existing | Engine statistics and graph metadata |
| **Knowledge** | `/v1/knowledge/index` | `POST` | ✅ Existing | *Not consumed by MVP (read-only)* |
| **Memory** | `/v1/memory/search` | `GET` | ✅ Existing | Search records with type/layer filters |
| **Memory** | `/v1/memory/stats` | `GET` | ✅ Existing | Per-layer record counts and sizes |
| **Memory** | `/v1/memory/layers` | `GET` | 🆕 New | List all layers with metadata (see Section 12) |
| **Runtime** | `/v1/status` | `GET` | ✅ Existing | Full runtime status and component map |
| **Runtime** | `/v1/health` | `GET` | ✅ Existing | Simple health boolean for monitoring |
| **Settings** | `/v1/status/config` | `GET` | 🆕 New | Current configuration (see Section 12) |
| **Settings** | `/v1/status` | `GET` | ✅ Existing | Plugin list and system info |

### Response Type Summaries

These are mapped from the OpenAPI spec at `api/rest/openapi.yaml`:

#### `GET /v1/health` → `RuntimeHealthResponse`

```typescript
interface HealthResponse {
  healthy: boolean;
  warnings: string[];
}
```

#### `GET /v1/status` → `RuntimeStatusResponse`

```typescript
interface StatusResponse {
  state: "uninitialized" | "initializing" | "ready" | "running" | "stopping" | "stopped" | "error" | "recovering";
  health: "unknown" | "healthy" | "degraded" | "unhealthy";
  uptime: string;        // e.g., "3h42m15s"
  version: string;
  components: Record<string, {
    name: string;
    status: string;
    uptime: string;
    message: string;
  }>;
}
```

#### `POST /v1/knowledge/search` → `KnowledgeSearchResponse`

```typescript
interface KnowledgeSearchRequest {
  query: string;
  limit?: number;        // default 20
  offset?: number;       // default 0
  types?: string[];      // document, chunk, entity, code_block
  path_filter?: string;
  enable_facets?: boolean;
}

interface KnowledgeSearchResponse {
  results: SearchResult[];
  total: number;
  duration_ms: number;
  facets?: Record<string, Record<string, number>>;
}

interface SearchResult {
  id: string;
  title: string;
  snippet: string;
  score: number;
  type: string;
  path: string;
}
```

#### `GET /v1/knowledge/stats` → `KnowledgeStatsResponse`

```typescript
interface KnowledgeStatsResponse {
  document_count: number;
  chunk_count: number;
  entity_count: number;
  vector_count: number;
  db_size_bytes: number;
  graph_stats?: {
    nodes: number;
    edges: number;
    density: number;
    components: number;
    is_empty: boolean;
  };
  last_indexed: string;  // ISO 8601
}
```

#### `GET /v1/memory/search` → `MemorySearchResponse`

```typescript
interface MemorySearchResponse {
  records: MemoryRecord[];
  total: number;
}

interface MemoryRecord {
  id: string;
  type: string;          // decision, pattern, bug, agent, project, architecture, session
  layer: string;         // global, workspace, project, session, temp
  content: string;
  priority: number;
  created_at: string;
  metadata: Record<string, string>;
}
```

#### `GET /v1/memory/stats` → `MemoryStatsResponse`

```typescript
interface MemoryStatsResponse {
  layers: Record<string, {
    record_count: number;
    size_bytes: number;
  }>;
}
```

---

## 5. Shared Components

These components are used across multiple pages and should be built first (Week 1):

| Component | Used By | Description |
|-----------|---------|-------------|
| `StatCard` | Dashboard, Knowledge, Memory | Metric display with icon, value, trend |
| `StatusBadge` | Dashboard, Runtime, Settings | Color-coded pill showing status text |
| `Skeleton` | All pages | Pulsing placeholder for loading states |
| `EmptyState` | All pages | Icon + message + optional action button |
| `ErrorBanner` | All pages | Dismissible error message with retry |
| `SearchBar` | Knowledge, Memory | Text input with submit, clear, keyboard shortcut |
| `RecordCard` | Knowledge, Memory | Search/browse result with title, snippet, metadata |
| `FilterChip` | Knowledge, Memory | Toggleable filter with count badge |
| `Pagination` | Knowledge, Memory | Page controls with total count |
| `DetailPanel` | Memory | Slide-out panel for full record view |
| `ConfigKeyValue` | Settings | Key-value row with copy and mask support |
| `SubsystemCard` | Dashboard, Runtime | Component health card with status icon |
| `SectionHeader` | All pages | Page section title with optional action |

All shared components must be in Storybook with dark + light theme variants.

---

## 6. Route Map

| Route | Page Component | Title | Nav Label | Initial Nav State |
|-------|---------------|-------|-----------|-------------------|
| `/` | Redirect to `/dashboard` | — | — | — |
| `/dashboard` | `DashboardPage` | Dashboard | Dashboard | ✅ Enabled |
| `/knowledge` | `KnowledgePage` | Knowledge Explorer | Knowledge | Enable (was disabled) |
| `/memory` | `MemoryPage` | Memory Viewer | Memory | Enable (was disabled) |
| `/runtime` | `RuntimePage` | Runtime Monitor | Runtime | 🆕 Add to nav |
| `/settings` | `SettingsPage` | Settings | Settings | Enable (was disabled) |

**Note**: The existing `NAV_ITEMS` in `web/src/lib/constants.ts` has Knowledge, Memory, and Settings marked as `disabled: true`. Phase 1 removes those flags and adds a Runtime entry. Plugins remains disabled (Phase 2 Agent Management).

---

## 7. Non-Functional Requirements

### Performance

| Metric | Target | Measured By |
|--------|--------|-------------|
| **LCP** (Largest Contentful Paint) | < 2.0s | Lighthouse |
| **TTI** (Time to Interactive) | < 3.0s | Lighthouse |
| **CLS** (Cumulative Layout Shift) | < 0.1 | Lighthouse |
| **Initial JS bundle** | < 200 KB (gzipped) | `next build` output |
| **API response (p95)** | < 500ms | Backend benchmark |
| **Search debounce** | 300ms | UX guideline |
| **Skeleton-to-content switch** | < 100ms flicker | Perceived performance |

### Accessibility (WCAG 2.1 AA)

| Criterion | Requirement |
|-----------|-------------|
| **1.1.1 Non-text Content** | All icons have `aria-label` or text alternative |
| **1.3.1 Info and Relationships** | Semantic HTML: `<nav>`, `<main>`, `<section>`, `<dl>`, `<ul>` |
| **1.4.1 Use of Color** | Color never the sole indicator; text labels accompany status colors |
| **1.4.3 Contrast (Minimum)** | All text ≥ 4.5:1 contrast ratio against background |
| **2.1.1 Keyboard** | All interactive elements reachable and operable via keyboard |
| **2.4.3 Focus Order** | Logical tab order; focus visible (ring) on all interactive elements |
| **2.4.7 Focus Visible** | 2px outline on focused elements, no `outline: none` without replacement |
| **3.3.2 Labels or Instructions** | All form inputs have visible labels or `aria-label` |
| **4.1.2 Name, Role, Value** | Custom components expose correct ARIA roles and states |
| **4.1.3 Status Messages** | Dynamic content changes use `role="status"` or `aria-live` |

### Responsive Breakpoints

| Breakpoint | Width | Layout Behavior |
|------------|-------|-----------------|
| **Mobile** | 320px – 767px | Single column, collapsible sidebar (hamburger), stacked cards, bottom sheet for filters |
| **Tablet** | 768px – 1023px | 2-column grid, persistent sidebar (collapsed), filters as horizontal chips |
| **Desktop** | 1024px+ | Multi-column grid, full sidebar, side panels for detail views |

### Theme

| Requirement | Implementation |
|-------------|---------------|
| Dark mode | Default for first visit. Full coverage of all pages. Colors from Tailwind config. |
| Light mode | Toggle in sidebar footer. Full coverage. |
| System preference | Respects `prefers-color-scheme` via `next-themes`. Persisted in `localStorage`. |
| Code blocks | Monospace font. Readable syntax highlighting in both themes (no theme-breaking inline styles). |

### Browser Support

| Browser | Minimum Version |
|---------|-----------------|
| Chrome | 110+ |
| Firefox | 115+ |
| Safari | 16.4+ |
| Edge | 110+ |

### Observability

- **No** console errors in production build
- **No** React hydration warnings
- API errors logged to console with `[Cosca:API]` prefix in development
- Empty/warning states logged with `[Cosca:State]` prefix in development

---

## 8. Out of Scope (Explicitly)

These features are **not** part of Phase 1 MVP. They are deferred to later phases:

### Phase 2 — Agent Management
- Agent list, agent run, agent detail pages
- Skill list, skill info
- Prompt list, prompt create
- Workflow list, workflow run
- Plugin management UI (install, enable, disable)

### Phase 3 — Platform Features
- Authentication and login (OIDC/OAuth2)
- Multi-tenant workspace switching
- Command palette (`Ctrl+K` global search)
- Global cross-subsystem search
- User preferences and profile
- RBAC (role-based access control UI)

### Phase 4 — Advanced Analytics
- Charts and graphs (time-series metrics, trends)
- Animated state machine transitions
- Search analytics (popular queries, zero-result queries)
- Performance dashboards (latency histograms)

### Never in MVP
- Write operations (create/index/store/delete) through the web console
- Real-time WebSocket updates (polling only via React Query `refetchInterval`)
- Offline support or PWA
- Internationalization (i18n)
- Email/push notifications
- Audit log viewer
- Plugin registry browser
- Cloud sync UI

---

## 9. Success Metrics

### Quantitative

| Metric | Target | Measurement |
|--------|--------|-------------|
| **User stories passing acceptance** | 100% (17/17) | Manual QA + automated test pass |
| **Console errors** | 0 | Browser DevTools on all pages |
| **WCAG AA compliance** | 100% of pages | axe-core automated scan + manual keyboard audit |
| **Lighthouse Performance** | > 90 | Lighthouse CI report on all 5 pages |
| **Lighthouse Accessibility** | > 95 | Lighthouse CI report on all 5 pages |
| **Lighthouse Best Practices** | > 90 | Lighthouse CI report on all 5 pages |
| **Lighthouse SEO** | > 90 | Lighthouse CI report on all 5 pages |
| **Responsive pass** | All 3 breakpoints | Visual check on mobile/tablet/desktop viewports |
| **Theme completeness** | 100% | Visual check: all pages in dark and light mode |
| **Storybook coverage** | All shared + page components | `npx storybook build` succeeds with no errors |
| **Unit test coverage** | ≥ 80% (statements) | `vitest --coverage` |
| **TypeScript errors** | 0 | `tsc --noEmit` |
| **ESLint errors** | 0 | `next lint` |
| **API mock parity** | All endpoints mocked in tests | MSW handlers for all 7 consumed endpoints |

### Qualitative

| Signal | How to Observe |
|--------|---------------|
| A new developer can find the search bar within 3 seconds of landing on Knowledge page | User testing |
| Runtime status is comprehensible at a glance (no CLI knowledge needed) | User testing |
| Error states are clear and actionable (not raw JSON or stack traces) | Review all error states |
| Empty states guide the user to next steps | Review all empty states |

---

## 10. Weekly Sprint Plan

### Week 1: Foundation + Dashboard

**Goal**: Shared component library + Dashboard page fully functional.

| Day | Deliverables |
|-----|-------------|
| 1-2 | `StatCard`, `StatusBadge`, `Skeleton`, `EmptyState`, `ErrorBanner` components in Storybook. API client typed with all response interfaces. MSW handlers for `/v1/health`, `/v1/status`, `/v1/knowledge/stats`, `/v1/memory/stats`. |
| 3-4 | Dashboard page with Health Overview (US-001), Quick Stats Cards (US-002). |
| 5 | Provider Status Section (US-003). Dashboard integration tests. Week 1 review. |

**DoD**: Dashboard page passes all 3 user stories. All 6 shared components in Storybook. Dark + light theme verified.

---

### Week 2: Knowledge Explorer

**Goal**: Full Knowledge Explorer page with search, stats, and pagination.

| Day | Deliverables |
|-----|-------------|
| 1-2 | `SearchBar`, `RecordCard` (knowledge variant), `FilterChip`, `Pagination` components. MSW handler for `POST /v1/knowledge/search`. |
| 3 | Knowledge Search (US-004) with empty/loading/error states. |
| 4 | Search Facets + Explanation (US-005). Knowledge Stats Panel (US-006). |
| 5 | Search Pagination (US-007). Integration tests. Week 2 review. |

**DoD**: Knowledge Explorer page passes all 4 user stories. All knowledge-specific components in Storybook. Debounced search working.

---

### Week 3: Memory Viewer

**Goal**: Full Memory Viewer page with layer browser, search, stats, and detail view.

| Day | Deliverables |
|-----|-------------|
| 1 | `DetailPanel` component. MSW handlers for `GET /v1/memory/search`, `GET /v1/memory/stats`, `GET /v1/memory/layers`. |
| 2 | Memory Layer Browser (US-008). Memory Record Search (US-009). |
| 3 | Memory Stats Overview (US-010). |
| 4 | Memory Record Detail View (US-011). Integration tests. |
| 5 | Settings page stubs (US-015, US-016, US-017). Week 3 review. |

**DoD**: Memory Viewer page passes all 4 user stories. Layer tab switching works. Detail modal accessible.

---

### Week 4: Runtime Monitor + Settings + Polish

**Goal**: Runtime Monitor, Settings completion, cross-page polish, final QA.

| Day | Deliverables |
|-----|-------------|
| 1 | Runtime Status Overview (US-012). Subsystem Health Grid (US-013). |
| 2 | State Machine Visualization (US-014). Runtime integration tests. |
| 3 | Settings: Configuration Viewer (US-015), Plugin List (US-016), System Info (US-017). MSW handler for `GET /v1/status/config`. |
| 4 | Cross-page QA: all error/loading/empty states verified. Lighthouse audits on all pages. Accessibility audit (axe-core + keyboard). |
| 5 | Bug fixes from QA. Storybook deployment. Final review and sign-off. |

**DoD**: All 17 user stories pass acceptance. All quality gates met. PR ready for merge.

---

## 11. Risks & Mitigations

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| Two new API endpoints (`/v1/memory/layers`, `/v1/status/config`) are not built in time | Medium | High — Memory Viewer detail and Settings config viewer blocked | Build MSW mocks first; UI can function with mock data. Memory Viewer can use hardcoded layer list. Settings can show "Coming soon" if endpoint unavailable. |
| REST API is unstable or returns unexpected shapes | Medium | Medium — UI shows incorrect data or crashes | All API responses validated with Zod schemas before rendering. Graceful degradation: if a field is missing, show "—" instead of crashing. |
| Search performance is slow for large knowledge bases (> 100K docs) | Low | Medium — poor UX on Knowledge Explorer | Client-side debouncing (300ms), `AbortController` for cancellation, loading states keep UI responsive, pagination limits rendered results |
| Mobile responsiveness is difficult for data-dense pages | Medium | Low — some pages are inherently desktop-oriented | Prioritize desktop-first design. Mobile view shows simplified layouts. Settings and Runtime Monitor are reference pages — acceptable to require desktop. |
| shadcn/ui components have theme bugs | Low | Medium — visual inconsistencies | All components tested in both themes in Storybook. Tailwind `dark:` variants used consistently. |
| Team velocity is slower than estimated | Medium | High — not all 17 stories complete in 4 weeks | User stories are prioritized. Must-have: Dashboard (US-001–003), Knowledge Search (US-004), Runtime Status (US-012). Nice-to-have: State Machine Viz (US-014), Settings (US-015–017). Defer lower-priority stories to Week 5 if needed. |

---

## 12. Dependencies on New API Endpoints

Two new REST API endpoints are required for Phase 1 MVP. These are simple read-only GET endpoints that the backend team should implement before Week 3.

### `GET /v1/memory/layers`

**Purpose**: List all available memory layers with metadata for the Memory Viewer layer browser (US-008).

**Request**: None (no parameters).

**Response**:

```json
{
  "layers": [
    {
      "name": "global",
      "description": "Cross-project patterns and preferences",
      "priority": 25,
      "persistence": "permanent",
      "record_count": 12
    },
    {
      "name": "workspace",
      "description": "Workspace conventions and shared decisions",
      "priority": 50,
      "persistence": "permanent",
      "record_count": 5
    },
    {
      "name": "project",
      "description": "Project-specific features, modules, architecture",
      "priority": 75,
      "persistence": "persistent",
      "record_count": 47
    },
    {
      "name": "session",
      "description": "Active context and current decisions",
      "priority": 100,
      "persistence": "ephemeral",
      "record_count": 3
    },
    {
      "name": "temp",
      "description": "Scratch data with short TTL",
      "priority": 10,
      "persistence": "ephemeral",
      "record_count": 0
    }
  ]
}
```

**Fallback**: If this endpoint is not available by Week 3, the Memory Viewer will use a hardcoded list of the 5 layers with `—` for record counts, fetched from `GET /v1/memory/stats` instead.

### `GET /v1/status/config`

**Purpose**: Return the current Cosca configuration for the Settings page (US-015).

**Request**: None (no parameters).

**Response**:

```json
{
  "general": {
    "version": "0.1.0",
    "data_directory": "/home/user/.cosca",
    "log_level": "info",
    "config_path": "/home/user/.cosca/config.yaml"
  },
  "api": {
    "rest_enabled": true,
    "rest_port": 8080,
    "api_key_set": true,
    "tls_enabled": false,
    "mcp_enabled": true,
    "mcp_port": 14120
  },
  "storage": {
    "database_path": "/home/user/.cosca/knowledge.db",
    "db_size_bytes": 15728640,
    "cache_enabled": true,
    "cache_ttl": "1h",
    "auto_sync": true
  },
  "providers": {
    "openai": { "configured": true, "model": "gpt-4o" },
    "anthropic": { "configured": false, "model": null },
    "local": { "configured": false, "model": null }
  }
}
```

**Security note**: API keys must NEVER be returned. The `api_key_set` field indicates only whether a key is configured (boolean). Provider configs indicate `configured: true/false` but never expose the key itself.

**Fallback**: If this endpoint is not available by Week 4, the Settings page will show "Configuration viewer coming soon. Use `cosca config get` to view settings." The Plugin List (US-016) and System Info (US-017) are not blocked since they consume `GET /v1/status`.

---

## Appendix A: Acceptance Criteria Checklist

Use this checklist during the final QA pass (Week 4, Day 4).

### All Pages

- [ ] Skeleton loading state rendered before data arrives
- [ ] Error state shown when API call fails (not white screen)
- [ ] Empty state shown when data is empty (not broken layout)
- [ ] Dark theme: all text readable, no hardcoded white backgrounds
- [ ] Light theme: all text readable, no white-on-white elements
- [ ] Mobile: no horizontal scroll, no overlapping elements
- [ ] Tablet: 2-column layouts where specified
- [ ] Desktop: full layout, no wasted space
- [ ] Keyboard: Tab through all interactive elements, Enter/Space to activate, Escape to close
- [ ] Screen reader: page has `<h1>`, regions have labels, dynamic content announced
- [ ] No `<a>` without `href`, no `<button>` without type, no `<img>` without `alt`
- [ ] All interactive elements have visible focus indicators

### Dashboard

- [ ] Health badge shows correct color + text for healthy/degraded/unhealthy
- [ ] Stat cards show live data from API
- [ ] Provider list shows available providers

### Knowledge Explorer

- [ ] Search bar submits on Enter, clears on Escape
- [ ] Results show title, snippet, score %, type badge, path
- [ ] Empty query disables search button
- [ ] Facets appear when API returns them, hidden when not
- [ ] Pagination shows correct page/total
- [ ] Stats panel shows all fields or "N/A"

### Memory Viewer

- [ ] Layer tabs switch content without page reload
- [ ] Records show type badge, content preview, metadata
- [ ] Filters work as checkboxes
- [ ] Detail modal opens/closes on click/Escape
- [ ] Stats bar shows per-layer breakdown

### Runtime Monitor

- [ ] Status header shows state, health, uptime, version
- [ ] Subsystem cards show individual health
- [ ] State machine diagram highlights current state

### Settings

- [ ] Config sections organized and readable
- [ ] Sensitive values masked with toggle
- [ ] Plugin list shows name, version, status, runtime
- [ ] System info shows version and environment

---

## Appendix B: Storybook Catalog

All shared components should be registered. Page-level components should be organized by page.

```
📚 Shared
  ├── StatCard
  ├── StatusBadge
  ├── Skeleton
  ├── EmptyState
  ├── ErrorBanner
  ├── SearchBar
  ├── RecordCard
  ├── FilterChip
  ├── Pagination
  ├── DetailPanel
  ├── ConfigKeyValue
  ├── SubsystemCard
  └── SectionHeader

📚 Dashboard
  ├── HealthOverview
  ├── QuickStats
  └── ProviderStatus

📚 Knowledge
  ├── KnowledgeSearchForm
  ├── KnowledgeResultCard
  ├── KnowledgeFacets
  ├── KnowledgeStatsPanel
  └── KnowledgePagination

📚 Memory
  ├── MemoryLayerTabs
  ├── MemoryRecordCard
  ├── MemoryFilters
  ├── MemoryStatsBar
  └── MemoryDetailPanel

📚 Runtime
  ├── RuntimeStatusHeader
  ├── SubsystemGrid
  └── StateMachineDiagram

📚 Settings
  ├── ConfigViewer
  ├── PluginList
  └── SystemInfo
```

---

> **Next Steps**: This backlog is now the source of truth for Phase 1 implementation.
> - **CTO**: Implement `GET /v1/memory/layers` and `GET /v1/status/config` before Week 3.
> - **UI/UX Chief**: Create wireframes for all 5 pages (coordinate with Product Chief).
> - **Frontend Chief**: Begin Week 1 sprint — shared components + Dashboard.
> - **QA Chief**: Prepare test plans aligned with acceptance criteria and the checklist in Appendix A.

---

> **Related Documents**:
> - [Strategic Roadmap](./README.md) — Overall platform roadmap and v1.0/v2.0 epics
> - [API Reference](../api-reference/overview.md) — REST API specification and schemas
> - [Architecture Overview](../architecture/overview.md) — System architecture and subsystems
> - [Memory Engine Overview](../memory/overview.md) — Memory layers, types, and storage
> - [Knowledge Engine Overview](../knowledge/overview.md) — Search pipeline and indexing
