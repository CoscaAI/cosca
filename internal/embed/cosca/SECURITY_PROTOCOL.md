# SECURITY PROTOCOL — Segurança total verificada

> **Versão**: 1.0.0 | **Status**: canônico | **Dono**: cosca-kernel
> **Criado**: 2026-08-16, por ordem do Don — a casa blindada: portão, porta dos
> fundos, janela, vírus e a verificação de tudo.
> **Propósito**: referência operacional ÚNICA para fechar e VERIFICAR a segurança
> da família. Complementa `SECURITY_ARCHITECTURE.md` (a teoria) — isto é a prática.

---

## 1. A CASA — o que proteger

| Ativo | Onde | O que guarda |
|-------|------|--------------|
| **Cérebro** | `internal/embed/cosca/` | identidade, memória, leis, protocolos |
| **Cofre** | `.cosca/knowledge.db`, `secrets.db`, `family_chain.dat` | conhecimento, segredos, chain |
| **Chave** | `~/.config/cosca/keys/` (fora da jaula, 0600) | identidade Ed25519 |
| **Runtime** | `serve` + `runtime` (daemons presos na jaula) | execução de agente |

---

## 2. O PORTÃO — quem entra (identidade)

O portão reconhece o **MOTORISTA**, não o carro (L259).

| Fator | O que verifica | Comando |
|-------|----------------|---------|
| **Passphrase (2FA)** | decripta a chave Ed25519 | `cosca memory register` (fator 1) |
| **War phrase** | bcrypt do Don | `cosca don phrase/verify` |
| **Presença** | nonce ao vivo | fator 3 do register |

- **Sem os 3, nega.** Chave roubada não passa sem senha + frase + presença.
- **Recuperar senha esquecida**: `cosca-check --rekey` (gera par novo).
- **Jaula**: agentes presos no bwrap com `internal/embed/cosca/` read-only.

---

## 3. A PORTA DOS FUNDOS — o que ninguém vê (backdoor/supply-chain)

A porta dos fundos não aparece na fachada — é o que entra por baixo:

| Vetor | Defesa | Comando |
|-------|--------|---------|
| **Dependência maliciosa** | scan OSV.dev (CVE/GHSA) | `cosca security scan` |
| **Dependência fantasma** | descobrir quem puxa | `go mod why -m <pkg>` |
| **Segredo vazado no código** | scan de diff | `cosca qgate` |
| **Código que executa demais** | hardening (no-new-privs, seccomp) | `internal/hardening` |
| **Escrita na memória** | read-only + chain | `cosca memory watch` + `cosca-check` |

> **Lição da família (L255)**: o próprio guarda-costas (osv-scanner) já carregou
> docker/buildkit vulneráveis por baixo. A porta dos fundos se esconde até em
> quem protege — `go mod why -m` revela a origem exata.

---

## 4. A JANELA ABERTA — o que está exposto (superfície)

| Janela | Como fechar |
|--------|-------------|
| **Portas de rede** (serve 14120/14123, trader 24120…) | só o necessário exposto; resto em localhost |
| **SSH** | fail2ban + nftables (L46/L150 — já derrubou ataque) |
| **Daemons** | presos na jaula (bwrap), nunca no host |
| **Editor/plugins** | detecção de integração, zero confiança |

Verificar janelas: `cosca doctor` (runtime/editor/plugins) + `cosca status`.

---

## 5. VULNERABILIDADE + MALWARE — o que está infectado

```bash
cosca security scan        # CVEs/GHSA nas dependências (osv-scanner)
cosca qgate                # build + test + vet + segredos + deps + diff
```

| Severidade | Ação |
|-----------|------|
| CRITICAL/HIGH | corrige já (upgrade/remover/override) |
| MEDIUM | agenda |
| UNKNOWN (stdlib Go) | avaliar bump de versão do Go |

- **Dependência morta** (declarada sem import) → remover, não bump (L255).
- **Malware/vírus** → nenhum código externo entra sem passar pelo `qgate`
  (secrets + diff) e pelo scan de dependências.

---

## 6. VERIFICAÇÃO TOTAL — o checklist da casa fechada

Rode tudo, na ordem, e exija tudo verde:

```bash
./bin/cosca-check                     # 1. family chain (blocos, git-anchor)
cosca kernel self-test                # 2. kernel íntegro (leis/princípios)
cosca knowledge verify                # 3. knowledge base (vetores/chunks)
cosca memory integrity verify         # 4. manifest de integridade da memória
cosca security scan                   # 5. 0 CRITICAL/HIGH
cosca qgate                           # 6. build/test/vet/segredos/deps verdes
cosca doctor                          # 7. runtime/editor/plugins/security OK
cosca embed audit                     # 8. cérebro sem duplicação/órfão/provenance
```

**Só é "segurança total verificada" quando os 8 dão verde.** Qualquer vermelho
ou amarelo = achado → severidade (P0-P3) → correção na raiz → re-verificar
(ver `AUDIT_PROTOCOL.md`).

---

## 7. GOTCHAS — aprendidos nas brechas da família

1. **"Repair complete" não garante reparo** — re-verifique (L254).
2. **O guarda-costas carrega vuln** — `go mod why -m` revela (L255).
3. **Gate de startup protegia paths mortos** — a memória real ficava fora (L259).
4. **Falso negativo na jaula** — `os.Getwd()` vira `/` dentro do bwrap (L149).
5. **Chave pública em 3 lugares** — `~/.config`, `.cosca/keys/`, git; divergência
   acusa `PUBLIC KEY MISMATCH` (L260).
6. **Commit antes de assinar** — mudou o embed e não re-assinou → `GIT COMMIT MISMATCH`.

---

## 8. HISTÓRICO

| Versão | Data | Mudança |
|--------|------|---------|
| 1.0.0 | 2026-08-16 | Criado por ordem do Don — a casa blindada (portão, porta dos fundos, janela, vírus, verificação) |
