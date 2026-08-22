# CLI PROTOCOL — Como operar o Cosca pela linha de comando

> **Versão**: 1.0.0 | **Status**: canônico | **Dono**: cosca-kernel
> **Criado**: 2026-08-16, por ordem do Don — consolidar o uso do CLI de governança/segurança
> **Propósito**: referência operacional ÚNICA para operar o CLI sem precisar adivinhar
> flags, ordem ou segredos. Complementa o `MEMORY_ACCESS_PROTOCOL.md`.

---

## 1. O MAPA — comandos de governança/segurança

| Comando | O que faz | Quem roda |
|---------|-----------|-----------|
| `cosca memory register` | registrar aprendizado (portão de 3 fatores) | o Don (via kernel) |
| `cosca memory watch` | vigiar o cofre 24h (watchdog + audit) | o Don / daemon |
| `cosca-check --sign-auto` | assinar chain (git-anchored, sem passphrase) | o kernel |
| `cosca-check --sign --passphrase-stdin` | assinar chain (Ed25519, com passphrase) | o Don |
| `cosca-check --rekey --passphrase-stdin` | gerar par novo (recuperar passphrase esquecida) | o Don |
| `cosca-check --watch` | watchdog (mesmo que `memory watch`) | o Don / daemon |
| `cosca-check` | verificar a family chain | qualquer um |
| `cosca don phrase "<frase>"` | armar a war phrase (fator 2) | o Don |
| `cosca don verify "<frase>"` | verificar a war phrase | o Don |
| `cosca kernel self-test` | integridade do kernel | qualquer um |
| `cosca knowledge verify [--fix]` | integridade do knowledge base | qualquer um |

---

## 2. REGISTRAR um aprendizado — o fluxo exato

`cosca memory register` exige **3 fatores** (o portão reconhece o MOTORISTA, não o carro):

1. **Passphrase** (fator 1, 2FA) — decripta a chave Ed25519. Vem do env
   `COSCA_KERNEL_PASSPHRASE` ou é digitada no prompt.
2. **War phrase** (fator 2) — verifica contra o bcrypt (`.cosca/don.phr`).
3. **Presença** (fator 3) — nonce aleatório digitado de volta ao vivo.

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

O comando então pede (interativamente): passphrase → frase de guerra → nonce.
Sem os 3, **nega**. Depois do sucesso, **commit + sign** (ver §5).

**Preview sem gravar**: `--dry-run`.

---

## 3. REKEY — recuperar passphrase esquecida

A passphrase é irrecuperável (a chave é criptografada com AES-256-GCM — de propósito).
Para resetar: gera um par NOVO com uma senha nova.

```bash
printf 'SUA_SENHA_NOVA\n' | ./bin/cosca-check --rekey --passphrase-stdin
git add internal/embed/cosca/keys/kernel_public.key
git commit -m "chave: nova identidade do kernel"
./bin/cosca-check --sign-auto
```

- A chain (git-anchored) **não depende** da Ed25519 — rekey não invalida nada.
- A chave pública precisa ficar nas **3 localizações** (o `--rekey` já cuida):
  `~/.config/cosca/keys/` + `.cosca/keys/` + `internal/embed/cosca/keys/`.

---

## 4. WAR PHRASE — armar/verificar (fator 2)

```bash
cosca don phrase "minha frase secreta"   # arma (grava só o hash bcrypt)
cosca don verify "minha frase secreta"   # verifica
cosca don status                          # está armada?
cosca don attempts                        # trilha de tentativas (audit)
```

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
- `--sign-auto` (git-anchor) é sem passphrase; `--sign --passphrase-stdin` é o
  Ed25519 completo (exige a passphrase do Don).

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

1. **`--passphrase-stdin`**, não `--passphrase-stdi` (erro comum de digitação).
2. **Re-signar após TODO commit** que mexe no `internal/embed/cosca/` — senão
   `GIT COMMIT MISMATCH`.
3. **Passphrase esquecida = irrecuperável** — use `--rekey` (não tem "recuperar").
4. **Chave pública em 3 lugares** — `~/.config`, `.cosca/keys/`, git. Se uma
   divergir, `cosca-check` acusa `PUBLIC KEY MISMATCH`.
5. **`memory register` e `memory integrity` rodam FORA da jaula** (admin) — o
   register precisa ler a chave, o integrity audita o manifesto.
6. **Sem os 3 fatores, o register nega** — mesmo o kernel não registra sozinho;
   o Don precisa estar presente (passphrase + frase + nonce).

---

## 9. HISTÓRICO

| Versão | Data | Mudança |
|--------|------|---------|
| 1.0.0 | 2026-08-16 | Criado por ordem do Don — consolida o CLI de governança/segurança |
