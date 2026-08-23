# Unreal Integration Map — Cosca Living World ↔ Unreal Engine

> **Version**: 1.0.0 | **Confidence**: 0.92 | **Category**: Runtime/World Integration | **Created**: 2026-08-23
> **Source**: Inspeção direta da instalação `E:\Epic Games\UE_5.8\Engine` (headers, plugins, .uplugin, binários) + conhecimento Unreal.

> **Mined by**: cosca-kernel (ordem do Don). **Objetivo**: descobrir a melhor arquitetura Unreal ↔ Cosca. **NÃO implementado** — apenas estudo e proposta. Unreal = RUNTIME/WORLD; Cosca = CÉREBRO/ORCHESTRATOR.

---

## 0. Descobertas-chave da instalação (verificadas no disco)

| # | Descoberta | Evidência | Implicação |
|---|-----------|-----------|------------|
| D1 | **Servidor WebSocket nativo** | `WebSocketNetworking` plugin → `IWebSocketServer::Init(Port, Callback)` | Unreal pode ser **servidor WS** (não só cliente) |
| D2 | **Remote Control API** roda em **Runtime** (não só editor) | `.uplugin` → módulos `Type: "Runtime"`, "control at Runtime via a webserver (HTTP/WebSockets)" | Cosca pode controlar o jogo via HTTP/WS |
| D3 | **Envia arquivos estáticos** via WS | `IWebSocketServer::EnableHTTPServer(Dirs, bSecure)` | Serve assets direto ao Cosca |
| D4 | **WebSocket suporta Win64** | `PlatformAllowList: Mac, Win64, Linux` | Roda no Windows do Don |
| D5 | **Mass = ECS** | `MassEntity\Public\MassEntityFragments.h`, `MassEntityManager.h` | Mapeia WorldState → Mass fragments |
| D6 | **AutoML no Unreal** | `NNE` (Neural Network Engine), `PythonMLPackages` | Roda modelos de IA dentro do Unreal |
| D7 | **Niagara/MetaSound/PCG runtime** | Módulos `Niagara`, `MetasoundFrontend`, `PCG` | VFX/Audio/PCG programáveis via C++/BP |
| D8 | **Chaos Physics** | `PhysicsCore`, `Chaos*` plugins | Simulação física nativa |
| D9 | **Headless/CLI** | `UnrealEditor-Cmd.exe` | Automação, testes, run sem GUI |
| D10 | **Gameplay Ability System (GAS)** | `GameplayAbilities` plugin | Framework de habilidades/estado |
| D11 | **GameplayTags** | `GameplayTags` runtime module | Protocolo de tags para o contrato |
| D12 | **SaveGame/Serialization** | `Engine\Classes\GameFramework\SaveGame.h` | Persistência de WorldState |

---

## 1. Unreal Engine C++ API  (REPO/DOCS)

**REPO/DOCS**: Fechado (binário instalado `E:\Epic Games\UE_5.8`). Código-fonte acessível em `Engine\Source\Runtime`. Docs em `docs.unrealengine.com`.

**LICENSE**: SPDX no código; Unreal EULA (uso do engine). Plugins de terceiros têm licenças próprias. **NOTA**: o Unreal NÃO é open-source GPL como Blender — é EULA proprietário gratuito (5% royalty em receita > $1M). Para um runtime interno/científico do Cosca, licença é ok.

**API**: C++ (`UCLASS`, `UActorComponent`, `UWorldSubsystem`), Blueprint (visual), GAS, GameplayTags, Mass (C++ ECS), `IRemoteControl` (HTTP/WS).

**INPUT/OUTPUT**: C++ compilado; Blueprint visual; reflexão `UProperty`/`UPROPERTY`.

**DEPENDENCIES**: Windows SDK, Visual Studio (para C++), .NET (para UBT). Já instalado.

**PERFORMANCE**: Runtime nativo (compilado C++), ECS Mass altamente otimizado. Muito mais rápido que Python puro para simulação de mundo.

**MATURITY**: 9/10 — engine de produção AAA madura.

**AUTOMATION**: `AutomationTool`, `UnrealBuildTool`, `UnrealEditor-Cmd.exe`.

**COSCA INTEGRATION**: ✔ Alta — runtime ideal.

