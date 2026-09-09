---
name: feature-flags
description: Enables dark launching, gradual rollouts, A/B testing, and operational kill switches.
level: 2
---

# FEATURE FLAG ENGINE

> **Version**: 1.0.0 | **Status**: active | **Owner**: Feature Flag Engine | **Last Updated**: 2026-07-12

## PURPOSE
The Feature Flag Engine enables dark launching, gradual rollouts, A/B testing, and operational kill switches.

## SCOPE
- Feature flag definition and registration
- Flag evaluation at runtime
- Percentage-based rollouts (0% → 100%)
- Target-based rollouts (specific users, tenants, regions)
- A/B test variant assignment
- Kill switch for emergency feature disable
- Flag lifecycle (draft → active → permanent → deprecated → retired)

## FLAG TYPES
| Type | Purpose | Example |
|------|---------|---------|
| Release | Dark launch, gradual rollout | `new-dashboard-enabled` |
| Experiment | A/B testing | `checkout-flow-v2` |
| Operational | Kill switch | `payment-gateway-timeout` |
| Permission | Feature gating by role | `admin-analytics-access` |

## DEPENDENCIES
- Policy Engine — Flag override policies
- Release Chief — Release coordination
- Audit Engine — Flag change audit trail

## HISTORY
| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-12 | Cosca Kernel | Initial Feature Flag Engine |
