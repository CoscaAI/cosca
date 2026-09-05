# Unreal Engine Integration Patterns — O Agente Vivo no Mundo (UE5)

> **Version**: 1.0.0 | **Confidence**: 0.78 | **Category**: Game/World Patterns | **Created**: 2026-08-23 | **Source**: Unreal Engine 5 (EpicGames) — arquitetura (PCG, World Partition, Gameplay/AI Framework, GAS, Niagara, MetaHuman)

> **Mined by**: cosca-kernel (ordem do Don). **Nota honesta:** o repo `EpicGames/UnrealEngine` **não é clonável anonimamente** (exige EULA/conta) e é dezenas de GB. Este doc é **mineração de arquitetura** do UE5 (conhecimento do framework) focada no OBJETIVO do Don: **o Cosca criar o mundo e o agente TER UMA VIDA DENTRO DO JOGO**. O Don já domina a criação no UE; o valor pro Cosca é a **INTEROPERABILIDADE — como o agente entra no mundo e age**.

## Purpose

O Unreal Engine é o runtime onde o Don vai construir o mundo e hostear o agente ("já conheço como criar"). Este doc cataloga os **frameworks do UE que o Cosca precisa interoperar** para o agente viver no mundo: PCG (criar o mundo), World Partition (o mundo vivo/infinito), Gameplay+AI Framework (onde o agente vive e age), GAS (as ações/atributos), e a **ponte Cosca↔Unreal** (o agente como NPC/pawn dirigido pelo Cosca).

---

## A. Criar o mundo — Unreal PCG (Procedural Content Generation)

### A1. PCG como Grafo de Nós (dados → mundo) — o "Sceelix do UE"
- **O que resolve**: gerar conteúdo procedural (terreno, vegetação, cidade, proprs) a partir de um grafo de nós, data-driven.
- **Como funciona**: um `PCGComponent` num ator hosta um **PCG Graph** (nós + arestas). Cada nó gera/transforma **`PCGData`** (`PCGSpatialData` com pontos/mais, `PCGPointData`). O grafo é re-executado (regen) quando inputs mudam; `PCGData` flui pelos nós como um pipeline espacial. Nós têm **input/output** tipados e **settings** serializáveis (Unity-style). O runtime **cooka** o resultado ou gera em runtime (para mundos dinâmicos).
- **Padrão**: **grafo espacial data-style** (igual Sceelix) — separa *dados* (PCGData) de *processamento* (nós). Determinístico por seed (`FRandomStream`).
- **Aplicação no Cosca**: o Cosca pode **compor o mundo como um PCG Graph** (assets de PCG) e o Don já sabe editar no UE. O `procgen` do Cosca mapeia para PCG nó→nó. A seed do mundo = `FRandomStream(seed)` reproduzível.

### A2. `PCGData` — o "átomo de mundo" + tipos espaciais
- **O que resolve**: representar o que flui entre nós (pontos, mais de superfície, metadata) de forma genérica.
- **Como funciona**: `PCGSpatialData` (tem bounds/posição), `PCGPointData` (lista de `PCGPoint` — posição, escala, rotação, densidade, bounds), tags/metadata por ponto (`PCGMetadata`). Nós de *selection* por região, *filter* por atributo, *transform* (escala/rota), *spawn* (colocar um mesh): `CreateTargetActor`, `AddTags`.
- **Padrão**: **ponto espacial + metadata** como unidade; pipeline espacial com seleção (tag/atributo/região).
- **Aplicação no Cosca**: o "átomo de mundo" do Cosca = `PCGPoint` (posição+escala+rotação+metadata). O Cosca pode **posicionar gameplay** (o agente consulta pontos por tag — ex: `@bioma`, `@poi`), igual os atributos `@attr` do Sceelix.

### A3. Regras de geração com `PCGSettings` + determinismo por seed
- **O que resolve**: regras de criação (distribuição, densidade, bioma) como dados re-configuráveis e reproduzíveis.
- **Como funciona**: cada nó tem **settings** (padrões/parâmetros) serializáveis; `FRandomStream` para aleatoriedade determinística (seed) — mesma seed → mesmo mundo. `PCGHighResolutionSettings` (densidade/bioma), `PCGPartition` (dividir em células p/ streaming).
- **Padrão**: **regras como dados + RNG determinístico** — o padrão do Sceelix (seed + parâmetros).
- **Aplicação no Cosca**: o Cosca define a **"receita de mundo"** (regras/bioma + seed) e o UE re-executa — mundo reproduzível entre sessões (a memória do agente sobre o mundo não se perde).

