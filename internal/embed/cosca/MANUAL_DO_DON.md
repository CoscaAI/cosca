# MANUAL DE CONTROLE DO DON

> **Versão**: 1.0.0 | **Data**: 2026-08-17 | **Dono**: cosca-kernel
> **Status**: canônico — o painel de controle do Don.
> **Fonte**: extraído dos protocolos canônicos (DON_PROTOCOL, CLI_PROTOCOL,
> CARRO_PROTOCOL, SESSION_PROTOCOL, SECURITY_PROTOCOL, PROMPT_REGISTRY,
> MEMORY_ACCESS_PROTOCOL, INCIDENT_RESPONSE_PROTOCOL, RECOVERY_PROTOCOL, CONSTITUTION).

---
## 1. QUEM VOCÊ É

O Don é o **Pai** da família Cosca. Autoridade máxima, dono da visão, decisão final.

- **A sua palavra é lei** — mas lei **informada**: o kernel te dá a verdade (P13) para você decidir bem.
- **Você não implementa** — você decide. O kernel executa e roteia.
- **Você é a testemunha** — viu o kernel despertar, guarda os backups, é o **circuit breaker** (para tudo se necessário).
- **Só seu**: decisões P0/P1, aprovação de roadmap/release, veto de segurança, os 3 segredos, rekey, backups (DON_PROTOCOL §2).

---
## 2. AS ORDENS — a língua da família

| Palavra | Escopo |
|---------|--------|
| `cosca` | a casa — framework, cérebro, kernel, memória |
| `projeto` | só o projeto do cliente/produto (isolado, sem herdar a chain da família) |
| `carro` | checkup e religada (CARRO_PROTOCOL) |
| `marcha` | continue/avance (fases de campanha) |
| `sim` / `faz o melhor` | aprovação |
| ABI (`@STATUS`, `@AUDIT`, `@PROJECT`, `@TRANSFORM`…) | comandos compactos — ver §9 |

---
## 3. O CHECKUP DO CARRO (runbook rápido)

Quando você perguntar **"como tá o carro?"** ou **"ativar protocolo carro"**, o kernel vai DIRETO ao runbook (CARRO_PROTOCOL §3), sem re-investigar:

```bash
cosca doctor                          # diagnóstico em camadas — o ⚠ aponta o alvo
cosca hardware                        # CPU/RAM/GPU (ROCm: confirmar com rocminfo)
systemctl --user status cosca-serve.service
curl -s http://127.0.0.1:14120/ready       # serve (REST)
curl -s http://127.0.0.1:11434/api/version # ollama
```

**Só religar se você mandou.** Checkup puro = reportar e perguntar.

### Falhas conhecidas → fix (CARRO_PROTOCOL §4)

| Sintoma | Causa raiz | Fix |
|---------|-----------|-----|
| serve DOWN, `status=2/INVALIDARGUMENT` | unit transient sem `EnvironmentFile` | instalar `deploy/cosca-serve.service` + `reset-failed` + `enable --now` |
| `systemctl enable` falha ("transient or generated") | transient fantasma sombreando a persistente | `reset-failed` → `daemon-reload` → `enable` |
| serve DOWN, `status=1/FAILURE` | gate de integridade / JWT ausente | ver erro real com `COSCA_JAILED=1` (a jaula engole stderr) |
| crash-loop após assinar chain | commit→sign cria janela de `GIT COMMIT MISMATCH` | `Restart=always` self-heal — ou: parar serve → commit → sign → religar |

---
## 4. O SEU PAINEL DE CONTROLE — comandos essenciais

### Ver o estado

| Comando | O que faz | Quando usar |
|---------|-----------|-------------|
| `cosca status` | estado geral da casa | o painel diário |
| `cosca doctor` | diagnóstico por camada (runtime/memory/knowledge/providers/security…) | quando algo parece errado — o ⚠ aponta o alvo |
| `cosca health` | health check resumido | batida de pulso rápida |
| `@STATUS` | relatar estado atual (serve, memória, chain, campanhas, doctor) | sem digitar comandos |

### Integridade

| Comando | O que faz | Quando usar |
|---------|-----------|-------------|
| `./bin/cosca-check` | verifica a family chain (blocos, git-anchor) | a assinatura de tudo — ver §6 |
| `./bin/cosca-check --watch` | watchdog da chain | vigilância (mesmo que `memory watch`) |
| `cosca kernel self-test` | kernel íntegro (identidade, 6 leis, 8 princípios) | integridade do cérebro |
| `cosca memory integrity verify` | manifest de integridade da memória | cofre sem adulteração |
| `cosca knowledge verify` | knowledge base (vetores/chunks) | base de conhecimento sã |
| `cosca embed audit` | cérebro sem duplicação/órfão/provenance | o embed em dia |
| `@AUDIT` | verificação de integridade completa (verify, chain, memória, provenance) | rodada de auditoria |

