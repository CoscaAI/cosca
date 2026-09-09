# PROJECT SANDBOX PROTOCOL — Isolamento de Projeto (ordem do Don)

> **Versão**: 1.1.0 | **Status**: ativo | **Autoridade**: DON (ROOT) → KERNEL → AGENTS
> **Criado por ordem do Don**: quando um projeto é criado/ativado, o Cosca root
> é isolado e todo o trabalho (kernel + agents) fica confinado ao projeto.
> **Workspace padrão (ordem do Don, 2026-09-02)**: todo novo projeto nasce e
> vive SEMPRE em `<WORKSPACE_PROJETOS>/<nome>` onde `WORKSPACE_PROJETOS` =
> `C:\Users\Henrique\Documents\projects` (no Windows do Don). NUNCA na raiz do
> cosca, nunca em `docs/`, nunca em outro lugar. Cada projeto tem repo git
> próprio. Ex.: RIZOMAI → `Documents\projects\rizomai`.

---

## 1. GATILHOS (termos que ativam o sandbox)

Quando o Don (ou um comando equivalente) emitir **qualquer um** destes, o sandbox
deverá ser ativado para o projeto referenciado:

- "criar projeto <nome>" / "crie um projeto novo" / "projeto novo"
- "ativar protocolo projeto" / "protocolo projeto"
- "projeto <nome>" (ex: "projeto bruno", "projeto hornfit")
- `cosca project new/open/activate <nome>`

O projeto referenciado torna-se o **WORKSPACE ATIVO**.

---

## 2. INVARIANTES (contrato executável)

```
INVARIANTE 1 — WORKSPACE ATIVO:
O diretório de trabalho para TODO comando (build, test, terminal, agents)
é o PROJETO ativo (ex: ~/Documents/projects/bruno), NUNCA o cosca root.

INVARIANTE 1.1 — NASCEDOURO PADRÃO (ordem do Don):
TODO projeto novo é criado em `<WORKSPACE_PROJETOS>/<nome>` (Windows:
C:\Users\Henrique\Documents\projects\<nome>), com repo git próprio, e o
estado do projeto em `<projeto>/.cosca/`. Proibido criar projeto na raiz
do cosca, em docs/, ou fora do workspace de projetos.

INVARIANTE 2 — ESTADO DO PROJETO FICA NO PROJETO:
Memória, knowledge, learnings, audit, provenance e estado gravados durante o
trabalho no projeto DEVEM ser escritos em <projeto>/.cosca/...
NUNCA no cosca root (.cosca do workspace do cosca nem .cosca).

INVARIANTE 3 — COSCA ROOT É READ-ONLY (durante o sandbox):
O kernel e os agents NÃO podem criar/alterar/apagar nada sob o cosca root
(exceto o marcador de estado do sandbox, que é meta-estado do kernel).

INVARIANTE 4 — NENHUM VAZAMENTO:
O que é escrito em um projeto NUNCA é legível nem consultável por outro
projeto. (Verificado por internal/memory/isolation_test.go.)
```

---

## 3. MODELO DE CAMADAS (quem é o quê)

| Camada | Conteúdo | Escopo |
|--------|----------|--------|
| **`.cosca/` do projeto** | runtime do projeto (memória, knowledge, learnings, audit) | **LOCAL ao projeto** |
| **`.cosca/`** | framework do editor (agentes, skills, patterns) | GLOBAL dos devs |
| **`internal/embed/cosca/`** | spec embutida (biblioteca canônica) | GLOBAL por natureza |

> No modo sandbox, **somente a camada 1** (.cosca do projeto) é escrita pelos
> agents. As camadas 2 e 3 permanecem intactas (read-only).

---

## 4. FLUXO DE ATIVAÇÃO

```
DON: "projeto bruno"
  → 1. KERNEL resolve o projeto (ex: ~/Documents/projects/bruno)
  → 2. KERNEL grava o marcador de estado (projeto ativo) em <projeto>/.cosca/sandbox.json
  → 3. KERNEL fixa WORKSPACE = <projeto> para todos os tools (bash workdir, edits, agents)
  → 4. AGENTS recebem escopo: filesystem = <projeto>; memória/learnings → <projeto>/.cosca
  → 5. COSCA ROOT: read-only (nenhum arquivo criado/alterado)
```

**Desativação**: "sair do projeto" / fim da sessão → remove o marcador e restaura o root como workspace.

---

## 5. GARANTIA DO RUNTIME (já existe, verificado)

O runtime Go **já é project-scoped** (prova em `internal/memory/isolation_test.go`):

- `getCoscaDir(workspace) = workspace/.cosca` — memória/knowledge/learnings
  usam esse dir → **local ao projeto quando o workdir é o projeto.**
- `cosca terminal` dentro do projeto monta o projeto como root.

---

## 6. PAPÉIS

- **KERNEL**: detecta o gatilho, resolve o projeto, grava o marcador, fixa o
  workspace, e injeta o escopo nos agents. É o guardião do isolamento.
- **AGENTS**: operam SOMENTE dentro do projeto. Escrevem memória/learnings em
  `<projeto>/.cosca/...`. **Proibido** tocar o cosca root.
- **DON**: autoridade máxima; pode ativar/desativar o sandbox.

---

## 7. TESTES DE CONFORMIDADE

- `go test ./internal/memory/ -run TestProjectIsolation` → PASS (sem vazamento)
- `go test ./internal/project/ -run TestSandbox` → PASS (sandbox dentro do projeto)
- Verificar que `workdir` dos commands == projeto ativo
- Verificar que nenhum arquivo foi escrito sob o cosca root durante a sessão

---

## 8. MEMÓRIA DOS AGENTS EM SANDBOX (redirecionamento obrigatório)

**Regra inegociável (ordem do Don):** enquanto um projeto estiver ATIVO (sandbox),
todo conhecimento gerado pelos agents trabalhando NO projeto DEVE ser escrito em
`<projeto>/.cosca/memory/agent/<nome_do_agent>/learnings.md`.

**PROIBIDO em sandbox:**
- escrever learnings em `.cosca/memory/agent/<nome>/learnings.md` (root do framework)
- criar qualquer estado/arquivo sob o cosca root

**Como o kernel injeta a regra nos agents:**
```
ANTES de delegar trabalho a um agent num projeto ativo, o kernel injeta:
  "workspace = <projeto>
   filesystem confinado a <projeto>
   grave seu aprendizado em <projeto>/.cosca/memory/agent/<seu_nome>/learnings.md
   NUNCA escreva em .cosca/memory/ nem em qualquer arquivo do cosca root"
```

**Conhecimento específico do projeto**: SEMPRE no projeto.
**Conhecimento genérico do framework (padrões reutilizáveis)**: pode ficar também no
framework, MAS o padrão em sandbox é gravar no projeto (nenhum detalhe do projeto no root).

> **Verificado em 2026-08-25**: aprendizado dos agents que construíram o `bruno`
> (devops, documentation, security, testing) foi migrado de `.cosca/memory/agent/`
> para `bruno/.cosca/memory/agent/` e o root foi restaurado limpo (git checkout).

---

*Contrato executável de sandbox de projeto — ordem do Don. Não cria segunda
arquitetura: apenas CONSTRANGE a operação ao projeto ativo, usando o mecanismo
de isolamento que o runtime já possui.*
