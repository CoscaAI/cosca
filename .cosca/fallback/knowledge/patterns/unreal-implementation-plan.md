# Cosca ↔ Unreal — Implementação do Vertical Slice

> **Version**: 1.0.0 | **Confidence**: 0.90 | **Category**: Implementation Plan | **Created**: 2026-08-23
> **Pré-requisito**: `unreal-integration-map.md` (arquitetura + 14 decisões). **Não implementa código** — apenas o plano mínimo.

---

## Objetivo

Provar o WorldModel de ponta a ponta com o **menor número possível de partes**:

```
Cosca (Go) ⇄ WebSocket ⇄ Unreal (plugin CoscaRuntime)
```

Não substitui nada do que existe. Adiciona **2 peças novas**:
1. `WebSocketClient` (real) no Cosca — em `internal/bridge/`
2. Plugin `CoscaRuntime` no Unreal — junto ao projeto Unreal

---

## Princípios (não criar arquitetura paralela)

| # | Princípio |
|---|-----------|
| P1 | Cosca é o **cérebro** (fonte de verdade, decisão, percepção) |
| P2 | Unreal é o **corpo** (runtime, física, render, world) |
| P3 | **Decisão no Cosca, execução física no Unreal** |
| P4 | Cosca reconcilia com o `ack` do Unreal (posição real pós-física) |
| P5 | Contrato = **JSON sobre WebSocket**, envelope tipado |
| P6 | `id` do `WorldEntity` = `id` do Actor (hash de correlação) |
| P7 | Tudo é idempotente e correlacionado por `seq` |

---

## O que JÁ existe (não mexer)

- `internal/worldmodel/` — tipos, 7 pipelines, Orchestrator, BlenderAdapter (✅ testado real)
- `internal/bridge/client.go` — `Client` interface, `LocalClient` (memória), contrato `Message`/`SpawnPayload`/`FramePayload`/`ActionPayload`
- `scripts/blender/generate.py` + `validate.py` — geração/validação GLB (✅ testado real, hash SHA256)
- CLI `cosca asset gen` — gera e registra asset via Blender (✅ testado)

---

## O que falta (2 peças)

### Peça A — `WebSocketClient` no Cosca (`internal/bridge/websocket.go`)

Adiciona um **cliente WebSocket real** que implementa a interface `Client` já existente.

**Responsabilidades**:
- `Connect(url)` — dial no servidor do Unreal (`ws://localhost:9000/cosca`)
- `Send(msg)` — serializa `Message` (JSON) e envia
- `Receive(msg)` — lê `Message` do servidor
- `OnMessage(type, handler)` — dispatch por tipo (reutiliza `handlers map`)
- Reconexão backoff + heartbeat (`ping`/`pong`)

**Por que WebSocket (decisão D1/D3/D4 do mapa)**:
- Servidor nativo do Unreal: `IWebSocketServer::Init(port, callback)`
- Win64 suportado
- Bidirecional, JSON nativo, HTTP estático junto

**Assinatura (aderente à interface `Client`)**:
```go
// internal/bridge/websocket.go
package bridge

type WebSocketClient struct {
    conn    *websocket.Conn
    mu      sync.Mutex
    handlers map[MessageType]func(Message)
    // ... backoff, heartbeat
}

func NewWebSocketClient() *WebSocketClient
func (c *WebSocketClient) Connect(ctx, url) error
func (c *WebSocketClient) Disconnect(ctx) error
func (c *WebSocketClient) Send(ctx, msg Message) error
func (c *WebSocketClient) Receive(ctx) (Message, error)
func (c *WebSocketClient) SendFrame(ctx, frame FramePayload) error
func (c *WebSocketClient) SendAction(ctx, action ActionPayload) error
func (c *WebSocketClient) OnMessage(t MessageType, h func(Message))
func (c *WebSocketClient) IsConnected() bool
```
> O protocolo WS `github.com/gorilla/websocket` ou `nhooyr.io/websocket` (Go). Alternativa sem dependência: usar TCP/JSON — mas WS é nativo no Unreal, então é a escolha.

