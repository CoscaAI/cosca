# Script Generation

> **Version**: 1.0.0 | **Status**: active | **Owner**: Automation Chief | **Last Updated**: 2026-07-27

## Purpose
Generate shell scripts, automation tooling, and development helper scripts for Cosca workflows.

## Process
1. Identify the repetitive task to automate (build, test, deploy, cleanup).
2. Choose scripting language: bash for system tasks, Python for complex logic, Go for CLI tools.
3. Design CLI interface: flags, args, help text, exit codes.
4. Implement with error handling: set -euo pipefail (bash), try/except (Python), if err != nil (Go).
5. Add validation: input checks, dependency verification, dry-run mode.
6. Test on clean environment. Document usage in script header.
7. Register in Makefile or .cosca/scripts/ as appropriate.

## Success Criteria
- Script completes without errors on clean environment
- Help text explains all flags and usage
- Exit code 0 on success, non-zero on failure
