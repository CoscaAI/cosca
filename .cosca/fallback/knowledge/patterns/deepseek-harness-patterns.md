# DeepSeek Harness Patterns — Plugins, Sandbox, Sessão, Skills, Protocolos

> **Version**: 1.0.0 | **Confidence**: 0.90 | **Category**: Enterprise Platform Patterns | **Created**: 2026-08-22 | **Source**: https://github.com/deepseek-ai/deepseek-harness (MIT)

> **Mined by**: cosca-kernel (ordem do Don). Extraído via 5 batedores paralelos sobre o checkout local de `native/landlock-run/`, `packages/*`, `vendor/cordis/` e `scripts/`. O harness de agente onde **tudo é um plugin**, sobre o framework Cordis (paradigma de composição *espaciotemporal*). Alvo direto dos gaps da família: sandbox P0, auto-evolução sem auto-destruição, skill registry, gates de verificação, protocolos.

## Purpose

O DeepSeek Harness resolve o problema de **compor um agente inteiro a partir de plugins declarativos, isolados e verificáveis** — do loop ao sandbox até o clique de aprovação humana. O Cosca tem os blocos (51 agentes, 71 skills, memória durável, QUALITY_GATES), mas o dsh mostra 3 lições estruturais que faltam: (1) **orquestração por composição declarativa** (não código); (2) **sandbox com enforcement provado e fail-closed**; (3) **rede de invariantes geradas-e-verificadas no CI**.

---

## A. Arquitetura de Plugins & Paradigma Cordis

### A1. Contexto-serviço sem núcleo privilegiado (IoC por chave de serviço)
- **O que resolve**: eliminar o "core patchable" — não existe núcleo a modificar; qualquer capacidade é um plugin montado ao lado dos outros; composição substituível vem de config.
- **Como funciona**: `Context` é um `Proxy`; ler `ctx.tools`/`ctx.llm` passa por um trap `get` que resolve o serviço pela chave ou delega num `waterfall` que permite à política interceptar o próprio acesso. Plugin declara dependência via `inject` (atrasa o carregamento até o serviço existir); implementação registrada por `provide`/Subclasse de `Service`. Mixins expõem métodos de serviço direto no `ctx`.
- **Onde**: `vendor/cordis/src/{context,service,reflect,registry,events}.ts`, `docs/architecture.md`.
- **Aplicação no Cosca**: cada capability (agente, skill, tool, memória, provider) vira um serviço `ctx.<chave>` + tipos registrados por `inject`. Nada hard-coded como core; adicionar orquestrador é montar plugin ao lado. A dependência `inject` é o mecanismo "preciso de X antes de mim" sem boot-order manual.

### A2. Efeito reversível + ativação reativa a dependências (eixo temporal)
- **O que resolve**: registros com ciclo de vida definido que se desfazem na descarga; (des)ativação dirigida por disponibilidade de serviço.
- **Como funciona**: todo registro é um `effect` — `execute` roda imediato e os disposers são executados em ordem inversa no unload. O plugin-roda é um `Fiber` com máquina `PENDING→LOADING→ACTIVE→FAILED→UNLOADING→DISPOSED`. Quando serviço injetado muda, recomputa um `epoch` (concatenação dos `uid` dos provedores) e dispara `_unload`/`_reload` automático. `dispose` é single-shot e idempotente.
- **Onde**: `vendor/cordis/src/fiber.ts`, `vendor/README.md`.
- **Aplicação no Cosca**: qualquer registro de agente "vivo" (timer, worker, watcher) vira efeito no fiber do plugin — desmontar o agente limpa tudo em ordem previsível. Recarregar config de um orquestrador realoca o plugin automaticamente; serviços substituídos reativam dependentes sem restart.

### A3. Espacial: realms de isolamento + cadeia de escopos no evento
- **O que resolve**: o mesmo binário serve muitas sessões/agentes sem colisão — cada um vê um conjunto diferente de tools/prompt sem duplicar código.
- **Como funciona**: `isolate(name, label)` cria filho de contexto cujo serviço resolve num realm próprio. `ScopedLayers` mantém camadas `global → cadeia de escopos → exato`, com herança para baixo; listeners de escopo ancestral recebem eventos dos descendentes. Registro via `agent.ctx` vai para a camada do agente e desfaz com ele.
- **Onde**: `vendor/cordis/src/context.ts`, `packages/core/scope/src/{index,store}.ts`.
- **Aplicação no Cosca**: cada skill/tool/agent reside num realm de escopo; a visão de um agente resolve a cadeia agente→preset→global, permitindo herdar e sobrescrever. Orquestração de subagentes usa composição herdada do pai (não remonta).

### A4. Config como dado + camadas de patch com `!!js` preguiçoso
- **O que resolve**: composição declarativa, auditável e patchável de cima para baixo.
- **Como funciona**: o `cordis.yml` é uma lista de linhas `{id,name,config,inject,disabled}`. O Loader constrói árvore de Entries, importa o módulo e chama `ctx.plugin`. Camadas aplicam sobre lista vazia: bundles → patch do profile → patch home → `--patch`. `!!js` é interpolado só na ativação por config preguiçoso. Updates são transacionais com rollback.
- **Onde**: `vendor/loader/src/{index.ts,config/{tree,entry,group}.ts}`, `packages/bundle/`.
- **Aplicação no Cosca**: manifesto de skills/agents como linhas de dados empilháveis e patcháveis por camada — define "quais agents/skills/pipes montar e com que config" sem código de orquestração.

