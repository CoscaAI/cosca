# Kubernetes Org Patterns — Sandbox (CRI), Scaling (VPA/CA), Governança (KEP/SIG), Metrics

> **Version**: 1.0.0 | **Confidence**: 0.90 | **Category**: Enterprise Platform Patterns | **Created**: 2026-08-22 | **Source**: https://github.com/kubernetes

> **Mined by**: cosca-kernel (ordem do Don). Extraído de `kubernetes/cri-api`, `kubernetes/autoscaler`, `kubernetes/community`, `kubernetes/kube-state-metrics`. **Complementa** o `kubernetes-core-patterns.md` (que já cobre o core: informer, workqueue, reconcile, admission, etc.) — aqui são os satélites que casam com os gaps do Cosca: **sandbox (P0), scaling de capacidade, governança e observabilidade de estado**.

## Purpose

A org do Kubernetes é rica em padrões de **runtime isolado e plugável (CRI)**, **escalonamento de capacidade com anti-thrash (VPA/CA)**, **governança por propostas/ownership (KEP/SIG)** e **observabilidade derivada de estado (kube-state-metrics)**. O Cosca tem o core do Kubernertes minerado; estes 4 satélites fecham os gaps de execução P0, capacity, governança e métricas.

---

## A. Sandbox & Runtime (CRI) — o gap P0 de execução do Cosca

### A1. Contrato de runtime plugável via gRPC com duas services ortogonais
- **O que resolve**: acoplar o orquestrador (kubelet) ao executor (containerd/CRI-O) sem recompilar; trocar o runtime sem tocar no código de orquestração.
- **Como funciona**: contrato proto3 + gRPC. `RuntimeService` (lifecycle de sandbox/container/exec) e `ImageService` (push/pull) separadas. `api_grpc.pb.go` gera `RuntimeServiceClient` (via `NewRuntimeServiceClient(cc grpc.ClientConnInterface)`) para o kubelet e `RuntimeServiceServer` + `RegisterRuntimeServiceServer` para o runtime. O pacote descompõe `RuntimeService` em sub-interfaces (`RuntimeVersioner`, `ContainerManager`, `PodSandboxManager`, `ContainerStatsManager`) injetáveis e testáveis à parte.
- **Onde**: `pkg/apis/runtime/v1/api.proto` (24-266), `pkg/apis/services.go`, `pkg/apis/testing/fake_runtime_service.go`.
- **Aplicação no Cosca**: blueprint para isolar "coordenação" (kernel/orquestrador) de "execução" (worker/runner). Definir uma `CoscaRuntimeService` de execução + `CoscaImageService` separadas, com client/server gRPC; o sandbox executor vira binário independente e o orquestrador consome só o client. Permite hot-swap de runtime (gVisor, Firecracker, process runner).

### A2. Sandbox como objeto de 1ª classe com lifecycle idempotente
- **O que resolve**: dar ao "pod/sandbox" um ciclo de vida explícito, isolado por namespaces e responsável por recursos, em vez de tratá-lo como container comum.
- **Como funciona**: `RunPodSandbox` (create+start, retorna ID), `StopPodSandbox` (mata processos, devolve IP/rede, **idempotente**, só falha se não removido), `RemovePodSandbox` (força remoção), `PodSandboxStatus`/`ListPodSandbox`. O sandbox concentra isolamento: `NamespaceOption` (network/pid/ipc com `NamespaceMode` POD|CONTAINER|NODE|TARGET + `UserNamespace`), `LinuxPodSandboxConfig` (`cgroup_parent`, `sysctls`, `overhead`/`resources`), `DNSConfig`, `PortMapping`. Metadata deixa o sandbox endereçável.
- **Onde**: `api.proto` (28-48, 561-733, `NamespaceMode` 397-417, `NamespaceOption` 433-457).
- **Aplicação no Cosca**: modelar o sandbox de execução como unidade de isolamento autônoma, com `RunSandbox`/`StopSandbox`/`RemoveSandbox` idempotentes. O `NamespaceMode` (POD/CONTAINER/NODE) e o `cgroup_parent`+`sysctls` são o mecanismo para garantir isolamento de rede/PID/IPC e controle de recursos por sandbox.

