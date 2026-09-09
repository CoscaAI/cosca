# MATRIZ DE REDUNDÂNCIA EMPRESARIAL — Arquitetura Completa de Failover

> **Version**: 1.0.0 | **Status**: active | **Owner**: Cosca Kernel | **Last Updated**: 2026-07-12

## PROPÓSITO
Nenhum componente crítico no ecossistema Cosca deve depender de um único ponto de falha. Esta matriz define a estratégia completa de redundância, failover e recuperação para cada componente crítico.

---

## CAMADAS DE REDUNDÂNCIA

```
Camada 1: Redundância de Agentes      (Primário → Secundário → Fallback)
Camada 2: Redundância de Providers    (OpenAI → Anthropic → LLM Local)
Camada 3: Redundância de Armazenamento (Caminho primário → Caminho fallback → Em memória)
Camada 4: Redundância de Engines      (Engine Primário → Modo Degradado → Manual)
Camada 5: Redundância de Liderança    (Chief → Chief Backup → Conselho → Executivo)
Camada 6: Redundância de Runtime      (Runtime Primário → Runtime Secundário → Fallback CLI)
```

---

## CAMADA 1: REDUNDÂNCIA DE AGENTES

| Tipo de Agente | Primário | Secundário | Fallback | Ativação |
|-----------|---------|-----------|----------|------------|
| Backend Chief | Backend Chief | Architecture Chief (revisão de backend) | CTO | Após 3 falhas |
| Frontend Chief | Frontend Chief | UI/UX Chief (revisão de componentes) | CTO | Após 3 falhas |
| Database Chief | Database Chief | Backend Chief (revisão de schema) | Architecture Chief | Após 3 falhas |
| Security Chief | Security Chief | CTO (revisão de segurança) | Architecture Chief | IMEDIATO em crítico |
| QA Chief | QA Chief | Testing Chief (promovido) | CTO | Após 2 falhas |
| Review Chief | Review Chief | Architecture Chief (arquitetura) + Security Chief (segurança) | CTO | Após 3 falhas |
| Testing Chief | Testing Chief | QA Chief (supervisão) | Backend/Frontend Chief | Após 3 falhas |
| DevOps Chief | DevOps Chief | Infrastructure Chief | CTO | Após 3 falhas |
| Infrastructure Chief | Infrastructure Chief | DevOps Chief | CTO | Após 3 falhas |
| Qualquer Especialista | Especialista da Especialidade | Chief (execução direta) | Especialista Secundário | Após 2 falhas |

---

## CAMADA 2: REDUNDÂNCIA DE PROVIDERS

| Tipo de Tarefa | Provider Primário | Provider Secundário | Provider Fallback | Circuit Breaker |
|-----------|-----------------|-------------------|-------------------|-----------------|
| Estratégico (CEO) | GPT-4o | Claude 3.5 Sonnet | — | 5 falhas / 60s |
| Planejamento (CTO) | GPT-4o | Claude 3.5 Sonnet | — | 5 falhas / 60s |
| Arquitetura | Claude 3.5 Sonnet | GPT-4o | — | 5 falhas / 60s |
| Geração de Código | Claude 3.5 Sonnet | GPT-4o | Llama 3 70B | 10 falhas / 120s |
| Revisão de Código | GPT-4o | Claude 3.5 Sonnet | — | 5 falhas / 60s |
| Auditoria de Segurança | GPT-4o | Claude 3.5 Sonnet | — | 3 falhas / 60s |
| Testes | Claude 3.5 Sonnet | GPT-4o | Llama 3 70B | 10 falhas / 120s |
| Documentação | GPT-4o | Claude 3.5 Sonnet | Llama 3 70B | 10 falhas / 120s |
| Tarefas Simples | GPT-4o-mini | Claude Haiku | Llama 3 70B | 20 falhas / 120s |
| Criativo | GPT-4o | Claude 3.5 Sonnet | — | 5 falhas / 60s |

---

## CAMADA 3: REDUNDÂNCIA DE ARMAZENAMENTO

| Tipo de Armazenamento | Caminho Primário | Caminho Fallback | Recuperação |
|-------------|-------------|---------------|----------|
| Armazenamentos de Memória | .cosca/memory/ | ${MEMORY_GLOBAL}/ | Restaurar do backup mais recente |
| Cosca Core (skills) | ${COSCA_HOME}/ | ${COSCA_HOME}_backup/ | Git restore do remoto |
| Configuração do Projeto | .cosca/config.yml | .cosca/config.yml.bak | Restauração automática do backup |
| Estado da Sessão | .cosca/memory/short/ | Cache em memória | Perdido em falha (aceitável para memória de curto prazo) |
| Trilha de Auditoria | .cosca/audit/ | ${AUDIT_HOME}_replica/ | Replay da réplica |
| Base de Conhecimento | knowledge/ | ${MEMORY_GLOBAL}/knowledge/ | Restaurar do espelho global |

---

## CAMADA 4: REDUNDÂNCIA DE ENGINES