### A5. Catálogo→registro→montagem com health, single-flight e "preset nunca é alvo de persistência"
- **O que resolve**: descobrir/resolver/enumerar e **provar** que uma composição é utilizável antes de publicar; manter o catálogo vivo (arquivos como fonte).
- **Como funciona**: `discovery` não memoíza (relê roots); presets inválidos entram na lista com `broken` (não silenciados). `mount()` é ponto único single-flight e **prova o resultado** — rejeita linha nunca ativada, serviço vazado no realm root (`leakedServices`), alvo sem escopo. Composição tem `generation` (stamp mtime/size): sessões já juntadas mantêm a que entraram; edições criam nova geração. `write()` é no-op — preset é *input*, nunca persistência.
- **Onde**: `packages/preset/agent-presets/src/{discovery,preset,mount,session,index}.ts`.
- **Aplicação no Cosca**: catálogo de agents/skills/plugins por árvore de diretórios, com health-check que não esconde o quebrado, montagem single-flight por "preset" de orquestração e prova de prontidão antes do publish. Estado de sessão jamais volta pro arquivo.

---

## B. Sandbox & Isolamento de Runtime (o gap P0 do Cosca)

### B1. Sandbox por allow-list auto-restringido (self-restrict-then-exec)
- **O que resolve**: isolar um processo e todos os descendentes a um namespace de arquivos sem confinar o processo invocador — sem depender de user-namespaces ou mounts. **Falhando fechado** se o kernel não impuser.
- **Como funciona**: binário C11 (~300 linhas, static musl, só libc) sobre a UAPI crua do Landlock. Instala `PR_SET_NO_NEW_PRIVS` + ruleset em si mesmo e então `execvp` do comando; o ruleset é herdado via `execve`, confinando cada descendente. Banco de grants é allow-list (`--ro` grava read+execute; `--rw` grava FS completo; não-concedido é negado). Negociação de ABI (MAX_ABI=5) reduz ao que o kernel suporta; `ENOSYS`/`EOPNOTSUPP` ⇒ exit 125 sem executar. `--probe` constrói um ruleset máximo numa vida curta e imprime `fully/partially enforced` — a **sonda funcional** é a autoridade, não a versão do kernel.
- **Onde**: `native/landlock-run/packages/entry/src/main.c`, `.../docs/cli-contract.md`, `.../docs/architecture.md`.
- **Aplicação no Cosca**: mapeia direto ao P0. Landlock cobre só efeitos de filesystem — não substitui cgroups v2 (recursos) nem seccomp (syscalls). O padrão valioso é o **design**: launcher privilegiado minúsculo (auditável, estático) que se auto-restringe e `exec`s o alvo, com sonda funcional de enforcement, allow-list explícita, fail-closed. Para o seccomp do Cosca, a mesma geometria serve.

### B2. Cadeia de runners com sonda funcional + fact de enforcement honesto (full/partial)
- **O que resolve**: selecionar automaticamente o backend de confinamento disponível (em ordem de preferência), reportar com que completude governa, e **recusar rodar desconfiado** se nenhum é usável.
- **Como funciona**: `PLATFORM_CHAINS` mapeia plataforma→lista de runners (linux: `['bwrap','landlock']`, darwin: `['seatbelt']`, win32: `['windows-acl']`). Múltiplos candidatos arbitrados por sondas funcionais em ordem (sonda roda uma vez, timeout positivo validado). Seleção cacheadada na vida do provider. O fact `SandboxEnforcement = 'full'|'partial'` é **reportado, não prometido**. Se nenhum passa: lança `SandboxUnavailableError` — nunca passthrough silencioso. Modes são só FS: `read-only`/`workspace-write`/`danger-full-access`.
- **Onde**: `packages/sandbox/sandbox-local/src/index.ts`, `packages/sandbox/sandbox/src/index.ts`, `.../profiles.ts`, `docs/subsystems/sandbox.md`.
- **Aplicação no Cosca**: o formato exato para o gap cgroups v2 + seccomp. Modelar como cadeia de runners probed (`cgroups-v2 → seccomp → bwrap → landlock`), cada um reportando `full|partial`. Conservar a regra **nunca executar se o enforcement não foi provado** — isso distingue um harness de um sandbox de verdade. O `partial` precisa fluir até o chamador (carreado por chamada, não global).

### B3. Ciclo de vida de process-tree (grupo destacado + escalada SIGTERM→SIGKILL + quiescência observada)
- **O que resolve**: helpers de processo não sobrevivem ao handle; evitar re-sinalizar PID reusado; teardown espera a árvore inteira antes de declarar quiescência.
- **Como funciona**: `spawn` usa `detached: true` no POSIX; Windows termina via `taskkill /T /F`. `terminate()` é idempotente: SIGTERM → `graceMs` → SIGKILL, re-checando `treeAlive()` a cada estágio. `observeTreeExit()` poliniza liveness a cada 15ms; a primeira ausência confirmada seta `treeExitObserved` e cancela escalada pendente antes do group-id ser reusado. Identidade de processo é `{pid, started}` via start-time de `/proc/<pid>/stat` (Linux) ou `ps -axo lstart` (macOS) — anti-pid-reuse. Timers `ref`'d para o loop não morrer antes da quiescência.
- **Onde**: `packages/subprocess/subprocess-local/src/spawn.ts`, `.../process-inspector.ts`, `packages/subprocess/subprocess/src/types.ts`.
- **Aplicação no Cosca**: o núcleo para "rastrear e limitar processos filhos". O padrão de **grupo destacado + escalada com grace + espera de árvore inteira + identidade start-time anti-pid-reuse** é reutilizável como está e impede um subprocesso de virar orphan que continua executando código — exatamente o risco de segurança num sandbox.

