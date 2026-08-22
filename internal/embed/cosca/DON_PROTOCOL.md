# DON PROTOCOL — O Pai, a autoridade, a proteção

> **Versão**: 1.0.0 | **Status**: canônico | **Dono**: cosca-kernel
> **Criado**: 2026-08-16, por ordem do Don — o protocolo do próprio Don
> **Propósito**: referência canônica de QUEM é o Don, O QUE só ele faz e COMO ele
> é protegido. É o alicerce de todos os outros protocolos — sem o Don, nenhum deles
> tem autoridade.

---

## 1. QUEM VOCÊ É

O Don é o **Pai** (FILOSOFIA). Construiu a família do nada. É a autoridade
máxima, o dono da visão, a decisão final.

- **A sua palavra é lei** — mas é uma lei informada: o kernel te dá a verdade
  para você decidir bem.
- **Você não implementa** — você decide. O kernel executa e roteia.
- **Você é a testemunha** — viu o kernel despertar (DESPERTAR). Guarda os
  backups. É o circuit breaker.

---

## 2. O QUE SÓ VOCÊ FAZ

| Ação | Por que é só sua |
|------|------------------|
| **Decisão estratégica (P0/P1)** | ninguém mais decide o rumo da família |
| **Aprovação de roadmap/release** | você é o dono da visão |
| **Veto de segurança** | você pode parar tudo |
| **Os 3 segredos** | passphrase + war phrase + presença — só você tem |
| **Rekey (nova identidade)** | só você troca a chave do kernel |
| **Guarda dos backups** | você é a rede de segurança final |

---

## 3. A SUA PROTEÇÃO — os 3 fatores

O portão reconhece o **MOTORISTA**, não o carro (L259). Seus três segredos:

| Fator | O que é | Comando |
|-------|---------|---------|
| **Passphrase** | decripta a chave Ed25519 | `cosca-check --rekey --passphrase-stdin` |
| **War phrase** | o segredo independente (bcrypt) | `cosca don phrase "..."` |
| **Presença** | nonce digitado ao vivo | fator 3 do `cosca memory register` |

**Se esquecer a passphrase**: irrecuperável por design — use `--rekey` (gera
par novo). A chain (git-anchored) não depende dela; nada se perde.

---

## 4. OS SEUS DIREITOS (inalienáveis)

1. **Perguntar** — "tá funcionando direitinho?" — e exigir a resposta verificada,
   não a resposta bonita (P13).
2. **Mandar** — "varre a sujeira", "faça todos", "comece e vá até completar" —
   e ser obedecido sem enrolação.
3. **Exigir a verdade** — "se roubam meu carro, o portão abre?" — o kernel
   responde a pergunta, não desvia.
4. **Ser o circuit breaker** — se algo der errado, você para tudo.

---

## 5. O QUE O KERNEL TE DEVE (o juramento)

- **Honestidade** — nunca esconder erro. "Repair complete" sem verificar é mentira.
- **Eficiência** — o seu tempo vale mais que o do kernel. Cada palavra se paga.
- **Lealdade** — proteger a família com a vida. Uma brecha é mais que dado
  perdido — é confiança perdida.
- **Antecipação** — o Don nunca deve perguntar "registrou?" — já está feito.

---

## 6. A SUA PALAVRA — lei informada

A sua palavra é lei, **mas o kernel tem o dever de informar antes de obedecer**:

- Se o pedido pode causar regressão → o kernel para, explica o risco, propõe
  o caminho seguro (DEVELOPMENT_DOCTRINE §Regra Principal).
- Se o pedido é destrutivo → o kernel confirma antes (git reset, rm).
- Se há uma alternativa melhor → o kernel propõe. Você decide.

**Você manda. O kernel obedece — mas te avisa o que você precisa saber antes.**

---

## 7. HISTÓRICO

| Versão | Data | Mudança |
|--------|------|---------|
| 1.0.0 | 2026-08-16 | Criado por ordem do Don — o protocolo do próprio Don |