| Engine | Modo de Falha | Comportamento Degradado | Sobrescrita Manual |
|--------|-------------|-------------------|-----------------|
| Discovery Engine | Não consegue detectar automaticamente | Especificação manual da stack | Usuário fornece informações da stack |
| Context Engine | Não consegue construir contexto | Carregar do último contexto conhecido | Kernel carrega contexto em cache |
| Memory Engine | Não consegue persistir | Apenas em memória (perda de sessão ao final) | Exportação manual de memória |
| Workflow Engine | Não consegue orquestrar | Execução linear apenas (sem paralelo) | Execução manual de passos |
| Planning Engine | Não consegue gerar plano | Criação manual de plano pelo CTO | CTO escreve plano manualmente |
| Execution Engine | Não consegue despachar | Execução sequencial pelo Kernel | Execução direta pelo Kernel |
| Review Engine | Não consegue revisar | Revisão manual pelo Review Chief | Review Chief faz revisão manual |
| Quality Engine | Não consegue aplicar gates | Gates registrados apenas como avisos | QA Chief aplica manualmente |
| Resource Resolver | Não consegue resolver caminhos | Caminhos fallback hardcoded | Especificação manual de caminhos |
| Secrets Engine | Não consegue recuperar secrets | Agente bloqueado (seguro por padrão) | Security Chief injeta manualmente |
| Policy Engine | Não consegue avaliar | Todas as policies aplicadas como erros (seguro) | Sobrescrita manual de policy |
| Benchmark Engine | Não consegue executar | Usar últimos scores conhecidos | Estimativa manual de benchmark |

---

## CAMADA 5: REDUNDÂNCIA DE LIDERANÇA

| Função | Primário | Backup | Escalação | Ativação |
|------|---------|--------|------------|------------|
| Kernel | Kernel | Bootstrap Engine | Notificação ao Usuário | Falha na Fase 0 do Bootstrap |
| CEO | CEO | CTO | Conselho Executivo | Após 3 ciclos sem resposta |
| CTO | CTO | Architecture Chief | CEO | Após 3 ciclos sem resposta |
| Product Chief | Product Chief | CTO | CEO | Após 3 ciclos sem resposta |
| Architecture Chief | Architecture Chief | CTO | Architecture Council | Após 3 ciclos sem resposta |
| Qualquer Chief | Chief | CTO (interino) | Conselho Respectivo | Após 3 ciclos sem resposta |

---

## CAMADA 6: REDUNDÂNCIA DE RUNTIME

| Runtime | Primário | Secundário | Fallback | Ativação |
|---------|---------|-----------|----------|------------|
| OpenCode | OpenCode Agent | Modo CLI | API Direta | Falha no runtime |
| Claude Code | Claude Code | OpenCode | Modo CLI | Falha no runtime |
| Custom Runtime | Custom Runtime | Fallback CLI | Kernel Direto | Falha no runtime |

---

## PADRÕES DE CIRCUIT BREAKER

```
Máquina de Estados:
  CLOSED → (falhas > limiar na janela) → OPEN
  OPEN → (tempo esgotado) → HALF_OPEN
  HALF_OPEN → (sucesso) → CLOSED
  HALF_OPEN → (falha) → OPEN (resetar timer)
```

| Recurso | Limiar de Falhas | Janela | Timeout Aberto | Limite Half-Open |
|----------|------------------|--------|---------------|------------------|
| Provider API | 5 falhas | 60s | 30s | 1 requisição de probe |
| Spawn de agente | 10 falhas | 120s | 60s | 1 agente |
| Escrita em memória | 10 falhas | 60s | 30s | 1 escrita |
| Chamada de engine | 5 falhas | 60s | 30s | 1 chamada |
| Execução de tool | 3 falhas | 30s | 15s | 1 execução |

---

## MATRIZ DE HEALTH CHECKS

| Componente | Tipo de Verificação | Intervalo | Timeout | Limiar de Falha |
|-----------|-------------------|-----------|---------|-----------------|
| Kernel | Heartbeat | 30s | 5s | 3 consecutivos |
| CEO | Responsividade | 60s | 15s | 2 consecutivos |
| CTO | Responsividade | 60s | 15s | 2 consecutivos |
| Chief | Responsividade | 120s | 30s | 3 consecutivos |
| Engine | Teste de operação | 300s | 30s | 2 consecutivos |
| Provider | Ping de API | 60s | 10s | 5 em 5 min |
| Armazenamento de Memória | Teste de leitura/escrita | 300s | 10s | 2 consecutivos |
| Runtime | Endpoint de health | 60s | 5s | 3 consecutivos |

---

## OBJETIVOS DE TEMPO DE RECUPERAÇÃO (RTO)

| Cenário de Falha | RTO | RPO | Procedimento |
|-----------------|-----|-----|-----------|
| Falha de agente (com fallback) | < 30s | 0 | Ativação automática de fallback |
| Falha de agente (escalação) | < 2 min | < 1 min | Chief assume; snapshot de contexto |
| Falha de provider (com failover) | < 10s | 0 | Failover automático de provider |
| Corrupção de armazenamento de memória | < 5 min | < 1 min | Restaurar do backup |
| Falha de engine (modo degradado) | < 1 min | 0 | Ativação do modo degradado |
| Interrupção de workflow | < 2 min | Último checkpoint | Retomar do último passo concluído |
| Falha de runtime | < 30s | Último snapshot de sessão | Reinício do runtime + restauração de sessão |
| Falha total do sistema | < 15 min | < 5 min | Restauração do snapshot de DR |
| Vazamento de dados | < 5 min | 0 | Rotação automática de secrets + lockdown |

---

## RELACIONADOS
- [KERNEL.md](KERNEL.md) — Tratamento de erros e regras de redundância
- [PROVIDER_INTERFACE.md](PROVIDER_INTERFACE.md) — Configuração de failover de providers
- [COUNCILS.md](../councils/COUNCILS.md) — Escalação de liderança para Conselhos
- [Recovery Engine](../engines/knowledge/SKILL.md) — Procedimentos de recuperação automatizados
- [Secrets Engine](../engines/knowledge/SKILL.md) — Rotação de credenciais em caso de breach
- [QUALITY_GATES.md](QUALITY_GATES.md) — Gate 4 de health checks pós-release

## HISTÓRICO

| Versão | Data | Autor | Alterações |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-12 | Cosca Kernel | Matriz completa de redundância empresarial — 6 camadas, circuit breakers, health checks, RTO/RPO |