### A3. Negociação de contrato + handshake de features (versionamento e feature gates)
- **O que resolve**: orquestrador e runtime evoluírem separados, com compatibilidade negociada em runtime, sem quebra de versão.
- **Como funciona**: `Version(VersionRequest)` retorna `runtime_name`, `runtime_version`, `runtime_api_version`. `Status()` + `RuntimeConfig()` retornam `runtime_handlers` (`RuntimeHandlerFeatures`), `features` (`RuntimeFeatures`) e `cgroup_driver`. O kubelet lê essas flags e desliga features não suportadas (feature gates). `runtime_handler` em cada request seleciona entre runtimes nomeados.
- **Onde**: `api.proto` (`Version` 26, `RuntimeHandler`/`RuntimeFeatures` 1872-1904, `CgroupDriver` 2380).
- **Aplicação no Cosca**: implementar `Version`/`Status`/`RuntimeConfig` como handshake inicial. É o mecanismo para o Cosca declarar quais capacidades de sandbox o runtime atual suporta (userns, mounts read-only, security profiles) e o orquestrador desabilitar features dinamicamente — em vez de hard-codar suposições. `runtime_handler` vira o multi-backend do Cosca.

### A4. Spec declarativa de recursos, mounts e security context
- **O que resolve**: expressar isolamento/montagens/segurança como dados que o runtime aplica, mantendo o orquestrador puramente declarativo.
- **Como funciona**: `LinuxContainerResources` espelha cgroups v1/v2 (cpu_period/quota/shares, memory_limit, oom_score_adj, cpuset_cpus/mems, `unified` para v2), lógico e portável. `Mount` cobre `host_path` OU `image`, `readonly`, `propagation`, `uidMappings`, `recursive_read_only`, `mount_options`. `LinuxContainerSecurityContext` amarra capabilities (add/drop/ambient), `privileged`, seccomp/apparmor (`SecurityProfile`), SELinux, `run_as_user`, `readonly_rootfs`, `no_new_privs`, `masked_paths`/`readonly_paths`.
- **Onde**: `api.proto` (`LinuxContainerResources` 982-1005, `Mount` 335-382, `LinuxContainerSecurityContext` 1044-1118).
- **Aplicação no Cosca**: define o "manifesto de isolamento" por sandbox. Como o Cosca roda código não-confiável, este é o vocabulário para especificar limites de CPU/RAM/swap, filesystem read-only, mounts idmap, e seccomp/AppArmor/capabilities em **dados**, não em código. O executor interpreta o spec e traduz para o backend escolhido.

### A5. Modelo de execução em dois níveis: ExecSync (síncrono/bounded) vs Exec/Attach (streaming interativo)
- **O que resolve**: separar "rodar e capturar" de "interagir ao vivo" num runtime isolado — casos de uso fundamentalmente diferentes.
- **Como funciona**: `ExecSync` executa e devolve de uma vez: `cmd`, `timeout`, captura `stdout`/`stderr` com **teto de 16MB** (defesa contra CVE) e `exit_code`. `Exec`/`Attach`/`PortForward` retornam apenas `url` (endereço de um **servidor de streaming separado**), com flags `stdin`/`stdout`/`stderr`/`tty` (regras explícitas; se `tty`, `stderr` deve ser false). O contrato não carrega bytes.
- **Onde**: `api.proto` (`ExecSync` 109, `ExecSyncRequest/Response` 1614-1638, `Exec`/`Attach` 111-115, 1640-1701).
- **Aplicação no Cosca**: `ExecSync` (com cap de 16MB e timeout) para tarefas de agente/ferramenta dentro do sandbox; `Exec`/`Attach` (URL + stream) para sessões interativas/REPL/port-forward. Os guards de `tty`/`stderr` e o teto de output são as superfícies de segurança que o sandbox deve herdar para evitar OOM/abuso de memória.

### A6. Streaming RPC para listagens e stats (evita limites de mensagem gRPC)
- **O que resolve**: listas enormes estouram o limite de 4MB de mensagem do gRPC.
- **Como funciona**: para cada `List*` existe um `Stream*` server-streaming, gated por `CRIListStreaming`: `StreamPodSandboxes`, `StreamContainers`, `StreamContainerStats`, `StreamImages`. Cada item aparece exatamente numa resposta, servidor fecha com EOF, kubelet impõe timeout no stream inteiro.
- **Onde**: `api.proto` (`StreamPodSandboxes` 63, `StreamContainers` 94, etc; feature gate `CRIListStreaming`).
- **Aplicação no Cosca**: qualquer sandbox que agregue muitos workers/jobs deve expor alternativas streaming. "Nunca liste em uma mensagem; streame em lotes com timeout global" é a diretriz de robustez para o painel/monitoramento a grande escala.

