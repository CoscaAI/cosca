# CLEANUP PROTOCOL — Como varrer a sujeira da casa

> **Versão**: 1.0.0 | **Status**: canônico | **Dono**: cosca-kernel
> **Criado**: 2026-08-16, por ordem do Don — o rito da varredura (nasceu do L274)
> **Propósito**: referência operacional ÚNICA para identificar e varrer a sujeira
> sem tocar no que importa. Complementa o `BACKUP_RECOVERY_PROTOCOL.md` (a rede
> de segurança) e o `SECURITY_PROTOCOL.md` (a casa blindada).

---

## 1. O QUE É "sujeira" (as 7 categorias)

| Categoria | Exemplo real | Por que é sujeira |
|-----------|--------------|-------------------|
| **Build artifacts** | `cosca-guard*` (18 cópias de 154MB), `cosca-embed`, `cosca-sym` | binário regenerável |
| **Clones de mineração** | `/eigen/`, `ccxt`, `barter-rs` | clone externo com `.git` próprio |
| **System dirs vazios** | `dev/ etc/ lib/ lib64/ proc/ tmp/ usr/` | vazamento de mount da jaula |
| **Build cache** | `go-build-cache/`, `node_modules/` | cache regenerável |
| **Aninhamento errado** | `internal/memory/memory/` | botched dir (memory dentro de memory) |
| **Backup redundante** | `knowledge.db.bak-*-orphans` | o git + auto-backup já cobrem |
| **Temp files** | `/tmp/opencode/` (blocos, scripts, testes) | trabalho temporário |

---

## 2. COMO identificar (os detectores)

```bash
git clean -nd                  # untracked (dry-run, não remove nada) — o melhor detector
find . -type f -size +20M -not -path "./.git/*"   # arquivos grandes
find . -name ".git" -type d -not -path "./.git"   # clones externos
du -sh <dir>                   # quanto cada sujeira pesa
```

**`git clean -nd` é o rei** — mostra exatamente o que é untracked sem remover.
Rode sempre ANTES de qualquer varredura.

---

## 3. COMO varrer (o processo)

```
1. DETECTAR  → git clean -nd + find (o que é sujeira?)
2. CLASSIFICAR → qual das 7 categorias? (só sujeira é removida)
3. PROTEGER  → conferir a lista do §4 (nunca varrer o que importa)
4. VARRER    → rm -rf (com chmod -R u+w se cache read-only)
5. VERIFICAR → git status limpo + integridade (chain/kernel/memória)
```

---

## 4. O QUE NUNCA varrer (lista de proteção)

| Ativo | Por quê |
|-------|---------|
| `.cosca/knowledge.db` | o cofre de conhecimento |
| `.cosca/family_chain.dat` | a chain da família |
| `internal/embed/cosca/` | o cérebro (identidade, memória, protocolos) |
| `~/.config/cosca/keys/` | as chaves Ed25519 |
| `internal/embed/cosca/memory/agent/cosca-*/` | memória dos agentes REAIS |
| `.cosca/backups/auto-*` | a rede de segurança (BACKUP_RECOVERY) |

**Regra**: se está no git OU é um ativo listado acima, NÃO é sujeira.

---

## 5. QUANDO varrer

- **Fim de sessão** (SESSION_PROTOCOL §5) — varredura rápida.
- **Antes de auditoria** — casa limpa antes de medir.
- **Quando o disco aperta** — os build artifacts são os maiores vilões.
- **Quando o Don manda** — "varre a sujeira pra fora".

---

## 6. GOTCHAS

1. **Go module cache é read-only** (0444) — `rm -rf` falha com "Permission denied";
   faça `chmod -R u+w <dir>` antes (L274).
2. **A jaula vaza system dirs** — quando o bwrap monta o workspace como `/`, ele
   cria `dev/`, `etc/`, `proc/` vazios. A varredura limpa; o FIX é na jaula.
3. **`git clean -nd` é dry-run** — o `-n` não remove; o `-f` remove. Nunca rode
   `git clean -f` sem o `-nd` primeiro.
4. **Backup redundante ≠ backup de segurança** — o manual pode ir; o auto fica.
5. **Verificar depois** — varreu? Re-verifique a integridade (não confiar no "removi").

---

## 7. HISTÓRICO

| Versão | Data | Mudança |
|--------|------|---------|
| 1.0.0 | 2026-08-16 | Criado por ordem do Don — o rito da varredura (nasceu do L274) |
