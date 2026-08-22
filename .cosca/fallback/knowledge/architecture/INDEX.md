# Architecture Knowledge Domain

> **Category**: Architecture | **Version**: 1.0.0 | **Owner**: Cosca Kernel | **Last Updated**: 2026-07-29

## Purpose

System architecture documentation — the definitive source of truth for Cosca's design, decisions, and evolution plans. This domain captures both the "as-is" architecture (what exists) and the "to-be" architecture (what is planned).

## Directory Structure

```
architecture/
├── adr/                              (15 ADRs + 1 audit)
├── system-architecture-overview.md   (System architecture overview)
├── database-architecture.md          (Database design & schema)
├── event-architecture.md             (Event bus & messaging design)
├── audit-2026-07-12.md               (Architecture audit)
├── grpc-server-plan.md               (gRPC server implementation plan)
├── mcp-tools-plan.md                 (MCP tools implementation plan)
├── streaming-plan.md                 (Streaming implementation plan)
└── DECISION_DNA.md                   (CMI F1.1 — Decision DNA format & rules)
```

## Architecture Standards (1)

| File | Description | Date |
|------|-------------|------|
| [`DECISION_DNA.md`](DECISION_DNA.md) | CMI F1.1 — Decision DNA: formato padronizado de decisões técnicas, exemplos, regras DDNA vs ADR, integração memorize-commit | 2026-07-30 |

## System Architecture Docs (3)

| File | Description | Date |
|------|-------------|------|
| [`system-architecture-overview.md`](system-architecture-overview.md) | High-level system architecture, component topology, and interaction patterns | 2026-07-12 |
| [`database-architecture.md`](database-architecture.md) | Database design, schema decisions, migration strategy, SQLite rationale | 2026-07-12 |
| [`event-architecture.md`](event-architecture.md) | Event bus design, pub/sub patterns, event contracts, lifecycle events | 2026-07-12 |

## Architecture Plans (3)

| File | Description | Status |
|------|-------------|--------|
| [`grpc-server-plan.md`](grpc-server-plan.md) | gRPC server implementation plan — proto definitions, handler design, service contracts | Planned |
| [`mcp-tools-plan.md`](mcp-tools-plan.md) | MCP (Model Context Protocol) tools plan — tool definitions, protocol integration | Planned |
| [`streaming-plan.md`](streaming-plan.md) | Streaming implementation plan — SSE, WebSocket, gRPC streaming design | Planned |

## Architecture Audits (2)

| File | Description | Date |
|------|-------------|------|
| [`audit-2026-07-12.md`](audit-2026-07-12.md) | Architecture audit — system design review and gap analysis | 2026-07-12 |
| [`adr/analytics-observability-audit-2026-07-28.md`](adr/analytics-observability-audit-2026-07-28.md) | Analytics & Observability audit report (Onda 5 Activation) | 2026-07-28 |

## ADRs — Architecture Decision Records (15)

All ADRs are stored in the [`adr/`](adr/) subdirectory and follow the canonical ADR format: Title, Status, Context, Decision, Rationale, Alternatives, Consequences.

| ADR | Title | Status | Date |
|-----|-------|:------:|------|
| [ADR-0001](adr/adr-0001-ai-architecture.md) | AI Architecture & Capability Framework | Accepted | 2026-07-28 |
| [ADR-1322](adr/adr-1322-compliance.md) | Create Compliance Chief | Accepted | 2026-07-23 |
| [ADR-1430](adr/adr-1430-plugin.md) | Create Plugin Chief | Accepted | 2026-07-23 |
| [ADR-1825](adr/adr-1825-performance.md) | Create Performance Chief | Accepted | 2026-07-23 |
| [ADR-2000](adr/adr-2000-platform.md) | Create Platform Chief | Accepted | 2026-07-23 |
| [ADR-2412](adr/adr-2412-governance.md) | Create Governance Chief | Accepted | 2026-07-23 |
| [ADR-2745](adr/adr-2745-sdk.md) | Create SDK Chief | Accepted | 2026-07-23 |
| [ADR-2958](adr/adr-2958-provider.md) | Create Provider Chief | Accepted | 2026-07-23 |
| [ADR-3159](adr/adr-3159-cache.md) | Create Cache Chief | Accepted | 2026-07-23 |
| [ADR-3428](adr/adr-3428-messaging.md) | Create Messaging Chief | Accepted | 2026-07-23 |
| [ADR-3584](adr/adr-3584-migration.md) | Create Migration Chief | Accepted | 2026-07-23 |
| [ADR-3692](adr/adr-3692-technical-debt.md) | Create Technical Debt Chief | Accepted | 2026-07-23 |
| [ADR-3929](adr/adr-3929-cli.md) | Create CLI Chief | Accepted | 2026-07-23 |
| [ADR-5567](adr/adr-5567-discovery.md) | Create Discovery Chief | Accepted | 2026-07-23 |
| [ADR-7116](adr/adr-7116-api.md) | Create API Chief | Accepted | 2026-07-23 |

## Statistics

| Metric | Value |
|--------|------:|
| Total files | 24 |
| Architecture standards | 1 |
| System architecture docs | 3 |
| Architecture plans | 3 |
| Architecture audits | 2 |
| ADRs | 15 |
| ADR status — Accepted | 15 |
| ADR status — Proposed | 0 |
| ADR date range | 2026-07-23 to 2026-07-28 |

---

*Architecture documents are the authoritative source for design decisions. Per heuristic H-013, always verify code against architecture docs. Per heuristic H-009, architecture docs must be kept in sync with code to prevent knowledge drift.*
*Decision DNA format defined in [DECISION_DNA.md](DECISION_DNA.md) (CMI F1.1) — canonical DDNA template, examples, and rules for DDNA vs ADR.*