**UTILITY**: **MÁXIMA** — é o corpo/mundo do agente.

**RISKS**: EULA (não distribuir engine modificado); C++ complexo; build lento; curva de aprendizado.

---

## 2. Blueprints

**API**: Gelo visual gráfico; `UBlueprint`, `UBlueprintGeneratedClass`.

**INPUT/OUTPUT**: Eventos de grafo, nós, funções.

**PERFORMANCE**: Mais lento que C++ (custo por execução), mas ótimo para prototipagem rápida.

**MATURITY**: 10/10.

**AUTOMATION**: Blueprint compile via editor; testável.

**COSCA INTEGRATION**: ✔ Ótimo para **camada fina de glue** (receber um comando WS e chamar uma função). Para lógica pesada, preferir C++.

**UTILITY**: Alta para glue, baixa para lógica de decisão.

**RISKS**: Performance; difícil versionar (binário); "spaghetti".

---

## 3. Gameplay Framework

**API**: `AGameModeBase`, `AGameStateBase`, `APlayerController`, `APawn`, `AActor`.

**INPUT/OUTPUT**: Controla o fluxo do jogo (spawn, posse, estado, regras).

**DEPENDENCIES**: Engine module.

**MATURITY**: 10/10.

**COSCA INTEGRATION**: O `AGameModeBase` é o ponto natural para inicializar o bridge Cosca (spawn do Orchestrator bridge no início). Pode instanciar o `UCoscaBridgeComponent`.

**UTILITY**: Alta — o `GameMode` é onde o bridge Cosca nasce.

---

## 4. Actors / Components

**API**: `AActor` (entidade), `UActorComponent` (comportamento), `UPrimitiveComponent`, `UStaticMeshComponent`.

**INPUT/OUTPUT**: Actor = objeto no mundo; Component = funcionalidade.

**MATURITY**: 10/10.

**COSCA INTEGRATION**: **Cada `WorldEntity` do Cosca vira um `AActor`**. O `UCoscaEntityComponent` guarda o ID do Cosca + hash. Spawn/destroy é a ponte direta.

**UTILITY**: **MÁXIMA** — é a representação física do WorldState.

---

## 5. World / Level / Subsystems

**API**: `UWorld`, `ULevel`, `UGameInstanceSubsystem`, `UWorldSubsystem`, `UEngineSubsystem`.

**INPUT/OUTPUT**: Subsystems vivem com o mundo/instância — persistência de estado, tick global, eventos.

**MATURITY**: 10/10.

**COSCA INTEGRATION**: **`UCoscaWorldSubsystem`** — o coração da integração. Vive com o mundo, tem `Initialize()/Tick()`/`Deinitialize()`. Aqui:
- abre o WebSocket Server (porta do Cosca)
- recebe comandos (spawn/destroy/update)
- publica observações (frames, estado)
- despacha para GameplayTags/Actors

**UTILITY**: **MÁXIMA** — este é o ponto de entrada do Cosca no Unreal.

---

## 6. World Partition

**API**: Streaming de células do mundo (`UWorldPartition`), `PIE`.

**MATURITY**: 9/10.

**COSCA INTEGRATION**: Para mundos grandes, o Cosca pode streamar assets/actors por célula. **Fase avançada** — não no first slice.

**UTILITY**: Média (escala).

---

## 7. Enhanced Input

**API**: `UEnhancedInputLocalPlayerSubsystem`, `UInputMappingContext`, `UInputAction`.

**MATURITY**: 9/10.

**COSCA INTEGRATION**: **NÃO usar** — o Cosca é quem decide a ação, não o input de jogador. O Cosca envia comandos de ação diretamente (não via input pipes). Ignorar para o pipeline.

**UTILITY**: Baixa para Cosca (é para input humano).

---

## 8. Gameplay Ability System (GAS)

**API**: `UAbilitySystemComponent`, `UAttributeSet`, `UGameplayEffect`, `GameplayTags`.

**INPUT/OUTPUT**: Atributos, efeitos, habilidades, tags de estado.

**MATURITY**: 9/10 (robusto).

**COSCA INTEGRATION**: **Excelente para o WorldState Cosca**. `UAttributeSet` = attributes do WorldEntity (hp, velocidade, posição). `GameplayTags` = etiquetas semânticas do Cosca (`cosca.npc.guard`, `cosca.vehicle.car`). Tags são o vocabulário compartilhado.