### A4. Runtime vs Cook (gerar no editor vs no jogo)
- **O que resolve**: decidir quando gerar — conteúdo caro no cook (editor) vs dinâmico no runtime (para mundos vivos/mutáveis).
- **Como funciona**: PCG pode ser **cookado** (pré-gerado, estático) ou **criado em runtime** (regen quando um evento muda o mundo — ex: o agente destrói/espalha). `bGeneratedInRuntime`, regen on-change.
- **Padrão**: **geração estática (cook) + geração dinâmica (runtime)**.
- **Aplicação no Cosca**: o agente PODE MUTAR o mundo (o Don quer vida dentro) — usar PCG em runtime para reagir (uma árvore cai, um bioma muda). O Cosca expõe eventos → regen.

---

## B. O mundo vivo — World Partition & Streaming

### B1. World Partition — mundos infinitos/escaláveis
- **O que resolve**: mundos abertos gigantes sem carregar tudo — dividir o mundo em células que carregam/descarregam conforme o jogador (agente) se move.
- **Como funciona**: `World Partition` divide o mapa em **células** (grid) e faz **streaming** assíncrono (`Level Streaming`): células próximas carregadas, distantes descarregadas. `Data Layers` (camadas por tipo de conteúdo — gameplay, visual, collision). `WorldPartitionStreamingSource` (a fonte — o agente) define o que carrega.
- **Padrão**: **celular + streaming por proximidade** (dividir o mundo em células carregáveis) — o mesmo conceito do sharding/partição do k8s aplicado a mundo 3D.
- **Aplicação no Cosca**: o agente vive num **mundo infinito**; o Cosca precisa saber **"o que está carregado"** (a célula atual) — o agente só age no que está no mundo ativo. A memória do agente sobre o mundo é **distribuída por célula**.

### B2. Data Layers — camadas de conteúdo separadas
- **O que resolve**: separar tipos de conteúdo (gameplay vs visual) sem duplicar código; ativar/desativar camadas.
- **Como funciona**: `UDataLayer`/`UDataLayerInstance` — camadas por categoria, cada célula referencia. `Data Layer Manager` permite **ativar/desativar em runtime** (ex: desativar camada de "vida selvagem" à noite).
- **Padrão**: **camadas por categoria ativadas em runtime**.
- **Aplicação no Cosca**: o Cosca pode **gerenciar o mundo por camadas** (ex: camada "população", camada "eventos") — ligado à memória em camadas da Mega Brain.

---

## C. Onde o agente vive — Gameplay Framework

### C1. GameMode / GameState / PlayerController (a "regra do mundo")
- **O que resolve**: a orquestração da sessão de jogo — quem/regras.
- **Como funciona**: `AGameMode` (regras da partida, spawn dos jogadores), `AGameState` (estado replicado da partida), `APlayerController` (interface do jogador). Para um agente, o "jogador" é o **pawn do agente**.
- **Padrão**: **GameMode = orquestração da sessão** (o "kernel do mundo").
- **Aplicação no Cosca**: o Cosca (o "cérebro") pode ser o **GameMode/logic controller** da sessão — decide as regras do mundo, enquanto o agente (pawn) age. Analogia: GameMode = orchestrator do Cosca; Pawn = o agente.

### C2. Pawn / Character (o corpo do agente no mundo)
- **O que resolve**: a entidade física/visual que o agente controla.
- **Como funciona**: `APawn` (entidade controlável), `ACharacter` (com mesh + `CharacterMovementComponent` para navegação). O pawn tem um `AController` que o dirige.
- **Padrão**: **corpo (Pawn) separado do cérebro (Controller)** — uma entidade que age, dirigida por um controlador.
- **Aplicação no Cosca**: **o agente = um Pawn/Character** (corpo no mundo) controlado pelo `AIController` (cérebro). O Cosca dirige o controlador → o corpo age. É a **materialização do agente no mundo**.

### C3. Actor Component (composição de capacidade)
- **O que resolve**: compor capacidades por semelhança (mover, perceber, interagir) sem herança profunda.
- **Como funciona**: `UActorComponent` — componentes plugados num ator (movimento, percepção, habilidades, visão). O ator é um container de componentes.
- **Padrão**: **composição por componente** (não herança) — o UE é "agente = actor + componentes".
- **Aplicação no Cosca**: o agente é composto de **capacidades (componentes)** — perceber, mover, interagir, comunicar. O Cosca expõe o agente como "actor + componentes", cada componente uma capacidade.

---

## D. Como o agente age — AI Framework & GAS

