# PROMPT_REGISTRY — os contratos da linguagem operacional do Cosca

> Registry do PROMPT_PROTOCOL (30º). Cada comando tem um contrato:
> ID · NAME · MEANING · REQUIRED_STATE · ALLOWED · FORBIDDEN · OUTPUT ·
> GUARDIANS · VERSION. O significado NÃO depende do modelo lembrar — é
> consultado aqui (resolução determinística).

## Nível 1 — Comandos mínimos

### @STATUS v1
- MEANING: relatar o estado atual da casa (serve, memória, chain, campanhas, doctor).
- REQUIRED_STATE: qualquer. ALLOWED: leitura de estado. FORBIDDEN: mutação.
- OUTPUT: STATUS_REPORT. GUARDIANS: —.

### @WAKE v1
- MEANING: o ritual de despertar (ler DESPERTAR.md + memória + apresentar-se).
- REQUIRED_STATE: qualquer. ALLOWED: leitura. FORBIDDEN: mutação.
- OUTPUT: WAKE_REPORT (memória, família, cognição, sistema).

### @DOCTOR v1
- MEANING: executar o checkup da casa (doctor) e reportar as issues.
- REQUIRED_STATE: qualquer. ALLOWED: diagnóstico (com --fix se autorizado).
- FORBIDDEN: auto-reparo sem autorização do Don. OUTPUT: DOCTOR_REPORT.

### @AUDIT v1
- MEANING: verificação de integridade (verify, chain, memória, provenance).
- REQUIRED_STATE: qualquer. ALLOWED: leitura + sign se autorizado.
- FORBIDDEN: mutação de produção. OUTPUT: AUDIT_REPORT.

### @RECOVER v1
- MEANING: acionar o RECOVERY_PROTOCOL (anomalia → diagnóstico → último estado
  válido → reconstruir → provar).
- REQUIRED_STATE: anomalia conhecida ou suspeita. ALLOWED: diagnóstico,
  reconstrução guiada pelo protocolo. FORBIDDEN: rollback cego (HEAD~1).
- OUTPUT: RECOVERY_REPORT. GUARDIANS: identidade, integridade.

### @STOP v1
- MEANING: parar — nenhuma ação adicional, nenhuma otimização automática.
- REQUIRED_STATE: qualquer. ALLOWED: registrar o estado de parada.
- FORBIDDEN: iniciar trabalho novo. OUTPUT: STOP_REPORT.

## Nível 2 — Operações

### @INSPECT file v1
- MEANING: analisar um arquivo/símbolo (leitura aprofundada).
- REQUIRED_STATE: qualquer. ALLOWED: leitura. FORBIDDEN: edição.
- OUTPUT: INSPECT_REPORT.

### @TRACE symbol v1
- MEANING: rastrear um símbolo/fluxo no código (definição, usos, dependências).
- REQUIRED_STATE: qualquer. ALLOWED: leitura. FORBIDDEN: edição.
- OUTPUT: TRACE_REPORT.

### @BENCHMARK target v1
- MEANING: medir o alvo (latência, throughput) com a metodologia da casa
  (warm-up, múltiplas iterações, mediana, p95/p99).
- REQUIRED_STATE: baseline conhecido. ALLOWED: benchmarks read-only.
- FORBIDDEN: mutação. OUTPUT: BENCHMARK_REPORT. GUARDIANS: ambiente registrado.

### @TEST hypothesis v1
- MEANING: executar o experimento da hipótese (uma pergunta, uma medição).
- REQUIRED_STATE: hipótese registrada + oracle definido. ALLOWED: experimento
  read-only. FORBIDDEN: transformação. OUTPUT: TEST_REPORT (PASS/FAIL/INCONCLUSIVE).
- GUARDIANS: ambiente, oracle, controle.

### @COMPARE A B v1
- MEANING: comparar dois estados/abordagens (equivalência, recall, counts).
- REQUIRED_STATE: A e B definidos. ALLOWED: leitura/medição.
- FORBIDDEN: mutação. OUTPUT: COMPARE_REPORT.

