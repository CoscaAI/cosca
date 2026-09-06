---
name: configuration-validation
description: Use when the user asks to validate configuration files for correctness, consistency, security, and conformance to standards.
---

> **Version**: 1.0.0 | **Status**: active | **Owner**: Platform Chief | **Last Updated**: 2026-07-23

# CONFIGURATION VALIDATION SKILL

## Description
Validate configuration files across the platform for correctness, consistency, security, and compliance with organizational standards. Supports JSON, YAML, TOML, INI, and environment variable formats.

## Inputs
| Input | Required | Description |
|-------|----------|-------------|
| config_path | Yes | Path to configuration file(s) |
| schema_path | No | Path to JSON Schema or similar validation schema |
| format | Yes | `json`, `yaml`, `toml`, `ini`, `env`, `auto-detect` |
| strict_mode | No | Enable strict validation (default: true) |

## Outputs
| Output | Description |
|--------|-------------|
| Validation report | Pass/fail per validation rule |
| Schema violations | Values that don't match schema |
| Security issues | Exposed secrets, permissive permissions |
| Recommendations | Best practice improvements |

## Validation Categories

### Schema Validation
- Required fields present
- Correct data types for all values
- Enum values within allowed set
- Format validation (email, URI, date-time, etc.)
- Minimum/maximum constraints satisfied
- Pattern constraints matched

### Security Validation
- No hardcoded secrets or credentials
- File permissions not overly permissive
- No sensitive data in config (passwords, tokens)
- TLS/SSL configuration secure
- CORS origins properly restricted

### Consistency Validation
- No duplicate keys or values
- Cross-reference values consistent (e.g., ports match)
- Environment-specific overrides complete
- No deprecated configuration keys
- Default values documented

### Best Practices
- Comments explaining non-obvious values
- Configuration organized logically
- Sensible defaults provided
- Documentation for each section
- Version pinning for dependencies

## Process
1. Parse configuration file based on format
2. Validate against schema (if provided or discovered)
3. Check required fields and data types
4. Scan for security issues (secrets, permissions)
5. Verify consistency across config files
6. Assess best practice compliance
7. Generate report with errors, warnings, suggestions

## Success Criteria
- [ ] Schema validation passed (if schema provided)
- [ ] Required fields present and typed correctly
- [ ] No security issues found
- [ ] Cross-configuration consistency verified
- [ ] Recommendations for improvement provided

## Related
- [Platform Chief](../../departments/platform/SKILL.md)
- [Project Bootstrap](./PROJECT_BOOTSTRAP.md)
- [Provider Integration](./PROVIDER_INTEGRATION.md)
- [DevOps Chief](../../departments/devops/SKILL.md)
