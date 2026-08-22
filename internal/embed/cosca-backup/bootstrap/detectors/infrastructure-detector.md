# INFRASTRUCTURE DETECTOR

> **Version**: 1.0.0 | **Status**: active | **Owner**: Cosca Bootstrap

## PURPOSE
Detect infrastructure configuration including containers, orchestration, CI/CD, cloud providers, and environment setup.

## DETECTION RULES

### Container Detection
| File | Detection |
|------|-----------|
| Dockerfile, Dockerfile.* | Docker |
| docker-compose.yml, docker-compose.yaml, compose.yml | Docker Compose |
| .dockerignore | Docker |

### Orchestration
| Path/File | Detection |
|-----------|-----------|
| k8s/, kubernetes/, deploy/ (with .yml/.yaml) | Kubernetes |
| helm/ | Helm |
| charts/ | Helm Charts |
| terraform/, *.tf | Terraform |
| Pulumi.yaml | Pulumi |
| cdk.json, cdk.out/ | AWS CDK |
| serverless.yml | Serverless Framework |
| vercel.json | Vercel |
| netlify.toml | Netlify |
| fly.toml | Fly.io |
| render.yaml | Render |
| railway.json | Railway |

### CI/CD
| Path/File | Detection |
|-----------|-----------|
| .github/workflows/*.yml | GitHub Actions |
| .gitlab-ci.yml | GitLab CI |
| Jenkinsfile | Jenkins |
| .circleci/config.yml | CircleCI |
| .travis.yml | Travis CI |
| bitbucket-pipelines.yml | Bitbucket Pipelines |
| azure-pipelines.yml | Azure Pipelines |
| .drone.yml | Drone CI |

### Cloud Providers
| Dependency / Config | Detection |
|---------------------|-----------|
| @aws-sdk/*, aws-sdk | AWS |
| @google-cloud/* | GCP |
| @azure/*, azure | Azure |
| @supabase/supabase-js | Supabase |
| @vercel/*, vercel | Vercel |
| planetscale | PlanetScale |
| @neondatabase/serverless | Neon |
| fly.io | Fly.io |
| cloudflare, wrangler | Cloudflare |
| @digitalocean/* | DigitalOcean |
| @heroku/* | Heroku |

### Environment Configuration
| File | Detection |
|------|-----------|
| .env, .env.example, .env.local | Environment Variables |
| .env.production, .env.staging | Multi-environment |
| config/*.env, env/*.env | Organized config |
| .secrets, secrets/ | Secrets Management |
| vault/ | HashiCorp Vault |
| .doppler.yaml | Doppler |

### Package Manager
| File | Detection |
|------|-----------|
| package-lock.json | npm |
| yarn.lock | yarn |
| pnpm-lock.yaml | pnpm |
| bun.lockb | bun |
| Pipfile, Pipfile.lock | pipenv |
| poetry.lock | poetry |
| Cargo.lock | cargo |
| go.sum | go modules |
| Gemfile.lock | bundler |

## OUTPUT
```yaml
detection:
  container: docker
  compose: docker-compose
  orchestration: null
  ci: github-actions
  cloud:
    primary: vercel
    database: supabase
  package_manager: pnpm
  environments:
    - development
    - production
```

## HISTORY
| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-11 | Cosca Bootstrap | Initial detector |
