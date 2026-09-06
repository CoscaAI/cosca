---
name: metrics-dashboard
description: Use when the user asks to build or review a metrics dashboard, including KPIs and observability panels.
---

# Metrics Dashboard

> **Version**: 1.0.0 | **Status**: active | **Owner**: Analytics Chief | **Last Updated**: 2026-07-27

## Purpose
Design and build analytics dashboards for Cosca operational metrics.

## Process
1. Define KPIs: API latency (p50/p99), error rate, active users, token usage, plugin count.
2. Choose dashboard tool: Grafana (Prometheus datasource) or custom React dashboard.
3. Query Prometheus metrics exposed by cosca serve (cosca_runtime_*, cosca_knowledge_*).
4. Build visualizations: time series for trends, gauges for health, tables for top-N.
5. Add alert thresholds: color-coded (green/yellow/red) based on SLO targets.
6. Test dashboard with real data from running instance.

## Success Criteria
- All KPIs visible on single dashboard
- Data refreshes < 30 seconds
- Alert thresholds trigger correctly
