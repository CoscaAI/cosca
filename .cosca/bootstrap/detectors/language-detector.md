# LANGUAGE DETECTOR

> **Version**: 1.0.0 | **Status**: active | **Owner**: Cosca Bootstrap

## PURPOSE
Detect programming languages used in the workspace by scanning for language-specific files and analyzing dependencies.

## DETECTION RULES

### Primary Detection (by config file)
| File | Language | Certainty |
|------|----------|-----------|
| package.json | JavaScript / TypeScript | High |
| tsconfig.json | TypeScript | Confirmed |
| requirements.txt | Python | High |
| pyproject.toml | Python | High |
| setup.py / setup.cfg | Python | High |
| Cargo.toml | Rust | Confirmed |
| go.mod | Go | Confirmed |
| Gemfile | Ruby | High |
| pom.xml | Java / Kotlin | High |
| build.gradle / build.gradle.kts | Java / Kotlin | High |
| composer.json | PHP | High |
| mix.exs | Elixir | Confirmed |
| CMakeLists.txt | C / C++ | High |
| *.csproj | C# | Confirmed |
| pubspec.yaml | Dart / Flutter | Confirmed |

### Secondary Detection (by file extension)
| Extension | Language(s) |
|-----------|------------|
| .ts, .tsx | TypeScript |
| .js, .jsx, .mjs | JavaScript |
| .py | Python |
| .rs | Rust |
| .go | Go |
| .rb | Ruby |
| .java | Java |
| .kt, .kts | Kotlin |
| .cs | C# |
| .php | PHP |
| .ex, .exs | Elixir |
| .c, .h | C |
| .cpp, .hpp, .cc | C++ |
| .dart | Dart |
| .swift | Swift |

### TypeScript Confirmation
If `package.json` exists AND (`tsconfig.json` OR `.ts`/`.tsx` files found OR `typescript` in dependencies):
- Language: TypeScript
Else if `package.json` exists:
- Language: JavaScript

### Multi-Language Detection
If multiple language files detected:
1. Count files per language
2. Primary = language with most files
3. Secondary = languages with > 10% file share
4. Report all detected

## OUTPUT
```yaml
detection:
  languages:
    - name: typescript
      confidence: high
      primary: true
    - name: python
      confidence: medium
      primary: false
  runtime: node
```

## HISTORY
| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-11 | Cosca Bootstrap | Initial detector |
