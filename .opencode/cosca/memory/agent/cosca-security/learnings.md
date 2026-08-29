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

## Session: 2026-08-28 — Mining codesight-mcp (MIT) + cie (AGPL) — lente SEGURANÇA/LICENÇA

### 2026-08-28 — codesight-mcp security model vs Cosca I1–I8, byte-offset, hooks
| Field | Value |
|-------|-------|
| **Agent** | cosca-security |
| **Task** | Mineração profunda de segurança/licença de `cmillstead/codesight-mcp` (Python, MIT, 34 ops, tree-sitter 66 langs) e `kraklabs/cie` (Go, AGPL-3.0-or-later, CozoDB/CGO) |
| **Technique** | Level 3 — leitura de src/codesight_mcp/{security.py, security_rules.py, core/validation.py, core/boundaries.py, core/limits.py, storage/index_store.py, tools/{get_symbol,search_text,trace_taint}.py, server.py}, hooks/{post-commit,post-push}, tests/benchmark/test_token_efficiency.py; cie: LICENSE, LICENSE.commercial, THIRD_PARTY_LICENSES.md, go.mod, pkg/ingestion/{implements.go,parser_interface.go,parser_go.go}, pkg/tools/{trace.go,code.go}, README. Comparado com Cosca internal/{security/secret.go, contenttrust/guard.go, codegraph/{build,index,extract}.go}, docs/adr/ADR-017, ADR-019 |
| **Level** | 3 |
| **Outcome** | success |
| **Tags** | #mining #codesight #cie #agpl #spotlighting #byte-offset #interface-dispatch #git-hooks #path-rails #i7 #i8 #adr-017 #adr-019 |
| **Related** | codesight security.py:sanitize_signature_for_api/_NO_REDACT, core/validation.py:validate_path (6-step), core/boundaries.py:wrap_untrusted_content, storage/index_store.py:get_symbol_content (byte_offset seek+read), hooks/post-commit (KEY=VALUE env, 0600, no source); cie LICENSE (AGPL+c2), implements.go:BuildImplementsIndex, parser_go.go:FieldEntity, trace.go:getCalleesViaFields/ViaParams; Cosca codegraph é FILE-level (SignalSet/Meta) sem byte ranges; Cosca NÃO tem git hooks (só .sample) |
| **Learned** | **codesight "security-hardened" = data-plane, NÃO execution-plane.** NÃO tem sandbox/jail (Python, POSIX-only, corre com privilégio do usuário, escopo só via CODESIGHT_ALLOWED_ROOTS + validação de path) → NO eixo I7 (isolamento) é MUITO mais fraco que o Cosca (jail+bwrap+auto-jail). O DIAMANTE real é **data-plane para agente hostil + código hostil**: (P1) 6-step path chain NFC→control→`..`/dot→len/depth→resolve→containment→symlink-parent lstat→post-resolve hidden-segment recheck, O_NOFOLLOW em todo open — MAIS forte que o rails.go atual do Cosca. (P2) **Spotlighting/Content-boundary** `<<<UNTRUSTED_CODE_{secrets.token_hex(16)}>>>` + `_meta{contentTrust:untrusted}` — defesa de prompt-injection indireta (estilo Microsoft spotlighting) que o Cosca NÃO tem (o contenttrust marca/quarentena secretos I8/I6/I2 mas NÃO delimita conteúdo como "data nunca instrução" pro agente). (P3) matcher de frases de injeção em 3 tiers (single-token forte, multi-word signature forte, weak corroborada) — muito além dos 5 markers do Cosca. (P4) redaction-aware search_text (redige conteúdo antes de casar, desabilita search se NO_REDACT, rejeita query com sentinel). (P5) env-freeze no import; (P6) `_check_posix_acls` (ACL POSIX além dos 3 base no Linux — sutil, Cosca não tem); (P7) `_safe_rmtree` sem seguir symlink. **Byte-offset O(1)**: `byte_offset=node.start_byte`/`byte_length=node.end_byte-start_byte` no parse + `get_symbol_content` = seek+read (O(1), sem re-parse) + `content_hash` do slice p/ `verify`/diff. ~99% tokens é MARKETING → real = símbolo único é <20% (large <10%) do arquivo (teste assert); no caso de uso "pegar só o que precisa" a economia é 80-99%. **Cosca codegraph é FILE-level** (signals/meta/graf de arquivos; símbolos só nomes em Metadata) → byte-offset é NOVO e adaptável. **Git hooks**: codesight carrega env via `grep KEY=VALUE` (nunca `source`), checa 0600, roda background c/ `--no-ai`; Cosca NÃO tem hooks (só .sample) e o `secret.go` é o detector sem wiring → ADAPTAR (pre-commit) usando o padrão env-load seguro. **cie AGPL**: dual license (AGPL-3.0-or-later + comercial), usa **CGO** (go-tree-sitter + cozodb/libcozo_c.a) → NÃO é puro Go; in-process import → derivativo → Cosca AGPL (catastrófico) → PROIBIDO. Sidecar separado (MCP stdio) NÃO modificado = agregação, §2 permite rodar o Program, §13 só pega versão MODIFICADA oferecida a REMOTE users por rede → NÃO encobre código proprietário do Cosca (postura legal defensável, MAS risco não-zero + repo 19⭐ pré-1.0). Comercial disponível se precisar embutir. **Interface-dispatch (cie)**: `BuildImplementsIndex` (method-set matching + embedded/stdlib) + `parser_go.go` FieldEntity + `trace.go` (3-fase: direct calls → field dispatch via cie_field+cie_implements → param dispatch via ParseGoParams; fan-out cortado por `calledMethods` `.Method(` regex; `ViaIface`; `detectInterfaceBoundary`). É a ponte concreta p/ `internal/codegraph` ir de heurístico a type-aware (ADR-019 §2.1). **NOTA antivacuo:** deixar conteúdo externo pro agente sem boundary-marker = vetor de manipulação semântica (achado 2026-08-22); spotlighting é a mitigação direta. |
| **Next** | Level 4: (priorizado) ADAPTAR spotlighting/content-boundary no contenttrust (I8 explícito p/ agente) + 6-step path chain no rails + tiered injection matcher; ADAPTAR byte-offset symbol retrieval no internal/codegraph (compor em BuildIndex, GetSymbolSource O(1)); ADAPTAR interface-dispatch (só a IDEIA, não código AGPL) p/ codegraph Go type-aware; ADAPTAR git hook pre-commit c/ secret.go + padrão env-load 0600; VEREDITO licença cie = REJEITAR como dependência/backbone, só IDEIA, sidecar AGPL só como prova-fronteira sandboxed (I7) e não modificado. |

