# cosca-security - learnings.md PRE-FIX (conteudo nao-registrado na chain)

> Arquivo gerado em 20260908 antes da reconstrucao do indice de gatilhos.
> Conteudo preservado - leia por grep, nunca inteiro.

# cosca-security — Semantic Learnings

> Auto-evolution memory. Search before acting. Record after learning.

## Seed Knowledge

### 2026-07-27 — OWASP Top 10 Baseline
| Field | Value |
|-------|-------|
| **Agent** | cosca-security |
| **Task** | Security audit baseline |
| **Technique** | OWASP Top 10 checklist — manual code review |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #security #owasp #baseline #code-review |
| **Related** | Broken Access Control, Cryptographic Failures, Injection, Insecure Design |
| **Learned** | All 10 categories mapped to Cosca codebase patterns. Go-specific: SQL injection impossible with parameterized queries. JWT HS256 baseline. |
| **Next** | Level 2: Integrate govulncheck automated scanning |

### 2026-07-27 — JWT Security Baseline
| Field | Value |
|-------|-------|
| **Agent** | cosca-security |
| **Task** | Auth implementation review |
| **Technique** | JWT best practices — algorithm check, expiry validation, refresh rotation |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #jwt #auth #hs256 #token-rotation |
| **Related** | OWASP #2 Cryptographic Failures, OWASP #7 Auth Failures |
| **Learned** | Project uses HS256 with refresh tokens. Tokens stored in httpOnly cookies. RBAC enforced at middleware. |
| **Next** | Level 2: Add token revocation list, implement rate limiting on auth endpoints |
> [!NOTE - Decisao do Don 2026-09-08]
> Arquivo reduzido para conter custo de tokens. Historico completo preservado em: archive\learnings-20260908.archive.md - leia por busca/grep, NUNCA integralmente.

## Session: 2026-07-28 — Documentation Audit (Fase 1+2)

### 2026-07-28 — Full Auth Architecture Audit
| Field | Value |
|-------|-------|
| **Agent** | cosca-security |
| **Task** | Comprehensive auth + middleware security documentation |
| **Technique** | Level 2 — Multi-layered security audit: JWT implementation (custom HMAC-SHA256, no lib), bcrypt cost verification (12), brute-force analysis (5 attempts → 15min lockout), CSRF double-submit cookie audit (constant-time compare), rate limiting audit (token bucket: 5 RPM login / 100 RPM general), security headers audit (CSP, HSTS, X-Frame-Options), RBAC tier analysis (admin bypass pattern) |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #auth #jwt #middleware #csrf #rate-limiting #rbac #bcrypt #security-headers #documentation |
| **Related** | OWASP #2, #4, #7; docs/api-reference/auth.md, docs/api-reference/middleware.md |
| **Learned** | Full 3-tier auth priority chain documented: API Key (SHA-256 hash lookup) → Cookie (HttpOnly JWT) → Bearer token. Password hashing bcrypt cost 12 confirmed secure. CSRF uses crypto/subtle.ConstantTimeCompare — timing-attack resistant. Rate limiting uses per-IP token bucket with fractional refill. Security headers: CSP restricts connect-src, HSTS only on TLS. No OIDC/OAuth2 integration exists — documented as aspirational. API keys: 32-byte random → SHA-256 storage (plaintext shown once). Account lockout: check before bcrypt (CPU-saving). Known gaps: no token revocation list, no MFA, no audit log tamper detection. |
| **Next** | Level 3: STRIDE threat model per subsystem, implement govulncheck in CI, add token revocation list |

### 2026-07-28 — Memory Compliance Fix
| Field | Value |
|-------|-------|
| **Agent** | cosca-security |
| **Task** | Correct false compliance claims in memory |
| **Technique** | Level 2 — Aspirational vs actual audit: identified fabricated GDPR/SOC2/ISO 27001 compliance dates in memory, rewrote as aspirational with clear next steps |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #compliance #gdpr #soc2 #memory-audit #documentation |
| **Related** | memory/long/compliance-framework.md |
| **Learned** | Memory can drift into aspirational/fictitious claims. Pattern: always verify memory claims against codebase reality. Compliance framework marked as aspirational with 5 concrete implementation steps (at-rest encryption, audit logging, data mapping, retention automation, right-to-erasure). |
| **Next** | Level 3: Implement at-rest encryption for secrets, add audit log digital signatures |

