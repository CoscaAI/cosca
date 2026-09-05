# WORKFLOW: chaos-testing

> **Version**: 1.0.0 | **Category**: testing | **Estimated Duration**: 2-5 days | **Status**: active | **Owner**: Infrastructure Chief | **Last Updated**: 2026-07-23

## OBJECTIVE
Execute controlled chaos experiments to validate system resilience. Inject failures (network latency, pod crashes, resource exhaustion) into production-like environments to identify weaknesses before they cause real incidents.

## INPUTS
| Name | Type | Required | Description |
|------|------|----------|-------------|
| target_services | String[] | Yes | Services to test |
| experiment_type | String | Yes | `pod-kill`, `network-latency`, `resource-exhaustion`, `dns-failure`, `region-failover`, `all` |
| blast_radius | String | Yes | `single-pod`, `single-service`, `single-az`, `single-region` |
| experiment_duration | Number | No | Max duration in minutes (default: 30) |
| rollback_plan | Document | Yes | Automatic rollback triggers |

## OUTPUTS
| Name | Type | Description |
|------|------|-------------|
| Experiment report | Document | Results and findings |
| Weakness inventory | Document | Identified resilience gaps |
| Improvement roadmap | Document | Prioritized resilience fixes |
| Runbook updates | Document | Updated incident runbooks |

## STEPS
### Step 1: Hypothesis Formation
- **Chief**: Infrastructure Chief
- **Specialists**: SRE
- **Task**: Define what resilience behaviors to validate
- **Output**: Experiment hypothesis document

### Step 2: Experiment Design
- **Chief**: Infrastructure Chief
- **Specialists**: SRE, DevOps Chief
- **Task**: Design experiment parameters and blast radius
- **Output**: Experiment design with rollback triggers

### Step 3: Environment Setup
- **Chief**: DevOps Chief
- **Specialists**: SRE
- **Task**: Configure staging environment, monitoring, and rollback automation
- **Output**: Ready experiment environment

### Step 4: Experiment Execution
- **Chief**: Infrastructure Chief
- **Specialists**: SRE, Monitoring Chief
- **Task**: Execute chaos experiment with monitoring
- **Output**: Real-time experiment telemetry

### Step 5: Results Analysis
- **Chief**: Infrastructure Chief
- **Specialists**: Monitoring Chief, Performance Chief
- **Task**: Analyze system behavior, identify weaknesses
- **Output**: Experiment findings

### Step 6: Improvement Planning
- **Chief**: Infrastructure Chief
- **Specialists**: All affected Chiefs
- **Task**: Prioritize resilience improvements
- **Output**: Improvement roadmap with owners

### Step 7: Runbook Updates
- **Chief**: Documentation Chief
- **Specialists**: Technical Writer
- **Task**: Update incident runbooks with findings
- **Output**: Updated runbooks

## SUCCESS CRITERIA
- [ ] All experiments executed with safety rollback
- [ ] Weaknesses documented with severity
- [ ] Improvement roadmap created
- [ ] Runbooks updated with findings
- [ ] No production impact (staging only)

## RELATED
- [Infrastructure Chief](../departments/infrastructure/SKILL.md)
- [workflows/disaster-recovery.md](./disaster-recovery.md)
- [workflows/incident-response.md](./incident-response.md)
- [skills/reliability/DISASTER_RECOVERY.md](../skills/reliability/DISASTER_RECOVERY.md)

## PRECONDITIONS
1. Staging environment isolated from production
2. Monitoring and alerting configured for all experiment targets
3. Rollback automation tested before experiment starts
4. Stakeholders notified of experiment window

## POSTCONDITIONS
1. Experiment results documented in memory/architecture
2. Weaknesses inventoried with severity rankings
3. Improvement roadmap created with owners
4. Runbooks updated with findings

## ERROR HANDLING
| Failure | Action |
|---------|--------|
| Blast radius exceeded | Immediate rollback, abort experiment |
| Unexpected production impact | Activate incident response workflow |
| Monitoring gaps detected | Pause experiment, fix monitoring |
| Rollback automation fails | Manual rollback via runbook |

## HISTORY
| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-23 | Cosca Enterprise Evolution | Initial workflow creation |
