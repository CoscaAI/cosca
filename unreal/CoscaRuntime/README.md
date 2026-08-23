# CoscaRuntime Plugin

O plugin que transforma o Unreal Engine no **corpo/world** do Cosca Living World.
Ele sobe um **WebSocket Server** e recebe comandos do **Cosca (Go)**:
`spawn`, `move`, `destroy`, etc.

## Arquitetura

```
Cosca (Go / cérebro)
  └─ WebSocketClient  ──ws://localhost:9000/cosca──►  Unreal (corpo)
                                                        UCoscaWorldSubsystem
                                                          ├─ IWebSocketServer (porta 9000)
                                                          ├─ SpawnEntity -> AActor + CoscaEntityComponent
                                                          ├─ MoveEntity -> NavMesh pathfind
                                                          └─ SendToCosca (ack/obs)
```

## Instalação

1. Copie a pasta `CoscaRuntime` para o seu projeto Unreal:
   ```
   SeuProjeto/Plugins/CoscaRuntime/
   ```

2. No `.uproject` do projeto, habilite os plugins (opcional — o `.uplugin` já
   declara as dependências WebSocketNetworking/GameplayTags/NavigationSystem):
   ```json
   "Plugins": [
     { "Name": "CoscaRuntime", "Enabled": true },
     { "Name": "WebSocketNetworking", "Enabled": true },
     { "Name": "GameplayTags", "Enabled": true }
   ]
   ```

3. Abra o projeto no Unreal Editor (UE_5.8). O editor compila o plugin via UBT.

4. O `UCoscaWorldSubsystem` **sobe automaticamente na porta 9000** no
   `BeginPlay` (`OnWorldBeginPlay`). Para usar outro port, chame
   `StartServer(Port)` via Blueprint na GameMode.

## Comandos que aceita (contrato)

| `type` | payload | ação |
|--------|---------|------|
| `spawn` | `{id, type, asset_hash, position:[x,y,z], scale:[x,y,z]}` | Spawna `AActor` |
| `action` | `{entity_id, action:"move_to", params:{target:"x,y,z"}}` | Move via NavMesh |
| `destroy` | `{entity_id}` | Remove `AActor` |
| `ping` | — | Ack como `pong` |

## Testar sem o Unreal (mock)

Dois terminais:

```bash
# Terminal 1 — mock server (simula o Unreal)
cosca bridge serve --port 9000

# Terminal 2 — conecta e testa handshake
cosca bridge connect --url ws://localhost:9000/cosca
```

## Aviso

Este código C++ é o **esqueleto da integração**. Ele requer validação e
ajuste final compilando no seu projeto com `UnrealEngine 5.8` (via UBT /
UnrealBuildTool). Teste em `Development Editor` com `--nullrhi` para
rodar `Automation` headless.
