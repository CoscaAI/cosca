# Command Design

> **Version**: 1.0.0 | **Status**: active | **Owner**: CLI Chief | **Last Updated**: 2026-07-27

## Purpose
Design Cobra CLI commands with consistent UX, help text, and shell completion.

## Process
1. Define command hierarchy: root → subcommand → flags/args.
2. Write help text: one-line summary, long description, examples section.
3. Implement flags: consistent naming (--kebab-case), types (string, int, bool), defaults.
4. Add validation: required flags, mutually exclusive flags, value ranges.
5. Implement shell completion: bash, zsh, fish via cobra completion command.
6. Style output: zerolog for logs, color for CLI output, JSON for machine parsing.
7. Test: --help output, invalid inputs, edge cases, pipe compatibility.

## Success Criteria
- --help output clear and complete for every command
- Shell completion works for all 3 shells
- --json flag available for machine-readable output
