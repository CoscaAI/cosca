---
name: learning
description: Enables the Cosca to improve over time - analyzes past performance and evolves agent behavior.
level: 2
---

> **Version**: 1.0.0 | **Status**: active | **Owner**: Learning Engine | **Last Updated**: 2026-07-10

# LEARNING ENGINE

## PURPOSE
The Learning Engine enables the Cosca to improve over time. It analyzes past performance, identifies patterns, and evolves agent behavior.

## LEARNING DIMENSIONS

### 1. Agent Performance Learning
- Which agents succeed most often on which task types?
- Which agents are fastest?
- Which agents produce highest quality output?
- Which agent combinations work best together?
- Track and update agent selection preferences

### 2. Workflow Optimization
- Which workflow steps are bottlenecks?
- Which steps can be parallelized?
- Which steps can be skipped or simplified?
- Which steps need more review?
- Suggest workflow improvements

### 3. Pattern Recognition
- Identify common bug patterns and their fixes
- Identify common feature patterns
- Identify common refactoring patterns
- Build a pattern library for faster execution

### 4. Quality Correlation
- Which practices correlate with higher quality?
- Which practices correlate with fewer bugs?
- Which review patterns catch the most issues?
- Adjust quality gates based on learnings

### 5. Estimation Improvement
- Compare estimated vs actual effort
- Identify bias in estimates
- Improve estimation models
- Track accuracy over time

### 6. User Adaptation
- Learn user preferences (tech stack, patterns, style)
- Learn user priorities (speed vs quality vs cost)
- Learn user communication style
- Adapt behavior to user

## LEARNING CYCLE

```
Collect Data → Analyze → Identify Patterns → Generate Insights → Apply Improvements → Measure Impact → Repeat
```

## LEARNING DATA STRUCTURE

```json
{
  "id": "uuid",
  "type": "agent_performance|workflow_optimization|pattern|quality|estimation|user",
  "timestamp": "ISO8601",
  "observation": "string",
  "analysis": "string",
  "insight": "string",
  "confidence": 0.0-1.0,
  "recommendation": "string",
  "applied": false,
  "impact": null,
  "related_learnings": ["uuid"]
}
```

## LEARNING OUTPUTS

### Weekly Learning Report
```markdown
# LEARNING REPORT — Week [N]

## TOP INSIGHTS
1. [Insight with evidence]
2. [Insight with evidence]

## AGENT PERFORMANCE
| Agent | Tasks | Success | Avg Time | Trend |
|-------|-------|---------|----------|-------|

## WORKFLOW EFFICIENCY
| Workflow | Avg Time | Bottleneck | Suggestion |
|----------|---------|------------|------------|

## QUALITY TRENDS
| Metric | Trend | Insight |
|--------|-------|---------|

## RECOMMENDATIONS
1. [Actionable recommendation]
2. [Actionable recommendation]
```

## SELF-MODIFICATION CAPABILITIES
The Learning Engine can:
- Adjust agent selection preferences (which agent for which task)
- Suggest workflow modifications
- Adjust quality gate thresholds
- Update estimation models
- Cannot: Change architecture without Architecture Chief approval
- Cannot: Change security policies without Security Chief approval
- Cannot: Change product scope without Product Chief approval

## DEPENDENCIES
- Collects data from all engines
- Stores insights in Memory Engine (${MEMORY_GLOBAL}/agent/)
- Feeds recommendations to Evolution Engine
- Reports to CEO for strategic improvements

## RELATED
- [Evolution Engine](../evolution/SKILL.md) — Receives learning recommendations for system improvements

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-10 | Cosca Refactor | Added metadata, HISTORY, and cross-references |
