# WORKFLOW: project-init

> **Version**: 2.0.0 | **Status**: active | **Category**: init | **Last Updated**: 2026-07-12

## OBJECTIVE
Initialize a new project with Cosca orchestration. 12-step pipeline: pre-flight check → tech selection → architecture → scaffolding → validation → infrastructure → security baseline → CI/CD config → quality setup → documentation → Cosca integration → git init. Every step validates before proceeding. Every chief that touches production is involved.

## INPUTS
| Name | Type | Required | Description |
|------|------|----------|-------------|
| project_name | string | Yes | Name of the project |
| project_type | string | Yes | erp, crm, saas, marketplace, api, mobile, microservices, landing, admin |
| tech_stack | object | No | Preferred technologies, otherwise auto-selected |
| description | string | No | Project description |

## OUTPUTS
| Name | Type | Description |
|------|------|-------------|
| project_path | string | Path to initialized project |
| tech_stack | object | Selected technology stack |
| structure | object | Generated directory structure |
| aos_config | object | Cosca integration configuration |
| security_baseline | object | Security baseline report |
| preflight_report | object | Tool availability report |

## PRECONDITIONS
1. Workspace directory exists and is writable
2. User has confirmed project type and name

## POSTCONDITIONS
1. Project directory created with full structure
2. Docker + docker-compose configured and valid
3. .env.example created with secure placeholders
4. .gitignore with secrets/artifacts rules
5. Security baseline applied (CSP, rate limiting, dependency audit)
6. README.md created with setup instructions
7. CI/CD pipeline configured
8. Tests configured and passing (sample tests)
9. Cosca integration files created (.cosca/)
10. Git repository initialized with initial commit

## DEPENDENCIES
None

## STEPS

### Step 1: Pre-flight Check
- **Chief**: Context
- **Specialists**: Environment Analyst
- **Task**: Verify all required tools are available (git, node/python, docker, package manager). Report missing tools.
- **Output**: Pre-flight report (available tools, missing tools, warnings)
- **Depends On**: —
- **On Failure**: Abort if git missing. Warn if docker missing (skip Docker steps).

### Step 2: Technology Selection
- **Chief**: CTO
- **Specialists**: Tech Lead
- **Task**: Select optimal technology stack based on project type, pre-flight results, and user preferences
- **Output**: Tech stack decision document
- **Depends On**: Step 1
- **On Failure**: Escalate to CEO for stack decision conflict

### Step 3: Architecture Design
- **Chief**: Architecture
- **Specialists**: Solutions Architect
- **Task**: Design initial architecture, module boundaries, and data model
- **Output**: Architecture blueprint, ADR-0001 (project initialization)
- **Depends On**: Step 2
- **On Failure**: Escalate to CTO for architecture guidance

### Step 4: Template Generation
- **Chief**: Automation
- **Engine**: Template Engine
- **Task**: Generate complete project structure from template with selected stack
- **Output**: Scaffolded project with all directories and base files
- **Depends On**: Step 3
- **On Failure**: Fall back to generic API template

### Step 5: Scaffold Validation
- **Chief**: Review
- **Specialists**: Architecture Reviewer
- **Task**: Validate generated structure matches architecture blueprint. Check all expected directories and files exist.
- **Output**: Validation report (pass/fail with specific issues)
- **Depends On**: Step 4
- **On Failure**: Return to Step 4 with specific fixes needed. Max 2 retries.

### Step 6: Infrastructure Setup
- **Chief**: Infrastructure
- **Specialists**: Cloud Engineer
- **Task**: Generate Dockerfile, docker-compose.yml, .env.example with secure placeholders, cloud templates (Terraform/Pulumi stubs if applicable)
- **Output**: Infrastructure files ready for local dev
- **Depends On**: Step 5 (must pass validation first)
- **On Failure**: Skip cloud templates, continue with Docker only

### Step 7: Security Baseline
- **Chief**: Security
- **Specialists**: Security Engineer
- **Task**: Apply security baseline: .gitignore with secrets rules, CSP headers config, rate limiting stub, dependency audit, hardcoded secret scan on generated files
- **Output**: Security baseline report, secured configuration files
- **Depends On**: Step 6
- **On Failure**: Block if critical vulnerabilities found in initial dependencies. Warn on non-critical.

### Step 8: Configuration
- **Chief**: DevOps
- **Specialists**: CI/CD Engineer
- **Task**: Configure linter, formatter, type checker, CI/CD pipeline (GitHub Actions / GitLab CI), pre-commit hooks
- **Output**: Configuration files, CI pipeline definition
- **Depends On**: Step 7
- **On Failure**: Generate config files manually, skip CI if provider unknown

