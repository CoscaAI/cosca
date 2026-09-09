> **Version**: 1.0.0 | **Status**: active | **Owner**: Memory Chief | **Last Updated**: 2026-07-23
> 
> # MEMORY SYNCHRONIZATION SKILL
> 
> ## Description
> Use this skill to synchronize memory across all memory stores. Promotes important entries from short to long memory, archives stale entries, updates indices, and ensures cross-store consistency.
> 
> ## Inputs
> | Input | Required | Description |
> |-------|----------|-------------|
> | session_id | Yes | Current session identifier |
> | sync_type | Yes | `session-end`, `full`, `archive`, `promote` |
> | stores | No | Specific stores to sync (default: all) |
> 
> ## Outputs
> | Output | Description |
> |--------|-------------|
> | Sync report | What was synchronized |
> | Promoted entries | Short → Long promotions |
> | Archived entries | Archived stale records |
> | Index updates | Updated store indices |
> 
> ## Process
> 1. Identify entries for promotion (short → long memory)
> 2. Evaluate entries for archival (based on retention policy)
> 3. Update memory store indices
> 4. Prune expired entries per MEMORY_MODEL.md retention
> 5. Cross-reference new entries with existing knowledge
> 6. Update search indices
> 7. Generate synchronization report
> 
> ## Promotion Criteria
> - Decision records: always promote
> - Architecture decisions: always promote
> - Bug patterns: promote if severity >= medium
> - Successful patterns: promote if used > 2 times
> - Agent learnings: promote if impact > threshold
> 
> ## Retention Enforcement
> | Memory Type | Retention | Action |
> |-------------|-----------|--------|
> | Short | Session only | Archive all |
> | Long | Project lifetime | Archive if project==current |
> | Agent | 12 months | Archive older records |
> | Pattern | Forever (review) | Flag for review if > 1yr |
> | Bug | Forever | No automatic archival |
> 
> ## Success Criteria
> - [ ] All stores synchronized
> - [ ] Entries promoted correctly
> - [ ] Stale entries archived
> - [ ] Indices updated and valid
> - [ ] Sync report generated
> 
> ## Related
> - [Memory Chief](../../departments/memory/SKILL.md)
> - [MEMORY_MODEL.md](../../MEMORY_MODEL.md) — Memory taxonomy
> - [Memory Engine](../../engines/memory/SKILL.md) — Memory operations
> - [Context Engine](../../engines/context/SKILL.md) — Context building
