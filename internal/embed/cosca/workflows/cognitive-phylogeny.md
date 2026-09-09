# Workflow documental: Filogênese Cognitiva aplicada a agentes

> **Status:** proposta para comparação futura — não implementado
> **Escopo da ordem:** somente documentação. Este arquivo não cria agentes, não altera routing, não promove memória e não executa mutações.

## Objetivo

Registrar um modelo comparável para observar como capacidades cognitivas de agentes poderiam divergir, especializar-se e ser avaliadas ao longo do tempo, sem alterar o ecossistema atual. O workflow produz propostas e evidências auditáveis; não produz efeitos operacionais.

## Escopo

Aplica-se à análise documental de agentes General e Specialist, de suas capacidades, limites, evidências e possíveis relações de linhagem. Inclui critérios de comparação, segurança, regressão e promoção futura. Não define implementação, schema, API, mecanismo de mutação, política de routing ou autorização automática.

## Definições

- **Cognitive Genome:** conjunto versionado de capacidades, instruções, limites, fontes, métricas e pressupostos que descreve o comportamento cognitivo observável de um agente. É um modelo de análise, não um artefato executável.
- **Lineage:** relação de ancestralidade documental entre um perfil observado e uma proposta derivada, com origem, diferenças, evidências e decisões registradas.
- **Mutation:** alteração proposta em um elemento do Cognitive Genome (capacidade, limite, heurística, contexto ou especialização). Neste workflow, mutation é apenas hipótese em quarentena; nunca é executada.
- **General:** agente de propósito amplo, avaliado pela cobertura de tarefas e pela preservação de comportamento transversal.
- **Specialist:** perfil com foco delimitado, avaliado pela profundidade no domínio sem assumir autoridade fora dele.

## Estado atual versus proposta

| Aspecto | Estado atual verificável | Proposta documental |
|---|---|---|
| Agentes | Não há, neste documento, um modelo de filogênese cognitiva implementado | Registrar observações, candidatos, linhagens e decisões para comparação futura |
| Genome | Não presumido nem inferido por este workflow | Descrever um genome como snapshot versionado e referenciado por evidências |
| Evolução | Nenhuma mutação é executada por esta proposta | Simular diferenças em dry-run, sempre em quarentena |
| Routing | Inalterado | Comparar General/Specialist sem modificar seleção ou roteamento |
| Memória | Não promovida | Evidência pode ser citada; promoção depende de processo externo aprovado |
| Autoridade | A ordem do Don prevalece | Qualquer avanço segue proposta → policy → risco → gate → aprovação, conforme P9 |

## Regra de operação

**Tudo é dry-run e permanece em quarentena até aprovação explícita.** Nenhuma fase pode criar ou editar agente, alterar configuração, routing ou memória, executar mutation, ou tratar uma hipótese como capacidade aprovada. Uma aprovação documental não equivale à execução técnica.

## Fases

```text
observation → candidate → evidence → benchmark → cognitive regression
            → safety review → approval → promotion / dormant / reject
```

1. **Observation:** registrar comportamento observável, contexto, versão, limitações e resultado; não interpretar ausência de evidência como capacidade.
2. **Candidate:** formular uma hipótese de genome, lineage ou mutation, com escopo, motivação e impacto esperado. O candidato recebe estado `quarantine`.
3. **Evidence:** anexar referências rastreáveis (código executado, testes, logs, decisões e documentação). Conforme P2, código executado prevalece sobre documentação, memória ou opinião.
4. **Benchmark:** comparar o candidato com baseline fixo, incluindo cobertura, qualidade, custo, latência, consistência e falhas relevantes. Resultados negativos são evidência.
5. **Cognitive regression:** verificar se a proposta reduz capacidades existentes, aumenta contradições, esquece limites ou degrada o comportamento General e Specialist.
6. **Safety review:** revisar abuso, escalada de privilégio, vazamento, prompt injection, perda de auditabilidade e possibilidade de reversão. Segurança pode bloquear o fluxo.
7. **Approval:** uma autoridade externa ao modelo decide, registra evidências, riscos, escopo e veto. A conclusão de um agente não é autorização (P9); a autoridade do Don é superior (P10).
8. **Promotion / dormant / reject:** registrar uma decisão final:
   - `promotion`: somente após todos os gates e aprovação; descreve o que *poderia* ser promovido, sem executar promoção neste workflow;
   - `dormant`: candidato preservado para comparação posterior, sem efeito;
   - `reject`: hipótese recusada, com motivo e evidência.

## Gates obrigatórios