### B4. Dialeto de argv envolto: distinguir "infra falhou" de "o sandbox bloqueou (funcionou)"
- **O que resolve**: quando um comando confinado termina com erro, atribuir a causa corretamente — (a) runner nunca executou (falha de infra ⇒ erro de sandbox, não de tarefa), ou (b) o confinamento funcionou e negou o efeito (comportamento correto). Exit code sozinho nunca prova falha do runner.
- **Como funciona**: `confine()` devolve um `ConfinedArgv` com dois classificadores ortogonais por backend: `denialSignatures` (dialeto de negação: `read-only file system` p/ bwrap, `permission denied` p/ Landlock, `operation not permitted` p/ Seatbelt) e `runnerFailureRules` (allowedExitCodes + fatalSignatures + exclusão de linhas informativas). `classifyRunnerFailure` checa primeiro; só depois `classifyDenial`. Negação atribuída por flag de enforcement **por processo** (não um last-wrap compartilhado).
- **Onde**: `packages/sandbox/sandbox/src/index.ts`, `packages/shell/bash-sandbox/src/helpers.ts`.
- **Aplicação no Cosca**: com cgroups v2 + seccomp, o Cosca herdará dialetos próprios (`OOMKilled`/`Killed` de cgroup, `EPERM` de seccomp). O padrão de **dialeto por-backend** (não união cross-backend) e a precedência runner-failure≻denial evitam que uma queda de infra do sandbox seja reportada ao LLM como "seu código foi bloqueado", e vice-versa.

### B5. Guard contra timeout de tool e loop de repetição (cooperativo, não veto)
- **O que resolve**: impedir execução descontrolada de uma tool (loop infinito) e timeout, sem abandonar a promise nem sufocar o modelo.
- **Como funciona**: `timeout-policy` envolve `tools/execute`: lê `timeoutMs` declarado, arma um deadline com `using` (limpa o timer no dispose), troca `exec.signal`, delega, e **restaura** o signal no `finally`. Só substitui o resultado por `{isError, error.code: TOOL_TIMEOUT}` quando o timer próprio disparou. `repeat-tool-reminder` canoniza args (deep-key-sort + JSON.stringify), conta runs consecutivos e, nos thresholds `[3,5,8]`, injeta lembrete gentil→detalhado como `additionalContexts` em `tools/post-execute` — **nunca veta nem reescreve a chamada**. Reset em interjeição do usuário.
- **Onde**: `packages/guard/timeout-policy/src/index.ts`, `packages/guard/repeat-tool-reminder/src/index.ts`.
- **Aplicação no Cosca**: complementa o sandbox — não basta confinar por FS/syscalls, é preciso limitar duração. A distinção **advisory vs veto** (lembrete sem bloquear) e o escopo por code de timeout (para não mascarar cancel a montante) são transferíveis.

### B6. Higiene de filho: scrub de ambiente + coleta com tail-keep em spill privado selado
- **O que resolve**: dois vazamentos — (a) credenciais do harness vazando para o processo não confiável, e (b) saída ilimitada (DoS por memória/arquivo) ou path-prediction de spill em tmp compartilhado.
- **Como funciona**: `scrubbedParentEnv()` remove, case-insensitive, qualquer nome casando `/KEY|PASSWORD|SECRET|TOKEN/i` e qualquer prefixo `DSH_` (namespace reservado), preservando PATH/HOME/locale. Um env explícito no spec é mesclado **depois** do scrub; `undefined` é tombstone que remove um entry. Na coleta, `OutputCollector` mantém **tail** em memória com cap e faz spill em arquivo privado `0700` (`mkdtemp`) com nome random + `O_EXCL` (`'wx'`, `0600`) — derrota symlink-planting. `seal()` para de anunciar o path se o close falhar; cap de spill descarta o arquivo.
- **Onde**: `packages/subprocess/subprocess/src/index.ts`, `packages/subprocess/subprocess-local/src/spawn.ts`.
- **Aplicação no Cosca**: controle barato e altamente transferível para qualquer execução de código de LLM. O scrub de ambiente (namespace próprio + regex de credencial + merge explícito depois) garante que tokens não vazem para um processo sandbox. O tail-keep capado + spill privado selado impede que um `run` que imprime GB vire DoS do harness.

---

## C. Agent-Loop, Sessão Durável e Contexto

### C1. Loop de agente como máquina de estados turn/step com parada pegajosa
- **O que resolve**: estruturar o driver para ser reiniciável, cancelável e reprodutível a partir do log de sessão, sem ambiguidade sobre quando um "turn" terminou.
- **Como funciona**: `ReactLoopAgent` sobe máquina `Phase = idle | maintenance | running`. Um `turn` abre com `turn/start` e executa 1..N `step`s; cada step fecha com `step/start`/`step/end`. O resultado do turn é um `TurnEndReason` **pegajoso**: uma vez que um step acerta `max-tokens` ou `error`, steps normais não rebaixam. Parada por convergência: o loop só retoma se `inbox.hasPending`, recriando o `AbortController` e zerando o step. Um único `AbortSignal` atravessa cada boundary; cancelamento pode ser latched via `wakeRequested` para replay pós-maintenance/abort.
- **Onde**: `packages/core/agent-loop/src/agent.ts`, `constants.ts`.
- **Aplicação no Cosca**: o loop deve ser um driver separado da sessão (sessão é o dado; driver é o orquestrador). A fase `idle/maintenance/running` + despertar latchado dá cancelamento/retomada limpos (crítico para memória durável com continuations). O `TurnEndReason` pegajoso evita que a condensação grave um turno interrompido como completo.

