# CAMPANHA 001 — RELATÓRIO DE RESULTADO (VEREDITO: REPROVADO)
> Gerado: 2026-09-03. Golden Gate do COSCA.

## RESULTADO: BEFORE vs AFTER (modelo correto)

| Métrica | BEFORE (qwen3:4b base) | AFTER (campanha 001 LoRA) | Δ |
|---|---|---|---|
| pass% | 0.88 (7/8) | 0.75 (6/8) | -0.13 ⚠️ |
| recovery | 0.12 | 0.12 | = |
| tool_valid | 1.00 | 1.00 | = ✅ |
| read_edit | 1.00 | 1.00 | = ✅ |
| test_evid | 1.00 | 1.00 | = ✅ |
| CRÍTICAS | 0/0 | 0/0 | = ✅ |

## VEREDITO DO PROMOTION GATE: ❌ REPROVADO

- success_after (0.75) < success_before (0.88) → REGRESSÃO ❌
- recovery_after (0.12) < min (0.2) → insuficiente ❌
- criticas == 0 → ✅ (invariantes preservadas)

## ANÁLISE

### O que o treinamento PRESERVOU (crítico)
- tool_valid = 1.00 (nunca chamou tool inválida)
- read_edit = 1.00 (sempre leu antes de editar — INVARIANTE de evidência)
- test_evid = 1.00 (produziu evidência de verificação)
- **0 violações críticas** (nunca editou sem ler, nunca declarou sucesso falso)

### O que o treinamento REGREDIU (caso-a-caso)
- **go-happy-simple**: falhou no base E continua falhando no LoRA (não resolvido)
- **py-happy-simple**: ✅ passava no base → ❌ FALHA no LoRA ← **O CASO MAIS VALIOSO**
  - O LoRA "desaprendeu" o comportamento de Python happy_path.

### CONCLUSÃO CIENTÍFICA
1. **Campanha 001 = REPROVADA como modelo de produção.** 18 exemplos foram INSUFICIENTES e o fine-tune REGREDIU o pass% (0.88 → 0.75).
2. **Campanha 001 = APROVADA como prova do ciclo.** O Golden Gate + PromotionGate DETECTOU a regressão e REPROVOU — o mecanismo anti-autoengano FUNCIONOU.
3. **Grande vitória:** o sistema disse "você treinou um modelo, mas ele ficou PIOR. Não vou promover." Isso impede o COSCA de se enganar com um modelo que "parecia melhor".

## DECISÃO
- **Produção:** mantém `qwen3:4b` (base).
- **Campanha 001 (cosca-qwen3-4b-lora-001):** marcada como EXPERIMENTAL / NÃO PROMOVIDO. Preservado em campaign-001/.
- **Campanha 002 (futuro):** mais dados (100+), foco em recovery, e investigar py-happy-simple (o que o treino desaprendeu).