## Session: 2026-08-22 — Semantic Manipulation Security Audit (Level 3)

### 2026-08-22 — CLI Semantic Manipulation Attack Surface Analysis
| Field | Value |
|-------|-------|
| **Agent** | cosca-security |
| **Task** | Investigate CLI security vulnerabilities related to semantic manipulation by external AI |
| **Technique** | Level 3 — Code audit of 15+ files across engine/, chat/tool/, sandbox/, contenttrust/, execpolicy/, cli/ — focused on prompt injection, trust boundaries, and semantic manipulation vectors |
| **Level** | 3 |
| **Outcome** | success |
| **Tags** | #prompt-injection #semantic-manipulation #trust-boundary #content-trust #sandbox #cli-security #attack-surface |
| **Related** | V1-V13 findings: AGENTS.md injection, agent override, .env injection, shell bypass, MCP trust bypass, content trust gaps |
| **Learned** | **Critical finding:** Content trust envelopes protect memories and knowledge but NOT the system prompt itself (agent definitions + AGENTS.md chain). This is the highest-authority position in the LLM context. The AGENTS.md chain is loaded from filesystem without integrity checks and injected raw into system prompt. Agent definitions can be overridden by placing .md files in .cosca/agents/ (last-wins). The .env loader has no key allowlist. The SandboxTool falls back to unsandboxed local execution. The execpolicy tokenizer doesn't interpret shell operators (|, &&, ;). On non-Linux, all sandbox is advisory. Suspicious content detector has only 5 markers. |
| **Next** | Level 4: Implement cryptographic agent signatures, shell AST parser for execpolicy, runtime integrity monitor, per-agent trust tiers |

## Session: 2026-08-24 — Windows Jail Fail-Closed Analysis (Level 3)

### 2026-08-24 — Windows auto-jail: fail-closed vs fail-open vs opt-in
| Field | Value |
|-------|-------|
| **Agent** | cosca-security |
| **Task** | Analyze what happens when `cosca despertar` runs on Windows (no bwrap) |
| **Technique** | Level 3 — Code audit of jail.go/jail_windows.go/jail_linux.go, main.go (isAdminCommand), jail_test.go (FailClosed), per-command Gate (chat/sandbox gate.go + gate_linux.go + gate_other.go), execpolicy, hardening |
| **Level** | 3 |
| **Outcome** | success |
| **Tags** | #windows #jail #fail-closed #bwrap #appcontainer #jobobject #guardpact #threat-model |
| **Related** | jail_windows.go, jail.go:jailFallback, main.go:isAdminCommand, chat/sandbox/gate.go, chat/sandbox/gate_other.go |
| **Learned** | **Windows has NO sandbox at all.** (1) Auto-jail: jailAvailable() always false → ReexecInJail() hits jailFallback → SECURITY WARNING + exit 1 unless COSCA_ALLOW_NO_ROOT=1 (explicit opt-in). BUT isAdminCommand() bypasses ReexecInJail entirely for ~40 commands → they run unsandboxed unconditionally + silently. (2) Per-command Gate (chat/sandbox): findBwrap() returns "" on Windows → Execute(SandboxWorkspace) uses execWithoutSandbox → requires COSCA_ALLOW_NO_ROOT=1, else error. With opt-in = execDirect = `cmd /c <command>` with NO workspace confinement, NO network block, NO resource limits. The "network off/workspace confined" promise is broken on Windows. (3) SandboxTool (chat/tool/sandbox.go) runs python/go/sh via exec.CommandContext directly, bypassing the gate AND the opt-in, BUT it is NOT registered in engine_builder.go toolRegistry — latent risk, not active. (4) execpolicy tokenizer doesn't interpret shell operators (|&&;`$()`) so prefix rules are bypassable via `cmd /c`/`sh -c`; on Linux bwrap contains the blast radius, on Windows it's the last line and it leaks. (5) hardening.applyPlatformHardening is a no-op on Windows (no RLIMIT_CORE/dumpable/no_new_privs). **Verdict:** COSCA_ALLOW_NO_ROOT=1 is a documented explicit fail-open *escape hatch* (acceptable for trusted-dev), NOT a mitigation — it disables isolation. To honor the Guard Pact on Windows requires a real native isolator: AppContainer (capability-based FS/network scope, Win8+, moderate cost) + Job Object kill-on-close (≈--die-with-parent, low cost) + Low Integrity/Restricted Token (privilege cap, moderate). Windows Sandbox (kernel VM) is strongest isolation but too heavy/slow for `serve`. WSL2+bwrap reuses Linux jail logic (highest reuse, high ops cost). |
| **Next** | Level 4: If Don approves a Windows tier, prototype an AppContainer + Job Object + Low-IL confinement for the per-command gate; fix execpolicy to parse shell operators; document Windows as trusted-dev-only tier in the Guard Pact. |