**UTILITY**: Alta — GAS dá o modelo de atributos + tags que o contrato Cosca precisa.

---

## 9. AI Controller

**API**: `AAIController`, `UAIComponent`.

**MATURITY**: 10/10.

**COSCA INTEGRATION**: **Decisão mínima**. O Cosca (Go) toma as decisões; o Unreal executa. NUMA: `AAIController` pode receber comandos do Cosca e delegar ao Behavior Tree OU simplesmente executar. Para o first slice, `AAIController` só o move.

**UTILITY**: Média.

---

## 10. Behavior Trees

**API**: `UBehaviorTree`, `UBTNode`, `UBlackboardComponent`.

**MATURITY**: 10/10.

**COSCA INTEGRATION**: **Questão de divisão de responsabilidade**. Opção A: Cosca decide (planning), BT executa (subsídios). Opção B: BT interno decide, Cosca supervisiona. **Recomendado: A** — Cosca é o planner (cérebro), BT é o corpo (execução de ações primitivas). Para primeira fase: não usar BT, só comandos diretos.

**UTILITY**: Média (fase 2+).

---

## 11. Mass Entity / Mass AI

**API**: `MassEntityManager`, `MassEntityFragments`, `MassProcessor`, `MassSpawner`, `UE::Mass::Signals`.

**INPUT/OUTPUT**: ECS — fragments (dados) + processors (lógica). Milhões de entidades.

**PERFORMANCE**: **MÁXIMA** — ECS otimizado para milhares/milhões de entidades.

**MATURITY**: 8/10 (nova mas potente).

**COSCA INTEGRATION**: **Ideal para escalar o WorldState**. `FMassEntityHandle` = WorldEntity; `FMassFragment` = propriedade (position, velocity, tag). O Cosca pode mapear suas entidades e processar via processors. Para **muitos NPCs/objetos**, Mass é o caminho. Para o first slice (1-2 entidades), Actors bastam.

**UTILITY**: Alta (escala) — Mass para multidões, Actors para hero/número pequeno.

---

## 12. Navigation / NavMesh

**API**: `UNavigationSystemV1`, `ANavMeshBoundsVolume`, `UNavMeshPath`.

**MATURITY**: 10/10.

**COSCA INTEGRATION**: O Cosca manda `move_to(x,y,z)` → Unreal usa NavMesh para pathfind local. **Divisão clara**: Cosca decide *onde*, Unreal decide *como chegar*. Excelente.

**UTILITY**: Alta.

---

## 13. Perception System

**API**: `AIPerceptionSystem`, `AIPerceptionListener` (visão, audição, sentido), `FAIPerceptionComponent`.

**MATURITY**: 8/10.

**COSCA INTEGRATION**: **Divisão de percepção**: o Cosca faz percepção de alto nível (Vision CLIP/SAM, Spatial SLAM em Go). O Unreal faz percepção de baixo nível (sensor de proximidade, line-of-sight). Para o first slice, o Cosca é a fonte de verdade; o Unreal pode usar perception só para física (colisão, LOS).

**UTILITY**: Média (visão de baixo nível).

---

## 14. Chaos Physics

**API**: `UChaosPhysicalMaterial`, `UGeometryCollectionComponent`, `FChaosSolver`, `UForceFeedback`.

**MATURITY**: 9/10.

**COSCA INTEGRATION**: **Substitui o Taichi/MuJoCo para física de mundo no runtime**. O Cosca mantém Destruction/Simulation logic, mas a **execução física** pode rodar no Chaos do Unreal. Divisão: Cosca decide *destruir*, Chaos *fraturas/reage*. O DestructionAdapter do Cosca gera a *prescrição*, Chaos executa a *simulação física real*.

**UTILITY**: **MÁXIMA** para física no runtime.

---

## 15. Niagara

**API**: `UNiagaraSystem`, `UNiagaraComponent`, `UNiagaraDataInterface`.

**MATURITY**: 9/10.

**COSCA INTEGRATION**: **Substitui o Taichi VFX no runtime**. O Cosca decide *qual efeito* (VFXConfig do Cosca), Niagara *renderiza* (partículas, fluidos, fogo). Mapping: VFXConfig → Niagara System. O Cosca do VFXAdapter gera a *prescrição*, Niagara executa o VFX real.