### A7. Checkpoint/Restore de pod como snapshot consistente (deadline-bound)
- **O que resolve**: migrar/retomar um sandbox inteiro de forma atômica e consistente, sem estado congelado.
- **Como funciona**: `CheckpointPod` usa deadline no contexto, pausa todos os containers antes de capturar, mantém pausados até capturados, e resume antes de retornar — "cut" consistente pod-wide. `RestorePod` cria sandbox+containers em **CREATED** sem executar o processo restaurado; caller roda pre-start hooks e chama `StartContainer`.
- **Onde**: `api.proto` (`CheckpointPod` 175, `RestorePod` 183, `RestoredContainer` 2247).
- **Aplicação no Cosca**: padrão definitivo para snapshot/quiescing de sandbox: capturar consistente com pausar→capturar→resumir, deadline no contexto, restauração para `CREATED` + `StartContainer`. Evita estados corrompidos de sandbox em checkpoint.

---

## B. Scaling & Capacidade (autoscaler) — capacity do Cosca

### B1. Decaying Histogram (signal store time-weighted)
- **O que resolve**: agregar uso/métricas de longo prazo em memória sem armazenar a série temporal inteira, com peso decrescente para amostras antigas — a recomendação reflete a carga *recente* sem esquecer picos sazonais.
- **Como funciona**: histograma de buckets guarda peso acumulado por bucket + `totalWeight`. Cada `AddSample` multiplica o peso por `2^((ts - referenceTimestamp)/halfLife)` — amostra perde metade da importância a cada `halfLife`. Como só pesos relativos importam, reescala quando o expoente cresce. `Percentile(p)` retorna o fim do bucket onde a soma acumulada cruza `p * totalWeight`. Checkpoints guardam `map[bucket]uint32` + totalWeight.
- **Onde**: `vertical-pod-autoscaler/pkg/recommender/util/decaying_histogram.go`.
- **Aplicação no Cosca**: primitiva para qualquer contador de capacidade/uso que precise de "janela deslizante" barata — `O(1)` de atualização + `O(buckets)` de leitura, meia-vida configurável por métrica.

### B2. Estimador em banda: target / lower / upper com margem de segurança e confidence-gating
- **O que resolve**: emitir uma região [lower, upper] em torno de um target, onde o updater só age se o pod sair da banda — mecanismo anti-thrash que evita flapping (o equivalente ao in-stability window do HPA).
- **Como funciona**: cada recurso tem 3 estimadores por percentile (`Target=0.9`, `Lower=0.5`, `Upper=0.95`), embrulhados por decorators em cascata: `percentile → +margin(SafetyMarginFraction) → x confidenceMultiplier → max(minResource)`. `confidenceMultiplier` escala por `(1 + k/confidence)^exponent`, onde `confidence = min(dias de história, nº amostras normalizado)`. Histórico curto ⇒ upper * INF (não force eviction), lower * 0; histórico longo ⇒ multiplicadores → 1.
- **Onde**: `vertical-pod-autoscaler/pkg/recommender/logic/estimator.go`.
- **Aplicação no Cosca**: para cada dimensão de capacidade, emitir `{target, lower, upper}` e só acionar mudança fora da banda. Usar a confiança (nº de observações) para alargar a banda — nunca mexer em alvos com poucos dados.

### B3. Semânticas de sinal diferentes por recurso: CPU=percentil, memória=pico-por-janela
- **O que resolve**: evitar recomendar errado tratando recursos como a mesma coisa — CPU é compressível (percentil importa), memória é incompressível (o *pico* é o risco de OOM).
- **Como funciona**: `AggregateContainerState` mantém dois histogramas distintos: `AggregateCPUUsage` (todo sample para `Percentile(p)`) vs `AggregateMemoryPeaks` (*um pico por janela*, `MemoryAggregationIntervalDuration`, × count). Cada container mantém `memoryPeak`/`oomPeak` que avançam a cada janela; só o pico da janela entra no histograma.
- **Onde**: `vertical-pod-autoscaler/pkg/recommender/model/aggregate_container_state.go`, `model/container.go`.
- **Aplicação no Cosca**: manter streams de sinal com agregação própria — "percentil" para dimensões elásticas, "máximo por janela" para dimensões rígidas (vazão de pico, memória, latência p99). Nunca misturar; cada eixo define sua janela.

