# AWS Agent Toolkit Patterns — Segurança na Borda, Least-Privilege, Injeção de Credencial

> **Version**: 1.0.0 | **Confidence**: 0.88 | **Category**: Security Patterns | **Created**: 2026-08-22 | **Source**: https://github.com/aws/agent-toolkit-for-aws

> **Mined by**: cosca-kernel (ordem do Don). Extraído de `skills/` (agents-pay, agents-connect, agents-harden, aws-iam, aws-auth, agents-build) e `rules/`. O playbook de segurança de agentes em cloud da AWS — o material mais direto para o Security/Governance Chief do Cosca.

## Purpose

O AWS Agent Toolkit resolve o problema de **deixar agentes de IA agirem em cloud com segurança**: controles autorizados em código (nunca em instruções de modelo), segredos na borda, least-privilege determinístico e separação de blast radius. É o antídoto para os dois medos clássicos: prompt-injection e escalada de privilégio.

---

## 1. Controles autorizados em código, nunca em instruções de modelo
- **O que resolve**: o problema central de segurança de agentes — pedir ao modelo para "não gastar mais que $5" ou "não chamar tool X" é apenas um pedido; um modelo prompt-injected pode recusá-lo. Limites ficam sem efeito se a decisão passa pelo LLM.
- **Como funciona**: a decisão de autorização vive em código determinístico, num arquivo de política, antes de qualquer assinatura/checagem. Em `agents-pay`, `ProcessPayment` só é alcançado por UM caminho que não pode ser entrado sem `x402_policy.load_config()` + todos os checks passando. `forbid` (Cedar) sempre vence `permit` — default é DENY assim que um policy engine está em ENFORCE. O caminho de admin (humano no TTY, tipando `approve`, fora do runtime) e o de runtime nunca se tocam.
- **Onde**: `plugins/aws-agents/skills/agents-pay/SKILL.md`, `.../x402_policy.py`, `agents-connect/references/policy.md`.
- **Aplicação no Cosca**: mapeia direto para cosca-security/compliance/governance. Toda ação sensível (gastar com provedor, escrever credencial, mutar recurso) deve ter o gate em código/validação, não na prompt do agente. Replicar o padrão "caminho admin vs caminho runtime" e o `LOG_ONLY → ENFORCE` para testar políticas antes de aplicar. Impedir que regra de limite seja "convencível" por injection.

## 2. Injeção de credencial na borda / padrão Gateway — o segredo nunca chega ao código do agente
- **O que resolve**: a fuga clássica de segredo: `client = openai.OpenAI(api_key=...)` dentro do entrypoint — uma prompt traceback/log/trace vazada exfiltra a chave.
- **Como funciona**: um Gateway externo injeta auth na requisição de saída; o agente vê apenas `session.call_tool(...)`. O catálogo vira uma lista de tools enumeráveis, e a config declara o `--outbound-auth` (OAuth 2LO/3LO, api-key, IAM SigV4). Credenciais em código são resolvidas por decorators `@requires_api_key`/`@requires_access_token` (cache+refresh no Secrets Manager), keyword-args injetados, nunca lidos de `os.getenv`. `.env.local` (gitignored) é só para dev local e não é uploadado no deploy. Senhas: usar prompt interativo, nunca flag na linha (histórico de shell).
- **Onde**: `plugins/aws-agents/skills/agents-connect/SKILL.md` (Paths A–D, Gateway vs direto), `agents-harden/SKILL.md`.
- **Aplicação no Cosca**: crítico para cosca-integrations/provider/credentials. O Cosca deve ter uma "credential surface" central: o agente só chama um resolver de credencial (por nome), nunca o valor bruto. Proibir segredo em variável/parâmetro de ferramenta visível, em log ou em prompt do agente — este é o critério de QA dos skills.

## 3. Geração determinística de least-privilege + confused deputy
- **O que resolve**: políticas IAM construídas por LLM são alucinógenas e não auditáveis; papéis auto-criados vêm largos demais.
- **Como funciona**: quando há código-fonte, usar análise estática determinística (`uvx iam-policy-autopilot generate-policies`) — reprodutível, sem interpretação do LLM; o agente só monta o comando com flags corretos, nunca redige a política. Sem código: fallback via Service Authorization Reference. Escopo fino: modelo(s) específico(s), repo ECR específico, trust policy com `aws:SourceAccount`+`aws:SourceArn/ArnLike` (anti confused-deputy), `iam:PassRole` restrito a ARNs/caminho específicos (senão = escalada de privilégio).
- **Onde**: `plugins/aws-core/skills/aws-iam/references/*`, `agents-harden/SKILL.md`.
- **Aplicação no Cosca**: cosca-security deve gerar least-privilege por análise estática em vez de LLM. Ligado ao cosca-evolution/technical-debt. O Cosca (que tem muitos agentes com papéis) precisa do padrão de trust-policy com `SourceAccount/SourceArn` para evitar confused deputy ao assumir papéis entre agentes.

## 4. Separação do raio de blast de ações privilegiadas + alerta auditável
- **O que resolve**: poderes de leitura e de execução foram mesclados ("você pode chamar o agente" → você pode rodar comando arbitrário no microVM dele). Isso dá code execution com o papel completo do runtime.
- **Como funciona**: `InvokeAgentRuntimeCommand` (shell no microVM vivo) é uma action IAM separada e distinta de `InvokeAgentRuntime`. Nunca conceder ao mesmo principal; política separada; CloudTrail + EventBridge para alertar em cada chamada; validação anti-injection de metacharacters (`&&`, `;`, `$(...)`, backticks, `|`) se o comando for montado de input do usuário.
- **Onde**: `agents-harden/SKILL.md` ("Shell Access"), `agents-build/references/integrate.md`.
- **Aplicação no Cosca**: cosca-security + cosca-monitoring. Identificar ações de alto raio (deletar memória, escrever em storage, invocar subagente) e separá-las em permissões próprias, com trilha de auditoria e sanitização de input em qualquer caminho que execute via shell/subshell.