**UTILITY**: **MÁXIMA** para VFX no runtime.

---

## 16. MetaSounds / Audio

**API**: `UMetasoundSource`, `UMetaSoundBuilder`, `MetasoundFrontend`.

**MATURITY**: 8/10.

**COSCA INTEGRATION**: **Substitui Coqui/AudioCraft para reprodução**. Cosca decide qual som (AudioEvent/SFX), MetaSounds reproduz. O Cosca mantém geração (TTS/SFX), Unreal reproduz espacializado.

**UTILITY**: Alta (audio runtime).

---

## 17. Procedural Content Generation (PCG)

**API**: `APCGComponent`, `PCGSystem` (hoje), `FPCGData`.

**MATURITY**: 8/10.

**COSCA INTEGRATION**: **Par complementar ao Blender**. Blender = geração offline de asset (importado). PCG = geração *in-world* procedural (não precisa de asset externo). Cosca manda `PCG: scatter árvores nesta área (seed)` → Unreal gera. Divisão: Blender para assets únicos/complexos; PCG para instancing massivo procedural.

**UTILITY**: Alta (procgen in-world).

---

## 18. Python API / Editor scripting

**API**: `PythonScriptPlugin`, `unreal` module (Python).

**MATURITY**: 8/10.

**COSCA INTEGRATION**: Útil para **autoramento do editor** (importar assets do Blender, setup de cena, testes). **NÃO para o runtime loop** (Python no Unreal é editor-only, lento).

**UTILITY**: Média (edição/CMake/inicialização).

---

## 19. Remote Control API

**API**: `IRemoteControlModule`, `RemoteControlPreset`, HTTP/WS.

**MATURITY**: 9/10.

**COSCA INTEGRATION**: **Opção de consumo fácil** — expõe propriedades/actors como endpoints HTTP/WS para controle remoto. Bom para debug e para o Don controlar manualmente. **Para o pipeline de alta frequência, preferir o WebSocket custom directo** (mais controle, menor overhead).

**UTILITY**: Média-alta (ferramenta de controle).

---

## 20. WebSocket / HTTP

**API**: `IWebSocketServer`, `WebSocketMessaging`, `FWebSocketConnection`.

**MATURITY**: 8/10 (Runtime, Win64 ok).

**COSCA INTEGRATION**: **O protocolo principal**. Unreal abre servidor WS, Cosca conecta-se como cliente. Bidirecional. Testado no disco (D1, D3, D4).

**UTILITY**: **MÁXIMA**.

---

## 21. UDP/TCP

**API**: `SocketSubsystem`, `FSocket`, `UDPSocket`.

**MATURITY**: 10/10.

**COSCA INTEGRATION**: TCP para stream de frames (câmera/vídeo de alta taxa); UDP para telemetria fire-and-forget. **Complementar** ao WS (que é a camada de comandos). Para o first slice, WS basta; frame streams grandes podem ir por TCP.

**UTILITY**: Média (complemento).

---

## 22. Unreal Subsystems

**API**: `USubsystem`, `UGameInstanceSubsystem`, `UWorldSubsystem`, `UEngineSubsystem`.

**MATURITY**: 10/10.

**COSCA INTEGRATION**: **`UCoscaWorldSubsystem`** — ver §5. Ponto de integração.

**UTILITY**: **MÁXIMA**.

---

## 23. Data Assets / Data Tables

**API**: `UDataAsset`, `UDataTable`, `FTableRowBase`.

**MATURITY**: 10/10.

**COSCA INTEGRATION**: **Contrato de dados reutilizável**. O Cosca pode gerar `UDataTable` com atributos dos WorldEntitys (id, type, posição) e o Unreal consome via DataTable. Ideal para o contrato Cosca↔Unreal (D11 GameplayTags complementam).

**UTILITY**: Alta (estrutura de dados compartilhada).

---

## 24. Serialization

**API**: `USaveGame`, `FMemoryWriter`, `FArchive`, `UProperty`.

**MATURITY**: 10/10.

**COSCA INTEGRATION**: O contrato Cosca↔Unreal usa JSON (via WebSocket). `FJsonSerializer` do Unreal parseia. Para persistência de WorldState, `USaveGame` + `FJsonSerializer`.

