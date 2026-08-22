# WORKSPACE STATE — Cosca v1.4.0-dev

> **Git**: `8eb40e8` (main, com mudanças locais) | **Updated**: 2026-08-01
> 
> Este arquivo modela **o estado do projeto**.
> Para **como o Kernel está pensando**, veja `cognitive-state.md`.

---

## 🔴 OPERAÇÃO EM CURSO — ROCm + Ollama local (independência de IA externa)

> **Estado**: fase 1 (ROCm) instalada — **aguardando reboot do Don** para validar GPU.
> **Ordem do Don**: todo comando `sudo` é enviado a ele; o Kernel valida. Kernel NUNCA roda root.

### Timeline
1. ✅ Diagnóstico: RX 6700 XT (gfx1031/RDNA2, 12GB), amdgpu nativo do kernel, /dev/kfd OK
2. ✅ Repo AMD: `/etc/apt/keyrings/rocm.gpg` + `/etc/apt/sources.list.d/rocm.list`
3. ✅ Pin: `/etc/apt/preferences.d/rocm-pin-1001` (resolveu conflito rocminfo Ubuntu 5.7.1 vs AMD 1.0.0.70204)
4. ✅ Instalado: **ROCm 7.2.4** — 30 pacotes (hip-runtime, rocblas, hipblas, rocfft, rccl, hsa-rocr...)
5. ✅ `cosca` adicionado a `render`+`video`
6. ⚠️ **PENDENTE**: reboot → validar `rocminfo` (esperado Agent gfx1031 + Marketing Name Radeon RX 6700 XT)
7. ⏭️ PRÓXIMO: instalar Ollama (HSA_OVERRIDE_GFX_VERSION se preciso), baixar modelo local (Qwen3 4B/Llama 3.2), configurar provider ollama + failover deepseek, corrigir user config órfão, teste E2E voz, commit

### ⚠️ CRÍTICO — user config órfão
`~/.config/cosca/config.yaml` está com `provider: name: ollama, model: deepseek-v4-flash, api_key fantasma, base_url vazio` — FOI A CAUSA DO APAGÃO DA VOZ ("Não consegui acessar o cérebro"). Corrigir depois que o Ollama estiver rodando. NUNCA trocar provider antes do substituto estar de pé.
Referências: decisão `2026-08-01-rocm-ollama-local.md`, learning L57.

---

## Architecture

| Componente | Estado |
|---|---|
| Stack | Go 1.25 + Next.js 15 + SQLite (modernc.org) |
| Module | github.com/CoscaAI/cosca |
| Version | 1.4.0-dev |
| Agents | 55 (45 chiefs + 9 specialists + kernel) |
| Activated | 54/55 (98%) — cosca-paradigm gated ate Out/2026 |
| CI | VERDE (-race + vet pass) |
| CIS Estimate | 88-90/100 |
| Confianca Media | ~0.62 |

---

## Features (ativas)

| Feature | Status |
|---|---|
| Auto-Jail | memfd_create + bwrap, workspace-scoped, binds minimos |
| Constraints | --no-network, --no-docs, --no-test, --no-build, --read-only, --max-files, --max-time |
| gRPC Auth | JWT AuthInterceptor + 11 testes |
| Sandbox | RLIMIT_AS + seccomp BPF (37 syscalls) |
| Circuit Breaker | Closed/Open/HalfOpen integrado providers |
| HNSW Index | Go puro, O(log n), 13 testes |
| Crypto | AES-256-GCM, COSCA_REAL_HOSTNAME |
| Build Protection | chmod -x (644) — build protegido |
| Provider Env | loadDotEnv + propagateProviderEnv |
| Embed Sync | internal/embed/cosca/ ↔ internal/embed/cosca/ alinhado |
| Kill Switch | emergencia do Kernel antes do despertar |
| Execution Plan Estimator | cosca plan, transparencia pre-aprovacao |
| CKL | Cosca Knowledge Lifecycle — Evidence Counter + Promotion Engine |
| Release Signing | assinatura de release (Fase 9, Opcao C) |
| License + Security Key | direitos autorais + chave de seguranca via repositorio |

---

## Recent Commits (10)

| Hash | Descricao |
|---|---|
| `17baa53` | feat(bootstrap): config global do OpenCode — Kernel inicia em qualquer projeto |
| `3aa4add` | docs(memory): snapshot de sessao + learnings versionados no git |
| `d8064f4` | docs(roadmap): Fase 9 — Assinatura de Release (Opcao C) |
| `6daa6e7` | feat(license): direitos autorais + chave de seguranca via repositorio |
| `f56d250` | feat(ckl): Hall da Fama + CKL no ORC + PAPER com tese |
| `238ac6d` | feat(ckl): Cosca Knowledge Lifecycle — Evidence Counter + Promotion Engine |
| `78b3fee` | feat(security): Onda 3 — logs, cookies, gRPC, audit, dev-admin, CSP, perms |
| `96398b7` | feat(security): Onda 2 — rate limit, tokens, body, ownership, SSRF, defaults |
| `35e85e5` | feat(security+delegate): Onda 1 criticas + integracao do estimator na delegacao |
| `54049f8` | feat(estimator): Execution Plan Estimator — transparencia pre-aprovacao |

---

## Ondas (concluidas)

| Onda | Agentes | Resultado |
|---|---|---|
| 2 | 10 agentes | 0.48 → 0.53 confianca |
| 3 | 9 especialistas | 33/55 (60%) |
| 5 | 6 business agents | 47/55 (85%) |
| 6 | 7 leadership+orfaos | 54/55 (98%) |

---

## Pending (P1)

| Item | Epic |
|---|---|
| Migrar @cosca/sdk axios→fetch | SDK |
| Soak test job no CI | CI/CD |
| WASM host functions | Platform Evolution |
| Abstração handlers REST/gRPC/MCP | Backend |

---

## Risks

| ID | Risco | Status |
|---|---|---|
| R5 | gRPC auth | ✅ resolvido |
| R22 | Streaming | ✅ resolvido |
| R4 | CI verde | ✅ resolvido |
| R1 | Agent activation | ✅ resolvido |
| R3 | Soak test | 🔴 ativo |
| R9 | Kernel SPOF | 🔴 ativo |
| R11 | Sem disaster recovery | 🔴 ativo |
