# Plugin Security

> **Version**: 1.0.0 | **Status**: active | **Owner**: Plugin Chief | **Last Updated**: 2026-07-27

## Purpose
Audit plugin sandboxing, capability permissions, and runtime isolation.

## Process
1. Review plugin manifest: are requested capabilities minimal? Can permissions be reduced?
2. Test sandbox isolation: attempt filesystem access outside allowed paths, network calls without permission.
3. Verify resource limits: memory cap enforced, execution timeout works, no CPU monopolization.
4. Check checksum integrity: verify SHA-256 before loading, detect tampering.
5. Audit plugin dependencies: scan WASM binary for known CVEs.
6. Test plugin uninstall: all resources cleaned up, no residual state.

## Success Criteria
- Zero sandbox escape vulnerabilities
- All capability requests justified and minimal
- Resource limits enforced under load
