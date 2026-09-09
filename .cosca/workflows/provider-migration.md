# WORKFLOW: provider-migration

> **Version**: 1.0.0 | **Category**: migration | **Estimated Duration**: 1-3 days | **Status**: active | **Owner**: Provider Chief | **Last Updated**: 2026-07-23

## OBJECTIVE
Migrate from one AI or cloud provider to another with minimal disruption. Includes parallel running, automatic failover readiness, cost/performance comparison, and comprehensive validation.

## INPUTS
| Name | Type | Required | Description |
|------|------|----------|-------------|
| source_provider | String | Yes | Provider to migrate from (name and version) |
| target_provider | String | Yes | Provider to migrate to (name and version) |
| services_affected | String[] | Yes | Services currently using the source provider |
| migration_reason | String | Yes | cost, performance, compliance, deprecation, capability |
| parallel_run_days | Number | No | Days of parallel operation before cutover (default: 7) |

## OUTPUTS
| Name | Type | Description |
|------|------|-------------|
| Migration result | Success/Fail | Migration outcome |
| Performance comparison | Report | Source vs target latency, throughput, cost |
| Cost impact analysis | Report | Projected cost difference |
| Rollback readiness | Document | Fallback plan and trigger conditions |

## PRECONDITIONS
1. Source provider deprecation notice received (if applicable)
2. Target provider evaluated and approved by Provider Chief
3. Integration adapter for target provider developed and tested
4. Stakeholders notified of migration plan and timeline

## POSTCONDITIONS
1. All services migrated to target provider
2. Source provider traffic at zero (or scheduled for decommission)
3. Performance verified and within SLOs
4. Cost impact documented
5. Rollback plan preserved for 30 days
6. Migration documented in memory/architecture

## STEPS
### Step 1: Provider Assessment
- **Chief**: Provider Chief
- **Specialists**: Integration Engineer, Benchmark Engineer
- **Task**: Evaluate target provider capabilities, costs, SLAs, compliance
- **Output**: Provider assessment report

### Step 2: Adapter Development
- **Chief**: Provider Chief
- **Specialists**: Integration Engineer
- **Task**: Develop provider adapter implementing standard interface
- **Output**: Provider adapter with tests

### Step 3: Parallel Running Setup
- **Chief**: Provider Chief
- **Specialists**: Integration Engineer, AI Chief (if AI provider)
- **Task**: Configure dual-provider mode (source + target running in parallel)
- **Output**: Parallel running configuration

### Step 4: Performance Validation
- **Chief**: Performance Chief
- **Specialists**: Benchmark Engineer
- **Task**: Compare latency, throughput, cost between source and target
- **Output**: Performance comparison report

### Step 5: Service Migration
- **Chief**: Provider Chief
- **Specialists**: Integration Engineer, affected service Chiefs
- **Task**: Migrate services one by one from source to target
- **Output**: Services migrated

### Step 6: Source Decommission
- **Chief**: Provider Chief
- **Specialists**: Integration Engineer
- **Task**: Verify all traffic on target, decommission source integration
- **Output**: Source provider decommissioned

### Step 7: Documentation & Lessons Learned
- **Chief**: Documentation Chief
- **Specialists**: Technical Writer
- **Task**: Document migration process, results, lessons learned
- **Output**: Migration report

## VALIDATION
1. All services functional on target provider
2. Performance meets or exceeds source provider
3. Cost within projected budget
4. No regressions in dependent services
5. Rollback plan tested and ready

## SUCCESS CRITERIA
- [ ] All services migrated successfully
- [ ] Performance validated against SLOs
- [ ] Cost impact within projections
- [ ] No production incidents during migration
- [ ] Rollback plan available
- [ ] Migration documented in memory

## ERROR HANDLING
| Failure | Action |
|---------|--------|
| Target provider performance below SLO | Extend parallel run, escalate to Provider Chief |
| Service migration causes errors | Rollback service to source provider, investigate |
| Cost exceeds projections by >20% | Escalate to CEO for budget decision |
| Provider adapter incompatible | Escalate to Platform Chief for adapter redesign |

## RELATED
- [Provider Chief](../departments/provider/SKILL.md)
- [Provider Integration skill](../skills/platform/PROVIDER_INTEGRATION.md)
- [PROVIDER_INTERFACE.md](../identidade/PROVIDER_INTERFACE.md)
- [migration-execution.md](./migration-execution.md)

## HISTORY
| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-23 | Cosca Enterprise Evolution | Initial workflow creation |