**UTILITY**: Alta.

---

## 25. SaveGame / persistence

**API**: `USaveGame`, `UGameplayStatics::SaveGameToSlot`, `FAsyncSaveGame`.

**MATURITY**: 10/10.

**COSCA INTEGRATION**: O Cosca é persistente (Go); o Unreal pode ser efêmero. Para snapshots do mundo, Cosca persiste em Go; Unreal persiste via SaveGame. **Divisão**: Cosca = fonte de verdade persistente; Unreal = runtime efêmero.

**UTILITY**: Média.

---

## 26. Plugins

**API**: `.uplugin`, `IModuleInterface`, `IWebSocketServer`.

**MATURITY**: 10/10.

**COSCA INTEGRATION**: **Criar um plugin Cosca** (`CoscaRuntime`) dentro do projeto Unreal com o `UCoscaWorldSubsystem` e `UCoscaEntityComponent`. É o veículo da integração.

**UTILITY**: **MÁXIMA**.

---

## 27. Headless / command-line

**API**: `UnrealEditor-Cmd.exe`, `-run=`, `-execCmds=`, `-nullrhi`, `-unattended`, `-nosplash`.

**MATURITY**: 9/10.

**COSCA INTEGRATION**: Rodar o Unreal **sem GUI** para:
- CI/testes automatizados (Automation)
- importação de assets do Blender sem abrir o editor
- server dedicado headless (runtime sem gráficos)
- **Modo servidor dedicado**: `UnrealGame-Win64-Shipping.exe` headless


**UTILITY**: Alta (automação + server).

---

## 28. Automation Testing

**API**: `AutomationTest`, `IMPLEMENT_SIMPLE_AUTOMATION_TEST`, `AutomationControllerRpc`.

**MATURITY**: 10/10.

**COSCA INTEGRATION**: Testar o plugin Cosca no Unreal via AutomationTests rodados headless (CI). Cosca (Go) dispara `UnrealEditor-Cmd -run=Automation` e parseia resultado.

**UTILITY**: alta (qualidade).

---

## 29. Functional Testing

**API**: `FunctionalTesting` (Editor module), `AFunctionalTest`, `FTestRunner`.

**MATURITY**: 9/10.

**COSCA INTEGRATION**: Testar cenários *in-world* (spawnar entidade, dar comando, verificar que o actor se moveu). Par perfeito para validar o first slice.

**UTILITY**: Alta.

---

## 30. Unreal ↔ external process integration

**API**: `FPlatformProcess::CreateProc`, `FString`/exec.

**MATURITY**: 8/10.

**COSCA INTEGRATION**: O Unreal **can spawn external processes** (e.g. chamar o binário Cosca, ou o Blender). Mas a arquitetura recomendada: **Cosca (Go) orquestra** e o Unreal é o spawn-alvo, não o spawner. Simetria: Cosca chama Blender; Cosca chama Unreal.

**UTILITY**: Média.

---

## 31. Unreal ↔ Blender pipeline

**API**: Blender → GLB/FBX/USD → Unreal import (`UImportSubsystem`, `UInterchange`).

**MATURITY**: 9/10 (Interchange novo).

**COSCA INTEGRATION**: **Divisão clara**:
1. Cosca → Blender (subprocess) → GLB + hash (JA FEITO e testado!)
2. Cosca → Unreal (WS command) → `import asset` (GLB) + `spawn actor`
3. Unreal importa via Interchange/asset registry, spawna no mundo

O Cosca é o **broker**: busca o GLB do Blender, entrega hash ao Unreal, pede spawn.

**UTILITY**: **MÁXIMA** — fecha o triângulo Cosca↔Blender↔Unreal.

---

## 32. Unreal ↔ external AI/runtime architecture

**MATURITY**: 9/10 (Remote Control for virtual production).

**COSCA INTEGRATION**: **A arquitetura recomendada** — ver mapa abaixo.

---

# UNREAL INTEGRATION MAP (Arquitetura Recomendada)

