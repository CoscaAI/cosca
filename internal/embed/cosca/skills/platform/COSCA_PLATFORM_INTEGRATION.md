# Cosca Platform Integration — How Skills Work with Cosca

> **Version**: 1.0.0 | **Owner**: Cosca Kernel

## Knowledge Readiness — Every Skill Must Start Here

```bash
# Before using ANY skill, verify the Cosca knows the tools:
cosca knowledge readiness --stack "swiftui,keychain,async-await"
cosca knowledge readiness --stack "compose,retrofit,hilt,room"
cosca knowledge readiness --stack "react,nextjs,tanstack-query,zustand"

# If unknown: cosca knowledge add <lib>
# If partial: cosca knowledge acquire <lib> --allow-remote --compile
```

## Deploy Integration

```bash
# Before deploy:
cosca doctor                    # system diagnostics
cosca knowledge verify          # knowledge integrity
make ci                         # Go tests + frontend tests + provider independence

# CI pipeline: .github/workflows/ci.yml
# Runs on every push: build, test, vet, audit
```

## Agent Integration

Agents must follow KNOWLEDGE_PROTOCOL.md:
```bash
# Before any task:
cosca knowledge readiness --detect

# During work:
cosca knowledge search "how to implement X"

# Before commit:
cosca delegate --target "changed/files" --task "description"
```

## Stack Detection

```bash
# Auto-detect project stack:
cosca knowledge readiness --detect --json

# Output example:
# Detected: go, chi, sqlite, cobra, viper
# → Suggest skills: backend/go/GO_API_IMPLEMENTATION, backend/go/GO_SECURITY
```

## Skill Selection Matrix

| If project has... | Use skills... |
|-------------------|---------------|
| `go.mod` | `backend/go/GO_API_IMPLEMENTATION`, `backend/go/GO_SECURITY` |
| `Cargo.toml` | `backend/rust/RUST_API_IMPLEMENTATION` |
| `pyproject.toml` | `backend/python/PYTHON_API_IMPLEMENTATION` |
| `package.json` + React | `frontend/react/REACT_IMPLEMENTATION` |
| `package.json` + Vue | `frontend/vue/VUE_IMPLEMENTATION` |
| `package.json` + Svelte | `frontend/svelte/SVELTE_IMPLEMENTATION` |
| `angular.json` | `frontend/angular/ANGULAR_IMPLEMENTATION` |
| `Podfile` | `mobile/swift/SWIFTUI_IMPLEMENTATION` |
| `build.gradle.kts` + Android | `mobile/kotlin/ANDROID_IMPLEMENTATION` |
| `pubspec.yaml` | `mobile/flutter/FLUTTER_IMPLEMENTATION` |
| `app.json` + Expo | `mobile/react-native/RN_IMPLEMENTATION` |
| `platformio.ini` | `embedded/MICROCONTROLLER_C` |
| `*.kicad_sch` | `embedded/PCB_DESIGN` |
| `package.xml` + ROS2 | `robotics/ROS2_IMPLEMENTATION` |

## Cosca Commands Every Agent Should Know

| Command | Purpose |
|---------|---------|
| `cosca knowledge readiness --detect` | Anti-hallucination gate |
| `cosca knowledge search "<query>"` | FTS5 knowledge lookup |
| `cosca knowledge acquire <lib> --allow-remote` | Fetch official docs |
| `cosca knowledge verify --fix` | Repair vector embeddings |
| `cosca doctor` | Full system diagnostic |
| `cosca delegate --target "..." --task "..."` | Delegate with plan |
| `cosca session register --files "..." --agent "..."` | Cross-session file lock |
| `cosca session status --file "..."` | Check if file is in use |
