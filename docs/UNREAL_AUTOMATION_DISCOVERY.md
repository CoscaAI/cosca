# UNREAL AUTOMATION — Discovery, Causa Raiz e Plano

> **Autor:** cosca-kernel | **Data:** 2026-08-24
> **Missão (professor):** Eliminar a dependência de "aperte Play". O Editor
> deve ser ferramenta de inspeção humana, não requisito para o pipeline.

---

## 1. DIAGNÓSTICO — Causa Raiz da Dependência de Play

### 1.1 O que está acontecendo
O comando `cosca bridge connect` e `cosca world spawn` **exigem que o usuário
aperte Play** no editor. Sem Play, o WebSocket na 9000 não sobe.

### 1.2 CAUSA RAIZ (encontrada no código)
O bridge WebSocket está preso ao **ciclo de vida do Runtime/PIE**:

```cpp
// CoscaWorldSubsystem.cpp
class UCoscaWorldSubsystem : public UTickableWorldSubsystem  // ← RUNTIME subsys
{
    ...
    void OnWorldBeginPlay(UWorld& InWorld) override
    {
        Super::OnWorldBeginPlay(InWorld);
        StartServer(9000, TEXT(""));   // ← SÓ roda em PLAY (PIE)
    }
};
```

- `OnWorldBeginPlay` **só dispara quando o mundo entra em PIE** (Play).
- No **Editor** (sem Play), `OnWorldBeginPlay` NUNCA é chamado → sem WebSocket.
- A classe base é `UTickableWorldSubsystem` (Runtime) — só existe com um mundo ativo.

**Conclusão:** o servidor foi colocado no **lifecycle errado**. Ele deveria
viver no **Editor** (via `UEditorSubsystem` ou módulo), não no Runtime de PIE.

---

## 2. CAPACIDADES DE AUTOMAÇÃO (instalação real)

### 2.1 UnrealEditor.exe — flags
| Flag | Uso |
|------|-----|
| `-run=<Commandlet>` | Executa commandlet não-interativo |
| `-ExecutePythonScript=<path>` | Roda script Python |
| `-unattended` | Sem diálogos (headless-friendly) |
| `-nullrhi` | Sem RHI (render null) — CI/server |
| `-game` | Standalone |
| `-NoLiveCoding` | Evita recompile em runtime |

### 2.2 Python no engine
- `python311.dll` presente → **Python Editor Scripting disponível**
- Usar via `-ExecutePythonScript` ou console

### 2.3 Commandlets disponíveis (engine)
`AssetRegistryExport`, `DataValidation`, `MaterialValidation`, etc.
→ Confirma que o padrão `-run=` funciona para automação.

### 2.4 Build/Batch
- `RunUAT.bat` presente (build virtual do plugin — já validado)
- `RunUAT BuildPlugin` → compila sem editor aberto (já funciona)

---

## 3. ESTADO: Processo como Máquina de Estados

O "processo existe" NÃO significa "Unreal pronta". Estados explícitos:

```
STOPPED → STARTING → EDITOR_LOADING → EDITOR_READY →
          TASK_RUNNING → READY
```

Para runtime (só quando necessário):
```
EDITOR_READY → SIM_STARTING → SIM_READY → SIM_RUNNING
```

**Regra:** nunca `sleep` fixo como prova de readiness. Usar capacidade
real (ex: `FileExists(path)` ou ping TCP) como evidência.

---

## 4. RELATÓRIO DO ENVIRONMENT (Unreal Automation Doctor)

Diagnóstico que o `cosca unreal doctor` deve produzir:

| Capacidade | Status | Como verificar |
|------------|--------|----------------|
| Engine | FOUND | `E:\Epic Games\UE_5.8` existe |
| Project | FOUND | `Cosca.uproject` existe |
| Python | AVAILABLE | `python311.dll` presente |
| Commandlets | AVAILABLE | `-run=` funciona |
| Editor Automation | AVAILABLE | `-ExecutePythonScript` |
| Runtime Bridge | ⚠️ BLOCKED (precisa Play) | causar raiz abaixo |