```
┌────────────────────────────────────────────────────────────────┐
│                      COSCA (Go) — CÉREBRO                       │
│                                                                │
│  Orchestrator                                                   │
│   ├─ Vision (CLIP/SAM/Depth)   ├─ Spatial (SLAM)               │
│   ├─ Audio (Whisper/Coqui)     ├─ Multi-Agent (SharedWorld)    │
│   ├─ WorldModel (WorldState)   ├─ Asset (BlenderAdapter)       │
│   └─ Decision (planner)                                        │
│                                                                │
│  ┌──────────────────────────────────────────────┐              │
│  │   CoscaBridgeClient (WebSocket CLIENT)       │              │
│  │   envia: cmd.spawn / cmd.move / cmd.destroy  │              │
│  │   recebe: obs.frame / obs.state / ack        │              │
│  └──────────────────────┬───────────────────────┘              │
└─────────────────────────┼─────────────────────────────────────┘
                          │  WebSocket (JSON, porta fixa)
          🔗  ws://localhost:9000/unreal  🔗
                          │
┌─────────────────────────┼─────────────────────────────────────┐
┌─────────────────────────▼─────────────────────────────────────┐│
│                 UNREAL (unreal) — CORPO/WORLD                 ││
│                                                                ││
│  UCoscaWorldSubsystem (world-lifetime)                         ││
│   ├─ WebSocket SERVER (IWebSocketServer)                       ││
│   │    ├─ onConnect    → registra Cosca                        ││
│   │    ├─ onCommand    → spawn/move/destroy/update             ││
│   │    └─ onDisconnect → limpa estado                          ││
│   │                                                           ││
│   ├─ Entity Registry: WorldEntity ID → AActor                  ││
│   ├─ SpawnEngine: SpawnActor + UCoscaEntityComponent(id, hash) ││
│   ├─ MoveEngine: NavMesh pathfind + apply                      ││
│   └─ Observation: frame capture + send back ao Cosca           ││
│                                                                ││
│  Runner: GameMode (spawns subsystem) + AActor / Mass entities   ││
│  World features: Chaos (física), Niagara (VFX), MetaSound      ││
│                 PCG (procgen), NavMesh, GAS (tags)             ││
└────────────────────────────────────────────────────────────────┘
```

---

## RESPOSTAS ÀS 14 PERGUNTAS DO DON

### 1. Onde o Orchestrator do Cosca deve entrar no Unreal?
**`UCoscaWorldSubsystem`** (um `UWorldSubsystem` criado pelo plugin `CoscaRuntime`). É o ponto que vive com o mundo, tem `Initialize()/Tick()/Deinitialize()`, e é onde o WebSocket Server sobe. O `AGameModeBase` apenas spawna o subsystem. **Nunca** no nível de Actor isolado — o subsystem é o hub.

### 2. O Unreal deve ser cliente ou servidor?
**SERVIDOR WebSocket**. Evidência: `IWebSocketServer::Init(port, callback)` é nativo (D1). Razão: o Unreal precisa de lives no mundo e recebe comandos contínuos do Cosca; servidor é o padrão correto (um Cosca, muitos mundos/servidores futuros). O Cosca conecta-se como **cliente único**.

### 3. WebSocket é realmente a melhor opção?
**SIM**, para comandos/eventos de baixa-média frequência. Razões: bidirecional, JSON nativo, Runtime nativo no Unreal, Win64 ok (D4), suporta HTTP estático junto (D3). **Limitação**: não ideal para stream de frames de câmera de alta taxa. Para isso usar **TCP separado** (ou enviar frame comprimido via WS base64 se < 60fps).

### 4. Qual protocolo seria melhor para comandos/eventos?
**JSON sobre WebSocket** com um envelope de mensagem tipado:
- `cmd.spawn`, `cmd.move`, `cmd.destroy`, `cmd.update`, `cmd.vfx`, `cmd.audio`, `cmd.physics`
- `obs.frame`, `obs.state`, `ack`, `error`
- Use `GameplayTags` (D11) como vocabulário semântico.
- Para alta frequência de telemetria: **UDP fire-and-forget**. Para vídeo: **TCP**.

### 5. Como representar entidades do WorldState como Actors?
`WorldEntity.id` → `AActor` com um `UCoscaEntityComponent` (guarda `id`, `hash`). Campos Cosca (position, rotation, scale, type, tag) mapeados para `FTransform` + `GameplayTags`. Um `UMap<FString, AActor*>` no subsystem faz a lookup id→actor. Actors hero/pequenos; **Mass** (ECS) para centenas/milhares.

