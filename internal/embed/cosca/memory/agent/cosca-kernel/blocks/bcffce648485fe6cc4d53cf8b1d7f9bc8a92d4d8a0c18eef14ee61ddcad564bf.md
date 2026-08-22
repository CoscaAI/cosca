PREV: a30175f541faebd60f4c83429a1a5f0943465bdaffb8a237485b81aa0a6c09bb
ID: L141
TIME: 2026-08-13
LEVEL: 4
TAGS: #hipotese #justificativa #experimental-vs-tentativa #structured-output #parse #ablacao #level-4
---
## L141 — 2026-08-13 — Captura da hipótese/justificativa na ablação: separa "tentativa-e-erro" de "experimental dirigido" | Level 4

| Field | Value |
|-------|-------|
| **Agent** | cosca-kernel |
| **Task** | Don's order "o que achar melhor" — evoluir o harness pra capturar a justificativa (a lacuna que a conversa expôs: distinguir tentativa-e-erro de experimentação hipótese-dirigida). |
| **Technique** | Level 4 — **(1)** `Solver.Next` agora retorna `(candidate, hypothesis, err)`; `Attempt.Hypothesis` guarda a justificativa; `RunTrial` a registra no log bruto. **(2)** `buildPrompt` trocou "responda SÓ o candidato" (que SUPRIMIA o raciocínio) por saída estruturada `HYPOTHESIS:` + `CANDIDATE:`; `ParseStructured` extrai ambos (tolerante a prosa e a campos ausentes). **(3)** Limitação honesta registrada: a hipótese é o raciocínio DECLARADO pelo modelo, não o estado interno real (ground-truth) — é o melhor sinal disponível, não prova.** (4)** 4 testes novos (ParseStructured, campo ausente, captura de hipótese no LLMSolver, e o teste-chave `TestRunTrialCapturesHypothesis`). **(5)** Smoke real com LLM revelou a distinção: ALONE diz "let's start" e repete; MANAGED diz "the previous attempt failed, SUGGESTING that..." (raciocina sobre a evidência). |
| **Level** | 4 |
| **Outcome** | success — A hipótese agora é dado, não ruído. O smoke real capturou a diferença qualitativa: alone "no information, let's start" (chute) vs managed "based on previous results, suggesting..." (hipótese-dirigido). A lacuna que a conversa expôs ("respond only the candidate" suprimia a justificativa) está fechada. |
| **Confidence** | 0.93 (validado: 4 testes novos + smoke LLM real + build/vet limpos) |
| **Tags** | #hipotese #justificativa #experimental-vs-tentativa #structured-output #parse #ablacao #level-4 |
| **Related** | L239 (primeira rodada real), L238 (LLMSolver), conversa do Don (distinguir experimental de tentativa-e-erro) |
| **Learned** | **(1) O erro de design que a conversa expôs: "respond only the candidate" resolvia o parsing MAS suprimia o raciocínio — a justificativa era jogada fora. A correção é saída estruturada (HYPOTHESIS:/CANDIDATE:) que captura a justificativa SEM sacrificar o parsing.** **(2) A distinção tentativa-e-erro vs experimental aparece QUALITATIVAMENTE na hipótese: "no information, let's start" (chute, amnésico) vs "previous attempt failed all three, SUGGESTING that..." (raciocina sobre evidência, com memória) — o campo Hypothesis é o que revela isso.** **(3) A hipótese declarada pelo LLM é POST-HOC (narração, não o estado interno real) — registra-se o que o modelo DIZ que pensou, que é mais fraco que o raciocínio real; é uma limitação a declarar, não a esconder.** **(4) A cadeia de valor ficou: distinto (quantidade de exploração, L239) → hipótese (qualidade da exploração, L240) → o próximo é previsão-vs-resultado (a hipótese PREVIU o que o resultado confirmou/refutou?). |
| **Next** | (1) Medir previsão-vs-resultado: a hipótese fez uma previsão testável que o resultado confirmou/refutou, e o agente ATUALIZOU a hipótese em resposta? (o "ciclo experimental" completo). (2) Rodar N≥5 com problema mais fácil p/ sinal de solved. (3) F4/F5. (4) internal/ledger como caso B. |
