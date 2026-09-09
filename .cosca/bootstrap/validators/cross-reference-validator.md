# CROSS-REFERENCE VALIDATOR

> **Version**: 1.0.0 | **Status**: active | **Owner**: Bootstrap Engine | **Last Updated**: 2026-07-12

## PURPOSE
Automatically validate all cross-references and internal links across the Cosca ecosystem. Detect broken links, orphan files, circular references, and stale index entries. Run as part of Gate 0 validation during Bootstrap Phase 9.

## ACTIVATION
- **Bootstrap Phase 9**: Quality validation before handover
- **On skill change**: After any SKILL.md or workflow update
- **On demand**: `/validate-references` command
- **Scheduled**: Weekly integrity check

## PROCESS

### Phase 1: Link Extraction
```
1. Scan all .md files in COSCA_HOME recursively
2. Extract all markdown links: [text](path)
3. Extract all file references in tables
4. Classify each link:
   - internal_relative: ../path/to/file.md
   - internal_absolute: /absolute/path
   - virtual_path: ${ENGINES_HOME}/engine/SKILL.md
   - external: https://...
```

### Phase 2: Path Resolution
```
For each extracted link:
  1. If virtual_path → resolve via Resource Resolver
  2. If relative → resolve from file location
  3. If absolute → use as-is
  4. Check if target file exists
  5. Record: resolved_path, exists: true/false
```

### Phase 3: Validation
```
Broken Links:
  → Target file does not exist
  → Severity: ERROR

Orphan Files:
  → File exists but is not referenced by any other file
  → Exception: Root-level governance docs, CHANGELOG
  → Severity: WARNING

Circular References:
  → A references B, B references C, C references A
  → Severity: WARNING

Stale Index:
  → COSCA_INDEX.md count does not match actual file count
  → COSCA_INDEX.md missing entries for existing files
  → COSCA_INDEX.md has entries for non-existent files
  → Severity: ERROR

Duplicate Content:
  → Two files with identical section content (> 80% similarity)
  → Severity: WARNING

Missing Convention:
  → File does not follow CONVENTIONS.md format
  → Severity: WARNING
```

### Phase 4: Report Generation
```
Generate validation report:
  → Count: total files, total links, broken, orphans, warnings
  → List: each broken link with file:line → target
  → List: each orphan file with suggested parent
  → List: each circular reference chain
  → Score: Link Health % (valid / total * 100)
```

## VALIDATION RULES

### Link Syntax Rules
| Rule | Pattern | Severity |
|------|---------|----------|
| Relative path uses ../ correctly | `../path` goes up one level from file | error |
| No raw absolute paths | Must not contain `/home/` or `C:\` | error |
| Virtual paths use correct variable | `${COSCA_HOME}`, `${ENGINES_HOME}`, etc. | error |
| No broken fragment links | `#section` must exist in target file | warn |
| Image links reference existing files | `![alt](img.png)` must exist | warn |

### File Organization Rules
| Rule | Description | Severity |
|------|-------------|----------|
| Department has SKILL.md | Every department dir must have exactly 1 SKILL.md | error |
| Engine has SKILL.md | Every engine dir must have exactly 1 SKILL.md | error |
| Memory store has INDEX.md | Every memory/ dir must have INDEX.md | error |
| INDEX files reference actual records | INDEX must not list records that don't exist | warn |
| No duplicate file names | No two SKILL.md files with same purpose | error |
| Templates follow naming convention | `templates/*/TEMPLATE.md` | warn |

### Cross-Reference Health Rules
| Rule | Threshold | Severity |
|------|-----------|----------|
| Link validity rate | ≥ 99% | error if below |
| Orphan file rate | ≤ 5% | warn if above |
| Circular reference count | 0 | warn if > 0 |
| INDEX accuracy | 100% | error if below |
| Convention compliance rate | ≥ 95% | warn if below |

## OUTPUT FORMAT

```markdown
# CROSS-REFERENCE VALIDATION REPORT
Generated: 2026-07-12 | Bootstrap Phase 9

## SUMMARY
- Files scanned: 120
- Total links: 450
- Valid links: 448 (99.6%)
- Broken links: 2 (0.4%)
- Orphan files: 3 (2.5%)
- Circular refs: 0
- INDEX accuracy: 100%
- HEALTH SCORE: A (98.5%)

## BROKEN LINKS (2)
| File | Line | Link | Target | Issue |
|------|------|------|--------|-------|
| engines/audit/SKILL.md | 99 | `../monitoring/SKILL.md` | (not found) | File was moved to departments/ |
| workflows/deployment.md | 45 | `QUALITY_GATES.md` | (wrong depth) | Should be `../QUALITY_GATES.md` |

## ORPHAN FILES (3)
| File | Suggested Parent |
|------|-----------------|
| memory/short/session-old.md | Archive or link from memory/short/INDEX.md |
| templates/deprecated/old-crm/TEMPLATE.md | Mark as deprecated or remove |
| .cosca-scaffold/legacy-config.yml | Archive |

## CIRCULAR REFERENCES (0)
None detected.

## INDEX ACCURACY
- COSCA_INDEX.md: 100% accuracy (120 files matched)

## RECOMMENDATIONS
1. Fix 2 broken links (see above)
2. Review 3 orphan files for archival
3. All clear — no structural issues detected
```

## INTEGRATION

### Bootstrap Phase 9 Integration
```
Bootstrap Phase 9 (Quality Validation):
  → Language Detector
  → Framework Detector
  → ...
  → Cross-Reference Validator ← runs here
  → Cosca Component Validator
  → Dependency Validator
  → Gate 0 Result
```

### Pre-Commit Hook Integration
```
On git commit:
  → Run cross-reference validator on changed files only
  → If broken links detected → block commit
  → Report: "Commit blocked: 2 broken links in engines/audit/SKILL.md"
```

## ERROR HANDLING

| Failure | Action |
|---------|--------|
| File not readable | Skip file, log warning, continue |
| Resource Resolver unavailable | Use last-known paths from cache |
| Too many broken links (> 10%) | Abort validation, report systemic issue |
| Circular dependency detected | Report chain, do not block (warning only) |

## RELATED
- [BOOTSTRAP.md](../BOOTSTRAP.md) — Bootstrap Phase 9 integration
- [project-validator.md](project-validator.md) — Project structure validation
- [cosca-validator.md](cosca-validator.md) — Cosca component validation
- [dependency-validator.md](dependency-validator.md) — Dependency health validation
- [Resource Resolver](../../engines/resource-resolver/SKILL.md) — Virtual Path resolution
- [CONVENTIONS.md](../../identidade/CONVENTIONS.md) — File format standards

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-12 | Cosca Kernel | Initial cross-reference validator — link checking, orphan detection, INDEX accuracy |
