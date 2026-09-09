# WORKFLOW: capacity-planning

> **Version**: 1.0.0 | **Category**: ops | **Estimated Duration**: 3-10 days | **Status**: active | **Owner**: Performance Chief | **Last Updated**: 2026-07-23

## OBJECTIVE
Plan infrastructure capacity to meet current and projected demand. Analyze usage trends, model growth scenarios, and recommend scaling strategies to ensure performance SLAs are maintained while optimizing cost.

## INPUTS
| Name | Type | Required | Description |
|------|------|----------|-------------|
| current_metrics | Data | Yes | Current resource utilization data (30+ days) |
| growth_projections | Document | Yes | Business growth forecasts (users, transactions) |
| slo_requirements | Document | Yes | Target SLOs for latency, throughput, availability |
| budget_constraints | String | No | Available budget for infrastructure (monthly) |

## OUTPUTS
| Name | Type | Description |
|------|------|-------------|
| Capacity plan | Document | Current and projected capacity analysis |
| Scaling recommendations | Document | Vertical and horizontal scaling actions |
| Cost forecast | Document | Projected infrastructure costs |
| Risk assessment | Document | Capacity risks and mitigation strategies |

## STEPS
### Step 1: Data Collection
- **Chief**: Performance Chief
- **Specialists**: Monitoring Chief
- **Task**: Collect 30+ days of utilization metrics (CPU, memory, network, storage, DB connections)
- **Output**: Utilization baseline report

### Step 2: Trend Analysis
- **Chief**: Performance Chief
- **Specialists**: Analytics Chief
- **Task**: Analyze growth trends, seasonality, and inflection points
- **Output**: Trend analysis with growth rates

### Step 3: Demand Modeling
- **Chief**: Performance Chief
- **Specialists**: Product Chief
- **Task**: Model future demand based on business projections
- **Output**: Demand model (3/6/12 months)

### Step 4: Capacity Gap Analysis
- **Chief**: Performance Chief
- **Specialists**: Infrastructure Chief
- **Task**: Compare projected demand against current capacity
- **Output**: Capacity gap analysis

### Step 5: Scaling Strategy
- **Chief**: Infrastructure Chief
- **Specialists**: DevOps Chief, Cache Chief, Database Chief
- **Task**: Design scaling strategies for identified gaps
- **Output**: Scaling recommendation document

### Step 6: Cost Analysis
- **Chief**: Provider Chief
- **Specialists**: Cost Analyst
- **Task**: Calculate costs for each scaling option
- **Output**: Cost comparison matrix

### Step 7: Plan Approval
- **Chief**: Performance Chief
- **Specialists**: CTO
- **Task**: Present plan for approval and budget allocation
- **Output**: Approved capacity plan

## SUCCESS CRITERIA
- [ ] Utilization baselines established for all resources
- [ ] Growth trends identified and modeled
- [ ] Capacity gaps quantified with impact analysis
- [ ] Scaling options evaluated with costs
- [ ] Budget approved for recommended actions

## RELATED
- [Performance Chief](../departments/performance/SKILL.md)
- [Infrastructure Chief](../departments/infrastructure/SKILL.md)
- [skills/performance/PERFORMANCE_AUDIT.md](../skills/performance/PERFORMANCE_AUDIT.md)
- [workflows/performance-optimization.md](./performance-optimization.md)

## PRECONDITIONS
1. Monitoring data available for 30+ days
2. Business growth projections provided by Product Chief
3. Budget constraints defined by CEO

## POSTCONDITIONS
1. Capacity plan approved by CTO
2. Scaling recommendations documented with cost estimates
3. Budget allocated for recommended capacity increases

## ERROR HANDLING
| Failure | Action |
|---------|--------|
| Insufficient data | Extend monitoring period, use industry benchmarks |
| Budget insufficient | Escalate to CEO for exception |
| Growth projections unreliable | Use conservative and aggressive scenarios |

## HISTORY
| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-23 | Cosca Enterprise Evolution | Initial workflow creation |