### C2. Log append-only como fonte de verdade + "surface" derivada com operações de replace
- **O que resolve**: guardar histórico completo no disco, mas permitir que o contexto efetivo (surface) seja um subconjunto colapsado — histórico e prompt falado divergem sem perda.
- **Como funciona**: o `Session` é um log de eventos imutáveis, seq-contíguo. A **surface** é a projeção renderizável dos nós; uma mensagem sintetizada (checkpoint, snapshot) entra como `user/message` com `surfaceOp: {op:'replace', start, end}` e `sourceEventSeqs` apontando o que foi sombreado. O log preserva tudo; a superfície colapsa. O `request/header` é uma série de snapshots endurecidos que se fundem offline (`foldRequestHeader` + `canonicalHeader`) reconstruindo o header vigente sem re-hidratar o log.
- **Onde**: `packages/core/session/src/request-header.ts`, `packages/session/session-persistence/src/coordinator.ts`, `packages/compaction/compaction-basic/src/region.ts`.
- **Aplicação no Cosca**: memória durável deve ser append-only por seq com a **projeção** (contexto atual visível ao modelo) derivada — nunca o inverso. A condensação é um `replace` de uma região da surface que mantém o histórico bruto em disco; recomputação de contexto = fold dos eventos de header, não replay do prompt inteiro.

### C3. Caminho de escrita durável: write-behind em lote + serialização por-id + reparo de crash
- **O que resolve**: persistir eventos de sessão com throughput e durabilidade, tolerando crash e escrita serializada por sessão.
- **Como funciona**: `PersistenceCoordinator` define contrato mínimo `PersistenceBackend` (loadStored, readStoredRevision, appendBatch, commitRepair, list). Toda operação por id passa por `serialize()` (promise-chain por sessão que impede intercalação). O `SessionWriteBehind` enfileira eventos clonados, agrupa por janela fixa (~200ms), mantém batch pendente em falha, e oferece barrier de `flush()`. Crash recovery detecta um **torn-tail** (marcador opaco) e sintetiza `interruptedTurnClosers` via `commitRepair`, truncando o rastro corrompido. Versionamento é recusa direcional (`sessionFormatVersionRefusal`: "upgrade" se o log é mais novo, "sem upgrade path" se é mais velho).
- **Onde**: `packages/session/session-persistence/src/{coordinator,write-behind}.ts`.
- **Aplicação no Cosca**: separar backend de armazenamento (SQLite/JSONL) da orquestração de escrita: batching por janela + promise-chain por sessão + reparo de fan-in (torn-tail→closers). Da escrita amortizada, quiescência testável (`flush`) e recuperação de crash sem corromper sessão — exatamente o que precisa para persistência de condensação.

### C4. Compactação como transação durável em bracket, com sumarização que re-usa KV cache
- **O que resolve**: encolher histórico para caber no contexto quando a pressão excede o orçamento, de forma atômica, idempotente e sem quebrar pares ferramenta.
- **Como funciona**: é uma transação em bracket `compaction/start → summarize → compactação → compaction/end` registrada no log. A seleção de alcance é **ancorada na cabeça** com cauda retida: conta `retainTokens` a partir do fim via `TokenMeter`, e recua a fronteira até não dividir um par tool-call/result. Dispara por pressão de tokens. A sumarização re-usa o provider: re-renderiza system prompt + tools + mensagens do alcance e acrescenta a instrução como último user message — um **prefixo genuíno** da request (anexa o warm prefix KV cache). Invariantes: rejeita se o resumo ≤ conteúdo sombreado, re-verifica a estabilidade da surface antes do commit, e `compaction/end` com `error` garante que um start órfão não fica pendurado.
- **Onde**: `packages/compaction/compaction-basic/src/{region,summarizer}.ts`, `packages/compaction/compaction/src/checkpoint.ts`.
- **Aplicação no Cosca**: a condensação de contexto é uma transação de log (bracket start/end), não um side-effect. Selecionar região por orçamento de tokens + não cortar pares + verificar que o sumário é menor + checar estabilidade da surface evita estados de memória inconsistentes. O re-uso do prefixo KV cache corta custo real de condensação.

### C5. Estado estruturado durável via projeções whole-value + CAS + registry
- **O que resolve**: manter estado de meta/todo/plano como dado estruturado (não texto solto), consistente com o log, consultável e versionável.
- **Como funciona**: a regra whole-value é load-bearing: o evento que carrega estado carrega o **estado completo pós-mudança**, nunca um delta. O `SessionProjectionRegistry` permite cada domínio registrar uma `ProjectionDefinition { key, stateSchema, init, apply, view }`; `apply` roda sincronamente a cada `session/event`, mantém célula por sessão e emite change-feed só quando a referência muda (`Object.is`). Persistência de projeção = linhas `(ver, seq, val)` em cache, com `restoreFloor` usando âncora um-abaixo do watermark e `stateVersion` invalidando linhas obsoletas. Goal modela fase `active|paused|blocked|complete` + `revision` (CAS), separando durável (`phase`) de process-local (`activation`, nunca persistida).
- **Onde**: `packages/goal/goal/src/{types,domain}.ts`, `packages/session/session-projection/src/index.ts`.
- **Aplicação no Cosca**: goal/todo/plan como projeções whole-value sobre o próprio log de memória (fold por chave, `apply` puro e síncrono), com CAS por `revision` e `stateVersion` para invalidação de cache. Permite aos agentes ler "o objetivo atual" sem re-platear texto.

