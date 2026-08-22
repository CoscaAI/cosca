> **Version**: 3.1.0 | **Last Updated**: 2026-08-17 | **Status**: active | **Owner**: Skills Engine
> 
> # Cosca SKILLS CATALOG
> 
> ## Purpose
> Comprehensive catalog of all reusable skills in the Cosca framework. Skills are specialized instructions that provide detailed workflows for specific tasks. They are loaded by agents when a task matches the skill's description.
>
> ## ⚠️ FONTE DA VERDADE: o Inventário Real (seção final deste arquivo — 88 skills, 29 categorias, sincronizado do disco em 2026-08-17). As seções abaixo deste aviso são LEGADO (v1.0/v3.0, 2026-07-23) e estão OBSOLETAS — não refletem o disco. Use sempre o Inventário Real no fim do arquivo (L409: audit sincronizou e marcou o legado).
>
> ## 🗑️ LEGADO OBSOLETO — não usar (ver Inventário Real no fim)
>
> ### Backend Implementation (7)
> | Skill | File | Stack |
> |-------|------|-------|
> | Go API | [backend/go/GO_API_IMPLEMENTATION.md](backend/go/GO_API_IMPLEMENTATION.md) | Go, chi, SQL, JWT |
> | Go Security | [backend/go/GO_SECURITY.md](backend/go/GO_SECURITY.md) | JWT, rate limit, secrets |
> | Rust API | [backend/rust/RUST_API_IMPLEMENTATION.md](backend/rust/RUST_API_IMPLEMENTATION.md) | Axum, sqlx, tokio |
> | Python API | [backend/python/PYTHON_API_IMPLEMENTATION.md](backend/python/PYTHON_API_IMPLEMENTATION.md) | FastAPI, Pydantic v2 |
> | Node.js API | [backend/node/NODE_API_IMPLEMENTATION.md](backend/node/NODE_API_IMPLEMENTATION.md) | Express, Zod, Helmet |
> | Java API | [backend/java/JAVA_API_IMPLEMENTATION.md](backend/java/JAVA_API_IMPLEMENTATION.md) | Spring Boot, JPA |
> | C# .NET API | [backend/dotnet/DOTNET_API_IMPLEMENTATION.md](backend/dotnet/DOTNET_API_IMPLEMENTATION.md) | ASP.NET Core, EF Core |
>
> ### Mobile (4)
> | Skill | File | Stack |
> |-------|------|-------|
> | SwiftUI | [mobile/swift/SWIFTUI_IMPLEMENTATION.md](mobile/swift/SWIFTUI_IMPLEMENTATION.md) | Swift, MVVM, Keychain |
> | Android | [mobile/kotlin/ANDROID_IMPLEMENTATION.md](mobile/kotlin/ANDROID_IMPLEMENTATION.md) | Kotlin, Compose, Hilt |
> | Flutter | [mobile/flutter/FLUTTER_IMPLEMENTATION.md](mobile/flutter/FLUTTER_IMPLEMENTATION.md) | Dart, Riverpod |
> | React Native | [mobile/react-native/RN_IMPLEMENTATION.md](mobile/react-native/RN_IMPLEMENTATION.md) | Expo, TypeScript |
>
> ### Frontend (4)
> | Skill | File | Stack |
> |-------|------|-------|
> | React/Next.js | [frontend/react/REACT_IMPLEMENTATION.md](frontend/react/REACT_IMPLEMENTATION.md) | React 19, TanStack Query |
> | Vue/Nuxt | [frontend/vue/VUE_IMPLEMENTATION.md](frontend/vue/VUE_IMPLEMENTATION.md) | Vue 3, Composition API |
> | Svelte/SvelteKit | [frontend/svelte/SVELTE_IMPLEMENTATION.md](frontend/svelte/SVELTE_IMPLEMENTATION.md) | Svelte 5 runes |
> | Angular | [frontend/angular/ANGULAR_IMPLEMENTATION.md](frontend/angular/ANGULAR_IMPLEMENTATION.md) | Angular 18, Signals |
>
> ### Security (1)
> | Skill | File | Scope |
> |-------|------|-------|
> | OWASP Top 10 2025 | [security/OWASP_TOP10_2025.md](security/OWASP_TOP10_2025.md) | 6 linguagens, defesa por categoria |
>
> ### Embedded + Robotics (3)
> | Skill | File | Scope |
> |-------|------|-------|
> | Microcontrollers | [embedded/MICROCONTROLLER_C.md](embedded/MICROCONTROLLER_C.md) | C/C++, ESP32, FreeRTOS |
> | PCB Design | [embedded/PCB_DESIGN.md](embedded/PCB_DESIGN.md) | KiCad, gerber, BOM |
> | ROS2 + Robotics | [robotics/ROS2_IMPLEMENTATION.md](robotics/ROS2_IMPLEMENTATION.md) | ROS2, PID, CV, safety |
>
> ### Platform (1)
> | Skill | File | Scope |
> |-------|------|-------|
> | Cosca Integration | [platform/COSCA_PLATFORM_INTEGRATION.md](platform/COSCA_PLATFORM_INTEGRATION.md) | Knowledge, CI/CD, agents, stack detection |
>
> ---
> 
> ## Skill Categories
> 
> ### Architecture (5)
> | Skill | File | Purpose |
> |-------|------|---------|
> | Architecture Analysis | [architecture/ARCHITECTURE_ANALYSIS.md](architecture/ARCHITECTURE_ANALYSIS.md) | Analyze system architecture, detect patterns, violations |
> | Architecture Validation | [architecture/ARCHITECTURE_VALIDATION.md](architecture/ARCHITECTURE_VALIDATION.md) | Validate code against architectural rules and ADRs |
> | Architecture Documentation | [architecture/ARCHITECTURE_DOCUMENTATION.md](architecture/ARCHITECTURE_DOCUMENTATION.md) | Generate architecture documentation from code |
> | Dependency Analysis | [architecture/DEPENDENCY_ANALYSIS.md](architecture/DEPENDENCY_ANALYSIS.md) | Analyze dependency graphs, detect cycles, violations |
> | ADR Generation | [architecture/ADR_GENERATION.md](architecture/ADR_GENERATION.md) | Generate Architecture Decision Records |
> 
> ### Code Quality (4)
> | Skill | File | Purpose |
> |-------|------|---------|
> | Code Review | [code-quality/CODE_REVIEW.md](code-quality/CODE_REVIEW.md) | Multi-dimensional code review |
> | Refactoring | [code-quality/REFACTORING.md](code-quality/REFACTORING.md) | Systematic code refactoring |
> | Technical Debt Analysis | [code-quality/TECHNICAL_DEBT_ANALYSIS.md](code-quality/TECHNICAL_DEBT_ANALYSIS.md) | Identify and measure technical debt |
> | Complexity Analysis | [code-quality/COMPLEXITY_ANALYSIS.md](code-quality/COMPLEXITY_ANALYSIS.md) | Analyze code complexity metrics |
> 
> ### Security (4)
> | Skill | File | Purpose |
> |-------|------|---------|
> | Security Audit | [security/SECURITY_AUDIT.md](security/SECURITY_AUDIT.md) | Comprehensive security audit |
> | Vulnerability Assessment | [security/VULNERABILITY_ASSESSMENT.md](security/VULNERABILITY_ASSESSMENT.md) | Identify and assess vulnerabilities |
> | Secrets Audit | [security/SECRETS_AUDIT.md](security/SECRETS_AUDIT.md) | Detect hardcoded secrets and credentials |
> | Compliance Validation | [security/COMPLIANCE_VALIDATION.md](security/COMPLIANCE_VALIDATION.md) | Validate compliance with security standards |
> 
> ### Performance (3)
> | Skill | File | Purpose |
> |-------|------|---------|
> | Performance Audit | [performance/PERFORMANCE_AUDIT.md](performance/PERFORMANCE_AUDIT.md) | Comprehensive performance analysis |
> | Load Testing | [performance/LOAD_TESTING.md](performance/LOAD_TESTING.md) | Plan and execute load tests |
> | Database Performance | [performance/DATABASE_PERFORMANCE.md](performance/DATABASE_PERFORMANCE.md) | Database query and schema optimization |
> 
> ### Testing (4)
> | Skill | File | Purpose |
> |-------|------|---------|
> | Unit Testing | [testing/UNIT_TESTING.md](testing/UNIT_TESTING.md) | Write unit tests following AAA pattern |
> | Integration Testing | [testing/INTEGRATION_TESTING.md](testing/INTEGRATION_TESTING.md) | Plan and execute integration tests |
> | E2E Testing | [testing/E2E_TESTING.md](testing/E2E_TESTING.md) | End-to-end testing strategy |
> | Contract Testing | [testing/CONTRACT_TESTING.md](testing/CONTRACT_TESTING.md) | API contract testing (Pact, Spring Cloud Contract) |
> 
> ### Documentation (3)
> | Skill | File | Purpose |
> |-------|------|---------|
> | Documentation Update | [documentation/DOCUMENTATION_UPDATE.md](documentation/DOCUMENTATION_UPDATE.md) | Update project documentation |
> | API Documentation | [documentation/API_DOCUMENTATION.md](documentation/API_DOCUMENTATION.md) | Generate API documentation from contracts |
> | ADR Creation | [documentation/ADR_CREATION.md](documentation/ADR_CREATION.md) | Create Architecture Decision Records |
> 
> ### DevOps (3)
> | Skill | File | Purpose |
> |-------|------|---------|
> | CI/CD Validation | [devops/CICD_VALIDATION.md](devops/CICD_VALIDATION.md) | Validate CI/CD pipeline configuration |
> | Docker Validation | [devops/DOCKER_VALIDATION.md](devops/DOCKER_VALIDATION.md) | Validate Dockerfile and container config |
> | Kubernetes Validation | [devops/KUBERNETES_VALIDATION.md](devops/KUBERNETES_VALIDATION.md) | Validate Kubernetes manifests |
> 
> ### Data (3)
> | Skill | File | Purpose |
> |-------|------|---------|
> | Database Audit | [data/DATABASE_AUDIT.md](data/DATABASE_AUDIT.md) | Database schema and query audit |
> | Data Migration Planning | [data/DATA_MIGRATION_PLANNING.md](data/DATA_MIGRATION_PLANNING.md) | Plan data migrations |
> | Query Optimization | [data/QUERY_OPTIMIZATION.md](data/QUERY_OPTIMIZATION.md) | Analyze and optimize database queries |
> 
> ### AI (3)
> | Skill | File | Purpose |
> |-------|------|---------|
> | Prompt Engineering | [ai/PROMPT_ENGINEERING.md](ai/PROMPT_ENGINEERING.md) | Design and optimize AI prompts |
> | Provider Discovery | [ai/PROVIDER_DISCOVERY.md](ai/PROVIDER_DISCOVERY.md) | Discover and evaluate AI providers |
> | Embedding Pipeline | [ai/EMBEDDING_PIPELINE.md](ai/EMBEDDING_PIPELINE.md) | Build and optimize embedding pipelines |
> 
> ### Governance (3)
> | Skill | File | Purpose |
> |-------|------|---------|
> | Convention Validation | [governance/CONVENTION_VALIDATION.md](governance/CONVENTION_VALIDATION.md) | Validate against Cosca conventions |
> | Quality Gate | [governance/QUALITY_GATE.md](governance/QUALITY_GATE.md) | Execute quality gate checks |
> | Memory Synchronization | [governance/MEMORY_SYNCHRONIZATION.md](governance/MEMORY_SYNCHRONIZATION.md) | Synchronize memory across stores |
> 
> ### API (3)
> | Skill | File | Purpose |
> |-------|------|---------|
> | API Audit | [api/API_AUDIT.md](api/API_AUDIT.md) | API contract and implementation audit |
> | OpenAPI Validation | [api/OPENAPI_VALIDATION.md](api/OPENAPI_VALIDATION.md) | Validate OpenAPI specifications |
> | API Design Review | [api/API_DESIGN_REVIEW.md](api/API_DESIGN_REVIEW.md) | Review API design for consistency |
> 
> ### Platform (3)
> | Skill | File | Purpose |
> |-------|------|---------|
> | Project Bootstrap | [platform/PROJECT_BOOTSTRAP.md](platform/PROJECT_BOOTSTRAP.md) | Initialize new projects |
> | Provider Integration | [platform/PROVIDER_INTEGRATION.md](platform/PROVIDER_INTEGRATION.md) | Integrate new providers |
> | Configuration Validation | [platform/CONFIGURATION_VALIDATION.md](platform/CONFIGURATION_VALIDATION.md) | Validate configuration files |
> 
> ### Reliability (2)
> | Skill | File | Purpose |
> |-------|------|---------|
> | Disaster Recovery Planning | [reliability/DISASTER_RECOVERY.md](reliability/DISASTER_RECOVERY.md) | Plan disaster recovery procedures |
> | Incident Response | [reliability/INCIDENT_RESPONSE.md](reliability/INCIDENT_RESPONSE.md) | Structured incident response |
> 
> > **Total Skills: 46** | **Last updated**: 2026-07-23
> > **Maintained by**: Skills Engine | **Audited by**: Governance Chief

