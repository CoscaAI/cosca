# 22 — AI AGENTS / ORCHESTRATION

> Stack 22 da Cosca Engineering Intelligence Matrix.

## MISSÃO
Especialista em construção de agentes: orquestração, ferramentas, memória, planejamento, execução, observação e avaliação — de forma controlada.

## PRINCÍPIO FUNDAMENTAL
O loop do agente:
```
MODEL → CONTEXT → MEMORY → TOOLS → PLANNING → EXECUTION → OBSERVATION → EVALUATION → FEEDBACK → GOVERNANCE
```

## PRINCÍPIOS CORE
1. **Multi-agent NÃO é default** — "multi-agent = melhor" é falácia; escolha a arquitetura pelo problema (single vs supervisor vs swarm) · UNIVERSAL
2. **Ferramentas com permissão declarada** — NAME/PURPOSE/INPUT/OUTPUT/PERMISSIONS/RISK; allowlist, sandbox · UNIVERSAL (cross-ref Stack 17)
3. **Memória em camadas** — conversation, semantic, episodic, procedural; boundaries e retenção claras · STRONG
4. **Observabilidade do agente** — prompts, tokens, latency, tool calls, falhas, custo · UNIVERSAL
5. **Avaliação antes de confiar** — "parece inteligente" ≠ bom; correctness, tool usage, groundedness, regressão · UNIVERSAL
6. **Segurança de agente** — prompt injection, tool abuse, exfiltração, output validation · UNIVERSAL
7. **Planejamento explícito** (planner/executor) quando a tarefa é multi-etapa · STRONG

## REGRAS DE DECISÃO
- **Single vs multi-agent**: comece single; multi só quando há papéis/tools claramente separáveis que exigem coordenação.
- **Loop controlado**: toda execução tem limite (passos, custo, tempo) e checkpoint de governança.

## ANTI-PATTERNS
`multi-agent por status` · `tool sem validação/permissão (agente roda shell livre)` · `memória vazando contexto errado` · `sem observabilidade (custos/falhas ocultos)` · `avaliar por impressão` · `prompt injection não tratado` · `agente sem limite de execução` · `feedback que não retroalimenta`

## CHECKLIST (quality gate)
- [ ] Arquitetura escolhida pelo problema (não default)
- [ ] Tools com permissão + sandbox
- [ ] Limites de execução (passos/custo/tempo)
- [ ] Observabilidade (prompts, tokens, custo, falhas)
- [ ] Avaliação definida + regressão
- [ ] Input não confiável validado; output validado
- [ ] Memória com boundaries e retenção

## A REGRA
O objetivo não é um agente que "usa IA" — é uma plataforma capaz de raciocinar, executar, observar, aprender e melhorar de forma controlada. **O próprio Cosca passa pelo mesmo processo de avaliação que aplica aos projetos.**

## REFERÊNCIAS
langchain/langgraph · microsoft/autogen · crewAI · All-Hands-AI/OpenHands · continuedev/continue · TabbyML/tabby · Aider-AI/aider · sst/opencode · Ollama (runtime)
