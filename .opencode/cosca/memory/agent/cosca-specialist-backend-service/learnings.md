# cosca-specialist-backend-service — Semantic Learnings

> Auto-evolution memory. Search before acting. Record after learning.

## Seed Knowledge

### 2026-07-27 — Baseline
| Field | Value |
|-------|-------|
| **Agent** | cosca-specialist-backend-service |
| **Task** | Initial capability establishment |
| **Technique** | Standard backend-service patterns — project conventions |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #backend-service #baseline #initialization |
| **Related** | .opencode/cosca/memory/codebase/overview.md |
| **Learned** | Project established. Core backend-service patterns documented. Ready for Level 2 techniques. |
| **Next** | Level 2: Identify first advanced technique to master |

## 2026-08-22 — Family chain gate: machine-bound DPAPI + consent-to-content nonce

| Field | Value |
|-------|-------|
| **Agent** | cosca-specialist-backend-service |
| **Task** | Migrate `internal/cli/memory_identity.go` + `cmd/cosca-check/main.go` to the DPAPI machine-bound chain signature (no passphrase); add the Don gate (machine + consent-to-content nonce); mark `internal/kernel/donauth.go` as legacy |
| **Technique** | (1) Fator 1 = `integrity.VerifyKernelIdentity(root)` (DPAPI, same machine+user, no secret to remember). (2) Fator 2 = consent-to-content: scan `internal/embed/cosca`, derive a deterministic BLAKE3 digest (`path:hash:size` sorted) + short approval token, show file-count/resumo, Don types token back. (3) M3 fail-closed: `!term.IsTerminal(int(os.Stdin.Fd()))` → deny; always read from real os.Stdin, never a piped/readable fallback. (4) M5: `crypto/subtle.ConstantTimeCompare` for the token (never `typed != nonce`). (5) `--sign`/`--push` call the gate first; `--sign` = strict Ed25519 (no silent git-anchor fallback), `--push` = gate + session credential (COSCA_GIT_TOKEN via ephemeral GIT_ASKPASS helper, or GIT_ASKPASS) and fails closed without it. (6) `--sign-auto` remains the explicit GIT-ANCHORED (immutability witness) path — M7 output distinguishes Ed25519 (Don authority) from GIT-ANCHORED. |
| **Level** | 3 |
| **Outcome** | success |
| **Tags** | #backend-service #integrity #security #don-gate #dpapi #nonce #m7 #m3 #m5 |
| **Related** | internal/cli/memory_identity.go, cmd/cosca-check/main.go, internal/kernel/donauth.go, internal/integrity/* |
| **Learned** | (1) `verifyDonIdentity` é unexported (package cli); para os callers de fora (cmd) é preciso um wrapper exportado `VerifyDonIdentity` — nunca renomear a unexported usada por `memory_register.go`. (2) O nonce de consentimento deve ser derivado do CONTÉUDO (determinístico) para que uma mudança de conteúdo force re-consentimento — não é um segredo, a prova real é a presença humana no TTY + vínculo de máquina. (3) `go build ./...` não compila `*_test.go`; `go vet ./...` sim — por isso o vet é o guarda real contra assinaturas não-migradas. (4) Um wrapper exportado mínimo no mesmo arquivo edições-preservado é a saída quando o caller externo está fora do escopo de edição. |
| **Next** | (1) Decidir se `--init`/`--rekey` (identidade) devem também exigir o portão do Don — hoje fecham só máquina (não-gate, para não quebrar bootstrap/recovery). (2) Squad de testes migrar/validar `internal/integrity/*_test.go` (já migrado por outra squad em paralelo). (3) Atualizar docs `internal/embed/cosca/*.md` e `.cosca`/`DON_PROTOCOL.md`/`CLI_PROTOCOL.md` que ainda citam `--passphrase-stdin`/`COSCA_KERNEL_PASSPHRASE` (fora do meu escopo P8). |

## 2026-08-22 — Revisão de segurança: gate de presença em --init/--rekey + anti-TOCTOU + token 16 hex

| Field | Value |
|-------|-------|
| **Agent** | cosca-specialist-backend-service |
| **Task** | Aplicar as correções da revisão de segurança no gate machine-bound (DPAPI) do `internal/cli/memory_identity.go` + `cmd/cosca-check/main.go`: gate HUMANO em `--init`/`--rekey` sem exigir a chave antiga; entropia do token 8→16 hex; TOCTOU consentimento→assinatura; limpar comentário stale em `memory_register.go`. |
| **Technique** | (1) SEPARAR o fator MÁQUINA do fator PRESENÇA: `verifyDonIdentityDigest` (M3+máquina+M4+M5) e `presenceFactor` (M3+M4+M5, SEM `integrity.VerifyKernelIdentity`). O rekey é o caminho de RECUPERAÇÃO — exigir o unprotect da chave antiga aqui bloquearia a própria recuperação. `VerifyDonPresence` exportado, sem tocar na chave. (2) TOCTOU: `VerifyDonIdentity` passa a retornar o `digest` consentido; após `integrity.Sign`, `DonConsentDigest` recomputa SEM prompt e compara em `subtle.ConstantTimeCompare`. Se divergir → aborta "conteúdo mudou entre consentimento e assinatura". (3) `token[:8]`→`token[:16]` (64 bits). (4) `verifyDonIdentity` unexported mantém `error` (memory_register.go usa) — não renomear; o wrapper exportado `VerifyDonIdentity` mudou para `(string, error)` só para o cosca-check. |
| **Level** | 3 |
| **Outcome** | success |
| **Tags** | #backend-service #integrity #security #don-gate #toctou #consent #presence #m3 #m4 #m5 #rekey #init |
| **Related** | internal/cli/memory_identity.go, cmd/cosca-check/main.go, internal/cli/memory_register.go, internal/integrity/sign.go |
| **Learned** | (1) `Sign` NÃO altera o embed (só apenda bloco em `.cosca/family_chain.dat`), logo recomputar o digest pós-sign só diverge se algo externo mudou o embed — é a base correta do anti-TOCTOU. (2) `go build ./...` não compila `*_test.go`; `go vet ./...` sim — rodar ambos; o vet pega assinaturas não-migradas. (3) Mudar a assinatura de um wrapper exportado (`VerifyDonIdentity` → `(string, error)`) é seguro se eu controlo TODOS os callers (só `cmd/cosca-check/main.go`); o unexported usado por `memory_register.go` NÃO pode mudar. (4) Para preservar a ordem M3→máquina→consentimento e reutilizar o código, quebrei em `m3RequireTTY` + `consentFactor` (compartilhados) + `presenceFactor`/`verifyDonIdentityDigest` (composições). (5) Compatível win32: tudo usa `os.Stdin`/TTY real; nenhum deltas específicos de OS. |
| **Next** | (1) Validar se `--init`/`--rekey` também exigiriam o anti-TOCTOU (hoje só `--sign` — o task pediu só sign). (2) Se o Don quiser, mover a comparação anti-TOCTOU para dentro do pacote (helper) para os testes unitários cobrirem-na. (3) Docs `internal/embed/cosca/*.md` + `CLI_PROTOCOL.md` ainda citam "passphrase (2FA) + war phrase + presença" e `--passphrase-stdin` — P8, fora do escopo. |