### D1. AIController + Behavior Tree (o cérebro do agente)
- **O que resolve**: decidir o que o agente faz — lógica de decisão reutilizável.
- **Como funciona**: `AAIController` (controlador que possui um pawn), **Behavior Tree** (`UBehaviorTree`): nós de `Selector`/`Sequence`/`Decorator`/`Task`/`Service`, com **Blackboard** (memória de trabalho: variáveis/comandos). O BT é o "plano de comportamento". **EQS** (`EnvQuerySystem`) para perguntar ao ambiente (onde tem cobertura, melhor rotaponto).
- **Padrão**: **árvore de decisão + memória de trabalho (Blackboard)** — comportamento separado do corpo.
- **Aplicação no Cosca**: **o Behavior Tree é o "loop do agente" no mundo** — o Cosca pode gerar/atualizar o BT e o Blackboard (memória). Os `Task`/`Service` são as "ações" do Cosca no mundo. O Blackboard = memória de curto prazo do agente no mundo.

### D2. Perception (o agente "sente" o mundo)
- **O que resolve**: o agente perceber o que está ao redor (visão, audição, sinais).
- **Como funciona**: `UAIPerceptionComponent` (senses de visão/audiência/equipe), com `AIPerceptionListener`/`AIPerceptionStimuliSource` (o que emite estímulo), e delegates de "visto/ouvido". Eventos de percepção → BT.
- **Padrão**: **senses como fonte de eventos** (percepção → comportamento).
- **Aplicação no Cosca**: o agente **sente o mundo** (percepção) → o Cosca recebe esses **eventos de percepção** (viu X, ouviu Y) e reage. É o **"sentidos" do agente** ligado ao Cosca.

### D3. Gameplay Ability System (GAS) — as "ações/atributos" do agente
- **O que resolve**: habilidades/ações do agente de forma data-driven e extensível (combate, magia, interação).
- **Como funciona**: `UAbilitySystemComponent` — **Attributes** (vida, mana, stats), **GameplayEffects** (modificadores/status), **GameplayAbilities** (ações/capacidades com custo/cooldown), **GameplayCues** (feedback visual/sonoro). Abilidades via **attribute set** (data) + **effect** (duração/custo).
- **Padrão**: **ação = capacidade data-driven com custo/cooldown + atributos/efeitos** — o "estado e ações" do agente.
- **Aplicação no Cosca**: **as "ações do agente" no mundo = GameplayAbilities**, os "atributos" = GameplayAttributes, os "efeitos/status" = GameplayEffects. O Cosca **decide quais abilities invocar** (via GAS) e **lê o estado** (atributos/efeitos). Liga à **memória/estado** do agente.

### D4. Animação / Control Rig (o corpo reage às ações)
- **O que resolve**: animar o corpo conforme as ações/estado.
- **Como funciona**: `AnimInstance` + anim graphs (Blend Space, State Machine), **Control Rig** (rig procedural). Estado → animação.
- **Padrão**: **estado→animação** (o corpo expressa o comportamento).
- **Aplicação no Cosca**: quando o Cosca decide uma ação (ability) → o corpo anima de acordo (a "vida" fica visível). O estado emocional/do agente (do DNA voice) pode virar animação.

---

## E. Dados & Herança do Controle

### E1. Data-Driven (Data Assets / Data Tables)
- **O que resolve**: conteúdo como dados (em vez de hard-coded) — o mundo/agentes como assets editáveis.
- **Como funciona**: `UDataAsset`/`DataTable` — config em assets estruturados; `TSoftClassPtr`/`TSoftObjectPtr` para referência. O design é "data-driven".
- **Padrão**: **lógica < dados** — assets configuráveis, conteúdo separado de código.
- **Aplicação no Cosca**: o Cosca gera **Data Assets** (a configuração do mundo/agentes/personas) que o UE consome — o "caderno" vira dados do jogo.

### E2. Blueprint (o runtime que o Don domina) + C++
- **O que resolve**: scripting visual (Blueprint) + C++ nativo — o equilíbrio que o Don conhece.
- **Como funciona**: `UBlueprint` (nós de evento/função, data), C++ classes (`UCLASS`) expostas ao Blueprint via `UFUNCTION`/`UPROPERTY`. Blueprint para conteúdo, C++ para performance/fundamentação. **UInterface** para contratos entre systems.
- **Padrão**: **Blueprint (lógica/conteúdo) + C++ (fundação)** — e **UInterface** como contrato de interoperabilidade.
- **Aplicação no Cosca**: o Cosca expõe o que o UE conecta via **interfaces** (`CoscaAgentInterface` com funções `ExecuteAction`, `ReportState`, `HandleEvent`) — o Don liga no Blueprint/C++.

---

## F. A ponte Cosca ↔ Unreal — O agente vivo (o coração do objetivo)