### Backend
- [GRPC_IMPLEMENTATION](backend/GRPC_IMPLEMENTATION.md) — gRPC server + client with interceptors, retry, streaming (v1.0.0)

### DevOps
- [HELM_DEPLOYMENT](devops/HELM_DEPLOYMENT.md) — Helm chart deployment + CI/CD + production hardening (v1.0.0)

### Performance
- [GO_BENCHMARKING](performance/GO_BENCHMARKING.md) — Go benchmarks + profiling (pprof, trace, benchstat) (v1.0.0)

---

## 🗂️ Inventário Real (gerado dos diretórios — 2026-08-17)

> O catálogo abaixo reflete o estado REAL dos arquivos de skill (88 arquivos, 29 categorias).
> **Este inventário substitui as contagens anteriores que estavam defasadas.**
> **FONTE DA VERDADE**: este inventário é gerado do disco (`find . -name '*.md' -not -name 'SKILLS_CATALOG.md'`) e bate 1:1 com os arquivos.

| Categoria | Skills | Arquivos |
|-----------|--------|----------|
| **ai** | 3 | EMBEDDING_PIPELINE, PROMPT_ENGINEERING, PROVIDER_DISCOVERY |
| **analytics** | 2 | EVENT_TRACKING, METRICS_DASHBOARD |
| **api** | 3 | API_AUDIT, API_DESIGN_REVIEW, OPENAPI_VALIDATION |
| **architecture** | 5 | ADR_GENERATION, ARCHITECTURE_ANALYSIS, ARCHITECTURE_DOCUMENTATION, ARCHITECTURE_VALIDATION, DEPENDENCY_ANALYSIS |
| **automation** | 2 | SCAFFOLDING, SCRIPT_GENERATION |
| **backend** | 8 | GRPC_IMPLEMENTATION, dotnet/DOTNET_API_IMPLEMENTATION, go/GO_API_IMPLEMENTATION, go/GO_SECURITY, java/JAVA_API_IMPLEMENTATION, node/NODE_API_IMPLEMENTATION, python/PYTHON_API_IMPLEMENTATION, rust/RUST_API_IMPLEMENTATION |
| **cache** | 2 | CACHE_PERFORMANCE, CACHE_STRATEGY |
| **code-quality** | 4 | CODE_REVIEW, COMPLEXITY_ANALYSIS, REFACTORING, TECHNICAL_DEBT_ANALYSIS |
| **context** | 1 | SESSION_CONTEXT |
| **cost** | 1 | COST_OPTIMIZATION |
| **data** | 3 | DATABASE_AUDIT, DATA_MIGRATION_PLANNING, QUERY_OPTIMIZATION |
| **devops** | 4 | CICD_VALIDATION, DOCKER_VALIDATION, HELM_DEPLOYMENT, KUBERNETES_VALIDATION |
| **discovery** | 1 | PROJECT_SCANNING |
| **documentation** | 3 | ADR_CREATION, API_DOCUMENTATION, DOCUMENTATION_UPDATE |
| **embedded** | 2 | MICROCONTROLLER_C, PCB_DESIGN |
| **frontend** | 6 | ACCESSIBILITY_AUDIT, COMPONENT_TESTING, angular/ANGULAR_IMPLEMENTATION, react/REACT_IMPLEMENTATION, svelte/SVELTE_IMPLEMENTATION, vue/VUE_IMPLEMENTATION |
| **governance** | 4 | CONVENTION_VALIDATION, MEMORY_SYNCHRONIZATION, POLICY_AUDIT, QUALITY_GATE |
| **messaging** | 2 | EVENT_SCHEMA_DESIGN, QUEUE_PATTERNS |
| **migration** | 2 | DATA_MIGRATION, SCHEMA_MIGRATION |
| **mobile** | 5 | REACT_NATIVE_AUDIT, flutter/FLUTTER_IMPLEMENTATION, kotlin/ANDROID_IMPLEMENTATION, react-native/RN_IMPLEMENTATION, swift/SWIFTUI_IMPLEMENTATION |
| **monitoring** | 1 | ALERT_CONFIGURATION |
| **performance** | 4 | DATABASE_PERFORMANCE, GO_BENCHMARKING, LOAD_TESTING, PERFORMANCE_AUDIT |
| **platform** | 4 | CONFIGURATION_VALIDATION, COSCA_PLATFORM_INTEGRATION, PROJECT_BOOTSTRAP, PROVIDER_INTEGRATION |
| **plugin** | 2 | PLUGIN_DEVELOPMENT, PLUGIN_SECURITY |
| **reliability** | 3 | BACKUP_TESTING, DISASTER_RECOVERY, INCIDENT_RESPONSE |
| **robotics** | 1 | ROS2_IMPLEMENTATION |
| **security** | 5 | COMPLIANCE_VALIDATION, OWASP_TOP10_2025, SECRETS_AUDIT, SECURITY_AUDIT, VULNERABILITY_ASSESSMENT |
| **testing** | 4 | CONTRACT_TESTING, E2E_TESTING, INTEGRATION_TESTING, UNIT_TESTING |
| **visual-media** | 1 | SKILL |

> **Total real: 88 arquivos de skill em 29 categorias** | **Atualizado**: 2026-08-17