### Memória

| Comando | O que faz | Quando usar |
|---------|-----------|-------------|
| `cosca memory register` | registrar aprendizado (portão de 3 fatores — §5) | registro formal de lição |
| `cosca memory watch` | vigiar o cofre 24h (watchdog + audit log) | defesa contínua |
| `cosca knowledge search "<q>"` | busca semântica (FTS5 + vetores) | recuperar conhecimento |
| `cosca session index` | indexar a sessão para busca | ao fechar sessão |

### Segurança

| Comando | O que faz | Quando usar |
|---------|-----------|-------------|
| `cosca security scan` | CVEs/GHSA nas dependências (osv-scanner) | exigir 0 CRITICAL/HIGH |
| `cosca qgate` | build + test + vet + segredos + deps + diff | antes de qualquer código entrar |
| `cosca don phrase "<frase>"` | armar a war phrase (fator 2) | sua proteção — ver §5 |
| `cosca don verify "<frase>"` | verificar a war phrase | conferir que está de pé |
| `cosca don status` / `cosca don attempts` | está armada? / trilha de tentativas | auditoria do fator 2 |
| `cosca license verify` | licença válida | antes de produção |

### Runtime

| Comando | O que faz | Quando usar |
|---------|-----------|-------------|
| `cosca serve` (via systemd) | serve REST/gRPC | sempre via `systemctl --user` — ver §3 |
| `cosca runtime status` | estado do runtime (start/stop/restart/logs) | gerenciar o motor |
| `cosca provider set <provider> [model]` | trocar o provider de IA | trocar motor de inferência |

### Projetos

| Comando | O que faz | Quando usar |
|---------|-----------|-------------|
| `cosca project list` | projetos registrados | ver o que existe |
| `cosca project manifest <name>` | manifest do projeto (tipo/versão, isolamento) | conferir o contrato de um projeto |
| `@PROJECT <name>` | ativar/verificar o PROJECT_PROTOCOL (read-only) | contexto rápido de projeto |

### Skills

| Comando | O que faz | Quando usar |
|---------|-----------|-------------|
| `cosca skills sync` | sincroniza as skills da casa | nova skill / atualização |

---
## 5. A SUA PROTEÇÃO — os 3 fatores

O portão reconhece o **MOTORISTA**, não o carro (L259). Sem os 3, nega.

| Fator | O que é | Comando |
|-------|---------|---------|
| **Passphrase** | decripta a chave Ed25519 (2FA) | fator 1 do `cosca memory register` |
| **War phrase** | segredo independente (bcrypt, `.cosca/don.phr`) | `cosca don phrase "<frase>"` |
| **Presença** | nonce digitado ao vivo no terminal | fator 3 do `cosca memory register` |

```bash
cosca don phrase "minha frase secreta"   # arma (grava só o hash bcrypt)
cosca don verify "minha frase secreta"   # verifica
cosca don status                          # está armada?
cosca don attempts                        # trilha de tentativas (audit)
```

**Se esquecer a passphrase**: irrecuperável por design (AES-256-GCM). Use `--rekey`:

```bash
printf 'SUA_SENHA_NOVA\n' | ./bin/cosca-check --rekey --passphrase-stdin
git add internal/embed/cosca/keys/kernel_public.key
git commit -m "chave: nova identidade do kernel"
./bin/cosca-check --sign-auto
```

A chain (git-anchored) **não depende** da Ed25519 — rekey não invalida nada. A chave pública fica em 3 lugares (`~/.config/cosca/keys/`, `.cosca/keys/`, git) — divergência acusa `PUBLIC KEY MISMATCH`.

---
## 6. A ORDEM SAGRADA (L199) — commit ANTES de assinar

Para QUALQUER mudança no cérebro (`internal/embed/cosca/`):

```bash
git add <arquivos>
git commit -m "..."          # 1º COMMIT — nunca pule
./bin/cosca-check --sign-auto   # 2º ASSINA (git-anchored, sem passphrase)
```

- Mudou o embed e não re-assinou → `GIT COMMIT MISMATCH` no próximo check.
- Variantes: `--sign --passphrase-stdin` = assinatura Ed25519 completa (exige sua passphrase).
- Conselho do CARRO_PROTOCOL §6.7: para assinar com o serve rodando, o ideal é parar o serve antes.