## 5. Modelo dual de inbound auth (SigV4 vs JWT) e a inversão "quem chama quem"
- **O que resolve**: o erro mais comum — confundir app→agente (direto) com agente→ferramenta (via Gateway/edge), o que modifica todo o desenho de segurança.
- **Como funciona**: duas direções nunca se invertem: `app → Runtime` (invocação direta, assinada com IAM SigV4 ou Bearer JWT) e `agente → Gateway → ferramenta` (agente é o cliente do catálogo). Inbound auth: escala default = `AWS_IAM`; web/mobile = `CUSTOM_JWT`. Gotchas de JWT: `allowedClients` (claim `client_id`) vs `allowedAudience` (claim `aud`); requisito de prefixo issuer↔discovery URL (RFC 8414); JWKS alcançável. Nunca `authorizer-type NONE` em produção. Cross-account via AssumeRole + `ExternalId` (anti confused-deputy) e cache de credenciais temporárias.
- **Onde**: `agents-harden/SKILL.md`, `agents-build/references/integrate.md`, `aws-core/skills/aws-auth/SKILL.md`.
- **Aplicação no Cosca**: cosca-integrations + cosca-frontend. Definir explicitamente a direção de cada chamada de agente antes de codificar. Padronizar auth dual para endpoints (service-to-service via auth assinado; web/mobile via JWT) e criar um helper único de validação JWT para evitar hand-rolling.

## 6. Ciclo de vida de sessão/quota como controle de produção (custo e runtime)
- **O que resolve**: sessões idle contam contra a quota e são a causa nº1 de `maxVms`/`ServiceQuotaExceededException` e de custo fora de controle. Novas sessões a cada request = nova VM = cold start.
- **Como funciona**: reaproveitar o mesmo `session_id` por conversa/batch; chamar `StopRuntimeSession` quando o trabalho termina; ajustar `idleRuntimeSessionTimeout` e `maxLifetime` por shape de workload (interativo 600–900s, request-reply 60–120s, batch ~120s, long-running com `add_async_task/complete_async_task` para sinalizar "ocupado"). Não passar `runtimeSessionId` e `mcpSessionId` juntos. Diagnóstico na ordem: stop→reuso→timeout; pedir aumento de quota por último.
- **Onde**: `agents-harden/SKILL.md` (Session lifecycle, Long-running background tasks), `agents-build/references/integrate.md`.
- **Aplicação no Cosca**: cosca-runtime + cosca-performance/governance. O Cosca (muitos agentes por sessão) deve ter política de reuso de sessão/contexto, liberação explícita ao terminar, e limites máximos por agente com medição. Reaproveitar a lógica de "busy signal" para tarefas de fundo.

## 7. Guardrails primeiro + progressive disclosure + observabilidade auto-instrumentada
- **O que resolve**: agentes operam em cloud com superfície ampla; conhecimento e segurança precisam ser entregues de forma escalonada e NÃO por livre arbítrio, além de observabilidade por padrão.
- **Como funciona**: três camadas. (1) **Skills com progressive disclosure**: `SKILL.md` enxuto + `references/` carregados sob demanda — mantém contexto enxuto e expertise profunda. (2) **Rules/guardrails obrigatórios**: `aws-agent-rules.md` impõe "MUST load aws-secrets-manager primeiro para qualquer segredo; MUST NOT chamar get-secret-value; usar `{{resolve:secretsmanager:...}}`" — resolução em runtime sem o segredo entrar no contexto. (3) **Observabilidade por padrão**: X-Ray + CloudWatch auto-instrumentados (wrapper opentelemetry no CMD do Dockerfile), IAM para logs, usar `logging` e não `print`, dashboard, retenção, baseline de eval + CI/CD quality gate.
- **Onde**: `rules/aws-agent-rules.md`, `skills/**/SKILL.md`, `agents-optimize/references/{observability,evals}.md`.
- **Aplicação no Cosca**: a fundação do próprio ecossistema. Mapear cosca-context/memory para o modelo de memória com estratégias (SEMANTIC/USER_PREFERENCE/EPISODIC/SUMMARIZATION) e namespaces por `actor_id`. cosca-security para guardrails de "MUST primeiro" + resolução de segredo sem entrar em contexto. cosca-observability/monitoring para instrumentação automática, dashboard de latência e quality-gate contínuo.

---

## Synthesis — o que o Cosca deveria copiar

| # | Padrão | Ganho |
|---|--------|-------|
| 1 | Gate em código, não em instrução de modelo | limite imune a prompt-injection |
| 2 | Credencial na borda via Gateway | segredo nunca entra no contexto/código do agente |
| 3 | Least-privilege determinístico + SourceAccount/SourceArn | política auditável, anti confused-deputy |
| 4 | Separar blast radius de ações + alerta | sem code execution com papel completo |
| 5 | Auth dual (SigV4 vs JWT) | direção "quem chama quem" explícita |
| 6 | Ciclo de vida de sessão/quota | custo controlado, sem cold start por request |

## Related Patterns

- [`deepseek-harness-patterns.md`](deepseek-harness-patterns.md) — scrub de env + sandbox (B1-B6)
- [`vercel-ai-sdk-patterns.md`](vercel-ai-sdk-patterns.md) — gateway de roteamento de provider
- [`hermes-agent-patterns.md`](hermes-agent-patterns.md) — progressive disclosure