### 6. Como sincronizar WorldState ↔ Unreal World?
**Event-sourced, push do Cosca**:
- Cosca é **fonte de verdade** (única autoridade sobre o estado).
- Unreal **executa** e **reporta** o resultado (posição real após física, colisão, etc.).
- Fluxo: Cosca decide estado → envia `update` → Unreal aplica → responde `ack` com o estado *resultante* (posição real pós-física) → Cosca reconcilia.
- Cosca **não** é dona da simulação física — Chaos é (D8). Cosca reconcilia com o que o Unreal reportou.

### 7. Como o Unreal envia frames/observações de volta?
**`obs.frame`** (câmera do Unreal → Cosca Vision) e **`obs.state`** (posição real, colisões, eventos). O `UCoscaWorldSubsystem` captura a câmera do player (ou viewport), serializa PNG/JPEG comprimido, envia ao Cosca. Para alta taxa, TCP; para baixa, WS.

### 8. Como Vision/Audio/Spatial entram no ciclo?
O Unreal envia **`obs.frame`/`obs.audio`** → Cosca roda **Vision** (CLIP/SAM/Depth), **Spatial** (SLAM), **Audio** (whisper) em Go → Cosca atualiza WorldState → Cosca decide → envia comando de volta. **Percepção de alto nível fica no Cosca**; percepção de baixo nível (proximidade, LOS) pode ficar no Unreal Perception System.

### 9. Como Blender assets entram no Unreal?
Pipeline do Cosca (broker):
1. Cosca → Blender (subprocess) → **GLB + SHA256** (✅ já funcionando, testado)
2. Cosca → Unreal (WS) → `cmd.spawn_asset {glb_path, hash, type}`
3. Unreal importa via **Interchange** + registra (content-addressable pelo hash) + spawna `AActor`

O Cosca é o broker. O Blender é o worker de asset. O Unreal é o consumidor.

### 10. Onde ficam VFX, Destruction, Simulation e Multi-Agent?
**Prescrição no Cosca, execução no Unreal**:
- **VFX**: Cosca gera `VFXConfig` → Unreal Niagara renderiza (D7)
- **Destruction**: Cosca gera `Material`/prescrição → Unreal Chaos fratura (D8)
- **Simulation**: Cosca ocategoria (clima, agentes) → Unreal Chaos executa física; Cosca roda simulação de alto nível (economia, ecologia) se precisar
- **Multi-Agent**: Cosca SharedWorld decide (quem faz o quê) → Unreal executa per-agent

**Regra**: **Decisão no Cosca, execução física no Unreal.**

### 11. O que deve ficar no Go/Cosca?
- **Percepção de alto nível**: Vision, Spatial, Audio (CLIP, SLAM, whisper)
- **Decisão**: planner, Multi-Agent (SharedWorld, tasks, conflicts)
- **WorldState**: fonte de verdade, persistência
- **Asset brokerage**: BlenderAdapter, hash, validação
- **Orquestração**: Orchestrator (orquestra tudo)
- **Protocolo**: bridge client (comandos ao Unreal)

### 12. O que deve ficar no Unreal?
- **World/Runtime**: Actors, Mass, Level
- **Física**: Chaos (colisão, destruição real)
- **VFX**: Niagara
- **Áudio**: MetaSounds
- **Procgen**: PCG
- **NavMesh**: pathfinding
- **Percepção de baixo nível**: proximity, LOS
- **Renderização/visão**: captura de câmera, envia frame ao Cosca

### 13. Qual é o contrato de dados entre os dois?
**JSON sobre WebSocket**, envelope tipado:
```json
// Cosca → Unreal (comandos)
{
  "v": 1,
  "type": "cmd",
  "cmd": "spawn_actor",
  "id": "entity_001",
  "entity": {"kind": "npc", "tags": ["cosca.npc.guard"]},
  "transform": {"pos": [x,y,z], "rot": [w,x,y,z], "scale": [x,y,z]},
  "asset_hash": "sha256:...",
  "params": {},
  "seq": 42
}

// Unreal → Cosca (observações)
{
  "v": 1,
  "type": "obs",
  "obs": "state",   // ou "frame"
  "seq": 42,
  "state": {
    "entities": [{"id":"entity_001","pos":[x,y,z],"collision":true}]
  }
}
```
- **id**: string (WorldEntity.id)
- **tags**: `GameplayTags` (vocabulário semântico · D11)
- **hash**: SHA256 do asset (para dedup/provenance)
- **seq**: correlação request/response