### B4. Feedback por evento de falha: OOM vira amostra sintética + bump-up
- **O que resolve**: fazer o sistema "aprender" com um evento discreto de falha, garantindo que um pico que causou OOM empurre a recomendação para cima imediatamente.
- **Como funciona**: `oom.Observer` escuta eviction/OOMKilled e injeta `OomInfo`. `RecordOOM` descarta OOMs antigos e cria amostra artificial de memória = `max(requested, memoryPeak)`, então `max(x + OOMMinBumpUp(100Mi), x * OOMBumpUpRatio(1.2))`. `oomPeak` é omitido em recomputações para não subir demais em OOMs consecutivos.
- **Onde**: `vertical-pod-autoscaler/pkg/recommender/model/container.go:206-231`, `pkg/recommender/input/{cluster_feeder,oom/observer}.go`.
- **Aplicação no Cosca**: monitorar eventos discretos de falha (timeout, 429, eviction, OOM) e converter em sinais quantitativos via margem mínima absoluta + fator multiplicativo — reação imediata a incidentes, sem depender só da métrica contínua.

### B5. Control loop de conciliação com anti-thrash (unneeded-time monotônico + health gates)
- **O que resolve**: um loop que reage a "mudança de estado" sem oscilar — só age após sinal persistir por tempo mínimo (hysteresis), respeita limites por pool e para se o sistema está doente.
- **Como funciona**: o algoritmo do CA: 1) gate de saúde global (>X% unready ⇒ halt); 2) remover nós não-registrados; 3) shrink de pools com nós faltando há tempo; 4) filtrar pods já agendáveis; 5) só expandir pools "saudáveis"; 6) contar o nó como unneeded e exigir ≥10min **e** sem scale-up recente antes de remover. `balance_similar.md` divide o scale-up entre pools "similares" (mesma capacidade, allocatable ±5%, mesmas labels) para não concentrar nós e gerar flapping.
- **Onde**: `cluster-autoscaler/proposals/{clusterstate,balance_similar,node_autoprovisioning}.md`.
- **Aplicação no Cosca**: o control loop de scaling deve ter (a) gate de saúde que pausa quando degrada, (b) contadores monotônicos de "folga" que só disparam após persistência + sem ação recente, (c) limites em nível de cluster soft/não-enforcing, (d) balanceamento entre pools equivalentes para evitar thrash.

### B6. Simulação de capacidade antes de agir: ClusterSnapshot + salvo de scale-up com time budget
- **O que resolve**: responder a um backlog grande num único loop sem errar por não saber dos recursos acabados de pedir — e sem travar o loop.
- **Como funciona**: "Scale Up Salvo" roda `runSingleScaleUp` repetidamente: após cada scale-up, injeta "nós futuros" clonados (anotação `autoscaling.k8s.io/upcoming-node`) no `ClusterSnapshot` e *simula* o agendamento dos pods nesses nós. A cada iteração o snapshot reflete o futuro, evitando pedir o mesmo recurso duas vezes. Deadline `--salvo-scale-up-budget` (padrão 1m).
- **Onde**: `cluster-autoscaler/proposals/scale_up_salvo.md`.
- **Aplicação no Cosca**: antes de decidir a capacidade final, construir um snapshot "futuro" in-memory (recursos a provisionar) e re-estimar em cima dele — recomendação com estado preditivo. Envelopado por orçamento de tempo.

### B7. Segurança de escala: eviction PDB-aware + placement gap com fallback por startup-deadline
- **O que resolve**: permitir escala/eviction sem derrubar disponibilidade — nunca evictar mais réplicas do que o orçamento permite, e distribuir réplicas considerando pods que ainda não subiram.
- **Como funciona**: o updater usa `PodsEvictionRestriction` por *creator/replica set*: `CanEvict` recusa se `belowMinReplicas`, se o grupo não é disruptável (via PDB) ou pod em Pending; conta `evicted++` por grupo para respeitar `evictionToleranceFraction`. O balancer calcula `Summary{Total, Running, NotStartedWithinDeadline}` com `StartupTimeoutSeconds` e realoca réplicas que não subiram no prazo — usando D'Hondt para split proporcional estável (ordenação por chave evita flapping).
- **Onde**: `vertical-pod-autoscaler/pkg/updater/restriction/pods_eviction_restriction.go`, `balancer/pkg/{pods/summary.go,policy/proportional.go}`.
- **Aplicação no Cosca**: ao reduzir capacidade, modelar "orçamento de disrupção" por grupo (PDB-like) e nunca alocar além dele; ao distribuir capacidade entre alvos, rastrear `NotStartedWithinDeadline` com timeout de startup e fallback para alvos saudáveis, com algoritmo determinístico.

