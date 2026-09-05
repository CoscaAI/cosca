# WORKFLOWS — Fluxos da Matriz

> O pipeline mestre (README) é a espinha dorsal. Estes são os workflows de domínio.

## Pipeline mestre (feature completa)

```
PRODUCT ANALYSIS → ARCHITECTURE → DATABASE → API → BACKEND → FRONTEND → MOBILE → SECURITY → TEST → PERFORMANCE → OBSERVABILITY → REVIEW → APPROVAL
```

## Workflows de domínio

### 1. product-feature
Nova funcionalidade de produto:
`PRODUCT ANALYSIS (design) → UX audit → IA → direção visual → especificação de interação`

### 2. full-stack-feature
Feature ponta a ponta: pipeline mestre completo, cada fase consultando o stack certo.

### 3. backend-feature
`DOMAIN → use case → transactions → idempotency → API surface → resiliência → teste`

### 4. api-change
`contrato atual → clients → compatibilidade → versão/migração → teste de contrato`

### 5. database-change
`modelo → migração (forward-only) → compatibilidade schema → índices → EXPLAIN → deploy`

### 6. mobile-feature
`DEVICE → USER CONTEXT → NETWORK → BATTERY → INPUT → LIFECYCLE → SECURITY → plataforma`

### 7. security-review
`ASSETS → ACTORS → TRUST BOUNDARIES → ATTACK SURFACES → THREATS → MITIGATIONS → RESIDUAL RISK` + scan (semgrep/gitleaks/trivy)

### 8. performance-review
`MEASURE → PROFILE → IDENTIFY → HYPOTHESIS → CHANGE → BENCHMARK → VERIFY` (nunca por intuição)

### 9. production-incident
`DETECT → TRIAGE → MITIGATE → COMMUNICATE → RECOVER → ROOT CAUSE → PREVENT` (postmortem sem culpa)

### 10. ecommerce-review
Auditoria de projeto de comércio (stack 16): `BUSINESS → DOMAIN → ARCHITECTURE → DATA MODEL → API → CATALOG → PRICING → INVENTORY → CART → CHECKOUT → PAYMENT → ORDER → SHIPPING → RETURNS → SECURITY → PERFORMANCE → OBSERVABILITY → UX → SEO → MOBILE → OPERATIONS → RECOMMEND` + maturidade (0-5) + quality gate

### 11. media-pipeline
Processamento de mídia (stack 18): `INPUT → MEDIA ANALYZER (tipo pelo conteúdo) → TASK PLANNER (pipeline pelo conteúdo) → EXECUTE (remove-bg/ocr/vectorize/restore/convert) → VALIDATION → OUTPUT`. Nunca rasterizar vetor; IA faz 95% → refinamento faz o produto.

### 12. video-media-pipeline
Processamento de vídeo/áudio (stack 19): `UNDERSTAND → ANALYZE → PLAN (codec/container/resolução/FPS/bitrate/CPU-GPU) → PROCESS → TRACK (consistência temporal) → VALIDATE (sync/artifacts) → OPTIMIZE → EXPORT`. Vídeo é dado temporal — nunca frames independentes.

### 13. audio-pipeline
Processamento de áudio (stack 20): `ANALYZE (noise/hum/clipping/voice) → DSP PLAN (só o necessário) → PROCESS (denoise/dehum/EQ/loudness-LUFS) → VALIDATE (sync/loudness) → PRESERVE (original)`. STT→diarização→legendas e separação de stems como pipelines compostos.

## Regras dos workflows

1. Todo workflow termina em REVIEW + APPROVAL — nada vai pra produção sem as duas.
2. Cada fase produz artefato no formato unificado (`_SCHEMA.md`).
3. A saída de uma fase é a entrada da próxima (ex.: ARCHITECTURE produz o ADR que DATABASE consome).
4. Se uma fase não se aplica, registrar explicitamente "não se aplica — motivo" (evita pulo silencioso).
