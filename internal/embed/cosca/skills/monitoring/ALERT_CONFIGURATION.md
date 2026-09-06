---
name: alert-configuration
description: Use when the user asks to configure, tune, or review monitoring alerts and alerting rules for latency, errors, and saturation.
---

# Alert Configuration

> **Version**: 1.0.0 | **Status**: active | **Owner**: Monitoring Chief | **Last Updated**: 2026-07-27

## Purpose
Design and configure alerting rules for Cosca runtime, API, and infrastructure.

## Process
1. Identify critical metrics (RED: Rate, Errors, Duration).
2. Define SLO thresholds per endpoint (p99 latency < 200ms, error rate < 1%).
3. Configure Prometheus alert rules for threshold breaches.
4. Set up notification channels (Slack, email, PagerDuty).
5. Test alerts with synthetic traffic.
6. Document alert runbook with severity classification (SEV1-SEV4).

## Success Criteria
- All critical endpoints have alert rules
- Alert notifications reach designated channels
- No false positive rate > 5% in first week