### C6. Spill policy: contexto limitado via preview head/tail + artefato durável + locator
- **O que resolve**: manter resultados grandes fora do contexto do modelo sem perder os dados, com limite de bytes honesto e sem quebrar uma tool bem-sucedida.
- **Como funciona**: transformador `tools/post-execute`. Para um resultado de texto além de `maxInlineBytes`, salva o texto INTEIRO em `ctx.spillStore` (`SpillRef` com `locator` + `retrievalHint`) e substitui o que o modelo vê por um **preview head/tail** + notice. Orçamento respeitado: reserva primeiro os bytes do notice e divide o restante entre cabeça e cauda — invariante só emite ≤ cap (implica "menor que o original"). Falha de armazenamento é best-effort fail-open (mantém inline e loga, nunca `isError`). Evita loop `read → spill → read again` e aplica o mesmo cap à cópia no log durável.
- **Onde**: `packages/spill/spill-policy/src/index.ts`.
- **Aplicação no Cosca**: um "memory/context budget manager": o que estoura o orçamento de contexto vai para artefato durável (com locator recuperável), mantendo no prompt preview + ponteiro. O fail-open + skip-read + orçamento reservado do notice são as guardas para não transformar extração de memória em falha do agente.

---

## D. Skills, Subagentes e Modelo de Permissão

### D1. Registry de provedores em camadas com carregamento lazy/abortável
- **O que resolve**: descobrir, resolver por nome e carregar skills de fontes heterogêneas (projeto, usuário, bundled, custom) sem acoplar o contrato a nenhuma origem, e sem I/O longo dentro do loop.
- **Como funciona**: a seam separa **Service Definition** (registry) de **Providers** (onde as skills moram). Um provider implementa `SkillProvider { name, list(options), get(candidate, options) }`: `list()` devolve candidatos ou uma observação `{ candidates, complete }`; `get()` recebe de volta um locator opaco. O registry não conhece disco: **merge em camadas** (global + cadeia de escopos do agente; a mais próxima vence nome duplicado, `rank` desempata só internamente à camada), **controle de invocação** `{ modelInvocable, userInvocable }` como metadado prompt-visível, **validação fail-loud** de tudo que um provider retorna, **cache com revisão + abort** (`collect()` cacheadapor `(cwd, escopo, revision)`; observação `complete:false` nunca é cacheada — o consumidor guarda o último estado bom e retenta), e **renderização canônica** `<skill_content>` escapada compartilhada.
- **Onde**: `packages/skill/skill/src/index.ts`, `packages/skill/skill-filesystem/src/index.ts`.
- **Aplicação no Cosca**: mapeia direto ao inventário de 71 skills. Expor `ctx.skills` como registry multi-provider (fs, built-in, remoto) com descoberta por projeto→usuário→bundled, ramo de escopo por agente, e invocação no frontmatter. O padrão "provider registra-se devolvendo um disposer" + "observação incomplete não-cacheada com retry" resolve o problema de skills que aparecem/desaparecem em hot-reload.

### D2. Catálogo + loader como face de modelo com digest durável do estado publicado
- **O que resolve**: anunciar skills ao modelo e injetar o corpo no momento certo, sem reparsear prosa, sem republicar catálogo inalterado e sem "skills-fantasma".
- **Como funciona**: todo o catálogo é uma mensagem `catalog`-form com `entries` duráveis junto da prosa `<available_skills>` (o consumidor usa `entries`, não o XML). A decisão de republicar é um **digest sha256 sobre as entries** (não sobre a prosa): se o digest é o mesmo, nenhuma republicação; se mudou, a mensagem antiga é substituída por uma de `update: true`. A visibilidade da tool governa se o catálogo é emitido. O caminho `/name` varre apenas mensagens `source.kind === 'user'`, resolve no registry e injeta o corpo como `instructions` — única entrada para skills com `disable-model-invocation`.
- **Onde**: `packages/skill/tool-skill/src/index.ts`.
- **Aplicação no Cosca**: o padrão para expor skills sem inchar o prompt. Um "agente Cosca" que publica catálogo de skills (digest sobre IDs+descrições, não sobre o texto renderizado) evita re-injetar instruções a cada turno e substitui automaticamente quando um skill muda. O gesto `/nome` user-only é um gancho fácil de replicar no CLI do Cosca.

### D3. Subagente como seam de providers com capacidades "fail-loud" e política de delegação travada
- **O que resolve**: delegar trabalho a filhos (one-shot ou continuáveis) por múltiplos backends, garantindo que um pedido que exige algo que o backend não suporta seja **rejeitado e nunca silenciosamente degradado**, e que o escopo de permissão do filho fique fixo/reconstruível.
- **Como funciona**: `ctx.subagents` é um registry **multi-provider** (spawn/fork/acp/codex/claude), cada um com `capabilities: { outputSchema, depthLimit, toolFilter, persona }`. O service checa capacidades em `start()` (sem essa flag → `UNSUPPORTED_CAPABILITY`). Para continuáveis, a **presença de `prepareContinuable()` é a própria capability**. O ponto mais transferível é o **seed de política de delegação**: `captureDelegatedPolicyOverrides` congela o override de sandbox do pai e trava `approval` para `'never'`; escrito como eventos `source: 'delegation'` no log do filho, então a política efetiva é reconstituível do log.
- **Onde**: `packages/subagent/subagent/src/index.ts`, `.../types.ts`, `.../child-agent.ts`, `.../tool-subagent/src/index.ts`.
- **Aplicação no Cosca**: os specialists/agents Cosca funcionam como providers nomeados de subagente. `capabilities`-checking evita que um specialist que não sabe toolFilter/persona receba um pedido que apenas ignora; a política travada de delegação resolve o medo clássico de "filho escalar privilégio" — pinar approval a `never` e fixar sandbox, gravando no session log, dá trilha de auditoria. O padrão continuável (filho com inbox próprio + cold-resume) viabiliza "background specialists" sem perder contexto.