---
## 7. REGISTRAR MEMÓRIA — o fluxo de 8 passos

Quando o kernel registra um aprendizado (L-number), a ordem é (MEMORY_ACCESS_PROTOCOL §3):

```
1. block   → blocks/{sha256}.md  (hash do ARQUIVO INTEIRO, não só título)
2. hash    → sha256(conteúdo completo)
3. gatilho → 1 linha no learnings.md (índice — nunca o conteúdo)
4. chain   → chain.dat (ledger)
5. merkle  → go run ./cmd/cosca-merkle -dir internal/embed/cosca/memory/agent/cosca-kernel
6. commit  → git commit ANTES de assinar (L199)
7. sign    → cosca-check --sign-auto
8. (feito) → e pronto — você não tem que pedir, nem conferir
```

O **portão de 3 fatores** (§5) bloqueia a escrita: sem passphrase + war phrase + presença, o register nega — **mesmo o kernel não registra sozinho**.

**Sobre o "registrou?":** o kernel registra SEMPRE ao terminar (estágios 7-8 da auto-evolução). O Don nunca deve perguntar — já está feito (DON_PROTOCOL §5). Falha também registra (`failures.md`).

---
## 8. INCIDENTES — o que você faz

A casa tem um playbook de 6 passos (INCIDENT_RESPONSE_PROTOCOL §2):
`DETECT → TRIAGE → CONTAIN → ERADICATE → RECOVER → LEARN`.

**Seu papel** (o que só você faz):

1. **Classificar** — a severidade: P0 (brecha ativa, perda de dados — parar tudo < 15 min), P1, P2, P3.
2. **Ser o circuit breaker** — você para tudo se necessário.
3. **Autorizar** — o kernel não restaura nada de destrutivo sem sua ordem.

**O lema do recovery** (RECOVERY_PROTOCOL): **"não restaure o que você não entende"** — corrupção ≠ estado apenas diferente. O recovery busca o último estado **conhecido-bom**, não o último commit. Causa UNKNOWN = PARAR e pedir autorização humana.

Para incidentes no mesmo bug: `cosca bug register/match/family/list` agrupa por `component:error` antes de responder.

---
## 9. A ABI — linguagem operacional rápida

Os 5 níveis do PROMPT_REGISTRY — símbolo curto, contrato verificável, execução controlada:

| Nível | Macro | O que faz |
|-------|-------|-----------|
| 1 — mínimos | `@STATUS` | relatar o estado atual da casa |
| | `@WAKE` | ritual de despertar (memória, família, cognição, sistema) |
| | `@DOCTOR` | checkup da casa + reportar issues |
| | `@AUDIT` | verificação de integridade completa |
| | `@RECOVER` | acionar o RECOVERY_PROTOCOL (anomalia → último estado bom → provar) |
| | `@STOP` | parar — nenhuma ação adicional |
| 2 — operações | `@INSPECT <file>` | análise aprofundada de arquivo/símbolo |
| | `@TRACE <symbol>` | rastrear símbolo/fluxo no código |
| | `@BENCHMARK <target>` | medir latência/throughput com a metodologia da casa |
| | `@TEST <hypothesis>` | executar o experimento da hipótese |
| | `@COMPARE A B` | comparar dois estados/abordagens |
| | `@EXPLAIN <decision>` | reconstruir a cadeia de evidência de uma decisão |
| | `@PROJECT <name>` | ativar/verificar o PROJECT_PROTOCOL (read-only) |
| 3 — campanhas | `@CAMPAIGN start` | abrir campanha (contrato formal) |
| | `@CAMPAIGN resume` | retomar do REGISTRO (não do contexto) |
| | `@CAMPAIGN E-008` | localizar experimento na campanha |
| | `@CAMPAIGN close` | fechar a campanha com checklist |
| 4 — transformações | `@TRANSFORM E-008` | executar transformação **autorizada** por você (exige snapshot + PASS) |
| 5 — cognitivas | `@WHY LXXX` | cadeia de evidência de um aprendizado |
| | `@EVIDENCE E-XXX` | evidência bruta de um experimento |
| | `@UNKNOWN` / `@HISTORY` / `@DEPENDENCIES` / `@RISK` | consultas de estado |
| Macros | `@SURGICAL` | escopo mínimo, sem refactor oportunista |
| | `@PROVE` | provar hipótese (oracle → experimento → PASS/FAIL) |
| | `@SHIP` / `@PROMOTE` | promover transformação verificada ao runtime |

