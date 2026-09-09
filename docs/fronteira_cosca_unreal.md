# Fronteira Cosca ↔ Unreal — Contrato Canônico

> **Decisão do Don (2026-08-24):** Cosca Go e Unreal são módulos SEPARADOS.
> **Regra de ouro:** Nenhum conhecimento do Mundo Cosca depende da Unreal.
> **Fase:** FASE 3 — Fronteira

---

## 1. A Fronteira Arquitetural

```
          COSCA (cérebro — Go Kernel)
                │
        World Model (semântico)
        Knowledge / Memory
        Protocol / SDK
                │
            contract/API
                │
                ▼
        COSCA-UNREAL (repo próprio)
                │
         Plugin CoscaRuntime
                │
                ▼
          Unreal Engine (corpo)
```

**O Go conhece entidades e comandos.**
**O plugin Unreal traduz para Unreal.**

---

## 2. Quem é DONO de cada conceito

| Conceito | Dono | Cosca Go | Unreal |
|----------|------|----------|--------|
| Mesa/World State | **Cosca** | `worldmodel.WorldState` | — (recebe snapshot) |
| Entidade canônica | **Cosca** | `worldmodel.WorldEntity`/`Entity` | — |
| AssetID | **Cosca** | `asset_registry` | ler registry |
| Transform | **Cosca** | `Vec3`/`Quat` | receber pos/rot |
| Clima | **Cosca** | `ClimateState` | receber weather |
| TimeOfDay | **Cosca** | `TimeOfDay` | receber hour |
| Command/Event | **Cosca** | `Command`/`Event` | executar/envia |
| **AActor** | **UNREAL** | — | `SpawnEntity` |
| **UStaticMesh** | **UNREAL** | — | `ImportMesh` |
| **UWorld** | **UNREAL** | — | `GetWorld()` |
| **UBlueprint** | **UNREAL** | — | — |
| LOD/Nanite/HISM | **UNREAL** | — | otimização |

**REGRAS:**
- Cosca NUNCA sabe que `tree_001` é um `AStaticMeshActor`.
- Cosca NUNCA importa `AActor`, `UStaticMesh`, `UWorld`, `UBlueprint`.
- Cosca NUNCA referencia paths `/Game/...` — usa AssetID.

---

## 3. Contrato Cosca ↔ Unreal

### 3.1 Commands (Cosca → Unreal, "faça isso")

| Command | Payload | Dono Cosca |
|---------|---------|------------|
| `spawn_entity` | entity_id, asset_id, transform | ✅ |
| `destroy_entity` | entity_id | ✅ |
| `set_transform` | entity_id, position, rotation, scale | ✅ |
| `set_material` | entity_id, material_ref | ✅ |
| `set_time` | hour (0-24) | ✅ |
| `set_weather` | type, intensity | ✅ |
| `load_asset` | asset_id | ✅ |
| `load_region` | region_id | ✅ |

### 3.2 Events (Unreal → Cosca, "isso aconteceu")

| Event | Payload | Origem Unreal |
|-------|---------|---------------|
| `entity_spawned` | entity_id | ✅ |
| `entity_destroyed` | entity_id | ✅ |
| `collision` | entity_a, entity_b, point | ✅ |
| `tree_destroyed` | entity_id | ✅ |
| `rain_started` | intensity | ✅ |
| `sunrise` / `sunset` | — | ✅ |
| `npc_entered_region` | npc_id, region_id | ✅ |
| `asset_failed` | asset_id, error | ✅ |

### 3.3 Protocolo

- **Transporte:** WebSocket (porta 9000, bytes binários).
- **Formato:** JSON.
- **Unreal** = servidor, **Cosca** = cliente.
- **Direção:** Cosca envia `Command`, Unreal executa e retorna `Event`.
- **Idempotência:** command tem `cmd_id`; Unreal dedupica.

---

## 4. Estrutura de Repositórios

### Cosca (Go Kernel) — repo atual
```
cmd/          → executáveis CLI
internal/     → pacotes Go (worldmodel, bridge, knowledge, ...)
pkg/          → libs públicas
sdk/          → SDK oficial
docs/         → arquitetura, world, protocols, decisions
```

### Cosca-Unreal (repo próprio — PROPOSTO)
```
CoscaRuntime.uplugin      → plugin UE
Source/CoscaRuntime/      → C++ do plugin
Content/                  → assets UE
Scripts/                  → python de import
README.md                 → docs do plugin
```

---

## 5. Regras de Fronteira (para os capos)

1. **Nenhum código Go referencia Unreal** (`AActor`, `UStaticMesh`, `UWorld`, `/Game/`).
2. **Nenhum conhecimento do mundo depende de implementação Unreal.**
3. **Nenhuma conversão geográfica espalhada** — um único conversor no Cosca.
4. **Nenhum procedimento gera conteúdo sem seed/versão/ID/proveniência.**
5. **Schemas sempre com `schema_version`.**
6. **Toda feature nova: EXATAMENTE UMA representação canônica.**

---

## 6. Status

- ✅ FASE 1 (parar)
- ✅ FASE 2 (identidade: raiz canônica, cosca/ órfã, nada perdido)
- ✅ FASE 3 (fronteira definida — este documento)
- ⏳ FASE 4 (learnings: preservar histórico)
- ⏳ FASE 5 (migrar Unreal → repo, atualizar bridge, testar, build, commit)

**PRÓXIMO BLOQUEIO (regra do Don):** Não iniciar World Model / Google Maps /
cidade procedural até a fronteira Cosca ↔ Unreal estar DEFINIDA E IMPLEMENTADA.