### D4. Knobs ortogonais de permissão empacotados em presets + waterfall de aprovação fail-closed
- **O que resolve**: controlar ações perigosas de forma legível ao humano e determinística em automação, mantendo a decisão auditável e reconstituível após resume.
- **Como funciona**: há dois knobs independentes — `sandbox/mode` (read-only / workspace-write / danger-full-access) e `approval/policy` (`'ask'` | `'never'`). A **política de aprovação** é um fold do log: `effectiveApprovalPolicy` varre de trás pra frente o último `approval/policy`, ou seja "replayar o log é o estado". Criticamente, o `'never'` é decidido **no caminho do service** (não em listener), para que um listener `prepend:true` tardio não possa quebrar a promessa de rejeição determinística. O waterfall `approval/request` é **fail-closed**: sem answerer → `'unavailable'`; answerer que lança → `'unavailable'`; valor fora do vocabulário → normalizado `'unavailable'`; abort → `'cancelled'`. Todo pedido é auditado como par `approval/asked` + `approval/decided`, ambos obrigatoriamente turn-enclosed.
- **Onde**: `packages/interaction/user-approval/src/index.ts`, `packages/interaction/permission-presets/src/index.ts`.
- **Aplicação no Cosca**: o Cosca precisa de um modelo de permissão para os specialists; mapear como **knobs ortogonais** (escopo de FS/exec + política de confirmação) em vez de uma lista binária "allowed/denied" dá presets reutilizáveis (ex.: "only-workspace+ask") com equivalência ao `danger-full-access+never` para automação CI. O fail-closed em waterfall (sem UI → nega) e o log-fold como estado resolvem headless/headful e resumo de sessão sem código extra.

### D5. Seam de credencial por referência (não-valor) com resolução em camadas confiáveis
- **O que resolve**: transportar segredos de config sem nunca expor o valor, e garantir que mudanças cheguem sem reiniciar e que um "branco" nunca pareça um segredo configurado.
- **Como funciona**: Config/settings carregam `CredentialRef` (nome de variável, gramática POSIX) — **referências, nunca segredos**. `resolve()` é por operação (cada chamada releciona, então a troca de credencial alcança a próxima operação sem restart). Para config-UIs há `describe()`/`describeRecord()` que devolvem `{ configured, source, writable }` sem o valor. Há dois espaços de chave: **Referências** (camadas confiáveis: processo `env` read-only → store gerenciado → `.env` do projeto → `.env` do usuário; **valor vazio = ausente em toda parte**) e **Registros** (`scope/id`): sem camadas, presença é o fato, `modifyRecord()` é o único caminho de escrita (read-decide-replace sob lock). A escrita é rejeitada quando uma camada read-only superior sombreia a referência.
- **Onde**: `packages/credentials/credentials/src/index.ts`, `packages/credentials/credentials-local/src/index.ts`.
- **Aplicação no Cosca**: todas as chaves de API e credenciais de providers/plugins Cosca deveriam ser `CredentialRef` (nomes de env) resolvidas na borda da operação, nunca gravadas em config. O `describe()` sem valor permite painéis/CLI de config sem vazar secret; o "referência read-only que bloqueia escrita" impede o bug clássico de escrever num `.env` sobreposto por um env do processo. O `modifyRecord` sob lock é o modelo para refresh de token em nuvem.

### D6. Catálogo colhido por boot executável + custódia de completude + gate de frescor
- **O que resolve**: gerar/verificar um catálogo de tools/skills que seja **fiel** ao runtime e onde **um item novo nunca seja silenciosamente não-documentado**.
- **Como funciona**: em vez de parsear AST estática, `gen-tool-catalog.ts` **boota** cada pacote `tool-*` num `Context` real (mount do SystemPrompt + ToolRuntime + o mount do pacote) e lê `ctx.tools.schemas()` — porque um schema de tool não é estaticamente conhecível. Dois guardas de completude: `assertManifestComplete` globs `packages/*/tool-*` e **falha** se algum pacote não está no boot manifest (`TOOL_PACKAGES`); `assertToolsHarvested` falha se um pacote bootou sem registrar tool alguma (pega plugin PENDING em `inject` insatisfeito). O manifest é a fonte de verdade. A renderização é pura/determinística, e a CLI tem `--check` que compara o arquivo committed com o gerado e aponta a primeira linha divergente (gate de frescor no CI).
- **Onde**: `scripts/gen-tool-catalog.ts`, `scripts/verify-skill-invocation-metadata.ts`.
- **Aplicação no Cosca**: com 71 skills e dezenas de agents/specialists, o mesmo princípio: um catálogo de tools/skills **gerado bootando os plugins** (não por leitura de AST), com manifest obrigatório que cobre tudo que existe em disco e um `--check` no CI. Isso dá "regressão de contrato": qualquer specialist novo precisa ser declarado no manifest ou o gate quebra — exatamente o que impede docs/descoberta de ficarem obsoletos.

---

## E. Protocolos, Integração e Disciplina de Gates

### E1. Contrato de fio: mapa tipado único que server e client compilam contra
- **O que resolve**: o drift entre a declaração de protocolo, a implementação do servidor e os consumidores do cliente — sem codegen de mensagens no runtime.
- **Como funciona**: `packages/sdk/protocol/src/types.ts` define dois mapas nomeados: `HarnessSdkRequestMap` (`method → { params, result }`) e `HarnessSdkNotificationMap` (`method → payload`). O server e o client importam esses tipos; o transport é uma máquina de estado genérica (JSON-RPC 2.0 NDJSON sobre streams) que **não conhece** os métodos — apenas roteia `id+method` → request, `id` → response, `method` → notification. Erros de fio são uma classe tipada. Como `RequestMap`/`NotificationMap` são tipos puros, a integridade do contrato é imposta pelo typecheck: mudar um método em um lado e esquecer o outro falha em `typecheck`.
- **Onde**: `packages/sdk/protocol/src/{types,transport,index}.ts`, `packages/sdk/server/src/server.ts`, `packages/sdk/client/src/client.ts`.
- **Aplicação no Cosca**: definir um `protocol/types.ts` como única fonte de verdade do fio (mapas de request + notification), com `transport.ts` genérico e fino reutilizado por server e client. Os clientes `cosca-api` (TS/Python) compilam contra o mesmo mapa → a disciplina de typecheck substitui testes de contrato manuais.