---

## C. Governança & Processo (community) — governança do Cosca

### C1. KEP — RFC classificada por estágios (provisional → implementable → implemented)
- **O que resolve**: falta de um mecanismo formal e rastreável para uma ideia virar decisão técnica ratificada. Sem KEP, mudanças grandes entram "por atacado" via PRs.
- **Como funciona**: proposta escrita num template estruturado (`Motivation/Goals/Non-goals`, `Design Details`, `Test Plan`, `Graduation Criteria`, `Drawbacks`, `Alternatives`), versionada em `keps/<sig>/NNNN-titulo/`. Estados: **Draft/Provisional → Implementable → Implemented → Withdrawn**. Metadados `owning-sig`, `status`, `milestone` consumidos por um gerador de relatórios.
- **Onde**: `kubernetes/enhancements` (referenciado), `committee-steering/governance/sig-governance-requirements.md:57`, `governance.md:166`.
- **Aplicação no Cosca**: a CONSTITUTION tem um Ciclo de Decisão de 10 passos mas não há formato de RFC com estados formais e critérios de "graduação". O padrão KEP adicionaria: um `docs/adr/` (já existe) ganhando **estados** (`provisional → approved/implementable → implemented → withdrawn`), campos de `Motivação`, `Critérios de Graduação` e `Milestone`. É o que permite auto-evolução sem regressão (P6) registrando o frame de decidido.

### C2. OWNERS + revisão em duas fases (lgtm → approve)
- **O que resolve**: limitante = nº de pessoas aptas a revisar; qualidade = familiaridade com o código. OWNERS trata ownership granular por diretório e separa "está correto" de "é aceitável holisticamente".
- **Como funciona**: arquivos `OWNERS` por diretório listam `approvers`, `reviewers`, `labels`, `options.no_parent_owners`, `filters`. Fluxo: reviewers avaliam qualidade → `/lgtm`; approvers (só quem está no OWNERS) dão `/approve` avaliando holística; Tide só mergeia se `lgtm`+`approved` presentes e labels bloqueantes ausentes. `emeritus_approvers` mantém memória de especialistas sem veto.
- **Onde**: `contributors/guide/owners.md`, `OWNERS_ALIASES`.
- **Aplicação no Cosca**: cada área teria um `OWNERS` declarando approvers (Chief do domínio) e reviewers (especialistas). As duas fases casam com o papel duplo QA Chief (qualidade) e Review Chief (aceitação holística). `no_parent_owners` permite que domínios profundos (ex.: `internal/embed/cosca/` — P8) não herdem approve de cima.

### C3. Charter de grupo com escopo explícito (SIG/WG) + ciclo de vida (criar → ratificar → aposentar)
- **O que resolve**: como criar um órgão de trabalho (domínio) e descomissioná-lo sem deixar "órfãos". WGs são deliberadamente **efêmeros e sem autoridade** — só influência.
- **Como funciona**: todo SIG tem `charter.md` (Scope in/out, Code/Binaries, Cross-cutting processes, Roles) revisado via fluxo **OARP** (Owners/Approvers/Reviewers/Participants): donos criam, comunidade revisa, Steering **aprova e ratifica**. WG não possui código — existe para cross-SIG e deve dissolver (gatilhos objetivos: sem chair 4 semanas, canais sem uso 3 meses). SIG tem quóruns de retirada (3+ meses sem quórum → SHOULD retire, 6+ → MUST).
- **Onde**: `committee-steering/governance/{README,sig-charter-template,sig-wg-lifecycle}.md`, `governance.md:30-133`.
- **Aplicação no Cosca**: o Cosca tem ~41 Chiefs mas sem "charter" por domínio. Cada Chief seria "SIG Owner" com `charter.md` declarando **in-scope/out-of-scope** (evita sobreposição). As iniciativas temporárias (WGs) mapeiam para os workflows. O gatilho objetivo de descomissionamento resolveria a acumulação de domínios obsoletos (P7), aplicado à *org*.