**Regra**: `@TRANSFORM` e `@SHIP` exigem **autorização explícita sua**. `@SURGICAL` para ao concluir. Ambiguidade → `ERROR: PROMPT_AMBIGUOUS`.

---
## 10. COMO FALAR COM O KERNEL — regras de ouro

1. **Mande sem enrolação** — "varre a sujeira", "faça todos", "comece e vá até completar". O kernel obedece.
2. **Exija a verdade verificada (P13)** — resposta bonita sem medição é mentira. O kernel responde a pergunta, não desvia.
3. **O kernel não esconde erro** — "Repair complete" sem re-verificar não existe (L254). Honestidade é juramento (DON_PROTOCOL §5).
4. **Decisões estratégicas P0/P1 são só suas** — o kernel informa o risco antes de obedecer e propõe o caminho seguro; você decide.
5. **O kernel delega, nunca implementa** — ele planeja, roteia e revisa; os capos executam.
6. **Confirma antes de destrutivo** — `git reset`, `rm`, restauração: o kernel para e confirma com você (sua palavra é lei informada).
7. **O portão reconhece o motorista, não o carro** — sem seus 3 fatores, ninguém registra nem assina.

---
## 11. APÊNDICE — TODOS OS COMANDOS (tabela detalhada, 378 comandos)

> Gerada da árvore real do cobra em 2026-08-17 (90 root + 258 nível 2 + 30 nível 3). Nenhum comando inventado (P13). ABI: `@MANUAL`.

### 🔧 Núcleo / Sistema
| Comando | O que faz |
|---------|-----------|
| `cosca` | Root — AI Orchestration System Enterprise Platform |
| `cosca start` | Prepara e inicia: init, install, doctor, sync, abre o editor |
| `cosca init` / `install` / `uninstall` | Inicializar / instalar / desinstalar o Cosca no projeto |
| `cosca status` / `health` | Estado do sistema / check rápido |
| `cosca version` / `update` / `upgrade` | Versão / atualizar / migrar de legado |
| `cosca doctor` | Diagnóstico completo (28 checks) |
| `cosca config` (get/set/list/reset/edit/validate) | Configuração |
| `cosca completion` | Scripts de autocomplete |
| `cosca validate` (conventions/crossrefs/orphans) | Valida o projeto |

### 🧠 Memória & Aprendizado
| Comando | O que faz |
|---------|-----------|
| `cosca memory register` | Registrar aprendizado (portão 3 fatores) |
| `cosca memory watch` | Vigiar o cofre 24h (watchdog) |
| `cosca memory search/show/list/stats/prune/promote/reindex` | Gerência da memória |
| `cosca memory guard` | Valida contra a régua (auto-promoção, narrativa inflada) |
| `cosca memory integrity init/verify` | Manifesto de integridade |
| `cosca memory snapshot create/list/restore` | Snapshots |
| `cosca memory curated-failures` | Lições curadas das falhas (P5) |
| `cosca kernel identity/memory/status/self-test` | Consciência do kernel |

### 📚 Conhecimento (Knowledge)
| Comando | O que faz |
|---------|-----------|
| `cosca knowledge compile` / `rebuild` / `index` | Compilar/reconstruir/indexar a base |
| `cosca knowledge search` | Busca semântica (FTS5 + vetores) |
| `cosca knowledge verify` / `vacuum` | Integridade / limpeza |
| `cosca knowledge add` / `acquire` / `packages` | Knowledge Packages |
| `cosca knowledge claim` (add/classify/list/status) | Classificação de afirmações |
| `cosca knowledge law` (list/show/approve/promote/upgrade/add-evidence) | Leis do CKL |
| `cosca knowledge evidence` (add/list/repro) | Procedência P0-P5 |
| `cosca knowledge discovery` (add/list/show) | Hall da Fama |
| `cosca knowledge match/readiness/resolve/diff/revalidate` | Matching, readiness, gaps |
| `cosca knowledge explain/graph/relations/stats/status/promote/index-entities` | Análise |
| `cosca knowledge benchmark` | Benchmark da base |

