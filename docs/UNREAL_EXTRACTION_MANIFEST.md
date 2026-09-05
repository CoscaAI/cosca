# UNREAL_EXTRACTION_MANIFEST.md

> Migração reversível do projeto Unreal (CoscaRuntime) para repositório próprio.
> **Regra:** COPIAR → VALIDAR → COMPARAR → CORTAR. Nada do `unreal/` original é apagado até os testes passarem.
> **Autor:** cosca-kernel | **Data:** 2026-08-24

---

## 1. Baseline de Referência

| Item | Valor |
|------|-------|
| HEAD Cosca (origem) | `95f601b0241ad48792a33c3c246958be786e79e9` |
| Branch | `main` |
| Remote Cosca | `github.com/CoscaAI/cosca-v4` |
| Árvore origem | `unreal/` (no repo Cosca) |
| Merkle hash (baseline) | `A84A7B5CAC930188CFD3EF421B86FD3C6B8AAA6DF54CD64A8A1D03303C6524DB` |
| Arquivos | 9 |
| Plugin | CoscaRuntime |

> O **Merkle hash** é calculado sobre os SHA256 dos 9 arquivos ordenados + contagem.
> Este é o fingerprint da árvore ANTES da migração — referência para validar a cópia.

---

## 2. Manifesto de Arquivos (arquivo / SHA256 / origem / destino / responsabilidade)

| # | Arquivo (origem: `unreal/CoscaRuntime/`) | SHA256 (completo) | Destino (`CoscaAI/cosca-unreal/`) | Responsabilidade |
|---|------------------------------------------|-------------------|-----------------------------------|------------------|
| 1 | `CoscaRuntime.uplugin` | `58825D712B6F4D14...` | `CoscaRuntime/CoscaRuntime.uplugin` | Plugin UE manifest |
| 2 | `README.md` | `90FE56FB9D8D784B...` | `README.md` | Docs do plugin |
| 3 | `Source/CoscaRuntime/CoscaRuntime.Build.cs` | `4F48EDF17A364763...` | `Source/CoscaRuntime/CoscaRuntime.Build.cs` | Módulos de build |
| 4 | `Source/CoscaRuntime/Private/CoscaEntityComponent.cpp` | `1B1BC8A55C9DD155...` | `Source/CoscaRuntime/Private/CoscaEntityComponent.cpp` | Componente de entidade |
| 5 | `Source/CoscaRuntime/Private/CoscaRuntimeModule.cpp` | `E4BDC90EEB9081C5...` | `Source/CoscaRuntime/Private/CoscaRuntimeModule.cpp` | IMPLEMENT_MODULE |
| 6 | `Source/CoscaRuntime/Private/CoscaWorldSubsystem.cpp` | `8B23A194FE99CEE9...` | `Source/CoscaRuntime/Private/CoscaWorldSubsystem.cpp` | WebSocket server + spawn + import + time/weather |
| 7 | `Source/CoscaRuntime/Public/CoscaEntityComponent.h` | `AFCC1953A484B1D1...` | `Source/CoscaRuntime/Public/CoscaEntityComponent.h` | Header componente |
| 8 | `Source/CoscaRuntime/Public/CoscaTypes.h` | `3F91CB36CF5C0FDC...` | `Source/CoscaRuntime/Public/CoscaTypes.h` | Contrato de tipos (payloads) |
| 9 | `Source/CoscaRuntime/Public/CoscaWorldSubsystem.h` | `0935BD898C4A3390...` | `Source/CoscaRuntime/Public/CoscaWorldSubsystem.h` | Header subsystem |

> SHA256 completos foram registrados durante a preparação (5.1). Este manifest referencia
> os primeiros 16 chars para legibilidade; a validação usa o hash completo.

---

## 3. Dependências Externas (Build.cs)

Módulos UE requeridos pelo plugin: `Core, CoreUObject, Engine, GameplayTags, NavigationSystem, Json, JsonUtilities, HTTP, WebSocketNetworking, AIModule, InputCore` (público) + `NetCore` (privado).

---

## 4. Referências Externas ao `unreal/` (no repo Cosca)

### 4.1 Código Go
- **NENHUMA dependência de path de arquivo UE.** ✅
- O Go usa **contrato** (AssetID + WebSocket JSON), não paths internos.
- Única menção a `/Game/` é um comentário de doc (`internal/bridge/controller.go:49`).
- Menções a `unreal`/`CoscaRuntime` em Go são: nome do módulo + string de tipo de entidade, NÃO paths de arquivo.

### 4.2 Scripts / Configs / CI
- **NENHUMA referência a `unreal/`** em `.ps1`, `.sh`, `.yml`, `.json`, `.toml` de scripts/deploy/build. ✅

### 4.3 Import do projeto UE real
- Cópia de trabalho em `E:\Epic Games\CoscaWorld\Cosca 5.8\Plugins\CoscaRuntime\` (fora do git, local do editor).

---

## 5. Plano de Migração Reversível

```
ANTES (agora)                    DEPOIS DA CÓPIA (5.3)
Cosca repo                        Cosca repo          Cosca-Unreal repo
└── unreal/  ────────────────→   └── unreal/ (intacto) └── plugin/
         ↑                             (conteúdo válido duplicado)
     dupla cópia válida
```

| Etapa | Ação |
|-------|------|
| 5.1 | ✅ Preparação (baseline + manifest) |
| 5.2 | Criar `CoscaAI/cosca-unreal` (estrutura) |
| 5.3 | **COPIAR** `unreal/CoscaRuntime/` → repo novo; validar por hash (Merkle == baseline) |
| 5.4 | Adaptar fronteira: bridge puro por contrato (já está) |
| 5.5 | **VALIDAR**: build Cosca, build plugin UE, PIE, WebSocket, spawn, time |
| 5.6 | **CORTAR** só depois: commit em ambos, registrar SHA, remover cópia antiga |

---

## 6. O que NÃO pode acontecer

- ❌ Apagar `unreal/` antes do teste 5.5 passar.
- ❌ Mover sem gerar manifest (é a auditoria).
- ❌ Mudar o contrato Cosca↔Unreal durante a migração (é a fronteira).
- ❌ Iniciar World Model antes da fronteira implementada.

---

## 7. Status

- ✅ 5.1 Preparação (baseline + manifest)
- ✅ 5.2 Criar destino (`cosca-unreal` em temp)
- ✅ 5.3 COPIAR + validar (Merkle IDENTICAL `A84A7B5C`)
- ✅ 5.4 Adaptar fronteira (contrato Go↔UE 1:1, sem paths internos)
- ✅ 5.5 VALIDAR (2026-08-24):
  - ✅ Build plugin UE: Succeeded (3 módulos)
  - ✅ Plugin carrega (WorldSubsystem init)
  - ✅ Ping/pong: Round-trip OK
  - ✅ import_mesh (tree): spawned
  - ✅ import_mesh (ground): spawned
  - ✅ TimeOfDay: hour=18.0 elevation=-0.0 (dll nova 297KB)
  - ✅ Weather: type=rain intensity=0.80
  - ⚠️ **BUG CORRIGIDO:** RunUAT BuildPlugin entrega em `LocalBuilds/Binaries/`, mas editor carrega de `Binaries/`. Sync manual `.dll`+`.pdb`+`.modules`. Editor aberto com `-NoLiveCoding` (evita recompile do módulo Cosca).
- ⏳ 5.6 Cortar (SÓ APÓS commit em ambos + registrar SHA)
