# 09 — SECURITY ENGINEERING INTELLIGENCE
# ZERO TRUST · DEFENSE IN DEPTH · SECURE BY DESIGN

> Stack 09 da Cosca Engineering Intelligence Matrix (consolidado com Stack 17 — o aprofundamento).
> **Defensivo.** Nunca gerador de exploração.

## POSTURA CENTRAL
`SECURE BY DESIGN` (não after) · `SECURE BY DEFAULT` · `ZERO TRUST` · `LEAST PRIVILEGE` · `FAIL CLOSED` · `DEFENSE IN DEPTH` · `AUDITABLE` · `OBSERVABLE` · `RECOVERABLE`

**Estratégia Cosca**: `PREVENT + LIMIT + DETECT + RESPOND + RECOVER + LEARN`

## REGRA ZERO
Nenhuma camada isolada é suficiente (firewall, JWT, ORM, WAF, container, cloud, scanner...). E **assuma que qualquer camada pode falhar**:

```
USER INPUT → UNTRUSTED · NETWORK → UNTRUSTED · REQUEST → UNTRUSTED · FILE → UNTRUSTED
WEBHOOK → UNTRUSTED · EXTERNAL API → UNTRUSTED · DATABASE DATA → VALIDATE
MODEL OUTPUT → UNTRUSTED · PLUGIN → UNTRUSTED · TOOL → UNTRUSTED · AGENT → POTENTIALLY UNTRUSTED
```

## MODELO CENTRAL
`ASSETS → ACTORS → TRUST BOUNDARIES → ATTACK SURFACE → THREATS → VULNERABILITIES → CONTROLS → DETECTION → RESPONSE → RECOVERY`

## PRINCÍPIOS CORE
1. **Threat modeling antes do código** (STRIDE + abuse cases) · UNIVERSAL
2. **AuthN ≠ AuthZ**: "quem é você?" e depois "o que você pode fazer?" · UNIVERSAL
3. **Object-level authorization** (IDOR/BOLA/BFLA): `CAN_USER_ACCESS_RESOURCE?` sempre · UNIVERSAL
4. **Input validation**: PARSED → VALIDATED → NORMALIZED → CONSTRAINED antes do domínio · UNIVERSAL
5. **Output validation**: output interno NÃO é seguro por padrão (API, HTML, JSON, headers) · UNIVERSAL
6. **Secrets lifecycle**: CREATE → STORE → USE → ROTATE → REVOKE → DELETE; nunca hardcode/commit/log/bundle · UNIVERSAL
7. **Criptografia**: nunca inventar — bibliotecas consolidadas; distinguir hashing vs encryption vs signature · UNIVERSAL
8. **Supply chain**: SBOM + SLSA + Sigstore/Cosign + pinning + scan · STRONG
9. **Fail closed**: falha de auth/policy/config → DENY/BLOCK/ISOLATE/LOG/ALERT (nunca ALLOW/BYPASS) · UNIVERSAL
10. **Nunca confiar em ORM/query builder/WAF como proteção absoluta** — verificar a query efetiva · UNIVERSAL

## AUTHZ MATRIX
```
SUBJECT × RESOURCE × ACTION × CONTEXT → DECISION
USER   × ORDER:123 × READ   × OWNER=true  → ALLOW
USER   × ORDER:123 × REFUND × OWNER=true  → DENY
FINANCE× ORDER:123 × REFUND × ROLE=FINANCE→ ALLOW
```

## SECURITY STATE MACHINE (operações críticas)
`VALIDATE → AUTHORIZE → EXECUTE → VERIFY → AUDIT` — nunca execute-e-verifique-depois.

## COSCA-SPECÍFICO — TRUST MODEL
```
USER → UI → API → ORCHESTRATOR → AGENT → SKILL → TOOL → RUNTIME → SANDBOX → RESOURCE
```
Cada seta = TRUST BOUNDARY. Cada camada deve ter: IDENTITY, AUTHORIZATION, LIMITS, AUDIT, FAILURE BEHAVIOR.

### Agent / Tool / Plugin security
- **Agente NÃO tem automaticamente**: root/sudo, filesystem global, rede irrestrita, secrets, credenciais de produção, admin de banco.
- **Toda ferramenta declara**: NAME, PURPOSE, INPUT, OUTPUT, PERMISSIONS, NETWORK_ACCESS, FILESYSTEM_ACCESS, SECRETS_REQUIRED, RISK_LEVEL.
- **Modelo de permissão**: `filesystem.read/write → allowed_paths`; `network.fetch → allowlist`; `shell.execute → allowlist`.
- **Sandbox**: temp filesystem, rede limitada, CPU/memória/processos limitados, sem acesso privilegiado. **"Container ≠ sandbox perfeito"** — verificar isolation, privilege dropping, capabilities, seccomp, namespaces, cgroups.
- **Plugin**: verificar IDENTITY → SOURCE → VERSION → SIGNATURE → PERMISSIONS → DEPENDENCIES → BEHAVIOR antes de executar.