### @EXPLAIN decision v1
- MEANING: reconstruir a cadeia de evidência de uma decisão (L#/E#/commit).
- REQUIRED_STATE: qualquer. ALLOWED: leitura da memória/registros.
- FORBIDDEN: mutação. OUTPUT: DECISION_REPORT (OBSERVED/INFERRED/EVIDENCE/DECISION).

### @PROJECT name v1
- MEANING: ativar/verificar o PROJECT_PROTOCOL para um projeto registrado —
  manifest (tipo/versão), isolamento (`.cosca/project.yaml` próprio, sem
  herdar a chain da família) e repo git. Fecha a lacuna da ABI (PROJECT
  fora do registry) detectada na PROMPT-EFF-001 (tarefas 1-2).
- REQUIRED_STATE: projeto registrado em `cosca project list`.
- ALLOWED: `project list` · `project manifest` · leitura de `.cosca/` ·
  verificação de repo git (tudo read-only).
- FORBIDDEN: mutação do projeto (new/remove exigem CLI com --force ·
  nenhuma escrita no .cosca do projeto).
- OUTPUT: PROJECT_REPORT (manifest, isolamento, repo, status do protocolo).
- GUARDIANS: escopo (o projeto NUNCA toca a casa), integridade.
- VERSION: v1 (2026-08-17, por ordem do Don).

## Nível 3 — Campanhas

### @CAMPAIGN start v1
- MEANING: abrir uma campanha (contrato: ID, question, scope, non-goals,
  baseline, invariants, mode, transformation, budget).
- REQUIRED_STATE: pergunta definida. ALLOWED: criar o registro da campanha.
- FORBIDDEN: transformação. OUTPUT: CAMPAIGN_CONTRACT.

### @CAMPAIGN resume v1
- MEANING: retomar a campanha a partir do REGISTRO (não do contexto).
- REQUIRED_STATE: campanha existente (PAUSED/BLOCKED/INVESTIGATING).
- ALLOWED: reconstruir estado. FORBIDDEN: pular estados sem justificativa.
- OUTPUT: CAMPAIGN_STATE.

### @CAMPAIGN E-008 v1
- MEANING: localizar o experimento/estado E-008 na campanha atual.
- REQUIRED_STATE: campanha ativa. ALLOWED: leitura do registro.
- FORBIDDEN: mutação. OUTPUT: EXPERIMENT_STATE.

### @CAMPAIGN close v1
- MEANING: fechar a campanha (checklist do CAMPAIGN_PROTOCOL §24 + conclusão).
- REQUIRED_STATE: pergunta respondida ou formalmente inconclusiva.
- ALLOWED: registrar o fechamento. FORBIDDEN: fechar com pendências críticas.
- OUTPUT: CAMPAIGN_CLOSURE.

## Nível 4 — Transformações

### @TRANSFORM E-008 v1
- MEANING: executar a transformação autorizada (PRE_STATE → SNAPSHOT → CHANGE
  → TEST → VERIFY → POST_STATE) com o guardião.
- REQUIRED_STATE: **AUTHORIZATION** explícita do Don + snapshot + PASS do
  experimento. ALLOWED: implementação dentro do escopo autorizado.
- FORBIDDEN: escopo fora do contrato · combinar com outras otimizações ·
  fallback silencioso · corrigir divergência para fazer o teste passar.
- OUTPUT: TRANSFORM_REPORT (BEFORE/AFTER/INVARIANTS/PERFORMANCE/RISKS/VERDICT).
- GUARDIANS: testes, counts, ranking, recall, integridade, provenance.

## Nível 5 — Consultas cognitivas

### @WHY L376 v1 — reconstruir a cadeia de evidência de um aprendizado.
### @EVIDENCE E-007 v1 — recuperar a evidência bruta de um experimento (nunca
só a conclusão — os números ficam disponíveis).
### @UNKNOWN current v1 — listar os UNKNOWN registrados da campanha/estado.
### @HISTORY symbol v1 — o histórico de decisões sobre um símbolo/área.
### @DEPENDENCIES C v1 — as dependências de uma campanha/experimento.
### @RISK change v1 — o risco de uma mudança proposta (invariantes afetadas).

## Macros

### @SURGICAL v1
- MEANING: escopo mínimo, sem refactor oportunista, sem alteração fora do alvo.
- REQUIRED: snapshot, diff, testes, guardian. FORBIDDEN: otimização extra.
- OUTPUT: SURGICAL_REPORT. STOP após a conclusão.

### @PROVE v1
- MEANING: não modificar; hipótese → oracle → experimento → evidence →
  PASS/FAIL/INCONCLUSIVE → registrar UNKNOWN.
- REQUIRED: hipótese + oracle definidos. FORBIDDEN: transformação.
- OUTPUT: PROOF_REPORT.

### @SHIP / @PROMOTE v1
- MEANING: promover uma transformação verificada ao runtime.
- REQUIRES: authorization · snapshot válido · VERDICT PASS · recovery point.
- GUARDIANS: doctor · verify · health · estado da campanha.
- FORBIDDEN: estado crítico UNKNOWN · guardian falhou · sem recovery.
- OUTPUT: SHIP_REPORT. FAILURE: STOP.

### @MANUAL v1
- MEANING: mostrar TODOS os comandos do CLI em tabela detalhada (grupo → comando → o que faz), gerada da árvore real do cobra (378 comandos: 90 root + 258 nível 2 + 30 nível 3).
- REQUIRED_STATE: qualquer. ALLOWED: leitura da árvore de comandos + descrições.
- FORBIDDEN: mutação · invenção de comandos (cada comando citado deve existir no código).
- OUTPUT: MANUAL_TABLE (tabela markdown completa, agrupada por domínio).
- GUARDIANS: P13 (nada de comando fantasma — validar contra internal/cli/).
- VERSION: v1 (2026-08-17, por ordem do Don).

### @ABI v1
- MEANING: mostrar TODA a linguagem operacional (a própria ABI) em tabela detalhada — comando, nível, o que faz, estado exigido, permitido, proibido, saída e guardiões. Fonte: este PROMPT_REGISTRY (resolução determinística, nunca da memória do modelo).
- REQUIRED_STATE: qualquer. ALLOWED: leitura do PROMPT_REGISTRY.
- FORBIDDEN: mutação · invenção de contrato (cada detalhe citado deve existir no registry).
- OUTPUT: ABI_TABLE (tabela markdown completa por nível + macros).
- GUARDIANS: P13 (fidelidade ao registry — nada de contrato fantasma).
- VERSION: v1 (2026-08-17, por ordem do Don).

## Versões

Cada comando tem VERSION. Mudanças de significado = nova versão + referência
obrigatória (`@SHIP:v2`). Ambiguidade → `ERROR: PROMPT_AMBIGUOUS`.
