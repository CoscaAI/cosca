> **Version**: 1.0.0 | **Status**: active | **Owner**: Platform Chief | **Last Updated**: 2026-07-23
>
> # PROJECT BOOTSTRAP SKILL
>
> ## Description
> Use this skill to initialize new projects following Cosca standards. Sets up project structure, configuration, CI/CD, documentation, and development environment.
>
> ## Inputs
> | Input | Required | Description |
> |-------|----------|-------------|
> | project_type | Yes | `saas`, `api`, `microservices`, `cli`, `sdk`, `web-app`, `mobile`, `library` |
> | project_name | Yes | Name of the project |
> | tech_stack | Yes | `node-ts`, `python`, `go`, `java`, `rust`, `dotnet` |
> | features | No | Comma-separated: `docker`, `ci-cd`, `docs`, `tests`, `linting`, `monorepo` |
>
> ## Outputs
> | Output | Description |
> |--------|-------------|
> | Project scaffold | Complete project structure |
> | Configuration files | Linting, formatting, CI/CD configs |
> | Documentation | README, contributing guide, API docs |
> | Development environment | Docker, devcontainer, scripts |
>
> ## Process
> 1. Select project template matching project_type
> 2. Generate directory structure
> 3. Create all configuration files
> 4. Set up package manager and dependencies
> 5. Create CI/CD pipeline configuration
> 6. Generate documentation scaffolding
> 7. Set up development environment (Docker, devcontainer)
> 8. Create initial tests
> 9. Configure linting and formatting
> 10. Initialize version control (git)
>
> ## Bootstrap Output Structure
> ```
> project-name/
> ├── src/                    # Source code
> ├── tests/                  # Test files
> ├── docs/                   # Documentation
> ├── scripts/                # Development scripts
> ├── .github/                # GitHub CI/CD (if applicable)
> ├── docker/                 # Docker configuration
> ├── .gitignore
> ├── .editorconfig
> ├── README.md
> ├── CONTRIBUTING.md
> ├── CHANGELOG.md
> ├── package.json / pyproject.toml / go.mod
> └── Makefile / Justfile
> ```
>
> ## Success Criteria
> - [ ] Project structure generated
> - [ ] All config files created
> - [ ] CI/CD pipeline configured
> - [ ] Documentation scaffolded
> - [ ] Development environment configured
> - [ ] Initial tests passing
> - [ ] Linting and formatting configured
>
> ## Related
> - [Platform Chief](../../departments/platform/SKILL.md)
> - [workflows/project-init.md](../../workflows/project-init.md)
> - [Bootstrap Engine](../../engines/templates/SKILL.md)
> - [templates/](../../templates/) — Project templates