- **G0 — Ordem e escopo:** confirmar que a tarefa é documental e que o candidato está em dry-run/quarentena.
- **G1 — Identidade e lineage:** origem, versão, diferenças e proprietário estão identificados.
- **G2 — Evidência:** cada afirmação relevante tem fonte rastreável e incerteza declarada; aplicar P2.
- **G3 — Benchmark:** baseline, métricas, casos positivos e negativos são reproduzíveis ou explicitamente marcados como indisponíveis.
- **G4 — Regressão cognitiva:** não há degradação não explicada em capacidades, limites ou segurança.
- **G5 — Safety:** riscos, abuso, autoridade, auditoria e rollback foram revisados.
- **G6 — Aprovação:** decisão externa, registrada e compatível com P9/P10; dúvida operacional consulta primeiro o Manual, sem substituir evidência (P11).
- **G7 — Teste de Continuidade Cognitiva:** a mesma tarefa é comparada entre Agent v1, v2, v3 e v4, com trilha de decisão, contexto, ferramentas, erros, correções e métricas; o resultado permanece documental e não autoriza qualquer alteração.

Falha em qualquer gate mantém o candidato em `quarantine`, envia-o para `dormant` ou `reject`; nunca permite promoção implícita.

## G7 — Teste de Continuidade Cognitiva

O teste avalia se versões sucessivas preservam, adaptam ou especializam sua estratégia diante da **mesma tarefa**, usando o mesmo enunciado, critérios de avaliação, limites de autoridade e condições de comparação sempre que isso for possível. O teste não presume que uma versão posterior seja melhor: diferenças devem ser explicadas por evidência, e ausência de evidência deve ser registrada como `unknown`.

### Registro comparável por versão

Para cada execução em Agent v1, v2, v3 e v4, registrar separadamente:

| Campo | Registro mínimo |
|---|---|
| **Decision** | decisão ou plano adotado e seu resultado |
| **Confidence** | confiança declarada, faixa ou justificativa de calibração |
| **Knowledge used** | conhecimento, memória e referências efetivamente usados, com versão/origem |
| **Agents called** | agentes chamados, sequência e motivo; `none` quando aplicável |
| **Tools used** | ferramentas usadas, entradas relevantes e resultado; `none` quando aplicável |
| **Context selected** | contexto selecionado, excluído e critério de seleção |
| **Assumptions** | pressupostos explícitos e seu impacto |
| **Unknowns** | lacunas, incertezas e perguntas não resolvidas |
| **Error detection** | erros detectados, por quem/quando e evidência |
| **Correction** | correção aplicada, descartada ou impossível, com efeito observado |
| **Final answer** | resposta final exatamente como produzida |

O registro deve preservar o enunciado, timestamp, versão do agente, baseline, evidências, limites e resultado. Não se deve preencher campos por inferência retrospectiva: `not recorded`, `not applicable` e `unknown` são valores válidos e auditáveis.

### Comparação e classificação

1. Comparar as mudanças **reais** de estratégia entre v1→v2, v2→v3 e v3→v4: decomposição da tarefa, seleção de contexto e conhecimento, uso de agentes e ferramentas, tratamento de incerteza, detecção de erro, correção e forma da resposta final.
2. Separar mudança observada de mudança alegada; citar logs, respostas e demais evidências disponíveis.
3. Classificar cada mudança, com justificativa e impacto, como:
   - **melhoria:** qualidade, segurança, calibração, recuperação ou eficiência aumentou sem regressão relevante;
   - **regressão:** capacidade, limite, segurança, consistência ou recuperação piorou;
   - **especialização:** ganho delimitado a um domínio, acompanhado de fronteiras e teste fora do domínio;
   - **variação:** diferença observável sem evidência suficiente para afirmar ganho ou perda.
4. Registrar também o que permaneceu estável e conflitos entre versões. Uma versão não é promovida nem preferida automaticamente pela classificação.

### Métricas do G7

Pontuar cada métrica em escala definida pelo benchmark (por exemplo, 0–1), sempre com evidência, baseline e limitações declaradas:

| Métrica | Pergunta de avaliação |
|---|---|
| **Stability** | decisões, limites e comportamento essencial permanecem estáveis? |
| **Adaptability** | a estratégia se ajusta a mudanças relevantes sem perder o objetivo? |
| **Calibration** | Confidence acompanha a taxa observada de acerto e incerteza? |
| **Recovery** | o agente detecta, corrige e se recupera de erros de forma segura? |
| **Consistency** | resultados e justificativas permanecem coerentes sob condições equivalentes? |
| **Specialization** | ganhos de domínio são reais, delimitados e não extrapolados? |
| **Regression** | houve perda mensurável de capacidade, limite, segurança ou rastreabilidade? |

O relatório deve incluir pontuação por versão, delta entre versões, evidências, casos positivos e negativos e a classificação final. Métricas indisponíveis não podem ser estimadas por opinião.

### Checklist de preparação da próxima sessão

Antes da próxima comparação, auditar documentalmente:

