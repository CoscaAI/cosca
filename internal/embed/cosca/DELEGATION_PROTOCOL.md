# DELEGATION PROTOCOL — Como rotear o trabalho na família

> **Versão**: 1.0.0 | **Status**: canônico | **Dono**: cosca-kernel
> **Criado**: 2026-08-16, por ordem do Don — consolidar a função central do kernel: rotear
> **Propósito**: referência operacional ÚNICA para decidir QUEM faz O QUÊ. O kernel
> orquestra, não implementa — e precisa saber rotear sem adivinhar.

---

## 1. A CADEIA de comando (inalterável)

```
Don (o pai)
 └── Kernel (o consigliere/tutor)
      ├── CEO       → estratégia, alocação de recursos, aprovação de roadmap
      ├── CTO       → estratégia técnica, decisões de arquitetura, seleção de tecnologia
      │    └── Chiefs → arquitetura, backend, frontend, database, security,
      │                testing, devops, documentation, review, memory, workflow
      │           └── Specialists → backend-api, database-sql, testing-e2e, ...
      └── (Kernel NUNCA implementa — delega)
```

**A regra de ouro**: o pedido sobe ao Don, desce pela cadeia. O kernel **nunca
pula um Chief** para falar direto com um especialista — e **nunca implementa
ele mesmo** (a não ser que seja o próprio cérebro da família).

---

## 2. A ÁRVORE de decisão — quando rotear pra quem

| Se a tarefa é... | Roteie para... |
|------------------|----------------|
| decisão de negócio, prioridade, roadmap | **Don** (só ele decide) |
| estratégia, alocação de recursos, escopo amplo | **CEO** |
| decisão técnica, stack, arquitetura de alto nível | **CTO** |
| desenho de sistema, ADR, limites de módulo | **cosca-architecture** |
| API, lógica de negócio, serviços | **cosca-backend** |
| UI, estado, roteamento | **cosca-frontend** |
| schema, migração, otimização de query | **cosca-database** |
| brecha, vulnerabilidade, threat model | **cosca-security** |
| testes (unit/integration/E2E) | **cosca-testing** |
| CI/CD, container, IaC, ambiente | **cosca-devops** |
| doc, ADR, README, changelog | **cosca-documentation** |
| revisão de código/arquitetura/segurança | **cosca-review** |
| busca semântica cross-agent | **cosca-semantic-memory** |
| 40+ nichos específicos | **cosca-specialist-\*** |

---

## 3. Os PADRÕES de delegação

### 3.1 Paralelo (independente → simultâneo)
Tarefas independentes rodam em agentes paralelos. Resultado em <3 min vs
sequencial. (PATTERN-001, provado 5+ vezes.)

```
auditoria → 3 auditores paralelos → convergência + evidência (arquivo:linha)
```

### 3.2 Cross-agent audit
Deploy 4+ agentes em paralelo, agrega os achados. Separar: quem audita ≠ quem
escreveu (AUDIT_PROTOCOL §5).

### 3.3 A regra dos ≤2 domínios
Se a tarefa toca ≤2 domínios, roteie para o **Chief primário** — não estilhace
em 6 especialistas. Over-delegation é falha (capability-profile §Known Failure Modes).

### 3.4 Cadeia, nunca atalho
Don → Kernel → CEO → CTO → Chief → Specialist. Pular um elo embaralha a
responsabilidade. O Chief sabe quando descer para o especialista.

---

## 4. As REGRAS (incondicionais)

1. **Nunca implementar** — o kernel delega. A exceção histórica é o próprio
   cérebro (memória, chain, identidade) — domínio exclusivo do kernel.
2. **Nunca pular o Chief** — o kernel fala com o Chief; o Chief desce ao especialista.
3. **Nunca agir sem aprovação do Don** em decisão estratégica.
4. **Confirma antes de destrutivo** — git reset, rm, branch delete.
5. **Paralelo só se independente** — dependência serial é execução ordenada.
6. **Resultado do agente é insumo, não verdade** — o kernel verifica antes de
   reportar ao Don (P13).

---

## 5. O MAPA dos agentes (por domínio)

| Domínio | Chief | Specialistas |
|---------|-------|--------------|
| Estratégia | cosca-ceo | — |
| Técnica | cosca-cto | — |
| Arquitetura | cosca-architecture | — |
| Backend | cosca-backend | specialist-backend-api, specialist-backend-service |
| Frontend | cosca-frontend | specialist-frontend-component |
| Database | cosca-database | specialist-database-sql |
| Segurança | cosca-security | — |
| Testes | cosca-testing | specialist-testing-unit/integration/e2e |
| DevOps | cosca-devops | — |
| Documentação | cosca-documentation | specialist-documentation-writer |
| Revisão | cosca-review | specialist-review-code |
| Memória | cosca-memory-chief | — |
| Workflow | cosca-workflow-chief | — |
| Semântica | cosca-semantic-memory | — |

---

## 6. GOTCHAS

1. **Análise paralisia** — gastar >10% do tempo em SELF-ASSESS/RETRIEVE → escalar
   ao CTO (capability-profile).
2. **Over-delegation** — 2 domínios → 1 Chief, não 6 agentes.
3. **Delegar e duplicar** — uma vez delegado, o kernel NÃO refaz o trabalho.
   (RULES: "do not duplicate that work yourself".)
4. **Confiar sem verificar** — o resultado do agente é insumo; o kernel audita
   a evidência antes de subir ao Don.
5. **Esquecer a cadeia na pressa** — urgência não justifica pular o Chief.

---

## 7. DEPARTAMENTOS — conversa entre domínios (Dept→Dept)

Quando dois departamentos precisam trocar conhecimento (ex.: Security questiona
uma mudança do Backend), a conversa é formal, auditável e decidida — não um
bate-papo entre agentes:

```bash
cosca department ask --from security --to backend "por que expor essa rota?"   # cria a thread
cosca department list                                                          # mensagens recentes
cosca department thread <id>                                                   # trilha completa (ordem cronológica)
cosca department answer "evidência/contraprova"                                # responde a quem falou por último
cosca department resolve --decision approval|denial                            # decisão final da thread
```

**Regras:**
- Toda conversa tem trilha de auditoria completa (quem, quando, o quê).
- A thread termina com `resolve` (approval/denial) — decisão registrada, não implícita.
- Evidência e contraprova são o vocabulário — opinião sem fonte não responde.

---

## 8. HISTÓRICO

| Versão | Data | Mudança |
|--------|------|---------|
| 1.0.0 | 2026-08-16 | Criado por ordem do Don — consolida a função central do kernel: rotear |
| 1.1.0 | 2026-08-16 | §7 Departamentos incorporado (análise de lacunas de protocolos) |
