# WORKFLOW: platform-bootstrap

> **Version**: 1.0.0 | **Category**: init | **Estimated Duration**: 3-10 days | **Status**: active | **Owner**: Platform Chief | **Last Updated**: 2026-07-23

## OBJECTIVE
Bootstrap the Internal Developer Platform (IDP) for an organization or team. Establishes golden paths, self-service tooling, CI/CD standards, developer portal, and platform documentation.

## INPUTS
| Name | Type | Required | Description |
|------|------|----------|-------------|
| organization_name | String | Yes | Organization or team name |
| team_size | Number | Yes | Number of developers the platform will serve |
| tech_stack | String[] | Yes | Primary technology stack (languages, frameworks) |
| initial_features | String[] | No | Priority platform features to implement first |
| cloud_provider | String | No | Primary cloud provider (aws, gcp, azure) |

## OUTPUTS
| Name | Type | Description |
|------|------|-------------|
| Platform architecture | Document | IDP architecture and design decisions |
| Golden path templates | Templates | Standard project templates with CI/CD |
| CI/CD pipeline templates | Config | Standardized pipeline configurations |
| Developer portal | Web app | Service catalog and documentation |
| Platform documentation | Document | Setup, usage, and operations guides |

## PRECONDITIONS
1. Executive sponsorship secured for platform investment
2. Team of platform engineers allocated
3. Cloud infrastructure budget approved
4. Existing development pain points documented

## POSTCONDITIONS
1. Developer portal operational and accessible to all teams
2. Golden paths documented for 80%+ of common scenarios
3. CI/CD templates deployed and validated
4. Platform metrics dashboard operational
5. Team trained on platform usage

## STEPS
### Step 1: Discovery & Assessment
- **Chief**: Platform Chief
- **Specialists**: Developer Tooling Engineer
- **Task**: Interview teams, document pain points, assess current tooling
- **Output**: Discovery report and requirements

### Step 2: Platform Architecture Design
- **Chief**: Platform Chief
- **Specialists**: Architecture Chief, Portal Engineer
- **Task**: Design IDP architecture, select tools (Backstage, etc.)
- **Output**: Platform architecture document

### Step 3: Developer Portal Setup
- **Chief**: Platform Chief
- **Specialists**: Portal Engineer
- **Task**: Deploy developer portal, configure service catalog, TechDocs
- **Output**: Operational developer portal

### Step 4: Golden Path Creation
- **Chief**: Platform Chief
- **Specialists**: Developer Tooling Engineer, Platform Engineer
- **Task**: Create project templates, CI/CD pipelines, deployment templates
- **Output**: Golden path templates

### Step 5: CI/CD Platform Configuration
- **Chief**: DevOps Chief
- **Specialists**: Platform Engineer
- **Task**: Configure CI/CD platform, pipeline templates, quality gates
- **Output**: CI/CD platform with templates

### Step 6: Self-Service Actions
- **Chief**: Platform Chief
- **Specialists**: Developer Tooling Engineer
- **Task**: Implement self-service actions in developer portal (create repo, provision env, deploy)
- **Output**: Self-service action catalog

### Step 7: Documentation & Training
- **Chief**: Documentation Chief
- **Specialists**: Platform Documenter
- **Task**: Write platform documentation, create training materials, onboard teams
- **Output**: Platform documentation

### Step 8: Launch & Iterate
- **Chief**: Platform Chief
- **Specialists**: — 
- **Task**: Launch platform, collect feedback, plan improvements
- **Output**: Platform launch and feedback plan

## VALIDATION
1. Developer portal accessible and functional
2. Golden path templates generate working projects
3. CI/CD pipelines execute successfully end-to-end
4. Self-service actions complete without manual intervention
5. Developer satisfaction baseline measured (DXI)

## SUCCESS CRITERIA
- [ ] Developer portal deployed and operational
- [ ] Golden paths cover 80%+ of common scenarios
- [ ] CI/CD templates validated with test projects
- [ ] Self-service actions implemented and tested
- [ ] Platform documentation published
- [ ] Developer satisfaction baseline established

## ERROR HANDLING
| Failure | Action |
|---------|--------|
| Developer portal setup fails | Escalate to Platform Chief, consider alternative tools |
| Golden path adoption low | Survey teams, iterate on templates based on feedback |
| CI/CD integration issues | Escalate to DevOps Chief for pipeline troubleshooting |
| Platform budget exceeded | Escalate to CTO for additional budget approval |

## RELATED
- [Platform Chief](../departments/platform/SKILL.md)
- [Project Bootstrap skill](../skills/platform/PROJECT_BOOTSTRAP.md)
- [project-init.md](./project-init.md)
- [DevOps Chief](../departments/devops/SKILL.md)

## HISTORY
| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-23 | Cosca Enterprise Evolution | Initial workflow creation |