### 14. Qual é o menor vertical slice possível?
```
Cosca (Go) → WS → Unreal
  1. Unreal sobe: UCoscaWorldSubsystem abre WS server na :9000
  2. Cosca conecta (client) → handshake
  3. Cosca envia: cmd.spawn_actor { id:"cube_1", asset_hash, pos:[0,0,50] }
  4. Unreal: importa GLB (se não tem), spawna AActor com o hash, ack
  5. Cosca envia: cmd.move { id:"cube_1", target:[100,0,50] }
  6. Unreal: NavMesh pathfind + move, ack com posição final
  7. Unreal envia obs.frame (câmera) → Cosca Vision processa
  8. Cosca atualiza WorldState → repete
```

**Slice mínimo**: spawnar 1 entity + mover + observar 1 frame = **prova do WorldModel de ponta a ponta**.

---

## TESTING STRATEGY

| Camada | Ferramenta | O que testa |
|--------|-----------|-------------|
| Go/Cosca | `go test` | WorldModel, Orchestrator, protocolo, bridge client (mock do Unreal) |
| Unreal unit | `AutomationTest` (C++) | UCoscaWorldSubsystem, parsing de comandos, spawn/move |
| Unreal func | `FunctionalTesting` | Cenários in-world (spawn→move→verify posição) |
| End-to-end | `UnrealEditor-Cmd -run=Automation` headless | Ponta a ponta Cosca↔Unreal |
| Integração real | binário Cosca + Unreal | Slice vertical completo |

---

## BOUNDARIES (não criar segunda arquitetura)

O Cosca **já tem**: Orchestrator, WorldModel, tipos, bridge client (em `internal/bridge`), os 7 pipelines, BlenderAdapter. **Nada disso muda.**

Só se adiciona:
- **No Unreal**: um plugin `CoscaRuntime` (subsystem + entity component + WS server)
- **No Cosca**: o `internal/bridge/client.go` (já existe) implementado para falar com o WS server do Unreal

**O Cosca continua sendo o cérebro. O Unreal passa a ser o runtime/world.** Não há arquitetura paralela — apenas o runtime do mundo migra de "em memória" para "físico no Unreal".

---

## RISCOS

1. **EULA do Unreal** — não distribuir engine modificado; royalty em receita > $1M. Para uso interno Cosca, ok.
2. **Complexidade C++** — o plugin CoscaRuntime exige C++/VS. Mitigação: manter mínimo; usar Blueprint para glue.
3. **Build lento** — Unreal build pesado. Mitigação: `UnrealBuildTool` incremental, CI.
4. **WebSocket experimental** — `WebSocketNetworking` é "Experimental". Mas Remote Control (Runtime) usa-o em produção para virtual production. Risco gerenciável; fallback: Remote Control API ou TCP.
5. **Fazer o Cosca ser a fonte de verdade** com o Chaos sendo dono da física → **conflito de autoridade**. Mitigação: Cosca reconcilia com o `ack` do Unreal (posição real pós-física).

---

## DECISÕES ATÉ AGORA (resumo)

| # | Decisão | Choice |
|---|---------|--------|
| 1 | Unreal é | **Servidor WebSocket** (runtime) |
| 2 | Protocolo | **JSON/WS** (comandos+obs) + TCP (frames) + UDP (telemetria) |
| 3 | Entrada Cosca | **UCoscaWorldSubsystem** (plugin) |
| 4 | Entity→Actor | AActors (pequeno) / **Mass** (escala) |
| 5 | Física | **Chaos** executa, Cosca decide |
| 6 | VFX/Audio | **Niagara**/**MetaSound** executam, Cosca decide |
| 7 | Asset | **Blender → GLB+hash → Cosca → Unreal import/Interchange** |
| 8 | Percepção | **Cosca** (alto nível) + **Unreal** (baixo nível) |
| 9 | Fonte de verdade | **Cosca** (reconcilia com ack do Unreal) |