### C4. Escada de contribuidor / ratificação social de papel por evidência
- **O que resolve**: como um participante **conquista** autoridade (em vez de herdar). Cada papel tem requisitos verificáveis e é ratificado por pares.
- **Como funciona**: `community-membership.md` define `Member → Reviewer → Approver → Subproject Lead`, cada nível com requisitos quantitativos (Ex.: Member = múltiplas contribuições + patrocínio de 2 reviewers; Reviewer = membro 3 meses + 5 PRs como reviewer + 20 PRs; Approver = +10 PRs como reviewer + 30 PRs). Feito via **PR que atualiza o OWNERS** (auditável). Inatividade = sem contribuições em 12 meses → remoção.
- **Onde**: `community-membership.md`, `owners.md:32`.
- **Aplicação no Cosca**: o AUTO_EVOLUTION tem níveis 1-5 de *capability técnica*, mas não tem nível de *autoridade ratificada socialmente*. Insere uma "escada de papel" (`specialist → reviewer → approver → lead`) com requisitos de evidência e **patrocínio de 2 Chiefs**. A inatividade de 12 meses é o complemento ao P7 para remover authority obsoleta.

### C5. Triage e lifecycle de issue automatizado (labels, stale, prioridade)
- **O que resolve**: transformar backlog em fluxo *puxado* gerenciável, com prazos objetivos, evitando work parado e ruído.
- **Como funciona**: issue novo entra com `needs-triage`; triage → `triage/accepted`. Categorias com labels (`kind/support`, `kind/bug`, `help wanted`, `good first issue`); prioridade em gradação `critical-urgent → important-soon → important-longterm → backlog`. SLAs objetivos: sem-info → fecha em 20 dias; owner sem PR em 30 dias → cobrado; sem atividade em **90 dias** → `lifecycle/stale`; `lifecycle/frozen` bloqueia o bot. PRs parados >90 dias são fechados.
- **Onde**: `contributors/guide/{issue-triage,pull-requests}.md`.
- **Aplicação no Cosca**: a CONSTITUTION tem prioridade P0-P3 mas sem automação de triage/stale/close. Adicionaria labels `needs-triage`→`triage/accepted`, estados `lifecycle/stale`/`frozen`, prioridade em 5 níveis, e um **gabinete de checagem** (30/90 dias). Mapeia para cosca-evolution/automation como processo de *pull* para tasks, evitando backlog-lixo (análogo ao P7).

### C6. Governança de decisão: escalação, quórum, e nível de compromisso (RFC2119)
- **O que resolve**: transparência de *como* uma decisão acontece — quem decide, como discordância é resolvida, quanto tempo uma decisão permanece válida. Evita deadlock e decisões "reversíveis a cada dor".
- **Como funciona**: `sig-governance-requirements.md` usa **RFC2119 (MUST/SHOULD/MAY)** e obriga a declarar: processo de proposta, decisores, resolução de divergência e **level of commitment** ("quando uma decisão pode ser revisitada"). Remoção de lead por **super-majority**. Escada de escalação: subproject → SIG Chairs → Steering (rara). Chair via **lazy consensus** com fallback em maioria. **Health checks anuais** com liaisons.
- **Onde**: `committee-steering/governance/sig-governance-requirements.md`, `sig-governance.md`, `governance.md:143-169`.
- **Aplicação no Cosca**: a CONSTITUTION já tem escalação e regras de conflito. O padrão adiciona: (a) **linguagem RFC2119** em vez de "Regra:" — requisitos testáveis; (b) **nível explícito de compromisso** — decisão ratificada só reverte por processo igualmente formal (evita P6 ser driblado); (c) **super-majority** para remover um Chief inativo; (d) **health check anual** por domínio — o mecanismo de auditoria objetiva de relevância (conecta ao P3/P7).

---

## D. Observabilidade de Estado (kube-state-metrics) — metrics do Cosca

### D1. MetricsStore é a própria cache.Store — lifecycle da métrica = lifecycle do objeto
- **O que resolve**: quando um objeto some, a métrica deve sumir automaticamente, sem "zumbis" de séries que falham em `absent()`.
- **Como funciona**: `MetricsStore` implementa `cache.Store`, então o reflector alimenta o store direto. `Add` (stores `o.GetUID()` → famílias em `sync.Map`), `Update` (re-Add), `Delete` (faz `metrics.Delete(o.GetUID())` — a série desaparece no próximo scrape), `Replace` (Clear + repopula após relist com novo `resourceVersion`).
- **Onde**: `pkg/metrics_store/metrics_store.go:30-239`.
- **Aplicação no Cosca**: mapear o "estado vivo" de entidades (agentes, workflows) para uma store os-UID→métricas. Ao tocar num `cache.Store` customizado, Cosca herda o ciclo delete→série removida e o relist com resourceVersion para resync sem estouro de memória.

