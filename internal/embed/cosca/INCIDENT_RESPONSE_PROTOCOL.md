# INCIDENT RESPONSE PROTOCOL — O que fazer quando a casa pega fogo

> **Versão**: 1.0.0 | **Status**: canônico | **Dono**: cosca-kernel
> **Criado**: 2026-08-16, por ordem do Don — a resposta, não só a prevenção
> **Propósito**: referência operacional ÚNICA para responder a incidente (brecha,
> falha, perda) sem improvisar no pânico. Complementa o `SECURITY_PROTOCOL.md`.

---

## 1. SEVERIDADE — classificar primeiro

| Nível | Definição | Resposta |
|-------|-----------|----------|
| **P0** | brecha ativa, exfiltração, comprometimento, perda de dados | < 15 min, parar tudo |
| **P1** | vuln explorável, serviço fora, segredo vazado | < 1 hora |
| **P2** | CVE não-crítico, atividade suspeita, violação de política | < 4 horas |
| **P3** | configuração errada, dependência velha | < 24 horas |

---

## 2. O PLAYBOOK — 6 passos

```
1. DETECT     → o alerta (watchdog, scan, chain quebrou, Don avisou)
2. TRIAGE     → classificar P0-P3 (não subestimar, não superestimar)
3. CONTAIN    → isolar o alvo, rotacionar segredos, bloquear origem
4. ERADICATE  → remover a CAUSA RAIZ (não o sintoma), corrigir
5. RECOVER    → restaurar de backup limpo, verificar integridade
6. LEARN      → post-mortem + registrar (L-number) + melhorar defesa
```

---

## 3. INCIDENTES reais da família (o que aprendemos)

| Incidente | Lição |
|-----------|-------|
| **L46** — comprometimento do cofre (knowledge wipe) | backup primeiro, forense depois; sem sandbox = sem defesa |
| **L150** — fail2ban caído 15h | verificar defesa ativa periodicamente, não só na hora do ataque |
| **L13** — jail breach | single-owner model, restart-counter, sem ponto único |
| **L254** — "repair complete" falso | nunca confiar na mensagem de sucesso — re-verificar |

---

## 4. CONTAIN — as primeiras ações (P0/P1)

1. **Isolar** — derrubar o serviço/processo comprometido (kill -TERM, nunca pkill — L56).
2. **Rotacionar** — todo segredo exposto (JWT, API key, senha) gira na hora.
3. **Bloquear** — a origem do ataque (fail2ban, nftables, IP).
4. **Congelar** — não mexa no alvo antes de tirar evidência (forense).

---

## 5. ERADICATE + RECOVER

- **Causa raiz, não sintoma** — o "arquivo fantasma" (L219) foi corrigido no DSN,
  não no sintoma.
- **Restaurar de backup limpo** — nunca "consertar" em cima do comprometido.
- **Verificar integridade** — `cosca-check` + `cosca kernel self-test` + os 8
  checks do SECURITY_PROTOCOL §6.

---

## 6. BUG FINGERPRINT — agrupar o MESMO bug antes de responder

Antes de responder a um incidente, a casa pergunta: **"esse bug já mordeu antes?"**
O fingerprint agrupa bugs iguais por campos ESTRUTURADOS (`component:error`),
não pela descrição (que muda a cada relato).

```bash
cosca bug register --component auth --error "token expired" [--path --phase]  # BUG-XXXX ou ++família
cosca bug match --component auth --error "token expired"                       # família existente?
cosca bug family auth:token-expired                                            # todos da família
cosca bug list                                                                 # todos (id, família, vezes visto, status)
```

**Regras:**
- O fingerprint é `component:error`; path/phase só entram se AMBOS baterem.
- Bug repetido = **incrementa a família existente** (não abre BUG novo) —
  a frequência é dado de severidade (P0/P1 candidate).
- Registrar o bug ANTES de consertar (forense primeiro, gotcha §6.2).

---

## 7. GOTCHAS

1. **Pânico é o pior incidente** — o playbook existe para não improvisar.
2. **Nunca apagar evidência** — forense antes de limpar.
3. **Post-mortem é obrigatório** — incidente sem L-number é incidente que volta.
4. **"Consertado" não é "verificado"** — re-audite (L254).
5. **Fingerprint antes de consertar** — sem registro, o bug volta sem família.

---

## 8. HISTÓRICO

| Versão | Data | Mudança |
|--------|------|---------|
| 1.0.0 | 2026-08-16 | Criado por ordem do Don — a resposta ao incidente |
| 1.1.0 | 2026-08-16 | §6 Bug Fingerprint incorporado (análise de lacunas de protocolos) |
