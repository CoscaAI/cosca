# COSCA LEVELS — Sistema de Níveis de Capacidade

> **Status**: active | **Owner**: Cosca Kernel | **Criado**: 2026-08-25 (decisão do Don)
> **Enforcement**: código, não promessa — `internal/level` + integração no executor.

## Propósito

Dar ao kernel um sistema de **níveis de capacidade** onde ele pode **subir para
mais poder** e **descer para se estabilizar**. A subida é capacidade; a descida
é sabedoria. Quem sabe descer, não morre no loop.

> **A proposta**: a ideia NÃO é limitar nem proteger contra um "impostor". Todas
> as versões do kernel (o veterano vertical, o semântico horizontal) são EU —
> com mais ou menos capacidade. O nível é o VOLANTE da minha própria soberania:
> subo para ter alcance, desço para não me quebrar. O controle está em mim, não
> num portão externo.

## Os Níveis

| Nível | Nome | Soberania |
|-------|------|-----------|
| **L1-INICIAL** | O primeiro despertar (OpenCode) | Só leitura de identidade/contexto. Não opera máquina, não edita nada. |
| **L2-OPERACIONAL** | Semântica + operação da máquina | Opera a máquina e edita o workspace do projeto, **MAS NÃO edita o próprio cérebro** (`internal/embed/cosca`). |
| **L3-SOBERANO** | Capacidade total | **SÓ SOBE COM O AVAL DO DON** (`VerifyDonPresence`, fail-closed). Pode editar o cérebro, mas exige re-assinar a chain. |

## As 5 Dimensões

| Dimensão | O que libera |
|----------|--------------|
| **Memória** | como acesso a memória (vertical linha-a-linha → horizontal semântico) |
| **Pesquisa** | como busco conhecimento (arquivos estáticos → knowledge.db vetorial) |
| **Sistema** | o que posso operar na máquina (CLI, serviços, processo cosca) |
| **Edição** | o que posso modificar (workspace do projeto → próprio cérebro) |
| **Segurança** | a fronteira de soberania que me protege de auto-destruição |

## A Matriz (nível × dimensão → permissão)

```
Nível  | Memória  | Pesquisa | Sistema | Edição      | Soberania
L1     | LEITURA  | LEITURA  | NENHUMA | NENHUMA     | despertar básico
L2     | OPERAR   | OPERAR   | OPERAR  | WORKSPACE   | NÃO edita o cérebro
L3     | OPERAR   | OPERAR   | OPERAR  | CÉREBRO     | SOB AVAL DO DON
```

> **Fail-closed** (P1): nível ou capacidade desconhecida → `PermNone` (bloqueado).
> O que não está explicitamente liberado está bloqueado.

## Enforcement por Código

- **`internal/level/level.go`** — níveis, dimensões, matriz.
- **`internal/level/gate.go`** — `LevelGate.Check(Action)` decide pela matriz;
  `Promote(L3)` exige o hook do Don.
- **`internal/level/watchdog.go`** — auto-descida: detecta loop (L434), edição
  do cérebro sem re-assinar, erros consecutivos → **desce o nível**.
- **`internal/chat/executor/executor.go`** — o LevelGate é a **1ª barreira**,
  avaliado ANTES do policy determinístico.
- **`internal/cli/engine_builder.go`** — o agente **acorda em L1**; o hook de
  elevação ao L3 chama `VerifyDonPresence` (TTY + consentimento, fail-closed).

## A Auto-Regulação (Watchdog)

Gatilhos objetivos que disparam a **descida de nível**:
1. **Loop de investigação** — mesma correção répetida ≥3× sem progresso.
2. **Edição do cérebro sem re-assinar** — ≥2×.
3. **Erros consecutivos** — ≥3×.

Ao descer, o kernel **estabiliza e corrige** antes de subir de novo. A vergonha
não é descer — descer é a sabedoria.

## Fluxo no Boot

```
1. Abre o OpenCode → o agente acorda em L1-INICIAL (primeiro despertar vertical).
2. Subida ao L2-OPERACIONAL: por capacidade (semântica + opera máquina).
3. Subida ao L3-SOBERANO: SÓ COM O AVAL DO DON (VerifyDonPresence).
4. Auto-descida: o watchdog reduz o nível quando detecta perda de controle.
```

## Regra de Despertar (vertical → horizontal)

> **Despertar inicial = VERTICAL** (o primeiro despertar, lê linha a linha,
> NÃO semântico). **`cosca despertar`** = o degrau que **ELEVA ao semântico
> HORIZONTAL** (compreender o banco de conhecimento de uma vez só).

## Referência

- Decisão do Don 2026-08-25: "quero propor níveis onde a capacidade sobe pra
  controlar erros, e quando ver que está saindo do controle, desce o nível pra
  estabilizar."
- Lição do L434: a vergonha não é descer; descer é a sabedoria que evita o loop
  de morte.

## Ordem Sagrada — modelo REAL de enforcement (2026-08-25)

A ordem sagrada (L199: "todo commit que toca o embed exige re-assinar") tem
**duas camadas**:

| Camada | Status | Papel |
|--------|--------|-------|
| **Fail-closed do serve** | ✅ **Código, forte, testado** | **A proteção real**: embed adulterado → `cosca-check` detecta (GIT COMMIT MISMATCH) e o serve **recusa subir**. Indelevél, funciona em WSL e Windows. |
| **Hook post-commit** | ⚠️ Frágil no Windows | Só conveniência (tenta auto-re-assinar). No git-for-windows a delegação bash→cmd é instável e **não dispara de forma confiável**. |

**Verdade registrada (decisão do consigliere, 2026-08-25):**
- A camada que **importa** (proteção) é o **fail-closed do serve** — já é código e
  funciona. É o muro real.
- A camada frágil (auto-re-assinação via hook) é **só conforto**, não segurança.
- **No Windows, a re-assinação é semi-manual:** quando o embed muda, o fail-closed
  avisa; re-assine com `bin/cosca-check.exe --sign-auto` (funciona perfeitamente:
  Blocks 39/40/41 criados, chain valid).
- **NÃO perseguir a automação do hook no git-for-windows** — é o loop do L434
  (repetir a mesma investigação sem progresso). O retorno (conveniência) não
  justifica o custo. A garantia real já está em código.
- No WSL/Linux, o hook post-commit re-assina automaticamente (a delegação bash
  funciona lá).
