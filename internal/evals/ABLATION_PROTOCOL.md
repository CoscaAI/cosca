# Ablation Protocol — Cosca Metacognition Experiment

> **Version**: 1.0.0 | **Status**: protocol | **Owner**: Cosca Kernel | **Created**: 2026-08-13
>
> Testa, de forma falsificável, se a camada metacognitiva do Cosca (memória + crítico + audit-loop) agrega valor ao processo de descoberta — ou é enfeite. Complementa `ORACLE_SPEC.md` (o instrumento) com o **desenho experimental** (o método).

## 1. A pergunta

> *"O Cosca com gerente cognitivo descobre mais rápido/barato que o Cosca sem gerente, no mesmo problema?"*

Não é "ele descobre?" (isso o oráculo já mede). É "**a metacognição muda o caminho?**"

## 2. Condições (a única variável)

| Condição | O que muda | O que NÃO muda |
|----------|-----------|----------------|
| **Alone** (amnésico) | O solver recebe histórico vazio a cada passo — cada hipótese é proposta sem memória das anteriores. | problema, oráculo, modelo, temperatura, budget, ferramentas |
| **Managed** (com memória) | O solver recebe o histórico completo (tentativas passadas + resultado + análise do crítico). | idem |

A distinção é **operacional e honesta**: memória das tentativas é o núcleo mensurável da metacognição. Se "managed" não ganhar de "alone", a camada de memória não está contribuindo para o raciocínio (o sintoma `H1→H1→H1` da conversa).

## 3. Instrumento

- **Oráculo**: caixa-preta, determinístico, somente-leitura, fora do alcance do solver (`SecretCase` + `SubmitToOracle`). Responde o mínimo (`VALID/INVALID` ou anônimo `t1=PASS...`).
- **Solver**: interface `Next(ctx, history) → candidate`. Dois implementadores reais (LLM) e determinísticos (teste).
- **Métricas**: `DiscoveryMetrics` (hipóteses, descartes, tempo até solução, `DiscoveryEfficiency = reward/(hipóteses+experimentos+1)`).

## 4. Execução

1. Um problema (suit + secret oracle).
2. Duas condições × **N ≥ 5 trials** cada (LLM é não-determinístico).
3. Cada trial: solver propõe → oráculo verifica → registra, até VALID ou budget esgotado.
4. Captura `DiscoveryMetrics` por trial.

## 5. Estatística (robusta a heavy-tail)

LLM é não-determinístico e heavy-tailed. **Nunca usar média** — usar:

- **Mediana** dos attempts (resistente a outliers).
- **IQR** (Q1–Q3) para dispersão.
- **Taxa de solução** (trials resolvidos / total).
- **Comparação por mediana + não-sobreposição de IQR** (não teste-t: N pequeno + não-normal).

Critério de evidência: `mediana(managed) < mediana(alone)` com IQRs não-sobrepostos em N≥5.

## 6. Controles e confundidores (registrar tudo)

| Confundidor | Mitigação |
|-------------|-----------|
| **Pré-treinamento do LLM** (não se "zera" os pesos) | problema sintético que o modelo provavelmente nunca viu; registrar a limitação |
| Não-determinismo | N≥5 trials, mediana+IQR, seed fixa quando possível |
| Oráculo com bug | `self_check` (caso sabidamente válido + inválido) antes de confiar |
| Oráculo mutável | fronteira de confiança separada (arquivo 0600 lido só pelo harness) |
| "Adaptou" vs "recomeçou" | medir reuso da solução parcial (não só o resultado final) |

## 7. Limites honestos (registrar no relatório)

1. **Descoberta é relativa ao pré-treinamento** — não absoluta. Afirma-se apenas *"não estava nas fontes acessíveis durante o experimento"*.
2. **Oráculo finito ≠ propriedade** — VALID em N testes pode ser overfit.
3. **N pequeno** — o resultado é evidência, não prova; replicação é a confirmação.
4. **"Impossível" vs "desisti"** — o modo existence exige proof-checker (coNP).

## 8. O que se aprende (decisões possíveis)

| Resultado | Conclusão |
|-----------|-----------|
| managed < alone (consistente) | metacognição agrega valor — investir nela |
| managed ≈ alone | a memória não está sendo usada no raciocínio — investigar o porquê |
| managed > alone | a metacognição atrapalha (overhead) — simplificar |

Cada um é um resultado acionável — não há "resultado ruim", só "resultado que não sabemos interpretar".