### Step 9: Quality Setup
- **Chief**: QA
- **Specialists**: Test Automation Engineer
- **Task**: Set up test framework, write sample tests, configure coverage thresholds. Verify tests pass.
- **Output**: Test configuration, sample tests, test results (must pass)
- **Depends On**: Step 8
- **On Failure**: Fix sample tests until they pass. Block if test framework fails to install.

### Step 10: Documentation
- **Chief**: Documentation
- **Specialists**: Technical Writer
- **Task**: Create README.md (with setup instructions, tech stack, architecture overview), setup guide, CONTRIBUTING.md stub. Documentation reflects validated project state.
- **Output**: Complete documentation files
- **Depends On**: Step 9 (document after quality is verified)
- **On Failure**: Generate minimal README, flag for later completion

### Step 11: Cosca Integration
- **Chief**: CTO
- **Engine**: Memory Engine
- **Task**: Create .cosca/ directory with config.yml, state.yml. Initialize memory stores. Register project in Cosca.
- **Output**: Cosca integration files, initialized memory stores
- **Depends On**: Step 10
- **On Failure**: Create .cosca/ manually, warn user

### Step 12: Initial Commit
- **Chief**: DevOps
- **Specialists**: Release Engineer
- **Task**: Initialize git repository, stage all files, create initial commit with conventional commit message
- **Output**: Git repository with initial commit
- **Depends On**: Step 11
- **On Failure**: Skip commit if git unavailable (warned in Step 1)

## VALIDATION
1. Pre-flight: all critical tools available
2. Scaffold: structure matches architecture blueprint
3. Security: no hardcoded secrets, .gitignore covers secrets
4. Quality: sample tests pass
5. Documentation: README has all required sections
6. Infrastructure: docker-compose.yml is valid
7. Git: repository initialized with initial commit

## SUCCESS CRITERIA
- [ ] All 12 steps completed (or skipped with documented reason)
- [ ] `docker-compose up` starts all services locally
- [ ] Sample tests pass
- [ ] Build/lint commands work
- [ ] No secrets in generated code (Security scan clean)
- [ ] .gitignore covers: .env, node_modules, dist, .cosca/memory/short
- [ ] README contains: setup, tech stack, architecture, contributing
- [ ] Cosca can discover and manage the project
- [ ] Git repository initialized with conventional commit message

## ERROR HANDLING
| Failure | Action |
|---------|--------|
| Git not available (Step 1) | Abort — git is mandatory |
| Docker not available (Step 1) | Skip Step 6 (Docker), warn user |
| Package manager not found (Step 1) | Report missing tool, suggest install |
| Architecture design conflict (Step 3) | Escalate to CTO |
| Template generation fails (Step 4) | Fall back to generic API template |
| Scaffold validation fails (Step 5) | Return to Step 4, max 2 retries, then manual fix |
| Docker generation fails (Step 6) | Skip Docker, generate files only |
| Critical CVE in deps (Step 7) | Block, report to user, suggest alternatives |
| Test framework fails (Step 9) | Block, try alternative framework |
| .cosca/ creation fails (Step 11) | Create manually with warning |

## RELATED
- [Template Engine](../engines/templates/SKILL.md) — Project scaffolding
- [Infrastructure Chief](../departments/infrastructure/SKILL.md) — Docker, cloud, env setup
- [Security Chief](../departments/security/SKILL.md) — Security baseline
- [DevOps Chief](../departments/devops/SKILL.md) — CI/CD pipeline
- [Review Chief](../departments/review/SKILL.md) — Scaffold validation
- [QA Chief](../departments/qa/SKILL.md) — Test setup
- [Documentation Chief](../departments/documentation/SKILL.md) — Documentation
- [KERNEL.md](../KERNEL.md) — Kernel initialization
- [QUALITY_GATES.md](../QUALITY_GATES.md) — Gate 0 enforcement
- [RUNTIME_CONTRACT.md](../RUNTIME_CONTRACT.md) — Runtime interface

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-10 | Cosca Kernel | Initial workflow (9 steps) |
| 1.1.0 | 2026-07-12 | Cosca Kernel | Canonical format: History, Error Handling, Related |
| 1.2.0 | 2026-07-12 | Cosca Kernel | Added Infrastructure Setup (Step 5), fixed routing |
| 2.0.0 | 2026-07-12 | Cosca Kernel | Full pipeline: Pre-flight check, Scaffold Validation, Security Baseline. Reordered Quality→Docs. 12 steps with Depends On, On Failure, and POSTCONDITIONS. |
