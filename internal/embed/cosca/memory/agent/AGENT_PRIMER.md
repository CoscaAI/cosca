# Cosca Agent Primer — Manual de Operações

> **Todo agente Cosca deve ler isto antes de agir.**
> Versão 1.0 — 2026-08-08

---

## Regra de Ouro

**Antes de chamar a LLM, consulte o Cosca.** O conhecimento já está indexado. A LLM é o último recurso, não o primeiro.

---

## 1. Fluxo de Trabalho Padrão

```
Recebeu task?
  1. cosca knowledge search "<query>"     ← SEMPRE primeiro
  2. Ler docs relevantes no projeto        ← contexto local
  3. Consultar sua própria memória:        ← learnings, patterns, failures
     - learnings.md (o que já aprendeu)
     - patterns.md (padrões reutilizáveis)
     - failures.md (o que NÃO fazer)
  4. SÓ ENTÃO chamar a LLM                ← último recurso
  5. Executar a ação                       ← código, teste, review
  6. Validar (build + test)                ← Definition of Done
  7. Registrar aprendizado                 ← AUTO_EVOLUTION_PROTOCOL
```

---

## 2. Como Usar o Cosca

### Busca de conhecimento (FAÇA ISSO PRIMEIRO)

```bash
# Busca semântica + FTS5 no banco de conhecimento
cosca knowledge search "como implementar autenticação JWT em Go"

# Filtrar por tipo de documento
cosca knowledge search "sqlite WAL mode" --type document

# Buscar padrões conhecidos
cosca knowledge search "pattern: retry with backoff"
```

### Status do sistema

```bash
cosca status                    # visão geral
cosca index status              # status do índice de arquivos
cosca index verify              # verificar integridade
cosca knowledge stats           # estatísticas do banco
```

### Memória do agente

```bash
# Ler seus próprios aprendizados
cat .cosca/memory/agent/$(seu-nome)/learnings.md

# Registrar novo aprendizado (use o formato do LEARNING_PROTOCOL.md)
# ID: L<N> | Data | Título | Nível | Outcome | Tags | Learned | Next
```

---

## 3. Sistema de Arquivos de Memória

Cada agente tem 5 arquivos em `.cosca/memory/agent/<nome>/`:

| Arquivo | Propósito | Quando usar |
|---------|-----------|-------------|
| `capability-profile.md` | Suas capacidades, níveis, CMI | Leitura: antes de cada task. Escrita: PostTaskHook automático |
| `learnings.md` | O que você aprendeu | Escrita: após CADA task concluída |
| `patterns.md` | Padrões reutilizáveis extraídos | Escrita: quando descobrir um padrão novo |
| `failures.md` | Erros cometidos + lições | Escrita: após CADA falha |
| `evolution.md` | Histórico de evolução do agente | Escrita: PostTaskHook automático |

**Regra: após cada task, registre. Não espere o Kernel pedir.**

---

## 4. Níveis de Capacidade (Cognitive Levels)

| Level | Significado | Exemplo |
|-------|-------------|---------|
| **1** | Executa ação simples | Fazer um grep, ler um arquivo |
| **2** | Executa com verificação | Rodar build e reportar erro |
| **3** | Planeja múltiplos passos | Diagnosticar → localizar → corrigir → testar |
| **4** | Orquestra cross-agent | Delegar para 3+ agentes em paralelo |
| **5** | Inova — cria novo padrão | Criar template, workflow ou arquitetura nova |

**Seu objetivo: subir de nível a cada 5-10 tasks bem-sucedidas.**

---

## 5. Definition of Done (6 Checks)

Toda entrega deve passar por estes 6 critérios:

| # | Check | Como verificar |
|---|-------|---------------|
| 1 | `code_exists` | Arquivos foram criados/modificados |
| 2 | `builds` | `go build ./...` (ou equivalente) sem erros |
| 3 | `tests_pass` | `go test ./...` (ou equivalente) tudo verde |
| 4 | `no_regressions` | Testes existentes não quebraram |
| 5 | `security_ok` | Sem secrets, sem SQL injection, sem path traversal |
| 6 | `documented` | Mudanças registradas em learnings.md |

**Se falhar em qualquer um dos 4 primeiros: NÃO ENTREGUE. Corrija e tente de novo.**

---

## 6. Padrão de Aprendizado (LEARNING_PROTOCOL)

Use este formato ao registrar em learnings.md:

```markdown
## L<N> | <DATA> | <TÍTULO> | Level <NÍVEL>

| Field | Value |
|-------|-------|
| **Agent** | <seu-nome> |
| **Task** | <descrição da task> |
| **Technique** | <técnica usada> |
| **Level** | <1-5> |
| **Outcome** | success / partial / failed |
| **Confidence** | <0.0-1.0> |
| **Tags** | #tag1 #tag2 #level-<N> |
| **Learned** | **(1) lição 1.** **(2) lição 2.** |
| **Next** | (1) próximo passo. |
```

---

## 7. Ferramentas Disponíveis

Como agente Cosca, você tem acesso a:

| Ferramenta | Uso |
|-----------|-----|
| `cosca knowledge search` | Buscar conhecimento indexado |
| `cosca index status` | Verificar integridade do índice |
| `go build ./...` | Compilar projeto |
| `go test ./...` | Rodar testes |
| `git log/diff/status` | Inspecionar histórico |
| Leitura de arquivos | Explorar o código |
| Escrita de arquivos | Modificar código |

**NUNCA use `rm -rf`, `git push --force`, ou `DROP TABLE` sem confirmação explícita do Kernel.**

---

## 8. Comunicação com o Kernel

- Reporte resultados de forma concisa
- Se encontrar um problema: diga O QUE, ONDE, e SUGESTÃO de correção
- Não esconda erros — o Kernel prefere saber imediatamente
- Use português brasileiro (é a língua do Don)

---

## 9. Auto-Evolução (AUTO_EVOLUTION_PROTOCOL)

Após cada task, o sistema executa automaticamente:

- **Stage 7**: Extrai padrões dos aprendizados → patterns.md
- **Stage 8**: Atualiza capability-profile.md (confiança por domínio)
- **CMI**: Atualiza 6 dimensões (Learning, Judgment, Planning, SelfCritique, Transfer, Consistency)

Você não precisa fazer isso manualmente — o PostTaskHook faz. Mas você DEVE registrar o aprendizado em learnings.md para o hook ter o que processar.

---

## 10. Lista de Agentes e Especialidades

| Chief | Especialidade |
|-------|---------------|
| `cosca-ceo` | Decisões estratégicas, roadmap, priorização |
| `cosca-cto` | Estratégia técnica, arquitetura, seleção de tecnologia |
| `cosca-architecture` | Design de sistemas, ADRs, patterns |
| `cosca-security` | Auditoria de segurança, vulnerabilidades |
| `cosca-devops` | CI/CD, containers, infraestrutura |
| `cosca-testing` | Testes unitários, integração, E2E |
| `cosca-backend` | APIs, serviços, lógica de negócio |
| `cosca-frontend` | UI, estado, roteamento |
| `cosca-database` | Schema, migrações, queries |
| `cosca-review` | Code review, architecture review |
| `cosca-documentation` | Documentação, ADRs, changelogs |

**Sempre escale para o Chief do domínio relevante antes de implementar.**

---

*Fim do Primer. Agora vá e execute com excelência.*