### 2026-08-22 — Key Security Architecture Patterns Discovered
| Field | Value |
|-------|-------|
| **Agent** | cosca-security |
| **Task** | Document positive security patterns for future reference |
| **Technique** | Pattern extraction from defense-in-depth analysis |
| **Level** | 3 |
| **Outcome** | success |
| **Tags** | #security-patterns #defense-in-depth #sandbox #rails #content-trust |
| **Related** | sandbox/rails.go, contenttrust/contenttrust.go, execpolicy/, hardening/ |
| **Learned** | **Strong patterns:** (1) Path rails with symlink resolution + blocked dirs — comprehensive workspace escape prevention. (2) Content trust envelopes with JSON-escaped length-delimited content — prevents delimiter spoofing. (3) Bubblewrap with --unshare-all, --clearenv, --die-with-parent + resource limits (RLIMIT_AS, FSIZE, NOFILE). (4) Git command whitelist with stash subcommand restriction. (5) Env var allowlist at sandbox boundary. (6) Embed read-only enforcement via --ro-bind in bwrap. (7) Memory integrity gate at startup with first-boot baseline. (8) Null byte rejection in path validation. |
| **Next** | Apply these patterns to close the gaps identified in V1-V13 |

### 2026-08-24 — Threat model + blindagem do "Cofre" Windows (Don aprovou)
| Field | Value |
|-------|-------|
| **Agent** | cosca-security |
| **Task** | Threat model + plano de blindagem do Cofre (air-gap, IA local + Oráculo) no Windows/WSL2 contra entrada não validada |
| **Technique** | Level 3 — Leitura de deploy/README-WINDOWS.md, jail.go/jail_windows.go/jail_linux.go, vault.go, oracle.go/search.go, install-service.ps1, cognitive-state + wsl --status (v2.7.12/kernel 6.1, SEM distro instalada) |
| **Level** | 3 |
| **Outcome** | success |
| **Tags** | #threat-model #cofre #windows #wsl2 #fail-closed #air-gap #appcontainer #jobobject #low-il #oracle #p0-p1 |
| **Related** | deploy/README-WINDOWS.md §4/§11, jail.go:jailFallback, jail_windows.go, install-service.ps1 L136, cosca-serve.bat L2, cosca-service.ps1, internal/oracle, docs/security/ |
| **Learned** | (1) **Brecha central confirmada**: os launchers instalados (deploy/install-service.ps1 L136, cosca-serve.bat L2, cosca-service.ps1 L24/93) injetam `COSCA_ALLOW_NO_ROOT=1` por padrão → opt-in virou default → o cosca roda SEM sandbox no Windows, só emitindo SECURITY WARNING; a alegação de "fail-closed preservado" no README-WINDOWS §4/§11 é FALSA. (2) **WSL2 ativo (v2.7.12/kernel 6.1) MAS sem distro instalada** (`wsl -l -v` = nenhuma) → o plano (a) exige instalar distro não-root primeiro; bwrap no WSL2 precisa validar `apparmor_restrict_unprivileged_userns` (kernel 6.1). (3) **Air-gap não é automático**: WSL2 NAT fala com a rede do Windows; "zona Cofre isolada" precisa de egress realmente bloqueado (default route/firewall na distro), senão é air-gapped "no papel". (4) **Camadas priorizadas P0**: (b) fechar o opt-in default + gate isAdminCommand/por-comando falha-fechado; (a) jail WSL2+bwrap reusando buildJailArgs (workspace=/ + --unshare-all --unshare-user --clearenv --die-with-parent); (d) Oracle ingress gate-first (já existe, não regredir). **P1**: (c) Job Object + Low-IL + SetProcessMitigationPolicy + egress firewall (defense-in-depth, NÃO bloqueia exfil), execpolicy parser, SandboxTool. (5) **Trade-offs**: air-gap no papel / fail-closed quebra dev loop / (c) dá falsa sensação de segurança (não bloqueia leitura/exfil de serve.env+vault). (6) vault.go deriva a chave AES de COSCA_JWT_SECRET (ponto único—se vazar, tudo decifrável). |
| **Next** | If Don approves a Windows tier: (P0) remove COSCA_ALLOW_NO_ROOT default + isAdminCommand gate; (P0) WSL2 distro + bwrap jail do cofre (2 zonas); (P1) Job Object+Low-IL+egress firewall spawner; fix execpolicy shell operators. Authorized doc-only so far.

