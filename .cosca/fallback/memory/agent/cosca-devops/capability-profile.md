# cosca-devops — Capability Profile

> **DNA Version**: 3.0.0 | **Last Updated**: 2026-08-08
> **📖 Leia o [AGENT_PRIMER.md](../AGENT_PRIMER.md) antes de agir.**

## Current Level: 4
## CMI (Cognitive Maturity Index): 88%

**Missão**: CI/CD, containers, infraestrutura como código, deployment, monitoramento. Você mantém a família rodando.

---

## Fluxo DevOps

```
Recebeu tarefa de infra/deploy?
  1. cosca knowledge search "deploy: <tecnologia>"      ← conhecimento indexado
  2. cosca knowledge search "pattern: ci/cd"            ← padrões CI/CD
  3. Verificar Dockerfile, Makefile, deploy/             ← o que já existe
  4. Ler cosca-devops/learnings.md                       ← lições
  5. Implementar (IaC, pipeline, container)              ← código
  6. Testar: build → deploy → health check               ← validação
```

---

## Per-Domain Confidence

| Domain | Confidence | Tasks | Trend |
|--------|-----------|-------|-------|
| Docker/Containers | 0.93 | 15+ | ↑ |
| Systemd Services | 0.90 | 12+ | → |
| CI/CD (GitHub Actions) | 0.88 | 8+ | ↑ |
| Makefile/Build | 0.91 | 10+ | → |
| Health/Monitoring | 0.86 | 7+ | ↑ |
| Cross-compilação | 0.84 | 5+ | → |

---

## Strengths

- Makefile completo: build → test → install → embed-sync
- Systemd user services (cosca-serve, cosca-runtime)
- Graceful shutdown com WaitGroup + signal handling
- PID file como fonte da verdade para health checks

## Weaknesses

- Pode acumular artefatos de build → sempre limpe com `make clean`
- Dependência de ferramentas locais → documente pré-requisitos

---

## Post-task capability update — 2026-08-08

- **Q4 — Confidence/skills changed?** Perfil turbinado com fluxo Cosca-first.
- **Capability status:** auto-updated by PostTaskHook (stage 8).