**Runtime Bridge bloqueado** — é o item a corrigir (ver §5).

---

## 5. SOLUÇÃO PROPOSTA — Mover o bridge para o EDITOR

### 5.1 A correção arquitetural (causa raiz)
Criar um **UEditorSubsystem** que inicia o WebSocket **sem Play**:

```cpp
// NOVO: EditorSubsystem (roda no EDITOR, sem PIE)
class UCoscaEditorSubsystem : public UEditorSubsystem
{
    virtual void Initialize(...) override {
        StartServer(9000, TEXT(""));  // ← sobe no EDITOR
    }
    virtual void Deinitialize(...) override {
        StopServer();
    }
};
```

Isso separa:
- **EditorSubsystem** → WebSocket bridge (disponível SEMPRE no editor)
- **WorldSubsystem** → Spawn/import/execução no mundo (runtime)

### 5.2 Alternativa mínima (se não quiser UEditorSubsystem)
Chamar `StartServer` no `Initialize` do módulo (via `IModuleInterface::StartupModule`)
em vez de `OnWorldBeginPlay`. O plugin já é Runtime, mas `StartupModule` roda
na carga do editor, não no PIE.

---

## 6. CONTRATO — Unreal Automation Controller

Camada de abstração (o Kernel NUNCA conhece `UnrealEditor.exe`):

```
UnrealController (interface Go)
├── EnsureEditor()         → inicia editor se preciso (headless)
├── ExecutePython(script)  → roda script no editor
├── RunCommandlet(name,args)→ executa commandlet
├── ImportAsset(req)       → importa sem UI
├── PrepareWorld(req)      → cria/carrega mapa
├── WaitForReady()         → evidência de readiness (não sleep)
├── StartSimulation()/StopSimulation()
└── Shutdown(policy)
```

**Regra:** Cosca pede intenções; a camada Unreal decide como executar.
O Kernel não conhece paths de instalação nem detalhes de processo.

---

## 7. ESTRUTURA DE MÓDULOS (separar Editor vs Runtime)

```
CoscaRuntime.uplugin
├── CoscaRuntime        (Runtime module)   — spawn/import no mundo
├── CoscaEditor         (NEW Editor module) — WebSocket bridge no editor
└── (commandlets próprios, se precisar)
```

`Editor` e `Runtime` têm responsabilidades diferentes — usar o módulo certo
para cada tarefa.

---

## 8. IDEMPOTÊNCIA

Toda operação retornar:
```
Asset tree_001:
  ✓ already imported
  ✓ source unchanged
  → no operation required
```
ou
```
  ✓ source changed → reimported
  ✓ registry updated
```

---

## 9. EPISTEMOLOGIA

| Classe | Exemplo |
|--------|---------|
| FACT | capacidade confirmada por execução |
| MEASURED | startup time, import duration |
| INFERRED | fallback decisions |
| DECISION | modo de execução escolhido |
| EVIDENCE | saída de comando/validação |

---

## 10. ORDEM DE TRABALHO (fases)

- **FASE A — Discovery** ✅ (este relatório: mapeado o que existe)
- **FASE B — Causa raiz** ✅ (OnWorldBeginPlay → Runtime lifecycle deve ser Editor)
- **FASE C — Contrato** (UnrealController, sem contaminar Kernel)
- **FASE D — Vertical slice** (1 operação end-to-end: terminal → import → validar → salvar)
- **FASE E — Runtime** (simular apenas onde necessário)
- **FASE F — Campaign** (repetir, testar falha, reinício, idempotência)

---

## 11. REGRA DO PROFESSOR

> **Não comece criando uma camada enorme. Primeiro estude a Unreal instalada
> e produza relatório de capacidades + causa raiz da dependência de Play.**
> Depois proponha o menor vertical slice que prove autonomia real.

**Este documento atende a FASE A e B.** O próximo passo (FASE D) é o menor
vertical slice: automatizar `cosca unreal import tree_001` de um terminal limpo,
sem Play, via `UnrealEditor.exe -run=...` ou `-ExecutePythonScript`.