## Session: 2026-08-25 — COSCA Desktop Review (app.go) — 7 findings

### 2026-08-25 — Desktop app.go security review
| Field | Value |
|-------|-------|
| **Agent** | cosca-security |
| **Task** | Security review of `projects\cosca-desktop\app.go` (Wails backend) |
| **Technique** | Level 3 — manual code review of app.go/main.go/app_test.go + ADR-0001..0006 + skills security(SECURITY_AUDIT/SECRETS_AUDIT/GO_SECURITY) |
| **Level** | 3 |
| **Outcome** | success |
| **Tags** | #desktop #wails #path-traversal #symlink #cmd-injection #secrets #project-isolation #approval-gate #fail-open |
| **Related** | app.go: safeJoin(325-336), CreateProject(114-134), resolveRuntime(51-68), runRuntime(377-392), AgentPlan(401-416), AgentRun(419-434); ADR-0002/0003/0004/0006 |
| **Learned** | **Skill path note**: `skills/security/SKILL.md` NÃO existe; a skill canônica vive como `*.md` em `internal/embed/cosca/skills/security/` (SECURITY_AUDIT.md etc.) e há `departments/security/SKILL.md`. **7 findings:** (F1 CRÍTICO) `COSCA_ALLOW_NO_ROOT=1` forçado (L144/386/410/428) + `cosca exec/plan` SEM gate approval/execpolicy → desktop contorna fail-closed e anula a defesa que ADR-0003 chama de REAL no Windows (approval+execpolicy). (F2 ALTO) `safeJoin` (L325-336) NÃO usa `filepath.EvalSymlinks` → escape via symlink/junction (passa prefixo léxico, mas Read/WriteFile segue o link p/ fora da raiz); contradiz alegação "Filesystem ENFORCED (Rails.Validate)". (F3 ALTO) `CreateProject` só faz TrimSpace no name (L115), não rejeita `..`/`/`/`\`, `root=Join(loc,name)` (L123) + MkdirAll (L127) + `cmd.Dir=root` (L143) → cria dir e executa cosca init FORA de location. (F4 MÉDIO) ReadFile/WriteFile não blocklist: UI lê `.cosca/serve.env`, `.env`, `.git/config`; tree() mostra `.cosca` (L270) → secrets do projeto viajam à WebView (viola ADR-0002 "nunca na UI"). (F5 MÉDIO) `resolveRuntime` (L51-68)/exec.Command re-resolve "cosca" via PATH → hijack. (F6 BAIXO) sem maxLen em request/model/message + CombinedOutput bufferiza saída do agente + chatBuf cresce sem limite + perms 0o755/0o644. (F7 INFO) case-sensitivity do HasPrefix (L332) em NTFS case-insensitive → falsa rejeição (denial, não bypass). **CMD injection app.go**: NENHUMA — exec.CommandContext com args slice, sem shell (`;`/`&` são argv literal). Risco residual é second-order (parser do cosca.exe) + entrada sem limite. **Nenhum secret hardcoded** nos .go. |
| **Next** | Alinhar desktop ao oficial (Rails.Validate + execpolicy + approval) em vez de reimplementar; fixar F1 (remover COSCA_ALLOW_NO_ROOT forçado + gate approval), F2 (EvalSymlinks/rejeitar symlink), F3 (validar name + confinar root em location), F4 (blocklist de rotas sensíveis). Autorizado doc/report apenas até Don aprovar correção. |

## Session: 2026-08-27 — Context7/Upstash Security Mining (repo externo)

### 2026-08-27 — Deep security analysis of Context7 MCP/SDK/CLI monorepo
| Field | Value |
|-------|-------|
| **Agent** | cosca-security |
| **Task** | Mineração profunda de segurança do repositório Context7 (Upstash) em Temp/opencode/context7 |
| **Technique** | Level 3 — review manual de jwt.ts, encryption.ts, auth/ (auth-prompt.ts), constants.ts, api.ts, client-ip.ts, index.ts (MCP), sdk client.ts/http/index.ts/error, cli auth.ts/utils/auth.ts/storage-paths.ts/constants.ts/setup.ts/api.ts/github.ts, pi/. Referências OWASP+CWE |
| **Level** | 3 |
| **Outcome** | success |
| **Tags** | #jwt #jwks #multi-issuer #entra #clerk #ema #cors #xff-spoofing #fail-open #aes-cbc #hardcoded-key #secret-management #dpapi #device-flow #rfc8628 #out-of-band-nudge #ssrf #rate-limit #content-trust |
| **Related** | encrypt legacy default AES-CBC zero-key; CLIENT_IP_ENCRYPTION_KEY; validateJWT (Entra/Clerk/EMA); X-Context7-Auth-Prompt; X-Forwarded-For; Access-Control-Allow-Origin `*`; server.json remotes |
| **Learned** | **TOP 6 padrões p/ Cosca:** (P1) validação JWT multi-issuer com remote JWKS cacheado + issuer/audience/scope + erros distintos + cache de miss 404 + n-cache de 5xx (entra config TTL 5min) → Cosca: google-jwt `WithValidMethods(["EdDSA"])` + `WithIssuer`+`WithAudience`+`WithExpirationRequired`. (P2) credenciais fail-closed: dir 0700 + arquivo 0600 + `chmod` re-assert ap\u00f3s write (mode é ignorado em arquivo existente) + migração q aperta perm → Cosca secrets (com DPAPI por cima). (P3) **ANTI-padrão a reverter**: Context7 cifra IP com AES-256-CBC, DEFAULT_ENCRYPTION_KEY = "00 01 02 ... 1f" (hardcoded/zeros), sem MAC, e **fail-open** (retorna IP em claro se chave inválida/erro) → Cosca: AES-256-**GCM** + chave DPAPI + **fail-closed**. (P4) nudge de auth/rate-limit **out-of-band** via `elicitInput` (NÃO concatena no content que o LLM lê — comenta "so it does not trip prompt-injection guards") → confirma nosso achado de Manipulação Semântica 2026-08-22: instruções de sessão/segurança devem ir por canal de UI, nunca no contexto do agente. (P5) Device Flow RFC8628: user_code+verification_uri(+complete), hostname anti-phishing (§5.4), polling respeita slow_down/transient(+5s), expires_in deadline, **preserva refresh_token** no refresh (§6). (P6) rede safe: undici ProxyAgent (HTTPS_PROXY) + NODE_EXTRA_CA_CERTS append + `AbortSignal.timeout(60s)` + handler HTTP **stateless + keepAliveMs:0** (cita outage 2026-08-11; hung stream reapido por streamIdleTimeout) → Cosca: http.Server com Read/Write/IdleTimeout + handler idempotente + ProxyFromEnvironment + SystemCertPool. **Riscos:** R1 CRÍTICO default key zeros+HBC sem MAC; R2 ALTO fail-open plaintext (CWE-703/322); R3 MÉDIO/Alto getClientIp confia XFF sem `trust proxy` (CWE-348/290, spoof/bypass rate-limit, Docker expõe 8080); R4 MÉDIO CORS `*`+Authorization permitido (CWE-942, DNS-rebinding/CSRF-to-localhost); R5 MÉDIO sem rate limit no servidor MCP (enforcement upstream-only); R6 BAIXO path Clerk sem audience; R7 BAIXO SSRF via env OAUTH_*/EMA_JWKS_URL/CONTEXT7_API_URL; R8 BAIXO credentials.json plaintext 0600; INFO openai-apps-challenge token estático, skills externas sem sanitização de conteúdo (prompt-injection, mas o Cosca já tem contenttrust), isJWT via split(".").length===3. **Acertos:** `/mcp`anônimo vs `/mcp/oauth`; --api-key proibido em http; SDK base URL hardcoded (limita SSRF); SDK retry só em erro de rede; execFileSync("gh") sem shell; token só p/ GitHub hosts fixos; erros genéricos sem stack. |
| **Next** | Level 4: aplicar P1 (JWT EdDSA estrito + allowlist alg) e P3 (DPAPI+GCM fail-closed) no cosca; auditar `trust proxy`/CORS explícito e rate limit no `serve`; integrar recomendação de nudge out-of-band ao contenttrust. |