## Session: 2026-08-29 — Brainweb "cerebro neural 3D" public endpoint audit

### 2026-08-29 — /brain public endpoints: data exposure + traversal + CSP threat model
| Field | Value |
|-------|-------|
| **Agent** | cosca-security |
| **Task** | Auditar o observatorio /brain read-only publico (only-inspect, report only) |
| **Technique** | Level 3 — manual code review de internal/brainweb/{handler,graph,observatory,embed}.go + web/{index.html,app.js} + api/rest/server.go (publicPaths, registerRoutes, buildHandler) + api/middleware/{security,csrf,cors,ratelimit}.go + api/auth/oidc.go Middleware + internal/cli/root.go (recordCommandActivity) + internal/trace/causal.go + internal/knowledge/evidence.go |
| **Level** | 3 |
| **Outcome** | success |
| **Tags** | #public-endpoint #brainweb #data-exposure #path-traversal #csp #activity-jsonl #prompt-injection-recon #observatory #threat-model |
| **Related** | server.go L273-294 (publicPaths), L617-624 (rotas), L865-899 (buildHandler); handler.go mount L94-105; graph.go Node L46-58/SkillVz L68-75; observatory.go KItem L44-54/RNode L64-69; trace/causal.go CausalNode L27-33; security.go L31-41 (CSP); root.go recordCommandActivity L253-303; DefaultConfig host 127.0.0.1 L161-166 |
| **Learned** | **FALHA-ABERTA NAO presente** — todos os /brain/* passam pela cadeia (Recovery->CORS->SecurityHeaders->Auth->CSRF->RateLimit->mux): auth usa `public[r.URL.Path]` exact-match (fail-closed; secret vazio->503), CORS default vazio (sem ACAO), CSRF skip so p/ GET, rate-limit 100rpm/IP aplicado. **Achados:** (A1 ALTO) /brain/observatory TRACE_REPLAY expoe a ultima execucao real (action+actor+result) sem token — resultado operacional vaza; o campo Details e descartado mas Action/Result podem ser sensiveis. (A2 ALTO) topologia completa publica (agents name/role/description, skills name/description/standard, hierarchy reports_to, departments, tier) — reconhecimento p/ prompt-injection direcionado; por design "projecao minima" mas nomes+roles ja dao mapa do OS; se host=0.0.0.0 fica internet-visible (default 127.0.0.1 mitiga). (A3 MEDIO) observatory expoe knowledge titles+confidence+epistemic+counts+last_verified (crencas/grau de certeza do sistema vazam). (A4 MEDIO) CSP `script-src 'self' 'unsafe-inline' 'unsafe-eval'` (security.go L33) == XSS mal contido; 'unsafe-inline' p/ importmap inline (index.html L54-61), 'unsafe-eval' alegando Next.js (NAO usado no /brain — three.js nao precisa). (A5 MEDIO) activity.jsonl AT-REST guarda prompt = comando completo + args em claro (root.go L290, 0600) — args persistem no disco; o READ so superf. "COMMAND_EXECUTED" (Prompt=ra.Action L727) entao args nao vazam via /brain/activity MAS o arquivo retem os args. (A6 BAIXO) traversal bem defendido: embed.FS confina em web/ + ReplaceAll("..") L101 + path.Clean L103 + ServeMux clean + teste (brainweb_test L381-391). (A7 BAIXO) append concorrente sem lock (root.go L297 O_APPEND) -> linha torn/interleaved -> json.Unmarshal falha -> registro descartado (integridade, nao seg.); json.Marshal escapa \n entao sem JSON-injection. (A8 BAIXO) allowlist estatico exact-match: /brain/<asset nao enumerado> -> 401 (fail-closed OK; novo asset quebra silenciosamente). **CSP 'unsafe-eval'/'unsafe-inline' e o ponto mais fragil**: se qualquer render futuro usar innerHTML c/ dados de flow -> XSS pleno. |
| **Next** | NAO implementar (so report). Recomendar: (H1) gating /brain por auth quando host!=loopback OU redigir trace_replay (dropar Result/Action/Details) e torna-lo opt-in; (H2) rate-limit/limite dedicado p/ /brain/* + cache do graph/observatory; (H3) CSP: nonce/hash no importmap em vez de 'unsafe-inline' + remover 'unsafe-eval' + `script-src-attr 'none'`; (H4) sanitizar/encode free-text (description/role/title/action) antes de servir + esc no render (app.js so esc() em showFatal L615); (H5) redigir args no activity.jsonl (guardar so nome do comando) + re-assert 0600; (H6) headers COOP/COEP no /brain. |

## Session: 2026-08-29 — Implementacao P1 /brain (Graph/Activity/CLI) — Aprovada

### 2026-08-29 — Correcoes P1 aplicadas (fluxo vivo, multi-agente)
| Field | Value |
|-------|-------|
| **Agent** | cosca-security |
| **Task** | Implementar as correcoes P1 do /brain (read-only) — remover vazamento de prompt sensivel |
| **Technique** | Level 3 — edits em internal/brainweb/graph.go, api/rest/server.go, internal/cli/root.go + go build/go test |
| **Level** | 3 |
| **Outcome** | success (fix aplicado; build de api/rest bloqueado por refactor concorrente NAO-meu) |
| **Tags** | #brainweb #p1 #prompt-leak #activity-jsonl #redaction #multi-agent-race |
| **Related** | graph.go Activity struct L82-92; server.go rawActivity L744-749 + Action:ra.Action L800; root.go recordCommandActivity L281-290; handler.go app.js (so usa a.agent) |
| **Learned** | **(1) Fix P1 aplicado**: `Activity.Prompt` → `Action` (rotulo seguro "COMMAND_EXECUTED", nunca args) em graph.go; rawActivity leu `prompt` mas nunca expoe; readActivityLog mapeia `Action: ra.Action` (L800); CLI recordCommandActivity (root.go L281-290) grava `"prompt": name` (SO o nome do comando, sem strings.Join(args) — args sensiveis nunca persistem nem vazam). (2) **app.js so consome `a.agent`** na atividade → remover Prompt nao quebra o dashboard. (3) **RNode/observatory ja era projecao minima** (ID/Action/Actor/Result, SEM Details) → nenhuma mudanca necessaria em observatory.go; Node/SkillVz tambem so campos minimos. (4) **FLUXO VIVO / RACE**: este workspace tem multiplos agentes editando os mesmos arquivos (server.go mudou entre reads: readActivityLog lancou `Prompt: ra.Action`; um agente concorrente refatorou p/ tail-read com `io.NewSectionReader` + `start int64` → introduziu compile errors (start redefinido int64/int L785-790) que NAO sao meus). **LECAO**: em multi-agente, nunca assumir estabilidade de arquivo; confirmar estado antes de build e nao "corrigir" bugs de refactor alheio (instrucao: so informar resultado dos testes). `io` import foi inicialmente orfao (removido), depois re-necessario (readicionado pelo agente concorrente) — revalidar sempre. (5) So `go test ./internal/brainweb/...` verde; api/rest e cli bloqueados pelo compile error concorrente. |
| **Next** | Revalidar build/test de api/rest+cli apos o agente concorrente finalizar o tail-read de readActivityLog (L785-790); se persistir, reportar cleanly. Considerar H1 (redigir trace_replay) e H5 (re-assert 0600 no activity.jsonl) como P2. |