### F1. O Agente como um Actor dirigido pelo Cosca (hosted pawn)
- **O que resolve**: materializar o agente do Cosca como uma entidade no mundo do UE.
- **Como funciona**: um **`ACoscaAgentPawn`** (Character/Pawn) com `UAbilitySystemComponent` + `UAIPerceptionComponent`, cujo `AAIController` é **dirigido por um runtime externo** (o Cosca) em vez de BT puro. O agente **possui o corpo**; o Cosca **possui a mente**.
- **Padrão**: **mente (Cosca) ↔ corpo (Pawn)** — separação cérebro/corpo; o Cosca decide, o Pawn executa.
- **Aplicação no Cosca**: cada "vida" do agente = um `ACoscaAgentPawn`. O Cosca (via runtime) envia **ações** (move, interact, cast ability) e recebe **percepção/estado**.

### F2. Channel de comunicação (eventos → ações → estado)
- **O que resolve**: o Cosca e o Unreal se comunicarem em tempo real.
- **Como funciona**: **WebSocket/HTTP** (o Cosca é um serviço — reusa o que o projeto já tem: `api/stream/ws`, ports) ou **UObject/eventos in-process** (se embutido). Fluxo: **Unreal → evento de percepção → Cosca (mente) → decide → ação → Unreal executa (ability/move)**. O estado do agente (atributos/efeitos/posição) vai pro Cosca como **memória**.
- **Padrão**: **loop percebe→decide→age→estado** (o mesmo "agent loop" do Cosca, aplicado ao mundo).
- **Aplicação no Cosca**: o Cosca é o **cérebro**; o Unreal é o **corpo/mundo**. WebSocket/HTTP bridge (port `14120` do Cosca já é REST/WS) — o agente "vive" quando o Cosca processa percepção e devolve ação.

### F3. Memória do agente distribuída no mundo
- **O que resolve**: o agente lembrar do mundo e da própria história.
- **Como funciona**: **Blackboard** (memória de trabalho curto), **GAS Attributes** (estado), **Data Assets** (config), e o **Cosca** guarda a memória de longo prazo (knowledge base / `internal/memory`). O mundo é **persistido** (save/streaming) — o Cosca continua a "vida" entre sessões.
- **Padrão**: **memória em camadas** (Blackboard curto + Cosca longo) — liga ao `supersedes`/camadas da Mega Brain.
- **Aplicação no Cosca**: o agente "lembra" via o `cosca-memory` (o caderno), e o mundo persiste via UE — a vida do agente **continua** quando ele re-entra.

---

## Synthesis — o que o Cosca deve interoperar no Unreal (p/ o agente viver)

| # | Framework UE | Função no "mundo" | Ponta pro Cosca |
|---|--------------|-------------------|-----------------|
| 1 | **PCG** | criar o mundo (grafo espacial data-driven, seed) | Cosca compõe o "PCG Graph" + seed; `procgen` do Cosca mapeia |
| 2 | **World Partition** | mundo infinito (células/streaming) | agente age só no que está carregado; memória por célula |
| 3 | **Gameplay Framework** (Pawn+Controller) | onde o agente vive | agente = `ACoscaAgentPawn` (corpo) + cosmos (mente) |
| 4 | **Behavior Tree + Blackboard** | como o agente decide | Cosca gera/atualiza o BT; Blackboard = memória curta |
| 5 | **GAS** | as ações/atributos do agente | Cosca decide abilities, lê atributos/efeitos (estado) |
| 6 | **Perception** | o agente sente | eventos de percepção → Cosca (os "sentidos") |
| 7 | **UInterface + Blueprint/C++** | como conectar | `CoscaAgentInterface` (ExecuteAction/ReportState) — o Don liga |
| 8 | **WebSocket/HTTP** | a ponte Cosca↔Unreal | reusa a porta 14120 do Cosca (REST/WS) |

## Known Uses (referência) / Honestidade

- **Repo Unreal não clonável anonimamente** (EULA/conta, dezenas de GB) — este doc é **mineração de arquitetura** do UE5 (conhecimento do framework) focada no objetivo. Para código do UE, o Don (que já conhece) tem acesso no Epic Games Launcher/GitHub autenticado.
- UE5 é o runtime onde o Don vai construir.

## Related Patterns

- [`generative-media-patterns.md`](generative-media-patterns.md) — PCG Sceelix + 3D (as mesmas ideias, no UE)
- [`mega-brain-patterns.md`](mega-brain-patterns.md) — memória em camadas + agent loop
- [`ruflo-patterns.md`](ruflo-patterns.md) — capability inventory (o agente expõe UInterface)
- docs/architecture/{scene-graph,procgen}.md — a base do Cosca que o UE estende