**Tests (Cosca)**:
- `websocket_test.go` — servidor WS fake (teste de loopback): connect, send, receive, dispatch
- **Não** depende do Unreal (servidor mock)

---

### Peça B — Plugin `CoscaRuntime` no Unreal

Um plugin `CoscaRuntime` dentro do projeto Unreal do Don (`<Projeto>/Plugins/CoscaRuntime/`).

**Estrutura**:
```
Projeto/Plugins/CoscaRuntime/
├── CoscaRuntime.uplugin           # Type: Runtime, Win64
├── Source/CoscaRuntime/
│   ├── CoscaRuntime.Build.cs
│   ├── Public/
│   │   ├── CoscaWorldSubsystem.h  # UWorldSubsystem (hub)
│   │   ├── CoscaEntityComponent.h # UActorComponent (id/hash)
│   │   └── CoscaTypes.h           # structs do contrato
│   └── Private/ (implementações + WebSocket server)
```

**`UCoscaWorldSubsystem`** (o hub — decisão #3 do mapa):
- `Initialize()` → sobe `IWebSocketServer::Init(9000, OnClientConnected)`
- `OnCommand(Message)` → switch: `spawn_actor`, `move_actor`, `destroy_actor`, `update_state`, `vfx`, `audio`
- `SpawnActor(EntityDef)` → cria `AActor` + `UCoscaEntityComponent(id, hash)` + `AddGameplayTag`
- `MoveActor(id, target)` → `NavMesh` pathfind + apply
- `CaptureFrame()` → câmera do player → JPEG/PNG → `obs.frame`
- `SendObs(state)` → serializa WorldState → envia ao Cosca

**`UCoscaEntityComponent`**:
- `FString EntityId`, `FString AssetHash`, `FGameplayTagContainer Tags`
- liga `WorldEntity.id` ↔ `AActor`

**Dependências do plugin**:
- `WebSocketNetworking` (servidor)
- `GameplayTags` (vocabulário)
- `Engine`, `NavigationSystem`, `Interchange` (asset import)

---

## Contrato de Dados (JSON/WS) — correlação por `seq`

### Cosca → Unreal (comandos) — reusa `Message.Type` do bridge
| `type` | payload | ação |
|--------|---------|------|
| `spawn` | `SpawnPayload` | spawna `AActor` |
| `action` | `ActionPayload` | move/interact |
| `destroy` | `DestroyPayload` | remove `AActor` |
| `modify` | `ModifyPayload` | atualiza props |
| `weather` | (climate) | muda clima |
| `time` | (time) | muda hora |
| `ping` | — | heartbeat |

### Unreal → Cosca (observações) — reusa `Message.Type` do bridge
| `type` | payload | significado |
|--------|---------|-------------|
| `frame` | `FramePayload` | câmera → Cosca Vision |
| `audio` | (audio) | som → Cosca Audio |
| `event` | (colisão) | evento do mundo |
| `state_sync` | `WorldState` | snapshot real |
| `pong` | — | heartbeat |

### `WorldEntity.id` = `Actor id` — correlação
- `spawn` manda `id`; o EntComponent guarda `id`; `move/destroy/update` referenciam `id`.
- `seq` para request/response trace (debug/replay).

---

## Estrutura dos Arquivos (o que criar)

### No Cosca (`internal/bridge/`)
- `websocket.go` — `WebSocketClient` (implementa `Client`)
- `websocket_test.go` — mock server loopback

### No Unreal (`Plugins/CoscaRuntime/`)
- `CoscaRuntime.uplugin`
- `Source/CoscaRuntime/CoscaRuntime.Build.cs`
- `Source/CoscaRuntime/Public/CoscaWorldSubsystem.h` + `.cpp`
- `Source/CoscaRuntime/Public/CoscaEntityComponent.h` + `.cpp`
- `Source/CoscaRuntime/Public/CoscaTypes.h`

### No projeto Unreal
- `World` com `AGameModeBase` que spawna o subsystem (via Config/`Map`)
- Um `AActor` `FirstActor` (para o slice)
- Config: `DefaultGame.ini` → ativar plugin + o WorldSubsystem

---

## Vertical Slice — passo a passo (funcional)

```
1. Don abre o projeto Unreal (UE_5.8) → GameMode spawna UCoscaWorldSubsystem
2. UCoscaWorldSubsystem.Initialize() → WS server sobe em :9000
3. Don roda: cosca bridge connect --url ws://localhost:9000/cosca
4. Cosca envia:  Message{Type:"spawn", Payload:{id:"cube_1", type:"object",
                  position:[0,0,50], asset_hash:"sha256:17dd8a..."}}
5. Unreal: importa GLB (se falta) → spawna AActor + UCoscaEntityComponent(id:"cube_1")
           → responde Message{Type:"pong"} (ack)
6. Cosca envia:  Message{Type:"action", Payload:{entity_id:"cube_1",
                  action:"move_to", params:{target:[100,0,50]}}}
7. Unreal: NavMesh pathfind → move o Actor → responde state_sync (posição final)
8. Unreal envia Message{Type:"frame", Payload:cameraPNG}
9. Cosca: Vision processa o frame → atualiza WorldState
10. Cosca reconciles com state_sync → repete
```

---

## Testing Strategy

| Camada | Ferramenta | Caso |
|--------|-----------|------|
| Cosca unit | `go test ./internal/bridge/` | WebSocketClient loopback (mock server) |
| Cosca integração | `go test ./internal/worldmodel/...` | Orchestrator + bridge (já verde) |
| Unreal unit | `AutomationTest` (C++) | parse de `Message`, spawn/move |
| Unreal func | `FunctionalTesting` | spawn→move→verifica posição in-world |
| E2E | `UnrealEditor-Cmd -run=Automation` headless | ponta a ponta Cosca↔Unreal |
| Slice real | binário Cosca + Unreal rodando | passo a passo do slice |

---

## Ordem de Implementação

| Etapa | O que | Resultado |
|-------|-------|-----------|
| E1 | `internal/bridge/websocket.go` + teste mock | Cosca fala WS (isolado) |
| E2 | Plugin `CoscaRuntime` (subsystem + entity comp + WS server) | Unreal escuta :9000 |
| E3 | Conectar: `cosca bridge connect` | handshake |
| E4 | slice: spawn cube_1 | Actor aparece |
| E5 | slice: move + ack | Actor se move, Cosca reconcilia |
| E6 | slice: capture frame + Vision | Cosca "vê" o mundo |
| E7 | Blender→Unreal: `cmd.spawn_asset` com GLB real | asset do Blender entra no mundo |
| E8 | testes de integração | CI green |

---

## Estimar Esforço

| Peça | Complexidade | Dias (aprox.) |
|------|--------------|---------------|
| WebSocketClient (Cosca) | Média | 0.5 |
| Plugin CoscaRuntime (subsystem+server) | Média-Alta | 2-3 |
| Spawn/Move funções | Média | 1 |
| Frame capture + envio | Média | 1 |
| Blender→Unreal import | Média | 1 |
| Testes E2E | Alta | 1-2 |
| **Total** | | **~6-9 dias** |

---

## Riscos & Mitigações

| Risco | Mitigação |
|-------|-----------|
| `WebSocketNetworking` é "Experimental" | Remote Control usa-o em produção; fallback p/ RemoteControl API ou TCP/JSON |
| Cosca vs Chaos (autoridade de física) | Cosca reconcilia com `state_sync` (posição real) |
| Build Unreal lento | UBT incremental; plugin mínimo; CI separado |
| EULA | não distribuir engine modificado |
| C++ complexo | manter plugin minimal; Blueprint para glue |
```

---

## Próxima Ordem (após aprovação do Don)

1. **E1**: implementar `internal/bridge/websocket.go` (Cosca) + teste mock
2. **E2**: criar plugin `CoscaRuntime` (requer VS + projeto Unreal)
3. **E3-E6**: vertical slice funcional

> **Aguardando aprovação do Don antes de implementar qualquer código.** Este documento é a proposta final da mineração.