### D2. FamilyGenerator — função pura "state -> metrics" injetada com nome tardio
- **O que resolve**: separar a *derivação* (lógica que lê um objeto e produz `Family`) da *nomenclatura/tipagem/ciclo de vida* da métrica.
- **Como funciona**: cada recurso expõe `FamilyGenerator{ Name, Help, Type, StabilityLevel, GenerateFunc(obj)*metric.Family }`. `Generate` chama a função e injeta `Name`/`Type` só depois (deduplicação, evita typo). `ComposeMetricGenFuncs` plicoteia numa única função. Cada recurso injeta labels padrão (`namespace`,`name`).
- **Onde**: `pkg/metric_generator/generator.go:30-128`, `internal/store/deployment.go`.
- **Aplicação no Cosca**: desacoplar coletores em "families" puras de derivação com injecção tardia de nome — cada collect vira uma `Func(obj) Family` testável em isolamento.

### D3. Config-driven para CRD: extração por path compilada (valuePath) e tipagem por métrica
- **O que resolve**: expor métricas de *qualquer* recurso/CRD sem escrever código Go — o usuário declara em YAML e o sistema compila para um coletor.
- **Como funciona**: config descreve `GroupVersionKind`, `metricNamePrefix`, `labelsFromPath`, e `Generator` com `metric` union (`gauge|info|stateSet`). `compile()` resolve `valuePath` (lista de `pathOp`) sobre `*unstructured.Unstructured`, com lookup de lista `[key=value]`, índice, e `*` (copiar mapa inteiro como labels). `toFloat64()` normaliza bool/string/quantity. GVKs resolvidos via descoberta de CRDs.
- **Onde**: `pkg/customresourcestate/config.go`, `registry_factory.go:38-742`.
- **Aplicação no Cosca**: permitir que cada agente/domínio declare "de que campo sai o quê" em YAML e obter métricas de entidades customizadas sem tocar no core — extensibilidade de produto dentro da observabilidade.

### D4. Cardinalidade controlada à priori: allowlist + sanitização + wildcard em cache
- **O que resolve**: o maior risco de observabilidade de estado — explodir cardinalidade com labels/annotations arbitrárias.
- **Como funciona**: labels/annotations só emitidas se na allowlist. `SanitizeLabelName` limpa inválidos; `toSnakeCase` normaliza; colisões ganham `_conflictN`. Wildcards expandidos para regex compilados **uma vez e cacheados** (`allowListPatternCache`), com `MaxPartialWildcardsPerLabel` para fail-closed em padrões explosivos. Allow/deny mutuamente exclusivos.
- **Onde**: `internal/store/utils.go:147-277`, `pkg/allowdenylist/allowdenylist.go:39-175`.
- **Aplicação no Cosca**: o cosca-core expõe por padrão um conjunto enxuto de labels "core" e só amplia mediante allowlist declarada. É o guarda de cardinalidade de um OpenMetrics de produto.

### D5. Estado/condição de saúde como one-hot (stateset) com reason limitado
- **O que resolve**: transformar status declarativo/multivalorado em séries numéricas consultáveis, mantendo a cardinalidade do reason limitada.
- **Como funciona**: `addConditionMetrics(status)` gera **um** métrico por status possível (True/False/Unknown), cada um com valor `boolFloat64(cs==status)` — decomposição one-hot. O `reason` passa por um allowlist e cai em `"unknown"` se não mapeado. Fases viram `kube_pod_status_phase`, etc. Quantidades viram pares `spec_replicas`/`status_replicas_*`.
- **Onde**: `internal/store/utils.go:68-79`, `internal/store/deployment.go:236-266`.
- **Aplicação no Cosca**: padronizar health de cada componente como um conjunto de notações one-hot (`cosca_workflow_status_condition{condition=...}`) em vez de "gauge de status final". Permite alertas tipo `1 - condition == 1` e soma agregada "quantos estão healthy".