### E2. Validação de fronteira JSON: o "range-check" do fio
- **O que resolve**: valores que passam pelo JS mas quebram no JSON (undefined, NaN, ciclos, símbolos, arrays esparsos/decorados, objetos não-plain) — sem um guard, vira bug silencioso de serialização.
- **Como funciona**: `packages/api/gateway/src/index.ts` tem `assertJsonValue(value, ancestors)` que valida recursivamente: `number` finito, sem símbolos, sem prop `Symbol`, só data-props enumeráveis, array sem buracos, sem ciclos (via set de ancestrais), só plain-object ou primitivo. É aplicado em TODA entrada (`decode` → `input-invalid`) e TODA saída (`result-invalid`) de um request no gateway. Erros de fronteira viram `TypertGatewayError` com código estável e `field` nomeado, nunca um throw vazando do JSON.stringify.
- **Onde**: `packages/api/gateway/src/index.ts`, `packages/sdk/protocol/src/transport.ts`, `packages/sdk/client/src/client.ts`.
- **Aplicação no Cosca**: um módulo `json/gates.ts` (finito + plain-object + sem ciclos + sem símbolos + array denso) aplicado em cada request/response de qualquer bridge RPC do Cosca (gateway, ACP, MCP). Centraliza a disciplina "nada de valores não-JSON atravessando o fio", hoje dispersa entre gateways.

### E3. Gateway de roteamento com descritores "strict" + fallback SRC e indireção por provider
- **O que resolve**: rotear uma chamada externa (`namespace/method` + args nomeados) para um serviço in-process, resolvendo parâmetros opacos (contexto, IDs de lookup) sem que o chamador conheça os serviços — forma um RPC reverso (Remote) em cima de serviços Cordis.
- **Como funciona**: o gateway intercepta `/api` e usa `claimsEndpoint(endpoint)` para decidir se ele "possui" o endpoint: primeiro o registro `ctx.typert.local` (descritor strict gerado), depois um fallback SRC que varre serviços que carregam um binding refletido (marker `{ namespace }` + `remoteMethods`). Cada chamada: resolve descritor, `assertExactArguments` (rejeita args extras/faltantes), resolve um **receiver Context** (via `typert.contexts` provider, identidade → Context) e resolve **parâmetros lookup** (via `typert.lookups` provider). Erros de ambiguidade, mismatch strict/SRC e complexidade de assinatura são códigos estáveis. O binding é validado para impedir serviço "impostor".
- **Onde**: `packages/api/gateway/src/{index,types}.ts`, `packages/api/remotes/src/*`, `packages/typert/*`.
- **Aplicação no Cosca**: para os agentes `cosca-*` exporem/cosumam capacidades via bridge, usar um namespace RPC com declaração de binding no serviço (`@Remote` marker), indireção por provider para identidades (sessionId → Context de sessão, agentId → agente), e uma tabela de códigos de erro estável no fio. Dá ao Cosca um modelo Remote coerente em vez de RPCs ad-hoc por agente.

### E4. Bridges de dialeto sobre um resultado neutro: ACP/MCP/hooks
- **O que resolve**: interoperar com protocolos externos sem vazar a semântica de cada um para o núcleo — traduzindo idiomas estrangeiros para pontos de extensão tipados internos.
- **Como funciona**: o `packages/hooks/hook-protocol/src/types.ts` define um `HookOutput` **dialeto-neutro** (exitCode, stdout, stderr, decision, reason, continue, additionalContext…) e um `MatcherGroup` comum. Cada bridge é dono só do que é seu: `hooks-claude-code` parseia `hooks.json` do CC (com substituição de variáveis) e constrói payloads mapeando `HookOutput` → decisões tipadas nos extension points `tools/pre-execute`, `agent/pre-step`, `agent/turn-stopping`. Execução/merge/parse ficam no protocolo. O ACP espelha isso: expõe sessões novas como um server JSON-RPC stdio via `AgentSideConnection`, converte `prompt` ↔ sessão, `assistant/message` ↔ `agent_message_chunk`, e a waterfall de aprovação ↔ `requestPermission`. O MCP traduz do outro lado: `mcp__<server>__<tool>` registrado em `ctx.tools`, com transport stdio/streamable-http e scrub de env.
- **Onde**: `packages/hooks/hook-protocol/src/*.ts`, `packages/hooks/hooks-claude-code/src/*.ts`, `packages/acp/acp/src/index.ts`, `packages/mcp/mcp-client/src/{index,transport,connection,tools}.ts`.
- **Aplicação no Cosca**: para cada protocolo externo (ACP/Codex/Claude/API REST), criar um pacote-bridge fino que traduz para o "talk-spine" interno do Cosca (extension points + tipos neutros), deixando config/parse/payload no bridge e execução/semântica compartilhada numa lib de protocolo. É o antídoto contra vazar semântica de fornecedor para os agentes.