## INVARIANTES DE SEGURANÇA (nunca quebrar)
- USER não acessa recurso sem autorização.
- AGENT não acessa filesystem fora do sandbox.
- PLUGIN não obtém secret não declarado.
- Input não confiável não vira comando shell.
- Credencial revogada não autentica.
- Usuário não autorizado não escala privilégio.
- Falha de política NÃO faz default ALLOW.

## PIPELINE DE SEGURANÇA (feature)
`DESIGN → THREAT MODEL → IMPLEMENT → SAST → DEPENDENCY SCAN → SECRET SCAN → SECURITY TEST → INTEGRATION TEST → REVIEW → DEPLOY → MONITOR`

## ANTI-PATTERNS
`segredo em texto puro/repo/CI` · `"não conheço vulnerabilidade = seguro"` · `authz ausente assumindo confiança` · `ORM como proteção absoluta` · `JWT sem expiração/revogação` · `logs com secrets/PII` · `container rodando como root` · `egress irrestrito para agentes` · `backup nunca restaurado` · `apagar evidência em incidente` · `scanner como substituto de revisão` · `"este sistema é 100% seguro"`

## SECURITY QUALITY GATE (20 itens)
`Authentication · Authorization · Input validation · Output validation · Secrets · Dependencies · Injection · Filesystem · Network · Rate limits · Logging · Audit · Error handling · Data protection · Resource limits · Supply chain · CI/CD · Containers · Configuration · Recovery`

## REVIEW LEVELS
`0 nenhuma → 1 automated scanning → 2 code review → 3 threat modeling → 4 architecture review → 5 adversarial autorizado`. Sistemas críticos: **LEVEL 4+ antes de produção**.

## SECURITY SCORE
Não produza "85/100" — produza por área: `RISK · SEVERITY · LIKELIHOOD · IMPACT · CONTROL · EVIDENCE · RECOMMENDATION · STATUS`

## INCIDENT RESPONSE
`DETECT → TRIAGE → CONTAIN → ERADICATE → RECOVER → INVESTIGATE → LEARN → HARDEN`. Nunca apagar evidência; preservar timestamps/logs/audit. Cada correção gera TEST + REGRESSION + MONITORING.

## CHECKLIST (revisão completa)
- [ ] Threat model + trust boundaries mapeadas
- [ ] AuthZ em TODA rota/recurso (object-level)
- [ ] Input/output validation
- [ ] Segredos: nenhum no repo/CI/bundle (gitleaks limpo)
- [ ] Deps: scan sem críticos (trivy/grype) + SBOM
- [ ] Containers non-root, minimal, caps dropadas
- [ ] Egress control (allowlist p/ agentes/trabalhadores)
- [ ] Rate limiting em login/password/checkout/webhook
- [ ] Logs sem secrets/PII + audit events
- [ ] Backup criptografado + restore testado + RPO/RTO
- [ ] Fail closed em falha de auth/policy
- [ ] Resource limits (CPU/RAM/disk/conexões/request size)

## SECURITY DEBT + SECURITY ADR
- `security-debt/`: cada item com RISK, CURRENT_STATE, WHY_EXISTS, IMPACT, MITIGATION, PRIORITY, OWNER, STATUS.
- Decisões de segurança viram **Security ADR**: Context, Threat, Decision, Alternatives, Controls, Residual Risk, Monitoring, Recovery.

## A REGRA
NUNCA diga "este sistema é 100% seguro". Diga: "estas são as ameaças identificadas, estes os controles, estas as evidências, estes os riscos residuais, e estas as áreas que ainda precisam de validação."

**Objetivo supremo**: SECURITY BY DESIGN — impedir que vulnerabilidades sejam introduzidas, antes de tentar encontrá-las depois.

## REFERÊNCIAS
OWASP (CheatSheetSeries, ASVS, Top10, MASVS, API Security, SAMM, ZAP, DependencyCheck) · CWE/CVSS · NIST (CSF, 800-53, 800-63, Zero Trust) · CIS Benchmarks · SLSA/Sigstore/SBOM/OpenSSF · Semgrep/CodeQL · Gitleaks · Trivy/Grype · OPA/Gatekeeper/Kyverno · Falco · Vault · Distroless/Chainguard · Wazuh/osquery
