# Plugin Development

> **Version**: 1.0.0 | **Status**: active | **Owner**: Plugin Chief | **Last Updated**: 2026-07-27

## Purpose
Build WASM plugins following the Cosca plugin SDK and runtime contracts.

## Process
1. Define plugin manifest: ID, name, version, runtime, capabilities, permissions.
2. Implement plugin logic following .cosca/plugins/sdk/plugin-sdk-spec.md.
3. Compile to WASM using TinyGo or Rust wasm32-wasi target.
4. Test in sandbox: verify capability isolation, resource limits, timeout handling.
5. Generate checksum (SHA-256) for integrity verification.
6. Package: manifest + wasm binary + checksum = plugin artifact.
7. Register in plugin registry and test installation flow.

## Success Criteria
- Plugin compiles to valid WASM binary
- Sandbox tests pass: capability isolation verified
- Checksum matches after packaging