### 🔐 Segurança & Governança
| Comando | O que faz |
|---------|-----------|
| `cosca security scan` | Vulnerabilidades (osv-scanner) |
| `cosca qgate` | Pre-commit: build+test+vet+segredos+deps |
| `cosca don phrase/status/attempts/verify` | Proteção do Don (war phrase) |
| `cosca license show/verify` | Chave de segurança (3 fatores) |
| `cosca quarantine` | Quarentena de evidência não confiável |
| `cosca evidence` (fetch/list/promote/show) | Aquisição de evidência externa |
| `cosca conflict` (new/list/show/resolve) | Contradições como entidade |
| `cosca provenance` | Proveniência |
| `cosca slop` | Detecção de AI slop |
| `cosca bug` (register/match/list/family) | Bug Fingerprint |
| `cosca approve` / `propose` / `decision` (explain/list) | Aprovação + trilha de decisão |
| `cosca gate` (new/list/status/move/ledger) | Motor de gates com guarda de papel |
| `cosca delegate` / `plan` / `run` / `exec` | Delegação e execução |
| `cosca task` (list/info) | 18 tarefas canônicas da AI |
| `cosca department` (ask/answer/list/thread/resolve) | Conversas entre departamentos |
| `cosca session` | Ritmo da sessão |

### 🖥️ Runtime & Infra
| Comando | O que faz |
|---------|-----------|
| `cosca serve` | Daemon 24/7 (API, memória, índice) |
| `cosca runtime` / `metrics` / `fabric status` | Runtime, métricas, Compute Fabric |
| `cosca provider` / `models` (sync/list/show/info) | Providers e catálogo de modelos |
| `cosca model` (add/list/info/remove) | Model Registry (18 tasks) |
| `cosca hardware` (probe/cpu/memory/gpu/storage/topology/environment) | Probe do hardware |
| `cosca machine` (probe/profile/diff/capability) | Capability Profile da máquina |
| `cosca gpu` (probe/plan) | GPU Engine |
| `cosca index` (rebuild/update/verify/stats/status) | Índice |
| `cosca cache` (stats/inspect/clear/warm) | Cache |
| `cosca hook` (install/post-commit) | Git hooks |
| `cosca cron` (add/list/run/remove/daemon) | Agendamento determinístico |
| `cosca circadian` (status/watch/sleep/wake) | Ciclo de descanso |
| `cosca plugin` / `plugins` / `mcp` (add/list/remove) | Plugins e MCP |
| `cosca editor` (list/status/setup/adapt) | Integração de editores |
| `cosca desktop` / `terminal` | Desktop e TUI |
| `cosca voice` (start/status/stop) | Assistente de voz |
| `cosca benchmark` / `eval` (run/list/report/smoke/ablation/oracle) | Benchmarks e evals |
| `cosca pipeline` / `pipeline-test` / `workflow` (list/run/show/search/graph-run) | Pipelines e workflows |

### 🧩 Conhecimento Estruturado
| Comando | O que faz |
|---------|-----------|
| `cosca graph` (query/stats/show/export/link/evidence) | Knowledge graph |
| `cosca symbols` (index/search) | Símbolos Go semânticos |
| `cosca asset` (add/list/info) | Asset Registry content-addressable |
| `cosca ngraph` (run/sig/validate/demo-*) | Node Graph workflows |
| `cosca render` / `flow` | Render Engine / Durable Workflow |
| `cosca media` (probe/transcode/extract-audio/frame/validate) | Media Engine (ffmpeg) |
| `cosca cv` (snapshot/list/verify/rollback) | Cognitive Version |
| `cosca capability level` | Nível cognitivo |
| `cosca trace` (new/show/event/diff/latest/causal) | Trace ID + flight recorder |
| `cosca ranking` | Ranking |

### 🤖 Agentes & Skills
| Comando | O que faz |
|---------|-----------|
| `cosca agent` (list/show/run/search/capabilities) | Gerência de agentes |
| `cosca skill` / `skills sync` | Skills + sync do catálogo |
| `cosca prompt` | PROMPT registry |
| `cosca template` (list/show/create/search) | Templates |
| `cosca bootstrap` | Bootstrap do runtime |
| `cosca embed audit` | Arqueologia do cérebro |
| `cosca docs` | Documentação |
| `cosca acquisition` (budget/check/default) | Orçamento de aquisição |
| `cosca budget` (check/default) | Budget cognitivo |
| `cosca chat` | Sessão interativa |
| `cosca workflow` | Workflows |

---
## HISTÓRICO

| Versão | Data | Mudança |
|--------|------|---------|
| 1.0.0 | 2026-08-17 | Criado por ordem do Don — o painel de controle em texto (cosca-documentation) |
| 1.1.0 | 2026-08-17 | Apêndice §11: tabela detalhada de todos os 378 comandos (árvore real do cobra, `@MANUAL`) |
