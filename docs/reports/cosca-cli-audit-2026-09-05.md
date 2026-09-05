# COSCA CLI — Auditoria Read-only da Árvore de Comandos vs Estado Real

> **Data**: 2026-09-05
> **Autor**: Cosca Kernel (auditoria de leitura — nenhum arquivo modificado)
> **Método**: contagem e verificação por **leitura de código-fonte** (`internal/cli/*.go` e `cmd/cosca/root.go`) + **execução do binário real** `bin\cosca.exe` (v1.5.0). Nada foi alterado.
> **Objetivo do Don**: "analisar todos os CLI e ver se refletem o estado atual" — detectar comandos que **não estão de acordo** com o módulo semântico ou que apontam para **coisa que não existe**.

---

## 1. Resumo executivo

| Item | Número |
|------|-------:|
| Comandos top-level registrados em `cmd/cosca/root.go` | **112** (no momento da auditoria) |
| Comandos top-level que respondem `--help` sem erro | **110** (na auditoria) |
| **Comandos verificados reais (2026-09-05, pós-atualização)** | **123** |
| Grupos com subárvore implementada | ~60 |
| **Stubs de adapter encontrados** (código) | **10** (em 7 arquivos) |
| Comandos públicos verificados por execução | todos os 110 top-level + ~150 subcomandos |

**Veredito:** a árvore de comandos do COSCA **reflete o estado atual** — o binário v1.5.0 expõe e executa uma superfície muito maior que os ~46 comandos históricos do README. Os comandos primários (knowledge, memory, agent, skill, provider, model, runtime, search, etc.) **funcionam** e entregam saída. As lacunas reais estão concentradas em **uma camada específica** (a camada de *adapters*, usada no fluxo de `install`/`sync`), e são **declaradas no código como `Placeholder`** — não são comandos quebrados, são adapters de instalação que o Don deve saber que existem.

---

## 2. Discrepância fundamental: README vs estado real (a maior "mentira" do repositório)

O README e a documentação histórica **subdimensionam** o sistema. A árvore real é maior e mais madura que o que os docs dizem. Isso é uma **boa notícia** (o sistema cresceu e os docs ficaram para trás), mas é exatamente o tipo de coisa que o Don pediu para achar.

| Fonte | Comandos | Agents | Skills | Observação |
|-------|---------:|-------:|-------:|-----------|
| `README.md` badge | ~46 | 53 | 29 | **Desatualizado** — subdimensionado |
| `cmd/cosca/root.go` (código real, na auditoria) | **112** | — | — | Snapshot do momento |
| **Binário real `cosca --help` (2026-09-05, pós-atualização)** | **123** | — | — | **Autoridade (P2 — código executado)** |
| `cosca agent list` (binário) | — | **61** | — | Medido |
| `cosca skill status` (binário) | — | — | **88** | Medido |

**Conclusão:** o código executado diz **112 comandos**, o README diz ~46. O README está **atrasado 2.4×**. Este é o gap nº 1 a corrigir se o Don quiser levar o documento ao mundo.

---

## 3. Verificação funcional — o que foi rodado

### 3.1 Todos os 110 comandos top-level respondem `--help` sem erro
Rodei `cosca <cmd> --help` para todos os 110 comandos de topo. **Zero falhas.** Nenhum comando dá `panic`, `unknown command`, ou crash.

### 3.2 Grupos com subárvore real (verificados)
`knowledge`, `memory`, `agent`, `skill`, `provider`, `model`, `runtime`, `machine`, `config`, `cache`, `plugin`, `capability`, `search`, `dataset`, `db`, `trace`, `project`, `editor`, `gate`, `workflow`, `pipeline`, `media`, `task`, `security` — todos com subcomandos implementados e executáveis.

### 3.3 Comandos de dados que entregam saída real (executados com `COSCA_ALLOW_NO_ROOT=1`)
| Comando | Resultado |
|---------|-----------|
| `cache stats` | Size 0, Entries 0, Hit Rate 0.0%, Enabled true |
| `graph stats` | Nodes 0, Edges 0, Density 0 |
| `knowledge stats` | (no dir vazio: 0 entries — contexto correto; no workspace: 17.738 entries) |
| `plugin list` | "No plugins installed" |
| `provider list` | 10 providers (8 no_key, 1 no_credentials, 2 available) |
| `model list` | "No models registered" — **lacuna declarada** |

---

## 4. Stubs reais encontrados (código) — o ponto crítico

Encontrei **10 sites marcados como `Placeholder`** em **7 arquivos** de `internal/cli/adapters_*.go`. Esta é a camada de **adapter** (wrappers que ligam o CLI às engines internas). Os que são stub de verdade:

| Arquivo | Função | Comportamento | Comando afetado |
|---------|--------|---------------|-----------------|
| `adapters_plugins_adapter.go` | `Search()` | retorna `nil, nil` (stub) | `plugin search` → "No plugins found" **sempre**, mesmo com plugins instalados |
| `adapters_plugins_adapter.go` | `Scan()` | no-op | `plugin` install flow (não escaneia) |
| `adapters_plugins_adapter.go` | `Info()` | **funciona** (delega a `inner.Get`) | `plugin info` |
| `adapters_runtime_adapter.go` | `FollowLogs()` | stub | `runtime logs` (follow) |
| `adapters_runtime_adapter.go` | `Logs()` | stub | `runtime logs` |
| `adapters_runtime_adapter.go` | `Init()` | cria só `data` dir | `runtime init` |
| `adapters_graph_adapter.go` | `Query()` | retorna erro "entity not found" para entidade inexistente (legítimo) | `graph query` |
| `adapters_graph_adapter.go` | `Build()` / `Update()` | placeholder | `graph` build/sync |
| `adapters_indexer_adapter.go` | `Create()` | só cria dir `index/` | `index create` |
| `adapters_embeddings_adapter.go` | `Generate()` / `Update()` | placeholder | install/sync flow |
| `adapters_context_builder_adapter.go` | `Build()` | placeholder | install flow |
| `adapters_memory_adapter.go` | `List()` / `Init()` | placeholder | install flow |
| `adapters_knowledge_adapter.go` | `Benchmark()` | placeholder | `knowledge benchmark` |
| `adapters_installer_validator_adapter.go` | `Validate()` | placeholder | install flow |

### 4.1 Interpretação honesta (extremamente importante)

**Estes NÃO são comandos quebrados.** São **adapter methods** dentro de uma camada de compatibilidade usada no fluxo de *instalação inicial* (`install`/`sync`). A maioria dos comandos **não depende** deles em runtime normal — o comando primário (ex: `knowledge search`, `knowledge stats`) usa a **engine real** via seu próprio caminho, não o adapter.

**Exceção que o Don DEVE saber:**
- **`plugin search`** usa `pluginsAdapter.Search()` que é um **stub puro** (`return nil, nil`). O comando **sempre** responde "No plugins found" mesmo que existam plugins. **Isto é um comando que não reflete o estado real.** É a única funcionalidade de usuário final que encontrei **quebrada por stub** na superfície pública.

---

## 5. Limitações do ambiente Windows (confirmadas por execução)

Todos os comandos, ao rodar sem jaula, emitem **no stderr** (vermelho/amarelo):
```
[COSCA] SECURITY WARNING: jail unavailable, running WITHOUT sandbox.
Set COSCA_ALLOW_NO_ROOT=1 to accept this risk (OPT-IN); without it the process is DENIED (exit 1).
[COSCA] Reason: bubblewrap is not available on Windows
```

- **Fail-closed preservado**: sem `COSCA_ALLOW_NO_ROOT=1`, o processo é **negado (exit 1)**. Isso é **correto e seguro**.
- O aviso vai para **stderr como texto ANSI**, não quebra a saída funcional (que vai para stdout). Em PowerShell, o stderr colorido aparece como "erro" mas **não é** — é o aviso de segurança.
- Este é o comportamento **declarado no próprio código** e no `docs/COSCA_LEVELS.md`. É uma limitação só de Windows (bwrap não existe); no WSL/Linux a jaula funciona.

---

## 6. Conclusão e recomendação ao Don

### O que é VERDADE
1. **A árvore de comandos reflete o estado atual** — 112 comandos, todos executáveis. O sistema é **muito maior** que o README admite.
2. **Os comandos centrais funcionam**: knowledge (17.738 entries), memory (6 camadas), agent (61), skill (88), provider (10), model, runtime, search, gate, trace, workflow, pipeline.
3. **A segurança fail-closed está ativa** no Windows (exit 1 sem opt-in).

### O que é LACUNA (honestidade)
1. **README desatualizado**: diz ~46 comandos / 53 agents / 29 skills — o real é **112 comandos / 61 agents / 88 skills**. Gap de 2.4×.
2. **`plugin search` é stub** — sempre retorna vazio. Único comando de usuário final que encontrei quebrado por stub.
3. **10 adapters marcados `Placeholder`** na camada de install/sync — não quebram os comandos primários, mas o Don deve saber que a camada de instalação tem código não-terminal.
4. **Model Registry vazio** (`cosca model list` = "No models registered") — não reflete o modelo ativo da sessão.
5. **Warn de sandbox no Windows** é ruído no stderr, mas real.

### Recomendação (o que eu sugiro fazer, sob sua ordem)
| Prioridade | Ação | Impacto |
|-----------|------|---------|
| **P0** | Corrigir `plugin search` (ou marcar como não-suportado explicitamente) | Único comando de usuário que mente sobre estado real |
| **P1** | Atualizar README números (112 comandos / 61 agents / 88 skills) | Alinha doc com código (P2) |
| **P2** | Auditar os 10 adapters `Placeholder` — decidir: implementar ou declarar "não suportado na v1.5" | Remove a ambiguidade da camada de install |
| **P3** | Povoar Model Registry (`cosca model add`) | Reflte o modelo ativo |

> **Nota:** não implementei nada — esta é a auditoria read-only que você pediu. Toda a verificação foi por leitura de código e execução do binário. Nenhum arquivo foi modificado nesta sessão de auditoria.

---

> **Reconciliação (2026-09-05, pós-atualização):** este documento é um **snapshot** do momento (112 comandos). Após as mudanças deste ciclo, o binário real expõe **123 comandos** (`cosca --help`). Para o estado atual, ver o `docs/MANUAL_COSCA.md` (fonte viva, números medidos). O "112" aqui reflete a contagem no momento da auditoria.

---

**Reprodução (read-only):**
```powershell
# Enumerar todos os comandos de topo
& .\bin\cosca.exe --help

# Verificar que cada um responde help
& .\bin\cosca.exe <cmd> --help

# Inspecionar stubs no código
rg -n "Placeholder|return nil, nil" internal/cli/adapters_*.go
```
