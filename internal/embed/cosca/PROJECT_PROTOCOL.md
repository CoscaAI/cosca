# PROJECT PROTOCOL — Como criar e conduzir projetos na família

> **Versão**: 1.0.0 | **Status**: canônico | **Dono**: cosca-kernel
> **Criado**: 2026-08-16, por ordem do Don — consolidar o padrão de projetos
> **Propósito**: referência operacional ÚNICA para criar, estruturar e conduzir
> projetos (cliente e produto) sem adivinhar. Complementa `CLI_PROTOCOL.md` e
> `MEMORY_ACCESS_PROTOCOL.md`.

---

## 1. Os DOIS tipos de projeto

| Tipo | O que é | Onde vive | Comando |
|------|---------|-----------|---------|
| **Cliente** | projeto de um cliente (HornFit, Iguaracy…) | `~/Documents/projects/<nome>` (ou `$COSCA_PROJECTS_DIR`) | `cosca project` |
| **Produto** | ferramenta da família (cosca-trader, cosca-desktop…) | repositório próprio + `bin/` | PADRÃO COSCA (§5) |

A regra de ouro: **cliente fica FORA do workspace** (`.cosca` isolado, não herda
a chain da família). **Produto é da família** (segue o padrão "uma ferramenta,
um comando").

---

## 2. CRIAR um projeto de cliente

```bash
cosca project new acme-app                    # cria em ~/Documents/projects/acme-app
cosca project new hornfit-cinema --type cinema  # declara o tipo no Manifest
cosca project new acme-app --force            # sobrescreve existente
```

O scaffold gera:
```
acme-app/
├── README.md
├── .gitignore
├── .cosca/          ← isolado (manifest, constraints, config, state)
│   └── project.yaml ← Project Manifest (§4)
├── src/
├── docs/
├── tests/
└── scripts/
```
+ `git init` (se git disponível).

**Isolamento**: o projeto NUNCA herda a chain da família. Quando você roda
`cosca terminal` dentro dele, a jaula monta **AQUELE projeto como root** — o
agente fica preso no projeto, sem ver o workspace do cosca.

---

## 3. TRABALHAR e ENTREGAR

```bash
cosca project list               # lista os projetos
cosca project open acme-app      # imprime o comando cd (ou só o path com --cd)
cd ~/Documents/projects/acme-app
cosca terminal                   # começa a trabalhar (jaula monta o projeto como root)
cosca project manifest acme-app  # mostra o Manifest
cosca project remove acme-app --force  # remove (exige --force)
```

---

## 4. O PROJECT MANIFEST (`.cosca/project.yaml`)

Todo projeto declara um **tipo** e registra o que precisa para ser reproduzível:

- **Tipo** (`--type`): `editor`, `image`, `cinema`, `music`, `game`,
  `scientific`, `3d`, `animation`, `document`, `lab`.
- **Registra**: modelos, assets, workflows, dependências e licenças
  (§33-§35 — proveniência e reprodutibilidade).

> Um projeto sem tipo ainda funciona, mas não é reprodutível. Para ecossistema
> criativo/científico/mídia, **declare o tipo sempre**.

---

## 5. PRODUTO — o PADRÃO COSCA (uma ferramenta, um comando)

Toda ferramenta de produto da família segue o padrão (L52):

| Regra | Detalhe |
|-------|---------|
| **Uma ferramenta, um comando** | `cosca trader`, `cosca desktop`, `cosca voice` — comando global com auto-build |
| **Auto-build + install** | o comando compila sozinho se o binário faltar |
| **`COSCA_WORKDIR`** | respeita o diretório de trabalho atual |
| **Wails (Go + WebView)** | desktop ~15MB, o padrão do cosca-desktop |
| **Código versionado** | git próprio + `.gitignore` (não vaza secret/runtime data) |

Fluxo de produto:
```bash
cosca desktop                    # auto-build + abre na pasta atual
cosca desktop --dev              # modo desenvolvimento (wails dev, hot-reload)
```

---

## 6. O CICLO DE VIDA

```
CRIAR (project new --type) → TRABALHAR (terminal, jail isolado) → ENTREGAR (commit, doc)
                                     ↓
                              REMOVER (project remove --force)
```

Para scaffold completo (12 passos: pre-flight → tech → arquitetura → scaffold →
validação → infra → segurança → CI/CD → qualidade → docs → cosca → git),
ver `workflows/project-init.md`.

---

## 7. COMANDOS — referência rápida

| Comando | Uso |
|---------|-----|
| `cosca project new <nome> [--type <t>]` | criar projeto de cliente |
| `cosca project list` | listar |
| `cosca project open <nome> [--cd]` | cd para o projeto |
| `cosca project manifest <nome>` | ver o Manifest |
| `cosca project remove <nome> --force` | remover |
| `cosca terminal` | trabalhar (jaula monta o projeto como root) |

---

## 8. GOTCHAS — não repita

1. **`remove` exige `--force`** — sem ele, não apaga (proteção contra acidente).
2. **Cliente ≠ produto** — cliente fica fora do workspace com `.cosca` isolado;
   produto segue "uma ferramenta, um comando".
3. **Declare o `--type`** — sem tipo o projeto não é reprodutível (§33-§35).
4. **A jaula monta o projeto como root** — o agente dentro do projeto NÃO vê o
   workspace do cosca (isolamento perfeito).
5. **Projeto de cliente não herda a chain da família** — é uma identidade própria.

---

## 9. HISTÓRICO

| Versão | Data | Mudança |
|--------|------|---------|
| 1.0.0 | 2026-08-16 | Criado por ordem do Don — consolida o padrão de projetos |