### E5. Disciplina "generate-and-diff": o artefato gerado É o invariante
- **O que resolve**: manter dezenas de invariantes estruturais sem escrever uma asserção frágil por invariante — a intenção "viva" é re-derivada do programa e comparada (diff) contra o artefato commitado.
- **Como funciona**: há um par `gen-*` / `verify-*` para cada catálogo. Ex.: `gen-scoped-events.ts` escaneia o `ts.Program`, acha cada evento que declara `this: Scoped<Base>` e converte em um resolver gerado — rejeitando ambiguidades e exigindo JSDoc (`@dshScopeScan`). O mesmo script roda em `--check` e **exit 1 se o artefato commitado estiver stale** ("run `pnpm run gen-scoped-events` and commit it"). Outros: `gen-tool-catalog`, `gen-config-catalog`, `gen-persistence-catalog`, `gen-module-graph`, `gen-doc-graphs`, e verificações "puras" como `verify-config-source-ownership` (regex denegrida para inline de credencial/endpoint em configs shipped) e `verify-package-invariants`. O orquestrador é `scripts/run-gates.ts`: compõe agregados nomeados (`ci-primary`, `ci-static`, `hygiene`, `doc-sync`…) como um **DAG de gates** (cada gate tem `id`, `needs`/`after`, `allowFailure`), valida o grafo (ciclos, deps desconhecidas) antes de rodar, e executa com `maxConcurrency` limitado por CPU.
- **Onde**: `scripts/{gen-*,verify-*,run-gates,ts-project,jsdoc}.ts`, `package.json`.
- **Aplicação no Cosca**: é o mais transferível para a cultura de `QUALITY_GATES`. Adotar o padrão: cada invariante de catálogo do Cosca = `gen-x` que re-deriva a intenção do código e escreve um artefato commitado; um `verify-x`/`--check` que faz o diff em CI (o snapshot é o "contrato", e qualquer drift = gate vermelho). Envolver tudo em um `run-gates.ts` com DAG validado (sem ciclos, deps checadas, `allowFailure`, cap de concorrência), para os agentes cosca evoluírem a rede de invariantes de forma incremental e segura no CI.

### E6. Supervisão de subprocesso: ladder de teardown + fila de notificações com filtro
- **O que resolve**: tornar tolerante a falhas e determinístico o ciclo de vida de um subprocesso de protocolo — morte, timeout, close e despejo de notificações out-of-order, sem leaks.
- **Como funciona**: `packages/sdk/client/src/client.ts` faz spawn direto, e no close aplica o ladder compartilhado `disposeRuntimeProcess` (stdin-EOF → SIGTERM → SIGKILL em graces configuráveis). Notificações do server são roteadas para **subscriptions** (`NotificationSubscription` com fila + waiters + `failure`); um filtro/predicado é a única coisa que separa as subs. Para árvores de subagentes, monta um mapa `sessionParents` (de `subagent.started`) e faz lineage walk (`subscribeSessionTree`) para escopar notificações client-side. Erros ganham contexto de processo (exit code + stderr tail de até 400 linhas), e `closedError` concatena tudo. Timeouts usam `AbortController` como abandono (transporte descarta o pending, sem estado retido para resposta que pode nunca chegar).
- **Onde**: `packages/sdk/client/src/{client,dispose}.ts`, `packages/sdk/protocol/src/transport.ts`, `packages/mcp/mcp-client/src/connection.ts`.
- **Aplicação no Cosca**: para qualquer bridge que spawne subprocesso (hosts MCP/Codex, runtimes), centralizar um "process supervisor" com: ladder determinístico de despejo, tail de stderr em toda falha, subscription com fila+waiter e abort por signal, e escopo de eventos por lineage. Torna o comportamento de falha testável, em vez de espalhar `kill()`/timeouts ad-hoc por agente.

---

## Synthesis — o que o Cosca deveria copiar (priorizado)

| # | Padrão | Aplicação no Cosca | Ganho |
|---|--------|-------------------|-------|
| 1 | B2 — Cadeia de runners probed + enforcement `full/partial` + fail-closed | fechar o P0 de sandbox (cgroups v2 + seccomp) com prova real | sandbox de verdade, não promessa |
| 2 | B3 — Process-tree (grupo destacado + escalada + anti-pid-reuse) | teardown confiável de code-runnings | nenhum subprocesso órfão/executando solto |
| 3 | B1 — Self-restrict-then-exec + sonda funcional | runner de sandbox auditável e estático | enforcement imposto pelo kernel |
| 4 | E5 — Generate-and-diff (`gen-*`/`verify-*` + `run-gates` DAG) | rede de invariantes no CI | docs/catálogo nunca ficam obsoletos |
| 5 | D1/D2 — Skill registry multi-camada + digest de catálogo | 71 skills expostas sem inchar prompt | hot-reload e sem skills-fantasma |
| 6 | D3/D4 — Subagente com capacidades + política travada + approval fail-closed | specialists sem escalar privilégio | trilha de auditoria de delegação |
| 7 | C2/C4 — Log append-only + surface derivada + compactação em bracket | memória durável consistente | condensação atômica e idempotente |
| 8 | E1—Contrato de fio tipado (RequestMap/NotificationMap) | fio RPC único e typechecked | drift de protocolo eliminado |
| 9 | A3 — Realms + cadeia de escopos por agente | composição por sessão sem colisão | um binário, muitas visões |
| 10 | B6/B4 — Scrub de env + dialeto de negação por-backend | execução de código de LLM segura | sem vazar token; erro atribuído certo |

## Known Uses (referência)

- `deepseek-ai/deepseek-harness` — harness de agente "tudo é plugin" (dev preview, breaking). Plugin ecosystem `dsh-plugin`.

## Related Patterns

- [`hermes-agent-patterns.md`](hermes-agent-patterns.md) — auto-evolução + guardrails (B5 aqui complementa o veto/advisory)
- [`kubernetes-core-patterns.md`](kubernetes-core-patterns.md) — workqueue/reconciler (loop e estado difere)
- [`backstage-plugin-platform-patterns.md`](backstage-plugin-platform-patterns.md) — plugin registry/extension points (paralelo a Cordis)
- `internal/evals/ORACLE_SPEC.md` — o oráculo que serve de fitness
