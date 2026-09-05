# CAMPAIGN PROMPT-EFF-001 — "Comandos compactos economizam tokens sem perder fidelidade operacional?"

> Registro vivo da campanha de medição da linguagem operacional
> (PROMPT_PROTOCOL — o 30º protocolo). Formato do CAMPAIGN_PROTOCOL v3.0.

## 1. Contrato

| Campo | Valor |
|---|---|
| ID | PROMPT-EFF-001 |
| Question | Os comandos compactos (@SHIP, @PROVE, @I …) economizam tokens de comunicação repetitiva SEM perder fidelidade operacional (task success, evidência, ambiguidade)? |
| Scope | Comunicação de instruções (Don ↔ Kernel) |
| Non-goals | Compressão do contexto interno · compressão de evidência · mudança de comportamento dos protocolos |
| Baseline | L400 — a medição inicial abaixo (tokens reais da sessão PERF) |
| Invariants | STRENGTH(CONCLUSION) ≤ STRENGTH(EVIDENCE) · task success ≥ baseline · evidência preservada |
| Mode | READ_ONLY (medição — análise dos prompts da sessão passada e das próximas) |
| Transformation | FORBIDDEN (o PROMPT_PROTOCOL já está ativo; a campanha SÓ mede) |
| Budget | 100 tarefas · max 2h de medição · 0 mutações |
| STOP IF | evidência contraditória · fidelidade degradada · ambiguidade crítica |

## 2. Baseline medido (L400 — da sessão real)

| Tarefa | Prompt natural | Compacto | Economia |
|---|---|---|---|
| Autorizar a transformação E-007 | ~962 tokens | `@SHIP E007` ~3 | 99.7% |
| Instruir a campanha (professor) | ~1.550 tokens | `@PROVE` ~2 | 99.9% |
| Autorizar o experimento #3 | ~362 tokens | `@I E007 RO G:recall,counts,rank` ~10 | 97.2% |
| Diretriz do H3 | ~700 tokens | `@CAMPAIGN PERF-2026-001 @H3` ~6 | 99.1% |
| **4 tarefas** | **~3.575 tokens** | **~16 tokens** | **99.5%** |

A economia é no **input repetido** (o runtime recupera o contrato persistido).
A ressalva: a fidelidade semântica (a tarefa executada identicamente) é o que
a campanha vai MEDIR nas próximas 100 tarefas — a economia de tokens não basta.

## 3. Métricas (a medir nas 100 tarefas)

INPUT_TOKENS · OUTPUT_TOKENS · TOTAL_TOKENS · LATENCY · CACHE_HIT ·
TASK_SUCCESS · ERROR_RATE · AMBIGUITY_RATE · RECOVERY_RATE ·
**SEMANTIC_FIDELITY** (a execução compacta == a execução natural?).

## 4. Regra de aprovação (do PROMPT_PROTOCOL §9)

```
TOKEN_SAVING > 0
AND TASK_SUCCESS >= baseline
AND EVIDENCE_LOSS = 0
AND CRITICAL_AMBIGUITY = 0
```

Se economizar 70% mas errar decisões → REJECT. Se 40% com equivalência
operacional → PASS.

## 5. Árvore de hipóteses

```
PROMPT-EFF-001
 └── H1: os comandos compactos executam as mesmas tarefas que os naturais
       (task success + fidelidade) → ⏳ (a medir nas 100 tarefas)
     └── H2: a resolução determinística elimina a ambiguidade crítica
           → ⏳ (ERROR: PROMPT_AMBIGUOUS nunca vira chute)
```

## 5b. TAREFAS MEDIDAS

| # | ORDEM (tokens) | EQUIVALENTE_NATURAL (estimado) | ECONOMIA | EXECUÇÃO (fidelidade) | SUCCESS | AMBIG. | EVIDÊNCIA |
|---|---|---|---|---|---|---|---|
| 1 | "ativar protocolo projeto cosca code" (7 tok) | ~60 tok (o pedido completo: verificar o manifest, abrir o projeto, ativar o isolamento do PROJECT_PROTOCOL) | **88%** | ✅ manifest ok + `project open` ok — o protocolo de projeto ATIVO (escopo isolado) | PASS | 0 | manifest (name/version/type editor) + open retornou o cd |
| 2 | "protocolo projeto cosca code" (5 tok, 2026-08-17) | ~60 tok (mesmo pedido da tarefa 1, sessão nova) | **92%** | ✅ PROJECT_PROTOCOL ATIVO — manifest (type editor, v0.1.0) + isolamento verificado (.cosca próprio em project.yaml, sem herdar a chain da família) + repo git próprio (9e7c5b7) | PASS | 0 | `project list` (7 projetos) + `project manifest` (type editor) + `ls .cosca/` (project.yaml 135B) + `git log` |
| 3 | "sim" (1 tok, 2026-08-17) — autoriza fechar a lacuna da ABI | ~35 tok (implementar o comando @PROJECT no registry: contrato completo com MEANING/REQUIRED_STATE/ALLOWED/FORBIDDEN/OUTPUT/GUARDIANS/VERSION) | **97%** | ✅ LACUNA FECHADA — `@PROJECT name v1` criado no PROMPT_REGISTRY (Nível 2, Operações) + PROMPT_PROTOCOL §2 atualizado; contrato read-only (escopo isolado, nunca toca a casa) | PASS | 0 | PROMPT_REGISTRY (contrato @PROJECT) + PROMPT_PROTOCOL §2 (tabela de níveis) + L404 |
| 4 | "@PROJECT cosca-code" (3 tok, 2026-08-17) — após `@PROJCT code` → ERROR: UNKNOWN_COMMAND → confirmação do Don | ~60 tok (o pedido natural de ativar/verificar o PROJECT_PROTOCOL no cosca-code) | **95%** (5 tok no total com o erro + confirmação) | ✅ PROJECT_REPORT emitido — manifest (type editor v0.1.0) + isolamento (project.yaml 135B, sem chain da família) + repo 9e7c5b7; o resolver NÃO chutou (erro explícito + sugestão + confirmação do Don) | PASS | 1 (resolvida por confirmação — não crítica) | `project manifest` + `ls .cosca/` + `git log` |

**Observação registrada (lacuna da ABI — NÃO corrigida):** o registry não tem
comando para ativar um PROJETO (o PROJECT_PROTOCOL não está na ABI — só
@CAMPAIGN). A ordem natural foi coberta pela casa (o PROJECT_PROTOCOL), mas a
ABI não a representaria sem ambiguidade. A apresentar ao Don na conclusão.

**LACUNA FECHADA (tarefa 3, por ordem do Don — "sim"):** `@PROJECT name v1`
adicionado ao PROMPT_REGISTRY (Nível 2 — Operações) e ao PROMPT_PROTOCOL §2.
Contrato read-only: ativar/verificar o PROJECT_PROTOCOL para um projeto
registrado (manifest + isolamento + repo). Nunca muta o projeto nem toca a
casa. A partir de agora "protocolo projeto <nome>" resolve deterministicamente
via registry (sem lacuna).

## 6. Provenance

L400 (`6971b1a`) — PROMPT_PROTOCOL + registry assinados.
