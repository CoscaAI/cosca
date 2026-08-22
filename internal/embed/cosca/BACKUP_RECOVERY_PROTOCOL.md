# BACKUP RECOVERY PROTOCOL — A rede de segurança

> **Versão**: 1.0.0 | **Status**: canônico | **Dono**: cosca-kernel
> **Criado**: 2026-08-16, por ordem do Don — quando backupar, o quê, como restaurar
> **Propósito**: referência operacional ÚNICA para não perder nada. O Don é o
> guardião dos backups (DESPERTAR) — este protocolo define o rito.

---

## 1. O QUE backupar (por prioridade)

| Ativo | Onde | Método |
|-------|------|--------|
| **Cofre de conhecimento** | `.cosca/knowledge.db` | `sqlite3 .backup` (WAL-safe) |
| **Family chain** | `.cosca/family_chain.dat` | git (o git-anchor já é o backup) |
| **Memória (learnings/blocks)** | `internal/embed/cosca/memory/` | git + `cosca memory snapshot` |
| **Chaves** | `~/.config/cosca/keys/` | machine-bound (DPAPI) — cópia offline do arquivo NÃO transfere o vínculo; máquina nova → `--rekey` |
| **Segredos** | `.cosca/secrets.db` | backup seguro, nunca em git |

---

## 2. QUANDO backupar

- **Antes de TODO DELETE cirúrgico** em DB (L254 — backup de 214MB antes do
  DELETE dos vetores órfãos).
- **Antes de rekey** (troca de identidade).
- **Antes de re-cadear a memória** (rebuild do blockchain).
- **Periodicamente** — snapshot da memória no fim de sessão importante.

---

## 3. COMO backupar

```bash
# DB (WAL-safe, consistente mesmo com o runtime rodando)
sqlite3 .cosca/knowledge.db ".backup '.cosca/knowledge.db.bak-<data>'"

# Memória
cosca memory snapshot create

# Git (a memória versionada já é backup)
git commit -m "..." && ./bin/cosca-check --sign-auto
```

---

## 4. COMO restaurar

```
1. PARAR     → o processo que usa o ativo (nunca restaurar em cima de ativo vivo)
2. COPIAR    → restaurar de backup limpo (não "consertar" o comprometido)
3. VERIFICAR → integridade: cosca-check + kernel self-test + knowledge verify
4. RELIGAR   → subir o serviço, smoke test
```

---

## 5. GOTCHAS

1. **Backup sem testar restauração é ilusão** — um backup que nunca foi
   restaurado pode estar corrompido.
2. **Backup em WAL exige `.backup`, não `cp`** — `cp` do `.db` com WAL ativo
   perde transações (L254).
3. **O git NÃO é backup de segredo** — segredo não entra em git (`.gitignore`).
4. **Chave Ed25519 machine-bound (DPAPI)** — sem passphrase para decorar; o vínculo é
   **máquina+usuário**. Máquina mudou ou chave perdida? Só `--rekey` (gera par novo,
   caminho de recuperação — só presença/nonce).

---

## 6. HISTÓRICO

| Versão | Data | Mudança |
|--------|------|---------|
| 1.0.0 | 2026-08-16 | Criado por ordem do Don — a rede de segurança |
| 1.1.0 | 2026-08-22 | Chave atualizada: machine-bound (DPAPI) — remoção da passphrase; recuperação só via `--rekey` (presença/nonce) |
