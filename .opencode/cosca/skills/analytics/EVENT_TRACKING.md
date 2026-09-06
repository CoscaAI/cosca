---
name: event-tracking
description: Use when the user asks to implement event tracking or analytics instrumentation across product flows.
---

# Event Tracking

> **Version**: 1.0.0 | **Status**: active | **Owner**: Analytics Chief | **Last Updated**: 2026-07-27

## Purpose
Implement analytics event pipelines and user behavior tracking.

## Process
1. Define event schema: user_id, event_type, timestamp, properties (JSON).
2. Implement client-side tracking: page views, button clicks, feature usage.
3. Implement server-side tracking: API calls, errors, auth events, performance metrics.
4. Choose event pipeline: direct SQLite insert (small scale) or message queue (scale).
5. Add privacy controls: PII scrubbing, opt-out support, data retention policies.
6. Build event analytics: funnel analysis, user journey mapping, retention cohorts.

## Success Criteria
- Events fire with < 10ms overhead
- PII never stored in event properties
- GDPR/LGPD compliant: opt-out and data export supported
