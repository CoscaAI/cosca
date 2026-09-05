# CLI PROTOCOL — Como operar o Cosca pela linha de comando

> **Versão**: 1.0.0 | **Status**: canônico | **Dono**: cosca-kernel
> **Criado**: 2026-08-16, por ordem do Don — consolidar o uso do CLI de governança/segurança
> **Propósito**: referência operacional ÚNICA para operar o CLI sem precisar adivinhar
> flags, ordem ou segredos. Complementa o `MEMORY_ACCESS_PROTOCOL.md`.

---

## 1. O MAPA — comandos de governança/segurança

| Comando | O que faz | Quem roda |
|---------|-----------|-----------|
| `cosca memory register` | registrar aprendizado (portão de 2 fatores — máquina + consentimento) | o Don (via kernel) |
| `cosca memory watch` | vigiar o cofre 24h (watchdog + audit) | o Don / daemon |
| `cosca-check --sign-auto` | assinar chain (git-anchored — testemunho de imutabilidade) | o kernel |
| `cosca-check --sign` | assinar chain (Ed25519 — máquina DPAPI + nonce) | o Don |
| `cosca-check --push` | publicar chain (máquina + nonce + credencial de sessão via GIT_ASKPASS) | o Don |
| `cosca-check --rekey` | gerar par novo (recuperação — só presença/nonce) | o Don |
| `cosca-check --watch` | watchdog (mesmo que `memory watch`) | o Don / daemon |
| `cosca-check` | verificar a family chain | qualquer um |
| `cosca kernel self-test` | integridade do kernel | qualquer um |
| `cosca knowledge verify [--fix]` | integridade do knowledge base | qualquer um |

---

## 2. REGISTRAR um aprendizado — o fluxo exato

`cosca memory register` exige **2 fatores** (o portão reconhece o MOTORISTA, não o carro):

1. **Máquina** (fator 1) — `integrity.VerifyKernelIdentity` desprotege a chave
   Ed25519 da family chain via **DPAPI** (`CryptProtectData`, CurrentUser): o
   vínculo é **máquina+usuário**, sem passphrase para decorar.
2. **Consentimento-ao-conteúdo** (fator 2) — nonce derivado do conteúdo que será
   assinado; comparação em tempo constante; exige **TTY real**. É a presença do Don.

```bash
cosca memory register \
  --title "Título do aprendizado" \
  --level 4 \
  --tags "#dominio #padrao #level-4" \
  --task "o que estava sendo feito" \
  --technique "técnica aplicada" \
  --outcome success \
  --confidence 0.90 \
  --learned "o que foi descoberto" \
  --next "próximo passo" \
  --related "L259 (referência)"
```

O comando então pede (interativamente): o nonce do consentimento-ao-conteúdo.
Sem máquina + consentimento, **nega**. Depois do sucesso, **commit + sign** (ver §5).

**Preview sem gravar**: `--dry-run`.

---

## 3. REKEY — caminho de recuperação (máquina nova)

A chave Ed25519 é **machine-bound** via DPAPI — se a máquina muda, o vínculo
se perde. `cosca-check --rekey` é o **caminho de recuperação**: gera um par
novo e **só exige PRESENÇA (nonce)** — não precisa desproteger a chave antiga.

```bash
printf 'nonce' | ./bin/cosca-check --rekey
git add internal/embed/cosca/keys/kernel_public.key
git commit -m "chave: nova identidade do kernel"
./bin/cosca-check --sign-auto
```

- A chain (git-anchored) **não depende** da Ed25519 — rekey não invalida nada.
- A chave pública precisa ficar nas **3 localizações** (o `--rekey` já cuida):
  `~/.config/cosca/keys/` + `.cosca/keys/` + `internal/embed/cosca/keys/`.

---

## 4. A AUTORIDADE DO DON — Ed25519 vs git-anchor (M7)

M7 é o princípio: **Ed25519** = autoridade do Don; **GIT-ANCHORED** = testemunho
de imutabilidade (sem autoridade). São sempre distinguidos:

| Assinatura | Autoridade | Exige |
|------------|-----------|-------|
| `--sign` / `--push` (Ed25519) | **autoridade do Don** | máquina (DPAPI) + nonce |
| `--sign-auto` (git-anchored) | **testemunho** (sem autoridade) | nada (o commit é a prova) |

A assinatura Ed25519 só acontece porque a chave é desprotegida pela **máquina**
(DPAPI) **e** o Don consente ao conteúdo (nonce em tempo constante, TTY real).
Passphrase e war phrase **não existem mais no fluxo**.

---

## 5. ASSINAR — a ORDEM SAGRADA (L199)

**Regra inegociável: commit ANTES de assinar.**

```bash
git add <arquivos>
git commit -m "..."        # 1º commit
./bin/cosca-check --sign-auto   # 2º assina (git-anchored)
```

- Se mudar qualquer arquivo do `internal/embed/cosca/` e **não** re-assinar, o
  `cosca-check` acusa `GIT COMMIT MISMATCH` no próximo check.
- `--sign-auto` (git-anchor) é o **testemunho de imutabilidade** (sem autoridade);
  `--sign` é o **Ed25519** completo — exige **máquina (DPAPI) + nonce** do Don.
- `--push` também exige **máquina + nonce** e ainda a **credencial de sessão** via
  `GIT_ASKPASS` efêmero — o token **nunca** é gravado em arquivo/config (a
  credencial de push não reside na máquina).

---

## 6. VERIFICAR — checks de integridade

```bash
./bin/cosca-check                      # family chain (210 blocos etc.)
cosca kernel self-test                 # kernel íntegro (leis/princípios)
cosca knowledge verify                 # knowledge base (vetores/chunks)
cosca memory integrity verify          # manifest de integridade da memória
```

---

## 7. VIGILÂNCIA 24h

```bash
cosca memory watch                     # watchdog + audit log (.cosca/audit/memory-watch.log)
cosca memory watch --interval 15       # polling mais frequente
```

Roda como cão de guarda: fsnotify (câmera) + chain (cachorro) + audit log
(quem entra, quem chega perto, quem sai). Pode virar daemon systemd.

---

## 8. GOTCHAS — não repita

1. **A assinatura é machine-bound (DPAPI)** — mudou de máquina/usuário? O
   vínculo se perde; use `--rekey` (caminho de recuperação, só nonce).
2. **Re-signar após TODO commit** que mexe no `internal/embed/cosca/` — senão
   `GIT COMMIT MISMATCH`.
3. **Consentimento-ao-conteúdo exige TTY real** — sem terminal interativo o
   nonce não é aceito (não há passphrase nem war phrase para "pular").
4. **Chave pública em 3 lugares** — `~/.config`, `.cosca/keys/`, git. Se uma
   divergir, `cosca-check` acusa `PUBLIC KEY MISMATCH`.
5. **`memory register` e `memory integrity` rodam FORA da jaula** (admin) — o
   register precisa ler a chave, o integrity audita o manifesto.
6. **Sem máquina (DPAPI) + consentimento (nonce), o register nega** — mesmo o
   kernel não registra sozinho; o Don precisa estar presente na mesma máquina.

---

## 9. HISTÓRICO

| Versão | Data | Mudança |
|--------|------|---------|
| 1.0.0 | 2026-08-16 | Criado por ordem do Don — consolida o CLI de governança/segurança |
| 1.1.0 | 2026-08-22 | Contrato de assinatura atualizado: machine-bound (DPAPI) + consentimento-ao-conteúdo (nonce) — passphrase/war phrase removidas do fluxo; M7 (Ed25519 autoridade vs GIT-ANCHORED testemunho) |
