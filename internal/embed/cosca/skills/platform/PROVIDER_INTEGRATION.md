---
name: provider-integration
description: Use when the user asks to integrate a new AI, cloud, or third-party provider following standardized patterns.
---

> **Version**: 1.0.0 | **Status**: active | **Owner**: Platform Chief | **Last Updated**: 2026-07-23

# PROVIDER INTEGRATION SKILL

## Description
Integrate new AI, cloud, or third-party providers into the platform following standardized patterns. Includes provider discovery, API integration, failover configuration, cost tracking, and monitoring setup.

## Inputs
| Input | Required | Description |
|-------|----------|-------------|
| provider_name | Yes | Name of the provider to integrate |
| provider_type | Yes | `ai-llm`, `ai-embedding`, `cloud`, `monitoring`, `auth`, `payment`, `other` |
| api_endpoint | Yes | Provider API base URL |
| auth_method | Yes | `api-key`, `oauth2`, `jwt`, `mtls`, `none` |
| features | No | Required features: `streaming`, `batch`, `webhooks`, `failover` |

## Outputs
| Output | Description |
|--------|-------------|
| Integration guide | Step-by-step integration documentation |
| Provider adapter | Configuration for provider abstraction layer |
| Monitoring dashboards | Provider health and performance dashboards |
| Failover configuration | Automatic failover rules and circuit breaker settings |

## Integration Steps

### 1. Provider Assessment
- API capability review (endpoints, rate limits, quotas)
- Authentication requirements
- SLA guarantees and historical reliability
- Pricing model and cost estimation
- Compliance and data residency requirements
- Deprecation and versioning policies

### 2. Adapter Implementation
- Create provider adapter implementing standardized interface
- Implement authentication flow
- Handle rate limiting and retry logic
- Implement error mapping and normalization
- Add telemetry and logging

### 3. Failover Configuration
- Configure circuit breaker (failure threshold, cooldown)
- Define fallback provider
- Set up health check endpoint monitoring
- Configure automatic failover triggers

### 4. Cost Tracking
- Implement usage tracking (token count, API calls)
- Configure cost allocation per team/service
- Set up budget alerts
- Create cost dashboards

### 5. Testing
- Unit test adapter against provider API
- Integration test with sandbox environment
- Failover test (primary → secondary)
- Load test (rate limits and throttling)
- Security test (authentication, data handling)

### 6. Documentation
- Provider integration guide
- Configuration reference
- Troubleshooting guide
- Migration guide (if replacing existing provider)

## Success Criteria
- [ ] Provider adapter implemented and tested
- [ ] Authentication configured and verified
- [ ] Failover tested (primary → secondary)
- [ ] Cost tracking configured
- [ ] Monitoring dashboards created
- [ ] Documentation complete
- [ ] Integration passes security review

## Related
- [Platform Chief](../../departments/platform/SKILL.md)
- [Provider Chief](../../departments/provider/SKILL.md)
- [Configuration Validation](./CONFIGURATION_VALIDATION.md)
- [Project Bootstrap](./PROJECT_BOOTSTRAP.md)
- [PROVIDER_INTERFACE.md](../../PROVIDER_INTERFACE.md)
- [workflows/provider-migration.md](../../workflows/provider-migration.md)