### D6. Coleção escalável: watch instrumentado + sharding por hash consistente (jump hash por UID)
- **O que resolve**: coletar de múltiplos namespaces sem saturar o apiserver nem estourar memória, e escalar horizontalmente com sharding.
- **Como funciona**: `InstrumentedListerWatcher` embrulha List/Watch para: (a) emitir auto-métricas do coletor (`list_total`/`watch_total`), (b) aplicar `objectLimit` (trunca lista), (c) usar `ResourceVersion="0"` se usar cache. `shardedListWatch` aplica **jump-consistent hash sobre `o.GetUID()`** (`fnv64a` + `jump.Hash`) para decidir keep/drop — objeto cai exatamente em um shard. Auto-sharding detecta o StatefulSet do pod.
- **Onde**: `pkg/watch/watch.go:30-175`, `pkg/sharding/listwatch.go:33-215`.
- **Aplicação no Cosca**: "métricas de saúde do coletor" (monitorar o monitorador) e shardedListWatch como modelo de distribuição horizontal — cada entidade contribui a um shard, sem serialização duplicada nem perda.

### D7. Serve-time: snapshot de writers, negociação de formato e reconfig em runtime (auto-sharding)
- **O que resolve**: servir um "estado derivado" grande e atualizado a cada scrape sem travar em reconfiguração nem duplicar headers.
- **Como funciona**: `MetricsHandler.ServeHTTP` **snapshot** dos writers sob RLock (libera o lock no corpo — client lento não segura re-shard), negocia `expfmt.NegotiateIncludingOpenMetrics` (fallback text), gzip, filtra por `?resources=`. `SanitizeHeaders` deduplica HELP/TYPE entre writers **ativos**. `BuildWriters` reconfigurável (cancela ctx anterior, constrói novo); `ConfigureSharding`/autosharding dispara rebuild ao detectar mudança de réplicas.
- **Onde**: `pkg/metricshandler/metrics_handler.go:192-301`, `pkg/metrics_store/metrics_writer.go`.
- **Aplicação no Cosca**: servir o agregado de estado com (1) snapshot concorrente para reconfig sem block, (2) filtro por grupo de entidade na query string, (3) deduplicar headers entre writers — receita para o endpoint `/metrics` de estado reconfigurável a quente.

---

## Synthesis — o que o Cosca deveria copiar (priorizado)

| # | Padrão | Gap | Aplicação no Cosca |
|---|--------|-----|--------------------|
| 1 | A2/A4 — Sandbox de 1ª classe + spec declarativa de isolamento | **P0 sandbox** | manifesto de isolamento por sandbox (cgroups/seccomp/readonly como dados) |
| 2 | A1/A3 — Runtime gRPC plugável + negociação de features | P0 sandbox | separar core de executor; hot-swap de runtime por backend |
| 3 | A5 — ExecSync bounded (16MB) vs Exec/Attach stream | P0 sandbox | teto de output + sessões interativas TTY |
| 4 | B2/B5 — Estimador em banda + anti-thrash (hysteresis/health gate) | scaling | {target, lower, upper}, só age fora da banda, gate de saúde |
| 5 | C1 — KEP com estados + critérios de graduação | governança | dar estados ao `docs/adr/` e critérios de graduation |
| 6 | C2 — OWNERS + revisão 2 fases (lgtm→approve) | governança | ownership granular por domínio + no_parent_owners (P8) |
| 7 | D4/D5 — Cardinalidade allowlist + health one-hot | observabilidade | guarda de cardinalidade + métricas de condição com reason allowlist |
| 8 | B1 — Decaying histogram | metrics | janela deslizante `O(1)`, meia-vida configurável |

## Known Uses (referência)

- `kubernetes/kubernetes` (124k★), `kubernetes/autoscaler`, `kubernetes/cri-api`, `kubernetes/community`, `kubernetes/kube-state-metrics`.

## Related Patterns

- [`kubernetes-core-patterns.md`](kubernetes-core-patterns.md) — informer/workqueue/reconcile/admission (o core que este complementa)
- [`deepseek-harness-patterns.md`](deepseek-harness-patterns.md) — sandbox landlock/process-tree (B1-B6, mesma família de isolamento)
- [`aws-agent-toolkit-patterns.md`](aws-agent-toolkit-patterns.md) — segredos na borda/least-privilege
- [`n8n-workflow-patterns.md`](n8n-workflow-patterns.md) — engine de execução (DAG/join)