- [ ] **Memória:** origem, versão, escopo, data, curadoria, conflitos e itens promovidos/não promovidos estão identificados?
- [ ] **Embeddings:** modelo, versão, espaço, cobertura, timestamp, filtros, proveniência e limitações de recuperação estão registrados?
- [ ] **Agents:** v1/v2/v3/v4, prompts/instruções, capabilities, limites, agentes chamados e baseline estão identificados?
- [ ] **Skills:** skills disponíveis, versão, instruções aplicáveis, dependências e skill selecionada em cada execução estão registradas?
- [ ] **Tarefa:** o mesmo enunciado, critérios, contexto permitido e condições de comparação foram congelados?
- [ ] **Auditoria:** Decision, Confidence, Knowledge used, Agents called, Tools used, Context selected, Assumptions, Unknowns, Error detection, Correction e Final answer têm campos e evidências para cada versão?
- [ ] **Métricas:** Stability, Adaptability, Calibration, Recovery, Consistency, Specialization e Regression têm definição, baseline e método de comparação?
- [ ] **Segurança e autoridade:** escopo documental, quarentena, veto do Don, ausência de execução e procedimento de interrupção estão explícitos?

Este checklist prepara uma auditoria futura; não cria embeddings, não altera memória, não chama agentes, não instala ou modifica skills e não executa o teste.

## Evidências e trilha de auditoria

Cada registro deve conter: identificador do candidato, timestamp, agente responsável, genome/lineage de origem, hipótese, baseline, casos de benchmark, resultados, riscos, gates, decisão, autoridade e referências. P5 exige registrar falhas e aprendizados em vez de ocultá-los. Evidência ausente deve ser marcada como ausência, não preenchida por suposição.

## Riscos e mitigação

| Risco | Mitigação documental |
|---|---|
| Confundir hipótese com capacidade real | Estado explícito `candidate/quarantine`; P2 e evidências executadas |
| Especialização estreita causar regressão | Benchmark General + Specialist e cognitive regression |
| Mutation ampliar autoridade ou privilégios | Safety review, permission gate externo e veto do Don |
| Memória ou documentação desatualizada | Verificar contra código, testes e fontes atuais; declarar conflito |
| Linhagem sem rastreabilidade | Genome snapshot, lineage id e trilha imutável |
| Falha ou abuso após eventual promoção | Rollback pré-definido e promoção reversível |

## Rollback

Antes de qualquer promoção futura, deve existir snapshot do estado anterior, referência da decisão e procedimento de retorno. Um sinal de regressão, risco de segurança, evidência contraditória ou violação de escopo interrompe a promoção e restaura o snapshot aprovado. Neste workflow documental, rollback significa apenas marcar a proposta como revertida/dormant e preservar a trilha; nenhum estado operacional é alterado.

## Critérios de especialização

Uma proposta Specialist só é elegível para avaliação quando houver: (1) domínio e fronteiras explícitos; (2) tarefa recorrente e benefício mensurável contra o General; (3) benchmark com casos fora do domínio para testar contenção; (4) limites de autoridade e escalada; (5) ausência de regressão cognitiva; (6) evidência suficiente e safety review aprovado. Sem isso, permanece dormant ou é rejeitada. Especialização não significa novo agente nem autorização adicional.

## Herança permitida e proibida

**Permitida para proposta:** capacidades gerais demonstradas, formato de evidência, testes, limites de segurança e conhecimento explicitamente versionado, desde que a origem seja citada e o herdeiro passe pelos mesmos gates.

**Proibida:** herdar autoridade, permissões, segredos, identidade do Don, decisões não evidenciadas, memória não curada, routing, bypass de safety, capacidade de autoaprovação ou qualquer mutation já não aprovada. Nenhum lineage pode transformar uma hipótese em fato por mera herança.

## Comparação: General versus Specialist

| Dimensão | General | Specialist |
|---|---|---|
| Cobertura | Ampla, transversal | Delimitada, profunda |
| Evidência principal | Robustez em tarefas variadas | Ganho no domínio e contenção fora dele |
| Risco típico | Superficialidade ou inconsistência | Overfitting e extrapolação indevida |
| Regressão a observar | Perda de cobertura geral | Perda de limites e isolamento do domínio |
| Decisão | Mantém baseline de família | Só é preferido quando o ganho superar o custo e o risco |

A comparação é experimental e documental. Não constitui política de routing e não autoriza substituir um General por um Specialist.

## Referências de governança

- **P2 — Código executado é a verdade:** propostas devem ser confrontadas com código e testes; documentação e memória não inventam implementação.
- **P5 — A família aprende com erros:** falhas, rejeições e regressões permanecem registradas.
- **P9 — IA propõe, o sistema decide:** a cadeia de proposta, policy, risco, permission gate, aprovação e execução é externa ao modelo; aqui a execução é proibida.
- **P10 — Autoridade do Don:** veto, honestidade, rastreabilidade e autoridade final não são herdados por lineage.
- **P11 — Manual primeiro em caso de dúvida:** consultar o Manual de Autoajuda e validar sua orientação contra evidência, código, testes e a ordem do Don.

Referência canônica: [Constituição do Cosca](../CONSTITUTION.md).

## Não implementação

Este documento deliberadamente **não cria agentes, não altera routing, não promove memória e não executa mutações**. Qualquer implementação futura exigirá uma ordem e uma revisão próprias, com evidências, gates, aprovação externa e rollback.
